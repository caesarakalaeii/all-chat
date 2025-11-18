# Phase 4: YouTube Integration - Detailed Plan

**Version**: 1.0
**Created**: 2025-11-13
**Duration**: 2-3 weeks (Dec 23 - Jan 12)
**Priority**: P1 (Second Platform - Multi-platform Support)

---

## Overview

Phase 4 adds YouTube Live Chat support, enabling multi-platform chat aggregation. This phase builds on Phase 3's Twitch implementation to support a second major streaming platform, with the Message Processor handling messages from both platforms.

**Key Deliverables**:
1. **YouTube Listener** - Poll YouTube Live Chat API, parse messages
2. **Source Manager** - Orchestrate multiple platform listeners, leader election
3. **Message Processor Enhancement** - Handle YouTube message format
4. **Multi-platform Testing** - Twitch + YouTube in single overlay

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│           Source Manager (Port 8088)         │
│  - Leader election per live stream              │
│  - Active source registry                       │
│  - Listener coordination                        │
└──────────────┬──────────────────────────────────┘
               │
      ┌────────┴────────┐
      │                 │
      ▼                 ▼
┌──────────────┐  ┌──────────────┐
│   Twitch     │  │   YouTube    │
│  Listener    │  │  Listener    │
│ (Port 8085)  │  │ (Port 8086)  │
│              │  │              │
│ IRC Connect  │  │ API Polling  │
└──────┬───────┘  └──────┬───────┘
       │                 │
       └────────┬────────┘
                ▼
         ┌──────────────┐
         │Redis Streams │
         │  chat:raw    │
         └──────┬───────┘
                ▼
         ┌──────────────────┐
         │Message Processor │
         │  - Twitch norm   │
         │  - YouTube norm  │
         │  - Emote enrich  │
         └──────┬───────────┘
                ▼
         ┌──────────────┐
         │Redis Pub/Sub │
         │ overlay:{id} │
         └──────────────┘
```

### Multi-Platform Message Flow

1. **Database** → Source Manager:
   - Poll for active overlay sources (platform=youtube)
   - Detect new/removed YouTube streams

2. **Source Manager** → YouTube Listener:
   - Leader election (one poller per stream)
   - Assign live stream IDs to poll
   - Health monitoring

3. **YouTube Listener** → YouTube API:
   - Poll Live Chat API (pollingIntervalMillis)
   - OAuth 2.0 authentication
   - Quota management (10,000 units/day)

4. **YouTube Listener** → Redis Streams:
   - Publish to `chat:raw` (same as Twitch)
   - Platform field = "youtube"

5. **Message Processor** → Redis Streams:
   - Consume from `chat:raw` (existing)
   - **NEW**: Detect platform and route to correct normalizer
   - YouTube normalizer converts to unified format

6. **Message Processor** → Redis Pub/Sub:
   - Publish to `overlay:{id}` (existing)
   - Multiple platforms can target same overlay

---

## Service 1: YouTube Listener

### Purpose
Poll YouTube Live Chat API for configured live streams, parse messages, and publish raw messages to Redis Streams (same stream as Twitch).

### Architecture

```
┌─────────────────────────────────┐
│   YouTube Listener              │
│   ┌─────────────────────────┐   │
│   │ OAuth Manager           │   │
│   │ - OAuth 2.0 flow        │   │
│   │ - Token refresh         │   │
│   │ - Per-stream tokens     │   │
│   └──────────┬──────────────┘   │
│              │                   │
│   ┌──────────▼──────────────┐   │
│   │ Stream Manager          │   │
│   │ - Active streams        │   │
│   │ - Leader election       │   │
│   │ - Stream lifecycle      │   │
│   └──────────┬──────────────┘   │
│              │                   │
│   ┌──────────▼──────────────┐   │
│   │ Polling Service         │   │
│   │ - Adaptive interval     │   │
│   │ - Quota tracking        │   │
│   │ - API client            │   │
│   └──────────┬──────────────┘   │
│              │                   │
│   ┌──────────▼──────────────┐   │
│   │ Message Handler         │   │
│   │ - Parse API response    │   │
│   │ - Extract user info     │   │
│   │ - Publish to stream     │   │
│   └─────────────────────────┘   │
└─────────────────────────────────┘
```

### Data Model

```go
// RawChatMessage (same struct as Twitch, different platform)
type RawChatMessage struct {
    MessageID string    `json:"message_id"`  // UUID
    Platform  string    `json:"platform"`    // "youtube"
    ChannelID string    `json:"channel_id"`  // YouTube channel ID
    StreamID  string    `json:"stream_id"`   // YouTube live stream ID
    UserID    string    `json:"user_id"`     // YouTube user ID
    Username  string    `json:"username"`    // Display name
    Text      string    `json:"text"`        // Message text
    Timestamp time.Time `json:"timestamp"`   // UTC
    Tags      map[string]string `json:"tags"` // YouTube-specific metadata
}
```

### YouTube API Integration

**API Endpoints**:
- `GET /youtube/v3/liveChat/messages` - Fetch messages
- `GET /youtube/v3/videos?part=liveStreamingDetails` - Get live stream info

**Authentication**:
- OAuth 2.0 with scope: `https://www.googleapis.com/auth/youtube.readonly`
- Per-user credentials (streamers must authorize)

