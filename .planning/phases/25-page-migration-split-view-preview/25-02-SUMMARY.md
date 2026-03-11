---
phase: 25-page-migration-split-view-preview
plan: "02"
subsystem: frontend-landing-page
tags: [landing-page, magnetic-glow, oauth, design-system, accessibility]
dependency_graph:
  requires: [25-01]
  provides: [landing-page-redesign, beta-warning-dialog-migration]
  affects: [frontend/src/app/page.tsx, frontend/src/components/BetaWarning.tsx]
tech_stack:
  added: []
  patterns:
    - "useMagneticGlow hook with direct DOM mutation via glowRefs (no useState re-renders)"
    - "MagGlowCard inline component with 300px radial glow blob"
    - "Dialog.Root controlled open state for BetaWarning"
key_files:
  created: []
  modified:
    - frontend/src/app/page.tsx
    - frontend/src/components/BetaWarning.tsx
    - frontend/src/stories/LandingPage.stories.tsx
decisions:
  - "useMagneticGlow uses direct DOM mutation on glow elements via refs to avoid re-render storms from pointermove events"
  - "BetaWarning uses Dialog.Root with controlled open prop — no internal state, parent drives open/close"
  - "Kick button uses style={{ backgroundColor: 'var(--color-kick)' }} to avoid Tailwind JIT purge issues with dynamic color values"
  - "LandingPage story renders LandingHeroCards (self-contained) rather than importing page.tsx directly, avoiding OAuth router dependencies in Storybook"
metrics:
  duration: "12min"
  completed_date: "2026-03-11"
  tasks: 2
  files: 3
---

# Phase 25 Plan 02: Landing Page Redesign — Magnetic Glow Hero Summary

**One-liner:** Magnetic glow hero landing page with 4 platform stat cards, platform-colored OAuth login buttons (Twitch purple / YouTube red / Kick green), and BetaWarning migrated to Dialog component.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Rewrite landing page with magnetic glow hero | b729a0edd | page.tsx, BetaWarning.tsx |
| 2 | Update LandingPage story with real hero sections | 611b70d31 | LandingPage.stories.tsx |

## What Was Built

### Task 1: Landing Page Rewrite

`frontend/src/app/page.tsx` was rewritten from a gradient-bg page (224 lines) to a full magnetic glow hero design (295 lines):

- **useMagneticGlow hook** (inline): tracks pointer position via `pointermove` event on window, updates glow blob positions directly on DOM refs — no `useState` to avoid re-render storms
- **MagGlowCard component** (inline): wraps content in `relative overflow-hidden` container with 300px `radial-gradient` blob, `pointer-events-none`, positioned via direct style mutation
- **4 platform stat cards**: Twitch (IRC-based), YouTube (InnerTube), Kick (Pusher WS), TikTok (Live API)
- **Platform login buttons**: `bg-twitch`, `bg-youtube`, `var(--color-kick)` — each with `aria-label`, correct branding
- **Feature grid**: 3 MagGlowCard instances with LayoutGrid, Zap, Palette icons below hero
- **No `window.alert()`**: replaced with `toastManager.add()`
- **No `bg-gray-*` classes**: replaced with design token equivalents

`frontend/src/components/BetaWarning.tsx` migrated from custom `fixed inset-0` overlay to `Dialog.Root` + `Dialog.Content` from `@/components/ui/dialog`. All logic preserved (title/message/existingUserMessage, Discord link, Cancel/Continue buttons).

### Task 2: LandingPage Story

`frontend/src/stories/LandingPage.stories.tsx` replaced inline placeholder with `LandingHeroCards` component:
- 4 platform stat cards using `PlatformBadge` and `PLATFORM_COLORS`
- 3 login buttons each with `aria-label` for a11y audit compliance
- 3 feature grid cards with Lucide icons
- Both `Default` and `HeroWithLoginButtons` story exports

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- `npm run build` → `Compiled successfully` (TypeScript clean)
- No `window.alert` in page.tsx
- No `bg-gray-*` in page.tsx
- BetaWarning uses `Dialog.Root` + `Dialog.Content`
- Login buttons: `bg-twitch`, `bg-youtube`, `var(--color-kick)`
- YouTube button: white text + white icon on red background
- 4 platform stat cards in hero grid
- 3 feature cards in grid below hero
- LandingPage story has `aria-label` on all 3 login buttons

## Self-Check: PASSED

- `frontend/src/app/page.tsx` exists with 295+ lines (min_lines: 150 ✓)
- `frontend/src/stories/LandingPage.stories.tsx` exists and compiles ✓
- Commits b729a0edd and 611b70d31 exist in git log ✓
