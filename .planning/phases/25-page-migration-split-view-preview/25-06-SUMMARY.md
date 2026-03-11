---
phase: 25-page-migration-split-view-preview
plan: 06
subsystem: ui
tags: [react, nextjs, tailwind, design-tokens, admin, dark-theme, storybook]

# Dependency graph
requires:
  - phase: 25-01
    provides: AppNav frosted glass pattern, logo-ring, bg-nav-bg design tokens

provides:
  - AdminNav component with frosted glass nav matching AppNav pattern
  - admin/layout.tsx with dark theme bg-bg and AdminNav
  - admin/page.tsx with Card/Skeleton components and design token classes
  - AdminLayout.stories.tsx with Default and Loading story variants

affects:
  - 25-07 (admin sub-pages inherit AdminNav layout pattern)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - AdminNav follows identical frosted glass pattern as AppNav with admin-specific links
    - StatCard inline component uses Card + Skeleton from ui library
    - Admin layout preserved ProtectedRoute requireAdmin and ToastProvider wrappers

key-files:
  created:
    - frontend/src/components/AdminNav.tsx
  modified:
    - frontend/src/app/admin/layout.tsx
    - frontend/src/app/admin/page.tsx
    - frontend/src/stories/AdminLayout.stories.tsx

key-decisions:
  - "AdminNav is a separate component (not AppNav reuse) — admin has its own 5-link sub-nav plus a breadcrumb-style Admin label"
  - "surface-hover token does not exist in design system — used bg-surface-2 for nav card hover states"
  - "admin/layout.tsx converted from 'use client' to server component — ProtectedRoute and AdminNav are client components imported normally"

patterns-established:
  - "Admin sub-nav uses same h-[60px] bg-nav-bg backdrop-blur-[20px] border-b border-border pattern as AppNav"
  - "Admin section label shown as text-text-sub span between logo and nav links"

requirements-completed:
  - PAGE-06
  - PAGE-08

# Metrics
duration: 3min
completed: 2026-03-11
---

# Phase 25 Plan 06: Admin Layout Dark Theme Migration Summary

**AdminNav frosted glass component with bg-nav-bg pattern, admin/layout.tsx dark theme, admin/page.tsx Card-based stats dashboard, and Storybook story with Default and Loading variants**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-11T19:41:01Z
- **Completed:** 2026-03-11T19:44:01Z
- **Tasks:** 2
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments
- Created `AdminNav` client component with frosted glass `bg-nav-bg backdrop-blur-[20px]` nav matching AppNav pattern, with 5 admin links (Dashboard, Users, Overlays, Sources, Viewers) and exact-match active indicator
- Replaced light-mode admin layout (`bg-gray-50`, `bg-white shadow-sm`) with dark theme `bg-bg` wrapper and AdminNav; preserved `ProtectedRoute requireAdmin` and `ToastProvider`
- Rewrote admin dashboard page replacing SVG icons + `bg-white shadow rounded-lg` cards with lucide-react icons + Card/Skeleton components and text-text/text-text-sub tokens
- Updated AdminLayout.stories.tsx from placeholder stub to full story with Default (populated stats) and Loading (skeleton) variants using dark bg-bg background

## Task Commits

1. **Task 1: Convert admin/layout.tsx and admin/page.tsx to dark theme** - `603b60645` (feat)
2. **Task 2: Update AdminLayout.stories.tsx to validate dark theme** - `ebdb82134` (feat)

## Files Created/Modified
- `frontend/src/components/AdminNav.tsx` - New frosted glass admin nav with 5 links and active underline indicator
- `frontend/src/app/admin/layout.tsx` - Converted from 'use client' + light-mode to server component + dark theme AdminNav
- `frontend/src/app/admin/page.tsx` - Rewritten with Card/Skeleton design system components, lucide-react icons, design tokens
- `frontend/src/stories/AdminLayout.stories.tsx` - Full story with Default and Loading variants on dark bg-bg background

## Decisions Made
- AdminNav created as separate component rather than extending AppNav — admin has a distinct 5-link sub-navigation plus a breadcrumb-style "Admin" label that would not fit AppNav's structure
- `bg-surface-hover` token doesn't exist in the design system; used `bg-surface-2` (maps to `--color-neutral-800`) for nav card hover states — same pattern as existing elevated elements
- `admin/layout.tsx` converted from `'use client'` to a standard server component since it only wraps client components (ProtectedRoute, AdminNav) — Next.js handles this correctly

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## Next Phase Readiness
- AdminNav and layout pattern fully established for admin section
- Plan 07 (admin sub-pages migration) can inherit AdminNav from this plan directly
- All admin pages now inherit dark theme layout automatically through `admin/layout.tsx`

---
*Phase: 25-page-migration-split-view-preview*
*Completed: 2026-03-11*