**Rate Limits & Quota**:
- **Quota**: 10,000 units/day per project (default)
- **liveChatMessages.list**: 5 units per request
- **Requests/day**: ~2,000 requests max
- **Polling Interval**: Recommended by API (2-5 seconds typical)
- **Mitigation**: Request quota increase to 1,000,000 units/day (typical for production)

**API Response Format**:
```json
{
  "items": [
    {
      "id": "message-id",
      "snippet": {
        "type": "textMessageEvent",
        "liveChatId": "live-chat-id",
        "authorChannelId": "channel-id",
        "publishedAt": "2025-11-13T10:00:00Z",
        "displayMessage": "Hello world!",
        "textMessageDetails": {
          "messageText": "Hello world!"
        }
      },
      "authorDetails": {
        "channelId": "channel-id",
        "channelUrl": "https://youtube.com/channel/...",
        "displayName": "Viewer123",
        "profileImageUrl": "https://...",
        "isVerified": false,
        "isChatOwner": false,
        "isChatSponsor": true,
        "isChatModerator": false
      }
    }
  ],
  "pollingIntervalMillis": 2000,
  "pageInfo": {
    "totalResults": 1,
    "resultsPerPage": 200
  }
}
```

### Tags to Extract

```go
tags := map[string]string{
    "channel_id":       "UCxxxxxx",
    "channel_url":      "https://youtube.com/channel/...",
    "profile_image":    "https://...",
    "is_verified":      "false",
    "is_owner":         "false",
    "is_sponsor":       "true",  // Channel member
    "is_moderator":     "false",
    "super_chat":       "0",     // Super Chat amount (cents)
    "super_sticker":    "0",     // Super Sticker amount (cents)
}
```

### Configuration

```bash
# Environment Variables
YOUTUBE_API_KEY=AIzaSyXXXXXXXXXXXX          # For public API calls (optional)
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx
YOUTUBE_REDIRECT_URL=http://localhost:8080/api/v1/auth/youtube/callback
REDIS_HOST=localhost
REDIS_PORT=6379
DATABASE_HOST=localhost
DATABASE_PORT=5432
LOG_LEVEL=info
PORT=8086
POLLING_INTERVAL_MS=2000                      # Default if API doesn't specify
QUOTA_LIMIT_DAILY=10000                       # Track and alert
```

### File Structure

