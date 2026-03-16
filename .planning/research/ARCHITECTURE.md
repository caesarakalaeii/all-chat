# Architecture Patterns: Discord Listener + Relay

**Domain:** Bidirectional Discord integration for All-Chat microservices platform
**Researched:** 2026-03-15
**Confidence:** HIGH (inbound listener, sharding model, loop prevention) / MEDIUM (relay rate-limit specifics)

---

## Recommendation: One Service, Two Goroutine Groups

**Use a single `discord-listener` service** that handles both inbound (Gateway WebSocket → Redis Streams) and outbound relay (Redis Pub/Sub → Discord REST). Do not create a separate relay service.

**Rationale:**
- Both directions share the same Discord bot token and REST client. Splitting means two services managing the same credential lifecycle.
- Both directions need the same guild/channel membership knowledge. Sharing in-process eliminates a cross-service lookup on every relay message.
- Loop prevention is simplest with shared state: the in-process source registry can tag relay-exclusion channel IDs without a round-trip.
- Precedent in codebase: Kick listener handles multiple concerns (subscribe, message, deletion) in one service. No existing service is split purely on direction.
- The relay workload is modest (one REST POST per non-Discord message per configured relay target), not an independent scaling axis.

**When to reconsider splitting:** If the platform grows to thousands of guilds, relay throughput independently saturates Discord REST rate limits, or separate on-call ownership is needed. At v1.5 scale, that is not the case.

---

## Recommended Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     discord-listener (new service)                       │
│                                                                          │
│  ┌──────────────────────┐    ┌────────────────────────────────────────┐  │
│  │  Inbound Goroutine   │    │  Relay Goroutine Group                 │  │
│  │  Group               │    │                                        │  │
│  │                      │    │  • Subscribes to overlay:{id} Pub/Sub  │  │
│  │  • Discord Gateway   │    │  • Filters: skip platform=="discord"   │  │
│  │    WebSocket (shard) │    │    AND source_channel_id in           │  │
│  │  • Receives MESSAGE_ │    │    configured_inbound_channels         │  │
│  │    CREATE events     │    │  • POSTs to Discord REST API           │  │
│  │  • Ignores bot's own │    │    POST /channels/{id}/messages        │  │
│  │    messages (author  │    │  • Respects per-channel rate limits    │  │
│  │    .bot == true)     │    │  • Queues with token bucket per        │  │
│  │  • Publishes to      │    │    channel_id (5 msg/s)                │  │
│  │    Redis Streams     │    │                                        │  │
│  │    chat:raw          │    └────────────────────────────────────────┘  │
│  └──────────────────────┘                                                │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │  Channel Registry (in-memory + Redis)                            │    │
│  │  • guild_id → shard_id mapping                                   │    │
│  │  • source registry: channel_id → []overlay_id                   │    │
│  │  • relay registry: overlay_id → outbound_channel_id             │    │
│  │  • inbound_channel_ids set (for loop-prevention lookup)         │    │
│  └──────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │  Shard Manager                                                   │    │
│  │  • Owns N Gateway WebSocket connections (one per shard)         │    │
│  │  • Shard ID = guild_id % num_shards                             │    │
│  │  • Only one pod per shard (source-manager leader election)      │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────┘
        │ Inbound                              │ Relay
        ▼                                      │
Redis Streams (chat:raw)            Redis Pub/Sub (overlay:{id})
        │                                      ▲
        ▼                                      │
Message Processor                   Message Processor
(adds platform="discord")           (publishes after normalization)
```

---

## Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| **discord-listener** (NEW) | Gateway WebSocket shards, inbound publish, relay subscribe + POST | Redis Streams (write), Redis Pub/Sub (subscribe), Discord Gateway WS, Discord REST API, source-manager (leadership), PostgreSQL (channel registry), auth-service (bot token storage) |
| **source-manager** (EXTEND) | Leader election for shard ownership | discord-listener replicas |
| **auth-service** (EXTEND) | Store Discord bot token (OAuth2 "Add to Server" flow) | PostgreSQL (oauth_tokens), discord-listener |
| **overlay-manager** (EXTEND) | CRUD for discord sources and relay config | PostgreSQL, discord-listener (via NOTIFY) |
| **message-processor** (EXTEND) | Add `discord` normalizer, parse MESSAGE_CREATE payload | Redis Streams (read), Redis Pub/Sub (write) |

---

## Data Flow: Inbound (Discord → Overlay)

```
1. User adds Discord channel as source to overlay
   overlay-manager inserts:
   platform="discord", channel_id="{discord_channel_id}",
   config={"guild_id":"...", "inbound_channel_id":"..."}

