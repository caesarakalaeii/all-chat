# Phase 32: Setup UI - Research

**Researched:** 2026-03-16
**Domain:** Next.js / React frontend — Discord source configuration UI
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Settings page Discord card**
- Card section in the settings page (same card-per-section layout as Profile, Data & Privacy) with heading "Discord"
- Disconnected state: A `Card` containing a "Connect Discord Server" button that calls `startOAuth` — consistent with existing OAuth button pattern in `AddSourceForm`
- Connected state per guild: Server icon (32–48px avatar-style, matching `profile_image_url` pattern), server name, and an inline "Disconnect" button with a confirmation `Dialog` — same pattern as "Remove source" confirmation in the overlay editor
- Multiple guilds supported: render one server row per connected guild (not limited to first)
- Disconnect: inline in the connected card (with Dialog confirmation), not in Danger Zone

**Add Discord source flow**
- Entry point in `AddSourceForm`: A "Connect Discord" button in the platform list — same row as Twitch/YouTube/Kick buttons; no-guild state shows an inline message "Connect a Discord server in Settings first" with a `/settings` link
- Interaction model: Dialog modal with a 2-step wizard:
  - Step 1: Guild selector (dropdown of connected guilds from guilds API)
  - Step 2: Channel dropdown grouped by Discord category using `<optgroup>` or equivalent — matches the channel listing API response format (grouped by category)
- Source display label in source list: `ServerName › #channel-name` — includes guild name for disambiguation when multiple servers are connected; `channel_name` field on the `ChatSource`

**Discord source card design**
- Extends the existing `SourceCard` component — same height, same `border-l-2 p-4` layout
- Brand color: `#5865F2` (Discord Blurple, official) — add `--color-discord: #5865F2` design token and `border-l-discord` static class to `PLATFORM_BORDER` map (same pattern as twitch/youtube/kick)
- Connection status: Small colored dot or pill badge using `is_active` — consistent with `StatusBadge` component pattern from shared_overlay sources
- Relay indicator: Secondary `'Relay ON' / 'Relay OFF'` badge next to the channel name, derived from `config.relay_enabled` (JSONB field)
- Card source label: `ChatSource.platform` type needs `'discord'` added alongside existing platforms

**Relay config panel**
- Presentation: Expandable section that renders below the Discord source card on click ("Configure relay" chevron/button on the card) — inline, no modal, no drawer
- Toggle UX: Toggle ON immediately reveals the outbound channel picker inline (sourced from the same grouped channel listing API, scoped to the same guild). "Save" is disabled until a channel is selected when relay is ON.
- Save behavior: Explicit "Save" button that PATCHes `config` JSONB (`relay_enabled` + `relay_channel_id`) via overlay-manager API, with optimistic UI update and Toast feedback — consistent with overlay customization "Save Changes" button pattern
- Loop-safe filter indicator: Static non-interactive info badge or info row inside the relay panel: "Loop filter: active — Discord messages are never relayed back to Discord." Not a toggle.

### Claude's Discretion
- Exact layout of the guild selector in the 2-step dialog (list vs. dropdown; icon + name or name-only)
- Whether the outbound channel picker reuses the same grouped component as the inbound picker or is a simpler inline `<select>`
- Exact Badge variant (color, size) for connection status and relay indicator
- API client module: whether a new `discord.ts` API module is created or Discord endpoints extend `auth.ts`
- Loading skeleton for the guild list and channel list in the dialog
- Whether PLATFORM_BORDER and PlatformBadge changes for Discord are in a shared constants file or co-located

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UI-01 | Settings page includes a Discord server connect card showing OAuth2 flow and connected server name/icon | `HandleConnect` returns `bot_invite_url`; `HandleGetGuilds` returns `[]*DiscordGuild` with `guild_id`, `guild_name`, `guild_icon`; `HandleDisconnect` is `DELETE /guilds/:guild_id`. Settings page card-per-section pattern confirmed in `settings/page.tsx`. |
| UI-02 | Overlay editor allows adding a Discord source with guild selector and inbound channel dropdown (from channel listing API) | `HandleGetGuildChannels` returns `{ categories: [{id, name, channels: [{id, name, position}]}] }`. `AddSourceForm` OAuth button pattern (`startOAuth`) confirmed. Source is created via existing `POST /:id/sources`. |
| UI-03 | Per-source relay configuration panel: toggle relay, pick outbound channel, visual indicator of active filter | No existing `PATCH /sources/:id` endpoint — must be added to overlay-manager. `config` JSONB field (`relay_enabled`, `relay_channel_id`) already exists on `overlay_chat_sources`. Channel listing API reused for outbound channel picker. |
| UI-04 | Discord source cards in the overlay editor display connection status and relay active/inactive indicator | `ChatSource.is_active` drives connection dot; `config.relay_enabled` drives relay badge. `PLATFORM_BORDER`/`PLATFORM_COLORS` static maps need `'discord'` entries. `platform` union type needs `'discord'` added in `overlay.ts` and `platform-colors.ts`. |
</phase_requirements>