```
services/youtube-listener/
├── cmd/
│   └── main.go                      # Entry point
├── handlers/
│   ├── health.go                    # Health checks
│   ├── health_test.go
│   ├── oauth.go                     # OAuth callback handler
│   └── oauth_test.go
├── oauth/
│   ├── manager.go                   # OAuth token management
│   ├── manager_test.go
│   ├── store.go                     # Store tokens in database
│   └── store_test.go
├── api/
│   ├── client.go                    # YouTube API client
│   ├── client_test.go
│   ├── parser.go                    # Parse API responses
│   ├── parser_test.go
│   └── mock.go                      # Mock API for testing
├── streams/
│   ├── manager.go                   # Active stream management
│   ├── manager_test.go
│   ├── poller.go                    # Polling service
│   ├── poller_test.go
│   └── repository.go                # DB queries for streams
├── quota/
│   ├── tracker.go                   # Track API quota usage
│   └── tracker_test.go
├── publisher/
│   ├── stream_publisher.go          # Redis Streams publisher (same as Twitch)
│   └── stream_publisher_test.go
├── models/
│   ├── raw_message.go               # RawChatMessage struct (shared)
│   ├── stream.go                    # YouTube stream metadata
│   └── models_test.go
├── go.mod
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Days 1-2: OAuth & API Client
- [ ] Create `oauth/manager.go` with OAuth 2.0 flow
- [ ] Implement token storage in database
- [ ] Create `api/client.go` for YouTube API
- [ ] Implement `liveChatMessages.list` API call
- [ ] Write tests with mock HTTP server
- [ ] Tests pass ✅

#### Days 3-4: Stream Management
- [ ] Create `streams/manager.go`
- [ ] Query database for active YouTube sources
- [ ] Resolve channel ID → live stream ID (via API)
- [ ] Detect when streams go live/offline
- [ ] Periodic sync with database (every 30s)
- [ ] Tests pass ✅

#### Days 5-6: Polling Service
- [ ] Create `streams/poller.go`
- [ ] Implement adaptive polling (use API's pollingIntervalMillis)
- [ ] Create `quota/tracker.go` for quota monitoring
- [ ] Alert when quota reaches 80% usage
- [ ] Parse API responses with `api/parser.go`
- [ ] Tests pass ✅

#### Days 7-8: Publishing & Integration
- [ ] Reuse `publisher/stream_publisher.go` from Twitch Listener
- [ ] Publish to same Redis Stream: `chat:raw`
- [ ] Wire everything in main.go
- [ ] Integration tests with Redis + Postgres
- [ ] All tests pass ✅

#### Days 9-10: Error Handling & Testing
- [ ] Handle API errors (quota exceeded, stream ended, auth failure)
- [ ] Implement backoff on rate limit errors
- [ ] Manual testing with real YouTube live stream
- [ ] End-to-end test with Message Processor
- [ ] All tests pass ✅

### Success Criteria

- [ ] OAuth 2.0 flow works for YouTube
- [ ] Can poll 50+ concurrent live streams
- [ ] Respects YouTube API quota limits
- [ ] Parses messages correctly with user metadata
- [ ] Publishes to Redis Streams (same as Twitch)
- [ ] Handles stream end gracefully (stop polling)
- [ ] Handles OAuth token expiration (refresh)
- [ ] Quota tracking and alerting works
- [ ] Test coverage ≥ 85%
- [ ] Docker build succeeds
- [ ] Can run with docker-compose

---

## Service 2: Source Manager

### Purpose
Orchestrate multiple platform listeners (Twitch, YouTube, future platforms), perform leader election for YouTube streams, maintain active source registry, and coordinate listener health.

### Architecture

```
┌─────────────────────────────────┐
│   Source Manager             │
│   ┌─────────────────────────┐   │
│   │ Active Source Registry  │   │
│   │ - All active sources    │   │
│   │ - Per-platform state    │   │
│   │ - Health status         │   │
│   └──────────┬──────────────┘   │
│              │                   │
│   ┌──────────▼──────────────┐   │
│   │ Leader Election         │   │
│   │ - Redis-based           │   │
│   │ - Per-stream locks      │   │
│   │ - TTL + heartbeat       │   │
│   └──────────┬──────────────┘   │
│              │                   │
│   ┌──────────▼──────────────┐   │
│   │ Listener Coordinator    │   │
│   │ - Assign work           │   │
│   │ - Health checks         │   │
│   │ - Failover              │   │
│   └─────────────────────────┘   │
└─────────────────────────────────┘
```

### Why Source Manager?

**Problem**: YouTube API polling requires exactly one poller per live stream to avoid:
1. Duplicate messages (multiple pollers)
2. Quota waste (redundant API calls)
3. Rate limit violations

**Solution**: Source Manager provides:
1. **Leader Election**: Ensure one YouTube Listener instance polls each stream
2. **Active Registry**: Track which listeners are polling which streams
3. **Health Monitoring**: Detect failed listeners and reassign work
4. **Coordination**: Future platforms (Kick, TikTok) may have similar requirements

### Leader Election Strategy

**Redis-based distributed locks**:

```bash
# Acquire lock for stream
SET leader:youtube:stream:{stream_id} {instance_id} NX EX 10

