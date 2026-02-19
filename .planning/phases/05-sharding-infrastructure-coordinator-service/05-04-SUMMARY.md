---
phase: 05-sharding-infrastructure-coordinator-service
plan: 04
subsystem: source-manager
tags: [http-api, heartbeat, prometheus, observability]
dependency_graph:
  requires:
    - 05-01-bounded-load-consistent-hashing
    - 05-02-kubernetes-lease-coordinator
    - 05-03-heartbeat-monitoring
  provides:
    - assignment-query-api
    - heartbeat-publishing-api
    - prometheus-sharding-metrics
  affects:
    - listener-pod-integration
    - coordinator-observability
tech_stack:
  added: []
  patterns:
    - http-api-with-jwt-auth
    - prometheus-metrics-integration
key_files:
  created:
    - shared/metrics/shard_metrics.go
    - services/source-manager/handlers/assignments.go
  modified:
    - services/source-manager/models/assignment.go
    - services/source-manager/coordination/coordinator.go
    - services/source-manager/cmd/main.go
decisions:
  - title: "Service JWT authentication for endpoints"
    rationale: "Reuse existing SERVICE_JWT_AUTH middleware from source-manager. Consistent security pattern across all protected endpoints."
  - title: "Query parameter for pod_id in GET /assignments"
    rationale: "RESTful pattern for filtering. Simple and explicit. Query string easier for debugging than path parameters."
  - title: "Metrics integrated into coordinator reconciliation loop"
    rationale: "Coordinator is source of truth for pod health and assignment state. Natural place to update metrics during reconciliation."
metrics:
  duration_minutes: 3
  tasks_completed: 3
  files_created: 2
  files_modified: 3
  commits: 3
  completed_date: 2026-02-19
---

# Phase 05 Plan 04: Listener Pod Heartbeat Publisher & Query Endpoints

**One-liner:** Production-ready HTTP API for listener pods to query assigned channels (GET /assignments) and publish heartbeats (POST /heartbeat) with Prometheus metrics for sharding observability.

## What Was Built

### 1. Prometheus Metrics for Sharding Operations

**File:** `shared/metrics/shard_metrics.go` (96 lines)

Comprehensive metrics package for monitoring sharding infrastructure:

**Assignment Operations:**
- `shard_assignments_total` - Counter for total assignments created
- `shard_assignment_query_duration_seconds` - Histogram for query latency
- `shard_assignment_errors_total` - Counter for assignment errors

**Heartbeat Operations:**
- `shard_heartbeats_published_total` - Counter for heartbeats published
- `shard_heartbeat_errors_total` - Counter for heartbeat errors

**Pod Health:**
- `shard_healthy_pods` - Gauge for number of healthy listener pods
- `shard_failed_pods` - Gauge for number of failed pods

**Load Distribution:**
- `shard_pod_load_max` - Gauge for maximum channel count across all pods
- `shard_pod_load_avg` - Gauge for average channel count per pod
- `shard_imbalance_ratio` - Gauge for load imbalance ratio (max_load / avg_load)

**Coordinator State:**
- `shard_coordinator_is_leader` - Gauge for leadership status (1=leader, 0=follower)
- `shard_reconciliation_cycles_total` - Counter for reconciliation cycles completed
- `shard_reconciliation_errors_total` - Counter for reconciliation errors
- `shard_orphaned_assignments` - Gauge for orphaned assignments detected

**Why these metrics:**
- Assignment query latency enables alerting on Redis performance degradation
- Load imbalance ratio enables alerting when >1.5 per RESEARCH.md Pitfall 4
- Coordinator leadership status enables monitoring split-brain scenarios
- Reconciliation errors enable alerting on pod discovery failures

### 2. Assignment HTTP Handlers

**File:** `services/source-manager/handlers/assignments.go` (107 lines)

Two HTTP endpoints for listener pod integration:

**GET /assignments?pod_id={pod_id}** (SHARD-03):
- Query all channel assignments for a specific listener pod
- Returns O(1) results from Redis registry (via GetAssignmentsForPod)
- Tracks query latency with Prometheus histogram
- Returns JSON response: `{assignments: [...], count: N}`
- Authentication: SERVICE_JWT_AUTH middleware (existing pattern)

**POST /heartbeat**:
- Publish heartbeat for a listener pod
- Request body: `{pod_id: "twitch-listener-abc123"}`
- Stores heartbeat timestamp in Redis Sorted Set (via HeartbeatMonitor)
- Increments `shard_heartbeats_published_total` metric
- Returns: `{status: "ok"}`

**Error Handling:**
- Bad request (400) for missing pod_id parameter
- Internal server error (500) for Redis failures
- All errors logged with zap.Logger
- All errors tracked in metrics (AssignmentErrors, HeartbeatErrors)

