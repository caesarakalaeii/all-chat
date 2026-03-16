---
phase: 32-setup-ui
verified: 2026-03-16T12:00:00Z
status: gaps_found
score: 14/15 must-haves verified
gaps:
  - truth: "After Step 2 channel selection, POST /sources creates the Discord source with config.guild_id and config.inbound_channel_id set"
    status: failed
    reason: "HandleAddSource in sources.go does not parse the `config` field from the POST request body. The struct only binds platform, channel_id, channel_name, channel_handle. Config is always initialized as make(map[string]interface{}) — an empty map. The frontend sends guild_id, guild_name, inbound_channel_id, relay_enabled, relay_channel_id in the config field but they are silently dropped."
    artifacts:
      - path: "services/overlay-manager/handlers/sources.go"
        issue: "HandleAddSource request struct (lines 297-307) has no Config field. Source is created with Config: make(map[string]interface{}) at line 402."
    missing:
      - "Add `Config map[string]interface{} json:\"config\"` to the HandleAddSource request struct"
      - "Assign config to source: `if req.Config != nil { source.Config = req.Config }`"
      - "The Redis channel registry at line 424 already attempts to read inbound_channel_id from source.Config — this will only work once Config is actually persisted from the POST body"
human_verification:
  - test: "Visit /settings, confirm Discord section card appears with 'Connect Discord Server' button"
    expected: "Card renders with heading 'Discord', button labeled 'Connect Discord Server', skeleton while loading, guild rows with icon/initial + name + Disconnect button when connected"
    why_human: "Visual layout, icon rendering, and Dialog confirmation require browser interaction"
  - test: "On overlay editor page, click 'Connect Discord' with no guilds connected"
    expected: "Inline message appears: 'Connect a Discord server in Settings first to add Discord sources.' with Settings link"
    why_human: "Conditional rendering based on guilds state requires live browser"
  - test: "On overlay editor page with a connected guild, use the 2-step dialog to add a Discord source"
    expected: "Step 1 shows guild list with icon/initial; Step 2 shows channels grouped by category; Add creates the source and closes dialog"
    why_human: "Multi-step dialog flow requires live browser and real guild/channel data"
  - test: "Click 'Configure relay' on a Discord source card"
    expected: "Relay panel expands below the card with: loop filter text, relay toggle, outbound channel picker (when relay ON), Save button (disabled when relay ON and no channel selected)"
    why_human: "Expandable panel and Save button state require live browser interaction"
  - test: "After a page refresh on the overlay editor with a Discord source, check source label and config"
    expected: "Source label reads 'GuildName > #channel-name' — requires guild_name to be persisted in config"
    why_human: "This test will likely FAIL until the config persistence gap is fixed — confirms severity of the gap"
---

# Phase 32: Setup UI Verification Report

**Phase Goal:** Implement the Discord UI — settings connect card, overlay editor source card with relay panel, and the 2-step Add Discord Source dialog — so users can visually manage Discord integrations in the app.
**Verified:** 2026-03-16T12:00:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Success Criteria from ROADMAP.md:

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| SC1 | Settings page shows a Discord server connect card — clicking it initiates the OAuth2 redirect; after completion the card updates to show the connected server name and icon | ? NEEDS HUMAN | `settings/page.tsx` lines 143-212: Discord Card with `startDiscordOAuth`, guild icon CDN rendering, initial fallback. Code is substantive and wired. Visual appearance needs human. |
| SC2 | Overlay editor allows adding a Discord source by selecting a guild and choosing an inbound channel from a dropdown populated from the channel listing API | ✗ FAILED | 2-step dialog exists (page.tsx lines 683-791) and calls `overlaysApi.addSource`. However, `HandleAddSource` backend drops the `config` field from the POST body (sources.go lines 297-307, 402). Guild_id and inbound_channel_id are NOT persisted. |
| SC3 | Each Discord source card in the overlay editor shows connection status and a visual indicator of whether relay is active or inactive | ? NEEDS HUMAN | `SourceCard` (page.tsx lines 256-287): connection status badge (green dot + "Connected"/"Disconnected"), relay badge ("Relay ON"/"Relay OFF"). Code verified. Visual appearance needs human. |
| SC4 | Per-source relay configuration panel lets the user toggle relay on/off and pick an outbound channel; the visual filter indicator updates immediately on toggle | ? NEEDS HUMAN | `RelayPanel` (page.tsx lines 353-458): toggle, grouped channel picker, loop filter static text, Save with optimistic update and toast. Code fully implemented. Visual/interaction needs human. |

