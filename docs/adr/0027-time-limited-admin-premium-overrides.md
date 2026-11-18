# ADR-0027: Time-Limited Admin Premium Overrides

**Date**: 2026-07-05
**Status**: Accepted
**Revised**: 2026-11-18 (brought in line with ADR-0034's durable-viewer premium route)
**Deciders**: caesarakalaeii

## Context

Admins grant premium via the tri-state `premium_admin_override` (`NULL` = follow subscription, `TRUE` = force-grant, `FALSE` = force-deny) on `users` (streamers, ADR-0018) and `viewers` (ADR-0019), materialized by `shared/premium.Recompute` / `RecomputeViewer`. That grant was permanent until revoked. Streamers occasionally need a comp: a 7-day grant, a giveaway prize, a trial — premium that reverts on its own.

The wrinkle: ADR-0018 deliberately kept the entitlement rule time-free (a subscription's grace is Patreon's own signal, not a timer we keep), and `is_premium` is *materialized* — an expiry that passes with no other write would leave the column stale.

## Decision

An optional server-computed expiry on the admin override only; the subscription half stays time-free.

- **Schema (migration 067).** Nullable `premium_admin_override_expires_at` on `users` and `viewers`, partial-indexed. `NULL` = permanent (unchanged behaviour). Idempotent, writes no values.
- **Recompute nullifies an expired override in SQL.** `recomputeUserTx` / `recomputeViewerTx` read the override as `NULL` when `expires_at <= NOW()` and feed that to the unchanged `Effective(override, active)`. An expired grant is indistinguishable from "no admin opinion"; premium falls through to the subscription. `Effective` stays time-free; the time lives in the read SQL against the DB clock (one clock).
- **Server-computed deadline.** The admin endpoints take an optional positive `duration_seconds` (capped ~10y); the repository sets `expires_at = NOW() + interval`, so the grant length is exact regardless of the browser clock. No duration ⇒ permanent; revoke clears both columns.
- **Single-replica expiry sweep in payment-service.** `reconcile.OverrideExpirySweeper` runs on `PAYMENT_OVERRIDE_SWEEP_INTERVAL` (default 5m), lists due rows and calls `ExpireUserOverrideIfDue` / `ExpireViewerOverrideIfDue`, each of which clears a lapsed row and recomputes `is_premium` in one transaction. The `WHERE expires_at <= NOW()` guard means a concurrent admin re-grant is never clobbered. payment-service is the natural home because it is single-replica; the other candidates are 2× replicated.

Because the recompute already nullifies an expired override, `is_premium` is correct whenever anything recomputes the subject; the sweep is the backstop for a subject with no other write after lapse, plus the column cleanup.

Alternatives considered: computing `is_premium` on read instead of materializing (rewrites the whole read path — what ADR-0018/0019 chose against); storing a browser-computed absolute timestamp (clock skew); running the sweep in share-service/auth-service (multi-replica redundancy).

## Consequences

- Temporary premium reverts on its own, clobber-free; readers and the `Effective` rule are unchanged; permanent grants behave exactly as before.
- `is_premium` can lag a lapse by up to one sweep interval for a subject with no other write in that window.
- A narrow amendment to ADR-0018's "time-free" property, scoped to the admin-override input only.

## Implementation

- Migration: `migrations/067_premium_admin_override_expiry.sql` (+ `_down`).
- Shared: `shared/premium/recompute.go` (expired-override nullification, `ExpireUserOverrideIfDue` / `ExpireViewerOverrideIfDue`).
- payment-service: `reconcile/override_expiry.go` + wiring (`PAYMENT_OVERRIDE_SWEEP_INTERVAL`, `PAYMENT_OVERRIDE_SWEEP_BATCH_SIZE`).
- share-service: `repository/premium_repo.go` `UpdateUserPremium(…, ttl)`; `handlers/admin.go` validates `duration_seconds`.
- auth-service: `repository/viewer_repository.go` `SetViewerPremium(…, ttl)`; `handlers/admin_viewers.go`; `premium_expires_at` in the admin lists.
- Gateway: existing `POST /api/v1/admin/premium/users/:id` and (since ADR-0034) `POST /api/v1/admin/viewers/identities/:viewer_id/premium` routes grew only their request body.
- Frontend: `components/admin/PremiumDurationChooser.tsx` wired into the admin users and viewers grant dialogs; expiry shown on the active-premium panels.

## Related

- [ADR-0018](./0018-premium-entitlements-via-patreon.md) / [ADR-0019](./0019-split-streamer-viewer-premium.md) — the derived columns this extends.
- [ADR-0020](./0020-beta-tester-role.md) — beta-tester; intentionally not time-limited.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — the viewer premium route is keyed on the durable viewer.
