# Phase 29: Inbound Enrichment - Research

**Researched:** 2026-03-16
**Domain:** Discord Gateway dispatch handling — deletion propagation + mention resolution in discord-listener
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### MESSAGE_DELETE handling
- Handle both `MESSAGE_DELETE` (single) and `MESSAGE_DELETE_BULK` (multi) dispatch events
- `MESSAGE_DELETE_BULK`: expand to N individual `RawChatMessage` deletion events — one per message ID in the `ids` array. No size cap (100 messages max per Discord spec, not a pipeline concern)
- Both `MESSAGE_DELETE` and `MESSAGE_DELETE_BULK` filter by channel: only emit deletion events for configured inbound channels (same filter logic as MESSAGE_CREATE from Phase 28)
- Deletion event format mirrors Twitch: `EventType: "message_deletion"`, `EventData: {"deletion_type": "single", "target_msg_id": "<snowflake_string>"}`
- Discord Snowflake IDs are strings (Phase 27 decision) — compatible with existing message ID registry key schema, no adaptation needed
- `NormalizeDeletion()` shared function in message-processor handles Discord deletions as-is — no new deletion type needed

#### Mention resolution — user mentions
- Resolution happens in **discord-listener**, inline before publishing to Redis Streams — `MESSAGE_CREATE` payload carries the `mentions` array (full user objects), so data is available at the right moment
- Replace both `<@USER_ID>` and `<@!USER_ID>` (guild member variant) using the same `mentions` array — treat both token formats identically
- Display name for resolved mention: `global_name` with fallback to `username` (matches Discord's own display convention)
- Output format: `@alice` — keep the `@` prefix, consistent with how other platforms render mentions in the overlay
- Unresolvable user mention fallback (ID not in `mentions` array): output `@unknown`, log at DEBUG

#### Mention resolution — channel mentions
- `<#CHANNEL_ID>` mentions: maintain a **Redis channel name cache** in discord-listener (`discord:channels:names:{channel_id}` → channel name string)
- Cache populated from `GUILD_CREATE` event on connect (lists all channels bot can see) — gives instant resolution with no REST calls
- Cache kept current via `CHANNEL_UPDATE`, `CHANNEL_CREATE`, `CHANNEL_DELETE` dispatch events
- No new Discord intents required — GUILDS intent (already configured, `1 << 0`) covers GUILD_CREATE and all CHANNEL_* events
- Unresolvable channel mention fallback (cache miss): output `#channel`, log at DEBUG

#### Mention resolution — role mentions
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

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INBD-03 | Discord message deletions are propagated through the existing deletion pipeline | `MESSAGE_DELETE` / `MESSAGE_DELETE_BULK` dispatch handling in gateway/client.go + existing `NormalizeDeletion()` in message-processor handles the downstream path with zero changes |
| INBD-04 | Discord @user and #channel mentions are resolved to human-readable names in message text | Inline resolution in `HandleMessageCreate` using `mentions` array (already on payload) + `GuildCache` interface (Redis-backed channel/role name caches populated from `GUILD_CREATE`) |
</phase_requirements>

---

## Summary

Phase 29 adds two complementary enrichments to the existing Discord message pipeline: deletion event propagation and mention text resolution. Both features live entirely within `services/discord-listener` and require no changes to `message-processor`.

For deletions, the Discord Gateway emits `MESSAGE_DELETE` (single) and `MESSAGE_DELETE_BULK` (multi) dispatch events. The existing `ChannelRegistry.GetOverlayForChannel` filter applies identically to deletion events — only configured channels propagate downstream. Each deletion is published as a `RawChatMessage` with `EventType: "message_deletion"` and `EventData: {"deletion_type": "single", "target_msg_id": "<snowflake>"}`, matching the Twitch pattern in `irc/parser.go:ParseClearMessage`. The downstream `NormalizeDeletion()` function in `message-processor/normalizer/normalizer.go` and `RedisDeletionBuffer` in `registry/buffer.go` already handle this format without modification.

For mention resolution, `MESSAGE_CREATE` payloads already carry a `mentions` array of full `DiscordUser` objects, so user mention resolution (`<@ID>`, `<@!ID>`) requires no external lookups. Channel and role mentions require a `GuildCache` backed by Redis string keys, populated eagerly from the `GUILD_CREATE` dispatch event that Discord sends on every connect. The GUILDS intent (`1 << 0`) already configured in `RequiredIntents` covers all guild/channel/role lifecycle events needed to keep the cache current.

**Primary recommendation:** Implement `GuildCache` interface (Redis-backed), wire it into `GatewayClient`, add `GUILD_CREATE` / `CHANNEL_*` / `GUILD_ROLE_*` dispatch case handlers, add `MESSAGE_DELETE` / `MESSAGE_DELETE_BULK` handlers with channel filter, and add `ResolveMentions()` helper called inside the enriched `HandleMessageCreate` after adding `Mentions []DiscordUser` to `MessageCreateData`.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9 (already in go.mod) | Guild name cache (channel + role), deletion events | Already used for SessionStore and ChannelRegistry |
| `regexp` (stdlib) | Go stdlib | Mention token replacement (`<@ID>`, `<@!ID>`, `<#ID>`, `<@&ID>`) | No external dependency needed for simple pattern matching |
| `github.com/google/uuid` | already used in twitch-listener | Deletion event MessageID generation | Consistent with Twitch pattern |
| `github.com/stretchr/testify` | already in go.mod | Unit tests for new handlers and GuildCache | Already used in gateway tests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.uber.org/zap` | already in go.mod | DEBUG logging for fallback/cache-miss paths | Consistent with existing service logging |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Redis string keys for guild cache | In-memory map | Redis survives pod restarts; in-memory simpler but loses cache on reconnect — Redis preferred since GUILD_CREATE repopulates anyway, but Redis avoids thundering-herd on restart |
| `regexp.MustCompile` pre-compiled patterns | `strings.Replace` per-call | Regex handles both `<@ID>` and `<@!ID>` variants cleanly; strings.Replace would require two separate passes per user token |

**Installation:** No new dependencies required — all needed packages are already in `services/discord-listener/go.mod`.

---

## Architecture Patterns

### Recommended Project Structure

Phase 29 changes are concentrated in two areas:

```
services/discord-listener/
├── gateway/
│   ├── types.go          # Add MessageDeleteData, MessageDeleteBulkData,
│   │                     #   GuildCreateData, DiscordChannel, DiscordRole structs;
│   │                     #   add Mentions []DiscordUser to MessageCreateData
│   ├── client.go         # Add GuildCache field; add dispatch case handlers for
│   │                     #   MESSAGE_DELETE, MESSAGE_DELETE_BULK, GUILD_CREATE,
│   │                     #   CHANNEL_CREATE/UPDATE/DELETE, GUILD_ROLE_CREATE/UPDATE/DELETE;
│   │                     #   add HandleMessageDelete, HandleMessageDeleteBulk,
│   │                     #   HandleGuildCreate (exported for unit tests);
│   │                     #   call ResolveMentions inside HandleMessageCreate
│   ├── mentions.go       # New: ResolveMentions(content, mentions, guildCache) helper
│   ├── client_test.go    # Existing
│   └── message_create_test.go  # Existing; extend with mention resolution tests
├── guild/
│   ├── cache.go          # New: GuildCache interface + RedisGuildCache implementation
│   └── cache_test.go     # New: unit tests with mock Redis
└── cmd/
    └── main.go           # Wire RedisGuildCache into GatewayClient constructor
```

### Pattern 1: Interface Injection for Cache (GuildCache)

**What:** Define a `GuildCache` interface with `SetChannelName`, `GetChannelName`, `SetRoleName`, `GetRoleName` methods. Implement with Redis string keys. Inject via `GatewayClient` constructor.

**When to use:** Anywhere a Redis-backed cache is needed and unit testability without live Redis is required. Established by `SessionStore` and `ChannelRegistry` in Phase 27/28.

```go
// Source: established pattern in services/discord-listener/gateway/client.go
type GuildCache interface {
    SetChannelName(ctx context.Context, channelID, name string) error
    GetChannelName(ctx context.Context, channelID string) (name string, found bool, err error)
    SetRoleName(ctx context.Context, roleID, name string) error
    GetRoleName(ctx context.Context, roleID string) (name string, found bool, err error)
}
```

Redis key schema (Claude's discretion — recommended):
- Channel names: `discord:guild:channels:{channel_id}` → channel name string
- Role names: `discord:guild:roles:{role_id}` → role name string

These are distinct from the Phase 28 channel registry keys (`discord:channels:{channel_id}`) which store overlay/source config JSON.

### Pattern 2: Deletion Event Construction (mirrors Twitch)

**What:** Build a `RawChatMessage` with `EventType: "message_deletion"` and `EventData` map containing `"deletion_type": "single"` and `"target_msg_id": <snowflake_string>`. Use `uuid.New().String()` for `MessageID` (the deletion event's own ID, not the message being deleted).

**When to use:** Both `MESSAGE_DELETE` and each entry in `MESSAGE_DELETE_BULK` ids array.

```go
// Source: services/twitch-listener/irc/parser.go:ParseClearMessage (established pattern)
// Deletion event published via existing publisher.RawMessage — add EventType/EventData fields
rawMsg := map[string]interface{}{
    "message_id": uuid.New().String(), // deletion event's own ID
    "platform":   "discord",
    "overlay_id": overlayID,
    "channel_id": msg.ChannelID,
    "event_type": "message_deletion",
    "event_data": map[string]interface{}{
        "deletion_type": "single",
        "target_msg_id": msg.ID, // Discord Snowflake string
    },
    "timestamp": time.Now().UTC(),
}
```

Note: `publisher.RawMessage` struct does not currently have `EventType`/`EventData` fields. Either add them to `publisher.RawMessage`, or use the `map[string]interface{}` path that goes through `publisherAdapter` (which re-marshals to JSON). Verify whether `message-processor` reads `event_type`/`event_data` from the stream entry's `data` field — it reads `RawChatMessage` which does have these fields. The `publisherAdapter` JSON round-trip will preserve them if the map keys match `json:"event_type"` tags on `models.RawChatMessage`.

**Important:** `publisher.RawMessage` (discord-listener) is distinct from `models.RawChatMessage` (message-processor). The `publisherAdapter` marshals the map to JSON and unmarshals into `publisher.RawMessage`. To pass `event_type` and `event_data` through, either:
1. Add `EventType string` and `EventData map[string]interface{}` fields to `publisher.RawMessage`, or
2. Rely on the existing `map[string]interface{}` → JSON → `publisher.RawMessage` path and add those fields to `publisher.RawMessage`.

Option 1 is cleaner. The `publisherAdapter` then passes through unchanged.

### Pattern 3: Mention Resolution (single-pass, map-based)

**What:** Build an ID→display-name map from the `mentions` array once per message. Apply regex replacements for `<@ID>`, `<@!ID>`, `<#ID>`, `<@&ID>` in a single pass or ordered sequential passes.

**When to use:** Inside `HandleMessageCreate`, after channel registry lookup and before building the `rawMsg` map.

```go
// Source: Claude's discretion — recommended implementation
// Build user lookup map (O(1) per replacement)
mentionMap := make(map[string]string, len(msg.Mentions))
for _, u := range msg.Mentions {
    name := u.GlobalName
    if name == "" {
        name = u.Username
    }
    mentionMap[u.ID] = name
}

content := ResolveMentions(msg.Content, mentionMap, guildCache, ctx, log)
```

`ResolveMentions` function (in `gateway/mentions.go`):
1. Replace `<@!ID>` and `<@ID>` using mentionMap; fallback `@unknown`
2. Replace `<#ID>` via `guildCache.GetChannelName`; fallback `#channel`
3. Replace `<@&ID>` via `guildCache.GetRoleName`; fallback `@unknown`

Pre-compiled package-level regex vars:
```go
var (
    reMentionUser    = regexp.MustCompile(`<@!?(\d+)>`)
    reMentionChannel = regexp.MustCompile(`<#(\d+)>`)
    reMentionRole    = regexp.MustCompile(`<@&(\d+)>`)
)
```

### Pattern 4: GUILD_CREATE Handler

**What:** On `GUILD_CREATE` dispatch, unmarshal the `channels` and `roles` arrays and bulk-set name keys in Redis.

**When to use:** Single handler called from the `OpDispatch` switch in `Connect()`. Can share a single handler function that populates both channel and role caches.

```go
// GuildCreateData struct (add to types.go)
type GuildCreateData struct {
    ID       string          `json:"id"`
    Channels []DiscordChannel `json:"channels"`
    Roles    []DiscordRole    `json:"roles"`
}

