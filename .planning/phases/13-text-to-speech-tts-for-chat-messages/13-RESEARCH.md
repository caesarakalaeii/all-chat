# Phase 13: Text-to-Speech (TTS) for chat messages - Research

**Researched:** 2026-04-23
**Domain:** Browser-native speech synthesis + ElevenLabs streaming proxy + AES-GCM key storage + per-overlay JWT
**Confidence:** HIGH

## Summary

Phase 13 is built almost entirely out of patterns that already exist in this codebase. The two prior phases (11, 12) establish the client-side utility + AppearancePanel group + postMessage live-update pattern that this phase mirrors line-for-line. The feature-gate infrastructure (Phase 07) is ready and already includes `RequirePremium` middleware and `FeatureGateCache`. **Crucially, AES-GCM encryption is already implemented and production-deployed** via `shared/encryption.AESEncryptor` (wired into auth-service for OAuth token storage) — the CLAUDE.md tech-debt note is stale. A second `shared/crypto.AESGCMCipher` also exists. The phase should REUSE `shared/encryption`, not rebuild it.

The real net-new engineering is: (1) a new DB migration for `overlay_tts_configs`, (2) wiring `featuregates.FeatureGateCache` + `RequirePremium` into overlay-manager (which currently has neither), (3) a per-overlay JWT sign/verify helper for the OBS-URL token scheme, (4) a streaming HTTP proxy to ElevenLabs, (5) the `ttsPlayer.ts` utility with queue/sampling/cooldown/rate-limiter, and (6) the UI group.

The ElevenLabs streaming endpoint returns raw audio bytes via chunked transfer — Gin's `c.Stream()` or a plain `io.Copy(c.Writer, resp.Body)` both work (the latter is already the api-gateway proxy pattern at `services/api-gateway/handlers/proxy.go:130`). Web Speech API's `getVoices()` timing is the one browser quirk: Chromium returns `[]` on first call and fires `onvoiceschanged` later; Firefox/Safari return synchronously. The `ttsPlayer.ts` must handle both.

**Primary recommendation:** REUSE `shared/encryption` instead of building `shared/crypto` (D-07 says "new package" — override this as Claude's Discretion allows: the package already exists with the exact API spec'd). MOVE `featuregates` and `middleware.RequirePremium` to `shared/` since overlay-manager is the second consumer. Mirror Phase 12 exactly for the frontend structure.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Phase Scope & Tiering (D-01 – D-04):**
- D-01: Both tiers (Web Speech free + ElevenLabs premium) ship in Phase 13. Single phase, full feature.
- D-02: Web Speech tier is free. ElevenLabs tier is premium.
- D-03: TTS registered as `feature_gates` row (`tts`, `is_premium=true`, description `"Text-to-speech for chat messages"`).
- D-04: 3 plans — Plan 01 Web Speech tier, Plan 02 ElevenLabs backend, Plan 03 ElevenLabs frontend + UX.

**ElevenLabs Key Storage & Auth (D-05 – D-10):**
- D-05: ElevenLabs API key is stored **server-side only**, never in the browser. Overlay calls a backend TTS proxy.
- D-06: Key stored **AES-GCM encrypted** in new `overlay_tts_configs` table (fields: `overlay_id`, `encrypted_api_key`, `voice_id`, `tts_signing_secret`, timestamps).
- D-07: "New `shared/crypto` package" — **researcher overrides this** per Claude's Discretion: `shared/encryption` already exists with the exact API and is production-deployed. Reuse it. (See Scope Area 1.)
- D-08: OBS overlay authenticates via **per-overlay signed JWT** in URL: `/overlay/{id}?tts_token=XXX`. Signed with per-overlay `tts_signing_secret`, claims `{sub: overlay_id, scope: "tts:use", iat}`. No exp — rotation-based revocation.
- D-09: "Copy OBS URL" button in dashboard builds URL+token and copies to clipboard.
- D-10: "Regenerate OBS URL" rotates `tts_signing_secret`, invalidates all prior JWTs.

**Backend API Surface (D-11 – D-17):** 6 new endpoints on `overlay-manager`: `POST /tts-config`, `DELETE /tts-config`, `POST /tts-config/rotate-token`, `GET /tts-voices`, `POST /tts-config/test`, `POST /tts?text=...&voice=...` (tts_token JWT auth, streaming proxy). Plus D-17: wire `featuregates.Cache` + `middleware.RequirePremium` into overlay-manager.

**Settings UI (D-18 – D-23):** Single `CollapsibleSection id="tts" title="Text-to-Speech"` with inline sub-sections: Voice → Throttling → Content → Priority → Advanced. `TTSGroup.tsx` analog to `SoundGroup.tsx`. No presets — safe defaults from issue #270. Test-voice button. `TTS_SETTINGS_UPDATE` postMessage. Quota display after Test-Key.

**DisplaySettings Extension (D-24):** 20 `tts_*` fields (see verbatim list in D-24). ElevenLabs `api_key` and `voice_id` are **explicitly NOT in display_settings** — they live in `overlay_tts_configs`.

**Content Formatting (D-25 – D-32):**
- D-25: `"{display_name} says: {message}"` when `tts_read_username=true`.
- D-26: `"{Platform}: {display_name} says: {message}"` when `tts_read_platform=true`.
- D-27: Single user-picked voice; no auto language detection.
- D-28: Voice URI fallback to default if persisted URI not in current browser's list; console.warn.
- D-29: Strip emote tokens; if emotes >50% of tokens AND `tts_skip_emote_only=true`, skip.
- D-30: `https?://` URLs replaced with literal `"link"`; skip if whitespace/punctuation-only and `tts_skip_links=true`.
- D-31: Only priority event types speak (sub, resub, gift, bits, raid, super_chat, etc.) with event-specific prefix.
- D-32: All 5 platforms enabled by default; TikTok `like_aggregate` always excluded.

**Priority & Queue (D-33 – D-37):**
- D-33: Priority event + queue full = drop oldest non-priority, enqueue priority. Current utterance never canceled mid-word.
- D-34: FIFO order; priority events append unless queue full.
- D-35: `Map<username, lastSpokenAt>` for cooldown, cleared on reload.
- D-36: Token-bucket: bucket size = `tts_messages_per_minute`, refill full bucket every 60s, no persistence. Priority bypasses.
- D-37: Dequeue-time staleness check: `now - message.timestamp > tts_staleness_seconds * 1000`.

**ElevenLabs Failure (D-38 – D-40):**
- D-38: Any failure (401/403/429/5xx/network/timeout) → switch entire session to Web Speech + one toast `"ElevenLabs unavailable — using browser voice."` Persists until reload.
- D-39: Test-Key verbose errors (401→"Invalid API key", 429→"Rate-limited", 5xx→"Service unavailable").
- D-40: Start with blob approach: accumulate stream → `new Audio(URL.createObjectURL(blob))`. AudioContext chunks deferred.

**Playback Integration (D-41, D-42):**
- D-41: Hook fires **after** `shouldFilterMessage` and **in parallel with** `soundPlayerRef.current?.play()`. Call: `ttsPlayerRef.current?.speak(message)`.
- D-42: TTS and notification sound independent; both fire for non-filtered messages.

### Claude's Discretion

- Exact AES-GCM API shape (**researcher recommendation:** `shared/encryption.AESEncryptor` already exists — skip new package)
- Whether featuregates lives in `shared/featuregates` or stays duplicated (**researcher recommendation:** move to `shared/featuregates` + `shared/middleware` — overlay-manager is the second consumer, ADR-0008 explicitly allows it: *"cache lives in share-service/featuregates/ not shared/ — move when second service needs it"*)
- Streaming audio decode approach (D-40 already prefers blob)
- Exact UI copy for buttons, quota format, test-sample text language
- Server-side rate limit for `/tts` proxy (default: 60/min/overlay)
- Whether to write a new ADR for AES-GCM (**researcher recommendation:** `shared/encryption` predates ADR culture. Write `0012-per-overlay-tts-jwt-auth.md` instead — the JWT scheme is the novel contribution)

### Deferred Ideas (OUT OF SCOPE)

Preserved from issue #270 and emerged during D-11 CONTEXT discussion:
- Mod-flagged message TTS (chat `/highlight` command)
- Server-side TTS audio caching for frequent phrases
- Per-viewer TTS opt-out (viewer-owned, not streamer-owned)
- Custom voice cloning via ElevenLabs
- Preset templates (Quiet/Chatty/Priority-only)
- Per-language voice routing for multilingual chat
- Auto-detect message language via `franc`
- Per-error fallback policy (different strategy per HTTP error class)
- Short-lived auto-rotating tokens with websocket push
- Hardcoded voice list fallback when user has no saved key
- TTS enabled/disabled per-source (currently per-platform only)
- Character-quota polling / notifications when low

## Project Constraints (from CLAUDE.md)

**Global (`~/.claude/CLAUDE.md`):**
- GitHub: use `gh` CLI
- Grafana: use Grafana MCP (not applicable to this phase — no dashboard changes)
- Kubernetes: verify context first (not applicable — local dev phase)
- Never use the type `any` — use proper typing [CITED: global CLAUDE.md]

**Project (`./CLAUDE.md`):**
- **Go 1.25+** backend, React 19 + Next.js 16 App Router + Tailwind v4 + `@base-ui/react` frontend [CITED: frontend/package.json, services/overlay-manager/go.mod line 3]
- **PostgreSQL 16 (pgx/v5)**, **Redis 7 (go-redis/v9)**, **Zap** logging, **Gin** HTTP
- Services follow Standard Go Layout: `cmd/main.go`, `handlers/`, `models/`, `repository/`
- Graceful shutdown 25s timeout
- Health checks: `/health/live` always 200, `/health/ready` checks DB + Redis
- CNPG PostgreSQL in Kubernetes namespace `allchat`
- Sealed-secrets for Kubernetes secret delivery

**License header enforcement:** Every new `.go`, `.ts`, `.tsx`, `.sql` file MUST have the AGPL-3.0 header [VERIFIED: commit `b499543a`]. The researcher inspected `shared/encryption/encryption.go`, `services/share-service/middleware/premium.go`, `services/share-service/featuregates/cache.go`, `migrations/048_impersonation_audit_log.sql`, and `frontend/src/lib/utils/soundPlayer.ts` — all carry the header. Omit the header at your peril: plan-checker will reject.

## Phase Requirements

