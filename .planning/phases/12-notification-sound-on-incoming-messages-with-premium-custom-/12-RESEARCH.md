# Phase 12: Notification Sound on Incoming Messages (with Premium Custom Sound) - Research

**Researched:** 2026-04-12
**Domain:** Browser Audio API, React frontend, Next.js overlay page
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Pooled HTMLAudioElement approach — pre-create a small pool of `<audio>` elements, pick one and play on each trigger. Simple, well-supported, handles overlapping sounds naturally.
- **D-02:** OBS browser source allows autoplay without user gesture. For standalone browsers, a single user click unlocks audio. No complex AudioContext setup needed.
- **D-03:** Ship 3-5 preset sound files as static assets in `/public/sounds/` (MP3 or OGG). Small footprint (~10-50KB total). Overlay page loads them by URL path.
- **D-04:** Preset names: at minimum "chime", "pop", "ping". Claude's discretion on the exact number and naming of additional presets.
- **D-05:** Cooldown timer — after playing a sound, enforce a minimum delay before the next trigger. Default ~500ms. Prevents audio spam in high-traffic chats.
- **D-06:** Cooldown duration is configurable via a slider in the Sound settings UI.
- **D-07:** Frontend-only gate — custom sound URL input is disabled with a PremiumBadge upsell prompt when `!user.is_premium`. Matches the existing viewer cosmetics pattern. No backend validation needed.
- **D-08:** When premium user provides a custom sound URL, it's used instead of the selected preset. Falls back to preset if custom URL fails to load.
- **D-09:** Add to `DisplaySettings` in `frontend/src/lib/types/overlay.ts`:
  - `notification_sound_enabled?: boolean`
  - `notification_sound_preset?: string` (e.g., "chime", "pop", "ping")
  - `notification_sound_url?: string` (premium: custom sound URL)
  - `notification_sound_volume?: number` (0–1)
  - `notification_sound_cooldown?: number` (milliseconds, default 500)

### Claude's Discretion

- Exact audio file formats (MP3 vs OGG vs both with fallback)
- Audio pool size (2-4 elements typically sufficient)
- Sound preview/test button in the editor UI
- Error handling for failed custom URL loads (silent fallback vs toast)
- Whether cooldown slider shows milliseconds or a friendlier label

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

---

## Summary

This phase adds notification sounds to the overlay page. When a chat message passes the filter check, a short audio clip plays. The audio mechanism is a small pool of pre-created `HTMLAudioElement` objects (pool size 3 is recommended). Sounds are gated by a cooldown timer (default 500ms, configurable). All users get preset sounds served from `/public/sounds/`; premium users can additionally supply a custom URL.

The implementation is entirely frontend-side: a new `SoundGroup` component in the AppearancePanel, a pure `soundPlayer.ts` utility (parallel to `filterMessage.ts`), extensions to the `DisplaySettings` type, and playback wired into the overlay page's `onmessage` handler. No backend changes are required — `DisplaySettings` is stored as `map[string]any` on the backend, so new fields auto-persist without a migration.

The Phase 11 `FilterGroup.tsx` is the canonical reference for this phase: it shows exactly how a new AppearancePanel group is registered, how `DisplaySettings`/`FilterSettings` states flow through the editor page, and how the Phase 11 postMessage pattern enables WYSIWYG preview.

