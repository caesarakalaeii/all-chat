# Feature Landscape: Discord Listener + Relay

**Domain:** Bidirectional Discord chat source for streaming overlay platform
**Researched:** 2026-03-15
**Confidence:** HIGH (Discord API fundamentals are stable; bot OAuth2 and Gateway have not changed materially since 2022)

---

## Research Notes

External network tools (WebSearch, WebFetch) were unavailable during this session. Findings are based on:
- Training knowledge of Discord API (stable, well-documented, high confidence)
- Direct codebase analysis (existing auth-service, overlay-manager, twitch-listener, existing discord-bot service)
- Official Discord API documentation patterns as of training cutoff (August 2025)

The existing `services/discord-bot/` is a **read-only quota monitor** (Node.js, static bot token, single hardcoded channel). It is NOT the new discord-listener. The new service must be a Go microservice integrated into the full source model, with per-user OAuth2 bot authorization.

---

## Category 1: Bot Authorization (OAuth2 "Add to Server")

### How Discord Bot Authorization Works

Discord uses a **two-token model** for managed bots:

1. **Bot Token** — The platform owns this. It never changes per-server. The bot authenticates to Discord Gateway with this token.
2. **OAuth2 "bot" grant** — Users authorize the bot into their server via an OAuth2 URL. This is NOT a user token exchange. It produces no refresh token and no access token for the platform to store. It only grants the bot **membership in the guild**.

**Bot authorization URL format** (HIGH confidence):
```
https://discord.com/api/oauth2/authorize
  ?client_id={APPLICATION_CLIENT_ID}
  &permissions={PERMISSIONS_INTEGER}
  &scope=bot+applications.commands
  &guild_id={GUILD_ID}               (optional, pre-selects server)
  &redirect_uri={REDIRECT_URI}       (required if using state/callback)
  &response_type=code                (required for callback flow)
  &state={CSRF_STATE}
```

**Scopes required:**
- `bot` — Grants bot membership in the server (required)
- `applications.commands` — Allows slash command registration (optional for v1.5, include for future-proofing)

**Bot permissions integer** required for this feature set:
- `VIEW_CHANNEL` (0x400) — See the channel
- `READ_MESSAGE_HISTORY` (0x10000) — Read past messages (needed for Gateway)
- `SEND_MESSAGES` (0x800) — Write relay messages to outbound channel

Combined minimum: `0x400 | 0x10000 | 0x800 = 0x11C00 = 72704`

**Auth flow for All-Chat:**
1. User clicks "Connect Discord Server" in the setup UI
2. Auth-service generates a state token (CSRF), stores in Redis with TTL
3. Auth-service redirects user to Discord authorization URL
4. Discord shows "Add Bot to Server" dialog — user selects their server and approves
5. Discord redirects to callback with `?code=...&guild_id=...&state=...`
6. Auth-service exchanges `code` for an OAuth2 token (only needed to retrieve guild_id and verify identity — the token itself is low-value for bots)
7. Auth-service stores `guild_id` in the database, associated with the All-Chat user
8. Bot is now present in the guild; the discord-listener can use the bot token to connect and listen

**Key difference from Twitch/YouTube OAuth:** No refresh token to store per user. The platform holds one bot token. The auth callback only needs to capture `guild_id` and confirm the bot was added. The `code` exchange is done only to get the guild_id server-side rather than trusting the client.

**Required Discord application scopes at the application level (Developer Portal):**
- OAuth2 redirect URI registered
- Bot enabled
- Gateway intents enabled (see Category 2)

| Feature | Complexity | Notes |
|---------|------------|-------|
| Bot OAuth2 authorization URL generation | LOW | Standard OAuth2 URL construction, no token exchange complexity |
| State/CSRF handling | LOW | Reuse existing state.go pattern from auth-service |
| OAuth2 callback handler (code exchange for guild_id) | LOW | One HTTP call to Discord token endpoint, extract guild_id from response |
| Guild ID persistence per user | LOW | Add `discord_guilds` table in PostgreSQL |
| Auth-service DiscordOAuth provider | MEDIUM | New oauth.go file following existing twitch.go/youtube.go pattern |
| "Bot already in server" detection | LOW | Discord returns error `{"code": 50035}` if bot is already there — handle gracefully |

---

## Category 2: Inbound (Discord → Overlay)

### Gateway Events and Intents

