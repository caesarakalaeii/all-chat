# Phase 28: Inbound Listener Core - Research

**Researched:** 2026-03-15
**Domain:** Discord Gateway MESSAGE_CREATE → Redis Streams → message-processor normalizer
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Channel filtering**
- Filter happens in discord-listener before publishing — never write non-configured channel messages to Redis Streams
- Source of truth: Redis cache populated by overlay-manager — stores `channel_id → {guild_id, overlay_id, source_id}` mapping
- Redis Pub/Sub invalidation from overlay-manager — when a Discord source is created/deleted, overlay-manager publishes an invalidation event; discord-listener reloads its in-memory set immediately
- On MESSAGE_CREATE with no matching configured channel: log at DEBUG level and drop — no WARN noise
- Same Redis channel registry provides the `overlay_id` for the `RawChatMessage` at publish time (one lookup per message)

**Discord user display in overlays**
- DisplayName: guild nickname (`member.nick`) with fallback to `author.username`
- Username (ID field): always `author.username` (the stable, unique identifier)
- User.Color: top role's color, but only if not `#000000` — `#000000` means no color assigned; leave `User.Color` empty in that case
- Badges: map specific role names to badges — if role name matches `moderator`, `admin`, `vip` (case-insensitive), add it as a badge. Skip all other roles.
- Bot filtering: filter in discord-listener before publishing — if `author.bot == true`, drop silently before Redis Streams

**Empty-content safety**
- If first MESSAGE_CREATE arrives with empty `content`: log ERROR and halt the service
- Detection is reactive only — triggered on first empty MESSAGE_CREATE, no synthetic startup probe

**Publisher architecture**
- Dedicated `publisher/` package in `services/discord-listener/` — mirrors twitch-listener/publisher/ and kick-listener/publisher/
- `Publisher` struct with `Publish(ctx, msg *RawChatMessage) error` method
- Duplicate the serialization pattern — no shared library; each service owns its publisher
- `RawChatMessage.OverlayID` set at publish time from Redis channel registry lookup
- `RawChatMessage.ChannelID` = Discord Snowflake channel ID (as string)
- `RawChatMessage.Platform` = `"discord"`

### Claude's Discretion

- Redis key schema for the channel registry (`discord:channels:{channel_id}` or similar)
- Exact Redis Pub/Sub channel name for source invalidation events
- RawChatMessage field mapping for Discord-specific fields (member roles in EventData or Tags)
- Whether `member.nick` is available on the MESSAGE_CREATE payload's `member` field or requires a separate API call
- Internal structure of the dispatcher in `gateway/client.go` — whether MESSAGE_CREATE dispatch is handled inline or via a registered handler interface

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INBD-01 | Discord channel messages appear in overlays as a first-class chat source | MESSAGE_CREATE dispatch → channel registry filter → publisher/package → `chat:raw` stream → message-processor routing |
| INBD-02 | Discord messages are normalized to the unified RawChatMessage schema via a discord normalizer in message-processor | `discord_normalizer.go` implementing `Normalizer` interface, registered in `normalizers` map in message-processor cmd/main.go |
</phase_requirements>

---

## Summary

Phase 28 adds two orthogonal pieces: (1) MESSAGE_CREATE dispatch handling in discord-listener that filters, enriches, and publishes to `chat:raw`; and (2) a `DiscordNormalizer` in message-processor that converts the resulting `RawChatMessage` to `UnifiedChatMessage`.

The discord-listener side requires three new components: a `MessageCreateData` struct in `gateway/types.go`, a `ChannelRegistry` interface (Redis-backed, matching the `SessionStore` pattern), and a `publisher/` package (identical structure to `kick-listener/publisher/redis.go`). The dispatcher in `gateway/client.go` adds a `MESSAGE_CREATE` branch inside `case OpDispatch:` — the exact insertion point is identified by the `// TODO(Phase 28): halt if first MESSAGE_CREATE has empty content` comment. overlay-manager must publish a Redis Pub/Sub invalidation event when Discord sources are created or deleted so discord-listener's in-memory channel set stays current.

