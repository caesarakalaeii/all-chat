---
phase: 24-component-library-setup-customization
plan: 05
subsystem: ui
tags: [storybook, a11y, wcag, tailwind, next-js, bundle-size, design-tokens]

# Dependency graph
requires:
  - phase: 24-component-library-setup-customization
    provides: Card, Input, Badge, Dialog, Toast, Skeleton, Button components with CVA + design tokens

provides:
  - a11y strict enforcement: Storybook a11y addon in 'error' mode with zero violations across all 34 stories
  - WCAG AA compliance: all component stories pass 4.5:1 contrast minimum
  - Performance budget baseline: Next.js 16 Turbopack build succeeds, 1.1MB total JS chunks (pre-gzip)
  - Human visual verification: awaiting (Task 3 checkpoint)
affects:
  - phase-25-page-redesign (bundle baseline established, a11y gate active for new stories)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "a11y error mode in Storybook preview.ts — violations fail CI, not just show warnings"
    - "Stories for input components require aria-label args when no placeholder is present"
    - "Platform colors in design tokens must meet WCAG AA 4.5:1 against dark backgrounds"

key-files:
  created: []
  modified:
    - frontend/.storybook/preview.ts
    - frontend/src/app/globals.css
    - frontend/src/stories/Input.stories.tsx
    - frontend/src/stories/button.css
    - frontend/src/stories/header.css
    - frontend/src/stories/page.css

key-decisions:
  - "Twitch color lightened from #9146FF to #A37BFF for WCAG AA compliance — original brand hex has 4.03:1 contrast (fails), A37BFF has ~5.4:1 (passes)"
  - "Input stories require explicit aria-label args when no placeholder is present — Storybook story args pass directly to rendered component"
  - "Storybook example stories (Header, Page, Button) CSS updated from #333 dark text to #cccccc light text to match dark-only design system"

patterns-established:
  - "Platform color tokens must target ≥4.5:1 contrast ratio against app dark background (#020204) for use as text"
  - "Input stories without placeholder must include aria-label in story args to satisfy a11y form label requirement"

requirements-completed: [COMP-08]

# Metrics
duration: ~10min (Tasks 1-2 complete, Task 3 awaiting human verify)
completed: 2026-03-10
---

# Phase 24 Plan 05: A11y Enforcement & Performance Budget Summary

**Storybook a11y strict mode enabled — 34/34 tests pass with zero WCAG violations; Next.js 16 Turbopack build succeeds with 1.1MB total JS (pre-gzip) across 36 chunks**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-10T22:15:03Z
- **Completed:** 2026-03-10T22:25:00Z (Tasks 1-2; Task 3 awaiting human verify)
- **Tasks:** 2 of 3 complete (Task 3 is checkpoint:human-verify)
- **Files modified:** 6

## Accomplishments
- Changed Storybook a11y mode from `'todo'` to `'error'` — violations now fail tests
- Fixed 4 categories of a11y violations: Input labels, Twitch badge contrast, Header/Page legacy text contrast
- Lightened Twitch design token from `#9146FF` (4.03:1, fails) to `#A37BFF` (5.4:1, passes WCAG AA)
- All 34 Storybook tests pass across 9 story files — zero a11y violations
- Next.js 16 Turbopack production build succeeds with no errors

## Performance Budget (COMP-08)

**Build tool:** Next.js 16.1.6 with Turbopack

**Build result:** Successful — `✓ Compiled successfully in 9.4s`

**Static bundle inventory (pre-gzip):**
- Total JS chunks: **1,102 KB** across 36 chunk files
- Total CSS: **87 KB** (2 files: main stylesheet + secondary)
- Largest JS chunk: 223 KB (`f92e56fd97640e4b.js`)
- Second largest: 112 KB (`a6dad97d9634a72d.js`)

**Route coverage:** 17 static pages + 8 dynamic routes — all generated successfully

**Budget assessment:**
- No single route is known to exceed 250 KB First Load JS (Next.js 16 Turbopack omits per-route table in output)
- Shared chunk total of 1.1 MB is the baseline for Phase 25 to compare against
- Real-world transfer sizes will be 60-70% smaller after gzip (estimated ~330-385 KB total JS gzipped)

**Warnings (pre-existing, out of scope):**
- `images.domains` deprecated (pre-Phase 24 config issue)
- Workspace root inference warning (multiple lockfiles in repo — pre-existing)

## Task Commits

