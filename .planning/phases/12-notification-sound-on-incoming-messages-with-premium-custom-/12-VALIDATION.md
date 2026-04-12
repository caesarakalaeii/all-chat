# Phase 12: Notification Sound — Validation Architecture

**Phase:** 12-notification-sound-on-incoming-messages-with-premium-custom-
**Sourced from:** 12-RESEARCH.md — Validation Architecture section
**Status:** Pre-execution (test files do not yet exist)

---

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (unit project) |
| Config file | `frontend/vitest.config.ts` |
| Test project | `unit` |
| Per-task run command | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/soundPlayer.test.ts src/components/appearance/__tests__/SoundGroup.test.tsx` |
| Full suite command | `cd frontend && npx vitest run --project unit` |

**Note on E2E:** Audio playback E2E tests are not feasible in a headless browser environment. `HTMLAudioElement.play()` requires a user gesture or OBS browser source privileges that Playwright cannot satisfy without special flags. Unit tests of the pure logic and component rendering are the correct test layer for this phase.

---

## Requirement-to-Test Map

| Requirement | Behavior Under Test | Test Type | Test File | Automated Command |
|-------------|---------------------|-----------|-----------|-------------------|
| SND-01 | `createSoundPlayer` play() is a no-op when `enabled: false` | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-01 | `createSoundPlayer` triggers Audio.play() when `enabled: true` | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-02 | `createSoundPlayer` respects cooldown — second call within cooldownMs is dropped | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-02 | `createSoundPlayer` plays again after cooldown elapses | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-03 | `createSoundPlayer` uses `/sounds/{preset}.mp3` URL when no customUrl | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-04 | `createSoundPlayer` uses customUrl when provided | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-04 | onerror on customUrl falls back to preset URL (D-08) | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-05 | `updateSettings()` changes volume on all pool elements | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-05 | `destroy()` pauses all pool elements and clears src | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-05 | play() does not re-assign el.src when URL has not changed (Pitfall 2) | unit | `soundPlayer.test.ts` | `vitest run --project unit ... soundPlayer.test.ts` |
| SND-06 | `SoundGroup` renders "Enable notification sounds" toggle | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-06 | Toggle onChange emits `{ notification_sound_enabled: true/false }` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-07 | `SoundGroup` renders preset selector with chime/pop/ping options | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-07 | Selecting preset emits `{ notification_sound_preset: 'pop' }` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-08 | `SoundGroup` renders Volume slider with value from displaySettings | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-08 | Moving volume slider emits `{ notification_sound_volume: 0.7 }` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-08 | `SoundGroup` renders Cooldown slider with value from displaySettings | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-08 | Moving cooldown slider emits `{ notification_sound_cooldown: 1000 }` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-09 | Custom URL input is disabled and PremiumBadge rendered when `isPremium=false` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-09 | Custom URL input is enabled and PremiumBadge absent when `isPremium=true` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-09 | Typing in custom URL input emits `{ notification_sound_url: '...' }` | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |
| SND-06 | Sound controls hidden when `notification_sound_enabled` is false | unit | `SoundGroup.test.tsx` | `vitest run --project unit ... SoundGroup.test.tsx` |

---

## Wave 0 Gaps (test files to create before implementation)

Both test files must be created in Wave 1 (TDD: RED phase first) as part of Plan 01 Task 1 and Plan 02 Task 1 respectively:

- [ ] `frontend/src/lib/utils/__tests__/soundPlayer.test.ts` — 11 behaviors covering soundPlayer logic (SND-01 through SND-05)
- [ ] `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx` — 13 behaviors covering SoundGroup UI (SND-06 through SND-09)

---

## Sampling Rate

| Gate | Command | When |
|------|---------|------|
| Per task commit | `npx vitest run --project unit src/lib/utils/__tests__/soundPlayer.test.ts src/components/appearance/__tests__/SoundGroup.test.tsx` | After each TDD GREEN step |
| Per wave merge | `npx vitest run --project unit` | After Wave 1 (Plan 01) and Wave 2 (Plan 02) complete |
| Phase gate | `npx vitest run --project unit` — full unit suite green | Before `/gsd-verify-work` is invoked |

---

## Coverage Gaps Acknowledged

| Gap | Reason | Alternative |
|-----|--------|-------------|
| E2E sound playback test | `HTMLAudioElement.play()` requires user gesture or OBS privileges; not satisfiable in headless Playwright | `mockPlay` spy in unit tests verifies `play()` is called |
| Actual MP3 audio decoding | Not testable without browser audio stack | Files verified to exist via `test -f frontend/public/sounds/*.mp3` acceptance criteria |
| Cross-origin CORS behavior for custom URLs | Requires real network requests | Documented as known limitation in RESEARCH.md Pitfall 3; `onerror` fallback unit-tested |
