---
phase: 29-viewer-color-gradient-editor
plan: 03
subsystem: ui
tags: [react, tailwind, gradient, css, chrome-extension, typescript, vitest]

# Dependency graph
requires:
  - phase: 29-01
    provides: "NameGradient type, buildGradientCSS utility, name_gradient on UserInfo"

provides:
  - "Overlay page.tsx username span branched on name_gradient using bg-clip-text text-transparent"
  - "getUsernameSpanProps helper function for testable gradient branching logic"
  - "3 vitest unit tests for overlay gradient render behavior"
  - "Extension LocalStorage viewer_name_gradient field"
  - "Extension getNameGradient / setNameGradient storage helpers"
  - "Extension SAVE_NAME_GRADIENT message type and service-worker handler"
  - "Extension ChatContainer gradient branch on viewer's own username"

affects:
  - 29-viewer-color-gradient-editor

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "getUsernameSpanProps: extract rendering logic to a pure helper for testability"
    - "bg-clip-text text-transparent + backgroundImage for pure CSS gradient text"
    - "useMemo for JSON.parse of gradient string — prevents parse on every render"
    - "Inlined buildGradientCSS in extension (separate repo, no shared lib)"

key-files:
  created:
    - "frontend/src/lib/utils/usernameSpan.ts"
    - "frontend/src/app/overlay/__tests__/gradient-render.test.tsx"
  modified:
    - "frontend/src/app/overlay/[id]/page.tsx"
    - "all-chat-extension/src/lib/types/extension.ts"
    - "all-chat-extension/src/lib/storage.ts"
    - "all-chat-extension/src/ui/components/ChatContainer.tsx"
    - "all-chat-extension/src/background/service-worker.ts"

key-decisions:
  - "Extracted getUsernameSpanProps pure helper to enable unit testing without DOM/JSX rendering in node environment"
  - "Gradient branch applies to all users in overlay (any message.user.name_gradient), but only viewer's own messages in extension (viewerInfo username match)"
  - "SAVE_NAME_GRADIENT in service-worker clears viewer_name_color for mutual exclusion"

patterns-established:
  - "TDD for rendering branches: extract pure prop-computing functions to test className/style outputs without React DOM"

requirements-completed: [VID-02, PREM-02, WEB-05]

# Metrics
duration: 8min
completed: 2026-03-15
---

# Phase 29 Plan 03: Gradient Render Branch Summary

**CSS bg-clip-text gradient branch on overlay username span and extension ChatContainer, with 3 vitest unit tests via extracted getUsernameSpanProps helper**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-15T21:39:57Z
- **Completed:** 2026-03-15T21:47:30Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Overlay `page.tsx` now branches on `message.user.name_gradient`: renders `bg-clip-text text-transparent` + `backgroundImage` when gradient set, falls back to inline `color` style otherwise
- 3 vitest unit tests pass GREEN for gradient render behavior (no DOM, pure helper function tested)
- Extension `LocalStorage` extended with `viewer_name_gradient`, `ExtensionMessage` extended with `SAVE_NAME_GRADIENT`, `getNameGradient`/`setNameGradient` helpers added, service-worker handles the new message, `ChatContainer` renders gradient on viewer's own username

## Task Commits

Each task was committed atomically:

1. **Task 1: Overlay gradient render branch + test scaffold** - `12f8842` (feat/TDD)
2. **Task 2: Extension types, storage helpers, and ChatContainer gradient branch** - `edb56eb` (feat, in extension repo)

## Files Created/Modified

- `frontend/src/lib/utils/usernameSpan.ts` - Pure helper: `getUsernameSpanProps(user)` returns `{ className, style }` branched on `name_gradient`
- `frontend/src/app/overlay/__tests__/gradient-render.test.tsx` - 3 unit tests: flat color absent, bg-clip-text present, backgroundImage contains linear-gradient
- `frontend/src/app/overlay/[id]/page.tsx` - Username span replaced with two-branch conditional; `buildGradientCSS` imported
- `all-chat-extension/src/lib/types/extension.ts` - `viewer_name_gradient` in `LocalStorage`; `SAVE_NAME_GRADIENT` in `ExtensionMessage`
- `all-chat-extension/src/lib/storage.ts` - `getNameGradient`, `setNameGradient` exported helpers
- `all-chat-extension/src/ui/components/ChatContainer.tsx` - `viewerNameGradient` state, `parsedGradient` useMemo, gradient branch on username span
- `all-chat-extension/src/background/service-worker.ts` - `SAVE_NAME_GRADIENT` case: persists gradient, clears name_color on non-null save

## Decisions Made

- **Extracted pure helper for TDD**: Because the unit test environment is `node` (no DOM), the gradient branching logic was extracted into `getUsernameSpanProps` in `usernameSpan.ts`. This makes the branching logic directly testable without rendering the full overlay page component.
- **Extension gradient scoped to viewer's own messages**: Unlike the overlay (which applies any message's `name_gradient`), the extension only applies the locally-stored gradient to messages from the logged-in viewer's username. This is correct — the extension has no server-delivered gradient data for other users.
- **Inlined buildGradientCSS in extension**: The extension is a separate repo without access to `@/lib/utils/gradient`. The helper is a one-liner, so inlining avoids cross-repo coupling.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Pre-existing TypeScript errors in `frontend/src/app/settings/viewer/__tests__/page.test.tsx` (4 TS2339 errors on `_store` property) — confirmed pre-existing before this plan's changes, out of scope per deviation rules.

## Next Phase Readiness

- Plan 01, 02, 03 of Phase 29 are complete: types + DB migration, frontend editor UI, and gradient render branching all delivered
- Requirements VID-02, PREM-02, WEB-05 fulfilled
- No blockers

---
*Phase: 29-viewer-color-gradient-editor*
*Completed: 2026-03-15*
