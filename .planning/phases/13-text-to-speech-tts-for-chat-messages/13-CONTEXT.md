# Phase 13: Text-to-Speech (TTS) for chat messages - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Read chat messages aloud in the overlay. Ships two tiers in this phase:

- **Free (default):** browser-native Web Speech API. No server round-trip. Voices vary per browser.
- **Premium:** ElevenLabs — user enters an API key in the dashboard, the backend stores it encrypted (AES-GCM), and the overlay calls a **backend TTS proxy** that forwards to ElevenLabs. The key never reaches the browser.

Also in scope:
- A `TTSQueue` client utility with sampling, per-user cooldown, token-bucket rate limiter, staleness discard, and priority bypass for subs/raids/bits/superchats.
- A single `Text-to-Speech` section in the `AppearancePanel` (with inline sub-headers) containing all ~20 TTS settings.
- Wiring in `overlay/[id]/page.tsx` and `overlays/[id]/preview/embed/page.tsx` so TTS plays in the live overlay AND the editor embed preview.
- A feature-gate row `tts` in the `feature_gates` table so admins can flip the whole feature free/premium without a redeploy.
- Backend: new DB migration, new `overlay_tts_configs` table (encrypted key + voice + per-overlay JWT signing secret), new `shared/crypto` AES-GCM package, TTS proxy + config + voice-list + test-key + token-rotation handlers in `overlay-manager`.
- No other backend services change. Message-processor stays stateless — TTS is entirely client-side (playback) + overlay-manager (proxy).

**Not in scope (new capabilities → other phases):**
- Mod-flagged message TTS triggered by chat commands (e.g. `/highlight`)
- Server-side TTS audio caching (frequent-phrase cache)
- Per-viewer TTS opt-out
- Custom voice cloning via ElevenLabs
- Speech-to-text (input side)

</domain>

<decisions>
## Implementation Decisions

### Phase Scope & Tiering

- **D-01:** Both tiers (Web Speech free + ElevenLabs premium) ship in Phase 13. Single phase, full feature.
- **D-02:** Web Speech tier is free. ElevenLabs tier is premium. Matches Phase 12's sound pattern (free presets + premium custom URL).
- **D-03:** TTS is registered as a row in the `feature_gates` table (`tts`, `is_premium=true`, description `"Text-to-speech for chat messages"`). Entire capability (both tiers) is behind the gate — admin can flip the single row to ship TTS as free later without a redeploy. Uses the existing Phase 07 feature-gate infrastructure.
- **D-04:** Plan count: **3 plans** (split from the original 2-plan guess because the backend scope expanded materially).
  - Plan 01 — Web Speech tier: `ttsPlayer.ts` TDD utility + queue/sampling/cooldown/rate-limiter + `TTSGroup.tsx` (Web-Speech controls) + `AppearancePanel` wiring + `overlay/[id]/page.tsx` + `overlays/[id]/preview/embed/page.tsx` TTS_SETTINGS_UPDATE postMessage + `feature_gates` row insertion.
  - Plan 02 — ElevenLabs backend: migration 049 (`overlay_tts_configs` table + per-overlay `tts_signing_secret`), new `shared/crypto` AES-GCM package, overlay-manager TTS handlers (POST config/key, POST rotate-token, POST test-key, GET voices, POST tts proxy) + featuregates middleware wiring in overlay-manager.
  - Plan 03 — ElevenLabs frontend + UX: API key input + voice picker + Test-Key button + Copy-OBS-URL-with-token button + premium gating UI + session-fallback-to-Web-Speech behavior + character-quota display.
  - Planner may adjust boundaries; 3 plans is the starting shape.

### ElevenLabs Key Storage & Auth (Security Critical)

