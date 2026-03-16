---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
plan: 06
subsystem: ui
tags: [chrome-extension, typescript, chrome-storage, popup, content-scripts]

# Dependency graph
requires:
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking
    provides: Extension popup with sign-in buttons and color picker (plan 04)
provides:
  - current_platform written to chrome.storage.session by all three content scripts on page load
  - Popup reads current_platform and filters sign-in buttons to match active platform
  - Color picker reset-to-default button (↺) sending SAVE_NAME_COLOR with color:null
affects:
  - Future extension plans requiring platform detection in popup context

# Tech tracking
tech-stack:
  added: []
  patterns:
    - chrome.storage.session for cross-context platform detection (content script → popup)
    - Fire-and-forget session writes with .catch() warning pattern
    - Derived UI list from session state (filter array before .map in JSX)

key-files:
  created: []
  modified:
    - all-chat-extension/src/content-scripts/twitch.ts
    - all-chat-extension/src/content-scripts/youtube.ts
    - all-chat-extension/src/content-scripts/kick.ts
    - all-chat-extension/src/popup/popup.tsx

key-decisions:
  - "Session write in content scripts fires even when streamer not configured — signals platform presence, not UI injection"
  - "currentPlatform filter: null shows all three buttons (fallback), non-null shows only matching platform button"
  - "handleColorReset sends null to existing SAVE_NAME_COLOR handler — no service-worker changes needed"

patterns-established:
  - "Platform detection pattern: content script writes platform string to chrome.storage.session; popup reads on open"
  - "Null-inclusive filter pattern: null means no filter (show all), value means show only matching"

requirements-completed: [EXT-01, EXT-02, EXT-03, EXT-04, VID-05, VID-06]

# Metrics
duration: ~30min
completed: 2026-03-15
---

# Phase 28 Plan 06: Extension UI Gap Closure Summary

**Content scripts now write current_platform to chrome.storage.session so the popup shows only the matching sign-in button and a color reset (↺) button clears viewer_name_color**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-03-15
- **Completed:** 2026-03-15
- **Tasks:** 3 (2 auto + 1 human-verify)
- **Files modified:** 4

## Accomplishments
- twitch.ts, youtube.ts, and kick.ts each write `current_platform` to `chrome.storage.session` on page load using fire-and-forget `.catch()` pattern
- popup.tsx reads `current_platform` in its useEffect and filters the sign-in button list — only the matching platform button shown on supported platform pages, all three shown elsewhere
- Color picker gained a ↺ reset button that sets nameColor to `#ffffff` and sends `{ type: 'SAVE_NAME_COLOR', color: null }` to the existing service-worker handler
- All six manual verification checks passed: build clean, Twitch/YouTube context filtering working, fallback showing all three buttons, reset button visible and functional

## Task Commits

Each task was committed atomically:

1. **Task 1: Write current_platform to chrome.storage.session in all three content scripts** - `1f55286` (feat)
2. **Task 2: Add currentPlatform state and context-aware sign-in buttons to popup.tsx** - `95c016d` (feat)
3. **Task 3: Human verification checkpoint** - APPROVED (no commit — verification only)

## Files Created/Modified
- `all-chat-extension/src/content-scripts/twitch.ts` - Added `chrome.storage.session.set({ current_platform: 'twitch' })` in initialize()
- `all-chat-extension/src/content-scripts/youtube.ts` - Added `chrome.storage.session.set({ current_platform: 'youtube' })` in initialize()
- `all-chat-extension/src/content-scripts/kick.ts` - Added `chrome.storage.session.set({ current_platform: 'kick' })` in initialize()
- `all-chat-extension/src/popup/popup.tsx` - Added currentPlatform state, session.get in useEffect, filtered platform button list, handleColorReset function, ↺ reset button in JSX

## Decisions Made
- Session write fires regardless of whether streamer is configured — it signals "user is on this platform page" not "All-Chat UI was injected". This matches verification truths #14 and #15 from 28-VERIFICATION.md.
- The null sentinel for currentPlatform doubles as "not yet loaded" and "not on a supported platform" — both cases correctly show all three buttons as the safe fallback.
- No service-worker changes: `saveNameColor(color: string | null)` already handled null by removing from localStorage and sending `{ name_color: null }` to PATCH cosmetics.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All six gap-closure verification items from 28-VERIFICATION.md are now satisfied for the extension UI layer
- EXT-01 reset-to-default fully implemented; EXT-02/03/04 context-aware popup behavior complete
- Extension ready for end-to-end testing with live platform pages

---
*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Completed: 2026-03-15*
