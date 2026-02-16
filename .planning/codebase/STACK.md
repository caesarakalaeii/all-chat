# Technology Stack

**Analysis Date:** 2026-02-16

## Languages

**Primary:**
- Go 1.25.6 - Used for all backend microservices (auth, overlay manager, emote service, API gateway, message processor, all listeners, token refresh)
- TypeScript - Used for TikTok listener (`services/tiktok-listener`) and frontend development
- JavaScript - Used for Discord bot service (`services/discord-bot`)

**Secondary:**
- SQL - PostgreSQL database schemas (`migrations/`)
- React/JSX - Frontend UI components

## Runtime

**Environment:**
- Go 1.25.6 (backend services)
- Node.js 18+ (TikTok listener, Discord bot)

**Package Manager:**
- Go modules (`go.mod`/`go.sum`) for all backend services
- npm (Node.js dependency manager for TikTok listener and Discord bot)
- Lockfile: Present (`go.sum`, `package-lock.json` files)

## Frameworks

**Core Backend:**
- Gin v1.11.0 - HTTP web framework for all Go microservices
- gorilla/websocket v1.5.3 - WebSocket server for API Gateway and message distribution

**Frontend:**
- Next.js 16.1.6 - React SSR/SSG framework with App Router
- React 19.2.4 - UI component library

**Database:**
- PostgreSQL 16 (Docker image: `postgres:16-alpine`) - Relational database managed via pgx/v5
- Redis 7 (Docker image: `redis:7-alpine`) - Message broker and caching (Streams and Pub/Sub)

**Testing:**
- testify v1.11.1 - Go testing assertions
- Vitest 4.0.18 - Frontend test runner
- Playwright 1.58.2 - E2E testing for frontend

**Build/Dev:**
- Tailwind CSS 4.1.18 - Utility-first CSS framework
- ESLint 10.0.0 - JavaScript linting
- TypeScript 5.3.3 - Type safety for frontend and Node.js services
- ts-node - TypeScript execution for Node.js

## Key Dependencies

**Critical Backend Libraries:**
- github.com/jackc/pgx/v5 v5.8.0 - PostgreSQL driver (high-performance)
- github.com/redis/go-redis/v9 v9.17.3 - Redis client with Streams and Pub/Sub support
- go.uber.org/zap v1.27.1 - Structured logging framework
- github.com/prometheus/client_golang v1.23.2 - Prometheus metrics client

**Authentication & OAuth:**
- golang.org/x/oauth2 v0.35.0 - OAuth 2.0 client library
- golang.org/x/oauth2/twitch - Twitch OAuth provider
- golang.org/x/oauth2/google - Google OAuth provider
- github.com/golang-jwt/jwt/v5 v5.3.1 - JWT token creation and verification

**API & Streaming:**
- google.golang.org/api v0.266.0 - YouTube API v3 client library
- google.golang.org/grpc v1.78.0 - gRPC framework for service communication
- google.golang.org/protobuf v1.36.11 - Protocol Buffers serialization

**Platform Listeners:**
- github.com/gempir/go-twitch-irc/v4 v4.3.1 - Twitch IRC client (for Twitch IRC-based chat)
- tiktok-live-connector v2.1.0 (npm) - Unofficial TikTok Live chat library
- discord.js v14.14.1 (npm) - Discord bot framework

**Observability & Tracing:**
- go.opentelemetry.io/otel v1.40.0 - OpenTelemetry tracing API
- go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.40.0 - OTLP gRPC exporter for distributed tracing

**Utilities:**
- github.com/google/uuid v1.6.0 - UUID generation
- golang.org/x/time v0.14.0 - Time utilities (rate limiting)
- github.com/gin-contrib/cors v1.7.6 - CORS middleware for Gin

**Frontend Dependencies:**
- zustand 5.0.11 - State management (lightweight alternative to Redux)
- react-hot-toast 2.6.0 - Toast notification system
- lucide-react 0.563.0 - Icon library
- date-fns 4.1.0 - Date manipulation utility
- clsx 2.1.0 - Conditional class name builder

## Configuration

**Environment:**
- `.env` file (not in git) for local development
- `.env.example` file with all required variables documented (at repo root)
- Environment variables loaded via `os.Getenv()` in Go services
- Environment variables loaded via `process.env` in Node.js services

**Key Configuration Variables:**
- `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`, `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH`
- `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, `YOUTUBE_TOKEN_ENCRYPTION_KEY`
- `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`
- `JWT_SECRET`, `TOKEN_ENCRYPTION_KEY`
- `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`
- `REDIS_HOST`, `REDIS_PORT`
- `DISCORD_BOT_TOKEN`, `DISCORD_CHANNEL_ID`
- YouTube quota management: `YOUTUBE_GLOBAL_DAILY_QUOTA`, `YOUTUBE_HIGH_TIER_QUOTA`, tier intervals
- TikTok backoff: `TIKTOK_STATUS_CHECK_CACHE_TTL_MS`, `TIKTOK_POLLER_INTERVAL_MS`

**Build Configuration:**
- `Dockerfile` per service (multi-stage builds, Alpine Linux for minimal images)
- `docker-compose.yml` (`deployments/docker-compose.yml`) - Local development orchestration
- `Makefile` - Development targets and build automation

## Platform Requirements

**Development:**
- Docker & Docker Compose (for local PostgreSQL, Redis, service orchestration)
- Go 1.25.6
- Node.js 18+
- Git

**Production:**
- Kubernetes cluster (with CloudNativePG for PostgreSQL)
- Docker container runtime
- External services: Twitch API, YouTube API, Kick API, Discord (optional for quota monitoring)

**Deployment Target:**
- Kubernetes cluster (via manifests in `deployments/`)
- Docker Compose (local development)

---

*Stack analysis: 2026-02-16*
