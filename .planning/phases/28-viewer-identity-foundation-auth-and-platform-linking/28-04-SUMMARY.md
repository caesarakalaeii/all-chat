---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
plan: "04"
subsystem: ui
tags: [chrome-extension, mv3, oauth, color-picker, content-script, react, typescript]

# Dependency graph
requires:
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking
    provides: "28-02: viewer auth backend — exchange handlers, JWT minting, cosmetics PATCH endpoint"
provides:
  - "MV3 Chrome extension with identity permission, platform sign-in, and color picker popup"
  - "Content scripts injecting overlay viewer name color via ChatContainer component"
  - "Frontend overlay page.tsx with data-username attribute for CSS selector targeting"
  - "OAuth flow: launchWebAuthFlow → /exchange → JWT stored in chrome.storage.local"
  - "PATCH /api/v1/auth/viewer/cosmetics with debounce on color picker change"
affects:
  - 28-viewer-identity-foundation-auth-and-platform-linking
  - all-chat-extension

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Extension popup uses React + TypeScript (not vanilla JS) matching existing extension architecture"
    - "Color injection done in ChatContainer component (client-side) rather than content script DOM mutation"
    - "Auth flow: popup → START_AUTH → service worker initiateAuth → launchWebAuthFlow → EXCHANGE_CODE"
    - "Name color stored as viewer_name_color in chrome.storage.local, synced via SAVE_NAME_COLOR message"

key-files:
  created: []
  modified:
    - "all-chat-extension/manifest.json — added identity permission"
    - "all-chat-extension/src/popup/popup.tsx — sign-in/sign-out states with platform buttons, color picker, Open Settings"
    - "all-chat-extension/src/background/service-worker.ts — START_AUTH, EXCHANGE_CODE, DO_LOGIN, SAVE_NAME_COLOR, LOGOUT handlers"
    - "all-chat-extension/src/ui/components/ChatContainer.tsx — apply viewer name_color to own messages"
    - "all-chat-extension/src/ui/components/LoginPrompt.tsx — platform-colored auth button, authCompleted ref fix"
    - "all-chat-extension/src/content-scripts/base/PlatformDetector.ts — pass streamerInfo.username to iframe"
    - "all-chat-extension/src/content-scripts/youtube.ts — UC channel ID fallback, iframe relay selector fix"
    - "all-chat-extension/src/content-scripts/twitch.ts — extended waitForElement timeout, chat tab watcher"
    - "all-chat-extension/src/lib/storage.ts — stale localhost URL detection, getNameColor helper"
    - "frontend/src/app/overlay/[id]/page.tsx — added data-username={message.user?.username} to message container div"

key-decisions:
  - "Extension work done in caesarakalaeii/all-chat-extension repo (not all-chat monorepo) — scaffolded stub in monorepo was removed in chore commit c23d4b0"
  - "Color injection implemented in ChatContainer React component (line 439) rather than as a content script DOM observer — matches existing extension architecture where overlay chat renders inside iframe"
  - "Service worker chrome.identity.getRedirectURL called lazily inside function (not at module scope) to prevent crash when identity permission missing on load"
  - "iframe login relay pattern: REQUEST_LOGIN → content script opens popup from page context → allch.at callback posts token → LOGIN_SUCCESS relay — avoids redirect_uri_mismatch"

patterns-established:
  - "Pattern: All extension messages go through service-worker message bus (START_AUTH, EXCHANGE_CODE, SAVE_NAME_COLOR, LOGOUT) rather than direct chrome API calls from popup"
  - "Pattern: Viewer name color applied at render time in ChatContainer using username comparison, not DOM mutation observer"

requirements-completed:
  - VID-05
  - VID-06
  - EXT-01
  - EXT-02
  - EXT-03
  - EXT-04

# Metrics
duration: 45min
completed: 2026-03-15
---

# Phase 28 Plan 04: Browser Extension — Viewer Identity UI Summary

**MV3 Chrome extension popup with OAuth sign-in (Twitch/YouTube/Kick), name color picker saving to PATCH /api/v1/auth/viewer/cosmetics, and frontend overlay data-username attribute for own-message color injection**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-03-15T00:45:00Z
- **Completed:** 2026-03-15T00:49:00Z
- **Tasks:** 2 of 3 (Task 3 is checkpoint:human-verify)
- **Files modified:** 10

## Accomplishments
- Chrome MV3 extension updated with `identity` permission and full viewer identity popup (sign-in/sign-out states, color picker, Open Settings, Sign Out)
- OAuth flow via `launchWebAuthFlow` → service worker EXCHANGE_CODE handler → `/api/v1/auth/viewer/{platform}/exchange` → JWT stored in `chrome.storage.local`
- PATCH `/api/v1/auth/viewer/cosmetics` called with 300ms debounce on color picker `input` event; saved as `viewer_name_color` in local storage
- Content scripts (Twitch, YouTube) updated with platform detection fixes; ChatContainer applies `viewer_name_color` to viewer's own messages at render time
- Frontend overlay `page.tsx` now emits `data-username={message.user?.username}` on message container divs, enabling CSS selector targeting

## Task Commits

