---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
verified: 2026-03-15T12:00:00Z
status: passed
score: 20/20 must-haves verified
re_verification:
  previous_status: gaps_found
  previous_score: 17/20
  gaps_closed:
    - "Content scripts write current_platform to chrome.storage.session on page load (twitch, youtube, kick)"
    - "Popup shows only the matching platform sign-in button when current_platform is set; shows all three when not on a supported platform"
    - "Color picker has a reset-to-default control (↺) that sends null to SAVE_NAME_COLOR and restores #ffffff"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Install extension, navigate to twitch.tv, open popup in signed-out state"
    expected: "Only 'Sign in with Twitch' button shown (context-aware filter active)"
    why_human: "Chrome extension OAuth flow requires a browser with extension loaded; cannot automate"
  - test: "Install extension, navigate to youtube.com/watch, open popup in signed-out state"
    expected: "Only 'Sign in with YouTube' button shown"
    why_human: "Same as above"
  - test: "Navigate to google.com (unsupported page), open popup"
    expected: "All three sign-in buttons shown (null-platform fallback)"
    why_human: "Same as above"
  - test: "Sign in, open popup, click ↺ reset button adjacent to color picker"
    expected: "Color resets to #ffffff, brief 'Saved' indicator appears, server receives PATCH cosmetics with {name_color: null}"
    why_human: "Runtime PATCH call and UI feedback require human verification"
  - test: "Navigate to /settings/viewer while a viewer_jwt_token is in localStorage"
    expected: "Page shows avatar, display name with platform badge, color picker with Save, Linked Platforms section"
    why_human: "JWT decode and three-state hydration require runtime verification"
---

# Phase 28: Viewer Identity Foundation Verification Report

