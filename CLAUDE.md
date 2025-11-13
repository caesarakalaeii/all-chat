# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Navigation Guide for LLM Agents

**IMPORTANT**: Before starting any task, read [GETTING_STARTED.md](./GETTING_STARTED.md) for:
- Quick reference to important files
- Repository structure overview
- Service-specific navigation guides
- Common tasks and where to find relevant code
- Development workflow best practices

The GETTING_STARTED.md file is your map for navigating this repository efficiently. Use it to quickly locate:
- Which files to read for specific tasks
- Where services are located and what they do
- Documentation for architecture, deployment, and testing
- Known issues and technical debt

## Project Overview

All-Chat is a cloud-native microservices platform for aggregating and displaying chat messages from **multiple live streaming platforms** (Twitch, YouTube, Kick, TikTok) on streaming overlays with support for 7TV, BTTV, and FFZ emotes. The project follows **Standard Go Layout** with clear service boundaries for maintainability and testability.

**Core Concept**: Users can create multiple overlays, each configured with one or more chat sources. An overlay can combine messages from Twitch + YouTube + Kick simultaneously, or be configured with a single source - providing full flexibility for streamers who multistream or want unified chat displays.

**Platform Priority**: Initial focus is Twitch and YouTube, with Kick and TikTok support planned for future releases.

**Current Status**: ~90% Phase 4 complete. All 5 core services implemented (API Gateway, Twitch Listener, YouTube Listener, Message Processor, Source Manager). Twitch integration fully tested and working. YouTube integration pending testing.

## Build & Test Commands

```bash
# Build all services
make build

# Run tests
make test              # All tests
make test-coverage     # With coverage report
go test -v ./services/api-gateway/...  # Single service tests

# Code quality
make fmt               # Format code
make lint              # Run golangci-lint
make deps              # Download dependencies

# Docker Compose (recommended for development)
make docker-up         # Start all services
make docker-down       # Stop all services
make docker-logs       # View logs
make docker-restart    # Restart services

# Database
make migrate-up        # Run migrations (requires PostgreSQL running)
```

## Architecture Principles

### Standard Go Service Layout

All services follow the standard Go project layout:
```
services/<service-name>/
├── cmd/               # Application entry points
│   └── main.go       # Service initialization
├── handlers/          # HTTP handlers (Gin)
├── models/           # Data models and domain entities
├── repository/       # Database layer (optional)
├── <domain-packages>/ # Domain-specific packages (e.g., oauth/, streams/, channels/)
├── go.mod            # Module dependencies
└── Dockerfile        # Container image
```

**Key Principles**:
- Clear separation of concerns via packages
- Dependency injection for testability
- Domain logic in dedicated packages (e.g., `oauth/`, `streams/`, `channels/`)
- HTTP handlers separate from business logic
- Database code in `repository/` packages when needed
- Each service is independently deployable

### Service Communication

- **Synchronous**: HTTP/REST between services (via API Gateway)
- **Asynchronous**: Redis Streams + Pub/Sub for real-time messages
  - Listeners publish raw messages to Redis Stream `chat:raw`
  - Message Processor consumes from stream, enriches, and publishes to `overlay:{overlay_id}` via Pub/Sub
  - API Gateway WebSocket subscribes to overlay-specific channels
- **Database**: Shared PostgreSQL database (separate schemas per service)
- **Caching**: Redis for emotes, session data, and leader election

### Unified Message Format

All chat messages from different platforms are normalized to a common format before publishing to Redis:

```json
{
  "id": "uuid",
  "overlay_id": "uuid",
  "platform": "twitch|youtube|kick|tiktok",
  "channel_id": "platform-specific-id",
  "channel_name": "display-name",
  "user": {
    "id": "platform-user-id",
    "username": "username",
    "display_name": "Display Name",
    "avatar_url": "https://...",
    "badges": ["subscriber", "moderator"],
    "color": "#FF0000"
  },
  "message": {
    "text": "Hello world!",
    "emotes": [
      {
        "code": "Kappa",
        "provider": "twitch",
        "url": "https://...",
        "positions": [[0, 5]]
      }
    ]
  },
  "timestamp": "2025-11-10T12:34:56Z",
  "metadata": {
    "is_subscriber": true,
    "is_moderator": false,
    "bits": 0,
    "super_chat_amount": 0
  }
}
```

