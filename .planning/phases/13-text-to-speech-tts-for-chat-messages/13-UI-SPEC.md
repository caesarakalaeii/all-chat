---
phase: 13
slug: text-to-speech-tts-for-chat-messages
status: draft
shadcn_initialized: true
preset: base-nova
created: 2026-04-23
---

# Phase 13 — UI Design Contract: Text-to-Speech

> Visual and interaction contract for the Text-to-Speech section in the overlay editor's Appearance panel. Consumed by the planner (to generate tasks) and the executor (to implement). Verified by gsd-ui-checker.

Scope: a single `CollapsibleSection id="tts" title="Text-to-Speech"` in `AppearancePanel` containing one `TTSGroup` React component with five inline sub-sections (Voice, Throttling, Content, Priority, Advanced). The Advanced sub-section hosts the premium ElevenLabs configuration (API key, voice picker, Test-Key, Copy OBS URL, Regenerate URL, character quota). All 20 `tts_*` display-settings fields surface here; all locked decisions D-01 … D-42 are honoured.

Everything in this phase reuses existing design tokens and component primitives. No new tokens, no new shadcn blocks, no new icons beyond what `lucide-react` already provides. The TTS section should be visually indistinguishable in density and rhythm from the existing `SoundGroup` and `FilterGroup` sections.

---

## Design System

| Property | Value |
|----------|-------|
| Tool | shadcn (initialized) |
| Preset | base-nova (detected in `frontend/components.json`) |
| Component library | @base-ui/react 1.3 + shadcn primitives |
| Icon library | lucide-react 0.563 |
| Font | Inter (system-ui fallback) — already active via `--font-sans` |
| Toast | react-hot-toast via `components/admin/ToastProvider.tsx` — already mounted |
| State contract | `DisplaySettings` JSONB merge (Phase 11/12 pattern); ElevenLabs secret in `overlay_tts_configs` table |

Design system reference: `frontend/DESIGN_SYSTEM.md` (slate palette, accent gradient, Inter, even-number spacing, 3-size typography limit per page).

---

## Spacing Scale

Declared values (all Tailwind defaults, multiples of 4):

| Token | Value | Usage in TTS section |
|-------|-------|----------------------|
| xs | 4px (`p-1`, `gap-1`) | Chip internal padding |
| sm | 8px (`p-2`, `gap-2`) | Chip rows, icon-to-label gap in headers |
| md | 16px (`space-y-4`, `p-4`) | Default gap between controls inside a sub-section (matches `SoundGroup` `space-y-4`) |
| lg | 24px (`space-y-6`, `pt-6`) | Gap between sub-sections (after the divider) |
| xl | 32px (`pb-8`) | Bottom padding of the whole collapsible block |

Exceptions: **none**. Reuse exactly the spacing classes present in `SoundGroup.tsx` and `FilterGroup.tsx`.

---

## Typography

| Role | Size | Weight | Line Height | Tailwind class (existing) |
|------|------|--------|-------------|---------------------------|
| Section trigger | 14px | 500 | 1.4 | `text-sm font-medium text-text` (inside `CollapsibleSection.Trigger`) |
| Sub-section header (small caps) | 12px | 600 | 1.3 | `text-xs font-semibold uppercase tracking-wide text-text-dim` |
| Control label | 14px | 400 | 1.4 | `text-sm text-text-sub` (matches `SliderControl`, `ToggleSwitch`) |
| Helper/caption | 12px | 400 | 1.4 | `text-xs text-text-dim` |
| Input text | 14px | 400 | 1.4 | `text-sm text-text` |
| API-key input | 14px | 400 | 1.4 | `text-sm text-text font-mono` (only monospaced exception) |
| Inline error | 12px | 500 | 1.4 | `text-xs font-medium text-red-400` |

Rule: sub-section headers `Voice / Throttling / Content / Priority / Advanced` are rendered as small-caps uppercase labels preceded by a 1px border top divider (`border-t border-border pt-4 mt-4`). First sub-section (Voice) has no top divider.

---

## Color

| Role | Value | Usage |
|------|-------|-------|
| Dominant (60%) | `--surface` (slate-900 / `#0f172a`) | Section background, panel interior |
| Secondary (30%) | `--surface-alt` (slate-800 / `#1e293b`) | Chip pills, dropdown background, `type=password` input background |
| Accent (10%) | `--twitch` (`#9146ff`) | Active toggle state, primary-CTA hover ring, active chip border — same as rest of Appearance panel |
| Destructive | `red-500` (`#ef4444`) | Inline error text, "Regenerate URL" confirmation button, PremiumBadge error state |
| Premium marker | `purple-500` (`#a855f7`) | `<PremiumBadge />` glyph (existing) |
| Success toast | `#10b981` (green-500) | Toast background for `OBS URL copied`, `New OBS URL copied`, `API key saved` (matches `ToastProvider`) |
| Error toast | `#ef4444` (red-500) | Toast background for D-38/D-39 failures (matches `ToastProvider`) |

Accent reserved for:
- Toggle switches in the enabled state (D-24 `tts_enabled`, `tts_read_username`, etc.)
- The `Test voice` button hover state
- The active-chip outline on `tts_enabled_platforms` pills
- Focus ring on the API-key input when premium

No element gets accent purely for decoration. No accent-on-accent. No gradient inside the TTS section.

---

## Copywriting Contract

All copy is English-only (D-27 single user-picked voice; multilingual is an ElevenLabs-model concern, not a UI concern). No emoji.

### Sub-section headers

| Header | Copy |
|--------|------|
| Voice | `VOICE` |
| Throttling | `THROTTLING` |
| Content | `CONTENT` |
| Priority | `PRIORITY` |
| Advanced | `ADVANCED (ELEVENLABS)` |