**Score:** 3/4 success criteria automatable-verified (1 failed, 3 need human)

### Plan 01 Must-Haves (Foundation)

| Truth | Status | Evidence |
|-------|--------|---------|
| PATCH /api/v1/overlays/:id/sources/:source_id updates config JSONB and returns 200 | ✓ VERIFIED | `HandleUpdateSourceConfig` exists (sources.go lines 518-561), `UpdateConfig` SQL in source_repo.go lines 170-174, route registered at main.go line 203 |
| TypeScript knows ChatSource.platform includes 'discord' | ✓ VERIFIED | overlay.ts line 58: `'twitch' | 'youtube' | 'kick' | 'tiktok' | 'shared_overlay' | 'discord'` |
| PLATFORM_COLORS.discord has literal 'text-discord' and 'bg-discord' strings | ✓ VERIFIED | platform-colors.ts line 12: `discord: { text: 'text-discord', bg: 'bg-discord' }` |
| globals.css @theme block declares --color-discord: #5865F2 | ✓ VERIFIED | globals.css line 29: `--color-discord: #5865F2;` |
| apiClient.patch<T> method exists and mirrors apiClient.put | ✓ VERIFIED | client.ts lines 101-107: `async patch<T>(endpoint, data)` using PATCH method |
| discord.ts API module exports getGuilds, getGuildChannels, disconnectGuild, updateSourceConfig | ✓ VERIFIED | discord.ts: all 4 exports confirmed (plus `startDiscordOAuth`) |
| cdn.discordapp.com is in next.config.js images.domains | ✓ VERIFIED | next.config.js line 34: `'cdn.discordapp.com'` |

### Plan 02 Must-Haves (Settings)

| Truth | Status | Evidence |
|-------|--------|---------|
| Settings page shows a Discord section card with heading 'Discord' | ✓ VERIFIED | settings/page.tsx lines 143-212: `<Card>` with `<h2>Discord</h2>` |
| Disconnected state: 'Connect Discord Server' button calls startDiscordOAuth | ✓ VERIFIED | settings/page.tsx line 152: `<Button onClick={startDiscordOAuth}>Connect Discord Server</Button>` |
| Connected state: one row per guild showing server icon or initial fallback, server name, and Disconnect button | ✓ VERIFIED | settings/page.tsx lines 156-203: guild.map with CDN Image or initial div, guild_name, Disconnect button |
| Disconnect button shows Dialog confirmation before calling disconnectGuild | ✓ VERIFIED | settings/page.tsx lines 174-202: Dialog.Root with confirmation before `handleDisconnectGuild` which calls `disconnectGuild` |
| After disconnect, guild row disappears | ✓ VERIFIED | settings/page.tsx line 58: `setGuilds((prev) => prev.filter(...))` |
| ?discord=connected query param shows success toast and re-fetches guilds | ✓ VERIFIED | settings/page.tsx lines 44-50: `useEffect` on searchParams detects 'connected', calls `toastManager.add`, `router.replace`, `fetchGuilds()` |

### Plan 03 Must-Haves (Overlay Editor)

