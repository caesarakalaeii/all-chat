# Overlay Manager

The Overlay Manager service handles CRUD operations for overlays and multi-source chat source configuration. It manages overlay settings, chat source assignments, mock chat injection for testing, and credit roll configurations.

**Port**: 8082
**Status**: ✅ Production Ready

---

## Features

- **Overlay CRUD**: Create, read, update, delete overlays
- **Multi-Source Configuration**: Add/remove chat sources (Twitch, YouTube, Kick, TikTok) per overlay
- **Source Activation/Deactivation**: Enable/disable chat sources dynamically
- **Mock Chat Injection**: Send test messages to overlays for development/preview
- **Credit Roll Management**: Configure end-of-stream credit roll settings
- **YouTube Channel Resolution**: Resolve YouTube @handles to channel IDs (with quota tracking)
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **Metrics**: Prometheus metrics for operations, errors, YouTube API usage

---

## Architecture

```
Frontend (Admin Dashboard)
  ↓ HTTP REST API
Overlay Manager
  ├─ Overlay CRUD
  │   ↓ write to database
  │   PostgreSQL (overlays table)
  │
  ├─ Chat Source Management
  │   ↓ write to database
  │   PostgreSQL (overlay_chat_sources table)
  │   ↓ notify listeners
  │   PostgreSQL NOTIFY (source_changes channel)
  │
  ├─ YouTube Channel Resolution
  │   ↓ call with quota tracking
  │   YouTube Listener (/quota/record endpoint)
  │
  └─ Mock Chat Injection
      ↓ publish test messages
      Redis Pub/Sub (overlay:{overlay_id})
```

---

## Environment Variables

### Required

```bash
# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis connection
REDIS_HOST=localhost
REDIS_PORT=6379

# YouTube Listener URL (for quota tracking)
YOUTUBE_LISTENER_URL=http://localhost:8086
```

### Optional

```bash
# Server configuration
PORT=8082
LOG_LEVEL=info  # debug, info, warn, error

# JWT Authentication
JWT_SECRET=your-secret-key  # Shared with auth-service

# OpenTelemetry tracing
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Application
APP_VERSION=dev
ENVIRONMENT=development

# Discord source guard (ADR-0048) — the same shared bot token discord-listener uses.
# Required to ADD or RECONFIGURE a Discord chat source: every Discord channel is acted on by
# the shared bot, so Discord authorizes the bot rather than the requesting user and will not
# refuse a channel the user has no claim to. overlay-manager resolves each channel's guild via
# GET /channels/{id} and requires a matching discord_guilds row for the overlay owner.
# When unset, Discord source add/patch fails closed with 503 (other platforms are unaffected).
DISCORD_BOT_TOKEN=your_discord_bot_token_here
```

---

## Running Locally

### Prerequisites

- Go 1.25+
- PostgreSQL with all-chat schema
- Redis
- YouTube Listener running (for YouTube channel resolution)

### Development

```bash
# Set environment variables
export DATABASE_HOST=localhost
export REDIS_HOST=localhost
export YOUTUBE_LISTENER_URL=http://localhost:8086
export JWT_SECRET=your-secret

# Run the service
cd services/overlay-manager
go run ./cmd

# Or build and run
go build -o overlay-manager ./cmd
./overlay-manager
```

---

## API Endpoints

### Overlays

```bash
# Create overlay
POST /api/v1/overlays
Authorization: Bearer <jwt-token>
Body: {
  "name": "My Stream Overlay",
  "description": "Multi-platform chat display",
  "settings": {
    "theme": "dark",
    "font_size": 16
  }
}

# List user's overlays
GET /api/v1/overlays
Authorization: Bearer <jwt-token>

# Get overlay by ID
GET /api/v1/overlays/:id
Authorization: Bearer <jwt-token>

# Update overlay
PUT /api/v1/overlays/:id
Authorization: Bearer <jwt-token>
Body: { "name": "Updated Name", ... }

# Delete overlay
DELETE /api/v1/overlays/:id
Authorization: Bearer <jwt-token>
```

### Chat Sources

```bash
# Add chat source to overlay
POST /api/v1/overlays/:overlay_id/sources
Authorization: Bearer <jwt-token>
Body: {
  "platform": "twitch",
  "channel_identifier": "xqc",  // Twitch username
  "priority": 1
}

# For YouTube (requires channel resolution):
Body: {
  "platform": "youtube",
  "channel_identifier": "@MrBeast",  // YouTube @handle
  "priority": 1
}
# → Service resolves @MrBeast to UCX6OQ3DkcsbYNE6H8uQQuVA
# → Records 100 units quota usage via YouTube Listener

# List sources for overlay
GET /api/v1/overlays/:overlay_id/sources
Authorization: Bearer <jwt-token>
# Each Twitch source carries two computed booleans used by the IRC→EventSub migration UI:
#   chat_via_eventsub — channel owner granted chat scopes, so chat is read via EventSub (else IRC)
#   is_own_channel    — the requesting user owns this channel and can re-consent to migrate it
#                       (true for Twitch-login owners and ADR-0016 linked-credential owners)

# Activate/deactivate source
PATCH /api/v1/overlays/:overlay_id/sources/:source_id
Authorization: Bearer <jwt-token>
Body: { "is_active": false }

# Delete source
DELETE /api/v1/overlays/:overlay_id/sources/:source_id
Authorization: Bearer <jwt-token>
```

