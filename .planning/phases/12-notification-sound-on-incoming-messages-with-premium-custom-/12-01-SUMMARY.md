---
phase: 12-notification-sound-on-incoming-messages-with-premium-custom-
plan: "01"
subsystem: frontend
tags: [audio, notification, soundPlayer, TDD, displaySettings]
dependency_graph:
  requires: []
  provides:
    - soundPlayer utility with pool + cooldown + custom URL fallback
    - DisplaySettings extended with 5 notification_sound_* fields
    - 3 preset MP3 audio files (chime, pop, ping)
    - overlay page wired for sound playback on incoming messages
    - embed preview page wired for sound playback + SOUND_SETTINGS_UPDATE postMessage
  affects:
    - frontend/src/lib/types/overlay.ts
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx
tech_stack:
  added:
    - ffmpeg (used to generate preset MP3 files, not a runtime dep)
  patterns:
    - Audio pool (3 elements, round-robin) for sound playback without AudioContext complexity
    - Cooldown timer using Date.now() delta to prevent audio spam in high-traffic chats
    - Custom URL onerror fallback to preset for reliability (D-08)
    - TDD: RED (11 failing tests) → GREEN (11 passing) before implementation
key_files:
  created:
    - frontend/src/lib/utils/soundPlayer.ts
    - frontend/src/lib/utils/__tests__/soundPlayer.test.ts
    - frontend/public/sounds/chime.mp3
    - frontend/public/sounds/pop.mp3
    - frontend/public/sounds/ping.mp3
  modified:
    - frontend/src/lib/types/overlay.ts
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx
decisions:
  - Audio pool size 3 — sufficient for typical chat rates; avoids AudioContext complexity per D-01/D-02
  - Round-robin pool index ensures no single element is reused while playing
  - PRESET_NAMES as const tuple for type safety without string widening
  - el.src only reassigned when URL changes — avoids unnecessary audio element reloads (Pitfall 2)
  - onerror handler set only when customUrl is provided — avoids spurious handler on preset-only elements
  - MockAudio class (not arrow fn) in tests — vi.stubGlobal requires constructor-callable function
metrics:
  duration: "~15 minutes"
  completed: "2026-04-12T00:40:21Z"
  tasks: 2
  files: 7
requirements:
  - SND-01
  - SND-02
  - SND-03
  - SND-04
  - SND-05
---

# Phase 12 Plan 01: Sound Player Utility, DisplaySettings Extension, and Overlay Wiring Summary

**One-liner:** Audio pool-based soundPlayer utility (TDD, 11 tests) with cooldown + custom URL fallback, wired into overlay and embed preview pages after the message filter guard.

## What Was Built

### Task 1: TDD soundPlayer + DisplaySettings + preset audio files (commit `8d2e065`)

**DisplaySettings extension** (`frontend/src/lib/types/overlay.ts`): Added 5 fields per D-09:
- `notification_sound_enabled?: boolean`
- `notification_sound_preset?: string`
- `notification_sound_url?: string`
- `notification_sound_volume?: number`
- `notification_sound_cooldown?: number`

**soundPlayer.ts** (`frontend/src/lib/utils/soundPlayer.ts`): Factory function `createSoundPlayer(settings)` returns a `SoundPlayer` with:
- `play()` — checks enabled flag, enforces cooldown via `Date.now()` delta, picks pool element round-robin, only reassigns `el.src` when URL changes, sets `onerror` fallback on custom URLs, calls `el.play().catch(() => {})`
- `updateSettings(settings)` — updates internal settings copy and syncs volume on all pool elements
- `destroy()` — pauses all pool elements and clears src

Exports: `createSoundPlayer`, `SoundSettings`, `SoundPlayer`, `PRESET_NAMES` (`['chime', 'pop', 'ping'] as const`).

**Preset audio files**: 3 MP3 files generated via ffmpeg sine wave synthesis:
- `chime.mp3`: 800Hz, 150ms, fade in/out (1.7KB)
- `pop.mp3`: 1200Hz, 80ms, sharp attack (1.3KB)
- `ping.mp3`: 1000Hz, 200ms, gentle fade (2.1KB)