- **D-05:** ElevenLabs API key is stored **server-side only**, never in the browser. Overlay calls a backend TTS proxy which forwards to ElevenLabs with the stored key. Fixes the public-config-endpoint leak that issue #270's literal spec would have created.
- **D-06:** Key is stored **AES-GCM encrypted** in a new `overlay_tts_configs` table (fields: `overlay_id`, `encrypted_api_key`, `voice_id`, `tts_signing_secret`, timestamps). Fulfills the CLAUDE.md tech-debt item "Token encryption is basic (TODO: implement AES-GCM)".
- **D-07:** New `shared/crypto` package provides the AES-GCM wrapper (Encrypt/Decrypt funcs, 12-byte nonce, AES-256). Master key is read from env var `CRYPTO_MASTER_KEY` (base64-encoded 32 bytes). Kubernetes secret delivery follows the existing sealed-secrets pattern.
- **D-08:** OBS overlay authenticates to the TTS proxy via a **per-overlay signed JWT** embedded in the OBS URL: `/overlay/{id}?tts_token=XXX`. The JWT is signed with the per-overlay `tts_signing_secret` (not the user's session key), claims: `{sub: overlay_id, scope: "tts:use", iat}`. No expiration on the JWT itself — revocation is by rotating the overlay's signing secret.
- **D-09:** The OBS URL is surfaced to the user via a **"Copy OBS URL" button** in the dashboard (next to the TTS section in the editor). Clicking builds the URL and copies to clipboard with a success toast. The plain `/overlay/{id}` URL (without token) continues to work — TTS just won't activate without the token.
- **D-10:** Token rotation is via a **"Regenerate OBS URL"** button. Click rotates the `tts_signing_secret` for that overlay (new random value), invalidates all previously issued JWTs, and returns a new URL. Intentional one-button action, no background auto-rotation.

### Backend API Surface (overlay-manager)

New endpoints on `overlay-manager`:

- **D-11:** `POST /api/v1/overlays/:id/tts-config` — auth-required (user JWT), gated by `RequirePremium("tts")` middleware. Body: `{api_key, voice_id}`. Saves encrypted key + voice.
- **D-12:** `DELETE /api/v1/overlays/:id/tts-config` — auth-required, removes the stored key + config row.
- **D-13:** `POST /api/v1/overlays/:id/tts-config/rotate-token` — auth-required, rotates `tts_signing_secret`, returns `{obs_url}`.
- **D-14:** `GET /api/v1/overlays/:id/tts-voices` — auth-required. Backend fetches `GET https://api.elevenlabs.io/v1/voices` using the stored key, returns the list. Lazy — only called when the voice dropdown opens.
- **D-15:** `POST /api/v1/overlays/:id/tts-config/test` — auth-required. Backend calls ElevenLabs `GET /v1/user` to validate + returns remaining character quota, optionally synthesizes a 2-second "Hello from All-Chat" sample and streams back to the browser for playback.
- **D-16:** `POST /api/v1/overlays/:id/tts?text=...&voice=...` — **tts_token JWT auth (not user JWT)**. Proxies to ElevenLabs `POST /v1/text-to-speech/{voice_id}/stream`, streams the audio response back to the overlay (Content-Type: audio/mpeg; chunked). Applies server-side per-overlay rate limiting (sanity cap: 60 requests/min per overlay).
- **D-17:** overlay-manager must **wire featuregates** (it doesn't today — currently only share-service uses the featuregates cache). Plan 02 adds the `featuregates.Cache` init in `cmd/main.go` and imports `middleware.RequirePremium` from share-service (or a shared move).

### Settings UI Organization

- **D-18:** A single `CollapsibleSection` in `AppearancePanel.tsx` titled **"Text-to-Speech"**, mirroring the Filters/Notification-Sounds pattern. Inside, inline sub-sections with small uppercase labels (and horizontal rule separators) in this order: **Voice** → **Throttling** → **Content** → **Priority** → **Advanced** (ElevenLabs block).
- **D-19:** New component `frontend/src/components/appearance/TTSGroup.tsx` — analog to `SoundGroup.tsx` + `FilterGroup.tsx`. Takes `displaySettings`, `onChange`, `isPremium`, and optional `onPreview`.
- **D-20:** **No preset templates** (Quiet/Chatty/Priority-only). Ship **safe defaults** from issue #270 (filter_mode=`sample`, sample_rate=0.25, cooldown=30s, rate_limit=8/min, staleness=15s, priority_events=true, all 5 platforms enabled) — first-enable works sensibly. Streamers tune from there. Presets can be added later if users ask.
- **D-21:** **"Test voice" button** next to the voice picker. Speaks fixed sample text `"Hello, this is how your chat will sound."` through the current provider/voice/rate/pitch. Click again mid-speech = cancel. For ElevenLabs, the test hits `POST /tts-config/test` (D-15).
- **D-22:** Editor → embed preview live update via `TTS_SETTINGS_UPDATE` postMessage, mirroring Phase 12's `SOUND_SETTINGS_UPDATE` pattern exactly. No debouncing — fires on every change, ttsPlayer settings update in place.
- **D-23:** Character-quota display: shown under the Test-Key button after a successful test (e.g. `"Quota remaining: 8,432 / 10,000 characters this month"`). Not polled — only refreshed on Test-Key click.

### DisplaySettings Extension

- **D-24:** Add to `DisplaySettings` in `frontend/src/lib/types/overlay.ts` (and map onto the JSONB blob backend-side; `map[string]any` needs no schema change):

```ts
// Core
tts_enabled?: boolean                    // default: false
tts_provider?: 'browser' | 'elevenlabs'  // default: 'browser'
tts_volume?: number                      // 0–1, default: 0.8

// Web Speech API options
tts_voice_uri?: string                   // browser voice URI
tts_rate?: number                        // 0.5–2.0, default: 1.0
tts_pitch?: number                       // 0–2, default: 1.0

// ElevenLabs premium (NOTE: key+voice_id live in overlay_tts_configs server-side, NOT here)
// The frontend knows "has a saved key?" via a hasElevenLabsConfig boolean returned by a new endpoint

// Message selection / throttling
tts_filter_mode?: 'all' | 'sample' | 'priority_only'  // default: 'sample'
tts_sample_rate?: number                 // 0.0–1.0, default: 0.25
tts_max_queue?: number                   // default: 5
tts_messages_per_minute?: number         // default: 8
tts_user_cooldown_seconds?: number       // default: 30
tts_staleness_seconds?: number           // default: 15

// Priority overrides
tts_priority_events?: boolean            // default: true
tts_priority_bits_min?: number           // default: 0

// Content formatting
tts_read_username?: boolean              // default: true
tts_read_platform?: boolean              // default: false
tts_max_message_chars?: number           // default: 200
tts_skip_emote_only?: boolean            // default: true
tts_skip_links?: boolean                 // default: true
tts_enabled_platforms?: string[]         // default: ['twitch','youtube','kick','tiktok','discord']
```

**Important:** The ElevenLabs `api_key` and `voice_id` from issue #270 are **explicitly NOT** in `display_settings` — they live in the new `overlay_tts_configs` table. The frontend only knows "is there a saved key?" via a `has_elevenlabs_config` boolean returned by a new GET endpoint.

### Content Formatting & Voice Behavior

- **D-25:** Username format: `"{display_name} says: {message}"` when `tts_read_username=true`. Uses `display_name` (friendlier than login). Omits when `false`.
- **D-26:** Platform prefix: when `tts_read_platform=true`, format becomes `"{Platform}: {display_name} says: {message}"` (e.g., `"Twitch: Nightbot says hello"`). Default `false`.
- **D-27:** Single user-picked voice for all messages. No auto language detection. Non-Latin characters may sound garbled through Web Speech — document as known limitation in the UI tooltip. ElevenLabs handles multilingual via its own model.
- **D-28:** Web Speech voice URI stability: voice list is browser-specific. If the persisted `tts_voice_uri` isn't in the current browser's voice list on load, fall back to the default voice for the browser's detected language. Log a console warning. No UI for language-per-voice routing.
- **D-29:** Emote handling: strip all tokens matching `message.emotes[]` positions from the text. After stripping, if emotes took >50% of original token count AND `tts_skip_emote_only=true`, skip the message entirely. Otherwise speak the remainder.
- **D-30:** URL handling: regex-detect `https?://` URLs, replace each with the literal word `"link"`. If the message after stripping contains only whitespace/punctuation AND `tts_skip_links=true`, skip entirely.
- **D-31:** Event announcement: only **priority event types** speak (subscription, resubscription, gift_subscription, mystery_gift, bits, raid, super_chat, super_sticker, kick_subscription, kick_gift_subscription, kick_donation). Format with event-specific prefix: `"New subscription from {display_name}"` / `"{display_name} raided with N viewers"` / `"{display_name} cheered N bits: {message}"` / `"Super chat from {display_name}: {message}"`. Non-priority events (follow, channel_points, like_aggregate, ritual) never trigger TTS. `tts_priority_events=true` gates this.
- **D-32:** Default platform enablement: all 5 platforms enabled by default (`['twitch','youtube','kick','tiktok','discord']`). TikTok `like_aggregate` event type is always excluded regardless (too noisy by construction). User disables per-platform via checkboxes in the TTSGroup.

### Priority & Queue Behavior

- **D-33:** Priority event + queue full: **drop oldest non-priority queued message and enqueue the priority event at the back**. Current utterance is NEVER canceled mid-word. The currently-speaking message finishes naturally. Priority events always have room because of the eviction. Feels smooth, doesn't interrupt.
- **D-34:** Queue ordering: FIFO by insertion. Priority events don't jump queue — they just displace a non-priority on overflow. A priority event arriving with queue slots available simply appends normally.
- **D-35:** Per-user cooldown: tracked via `Map<username, lastSpokenAt>` in memory. Cleared on overlay reload. `tts_user_cooldown_seconds` default 30 prevents one chatter from monopolizing audio.
- **D-36:** Token-bucket rate limiter: bucket size = `tts_messages_per_minute`, refill = full bucket every 60 seconds, no persistence. Priority events bypass the rate limiter entirely. Reset on page reload.
- **D-37:** Staleness: when dequeuing, if `now - message.timestamp > tts_staleness_seconds * 1000`, drop silently. Prevents a backlog from spiking the audio after a pause.

### ElevenLabs Failure Behavior

- **D-38:** On any ElevenLabs failure during a live session (401/403/429/5xx/network/timeout), switch the entire session to Web Speech fallback and show **one toast**: `"ElevenLabs unavailable — using browser voice."` No further toasts. Fallback persists until the overlay page is reloaded. Simple, robust, user-obvious.
- **D-39:** Test-Key button reports errors verbosely (401 → `"Invalid API key"`, 429 → `"Rate-limited — try again in a minute"`, 5xx → `"ElevenLabs service unavailable"`). Testing is a debug/setup flow, so verbose is fine.
- **D-40:** Streaming audio: overlay receives the proxy response as `ReadableStream<Uint8Array>`, pipes into an `AudioContext` via `decodeAudioData` chunks, or simpler `new Audio(URL.createObjectURL(blob))` once the full stream completes. Planner/researcher picks the approach — either works. Start with the blob approach for simplicity.

### Playback Integration Point

- **D-41:** In `overlay/[id]/page.tsx` and `overlays/[id]/preview/embed/page.tsx`, TTS hook fires **after** `shouldFilterMessage` and **in parallel with** `soundPlayerRef.current?.play()`. Same insertion point as Phase 12. Call signature: `ttsPlayerRef.current?.speak(message)`.
- **D-42:** TTS and notification sound are independent — both can fire for the same message. Filtered messages trigger neither.

### Claude's Discretion

- Exact nonce strategy for AES-GCM (random 12-byte per encryption is standard)
- Whether `shared/crypto` exports just `Encrypt([]byte) ([]byte, error)` / `Decrypt([]byte) ([]byte, error)` or a more elaborate key-rotation API (keep it small for now)
- Whether the featuregates middleware lives in `shared/featuregates` (moved from share-service) or is re-imported/duplicated in overlay-manager (pragmatic: move it to `shared/featuregates`)
- Streaming audio decode approach (blob vs AudioContext chunks)
- Exact wording/emoji of the "Copy OBS URL" and "Regenerate OBS URL" buttons
- Character-quota display formatting (commas, K-suffix, percent bar)
- Test-sample text language (English only, or pick based on detected voice lang)
- Server-side rate limit for `/tts` proxy (default 60/min/overlay; tune if load warrants)
- Whether to write a new ADR for AES-GCM crypto or note it inline (recommend: write `0012-aes-gcm-secret-encryption.md`)
- The exact event-type prefix strings (D-31) — fine-tune copy during implementation

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source Spec
- `github.com/caesarakalaeii/all-chat/issues/270` — Full TTS feature spec from the user (settings list, throttling algorithm, implementation plan, out-of-scope list). Fetched 2026-04-23. **This is the spec of record**; D-05..D-17 overrule the issue's "stored in JSONB" clause because of the public-config-endpoint leak.

### Prior Phase Contexts (identical-pattern prior art)
- `.planning/phases/11-add-username-keyword-exclude-list-to-overlay-filter-settings/11-CONTEXT.md` — Filters: pure utility + AppearancePanel group + client-side hook pattern. Mirror exactly.
- `.planning/phases/12-notification-sound-on-incoming-messages-with-premium-custom-/12-CONTEXT.md` — Sound: audio-pool utility, premium gating pattern, postMessage live-update pattern, DisplaySettings JSONB extension. Mirror exactly.
- `.planning/phases/07-feature-gate-infrastructure/07-CONTEXT.md` — Feature-gate cache + RequirePremium middleware + admin toggle. Phase 13 registers a new `tts` gate row.

### ADRs
- `docs/adr/0008-feature-gate-infrastructure.md` — Feature gates (applies for `tts` registration).
- (New) `docs/adr/0012-aes-gcm-secret-encryption.md` — Write during Plan 02; documents the `shared/crypto` package.

### Frontend — Types
- `frontend/src/lib/types/overlay.ts` — `DisplaySettings` (lines 54–75, extend), `OverlayConfig`. Add all `tts_*` fields from D-24.
- `frontend/src/lib/types/message.ts` — `ChatMessage`, `EventType`, `EventTier`, `EventInfo`, `UserInfo`. Used by ttsPlayer to extract display_name, event type, platform.

### Frontend — Existing Patterns to Mirror
- `frontend/src/lib/utils/soundPlayer.ts` — Pure utility with pool + cooldown + settings + destroy. `ttsPlayer.ts` follows this shape (plus queue/sampling/rate-limiter additions).
- `frontend/src/lib/utils/filterMessage.ts` — Pure boolean function signature. ttsPlayer's "should speak this?" internal check can follow.
- `frontend/src/components/appearance/SoundGroup.tsx` — Props `(displaySettings, onChange, isPremium, onPreview?)`. `TTSGroup.tsx` mirrors the signature.
- `frontend/src/components/appearance/FilterGroup.tsx` — Tag-style list inputs (used for `tts_enabled_platforms` multi-select) and AppearancePanel integration.
- `frontend/src/components/appearance/AppearancePanel.tsx` — Where the new `CollapsibleSection id="tts" title="Text-to-Speech"` is mounted (mirror the `id="sounds"` block).
- `frontend/src/components/appearance/CollapsibleSection.tsx`, `SliderControl.tsx`, `ToggleSwitch.tsx`, `ColorPickerControl.tsx` — Reuse directly.
- `frontend/src/components/PremiumBadge.tsx` — Premium-gated input decorator. Used on API-key input and ElevenLabs voice picker.

### Frontend — Integration Points
- `frontend/src/app/overlay/[id]/page.tsx` — Live OBS overlay. Lines 100–145 show state/ref setup for filter + sound; TTS follows the same pattern. Line 411 is the filter check; line 414 is the sound-play hook; add `ttsPlayerRef.current?.speak(message)` adjacent to line 414 (both fire for non-filtered messages). Lines 232–256 show config-load for sound settings; mirror for TTS.
- `frontend/src/app/overlays/[id]/page.tsx` — Editor page. Line 1571, 2078, 2117 show the existing `user?.is_premium` gating pattern. Lines 1393–1403 show sound-settings load-on-mount; mirror. Lines 1741–1745 and surrounding `handleSaveConfiguration` show config save; extend with `tts_*` fields (display_settings only — ElevenLabs key saves via separate endpoint).
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — Editor embed preview iframe. Lines 48–51 import + 272–285 `SOUND_SETTINGS_UPDATE` listener + 347–369 config-load + 397–400 playback hook. Mirror all four for `TTS_SETTINGS_UPDATE`.

### Backend — overlay-manager
- `services/overlay-manager/models/config.go` — `OverlayConfig` with `DisplaySettings map[string]any`. No schema change; `tts_*` fields auto-merge.
- `services/overlay-manager/handlers/config.go` — Existing `display_settings` merge logic. New handlers for TTS config live in a new file `handlers/tts.go`.
- `services/overlay-manager/repository/config_repo.go` — Existing repository pattern. New `services/overlay-manager/repository/tts_config_repo.go` for `overlay_tts_configs` table CRUD (encrypted blob in, encrypted blob out; encryption handled at handler layer).
- `services/overlay-manager/cmd/main.go` — Where the new featuregates cache, crypto helper, and TTS handler are wired. Currently has no featuregates/premium middleware — Plan 02 adds the wiring.

### Backend — Existing Patterns to Reuse / Move
- `services/share-service/middleware/premium.go` — `RequirePremium(db, gates, featureKey, logger)` middleware. **Recommend moving to `shared/middleware/premium.go`** during Plan 02 so overlay-manager can import it. Alternative: duplicate. Planner's call.
- `services/share-service/featuregates/cache.go` — `FeatureGateCache` with in-memory + Pub/Sub invalidation. Move to `shared/featuregates/cache.go` (same migration decision as above). Overlay-manager imports it.
- `services/share-service/handlers/admin_featuregates.go` — Admin GET/PATCH endpoints for feature gates. No change needed (already handles arbitrary gate keys); Phase 13 just inserts a `tts` row via migration.

### Database
- `migrations/` — Latest migration `048_impersonation_audit_log.sql`. New: `049_overlay_tts_configs.sql` creates:
  - `overlay_tts_configs` table (`id UUID PK`, `overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE UNIQUE`, `encrypted_api_key BYTEA NOT NULL`, `voice_id TEXT NOT NULL`, `tts_signing_secret BYTEA NOT NULL`, `created_at`, `updated_at`)
  - `feature_gates` row: `INSERT INTO feature_gates (feature_key, is_premium, description) VALUES ('tts', true, 'Text-to-speech for chat messages') ON CONFLICT DO NOTHING;`
  - Down migration: `DROP TABLE overlay_tts_configs; DELETE FROM feature_gates WHERE feature_key = 'tts';`
- `migrations/001_initial_schema.sql` — `overlay_configs` table for reference (display_settings JSONB column).

### External APIs
- ElevenLabs API docs — https://elevenlabs.io/docs/api-reference
  - `GET /v1/voices` — voice list (D-14)
  - `GET /v1/user` — key validation + quota (D-15)
  - `POST /v1/text-to-speech/{voice_id}/stream` — streaming synthesis (D-16)
  - `GET /v1/user/subscription` — character quota detail (D-23)
  - CORS: does not matter because we proxy server-side.
- Web Speech API — https://developer.mozilla.org/en-US/docs/Web/API/Web_Speech_API
  - `window.speechSynthesis` (getVoices, speak, cancel, speaking, pending)
  - `SpeechSynthesisUtterance` (text, voice, rate, pitch, volume, lang, onstart, onend, onerror)

### Documentation / Context
- `CLAUDE.md` — Known Issues section: "Token encryption is basic (TODO: implement AES-GCM)" — this phase delivers the AES-GCM package via `shared/crypto`. Close this tech-debt item.
- `.planning/PROJECT.md` — Frontend stack (Next.js App Router, Tailwind v4, @base-ui/react), backend stack (Go 1.25, pgx/v5, redis/v9, Gin).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`SoundGroup.tsx` + `soundPlayer.ts`** (Phase 12) — the nearest analog. `TTSGroup.tsx` and `ttsPlayer.ts` should mirror their shape, tests, and integration points. One-to-one mapping from notification_sound_* fields to tts_* fields for the shared UX elements (enabled toggle, volume slider, preview button).
- **`FilterGroup.tsx` + `filterMessage.ts`** (Phase 11) — pattern for list-style settings (banned_users is similar to tts_enabled_platforms multi-select).
- **`CollapsibleSection`, `ToggleSwitch`, `SliderControl`, `ColorPickerControl`** — all directly reusable.
- **`PremiumBadge`** — wrap around the API-key input and the "Regenerate OBS URL" button for non-premium users (even though the feature-gate already blocks access, the badge communicates the tier).
- **`updateConfig()` in `frontend/src/lib/api/overlays.ts`** — already handles `display_settings` JSONB merge. New `tts_*` fields auto-persist. A new API client function is needed for the separate ElevenLabs key endpoints (`overlays.saveTTSKey()`, `overlays.rotateTTSToken()`, `overlays.getTTSVoices()`, `overlays.testTTSKey()`).
- **`middleware.RequirePremium`** + **`featuregates.FeatureGateCache`** (share-service) — **move to `shared/` in Plan 02** so overlay-manager can import. Pattern for the TTS-config endpoints.
- **`pg_notify` / Redis Pub/Sub feature-gate invalidation** (Phase 07) — already generic; no change needed for `tts` key.

### Established Patterns

- **Client-side feature with JSONB display_settings** — Phase 11 + 12 both use this. Backend `map[string]any` auto-merges; no migration for DisplaySettings fields.
- **Pure utility + React group component + AppearancePanel mount** — three-file pattern (`lib/utils/*.ts` + `components/appearance/*Group.tsx` + mount in `AppearancePanel.tsx`).
- **Live preview via postMessage** — `SOUND_SETTINGS_UPDATE` / `VISUAL_CSS_UPDATE`. Add `TTS_SETTINGS_UPDATE`.
- **TDD for pure utilities** — Phase 11 and 12 both wrote RED tests first, then implementation. `ttsPlayer.ts` should follow (queue behavior, sampling, cooldown, rate limiter, staleness, priority bypass all testable in isolation with fake timers + mocked `speechSynthesis`).
- **Premium gating is frontend-only for display settings** — Phase 12 custom-sound URL works this way. But note: **ElevenLabs key upload is NOT display_settings** and DOES need backend enforcement (`RequirePremium("tts")` middleware on `POST /tts-config`).
- **Admin toggle for feature gates** (Phase 07) — no code change needed; admin UI already supports any gate key.

### Integration Points

- **AppearancePanel.tsx** — add one `CollapsibleSection id="tts" title="Text-to-Speech"` after the `id="sounds"` block.
- **overlay/[id]/page.tsx** — adjacent to `soundPlayerRef.current?.play()` on line 414, add `ttsPlayerRef.current?.speak(message)`. Also add state refs, config load (lines ~232), destroy effect (lines ~138).
- **overlays/[id]/preview/embed/page.tsx** — mirror the three touch points from the live overlay page (refs, config load, playback), plus the `TTS_SETTINGS_UPDATE` postMessage listener mirroring `SOUND_SETTINGS_UPDATE` (lines ~272).
- **overlays/[id]/page.tsx** — TTS settings load from config (lines ~1393), save via existing `updateConfig()` for `tts_*` fields, separate API calls for ElevenLabs key endpoints. "Copy OBS URL" and "Regenerate OBS URL" buttons in the TTS section.
- **overlay-manager `cmd/main.go`** — wire new `featuregates.Cache`, AES-GCM helper, TTS handler, JWT verifier for the `tts_token` param.
- **New file `services/overlay-manager/handlers/tts.go`** — all TTS endpoints.
- **New file `services/overlay-manager/repository/tts_config_repo.go`** — CRUD on `overlay_tts_configs`.
- **New package `shared/crypto/`** — AES-GCM Encrypt/Decrypt functions + key-loading helper + a focused unit test.
- **Migration `049_overlay_tts_configs.sql`** — new table + feature_gates row.

### Constraints From Existing Code

- `DisplaySettings` map[string]any is serialized to the **public config endpoint** — **never** put the ElevenLabs key there (D-05).
- `feature_gates` table currently used by share-service only — Plan 02 extends overlay-manager to subscribe. Pub/Sub channel name is `feature_gates_changed` (per ADR-0008).
- The existing `FeatureGateCache` lives under `services/share-service/featuregates/` — moving it to `shared/` is recommended; the ADR-0008 pitfalls around promauto duplicate registration apply.
- `shared/crypto` must register **no Prometheus metrics** (shared libs don't — each service wires its own).
- AGPL-3.0 license header must go on every new `.go` / `.ts` / `.tsx` / `.sql` file (recent commit `b499543a` established this).
- DB migrations must include `ON CONFLICT DO NOTHING` for the `feature_gates` INSERT (idempotent re-run during migration replays).

</code_context>

<specifics>
## Specific Ideas

- Issue #270's "stored in JSONB, never logged" clause is **factually wrong** for this codebase — the public config endpoint serves `display_settings` unauthenticated. D-05..D-17 override that part of the issue. All other issue #270 details (field names, defaults, throttling algorithm, priority event list) are preserved.
- The `shared/crypto` AES-GCM package is strictly necessary for this phase, but is also genuinely useful project-wide. Building it right (simple API, tested, ADR-documented) closes the CLAUDE.md tech-debt item.
- The single "Text-to-Speech" collapsible with inline sub-headers (not multiple collapsibles, not tabs) keeps the appearance panel visually consistent. The trade-off is a longer scroll inside the section — acceptable because TTS settings are usually configured once.
- "Test voice" button with a fixed sample is intentional — the split-view preview already lets streamers hear TTS in real chat flow. The explicit button is for tuning voice/rate/pitch without waiting for a chat message.
- Event-type prefixes (`"{display_name} raided with N viewers"`) are **more valuable than the raw message text** for subs/raids/bits — the announcement IS the message. Copy quality matters; refine the exact strings during implementation.
- Priority events never interrupt mid-word (D-33). A sub notification that cuts off mid-sentence of a regular message feels jarring. Dropping the oldest non-priority queued message preserves smoothness.
- Session-wide fallback on ElevenLabs failure (D-38) is a deliberate simplification — retrying per-error would be more robust but adds code/complexity and users typically don't want audio glitches; falling back to Web Speech for the rest of the session is less disruptive than repeated stutters.
- The per-overlay JWT signing secret (not a per-user secret) means regenerating the OBS URL on one overlay doesn't affect the user's other overlays — granular revocation.
- Voice URI persistence across browsers (D-28) will bite users who switch OBS installs. Accept it; log a warning and fall back. Not worth the complexity of a voice-matching algorithm in v1.

</specifics>

<deferred>
## Deferred Ideas

**Inherited from issue #270's "Out of scope" list — preserve for roadmap backlog:**
- Mod-flagged message TTS (chat `/highlight` command triggers TTS bypass)
- Server-side TTS audio caching for frequently used phrases (e.g., common sub announcement templates)
- Per-viewer TTS opt-out (chat command that suppresses TTS for a specific user, viewer-owned not streamer-owned)
- Custom voice cloning via ElevenLabs (users upload samples, ElevenLabs clones voice)

**Emerged during this discussion:**
- **Preset templates** (Quiet / Chatty / Priority-only bundles) — declined for v1 in favor of safe defaults. Add if users ask.
- **Per-language voice routing** for multilingual chat — declined; single user-picked voice. Revisit if international streamers complain.
- **Auto-detect message language via `franc` or similar** — same as above; JS bundle cost not worth the inconsistency.
- **Per-error fallback policy** (different strategy for 401 vs 429 vs 5xx on ElevenLabs) — declined in favor of session-wide fallback. Revisit if users report frustration with noisy toasts or dropped messages.
- **Short-lived auto-rotating tokens with websocket push** for the OBS overlay auth — declined; "Regenerate OBS URL" button is enough.
- **Hardcoded voice list fallback** when a user has no saved key yet — declined; voice dropdown is simply empty until a key is saved.
- **TTS enabled/disabled per-source** (currently per-platform) — declined; platform-level granularity is sufficient. Per-source was not in issue #270 either.
- **Character-quota polling / notifications when low** — declined; only shown after Test-Key click.

</deferred>

---

*Phase: 13-text-to-speech-tts-for-chat-messages*
*Context gathered: 2026-04-23*
