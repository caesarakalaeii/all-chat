# Architecture

**Analysis Date:** 2026-02-16

## Pattern Overview

**Overall:** Microservices with Standard Go Layout + Redis hybrid messaging + Reverse proxy pattern for frontend integration

**Key Characteristics:**
- **Standard Go Layout** (not hexagonal/onion) - Service per directory with `cmd/`, `handlers/`, domain packages
- **Hybrid message routing** - Redis Streams for durable queues, Redis Pub/Sub for real-time broadcast
- **Stateless services** - All compute services scale horizontally; state in PostgreSQL + Redis only
- **Cloud-native** - Kubernetes-deployable with health checks, graceful shutdown, metrics/tracing
- **LLM-friendly** - Minimal boilerplate, clear separation of concerns, explicit imports

---

## Layers

**Listener Layer:**
- Purpose: Connect to external streaming platforms, parse protocol-specific messages
- Location: `services/twitch-listener/`, `services/youtube-listener/`, `services/kick-listener/`, `services/tiktok-listener/`
- Contains: Platform-specific clients (IRC, HTTP, WebSocket), channel managers, publishers
- Depends on: Redis (streams publish), PostgreSQL (channel store, token storage), platform SDKs
- Used by: Message Processor (consumes from Redis Streams)

**Message Processing Layer:**
- Purpose: Normalize platform-specific messages, enrich with emotes, route to overlays
- Location: `services/message-processor/`
- Contains: Normalizer, enricher (emote lookups), router (overlay matching), consumer (Redis Streams), publisher (Redis Pub/Sub)
- Depends on: Redis (streams consume, pub/sub publish), PostgreSQL (overlay config), Emote Service
- Used by: API Gateway (receives via Redis Pub/Sub)

**API Gateway Layer:**
- Purpose: WebSocket server, HTTP reverse proxy, real-time message distribution
- Location: `services/api-gateway/`
- Contains: WebSocket connection manager, subscription handler, HTTP proxy for auth/overlay endpoints
- Depends on: Redis (pub/sub subscribe), PostgreSQL (overlay verification), Auth Service
- Used by: Frontend (WebSocket connections)

**Support Services Layer:**
- Purpose: OAuth, emote caching, overlay CRUD, token refresh, source coordination
- Location: `services/auth-service/`, `services/emote-service/`, `services/overlay-manager/`, `services/token-refresh-service/`, `services/source-manager/`
- Contains: OAuth handlers, emote cache, overlay repository, token rotation, leader election
- Depends on: PostgreSQL, Redis (optional), external platform APIs
- Used by: API Gateway (auth), Message Processor (emotes), Listeners (config)

**Frontend Layer:**
- Purpose: Overlay display, dashboard, administration
- Location: `frontend/`
- Contains: Next.js App Router, React components, WebSocket client
- Depends on: API Gateway (HTTP + WebSocket)
- Used by: Streamers (viewers access through overlay embed)

**Shared Utilities Layer:**
- Purpose: Common infrastructure code reused across services
- Location: `shared/`
- Contains: Database pool, Redis client wrappers, logger, middleware, metrics, auth, tracing
- Depends on: PostgreSQL driver (pgx), Redis driver (go-redis), OpenTelemetry SDK, Zap logger
- Used by: All services

---

## Data Flow

**Raw Message Capture Flow:**

1. **Platform → Listener** (varies by platform)
   - Twitch: IRC PRIVMSG → `twitch-listener` parses and extracts user, message, emotes
   - YouTube: HTTP GET live chat → `youtube-listener` polls and extracts
   - Kick: WebSocket subscription → `kick-listener` receives JSON payloads
   - TikTok: Unofficial client → `tiktok-listener` extracts

2. **Listener → Redis Streams** (`stream:raw-messages`)
   ```
   XADD stream:raw-messages * platform=twitch channel_id=12345 user_id=999 \
        username=viewer text="Hello Kappa" raw_emotes="25:13-17" timestamp=...
   ```

3. **Message Processor consumes** (consumer group: `msg-processor`)
   ```
   XREADGROUP GROUP msg-processor consumer1 STREAMS stream:raw-messages >
   ```

4. **Normalize → Enrich → Route**
   - Normalizer: Transform platform fields to unified schema
   - Enricher: Query Emote Service for 7TV, BTTV, FFZ, platform-native emotes
   - Router: Query PostgreSQL for overlays monitoring this channel, publish per overlay

5. **Publish to Redis Pub/Sub** (per overlay)
   ```
   PUBLISH overlay:abc-123-def-456 {"message": {...}, "emotes": [...]}
   ```

6. **API Gateway subscribes** (`SUBSCRIBE overlay:*`)
   - Maintains subscriber registry in Redis: `subscribers:overlay:{id}` = set of WebSocket connection IDs
   - On message: lookup connected clients, push via WebSocket

