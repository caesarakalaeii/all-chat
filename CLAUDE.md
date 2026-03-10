# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project Overview

All-Chat is a **cloud-native microservices platform** for aggregating and displaying chat messages from **multiple live streaming platforms** (Twitch, YouTube, Kick, TikTok) on streaming overlays with support for 7TV, BTTV, and FFZ emotes.

**Core Concept**: Users can create multiple overlays, each configured with one or more chat sources. An overlay can combine messages from Twitch + YouTube + Kick + TikTok simultaneously, providing full flexibility for streamers who multistream.

**Architecture**: Standard Go Layout with microservices communicating via Redis Streams (raw messages) → Message Processor (normalization + enrichment) → Redis Pub/Sub (overlay-specific) → API Gateway WebSocket (client delivery).

**Platform Status**:
- ✅ Twitch (IRC-based) | ✅ YouTube (HTTP polling with quota tracking) | ✅ Kick (Pusher WebSocket) | ✅ TikTok (Unofficial library)

**Current Phase**: Phase 4 complete. All 6 core listener services implemented and production-ready.

---

## Quick Start

```bash
# Full development environment (all services)
make docker-up         # Start postgres, redis, all services
make test              # Run tests
make migrate-up        # Apply database migrations

# Frontend-only development (minimal backend)
make frontend-dev      # Start postgres, redis, gateway, overlay-manager, message-processor
make frontend-seed     # Create test overlay and chat sources
make frontend-messages # Generate mock chat messages
cd frontend && npm run dev  # Start frontend

# Access services
# - API Gateway: http://localhost:8080
# - Frontend: http://localhost:3000
```

**First Time Setup**: See [GETTING_STARTED.md](./GETTING_STARTED.md) for complete onboarding guide.

**Frontend Development**: See [FRONTEND_QUICK_START.md](./FRONTEND_QUICK_START.md) for minimal backend setup (30 seconds).
  - **All-in-one**: `make frontend-quick` (starts services, seeds data, verifies setup)
  - **Full docs**: [FRONTEND_DEV_SETUP.md](./FRONTEND_DEV_SETUP.md)
  - **File index**: [FRONTEND_FILES_INDEX.md](./FRONTEND_FILES_INDEX.md)

---

## Navigation by Task

### Common Tasks → Quick Reference Guides

**Need to...**

| Task | Guide | Lines |
|------|-------|-------|
| Add support for a new platform | [QUICK-REF-ADD-PLATFORM.md](./docs/llm-guides/QUICK-REF-ADD-PLATFORM.md) | ~150 |
| Debug YouTube quota issues | [QUICK-REF-DEBUG-QUOTA.md](./docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md) | ~200 |
| Add a new HTTP endpoint | [QUICK-REF-ADD-ENDPOINT.md](./docs/llm-guides/QUICK-REF-ADD-ENDPOINT.md) | ~100 |
| Perform security audit | [QUICK-REF-SECURITY-AUDIT.md](./docs/llm-guides/QUICK-REF-SECURITY-AUDIT.md) | ~150 |
| Scale services or infrastructure | [QUICK-REF-SCALING.md](./docs/llm-guides/QUICK-REF-SCALING.md) | ~150 |
| Create database migration | [QUICK-REF-DATABASE-MIGRATION.md](./docs/llm-guides/QUICK-REF-DATABASE-MIGRATION.md) | ~100 |
| Debug Kubernetes issues | [QUICK-REF-KUBERNETES-DEBUG.md](./docs/llm-guides/QUICK-REF-KUBERNETES-DEBUG.md) | ~150 |
| Inspect Redis Streams/Pub/Sub | [QUICK-REF-REDIS-OPERATIONS.md](./docs/llm-guides/QUICK-REF-REDIS-OPERATIONS.md) | ~100 |

### Troubleshooting

**Having issues?** Start with the decision tree:

→ [Troubleshooting Decision Tree](./docs/troubleshooting/decision-tree.md) - High-level triage for all common issues