**Primary recommendation:** Follow the FilterGroup pattern exactly. Extract playback logic into `src/lib/utils/soundPlayer.ts`, add `SoundGroup.tsx` next to `FilterGroup.tsx`, wire both into `AppearancePanel`, propagate sound settings through the editor the same way filter settings are propagated, and call `playNotificationSound()` immediately after the filter guard in `overlay/[id]/page.tsx`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Browser HTMLAudioElement | native | Audio playback | No import needed; universally supported in Chromium (OBS) and all modern browsers [VERIFIED: MDN] |
| React hooks (useState, useRef, useEffect, useCallback) | 19+ | Pool lifecycle, settings state | Already in use throughout the codebase [VERIFIED: codebase grep] |
| TypeScript | 5.x | Type-safe settings interface | Project standard [VERIFIED: package.json] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Vitest + @testing-library/react | current | Unit tests for SoundGroup, soundPlayer | All appearance components have parallel tests [VERIFIED: codebase] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pooled HTMLAudioElement | Web Audio API (AudioContext) | Web Audio API requires unlocking on first gesture for standard browsers, more complex, overkill for simple one-shot notifications. HTMLAudioElement pool handles overlapping sounds natively and OBS browser source never requires a gesture unlock (D-02). |
| MP3 preset files | OGG or WAV | MP3 is universally decoded including OBS's bundled Chromium. OGG has slightly better open-source licensing but MP3 support is universal as of 2017 patent expiry. Recommendation: ship both .mp3 and .ogg versions per file and use `<source>` fallback for maximum compatibility (Claude's discretion). |

**Installation:** No new packages. All functionality uses browser-native APIs and already-installed React.

---

## Architecture Patterns

### Recommended Project Structure

```
frontend/
├── public/
│   └── sounds/                      # NEW — static preset audio assets
│       ├── chime.mp3
│       ├── chime.ogg
│       ├── pop.mp3
│       ├── pop.ogg
│       ├── ping.mp3
│       └── ping.ogg
├── src/
│   ├── lib/
│   │   ├── types/
│   │   │   └── overlay.ts           # EDIT — extend DisplaySettings (D-09)
│   │   └── utils/
│   │       ├── soundPlayer.ts       # NEW — pool + cooldown logic (pure utility)
│   │       └── __tests__/
│   │           └── soundPlayer.test.ts  # NEW — unit tests
│   └── components/
│       └── appearance/
│           ├── SoundGroup.tsx       # NEW — UI for sound settings
│           ├── AppearancePanel.tsx  # EDIT — add SoundGroup
│           └── __tests__/
│               └── SoundGroup.test.tsx  # NEW — component tests
└── src/app/
    ├── overlay/[id]/page.tsx        # EDIT — read sound settings, call playNotificationSound
    └── overlays/[id]/page.tsx       # EDIT — add sound state, send postMessage to embed
```

### Pattern 1: Pure Utility for Playback Logic (follows filterMessage.ts)

**What:** Extract all audio pool and cooldown logic into `src/lib/utils/soundPlayer.ts`. The overlay page imports `createSoundPlayer()` and the returned object manages internal pool state. The SoundGroup tests test the utility directly.

**When to use:** Whenever logic needs to be testable independently of React. Exactly how `shouldFilterMessage` is tested without rendering the overlay page.

**Example:**
```typescript
// src/lib/utils/soundPlayer.ts
// Source: Phase 11 filterMessage.ts pattern [VERIFIED: codebase]

export interface SoundSettings {
  enabled: boolean
  preset: string          // 'chime' | 'pop' | 'ping'
  volume: number          // 0–1
  cooldownMs: number      // default 500
  customUrl?: string      // premium only
}

export interface SoundPlayer {
  play(): void
  updateSettings(s: SoundSettings): void
  destroy(): void
}

const PRESET_BASE = '/sounds/'
const POOL_SIZE = 3

export function createSoundPlayer(initial: SoundSettings): SoundPlayer {
  let settings = { ...initial }
  let lastPlayedAt = 0

  // Pre-create pool eagerly — no gesture required in OBS (D-02)
  const pool: HTMLAudioElement[] = Array.from({ length: POOL_SIZE }, () => {
    const el = new Audio()
    el.volume = settings.volume
    return el
  })
  let poolIndex = 0

  function resolveUrl(): string {
    if (settings.customUrl) return settings.customUrl
    return `${PRESET_BASE}${settings.preset}.mp3`
  }

  function play(): void {
    if (!settings.enabled) return
    const now = Date.now()
    if (now - lastPlayedAt < settings.cooldownMs) return
    lastPlayedAt = now

    const el = pool[poolIndex % POOL_SIZE]
    poolIndex++
    el.volume = settings.volume
    el.src = resolveUrl()
    el.currentTime = 0
    el.play().catch(() => {
      // Autoplay blocked — silently ignore (D-02: one click unlocks)
    })
  }

  function updateSettings(s: SoundSettings): void {
    settings = { ...s }
    pool.forEach((el) => { el.volume = s.volume })
  }

  function destroy(): void {
    pool.forEach((el) => { el.pause(); el.src = '' })
  }

  return { play, updateSettings, destroy }
}
```

