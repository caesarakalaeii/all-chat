---
phase: 17-message-routing
plan: "01"
subsystem: message-routing
tags: [go, postgres, sql, redis, message-processor, overlay-manager, shared-overlay, union-query]

# Dependency graph
requires:
  - phase: 17-00
    provides: Failing RED test scaffolds for FindOverlaysForMessage UNION fan-out and shared_overlay is_active=true contract
  - phase: 16-shared-overlay-sources
    provides: shared_overlay platform branch in HandleAddSource and nil-db guard pattern
provides:
  - UNION SQL query in FindOverlaysForMessage routing messages to recipient overlays via accepted shares
  - is_active=true override for shared_overlay sources in HandleAddSource
  - SOURCE-04 and SOURCE-05 end-to-end fan-out routing
affects:
  - 17-02 (if exists — integration/E2E verification of fan-out routing)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SQL UNION for message fan-out routing: direct JOIN + shared_overlay JOIN via share_requests WHERE status='accepted'"
    - "Inline is_active override: req.Platform == 'shared_overlay' expression sets IsActive directly in struct literal"

key-files:
  created: []
  modified:
    - services/message-processor/router/overlay_router.go
    - services/message-processor/router/overlay_router_test.go
    - services/overlay-manager/handlers/sources.go

key-decisions:
  - "UNION (not UNION ALL) for deduplication: overlays appearing in both direct and shared branches returned exactly once via SELECT DISTINCT"
  - "Subquery IN for shared fan-out: sr.sender_overlay_id IN (SELECT o2.id ...) reuses $1/$2 params without additional bound params"
  - "ocs.is_active=true added to direct branch: sources with is_active=false should not route messages; UNION branch consistency requires this filter in both branches"
  - "IsActive: req.Platform == 'shared_overlay' expression: minimal inline change avoids separate override block; boolean expression is self-documenting"
  - "Wave 1 sentinel update: productionQueryHasUnion updated from false to true; directResults 2-item assertion replaced with unionResult 2-item assertion"

patterns-established:
  - "SQL UNION fan-out pattern: use UNION (not UNION ALL) + SELECT DISTINCT to prevent duplicate overlay IDs when overlay appears in multiple branches"
  - "Accepted-only share filter: always JOIN share_requests ON sr.status='accepted' to automatically exclude revoked/expired shares without separate query"

requirements-completed:
  - SOURCE-04
  - SOURCE-05

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 17 Plan 01: Message Routing Fan-Out Summary

**SQL UNION in FindOverlaysForMessage routes messages to recipient overlays via accepted share_requests, with shared_overlay sources created as is_active=true in HandleAddSource**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T17:41:00Z
- **Completed:** 2026-03-10T17:43:13Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Extended FindOverlaysForMessage with UNION query: messages for platform/channel now fan out to both direct overlays AND recipient overlays that subscribed via accepted shared_overlay sources
- Added ocs.is_active=true filter to the existing direct branch (consistency with UNION branch; sources with is_active=false should not receive messages)
- Fixed is_active=false bug for shared_overlay sources: they are immediately active at creation since the share was already accepted, no listener activation step exists
- All 5 TestFindOverlaysForMessage sub-tests pass GREEN (direct_only, no_match, shared_fan_out, revoked_excluded, both_direct_and_shared)

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend FindOverlaysForMessage with SQL UNION for shared overlay fan-out** - `ae09ee2` (feat)
2. **Task 2: Fix shared_overlay source creation — set is_active=true in HandleAddSource** - `47df2b9` (feat)

## Files Created/Modified
- `services/message-processor/router/overlay_router.go` - UNION query replacing simple JOIN; adds ocs.is_active=true to direct branch
- `services/message-processor/router/overlay_router_test.go` - Wave 0 RED sentinels updated to Wave 1 GREEN (productionQueryHasUnion=true, unionResult len assertion)
- `services/overlay-manager/handlers/sources.go` - IsActive: req.Platform == "shared_overlay" replaces hardcoded IsActive: false

## Decisions Made
- Used `IsActive: req.Platform == "shared_overlay"` inline expression instead of a separate override block — minimal diff, self-documenting
- UNION (not UNION ALL): SELECT DISTINCT handles deduplication for overlays appearing in both branches
- Subquery `IN (SELECT o2.id ...)` for the UNION branch's sender_overlay_id filter reuses $1/$2 parameters across both UNION branches — pgx binds by index across the entire query string
- Updated Wave 0 test sentinels to Wave 1 GREEN state rather than removing them — preserves the documented contract in test comments

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SOURCE-04 and SOURCE-05 are fully implemented: UNION routes to recipient overlays, each receiving messages on its own overlay:{id} Redis Pub/Sub channel (display setting isolation is structural)
- Both services compile cleanly and all tests pass
- No blockers for subsequent phases

---
*Phase: 17-message-routing*
*Completed: 2026-03-10*