**Phase Goal:** Establish viewer identity foundation — auth, platform account linking, and browser extension UI with color cosmetics
**Verified:** 2026-03-15T12:00:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plan 28-06)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | viewers, viewer_platform_identities, and viewer_cosmetics tables exist in the database | VERIFIED | `migrations/035_viewer_identity.sql` contains all three CREATE TABLE statements |
| 2 | viewer_sessions has a viewer_id FK column pointing to viewers.id | VERIFIED | `migrations/035_viewer_identity.sql`: ALTER TABLE with REFERENCES viewers(id) ON DELETE SET NULL |
| 3 | ViewerClaims JWT struct carries viewer_id, display_name, and avatar_url fields | VERIFIED | `shared/auth/jwt.go` lines 48-57: all three fields with correct json tags |
| 4 | ViewerIdentityRepository can look up or create a viewer by (platform, platform_user_id) | VERIFIED | `services/auth-service/repository/viewer_identity_repository.go`: full 4-step transactional GetOrCreateViewerByPlatform |
| 5 | POST /viewer/{platform}/exchange accepts {code, state} and returns {token, viewer_info} JSON | VERIFIED | `services/auth-service/handlers/viewer_exchange.go`: HandleTwitchExchange, HandleYouTubeExchange, HandleKickExchange; routes at auth-service main.go lines 260/263/266; API gateway lines 357/360/363 |
| 6 | The JWT token returned by exchange contains viewer_id | VERIFIED | `services/auth-service/handlers/viewer_auth.go` line 335: generateViewerJWT(session, viewerID uuid.UUID) populates ViewerID at line 350 |
| 7 | PATCH /viewer/cosmetics saves name_color to viewer_cosmetics and invalidates Redis cache | VERIFIED | `services/auth-service/handlers/viewer_cosmetics.go`: UpsertViewerCosmetics + Redis key delete; wired at main.go line 295 and API gateway line 394 |
| 8 | Sign-in creates or reuses viewer_id via GetOrCreateViewerByPlatform | VERIFIED | All three callback handlers call identityRepo.GetOrCreateViewerByPlatform before generating JWT |
| 9 | ViewerBadgeEnricher resolves platform user_id to viewer_id via Redis cache, falls back to DB on miss | VERIFIED | `services/message-processor/enricher/viewer_badge_enricher.go`: full Redis to null-sentinel to DB-JOIN logic; 7 tests pass |
| 10 | When viewer has name_color set, msg.User.Color is overridden | VERIFIED | `viewer_badge_enricher.go` line 129: `msg.User.Color = *nameColor` on DB hit |
| 11 | Null sentinel prevents thundering herd; cache TTL is 5 minutes | VERIFIED | `viewer_badge_enricher.go` line 110: caches "null"; ViewerIdentityCacheTTL = 5 * time.Minute |
| 12 | ViewerBadgeEnricher is wired into both CHAT and EVENT paths in message-processor | VERIFIED | `services/message-processor/cmd/main.go` lines 402 and 479: Enrich called in both paths |
| 13 | Extension manifest is MV3 with chrome.identity permission | VERIFIED | `all-chat-extension/manifest.json`: manifest_version=3, "identity" in permissions array |
| 14 | Content script detects platform page and writes platform string to chrome.storage.session | VERIFIED (gap closed) | twitch.ts line 161, youtube.ts line 242, kick.ts line 188: each calls `chrome.storage.session.set({ current_platform: '...' })` with correct string inside initialize() after extensionEnabled guard |
| 15 | Popup shows signed-out state with context-aware platform sign-in buttons | VERIFIED (gap closed) | popup.tsx line 18: currentPlatform state; line 28-29: chrome.storage.session.get in useEffect; lines 198-200: .filter((p) => currentPlatform === null || currentPlatform === p) before .map() |
| 16 | Popup shows signed-in state with display name, color picker, Open Settings, Sign Out | VERIFIED | popup.tsx lines 163-190: display_name, color input, reset button, Settings button, Sign out button |
| 17 | Clicking a sign-in button launches launchWebAuthFlow and calls POST /exchange | VERIFIED | popup.tsx line 96: chrome.identity.launchWebAuthFlow; line 107: EXCHANGE_CODE message to service-worker which fetches POST /api/v1/auth/viewer/${platform}/exchange |
| 18 | Color picker change triggers PATCH /viewer/cosmetics with debounce (EXT-02) | VERIFIED | popup.tsx handleColorChange: 300ms setTimeout at line 61; service-worker SAVE_NAME_COLOR calls PATCH cosmetics |
| 19 | Color picker has reset-to-default option (EXT-01) | VERIFIED (gap closed) | popup.tsx lines 73-84: handleColorReset sends { type: 'SAVE_NAME_COLOR', color: null }; lines 177-183: ↺ button adjacent to color input with onClick={handleColorReset} |
| 20 | /settings/viewer route exists with viewer info display and Linked Platforms section | VERIFIED | `frontend/src/app/settings/viewer/page.tsx`: 'use client', JWT decode, viewer info, color picker + PATCH, Linked Platforms section; data-username on overlay at page.tsx line 599 |

