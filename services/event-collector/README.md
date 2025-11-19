# Event Collector Service

The Event Collector service collects streaming events (follows, subscriptions, bits, raids, etc.) from all platforms in real-time and stores them in a unified format for credit roll generation.

## Overview

**Port**: 8090
**Purpose**: Real-time event collection and normalization
**Dependencies**: PostgreSQL, Redis

## Features

- ✅ **Twitch EventSub WebSocket** - Real-time event collection
  - Follows, Subscriptions, Gift Subs, Bits, Raids
  - Stream online/offline detection
  - Automatic reconnection
- ⏳ **YouTube Event Extraction** - Integration with YouTube Listener (planned)
- ⏳ **Kick Event Listener** - Integration with Kick Listener (planned)
- ⏳ **TikTok Webhook Handler** - TikTok Events API (planned)
- ✅ **Event Normalization** - Unified format across platforms
- ✅ **Real-time Statistics** - Session stats updated live
- ✅ **RESTful API** - Query events and sessions

## Architecture

```
Platform Events → EventSub/WebSocket → Event Collector
                                            ↓
                                      Normalize Event
                                            ↓
                                   Store in stream_events
                                            ↓
                                   Update session.stats
```

## Data Models

### StreamEvent (Unified Format)

All platform events are normalized to this structure:

```json
{
  "id": "uuid",
  "stream_session_id": "uuid",
  "user_id": "uuid",
  "platform": "twitch|youtube|kick|tiktok",
  "event_type": "follow|sub|bits|raid|gift_sub|super_chat|chatter|...",
  "event_subtype": "new_sub|resub|tier_1|tier_2|tier_3",
  "platform_user": {
    "id": "platform-user-id",
    "username": "username",
    "display_name": "Display Name",
    "avatar_url": "https://..."
  },
  "metadata": {
    "amount": 500,
    "tier": "1000",
    "months": 12,
    "message": "Great stream!"
  },
  "occurred_at": "2025-11-19T12:34:56Z",
  "created_at": "2025-11-19T12:34:57Z"
}
```

### StreamSession

Tracks individual streaming sessions:

```json
{
  "id": "uuid",
  "user_id": "uuid",
  "title": "Stream title",
  "started_at": "2025-11-19T18:00:00Z",
  "ended_at": "2025-11-19T21:30:00Z",
  "status": "live|ended|archived",
  "platform_info": {
    "twitch": {"channel_id": "12345", "game": "Just Chatting"}
  },
  "stats": {
    "total_events": 125,
    "followers": 42,
    "subscribers": 15,
    "bits_total": 1250,
    "unique_chatters": 87
  }
}
```

## API Endpoints

### Health Checks

```
GET /health/live   - Liveness probe (always returns 200)
GET /health/ready  - Readiness probe (checks DB + Redis)
```

### Events API

```
GET /api/v1/sessions/:id/events
  - Get all events for a session
  - Query params: ?type=follow (filter by event type)

GET /api/v1/sessions/:id/stats
  - Get aggregated statistics for a session

GET /api/v1/users/:id/sessions
  - Get all sessions for a user
  - Query params: ?limit=10

GET /api/v1/users/:id/sessions/active
  - Get current active session for a user
```

## Twitch EventSub Integration

### How It Works

1. **WebSocket Connection**: Connect to `wss://eventsub.wss.twitch.tv/ws`
2. **Welcome Message**: Receive session ID
3. **Subscribe to Events**: Use Helix API to subscribe to event types
4. **Receive Events**: Process notifications in real-time
5. **Auto-Reconnect**: Handle disconnections gracefully

### Supported Event Types

| EventSub Type | Unified Event Type | Description |
|---------------|-------------------|-------------|
| `channel.follow` | `follow` | New follower |
| `channel.subscribe` | `sub` | New subscription |
| `channel.subscription.message` | `sub` (resub) | Resub with message |
| `channel.subscription.gift` | `gift_sub` | Gifted subscriptions |
| `channel.cheer` | `bits` | Bits/cheers |
| `channel.raid` | `raid` | Incoming raid |
| `stream.online` | (creates session) | Stream starts |
| `stream.offline` | (ends session) | Stream ends |

### Required OAuth Scopes (Twitch)

```
channel:read:subscriptions      # Read subscription events
bits:read                       # Read bits events
moderator:read:followers        # Read follower information (v2)
```

**Note**: User must grant these scopes in addition to existing chat scopes.

## Environment Variables

