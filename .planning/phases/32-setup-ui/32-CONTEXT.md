# Phase 32: Setup UI - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Frontend-only phase: configure Discord sources end-to-end without leaving the app. Covers the settings page Discord server connect card, the overlay editor add-Discord-source flow (2-step guild → channel picker), the Discord source card design (with connection status + relay indicators), and the per-source relay config panel (toggle + outbound channel picker + filter indicator). No new backend services; all API endpoints provided by previous phases (27–31).

</domain>

<decisions>
## Implementation Decisions

### Settings page Discord card

- Card section in the settings page (same card-per-section layout as Profile, Data & Privacy) with heading "Discord"
- **Disconnected state:** A `Card` containing a "Connect Discord Server" button that calls `startOAuth` — consistent with existing OAuth button pattern in `AddSourceForm`
- **Connected state per guild:** Server icon (32–48px avatar-style, matching `profile_image_url` pattern), server name, and an inline "Disconnect" button with a confirmation `Dialog` — same pattern as "Remove source" confirmation in the overlay editor
- Multiple guilds supported: render one server row per connected guild (not limited to first)
- Disconnect: inline in the connected card (with Dialog confirmation), not in Danger Zone

### Add Discord source flow

- **Entry point in `AddSourceForm`:** A "Connect Discord" button in the platform list — same row as Twitch/YouTube/Kick buttons; no-guild state shows an inline message "Connect a Discord server in Settings first" with a `/settings` link
- **Interaction model:** Dialog modal with a 2-step wizard:
  - Step 1: Guild selector (dropdown of connected guilds from guilds API)
  - Step 2: Channel dropdown grouped by Discord category using `<optgroup>` or equivalent — matches the channel listing API response format (grouped by category)
- **Source display label in source list:** `ServerName › #channel-name` — includes guild name for disambiguation when multiple servers are connected; `channel_name` field on the `ChatSource`

### Discord source card design

- Extends the existing `SourceCard` component — same height, same `border-l-2 p-4` layout
- **Brand color:** `#5865F2` (Discord Blurple, official) — add `--color-discord: #5865F2` design token and `border-l-discord` static class to `PLATFORM_BORDER` map (same pattern as twitch/youtube/kick)
- **Connection status:** Small colored dot or pill badge using `is_active` — consistent with `StatusBadge` component pattern from shared_overlay sources
- **Relay indicator:** Secondary `'Relay ON' / 'Relay OFF'` badge next to the channel name, derived from `config.relay_enabled` (JSONB field)
- **Card source label:** `ChatSource.platform` type needs `'discord'` added alongside existing platforms

### Relay config panel

- **Presentation:** Expandable section that renders below the Discord source card on click ("Configure relay" chevron/button on the card) — inline, no modal, no drawer
- **Toggle UX:** Toggle ON immediately reveals the outbound channel picker inline (sourced from the same grouped channel listing API, scoped to the same guild). "Save" is disabled until a channel is selected when relay is ON.
- **Save behavior:** Explicit "Save" button that PATCHes `config` JSONB (`relay_enabled` + `relay_channel_id`) via overlay-manager API, with optimistic UI update and Toast feedback — consistent with overlay customization "Save Changes" button pattern
- **Loop-safe filter indicator:** Static non-interactive info badge or info row inside the relay panel: "Loop filter: active — Discord messages are never relayed back to Discord." Not a toggle.

### Claude's Discretion

- Exact layout of the guild selector in the 2-step dialog (list vs. dropdown; icon + name or name-only)
- Whether the outbound channel picker reuses the same grouped component as the inbound picker or is a simpler inline `<select>`
- Exact Badge variant (color, size) for connection status and relay indicator
- API client module: whether a new `discord.ts` API module is created or Discord endpoints extend `auth.ts`
- Loading skeleton for the guild list and channel list in the dialog
- Whether PLATFORM_BORDER and PlatformBadge changes for Discord are in a shared constants file or co-located

</decisions>

<specifics>
## Specific Ideas

- The `startOAuth` pattern in `AddSourceForm` is the direct reference: `fetch endpoint → redirect to auth_url`. Discord "Add to Server" OAuth uses the same function with the discord auth endpoint.
- The `StatusBadge` component in `src/app/dashboard/shares/components/StatusBadge.tsx` is the reference for the connection status badge on source cards.
- The relay config panel "Configure relay" trigger can be a small gear icon or chevron on the Discord source card — reuse the existing button/icon style pattern.
- Settings page server icon rendering mirrors the user avatar in the Profile card (`Image` component, `rounded-full`, 32–48px).

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/app/overlays/[id]/page.tsx:SourceCard` — existing card component; Discord extends it with connection status + relay badges and a "Configure relay" expand trigger
- `frontend/src/app/overlays/[id]/page.tsx:AddSourceForm` — existing form; Discord slots in as a new platform button + a 2-step Dialog (no-guild state: inline message)
- `frontend/src/app/overlays/[id]/page.tsx:startOAuth(endpoint)` — existing function: fetches auth_url → redirects. Discord "Add to Server" uses same function with discord endpoint
- `frontend/src/app/dashboard/shares/components/StatusBadge.tsx` — connection status pill badge for source cards
- `frontend/src/app/settings/page.tsx` — settings page Card-per-section layout and Dialog pattern for destructive actions
- `frontend/src/components/ui/` — Button, Card, Input, Skeleton, Dialog, Badge, Toast all available
- `frontend/src/lib/types/overlay.ts:ChatSource` — needs `'discord'` added to `platform` union type and Discord-specific fields added to `config` type

### Established Patterns
- **Static class maps for Tailwind JIT:** `PLATFORM_BORDER: Record<string, string>` with full literal class strings. Must add `discord: 'border-l-discord'` as a literal. Same for `PLATFORM_COLORS` wherever used.
- **OAuth source addition:** `startOAuth` → `source_added` query param callback → toast notification (already wired in overlay editor)
- **Optimistic UI + Toast:** All overlay mutations use `toastManager.add({ title, type })` after API call — relay config save follows this pattern
- **Dialog confirmation for destructive actions:** Disconnect guild uses `Dialog.Root` + `Dialog.Trigger` + confirm button inside `Dialog.Content`
- **Skeleton loading:** `Skeleton` component for loading states in guild list and channel picker

### Integration Points
- `frontend/src/lib/types/overlay.ts:ChatSource.platform` — add `'discord'` to union
- `frontend/src/app/overlays/[id]/page.tsx:PLATFORM_BORDER` — add `discord: 'border-l-discord'`
- `frontend/src/app/settings/page.tsx` — add Discord server connect Card section
- `frontend/src/styles/globals.css` — add `--color-discord: #5865F2` design token under `@theme`
- New API module `frontend/src/lib/api/discord.ts` — guilds list, channels list, disconnect endpoints
- Overlay-manager API: PATCH `config` JSONB for relay settings (extend `overlays.ts` or new discord module)

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 32-setup-ui*
*Context gathered: 2026-03-16*
