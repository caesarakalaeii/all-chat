---
phase: 30-avatar-frame-flair-system
plan: "04"
subsystem: ui
tags: [react, nextjs, typescript, avatar, cosmetics, extension]

# Dependency graph
requires:
  - phase: 30-02
    provides: backend CRUD endpoints for frames/flairs catalog and PATCH /auth/viewer/cosmetics
  - phase: 30-03
    provides: enricher injecting avatar_frame_url and avatar_flair_url into ChatMessage
provides:
  - UserAvatar composite component (monorepo frontend and extension)
  - AvatarCosmeticsCard on settings/viewer with catalog fetch, grid, live preview, Save
  - /admin/cosmetics page with Frames/Flairs tabs, add form, delete buttons
  - AdminNav Cosmetics link
  - Overlay /overlay/[id] uses UserAvatar replacing old avatar block
  - Extension ChatContainer renders UserAvatar with frame/flair support
affects: [overlay rendering, extension chat display, viewer settings, admin tooling]

# Tech tracking
tech-stack:
  added: []
  patterns: [UserAvatar composite stacking (avatar + frame centered 1.4x + flair bottom-right 0.4x), TDD with jsdom vitest environment annotation for React components, admin tab pattern with fetch/add/delete per tab]

key-files:
  created:
    - frontend/src/components/UserAvatar.tsx
    - frontend/src/components/__tests__/UserAvatar.test.tsx
    - frontend/src/app/admin/cosmetics/page.tsx
    - all-chat-extension/src/ui/components/UserAvatar.tsx
  modified:
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/app/settings/viewer/page.tsx
    - frontend/src/components/AdminNav.tsx
    - all-chat-extension/src/ui/components/ChatContainer.tsx

key-decisions:
  - "Extension UserAvatar uses inline styles (not Tailwind classes) since extension doesn't use Tailwind globals"
  - "// @vitest-environment jsdom annotation on test file rather than changing global vitest config environment — avoids breaking existing node-environment unit tests"
  - "Badge variant prop removed — project Badge component has only size variant, not default/outline/destructive variants"
  - "Extension ChatContainer adds UserAvatar before platform icon — visible even without avatar_url (shows initials fallback)"
  - "Admin cosmetics page reuses AdminNav for consistent navigation header"

patterns-established:
  - "UserAvatar composite pattern: outer div with overflow:visible, avatar fills container, frame absolutely centered at 1.4x, flair absolutely at bottom-right 0.4x"
  - "TDD React component tests: use // @vitest-environment jsdom annotation when render() needed in node-environment vitest project"
  - "Catalog fetch with None item prepended: [NONE_ITEM, ...(data.frames ?? [])] pattern"

requirements-completed: [PREM-03, PREM-04, PREM-05, WEB-03, WEB-04]

# Metrics
duration: 7min
completed: 2026-03-16
---

# Phase 30 Plan 04: Avatar Frame and Flair System Summary

**UserAvatar composite component (avatar + frame 1.4x centered + flair 0.4x bottom-right) landing in overlay, settings page AvatarCosmeticsCard with live preview, /admin/cosmetics catalog management page, and extension ChatContainer integration**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-03-15T23:58:51Z
- **Completed:** 2026-03-16T00:06:00Z
- **Tasks:** 2 (Task 3 is checkpoint:human-verify)
- **Files modified:** 8

## Accomplishments
- UserAvatar composite component with conditional frame (1.4x centered) and flair (0.4x bottom-right), tested with 10 vitest unit tests
- Overlay page replaces old Image/div avatar block with UserAvatar, wiring avatar_frame_url and avatar_flair_url from message.user
- Settings page AvatarCosmeticsCard with 4-col grids for frames and flairs, None first, premium locking with lock icon, live preview, and Save via PATCH /cosmetics
- /admin/cosmetics page with Frames/Flairs tabs, 64x64 thumbnail list, blur-preview add form, delete buttons
- Extension UserAvatar mirrors monorepo component (inline styles for non-Tailwind environment)
- Extension ChatContainer adds UserAvatar to message row before platform icon

