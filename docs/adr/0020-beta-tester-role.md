# ADR-0020: Beta-Tester Role + Early-Access Feature Gates

**Date**: 2026-06-20
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Before paid monetization (ADR-0018), a handful (~5) of users held `users.is_premium` as a manual admin grant. Now that premium is a paid product, we want to **thank those early users** by moving them to a standing **beta-tester** status that grants *all premium features plus early-access ones* (ROADMAP Phase 17).

Two things are missing today:

1. **A role above premium.** `users.is_premium` (ADR-0018, derived from admin override + Patreon subscription) and the `feature_gates` layer (ADR-0008, `is_premium` per key) have no notion of "premium **and** early access". `admin` is the only role above a normal user, and it is far too broad.
2. **A safe way to grandfather the ~5 users.** A blanket `UPDATE users SET … WHERE is_premium = TRUE` data migration would be **re-applied on every pod start** by the re-running migration runner (the 009-incident class of bug) and would also sweep in users who became premium *by paying*, which is not the intent.

## Decision Drivers

- **Reuse the proven patterns.** ADR-0018's derived-column `Recompute` and ADR-0008's `feature_gates` + `shared/middleware` gate are exactly the shapes this needs; don't invent a new authorization subsystem.
- **Fresh enforcement.** Granting the role from the admin dashboard must take effect immediately, with no "log out and back in" step.
- **Manual, idempotent grandfathering.** No entitlement-writing data migration (the runner re-runs everything); grandfathering is an explicit, per-user admin action.
- **Least new surface, symmetric with premium.** A beta-tester is "premium-plus"; model it next to premium, not as a parallel universe.

## Considered Options

1. **`users.is_beta_tester` boolean, folded into the `is_premium` derivation, + a DB-backed early-access gate.** Beta-tester is an independent admin-granted flag; `shared/premium.Recompute` ORs it into the premium-granting input (so a beta-tester *is* premium); a new `feature_gates.early_access` dimension is enforced by `RequireEarlyAccess`, which reads `is_beta_tester` from the DB exactly as `RequirePremium` reads `is_premium`.
   - ✅ Reuses both proven patterns; beta-testers transparently pass every existing `RequirePremium` gate; enforcement is fresh on grant (DB read, no token round-trip); symmetric with premium; tiny surface.
   - ❌ Two reads per gated early-access request (same cost model as `RequirePremium`).

2. **Surface `beta_tester` into the JWT `claims.Roles` and gate on the role claim** (the ROADMAP Phase 17 sketch, mirroring `admin`/`AdminOnly`).
   - ✅ No per-request DB read; mirrors the `admin` role mechanism.
   - ❌ **Stale until re-login** — a freshly-granted beta-tester would get 403 on early-access features until their JWT is reissued (up to the token lifetime), an asymmetry with the immediate premium effect. Also churns all four JWT generators (`GenerateJWT`/`GenerateToken`/`*WithKid`) and ~6 issuance call sites. The one existing *entitlement* gate (`RequirePremium`) is DB-backed, not JWT-backed; an early-access gate is an entitlement gate, so it should match.

3. **A generalized roles table (`user_roles`).**
   - ✅ Scales to many roles.
   - ❌ Over-engineered for a single new role; a migration + join + cache for one boolean. Revisit if a third role appears.

4. **Auto data-migration to grandfather** (`UPDATE … WHERE is_premium = TRUE`).
   - ❌ Re-applied every pod start (009-incident class); also conflates paid premium users with the pre-monetization cohort. Rejected outright (a Decision Driver).

## Decision Outcome

**Chosen**: **Option 1** — a `users.is_beta_tester` flag folded into the `is_premium` derivation, plus a `feature_gates.early_access` dimension enforced by a DB-backed `RequireEarlyAccess`. Grandfathering is **manual**, via an admin "Grant Beta Tester" button (Option 4 explicitly rejected).

**Rationale**: It reuses both established patterns (driver 1), enforces freshly on grant (driver 2), keeps grandfathering a per-user idempotent admin action with no data migration (driver 3), and models beta-tester as "premium-plus" right next to premium (driver 4). Option 2 (the ROADMAP sketch) was rejected specifically because a JWT-role gate is stale-until-relogin and inconsistent with the DB-backed `RequirePremium` precedent.

### Key sub-decisions

- **A beta-tester is premium, by derivation — not by a separate grant.** `shared/premium.Recompute` becomes:
  ```
  users.is_premium = Effective(
      users.premium_admin_override,                 -- tri-state, unchanged (ADR-0018)
      hasActiveSubscription OR users.is_beta_tester
  )
  ```
  `Effective` is unchanged (its second argument is "any non-override premium reason"). So granting beta-tester force-grants premium the same clobber-free way a comp does; an admin **force-deny** (`premium_admin_override = FALSE`) still beats it. Every existing `RequirePremium` gate therefore admits beta-testers with no gate change.
