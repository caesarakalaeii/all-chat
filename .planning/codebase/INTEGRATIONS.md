# External Integrations

**Analysis Date:** 2026-02-16

## APIs & External Services

**Streaming Platforms - Chat Collection:**
- **Twitch** - Primary streamer platform
  - SDK/Client: `github.com/gempir/go-twitch-irc/v4` (IRC-based chat)
  - Implementation: `services/twitch-listener/` (IRC connection, channel management)
  - Auth: OAuth 2.0 via `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`
  - Chat collection: Bot username + OAuth token via `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH`
  - Scopes: `channel:read:redemptions`, `channel:read:subscriptions`, `bits:read`, `moderator:read:followers`

- **YouTube** - Secondary platform
  - SDK/Client: `google.golang.org/api/youtube/v3` (YouTube Data API v3)
  - Implementation: `services/youtube-listener/` (HTTP polling, quota management)
  - Auth: OAuth 2.0 via `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`
  - Scopes: `https://www.googleapis.com/auth/youtube.readonly`, `https://www.googleapis.com/auth/youtube.force-ssl`
  - Token encryption: Base64-encoded 32-byte AES key via `YOUTUBE_TOKEN_ENCRYPTION_KEY`
  - Quota tracking: Daily limit 1,009,000 units with per-channel tiered quotas
  - Hybrid detection: Status checks + full detection fallback

- **Kick** - Emerging platform
  - SDK/Client: Custom OAuth 2.1 with PKCE implementation
  - Implementation: `services/kick-listener/` (Pusher WebSocket)
  - Auth: OAuth 2.1 via `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`
  - WebSocket transport: `github.com/gorilla/websocket`

- **TikTok** - Beta platform (unofficial)
  - SDK/Client: `tiktok-live-connector` npm package (v2.1.0)
  - Implementation: `services/tiktok-listener/` (Node.js + TypeScript)
  - Auth: Username-based (no OAuth required)
  - Limitations: Uses unofficial library; official TikTok Live API not yet available
  - Backoff: Configurable offline/error backoff with exponential increase

**Emote Providers:**
- **Twitch Emotes** - Platform-native emotes
  - SDK/Client: Twitch Helix API via custom client
  - Implementation: `services/emote-service/clients/twitch_emotes.go`
  - Auth: OAuth via `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`

- **7TV** - Community emote platform
  - API: REST endpoints (no SDK)
  - Implementation: `services/emote-service/clients/seventv.go`
  - Auth: Public API (no authentication required)

- **BTTV (Better Twitch.tv)** - Community emote platform
  - API: REST endpoints (no SDK)
  - Implementation: `services/emote-service/clients/bttv.go`
  - Auth: Public API (no authentication required)

- **FFZ (FrankerFaceZ)** - Community emote platform
  - API: REST endpoints (no SDK)
  - Implementation: `services/emote-service/clients/ffz.go`
  - Auth: Public API (no authentication required)

**Discord Integration:**
- **Discord Bot** - Quota monitoring and notifications
  - SDK/Client: `discord.js` v14.14.1 (npm)
  - Implementation: `services/discord-bot/`
  - Auth: Bot token via `DISCORD_BOT_TOKEN`
  - Target: Posts quota alerts to channel ID via `DISCORD_CHANNEL_ID`
  - Purpose: YouTube API quota usage monitoring and periodic status updates

## Data Storage

**Databases:**
- **PostgreSQL 16**
  - Connection: `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`
  - Client: `github.com/jackc/pgx/v5` (high-performance driver)
  - ORM: None (raw SQL via pgx)
  - Schemas: SQL migrations in `migrations/` directory
  - Key tables: users, overlays, chat_sources, youtube_channels, quota_reservations, stream_history, oauth_tokens

**Redis 7:**
  - Connection: `REDIS_HOST`, `REDIS_PORT`
  - Client: `github.com/redis/go-redis/v9`
  - Purpose: Message transport + caching
  - Streams: `chat:raw` - Raw messages from all listeners
  - Pub/Sub: `overlay:{overlay_id}` - Per-overlay message distribution
  - Data structures: Strings, Streams, Channels (Pub/Sub)

**File Storage:**
- Local filesystem only
- Frontend static assets served via Next.js
- No cloud storage integration

**Caching:**
- Redis 7 (in-memory caching + message transport)
- No external cache service

## Authentication & Identity

**Auth Provider:**
- Custom OAuth 2.0 implementation for Twitch, YouTube, Kick
- Custom Kick OAuth 2.1 with PKCE
- Username-based for TikTok (no auth required)