---

## Summary

Phase 32 is a pure frontend phase. All backend API endpoints needed are live from phases 27–31, with one exception: the relay config save requires a new `PATCH /api/v1/overlays/:id/sources/:source_id` endpoint in overlay-manager that updates the `config` JSONB column. Everything else is frontend component work that extends existing patterns.

The frontend stack is Next.js 14+ App Router with React 19, TypeScript, Tailwind CSS v4, `@base-ui/react` for Dialog/primitive components, Vitest for unit tests, and Playwright for e2e tests. The project's design system uses a `@theme` block in `globals.css` for design tokens and static literal Tailwind class maps (never dynamic string construction) for JIT safety.

The four UI requirements map cleanly to four isolated work areas: (1) settings page Discord card, (2) `AddSourceForm` 2-step dialog, (3) Discord-specific `SourceCard` extension with relay panel, and (4) type system and token updates. The relay panel requires the most new logic — local state for toggle + channel selection, a save that calls `PATCH`, and optimistic UI with Toast. All other areas are primarily additive modifications to existing components.

**Primary recommendation:** Add the `PATCH /sources/:source_id` endpoint to overlay-manager in the first plan of this phase (backend gap), then implement frontend in subsequent plans.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Next.js | ^16.1.6 (App Router) | React framework with SSR | Project standard — all pages use it |
| React | ^19.2.4 | UI rendering | Project standard |
| TypeScript | ^5.3.3 | Type safety | Project standard — no `any` |
| Tailwind CSS | ^4.1.18 | Utility-first styles | Project standard; v4 `@theme` tokens |
| `@base-ui/react` | ^1.2.0 | Headless Dialog, Tooltip primitives | Project standard — all dialogs use it |
| `lucide-react` | ^0.563.0 | Icons | Project standard |
| `zustand` | ^5.0.11 | Auth store | Project standard |
| `class-variance-authority` | ^0.7.1 | Component variants | Project standard — used in badge.tsx |
| `clsx` | ^2.1.1 | Conditional class names | Project standard |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Vitest | ^4.0.18 | Unit tests | Static map / pure logic tests |
| Playwright | ^1.58.2 | E2E tests | Smoke tests for new UI flows |
| `@testing-library/react` | ^16.3.2 | Component test utilities | Component interaction tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Inline `<select>` for channel dropdown | Custom dropdown component | `<select>` natively supports `<optgroup>` for category grouping — simpler and accessible; custom dropdown adds complexity with no benefit |
| New `discord.ts` API module | Extend `auth.ts` | New module keeps Discord endpoints separate and matches the `shares.ts`/`overlays.ts`/`auth.ts` pattern — preferred |

**Installation:** No new packages required — all dependencies are already installed.

---

## Architecture Patterns

### Recommended Project Structure
```
frontend/src/
├── app/
│   ├── settings/page.tsx            # Add Discord Card section here
│   └── overlays/[id]/page.tsx       # Modify SourceCard, AddSourceForm, PLATFORM_BORDER
├── lib/
│   ├── api/
│   │   └── discord.ts               # NEW: guilds list, channels list, disconnect, patch config
│   ├── types/
│   │   └── overlay.ts               # Add 'discord' to platform union; add DiscordSourceConfig
│   └── platform-colors.ts           # Add discord entry
└── app/globals.css                  # Add --color-discord token
```