# Heartbeat every 5 seconds
SET leader:youtube:stream:{stream_id} {instance_id} EX 10

# Check if leader
GET leader:youtube:stream:{stream_id} == {instance_id}
```

**Leadership Lifecycle**:
1. YouTube Listener queries Source Manager for assigned streams
2. Source Manager tries to acquire lock for stream
3. If acquired → Listener becomes leader, starts polling
4. Leader sends heartbeat every 5 seconds
5. If heartbeat stops → lock expires, another listener can take over

### Data Model

```go
// ActiveSource represents a source that should be polled
type ActiveSource struct {
    ID        string    `json:"id"`         // overlay_chat_source.id
    OverlayID string    `json:"overlay_id"` // overlay ID
    Platform  string    `json:"platform"`   // "twitch", "youtube"
    ChannelID string    `json:"channel_id"` // Platform-specific channel
    StreamID  string    `json:"stream_id"`  // YouTube live stream ID (empty for Twitch)
    IsActive  bool      `json:"is_active"`  // Should be polled
    UpdatedAt time.Time `json:"updated_at"`
}

// LeadershipStatus represents leadership for a stream
type LeadershipStatus struct {
    StreamID   string    `json:"stream_id"`   // YouTube stream ID
    LeaderID   string    `json:"leader_id"`   // Listener instance ID
    AcquiredAt time.Time `json:"acquired_at"`
    ExpiresAt  time.Time `json:"expires_at"`
}
```

### API Endpoints

```
GET  /sources?platform=youtube    # List active sources for platform
POST /sources/:id/claim           # Claim leadership for source
POST /sources/:id/heartbeat       # Renew leadership
DELETE /sources/:id/release       # Release leadership
GET  /health/live                 # Liveness probe
GET  /health/ready                # Readiness probe
GET  /status                      # Current leadership state
```

### File Structure

```
services/source-manager/
├── cmd/
│   └── main.go                      # Entry point
├── handlers/
│   ├── sources.go                   # Source API handlers
│   ├── sources_test.go
│   ├── health.go                    # Health checks
│   └── health_test.go
├── registry/
│   ├── registry.go                  # Active source registry
│   ├── registry_test.go
│   └── repository.go                # DB queries
├── election/
│   ├── leader.go                    # Leader election with Redis
│   ├── leader_test.go
│   └── heartbeat.go                 # Heartbeat management
├── coordinator/
│   ├── coordinator.go               # Assign work to listeners
│   ├── coordinator_test.go
│   └── health_monitor.go            # Monitor listener health
├── models/
│   ├── source.go                    # ActiveSource struct
│   ├── leadership.go                # LeadershipStatus struct
│   └── models_test.go
├── go.mod
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Days 1-2: Active Source Registry
- [ ] Create `registry/registry.go`
- [ ] Query database for active overlay_chat_sources
- [ ] Periodic sync with database (every 30s)
- [ ] API endpoint: `GET /sources?platform=youtube`
- [ ] Tests with mock database
- [ ] Tests pass ✅

