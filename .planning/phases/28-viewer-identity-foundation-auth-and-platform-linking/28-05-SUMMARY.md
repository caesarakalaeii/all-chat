---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
plan: 05
subsystem: ui
tags: [nextjs, react, typescript, viewer-identity, jwt, cosmetics, settings]

# Dependency graph
requires:
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking
    plan: 01
    provides: viewer JWT claims (viewer_id, display_name, avatar_url, platform), cosmetics PATCH endpoint
provides:
  - frontend/src/app/settings/viewer/page.tsx — /settings/viewer route, viewer identity settings page stub
affects:
  - 28-06+ (Phase 29 cosmetics editor extends this page with full gradient editor)
  - Extension popup (EXT-03 Open Settings button destination)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "localStorage viewer JWT decode: JSON.parse(atob(token.split('.')[1])) with exp check"
    - "Three-state hydration guard: undefined (loading) | null (unauthenticated) | ViewerJWTClaims (authenticated)"
    - "Cosmetics PATCH: inline fetch with viewer_jwt_token Authorization header; silent fail on error"

key-files:
  created:
    - frontend/src/app/settings/viewer/page.tsx
  modified: []

key-decisions:
  - "Used localStorage key viewer_jwt_token (matches viewer-auth-store.ts) not viewer_jwt as plan spec suggested"
  - "Three-state undefined/null/claims hydration prevents flash of unauthenticated content on load"
  - "Color picker saves to localStorage immediately on change; server PATCH is explicit via Save button"

patterns-established:
  - "Pattern: /settings/viewer page reads viewer JWT independently without useViewerAuthStore — avoids hydration coupling"

requirements-completed: [VID-03, VID-06]

# Metrics
duration: 2min
completed: 2026-03-14
---

# Phase 28 Plan 05: Viewer Settings Page Summary

**Next.js App Router /settings/viewer page stub with JWT-decoded viewer identity display, name color picker with PATCH cosmetics endpoint, and linked platforms section scoped to single-platform Phase 28 state**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-14T16:08:50Z
- **Completed:** 2026-03-14T16:10:06Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- /settings/viewer route auto-registered via Next.js App Router directory structure
- Unauthenticated state renders sign-in links for all three platforms (Twitch, YouTube, Kick)
- Authenticated state shows profile (avatar, display name, platform badge), name color picker, and linked platforms
- Color picker writes to localStorage immediately and PATCHes /api/v1/auth/viewer/cosmetics on explicit Save
- Linked Platforms section shows only current JWT platform as "Connected"; others show "Connect" button
- TypeScript compiles without errors

## Task Commits

Each task was committed atomically:

1. **Task 1: /settings/viewer page stub** - `dfc93e4f5` (feat)

## Files Created/Modified

- `frontend/src/app/settings/viewer/page.tsx` - Viewer identity settings page: unauthenticated state, profile card, name color picker, linked platforms section

## Decisions Made

- Used `localStorage.getItem('viewer_jwt_token')` (matching the existing viewer-auth-store.ts key) rather than `viewer_jwt` as listed in the plan spec — plan spec had a stale key name
- Three-state hydration pattern (undefined = loading, null = no token/expired, ViewerJWTClaims = authenticated) prevents flash of wrong UI state during SSR hydration
- Color picker updates localStorage on every change for instant persistence; server PATCH triggered by explicit "Save color" button to avoid excessive API calls

## Deviations from Plan

None — plan executed exactly as written. The localStorage key correction (viewer_jwt → viewer_jwt_token) is a trivial auto-fix (Rule 1) to match the existing store implementation; no behavior change.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- /settings/viewer is live and navigable as EXT-03 Open Settings destination
- Page is intentionally minimal per Phase 28 scope — Phase 29 expands with full color + gradient editor
- The Save color button calls PATCH /api/v1/auth/viewer/cosmetics which was implemented in plan 28-03

---
*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Completed: 2026-03-14*
