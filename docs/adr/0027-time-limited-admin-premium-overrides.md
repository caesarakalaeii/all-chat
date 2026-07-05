# ADR-0027: Time-Limited Admin Premium Overrides

**Date**: 2026-07-05
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

An admin can grant premium to a streamer (`users.is_premium`) or a viewer (`viewers.is_premium`) via the tri-state `premium_admin_override` (`NULL`=follow subscription, `TRUE`=force-grant, `FALSE`=reserved deny), from which `is_premium` is materialized by `shared/premium.Recompute` / `RecomputeViewer` (ADR-0018, ADR-0019). Today that grant is **permanent until an admin revokes it**.

We want the admin to grant premium **for a limited amount of time** — e.g. a 7-day comp, a giveaway prize, a trial — for a user and/or a viewer, after which premium reverts on its own (to whatever the subscription warrants) with no second admin action.

The wrinkle: ADR-0018 deliberately kept the entitlement rule **time-free** ("`RecomputePremium` therefore stays time-free"), because a subscription's grace is Patreon's own signal (`active_patron` through its retry window), not a timer we keep. Introducing an admin-set expiry adds time to the *override* input. And because `is_premium` is **materialized**, an expiry that passes with no other write would leave the column stale (still `TRUE`).

## Decision Drivers

- **Reuse the derived-column model.** Don't fork a parallel entitlement path — extend the ADR-0018/0019 override + recompute so readers (`shared/middleware/premium.go`, moderation-service, `ViewerBadgeEnricher`, viewer JWT) stay untouched.
- **Clobber-free.** A lapse-driven revert must never clobber a fresh re-grant, and must not race the subscription writer (the exact hazard ADR-0018 eliminated).
- **Convergent + correct on any recompute.** Whatever triggers a recompute (webhook, reconcile, admin write, override lapse), the result must be correct.
- **One clock.** Expiry comparisons must not depend on per-pod wall-clock skew.
- **Minimal blast radius.** Prefer the smallest change that is correct.

## Considered Options

1. **Optional expiry column on the override; recompute nullifies an expired override in SQL; a single-replica periodic sweep clears lapsed grants.**
   - ✅ Tiny surface (two nullable columns); `Effective(override, active)` stays an unchanged pure boolean; correct on every recompute; converges even with no other write.
   - ❌ Adds a materialization-refresh sweep (accepted — payment-service already runs one single-replica loop).
2. **Compute `is_premium` on read instead of materializing.**
   - ✅ No sweep; always current.
   - ❌ Rewrites the whole read path (every reader, hot paths included) — exactly what ADR-0018/0019 chose *not* to do. Rejected.
3. **Store an absolute expiry computed by the admin's browser and compare to app wall-clock.**
   - ❌ Browser/app clock skew makes "grant for 7 days" imprecise and the compare ambiguous. Rejected in favour of a server-computed deadline (`NOW() + interval`) compared to DB `NOW()`.
4. **Run the sweep in each write service (share-service / auth-service).**
   - ❌ Both are multi-replica (2×): the periodic scan would run redundantly on every replica. payment-service is the single-replica entitlement authority — the natural home. Rejected.

## Decision Outcome

**Chosen**: Option 1.

