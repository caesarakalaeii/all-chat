# ADR-0042: Overlay Editor Settings Navigation (Left Nav Replaces Stacked Drawers)

**Date**: 2026-07-23
**Status**: Accepted
**Revised**: 2026-11-18 (trimmed; record what shipped and what did not)
**Deciders**: caesarakalaeii

## Context

The overlay editor presented all configuration as a vertical stack of `CollapsibleSection` accordion drawers in the config column, nested: seven top-level drawers, one of which ("Appearance") contained eight to ten more. Opening a drawer reflowed everything below it, and the nesting persisted state under two different localStorage keys. External usability feedback (2026-07, from a fellow overlay-tool developer) named the failure modes: disorientation ("drawers stacked on top of each other"), poor findability (a user told "you can remove the badge" could not locate the toggle behind a collapsed drawer with no search), and no usage tiering (rarely-touched controls sat visually equal to theme and sources).

The customizer state layer was already decoupled from presentation — all controls feed local React state flushed by one `handleSaveConfiguration()`, and the preview streams via `postMessage` — so the UI grouping could change without any API or backend change.

## Decision

**Left nav + one-section-at-a-time panel + settings search + per-section advanced disclosure**, inside the existing `SplitView` config column.

Alternatives considered: keep the drawer stack and add search only (leaves nesting and reflow); un-nest Appearance into the stack (a 16-drawer scroll, same reflow); a full-page tabbed layout (breaks the side-by-side live-preview loop).

Key sub-decisions:

- **Flat, first-class sections.** The Appearance sub-groups (Typography, Colors, Background & Bubbles, Visibility, Sizing, Platform Colors, Events, Filters, Sounds, Text-to-Speech) become top-level nav entries; `AppearancePanel` is dissolved but its group components are kept and rendered directly by the panel.
- **Nav grouping encodes workflow**: Setup (Theme, Sources, Moderators, Testing), Appearance, Behavior (Messages, Filters, Sounds, Text-to-Speech, Engagement), Advanced (Custom CSS, Danger Zone). The mock message/event injector moves to a first-class **Testing** section under Setup — positioning the overlay in OBS with mock traffic is setup, not expert work.
- **Search is a static registry, not DOM scraping.** A `sectionRegistry` declares each section's id, group, title and searchable control entries (label + synonyms + anchor id). A result activates the section, scrolls to the anchor, expands the Advanced disclosure if needed and flash-highlights the control. Registry entries are colocated with section definitions and reviewed in the same diff.
- **Advanced tiering is presentation-only.** An `AdvancedDisclosure` (collapsed by default, not persisted) wraps low-traffic controls; no schema flag or gate.
- **Onboarding spotlight targets the nav.** Drawer `forceOpen` is replaced by steering the active section while the first-run guide is active (one-shot navigation, not a lock). `CollapsibleSection` had no consumers left and is deleted with its test; the two old localStorage keys are retired, with only `editor-active-section-v1` persisted for the last active section.
- **Section JSX stays in the editor page for now.** The plan to move sections into `frontend/src/components/editor/sections/` was not carried out: groups still render inline in `frontend/src/app/overlays/[id]/page.tsx`, composing components from `frontend/src/components/appearance/`. Decomposition stays opportunistic.

## Consequences

- Named settings are findable via search; geometry is stable (the nav never moves); every section is permanently visible; screen-reader heading navigation maps one section to one heading.
- One section at a time means cross-section comparison requires switching (one click, never reflows). The nav consumes ~160px of the config column (user-resizable 25–70%; collapses to horizontal chips below the container breakpoint). The search registry must be maintained alongside new controls.
- Out of scope: the Event Settings and Credits pages keep their own routes; the monitor view (ADR-0013) is unaffected; no backend change.
