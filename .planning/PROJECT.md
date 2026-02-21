# All-Chat

## What This Is

All-Chat is a cloud-native platform that aggregates live chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and displays them in unified overlays for streamers. The platform now features intelligent load distribution with hybrid hash-based sharding and automatic rebalancing, enabling efficient horizontal scaling for high-volume streams while maintaining zero-message-loss guarantees.

## Core Value

Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing and auto-scaling.

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

### Active

<!-- Current scope — will be populated in next milestone -->

(Empty — awaiting next milestone planning)

### Out of Scope

- Cross-region load balancing — Single Kubernetes cluster sufficient for current scale
- Predictive scaling based on stream schedules — Reactive scaling meets needs
- Custom load metrics from user configuration — Message rate provides accurate signal
- Channel affinity/pinning — Rebalancing flexibility more valuable than pinning
- Multi-tenancy isolation — Single-tenant deployment model
- YouTube load balancing — Quota is bottleneck, not connections (existing leader election sufficient)

## Context

**Current State (v1.1 shipped):**
- 7 microservices deployed in Kubernetes (api-gateway, auth-service, emote-service, message-processor, overlay-manager, source-manager, token-refresh-service)
- 4 listener services with load balancing (twitch-listener, kick-listener, tiktok-listener, youtube-listener)
- Coordination code: 5,338 lines (bounded-load consistent hashing, migration protocol, rebalancing)
- Shared infrastructure: 1,123 lines (metrics, distributed tracing)
- 110 files changed in v1.1 (+30,501 lines, -253 lines)

**Load Balancing Implementation:**
- **Coordinator:** Kubernetes Lease-based leader election, bounded-load consistent hashing (1.25x average limit)
- **Migration:** Overlap protocol (new pod connects before old disconnects), zero-loss guarantee via first-message confirmation
- **Rebalancing:** Composite load scoring (70% message rate + 30% channel count), automatic hot channel redistribution (>3x average rate)
- **Safeguards:** 5-minute cooldown, 20% per-operation limit, thrashing detection, distributed lock coordination
- **HPA:** Startup jitter (0-30s) prevents thundering herd, filtered assignment count for readiness probes
- **Observability:** 16 distributed tracing spans, Grafana dashboards with Pod×Platform and Pod×Time heatmaps, Prometheus alerts

**Technical Environment:**
- Go 1.25.6 microservices with Standard Go Layout
- Redis 7 (Streams for queuing, Pub/Sub for broadcast, assignment registry)
- Kubernetes with CloudNativePG (PostgreSQL), HPA, Horizontal Pod Autoscaler
- OpenTelemetry tracing, Prometheus metrics, Grafana dashboards

**Known Issues / Technical Debt:**
- None identified in v1.1 milestone audit
- Human verification pending for: HPA scale-up behavior (cluster testing), Jaeger trace visualization, Grafana dashboard rendering

## Constraints

- **Tech Stack**: Go 1.25.6, existing microservices, Redis 7, Kubernetes — No new infrastructure dependencies
- **Backward Compatibility**: Must not break existing message flow — Single-pod deployments still supported
- **Zero Downtime**: Channel migration must be lossless — Enforced via overlap protocol and confirmation signaling
- **Platform Connection Limits**: Respect platform rate limits and connection quotas — YouTube API quota especially
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

---
*Last updated: 2026-02-21 after v1.1 milestone completion*