The message-processor side is purely additive: one new file `normalizer/discord_normalizer.go` implementing the `Normalizer` interface, registered in `cmd/main.go`'s `normalizers` map under key `"discord"`. Discord messages carry no platform emotes so the emote enricher runs over an empty emotes list (same as TikTok). Role-based badge mapping and color extraction from Discord's role color (`#000000` → empty) are the Discord-specific logic.

**Primary recommendation:** Implement the three gateway/client.go sub-tasks (types, channel registry, dispatcher) and the kick-listener publisher clone first, then tackle the normalizer — these are independent and can be built in parallel plans.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis GET (channel registry), XADD (stream publish), Pub/Sub subscribe | Already in discord-listener go.mod |
| `go.uber.org/zap` | v1.27.1 | Structured logging | Project-wide standard |
| `encoding/json` | stdlib | Serialization of RawChatMessage, MessageCreateData | No extra dep |
| `github.com/stretchr/testify` | v1.11.1 | Unit test assertions | Already in both go.mods |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/uuid` | already transitive | Generate deterministic MessageID if needed | UUID generation for message IDs |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory channel set + Pub/Sub invalidation | Pure Redis GET per message | In-memory + invalidation = O(1) lookup with low Redis traffic; pure GET works too but adds ~0.1ms per message |
| Inline dispatcher | Handler interface registered by Init | Inline is simpler and matches how READY is handled today; handler interface adds unnecessary indirection |

**Installation:** No new dependencies required for discord-listener or message-processor.

---

## Architecture Patterns

### Recommended Project Structure
```
services/discord-listener/
├── cmd/main.go               # Wire up publisher + channel registry (existing)
├── gateway/
│   ├── client.go             # Add MESSAGE_CREATE branch in OpDispatch case
│   └── types.go              # Add MessageCreateData struct
└── publisher/
    └── stream_publisher.go   # New — mirrors kick-listener/publisher/redis.go

services/message-processor/
└── normalizer/
    └── discord_normalizer.go # New — mirrors tiktok_normalizer.go pattern

services/overlay-manager/
└── handlers/
    └── sources.go            # Extend: publish Redis Pub/Sub on Create/Delete
```

### Pattern 1: ChannelRegistry Interface (mirrors SessionStore)
**What:** An interface that wraps Redis GET for channel → overlay mapping. Enables unit testing without live Redis.
**When to use:** Injected into `GatewayClient` (or a separate `MessageDispatcher`) for all channel filtering decisions.
**Example:**
```go
// Source: gateway/client_test.go — MockRedis pattern already established
type ChannelRegistry interface {
    GetOverlayForChannel(ctx context.Context, channelID string) (overlayID string, found bool, err error)
    Subscribe(ctx context.Context, invalidationCh chan<- struct{}) error
}

// Redis key: "discord:channels:{channel_id}" → overlayID (string)
// Pub/Sub channel: "discord:channel:invalidation"
```

### Pattern 2: Publisher Package (mirrors kick-listener/publisher/redis.go)
**What:** A `StreamPublisher` struct wrapping `*redis.Client` with a `Publish(ctx, msg *RawChatMessage) error` method.
**When to use:** Called from the MESSAGE_CREATE handler after channel filter passes.
**Example:**
```go
// Source: services/kick-listener/publisher/redis.go (lines 34-80)
type StreamPublisher struct {
    redis  *redis.Client
    logger *zap.Logger
}

func (p *StreamPublisher) Publish(ctx context.Context, msg *RawMessage) error {
    data, _ := json.Marshal(msg)
    _, err = p.redis.XAdd(ctx, &redis.XAddArgs{
        Stream: "chat:raw",
        Values: map[string]interface{}{"data": string(data)},
    }).Result()
    return err
}
```

Key insight from kick-listener: only a `"data"` field is written to the stream (full JSON). The message-processor reads `msg.Values["data"]` and unmarshals it. The twitch-listener publisher also adds top-level index fields (`message_id`, `platform`, etc.) alongside `data` — either works since message-processor only reads `data`.

### Pattern 3: DiscordNormalizer (mirrors tiktok_normalizer.go)
**What:** `type DiscordNormalizer struct{}` with a `Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error)` method.
**When to use:** Registered in message-processor's `normalizers` map under key `"discord"`.
**Example:**
```go
// Source: services/message-processor/normalizer/tiktok_normalizer.go pattern
type DiscordNormalizer struct{}