2. source-manager detects new discord source
   → NOTIFY discord_source_changes

3. discord-listener receives NOTIFY
   → Channel manager adds channel to in-memory registry
   → Shard manager ensures shard for guild_id is active
   → Subscribes to MESSAGE_CREATE events on that channel

4. Discord user posts message
   → Gateway sends MESSAGE_CREATE on shard for guild_id % num_shards
   → discord-listener receives event
   → Tags message with source_channel_id

5. discord-listener publishes to Redis Streams chat:raw:
   {
     "message_id":   "<discord_message_snowflake>",
     "platform":     "discord",
     "overlay_id":   "<overlay_uuid>",
     "channel_id":   "<discord_channel_id>",
     "channel_name": "<#channel-name>",
     "user_id":      "<discord_user_snowflake>",
     "username":     "<username#discriminator>",
     "text":         "<message content>",
     "timestamp":    "<ISO8601>",
     "tags": {
       "guild_id":          "<discord_guild_id>",
       "guild_name":        "<server name>",
       "inbound_channel_id": "<discord_channel_id>",
       "is_bot":            "false"
     },
     "raw_message":  <full MESSAGE_CREATE JSON>
   }

6. message-processor normalizes → publishes to overlay:{overlay_id}

7. API Gateway WebSocket → overlay client receives message
```

---

## Data Flow: Outbound Relay (Overlay → Discord)

```
1. User configures relay: overlay X → discord channel Y
   overlay-manager inserts config:
   config={"relay_channel_id": "<discord_channel_id>", "relay_enabled": true}

2. discord-listener subscribes to overlay:{overlay_id} on Redis Pub/Sub

3. Message arrives on overlay:{overlay_id}
   discord-listener receives UnifiedMessage

4. Loop prevention check (in-process, O(1)):
   IF message.platform == "discord"
      AND message.tags["inbound_channel_id"] IN inbound_channel_ids
   THEN DROP (this message originated from Discord, would echo back)

5. Format relay message:
   "[platform-emoji] username: text"
   e.g. "📺 xQcOW: Pepega OMEGALUL"

6. Enqueue in per-channel token bucket (5 msg/s per Discord channel)

7. POST /channels/{relay_channel_id}/messages
   Authorization: Bot {bot_token}
   {"content": "<formatted message>"}

