# YouTube Listener

YouTube Data API chat listener (gRPC `liveChatMessages.streamList`) for premium official-API mode.

## Overview

This listener serves only YouTube sources that opted into official-API mode
(`config.official_api: true`, premium gate `youtube_official_api`, ADR-0065). Every other
YouTube channel is served by [youtube-listener-innertube](../youtube-listener-innertube/README.md),
which spends no quota. The official API is used where InnerTube cannot help: unlisted streams
that are not listed on the channel page and members-only chat, which only the channel owner's
own credentials can read.

### Which channels it serves

A source is eligible when it opted in, its overlay is active, its owner is not banned, the owner
holds a `youtube_oauth_tokens` row for the channel, and the owner is premium (or the gate is
free). Eligibility is re-read on every round, so a lapsed subscription falls back to InnerTube
while the opt-in stays stored.

A token row is not proof of ownership: overlay-manager's add-by-link path files a copy of an
admin's token under the channel added, and did so for every user before it was limited to admin
sessions. Each owner is therefore verified with `channels.list(mine=true)` on
the stored token (cached six hours, one unit per check), and only a token whose own channel is
the source's channel is used.

### Claims and handover

Every 60 seconds the global-sync leader writes `youtube:official:claim:{channelID}` (TTL 3
minutes, `shared/youtubeclaim`) for each verified channel whose opted-in owner has an overlay
connected (an overlay inside the 90s disconnect debounce still counts), and `syncStreams` serves
only claimed channels; a replica that does not hold the global-sync lease serves nothing. innertube
skips claimed channels and stops what it polls on them. A claim is released when the channel stops
qualifying, when the owner's overlay disconnects, while the global quota state is Critical or
worse, when a sync hits a token or auth error on the owner's credentials (the ownership verdict is
forgotten too, so the next round re-verifies), and when the replica shuts down. A crashed listener
lets the TTL expire. Either way InnerTube takes the channel back on its next sync.

It implements:

- Per-streamer OAuth (the owner's `youtube.readonly` token)
- Live broadcast discovery with `liveBroadcasts.list broadcastStatus=active broadcastType=all`,
  which returns unlisted, members-only and "Stream now" broadcasts
- gRPC server streaming of chat
- API quota tracking and alerting
- Message normalization to unified format

## Architecture

```
YouTube Live Chat API
        ↓
  OAuth Manager
        ↓
   Stream Manager (with Global Sync Leader Election)
     ↓        ↓
  Poller1  Poller2  (one per live stream, per-stream leader election)
        ↓
   API Client
        ↓
     Parser
        ↓
Redis Streams (chat:raw)
```

**Leader Election Strategy:**
- **Global Sync Leader**: Only one replica performs broadcast discovery and refreshes claims
  - Uses Redis lock with stream ID: `"global-sync"`
  - Checked before periodic sync, PostgreSQL LISTEN events, and overlay connections
  - Prevents quota waste when running multiple replicas (e.g., 3 replicas = 66% quota savings)
- **Per-Stream Leader**: Only one replica polls chat messages for each live stream
  - Uses Redis lock with actual stream ID (e.g., video ID)
  - Prevents duplicate message publishing
  - Ensures high availability (if leader fails, another replica takes over)

## Features

- **OAuth 2.0**: Per-user OAuth tokens stored in PostgreSQL
- **Live Stream Detection**: Automatically detects when channels go live
- **Adaptive Polling**: Uses API-recommended polling intervals (typically 2-5 seconds)
- **Quota Management**: Tracks daily YouTube API quota (default 1,009,000 units/day)
- **Health Checks**: Liveness, readiness, and status endpoints
- **Graceful Shutdown**: Stops all pollers cleanly

## Configuration

### Environment Variables

```bash
# YouTube OAuth (required): the same OAuth client that minted the youtube_oauth_tokens rows
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx
FRONTEND_URL=http://localhost:3000       # OAuth redirect URL is derived from it

# Token encryption (required): the same key chain as auth-service and token-refresh-service.
# TOKEN_ENCRYPTION_KEY_V1 is required; V2.. as rotated; the legacy keys below are read
# as decryption fallbacks when set.
TOKEN_ENCRYPTION_KEY_V1=<key, same value as auth-service>
# TOKEN_ENCRYPTION_KEY=<legacy key>
# YOUTUBE_TOKEN_ENCRYPTION_KEY=<legacy key>

# Database (required)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password

# Redis (required)
REDIS_HOST=localhost
REDIS_PORT=6379
# REDIS_PASSWORD=

# Source Manager (leader election). Required in any multi-replica deployment: without the
# secret coordination is disabled, and every replica claims and polls every channel.
SOURCE_MANAGER_URL=http://localhost:8088
SOURCE_MANAGER_SECRET=dev-service-secret

# Service configuration
PORT=8086
LOG_LEVEL=info
QUOTA_LIMIT_DAILY=1009000         # Daily API quota limit (keep the shared default)
# youtube-quota-monitor owns quota:alerts (ADR-0023); set false in deployments that run it,
# or every listener replica posts each quota transition again.
QUOTA_NOTIFIER_ENABLED=false
# CIRCUIT_BREAKER_OPEN_DURATION_MINUTES is clamped to 1; leave it unset.
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

- `liveBroadcasts.list` (find the owner's active broadcasts): **1 unit** (estimate: not stated on the
  method's page; `QuotaCostLiveBroadcasts` in `quota/tracker.go`)
- `channels.list mine=true` (owner verification, cached 6h): **1 unit** (estimate, `QuotaCostChannels`)
- `videos.list` (get stream details): **1 unit**
- `liveChatMessages.streamList` (fetch messages): **5 units** per stream connection

## Quota Tracking System

The YouTube Listener implements a sophisticated **reserve-confirm-rollback** quota tracking system to ensure 100% accurate quota accounting and prevent waste.

### Architecture

**Reserve-Confirm-Rollback Pattern:**
```
1. RESERVE quota BEFORE API call (atomic database operation)
   ↓
2. Make YouTube API call
   ↓
3a. SUCCESS or 5xx error → CONFIRM (move reserved → used)
3b. 4xx client error → ROLLBACK (release reservation)
```

**Benefits:**
- ✅ **Zero drift**: Quota reserved before call, impossible to diverge
- ✅ **Atomic operations**: Database row-level locking prevents race conditions
- ✅ **Smart charging**: 4xx client errors don't consume quota
- ✅ **Crash recovery**: Stale reservations auto-recovered every minute
- ✅ **99.95%+ accuracy**: Database is authoritative source of truth

### Quota Optimizations

**Waste Elimination** (~9,000 units/day savings):

1. **Stop on exhaustion**: Pollers stop entirely when quota depleted (not retry every 5 min)
   - Saves: 1,440 units/day

2. **Immediate cache clearing**: Video cache cleared on first offline detection (not after 3 checks)
   - Saves: 200+ units per stream end
   - Prevents: Up to 288,000 units/day worst-case

3. **Enhanced status check**: CheckStreamStatus returns liveChatID (eliminates redundant call)
   - Saves: 2,880 units/day per channel (50% reduction in cached polling cost)

4. **Smart disconnect**: Stops polling immediately when last overlay disconnects
   - Saves: 75-90 units per disconnect event

5. **Connection batching**: 5-second debounce batches rapid overlay connections
   - Saves: 400+ units when 5 overlays connect quickly

**Cross-Service Tracking:**
- overlay-manager YouTube API calls tracked via HTTP client to youtube-listener
- Prevents untracked quota consumption from channel resolution (100 units per resolve)
- Fail-open circuit breaker: quota tracking failure doesn't block users

### Quota API Endpoints

```bash
# Get current quota status (global + per-channel)
GET /quota/status
Response: {
  "global": {
    "state": "HEALTHY",           # HEALTHY | DEGRADED | CRITICAL | EXHAUSTED | DEPLETED
    "used": 2500,
    "reserved": 50,                # New: in-flight API calls
    "remaining": 7450,
    "percentage": 25.5,
    "resets_at": "2026-01-10T00:00:00Z",
    "polling_multiplier": 1.0      # Adaptive polling slowdown
  },
  "channels": [...]
}

# Record quota usage from external services (overlay-manager)
POST /quota/record
Body: {"units": 100}

# Get quota history (last N days)
GET /quota/history?days=7

# Get quota predictions (forecasting)
GET /quota/predictions

# Get per-channel quota
GET /quota/channels/:channel_id
```

### Quota State Machine

| State | Percentage | Behavior | Polling Multiplier |
|-------|------------|----------|-------------------|
| **HEALTHY** | 0-70% | Normal polling | 1.0x |
| **DEGRADED** | 70-85% | Slow down non-critical | 1.5x |
| **CRITICAL** | 85-95% | Only high-priority channels | 2.0x |
| **EXHAUSTED** | 95-100% | Stop new discoveries | Stop |
| **DEPLETED** | >100% | Stop all polling | Stop |

### Polling Strategy

1. **Discovery**: Check channels for live streams
   - Uses cached video ID first (1 unit lightweight check)
   - Falls back to `liveBroadcasts.list` on the owner's token if the cache misses (1 unit)
   - Backoff when no stream is found is capped at one minute (`maxDetectionBackoff`: detection
     interval, negative cache and circuit-breaker open window), so a go-live is seen within about
     a minute while innertube is kept off the claimed channel

2. **Polling**: Poll each live stream's chat at API-recommended interval
   - Typical: 2-5 seconds based on `pollingIntervalMillis`
   - Stops immediately on quota exhaustion
   - Resumes after midnight PST quota reset

3. **Adaptive**: Responds to quota pressure
   - Slows polling in DEGRADED/CRITICAL states
   - Prioritizes high-tier channels
   - Batches overlay connection syncs (5-second window)

4. **Quota Conscious**: Every decision optimized for quota efficiency
   - Leadership election prevents duplicate work
   - Connection debouncing batches expensive syncs
   - Smart disconnect stops unnecessary polling

### Rate Limits

- **Quota**: 1,009,000 units/day (increased from default 10,000)
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

- Go 1.26+
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

**Check current status:**
```bash
kubectl exec -n allchat youtube-listener-<pod> -- \
  wget -qO- http://localhost:8086/quota/status | jq .
```

**Common causes:**
- Normal usage approaching daily limit (check `percentage` in status)
- Untracked API calls from other services (should be fixed now)
- Quota tracking drift (should be <±5 units with reserve-confirm-rollback)

**Solutions:**
1. **Wait for reset**: Quota resets at midnight Pacific Time (00:00 PST/PDT)
2. **Check tracking accuracy**:
   ```sql
   SELECT date, units_used, units_reserved, units_limit
   FROM youtube_quota_usage
   WHERE date = CURRENT_DATE;
   ```
3. **Verify all services using quota tracking**:
   - youtube-listener: All API calls use reserve-confirm-rollback ✅
   - overlay-manager: Uses YouTubeQuotaClient HTTP client ✅
4. **Request increase**: Go to Google Cloud Console to request 1,000,000 units/day

**Check for stale reservations:**
```sql
SELECT cleanup_stale_quota_reservations();
```

**Monitoring:**
- Units reserved should be <50 under normal operation
- If units_reserved grows continuously, check for crashes/errors
- Stale reservations auto-cleanup every minute (>5 min old)

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
2. **Leader Election**: 
   - **Global Sync Leader**: Only one replica performs broadcast discovery and refreshes claims
   - **Per-Stream Leader**: Prevents duplicate chat message polling across replicas
   - Both use Source Manager for distributed leadership coordination
3. **Token Management**: Monitor token refresh success rate
4. **Scaling**: 
   - Multiple replicas supported (HPA scales 1-5 pods)
   - Global sync leader election ensures only one replica does expensive API calls
   - One instance can handle ~50 concurrent streams of chat polling
5. **Monitoring**: Export Prometheus metrics for quota and polling rates

## Related Services

- **Twitch Listener**: Same Redis Stream (`chat:raw`)
- **Message Processor**: Consumes from `chat:raw`, normalizes YouTube messages
- **Source Manager**: Manages leader election for YouTube streams

## License

See repository root LICENSE file.
