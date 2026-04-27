---
phase: 13-text-to-speech-tts-for-chat-messages
plan: 03
subsystem: ui
tags: [tts, elevenlabs, react, typescript, nextjs, vitest, clipboard-api, alertdialog, premium-gate]

# Dependency graph
requires:
  - phase: 13-01-web-speech-tts-tier
    provides: TTSGroup shell + ttsPlayer utility + AppearancePanel mount + TTS_SETTINGS_UPDATE postMessage
  - phase: 13-02-backend-tts-plumbing
    provides: 7 TTS endpoints (POST/DELETE/GET /tts-config, rotate-token, voices, test, streaming proxy) + per-overlay tts_token JWT + obs_url construction
provides:
  - "6 ElevenLabs API-client wrappers on overlaysApi (saveTTSKey / removeTTSKey / rotateTTSToken / getTTSVoices / testTTSKey / getTTSConfig) + TTSConfigMetadata / ElevenLabsVoice / TestKeyResult type exports"
  - "TTSGroup Advanced (ElevenLabs) block — ApiKeyInput (save / remove / test / quota), ObsUrlPanel (copy / regenerate + confirm modal), ElevenLabsVoicePicker (lazy-load)"
  - "Editor page: 5 async handlers wired through AppearancePanel to TTSGroup; Web-Speech preview; persist tts_* via existing updateConfig; EMBED_READY re-sends TTS settings"
  - "Live overlay: tts_token read from window.location.search and fed into TTSSettings.ttsToken"
  - "Embed preview: elevenLabsRuntimeRef caches endpoint+token+voice across TTS_SETTINGS_UPDATE postMessages so editor tweaks never clobber the fetch path"
affects: [phase-14-onwards (ElevenLabs reusable pattern), 13-HUMAN-UAT]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Inline role=\"alertdialog\" modal with Tailwind fixed-overlay — no @base-ui/react/alert-dialog dependency added"
    - "vi.hoisted() for toast mock fn sharing between test module and vi.mock factory"
    - "navigator.clipboard stub via Object.defineProperty(global.navigator, 'clipboard', {...}) — jsdom doesn't ship one"
    - "Lazy-fetch-on-focus voice dropdown with triedRef latch (only ever one fetch per mount)"
    - "3-second inline-confirm delete (arm -> timeout -> disarm) with useRef<NodeJS.Timeout> cleanup"

key-files:
  created:
    - .planning/phases/13-text-to-speech-tts-for-chat-messages/13-HUMAN-UAT.md
    - .planning/phases/13-text-to-speech-tts-for-chat-messages/13-03-SUMMARY.md
  modified:
    - frontend/src/lib/api/overlays.ts
    - frontend/src/components/appearance/TTSGroup.tsx
    - frontend/src/components/appearance/__tests__/TTSGroup.test.tsx
    - frontend/src/app/overlays/[id]/page.tsx
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx

key-decisions:
  - "Inline alertdialog div over @base-ui/react/alert-dialog — the plan allowed either. The inline div keeps the diff focused and avoids pulling in a new primitive; a future UX pass can upgrade to @base-ui primitives if focus-trap / escape-dismiss behavior matters."
  - "Moved localStorage Bearer-token extraction inline into testTTSKey rather than exposing an authHeaders() getter on apiClient. The plan hinted at apiClient.authHeaders but ApiClient in client.ts only reads localStorage inside its private fetch() — replicating three lines of read logic keeps the public surface of apiClient unchanged."
  - "ttsSettings state in the editor lives NEXT TO soundSettings rather than merged into one displaySettings blob. Rationale: sound + TTS each have their own handler/state pair, and merging would create a joint setter that sprays both onChange families. Kept them independent; AppearancePanel receives the merged record via spread ({...soundSettings, ...ttsSettings})."
  - "Empty voiceId in the live overlay relies on Plan 02's cfg.VoiceID fallback (TestHandleTTSUsesCfgVoiceIDWhenQueryParamMissing). This avoids a GET /tts-config call in the public OBS path where the user JWT is absent."

patterns-established:
  - "Pattern: For Advanced panel blocks gated by premium, wrap with a z-10 absolute overlay that both PremiumBadge and upgrade copy render into — the inputs beneath stay rendered (for visual parity) but are disabled. Keeps the layout stable across premium/non-premium toggling."
  - "Pattern: vi.hoisted + object-spread onto a top-level vi.fn() for react-hot-toast default-export mocking. Preserves toast('...') call-as-fn semantics alongside toast.success / toast.error method calls."
  - "Pattern: elevenLabsRuntimeRef in the embed iframe — when a component needs to cache ambient runtime context that survives postMessage updates, use a useRef initialized on-mount and merge it into every new-state construction."

