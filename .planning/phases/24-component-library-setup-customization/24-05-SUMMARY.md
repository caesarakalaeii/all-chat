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
  - Human visual verification: approved — all 7 visual checks passed
affects:
  - phase-25-page-redesign (bundle baseline established, a11y gate active for new stories)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "a11y error mode in Storybook preview.ts — violations fail CI, not just show warnings"
    - "Stories for input components require aria-label args when no placeholder is present"
    - "Platform colors in design tokens must meet WCAG AA 4.5:1 against dark backgrounds"
    - "Design token class names must be resolvable by Tailwind JIT — use defined token names (border-l-youtube not border-l-destructive if destructive is not in @theme)"

key-files:
  created: []
  modified:
    - frontend/.storybook/preview.ts
    - frontend/src/app/globals.css
    - frontend/src/components/ui/toast.tsx
    - frontend/src/stories/Input.stories.tsx
    - frontend/src/stories/Toast.stories.tsx
    - frontend/src/stories/button.css
    - frontend/src/stories/header.css
    - frontend/src/stories/page.css

key-decisions:
  - "Twitch color lightened from #9146FF to #A37BFF for WCAG AA compliance — original brand hex has 4.03:1 contrast (fails), A37BFF has ~5.4:1 (passes)"
  - "Input stories require explicit aria-label args when no placeholder is present — Storybook story args pass directly to rendered component"
  - "Storybook example stories (Header, Page, Button) CSS updated from #333 dark text to #cccccc light text to match dark-only design system"
  - "Toast border colors use defined @theme tokens: border-l-youtube (red) for error, border-l-tiktok (teal) for info — border-l-destructive and border-l-ring are not defined in the custom token system"

patterns-established:
  - "Platform color tokens must target ≥4.5:1 contrast ratio against app dark background (#020204) for use as text"
  - "Input stories without placeholder must include aria-label in story args to satisfy a11y form label requirement"
  - "When using Tailwind classes for color, verify the token name exists in @theme — undefined tokens silently produce no styling"

requirements-completed: [COMP-08]

# Metrics
duration: ~15min (including human visual verify session)
completed: 2026-03-11
---

# Phase 24 Plan 05: A11y Enforcement & Performance Budget Summary

**Storybook a11y strict mode enabled with 34/34 tests passing; human-verified micro-interactions, gradient CTA, and platform glow dots; Phase 24 component library fully complete**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-10T22:15:03Z
- **Completed:** 2026-03-11T18:33:00Z
- **Tasks:** 3 of 3 complete (including human visual verification)
- **Files modified:** 8

## Accomplishments
- Changed Storybook a11y mode from `'todo'` to `'error'` — violations now fail CI
- Fixed 4 categories of a11y violations: Input labels, Twitch badge contrast, Header/Page legacy text contrast
- Lightened Twitch design token from `#9146FF` (4.03:1, fails) to `#A37BFF` (5.4:1, passes WCAG AA)
- Fixed Toast border color tokens: `border-l-destructive` and `border-l-ring` were undefined — replaced with `border-l-youtube` (red) and `border-l-tiktok` (teal)
- All 34 Storybook tests pass across 9 story files with zero a11y violations
- Next.js 16 Turbopack production build succeeds with no errors
- Human visual sign-off obtained: all 7 visual checks approved

## Visual Verification Results (Task 3)

All 7 checks passed — approved by human on 2026-03-11:

1. **Card micro-interaction** — renders correctly with hover scale, Interactive and With Content stories confirmed
2. **Button gradient** — `#A37BFF` (Twitch purple) → `#69C9D0` (TikTok teal) left-to-right gradient confirmed
3. **Platform badges** — all 4 platforms (Twitch purple, YouTube red, Kick green, TikTok teal) with correct glow dots
4. **Input focus ring** — bright white border appears on click/focus
5. **Dialog** — frosted glass panel, centered, title/description/buttons/close X all present
6. **Toast colors** — left border color coding correct after fix: green (success), red (error via `border-l-youtube`), teal (info via `border-l-tiktok`)
7. **Skeleton pulse** — renders as dark shape on dark background (correct for overlay use case)

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
- Shared chunk total of 1.1 MB is the Phase 25 baseline for comparison
- Real-world transfer sizes will be 60-70% smaller after gzip (estimated ~330-385 KB total JS gzipped)

**Warnings (pre-existing, out of scope):**
- `images.domains` deprecated (pre-Phase 24 config issue)
- Workspace root inference warning (multiple lockfiles in repo — pre-existing)

## Task Commits

1. **Task 1: Enable a11y error mode and fix all violations** — `70a5f7a92` (feat)
2. **Task 2: Measure performance budget** — No commit (documentation only, no files changed)
3. **Task 3: Human visual verify + Toast fix** — `7932b3bfd` (fix, applied during verification session)

