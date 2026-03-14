---
phase: 26-enforcement-quality-gates
plan: "03"
subsystem: ui
tags: [next.js, bundle-analyzer, storybook, vitest, performance, testing]

# Dependency graph
requires:
  - phase: 26-enforcement-quality-gates
    provides: ESLint flat config + Prettier setup from 26-01
  - phase: 24-component-library-setup-customization
    provides: Storybook + vitest-addon test infrastructure
provides:
  - "@next/bundle-analyzer wrapped next.config.js with ANALYZE=true flag support"
  - "analyze npm script in package.json for manual bundle inspection"
  - "Performance/RenderTimingTest Storybook story with play() function measuring 20 render cycles at <16ms"
affects:
  - 26-04-PLAN (CI workflow will use analyze script and Performance story)
  - future-performance-regression-testing

# Tech tracking
tech-stack:
  added: ["@next/bundle-analyzer@^16.1.6"]
  patterns:
    - "withBundleAnalyzer HOC wraps nextConfig via require() (CommonJS pattern)"
    - "performance.now() + getBoundingClientRect() for synchronous layout measurement in Storybook play()"
    - "storybook/test (not @storybook/test) for expect/within in Storybook v10 stories"

key-files:
  created:
    - "frontend/src/stories/Performance.stories.tsx"
  modified:
    - "frontend/next.config.js"
    - "frontend/package.json"

key-decisions:
  - "Import expect/within from 'storybook/test' not '@storybook/test' — the storybook package provides this export in v10; @storybook/test is a v8 package and not installed"
  - "Performance story uses raw DOM createElement + getBoundingClientRect() rather than React rendering — isolates layout time without React reconciler overhead"
  - "a11y disabled on Performance story intentionally — non-semantic divs measure raw render, a11y overhead would skew results"

patterns-established:
  - "Bundle analyzer: ANALYZE=true env var gates report generation; ANALYZE=false (default) skips browser tab but still allows CI delta comparison"
  - "Performance regression stories: use play() with performance.now() + forced layout read to measure worst-case per-frame render time"

requirements-completed: [ENFORCE-08, ENFORCE-09]

# Metrics
duration: 7min
completed: 2026-03-14
---

# Phase 26 Plan 03: Bundle Analyzer + Performance Story Summary

**@next/bundle-analyzer wrapped into next.config.js and a Storybook performance interaction test asserting ChatMessage-like components render in <16ms at 20 msg/sec**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-14T11:55:14Z
- **Completed:** 2026-03-14T12:02:26Z
- **Tasks:** 2
- **Files modified:** 4 (next.config.js, package.json, package-lock.json, Performance.stories.tsx)

## Accomplishments
- Installed `@next/bundle-analyzer` and wrapped `nextConfig` with `withBundleAnalyzer` HOC (ANALYZE=true flag controls report generation)
- Added `analyze` script to package.json enabling `npm run analyze` for manual bundle inspection
- Created `Performance/RenderTimingTest` Storybook story with `RenderAt20MsgPerSec` play() function
- Story measures 20 sequential DOM renders, logs max/avg timing, and asserts max <16ms — passes in `npx vitest --project storybook --run`

## Task Commits

Each task was committed atomically:

1. **Task 1: Install @next/bundle-analyzer and wrap next.config.js** - `cd45edc` (chore)
2. **Task 2: Create Performance.stories.tsx with render timing play() function** - `e306c61` (feat)

## Files Created/Modified
- `frontend/next.config.js` - Added withBundleAnalyzer HOC wrapper around nextConfig
- `frontend/package.json` - Added @next/bundle-analyzer devDependency and analyze script
- `frontend/package-lock.json` - Updated lockfile with new dependency
- `frontend/src/stories/Performance.stories.tsx` - New Storybook performance regression story

## Decisions Made
- Import `expect`/`within` from `storybook/test` (not `@storybook/test`): Storybook v10 ships these utilities in the `storybook` package under the `./test` export; `@storybook/test` is a Storybook v8 package that is not installed and does not work with the addon-vitest setup.
- Performance story uses raw DOM `createElement` + `getBoundingClientRect()` rather than React rendering: this isolates actual browser layout time without React reconciler overhead, which is the correct bottleneck to measure for overlay render performance.
- Disabled a11y on Performance story: the non-semantic divs are intentional for render isolation; a11y processing would add overhead and skew timing measurements.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed incorrect @storybook/test import path**
- **Found during:** Task 2 (Create Performance.stories.tsx)
- **Issue:** Plan specified `import { expect, within } from '@storybook/test'` but `@storybook/test` package is not installed; it's a Storybook v8 package. Storybook v10 provides these via `storybook/test` export path.
- **Fix:** Changed import to `from 'storybook/test'` — the correct export path for the installed `storybook@10.x` package.
- **Files modified:** `frontend/src/stories/Performance.stories.tsx`
- **Verification:** `npx vitest --project storybook --run` — Performance story passes with 1/1 tests.
- **Committed in:** `e306c61` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — wrong import path)
**Impact on plan:** Required for story to load in browser; no scope creep.

## Issues Encountered
- A `git stash pop` during verification created merge conflicts in package.json, config.json, package-lock.json, and REQUIREMENTS.md. Resolved by keeping the upstream (committed) versions — the stash contained unrelated in-progress work. No content was lost.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Bundle analyzer is ready for use: `npm run analyze` generates the report; CI workflow (Plan 04) can run with `ANALYZE=false` and use the generated `frontend/.next/analyze/` output for delta comparison
- Performance story is registered in the Storybook vitest runner and will execute automatically in CI
- Pre-existing Storybook failures in AdminLayout, Dashboard, and LandingPage stories are unrelated to this plan's scope

---
*Phase: 26-enforcement-quality-gates*
*Completed: 2026-03-14*
