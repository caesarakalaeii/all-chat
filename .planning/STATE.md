---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Visual Overlay Customizer
status: planning
stopped_at: Completed 36-02-PLAN.md
last_updated: "2026-03-18T14:24:22.195Z"
last_activity: 2026-03-18 — v1.6 roadmap created, 5 phases (33-37), 18 requirements mapped
progress:
  total_phases: 23
  completed_phases: 21
  total_plans: 78
  completed_plans: 77
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-18)

**Core value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.
**Current focus:** v1.6 Visual Overlay Customizer — Phase 33 (CSS Architecture Foundation)

## Current Position

Phase: 33 of 37 (CSS Architecture Foundation)
Plan: — (not yet planned)
Status: Ready to plan
Last activity: 2026-03-18 — v1.6 roadmap created, 5 phases (33-37), 18 requirements mapped

Progress: [░░░░░░░░░░] 0% (v1.6 — 0 plans complete)

## Performance Metrics

**Velocity (prior milestones):**
- v1.0: 11 plans (3 phases)
- v1.1: 21 plans (7 phases)
- v1.2: 21 plans (12 phases)
- v1.3: 20 plans (4 phases)
- v1.5: 15 plans (6 phases)

**By Milestone:**

| Milestone | Phases | Plans | Status |
|-----------|--------|-------|--------|
| v1.0 Message Deletion | 1-3 | 11 | Complete |
| v1.1 Load Balancing | 4-10 | 21 | Complete |
| v1.2 InnerTube Listener | 11-22 | 21 | Complete |
| v1.3 Frontend Redesign | 23-26 | 20 | Complete |
| v1.5 Discord Listener | 27-32 | 15 | Complete |
| v1.6 Visual Overlay Customizer | 33-37 | TBD | Not started |

*Updated: 2026-03-18 after milestone start*
| Phase 33 P02 | 8 | 3 tasks | 3 files |
| Phase 34-appearance-controls-core P02 | 15 | 3 tasks | 4 files |
| Phase 34 P01 | 9m | 2 tasks | 12 files |
| Phase 34 P03 | 15 | 2 tasks | 7 files |
| Phase 36-events-styling-theme-import P01 | 3 | 2 tasks | 7 files |
| Phase 36-events-styling-theme-import P02 | 82 | 2 tasks | 3 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Key decisions relevant to v1.6:

- **Cascade layer order**: `base → design-system → marketplace-themes → visual-customizer → user-css-overrides` — visual editor overrides theme defaults; raw CSS still wins
- **visual_settings as JSONB**: Stored in `overlay_configs` table; no new table needed; existing `PUT /api/v1/overlays/{id}/config` handles new field
- **Theme import via CSS parsing**: When user loads a theme, parse its `--chat-*` CSS custom properties to pre-populate visual controls
- **CSS generator utility**: `visual-settings-to-css.ts` converts VisualSettings JSON → `@layer visual-customizer { :root { --chat-*: value } }` block
- **No new stack additions**: Frontend only (TypeScript/React); backend adds one DB column + one model field
- [Phase 33]: VisualSettings visibility fields use union literal types ('inline'|'none', 'block'|'none') for direct CSS value mapping without runtime transformation
- [Phase 33]: visualSettingsToCss returns empty string for empty/undefined-only input (not empty CSS block) to allow callers to skip injection
- [Phase 34-appearance-controls-core]: Google Font names duplicated in embed page (not imported from FontFamilyCombobox) — embed and component are in different routing contexts
- [Phase 34-appearance-controls-core]: style#visual-customizer-style managed imperatively via DOM to avoid re-render overhead on CSS updates
- [Phase 34]: CollapsibleSection uses localStorage key appearance-panel-sections-v1 for section open/close persistence
- [Phase 34]: FontFamilyCombobox requires Combobox.Portal wrapper for Positioner context (base-ui v1.2.0 requirement)
- [Phase 34]: Opacity stored as decimal string ('0.0'–'1.0'); opacity slider uses 0-100 int range and converts on change
- [Phase 36]: EventsGroup visual-customizer CSS fallback 1.05 matches marketplace-themes baseline to prevent visual regression
- [Phase 36-events-styling-theme-import]: PROPERTY_MAP exported from visual-settings-to-css.ts so theme-css-parser.ts can import it directly
- [Phase 36-events-styling-theme-import]: CSS_VAR_REGEX defined inside parseCssToVisualSettings function body (fresh regex per call, avoids stale lastIndex)

### Pending Todos

None yet.

### Blockers/Concerns

None identified at milestone start.

## Session Continuity

Last session: 2026-03-18T14:24:22.192Z
Stopped at: Completed 36-02-PLAN.md
Resume file: None

**Next action:** Phase 33 complete — proceed to Phase 34 (Appearance Controls — Core)