#### Days 3-4: Leader Election
- [ ] Create `election/leader.go`
- [ ] Implement Redis-based distributed locks
- [ ] API endpoint: `POST /sources/:id/claim`
- [ ] API endpoint: `POST /sources/:id/heartbeat`
- [ ] API endpoint: `DELETE /sources/:id/release`
- [ ] Tests with mock Redis
- [ ] Tests pass ✅

#### Days 5-6: Coordinator & Health Monitoring
- [ ] Create `coordinator/coordinator.go`
- [ ] Detect missing heartbeats (expired locks)
- [ ] Reassign work to healthy listeners
- [ ] Create `coordinator/health_monitor.go`
- [ ] API endpoint: `GET /status`
- [ ] Tests pass ✅

#### Day 7: Integration & Testing
- [ ] Wire everything in main.go
- [ ] Integration tests with Redis + Postgres
- [ ] Manual testing with YouTube Listener
- [ ] All tests pass ✅

### Success Criteria

- [ ] Maintains active source registry from database
- [ ] Provides leader election for YouTube streams
- [ ] Only one listener polls each stream (verified)
- [ ] Heartbeat mechanism prevents stale locks
- [ ] Detects and reassigns failed listeners
- [ ] API endpoints work correctly
- [ ] Test coverage ≥ 80%
- [ ] Docker build succeeds
- [ ] Can run with docker-compose

---

## Enhancement: Message Processor (YouTube Support)

### Current State (Phase 3)
Message Processor currently handles Twitch messages with a Twitch-specific normalizer.

### Required Changes

#### 1. Platform Detection & Routing

**Before**:
```go
// Hardcoded Twitch normalizer
func (p *Processor) Process(raw RawChatMessage) {
    unified := p.twitchNormalizer.Normalize(raw)
    // ...
}
```

**After**:
```go
// Dynamic normalizer selection
func (p *Processor) Process(raw RawChatMessage) {
    var normalizer Normalizer
    switch raw.Platform {
    case "twitch":
        normalizer = p.twitchNormalizer
    case "youtube":
        normalizer = p.youtubeNormalizer
    default:
        log.Warn("Unknown platform", zap.String("platform", raw.Platform))
        return
    }
    unified := normalizer.Normalize(raw)
    // ...
}
```

#### 2. YouTube Normalizer

```go
// normalizer/youtube_normalizer.go
type YouTubeNormalizer struct{}

func (n *YouTubeNormalizer) Normalize(raw RawChatMessage) UnifiedChatMessage {
    return UnifiedChatMessage{
        ID:          uuid.New().String(),
        OverlayID:   "", // Populated by router
        Platform:    "youtube",
        ChannelID:   raw.ChannelID,
        ChannelName: raw.Tags["display_name"],
        User: UserInfo{
            ID:          raw.UserID,
            Username:    raw.Username,
            DisplayName: raw.Username,
            AvatarURL:   raw.Tags["profile_image"],
            Badges:      extractYouTubeBadges(raw.Tags),
            Color:       "", // YouTube doesn't have user colors
        },
        Message: MessageInfo{
            Text:   raw.Text,
            Emotes: []Emote{}, // Will be enriched later
        },
        Timestamp: raw.Timestamp,
        Metadata: map[string]interface{}{
            "is_verified":  raw.Tags["is_verified"] == "true",
            "is_owner":     raw.Tags["is_owner"] == "true",
            "is_sponsor":   raw.Tags["is_sponsor"] == "true",
            "is_moderator": raw.Tags["is_moderator"] == "true",
            "super_chat":   parseIntOrZero(raw.Tags["super_chat"]),
        },
    }
}

func extractYouTubeBadges(tags map[string]string) []string {
    badges := []string{}
    if tags["is_owner"] == "true" {
        badges = append(badges, "owner")
    }
    if tags["is_sponsor"] == "true" {
        badges = append(badges, "member")
    }
    if tags["is_moderator"] == "true" {
        badges = append(badges, "moderator")
    }
    if tags["is_verified"] == "true" {
        badges = append(badges, "verified")
    }
    return badges
}
```

