# ADR-0019: Split Streamer vs Viewer Premium via a Polymorphic Patreon Subject

**Date**: 2026-06-20
**Status**: Accepted
**Revised**: 2026-11-18 (viewer routes read premium from the database, not the JWT claim; admin grant keyed on the viewer)
**Deciders**: caesarakalaeii

## Context

Two audiences sit under one "premium" concept: `users.is_premium` (streamer/overlay-owner features, derived from Patreon plus admin override per ADR-0018) and `viewers.is_premium` (cosmetic badge, read on the message hot path by `ViewerBadgeEnricher` and into the viewer JWT; set by admin or inherited from a premium streamer account at viewer login). The goal is a cheaper, separately-priced viewer subscription granting only viewer perks. The hard part is identity: streamer premium is keyed by a `users` account that OAuth-links Patreon; a viewer is a platform identity (`viewer_platform_identities`) usually without a `users` row, so a pledge has no obvious subject.

Alternatives rejected: inheritance-only (a pure viewer could never buy anything, which fails the product goal); splitting write ownership between services (reintroduces clobbering); a parallel viewer-only payment stack (two webhooks/reconciles/code paths to keep convergent).

## Decision

Generalize the ADR-0018 pipeline: a polymorphic premium subject (`user` | `viewer`) with a tier-driven `product`.

- **One Patreon account, one identity**: a connection anchors to exactly one subject (`user` XOR `viewer`), enforced by the existing unique constraints plus CHECKs. A single human who is both streamer and distinct viewer can only link one side; the other is covered by inheritance where applicable (documented limitation).
- **Product follows the connection subject, with per-product thresholds**: a viewer connection is active iff `active_patron` and `cents >= PATREON_VIEWER_MIN_TIER_CENTS` (cheaper); a user connection iff `cents >= PATREON_MIN_TIER_CENTS`. Reuses `SubscriptionStatusFor(snap, threshold)` unchanged; no explicit tier-id map (cents are simpler and already returned).
- **Viewer connect lives in payment-service, viewer-JWT authenticated** (`shared/auth.ValidateViewerJWTWithKeyChain`): `GET /api/v1/payment/viewer/patreon/connect`, `GET .../status`, `DELETE .../connection`. The OAuth callback and webhook stay single endpoints; the callback's Redis state carries the subject and the webhook resolves it from the polymorphic token row, which every subscription write also reads. A viewer disconnect deletes the viewer's membership rows (ADR-0018 connection rule).
- **`viewers.is_premium` becomes a single-writer derived column**:
  `Effective(viewers.premium_admin_override, hasActiveViewerSubscription OR linkedStreamerIsPremium)`
  reusing `shared/premium.Effective`. Inheritance survives as the `linkedStreamerIsPremium` term (a premium `users` row joined via `viewer_sessions.user_id`), so nothing changes for readers; `LinkViewerToUser` and the admin path stop writing the boolean and instead set their input + recompute, fixing the staleness bug where inherited premium was never revoked on streamer lapse.
- **Read path unchanged**: `ViewerBadgeEnricher` untouched. The viewer JWT's `is_premium` claim is only a sign-in snapshot, so `FreshViewerPremium` middleware replaces it with the current `viewers.is_premium` on every `/viewer/*` request and `GET /viewer/me` returns the current value: admin grants, subscriptions and lapses take effect immediately. Admin grants target the `viewers` row (`POST /admin/viewers/identities/:viewer_id/premium`, ADR-0034, with ADR-0027's optional override expiry).

Schema (migration 064, no edits to 063; idempotent, writes no entitlement values): `premium_subscriptions` gains `product` and `viewer_id`; `patreon_oauth_tokens` drops its inline `user_id` uniqueness for partial unique indexes on either subject; `viewers` gains a tri-state `premium_admin_override`.

## Consequences

Pure viewers self-serve a cheaper subscription; one payment stack serves both products; `viewers.is_premium` gains the same convergent single-writer guarantees as `users.is_premium`; both read paths untouched. Costs: the one-Patreon-account-per-identity limitation; inheritance revocation is eventually convergent (next viewer activity or reconcile pass, not instant); a small new viewer-authenticated inbound surface inside payment-service.

## Implementation

`migrations/064_viewer_premium_product.sql`; `RecomputeViewer` in `shared/premium/recompute.go`; subject-aware `entitlement.Apply` (`Subject{UserID, ViewerID, Product}`), repository and reconcile in payment-service; `cmd/main.go` adds `PATREON_VIEWER_MIN_TIER_CENTS` + viewer routes; auth-service routes `LinkViewerToUser`/`SetViewerPremium` through `RecomputeViewer`; frontend `frontend/src/app/settings/viewer/premium/page.tsx` via `lib/api/viewer.ts`. Tests mirror ADR-0018: viewer webhook flips the flag, override survives lapse, inheritance preserved, exactly-one-subject CHECK, migration rerun idempotency.

Related: ADR-0018 (pipeline this generalizes), ADR-0008, ADR-0016.
