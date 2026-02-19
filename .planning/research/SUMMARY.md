# Project Research Summary

**Project:** All-Chat Listener Load Balancing
**Domain:** Distributed Channel Sharding for Real-Time Messaging Microservices
**Researched:** 2026-02-19
**Confidence:** HIGH

## Executive Summary

Implementing distributed channel sharding for All-Chat listener services requires a hybrid consistent hashing architecture with load-aware rebalancing. Research reveals that modern production systems (Kafka, Elasticsearch, Kubernetes) have evolved from static hash-based distribution to volume-aware dynamic rebalancing with cooperative migration protocols. The recommended approach combines bounded-load consistent hashing for initial assignment with Redis-based coordination and graceful migration patterns to prevent message loss during rebalancing.

The critical insight is that naive consistent hashing solves distribution but not the "celebrity problem" - a single high-traffic channel (xQc with 50k viewers) can overwhelm a pod. This requires bounded-load consistent hashing that limits any pod to 1.25x average load. The technology stack is minimal and proven: buraksezer/consistent for hashing, Redis sorted sets for assignment tracking, Prometheus for load metrics, and cooperative rebalancing inspired by Kafka's protocol. All libraries are production-tested and integrate seamlessly with All-Chat's existing infrastructure.

Key risks center on three failure modes: split-brain during channel assignment (causing duplicate connections), message loss during pod migration (requiring overlap migration), and thundering herd on HPA scale-up (exhausting YouTube quota). All three are preventable through proper leader election with fencing tokens, cooperative migration protocols, and staggered pod startup with rate limiting. The architecture must be designed correctly from Phase 1 - retrofitting split-brain prevention or overlap migration after production launch is prohibitively expensive.

## Key Findings

### Recommended Stack

The stack additions are minimal and leverage proven production libraries. Core requirement is bounded-load consistent hashing to prevent hot spots, custom Prometheus metrics for load tracking, Redis sorted sets for assignment registry, and graceful migration patterns. All libraries are actively maintained and integrate with existing All-Chat infrastructure (Redis 7, Kubernetes, Prometheus).

**Core technologies:**
- **buraksezer/consistent v0.10.0+**: Bounded-load consistent hashing for channel-to-pod assignment. Implements Google Research algorithm preventing any pod from exceeding 1.25x average load. Production-used by OpenTelemetry and SeaweedFS despite last release in 2022. No breaking changes expected.
- **prometheus/client_golang v1.23.2+**: Custom load metrics (messages/sec, channel count, connection health). Official Prometheus client already integrated with All-Chat Kubernetes via ServiceMonitor. Provides Gauge, Counter, and Histogram metric types.
- **redis/go-redis/v9 v9.18.0+**: Assignment registry using sorted sets (O(log N) load queries) and hashes (O(1) channel lookups). Already in use by All-Chat, recent release Feb 2026, imported by 14,968 projects.
- **gorilla/websocket v1.5.3+**: Graceful WebSocket migration during rebalancing. Already used by Kick listener and API Gateway. Essential for zero-message-loss migration through connection draining.

**Confidence:** HIGH - All technologies are production-standard, most already in All-Chat stack. Only new dependency is buraksezer/consistent (MEDIUM confidence due to minimal maintenance, but stable).

