# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

All-Chat is a cloud-native microservices platform for aggregating and displaying chat messages from **multiple live streaming platforms** (Twitch, YouTube, Kick, TikTok) on streaming overlays with support for 7TV, BTTV, and FFZ emotes. The project follows **Hexagonal Architecture** (Ports & Adapters) for maintainability and testability.

**Core Concept**: Users can create multiple overlays, each configured with one or more chat sources. An overlay can combine messages from Twitch + YouTube + Kick simultaneously, or be configured with a single source - providing full flexibility for streamers who multistream or want unified chat displays.

**Platform Priority**: Initial focus is Twitch and YouTube, with Kick and TikTok support planned for future releases.

**Current Status**: ~60% complete. Auth Service, Overlay Manager, and Emote Service are fully implemented. Multi-source Chat Listener and API Gateway are pending.

## Build & Test Commands

```bash
# Build all services
make build

# Build individual services
make build-auth         # Auth Service
make build-overlay      # Overlay Manager
make build-emote        # Emote Service
make build-chat         # Chat Listener (TODO)
make build-api-gateway  # API Gateway (TODO)

# Run tests
make test              # All tests
make test-coverage     # With coverage report
go test -v ./internal/auth-service/...  # Single service tests

# Run individual services locally (requires PostgreSQL & Redis running)
make run-auth
make run-overlay
make run-emote

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

### Hexagonal Architecture Structure

All services follow this pattern:
```
internal/<service-name>/
├── adapters/           # External implementations
│   ├── api/           # HTTP handlers (Gin)
│   └── repository/    # Database implementations (pgx)
└── core/              # Business logic (no external dependencies)
    ├── domain/        # Entities and value objects
    ├── ports/         # Interfaces (Service & Repository)
    └── services/      # Business logic implementations
```

**Key Rules**:
- Core domain never imports adapters
- All external dependencies injected via ports (interfaces)
- Domain models are in `core/domain/`
- Business logic is in `core/services/`
- HTTP handlers are in `adapters/api/`
- Database code is in `adapters/repository/`

### Service Communication

- **Synchronous**: HTTP/REST between services (via API Gateway)
- **Asynchronous**: Redis Pub/Sub for real-time messages
  - Chat Listener publishes to `overlay:{overlay_id}`
  - API Gateway WebSocket subscribes to channels
- **Database**: Each service has its own PostgreSQL schema/tables (future: separate DBs)
- **Caching**: Redis for emotes and session data

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

### Core Dependencies
- **Go 1.23+** (required)
- **Gin** (`gin-gonic/gin`) - HTTP framework
- **PostgreSQL 16** with `pgx/v5` - Database
- **Redis 7** (`go-redis/v9`) - Cache & Pub/Sub
- **Twitch IRC** (`gempir/go-twitch-irc/v4`) - Chat listener
- **JWT** (`golang-jwt/jwt/v5`) - Auth tokens
- **Zap** (`uber-go/zap`) - Structured logging

### Shared Packages (`pkg/`)
- `pkg/auth/` - JWT utilities (Generate, Validate, Claims)
- `pkg/database/` - PostgreSQL connection pooling
- `pkg/redis/` - Redis client wrapper
- `pkg/logger/` - Zap logger initialization
- `pkg/middleware/` - HTTP middleware (CORS, Auth)

## Service Details

### Auth Service (Port 8081) ✅
**Purpose**: Twitch OAuth & JWT management

**Key Files**:
- `cmd/auth-service/main.go` - Entry point
- `internal/auth-service/core/services/auth_service.go` - OAuth flow
- `internal/auth-service/adapters/api/handlers.go` - Endpoints

**Endpoints**:
- `GET /auth/login` - Start OAuth flow
- `GET /auth/callback` - OAuth callback
- `POST /auth/refresh` - Refresh tokens
- `GET /auth/me` - Get current user
- `POST /auth/logout` - Logout

**Environment**:
- `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET` (required)
- `TWITCH_REDIRECT_URL` (default: `http://localhost:8080/api/v1/auth/callback`)
- `JWT_SECRET` (required for production)