8. On 429 Too Many Requests: parse Retry-After header, requeue after delay
```

---

## Discord Gateway Sharding Model

**Confidence: MEDIUM** (based on training data; verify against https://discord.com/developers/docs/topics/gateway#sharding before implementation)

### How Discord sharding works

Discord requires sharding when a bot is in 2,500+ guilds (servers). Each shard is a separate Gateway WebSocket connection. A guild's events are always delivered to the shard with ID:

```
shard_id = guild_id % num_shards
```

The bot sends an `IDENTIFY` payload with `[shard_id, num_shards]` when opening each Gateway connection.

### Sharding vs. existing hash-based load balancing

The existing system uses CRC32 consistent hashing to distribute channels across listener pods. Discord's sharding is fundamentally different: Discord determines which shard receives which guild's events. The service cannot freely redistribute guilds across shards — the shard assignment is imposed by Discord protocol.

**Key implication:** For discord-listener, load balancing across pods means assigning shard ownership, not individual channel ownership. One pod owns one or more shards. All guilds on that shard are handled by that pod.

### At v1.5 scale (small bot, few guilds)

With fewer than 2,500 guilds (likely at launch: single-digit to hundreds), sharding is not required. A single Gateway connection (shard 0 of 1) handles all guilds. The service should:

1. Start with `num_shards=1` (no sharding)
2. The existing source-manager leader election ensures only one pod holds the single Gateway connection
3. When the bot grows, increase `num_shards` and assign shard ranges to pods

### Leader election approach for shards

Reuse source-manager's leadership API with a shard-scoped key:

```
Redis key: leader:discord:shard:{shard_id}
Value:     "discord-listener-pod-abc123"
TTL:       60s (renewed every 30s)
```

One discord-listener pod claims leadership for shard 0 (or shards 0-N if multi-shard). Other pods in standby. This reuses the exact same pattern as youtube-listener-innertube.

### Coordinator-based shard assignment (future, multi-shard)

When `num_shards > 1`, extend the coordinator to distribute shard ranges:

```
Pod 1: shards [0, 1, 2]    → handles guilds where guild_id % 6 ∈ {0,1,2}
Pod 2: shards [3, 4, 5]    → handles guilds where guild_id % 6 ∈ {3,4,5}
```

The bounded-load consistent hashing used for Twitch/Kick/TikTok is not applicable here. Use simple range partitioning for shard assignment.

---

## Loop Prevention

**This is the most critical correctness requirement.**

### Where loop prevention lives

Loop prevention is enforced in the **discord-listener relay goroutine** before any HTTP call is made. It is NOT implemented in message-processor (which doesn't know about relay targets).

### Detection algorithm

A message is a potential echo if ALL of the following are true:
1. `message.platform == "discord"` (came from Discord originally)
2. `message.tags["inbound_channel_id"]` is in the set of Discord channels configured as inbound sources for the same overlay

```go
// In-process check, O(1) hash lookup
func (r *RelayWorker) isEcho(msg *models.UnifiedMessage, relayConfig RelayConfig) bool {
    if msg.Platform != "discord" {
        return false
    }
    inboundChannelID, ok := msg.Tags["inbound_channel_id"]
    if !ok {
        return false
    }
    // Check if the message's source channel is an inbound channel for this overlay
    return r.channelRegistry.IsInboundChannel(relayConfig.OverlayID, inboundChannelID)
}
```

### Why not filter in message-processor

Message-processor does not know which overlays have relay configured, nor which Discord channels are inbound vs outbound. Adding relay awareness to message-processor would couple two concerns that should stay separate. The discord-listener already subscribes to the Pub/Sub channel and can filter before calling Discord REST.

### Scenario table

| Message origin | Relay target | Action |
|----------------|-------------|--------|
| Twitch | Discord channel Y | Relay: post to Discord |
| YouTube | Discord channel Y | Relay: post to Discord |
| Discord channel X (same overlay, inbound source) | Discord channel Y | DROP (echo) |
| Discord channel X (different overlay, no inbound source match) | Discord channel Y | Relay: post to Discord |
| Discord channel Y (the relay target itself) | Discord channel Y | DROP (bot's own message filtered by `author.bot == true` at inbound) |

### Bot self-message filtering

The discord-listener inbound goroutine must always drop `MESSAGE_CREATE` events where `author.bot == true` AND `author.id == bot_application_id`. This prevents the relay's outbound messages from being re-ingested as chat messages.

---

## Integration with Existing Services

### source-manager

Discord channels are registered as sources with:
- `platform = "discord"`
- `channel_id = "{discord_channel_snowflake}"`
- `config = {"guild_id": "...", "inbound_channel_id": "...", "relay_channel_id": "...", "relay_enabled": true}`

Source-manager's existing `/sources?platform=discord` query works without modification. Leadership API is reused with `stream_id = "discord:shard:{shard_id}"`.

Source-manager needs one change: add `"discord"` to the list of known platforms so it includes Discord sources in the active source registry sync.

### auth-service

Discord uses OAuth2 with the "Add to Server" flow (not user-token OAuth). The bot token is a long-lived application token, not a per-user OAuth token. Storage options:

**Option A (Recommended):** Store the bot token in Kubernetes Secret, inject as environment variable `DISCORD_BOT_TOKEN`. The auth-service handles the OAuth2 "Add to Server" web flow (guild authorization for the streamer), but the bot token is global — not per-user.

**Option B:** Store in `oauth_tokens` table with `platform="discord"` and a synthetic `user_id`. More complex, no benefit at v1.5.

**Guild authorization (streamer flow):** When a streamer connects their Discord server, they complete a web OAuth2 flow (not bot token exchange) that grants the bot permission to join their guild. This results in the bot being added to the guild. The auth-service handles the callback and stores the `guild_id` in the user's profile or overlay config — NOT a per-user token.

The auth-service needs a new OAuth endpoint:
```
GET /api/v1/auth/discord/authorize   → redirects to Discord OAuth2 "Add to Server" URL
GET /api/v1/auth/discord/callback    → receives code, exchanges for guild_id confirmation, stores guild membership
```

### overlay-manager

Add `discord` as a supported platform. The `overlay_chat_sources` table already supports arbitrary `platform` values and `config` JSONB. No schema changes to the table itself.

New validation on source create/update: if `platform="discord"`, verify `guild_id` in config matches a guild the bot has joined (can query Discord REST `GET /guilds/{guild_id}` or rely on startup registry).

### message-processor

Add a Discord normalizer (`normalizer/discord_normalizer.go`). Input is the `raw_message` field from the `chat:raw` stream entry. The normalizer extracts:
- `user.id` ← `author.id`
- `user.username` ← `author.username` (+ optional `#discriminator` for legacy accounts)
- `user.display_name` ← `member.nick` if present, else `author.global_name`, else `author.username`
- `user.avatar_url` ← constructed from `author.avatar` hash
- `message.text` ← `content` (with mention resolution if desired)
- `badges` ← roles mapped to badge names (optional for v1.5)

