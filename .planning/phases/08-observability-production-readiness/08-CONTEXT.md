# Phase 8: Observability & Production Readiness - Context

**Gathered:** 2026-02-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement comprehensive monitoring, metrics, distributed tracing, and dashboards to make the distributed sharding system observable and debuggable in production. This includes Prometheus metrics exposure, Grafana dashboards, Prometheus alerting, and OpenTelemetry distributed tracing for all listener services and the coordinator.

</domain>

<decisions>
## Implementation Decisions

### Dashboard layout & visualizations
- Single comprehensive dashboard (not multiple focused dashboards)
- Two heatmap views for channel distribution:
  - Pod (Y) × Platform (X) - shows platform-specific distribution
  - Pod (Y) × Time (X) - shows rebalancing effects over time
- Rebalancing timeline uses both approaches:
  - Event markers/annotations on time series graphs for correlation
  - Dedicated event timeline panel with details (channels moved, reason, duration)
- Primary at-a-glance view at dashboard top: Claude's discretion (choose most useful)

### Alert design & thresholds
- Imbalance ratio alert threshold: 0.7 for 10 minutes (keep as specified in requirements)
- Alert severity based on impact to users:
  - Critical = message loss or user-visible failures
  - Warning = degraded performance or suboptimal state
- Alerts include remediation steps in descriptions (suggested actions, not just notifications)
- Rebalancing thrashing alert: Relaxed to >5 rebalances in 30 minutes (was >3 in 15min)

### Tracing granularity & context
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

### Metrics cardinality & labels
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

</decisions>

<specifics>
## Specific Ideas

- Need operational flexibility via env var for trace sampling - start high (1.0) during initial weeks to verify everything works, then reduce to lower rate (0.1) for production monitoring
- Ability to turn sampling back up if pods misbehave without code changes
- Each listener service pod is platform-specific (twitch-listener only handles Twitch, etc.) - platform is implicit, not a metric label dimension

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope

</deferred>

---

*Phase: 08-observability-production-readiness*
*Context gathered: 2026-02-20*