### Pattern 1: Design Token + Static Class Map for New Platform

**What:** Register `#5865F2` as a CSS variable in `@theme` block and add literal class strings to both `PLATFORM_BORDER` and `PLATFORM_COLORS` maps.

**When to use:** Any time a new platform is added — the same pattern was applied for twitch, youtube, kick, tiktok.

**Critical constraint:** Tailwind v4 JIT requires full literal class strings. NEVER construct class names dynamically (e.g., `'border-l-' + platform` will NOT work and will silently produce invisible styles).

```typescript
// Source: frontend/src/lib/platform-colors.ts (verified)
export const PLATFORM_COLORS = {
  twitch: { text: 'text-twitch', bg: 'bg-twitch' },
  youtube: { text: 'text-youtube', bg: 'bg-youtube' },
  kick: { text: 'text-kick', bg: 'bg-kick' },
  tiktok: { text: 'text-tiktok', bg: 'bg-tiktok' },
  system: { text: 'text-text-sub', bg: 'bg-surface' },
  // ADD:
  discord: { text: 'text-discord', bg: 'bg-discord' },
} as const
```

```css
/* Source: frontend/src/app/globals.css @theme block (verified) */
@theme {
  --color-discord: #5865F2;   /* ADD under platform colors */
}
```

```typescript
// Source: frontend/src/app/overlays/[id]/page.tsx (verified)
const PLATFORM_BORDER: Record<string, string> = {
  twitch: 'border-l-twitch',
  youtube: 'border-l-youtube',
  kick: 'border-l-kick',
  tiktok: 'border-l-tiktok',
  shared_overlay: 'border-l-twitch',
  // ADD:
  discord: 'border-l-discord',
}
```

### Pattern 2: startOAuth for Discord Connect

**What:** Call `GET /api/v1/auth/discord/connect` to get `bot_invite_url`, then redirect to it. The backend returns `{ bot_invite_url: string }`.

**When to use:** Settings page "Connect Discord Server" button and any future reconnect flow.

```typescript
// Source: inferred from HandleConnect in handlers/discord.go (verified)
// The function returns { bot_invite_url: "https://discord.com/api/oauth2/authorize?..." }
async function startDiscordOAuth(token: string) {
  const res = await fetch('/api/v1/auth/discord/connect', {
    headers: { Authorization: `Bearer ${token}` },
  })
  const data = await res.json()
  if (data.bot_invite_url) {
    window.location.href = data.bot_invite_url
  }
}
```

**Key distinction from `startOAuth` in AddSourceForm:** Discord connect returns `bot_invite_url` (not `auth_url`). Must check the correct field name.

### Pattern 3: Channel Listing API Response Shape

**What:** `GET /api/v1/auth/guilds/:guild_id/channels` returns grouped categories.

**When to use:** Both inbound channel picker (Step 2 of Add Discord dialog) and outbound channel picker (relay panel).

```typescript
// Source: handlers/discord.go channelCategory/channelSummary types (verified)
interface ChannelSummary {
  id: string
  name: string
  position: number
}

interface ChannelCategory {
  id: string        // empty string means "Uncategorized"
  name: string
  channels: ChannelSummary[]
}

interface GuildChannelsResponse {
  categories: ChannelCategory[]
}
```

**HTML `<optgroup>` rendering for channel dropdown:**
```tsx
<select>
  {categories.map((cat) => (
    <optgroup key={cat.id || 'uncategorized'} label={cat.name}>
      {cat.channels.map((ch) => (
        <option key={ch.id} value={ch.id}>
          #{ch.name}
        </option>
      ))}
    </optgroup>
  ))}
</select>
```

### Pattern 4: Guild List API Response Shape

**What:** `GET /api/v1/auth/guilds` returns an array of DiscordGuild objects.

```typescript
// Source: models/discord_guild.go (verified)
interface DiscordGuild {
  id: string
  user_id: string
  guild_id: string    // Snowflake — always string, NEVER number
  guild_name: string
  guild_icon: string | null  // CDN hash, nullable
  connected_at: string       // ISO timestamp
}
```