This unified format allows the frontend to display messages from all platforms consistently.

## Tech Stack Details

### Backend Dependencies
- **Go 1.23+** (required)
- **Gin** (`gin-gonic/gin`) - HTTP framework
- **PostgreSQL 16** with `pgx/v5` - Database
- **Redis 7** (`go-redis/v9`) - Cache & Pub/Sub
- **Twitch IRC** (`gempir/go-twitch-irc/v4`) - Chat listener
- **JWT** (`golang-jwt/jwt/v5`) - Auth tokens
- **Zap** (`uber-go/zap`) - Structured logging

### Frontend Stack
- **React 18+** - UI library
- **Next.js 14+** (App Router) - SSR framework
- **TypeScript** - Type safety
- **Tailwind CSS** - Styling
- **WebSocket API** - Real-time message streaming

### Shared Packages (`shared/`)
- `shared/auth/` - JWT utilities (Generate, Validate, Claims)
- `shared/database/` - PostgreSQL connection pooling
- `shared/redis/` - Redis client wrapper
- `shared/logger/` - Zap logger initialization
- `shared/middleware/` - HTTP middleware (CORS, Auth)

## Service Details

### API Gateway (Port 8080) ✅
**Purpose**: Entry point, WebSocket management, HTTP routing

**Key Files**:
- `services/api-gateway/cmd/main.go` - Entry point
- `services/api-gateway/handlers/` - HTTP handlers
- `services/api-gateway/websocket/` - WebSocket server implementation

**Features**:
- HTTP reverse proxy to backend services
- WebSocket server for overlay connections (`ws://localhost:8080/ws/overlay/:id`)
- Subscribe to Redis pub/sub channels (`overlay:{overlay_id}`)
- Broadcast messages to WebSocket clients
- Connection pooling per overlay
- CORS and request logging
- Health aggregation

**Documentation**: See `services/api-gateway/README.md`

### Twitch Listener (Port 8085) ✅
**Purpose**: Connect to Twitch IRC, listen to chat, publish raw messages to Redis Streams

**Key Files**:
- `services/twitch-listener/cmd/main.go` - Entry point
- `services/twitch-listener/irc/client.go` - Twitch IRC client implementation
- `services/twitch-listener/channels/manager.go` - Dynamic channel join/leave management
- `services/twitch-listener/publisher/redis.go` - Publishes to Redis Stream `chat:raw`

**Features**:
- Twitch IRC connection (gempir/go-twitch-irc)
- Dynamic channel JOIN/PART
- Rate limiting (20 JOIN/10s per Twitch limits)
- Publishes to Redis Streams (`chat:raw`)
- Automatic reconnection
- Health checks and status

**Environment**:
- `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH` (required)

**Documentation**: See `services/twitch-listener/README.md`

### YouTube Listener (Port 8086) ✅
**Purpose**: Poll YouTube Live Chat API, publish raw messages to Redis Streams

**Key Files**:
- `services/youtube-listener/cmd/main.go` - Entry point with leader election
- `services/youtube-listener/youtube/client.go` - YouTube API client wrapper
- `services/youtube-listener/streams/poller.go` - Live chat polling logic (2-5s interval)
- `services/youtube-listener/channels/manager.go` - Active stream tracking
- `services/youtube-listener/publisher/redis.go` - Publishes to Redis Stream `chat:raw`

**Features**:
- YouTube Live Chat API polling
- OAuth 2.0 per-user authentication
- Adaptive polling intervals (2-5 seconds)
- Quota tracking (10,000 units/day default)
- Leader election (prevents duplicate polling)
- Publishes to Redis Streams (`chat:raw`)
- Health checks and status

**Environment**:
- `YOUTUBE_API_KEY`, `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET` (required)

**Documentation**: See `services/youtube-listener/README.md`

