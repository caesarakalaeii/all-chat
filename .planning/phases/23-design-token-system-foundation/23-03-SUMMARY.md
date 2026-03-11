---
phase: 23-design-token-system-foundation
plan: 03
subsystem: ui
tags: [css, cascade-layers, design-system, marketplace, events, overlay]

# Dependency graph
requires:
  - phase: 23-design-token-system-foundation
    plan: 01
    provides: "globals.css cascade layer order declaration (@layer base, design-system, marketplace-themes, user-overrides)"
  - phase: 23-design-token-system-foundation
    plan: 02
    provides: "PLATFORM_COLORS static map, gradient migration completed"
provides:
  - "events.css wrapped in @layer marketplace-themes with zero !important declarations"
  - "EVENTS_CSS_API.md stability contract documenting frozen class names for marketplace theme authors"
affects:
  - 24-component-library
  - 25-page-migration
  - 26-enforcement

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CSS cascade layer wrapping: @layer marketplace-themes { } for overlay-specific public API"
    - "@keyframes at document scope (outside layers) for animation name global resolution"
    - "Stability contract documentation for frozen public CSS APIs"

key-files:
  created:
    - frontend/src/styles/EVENTS_CSS_API.md
  modified:
    - frontend/src/styles/events.css

key-decisions:
  - "@keyframes kept outside @layer because animation names are global regardless of layer context — animations referenced inside a layer still resolve to keyframes defined outside"
  - "Layer order declaration duplicated in events.css because overlay preview pages load events.css independently without globals.css in some contexts"
  - "All 14 !important declarations removed — cascade layer priority (marketplace-themes > design-system) provides equivalent specificity control without specificity hacks"

patterns-established:
  - "Public API CSS: all overlay marketplace rules live in @layer marketplace-themes, @keyframes at document scope"
  - "Stability contract docs: frozen class names documented in EVENTS_CSS_API.md with 30-day deprecation policy"

requirements-completed: [FOUND-04, FOUND-06]

# Metrics
duration: 4min
completed: 2026-03-10
---

# Phase 23 Plan 03: events.css Cascade Layer Migration Summary

**events.css migrated to @layer marketplace-themes with zero !important, plus EVENTS_CSS_API.md stability contract for overlay marketplace theme authors**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-10T20:32:00Z
- **Completed:** 2026-03-10T20:36:16Z
- **Tasks:** 3 of 3 complete
- **Files modified:** 2

## Accomplishments

- Rewrote events.css with @layer order declaration at top, all @keyframes at document scope, all 304 lines of rule sets wrapped inside @layer marketplace-themes
- Removed all 14 !important declarations — cascade priority now handles specificity correctly
- Created EVENTS_CSS_API.md documenting the frozen public API class names, cascade layer architecture, usage example, and 30-day deprecation change policy
- npm run build passes with no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate events.css to @layer marketplace-themes** - `aa825f960` (feat)
2. **Task 2: Create EVENTS_CSS_API.md stability contract** - `a0a943053` (feat)
3. **Task 3: Visual verification of design system foundation** - checkpoint:human-verify approved

**Plan metadata:** `71ea65882` (docs: complete plan + human-verify approved)

## Files Created/Modified

- `frontend/src/styles/events.css` - Rewritten with @layer cascade architecture, zero !important, @keyframes at document scope
- `frontend/src/styles/EVENTS_CSS_API.md` - New stability contract documenting frozen class names, cascade layer priority, usage example, change policy

## Decisions Made

- @keyframes blocks kept outside @layer — CSS animation names are globally scoped regardless of cascade layer context; animations referenced inside a layer will resolve to keyframes defined at document scope
- Layer order declaration (`@layer base, design-system, marketplace-themes, user-overrides`) added to events.css directly, because overlay preview pages can load events.css independently without globals.css
- All !important removed — @layer marketplace-themes sits above @layer design-system in the cascade, providing equivalent or greater specificity without the arms race

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 23 foundation complete:
- Plan 01: globals.css clean-slate with three-tier design tokens, cascade layer order, Barlow + DM Mono fonts
- Plan 02: PLATFORM_COLORS static map, ThemePreview.tsx migrated off getPlatformColor(), gradient migration
- Plan 03: events.css @layer marketplace-themes migration, EVENTS_CSS_API.md stability contract

Visual verification checkpoint (Task 3) approved:
- Background: dark purple gradient (near-black, correct)
- Font: Barlow loading correctly
- --color-twitch: #9146ff confirmed
- --color-youtube: #FF4444 confirmed
- --color-tiktok: #69C9D0 confirmed
- bg-gradient-to-* remaining: zero occurrences
- !important in events.css: zero
- @layer marketplace-themes present

Phase 24 (Component Library) is unblocked. No blockers — all automated checks green.

## Self-Check: PASSED

- FOUND: frontend/src/styles/events.css
- FOUND: frontend/src/styles/EVENTS_CSS_API.md
- FOUND: .planning/phases/23-design-token-system-foundation/23-03-SUMMARY.md
- FOUND: commit aa825f960 (Task 1: migrate events.css)
- FOUND: commit a0a943053 (Task 2: EVENTS_CSS_API.md)

---
*Phase: 23-design-token-system-foundation*
*Completed: 2026-03-10*