**Guild icon URL construction** (Discord CDN pattern — confirmed from CONTEXT.md):
```typescript
// Discord guild icon URL:
const iconUrl = guild.guild_icon
  ? `https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.guild_icon}.png?size=64`
  : null
```

### Pattern 5: New PATCH Source Config Endpoint (Backend Gap)

**What:** The relay config save needs `PATCH /api/v1/overlays/:id/sources/:source_id` to update the `config` JSONB column. This endpoint DOES NOT EXIST yet.

**Why it must be added:** The source `config` column is `map[string]interface{}` in the model. Currently only `Create` and `Delete` exist on `SourceRepository` — no `UpdateConfig` method.

**Required additions (overlay-manager):**
1. `SourceRepository.UpdateConfig(ctx, id, config)` — SQL: `UPDATE overlay_chat_sources SET config = $2, updated_at = NOW() WHERE id = $1`
2. `SourcesHandler.HandleUpdateSourceConfig(c *gin.Context)` — PATCH handler: parse body `{ config: { relay_enabled: bool, relay_channel_id: string } }`, verify overlay ownership, call repo
3. Route registration in `cmd/main.go`: `protected.PATCH("/:id/sources/:source_id", sourcesHandler.HandleUpdateSourceConfig)`

**Frontend call (overlay-manager proxy path):**
```typescript
// In discord.ts API module
async function updateSourceConfig(
  overlayId: string,
  sourceId: string,
  config: DiscordSourceConfig
): Promise<ChatSource> {
  return apiClient.patch<ChatSource>(
    `/api/v1/overlays/${overlayId}/sources/${sourceId}`,
    { config }
  )
}
```

**Note:** `apiClient` in `client.ts` has no `patch` method yet — it must be added alongside the `put` method.

### Pattern 6: Dialog-based 2-Step Wizard

**What:** The Add Discord source flow uses `Dialog.Root` + state-driven step rendering inside `Dialog.Content`.

**When to use:** Multi-step flows that don't warrant a full page route.

```tsx
// Source: frontend/src/components/ui/dialog.tsx (verified)
// Step state pattern:
const [step, setStep] = useState<1 | 2>(1)
const [selectedGuild, setSelectedGuild] = useState<DiscordGuild | null>(null)
const [selectedChannelId, setSelectedChannelId] = useState<string>('')

<Dialog.Root open={open} onOpenChange={setOpen}>
  <Dialog.Content size="default">
    <Dialog.Title>
      {step === 1 ? 'Select Discord Server' : 'Select Channel'}
    </Dialog.Title>
    {step === 1 && <GuildStep guilds={guilds} onSelect={(g) => { setSelectedGuild(g); setStep(2) }} />}
    {step === 2 && <ChannelStep guild={selectedGuild} onSelect={setSelectedChannelId} />}
    {/* Footer buttons */}
  </Dialog.Content>
</Dialog.Root>
```

### Pattern 7: Toast + Optimistic UI for Config Save

**What:** All overlay mutations use `toastManager.add(...)` after API call. Relay config save follows this exactly.

```typescript
// Source: frontend/src/app/overlays/[id]/page.tsx (verified — toastManager.add pattern)
async function handleSaveRelayConfig(source: ChatSource, relayEnabled: boolean, relayChannelId: string | null) {
  const newConfig = {
    ...source.config,
    relay_enabled: relayEnabled,
    relay_channel_id: relayChannelId,
  }
  // Optimistic update
  setSources((prev) => prev.map((s) => s.id === source.id ? { ...s, config: newConfig } : s))
  try {
    await discordApi.updateSourceConfig(overlayId, source.id, newConfig)
    toastManager.add({ title: 'Relay settings saved', type: 'success' })
  } catch {
    // Rollback
    setSources((prev) => prev.map((s) => s.id === source.id ? source : s))
    toastManager.add({ title: 'Failed to save relay settings', type: 'error' })
  }
}
```

### Pattern 8: Settings Page Discord Connect Callback

