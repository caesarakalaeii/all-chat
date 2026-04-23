---
phase: 13
slug: text-to-speech-tts-for-chat-messages
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-23
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: research `## Validation Architecture` section (see 13-RESEARCH.md:1032).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Frontend framework** | `vitest` + `@testing-library/react` |
| **Frontend config** | `frontend/vitest.config.ts` (existing) |
| **Frontend quick run** | `cd frontend && npm test -- --run <path>` |
| **Frontend full suite** | `cd frontend && npm test -- --run` |
| **Backend framework** | `go test` + `stretchr/testify` |
| **Backend config** | none (built-in) |
| **Backend quick run** | `go test ./services/overlay-manager/... -run <TestName> -v` |
| **Backend full suite** | `make test` |
| **Estimated quick runtime** | ~15 seconds (ttsPlayer suite) |
| **Estimated wave runtime** | ~2 minutes (frontend + overlay-manager) |

---

## Sampling Rate

- **After every task commit:** Frontend unit suite for the touched file (e.g. `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts`) or the Go package (e.g. `go test ./services/overlay-manager/handlers/... -run TestHandleTTS`).
- **After every plan wave:** `cd frontend && npm test -- --run && go test ./services/overlay-manager/...` (~2 minutes).
- **Before `/gsd-verify-work`:** `make test` — full suite must be green.
- **Max feedback latency:** 15 seconds for per-task sampling; 2 minutes for wave merge gate.

---

## Per-Decision Verification Map

