# Phase 4: Grafana Dashboard Audit & Metrics Gap Implementation - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Audit all existing Grafana dashboards and Prometheus metrics against what the platform actually needs, then fill the gaps — wire up missing metric recording in all services (Go + Node.js listeners), create a tiered dashboard set, and ensure alerting covers all critical paths. Dashboards and alerts are stored as code in the caesar-deployment repo.

</domain>

<decisions>
## Implementation Decisions

### Audit scope & approach
- **Both code + live audit**: grep through all services for metrics.RecordX() calls to find code gaps, then query live Prometheus via Grafana MCP to verify what's actually being scraped — ground truth from production
- **All 14 services audited equally** — no prioritization by criticality; comprehensive audit across the full platform
- **Full dashboard audit included** — check existing Grafana dashboards for broken queries, stale panels, missing data sources (3 new listeners added post-v1.1 without dashboard coverage)
- **Gap matrix output**: Service × Metric matrix showing: ✅ wired + scraped, ⚠️ defined but not wired, ❌ missing entirely — feeds directly into planning as a task checklist

### Dashboard strategy
- **Tiered dashboards**: 1 overview dashboard + 4 focused dashboards grouped by pipeline stage
- **Overview dashboard**: Service health grid — traffic light grid where each service shows green/yellow/red based on health check + key metric (messages/sec for listeners, connections for gateway, etc.)
- **Focused dashboards by pipeline stage**:
  1. **Listeners** — all 7 listeners (Twitch, YouTube, Kick, TikTok, Discord, InnerTube, twitch-eventsub)
  2. **Message Processing** — message-processor + emote-service
  3. **Delivery** — api-gateway + WebSocket connections
  4. **Platform Ops** — auth-service, overlay-manager, source-manager, token-refresh-service
- **Dashboards stored as code** in `../caesar-deployment` repo — JSON provisioned via ConfigMaps/Grafana provisioning, version-controlled and reviewable in PRs
- Carries forward from Phase 8: W3C Trace Context, configurable sampling, top-N hot channels only

### Metrics wiring priority
- **Message flow first**: wire up end-to-end message pipeline (listeners messages received/published → processor consumed/processed/enriched → gateway WebSocket delivery) before other metric categories
- **Extend shared/metrics/ package first** for new services: audit if ListenerMetrics needs new metric types for Discord (Gateway events), InnerTube (batch detection), twitch-eventsub (webhook events) before wiring
- **All listeners included**: all 7 Go listeners + tiktok-listener (Node.js with prom-client)
- **Support-bot excluded** — different concern, not a listener, doesn't need observability metrics in this phase
- Connection health, API calls, and business metrics follow after message flow is wired

### Alerting gaps
- **Four new alert categories** (in addition to existing YouTube quota and sharding alerts):
  1. **Listener disconnections** — alert when any listener loses platform connection for >2min
  2. **Message pipeline stalls** — alert when messages stop flowing through processor/gateway for >1min while listeners are active
  3. **WebSocket connection drops** — alert on >50% drop in active connections in 5min, or zero connections
  4. **Error rate spikes** — alert when error rate crosses >5% of requests for any service
- **Alert severity** (carried from Phase 8): Critical = message loss or user-visible failures, Warning = degraded/suboptimal
- **Alert routing**: Discord channel via Grafana webhook — lead dev pinged for critical alerts
- **Alerts stored as code** in caesar-deployment repo (extends existing `grafana-allchat-alerts.yaml` pattern)
- **Inline remediation steps** in alert descriptions — 2-3 line actionable guidance (e.g., "Check pod logs: kubectl -n allchat logs ...")