### Message Processor (Port 8087) ✅
**Purpose**: Consume messages from Redis Streams, normalize, enrich with emotes, publish to overlay-specific Pub/Sub

**Key Files**:
- `services/message-processor/cmd/main.go` - Consumer group initialization
- `services/message-processor/consumer/streams.go` - Redis Streams XREADGROUP consumer
- `services/message-processor/normalizer/twitch_normalizer.go` - Twitch message parsing
- `services/message-processor/normalizer/youtube_normalizer.go` - YouTube message parsing
- `services/message-processor/enricher/emote_enricher.go` - Emote enrichment
- `services/message-processor/publisher/pubsub.go` - Publishes to `overlay:{overlay_id}`
- `services/message-processor/router/router.go` - Routes messages based on platform

**Features**:
- Redis Streams consumer (consumer group `message-processors`)
- Message normalization (Twitch + YouTube → Unified format)
- Emote enrichment (7TV, BTTV, FFZ via external APIs)
- Platform detection and routing
- Overlay-specific publishing via Redis Pub/Sub
- Health checks and status

**Documentation**: See `services/message-processor/README.md`

### Source Manager (Port 8088) ✅
**Purpose**: Active source registry and Redis-based leader election for YouTube polling

**Key Files**:
- `services/source-manager/cmd/main.go` - Entry point
- `services/source-manager/registry/` - Active source tracking
- `services/source-manager/election/` - Leader election implementation
- `services/source-manager/handlers/` - HTTP API for leadership

**Features**:
- Active source registry (syncs from database)
- Redis-based leader election (prevents duplicate YouTube polling)
- Leadership API (claim, renew, release)
- Health checks and status
- Coordination between YouTube Listener instances

**Environment**:
- `REDIS_HOST`, `REDIS_PORT` (required)
- `DATABASE_*` variables for source registry sync

## Environment Variables

See `.env.example` for template. Key variables:

```bash
# Twitch IRC Bot (required for Twitch Listener)
TWITCH_BOT_USERNAME=
TWITCH_BOT_OAUTH=oauth:...

# YouTube API (required for YouTube Listener)
YOUTUBE_API_KEY=
YOUTUBE_CLIENT_ID=
YOUTUBE_CLIENT_SECRET=

# Database (defaults for local dev)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis (defaults for local dev)
REDIS_HOST=localhost
REDIS_PORT=6379
```

## Database Schema

The application uses PostgreSQL for persistent data storage. Key tables used by the services:

**Active Sources** (`active_sources` or similar):
- Tracks which chat sources are currently active
- Used by Source Manager for registry and leader election
- Used by Listeners to know which channels/streams to connect to
- Fields typically include: `overlay_id`, `platform`, `channel_id`, `is_active`

**Configuration Storage**:
- Overlay configurations and chat source mappings
- Referenced by services to determine what to listen to
- Platform-specific settings (polling intervals, auth tokens, etc.)

**Migrations**:
See `migrations/` directory for database schema evolution. The schema supports:
- Multiple streaming platforms (Twitch, YouTube, Kick, TikTok)
- Multi-source overlays (one overlay can aggregate multiple chat sources)
- Platform-specific configuration (JSONB fields for flexibility)

## Common Development Patterns

### Adding a New Endpoint

1. Implement business logic in appropriate domain package (e.g., `oauth/`, `streams/`)
2. Add HTTP handler function in `handlers/<feature>.go`
3. Register route in `cmd/main.go` or handler initialization
4. Add tests in `handlers/<feature>_test.go`

### Adding a New Service

1. Create directory structure following standard Go layout in `services/<service-name>/`
2. Copy `services/<existing-service>/cmd/main.go` as template
3. Follow patterns: logger initialization, DB/Redis connection, health checks
4. Organize domain logic in dedicated packages (e.g., `oauth/`, `streams/`, `channels/`)
5. Add Dockerfile at `services/<service-name>/Dockerfile`
6. Add deployment manifests in `deployments/k8s/base/<service-name>/`
7. Add Make targets to `Makefile`

### Error Handling