type DiscordChannel struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Type int    `json:"type"` // 0 = text, useful for filtering
}

type DiscordRole struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

### Anti-Patterns to Avoid

- **Making REST calls per message for mention resolution:** The `mentions` array is already present on `MESSAGE_CREATE`. REST calls add latency and consume rate limit budget.
- **Using the Phase 28 channel registry key prefix for guild channel names:** `discord:channels:{id}` stores overlay/source config JSON. Use `discord:guild:channels:{id}` for plain name strings.
- **Blocking service startup on GUILD_CREATE:** GUILD_CREATE arrives asynchronously after READY. The service should start serving MESSAGE_CREATE immediately; missing cache entries fall back gracefully.
- **Halting the service on GuildCache errors:** Cache errors should log WARN and fall back to the literal token (e.g., `<#ID>`) or the agreed fallback strings, not halt.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mention token regex | Custom string scanning | `regexp.MustCompile` (stdlib) | Handles overlapping match positions, `<@!ID>` vs `<@ID>` variants, and `<@&ID>` cleanly |
| Deletion race condition handling | Custom buffer | `registry.RedisDeletionBuffer` (already in message-processor) | Already handles deletion-before-message-arrives race with 60s TTL |
| Channel name lookup via REST | HTTP call to Discord API | Redis guild cache from GUILD_CREATE | Zero latency, no rate limit consumption, GUILDS intent already covers lifecycle events |
| Deletion downstream normalization | New normalizer | `NormalizeDeletion()` in message-processor/normalizer | Already handles `single` type with `target_msg_id`; Discord Snowflake strings work as-is |