**Detailed troubleshooting guides** (created as needed):
- [build-errors.md](./docs/troubleshooting/build-errors.md) - Go compilation, Docker build, startup failures
- [connection-errors.md](./docs/troubleshooting/connection-errors.md) - PostgreSQL, Redis connection issues
- [youtube-quota-exceeded.md](./docs/troubleshooting/youtube-quota-exceeded.md) - Quota state machine, recovery procedures
- [twitch-irc-issues.md](./docs/troubleshooting/twitch-irc-issues.md) - IRC connection, channel join, message parsing
- [websocket-disconnects.md](./docs/troubleshooting/websocket-disconnects.md) - API Gateway WebSocket issues

### Architecture & Design Decisions

**Understand the system**:
- [Architecture Overview](./docs/architecture/00-OVERVIEW.md) - High-level design, service map
- [Data Flow](./docs/architecture/01-DATA-FLOW.md) - Message processing pipeline
- [Deployment](./docs/architecture/02-DEPLOYMENT.md) - Kubernetes architecture
- [Scaling](./docs/architecture/03-SCALING.md) - Performance and scalability
- [Observability](./docs/architecture/04-OBSERVABILITY.md) - Metrics, logging, tracing
- [Security](./docs/architecture/05-SECURITY.md) - Security architecture

**Understand WHY decisions were made**:
- [ADR Index](./docs/adr/README.md) - Architecture Decision Records
  - ADR-0001: Standard Go Layout (not hexagonal)
  - ADR-0002: Redis Streams + Pub/Sub hybrid
  - ADR-0003: CloudNativePG for PostgreSQL
  - ADR-0004: No ports/adapters abstraction
  - ADR-0005: React + Next.js frontend
  - ADR-0006: YouTube quota reserve-confirm-rollback

### Service Documentation

Each service has a detailed README:
- [api-gateway](./services/api-gateway/README.md) - WebSocket server, HTTP routing
- [auth-service](./services/auth-service/README.md) - OAuth flows, JWT tokens
- [emote-service](./services/emote-service/README.md) - 7TV, BTTV, FFZ emote APIs
- [twitch-listener](./services/twitch-listener/README.md) - IRC client, channel management
- [youtube-listener](./services/youtube-listener/README.md) - HTTP polling, quota tracking
- [kick-listener](./services/kick-listener/README.md) - Pusher WebSocket client
- [tiktok-listener](./services/tiktok-listener/README.md) - Unofficial TikTok Live library
- [message-processor](./services/message-processor/README.md) - Normalization, emote enrichment
- [overlay-manager](./services/overlay-manager/README.md) - Overlay CRUD, source configuration
- [source-manager](./services/source-manager/README.md) - Leader election, active source registry
- [token-refresh-service](./services/token-refresh-service/README.md) - OAuth token refresh

### Development Guides

- [GETTING_STARTED.md](./GETTING_STARTED.md) - Complete navigation guide for LLM agents
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Pull request process, code review guidelines
- [Testing Guide](./docs/development/TESTING_COMPREHENSIVE.md) - Unit, integration, E2E tests

---

## Tech Stack

**Backend**: Go 1.23+, Gin (HTTP), PostgreSQL 16 (pgx/v5), Redis 7 (go-redis/v9), Zap (logging)

**Frontend**: React 18+, Next.js 14+ (App Router), TypeScript, Tailwind CSS

**Infrastructure**: Kubernetes (CNPG for PostgreSQL), Docker Compose (local dev)

**Key Libraries**:
- `gempir/go-twitch-irc/v4` - Twitch IRC client
- `gorilla/websocket` - Kick Pusher WebSocket, API Gateway WebSocket server
- `golang-jwt/jwt/v5` - JWT authentication

---

## Standard Go Service Layout

All services follow this structure:

```
services/<service-name>/
├── cmd/main.go           # Entry point (logger, DB/Redis, HTTP server)
├── handlers/             # HTTP handlers (Gin)
├── models/               # Data models
├── repository/           # Database layer (if needed)
├── <domain-packages>/    # Domain logic (oauth/, streams/, channels/, etc.)
├── go.mod                # Dependencies
└── Dockerfile            # Container image
```

**Key Principles**:
- Clear separation: handlers → domain logic → repository
- Dependency injection for testability
- Graceful shutdown (25s timeout)
- Health checks: `/health/live` (always 200), `/health/ready` (checks DB + Redis)

---

## Message Flow Architecture

```
Listeners (Twitch/YouTube/Kick/TikTok)
  ↓ publish raw messages
Redis Streams (chat:raw)
  ↓ consume via XREADGROUP
Message Processor
  ├─ Normalize (platform → unified format)
  ├─ Enrich (7TV, BTTV, FFZ emotes)
  └─ Publish to Redis Pub/Sub (overlay:{overlay_id})
    ↓ subscribe
API Gateway WebSocket
  ↓ broadcast
Frontend Overlay (React)
```

**Unified Message Format**: All platforms normalized to common schema with user, message, emotes, badges, metadata.

**See**: [Data Flow Integration](./docs/architecture/01-DATA-FLOW.md) for complete details.

---

## Environment Variables

**Required for local development**:

```bash
# Twitch
TWITCH_BOT_USERNAME=your_bot
TWITCH_BOT_OAUTH=oauth:token_from_twitchapps

# YouTube
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxx

# Database (localhost defaults)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password

# Redis (localhost defaults)
REDIS_HOST=localhost
REDIS_PORT=6379
```

**See**: `.env.example` for complete list. **Kubernetes**: Secrets managed via sealed-secrets.

---

## Known Issues & Technical Debt

### Security
- Token encryption is basic (TODO: implement AES-GCM)
- CORS allows `*` in dev (configure for production)
- No service-to-service auth (Kubernetes NetworkPolicies only)

### Testing
- Integration tests incomplete for YouTube Listener
- E2E tests needed for full message flow

### Scalability
- Shared PostgreSQL database (consider separate per service)
- YouTube quota limit increased to 1,009,000 units/day (was 10,000)

**See**: [CRITICAL_ARCHITECTURE_ANALYSIS.md](./docs/phase-reports/CRITICAL_ARCHITECTURE_ANALYSIS.md) for historical security audit.

---

## Production Readiness

**Services ready for production**:
- ✅ API Gateway, Twitch Listener, YouTube Listener, Kick Listener, Message Processor

**Production considerations**:
- Request YouTube API quota increase (1,000,000 units/day)
- Configure CORS for production domain
- Set up monitoring (Prometheus + Grafana)
- Enable TLS for all services
- Implement rate limiting on API Gateway

**See**: [Deployment Guide](./docs/architecture/02-DEPLOYMENT.md) for Kubernetes manifests and configuration.

---

## External Resources

- **Twitch OAuth**: https://twitchapps.com/tmi/
- **YouTube API Console**: https://console.developers.google.com/
- **7TV API**: https://7tv.io/docs
- **BTTV API**: https://betterttv.com/developers
- **FFZ API**: https://www.frankerfacez.com/developers

---

## Need More Help?

1. **Start with**: [GETTING_STARTED.md](./GETTING_STARTED.md) - Complete navigation guide
2. **Troubleshooting**: [Decision Tree](./docs/troubleshooting/decision-tree.md) - Triage common issues
3. **Architecture**: [ADR Index](./docs/adr/README.md) - Understand design decisions
4. **Service Details**: [services/*/README.md](./services/) - Service-specific documentation
5. **Quick Tasks**: [docs/llm-guides/](./docs/llm-guides/) - Task-oriented quick references

**Database Note**: PostgreSQL is deployed in Kubernetes cluster (namespace: allchat) as a CNPG (CloudNativePG) cluster with automated failover and backups.
