# Message Processor

The Message Processor service consumes raw chat messages from Redis Streams, normalizes them to a unified format, enriches them with emotes and user metadata, and publishes them to overlay-specific Redis Pub/Sub channels for real-time delivery.

**Port**: 8087
**Status**: ✅ Production Ready

---

## Features

- **Redis Streams Consumer**: Consumes from `chat:raw` stream using consumer group `message-processors`
- **Multi-Platform Normalization**: Converts Twitch, YouTube, Kick, TikTok messages to unified format
- **Emote Enrichment**: Adds 7TV, BTTV, FFZ emotes via external APIs with caching
- **Message Age Filtering**: Ignores messages >60 seconds old (configurable)
- **Real-time 7TV Updates**: WebSocket connection to 7TV EventAPI for immediate cache invalidation
- **Session Tracking**: Tracks stream sessions for credit roll feature
- **Deduplication**: Prevents duplicate messages across platforms
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **Metrics**: Prometheus metrics for message throughput, latency, errors

---

## Architecture

```
Redis Streams (chat:raw)
  ↓ XREADGROUP (consumer group: message-processors)
Consumer (5-10 replicas)
  ↓ route by platform
Platform-Specific Normalizers
  ├─ Twitch Normalizer    (IRC tags → unified format)
  ├─ YouTube Normalizer   (API response → unified format)
  ├─ Kick Normalizer      (Pusher event → unified format)
  └─ TikTok Normalizer    (TikTok Live → unified format)
  ↓ unified format
Enrichment Pipeline
  ├─ Avatar Enrichment    (fetch user avatars)
  ├─ Badge Enrichment     (platform badges + 7TV)
  ├─ Emote Enrichment     (7TV, BTTV, FFZ with cache)
  └─ Filter (age, duplicates)
  ↓ enriched messages
Session Tracking (credit roll feature)
  ↓ track message counts
Publisher
  ↓ PUBLISH overlay:{overlay_id}
Redis Pub/Sub
  ↓ SUBSCRIBE
API Gateway (broadcast to WebSocket clients)
```

---

## Environment Variables

### Required

```bash
# Redis connection
REDIS_HOST=localhost
REDIS_PORT=6379

# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Emote service URL
EMOTE_SERVICE_URL=http://localhost:8083
```

### Optional

```bash
# Server configuration
PORT=8087
LOG_LEVEL=info                      # debug, info, warn, error

# Message age filtering
MESSAGE_AGE_CUTOFF_SECONDS=60       # Ignore messages older than this (default: 60)

# Consumer group settings
CONSUMER_GROUP=message-processors   # Redis Streams consumer group name
CONSUMER_ID=processor-1             # Unique ID for this instance (auto-generated if not set)

# OpenTelemetry tracing
OTEL_ENABLED=false                  # Enable distributed tracing
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Application
APP_VERSION=dev
ENVIRONMENT=development
```

---

## Running Locally

### Prerequisites