#### 3. File Changes

**New Files**:
- `services/message-processor/normalizer/youtube_normalizer.go`
- `services/message-processor/normalizer/youtube_normalizer_test.go`

**Modified Files**:
- `services/message-processor/cmd/main.go` - Initialize YouTube normalizer
- `services/message-processor/normalizer/normalizer.go` - Add Normalizer interface
- Update tests to cover both platforms

### Implementation Checklist

#### Days 1-2: YouTube Normalizer
- [ ] Create `normalizer/youtube_normalizer.go`
- [ ] Implement YouTube-specific badge extraction
- [ ] Handle Super Chat metadata
- [ ] Write comprehensive tests
- [ ] Tests pass ✅

#### Day 3: Platform Routing
- [ ] Update `cmd/main.go` to initialize both normalizers
- [ ] Add platform detection in processor
- [ ] Route to correct normalizer based on platform
- [ ] Update tests
- [ ] Tests pass ✅

### Success Criteria

- [ ] Handles both Twitch and YouTube messages
- [ ] YouTube messages normalized correctly
- [ ] Existing Twitch functionality not broken
- [ ] Test coverage maintained ≥ 85%
- [ ] Integration tests with both platforms pass

---

## Phase 4 Integration Testing

### Test Environment Setup

```bash
# Start all services (including Phase 4)
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
# - youtube-listener:8086       # NEW
# - message-processor:8087
# - source-manager:8088       # NEW
```

### E2E Test Scenarios

#### Scenario 1: Single Overlay, YouTube Only

1. **Create Overlay**:
   ```bash
   TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/twitch/login | jq -r '.token')

   OVERLAY_ID=$(curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name":"YouTube Test Overlay"}' \
     http://localhost:8080/api/v1/overlays | jq -r '.id')
   ```

2. **Complete YouTube OAuth** (requires manual browser flow):
   ```bash
   # User visits OAuth URL, authorizes, gets redirected
   # Store YouTube credentials in database
   ```

3. **Add YouTube Source**:
   ```bash
   curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"platform":"youtube","channel_id":"UC-channel-id"}' \
     http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources
   ```

4. **Connect WebSocket**:
   ```bash
   websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN"
   ```

5. **Send Test Message in YouTube Live Chat**