### Pattern 2: SoundGroup Component (follows FilterGroup.tsx exactly)

**What:** A new `SoundGroup.tsx` component that accepts `displaySettings: Partial<DisplaySettings>` and `onChange: (patch: Partial<DisplaySettings>) => void`. Uses `ToggleSwitch`, `SliderControl`, and select/input for preset selection and custom URL.

**When to use:** Adding a new settings group to `AppearancePanel`. This is identical to how `FilterGroup` was added in Phase 11.

**Example:**
```tsx
// src/components/appearance/SoundGroup.tsx
// Source: Phase 11 FilterGroup.tsx pattern [VERIFIED: codebase]
'use client'

import React from 'react'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import { PremiumBadge } from '@/components/PremiumBadge'
import type { DisplaySettings } from '@/lib/types/overlay'

export const SOUND_PRESETS = ['chime', 'pop', 'ping'] as const
export type SoundPreset = typeof SOUND_PRESETS[number]

export interface SoundGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  onPreview?: () => void   // Claude's discretion: optional preview button
}

export function SoundGroup({ displaySettings, onChange, isPremium, onPreview }: SoundGroupProps): React.ReactElement {
  const enabled = displaySettings.notification_sound_enabled ?? false
  const preset = displaySettings.notification_sound_preset ?? 'chime'
  const volume = displaySettings.notification_sound_volume ?? 0.5
  const cooldown = displaySettings.notification_sound_cooldown ?? 500
  const customUrl = displaySettings.notification_sound_url ?? ''

  return (
    <div className="space-y-4">
      <ToggleSwitch
        label="Enable notification sounds"
        checked={enabled}
        onChange={(checked) => onChange({ notification_sound_enabled: checked })}
      />
      {/* ... preset selector, volume slider, cooldown slider, premium custom URL ... */}
    </div>
  )
}
```

### Pattern 3: AppearancePanel Extension

**What:** Add `SoundGroup` to `AppearancePanel` as a conditional section, passing `displaySettings`, `onSoundChange`, and `isPremium`. This follows the exact conditional pattern for `FilterGroup`:

```tsx
// In AppearancePanel.tsx — add after FilterGroup section
{displaySettings && onSoundChange && (
  <CollapsibleSection id="sound" title="Notification Sounds">
    <SoundGroup
      displaySettings={displaySettings}
      onChange={onSoundChange}
      isPremium={isPremium ?? false}
    />
  </CollapsibleSection>
)}
```

`AppearancePanelProps` gains `displaySettings?: Partial<DisplaySettings>`, `onSoundChange?: (patch: Partial<DisplaySettings>) => void`, and `isPremium?: boolean`.

### Pattern 4: Editor Page Integration (follows Phase 11 filter pattern)

**What:** The editor page (`overlays/[id]/page.tsx`) maintains sound settings state, saves it in `handleSaveConfiguration`, and sends live updates to the embed iframe via postMessage.

**Key state additions:**
```typescript
const [soundSettings, setSoundSettings] = useState<Partial<DisplaySettings>>({})
const soundSettingsRef = useRef<Partial<DisplaySettings>>({})
soundSettingsRef.current = soundSettings
```

**Save integration** — extend `handleSaveConfiguration`'s `display_settings` object with the 5 new fields from `soundSettings`.

**postMessage pattern** — add `SOUND_SETTINGS_UPDATE` message type, parallel to `FILTER_SETTINGS_UPDATE`:
```typescript
const sendSoundSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
  iframeRef.current?.contentWindow?.postMessage(
    { type: 'SOUND_SETTINGS_UPDATE', soundSettings: settings },
    '*'
  )
}, [])
```

