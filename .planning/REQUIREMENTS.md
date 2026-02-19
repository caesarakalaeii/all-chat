# Requirements: All-Chat Listener Load Balancing

**Defined:** 2026-02-19
**Core Value:** Listener instances must efficiently distribute channel workload based on actual message volume, enabling cost-effective scaling and reliable service for both small and high-traffic streams.

## Milestone v1.1 Requirements

### Core Sharding Infrastructure

- [ ] **SHARD-01**: System computes channel-to-pod assignment using consistent hashing with virtual nodes
- [ ] **SHARD-02**: System stores channel assignments in Redis registry with O(1) lookup performance
- [ ] **SHARD-03**: Listener pod queries assignment registry on startup to determine which channels to connect
- [ ] **SHARD-04**: Listener pod publishes heartbeat to Redis every 10 seconds with pod ID and timestamp
- [ ] **SHARD-05**: System detects pod failure when heartbeat missing for 30 seconds
- [ ] **SHARD-06**: System redistributes channels from failed pod to healthy pods within 60 seconds
- [ ] **SHARD-07**: System uses Kubernetes Lease API for coordinator leader election (not Redlock)
- [ ] **SHARD-08**: System uses fencing tokens to prevent split-brain during leader failover

### Rebalancing & Coordination

- [ ] **REBAL-01**: System monitors per-pod message rate (messages/sec) every 30 seconds
- [ ] **REBAL-02**: System calculates load imbalance ratio (max_load / avg_load)
- [ ] **REBAL-03**: System triggers automatic rebalancing when imbalance ratio exceeds 0.5
- [ ] **REBAL-04**: System identifies hot channels (channels with >3x average message rate)
- [ ] **REBAL-05**: System reassigns hot channels from overloaded pods to underutilized pods
- [ ] **REBAL-06**: System enforces 5-minute cooldown between rebalancing operations
- [ ] **REBAL-07**: System limits rebalancing to maximum 20% of channels per operation
- [ ] **REBAL-08**: Coordinator service extends existing source-manager with rebalancing logic

### Channel Migration

- [ ] **MIGRATE-01**: System implements overlap migration pattern (new pod connects before old disconnects)
- [ ] **MIGRATE-02**: New pod subscribes to channel and waits for first message before signaling ready
- [ ] **MIGRATE-03**: Old pod receives migration signal and gracefully disconnects after 45 seconds
- [ ] **MIGRATE-04**: System guarantees zero message loss during migration (no dropped messages)
- [ ] **MIGRATE-05**: System publishes migration events to Redis Streams for observability
- [ ] **MIGRATE-06**: System uses sequence numbers per channel to detect message gaps during migration

### Twitch Load Balancing (CRITICAL)

- [ ] **TWITCH-01**: Twitch listener queries shard coordinator for assigned channels on startup
- [ ] **TWITCH-02**: Twitch listener connects to IRC only for assigned channels (not all channels)
- [ ] **TWITCH-03**: Twitch listener supports multiple IRC connections when assigned >100 channels
- [ ] **TWITCH-04**: Twitch listener stores IRC JOIN list state in ConnectionSnapshot for migration
- [ ] **TWITCH-05**: Twitch listener gracefully parts IRC channels during migration (sends PART command)
- [ ] **TWITCH-06**: System allows HPA to scale Twitch listener from 1 to 5 replicas successfully
- [ ] **TWITCH-07**: All Twitch listener pods report ready status (fixes current 1/5 ready issue)

### Kick Load Balancing

- [ ] **KICK-01**: Kick listener queries shard coordinator for assigned channels on startup
- [ ] **KICK-02**: Kick listener connects to Pusher WebSocket only for assigned channels
- [ ] **KICK-03**: Kick listener stores Pusher subscription IDs in ConnectionSnapshot for migration
- [ ] **KICK-04**: Kick listener gracefully unsubscribes from channels during migration
- [ ] **KICK-05**: System allows HPA to scale Kick listener from 1 to 5 replicas successfully

### TikTok Load Balancing

- [ ] **TIKTOK-01**: TikTok listener queries shard coordinator for assigned channels on startup
- [ ] **TIKTOK-02**: TikTok listener connects via tiktok-live-connector only for assigned channels
- [ ] **TIKTOK-03**: TikTok listener stores connection state in ConnectionSnapshot for migration
- [ ] **TIKTOK-04**: TikTok listener handles connection state migration for unofficial library
- [ ] **TIKTOK-05**: System allows HPA to scale TikTok listener from 1 to 3 replicas successfully

### Observability & Metrics

- [ ] **METRICS-01**: Each listener pod exposes Prometheus metrics at /metrics endpoint
- [ ] **METRICS-02**: System tracks per-pod channel count as Gauge metric (shard_channel_count)
- [ ] **METRICS-03**: System tracks per-pod message rate as Counter metric (shard_messages_total)
- [ ] **METRICS-04**: System tracks rebalancing events as Counter metric (shard_rebalancing_total)
- [ ] **METRICS-05**: System tracks migration success/failure as Counter metrics (shard_migration_success/failure)
- [ ] **METRICS-06**: System tracks load imbalance ratio as Gauge metric (shard_imbalance_ratio)
- [ ] **METRICS-07**: Grafana dashboard visualizes channel distribution across pods (heatmap)
- [ ] **METRICS-08**: Grafana dashboard shows rebalancing timeline and migration events
- [ ] **METRICS-09**: Prometheus alert triggers when imbalance ratio >0.7 for 10 minutes
- [ ] **METRICS-10**: Prometheus alert triggers on split-brain detection (multiple leaders)
- [ ] **METRICS-11**: Prometheus alert triggers on rebalancing thrashing (>3 rebalances in 15min)

### Distributed Tracing

- [ ] **TRACE-01**: System instruments channel assignment operations with OpenTelemetry spans
- [ ] **TRACE-02**: System instruments migration operations with OpenTelemetry spans
- [ ] **TRACE-03**: System instruments rebalancing operations with OpenTelemetry spans
- [ ] **TRACE-04**: System propagates trace context through Redis Streams messages
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
| (To be populated by roadmapper) | | |

**Coverage:**
- v1.1 requirements: 50 total
- Mapped to phases: (to be counted)
- Unmapped: (to be counted)

---
*Requirements defined: 2026-02-19*
*Last updated: 2026-02-19 after initial definition*