- Go 1.25+
- PostgreSQL with all-chat schema
- Redis
- Emote Service running (http://localhost:8083)

### Development

```bash
# Set environment variables
export DATABASE_HOST=localhost
export REDIS_HOST=localhost
export EMOTE_SERVICE_URL=http://localhost:8083

# Run the service
cd services/message-processor
go run ./cmd

# Or build and run
go build -o message-processor ./cmd
./message-processor
```

### With Docker Compose

```bash
# Start all dependencies
make docker-up

# Message Processor starts automatically
# Check logs
docker-compose logs -f message-processor
```

---

## API Endpoints

### Health Checks

```bash
# Liveness probe (always returns 200 if service is running)
GET /health/live

# Readiness probe (checks Redis + Database connections)
GET /health/ready

# Detailed status
GET /status
```

**Example Response** (`/status`):
```json
{
  "status": "running",
  "consumer": {
    "group": "message-processors",
    "consumer_id": "processor-abc123",
    "stream": "chat:raw",
    "pending_messages": 42
  },
  "metrics": {
    "messages_processed": 15420,
    "messages_published": 15390,
    "cache_hit_rate": 0.96
  }
}
```

### Public Test-Stream Generator

Drives fake chat, poll votes (`1`/`2`/`3`/`4`) and platform events onto a single
fixed test overlay so external tools can be tested against the WebSocket feed
without any real streaming platform. It only ever targets the fixed test overlay
(`TEST_STREAM_OVERLAY_ID`, seeded by migration `058`). Disable with
`TEST_STREAM_ENABLED=false`.

**Just connect — no trigger needed.** A `DemandWatcher` reuses the api-gateway
connection-presence signal (`overlay:connected:{id}` / the `overlay:connections`
channel — the same demand signal the youtube-listener uses). The moment a client
connects the WebSocket to the test overlay, the generator starts; when the last
client disconnects, it stops. So an external tool only has to:

```
connect:  wss://allch.at/ws/overlay/00000000-0000-4000-8000-000000000a11
```

and traffic flows. Auto-start is on by default; disable with
`TEST_STREAM_AUTOSTART=false`. Tune the demand run with `TEST_STREAM_RATE`,
`TEST_STREAM_VOTE_RATIO`, `TEST_STREAM_EVENT_EVERY_N`.

**Manual control (optional)** — unauthenticated HTTP endpoints, proxied by the
api-gateway at `/api/v1/test-stream/*`. Useful for local dev where no gateway
writes the presence signal, or to force a bounded run:

```bash
# Bounded run (all body fields optional; defaults: 60s, 5 msg/s, 40% votes, event every 12)
POST /public/test-stream/start
{ "duration_seconds": 120, "rate_per_second": 8, "vote_ratio": 0.5, "event_every_n": 10 }

GET  /public/test-stream/status   # state + ws_url
POST /public/test-stream/stop
```

Helper: `make test-stream` / `scripts/start-test-stream.sh`. Note: while a client
is connected, the watcher keeps the stream alive, so a manual `stop` is
re-asserted on the next reconcile (~10s).

### Metrics

```bash
# Prometheus metrics endpoint
GET /metrics
```

**Key Metrics**:
- `processor_messages_consumed_total` - Messages consumed from Redis Streams
- `processor_messages_processed_total{result="success|error"}` - Processing results
- `processor_stage_duration_seconds{stage="normalize|avatar|badge|emote"}` - Per-stage latency
- `processor_message_duration_seconds` - End-to-end processing time
- `processor_emote_cache_hits_total` - Emote cache efficiency
- `processor_emote_cache_misses_total` - Cache misses requiring API calls

---

## Processing Pipeline

### 1. Consume from Redis Streams

**Consumer Group**: `message-processors`
**Stream**: `chat:raw`
**Pattern**: XREADGROUP with acknowledgment (XACK)

```go
// consumer/streams.go
streams, _ := client.XReadGroup(ctx, &redis.XReadGroupArgs{
    Group:    "message-processors",
    Consumer: consumerID,
    Streams:  []string{"chat:raw", ">"},
    Count:    10,
    Block:    2 * time.Second,
}).Result()
```

### 2. Route by Platform

**Router**: Detects platform field and routes to appropriate normalizer

```go
// router/router.go
switch platform {
case "twitch":
    return normalizer.ParseTwitchMessage(rawMsg)
case "youtube":
    return normalizer.ParseYouTubeMessage(rawMsg)
case "kick":
    return normalizer.ParseKickMessage(rawMsg)
case "tiktok":
    return normalizer.ParseTikTokMessage(rawMsg)
default:
    return nil, fmt.Errorf("unsupported platform: %s", platform)
}
```

### 3. Normalize to Unified Format

**Platform-Specific Normalizers**:
- `normalizer/twitch_normalizer.go` - Parse IRC tags to extract user info, badges, emotes
- `normalizer/youtube_normalizer.go` - Parse YouTube API response (authorDetails, textMessageDetails)
- `normalizer/kick_normalizer.go` - Parse Pusher WebSocket event (sender, identity, badges)
- `normalizer/tiktok_normalizer.go` - Parse TikTok Live unofficial library format

**Output**: `models.UnifiedMessage` (common schema across all platforms)

### 4. Enrich with Emotes

**Enrichment Pipeline**:

1. **Avatar Enrichment** (`enricher/avatar.go`):
   - Fetch user avatar URL from platform
   - Cache in Redis (1-hour TTL)

2. **Badge Enrichment** (`enricher/badge.go`):
   - Extract platform-specific badges (subscriber, moderator)
   - Fetch 7TV user badges (artist, developer, subscriber)
   - Cache in Redis

3. **Emote Enrichment** (`enricher/emote.go`):
   - Parse emote codes from message text
   - Call Emote Service to resolve emote URLs
   - Support 7TV, BTTV, FFZ providers
   - Cache emote data (1-hour TTL, 95%+ hit rate)

4. **Viewer Identity Enrichment** (`enricher/viewer_badge_enricher.go`):
   - Resolve All-Chat platform identity (name_color, avatar frame/flair, admin/premium badges)
   - Cross-platform Twitch username resolution via `LEFT JOIN viewer_platform_identities`
   - Sets `msg.User.TwitchUsername` (internal pipeline field, never serialized)
   - Cache: `viewer:identity:{platform}:{user_id}`, 5 min TTL

5. **Pronoun Enrichment** (`enricher/pronoun_enricher.go`):
   - Fetches pronouns from Alejo API (api.pronouns.alejo.io/v1/)
   - Runs **after** ViewerBadgeEnricher so `TwitchUsername` is available for cross-platform lookup
   - **CHAT PATH only** — not applied to events (subscriptions, raids, etc.)
   - Cache key: `pronoun:{twitch_login}`, 24h TTL
   - 404 response cached as empty sentinel to avoid re-fetching
   - Silent skip on API errors (D-05) — message renders without pronouns
   - Cross-platform: non-Twitch users with a linked Twitch account get pronouns via `TwitchUsername`
   - See: [ADR-0010](../../docs/adr/0010-pronoun-enricher-alejo-api.md)

**7TV Real-Time Updates** (`seventv/eventapi.go`):
- WebSocket connection to 7TV EventAPI (`wss://events.7tv.io/v3`)
- Subscribe to emote set updates (emote.create, emote.update, emote.delete)
- Invalidate cache immediately when emotes change
- Prevents stale emotes in cache (was 1-hour delay, now <1 second)

### 5. Filter Messages

**Age Filter** (`filter/age.go`):
- Ignore messages older than `MESSAGE_AGE_CUTOFF_SECONDS` (default 60s)
- Prevents processing backlog of stale messages (e.g., after service restart)

**Deduplication** (`dedup/dedup.go`):
- Tracks message IDs in Redis Set (5-minute TTL)
- Prevents duplicate messages if platform sends twice

### 6. Publish to Redis Pub/Sub

**Publisher** (`publisher/pubsub.go`):
- Publishes to overlay-specific channel: `overlay:{overlay_id}`
- API Gateway subscribes to these channels
- Fan-out to all overlays displaying this chat source

```go
// publisher/pubsub.go
channel := fmt.Sprintf("overlay:%s", message.OverlayID)
err := redis.Publish(ctx, channel, messageJSON).Err()
```

---

## Message Format

### Input (from Redis Streams `chat:raw`)

```json
{
  "platform": "twitch",
  "overlay_id": "uuid",
  "channel_id": "xqc",
  "channel_name": "xQc",
  "raw_message": {
    // Platform-specific format (IRC tags, YouTube API, Pusher event, etc.)
  },
  "timestamp": "2026-01-28T10:00:00Z"
}
```

### Output (to Redis Pub/Sub `overlay:{id}`)

```json
{
  "id": "uuid",
  "overlay_id": "uuid",
  "platform": "twitch",
  "channel_id": "xqc",
  "channel_name": "xQc",
  "user": {
    "id": "12345678",
    "username": "viewer123",
    "display_name": "Viewer123",
    "avatar_url": "https://static-cdn.jtvnw.net/...",
    "badges": ["subscriber", "moderator"],
    "color": "#FF0000"
  },
  "message": {
    "text": "Hello Kappa PogChamp",
    "emotes": [
      {
        "code": "Kappa",
        "provider": "twitch",
        "url": "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/1.0",
        "positions": [[6, 10]]
      },
      {
        "code": "PogChamp",
        "provider": "7tv",
        "url": "https://cdn.7tv.app/emote/.../1x.webp",
        "positions": [[12, 19]]
      }
    ]
  },
  "timestamp": "2026-01-28T10:00:00Z",
  "metadata": {
    "is_subscriber": true,
    "is_moderator": false,
    "bits": 0,
    "super_chat_amount": 0
  }
}
```

---

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./normalizer -v
go test ./enricher -v
go test ./consumer -v
```

---

## How It Works

### Consumer Group Pattern

Multiple Message Processor replicas share workload using Redis Streams consumer groups:

```
chat:raw stream: [msg1, msg2, msg3, msg4, msg5, msg6, ...]
                    ↓      ↓      ↓      ↓      ↓      ↓
Consumer Group:  processor-1  processor-2  processor-3
                 (msg1, msg4) (msg2, msg5) (msg3, msg6)
```

**Benefits**:
- Load balancing across replicas
- Fault tolerance (if replica crashes, messages reprocessed by others)
- Horizontal scaling (add more replicas to increase throughput)

**XACK Acknowledgment**:
- Message processed successfully → XACK (remove from pending)
- Processing failed → Don't XACK (retry later or claimed by another consumer)

### 7TV EventAPI Integration

**Real-time emote updates** via WebSocket:

```
7TV EventAPI (wss://events.7tv.io/v3)
  ↓ emote.create | emote.update | emote.delete
Message Processor (seventv/eventapi.go)
  ↓ invalidate cache
Redis Cache (emote:{code})
  ↓ cache miss on next lookup
Emote Service (fetch latest)
  ↓ cache result
Message Processor (use updated emote)
```

**Latency**: <1 second from emote change to cache invalidation (vs 1 hour with TTL-only caching).

**→ Documentation**: [seventv/README.md](./seventv/README.md)

---

## Monitoring

### Key Metrics to Monitor

```promql
# Message throughput
rate(processor_messages_consumed_total[5m])
rate(processor_messages_processed_total{result="success"}[5m])

# Processing latency (P95)
histogram_quantile(0.95, rate(processor_message_duration_seconds_bucket[5m]))

# Emote cache efficiency
rate(processor_emote_cache_hits_total[5m]) / (rate(processor_emote_cache_hits_total[5m]) + rate(processor_emote_cache_misses_total[5m]))

# Consumer lag (pending messages)
redis_xpending{stream="chat:raw", group="message-processors"}
```

### Alerts

**High Consumer Lag**:
```yaml
alert: MessageProcessorLag
expr: redis_xpending{stream="chat:raw", group="message-processors"} > 10000
for: 5m
severity: warning
```

**High Error Rate**:
```yaml
alert: MessageProcessorErrors
expr: rate(processor_messages_processed_total{result="error"}[5m]) > 0.05
for: 5m
severity: warning
```

---

## Troubleshooting

### High Consumer Lag (XPENDING)

**Symptom**: Messages accumulating in stream, not being processed

**Check lag**:
```bash
redis-cli XPENDING chat:raw message-processors

# Output:
# 1) (integer) 5420        # Total pending
# 2) "1234567890-0"        # Oldest message ID
# 3) "1234567899-0"        # Newest message ID
# 4) ...                   # Per-consumer breakdown
```

**Solutions**:
1. **Scale up**: Increase replicas (3 → 5 → 10)
2. **Check CPU**: If CPU >80%, processor overwhelmed with emote enrichment
3. **Check emote cache**: Low hit rate = more API calls = slower processing
4. **Check Redis**: Slow PUBLISH to Pub/Sub can bottleneck
5. **Increase MAXLEN**: If stream fills too quickly

**File**: `consumer/streams.go:Consume()`

---

### Messages Not Being Published

**Symptom**: Messages processed but not appearing in overlays

**Check Pub/Sub**:
```bash
# Check if messages being published
redis-cli PUBSUB CHANNELS overlay:*

# Subscribe to specific overlay (manual test)
redis-cli SUBSCRIBE overlay:{overlay-id}

# Should see messages appearing in real-time
```

**Solutions**:
1. Check Redis Pub/Sub connection
2. Verify overlay ID matches database records
3. Check API Gateway is subscribed to overlay channel
4. Review logs for publishing errors

**File**: `publisher/pubsub.go:Publish()`

---

### Emote Enrichment Slow

**Symptom**: High P95 latency in `processor_stage_duration_seconds{stage="emote"}`

**Check cache hit rate**:
```bash
curl http://localhost:8087/metrics | grep emote_cache

# Target: >95% hit rate
# If <90%, investigate:
# - Redis connection slow?
# - Cache TTL too short?
# - Many unique emotes (low reuse)?
```

**Solutions**:
1. **Increase cache TTL**: 1 hour → 6 hours (reduces API calls)
2. **Preload popular emotes**: Cache top 1,000 global emotes on startup
3. **Batch API calls**: Fetch multiple emotes in one request (if emote service supports)
4. **Check 7TV EventAPI**: Ensure WebSocket connected for real-time updates

**File**: `enricher/emote.go:EnrichEmotes()`

---

### 7TV EventAPI Not Connected

**Symptom**: 7TV emotes not updating in real-time (1-hour delay from cache TTL)

**Check connection**:
```bash
# Check logs for 7TV EventAPI
kubectl logs -n allchat deployment/message-processor | grep "7TV EventAPI"

# Expected:
# INFO: 7TV EventAPI connected  url=wss://events.7tv.io/v3
# INFO: Subscribed to emote set  set_id=abc123
```

**Solutions**:
1. Verify network connectivity: `curl https://events.7tv.io`
2. Check firewall rules (WebSocket port may be blocked)
3. Review logs for WebSocket errors
4. Restart service to reconnect

**File**: `seventv/eventapi.go:Connect()`

---

## Performance

### Capacity

**Per Replica**:
- **Throughput**: ~3,000 messages/second (with emote enrichment, 95% cache hit)
- **CPU**: 200-1000m (varies with emote API calls)
- **Memory**: 256Mi-1Gi (depends on cache size)

**Bottlenecks**:
1. **Emote Enrichment**: External API calls to 7TV, BTTV, FFZ (50-200ms per miss)
2. **JSON Serialization**: Marshaling messages for Pub/Sub (~1ms per message)
3. **Redis PUBLISH**: Pub/Sub publish latency (~5-10ms)

### Scaling Guidelines

| Message Rate | Replicas | CPU Total | Memory Total |
|--------------|----------|-----------|--------------|
| 500 msg/s | 3 (min) | 600m | 768Mi |
| 1,000 msg/s | 3 | 900m | 768Mi |
| 3,000 msg/s | 5 | 2 cores | 2.5Gi |
| 5,000 msg/s | 7 | 4 cores | 4Gi |
| 10,000 msg/s | 10 | 6 cores | 6Gi |

---

## Production Considerations

1. **Consumer Group**: All replicas must use same group name (`message-processors`)
2. **Consumer ID**: Must be unique per replica (auto-generated with pod name)
3. **Emote Cache**: Monitor hit rate, target >95% (higher = lower latency)
4. **7TV EventAPI**: Keep WebSocket connected for real-time emote updates
5. **Message Age Filter**: Tune `MESSAGE_AGE_CUTOFF_SECONDS` based on use case
6. **Session Tracking**: Requires database (credit roll feature depends on this)
7. **Monitoring**: Alert on high consumer lag (>10,000 messages pending)

---

## Related Services

- **Twitch Listener**: Publishes to `chat:raw` stream
- **YouTube Listener**: Publishes to `chat:raw` stream
- **Kick Listener**: Publishes to `chat:raw` stream
- **TikTok Listener**: Publishes to `chat:raw` stream
- **Emote Service**: Provides emote URLs (7TV, BTTV, FFZ)
- **API Gateway**: Subscribes to `overlay:*` Pub/Sub channels

---

## Further Reading

- **[01-DATA-FLOW.md](../../docs/architecture/01-DATA-FLOW.md)** - Complete message flow architecture
- **[ADR-0002](../../docs/adr/0002-redis-streams-pubsub.md)** - Redis Streams + Pub/Sub decision
- **[QUICK-REF-REDIS-OPERATIONS.md](../../docs/llm-guides/QUICK-REF-REDIS-OPERATIONS.md)** - Redis debugging commands

---

## License

Copyright © 2025 All-Chat. All rights reserved.
