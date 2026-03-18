# Phase 33: CSS Architecture Foundation - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the technical plumbing for the v1.6 Visual Overlay Customizer: DB column, Go model field, TypeScript type, CSS generator utility, and cascade layer declaration. No visible UI in this phase. All subsequent phases (34–37) build on top of this foundation.

</domain>

<decisions>
## Implementation Decisions

### VisualSettings property catalog — CSS variable naming
- **Category-prefixed** CSS custom property names: `--chat-typography-font-family`, `--chat-color-text-body`, `--chat-bg-overlay`, `--chat-bubble-border-radius`, etc.
- Prefix pattern: `--chat-{category}-{property}`
- Categories mirror the Phase 37 collapsible section names: typography, color, bg, bubble, visibility, sizing, platform, events

### VisualSettings property catalog — color+opacity pairs
- Single rgba string field per color+opacity control (e.g., `overlayBackground?: string` → stores `'rgba(0,0,0,0.7)'`)
- UI slider in Phase 34 parses the stored rgba value to extract/set opacity
- Applies to: `overlayBackground`, `bubbleBackground`, and any other color+opacity pairs

### VisualSettings property catalog — visibility toggles
- Claude's discretion: boolean fields (e.g., `showAvatars?: boolean`)
- CSS generator emits `display: none` / `display: block` for false/true values
- Covers: avatars, badges, timestamps, platform badge, emotes, username

### VisualSettings property catalog — special events (APPR-09)
- Per-event-type size modifier fields only (no show/hide fields — event visibility is handled by existing filter settings)
- Fields: `superChatSizeModifier?: number`, `subscriptionSizeModifier?: number`, `raidSizeModifier?: number`
- CSS var mapping: `--chat-events-superchat-size-modifier`, etc.

### TypeScript type structure
- **Nested/grouped interface** — properties grouped by category, each group is an optional sub-object
- Categories: `typography`, `colors`, `background`, `bubbles`, `visibility`, `sizing`, `platformColors`, `events`
- All fields at all levels are optional (`?`)
- Example structure:
  ```ts
  interface VisualSettings {
    typography?: { fontFamily?: string; fontWeight?: number; lineHeight?: number; letterSpacing?: string }
    colors?: { textBody?: string; textUsername?: string; textTimestamp?: string }
    background?: { overlayBackground?: string }
    bubbles?: { bubbleBackground?: string; borderRadius?: number; borderWidth?: number; borderColor?: string; innerPadding?: number; gapBetweenMessages?: number }
    visibility?: { showAvatars?: boolean; showBadges?: boolean; showTimestamps?: boolean; showPlatformBadge?: boolean; showEmotes?: boolean; showUsername?: boolean }
    sizing?: { avatarSize?: number; badgeSize?: number; emoteScale?: number }
    platformColors?: { twitch?: string; youtube?: string; kick?: string; tiktok?: string; discord?: string }
    events?: { superChatSizeModifier?: number; subscriptionSizeModifier?: number; raidSizeModifier?: number }
  }
  ```

### TypeScript update strategy
- Always send the full `VisualSettings` object on save (full replace, no partial merge)
- Backend simply stores what arrives — no deep-merge logic needed
- Simplifies the API and DB interaction

### Cascade layer — rename
- Rename `user-overrides` → `user-css-overrides` across the codebase
- New full layer order: `@layer base, design-system, marketplace-themes, visual-customizer, user-css-overrides`

### Cascade layer — update scope
- Update **both** `frontend/src/styles/events.css` and `frontend/src/app/globals.css`
- Both currently declare `@layer base, design-system, marketplace-themes, user-overrides`
- Both get the updated declaration with `visual-customizer` inserted and `user-overrides` renamed

### Go model field type
- `VisualSettings json.RawMessage \`json:"visual_settings"\`` in `OverlayConfig` struct
- Fully opaque — no Go-side parsing or schema validation
- Perfect round-trip: DB JSONB → raw bytes → JSON response, unchanged

### Go model — NULL handling
- Legacy `overlay_configs` rows without `visual_settings` will have NULL in DB
- API returns `{}` (empty object) for NULL, never `null` or omitted field
- Implementation: `coalesce(visual_settings, '{}')` in SELECT query, or nil-check on `json.RawMessage` returning `json.RawMessage("{}")`

### Claude's Discretion
- Exact DB migration number (next after 040)
- Boolean-to-CSS translation details in the generator (e.g., `false` → `display: none` vs CSS custom property trick)
- Whether `json.RawMessage` null-default uses coalesce in SQL or Go-side nil check
- Exact field count (roadmap says ~50 — adjust based on APPR requirements coverage)

</decisions>

<specifics>
## Specific Ideas

- CSS generator output format is specified in roadmap: `@layer visual-customizer { :root { --chat-*: value } }` — only emit properties that are set (non-undefined)
- The nested TypeScript type was confirmed by the user seeing the preview and choosing it explicitly
- Events section only has size modifiers (no show/hide) because event visibility is already controlled by filter settings

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/overlay-manager/models/config.go`: `OverlayConfig` struct — add `VisualSettings json.RawMessage` field here
- `services/overlay-manager/repository/config_repo.go`: `GetByOverlayID` and `Update` queries — update SELECT/UPDATE to include `visual_settings` column
- `frontend/src/lib/types/overlay.ts`: `OverlayConfig` interface — add `visual_settings?: VisualSettings` field (imported from new type file)
- `frontend/src/styles/events.css`: layer order declaration to update
- `frontend/src/app/globals.css`: layer order declaration to update

### Established Patterns
- DB migrations: numbered SQL files in `migrations/` — next is 041
- Go JSON fields: `map[string]any` used for `DisplaySettings` and `FilterSettings`; `json.RawMessage` is a deliberate departure for opaque passthrough
- TypeScript types: `frontend/src/lib/types/` directory, PascalCase filenames — new file `visual-settings.ts`
- TypeScript utilities: `frontend/src/lib/` directory, camelCase filenames — new file `visual-settings-to-css.ts`
- Vitest used for frontend unit tests — test file co-located or in `__tests__/`

### Integration Points
- `overlay_configs` table: new `visual_settings JSONB DEFAULT '{}'` column
- `PUT /api/v1/overlays/{id}/config` handler in `services/overlay-manager/handlers/config.go` — passes through `visual_settings` without inspection
- Embed page (`frontend/src/app/overlays/[id]/preview/embed/page.tsx`) imports `@/styles/events.css` — cascade layer declaration reaches embed page through this import
- `OverlayConfig` TypeScript type in `overlay.ts` — downstream pages/components that read `custom_css` will later also read `visual_settings`

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 33-css-architecture-foundation*
*Context gathered: 2026-03-18*