**File:** `services/source-manager/models/assignment.go` (21 lines)

Added JSON response models:
- `AssignmentResponse` - Array of assignments with count
- `HeartbeatRequest` - Pod ID binding for heartbeat publishing

### 3. Endpoint Registration and Metrics Integration

**File:** `services/source-manager/cmd/main.go`

**Changes:**
1. Initialize ShardMetrics after BusinessMetrics
2. Pass shardMetrics to Coordinator constructor
3. Create AssignmentHandler with registry, heartbeatMonitor, shardMetrics, logger
4. Register endpoints under protected route group:
   - `protected.GET("/assignments", assignmentHandler.GetAssignments)`
   - `protected.POST("/heartbeat", assignmentHandler.PublishHeartbeat)`

**File:** `services/source-manager/coordination/coordinator.go`

**Metrics Integration:**

Updated coordinator to track metrics during reconciliation:

1. **Leadership callbacks:**
   - `OnStartedLeading`: Set `CoordinatorIsLeader = 1`
   - `OnStoppedLeading`: Set `CoordinatorIsLeader = 0`

2. **Reconciliation cycle:**
   - Increment `ReconciliationCycles` on every cycle completion
   - Increment `ReconciliationErrors` on failures (failed pod detection, pod queries, assignments)

3. **Pod health tracking:**
   - Set `FailedPods` gauge from heartbeat monitor
   - Set `HealthyPods` gauge after Kubernetes API query

4. **Assignment operations:**
   - Increment `AssignmentsTotal` for each successful assignment storage
   - Increment `ReconciliationErrors` for assignment/storage failures

**Why coordinator integration:**
- Coordinator has authoritative view of cluster state (pod health, assignments)
- Natural place to update metrics during reconciliation loop
- No additional Redis queries needed (reuses existing data)

## Deviations from Plan

None - plan executed exactly as written.

**Clarifications:**
- Coordinator signature updated to accept `*metrics.ShardMetrics` parameter
- Registry already had `GetAssignmentsForPod` method (created in Plan 05-01)
- Handler converts `[]*models.Assignment` (registry API) to `[]models.Assignment` (response API)

## Key Decisions

### 1. Service JWT Authentication for Endpoints

**Context:** Listener pods need secure access to assignment queries and heartbeat publishing

**Decision:** Reuse existing `middleware.ServiceJWTAuth(serviceAuthSecret)` from source-manager

**Why:**
- Consistent security pattern across all source-manager endpoints
- Listener pods already configured with SERVICE_JWT_SECRET (existing pattern)
- No additional infrastructure needed (no new secrets, no new middleware)
- Follows existing pattern from `/sources`, `/leadership` endpoints

**Alternative considered:** No authentication - rejected, exposes internal sharding API

### 2. Query Parameter for pod_id in GET /assignments

**Context:** Need to filter assignments by pod

**Decision:** Use query parameter: `GET /assignments?pod_id={pod_id}`

**Why:**
- RESTful pattern for filtering/searching resources
- Simple and explicit (clear what parameter does)
- Query string easier to read/debug than path parameters
- Consistent with existing `/sources?platform=twitch` pattern in source-manager

**Alternatives considered:**
- Path parameter (`/assignments/{pod_id}`) - less clear for filtering operation
- POST with body - incorrect HTTP semantics for read operation

### 3. Metrics Integrated into Coordinator Reconciliation Loop

**Context:** Need real-time metrics for pod health, assignments, and coordinator state

**Decision:** Update metrics during coordinator reconciliation (every 30s)

**Why:**
- Coordinator is authoritative source of truth for cluster state
- Natural place to update metrics (already has all data needed)
- No additional Redis queries required (reuses existing data)
- Metrics reflect actual reconciliation state (not stale)

**Alternatives considered:**
- Separate metrics collection goroutine - rejected, duplicate Redis queries
- Update metrics in handlers only - rejected, doesn't capture reconciliation failures

## Integration Points

**Upstream (Plans 05-01, 05-02, 05-03):**
- ✅ Uses AssignmentRegistry.GetAssignmentsForPod from Plan 05-01
- ✅ Uses HeartbeatMonitor.PublishHeartbeat from Plan 05-03
- ✅ Coordinator integrates with existing reconciliation loop from Plan 05-02

**Downstream (Phase 06):**
- ✅ GET /assignments endpoint ready for listener pod startup queries
- ✅ POST /heartbeat endpoint ready for listener pod background goroutine
- ✅ Prometheus metrics ready for Grafana dashboards
- ✅ All endpoints protected by JWT authentication (listener pods can authenticate)

## Validation Results

**Success Criteria from Plan:**

✅ shared/metrics/shard_metrics.go defines Prometheus metrics for assignments, heartbeats, pod health, load distribution, and coordinator state

✅ handlers/assignments.go implements GetAssignments endpoint querying assignments by pod_id (SHARD-03)

