# Phase 3: Twitch Real-Time - Detailed Plan

**Version**: 1.0
**Created**: 2025-11-13
**Duration**: 3-4 weeks (Nov 13 - Dec 11)
**Priority**: P0 (Critical Path to Twitch Launch)

---

## Overview

Phase 3 implements real-time chat message flow for Twitch, completing the core platform functionality. This phase builds on the infrastructure from Phases 1-2 (Auth, Overlay Manager, Emote Service, API Gateway) to enable live chat aggregation.

**Key Deliverables**:
1. **Twitch Listener** - IRC connection, message parsing
2. **Message Processor** - Message normalization, emote enrichment, routing
3. **API Gateway WebSocket** - Real-time message delivery to overlays

---

## Architecture

```
┌──────────────┐
│ Twitch IRC   │
└──────┬───────┘
       │ IRC Protocol
       ▼
┌────────────────────┐       ┌──────────────┐
│ Twitch Listener    │──────→│ Redis Streams│
│ (Port 8085)        │       │ chat:raw     │
└────────────────────┘       └──────┬───────┘
                                     │
                                     ▼
┌────────────────────┐       ┌──────────────┐
│ Message Processor  │←──────│ Redis Streams│
│ (Port 8087)        │       │ chat:raw     │
└────────┬───────────┘
         │ Normalize & Enrich
         │
         ├──→ Emote Service (8083)
         │
         ▼
┌──────────────┐
│ Redis Pub/Sub│
│ overlay:{id} │
└──────┬───────┘
       │
       ▼
┌────────────────────┐       ┌──────────────┐
│ API Gateway        │──────→│  Overlay     │
│ WebSocket (8080)   │       │  (Browser)   │
└────────────────────┘       └──────────────┘
```

### Message Flow

1. **Twitch IRC → Twitch Listener**:
   - Connect to IRC (irc.chat.twitch.tv:6667)
   - JOIN channels based on active overlays
   - Parse PRIVMSG into raw message struct

2. **Twitch Listener → Redis Streams**:
   - Publish to stream: `chat:raw`
   - Message includes: platform, channel, user, text, timestamp

3. **Message Processor → Redis Streams**:
   - Consumer group reads from `chat:raw`
   - Normalize to unified format
   - Call Emote Service to enrich with emote metadata
   - Determine target overlay(s) from database

4. **Message Processor → Redis Pub/Sub**:
   - Publish to `overlay:{overlay_id}` channel
   - Message in final JSON format ready for overlay

5. **API Gateway → Overlay (WebSocket)**:
   - Client connects: `ws://localhost:8080/ws/overlay/{id}?token=JWT`
   - Gateway subscribes to `overlay:{id}` Redis channel
   - Broadcasts messages to all connected WebSocket clients

---

## Service 1: Twitch Listener

### Purpose
Connect to Twitch IRC, listen to configured channels, parse messages, and publish raw messages to Redis Streams.

### Architecture

```
┌─────────────────────────────┐
│   Twitch Listener           │
│   ┌─────────────────────┐   │
│   │ Connection Manager  │   │
│   │ - IRC connection    │   │
│   │ - Reconnection      │   │
│   │ - Health monitoring │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Channel Manager     │   │
│   │ - JOIN/PART logic   │   │
│   │ - Active channels   │   │
│   │ - Rate limiting     │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Message Handler     │   │
│   │ - Parse PRIVMSG     │   │
│   │ - Extract user info │   │
│   │ - Publish to stream │   │
│   └─────────────────────┘   │
└─────────────────────────────┘
```

### Data Model

```go
// RawChatMessage is published to Redis Streams
type RawChatMessage struct {
    MessageID string    `json:"message_id"`  // UUID
    Platform  string    `json:"platform"`    // "twitch"
    ChannelID string    `json:"channel_id"`  // Twitch channel name
    UserID    string    `json:"user_id"`     // Twitch user ID
    Username  string    `json:"username"`    // Display name
    Text      string    `json:"text"`        // Raw message text
    Timestamp time.Time `json:"timestamp"`   // UTC
    Tags      map[string]string `json:"tags"` // IRC tags
}
```

### IRC Tags to Extract

```go
tags := map[string]string{
    "user-id":       "12345678",
    "display-name":  "XQC",
    "color":         "#FF0000",
    "badges":        "broadcaster/1,subscriber/12",
    "subscriber":    "1",
    "mod":           "0",
    "turbo":         "0",
    "emotes":        "25:0-4,12-16/1902:6-10", // Twitch native emotes
}
```

