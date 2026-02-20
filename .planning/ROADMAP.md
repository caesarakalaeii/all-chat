# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** - Phases 1-3 (partial, archived 2026-02-18)
- 🚧 **v1.1 Listener Load Balancing** - Phases 5-8 (in progress)

## Overview

**v1.1 Milestone:** This milestone implements distributed channel sharding across listener instances, enabling efficient horizontal scaling for high-volume streams. The journey moves from foundation (consistent hashing and coordinator service) through safe migration protocols (zero message loss) to production-grade scaling (HPA integration, rebalancing) and observability (metrics, tracing, dashboards).

## Phases

**Phase Numbering:**
- Integer phases (5, 6, 7, 8): v1.1 planned milestone work
- Decimal phases (5.1, 5.2): Urgent insertions (marked with INSERTED)
- v1.0 ended at Phase 3 (partial completion)

<details>
<summary>✅ v1.0 Message Deletion Support (Phases 1-3) - PARTIAL (archived 2026-02-18)</summary>

### Phase 1: Foundation + Twitch
**Goal**: Establish message deletion infrastructure with Twitch platform
**Plans**: 5 plans
**Status**: Complete (2026-02-18)

Plans:
- [x] 01-01: Message ID Registry infrastructure (Redis hash, O(1) lookups)
- [x] 01-02: Twitch deletion event capture (IRC handlers)
- [x] 01-03: Message processor deletion handling
- [x] 01-04: Frontend deletion handling (WebSocket events)
- [x] 01-05: End-to-end integration testing

### Phase 2: YouTube Integration
**Goal**: YouTube deletion support via polling
**Plans**: 2 plans
**Status**: Complete (2026-02-18)

Plans:
- [x] 02-01: YouTube parser deletion event mapping
- [x] 02-02: YouTube registry integration

### Phase 3: Kick Integration + Edge Cases
**Goal**: Kick WebSocket deletion events and reconnection handling
**Plans**: 4 plans
**Status**: Complete (2026-02-18)

Plans:
- [x] 03-01: Kick deletion event handler
- [x] 03-02: Deletion replay buffer (Redis sorted set)
- [x] 03-03: Load testing and documentation
- [x] 03-04: Replay buffer test compilation fix

**Note**: Phase 4 (TikTok Integration) was deferred, ending v1.0 at 80% completion.

</details>

### 🚧 v1.1 Listener Load Balancing (In Progress)

**Milestone Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services, enabling cost-effective scaling and reliable service for high-volume streams.

- [ ] **Phase 5: Sharding Infrastructure & Coordinator** - Foundation with split-brain prevention
- [ ] **Phase 6: Connection Management & Migration** - Graceful channel migration across all platforms
- [x] **Phase 7: Dynamic Rebalancing & HPA** - Automatic load distribution and scaling (completed 2026-02-20)
- [ ] **Phase 8: Observability & Production Readiness** - Comprehensive metrics, tracing, and dashboards

### Phase 5: Sharding Infrastructure & Coordinator Service
**Goal**: Production-ready consistent hashing and coordinator service with split-brain prevention
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: SHARD-01, SHARD-02, SHARD-03, SHARD-04, SHARD-05, SHARD-06, SHARD-07, SHARD-08, REBAL-08
**Success Criteria** (what must be TRUE):
  1. Coordinator computes channel-to-pod assignments using bounded-load consistent hashing (no pod exceeds 1.25x average load)
  2. Assignment registry stores mappings in Redis with O(1) channel lookup and O(log N) load queries
  3. Leader election via Kubernetes Lease API prevents split-brain (only one coordinator computes assignments at a time)
  4. Failed pod channels redistribute to healthy pods within 60 seconds of heartbeat timeout
  5. Coordinator survives chaos testing (network partitions, pod failures, leader changes)
**Plans**: 5 plans

Plans:
- [ ] 05-01-PLAN.md — Bounded-load consistent hashing + Redis assignment registry (TDD)
- [ ] 05-02-PLAN.md — Kubernetes Lease coordinator with reconciliation loop
- [ ] 05-03-PLAN.md — Heartbeat monitoring with 15s timeout detection
- [ ] 05-04-PLAN.md — HTTP endpoints for assignment queries and heartbeat publishing
- [ ] 05-05-PLAN.md — Chaos testing validation (checkpoint for human verification)

### Phase 6: Connection Management & Migration Protocol
**Goal**: All platform listeners (Twitch, Kick, TikTok) integrate with coordinator and support graceful zero-loss channel migration
**Depends on**: Phase 5
**Requirements**: MIGRATE-01, MIGRATE-02, MIGRATE-03, MIGRATE-04, MIGRATE-05, MIGRATE-06, TWITCH-01, TWITCH-02, TWITCH-03, TWITCH-04, TWITCH-05, TWITCH-06, TWITCH-07, KICK-01, KICK-02, KICK-03, KICK-04, KICK-05, TIKTOK-01, TIKTOK-02, TIKTOK-03, TIKTOK-04, TIKTOK-05
**Success Criteria** (what must be TRUE):
  1. Listener pods query coordinator on startup and connect ONLY to assigned channels (not all channels)
  2. Channel migration uses overlap protocol (new pod connects before old pod disconnects, zero message loss)
  3. Platform-specific connection state migrates correctly (Twitch IRC JOIN list, Kick Pusher subscription IDs, TikTok connection state)
  4. HPA can scale Twitch listener from 1 to 5 replicas with all pods reporting ready (fixes current 1/5 ready issue)
  5. Migration events publish to Redis Streams with sequence numbers for gap detection