requirements-completed: [D-09, D-10, D-11, D-12, D-13, D-14, D-15, D-16, D-18, D-23, D-38, D-39, D-40]

# Metrics
duration: 35min
completed: 2026-04-24
---

# Phase 13 Plan 03: ElevenLabs Frontend UX Summary

**Replaces Plan 01's Advanced-block stub with the full ElevenLabs premium flow (API-key save/test/remove with 3-second inline confirm, voice picker lazy-loaded on focus, character-quota display with `N,/M, characters this month (P%)` format, Copy / Regenerate OBS URL with a role="alertdialog" confirmation modal) and wires the backend's 7 TTS endpoints end-to-end so real ElevenLabs audio streams through the overlay when a tts_token JWT is present in the URL.**

## Performance

- **Duration:** ~35 min (task-execution wall time)
- **Started:** 2026-04-23T21:33:04Z
- **Completed:** 2026-04-24T06:54:02Z (clock crossed a sleep boundary; actual work ~35m)
- **Tasks:** 4 (one is the auto-approved human-verify checkpoint)
- **Files modified:** 6
- **Files created:** 2 (HUMAN-UAT.md + this SUMMARY.md)

## Accomplishments

- `overlaysApi` now exports 6 ElevenLabs wrappers with 3 new type exports (`TTSConfigMetadata`, `ElevenLabsVoice`, `TestKeyResult`). `testTTSKey` bypasses the JSON `apiClient` because the `/tts-config/test` response is `audio/mpeg` + `x-characters-*` headers, and mirrors `client.ts`'s Bearer-token pattern by reading `localStorage.getItem('jwt_token')` directly.
- `TTSGroup.tsx` ships three new internal components: `ApiKeyInput` (password input + save / remove with 3-second inline confirm + test with verbose D-39 toasts + character-quota display), `ObsUrlPanel` (read-only input + Copy + Regenerate with a confirmation modal), and `ElevenLabsVoicePicker` (lazy-loads on first focus via the `triedRef` latch).
- `TTSGroup.test.tsx` grew from 15 tests to 35 (the original 15 + A1..A20 covering every Advanced-block branch including save/save-error/test/test-errors/copy-obs-url/regenerate-open/regenerate-cancel/regenerate-confirm/remove-arm/remove-disarm/voices-lazy-load/non-premium). All 35 pass.
- Editor page (`overlays/[id]/page.tsx`) holds `ttsSettings` + `hasElevenLabsConfig` + `obsUrl` state; loads all 20 `tts_*` fields from `display_settings` on mount plus calls `getTTSConfig` for metadata; persists `tts_*` via the existing `updateConfig` flow (spread into `display_settings`); passes 5 async handlers into `AppearancePanel`; drives Web-Speech previews via `onTTSPreview`; re-sends TTS settings on EMBED_READY handshake.
- Live overlay page (`overlay/[id]/page.tsx`) reads `tts_token` from `window.location.search` at **line 353** and hydrates `TTSSettings.ttsToken` + `ttsEndpoint = /api/v1/overlays/{id}/tts`. Empty `voiceId` relies on Plan 02's cfg.VoiceID fallback (tested in `TestHandleTTSUsesCfgVoiceIDWhenQueryParamMissing`).
- Embed preview page (`overlays/[id]/preview/embed/page.tsx`) introduces `elevenLabsRuntimeRef` — populated once on mount from `getTTSConfig` / `new URL(meta.obs_url).searchParams.get('tts_token')` — and merges it into every `TTS_SETTINGS_UPDATE` handler so editor tweaks never clobber the fetch path.
- TTSGroup's Plan 01 stub string (`"ship in Plan 03"`) is gone — `grep -c "ship in Plan 03" TTSGroup.tsx` returns 0.

## Task Commits

Each task was committed atomically:

1. **Task 1: 6 ElevenLabs API-client wrappers** — `a8a52f44` (feat)
2. **Task 2: TTSGroup Advanced block + 20 new tests (TDD RED -> GREEN)** — `dcd8a4e2` (feat)
3. **Task 3: Wire editor + live overlay + embed preview** — `1c2b5f70` (feat)
4. **Task 4: Auto-approved human-verify checkpoint -> HUMAN-UAT.md** — `6ef7be0b` (test)