6. **Verify Message Appears in WebSocket**:
   - Should see JSON message with YouTube user info
   - Badges include "member", "moderator", "owner" if applicable
   - No color field (YouTube doesn't support)

#### Scenario 2: Multi-Platform Overlay (Twitch + YouTube)

1. Create overlay
2. Add Twitch source: `{"platform":"twitch","channel_id":"xqc"}`
3. Add YouTube source: `{"platform":"youtube","channel_id":"UC-channel-id"}`
4. Connect WebSocket
5. Send messages in both Twitch and YouTube chats
6. **Verify**:
   - Messages from both platforms appear
   - Platform field correctly identifies source
   - Emotes enriched for both platforms
   - Messages interleaved by timestamp

#### Scenario 3: Leader Election (Multiple YouTube Listener Instances)

1. Start 3 YouTube Listener instances
2. Create overlay with YouTube source
3. Check Source Manager status: `GET http://localhost:8088/status`
4. **Verify**:
   - Only one instance is leader for stream
   - Leader sends heartbeats
5. Kill leader instance
6. Wait 10 seconds
7. Check status again
8. **Verify**:
   - New leader elected
   - Polling continues without message loss

#### Scenario 4: YouTube Quota Monitoring

1. Set quota limit to 100 units (for testing)
2. Add 20+ YouTube sources (5 units per poll)
3. Monitor logs
4. **Verify**:
   - Quota tracker logs usage
   - Alert triggers at 80% usage
   - Polling stops before exceeding quota

#### Scenario 5: OAuth Token Refresh

1. Create YouTube source with valid OAuth
2. Manually set token expiry to 5 minutes in database
3. Wait for token to expire
4. Send message in YouTube chat
5. **Verify**:
   - YouTube Listener detects expired token
   - Automatically refreshes token
   - Polling resumes
   - Message appears in overlay

### Success Criteria

- [ ] All E2E scenarios pass
- [ ] YouTube API integration stable
- [ ] Messages flow from YouTube API → WebSocket in < 2s (polling interval dependent)
- [ ] Multi-platform overlays work correctly
- [ ] Leader election prevents duplicate polling
- [ ] Quota tracking and alerting works
- [ ] OAuth token refresh works automatically
- [ ] No message loss under normal conditions
- [ ] Load test: 50 YouTube streams + 100 Twitch channels simultaneously

---

## Docker Compose Updates

### New Services

```yaml
# services/youtube-listener
youtube-listener:
  build:
    context: ../
    dockerfile: services/youtube-listener/Dockerfile
  container_name: allchat-youtube-listener
  environment:
    - PORT=8086
    - LOG_LEVEL=info
    - YOUTUBE_CLIENT_ID=${YOUTUBE_CLIENT_ID}
    - YOUTUBE_CLIENT_SECRET=${YOUTUBE_CLIENT_SECRET}
    - YOUTUBE_REDIRECT_URL=${YOUTUBE_REDIRECT_URL}
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - DATABASE_HOST=postgres
    - DATABASE_PORT=5432
    - DATABASE_NAME=allchat
    - DATABASE_USER=allchat
    - DATABASE_PASSWORD=allchat_dev_password
    - POLLING_INTERVAL_MS=2000
    - QUOTA_LIMIT_DAILY=10000
  ports:
    - "8086:8086"
  depends_on:
    - redis
    - postgres
    - source-manager
  networks:
    - allchat
  restart: unless-stopped

# services/source-manager
source-manager:
  build:
    context: ../
    dockerfile: services/source-manager/Dockerfile
  container_name: allchat-source-manager
  environment:
    - PORT=8088
    - LOG_LEVEL=info
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - DATABASE_HOST=postgres
    - DATABASE_PORT=5432
    - DATABASE_NAME=allchat
    - DATABASE_USER=allchat
    - DATABASE_PASSWORD=allchat_dev_password
  ports:
    - "8088:8088"
  depends_on:
    - redis
    - postgres
  networks:
    - allchat
  restart: unless-stopped
```

### Environment Variables (.env)

```bash
# YouTube OAuth (required for Phase 4)
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx
YOUTUBE_REDIRECT_URL=http://localhost:8080/api/v1/auth/youtube/callback
```

---

## Makefile Updates

```makefile
# Build Phase 4 services
.PHONY: build-youtube-listener
build-youtube-listener:
	cd services/youtube-listener && go build -o youtube-listener ./cmd

.PHONY: build-source-manager
build-source-manager:
	cd services/source-manager && go build -o source-manager ./cmd

# Test Phase 4 services
.PHONY: test-youtube-listener
test-youtube-listener:
	cd services/youtube-listener && go test -v -cover ./...

.PHONY: test-source-manager
test-source-manager:
	cd services/source-manager && go test -v -cover ./...

# Run Phase 4 services locally
.PHONY: run-youtube-listener
run-youtube-listener:
	cd services/youtube-listener && go run ./cmd

.PHONY: run-source-manager
run-source-manager:
	cd services/source-manager && go run ./cmd
```

---

## Database Schema Updates

### New Tables

```sql
-- Store YouTube OAuth tokens per user/channel
CREATE TABLE youtube_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id VARCHAR(255) NOT NULL,  -- YouTube channel ID
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, channel_id)
);

CREATE INDEX idx_youtube_oauth_user_id ON youtube_oauth_tokens(user_id);
CREATE INDEX idx_youtube_oauth_channel_id ON youtube_oauth_tokens(channel_id);

-- Track YouTube API quota usage
CREATE TABLE youtube_quota_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL,
    units_used INT NOT NULL DEFAULT 0,
    units_limit INT NOT NULL DEFAULT 10000,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(date)
);

CREATE INDEX idx_youtube_quota_date ON youtube_quota_usage(date);
```

### Migration

```sql
-- migrations/003_youtube_support.sql
BEGIN;

-- YouTube OAuth tokens
CREATE TABLE youtube_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, channel_id)
);

CREATE INDEX idx_youtube_oauth_user_id ON youtube_oauth_tokens(user_id);
CREATE INDEX idx_youtube_oauth_channel_id ON youtube_oauth_tokens(channel_id);

-- YouTube quota tracking
CREATE TABLE youtube_quota_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL,
    units_used INT NOT NULL DEFAULT 0,
    units_limit INT NOT NULL DEFAULT 10000,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(date)
);

CREATE INDEX idx_youtube_quota_date ON youtube_quota_usage(date);

-- Update supported_platforms table (if exists)
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('youtube', 'YouTube', true, true, '{
  "oauth_scopes": ["https://www.googleapis.com/auth/youtube.readonly"],
  "quota_limit": 10000
}'::jsonb)
ON CONFLICT (platform) DO UPDATE SET is_enabled = true;

COMMIT;
```

---

## Timeline

| Week | Days | Tasks | Status |
|------|------|-------|--------|
| **Week 1** | Mon-Wed | YouTube Listener: OAuth + API Client | ⏳ |
| | Thu-Fri | YouTube Listener: Stream Management + Polling | ⏳ |
| **Week 2** | Mon-Tue | YouTube Listener: Publishing + Testing | ⏳ |
| | Wed-Thu | Source Manager: Registry + Leader Election | ⏳ |
| | Fri | Source Manager: Coordinator | ⏳ |
| **Week 3** | Mon-Tue | Message Processor: YouTube Normalizer | ⏳ |
| | Wed-Thu | Integration testing (multi-platform) | ⏳ |
| | Fri | Documentation, cleanup | ⏳ |

---

## Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **YouTube API Quota** | High | High | Request quota increase (1M units/day), implement caching, adaptive polling |
| **OAuth Complexity** | Medium | Medium | Comprehensive tests, clear user documentation, token refresh automation |
| **Leader Election Bugs** | Medium | High | Extensive testing with multiple instances, Redis lock timeouts |
| **YouTube API Changes** | Low | High | Monitor API deprecation notices, version API client |
| **Live Stream Detection** | Medium | Medium | Periodic re-check if stream is live, handle edge cases |

---

## Definition of Done

Phase 4 is complete when:

- [ ] YouTube Listener deployed and tested
- [ ] Source Manager deployed and tested
- [ ] Message Processor handles YouTube messages
- [ ] OAuth flow works for YouTube
- [ ] Leader election prevents duplicate polling
- [ ] All unit tests passing (85%+ coverage)
- [ ] All integration tests passing
- [ ] E2E test scenarios verified (multi-platform)
- [ ] Docker Compose updated with new services
- [ ] Database migrations applied
- [ ] Documentation updated (README, API docs)
- [ ] Multi-platform testing shows no regressions
- [ ] Ready to start Phase 5 (Frontend)

---

## Success Metrics

- [ ] Can aggregate chat from Twitch + YouTube simultaneously
- [ ] Only one poller per YouTube stream (verified)
- [ ] YouTube messages normalized correctly
- [ ] OAuth token refresh works automatically
- [ ] Quota tracking prevents API overuse
- [ ] Latency < 2 seconds for YouTube (polling interval dependent)
- [ ] No message loss under normal load
- [ ] System handles 50 YouTube streams + 100 Twitch channels
- [ ] All tests pass (200+ tests total across all services)

---

**Next Phase**: [Phase 5: Frontend & User Experience](./PHASE_5_PLAN.md) (Svelte UI for multi-platform overlays)