✅ handlers/assignments.go implements PublishHeartbeat endpoint storing heartbeat in Redis

✅ models/assignment.go includes JSON tags for API responses (AssignmentResponse, HeartbeatRequest)

✅ AssignmentRegistry.GetAssignmentsForPod() already exists (created in Plan 05-01)

✅ cmd/main.go initializes ShardMetrics and passes to AssignmentHandler and Coordinator

✅ GET /assignments and POST /heartbeat registered under protected route group (SERVICE_JWT_AUTH)

✅ Assignment query latency tracked with histogram metric

✅ Heartbeat operations tracked with counter metrics

✅ Service compiles successfully with all endpoints registered

✅ Endpoints follow existing source-manager patterns (JWT auth, error handling, logging)

**Verification Commands:**

```bash
# Metrics package compiles
cd shared/metrics && go build ./shard_metrics.go
# ✓ Exit code 0

# Handler compiles
cd services/source-manager && go build ./handlers/assignments.go
# ✓ Exit code 0

# Service compiles
cd services/source-manager && go build ./cmd/main.go
# ✓ Exit code 0

# Endpoints registered
grep "GET.*assignments" services/source-manager/cmd/main.go
grep "POST.*heartbeat" services/source-manager/cmd/main.go
# ✓ Both endpoints present under protected route group
```

## API Documentation

### GET /assignments

**Query all channel assignments for a specific listener pod**

**Endpoint:** `GET /assignments?pod_id={pod_id}`

**Authentication:** SERVICE_JWT_AUTH (Bearer token required)

**Query Parameters:**
- `pod_id` (required) - Kubernetes pod name (e.g., "twitch-listener-abc123")

**Response (200 OK):**
```json
{
  "assignments": [
    {
      "source_id": "uuid-1",
      "pod_id": "twitch-listener-abc123",
      "timestamp": "2026-02-19T19:40:00Z",
      "version": 42
    },
    {
      "source_id": "uuid-2",
      "pod_id": "twitch-listener-abc123",
      "timestamp": "2026-02-19T19:40:01Z",
      "version": 43
    }
  ],
  "count": 2
}
```

**Error Responses:**
- `400 Bad Request` - Missing pod_id parameter
- `500 Internal Server Error` - Redis query failure

**Usage Example (Go):**
```go
req, _ := http.NewRequest("GET", "http://source-manager:8088/assignments?pod_id=twitch-listener-abc123", nil)
req.Header.Set("Authorization", "Bearer " + serviceJWT)
resp, _ := client.Do(req)
```

### POST /heartbeat

**Publish heartbeat for a listener pod**

**Endpoint:** `POST /heartbeat`

**Authentication:** SERVICE_JWT_AUTH (Bearer token required)

**Request Body:**
```json
{
  "pod_id": "twitch-listener-abc123"
}
```

**Response (200 OK):**
```json
{
  "status": "ok"
}
```

**Error Responses:**
- `400 Bad Request` - Invalid request body or missing pod_id
- `500 Internal Server Error` - Redis write failure

**Usage Example (Go):**
```go
body := `{"pod_id":"twitch-listener-abc123"}`
req, _ := http.NewRequest("POST", "http://source-manager:8088/heartbeat", strings.NewReader(body))
req.Header.Set("Authorization", "Bearer " + serviceJWT)
req.Header.Set("Content-Type", "application/json")
resp, _ := client.Do(req)
```

**Heartbeat Frequency:**
- Listener pods should publish heartbeat every 10 seconds (recommended)
- Heartbeat timeout: 15 seconds (pods marked failed if no heartbeat for >15s)
- Coordinator queries failed pods every 30 seconds (reconciliation interval)

## Prometheus Metrics Reference

### Alerting Rules (Recommended)

**High assignment query latency:**
```yaml
alert: HighAssignmentQueryLatency
expr: histogram_quantile(0.99, shard_assignment_query_duration_seconds) > 0.1
for: 5m
severity: warning
description: "99th percentile assignment query latency is {{ $value }}s (threshold: 0.1s)"
```

**Load imbalance detected:**
```yaml
alert: HighLoadImbalance
expr: shard_imbalance_ratio > 1.5
for: 10m
severity: warning
description: "Load imbalance ratio is {{ $value }} (threshold: 1.5)"
```

**No coordinator leader:**
```yaml
alert: NoCoordinatorLeader
expr: sum(shard_coordinator_is_leader) == 0
for: 1m
severity: critical
description: "No coordinator leader elected (split-brain or all replicas down)"
```

**High reconciliation errors:**
```yaml
alert: HighReconciliationErrors
expr: rate(shard_reconciliation_errors_total[5m]) > 0.1
for: 5m
severity: warning
description: "Reconciliation error rate is {{ $value }} errors/sec"
```

