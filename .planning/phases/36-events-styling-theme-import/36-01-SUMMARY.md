---
phase: 36-events-styling-theme-import
plan: "01"
subsystem: frontend-appearance
tags: [events, visual-settings, css-layers, appearance-panel, tdd]
dependency_graph:
  requires: []
  provides:
    - EventsGroup component with 5 event type rows
    - membershipGiftSizeModifier in VisualSettings + PROPERTY_MAP (50 entries)
    - events.css visual-customizer layer with per-event-type transform scale rules
    - AppearancePanel Events section (last CollapsibleSection)
  affects:
    - frontend/src/lib/types/visual-settings.ts
    - frontend/src/lib/utils/visual-settings-to-css.ts
    - frontend/src/components/appearance/AppearancePanel.tsx
    - frontend/src/styles/events.css
tech_stack:
  added: []
  patterns:
    - TDD (RED-GREEN) for both tasks
    - EVENT_ROWS array pattern matching VisibilityGroup ROWS pattern
    - CSS @layer visual-customizer with CSS custom property fallbacks matching marketplace-themes baseline (1.05)
key_files:
  created:
    - frontend/src/components/appearance/EventsGroup.tsx
    - frontend/src/components/appearance/__tests__/EventsGroup.test.tsx
  modified:
    - frontend/src/lib/types/visual-settings.ts
    - frontend/src/lib/utils/visual-settings-to-css.ts
    - frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts
    - frontend/src/styles/events.css
    - frontend/src/components/appearance/AppearancePanel.tsx
decisions:
  - EventsGroup lays out toggle and slider per row in a space-y-2 div (stacked), matching the vertical density of other groups
  - CSS fallback 1.05 in visual-customizer matches existing marketplace-themes baseline transform scale, ensuring no visual regression when no CSS var is set
metrics:
  duration: "~2.5 minutes"
  completed: "2026-03-18"
  tasks: 2
  files: 7
---

# Phase 36 Plan 01: Events Styling Controls + Type Extension Summary

EventsGroup component wiring five event-type rows (toggle + size modifier slider each) into AppearancePanel, with membershipGiftSizeModifier added to the type system and CSS transform scale rules in events.css visual-customizer layer.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add membershipGiftSizeModifier to type + PROPERTY_MAP + update existing test | 7aea27d | visual-settings.ts, visual-settings-to-css.ts, visual-settings-to-css.test.ts |
| 2 | Create EventsGroup component + tests, update events.css + AppearancePanel | 4004505 | EventsGroup.tsx, EventsGroup.test.tsx, events.css, AppearancePanel.tsx |

## What Was Built

### VisualSettings type extension
- Added `membershipGiftSizeModifier?: string` after `bitsSizeModifier` in the interface
- PROPERTY_MAP now has 50 entries with `['membershipGiftSizeModifier', '--chat-membership-gift-size-modifier']`

### EventsGroup component
- Five event rows: Super Chat, Subscriptions, Raids, Bits, Membership Gift
- Each row: `ToggleSwitch` (checked when value `!== 'none'`, emits `'block'|'none'`) + `SliderControl` (range 0.5–3.0, step 0.1, emits unitless float string)
- Uses `EVENT_ROWS` array of `{ label, showField, sizeField }` for DRY row definition

### events.css — @layer visual-customizer block
- Added after the closing `}` of `@layer marketplace-themes`
- Five per-event-type `transform: scale(var(--chat-*-size-modifier, 1.05))` rules
- Fallback `1.05` matches the existing marketplace-themes baseline so no visual regression occurs when no size modifier is set

### AppearancePanel wiring
- Imported `EventsGroup` from `'./EventsGroup'`
- Added `<CollapsibleSection id="events" title="Events">` as the last section after Platform Colors

## Test Results

- `visual-settings-to-css.test.ts`: 7/7 passed — confirms 50 CSS var declarations, `--chat-membership-gift-size-modifier: 1.2` present
- `EventsGroup.test.tsx`: 9/9 passed — labels, toggle count, slider count, checked state, onChange emissions for toggles and sliders
- Full unit suite: 21 test files, 127 tests passed

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED
