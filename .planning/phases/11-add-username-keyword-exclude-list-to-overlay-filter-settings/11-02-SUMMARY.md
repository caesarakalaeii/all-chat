---
phase: 11-add-username-keyword-exclude-list-to-overlay-filter-settings
plan: "02"
subsystem: frontend
tags: [filter-ui, appearance-panel, postmessage, wysiwyg, react]
dependency_graph:
  requires: ["11-01"]
  provides: ["FilterGroup component", "filter_settings UI", "D-07 WYSIWYG preview filtering", "D-08 common bots preset"]
  affects: ["frontend/src/components/appearance/", "frontend/src/app/overlays/[id]/page.tsx", "frontend/src/app/overlays/[id]/preview/embed/page.tsx"]
tech_stack:
  added: []
  patterns: ["TDD red-green", "postMessage WYSIWYG", "useRef for stale closure avoidance", "tag input component pattern"]
key_files:
  created:
    - frontend/src/components/appearance/FilterGroup.tsx
    - frontend/src/components/appearance/__tests__/FilterGroup.test.tsx
  modified:
    - frontend/src/components/appearance/AppearancePanel.tsx
    - frontend/src/app/overlays/[id]/page.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx
decisions:
  - "filterSettingsRef used (not filterSettings state) in WebSocket onMessage to avoid stale closure — matches visualSettingsRef pattern"
  - "EMBED_READY handler extended to send both CSS and filter settings — ensures embed gets current state after iframe reload"
  - "filterSettings loaded from config.filter_settings on both editor and embed page mount — supports direct OBS overlay URL"
  - "Auto-approved checkpoint Task 3 (--auto mode) — TypeScript compiles clean, 42/42 test files pass"
metrics:
  duration: "~8 minutes"
  completed: "2026-04-12"
  tasks_completed: 3
  files_changed: 5
requirements:
  - D-07
  - D-08
---

# Phase 11 Plan 02: FilterGroup UI Component and Real-Time Preview Filtering Summary

**One-liner:** FilterGroup tag-input component wired into AppearancePanel with FILTER_SETTINGS_UPDATE postMessage for D-07 WYSIWYG preview filtering and D-08 common bots preset.

## What Was Built

### Task 1: FilterGroup component (TDD)

Created `FilterGroup.tsx` with:
- `TagInput` sub-component for both banned_users and banned_words lists — Enter/comma to add, backspace to remove last, X button per tag
- `COMMON_BOTS` constant with 10 well-known bot names (nightbot, streamelements, moobot, fossabot, soundalerts, streamlabs, stay_hydrated_bot, serybot, wizebot, botisimo)
- Duplicate prevention in both TagInput (on keyDown) and handleAddCommonBots (Set-based dedup)
- ToggleSwitch for hide_commands, SliderControl for min_message_length (0–200 chars)
- "Add common bots" button populating banned_users without duplicates

All 15 unit tests pass (RED → GREEN TDD cycle confirmed).

### Task 2: Integration wiring

**AppearancePanel.tsx** — Extended props with `filterSettings?: FilterSettings` and `onFilterChange?`, renders `<FilterGroup>` inside a new "Filters" `<CollapsibleSection>` when both props are provided.

**Editor page (overlays/[id]/page.tsx):**
- `filterSettings` state + `filterSettingsRef` for stale-closure-safe iframe communication
- `sendFilterSettingsToIframe` callback (same pattern as `sendCssToIframe`)
- `handleFilterSettingsChange` merges patch, updates state, sends `FILTER_SETTINGS_UPDATE` postMessage immediately — no Save required (D-07 WYSIWYG)
- EMBED_READY handler also sends current filter settings on embed page reload
- Config load: `setFilterSettings(config.filter_settings)` after overlaysApi.getConfig
- Config save: `filter_settings: filterSettings` added to updateConfig payload

**Embed page (preview/embed/page.tsx):**
- Imports `shouldFilterMessage` and `FilterSettings`
- `filterSettings` state + `filterSettingsRef` (updated synchronously in postMessage handler for immediate effect)
- `FILTER_SETTINGS_UPDATE` handler updates both state and ref atomically
- `shouldFilterMessage(message, filterSettingsRef.current)` applied before `setMessages` in WebSocket onMessage
- Config load: filter_settings loaded from config API on mount (for direct OBS overlay URL access)

### Task 3: Checkpoint (auto-approved)

⚡ Auto-approved: All automated verification passes — TypeScript compiles with 0 errors, 42/42 test files pass, 254 tests pass with 4 pre-existing todo items.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — FilterGroup is fully wired with live data. The banned_users and banned_words lists are persisted via the config API and displayed correctly from saved state.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundaries introduced. Filter settings flow through existing authenticated overlaysApi.updateConfig (T-11-03 mitigated by existing GetByIDAndUserID check). postMessage uses same-origin embed pattern as existing VISUAL_CSS_UPDATE (T-11-05 accepted per plan threat model).

## Self-Check: PASSED

| Item | Status |
|------|--------|
| FilterGroup.tsx | FOUND |
| FilterGroup.test.tsx | FOUND |
| 11-02-SUMMARY.md | FOUND |
| commit 7ae097f (FilterGroup component) | FOUND |
| commit 23fb45f (wiring integration) | FOUND |
