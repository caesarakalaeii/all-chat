---
phase: 25-page-migration-split-view-preview
plan: 01
subsystem: ui
tags: [react, nextjs, tailwind, storybook, appnav, design-system, a11y]

# Dependency graph
requires:
  - phase: 24-component-library-setup-customization
    provides: Storybook infrastructure, design tokens, globals.css CSS custom properties
  - phase: 23-design-token-system-foundation
    provides: bg-nav-bg, color-twitch, color-tiktok tokens and @layer ordering decisions
provides:
  - AppNav shared frosted glass nav component with logo-ring animation
  - Storybook story stubs for Landing, Dashboard, OverlayEditor, Settings, Admin pages
  - .logo-ring CSS class and @keyframes ring-spin in globals.css
affects:
  - 25-02
  - 25-03
  - 25-04
  - 25-05
  - 25-06

# Tech tracking
tech-stack:
  added: []
  patterns:
    - AppNav as shared component — import once per page, not duplicated across layouts
    - Storybook story stubs with inline placeholder components (Phase 24 pattern) for wave-0 test infrastructure

key-files:
  created:
    - frontend/src/components/AppNav.tsx
    - frontend/src/stories/LandingPage.stories.tsx
    - frontend/src/stories/Dashboard.stories.tsx
    - frontend/src/stories/OverlayEditor.stories.tsx
    - frontend/src/stories/Settings.stories.tsx
    - frontend/src/stories/AdminLayout.stories.tsx
  modified:
    - frontend/src/app/globals.css

key-decisions:
  - "AppNav is a named export (not default) for explicit import clarity"
  - "isActive uses exact match OR startsWith(href + '/') to handle nested routes"
  - ".logo-ring class added to @layer design-system; @keyframes ring-spin kept at document scope outside @layer per Phase 23-03 decision"
  - "Story stubs use inline placeholder components — avoids TypeScript module resolution errors before real page components are built"

patterns-established:
  - "Pattern 1: Story stubs precede page migration — all pages have Storybook coverage from day one (Nyquist rule)"
  - "Pattern 2: bg-bg not bg-white in all dark-theme stories — validates dark theme correctness from the start"
  - "Pattern 3: AppNav imported directly in each page component — not in layout.tsx — per ProtectedRoute pattern"

requirements-completed: [PAGE-01, PAGE-02, PAGE-03, PAGE-05, PAGE-06, PAGE-07, PAGE-08]

# Metrics
duration: 7min
completed: 2026-03-11
---

# Phase 25 Plan 01: AppNav Component and Story Stubs Summary

**Frosted glass AppNav with logo-ring conic gradient animation and Storybook wave-0 story stubs for all 5 page migration targets**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-11T18:21:16Z
- **Completed:** 2026-03-11T18:28:00Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Created AppNav shared component with frosted glass sticky nav, logo-ring animation, and active gradient underline (purple → teal)
- Added .logo-ring CSS class to globals.css design-system layer and @keyframes ring-spin at document scope
- Created 5 Storybook story stubs (LandingPage, Dashboard, OverlayEditor, Settings, AdminLayout) as wave-0 test infrastructure
- Dashboard story includes Loading and Empty stories for PAGE-09/10 coverage; OverlayEditor includes SplitView stub for FEAT-01

## Task Commits

Each task was committed atomically:

1. **Task 1: Create AppNav shared component with frosted glass nav + logo-ring** - `37eb8431b` (feat)
2. **Task 2: Create Storybook story stubs for all 5 page migration targets** - `74616b4ba` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `frontend/src/components/AppNav.tsx` - Shared frosted glass nav for all authenticated pages
- `frontend/src/app/globals.css` - Added .logo-ring class and @keyframes ring-spin
- `frontend/src/stories/LandingPage.stories.tsx` - Pages/Landing story stub
- `frontend/src/stories/Dashboard.stories.tsx` - Pages/Dashboard with Default, Loading, Empty stories
- `frontend/src/stories/OverlayEditor.stories.tsx` - Pages/OverlayEditor with Default and SplitView stub
- `frontend/src/stories/Settings.stories.tsx` - Pages/Settings story stub
- `frontend/src/stories/AdminLayout.stories.tsx` - Pages/Admin story stub

## Decisions Made
- AppNav uses named export only (not default export) for consistent import style across the codebase
- `isActive` implements exact match OR nested route detection via `startsWith(href + '/')` to avoid false positives
- `.logo-ring` CSS placed in @layer design-system; `@keyframes ring-spin` at document scope per existing Phase 23-03 decision that animation names are globally scoped regardless of cascade layer context
- Story stubs use inline placeholder components per Phase 24 pattern — avoids TypeScript module resolution errors before real components exist

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- AppNav component ready for import by all migrated pages in Plans 25-02 through 25-06
- Storybook story stubs provide automated a11y verification coverage from day one
- All subsequent plans can import AppNav with: `import { AppNav } from '@/components/AppNav'`
- TypeScript build is clean, no errors

---
*Phase: 25-page-migration-split-view-preview*
*Completed: 2026-03-11*
