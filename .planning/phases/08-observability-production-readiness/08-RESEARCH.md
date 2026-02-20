# Phase 8: Observability & Production Readiness - Research

**Researched:** 2026-02-20
**Domain:** Distributed systems observability (Prometheus metrics, OpenTelemetry tracing, Grafana visualization, Prometheus alerting)
**Confidence:** HIGH

## Summary

Phase 8 adds comprehensive observability for the distributed sharding system built in Phases 5-7. The codebase already has strong observability foundations: Prometheus metrics package (`shared/metrics/shard_metrics.go`), OpenTelemetry tracing infrastructure (`shared/tracing/`), and existing monitoring deployments (`deployments/k8s/monitoring/`). Research validates that the existing stack (Prometheus + Grafana + OpenTelemetry + Jaeger) is production-ready and follows 2026 best practices.

Key findings:
- **Metrics foundation exists**: `ShardMetrics` already includes rebalancing counters, load imbalance ratio gauge, and per-pod load scores. Migration metrics (success/failure counters) need to be added.
- **Tracing infrastructure ready**: OpenTelemetry SDK configured with OTLP gRPC exporter, W3C Trace Context propagation, and Gin middleware. Currently uses `AlwaysSample()` - needs environment-configurable sampling per CONTEXT.md requirements.
- **Migration events logged to Redis Streams**: `migration:log` stream already captures migration events and confirmations with migration_id, channel_id, pod_id, status, timestamp. This provides trace context anchor points.
- **Prometheus deployment exists**: `/metrics` endpoints on all services, AlertManager configured, basic alerts defined in `deployments/k8s/monitoring/alerts/`.

**Primary recommendation:** Extend existing infrastructure rather than rebuild. Add missing metrics (channel count per pod, migration counters with outcome labels, hot channel tracking), instrument migration/rebalancing operations with OpenTelemetry spans, create Grafana dashboard JSON with heatmap + timeline panels, and add production-ready Prometheus alerts.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Dashboard layout & visualizations:**
- Single comprehensive dashboard (not multiple focused dashboards)
- Two heatmap views for channel distribution:
  - Pod (Y) × Platform (X) - shows platform-specific distribution
  - Pod (Y) × Time (X) - shows rebalancing effects over time
- Rebalancing timeline uses both approaches:
  - Event markers/annotations on time series graphs for correlation
  - Dedicated event timeline panel with details (channels moved, reason, duration)

**Alert design & thresholds:**
- Imbalance ratio alert threshold: 0.7 for 10 minutes (keep as specified in requirements)
- Alert severity based on impact to users:
  - Critical = message loss or user-visible failures
  - Warning = degraded performance or suboptimal state
- Alerts include remediation steps in descriptions (suggested actions, not just notifications)
- Rebalancing thrashing alert: Relaxed to >5 rebalances in 30 minutes (was >3 in 15min per requirements)

**Tracing granularity & context:**
- Migration traces use medium detail: 8-10 spans per migration
  - Include sub-operations (Redis pub, listener actions, confirmation wait)
  - Balance between overview and deep debugging
- Traces include all data: channel IDs, usernames, message counts, error details
  - Full debugging capability prioritized over privacy restrictions
- Trace context propagation: W3C Trace Context headers (traceparent/tracestate)
  - Standard-compliant propagation through Redis Streams messages
- Sampling strategy: Environment-configurable with error always-on
  - OTEL_SAMPLING_RATE env var (0.0-1.0) for normal operations
  - Always trace errors/failures regardless of sampling rate
  - Start at 1.0 for initial weeks, reduce to 0.1 for steady state

**Metrics cardinality & labels:**
- Channel count metric labels: Pod name only (not service/platform - implicit in pod name)
- Channel-level metrics: Top-N hot channels only (10-20 highest-volume)
  - Avoids Prometheus cardinality explosion
  - Still identifies problematic hot channels
- Migration/rebalancing event metrics: Count with outcome labels
  - status=success/failure
  - reason=load/hpa/manual/etc
  - Enables failure pattern analysis

### Claude's Discretion

- Primary dashboard at-a-glance view (imbalance ratio, load distribution, or recent events)
- Coordinator-specific metrics (separate namespace vs shared vs minimal subset)
- Exact span breakdown for medium-detail traces (8-10 spans structure)
- Top-N threshold for hot channel metrics (10, 15, or 20)

### Deferred Ideas (OUT OF SCOPE)

None - discussion stayed within phase scope

</user_constraints>

---

<phase_requirements>
## Phase Requirements

This phase addresses requirements METRICS-01 through METRICS-11 and TRACE-01 through TRACE-06 from REQUIREMENTS.md.

