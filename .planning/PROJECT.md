# All-Chat

## What This Is

All-Chat is a cloud-native platform that aggregates live chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and displays them in unified overlays for streamers. The platform features intelligent load distribution with hybrid hash-based sharding, automatic rebalancing, quota-free YouTube ingestion via InnerTube API, and bidirectional chat overlay sharing between streamers as its first premium feature.

## Requirements

### Validated

<!-- Shipped and confirmed valuable -->

**Infrastructure (existing):**
- ✓ Multi-platform chat aggregation (Twitch, YouTube, Kick, TikTok) — existing
- ✓ Real-time message delivery via WebSocket to overlays — existing
- ✓ Message normalization across platforms (unified schema) — existing
- ✓ Emote enrichment (7TV, BTTV, FFZ, platform-native) — existing
- ✓ Overlay configuration with multi-source support — existing
- ✓ OAuth authentication for platform access — existing
- ✓ Redis Streams for durable message queuing — existing
- ✓ Redis Pub/Sub for real-time broadcast to overlays — existing
- ✓ Microservices architecture (Standard Go Layout) — existing
- ✓ Kubernetes-deployable with health checks and graceful shutdown — existing

**Message Deletion (v1.0):**
- ✓ Message deletion support (Twitch, YouTube, Kick) — v1.0
- ✓ Message ID tracking and registry — v1.0
- ✓ Deletion event propagation pipeline — v1.0
- ✓ WebSocket reconnection replay buffer — v1.0

**Load Balancing (v1.1):**
- ✓ Hash-based channel sharding (consistent hashing across pods) — v1.1
- ✓ Per-pod load metrics collection (messages/sec, channel count) — v1.1
- ✓ Load imbalance detection (threshold-based triggers) — v1.1
- ✓ Automatic hot channel identification (high message rate detection) — v1.1
- ✓ Graceful channel migration (zero message loss during rebalancing) — v1.1
- ✓ Channel assignment registry (Redis-based, pod → channels mapping) — v1.1
- ✓ Rebalancing coordinator (leader-elected, triggers migrations) — v1.1
- ✓ Load balancing across all 4 listener types (Twitch, YouTube, Kick, TikTok) — v1.1
- ✓ Kubernetes HPA integration (scale based on aggregate load) — v1.1
- ✓ Load balancing observability (metrics, dashboards, alerts) — v1.1

**InnerTube YouTube Listener (v1.2):**
- ✓ InnerTube API integration (quota-free YouTube chat ingestion) — v1.2
- ✓ Drop-in replacement architecture (RawChatMessage schema compatibility) — v1.2
- ✓ All event types (messages, Super Chat, Super Sticker, memberships, deletions) — v1.2
- ✓ Batch deletion detection (5+ deletions in 100ms time window) — v1.2
- ✓ Deletion event buffering (500ms delay for race condition handling) — v1.2
- ✓ Dynamic stream management (discovery, lifecycle, offline detection) — v1.2
- ✓ Source-manager integration (leadership coordination) — v1.2
- ✓ Contract validation infrastructure (golden files, dual-listener test) — v1.2
- ✓ Production rollout infrastructure (Argo Rollouts, canary deployment) — v1.2
- ✓ Advanced metrics (per-channel message rate, network error classification) — v1.2

**Chat Overlay Sharing (v1.3):**
- ✓ User can search for other users by platform username — v1.3
- ✓ User can send share request with selected overlay — v1.3
- ✓ User can view pending share requests in dashboard — v1.3
- ✓ User can accept share request, selecting overlay to share back and expiry option — v1.3
- ✓ Users can add shared overlays as sources to any overlay (like platform sources) — v1.3
- ✓ Shared overlay source delivers all messages from source overlay's chat sources — v1.3
- ✓ Display settings (CSS, events) apply from displaying overlay, not source — v1.3
- ✓ Share expires when either user's stream ends (if "this stream" selected) — v1.3
- ✓ Share expires after duration (if time-based selected) — v1.3
- ✓ Either user can revoke share at any time — v1.3
- ✓ Revoked/expired shares marked as inactive (not removed from config) — v1.3
- ✓ Premium enforcement blocks non-premium users from sharing — v1.3
- ✓ Admin can mark specific users as premium for testing — v1.3
- ✓ Stream lifecycle detection: Twitch (EventSub), YouTube (HandleStreamOffline), TikTok (disconnected handler); Kick gracefully disabled — v1.3

### Active

<!-- Next milestone scope: TBD -->

### Out of Scope