| Truth | Status | Evidence |
|-------|--------|---------|
| AddSourceForm shows a 'Connect Discord' button in the platform list | ✓ VERIFIED | page.tsx line 678: `Connect Discord` button in platform list |
| No-guild state shows inline message with /settings link | ✓ VERIFIED | page.tsx lines 657-664: conditional inline message with `<Link href="/settings">` |
| 2-step Dialog: guild selector + grouped channel picker | ✓ VERIFIED | page.tsx lines 683-791: Dialog with step state, guild buttons (step 1), optgroup channel select (step 2) |
| POST /sources creates Discord source with config.guild_id and config.inbound_channel_id set | ✗ FAILED | Frontend sends config correctly in `handleAddDiscordSource` (page.tsx lines 545-551), but `HandleAddSource` backend struct (sources.go line 297-307) has no `Config` field — config is always `make(map[string]interface{})` |
| Discord source cards show Discord Blurple left border | ✓ VERIFIED | page.tsx line 73: `discord: 'border-l-discord'` literal in PLATFORM_BORDER |
| Discord source card label reads 'ServerName > #channel-name' | ⚠ PARTIAL | Label construction in SourceCard (page.tsx lines 233-236) uses `source.config.guild_name`. Will render correctly when config is populated (optimistic), but config will be empty after page refresh until backend gap is fixed. |
| Relay panel: toggle + outbound picker + Save (disabled without channel when relay ON) + loop filter text | ✓ VERIFIED | RelayPanel (page.tsx lines 353-458): all elements present and wired. Save disabled at line 401: `saving || (relayEnabled && !relayChannelId)` |
| Saving relay config calls PATCH with optimistic update and Toast | ✓ VERIFIED | RelayPanel handleSave (lines 381-398): optimistic update, `updateSourceConfig` call, toast on success/error, rollback on failure |
| Loop filter indicator visible as static info text | ✓ VERIFIED | page.tsx lines 405-408: `Loop filter: active — Discord messages are never relayed back to Discord.` |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/overlay-manager/repository/source_repo.go` | UpdateConfig(ctx, id, config) method | ✓ VERIFIED | Lines 169-174: SQL UPDATE overlay_chat_sources SET config = $2 |
| `services/overlay-manager/handlers/sources.go` | HandleUpdateSourceConfig PATCH handler | ✓ VERIFIED | Lines 518-561: complete implementation with ownership check |
| `services/overlay-manager/handlers/sources_patch_test.go` | Tests for PATCH handler | ✓ VERIFIED | 3 tests: Success, NonOwner (403), MissingConfig (400) |
| `frontend/src/lib/api/discord.ts` | Discord API client functions | ✓ VERIFIED | 5 exports: getGuilds, getGuildChannels, disconnectGuild, startDiscordOAuth, updateSourceConfig |
| `frontend/src/lib/types/overlay.ts` | DiscordSourceConfig + updated platform union | ✓ VERIFIED | Lines 55-74: platform union includes 'discord', DiscordSourceConfig interface |
| `frontend/src/lib/platform-colors.ts` | discord entry in PLATFORM_COLORS | ✓ VERIFIED | Line 12: `discord: { text: 'text-discord', bg: 'bg-discord' }` |
| `frontend/src/app/globals.css` | --color-discord token | ✓ VERIFIED | Line 29: `--color-discord: #5865F2;` |
| `frontend/src/app/settings/page.tsx` | Discord server connect section | ✓ VERIFIED | Lines 143-212: complete Discord card |
| `frontend/src/app/overlays/[id]/page.tsx` | Discord source card, relay panel, 2-step dialog | ✓ VERIFIED (with noted gap) | Lines 66-74, 216-351, 353-458, 473-791 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `handlers/sources.go` | `repository/source_repo.go` | `sourceRepo.UpdateConfig call` | ✓ WIRED | Line 551: `h.sourceRepo.UpdateConfig(c.Request.Context(), sourceID, req.Config)` |
| `cmd/main.go` | `handlers/sources.go` | `protected.PATCH route` | ✓ WIRED | Line 203: `protected.PATCH("/:id/sources/:source_id", sourcesHandler.HandleUpdateSourceConfig)` |
| `discord.ts` | `/api/v1/auth/guilds` | `apiClient.get` | ✓ WIRED | discord.ts line 30: `apiClient.get<DiscordGuild[]>('/api/v1/auth/guilds')` |
| `settings/page.tsx` | `discord.ts` | `getGuilds, disconnectGuild, startDiscordOAuth imports` | ✓ WIRED | settings/page.tsx line 16 |
| `overlays/[id]/page.tsx` | `discord.ts` | `getGuilds, getGuildChannels, updateSourceConfig imports` | ✓ WIRED | page.tsx line 31 |
| `relay panel Save` | `discord.updateSourceConfig` | `handleSave` | ✓ WIRED | page.tsx line 391: `await updateSourceConfig(overlayId, source.id, newConfig)` |
| `SourceCard Discord branch` | `PLATFORM_BORDER.discord` | `border-l class lookup` | ✓ WIRED | page.tsx line 242: `PLATFORM_BORDER[source.platform]` where discord → 'border-l-discord' |
| `handleAddDiscordSource` | `/api/v1/overlays/:id/sources (POST)` | `overlaysApi.addSource` | ✗ PARTIAL | Frontend sends config correctly (page.tsx lines 552-557) but backend drops it — HandleAddSource struct has no Config field (sources.go lines 297-307) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| UI-01 | 32-02 | Settings page includes a Discord server connect card showing OAuth2 flow and connected server name/icon | ✓ SATISFIED | settings/page.tsx fully implements connect card, guild list, disconnect dialog |
| UI-02 | 32-03 | Overlay editor allows adding a Discord source with guild selector and inbound channel dropdown | ✗ BLOCKED (partially) | 2-step dialog exists and UI is complete, but backend drops config on POST — guild_id/inbound_channel_id not persisted in DB |
| UI-03 | 32-03 | Per-source relay configuration panel: toggle relay, pick outbound channel, visual indicator of active filter | ✓ SATISFIED | RelayPanel in page.tsx is complete with toggle, picker, loop filter text, Save |
| UI-04 | 32-03 | Discord source cards display connection status and relay active/inactive indicator | ✓ SATISFIED | SourceCard has status badge and relay badge when platform === 'discord' |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `services/overlay-manager/handlers/sources.go` | 297-307, 402 | `HandleAddSource` request struct has no `config` field; Discord config sent by frontend is silently dropped | ✗ Blocker | Discord source persisted with empty config — guild_name, inbound_channel_id, relay_enabled, relay_channel_id all lost |