- **`early_access` is a new, orthogonal `feature_gates` dimension.** `feature_gates` gains `early_access BOOLEAN NOT NULL DEFAULT FALSE`. A gate may require premium, early-access, both, or neither. `RequireEarlyAccess(db, gates, key)` mirrors `RequirePremium`: auth → if `!gates.IsEarlyAccess(key)` allow (deferring to any premium gate) → else require `users.is_beta_tester`, 403 otherwise. The `FeatureGateCache` (ADR-0008) caches both flags; unknown keys fail **closed** (early-access-required), mirroring `IsPremium`. An admin "graduates" an early-access feature by flipping `early_access` off, exactly like graduating a premium gate to free.
- **Enforcement reads the DB, not the JWT.** `is_beta_tester` is read at gate time (and folded into the materialized `is_premium`), so an admin grant takes effect immediately. `beta_tester` is deliberately **not** added to `claims.Roles`; the frontend learns the status from the `users` API object (`is_beta_tester`, like `is_admin`/`is_premium`), not by decoding the token. (If a future need arises for a stateless cross-service role check, surfacing it into the JWT can be added then.)
- **The admin button is the only grandfathering mechanism.** `share-service` gains `POST /api/v1/admin/beta-tester/users/:id` → `PremiumRepository.SetUserBetaTester` (writes `is_beta_tester`, then `Recompute`), mirroring `UpdateUserPremium`/`SetUserPremium`. The frontend admin users page gains a "Grant/Revoke Beta Tester" control mirroring the premium toggle. The ~5 users are moved by hand; **no data migration writes `is_beta_tester` values**.
- **Migration 065 (new; edits no prior migration).** Idempotent ALTERs that write **no** entitlement values: `users.is_beta_tester BOOLEAN NOT NULL DEFAULT FALSE` (+ partial index) and `feature_gates.early_access BOOLEAN NOT NULL DEFAULT FALSE`. A re-run can never clobber a live grant, per the runner's re-run-everything contract.

## Consequences

### Positive
- Beta-testers get the entire premium feature set (via the derivation) **plus** early-access features, with one flag.
- Reuses ADR-0018 (`Recompute`/`Effective`) and ADR-0008 (`feature_gates` + gate middleware + cache) with no new authorization subsystem; both read paths and all existing gates are untouched.
- Grant/revoke is fresh (DB-backed) and convergent/idempotent; revoking simply clears the flag and premium reverts to following the subscription/override on the next recompute.
- Grandfathering carries no data-migration risk and never sweeps in paid premium users.

### Negative
- An early-access-gated request does one extra DB read (same cost profile as `RequirePremium`).
- `beta_tester` is not in the JWT, so a hypothetical future stateless cross-service role check would need it added then (acceptable; no current need).
- A user who is both `is_beta_tester = TRUE` and `premium_admin_override = FALSE` is *not* premium (override wins) yet still passes early-access gates — a contrived admin combination, documented; resolve by clearing one flag.

## Implementation

- **Migration**: `migrations/065_beta_tester_role.sql` (+ `_down`).
- **Shared**: `shared/premium/recompute.go` — `Recompute` ORs `is_beta_tester` into the premium input (`Effective` unchanged). `shared/featuregates/cache.go` — cache both flags, add `IsEarlyAccess`. `shared/middleware/early_access.go` — `RequireEarlyAccess` (+ injectable-querier test variant); `GateChecker` gains `IsEarlyAccess`.
- **share-service**: `PremiumRepository.SetUserBetaTester` (write + recompute); `AdminHandler.SetUserBetaTester` + `/admin/beta-tester/users/:id` route; `admin_featuregates` ListGates/UpdateGate manage `early_access`.
- **auth-service**: `models.User.IsBetaTester` + `is_beta_tester` in the user-load queries/scan and the admin user-list response (so the frontend + admin UI see it).
- **api-gateway**: `service_config.go` + admin route for `/api/v1/admin/beta-tester/...` → share-service.
- **Frontend**: `User.is_beta_tester` type; admin users page "Grant/Revoke Beta Tester" control + BETA badge; admin features page `early_access` toggle.
- **Tests**: unit (`RequireEarlyAccess` truth table; `IsEarlyAccess` cache; admin handler + the premium not-found fix; early_access gate-update); testcontainers E2E (beta-tester grants premium via `Recompute`; admin force-deny beats it; the real `RequireEarlyAccess` gate admits a beta-tester and denies others; graduation opens it).

## Related Decisions

- [ADR-0018](./0018-premium-entitlements-via-patreon.md) — the derived `is_premium` / `Recompute` / `Effective` model this folds beta-tester into.
- [ADR-0019](./0019-split-streamer-viewer-premium.md) — the viewer-premium split; the admin tri-state override + `RecomputeViewer` template this mirrors.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the `feature_gates` + middleware + cache layer extended with the `early_access` dimension.