**Key insight:** The deletion pipeline from Twitch (listener → Redis Stream → message-processor `NormalizeDeletion` → RedisDeletionBuffer) is fully reusable for Discord. The only new work is emitting the correctly-shaped `RawChatMessage` deletion events from discord-listener.

---

## Common Pitfalls

### Pitfall 1: publisher.RawMessage missing EventType/EventData fields
**What goes wrong:** Deletion events published via `publisherAdapter` lose `event_type`/`event_data` because `publisher.RawMessage` struct doesn't have these fields — JSON round-trip silently drops them.
**Why it happens:** `publisher.RawMessage` was designed for chat messages; deletion event fields were not included in Phase 28.
**How to avoid:** Add `EventType string` and `EventData map[string]interface{}` fields to `publisher.RawMessage` in `services/discord-listener/publisher/stream_publisher.go`. Annotate with `json:"event_type,omitempty"` and `json:"event_data,omitempty"`.
**Warning signs:** message-processor processes deletion events but `EventType` is empty string, causing `NormalizeDeletion` to return `deletion_type: "unknown"`.

### Pitfall 2: Channel cache key collision with Phase 28 ChannelRegistry
**What goes wrong:** Using `discord:channels:{channel_id}` for guild channel name cache overwrites the JSON config blob used by `ChannelRegistry.GetOverlayForChannel`.
**Why it happens:** Reusing an intuitive key prefix without checking existing usage.
**How to avoid:** Use `discord:guild:channels:{channel_id}` for plain name strings (as noted in CONTEXT.md specifics section).
**Warning signs:** `GetOverlayForChannel` returns JSON unmarshal errors after GUILD_CREATE runs.