| ID | Description | Research Support |
|----|-------------|-----------------|
| METRICS-01 | Each listener pod exposes Prometheus metrics at /metrics endpoint | All services already expose /metrics (verified in shared/metrics/README.md) - need to verify all listener deployments include Prometheus annotations |
| METRICS-02 | System tracks per-pod channel count as Gauge metric (shard_channel_count) | Need to add metric - existing ShardMetrics has pod load tracking but not explicit channel count gauge per pod |
| METRICS-03 | System tracks per-pod message rate as Counter metric (shard_messages_total) | Existing listener metrics track messages_received_total and messages_published_total - need pod_id label |
| METRICS-04 | System tracks rebalancing events as Counter metric (shard_rebalancing_total) | Already exists in shared/metrics/shard_metrics.go line 107-110 |
| METRICS-05 | System tracks migration success/failure as Counter metrics | Need to add shard_migration_success_total and shard_migration_failure_total with outcome labels |
| METRICS-06 | System tracks load imbalance ratio as Gauge metric (shard_imbalance_ratio) | Already exists in shared/metrics/shard_metrics.go line 81-84 |
| METRICS-07 | Grafana dashboard visualizes channel distribution across pods (heatmap) | Grafana heatmap panel supports time-series data with color intensity for value magnitude - need to create dashboard JSON |
| METRICS-08 | Grafana dashboard shows rebalancing timeline and migration events | Grafana annotations from Prometheus ALERTS metric + dedicated timeline panel using migration:log Redis Stream data |
| METRICS-09 | Prometheus alert triggers when imbalance ratio >0.7 for 10 minutes | Add PrometheusRule to deployments/k8s/monitoring/alerts/ - threshold and duration locked per CONTEXT.md |
| METRICS-10 | Prometheus alert triggers on split-brain detection (multiple leaders) | Add alert on sum(shard_coordinator_is_leader) > 1 - metric already exists line 91-94 |
| METRICS-11 | Prometheus alert triggers on rebalancing thrashing (>5 rebalances in 30min) | Add alert on rate(shard_rebalancing_total[30m]) > 5 - relaxed threshold per CONTEXT.md |
| TRACE-01 | System instruments channel assignment operations with OpenTelemetry spans | OpenTelemetry SDK ready in shared/tracing/ - need to instrument coordination/assigner.go |
| TRACE-02 | System instruments migration operations with OpenTelemetry spans | Migration events published to Redis Streams provide trace anchor - need to add spans to coordination/migration_publisher.go |
| TRACE-03 | System instruments rebalancing operations with OpenTelemetry spans | Need to add spans to coordination/rebalancer.go PlanRebalancing and ExecuteRebalancing |
| TRACE-04 | System propagates trace context through Redis Streams messages | W3C Trace Context propagation via traceparent header in Redis Stream message fields - use otel.GetTextMapPropagator().Inject() |
| TRACE-05 | Jaeger UI shows end-to-end trace for channel migration (all phases) | 8-10 span structure: coordinator decision, Redis pub, listener receive, platform connect, confirmation wait, cleanup |
| TRACE-06 | Jaeger UI shows trace for rebalancing decision (trigger → completion) | Spans: load monitoring, imbalance detection, plan creation, migration execution, cooldown enforcement |

</phase_requirements>

---

## Standard Stack

### Core Observability Stack

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| prometheus/client_golang | v1.23.2 | Prometheus metrics SDK for Go | Official Prometheus client, already used across all services (verified in shared/go.mod) |
| go.opentelemetry.io/otel | v1.40.0 | OpenTelemetry API and SDK | Official OTel Go implementation, already integrated in shared/tracing/ |
| go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc | v1.40.0 | OTLP gRPC trace exporter | Standard exporter for Jaeger/Tempo, already configured in shared/tracing/tracer.go |
| go.opentelemetry.io/otel/sdk/trace | v1.40.0 | Trace SDK with sampling | Provides TracerProvider and sampling configuration |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/redis/go-redis/extra/redisotel/v9 | v9.17.3 | Redis instrumentation for OpenTelemetry | Automatic tracing for Redis operations (already in shared/go.mod) |
| github.com/exaring/otelpgx | v0.10.0 | PostgreSQL instrumentation for OpenTelemetry | Automatic tracing for database queries (already in shared/go.mod) |
| grafana/grafana | 11.x+ | Dashboard and visualization platform | Already deployed in deployments/k8s/monitoring/ |
| prometheus/prometheus | 2.x+ | Metrics collection and storage | Already deployed with 30s scrape interval (verified in docs/architecture/04-OBSERVABILITY.md) |
| jaegertracing/all-in-one | latest | Distributed tracing backend | Compatible with OpenTelemetry OTLP exporter |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Jaeger | Grafana Tempo | Tempo integrates better with Grafana but Jaeger has more mature UI and query capabilities |
| PrometheusRule CRDs | Grafana Alerting | Grafana Alerting offers unified alerting but PrometheusRule CRDs already deployed and working |
| Manual heatmap queries | Grafana heatmap plugin (marcusolsson-hourly-heatmap-panel) | Plugin provides hourly bucketing but core heatmap visualization supports time-series data natively |

**Installation:**

Existing infrastructure - no new installations required. All dependencies already in shared/go.mod and deployments/k8s/monitoring/.

---

## Architecture Patterns

### Recommended Metrics Organization

```
shared/metrics/
├── shard_metrics.go        # Sharding/rebalancing metrics (existing)
├── listener.go             # Listener-specific metrics (existing)
├── processor.go            # Message processor metrics (existing)
└── gateway.go              # API Gateway metrics (existing)
```

All listener services register ShardMetrics in main.go and record metrics during operations.

### Pattern 1: Per-Pod Channel Count Metric

**What:** Gauge metric tracking assigned channel count per pod, updated when assignments change
**When to use:** Need to visualize channel distribution across pods in heatmap
**Example:**

```go
// Source: Prometheus best practices + existing ShardMetrics pattern
// Add to shared/metrics/shard_metrics.go

PodChannelCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
    Name: "shard_channel_count",
    Help: "Number of channels assigned to this pod",
}, []string{"pod_id"}),

// Record in listener main.go after QueryAssignments
metrics.PodChannelCount.WithLabelValues(podID).Set(float64(len(assignments)))
```

**Cardinality:** pod_id label only. Platform is implicit in pod name (e.g., twitch-listener-0, kick-listener-1). Avoids cardinality explosion (max ~20 pods × 4 platforms = 80 series).

### Pattern 2: Migration Outcome Metrics

**What:** Counter metrics with outcome and reason labels for migration analysis
**When to use:** Track migration success/failure patterns, diagnose migration issues
**Example:**

