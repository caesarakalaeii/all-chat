# All-Chat: Observability & Monitoring

**Version**: 2.0 (Consolidated)
**Last Updated**: 2026-01-28
**Status**: Production Ready

---

## Table of Contents

1. [Introduction](#introduction)
2. [Architecture Overview](#architecture-overview)
3. [Metrics (Prometheus)](#metrics-prometheus)
4. [Logging (Loki)](#logging-loki)
5. [Tracing (OpenTelemetry)](#tracing-opentelemetry)
6. [Alerting](#alerting)
7. [Dashboards (Grafana)](#dashboards-grafana)
8. [Resource Limits](#resource-limits)
9. [SLIs, SLOs, SLAs](#slis-slos-slas)

---

## Introduction

All-Chat implements comprehensive observability across three pillars:
- **Metrics**: Time-series data (Prometheus + Mimir)
- **Logs**: Structured event data (Zap → Loki)
- **Traces**: Request flow tracking (OpenTelemetry planned)

### Observability Goals

| Goal | Target | Status |
|------|--------|--------|
| **Mean Time to Detect (MTTD)** | < 2 minutes | ✅ Alerts deployed |
| **Mean Time to Resolve (MTTR)** | < 15 minutes | ✅ Runbooks available |
| **Service Availability** | 99.9% (3 nines) | ✅ Measuring |
| **Metric Collection** | 100% of services | ✅ Complete |
| **Log Aggregation** | 100% of services | ✅ Structured logging |

---

## Architecture Overview

### LGTM Stack (Grafana Labs)

| Component | Purpose | Status |
|-----------|---------|--------|
| **Loki** | Log aggregation & search | ✅ Deployed |
| **Grafana** | Unified dashboards | ✅ Deployed |
| **Prometheus** | Short-term metrics (15-day) | ✅ Deployed |
| **Mimir** | Long-term metrics (90-day) | ⏳ Phase 2 |
| **Tempo** | Distributed tracing | ⏳ Phase 2 |
| **Alertmanager** | Alert routing | ✅ Deployed |
| **Promtail** | DaemonSet log scraper | ✅ Deployed |

### Data Flow

```
Application Services
  ├─ Metrics → Prometheus (/metrics endpoints)
  ├─ Logs → Promtail (stdout/stderr) → Loki
  └─ Traces → OpenTelemetry Collector (planned) → Tempo
           ↓
       Grafana (unified visualization)
           ↓
       Alertmanager (alert routing)
           ↓
   On-Call Engineer (Slack/Email/PagerDuty)
```

**Storage**:
- **Phase 1 (Current)**: Local persistent volumes on Hetzner VPS
- **Phase 2+ (Production)**: Hetzner Object Storage / S3-compatible

---

## Metrics (Prometheus)

### Metric Naming Convention

```
<service>_<subsystem>_<metric>_<unit>

Examples:
- api_gateway_http_requests_total
- message_processor_messages_processed_total
- listener_connection_status
```

### Critical Metrics

#### Service Health

```promql
# Connection status (1=connected, 0=disconnected)
listener_connection_status{platform="twitch"}
listener_connection_status{platform="youtube"}
listener_connection_status{platform="kick"}

# Connection attempts
rate(listener_connection_attempts_total{result="success"}[5m])
rate(listener_connection_attempts_total{result="failed"}[5m])
```

#### Message Throughput

```promql
# Messages per second by platform
rate(listener_messages_received_total[5m])

# Messages consumed by processor
rate(processor_messages_consumed_total[5m])

# Messages delivered to overlays
rate(gateway_messages_sent_total[5m])
```

#### Active Resources

```promql
# Active chat channels/streams
listener_active_sources_total{platform="twitch"}
listener_active_sources_total{platform="youtube"}
listener_active_sources_total{platform="kick"}

# Active overlay viewers
gateway_websocket_connections_active

# Active overlays
gateway_overlay_subscriptions_active
```

#### YouTube Quota (Critical!)

```promql
# Remaining quota units
listener_quota_remaining{platform="youtube"}

# Usage percentage (0-100)
listener_quota_usage_percentage{platform="youtube"}

# Rate limit warnings
listener_rate_limit_hits_total{limit_type="api_quota_warning"}    # 80%
listener_rate_limit_hits_total{limit_type="api_quota_critical"}   # 90%
```

#### Performance

```promql
# Message processing latency (P50, P95, P99)
histogram_quantile(0.50, rate(processor_stage_duration_seconds_bucket[5m]))
histogram_quantile(0.95, rate(processor_stage_duration_seconds_bucket[5m]))
histogram_quantile(0.99, rate(processor_stage_duration_seconds_bucket[5m]))

# End-to-end message latency (listener → overlay)
histogram_quantile(0.95, rate(processor_message_duration_seconds_bucket[5m]))
```

#### Error Rates

```promql
# Error rate by service
rate(listener_errors_total[5m])
rate(processor_errors_total[5m])
rate(gateway_errors_total[5m])

# Error types
sum by (error_type) (rate(listener_errors_total[5m]))
```

### Service-Specific Metrics

#### API Gateway

```go
// WebSocket Metrics
api_gateway_websocket_connections_active           // Current active connections
api_gateway_websocket_connections_rejected_total   // Rejected (at capacity)
api_gateway_overlay_subscriptions_active           // Active overlay subscriptions
api_gateway_messages_sent_total                    // Messages broadcast to clients

// HTTP Metrics
api_gateway_http_requests_total{method,path,status}
api_gateway_http_request_duration_seconds{method,path}
```

#### Twitch Listener

```go
listener_connection_status{platform="twitch"}              // 1=connected, 0=disconnected
listener_messages_received_total{platform="twitch"}        // Messages from IRC
listener_messages_published_total{platform="twitch"}       // Published to Redis
listener_active_sources_total{platform="twitch"}           // Active channels
listener_source_events_total{event="added|removed"}       // Channel lifecycle
```

#### YouTube Listener

```go
listener_connection_status{platform="youtube"}            // Polling active
listener_quota_remaining{platform="youtube"}              // Remaining quota units
listener_quota_usage_percentage{platform="youtube"}       // Usage % (0-100)
listener_api_calls_total{operation="search|videos|chat"} // API call counts
listener_rate_limit_hits_total{limit_type}               // Quota warnings
```

### InnerTube Listener Metrics

#### Message Rate Tracking

**Metric:** `youtube_listener_messages_published_total` (Counter)
**Labels:** `service`, `channel_id`
**Purpose:** Track per-channel message throughput

**Use Cases:**
- Identify stuck channels (rate = 0 for 5+ minutes)
- Compare InnerTube vs official listener message rates (canary validation)
- Capacity planning (messages/sec by channel)

**PromQL Pattern (1-minute rolling average):**
```promql
rate(youtube_listener_messages_published_total{channel_id="XXX"}[1m])
```

**Why Counter not Gauge:** Prometheus best practice for rates. Counter tracks cumulative count, `rate()` calculates derivative server-side. Gauges require client-side rate calculation and are less accurate for rolling averages.

#### Error Classification

**Metric:** `youtube_listener_errors_total` (Counter)
**Labels:** `service`, `error_type`
**Error Types:**
- `network`: DNS, connection, timeout, TLS errors
- `http`: 4xx, 5xx HTTP status codes
- `parse`: JSON unmarshaling failures
- `rate_limit`: 429 rate limiting
- `redis`: Redis publish failures

**Diagnostic Workflow:**
1. High error rate alert triggers
2. Check error breakdown: `sum by (error_type) (rate(youtube_listener_errors_total[5m]))`
3. If `network` errors: Check DNS, connectivity, firewall
4. If `http` errors: Check InnerTube API status, credentials
5. If `parse` errors: Check for InnerTube schema changes
6. If `rate_limit` errors: Reduce poll frequency, add backoff
7. If `redis` errors: Check Redis health, connection pool

#### Deletion Buffer Observability

**Metric:** `youtube_listener_deletion_buffer_overflows_total` (Counter)
**Labels:** `service`, `channel_id`
**Purpose:** Track deletion event buffer overflows (max 1000 events, FIFO drop)

**Alert Threshold:** `rate(youtube_listener_deletion_buffer_overflows_total[5m]) > 0`

**Overflow indicates:**
- Mass ban/timeout event (>1000 deletions in 500ms window)
- Possible spam attack or moderation action
- Buffer may need size increase for high-volume channels

**Mitigation:**
- Increase `BATCH_DELETION_THRESHOLD` to reduce event granularity
- Investigate channel for spam patterns
- Consider dynamic buffer sizing in future (currently fixed 1000)

#### Message Processor

```go
processor_messages_consumed_total                         // From Redis Stream
processor_messages_processed_total{result="success|error"}
processor_stage_duration_seconds{stage}                   // Per-stage latency
processor_message_duration_seconds                        // End-to-end latency
processor_emote_cache_hits_total                          // Emote cache efficiency
processor_emote_cache_misses_total
```

### Infrastructure Metrics

#### PostgreSQL (via CNPG Exporter)

```promql
cnpg_pg_stat_database_xact_commit_total               # Transactions committed
cnpg_pg_stat_database_xact_rollback_total             # Transactions rolled back
cnpg_pg_stat_database_tup_inserted_total              # Rows inserted
cnpg_pg_replication_lag_seconds                       # Replication lag
cnpg_pg_stat_database_conflicts_total                 # Conflicts (replicas)
```

#### Redis

```promql
redis_connected_clients                                # Active connections
redis_used_memory_bytes                                # Memory usage
redis_keyspace_hits_total                              # Cache hits
redis_keyspace_misses_total                            # Cache misses
redis_commands_processed_total                         # Commands executed
rate(redis_commands_duration_seconds_sum[5m])          # Avg command duration
```

### Deployment Configuration

All services expose `/metrics` endpoint for Prometheus scraping:

```yaml
# Kubernetes pod annotations
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "<service-port>"
    prometheus.io/path: "/metrics"
```

**Scrape Configuration** (`monitoring/kube-prometheus-values.yaml`):
- Scrape interval: 30 seconds
- Scrape timeout: 10 seconds
- Retention: 15 days (Prometheus), 90 days (Mimir)

---

## Logging (Loki)

### Structured Logging (Zap)

All services use **Zap** for structured JSON logging:

```go
// shared/logger/logger.go
logger := zap.NewProduction()
logger.Info("Message published",
    zap.String("platform", "twitch"),
    zap.String("channel", "xqc"),
    zap.Int("message_count", 42),
)
```

**Output**:
```json
{
  "level": "info",
  "ts": "2026-01-28T10:00:00.000Z",
  "service": "twitch-listener",
  "msg": "Message published",
  "platform": "twitch",
  "channel": "xqc",
  "message_count": 42
}
```

### Log Levels

| Level | Usage | Retention |
|-------|-------|-----------|
| **DEBUG** | Development only | 7 days |
| **INFO** | Normal operations | 30 days |
| **WARN** | Potential issues | 30 days |
| **ERROR** | Errors requiring attention | 90 days |
| **FATAL** | Critical failures | 90 days |

### Log Collection

**Promtail** DaemonSet scrapes all pod stdout/stderr:

```yaml
# Kubernetes labels automatically added
{
  namespace="allchat",
  pod="twitch-listener-abc123",
  service="twitch-listener"
}
```

### Useful LogQL Queries

#### Find Errors in Last Hour
```logql
{namespace="allchat"} |= "level\":\"error\"" | json
```

#### YouTube Quota Exhaustion
```logql
{service="youtube-listener"} |= "quota" |= "EXHAUSTED" | json
```

#### WebSocket Connection Drops
```logql
{service="api-gateway"} |= "websocket" |= "disconnect" | json
```

#### Message Processing Errors
```logql
{service="message-processor"} |= "error" | json | error_type != ""
```

#### Rate Limit Hits
```logql
{namespace="allchat"} |= "rate limit" | json | count_over_time([5m])
```

### Log Aggregation Pipeline

```
Application Pod
  └─ stdout/stderr (JSON logs)
       ↓
    Promtail (DaemonSet)
       ↓ scrape & label
    Loki (aggregation)
       ↓ query via LogQL
    Grafana (visualization)
```

---

## Tracing (OpenTelemetry)

**Status**: ⏳ Planned for Phase 2

### Architecture (Planned)

```
Application Services
  └─ OpenTelemetry SDK
       ↓ export traces
    OpenTelemetry Collector
       ↓ process & route
    Tempo (storage)
       ↓ query
    Grafana (visualization)
```

### Trace Spans (Planned)

**Message Processing Pipeline**:
1. `listener.receive` - Message received from platform
2. `redis.publish` - Published to chat:raw stream
3. `processor.consume` - Consumed from stream
4. `processor.normalize` - Platform → unified format
5. `processor.enrich` - Emote enrichment
6. `redis.publish.pubsub` - Published to overlay channel
7. `gateway.broadcast` - Sent to WebSocket clients

**Target**: P95 latency <500ms from listener → overlay

---

## Alerting

### Critical Alerts (PagerDuty)

#### Service Down
```yaml
alert: ServiceDown
expr: up{job=~"twitch-listener|youtube-listener|message-processor|api-gateway"} == 0
for: 2m
severity: critical
```

#### YouTube Quota Critical
```yaml
alert: YouTubeQuotaCritical
expr: listener_quota_usage_percentage{platform="youtube"} > 90
for: 5m
severity: critical
```

#### WebSocket Connection Surge
```yaml
alert: WebSocketCapacity
expr: api_gateway_websocket_connections_active > 2000
for: 5m
severity: warning
```

#### Message Processing Lag
```yaml
alert: MessageProcessingLag
expr: rate(listener_messages_received_total[5m]) > rate(processor_messages_consumed_total[5m]) * 1.2
for: 5m
severity: warning
```

### Warning Alerts (Slack)

#### High Error Rate
```yaml
alert: HighErrorRate
expr: rate(listener_errors_total[5m]) > 0.05
for: 5m
severity: warning
```

#### Database Connection Pool Saturation
```yaml
alert: DBPoolSaturated
expr: pgx_pool_acquired_conns / pgx_pool_max_conns > 0.8
for: 5m
severity: warning
```

#### Redis Memory High
```yaml
alert: RedisMemoryHigh
expr: redis_used_memory_bytes / redis_memory_limit_bytes > 0.8
for: 10m
severity: warning
```

### Alert Routing (Alertmanager)

```yaml
route:
  receiver: slack-default
  routes:
    - match:
        severity: critical
      receiver: pagerduty
    - match:
        severity: warning
      receiver: slack-warnings
    - match:
        service: youtube-listener
      receiver: slack-youtube
```

---

## Dashboards (Grafana)

### Dashboard Organization

1. **Overview Dashboard** - System health at a glance
2. **Service Dashboards** - Per-service metrics (Twitch, YouTube, Kick, Processor, Gateway)
3. **Infrastructure Dashboard** - PostgreSQL, Redis, Kubernetes
4. **Business Intelligence** - Active overlays, viewers, message rates
5. **YouTube Quota Dashboard** - Critical quota tracking

### Key Dashboard Panels

#### Overview Dashboard

- Service status (up/down indicators)
- Total messages/second across all platforms
- Active WebSocket connections
- YouTube quota gauge
- Error rate heatmap
- P95 message latency

#### Twitch Listener Dashboard

- IRC connection status
- Active channels (time series)
- Messages received/published rate
- Channel lifecycle events (JOIN/PART)
- Connection duration
- Error breakdown by type

#### YouTube Listener Dashboard

- **Quota usage gauge** (0-100%, green/yellow/red)
- Quota remaining (absolute units)
- API call breakdown (search/videos/chat)
- Active streams
- Polling errors
- OAuth token refresh status

#### Message Processor Dashboard

- Messages consumed vs published
- Processing stages duration (P50/P95/P99)
- Emote cache hit rate
- Error rate by stage
- Consumer lag (XPENDING)

#### API Gateway Dashboard

- Active WebSocket connections
- Connection rate (new/closed)
- Messages broadcast/second
- Overlay subscriptions
- WebSocket errors
- HTTP request latency

---

## Resource Limits

### Kubernetes Resource Limits (Per Pod)

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit |
|---------|-------------|-----------|----------------|--------------|
| **API Gateway** | 100m | 500m | 128Mi | 512Mi |
| **Message Processor** | 200m | 1000m | 256Mi | 1Gi |
| **Twitch Listener** | 100m | 500m | 128Mi | 512Mi |
| **YouTube Listener** | 100m | 500m | 128Mi | 512Mi |
| **Kick Listener** | 100m | 500m | 128Mi | 512Mi |
| **Auth Service** | 50m | 200m | 64Mi | 256Mi |
| **Overlay Manager** | 50m | 200m | 64Mi | 256Mi |
| **Emote Service** | 50m | 200m | 64Mi | 256Mi |
| **Source Manager** | 50m | 200m | 64Mi | 256Mi |

### Connection Limits

#### API Gateway WebSocket

```go
const (
    MaxConnectionsPerPod     = 2500   // Per pod (512Mi memory)
    MaxConnectionsPerOverlay = 1000   // Prevent monopolization
    MaxMessageSize           = 64KB   // Prevent abuse
    WriteTimeout             = 10s
    ReadTimeout              = 60s
    PingInterval             = 30s
)
```

#### PostgreSQL (pgx)

```go
MaxConns         = 20                 // Per service instance
MinConns         = 5                  // Keep warm connections
MaxConnLifetime  = 1 hour             // Recycle connections
MaxConnIdleTime  = 10 minutes         // Close idle
HealthCheckPeriod = 1 minute          // Verify connections
```

#### Redis

```go
PoolSize        = 50                  // Per service instance
MinIdleConns    = 10                  // Keep warm
ConnMaxIdleTime = 10 minutes
DialTimeout     = 5 seconds
ReadTimeout     = 3 seconds
WriteTimeout    = 3 seconds
```

### Rate Limits

#### YouTube API Quota

- **Daily Limit**: 10,000 units (request increase to 1M)
- **Reserve-Confirm-Rollback**: Atomic quota tracking (99.95%+ accuracy)
- **State Machine**: HEALTHY (0-70%) → DEGRADED (70-85%) → CRITICAL (85-95%) → EXHAUSTED (95-100%) → DEPLETED (>100%)
- **Expected Usage**: 2,000-3,000 units/day

#### Twitch IRC

- **JOIN Rate**: 20 channels per 10 seconds
- **Message Rate**: 100 messages per 30 seconds (not enforced for listeners)

#### API Gateway

- **Connection Rate**: 100 new connections per second per pod
- **Message Rate**: 10,000 messages per second per pod

---

## SLIs, SLOs, SLAs

### Service Level Indicators (SLIs)

| SLI | Measurement | Target |
|-----|-------------|--------|
| **Availability** | `up{service=~".*-listener|processor|gateway"} == 1` | 99.9% |
| **Latency (P95)** | `histogram_quantile(0.95, processor_message_duration_seconds)` | <500ms |
| **Error Rate** | `rate(listener_errors_total[5m]) / rate(listener_messages_received_total[5m])` | <1% |
| **Message Delivery** | `rate(gateway_messages_sent_total[5m]) / rate(listener_messages_received_total[5m])` | >99% |

### Service Level Objectives (SLOs)

| Service | Availability | Latency (P95) | Error Budget |
|---------|--------------|---------------|--------------|
| **API Gateway** | 99.9% | <100ms (HTTP), <500ms (WebSocket) | 43 min/month |
| **Message Processor** | 99.9% | <500ms | 43 min/month |
| **Listeners** (Twitch/YouTube/Kick) | 99.5% | <100ms | 3.6 hours/month |

### Service Level Agreements (SLAs)

**Internal SLA** (between services):
- **Availability**: 99.5% uptime (excluding planned maintenance)
- **Latency**: P95 <500ms for message processing pipeline
- **Error Rate**: <1% message processing errors

**User-Facing SLA** (overlay viewers):
- **Availability**: 99.9% uptime
- **Latency**: <2 seconds from chat message to overlay display
- **Support Response**: <24 hours for critical issues

---

## Quick Reference

### Health Check Endpoints

```bash
# All services expose health checks
curl http://<service>:<port>/health/live    # Liveness
curl http://<service>:<port>/health/ready   # Readiness
curl http://<service>:<port>/status         # Detailed status
curl http://<service>:<port>/metrics        # Prometheus metrics
```

### Grafana Dashboards

- **URL**: `http://grafana.allchat.local` (or `http://<external-ip>:3000`)
- **Default Credentials**: `admin` / `<generated-password>`
- **Dashboards**: Dashboards → Manage → All-Chat folder

### Loki Queries (LogQL)

```bash
# Access Grafana → Explore → Loki data source

# Recent errors
{namespace="allchat"} |= "level\":\"error\"" | json

# Specific service logs
{service="youtube-listener"} | json

# Count errors over time
sum by (service) (count_over_time({namespace="allchat"} |= "error" [5m]))
```

### Prometheus Queries (PromQL)

```bash
# Access Grafana → Explore → Prometheus data source

# Service health
up{namespace="allchat"}

# Message rate
rate(listener_messages_received_total[5m])

# YouTube quota
listener_quota_usage_percentage{platform="youtube"}
```

---

## Related Documentation

- **[Deployment Guide](./02-DEPLOYMENT.md)** - Kubernetes deployment and configuration
- **[Scaling Guide](./03-SCALING.md)** - HPA, resource scaling, bottlenecks
- **[Operations Guide](../operations/OBSERVABILITY_DEPLOYMENT_GUIDE.md)** - Deploy Prometheus, Loki, Grafana
- **[Troubleshooting Decision Tree](../troubleshooting/decision-tree.md)** - Diagnose issues using metrics/logs

---

## Summary

**Status**: ✅ Production Ready

- ✅ All services expose `/metrics` endpoints
- ✅ Structured logging (Zap) across all services
- ✅ Prometheus scraping configured (30s interval)
- ✅ Loki log aggregation deployed
- ✅ Grafana dashboards available
- ✅ Critical alerts configured (PagerDuty + Slack)
- ⏳ OpenTelemetry tracing (Phase 2)

**Total Observability Coverage**: 100% of critical services monitored, logged, and alerted.