- Cross-region load balancing — Single Kubernetes cluster sufficient for current scale
- Predictive scaling based on stream schedules — Reactive scaling meets needs
- Custom load metrics from user configuration — Message rate provides accurate signal
- Channel affinity/pinning — Rebalancing flexibility more valuable than pinning
- Multi-tenancy isolation — Single-tenant deployment model
- Event suppression (emit 1 batch event vs N events) — Deferred to future (detection working, metadata provided)
- Self-hoster migration guide — Canary deployment guide exists, standalone migration deferred
- Kick stream lifecycle detection — Researched in v1.3; no reliable webhook/API found; graceful disable implemented
- Public overlay directory — Moderation burden, copyright risk, DMCA exposure
- Automatic share acceptance — Violates consent model, security risk
- Cross-platform relay (A→B→C) — Permission complexity, amplifies load issues
- Separate expiry microservice — Unnecessary complexity, collocated in share-service

## Context

**Current State (v1.3 shipped):**
- 8 core microservices (added share-service in v1.3: share request management, premium enforcement, expiry jobs, lifecycle subscription)
- 5 listener services: 4 with load balancing + 1 quota-free InnerTube listener; Twitch/YouTube/TikTok publish lifecycle:stream_end for share expiry
- share-service: bidirectional overlay sharing, cycle detection, DFS algorithm, WebSocket notifications, background expiry jobs
- 246 files changed in v1.3 (+28,236 insertions); 3-day delivery (2026-03-09 → 2026-03-11)
- Premium feature enforcement pattern established for future monetization features

**Load Balancing Implementation (v1.1):**
- **Coordinator:** Kubernetes Lease-based leader election, bounded-load consistent hashing (1.25x average limit)
- **Migration:** Overlap protocol (new pod connects before old disconnects), zero-loss guarantee via first-message confirmation
- **Rebalancing:** Composite load scoring (70% message rate + 30% channel count), automatic hot channel redistribution (>3x average rate)
- **Safeguards:** 5-minute cooldown, 20% per-operation limit, thrashing detection, distributed lock coordination
- **HPA:** Startup jitter (0-30s) prevents thundering herd, filtered assignment count for readiness probes
- **Observability:** 16 distributed tracing spans, Grafana dashboards with Pod×Platform and Pod×Time heatmaps, Prometheus alerts

**InnerTube Listener Architecture (v1.2):**
- **API Integration:** InnerTube HTTP POST client eliminates YouTube API quota constraints (10,000 units/day → unlimited)
- **Schema Compatibility:** RawChatMessage byte-for-byte compatible with official listener, zero downstream changes
- **Event Support:** All chat types (regular messages, Super Chat, Super Sticker, membership events, tickers, deletions)
- **Batch Detection:** Time-windowed aggregation (5+ deletions in 100ms) with reason classification (timeout vs ban)
- **Deletion Buffer:** 500ms delay before emission handles race conditions (deletion arrives before original message)
- **Lifecycle Management:** Stream discovery (HTML parsing), offline detection (empty continuations), auto-resume, graceful shutdown
- **Testing:** 69 automated tests (contract validation, lifecycle behaviors, deletion events), 24-hour dual-listener test infrastructure

**Technical Environment:**
- Go 1.25.6 microservices with Standard Go Layout
- Redis 7 (Streams for queuing, Pub/Sub for broadcast, assignment registry)
- Kubernetes with CloudNativePG (PostgreSQL), HPA, Horizontal Pod Autoscaler
- OpenTelemetry tracing, Prometheus metrics, Grafana dashboards
- Argo Rollouts for canary deployments (InnerTube listener)

**Known Issues / Technical Debt:**
- Health handler test mock signature needs fix (Phase 9 minor gap, non-blocking)
- Self-hoster migration guide incomplete (canary deployment covered, standalone path deferred)
- Event suppression architecture deferred (emits N events where 5th is tagged 'batch', not 1 batch event)
- Argo Rollouts CRD not installed in cluster yet (prerequisite for InnerTube canary deployment)
- Human verification pending: 24-hour dual-listener test, Grafana dashboard rendering, Kubernetes dry-run validation

## Constraints