### Human Verification Required

#### 1. Settings Discord Card Visual

**Test:** Start frontend at http://localhost:3000/settings
**Expected:** Discord section card appears after Data & Privacy, before Danger Zone. Heading reads "Discord". Disconnected state shows "Connect Discord Server" button. If guild connected: icon or initial fallback, guild name, Disconnect button. Clicking Disconnect opens Dialog confirmation.
**Why human:** Visual layout, CDN icon rendering, Dialog UX require browser.

#### 2. OAuth Callback Toast

**Test:** Visit http://localhost:3000/settings?discord=connected
**Expected:** Success toast fires ("Discord server connected!"), URL clears to /settings, guilds re-fetch.
**Why human:** Toast timing, URL cleanup, and re-fetch require live browser.

#### 3. Overlay Editor Discord Source Add Flow (2-step Dialog)

**Test:** Open an overlay at /overlays/[id]. Click "Connect Discord" in Add Source.
**Expected:** Step 1 shows guild list (icon/initial + name). Selecting a guild advances to Step 2 (grouped channel dropdown). Selecting a channel enables "Add" button. Clicking Add creates source and shows toast.
**Why human:** Multi-step dialog flow, guild data, channel grouping require live browser.

#### 4. Discord Source Card Appearance

**Test:** After adding a Discord source, observe the source card.
**Expected:** Blurple (#5865F2) left border. Status badge (Connected/Disconnected). Relay OFF badge. "Configure relay" button visible.
**Why human:** Color rendering, badge visual style require browser.

#### 5. Relay Panel Interaction

**Test:** Click "Configure relay" on a Discord source card.
**Expected:** Panel expands below. Loop filter text visible. Toggle relay ON — outbound channel picker appears. Select channel — Save button enables. Click Save — toast "Relay settings saved", relay badge updates to "Relay ON".
**Why human:** Expandable panel, state transitions, toast require browser.

#### 6. Source Label After Page Refresh (LIKELY TO FAIL)

**Test:** Add a Discord source, reload the page, observe the source card label.
**Expected:** Label reads "GuildName › #channel-name". This requires guild_name to be in source.config.
**Why human (and expected failure):** This test will likely fail because `HandleAddSource` drops the config field. The label renders correctly on first creation (frontend optimistic state) but shows only "#channel-name" after refresh since guild_name is not persisted.

## Gaps Summary

One blocker gap was found:

**Backend does not persist Discord source config on POST.** The frontend correctly sends `{ platform: 'discord', channel_id, channel_name, config: { guild_id, guild_name, inbound_channel_id, relay_enabled, relay_channel_id } }` when creating a Discord source. However, `HandleAddSource` in `sources.go` only binds `platform`, `channel_id`, `channel_name`, `channel_handle` from the request — the `config` object is never parsed and the source is always created with an empty `Config: make(map[string]interface{})`.

Consequences:
- `guild_name` is not stored, so the source card label falls back to `"" › #channel-name` after page refresh
- `inbound_channel_id` is not in config, so the Redis channel registry write at line 424 falls back to `channel_id` (which in the frontend is `selectedChannelId` — the Discord channel snowflake — so the registry write itself still works)
- `relay_enabled` and `relay_channel_id` are not persisted, so relay state is always off after refresh

The fix is small: add `Config map[string]interface{} \`json:"config"\`` to the HandleAddSource request struct and assign it when non-nil.

The relay PATCH endpoint (plan 01) is fully functional and correctly persists config via `updateSourceConfig`. This gap only affects the initial source creation.

---

_Verified: 2026-03-16T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
