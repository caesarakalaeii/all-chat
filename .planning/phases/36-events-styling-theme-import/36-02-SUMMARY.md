---
phase: 36-events-styling-theme-import
plan: "02"
subsystem: frontend-appearance
tags: [theme-import, css-parser, visual-settings, tdd]
dependency_graph:
  requires:
    - 36-01 (PROPERTY_MAP 50 entries, VisualSettings type)
  provides:
    - parseCssToVisualSettings utility (CSS string -> Partial<VisualSettings>)
    - PROPERTY_MAP exported from visual-settings-to-css.ts
  affects:
    - frontend/src/lib/utils/visual-settings-to-css.ts
tech_stack:
  added: []
  patterns:
    - TDD (RED-GREEN) for parseCssToVisualSettings
    - Regex defined inside function body to avoid stale lastIndex
    - Module-level REVERSE_MAP built once from exported PROPERTY_MAP
key_files:
  created:
    - frontend/src/lib/utils/theme-css-parser.ts
    - frontend/src/lib/utils/__tests__/theme-css-parser.test.ts
  modified:
    - frontend/src/lib/utils/visual-settings-to-css.ts
decisions:
  - PROPERTY_MAP exported from visual-settings-to-css.ts so theme-css-parser.ts can import it directly (safe non-breaking change)
  - CSS_VAR_REGEX defined inside parseCssToVisualSettings function body (fresh regex per call, avoids stale lastIndex)
  - REVERSE_MAP built once at module load as Map<string, keyof VisualSettings> (immutable, no per-call cost)
  - Cast via `(result as Record<string, string>)[field]` avoids `any` while remaining runtime-correct
metrics:
  duration: "~82 seconds"
  completed: "2026-03-18"
  tasks: 2
  files: 3
---

# Phase 36 Plan 02: theme-css-parser Utility Summary

Pure TypeScript utility `parseCssToVisualSettings` that reverse-maps `--chat-*` and `--platform-*` CSS custom property declarations to `Partial<VisualSettings>` fields using an exported PROPERTY_MAP from visual-settings-to-css.ts.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Write failing tests for parseCssToVisualSettings | d05953a | theme-css-parser.test.ts |
| 2 (GREEN) | Implement parseCssToVisualSettings + export PROPERTY_MAP | 7201eeb | theme-css-parser.ts, visual-settings-to-css.ts |

## What Was Built

### PROPERTY_MAP export
- Added `export` keyword to `const PROPERTY_MAP` in `visual-settings-to-css.ts`
- Non-breaking change — existing import of `visualSettingsToCss` unaffected
- All 7 existing tests in visual-settings-to-css.test.ts continue to pass

### parseCssToVisualSettings utility
- Signature: `parseCssToVisualSettings(css: string): Partial<VisualSettings>`
- `REVERSE_MAP`: `Map<string, keyof VisualSettings>` built once at module load from `PROPERTY_MAP`
- `CSS_VAR_REGEX`: defined inside the function body (`/(--(chat|platform)-[\w-]+)\s*:\s*([^;}\n]+?)\s*;/g`) — fresh instance per call
- Known vars: mapped to result with trimmed value via `(result as Record<string, string>)[field] = value`
- Unknown vars: `console.warn('[theme-css-parser] Unknown CSS variable: ' + cssVar)`
- Empty input: returns `{}`
- No `any` types used

## Test Results

- `theme-css-parser.test.ts`: 7/7 passed
  - empty input returns `{}`
  - single known var extracted correctly
  - unknown var triggers `console.warn` and excluded from result
  - full 50-property theme CSS returns object with `Object.keys(result).length === 50`
  - overlayBgColor and overlayBgOpacity parsed as independent fields
  - second call not affected by first call (no stale regex state)
  - values with extra whitespace trimmed
- Full unit suite: 22 test files, 134 tests passed
- TypeScript: no errors (`npx tsc --noEmit`)

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED
