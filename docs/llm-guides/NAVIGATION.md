# LLM Agent Navigation Guide

**Purpose**: Quick navigation to locate relevant files for any task in All-Chat repository.

**Last Updated**: 2026-05-27

---

## Start Here

**First time in this repo?** Read in this order:
1. **[CLAUDE.md](../../CLAUDE.md)** - Project overview, architecture, navigation hub
2. **This file** (NAVIGATION.md) - Detailed service-by-service navigation
3. **[Architecture Overview](../architecture/00-OVERVIEW.md)** - System architecture deep dive

---

## Service-Specific Navigation

### API Gateway (`:8080`) ✅

**Purpose**: HTTP reverse proxy + WebSocket hub for overlay connections

**Key Files**:
- `services/api-gateway/cmd/main.go` - Entry point, route registration
- `services/api-gateway/websocket/manager.go` - WebSocket connection management, Redis Pub/Sub subscription
- `services/api-gateway/handlers/` - HTTP handlers for proxying to backend services
- `services/api-gateway/sessions/manager.go` - Session management (WebSocket TTL)

**Read When**: Working on WebSocket connections, HTTP routing, CORS, or reverse proxy logic.

**→ Full Docs**: [services/api-gateway/README.md](../../services/api-gateway/README.md)

---

### Auth Service (`:8081`) ✅

**Purpose**: OAuth flows (Twitch, YouTube, Kick) + JWT token issuance

**Key Files**:
- `services/auth-service/cmd/main.go` - Entry point, route registration
- `services/auth-service/handlers/auth.go` - OAuth callbacks, login/logout
- `services/auth-service/oauth/` - Platform-specific OAuth logic (twitch.go, youtube.go, kick.go)
- `services/auth-service/repository/` - User and token persistence

**Read When**: Working on OAuth flows, JWT tokens, user authentication.

**→ Full Docs**: [services/auth-service/README.md](../../services/auth-service/README.md)

---

### Overlay Manager (`:8082`) ✅

**Purpose**: Overlay CRUD operations + multi-source chat configuration

**Key Files**:
- `services/overlay-manager/cmd/main.go` - Entry point
- `services/overlay-manager/handlers/config.go` - Overlay CRUD endpoints
- `services/overlay-manager/handlers/mock_message.go` - Mock chat injection for testing
- `services/overlay-manager/creditroll/` - Credit roll configuration
- `services/overlay-manager/repository/` - Database access for overlays, sources
- `services/overlay-manager/handlers/youtube.go` - YouTube @handle → channel ID resolution

**Read When**: Working on overlay configuration, chat source management, mock chat, or YouTube channel resolution.

**→ Full Docs**: [services/overlay-manager/README.md](../../services/overlay-manager/README.md)

---

### Emote Service (`:8083`) ✅

**Purpose**: Aggregate and cache emotes from 7TV, BTTV, FFZ

**Key Files**:
- `services/emote-service/cmd/main.go` - Entry point
- `services/emote-service/clients/` - 7TV, BTTV, FFZ API clients
- `services/emote-service/cache/` - Cache layer (Redis-backed)
- `services/emote-service/handlers/` - HTTP API for emote resolution

**Read When**: Working on emote providers, caching strategy, or emote API integration.

**→ Full Docs**: [services/emote-service/README.md](../../services/emote-service/README.md)

---

### Twitch Listener (`:8085`) ✅

**Purpose**: Connect to Twitch IRC, join channels, publish messages to Redis Streams

**Key Files**:
- `services/twitch-listener/cmd/main.go` - Entry point, IRC client initialization
- `services/twitch-listener/irc/connection.go` - Twitch IRC connection wrapper (gempir/go-twitch-irc)
- `services/twitch-listener/irc/parser.go` / `event_parser.go` - PRIVMSG/event parsing
- `services/twitch-listener/channels/manager.go` - Dynamic channel JOIN/PART management
- `services/twitch-listener/publisher/stream_publisher.go` - Publish to Redis Stream `chat:raw`
- `services/twitch-listener/zombie/detector.go` - Zombie liveness detector (see ADR-0011)

**Read When**: Working on Twitch IRC integration, channel management, or IRC message parsing.

**→ Full Docs**: [services/twitch-listener/README.md](../../services/twitch-listener/README.md)

---

### YouTube Listener (`:8086`) ✅

**Purpose**: Poll YouTube Live Chat API, manage quota, publish messages to Redis Streams

**Key Files**:
- `services/youtube-listener/cmd/main.go` - Entry point with leader election
- `services/youtube-listener/api/client.go` - YouTube API client wrapper (also `grpc_client.go`)
- `services/youtube-listener/streams/poller.go` - Live chat polling (2-5s intervals)
- `services/youtube-listener/streams/manager.go` - Active stream tracking
- `services/youtube-listener/quota/tracker.go` - **Reserve-confirm-rollback quota tracking**
- `services/youtube-listener/publisher/stream_publisher.go` - Publish to Redis Stream `chat:raw`

**Read When**: Working on YouTube integration, quota management, leader election, or API polling.

**→ Full Docs**: [services/youtube-listener/README.md](../../services/youtube-listener/README.md)

---

### Message Processor (`:8087`) ✅

**Purpose**: Normalize, enrich, and route messages from all platforms

**Key Files**:
- `services/message-processor/cmd/main.go` - Consumer group initialization
- `services/message-processor/consumer/stream_consumer.go` - Redis Streams XREADGROUP consumer
- `services/message-processor/consumer/dlq.go` / `retry.go` - DLQ and retry handling
- `services/message-processor/normalizer/` - Platform-specific normalizers (twitch, youtube, kick, tiktok, discord)
- `services/message-processor/enricher/emote_enricher.go` - Emote enrichment pipeline
- `services/message-processor/enricher/pronoun_enricher.go` - Alejo pronoun enricher (ADR-0010)
- `services/message-processor/publisher/pubsub_publisher.go` - Publish to Redis Pub/Sub `overlay:{id}`
- `services/message-processor/router/overlay_router.go` - Route messages to overlays
- `services/message-processor/seventv/` - 7TV EventAPI real-time updates
- `services/message-processor/sessions/capture.go` - Stream session tracking (credit roll)

**Read When**: Working on message normalization, emote enrichment, platform support, or credit roll feature.

**→ Full Docs**: [services/message-processor/README.md](../../services/message-processor/README.md)

---

### Source Manager (`:8088`) ✅

**Purpose**: Active source registry + Redis-based leader election coordination

**Key Files**:
- `services/source-manager/cmd/main.go` - Entry point, registry initialization
- `services/source-manager/registry/registry.go` - Active source tracking (syncs from database)
- `services/source-manager/election/leader.go` - Redis-based leader election logic
- `services/source-manager/handlers/` - Leadership API (claim, renew, release)

**Read When**: Working on leader election, active source registry, or YouTube Listener coordination.

**→ Full Docs**: [services/source-manager/README.md](../../services/source-manager/README.md)

---

### Kick Listener (`:8089`) ✅

**Purpose**: Connect to Kick Pusher WebSocket, subscribe to channels, publish messages

**Key Files**:
- `services/kick-listener/cmd/main.go` - Entry point
- `services/kick-listener/websocket/client.go` - Pusher Protocol 7 WebSocket client
- `services/kick-listener/channels/manager.go` - Channel subscription management
- `services/kick-listener/publisher/redis.go` - Publish to Redis Stream `chat:raw`

**Read When**: Working on Kick integration, Pusher WebSocket, or chatroom ID resolution.

**→ Full Docs**: [services/kick-listener/README.md](../../services/kick-listener/README.md)

---

### TikTok Listener (`:8090`) ✅

**Purpose**: Connect to TikTok Live (unofficial library), publish messages

**Key Files**:
- `services/tiktok-listener/` - Node.js implementation (unofficial library)
- Service documentation for details

**Read When**: Working on TikTok integration or unofficial library integration.

**→ Full Docs**: [services/tiktok-listener/README.md](../../services/tiktok-listener/README.md)

---

### Token Refresh Service (CronJob) ✅

**Purpose**: Background job to refresh OAuth tokens before expiry

**Key Files**:
- `services/token-refresh-service/cmd/main.go` - CronJob entry point
- `services/token-refresh-service/refresh/` - Platform-specific refresh logic

**Read When**: Working on OAuth token refresh, expiry handling, or alert integration.

**→ Full Docs**: [services/token-refresh-service/README.md](../../services/token-refresh-service/README.md)

---

## Shared Packages (`shared/`)

Reusable Go packages used across services:

| Package | Purpose | Used By |
|---------|---------|---------|
| **auth/** | JWT utilities (Generate, Validate, Claims) | API Gateway, Auth Service |
| **database/** | PostgreSQL connection pooling (pgx) | All services with DB access |
| **redis/** | Redis client wrapper | All services |
| **logger/** | Zap logger initialization | All services |
| **middleware/** | HTTP middleware (CORS, Auth, Logging) | All HTTP services |
| **metrics/** | Prometheus metrics (business + technical) | All services |
| **ratelimit/** | Rate limiting primitives | Overlay Manager, Auth Service |

**Read When**: Implementing cross-cutting concerns (logging, auth, database access).

---

## Common Tasks → Where to Look

### Add Support for New Platform (e.g., Rumble, Facebook Gaming)

**Files to Create**:
1. `services/<platform>-listener/` - New listener service (copy template)
2. `services/message-processor/normalizer/<platform>_normalizer.go` - Message parser
3. `migrations/XXX_<platform>_support.sql` - Database schema

**Files to Modify**:
- `services/message-processor/router/router.go` - Add platform case
- `CLAUDE.md` - Update platform status

**→ Complete Guide**: [QUICK-REF-ADD-PLATFORM.md](./QUICK-REF-ADD-PLATFORM.md) (~150 lines)

---

### Debug YouTube Quota Issues

**Files to Check**:
1. `services/youtube-listener/quota/tracker.go` - Quota tracking logic
2. `services/youtube-listener/handlers/quota.go` - `/quota/status` endpoint
3. `migrations/008_quota_reservations.sql` - Reservation schema

**Commands**:
```bash
curl http://localhost:8086/quota/status | jq .
psql -c "SELECT * FROM youtube_quota_usage WHERE date = CURRENT_DATE;"
```

**→ Complete Guide**: [QUICK-REF-DEBUG-QUOTA.md](./QUICK-REF-DEBUG-QUOTA.md) (~200 lines)

---

### Add New HTTP Endpoint

**Files to Create/Modify**:
1. `services/<service>/handlers/<feature>.go` - Handler function
2. `services/<service>/cmd/main.go` - Register route
3. `services/<service>/handlers/<feature>_test.go` - Tests

**→ Complete Guide**: [QUICK-REF-ADD-ENDPOINT.md](./QUICK-REF-ADD-ENDPOINT.md) (~100 lines)

---

### Scale Service or Infrastructure

**Files to Check**:
1. `deployments/k8s/base/<service>/hpa.yaml` - HPA configuration
2. `deployments/k8s/base/<service>/deployment.yaml` - Resource limits
3. `docs/architecture/03-SCALING.md` - Scaling strategies

**→ Complete Guide**: [QUICK-REF-SCALING.md](./QUICK-REF-SCALING.md) (~80 lines)

---

### Create Database Migration

**Files to Create**:
1. `migrations/XXX_<description>.sql` - New migration file

**Commands**:
```bash
# Find next migration number
ls -1 migrations/*.sql | tail -1

# Run migrations
make migrate
```

**→ Complete Guide**: [QUICK-REF-DATABASE-MIGRATION.md](./QUICK-REF-DATABASE-MIGRATION.md) (~100 lines)

---

## Documentation Navigation

### Architecture Documentation

Read in numbered order (00 → 05):
1. [00-OVERVIEW.md](../architecture/00-OVERVIEW.md) - System overview, service map
2. [01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md) - Message processing pipeline
3. [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - Kubernetes deployment
4. [03-SCALING.md](../architecture/03-SCALING.md) - Performance and scaling
5. [04-OBSERVABILITY.md](../architecture/04-OBSERVABILITY.md) - Metrics, logs, traces
6. [05-SECURITY.md](../architecture/05-SECURITY.md) - Security architecture

**Total Reading Time**: ~2 hours for complete architecture understanding

### Architecture Decision Records (ADRs)

**Why decisions were made**:
- [ADR-0001](../adr/0001-standard-go-layout.md) - Standard Go Layout (not hexagonal)
- [ADR-0002](../adr/0002-redis-streams-pubsub.md) - Redis Streams + Pub/Sub hybrid
- [ADR-0003](../adr/0003-cloudnative-postgres.md) - CloudNativePG operator
- [ADR-0004](../adr/0004-no-hexagonal-architecture.md) - No ports/adapters
- [ADR-0005](../adr/0005-react-nextjs-frontend.md) - React + Next.js
- [ADR-0006](../adr/0006-youtube-quota-tracking.md) - YouTube quota tracking
- [ADR-0007](../adr/0007-leadership-rebalancing.md) — Leadership rebalancing
- [ADR-0008](../adr/0008-feature-gate-infrastructure.md) — Feature gate infra
- [ADR-0009](../adr/0009-ring-buffer-publisher.md) — Ring buffer publisher
- [ADR-0010](../adr/0010-pronoun-enricher-alejo-api.md) — Pronoun enricher (Alejo)
- [ADR-0011](../adr/0011-zombie-listener-detection.md) — Zombie listener detection
- [ADR-0012](../adr/0012-oauth-scope-minimisation.md) — OAuth scope minimisation

---

## Quick Links Summary

### Most Useful for LLM Agents

| Task | Quick Reference | Lines |
|------|-----------------|-------|
| Add new platform | [QUICK-REF-ADD-PLATFORM.md](./QUICK-REF-ADD-PLATFORM.md) | 150 |
| Debug YouTube quota | [QUICK-REF-DEBUG-QUOTA.md](./QUICK-REF-DEBUG-QUOTA.md) | 200 |
| Add HTTP endpoint | [QUICK-REF-ADD-ENDPOINT.md](./QUICK-REF-ADD-ENDPOINT.md) | 100 |
| Security audit | [QUICK-REF-SECURITY-AUDIT.md](./QUICK-REF-SECURITY-AUDIT.md) | 100 |
| Scale services | [QUICK-REF-SCALING.md](./QUICK-REF-SCALING.md) | 80 |
| Database migration | [QUICK-REF-DATABASE-MIGRATION.md](./QUICK-REF-DATABASE-MIGRATION.md) | 100 |
| Kubernetes debug | [QUICK-REF-KUBERNETES-DEBUG.md](./QUICK-REF-KUBERNETES-DEBUG.md) | 100 |
| Redis operations | [QUICK-REF-REDIS-OPERATIONS.md](./QUICK-REF-REDIS-OPERATIONS.md) | 100 |

### Troubleshooting

**Start with**: [decision-tree.md](../troubleshooting/decision-tree.md) - High-level triage

**Specific issues**:
- [build-errors.md](../troubleshooting/build-errors.md) - Compilation, Docker, startup
- [connection-errors.md](../troubleshooting/connection-errors.md) - PostgreSQL, Redis
- [youtube-quota-exceeded.md](../troubleshooting/youtube-quota-exceeded.md) - Quota issues
- [twitch-irc-issues.md](../troubleshooting/twitch-irc-issues.md) - IRC, channel joins
- [websocket-disconnects.md](../troubleshooting/websocket-disconnects.md) - WebSocket issues

---

## Database Schema (`migrations/`)

**Migration Files** (applied in order):
```
001_initial_schema.sql          # Users, overlays, basic tables
003_youtube_support.sql         # YouTube OAuth, channels
005_kick_support.sql            # Kick OAuth, chatrooms
008_quota_reservations.sql      # YouTube quota reserve-confirm-rollback
021_credit_roll_configs.sql     # Credit roll feature
022_stream_sessions.sql         # Session tracking
```

**Key Tables**:
- `users` - User accounts (platform-linked)
- `overlays` - Overlay configurations
- `overlay_chat_sources` - Multi-source assignments
- `youtube_oauth_tokens` - YouTube OAuth per user
- `youtube_quota_usage` - Daily quota tracking
- `youtube_quota_reservations` - In-flight API call reservations

---

## Useful Commands

```bash
# Build & Test
make build              # Build all services
make test               # Run all tests
make docker-up          # Start local environment

# Database
make migrate            # Apply migrations
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat

# Redis
redis-cli
XINFO STREAM chat:raw
PUBSUB CHANNELS overlay:*

# Kubernetes
kubectl get pods -n allchat
kubectl logs -n allchat deployment/<service> -f
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat
```

---

## Tips for LLM Agents

1. **Read service READMEs first** - Each service has comprehensive documentation
2. **Use quick reference cards** - Task-oriented guides save time (150-200 lines vs 1,000+)
3. **Check ADRs for "why"** - Understand design decisions before suggesting changes
4. **Follow troubleshooting decision tree** - Structured diagnosis saves time
5. **Verify with commands** - Always test assumptions with actual commands

---

## Summary

**Navigation Strategy**:
1. **Task-oriented?** → Use quick reference cards (docs/llm-guides/QUICK-REF-*)
2. **Service-specific?** → Read service README (services/*/README.md)
3. **Architecture questions?** → Read architecture docs (00-05)
4. **Why was X chosen?** → Check ADRs (docs/adr/)
5. **Troubleshooting?** → Start with decision tree, then specific guide

**Total Documentation**: ~10,000 lines organized for maximum LLM efficiency.
