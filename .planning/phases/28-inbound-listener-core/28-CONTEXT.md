# Phase 28: Inbound Listener Core - Context

**Gathered:** 2026-03-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend the existing `GatewayClient` to handle `MESSAGE_CREATE` dispatch events, filter them to the configured inbound channel, and publish them to Redis Streams (`chat:raw`) as `RawChatMessage`. Add a `discord_normalizer.go` in `message-processor` to normalize Discord messages to `UnifiedChatMessage`. No relay, no deletions, no mention resolution — those are later phases. This phase delivers Discord messages appearing in overlays as a first-class chat source.

</domain>

<decisions>
## Implementation Decisions

### Channel filtering
- Filter happens in **discord-listener before publishing** — never write non-configured channel messages to Redis Streams
- Source of truth: **Redis cache populated by overlay-manager** — stores `channel_id → {guild_id, overlay_id, source_id}` mapping
- Redis Pub/Sub invalidation from overlay-manager — when a Discord source is created/deleted, overlay-manager publishes an invalidation event; discord-listener reloads its in-memory set immediately
- On MESSAGE_CREATE with no matching configured channel: **log at DEBUG level and drop** — no WARN noise
- Same Redis channel registry provides the `overlay_id` for the `RawChatMessage` at publish time (one lookup per message)

### Discord user display in overlays
- **DisplayName**: guild nickname (`member.nick`) with fallback to `author.username` — matches Discord's own display convention
- **Username (ID field)**: always `author.username` (the stable, unique identifier)
- **User.Color**: top role's color, but **only if not `#000000`** — `#000000` means no color assigned in Discord; leave `User.Color` empty in that case
- **Badges**: map specific role names to badges — if role name matches `moderator`, `admin`, `vip` (case-insensitive), add it as a badge. Skip all other roles. No badge system for generic roles.
- **Bot filtering**: filter in **discord-listener before publishing** — if `author.bot == true`, drop the message silently before it ever reaches Redis Streams

### Empty-content safety
- If first `MESSAGE_CREATE` arrives with empty `content` (indicates `MESSAGE_CONTENT` privileged intent not enabled in Discord Developer Portal): **log ERROR and halt the service**
- Detection is **reactive only** — triggered on first empty MESSAGE_CREATE, no synthetic startup probe
- This enforces the operator-must-fix behavior rather than silently running in a degraded state

### Publisher architecture
- Dedicated **`publisher/` package** in `services/discord-listener/` — mirrors `twitch-listener/publisher/` and `kick-listener/publisher/` patterns
- `Publisher` struct with a `Publish(ctx, msg *RawChatMessage) error` method
- **Duplicate the serialization pattern** — no shared library; each service has its own `go.mod` and owns its publisher implementation
- `RawChatMessage.OverlayID` set at publish time from the Redis channel registry lookup
- `RawChatMessage.ChannelID` = Discord Snowflake channel ID (as string)
- `RawChatMessage.Platform` = `"discord"`

### Claude's Discretion
- Redis key schema for the channel registry (`discord:channels:{channel_id}` or similar)
- Exact Redis Pub/Sub channel name for source invalidation events
- RawChatMessage field mapping for Discord-specific fields (member roles in EventData or Tags)
- Whether `member.nick` is available on the `MESSAGE_CREATE` payload's `member` field or requires a separate API call (if separate call required, fall back to username only)
- Internal structure of the dispatcher in `gateway/client.go` — whether MESSAGE_CREATE dispatch is handled inline or via a registered handler interface

</decisions>

<specifics>
## Specific Ideas

- The Phase 27 `GatewayClient.Connect()` already has the dispatch switch at `case OpDispatch:` — Phase 28 adds `MESSAGE_CREATE` handling there, keeping the existing `READY` handling in place
- The existing `TODO(Phase 28): halt if first MESSAGE_CREATE has empty content` comment in `gateway/client.go` is the exact insertion point for the empty-content halt logic
- The Redis channel registry doubles as both the filter (is this channel configured?) and the overlay_id lookup (what overlay does this channel serve?) — a single Redis GET per message

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/discord-listener/gateway/client.go` — `GatewayClient.Connect()` already has the `OpDispatch` case; add `MESSAGE_CREATE` branch here. `SessionStore` interface pattern can be reused for a `ChannelRegistry` interface (testable Redis injection)
- `services/discord-listener/gateway/types.go` — `GatewayPayload`, `ReadyEventData` types already defined; add `MessageCreateData` struct here
- `services/message-processor/normalizer/normalizer.go` — `Normalizer` interface: `Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error)`. Discord normalizer implements this exactly
- `services/message-processor/normalizer/kick_normalizer.go` — canonical normalizer implementation pattern to follow: struct, constructor, `Normalize()` method, platform guard, field extraction, metadata map
- `services/message-processor/consumer/stream_consumer.go` — confirms `chat:raw` stream key, `RawChatMessage` schema, consumer group pattern
- `services/twitch-listener/publisher/` and `services/kick-listener/publisher/` — publisher package patterns to mirror for `services/discord-listener/publisher/`

### Established Patterns
- **Normalizer pattern**: `type DiscordNormalizer struct{}` → `NewDiscordNormalizer()` → `Normalize()` with platform guard. Register in message-processor router alongside existing normalizers
- **Publisher pattern**: Each listener has its own `publisher/` package. `Publisher` wraps `*redis.Client` and serializes `RawChatMessage` fields to Redis Streams `XADD` call
- **Interface injection for testability**: Phase 27 introduced `SessionStore` interface — apply same pattern for `ChannelRegistry` (Redis) and publisher in gateway dispatcher to keep unit tests clean
- **Snowflake IDs as strings**: All Discord IDs (channel_id, guild_id, user_id, message_id) stored and transmitted as `string`

### Integration Points
- `services/discord-listener/gateway/client.go:OpDispatch case` — add `MESSAGE_CREATE` handling
- `services/message-processor/normalizer/` — add `discord_normalizer.go`, register in router
- `services/message-processor/cmd/main.go` or router — register `DiscordNormalizer` for `"discord"` platform key
- `services/overlay-manager` — must publish Redis Pub/Sub invalidation events when Discord sources are created/deleted (new behavior required in overlay-manager as part of this phase)
- Redis: new key schema for channel registry (`discord:channels:{channel_id}` style) and Pub/Sub channel for invalidation

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 28-inbound-listener-core*
*Context gathered: 2026-03-15*