7. **Frontend receives** (WebSocket)
   - Parse message, render with emote images, display in overlay

**State Management:**

- **PostgreSQL**: Overlays, sources, users, credentials, tokens, stream history, quotas, ban lists
- **Redis Streams**: Raw messages (retention: 50K entries per stream, ~1 day in production)
- **Redis Pub/Sub**: Transient message delivery (no persistence; fire-and-forget)
- **Redis Hashes/Strings**: Session tokens, emote cache, source registry, subscriber tracking

---

## Key Abstractions

**Unified Message Format:**
- Purpose: Single schema across all platforms for downstream processing
- Examples: `services/message-processor/models/message.go`, `services/api-gateway/models/`
- Pattern: Struct with platform identifier, user info (id, name, display_name), message text, emotes array, badges, metadata JSON

```go
type Message struct {
    ID          string
    Platform    string
    ChannelID   string
    UserID      string
    Username    string
    DisplayName string
    Text        string
    Emotes      []Emote
    Badges      []Badge
    Timestamp   time.Time
    Metadata    map[string]interface{}
}
```

**Channel Manager:**
- Purpose: Track active channels per platform listener, manage subscriptions
- Examples: `services/twitch-listener/channels/manager.go`, `services/youtube-listener/streams/`
- Pattern: Map of channel_id → channel state, publish/subscribe on join/part, handle channel-level errors

**Emote Provider:**
- Purpose: Abstract 7TV, BTTV, FFZ API differences
- Examples: `services/emote-service/handlers/`, `services/message-processor/enricher/`
- Pattern: Interface-based (GetEmotes(channelID) → []Emote), cache backend strategy

**Redis Stream Consumer Group:**
- Purpose: Horizontal consumer scaling, at-least-once delivery with acknowledgment
- Examples: `services/message-processor/consumer/`
- Pattern: XREADGROUP with XACK after processing, retry on error

**Overlay Router:**
- Purpose: Determine which overlays should receive each message based on sources
- Examples: `services/message-processor/router/router.go`
- Pattern: Query DB for (overlay_id, [sources]) mapping, filter by channel match

---

## Entry Points

**Twitch Listener:**
- Location: `services/twitch-listener/cmd/main.go`
- Triggers: Container startup
- Responsibilities:
  1. Load TWITCH_BOT_USERNAME, TWITCH_BOT_OAUTH from environment
  2. Connect to PostgreSQL (fetch subscribed channels from DB)
  3. Connect to Redis (publish stream)
  4. Initialize IRC client, join channels
  5. Listen for PRIVMSG, publish to Redis Streams
  6. Expose `/health/live`, `/health/ready`, `/metrics` on port 8085

**YouTube Listener:**
- Location: `services/youtube-listener/cmd/main.go`
- Triggers: Container startup
- Responsibilities:
  1. Load YOUTUBE_CLIENT_ID, YOUTUBE_CLIENT_SECRET, API credentials
  2. Connect to PostgreSQL (fetch YouTube channels from DB, track quota)
  3. Connect to Redis (publish stream)
  4. Poll YouTube Live Chat API on interval (varies by quota availability)
  5. Publish to Redis Streams
  6. Expose `/health/live`, `/health/ready`, `/metrics` on port 8086

**Message Processor:**
- Location: `services/message-processor/cmd/main.go`
- Triggers: Container startup
- Responsibilities:
  1. Connect to PostgreSQL (query overlay config)
  2. Connect to Redis (consume streams, publish pub/sub)
  3. Create consumer group if not exists: XGROUP CREATE stream:raw-messages msg-processor $ MKSTREAM
  4. Loop: XREADGROUP, normalize, enrich (call emote-service), route, publish
  5. XACK after successful publish
  6. Expose `/health/live`, `/health/ready`, `/metrics` on port 8087

**API Gateway:**
- Location: `services/api-gateway/cmd/main.go`
- Triggers: Container startup or SIGHUP (reload)
- Responsibilities:
  1. Connect to PostgreSQL (verify overlays, fetch viewer tokens)
  2. Connect to Redis (subscribe to overlay:*, maintain subscriber registry)
  3. Initialize WebSocket hub with connection manager
  4. Route:
     - `/api/auth/*` → Auth Service (reverse proxy)
     - `/api/overlays/*` → Overlay Manager (reverse proxy)
     - `/api/emotes/*` → Emote Service (reverse proxy)
     - `/api/health/live` → local 200 OK
     - `/api/health/ready` → check DB + Redis + auth service
     - `/ws/:overlayID` → WebSocket upgrade
  5. On WebSocket connect: validate overlay access, subscribe to overlay:* channel
  6. On Redis message: broadcast to all connected viewers
  7. Expose `/metrics` on port 8080