### Twitch IRC Rate Limits

- **JOIN**: 20 channels per 10 seconds (authenticated)
- **PRIVMSG**: 100 messages per 30 seconds (verified bot)
- **Connection**: Max 50 concurrent channels per connection (use multiple connections if needed)

### Configuration

```bash
# Environment Variables
TWITCH_BOT_USERNAME=allchat_bot
TWITCH_BOT_OAUTH=oauth:abc123...
REDIS_HOST=localhost
REDIS_PORT=6379
DATABASE_HOST=localhost
DATABASE_PORT=5432
LOG_LEVEL=info
PORT=8085
```

### File Structure

```
services/twitch-listener/
├── cmd/
│   └── main.go                      # Entry point
├── handlers/
│   ├── health.go                    # Health checks
│   └── health_test.go
├── irc/
│   ├── connection.go                # IRC connection manager
│   ├── connection_test.go
│   ├── parser.go                    # IRC message parser
│   ├── parser_test.go
│   └── mock.go                      # Mock IRC for testing
├── channels/
│   ├── manager.go                   # Channel JOIN/PART logic
│   ├── manager_test.go
│   └── repository.go                # DB queries for active channels
├── publisher/
│   ├── stream_publisher.go          # Redis Streams publisher
│   └── stream_publisher_test.go
├── models/
│   ├── raw_message.go               # RawChatMessage struct
│   └── raw_message_test.go
├── go.mod
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Days 1-2: IRC Connection & Parsing
- [ ] Create `irc/connection.go` with basic IRC client
- [ ] Implement PASS, NICK, CAP REQ, JOIN commands
- [ ] Parse PRIVMSG and extract tags
- [ ] Write tests with mock IRC server
- [ ] Tests pass ✅

#### Days 3-4: Channel Management
- [ ] Create `channels/manager.go`
- [ ] Query database for active overlay sources (platform=twitch)
- [ ] Implement JOIN logic with rate limiting (20/10s)
- [ ] Implement PART logic when overlay deactivated
- [ ] Periodic sync with database (every 30s)
- [ ] Tests pass ✅

#### Days 5-6: Redis Streams Publishing
- [ ] Create `publisher/stream_publisher.go`
- [ ] Publish messages to `chat:raw` stream
- [ ] Use XADD with MAXLEN ~1000000 (1M message sliding window)
- [ ] Handle Redis connection failures with retry
- [ ] Tests pass ✅

#### Days 7-8: Reconnection & Health
- [ ] Implement exponential backoff for reconnections
- [ ] Detect disconnections (PING timeout)
- [ ] Re-JOIN all channels after reconnection
- [ ] Health endpoint shows IRC status
- [ ] Manual testing with real Twitch IRC
- [ ] All tests pass ✅

### Success Criteria

- [ ] Can connect to Twitch IRC
- [ ] Can JOIN 100+ channels
- [ ] Parses PRIVMSG correctly with all tags
- [ ] Publishes to Redis Streams with no data loss
- [ ] Handles disconnections and reconnects automatically
- [ ] Respects Twitch rate limits
- [ ] Test coverage ≥ 85%
- [ ] Docker build succeeds
- [ ] Can run with docker-compose

---

## Service 2: Message Processor

### Purpose
Consume raw messages from Redis Streams, normalize to unified format, enrich with emotes, determine target overlays, and publish to overlay-specific Redis Pub/Sub channels.

### Architecture

```
┌─────────────────────────────┐
│   Message Processor         │
│   ┌─────────────────────┐   │
│   │ Stream Consumer     │   │
│   │ - Consumer group    │   │
│   │ - ACK management    │   │
│   │ - Backlog handling  │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Normalizer          │   │
│   │ - Platform-specific │   │
│   │ - Unified format    │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Emote Enricher      │   │
│   │ - Call Emote Svc    │   │
│   │ - Match emote codes │   │
│   │ - Add URLs          │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Overlay Router      │   │
│   │ - Query DB          │   │
│   │ - Find overlays     │   │
│   │ - Publish to ch     │   │
│   └─────────────────────┘   │
└─────────────────────────────┘
```

### Data Model

```go
// UnifiedChatMessage is published to Redis Pub/Sub
type UnifiedChatMessage struct {
    ID          string                 `json:"id"`           // UUID
    OverlayID   string                 `json:"overlay_id"`   // Target overlay
    Platform    string                 `json:"platform"`     // "twitch", "youtube"
    ChannelID   string                 `json:"channel_id"`   // Platform channel
    ChannelName string                 `json:"channel_name"` // Display name
    User        UserInfo               `json:"user"`
    Message     MessageInfo            `json:"message"`
    Timestamp   time.Time              `json:"timestamp"`
    Metadata    map[string]interface{} `json:"metadata"`
}

type UserInfo struct {
    ID          string   `json:"id"`
    Username    string   `json:"username"`
    DisplayName string   `json:"display_name"`
    AvatarURL   string   `json:"avatar_url,omitempty"`
    Badges      []string `json:"badges"`
    Color       string   `json:"color,omitempty"`
}

type MessageInfo struct {
    Text   string  `json:"text"`
    Emotes []Emote `json:"emotes"`
}

type Emote struct {
    Code      string     `json:"code"`      // "Kappa"
    Provider  string     `json:"provider"`  // "twitch", "7tv", "bttv", "ffz"
    URL       string     `json:"url"`
    Positions [][]int    `json:"positions"` // [[0, 5], [12, 17]]
}
```

### Redis Streams Consumer Group

```bash
# Create consumer group (run once)
XGROUP CREATE chat:raw message-processor $ MKSTREAM

# Read messages
XREADGROUP GROUP message-processor consumer-1 COUNT 100 BLOCK 5000 STREAMS chat:raw >

# ACK messages after processing
XACK chat:raw message-processor <message-id>
```

### Emote Enrichment Flow

1. **Extract Twitch Native Emotes** (from IRC tags):
   ```
   emotes: "25:0-4,12-16/1902:6-10"
   → Kappa at positions [0-4], [12-16]
   → Keepo at positions [6-10]
   ```

2. **Call Emote Service**:
   ```
   GET http://emote-service:8083/emotes/channel/{channel}
   → Returns all 7TV, BTTV, FFZ emotes for channel
   ```

3. **Match Emote Codes in Text**:
   - Tokenize message text
   - For each word, check if it matches an emote code
   - Add emote object with URL and positions

### Overlay Routing Logic

```sql
-- Find all overlays that should receive this message
SELECT DISTINCT o.id, o.user_id
FROM overlays o
JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
WHERE o.is_active = true
  AND ocs.platform = 'twitch'
  AND ocs.channel_id = $1
```

### File Structure

```
services/message-processor/
├── cmd/
│   └── main.go                      # Entry point
├── handlers/
│   ├── health.go                    # Health checks
│   └── health_test.go
├── consumer/
│   ├── stream_consumer.go           # Redis Streams consumer
│   ├── stream_consumer_test.go
│   └── consumer_group.go            # Consumer group management
├── normalizer/
│   ├── twitch_normalizer.go         # Twitch → Unified
│   ├── twitch_normalizer_test.go
│   └── normalizer.go                # Interface
├── enricher/
│   ├── emote_enricher.go            # Call Emote Service
│   ├── emote_enricher_test.go
│   └── emote_client.go              # HTTP client for Emote Service
├── router/
│   ├── overlay_router.go            # Find target overlays
│   ├── overlay_router_test.go
│   └── repository.go                # DB queries
├── publisher/
│   ├── pubsub_publisher.go          # Redis Pub/Sub
│   └── pubsub_publisher_test.go
├── models/
│   ├── raw_message.go               # Input from Streams
│   ├── unified_message.go           # Output to Pub/Sub
│   └── models_test.go
├── go.mod
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Days 1-2: Stream Consumer
- [ ] Create `consumer/stream_consumer.go`
- [ ] Implement XREADGROUP with consumer group
- [ ] Handle XACK after successful processing
- [ ] Implement backlog recovery (XPENDING)
- [ ] Tests with mock Redis Streams
- [ ] Tests pass ✅

#### Days 3-4: Normalizer
- [ ] Create `normalizer/twitch_normalizer.go`
- [ ] Parse Twitch IRC tags into UserInfo
- [ ] Extract native Twitch emotes from tags
- [ ] Generate UUID for message ID
- [ ] Write comprehensive tests
- [ ] Tests pass ✅