**Critical anti-patterns to avoid:**
- Jump Hash (no bounded load support, creates hot spots)
- Redlock (Martin Kleppmann's critique - use Kubernetes Lease API instead)
- Standard consistent hashing without virtual nodes (allows unbounded load imbalance)

### Expected Features

Production-grade distributed channel sharding systems balance three concerns: fair distribution (preventing hot partitions), operational resilience (handling failures gracefully), and observability (understanding system behavior). Research reveals a clear hierarchy: basic shard assignment is table stakes, dynamic rebalancing based on actual load is the differentiator, and complex predictive algorithms are over-engineering.

**Must have (table stakes):**
- Consistent hashing with virtual nodes (150-200 per pod) - prevents massive reshuffling on topology changes, only affected channels move
- Quorum-based leader election - prevents split-brain where multiple pods claim same channels, requires >50% for leadership
- Graceful shutdown with connection draining - prevents message loss when pods terminate, 45-60s grace period standard
- Separate readiness vs liveness probes - readiness = "ready for channels", liveness = "still alive", failed readiness removes from rotation
- Heartbeat-based membership tracking - 10s heartbeat interval with 30s timeout, detects failed pods and triggers rebalancing
- Idempotent channel assignment - re-assigning channel to same pod must be safe, prevents duplicate connections during rebalancing

**Should have (competitive differentiators):**
- Volume-based rebalancing - automatically detect "hot" channels (5000 msgs/min) and redistribute load, prevents single pod bottleneck
- Cooperative rebalancing - only move channels that need to move, non-affected channels keep processing, reduces disruption by 80% vs stop-the-world
- Weighted pod capacity - assign more channels to pods with more resources, useful for heterogeneous pod sizing
- Rebalancing backoff with exponential delay - prevents rebalancing storms during outages (1s, 2s, 4s, 8s delays)
- Metrics per shard - track messages/sec, CPU%, memory per assigned channel set, enables debugging "why is pod-3 overloaded?"

**Defer (v2+):**
- Predictive load balancing with ML - adds complexity without benefit, 99% of systems don't need this, reactive rebalancing is sufficient
- Automatic hot partition splitting - split single channel across multiple pods via sub-sharding, only needed if single channel exceeds pod capacity (rare)

**Anti-features (explicitly not recommended):**
- Perfect load balance enforcement - causes constant rebalancing thrash, accept imbalance <0.3 as healthy
- Manual shard assignment UI - defeats automation purpose, creates human bottleneck
- Synchronous rebalancing - single slow pod blocks entire rebalancing, use async with timeouts instead

### Architecture Approach

The architecture adds a load balancing layer above existing listeners without changing the core message flow (Listener → Redis Streams → Message Processor → Redis Pub/Sub → Gateway). A new Shard Coordinator service implements consistent hashing and orchestrates rebalancing, storing assignments in Redis data structures (sorted sets for load queries, hashes for channel lookups). Listeners are enhanced to read assignments from Redis, report load metrics via heartbeat, and handle migration events through cooperative protocol.

**Major components:**

1. **Shard Coordinator (NEW SERVICE)** - Runs consistent hashing ring calculation, computes channel-to-pod assignments, orchestrates rebalancing when topology changes or load imbalances detected, triggers migrations via Redis Streams. Extends existing source-manager which already has leader election via Redis locks.

2. **Assignment Registry (NEW REDIS DATA STRUCTURES)** - Sorted set `assignments:load` for O(log N) load-based queries (find underloaded pods), hash `assignment:{channel_id}` for O(1) channel metadata lookups, hash `pod:{pod_id}:channels` for fast rebalancing decisions, stream `migrations:{platform}` for migration events with consumer groups.

3. **Load Monitor (NEW COMPONENT IN EACH POD)** - Tracks per-pod metrics (channel count, message rate per 10s, memory usage), publishes to Redis every 10s with 30s TTL (pod considered dead if not refreshed), enables coordinator to detect failures and trigger rebalancing.

4. **Enhanced Listener Pods (MODIFY EXISTING)** - Startup sequence: register with coordinator → query assignment registry → join assigned channels only → start load monitor. Migration handling: consume migration events from Redis Stream via XREADGROUP, gracefully PART channels when migration event received, ack completion.

**Key architectural patterns:**
- **Consistent hashing with virtual nodes**: 150 virtual nodes per pod for even distribution, O(1) lookup complexity, only K/N channels reassigned on topology changes
- **Redis-based assignment registry**: Client-side sharding pattern, coordinator writes assignments, listeners poll every 30s, eventual consistency acceptable (30s delay for new channels)
- **Cooperative rebalancing**: Kafka-style protocol, only migrate channels that must move, publish migration events to Redis Stream, source pods consume and gracefully disconnect, destination pods then connect
- **Heartbeat-based failure detection**: Pods self-report health every 10s with 30s TTL, coordinator scans for missing heartbeats, triggers rebalancing on pod failure

**Integration with existing architecture:**
- Message flow unchanged: Listener → Redis Streams → Message Processor → Redis Pub/Sub → Gateway
- Source Manager extended: publishes source changes to Shard Coordinator via webhook (new overlay added/removed)
- YouTube listener unchanged: already has leader election via source-manager, sharding not applicable due to quota bottleneck
- Database unchanged: overlay_chat_sources remains single source of truth, source-manager polls and notifies coordinator

### Critical Pitfalls

Research identified seven critical pitfalls with HIGH severity, all preventable through proper design in Phase 1-2. Recovery costs range from MEDIUM (overlap migration) to HIGH (rewrite for Redlock) to IMPOSSIBLE (YouTube quota exhaustion = 24h lockout).

1. **Split-brain during channel assignment** - Network partitions cause multiple pods to believe they own same channel, resulting in duplicate connections and duplicate message delivery. Avoid with: Kubernetes Lease API with proper TTLs, fencing tokens (monotonic counters), majority quorum for ownership decisions, platform-level duplicate detection. Warning signs: duplicate messages in overlays, sum(channels_per_pod) > total_active_channels. CRITICAL severity - causes duplicate API quota charges and rate limit bans.

2. **Message loss during channel migration** - Old pod disconnects before new pod establishes connection, creating 1-10 second gap where messages aren't captured. IRC connections take 2-5s to establish (TCP → TLS → IRC JOIN). Avoid with: overlap migration (new pod connects BEFORE old pod disconnects), Redis Streams offset tracking, replay buffer in Message Processor (60s retention), dual-connection during migration for critical channels. Warning signs: users report missed messages during deployments, gaps in consumer group offsets. CRITICAL severity - violates product promise.

3. **Thundering herd on HPA scale-up** - When HPA adds 5 new pods simultaneously, all attempt rebalancing at once, causing Redis lock contention, platform rate limits (YouTube quota exhausted), cascading failures, election storms. Avoid with: startup jitter (sleep random 0 to pod_ordinal * 2), gradual rebalancing in batches (10 channels/sec), token bucket rate limiter via Redis, HPA scaleUp stabilization (60s window, max 2 pods/min). Warning signs: redis_lock_timeout errors during scale-up, platform 429 errors correlated with HPA events. CRITICAL severity - YouTube quota exhaustion affects all users for 24 hours.

4. **Inconsistent hashing key selection** - Poor key choice causes hot spots (all popular channels hash to same pod), migration churn (different components use different keys), state loss. Using overlayID instead of channelID causes channel to bounce between pods. Avoid with: always hash on channelID (or platform:channelID), 150-200 virtual nodes per pod, hot key detection using Count-Min Sketch, hash key validation in tests. Warning signs: highly variable load across pods, frequent migrations for same channel. MAJOR severity - degrades performance and causes cascading issues.

5. **Platform-specific connection state not migrated** - Different platforms have different stateful requirements: IRC channels must be explicitly JOINed, YouTube polling offset (pageToken) resets causing duplicates/missed messages, Kick subscription IDs must match original connection. Avoid with: platform-specific ConnectionSnapshot interface (Capture/Restore), store snapshots in Redis with TTL, per-platform migration handlers, integration tests for mid-stream migration. Warning signs: message rate drops to zero after migration, duplicate messages appear, platform errors "Not in channel". CRITICAL severity - migration equals downtime without this.

6. **Message ordering violations during rebalancing** - Channel temporarily has two producers (old pod + new pod), Redis Streams only provides ordering within single producer, messages arrive out-of-order at overlays. Avoid with: sequence numbers per channel, Message Processor validates sequence and buffers out-of-order messages, old pod stops publishing BEFORE new pod starts (brief gap acceptable), migration coordinator pattern for zero-gap. Warning signs: users report wrong message order during deployments, "sequence gap detected" in logs. MAJOR severity - confusing but not catastrophic.

7. **Redlock anti-pattern for channel ownership** - Team uses Redlock believing it's safer than single-instance locks, actually adds complexity (3+ Redis instances), doesn't solve fundamental problems (clock skew causes false acquisitions), no fencing tokens (stale pod can act on expired lock), fails during network partitions. Avoid with: Kubernetes Lease API with fencing tokens, or single Redis with proper patterns (SET NX + PX, unique token, Lua delete script), monotonic counter for fencing. Warning signs: multiple Redis instances for locking, no fencing tokens, locks acquired by multiple pods. MAJOR severity - causes subtle split-brain under network partitions.

**All-Chat specific warnings:**
- **YouTube quota is non-negotiable**: Once exhausted, ALL users affected for 24h. Thundering herd during scale-up can exhaust quota in minutes. Use centralized quota tracking in Redis (existing quota-manager service), circuit breaker at 90% quota, pause YouTube connections during rebalancing.
- **IRC connection limits (Twitch)**: ~50 channels per connection, limits total connections per IP. Each pod maintains 1 IRC connection and JOINs multiple channels. During migration, new pod JOINs BEFORE old pod PARTs (brief overlap acceptable, not duplicate connection).
- **Emote service load**: More pods = more emote API calls (7TV, BTTV, FFZ have rate limits). Use centralized emote cache in Redis (shared across pods), 5 min TTL, only leader pod refreshes cache.

## Implications for Roadmap

Based on research, recommended phase structure follows dependency graph: foundation (consistent hashing + Redis data structures) → coordinator service → listener integration → production hardening. Critical features must be implemented in Phase 1 (split-brain prevention, leader election) and Phase 2 (overlap migration, platform-specific state transfer) - these cannot be retrofitted after production launch.

### Phase 1: Sharding Infrastructure & Coordinator Service

**Rationale:** Foundational components have no dependencies and must be correct from start. Split-brain prevention and consistent hashing key selection are expensive to fix later (require rewrites). Leader election with fencing tokens is critical for preventing duplicate connections. This phase delivers a working coordinator that can compute assignments, but listeners don't consume them yet.

**Delivers:**
- Consistent hashing library (buraksezer/consistent with bounded loads)
- Shard Coordinator service (HTTP server, assignment computation, health checks)
- Redis data structures (assignments hash, load tracking, active pods set, metrics with TTL)
- Leader election via Kubernetes Lease API with fencing tokens
- Assignment computation logic (consistent hash → Redis writes)
- Basic observability (Prometheus metrics for assignment counts)

**Addresses features:**
- Consistent hashing with virtual nodes (table stakes)
- Quorum-based leader election (table stakes)
- Heartbeat-based membership tracking (table stakes)

**Avoids pitfalls:**
- Split-brain during channel assignment (implement fencing tokens, majority quorum)
- Inconsistent hashing key selection (use channelID, 150 virtual nodes, document in ADR)
- Redlock anti-pattern (use Kubernetes Lease API, not Redlock)

**Stack elements:**
- buraksezer/consistent v0.10.0 (new dependency)
- prometheus/client_golang v1.23.2 (already in use)
- redis/go-redis/v9 v9.18.0 (already in use)

**Implementation notes:**
- Coordinator is stateless (reads state from Redis)
- Leader election scope: one leader computes assignments for all platforms
- Hash key decision: platform:channelID for global uniqueness
- Virtual nodes: 150 per pod (balance between distribution quality and memory)

**Exit criteria:**
- Coordinator can compute assignments for 1000 channels across 10 pods
- Leader election recovers in <5s on pod failure
- Chaos test: network partition between pods, verify only one pod computes assignments
- Metrics show: sum(channels_per_pod) == total_active_channels (no duplicates)

### Phase 2: Connection Management & Migration

**Rationale:** Phase 1 computes assignments but listeners don't consume them. Phase 2 integrates listeners to read assignments and implements cooperative migration protocol. This is where message loss prevention happens - overlap migration must be built before production. Platform-specific state migration (YouTube pageToken, Kick subscription IDs) prevents silent failures.

**Delivers:**
- Enhanced listener startup (register with coordinator → query assignments → join assigned channels)
- Load monitor component (publish metrics every 10s with 30s TTL)
- Migration handler (XREADGROUP consume migration events, graceful PART/disconnect)
- Overlap migration protocol (new pod connects BEFORE old pod disconnects)
- Platform-specific ConnectionSnapshot interface (Twitch JOIN list, YouTube pageToken, Kick subscription IDs)
- Graceful shutdown (45-60s grace period, complete in-flight messages)
- Separate readiness vs liveness probes

**Addresses features:**
- Graceful shutdown with connection draining (table stakes)
- Separate readiness/liveness probes (table stakes)
- Idempotent channel assignment (table stakes)
- Cooperative rebalancing (differentiator)

**Avoids pitfalls:**
- Message loss during channel migration (implement overlap migration, replay buffer)
- Platform-specific connection state not migrated (ConnectionSnapshot interface per platform)
- Message ordering violations (sequence numbers per channel, validate in Message Processor)

**Uses stack:**
- gorilla/websocket v1.5.3+ (graceful WebSocket migration for Kick)
- context package (cancellation propagation during graceful shutdown)

**Implementation notes:**
- Migration event stream: `shard:migrations:{platform}` with consumer groups
- Overlap window: new pod waits for first message BEFORE old pod disconnects (verify receiving)
- Platform handlers:
  - Twitch: capture active JOIN list, replay on new connection
  - YouTube: store last pageToken + timestamp, resume from that point
  - Kick: store subscription IDs, re-subscribe with same IDs
  - TikTok: similar to Kick (WebRTC-based)
- Graceful shutdown: 10s preStop + 20s drain + 5s cleanup + 10s buffer = 45s total

**Exit criteria:**
- E2E test: migrate channel mid-stream, verify zero message loss
- E2E test: verify message ordering preserved during migration
- Load test: 1000 channels across 10 pods, migrate 100 channels, <1s gap per migration
- Manual test: send SIGTERM to pod, verify continues processing for 20s, clean handoff

### Phase 3: Scaling & Resilience

**Rationale:** Phase 1-2 handle steady-state operation and single pod failures. Phase 3 adds safeguards for production scenarios: HPA scale-up, thundering herd prevention, automatic rebalancing triggers, volume-based rebalancing for hot channels. This phase tests under realistic load and chaos scenarios.

**Delivers:**
- Automatic rebalancing triggers (pod added/removed, heartbeat timeout, load imbalance >0.5)
- Staggered pod startup (jitter: sleep random 0 to pod_ordinal * 2)
- Gradual rebalancing (assign channels in batches of 10/second, not all-at-once)
- Rebalancing backoff (exponential: 1s, 2s, 4s, 8s, max 5 min)
- Volume-based rebalancing (detect channels with >500 msgs/sec, redistribute load)
- Token bucket rate limiter for platform APIs (shared across pods via Redis)
- HPA configuration (stabilizationWindowSeconds: 60, max 2 pods/min)
- All-Chat specific: YouTube quota circuit breaker (throttle at 90%, pause during rebalancing)

**Addresses features:**
- Volume-based rebalancing (differentiator)
- Rebalancing backoff (differentiator)
- Weighted pod capacity (differentiator, if needed)

**Avoids pitfalls:**
- Thundering herd on HPA scale-up (startup jitter, gradual rebalancing, rate limiter, HPA limits)
- YouTube quota exhaustion (centralized tracking, circuit breaker, pause during rebalancing)

**Implementation notes:**
- Rebalancing threshold: 0.5 imbalance ratio (any pod has >150% of average load)
- Rebalancing cooldown: 5 minutes between rebalances
- Volume threshold: 500 msgs/sec per channel triggers rebalancing
- YouTube circuit breaker: at 90% quota, throttle polling rate globally, prioritize high-traffic channels
- Emote cache: centralized in Redis with 5 min TTL, only leader refreshes

**Exit criteria:**
- Chaos test: HPA scale-up 2→10 pods, verify staggered startup, no lock contention
- Load test: trigger volume-based rebalancing, verify hot channel redistributed in <60s
- Quota test: approach YouTube 90% threshold, verify circuit breaker activates
- Soak test: 24+ hours with 1000 channels, verify no memory leaks, stable load distribution

### Phase 4: Observability & Production Readiness

**Rationale:** Phase 1-3 deliver functional sharding. Phase 4 ensures debuggability in production through comprehensive metrics, distributed tracing, and operational dashboards. Without observability, production issues are impossible to diagnose (blind to hot spots, migration failures, lock contention).

**Delivers:**
- Comprehensive Prometheus metrics:
  - Per-pod channel count distribution (detect hot spots)
  - Rebalancing frequency, duration, success rate
  - Channel migration events per platform
  - Connection establishment time per platform
  - Message gap duration during migrations
  - Redis lock contention and wait time
  - Leader election changes
- Distributed tracing via OpenTelemetry (platform → listener → Redis → processor → gateway → overlay)
- Grafana dashboards:
  - Load distribution fairness (histogram of channels per pod)
  - Rebalancing activity (migration events, duration)
  - Health metrics (heartbeat failures, pod restarts)
  - Performance (messages/sec per pod, CPU/memory)
  - Failure modes (split-brain events, duplicate connections)
- Alerting rules:
  - Alert: max(channels_per_pod) > 2 * avg(channels_per_pod)
  - Alert: migration_failure_rate > 5%
  - Alert: p95(connection_time) > 10s
  - Alert: leader_changes > 2 per hour
  - Alert: sequence_gap_events > 0
- Integration tests for all failure scenarios
- Documentation (runbooks, troubleshooting guides)

**Addresses features:**
- Metrics per shard (differentiator)

**Avoids pitfalls:**
- Observability gaps (comprehensive metrics prevent debugging blindness)

**Implementation notes:**
- Metric cardinality: avoid per-channel labels (use per-pod aggregates)
- Tracing sampling: 1% of messages (high-traffic overlays would overwhelm collector)
- Dashboard refresh: 30s for load distribution, 5s for real-time migration events
- Log retention: 7 days for debug logs, 30 days for error logs

**Exit criteria:**
- Can identify root cause of simulated failure scenarios from metrics alone in <5 min
- Distributed trace shows end-to-end message latency from platform to overlay
- All alerting rules tested with simulated failures
- Runbooks cover: split-brain recovery, migration failure recovery, quota exhaustion

### Phase Ordering Rationale

- **Phase 1 first**: Foundation must be correct (split-brain prevention, hash key selection). These are expensive to fix later.
- **Phase 2 before Phase 3**: Must prevent message loss before scaling. Overlap migration is non-negotiable for production.
- **Phase 3 before Phase 4**: Scaling scenarios inform observability requirements. Test under load to discover what metrics are critical.
- **Phase 4 throughout**: Basic observability starts in Phase 1, comprehensive in Phase 4. Tracing and alerting are final polish.

**Dependency chains:**
- Overlap migration (Phase 2) requires cooperative rebalancing (Phase 2) requires migration events (Phase 1)
- Volume-based rebalancing (Phase 3) requires metrics per shard (Phase 4) requires load monitor (Phase 2)
- All phases require consistent hashing (Phase 1) and leader election (Phase 1)

**Critical path:** Phase 1 (Coordinator) blocks everything. Phase 2 (Connection Management) blocks production launch. Phase 3 (Scaling) blocks autoscaling. Phase 4 (Observability) is polish but critical for operations.

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 2: Platform-specific migration**: Each platform (Twitch IRC, YouTube polling, Kick WebSocket, TikTok WebRTC) has different connection state requirements. May need platform-specific research for:
  - YouTube: pageToken continuation API, behavior during polling gaps
  - Kick: Pusher subscription ID lifecycle, can IDs be reused?
  - TikTok: Unofficial library stability, connection migration patterns

- **Phase 3: YouTube quota optimization**: Circuit breaker implementation needs deeper research on:
  - Google quota API (can we query remaining quota programmatically?)
  - Polling rate throttling strategies (which channels to prioritize?)
  - Quota increase request process (how long does approval take?)

Phases with standard patterns (skip research-phase):

- **Phase 1: Consistent hashing**: Well-documented pattern, buraksezer/consistent library handles complexity
- **Phase 2: Graceful shutdown**: Kubernetes termination lifecycle is standard, 45-60s grace period is industry norm
- **Phase 3: HPA configuration**: Standard Kubernetes pattern, well-documented stabilization and rate limiting
- **Phase 4: Observability**: Prometheus + Grafana + OpenTelemetry are established stack, reference dashboards exist

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technologies are production-proven, most already in All-Chat. Only new dependency (buraksezer/consistent) is stable despite minimal maintenance. No breaking changes expected. |
| Features | MEDIUM-HIGH | Table stakes features have HIGH confidence (well-established patterns). Differentiators have MEDIUM confidence (observed in production systems like Kafka/Elasticsearch but implementation complexity varies). MVP recommendations have HIGH confidence. |
| Architecture | HIGH | Patterns are verified with official docs, multiple credible sources, and industry standards (Kafka, Elasticsearch, Kubernetes). Redis-based coordination matches existing source-manager pattern. Integration points are well-defined. |
| Pitfalls | MEDIUM-HIGH | Critical pitfalls have HIGH confidence (backed by production post-mortems and Martin Kleppmann's distributed systems research). Some pitfalls inferred from general distributed systems principles (MEDIUM confidence). Recovery strategies based on industry practices. |

**Overall confidence:** HIGH

Research is comprehensive with extensive sourcing (40+ references across Stack, Features, Architecture, Pitfalls). All recommended technologies are production-tested and actively used by major systems (Kafka, Elasticsearch, OpenTelemetry, Kubernetes). Architecture patterns are industry-standard with clear documentation. Pitfalls are backed by production case studies and expert analysis (Martin Kleppmann on Redlock, Kafka's evolution to cooperative rebalancing).

### Gaps to Address

Areas where research was inconclusive or needs validation during implementation:

- **Platform API behavior during migration**: Research focused on general patterns. Need platform-specific validation for:
  - YouTube: Does pageToken API tolerate gaps? What happens if polling stops for 60s?
  - Kick: Can Pusher subscription IDs be reused across connections? What's the subscription ID lifecycle?
  - TikTok: Unofficial library stability - can connections be migrated at all? May need connection pool instead.
  - **Recommendation**: Implement ConnectionSnapshot interface in Phase 2, add integration tests per platform to validate migration behavior.

- **Bounded-load consistent hashing performance at scale**: buraksezer/consistent is production-used but last release was 2022. Research shows O(1) lookup with O(P) add/remove (P = partition count), but no direct benchmarks for 1000+ channels.
  - **Recommendation**: Add benchmarks in Phase 1 to validate performance at target scale (1000 channels, 10 pods). If performance degrades, consider alternatives (stathat/consistent with virtual nodes).

- **Redis single-instance bottleneck**: Research shows Redis single-node handles 100K ops/sec, All-Chat workload is ~100 ops/sec. Massive headroom, but no direct testing with sorted set queries under load.
  - **Recommendation**: Load test in Phase 3 with 1000 channels, 10 pods, 10 rebalances/min. Verify Redis CPU stays <20%. If bottleneck appears, consider Redis Cluster.

- **YouTube quota API programmatic queries**: Circuit breaker design assumes we can query remaining quota. Google Quota API documentation doesn't clearly state if this is possible.
  - **Recommendation**: Research YouTube Quota API in Phase 3 planning. If not queryable, use estimation (track requests, calculate remaining based on 24h reset).

- **Message ordering guarantees during overlap migration**: Research shows Redis Streams provides ordering within producer, but overlap migration temporarily creates two producers. Sequence numbers solve this, but requires Message Processor changes.
  - **Recommendation**: Design sequence number system in Phase 2 planning. Add to message format early (difficult to add later). Alternative: old pod routes messages through new pod during overlap (complex).

## Sources

### Primary (HIGH confidence)

**Stack research:**
- [buraksezer/consistent GitHub](https://github.com/buraksezer/consistent) - v0.10.0, production use by OpenTelemetry and SeaweedFS
- [prometheus/client_golang pkg.go.dev](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) - v1.23.2 official client
- [redis/go-redis pkg.go.dev](https://pkg.go.dev/github.com/redis/go-redis/v9) - v9.18.0 official client, 14,968 imports
- [gorilla/websocket GitHub](https://github.com/gorilla/websocket) - v1.5.3+ graceful shutdown patterns
- [Kubernetes HPA Documentation](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/)

**Architecture research:**
- [Consistent Hashing Explained (Ably)](https://ably.com/blog/implementing-efficient-consistent-hashing)
- [Ultimate Guide to Consistent Hashing (Toptal)](https://www.toptal.com/big-data/consistent-hashing)
- [Redis Ring for Consistent Hashing](https://redis.uptrace.dev/guide/ring.html)
- [Kafka Partition Rebalancing](https://oneuptime.com/blog/post/2026-02-02-kafka-partition-rebalancing/view)
- [StatefulSet Management (Kubernetes)](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)

**Features research:**
- [Kafka Cooperative Rebalancing (Confluent)](https://www.confluent.io/blog/cooperative-rebalancing-in-kafka-streams-consumer-ksqldb/)
- [KIP-848 Consumer Rebalance Protocol](https://www.confluent.io/blog/kip-848-consumer-rebalance-protocol/)
- [Google Cloud: Terminating with Grace](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace)
- [Google Cloud: Readiness and Liveness Probes](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-setting-up-health-checks-with-readiness-and-liveness-probes)

**Pitfalls research:**
- [Martin Kleppmann: How to do distributed locking](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html) - Redlock critique
- [Split-brain in Distributed Systems (DZone)](https://dzone.com/articles/split-brain-in-distributed-systems)
- [Leader Election in Distributed Systems 2026](https://www.devahmedali.click/post/leader-election-in-distributed-systems-complete-guide)
- [Migrate Stateful Workloads with Zero Downtime (Cast AI)](https://cast.ai/blog/how-to-migrate-stateful-workloads-on-kubernetes-with-zero-downtime/)

### Secondary (MEDIUM confidence)

- [Consistent Hashing Guide by Senthil (Medium)](https://medium.com/@sent0hil/consistent-hashing-a-guide-go-implementation-fe3421ac3e8f)
- [Data Sharding in Golang - Coding Explorations](https://www.codingexplorations.com/blog/data-sharding-in-golang-optimizing-performance-and-scalability)
- [Hot Partition Balancing (Medium)](https://medium.com/ai-ml-interview-playbook/re-sharding-and-hot-partition-balancing-keeping-your-distributed-systems-healthy-at-scale-8eda61aaf9b2)
- [Vimeo: Improving Load Balancing with Bounded Consistent Hashing](https://medium.com/vimeo-engineering-blog/improving-load-balancing-with-a-new-consistent-hashing-algorithm-9f1bd75709ed)
- [Thundering Herd Problem (Encore)](https://encore.dev/blog/thundering-herd-problem)
- [Message Ordering in Event-Driven Systems (OneUpTime)](https://oneuptime.com/blog/post/2026-01-24-message-ordering-event-driven/view)

### Tertiary (LOW confidence, needs validation)

- Community reports on Kick Pusher channel limits (varies by plan, not officially documented)
- TikTok unofficial library stability (anecdotal reports suggest 10-20 concurrent streams)
- Elasticsearch shard allocation patterns (referenced but not deeply researched for All-Chat use case)

---

**Research completed:** 2026-02-19
**Ready for roadmap:** YES

**Recommended next steps:**
1. Review SUMMARY.md with stakeholders to validate phase structure
2. Create roadmap in `.planning/roadmap/ROADMAP.md` based on phase suggestions
3. Begin Phase 1 planning with focus on leader election and consistent hashing implementation
4. Schedule architecture review before Phase 2 to validate migration protocol design
5. Plan chaos engineering tests for Phase 3 (network partitions, HPA scale-up, quota exhaustion)