Discord does not use emote codes in the 7TV/BTTV/FFZ sense. The emote enrichment stage can be skipped (pass-through) for `platform="discord"`.

---

## New Components

### New: discord-listener service

```
services/discord-listener/
├── cmd/main.go                    # Entry: DB, Redis, Gateway, Relay, HTTP
├── gateway/
│   ├── client.go                  # Discord Gateway WebSocket client
│   ├── shard.go                   # Shard lifecycle (connect, identify, heartbeat, resume)
│   ├── types.go                   # Gateway event structs (MESSAGE_CREATE, READY, etc.)
│   └── handler.go                 # Routes gateway events to message handler
├── channels/
│   ├── manager.go                 # Syncs active Discord sources from DB
│   ├── registry.go                # In-memory channel→overlay mapping + inbound set
│   └── repository.go              # DB queries for discord sources
├── relay/
│   ├── worker.go                  # Subscribes to Pub/Sub, filters, queues relay messages
│   ├── poster.go                  # Discord REST POST /channels/{id}/messages
│   └── ratelimit.go               # Token bucket per channel_id, 429 backoff
├── publisher/
│   └── redis.go                   # Publishes RawChatMessage to chat:raw
├── handlers/
│   └── health.go                  # /health/live, /health/ready, /status
├── metrics/
│   └── metrics.go                 # Prometheus counters/histograms
├── go.mod
└── Dockerfile
```

### Modified: source-manager

- Add `"discord"` to `SUPPORTED_PLATFORMS` list
- Add shard leadership key namespace: `leader:discord:shard:{shard_id}`
- No API surface changes

### Modified: auth-service

- Add `GET /api/v1/auth/discord/authorize` (redirects to Discord OAuth2 guild authorization URL)
- Add `GET /api/v1/auth/discord/callback` (stores guild_id, confirms bot membership)
- Add `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET`, `DISCORD_BOT_TOKEN` env vars

### Modified: message-processor

- Add `normalizer/discord_normalizer.go`
- Add case `"discord"` in router switch
- Skip emote enrichment stage for `platform="discord"` (Discord embeds are not third-party emotes)

### Modified: overlay-manager

- Add `"discord"` to supported platforms enum/validation
- Add validation: if `platform="discord"`, verify `guild_id` in config is known to bot

---

## Patterns to Follow

### Pattern 1: Gateway Client Mirrors Kick Pusher Client

The Kick listener's `websocket/client.go` is the closest structural analog: it manages a persistent WebSocket, handles reconnects, parses typed events, and calls registered handlers. Discord Gateway is more complex (heartbeat interval from HELLO, sequence numbers for RESUME, zlib decompression optionally), but the same read/write pump goroutine pattern applies.

**Key differences from Kick:**

