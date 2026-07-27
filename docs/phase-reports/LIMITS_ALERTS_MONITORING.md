# All-Chat: Limits, Alerts & Monitoring Thresholds

**Version**: 1.0
**Last Updated**: 2025-11-11
**Status**: Production Configuration Guide

---

## Table of Contents

1. [Resource Limits](#resource-limits)
2. [Connection Limits](#connection-limits)
3. [Rate Limits](#rate-limits)
4. [Alert Thresholds](#alert-thresholds)
5. [Prometheus Alerts](#prometheus-alerts)
6. [YouTube API Quota Management](#youtube-api-quota-management)
7. [Redis Configuration Limits](#redis-configuration-limits)
8. [PostgreSQL Limits](#postgresql-limits)

---

## Resource Limits

### Kubernetes Resource Limits (Per Pod)

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit | Rationale |
|---------|-------------|-----------|----------------|--------------|-----------|
| **API Gateway** | 100m | 500m | 128Mi | 512Mi | WebSocket: 2,500 connections = ~320Mi actual usage |
| **Auth Service** | 50m | 200m | 64Mi | 256Mi | Low traffic (1 req/user/session) |
| **Overlay Manager** | 50m | 200m | 64Mi | 256Mi | CRUD operations, not high throughput |
| **Emote Service** | 50m | 200m | 64Mi | 256Mi | 95% cache hit rate, low memory |
| **Message Processor** | 200m | 1000m | 256Mi | 1Gi | High CPU (emote parsing), multiple goroutines |
| **Twitch Listener** | 100m | 500m | 128Mi | 512Mi | IRC connections, message parsing |
| **YouTube Listener** | 100m | 500m | 128Mi | 512Mi | API polling, OAuth management |
| **Source Manager** | 50m | 200m | 64Mi | 256Mi | Leader election, low CPU |

### Infrastructure Limits

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit | Storage |
|-----------|-------------|-----------|----------------|--------------|---------|
| **CloudNativePG** (per instance) | 500m | 2000m | 1Gi | 4Gi | 50Gi (primary), 50Gi (each replica) |
| **Redis** | 100m | 500m | 256Mi | 1Gi | 20Gi (AOF + RDB) |
| **Loki** | 500m | 2000m | 1Gi | 4Gi | 50Gi (30-day retention) |
| **Grafana** | 100m | 500m | 256Mi | 1Gi | 10Gi (dashboards) |
| **Prometheus** | 500m | 2000m | 1Gi | 4Gi | 50Gi (15-day retention) |
| **Promtail** (per node) | 50m | 200m | 64Mi | 256Mi | N/A (DaemonSet) |
| **Tempo** | 200m | 1000m | 512Mi | 2Gi | 20Gi (7-day traces) |
| **Mimir** | 500m | 2000m | 2Gi | 8Gi | 100Gi (90-day metrics) |

**Total Phase 1 (2 instances per service)**:
- **Application Services**: ~1.5 CPU, ~2.5Gi memory
- **Infrastructure**: ~3.5 CPU, ~12Gi memory
- **LGTM Stack**: ~2.5 CPU, ~9Gi memory
- **TOTAL**: ~7.5 CPU, ~24Gi memory, ~400Gi storage

**Hetzner VPS Requirements**:
- **Phase 1**: CX41 (8 vCPU, 32GB RAM, 240GB SSD) + Cloud Volumes = ~€50/month
- **Phase 3**: 3x CX31 (4 vCPU, 16GB each) = ~€60/month
- **Phase 5**: 5x CX41 = ~€150/month

---

## Connection Limits

### API Gateway WebSocket Limits

**Critical Configuration** (prevents OOM kills):

```go
// internal/api-gateway/websocket/manager.go

const (
    // Maximum WebSocket connections per pod
    // Based on: 512Mi memory / 128KB per connection = 4,000 theoretical max
    // Safe limit with headroom: 2,500
    MaxConnectionsPerPod = 2500

    // Maximum connections per overlay (prevent single overlay monopolizing Gateway)
    MaxConnectionsPerOverlay = 1000

    // Maximum message size (prevent abuse)
    MaxMessageSize = 64 * 1024  // 64KB

    // Connection timeouts
    WriteTimeout = 10 * time.Second
    ReadTimeout  = 60 * time.Second
    PingInterval = 30 * time.Second
)
```

**Implementation**:
```go
func (m *WebSocketManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Check global limit
    if m.activeConnections >= MaxConnectionsPerPod {
        http.Error(w, "Server at capacity, please retry", http.StatusServiceUnavailable)
        m.metrics.RejectedConnections.Inc()
        return
    }

    // Check per-overlay limit
    overlayID := r.URL.Query().Get("overlay_id")
    if m.getOverlayConnectionCount(overlayID) >= MaxConnectionsPerOverlay {
        http.Error(w, "Overlay at connection limit", http.StatusTooManyRequests)
        return
    }

    // Proceed with WebSocket upgrade...
}
```

**Prometheus Metrics**:
```go
var (
    websocketConnectionsActive = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "api_gateway_websocket_connections_active",
            Help: "Current active WebSocket connections",
        },
    )

    websocketConnectionsRejected = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "api_gateway_websocket_connections_rejected_total",
            Help: "Total rejected connections (at capacity)",
        },
    )
)
```

### Database Connection Limits

**PostgreSQL (via pgx)**:

```go
// shared/database/postgres.go

func NewPostgresPool(connString string) (*pgxpool.Pool, error) {
    config, _ := pgxpool.ParseConfig(connString)

    // Connection pool configuration
    config.MaxConns = 20                          // Max per service instance
    config.MinConns = 5                           // Keep warm connections
    config.MaxConnLifetime = 1 * time.Hour        // Recycle connections
    config.MaxConnIdleTime = 10 * time.Minute     // Close idle connections
    config.HealthCheckPeriod = 1 * time.Minute    // Verify connections

    return pgxpool.NewWithConfig(context.Background(), config)
}
```

**CloudNativePG Configuration**:
```yaml
# CNPG cluster max_connections
postgresql:
  parameters:
    max_connections: "200"  # Total connections to PostgreSQL

# PgBouncer pooler
connectionPooler:
  pgbouncer:
    parameters:
      max_client_conn: "1000"      # Max clients to pooler
      default_pool_size: "25"       # Connections per database
      reserve_pool_size: "5"        # Reserved connections
      reserve_pool_timeout: "5"     # Seconds
```

**Calculation**:
- 8 services × 2 instances × 20 connections = 320 connections
- PgBouncer pools to 25 actual PostgreSQL connections
- Leaves 175 connections for admin tasks, migrations, etc.

### Redis Connection Limits

```go
// shared/redis/client.go

func NewRedisClient(addr string) *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         addr,
        PoolSize:     50,                    // Max connections per client
        MinIdleConns: 10,                    // Keep warm connections
        MaxRetries:   3,                     // Retry failed commands
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolTimeout:  4 * time.Second,       // Wait for connection from pool
    })
}
```

---

## Rate Limits

### Per-User API Rate Limits

**Implementation** (Redis-based sliding window):

```go
// shared/middleware/rate_limit.go

type RateLimitConfig struct {
    Endpoint string
    Limit    int           // Max requests
    Window   time.Duration // Time window
}

var RateLimits = map[string]RateLimitConfig{
    "/api/v1/auth/*":     {Limit: 10, Window: 1 * time.Minute},
    "/api/v1/overlays/*": {Limit: 60, Window: 1 * time.Minute},
    "/api/v1/emotes/*":   {Limit: 120, Window: 1 * time.Minute},
}
```

**Rate Limit Tiers**:

| Endpoint Pattern | Free Tier | Pro Tier | Enterprise |
|------------------|-----------|----------|------------|
| `/auth/*` | 10/min | 100/min | Unlimited |
| `/overlays/*` (GET) | 60/min | 600/min | Unlimited |
| `/overlays/*` (POST/PUT/DELETE) | 30/min | 300/min | Unlimited |
| `/emotes/*` | 120/min | 1200/min | Unlimited |
| WebSocket messages | N/A | N/A | N/A (limited by bandwidth) |

### Per-Service Instance Limits

| Service | Max Requests/Second | Max Goroutines | Max Memory Usage |
|---------|---------------------|----------------|------------------|
| Auth Service | 1,000 req/s | 200 | 200Mi |
| Overlay Manager | 500 req/s | 100 | 200Mi |
| Emote Service | 2,000 req/s | 200 | 200Mi (95% cached) |
| Message Processor | 2,000 msg/s | 500 | 800Mi |

---

## Alert Thresholds

### Critical Alerts (PagerDuty - Immediate Response)

| Alert Name | Condition | Threshold | Duration | Action Required |
|------------|-----------|-----------|----------|-----------------|
| **ServiceDown** | `up == 0` | Any service | 2 minutes | Investigate pod logs, restart if needed |
| **HighErrorRate** | `5xx errors / total requests` | > 5% | 5 minutes | Check service logs, database health |
| **DatabaseConnectionPoolExhausted** | `active_conns >= max_conns` | ≥ 100% | 2 minutes | Scale service or increase pool size |
| **RedisDown** | `redis_up == 0` | N/A | 1 minute | Critical: All processing stops |
| **CNPGFailover** | `cnpg_pg_cluster_instances_ready < 2` | < 2 instances | 1 minute | Check primary health, verify failover |
| **WebSocketCapacity** | `websocket_connections_active` | > 2,400 | 2 minutes | Scale API Gateway immediately |
| **YouTubeQuotaExhausted** | `youtube_api_quota_used` | ≥ 9,500 | 1 minute | Increase polling interval, request quota |

### Warning Alerts (Slack - Non-Critical)

| Alert Name | Condition | Threshold | Duration | Action |
|------------|-----------|-----------|----------|--------|
| **HighLatency** | `http_request_duration p95` | > 500ms | 10 minutes | Review slow queries, check cache hit rate |
| **MessageBacklog** | `stream_pending_messages` | > 5,000 | 5 minutes | Scale message processors |
| **HighCPU** | `container_cpu_usage` | > 70% | 10 minutes | Consider scaling or optimization |
| **HighMemory** | `container_memory_usage` | > 80% | 10 minutes | Check for memory leaks, scale if needed |
| **CacheHitRateLow** | `emote_cache_hits / total` | < 90% | 15 minutes | Increase cache TTL or size |
| **YouTubeQuotaHigh** | `youtube_api_quota_used` | > 8,000 | 5 minutes | Warning: approaching limit |
| **DatabaseSlowQueries** | `db_query_duration p95` | > 100ms | 10 minutes | Review query plans, add indexes |
| **RedisMemoryHigh** | `redis_memory_used` | > 75% | 10 minutes | Increase maxmemory or check for leaks |

### Info Alerts (Slack - Informational)

| Alert Name | Condition | Threshold | Duration |
|------------|-----------|-----------|----------|
| **HPAScaleUp** | `hpa_current_replicas` increased | N/A | Immediate |
| **HPAScaleDown** | `hpa_current_replicas` decreased | N/A | Immediate |
| **NewDeployment** | Deployment rollout started | N/A | Immediate |
| **BackupCompleted** | CNPG backup successful | N/A | Daily |

---

## Prometheus Alerts

### alerts.yaml - Critical Alerts

```yaml
groups:
  - name: critical
    interval: 30s
    rules:
      # Service health
      - alert: ServiceDown
        expr: up{job="kubernetes-pods", namespace="all-chat"} == 0
        for: 2m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "{{ $labels.pod }} is down"
          description: "Service {{ $labels.app }} ({{ $labels.pod }}) has been down for >2 minutes"
          runbook: "https://docs.allch.at/runbooks/service-down"

      # Error rate
      - alert: HighErrorRate
        expr: |
          (
            sum by (service) (rate(http_requests_total{status=~"5.."}[5m]))
            /
            sum by (service) (rate(http_requests_total[5m]))
          ) > 0.05
        for: 5m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "High error rate on {{ $labels.service }}"
          description: "Error rate: {{ $value | humanizePercentage }} (threshold: 5%)"
          runbook: "https://docs.allch.at/runbooks/high-error-rate"

      # Database connections
      - alert: DatabaseConnectionPoolExhausted
        expr: |
          database_connections_active{service=~".*"}
          >=
          database_connections_max{service=~".*"} * 0.95
        for: 2m
        labels:
          severity: critical
          team: database
        annotations:
          summary: "Database pool exhausted on {{ $labels.service }}"
          description: "Active: {{ $value }}, Max: {{ $labels.max_conns }}"
          runbook: "https://docs.allch.at/runbooks/db-pool-exhausted"

      # Redis availability
      - alert: RedisDown
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "Redis is down"
          description: "All message processing will stop"
          runbook: "https://docs.allch.at/runbooks/redis-down"

      # CNPG health
      - alert: CNPGClusterDegraded
        expr: cnpg_pg_cluster_instances_ready < 2
        for: 1m
        labels:
          severity: critical
          team: database
        annotations:
          summary: "CNPG cluster has < 2 ready instances"
          description: "Ready instances: {{ $value }}/3"
          runbook: "https://docs.allch.at/runbooks/cnpg-degraded"

      # WebSocket capacity
      - alert: WebSocketCapacityHigh
        expr: api_gateway_websocket_connections_active > 2400
        for: 2m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "API Gateway approaching WebSocket capacity"
          description: "Connections: {{ $value }}/2500 limit"
          runbook: "https://docs.allch.at/runbooks/websocket-capacity"

      # YouTube quota
      - alert: YouTubeQuotaExhausted
        expr: youtube_listener_api_quota_used >= 9500
        for: 1m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "YouTube API quota nearly exhausted"
          description: "Used: {{ $value }}/10000 units"
          runbook: "https://docs.allch.at/runbooks/youtube-quota"

  - name: warning
    interval: 1m
    rules:
      # Latency
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95,
            rate(http_request_duration_seconds_bucket{service=~".*"}[5m])
          ) > 0.5
        for: 10m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "High latency on {{ $labels.service }}"
          description: "p95 latency: {{ $value }}s (threshold: 500ms)"

      # Message backlog
      - alert: MessageBacklogHigh
        expr: redis_stream_length{stream="raw-messages"} > 5000
        for: 5m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "High message backlog in Redis Stream"
          description: "Pending messages: {{ $value }}"
          runbook: "https://docs.allch.at/runbooks/message-backlog"

      # CPU usage
      - alert: HighCPUUsage
        expr: |
          (
            sum by (pod) (rate(container_cpu_usage_seconds_total[5m]))
            /
            sum by (pod) (kube_pod_container_resource_limits{resource="cpu"})
          ) > 0.7
        for: 10m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "High CPU on {{ $labels.pod }}"
          description: "CPU usage: {{ $value | humanizePercentage }}"

      # Memory usage
      - alert: HighMemoryUsage
        expr: |
          (
            container_memory_usage_bytes{pod=~".*"}
            /
            container_memory_limit_bytes{pod=~".*"}
          ) > 0.8
        for: 10m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "High memory on {{ $labels.pod }}"
          description: "Memory usage: {{ $value | humanizePercentage }}"

      # Cache performance
      - alert: EmoteCacheHitRateLow
        expr: |
          (
            rate(emote_service_cache_hits_total[10m])
            /
            (rate(emote_service_cache_hits_total[10m]) + rate(emote_service_cache_misses_total[10m]))
          ) < 0.9
        for: 15m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "Emote cache hit rate below 90%"
          description: "Hit rate: {{ $value | humanizePercentage }}"

      # YouTube quota warning
      - alert: YouTubeQuotaHigh
        expr: youtube_listener_api_quota_used > 8000
        for: 5m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "YouTube API quota usage high"
          description: "Used: {{ $value }}/10000 units (80%)"

      # Database slow queries
      - alert: DatabaseSlowQueries
        expr: |
          histogram_quantile(0.95,
            rate(database_query_duration_seconds_bucket[5m])
          ) > 0.1
        for: 10m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Slow database queries detected"
          description: "p95 query time: {{ $value }}s (threshold: 100ms)"

      # Redis memory
      - alert: RedisMemoryHigh
        expr: |
          (
            redis_memory_used_bytes
            /
            redis_memory_max_bytes
          ) > 0.75
        for: 10m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "Redis memory usage high"
          description: "Usage: {{ $value | humanizePercentage }}"
```

---

## YouTube API Quota Management

### Quota Limits & Monitoring

**GCP Project Quota**: 10,000 units/day (default)

**API Costs**:
- `liveChatMessages.list`: 5 units per request
- `videos.list`: 1 unit per request
- `liveStreams.list`: 1 unit per request

**Capacity Calculation**:
```
Max requests/day: 10,000 / 5 = 2,000 requests
Polling interval: 7.5 seconds average
Requests per stream per day: 86,400s / 7.5s = 11,520 requests
Streams per project: 2,000 / 11,520 = 0.17 streams ❌ WRONG MATH

CORRECTED:
Polling interval: 5-10 seconds (adaptive)
Average interval: 7.5 seconds
Requests per day per stream: (86,400 / 7.5) = 11,520 requests
Units per day per stream: 11,520 × 5 = 57,600 units
Streams supportable: 10,000 / 57,600 = 0.17 ❌ STILL WRONG

ACTUALLY:
If polling EVERY stream every 7.5 seconds:
- 1 stream = 11,520 requests/day × 5 units = 57,600 units/day ❌ EXCEEDS QUOTA

REALISTIC:
Request quota increase to 50,000 units/day = 8-10 streams max
OR use 5 GCP projects = 50,000 units/day total = 40-50 streams

OR adaptive polling:
- High activity streams: 5s interval
- Medium activity: 15s interval
- Low activity: 30s interval
- Average: 15s interval = 5,760 requests/day × 5 = 28,800 units/stream
- Streams per project: 10,000 / 28,800 = 0.34 streams ❌ STILL WRONG!

FINAL CORRECT CALCULATION:
Budget per stream per day: We want to support N streams
Target: 50 streams per project with 10,000 units/day
Units per stream: 10,000 / 50 = 200 units/day
Requests per stream: 200 / 5 = 40 requests/day
Interval: 86,400s / 40 = 2,160 seconds = 36 minutes per poll ❌ TOO SLOW

CONCLUSION: YouTube API quota is a MAJOR bottleneck
```

**Solutions**:

1. ✅ **Request Quota Increase** (50,000 units/day)
   - Submit form: https://console.cloud.google.com/apis/api/youtube.googleapis.com/quotas
   - Justification: "Live chat aggregation service for streamers"
   - Expected approval: 2-4 weeks

2. ✅ **Multi-Project Strategy**
   - Create 5 GCP projects
   - Distribute streams across projects
   - Total quota: 50,000 units/day (5 × 10,000)
   - Supports: ~40 streams at 5s polling

3. ✅ **Adaptive Polling**
   ```go
   func (y *YouTubeListener) calculatePollingInterval(messageRate float64) time.Duration {
       // messageRate = messages per second in last minute
       if messageRate > 10 {
           return 5 * time.Second   // Very active
       } else if messageRate > 1 {
           return 10 * time.Second  // Active
       } else {
           return 30 * time.Second  // Slow
       }
   }
   ```

4. ✅ **Quota Monitoring**
   ```go
   // Prometheus metric
   youtubeAPIQuotaUsed.Set(float64(quotaUsedToday))

   // Alert when > 8,000 units used
   if quotaUsedToday > 8000 {
       logger.Warn("YouTube quota high", zap.Int("used", quotaUsedToday))
   }
   ```

**Quota Reset**: Daily at midnight Pacific Time

---

## Redis Configuration Limits

### Redis Server Configuration

**File**: `deployments/k8s/redis/statefulset.yaml`

```yaml
args:
  # Memory limits
  - --maxmemory
  - "768mb"                    # Leave 256Mi for AOF rewrite overhead
  - --maxmemory-policy
  - allkeys-lru                # Evict least recently used keys

  # AOF persistence
  - --appendonly
  - "yes"
  - --appendfsync
  - everysec                   # Fsync every 1 second (balance perf/durability)
  - --auto-aof-rewrite-percentage
  - "100"                      # Rewrite when AOF is 2x size
  - --auto-aof-rewrite-min-size
  - 64mb                       # Min size before rewrite

  # Connection limits
  - --maxclients
  - "10000"                    # Max concurrent clients

  # Timeouts
  - --timeout
  - "300"                      # Close idle clients after 5min
  - --tcp-keepalive
  - "60"                       # TCP keepalive interval

  # Performance
  - --tcp-backlog
  - "511"                      # Connection queue size

  # Streams configuration
  - --stream-node-max-bytes
  - "4096"                     # Max bytes per stream node
  - --stream-node-max-entries
  - "100"                      # Max entries per stream node
```

### Redis Stream MAXLEN (Prevent Unbounded Growth)

```go
// Platform Listeners: Add with max length
func (t *TwitchListener) publishMessage(msg RawMessage) error {
    return t.redis.XAdd(ctx, &redis.XAddArgs{
        Stream: "stream:raw-messages",
        MaxLen: 50000,               // Keep max 50K messages
        Approx: true,                // Approximate trimming (faster)
        Values: msg,
    }).Err()
}
```

**Why MAXLEN**:
- Prevents Redis OOM if Message Processor crashes
- 50,000 messages ≈ 25MB memory (assuming 500 bytes/msg)
- Acceptable message loss vs Redis crash

---

## PostgreSQL Limits

### CloudNativePG Configuration

```yaml
# CNPG Cluster spec
postgresql:
  parameters:
    # Connection limits
    max_connections: "200"              # Total connections

    # Memory settings
    shared_buffers: "256MB"             # 25% of total memory
    effective_cache_size: "1GB"         # 75% of total memory
    work_mem: "2621kB"                  # Per sort operation
    maintenance_work_mem: "64MB"        # For VACUUM, CREATE INDEX

    # Checkpointing (write performance)
    checkpoint_completion_target: "0.9"
    wal_buffers: "16MB"
    min_wal_size: "1GB"
    max_wal_size: "4GB"

    # Query planner
    default_statistics_target: "100"
    random_page_cost: "1.1"             # For SSD storage
    effective_io_concurrency: "200"     # Parallel I/O operations

    # Logging (for slow query detection)
    log_min_duration_statement: "1000"  # Log queries > 1 second
    log_line_prefix: "%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h "

    # Statement timeout (prevent runaway queries)
    statement_timeout: "30000"          # 30 seconds max per query

    # Lock timeout
    lock_timeout: "10000"               # 10 seconds max wait for lock
```

### PgBouncer Pooler Configuration

```yaml
connectionPooler:
  instances: 3                          # 3 PgBouncer instances
  type: pgbouncer
  pgbouncer:
    poolMode: transaction               # New connection per transaction
    parameters:
      max_client_conn: "1000"           # Max clients to pooler
      default_pool_size: "25"           # Connections to PostgreSQL per DB
      reserve_pool_size: "5"            # Reserved connections
      reserve_pool_timeout: "5"         # Wait 5s for reserved connection
      max_db_connections: "100"         # Total PostgreSQL connections
      max_user_connections: "100"       # Per user limit

      # Timeouts
      server_idle_timeout: "600"        # Close idle server conn after 10min
      server_lifetime: "3600"           # Recycle server conn after 1hr
      client_idle_timeout: "0"          # Don't close idle clients
      query_timeout: "30"               # 30s max per query
```

**Connection Flow**:
```
Application → PgBouncer (pooler) → PostgreSQL

Example:
- 8 services × 2 instances × 20 client connections = 320 clients
- PgBouncer pools down to 25 actual PostgreSQL connections
- PostgreSQL sees only 25 connections (well within 200 limit)
```

---

## HPA Scaling Thresholds

### API Gateway HPA

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-gateway-hpa
  namespace: all-chat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
  minReplicas: 2
  maxReplicas: 20                       # Increased from 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60        # Scale up at 60% CPU
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 70        # Scale up at 70% memory
    - type: Pods
      pods:
        metric:
          name: websocket_connections_active
        target:
          type: AverageValue
          averageValue: "2000"          # Scale before hitting 2,500 limit

  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60    # Wait 60s before scaling up
      policies:
        - type: Percent
          value: 100                    # Double pods if needed
          periodSeconds: 60
        - type: Pods
          value: 2                      # Or add 2 pods
          periodSeconds: 60
      selectPolicy: Max                 # Use faster policy

    scaleDown:
      stabilizationWindowSeconds: 300   # Wait 5min before scaling down
      policies:
        - type: Pods
          value: 1                      # Remove 1 pod at a time
          periodSeconds: 60
      selectPolicy: Min
```

### Message Processor HPA

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: message-processor-hpa
  namespace: all-chat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: message-processor
  minReplicas: 3                        # Always 3 for consumer group
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
    - type: Pods
      pods:
        metric:
          name: redis_stream_pending_messages
        target:
          type: AverageValue
          averageValue: "1000"          # Scale if >1000 pending per instance
```

---

## Monitoring Dashboards

### Required Grafana Dashboards

#### 1. System Overview Dashboard

**Panels**:
- Service Health (up/down status)
- Request Rate (req/s per service)
- Error Rate (% per service)
- Latency (p50, p95, p99)
- Active WebSocket Connections
- CPU/Memory Usage (all pods)

**Queries**:
```promql
# Service health
up{namespace="all-chat"}

# Request rate
sum(rate(http_requests_total{namespace="all-chat"}[5m])) by (service)

# Error rate
sum(rate(http_requests_total{status=~"5..",namespace="all-chat"}[5m]))
/
sum(rate(http_requests_total{namespace="all-chat"}[5m]))

# p95 latency
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
)
```

#### 2. Message Pipeline Dashboard

**Panels**:
- Message Ingestion Rate (msg/s per platform)
- Message Processing Rate (msg/s)
- Processing Latency (p50, p95, p99)
- Stream Backlog Size
- Emote Cache Hit Rate
- Platform Listener Connections

**Queries**:
```promql
# Ingestion rate
sum(rate(platform_listener_messages_received_total[5m])) by (platform)

# Processing rate
sum(rate(message_processor_messages_processed_total[5m]))

# Stream backlog
redis_stream_length{stream="raw-messages"}

# Cache hit rate
sum(rate(emote_service_cache_hits_total[5m]))
/
(sum(rate(emote_service_cache_hits_total[5m])) + sum(rate(emote_service_cache_misses_total[5m])))
```

#### 3. Database Dashboard

**Panels**:
- CNPG Cluster Status (primary/replica ready)
- Active Connections
- Connection Pool Utilization
- Query Latency (p95)
- Slow Queries Count
- Replication Lag

**Queries**:
```promql
# CNPG status
cnpg_pg_cluster_instances_ready

# Active connections
sum(database_connections_active) by (service)

# Pool utilization
(database_connections_active / database_connections_max) * 100

# Replication lag
cnpg_pg_replication_lag_seconds
```

#### 4. Redis Dashboard

**Panels**:
- Memory Usage
- Operations/Second
- Stream Length (raw-messages)
- Pub/Sub Channels Active
- Cache Hit Rate
- AOF Last Write Status

#### 5. YouTube API Quota Dashboard

**Panels**:
- Quota Used Today (gauge)
- Quota Remaining (10,000 - used)
- Hourly Quota Burn Rate
- Active Streams Count
- Average Polling Interval

**Queries**:
```promql
# Quota used
youtube_listener_api_quota_used

# Quota remaining
10000 - youtube_listener_api_quota_used

# Burn rate (units per hour)
rate(youtube_listener_api_quota_used[1h]) * 3600

# Projected quota exhaustion (hours remaining)
(10000 - youtube_listener_api_quota_used) / rate(youtube_listener_api_quota_used[1h])
```

---

## Loki Log Queries (LogQL)

### Useful Queries for Troubleshooting

```logql
# All errors in last hour
{namespace="all-chat"} |= "level=error" | json

# Errors from specific service
{namespace="all-chat", app="message-processor"} |= "level=error" | json

# Slow operations (duration > 1s)
{namespace="all-chat"} | json | duration_ms > 1000

# Failed database operations
{namespace="all-chat"} |~ "database.*error" | json

# YouTube quota warnings
{app="youtube-listener"} |= "quota" | json

# WebSocket connection rejections
{app="api-gateway"} |= "connection rejected" | json

# Redis connection errors
{namespace="all-chat"} |~ "redis.*connection.*failed" | json
```

---

## Summary

This document provides comprehensive limits and monitoring thresholds:

### Resource Limits
- ✅ Per-pod CPU/memory limits
- ✅ Connection pool sizes (PostgreSQL, Redis)
- ✅ WebSocket connection limits (2,500/pod, 1,000/overlay)

### Alert Thresholds
- ✅ Critical alerts (PagerDuty)
- ✅ Warning alerts (Slack)
- ✅ Info alerts (Slack)

### Monitoring
- ✅ 5 Grafana dashboards
- ✅ Prometheus queries
- ✅ Loki LogQL queries

### Critical Issues
- 🚨 YouTube API quota is MAJOR bottleneck (request increase NOW)
- 🚨 WebSocket capacity requires connection limits in code
- ✅ Redis AOF prevents data loss
- ✅ CNPG provides automated failover

**Next Step**: Implement these limits in code and deploy monitoring stack

---

**Document Maintainers**: SRE Team
**Last Updated**: 2025-11-11
**Review Frequency**: After each phase deployment