### Control labels

| Field (DisplaySettings key) | Label copy | Helper (if any) |
|-----------------------------|------------|-----------------|
| `tts_enabled` | `Enable text-to-speech` | — |
| `tts_provider` | `Voice provider` | — (radio: `Browser (free)` / `ElevenLabs (premium)`) |
| `tts_volume` | `Volume` | — |
| `tts_voice_uri` | `Voice` | `Browser voice — list depends on your OS/browser.` |
| `tts_rate` | `Speech rate` | — |
| `tts_pitch` | `Pitch` | — |
| `tts_filter_mode` | `Which messages are spoken` | — (radio: `All` / `Sample` / `Priority-only`) |
| `tts_sample_rate` | `Sample rate` | `Chance a non-priority message is spoken.` (only visible when `tts_filter_mode=sample`) |
| `tts_max_queue` | `Max queue length` | — |
| `tts_messages_per_minute` | `Messages per minute` | — |
| `tts_user_cooldown_seconds` | `Per-user cooldown` | — |
| `tts_staleness_seconds` | `Drop messages older than` | — |
| `tts_priority_events` | `Announce priority events` | `Subs, raids, bits, super chats, donations.` |
| `tts_priority_bits_min` | `Minimum bits to announce` | `0 = announce all bits.` |
| `tts_read_username` | `Read username` | — |
| `tts_read_platform` | `Read platform name` | — |
| `tts_max_message_chars` | `Max message length` | — |
| `tts_skip_emote_only` | `Skip emote-only messages` | — |
| `tts_skip_links` | `Skip link-only messages` | — |
| `tts_enabled_platforms` | `Platforms` | — (chip list) |

### Buttons

| Button | Idle label | Active label | Placement |
|--------|-----------|--------------|-----------|
| Test voice (Browser + ElevenLabs) | `Test voice` | `Stop` | Voice sub-section, right of the voice dropdown |
| Save API key | `Save key` | `Saving…` | Advanced sub-section, right of the API-key input |
| Test API key | `Test key` | `Testing…` | Advanced sub-section, below the Save-key row (only visible after a key is saved) |
| Copy OBS URL | `Copy OBS URL` | `Copied` (toast fires; button label does NOT change — the toast is the feedback) | Advanced sub-section, OBS URL row |
| Regenerate URL | `Regenerate URL` | `Regenerating…` | Advanced sub-section, OBS URL row, right of Copy button |
| Remove key | `Remove key` | `Removing…` | Advanced sub-section, small ghost button below quota row (only visible after a key is saved) |

### Status and helper copy

| Element | Copy |
|---------|------|
| Empty voice dropdown while loading | `Loading voices…` (disabled `<select>` with this as a placeholder option) |
| Voice dropdown when browser has zero voices | `No voices available in this browser.` (disabled select; link to known-limitation tooltip) |
| Voice URI not in list (load-time) | Silent fallback; `console.warn` only (D-28). No UI toast. |
| No saved ElevenLabs config | `You need to add your ElevenLabs API key.` (shown as inline helper below the provider radio when `tts_provider=elevenlabs`, no config saved) |
| API-key placeholder | `sk-...` |
| API-key helper (non-premium) | `Upgrade to Premium to use ElevenLabs voices.` |
| API-key helper (premium, saved) | `Key saved and encrypted. Click Test key to verify.` |
| API-key helper (premium, unsaved) | `Your key is encrypted server-side and never returned.` |
| Quota display format | `{N} / {M} characters this month ({P}%)` — N and M comma-separated integers, P integer 0-100. Example: `8,432 / 10,000 characters this month (84%)`. |
| Quota not yet fetched | `Click Test key to see your remaining quota.` (after key saved, before first test) |
| Quota loading | `Checking…` |
| OBS URL display | Read-only `<input>` filled with `https://allch.at/overlay/{id}?tts_token={jwt}` (full JWT visible — this is a bearer by design, user can screenshot it; rotation is the recovery) |
| OBS URL helper | `Paste this URL into OBS as your browser source to enable ElevenLabs TTS.` |
| Regenerate confirmation title | `Regenerate OBS URL?` |
| Regenerate confirmation body | `This invalidates the current OBS URL. You'll need to paste the new URL into OBS.` |
| Regenerate confirmation CTA | `Regenerate URL` (destructive styling) |
| Regenerate confirmation cancel | `Cancel` |

### Toasts

Toast copy is **identical verbatim** to D-38/D-39 — do not paraphrase.

| Trigger | Variant | Copy |
|---------|---------|------|
| Mid-session ElevenLabs failure (D-38) | info (once per session) | `ElevenLabs unavailable — using browser voice.` |
| Test-key 401 (D-39) | error | `Invalid API key` |
| Test-key 429 (D-39) | error | `Rate-limited — try again in a minute` |
| Test-key 5xx (D-39) | error | `ElevenLabs service unavailable` |
| Test-key network error | error | `Could not reach ElevenLabs. Check your connection.` |
| Save key success | success | `API key saved.` |
| Save key failure (server) | error | `Could not save key: {server message}` |
| Remove key success | success | `API key removed.` |
| Copy OBS URL success | success | `OBS URL copied.` |
| Regenerate URL success | success | `New OBS URL copied to clipboard.` |
| Regenerate URL failure | error | `Could not regenerate URL. Try again.` |

### Primary CTA

The Appearance panel has no single "Save" CTA inside TTSGroup — changes persist automatically via the parent editor's existing `updateConfig()` flow (same as Phase 12). The page-level `Save Configuration` button in `overlays/[id]/page.tsx` remains the primary CTA for display_settings. The Advanced sub-section's `Save key` button is the primary CTA **of that block**, because ElevenLabs secrets save to a separate endpoint, not through `display_settings`.