**Implementation:**
- `services/auth-service/oauth/` - OAuth handlers per platform
- JWT tokens via `github.com/golang-jwt/jwt/v5`
- Token encryption: Custom AES-CBC implementation (basic - TODO: upgrade to AES-GCM)
- Token refresh: `services/token-refresh-service/` - Automatic token lifecycle management
- Scopes: Platform-specific (Twitch: EventSub, YouTube: readonly, Kick: PKCE)

## Monitoring & Observability

**Error Tracking:**
- Not detected - No error tracking service (Sentry, DataDog, etc.)
- Application logs sent to stdout/stderr captured by container runtime

**Logs:**
- Structured logging via `go.uber.org/zap` (Go services)
- Winston logger (Node.js services)
- Log level configurable via `LOG_LEVEL` environment variable
- No centralized logging aggregation detected

**Tracing:**
- Distributed tracing via OpenTelemetry
- `go.opentelemetry.io/otel` v1.40.0 (tracing API)
- OTLP gRPC exporter: `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
- Auto-instrumentation available via `@opentelemetry/auto-instrumentations-node` (Node.js)
- No external tracing backend detected (OTEL collector configuration not visible)

**Metrics:**
- Prometheus metrics via `github.com/prometheus/client_golang`
- Metrics endpoint: `/metrics` (standard Prometheus format)
- Service metrics: Request counts, latencies, listener connection states
- No metrics scraper detected (Prometheus server configuration not in codebase)

## CI/CD & Deployment

**Hosting:**
- Kubernetes (production target)
- Docker Compose (local development)
- Manifests: `deployments/` directory

**CI Pipeline:**
- Not detected in codebase
- Makefile targets: `build`, `test`, `test-coverage` (manual local execution)

## Environment Configuration

**Required Environment Variables:**
- OAuth credentials: `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`, `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`
- Bot credentials: `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH`, `DISCORD_BOT_TOKEN`
- Security: `JWT_SECRET`, `TOKEN_ENCRYPTION_KEY`, `YOUTUBE_TOKEN_ENCRYPTION_KEY`
- Database: `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`
- Redis: `REDIS_HOST`, `REDIS_PORT`
- URLs: `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`, `WEBSOCKET_ALLOWED_ORIGINS`
- YouTube quotas: `YOUTUBE_GLOBAL_DAILY_QUOTA`, tier quotas, detection intervals

**Secrets Location:**
- Local: `.env` file (not committed)
- Reference: `.env.example` documents all vars
- Production: Kubernetes Secrets managed via sealed-secrets (not in codebase)

## Webhooks & Callbacks

**Incoming:**
- Twitch chat messages via IRC connection (pull-based, not push)
- YouTube live chat polling (pull-based)
- Kick chat via Pusher WebSocket (push-based)
- TikTok chat via WebSocket (push-based)
- OAuth redirect callbacks: Each auth service has `/oauth/callback` endpoint

**Outgoing:**
- Discord bot sends quota alerts to Discord channel
- No outgoing webhooks to external services detected

## Data Flow Architecture

**Message Pipeline:**

```
Listeners (Twitch IRC, YouTube polling, Kick WebSocket, TikTok WebSocket)
  ↓ publish raw messages
Redis Streams (chat:raw) - Raw, unprocessed platform messages
  ↓ XREADGROUP consumer
Message Processor (services/message-processor)
  ├─ Normalize to unified schema
  ├─ Enrich with emotes (7TV, BTTV, FFZ)
  ├─ Enrich with badges
  └─ Publish to Redis Pub/Sub (overlay:{overlay_id})
    ↓ subscribe
API Gateway WebSocket (services/api-gateway)
  ↓ broadcast per client subscription
Frontend Overlay (React + Next.js)
```

**Token Lifecycle:**

```
OAuth Authorization → Access Token (encrypted in DB)
  ↓
Token Refresh Service (via cron/polling)
  ├─ Check expiry
  ├─ Exchange for new token
  └─ Update encrypted store
  ↓
Listeners use current token for API calls
```

**YouTube Quota Management:**

```
Quota Reserve (before API call) → Confirm/Rollback (after call)
  ├─ Reserve units in quota_reservations table
  ├─ Attempt API call
  ├─ If success: Confirm reservation (debit global pool)
  └─ If fail: Rollback (return units to pool)
```

---

*Integration audit: 2026-02-16*
