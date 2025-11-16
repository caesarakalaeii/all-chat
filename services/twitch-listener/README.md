# Twitch Listener

The Twitch Listener service connects to Twitch IRC, monitors configured channels, parses chat messages, and publishes them to Redis Streams for processing by the Message Processor.

## Features

- **Twitch IRC Connection**: Connects to Twitch IRC using go-twitch-irc library
- **Dynamic Channel Management**: Automatically syncs with database to JOIN/PART channels
- **Rate Limiting**: Respects Twitch rate limits (20 JOIN/10s)
- **Message Parsing**: Extracts user info, badges, emotes, colors from IRC tags
- **Redis Streams**: Publishes raw messages to `chat:raw` stream
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **Graceful Shutdown**: Properly disconnects from IRC and completes in-flight operations

## Architecture

```
Database (active overlays)
  ↓
Channel Manager (sync every 30s)
  ↓
IRC Connection (JOIN/PART channels)
  ↓
Message Handler (parse PRIVMSG)
  ↓
Redis Streams (chat:raw)
```

## Environment Variables

### Required

```bash
# Twitch IRC credentials
TWITCH_BOT_USERNAME=your_bot_username
TWITCH_BOT_OAUTH=oauth:your_oauth_token

# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password

# Redis connection
REDIS_HOST=localhost
REDIS_PORT=6379

# Source Manager (leadership)
SOURCE_MANAGER_URL=http://localhost:8088
SOURCE_MANAGER_SECRET=dev-service-secret
```

### Optional

```bash
# Server configuration
PORT=8085
LOG_LEVEL=info  # debug, info, warn, error
```

## Getting Twitch OAuth Token

1. Go to https://twitchapps.com/tmi/
2. Authorize with your bot account
3. Copy the OAuth token (format: `oauth:abc123...`)
4. Set `TWITCH_BOT_OAUTH` environment variable

## Running Locally

### Prerequisites

- Go 1.25+
- PostgreSQL with all-chat schema
- Redis
- Twitch bot account with OAuth token

### Development

```bash
# Set environment variables
export TWITCH_BOT_USERNAME=your_bot
export TWITCH_BOT_OAUTH=oauth:your_token
export DATABASE_HOST=localhost
export REDIS_HOST=localhost

# Run the service
go run ./cmd

# Or build and run
go build -o twitch-listener ./cmd
./twitch-listener
```

### With Docker Compose

```bash
# Add to .env file
TWITCH_BOT_USERNAME=your_bot
TWITCH_BOT_OAUTH=oauth:your_token

# Start service
cd deployments
docker-compose up twitch-listener
```

## API Endpoints

### Health Checks

```bash
# Liveness probe (always returns 200 if service is running)
GET /health/live

# Readiness probe (checks IRC + Redis connections)
GET /health/ready

# Detailed status
GET /status
```

**Example Response** (`/status`):
```json
{
  "status": "ok",
  "irc": {
    "connected": true,
    "active_channels": 5,
    "channels": ["xqc", "summit1g", "shroud", "pokimane", "ninja"]
  }
}
```

## Message Format

Messages published to Redis Streams (`chat:raw`):

```json
{
  "message_id": "uuid",
  "platform": "twitch",
  "channel_id": "xqc",
  "user_id": "12345678",
  "username": "viewer123",
  "text": "Hello Kappa PogChamp",
  "timestamp": "2025-11-13T10:00:00Z",
  "tags": {
    "user-id": "12345678",
    "display-name": "Viewer123",
    "color": "#FF0000",
    "badges": "subscriber/12,moderator/1",
    "subscriber": "1",
    "mod": "1",
    "turbo": "0",
    "emotes": "25:6-10/305954156:12-20",
    "id": "abc-123-def",
    "room-id": "71092938",
    "tmi-sent-ts": "1699876543210"
  }
}
```

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run without integration tests (no Redis/DB required)
go test ./... -short

# Run specific package
go test ./channels -v
go test ./irc -v
go test ./publisher -v
```

## How It Works

### 1. Channel Synchronization

Every 30 seconds, the Channel Manager queries the database for active Twitch sources:

```sql
SELECT DISTINCT ocs.channel_id
FROM overlays o
JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
WHERE o.is_active = true
  AND ocs.platform = 'twitch'
```

### 2. JOIN/PART Logic

- **JOIN**: New channels are joined with rate limiting (20 per 10 seconds)
- **PART**: Channels no longer in database are departed immediately

### 3. Message Processing

When a PRIVMSG arrives:

1. Parse IRC message → `RawChatMessage`
2. Extract user info, badges, emotes from IRC tags
3. Generate UUID for message ID
4. Publish to Redis Streams (`chat:raw`)

### 4. Rate Limits

Twitch IRC rate limits for authenticated connections:

- **JOIN**: 20 channels per 10 seconds
- **PRIVMSG**: 100 messages per 30 seconds (not applicable for listeners)
- **Connections**: Max 50 channels per connection (can use multiple if needed)

## Monitoring

### Key Metrics to Monitor

- IRC connection status
- Number of active channels
- Messages published per second
- Redis Streams backlog (`XPENDING`)

### Logs

All logs are structured JSON (Zap logger):

```json
{
  "level": "info",
  "ts": "2025-11-13T10:00:00Z",
  "service": "twitch-listener",
  "msg": "Joined channel",
  "channel": "xqc"
}
```

## Troubleshooting

### IRC Connection Fails

```
Error: Failed to connect to Twitch IRC
```

**Solution**: Verify `TWITCH_BOT_USERNAME` and `TWITCH_BOT_OAUTH` are correct. OAuth token must start with `oauth:`.

### No Channels Joined

```
INFO: Channel sync completed {"total_active": 0, "joined": 0, "parted": 0}
```

**Solution**:
1. Check database has active overlays with Twitch sources
2. Verify `overlay_chat_sources` table has records with `platform='twitch'`

### Messages Not Publishing

```
ERROR: Failed to publish message
```

**Solution**:
1. Check Redis connection: `redis-cli ping`
2. Verify `REDIS_HOST` and `REDIS_PORT` are correct
3. Check Redis disk space

### Rate Limit Exceeded

Twitch will disconnect if you JOIN too many channels too quickly.

**Solution**: The service already implements rate limiting (20/10s). If you need more channels, use multiple bot accounts.

## Production Considerations

- Use a dedicated Twitch bot account (not your main account)
- Enable Twitch bot verification for higher rate limits
- Monitor IRC reconnections (implement alerting)
- Set up Redis persistence for message buffer
- Configure log aggregation (Loki, CloudWatch, etc.)
- Add Prometheus metrics for monitoring

## License

Copyright © 2025 All-Chat. All rights reserved.
