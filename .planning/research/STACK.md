# Stack Research: Listener Load Balancing

**Domain:** Distributed Channel Sharding with Load-Aware Rebalancing
**Researched:** 2026-02-19
**Confidence:** HIGH

## Executive Summary

For implementing hybrid hash-based channel sharding with load-aware rebalancing across Go microservices, the stack additions required are minimal and leverage proven production libraries. The core requirements are: (1) consistent hashing with bounded loads for channel-to-pod assignment, (2) custom Prometheus metrics for load tracking, (3) Redis sorted sets for assignment registry, and (4) graceful WebSocket migration patterns. All required libraries are production-ready, actively maintained, and integrate with the existing All-Chat infrastructure (Redis 7, Kubernetes, Prometheus).

## Recommended Stack

### Core Sharding Library

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/buraksezer/consistent** | v0.10.0+ | Consistent hashing with bounded loads for channel → pod assignment | Industry-proven bounded load consistent hashing used by OpenTelemetry, SeaweedFS, and Celo Blockchain. Implements Google Research algorithm that prevents hotspots by calculating average load per member and ensuring no member exceeds it. Despite last release in Nov 2022, it's stable and production-tested. |

**Confidence:** MEDIUM - Library is stable and production-used but minimally maintained (last release 2022). No breaking changes expected for use case.

**Rationale for Bounded Loads:** Standard consistent hashing (like Jump Hash) only minimizes reassignment on topology changes but doesn't prevent load imbalance. Bounded load consistent hashing ensures no pod exceeds average load by 1 + ε factor, critical for preventing cascading failures when high-traffic channels (e.g., xQc with 50k viewers) would overwhelm a single pod.

### Metrics Collection

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/prometheus/client_golang** | v1.23.2+ | Custom load metrics (messages/sec, channel count, connection health) | Official Prometheus Go client with v1.23.2 released Sep 2025. Provides Gauge (current load), Counter (total messages), and Histogram (latency) metrics. Already integrated with Kubernetes via ServiceMonitor. |

**Confidence:** HIGH - Official library, actively maintained, already in use by All-Chat infrastructure.

**Key Metric Types for Load Balancing:**
- **Gauge**: Current channels per pod (`listener_channels_active`), current msg/sec (`listener_messages_per_second`)
- **Counter**: Total messages processed (`listener_messages_total`), rebalances triggered (`listener_rebalances_total`)
- **Histogram**: Rebalancing duration (`listener_rebalance_duration_seconds`)

### Redis Data Structures

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/redis/go-redis/v9** | v9.18.0+ | Assignment registry using sorted sets and hashes | Already in use by All-Chat (v9.x). Redis sorted sets enable efficient load-based queries (`ZRANGEBYSCORE` for finding underloaded pods), hashes store channel metadata. Published Feb 16, 2026, imported by 14,968 projects. |

**Confidence:** HIGH - Already part of All-Chat stack, official client, recent release.

**Data Structure Choices:**
- **Sorted Set** (`assignments:load`): Pod ID → current load score for O(log N) load-based queries
- **Hash** (`assignment:{channel_id}`): Channel metadata (pod_id, platform, assigned_at) for O(1) lookups
- **Hash** (`pod:{pod_id}:channels`): Pod's channel list for fast rebalancing decisions

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **github.com/gorilla/websocket** | v1.5.3+ | Graceful WebSocket migration during rebalancing | Already in All-Chat for Kick/API Gateway. Use for implementing connection draining: send migration notification, wait for client reconnect, close old connection. Essential for zero-message-loss migration. |
| **context** (stdlib) | Go 1.23+ | Cancellation propagation during graceful shutdown | Standard library. Use for coordinating shutdown across goroutines (IRC read loop, message processor, health check). Pass context from rebalance coordinator to listener instances. |

**Confidence:** HIGH - Both are production-standard, gorilla/websocket already in use.

## Installation

```bash
# New dependency for consistent hashing
go get github.com/buraksezer/consistent@v0.10.0

# Already present in All-Chat (verify versions)
go get github.com/prometheus/client_golang@v1.23.2
go get github.com/redis/go-redis/v9@v9.18.0
go get github.com/gorilla/websocket@v1.5.3
```

## Alternatives Considered

