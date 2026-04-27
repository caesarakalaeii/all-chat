---
phase: 13-text-to-speech-tts-for-chat-messages
plan: 01
subsystem: ui

tags: [tts, web-speech-api, react, typescript, nextjs, vitest, tdd]

# Dependency graph
requires:
  - phase: 11-add-username-keyword-exclude-list-to-overlay-filter-settings
    provides: shouldFilterMessage pattern; filter-guard integration point in overlay + embed pages
  - phase: 12-notification-sound-on-incoming-messages-with-premium-custom-
    provides: SoundGroup/soundPlayer analog pattern; display_settings JSONB extension; SOUND_SETTINGS_UPDATE postMessage
provides:
  - DisplaySettings extended with 20 tts_* fields
  - ttsPlayer.ts client-side TTS pipeline (queue, sampling, cooldown, rate limit, staleness, priority, session fallback)
  - useBrowserVoices React hook with Chromium voiceschanged fix
  - TTSGroup settings component with 5 sub-sections and Advanced (ElevenLabs) stub
  - AppearancePanel CollapsibleSection id="tts" mount
  - overlay/[id] + overlays/[id]/preview/embed integration with TTS_SETTINGS_UPDATE postMessage
  - D-24..D-38, D-41, D-42 decisions implemented client-side
affects: [13-02 backend, 13-03 ElevenLabs UX]

# Tech tracking
tech-stack:
  added: [Web Speech API usage (speechSynthesis, SpeechSynthesisUtterance)]
  patterns:
    - "Pure client-side utility + React group component + AppearancePanel mount (three-file pattern, mirrors Phase 11/12)"
    - "TDD for pure utilities — mocked global speechSynthesis + SpeechSynthesisUtterance"
    - "Session-wide ElevenLabs → Web Speech fallback with one-shot toast (D-38)"
    - "TTS_SETTINGS_UPDATE postMessage for live editor preview (mirrors SOUND_SETTINGS_UPDATE)"
    - "Per-overlay fetch URL with tts_token + voice + text query params (proxy contract for Plan 02)"

key-files:
  created:
    - frontend/src/lib/utils/ttsPlayer.ts
    - frontend/src/lib/utils/__tests__/ttsPlayer.test.ts
    - frontend/src/lib/hooks/useBrowserVoices.ts
    - frontend/src/components/appearance/TTSGroup.tsx
    - frontend/src/components/appearance/__tests__/TTSGroup.test.tsx
  modified:
    - frontend/src/lib/types/overlay.ts
    - frontend/src/components/appearance/AppearancePanel.tsx
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx

key-decisions:
  - "ChatMessage.message is a MessageInfo object ({text, emotes}); ttsPlayer reads msg.message.text and msg.message.emotes (not msg.message directly)"
  - "Event metadata (bits count, raider viewer count) lives in msg.event.metadata (Record<string, unknown>); ttsPlayer uses a safe eventNumber() accessor"
  - "D-30 skip_links logic: skip only when, after removing URLs entirely, remainder is pure punct/whitespace AND the message originally contained a URL. Otherwise URLs become literal 'link'"
  - "TTSGroup Advanced (ElevenLabs) block is an intentional stub in Plan 01; Plan 03 replaces it with full ElevenLabs UX (API key input, voice picker, Test-Key button, OBS URL, quota display)"
  - "useBrowserVoices cleanup uses a captured synth reference and try/catch to survive teardown-time global removal (test environment)"

patterns-established:
  - "Pattern: pure TypeScript utility with fake-timer-safe microtask pump (see pump() in ttsPlayer.ts). Downstream plans that need queue-based media should mirror this."
  - "Pattern: global-stub + window-installed speechSynthesis mock for jsdom tests. Use Object.defineProperty on window rather than only vi.stubGlobal, because hooks read window.speechSynthesis."

requirements-completed:
  - D-18
  - D-19
  - D-20
  - D-21
  - D-22
  - D-24
  - D-25
  - D-26
  - D-27
  - D-28
  - D-29
  - D-30
  - D-31
  - D-32
  - D-33
  - D-34
  - D-35
  - D-36
  - D-37
  - D-38
  - D-41
  - D-42

# Metrics
duration: 28min
completed: 2026-04-23
---

# Phase 13 Plan 01: Web Speech TTS Tier Summary