**What:** After successful Discord OAuth, the backend redirects to `/settings?discord=connected`. The settings page must handle this query param to show a success toast (same pattern as `source_added` query param in overlay editor).

```typescript
// Source: handlers/discord.go line 244 (verified)
// redirectURL := strings.TrimSuffix(h.frontendURL, "/") + "/settings?discord=connected"
const searchParams = useSearchParams()
useEffect(() => {
  if (searchParams.get('discord') === 'connected') {
    toastManager.add({ title: 'Discord server connected', type: 'success' })
    // clear param from URL
    router.replace('/settings')
    // re-fetch guilds
    fetchGuilds()
  }
}, [searchParams])
```

### Anti-Patterns to Avoid

- **Dynamic Tailwind class construction:** `'border-l-' + platform` silently breaks JIT. Always use literal strings in static maps.
- **Snowflake IDs as numbers:** Discord guild_id and channel IDs MUST be strings. Never coerce to `Number` or `parseInt`.
- **Trusting guild from OAuth query params:** The backend already validates — the frontend never needs to parse `guild_id` from the callback URL.
- **Calling `apiClient.get` with no auth header:** `apiClient` automatically adds the JWT from localStorage. No manual header injection needed.
- **Using `any` type:** Project rule — use proper types. `config` on `ChatSource` is typed `Record<string, unknown>`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Accessible modal/dialog | Custom overlay + focus trap | `Dialog` from `@base-ui/react` (already available via `frontend/src/components/ui/dialog.tsx`) | Focus management, keyboard nav, ARIA roles handled |
| Platform color consistency | Per-component color constants | `PLATFORM_COLORS` in `platform-colors.ts` + `@theme` token | JIT-safe, single source of truth |
| Status indicator component | Custom dot/badge | `StatusBadge` in `dashboard/shares/components/StatusBadge.tsx` (reuse or copy pattern) | Already handles color variants + text |
| OAuth redirect | Custom fetch + redirect | `startOAuth(endpoint)` pattern from `AddSourceForm` | Already handles auth header, error logging |
| Toast notifications | Custom notification system | `toastManager` from `@/lib/toast` | Already wired across the app |
| Loading states | Custom spinner | `Skeleton` from `frontend/src/components/ui/skeleton.tsx` | Consistent with existing loading patterns |

**Key insight:** This phase is 90% wiring existing patterns together. The novel code is: (1) the 2-step guild→channel dialog, (2) the expandable relay panel with toggle + save, and (3) the PATCH source config endpoint in the backend.

---

## Common Pitfalls

### Pitfall 1: PATCH endpoint missing from `apiClient`
**What goes wrong:** `apiClient` in `client.ts` has `get`, `post`, `put`, `delete` — but no `patch`. Calling a non-existent method fails at compile time or runtime.
**Why it happens:** `PUT` was sufficient for all prior endpoints; `PATCH` was never needed.
**How to avoid:** Add `async patch<T>(endpoint: string, data: unknown): Promise<T>` to `ApiClient` in `client.ts` alongside `put` — identical implementation except `method: 'PATCH'`.
**Warning signs:** TypeScript error `Property 'patch' does not exist on type 'ApiClient'`.

### Pitfall 2: Tailwind JIT not seeing `border-l-discord` / `text-discord` / `bg-discord`
**What goes wrong:** The left border on the Discord source card is invisible; the platform badge text color is wrong.
**Why it happens:** Tailwind v4 JIT purges any class not present as a full literal string in scanned source files. Dynamically constructed strings (`'border-l-' + platform`) are invisible to the scanner.
**How to avoid:** The static maps in `PLATFORM_BORDER` and `PLATFORM_COLORS` use full literal strings — just add `discord: 'border-l-discord'` and `discord: { text: 'text-discord', bg: 'bg-discord' }` as verbatim string literals. The `--color-discord` token in `@theme` enables these utility classes automatically.
**Warning signs:** Source card renders without left border color; platform badge shows wrong color.