### Destructive actions

| Action | Confirmation required? | Confirmation copy |
|--------|------------------------|-------------------|
| Regenerate OBS URL | Yes (modal) | See `Regenerate confirmation` above |
| Remove API key | Yes (inline confirm: second click of the button within 3s confirms; otherwise cancels) | Button label toggles to `Confirm remove` for 3s; second click deletes. Avoids a second modal. |
| Disable TTS (toggle off) | No | Non-destructive; just stops speaking — state preserved |

### Empty / error / loading states

| State | UX |
|-------|-----|
| `tts_enabled=false` (collapsed) | All controls below the master toggle hidden (same pattern as `SoundGroup`). |
| Browser has no Web Speech API (`window.speechSynthesis` absent) | Master toggle disabled; helper text: `This browser does not support text-to-speech.` |
| Non-premium user, `tts_provider=elevenlabs` selected | Advanced block rendered but all inputs disabled; overlay of `<PremiumBadge />` + copy `Upgrade to Premium to use ElevenLabs voices.` |
| Premium user, no saved key, voice picker opened | Show `Save your API key to load voices.` in the dropdown — not an error, instructional. |
| Premium user, saved key, voice list fetch fails | Inline error under voice picker: `Could not load voices.` + retry link. No session-wide fallback (this is setup, not runtime). |

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| shadcn official (base-nova) | none net-new — TTS section composes existing in-tree primitives (`ToggleSwitch`, `SliderControl`, `CollapsibleSection`, `PremiumBadge`) | not required |
| Third-party registries | none | not applicable |

No third-party shadcn block is pulled in. No `npx shadcn add` invocation is planned. The planner should call out this fact explicitly so the checker does not look for a registry vetting trail.

---

## Component Inventory

All net-new components introduced by Phase 13. Reused components are listed separately to show the composition surface.

### Net-new