### Overlay Manager (Port 8082) ✅
**Purpose**: CRUD operations for overlays and configurations

**Key Files**:
- `cmd/overlay-manager/main.go` - Entry point
- `internal/overlay-manager/core/services/overlay_service.go` - Business logic
- `internal/overlay-manager/adapters/repository/postgres_overlay_repository.go` - Persistence

**Endpoints**:
- `GET/POST /overlays` - List/Create overlays
- `GET/PUT/DELETE /overlays/:id` - Get/Update/Delete overlay
- `GET/PUT /overlays/:id/config` - Get/Update configuration
- `GET /overlays/:id/sources` - List chat sources for overlay
- `POST /overlays/:id/sources` - Add chat source to overlay
- `DELETE /overlays/:id/sources/:source_id` - Remove chat source from overlay
- `PUT /overlays/:id/sources/:source_id` - Update chat source configuration

**Database Tables**:
- `overlays` - Overlay metadata (one-to-many with users)
- `overlay_chat_sources` - Chat sources for each overlay (many-to-many relationship)
- `overlay_configs` - Display and filter configuration (one-to-one with overlays)

### Emote Service (Port 8083) ✅
**Purpose**: Fetch & cache emotes from 7TV, BTTV, FFZ

**Key Files**:
- `cmd/emote-service/main.go` - Entry point
- `internal/emote-service/core/services/emote_service.go` - Caching logic
- `internal/emote-service/adapters/clients/emote_clients.go` - External API clients

**Endpoints**:
- `GET /emotes/channel/:channel` - Get all emotes for channel
- `GET /emotes/7tv/:channel` - 7TV emotes only
- `GET /emotes/bttv/:channel` - BTTV emotes only
- `GET /emotes/ffz/:channel` - FFZ emotes only

**Caching**: Uses Redis with TTL (e.g., `emotes:7tv:{channel}`)

### Chat Listener (TODO) ⏳
**Purpose**: Connect to multiple live streaming platforms, normalize messages, enrich with emotes, publish to Redis

**Architecture**:
- **Plugin-based design**: Each streaming platform (Twitch, YouTube, Kick, TikTok) is a separate adapter
- **Unified message format**: All messages normalized to common structure regardless of source
- **Dynamic source management**: Connect/disconnect from sources based on active overlay configurations

**Expected Flow**:
1. Poll database for active overlay chat sources
2. Connect to each platform's API/IRC (Twitch IRC, YouTube Live Chat API, Kick WebSocket, TikTok API)
3. Join channels/streams based on active overlays
4. Parse incoming messages and normalize to unified format
5. Enrich with emotes from Emote Service (platform-specific emotes)
6. Publish to Redis channels (`overlay:{overlay_id}`) - messages from all sources for that overlay
7. Handle reconnections and rate limits per platform

**Supported Sources**:
- **Twitch** (Phase 1): IRC connection (gempir/go-twitch-irc) ✅ Priority
- **YouTube** (Phase 1): Live Chat API (requires OAuth) ✅ Priority
- **Kick** (Phase 2): WebSocket/API connection
- **TikTok** (Phase 2): Live Chat API

**Environment**:
- `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH` (required for Twitch)
- `YOUTUBE_API_KEY`, `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET` (required for YouTube)
- `KICK_API_TOKEN` (required for Kick - future)
- `TIKTOK_API_KEY` (required for TikTok - future)

### API Gateway (Port 8080) (TODO) ⏳
**Purpose**: Entry point, WebSocket management, HTTP routing

**Expected Features**:
- HTTP reverse proxy to backend services
- WebSocket server for overlay connections
- Subscribe to Redis pub/sub channels
- Broadcast messages to WebSocket clients
- Serve static frontend files

**WebSocket**: `ws://localhost:8080/ws/overlay/:id?token=JWT`

## Environment Variables

See `.env.example` for template. Key variables:

```bash
# Twitch OAuth (required)
TWITCH_CLIENT_ID=
TWITCH_CLIENT_SECRET=
TWITCH_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback

# Twitch IRC Bot (required for Chat Listener)
TWITCH_BOT_USERNAME=
TWITCH_BOT_OAUTH=oauth:...

# JWT (required)
JWT_SECRET=

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

**Users Table** (`users`):
- Stores Twitch user info
- OAuth tokens encrypted at rest (basic encryption, TODO: AES-GCM)
- Indexed on `twitch_id` and `username`

**Overlays Table** (`overlays`):
- One-to-many with users (cascade delete)
- `is_active` flag for soft enable/disable
- Indexed on `user_id` and `is_active`

**Overlay Chat Sources Table** (`overlay_chat_sources`):
- Many-to-one with overlays (cascade delete)
- Supports multiple sources per overlay
- Fields: `platform` (twitch, youtube, discord, kick, irc), `channel_id`, `auth_required`
- Each source can have platform-specific configuration in JSONB field
- Indexed on `overlay_id` and `platform`

**Overlay Configs Table** (`overlay_configs`):
- One-to-one with overlays (cascade delete)
- JSONB fields: `display_settings`, `filter_settings`
- Emote flags: `enable_7tv`, `enable_bttv`, `enable_ffz`
- NO LONGER stores single `twitch_channel` - replaced by `overlay_chat_sources` table

**Supported Platforms Table** (`supported_platforms`):
- Registry of available streaming platforms
- Fields: `platform`, `display_name`, `is_enabled`, `requires_oauth`, `config_schema`
- Initial platforms: Twitch (enabled), YouTube (enabled), Kick (disabled), TikTok (disabled)

**Migrations**:
- `migrations/001_initial_schema.sql` - Initial schema (single Twitch channel per overlay)
- `migrations/002_add_multi_source_support.sql` - Multi-source support (BREAKING CHANGE)
  - Adds `overlay_chat_sources` table
  - Removes `twitch_channel` from `overlay_configs`
  - Renames `active_channels` to `active_platform_channels` with platform support
  - Adds `supported_platforms` registry table

## Common Development Patterns

### Adding a New Endpoint

1. Define method in `core/ports/service.go` interface
2. Implement in `core/services/<service>_service.go`
3. Add handler in `adapters/api/handlers.go`
4. Register route in handler's `RegisterRoutes` method

### Adding a New Service

1. Create directory structure following hexagonal architecture
2. Copy `cmd/<existing-service>/main.go` as template
3. Follow patterns: logger initialization, DB/Redis connection, health checks
4. Add Dockerfile in `deployments/docker/<service>.Dockerfile`
5. Add deployment manifests in `deployments/k8s/<service>/`
6. Add Make targets to `Makefile`

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
- Auth: `http://localhost:8081`
- Overlay: `http://localhost:8082`
- Emote: `http://localhost:8083`

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
- [ ] Apply multi-source migration (`002_add_multi_source_support.sql`)
- [ ] Update Overlay Manager to support chat sources CRUD endpoints
- [ ] Complete Chat Listener service (Phase 1: Twitch + YouTube)
- [ ] Complete API Gateway service with WebSocket multi-source support
- [ ] Build Svelte 5 frontend with multi-source configuration UI
- [ ] Add Chat Listener adapters for Kick and TikTok (Phase 2)
- [ ] Add Prometheus metrics endpoint
- [ ] Implement distributed tracing (OpenTelemetry)
- [ ] Add comprehensive unit/integration tests
- [ ] Set up CI/CD pipeline (GitHub Actions recommended)

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

**OAuth Errors**:
- Verify Twitch app redirect URL matches `.env` (`TWITCH_REDIRECT_URL`)
- Ensure Client ID and Secret are correct
- Check Twitch app is not rate-limited

## Resources

- **Twitch Developer Console**: https://dev.twitch.tv/console/apps
- **Twitch IRC OAuth**: https://twitchapps.com/tmi/
- **7TV API**: https://7tv.io/docs
- **BTTV API**: https://betterttv.com/developers
- **FFZ API**: https://www.frankerfacez.com/developers
- **SVELTE Docs**: @.resources/svelte/docs