### Pitfall 3: `guild_icon` is null for servers without a custom icon
**What goes wrong:** `Image` component throws or renders broken image when `guild_icon` is null.
**Why it happens:** `guild_icon` is a nullable CDN hash in `models.DiscordGuild` (`*string` in Go → `string | null` in TypeScript).
**How to avoid:** Always null-check `guild.guild_icon` before constructing the CDN URL. Fall back to a placeholder (first letter of guild name or a generic Discord icon).
**Warning signs:** Broken image icon for servers without custom icons.

### Pitfall 4: Relay panel fetches channels before guild is known
**What goes wrong:** The outbound channel picker in the relay panel tries to call the channels API but `guild_id` is not on the source in the standard fields.
**Why it happens:** Discord source's `guild_id` is in `config.guild_id` (JSONB), not a top-level column. The frontend must extract it from `source.config`.
**How to avoid:** Define a `DiscordSourceConfig` type with `guild_id: string` and cast/narrow `source.config` to it when rendering the relay panel. Only render the channel picker when `guild_id` is available.
**Warning signs:** API call to channels endpoint with undefined `guild_id`.

### Pitfall 5: `source_added` callback wiring not needed for Discord
**What goes wrong:** Developer assumes Discord source addition follows the OAuth `source_added` query param callback pattern (like Twitch/YouTube/Kick).
**Why it happens:** Twitch/YouTube/Kick OAuth redirects back to the overlay editor with `?source_added=true` after completion. Discord's "Add to Server" flow redirects to `/settings?discord=connected`, not back to the overlay editor.
**How to avoid:** The 2-step wizard adds the Discord source via a direct `POST /sources` call (not OAuth redirect) after the user selects guild + channel. No `source_added` callback handling is needed in the overlay editor for Discord.
**Warning signs:** Expecting `source_added` query param in overlay editor after Discord connection.

### Pitfall 6: `Platform` type in `platform-colors.ts` must be updated
**What goes wrong:** TypeScript error when passing `'discord'` to `PlatformBadge` — `platform` prop expects `Platform` type which doesn't include `'discord'`.
**Why it happens:** `Platform` is derived from `keyof typeof PLATFORM_COLORS` — a discriminated union of the current keys. Adding to the map extends the type automatically.
**How to avoid:** Add `discord` entry to `PLATFORM_COLORS` first — the `Platform` type expands automatically. No separate type declaration needed.
**Warning signs:** TS error `Type '"discord"' is not assignable to type 'Platform'`.

---

## Code Examples

Verified patterns from official sources:

### Existing `startOAuth` pattern (AddSourceForm)
```typescript
// Source: frontend/src/app/overlays/[id]/page.tsx lines 324-338 (verified)
const startOAuth = async (endpoint: string) => {
  try {
    const res = await fetch(endpoint, {
      headers: { Authorization: `Bearer ${token}` },
    })
    const data = await res.json()
    if (data.auth_url) {
      window.location.href = data.auth_url
    } else {
      console.error('No auth_url returned', data)
    }
  } catch (err) {
    console.error('Failed to initiate OAuth', err)
  }
}
```
**Discord variation:** Replace `data.auth_url` check with `data.bot_invite_url` since `HandleConnect` returns `{ bot_invite_url: string }`.

### Existing Dialog confirmation pattern
```tsx
// Source: frontend/src/app/settings/page.tsx lines 102-117 (verified)
<Dialog.Root>
  <Dialog.Trigger render={<Button variant="destructive">Delete Account</Button>} />
  <Dialog.Content showCloseButton={false}>
    <Dialog.Title>Delete your account?</Dialog.Title>
    <Dialog.Description>...</Dialog.Description>
    <div className="mt-6 flex justify-end gap-3">
      <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
      <Button variant="destructive" onClick={handleDeleteAccount}>
        Yes, delete my account
      </Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
```

