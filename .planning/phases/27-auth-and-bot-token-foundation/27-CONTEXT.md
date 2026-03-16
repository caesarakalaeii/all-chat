# Phase 27: Auth and Bot Token Foundation - Context

**Gathered:** 2026-03-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Discord bot authorization ("Add to Server" OAuth2 flow), guild management (connect/disconnect/list), channel listing endpoint, bot permission validation, and Gateway WebSocket connection with correct intents bitmask. No message ingestion, no relay, no overlay editor UI — those are later phases. This phase proves the bot can join a server, validate its permissions, and maintain a stable Gateway connection.

</domain>

<decisions>
## Implementation Decisions

### Disconnect behavior
- Bot leaves the guild via Discord's Leave Guild API (`DELETE /users/@me/guilds/{guild_id}`) — bot visibly disappears from the Discord server's member list
- All discord sources for that guild are hard-deleted immediately from `overlay_chat_sources`
- Disconnect proceeds regardless of whether the Leave Guild API call succeeds — log the error but always clean up local records (bot may still technically be in the server on API failure, but All-Chat state is clean)
- If the bot is manually re-added to a previously disconnected guild, the bot auto-activates on `GUILD_CREATE` Gateway event (no need for re-doing the "Add to Server" flow)

### Multi-guild scope
- Multiple Discord servers supported in v1.5 — no single-server-per-user restriction
- New `discord_guilds` DB table: `user_id`, `guild_id`, `guild_name`, `guild_icon`, `connected_at` — overrides STATE.md "no new DB tables" assumption (that was written before multi-guild was confirmed)
- No hard cap on number of connected servers per user

### Channel listing
- Channels fetched live from Discord REST API (`GET /guilds/{id}/channels`) on each request — no Redis caching
- Return text channels only (type=0 `GUILD_TEXT`) — exclude voice, announcements, forums, threads
- Response grouped by Discord category (parent category name + channels under it), not a flat sorted list

### Permission validation
- Validated at two points: (1) during the OAuth callback after the bot joins, and (2) on each channel list request
- Required permissions: `VIEW_CHANNEL`, `READ_MESSAGE_HISTORY`, `SEND_MESSAGES`
- If permissions are missing at connect time: block the connection, do not save the guild to `discord_guilds`, return a specific error listing which permissions are missing (e.g., "Bot is missing: View Channels, Send Messages. Please re-invite with the correct permissions.")
- If permissions are revoked after initial connect: return the same specific error on channel list requests

### Gateway connection
- Connect with correct intents bitmask: `GUILDS` + `GUILD_MESSAGES` + `MESSAGE_CONTENT` (privileged)
- Startup assertion: on first `READY` event, verify `MESSAGE_CONTENT` intent is active by checking a known non-empty message — log a clear error and halt if MESSAGE_CONTENT is missing (silent empty messages indicate the privileged intent is not enabled in Discord Developer Portal)

### Claude's Discretion
- Where in auth-service the Discord OAuth provider is wired (new file `oauth/discord.go` following existing provider pattern)
- Gateway session resume logic (session_id + resume_gateway_url in Redis per LOAD-03)
- Exact Redis key schema for Gateway session state
- Migration SQL for `discord_guilds` table

</decisions>

<specifics>
## Specific Ideas

- Auto-reconnect on `GUILD_CREATE`: if the bot is manually re-added after disconnect, it should just work without the user re-doing the setup flow
- The `disconnect` flow should be "best effort leave + always clean local state" — never leave the user stuck because Discord had a hiccup

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/auth-service/oauth/platform.go` — `OAuthProvider` interface with `GetAuthURL`, `ExchangeCode`, `GetUserInfo`, `RefreshToken`, `GetPlatform` methods. Discord OAuth provider should implement this interface (new `oauth/discord.go`)
- `services/auth-service/handlers/platform_auth_v2.go` — `PlatformAuthHandlerV2` dispatches to any registered `OAuthProvider` by platform key. Discord slots in as a new provider registration — minimal handler code needed
- `services/overlay-manager/repository/source_repo.go` — `Create`, `ListByOverlayID`, `Delete` already handle `overlay_chat_sources`. Discord sources use `platform="discord"` with `guild_id`, `inbound_channel_id`, `relay_channel_id`, `relay_enabled` in `config` JSONB
- `services/overlay-manager/models/chat_source.go` — `validPlatforms` map needs `"discord": true` added

### Established Patterns
- OAuth flow: generate CSRF token → store state in Redis → redirect to platform → callback validates state → exchange code → store user/token → redirect frontend. Discord "Add to Server" follows the same pattern but captures `guild_id` from the callback instead of fetching user info
- Bot Token: static `DISCORD_BOT_TOKEN` Kubernetes sealed-secret. NOT stored via token-refresh-service. Auth-service reads it from env for all Discord REST calls
- Snowflake IDs as strings — all Discord IDs stored and transmitted as `string` to avoid JS safe-integer truncation above 2^53

### Integration Points
- New `discord_guilds` table migration in `services/auth-service` or a shared migrations directory
- `services/overlay-manager/models/chat_source.go:validPlatforms` — add `"discord"`
- `services/auth-service/cmd/main.go` — register Discord OAuth provider and new channel listing route

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 27-auth-and-bot-token-foundation*
*Context gathered: 2026-03-15*
