# ADR-0042: Overlay Editor Settings Navigation (Left Nav Replaces Stacked Drawers)

**Date**: 2026-07-23
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The overlay editor (`frontend/src/app/overlays/[id]/page.tsx`) presents all overlay configuration as a vertical stack of `CollapsibleSection` accordion drawers inside the split-view config column. The stack is **nested**: seven top-level drawers, one of which ("Appearance") contains eight to ten more drawers (`AppearancePanel`), putting many controls two to three accordion levels deep. Opening a drawer pushes everything below it down, and the two nesting levels persist open/closed state under two different localStorage keys.

External usability feedback (2026-07, from a fellow overlay-tool developer) identified the concrete failure modes:

1. **Disorientation** — "drawers stacked on top of each other" is how users get lost; open panels reflow the whole column.
2. **Poor findability** — a user who was *told* a setting exists ("you can remove the badge from the overlay") could not locate it. The toggle sits at Appearance → Visibility, behind a collapsed-by-default drawer, and the word "badge" matches two different toggles once there. There is no search.
3. **No usage tiering** — rarely-touched controls (letter spacing, backdrop blur, TTS throttling) sit visually equal to controls everyone changes (theme, sources, message duration).

The customizer state layer is already decoupled from presentation: all controls feed local React state flushed by a single `handleSaveConfiguration()` → `updateConfig` call, and the live preview streams via `postMessage` driven by state. The UI grouping can therefore change without touching any API contract or backend service.

## Decision Drivers

- **Findability first.** A user must be able to find a setting they can name, without knowing our grouping taxonomy.
- **Stable geometry.** Navigating settings must not reflow other settings; the user's spatial memory of the page should survive interaction.
- **Flat discoverability.** Every section should be *visible* (as a nav entry) even when not *open*; hidden-inside-collapsed-drawer is how the badge toggle got lost.
- **Usage tiering.** Frequently-changed settings easy to reach; long-tail controls present but out of the default eye-line (explicit request from the feedback).
- **No backend churn.** The save/load contract (`display_settings`, `visual_settings`, `filter_settings`, …) and the preview `postMessage` protocol must not change.
- **Onboarding continuity.** The first-run setup guide spotlights the Theme/Sources/Appearance sections (`forceOpen`); the redesign must keep an equivalent spotlight mechanism.
- **Accessibility.** Keep the WCAG heading-navigation structure (ADR-scoped contracts in docs/ACCESSIBILITY.md) and keyboard operability at least as good as the drawers.

## Considered Options

1. **Left nav + single visible section panel, with settings search and per-section advanced disclosure.** A slim vertical nav (grouped: Setup / Appearance / Behavior / Advanced) inside the existing config column lists every section flat; exactly one section renders at a time; a search field indexes control labels and synonyms and jumps to the owning section, highlighting the control; low-traffic controls sit behind a collapsed "Advanced" disclosure per section.
2. **Keep the drawer stack, add search only.** Cheapest fix for findability, but leaves the nesting and reflow problems (drivers 2–3) untouched; search would deep-open nested drawers, making geometry *less* stable.
3. **Un-nest the drawers (flatten Appearance into the top level) but keep the accordion stack.** Removes the worst nesting but the column becomes a 16-drawer stack — more scrolling, same reflow-on-open disorientation, still no tiering.
4. **Move settings to a full-page tabbed layout (separate route per section).** Maximum room per section, but breaks the side-by-side live-preview editing loop that the split view exists for, and turns every settings change into navigation.

## Decision Outcome

**Chosen**: **Option 1** — left nav + one-section-at-a-time panel + settings search + per-section advanced disclosure, all inside the existing `SplitView` config column.

**Rationale**: it is the only option that addresses all three reported failure modes (drivers 1–4) while preserving the live-preview editing loop and the state/save layer untouched (driver 5). Option 2 fixes findability but not disorientation; Option 3 fixes nesting but not reflow; Option 4 sacrifices the split view. The approach was validated with an interactive mockup against the reporter's specific failure case (finding the badge toggle: search "badge" → two results with breadcrumbs → one click).

### Key sub-decisions

- **Sections become flat and first-class.** `AppearancePanel`'s nested groups (Typography, Colors, Background & Bubbles, Visibility, Sizing, Platform Colors, Events, Filters, Sounds, Text-to-Speech) are promoted to top-level nav entries. `AppearancePanel` is dissolved; the group components (`TypographyGroup` et al.) are kept as-is and rendered directly by the section panel.
- **Nav grouping encodes workflow, not implementation.** Groups: **Setup** (Theme, Sources, Testing), **Appearance** (visual groups), **Behavior** (Messages, Filters, Sounds, Text-to-Speech, Engagement), **Advanced** (Custom CSS, Danger Zone). The mock message/event injector moves from the "Expert" drawer to a first-class **Testing** section under Setup: positioning the overlay in OBS with mock traffic is a setup activity every streamer performs, not an expert feature.
- **Search is a static registry, not DOM scraping.** A `sectionRegistry` module declares each section's id, group, title, and searchable control entries (label + synonym keywords + anchor id). Search filters this registry; selecting a result activates the section, scrolls to the anchored control, expands its Advanced disclosure if needed, and flash-highlights it. Registry entries are colocated with section definitions so a new control's registry entry is reviewable in the same diff.
- **Advanced tiering is presentation-only.** An `AdvancedDisclosure` component (collapsed by default, not persisted) wraps low-traffic controls within their section. No schema flag, no feature gate: which controls are "advanced" is a UI judgment revisable without migration.
- **Onboarding spotlight targets the nav.** `forceOpen` forcing of drawer state is replaced by steering the *active section* while the guide is active (a one-shot navigation, not a lock); the user's persisted last-active section is untouched. `CollapsibleSection` had no consumers left after the editor migrated, so it is deleted with its test.
- **The editor page sheds its section JSX.** Each section's panel content moves toward dedicated components under `frontend/src/components/editor/sections/`, shrinking the 3,600-line page. This is done opportunistically per section, not as a blocking rewrite.
- **localStorage keys `editor-panel-sections-v1` / `appearance-panel-sections-v1` are retired.** The nav persists only the last active section (`editor-active-section-v1`). Stale keys are left in place (harmless) rather than migrated.

### Consequences

- **Positive**: named settings are findable via search; geometry is stable (nav never moves); every section is permanently visible in the nav; long-tail controls stop competing with everyday ones; the monolithic editor page gets decomposed; screen-reader heading navigation maps one section = one `h2`.
- **Negative / accepted**: one section visible at a time means cross-section comparison requires switching (mitigated: switching is one click and never reflows); the nav consumes ~160px of the config column (mitigated: the column is user-resizable 25–70% and the nav collapses to horizontal chips below the container breakpoint); the search registry must be maintained alongside new controls (mitigated: colocated declaration, reviewed in the same PR).
- **Out of scope**: the separate Event Settings and Credits pages keep their own routes; the overlay Monitor View (ADR-0013) is unaffected; no backend change.
