# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 5-8 (shipped 2026-02-21)
- 🚧 **v1.2 InnerTube YouTube Listener** — Phases 9-13 (in progress)

## Phases

<details>
<summary>✅ v1.0 Message Deletion Support (Phases 1-3) — SHIPPED 2026-02-18</summary>

### Phase 1: Foundation + Twitch
**Goal**: Establish message deletion infrastructure with Twitch platform
**Plans**: 5/5 complete
**Status**: Complete (2026-02-18)

### Phase 2: YouTube Integration
**Goal**: YouTube deletion support via polling
**Plans**: 2/2 complete
**Status**: Complete (2026-02-18)

### Phase 3: Kick Integration + Edge Cases
**Goal**: Kick WebSocket deletion events and reconnection handling
**Plans**: 4/4 complete
**Status**: Complete (2026-02-18)

**Note**: Phase 4 (TikTok Integration) was deferred, ending v1.0 at 80% completion.

**Archive**: [v1.0 details](milestones/) (if archived)

</details>

<details>
<summary>✅ v1.1 Listener Load Balancing (Phases 5-8) — SHIPPED 2026-02-21</summary>

**Milestone Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services, enabling cost-effective scaling and reliable service for high-volume streams.

### Phase 5: Sharding Infrastructure & Coordinator Service
**Goal**: Production-ready consistent hashing and coordinator service with split-brain prevention
**Plans**: 5/5 complete
**Status**: Complete (2026-02-19)

### Phase 6: Connection Management & Migration Protocol
**Goal**: All platform listeners integrate with coordinator and support graceful zero-loss channel migration
**Plans**: 8/8 complete
**Status**: Complete (2026-02-20)

### Phase 7: Dynamic Rebalancing & HPA Integration
**Goal**: Automatic load-aware rebalancing with safeguards against thundering herd and quota exhaustion
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

### Phase 8: Observability & Production Readiness
**Goal**: Comprehensive metrics, distributed tracing, Grafana dashboards, and alerting for production operations
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

**Delivered:**
- Bounded-load consistent hashing with Kubernetes Lease-based coordinator
- Zero-loss channel migration protocol across all platforms (Twitch, Kick, TikTok)
- Load-aware automatic rebalancing with composite scoring and cooldown safeguards
- Comprehensive observability with 16 distributed tracing spans, Grafana dashboards, and Prometheus alerts

**Archive**: [v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md) | [v1.1-REQUIREMENTS.md](milestones/v1.1-REQUIREMENTS.md) | [v1.1-MILESTONE-AUDIT.md](milestones/v1.1-MILESTONE-AUDIT.md)

</details>

### 🚧 v1.2 InnerTube YouTube Listener (In Progress)

**Milestone Goal:** Build quota-free YouTube listener using InnerTube API as drop-in replacement for official API listener, maintaining identical downstream behavior while eliminating quota limitations.

- [ ] **Phase 9: Core Ingestion PoC** - Validate InnerTube viability with basic message flow
- [x] **Phase 10: Production Minimum** - Dynamic stream management and HTTP control plane (completed 2026-02-21)
- [x] **Phase 11: Contract Validation** - Prove behavioral equivalence with official listener (completed 2026-02-21)
- [ ] **Phase 12: Production Rollout** - Canary deployment with monitoring and rollback
- [ ] **Phase 13: Feature Parity** - Deletion events and advanced metrics

## Phase Details

### Phase 9: Core Ingestion PoC
**Goal**: Validate InnerTube API viability by establishing basic message flow from InnerTube to Redis Streams
**Depends on**: Nothing (first phase of v1.2)
**Requirements**: CORE-01, CORE-02, CORE-03, CORE-04, CORE-05, CORE-06, EVENT-01, EVENT-02
**Success Criteria** (what must be TRUE):
  1. Service can poll live YouTube chat via InnerTube and publish to Redis Streams (chat:raw)
  2. Message-processor consumes InnerTube messages without code changes (RawChatMessage contract maintained)
  3. Health checks return correct status (/health/live returns 200, /health/ready checks Redis)
  4. Messages contain user metadata (username, avatar, badges) in expected format
**Plans**: 5 plans

Plans:
- [ ] 09-01-PLAN.md — InnerTube client and message parser with strict schema validation
- [ ] 09-02-PLAN.md — Polling loop with continuation tokens and exponential backoff
- [ ] 09-03-PLAN.md — Service integration with Redis publishing and health checks
- [ ] 09-04-PLAN.md — Fix health handler test mock signature (gap closure)
- [ ] 09-05-PLAN.md — Fix backoff error classification and jitter (gap closure)

### Phase 10: Production Minimum
**Goal**: Enable dynamic stream management and production lifecycle behaviors
**Depends on**: Phase 9
**Requirements**: STREAM-01, STREAM-02, STREAM-03, STREAM-04, STREAM-05, STREAM-06, STREAM-07, EVENT-03, EVENT-04, EVENT-05, EVENT-06, EVENT-07
**Success Criteria** (what must be TRUE):
  1. Service can discover latest live stream from channel ID and filter out premieres
  2. Service can start/stop monitoring streams via source-manager integration (no HTTP API)
  3. Service detects stream offline and stops polling automatically
  4. Service reconnects on network errors with exponential backoff (no crash on transient failures)
  5. Service handles SIGTERM gracefully (cleanup connections, flush Redis buffers within 25s)
  6. Service parses all event types (Super Chat, Super Sticker, memberships, milestones, tickers)
