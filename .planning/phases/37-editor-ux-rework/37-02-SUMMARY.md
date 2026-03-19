---
phase: 37-editor-ux-rework
plan: 02
subsystem: ui
tags: [react, nextjs, collapsible-sections, theme-marketplace, editor-panel]

requires:
  - phase: 37-editor-ux-rework-01
    provides: CollapsibleSection with storageKey/defaultOpen props, ThemeContent component

provides:
  - Overlay editor panel restructured into 5 CollapsibleSections with editor-panel-sections-v1 storage key
  - Theme section (first, open by default) with inline ThemeContent and Reset button
  - Appearance section wrapping AppearancePanel
  - Sources section with source list, accepted shares, and AddSourceForm inline
  - Behavior section with font size, message controls, platform badge, and emote providers
  - Expert section with Custom CSS editor (MonacoCSSEditor) and Mock Messages
  - Sticky Save footer always visible at bottom of editor panel
  - handleThemeApply function replacing ThemeMarketplaceModal.onApplyTheme logic
  - ThemeMarketplaceModal fully removed from overlay editor

affects:
  - 37-editor-ux-rework-03
  - overlay editor UX

tech-stack:
  added: []
  patterns: [CollapsibleSection with separate storageKey per panel (editor-panel-sections-v1 vs appearance-panel-sections-v1)]

key-files:
  created: []
  modified:
    - frontend/src/app/overlays/[id]/page.tsx

key-decisions:
  - "ThemeContent rendered inline in Theme CollapsibleSection — no modal needed (ThemeMarketplaceModal fully removed)"
  - "handleThemeApply function extracted from ThemeMarketplaceModal.onApplyTheme inline handler"
  - "Sticky footer uses position:sticky bottom-0 — works because split-view-config has overflow-y-auto"
  - "Font size slider stays in Behavior section for this plan — Plan 03 will migrate it to TypographyGroup"
  - "Browse Themes button removed from Custom CSS header (ThemeContent now inline in Theme section)"

patterns-established:
  - "Editor top-level sections use storageKey=editor-panel-sections-v1, distinct from AppearancePanel's appearance-panel-sections-v1"
  - "5-section order: Theme (open), Appearance, Sources (open), Behavior, Expert — visual-first then advanced"

requirements-completed: [EDUX-01, EDUX-02, EDUX-03]

duration: 15min
completed: 2026-03-19
---

# Phase 37 Plan 02: Editor Panel Restructure Summary

**Overlay editor panel converted from flat card layout to 5 collapsible sections (Theme, Appearance, Sources, Behavior, Expert) with sticky Save footer and ThemeMarketplaceModal fully removed**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-19T10:20:00Z
- **Completed:** 2026-03-19T10:35:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Replaced flat scrollable card layout with 5 labeled CollapsibleSection wrappers using `storageKey="editor-panel-sections-v1"`
- Theme section is first and open by default — contains inline ThemeContent (dynamic import) + Reset to theme defaults button
- Expert section is last and closed by default — contains MonacoCSSEditor + Mock Messages (both demoted from top-level cards)
- Save button moved to `position: sticky; bottom: 0` footer always visible in the editor panel
- ThemeMarketplaceModal and all `showThemeMarketplace` state references fully removed

## Task Commits

1. **Task 1: Inspect SplitView and add ThemeContent dynamic import** - `16d4742` (feat)
2. **Task 2: Restructure editor panel into 5 CollapsibleSections with sticky footer** - `34bdd02` (feat)

## Files Created/Modified

- `frontend/src/app/overlays/[id]/page.tsx` - Overlay editor page with 5-section collapsible panel replacing flat card layout

## Decisions Made

- ThemeContent rendered inline in Theme CollapsibleSection — eliminates the modal entirely, no need to preserve ThemeMarketplaceModal
- handleThemeApply extracted as named function (was inline in ThemeMarketplaceModal's onApplyTheme prop) — same logic: check for existing visual settings, prompt confirm or apply immediately
- Sticky footer uses `position: sticky; bottom: 0` — confirmed valid because SplitView's config panel uses `overflow-y-auto` on `.split-view-config` class, making it the scroll container
- Font size slider kept in Behavior section (not migrated to TypographyGroup) — Plan 03 handles that migration
- Browse Themes button removed from Custom CSS header since ThemeContent is now the canonical entry point

## Deviations from Plan

None — plan executed exactly as written. The one minor clarification was that the `showThemeMarketplace` state had a lingering reference in the Browse Themes button within the existing CSS section; this was removed as part of Task 1 (plan specified removing all references).

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 37-02 complete — editor has the 5-section structure with correct storage key
- Plan 03 can proceed: migrate font size slider from Behavior to TypographyGroup inside Appearance section
- All CollapsibleSection state (open/closed) persists independently via `editor-panel-sections-v1` key in localStorage

---
*Phase: 37-editor-ux-rework*
*Completed: 2026-03-19*