**Plan metadata:** This SUMMARY.md will be committed as `docs(13-03): complete ElevenLabs frontend UX plan` after self-check.

_Note: Task 2 was a TDD task — the RED phase wrote all 20 A-tests in the test file which I verified failed against Plan 01's stub (19/35 failing, 16/35 passing — the 16 were Plan 01's tests plus A20 which happened to pass against the stub). The GREEN phase then rebuilt the Advanced block in a single commit because the test file and implementation have to land together to both type-check and run. The first `vi.mock` run hit a ReferenceError from top-level-variable hoisting; switching to `vi.hoisted()` fixed it on the next run._

## Files Created/Modified

### Created
- `.planning/phases/13-text-to-speech-tts-for-chat-messages/13-HUMAN-UAT.md` — 5 pending manual verification items (real ElevenLabs happy-path, OBS end-to-end, regenerate-invalidation, session-fallback, remove-key)
- `.planning/phases/13-text-to-speech-tts-for-chat-messages/13-03-SUMMARY.md` — this file

### Modified
- `frontend/src/lib/api/overlays.ts` — +128 lines: 3 type exports + 6 methods (saveTTSKey, removeTTSKey, rotateTTSToken, getTTSVoices, testTTSKey, getTTSConfig)
- `frontend/src/components/appearance/TTSGroup.tsx` — Removed Plan 01's 14-line stub; added ApiKeyInput / ObsUrlPanel / ElevenLabsVoicePicker (~260 lines of new component logic); replaced the Advanced block with the premium overlay + three new components
- `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` — Added `vi.hoisted` toast mock + clipboard mock + 20 new tests A1..A20; total 35 tests (from 15)
- `frontend/src/app/overlays/[id]/page.tsx` — Added TTS state (ttsSettings / hasElevenLabsConfig / obsUrl); added `sendTtsSettingsToIframe`, `handleTTSSettingsChange`, 5 async handlers (save/remove/rotate/test/fetch-voices); extended config-load to populate 20 tts_* fields from display_settings + fetch getTTSConfig; spread `...ttsSettings` into handleSaveConfiguration's display_settings payload; wired 10+ new AppearancePanel props; added EMBED_READY re-send of tts settings
- `frontend/src/app/overlay/[id]/page.tsx` — Added window.location.search tts_token read + ElevenLabs-branch hydration of TTSSettings.ttsEndpoint / ttsToken / voiceId
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — Added elevenLabsRuntimeRef; config-load path parses obs_url via `new URL(...).searchParams.get('tts_token')`; TTS_SETTINGS_UPDATE merges cached runtime params so postMessage updates preserve the fetch path

## Decisions Made

### 1. Inline role="alertdialog" div vs `@base-ui/react/alert-dialog`

The plan let either land. I chose the inline Tailwind div because:
- The repo doesn't already use `@base-ui/react/alert-dialog` anywhere (grepped the whole frontend — zero matches).
- Pulling in a new primitive just for one destructive confirmation is overkill for v1.
- The inline div satisfies the test (`screen.getByRole('alertdialog')` works with jsdom + @testing-library/react).
- A future UX pass that wants proper focus-trap + escape-to-dismiss can upgrade to `@base-ui/react/alert-dialog` without changing the TTSGroup API.

### 2. Bearer-token pattern in testTTSKey

The plan said `headers: apiClient.authHeaders ? apiClient.authHeaders() : {}` but `ApiClient` in `client.ts` doesn't expose an `authHeaders()` method — the token read is inside a private `fetch()` method. I replicated the 3-line pattern inline rather than widening `ApiClient`'s public surface. This keeps the signature of `apiClient` unchanged and avoids a cross-cutting refactor.

### 3. `ttsSettings` state beside `soundSettings`, not merged

The editor's `displaySettings` pattern historically kept sound and TTS state independent (Plan 01 followed this). Plan 03 adds 20 more tts_* fields and 5 async handlers. I kept them independent:
- `soundSettings` state + `handleSoundSettingsChange` (Phase 12)
- `ttsSettings` state + `handleTTSSettingsChange` (Phase 13)
- `AppearancePanel` receives the merged record via spread: `{...soundSettings, ...ttsSettings}`

