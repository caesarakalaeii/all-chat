# Phase 13: Text-to-Speech (TTS) for chat messages - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 13-text-to-speech-tts-for-chat-messages
**Areas discussed:** Phase scope, ElevenLabs API key storage, Settings UI organization, Content formatting & voice behavior

---

## Phase scope

### What ships in Phase 13?

| Option | Description | Selected |
|--------|-------------|----------|
| Both tiers (Web Speech + ElevenLabs) | Single phase, full feature as issue #270 describes | ✓ |
| Web Speech tier only, ElevenLabs as Phase 14 | Smaller phase, follow-up for premium tier | |
| Web Speech only, ElevenLabs deferred indefinitely | Phase 13 ships free tier only; ElevenLabs stays on roadmap backlog | |

**User's choice:** Both tiers (Recommended)

### Where does the premium paywall sit?

| Option | Description | Selected |
|--------|-------------|----------|
| TTS feature-wide: all of TTS is premium | Whole feature gated behind is_premium | |
| ElevenLabs tier is premium, Web Speech is free | Free users get Web Speech; premium unlocks ElevenLabs | ✓ |
| All TTS free, ElevenLabs tier requires key | No paywall; any user with a key can use that tier | |

**User's choice:** ElevenLabs tier premium, Web Speech free (Recommended)

### Should TTS be wrapped in a feature_gates toggle?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, ship gated as premium experimental | Register 'tts' in feature_gates table as is_premium=true; admin can flip | ✓ |
| No, use direct is_premium check only | Simpler wiring, no feature_gates row | |
| No, TTS is entirely free, no gate at all | Only valid if 'All free' was picked; otherwise inconsistent | |

**User's choice:** Yes, feature_gates entry (Recommended)

### Plan count for Phase 13?

| Option | Description | Selected |
|--------|-------------|----------|
| 2 plans: core queue + Web Speech, then ElevenLabs + UI | Mirror Phase 11/12's 2-plan shape | (initially) |
| 3 plans: utility, Web Speech wiring, ElevenLabs+UI | Finer-grained | |
| Let the planner decide | Don't lock plan count | |

**User's choice (initial):** 2 plans (Recommended)

### Re-asked after ElevenLabs storage decisions grew scope

| Option | Description | Selected |
|--------|-------------|----------|
| Keep 2 plans — planner splits at natural boundary | Plan 02 is large but coherent | |
| Split Plan 02 into two plans | Plan 01 = Web Speech; Plan 02 = backend; Plan 03 = ElevenLabs UI | ✓ |
| Planner decides | Don't lock plan count | |

**User's choice (revised):** Split Plan 02 — 3 plans total

**Notes:** User confirmed the expanded scope after the backend-proxy architecture was chosen. Plan boundaries locked at 3.

---

## ElevenLabs API key storage

### Where does the ElevenLabs API key live?

| Option | Description | Selected |
|--------|-------------|----------|
| localStorage only, URL fragment for OBS | Dashboard localStorage + /overlay/{id}#tts_key=XXX fragment for OBS | |
| Plaintext in display_settings | Matches issue #270's literal text; leaks via public endpoint | |
| New private, auth-required API + token URL param for OBS | Backend-stored key, JWT-authenticated overlay URL | ✓ |
| AES-GCM encrypted in display_settings with user-derived key | Encrypted blob; decryption key location problematic | |

**User's choice:** New private, auth-required API + token URL param for OBS

**Notes:** This overrides issue #270's "stored in JSONB" clause for security reasons. The public config endpoint would otherwise leak the key.

### How is the OBS URL with token provided?

| Option | Description | Selected |
|--------|-------------|----------|
| 'Copy OBS URL with key' button in dashboard | Explicit button that copies /overlay/{id}?tts_token=XXX | ✓ |
| Auto-build URL when key is entered, show inline | Readonly field auto-updates below API key input | |
| Dialog with instructions + QR code | Modal with step-by-step OBS setup | |

**User's choice:** 'Copy OBS URL with key' button in dashboard (Recommended)

### How does the 'Test Key' flow work?

| Option | Description | Selected |
|--------|-------------|----------|
| Test button pings /v1/user + plays sample voice | Validate key + synthesize 2-second sample | ✓ |
| Test button validates key only (no sample) | /v1/user ping only; quota untouched | |
| No Test button — errors surface on first real TTS | Minimal UI; delays feedback | |

**User's choice:** Test button pings /v1/user + plays sample voice (Recommended)

