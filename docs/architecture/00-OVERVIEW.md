# All-Chat: Architecture Overview

**Version**: 2.0
**Last Updated**: 2026-01-28
**Status**: Phase 4 Complete - Production Ready

---

## Table of Contents

1. [Introduction](#introduction)
2. [Architecture Principles](#architecture-principles)
3. [System Overview](#system-overview)
4. [Service Map](#service-map)
5. [Technology Stack](#technology-stack)
6. [Message Flow](#message-flow)
7. [Design Decisions](#design-decisions)
8. [Further Reading](#further-reading)

---

## Introduction

All-Chat is a **cloud-native microservices platform** for aggregating and displaying chat messages from multiple live streaming platforms (Twitch, YouTube, Kick, TikTok) on streaming overlays.

**Core Value Proposition**: Unified chat display for multi-platform streamers, supporting 7TV, BTTV, and FFZ emotes with real-time updates.

**Architecture Philosophy**:
- **Standard Go Layout** (not hexagonal) - LLM-friendly, less boilerplate
- **Microservices** - Clear boundaries, independent scaling
- **Cloud-native** - Kubernetes, CNPG (PostgreSQL), LGTM observability stack
- **LLM-first development** - Optimized for code generation by AI agents

---

## Architecture Principles

### Key Design Principles

1. **Horizontal Scalability**: All stateless services can scale to N replicas
2. **Fault Tolerance**: Services are resilient to dependency failures
3. **Separation of Concerns**: Each service has a single, well-defined responsibility
4. **Modularity**: Platform listeners are pluggable (can enable/disable any platform)
5. **Observability**: Comprehensive metrics, logs, and traces (LGTM stack)
6. **Eventual Consistency**: Acceptable for non-critical paths (emote cache, metadata)

### Non-Goals (Explicit Constraints)

- **Not a chat bot platform** - Only displays chat, does not send messages
- **Not a chat archive** - Messages are transient (Redis Streams trimmed to 50K)
- **Not a moderation tool** - No filtering, banning, or spam detection
- **Not multi-region (Phase 1)** - Single data center deployment

---

## System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     External Platforms                       │
│  Twitch IRC  │  YouTube API  │  Kick WebSocket  │  TikTok   │
└──────────────┬──────────────┬──────────────┬────────────────┘
               │              │              │
               ▼              ▼              ▼
         ┌──────────────────────────────────────┐
         │      Platform Listeners (8085-8089)   │
         │  • Twitch Listener (IRC)              │
         │  • YouTube Listener (HTTP polling)    │
         │  • Kick Listener (Pusher WebSocket)   │
         │  • TikTok Listener (unofficial lib)   │
         └───────────────┬──────────────────────┘
                         │ publish raw messages
                         ▼
                  ┌─────────────┐
                  │ Redis Stream│ (chat:raw)
                  │ XADD        │
                  └──────┬──────┘
                         │ XREADGROUP (consumer group)
                         ▼
              ┌──────────────────────┐
              │ Message Processor     │ (8087)
              │ • Normalize format    │
              │ • Enrich emotes       │
              │ • Filter by age       │
              └──────────┬────────────┘
                         │ publish enriched messages
                         ▼
                  ┌─────────────┐
                  │ Redis Pub/Sub│ (overlay:*)
                  │ PUBLISH      │
                  └──────┬──────┘
                         │ SUBSCRIBE
                         ▼
              ┌──────────────────────┐
              │  API Gateway (8080)   │
              │  • WebSocket Hub      │
              │  • HTTP Reverse Proxy │
              └──────────┬────────────┘
                         │ WebSocket broadcast
                         ▼
              ┌──────────────────────┐
              │  Overlay (Frontend)   │
              │  • React + Next.js    │
              │  • Real-time display  │
              └───────────────────────┘
```

### Supporting Services

```
┌─────────────────────────────────────────────────────────────┐
│                     Core Services                            │
│                                                               │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Auth Service│  │Overlay Manager│  │Emote Service │      │
│  │   (8081)    │  │    (8082)     │  │   (8083)     │      │
│  │ • OAuth     │  │ • CRUD        │  │ • 7TV cache  │      │
│  │ • JWT       │  │ • Multi-source│  │ • BTTV cache │      │
│  └─────────────┘  └──────────────┘  └──────────────┘      │
│                                                               │
│  ┌──────────────────────────────────────────────────┐       │
│  │          Source Manager (8088)                    │       │
│  │          • Leader election (YouTube)              │       │
│  │          • Active source registry                 │       │
│  └──────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                   Infrastructure                             │
│                                                               │
│  ┌──────────────────┐         ┌─────────────────────┐       │
│  │  PostgreSQL      │         │  Redis 7            │       │
│  │  (CloudNativePG) │         │  • Streams (durable)│       │
│  │  • 3 nodes       │         │  • Pub/Sub (fast)   │       │
│  │  • Auto failover │         │  • Cache (emotes)   │       │
│  └──────────────────┘         └─────────────────────┘       │
│                                                               │
│  ┌──────────────────────────────────────────────────┐       │
│  │  LGTM Stack (Observability)                       │       │
│  │  • Loki (logs) • Grafana • Prometheus • Tempo    │       │
│  └──────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## Service Map

### Edge Layer

**API Gateway** (`:8080`)
- **Purpose**: Entry point for all HTTP/WebSocket traffic
- **Features**: HTTP reverse proxy, WebSocket hub, CORS, auth middleware
- **Scaling**: 2-20 replicas, 2,500 WebSocket connections per pod
- **→ Documentation**: [services/api-gateway/README.md](../../services/api-gateway/README.md)

### Core Services

**Auth Service** (`:8081`)
- **Purpose**: OAuth flows (Twitch, YouTube, Kick) + JWT token issuance
- **Features**: Multi-platform OAuth, token refresh, user management
- **Scaling**: 2-5 replicas (low traffic, mostly session-based)
- **→ Documentation**: [services/auth-service/README.md](../../services/auth-service/README.md)

**Overlay Manager** (`:8082`)
- **Purpose**: Overlay CRUD operations + multi-source configuration
- **Features**: Create/update/delete overlays, add/remove chat sources per overlay
- **Scaling**: 2-5 replicas (low traffic, CRUD operations)
- **→ Documentation**: [services/overlay-manager/README.md](../../services/overlay-manager/README.md)

**Emote Service** (`:8083`)
- **Purpose**: Cache 7TV, BTTV, FFZ emotes with real-time updates
- **Features**: Emote API aggregation, Redis cache (1-hour TTL), 7TV EventAPI WebSocket
- **Scaling**: 2-5 replicas (cache hit rate >95%)
- **→ Documentation**: [services/emote-service/README.md](../../services/emote-service/README.md)

### Platform Listeners

**Twitch Listener** (`:8085`)
- **Purpose**: Connect to Twitch IRC, publish raw messages to Redis Streams
- **Protocol**: IRC (gempir/go-twitch-irc)
- **Capacity**: 500+ concurrent channels per pod
- **Rate Limits**: 20 JOIN/10 seconds (Twitch limit)
- **→ Documentation**: [services/twitch-listener/README.md](../../services/twitch-listener/README.md)

**YouTube Listener** (`:8086`)
- **Purpose**: Poll YouTube Live Chat API, publish raw messages to Redis Streams
- **Protocol**: HTTP polling (2-5 second intervals)
- **Capacity**: 50+ concurrent streams per pod (quota-limited)
- **Critical**: Advanced quota tracking (reserve-confirm-rollback, 99.95%+ accuracy)
- **→ Documentation**: [services/youtube-listener/README.md](../../services/youtube-listener/README.md)

**Kick Listener** (`:8089`)
- **Purpose**: Connect to Kick Pusher WebSocket, publish raw messages to Redis Streams
- **Protocol**: Pusher WebSocket Protocol 7
- **Capacity**: 200+ concurrent channels per pod
- **Latency**: ~100ms (real-time WebSocket)
- **→ Documentation**: [services/kick-listener/README.md](../../services/kick-listener/README.md)

**TikTok Listener** (`:8090` - planned)
- **Purpose**: Connect to TikTok Live, publish raw messages to Redis Streams
- **Protocol**: Unofficial TikTok Live library
- **Status**: ✅ Implemented (username-based, no OAuth)
- **→ Documentation**: [services/tiktok-listener/README.md](../../services/tiktok-listener/README.md)

### Processing Layer

**Message Processor** (`:8087`)
- **Purpose**: Normalize, enrich, and route messages
- **Features**:
  - Platform-specific normalizers (Twitch, YouTube, Kick, TikTok)
  - Emote enrichment (7TV, BTTV, FFZ)
  - Message age filtering (60s cutoff)
  - Publish to overlay-specific Redis Pub/Sub channels
- **Capacity**: ~3,000 messages/second per pod
- **Scaling**: 3-10 replicas (CPU-intensive with emote enrichment)
- **→ Documentation**: [services/message-processor/README.md](../../services/message-processor/README.md)

**Source Manager** (`:8088`)
- **Purpose**: Active source registry + Redis-based leader election
- **Features**:
  - Track which chat sources are active
  - Leader election for YouTube Listener (prevents duplicate API calls)
  - Coordination between listener replicas
- **Scaling**: 1-3 replicas (lightweight, coordination only)
- **→ Documentation**: [services/source-manager/README.md](../../services/source-manager/README.md)

**Token Refresh Service** (background job)
- **Purpose**: Refresh OAuth tokens before expiry
- **Features**: Platform-specific refresh flows, retry logic, error handling
- **Scaling**: 1 replica (CronJob)
- **→ Documentation**: [services/token-refresh-service/README.md](../../services/token-refresh-service/README.md)

---

## Technology Stack

### Backend

| Technology | Purpose | Version |
|------------|---------|---------|
| **Go** | Microservices language | 1.23+ |
| **Gin** | HTTP framework | Latest |
| **pgx/v5** | PostgreSQL driver | v5 |
| **go-redis/v9** | Redis client | v9 |
| **Zap** | Structured logging | Latest |
| **gempir/go-twitch-irc** | Twitch IRC client | v4 |
| **gorilla/websocket** | WebSocket server/client | Latest |
| **golang-jwt/jwt** | JWT authentication | v5 |

### Frontend

| Technology | Purpose | Version |
|------------|---------|---------|
| **React** | UI library | 18+ |
| **Next.js** | SSR framework | 14+ (App Router) |
| **TypeScript** | Type safety | Latest |
| **Tailwind CSS** | Styling | Latest |

### Infrastructure

| Technology | Purpose | Version |
|------------|---------|---------|
| **Kubernetes** | Container orchestration | 1.28+ |
| **CloudNativePG** | PostgreSQL operator | Latest |
| **Hetzner Cloud** | VPS hosting | - |
| **Loki** | Log aggregation | Latest |
| **Grafana** | Dashboards | Latest |
| **Prometheus** | Metrics | Latest |
| **Tempo** | Tracing (planned) | Latest |

---

## Message Flow

### End-to-End Message Pipeline

```
1. Platform → Listener
   Twitch IRC message arrives
   ↓
2. Listener → Redis Streams
   Publish to chat:raw (XADD)
   ↓
3. Redis Streams → Message Processor
   Consumer group reads (XREADGROUP)
   ↓
4. Message Processor - Normalize
   Platform-specific format → Unified format
   ↓
5. Message Processor - Enrich
   Add 7TV/BTTV/FFZ emotes, user badges
   ↓
6. Message Processor → Redis Pub/Sub
   Publish to overlay:{overlay_id} (PUBLISH)
   ↓
7. Redis Pub/Sub → API Gateway
   Gateway subscribes to overlay channels (SUBSCRIBE)
   ↓
8. API Gateway → Frontend
   Broadcast to WebSocket clients
   ↓
9. Frontend - Render
   Display message with emotes in overlay
```

**Latency Budget** (P95):
- Listener → Redis Streams: <50ms
- Redis Streams → Processor: <100ms
- Processor (normalize + enrich): <300ms
- Processor → Redis Pub/Sub: <50ms
- Redis Pub/Sub → Gateway: <50ms
- Gateway → Frontend: <50ms
- **Total P95**: <500ms (listener → overlay display)

### Unified Message Format

All platforms normalized to common schema:

```json
{
  "id": "uuid",
  "overlay_id": "uuid",
  "platform": "twitch|youtube|kick|tiktok",
  "channel_id": "platform-channel-id",
  "channel_name": "DisplayName",
  "user": {
    "id": "platform-user-id",
    "username": "username",
    "display_name": "DisplayName",
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
  "timestamp": "2026-01-28T10:00:00Z",
  "metadata": {
    "is_subscriber": true,
    "is_moderator": false,
    "bits": 0,
    "super_chat_amount": 0
  }
}
```

---

## Design Decisions

### Why Standard Go Layout (Not Hexagonal)?

**Decision**: Use Standard Go project layout, not ports/adapters hexagonal architecture.

**Rationale**:
- 60% less boilerplate code
- LLMs generate more accurate code (familiar patterns)
- Go community standard (examples, tutorials align)
- Easier to onboard new developers
- Testing still straightforward with dependency injection

**→ Details**: [ADR-0001: Standard Go Layout](../adr/0001-standard-go-layout.md)

### Why Redis Streams + Pub/Sub Hybrid?

**Decision**: Use Redis Streams for durable message queue + Redis Pub/Sub for low-latency broadcast.

**Rationale**:
- **Streams**: Durable, consumer groups, backpressure-capable (chat:raw)
- **Pub/Sub**: Low-latency, fan-out to many subscribers (overlay:*)
- Single Redis instance (Phase 1), simpler than Kafka/NATS
- 100-500ms latency achievable (vs 2-5s with Kafka)

**Trade-offs**:
- Pub/Sub not durable (messages lost if subscriber crashes)
- O(n) memory growth with subscribers (mitigated by Redis Cluster in Phase 2)

**→ Details**: [ADR-0002: Redis Streams + Pub/Sub](../adr/0002-redis-streams-pubsub.md)

### Why CloudNativePG?

**Decision**: Use CloudNativePG operator for PostgreSQL instead of manual StatefulSet.

**Rationale**:
- Team has production experience with CNPG
- Automated failover (<30 seconds RTO)
- PITR (Point-in-Time Recovery) built-in
- Backup automation (daily to S3-compatible storage)
- Rolling updates with zero downtime

**→ Details**: [ADR-0003: CloudNativePG](../adr/0003-cloudnative-postgres.md)

### Why React + Next.js?

**Decision**: Use React 18+ with Next.js 14+ (App Router) for frontend.

**Rationale**:
- LLMs excel at generating React code (90%+ accuracy)
- SSR for SEO and initial load performance
- App Router for server components (reduced bundle size)
- TypeScript for type safety
- Large ecosystem of UI libraries

**→ Details**: [ADR-0005: React + Next.js](../adr/0005-react-nextjs-frontend.md)

### Why Reserve-Confirm-Rollback Quota Tracking?

**Decision**: Use atomic database reservations for YouTube API quota tracking instead of simple counter.

**Problem**: Simple counter had ±500 units/day drift (5% error), risking quota exhaustion.

**Solution**:
- Reserve quota BEFORE API call (atomic database operation)
- Confirm if call succeeds (move reserved → used)
- Rollback if 4xx client error (release reservation)
- Cleanup stale reservations every 60 seconds (crash recovery)

**Impact**: 99.95%+ accuracy, 9,000+ units/day waste eliminated (90% reduction).

**→ Details**: [ADR-0006: YouTube Quota Tracking](../adr/0006-youtube-quota-tracking.md)

---

## Further Reading

### Architecture Documentation

- **[01-DATA-FLOW.md](./01-DATA-FLOW.md)** - Detailed message flow, Redis patterns
- **[02-DEPLOYMENT.md](./02-DEPLOYMENT.md)** - Kubernetes deployment, infrastructure
- **[03-SCALING.md](./03-SCALING.md)** - Scaling strategies, performance bottlenecks
- **[04-OBSERVABILITY.md](./04-OBSERVABILITY.md)** - Metrics, logging, tracing, alerts
- **[05-SECURITY.md](./05-SECURITY.md)** - Security architecture, threat model

### Architecture Decision Records (ADRs)

- **[ADR Index](../adr/README.md)** - All ADRs with status and context
- **[ADR-0001](../adr/0001-standard-go-layout.md)** - Standard Go Layout (not hexagonal)
- **[ADR-0002](../adr/0002-redis-streams-pubsub.md)** - Redis Streams + Pub/Sub hybrid
- **[ADR-0003](../adr/0003-cloudnative-postgres.md)** - CloudNativePG operator
- **[ADR-0004](../adr/0004-no-hexagonal-architecture.md)** - No ports/adapters abstraction
- **[ADR-0005](../adr/0005-react-nextjs-frontend.md)** - React + Next.js frontend
- **[ADR-0006](../adr/0006-youtube-quota-tracking.md)** - YouTube quota reserve-confirm-rollback

### Service Documentation

All services have detailed READMEs: [services/*/README.md](../../services/)

### Quick Reference Guides

- **[QUICK-REF-ADD-PLATFORM.md](../llm-guides/QUICK-REF-ADD-PLATFORM.md)** - Add new platform (150 lines)
- **[QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md)** - YouTube quota debugging (200 lines)
- **[QUICK-REF-SCALING.md](../llm-guides/QUICK-REF-SCALING.md)** - Scale services (150 lines)

### Troubleshooting

- **[Troubleshooting Decision Tree](../troubleshooting/decision-tree.md)** - High-level triage for common issues

---

## Current Status

**Phase 4 Complete** (2026-01-28):
- ✅ All 13 services implemented and deployed
- ✅ Twitch, YouTube, Kick, TikTok integrations ready for production
- ✅ CloudNativePG deployed (3-node cluster, automated failover)
- ✅ LGTM observability stack deployed (Loki, Grafana, Prometheus)
- ✅ Comprehensive metrics (100% service coverage)
- ✅ HPA autoscaling configured (all services)

**Known Limitations**:
- YouTube API quota: 10,000 units/day (request increase to 1M)
- Redis single instance (Phase 1 bottleneck, Redis Cluster in Phase 2)
- Token encryption is basic (TODO: implement AES-GCM)
- No service-to-service auth (Kubernetes NetworkPolicies only)

**Next Phase**: Phase 5 - Production hardening (multi-node K8s, Redis Cluster, security enhancements)
