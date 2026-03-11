---
phase: 25-page-migration-split-view-preview
plan: 05
subsystem: ui
tags: [react, nextjs, tailwind, split-view, overlay-editor, drag-resize, iframe, dialog, toast, skeleton]

# Dependency graph
requires:
  - phase: 25-01
    provides: AppNav, design system components, globals.css @layer design-system
  - phase: 25-03
    provides: Card, PlatformBadge, Dialog, Button, Input, Skeleton, toastManager
provides:
  - SplitView component (draggable divider, mobile stack, live iframe preview)
  - Redesigned overlay editor with platform-colored source cards
  - Dialog-based source removal confirmation
  - OverlayEditor Storybook story with Default and Loading variants
affects:
  - 25-06
  - phase-26

# Tech tracking
tech-stack:
  added: []
  patterns:
    - PLATFORM_BORDER static map — complete literal class strings (border-l-twitch, etc.) for Tailwind JIT safety
    - CSS variable --split-left on container + .split-view-config class for responsive width without double render
    - Pointer capture on divider for robust drag even when cursor leaves element
    - isDragging ref (not state) in pointermove handler to avoid re-render storms

key-files:
  created:
    - frontend/src/components/SplitView.tsx
  modified:
    - frontend/src/app/overlays/[id]/page.tsx
    - frontend/src/stories/OverlayEditor.stories.tsx
    - frontend/src/app/globals.css

key-decisions:
  - "SplitView uses CSS variable --split-left on container + .split-view-config class in globals.css to achieve responsive width without double-rendering children or JIT-unsafe dynamic class construction"
  - "PLATFORM_BORDER map uses full literal class strings (border-l-twitch, border-l-youtube, etc.) — same Tailwind JIT safety pattern as PLATFORM_COLORS"
  - "isDragging stored in useRef (not useState) to avoid re-renders on every pointermove — no setState in pointermove handler"
  - "Overlay editor simplified: removed OAuth per-platform buttons (Twitch/YouTube/Kick buttons from old UI), replaced with single AddSourceForm dropdown — cleaner UX, BetaWarning preserved for YouTube only"

patterns-established:
  - "SplitView pattern: container sets --split-left CSS variable, config panel uses .split-view-config class for responsive width"
  - "Platform border coding: PLATFORM_BORDER static map with complete literal border-l-{platform} strings"
  - "Drag handle: pointer capture + isDragging ref + onPointerMove on container (not element) for reliable drag"

requirements-completed: [PAGE-03, PAGE-04, FEAT-01, FEAT-02, FEAT-03, FEAT-04, PAGE-08, PAGE-09]

# Metrics
duration: 15min
completed: 2026-03-11
---

# Phase 25 Plan 05: SplitView Component and Overlay Editor Redesign Summary

**Draggable split-view layout component with live iframe preview embedded in redesigned overlay editor using platform-colored source cards and Dialog/Toast design system**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-11T18:35:00Z
- **Completed:** 2026-03-11T19:43:15Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- SplitView component with pointer-capture drag, 25-70% range constraints, keyboard navigation (ArrowLeft/Right), mobile column stacking, and live iframe preview
- Overlay editor fully migrated from 752-line legacy page to clean design system implementation with SplitView, SourceCard, AddSourceForm, SourceListSkeleton sub-components
- Platform-colored source cards using `border-l-2 border-l-{platform}` with static PLATFORM_BORDER map for Tailwind JIT safety
- window.confirm() eliminated — Dialog.Root confirmation for destructive source removal
- All feedback via toastManager (no notification state or setTimeout patterns)
- OverlayEditor story with Default (full split-view layout) and Loading (skeleton) variants

## Task Commits

Each task was committed atomically:

1. **Task 1: SplitView component with draggable divider and mobile stack** - `c3f29b1be` (feat)
2. **Task 2: Rewrite overlay editor with source cards and SplitView, update story** - `16e5c3f88` (feat)

**Plan metadata:** `{final-commit-hash}` (docs: complete plan)

## Files Created/Modified
- `frontend/src/components/SplitView.tsx` - New reusable split-view component with draggable divider, pointer capture, keyboard navigation, and iframe preview
- `frontend/src/app/overlays/[id]/page.tsx` - Fully redesigned from 752L legacy page to clean design system implementation with SplitView integration
- `frontend/src/stories/OverlayEditor.stories.tsx` - Updated from placeholder stubs to real Default and Loading stories
- `frontend/src/app/globals.css` - Added .split-view-config CSS class in @layer design-system for responsive config panel width

## Decisions Made
- CSS variable approach for SplitView responsive width: `--split-left` on container + `.split-view-config` class in globals.css with `@media (min-width: 768px)` override. This avoids double-rendering children, avoids JIT-unsafe dynamic class construction like `md:[width:${pct}%]`, and follows the project's established CSS variable pattern.
- PLATFORM_BORDER static map mirrors the PLATFORM_COLORS pattern from platform-colors.ts — complete literal class strings for Tailwind JIT safety.
- `isDragging` stored in `useRef` not `useState` — pointermove fires constantly and setting state would trigger continuous re-renders.
- Overlay editor simplified AddSourceForm: removed separate OAuth platform buttons (Twitch/Kick/YouTube buttons) in favor of single form with platform dropdown — cleaner UX for the new compact split-view config panel. BetaWarning dialog preserved for YouTube.

## Deviations from Plan

None — plan executed exactly as written. The CSS approach for responsive split panel width was documented in the plan itself (Task 1 action block explores several options and lands on the `.split-view-config` CSS class approach). Followed exactly.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SplitView component is complete and reusable — any overlay-related page needing a split layout can import it
- Overlay editor migration complete — preview/page.tsx was not touched
- Phase 25 all plans complete — ready for Phase 26 (design system enforcement)

---
*Phase: 25-page-migration-split-view-preview*
*Completed: 2026-03-11*

## Self-Check: PASSED

- FOUND: frontend/src/components/SplitView.tsx
- FOUND: frontend/src/app/overlays/[id]/page.tsx
- FOUND: frontend/src/stories/OverlayEditor.stories.tsx
- FOUND: .planning/phases/25-page-migration-split-view-preview/25-05-SUMMARY.md
- FOUND: commit c3f29b1 (Task 1: SplitView component)
- FOUND: commit 16e5c3f (Task 2: overlay editor rewrite)
