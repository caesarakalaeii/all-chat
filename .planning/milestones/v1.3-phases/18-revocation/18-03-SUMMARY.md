---
phase: 18-revocation
plan: "03"
subsystem: ui
tags: [react, typescript, next.js, shares, revocation, modal]

# Dependency graph
requires:
  - phase: 18-01
    provides: Backend POST /shares/:id/revoke endpoint
  - phase: 18-02
    provides: ShareStatus field in overlay source list response
provides:
  - RevocationConfirmModal component with loading state and error handling
  - sharesApi.revokeShare(shareId) API function
  - Revoke button on accepted share cards in dashboard
  - History tab includes 'revoked' status cards
  - ChatSource type extended with is_active and share_status fields
affects: [dashboard shares page, overlay source list, share feature]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Modal component mirroring AcceptModal structure (fixed inset backdrop, centered card, Cancel/Confirm buttons)
    - Optimistic onUpdate() trigger after revocation for re-fetch without page reload

key-files:
  created:
    - frontend/src/app/dashboard/shares/components/RevocationConfirmModal.tsx
  modified:
    - frontend/src/lib/types/overlay.ts
    - frontend/src/lib/api/shares.ts
    - frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx
    - frontend/src/app/dashboard/shares/page.tsx
    - frontend/src/app/dashboard/shares/components/AddSourceModal.test.tsx

key-decisions:
  - "Revoke button placed below StatusBadge, shown only for accepted status — clean visual hierarchy"
  - "onRevoked callback triggers onUpdate() which re-fetches the share list, keeping card data fresh without page reload"

patterns-established:
  - "RevocationConfirmModal mirrors AcceptModal structure: backdrop + centered card + Cancel/Confirm buttons"

requirements-completed: [SHARE-06, SHARE-07]

# Metrics
duration: 2min
completed: 2026-03-10
---

# Phase 18 Plan 03: Dashboard Revocation UI Summary

**Dashboard revocation UI: Revoke button on accepted share cards, RevocationConfirmModal, revokeShare() API method, and History tab fix to include revoked cards**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-10T19:36:15Z
- **Completed:** 2026-03-10T19:38:27Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- Added `is_active: boolean` and `share_status?: 'accepted' | 'revoked' | 'expired'` to `ChatSource` type
- Added `sharesApi.revokeShare(shareId)` method calling `POST /api/v1/shares/:id/revoke`
- Created `RevocationConfirmModal` with confirmation text, loading state, and toast notifications
- Added Revoke button (red pill) on accepted share cards in `ShareRequestCard`
- Fixed History tab filter in `page.tsx` to include `'revoked'` status (was missing, revoked cards were hidden)

## Task Commits

Each task was committed atomically:

1. **Task 1: ChatSource type extension + sharesApi.revokeShare()** - `fef6813` (feat)
2. **Task 2: RevocationConfirmModal component** - `4b1c201` (feat)
3. **Task 3: ShareRequestCard Revoke button + History tab 'revoked' fix** - `713a53b` (feat)

## Files Created/Modified

- `frontend/src/lib/types/overlay.ts` - ChatSource extended with is_active and share_status fields
- `frontend/src/lib/api/shares.ts` - revokeShare(shareId) method added to sharesApi
- `frontend/src/app/dashboard/shares/components/RevocationConfirmModal.tsx` - New confirmation modal component
- `frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx` - Revoke button + RevocationConfirmModal wired in
- `frontend/src/app/dashboard/shares/page.tsx` - History filter includes 'revoked'
- `frontend/src/app/dashboard/shares/components/AddSourceModal.test.tsx` - Test fixtures updated to include is_active (auto-fix)

## Decisions Made

- Revoke button placed below StatusBadge, shown only for `accepted` status — clean visual hierarchy that doesn't clutter other states
- `onRevoked()` callback triggers `onUpdate()` which re-fetches the share list, keeping data fresh without page reload

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed AddSourceModal.test.tsx fixtures broken by ChatSource type change**
- **Found during:** Task 1 (ChatSource type extension)
- **Issue:** Adding required `is_active: boolean` to `ChatSource` broke two test fixtures: one was missing `is_active` and another had an unknown `auth_required` property
- **Fix:** Added `is_active: true` to first fixture; removed `auth_required: false` from second fixture (property doesn't exist on ChatSource)
- **Files modified:** `frontend/src/app/dashboard/shares/components/AddSourceModal.test.tsx`
- **Verification:** `npx tsc --noEmit` produces no errors
- **Committed in:** fef6813 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in test fixture from required field addition)
**Impact on plan:** Required fix for TypeScript compilation. No scope creep.

## Issues Encountered

None beyond the auto-fixed test fixture deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Full revocation flow is now complete: backend endpoint (18-01), share_status in source list (18-02), and dashboard UI (18-03)
- Phase 18-04 (WebSocket share_revoked notification) can proceed
- No blockers

---
*Phase: 18-revocation*
*Completed: 2026-03-10*