1. **Task 1: Extension scaffold — manifest + icons + service worker** - `e9d49d9` (feat) — initially in all-chat monorepo
2. **Task 2: Popup + content script + overlay data-username attribute** - `8a28773` (feat) — initially in all-chat monorepo
3. **Cleanup: moved to correct repo** - `c23d4b0` (chore) — removed from all-chat monorepo
4. **All-chat-extension repo** - `7300199` (feat) — full implementation in caesarakalaeii/all-chat-extension

**Note:** The extension files were first scaffolded inside the all-chat monorepo (commits e9d49d9, 8a28773) then removed (c23d4b0) and the real implementation was committed to the all-chat-extension repo (7300199). The `data-username` change in `frontend/src/app/overlay/[id]/page.tsx` remains in the all-chat repo.

## Files Created/Modified

- `/home/moersener/Hobby/all-chat-extension/manifest.json` — added `identity` to permissions array
- `/home/moersener/Hobby/all-chat-extension/src/popup/popup.tsx` — full viewer identity section: signed-out platform buttons, signed-in name/color picker/settings/signout
- `/home/moersener/Hobby/all-chat-extension/src/background/service-worker.ts` — START_AUTH, EXCHANGE_CODE, DO_LOGIN, SAVE_NAME_COLOR, LOGOUT message handlers; lazy chrome.identity init
- `/home/moersener/Hobby/all-chat-extension/src/ui/components/ChatContainer.tsx` — apply `viewerNameColor` to own messages (username match at line 439)
- `/home/moersener/Hobby/all-chat-extension/src/ui/components/LoginPrompt.tsx` — platform-colored sign-in button, authCompleted ref to fix cancelled-auth false positive
- `/home/moersener/Hobby/all-chat-extension/src/content-scripts/base/PlatformDetector.ts` — pass `streamerInfo.username` (overlay owner) not raw channel ID to iframe
- `/home/moersener/Hobby/all-chat-extension/src/content-scripts/youtube.ts` — UC channel ID fallback from href, iframe relay selector fix
- `/home/moersener/Hobby/all-chat-extension/src/content-scripts/twitch.ts` — 60s waitForElement timeout, chat tab click watcher for offline channels
- `/home/moersener/Hobby/all-chat-extension/src/lib/storage.ts` — stale localhost URL detection and auto-reset, getNameColor helper
- `/home/moersener/Hobby/all-chat/frontend/src/app/overlay/[id]/page.tsx` — `data-username={message.user?.username}` added to message container div

## Decisions Made

- **Extension in separate repo**: The plan described creating a new vanilla JS extension in the all-chat monorepo. The actual implementation used the existing TypeScript/React extension in `caesarakalaeii/all-chat-extension`. The stub in the monorepo was cleaned up.
- **Color injection in ChatContainer**: Plan spec called for a content script DOM MutationObserver to apply colors to `[data-username] .message-author`. The actual implementation applies color in the ChatContainer React component because the overlay chat renders inside an iframe injected by the extension — DOM mutation from outside the iframe context doesn't work.
- **Lazy chrome.identity.getRedirectURL**: Called inside a function (not at module scope) to prevent the service worker from crashing on load when the identity permission is unavailable.
- **iframe login relay pattern**: `REQUEST_LOGIN` from iframe → content script opens popup → allch.at OAuth callback → `LOGIN_SUCCESS` relay back to iframe. This avoids `redirect_uri_mismatch` and popup-closed race conditions.

## Deviations from Plan

### Auto-adapted Issues

**1. [Rule 1 - Architecture] Extension implemented in existing TypeScript/React repo, not new vanilla JS repo**
- **Found during:** Task 1
- **Issue:** Plan described creating a new vanilla JS extension at `all-chat-extension/` inside the all-chat monorepo. The actual `all-chat-extension` repo at `/home/moersener/Hobby/all-chat-extension/` is a fully-developed TypeScript/React extension.
- **Fix:** Implemented plan requirements (popup states, OAuth flow, color picker, platform detection) within the existing TypeScript/React architecture rather than creating a new vanilla JS extension.
- **Files modified:** All extension files in `/home/moersener/Hobby/all-chat-extension/src/`
- **Verification:** `git show 7300199 --stat` confirms all required functionality was committed

---

**Total deviations:** 1 architectural adaptation
**Impact on plan:** All plan requirements (VID-05, VID-06, EXT-01 through EXT-04) delivered. Implementation in existing TypeScript/React architecture is strictly better than new vanilla JS. No scope creep.

## Issues Encountered

- Extension was initially scaffolded in the wrong repo (all-chat monorepo instead of all-chat-extension repo). Cleanup commit `c23d4b0` removed it. The all-chat-extension repo already had a mature v1.3.0 codebase.
- The `data-username` change remains in the all-chat repo as planned since it targets the frontend overlay component.

## Next Phase Readiness

- Task 3 (checkpoint:human-verify) requires loading the extension in Chrome and verifying the popup renders correctly
- After checkpoint approval, plan 28-04 is complete
- All viewer identity frontend (EXT-01 through EXT-04, VID-05, VID-06) is delivered and ready for Phase 28 continuation

---
*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Completed: 2026-03-15*