| Decision | Behavior | Test Type | Automated Command | Test File | Status |
|----------|----------|-----------|-------------------|-----------|--------|
| D-03 | `tts` feature-gate row exists after migration 049 | integration | `go test ./migrations/... -run TestMigration049` | ❌ W0 | ⬜ pending |
| D-05 | Public config endpoint does NOT return encrypted API key | integration | `go test ./services/overlay-manager/handlers/... -run TestPublicConfigHidesTTSKey` | ❌ W0 | ⬜ pending |
| D-06, D-07 | AES-GCM roundtrip with random nonce | unit | `go test ./shared/encryption/ -run TestEncryptDecryptRoundTrip` | ✅ (exists) | ⬜ pending |
| D-06 | DB stores ciphertext not plaintext for ElevenLabs key | integration | `go test ./services/overlay-manager/handlers/... -run TestSaveTTSConfigEncryptsKey` | ❌ W0 | ⬜ pending |
| D-08 | JWT signed with per-overlay secret, claims validate | unit | `go test ./services/overlay-manager/tts/ -run TestSignVerifyJWT` | ❌ W0 | ⬜ pending |
| D-08, D-10 | Rotating secret invalidates old JWTs | unit | `go test ./services/overlay-manager/tts/ -run TestRotationInvalidatesOldTokens` | ❌ W0 | ⬜ pending |
| D-11 | `POST /tts-config` requires premium (RequirePremium middleware) | integration | `go test ./services/overlay-manager/handlers/... -run TestSaveTTSConfigRequiresPremium` | ❌ W0 | ⬜ pending |
| D-12 | `DELETE /tts-config` removes encrypted row | integration | `go test ./services/overlay-manager/handlers/... -run TestDeleteTTSConfig` | ❌ W0 | ⬜ pending |
| D-13 | `POST /tts-config/rotate-token` rotates secret and returns new OBS URL | integration | `go test ./services/overlay-manager/handlers/... -run TestRotateTokenReturnsNewURL` | ❌ W0 | ⬜ pending |
| D-14 | `GET /tts-voices` proxies to ElevenLabs | integration | `go test ./services/overlay-manager/handlers/... -run TestGetVoicesProxies` | ❌ W0 | ⬜ pending |
| D-15 | `POST /tts-config/test` validates key and returns quota + sample stream | integration | `go test ./services/overlay-manager/handlers/... -run TestTestKeyHandler` | ❌ W0 | ⬜ pending |
| D-16 | `POST /tts` streams audio with audio/mpeg Content-Type | integration | `go test ./services/overlay-manager/handlers/... -run TestHandleTTSStreamsAudioMpeg` | ❌ W0 | ⬜ pending |
| D-16 | `POST /tts` rejects missing/invalid `tts_token` with 401 | integration | `go test ./services/overlay-manager/handlers/... -run TestHandleTTSRequiresToken` | ❌ W0 | ⬜ pending |
| D-16 | Client disconnect propagates context cancellation to upstream | integration | `go test ./services/overlay-manager/handlers/... -run TestHandleTTSCancelPropagates` | ❌ W0 | ⬜ pending |
| D-19..D-24 | `TTSGroup` renders all 20 settings | unit | `cd frontend && npm test -- --run src/components/appearance/__tests__/TTSGroup.test.tsx` | ❌ W0 | ⬜ pending |
| D-22 | `TTS_SETTINGS_UPDATE` postMessage updates the player | unit | `cd frontend && npm test -- --run src/app/overlays/[id]/preview/embed/__tests__/tts-update.test.tsx` | ❌ W0 | ⬜ pending |
| D-25..D-30 | Content formatter: username "says", URL→"link", emote strip | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "formatContent"` | ❌ W0 | ⬜ pending |
| D-31 | Priority event detection: sub/raid/bits speak with prefix | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "priority"` | ❌ W0 | ⬜ pending |
| D-32 | `like_aggregate` always excluded | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "excludes like_aggregate"` | ❌ W0 | ⬜ pending |
| D-33 | Priority event + full queue drops oldest non-priority | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "drop oldest non-priority"` | ❌ W0 | ⬜ pending |
| D-34 | FIFO ordering preserved | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "FIFO"` | ❌ W0 | ⬜ pending |
| D-35 | Per-user cooldown suppresses rapid repeat messages | unit (fake timers) | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "cooldown"` | ❌ W0 | ⬜ pending |
| D-36 | Token bucket consumed; priority bypasses | unit (fake timers) | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "token bucket"` | ❌ W0 | ⬜ pending |
| D-37 | Staleness drops old messages at dequeue | unit (fake timers) | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "stale"` | ❌ W0 | ⬜ pending |
| D-38 | ElevenLabs failure sets sessionFallback; next speak goes to Web Speech | unit | `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts -t "session fallback"` | ❌ W0 | ⬜ pending |
| D-41 | Live overlay page calls `ttsPlayerRef.current?.speak` after filter, alongside sound | manual smoke | `make frontend-dev` → open overlay → send mock message → verify TTS triggers | manual | ⬜ pending |
| D-42 | Filter-blocked messages trigger neither sound nor TTS | unit | `cd frontend && npm test -- --run src/app/overlay/[id]/__tests__/page.test.tsx -t "filtered message"` | ❌ W0 (optional) | ⬜ pending |
| E2E | OBS URL + token grants TTS access; rotation revokes | manual | Copy OBS URL → paste in OBS → message → hear speech → rotate → refresh OBS → TTS fails | manual | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` — covers D-25..D-38 (~30 cases)
- [ ] `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` — covers D-19..D-23
- [ ] `services/overlay-manager/handlers/tts_test.go` — covers D-11..D-16
- [ ] `services/overlay-manager/tts/jwt_test.go` — covers D-08, D-10
- [ ] `services/overlay-manager/repository/tts_config_repo_test.go` — covers D-06 DB roundtrip
- [ ] (existing) `shared/encryption/encryption_test.go` already covers AES-GCM roundtrip — reuse
- [ ] (existing, if moved) `shared/middleware/premium_test.go` — ships with the featuregates move

**Framework install:** None. `vitest` + `go test` already installed.

---

## Manual-Only Verifications

| Behavior | Decision | Why Manual | Instructions |
|----------|----------|------------|--------------|
| OBS browser-source playback with JWT in URL | D-08, D-09 | Requires OBS/CEF runtime, not headless | Generate URL in editor → paste in OBS browser source → send chat message → hear speech |
| Regenerate OBS URL invalidates old token | D-10 | Same runtime requirement | Refresh OBS source with stale token after rotation → TTS silent, proxy returns 401 |
| Web Speech playback across browsers | D-27, D-28 | Voice availability varies per browser/OS | Test in Chromium/Firefox/OBS CEF, confirm voice fallback when persisted URI missing |
| ElevenLabs live streaming audio | D-16, D-40 | Needs real key and quota | Save key → test-voice button → hear audible sample within ~1 second |
| ElevenLabs failure toast | D-38 | Requires real failure condition | Save invalid key → trigger TTS → observe "ElevenLabs unavailable — using browser voice." toast, subsequent messages use Web Speech |
| Character-quota display | D-23 | Requires real ElevenLabs account | Click Test-Key → observe remaining quota string |

---

## Validation Sign-Off

- [ ] All tasks include `<acceptance_criteria>` with grep/test-verifiable conditions
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags (`--watch` disallowed in automated commands)
- [ ] Feedback latency < 15s per task
- [ ] `nyquist_compliant: true` set in frontmatter once Wave 0 tasks land

**Approval:** pending
