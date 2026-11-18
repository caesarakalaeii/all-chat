# ADR-0018: Premium Entitlements via Patreon

**Date**: 2026-06-20
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation; Future Work items were since built, see ADR-0019/0020/0027; a membership is linked to an account only while the Patreon account is connected, and disconnecting deletes it)
**Deciders**: caesarakalaeii

## Context and Problem Statement

`users.is_premium` gates overlay-owner features (sharing, moderation, TTS, stream-selection) via `shared/middleware/premium.go` + the `feature_gates` table (ADR-0008). Today it can **only** be set by an admin (`POST /api/v1/admin/premium/users/:id`, share-service). There is no self-serve monetization.

We want users to **unlock premium themselves by backing All-Chat's own Patreon campaign**. This is All-Chat's first **inbound payment** integration: an external provider must be able to grant and revoke an internal entitlement, automatically and reliably, without an admin in the loop — and without breaking the existing admin-grant capability.

## Decision Drivers

- **Don't touch the read path.** Every consumer of premium (`shared/middleware/premium.go`, `moderation-service`, and the separate `viewers.is_premium` cosmetic flag) should keep reading `users.is_premium` unchanged. We only change *who writes it and how*.
- **Two writers, one boolean — no clobbering.** Admin comps (staff, partners, giveaways) must survive a subscription lapse, and a subscription must not silently override an admin decision. A naive second writer would race the admin write.
- **Reliable identity + lifecycle.** "Subscription grants premium" needs trustworthy identity matching and a real lifecycle (active → lapsed → revoked), including cancellation signals.
- **Idempotent/convergent.** Webhooks, a reconcile job, and the OAuth callback can all touch the same subscription concurrently and out of order; the result must converge.
- **Least surface.** Inbound webhooks + OAuth are a new, security-sensitive surface; isolate them.

## Considered Options

1. **New `payment-service`; `users.is_premium` becomes a derived column** via a tri-state `premium_admin_override` + a shared `RecomputePremium`. Patreon OAuth connect + membership webhooks + reconcile job.
   - ✅ Isolates the new inbound surface; readers untouched; admin and payment are independent inputs that can't clobbing each other; convergent by construction.
   - ❌ One new service; `users.is_premium` is now materialized (must be recomputed on every input change).
2. **Extend `share-service`** (it already owns the admin premium endpoint).
   - ✅ Premium write code already lives there.
   - ❌ Mixes inbound webhooks + OAuth token storage + a reconcile loop into a request/response service; widens blast radius; payment is a distinct concern with its own deploy cadence.
3. **Keep a single boolean; scope the reconcile demotion** to users that have a subscription row (no override column).
   - ✅ No schema change beyond the subscription table.
   - ❌ Cannot express an admin *deny*; admin-write vs webhook-write is a last-writer race; a comped user who once had a (now-lapsed) Patreon row would be wrongly demoted.
4. **Ko-fi instead of / alongside Patreon.**
   - ❌ Ko-fi is webhook-only with **no cancellation webhooks** and email-only identity matching — unfit as the primary "subscription = premium" source. Deferred; Ko-fi support was never built.

## Decision Outcome

**Chosen**: Option 1 — a new `payment-service`, with `users.is_premium` derived from two independent inputs, **Patreon only**, against **All-Chat's own single campaign**.

Effective entitlement (materialized into `users.is_premium` by `shared/premium.RecomputePremium`; since ADR-0020 the derivation also ORs in `users.is_beta_tester`, and since ADR-0027 an override past its expiry counts as absent):

```
is_premium = (users.premium_admin_override IS TRUE)
             OR (users.premium_admin_override IS NULL AND <user has an 'active' premium_subscriptions row>)
```

Key sub-decisions:

- **One campaign (model A).** Users back All-Chat's own Patreon; active patrons at/above `PATREON_MIN_TIER_CENTS` get premium. (Not a multi-tenant "each streamer's own campaign" model.)
- **Tri-state admin override.** `users.premium_admin_override`: `NULL` = follow subscription, `TRUE` = force-grant (comp), `FALSE` = force-deny (reserved). The existing admin endpoint maps **grant → TRUE** and **revoke → NULL** (clears the comp so premium then follows any subscription); `FALSE` is reserved for a future explicit "premium ban" action. This is the mechanism that lets comps survive lapses and keeps the read path unchanged.
- **Convergent write path.** `RecomputePremium(userID)` (in `shared/premium`, callable in-process by both share-service and payment-service) reads the override + an `EXISTS(active subscription)` under `SELECT … FOR UPDATE` and writes the boolean in one transaction. It is a pure function of current rows, so webhook / reconcile / OAuth-callback converge regardless of order or concurrency.
- **Identity via Patreon OAuth**, not email. The user connects Patreon (`identity` scope only, no `identity[email]` — the Patreon user id is enough); we read their membership to our campaign from the `/identity` endpoint (filtered by campaign id) and store the link in `patreon_oauth_tokens` (encrypted with the shared `MultiKeyEncryptor`). Webhooks then resolve the all-chat user by Patreon user id.
- **The membership link follows the connection.** A `premium_subscriptions` row is linked to an account only while that account has the Patreon account connected in `patreon_oauth_tokens`. The upsert reads the subject from the connection, not from its caller, so a webhook for a patron with no connection, or a reconcile pass applying a token that was disconnected after the pass read it, stores the row unlinked; only connecting through OAuth links it again. Disconnecting deletes the connection and the account's membership rows, then recomputes premium, so premium ends at once and a later webhook cannot bring it back. Each reconcile pass first deletes any row still linked without its connection (left by a disconnect from before this rule, or by switching to another Patreon account) and recomputes that account. The data export and account deletion therefore only ever see memberships of a connected Patreon account.
- **Webhooks: HMAC-MD5.** Patreon signs `members:create/update/delete` with **HMAC-MD5 of the raw body** (hex) in `X-Patreon-Signature` — deliberately NOT the Twitch EventSub HMAC-SHA256+timestamp scheme. Verified constant-time over the raw body; deduplicated in Redis after successful processing.
- **Status from Patreon's own signal.** A subscription is `active` iff `patron_status == active_patron` AND `currently_entitled_amount_cents >= PATREON_MIN_TIER_CENTS`. Patreon keeps a patron `active_patron` (with entitled cents) through its own payment-retry/grace window, so we honor its grace and keep **no separate grace timer**. `RecomputePremium` therefore stays time-free.
- **Reconcile backstop (single replica).** A periodic job re-queries every connected user's membership (refreshing tokens near expiry), catching missed webhooks (e.g. a cancellation Patreon never delivered) and revoking premium for lapsed patrons. Single replica so the loop never runs concurrently.