### Pitfall 3: GUILD_CREATE arrives before ChannelRegistry is populated
**What goes wrong:** Channel filter in deletion/mention handlers rejects events because `GetOverlayForChannel` returns `found=false` even for configured channels — overlay-manager hasn't written channel config keys yet.
**Why it happens:** GUILD_CREATE fires on connect; overlay-manager writes to Redis independently. Boot order is non-deterministic.
**How to avoid:** Guild cache population (channel/role names) is separate from the channel registry (overlay config). Guild cache populates from GUILD_CREATE regardless of channel registration state. Channel filter (`GetOverlayForChannel`) is only applied to MESSAGE_DELETE, not to GUILD_CREATE cache population.
**Warning signs:** Deletion events from configured channels are silently dropped on cold start.

### Pitfall 4: `<@!ID>` guild member mention variant missed
**What goes wrong:** Only `<@ID>` pattern matched; `<@!ID>` (guild member nickname variant) renders as raw token in overlay.
**Why it happens:** Documentation often only shows `<@ID>`; `<@!ID>` is a legacy/guild-member variant still present in some clients.
**How to avoid:** Use `<@!?(\d+)>` regex to match both. CONTEXT.md explicitly requires both variants.
**Warning signs:** Test with a message that has both variants — `<@!ID>` fails to resolve.

