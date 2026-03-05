---
phase: 12-production-rollout
plan: 01
subsystem: monitoring
tags: [prometheus, metrics, canary, observability]
requires: [innertube-client, redis-publisher, poller]
provides: [prometheus-metrics, canary-monitoring]
affects: [api-gateway-innertube, rollout-analysis]
tech-stack:
  added:
    - prometheus/client_golang v1.23.2
  patterns:
    - Prometheus push metrics pattern
    - Service label-based canary comparison
    - Error rate tracking by type
key-files:
  created:
    - services/youtube-listener-innertube/metrics/innertube_metrics.go
  modified:
    - services/youtube-listener-innertube/publisher/redis_publisher.go
    - services/youtube-listener-innertube/innertube/client.go
    - services/youtube-listener-innertube/poller/poller.go
    - services/youtube-listener-innertube/streams/manager.go
    - services/youtube-listener-innertube/cmd/main.go
decisions:
  - "Use exact metric names from official youtube-listener for PromQL compatibility"
  - "Hardcode service label 'youtube-listener-innertube-canary' to match Kubernetes Service name"
  - "Track errors by type (http, parse, rate_limit, redis) for granular analysis"
  - "Track reconnections with reason labels for stability monitoring"
  - "Use nil-safe metrics handling for backward compatibility"
metrics:
  duration: 13.5
  tasks_completed: 3
  tasks_total: 3
  files_created: 1
  files_modified: 5
  commits: 3
completed: 2026-03-05T11:41:00Z
---

# Phase 12 Plan 01: InnerTube Prometheus Metrics Summary

**One-liner:** Instrumented InnerTube YouTube Listener with Prometheus metrics for canary deployment monitoring and automatic rollback decisions via Argo Rollouts AnalysisTemplate.

## What Was Built

Created comprehensive Prometheus metrics instrumentation for InnerTube YouTube Listener to enable metrics-based canary deployment with automatic promotion/rollback via Argo Rollouts.

### Metrics Package (metrics/innertube_metrics.go)

**Created InnerTubeMetrics struct with 7 metric families:**

1. **Error Tracking (for error rate threshold analysis)**
   - `youtube_listener_errors_total` (counter) - labels: service, error_type
   - `youtube_listener_requests_total` (counter) - labels: service
   - Error types: http, parse, rate_limit, redis

2. **Message Publishing (for message rate comparison)**
   - `youtube_listener_messages_published_total` (counter) - labels: service, channel_id

3. **Redis Health (for downstream failure detection)**
   - `youtube_listener_redis_publish_attempts_total` (counter) - labels: service
   - `youtube_listener_redis_publish_success_total` (counter) - labels: service
   - `youtube_listener_redis_publish_latency_seconds` (histogram) - labels: service

4. **Reconnection Tracking (for stability monitoring)**
   - `youtube_listener_reconnections_total` (counter) - labels: service, channel_id, reason
   - Reconnection reasons: error, offline, backoff, rediscovery

**Key Design Decisions:**
- Metric names match official youtube-listener EXACTLY (enables shared PromQL queries)
- Service label constant: `youtube-listener-innertube-canary`
- Exported constants for error types and reconnection reasons (type safety)

### Component Instrumentation

**1. Redis Publisher (publisher/redis_publisher.go)**
- Added `metrics *metrics.InnerTubeMetrics` field to StreamPublisher
- Track publish attempts before XADD operation
- Track successful publishes and message counts by channel
- Track Redis errors separately from HTTP errors
- Measure publish latency with histogram (DefBuckets: 0.005s to 10s)
- Updated constructor: `NewStreamPublisher(client, logger, metrics)`

**2. InnerTube Client (innertube/client.go)**
- Added `metrics *metrics.InnerTubeMetrics` field to Client
- Track all API requests (successful + failed)
- Track HTTP errors (4xx, 5xx) with separate counter
- Track rate limit errors (429) specifically
- Track parse errors (JSON unmarshal failures)
- Updated ClientOptions to include Metrics field

**3. Poller (poller/poller.go)**
- Added `metrics *metrics.InnerTubeMetrics` field to Poller
- Track reconnection attempts during transient error backoff
- Updated PollerOptions to include Metrics field
- Track reason labels: error, offline, backoff, rediscovery

**4. Stream Manager (streams/manager.go)**
- Added `metrics *metrics.InnerTubeMetrics` field to Manager
- Pass metrics to all created pollers
- Updated constructor: `NewManager(..., metrics)`

**5. Main Service (cmd/main.go)**
- Initialize InnerTubeMetrics before all components
- Pass metrics to InnerTube client (via ClientOptions)
- Pass metrics to StreamPublisher constructor
- Pass metrics to StreamManager constructor
- Register `/metrics` endpoint via `promhttp.Handler()`
- Add prometheus/client_golang import

## How It Works

### Metrics Collection Flow

```
1. InnerTube Client makes API request
   → Increment requests_total
   → On error: Increment errors_total{error_type="http|parse|rate_limit"}

2. Messages parsed successfully
   → Publisher.Publish() called

3. Redis publish attempt
   → Increment redis_publish_attempts_total
   → Measure latency (start timer)
   → On success: Increment redis_publish_success_total + messages_published_total{channel_id}
   → On failure: Increment errors_total{error_type="redis"}
   → Record latency histogram

4. Transient error occurs
   → Poller enters backoff
   → Increment reconnections_total{reason="error"}
```