**Plan metadata:** `de33a132e` (docs: complete plan — initial), updated in final commit below

## Files Created/Modified
- `frontend/.storybook/preview.ts` — a11y test mode changed from `'todo'` to `'error'`
- `frontend/src/app/globals.css` — `--color-twitch` lightened `#9146FF` → `#A37BFF` for WCAG AA
- `frontend/src/components/ui/toast.tsx` — border colors fixed: `border-l-destructive` → `border-l-youtube`, `border-l-ring` → `border-l-tiktok`
- `frontend/src/stories/Input.stories.tsx` — Added `aria-label` to Default, Disabled, Small stories
- `frontend/src/stories/Toast.stories.tsx` — Updated to reflect corrected token names
- `frontend/src/stories/button.css` — Secondary button color `#333` → `#cccccc` (dark theme compat)
- `frontend/src/stories/header.css` — Welcome text color `#333` → `#cccccc` (dark theme compat)
- `frontend/src/stories/page.css` — Page text color `#333` → `#cccccc` (dark theme compat)

## Decisions Made
- **Twitch color adjustment:** The original Twitch brand hex `#9146FF` has 4.03:1 contrast against `#121214` (badge background). WCAG AA minimum for small text is 4.5:1. Changed to `#A37BFF` which provides ~5.4:1. Still unmistakably Twitch purple.
- **Story-level aria-label fix:** Input stories without placeholder text fail the `label` a11y rule. Fixed by adding `aria-label` directly in story args — passes through to rendered component and represents accessible usage.
- **Example story CSS fix:** Storybook's default example stories use `color: #333` which is near-invisible on `#020204` dark background. Updated to `#cccccc`.
- **Toast token correction:** `border-l-destructive` and `border-l-ring` are not defined in the custom `@theme` token system — they are shadcn defaults that were removed in Phase 23. Use `border-l-youtube` (red) for error and `border-l-tiktok` (teal) for info, which are defined tokens with the correct semantic colors.

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
- **Fix:** Added `aria-label` prop to story args — demonstrates accessible usage pattern
- **Files modified:** `frontend/src/stories/Input.stories.tsx`
- **Verification:** All 4 Input stories now pass
- **Committed in:** `70a5f7a92` (Task 1 commit)

**3. [Rule 1 - Bug] Storybook example story CSS uses dark text on dark background**
- **Found during:** Task 1 (a11y error mode enabled)
- **Issue:** `button.css`, `header.css`, `page.css` use `color: #333` — near-invisible on `#020204` dark background
- **Fix:** Changed all three to `color: #cccccc`
- **Files modified:** `frontend/src/stories/button.css`, `frontend/src/stories/header.css`, `frontend/src/stories/page.css`
- **Verification:** Header (2) and Page (2) stories now pass
- **Committed in:** `70a5f7a92` (Task 1 commit)

**4. [Rule 1 - Bug] Toast border colors used undefined Tailwind tokens**
- **Found during:** Task 3 (human visual verification — border colors not visible)
- **Issue:** `border-l-destructive` and `border-l-ring` are shadcn defaults removed in Phase 23; no styling applied
- **Fix:** Replaced with `border-l-youtube` (red, error) and `border-l-tiktok` (teal, info) — defined in `@theme`
- **Files modified:** `frontend/src/components/ui/toast.tsx`, `frontend/src/stories/Toast.stories.tsx`
- **Verification:** Human confirmed correct left border colors in all 3 Toast stories
- **Committed in:** `7932b3bfd`

---

**Total deviations:** 4 auto-fixed (2 bugs from a11y scan, 1 missing critical, 1 bug found in visual verify)
**Impact on plan:** All auto-fixes required for correctness and a11y compliance. No scope creep.

## Issues Encountered
- Next.js 16 Turbopack build output does not include per-route "First Load JS" table (removed in this version). Bundle sizes measured from `.next/static/chunks/` directory instead.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Phase 24 fully complete — all 5 plans executed, all COMP-0X requirements satisfied
- Phase 25 (page redesign) can begin immediately
- Bundle baseline established: 1.1MB total JS pre-gzip, ~330-385KB estimated gzipped
- a11y error mode active — new Phase 25 stories must pass WCAG AA
- Toast, Dialog, Badge, Input, Card, Button, Skeleton all production-ready with verified visual appearance

## Self-Check: PASSED
- `frontend/.storybook/preview.ts` — FOUND, contains `test: 'error'`
- `frontend/src/app/globals.css` — FOUND, contains `#A37BFF`
- `frontend/src/components/ui/toast.tsx` — FOUND, Toast fix applied
- `24-05-SUMMARY.md` — FOUND
- Commit `70a5f7a92` — FOUND
- Commit `7932b3bfd` — FOUND
- 34/34 tests pass post-fix

---
*Phase: 24-component-library-setup-customization*
*Completed: 2026-03-11*