#### Days 5-6: Emote Enricher
- [ ] Create `enricher/emote_enricher.go`
- [ ] HTTP client for Emote Service
- [ ] Match emote codes in message text
- [ ] Handle Emote Service failures gracefully (skip enrichment)
- [ ] Cache emote responses (1 minute TTL)
- [ ] Tests with mock HTTP server
- [ ] Tests pass ✅

#### Days 7-8: Overlay Router & Publisher
- [ ] Create `router/overlay_router.go`
- [ ] Query database for matching overlays
- [ ] Create `publisher/pubsub_publisher.go`
- [ ] Publish to `overlay:{id}` Redis channels
- [ ] Wire everything in main.go
- [ ] Integration tests with Redis + Postgres
- [ ] All tests pass ✅

### Success Criteria

- [ ] Consumes from Redis Streams with consumer group
- [ ] Normalizes Twitch messages to unified format
- [ ] Enriches with emotes from Emote Service
- [ ] Routes to correct overlay Pub/Sub channels
- [ ] Handles failures with retry logic
- [ ] Processes 1000+ messages/second
- [ ] Test coverage ≥ 85%
- [ ] Docker build succeeds
- [ ] Can run with docker-compose

---

## Service 3: API Gateway WebSocket Enhancement

### Purpose
Add WebSocket support to existing API Gateway to deliver real-time messages to overlay clients.

### Architecture

```
┌─────────────────────────────┐
│   API Gateway (Enhanced)    │
│   ┌─────────────────────┐   │
│   │ HTTP Handlers       │   │
│   │ (Phase 2)          │   │
│   └─────────────────────┘   │
│   ┌─────────────────────┐   │
│   │ WebSocket Manager   │   │
│   │ - Connection pool   │   │
│   │ - JWT validation    │   │
│   │ - Ping/Pong        │   │
│   └──────────┬──────────┘   │
│              │               │
│   ┌──────────▼──────────┐   │
│   │ Subscription Mgr    │   │
│   │ - Redis Pub/Sub     │   │
│   │ - Per-overlay subs  │   │
│   │ - Broadcast         │   │
│   └─────────────────────┘   │
└─────────────────────────────┘
```

### WebSocket Endpoint

```
GET ws://localhost:8080/ws/overlay/{overlay_id}?token={jwt}
```

**Query Parameters**:
- `token`: JWT token (same as HTTP auth)

**Connection Lifecycle**:
1. Client connects with JWT
2. Validate JWT and extract user_id
3. Verify user owns overlay_id (query database)
4. Subscribe to Redis channel `overlay:{overlay_id}`
5. Send messages as they arrive
6. Handle disconnection (unsubscribe)

### WebSocket Message Format

**Server → Client (chat message)**:
```json
{
  "type": "chat_message",
  "data": {
    "id": "uuid",
    "overlay_id": "uuid",
    "platform": "twitch",
    "channel_name": "xqc",
    "user": {
      "username": "viewer123",
      "display_name": "Viewer123",
      "color": "#FF0000",
      "badges": ["subscriber"]
    },
    "message": {
      "text": "Hello Kappa",
      "emotes": [
        {
          "code": "Kappa",
          "provider": "twitch",
          "url": "https://...",
          "positions": [[6, 11]]
        }
      ]
    },
    "timestamp": "2025-11-13T10:00:00Z"
  }
}
```

**Server → Client (ping)**:
```json
{
  "type": "ping",
  "timestamp": "2025-11-13T10:00:00Z"
}
```

**Client → Server (pong)**:
```json
{
  "type": "pong",
  "timestamp": "2025-11-13T10:00:00Z"
}
```

### File Structure

```
services/api-gateway/
├── cmd/main.go                      # Update with WebSocket routes
├── handlers/
│   ├── websocket.go                 # NEW: WebSocket handler
│   ├── websocket_test.go            # NEW: WebSocket tests
│   ├── proxy.go                     # Existing
│   └── health.go                    # Existing
├── websocket/
│   ├── manager.go                   # NEW: Connection manager
│   ├── manager_test.go
│   ├── connection.go                # NEW: Single WS connection
│   ├── connection_test.go
│   └── pool.go                      # NEW: Connection pool
├── subscription/
│   ├── subscriber.go                # NEW: Redis Pub/Sub
│   ├── subscriber_test.go
│   └── repository.go                # NEW: Verify overlay ownership
├── models/
│   ├── ws_message.go                # NEW: WebSocket message types
│   └── ws_message_test.go
└── ... (existing files)
```

### Implementation Checklist

