# Phase 30: Outbound Relay - Research

**Researched:** 2026-03-16
**Domain:** Go service extension — Redis Pub/Sub subscription management, PostgreSQL LISTEN/NOTIFY, Discord REST API
**Confidence:** HIGH

## Summary

Phase 30 adds outbound relay capability to the existing `discord-listener` service. When a normalized `UnifiedChatMessage` is published to a Redis Pub/Sub channel (`overlay:{overlay_id}`), the relay reads it, filters out messages with `platform == "discord"` to prevent echo loops, formats the remaining messages as `[emoji] username: text`, and POSTs to a Discord channel via the Discord REST API.

All architectural decisions are locked by CONTEXT.md. The implementation is a pure addition — no existing files are significantly restructured. The reference implementation is `services/twitch-listener/channels/manager.go`, which provides the exact pattern (30s ticker + `LISTEN chat_source_changes` + dynamic add/remove) to port. The relay needs: a new `relay/` package inside `discord-listener`, a DB dependency (`pgx/v5`), and wiring in `cmd/main.go`.

The key complexity areas are (1) the relay config discovery loop (straight port from twitch-listener), (2) the dynamic Redis Pub/Sub subscribe/unsubscribe lifecycle, and (3) the Discord REST 429 handling with `Retry-After`.

**Primary recommendation:** Create `services/discord-listener/relay/` package with `Manager`, `Repository`, and `DiscordPoster` interface. Wire into `cmd/main.go` alongside the existing Gateway goroutine.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Platform emoji mapping**
- `[emoji] username: text` format with per-platform emoji:
  - 🟣 Twitch
  - 🔴 YouTube
  - 💚 Kick
  - 🎵 TikTok
  - 💬 fallback for unknown/future platform
- Emoji-to-platform mapping lives in discord-listener relay package (not message-processor)

**Relay config discovery**
- Add `github.com/jackc/pgx/v5` to discord-listener `go.mod` (not present yet)
- Startup + 30-second periodic sync from `overlay_chat_sources` WHERE platform='discord' AND relay_enabled=true
- PostgreSQL `LISTEN chat_source_changes` for instant notification → immediate re-sync
- Individual `SUBSCRIBE overlay:{overlay_id}` per overlay with a relay-enabled Discord source (no wildcard psubscribe)
- Dynamic subscribe/unsubscribe at runtime without restart

**Discord REST failure handling**
- Network error / 5xx: log ERROR, drop — no retry
- 429 Too Many Requests: honor `Retry-After` header, pause relay goroutine for that duration, retry once
- 403 / 404: log WARN, drop — no automatic DB update

**relay_enabled runtime refresh**
- Acceptable lag: up to 30 seconds (NOTIFY makes it near-instant)
- Toggle-OFF: unsubscribe immediately, discard buffered messages — no drain
- Toggle-ON: subscribe to `overlay:{overlay_id}`, relay on next message

### Claude's Discretion

- Redis Pub/Sub subscription management internals (goroutine-per-overlay vs. single goroutine with select)
- Exact PostgreSQL query to fetch relay-enabled Discord sources and their relay_channel_id
- Whether relay config is a struct or map derived from `config` JSONB
- Internal naming of the relay component (e.g. `relay.Manager`, `relay.Worker`)

### Deferred Ideas (OUT OF SCOPE)

