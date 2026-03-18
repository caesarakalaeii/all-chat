---
phase: 34-appearance-controls-core
plan: "03"
subsystem: frontend/appearance
tags: [react, typescript, color-picker, visual-customizer, appearance-controls]
dependency_graph:
  requires: [34-01, 34-02]
  provides: [ColorPickerControl, ColorsGroup, BackgroundGroup, AppearancePanel]
  affects: [frontend/src/app/overlays/[id]/page.tsx]
tech_stack:
  added: []
  patterns: [TDD, use-client, click-outside-ref, HexColorPicker popover, opacity-as-decimal-string]
key_files:
  created:
    - frontend/src/components/appearance/ColorPickerControl.tsx
    - frontend/src/components/appearance/ColorsGroup.tsx
    - frontend/src/components/appearance/BackgroundGroup.tsx
    - frontend/src/components/appearance/AppearancePanel.tsx
  modified:
    - frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx
    - frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx
    - frontend/src/app/overlays/[id]/page.tsx
decisions:
  - "Opacity stored as decimal string ('0.0'–'1.0'); slider input uses 0–100 range and converts on change"
  - "Tests use getAllByText for labels that appear in both section heading and ColorPickerControl label props"
  - "AppearancePanel inserted before Custom CSS card (section 6→7 renumbering)"
metrics:
  duration: "~15m"
  completed: "2026-03-18T10:50:30Z"
  tasks_completed: 2
  files_created: 4
  files_modified: 3
---

# Phase 34 Plan 03: ColorPickerControl + ColorsGroup + BackgroundGroup + AppearancePanel Summary

**One-liner:** HexColorPicker-based color controls for message/username/timestamp colors and overlay/bubble backgrounds with opacity sliders, border controls, and backdrop blur, all composed into AppearancePanel mounted in the overlay editor for live preview.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | TDD failing tests | f3423cb | ColorsGroup.test.tsx, BackgroundGroup.test.tsx |
| 1 (GREEN) | ColorPickerControl + ColorsGroup + BackgroundGroup | f719ecd | ColorPickerControl.tsx, ColorsGroup.tsx, BackgroundGroup.tsx |
| 2 | AppearancePanel + editor page mount | d6bc8a7 | AppearancePanel.tsx, page.tsx |

## What Was Built

**ColorPickerControl** — Reusable hex color swatch with popover HexColorPicker (react-colorful). Click on swatch opens popover; click outside closes it via useEffect + mousedown listener on a container ref. Optional opacity slider rendered after swatch when `showOpacity=true`.

**ColorsGroup** — Three `ColorPickerControl` rows for `messageColor` (default #ffffff), `usernameColor` (default #a0a0ff), and `timestampColor` (default #888888). No opacity (text colors are solid only).

**BackgroundGroup** — Overlay background (color + opacity slider), bubble background (color + opacity slider), bubble border color, plus SliderControl rows for border radius (0–24px), border width (0–8px), padding (0–32px, step 2), message gap (0–24px, step 2), and backdrop blur (0–20px). Opacity values stored as decimal strings ('0.0'–'1.0').

**AppearancePanel** — Composes all three groups inside CollapsibleSection wrappers in order: Typography → Colors → Background & Bubbles. Outer wrapper is `<div className="flex flex-col gap-0">`.

**Editor page** — AppearancePanel imported and rendered in `frontend/src/app/overlays/[id]/page.tsx` as a new Card section before the Custom CSS card, passing `visualSettings` and `handleVisualSettingsChange`. Changing any control triggers `postMessage` to the preview iframe (via the pre-existing `handleVisualSettingsChange → sendCssToIframe` chain from Plan 02).

## Verification Results

- ColorsGroup unit tests: 6/6 passed
- BackgroundGroup unit tests: 7/7 passed
- Full unit suite: 17 test files, 91 tests passed, 0 failures
- TypeScript: 0 errors (`npx tsc --noEmit`)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test query ambiguity for duplicate label text**
- **Found during:** Task 1 GREEN phase
- **Issue:** `screen.getByText(/overlay background/i)` and `screen.getByText(/bubble background/i)` threw "multiple elements found" because the same text appears in both the `<p>` section heading and the ColorPickerControl `label` prop.
- **Fix:** Replaced `getByText` with `getAllByText(...).length >= 1` checks in BackgroundGroup tests.
- **Files modified:** `frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx`
- **Commit:** f3423cb (part of RED commit)

None other — plan executed as written.

## Self-Check: PASSED

Files created: FOUND x4
Files modified: FOUND x3
Commits: f3423cb, f719ecd, d6bc8a7 — all verified in git log
