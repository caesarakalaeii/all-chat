# Requirements: All-Chat Listener Load Balancing

**Defined:** 2026-02-19
**Core Value:** Listener instances must efficiently distribute channel workload based on actual message volume, enabling cost-effective scaling and reliable service for both small and high-traffic streams.

## Milestone v1.1 Requirements

### Core Sharding Infrastructure

- [x] **SHARD-01**: System computes channel-to-pod assignment using consistent hashing with virtual nodes
- [x] **SHARD-02**: System stores channel assignments in Redis registry with O(1) lookup performance
- [x] **SHARD-03**: Listener pod queries assignment registry on startup to determine which channels to connect
- [x] **SHARD-04**: Listener pod publishes heartbeat to Redis every 10 seconds with pod ID and timestamp
- [x] **SHARD-05**: System detects pod failure when heartbeat missing for 15 seconds (user override from 30s - per CONTEXT.md for fast stream recovery)
- [x] **SHARD-06**: System redistributes channels from failed pod to healthy pods within 60 seconds
- [x] **SHARD-07**: System uses Kubernetes Lease API for coordinator leader election (not Redlock)
- [x] **SHARD-08**: System uses fencing tokens to prevent split-brain during leader failover

### Rebalancing & Coordination

- [x] **REBAL-01**: System monitors per-pod message rate (messages/sec) every 30 seconds
- [x] **REBAL-02**: System calculates load imbalance ratio (max_load / avg_load)
- [x] **REBAL-03**: System triggers automatic rebalancing when imbalance ratio exceeds 0.5
- [x] **REBAL-04**: System identifies hot channels (channels with >3x average message rate)
- [x] **REBAL-05**: System reassigns hot channels from overloaded pods to underutilized pods
- [x] **REBAL-06**: System enforces 5-minute cooldown between rebalancing operations
- [x] **REBAL-07**: System limits rebalancing to maximum 20% of channels per operation
- [x] **REBAL-08**: Coordinator service extends existing source-manager with rebalancing logic

### Channel Migration

- [x] **MIGRATE-01**: System implements overlap migration pattern (new pod connects before old disconnects)
- [x] **MIGRATE-02**: New pod subscribes to channel and waits for first message before signaling ready
- [x] **MIGRATE-03**: Old pod receives migration signal and gracefully disconnects after 45 seconds
- [x] **MIGRATE-04**: System guarantees zero message loss during migration (no dropped messages)
- [x] **MIGRATE-05**: System publishes migration events to Redis Streams for observability
- [x] **MIGRATE-06**: System uses sequence numbers per channel to detect message gaps during migration

### Twitch Load Balancing (CRITICAL)

- [x] **TWITCH-01**: Twitch listener queries shard coordinator for assigned channels on startup
- [x] **TWITCH-02**: Twitch listener connects to IRC only for assigned channels (not all channels)
- [x] **TWITCH-03**: Twitch listener supports multiple IRC connections when assigned >100 channels
- [x] **TWITCH-04**: Twitch listener stores IRC JOIN list state in ConnectionSnapshot for migration
- [x] **TWITCH-05**: Twitch listener gracefully parts IRC channels during migration (sends PART command)
- [x] **TWITCH-06**: System allows HPA to scale Twitch listener from 1 to 5 replicas successfully
- [x] **TWITCH-07**: All Twitch listener pods report ready status (fixes current 1/5 ready issue)

### Kick Load Balancing

- [x] **KICK-01**: Kick listener queries shard coordinator for assigned channels on startup
- [x] **KICK-02**: Kick listener connects to Pusher WebSocket only for assigned channels
- [x] **KICK-03**: Kick listener stores Pusher subscription IDs in ConnectionSnapshot for migration
- [x] **KICK-04**: Kick listener gracefully unsubscribes from channels during migration
- [x] **KICK-05**: System allows HPA to scale Kick listener from 1 to 5 replicas successfully

### TikTok Load Balancing

- [x] **TIKTOK-01**: TikTok listener queries shard coordinator for assigned channels on startup
- [x] **TIKTOK-02**: TikTok listener connects via tiktok-live-connector only for assigned channels
- [x] **TIKTOK-03**: TikTok listener stores connection state in ConnectionSnapshot for migration
- [x] **TIKTOK-04**: TikTok listener handles connection state migration for unofficial library
- [x] **TIKTOK-05**: System allows HPA to scale TikTok listener from 1 to 3 replicas successfully

### Observability & Metrics

- [x] **METRICS-01**: Each listener pod exposes Prometheus metrics at /metrics endpoint
- [x] **METRICS-02**: System tracks per-pod channel count as Gauge metric (shard_channel_count)
- [x] **METRICS-03**: System tracks per-pod message rate as Counter metric (shard_messages_total)
- [ ] **METRICS-04**: System tracks rebalancing events as Counter metric (shard_rebalancing_total)
- [x] **METRICS-05**: System tracks migration success/failure as Counter metrics (shard_migration_success/failure)
- [ ] **METRICS-06**: System tracks load imbalance ratio as Gauge metric (shard_imbalance_ratio)
- [ ] **METRICS-07**: Grafana dashboard visualizes channel distribution across pods (heatmap)
- [ ] **METRICS-08**: Grafana dashboard shows rebalancing timeline and migration events
- [ ] **METRICS-09**: Prometheus alert triggers when imbalance ratio >0.7 for 10 minutes
- [ ] **METRICS-10**: Prometheus alert triggers on split-brain detection (multiple leaders)
- [ ] **METRICS-11**: Prometheus alert triggers on rebalancing thrashing (>3 rebalances in 15min)