**Score: 20/20 truths verified**

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `migrations/035_viewer_identity.sql` | viewers, viewer_platform_identities, viewer_cosmetics DDL + viewer_sessions alter | VERIFIED | All four DDL statements present |
| `shared/auth/jwt.go` | ViewerClaims with ViewerID, DisplayName, AvatarURL | VERIFIED | All three fields present |
| `services/auth-service/repository/viewer_identity_repository.go` | GetOrCreateViewerByPlatform, GetViewerCosmetics, UpsertViewerCosmetics | VERIFIED | All three methods implemented |
| `services/auth-service/handlers/viewer_exchange.go` | HandleTwitchExchange, HandleYouTubeExchange, HandleKickExchange | VERIFIED | All three POST handlers present |
| `services/auth-service/handlers/viewer_cosmetics.go` | HandlePatchCosmetics with hex validation + Redis invalidation | VERIFIED | Validates `^#[0-9a-fA-F]{6}$`, calls UpsertViewerCosmetics, invalidates cache key |
| `services/auth-service/handlers/viewer_cosmetics_test.go` | Tests: valid color, null, invalid hex, unauthorized, missing viewer_id | VERIFIED | 5 test functions pass |
| `services/auth-service/cmd/main.go` | ViewerIdentityRepository wired; exchange + cosmetics routes registered | VERIFIED | Lines 190/195/198 instantiate repos; lines 260/263/266/295 register routes |
| `services/api-gateway/cmd/main.go` | PATCH /auth/viewer/cosmetics + POST exchange routes proxied | VERIFIED | Lines 357/360/363/394 |
| `services/message-processor/enricher/viewer_badge_enricher.go` | ViewerBadgeEnricher with Enrich method | VERIFIED | Full implementation with viewerDB interface + pgxPoolAdapter |
| `services/message-processor/enricher/viewer_badge_enricher_test.go` | 7 tests covering all cache/DB scenarios | VERIFIED | 7 test functions pass |
| `services/message-processor/cmd/main.go` | viewerBadgeEnricher in enricher chain | VERIFIED | Lines 169/402/479 |
| `all-chat-extension/manifest.json` | MV3, chrome.identity permission | VERIFIED | manifest_version=3, "identity" in permissions |
| `all-chat-extension/src/content-scripts/twitch.ts` | chrome.storage.session.set({ current_platform: 'twitch' }) in initialize() | VERIFIED | Line 161: session write after extensionEnabled guard, before globalDetector.init() |
| `all-chat-extension/src/content-scripts/youtube.ts` | chrome.storage.session.set({ current_platform: 'youtube' }) in initialize() | VERIFIED | Line 242: session write after globalDetector assigned, before waitForElement |
| `all-chat-extension/src/content-scripts/kick.ts` | chrome.storage.session.set({ current_platform: 'kick' }) in initialize() | VERIFIED | Line 188: session write after KickDetector assigned, before globalDetector.init() |
| `all-chat-extension/src/popup/popup.tsx` | currentPlatform state, session.get in useEffect, filtered buttons, handleColorReset, ↺ button | VERIFIED | Line 18: state; lines 28-29: session.get; line 199: filter; lines 73-84: handleColorReset; lines 177-183: ↺ button |
| `all-chat-extension/src/background/service-worker.ts` | START_AUTH, EXCHANGE_CODE, SAVE_NAME_COLOR, LOGOUT handlers | VERIFIED | Lines 88/93/104 message handlers; SAVE_NAME_COLOR handles null (no changes needed in plan 06) |
| `all-chat-extension/src/ui/components/ChatContainer.tsx` | Applies viewer_name_color to own messages | VERIFIED | Lines 85/439-441: viewerNameColor state applied when username matches |
| `frontend/src/app/overlay/[id]/page.tsx` | data-username attribute on message container div | VERIFIED | Line 599: `data-username={message.user?.username}` |
| `frontend/src/app/settings/viewer/page.tsx` | /settings/viewer route stub | VERIFIED | File exists with 'use client', JWT decode, viewer info, Linked Platforms, PATCH cosmetics |
| `services/auth-service/repository/viewer_identity_repository_test.go` | Integration test scaffolds | VERIFIED | File exists with //go:build integration guard |
| `services/auth-service/handlers/viewer_exchange_test.go` | Tests for exchange handlers | VERIFIED | 3 test functions pass |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `shared/auth/jwt.go ViewerClaims` | `viewer_auth.go generateViewerJWT` | ViewerID populated from identity repo result | VERIFIED | generateViewerJWT(session, viewerID uuid.UUID) at line 335; ViewerID assigned at line 350 |
| `migrations/035_viewer_identity.sql` | `viewer_identity_repository.go` | viewer_platform_identities table query | VERIFIED | Repository queries viewer_platform_identities in GetOrCreateViewerByPlatform |
| `HandleTwitchExchange` | `ViewerIdentityRepository.GetOrCreateViewerByPlatform` | Called after ExchangeCode succeeds | VERIFIED | viewer_auth.go line 194 |
| `HandlePatchCosmetics` | `ViewerIdentityRepository.UpsertViewerCosmetics` | Writes name_color then deletes Redis cache | VERIFIED | viewer_cosmetics.go line 117: UpsertViewerCosmetics; line 55: cache key deleted |
| `generateViewerJWT` | `ViewerClaims.ViewerID` | viewerID.String() assigned | VERIFIED | viewer_auth.go lines 339-350: uuid.Nil check + String() assignment |
| `ViewerBadgeEnricher.Enrich` | Redis `viewer:identity:{platform}:{user_id}` | redis.Get then redis.Set | VERIFIED | viewer_badge_enricher.go line 82: redis.Get; lines 110/124: redis.Set with TTL |
| `ViewerBadgeEnricher.Enrich` | `viewer_platform_identities JOIN viewer_cosmetics` DB query | db.QueryRow on cache miss | VERIFIED | viewer_badge_enricher.go lines 100-105: LEFT JOIN query on cache miss |
| `message-processor cmd/main.go enricher chain` | `viewerBadgeEnricher.Enrich` | Called after cheermoteEnricher in both paths | VERIFIED | Lines 402 (CHAT) and 479 (EVENT) |
| `popup.tsx signInWithPlatform` | `POST /api/v1/auth/viewer/{platform}/exchange` | fetch after launchWebAuthFlow | VERIFIED | popup.tsx line 96 to service-worker EXCHANGE_CODE handler |
| `popup.tsx color input event` | `PATCH /api/v1/auth/viewer/cosmetics` | debounced via SAVE_NAME_COLOR message | VERIFIED | popup.tsx line 63 to service-worker line 478 |
| `popup.tsx handleColorReset` | `service-worker SAVE_NAME_COLOR` | sendMessage({ type: 'SAVE_NAME_COLOR', color: null }) | VERIFIED | popup.tsx line 77: exact call confirmed; service-worker already handles null |
| `twitch.ts initialize()` | `chrome.storage.session` | chrome.storage.session.set({ current_platform: 'twitch' }) | VERIFIED | twitch.ts line 161: call confirmed in correct position inside initialize() |
| `popup.tsx useEffect` | `chrome.storage.session` | chrome.storage.session.get(['current_platform']) | VERIFIED | popup.tsx lines 28-29: get call confirmed; result stored in currentPlatform state |
| `content scripts + ChatContainer.tsx` | overlay message container `[data-username]` | ChatContainer React component applies color when username matches | VERIFIED (adapted) | Color injection in ChatContainer.tsx lines 439-441; data-username at overlay page.tsx line 599 |