None — this project does not maintain a `REQUIREMENTS.md`. Requirements are captured as D-01 through D-42 in CONTEXT.md. The table below re-summarizes for the planner:

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01..D-04 | Both tiers ship, 3 plans, feature-gate `tts` is_premium=true | Scope Area 3 (featuregates wiring), Plan structure matches Phase 12 precedent |
| D-05..D-10 | AES-GCM key storage + per-overlay JWT | Scope Area 1 (AES-GCM), Scope Area 2 (JWT) |
| D-11..D-17 | 6 new endpoints + featuregates wiring | Scope Area 3 (wiring), Scope Area 4 (ElevenLabs API), Scope Area 5 (streaming proxy) |
| D-18..D-23 | Single `CollapsibleSection`, TTSGroup, postMessage | Mirror of Phase 12 SoundGroup |
| D-24 | DisplaySettings extension (20 fields) | JSONB blob `map[string]any` auto-merges (Phase 11/12 pattern) |
| D-25..D-32 | Content formatting rules | Scope Area 7 (ttsPlayer.ts content pipeline) |
| D-33..D-37 | Queue/cooldown/rate-limiter/staleness | Scope Area 7 (queue design) |
| D-38..D-40 | ElevenLabs failure + streaming audio | Scope Area 4 (failure paths), Scope Area 5 (browser blob) |
| D-41..D-42 | Integration at `page.tsx:414` | Scope Area 8 (verified line numbers) |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Web Speech synthesis | Browser / Client | — | Browser-native API (`window.speechSynthesis`); zero backend round-trip |
| ElevenLabs API key storage | Database / Storage | API / Backend (encryption at handler layer) | Never reaches browser per D-05; decrypted at request time, ciphertext at rest |
| ElevenLabs TTS proxy | API / Backend (overlay-manager) | — | User's key must stay server-side; backend forwards streaming MP3 back to browser |
| Voice list retrieval | API / Backend | — | Upstream `GET /v1/voices` requires `xi-api-key` which lives server-side |
| Test-key / quota validation | API / Backend | — | Same: requires user's stored key |
| Premium gating (feature-gate check) | API / Backend (overlay-manager) | Database (feature_gates table) | Backend-enforced for key upload endpoint (D-11); frontend-enforced for UI (ADR-0008 pattern) |
| OBS-URL JWT signing | API / Backend | Database (`tts_signing_secret`) | Per-overlay secret kept server-side; JWT visible in URL is fine (it's a bearer token) |
| JWT verification for `POST /tts` | API / Backend | — | Standard Gin middleware, reads `tts_token` query param, verifies against `tts_signing_secret` |
| Message queue / sampling / cooldown | Browser / Client | — | State is per-session, per-overlay-tab; no persistence (D-35, D-36 explicit) |
| Priority event detection | Browser / Client | — | `ChatMessage.event.type` arrives pre-normalized from message-processor; client classifies |
| Content formatting (emote/URL strip) | Browser / Client | — | Runs on `ChatMessage` post-filter; pure function (D-29, D-30) |
| TTS settings UI | Browser / Client | API (load/save via existing `updateConfig`) | Reuses Phase 11/12 pattern exactly |
| `TTS_SETTINGS_UPDATE` postMessage | Browser / Client | — | Editor `window.parent.postMessage` → embed preview iframe listener |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `shared/encryption.AESEncryptor` | in-tree | AES-GCM encrypt/decrypt ElevenLabs key | Already production-deployed for OAuth tokens in auth-service. AES-256 (32-byte key), base64 + random 12-byte nonce prefix, standard `crypto/aes` + `crypto/cipher.NewGCM` [VERIFIED: shared/encryption/encryption.go:19-103] |
| `github.com/golang-jwt/jwt/v5` | already used | Per-overlay JWT signing + verification | Existing project standard (`shared/auth/jwt.go`, all services) — HMAC-SHA256 [VERIFIED: shared/auth/jwt.go:24] |
| `github.com/gin-gonic/gin` v1.12.0 | already used | HTTP routing + streaming response (`c.Stream` or `io.Copy(c.Writer, body)`) | Existing standard; streaming proxy pattern at `services/api-gateway/handlers/proxy.go:130` [VERIFIED: services/overlay-manager/go.mod:7] |
| `github.com/jackc/pgx/v5` v5.9.1 | already used | DB access for `overlay_tts_configs` CRUD | Existing standard [VERIFIED: go.mod:9] |
| `github.com/caesar/all-chat/shared/featuregates` (NEW — moved from share-service) | in-tree | Feature-gate cache | Will be moved to `shared/` in Plan 02 per D-17 + ADR-0008 guidance |
| `github.com/caesar/all-chat/shared/middleware.RequirePremium` (NEW — moved from share-service) | in-tree | Premium enforcement middleware | Same move as above |
| `window.speechSynthesis` | browser-native | Free-tier TTS | Baseline widely available since Sept 2018 [CITED: MDN Baseline on SpeechSynthesis page]. `SpeechSynthesisUtterance` constructor + `synth.speak()` + `synth.cancel()` |
| Native `fetch` + `ReadableStream` + `Blob` | browser-native | ElevenLabs proxy stream consumption | Per D-40, start with `await fetch()` → `blob()` → `new Audio(URL.createObjectURL(blob))` [CITED: WHATWG Fetch] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `vitest` | existing | Unit tests for `ttsPlayer.ts` (with `vi.useFakeTimers`, `vi.stubGlobal`) | Frontend utility tests mirror `soundPlayer.test.ts` [VERIFIED: frontend/package.json:11, frontend/src/lib/utils/__tests__/soundPlayer.test.ts:49] |
| `github.com/stretchr/testify` v1.11.1 | existing | Go handler + middleware tests | Matches `premium_test.go` pattern [VERIFIED: services/overlay-manager/go.mod:12, services/share-service/middleware/premium_test.go:26] |
| `net/http/httptest` | stdlib | HTTP handler tests | Standard Go pattern [CITED: Go stdlib] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `shared/encryption.AESEncryptor` | `shared/crypto.AESGCMCipher` (also exists) | Both packages exist. `shared/encryption` is wired into auth-service (production); `shared/crypto` has `StringCipher` interface for test mocking. Planner should check actual import patterns — **recommend `shared/encryption`** for consistency with OAuth token storage [VERIFIED: services/auth-service/cmd/main.go:134-139] |
| JWT in URL query param (D-08) | Cookie or Authorization header | OBS browser sources don't handle custom headers easily; cookies are SameSite-restricted. URL token is the standard pattern for this use case [reasoning from D-08 lock] |
| Streaming blob accumulation (D-40) | `AudioContext.decodeAudioData` + chunked playback | Deferred to v2 per D-40. Blob approach has ~1-2s startup latency but zero complexity |
| `io.Copy(c.Writer, body)` | `c.Stream(func(w) bool {...})` | Both work. `io.Copy` is simpler and matches the api-gateway proxy pattern; `c.Stream` detects client disconnect via bool return [VERIFIED: gin.Context.Stream signature] |

**Installation:** No new npm or Go dependencies required. Every package in the stack is already in the module graph.

**Version verification:**
- `shared/encryption` - verified via `shared/encryption/encryption_test.go` (passes with `c2VjcmV0X2tleV9zZWNyZXRfa2V5X3NlY3JldF9rZXk=` — a valid 32-byte base64 key)
- Gin v1.12.0 - verified in overlay-manager go.mod line 7
- `golang-jwt/jwt/v5` - verified in shared/auth/jwt.go line 24
- pgx v5.9.1 - verified in overlay-manager go.mod line 9
- Web Speech API: Baseline widely available since September 2018 [CITED: MDN]

## Architecture Patterns

### System Architecture Diagram

```
                                         Dashboard Editor (frontend/src/app/overlays/[id]/page.tsx)
                                                |
                                                | saveTTSKey(overlay_id, apiKey, voiceId)
                                                v
                     POST /api/v1/overlays/:id/tts-config  [JWTAuth + RequirePremium("tts")]
                                                |
                                                v
                                         overlay-manager
                                         handlers/tts.go
                                         + shared/encryption.Encrypt(apiKey)
                                         + generate tts_signing_secret (32 random bytes)
                                                |
                                                v
                                    PostgreSQL: overlay_tts_configs
                                    (encrypted_api_key BYTEA, voice_id TEXT,
                                     tts_signing_secret BYTEA, UNIQUE overlay_id)

                     ───────────────────────────────────────────────────────

                    User clicks "Copy OBS URL" → builds /overlay/{id}?tts_token={JWT}
                    JWT = jwt.SignedString(tts_signing_secret, {sub: overlay_id, scope: "tts:use", iat})

                     ───────────────────────────────────────────────────────

                                    OBS Browser Source opens
                                    https://allch.at/overlay/{id}?tts_token=XXX
                                                |
                                                v
                                    Overlay page loads config via
                                    GET /api/v1/overlays/public/:id/config  (unauth, stays unauth — NO secret data)
                                                |
                                                | WebSocket chat stream from api-gateway
                                                v
                                    Incoming ChatMessage →
                                    shouldFilterMessage(msg)? → if filtered: stop
                                                |
                                                | if NOT filtered (both fire in parallel per D-42):
                                                v
                                    soundPlayerRef.current?.play()        ← Phase 12 (unchanged)
                                    ttsPlayerRef.current?.speak(message)  ← Phase 13 (NEW)
                                                |
                                                v
                                    ttsPlayer.ts internal pipeline:
                                    1. Platform-enabled check (D-32)
                                    2. Priority detection (is priority event per D-31?)
                                    3. Sampling: if not priority, Math.random() < sample_rate
                                    4. Per-user cooldown check (D-35)
                                    5. Token-bucket: consume 1 (priority bypasses) (D-36)
                                    6. Queue enqueue: if full AND priority: drop-oldest-non-priority (D-33)
                                    7. Content format: strip emotes, URL→"link", add prefix (D-25..D-30)
                                    8. speak() dispatcher:
                                        ├── if tts_provider === 'browser':
                                        │       SpeechSynthesisUtterance + synth.speak()
                                        └── if tts_provider === 'elevenlabs':
                                                fetch('/api/v1/overlays/:id/tts?text=...&voice=...&tts_token=XXX')
                                                        ↓
                                             overlay-manager POST /tts handler [JWT-via-tts_signing_secret Auth]
                                                        ↓
                                             decrypt api_key from overlay_tts_configs
                                                        ↓
                                             POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}/stream
                                                  with xi-api-key, body {text, model_id: "eleven_multilingual_v2"}
                                                        ↓
                                             io.Copy(c.Writer, resp.Body) — chunked audio/mpeg streamed back
                                                        ↓
                                             ON 401/403/429/5xx/timeout (D-38):
                                                  fallback session-wide to Web Speech
                                                  show 1 toast
                                                        ↓
                                             browser: accumulate stream into Blob →
                                                      new Audio(URL.createObjectURL(blob)) → .play()

                     ───────────────────────────────────────────────────────

                                    Editor → Embed Preview iframe
                                    window.parent.postMessage({type: 'TTS_SETTINGS_UPDATE', ttsSettings: {...}})
                                                |
                                                v
                                    Embed page listener: updateSettings(newSettings) on the ttsPlayer instance
```

### Recommended Project Structure

```
services/overlay-manager/
├── cmd/main.go                       # Wire featuregates.Cache, RequirePremium, crypto, JWT, TTS handlers (NEW blocks)
├── handlers/
│   ├── config.go                     # EXISTING — no change
│   └── tts.go                        # NEW — all 6 TTS endpoints
├── repository/
│   ├── config_repo.go                # EXISTING — no change
│   └── tts_config_repo.go            # NEW — CRUD on overlay_tts_configs (returns ciphertext; encryption at handler)
├── tts/                              # NEW package — optional extraction for the HTTP proxy pipeline
│   ├── elevenlabs_client.go          # HTTP client wrapping ElevenLabs endpoints
│   └── jwt.go                        # Signer + Verifier for tts_token using per-overlay secret
└── models/
    └── tts_config.go                 # NEW — TTSConfig struct matching the DB row

shared/
├── featuregates/                     # NEW — moved from share-service/featuregates/
│   └── cache.go                      # identical to existing share-service file
├── middleware/                       # EXISTING — add premium.go
│   └── premium.go                    # NEW — moved from share-service/middleware/premium.go
└── encryption/                       # EXISTING — no change; reuse
    └── encryption.go                 # already has AESEncryptor + ParseKey

migrations/
└── 049_overlay_tts_configs.sql       # NEW — table + feature_gates INSERT

frontend/src/
├── lib/
│   ├── utils/
│   │   ├── ttsPlayer.ts              # NEW — pure utility with queue/sampling/cooldown/rate-limiter
│   │   └── __tests__/
│   │       └── ttsPlayer.test.ts     # NEW — vitest, mirrors soundPlayer.test.ts
│   ├── api/
│   │   └── overlays.ts               # EXTEND — add saveTTSKey, rotateTTSToken, getTTSVoices, testTTSKey, getTTSConfig
│   └── types/
│       └── overlay.ts                # EXTEND DisplaySettings per D-24 (20 new tts_* fields)
├── components/
│   └── appearance/
│       ├── TTSGroup.tsx              # NEW — analog to SoundGroup.tsx
│       ├── AppearancePanel.tsx       # EXTEND — add CollapsibleSection id="tts"
│       └── __tests__/
│           └── TTSGroup.test.tsx     # NEW — mirrors SoundGroup.test.tsx
└── app/
    ├── overlay/[id]/page.tsx         # EXTEND — ttsPlayerRef + load + integration at line 414
    └── overlays/[id]/
        ├── page.tsx                  # EXTEND — settings load/save, Copy OBS URL button, Regenerate button
        └── preview/embed/page.tsx    # EXTEND — TTS_SETTINGS_UPDATE listener at ~line 272, ref + load
```

### Pattern 1: Pure Client-Side Utility + Group Component

**What:** Phase 11 (filters) + Phase 12 (sounds) established a three-file pattern for any settings-driven client-side feature.

**When to use:** Always, for any user-configurable overlay behavior that runs in the browser.

**Example:**
```typescript
// 1. Pure utility (frontend/src/lib/utils/ttsPlayer.ts)
// Source: mirrors frontend/src/lib/utils/soundPlayer.ts
export interface TTSSettings { /* ... */ }
export interface TTSPlayer {
  speak(message: ChatMessage): void
  updateSettings(s: TTSSettings): void
  destroy(): void
}
export function createTTSPlayer(initialSettings: TTSSettings): TTSPlayer { /* ... */ }

// 2. Group component (frontend/src/components/appearance/TTSGroup.tsx)
// Source: mirrors frontend/src/components/appearance/SoundGroup.tsx
export function TTSGroup({ displaySettings, onChange, isPremium, onPreview }): React.ReactElement { /* ... */ }

// 3. AppearancePanel mount (frontend/src/components/appearance/AppearancePanel.tsx)
<CollapsibleSection id="tts" title="Text-to-Speech">
  <TTSGroup displaySettings={displaySettings} onChange={onTTSChange} isPremium={isPremium} onPreview={handleTTSPreview} />
</CollapsibleSection>
```

### Pattern 2: Backend Featuregates + Premium Middleware Wiring

**What:** Phase 07 established `FeatureGateCache` + `RequirePremium` for share-service. Overlay-manager becomes the second consumer.

**When to use:** Any service that needs capability-level premium gating.

**Example (from `services/share-service/cmd/main.go:103-107` and `:186-190`):**
```go
// Init cache
gateCache := featuregates.NewFeatureGateCache(dbPool, redisClient, log)
if err := gateCache.Start(context.Background()); err != nil {
    log.Fatal("Failed to start feature gate cache", zap.Error(err))
}

// Apply middleware to specific routes
premiumRoutes := api.Group("")
premiumRoutes.Use(middleware.RequirePremium(dbPool, gateCache, "tts", log))
{
    premiumRoutes.POST("/:id/tts-config", ttsHandler.HandleSaveTTSConfig)
    premiumRoutes.DELETE("/:id/tts-config", ttsHandler.HandleDeleteTTSConfig)
    premiumRoutes.POST("/:id/tts-config/rotate-token", ttsHandler.HandleRotateToken)
    premiumRoutes.GET("/:id/tts-voices", ttsHandler.HandleGetVoices)
    premiumRoutes.POST("/:id/tts-config/test", ttsHandler.HandleTestKey)
}

// The /tts streaming proxy is NOT under RequirePremium — it uses tts_token JWT auth (D-16).
// If the overlay ever got downgraded to non-premium, existing tts_tokens continue to work
// until the overlay owner rotates them. This is intentional (graceful premium loss).
```

### Pattern 3: Live Preview via postMessage

**What:** Editor iframe communication pattern used by visual/filter/sound settings.

**When to use:** Any setting the user should be able to hear/see change in real-time before save.

**Example (from `frontend/src/app/overlays/[id]/preview/embed/page.tsx:272-288`):**
```typescript
// In the editor page, on settings change:
previewIframeRef.current?.contentWindow?.postMessage(
  { type: 'TTS_SETTINGS_UPDATE', ttsSettings: newTTSSettings },
  '*'
)

// In the embed page listener:
if (event.data?.type === 'TTS_SETTINGS_UPDATE') {
  const s = event.data.ttsSettings as Partial<DisplaySettings>
  const newSettings: TTSSettings = { /* merge */ }
  ttsSettingsRef.current = newSettings
  ttsPlayerRef.current?.updateSettings(newSettings)
}
```

### Anti-Patterns to Avoid

- **Storing ElevenLabs API key in `display_settings` JSONB** — exposes it on `GET /public/:id/config` which is unauthenticated (D-05, verified at `services/overlay-manager/handlers/config.go:182`). Always use `overlay_tts_configs` with encrypted BYTEA.
- **Hand-rolling an HTTP retry or circuit breaker for ElevenLabs** — D-38 locks session-wide fallback. Don't build a reconnect ladder.
- **Calling `speechSynthesis.getVoices()` once at import time** — Chromium returns `[]` until `onvoiceschanged`. Always register `onvoiceschanged` and refresh lazily.
- **Re-registering Prometheus metrics when moving `featuregates` to `shared/`** — Phase 07 flagged this explicitly in STATE.md: "RingBufferPublisher uses prometheus.Registerer injection (not promauto) to allow per-test isolated registries." `featuregates.cache.go` already has no metrics — confirm still true after the move [VERIFIED: grep'd cache.go for `prometheus`/`promauto` — zero matches].
- **Adding a new Go module** — `shared/` is a single module; new packages go under it.
- **Blocking on the full audio stream before playing** — D-40 starts with blob accumulation. Accept the ~1-2s latency for v1; AudioContext chunking is v2.
- **Using `any` type in TypeScript** — global CLAUDE.md forbids; use `unknown` or proper type definitions.
- **Debouncing `TTS_SETTINGS_UPDATE` postMessage** — D-22 says no debouncing; fires on every change.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| AES-GCM encryption | Raw `crypto/aes` + nonce management | `shared/encryption.AESEncryptor` (already in tree, production-deployed, tested) | Nonce collision risk, padding bugs, IV reuse = catastrophic. Existing package has been audited for OAuth tokens. |
| JWT sign/verify | Hand-rolled HMAC | `github.com/golang-jwt/jwt/v5` + claims struct + `jwt.ParseWithClaims` | Header-parsing bugs, algorithm confusion attacks, `none` alg bypass. See `shared/auth/jwt.go` for existing patterns. |
| Premium gate check | `if user.is_premium { ... }` in handler | `middleware.RequirePremium(db, gateCache, featureKey, logger)` | Loses feature-gate soft-flip capability (ADR-0008). Also duplicates DB queries. |
| HTTP streaming proxy | Manual `http.ResponseWriter.Flush()` loop | `io.Copy(c.Writer, backendResp.Body)` or `c.Stream(func)` | Gin/stdlib handle chunked transfer automatically. See `services/api-gateway/handlers/proxy.go:130`. |
| Voice list caching | Background poller with Redis TTL | Lazy on-demand (D-14) — call `GET /v1/voices` when dropdown opens | Voice list rarely changes; per-request cost is ~200ms; over-engineering for the use case. |
| Token bucket rate limiter | Leaky bucket with goroutine | `Map<string, number>` + full-refill every 60s (D-36 explicit) | D-36 locks the algorithm; don't import `golang.org/x/time/rate` client-side. |
| Text-to-speech | Custom phoneme engine | `window.speechSynthesis` (free) OR ElevenLabs API (premium) | It's a solved problem at two price points. |
| Browser audio pool | `new AudioContext()` with channel routing | Single `new Audio()` element for ElevenLabs blob (D-40) | D-40 locks blob approach. `SpeechSynthesisUtterance` uses OS audio directly, no pool needed. |

**Key insight:** Every component of Phase 13 except the ElevenLabs HTTP client has direct prior art in this codebase. The phase is 80% assembly, 20% net-new code. Resist the urge to rebuild anything marked "existing" — reuse aggressively.

## Runtime State Inventory

This is **not** a rename/refactor phase — it's additive. A new table, new fields, new endpoints. No renames. Inventory is not required.

**Stored data:** N/A. New tables (`overlay_tts_configs`) and new JSONB keys are added; nothing renamed.

**Live service config:** Feature-gate row `tts` is added idempotently via migration. Existing feature gates unaffected.

**OS-registered state:** None — the phase adds no systemd units, Task Scheduler entries, pm2 processes, or Docker images.

**Secrets/env vars:** New env var? **No.** The existing `TOKEN_ENCRYPTION_KEY` (16/24/32 bytes base64) already used by auth-service should be reused for `overlay_tts_configs.encrypted_api_key`. D-07 specified `CRYPTO_MASTER_KEY`; researcher recommends using `TOKEN_ENCRYPTION_KEY` instead for consistency. [VERIFIED: `.env.example:34`, `deployments/docker-compose.yml:70`, `deployments/k8s/base/auth-service/deployment.yaml:107`]

**Build artifacts:** None. No compiled binary names change; no npm package renames.

## Common Pitfalls

### Pitfall 1: Web Speech API `getVoices()` returns empty array on first call in Chromium

**What goes wrong:** In Chrome/Chromium-based browsers (including OBS browser source which uses CEF), `speechSynthesis.getVoices()` returns `[]` until the engine finishes loading voices. User selects a voice in the dropdown → the voice dropdown is empty → user frustrated.

**Why it happens:** Chromium loads voices asynchronously. Firefox and Safari Desktop return voices synchronously on first call [CITED: dev.to/jankapunkt cross-browser Web Speech article].

**How to avoid:** Register `speechSynthesis.onvoiceschanged` and re-populate the dropdown. Call `getVoices()` inside both the synchronous initialization AND the handler. Example:
```typescript
function populateVoices() {
  const voices = speechSynthesis.getVoices()
  setVoiceOptions(voices)
}
populateVoices()
if (typeof speechSynthesis.onvoiceschanged !== 'undefined') {
  speechSynthesis.onvoiceschanged = populateVoices
}
```

**Warning signs:** Voice dropdown shows 0 options on first page load; refreshing "fixes" it. Safari iOS may require a user gesture before voices populate [CITED: Apple Developer Forums thread 723503].

### Pitfall 2: Voice URI is not portable across browsers (D-28 fallback)

**What goes wrong:** User saves `voiceURI: "Google UK English Male"` in Chrome on one machine. They install OBS on a new machine — OBS CEF doesn't have Google voices (only system voices). TTS is silent or speaks with a jarring default voice.

**Why it happens:** Voice URIs embed the source (local OS TTS engine vs. Chrome's cloud voices). There is no normalized cross-browser voice name registry.

**How to avoid:** On load, check `voices.find(v => v.voiceURI === savedURI)`. If not found: log `console.warn("TTS voice '{uri}' not available, falling back to default")` and omit `utterance.voice` (engine picks default based on `utterance.lang`). This is D-28 as written.

**Warning signs:** User reports "TTS works at home but not in my stream PC." Diagnosis: platform mismatch.

### Pitfall 3: OBS browser source autoplay may silently drop first utterance

**What goes wrong:** In standalone browsers (not OBS CEF), `speechSynthesis.speak()` without a prior user gesture is often blocked. First TTS message on overlay load produces no sound; subsequent ones work after any click.

**Why it happens:** Chrome's autoplay policy for audible media. Web Speech generally has fewer restrictions than `<audio>` but is not exempt.

**How to avoid:** OBS browser source is allowed to autoplay by default (explicit setting in OBS). In the editor preview iframe, wrap the test-voice trigger in a user-initiated event (it already is — it's a button click). Document the standalone-browser caveat in a tooltip. Do NOT attempt a workaround (no "silent priming utterance" tricks).

**Warning signs:** Test-voice button always works; live-overlay first message sometimes doesn't (in editor preview, not OBS).

### Pitfall 4: AES-GCM nonce reuse with the same key is catastrophic

**What goes wrong:** If you accidentally reuse a 12-byte nonce with the same key to encrypt two different plaintexts, an attacker can XOR the ciphertexts to recover the plaintext XOR, and potentially forge authenticated messages. AES-GCM security COLLAPSES on nonce reuse [CITED: NIST SP 800-38D §8.3].

**Why it happens:** If you use a deterministic counter, a bug that resets the counter; if you use `time.Now().UnixNano()`, clock skew on restart; if you feed `Seal` a nil nonce by mistake.

**How to avoid:** Use `crypto/rand.Reader` to generate a fresh 12-byte nonce per encryption. This is exactly what `shared/encryption.AESEncryptor.EncryptString` already does [VERIFIED: shared/encryption/encryption.go:77-79]. Don't touch the nonce logic.

**Warning signs:** Code comment saying "optimization: reuse nonce across requests". Reject the PR.

### Pitfall 5: `ON CONFLICT DO NOTHING` on feature_gates INSERT

**What goes wrong:** Migrations are replayed during dev (migration tools don't always track state). A plain `INSERT INTO feature_gates (feature_key) VALUES ('tts')` on the second run fails with a primary-key violation, poisoning the migration chain.

**Why it happens:** Missing idempotency clause.

**How to avoid:** Always `INSERT ... ON CONFLICT (feature_key) DO NOTHING`. Phase 07's migration 044 does this [VERIFIED: migrations/044_feature_gates.sql:20-22]. Mirror exactly.

**Warning signs:** Fresh dev environment setup fails on second `make migrate-up`.

### Pitfall 6: Promauto duplicate registration when moving featuregates to shared/

**What goes wrong:** If `shared/featuregates/cache.go` registers any Prometheus metric via `promauto.NewCounter(...)`, and both overlay-manager and share-service import it, they hit the default registry twice — `panic: duplicate metrics collector registration attempted`.

**Why it happens:** `promauto` uses a package-level default registry.

**How to avoid:** Verify before moving: `grep -n promauto services/share-service/featuregates/cache.go` returns zero. Confirmed: the file has no metrics [VERIFIED: read of the entire file]. Safe to move as-is. Phase 08 STATE.md captures this lesson: "RingBufferPublisher uses prometheus.Registerer injection (not promauto) to allow per-test isolated registries."

**Warning signs:** `TestMain` in another service panics on boot after the move.

### Pitfall 7: Streaming response client-disconnect handling

**What goes wrong:** User closes the overlay tab mid-TTS-stream. The overlay-manager keeps copying MB of ElevenLabs audio into a dead TCP socket. Eventually `io.Copy` returns an error; logs fill with `broken pipe`. Worse: the ElevenLabs character quota is consumed for audio nobody hears.

**Why it happens:** No early cancellation propagation from client disconnect to upstream HTTP request.

**How to avoid:** Pass `c.Request.Context()` to the upstream `http.NewRequestWithContext(ctx, ...)`. Gin cancels the context on client disconnect. ElevenLabs will abort generation when the connection drops. Pattern:
```go
req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, elevenLabsURL, body)
```

**Warning signs:** `broken pipe` log spam under traffic; ElevenLabs quota drained faster than expected.

### Pitfall 8: CORS on the `/tts` streaming proxy

**What goes wrong:** Overlay iframes into OBS (origin: browser source, often no origin header), into the editor preview (origin: `https://allch.at`), or directly in OBS (origin: `null`). Any strict CORS policy breaks one of these cases.

**Why it happens:** API-gateway currently handles CORS centrally (see overlay-manager main.go line 205 comment). But streaming audio crosses via the same path.

**How to avoid:** Let API-gateway's existing CORS middleware handle it. Overlay-manager does NOT add its own CORS headers (precedent from existing handlers). Verify the `/tts` endpoint goes through api-gateway's proxy (it should — `/api/v1/overlays/:id/*` already routes via proxyHandler [VERIFIED: services/api-gateway/cmd/main.go:457-476]).

**Warning signs:** Browser console shows `CORS preflight failed` on `POST /api/v1/overlays/:id/tts`. Fix is in API-gateway, not overlay-manager.

### Pitfall 9: Token-bucket refill with `document.hidden`

**What goes wrong:** User opens the overlay in a background tab. Browsers throttle timers in backgrounded tabs. If the token-bucket refill uses `setTimeout(fill, 60000)`, the refill may not fire for minutes. When the tab becomes visible, all queued messages fire at once.

**Why it happens:** Timer throttling in hidden tabs.

**How to avoid:** Per D-36 the bucket refills "full bucket every 60 seconds". Use a lazy check at consume time: `if (now - lastRefillAt >= 60000) { tokens = MAX; lastRefillAt = now; }`. No timers. OBS browser source considers itself "visible" always, so this is mostly a concern for the editor preview.

**Warning signs:** Burst of TTS utterances when user switches back to the tab.

### Pitfall 10: Concurrent Test-voice clicks overlapping

**What goes wrong:** User clicks "Test voice", clicks again 200ms later (or while first utterance is still speaking). Two `SpeechSynthesisUtterance`s play simultaneously — garbled audio.

**Why it happens:** `speak()` queues internally; it doesn't cancel prior utterances.

**How to avoid:** Before calling `synth.speak(u)`, call `synth.cancel()`. This clears the queue AND stops any in-flight utterance. Cheap, idempotent when empty.

**Warning signs:** User complaint: "Test button makes it speak twice." Fix: `synth.cancel()` first.

### Pitfall 11: ElevenLabs quota exhaustion mid-stream

**What goes wrong:** User's monthly character quota hits 0 during a live stream. Next `/tts` call returns 429 or character-limit error. Without a fallback, TTS goes silent.

**Why it happens:** Quota is per-account, tracked by ElevenLabs.

**How to avoid:** D-38 is definitive — any error → session-wide fallback to Web Speech + one toast. The implementation: catch ANY non-2xx in the fetch, set a session flag `elevenLabsFailed = true`, redirect all subsequent `.speak()` calls to the Web Speech branch, show toast once (gate with another flag). Don't try to detect quota specifically; any error path is fallback.

**Warning signs:** User reports "TTS went quiet halfway through my stream." Diagnosis: check ElevenLabs dashboard for quota.

### Pitfall 12: JWT with no expiration — rotation is the revocation mechanism

**What goes wrong:** User leaks their OBS URL (e.g., screenshares the URL bar on camera). The JWT has no `exp` claim (D-08). An attacker can use the URL indefinitely.

**Why it happens:** Intentional design — OBS URLs should "just work" long-term. Revocation is manual via the "Regenerate OBS URL" button (D-10).

**How to avoid:** Clearly document this in the button's tooltip/help text: "If your OBS URL is ever exposed publicly, click Regenerate to invalidate the old URL." Make the button prominent. Rotate the 32-byte `tts_signing_secret` with fresh `crypto/rand.Read`. All prior JWTs signed with the old secret fail verification.

**Warning signs:** Support ticket: "Someone is spamming my TTS." Fix: regenerate URL.

## Code Examples

### Example 1: AES-GCM Encrypt/Decrypt (ALREADY IN TREE — REUSE)

```go
// Source: shared/encryption/encryption.go (lines 34-103, abridged)
package encryption

func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
    if len(key) != 16 && len(key) != 24 && len(key) != 32 {
        return nil, ErrInvalidKeyBytes
    }
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    return &AESEncryptor{gcm: gcm, nonceSize: gcm.NonceSize()}, nil
}

func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
    nonce := make([]byte, e.nonceSize)
    io.ReadFull(rand.Reader, nonce)
    ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Usage in overlay-manager handlers/tts.go:
parsedKey, _ := encryption.ParseKey(os.Getenv("TOKEN_ENCRYPTION_KEY"))
cipher, _ := encryption.NewAESEncryptor(parsedKey)
encrypted, _ := cipher.EncryptString(userElevenLabsKey)  // store in BYTEA as []byte(encrypted)
plaintext, _ := cipher.DecryptString(string(ciphertextFromDB))
```

### Example 2: Per-overlay JWT sign + verify

```go
// NEW file: services/overlay-manager/tts/jwt.go
package tts

import (
    "errors"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type TTSClaims struct {
    Scope string `json:"scope"`
    jwt.RegisteredClaims
}

func SignOverlayToken(overlayID string, signingSecret []byte) (string, error) {
    claims := TTSClaims{
        Scope: "tts:use",
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:  overlayID,
            IssuedAt: jwt.NewNumericDate(time.Now()),
            Issuer:   "all-chat",
            // No ExpiresAt — revocation via signing-secret rotation
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(signingSecret)
}

func VerifyOverlayToken(tokenString, overlayID string, signingSecret []byte) error {
    parsed, err := jwt.ParseWithClaims(tokenString, &TTSClaims{}, func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return signingSecret, nil
    })
    if err != nil || !parsed.Valid {
        return errors.New("invalid tts_token")
    }
    claims := parsed.Claims.(*TTSClaims)
    if claims.Subject != overlayID || claims.Scope != "tts:use" {
        return errors.New("tts_token scope/subject mismatch")
    }
    return nil
}
```

### Example 3: Gin streaming proxy to ElevenLabs

```go
// handlers/tts.go — HandleTTS (D-16)
func (h *TTSHandler) HandleTTS(c *gin.Context) {
    overlayID := c.Param("id")
    ttsToken := c.Query("tts_token")

    // 1. Load per-overlay signing secret + encrypted api key
    cfg, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
    if err != nil {
        c.JSON(404, gin.H{"error": "tts config not found"})
        return
    }

    // 2. Verify the JWT
    if err := tts.VerifyOverlayToken(ttsToken, overlayID, cfg.SigningSecret); err != nil {
        c.JSON(401, gin.H{"error": "invalid tts_token"})
        return
    }

    // 3. Decrypt the api key
    apiKey, err := h.cipher.DecryptString(string(cfg.EncryptedAPIKey))
    if err != nil {
        c.JSON(500, gin.H{"error": "decrypt failed"})
        return
    }

    text := c.Query("text")
    voiceID := c.Query("voice")

    body, _ := json.Marshal(map[string]any{
        "text":     text,
        "model_id": "eleven_multilingual_v2",  // default per ElevenLabs spec
    })

    // 4. Build upstream request with REQUEST CONTEXT for client-disconnect propagation
    req, _ := http.NewRequestWithContext(
        c.Request.Context(),
        http.MethodPost,
        fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s/stream", voiceID),
        bytes.NewReader(body),
    )
    req.Header.Set("xi-api-key", apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "audio/mpeg")

    resp, err := h.httpClient.Do(req)
    if err != nil {
        c.JSON(502, gin.H{"error": "elevenlabs upstream error"})
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        // Propagate status to let the browser detect fallback trigger (D-38)
        c.Status(resp.StatusCode)
        io.Copy(c.Writer, resp.Body)
        return
    }

    // 5. Stream the audio chunks back to the overlay
    c.Writer.Header().Set("Content-Type", "audio/mpeg")
    c.Status(200)
    if _, err := io.Copy(c.Writer, resp.Body); err != nil {
        // Client disconnect or network error — logged, not returned
        c.Error(err)
    }
}
```

### Example 4: `ttsPlayer.ts` shape (mirrors `soundPlayer.ts`)

```typescript
// Source: mirrors frontend/src/lib/utils/soundPlayer.ts structure
export interface TTSSettings {
  enabled: boolean
  provider: 'browser' | 'elevenlabs'
  volume: number
  voice_uri?: string
  rate: number
  pitch: number
  filter_mode: 'all' | 'sample' | 'priority_only'
  sample_rate: number
  max_queue: number
  messages_per_minute: number
  user_cooldown_seconds: number
  staleness_seconds: number
  priority_events: boolean
  priority_bits_min: number
  read_username: boolean
  read_platform: boolean
  max_message_chars: number
  skip_emote_only: boolean
  skip_links: boolean
  enabled_platforms: string[]
  // Runtime flag (not persisted): session-wide fallback after ElevenLabs failure
  elevenLabsFailed?: boolean
  // Endpoint + token for ElevenLabs proxy (present only when provider === 'elevenlabs')
  ttsEndpoint?: string   // e.g. `/api/v1/overlays/${id}/tts`
  ttsToken?: string
  voiceId?: string
}

export interface TTSPlayer {
  speak(message: ChatMessage): void
  updateSettings(s: TTSSettings): void
  destroy(): void
}

const PRIORITY_EVENTS = new Set<EventType>([
  'subscription', 'resubscription', 'gift_subscription', 'mystery_gift',
  'bits', 'raid', 'super_chat', 'super_sticker',
  'kick_subscription', 'kick_gift_subscription', 'kick_donation',
])

export function createTTSPlayer(initialSettings: TTSSettings, onFallback?: () => void): TTSPlayer {
  let settings: TTSSettings = { ...initialSettings }
  const queue: ChatMessage[] = []
  const cooldowns: Map<string, number> = new Map()   // username -> lastSpokenAt
  let bucketTokens = settings.messages_per_minute
  let bucketLastRefill = Date.now()
  let sessionFallback = false  // D-38
  let speaking = false
  const synth = typeof window !== 'undefined' ? window.speechSynthesis : null

  function isPriority(msg: ChatMessage): boolean {
    if (!settings.priority_events) return false
    const t = msg.event?.type
    if (!t) return false
    return PRIORITY_EVENTS.has(t)
  }

  function refillBucket() {
    const now = Date.now()
    if (now - bucketLastRefill >= 60_000) {
      bucketTokens = settings.messages_per_minute
      bucketLastRefill = now
    }
  }

  function formatContent(msg: ChatMessage): string | null {
    // D-29 strip emotes, D-30 URL→"link", D-25/26 prefix, D-31 event prefix
    // Returns null to skip entirely
    // (Full impl: ~40 lines — see Plan 01 Task details)
    return /* formatted string or null */ ''
  }

  function speak(message: ChatMessage): void {
    if (!settings.enabled) return
    if (!settings.enabled_platforms.includes(message.platform)) return
    if (message.event?.type === 'like_aggregate') return  // D-32 always exclude

    const priority = isPriority(message)

    // D-37 sampling / D-31 priority gating
    if (!priority) {
      if (settings.filter_mode === 'priority_only') return
      if (settings.filter_mode === 'sample' && Math.random() >= settings.sample_rate) return
    }

    // D-35 per-user cooldown (priority bypasses cooldown)
    const uname = message.user.username.toLowerCase()
    if (!priority) {
      const last = cooldowns.get(uname) ?? 0
      if (Date.now() - last < settings.user_cooldown_seconds * 1000) return
    }

    // D-36 token bucket (priority bypasses)
    refillBucket()
    if (!priority) {
      if (bucketTokens <= 0) return
      bucketTokens--
    }

    cooldowns.set(uname, Date.now())

    // D-33 queue management
    if (queue.length >= settings.max_queue) {
      if (priority) {
        // drop oldest non-priority
        const idx = queue.findIndex(m => !isPriority(m))
        if (idx >= 0) queue.splice(idx, 1)
        else return  // all priority, no room
      } else return   // non-priority full: drop
    }
    queue.push(message)
    pump()
  }

  async function pump(): Promise<void> {
    if (speaking) return
    const msg = queue.shift()
    if (!msg) return

    // D-37 staleness (dequeue-time)
    const ts = typeof msg.timestamp === 'string' ? Date.parse(msg.timestamp) : Number(msg.timestamp)
    if (Date.now() - ts > settings.staleness_seconds * 1000) {
      pump()
      return
    }

    const text = formatContent(msg)
    if (!text) { pump(); return }

    speaking = true
    try {
      if (sessionFallback || settings.provider === 'browser') {
        await speakBrowser(text)
      } else {
        await speakElevenLabs(text)
      }
    } catch {
      // D-38: ElevenLabs error → switch to Web Speech for rest of session
      if (!sessionFallback) {
        sessionFallback = true
        onFallback?.()
      }
      try { await speakBrowser(text) } catch { /* swallow */ }
    } finally {
      speaking = false
      pump()  // drain next
    }
  }

  async function speakBrowser(text: string): Promise<void> {
    return new Promise((resolve) => {
      if (!synth) return resolve()
      synth.cancel()  // Pitfall 10: ensure no overlap
      const u = new SpeechSynthesisUtterance(text)
      u.volume = settings.volume
      u.rate = settings.rate
      u.pitch = settings.pitch
      const voices = synth.getVoices()
      const voice = voices.find(v => v.voiceURI === settings.voice_uri)
      if (voice) u.voice = voice
      else if (settings.voice_uri) console.warn(`[TTS] Voice '${settings.voice_uri}' not available, using default`)
      u.onend = () => resolve()
      u.onerror = () => resolve()
      synth.speak(u)
    })
  }

  async function speakElevenLabs(text: string): Promise<void> {
    const url = `${settings.ttsEndpoint}?tts_token=${settings.ttsToken}&voice=${settings.voiceId}&text=${encodeURIComponent(text)}`
    const resp = await fetch(url)
    if (!resp.ok) throw new Error(`elevenlabs ${resp.status}`)
    const blob = await resp.blob()
    const audio = new Audio(URL.createObjectURL(blob))
    audio.volume = settings.volume
    return new Promise((resolve) => {
      audio.onended = () => { URL.revokeObjectURL(audio.src); resolve() }
      audio.onerror = () => { URL.revokeObjectURL(audio.src); resolve() }
      audio.play().catch(() => resolve())
    })
  }

  function updateSettings(s: TTSSettings) {
    settings = { ...s, elevenLabsFailed: sessionFallback || s.elevenLabsFailed }
  }

  function destroy() {
    synth?.cancel()
    queue.length = 0
    cooldowns.clear()
  }

  return { speak, updateSettings, destroy }
}
```

### Example 5: Web Speech voice-loaded pattern

```typescript
// In TTSGroup.tsx — voice dropdown populator
import { useEffect, useState } from 'react'

function useBrowserVoices(): SpeechSynthesisVoice[] {
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([])

  useEffect(() => {
    if (typeof window === 'undefined' || !window.speechSynthesis) return

    const update = () => setVoices(window.speechSynthesis.getVoices())

    update()  // initial — works in Firefox/Safari
    // Chrome fires onvoiceschanged when ready
    window.speechSynthesis.addEventListener('voiceschanged', update)
    return () => window.speechSynthesis.removeEventListener('voiceschanged', update)
  }, [])

  return voices
}

// Safari iOS fallback (voices sometimes never load until user gesture) —
// NOT addressed in v1. Document known limitation in tooltip.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `shared/crypto` (the package D-07 speaks of creating) | `shared/encryption.AESEncryptor` already exists | Preexisting — auth-service token encryption (~2025-02 per `docs/migrations/2025-02-auth-token-encryption.md`) | Don't build a new package. Reuse. |
| CLAUDE.md: "Token encryption is basic (TODO: implement AES-GCM)" | AES-GCM IS implemented | Preexisting as of 2025-02 migration | CLAUDE.md note is stale. Phase 13 does NOT close this tech-debt because it's already closed; the note just needs updating. |
| featuregates package in `share-service/featuregates/` | Move to `shared/featuregates/` | Phase 13 Plan 02 | ADR-0008 explicitly blessed this move once a second service needs it. Overlay-manager is that service. |
| Gin `c.Stream(func(w) bool)` | `io.Copy(c.Writer, body)` | Preexisting — api-gateway proxy pattern | Both valid. `io.Copy` is simpler and more idiomatic for pure proxy. |
| ElevenLabs `eleven_monolingual_v1` | `eleven_multilingual_v2` (current default) | ~2024 | Single model handles English + non-Latin scripts. D-27 notes Web Speech garbles non-Latin; ElevenLabs v2 handles it [CITED: WebFetch of ElevenLabs streaming spec] |

**Deprecated/outdated:**
- ElevenLabs `GET /v1/voices` — replaced by `GET /v2/voices` per the streaming docs page. Both work; v2 returns paginated. For this phase, v1 is simpler (no pagination); response field names (`voice_id`, `name`, `category`, `preview_url`, `labels`) are stable across versions [CITED: WebFetch on GetVoicesV2ResponseModel].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | [ASSUMED] The `overlays` database table is named `overlays` (not `overlay_configs`) for the FK in `overlay_tts_configs.overlay_id REFERENCES overlays(id)`. | Migration | Migration fails on apply. Easily fixed. Verified via `migrations/001_initial_schema.sql:23`: confirmed `overlays` table exists. Upgrading to [VERIFIED]. |
| A2 | [VERIFIED: migration 001:42] `overlay_configs.display_settings` is `JSONB DEFAULT '{}'::jsonb` — no schema change needed to add `tts_*` keys. | DisplaySettings extension | N/A — verified. |
| A3 | [ASSUMED] ElevenLabs's 401/403 responses are distinguishable from 422 validation errors — the planner may need the overlay-manager to map all 4xx to a single "ElevenLabs error" status without trying to differentiate. | Streaming proxy | D-38 eliminates the need; any error → session fallback. Safe assumption. |
| A4 | [VERIFIED: shared/encryption code read] `shared/encryption.AESEncryptor` supports AES-256 (32-byte key) and uses random 12-byte nonce per call. | Crypto reuse | N/A — verified. |
| A5 | [VERIFIED: MDN + dev.to cross-browser article] `onvoiceschanged` is supported in Chrome but not reliably in Firefox Desktop or Safari — synchronous `getVoices()` works on the latter two. | Web Speech integration | The useEffect must call `getVoices()` on mount AND register the event listener. Pattern in Example 5 handles both. |
| A6 | [VERIFIED: migration 044] Migration number 049 is the next available (048 is the latest existing). | Migration | N/A — verified via Glob + Read. |
| A7 | [ASSUMED] `TOKEN_ENCRYPTION_KEY` env var is available to overlay-manager in the k8s deployment. Currently it's wired into auth-service only. | Deployment | Planner must add `TOKEN_ENCRYPTION_KEY` to `deployments/k8s/base/overlay-manager/deployment.yaml` (as in `auth-service/deployment.yaml:107`). Verify in Plan 02. |
| A8 | [ASSUMED] The DSGVO / data-residency policy permits user ElevenLabs API keys to be stored encrypted in the EU-hosted PostgreSQL. | Compliance | The codebase already stores OAuth access/refresh tokens encrypted via the same mechanism, so precedent is set. User confirmation NOT required unless planner knows otherwise. |
| A9 | [VERIFIED: ElevenLabs search result] Default model for streaming is `eleven_multilingual_v2`; response is MP3 via chunked transfer encoding. | ElevenLabs API | N/A — verified. |
| A10 | [ASSUMED] Gin v1.12.0 supports `http.NewRequestWithContext(c.Request.Context(), ...)` cancellation propagation for streaming upstream requests. | Proxy handler | Go stdlib behavior, not Gin-specific. Safe assumption [CITED: `net/http` stdlib docs]. |

**User confirmation needed:** A7 (env var rollout for overlay-manager) and A8 (DSGVO acceptability) are the only items that could benefit from a yes/no from the user. The planner may proceed on the current assumptions and surface both in the PR description for reviewer sign-off.

## Open Questions

1. **Should `shared/crypto` and `shared/encryption` be merged, or does the planner accept the duplication and pick one?**
   - What we know: Both exist; `shared/encryption` is actively used by auth-service for OAuth tokens; `shared/crypto` exists but researcher could not find active usage in Go services (only documented in `.planning/phases/13-.../13-CONTEXT.md`). Both implement AES-GCM correctly.
   - What's unclear: Why two packages exist. Possibly Phase N created `shared/crypto` and Phase N+1 created `shared/encryption` independently.
   - Recommendation: Use `shared/encryption` (production-deployed, tested, follows existing env var pattern). Flag `shared/crypto` for removal in a follow-up quick task. Do NOT re-create a third package for this phase.

2. **Is there a backend rate-limit library already in use, or do we build a per-overlay counter in-memory in overlay-manager?**
   - What we know: `shared/ratelimit/` exists [VERIFIED: Glob]. Researcher did not inspect its API in depth.
   - What's unclear: Whether `shared/ratelimit` supports the "60/min per overlay" pattern (D-16) or is tuned for something else.
   - Recommendation: Plan 02 task: inspect `shared/ratelimit/ratelimit.go`. If it fits, use it; if not, a simple `Map<overlayID, {count, resetAt}>` in the TTSHandler is fine (server restart is an acceptable rate-limit reset — same as the client-side bucket).

3. **How does the frontend know it has a saved ElevenLabs key (for UI state)?**
   - D-24 explicitly says `has_elevenlabs_config` boolean returned by a new GET endpoint. The current 6 endpoints (D-11 to D-16) don't include a `GET /tts-config`.
   - Recommendation: Add a 7th endpoint: `GET /api/v1/overlays/:id/tts-config` — returns `{has_elevenlabs_config: bool, voice_id?: string, obs_url?: string}`. Auth-required, no premium gate (reading config is allowed even for downgraded users). Planner: propose this in Plan 02 or Plan 03.

4. **For the Test-Key endpoint (D-15), does the 2-second "Hello from All-Chat" sample speak through the browser immediately or return a URL for playback?**
   - Recommendation: Stream the audio back in the same response (same pattern as `/tts`, but fixed text). Frontend plays the blob. This avoids a second round-trip.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `shared/encryption` package | Plan 02 (key storage) | ✓ | in-tree | — |
| `shared/auth` JWT package | Plan 02 (OBS URL JWT) | ✓ | in-tree | — |
| `shared/featuregates` package | Plan 02 (TTS gate) | ✓ (at `services/share-service/featuregates/` — needs moving to `shared/`) | v0.0.0 (in-tree) | Move during Plan 02 |
| `shared/middleware.RequirePremium` | Plan 02 (TTS config endpoints) | ✓ (at `services/share-service/middleware/premium.go` — needs moving to `shared/middleware/`) | v0.0.0 (in-tree) | Move during Plan 02 |
| PostgreSQL 16 (CNPG) | Plan 02 (migration 049) | ✓ (assumed — production cluster) | 16 | — |
| Redis 7 | Plan 02 (Pub/Sub feature-gate invalidation) | ✓ | 7 | — |
| `TOKEN_ENCRYPTION_KEY` env var | Plan 02 (AES-GCM master key) | Partial — wired to auth-service only | base64 32-byte string | Planner must extend `deployments/k8s/base/overlay-manager/deployment.yaml` |
| ElevenLabs API (api.elevenlabs.io) | Plan 02 runtime | N/A (external; user-supplied API key authenticates) | — | D-38 session-wide Web Speech fallback |
| `window.speechSynthesis` browser API | Plan 01 (Web Speech tier) | ✓ (all modern browsers incl. OBS CEF) | Baseline widely available since Sept 2018 | Graceful: if absent, TTS toggle visibly disabled |
| `golang-jwt/jwt/v5` | Plan 02 (JWT sign/verify) | ✓ (in `shared/auth`) | v5 | — |
| `gin` v1.12.0 | Plan 02 (routing + streaming) | ✓ | 1.12.0 | — |
| `pgx/v5` v5.9.1 | Plan 02 (DB) | ✓ | 5.9.1 | — |
| `vitest` | Plan 01 + 03 (frontend tests) | ✓ | per package.json | — |
| `react-hot-toast` | Plan 03 (failure toast D-38) | ✓ | 2.6.0 [VERIFIED: frontend/package.json] | — |
| `@base-ui/react` toastManager | Plan 03 (existing toast API) | ✓ | 1.3.0 | — |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** `TOKEN_ENCRYPTION_KEY` env var scope — must be extended to overlay-manager k8s deployment during Plan 02.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Frontend framework | `vitest` with `@testing-library/react` |
| Frontend config file | `frontend/vitest.config.ts` (existing) |
| Frontend quick run | `cd frontend && npm test -- --run path/to/file.test.ts` |
| Frontend full suite | `cd frontend && npm test -- --run` |
| Backend framework | `go test` + `stretchr/testify` |
| Backend config file | none — go test built-in |
| Backend quick run | `go test ./services/overlay-manager/handlers/... -run TestHandleTTS -v` |
| Backend full suite | `make test` (root Makefile) |
| Phase gate | Full suite green before `/gsd-verify-work` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-03 | `tts` feature-gate row exists after migration | integration | `go test ./services/overlay-manager/repository/... -run TestTTSMigration` | ❌ Wave 0 |
| D-05 | Public config endpoint does NOT return encrypted API key | integration | `go test ./services/overlay-manager/handlers/config_test.go -run TestPublicConfigHidesTTSKey` | ❌ Wave 0 |
| D-06, D-07 | Saving TTS config encrypts key with AES-GCM roundtrip | unit | `go test ./shared/encryption/ -run TestEncryptDecryptRoundTrip` | ✅ (exists) |
| D-06, D-07 | DB stores ciphertext, not plaintext | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestSaveTTSConfigEncryptsKey` | ❌ Wave 0 |
| D-08 | JWT signing with per-overlay secret, claims validated | unit | `go test ./services/overlay-manager/tts/ -run TestSignVerifyJWT` | ❌ Wave 0 |
| D-08, D-10 | Rotating secret invalidates old JWTs | unit | `go test ./services/overlay-manager/tts/ -run TestRotationInvalidatesOldTokens` | ❌ Wave 0 |
| D-11 | `POST /tts-config` requires premium | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestSaveTTSConfigRequiresPremium` | ❌ Wave 0 |
| D-14 | `GET /tts-voices` proxies to ElevenLabs | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestGetVoicesProxies` (using httptest mock) | ❌ Wave 0 |
| D-16 | `POST /tts` streams audio with correct Content-Type | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestHandleTTSStreamsAudioMpeg` | ❌ Wave 0 |
| D-16 | `POST /tts` rejects missing tts_token | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestHandleTTSRequires401OnBadToken` | ❌ Wave 0 |
| D-16 | Client disconnect propagates context cancellation to upstream | integration | `go test ./services/overlay-manager/handlers/tts_test.go -run TestHandleTTSCancelPropagates` | ❌ Wave 0 |
| D-19..D-24 | `TTSGroup` renders all 20 settings | unit | `npm test -- --run frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` | ❌ Wave 0 |
| D-22 | `TTS_SETTINGS_UPDATE` postMessage updates the player | unit (embed page test) | `npm test -- --run frontend/src/app/overlays/[id]/preview/embed/__tests__/tts-update.test.tsx` | ❌ Wave 0 (optional) |
| D-25..D-30 | Content formatter: username "says", URL→"link", emote strip | unit | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "formatContent"` | ❌ Wave 0 |
| D-31 | Priority event detection: sub/raid/bits speak with prefix | unit | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "priority"` | ❌ Wave 0 |
| D-32 | `like_aggregate` always excluded | unit | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "excludes like_aggregate"` | ❌ Wave 0 |
| D-33 | Priority event + full queue drops oldest non-priority | unit | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "drop oldest non-priority"` | ❌ Wave 0 |
| D-35 | Per-user cooldown suppresses rapid repeat messages | unit (fake timers) | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "cooldown"` | ❌ Wave 0 |
| D-36 | Token bucket consumed; priority bypasses | unit (fake timers) | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "token bucket"` | ❌ Wave 0 |
| D-37 | Staleness drops old messages at dequeue | unit (fake timers) | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "stale"` | ❌ Wave 0 |
| D-38 | ElevenLabs failure sets sessionFallback, next speak goes to Web Speech | unit (mock fetch) | `npm test -- --run frontend/src/lib/utils/__tests__/ttsPlayer.test.ts -t "session fallback"` | ❌ Wave 0 |
| D-41 | Live overlay calls `ttsPlayerRef.current?.speak` after filter, alongside sound | manual + smoke | run `make frontend-dev` → open overlay → send mock message → verify console shows TTS speak call | manual |
| D-42 | Filter-blocked messages trigger neither sound nor TTS | unit (integration of overlay/[id] page) | `npm test -- --run frontend/src/app/overlay/[id]/__tests__/page.test.tsx -t "filtered message"` | ❌ Wave 0 (optional — manual OK) |
| E2E | OBS URL + token grants TTS access; rotation revokes | manual | OBS browser source test — editor "Copy URL" → paste in OBS → send msg → hear speech → click "Regenerate" → refresh OBS → TTS fails | manual |

### Sampling Rate

- **Per task commit:** Run the frontend unit suite only: `cd frontend && npm test -- --run src/lib/utils/__tests__/ttsPlayer.test.ts` — under 15 seconds.
- **Per wave merge:** Full frontend + overlay-manager unit + handler tests: `cd frontend && npm test -- --run && go test ./services/overlay-manager/...` — under 2 minutes.
- **Phase gate:** `make test` (full suite green) before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` — covers D-25..D-38 (biggest single test file, ~30 cases)
- [ ] `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` — covers D-19..D-23
- [ ] `services/overlay-manager/handlers/tts_test.go` — covers D-11..D-16
- [ ] `services/overlay-manager/tts/jwt_test.go` — covers D-08, D-10
- [ ] `services/overlay-manager/repository/tts_config_repo_test.go` — covers D-06 DB roundtrip
- [ ] (existing) `shared/encryption/encryption_test.go` already covers AES-GCM roundtrip — reuse, don't duplicate in overlay-manager
- [ ] (existing) `services/share-service/middleware/premium_test.go` already covers premium middleware — if moved to `shared/middleware/`, tests move with it

**Framework install:** None — vitest and go test are both already installed.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | JWT via `golang-jwt/jwt/v5` (user JWT for 5 of 6 endpoints + per-overlay tts_token for `/tts`) |
| V3 Session Management | yes | Session-wide fallback flag is memory-only per browser session (D-38); OBS URL JWT has no session semantics (stateless bearer) |
| V4 Access Control | yes | `middleware.RequirePremium("tts")` for the 5 user-auth endpoints; JWT scope check for `/tts`; owner-only access to `overlay_tts_configs` verified via overlay ownership check (pattern at `handlers/config.go:88`) |
| V5 Input Validation | yes | `text` query param bounded via `tts_max_message_chars` (default 200, client-side enforced + server-side defensive truncate); voice_id is alphanumeric pattern; no raw SQL — pgx parameterized queries only |
| V6 Cryptography | yes | AES-GCM via `shared/encryption.AESEncryptor` (AES-256, random 12-byte nonce, auth tag included); HS256 JWT; **never** roll own crypto |
| V7 Error Handling | yes | D-38 intentional error-swallowing with user toast; test-key endpoint D-39 verbose; avoid leaking stack traces in 5xx |
| V8 Data Protection | yes | ElevenLabs API key is secret at rest (encrypted BYTEA) and in transit (HTTPS to api.elevenlabs.io); never returned in any API response; not logged |
| V9 Communications | yes | HTTPS for ElevenLabs; TLS for allch.at → overlay-manager (gateway-terminated) |
| V13 API Security | yes | No CORS wildcard in overlay-manager itself (CORS is at api-gateway); JWT-required on all mutating endpoints |

### Known Threat Patterns for TTS stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| ElevenLabs key theft from DB | Information Disclosure | AES-GCM encryption at rest; master key from env var `TOKEN_ENCRYPTION_KEY` (Kubernetes secret) |
| ElevenLabs key theft via logs | Information Disclosure | Never log `req.api_key`, `cipher.DecryptString(...)` result, or `req.Header.Get("xi-api-key")`. Zap logger treats structured fields safely; planner must ensure no `zap.String("apiKey", ...)` patterns. |
| Replay attack on old tts_token | Spoofing | D-10 rotation-based revocation via new `tts_signing_secret`; no `jti` / revocation-list (stateless is acceptable given rotation model) |
| Algorithm-confusion attack on JWT | Tampering | `jwt.ParseWithClaims` callback enforces `*jwt.SigningMethodHMAC` only [VERIFIED: pattern in shared/auth/jwt.go:156] |
| Timing attack on JWT signing-secret comparison | Information Disclosure | `jwt/v5` library uses constant-time compare; don't reimplement |
| CSRF on POST /tts-config | Tampering | JWT-based auth (no cookies), same-origin policy + CORS from api-gateway provide CSRF mitigation |
| Overlay ID enumeration | Information Disclosure | Overlay IDs are UUIDv4 — 122 bits of randomness, not enumerable |
| Billing abuse (attacker spams /tts to burn user's quota) | Denial of Service | Per-overlay rate limit (60/min) on the proxy; JWT gates access; rotation on compromise |
| Text injection / prompt injection via `text` param | Elevation of Privilege | Bounded length; ElevenLabs treats it as speech text, not a prompt; no command parsing |
| Auto-fallback cascade (attacker triggers ElevenLabs 500s to force Web Speech) | Tampering | D-38 is acceptable — Web Speech is a graceful degradation, not a security boundary |
| Nonce collision (AES-GCM) | Tampering | Random 12-byte nonce per encryption operation — collision probability is 2^-48 even at 10^7 encryptions [CITED: NIST SP 800-38D] |
| Public-config leak (issue #270 original design) | Information Disclosure | D-05 fix: key is NOT in `display_settings`. Public endpoint at `handlers/config.go:182` already verified to expose only `display_settings`, `filter_settings`, `custom_css`, `visual_settings`, `sources` — no `overlay_tts_configs` table read. |

## Sources

### Primary (HIGH confidence)

- **Codebase files (all read directly):**
  - `.planning/phases/13-text-to-speech-tts-for-chat-messages/13-CONTEXT.md` (42 locked decisions)
  - `.planning/ROADMAP.md` lines 369-377
  - `.planning/STATE.md`
  - `.planning/config.json` (nyquist_validation implicit — not disabled)
  - `services/overlay-manager/cmd/main.go` (verified: NO featuregates wiring currently)
  - `services/share-service/featuregates/cache.go` (verified: no prometheus imports — safe to move)
  - `services/share-service/middleware/premium.go` (verified: clean interface, testable)
  - `services/share-service/cmd/main.go` (reference wiring pattern, lines 103-105, 186-190)
  - `shared/encryption/encryption.go` (verified: AES-GCM already implemented)
  - `shared/crypto/crypto.go` (verified: second AES-GCM package also exists)
  - `shared/auth/jwt.go` (verified: golang-jwt/jwt/v5 wrapper patterns)
  - `shared/middleware/auth.go` (verified: JWTAuth + AdminOnly patterns)
  - `services/auth-service/cmd/main.go` lines 134-139 (verified: TOKEN_ENCRYPTION_KEY is already used)
  - `services/overlay-manager/handlers/config.go` lines 144-188 (verified: HandleGetPublicConfig exposes display_settings)
  - `services/overlay-manager/models/config.go` (verified: DisplaySettings is `map[string]any`)
  - `services/api-gateway/cmd/main.go` lines 413-482 (verified: `/api/v1/overlays/*` proxy routing)
  - `services/api-gateway/handlers/proxy.go` line 130 (verified: `io.Copy(c.Writer, backendResp.Body)` streaming pattern)
  - `frontend/src/lib/utils/soundPlayer.ts` (Phase 12 reference)
  - `frontend/src/lib/utils/filterMessage.ts` (Phase 11 reference)
  - `frontend/src/lib/utils/__tests__/soundPlayer.test.ts` (verified: `vi.stubGlobal` + `vi.useFakeTimers` pattern)
  - `frontend/src/components/appearance/SoundGroup.tsx` (Phase 12 reference)
  - `frontend/src/components/appearance/AppearancePanel.tsx` (mount point at line 89-97)
  - `frontend/src/app/overlay/[id]/page.tsx` lines 110-414 (verified: filter at 411, sound at 414, destroy at 139, load at 232)
  - `frontend/src/app/overlays/[id]/preview/embed/page.tsx` lines 48-400 (verified: SOUND_SETTINGS_UPDATE at 272, load at 347, playback at 400)
  - `frontend/src/app/overlays/[id]/page.tsx` lines 1388-1762 (verified: config load at 1393, save at 1727)
  - `frontend/src/lib/types/overlay.ts` (verified: DisplaySettings structure)
  - `frontend/src/lib/types/message.ts` (verified: EventType enum)
  - `migrations/001_initial_schema.sql` (verified: overlays table, display_settings JSONB)
  - `migrations/044_feature_gates.sql` (verified: ON CONFLICT DO NOTHING pattern)
  - `migrations/048_impersonation_audit_log.sql` (verified: latest = next is 049)
  - `docs/adr/0008-feature-gate-infrastructure.md` (verified: explicit blessing of move to shared/)
  - `shared/encryption/encryption_test.go` (verified: test pattern for AES roundtrip)
  - `services/share-service/middleware/premium_test.go` (verified: middleware test pattern with GateChecker mock + premiumQuerier injection)
  - `services/overlay-manager/go.mod` (verified: Gin v1.12.0, pgx v5.9.1, Go 1.25.6)

- **Git:** commit `b499543a chore: add AGPL-3.0 license headers to all source files and Dockerfiles` (verified via `git log`)

### Secondary (MEDIUM confidence — WebFetch / WebSearch verified with official sources)

- [ElevenLabs Stream speech docs](https://elevenlabs.io/docs/api-reference/text-to-speech/stream) — confirmed `POST /v1/text-to-speech/{voice_id}/stream`, `text` required, `model_id` default `eleven_multilingual_v2`, `xi-api-key` header, response is streaming audio (`audio/mpeg` via chunked transfer)
- [ElevenLabs List voices docs](https://elevenlabs.io/docs/api-reference/voices/search) — GET /v2/voices returns paginated `{voices[], has_more, total_count, next_page_token}`; each voice has `voice_id`, `name`, `category`, `labels`, `preview_url`, `settings`, `description`
- [ElevenLabs User endpoint docs](https://elevenlabs.io/docs/api-reference/user/get) — `subscription.character_count`, `character_limit`, `tier`, `next_character_count_reset_unix`
- [MDN SpeechSynthesisUtterance](https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesisUtterance) — properties `text`, `voice`, `rate (0.1-10)`, `pitch (0-2)`, `volume (0-1)`, `lang`; events `onstart`, `onend`, `onerror`, `onboundary`
- [MDN SpeechSynthesis getVoices](https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesis/getVoices) — asynchronous loading; use `onvoiceschanged` event to repopulate
- [Gin Context.Stream godoc](https://pkg.go.dev/github.com/gin-gonic/gin#Context.Stream) — signature `func(step func(io.Writer) bool) bool`; auto-flushes after each step; returns false on client disconnect

### Tertiary (LOW confidence — WebSearch only)

- [dev.to: Cross-browser Speech Synthesis](https://dev.to/jankapunkt/cross-browser-speech-synthesis-the-hard-way-and-the-easy-way-353) — claims Firefox/Safari Desktop return voices synchronously; Chrome Desktop returns empty initially. Web search consensus.
- [Apple Developer Forum thread 723503](https://developer.apple.com/forums/thread/723503) — Safari iOS may require user gesture before voices populate (anecdotal; flagged as known limitation only)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified via direct file read
- Architecture: HIGH — patterns from Phase 11/12 are directly adjacent and explicitly called out in CONTEXT.md
- Pitfalls: HIGH for codebase-specific (line numbers, existing AES-GCM package), MEDIUM for ElevenLabs runtime pitfalls (not yet integrated in this codebase), MEDIUM for Web Speech cross-browser (web-search-only on some browsers)
- ElevenLabs API: MEDIUM — verified via WebFetch on one endpoint + search summary on others; spec is well-documented and stable
- Web Speech API: HIGH for core API (MDN), MEDIUM for cross-browser quirks (community articles)
- Integration line numbers: HIGH — directly grep'd and read

**Research date:** 2026-04-23
**Valid until:** 2026-05-23 (30 days; stack is stable; ElevenLabs API is mature; no breaking changes expected in Next.js 16 / Go 1.25 within this window)

---

## RESEARCH COMPLETE

**Phase:** 13 - Text-to-Speech (TTS) for chat messages
**Confidence:** HIGH

### Key Findings

- **`shared/encryption.AESEncryptor` already exists and is production-deployed** (auth-service OAuth tokens since 2025-02). D-07's "new `shared/crypto` package" is moot — reuse existing. A second redundant `shared/crypto.AESGCMCipher` also exists; flag for cleanup.
- **Featuregates infrastructure is ready**: `FeatureGateCache` at `services/share-service/featuregates/cache.go` has zero prometheus imports → safe to move to `shared/featuregates/` per ADR-0008's explicit blessing for the "second consumer" case. `RequirePremium` middleware has clean `GateChecker` interface + injectable `premiumQuerier` for tests.
- **Integration line numbers VERIFIED**: live overlay filter at `page.tsx:411`, sound play at `page.tsx:414` → TTS hook goes immediately adjacent at line 414 (parallel with sound, both after filter). Embed preview mirrors at line 272 (SOUND_SETTINGS_UPDATE) + line 397 (filter) + line 400 (sound).
- **Public config endpoint VERIFIED to expose full `display_settings` JSONB** (`handlers/config.go:182`) — confirms D-05 threat model. Never put the ElevenLabs key there.
- **ElevenLabs streaming spec CONFIRMED**: `POST /v1/text-to-speech/{voice_id}/stream` with `xi-api-key` header + `{text, model_id: "eleven_multilingual_v2"}` body returns `audio/mpeg` via chunked transfer. Gin `io.Copy(c.Writer, resp.Body)` + `http.NewRequestWithContext(c.Request.Context(), ...)` handles both streaming AND client-disconnect cancellation.
- **Web Speech API cross-browser quirk**: `getVoices()` is async in Chromium (including OBS CEF) — requires `onvoiceschanged` handler. Sync in Firefox/Safari. Pattern verified on MDN + community sources.
- **Migration 049 is the next free slot** (048 is latest). Use `INSERT INTO feature_gates ... ON CONFLICT DO NOTHING` pattern verified in migration 044.
- **Env var recommendation**: Reuse `TOKEN_ENCRYPTION_KEY` instead of creating `CRYPTO_MASTER_KEY` (D-07) — matches the auth-service convention and avoids a parallel secret. Planner must add the env var to the overlay-manager k8s deployment.
- **Tests: Wave 0 gap list is 5 new files** (ttsPlayer.test.ts, TTSGroup.test.tsx, tts_test.go handler, jwt_test.go, tts_config_repo_test.go). Everything else reuses existing patterns — no framework installs needed.

### File Created

`.planning/phases/13-text-to-speech-tts-for-chat-messages/13-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Every package verified via direct file read (`shared/encryption`, `shared/auth`, `shared/featuregates`, `shared/middleware`) |
| Architecture | HIGH | Phase 11+12 patterns directly adjacent; integration line numbers grep-verified |
| Pitfalls | HIGH (codebase) / MEDIUM (ElevenLabs runtime) | Codebase pitfalls from direct read of ADR-0008 + STATE.md; ElevenLabs pitfalls from API spec review |
| ElevenLabs API | MEDIUM | Spec verified via WebFetch for streaming endpoint; voices + user endpoints cross-referenced via WebSearch |
| Web Speech API | HIGH (core) / MEDIUM (cross-browser) | MDN primary; cross-browser from community sources |

### Open Questions (enumerated in main body)

1. Merge or deprecate one of `shared/crypto` vs `shared/encryption`? (Recommendation: use encryption, deprecate crypto)
2. Use existing `shared/ratelimit` or in-memory Map for the 60/min server-side cap? (Recommendation: inspect during Plan 02; both work)
3. Add a 7th endpoint `GET /tts-config` returning `{has_elevenlabs_config, voice_id, obs_url}`? (D-24 implies yes; recommendation: yes, add in Plan 02 or 03)
4. Test-Key endpoint response format: stream audio or return URL? (Recommendation: stream inline, same pattern as `/tts`)

### Ready for Planning

Research complete. Planner has: concrete file path inventory, verified integration line numbers, confirmed existing crypto + JWT + featuregates packages for reuse, end-to-end system diagram, 12 numbered pitfalls with mitigations, 5 code snippets for orientation, complete validation test matrix, 10 assumptions logged. No blockers.