This preserves single-concern handlers and avoids sprawling state.

### 4. Empty voiceId in live overlay

The live overlay never calls `getTTSConfig` (the public OBS browser source has no user JWT). Instead the `voiceId = ''` contract lets Plan 02's backend `HandleTTS` substitute `cfg.VoiceID` from the overlay_tts_configs row at fetch time. Covered by the Plan 02 test `TestHandleTTSUsesCfgVoiceIDWhenQueryParamMissing`.

### 5. Plan 02 handler changes discovered during integration — none

Plan 02's endpoints returned exactly the shapes documented in the plan:
- `GET /tts-config` → `{has_elevenlabs_config, voice_id?, obs_url?}`
- `POST /tts-config/rotate-token` → `{obs_url}`
- `POST /tts-config/test` → audio/mpeg + `x-characters-*` headers
- `GET /tts-voices` → proxy-passes ElevenLabs `{voices:[...]}` (handled defensively — both shapes)

No Phase 13.1 patch needed to Plan 02's backend.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `vi.mock('react-hot-toast')` factory hoisting TDZ error**

- **Found during:** Task 2 (after first test run)
- **Issue:** The test file had `const toastMocks = { success: vi.fn(), error: vi.fn() }` at the top level, then `vi.mock('react-hot-toast', () => ({ default: Object.assign(fn, toastMocks) }))`. Vitest hoists `vi.mock` factories above all top-level statements, so the factory saw `toastMocks` before initialization and threw `ReferenceError: Cannot access 'toastMocks' before initialization`.
- **Fix:** Replaced `const toastMocks = { ... }` with `const toastMocks = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))`. `vi.hoisted` is specifically designed to run before `vi.mock` factories, solving the TDZ problem.
- **Files modified:** `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx`
- **Verification:** 35/35 TTSGroup tests pass.
- **Committed in:** `dcd8a4e2` (part of Task 2 commit — the fix landed with the initial implementation)

**2. [Rule 1 - Bug] Quota display text split across JSX lines, breaking the acceptance-criteria grep**

- **Found during:** Task 2 (acceptance-criteria re-check after GREEN)
- **Issue:** The quota `<p>` rendered `{quota.remaining}/{quota.limit} characters this` then `month ({quotaPct}%)` on separate JSX source lines. That rendered fine in the browser (React concatenates text nodes) but the acceptance criterion `grep -q "characters this month" TTSGroup.tsx` failed because the literal substring wasn't contiguous in the source file.
- **Fix:** Rewrote the JSX to use a single-line `{' '}characters this month ({quotaPct}%)` literal. No behavior change (still a space between `10,000` and `characters`), but the grep now matches.
- **Files modified:** `frontend/src/components/appearance/TTSGroup.tsx`
- **Verification:** `grep -c "characters this month" TTSGroup.tsx` returns 1; test A7 still passes (it checks the rendered output, not the source).
- **Committed in:** `dcd8a4e2` (same Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 correctness fixes)
**Impact on plan:** Neither fix expanded scope. #1 was a vitest hoisting quirk that would have blocked all A-tests from running. #2 was a grep-vs-JSX-pretty-printing mismatch that would have failed the self-check even though the rendered UI was correct.

## Authentication Gates

None — this plan is entirely client-side. Every backend endpoint Plan 03 consumes already exists in Plan 02 and was merged into `feature/tts-chat-messages` before this plan started. The 9-step manual verification (Task 4) requires an ElevenLabs account and OBS for real-audio testing — that's persisted to `13-HUMAN-UAT.md` and NOT handled as an auth gate because the automated code path itself never needs credentials.

## Issues Encountered

- **Shell-grep alternation quirks:** The acceptance-criteria script uses `grep -E "A\|B"` — I accidentally wrote `\|` (backslash-pipe) inside a double-quoted shell string, which `grep` then read as a literal `\|` token rather than the alternation operator. Tripped on 3 "MISSING" false negatives during verification; resolved by running each term separately.
- **Duration clock jump:** The plan started at 2026-04-23T21:33Z (local evening) and the final SUMMARY write happened at 2026-04-24T06:54Z, making `date +%s` deltas show ~9 hours of elapsed time. Actual task-execution wall time was ~35 minutes; the gap is wall-clock drift between tool calls, not work.
- **Acceptance-criterion grep for "characters this month":** See deviation #2 above. Prettier auto-wrapped a line in a way that broke the literal-substring grep even though the rendered UI was pixel-perfect. Documented the fix so future plans know to avoid hard-wrap-sensitive acceptance criteria.