| Aspect | Kick (Pusher) | Discord Gateway |
|--------|---------------|-----------------|
| Reconnect | Reconnect and re-subscribe | Reconnect → RESUME if sequence < threshold; else IDENTIFY |
| Heartbeat | Pusher application-level ping/pong | Send `{"op":1,"d":sequence}` every `heartbeat_interval` ms |
| Multiple guilds | Multiple Pusher channels per connection | All guilds on the shard on one connection |
| Auth | App key in URL | `IDENTIFY` payload with bot token |

### Pattern 2: Relay Worker as Pub/Sub Consumer

```go
// relay/worker.go
func (w *RelayWorker) Start(ctx context.Context) {
    // Subscribe to all configured overlay channels
    for _, overlayID := range w.registry.GetRelayOverlayIDs() {
        w.client.Subscribe("overlay:" + overlayID)
    }

    ch := w.client.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-ch:
            w.handleMessage(msg)
        }
    }
}

func (w *RelayWorker) handleMessage(msg *redis.Message) {
    var unified models.UnifiedMessage
    json.Unmarshal([]byte(msg.Payload), &unified)

    config, ok := w.registry.GetRelayConfig(unified.OverlayID)
    if !ok || !config.RelayEnabled {
        return
    }

    if w.isEcho(&unified, config) {
        return // loop prevention
    }

    w.queue.Enqueue(config.RelayChannelID, &unified)
}
```

### Pattern 3: Token Bucket Rate Limiting for Relay

Discord REST allows ~5 messages/second per channel (confirmed in documentation; MEDIUM confidence — verify exact limits). Use a per-channel token bucket:

```go
// relay/ratelimit.go
type ChannelRateLimiter struct {
    buckets map[string]*rate.Limiter  // channel_id → limiter
    mu      sync.Mutex
}

func (l *ChannelRateLimiter) Wait(ctx context.Context, channelID string) error {
    l.mu.Lock()
    limiter, ok := l.buckets[channelID]
    if !ok {
        limiter = rate.NewLimiter(rate.Limit(5), 5) // 5/s, burst 5
        l.buckets[channelID] = limiter
    }
    l.mu.Unlock()
    return limiter.Wait(ctx)
}
```

Also handle Discord 429 responses:
```go
if resp.StatusCode == 429 {
    var rateLimit DiscordRateLimit
    json.NewDecoder(resp.Body).Decode(&rateLimit)
    time.Sleep(time.Duration(rateLimit.RetryAfter * float64(time.Second)))
    // retry
}
```

### Pattern 4: Source Registry with NOTIFY-Driven Updates

Mirror the Twitch/Kick channel manager pattern: query DB on startup for all active Discord sources, then receive `NOTIFY discord_source_changes` from overlay-manager to add/remove channels without full resync.