Re-send on `EMBED_READY` alongside filter settings.

### Pattern 5: Overlay Page Playback Integration

**What:** In `overlay/[id]/page.tsx`, load sound settings from `display_settings`, create a `SoundPlayer` via `createSoundPlayer()`, and call `player.play()` immediately after the Phase 11 filter guard.

**Exact insertion point** (line ~347 in current code):
```typescript
// Phase 11: apply filter settings before adding to render queue (D-01, D-02)
if (shouldFilterMessage(message, filterSettingsRef.current)) return;

// Phase 12: play notification sound for messages that pass the filter (D-05, D-08)
soundPlayerRef.current?.play();
```

The `soundPlayerRef` is created in a `useEffect` that watches the loaded sound settings, so `destroy()` is called on cleanup.

### Anti-Patterns to Avoid

- **Creating a new `Audio()` on every message:** Causes garbage collection pressure and potential AudioContext limit exhaustion in some browsers. Use the pool.
- **Calling `play()` synchronously in `useState` setter callback:** React batches state updates; the `onmessage` handler is already outside React's render cycle so calling `play()` directly is safe.
- **Making the custom URL CORS-dependent for the settings page:** The settings editor is served from the same origin. Custom URLs for playback in the overlay page may hit CORS if the URL is cross-origin — document this as a known limitation (user's responsibility to use CORS-enabled CDN URLs).
- **Adding sound settings to `VisualSettings` instead of `DisplaySettings`:** `VisualSettings` drives CSS variables via `visualSettingsToCss`. Sound settings have no CSS analog and belong in `DisplaySettings` per D-09.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Audio format support | Custom codec detection logic | Serve both `.mp3` and `.ogg`; browser auto-selects via `<source>` or try `.mp3` first (universally supported) | MP3 patent-free since 2017; Chromium (OBS) decodes both |
| Debounce/throttle library | Custom timing wheel | Simple `Date.now()` delta comparison in `soundPlayer.ts` | The cooldown is trivially implementable; no library needed for a single timer |
| State persistence | IndexedDB or custom serialization | `DisplaySettings` JSONB field already persisted via `updateConfig()` | Already works — zero migration required |

---

## Runtime State Inventory

Step 2.5: SKIPPED — this is a greenfield feature addition, not a rename/refactor/migration phase. No runtime state carries the old state that needs migrating.

---

## Environment Availability

Step 2.6: The phase requires:

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Build/test | Yes | v25.9.0 | — |
| Vitest | Unit tests | Yes | current | — |
| Browser HTMLAudioElement | Audio playback | Yes (Chromium/OBS) | native | — |
| `/public/sounds/` dir | Static audio files | No (must create) | — | Create in Wave 0 |

**Missing dependencies with no fallback:**
- `/public/sounds/` directory and preset audio files (`.mp3`/`.ogg`) must be created and committed as binary assets in Wave 0.

**Missing dependencies with fallback:**
- None.

---

## Common Pitfalls

### Pitfall 1: Autoplay Policy in Non-OBS Browsers

**What goes wrong:** Calling `audioElement.play()` before any user gesture throws a `DOMException: play() failed because the user didn't interact with the document first`.

**Why it happens:** Chromium's autoplay policy blocks audio playback without prior user interaction. OBS browser source is exempt, but users previewing the overlay in a regular browser tab are not.

**How to avoid:** Wrap `play()` calls in `.catch(() => {})` to silence the rejection. The overlay will silently skip the first sounds until the user clicks anywhere on the page. Document this in the UI: "Sounds require a click to activate in regular browsers (not required in OBS)."

**Warning signs:** `Uncaught (in promise) DOMException` in the browser console.

### Pitfall 2: Pool `src` Assignment Triggers Network Request on Every Play

**What goes wrong:** Setting `el.src = resolveUrl()` on every `play()` call causes the browser to re-fetch the audio file each time, even if it's the same URL.

**How to avoid:** Track the last-assigned src per pool element. Only reassign `el.src` when the URL changes (preset switched or custom URL changed). The `updateSettings()` method is the right place to flush pool `src` values when settings change.

### Pitfall 3: Custom URL CORS Errors Silently Break Fallback

**What goes wrong:** A user provides a custom sound URL from a server without CORS headers. `el.play()` rejects or audio is blocked. The fallback to preset may not trigger if the error is asynchronous.

**How to avoid:** In the `onerror` handler on the pooled element, detect load failures and re-assign the preset URL as fallback (D-08). Log a console warning so the user can diagnose.

```typescript
el.onerror = () => {
  console.warn('[SoundPlayer] Custom URL failed, falling back to preset')
  el.src = `${PRESET_BASE}${settings.preset}.mp3`
}
```

### Pitfall 4: Stale Settings in WebSocket Closure

**What goes wrong:** The `soundPlayerRef.current` captures a snapshot of settings at effect creation time. If `updateSettings()` is not called when settings change, the player uses stale values.

**Why it happens:** Same closure-staleness issue that Phase 11 solved for `filterSettingsRef`.

**How to avoid:** Use a ref pattern: `soundSettingsRef.current = soundSettings` kept in sync via a non-effect assignment (same as `filterSettingsRef`). The `SoundPlayer.updateSettings()` call happens in the ref-update path, not in the WebSocket handler.

### Pitfall 5: Audio Files Not Committed as Binary

**What goes wrong:** Git may try to diff `.mp3`/`.ogg` files as text, or LFS may be triggered unexpectedly.

**How to avoid:** Check `.gitattributes` before committing. Add `*.mp3 binary` and `*.ogg binary` if not already present. The files are small (~10-50KB total) so LFS is not required.

---

## Code Examples

### soundPlayer.ts — Full Implementation Pattern

```typescript
// Source: Phase 11 filterMessage.ts pure utility pattern [VERIFIED: codebase]
// src/lib/utils/soundPlayer.ts

export interface SoundSettings {
  enabled: boolean
  preset: string
  volume: number          // 0–1
  cooldownMs: number
  customUrl?: string
}

export interface SoundPlayer {
  play(): void
  updateSettings(s: SoundSettings): void
  destroy(): void
}

export const PRESET_NAMES = ['chime', 'pop', 'ping'] as const
const PRESET_BASE = '/sounds/'
const POOL_SIZE = 3

export function createSoundPlayer(initial: SoundSettings): SoundPlayer {
  let settings = { ...initial }
  let lastPlayedAt = 0

  const pool: HTMLAudioElement[] = Array.from({ length: POOL_SIZE }, () => {
    const el = new Audio()
    el.volume = settings.volume
    return el
  })
  let poolIdx = 0

  function resolveUrl(s: SoundSettings): string {
    return s.customUrl ?? `${PRESET_BASE}${s.preset}.mp3`
  }

  function play(): void {
    if (!settings.enabled) return
    const now = Date.now()
    if (now - lastPlayedAt < settings.cooldownMs) return
    lastPlayedAt = now

    const el = pool[poolIdx % POOL_SIZE]
    poolIdx++

    const url = resolveUrl(settings)
    if (el.src !== url) {
      el.src = url
      el.onerror = () => {
        // D-08: fallback to preset on custom URL failure
        if (settings.customUrl) {
          console.warn('[SoundPlayer] Custom URL failed, falling back to preset')
          el.src = `${PRESET_BASE}${settings.preset}.mp3`
          el.onerror = null
        }
      }
    }
    el.volume = settings.volume
    el.currentTime = 0
    el.play().catch(() => {
      // Autoplay blocked — silent; first user click will unlock (D-02)
    })
  }

  function updateSettings(s: SoundSettings): void {
    settings = { ...s }
    pool.forEach((el) => { el.volume = s.volume })
  }

  function destroy(): void {
    pool.forEach((el) => { el.pause(); el.src = '' })
  }

  return { play, updateSettings, destroy }
}
```

### overlay/[id]/page.tsx — Sound Settings Load + Playback

```typescript
// Source: Phase 11 filterSettings pattern [VERIFIED: codebase]

// --- State ---
const [soundEnabled, setSoundEnabled] = useState(false)
const [soundPreset, setSoundPreset] = useState('chime')
const [soundVolume, setSoundVolume] = useState(0.5)
const [soundCooldown, setSoundCooldown] = useState(500)
const [soundCustomUrl, setSoundCustomUrl] = useState<string | undefined>(undefined)

const soundPlayerRef = useRef<import('@/lib/utils/soundPlayer').SoundPlayer | null>(null)

// --- Init player when settings load ---
useEffect(() => {
  const { createSoundPlayer } = require('@/lib/utils/soundPlayer')  // or top-level import
  if (soundPlayerRef.current) {
    soundPlayerRef.current.updateSettings({
      enabled: soundEnabled,
      preset: soundPreset,
      volume: soundVolume,
      cooldownMs: soundCooldown,
      customUrl: soundCustomUrl,
    })
  } else {
    soundPlayerRef.current = createSoundPlayer({
      enabled: soundEnabled,
      preset: soundPreset,
      volume: soundVolume,
      cooldownMs: soundCooldown,
      customUrl: soundCustomUrl,
    })
  }
  return () => {
    soundPlayerRef.current?.destroy()
    soundPlayerRef.current = null
  }
}, [soundEnabled, soundPreset, soundVolume, soundCooldown, soundCustomUrl])

// --- In config load (after Phase 11 filter settings block) ---
// Phase 12: load sound settings
if (typeof display.notification_sound_enabled === 'boolean') {
  setSoundEnabled(display.notification_sound_enabled)
}
if (typeof display.notification_sound_preset === 'string') {
  setSoundPreset(display.notification_sound_preset)
}
if (typeof display.notification_sound_volume === 'number') {
  setSoundVolume(display.notification_sound_volume)
}
if (typeof display.notification_sound_cooldown === 'number') {
  setSoundCooldown(display.notification_sound_cooldown)
}
if (typeof display.notification_sound_url === 'string') {
  setSoundCustomUrl(display.notification_sound_url || undefined)
}

// --- In ws.onmessage, after Phase 11 filter guard ---
// Phase 11: apply filter settings before adding to render queue
if (shouldFilterMessage(message, filterSettingsRef.current)) return;

// Phase 12: play notification sound for messages that pass the filter (D-05)
soundPlayerRef.current?.play();
```

### Editor Page — Sound Settings Propagation

```typescript
// Source: Phase 11 filter settings propagation pattern [VERIFIED: codebase]

// State (parallel to filterSettings)
const [soundSettings, setSoundSettings] = useState<Partial<DisplaySettings>>({})
const soundSettingsRef = useRef<Partial<DisplaySettings>>({})
soundSettingsRef.current = soundSettings

// postMessage sender
const sendSoundSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
  iframeRef.current?.contentWindow?.postMessage(
    { type: 'SOUND_SETTINGS_UPDATE', soundSettings: settings },
    '*'
  )
}, [])

// Change handler
const handleSoundSettingsChange = useCallback((patch: Partial<DisplaySettings>) => {
  setSoundSettings((prev) => {
    const next = { ...prev, ...patch }
    sendSoundSettingsToIframe(next)
    return next
  })
}, [sendSoundSettingsToIframe])

// Re-send on EMBED_READY (add alongside filter settings send)
sendSoundSettingsToIframe(soundSettingsRef.current)

// Save integration — add to display_settings in handleSaveConfiguration
display_settings: {
  // ... existing fields ...
  notification_sound_enabled: soundSettings.notification_sound_enabled ?? false,
  notification_sound_preset: soundSettings.notification_sound_preset ?? 'chime',
  notification_sound_volume: soundSettings.notification_sound_volume ?? 0.5,
  notification_sound_cooldown: soundSettings.notification_sound_cooldown ?? 500,
  notification_sound_url: soundSettings.notification_sound_url,
}
```

### Embed Page — postMessage Listener Addition

```typescript
// Source: Phase 11 FILTER_SETTINGS_UPDATE listener pattern [VERIFIED: codebase]
// In embed/page.tsx handleMessage(), add after FILTER_SETTINGS_UPDATE block:

if (event.data?.type === 'SOUND_SETTINGS_UPDATE') {
  const s = event.data.soundSettings as Partial<DisplaySettings>
  // Update sound state — embed page plays sounds if the editor enables preview
  setSoundSettings(s)  // or directly update soundPlayerRef if embed also plays
  return
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Web Audio API for simple notifications | HTMLAudioElement pool | N/A | HTMLAudioElement is simpler; Web Audio API needed only for procedural synthesis |
| Playing on every message (no throttle) | Cooldown timer (D-05) | Design decision | Prevents audio spam in busy chats |

**Deprecated/outdated:**
- None for this domain.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (unit project) |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/soundPlayer.test.ts src/components/appearance/__tests__/SoundGroup.test.tsx` |
| Full suite command | `cd frontend && npx vitest run --project unit` |

### Phase Requirements to Test Map

| Behavior | Test Type | Automated Command | File Exists? |
|----------|-----------|-------------------|-------------|
| `createSoundPlayer` respects `enabled: false` — `play()` is a no-op | unit | `vitest run ... soundPlayer.test.ts` | No — Wave 0 |
| `createSoundPlayer` respects cooldown — second `play()` within cooldown window is dropped | unit | `vitest run ... soundPlayer.test.ts` | No — Wave 0 |
| `createSoundPlayer` uses `customUrl` when set | unit | `vitest run ... soundPlayer.test.ts` | No — Wave 0 |
| `createSoundPlayer` falls back to preset when `customUrl` is not set | unit | `vitest run ... soundPlayer.test.ts` | No — Wave 0 |
| `updateSettings` changes volume on all pool elements | unit | `vitest run ... soundPlayer.test.ts` | No — Wave 0 |
| `SoundGroup` renders "Enable notification sounds" toggle | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |
| `SoundGroup` renders preset selector with chime/pop/ping options | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |
| `SoundGroup` renders custom URL input disabled when `isPremium=false` | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |
| `SoundGroup` renders custom URL input enabled when `isPremium=true` | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |
| `SoundGroup` onChange called with correct patch on toggle | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |
| `SoundGroup` onChange called with correct patch on volume slider | unit | `vitest run ... SoundGroup.test.tsx` | No — Wave 0 |

### Sampling Rate

- **Per task commit:** `npx vitest run --project unit src/lib/utils/__tests__/soundPlayer.test.ts src/components/appearance/__tests__/SoundGroup.test.tsx`
- **Per wave merge:** `npx vitest run --project unit`
- **Phase gate:** Full unit suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `frontend/src/lib/utils/__tests__/soundPlayer.test.ts` — covers all soundPlayer behaviors
- [ ] `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx` — covers SoundGroup UI behaviors

Note: E2E tests for audio playback are not feasible in a headless browser environment — `HTMLAudioElement.play()` requires user gesture or browser source privileges that Playwright cannot satisfy without special flags. Unit tests of the pure logic and component rendering are the appropriate test layer here.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | yes (frontend-only premium gate) | `isPremium` check disables custom URL input — D-07 explicitly accepts frontend-only gating |
| V5 Input Validation | yes (custom URL) | Validate that custom URL is a non-empty string before assigning to `el.src`; trust the browser to handle malformed URLs safely |
| V6 Cryptography | no | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Custom URL pointing to a malicious asset | Spoofing/Tampering | Browser's same-origin and CORS policy limits cross-origin reads; only audio playback is triggered, not script execution. No mitigation needed beyond browser-default. |
| XSS via custom URL injected into DOM | Tampering | Never inject `notification_sound_url` into innerHTML/DOM attributes. Only assign to `el.src` which is a safe assignment. |
| Premium bypass by editing localStorage | Elevation of Privilege | D-07 explicitly accepts this: overlay page is public, only the overlay owner configures settings. Custom URL in config was saved by the authenticated owner. The frontend gate is for the settings UI only, not playback. |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `/public/sounds/` will be served correctly by Next.js static file serving | Standard Stack | Low — Next.js always serves `/public` at root; this is core Next.js behavior [ASSUMED but standard] |
| A2 | OBS browser source (Chromium-based) does not require a user gesture to unlock audio autoplay | Standard Stack | Medium — if wrong, sounds will not play in OBS. Verified against documented OBS browser source behavior and W3C autoplay policy Chromium flags. [ASSUMED from training knowledge — verified in practice by community] |
| A3 | Audio preset files (`.mp3`) can be committed to git without triggering LFS | Environment | Low — files are small (~10-50KB total). Check `.gitattributes` before committing. [ASSUMED] |

---

## Open Questions

1. **Sound preview in the editor**
   - What we know: Claude's discretion (CONTEXT.md)
   - What's unclear: Whether preview button triggers playback in the editor page itself (not just the embed), and whether that requires initializing a SoundPlayer in the editor page too
   - Recommendation: Add preview in `SoundGroup.tsx` via a dedicated `onPreview` prop. The editor page creates a short-lived `SoundPlayer` just for preview (outside the embed). This avoids complicating the embed page.

2. **Cooldown slider display format**
   - What we know: Claude's discretion. Current `SliderControl` appends a `unit` string.
   - Recommendation: Display as `500 ms` using `unit=" ms"`. Range: 100ms–5000ms, step 100ms. A friendlier label like "0.5s" would require a format function — use raw ms to match the SliderControl pattern without modification.

3. **Number of presets**
   - What we know: CONTEXT.md specifies at minimum "chime", "pop", "ping"
   - Recommendation: Ship exactly 3 (chime, pop, ping). Small footprint; easy to expand later.

4. **Embed page sound preview**
   - What we know: The embed page is the WYSIWYG preview in the editor's split-view iframe
   - What's unclear: Should the embed preview also play sounds when messages arrive during preview?
   - Recommendation: Yes — the embed page should also initialize a SoundPlayer from `SOUND_SETTINGS_UPDATE` postMessage messages. This gives accurate WYSIWYG sound preview. The embed page's sound player must also call `.play()` after the filter guard in its own `onmessage` handler.

---

## Sources

### Primary (HIGH confidence)

- [VERIFIED: codebase] `frontend/src/lib/utils/filterMessage.ts` — pure utility pattern for Phase 12's `soundPlayer.ts`
- [VERIFIED: codebase] `frontend/src/components/appearance/FilterGroup.tsx` — canonical reference for `SoundGroup.tsx`
- [VERIFIED: codebase] `frontend/src/app/overlays/[id]/page.tsx` lines 1104–1174 — filter settings state and postMessage pattern
- [VERIFIED: codebase] `frontend/src/app/overlays/[id]/preview/embed/page.tsx` lines 199–248 — postMessage listener pattern
- [VERIFIED: codebase] `frontend/src/app/overlay/[id]/page.tsx` lines 346–352 — filter guard insertion point for sound playback
- [VERIFIED: codebase] `frontend/src/lib/types/overlay.ts` lines 36–52 — DisplaySettings interface to extend
- [VERIFIED: codebase] `frontend/vitest.config.ts` — test framework and project structure
- [VERIFIED: codebase] `frontend/src/components/appearance/__tests__/FilterGroup.test.tsx` — test pattern for SoundGroup tests

### Secondary (MEDIUM confidence)

- [CITED: MDN Web Docs] `HTMLAudioElement` and autoplay policy — browser-native audio playback API
- [CITED: MDN Web Docs] Autoplay policy — Chromium autoplay blocking and OBS browser source exemption

### Tertiary (LOW confidence)

- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all APIs are native browser or already in use; no new npm packages
- Architecture: HIGH — directly modeled on Phase 11 FilterGroup implementation, fully verified in codebase
- Pitfalls: HIGH — Autoplay policy and closure staleness are well-documented patterns visible in existing code

**Research date:** 2026-04-12
**Valid until:** 2026-06-12 (stable domain; Next.js static serving and HTMLAudioElement API are not fast-moving)