### How/when is the ElevenLabs voice list fetched?

| Option | Description | Selected |
|--------|-------------|----------|
| On dashboard load when key is present | Fetch /v1/voices once at settings page load | |
| On demand — fetched only when dropdown opens | Defer until voice dropdown is clicked | ✓ |
| Hardcoded list of common voices with manual ID entry | No live API fetch | |

**User's choice:** On demand

### How does the auth-required TTS API actually work?

| Option | Description | Selected |
|--------|-------------|----------|
| Backend proxies ElevenLabs calls — key never leaves server | Overlay calls /api/v1/overlays/{id}/tts; backend forwards | ✓ |
| Auth API returns the key to the browser | Overlay fetches key, then calls ElevenLabs directly | |
| Backend signs per-request ElevenLabs URLs | ElevenLabs has no URL-signing; rejected | |

**User's choice:** Backend proxies ElevenLabs calls (Recommended)

### Where is the key stored server-side?

| Option | Description | Selected |
|--------|-------------|----------|
| Encrypted (AES-GCM) in new overlay_tts_configs table | Separate table, AES-GCM at rest | ✓ |
| Encrypted in new private column on overlay_configs | Mix public + private in one table | |
| Plaintext in new overlay_tts_configs table | Matches 'basic token encryption' tech-debt; defer AES-GCM | |

**User's choice:** Encrypted in new overlay_tts_configs table (Recommended)

### How does the OBS overlay authenticate to the TTS proxy?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-overlay signed token (JWT) embedded in OBS URL | Long-lived JWT scoped to overlay_id + tts:use | ✓ |
| Per-user session token (existing auth) | Requires streamer to be logged in in OBS browser source | |
| HMAC-signed URL (no JWT, just a signature) | Stateless; revocation = rotate secret | |

**User's choice:** Per-overlay signed JWT (Recommended)

### How does the user revoke or rotate a token?

| Option | Description | Selected |
|--------|-------------|----------|
| 'Regenerate OBS URL' button in dashboard | Single button rotates signing secret, invalidates old tokens | ✓ |
| Token auto-expires + dashboard auto-rotates daily | Requires websocket/SSE push to OBS | |
| Token never expires, no rotation UI | Revoke by deleting key entry | |

**User's choice:** 'Regenerate OBS URL' button (Recommended)

---

## Settings UI organization

### How should ~20 TTS settings be organized?

| Option | Description | Selected |
|--------|-------------|----------|
| Single CollapsibleSection with inline sub-sections | One section with 'Voice', 'Throttling', 'Content', 'Priority' inline sub-headers | ✓ |
| Two CollapsibleSections: 'Text-to-Speech' + 'TTS Advanced' | Basic + Advanced split | |
| Single section with progressive disclosure toggle | 'Show advanced settings' toggle reveals extras | |
| Tabs inside a single CollapsibleSection | Basic/Throttling/Content/Priority tabs | |

**User's choice:** Single section with inline sub-sections (Recommended)

### Default TTS mode for a first-time user?

| Option | Description | Selected |
|--------|-------------|----------|
| Safe defaults from issue #270 | sample_rate=0.25, cooldown=30s, rate_limit=8/min | ✓ |
| Preset templates (Quiet / Chatty / Priority-only) | Three preset buttons that apply tuned bundles | |
| All settings off/zero until user configures | First enable does nothing meaningful | |

**User's choice:** Safe defaults from issue #270 (Recommended)

### TTS preview / test button?

| Option | Description | Selected |
|--------|-------------|----------|
| 'Test voice' button speaks a fixed sample message | "Hello, this is how your chat will sound." | ✓ |
| Test button + 'Custom preview text' input field | User can type sample text for specific pronunciation | |
| No test button — use mock messages in split-view | Reuse existing preview | |

**User's choice:** 'Test voice' with fixed sample (Recommended)

### Editor → embed preview live update?

| Option | Description | Selected |
|--------|-------------|----------|
| TTS_SETTINGS_UPDATE postMessage (mirror Phase 12) | Same pattern as SOUND_SETTINGS_UPDATE | ✓ |
| Save-to-server on change then iframe reloads config | Backend churn on every slider tick | |
| Debounced postMessage (250ms) | Smoother but adds complexity | |

**User's choice:** TTS_SETTINGS_UPDATE postMessage (Recommended)

---

## Content formatting & voice behavior

### How should the username be spoken?

