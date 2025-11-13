# YouTube Listener

YouTube Live Chat API polling service for All-Chat platform.

## Overview

The YouTube Listener service polls YouTube Live Chat API for configured channels, parses messages, and publishes them to Redis Streams for processing. It implements:

- YouTube OAuth 2.0 authentication per streamer
- Live stream discovery and monitoring
- Adaptive polling intervals (respects API's pollingIntervalMillis)
- API quota tracking and alerting
- Message normalization to unified format

## Architecture

```
YouTube Live Chat API
        ↓
  OAuth Manager
        ↓
   Stream Manager
    ↓        ↓
 Poller1  Poller2  (one per live stream)
        ↓
   API Client
        ↓
     Parser
        ↓
Redis Streams (chat:raw)
```

## Features

- **OAuth 2.0**: Per-user OAuth tokens stored in PostgreSQL
- **Live Stream Detection**: Automatically detects when channels go live
- **Adaptive Polling**: Uses API-recommended polling intervals (typically 2-5 seconds)
- **Quota Management**: Tracks daily YouTube API quota (default 10,000 units/day)
- **Health Checks**: Liveness, readiness, and status endpoints
- **Graceful Shutdown**: Stops all pollers cleanly

## Configuration

### Environment Variables

```bash
# YouTube OAuth (required)
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx
YOUTUBE_REDIRECT_URL=http://localhost:8080/api/v1/auth/youtube/callback

# Database (required)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password

# Redis (required)
REDIS_HOST=localhost
REDIS_PORT=6379

# Service configuration
PORT=8086
LOG_LEVEL=info
POLLING_INTERVAL_MS=2000          # Default if API doesn't specify
QUOTA_LIMIT_DAILY=10000           # Daily API quota limit
```

## Database Schema

Requires the following tables (created by migration `003_youtube_support.sql`):

```sql
-- YouTube OAuth tokens
CREATE TABLE youtube_oauth_tokens (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    channel_id VARCHAR(255),
    access_token TEXT,
    refresh_token TEXT,
    token_type VARCHAR(50),
    expiry TIMESTAMP,
    ...
    UNIQUE(user_id, channel_id)
);

-- YouTube quota tracking
CREATE TABLE youtube_quota_usage (
    id UUID PRIMARY KEY,
    date DATE UNIQUE,
    units_used INT DEFAULT 0,
    units_limit INT DEFAULT 10000,
    ...
);
```

## API Endpoints

### Health Checks

```bash
# Liveness probe (always returns 200 if running)
GET /health/live

# Readiness probe (checks Redis connection)
GET /health/ready

# Detailed status (active streams, quota usage)
GET /status
```

### Status Response Example

```json
{
  "status": "running",
  "streams": {
    "active_count": 2,
    "streams": [
      {
        "stream_id": "abc123xyz",
        "channel_id": "UCxxxxxx",
        "channel_name": "Example Channel",
        "is_live": true,
        "polling_interval": 2000,
        "last_polled_at": "2025-11-13T10:00:00Z"
      }
    ]
  },
  "quota": {
    "used": 1500,
    "remaining": 8500,
    "percentage": 15.0
  }
}
```

## YouTube API Integration

### API Costs (Quota Units)

- `search.list` (find live streams): 100 units
- `videos.list` (get stream details): 1 unit
- `liveChatMessages.list` (fetch messages): 5 units per request

### Polling Strategy

1. **Discovery**: Periodically check channels for live streams (every 30s)
2. **Polling**: Poll each live stream's chat at API-recommended interval
3. **Adaptive**: Adjust interval based on `pollingIntervalMillis` from API response
4. **Quota**: Track usage and alert at 80% / 90% thresholds

### Rate Limits

- **Quota**: 10,000 units/day (default), can request increase to 1,000,000
- **Example**: 10,000 units ÷ 5 units/poll = ~2,000 API calls/day
- With 2-second intervals: ~4 hours of continuous polling per day

## Message Format

Published to Redis Streams (`chat:raw`) with platform = "youtube":

```json
{
  "message_id": "uuid",
  "platform": "youtube",
  "channel_id": "UCxxxxxx",
  "stream_id": "abc123xyz",
  "user_id": "UCyyyyyy",
  "username": "Viewer123",
  "text": "Hello world!",
  "timestamp": "2025-11-13T10:00:00Z",
  "tags": {
    "channel_url": "https://youtube.com/channel/...",
    "profile_image": "https://...",
    "is_verified": "false",
    "is_owner": "false",
    "is_sponsor": "true",
    "is_moderator": "false",
    "super_chat": "0",
    "super_sticker": "0"
  }
}
```

## Development

### Prerequisites

- Go 1.25+
- PostgreSQL 16 (with migrations applied)
- Redis 7
- YouTube OAuth credentials

### Setup YouTube OAuth

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project
3. Enable YouTube Data API v3
4. Create OAuth 2.0 credentials
5. Add authorized redirect URI: `http://localhost:8080/api/v1/auth/youtube/callback`
6. Copy Client ID and Client Secret to `.env`

### Build & Run

```bash
# Build
cd services/youtube-listener
go build -o youtube-listener ./cmd

# Run
./youtube-listener

# Or use Make
make build-youtube-listener
make run-youtube-listener
```

### Run Tests

```bash
go test ./... -v
go test ./... -cover
```

### Docker

```bash
# Build
docker build -t allchat-youtube-listener -f services/youtube-listener/Dockerfile .

# Run
docker run -d \
  --name youtube-listener \
  -p 8086:8086 \
  -e YOUTUBE_CLIENT_ID=xxx \
  -e YOUTUBE_CLIENT_SECRET=yyy \
  -e DATABASE_HOST=postgres \
  -e REDIS_HOST=redis \
  allchat-youtube-listener
```

## Monitoring

### Metrics to Track

- **Active Streams**: Number of currently polled live streams
- **Quota Usage**: Daily API quota consumption percentage
- **Poll Latency**: Time taken for each API call
- **Message Rate**: Messages published per minute

### Alerts

- Quota usage ≥ 80%: Warning
- Quota usage ≥ 90%: Critical
- OAuth token refresh failures
- Stream polling errors

## Troubleshooting

### "Invalid OAuth token"

- Check token hasn't been revoked by user
- Verify token refresh is working
- Check OAuth credentials are correct

### "Quota exceeded"

- Check `/status` endpoint for current usage
- Wait for daily quota reset (midnight Pacific Time)
- Request quota increase from Google

### "No live streams found"

- Verify channel is actually streaming
- Check OAuth scopes include `youtube.readonly`
- Verify channel ID is correct

### "Polling slow or delayed"

- Check `pollingIntervalMillis` from API responses
- Verify network latency to YouTube API
- Check for API throttling

## Production Considerations

1. **Quota**: Request increase to 1,000,000 units/day
2. **Leader Election**: Use Source Manager to prevent duplicate polling
3. **Token Management**: Monitor token refresh success rate
4. **Scaling**: One instance can handle ~50 concurrent streams
5. **Monitoring**: Export Prometheus metrics for quota and polling rates

## Related Services

- **Twitch Listener**: Same Redis Stream (`chat:raw`)
- **Message Processor**: Consumes from `chat:raw`, normalizes YouTube messages
- **Source Manager**: Manages leader election for YouTube streams

## License

See repository root LICENSE file.