func NewDiscordNormalizer() *DiscordNormalizer { return &DiscordNormalizer{} }

func (n *DiscordNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
    if raw.Platform != "discord" {
        return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
    }
    // Extract fields from raw.Tags or unmarshal raw.RawMessage
    // DisplayName = tags["member_nick"] or tags["author_username"]
    // Username    = tags["author_username"]
    // UserID      = tags["author_id"]
    // Color       = tags["role_color"] (empty if "#000000")
    // Badges      = derived from tags["roles"] (comma-separated, filter to moderator/admin/vip)
    return unified, nil
}
```

### Pattern 4: MessageCreateData Struct
**What:** A new type in `gateway/types.go` for the Discord `MESSAGE_CREATE` payload `d` field.
**When to use:** Unmarshalled from `GatewayPayload.D` when `*payload.T == "MESSAGE_CREATE"`.

The Discord `MESSAGE_CREATE` payload structure (confirmed against Discord Gateway docs):
```go
// Source: Discord Gateway API docs — MESSAGE_CREATE event
type MessageCreateData struct {
    ID        string          `json:"id"`         // Snowflake message ID
    ChannelID string          `json:"channel_id"` // Snowflake channel ID
    GuildID   string          `json:"guild_id"`   // Snowflake guild ID (may be absent in DMs)
    Content   string          `json:"content"`    // Message text (requires MESSAGE_CONTENT intent)
    Timestamp string          `json:"timestamp"`  // ISO8601
    Author    DiscordUser     `json:"author"`
    Member    *DiscordMember  `json:"member"`     // Present for guild messages; contains nick + roles
}

type DiscordUser struct {
    ID            string `json:"id"`
    Username      string `json:"username"`
    GlobalName    string `json:"global_name"` // New display name (Discord username update 2023)
    Bot           bool   `json:"bot"`
}