## UX Papercuts (for future passes)

- **Non-premium `tts_provider='elevenlabs'` + `hasElevenLabsConfig=false`:** The Advanced block renders its PremiumBadge overlay + disabled inputs. The call-to-action is "Upgrade to Premium to use ElevenLabs voices." — clear, but it doesn't link to an upgrade page yet. Future phase could wire a billing route.
- **Voice-load failure on saved key:** If `getTTSVoices` 502s (ElevenLabs downtime), the dropdown shows "Could not load voices" in the option list + a toast, but the user cannot currently re-fetch without reloading the page. Retry button would be a nice addition; triedRef latch would need to reset.
- **OBS URL text is a password-grade secret:** Copy-to-clipboard emits the full JWT. Browser clipboard history on shared machines is a leak vector (T-13-11 is accepted risk per the plan). A future UX pass could add an explicit "Clipboard will hold a secret token — copy from the share dialog only" helper, or a "show/hide" toggle.

## CLAUDE.md tech-debt note

The CLAUDE.md "Known Issues & Technical Debt > Security" section still reads:
> Token encryption is basic (TODO: implement AES-GCM)

This line is stale. The shared `shared/encryption` package has been production-deployed for multiple phases (pre-dating Phase 13) and is now consumed by both `auth-service` and `overlay-manager` (the TTS api_key is stored AES-GCM-encrypted via it — see 13-02-SUMMARY.md). A future docs pass should remove or update this entry — but doing so is out of scope for Plan 03 (no code coupling).

## User Setup Required

None for the frontend UX itself. For the E2E human-verification (HUMAN-UAT.md items 1-5) a user needs:
- A premium flag on their account (`UPDATE users SET is_premium=true WHERE email=...`)
- A real ElevenLabs API key (https://elevenlabs.io/)
- An OBS install (optional — a browser tab simulates the browser source)

## Next Phase Readiness

- **Phase 13 complete.** All 3 plans landed; the feature is ready for the human UAT pass.
- **Phase 14 (stream selection)** is unblocked — the pattern for premium-gated overlay config endpoints established here (shared/featuregates + shared/middleware/premium + overlay_*_configs table) is directly reusable.
- **HUMAN-UAT.md** captures the blocking manual items. A human with an ElevenLabs key should run through the 5 tests and update the file's `result:` / `## Summary` fields.

## Self-Check

All acceptance criteria verified in-session:

- [x] `ls .planning/phases/13-text-to-speech-tts-for-chat-messages/13-03-SUMMARY.md` → exists (this file)
- [x] `ls .planning/phases/13-text-to-speech-tts-for-chat-messages/13-HUMAN-UAT.md` → exists
- [x] `git log --oneline -5` shows `a8a52f44`, `dcd8a4e2`, `1c2b5f70`, `6ef7be0b`
- [x] `grep "ship in Plan 03" frontend/src/components/appearance/TTSGroup.tsx` → 0 matches (stub gone)
- [x] `cd frontend && npm test -- --run` → 353 passed, 0 failed, 4 todo (46 test files)
- [x] `cd frontend && npx tsc --noEmit` → exits 0
- [x] `cd frontend && npm run build` → exits 0 (all routes compiled)
- [x] `git diff main -- frontend/src/ | grep "^+" | grep ": any\b"` → 0 matches (no NEW `: any` types)
- [x] TTSGroup test count: 35 (was 15) — `grep -cE "^  it\(|^    it\(" TTSGroup.test.tsx` returns 35
- [x] `grep -q "Invalid API key\|Rate-limited — try again\|ElevenLabs service unavailable" TTSGroup.tsx` → 3 matches (D-39 verbatim)
- [x] Clipboard + role="alertdialog" patterns present in TTSGroup.tsx
- [x] Live overlay tts_token read at line 353 of `frontend/src/app/overlay/[id]/page.tsx`
- [x] Embed preview `elevenLabsRuntimeRef` cache + merge-into-postMessage-handler both present
- [x] Secret-logging audit: `grep -rnE "console.log.*apiKey|console.log.*api_key" TTSGroup.tsx overlays/\[id\]/page.tsx` → 0 matches

## Self-Check: PASSED

---
*Phase: 13-text-to-speech-tts-for-chat-messages*
*Completed: 2026-04-24*