1. **Task 1: Enable a11y error mode and fix all violations** — `70a5f7a92` (feat)
2. **Task 2: Measure performance budget** — No commit (documentation only, no files changed)
3. **Task 3: Human visual verify** — PENDING (checkpoint:human-verify)

## Files Created/Modified
- `frontend/.storybook/preview.ts` — a11y test mode changed from `'todo'` to `'error'`
- `frontend/src/app/globals.css` — `--color-twitch` lightened `#9146FF` → `#A37BFF` for WCAG AA
- `frontend/src/stories/Input.stories.tsx` — Added `aria-label` to Default, Disabled, Small stories
- `frontend/src/stories/button.css` — Secondary button color `#333` → `#cccccc` (dark theme compat)
- `frontend/src/stories/header.css` — Welcome text color `#333` → `#cccccc` (dark theme compat)
- `frontend/src/stories/page.css` — Page text color `#333` → `#cccccc` (dark theme compat)

## Decisions Made
- **Twitch color adjustment:** The original Twitch brand hex `#9146FF` has 4.03:1 contrast against `#121214` (badge background). WCAG AA minimum for small text is 4.5:1. Changed to `#A37BFF` which provides 5.4:1. This is slightly lighter but still unmistakably Twitch purple and used for both badge text and gradient/glow effects.
- **Story-level aria-label fix:** Input stories without placeholder text fail the `label` a11y rule. Fixed by adding `aria-label` directly in story args — these pass through to the rendered component and represent valid accessible usage patterns.
- **Example story CSS fix:** Storybook's default example stories (`Header`, `Page`, `Button`) use `color: #333` which is near-invisible on our dark `#020204` background. Updated to `#cccccc` — these are example/demo stories only, not part of the UI component library.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Twitch color fails WCAG AA contrast in badge context**
- **Found during:** Task 1 (a11y error mode enabled)
- **Issue:** `#9146FF` has 4.03:1 contrast ratio against `#121214` badge background — below 4.5:1 minimum
- **Fix:** Updated `--color-twitch` in `globals.css` from `#9146FF` to `#A37BFF` (same hue, lighter value)
- **Files modified:** `frontend/src/app/globals.css`
- **Verification:** All 8 Badge stories now pass including Twitch, Twitch (sm), All Platforms, All Platforms (sm)
- **Committed in:** `70a5f7a92` (Task 1 commit)

**2. [Rule 2 - Missing Critical] Input stories missing required aria-label**
- **Found during:** Task 1 (a11y error mode enabled)
- **Issue:** Default, Disabled, and Small Input stories had no label or aria-label — fails `label` a11y rule
- **Fix:** Added `aria-label` prop to story args (e.g. `'aria-label': 'Text input'`) — demonstrates accessible usage
- **Files modified:** `frontend/src/stories/Input.stories.tsx`
- **Verification:** All 4 Input stories now pass (Default, Disabled, WithPlaceholder already passed, Small now passes)
- **Committed in:** `70a5f7a92` (Task 1 commit)

**3. [Rule 1 - Bug] Storybook example story CSS uses dark text on dark background**
- **Found during:** Task 1 (a11y error mode enabled)
- **Issue:** `button.css`, `header.css`, `page.css` use `color: #333` which renders as near-invisible dark text on our `#020204` dark background
- **Fix:** Changed all three to `color: #cccccc` — light gray with sufficient contrast
- **Files modified:** `frontend/src/stories/button.css`, `frontend/src/stories/header.css`, `frontend/src/stories/page.css`
- **Verification:** Header stories (2 tests) and Page stories (2 tests) now pass
- **Committed in:** `70a5f7a92` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (1 bug, 1 missing critical, 1 bug)
**Impact on plan:** All auto-fixes required for a11y compliance. Twitch color change is minimal (same hue, slightly lighter) and preserves design intent. No scope creep.

## Issues Encountered
- Next.js 16 Turbopack build output does not include per-route "First Load JS" table (removed in this version). Bundle sizes measured from `.next/static/chunks/` directory instead. This is a known change in Next.js 16.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Phase 24 complete pending human visual verification (Task 3)
- Phase 25 (page redesign) can begin once human visual sign-off is given
- Bundle baseline established: 1.1MB total JS pre-gzip, ~330-385KB estimated gzipped
- a11y error mode is now active — any new stories in Phase 25 must pass WCAG AA

---
*Phase: 24-component-library-setup-customization*
*Completed: 2026-03-10*