### Pitfall 5: MESSAGE_DELETE_BULK channel_id absent on individual entries
**What goes wrong:** `MESSAGE_DELETE_BULK` payload has `channel_id` and `guild_id` at the top level, with `ids` as an array. The individual IDs are bare Snowflake strings, not objects with `channel_id`.
**Why it happens:** Misreading the Discord API docs — assuming each ID has its own channel metadata.
**How to avoid:** `MessageDeleteBulkData` has `ChannelID string`, `GuildID string`, `IDs []string`. Expansion to N deletion events uses the single `channel_id` from the top-level payload for all entries.
**Warning signs:** Compiler error or runtime panic trying to access `.channel_id` on string slice elements.

---

## Code Examples

### New types in gateway/types.go

```go
// MessageDeleteData is the payload for the MESSAGE_DELETE dispatch event
type MessageDeleteData struct {
    ID        string `json:"id"`
    ChannelID string `json:"channel_id"`
    GuildID   string `json:"guild_id"`
}

// MessageDeleteBulkData is the payload for the MESSAGE_DELETE_BULK dispatch event
type MessageDeleteBulkData struct {
    IDs       []string `json:"ids"`
    ChannelID string   `json:"channel_id"`
    GuildID   string   `json:"guild_id"`
}

// GuildCreateData is the payload for the GUILD_CREATE dispatch event
type GuildCreateData struct {
    ID       string           `json:"id"`
    Channels []DiscordChannel `json:"channels"`
    Roles    []DiscordRole    `json:"roles"`
}

// DiscordChannel represents a channel entry in GUILD_CREATE
type DiscordChannel struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Type int    `json:"type"` // 0 = text channel
}

// DiscordRole represents a role entry in GUILD_CREATE
type DiscordRole struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

Also add `Mentions []DiscordUser` to existing `MessageCreateData`:
```go
type MessageCreateData struct {
    // ... existing fields ...
    Mentions []DiscordUser `json:"mentions"` // Add this field
}
```

### GuildCache interface (gateway/client.go or guild/cache.go)

```go
// GuildCache provides guild channel and role name lookups.
// Backed by Redis string keys distinct from the Phase 28 ChannelRegistry.
type GuildCache interface {
    SetChannelName(ctx context.Context, channelID, name string) error
    GetChannelName(ctx context.Context, channelID string) (string, bool, error)
    DeleteChannelName(ctx context.Context, channelID string) error
    SetRoleName(ctx context.Context, roleID, name string) error
    GetRoleName(ctx context.Context, roleID string) (string, bool, error)
    DeleteRoleName(ctx context.Context, roleID string) error
}
```

Redis key schema:
- `discord:guild:channels:{channel_id}` → plain string channel name
- `discord:guild:roles:{role_id}` → plain string role name

No TTL on these keys — they are kept current via lifecycle events and refreshed wholesale on reconnect via GUILD_CREATE.

### Deletion event publishing (HandleMessageDelete)

```go
// HandleMessageDelete processes a MESSAGE_DELETE dispatch event.
// Exported for direct unit testing.
func (c *GatewayClient) HandleMessageDelete(ctx context.Context, msg MessageDeleteData) error {
    overlayID, found, err := c.registry.GetOverlayForChannel(ctx, msg.ChannelID)
    if err != nil {
        c.log.Warn("Channel registry lookup failed for deletion",
            zap.String("channel_id", msg.ChannelID), zap.Error(err))
        return nil
    }
    if !found {
        c.log.Debug("Channel not configured, dropping deletion",
            zap.String("channel_id", msg.ChannelID))
        return nil
    }

    rawMsg := map[string]interface{}{
        "message_id": uuid.New().String(),
        "platform":   "discord",
        "overlay_id": overlayID,
        "channel_id": msg.ChannelID,
        "event_type": "message_deletion",
        "event_data": map[string]interface{}{
            "deletion_type": "single",
            "target_msg_id": msg.ID,
        },
        "timestamp": time.Now().UTC(),
    }
    return c.publisher.Publish(ctx, rawMsg)
}
```

### Mention resolution (gateway/mentions.go)

```go
var (
    reMentionUser    = regexp.MustCompile(`<@!?(\d+)>`)
    reMentionChannel = regexp.MustCompile(`<#(\d+)>`)
    reMentionRole    = regexp.MustCompile(`<@&(\d+)>`)
)