```go
// Source: Prometheus label best practices (2026)
// Add to shared/metrics/shard_metrics.go

MigrationTotal: promauto.NewCounterVec(prometheus.CounterOpts{
    Name: "shard_migration_total",
    Help: "Total number of channel migrations",
}, []string{"status", "reason"}),

// Record in coordination/migration_publisher.go
metrics.MigrationTotal.WithLabelValues("success", "rebalance").Inc()
metrics.MigrationTotal.WithLabelValues("failure", "pod_failure").Inc()
```

**Labels:**
- `status`: success, failure
- `reason`: rebalance, pod_failure, scale_up, scale_down, manual

### Pattern 3: Environment-Configurable Trace Sampling

**What:** Use OpenTelemetry environment variables for runtime-configurable sampling
**When to use:** Start with 100% sampling, reduce to 10% for production, ramp up when debugging
**Example:**

```go
// Source: OpenTelemetry specification - Environment Variable Specification
// Replace in shared/tracing/tracer.go

import sdktrace "go.opentelemetry.io/otel/sdk/trace"

// Remove hardcoded AlwaysSample()
// OLD: sdktrace.WithSampler(sdktrace.AlwaysSample())

// NEW: Use environment-based sampling with error always-on
sampler := createConfigurableSampler()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(traceExporter),
    sdktrace.WithResource(res),
    sdktrace.WithSampler(sampler),
)

func createConfigurableSampler() sdktrace.Sampler {
    // Read OTEL_SAMPLING_RATE (0.0-1.0, default 1.0)
    samplingRate := getEnvFloat("OTEL_SAMPLING_RATE", 1.0)

    // Base sampler: TraceIDRatioBased
    baseSampler := sdktrace.TraceIDRatioBased(samplingRate)

    // Always sample errors regardless of rate
    return &alwaysSampleErrorsSampler{
        delegate: baseSampler,
    }
}
```

**Environment variables (per OpenTelemetry spec):**
- `OTEL_TRACES_SAMPLER=traceidratio` (or parentbased_traceidratio)
- `OTEL_TRACES_SAMPLER_ARG=0.1` (10% sampling)
- Custom: `OTEL_SAMPLING_RATE=0.1` (simpler for operators)

### Pattern 4: W3C Trace Context Propagation via Redis Streams

**What:** Inject traceparent header into Redis Stream message fields, extract in consumers
**When to use:** Correlate migration events across coordinator → listener → confirmation
**Example:**

```go
// Source: OpenTelemetry Context Propagation spec + Redis Streams pattern
// In coordination/migration_publisher.go

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
)

func (m *MigrationPublisher) PublishMigrationEvent(ctx context.Context, event *MigrationEvent) error {
    tracer := otel.Tracer("source-manager")
    ctx, span := tracer.Start(ctx, "publish-migration-event")
    defer span.End()

    // Inject trace context into Redis message
    carrier := make(propagation.MapCarrier)
    otel.GetTextMapPropagator().Inject(ctx, carrier)

    // Add to Redis Streams message fields
    err = m.redisClient.XAdd(ctx, &redis.XAddArgs{
        Stream: "migration:log",
        Values: map[string]interface{}{
            "migration_id": event.MigrationID,
            "channel_id":   event.ChannelID,
            // ... other fields ...
            "traceparent":  carrier.Get("traceparent"),  // W3C Trace Context
            "tracestate":   carrier.Get("tracestate"),
        },
    }).Err()

    return err
}

// In listener - extract trace context when consuming
func (l *Listener) handleMigrationEvent(msg map[string]interface{}) {
    carrier := propagation.MapCarrier{
        "traceparent": msg["traceparent"].(string),
        "tracestate":  msg["tracestate"].(string),
    }
    ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

    tracer := otel.Tracer("twitch-listener")
    ctx, span := tracer.Start(ctx, "handle-migration")
    defer span.End()
    // ... migration logic ...
}
```

### Pattern 5: Grafana Heatmap for Channel Distribution

**What:** Two heatmap panels - one for platform distribution, one for time-based rebalancing visualization
**When to use:** At-a-glance view of channel assignment fairness and rebalancing effects
**Example:**

```json
// Source: Grafana Heatmap documentation + Prometheus histogram patterns
// Dashboard panel configuration

{
  "type": "heatmap",
  "title": "Channel Distribution: Pod × Platform",
  "targets": [
    {
      "expr": "sum by (pod_id, platform) (shard_channel_count)",
      "legendFormat": "{{ pod_id }}"
    }
  ],
  "options": {
    "calculate": true,
    "cellGap": 2,
    "color": {
      "scheme": "Spectral"
    },
    "yAxis": {
      "axisLabel": "Pod"
    },
    "xAxis": {
      "axisLabel": "Platform"
    }
  }
}
```

**Second heatmap (time-based):**

```json
{
  "type": "heatmap",
  "title": "Channel Distribution Over Time: Pod × Time",
  "targets": [
    {
      "expr": "shard_channel_count",
      "legendFormat": "{{ pod_id }}"
    }
  ],
  "options": {
    "calculate": true,
    "yAxis": {
      "axisLabel": "Pod ID",
      "decimals": 0
    }
  }
}
```

### Pattern 6: Grafana Annotations from Migration Events

**What:** Query migration:log Redis Stream and render as Grafana annotations on time-series panels
**When to use:** Correlate rebalancing events with metric changes (load spikes, imbalance ratio drops)
**Example:**

