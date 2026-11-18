# ADR-0018: Premium Entitlements via Patreon

**Date**: 2026-06-20
**Status**: Accepted
**Revised**: 2026-11-18 (connection rules folded in; future-work items built in ADR-0019/0020/0027)
**Deciders**: caesarakalaeii

## Context

`users.is_premium` gates overlay-owner features via `shared/middleware/premium.go` and the feature gates (ADR-0008), and could only be set by an admin. Self-serve premium requires an inbound payment integration that can grant and revoke the entitlement automatically, without an admin in the loop, without clobbering admin comps, and without changing any premium reader.

## Decision

A dedicated `payment-service`; `users.is_premium` becomes a materialized derivation from two independent inputs, Patreon-only, against All-Chat's own single campaign:

```
is_premium = (users.premium_admin_override IS TRUE)
             OR (users.premium_admin_override IS NULL AND <user has an 'active' premium_subscriptions row>)
```

Materialized by `shared/premium.RecomputePremium`, callable in-process by both services. Later extensions: ADR-0019 ORs in `viewers`-side logic (`RecomputeViewer`), ADR-0020 ORs `users.is_beta_tester`, ADR-0027 treats an override past its expiry as absent.

Sub-decisions:

- **Tri-state `users.premium_admin_override`**: `NULL` follow subscription, `TRUE` force-grant (comp; survives a lapse), `FALSE` force-deny (reserved). The admin endpoint maps grant → `TRUE`, revoke → `NULL`, unchanged contract.
- **Convergent writes**: recomputation is a pure function of the current rows (override + `EXISTS(active subscription)` under `FOR UPDATE`, one transaction), so webhook, reconcile job, and OAuth callback converge regardless of order.
- **Identity via Patreon OAuth** (`identity` scope only, no email): the membership is read from `/identity` filtered by campaign id; tokens stored in `patreon_oauth_tokens` (shared `MultiKeyEncryptor`); webhooks resolve the user by Patreon id.
- **The membership link follows the connection**: a `premium_subscriptions` row is linked to an account only while the Patreon account is connected. Upserts read the subject from the connection, so a webhook for a disconnected patron stores unlinked. Disconnect deletes the connection and the account's membership rows and recomputes, so premium ends immediately and a later webhook cannot restore it. Each reconcile pass first deletes rows still linked without their connection. Data export and account deletion therefore only ever see memberships of a connected account.
- **Webhooks**: Patreon signs `members:create/update/delete` with HMAC-MD5 of the raw body in `X-Patreon-Signature` (deliberately not the Twitch HMAC-SHA256 scheme; verified constant-time; deduplicated in Redis).
- **Status**: a subscription is `active` iff `patron_status == active_patron` and `currently_entitled_amount_cents >= PATREON_MIN_TIER_CENTS`. Patreon keeps patrons active through its own grace window, so there is no separate grace timer and the recomputation is time-free.
- **Reconcile backstop**: a periodic single-replica job re-queries every connected user's membership (refreshing tokens near expiry), catching missed webhooks and revoking lapsed patrons.

Alternatives rejected: extending share-service (mixes inbound webhooks and a reconcile loop into a request/response service); a single boolean without the override column (cannot express admin deny and would demote comped users); Ko-fi (no cancellation webhooks, email-only identity).

## Consequences

Premium readers unchanged; admin and payment inputs independent; convergent and idempotent; strong identity and real cancellation signals; the new inbound surface isolated. Costs: every input change must call `RecomputePremium` (a missed call leaves stale premium until the next reconcile); one Patreon-specific HMAC-MD5 deviation; Patreon-only; reconcile is single-replica.

## Implementation

- Migration `migrations/063_payment_subscriptions.sql` (idempotent; writes no entitlement values; the DSGVO cleanup extension it added was later dropped by migration 118; the retention-sweeper, ADR-0089, keeps Patreon tokens).
- `shared/premium/recompute.go`: `Effective(override, hasActiveSub)` truth table + transactional `Recomputer.Recompute`.
- `services/payment-service/`: `patreon/`, `repository/`, `entitlement/`, `handlers/` (connect/callback/status/disconnect/webhook), `reconcile/`, port 8091.
- share-service: `repository/premium_repo.go` writes the override and recomputes; gateway routes: public `POST /api/v1/webhooks/patreon` and `GET /api/v1/payment/patreon/callback`, JWT-protected connect/status/disconnect.
- Frontend: `frontend/src/app/settings/premium/page.tsx` + `lib/api/payment.ts`.
- Deploy: payment-service in `build-and-push.yml`; replication 1 (no HPA) with `patreon-client-id/secret` and `patreon-webhook-secret` in `allchat-secrets`; Patreon OAuth client + webhook registered in the Patreon portal.
- Tests: unit (status mapping, HMAC verify, parsers, `Effective` truth table); testcontainers integration (grant/revoke cycle, override-survives-lapse, signed synthetic webhook flipping a gated endpoint 403→200→403; disconnect then webhook leaves the account unlinked, `disconnect_test.go`). Live Patreon E2E intentionally out of scope.

Related: ADR-0008 (feature gates), ADR-0016 (token storage pattern), ADR-0019/0020/0027.