Discord bots receive messages via the **Discord Gateway** (WebSocket) using a versioned protocol (currently v10). The bot connects with a bot token and declares which **Gateway Intents** it needs.

**Gateway Intent required for reading messages** (HIGH confidence):
- `GUILD_MESSAGES` (Intent bit 9, value 512) — Receive `MESSAGE_CREATE` events in guilds
- `MESSAGE_CONTENT` (Intent bit 15, value 32768) — **Privileged Intent** — Access the actual text content of messages

**Critical: `MESSAGE_CONTENT` is a Privileged Intent.** Since August 2022, bots that are in 100+ servers must apply for verification and privileged intent approval. For bots in fewer than 100 servers (typical for self-hosted or small user-base scenarios), this is enabled in the Developer Portal without review. All-Chat as a managed bot serving many users will eventually need Discord verification for this intent. This is a deployment-time concern, not a code concern, but must be planned for.

**Non-privileged intents needed:**
- `GUILDS` (Intent bit 0, value 1) — Required for the bot to receive guild/channel metadata on startup
- `GUILD_MESSAGES` (Intent bit 9, value 512) — Receive message events (non-privileged for text content since Discord changed policy — but `MESSAGE_CONTENT` is still required to read the body)

**Combined Gateway intent value:** `1 | 512 | 32768 = 33281`

**Relevant Gateway events:**
| Event | Trigger | Use |
|-------|---------|-----|
| `MESSAGE_CREATE` | New message in a channel the bot can see | Primary inbound event |
| `MESSAGE_DELETE` | Message deleted | Forward deletion events downstream |
| `MESSAGE_UPDATE` | Message edited | Optional: forward edit events |
| `READY` | Bot connected to Gateway | Confirm connected guilds list |
| `GUILD_CREATE` | Bot joins a guild or resumes | Populate available channels list |

**MESSAGE_CREATE payload key fields:**
- `id` — Snowflake message ID (use as MessageID in RawChatMessage)
- `channel_id` — Which channel the message is in (used to filter to the configured inbound channel)
- `guild_id` — Guild the message belongs to
- `author.id` — User snowflake ID
- `author.username` — Display name
- `author.avatar` — Avatar hash (construct URL as `https://cdn.discordapp.com/avatars/{user_id}/{avatar}.png`)
- `author.bot` — Boolean; use this for loop prevention (see Category 4)
- `content` — Raw message text (requires `MESSAGE_CONTENT` intent)
- `attachments` — File attachments (optional enrichment)
- `embeds` — Rich embeds (usually skip for chat display)
- `timestamp` — ISO8601 timestamp

**Channel listing (for inbound channel picker):**
- REST endpoint `GET /guilds/{guild_id}/channels` returns all channels the bot can see
- Filter to `type=0` (GUILD_TEXT) channels
- Returns `id`, `name`, `position`, `parent_id` (category)
- No additional scope needed — bot membership + `VIEW_CHANNEL` permission is sufficient

| Feature | Complexity | Notes |
|---------|------------|-------|
| Discord Gateway WebSocket client (bot token auth) | MEDIUM | Standard WebSocket with heartbeat, reconnection, session resumption |
| Gateway intent declaration (`GUILDS + GUILD_MESSAGES + MESSAGE_CONTENT`) | LOW | Integer bitmask at connection time |
| `MESSAGE_CREATE` handler → RawChatMessage | LOW | Map fields to existing schema; `tags` map carries Discord-specific metadata |
| Channel ID filtering (only process configured inbound channel) | LOW | Single equality check on `channel_id` field |
| Redis Streams publisher (`chat:raw`) | LOW | Reuse existing publisher pattern from twitch-listener |
| Privileged Intent (`MESSAGE_CONTENT`) registration | LOW (code), MEDIUM (ops) | Enable in Developer Portal; requires Discord verification at scale |
| `MESSAGE_DELETE` forwarding | LOW | Same pattern as Twitch/YouTube deletion events |
| Bot membership in guild (source registration) | LOW | Source-manager integration: register `discord:{guild_id}:{channel_id}` |
| Load balancing / hash-based sharding | HIGH | Same coordinator pattern as other listeners; `channel_id` as shard key |
| HPA integration | MEDIUM | Same pattern as twitch-listener HPA config |
| Discord Gateway session resumption on reconnect | MEDIUM | Must store `session_id` and `resume_gateway_url` in Redis for stateless pods |

