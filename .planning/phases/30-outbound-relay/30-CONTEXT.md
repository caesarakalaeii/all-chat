# Phase 30: Outbound Relay - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Non-Discord overlay messages are POSTed to a user-configured Discord channel by discord-listener. Loop-safe: messages with `platform == "discord"` are unconditionally filtered before relay. Config fields (`relay_enabled`, `relay_channel_id`) live in the existing `config` JSONB on `overlay_chat_sources`. No new DB tables. No new services.

</domain>

<decisions>
## Implementation Decisions

### Platform emoji mapping
- Platform-specific emoji per message format: `[emoji] username: text`
  - 🟣 Twitch
  - 🔴 YouTube
  - 💚 Kick
  - 🎵 TikTok
  - 💬 fallback for any unknown / future platform
- Emoji-to-platform mapping lives in discord-listener (not message-processor) — relay-specific formatting, not general pipeline concern

### Relay config discovery
- Add PostgreSQL (pgx) dependency to discord-listener — follows the same pattern as twitch-listener
- Startup + 30-second periodic sync from `overlay_chat_sources` WHERE platform='discord' AND relay_enabled=true
- PostgreSQL `LISTEN chat_source_changes` for instant notification on source create/update/delete (triggers immediate re-sync; same mechanism as twitch-listener)
- Individual Redis Pub/Sub `SUBSCRIBE overlay:{overlay_id}` per overlay that has a relay-enabled Discord source (not a wildcard psubscribe)
- Dynamic subscribe/unsubscribe: when DB sync detects added/removed/toggled sources, relay goroutine adjusts subscriptions at runtime without restart

### Discord REST failure handling
- **Network error / 5xx**: log at ERROR and drop — best-effort delivery, no retry queue, relay stays stateless
- **429 Too Many Requests**: honor the `Retry-After` response header — pause the relay goroutine for that duration and retry the same message once (required by Discord API ToS)
- **403 Forbidden / 404 Not Found**: log at WARN and drop — no automatic `relay_enabled=false` flip in DB; operator fixes via UI

### relay_enabled runtime refresh
- Acceptable lag: up to 30 seconds (pg NOTIFY makes it near-instant on the happy path)
- On toggle-OFF: unsubscribe immediately, discard any buffered messages — no drain (relay is best-effort)
- On toggle-ON: subscribe to `overlay:{overlay_id}` and begin relaying on next message

### Claude's Discretion
- Redis Pub/Sub subscription management internals (goroutine-per-overlay vs. single goroutine with select)
- Exact PostgreSQL query to fetch relay-enabled Discord sources and their relay_channel_id
- Whether the relay config is a struct or a map derived from `config` JSONB
- Internal naming of the relay component (e.g. `relay.Manager`, `relay.Worker`)

</decisions>

<specifics>
## Specific Ideas

- twitch-listener's `channels.Manager` (30s ticker + pg LISTEN on `chat_source_changes`) is the direct reference implementation — port the discovery pattern into a `relay.Manager` in discord-listener
- The `config` JSONB keys to read: `relay_channel_id` (string, Snowflake), `relay_enabled` (bool)
- Discord REST endpoint: `POST /channels/{relay_channel_id}/messages` with body `{"content": "[emoji] username: text"}`
- The loop-safety filter (`platform == "discord"`) must be applied before any relay action — not configurable, always on

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/twitch-listener/channels/manager.go` — `Manager` with `syncTicker` (30s) + `LISTEN chat_source_changes` + dynamic join/depart pattern. Port this discovery loop into a `relay.Manager` in discord-listener
- `services/discord-listener/cmd/main.go` — two goroutine groups already planned (Gateway inbound + relay outbound). Add relay.Manager startup alongside the Gateway goroutine
- `services/message-processor/models/message.go:UnifiedChatMessage` — `Platform` and `User.Username` and `Message.Text` fields are what the relay reads from the Pub/Sub payload
- `services/message-processor/publisher/pubsub_publisher.go` — confirms Pub/Sub channel format: `overlay:{overlay_id}`

### Established Patterns
- **Best-effort delivery**: all other listeners log and drop on publish errors — relay follows the same pattern for network/5xx
- **pg LISTEN/NOTIFY**: `LISTEN chat_source_changes` already in use by twitch-listener for instant config updates — same channel available
- **Interface injection for testability**: `SessionStore`, `ChannelRegistry`, `GuildCache` all use interfaces. Apply the same pattern to the relay's Discord REST client (e.g. `DiscordPoster` interface) for unit testing without HTTP
- **Snowflake IDs as strings**: `relay_channel_id` is a string throughout — Phase 27 decision

### Integration Points
- `services/discord-listener/go.mod` — add `github.com/jackc/pgx/v5` dependency (not present yet)
- `services/discord-listener/cmd/main.go` — wire DB pool, relay.Manager startup, and relay goroutine group
- `overlay_chat_sources` table — query WHERE platform='discord' for relay config; `config` JSONB fields `relay_enabled` and `relay_channel_id`
- Redis Pub/Sub — relay subscribes to `overlay:{overlay_id}` channels (same channels api-gateway WebSocket uses)

</code_context>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 30-outbound-relay*
*Context gathered: 2026-03-16*