### Grafana Dashboard Panels (Recommended)

**Panel 1: Pod Health Overview**
- Gauge: `shard_healthy_pods` (green)
- Gauge: `shard_failed_pods` (red)
- Graph: Both metrics over time (last 1h)

**Panel 2: Assignment Operations**
- Counter: `rate(shard_assignments_total[5m])`
- Histogram: `shard_assignment_query_duration_seconds` (p50, p99)
- Counter: `rate(shard_assignment_errors_total[5m])`

**Panel 3: Heartbeat Operations**
- Counter: `rate(shard_heartbeats_published_total[5m])`
- Counter: `rate(shard_heartbeat_errors_total[5m])`

**Panel 4: Load Distribution**
- Gauge: `shard_pod_load_max`
- Gauge: `shard_pod_load_avg`
- Gauge: `shard_imbalance_ratio` (alert if >1.5)

**Panel 5: Coordinator State**
- Gauge: `shard_coordinator_is_leader` (1=leader, 0=follower)
- Counter: `rate(shard_reconciliation_cycles_total[5m])`
- Counter: `rate(shard_reconciliation_errors_total[5m])`
- Gauge: `shard_orphaned_assignments`

## Files Created

1. **shared/metrics/shard_metrics.go** (96 lines)
   - Prometheus metrics for sharding operations
   - Exports: NewShardMetrics

2. **services/source-manager/handlers/assignments.go** (107 lines)
   - HTTP handlers for assignment queries and heartbeat publishing
   - Exports: NewAssignmentHandler, GetAssignments, PublishHeartbeat

## Files Modified

1. **services/source-manager/models/assignment.go**
   - Added AssignmentResponse struct for JSON response
   - Added HeartbeatRequest struct for JSON request

2. **services/source-manager/coordination/coordinator.go**
   - Added metrics field to Coordinator struct
   - Updated NewCoordinator to accept shardMetrics parameter
   - Integrated metrics tracking in leadership callbacks
   - Integrated metrics tracking in reconciliation loop

3. **services/source-manager/cmd/main.go**
   - Initialize ShardMetrics
   - Pass shardMetrics to Coordinator constructor
   - Create AssignmentHandler
   - Register GET /assignments and POST /heartbeat endpoints

## Commits

| Commit | Type | Message |
|--------|------|---------|
| 3971951 | feat | Add Prometheus metrics for sharding operations |
| c7687fc | feat | Implement assignment HTTP handlers |
| 3e49513 | feat | Register assignment endpoints and integrate metrics |

**Commit sequence:** Metrics → Handlers → Integration (clean separation of concerns)

## Next Steps (Plan 05-05)

**Prerequisites met:**
- ✅ GET /assignments endpoint ready for listener pod queries
- ✅ POST /heartbeat endpoint ready for listener pod background goroutine
- ✅ Prometheus metrics ready for monitoring and alerting
- ✅ All endpoints protected by JWT authentication

**Ready to implement (Phase 06):**
1. Listener pod startup: Query GET /assignments?pod_id={pod_id} on startup
2. Listener pod heartbeat publisher: Background goroutine publishes heartbeat every 10s
3. Listener pod assignment watcher: Poll for assignment changes, reconnect channels
4. Listener pod authentication: Use SERVICE_JWT_SECRET to generate JWT for API calls
5. Grafana dashboard: Create panels for sharding metrics
6. Alerting: Configure Prometheus alerts for load imbalance, query latency, reconciliation errors

## Self-Check: PASSED

**Files created:**
```
✅ shared/metrics/shard_metrics.go (96 lines)
✅ services/source-manager/handlers/assignments.go (107 lines)
```

**Files modified:**
```
✅ services/source-manager/models/assignment.go (JSON tags added)
✅ services/source-manager/coordination/coordinator.go (metrics integration)
✅ services/source-manager/cmd/main.go (endpoint registration)
```

**Commits exist:**
```
✅ 3971951: feat(05-04): add Prometheus metrics for sharding operations
✅ c7687fc: feat(05-04): implement assignment HTTP handlers
✅ 3e49513: feat(05-04): register assignment endpoints and integrate metrics
```

**Build verification:**
```
✅ go build shared/metrics/shard_metrics.go (successful)
✅ go build services/source-manager/handlers/assignments.go (successful)
✅ go build services/source-manager/cmd/main.go (successful)
```

**Endpoint registration:**
```
✅ protected.GET("/assignments", assignmentHandler.GetAssignments)
✅ protected.POST("/heartbeat", assignmentHandler.PublishHeartbeat)
```

**Requirements met:**
```
✅ SHARD-03: Assignment query endpoint with O(1) performance
✅ Heartbeat publishing endpoint for pod health tracking
✅ Prometheus metrics for observability
✅ JWT authentication for all endpoints
```