### Claude's Discretion
- Exact gap matrix format and level of detail
- shared/metrics/ package extensions needed for new listener types
- Dashboard panel layout and specific PromQL queries
- Alert threshold tuning (exact values for connection timeout, pipeline stall duration, etc.)
- prom-client setup pattern for tiktok-listener (Node.js)
- Order of services within each wiring phase
- Grafana provisioning mechanism (ConfigMap vs sidecar)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing metrics infrastructure
- `shared/metrics/README.md` — Usage guide for the metrics package (ListenerMetrics, ProcessorMetrics, GatewayMetrics, BusinessMetrics)
- `shared/metrics/listener.go` — Listener metric definitions (connection, source, message, rate limit, API, error)
- `shared/metrics/processor.go` — Message processor metric definitions
- `shared/metrics/gateway.go` — API Gateway metric definitions
- `shared/metrics/business.go` — Business intelligence metric definitions
- `shared/metrics/shard_metrics.go` — Sharding/load balancing metric definitions

### Metrics rollout status
- `docs/METRICS_ROLLOUT_COMPLETE.md` — Current state: infrastructure complete, wiring incomplete; lists per-service integration points
- `docs/METRICS_IMPLEMENTATION_PLAN.md` — Original implementation plan with phased rollout

### Existing Grafana/alerting config
- `deployments/k8s/monitoring/grafana-alerts/grafana-allchat-alerts.yaml` — Current alert rules (YouTube quota, sharding)

### Prior observability decisions
- `.planning/phases/08-observability-production-readiness/08-CONTEXT.md` — Phase 8 decisions: single dashboard, W3C tracing, configurable sampling, top-N hot channels, alert severity model

### Deployment repo (dashboards + alerts destination)
- `../caesar-deployment/apps/workloads/all-chat/` — Kubernetes manifests for all-chat services; dashboards and alerts go here

### Architecture docs
- `docs/architecture/04-OBSERVABILITY.md` — Observability architecture overview
- `docs/architecture/01-DATA-FLOW.md` — Message pipeline flow (listeners → processor → gateway)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/metrics/` package: 5 metric type files with `promauto` registration — ready to use, just needs `RecordX()` calls wired into service components
- All 9 Go services already have `/metrics` endpoint via `promhttp.Handler()` — no endpoint work needed
- `grafana-allchat-alerts.yaml`: existing alert ConfigMap pattern — extend with new alert rules

### Established Patterns
- `promauto.NewCounterVec` / `NewGaugeVec` / `NewHistogramVec` used throughout `shared/metrics/` — standard Prometheus client pattern
- Metrics initialized in `cmd/main.go` of each service, passed via dependency injection
- Kick-listener already has partial metrics wiring in `websocket/client.go` and `channels/manager.go` — reference implementation

### Integration Points
- Each service's `cmd/main.go`: initialize metrics and pass to domain packages
- Listener domain packages (`irc/client.go`, `websocket/client.go`, etc.): add `RecordMessage()`, `RecordConnection()` calls
- Message processor pipeline (`consumer/`, `normalizer/`, `enricher/`, `publisher/`): add `RecordProcessed()`, `RecordEnriched()` calls
- API Gateway (`websocket/hub.go`, `subscription/manager.go`): add `RecordConnection()`, `RecordMessageSent()` calls
- tiktok-listener (Node.js): needs `prom-client` npm package + `/metrics` endpoint

</code_context>

<specifics>
## Specific Ideas

- Live audit via Grafana MCP gives ground truth — catches both missing wiring AND missing scrape configs
- Gap matrix format directly feeds into planner as a task checklist — each ⚠️ or ❌ becomes a plan task
- Dashboard JSON in caesar-deployment follows existing pattern (alerts already there)
- tiktok-listener is the only Node.js listener needing prom-client — keep its metrics API surface consistent with Go listeners' metric names

</specifics>

<deferred>
## Deferred Ideas

- Support-bot metrics (query count, response time, memory operations) — separate concern, not part of this phase
- SLO/SLI framework and error budget tracking — future phase
- Distributed tracing gap analysis (new services may lack OpenTelemetry spans) — could be its own phase
- Custom Grafana plugins or complex visualizations — standard panels sufficient for now

</deferred>

---

*Phase: 04-grafana-dashboard-audit-metrics-gap-implementation*
*Context gathered: 2026-03-26*