**Client-side Text-to-Speech pipeline with queue, sampling, per-user cooldown, token-bucket rate limit, staleness, priority event handling, and session-wide ElevenLabs→Web-Speech fallback, plus TTSGroup UI and overlay+embed integration.**

## Performance

- **Duration:** ~28 min
- **Started:** 2026-04-23T18:40:48Z
- **Completed:** 2026-04-23T19:08:29Z
- **Tasks:** 3
- **Files created:** 5
- **Files modified:** 4

## Accomplishments

- Shipped `ttsPlayer.ts` with the full D-25..D-38 pipeline: content formatting (username/platform prefix, emote stripping, URL→"link", event-specific prefixes for the 11 priority event types), FIFO queue with priority-eviction (D-33/D-34), per-user cooldown (D-35), token-bucket rate limiter (D-36), dequeue-time staleness check (D-37), session-wide ElevenLabs fallback with one-shot callback (D-38), and voice-URI console.warn fallback (D-28).
- 40 unit tests covering every decision explicitly (D-25..D-38) using mocked `speechSynthesis`, `SpeechSynthesisUtterance`, `fetch`, `Audio`, and `URL.createObjectURL`. All green.
- `useBrowserVoices()` hook subscribes to `voiceschanged` (Chromium async-load fix) and defensively cleans up when the global is removed before unmount (test environment).
- `TTSGroup.tsx` renders the full 5-sub-section layout per `13-UI-SPEC.md`: Voice (provider radio, volume, voice picker, speech rate, pitch [hidden for ElevenLabs], Test-voice button), Throttling (filter-mode radio, sample rate [conditional], 4 NumberControls), Content (read-username/platform toggles, max-chars, skip-emote-only/skip-links toggles, PlatformChipRow), Priority (announce-events toggle, min-bits NumberControl [conditional]), and Advanced (ElevenLabs stub). 15 component tests green.
- `AppearancePanel` mounts `CollapsibleSection id="tts" title="Text-to-Speech"` immediately after the Notification Sounds block, gated on `displaySettings && onTTSChange && overlayId`.
- Live overlay page (`/overlay/[id]`) and editor embed preview (`/overlays/[id]/preview/embed`) both instantiate `ttsPlayer`, load `tts_*` settings on mount from `display_settings`, call `ttsPlayerRef.current?.speak(message)` adjacent to `soundPlayerRef.current?.play()` AFTER the `shouldFilterMessage` guard (D-41, D-42), and show one `"ElevenLabs unavailable — using browser voice."` toast on session fallback.
- Embed preview handles the `TTS_SETTINGS_UPDATE` postMessage from the editor (D-22) with full per-field merging against the current ref, so no debounce is needed.

## Task Commits

1. **Task 1: DisplaySettings extension + ttsPlayer.ts (TDD RED→GREEN) + useBrowserVoices hook** — `d778322e` (feat)
2. **Task 2: TTSGroup.tsx + TTSGroup.test.tsx + AppearancePanel mount** — `fb226b2c` (feat)
3. **Task 3: Wire ttsPlayer into overlay/[id]/page.tsx + preview/embed/page.tsx** — `9673a277` (feat)

_Note: Task 1 and Task 2 both followed the RED→GREEN TDD flow. They were combined into a single `feat` commit per task (the project does not yet have a per-phase "test" commit convention, and the task definition explicitly says "Commit: feat(...)" after both phases pass). Commits include both the test file and the implementation._

## Files Created/Modified

### Created
- `frontend/src/lib/utils/ttsPlayer.ts` — Core TTS pipeline (queue, sampling, cooldown, rate limiter, staleness, priority, session fallback)
- `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` — 40-case TDD suite with coverage matrix below
- `frontend/src/lib/hooks/useBrowserVoices.ts` — React hook returning `SpeechSynthesisVoice[]` with `voiceschanged` subscription
- `frontend/src/components/appearance/TTSGroup.tsx` — Settings UI component with 5 sub-sections
- `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` — 15 component tests