// ResolveMentions replaces Discord mention tokens with human-readable names.
// userMap is built from the MessageCreateData.Mentions array (ID → display name).
// guildCache is queried for channel and role names.
func ResolveMentions(
    content string,
    userMap map[string]string,
    cache GuildCache,
    ctx context.Context,
    log *zap.Logger,
) string {
    // 1. User mentions: <@ID> and <@!ID>
    content = reMentionUser.ReplaceAllStringFunc(content, func(match string) string {
        id := reMentionUser.FindStringSubmatch(match)[1]
        if name, ok := userMap[id]; ok {
            return "@" + name
        }
        if log != nil {
            log.Debug("Unresolvable user mention", zap.String("id", id))
        }
        return "@unknown"
    })

    // 2. Channel mentions: <#ID>
    content = reMentionChannel.ReplaceAllStringFunc(content, func(match string) string {
        id := reMentionChannel.FindStringSubmatch(match)[1]
        name, found, err := cache.GetChannelName(ctx, id)
        if err != nil || !found {
            if log != nil {
                log.Debug("Unresolvable channel mention", zap.String("id", id), zap.Error(err))
            }
            return "#channel"
        }
        return "#" + name
    })

    // 3. Role mentions: <@&ID>
    content = reMentionRole.ReplaceAllStringFunc(content, func(match string) string {
        id := reMentionRole.FindStringSubmatch(match)[1]
        name, found, err := cache.GetRoleName(ctx, id)
        if err != nil || !found {
            if log != nil {
                log.Debug("Unresolvable role mention", zap.String("id", id), zap.Error(err))
            }
            return "@unknown"
        }
        return "@" + name
    })

    return content
}
```

### GatewayClient struct update

```go
type GatewayClient struct {
    // ... existing fields ...
    guildCache GuildCache // new — may be nil if not needed (tests)
}

