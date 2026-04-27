---
status: partial
phase: 13-text-to-speech-tts-for-chat-messages
source: [13-03-PLAN.md]
started: 2026-04-24
updated: 2026-04-24
---

## Current Test

[Test 1 blocked on real ElevenLabs account — all other tests verified via Playwright against live local stack]

## Tests

### 1. ElevenLabs real-key happy path
expected: Save real ElevenLabs key -> Test-Key -> hear audible sample, quota displayed (format `8,432 / 10,000 characters this month (84%)`)
result: blocked — requires a real ElevenLabs API key (no automated path)
notes: Fake-key analog passed — Test-Key with `sk-fake-…` returned HTTP 401 `{"error":"Invalid API key"}` and the UI rendered toast `Invalid API key` (D-39 verified).

### 2. Copy OBS URL -> paste in OBS -> send chat -> hear ElevenLabs voice
expected: OBS browser source renders overlay with audible TTS via ElevenLabs voice
result: partial — URL format + JWT claims + encrypted-at-rest storage verified; live OBS runtime still requires user testing with a real ElevenLabs key.
notes: GET /tts-config returns `obs_url: http://localhost:3000/overlay/{id}?tts_token=…` with decoded JWT `{scope: "tts:use", sub: overlay_id, iss: "all-chat", iat: …}` — no `exp` (D-08 no-expiry confirmed). Ciphertext row in `overlay_tts_configs` is 100 bytes, plaintext substring check returned FALSE.

### 3. Regenerate OBS URL invalidates prior URL
expected: Old URL returns 401 in POST /tts; new URL works
result: passed (Playwright UAT 2026-04-24)
notes: curl sequence — token A → rotate-token endpoint → curl POST /tts with token A returned HTTP 401 `{"error":"unauthorized"}`, curl with new token B returned HTTP 401 `{"detail":{"status":"invalid_api_key"}}` (JWT valid, ElevenLabs rejects fake key). D-10 rotation-based revocation confirmed.

### 4. ElevenLabs failure triggers session-wide Web Speech fallback + one-time toast
expected: Invalid key at runtime -> toast "ElevenLabs unavailable — using browser voice." -> subsequent messages use Web Speech
result: verified via unit tests (40/40 ttsPlayer tests including D-38 session-fallback for 401/429/500/network/400), backend API level. Full runtime fallback (save invalid key → enable TTS → send chat → observe one-time toast) still requires manual verification with live chat.
notes: Test 27a..27d in ttsPlayer.test.ts covers all HTTP error paths. The runtime toast copy `"ElevenLabs unavailable — using browser voice."` is present in source (TTSGroup.tsx) and confirmed mountable via react-hot-toast Toaster (fix committed in 264104cd).

### 5. Remove key clears row + revokes access
expected: DELETE /tts-config -> POST /tts returns 404/401 -> UI shows "no key saved" state
result: passed (Playwright UAT 2026-04-24)
notes: Two-click remove flow (arm → confirm) triggered DELETE. Post-remove: `SELECT COUNT(*) FROM overlay_tts_configs WHERE overlay_id=…` returned 0; POST /tts returned 401 `unauthorized` (signing secret gone); GET /tts-config returned `{has_elevenlabs_config: false}`; UI reverted to empty state with "Save key" button.

## Summary

total: 5
passed: 2 (#3, #5)
issues: 0
pending: 2 (#1, #2 — need real ElevenLabs key + live OBS)
blocked: 1 (#4 partial — unit-level green, runtime-level still manual)

## Bugs Discovered During UAT (and Fixed)

| ID | Severity | Description | Fix Commit |
|----|----------|-------------|-----------|
| UAT-01 | high | api-gateway/cmd/main.go missing forwards for 7 TTS endpoints — Plan 02 wired them into overlay-manager but the gateway layer was overlooked. 404 at /api/v1/overlays/:id/tts-config. | 264104cd |
| UAT-02 | medium | frontend/src/app/layout.tsx missing `<Toaster>` mount — TTSGroup.tsx imports react-hot-toast but the shell only had base-ui ToastProvider, so toast.error/success calls fired silently. | 264104cd |
| UAT-03 | medium | docker-compose.frontend.yml missing TOKEN_ENCRYPTION_KEY and OVERLAY_PUBLIC_BASE_URL env vars for overlay-manager — dev stack couldn't boot Phase 13 code. | 264104cd |

## Gaps

- 5 is only partially verified (unit + static); a runtime test of the session-wide fallback UX still requires a human with an invalid key mid-session to confirm the toast fires exactly once and the rest of the session uses Web Speech.
- 1 + 2 require a real ElevenLabs account with a subscription to test quota display and audio streaming. No automated path possible.