### Modified
- `frontend/src/lib/types/overlay.ts` — `DisplaySettings` extended with 20 `tts_*` fields per D-24
- `frontend/src/components/appearance/AppearancePanel.tsx` — New `CollapsibleSection id="tts"` mount + 11 new optional props for TTS wiring
- `frontend/src/app/overlay/[id]/page.tsx` — TTS refs, config load, destroy effect, session-fallback toast, `speak(message)` call at line 511 (adjacent to `soundPlayerRef.current?.play()` on line 509, after `shouldFilterMessage` guard on line 506)
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — TTS refs, config load, destroy effect, session-fallback toast, `TTS_SETTINGS_UPDATE` postMessage handler, `speak(message)` call at line 528 (adjacent to `soundPlayerRef.current?.play()` on line 526, after `shouldFilterMessage` guard on line 523)

## Decisions Made

- **Schema adaptation (D-24):** The plan referenced `msg.message` as a string, but the canonical `ChatMessage` type has `message: MessageInfo` (`{text, emotes}`). All accesses go through `msg.message?.text ?? ''` and `msg.message?.emotes ?? []`.
- **Emote position format:** `Emote.positions` is `number[][]` (each inner array is `[start, end]`), not `{start, end}[]`. The `stripEmotes` helper normalizes these before sorting and stripping.
- **Event metadata extraction:** Bits count (for priority-bits-min gate and "cheered N bits" prefix) and raid viewer count live in `msg.event.metadata` (`Record<string, unknown>`), not on the event object directly. A safe `eventNumber()` accessor handles number/string coercion.
- **D-30 skip_links semantics:** The plan's one-liner ("if message after stripping contains only whitespace/punctuation AND skip_links=true, skip entirely") was ambiguous about whether "stripping" meant replace-with-"link" or replace-with-empty. Chose the latter: skip fires only when (a) message originally contained a URL AND (b) removing URLs entirely leaves only punct/whitespace. This means a pure-URL message with skip_links=true is skipped (matches Phase 13 intent — user just dropped a URL, no real content), while "check this https://example.com out" becomes "check this link out" and speaks.
- **useBrowserVoices defensive cleanup:** jsdom does not implement `speechSynthesis`. Tests install/remove the mock on `window.speechSynthesis` between tests. The hook captures a `synth` reference at effect-start and wraps `removeEventListener` in try/catch so a removed-before-unmount scenario doesn't throw. Production browsers never hit this path.

## Test Coverage Matrix (ttsPlayer.test.ts — 40 cases)

| Decision | Tests |
| --- | --- |
| Basic API | 3 tests (factory shape, enabled=false no-op, enabled=true speaks) |
| D-25 username prefix | 1 test |
| D-26 platform prefix | 1 test |
| D-29 emote stripping | 2 tests (strip positions, skip emote-heavy) |
| D-30 URL→"link" + skip_links | 3 tests (inline URL, skip link-only, pure-URL speaks when skip_links=false) |
| D-32 like_aggregate exclusion | 1 test |
| Platform filter | 1 test |
| D-31 sampling | 2 tests (rate=0 never, rate=1 always) |
| D-31 priority events | 5 tests (speaks in priority_only mode; subscription/raid/bits/super_chat prefix format; PRIORITY_EVENTS set contents) |
| D-35 per-user cooldown | 2 tests (suppress within window; priority bypass) |
| D-36 token bucket | 3 tests (drop over capacity; refill after 60s; priority bypass) |
| D-33 queue overflow | 2 tests (non-priority drop; priority evicts oldest non-priority) |
| D-34 FIFO | 1 test (priority + non-priority insertion order) |
| D-37 staleness | 1 test (stale dequeue drop) |
| D-38 session fallback | 6 tests (fetch URL format; 400/401/429/500/network error each trigger one-shot onFallback) |
| D-28 voice URI fallback | 2 tests (missing voice → warn + default; matching voice → set) |
| Utterance properties | 1 test (volume/rate/pitch applied) |
| destroy + updateSettings | 2 tests |

**Total: 40 test cases, all green. Run time ~35ms.**

## Test Coverage Matrix (TTSGroup.test.tsx — 15 cases)

