# ADR-0019: Split Streamer vs Viewer Premium via a Polymorphic Patreon Subject

**Date**: 2026-06-20
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

All-Chat conflates two audiences under one "premium" concept (see ADR-0018 Future Work, ROADMAP Phase 16):

- `users.is_premium` gates **streamer / overlay-owner** features (sharing, moderation, TTS, stream-selection) via `shared/middleware/premium.go` + `feature_gates` (ADR-0008). It is now derived from a Patreon subscription + admin override (ADR-0018), written by `payment-service` / `share-service` via `shared/premium.Recompute`.
- `viewers.is_premium` is a **viewer cosmetic** flag (premium chat badge), read on the message hot path by `message-processor`'s `ViewerBadgeEnricher` and into the viewer JWT. Today it is only set by an admin (`SetViewerPremium`) or **inherited** at viewer-login when the viewer's platform identity matches a premium streamer `users` account (`LinkViewerToUser`).

We want a **cheaper, separately-priced viewer subscription** that grants only viewer-facing perks, while streamer premium keeps the richer set. The hard part is **identity**: streamer premium is keyed by an all-chat `users` account that OAuth-links Patreon (`patreon_oauth_tokens.user_id`), but a viewer is a *platform identity* (`viewer_platform_identities`: platform + platform_user_id) — usually **not** a `users` account. So: how does a Patreon pledge grant *viewer* premium to someone who has no `users` row?

## Decision Drivers

- **Reuse ADR-0018.** One `payment-service`, one Patreon campaign, one webhook endpoint, one reconcile loop, one `Effective` entitlement rule. Don't fork a parallel payment stack.
- **Serve pure viewers.** The target buyer is a viewer who is *not* a streamer; the design must grant them premium without requiring a dashboard/`users` account.
- **Don't touch the read path** (ADR-0018 principle). `ViewerBadgeEnricher` and the viewer JWT keep reading `viewers.is_premium`; we only change *who writes it and how*.
- **Convergent / clobber-free.** `viewers.is_premium` has up to three inputs (admin, viewer subscription, inherited streamer premium). They must not clobber each other — the ADR-0018 derived-column model already solved this shape for `users.is_premium`.
- **Least new surface.** A viewer-facing inbound payment path is security-sensitive; keep it inside the already-isolated `payment-service`.

## Considered Options

1. **Viewer-side "connect Patreon" + polymorphic subject** — generalize the ADR-0018 pipeline so a Patreon connection/subscription is anchored to *either* a `users.id` (streamer) *or* a `viewers.id` (viewer), with a `product` dimension; `viewers.is_premium` becomes a derived column via a new `RecomputeViewer`.
   - ✅ Serves pure viewers; reuses the whole payment stack (OAuth client, webhook verify, status mapping, reconcile, `Effective`); one webhook resolves to either subject; mirrors the proven `users` model exactly.
   - ❌ Touches the freshly-built (committed, undeployed) ADR-0018 tables (via a new migration); adds a viewer-JWT-authenticated connect surface to `payment-service`.

2. **Only viewers linked to a `users` account inherit it** — no viewer-side purchase; viewer premium is a perk of a Patreon-backed streamer account.
   - ✅ Zero new payment surface; inheritance already exists.
   - ❌ **Fails the product goal** — pure viewers (the buyer) have no `users` account, so they could never buy it. This is essentially today's behavior.

3. **Split write ownership** (payment-service writes `viewers.is_premium` for linked viewers, auth-service for the rest) — a who-writes tweak.
   - ✅ Small.
   - ❌ Doesn't answer the identity question (how a pure viewer pledges); reintroduces the multi-writer clobber problem ADR-0018 eliminated.

4. **Parallel viewer-only payment tables/service** — duplicate `patreon_oauth_tokens` / `premium_subscriptions` / reconcile for viewers.
   - ✅ Leaves ADR-0018 tables untouched.
   - ❌ Two webhooks or a fan-out from one; duplicated reconcile + status mapping; two code paths to keep convergent. Higher long-run cost than a polymorphic subject.

## Decision Outcome

**Chosen**: Option 1 — **a polymorphic premium subject (`user` | `viewer`) with a tier-driven `product`**, reusing ADR-0018 end to end.

**Rationale**: It is the only option that serves pure viewers (driver 2) while reusing one payment stack (driver 1); it extends the exact derived-column model that already made `users.is_premium` convergent and clobber-free (driver 4) and leaves both readers untouched (driver 3).

### Key sub-decisions