**Plans**: 4 plans

Plans:
- [ ] 10-01-PLAN.md — Stream discovery (HTML parsing) and Redis persistence for channel→video mappings
- [ ] 10-02-PLAN.md — Advanced event parsing (Super Chat, Super Sticker, memberships, milestones, tickers)
- [ ] 10-03-PLAN.md — Source-manager integration with async discovery and leadership coordination
- [ ] 10-04-PLAN.md — Production lifecycle (offline detection, auto-resume, graceful shutdown)

### Phase 11: Contract Validation
**Goal**: Prove behavioral equivalence with official youtube-listener through comprehensive contract testing
**Depends on**: Phase 10
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, DEL-01, DEL-02
**Success Criteria** (what must be TRUE):
  1. Schema tests validate RawChatMessage JSON output matches official listener byte-for-byte (100+ golden file comparisons)
  2. Dual-listener integration test shows <0.1% mismatch rate on same live stream over 24 hours
  3. Lifecycle tests verify connection gating and stream offline detection match official behavior
  4. Deletion event tests validate single message deletion detection and emission
**Plans**: 4 plans in 1 wave (all independent)

Plans:
- [ ] 11-01-PLAN.md — Golden file capture and schema validation infrastructure (TEST-01)
- [ ] 11-02-PLAN.md — 24-hour dual-listener Kubernetes integration test (TEST-02)
- [ ] 11-03-PLAN.md — Lifecycle behavior tests with testcontainers (TEST-03, TEST-04)
- [ ] 11-04-PLAN.md — Deletion event detection and emission tests (DEL-01, DEL-02)

### Phase 12: Production Rollout
**Goal**: Deploy to production with gradual canary rollout, monitoring, and automatic rollback
**Depends on**: Phase 11
**Requirements**: PROD-01, PROD-02, PROD-03, PROD-04, PROD-05
**Success Criteria** (what must be TRUE):
  1. Kubernetes manifests deployed with 10% traffic canary (2 pods innertube, 8 pods official)
  2. Prometheus metrics track messages/sec, errors, reconnections with InnerTube-specific labels
  3. Error rate monitoring triggers automatic rollback when >5% error rate detected
  4. Documentation explains ToS disclosure (InnerTube unofficial API) and Docker image swap process
  5. Canary promotes to 50% then 100% after error rate validation (<1% threshold)
**Plans**: 3 plans in 2 waves

Plans:
- [ ] 12-01-PLAN.md — Prometheus metrics implementation for InnerTube listener
- [ ] 12-02-PLAN.md — Argo Rollouts manifests with AnalysisTemplate and canary strategy
- [ ] 12-03-PLAN.md — Grafana dashboard, deployment runbook, and troubleshooting guide

### Phase 13: Feature Parity
**Goal**: Add deletion event detection and advanced metrics leveraging InnerTube advantages
**Depends on**: Phase 12
**Requirements**: DEL-03, DEL-04, DEL-05
**Success Criteria** (what must be TRUE):
  1. Service detects batch deletion events (ban/timeout) and emits with deletion_type="batch"
  2. Service buffers deletion events to handle race conditions (deletion arrives before original message)
  3. Metrics track per-channel message rate (1-minute rolling average via PromQL), error breakdown by type (http, parse, network, rate_limit, redis)
  4. Batch deletion detector synthesizes single event from 5+ deletions in 100ms window
**Plans**: 5 plans in 1 wave (3 original + 2 gap closure)

Plans:
- [ ] 13-01-PLAN.md — Batch deletion detector with time-windowed aggregation
- [ ] 13-02-PLAN.md — Deletion event buffer with circular buffer and 500ms delay
- [ ] 13-03-PLAN.md — Advanced metrics with network error tracking and per-channel message rate
- [ ] 13-04-PLAN.md — Wire batch detection results to parser emission (gap closure)
- [ ] 13-05-PLAN.md — Fix publisher test signature issues (gap closure)

## Progress

**Execution Order:**
Phases execute in numeric order: 9 → 10 → 11 → 12 → 13

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation + Twitch | v1.0 | 5/5 | Complete | 2026-02-18 |
| 2. YouTube Integration | v1.0 | 2/2 | Complete | 2026-02-18 |
| 3. Kick Integration + Edge Cases | v1.0 | 4/4 | Complete | 2026-02-18 |
| 5. Sharding Infrastructure & Coordinator | v1.1 | 5/5 | Complete | 2026-02-19 |
| 6. Connection Management & Migration | v1.1 | 8/8 | Complete | 2026-02-20 |
| 7. Dynamic Rebalancing & HPA | v1.1 | 4/4 | Complete | 2026-02-20 |
| 8. Observability & Production Readiness | v1.1 | 4/4 | Complete | 2026-02-20 |
| 9. Core Ingestion PoC | v1.2 | 5/5 | Complete | 2026-02-21 |
| 10. Production Minimum | v1.2 | 4/4 | Complete | 2026-02-21 |
| 11. Contract Validation | v1.2 | 4/4 | Complete | 2026-02-21 |
| 12. Production Rollout | v1.2 | 3/3 | Complete | 2026-03-05 |
| 13. Feature Parity | v1.2 | 0/5 | Not started | - |

---
*Last updated: 2026-03-05 after Phase 13 gap closure planning*