```json
// Source: Grafana annotations documentation + Prometheus ALERTS pattern
// Dashboard annotation configuration

{
  "annotations": {
    "list": [
      {
        "name": "Rebalancing Events",
        "datasource": "Prometheus",
        "expr": "ALERTS{alertname=\"RebalancingTriggered\"}",
        "tagKeys": "reason,migration_count",
        "titleFormat": "Rebalancing: {{ reason }}",
        "textFormat": "Migrated {{ migration_count }} channels"
      }
    ]
  }
}
```

**Alternative using Redis Stream exporter (if available):**

Could export migration:log to Prometheus via custom exporter, but simpler approach is to create Prometheus recording rule that fires when rebalancing metrics change, then use ALERTS metric for annotations.

### Pattern 7: Medium-Detail Migration Trace (8-10 Spans)

**What:** Hierarchical span structure capturing migration lifecycle phases
**When to use:** Debug migration failures, measure migration duration, identify bottlenecks
**Example:**

```
migration-operation (parent span - coordinator)
├── plan-migration (coordinator)
│   ├── query-pod-assignments
│   └── calculate-target-pod
├── publish-migration-event (coordinator)
│   ├── redis-pub-notify
│   └── redis-stream-log
├── listener-receive-event (new pod - child of parent via trace propagation)
│   ├── platform-connect (e.g., twitch-join-channel)
│   └── wait-first-message
├── confirm-migration (new pod)
│   └── redis-pub-confirmation
├── listener-disconnect (old pod)
│   └── platform-disconnect (e.g., twitch-part-channel)
└── verify-migration (coordinator)
    └── wait-confirmation-timeout
```

**Span count:** 9 spans (within 8-10 target). Attributes on each span include channel_id, pod_id, platform, migration_id, and operation-specific data (message_count, duration, error).

### Anti-Patterns to Avoid

- **High-cardinality labels:** Don't add channel_id or username as Prometheus labels - use attributes in traces instead. Prometheus cardinality explosions cause query slowdowns and memory issues.
- **Always sampling in production:** Don't leave `AlwaysSample()` in production code - trace volume grows linearly with request rate and overwhelms Jaeger storage.
- **Missing trace context propagation:** Don't start new root spans in listeners - always extract context from Redis messages to maintain parent-child relationships.
- **Alert fatigue:** Don't set alert thresholds too low - use locked values from CONTEXT.md (imbalance >0.7, thrashing >5 in 30min) to avoid false positives.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Metrics collection | Custom metrics HTTP endpoint | prometheus/client_golang with promhttp.Handler() | Handles format negotiation, compression, concurrent scraping, standard /metrics endpoint |
| Trace sampling | Custom probabilistic sampler | sdktrace.TraceIDRatioBased + ParentBased | Handles trace ID hashing, parent sampling decisions, distributed sampling consistency |
| Grafana dashboards | Custom UI or manual JSON editing | Grafana UI with dashboard provisioning | Drag-and-drop panels, variable templating, version control via JSON export, auto-reload |
| Alert routing | Custom webhook handlers | Prometheus Alertmanager | Deduplication, grouping, inhibition, silencing, retry logic, multi-receiver routing |
| Heatmap calculations | Custom aggregation logic | Grafana heatmap panel + Prometheus histogram_quantile | Automatic bucketing, color gradients, time-series alignment, zoom/pan |
| Trace context propagation | Custom header injection | otel.GetTextMapPropagator().Inject/Extract | W3C Trace Context spec compliance, baggage propagation, multi-format support |

**Key insight:** Observability tooling has complex edge cases (scrape failures, trace sampling consistency, alert deduplication) that standard libraries handle. Custom implementations miss these cases and create operational blind spots.

---

## Common Pitfalls

### Pitfall 1: Prometheus Cardinality Explosion

**What goes wrong:** Adding high-cardinality labels (channel_id, username, message_id) creates millions of time series, causing Prometheus to slow down or crash.

**Why it happens:** Each unique label combination creates a new time series. 1000 channels × 20 pods = 20,000 series. Add username (10,000 unique) → 200 million series.

**How to avoid:**
- **LOCKED CONSTRAINT:** Use pod_id label only for channel count metrics. Platform is implicit in pod name.
- **LOCKED CONSTRAINT:** Track top-N hot channels only (10-20 highest-volume) for channel-level metrics.
- Use Prometheus recording rules to pre-aggregate high-cardinality metrics.
- Put high-cardinality data (channel IDs, usernames) in trace span attributes, not metrics labels.

**Warning signs:**
- Prometheus query timeouts or slow dashboard load times
- High memory usage on Prometheus pod (>80% of limit)
- Prometheus logs showing "too many series" warnings

### Pitfall 2: Missing Trace Context Across Async Boundaries

**What goes wrong:** Traces break at Redis Pub/Sub or Streams boundaries, creating orphaned spans that can't be correlated.

**Why it happens:** Go context doesn't automatically propagate through Redis messages. Need explicit inject/extract.

**How to avoid:**
- **LOCKED PATTERN:** Use W3C Trace Context (traceparent/tracestate) in Redis Stream message fields.
- Always extract context before starting child spans in listeners.
- Test trace continuity with Jaeger UI - verify parent-child relationships visible.

**Warning signs:**
- Jaeger UI shows multiple disconnected traces for single migration
- Unable to find listener spans when searching by coordinator trace ID
- Trace search by migration_id returns partial results

### Pitfall 3: AlwaysSample in Production

**What goes wrong:** 100% sampling generates massive trace volume (millions of spans/day), filling Jaeger storage and degrading query performance.

**Why it happens:** Current code uses `sdktrace.AlwaysSample()` (line 80 in shared/tracing/tracer.go) - acceptable for development, catastrophic in production.