```go
// channels/manager.go — same structure as kick-listener/channels/manager.go
// channels/repository.go — query WHERE platform='discord' AND is_active=true
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Separate Relay Service

**What:** Create `discord-relay` as a separate Kubernetes Deployment
**Why bad:**
- Both inbound listener and relay need the same Discord bot token and channel registry
- Loop prevention requires shared knowledge of which channels are inbound sources
- Two services to deploy, monitor, and maintain for one logical feature
- No independent scaling benefit at current scale (relay is rate-limit-bound, not CPU-bound)

**Instead:** Single `discord-listener` service with two goroutine groups

### Anti-Pattern 2: Message-Processor-Level Loop Prevention

**What:** Adding a "drop if platform==discord AND is relay echo" filter in message-processor

**Why bad:**
- Message-processor does not know which overlays have relay configured
- Requires message-processor to query relay config on every Discord message (hot path)
- Couples the relay feature into the general message processing pipeline
- Loop prevention should be at the relay boundary, not the normalization boundary

**Instead:** discord-listener relay worker filters before calling Discord REST

### Anti-Pattern 3: Subscribing Relay to chat:raw Stream Instead of Pub/Sub

**What:** Relay worker consumes from `chat:raw` Redis Stream to pick up messages to relay

**Why bad:**
- `chat:raw` contains unnormalized, unenriched messages — the relay would need its own normalization
- Adds a consumer group to `chat:raw`, increasing stream fan-out
- Messages in `chat:raw` are not yet filtered by `MESSAGE_AGE_CUTOFF_SECONDS`
- The relay should send the same human-readable text that appears in the overlay, which is the normalized form

**Instead:** Subscribe to `overlay:{overlay_id}` Pub/Sub (post-normalization)

### Anti-Pattern 4: Hash-Based Sharding Applied to Discord Channels

**What:** Using CRC32 consistent hashing to distribute individual Discord channels across pods

**Why bad:**
- Discord Gateway sharding is guild-based (not channel-based) and imposed by Discord protocol
- A guild's events all arrive on `shard_id = guild_id % num_shards` — you cannot route individual channels from the same guild to different Gateway connections
- Applying all-chat's channel-level consistent hashing would require opening one Gateway connection per channel, violating Discord's connection model (one connection per shard, which covers all guilds on that shard)

**Instead:** Assign shard ownership to pods. One pod owns one shard (or a range of shards).

### Anti-Pattern 5: Re-using the Existing Node.js discord-bot Service

**What:** Extending `services/discord-bot` (the YouTube quota monitoring Node.js bot) to also handle chat listening/relay

**Why bad:**
- That service is Node.js, not Go — inconsistent with all other services
- It is a monitoring-only bot with no message ingestion or Redis Streams publishing
- Mixing quota monitoring and chat routing concerns increases blast radius
- The all-chat platform constraint: "No new infrastructure dependencies" — adding Node.js as a pattern for core services violates the Go-only backend decision

**Instead:** New `services/discord-listener` in Go, following Standard Go Layout

---

## Scalability Considerations

| Concern | At 100 guilds | At 10K guilds | At 1M guilds |
|---------|---------------|---------------|--------------|
| **Gateway connections** | 1 shard, 1 pod | 4 shards (Discord recommends 2,500/shard), 4 pods | 400 shards, 400 pods |
| **Relay throughput** | Single REST client, token bucket sufficient | Relay worker pool per overlay, per-channel queue | Dedicated relay service, Redis queue with multiple workers |
| **Channel registry** | In-memory map, trivial | In-memory + Redis backup | Redis Cluster, region-local replica |
| **Loop prevention** | In-process set lookup | In-process set (thousands of entries, still trivial) | Bloom filter or Redis SET |
| **Pub/Sub subscriptions** | One subscription per relay overlay | Standard Redis Pub/Sub handles thousands | Redis Cluster with Pub/Sub sharding |

**At v1.5 scale:** In-process everything is correct. Redis-backed state as fallback for pod restarts.

---

## Database Schema

No new tables are required. Discord sources fit the existing `overlay_chat_sources` schema:

```sql
-- Discord inbound source (read messages from this channel)
INSERT INTO overlay_chat_sources (
    overlay_id,
    platform,           -- 'discord'
    channel_id,         -- Discord channel snowflake (the inbound text channel)
    channel_name,       -- '#channel-name'
    config,             -- JSONB
    is_active
) VALUES (
    '<overlay_uuid>',
    'discord',
    '1234567890123456789',  -- channel snowflake
    '#general',
    '{
        "guild_id":          "9876543210987654321",
        "guild_name":        "xQc Server",
        "inbound_channel_id": "1234567890123456789",
        "relay_enabled":     true,
        "relay_channel_id":  "9999999999999999999"
    }',
    true
);
```

**Config JSONB fields:**

| Field | Type | Purpose |
|-------|------|---------|
| `guild_id` | string (snowflake) | Discord server ID — determines shard assignment |
| `guild_name` | string | Display name (denormalized for UX) |
| `inbound_channel_id` | string (snowflake) | Channel to read chat from |
| `relay_enabled` | bool | Whether to relay overlay messages back to Discord |
| `relay_channel_id` | string (snowflake) | Channel to post relay messages to (can equal `inbound_channel_id`) |

---

## Build Order and Dependencies

### Phase 1: Discord bot token + auth flow (no chat yet)
**Goal:** Bot can join servers, token stored, auth-service extended
- `auth-service`: Discord OAuth2 endpoints (authorize, callback, guild membership storage)
- Kubernetes Secret: `DISCORD_BOT_TOKEN`
- Database: no schema changes needed

**Dependencies:** None — standalone auth work

**Validation:** Bot appears in Discord server after "Add to Server" OAuth flow

### Phase 2: Inbound listener (Discord → overlay)
**Goal:** Discord messages appear in overlays
- `discord-listener/gateway`: WebSocket client, shard manager, inbound goroutine group
- `discord-listener/channels`: Channel registry, source sync from DB
- `discord-listener/publisher`: Publish to `chat:raw`
- `message-processor`: Add Discord normalizer
- `source-manager`: Add `"discord"` platform
- `overlay-manager`: Add `"discord"` platform validation

**Dependencies:** Phase 1 (bot token available)

**Validation:** Discord message appears in overlay WebSocket stream

### Phase 3: Outbound relay (overlay → Discord)
**Goal:** Non-Discord overlay messages are relayed to configured Discord channel
- `discord-listener/relay`: Pub/Sub consumer, loop prevention, token bucket, REST poster
- End-to-end test: Twitch message → overlay → Discord relay channel

**Dependencies:** Phase 2 (inbound working, channel registry established)

**Validation:** Twitch message appears in Discord channel; Discord message does NOT echo back

### Phase 4: Load balancing + HPA (production hardening)
**Goal:** Multiple discord-listener pods, shard ownership via leader election
- `discord-listener`: Startup jitter, coordinator integration, shard assignment
- Kubernetes: HPA config, Prometheus metrics, Grafana dashboard
- `source-manager`: Shard leadership keys

**Dependencies:** Phase 2-3 (service functional as single pod)

**Validation:** Scale to 3 replicas, one pod owns shard 0, others standby; failover within 60s

### Phase 5: Setup UI
**Goal:** Streamer can configure Discord sources and relay in overlay editor
- Frontend: Discord server connect card, channel picker, relay toggle
- Integrates with Phase 1 auth flow and Phase 2-3 source management

**Dependencies:** Phase 1-3 (backend APIs working)

---

## Service Interface Contracts

### discord-listener HTTP API

```
GET  /health/live         → 200 always
GET  /health/ready        → 200 if Gateway connected + Redis reachable
GET  /status              → JSON: shard status, channel count, relay count
GET  /metrics             → Prometheus metrics
```

### Key Prometheus Metrics

```
discord_gateway_messages_total{event_type}       # MESSAGE_CREATE, etc.
discord_gateway_shard_connected{shard_id}        # 0 or 1
discord_relay_messages_total{result}             # sent, dropped_echo, dropped_bot, error
discord_relay_queue_depth{channel_id}            # per-channel relay queue
discord_relay_rate_limit_waits_total{channel_id} # 429 backoffs
discord_inbound_messages_published_total         # published to chat:raw
```

---

## Environment Variables

```bash
# Discord credentials
DISCORD_BOT_TOKEN=Bot.xxxxx          # Long-lived bot token
DISCORD_APPLICATION_ID=12345...      # Bot's application/user ID (for self-filtering)
DISCORD_NUM_SHARDS=1                 # Increase when guild count approaches 2500

# Source Manager
SOURCE_MANAGER_URL=http://source-manager:8088
SOURCE_MANAGER_SECRET=dev-service-secret

# Standard (shared with all services)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat
REDIS_HOST=localhost
REDIS_PORT=6379
PORT=8092
LOG_LEVEL=info
OTEL_ENABLED=false
APP_VERSION=dev
ENVIRONMENT=development
```

---

## Sources

Research informed by:
- All-Chat existing service READMEs: `services/*/README.md`
- All-Chat existing service code: `services/kick-listener/cmd/main.go`, `services/kick-listener/websocket/client.go`
- All-Chat architecture docs: `CLAUDE.md`, `.planning/PROJECT.md`
- All-Chat prior research: `.planning/research/ARCHITECTURE.md` (sharing pattern)
- Discord Gateway API documentation (training data, MEDIUM confidence for specific limits — verify at https://discord.com/developers/docs/topics/gateway before implementation)
- Discord REST API rate limits (training data: ~5 msg/s per channel, MEDIUM confidence — verify at https://discord.com/developers/docs/topics/rate-limits)
- Discord Gateway sharding requirement: 2,500 guilds per shard limit (training data, MEDIUM confidence — verify at https://discord.com/developers/docs/topics/gateway#sharding)
