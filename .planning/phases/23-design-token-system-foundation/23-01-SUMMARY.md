---
phase: 23-design-token-system-foundation
plan: 01
subsystem: ui
tags: [tailwind, css, design-tokens, fonts, barlow, dm-mono, next-font, globals-css]

# Dependency graph
requires: []
provides:
  - Single-source design token system in globals.css with three-tier hierarchy
  - Cascade layer order (base, design-system, marketplace-themes, user-overrides)
  - Brand-exact platform colors (Twitch #9146FF, YouTube #FF4444, Kick #53FC18, TikTok #69C9D0)
  - Barlow and DM Mono fonts loaded via next/font/google as CSS variables
  - Font bridge mapping --font-barlow/--font-dm-mono into Tailwind via @theme inline
affects:
  - 24-component-library
  - 25-page-migration
  - 26-design-system-enforcement

# Tech tracking
tech-stack:
  added: [Barlow (next/font/google), DM_Mono (next/font/google)]
  patterns:
    - "@theme for raw-value tokens, @theme inline only for var() references"
    - "Three-tier token hierarchy: base (raw palette) → semantic (purpose-named) → component (specific-use)"
    - "Cascade layer order declared as @layer base, design-system, marketplace-themes, user-overrides"
    - "Dark-only (no light/dark toggle) — single token set, no .dark class"

key-files:
  created: []
  modified:
    - frontend/src/app/globals.css
    - frontend/src/app/layout.tsx

key-decisions:
  - "Removed shadcn/tailwind.css import — it was pulling in HSL defaults incompatible with new token system"
  - "Removed @custom-variant dark — app is dark-only, no class toggle needed"
  - "Used hex for platform colors (brand-exact), oklch for neutral scale (perceptually uniform)"
  - "color-mix() used for stat-glow and shadow tokens (Chrome 111+ / all modern browsers)"
  - "body element has no className — fonts inherit from html via CSS variable"

patterns-established:
  - "Token tier 1 (base): raw hex/oklch literals in @theme"
  - "Token tier 2 (semantic): var() references to tier 1 in @theme"
  - "Token tier 3 (component): color-mix() / oklch() computed from tier 2 in @theme"
  - "Font loading: next/font/google variable → html className → @theme inline bridge → Tailwind utility"

requirements-completed: [FOUND-01, FOUND-02, FOUND-06]

# Metrics
duration: 8min
completed: 2026-03-10
---

# Phase 23 Plan 01: Design Token System Foundation Summary

**Single-source dark-only CSS token system in globals.css with three-tier @theme hierarchy, cascade layer order, and Barlow/DM Mono fonts replacing Geist/Inter**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-10T19:15:00Z
- **Completed:** 2026-03-10T19:23:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Clean-slate globals.css rewrite eliminating two duplicate @layer base blocks, bare :root HSL block, bare :root oklch block, and .dark block
- Three-tier @theme token hierarchy established: base palette (neutral oklch scale + platform hex colors), semantic purpose-names (bg, surface, border, text), component tokens (stat glows, shadow glows, badge, nav)
- Cascade layer order declared: `@layer base, design-system, marketplace-themes, user-overrides`
- Barlow (weights 400-800) and DM Mono (weights 400-500) replace Geist and Inter, bridged into Tailwind via @theme inline
- npm run build and type-check both pass with zero errors

## Task Commits

Each task was committed atomically:

1. **Task 1: Update layout.tsx to load Barlow and DM Mono fonts** - `6ad370f4d` (feat)
2. **Task 2: Rewrite globals.css with design token system and cascade layers** - `8bb110aed` (feat)

## Files Created/Modified
- `frontend/src/app/layout.tsx` - Replaced Geist/Inter with Barlow/DM_Mono, removed body className
- `frontend/src/app/globals.css` - Complete clean-slate rewrite: @theme token system, cascade layers, font bridge

## Decisions Made
- Removed `@import "shadcn/tailwind.css"` — it was injecting the old HSL-based token system that conflicts with the new design token architecture
- Removed `@custom-variant dark` — the app is dark-only; no class toggle is needed, simplifying the token system
- Used plain hex for platform colors (guarantees brand-exact values), oklch with constant chroma/hue for neutral scale (perceptually uniform stepping)
- `color-mix()` used for stat-glow and shadow component tokens — safe for Chrome 111+ which covers all modern browsers and the target overlay environment
- `body` element has no `className` prop — fonts inherit from the `html` element via `--font-sans` CSS variable

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- globals.css is the authoritative token source that Phase 24 (component library) and Phase 25 (page migration) depend on
- All design tokens are available as Tailwind utilities: `bg-twitch`, `text-youtube`, `bg-bg`, `text-text-sub`, etc.
- Components using old shadcn tokens (--background, --foreground, --primary, etc.) will show broken styles until Phase 24 migration
- Cascade layer order is established, ready for marketplace-themes and user-overrides in later phases

---
*Phase: 23-design-token-system-foundation*
*Completed: 2026-03-10*
