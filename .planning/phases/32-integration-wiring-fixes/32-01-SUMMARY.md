---
phase: 32-integration-wiring-fixes
plan: 01
subsystem: database
tags: [postgres, go, enricher, message-processor, viewer-identity]

# Dependency graph
requires:
  - phase: 31-all-chat-platform-badges
    provides: ViewerBadgeEnricher SQL query with is_premium SELECT column
  - phase: 30-avatar-frame-flair-system
    provides: viewer_cosmetics, cosmetic_frames, cosmetic_flairs joins in enricher
provides:
  - Corrected enricher SQL reading viewers.is_premium from viewers table (migration 036)
  - Premium badge now injected for viewers with is_premium=true in the viewers table
affects:
  - message-processor enricher pipeline
  - any phase extending viewer identity enrichment

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "JOIN viewers v ON v.id = vpi.viewer_id pattern for viewer cosmetic flags separate from users table"

key-files:
  created: []
  modified:
    - services/message-processor/enricher/viewer_badge_enricher.go

key-decisions:
  - "viewers.is_premium is the viewer cosmetic flag (migration 036); users.is_premium is the streamer/owner account flag — they must not be confused"

patterns-established:
  - "Viewer cosmetic flags (is_premium) read from viewers table via direct JOIN; streamer account flags (is_admin) read from users table via viewer_sessions LATERAL JOIN"

requirements-completed:
  - BADGE-02

# Metrics
duration: 3min
completed: 2026-03-16
---

# Phase 32 Plan 01: Integration Wiring Fixes Summary

**SQL bug fix: enricher reads viewers.is_premium (viewer cosmetic flag, migration 036) instead of users.is_premium (streamer-only account flag) so premium viewers receive the gem badge in overlays**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-16
- **Completed:** 2026-03-16
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Fixed SQL JOIN bug where `COALESCE(u.is_premium, false)` read from `users` table (which only holds streamer accounts), causing `isPremium` to always be false for viewers
- Added `LEFT JOIN viewers v ON v.id = vpi.viewer_id` to enricher query
- Changed SELECT column to `COALESCE(v.is_premium, false) AS is_premium`
- All 21 enricher unit tests pass including `TestEnrich_PremiumBadge`, `TestEnrich_AdminBadge`, `TestEnrich_AdminAndPremiumBadge`, `TestEnrich_NoBadgesForNonRegisteredViewer`

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix enricher SQL — add viewers JOIN and read v.is_premium** - `51e7622` (fix)

## Files Created/Modified
- `services/message-processor/enricher/viewer_badge_enricher.go` - Added `LEFT JOIN viewers v ON v.id = vpi.viewer_id`; changed `COALESCE(u.is_premium, false)` to `COALESCE(v.is_premium, false)`

## Decisions Made
- `is_admin` continues to read from `users` table (streamer account flag) via `u.is_admin` — this is correct and unchanged. Only `is_premium` was wrong because it references the viewer cosmetic flag added in migration 036.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- BADGE-02 closed: premium viewers will receive the gem badge in overlays; cache entries refresh within 5 min TTL
- Enricher SQL is now correct for both admin (users table) and premium (viewers table) flags

---
*Phase: 32-integration-wiring-fixes*
*Completed: 2026-03-16*
