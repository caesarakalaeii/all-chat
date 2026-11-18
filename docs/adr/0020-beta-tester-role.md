# ADR-0020: Beta-Tester Role + Early-Access Feature Gates

**Date**: 2026-06-20
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

After premium became a paid product (ADR-0018), a handful of pre-monetization users still held premium as an admin grant. They should be thanked with a standing beta-tester status: all premium features plus early-access ones. Two things were missing:

1. No role above premium exists. `users.is_premium` and the `feature_gates` layer (ADR-0008) have no notion of "premium plus early access"; `admin` is far too broad.
2. Grandfathering must be manual. A blanket data migration would be re-applied on every pod start (the runner re-runs everything) and would sweep in paying users.

## Decision

A `users.is_beta_tester` flag folded into the `is_premium` derivation, plus an `early_access` dimension on `feature_gates` enforced by a new DB-backed `RequireEarlyAccess`. Grandfathering is a per-user admin action; no data migration writes `is_beta_tester`.

Alternatives considered:

- **Surface `beta_tester` into the JWT** (the original roadmap sketch): stale until re-login, inconsistent with the DB-backed `RequirePremium` precedent. Rejected.
- **Generalized `user_roles` table**: over-engineering for one flag. Revisit if another role appears (ADR-0041 later accepted a third role and still used a boolean).
- **Data migration to grandfather**: re-run hazard and conflates paid users with the cohort. Rejected.

### How it works

- `shared/premium.Recompute` ORs `is_beta_tester` into the premium input; `Effective` is unchanged, so an admin force-deny (`premium_admin_override = FALSE`) still wins and every existing `RequirePremium` gate admits beta-testers with no gate change.
- `feature_gates` gains `early_access BOOLEAN NOT NULL DEFAULT FALSE`. `RequireEarlyAccess(db, gates, key)` mirrors `RequirePremium`: if the gate is not early-access, defer to any premium gate; otherwise require the user to hold early-access eligibility, 403 otherwise. Since ADR-0041 that eligibility is `is_beta_tester OR is_ambassador`. The `FeatureGateCache` caches both flags; unknown keys fail closed.
- Enforcement reads the DB, not the JWT, so a grant takes effect immediately. The frontend learns the status from the `users` API object (`is_beta_tester`), not by decoding the token.
- The only grandfathering mechanism is the admin button: `POST /api/v1/admin/beta-tester/users/:id` → `PremiumRepository.SetUserBetaTester` (writes the flag, then recomputes), mirroring the premium toggle.
- Migration 065 is idempotent ALTERs that write no values: `users.is_beta_tester` and `feature_gates.early_access`, plus partial indexes.

## Consequences

- One flag delivers the whole premium set plus early access; revoking simply clears the flag.
- An early-access-gated request costs one extra DB read, the same cost model as `RequirePremium`.
- A user who is both `is_beta_tester = TRUE` and `premium_admin_override = FALSE` is not premium (override wins) but still passes early-access gates. Contrived admin combination; resolve by clearing one flag.
- `beta_tester` is not in the JWT; a future stateless cross-service role check would need it added then.

## Implementation

- Migration: `migrations/065_beta_tester_role.sql` (+ `_down`).
- Shared: `shared/premium/recompute.go`, `shared/featuregates/cache.go` (`IsEarlyAccess`), `shared/middleware/early_access.go` (`RequireEarlyAccess`).
- share-service: `PremiumRepository.SetUserBetaTester`, `AdminHandler.SetUserBetaTester` + route, `admin_featuregates` manages `early_access`.
- auth-service: `models.User.IsBetaTester` in user-load queries and the admin user list.
- api-gateway: `/api/v1/admin/beta-tester/...` route to share-service.
- Frontend: admin users page grant/revoke control + BETA badge; admin features page `early_access` toggle.

## Related

- [ADR-0018](./0018-premium-entitlements-via-patreon.md) and [ADR-0019](./0019-split-streamer-viewer-premium.md) — the derived-premium model this folds into.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the gate layer extended with `early_access`.
- [ADR-0041](./0041-ambassador-role.md) — second role in the same pattern.
