# Phase 11: Add username/keyword exclude list to overlay filter settings - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Enable streamers to configure message filtering for their overlays — block specific usernames, keywords/phrases, bot commands, and short messages from appearing in the chat overlay. Adds a Filters section to the overlay editor's AppearancePanel with tag-style inputs for usernames and keywords, a hide-commands toggle, and a minimum message length control. Settings persist via the existing overlay config API.

</domain>

<decisions>
## Implementation Decisions

### Filtering Location
- **D-01:** Client-side filtering only — filter messages in the overlay page JavaScript before rendering. Message-processor stays stateless and does not fetch per-overlay configs.
- **D-02:** The overlay page (`frontend/src/app/overlay/[id]/page.tsx`) applies FilterSettings from the config to incoming WebSocket messages before adding them to the render queue.

### Matching Behavior
- **D-03:** Keywords (`banned_words`) support regex patterns. Since filtering is client-side only, performance and regex complexity are non-concerns. Power users can use regex; simple users can type plain strings (which work as-is since literal strings are valid regex).
- **D-04:** Username matching (`banned_users`) is exact match, case-insensitive. Banning "nightbot" blocks "Nightbot" and "NIGHTBOT" but not "nightbot_fan".
- **D-05:** `hide_commands` suppresses messages starting with `!` (standard bot command prefix).
- **D-06:** `min_message_length` filters messages shorter than the threshold (0 = disabled).

### Editor Preview
- **D-07:** The overlay editor preview applies filters in real-time (WYSIWYG). Filtered messages do not appear in the preview, matching live overlay behavior. Streamers can test filters immediately.

### Default Presets
- **D-08:** "Add common bots" quick-add button that populates the banned_users list with Nightbot, StreamElements, Moobot, Fossabot, SoundAlerts, and other widely-used bots. Users can remove any they don't want after adding.

### Claude's Discretion
- Exact UI layout and spacing of the Filters section within AppearancePanel
- Tag/chip input component implementation details
- Order of filter checks (username → keyword → commands → length, or any other order)
- Error handling for invalid regex patterns (inline validation or silent skip)
- The specific bot names included in the preset list

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Types and Models
- `frontend/src/lib/types/overlay.ts` — FilterSettings interface (lines 54-59), OverlayConfig (line 26)
- `services/overlay-manager/models/config.go` — Backend config model with FilterSettings as map[string]any (line 10)

### API Layer
- `frontend/src/lib/api/overlays.ts` — updateConfig() method for persisting overlay settings
- `services/overlay-manager/handlers/config.go` — Config update handler, FilterSettings merge logic (lines 85-102)

### UI Components
- `frontend/src/components/appearance/AppearancePanel.tsx` — Parent panel where Filters section will be added
- `frontend/src/components/appearance/CollapsibleSection.tsx` — Collapsible section component used by all groups
- `frontend/src/components/appearance/ToggleSwitch.tsx` — Toggle component for hide_commands
- `frontend/src/components/appearance/SliderControl.tsx` — Slider component for min_message_length

### Overlay Page (filtering logic)
- `frontend/src/app/overlay/[id]/page.tsx` — Live overlay page with WebSocket message handler
- `frontend/src/app/overlays/[id]/page.tsx` — Overlay editor page with preview

### Database
- `migrations/001_initial_schema.sql` — overlay_configs table with filter_settings JSONB column (line 42)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `CollapsibleSection` component: Used by all AppearancePanel groups (Typography, Colors, Background, etc.) — use for the new Filters section
- `ToggleSwitch` component: Already used for boolean settings — reuse for `hide_commands`
- `SliderControl` component: Already used for font_size and other numeric settings — reuse for `min_message_length`
- `updateConfig()` API: Already handles partial OverlayConfig updates including filter_settings — no API changes needed

### Established Patterns
- Each AppearancePanel section is a separate `*Group.tsx` component (e.g., `TypographyGroup.tsx`, `ColorsGroup.tsx`) imported into `AppearancePanel.tsx`
- Config changes propagate via `onChange` callback from group → panel → editor page → API call
- The overlay editor page loads config at startup and applies display_settings/visual_settings — filter_settings needs to be loaded the same way

### Integration Points
- **AppearancePanel.tsx**: Import and render new `FilterGroup.tsx` as a collapsible section
- **Overlay editor page** (`overlays/[id]/page.tsx`): Load filter_settings from config, pass to AppearancePanel, apply to preview messages
- **Overlay page** (`overlay/[id]/page.tsx`): Apply filter_settings to WebSocket messages in the `onmessage` handler before adding to message state

</code_context>

<specifics>
## Specific Ideas

- Regex support for keywords is intentional — streamers who know regex get power filtering, streamers who don't just type plain words (which are valid regex)
- The "Add common bots" button should feel quick and reversible — one click to populate, easy to remove individual entries
- Preview filtering should be seamless — no toggle to enable/disable, filters just apply as soon as they're configured

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-add-username-keyword-exclude-list-to-overlay-filter-settings*
*Context gathered: 2026-04-12*