#### Days 1-2: WebSocket Connection
- [ ] Create `websocket/connection.go`
- [ ] Implement gorilla/websocket upgrade
- [ ] JWT validation from query parameter
- [ ] Ping/Pong keep-alive (30s interval)
- [ ] Write tests with mock WebSocket
- [ ] Tests pass ✅

#### Days 3-4: Connection Manager & Pool
- [ ] Create `websocket/manager.go`
- [ ] Connection pool per overlay
- [ ] Thread-safe add/remove connections
- [ ] Broadcast message to all connections in pool
- [ ] Tests pass ✅

#### Days 5-6: Redis Pub/Sub Integration
- [ ] Create `subscription/subscriber.go`
- [ ] Subscribe to `overlay:{id}` on first connection
- [ ] Unsubscribe when last connection closes
- [ ] Forward messages from Redis to WebSocket
- [ ] Tests with mock Redis
- [ ] Tests pass ✅

#### Days 7: Integration & Testing
- [ ] Wire everything in main.go
- [ ] Verify overlay ownership before allowing connection
- [ ] Integration tests with real Redis
- [ ] Manual testing with websocat
- [ ] All tests pass ✅

### Success Criteria

- [ ] WebSocket endpoint accepts connections with JWT
- [ ] Validates user owns overlay
- [ ] Subscribes to correct Redis Pub/Sub channel
- [ ] Broadcasts messages to all connected clients
- [ ] Handles disconnections gracefully
- [ ] Ping/Pong keeps connections alive
- [ ] Supports 10,000+ concurrent connections per instance
- [ ] Test coverage ≥ 80%
- [ ] Can run with docker-compose

---

## Phase 3 Integration Testing

### Test Environment Setup

```bash
# Start all services
cd /home/moersener/Hobby/all-chat/deployments
docker-compose up -d

# Services should be running:
# - postgres:5432
# - redis:6379
# - auth-service:8081
# - overlay-manager:8082
# - emote-service:8083
# - api-gateway:8080
# - twitch-listener:8085
# - message-processor:8087
```

### E2E Test Scenarios

#### Scenario 1: Single Overlay, Single Source (Twitch)

1. **Create Overlay**:
   ```bash
   TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login | jq -r '.token')

   OVERLAY_ID=$(curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name":"Test Overlay"}' \
     http://localhost:8080/api/v1/overlays | jq -r '.id')
   ```

2. **Add Twitch Source**:
   ```bash
   curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"platform":"twitch","channel_id":"xqc"}' \
     http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources
   ```

3. **Connect WebSocket**:
   ```bash
   websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN"
   ```

4. **Send Test Message in Twitch Chat** (xqc channel)

5. **Verify Message Appears in WebSocket**:
   - Should see JSON message with user, text, emotes
   - Latency should be < 500ms

#### Scenario 2: Multiple Overlays, Same Channel

1. Create 2 overlays for different users
2. Both add source: `twitch:xqc`
3. Both connect WebSocket
4. Send message in xqc channel
5. Verify both overlays receive the message

#### Scenario 3: Message with Emotes

1. Create overlay with Twitch source: `xqc`
2. Connect WebSocket
3. Send message: "Hello Kappa OMEGALUL"
4. Verify response includes:
   - Twitch native emote: Kappa
   - 7TV emote: OMEGALUL (if xqc has it)
   - Correct URLs for both emotes

#### Scenario 4: Reconnection Handling

1. Create overlay and connect WebSocket
2. Stop Twitch Listener service
3. Verify WebSocket stays connected (no messages)
4. Start Twitch Listener
5. Send message in Twitch chat
6. Verify message appears (after reconnection)

#### Scenario 5: Load Testing

```bash
# Use vegeta or hey for load testing
echo "GET http://localhost:8080/api/v1/overlays" | \
  vegeta attack -duration=30s -rate=100 -header "Authorization: Bearer $TOKEN" | \
  vegeta report
```

- 100 concurrent overlays
- 1000 messages/second throughput
- < 500ms latency (p95)
- No message loss

### Success Criteria

- [ ] All E2E scenarios pass
- [ ] Twitch IRC connection stable
- [ ] Messages flow from IRC → WebSocket in < 500ms
- [ ] Emote enrichment works correctly
- [ ] Multiple overlays receive same message
- [ ] Reconnection logic works
- [ ] Load test passes without errors
- [ ] No memory leaks under load

