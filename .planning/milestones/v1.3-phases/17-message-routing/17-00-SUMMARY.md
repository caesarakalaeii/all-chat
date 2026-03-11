---
phase: 17-message-routing
plan: "00"
subsystem: testing
tags: [go, pgx, testify, tdd, message-routing, overlay-router, shared-overlay]

# Dependency graph
requires:
  - phase: 16-shared-overlay-sources
    provides: shared_overlay platform branch in HandleAddSource and nil-db guard pattern
provides:
  - Failing RED test scaffold for FindOverlaysForMessage UNION fan-out (overlay_router_test.go)
  - Documented RED contract for shared_overlay is_active=true (sources_shared_overlay_test.go)
  - Passing GREEN baseline for NonSharedPlatform is_active=false
affects:
  - 17-01 (Wave 1 implementation — turns these tests GREEN)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "overlayFinderStub: in-process key-map stub for pgxpool-backed queries (avoids pgxmock dependency)"
    - "productionQueryHasUnion sentinel: assert false to make RED test when query lacks UNION branch"
    - "t.Skip with contract comment: document Wave 1 DB-dependent tests without false-RED noise"

key-files:
  created:
    - services/message-processor/router/overlay_router_test.go
  modified:
    - services/overlay-manager/handlers/sources_shared_overlay_test.go

key-decisions:
  - "Stub-based router tests (no pgxmock): message-processor go.mod lacks pgxmock/testcontainers; in-process overlayFinderStub with map lookup suffices for Wave 0 RED scaffolding"
  - "productionQueryHasUnion=false sentinel: asserts Wave 1 must add UNION branch; clean RED failure with descriptive message"
  - "t.Skip for is_active=true test: nil-db guard returns 403 before createFunc fires; DB dependency deferred to Wave 1 integration test"

patterns-established:
  - "Wave 0 RED pattern: Use assert.False(t, false, description) as a sentinel assertion that documents a required production code change"
  - "createFunc capture pattern: mockSourceRepository.createFunc captures *models.ChatSource for is_active assertions without real DB"

requirements-completed:
  - SOURCE-04

# Metrics
duration: 4min
completed: 2026-03-10
---

# Phase 17 Plan 00: Message Routing Test Scaffolds Summary

**Nyquist Wave 0 scaffolds: failing RED tests for FindOverlaysForMessage UNION fan-out and shared_overlay is_active=true contract, using stub-based approach without pgxmock**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-10T17:36:10Z
- **Completed:** 2026-03-10T17:39:55Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created overlay_router_test.go with 5 sub-tests; shared_fan_out and revoked_excluded fail RED documenting the UNION branch requirement
- Appended TestHandleAddSource_SharedOverlay_IsActiveTrue (SKIPPED/documented RED) and TestHandleAddSource_NonSharedPlatform_IsActiveFalse (GREEN) to sources_shared_overlay_test.go
- Existing TestHandleAddSource_SharedOverlay_Forbidden continues to pass GREEN unchanged

## Task Commits

Each task was committed atomically:

1. **Task 1: Router test scaffold — failing tests for FindOverlaysForMessage UNION** - `13d169c` (test)
2. **Task 2: Sources test — failing test for shared_overlay is_active=true** - `b8a5b35` (test)

## Files Created/Modified
- `services/message-processor/router/overlay_router_test.go` - 5 table-driven sub-tests for FindOverlaysForMessage; shared_fan_out and revoked_excluded FAIL RED
- `services/overlay-manager/handlers/sources_shared_overlay_test.go` - Appended IsActiveTrue (SKIP) and NonSharedPlatform (GREEN) tests

## Decisions Made
- Used `overlayFinderStub` (in-process map lookup) instead of pgxmock since message-processor go.mod does not include pgxmock or testcontainers. This keeps the test dependency footprint minimal.
- Used `productionQueryHasUnion := false; assert.True(t, productionQueryHasUnion, ...)` sentinel for `revoked_excluded` to produce a clean, self-documenting RED failure without requiring DB access.
- Used `t.Skip` with a TODO comment for `TestHandleAddSource_SharedOverlay_IsActiveTrue` per plan specification — nil-db guard prevents createFunc from being reached for shared_overlay without a real database.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave 0 test scaffolds in place; Wave 1 (17-01) can now implement the UNION query and set is_active=true for shared_overlay sources, turning the RED tests GREEN
- Both services compile cleanly (`go build ./...` passes)
- No blockers

---
*Phase: 17-message-routing*
*Completed: 2026-03-10*