- None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| RELY-01 | Overlay messages from non-Discord sources relayed to configured Discord channel; `platform == "discord"` unconditionally filtered | Loop-safety filter applied before relay action in `relay.Manager.handleMessage()`; validated by unit test asserting no REST call for platform=discord messages |
| RELY-02 | Each Discord source has `relay_enabled` toggle; inbound-only (read-only) mode supported | DB query filters WHERE `(config->>'relay_enabled')::boolean = true`; toggle-OFF unsubscribes immediately on sync |
| RELY-03 | Relay target channel (outbound) configurable per-source; same or different from inbound | `relay_channel_id` read from `config` JSONB; stored as Snowflake string; used as path param in REST POST |
| RELY-04 | Relayed messages posted as plain text `[emoji] username: text` | Platform emoji map in relay package; `UnifiedChatMessage.Platform` + `.User.Username` + `.Message.Text` fields |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5` | v5.x | PostgreSQL pool + LISTEN/NOTIFY | Already used by twitch-listener; same pattern being ported |
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis Pub/Sub subscribe/unsubscribe | Already in discord-listener go.mod |
| `go.uber.org/zap` | v1.27.1 | Structured logging | Already in discord-listener go.mod |
| `net/http` (stdlib) | Go stdlib | Discord REST POST | No extra dependency; used by existing handlers |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` (stdlib) | Go stdlib | Deserialize UnifiedChatMessage from Pub/Sub | Deserializing the JSON blob from Redis Pub/Sub message payload |
| `fmt` (stdlib) | Go stdlib | Format `[emoji] username: text` | Message formatting |

**Installation:**
```bash
# In services/discord-listener/
go get github.com/jackc/pgx/v5
```

---

## Architecture Patterns

### Recommended Project Structure

New package to add:
```
services/discord-listener/
├── cmd/main.go                  # Wire relay.Manager startup (add DB pool + relay goroutine group)
├── gateway/                     # EXISTING — inbound Gateway client
├── handlers/                    # EXISTING — health handlers
├── publisher/                   # EXISTING — Redis Stream publisher
└── relay/                       # NEW
    ├── manager.go               # relay.Manager — config sync loop + Redis Pub/Sub lifecycle
    ├── repository.go            # relay.Repository — pgx query for relay-enabled discord sources
    ├── poster.go                # relay.DiscordPoster interface + httpPoster implementation
    └── manager_test.go          # unit tests: loop-safety filter, format, toggle-OFF suppression
```

### Pattern 1: Config Discovery Loop (port from twitch-listener)

**What:** `relay.Manager` replicates the `channels.Manager` pattern: startup sync + 30s ticker + `LISTEN chat_source_changes` → immediate re-sync on notification.

**When to use:** Whenever relay-enabled Discord sources need to be discovered without restarting the service.

**Reference implementation:** `services/twitch-listener/channels/manager.go` — `Start()`, `syncLoop()`, `listenForChanges()`, `listenAndWait()`.

Key differences from the Twitch reference:
- No leadership/coordinator logic (not needed for relay at v1.5 scale)
- Discovery query: `overlay_chat_sources` WHERE `platform='discord'` AND `(config->>'relay_enabled')::bool = true`
- Produces a map of `overlay_id → relay_channel_id` instead of IRC channel names

```go
// Source: services/twitch-listener/channels/manager.go (ported pattern)
func (m *Manager) Start(ctx context.Context) error {
    if err := m.SyncRelayConfigs(ctx); err != nil {
        return err
    }
    m.wg.Add(1)
    go m.syncLoop(ctx)
    if m.dbConn != nil {
        m.wg.Add(1)
        go m.listenForChanges(ctx)
    }
    return nil
}

func (m *Manager) syncLoop(ctx context.Context) {
    defer m.wg.Done()
    for {
        select {
        case <-m.syncTicker.C:
            if err := m.SyncRelayConfigs(ctx); err != nil {
                m.logger.Error("relay config sync failed", zap.Error(err))
            }
        case <-m.stopChan:
            return
        case <-ctx.Done():
            return
        }
    }
}
```

### Pattern 2: Dynamic Redis Pub/Sub Subscribe/Unsubscribe

**What:** The relay.Manager maintains a `map[string]*redis.PubSub` keyed by `overlay_id`. On sync, it computes diff between desired and active subscriptions, subscribes/unsubscribes accordingly.

**When to use:** On every `SyncRelayConfigs()` call — triggered by ticker or pg NOTIFY.

