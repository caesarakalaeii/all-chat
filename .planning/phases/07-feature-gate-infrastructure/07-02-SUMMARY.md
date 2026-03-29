---
phase: 07-feature-gate-infrastructure
plan: "02"
subsystem: share-service/handlers
tags: [feature-gates, admin-api, redis-pubsub, tdd]
dependency_graph:
  requires: []
  provides: [AdminFeatureGatesHandler, featureGateDB interface, featureGateRedis interface]
  affects: [services/share-service/cmd/main.go (Plan 03 wiring)]
tech_stack:
  added: []
  patterns: [interface-based mock injection, *bool pointer for Gin bool binding]
key_files:
  created:
    - services/share-service/handlers/admin_featuregates.go
    - services/share-service/handlers/admin_featuregates_test.go
  modified: []
decisions:
  - "featureGateDB and featureGateRedis narrow interfaces defined in handler file for mock injection without pgxmock dependency"
  - "*bool pointer for IsPremium in updateFeatureGateRequest avoids Gin binding:required rejecting false"
  - "featureGatesPubSubChannel const defined locally to avoid Plan 01 compile dependency; Plan 03 unifies to featuregates.PubSubChannel"
  - "Redis publish failure is best-effort — 200 returned even if publish fails since DB already updated"
metrics:
  duration: 116s
  completed: "2026-03-29T13:30:27Z"
  tasks_completed: 1
  files_changed: 2
---

# Phase 07 Plan 02: Admin Feature Gates Handler Summary

Admin feature gates handler with GET (list) and PATCH (toggle) endpoints, full unit test coverage including the `is_premium=false` bool binding pitfall.

## What Was Built

`AdminFeatureGatesHandler` in `services/share-service/handlers/admin_featuregates.go` providing:

- **`ListGates`** — `GET /api/v1/admin/feature-gates`: queries all rows from `feature_gates` table, returns JSON array (empty `[]` not `null` when no rows exist).
- **`UpdateGate`** — `PATCH /api/v1/admin/feature-gates/:key`: updates `is_premium` in DB, checks `RowsAffected()` for 404, publishes invalidation to `feature-gates:invalidate` Redis channel (best-effort).

### Interface Design

Two narrow interfaces were introduced to enable mock injection in tests without adding pgxmock or miniredis dependencies:

- `featureGateDB` — `QueryFeatureGates` and `UpdateFeatureGate` methods
- `featureGateRedis` — `Publish` method

Production code uses `pgxFeatureGateDB` and `redisFeatureGateClient` wrappers. Tests inject `mockFeatureGateDB` and `mockFeatureGateRedis` structs.

### Gin Bool Binding Pitfall (D-13 explicit requirement)

`updateFeatureGateRequest.IsPremium` uses `*bool` (pointer type). Gin's `binding:"required"` treats the zero value of `bool` (which is `false`) as "not provided", causing `PATCH` with `{"is_premium": false}` to return 400. The `*bool` approach correctly distinguishes `nil` (absent) from `false` (set to free).

## Test Coverage (9 tests)

| Test | Validates |
|------|-----------|
| `TestListGates_ReturnsAllGates` | 200 with populated JSON array |
| `TestListGates_ReturnsEmptyArrayWhenNoGates` | 200 with `[]` (not `null`) |
| `TestListGates_Returns500OnDBError` | 500 on DB failure |
| `TestUpdateGate_SetIsPremiumFalse` | 200 + Redis publish for `false` value |
| `TestUpdateGate_SetIsPremiumTrue` | 200 + Redis publish for `true` value |
| `TestUpdateGate_Returns404ForNonExistentKey` | 404 when RowsAffected=0, no publish |
| `TestUpdateGate_Returns400WhenBodyMissing` | 400 when `is_premium` absent |
| `TestUpdateGate_Returns400WhenBodyInvalid` | 400 on malformed JSON |
| `TestUpdateGate_PublishErrorDoesNotFailRequest` | 200 even when Redis publish fails |

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Admin feature gates handler with tests | 28d7436 | admin_featuregates.go, admin_featuregates_test.go |

## Deviations from Plan

**1. [Rule 2 - Missing abstraction] Interface-based mock injection added**
- **Found during:** Task 1 (test setup)
- **Issue:** No pgxmock or miniredis in go.mod; plan said "use pgxmock or test against real DB" but neither was available/appropriate
- **Fix:** Defined narrow `featureGateDB` and `featureGateRedis` interfaces in the handler file; thin wrappers for production, mock structs for tests
- **Files modified:** admin_featuregates.go
- **Impact:** None — public API (`NewAdminFeatureGatesHandler`) takes `*pgxpool.Pool` and `*redis.Client` as before

No other deviations — plan executed as specified.

## Self-Check: PASSED

- [x] `services/share-service/handlers/admin_featuregates.go` exists
- [x] `services/share-service/handlers/admin_featuregates_test.go` exists
- [x] Commit 28d7436 exists
- [x] All 9 tests pass
