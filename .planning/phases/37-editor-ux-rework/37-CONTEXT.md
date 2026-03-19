# Phase 37: Editor UX Rework - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Restructure the overlay editor panel into 5 labeled collapsible sections in order: Theme → Appearance → Sources → Behavior → Expert. Theme Marketplace moves from a modal trigger in the CSS editor row to the first and primary inline section. AppearancePanel (all 7 sub-groups) moves into the Appearance section. Sources management moves inline into the Sources section. Behavior settings remain as-is but regrouped. CSS editor (Monaco) moves into the Expert section, which is collapsed by default.

</domain>

<decisions>
## Implementation Decisions

### Theme section format
- **Inline embedded content** — `CollapsibleSection` titled "Theme" expands to show theme cards directly in the editor panel (no modal)
- Reuse existing `ThemeMarketplaceModal` content: extract the theme list/grid and render it inline, compact sizing
- "Reset to theme defaults" button (from Phase 36) lives **inside the Theme section** alongside the theme list
- Theme section starts **expanded by default** — it's the primary entry point for the v1.6 customizer
- The existing `ThemeMarketplaceModal` component and "Browse Themes" button in the CSS editor row are removed

### Section open/close defaults
- **Theme**: expanded by default
- **Sources**: expanded by default
- **Appearance**: collapsed by default
- **Behavior**: collapsed by default
- **Expert**: collapsed by default (roadmap requirement — CSS editor hidden by default)
- State persisted in **localStorage** using a new key: `editor-panel-sections-v1` (separate from `appearance-panel-sections-v1` used by AppearancePanel sub-groups)

### Sources section content
- **Inline add/remove controls** — full source management in the editor panel, no navigation needed
- Shows list of configured sources with platform icon + channel name + ✕ remove button per row
- "Add Source" expands an **inline form** within the section: platform selector + channel input fields
- Removing a source is **immediate** (no confirmation prompt)

### Behavior section scope
- **In Behavior**: Max Messages, Message Duration, Disable Message Fade, Platform Badge settings (show/hide + position + style), Emote Providers (7TV, BTTV, FFZ)
- **Moved to Appearance** (Typography sub-group): Font Size slider (was a standalone control)
- **Moved to Expert**: Mock Messages (inject messages, sample chat/events, clear)
- **Stats** (message count, connection status): stays in Behavior section

### Expert section contents
- CSS editor (MonacoCSSEditor) with Enable/Disable toggle
- Mock Messages testing tools (inject, sample chat, sample events, clear)

### Save button placement
- **Sticky footer** — pinned at the bottom of the editor panel sidebar, always visible regardless of which sections are open

### Font Size placement
- Merged into the existing **Typography sub-group** inside AppearancePanel — consistent with other typography properties (font family, weight, line height, letter spacing)
- The legacy standalone `fontSize` state in the editor page maps to `visualSettings.typography.fontSize`

### CollapsibleSection reuse
- Use the existing `CollapsibleSection` component from `frontend/src/components/appearance/CollapsibleSection.tsx`
- Top-level section IDs: `"theme"`, `"appearance"`, `"sources"`, `"behavior"`, `"expert"`
- Override default (closed) behavior per-section: pass `defaultOpen` prop or handle in the component via the `editor-panel-sections-v1` key

</decisions>

<specifics>
## Specific Ideas

- Theme section is the MVP entry point for new users — showing it open by default guides them to start with a theme before tweaking
- Sources starts open so users can see what's configured at a glance
- The "sticky footer" Save button ensures users always know how to save without hunting for it

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/appearance/CollapsibleSection.tsx`: use for all 5 top-level sections; currently uses `appearance-panel-sections-v1` storage key — needs a prop or secondary component to support `editor-panel-sections-v1` key
- `frontend/src/components/appearance/AppearancePanel.tsx`: drop into the Appearance section as-is
- `frontend/src/components/MonacoCSSEditor.tsx`: drop into the Expert section
- `frontend/src/components/theme-marketplace/ThemeMarketplaceModal.tsx`: extract inline-renderable content component from it for the Theme section; modal wrapper can be removed
- `frontend/src/app/overlays/[id]/preview/page.tsx`: the editor panel sidebar (line ~1230–1480) is where all restructuring happens

### Established Patterns
- `CollapsibleSection` uses `@base-ui/react` Collapsible primitive — extend to support configurable storage key or section-level defaults
- Sources API: `overlaysApi.getSources(id)` and `overlaysApi.addSource(id, {...})` / `overlaysApi.deleteSource(id, sourceId)` — already used in the page
- Editor page state: `useState` for all settings, `handleSaveCustomization()` for saving — keep this pattern unchanged

### Integration Points
- Editor panel sidebar starts at line ~1230 in `page.tsx` — replace inner content with 5 `CollapsibleSection` wrappers
- `ThemeMarketplaceModal` is referenced at line 1592 — remove modal invocation, replace with inline Theme section content
- `showThemeMarketplace` state and `setShowThemeMarketplace` — can be removed after Theme section goes inline
- `sources` state (line 456): already loaded for mock messages; reuse for Sources section display
- `fontSize` state: wire to `visualSettings.typography.fontSize` and remove standalone slider

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 37-editor-ux-rework*
*Context gathered: 2026-03-19*