type DiscordMember struct {
    Nick  *string  `json:"nick"`  // Guild nickname (nil if not set)
    Roles []string `json:"roles"` // List of role IDs (Snowflakes)
}
```

**Critical finding:** `member.nick` IS present on the MESSAGE_CREATE payload for guild messages — no separate API call required. This is confirmed by Discord's Gateway intent model: when `IntentGuildMessages` (bit 9) is enabled, the `member` partial object is included in MESSAGE_CREATE events. However, `member.roles` contains only role **IDs** (Snowflakes), not role **names**. To map role IDs to names (`moderator`, `admin`, `vip`), the service would need either a roles cache or a REST API call — see Discretion area below.

### Pattern 5: overlay-manager Pub/Sub Invalidation
**What:** When `HandleAddSource` or `HandleDeleteSource` creates/deletes a `discord` platform source, publish a Redis Pub/Sub message on channel `"discord:channel:invalidation"`.
**When to use:** Immediately after `sourceRepo.Create()` or `sourceRepo.Delete()` succeeds for platform == "discord".
**Example:**
```go
// Source: existing overlay-manager redisClient wiring in cmd/main.go (line 96-101)
// After source create for platform == "discord":
redisClient.Publish(ctx, "discord:channel:invalidation", channelID)
// discord-listener subscribes and reloads its in-memory registry
```

### Anti-Patterns to Avoid
- **Calling Discord REST API per message for role names:** Each MESSAGE_CREATE triggers a `/guilds/{guild_id}/roles` call — O(n) REST calls under load. Use the Tags/RawMessage approach with role IDs and resolve to names at startup or via a separate cache if needed.
- **Storing guild roles in the normalizer:** The normalizer in message-processor has no access to Discord state. Role-name-to-badge mapping must happen in discord-listener at publish time (encode resolved badge names into Tags) OR accept that badges only work for roles the bot can resolve from a startup cache.
- **Publishing non-configured channels to Redis Streams:** The filter MUST occur in discord-listener. Never relax this to "let message-processor filter" — it adds unnecessary processing and violates the phase decision.
- **Panicking on empty content:** Log ERROR and return a fatal error that causes the service to exit gracefully (via context cancellation or `log.Fatal`), not a panic.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis Pub/Sub subscribe loop | Custom channel listener | `redisClient.Subscribe()` from go-redis/v9 | Handles reconnection, channel management |
| UUID generation for message IDs | Custom ID generation | `msg.ID` from Discord (Snowflake as string) | Discord message IDs are globally unique |
| JSON serialization | Custom marshaling | `encoding/json` + struct tags | Already used everywhere; kick-publisher pattern confirmed |
| Platform guard | Switch on platform | Early return `fmt.Errorf("unsupported platform: %s", raw.Platform)` | Established pattern in all normalizers |

**Key insight:** The `OverlayID` for the publisher comes from the channel registry Redis GET — do NOT derive it from the database in discord-listener. The message-processor's router (overlay_router.go) already handles DB-based routing, but the CONTEXT.md decision gives the channel registry the `overlay_id` at publish time, which bypasses the DB query entirely (line 207-210 in message-processor/cmd/main.go: `if rawMsg.OverlayID != "" { overlays = []models.OverlayTarget{{OverlayID: rawMsg.OverlayID}} }`).

---

## Common Pitfalls

### Pitfall 1: Role IDs vs Role Names in member.roles
**What goes wrong:** `member.roles` in MESSAGE_CREATE contains role Snowflake IDs (e.g., `"1234567890"`), not role names. Attempting to match against `"moderator"`, `"admin"`, `"vip"` against IDs always fails — no badges appear for any Discord user.
**Why it happens:** Discord separates role definition (guild roles endpoint) from role assignment (member.roles in messages).
**How to avoid:** Two acceptable approaches: (A) Fetch guild roles at bot startup via `GET /guilds/{guild_id}/roles`, cache them in memory as `roleID → roleName`, refresh on reconnect. (B) Accept no badge support for phase 28 scope and implement role caching in a follow-up. The CONTEXT.md decision says to map role names — so approach A is required. Cache must be populated before message handling begins (during READY event processing or after it).
**Warning signs:** All Discord users appear without badges in overlays even when they have moderator roles.

### Pitfall 2: member field may be nil
**What goes wrong:** Runtime nil pointer dereference when accessing `msg.Member.Nick` or `msg.Member.Roles` if `member` is absent from the payload.
**Why it happens:** `member` is only present for guild (server) messages, not DMs. While Discord sources are always guild channels, transient gateway edge cases may omit it.
**How to avoid:** `Member *DiscordMember` — pointer type with nil check before dereference. Fallback to `author.username` for display name if `Member == nil` or `Member.Nick == nil`.
**Warning signs:** Intermittent nil pointer panics in the gateway read loop.

### Pitfall 3: Empty content on first message (missing intent)
**What goes wrong:** Service receives MESSAGE_CREATE with `content: ""` because `MESSAGE_CONTENT` privileged intent is not enabled in Discord Developer Portal. Service silently processes empty messages that appear in overlays as blank entries.
**Why it happens:** The intent is required but must be manually enabled in the Discord Developer Portal. The Phase 27 WARN log reminds operators, but doesn't enforce.
**How to avoid:** The locked decision requires halting on first empty MESSAGE_CREATE. Implement: track `firstMessageSeen bool`; on first MESSAGE_CREATE, if `content == ""`, log ERROR and cancel the service context.
**Warning signs:** Messages appear in overlays with no text content.

### Pitfall 4: channel registry not populated before messages arrive
**What goes wrong:** Discord-listener starts, connects to Gateway, receives MESSAGE_CREATE events, but channel registry is empty (overlay-manager hasn't published any keys yet or they expired). All messages are dropped at DEBUG.
**Why it happens:** Service startup race — gateway connects before Redis is populated with channel → overlay mappings.
**How to avoid:** On startup, discord-listener should load all active Discord sources from Redis (keys matching `discord:channels:*`) OR query the DB directly. The Pub/Sub invalidation handles live updates but the initial state must be loaded from Redis keys populated by overlay-manager.
**Warning signs:** No Discord messages ever appear in overlays despite correct configuration.

### Pitfall 5: Pub/Sub invalidation fires before source is queryable
**What goes wrong:** overlay-manager publishes the invalidation event but discord-listener reloads the registry before the new key is written to Redis (or before it's committed to PostgreSQL).
**Why it happens:** Pub/Sub publish happens immediately; the Redis SET for the new channel key may happen slightly after or not at all if overlay-manager only publishes the invalidation without writing the key.
**How to avoid:** overlay-manager must write the `discord:channels:{channel_id}` Redis key (with `guild_id`, `overlay_id`, `source_id` as JSON) BEFORE publishing the invalidation Pub/Sub event. This ensures discord-listener can always do a GET after receiving the invalidation.
**Warning signs:** Intermittent "channel not found" drops immediately after adding a new Discord source.

---

## Code Examples

### MessageCreateData types addition in gateway/types.go
```go
// Source: gateway/types.go pattern (existing ReadyEventData at line 59)
// MessageCreateData is the d field of the MESSAGE_CREATE dispatch event
type MessageCreateData struct {
    ID        string         `json:"id"`
    ChannelID string         `json:"channel_id"`
    GuildID   string         `json:"guild_id"`
    Content   string         `json:"content"`
    Timestamp string         `json:"timestamp"`
    Author    DiscordUser    `json:"author"`
    Member    *DiscordMember `json:"member"`
}

