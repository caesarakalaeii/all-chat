# TikTok Listener Service

⚠️ **BETA SERVICE** - Uses unofficial TikTok-Live-Connector library

Real-time TikTok LIVE chat monitoring service for All-Chat. Connects to TikTok live streams and publishes chat messages to Redis Streams for processing.

> **Note**: This service is currently in beta development and uses an unofficial library.

## ⚠️ Important Notice

This service uses the **unofficial** [TikTok-Live-Connector](https://github.com/zerodytrash/TikTok-Live-Connector) library, which is based on reverse engineering TikTok's internal WebSocket service.

**Limitations:**
- Not production-ready according to library authors
- May break if TikTok changes their internal APIs
- No official support from TikTok
- Should be replaced when TikTok releases an official Live Chat API

## Features

- ✅ Monitor multiple TikTok live streams simultaneously
- ✅ Real-time chat message capture
- ✅ **Message deduplication** (prevents replay on reconnect using native TikTok message IDs)
- ✅ **Native timestamp preservation** (uses TikTok's original message timestamps)
- ✅ Publishes to Redis Streams (`chat:raw`)
- ✅ Dynamic stream management (polls database for active channels)
- ✅ Health check endpoints
- ✅ Graceful shutdown handling
- ✅ TypeScript implementation
- ✅ Resource-optimized with deduplication cache limits

## Architecture

```
┌─────────────────────────────────────────┐
│  TikTok Listener Service (Node.js)      │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  TikTok-Live-Connector Library     │ │
│  │  (Unofficial WebSocket Client)     │ │
│  └────────────────┬───────────────────┘ │
│                   │                      │
│                   ▼                      │
│  ┌────────────────────────────────────┐ │
│  │   Message Handler                  │ │
│  │   - Normalizes to RawChatMessage   │ │
│  │   - Adds overlay_id tagging        │ │
│  └────────────────┬───────────────────┘ │
│                   │                      │
│                   ▼                      │
│  ┌────────────────────────────────────┐ │
│  │   Redis Streams Publisher          │ │
│  │   Stream: chat:raw                 │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Prerequisites

- Node.js 18+
- Redis (for message publishing)
- PostgreSQL (for active stream tracking)

## Installation

```bash
cd services/tiktok-listener
npm install
```

## Configuration

Set the following environment variables:

```bash
# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# PostgreSQL
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Service
PORT=8089
LOG_LEVEL=info
POLL_INTERVAL_MS=30000  # Poll for active streams every 30 seconds

# Message Deduplication (prevents replay on reconnect)
TIKTOK_DEDUP_TTL_MS=300000           # Keep dedup cache for 5 minutes (default)
TIKTOK_DEDUP_CLEANUP_INTERVAL_MS=60000  # Cleanup interval: 1 minute (default)
TIKTOK_DEDUP_MAX_CACHE_SIZE=10000    # Max messages in dedup cache (default)
```

## Development

```bash
# Run in development mode with ts-node
npm run dev

# Build TypeScript to JavaScript
npm run build

# Run production build
npm start
```

## Message Format

The service publishes messages to Redis Stream `chat:raw` in the following format:

```json
{
  "message_id": "tiktok_native_msg_id",
  "platform": "tiktok",
  "channel_id": "tiktok_username",
  "stream_id": null,
  "user_id": "unique_user_id",
  "username": "Display Name",
  "text": "Hello from TikTok!",
  "timestamp": "2025-11-15T12:34:56.789Z",
  "tags": {
    "overlay_id": "uuid",
    "user_unique_id": "@username",
    "profile_picture_url": "https://...",
    "is_follower": "true",
    "is_subscriber": "false",
    "badge_level": "0",
    "native_msg_id": "tiktok_native_msg_id",
    "native_create_time": "1731675296"
  }
}
```

**Note**: 
- `message_id` now uses TikTok's native message ID for accurate deduplication
- `timestamp` uses TikTok's original `createTime` (converted from Unix timestamp)
- This prevents duplicate messages from appearing when the service reconnects
    "badge_level": "0"
  }
}
```

## Health Endpoints

- `GET /health/live` - Liveness probe (always returns 200)
- `GET /health/ready` - Readiness probe (checks Redis connection)
- `GET /status` - Service status with active stream count

## Database Requirements

The service queries the `overlay_chat_sources` table to determine which TikTok streams to monitor:

```sql
SELECT DISTINCT
  ocs.overlay_id,
  ocs.channel_id as tiktok_username,
  ocs.is_active
FROM overlay_chat_sources ocs
WHERE ocs.platform = 'tiktok'
  AND ocs.is_active = true
```

**Requirements:**
- `channel_id` should contain the TikTok username (e.g., `@officialgeilegisela` or `officialgeilegisela`)
- `platform` must be `'tiktok'`
- `is_active` must be `true`

## How It Works

1. **Polling**: Every 30 seconds (configurable), the service polls PostgreSQL for active TikTok channels
2. **Connection**: For each active channel, creates a TikTok-Live-Connector instance
3. **Event Listening**: Subscribes to `WebcastEvent.CHAT` events from the library
4. **Message Deduplication**: Uses TikTok's native message ID to detect and skip duplicate messages (important during reconnects)
5. **Timestamp Preservation**: Extracts TikTok's original `createTime` timestamp instead of generating new ones
6. **Message Publishing**: Normalizes chat messages to `RawChatMessage` format and publishes to Redis Stream
7. **Dynamic Management**: Automatically connects to new channels and disconnects from removed ones

## Resource Optimizations

### Message Deduplication

The service implements message deduplication to prevent replayed messages on reconnection:

- **Native Message ID Tracking**: Uses TikTok's `msgId` from the message's `common` property
- **TTL-based Cache**: Keeps track of seen messages for 5 minutes (configurable via `TIKTOK_DEDUP_TTL_MS`)
- **Automatic Cleanup**: Expired entries are cleaned up every minute to prevent memory leaks
- **Size Limits**: Cache limited to 10,000 messages by default (`TIKTOK_DEDUP_MAX_CACHE_SIZE`)

When the service restarts or reconnects, TikTok may replay recent messages. The deduplicator detects these replays and prevents them from being published to Redis.

### Memory Management

- **Increased Memory Limits**: Default memory limit increased to 1GB for better stability
- **Bounded Cache**: Deduplication cache has hard limits to prevent unbounded growth
- **Periodic Cleanup**: Automatic cleanup of expired cache entries every minute

## Limitations & Known Issues

1. **Unofficial Library**: May break if TikTok changes internal APIs
2. **No Authentication**: Library doesn't require OAuth (connects anonymously)
3. **Username-Based**: Requires TikTok username, not live stream ID
4. **Limited Metadata**: Some data (like stream_id) not available via unofficial library
5. **Rate Limits**: Unknown rate limits from TikTok's side
6. **Connection Stability**: May experience disconnections during long streams

## Troubleshooting

### Connection Fails

```
Error: Failed to connect to TikTok stream
```

**Possible causes:**
- Username is incorrect
- User is not currently live
- TikTok has blocked the connection
- Network connectivity issues

**Solutions:**
- Verify the username is correct (without @ symbol)
- Ensure the user is actively streaming
- Check network connectivity
- Try again later if rate limited

### Messages Not Appearing

**Check:**
1. Redis connection: `redis-cli PING`
2. Stream exists: `redis-cli XLEN chat:raw`
3. Service logs for errors
4. User is actually live on TikTok
5. Chat is enabled on the stream

## Migration Path (Future)

When TikTok releases an official Live Chat API:

1. Update OAuth scopes in auth service (add live chat permissions)
2. Replace `TikTok-Live-Connector` with official TikTok API client
3. Update authentication to use OAuth tokens from database
4. Modify message handler to parse official API response format
5. Update health checks to validate OAuth token status
6. Remove BETA labels from UI

## Dependencies

- `tiktok-live-connector` - Unofficial TikTok LIVE WebSocket client
- `redis` - Redis client for stream publishing
- `pg` - PostgreSQL client for active stream queries
- `winston` - Logging
- `typescript` - Type safety

## Docker

See `Dockerfile` for containerization details.

## License

MIT (same as All-Chat project)
