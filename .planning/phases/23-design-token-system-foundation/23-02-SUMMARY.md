---
phase: 23-design-token-system-foundation
plan: 02
subsystem: ui
tags: [tailwind, design-tokens, platform-colors, tailwind-v4, vitest, typescript]

# Dependency graph
requires:
  - phase: 23-design-token-system-foundation
    provides: "Design token system in globals.css and Tailwind v4 config (plan 01)"
provides:
  - "PLATFORM_COLORS static map in frontend/src/lib/platform-colors.ts"
  - "Platform type export for type-safe platform lookups"
  - "Unit test project in vitest.config.ts"
  - "10 passing unit tests for PLATFORM_COLORS"
  - "Tailwind v4 gradient class migration across 4 files"
affects:
  - 23-design-token-system-foundation
  - 25-page-migration

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Static platform color map: use PLATFORM_COLORS[platform as Platform]?.text ?? PLATFORM_COLORS.system.text"
    - "Vitest unit test project separate from storybook browser tests"

key-files:
  created:
    - frontend/src/lib/platform-colors.ts
    - frontend/src/lib/__tests__/platform-colors.test.ts
  modified:
    - frontend/src/components/theme-marketplace/ThemePreview.tsx
    - frontend/src/app/overlay/[id]/credits/page.tsx
    - frontend/src/app/page.tsx
    - frontend/src/components/legal/LegalLayout.tsx
    - frontend/src/components/theme-marketplace/CreditRollThemePreview.tsx
    - frontend/vitest.config.ts

key-decisions:
  - "Static PLATFORM_COLORS map uses complete literal class strings for Tailwind JIT safety — no dynamic concatenation like 'text-' + platform"
  - "Added unit test project to vitest.config.ts alongside storybook project (non-browser, node environment)"

patterns-established:
  - "Platform color lookup: PLATFORM_COLORS[msg.platform as Platform]?.text ?? PLATFORM_COLORS.system.text"
  - "All bg-gradient-to-* must be bg-linear-to-* in Tailwind v4"

requirements-completed: [FOUND-03, FOUND-05]

# Metrics
duration: 4min
completed: 2026-03-10
---

# Phase 23 Plan 02: Platform Colors Static Map & Gradient Migration Summary

**JIT-safe PLATFORM_COLORS static map with 10 unit tests, ThemePreview.tsx migrated off getPlatformColor(), and 7 bg-gradient-to-* classes replaced with bg-linear-to-* across 4 files**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-10T20:26:48Z
- **Completed:** 2026-03-10T20:30:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- Created `PLATFORM_COLORS` static map with complete literal Tailwind class strings for all 4 platforms + system fallback, ensuring Tailwind JIT can statically analyze them
- Wrote 10 unit tests using vitest (required adding a unit test project to vitest.config.ts, which previously only had storybook)
- Replaced the broken `getPlatformColor()` function in ThemePreview.tsx that returned shadcn-era tokens like `text-purple-400` with the new static map
- Migrated all 7 `bg-gradient-to-*` occurrences to `bg-linear-to-*` across 4 files (Tailwind v4 renamed this utility)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create platform-colors.ts with unit tests (TDD)** - `79861b1ee` (feat)
2. **Task 2: Migrate ThemePreview.tsx to use PLATFORM_COLORS static map** - `3b53c04dd` (feat)
3. **Task 3: Migrate bg-gradient-to-* to bg-linear-to-*** - `0072d4ff6` (fix)

## Files Created/Modified

- `frontend/src/lib/platform-colors.ts` - PLATFORM_COLORS const and Platform type export
- `frontend/src/lib/__tests__/platform-colors.test.ts` - 10 unit tests for all platforms
- `frontend/vitest.config.ts` - Added unit test project (node environment, non-browser)
- `frontend/src/components/theme-marketplace/ThemePreview.tsx` - Removed getPlatformColor(), uses PLATFORM_COLORS
- `frontend/src/app/overlay/[id]/credits/page.tsx` - 3x bg-gradient-to-b -> bg-linear-to-b
- `frontend/src/app/page.tsx` - bg-gradient-to-br, bg-gradient-to-r migrated
- `frontend/src/components/legal/LegalLayout.tsx` - bg-gradient-to-b migrated
- `frontend/src/components/theme-marketplace/CreditRollThemePreview.tsx` - bg-gradient-to-b migrated

## Decisions Made

- Added a dedicated `unit` vitest project (node environment) to vitest.config.ts. The existing config only had a storybook project requiring a browser, making it unsuitable for pure unit tests.
- No new npm dependencies needed — platform-colors.ts is pure TypeScript constants.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added unit test project to vitest.config.ts**
- **Found during:** Task 1 (TDD RED phase)
- **Issue:** vitest.config.ts only contained a storybook browser project — running `npx vitest run src/lib/__tests__/platform-colors.test.ts` returned "No test files found" because the include patterns only matched storybook stories
- **Fix:** Added a `unit` project with `environment: 'node'`, `include` pattern for `src/**/__tests__/**/*.test.ts`, and `@/` alias pointing to `src/`
- **Files modified:** frontend/vitest.config.ts
- **Verification:** Tests found and ran successfully after fix
- **Committed in:** `79861b1ee` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking infrastructure issue)
**Impact on plan:** Necessary for test infrastructure to work. No scope creep — the fix enabled the planned TDD workflow.

## Issues Encountered

- `getPlatformColor` still exists in 5 other files (admin/overlays, admin/sources, overlay/[id]/page, overlays/[id]/preview, overlays/[id]/page) — these are pre-existing occurrences in files outside this plan's scope. Deferred to Phase 25 all-page migration.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- PLATFORM_COLORS is ready for use in Phase 25 page migrations
- Pattern established: always use `PLATFORM_COLORS[platform as Platform]?.text ?? PLATFORM_COLORS.system.text`
- Remaining getPlatformColor() instances in 5 other files will be migrated in Phase 25 as part of the full page migration sweep

---
*Phase: 23-design-token-system-foundation*
*Completed: 2026-03-10*