| Category | Recommended | Alternative | When to Use Alternative |
|----------|-------------|-------------|-------------------------|
| Consistent Hashing | buraksezer/consistent (bounded) | lithammer/go-jump-consistent-hash (Jump Hash) | **DO NOT use Jump Hash** - No bounded load support, requires fixed bucket count (incompatible with dynamic pod scaling), cannot prevent hotspots. Only use if pod count is static and all channels have similar load. |
| Consistent Hashing | buraksezer/consistent | lafikl/consistent (bounded) | **DO NOT use** - No official releases, unclear maintenance status (688 stars but "No releases published"). API looks similar but production risk too high. |
| Metrics | Prometheus client_golang | Custom metrics exporter | Only if you need metrics in multiple formats (e.g., StatsD + Prometheus). Adds complexity without benefit since Kubernetes ecosystem standardizes on Prometheus. |
| Assignment Registry | Redis Sorted Sets | PostgreSQL table with indexes | Only if you need ACID transactions across assignments + other tables. Redis sorted sets provide O(log N) load queries vs O(N) table scans. For 100-1000 pods, Redis is 10-100x faster. |
| Assignment Registry | Redis | etcd v3 (watch API) | Consider if you need strong consistency guarantees and already use etcd. Requires additional infrastructure. Redis is sufficient since source-manager already provides leader election via locking. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **Jump Consistent Hash** (dgryski/go-jump, lithammer/go-jump-consistent-hash) | No bounded load support - will create hotspots with high-traffic channels. Fixed bucket count incompatible with Kubernetes HPA. | buraksezer/consistent with bounded loads |
| **Standard consistent hash** (stathat/consistent, serialx/hashring) | Minimizes reassignment but allows unbounded load imbalance. One pod could handle 90% of traffic while others idle. | buraksezer/consistent with bounded loads |
| **Rendezvous hashing** (HRW/highest random weight) | O(N) lookup complexity per key (must compute hash for every node). Impractical for 1000+ channels with frequent lookups. | buraksezer/consistent (O(1) lookup) |
| **Custom HPA metrics via metrics-server** | Kubernetes metrics-server only exposes CPU/memory, requires custom metrics API and Prometheus Adapter. | Direct Prometheus metrics + manual rebalancing logic (simpler, no adapter dependency) |
| **KEDA for autoscaling** | Overkill for this use case - designed for event-driven scale-to-zero (queue depth, cron). All-Chat listeners must stay running (WebSocket connections). | HPA with CPU/memory metrics (already works) |

## Stack Patterns by Scenario

### Scenario 1: Initial Implementation (Phase 1)

**Use:**
- buraksezer/consistent with static config (load factor = 1.25)
- Redis sorted set for pod load tracking
- Prometheus Gauge metrics (messages/sec, channel count)
- Manual rebalancing trigger (admin API endpoint)

**Because:** Simplest path to production. Manual rebalancing reduces blast radius during initial rollout. Load factor 1.25 allows 25% imbalance before rebalancing (proven reasonable by Google Research paper).

### Scenario 2: Production Hardening (Phase 2)

**Use:**
- Same as Phase 1, add automatic rebalancing based on Prometheus metrics
- Graceful WebSocket migration (gorilla/websocket CloseMessage with code 1012 "Service Restart")
- Context-based cancellation for listener shutdown
- Rebalancing backoff (exponential, max 5 minutes)

**Because:** Automatic rebalancing enables true production autonomy but needs safeguards (backoff prevents thrashing, graceful migration prevents message loss).

### Scenario 3: High Scale (1000+ channels)

**Use:**
- Increase partition count in buraksezer/consistent (default 271, increase to 1009 for better distribution)
- Redis cluster mode (if single-node Redis becomes bottleneck)
- Histogram metrics for rebalancing latency percentiles (p50, p95, p99)

**Because:** Higher partition count improves load distribution but increases memory (271 partitions = 271 * member count entries). Only needed at scale. Redis cluster adds complexity but handles 100K+ ops/sec.

## Integration with Existing Infrastructure

### Redis 7 Compatibility

**Data structures required:**
- `ZADD`, `ZINCRBY`, `ZRANGEBYSCORE` (sorted sets) - Available since Redis 1.2
- `HSET`, `HGET`, `HSCAN` (hashes) - Available since Redis 2.0
- `PUBLISH` (Pub/Sub) - Available since Redis 2.0

**Confidence:** HIGH - All operations available in Redis 7, no compatibility issues.

### Kubernetes Integration

**Custom metrics path (OPTIONAL):**
1. Prometheus scrapes `/metrics` endpoint (already configured via ServiceMonitor)
2. Install prometheus-adapter (translates Prometheus metrics → Kubernetes custom metrics API)
3. Configure HPA to use `listener_messages_per_second` for autoscaling

**Simpler path (RECOMMENDED):**
- Use existing HPA with CPU/memory metrics
- Implement rebalancing in source-manager (already has leader election)
- Query Prometheus API directly from Go using `github.com/prometheus/client_golang/api`

**Confidence:** MEDIUM - Prometheus Adapter adds operational complexity. Recommend direct API queries.

### Source-Manager Integration

**Leverage existing capabilities:**
- Leader election (source-manager already uses Redis-based locking)
- Active source registry (extend to include pod assignments)
- Heartbeat mechanism (detect pod failures for reassignment)

