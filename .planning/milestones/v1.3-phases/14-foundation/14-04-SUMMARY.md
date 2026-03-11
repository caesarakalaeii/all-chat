---
phase: 14-foundation
plan: 04
subsystem: background-jobs, ui
tags: [expiry-job, time-ticker, react, dashboard, tailwind]

# Dependency graph
requires:
  - phase: 14-02
    provides: Share request repository and API endpoints
  - phase: 14-03
    provides: Premium enforcement middleware
provides:
  - Background expiry job running every 5 minutes for automatic request cleanup
  - Dashboard UI with card layout and tab filtering for viewing incoming share requests
  - Platform badge components for visual source identification
affects: [15-acceptance, 16-delivery]

# Tech tracking
tech-stack:
  added: [date-fns]
  patterns: [background-job-with-ticker, card-based-dashboard, tab-filtering]

key-files:
  created:
    - services/share-service/jobs/expiry.go
    - services/share-service/jobs/expiry_test.go
    - frontend/src/app/dashboard/shares/page.tsx
    - frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx
    - frontend/src/app/dashboard/shares/components/PlatformBadge.tsx
    - frontend/src/lib/api/shares.ts
    - frontend/src/lib/types/share.ts
  modified:
    - services/share-service/cmd/main.go
    - services/share-service/repository/share_repo.go

key-decisions:
  - "Expiry job runs every 5 minutes with immediate execution on start"
  - "7-day expiry window for pending requests enforced at database level"
  - "Card-based UI with responsive grid (1→2→3 columns) matching admin pages pattern"
  - "Tab filtering separates Pending (actionable) from History (completed/expired)"
  - "Graceful shutdown stops expiry job before HTTP server to allow current operation to complete"

patterns-established:
  - "Background job pattern: time.Ticker with done channel for graceful shutdown"
  - "Dashboard layout: card grid with tab filtering reuses admin/users page pattern"
  - "Platform badges: colored chips with tooltips for source identification"

requirements-completed: [SHARE-03]

# Metrics
duration: 4min
completed: 2026-03-09
---

# Phase 14 Plan 4: Dashboard and Expiry Summary

**Background job running every 5 minutes to auto-expire stale share requests, React dashboard with card layout displaying incoming requests filtered by pending/history status**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-09T17:42:19Z
- **Completed:** 2026-03-09T17:49:16Z
- **Tasks:** 5 (4 auto + 1 checkpoint)
- **Files modified:** 10

## Accomplishments
- Background expiry job with time.Ticker auto-expires pending requests older than 7 days
- Graceful shutdown protocol ensures current expiry operation completes before service stops
- Dashboard page with card layout and tab filtering (Pending/History)
- Platform badges with color coding and tooltips for visual source identification
- API client for frontend with fetchIncoming, createRequest, and searchUsers methods
- Responsive grid layout (1→2→3 columns) matching existing admin pages

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement background expiry job with time.Ticker** - `a57c5e6` (feat)
2. **Task 2: Wire expiry job into main.go with graceful shutdown** - `325bfd0` (feat)
3. **Task 3: Create share requests API client for frontend** - `320dd79` (feat)
4. **Task 4: Create dashboard page with card layout and tab filtering** - `5fc2dae` (feat)
5. **Task 5: Verify dashboard UI and expiry job functionality** - User approved checkpoint

**Plan metadata:** [Will be committed after this summary]

## Files Created/Modified

**Backend:**
- `services/share-service/jobs/expiry.go` - Background job with 5-minute ticker for expiring old pending requests
- `services/share-service/jobs/expiry_test.go` - Test coverage for expiry job Start/Stop lifecycle
- `services/share-service/cmd/main.go` - Wire expiry job with graceful shutdown before HTTP server
- `services/share-service/repository/share_repo.go` - Added ExpirePendingRequests method

**Frontend:**
- `frontend/src/app/dashboard/shares/page.tsx` - Dashboard with card layout and tab filtering (120+ lines)
- `frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx` - Individual request card with user info, platform badges, status (60+ lines)
- `frontend/src/app/dashboard/shares/components/PlatformBadge.tsx` - Colored platform badges with tooltips
- `frontend/src/lib/api/shares.ts` - API client for share endpoints (40+ lines)
- `frontend/src/lib/types/share.ts` - TypeScript types for ShareRequest and UserSearchResult

## Decisions Made

1. **Expiry job runs immediately on start** - Don't wait 5 minutes for first execution, expire old requests right away
2. **30-second timeout per expiry operation** - Prevent indefinite blocking if database slow
3. **Graceful shutdown order** - Stop expiry job BEFORE HTTP server shutdown to allow current tick to complete
4. **Sender and overlay_sources nullable** - Dashboard displays NULL for now, will be populated via JOIN in Phase 15
5. **Accept/Reject buttons as placeholders** - Console.log only, full functionality in Phase 15 acceptance flow
6. **Most recent first sorting** - Display requests in DESC created_at order for user experience

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed as specified with checkpoint verification passed by user.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Dashboard UI ready for Phase 15 acceptance/rejection functionality
- Expiry job running in production ensures stale requests cleaned up automatically
- API client ready for Phase 15 to add accept/reject endpoints
- Repository layer ready for JOIN queries to populate sender and overlay_sources

**Blockers:** None

---
*Phase: 14-foundation*
*Completed: 2026-03-09*