### Distributed Tracing

- [x] **TRACE-01**: System instruments channel assignment operations with OpenTelemetry spans
- [x] **TRACE-02**: System instruments migration operations with OpenTelemetry spans
- [ ] **TRACE-03**: System instruments rebalancing operations with OpenTelemetry spans
- [x] **TRACE-04**: System propagates trace context through Redis Streams messages
- [ ] **TRACE-05**: Jaeger UI shows end-to-end trace for channel migration (all phases)
- [ ] **TRACE-06**: Jaeger UI shows trace for rebalancing decision (trigger → completion)

## Future Requirements (Deferred)

### Cross-Region Load Balancing
- **REGION-01**: Channel assignment considers pod region for latency optimization
- **REGION-02**: Rebalancing prefers same-region migrations to minimize latency

### Predictive Scaling
- **PRED-01**: System learns stream schedules and pre-scales listeners before high-traffic events
- **PRED-02**: System integrates with Twitch/YouTube APIs to detect upcoming high-traffic streams

### Advanced HPA Integration
- **HPA-01**: HPA uses custom metrics (channel count, messages/sec) instead of CPU
- **HPA-02**: Prometheus Adapter exposes shard metrics for HPA consumption

## Out of Scope

| Feature | Reason |
|---------|--------|
| YouTube load balancing | Quota is bottleneck, not connections - keep existing leader election |
| Manual channel pinning | All channels are rebalanceable for optimal load distribution |
| Multi-tenancy isolation | Single Redis instance shared across pods - defer to future |
| Cross-cluster sharding | Single Kubernetes cluster only for v1.1 |
| Real-time rebalancing | 60-second migration window acceptable for chat aggregation |
| Perfect load balance | Causes thrashing - accept 0.5 imbalance threshold |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SHARD-01 | Phase 5 | Complete |
| SHARD-02 | Phase 5 | Complete |
| SHARD-03 | Phase 5 | Complete |
| SHARD-04 | Phase 5 | Complete |
| SHARD-05 | Phase 5 | Complete |
| SHARD-06 | Phase 5 | Complete |
| SHARD-07 | Phase 5 | Complete |
| SHARD-08 | Phase 5 | Complete |
| REBAL-01 | Phase 7 | Complete |
| REBAL-02 | Phase 7 | Complete |
| REBAL-03 | Phase 7 | Complete |
| REBAL-04 | Phase 7 | Complete |
| REBAL-05 | Phase 7 | Complete |
| REBAL-06 | Phase 7 | Complete |
| REBAL-07 | Phase 7 | Complete |
| REBAL-08 | Phase 5 | Complete |
| MIGRATE-01 | Phase 6 | Complete |
| MIGRATE-02 | Phase 6 | Complete |
| MIGRATE-03 | Phase 6 | Complete |
| MIGRATE-04 | Phase 6 | Complete |
| MIGRATE-05 | Phase 6 | Complete |
| MIGRATE-06 | Phase 6 | Complete |
| TWITCH-01 | Phase 6 | Complete |
| TWITCH-02 | Phase 6 | Complete |
| TWITCH-03 | Phase 6 | Complete |
| TWITCH-04 | Phase 6 | Complete |
| TWITCH-05 | Phase 6 | Complete |
| TWITCH-06 | Phase 6 | Complete |
| TWITCH-07 | Phase 6 | Complete |
| KICK-01 | Phase 6 | Complete |
| KICK-02 | Phase 6 | Complete |
| KICK-03 | Phase 6 | Complete |
| KICK-04 | Phase 6 | Complete |
| KICK-05 | Phase 6 | Complete |
| TIKTOK-01 | Phase 6 | Complete |
| TIKTOK-02 | Phase 6 | Complete |
| TIKTOK-03 | Phase 6 | Complete |
| TIKTOK-04 | Phase 6 | Complete |
| TIKTOK-05 | Phase 6 | Complete |
| METRICS-01 | Phase 8 | Complete |
| METRICS-02 | Phase 8 | Complete |
| METRICS-03 | Phase 8 | Complete |
| METRICS-04 | Phase 8 | Pending |
| METRICS-05 | Phase 8 | Complete |
| METRICS-06 | Phase 8 | Pending |
| METRICS-07 | Phase 8 | Pending |
| METRICS-08 | Phase 8 | Pending |
| METRICS-09 | Phase 8 | Pending |
| METRICS-10 | Phase 8 | Pending |
| METRICS-11 | Phase 8 | Pending |
| TRACE-01 | Phase 8 | Complete |
| TRACE-02 | Phase 8 | Complete |
| TRACE-03 | Phase 8 | Pending |
| TRACE-04 | Phase 8 | Complete |
| TRACE-05 | Phase 8 | Pending |
| TRACE-06 | Phase 8 | Pending |