type DiscordUser struct {
    ID         string `json:"id"`
    Username   string `json:"username"`
    GlobalName string `json:"global_name"`
    Bot        bool   `json:"bot"`
}

type DiscordMember struct {
    Nick  *string  `json:"nick"`
    Roles []string `json:"roles"`
}
```

### MESSAGE_CREATE dispatch in gateway/client.go
```go
// Source: gateway/client.go lines 132-152 — existing READY handling in OpDispatch
// Add after existing READY block, before the closing comment:
if payload.T != nil && *payload.T == "MESSAGE_CREATE" {
    var msg MessageCreateData
    if err := json.Unmarshal(payload.D, &msg); err != nil {
        c.log.Warn("Failed to parse MESSAGE_CREATE", zap.Error(err))
        continue
    }
    if err := c.handleMessageCreate(ctx, msg); err != nil {
        c.log.Error("MESSAGE_CREATE handler failed", zap.Error(err))
        // Halt on empty content (missing intent)
        return err
    }
}
```

### discord-listener publisher (mirrors kick-listener/publisher/redis.go)
```go
// Source: services/kick-listener/publisher/redis.go — exact pattern to mirror
type RawMessage struct {
    MessageID   string            `json:"message_id"`
    Platform    string            `json:"platform"`    // "discord"
    OverlayID   string            `json:"overlay_id"`
    ChannelID   string            `json:"channel_id"`  // Discord Snowflake
    ChannelName string            `json:"channel_name"`
    UserID      string            `json:"user_id"`     // author.id Snowflake
    Username    string            `json:"username"`    // author.username
    Text        string            `json:"text"`
    Tags        map[string]string `json:"tags,omitempty"`  // badges, color, member_nick
    RawMessage  json.RawMessage   `json:"raw_message,omitempty"`
    Timestamp   time.Time         `json:"timestamp"`
}
```

### DiscordNormalizer extracting from Tags
```go
// Source: normalizer/tiktok_normalizer.go extractUserInfo pattern (line 57-80)
// Tags populated by discord-listener publisher:
//   tags["author_id"]      → raw.UserID
//   tags["member_nick"]    → DisplayName (if non-empty, else author.username)
//   tags["role_color"]     → User.Color (empty if "#000000" or absent)
//   tags["badges"]         → comma-separated: "moderator", "admin", "vip" as applicable
func (n *DiscordNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
    if raw.Platform != "discord" {
        return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
    }
    displayName := raw.Tags["member_nick"]
    if displayName == "" {
        displayName = raw.Username
    }
    color := raw.Tags["role_color"] // "" if black/absent
    badges := extractDiscordBadges(raw.Tags["badges"])
    unified := &models.UnifiedChatMessage{
        ID:          raw.MessageID,
        OverlayID:   overlayID,
        Platform:    "discord",
        ChannelID:   raw.ChannelID,
        ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
        User: models.UserInfo{
            ID:          raw.UserID,
            Username:    raw.Username,
            DisplayName: displayName,
            Color:       color,
            Badges:      badges,
        },
        Message:   models.MessageInfo{Text: raw.Text, Emotes: []models.Emote{}},
        Timestamp: raw.Timestamp,
        Metadata:  map[string]interface{}{},
    }
    return unified, nil
}
```

### Registering DiscordNormalizer in message-processor/cmd/main.go
```go
// Source: message-processor/cmd/main.go lines 129-142 — existing normalizer map
discordNormalizer := normalizer.NewDiscordNormalizer()

