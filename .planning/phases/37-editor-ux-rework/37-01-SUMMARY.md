---
phase: 37-editor-ux-rework
plan: 01
subsystem: ui
tags: [react, typescript, localStorage, collapsible, theme-marketplace, vitest]

requires:
  - phase: 36-events-styling-theme-import
    provides: ThemeMarketplaceModal and useThemeMarketplace hook that ThemeContent extracts from

provides:
  - CollapsibleSection with storageKey prop (per-panel localStorage namespace isolation)
  - CollapsibleSection with defaultOpen prop (initial open state control)
  - ThemeContent inline component (theme grid/filters without modal wrapper)

affects:
  - 37-02 (editor panel restructure that embeds ThemeContent and uses storageKey)

tech-stack:
  added: []
  patterns:
    - "CollapsibleSection storageKey isolates localStorage state between independent panel hierarchies"
    - "ThemeContent as a content-only component — hook + grid/filters — separates display from modal chrome"

key-files:
  created:
    - frontend/src/components/theme-marketplace/ThemeContent.tsx
    - frontend/src/components/theme-marketplace/__tests__/ThemeContent.test.tsx
  modified:
    - frontend/src/components/appearance/CollapsibleSection.tsx
    - frontend/src/components/appearance/__tests__/CollapsibleSection.test.tsx

key-decisions:
  - "storageKey prop defaults to 'appearance-panel-sections-v1' so existing AppearancePanel callers require zero changes"
  - "readStoredSections refactored to accept key parameter; STORAGE_KEY constant remains as default value"
  - "ThemeContent uses single-column grid (not 3-col) for compact sidebar display context"
  - "onApply passed directly to ThemeCard — no intermediate handleApply that calls onClose (no modal to close)"

patterns-established:
  - "ThemeContent pattern: extract hook + content from modal, keeping modal chrome separate"
  - "storageKey pattern: optional prop with backward-compatible default for namespace isolation"

requirements-completed: [EDUX-01, EDUX-02]

duration: 8min
completed: 2026-03-19
---

# Phase 37 Plan 01: Editor UX Rework — Primitives Summary

**CollapsibleSection extended with storageKey/defaultOpen props, and ThemeContent inline component extracted from ThemeMarketplaceModal for direct editor panel embedding**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-19T10:25:00Z
- **Completed:** 2026-03-19T10:33:00Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Extended CollapsibleSection with `storageKey` prop so editor-panel sections and appearance-panel sub-groups each write to their own localStorage key without corrupting each other
- Extended CollapsibleSection with `defaultOpen` prop so Theme and Sources sections can start expanded
- Created `ThemeContent` as an inline component (no modal wrapper, no fixed positioning, no ESC handler, no body scroll lock) ready for direct embedding in the editor panel
- Added 5 new tests for CollapsibleSection prop behaviors; added 4 new tests confirming ThemeContent has no modal wrapper artifacts
- Full unit suite remains green: 143 tests, 23 files, 0 failures

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend CollapsibleSection** - `3d69a75` (feat)
2. **Task 2: Create ThemeContent** - `048a127` (feat)
3. **Task 3: TypeScript compile + auto-fix** - `9d2f29a` (fix)

_Note: Task 3 auto-fixed TypeScript errors in the new test files (missing `filename` field in Theme fixture, `mockImplementation` return type). No pre-existing errors; all errors were introduced by this plan's new code._

## Files Created/Modified

- `frontend/src/components/appearance/CollapsibleSection.tsx` - Extended with `storageKey` and `defaultOpen` props; `readStoredSections` now accepts key parameter
- `frontend/src/components/appearance/__tests__/CollapsibleSection.test.tsx` - 5 new tests for storageKey and defaultOpen behavior; existing tests unchanged
- `frontend/src/components/theme-marketplace/ThemeContent.tsx` - New inline component: ThemeFilters + loading/error/empty states + single-column ThemeCard grid
- `frontend/src/components/theme-marketplace/__tests__/ThemeContent.test.tsx` - 4 tests: no fixed positioning, no dialog role, renders filters, calls onApply

## Decisions Made

- `storageKey` defaults to `'appearance-panel-sections-v1'` so all existing callers remain unchanged
- `ThemeContent` uses single-column grid (`grid-cols-1`) not the 3-column modal grid — optimized for compact sidebar display
- `onApply` passed directly to ThemeCard without an intermediate function (no `onClose` needed — there is no modal)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed TypeScript errors in new test files**
- **Found during:** Task 3 (TypeScript compile check)
- **Issue:** ThemeContent.test.tsx used a Theme fixture missing `filename` field (required by Theme interface). CollapsibleSection.test.tsx `mockImplementation` return type didn't match mock's declared `(key: string) => string` signature (returned `null`)
- **Fix:** Added `filename: 'dark-theme.css'` to Theme fixture; changed `mockImplementation` to return `''` (empty string) instead of `null` and typed explicitly as `string`
- **Files modified:** Both test files
- **Verification:** `npx tsc --noEmit` exits 0; all 11 CollapsibleSection tests and all 4 ThemeContent tests pass
- **Committed in:** `9d2f29a`

---

**Total deviations:** 1 auto-fixed (Rule 1 - type errors in new test code)
**Impact on plan:** Fix necessary for TypeScript correctness. No scope creep.

## Issues Encountered

None beyond the TypeScript type errors noted in Deviations.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02 can now use `storageKey="editor-panel-sections-v1"` on top-level editor CollapsibleSections
- Plan 02 can embed `<ThemeContent onApply={...} />` directly in the editor panel without any modal
- No blockers

---
*Phase: 37-editor-ux-rework*
*Completed: 2026-03-19*