- Use custom errors defined in services (e.g., `ErrOverlayNotFound`)
- Log errors with `logger.Error()` before returning
- Return appropriate HTTP status codes in handlers
- Never expose internal errors to clients

### Graceful Shutdown

All services implement:
```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
// 25-second timeout for graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
srv.Shutdown(ctx)
```

## Health Checks

All services expose:
- `GET /health/live` - Liveness probe (always returns 200)
- `GET /health/ready` - Readiness probe (checks DB + Redis)

Kubernetes uses these for rolling updates and self-healing.

## Deployment

### Local Development (Docker Compose)
```bash
make docker-up    # Starts: postgres, redis, all services
make docker-logs  # Watch logs
```

Services accessible at:
- API Gateway: `http://localhost:8080`
- Twitch Listener: `http://localhost:8085`
- YouTube Listener: `http://localhost:8086`
- Message Processor: `http://localhost:8087`
- Source Manager: `http://localhost:8088`

### Kubernetes
```bash
kubectl apply -f deployments/k8s/namespace.yaml
# Create secrets (see README.md)
kubectl apply -f deployments/k8s/configmaps/
kubectl apply -f deployments/k8s/<service>/
```

Each service has:
- Deployment with resource limits
- Service (ClusterIP)
- HorizontalPodAutoscaler (CPU-based scaling)

## Known Issues & TODOs

### Security
- Token encryption is basic (TODO: implement AES-GCM)
- No rate limiting implemented (TODO)
- CORS currently allows `*` in dev (configure for production)

### Implementation TODOs
- [ ] Complete integration testing for YouTube Listener
- [ ] Add Listener adapters for Kick and TikTok (Phase 2)
- [ ] Build React + Next.js frontend for overlay display and configuration
- [ ] Add Prometheus metrics endpoint to all services
- [ ] Implement distributed tracing (OpenTelemetry)
- [ ] Add comprehensive unit/integration tests
- [ ] Implement authentication/authorization for production use
- [ ] Add overlay management API (CRUD operations)

### Scalability TODOs
- [ ] Separate databases per service
- [ ] Implement API Gateway rate limiting
- [ ] Add WebSocket connection pooling
- [ ] Implement message queue for high-volume channels

## Troubleshooting

**Build Errors**:
- Run `make deps` to ensure dependencies are up to date
- Check Go version: `go version` (requires 1.23+)

**Connection Errors**:
- Verify PostgreSQL/Redis are running: `make docker-up` or `docker-compose up postgres redis -d`
- Check `.env` file has correct credentials
- Test DB connection: `psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat`

**Migration Errors**:
- Ensure database exists: `createdb allchat`
- Run migrations: `make migrate-up`
- Check `migrations/001_initial_schema.sql` for schema

**Twitch Listener Errors**:
- Verify Twitch bot OAuth token is valid (get from https://twitchapps.com/tmi/)
- Check bot username matches token
- Ensure channels exist and are spelled correctly

**YouTube Listener Errors**:
- Verify YouTube API credentials are valid
- Check API quota hasn't been exceeded (10,000 units/day default)
- Ensure video IDs are live streams with chat enabled

## Resources

- **Twitch IRC OAuth**: https://twitchapps.com/tmi/
- **Twitch Developer Console**: https://dev.twitch.tv/console/apps
- **YouTube Live Chat API**: https://developers.google.com/youtube/v3/live/docs
- **YouTube API Console**: https://console.developers.google.com/
- **7TV API**: https://7tv.io/docs
- **BTTV API**: https://betterttv.com/developers
- **FFZ API**: https://www.frankerfacez.com/developers
- **React Docs**: https://react.dev
- **Next.js Docs**: https://nextjs.org/docs

## Additional Navigation Help

For comprehensive navigation guidance, file locations, and task-specific instructions, see:
- **[GETTING_STARTED.md](./GETTING_STARTED.md)** - Complete LLM agent navigation guide

This guide includes:
- Quick links to all important files
- Service-specific navigation (what each service does and where to find code)
- Common tasks and where to look
- Architecture documentation map
- Development workflows and commands
- Known issues and technical debt references
