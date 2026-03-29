---
phase: 07-feature-gate-infrastructure
plan: 01
subsystem: database
tags: [postgres, redis, pubsub, feature-gates, premium, go, miniredis, tdd]

requires: []

provides:
  - "feature_gates PostgreSQL table (migrations/044_feature_gates.sql) with sharing seed row"
  - "FeatureGateCache Go package (services/share-service/featuregates/) with IsPremium, Start, reload, GetAll"
  - "9 unit tests covering safe default, Pub/Sub invalidation, periodic refresh"
  - "ADR-0008 documenting DB + in-memory cache feature gate design"

affects:
  - "07-02: RequirePremium middleware rewrite depends on FeatureGateCache.IsPremium"
  - "07-03: Admin endpoints use FeatureGateCache.GetAll and trigger Pub/Sub invalidation"
  - "07-04: Admin UI reads gate state via admin endpoints built on this cache"

tech-stack:
  added:
    - "github.com/alicebob/miniredis/v2 v2.37.0 (share-service test dependency)"
  patterns:
    - "FeatureGateCache pattern: in-memory map + sync.RWMutex + Pub/Sub goroutine + ticker refresh"
    - "Safe default: IsPremium returns true for unknown keys (deny by default)"
    - "onReload test hook: allows verifying reload fires without real DB"
    - "NewFeatureGateCacheForTest / NewFeatureGateCacheForTestWithInterval: test constructors for Pub/Sub and ticker tests"

key-files:
  created:
    - "migrations/044_feature_gates.sql"
    - "services/share-service/featuregates/cache.go"
    - "services/share-service/featuregates/cache_test.go"
    - "docs/adr/0008-feature-gate-infrastructure.md"
  modified:
    - "docs/adr/README.md"
    - "services/share-service/go.mod"
    - "services/share-service/go.sum"

key-decisions:
  - "Cache lives in share-service/featuregates/, not shared/ — avoids premature abstraction; move when second service needs it"
  - "Unknown gate keys return true (premium required) — safe default per D-10 pitfall 2"
  - "onReload test hook injected via test constructors — allows Pub/Sub and ticker verification without real DB"
  - "refreshIntervalOverride field allows fast periodic-refresh tests (50ms interval)"
  - "miniredis v2.37.0 added as direct dependency for Redis Pub/Sub testing"
  - "NewFeatureGateCacheWithGates for DB-free IsPremium unit tests; NewFeatureGateCacheForTest for goroutine-level tests"

patterns-established:
  - "FeatureGateCache.run() follows lifecycle_subscriber.go pattern: select on ctx.Done/ticker.C/pubsub.Channel"
  - "Test constructors (ForTest, ForTestWithInterval) inject hooks without exposing internal fields"

requirements-completed: [D-01, D-02, D-03, D-04, D-07, D-08, D-09, D-10]

duration: 4min
completed: 2026-03-29
---

# Phase 07 Plan 01: Feature Gate Infrastructure — Cache Layer Summary

**feature_gates Postgres table with FeatureGateCache (Pub/Sub + 60s TTL) and ADR-0008, all tests green**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-29T13:27:45Z
- **Completed:** 2026-03-29T13:31:32Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Created migration 044 with `feature_gates` DDL, `update_updated_at_column` trigger, and sharing seed row
- Implemented `FeatureGateCache` with `IsPremium` (safe default for unknown keys), `Start` (initial reload + Pub/Sub goroutine), `reload` (atomic map swap), `GetAll` (admin endpoint use)
- 9 unit tests pass: known key true/false, unknown key safe default, empty cache safe default, Pub/Sub invalidation, periodic refresh (50ms ticker), DB row mapping, constant values
- Created ADR-0008 documenting the DB + in-memory cache decision over env vars, per-user flags, and hardcoded constants
- Updated ADR README index (total ADRs: 8)

## Task Commits

1. **Task 1: Migration + FeatureGateCache with tests** - `1c82dc7` (feat)
2. **Task 2: ADR-0008 Feature Gate Infrastructure** - `394a3b1` (docs)

## Files Created/Modified

- `migrations/044_feature_gates.sql` — CREATE TABLE feature_gates with sharing seed row
- `services/share-service/featuregates/cache.go` — FeatureGateCache implementation (IsPremium, Start, reload, GetAll, test constructors)
- `services/share-service/featuregates/cache_test.go` — 9 unit tests via miniredis + testable constructors
- `services/share-service/go.mod` — miniredis/v2 v2.37.0 added as direct dependency
- `docs/adr/0008-feature-gate-infrastructure.md` — ADR-0008
- `docs/adr/README.md` — ADR-0008 entry added, total updated to 8

## Decisions Made

- Cache lives in `share-service/featuregates/` not `shared/` — avoids premature abstraction; move when second service needs it
- Unknown gate keys return `true` (premium required) — safe default per D-10 pitfall 2
- `onReload` test hook + `refreshIntervalOverride` allow goroutine behavior testing without a real DB or long waits
- Used `NewFeatureGateCacheWithGates` for pure `IsPremium` unit tests (no goroutines); `NewFeatureGateCacheForTest` for Pub/Sub/ticker goroutine tests

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. Migration 044 will be applied automatically on next deployment.

## Next Phase Readiness

- `FeatureGateCache` is ready for use in Plan 02 (RequirePremium middleware rewrite)
- `GateSharing` constant is exported for middleware to reference
- `GetAll` method is ready for Plan 03 (admin GET endpoint)
- `PubSubChannel` constant is exported for Plan 03 (admin UPDATE to publish invalidation)

---
*Phase: 07-feature-gate-infrastructure*
*Completed: 2026-03-29*
