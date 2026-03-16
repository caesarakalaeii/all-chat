---
phase: 31-all-chat-platform-badges
plan: "02"
subsystem: ui
tags: [react, typescript, vitest, badges, overlay, twitch, youtube, kick]

# Dependency graph
requires:
  - phase: 31-all-chat-platform-badges
    provides: Phase context and research for badge rendering approach

provides:
  - AllChatBadge React component wrapping animated InfinityLogo at inline badge size
  - PremiumBadge React component with inline purple gem SVG
  - badgeOrder.ts extended with allchat:-2 and premium:-1 ROLE_PRIORITIES
  - Vitest tests asserting allchat/premium sort order
  - Overlay page badge render blocks updated to 3-way name-check pattern

affects:
  - overlay rendering (AllChatBadge/PremiumBadge display in OBS overlays)
  - badge sort order (all-chat platform badges appear before moderator/vip/broadcaster)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - 3-way badge name-check pattern (allchat/premium/icon_url/span fallback)
    - SVG <title> child element for accessible tooltip on SVG badges
    - Negative ROLE_PRIORITIES values for platform-level badge priority

key-files:
  created:
    - frontend/src/components/AllChatBadge.tsx
    - frontend/src/components/PremiumBadge.tsx
    - frontend/src/lib/__tests__/badgeOrder.test.ts
  modified:
    - frontend/src/lib/badgeOrder.ts
    - frontend/src/app/overlay/[id]/page.tsx

key-decisions:
  - "Test file placed in src/lib/__tests__/ (not src/lib/) to match vitest project include pattern src/**/__tests__/**/*.test.ts"
  - "PremiumBadge uses SVG <title> child element (not HTML title attr) — SVGProps<SVGSVGElement> does not include title as prop"
  - "Image badge sizing changed from w-4 h-4 (fixed 16px) to h-[1em] w-auto (responsive to overlay font-size setting)"
  - "AllChatBadge is 'use client' because it wraps the animated InfinityLogo which uses requestAnimationFrame"

patterns-established:
  - "Platform-level badge components: inline SVG or client component wrapped in <span> with aria-label and optional <title>"
  - "Badge render block: 3-way name-check before icon_url check — component badges never have icon_url"

requirements-completed:
  - BADGE-01
  - BADGE-02
  - BADGE-03
  - BADGE-04

# Metrics
duration: 2min
completed: 2026-03-16
---

# Phase 31 Plan 02: All-Chat Platform Badges - Frontend Components Summary

**AllChatBadge (InfinityLogo wrapper) and PremiumBadge (inline gem SVG) rendered in overlay with 3-way name-check, h-[1em] responsive sizing, and sort priorities allchat:-2/premium:-1**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-16T08:32:43Z
- **Completed:** 2026-03-16T08:35:10Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Created AllChatBadge component wrapping animated InfinityLogo at size=18 default
- Created PremiumBadge component with inline purple gem SVG using aria-accessible title child
- Extended badgeOrder.ts ROLE_PRIORITIES with allchat:-2 and premium:-1 so platform badges sort first
- Added 5 Vitest tests covering all sort order assertions
- Updated both overlay page badge render blocks with 3-way name-check pattern and h-[1em] responsive sizing

## Task Commits

Each task was committed atomically:

1. **Task 1: AllChatBadge/PremiumBadge components + badgeOrder extension** - `f3bcf36` (feat)
2. **Task 2: badgeOrder tests + overlay page badge render update** - `7013ddf` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `frontend/src/components/AllChatBadge.tsx` - 'use client' component wrapping InfinityLogo, exported AllChatBadge with size/title props
- `frontend/src/components/PremiumBadge.tsx` - Pure SVG purple gem badge, exported PremiumBadge with size/title props
- `frontend/src/lib/badgeOrder.ts` - Added allchat:-2 and premium:-1 to ROLE_PRIORITIES
- `frontend/src/lib/__tests__/badgeOrder.test.ts` - 5 tests: allchat/premium before moderator, allchat before premium, allchat before subscriber, combined order
- `frontend/src/app/overlay/[id]/page.tsx` - Added AllChatBadge/PremiumBadge imports, updated both badge render blocks with 3-way name-check

## Decisions Made

- Test file placed in `src/lib/__tests__/` not `src/lib/` — vitest config `include` pattern requires `__tests__` directory
- PremiumBadge uses `<title>` SVG child element rather than HTML `title` attribute — TypeScript `SVGProps<SVGSVGElement>` does not include `title` as a prop
- Image badge sizing changed from `w-4 h-4` (fixed 16px) to `h-[1em] w-auto` to scale with overlay font size setting
- AllChatBadge requires `'use client'` because InfinityLogo uses `requestAnimationFrame` via `useEffect`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] PremiumBadge SVG title attribute type error**
- **Found during:** Task 1 (TypeScript compile verification)
- **Issue:** Plan specified `title={title}` as JSX attribute on `<svg>` but `SVGProps<SVGSVGElement>` does not accept `title` as a prop — TypeScript error TS2322
- **Fix:** Changed to `{title && <title>{title}</title>}` as SVG child element (standard SVG accessible pattern)
- **Files modified:** `frontend/src/components/PremiumBadge.tsx`
- **Verification:** `npx tsc --noEmit` passes with no errors
- **Committed in:** `f3bcf36` (Task 1 commit)

**2. [Rule 3 - Blocking] Test file path moved to __tests__ subdirectory**
- **Found during:** Task 2 (running Vitest)
- **Issue:** Plan specified `frontend/src/lib/badgeOrder.test.ts` but vitest config unit project include pattern is `src/**/__tests__/**/*.test.ts` — test file not discovered at top-level lib directory
- **Fix:** Created file at `frontend/src/lib/__tests__/badgeOrder.test.ts` with relative imports
- **Files modified:** `frontend/src/lib/__tests__/badgeOrder.test.ts`
- **Verification:** `npx vitest run --project unit src/lib/__tests__/badgeOrder` — 5/5 tests pass
- **Committed in:** `7013ddf` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes necessary for TypeScript correctness and test discoverability. No scope creep.

## Issues Encountered

None beyond the two deviations documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- AllChatBadge and PremiumBadge components ready for use in any frontend component
- Badge sort order ensures platform badges appear first in all overlay configurations
- Plan 31-03 can proceed (backend badge assignment / message enricher integration)

---
*Phase: 31-all-chat-platform-badges*
*Completed: 2026-03-16*

## Self-Check: PASSED

All files and commits verified present.