---

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|---------------|-------------|--------|----------|
| VID-03 | 28-01, 28-02, 28-03, 28-05 | Viewer color preference persists server-side, survives extension reinstall | SATISFIED | viewer_cosmetics table + UpsertViewerCosmetics + PATCH endpoint + ViewerBadgeEnricher runtime injection + settings page |
| VID-04 | 28-01, 28-02 | Viewer can link platform identities to All-Chat account | SATISFIED | viewer_platform_identities table + GetOrCreateViewerByPlatform on every sign-in |
| VID-05 | 28-01, 28-02, 28-04, 28-06 | Viewer can authenticate from browser extension popup | SATISFIED | popup.tsx handleSignIn to START_AUTH to launchWebAuthFlow to EXCHANGE_CODE to JWT stored |
| VID-06 | 28-01, 28-02, 28-04, 28-05, 28-06 | Extension popup shows auth status and signed-in display name/avatar | SATISFIED | popup.tsx signed-in state shows display_name at line 164; settings/viewer page shows viewer info |
| EXT-01 | 28-04, 28-06 | Extension popup shows inline name color picker with reset-to-default option | SATISFIED | Color picker input type="color" present + ↺ button (handleColorReset) present; reset sends { color: null } to PATCH cosmetics |
| EXT-02 | 28-04 | Color change saves immediately to server and local storage | SATISFIED | 300ms debounce triggers SAVE_NAME_COLOR to PATCH cosmetics; localStorage updated optimistically |
| EXT-03 | 28-04, 28-05 | "Open Settings" button navigates to /settings/viewer | SATISFIED | popup.tsx lines 101-103: openSettings() to chrome.tabs.create; page exists at frontend/src/app/settings/viewer/page.tsx |
| EXT-04 | 28-04, 28-06 | Content scripts apply viewer name_color to own messages in overlay | SATISFIED | ChatContainer.tsx lines 439-441: viewerNameColor applied when username === viewerInfo.username; data-username on overlay container at page.tsx line 599 |