---

## Docker Compose Updates

### New Services

```yaml
# services/twitch-listener
twitch-listener:
  build:
    context: ../
    dockerfile: services/twitch-listener/Dockerfile
  container_name: allchat-twitch-listener
  environment:
    - PORT=8085
    - LOG_LEVEL=info
    - TWITCH_BOT_USERNAME=${TWITCH_BOT_USERNAME}
    - TWITCH_BOT_OAUTH=${TWITCH_BOT_OAUTH}
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - DATABASE_HOST=postgres
    - DATABASE_PORT=5432
    - DATABASE_NAME=allchat
    - DATABASE_USER=allchat
    - DATABASE_PASSWORD=allchat_dev_password
  ports:
    - "8085:8085"
  depends_on:
    - redis
    - postgres
  networks:
    - allchat
  restart: unless-stopped

# services/message-processor
message-processor:
  build:
    context: ../
    dockerfile: services/message-processor/Dockerfile
  container_name: allchat-message-processor
  environment:
    - PORT=8087
    - LOG_LEVEL=info
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - DATABASE_HOST=postgres
    - DATABASE_PORT=5432
    - DATABASE_NAME=allchat
    - DATABASE_USER=allchat
    - DATABASE_PASSWORD=allchat_dev_password
    - EMOTE_SERVICE_URL=http://emote-service:8083
  ports:
    - "8087:8087"
  depends_on:
    - redis
    - postgres
    - emote-service
  networks:
    - allchat
  restart: unless-stopped
```

---

## Makefile Updates

```makefile
# Build Phase 3 services
.PHONY: build-twitch-listener
build-twitch-listener:
	cd services/twitch-listener && go build -o twitch-listener ./cmd

.PHONY: build-message-processor
build-message-processor:
	cd services/message-processor && go build -o message-processor ./cmd

# Test Phase 3 services
.PHONY: test-twitch-listener
test-twitch-listener:
	cd services/twitch-listener && go test -v -cover ./...

.PHONY: test-message-processor
test-message-processor:
	cd services/message-processor && go test -v -cover ./...

# Run Phase 3 services locally
.PHONY: run-twitch-listener
run-twitch-listener:
	cd services/twitch-listener && go run ./cmd

.PHONY: run-message-processor
run-message-processor:
	cd services/message-processor && go run ./cmd
```

---

## Timeline

| Week | Days | Tasks | Status |
|------|------|-------|--------|
| **Week 1** | Mon-Wed | Twitch Listener: IRC + Parsing | ⏳ |
| | Thu-Fri | Twitch Listener: Channels + Publishing | ⏳ |
| **Week 2** | Mon-Tue | Message Processor: Consumer + Normalizer | ⏳ |
| | Wed-Thu | Message Processor: Enricher + Router | ⏳ |
| | Fri | Message Processor: Testing | ⏳ |
| **Week 3** | Mon-Tue | API Gateway: WebSocket connection | ⏳ |
| | Wed-Thu | API Gateway: Pub/Sub integration | ⏳ |
| | Fri | Integration testing | ⏳ |
| **Week 4** | Mon-Wed | E2E testing, bug fixes | ⏳ |
| | Thu-Fri | Documentation, cleanup | ⏳ |

---

## Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **Twitch IRC rate limits** | Medium | High | Implement rate limiting, multiple connections if needed |
| **Redis Streams backlog** | Medium | Medium | Monitor XPENDING, scale processors horizontally |
| **WebSocket connection limits** | Low | Medium | Use connection pooling, test with 10K+ connections |
| **Emote Service latency** | Medium | Low | Cache emote responses, make enrichment optional |
| **Database query performance** | Medium | Medium | Index overlay_chat_sources on (platform, channel_id) |

---

## Definition of Done

Phase 3 is complete when:

- [ ] Twitch Listener deployed and tested
- [ ] Message Processor deployed and tested
- [ ] API Gateway WebSocket deployed and tested
- [ ] All unit tests passing (85%+ coverage)
- [ ] All integration tests passing
- [ ] E2E test scenarios verified
- [ ] Docker Compose updated with new services
- [ ] Documentation updated (README, API docs)
- [ ] Load testing shows acceptable performance
- [ ] Ready to start Phase 4 (YouTube Integration)

---

**Next Phase**: [Phase 4: YouTube Integration](./PHASE_4_PLAN.md) (YouTube Listener + Source Manager)