```go
// Source: go-redis/v9 PubSub API — verified pattern
// Subscribe
sub := m.redisClient.Subscribe(ctx, "overlay:"+overlayID)
m.activeSubs[overlayID] = sub
go m.drainOverlay(ctx, overlayID, sub, relayChannelID)

// Unsubscribe on toggle-OFF
if sub, ok := m.activeSubs[overlayID]; ok {
    _ = sub.Close()
    delete(m.activeSubs, overlayID)
    // cancel the drainOverlay goroutine via the sub.Close() triggering channel close
}
```

**Goroutine model recommendation (Claude's Discretion):** One goroutine per active subscription (`drainOverlay`). Each goroutine reads from `sub.Channel()`, applies the platform filter, formats, and calls `DiscordPoster.Post()`. When `sub.Close()` is called, `sub.Channel()` is closed and the goroutine exits cleanly. This is simpler than a single-goroutine fan-in with dynamic channel registration.

### Pattern 3: Discord REST POST with 429 Handling

**What:** POST `https://discord.com/api/v10/channels/{channel_id}/messages` with `Authorization: Bot {token}` and JSON body `{"content": "[emoji] username: text"}`.

**When to use:** Each message that passes the platform filter and has a configured `relay_channel_id`.

```go
// Source: Discord API docs https://discord.com/developers/docs/resources/message#create-message
// Endpoint: POST /channels/{channel.id}/messages
func (p *httpPoster) Post(ctx context.Context, channelID, content string) error {
    body, _ := json.Marshal(map[string]string{"content": content})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        "https://discord.com/api/v10/channels/"+channelID+"/messages",
        bytes.NewReader(body))
    req.Header.Set("Authorization", "Bot "+p.token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return fmt.Errorf("discord REST error: %w", err)  // log ERROR, drop
    }
    defer resp.Body.Close()

    switch resp.StatusCode {
    case http.StatusOK, http.StatusCreated:
        return nil
    case http.StatusTooManyRequests:
        retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")) // seconds as float
        time.Sleep(retryAfter)
        return p.Post(ctx, channelID, content) // retry once
    case http.StatusForbidden, http.StatusNotFound:
        // log WARN, drop — operator fixes via UI
        return nil
    default:
        return fmt.Errorf("discord REST %d: drop", resp.StatusCode) // log ERROR, drop
    }
}
```

### Pattern 4: DiscordPoster Interface for Testability

**What:** Abstract the HTTP call behind an interface so unit tests inject a mock that records calls.

```go
// Source: established project pattern (SessionStore, ChannelRegistry, GuildCache)
type DiscordPoster interface {
    Post(ctx context.Context, channelID, content string) error
}
```

### Pattern 5: relay.Repository — pgx Query

**What:** Single query returns `(overlay_id, relay_channel_id)` pairs for all relay-enabled Discord sources on active overlays.

```go
// Source: derived from twitch-listener/channels/repository.go pattern
const queryRelayConfigs = `
    SELECT ocs.overlay_id,
           ocs.config->>'relay_channel_id' AS relay_channel_id
    FROM overlay_chat_sources ocs
    JOIN overlays o ON o.id = ocs.overlay_id
    WHERE ocs.platform = 'discord'
      AND (ocs.config->>'relay_enabled')::boolean = true
      AND ocs.config->>'relay_channel_id' IS NOT NULL
      AND o.is_active = true
`
```

### Anti-Patterns to Avoid

- **Wildcard psubscribe:** Do not use `PSubscribe("overlay:*")`. Individual subscriptions are required per CONTEXT.md decision.
- **Retry queue:** Do not buffer failed messages for retry (except the single 429 retry). Relay is explicitly best-effort.
- **DB write on 403/404:** Do not flip `relay_enabled=false` in the database on REST errors. Operator fixes via UI.
- **Platform filter as configurable:** The `platform == "discord"` filter must be unconditional — not a config flag, always applied first.
- **Circular import:** Do not import `message-processor/models` directly. Deserialize `UnifiedChatMessage` into a local struct within the relay package (same fields, local definition) OR re-export from a shared package. The existing project uses `map[string]interface{}` or JSON re-marshal to bridge cross-service types; follow the same pattern.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PostgreSQL LISTEN/NOTIFY connection | Custom pg TCP listener | `pgxpool.Pool.Acquire()` + `conn.Conn().WaitForNotification()` | Already proven in twitch-listener; handles reconnect, context cancellation |
| Redis Pub/Sub channel management | Custom subscribe bookkeeping | `go-redis/v9 PubSub` via `client.Subscribe()` | Handles reconnect, clean close via `sub.Close()`, channel fan-out |
| Discord rate limit tracking | Custom token bucket per channel | Read `Retry-After` response header directly | Discord communicates exact pause duration; per-channel buckets are premature at v1.5 scale |
| JSON deserialization of UnifiedChatMessage | Custom parser | `encoding/json.Unmarshal` into local struct | Standard library; message format is stable |

---

## Common Pitfalls

### Pitfall 1: pgx `WaitForNotification` blocks forever without context

**What goes wrong:** If the context is cancelled (SIGTERM) and no notification arrives, `WaitForNotification` will not unblock, causing the goroutine to leak.

**Why it happens:** `WaitForNotification` respects context cancellation, but only if the context passed is the same one used throughout. The twitch-listener `listenAndWait` pattern already handles this correctly via the `ctx` parameter — port it exactly.

**How to avoid:** Pass `ctx` to `WaitForNotification(ctx)`. The goroutine also selects on `stopChan` in the outer loop; when `stopChan` closes, the goroutine returns before re-entering `listenAndWait`.

**Warning signs:** Shutdown hanging past the 25s graceful timeout.

### Pitfall 2: Redis Pub/Sub `sub.Channel()` not drained after `sub.Close()`

**What goes wrong:** If the `drainOverlay` goroutine is reading from `sub.Channel()` and `sub.Close()` is called, the channel is closed. A range loop over a closed channel exits cleanly, but a `for { select { case msg := <-sub.Channel() } }` pattern will spin if not handled correctly.

**Why it happens:** `sub.Channel()` returns a `<-chan *redis.Message`. After `sub.Close()`, the channel is closed and will yield zero values.

**How to avoid:** Use `for msg := range sub.Channel()` — exits when channel closes. Or check `msg == nil` explicitly.

**Warning signs:** CPU spike on relay goroutine after toggling relay_enabled off.

### Pitfall 3: JSONB boolean extraction produces string "true"/"false" not bool

**What goes wrong:** `config->>'relay_enabled'` returns the text representation. Casting `::boolean` in SQL works correctly for `'true'` and `'false'` strings but fails silently if the field is absent (returns NULL).

**Why it happens:** JSONB `->>` operator always returns text. Absent keys return NULL, and NULL::boolean is NULL not false.

**How to avoid:** Use `(ocs.config->>'relay_enabled')::boolean = true` in the WHERE clause. Absent key yields NULL which fails the `= true` check — correct behavior (absent = not relay-enabled). Add `AND ocs.config->>'relay_channel_id' IS NOT NULL` to exclude misconfigured rows.

**Warning signs:** Sources with no `relay_enabled` key accidentally matching the query.

### Pitfall 4: Retry-After header value format

**What goes wrong:** Discord's `Retry-After` header returns a float (seconds, e.g. `"1.234"`). Parsing as integer truncates incorrectly; parsing as string without conversion causes a panic.

**Why it happens:** Discord API v10 returns `Retry-After` as a decimal float per their rate limit documentation.

**How to avoid:** Parse with `strconv.ParseFloat(header, 64)` and convert to `time.Duration` via multiplication by `time.Second`.

**Warning signs:** Early retry (underrun) triggering a second 429.

### Pitfall 5: Discord REST bot self-message causes infinite relay loop if filter missing

**What goes wrong:** When discord-listener POSTs a relay message to Discord, Discord delivers that message to the Gateway as a `MESSAGE_CREATE` event. If the loop-safety filter is absent or mis-placed, it re-relays that message.

**Why it happens:** The relay bot's own messages come through the Gateway like any other message. The `HandleMessageCreate` bot filter (`msg.Author.Bot == true`) would catch messages from bots but Discord's own webhooks/apps may not set `bot: true`.

**How to avoid:** Apply the platform filter in the relay's message handler — if `UnifiedChatMessage.Platform == "discord"` the message was sourced from Discord inbound and must be dropped unconditionally before any relay action. This is separate from (and in addition to) the `Author.Bot` filter in the Gateway.

**Warning signs:** Test asserting zero REST calls for platform=discord messages failing.

---

## Code Examples

### UnifiedChatMessage deserialization in relay package

```go
// Source: services/message-processor/models/message.go (field names confirmed)
// Local relay-package struct — avoids cross-service import
type relayMessage struct {
    Platform  string `json:"platform"`
    OverlayID string `json:"overlay_id"`
    User      struct {
        Username string `json:"username"`
    } `json:"user"`
    Message struct {
        Text string `json:"text"`
    } `json:"message"`
}
```

### Platform emoji mapping

```go
// Source: CONTEXT.md locked decision
var platformEmoji = map[string]string{
    "twitch":  "🟣",
    "youtube": "🔴",
    "kick":    "💚",
    "tiktok":  "🎵",
}

func formatRelayContent(platform, username, text string) string {
    emoji, ok := platformEmoji[platform]
    if !ok {
        emoji = "💬"
    }
    return fmt.Sprintf("%s %s: %s", emoji, username, text)
}
```

### Relay config struct

```go
// Source: CONTEXT.md — relay_channel_id is a Snowflake string (Phase 27 decision)
type relayConfig struct {
    OverlayID       string
    RelayChannelID  string // Discord Snowflake as string
}
```

### cmd/main.go wiring additions

```go
// Source: CONTEXT.md integration point + existing cmd/main.go structure
// After existing Redis + Gateway setup:

dbPool, err := pgxpool.New(ctx, buildDatabaseDSN())
if err != nil {
    log.Fatal("failed to connect to database", zap.Error(err))
}
defer dbPool.Close()

relayRepo := relay.NewRepository(dbPool)
poster := relay.NewHTTPPoster(botToken, http.DefaultClient)
relayMgr := relay.NewManager(relayRepo, poster, rdb, log)

go func() {
    if err := relayMgr.Start(ctx); err != nil && ctx.Err() == nil {
        log.Error("relay manager start failed", zap.Error(err))
    }
}()

// In shutdown:
relayMgr.Stop()
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-shard Discord bots use one global PubSub subscriber | Per-overlay individual Subscribe calls (no wildcard) | Phase 30 decision | More precise subscription scope; no filtering needed after receive |
| REST clients without Retry-After handling | Must honor Retry-After per Discord ToS | Discord API ToS requirement | Required for production bots |

---

## Open Questions

1. **Discord REST rate limit bucket values (from STATE.md research flag)**
   - What we know: Discord returns `X-RateLimit-Bucket` and `Retry-After` headers. Per-channel POST limit is 5 messages per 5 seconds (documented).
   - What's unclear: Actual live bucket values may differ from docs. STATE.md flags this as needing live-bot validation.
   - Recommendation: The single-retry-on-429 strategy in CONTEXT.md is sufficient for v1.5 relay volumes. No token bucket pre-configuration needed — reactive handling is correct. Validate after first integration test against live Discord.

2. **pgx/v5 version pinning in go.mod**
   - What we know: Twitch-listener uses pgx/v5 (confirmed in `channels/manager.go` import). Version is not visible in discord-listener go.mod.
   - What's unclear: Exact semver to pin.
   - Recommendation: Run `go get github.com/jackc/pgx/v5` which resolves to latest stable; match the twitch-listener version with `go list -m github.com/jackc/pgx/v5` in that module for consistency.

---

## Validation Architecture

> `workflow.nyquist_validation` key is absent from `.planning/config.json` — treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + testify v1.11.1 |
| Config file | none — `go test ./...` from service root |
| Quick run command | `cd services/discord-listener && go test ./relay/... -v` |
| Full suite command | `cd services/discord-listener && go test ./... -v` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RELY-01 | `platform == "discord"` message produces zero REST calls | unit | `go test ./relay/... -run TestRelayManager_DiscordPlatformFiltered` | ❌ Wave 0 |
| RELY-01 | Non-discord message produces one REST call | unit | `go test ./relay/... -run TestRelayManager_NonDiscordRelayed` | ❌ Wave 0 |
| RELY-02 | relay_enabled=false source is not subscribed | unit | `go test ./relay/... -run TestRepository_OnlyRelayEnabledReturned` | ❌ Wave 0 |
| RELY-03 | relay_channel_id from config used as POST path param | unit | `go test ./relay/... -run TestHTTPPoster_UsesCorrectChannelID` | ❌ Wave 0 |
| RELY-04 | Format produces `[emoji] username: text` | unit | `go test ./relay/... -run TestFormatRelayContent` | ❌ Wave 0 |
| RELY-04 | Unknown platform uses 💬 fallback | unit | `go test ./relay/... -run TestFormatRelayContent_UnknownPlatform` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/discord-listener && go test ./relay/... -v`
- **Per wave merge:** `cd services/discord-listener && go test ./... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/discord-listener/relay/manager_test.go` — covers RELY-01, RELY-02
- [ ] `services/discord-listener/relay/poster_test.go` — covers RELY-03, RELY-04
- [ ] `services/discord-listener/relay/repository.go` + `relay/manager.go` + `relay/poster.go` — implementation files (created in Wave 1)

---

## Sources

### Primary (HIGH confidence)
- `services/twitch-listener/channels/manager.go` — full reference implementation for discovery loop
- `services/twitch-listener/channels/repository.go` — pgx query patterns for `overlay_chat_sources`
- `services/discord-listener/cmd/main.go` — existing wiring pattern; identifies where relay.Manager plugs in
- `services/discord-listener/gateway/client.go` — confirms interface injection pattern (SessionStore, ChannelRegistry, GuildCache, MessagePublisher) to replicate for DiscordPoster
- `services/message-processor/models/message.go` — confirmed field names: `Platform`, `User.Username`, `Message.Text`, `OverlayID`
- `services/message-processor/publisher/pubsub_publisher.go` — confirmed Pub/Sub channel format: `overlay:{overlay_id}`
- `migrations/001_initial_schema.sql` — confirmed `config JSONB` on `overlay_chat_sources`
- `migrations/004_source_change_notifications.sql` — confirmed `chat_source_changes` pg notify channel exists
- `migrations/035_discord_guilds.sql` — confirmed discord platform registered; no relay-specific migration needed (CONTEXT.md: no new tables)
- `services/discord-listener/go.mod` — confirmed `pgx/v5` NOT present, must be added

### Secondary (MEDIUM confidence)
- Discord API documentation (https://discord.com/developers/docs/resources/message#create-message) — POST endpoint, `Retry-After` header behavior, rate limit response format; verified against CONTEXT.md decisions

### Tertiary (LOW confidence)
- Specific `Retry-After` float format assumption — CONTEXT.md notes this needs live-bot validation before finalizing

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries confirmed in existing service go.mod files or direct go.mod inspection
- Architecture: HIGH — pattern is a direct port of existing twitch-listener manager with confirmed reference file
- Pitfalls: HIGH — derived from reading the actual reference code and go-redis/v9 pub/sub behavior; pgx WaitForNotification context behavior is confirmed by the existing listenAndWait implementation

**Research date:** 2026-03-16
**Valid until:** 2026-04-15 (Discord API v10 stable; go-redis/v9 stable)