**Plans**: 8 plans

Plans:
- [x] 06-01-PLAN.md — Shared migration infrastructure (coordinator client, Redis Pub/Sub subscriber, migration event models)
- [x] 06-02-PLAN.md — Twitch listener integration (startup query, heartbeat, multiple IRC connections, migration, readiness)
- [x] 06-03-PLAN.md — Kick listener integration (startup query, heartbeat, Pusher subscription management, migration, readiness)
- [x] 06-04-PLAN.md — TikTok listener integration (TypeScript coordinator client, assignment filtering, migration, HPA 1-3 replicas)
- [x] 06-05-PLAN.md — Migration observability + coordinator publisher (Redis Pub/Sub + Streams, migration triggers, confirmation wait)
- [x] 06-06-PLAN.md — End-to-end migration testing (HPA scale tests, zero-loss validation, checkpoint for human verification)
- [x] 06-07-PLAN.md — Gap closure: Fix migration confirmation publishing (wire firstMessageChan, implement Redis Streams publishing)
- [x] 06-08-PLAN.md — Gap closure: Fix readiness probe to use filtered assignment count (unblocks HPA scaling and migration confirmation testing)

### Phase 7: Dynamic Rebalancing & HPA Integration
**Goal**: Automatic load-aware rebalancing with safeguards against thundering herd and quota exhaustion
**Depends on**: Phase 6
**Requirements**: REBAL-01, REBAL-02, REBAL-03, REBAL-04, REBAL-05, REBAL-06, REBAL-07
**Success Criteria** (what must be TRUE):
  1. System monitors per-pod load (messages/sec, channel count) and triggers rebalancing when imbalance ratio exceeds 0.5
  2. Hot channels (>3x average message rate) automatically redistribute from overloaded pods to underutilized pods
  3. Rebalancing enforces cooldown (5 minutes) and limits (max 20% channels per operation) to prevent thrashing
  4. HPA scale-up from 2 to 10 pods completes without Redis lock contention or YouTube quota exhaustion
  5. Staggered pod startup with jitter prevents thundering herd on topology changes
**Plans**: 4 plans

Plans:
- [ ] 07-01-PLAN.md — Load monitoring + imbalance detection (composite load scores, threshold gating)
- [ ] 07-02-PLAN.md — Proportional rebalancing algorithm (channel selection, 20% limit enforcement)
- [ ] 07-03-PLAN.md — Cooldown + thrashing safeguards (5-min cooldown, escalation overrides)
- [ ] 07-04-PLAN.md — HPA coordination + staggered startup (distributed locks, scale event detection, jitter)

### Phase 8: Observability & Production Readiness
**Goal**: Comprehensive metrics, distributed tracing, Grafana dashboards, and alerting for production operations
**Depends on**: Phase 7
**Requirements**: METRICS-01, METRICS-02, METRICS-03, METRICS-04, METRICS-05, METRICS-06, METRICS-07, METRICS-08, METRICS-09, METRICS-10, METRICS-11, TRACE-01, TRACE-02, TRACE-03, TRACE-04, TRACE-05, TRACE-06
**Success Criteria** (what must be TRUE):
  1. Prometheus metrics expose per-pod channel count, message rate, rebalancing events, and migration success/failure
  2. Grafana dashboard visualizes channel distribution (heatmap), rebalancing timeline, and load imbalance ratio
  3. Prometheus alerts trigger on critical conditions (imbalance >0.7, split-brain, rebalancing thrashing)
  4. OpenTelemetry traces show end-to-end migration (all phases) and rebalancing decisions in Jaeger UI
  5. Simulated failure scenarios (split-brain, migration failure, quota exhaustion) can be diagnosed from metrics alone in <5 minutes
**Plans**: TBD

Plans:
- [ ] 08-01: TBD (planning required)

## Progress

**Execution Order:**
Phases execute in numeric order: 5 → 6 → 7 → 8

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation + Twitch | v1.0 | 5/5 | Complete | 2026-02-18 |
| 2. YouTube Integration | v1.0 | 2/2 | Complete | 2026-02-18 |
| 3. Kick Integration + Edge Cases | v1.0 | 4/4 | Complete | 2026-02-18 |
| 5. Sharding Infrastructure & Coordinator | v1.1 | 5/5 | Complete | 2026-02-19 |
| 6. Connection Management & Migration | v1.1 | 8/8 | Complete | 2026-02-20 |
| 7. Dynamic Rebalancing & HPA | 4/4 | Complete   | 2026-02-20 | - |
| 8. Observability & Production Readiness | v1.1 | 0/? | Not started | - |

---
*Last updated: 2026-02-20 after Phase 7 planning complete*