**How to avoid:**
- **LOCKED REQUIREMENT:** Environment-configurable sampling with OTEL_SAMPLING_RATE env var.
- **LOCKED REQUIREMENT:** Always trace errors regardless of sampling rate.
- Start at 1.0 (100%) for initial weeks to verify instrumentation, then reduce to 0.1 (10%).

**Warning signs:**
- Jaeger storage growing faster than retention policy can delete
- Jaeger query timeouts or UI slowness
- High network traffic between services and Jaeger collector

### Pitfall 4: Alert Threshold Tuning Too Aggressive

**What goes wrong:** Alerts fire constantly during normal operations, causing alert fatigue and operators ignoring critical alerts.

**Why it happens:** Setting thresholds too low (imbalance >0.3, thrashing >2 rebalances) creates noise from expected variance.

**How to avoid:**
- **LOCKED THRESHOLDS:** Use values from CONTEXT.md - imbalance >0.7 for 10min, thrashing >5 in 30min.
- Include `for: 10m` duration in Prometheus alerts to suppress transient spikes.
- Add remediation steps in alert descriptions - operators need actions, not just notifications.

**Warning signs:**
- PagerDuty fatigue - oncall ignoring alerts
- Alerts firing during known rebalancing operations
- High alert close rate without action taken

### Pitfall 5: Grafana Heatmap Data Format Mismatch

**What goes wrong:** Heatmap panel shows empty or garbled visualization despite metrics existing.

**Why it happens:** Grafana heatmap expects specific data format (histogram buckets or time-series matrix). Simple gauge metrics won't render correctly.

**How to avoid:**
- Use `sum by (pod_id, platform)` aggregation for Pod × Platform heatmap.
- For time-based heatmap (Pod × Time), use raw `shard_channel_count` metric with pod_id label.
- Test with sample data - verify color gradient appears and axes labels correct.

**Warning signs:**
- Heatmap panel shows "No data" despite Prometheus query returning results
- Axes labels missing or incorrect (showing metric name instead of pod ID)
- Color gradient all single color (no variation)

### Pitfall 6: Migration Span Parent-Child Relationships Lost

**What goes wrong:** Migration spans created but not linked to coordinator parent span, making end-to-end trace invisible in Jaeger UI.

**Why it happens:** Listener starts new root span instead of extracting context from migration event message.

**How to avoid:**
- **LOCKED PATTERN:** Extract traceparent from Redis Stream message fields before starting listener spans.
- Use `trace.WithLinks()` if direct parent-child relationship not possible (e.g., asynchronous fan-out).
- Verify in Jaeger UI - search by migration_id attribute, ensure all spans in same trace tree.

**Warning signs:**
- Jaeger search by trace ID only shows coordinator spans, not listener spans
- Migration traces show <8 spans (missing listener operations)
- Unable to measure end-to-end migration duration

---

## Code Examples

Verified patterns from official sources and existing codebase:

### Example 1: Add Missing Migration Metrics

```go
// Source: shared/metrics/shard_metrics.go (existing pattern)
// Add to ShardMetrics struct

type ShardMetrics struct {
    // ... existing fields ...

    // Migration operations (Phase 8)
    MigrationTotal      *prometheus.CounterVec // Total migrations with outcome labels
    MigrationDuration   prometheus.Histogram   // Migration duration distribution
    PodChannelCount     *prometheus.GaugeVec   // Per-pod channel count
}

func NewShardMetrics() *ShardMetrics {
    return &ShardMetrics{
        // ... existing metrics ...

        MigrationTotal: promauto.NewCounterVec(prometheus.CounterOpts{
            Name: "shard_migration_total",
            Help: "Total number of channel migrations",
        }, []string{"status", "reason"}),

        MigrationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "shard_migration_duration_seconds",
            Help:    "Migration duration in seconds",
            Buckets: []float64{1, 5, 10, 30, 60, 120}, // Migration expected <60s
        }),

        PodChannelCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
            Name: "shard_channel_count",
            Help: "Number of channels assigned to this pod",
        }, []string{"pod_id"}),
    }
}
```

### Example 2: Instrument Migration with OpenTelemetry Spans

```go
// Source: OpenTelemetry Go instrumentation patterns + existing tracing/middleware.go
// In coordination/migration_publisher.go

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

func (m *MigrationPublisher) PublishMigrationEvent(ctx context.Context, event *MigrationEvent) error {
    tracer := otel.Tracer("source-manager")
    ctx, span := tracer.Start(ctx, "publish-migration-event",
        trace.WithAttributes(
            attribute.String("migration_id", event.MigrationID),
            attribute.String("channel_id", event.ChannelID),
            attribute.String("platform", event.Platform),
            attribute.String("from_pod", event.FromPod),
            attribute.String("to_pod", event.ToPod),
            attribute.String("reason", event.Reason),
        ),
    )
    defer span.End()

    // Inject trace context into Redis message
    carrier := make(propagation.MapCarrier)
    otel.GetTextMapPropagator().Inject(ctx, carrier)

    // Publish to Pub/Sub (child span)
    ctx, pubSpan := tracer.Start(ctx, "redis-publish-notification")
    err := m.redisClient.Publish(ctx, "migration:events", jsonPayload).Err()
    if err != nil {
        pubSpan.RecordError(err)
        pubSpan.SetStatus(codes.Error, err.Error())
        pubSpan.End()
        span.RecordError(err)
        span.SetStatus(codes.Error, "failed to publish migration event")
        return err
    }
    pubSpan.SetStatus(codes.Ok, "")
    pubSpan.End()

    // Log to Streams (child span)
    ctx, streamSpan := tracer.Start(ctx, "redis-stream-log")
    err = m.redisClient.XAdd(ctx, &redis.XAddArgs{
        Stream: "migration:log",
        Values: map[string]interface{}{
            "migration_id": event.MigrationID,
            "channel_id":   event.ChannelID,
            "platform":     event.Platform,
            "from_pod":     event.FromPod,
            "to_pod":       event.ToPod,
            "timestamp":    event.Timestamp.Unix(),
            "reason":       event.Reason,
            "traceparent":  carrier.Get("traceparent"),  // W3C Trace Context
            "tracestate":   carrier.Get("tracestate"),
        },
    }).Err()
    if err != nil {
        streamSpan.RecordError(err)
        streamSpan.SetStatus(codes.Error, err.Error())
        streamSpan.End()
        span.RecordError(err)
        span.SetStatus(codes.Error, "failed to log to streams")
        return err
    }
    streamSpan.SetStatus(codes.Ok, "")
    streamSpan.End()

    span.SetStatus(codes.Ok, "migration event published")
    return nil
}
```