**RawChatMessage mapping for Discord:**
```
Platform:  "discord"
ChannelID: "{guild_id}:{channel_id}"    (composite key, unique across guilds)
UserID:    author.id
Username:  author.username
Text:      content
Timestamp: parsed from message.timestamp
Tags: {
  "guild_id":        guild_id,
  "channel_id":      channel_id,
  "message_id":      id (snowflake),
  "bot":             "true"/"false",    (for loop prevention downstream)
  "avatar_hash":     author.avatar,
  "discriminator":   author.discriminator (may be "0" for new usernames),
  "source_guild_id": guild_id           (for relay loop filter)
}
```

---

## Category 3: Outbound (Overlay → Discord Relay)

### How Relay Typically Works

A relay bot subscribes to the normalized message stream (overlay's Redis Pub/Sub channel) and forwards messages to a designated Discord channel using the Discord REST API (`POST /channels/{channel_id}/messages`).

**Relay message format options:**
1. **Plain text** — `[Platform] Username: message text` — Low complexity, works everywhere
2. **Webhook embed** — Rich embed with platform color, avatar, username — Higher visual quality
3. **Discord webhook** — Bot posts via webhook URL rather than bot token — Simpler auth but harder to manage per-channel

For All-Chat, the bot token approach (option 1 or 2) is preferred over webhooks because:
- The bot is already present in the guild for inbound listening
- Webhooks require separate management per channel
- Consistent auth model

**Rate limits for relay:**
- Discord allows 5 messages per second per channel (global bot rate limit: 50 req/s)
- Burstable: 5 messages in 5 seconds per channel
- For high-traffic overlays relaying to Discord, message batching or throttling must be applied
- Discord returns HTTP 429 with `retry_after` when rate limited

| Feature | Complexity | Notes |
|---------|------------|-------|
| Redis Pub/Sub subscriber for overlay messages | LOW | Reuse existing pub/sub pattern from api-gateway |
| Filter Discord-sourced messages from relay (loop prevention) | LOW | Check `tags["bot"]` or `platform == "discord"` before relaying |
| REST API call: `POST /channels/{channel_id}/messages` | LOW | Single HTTP call with bot token in Authorization header |
| Plain text relay format `[Platform] Username: message` | LOW | String formatting, no external deps |
| Rich embed relay (platform color, avatar, username badge) | MEDIUM | Construct Discord Embed object; needs platform color map |
| Per-overlay relay toggle (enable/disable) | LOW | Config field on ChatSource; checked before publishing |
| Rate limit handling (HTTP 429 + retry_after) | MEDIUM | Exponential backoff; queue messages during backoff window |
| Relay channel configuration (separate from inbound channel) | LOW | Second channel_id field in ChatSource.Config map |
| Message batching for high-traffic overlays | MEDIUM | Buffer messages, flush periodically or at batch size threshold |

---

## Category 4: Loop Prevention

### Why Loops Occur

Without filtering, a relay loop forms:
1. User messages in Discord channel → bot reads via Gateway → normalizes to RawChatMessage
2. Message-processor publishes to overlay's Redis Pub/Sub
3. Relay subscriber picks it up → posts back to Discord channel
4. Discord Gateway fires `MESSAGE_CREATE` again → infinite loop

### Standard Prevention Approaches

**Approach 1: Author Bot Flag (HIGH confidence — industry standard)**
Discord's `MESSAGE_CREATE` event includes `author.bot = true` for any message sent by a bot account. The discord-listener should **reject all messages where `author.bot == true`**. This is the simplest and most reliable filter because:
- The platform's own bot will always have `author.bot = true`
- Other bots' messages are also filtered (reduces noise)
- No application-level state needed

**Approach 2: Source Platform Filtering (HIGH confidence — application-level)**
Before relaying a message to Discord, check if `platform == "discord"`. This is the application-level complement to the Gateway-level filter. Even if a Discord message somehow passes the bot flag check, this prevents it from being relayed back.

**Approach 3: Application ID Check (MEDIUM confidence — belt-and-suspenders)**
Discord includes `application_id` on messages sent by bots with application context. Store the bot's own application ID and skip messages where `application_id` matches. This handles edge cases where `author.bot` might be absent (unusual).

**Recommended combined strategy:**

```
Gateway filter (discord-listener):
  if message.author.bot == true → DROP (never publish to Redis Streams)

Relay filter (relay component of discord-listener):
  if rawMessage.platform == "discord" → SKIP (never relay Discord-sourced messages)

Belt-and-suspenders:
  if rawMessage.tags["source_guild_id"] == relayTargetGuildID → SKIP
  (handles future cross-guild scenarios)
```

| Feature | Complexity | Notes |
|---------|------------|-------|
| Bot author flag check in Gateway handler | LOW | Single boolean field check before publishing |
| Platform check in relay subscriber | LOW | Single string equality check |
| Application ID self-check | LOW | Optional belt-and-suspenders; store bot app ID as env var |
| Source guild tagging in RawChatMessage | LOW | Add `source_guild_id` to tags at publish time |
| Relay audit logging (log skipped messages) | LOW | Structured log at debug level with reason |

---

## Category 5: Setup UI

### UX Flow for Discord Source Configuration

Based on how Twitch/YouTube sources are configured in the existing overlay editor, and standard Discord bot setup patterns:

**Step 1: Server Connection**
- User clicks "Add Discord Source" in overlay editor
- Redirected to Discord OAuth2 authorization page ("Add Bot to Server")
- Returns to All-Chat after authorization; `guild_id` stored
- UI shows the connected server name and icon (fetched via `GET /guilds/{guild_id}`)

**Step 2: Inbound Channel Picker**
- Dropdown populated via `GET /guilds/{guild_id}/channels` (filtered to text channels)
- Channels grouped by category (parent_id)
- User selects which channel to read from
- Selected `channel_id` stored as the source's primary `ChannelID`

**Step 3: Outbound Channel Picker (Optional)**
- Toggle: "Relay overlay messages to Discord"
- If enabled: second dropdown for outbound channel
- Same channel list, different selection
- Selected outbound `channel_id` stored in `ChatSource.Config["relay_channel_id"]`

**Step 4: Overlay Editor Integration**
- Discord source appears alongside Twitch/YouTube/Kick/TikTok in the sources list
- Source card shows: Discord logo, server name, inbound channel name, relay status
- Remove source button disconnects bot from that source (does NOT remove bot from server)

**Deauth / Remove Server flow:**
- "Disconnect server" removes all sources using that guild from all overlays
- Does NOT remove the bot from the server (Discord requires users to manually kick bots)
- Stores a soft-deleted guild record for cleanup tracking

| Feature | Complexity | Notes |
|---------|------------|-------|
| "Connect Discord Server" button in overlay editor | LOW | OAuth2 redirect, consistent with Twitch/YouTube connect buttons |
| Discord OAuth2 callback page | LOW | Minimal page, redirects back to overlay editor with guild_id |
| Guild info display (server name, icon) | LOW | Single REST call `GET /guilds/{guild_id}` with bot token |
| Inbound channel picker (grouped dropdown) | MEDIUM | Fetch channels, group by category, render hierarchical dropdown |
| Outbound channel picker + relay toggle | MEDIUM | Same channel list component, conditional rendering |
| Overlay editor Discord source card | LOW | Same pattern as existing Twitch/YouTube source cards |
| Multiple servers per user | MEDIUM | Data model must support N guilds per user; picker shows which server each source belongs to |
| "Disconnect server" flow | LOW | Soft delete guild record, deactivate all associated sources |
| Permission error handling (bot lacks channel access) | MEDIUM | Detect `403` from channel API, surface actionable error in UI |

---

## Table Stakes

Features users expect. Missing these makes the feature feel incomplete.

| Feature | Why Expected | Complexity | Category |
|---------|--------------|------------|---------|
| OAuth2 "Add Bot to Server" flow | Standard managed-bot pattern; users expect one-click connection, not manual token pasting | LOW-MEDIUM | Auth |
| Inbound channel picker | Users expect to choose which channel to monitor, not configure by raw ID | MEDIUM | UI |
| Messages appear in overlay in real-time | Core value proposition; Discord messages must flow through the same pipeline as Twitch/YouTube | MEDIUM | Inbound |
| `author.bot` filtering (no bot messages in overlay) | Bot spam would pollute overlays immediately; users assume chat bots are filtered | LOW | Loop Prevention |
| Platform label "discord" on messages | Users need to see which platform each message came from | LOW | Inbound |
| Discord-sourced messages not relayed back | Any visible loop would make the feature unusable immediately | LOW | Loop Prevention |
| Relay toggle (on/off per source) | Some users want inbound only; relay is optional | LOW | Outbound |
| Source card in overlay editor showing connected server | Consistent with other platform source cards | LOW | UI |

---

## Differentiators

Features that set this Discord integration apart.

| Feature | Value Proposition | Complexity | Category |
|---------|-------------------|------------|---------|
| Rich embed relay (platform avatar + color) | Messages relayed to Discord look polished, not like raw text dumps | MEDIUM | Outbound |
| Channel hierarchy display in picker (grouped by category) | Discord servers have many channels; flat lists are hard to navigate | MEDIUM | UI |
| Separate inbound and outbound channels | Flexibility: read from #stream-chat, relay to #bot-output; most bridge bots use one channel for both | LOW | Inbound+Outbound |
| Multiple Discord servers per user | Streamers often manage multiple Discord communities; allows one overlay to aggregate from multiple servers | MEDIUM | Auth+UI |
| Message deletion forwarding (Discord → overlay) | Consistent with Twitch/YouTube deletion support already in the platform | LOW | Inbound |

---

## Anti-Features

Features to explicitly not build in v1.5.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Per-user bot token (users paste their own bot token) | Security risk (token stored in All-Chat DB), poor UX, breaks managed bot model | Managed bot with OAuth2 "Add to Server" |
| Slash commands in Discord | Out of scope for a chat relay; adds significant complexity for zero overlay value | Register `applications.commands` scope for future use, implement nothing |
| Discord DM reading | Privacy violation expectations; DMs are not "chat" in the streaming sense | Restrict to guild text channels only |
| Relay embeds with full message history context | Complex threading, Discord embed limits (6000 char total, 25 fields) | Plain text or simple single-embed per message |
| Webhook-based relay (instead of bot token) | Requires separate webhook management per channel, inconsistent auth model | Use bot token for all Discord API calls |
| Voice channel transcription | Entirely different problem domain; requires separate audio pipeline | Out of scope |
| @mention filtering or moderation in relay | Feature creep; moderation is a separate domain | Relay all non-bot messages as-is |
| Custom bot name/avatar per user | Discord requires paid "verified bot" status for per-guild customization | Single managed bot identity |

---

## Feature Dependencies

```
Bot Authorization (OAuth2 "Add to Server")
    └──required by──> Inbound Channel Picker (need guild_id to list channels)
    └──required by──> Outbound Channel Picker (same)
    └──required by──> discord-listener Gateway connection (bot must be in guild)

Gateway Client (discord-listener service)
    └──requires──> Bot Token (managed, single env var)
    └──requires──> MESSAGE_CONTENT privileged intent (Developer Portal setting)
    └──requires──> Guild membership (bot must be added via OAuth2)
    └──produces──> RawChatMessage on chat:raw Redis Stream

RawChatMessage (platform="discord")
    └──consumed by──> message-processor (existing, needs Discord normalizer added)
    └──requires──> ChatSource platform="discord" added to overlay-manager validPlatforms

Relay Component
    └──requires──> Redis Pub/Sub subscriber (overlay:{overlay_id} channel)
    └──requires──> Outbound channel_id in ChatSource.Config
    └──requires──> Loop prevention filter (platform=="discord" check)
    └──requires──> Bot token (same as Gateway client)

Loop Prevention
    └──author.bot check──> Gateway handler (drop on inbound)
    └──platform check──> Relay subscriber (drop on outbound)

Setup UI (overlay editor)
    └──requires──> Auth-service Discord OAuth endpoints
    └──requires──> overlay-manager API: ChatSource platform="discord" support
    └──requires──> Channel listing API endpoint (new endpoint in discord-listener or auth-service)
    └──enhances──> Existing overlay editor source management UI

Load Balancing
    └──requires──> discord-listener integrated with source-manager (leader election)
    └──requires──> hash-based sharding on channel_id
    └──requires──> same HPA pattern as twitch-listener/kick-listener
```

---

## Source Model Integration

The existing `ChatSource` model uses:
- `Platform` — must add `"discord"` to `validPlatforms` map
- `ChannelID` — store as `"{guild_id}:{channel_id}"` (composite, unique across guilds)
- `ChannelName` — store as `"#{channel_name} @ ServerName"`
- `Config` map — store:
  - `"guild_id"` — Discord server snowflake ID
  - `"inbound_channel_id"` — Discord channel snowflake for reading
  - `"relay_channel_id"` — Discord channel snowflake for writing (optional)
  - `"relay_enabled"` — boolean string "true"/"false"

No schema changes needed beyond adding `"discord"` to the platform allow-list and using the `Config` map for Discord-specific fields.

---

## MVP Definition

### Launch With (v1.5)

- "Add Bot to Server" OAuth2 flow (auth-service Discord provider)
- Inbound channel selection and persistence
- `MESSAGE_CREATE` Gateway handler → RawChatMessage → Redis Streams
- `author.bot` filtering (loop prevention, inbound)
- Discord normalizer in message-processor
- `platform="discord"` added to ChatSource valid platforms
- Relay: subscribe to overlay pub/sub, post non-Discord messages to outbound channel
- `platform=="discord"` relay filter (loop prevention, outbound)
- Setup UI: server connect, inbound channel picker, outbound channel picker, relay toggle
- Overlay editor source card for Discord
- Load balancing with hash-based sharding + HPA (consistent with other listeners)
- Gateway session resumption stored in Redis (stateless pods)

### Add After Validation (v1.x)

- Rich embed relay (platform color, avatar) — only if users request it
- Multiple Discord servers per overlay source — only if multistream use case emerges
- `MESSAGE_DELETE` forwarding — consistent with other platforms, low effort
- Channel grouping by category in picker — UX improvement, not blocking

### Future Consideration (v2+)

- Slash commands (`/status`, `/sources`)
- Discord stage channel support
- Role-based message filtering in overlay
- Discord verification for `MESSAGE_CONTENT` at scale (ops concern, not code)

---

## Phase-Specific Complexity Notes

| Phase Area | Complexity Driver | Notes |
|------------|-------------------|-------|
| Auth: OAuth2 bot add-to-server | LOW | Simpler than Twitch/YouTube — no refresh token storage, just capture guild_id |
| Auth: Guild/channel listing API | LOW | Single REST call, straightforward |
| Inbound: Gateway WebSocket client | MEDIUM | Heartbeat, reconnection, session resumption, intent bitmask |
| Inbound: MESSAGE_CONTENT privileged intent | LOW (code) | Single checkbox in Developer Portal; ops concern at scale |
| Inbound: Discord normalizer | LOW | Map ~6 fields to RawChatMessage; no complex tag parsing like IRC |
| Inbound: Load balancing | HIGH | Identical complexity to other listeners; well-understood pattern now |
| Outbound: Relay subscriber + post | LOW | Redis sub + HTTP POST; straightforward |
| Outbound: Rate limit handling | MEDIUM | Must handle HTTP 429 gracefully; queue during backoff |
| Loop prevention: Combined filter | LOW | Two single-field checks; no shared state needed |
| UI: OAuth2 connect flow | LOW | Follows existing Twitch/YouTube connect pattern |
| UI: Channel picker | MEDIUM | Hierarchical dropdown with category grouping is non-trivial |
| UI: Overlay editor integration | LOW | Same source card pattern as other platforms |
| Service architecture: Single vs split | MEDIUM | Decision: single service handles both Gateway inbound and relay outbound, OR separate relay service. Recommendation: single service (bot token shared, simpler deployment) |

---

## Sources

**Discord API (training knowledge, HIGH confidence for these stable APIs):**
- Discord OAuth2 scopes and bot authorization URL: stable since 2022, `bot` + `applications.commands` scopes
- Gateway Intents: `GUILD_MESSAGES` (bit 9), `MESSAGE_CONTENT` (bit 15, privileged), `GUILDS` (bit 0)
- `MESSAGE_CREATE` event structure: `author.bot`, `content`, `channel_id`, `guild_id` — stable since API v8 (2020)
- Rate limits: 5 msg/5s per channel, 50 req/s global — documented behavior since API v10

**Codebase analysis (HIGH confidence):**
- `/services/overlay-manager/models/chat_source.go` — validPlatforms map, ChatSource.Config map pattern
- `/services/auth-service/oauth/twitch.go` + `youtube.go` — OAuthProvider interface, state handling pattern
- `/services/twitch-listener/models/raw_message.go` — RawChatMessage schema, Tags map
- `/services/discord-bot/README.md` — Existing quota monitor bot (Node.js, NOT the new listener)
- `/services/message-processor/README.md` — Normalizer pipeline, platform routing pattern

---
*Feature research for: All-Chat Discord Listener + Relay (v1.5)*
*Researched: 2026-03-15*
*Confidence: HIGH — Discord API fundamentals are stable; codebase analysis confirms integration points*