- **Tech Stack**: Go 1.25.6, existing microservices, Redis 7, Kubernetes — No new infrastructure dependencies
- **Backward Compatibility**: Must not break existing message flow — Single-pod deployments still supported
- **Zero Downtime**: Channel migration must be lossless — Enforced via overlap protocol and confirmation signaling
- **Platform Connection Limits**: Respect platform rate limits and connection quotas — InnerTube bypasses YouTube quota
- **Stateless Services**: Listeners remain stateless — All state in Redis, not in-memory

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Hybrid hash-based + load-aware approach | Predictable under normal load, adapts to real-world usage patterns | ✓ Good — v1.1 delivers both predictability and load awareness |
| Consistent hashing for channel assignment | Minimizes reassignments when pods scale, predictable placement | ✓ Good — CRC32 hash with virtual nodes, bounded-load (1.25x) |
| Redis for assignment registry | Centralized state, atomic updates, survives pod restarts | ✓ Good — O(1) lookups, O(log N) load queries via sorted sets |
| Reuse source-manager for coordination | Already has leader election, knows active sources, avoid duplication | ✓ Good — Extended with coordination logic, no new service needed |
| Kubernetes Lease-based leader election | Built-in fencing via resourceVersion, automatic leader failover | ✓ Good — Prevents split-brain, tested with chaos scenarios |
| Overlap migration protocol | Zero message loss guarantee, confirmation signaling | ✓ Good — First-message confirmation via channel signaling |
| Composite load scoring (70% message rate + 30% channel count) | Message processing dominates CPU, channel count matters for memory | ✓ Good — Validated via Prometheus query analysis |
| 5-minute rebalancing cooldown | Prevents thrashing while allowing timely redistribution | ✓ Good — Thrashing detection (>5 in 30min) as backstop |
| 20% per-operation migration limit | Balance between quick redistribution and system stability | ✓ Good — Minimum 1 channel enforced, proportional strategy |
| Startup jitter (0-30s random delay) | Prevents thundering herd during HPA scale-up | ✓ Good — Applied across all listeners, prevents coordinator overload |
| W3C Trace Context propagation through Redis | Standard propagation format, interoperable with observability tools | ✓ Good — 16 spans instrumented, trace context in Redis Streams |
| **InnerTube API over official YouTube API** | Eliminates quota constraints (10,000 units/day bottleneck), faster deletion detection | ✓ Good — Quota-free unlimited access, drop-in replacement architecture proven |
| **Drop-in replacement strategy (RawChatMessage compatibility)** | Zero downstream changes, maintains contract with message-processor | ✓ Good — Byte-for-byte schema match, contract validation tests pass |
| **Batch deletion time-window aggregation (100ms)** | Balances detection speed with false positive risk | ✓ Good — Detects bans/timeouts reliably, 5+ threshold works well |
| **Deletion buffer with 500ms delay** | Handles race condition (deletion arrives before original message) | ✓ Good — Buffering works, suppression architecture deferred to future |
| **Argo Rollouts for canary deployment** | Automatic promotion/rollback based on Prometheus metrics | ✓ Good — Infrastructure ready, 10%→50%→100% progression with <1% error threshold |
| **HTML parsing for stream discovery** | InnerTube browse endpoint unstable, HTML canonical link reliable | ✓ Good — Premiere filtering works, 15-minute timeout appropriate |
| **Source-manager leadership coordination** | Reuse existing leader election, single source of truth for active streams | ✓ Good — Async discovery, Redis caching, graceful lifecycle management |
| **New share-service (not extend overlay-manager)** | Share management has distinct concerns (permission model, premium enforcement, cycle detection) | ✓ Good — Clean separation, share-service owns all sharing logic |
| **ON DELETE RESTRICT for share FK** | Prevents data loss on user deletion; application layer handles cleanup | ✓ Good — Explicit cascade logic, no accidental data loss |
| **DFS cycle detection for share acceptance** | Prevents circular share dependencies (A shares with B, B shares with A) | ✓ Good — SELECT FOR UPDATE transaction prevents concurrent acceptance races |
| **SQL UNION fan-out in FindOverlaysForMessage** | Message routing to recipient overlays happens at query level, no separate routing service | ✓ Good — Zero downstream changes, unified enrichment pipeline |
| **60s debounce in LifecycleSubscriber** | Prevents phantom expiry during Twitch stream restart/category change | ✓ Good — Eliminates false positive share expiry during brief offline events |
| **Kick lifecycle: gracefully disabled** | No reliable webhook/API found during research; disabling "This stream" for Kick in AcceptModal | ✓ Good — Honest limitation surfaced at UI level with clear messaging |
| **Redis lifecycle relay (not direct DB writes from listeners)** | Listeners publish lifecycle:stream_end to Redis; share-service subscribes and expires | ✓ Good — Loose coupling; Redis ping failure is non-fatal, service continues |
| **No premium check on GET /shares/accepted** | Read-only informational endpoint; premium gates mutations only | ✓ Good — Correct authorization model, no unnecessary gatekeeping |

---
*Last updated: 2026-03-11 after v1.3 milestone*