- **One Patreon account ↔ one all-chat identity.** A connection is anchored to *exactly one* subject (`user` XOR `viewer`). The existing `UNIQUE(patreon_user_id)` (token) and `UNIQUE(provider, provider_user_id)` (subscription) are kept and now mean "one connection per Patreon account". A single human who is *both* a streamer and a distinct viewer can link Patreon to only one; the other side is covered by the existing inheritance term where applicable. (Documented limitation.)
- **Product = connection subject, gated by a per-product cents threshold.** A viewer connection ⇒ `product='viewer'`, active iff `patron_status=active_patron AND cents >= PATREON_VIEWER_MIN_TIER_CENTS` (cheaper). A user connection ⇒ `product='streamer'`, active iff `cents >= PATREON_MIN_TIER_CENTS` (unchanged). This realizes ROADMAP's "cheaper tier → viewer" as two config thresholds, reusing `SubscriptionStatusFor(snap, threshold)` unchanged. (An explicit tier-id→product map is deferred — cents thresholds are simpler and Patreon already returns entitled cents.)
- **Viewer "Connect Patreon" lives in `payment-service`, authenticated by a viewer JWT.** Reuses `shared/auth.ValidateViewerJWTWithKeyChain` (same signing keychain, `ViewerClaims.ViewerID`). Keeps the inbound payment surface isolated per ADR-0018 rather than bolting Patreon OAuth onto auth-service. New routes: `GET /api/v1/payment/viewer/patreon/connect`, `GET /api/v1/payment/viewer/status`, `DELETE /api/v1/payment/viewer/patreon/connection`. The OAuth **callback** and the **webhook** stay single endpoints; the callback's Redis state carries the subject, the webhook resolves the subject from the polymorphic token row.
- **`viewers.is_premium` becomes a single-writer derived column via `RecomputeViewer`:**
  ```
  viewers.is_premium = Effective(
      viewers.premium_admin_override,                  -- tri-state, mirrors users
      hasActiveViewerSubscription OR linkedStreamerIsPremium
  )
  ```
  `linkedStreamerIsPremium` = a `users` row joined through `viewer_sessions.user_id` that `is_premium` — this **preserves the inheritance the enricher/JWT already depend on** and makes the column clobber-free. Reuses `shared/premium.Effective` (same truth table). Both `LinkViewerToUser` and the admin path stop writing `viewers.is_premium` directly and instead set their input + call `RecomputeViewer`, so there is exactly **one** writer (fixes today's bug where inherited premium is never revoked on streamer lapse — it now converges on any recompute).
- **Migration 064 (new; does not edit 063).** Idempotent ALTERs that write **no** entitlement values (so re-runs can't clobber a live grant, per the runner's re-run-everything contract):
  - `premium_subscriptions`: add `product VARCHAR(16) NOT NULL DEFAULT 'streamer'` (CHECK in `'streamer','viewer'`), add `viewer_id UUID REFERENCES viewers(id) ON DELETE CASCADE`, CHECK not-both-subjects; index `(viewer_id)`.
  - `patreon_oauth_tokens`: `ALTER user_id DROP NOT NULL`; add `viewer_id UUID REFERENCES viewers(id) ON DELETE CASCADE`; drop the inline `UNIQUE(user_id)` → two partial unique indexes (`user_id WHERE NOT NULL`, `viewer_id WHERE NOT NULL`); keep `UNIQUE(patreon_user_id)`; CHECK exactly-one-subject.
  - `viewers`: add `premium_admin_override BOOLEAN DEFAULT NULL` (tri-state mirror).
- **Read path unchanged.** `ViewerBadgeEnricher` and `generateViewerJWT` keep reading `viewers.is_premium`; no enricher change.

## Consequences

### Positive
- Pure viewers can self-serve a cheaper viewer subscription; streamer premium is unchanged.
- One payment-service / webhook / reconcile / `Effective` rule serves both products.
- `viewers.is_premium` gains the same convergent, single-writer, clobber-free guarantees as `users.is_premium`, and fixes the current "inherited premium never revoked" staleness.
- Both premium read paths are untouched (enricher hot path, viewer JWT).

### Negative
- One Patreon account grants premium to a single all-chat identity (documented limitation).
- Modifies the just-built, not-yet-deployed ADR-0018 tables via migration 064 (acceptable: 063 is not in production, and a forward migration is the auditable path — 063 is not edited).
- A new viewer-JWT-authenticated inbound surface inside payment-service (small; reuses existing verify + isolation).
- Streamer→viewer inheritance revocation is *eventually* convergent (on next viewer activity / reconcile), not instant, unless a cascade from the user recompute is added (deferred; no worse than today).

## Implementation

- **Migration**: `migrations/064_viewer_premium_product.sql` (+ `_down`).
- **Shared**: `shared/premium/recompute.go` — add `RecomputeViewer(viewerID)` (reuses `Effective`); keep `Recompute` for users.
- **payment-service**: subject-aware `entitlement.Apply` (`Subject{UserID, ViewerID, Product}`, per-product threshold); `repository` token+subscription carry `viewer_id`/`product`, `GetByPatreonUserID` returns the subject, `ListAll` returns subjects; viewer connect/status/disconnect handlers; subject in OAuth-state; reconcile iterates both subjects; `cmd/main.go` adds `PATREON_VIEWER_MIN_TIER_CENTS` + viewer routes + viewer-JWT middleware.
- **auth-service**: `LinkViewerToUser` and `SetViewerPremium` route through `RecomputeViewer`; admin writes `viewers.premium_admin_override` (tri-state), mirroring share-service's `UpdateUserPremium`.
- **Gateway**: `service_config.go` already routes `/api/v1/payment/*`; add the viewer connect routes (public callback already covered).
- **Frontend**: a viewer-facing subscribe/pricing page (viewer-authenticated) mirroring `settings/premium`, + `lib/api/payment.ts` viewer methods; clarify per-product entitlements.
- **Config**: `PATREON_VIEWER_MIN_TIER_CENTS` (configmap + `.env.example`).
- **Tests**: unit (`Effective` reused; product/threshold mapping; subject resolution); testcontainers E2E mirroring ADR-0018 (viewer webhook flips `viewers.is_premium`; admin-override survives lapse; inheritance preserved; one-subject CHECK); migration 064 re-run idempotency.

## Related Decisions

- [ADR-0018](./0018-premium-entitlements-via-patreon.md) — the user-premium pipeline this generalizes (subject, `Effective`, webhook, reconcile).
- [ADR-0008](./0008-feature-gate-infrastructure.md) — premium feature-gate layer (streamer side).
- [ADR-0016](./0016-linked-twitch-credentials.md) — the encrypted per-subject OAuth-token pattern reused for the viewer connection.
- ROADMAP Phase 16 (this) and Phase 17 (beta-tester role, builds on the viewer override + `RecomputeViewer`).