| Behavior | Test |
| --- | --- |
| Master toggle fires `{tts_enabled: !current}` | 1 |
| Sub-section headers hidden when disabled | 2 |
| Sub-section headers visible when enabled | 3 |
| Provider radio + ElevenLabs disabled for non-premium | 4 |
| Sliders fire `onChange` with mapped `tts_*` field | 5 |
| Voice picker populated from `useBrowserVoices` | 6 |
| Pitch slider hidden when provider=elevenlabs | 7 |
| Sample rate slider hidden when filter_mode≠sample | 8 |
| Min-bits field hidden when priority_events=false | 9 |
| Platform chip row toggles state | 10 |
| Test-voice button invokes `onPreview` | 11 |
| Advanced block hidden when provider=browser | 12 |
| Advanced stub renders with Plan 03 placeholder | 13 |
| Non-premium user sees disabled ElevenLabs + upgrade copy | 14 |
| Master toggle disabled + helper when speechSynthesis undefined | 15 |

## Deviations from Plan

None of the deviations below changed the **plan behavior** — they adapted the plan's pseudo-code to the actual codebase schema and tooling. All were auto-fixed per Rule 3 (blocking) or Rule 1 (correctness).

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted ttsPlayer to actual ChatMessage schema**
- **Found during:** Task 1 (ttsPlayer.ts implementation)
- **Issue:** Plan's pseudo-code assumed `msg.message` is a string and `msg.emotes` is a top-level field with `positions: {start, end}[]`. Real `ChatMessage` has `msg.message: {text, emotes}` and `Emote.positions: number[][]`.
- **Fix:** All accesses normalized — `msg.message?.text ?? ''`, `msg.message?.emotes ?? []`, and `stripEmotes` normalizes `number[][]` positions into tuples before processing. Safe `eventNumber()` accessor extracts `bits`/`viewers` from `msg.event.metadata`.
- **Files modified:** `frontend/src/lib/utils/ttsPlayer.ts`
- **Verification:** 40 unit tests green; tests exercise actual ChatMessage shapes.
- **Committed in:** `d778322e` (Task 1 commit)

**2. [Rule 1 - Bug] Defensive cleanup in useBrowserVoices**
- **Found during:** Task 2 (TTSGroup tests with jsdom)
- **Issue:** Test 15 removes `window.speechSynthesis` to verify fallback UI. The useBrowserVoices cleanup function tried to call `removeEventListener` on the removed global at unmount, producing `TypeError: Cannot read properties of undefined`.
- **Fix:** Capture `synth` reference at effect-start and wrap `removeEventListener` in try/catch.
- **Files modified:** `frontend/src/lib/hooks/useBrowserVoices.ts`
- **Verification:** All 15 TTSGroup tests green; no regression in production code path.
- **Committed in:** `fb226b2c` (Task 2 commit)

**3. [Rule 1 - Clarification] D-30 skip_links semantics**
- **Found during:** Task 1 (writing ttsPlayer tests)
- **Issue:** The plan's behavior list had two tests (9 and 9b) that were individually testing overlapping cases of the same decision (D-30), but phrased ambiguously. Plan 9's phrasing "skips when message is link-only" conflicted with a comment in the plan that suggested "link" should be the spoken output.
- **Fix:** Aligned with D-30's intent (skip link-only; speak inline links as "link"). Test 9: `skip_links=true` + pure URL → skipped. Test 9b: `skip_links=false` + pure URL → speaks "link". Test 8: inline URL with context → "check link now" regardless of skip_links. Implementation checks both "had URL" AND "non-URL remainder is punct/whitespace" before skipping.
- **Files modified:** `frontend/src/lib/utils/ttsPlayer.ts`, `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts`
- **Verification:** Tests 8, 9, 9b all green; matches D-30 one-liner.
- **Committed in:** `d778322e` (Task 1 commit)

---

**Total deviations:** 3 auto-fixed (1 Rule 3, 2 Rule 1).
**Impact on plan:** No scope creep. All three were necessary to integrate the plan's pseudo-code with the actual codebase.

## Issues Encountered

- **vi.fn generic signature mismatch:** Vitest 4.x has a different `vi.fn<T>()` signature than older versions (only accepts one type argument, not two). Initial attempt to use `vi.fn<[Parameters<OnChangeFn>, ReturnType<OnChangeFn>]>` failed TypeScript. Resolved by casting `vi.fn() as ReturnType<typeof vi.fn> & OnChangeFn`.
- **jsdom does not implement Web Speech API.** Confirmed via a short node script. TTSGroup tests must install `speechSynthesis` on `window` directly (not just `vi.stubGlobal`) because the component reads `window.speechSynthesis`. Documented this as a pattern in the summary.
- **autoResolveOnEnd toggle pattern in queue tests:** Queue-overflow tests (22/23) needed to hold the "speaking" state open across multiple `speak()` calls. Implemented by flipping a module-scoped `autoResolveOnEnd` flag that controls whether `mockSpeak` auto-resolves `onend`. Tests explicitly re-fire `lastUtterance?.onend?.()` to drain the queue.