| Option | Description | Selected |
|--------|-------------|----------|
| '{display_name} says: {message}' when tts_read_username=true | Uses display_name; 'says' connector | ✓ |
| '{display_name}:' (colon pause) instead of 'says' | Shorter audio, less robotic | |
| Platform-prefixed: 'Twitch: {display_name} says: ...' | Audio cue indicates source platform | |

**User's choice:** '{display_name} says: {message}' (Recommended)

### Multilingual chat handling?

| Option | Description | Selected |
|--------|-------------|----------|
| One user-picked voice for all messages | Simple, predictable; non-Latin may sound garbled | ✓ |
| Auto-detect language per message, pick best voice | Adds franc JS dep; unreliable for short msgs | |
| Voice per language picker (2-3 languages) | Middle ground; bigger UI | |

**User's choice:** One user-picked voice (Recommended)

### Priority event + queue full behavior?

| Option | Description | Selected |
|--------|-------------|----------|
| Drop oldest non-priority, enqueue priority (no interrupt) | Current utterance finishes naturally | ✓ |
| Cancel current utterance, speak priority immediately | speechSynthesis.cancel() + jarring cut-off | |
| Queue priority at front but respect current utterance | Current finishes, priority speaks next; queue grows past max | |

**User's choice:** Drop oldest non-priority, no interrupt (Recommended)

### ElevenLabs mid-session failure?

| Option | Description | Selected |
|--------|-------------|----------|
| Fallback to Web Speech for the session + one-time toast | Simple, robust, no toast spam | ✓ |
| Per-error fallback: 401/403 permanent, 429/5xx retry with backoff | More robust; more code | |
| Silent fallback, no toast | Simplest; streamer never learns key is bad | |

**User's choice:** Session-wide fallback with one-time toast (Recommended)

### How are emote tokens handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Strip all emote tokens, skip msg if >50% emotes | Clean audio; aligns with tts_skip_emote_only=true | ✓ |
| Strip emotes, speak remainder even if short | Risk of reading 'hi' repeatedly during emote storms | |
| Speak emote names as words | Full vocalization; audio spam | |

**User's choice:** Strip + skip when >50% (Recommended)

### Default platform enablement?

| Option | Description | Selected |
|--------|-------------|----------|
| All 5 platforms | Default tts_enabled_platforms = [twitch, youtube, kick, tiktok, discord] | ✓ |
| Twitch + YouTube + Kick only (exclude TikTok + Discord) | Exclude noisy defaults | |
| Twitch only, user adds others | Conservative; opt-in | |

**User's choice:** All 5 platforms (Recommended)

### TTS for event messages?

| Option | Description | Selected |
|--------|-------------|----------|
| Only priority event types speak, with event-type prefix | Subs/raids/bits/superchats with specific prefixes | ✓ |
| All event types speak with prefix | Follows/likes/channel-points included; noisy | |
| No event announcements — only chat text | Events speak only message body | |

**User's choice:** Priority events only with event-type prefix (Recommended)

### How are URLs handled?

| Option | Description | Selected |
|--------|-------------|----------|
| Replace URL with spoken word 'link', skip msg if only URL | Clean pronunciation; respects tts_skip_links=true | ✓ |
| Speak URLs literally | 'h-t-t-p-s colon slash slash' — rejected | |
| Strip URLs silently, speak remainder | Simpler than 'link' substitution | |

**User's choice:** Replace URL with 'link', skip if only URL (Recommended)

---

## Claude's Discretion

Items where the user said "you decide" or deferred to Claude:
- Nonce strategy for AES-GCM (random 12-byte is the default)
- Whether to export a simple or rotation-capable crypto API
- Feature-gate middleware: move to `shared/` or duplicate in overlay-manager
- Streaming audio decode (blob URL vs AudioContext chunks)
- Exact UI microcopy ("Copy OBS URL" etc.)
- Character-quota display formatting
- Server-side rate-limit bucket size for the TTS proxy
- Whether to write ADR-0012 for AES-GCM (recommended: yes)
- Exact event-type prefix wording

## Deferred Ideas

Ideas mentioned or that surfaced for future phases (see CONTEXT.md `<deferred>` section for the full list):
- Mod-flagged message TTS (`/highlight` command)
- Server-side TTS audio caching
- Per-viewer TTS opt-out
- Custom voice cloning via ElevenLabs
- Preset templates (Quiet/Chatty/Priority-only)
- Per-language voice routing
- Auto-detect message language
- Per-error fallback policy
- Short-lived auto-rotating tokens
- Character-quota polling
- Per-source (not per-platform) TTS toggles