## Consequences

### Positive
- Zero changes to premium **readers**; `users.is_premium` stays the one column everyone checks.
- Admin comps and payment are independent — neither clobbers the other; an admin grant survives a Patreon lapse, and an admin can deny premium independently (when `FALSE` is wired up).
- Convergent, idempotent write path tolerant of webhook loss, retries, and races.
- Strong identity (OAuth + verified Patreon user id) and real cancellation signals (Patreon `members:delete` + reconcile).
- New inbound surface isolated in its own service with its own deploy/secrets/metrics.

### Negative
- `users.is_premium` is now a materialized derivation — every input change must call `RecomputePremium` (a missed call shows stale premium until the next reconcile/recompute).
- Two writers of one boolean — accepted because the derivation makes the result a pure function of the two inputs, not a race.
- One Patreon-specific deviation from the webhook template (HMAC-MD5) — a known footgun, called out in code + README.
- Patreon-only; Ko-fi users are unserved for now.
- Reconcile is single-replica (no HA for that loop; the HTTP surface is unaffected).

## Implementation

- **Migration**: `migrations/063_payment_subscriptions.sql` — `users.premium_admin_override` (tri-state), `patreon_oauth_tokens` (encrypted), `premium_subscriptions` (source of truth, `UNIQUE(provider, provider_user_id)`); it also extended the DSGVO `cleanup_expired_oauth_tokens()` to Patreon tokens, a function migration 118 later dropped (retention moved to the `retention-sweeper` CronJob, ADR-0089, which keeps Patreon tokens). Idempotent (writes no entitlement *values*, so re-runs can't clobber a live grant).
- **Shared**: `shared/premium/recompute.go` — `Effective(override, hasActiveSub)` (the rule, exhaustively unit-tested) + `Recomputer.Recompute` (the transactional write). Later extended by ADR-0019 (`RecomputeViewer`), ADR-0020 (beta-tester OR) and ADR-0027 (override expiry).
- **Service**: `services/payment-service/` — `patreon/` (OAuth client, `/identity` membership read, HMAC-MD5 verify + payload parse, status mapping), `repository/` (token + subscription), `entitlement/` (apply snapshot → upsert → recompute), `handlers/` (connect/callback/status/disconnect/webhook), `reconcile/` (periodic backstop), `cmd/main.go`. Port 8091.
- **share-service**: `repository/premium_repo.go` now writes the override + recomputes (mapping grant→TRUE / revoke→NULL); `cmd/main.go` constructs the `Recomputer`. The admin endpoint, route, and `{is_premium:bool}` contract are unchanged.
- **Gateway**: registry entries + routes in `services/api-gateway/models/service_config.go` and `cmd/main.go` — public `POST /api/v1/webhooks/patreon` (HMAC) and `GET /api/v1/payment/patreon/callback` (one-time Redis state); JWT-protected `GET /payment/patreon/connect`, `GET /payment/status`, `DELETE /payment/patreon/connection`.
- **Frontend**: `frontend/src/app/settings/premium/page.tsx` (Connect / status / Disconnect) + `lib/api/payment.ts`, linked from `settings/page.tsx`.
- **CI/Deploy**: payment-service added to `.github/workflows/build-and-push.yml` (detect-changes, filters, build + test matrices); deploy repo gets `payment-service-deployment.yaml` (replicas:1, no HPA), kustomization + configmap entries. **Manual step**: add `patreon-client-id`, `patreon-client-secret`, `patreon-webhook-secret` to the sealed `allchat-secrets` (ksops reseal), and register the Patreon OAuth client + webhook in the Patreon portal.
- **Tests**: unit (status mapping, HMAC-MD5 verify, payload parsers, `Effective` truth table); integration/E2E with testcontainers (entitlement apply grant/revoke, recompute truth table incl. admin-override-survives-lapse, and a signed synthetic webhook flipping a premium-gated endpoint 403→200→403; disconnect followed by a webhook for the same patron leaves the account unlinked and not premium, `integrationtest/disconnect_test.go`). Live Patreon E2E is intentionally out of scope (interactive OAuth, real charges, no sandbox).

## Related Decisions

- [ADR-0008](./0008-feature-gate-infrastructure.md) — the premium feature-gate layer this feeds.
- [ADR-0016](./0016-linked-twitch-credentials.md) — the encrypted per-user OAuth-token storage pattern reused for `patreon_oauth_tokens`.
- [ADR-0001](./0001-standard-go-layout.md), [ADR-0004](./0004-no-hexagonal-architecture.md) — service layout / shared DB (RecomputePremium called in-process).
- [ADR-0002](./0002-redis-streams-pubsub.md) — Redis (OAuth state + webhook dedup).