| Component | Path | Purpose | Props |
|-----------|------|---------|-------|
| `TTSGroup` | `frontend/src/components/appearance/TTSGroup.tsx` | Root group component hosting all 5 sub-sections. Analog of `SoundGroup`. | `{ displaySettings: Partial<DisplaySettings>; onChange: (patch: Partial<DisplaySettings>) => void; isPremium: boolean; overlayId: string; hasElevenLabsConfig: boolean; obsUrl?: string; onPreview?: () => void; onPreviewStop?: () => void; onSaveKey: (key: string, voiceId: string) => Promise<void>; onTestKey: () => Promise<{ok: boolean; charactersRemaining?: number; charactersLimit?: number; errorCode?: number}>; onRotateToken: () => Promise<{obsUrl: string}>; onRemoveKey: () => Promise<void>; onFetchVoices: () => Promise<ElevenLabsVoice[]>; }` |
| `VoicePicker` (internal to `TTSGroup.tsx` or extracted) | same file or `frontend/src/components/appearance/tts/VoicePicker.tsx` | Dropdown that lists Web Speech voices (from `useBrowserVoices()` hook) OR ElevenLabs voices (lazy-fetched on open, per D-14). Shows skeleton shimmer while loading. | `{ provider: 'browser' \| 'elevenlabs'; selected?: string; onChange: (uri: string) => void; onOpen: () => void; elevenLabsVoices?: ElevenLabsVoice[]; loading: boolean; disabled: boolean; }` |
| `ApiKeyInput` (internal) | same file or `frontend/src/components/appearance/tts/ApiKeyInput.tsx` | Password-typed masked input with Save button; reveals Test button after a successful save. | `{ hasSavedKey: boolean; onSave: (key: string) => Promise<void>; onRemove: () => Promise<void>; disabled: boolean; }` |
| `QuotaDisplay` (internal) | same file | Text-only quota row; shown only after a successful `onTestKey` call returns. | `{ charactersRemaining?: number; charactersLimit?: number; }` |
| `ObsUrlPanel` (internal) | same file | Read-only input with Copy + Regenerate buttons + confirmation modal. | `{ obsUrl: string; onCopy: () => Promise<void>; onRegenerate: () => Promise<void>; }` |
| `PlatformChipRow` (internal) | same file | Chip-style multi-select for `tts_enabled_platforms`. | `{ platforms: string[]; onToggle: (p: string) => void; }` |
| `RegenerateConfirmDialog` (internal, `@base-ui/react/alert-dialog`-backed) | same file | Modal confirming OBS URL rotation. | `{ open: boolean; onConfirm: () => void; onCancel: () => void; }` |
| `useBrowserVoices` (hook) | `frontend/src/lib/hooks/useBrowserVoices.ts` | Returns `SpeechSynthesisVoice[]`, subscribes to `voiceschanged`, handles Chromium async load (Pitfall #1 from RESEARCH). | `() => SpeechSynthesisVoice[]` |

### Reused (no change)

| Component | Path | Role |
|-----------|------|------|
| `CollapsibleSection` | `frontend/src/components/appearance/CollapsibleSection.tsx` | Wraps `TTSGroup` with `id="tts" title="Text-to-Speech"` inside `AppearancePanel` |
| `ToggleSwitch` | `frontend/src/components/appearance/ToggleSwitch.tsx` | All boolean `tts_*` fields |
| `SliderControl` | `frontend/src/components/appearance/SliderControl.tsx` | `tts_volume`, `tts_rate`, `tts_pitch`, `tts_sample_rate` |
| `PremiumBadge` | `frontend/src/components/PremiumBadge.tsx` | Inline decorator on Advanced block for non-premium users |
| `react-hot-toast` `toast.success/error` | via existing `ToastProvider` | All toasts |

### Component tree (inside `CollapsibleSection id="tts"`)

```
<TTSGroup>
├── ToggleSwitch "Enable text-to-speech"   (tts_enabled)
│
└── (when tts_enabled === true)
    ├── --- VOICE header ---
    │   ├── Radio group "Voice provider"       (tts_provider)
    │   │     └── "Browser (free)" | "ElevenLabs (premium)" (second disabled for non-premium — inline helper appears)
    │   ├── SliderControl "Volume"             (tts_volume)
    │   ├── VoicePicker                        (tts_voice_uri when provider=browser; ElevenLabs voice_id when provider=elevenlabs — the latter lives in Advanced block NOT here)
    │   ├── SliderControl "Speech rate"        (tts_rate)
    │   ├── SliderControl "Pitch"              (tts_pitch)           [hidden when provider=elevenlabs — not supported]
    │   └── Button "Test voice" / "Stop"       (onPreview / onPreviewStop)
    │
    ├── --- THROTTLING header ---
    │   ├── Radio group "Which messages are spoken"  (tts_filter_mode)
    │   ├── SliderControl "Sample rate"              (tts_sample_rate) [visible only when tts_filter_mode=sample]
    │   ├── NumberControl "Max queue length"         (tts_max_queue)
    │   ├── NumberControl "Messages per minute"      (tts_messages_per_minute)
    │   ├── NumberControl "Per-user cooldown"        (tts_user_cooldown_seconds)  — unit " s"
    │   └── NumberControl "Drop messages older than" (tts_staleness_seconds)      — unit " s"
    │
    ├── --- CONTENT header ---
    │   ├── ToggleSwitch "Read username"              (tts_read_username)
    │   ├── ToggleSwitch "Read platform name"         (tts_read_platform)
    │   ├── NumberControl "Max message length"        (tts_max_message_chars) — unit " chars"
    │   ├── ToggleSwitch "Skip emote-only messages"   (tts_skip_emote_only)
    │   ├── ToggleSwitch "Skip link-only messages"    (tts_skip_links)
    │   └── PlatformChipRow (5 chips)                 (tts_enabled_platforms)
    │
    ├── --- PRIORITY header ---
    │   ├── ToggleSwitch "Announce priority events"   (tts_priority_events)
    │   └── NumberControl "Minimum bits to announce"  (tts_priority_bits_min)     [visible only when tts_priority_events=true]
    │
    └── --- ADVANCED (ELEVENLABS) header ---
        (rendered only when tts_provider === 'elevenlabs')
        (entire block wrapped in PremiumBadge-decorated disabled state for non-premium)
        ├── ApiKeyInput (password field + Save / Remove buttons)
        ├── VoicePicker (ElevenLabs voices, lazy on open — select voice_id)
        ├── Button "Test key" (enabled only after Save)
        ├── QuotaDisplay (shown only after a successful Test key click)
        └── ObsUrlPanel
              ├── readonly <input> (OBS URL)
              ├── Button "Copy OBS URL"
              └── Button "Regenerate URL" (opens confirmation modal)
```

**`NumberControl`**: a tiny wrapper around an `<input type="number">` with the same label/value/unit layout as `SliderControl`. Introduce it inline in `TTSGroup.tsx` for number-only fields (queue, MPM, cooldown, staleness, max_message_chars, priority_bits_min, max_message_chars). It is not a generically-reused primitive; treat it as `TTSGroup`-local.

---

## State Machine per Interaction Flow

### Flow 1 — First enable of TTS (provider=browser by default)

```
Initial: tts_enabled=false, all controls hidden
  │
  │  user clicks ToggleSwitch "Enable text-to-speech"
  ▼
tts_enabled=true (optimistic), patch sent via onChange → editor state → postMessage TTS_SETTINGS_UPDATE → embed iframe receives → ttsPlayer instantiated with provider=browser
  │
  │  provider defaults to 'browser'
  │  voice list: useBrowserVoices() subscribes; may be empty on first tick in Chromium (Pitfall #1)
  │
  ├── IF voices.length === 0 AT RENDER TIME:
  │       VoicePicker shows disabled <select> with one placeholder option "Loading voices…"
  │       voiceschanged fires → voices populate → VoicePicker re-renders with full list
  │
  ├── IF saved tts_voice_uri NOT in list on mount:
  │       console.warn (D-28), no toast, no UI change; dropdown stays on its own default
  │
  └── user is now free to pick a voice, adjust rate/pitch/volume, and hit "Test voice"
```

### Flow 2 — Test voice (Browser)

```
idle
  │
  │  click Test voice
  ▼
speaking
  │   synth.cancel()              (Pitfall #10 — clear any overlap)
  │   u = new SpeechSynthesisUtterance("Hello, this is how your chat will sound.")
  │   u.voice = voices.find(v => v.voiceURI === tts_voice_uri) ?? undefined
  │   u.volume = tts_volume; u.rate = tts_rate; u.pitch = tts_pitch
  │   synth.speak(u)
  │   button label → "Stop", aria-pressed=true
  │
  ├── u.onend → back to idle, aria-pressed=false
  ├── u.onerror → back to idle, console.warn, no toast (preview failure is a non-event)
  └── user clicks again (now labeled "Stop"):
         synth.cancel() → u.onend fires → back to idle
```

### Flow 3 — Test voice (ElevenLabs)

```
idle
  │
  │  click Test voice  (only available if provider=elevenlabs AND hasElevenLabsConfig=true AND voice_id set)
  ▼
speaking
  │   fetch POST /api/v1/overlays/:id/tts-config/test
  │   button label → "Stop", aria-pressed=true
  │
  ├── response.ok: body is audio/mpeg blob → new Audio(URL.createObjectURL(blob)).play()
  │     ├── audio.onended → revokeObjectURL → idle
  │     └── user clicks Stop → audio.pause() → revokeObjectURL → idle
  │
  └── response 401/429/5xx/network (D-39):
        verbose toast per D-39 table
        button → idle
        NOTE: Test-key failure does NOT trigger session fallback (D-38). This is a setup flow.
```

### Flow 4 — Switch provider Browser → ElevenLabs

```
Initial: tts_provider='browser'
  │
  │  user clicks radio "ElevenLabs (premium)"
  ▼
case A — non-premium user:
  Radio button is rendered with disabled attribute; click no-op.
  Inline helper below radio: "Upgrade to Premium to use ElevenLabs voices." (non-dismissable, always shown for non-premium)

case B — premium user, hasElevenLabsConfig=false:
  tts_provider='elevenlabs' applied optimistically (persists on next save)
  Advanced sub-section becomes visible
  Inline helper above the Save key button (yellow-amber tone): "You need to add your ElevenLabs API key."
  Scroll / focus shifts to the API-key input.

case C — premium user, hasElevenLabsConfig=true:
  tts_provider='elevenlabs' applied.
  Advanced sub-section becomes visible.
  No prompt. TTS speech will use the proxy on the next message.
```

### Flow 5 — Save API key

```
user types key, clicks "Save key"
  │
  │  button label → "Saving…", disabled=true
  ▼
POST /api/v1/overlays/:id/tts-config  { api_key, voice_id }
  │
  ├── 200 OK:
  │     hasElevenLabsConfig=true (parent sets state)
  │     toast.success("API key saved.")
  │     input value clears and placeholder becomes sk-••••••••••••  (masked display — no key is returned from server)
  │     Test key button becomes visible + enabled
  │     Save key button returns to "Save key"
  │
  ├── 402 Payment Required (premium check fail):
  │     toast.error("Premium required for ElevenLabs.")
  │     button → idle
  │     (shouldn't happen because UI gated; backend still enforces per D-11)
  │
  └── 500 or network:
        toast.error(`Could not save key: ${server.message ?? 'network error'}`)
        inline error below input: "Could not save. Try again."
        button → "Save key"
```

### Flow 6 — Test API key

```
hasElevenLabsConfig=true, user clicks "Test key"
  │
  │  button → "Testing…", disabled
  ▼
POST /api/v1/overlays/:id/tts-config/test
  │
  ├── 200 OK + audio/mpeg body + headers `x-characters-remaining`, `x-characters-limit`:
  │     play the returned sample (same as Flow 3 success branch)
  │     QuotaDisplay now rendered, values from headers
  │     toast.success("API key valid.")  — optional; the played audio is self-evident
  │
  ├── 401:  toast.error("Invalid API key")
  ├── 429:  toast.error("Rate-limited — try again in a minute")
  ├── 5xx:  toast.error("ElevenLabs service unavailable")
  └── network:  toast.error("Could not reach ElevenLabs. Check your connection.")
```

### Flow 7 — Copy OBS URL

```
click "Copy OBS URL"
  │
  ▼
navigator.clipboard.writeText(obsUrl)
  ├── success → toast.success("OBS URL copied.")
  └── permission denied / missing API →
        fallback: select the read-only input, document.execCommand('copy') → toast or alert
```

### Flow 8 — Regenerate OBS URL

```
click "Regenerate URL"
  │
  ▼
RegenerateConfirmDialog opens
  ├── user clicks "Cancel" → dialog closes, no-op
  └── user clicks "Regenerate URL" (destructive red button):
        button on confirm dialog → "Regenerating…", disabled
        POST /api/v1/overlays/:id/tts-config/rotate-token
        ├── 200 OK:
        │     new obsUrl in response → parent updates state → obsUrl input re-renders
        │     navigator.clipboard.writeText(newObsUrl)
        │     toast.success("New OBS URL copied to clipboard.")
        │     dialog closes
        └── error:
              dialog stays open
              toast.error("Could not regenerate URL. Try again.")
              button returns to enabled state
```

### Flow 9 — Remove API key

```
click "Remove key"
  │
  │  button label becomes "Confirm remove" for 3s, bgcolor turns destructive red
  │  (inline confirmation; no modal)
  │
  ├── user does nothing for 3s → button reverts to "Remove key"
  └── user clicks again within 3s:
        button → "Removing…", disabled
        DELETE /api/v1/overlays/:id/tts-config
        ├── 200 OK:
        │     hasElevenLabsConfig=false
        │     Test key / QuotaDisplay / ObsUrlPanel all hidden
        │     toast.success("API key removed.")
        │     tts_provider automatically reverts to 'browser' (optimistic; saves next cycle)
        └── error:
              toast.error("Could not remove key. Try again.")
              button → "Remove key"
```

### Flow 10 — Platform chip toggle (`tts_enabled_platforms`)

```
tts_enabled_platforms = ['twitch','youtube','kick','tiktok','discord']  (default)

render 5 chips:
  [✓ Twitch]  [✓ YouTube]  [✓ Kick]  [✓ TikTok]  [✓ Discord]

click "TikTok":
  tts_enabled_platforms = tts_enabled_platforms.filter(p => p !== 'tiktok')
  onChange patch fires → TTS_SETTINGS_UPDATE postMessage → ttsPlayer.updateSettings()
  TikTok chip render: [   TikTok] (outline-only, text-dim)

click "TikTok" again: re-adds; chip solid accent border again.
```

### Flow 11 — Premium gating (non-premium user enters TTS section)

```
non-premium user opens TTS section
  │
  ├── Voice / Throttling / Content / Priority sub-sections all fully functional (Web Speech is free)
  └── If user somehow flips tts_provider=elevenlabs (UI should disable the radio, but defensive):
        Advanced block renders with:
        - All inputs disabled (grayed, cursor-not-allowed)
        - PremiumBadge overlay on the block
        - Upsell copy "Upgrade to Premium to use ElevenLabs voices." with a link to /settings/billing (or wherever premium upgrades live — reuse Phase 12 link)
```

### Flow 12 — Feature-gate disabled entirely (`featureGate.enabled === false`)

Not applicable in this phase (TTS is registered via migration 049 and stays enabled). Documented for completeness: if `GET /feature-gates` ever returns `enabled=false` for `tts`, the parent `AppearancePanel` should simply not render the `CollapsibleSection id="tts"` (conditional guard, same pattern as `displaySettings && onSoundChange && (…)` at `AppearancePanel.tsx:89`). Planner: add the guard.

---

## Live Preview Data Flow

Mirrors Phase 12's `SOUND_SETTINGS_UPDATE` pattern exactly.

### Producer side — editor `overlays/[id]/page.tsx`

Every mutation of `displaySettings.tts_*` (via the `onChange` prop passed to `TTSGroup`) immediately triggers:

```ts
previewIframeRef.current?.contentWindow?.postMessage(
  {
    type: 'TTS_SETTINGS_UPDATE',
    ttsSettings: {
      tts_enabled, tts_provider, tts_volume,
      tts_voice_uri, tts_rate, tts_pitch,
      tts_filter_mode, tts_sample_rate,
      tts_max_queue, tts_messages_per_minute,
      tts_user_cooldown_seconds, tts_staleness_seconds,
      tts_priority_events, tts_priority_bits_min,
      tts_read_username, tts_read_platform,
      tts_max_message_chars, tts_skip_emote_only, tts_skip_links,
      tts_enabled_platforms,
    },
  },
  '*',
)
```

No debounce (D-22). Every slider tick fires.

The ElevenLabs-specific runtime data (`ttsEndpoint`, `ttsToken`, `voiceId`) does NOT ride the postMessage — it's loaded inside the embed page from `/api/v1/overlays/:id/tts-config` on mount, because it's stable across an editor session. If the user rotates the token, the embed iframe is manually refreshed (next token fetch picks it up).

### Consumer side — embed `overlays/[id]/preview/embed/page.tsx`

New listener clause added next to the existing `SOUND_SETTINGS_UPDATE` block (line ~272):

```ts
if (event.data?.type === 'TTS_SETTINGS_UPDATE') {
  const s = event.data.ttsSettings as Partial<DisplaySettings>
  const newSettings: TTSSettings = { /* project s.* onto TTSSettings shape */ }
  ttsSettingsRef.current = newSettings
  if (ttsPlayerRef.current) {
    ttsPlayerRef.current.updateSettings(newSettings)
  } else {
    ttsPlayerRef.current = createTTSPlayer(newSettings, onFallback)
  }
  return
}
```

Symmetric hooks: `ttsPlayerRef`, `ttsSettingsRef`, `onFallback` (fires the D-38 toast once), `destroy()` in the cleanup effect.

### Playback hook — same embed page + live overlay page

At the filter check sites:

- `frontend/src/app/overlays/[id]/preview/embed/page.tsx:400` — alongside `soundPlayerRef.current?.play()`:
  ```ts
  ttsPlayerRef.current?.speak(message)
  ```
- `frontend/src/app/overlay/[id]/page.tsx:414` — same adjacency.

Both fire for non-filtered messages. D-42: independent; filtered messages trigger neither.

### Priming the audio context (Pitfall #3)

Web Speech on standalone browsers may silently drop the first utterance without a prior user gesture. The `Test voice` button is the intended first-gesture affordance — explicitly document this in a small footnote near the Test voice button when `tts_provider=browser`:

> `Tip: Click Test voice once after opening the editor — browser audio may be muted until you do.`

Do not attempt any "silent priming" hack. The tip is enough.

---

## Error, Empty, and Loading States

| Context | State | UX |
|---------|-------|-----|
| Voice dropdown, provider=browser, voices not yet available | loading | disabled `<select>` with one option `Loading voices…` (no skeleton pulse — the disabled state is self-evident). Updates automatically when `voiceschanged` fires. |
| Voice dropdown, provider=browser, no voices at all (rare) | empty | disabled `<select>` with one option `No voices available in this browser.` |
| Voice dropdown, provider=elevenlabs, key not saved | empty | disabled `<select>` with one option `Save your API key to load voices.` |
| Voice dropdown, provider=elevenlabs, dropdown opened, fetch in flight | loading | `<select>` disabled; one option `Loading voices…` |
| Voice dropdown, provider=elevenlabs, fetch failed | error | `<select>` disabled; one option `Could not load voices`; small `Retry` ghost button below |
| API key input, premium user | idle | empty input with `placeholder="sk-..."` and helper `Your key is encrypted server-side and never returned.` |
| API key input, non-premium user | disabled | empty input disabled with `PremiumBadge`; helper `Upgrade to Premium to use ElevenLabs voices.` |
| Quota display, never tested | empty | helper text `Click Test key to see your remaining quota.` (no numbers) |
| Quota display, test in flight | loading | `Checking…` |
| Quota display, test succeeded | populated | `{N} / {M} characters this month ({P}%)` (see Copywriting) |
| Quota display, test failed | hidden | No quota row; error was already surfaced as a toast per D-39 |
| OBS URL input | populated | read-only, full URL visible; focus-selects-all on click |
| OBS URL input, config just created | populated | same as above; no special "new!" state |
| Test voice button, Web Speech not supported | disabled | button disabled; parent toggle `Enable text-to-speech` also disabled with helper `This browser does not support text-to-speech.` |
| Test voice button, speaking | active | label `Stop`; `aria-pressed=true` |
| Platform chip row, zero platforms selected | warning | small inline helper in `text-amber-400`: `No platforms selected — TTS is effectively off.` (not blocking; user may deliberately want a no-op state) |
| Sample rate slider | conditional | hidden when `tts_filter_mode !== 'sample'` |
| Priority bits min input | conditional | hidden when `tts_priority_events === false` |
| Pitch slider | conditional | hidden when `tts_provider === 'elevenlabs'` (ElevenLabs does not expose pitch) |

---

## Accessibility Requirements

All controls must satisfy WCAG 2.2 AA. Specifics for this section:

| Element | Requirement |
|---------|-------------|
| Master `Enable text-to-speech` toggle | `role="switch"`, `aria-checked`, keyboard-operable (Space toggles) — inherited from existing `ToggleSwitch` |
| Voice provider radio | `role="radiogroup"` on wrapper; each option `role="radio"` with `aria-checked`; arrow keys switch |
| All SliderControls | `role="slider"` inherited from `<input type="range">`; `aria-label` from `label` prop; keyboard arrow-keys move step |
| All NumberControls | `<input type="number">` with explicit `<label htmlFor>`; up/down arrows move step |
| Voice picker `<select>` | explicit `<label htmlFor>`; native keyboard nav (typing letter jumps to entry) |
| Platform chips | `role="checkbox"`, `aria-checked`, keyboard `Space` toggles; `aria-label="{Platform} — enabled"` / `... disabled` |
| `Test voice` button | `aria-pressed={speaking}`; label changes to `Stop` visually AND in accessible name |
| `Copy OBS URL` success | toast has `role="status"`, `aria-live="polite"` — inherited from `react-hot-toast` Toaster config |
| `Save key` in-band error | rendered inside a container with `role="alert"` when the error message is present |
| `Regenerate URL` confirmation modal | `@base-ui/react/alert-dialog` — traps focus, `Escape` cancels, `Enter` confirms (on `Regenerate URL` button) |
| API-key input | `type="password"`, `autocomplete="off"`, `spellCheck={false}`, `aria-label="ElevenLabs API key"`; never logged, never mirrored into state that could serialize to console |
| Read-only OBS URL input | `readOnly`, `aria-label="OBS URL — copy and paste into OBS browser source"` |
| PremiumBadge overlay | `aria-label="Premium feature"` present on the badge (existing `PremiumBadge` sets `aria-label="Premium badge"` — extend title prop with `"Premium feature"` where overlaid on disabled inputs) |
| Color contrast | All text colors (`text-text`, `text-text-sub`, `text-text-dim`) already verified AA against `--surface` background in DESIGN_SYSTEM.md |
| Focus visible | Inherited from Tailwind default; existing `focus:ring-2 focus:ring-twitch` on interactive elements |
| Keyboard traversal order | Linear top-to-bottom within the collapsible panel; no positive tabindex; no custom skip |
| Screen-reader announcements | Toasts (`role=status`), error-region alerts (`role=alert`), and the modal (`role=alertdialog`) all announce on state change |

---

## Tailwind / Styling Rules

Mirror exactly the class vocabulary already used in `SoundGroup.tsx` and `FilterGroup.tsx`. No new design tokens. Specifically:

- Root container: `<div class="space-y-4">` (matches `SoundGroup`)
- Sub-section header row: `<div class="flex items-center gap-2 border-t border-border pt-4 mt-4"> <span class="text-xs font-semibold uppercase tracking-wide text-text-dim">VOICE</span> </div>` — first header omits `border-t pt-4 mt-4`
- Control label: `<span class="text-sm text-text-sub">` (matches `SliderControl` + `ToggleSwitch`)
- Helper text: `<p class="text-xs text-text-dim">`
- Chip pill (active): `rounded-full border border-twitch bg-twitch/15 px-3 py-1 text-xs text-text`
- Chip pill (inactive): `rounded-full border border-border bg-surface-alt px-3 py-1 text-xs text-text-sub`
- Primary action button (Save key, Test key, Copy OBS URL, Regenerate URL, Test voice): `rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text hover:bg-surface-alt` (matches `SoundGroup`'s Test sound button)
- Destructive confirmation button (inside modal): use existing red-600 pattern from DESIGN_SYSTEM.md buttons section
- Password input: `rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text font-mono placeholder:text-text-dim disabled:cursor-not-allowed disabled:opacity-50` (font-mono is the only deviation from `SoundGroup`'s custom URL input)
- Read-only OBS URL input: same as password input minus `font-mono`, plus `select-all` CSS and an attached `readOnly` attribute
- Premium overlay wrap: `<div class="relative">` containing the disabled controls with `<div class="absolute inset-0 flex items-center justify-center bg-surface/80"><PremiumBadge /> + copy</div>`
- Do NOT introduce any `grid` layouts; use stacked `space-y-*` and inline `flex` rows matching existing siblings

---

## Inferred Defaults (Claude's Discretion items resolved)

These items were under "Claude's Discretion" in CONTEXT.md (D-designs 39, 23, etc.) or explicitly left to the UI researcher. All now locked for this spec:

| Item | Value | Rationale |
|------|-------|-----------|
| Character-quota format | `{N,} / {M,} characters this month ({P}%)` with integers only | Matches DESIGN_SYSTEM numeric style; no K-suffix (streamer can eyeball), no bar chart (deferred per D-23) |
| Copy OBS URL label | `Copy OBS URL` | Matches user-visible doc conventions |
| Copy OBS URL toast | `OBS URL copied.` | Period at end; terse |
| Regenerate URL label | `Regenerate URL` | Matches D-10 wording |
| Regenerate URL toast | `New OBS URL copied to clipboard.` | Signals the clipboard side-effect |
| Regenerate URL confirmation body | `This invalidates the current OBS URL. You'll need to paste the new URL into OBS.` | Explicit about both the invalidation and the required user action |
| Test voice idle label | `Test voice` | Matches D-21 wording |
| Test voice active label | `Stop` | Universal, non-localized term, short |
| Test sample text | `Hello, this is how your chat will sound.` | Verbatim from D-21 |
| Test-key button copy | `Test key` | Matches D-39 wording |
| Provider radio options | `Browser (free)` / `ElevenLabs (premium)` | Makes the cost boundary explicit |
| Sub-section order | Voice → Throttling → Content → Priority → Advanced | Verbatim from D-18 |
| Advanced header copy | `ADVANCED (ELEVENLABS)` | Signals the block is ElevenLabs-specific without reading as cryptic |
| Platform chip labels | `Twitch` / `YouTube` / `Kick` / `TikTok` / `Discord` | Platform canonical names, capitalized |
| Remove-key pattern | Two-click inline confirm (no modal) | Less friction than a modal for a small destructive action; modal reserved for Regenerate (which invalidates external URLs) |
| Character-quota refresh cadence | Manual only (on Test-key click) | D-23 explicit; no polling |
| Number-input widget | Small `<input type="number">` with label left + value-with-unit right | Matches `SliderControl` rhythm; no up/down chrome visible |

---

## Deferred UI Items (NOT in Phase 13)

Explicitly out of scope for this spec — listed so the checker and planner don't flag their absence:

| Item | Deferred per |
|------|--------------|
| Preset templates (Quiet / Chatty / Priority-only) | D-20 |
| Per-language voice routing | D-27 |
| Character-quota bar / graph / sparkline | D-23 |
| Character-quota polling or low-quota warning toast | D-23 |
| Per-viewer TTS opt-out UI | CONTEXT `<deferred>` |
| Mod-flagged `/highlight`-triggered TTS | CONTEXT `<deferred>` |
| Auto-language detection dropdown | CONTEXT `<deferred>` |
| Per-source (not per-platform) TTS toggle | CONTEXT `<deferred>` |
| Per-error ElevenLabs fallback strategy UI | D-38 (simplified session-wide) |
| Hardcoded voice list fallback for unsaved key | CONTEXT `<deferred>` |
| Custom voice cloning upload | CONTEXT `<deferred>` |
| Short-lived auto-rotating OBS tokens with websocket push | CONTEXT `<deferred>` |

---

## Mount Points (for planner)

One-line patches the planner will translate into tasks. Paths and line numbers from RESEARCH.md.

| File | Change |
|------|--------|
| `frontend/src/components/appearance/AppearancePanel.tsx` | Add `CollapsibleSection id="tts" title="Text-to-Speech"` containing `<TTSGroup …/>` immediately after the existing `id="sounds"` block (line 89-97). Extend `AppearancePanelProps` with `onTTSChange`, `overlayId`, `hasElevenLabsConfig`, `obsUrl`, plus the TTS async callbacks. |
| `frontend/src/components/appearance/TTSGroup.tsx` | New file. See component inventory above. |
| `frontend/src/lib/hooks/useBrowserVoices.ts` | New file. Handles Chromium `voiceschanged` quirk. |
| `frontend/src/lib/types/overlay.ts` | Extend `DisplaySettings` with the 20 `tts_*` fields (D-24 verbatim). |
| `frontend/src/lib/api/overlays.ts` | Add `saveTTSKey`, `rotateTTSToken`, `getTTSVoices`, `testTTSKey`, `removeTTSKey`, `getTTSConfig` (the 7th endpoint per RESEARCH Open Question 3). |
| `frontend/src/app/overlays/[id]/page.tsx` | Load TTS config on mount (mirror `notification_sound_*` load at lines 1393-1403); pass `overlayId`, `hasElevenLabsConfig`, `obsUrl`, all callbacks into `AppearancePanel`; on `onTTSChange` patch, fire `TTS_SETTINGS_UPDATE` postMessage to embed iframe. |
| `frontend/src/app/overlays/[id]/preview/embed/page.tsx` | Add `ttsPlayerRef`, `ttsSettingsRef`, the `TTS_SETTINGS_UPDATE` listener near line 272, the config-load branch near line 347, and the `ttsPlayerRef.current?.speak(message)` call adjacent to line 400. |
| `frontend/src/app/overlay/[id]/page.tsx` | Same three integration points as the embed page (refs, load, speak-on-non-filtered at line 414). |

---

## Open Questions for the Planner

**None.** All design decisions are locked against CONTEXT.md D-01 … D-42 and/or resolved under Inferred Defaults. Everything downstream can proceed from this contract without further user input.

---

## Checker Sign-Off

- [ ] Dimension 1 Copywriting: PASS — explicit table above, all CTAs/errors/toasts specified verbatim
- [ ] Dimension 2 Visuals: PASS — component tree + mount points + reused primitives
- [ ] Dimension 3 Color: PASS — palette reused from DESIGN_SYSTEM.md; accent list explicit
- [ ] Dimension 4 Typography: PASS — 4-role scale, 2 weights (400 / 600) exactly; no new tokens
- [ ] Dimension 5 Spacing: PASS — Tailwind 4/8/16/24/32 only; explicit class list; no odd values
- [ ] Dimension 6 Registry Safety: PASS — zero third-party blocks introduced

**Approval:** pending

---

## UI-SPEC COMPLETE