**Unit tests** (`frontend/src/lib/utils/__tests__/soundPlayer.test.ts`): 11 tests covering all specified behaviors. TDD cycle: RED (module not found) → GREEN (all 11 pass). MockAudio uses a class-based constructor to satisfy `vi.stubGlobal` requirements.

### Task 2: Wire sound playback into overlay and embed pages (commit `6e3138a`)

**Overlay page** (`frontend/src/app/overlay/[id]/page.tsx`):
- Imports `createSoundPlayer`, `SoundPlayer`, `SoundSettings`
- Adds `soundPlayerRef` and `soundSettingsRef` refs with disabled defaults
- Config load useEffect: reads all 5 `notification_sound_*` fields from `display_settings`, creates or updates the sound player
- WebSocket `onmessage`: calls `soundPlayerRef.current?.play()` immediately after the `shouldFilterMessage` guard (filtered messages do not trigger sound)
- Cleanup useEffect: destroys the player on unmount

**Embed preview page** (`frontend/src/app/overlays/[id]/preview/embed/page.tsx`):
- Same imports, refs, config load, play() call, and cleanup as overlay page
- Additional `SOUND_SETTINGS_UPDATE` postMessage handler: receives `DisplaySettings` partial from editor, constructs `SoundSettings`, creates or updates player in real-time (mirrors `FILTER_SETTINGS_UPDATE` pattern from Phase 11)

## Verification

- `npx vitest run --project unit src/lib/utils/__tests__/soundPlayer.test.ts` — 11/11 tests pass
- `npx tsc --noEmit` — exits 0, no TypeScript errors

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] MockAudio must be a class, not an arrow function**

- **Found during:** Task 1 GREEN phase — first test run with `vi.fn(() => { ... })` threw `TypeError: ... is not a constructor`
- **Issue:** `vi.stubGlobal('Audio', vi.fn(() => {...}))` registers an arrow function which is not callable with `new`. The vitest warning said: "The vi.fn() mock did not use 'function' or 'class' in its implementation."
- **Fix:** Replaced the `vi.fn(() => {...})` stub with a `class MockAudio { constructor() { mockAudioInstances.push(this) } }` and passed the class directly to `vi.stubGlobal('Audio', MockAudio)`.
- **Files modified:** `frontend/src/lib/utils/__tests__/soundPlayer.test.ts`
- **Commit:** included in `8d2e065`

## Known Stubs

None — all sound settings fields read from the real `display_settings` API response. The defaults (`enabled: false`, `preset: 'chime'`, `volume: 0.5`, `cooldownMs: 500`) are intentional safe defaults for when config has not yet loaded or sound is not configured.

## Threat Surface Scan

No new network endpoints introduced. The `notification_sound_url` custom URL is assigned only to `el.src` (safe DOM property, not innerHTML). This matches T-12-02 mitigation in the plan's threat model. No additional threat flags beyond what the plan already documented.

## Self-Check: PASSED

| Item | Status |
|------|--------|
| `frontend/src/lib/utils/soundPlayer.ts` | FOUND |
| `frontend/src/lib/utils/__tests__/soundPlayer.test.ts` | FOUND |
| `frontend/public/sounds/chime.mp3` | FOUND |
| `frontend/public/sounds/pop.mp3` | FOUND |
| `frontend/public/sounds/ping.mp3` | FOUND |
| commit `8d2e065` (Task 1) | FOUND |
| commit `6e3138a` (Task 2) | FOUND |
| `notification_sound_enabled` in overlay.ts | FOUND |
| `createSoundPlayer` exported | FOUND |
| `PRESET_NAMES` exported | FOUND |
| `SoundSettings` interface exported | FOUND |
| `SoundPlayer` interface exported | FOUND |
| 11 unit tests pass | VERIFIED |
| TypeScript compiles clean | VERIFIED |
