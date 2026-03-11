---
phase: 18-revocation
plan: "02"
subsystem: api
tags: [go, postgresql, overlay-manager, chat-sources, share-requests]

# Dependency graph
requires:
  - phase: 18-revocation-01
    provides: RevokeShareRequest endpoint that sets share_requests.status='revoked'
  - phase: 16-shared-overlay-sources
    provides: shared_overlay platform and channel_id=share_id convention
provides:
  - ShareStatus *string field on ChatSource model (omitempty JSON tag)
  - ListByOverlayID LEFT JOIN share_requests to populate share_status per source
affects:
  - 18-revocation-03 (frontend badge rendering uses share_status from this endpoint)
  - 18-revocation-04 (WS notification may reference share_status in events)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "LEFT JOIN with platform guard: sr.id::text = ocs.channel_id prevents uuid cast errors on non-uuid channel_ids"
    - "Optional field with *string + omitempty: absent from JSON for non-shared sources, present for shared_overlay"

key-files:
  created:
    - services/overlay-manager/repository/source_repo_test.go
  modified:
    - services/overlay-manager/models/chat_source.go
    - services/overlay-manager/repository/source_repo.go

key-decisions:
  - "Cast sr.id to text (sr.id::text = ocs.channel_id) rather than channel_id to uuid — avoids invalid uuid cast errors for twitch/youtube/kick channel IDs in the same query"

patterns-established:
  - "JOIN with heterogeneous key types: cast the UUID side to text rather than the varchar side to uuid"

requirements-completed:
  - SHARE-07

# Metrics
duration: 12min
completed: 2026-03-10
---

# Phase 18 Plan 02: Share Status in Source List Summary

**ChatSource model extended with `ShareStatus *string` + LEFT JOIN on share_requests so overlay editor gets revoked/expired/active status without a second round-trip**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-03-10T20:00:00Z
- **Completed:** 2026-03-10T20:14:00Z
- **Tasks:** 1 (TDD: RED commit + GREEN commit)
- **Files modified:** 3

## Accomplishments
- Added `ShareStatus *string` with `json:"share_status,omitempty"` to `ChatSource` struct
- Updated `ListByOverlayID` to LEFT JOIN `share_requests` on `shared_overlay` sources only
- Non-shared rows return `nil` ShareStatus (field absent from JSON via omitempty)
- Shared rows return the current `share_requests.status` value (active/revoked/expired/pending)
- 4 integration tests added covering all cases (non-shared, active, revoked, mixed)

## Task Commits

1. **RED tests: failing tests for ShareStatus in ListByOverlayID** - `67648fe` (test)
2. **GREEN: implement ShareStatus field and LEFT JOIN query** - `bba2ef9` (feat)

## Files Created/Modified
- `services/overlay-manager/models/chat_source.go` — Added `ShareStatus *string` field after `IsActive`
- `services/overlay-manager/repository/source_repo.go` — Updated `ListByOverlayID` query with LEFT JOIN; scan includes `&source.ShareStatus`
- `services/overlay-manager/repository/source_repo_test.go` — 4 integration tests with testcontainers

## Decisions Made
- **Cast uuid side to text** (`sr.id::text = ocs.channel_id`) rather than casting the varchar channel_id to uuid. Reason: channel_ids for twitch/youtube/kick are not valid uuids ("chan1", "twitch-chan"), so casting them would raise `invalid input syntax for type uuid` errors even in LEFT JOIN ON clauses where PostgreSQL doesn't short-circuit per-row.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed uuid/varchar type mismatch in LEFT JOIN ON clause**
- **Found during:** Task 1 (GREEN implementation)
- **Issue:** Plan specified `sr.id = ocs.channel_id` but sr.id is uuid and ocs.channel_id is varchar. First fix attempt used `ocs.channel_id::uuid` which caused `invalid input syntax for type uuid` errors for non-uuid channel_ids (twitch, youtube, kick sources).
- **Fix:** Changed to `sr.id::text = ocs.channel_id` — cast the UUID side to text, leaving the varchar unchanged
- **Files modified:** services/overlay-manager/repository/source_repo.go
- **Verification:** All 4 integration tests pass including TestListByOverlayID_MixedSources which has both twitch (non-uuid channel_id) and shared_overlay sources
- **Committed in:** bba2ef9 (Task 1 feat commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug)
**Impact on plan:** Required for correctness. The cast direction was not specified in the plan but is essential for production correctness. No scope creep.

## Issues Encountered
- PostgreSQL does not short-circuit LEFT JOIN ON clause conditions per row — attempting `ocs.channel_id::uuid` evaluates the cast for every row in the table, causing errors for non-uuid channel_ids. Cast uuid→text instead.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `ChatSource.ShareStatus` is now available in `GET /api/v1/overlays/:id/sources` responses
- Frontend badge rendering (plan 18-03) can use `share_status` to distinguish Revoked vs Expired states
- No new migrations required — query reads from existing share_requests table

---
*Phase: 18-revocation*
*Completed: 2026-03-10*