### Example 3: Environment-Configurable Sampling with Error Always-On

```go
// Source: OpenTelemetry sampling specification + Better Stack OTel best practices (2026)
// Replace in shared/tracing/tracer.go

import (
    "os"
    "strconv"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace"
)

// AlwaysSampleErrorsSampler wraps a base sampler and always samples error traces
type AlwaysSampleErrorsSampler struct {
    delegate sdktrace.Sampler
}

func (s *AlwaysSampleErrorsSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
    // Check if span will be an error (via attributes or context)
    for _, attr := range p.Attributes {
        if attr.Key == "error" && attr.Value.AsBool() {
            return sdktrace.SamplingResult{
                Decision:   sdktrace.RecordAndSample,
                Tracestate: trace.SpanContextFromContext(p.ParentContext).TraceState(),
            }
        }
    }

    // Delegate to base sampler for non-errors
    return s.delegate.ShouldSample(p)
}

func (s *AlwaysSampleErrorsSampler) Description() string {
    return "AlwaysSampleErrorsSampler{" + s.delegate.Description() + "}"
}

func createConfigurableSampler() sdktrace.Sampler {
    // Read OTEL_SAMPLING_RATE environment variable (default 1.0 = 100%)
    samplingRateStr := os.Getenv("OTEL_SAMPLING_RATE")
    samplingRate := 1.0
    if samplingRateStr != "" {
        if rate, err := strconv.ParseFloat(samplingRateStr, 64); err == nil {
            if rate >= 0.0 && rate <= 1.0 {
                samplingRate = rate
            }
        }
    }

    // Create base sampler with parent-based trace ID ratio
    baseSampler := sdktrace.ParentBased(
        sdktrace.TraceIDRatioBased(samplingRate),
    )

    // Wrap with always-sample-errors logic
    return &AlwaysSampleErrorsSampler{
        delegate: baseSampler,
    }
}

// Update InitTracer to use configurable sampler
func InitTracer(cfg Config, logger *zap.Logger) (func(context.Context) error, error) {
    // ... existing code ...

    sampler := createConfigurableSampler()
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sampler),  // Use configurable sampler instead of AlwaysSample()
    )

    // ... rest of function ...
}
```

### Example 4: Prometheus Alert for Imbalance Ratio

```yaml
# Source: Prometheus alerting best practices + existing alerts/allchat-critical-alerts.yaml
# Add to deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml

- alert: ShardLoadImbalance
  expr: shard_imbalance_ratio > 0.7
  for: 10m
  labels:
    severity: warning
    team: allchat
  annotations:
    summary: "Channel load imbalance detected (ratio {{ $value | humanize }})"
    description: |
      Load imbalance ratio has exceeded 0.7 for more than 10 minutes.
      Current ratio: {{ $value | humanize }} (threshold: 0.7)

      Impact: Some pods are handling significantly more channels than others,
      leading to uneven resource usage and potential performance degradation.

      Remediation Steps:
      1. Check if rebalancing is in cooldown: kubectl logs -n allchat -l app=source-manager | grep "cooldown"
      2. Verify rebalancing enabled: Check source-manager config for REBALANCING_ENABLED=true
      3. Check load distribution: Open Grafana dashboard "Sharding Overview" → Channel Distribution heatmap
      4. Manually trigger rebalancing if needed: curl -X POST http://source-manager:8088/admin/rebalance
      5. Check for pod failures: kubectl get pods -n allchat | grep -v Running

      Auto-rebalancing should trigger automatically. Alert fires if rebalancing
      fails or is stuck in cooldown during persistent imbalance.

- alert: ShardRebalancingThrashing
  expr: increase(shard_rebalancing_total[30m]) > 5
  for: 5m
  labels:
    severity: warning
    team: allchat
  annotations:
    summary: "Rebalancing thrashing detected ({{ $value }} rebalances in 30min)"
    description: |
      More than 5 rebalancing operations occurred in the last 30 minutes.
      This indicates thrashing - repeated rebalancing without achieving stability.

      Impact: Frequent migrations cause connection churn and potential message loss.

      Remediation Steps:
      1. Check rebalancing logs: kubectl logs -n allchat -l app=source-manager | grep "rebalancing"
      2. Review recent migration failures: Check Grafana → Migration Events timeline
      3. Verify load calculation accuracy: Check Prometheus for pod message rates
      4. Increase cooldown duration: Set REBALANCING_COOLDOWN=10m (currently 5m)
      5. Check for oscillating load: Review time-series graphs for stable vs fluctuating loads

      Consider temporarily disabling auto-rebalancing if thrashing persists:
      kubectl set env -n allchat deployment/source-manager REBALANCING_ENABLED=false

- alert: ShardCoordinatorSplitBrain
  expr: sum(shard_coordinator_is_leader) > 1
  for: 1m
  labels:
    severity: critical
    team: allchat
  annotations:
    summary: "Multiple coordinator leaders detected ({{ $value }} leaders)"
    description: |
      More than one source-manager pod believes it is the leader.
      This is a CRITICAL split-brain condition that can cause conflicting assignments.

      Impact: Multiple coordinators may assign same channel to different pods,
      causing duplicate messages or migration conflicts.

      IMMEDIATE ACTION REQUIRED:
      1. Check all source-manager pods: kubectl get pods -n allchat -l app=source-manager
      2. View leader election logs: kubectl logs -n allchat -l app=source-manager | grep "leader"
      3. Check Kubernetes Lease: kubectl get lease -n allchat coordinator-leader
      4. Restart all source-manager pods: kubectl rollout restart -n allchat deployment/source-manager
      5. Verify single leader after restart: Check alert resolution + Prometheus metric

      Root cause: Kubernetes Lease API clock skew or network partition.
      This should resolve automatically via lease expiration (30s).
```