## Task Commits

Each task was committed atomically:

1. **TDD RED: UserAvatar tests** - `f8d30a0` (test)
2. **Task 1: UserAvatar component + overlay + extension** - `918edfd` (monorepo), `6eec25b` (extension)
3. **Task 2: AvatarCosmeticsCard + admin cosmetics + AdminNav** - `4f5676f` (feat)

## Files Created/Modified
- `frontend/src/components/UserAvatar.tsx` - Composite avatar component with frame and flair stacking
- `frontend/src/components/__tests__/UserAvatar.test.tsx` - 10 vitest unit tests (jsdom environment)
- `frontend/src/app/admin/cosmetics/page.tsx` - Admin catalog management with Frames/Flairs tabs
- `frontend/src/app/overlay/[id]/page.tsx` - Replaces old avatar block with UserAvatar
- `frontend/src/app/settings/viewer/page.tsx` - Adds AvatarCosmeticsCard below ColorGradientCard
- `frontend/src/components/AdminNav.tsx` - Adds Cosmetics link after Viewers
- `all-chat-extension/src/ui/components/UserAvatar.tsx` - Extension copy with inline styles
- `all-chat-extension/src/ui/components/ChatContainer.tsx` - Adds UserAvatar to message row

## Decisions Made
- Extension UserAvatar uses inline styles rather than Tailwind className strings since the extension environment doesn't have Tailwind utility classes available in the same way.
- Added `// @vitest-environment jsdom` annotation to the test file rather than changing the global vitest config `environment: 'node'` — preserves fast node environment for other unit tests.
- Badge component in project has only `size` variant, not `variant` prop — removed `variant="default"` from admin page (Rule 1 auto-fix).
- Admin cosmetics page includes AdminNav header for consistent navigation.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Badge variant prop removed — component doesn't have it**
- **Found during:** Task 2 (admin cosmetics page)
- **Issue:** Used `<Badge variant="default">` but project Badge component only accepts `size` variant
- **Fix:** Removed `variant` prop from Badge element — defaults correctly without it
- **Files modified:** frontend/src/app/admin/cosmetics/page.tsx
- **Verification:** `tsc --noEmit` passes after removal
- **Committed in:** 4f5676f (Task 2 commit)

**2. [Rule 1 - Bug] Test file needed jsdom environment annotation**
- **Found during:** Task 1 TDD GREEN phase
- **Issue:** vitest.config.ts sets `environment: 'node'` for unit tests; `render()` requires DOM (`document is not defined`)
- **Fix:** Added `// @vitest-environment jsdom` at top of test file
- **Files modified:** frontend/src/components/__tests__/UserAvatar.test.tsx
- **Verification:** All 10 tests pass after annotation added
- **Committed in:** f8d30a0 and 918edfd (TDD RED + GREEN)

---

**Total deviations:** 2 auto-fixed (1 TypeScript type error, 1 test environment)
**Impact on plan:** Both fixes essential for correctness. No scope creep.

## Issues Encountered
- None beyond the auto-fixed deviations above.

## Self-Check

Files exist:
- [x] frontend/src/components/UserAvatar.tsx
- [x] frontend/src/app/admin/cosmetics/page.tsx
- [x] all-chat-extension/src/ui/components/UserAvatar.tsx

Commits exist:
- [x] f8d30a0 (TDD RED test)
- [x] 918edfd (Task 1 feat)
- [x] 4f5676f (Task 2 feat)

## Next Phase Readiness
- Task 3 (checkpoint:human-verify) is pending user visual verification
- All code is complete and TypeScript-clean; checkpoint is purely functional/visual QA
- After checkpoint approved: Phase 30 is fully complete end-to-end

---
*Phase: 30-avatar-frame-flair-system*
*Completed: 2026-03-16*
