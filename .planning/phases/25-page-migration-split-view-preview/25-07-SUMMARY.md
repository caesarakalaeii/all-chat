---
phase: 25-page-migration-split-view-preview
plan: 07
subsystem: ui
tags: [react, tailwindcss, design-tokens, dark-theme, admin, dialog, toast]

# Dependency graph
requires:
  - phase: 25-06
    provides: "admin/layout.tsx dark theme with AdminNav — sub-pages inherit dark bg-bg"
provides:
  - "admin/users/page.tsx with dark theme, Dialog confirmations, toastManager feedback"
  - "admin/overlays/page.tsx with dark theme, PlatformBadge, Skeleton loading"
  - "admin/sources/page.tsx with dark theme, PlatformBadge for platform column, Skeleton"
  - "admin/viewers/page.tsx with dark theme, Dialog ban/unban, toastManager, Skeleton"
affects:
  - 25-08
  - phase-26-design-enforcement

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Card-wrapped tables: overflow-hidden Card > overflow-x-auto div > table with bg-surface-2 thead"
    - "Dialog.Root controlled pattern for destructive action confirmation"
    - "Dialog.Root with textarea input for ban-with-reason flow"
    - "toastManager replaces react-hot-toast, alert(), and confirm() in admin pages"
    - "PlatformBadge replaces getPlatformColor() helper functions in admin tables"
    - "Skeleton rows inside Card for loading states replacing text spinners"

key-files:
  created: []
  modified:
    - frontend/src/app/admin/users/page.tsx
    - frontend/src/app/admin/overlays/page.tsx
    - frontend/src/app/admin/sources/page.tsx
    - frontend/src/app/admin/viewers/page.tsx

key-decisions:
  - "[Phase 25-07]: BanModal component replaced with inline Dialog.Root in users page — eliminates light-mode modal, simplifies component tree"
  - "[Phase 25-07]: viewers/page.tsx embedded nav removed — redundant with admin/layout.tsx AdminNav from Plan 06"
  - "[Phase 25-07]: getPlatformColor() helper removed from overlays/sources pages — PlatformBadge component handles platform coloring with design tokens"
  - "[Phase 25-07]: react-hot-toast removed from users page imports — toastManager from @/lib/toast used uniformly across all admin pages"

patterns-established:
  - "Card table pattern: <Card className='overflow-hidden'><div className='overflow-x-auto'><table><thead className='bg-surface-2 border-b border-border'>"
  - "Dialog ban with reason: Dialog.Root controlled open state + textarea in content"
  - "Dialog confirm: Dialog.Root > Trigger(render=Button) + Content(showCloseButton=false)"

requirements-completed: [PAGE-06, PAGE-08, PAGE-09]

# Metrics
duration: 5min
completed: 2026-03-11
---

# Phase 25 Plan 07: Admin Sub-Pages Dark Theme Migration Summary

**All 4 admin sub-pages (users, overlays, sources, viewers) fully migrated to dark theme with Card tables, Dialog confirmations, and toastManager feedback replacing window.confirm/alert**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-11T19:46:36Z
- **Completed:** 2026-03-11T19:51:36Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Migrated all 4 admin sub-pages from light mode to dark design tokens — zero legacy gray-scale classes remain
- Replaced `window.confirm()` / `alert()` with `Dialog.Root` confirmations for impersonate, unban, and delete actions
- Replaced BanModal (light-mode custom component) with inline Dialog.Root containing textarea for ban reason input
- Replaced `react-hot-toast` and `toast` from react-hot-toast with `toastManager` from `@/lib/toast` uniformly
- Added `PlatformBadge` component to overlays and sources pages — eliminates hand-rolled `getPlatformColor()` helpers
- Added `Skeleton` loading states to all 4 pages replacing text spinners and loading divs
- Removed duplicated navbar from viewers/page.tsx — admin/layout.tsx (Plan 06) provides the nav

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate admin/users/page.tsx to dark theme** - `b6968d241` (feat)
2. **Task 2: Migrate admin overlays, sources, viewers to dark theme** - `fb468bd9c` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `frontend/src/app/admin/users/page.tsx` — Dark theme, Card list, Dialog impersonate/unban/ban confirmations, toastManager, Skeleton loading
- `frontend/src/app/admin/overlays/page.tsx` — Dark theme, Card list/detail panels, PlatformBadge, Skeleton loading
- `frontend/src/app/admin/sources/page.tsx` — Dark theme, Card stats, Card table with PlatformBadge, filters with dark inputs, Skeleton loading
- `frontend/src/app/admin/viewers/page.tsx` — Dark theme, removed embedded nav, Card table, Dialog ban/unban, toastManager, Skeleton loading

## Decisions Made
- BanModal component (`@/components/admin/BanModal`) replaced inline — its light-mode styles would conflict and Dialog is the established pattern
- viewers/page.tsx had its own `<nav>` block hardcoded which is now redundant — removed cleanly
- `getPlatformColor()` helper functions returning `bg-purple-100 text-purple-800` etc. replaced with `PlatformBadge` component

## Deviations from Plan

None — plan executed exactly as written. The replacement of BanModal with an inline Dialog was implicitly guided by the plan's instruction to remove `window.confirm`/`alert` and use Dialog for destructive actions.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All admin pages now use consistent dark theme — Phase 26 enforcement can scan admin directory cleanly
- No legacy gray-scale classes remain in `frontend/src/app/admin/`
- PAGE-06, PAGE-08, PAGE-09 requirements satisfied

---
*Phase: 25-page-migration-split-view-preview*
*Completed: 2026-03-11*