// NewGatewayClient signature extension
func NewGatewayClient(
    token, gatewayURL string,
    store SessionStore,
    log *zap.Logger,
    registry ChannelRegistry,
    pub MessagePublisher,
    guildCache GuildCache, // new parameter
) *GatewayClient {
```

This is a breaking change to the constructor — `cmd/main.go` and all tests that call `NewGatewayClient` must be updated.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-message REST call for mention resolution | Inline resolution from MESSAGE_CREATE `mentions` array | Discord API v10 (current) | Zero REST calls per message |
| Single `MESSAGE_DELETE` only | Both `MESSAGE_DELETE` and `MESSAGE_DELETE_BULK` | Discord API v6+ | Bulk deletions (moderation tools) would be missed without bulk handler |

**Discord API version in use:** v10 (configured in `DISCORD_GATEWAY_URL` default: `wss://gateway.discord.gg/?v=10&encoding=json`).

---

## Open Questions

1. **`NewGatewayClient` constructor signature change**
   - What we know: Adding `GuildCache` parameter breaks existing tests that call `NewGatewayClient` with 6 args
   - What's unclear: Whether to use variadic options pattern or simply add the parameter and update all call sites
   - Recommendation: Direct parameter addition — there are only two call sites (cmd/main.go and tests). Options pattern adds complexity not needed at v1.5 scale.

2. **`publisher.RawMessage` EventType/EventData fields**
   - What we know: The struct currently lacks these fields; deletion events need them to survive the JSON round-trip through `publisherAdapter`
   - What's unclear: Whether to add to `publisher.RawMessage` or switch deletion events to a separate publish path
   - Recommendation: Add `EventType string` and `EventData map[string]interface{}` to `publisher.RawMessage` — matches `models.RawChatMessage` shape and is the minimal change.

3. **GUILD_CREATE channel type filtering**
   - What we know: `DiscordChannel.Type = 0` is text; GUILD_CREATE may include voice (2), category (4), etc.
   - What's unclear: Whether to store only text channel names or all channel types in the cache
   - Recommendation: Store all channel types — the cache key is only accessed when a `<#ID>` mention appears, which can only reference channels visible to the bot. Filtering by type adds complexity with no practical benefit.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (github.com/stretchr/testify) |
| Config file | none — standard `go test ./...` |
| Quick run command | `cd services/discord-listener && go test ./gateway/... ./guild/... -v -count=1` |
| Full suite command | `cd services/discord-listener && go test ./... -v -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INBD-03 | MESSAGE_DELETE filtered channel drops silently | unit | `go test ./gateway/... -run TestHandleMessageDelete_UnknownChannel` | ❌ Wave 0 |
| INBD-03 | MESSAGE_DELETE configured channel publishes deletion event | unit | `go test ./gateway/... -run TestHandleMessageDelete_HappyPath` | ❌ Wave 0 |
| INBD-03 | MESSAGE_DELETE_BULK expands to N deletion events | unit | `go test ./gateway/... -run TestHandleMessageDeleteBulk_Expansion` | ❌ Wave 0 |
| INBD-03 | Deletion event shape: event_type=message_deletion, deletion_type=single, target_msg_id=snowflake | unit | `go test ./gateway/... -run TestHandleMessageDelete_EventShape` | ❌ Wave 0 |
| INBD-04 | User mention `<@ID>` resolves to `@name` | unit | `go test ./gateway/... -run TestResolveMentions_User` | ❌ Wave 0 |
| INBD-04 | User mention `<@!ID>` resolves identically to `<@ID>` | unit | `go test ./gateway/... -run TestResolveMentions_UserNickVariant` | ❌ Wave 0 |
| INBD-04 | Unresolvable user mention falls back to `@unknown` | unit | `go test ./gateway/... -run TestResolveMentions_UserFallback` | ❌ Wave 0 |
| INBD-04 | Channel mention `<#ID>` resolves to `#name` via GuildCache | unit | `go test ./gateway/... -run TestResolveMentions_Channel` | ❌ Wave 0 |
| INBD-04 | Role mention `<@&ID>` resolves to `@RoleName` via GuildCache | unit | `go test ./gateway/... -run TestResolveMentions_Role` | ❌ Wave 0 |
| INBD-04 | GUILD_CREATE handler populates channel + role caches | unit | `go test ./gateway/... -run TestHandleGuildCreate_CachesPopulated` | ❌ Wave 0 |
| INBD-04 | CHANNEL_DELETE clears cache entry | unit | `go test ./gateway/... -run TestHandleChannelDelete_CacheCleared` | ❌ Wave 0 |
| INBD-04 | GuildCache Redis implementation get/set/delete | unit | `go test ./guild/... -run TestRedisGuildCache` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/discord-listener && go test ./gateway/... -v -count=1`
- **Per wave merge:** `cd services/discord-listener && go test ./... -v -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/discord-listener/gateway/message_delete_test.go` — covers INBD-03 deletion events
- [ ] `services/discord-listener/gateway/mentions_test.go` — covers INBD-04 mention resolution
- [ ] `services/discord-listener/gateway/guild_create_test.go` — covers INBD-04 GUILD_CREATE handler
- [ ] `services/discord-listener/guild/cache_test.go` — covers GuildCache interface + Redis implementation

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/discord-listener/gateway/types.go` — existing struct definitions; confirmed `DiscordUser` fields (`global_name`, `username`)
- Direct code inspection: `services/discord-listener/gateway/client.go` — existing `GatewayClient`, interfaces, `HandleMessageCreate`, dispatch switch
- Direct code inspection: `services/twitch-listener/irc/parser.go` — canonical deletion event pattern (`ParseClearMessage`)
- Direct code inspection: `services/message-processor/normalizer/normalizer.go` — `NormalizeDeletion()` function; confirmed handles `single`/`batch`/`clear` with `target_msg_id` string
- Direct code inspection: `services/message-processor/registry/buffer.go` — `RedisDeletionBuffer` confirmed handles race condition
- Direct code inspection: `services/discord-listener/publisher/stream_publisher.go` — `RawMessage` struct; confirmed missing `EventType`/`EventData` fields
- Direct code inspection: `services/discord-listener/cmd/main.go` — wiring pattern for interfaces; confirmed `publisherAdapter` JSON round-trip
- Direct code inspection: `services/discord-listener/gateway/message_create_test.go` — existing test patterns; confirmed `mockChannelRegistry`, `capturePublisher` test helpers

### Secondary (MEDIUM confidence)
- CONTEXT.md decisions: Discord `MESSAGE_DELETE_BULK` has single top-level `channel_id` with `ids []string` array — consistent with Discord API v10 spec
- CONTEXT.md decisions: `GUILD_CREATE` delivers `channels` and `roles` arrays — consistent with Discord Gateway API docs

### Tertiary (LOW confidence)
- None — all critical claims are backed by direct code inspection or locked CONTEXT.md decisions.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already present in go.mod; no new dependencies
- Architecture: HIGH — patterns verified by direct inspection of Phase 27/28 code; no speculation
- Pitfalls: HIGH — identified by direct struct/interface analysis (publisher.RawMessage gap, key collision risk)

**Research date:** 2026-03-16
**Valid until:** 2026-05-16 (stable — Go stdlib + redis/go-redis/v9 APIs change rarely)