All 8 requirement IDs (VID-03, VID-04, VID-05, VID-06, EXT-01, EXT-02, EXT-03, EXT-04) are SATISFIED.

---

### Anti-Patterns Found

None. All previously-flagged anti-patterns (missing context-aware filtering and missing reset button) were resolved by plan 28-06. TypeScript build exits 0 with no errors or warnings (`webpack 5 compiled successfully`).

---

### Human Verification Required

The six manual checks listed in plan 28-06 Task 3 were approved by human as part of the gap-closure checkpoint. The items below are retained for completeness and end-to-end session confirmation.

#### 1. Context-Aware Twitch Sign-In Button

**Test:** Install built extension in Chrome. Navigate to twitch.tv (any channel). Open the extension popup in signed-out state.
**Expected:** Only "Sign in with Twitch" is shown — YouTube and Kick buttons are absent.
**Why human:** Chrome extension cross-context session storage and OAuth flow require a real browser with extension loaded.

#### 2. Context-Aware YouTube Sign-In Button

**Test:** Navigate to youtube.com/watch (any video/stream). Open popup.
**Expected:** Only "Sign in with YouTube" is shown.
**Why human:** Same as above.

#### 3. Unsupported-Page Fallback

**Test:** Navigate to google.com. Open popup.
**Expected:** All three sign-in buttons appear (currentPlatform is null, no filter applied).
**Why human:** Same as above.

#### 4. Color Picker Reset Button

**Test:** Sign in with any platform. Open popup. Verify the ↺ symbol is visible adjacent to the color swatch. Click it.
**Expected:** Color resets to white (#ffffff), a brief "Saved" indicator appears, server receives PATCH /api/v1/auth/viewer/cosmetics with body `{name_color: null}`.
**Why human:** Runtime PATCH call and visual feedback require a browser session.

#### 5. Settings Page Viewer Info Rendering

**Test:** Navigate to /settings/viewer while `viewer_jwt_token` is present in localStorage.
**Expected:** Page shows viewer avatar, display name with platform badge, name color picker with Save button, and Linked Platforms section with one "Connected" platform and two "Connect" states.
**Why human:** JWT decode and three-state hydration require runtime verification.

---

### Re-Verification Summary

All three gaps from the initial verification (2026-03-15T10:00:00Z) have been closed by plan 28-06. No regressions were introduced.

**Gap 1 closed — content scripts now write current_platform to chrome.storage.session:**
twitch.ts (line 161), youtube.ts (line 242), and kick.ts (line 188) each call `chrome.storage.session.set({ current_platform: '...' })` fire-and-forget inside `initialize()` after the extensionEnabled guard and before `globalDetector.init()`. The base `PlatformDetector.ts` is correctly untouched (zero matches confirmed).

**Gap 2 closed — popup shows context-aware sign-in buttons:**
popup.tsx holds `currentPlatform` state (line 18), reads `chrome.storage.session.get(['current_platform'])` in its load useEffect (lines 28-29), and applies `.filter((p) => currentPlatform === null || currentPlatform === p)` before the platform button `.map()` (lines 198-200). null means no platform detected — all three buttons shown as the safe fallback.

**Gap 3 closed — color picker has reset-to-default button (EXT-01 now fully satisfied):**
`handleColorReset` (lines 73-84) sends `{ type: 'SAVE_NAME_COLOR', color: null }` to the service-worker which already handled null. The ↺ button (&#x21BA;) is adjacent to the `<input type="color">` at lines 177-183. No service-worker changes were needed.

No regressions detected: existing popup wiring (launchWebAuthFlow, EXCHANGE_CODE, SAVE_NAME_COLOR for color changes, LOGOUT) all remain intact. TypeScript build passes cleanly.

The backend foundation (DB schema, JWT claims, repository, exchange endpoints, cosmetics PATCH, ViewerBadgeEnricher) was fully verified in the initial pass and is untouched by plan 28-06.

The phase goal — establish viewer identity foundation with auth, platform account linking, and browser extension UI with color cosmetics — is fully achieved.

---

_Verified: 2026-03-15T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Re-verification: Yes — gaps closed by plan 28-06_