## Known Stubs

### TTSGroup Advanced (ElevenLabs) Block

- **File:** `frontend/src/components/appearance/TTSGroup.tsx`
- **Lines:** Advanced block — `provider === 'elevenlabs'` branch renders a dashed-border container with copy `"ElevenLabs controls (API key, voice picker, test, OBS URL) ship in Plan 03."`
- **Reason:** Intentional — Plan 03 will replace the stub with `ApiKeyInput`, `QuotaDisplay`, `ObsUrlPanel`, `RegenerateConfirmDialog`, and the ElevenLabs VoicePicker per UI-SPEC. The stub is gated on `provider === 'elevenlabs'`, which is itself disabled unless `isPremium=true`, so no end-user can reach it in production without a premium account.
- **Plan that resolves:** Plan 13-03.

### ElevenLabs runtime not wired

- **Files:** `frontend/src/app/overlay/[id]/page.tsx`, `frontend/src/app/overlays/[id]/preview/embed/page.tsx`
- **Status:** ttsPlayer is instantiated with `provider='browser'` even when `display.tts_provider='elevenlabs'` would be set, because the `ttsEndpoint`, `ttsToken`, and `voiceId` runtime fields are not populated in this plan. The fetch/fallback code path EXISTS and is unit-tested, but is dormant until Plan 03 wires the `/api/v1/overlays/:id/tts` endpoint (Plan 02) and the OBS URL token (Plan 03).
- **Reason:** Backend endpoint (Plan 02) and user-facing key management (Plan 03) are sequential prerequisites.
- **Plan that resolves:** Plan 13-03.

## Threat Flags

None. All threat-model items in the plan's `<threat_model>` are handled:
- **T-13-FE-01 (formatContent → SpeechSynthesisUtterance injection):** `SpeechSynthesisUtterance.text` is plain-text only; no HTML rendering. URL regex replaces before concatenation.
- **T-13-FE-02 (fingerprinting via voice_uri):** Accepted risk; user opts in by saving voice. `console.warn` surfaces fallback (D-28).
- **T-13-FE-03 (stale tts_token):** Plan 01 only handles URL transport (query param passthrough). Rotation + enforcement live in Plan 02.

No new surface introduced beyond what the plan anticipated.

## User Setup Required

None — no external service configuration required for the Web Speech tier. The ElevenLabs tier (Plans 02 + 03) will require user-supplied API keys.

## Next Phase Readiness

- **Plan 13-02 (backend) can proceed independently.** Plan 13-01 produces display-settings fields and the `ttsEndpoint?`/`ttsToken?`/`voiceId?` TTSSettings contract. Plan 13-02 builds the backend endpoint; Plan 13-03 ties them together.
- **Plan 13-03 can consume TTSGroup's Advanced block** by replacing the stub with the full ElevenLabs UX. The TTSGroupProps already exports all the async callback slots (`onSaveKey`, `onTestKey`, `onRotateToken`, `onRemoveKey`, `onFetchVoices`) that Plan 03 will fill in.

## Self-Check: PASSED

- `ls .planning/phases/13-text-to-speech-tts-for-chat-messages/13-01-SUMMARY.md` → exists (this file).
- `git log --oneline -5` shows `d778322e`, `fb226b2c`, `9673a277`.
- `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts` → 40/40 passing.
- `cd frontend && npm test -- --run src/components/appearance/__tests__/TTSGroup.test.tsx` → 15/15 passing.
- `cd frontend && npx tsc --noEmit` → 0 errors.
- `cd frontend && npm run build` → success.
- AGPL-3.0 header present on every new .ts/.tsx file (verified via `head -2 | grep "This file is part of All-Chat"`).
- No `: any` types in new files (verified via `grep ": any\b" ...` → 0 matches).

---
*Phase: 13-text-to-speech-tts-for-chat-messages*
*Completed: 2026-04-23*