**Auth Service:**
- Location: `services/auth-service/cmd/main.go`
- Triggers: Container startup
- Responsibilities:
  1. Connect to PostgreSQL (user store, credentials, tokens)
  2. Route:
     - `POST /api/auth/login` → OAuth redirect
     - `GET /api/auth/callback` → Exchange code, store token encrypted, set JWT cookie
     - `GET /api/auth/logout` → Clear JWT
     - `GET /api/auth/me` → Return current user
     - `POST /api/auth/viewer-token` → Generate ephemeral viewer JWT
  3. Expose `/health/live`, `/health/ready`, `/metrics` on port 8081

**Overlay Manager:**
- Location: `services/overlay-manager/cmd/main.go`
- Triggers: Container startup
- Responsibilities:
  1. Connect to PostgreSQL (overlay CRUD)
  2. Route:
     - `POST /api/overlays` → Create overlay
     - `GET /api/overlays/:id` → Fetch overlay
     - `PUT /api/overlays/:id` → Update overlay (add/remove sources)
     - `DELETE /api/overlays/:id` → Delete overlay
  3. Maintain reverse mapping: overlayID → sources in cache (used by message processor)
  4. Expose `/health/live`, `/health/ready`, `/metrics` on port 8082

**Frontend:**
- Location: `frontend/src/app/page.tsx` (Next.js App Router entry)
- Triggers: HTTP GET /
- Responsibilities:
  1. Proxy API calls to API Gateway (`/api/*` → http://api-gateway:8080/api/*)
  2. Render dashboard, admin panel, overlay embed
  3. Establish WebSocket to API Gateway on overlay page
  4. Listen for messages, render in real-time
  5. Serve on port 3000

---

## Error Handling

**Strategy:** Graceful degradation with exponential backoff for external APIs

**Patterns:**

**Listener Reconnect:**
- Twitch IRC disconnect → exponential backoff (1s, 2s, 4s... 60s max), rejoin channels
- YouTube API 403 (quota exceeded) → delay polling, check reserve-confirm-rollback state machine
- Kick WebSocket close → reconnect with exponential backoff
- File: `services/twitch-listener/irc/client.go` (reconnect loop)

**Message Processing Retry:**
- Redis Streams XACK fails → message stays in queue, picked up by another consumer
- Emote Service timeout → publish message without emotes (partial enrichment acceptable)
- Database query fail → retry with exponential backoff, eventually NACK
- File: `services/message-processor/consumer/consumer.go`

**Pub/Sub Delivery:**
- Redis Pub/Sub loss (no persistence) → acceptable, real-time only
- WebSocket client disconnect → close connection, clean up subscriber registry
- File: `services/api-gateway/websocket/hub.go`

**HTTP Endpoints:**
- 5xx errors → return 500 with error ID for logging
- 4xx errors (validation) → return 400 with field errors
- Database unavailable → `/health/ready` returns 503, orchestrator removes from load balancer
- File: `shared/middleware/error_handler.go`

---

## Cross-Cutting Concerns

**Logging:**
- Framework: `go.uber.org/zap` (structured JSON)
- Every service initializes in `cmd/main.go` with level from LOG_LEVEL env (default: info)
- Pattern: `log.Info("message", zap.String("key", "value"), zap.Error(err))`
- File: `shared/logger/logger.go`

**Validation:**
- Framework: Gin request binding (`c.ShouldBindJSON()`)
- Custom validators in handlers (e.g., overlay_id UUID format)
- Pattern: Validate early in handler, return 400 if invalid
- File: Each `handlers/` directory

**Authentication:**
- JWT tokens stored in HTTP-only cookies (streamers) or ephemeral viewer tokens
- Middleware checks token on protected routes
- Pattern: `middleware.AuthRequired()` in Gin router
- File: `shared/middleware/auth.go`

**Rate Limiting:**
- Applied at API Gateway for `/api/*` endpoints
- Pattern: Token bucket per IP or user
- File: `shared/ratelimit/`

**Metrics:**
- Framework: Prometheus (counters, gauges, histograms)
- Exposed on `/metrics` port 9090 (or combined with service port)
- Pattern: `metrics.RecordLatency("message_processor.enrich", duration)`
- File: `shared/metrics/`

**Tracing:**
- Framework: OpenTelemetry (OTLP exporter to Tempo)
- Optional: enabled via OTEL_ENABLED=true env var
- Pattern: Automatic span creation for HTTP handlers, manual spans for long operations
- File: `shared/tracing/`

**Database Migrations:**
- Tool: Manual SQL files in `migrations/` directory (no ORM migration tool)
- Pattern: Numbered files (001, 002, ...) with UP and DOWN SQL
- Location: `migrations/*.sql`
- Applied via `make migrate-up` (uses pgx connection)