### Service Label Convention

All metrics use `service="youtube-listener-innertube-canary"` label:
- Matches Kubernetes Service name from deployment manifest
- Enables PromQL filtering: `rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m])`
- Allows side-by-side comparison with baseline: `service="youtube-listener"`

### AnalysisTemplate Integration (Phase 12-02)

Metrics designed for Argo Rollouts AnalysisTemplate queries:

**Error Rate Comparison:**
```promql
# Canary error rate
rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m])
/ rate(youtube_listener_requests_total{service="youtube-listener-innertube-canary"}[5m])

# Baseline error rate
rate(youtube_listener_errors_total{service="youtube-listener"}[5m])
/ rate(youtube_listener_requests_total{service="youtube-listener"}[5m])
```

**Message Rate Comparison:**
```promql
rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m])
vs
rate(youtube_listener_messages_published_total{service="youtube-listener"}[5m])
```

**Redis Health Check:**
```promql
rate(youtube_listener_redis_publish_success_total{service="youtube-listener-innertube-canary"}[5m])
/ rate(youtube_listener_redis_publish_attempts_total{service="youtube-listener-innertube-canary"}[5m])
```

**Reconnection Rate (Stability):**
```promql
rate(youtube_listener_reconnections_total{service="youtube-listener-innertube-canary"}[5m])
```

## Deviations from Plan

None - plan executed exactly as written.

## Testing Verification

**Build Verification:**
```bash
✅ go build ./services/youtube-listener-innertube/...
   Compiles without errors
```

**Metric Names Verification:**
```bash
✅ grep -r "youtube_listener_" services/youtube-listener-innertube/metrics/
   All 7 metric families use correct naming convention
   - youtube_listener_errors_total
   - youtube_listener_requests_total
   - youtube_listener_messages_published_total
   - youtube_listener_redis_publish_attempts_total
   - youtube_listener_redis_publish_success_total
   - youtube_listener_redis_publish_latency_seconds
   - youtube_listener_reconnections_total
```

**Service Label Verification:**
```bash
✅ grep -r "youtube-listener-innertube-canary" services/youtube-listener-innertube/
   Consistent service label usage in metrics package
   const ServiceLabel = "youtube-listener-innertube-canary"
```

**Endpoint Registration Verification:**
```bash
✅ grep "promhttp.Handler" services/youtube-listener-innertube/cmd/main.go
   router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

## Example PromQL Queries for Manual Testing

### 1. Check if metrics are being collected
```promql
youtube_listener_requests_total{service="youtube-listener-innertube-canary"}
```

### 2. Error rate over last 5 minutes
```promql
rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m])
/ rate(youtube_listener_requests_total{service="youtube-listener-innertube-canary"}[5m])
```

### 3. Messages published per second
```promql
rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[1m])
```

### 4. Redis publish success rate
```promql
rate(youtube_listener_redis_publish_success_total{service="youtube-listener-innertube-canary"}[5m])
/ rate(youtube_listener_redis_publish_attempts_total{service="youtube-listener-innertube-canary"}[5m])
* 100
```

### 5. P95 Redis publish latency
```promql
histogram_quantile(0.95,
  rate(youtube_listener_redis_publish_latency_seconds_bucket{service="youtube-listener-innertube-canary"}[5m])
)
```

### 6. Reconnection rate (stability indicator)
```promql
rate(youtube_listener_reconnections_total{service="youtube-listener-innertube-canary"}[5m])
```

### 7. Error breakdown by type
```promql
sum by (error_type) (
  rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m])
)
```

## Next Steps (Phase 12-02)

1. **Create Kubernetes ServiceMonitor** for Prometheus scraping
2. **Define Argo Rollouts AnalysisTemplate** with metric thresholds
3. **Create canary Deployment manifest** with rollout strategy
4. **Test metrics endpoint** with curl/Prometheus before deployment
5. **Validate PromQL queries** return expected data structure

## Self-Check: PASSED

**Created files verification:**
```bash
✅ services/youtube-listener-innertube/metrics/innertube_metrics.go
   [ -f "services/youtube-listener-innertube/metrics/innertube_metrics.go" ] && echo "FOUND"
   FOUND
```

**Modified files verification:**
```bash
✅ services/youtube-listener-innertube/publisher/redis_publisher.go (metrics tracking added)
✅ services/youtube-listener-innertube/innertube/client.go (error tracking added)
✅ services/youtube-listener-innertube/poller/poller.go (reconnection tracking added)
✅ services/youtube-listener-innertube/streams/manager.go (metrics propagation added)
✅ services/youtube-listener-innertube/cmd/main.go (metrics initialized, /metrics endpoint registered)
```

**Commits verification:**
```bash
✅ 250360b: feat(12-01): create InnerTube Prometheus metrics package
   git log --oneline --all | grep -q "250360b" && echo "FOUND: 250360b"
   FOUND: 250360b

✅ 91413f0: feat(12-01): instrument InnerTube components with metrics
   git log --oneline --all | grep -q "91413f0" && echo "FOUND: 91413f0"
   FOUND: 91413f0

✅ 42bc0fb: feat(12-01): register Prometheus HTTP handler in main
   git log --oneline --all | grep -q "42bc0fb" && echo "FOUND: 42bc0fb"
   FOUND: 42bc0fb
```

All artifacts created, all commits present, all verifications passed.
