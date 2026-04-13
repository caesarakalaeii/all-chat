---
phase: 09-add-optional-support-for-alejo-pronouns
plan: "02"
subsystem: frontend
tags: [pronouns, overlay, typescript, vitest, tdd]
dependency_graph:
  requires: ["09-01"]
  provides: ["pronoun-pill-rendering", "pronoun-types"]
  affects: ["frontend/overlay-page", "frontend/types"]
tech_stack:
  added: ["frontend/src/lib/utils/pronounPill.ts"]
  patterns: ["helper-function-extract-for-testability", "tdd-red-green", "config-cascade-display-then-visual"]
key_files:
  created:
    - frontend/src/lib/utils/pronounPill.ts
    - frontend/src/app/overlay/__tests__/pronoun-pill.test.tsx
  modified:
    - frontend/src/lib/types/message.ts
    - frontend/src/lib/types/overlay.ts
    - frontend/src/lib/types/visual-settings.ts
    - frontend/src/app/overlay/[id]/page.tsx
    - frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts
decisions:
  - "pronounPill.ts extracted as pure helper for testability — follows usernameSpan.ts pattern"
  - "shouldRenderPronounPill takes targetPosition param — enables both render sites to call the same function"
  - "Config cascade: display_settings loaded first, visual_settings overrides second — matches platformBadge pattern"
  - "visual-settings-to-css test Required<VisualSettings> updated with pronoun fields (non-CSS, not emitted)"
metrics:
  duration: "~3min"
  completed: "2026-04-03T23:12:43Z"
  tasks: 2
  files: 6
---

# Phase 09 Plan 02: Pronoun Pill — Frontend Types and Overlay Rendering Summary

Pronoun pill rendering added to the chat overlay with configurable position (before/after username) and color. TypeScript types extended across three files. Vitest unit tests verify all rendering conditions. Zero visual impact on messages without pronouns.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Extend frontend types for pronouns | 6bfffff | message.ts, overlay.ts, visual-settings.ts, visual-settings-to-css.test.ts |
| 2 | Pronoun pill rendering + config loading + Vitest tests | 8695616 (RED), 08e58bf (GREEN) | pronounPill.ts, pronoun-pill.test.tsx, page.tsx |

## What Was Built

**TypeScript type extensions:**
- `UserInfo.pronouns?: string` — Phase 9 field for enriched pronoun data from the backend
- `DisplaySettings.show_pronouns?: boolean`, `pronoun_position?: 'before' | 'after'`, `pronoun_color?: string`
- `VisualSettings.showPronouns?: 'inline' | 'none'`, `pronounPosition?: 'before' | 'after'`, `pronounColor?: string`

**Pronoun pill rendering in overlay page:**
- State: `showPronouns` (default `true`), `pronounPosition` (default `'after'`), `pronounColor` (default `'#7B68EE'`)
- Config cascade: `display_settings` loaded first, `visual_settings` overrides second
- Two render sites: before and after the username block, each gated by `pronounPosition === 'before'/'after'`
- Pill JSX: `<span className="inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white" style={{ backgroundColor: pronounColor }}>`

**pronounPill.ts helper:**
- `shouldRenderPronounPill(showPronouns, pronouns, position, targetPosition): boolean`
- `getPronounPillProps(pronouns, color): { text, className, style }`

**Vitest tests (14 passing):**
- `shouldRenderPronounPill`: renders when all conditions met; does not render when pronouns undefined, showPronouns false, or position mismatch
- `getPronounPillProps`: correct text, color, CSS classes
- Default values verified: show=true, position=after, color=#7B68EE

## Deviations from Plan

**1. [Rule 2 - Missing critical functionality] Updated visual-settings-to-css.test.ts**
- **Found during:** Task 1
- **Issue:** Adding three new fields to `VisualSettings` broke the existing `Required<VisualSettings>` exhaustiveness test in `visual-settings-to-css.test.ts` — TypeScript reported a compile error
- **Fix:** Added `showPronouns: 'inline'`, `pronounPosition: 'after'`, `pronounColor: '#7B68EE'` to the `full` object in the test. Count assertion (`toBe(51)`) unchanged since new fields are non-CSS and not emitted.
- **Files modified:** `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts`
- **Commit:** 6bfffff (included in Task 1 commit)

## Known Stubs

None. Pronoun data flows from `message.user.pronouns` which is populated by the backend enrichment in Plan 01. When the field is absent (no pronouns set), the pill simply does not render — no placeholder or empty space.

## Self-Check: PASSED

Files exist:
- `frontend/src/lib/utils/pronounPill.ts` — FOUND
- `frontend/src/app/overlay/__tests__/pronoun-pill.test.tsx` — FOUND

Commits exist:
- 6bfffff — FOUND (feat: extend frontend types)
- 8695616 — FOUND (test: RED failing tests)
- 08e58bf — FOUND (feat: GREEN implementation)