### Example 5: Grafana Dashboard JSON (Partial)

```json
// Source: Grafana heatmap documentation + existing dashboard patterns
// Create as deployments/k8s/monitoring/grafana-dashboards/sharding-overview.json

{
  "dashboard": {
    "title": "Sharding & Rebalancing Overview",
    "tags": ["allchat", "sharding", "rebalancing"],
    "timezone": "browser",
    "panels": [
      {
        "id": 1,
        "type": "stat",
        "title": "Load Imbalance Ratio",
        "targets": [
          {
            "expr": "shard_imbalance_ratio",
            "refId": "A"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "mode": "absolute",
              "steps": [
                {"value": 0, "color": "green"},
                {"value": 0.5, "color": "yellow"},
                {"value": 0.7, "color": "red"}
              ]
            }
          }
        },
        "gridPos": {"h": 4, "w": 6, "x": 0, "y": 0}
      },
      {
        "id": 2,
        "type": "heatmap",
        "title": "Channel Distribution: Pod × Platform",
        "targets": [
          {
            "expr": "sum by (pod_id) (shard_channel_count)",
            "refId": "A",
            "legendFormat": "{{ pod_id }}"
          }
        ],
        "options": {
          "calculate": true,
          "cellGap": 2,
          "color": {
            "scheme": "Spectral"
          },
          "yAxis": {
            "axisLabel": "Pod ID",
            "decimals": 0
          }
        },
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 4}
      },
      {
        "id": 3,
        "type": "graph",
        "title": "Rebalancing Events Timeline",
        "targets": [
          {
            "expr": "increase(shard_rebalancing_total[5m])",
            "refId": "A",
            "legendFormat": "Rebalancing Rate"
          }
        ],
        "annotations": {
          "list": [
            {
              "name": "Rebalancing Triggers",
              "datasource": "Prometheus",
              "expr": "ALERTS{alertname=\"ShardLoadImbalance\"}",
              "tagKeys": "severity",
              "titleFormat": "Imbalance Alert",
              "textFormat": "Ratio: {{ $value }}"
            }
          ]
        },
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 4}
      },
      {
        "id": 4,
        "type": "table",
        "title": "Recent Migration Events",
        "targets": [
          {
            "expr": "shard_migration_total",
            "refId": "A",
            "format": "table",
            "instant": true
          }
        ],
        "transformations": [
          {
            "id": "organize",
            "options": {
              "excludeByName": {},
              "indexByName": {},
              "renameByName": {
                "status": "Status",
                "reason": "Reason",
                "Value": "Count"
              }
            }
          }
        ],
        "gridPos": {"h": 6, "w": 12, "x": 0, "y": 12}
      }
    ],
    "templating": {
      "list": [
        {
          "name": "namespace",
          "type": "constant",
          "current": {"value": "allchat"}
        }
      ]
    },
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "refresh": "30s"
  }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hardcoded sampling (AlwaysSample or NeverSample) | Environment-configurable sampling with TraceIDRatioBased | OTel SDK 1.0+ (2021) | Allows runtime adjustment without code changes - critical for production |
| Separate trace/metric/log backends | Unified observability (LGTM stack: Loki, Grafana, Tempo, Mimir) | Grafana Labs ~2022-2023 | Single pane of glass - correlate traces/metrics/logs in one UI |
| Prometheus Alertmanager webhooks for Grafana annotations | Native Grafana alerting with Prometheus data source | Grafana 8.0+ (2021) | Simpler configuration, unified alerting UI |
| Manual trace context propagation | W3C Trace Context standard (traceparent header) | OTel 1.0+ (2021), W3C recommendation 2020 | Vendor-neutral propagation across any OTel-compatible system |
| Recording rules for all aggregations | On-demand PromQL queries with caching | Prometheus 2.x+ | Reduces storage, allows ad-hoc queries without pre-aggregation |

**Deprecated/outdated:**
- **OpenTracing/OpenCensus**: Merged into OpenTelemetry (2019-2021). All new instrumentation should use OTel APIs.
- **Jaeger native SDKs**: Jaeger now uses OTel for instrumentation. Jaeger SDKs deprecated in favor of OTel exporters.
- **Grafana provisioning via ConfigMaps**: Kubernetes Operator pattern preferred for Grafana deployment (2024+).
- **Prometheus 1.x query language**: Prometheus 2.x PromQL is standard (since 2017).

**Current (2026) best practices:**
- OpenTelemetry for all instrumentation (traces, metrics, logs)
- W3C Trace Context for propagation
- Environment-based configuration (12-factor app principles)
- OTLP gRPC exporter (not Jaeger-specific formats)
- Parent-based sampling with configurable ratio
- Low-cardinality metric labels (pod_id only, not channel_id)

---

## Open Questions

1. **Dashboard Top-N Hot Channel Threshold**
   - What we know: CONTEXT.md specifies top-N hot channels only (10-20) to avoid cardinality explosion
   - What's unclear: Optimal N value - 10 provides quick overview, 20 provides more detail
   - Recommendation: Start with 15 (middle ground), expose as configurable Grafana variable

2. **Coordinator Metrics Namespace**
   - What we know: Coordinator (source-manager) has distinct metrics (leader election, reconciliation cycles)
   - What's unclear: Should use separate metric prefix (coordinator_*) or reuse shard_* prefix
   - Recommendation: Reuse shard_* prefix for consistency. Coordinator is part of sharding system, not separate domain.

3. **Migration Duration Alert Threshold**
   - What we know: Migrations expected to complete in <60 seconds per design
   - What's unclear: When to alert on slow migrations - 90s? 120s?
   - Recommendation: Alert on P95 migration duration >90s for 10 minutes. Allows transient slowdowns but catches persistent issues.

4. **Grafana Primary At-a-Glance View**
   - What we know: CONTEXT.md delegates choice to Claude's discretion (imbalance ratio, load distribution, or recent events)
   - What's unclear: Which provides most value for operators
   - Recommendation: Imbalance ratio gauge (large stat panel) - single number operator can glance at. Green <0.5, yellow 0.5-0.7, red >0.7. Most actionable.

---

## Sources

### Primary (HIGH confidence)

- **Existing codebase:**
  - shared/metrics/shard_metrics.go - Verified existing metrics, patterns, naming conventions
  - shared/tracing/tracer.go - Verified OpenTelemetry SDK configuration, current AlwaysSample() usage
  - shared/tracing/middleware.go - Verified Gin middleware, span creation patterns
  - services/source-manager/coordination/migration_publisher.go - Verified migration event structure, Redis Streams logging
  - deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml - Verified existing alert patterns
  - docs/architecture/04-OBSERVABILITY.md - Verified infrastructure deployment status, metric naming

- **Official documentation:**
  - [Prometheus Recording Rules](https://prometheus.io/docs/practices/rules/) - Recording rule patterns, cardinality management
  - [OpenTelemetry Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/) - OTEL_TRACES_SAMPLER, OTEL_TRACES_SAMPLER_ARG configuration
  - [OpenTelemetry Sampling](https://opentelemetry.io/docs/concepts/sampling/) - TraceIDRatioBased, ParentBased sampling strategies
  - [Grafana Heatmap Visualization](https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/visualizations/heatmap/) - Heatmap panel configuration, time-series support

### Secondary (MEDIUM confidence)

- **2026 guides and best practices:**
  - [How to Configure OpenTelemetry Sampling Strategies](https://oneuptime.com/blog/post/2026-01-24-opentelemetry-sampling-strategies/view) - Sampling rate configuration, error always-on patterns
  - [Prometheus Label Best Practices](https://oneuptime.com/blog/post/2026-01-30-prometheus-label-best-practices/view) - Cardinality explosion prevention, label design
  - [OpenTelemetry W3C Context Propagation](https://oneuptime.com/blog/post/2026-01-30-opentelemetry-w3c-context-propagation/view) - Traceparent header format, injection/extraction
  - [Prometheus Recording Rule Optimization](https://oneuptime.com/blog/post/2026-01-30-prometheus-recording-rules/view) - Pre-aggregation strategies, query performance

- **Community guides:**
  - [Essential OpenTelemetry Best Practices](https://betterstack.com/community/guides/observability/opentelemetry-best-practices/) - Span attributes, error handling, sampling strategies
  - [OpenTelemetry Context Propagation Explained](https://betterstack.com/community/guides/observability/otel-context-propagation/) - Trace context across async boundaries
  - [Prometheus & Grafana: Complete Monitoring Guide 2026](https://devtoolbox.dedyn.io/blog/prometheus-grafana-complete-guide) - Integration patterns, annotation configuration

### Tertiary (LOW confidence - needs validation)

- **Grafana annotations from AlertManager:** Search results mention querying Prometheus ALERTS metric for annotations, but specific implementation details need testing with actual deployment.
- **Redis Streams trace propagation:** Pattern is documented for message queues generally, but specific Redis Streams + OpenTelemetry Go integration not widely documented. Implementation needs testing.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, versions verified in go.mod
- Architecture patterns: HIGH - Prometheus/OTel patterns verified in official docs, existing code demonstrates patterns
- Metrics implementation: HIGH - Existing ShardMetrics provides template, missing metrics identified
- Tracing instrumentation: MEDIUM-HIGH - OTel SDK ready, but specific migration trace structure needs implementation/testing
- Grafana dashboards: MEDIUM - Heatmap/annotation patterns documented, but dashboard JSON needs creation and testing
- Pitfalls: HIGH - Cardinality explosion, sampling, and trace context propagation are well-documented pitfalls with clear solutions

**Research date:** 2026-02-20
**Valid until:** ~30 days (stable domain, but Grafana/OTel versions update quarterly)

**Key unknowns resolved during research:**
- ✅ Trace context propagation through Redis: Use W3C Trace Context in message fields (traceparent/tracestate)
- ✅ Sampling configuration: Environment variables OTEL_TRACES_SAMPLER + OTEL_SAMPLING_RATE
- ✅ Cardinality management: Top-N hot channels, pod_id label only per CONTEXT.md constraints
- ✅ Heatmap visualization: Grafana native heatmap panel supports time-series data, no plugin required
- ✅ Alert thresholds: Locked values from CONTEXT.md (0.7 imbalance, >5 rebalances in 30min)
