---
phase: 34-appearance-controls-core
plan: "01"
subsystem: frontend/appearance
tags:
  - typescript
  - react
  - visual-settings
  - appearance-controls
  - base-ui
dependency_graph:
  requires:
    - Phase 33 CSS architecture foundation (visual-settings.ts, visual-settings-to-css.ts)
  provides:
    - VisualSettings.usernameFontFamily and timestampFontFamily fields
    - PROPERTY_MAP entries for --chat-username-font-family and --chat-timestamp-font-family
    - CollapsibleSection component (localStorage persistence)
    - FontFamilyCombobox component (GOOGLE_FONT_NAMES export)
    - SliderControl component
    - TypographyGroup component
  affects:
    - overlay.ts (visual_settings now Partial<VisualSettings>)
    - Plan 03 AppearancePanel composition
tech_stack:
  added: []
  patterns:
    - "@base-ui/react Collapsible (controlled, keepMounted)"
    - "@base-ui/react Combobox with Portal+Positioner+Popup"
    - "@base-ui/react Select with Portal+Positioner+Popup"
    - "TDD: RED test scaffolds before type changes; GREEN after implementation"
key_files:
  created:
    - frontend/src/components/appearance/CollapsibleSection.tsx
    - frontend/src/components/appearance/FontFamilyCombobox.tsx
    - frontend/src/components/appearance/SliderControl.tsx
    - frontend/src/components/appearance/TypographyGroup.tsx
    - frontend/src/components/appearance/__tests__/CollapsibleSection.test.tsx
    - frontend/src/components/appearance/__tests__/TypographyGroup.test.tsx
    - frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx
    - frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx
  modified:
    - frontend/src/lib/types/visual-settings.ts
    - frontend/src/lib/utils/visual-settings-to-css.ts
    - frontend/src/lib/types/overlay.ts
    - frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts
decisions:
  - "Used @base-ui/react Combobox.Portal wrapper to satisfy Positioner context requirement"
  - "Stub tests for ColorsGroup and BackgroundGroup use test.todo; component tests (CollapsibleSection, TypographyGroup) use real renders with afterEach(cleanup)"
  - "CollapsibleSection tests include one todo for toggle state verification (requires full open/close animation testing)"
metrics:
  duration: "~9 minutes"
  completed_date: "2026-03-18"
  tasks: 2
  files: 12
---

# Phase 34 Plan 01: Appearance Controls Foundation — VisualSettings types + CollapsibleSection + TypographyGroup

Extended VisualSettings with per-element font family fields, tightened overlay.ts type to Partial<VisualSettings>, created Wave 0 test scaffolds, and implemented CollapsibleSection + TypographyGroup with FontFamilyCombobox and SliderControl primitives.

## What Was Built

### Type Extensions

- Added `usernameFontFamily?: string` and `timestampFontFamily?: string` to `VisualSettings` interface in the Username typography section
- Added `['usernameFontFamily', '--chat-username-font-family']` and `['timestampFontFamily', '--chat-timestamp-font-family']` to `PROPERTY_MAP` in `visual-settings-to-css.ts`
- Updated `OverlayConfig.visual_settings` from `Record<string, unknown>` to `Partial<VisualSettings>` in `overlay.ts`

### New Components

**CollapsibleSection** (`/frontend/src/components/appearance/CollapsibleSection.tsx`)
- Uses `@base-ui/react` `Collapsible.Root` in controlled mode
- Lazy `useState` initializer reads from `localStorage` key `appearance-panel-sections-v1`
- `onOpenChange` writes updated JSON back to localStorage
- Trigger shows title + `ChevronDown` icon; panel uses `keepMounted`

**FontFamilyCombobox** (`/frontend/src/components/appearance/FontFamilyCombobox.tsx`)
- 9 system fonts + 10 Google fonts in grouped combobox
- Each option rendered in its own font via inline `style={{ fontFamily }}`
- Exports `GOOGLE_FONT_NAMES: Set<string>` for dynamic font loading
- Client-side filtering via `onInputValueChange` + local `inputValue` state
- Uses `Combobox.Portal` wrapper (required by `Combobox.Positioner`)

**SliderControl** (`/frontend/src/components/appearance/SliderControl.tsx`)
- Pure presentational: label (fixed width), range input (flex-1), value + unit (fixed width)

**TypographyGroup** (`/frontend/src/components/appearance/TypographyGroup.tsx`)
- 3 `FontFamilyCombobox` pickers (body, username, timestamp)
- Font weight `Select` with 8 options (100 Thin — 900 Black)
- 3 number inputs for font sizes (body, username, timestamp)
- 2 `SliderControl` components (line height 1.0-2.5, letter spacing -2 to 8px)
- All `onChange` calls pass single-field `Partial<VisualSettings>` patches

### Test Scaffolds

Wave 0 stubs for all 4 component test files created and importable:
- `CollapsibleSection.test.tsx`: 2 tests passing, 1 todo
- `TypographyGroup.test.tsx`: 4 tests passing, 3 todo
- `ColorsGroup.test.tsx`: 1 passing, 4 todo (no component yet)
- `BackgroundGroup.test.tsx`: 1 passing, 5 todo (no component yet)

## Test Results

```
Test Files  17 passed (17)
Tests       80 passed | 13 todo (93)
```

`npx tsc --noEmit` — clean, no errors.

## Commits

| Commit | Message |
|--------|---------|
| `6738a48` | feat(34-01): extend VisualSettings types, update overlay.ts, add Wave 0 test scaffolds |
| `edb1b72` | feat(34-01): implement CollapsibleSection, FontFamilyCombobox, SliderControl, TypographyGroup |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added Combobox.Portal wrapper**
- **Found during:** Task 2
- **Issue:** `@base-ui/react` `Combobox.Positioner` requires a `Combobox.Portal` ancestor — missing portal caused "Base UI: <Combobox.Portal> is missing" runtime error
- **Fix:** Wrapped `Combobox.Positioner` inside `Combobox.Portal` in `FontFamilyCombobox.tsx`
- **Files modified:** `frontend/src/components/appearance/FontFamilyCombobox.tsx`
- **Commit:** `edb1b72`

**2. [Rule 3 - Blocking] Added afterEach(cleanup) to jsdom test files**
- **Found during:** Task 2
- **Issue:** Without global setup, `@testing-library/react` renders accumulated in the same DOM, causing "Found multiple elements" failures on subsequent tests
- **Fix:** Added `afterEach(() => { cleanup() })` to `CollapsibleSection.test.tsx` and `TypographyGroup.test.tsx`
- **Files modified:** Both test files
- **Commit:** `edb1b72`

**3. [Rule 1 - Bug] Combobox API uses onInputValueChange not onInputChange**
- **Found during:** Task 2
- **Issue:** Plan specified `onInputChange` but the actual `@base-ui/react` v1.2.0 API uses `onInputValueChange`
- **Fix:** Used `onInputValueChange` in `FontFamilyCombobox.tsx`
- **Files modified:** `frontend/src/components/appearance/FontFamilyCombobox.tsx`
- **Commit:** `edb1b72`

**4. [Rule 1 - Bug] Updated full-input test count from 47 to 49**
- **Found during:** Task 1
- **Issue:** Existing test asserted "47 properties" but adding 2 new font family fields brings total to 49
- **Fix:** Updated count assertion and added new fields to `Required<VisualSettings>` fixture
- **Files modified:** `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts`
- **Commit:** `6738a48`

## Self-Check: PASSED

All created files exist on disk. Both commits (`6738a48`, `edb1b72`) confirmed in git log.