**New responsibilities:**
- Run consistent hash ring calculation (leader only)
- Trigger rebalancing when load imbalance > threshold
- Coordinate graceful channel migrations across pods

**Confidence:** HIGH - Minimal changes to source-manager, fits existing responsibilities.

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| buraksezer/consistent@v0.10.0 | Go 1.11+ (uses modules) | No dependencies, pure Go. Thread-safe. Compatible with existing All-Chat Go 1.23+ requirement. |
| prometheus/client_golang@v1.23.2 | Go 1.21+ | Works with existing Prometheus 2.x in All-Chat Kubernetes cluster. |
| redis/go-redis/v9@v9.18.0 | Go 1.18+ (uses generics) | All-Chat already uses Go 1.23+, no issues. Requires Redis 6+ (All-Chat uses Redis 7). |
| gorilla/websocket@v1.5.3 | Go 1.20+ | All-Chat already uses this for Kick listener and API Gateway. |

## Performance Characteristics

### Consistent Hashing Performance

**buraksezer/consistent:**
- Lookup: O(1) average case (hash → partition → member mapping)
- Add/Remove member: O(P) where P = partition count (default 271)
- Memory: O(M * P) where M = member count, P = partition count
  - For 10 pods, 271 partitions: ~2.7KB metadata
  - For 100 pods, 271 partitions: ~27KB metadata

**Comparison with Jump Hash:**
- Jump Hash: O(1) lookup, O(1) add/remove, but NO bounded loads
- Consistent Hash Ring: O(log N) lookup (binary search), O(1) add/remove

**Verdict:** buraksezer/consistent offers best tradeoff - O(1) lookup with bounded loads.

### Redis Performance

**Expected operations:**
- Channel assignment lookup: `HGET assignment:{channel_id} pod_id` - O(1), <1ms
- Find underloaded pod: `ZRANGEBYSCORE assignments:load 0 avg_load LIMIT 0 1` - O(log N + M), <5ms for 100 pods
- Update pod load: `ZINCRBY assignments:load increment pod_id` - O(log N), <1ms

**Scale characteristics:**
- Redis single-node: 100K ops/sec sustained
- All-Chat workload: ~1000 channels * 10 messages/sec = 10K msg/sec, ~100 assignment lookups/sec
- Headroom: 1000x, Redis is not the bottleneck

**Confidence:** HIGH - Redis is massively over-provisioned for this workload.

## Sources

### High Confidence (Official Documentation)
- [buraksezer/consistent GitHub](https://github.com/buraksezer/consistent) - v0.10.0 (Nov 2022), production use by OpenTelemetry, SeaweedFS, Celo
- [prometheus/client_golang pkg.go.dev](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) - v1.23.2 (Sep 2025), official Prometheus Go client
- [redis/go-redis pkg.go.dev](https://pkg.go.dev/github.com/redis/go-redis/v9) - v9.18.0 (Feb 2026), official Redis Go client, 14,968 imports
- [gorilla/websocket GitHub](https://github.com/gorilla/websocket) - v1.5.3+, graceful shutdown patterns
- [Kubernetes HPA Documentation](https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/) - Official K8s autoscaling guide

### Medium Confidence (Community Best Practices)
- [Consistent Hashing Guide by Senthil](https://medium.com/@sent0hil/consistent-hashing-a-guide-go-implementation-fe3421ac3e8f) - Implementation patterns
- [Data Sharding in Golang - Coding Explorations](https://www.codingexplorations.com/blog/data-sharding-in-golang-optimizing-performance-and-scalability) - Best practices for Go sharding
- [Monitor Custom Kubernetes Pod Metrics | Better Stack](https://betterstack.com/community/questions/monitor-custom-kubernetes-pod-metrics-using-prometheus/) - Prometheus custom metrics patterns
- [Redis Task Scheduling | Compile N Run](https://www.compilenrun.com/docs/middleware/redis/redis-development-patterns/redis-task-scheduling/) - Redis sorted sets for task distribution
- [How to Implement Graceful Shutdown in Go | OneUpTime](https://oneuptime.com/blog/post/2026-01-23-go-graceful-shutdown/view) - Jan 2026 graceful shutdown patterns

### Alternative Libraries Evaluated
- [lafikl/consistent GitHub](https://github.com/lafikl/consistent) - No official releases, rejected
- [lithammer/go-jump-consistent-hash](https://pkg.go.dev/github.com/lithammer/go-jump-consistent-hash) - Jump Hash implementation, no bounded loads, rejected
- [dgryski/go-jump GitHub](https://github.com/dgryski/go-jump) - Another Jump Hash, same limitations, rejected

---

**Stack research for:** All-Chat Listener Load Balancing
**Researched:** 2026-02-19
**Next step:** Use findings to create roadmap phases for implementation