### Mock Chat (Testing)

```bash
# Send mock message to overlay (for testing/preview)
POST /api/v1/overlays/:overlay_id/mock
Authorization: Bearer <jwt-token>
Body: {
  "username": "TestUser",
  "text": "Hello world! Kappa",
  "platform": "twitch"
}
→ Published to overlay:{overlay_id} Pub/Sub channel (appears in overlay immediately)
```

### Credit Roll

```bash
# Configure credit roll for overlay
POST /api/v1/overlays/:overlay_id/creditroll
Authorization: Bearer <jwt-token>
Body: {
  "enabled": true,
  "min_messages": 5,
  "duration_seconds": 30
}

# Get credit roll configuration
GET /api/v1/overlays/:overlay_id/creditroll
Authorization: Bearer <jwt-token>
```

### Health Checks

```bash
GET /health/live   # Liveness
GET /health/ready  # Readiness (checks DB + Redis)
GET /metrics       # Prometheus metrics
```

---

## YouTube Channel Resolution

### Problem

Users often know YouTube channel **@handle** (e.g., `@MrBeast`) but not the **channel ID** (e.g., `UCX6OQ3DkcsbYNE6H8uQQuVA`). YouTube Listener requires channel ID for API calls.

### Solution

Overlay Manager resolves @handles to channel IDs using YouTube `search.list` API (costs 100 quota units per call).

**Flow**:
```
User enters: @MrBeast
  ↓
Overlay Manager calls YouTube API: search.list?q=MrBeast&type=channel
  ↓ (costs 100 units)
YouTube API returns: UCX6OQ3DkcsbYNE6H8uQQuVA
  ↓
Overlay Manager records quota usage: POST youtube-listener:8086/quota/record
  ↓
Store in database: channel_identifier=UCX6OQ3DkcsbYNE6H8uQQuVA
```

**Quota Tracking Integration**:
- Uses `youtube/quota_client.go` HTTP client wrapper
- Calls `/quota/record` endpoint on YouTube Listener
- Ensures channel resolution quota is tracked (prevents untracked consumption)

**File**: `youtube/resolver.go:ResolveChannel()`, `youtube/quota_client.go`

---

## Database Schema

### overlays Table

```sql
CREATE TABLE overlays (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    settings JSONB DEFAULT '{}',       -- Theme, font, colors
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### overlay_chat_sources Table

```sql
CREATE TABLE overlay_chat_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,          -- 'twitch', 'youtube', 'kick', 'tiktok'
    channel_identifier VARCHAR(255) NOT NULL,
    channel_name VARCHAR(255),
    priority INT DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',            -- Platform-specific data
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### credit_roll_configs Table

```sql
CREATE TABLE credit_roll_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE UNIQUE,
    enabled BOOLEAN DEFAULT false,
    min_messages INT DEFAULT 5,
    duration_seconds INT DEFAULT 30,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

---

## Troubleshooting

### YouTube Channel Resolution Fails

**Symptom**: POST /overlays/:id/sources returns 500 when adding YouTube source

**Solutions**:
1. **Check YouTube Listener**: Ensure running at `YOUTUBE_LISTENER_URL`
2. **Check quota**: Call `/quota/status` on YouTube Listener (may be exhausted)
3. **Invalid @handle**: Verify channel exists on YouTube
4. **API credentials**: Ensure YouTube Listener has valid API key

**File**: `youtube/resolver.go:ResolveChannel()`

### Source Not Activating Listeners

**Symptom**: Added chat source but messages not appearing

**Check database NOTIFY**:
```sql
-- Check if NOTIFY triggered
LISTEN source_changes;
-- (add source via API)
-- Should see: NOTIFY source_changes with payload
```

**Solutions**:
1. Verify `is_active=true` for overlay AND source
2. Check listener received NOTIFY (check listener logs)
3. Verify platform listener is running
4. Check listener is polling database or received NOTIFY

**File**: `repository/source_repo.go:CreateSource()`

---

## Production Considerations

1. **Authentication**: All endpoints require valid JWT (except health checks)
2. **Rate Limiting**: YouTube channel resolution limited to 10 req/min per user (prevent quota abuse)
3. **Quota Tracking**: YouTube resolution calls tracked via YouTube Listener
4. **Mock Chat**: Disable in production or require admin role
5. **Database NOTIFY**: Ensure PostgreSQL LISTEN connections maintained by listeners

---

## Related Services

- **Auth Service**: Issues JWT tokens for API authentication
- **YouTube Listener**: Provides quota tracking endpoint, uses resolved channel IDs
- **Twitch/Kick/TikTok Listeners**: Listen to PostgreSQL NOTIFY for source changes
- **API Gateway**: Routes HTTP requests to Overlay Manager

---

## Further Reading

- **[00-OVERVIEW.md](../../docs/architecture/00-OVERVIEW.md)** - System architecture
- **[ADR-0006](../../docs/adr/0006-youtube-quota-tracking.md)** - YouTube quota tracking
- **[QUICK-REF-ADD-PLATFORM.md](../../docs/llm-guides/QUICK-REF-ADD-PLATFORM.md)** - Add new platform integration

---

## License

Copyright © 2025 All-Chat. All rights reserved.