### Guild icon Image pattern (Profile card reference)
```tsx
// Source: frontend/src/app/settings/page.tsx lines 47-56 (verified)
<Image
  src={user.profile_image_url}
  alt={user.display_name}
  width={48}
  height={48}
  className="rounded-full object-cover"
/>
// Discord guild icon — same dimensions/classes:
{guild.guild_icon && (
  <Image
    src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.guild_icon}.png?size=64`}
    alt={guild.guild_name}
    width={32}
    height={32}
    className="rounded-full object-cover"
  />
)}
```

### `apiClient.patch` addition needed
```typescript
// Source: frontend/src/lib/api/client.ts — current state (no patch method, verified)
// ADD alongside put():
async patch<T>(endpoint: string, data: unknown): Promise<T> {
  const response = await this.fetch(endpoint, {
    method: 'PATCH',
    body: JSON.stringify(data),
  })
  return response.json()
}
```

### Discord source config type
```typescript
// New type needed in frontend/src/lib/types/overlay.ts
export interface DiscordSourceConfig {
  guild_id: string
  inbound_channel_id: string
  relay_enabled: boolean
  relay_channel_id: string | null
  [key: string]: unknown  // allow other JSONB fields
}
```

---

## API Endpoints Summary

All endpoints are behind API Gateway at `/api/v1/auth/*` (proxied from auth-service) or `/api/v1/overlays/*` (overlay-manager).

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/api/v1/auth/discord/connect` | GET | JWT | Get `bot_invite_url` for OAuth |
| `/api/v1/auth/discord/callback` | GET | None (CSRF state) | OAuth callback — redirects to `/settings?discord=connected` |
| `/api/v1/auth/guilds` | GET | JWT | List connected guilds → `DiscordGuild[]` |
| `/api/v1/auth/guilds/:guild_id/channels` | GET | JWT | Channel listing → `{ categories: ChannelCategory[] }` |
| `/api/v1/auth/guilds/:guild_id` | DELETE | JWT | Disconnect guild |
| `/api/v1/overlays/:id/sources` | POST | JWT | Add Discord source (existing) |
| `/api/v1/overlays/:id/sources/:source_id` | PATCH | JWT | Update source config — **MUST BE ADDED** |

---

## Backend Gap: PATCH Source Config

This is the only backend work required in Phase 32.

**Overlay-manager changes needed:**

1. **`repository/source_repo.go`** — Add `UpdateConfig` method:
```go
func (r *SourceRepository) UpdateConfig(ctx context.Context, id string, config map[string]interface{}) error {
    query := `UPDATE overlay_chat_sources SET config = $2, updated_at = NOW() WHERE id = $1`
    _, err := r.pool.Exec(ctx, query, id, config)
    return err
}
```

2. **`handlers/sources.go`** — Add `HandleUpdateSourceConfig` handler (PATCH `/:id/sources/:source_id`):
   - Extract `user_id` from context
   - Verify overlay ownership via `overlayRepo.GetByIDAndUserID`
   - Parse `{ config: map[string]interface{} }` from body
   - Call `sourceRepo.UpdateConfig`
   - Return updated source or 204

3. **`cmd/main.go`** — Register route:
```go
protected.PATCH("/:id/sources/:source_id", sourcesHandler.HandleUpdateSourceConfig)
```

4. **`handlers/sources_test.go`** (or new file) — Unit test for the handler.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Tailwind v3 dynamic class construction | Tailwind v4 requires full literal strings in static maps | v1.3 frontend redesign | All new platform entries must be literal strings |
| `any` on config fields | `Record<string, unknown>` or typed interfaces | Project-wide convention | Discord config fields need proper interface |

**Deprecated/outdated:**
- None relevant to this phase.

---

## Open Questions

1. **Guild icon in Next.js `<Image>` — domain allowlist**
   - What we know: `next.config.js` must whitelist external image domains for `next/image`. Discord CDN is `cdn.discordapp.com`.
   - What's unclear: Whether `cdn.discordapp.com` is already in the Next.js image domains config.
   - Recommendation: Check `next.config.js` before implementing guild icon rendering. If absent, add it. Alternative: use plain `<img>` tag with explicit dimensions to bypass the constraint.

2. **Source config `guild_id` field naming**
   - What we know: `HandleAddSource` in `sources.go` creates sources with `config: make(map[string]interface{})` — an empty config. The `guild_id`, `inbound_channel_id`, `relay_channel_id`, `relay_enabled` are supposed to be in JSONB per CONTEXT.md decisions.
   - What's unclear: Which code currently writes `guild_id` into `config` on source creation — the frontend POST body must include `config` with `guild_id` set, or the backend must extract it from channel info.
   - Recommendation: The `POST /sources` body from the frontend wizard should include `config: { guild_id, inbound_channel_id }` — the overlay-manager's `HandleAddSource` already stores whatever `config` is passed (no stripping). Verify this works by checking that the Add Source handler doesn't reset config to empty after creation.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest ^4.0.18 |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npm test -- --project=unit --run` |
| Full suite command | `cd frontend && npm test -- --run` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UI-01 | `PLATFORM_COLORS.discord` has correct text/bg literals | unit | `cd frontend && npm test -- --project=unit --run src/lib/__tests__/platform-colors.test.ts` | ❌ Wave 0 |
| UI-02 | `DiscordGuild` type accepts null `guild_icon` | unit | `cd frontend && npm test -- --project=unit --run src/lib/__tests__/types.test.ts` | ❌ Wave 0 |
| UI-03 | `discordApi.updateSourceConfig` calls PATCH with correct path | unit | `cd frontend && npm test -- --project=unit --run src/lib/api/__tests__/discord.test.ts` | ❌ Wave 0 |
| UI-04 | `PLATFORM_BORDER['discord']` equals `'border-l-discord'` (static literal test) | unit | `cd frontend && npm test -- --project=unit --run src/lib/__tests__/platform-colors.test.ts` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd frontend && npm test -- --project=unit --run`
- **Per wave merge:** `cd frontend && npm test -- --run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/lib/__tests__/platform-colors.test.ts` — extend existing file to cover `discord` entries (UI-01, UI-04)
- [ ] `frontend/src/lib/api/__tests__/discord.test.ts` — covers `discordApi` module functions (UI-03)
- [ ] `frontend/src/lib/__tests__/types.test.ts` — covers type narrowing for `DiscordSourceConfig` (UI-02)

---

## Sources

### Primary (HIGH confidence)
- `frontend/src/app/overlays/[id]/page.tsx` — `SourceCard`, `AddSourceForm`, `startOAuth`, `PLATFORM_BORDER` patterns
- `frontend/src/app/settings/page.tsx` — Settings card-per-section layout, Dialog confirmation pattern
- `frontend/src/lib/platform-colors.ts` — `PLATFORM_COLORS` map and `Platform` type
- `frontend/src/app/globals.css` — `@theme` design token system
- `frontend/src/components/ui/badge.tsx` — `PlatformBadge` implementation
- `frontend/src/components/ui/dialog.tsx` — `@base-ui/react` Dialog wrapper API
- `frontend/src/lib/api/client.ts` — `ApiClient` method list (no `patch` confirmed)
- `frontend/src/lib/types/overlay.ts` — `ChatSource` platform union (no `discord`)
- `services/auth-service/handlers/discord.go` — All Discord endpoints, request/response shapes
- `services/auth-service/models/discord_guild.go` — `DiscordGuild` model shape
- `services/auth-service/cmd/main.go` — Route registrations for Discord
- `services/overlay-manager/handlers/sources.go` — `HandleAddSource` with Discord channel registry logic
- `services/overlay-manager/repository/source_repo.go` — No `UpdateConfig` confirmed
- `services/overlay-manager/cmd/main.go` — No PATCH route for sources confirmed
- `frontend/package.json` — Dependency versions, test scripts
- `frontend/vitest.config.ts` — Test configuration

### Secondary (MEDIUM confidence)
- Discord CDN URL pattern: `https://cdn.discordapp.com/icons/{guild_id}/{guild_icon}.png?size=64` — standard Discord CDN format, confirmed against Discord documentation patterns

### Tertiary (LOW confidence)
- `next.config.js` image domain for `cdn.discordapp.com` — not read; flagged as open question

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified from `package.json`
- Architecture: HIGH — all patterns verified from existing source files
- API endpoints: HIGH — verified from handler source code
- Backend gap (PATCH endpoint): HIGH — absence of endpoint confirmed by scanning all routes and repository methods
- Pitfalls: HIGH — all based on direct code inspection

**Research date:** 2026-03-16
**Valid until:** 2026-04-16 (stable codebase, 30-day validity)