- **Schema (migration 067).** Add nullable `premium_admin_override_expires_at TIMESTAMP` to `users` and `viewers`, each with a partial index `WHERE … IS NOT NULL`. `NULL` = permanent (unchanged behaviour). Idempotent, writes no values (re-run-safe per the runner's re-run-everything contract; guarded by `migrations_rerun_test.go`).
- **Recompute nullifies an expired override in SQL.** `recomputeUserTx` / `recomputeViewerTx` read
  ```sql
  CASE WHEN premium_admin_override_expires_at IS NOT NULL
            AND premium_admin_override_expires_at <= NOW()
       THEN NULL ELSE premium_admin_override END
  ```
  and feed that to the **unchanged** `Effective(override, hasActiveSub || …)`. So an expired grant is indistinguishable from "no admin opinion" and premium falls through to the subscription. `Effective` stays time-free and its truth table is untouched; the time lives in the read SQL against the **DB clock** (one clock). **Scope of the ADR-0018 amendment**: only the *admin-override* input gains an optional expiry — the *subscription* half stays time-free and still honors Patreon's own grace.
- **Server-computed deadline.** The admin endpoints take an optional positive `duration_seconds` (capped ~10y); the repository sets `premium_admin_override_expires_at = NOW() + make_interval(secs => …)`, so a grant lasts exactly the requested duration regardless of the browser clock. Grant with no duration ⇒ `NULL` (permanent); revoke ⇒ both cleared.
- **Single-replica expiry sweep (payment-service).** `reconcile.OverrideExpirySweeper` runs on its own short interval (`PAYMENT_OVERRIDE_SWEEP_INTERVAL`, default 5m) alongside the Patreon reconcile loop. It lists due rows (`… IS NOT NULL AND … <= NOW()`) and calls `Recomputer.ExpireUserOverrideIfDue` / `ExpireViewerOverrideIfDue`, each of which — in **one transaction** — runs a **guarded** clear (`UPDATE … SET override=NULL, expires_at=NULL WHERE … expires_at <= NOW()`) and, iff a row was cleared, recomputes `is_premium`. The `WHERE … <= NOW()` guard means a concurrent admin re-grant (fresh future expiry, or permanent) is **not clobbered**; the shared transaction means a crash can never strand a cleared-but-not-recomputed row.

Because the recompute already nullifies an expired override, `is_premium` is correct the instant *anything* recomputes a subject; the sweep is purely the backstop for a subject that has no other write after its grant lapses, plus the column cleanup that makes the row stop matching.

## Consequences

### Positive
- Admins can hand out temporary premium to users and viewers; it reverts on its own, clobber-free.
- Readers and the `Effective` rule are unchanged; permanent grants behave exactly as before.
- Correct on any recompute (webhook/reconcile/admin/lapse); converges with no other write via the sweep.
- One clock (DB `NOW()`); grant length is exact and browser-clock-independent.

### Negative
- `is_premium` can lag a lapse by up to one sweep interval (≤5m default) *only* for a subject with no other write in that window — bounded and tunable.
- One more single-replica loop in payment-service (cheap: an indexed scan + a few recomputes).
- A narrow, documented amendment to ADR-0018's "time-free" property — scoped to the admin-override input only.

## Implementation

- **Migration**: `migrations/067_premium_admin_override_expiry.sql` (+ `_down`).
- **Shared**: `shared/premium/recompute.go` — extracted `recomputeUserTx`/`recomputeViewerTx` (expired-override nullification), added `ExpireUserOverrideIfDue`/`ExpireViewerOverrideIfDue` (guarded atomic clear+recompute).
- **payment-service**: `reconcile/override_expiry.go` (`OverrideExpirySweeper`) + wiring in `cmd/main.go` (`PAYMENT_OVERRIDE_SWEEP_INTERVAL`, `PAYMENT_OVERRIDE_SWEEP_BATCH_SIZE`).
- **share-service**: `repository/premium_repo.go` `UpdateUserPremium(…, ttl)`, `handlers/admin.go` parses/validates `duration_seconds`.
- **auth-service**: `repository/viewer_repository.go` `SetViewerPremium(…, ttl)`, `handlers/admin_viewers.go` parses/validates `duration_seconds`; `premium_expires_at` surfaced in the admin user + viewer lists.
- **Gateway**: unchanged — the existing `POST /api/v1/admin/premium/users/:id` and `POST /api/v1/admin/viewers/:session_id/premium` routes only grow their request body.
- **Frontend**: `components/admin/PremiumDurationChooser.tsx` (presets + custom days, pure `customDaysToSeconds`), wired into `admin/users` and `admin/viewers` grant dialogs; expiry shown on the active-premium panels.
- **Tests**: unit (`customDaysToSeconds`; handler duration validation) + testcontainers integration (recompute expiry truth table, `ExpireXOverrideIfDue` guard, end-to-end sweep flips `is_premium` and clears columns).

## Related Decisions

- [ADR-0018](./0018-premium-entitlements-via-patreon.md) — the user-premium derived column + `Effective` this extends (and whose "time-free" property this narrowly amends).
- [ADR-0019](./0019-split-streamer-viewer-premium.md) — the viewer-premium derived column + `RecomputeViewer` this extends.
- [ADR-0020](./0020-beta-tester-role.md) — beta-tester role; intentionally **not** time-limited (a separate early-access role).
