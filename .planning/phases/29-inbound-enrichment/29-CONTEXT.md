# Phase 29: Inbound Enrichment - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Discord messages carry deletion events and resolved mention text through the existing platform pipelines. Covers INBD-03 (deletion propagation) and INBD-04 (mention resolution). No relay, no load balancing — those are later phases. The result: a deleted Discord message disappears from overlays, and @alice/@general renders correctly instead of raw Snowflake IDs.

</domain>

<decisions>
## Implementation Decisions

### MESSAGE_DELETE handling
- Handle both `MESSAGE_DELETE` (single) and `MESSAGE_DELETE_BULK` (multi) dispatch events
- `MESSAGE_DELETE_BULK`: expand to N individual `RawChatMessage` deletion events — one per message ID in the `ids` array. No size cap (100 messages max per Discord spec, not a pipeline concern)
- Both `MESSAGE_DELETE` and `MESSAGE_DELETE_BULK` filter by channel: only emit deletion events for configured inbound channels (same filter logic as MESSAGE_CREATE from Phase 28)
- Deletion event format mirrors Twitch: `EventType: "message_deletion"`, `EventData: {"deletion_type": "single", "target_msg_id": "<snowflake_string>"}`
- Discord Snowflake IDs are strings (Phase 27 decision) — compatible with existing message ID registry key schema, no adaptation needed
- `NormalizeDeletion()` shared function in message-processor handles Discord deletions as-is — no new deletion type needed

### Mention resolution — user mentions
- Resolution happens in **discord-listener**, inline before publishing to Redis Streams — `MESSAGE_CREATE` payload carries the `mentions` array (full user objects), so data is available at the right moment
- Replace both `<@USER_ID>` and `<@!USER_ID>` (guild member variant) using the same `mentions` array — treat both token formats identically
- Display name for resolved mention: `global_name` with fallback to `username` (matches Discord's own display convention)
- Output format: `@alice` — keep the `@` prefix, consistent with how other platforms render mentions in the overlay
- Unresolvable user mention fallback (ID not in `mentions` array): output `@unknown`, log at DEBUG

### Mention resolution — channel mentions
- `<#CHANNEL_ID>` mentions: maintain a **Redis channel name cache** in discord-listener (`discord:channels:names:{channel_id}` → channel name string)
- Cache populated from `GUILD_CREATE` event on connect (lists all channels bot can see) — gives instant resolution with no REST calls
- Cache kept current via `CHANNEL_UPDATE`, `CHANNEL_CREATE`, `CHANNEL_DELETE` dispatch events
- No new Discord intents required — GUILDS intent (already configured, `1 << 0`) covers GUILD_CREATE and all CHANNEL_* events
- Unresolvable channel mention fallback (cache miss): output `#channel`, log at DEBUG

### Mention resolution — role mentions
- `<@&ROLE_ID>` mentions: resolve to `@RoleName` using a **Redis role name cache** (`discord:roles:names:{role_id}` → role name string)
- Role cache populated from `GUILD_CREATE` event's roles array (same event as channel cache population)
- Updated on `GUILD_ROLE_UPDATE`, `GUILD_ROLE_CREATE`, `GUILD_ROLE_DELETE` dispatch events (covered by GUILDS intent)
- Unresolvable role mention fallback: output `@unknown`, log at DEBUG (consistent with user mention fallback)

### Claude's Discretion
- Redis key schema for channel name cache and role name cache
- Whether channel and role caches share a single GUILD_CREATE handler or are separate
- Exact regex/string replacement implementation for mention tokens
- Whether `mentions` array lookup is by ID map (built once per message) or linear scan
- Order of mention substitution (user → channel → role, or combined single-pass)

</decisions>

<specifics>
## Specific Ideas

- The Phase 28 `ChannelRegistry` already uses `discord:channels:{channel_id}` keys for source config — the new channel name cache should use a distinct key prefix (e.g., `discord:guild:channels:{channel_id}`) to avoid collision
- GUILD_CREATE is already dispatched on connect — the cache population can hook into the existing `case "READY"` / dispatch handling alongside current session state storage

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/discord-listener/gateway/types.go` — `GatewayPayload`, `MessageCreateData`, `DiscordUser`, `DiscordMember` already defined. Add `MessageDeleteData` and `MessageDeleteBulkData` structs here. Also add `Mentions []DiscordUser` field to `MessageCreateData`
- `services/discord-listener/gateway/client.go` — `OpDispatch` switch already handles `READY` and `MESSAGE_CREATE`. Add `MESSAGE_DELETE`, `MESSAGE_DELETE_BULK`, `GUILD_CREATE`, `CHANNEL_UPDATE`, `CHANNEL_CREATE`, `CHANNEL_DELETE`, `GUILD_ROLE_UPDATE`, etc. cases here
- `services/message-processor/normalizer/normalizer.go:NormalizeDeletion()` — shared function already handles `single`/`batch`/`clear` deletion types with `target_msg_id` — Discord deletion events plug in directly
- `services/message-processor/registry/buffer.go:RedisDeletionBuffer` — already handles race condition (deletion arrives before original message). Discord deletions use this buffer via the same pipeline path

### Established Patterns
- **Deletion event pattern**: `EventType: "message_deletion"` + `EventData: {"deletion_type": "single", "target_msg_id": "..."}` — established by Twitch listener (`irc/parser.go:ParseClearMessage`)
- **Interface injection for testability**: Phase 27/28 introduced `SessionStore` and `ChannelRegistry` interfaces. Apply same pattern for `GuildCache` (channel + role name lookups) to enable unit testing without Redis
- **Channel filter at source**: Phase 28 decided filtering happens in discord-listener before publishing. Same filter applies to deletion events — only configured channels propagate

### Integration Points
- `services/discord-listener/gateway/client.go:OpDispatch case` — add `MESSAGE_DELETE`, `MESSAGE_DELETE_BULK`, `GUILD_CREATE`, `CHANNEL_UPDATE`, `GUILD_ROLE_UPDATE` (and related) handling
- `services/discord-listener/gateway/types.go` — add `MessageDeleteData`, `MessageDeleteBulkData`, `GuildCreateData` (with channels + roles arrays), `DiscordChannel`, `DiscordRole` structs
- `services/discord-listener/publisher/` — deletion events flow through existing publisher (same `RawChatMessage` schema, `EventType: "message_deletion"`)
- Redis: new key namespace for guild channel and role name caches (distinct from Phase 28 channel registry keys)
- `services/discord-listener/cmd/main.go` — initialize `GuildCache` (Redis-backed), wire into gateway client

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 29-inbound-enrichment*
*Context gathered: 2026-03-16*
