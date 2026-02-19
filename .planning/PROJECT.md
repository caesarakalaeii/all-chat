# All-Chat: Listener Load Balancing

## What This Is

All-Chat is a cloud-native platform that aggregates live chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and displays them in unified overlays for streamers. This milestone implements intelligent load distribution across listener instances to enable efficient horizontal scaling, ensuring high-volume channels don't overload individual pods while maintaining predictable channel assignment.

## Core Value

Listener instances must efficiently distribute channel workload based on actual message volume, enabling cost-effective scaling and reliable service for both small and high-traffic streams.

## Current Milestone: v1.1 Listener Load Balancing

**Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services (Twitch, YouTube, Kick, TikTok).

**Target features:**
- Hash-based channel assignment for predictability
- Per-pod load monitoring (messages/sec, active channels)
- Automatic rebalancing when load imbalance detected
- Graceful channel migration without message loss
- Unified load balancing across all 4 platform listeners

## Requirements

### Validated

<!-- Shipped in v1.0 -->

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
- ✓ Message deletion support (Twitch, YouTube, Kick) — v1.0
- ✓ Message ID tracking and registry — v1.0
- ✓ Deletion event propagation pipeline — v1.0
- ✓ WebSocket reconnection replay buffer — v1.0

### Active

<!-- v1.1 milestone scope -->

- [ ] Hash-based channel sharding (consistent hashing across pods)
- [ ] Per-pod load metrics collection (messages/sec, channel count, memory)
- [ ] Load imbalance detection (threshold-based triggers)
- [ ] Automatic hot channel identification (high message rate detection)
- [ ] Graceful channel migration (no message loss during rebalancing)
- [ ] Channel assignment registry (Redis-based, pod → channels mapping)
- [ ] Rebalancing coordinator (leader-elected, triggers migrations)
- [ ] Apply load balancing to all 4 listener types (Twitch, YouTube, Kick, TikTok)
- [ ] Kubernetes HPA integration (scale based on aggregate load)
- [ ] Load balancing observability (metrics, dashboards, alerts)

### Out of Scope

- Cross-region load balancing — Single Kubernetes cluster only for v1.1
- Predictive scaling based on stream schedules — Reactive only for now
- Custom load metrics from user configuration — Use message rate only
- Channel affinity/pinning — All channels are rebalanceable
- Multi-tenancy isolation — Single Redis instance shared across pods

## Context

**Current Listener Behavior (Problem):**
- Multiple listener pods running per platform (observed: high Twitch replica count)
- Each pod subscribes to ALL channels (full list duplication)
- No coordination between pods
- Inefficient resource usage: 3 pods × 100 channels = 300 total connections
- Can't scale efficiently: adding pods doesn't reduce per-pod load

**Desired Behavior (Solution):**
- Channels distributed across available pods
- Each channel handled by exactly one pod
- Load-aware: high-volume channels don't overload single pod
- Predictable: channel → pod mapping is deterministic (hash-based)
- Dynamic: rebalances when load imbalance detected or pods scale

**Current Message Flow:**
1. Platform → Listener (IRC/HTTP/WebSocket) ← **LOAD BALANCING HERE**
2. Listener → Redis Streams (`stream:raw-messages`)
3. Message Processor consumes, normalizes, enriches, routes
4. Message Processor → Redis Pub/Sub per overlay (`overlay:{id}`)
5. API Gateway subscribes, broadcasts via WebSocket
6. Frontend overlay displays messages

**Listener Service Details:**
- Twitch: IRC client (go-twitch-irc), maintains persistent connections per channel
- YouTube: HTTP polling per live video, quota-tracked
- Kick: WebSocket (Pusher), one connection per channel
- TikTok: Unofficial library, WebSocket per stream

**Technical Environment:**
- Go 1.25.6 microservices with Standard Go Layout
- Redis 7 (Streams for queuing, Pub/Sub for broadcast)
- Kubernetes with HPA (Horizontal Pod Autoscaler)
- Existing source-manager service (leader election, active source registry)

## Constraints

- **Tech Stack**: Go 1.25.6, existing microservices, Redis 7, Kubernetes — No new infrastructure dependencies
- **Backward Compatibility**: Must not break existing message flow — Existing single-pod deployments still work
- **Zero Downtime**: Channel migration must be lossless — No messages dropped during rebalancing
- **Platform Connection Limits**: Respect platform rate limits and connection quotas — YouTube API quota especially
- **Stateless Services**: Listeners remain stateless — All state in Redis, not in-memory

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Hybrid hash-based + load-aware approach | Predictable under normal load, adapts to real-world usage patterns | — Pending |
| Consistent hashing for channel assignment | Minimizes reassignments when pods scale, predictable placement | — Pending |
| Redis for assignment registry | Centralized state, atomic updates, survives pod restarts | — Pending |
| Reuse source-manager for coordination | Already has leader election, knows active sources, avoid duplication | — Pending |

---
*Last updated: 2026-02-19 after v1.1 milestone start*