```bash
# Service configuration
PORT=8090

# Database
DATABASE_URL=postgresql://allchat:allchat_dev_password@localhost:5432/allchat
DATABASE_HOST=localhost
DATABASE_PORT=5432

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Twitch EventSub
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret
TWITCH_ACCESS_TOKEN=your_app_access_token  # App token for EventSub subscriptions

# Future platforms
TIKTOK_CLIENT_KEY=...
TIKTOK_CLIENT_SECRET=...
```

## Development

### Build

```bash
cd services/event-collector
go build -o event-collector ./cmd/main.go
```

### Run Locally

```bash
# Ensure PostgreSQL and Redis are running
make docker-up

# Run service
cd services/event-collector
go run ./cmd/main.go
```

### Test

```bash
go test ./...
```

### Docker

```bash
# Build image
docker build -t event-collector:latest -f services/event-collector/Dockerfile .

# Run container
docker run -p 8090:8090 \
  -e DATABASE_URL=postgresql://... \
  -e REDIS_HOST=redis \
  -e TWITCH_CLIENT_ID=... \
  event-collector:latest
```

## Database Schema

See `migrations/009_credit_roll_support.sql` for complete schema.

### Key Tables

- **stream_sessions**: Individual streaming sessions
- **stream_events**: All platform events with unified format
- **clips**: Platform clips for background videos
- **user_credit_roll_settings**: User configuration

### Indexes

Optimized for:
- Querying events by session and type
- Finding active sessions by user
- Time-range queries on events
- Leaderboard queries (top bits, top chatters)

## Usage Example

### Query Events for Active Session

```bash
# Get user's active session
GET /api/v1/users/{user_id}/sessions/active
# Response: {"id": "session-uuid", "status": "live", ...}

# Get events for that session
GET /api/v1/sessions/{session_id}/events
# Response: {"events": [...]}

# Get just followers
GET /api/v1/sessions/{session_id}/events?type=follow
# Response: {"events": [{event_type: "follow", ...}, ...]}

# Get session stats
GET /api/v1/sessions/{session_id}/stats
# Response: {"stats": {"followers": 42, "subscribers": 15, ...}}
```

## Integration with Credit Roll Feature

This service provides the **event data** for credit rolls:

1. **During Stream**: Events automatically collected and stored
2. **End of Stream**: User switches to "Ending Soon" scene in OBS
3. **Overlay Loads**: Credit Roll Generator queries this service for TODAY'S events
4. **Display**: Events shown in Hollywood-style credits

See `docs/CREDIT_ROLL_ROADMAP.md` for complete feature documentation.

## Monitoring

### Health Checks

```bash
# Kubernetes liveness probe
curl http://localhost:8090/health/live

# Kubernetes readiness probe
curl http://localhost:8090/health/ready
```

### Metrics (TODO)

- Event ingestion rate (events/sec)
- Event processing latency (ms)
- WebSocket connection status
- Failed event count

## Troubleshooting

### EventSub Connection Fails

**Issue**: Cannot connect to Twitch EventSub WebSocket

**Solutions**:
- Check internet connectivity
- Verify Twitch API is not experiencing outages
- Check logs for specific error messages

### Events Not Being Stored

**Issue**: EventSub receiving events but not storing in database

**Solutions**:
- Check database connection in `/health/ready`
- Verify user has active session (stream must be online)
- Check logs for normalization errors

### Missing Events

**Issue**: Some events not appearing in credit roll

**Solutions**:
- Verify broadcaster has granted required OAuth scopes
- Check EventSub subscription status (may have been revoked)
- Verify event occurred during active session (between started_at and ended_at)

## Future Enhancements

- [ ] YouTube Live Chat event extraction integration
- [ ] Kick Pusher WebSocket event listener
- [ ] TikTok Events API webhook handler
- [ ] Batch event insertion for high-volume streams
- [ ] Redis-based event buffer for resilience
- [ ] Prometheus metrics endpoint
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Event replay/backfill functionality

## Related Services

- **Twitch Listener** (`services/twitch-listener/`) - Chat messages only
- **YouTube Listener** (`services/youtube-listener/`) - Chat messages + Super Chats
- **Kick Listener** (`services/kick-listener/`) - Chat messages
- **Credit Roll Generator** (`services/credit-roll-generator/`) - Generates timelines (planned)
- **Clip Manager** (`services/clip-manager/`) - Fetches and ranks clips (planned)

## License

See root LICENSE file.