normalizers := map[string]normalizer.Normalizer{
    "twitch":  twitchNormalizer,
    "youtube": youtubeNormalizer,
    "tiktok":  tiktokNormalizer,
    "kick":    kickNormalizer,
    "system":  systemNormalizer,
    "discord": discordNormalizer,  // Add this line
}
```

### overlay-manager Pub/Sub publish on source create/delete
```go
// Source: overlay-manager/handlers/sources.go HandleAddSource (line 350)
// After sourceRepo.Create() succeeds for platform == "discord":
if source.Platform == "discord" {
    // Write channel registry key BEFORE publishing invalidation
    regKey := fmt.Sprintf("discord:channels:%s", channelID)
    regVal, _ := json.Marshal(map[string]string{
        "overlay_id": overlayID,
        "source_id":  source.ID,
    })
    h.redisClient.Set(ctx, regKey, string(regVal), 0)
    h.redisClient.Publish(ctx, "discord:channel:invalidation", channelID)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-message DB query for overlay routing | OverlayID set at publisher time from Redis cache | Established in Phase 28 CONTEXT.md | Eliminates DB query per Discord message in message-processor |
| Shared library for publisher | Each listener owns its publisher package | Project ADR-0004 (no ports/adapters) | Clean service boundaries; each go.mod is independent |

**Deprecated/outdated:**
- Phase 27 comment `// TODO(Phase 28): halt if first MESSAGE_CREATE has empty content` — this is the exact insertion point for the empty-content halt logic; the TODO is replaced by real implementation.

---

## Open Questions

1. **Role name resolution for badges**
   - What we know: `member.roles` contains Snowflake IDs. The CONTEXT.md decision requires mapping role names `moderator`, `admin`, `vip`.
   - What's unclear: Where does the role ID → name mapping live? Does discord-listener need to call `GET /guilds/{guild_id}/roles` at startup?
   - Recommendation: Implement a lightweight `RoleCache` struct in discord-listener that fetches guild roles via Discord REST API once on READY event and caches them in memory. Role names are stable (rarely change). Refresh on reconnect. This is internal to discord-listener; no interface injection needed for this phase.

2. **overlay-manager redisClient availability in SourcesHandler**
   - What we know: `SourcesHandler` (handlers/sources.go) receives `*pgxpool.Pool` and `*zap.Logger` but NOT `*redis.Client`. The overlay-manager's main.go does create a `redisClient` but does not pass it to `SourcesHandler`.
   - What's unclear: Must `SourcesHandler` be refactored to accept `*redis.Client` for Pub/Sub publishing?
   - Recommendation: Yes — `NewSourcesHandler` signature must be extended to accept `*redis.Client` (or a minimal `ChannelPublisher` interface for testability). This is a required change in overlay-manager.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | testify v1.11.1 + go test |
| Config file | none (standard go test) |
| Quick run command | `cd services/discord-listener && go test ./gateway/... -v -run TestMessageCreate` |
| Full suite command | `cd services/discord-listener && go test ./... && cd ../message-processor && go test ./normalizer/... -run TestDiscord` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INBD-01 | Bot messages (author.bot==true) are filtered before publish | unit | `go test ./gateway/... -run TestHandleMessageCreate_BotFiltered` | Wave 0 |
| INBD-01 | Non-configured channel messages are dropped (no publish) | unit | `go test ./gateway/... -run TestHandleMessageCreate_UnknownChannel` | Wave 0 |
| INBD-01 | Empty content triggers service halt | unit | `go test ./gateway/... -run TestHandleMessageCreate_EmptyContent` | Wave 0 |
| INBD-01 | Configured channel message is published to Redis stream | unit | `go test ./publisher/... -run TestPublish_HappyPath` | Wave 0 |
| INBD-02 | DiscordNormalizer returns correct UnifiedChatMessage fields | unit | `go test ./normalizer/... -run TestDiscordNormalizer` | Wave 0 |
| INBD-02 | DiscordNormalizer rejects non-discord platform | unit | `go test ./normalizer/... -run TestDiscordNormalizer_WrongPlatform` | Wave 0 |
| INBD-02 | member.nick used as DisplayName when present | unit | `go test ./normalizer/... -run TestDiscordNormalizer_NickFallback` | Wave 0 |
| INBD-02 | Black role color (#000000) maps to empty User.Color | unit | `go test ./normalizer/... -run TestDiscordNormalizer_BlackColor` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./gateway/... ./publisher/...` (discord-listener) or `go test ./normalizer/... -run TestDiscord` (message-processor)
- **Per wave merge:** `go test ./...` in both `services/discord-listener` and `services/message-processor`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/discord-listener/gateway/message_create_test.go` — covers INBD-01 filtering + halt behavior
- [ ] `services/discord-listener/publisher/stream_publisher_test.go` — covers publish happy path
- [ ] `services/message-processor/normalizer/discord_normalizer_test.go` — covers INBD-02 normalization

*(All existing test infrastructure is in place; only new test files needed for new code.)*

---

## Sources

### Primary (HIGH confidence)
- Direct code reading of `services/discord-listener/gateway/client.go` — OpDispatch case structure, TODO insertion point confirmed
- Direct code reading of `services/discord-listener/gateway/types.go` — existing type patterns confirmed
- Direct code reading of `services/kick-listener/publisher/redis.go` — exact publisher pattern to mirror
- Direct code reading of `services/message-processor/normalizer/kick_normalizer.go` and `tiktok_normalizer.go` — normalizer pattern confirmed
- Direct code reading of `services/message-processor/cmd/main.go` — normalizers map registration pattern confirmed
- Direct code reading of `services/message-processor/router/overlay_router.go` — `rawMsg.OverlayID != ""` short-circuit path confirmed (line 207-210)
- Direct code reading of `services/overlay-manager/handlers/sources.go` — SourcesHandler constructor lacks redisClient (open question confirmed)

### Secondary (MEDIUM confidence)
- Discord Gateway API documentation (from training data, 2023-era): `MESSAGE_CREATE` payload includes `member` partial with `nick` and `roles` for guild messages when `GUILD_MESSAGES` intent is active. Confidence: HIGH — this is well-established Discord API behavior unchanged since 2021.
- Discord `member.roles` contains Snowflake IDs not names — MEDIUM (training data, widely documented, stable behavior)

### Tertiary (LOW confidence)
- Role cache via `GET /guilds/{guild_id}/roles` REST endpoint at startup — behavior inferred from Discord docs, not verified against live bot in this environment.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod, no new dependencies
- Architecture: HIGH — all patterns read directly from existing code
- Pitfalls: HIGH — role IDs vs names confirmed by Discord API design; nil member and empty content pitfalls confirmed by code inspection
- Open questions: MEDIUM — redisClient absence in SourcesHandler confirmed by code; role cache approach inferred from Discord API design

**Research date:** 2026-03-15
**Valid until:** 2026-04-15 (Discord Gateway API is stable; go-redis v9 API stable)
