# Phase 04: Grafana Dashboard Audit & Metrics Gap Implementation - Research

**Researched:** 2026-03-26
**Domain:** Prometheus metrics wiring, Grafana dashboard-as-code, Kubernetes ServiceMonitor, prom-client (Node.js)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Audit scope & approach**
- Both code + live audit: grep through all services for RecordX() calls to find code gaps, then query live Prometheus via Grafana MCP to verify what's actually being scraped — ground truth from production
- All 14 services audited equally — no prioritization by criticality; comprehensive audit across the full platform
- Full dashboard audit included — check existing Grafana dashboards for broken queries, stale panels, missing data sources (3 new listeners added post-v1.1 without dashboard coverage)
- Gap matrix output: Service × Metric matrix showing: wired + scraped, defined but not wired, missing entirely — feeds directly into planning as a task checklist

**Dashboard strategy**
- Tiered dashboards: 1 overview dashboard + 4 focused dashboards grouped by pipeline stage
- Overview dashboard: Service health grid — traffic light grid where each service shows green/yellow/red based on health check + key metric
- Focused dashboards by pipeline stage:
  1. Listeners — all 7 listeners (Twitch, YouTube, Kick, TikTok, Discord, InnerTube, twitch-eventsub)
  2. Message Processing — message-processor + emote-service
  3. Delivery — api-gateway + WebSocket connections
  4. Platform Ops — auth-service, overlay-manager, source-manager, token-refresh-service
- Dashboards stored as code in `../caesar-deployment` repo — JSON provisioned via ConfigMaps/Grafana provisioning, version-controlled and reviewable in PRs
- Carries forward from Phase 8: W3C Trace Context, configurable sampling, top-N hot channels only

**Metrics wiring priority**
- Message flow first: wire up end-to-end message pipeline (listeners messages received/published → processor consumed/processed/enriched → gateway WebSocket delivery) before other metric categories
- Extend shared/metrics/ package first for new services: audit if ListenerMetrics needs new metric types for Discord (Gateway events), InnerTube (batch detection), twitch-eventsub (webhook events) before wiring
- All listeners included: all 7 Go listeners + tiktok-listener (Node.js with prom-client)
- Support-bot excluded — different concern, not a listener, doesn't need observability metrics in this phase
- Connection health, API calls, and business metrics follow after message flow is wired

**Alerting gaps**
- Four new alert categories (in addition to existing YouTube quota and sharding alerts):
  1. Listener disconnections — alert when any listener loses platform connection for >2min
  2. Message pipeline stalls — alert when messages stop flowing through processor/gateway for >1min while listeners are active
  3. WebSocket connection drops — alert on >50% drop in active connections in 5min, or zero connections
  4. Error rate spikes — alert when error rate crosses >5% of requests for any service
- Alert severity: Critical = message loss or user-visible failures, Warning = degraded/suboptimal
- Alert routing: Discord channel via Grafana webhook — lead dev pinged for critical alerts
- Alerts stored as code in caesar-deployment repo (extends existing grafana-allchat-alerts.yaml pattern)
- Inline remediation steps in alert descriptions — 2-3 line actionable guidance

### Claude's Discretion
- Exact gap matrix format and level of detail
- shared/metrics/ package extensions needed for new listener types
- Dashboard panel layout and specific PromQL queries
- Alert threshold tuning (exact values for connection timeout, pipeline stall duration, etc.)
- prom-client setup pattern for tiktok-listener (Node.js)
- Order of services within each wiring phase
- Grafana provisioning mechanism (ConfigMap vs sidecar)

### Deferred Ideas (OUT OF SCOPE)
- Support-bot metrics (query count, response time, memory operations) — separate concern, not part of this phase
- SLO/SLI framework and error budget tracking — future phase
- Distributed tracing gap analysis (new services may lack OpenTelemetry spans) — could be its own phase
- Custom Grafana plugins or complex visualizations — standard panels sufficient for now
</user_constraints>

---

## Summary

The metrics infrastructure for all-chat is in a "skeleton complete, muscles missing" state. Every service exposes `/metrics` via `promhttp.Handler()` (or Node.js equivalent), the shared `metrics` package defines all metric types with `promauto`, and a Prometheus ServiceMonitor scrapes nine services. However, only **four services have actual `RecordX()` calls wired into their domain logic**: twitch-listener (full wiring), youtube-listener (quota only), message-processor (pipeline stages), and api-gateway (WebSocket connect/disconnect only). Five listeners are completely unwired at the code level: kick-listener uses its own local package with partial wiring, discord-listener has its own four-metric package, innertube has its own package, tiktok-listener has prom-client already installed and wired, and twitch-eventsub-listener exposes the endpoint but records nothing.

The existing dashboard ConfigMap (`allchat-grafana-dashboards.yaml`) contains four dashboards: Listener Health, Message Pipeline, Platform Overview, and YouTube Quota Monitoring. These reference Twitch, Kick, and YouTube metrics but have zero panels for Discord listener, InnerTube, and twitch-eventsub — three services added in v1.5/v1.6. The ServiceMonitor (`servicemonitor.yaml`) likewise omits all three of these newer listeners.

The deployment mechanism is well-established: dashboard JSON lives in a Kubernetes ConfigMap (`allchat-grafana-dashboards`) in the `kube-prometheus-stack` dashboards directory, and alerts live in a separate ConfigMap (`grafana-allchat-alerts`) with the `grafana_alert: "1"` label. Both are sidecar-loaded by Grafana. The planner should follow this exact same pattern for new dashboards and alerts.

**Primary recommendation:** Audit live → build gap matrix → wire missing RecordX() calls service by service → add ServiceMonitor entries for three new listeners → create five new dashboards as code → extend allchat-alerts.yaml with four new alert groups.

---

## Gap Matrix (Code Audit Results)

This is the authoritative pre-audit gap matrix from code inspection. The live Prometheus audit (via Grafana MCP during planning) will confirm what's actually scraped.

### Services × Metric Coverage

| Service | `/metrics` endpoint | Connection | Messages Rx | Messages Pub | API Calls | Errors | Special |
|---------|---------------------|------------|-------------|--------------|-----------|--------|---------|
| twitch-listener | ✅ | ✅ wired | ✅ wired | ✅ wired | ❌ missing | ❌ missing | SetActiveSources ✅ |
| kick-listener | ✅ | ✅ local pkg | ❌ missing | ✅ local pkg | ❌ missing | ❌ missing | Uses kick_listener_* names (not shared/) |
| youtube-listener | ✅ | ❌ missing | ❌ missing | ❌ missing | ❌ missing | ❌ missing | Quota ✅ wired via quota/tracker.go |
| tiktok-listener | ✅ (Node.js) | ✅ prom-client | ✅ prom-client | ✅ prom-client | ❌ missing | ✅ prom-client | Full prom-client already done |
| discord-listener | ✅ | ✅ local pkg | ❌ missing | ❌ missing | ❌ missing | ❌ missing | 4 metrics: gateway events, guilds, shard, resume |
| youtube-listener-innertube | ✅ | ❌ missing | ✅ local pkg | ✅ local pkg | ✅ local pkg | ✅ local pkg | Own metrics package; not using shared/ |
| twitch-eventsub-listener | ✅ | ❌ missing | ❌ missing | ❌ missing | ❌ missing | ❌ missing | Endpoint only, zero wiring |
| message-processor | ✅ | N/A | ✅ wired | ✅ wired | N/A | ❌ missing | Emote enrichment ❌ missing |
| api-gateway | ✅ | ✅ partial | ❌ missing | ❌ missing | ❌ missing | ❌ missing | Connect/disconnect only; no message sent/received |
| auth-service | ✅ | N/A | N/A | N/A | ❌ missing | ❌ missing | Zero custom wiring |
| overlay-manager | ✅ | N/A | N/A | N/A | ❌ missing | ❌ missing | Zero custom wiring |
| source-manager | ✅ | N/A | N/A | N/A | ❌ missing | ❌ missing | Shard metrics wired (ShardMetrics pkg) |
| token-refresh-service | ✅ | N/A | N/A | N/A | ❌ missing | ❌ missing | Zero custom wiring |
| emote-service | ✅ | N/A | N/A | N/A | ❌ missing | ❌ missing | Zero custom wiring |

### ServiceMonitor Coverage Gaps

| Listener | In ServiceMonitor | Action needed |
|----------|------------------|---------------|
| twitch-listener | ✅ allchat-listeners | None |
| kick-listener | ✅ allchat-listeners | None |
| tiktok-listener | ✅ allchat-listeners | None |
| youtube-listener | ✅ allchat-listeners | None |
| discord-listener | ❌ MISSING | Add to ServiceMonitor |
| youtube-listener-innertube | ❌ MISSING | Add to ServiceMonitor |
| twitch-eventsub-listener | ❌ MISSING | Add to ServiceMonitor |
| message-processor | ✅ allchat-services | None |
| api-gateway | ✅ allchat-services | None |
| source-manager | ✅ allchat-source-manager | None |

### Existing Dashboard Coverage Gaps

Current dashboards in `allchat-grafana-dashboards.yaml`:
1. **All-Chat Listener Health** — covers Twitch, YouTube (quota), Kick only; no Discord, InnerTube, twitch-eventsub
2. **All-Chat Message Pipeline** — covers generic pipeline but missing platform breakdown
3. **All-Chat Platform Overview** — high-level health, deployment availability
4. **All-Chat YouTube Quota Monitoring** — YouTube-specific quota deep dive

Missing coverage:
- Discord listener panels (gateway events, active guilds, shard ownership)
- InnerTube listener panels (messages published, error rate, batch detection)
- twitch-eventsub listener panels (webhook events, connection status)
- End-to-end message flow visualization (listeners → processor → gateway in one view)
- WebSocket delivery latency and drop rate panels

---

## Standard Stack

### Core (Confirmed from codebase inspection)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `prometheus/client_golang` | v1.23.2 | Go metrics | Already in shared/go.mod, used by all Go services |
| `promauto` | (bundled with client_golang) | Auto-registration | Used throughout shared/metrics/; avoids manual MustRegister |
| `promhttp` | (bundled with client_golang) | HTTP handler | Used in every Go service's cmd/main.go |
| `prom-client` | (already installed in tiktok-listener) | Node.js metrics | Already used by tiktok-listener; no new install needed |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `monitoring.coreos.com/v1 ServiceMonitor` | (kube-prometheus-stack) | Prometheus scrape config | For every service with a `/metrics` endpoint |
| Grafana sidecar dashboard ConfigMap | `grafana_dashboard: "1"` label | Dashboard provisioning | For all new dashboards — existing pattern in kube-prometheus-stack |
| Grafana alert ConfigMap | `grafana_alert: "1"` label | Alert provisioning | For all new alert rules — existing pattern |

### Alternatives Considered (NOT to use)

| Standard | Alternative | Why Standard Wins |
|----------|-------------|-------------------|
| Shared `metrics/` package | Per-service metric packages | Kick/discord/innertube each created their own — acceptable but creates naming inconsistency; for new wiring, use shared/ |
| ConfigMap + sidecar provisioning | Grafana API | Already established; consistent with allchat-grafana-dashboards.yaml pattern |

**Installation:**
```bash
# All Go dependencies already in place — no new installs needed
# tiktok-listener prom-client already installed
# Verify Go shared metrics module
cat /home/moersener/Hobby/all-chat/shared/go.mod | grep prometheus
```

---

## Architecture Patterns

### Established Deployment Structure

```
caesar-deployment/
apps/platform/kube-prometheus-stack/
├── dashboards/
│   └── allchat-grafana-dashboards.yaml    # ConfigMap with dashboard JSON
├── grafana-alerts/
│   ├── allchat-alerts.yaml                # Alert rules ConfigMap (grafana_alert: "1")
│   ├── allchat-contact-points.yaml        # Discord webhook contact point
│   └── allchat-notification-policies.yaml # Routing policies
└── helm-values.yaml                        # dashboardsConfigMaps: default: allchat-grafana-dashboards
```

Dashboard provisioning: Grafana Helm chart's `dashboardsConfigMaps.default` mounts the ConfigMap into `/var/lib/grafana/dashboards/default`. The sidecar with `label: grafana_dashboard` also watches for labeled ConfigMaps. Alert ConfigMaps use `grafana_alert: "1"` label and are hot-reloaded.

### Pattern 1: Adding RecordX() Calls to a Go Service

**What:** Inject `*metrics.ListenerMetrics` into domain components via cmd/main.go, call RecordX() at key points.

**When to use:** Any Go listener service that already initializes `metrics.NewListenerMetrics()` in cmd/main.go.

**Example (from twitch-listener, the reference implementation):**
```go
// Source: services/twitch-listener/irc/connection.go
// In domain struct:
type ConnectionManager struct {
    metrics *metrics.ListenerMetrics
    // ...
}

// At connection:
cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "attempting")
cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", true)

// On message:
cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "chat")
cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
```

### Pattern 2: Extending shared/metrics/ for New Listener Types

**What:** For listeners with unique metric needs (Discord: gateway events, twitch-eventsub: webhook events), either extend ListenerMetrics with optional fields or use the service's existing local package.

**When to use:** When the standard ListenerMetrics fields don't cover platform-specific metrics.

**Recommendation:** Keep existing local packages for kick-listener, discord-listener, and innertube (they're already wired and use service-specific metric names). For twitch-eventsub, use shared/metrics/ListenerMetrics since it has zero wiring — no local package exists.

### Pattern 3: Dashboard JSON as ConfigMap

**What:** Dashboard JSON stored in `allchat-grafana-dashboards.yaml` under a data key per dashboard.

**When to use:** All new dashboards for this phase.

**Example structure (from existing file):**
```yaml
apiVersion: v1
data:
  my-new-dashboard.json: |
    {
      "title": "All-Chat Listeners",
      "panels": [...],
      "uid": "allchat-listeners-v2"
    }
kind: ConfigMap
metadata:
  name: allchat-grafana-dashboards
  namespace: monitoring
  labels:
    grafana_dashboard: "1"
```

**IMPORTANT:** The existing configmap uses `dashboardsConfigMaps.default: allchat-grafana-dashboards` — new dashboard JSON should be added as new keys in the SAME ConfigMap (not separate ones) unless the file grows unwieldy.

### Pattern 4: Alert Rules as ConfigMap

**What:** Alert rules appended to `allchat-alerts.yaml` under existing groups or new named groups.

**Example (from existing rules — add new groups to same ConfigMap):**
```yaml
- orgId: 1
  name: allchat-listener-health
  folder: All-Chat Alerts
  interval: 1m
  rules:
    - uid: listener-disconnected
      title: Listener Disconnected
      condition: B
      data:
        - refId: A
          datasourceUid: prometheus
          model:
            expr: listener_connection_status == 0
        - refId: B
          datasourceUid: __expr__
          model:
            type: threshold
            conditions:
              - evaluator: {params: [1], type: gte}
      for: 2m
      annotations:
        summary: "{{ $labels.platform }} listener disconnected"
        description: "Check pod logs: kubectl -n allchat logs -l app={{ $labels.service }} --tail=50"
      labels:
        severity: critical
        team: allchat
```

### Anti-Patterns to Avoid

- **Creating a new ConfigMap per dashboard:** Use the existing `allchat-grafana-dashboards` ConfigMap — the Helm chart mounts one ConfigMap by name. New JSON keys in the same map is the correct pattern.
- **High-cardinality labels:** The existing `MessagesReceived` metric uses `channel_id` as a label — for dashboards, use `topk()` or `sum by (platform)` to avoid cardinality explosion.
- **Separate alert ConfigMaps for each category:** All alert rules go into `allchat-alerts.yaml` as new group entries. Separate files would require Helm values changes.
- **Not using promauto:** All existing metrics use `promauto.NewCounterVec` etc. — never use `prometheus.MustRegister` directly.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Prometheus text format | Custom metrics serializer | `promhttp.Handler()` (Go) / `prom-client` (Node.js) | Already standard across all services |
| Alert routing | Custom webhook logic | Grafana alert notification policies (allchat-notification-policies.yaml) | Already wired to Discord channel with severity routing |
| Dashboard provisioning | Kubernetes operator or manual upload | Grafana sidecar ConfigMap (`grafana_dashboard: "1"` label) | Already working in production |
| Alert templating | Custom Go templates | Grafana alert template syntax (`{{ $labels.platform }}`) | Existing contact point template already handles multi-alert formatting |

**Key insight:** The entire observability infrastructure (Prometheus, Grafana, alert routing, provisioning) is already deployed and working. This phase is purely about filling content gaps — adding RecordX() calls in Go, ensuring three services are in ServiceMonitor, adding JSON panels for three new listeners, and adding four alert groups.

---

## Common Pitfalls

### Pitfall 1: promauto Panic on Duplicate Registration

**What goes wrong:** `promauto.NewCounterVec` panics at startup with "duplicate metrics collector registration" if the same metric name is registered twice. This happens if a service imports both `shared/metrics/` and also re-registers the same metric name in a local package.

**Why it happens:** The kick-listener uses a local `metrics/` package with names like `kick_listener_messages_total` (different from `listener_messages_received_total` in shared/). No conflict. But if someone tries to wire `shared/metrics/ListenerMetrics` into kick-listener ON TOP of the existing local package, the local `promauto` registrations and the shared `promauto` registrations will conflict on the global registry.

**How to avoid:** For kick-listener, discord-listener, and innertube — wire their EXISTING local package RecordX() equivalents, do NOT also import shared/metrics/ListenerMetrics. Only twitch-eventsub-listener should use shared/metrics/ListenerMetrics (it has no local package).

**Warning signs:** Pod crash at startup with "panic: registering duplicate metrics collector" in logs.

### Pitfall 2: ServiceMonitor Selector Mismatch

**What goes wrong:** New ServiceMonitor entries reference an `app` label value that doesn't match the pod labels in the Deployment manifest. Prometheus silently scrapes nothing.

**Why it happens:** The existing ServiceMonitor uses `matchExpressions` with `key: app, operator: In, values: [...]`. If the Deployment uses a different label key or value, scraping silently fails.

**How to avoid:** Before adding ServiceMonitor entries for discord-listener, innertube, and twitch-eventsub, verify the actual pod label values:
```bash
kubectl --context k8s-daemon01 -n allchat get pods -l app=discord-listener -o jsonpath='{.items[0].metadata.labels}'
```

**Warning signs:** Prometheus target shows "0 targets" for new ServiceMonitor, or `up == 0` for the new service.

### Pitfall 3: Dashboard JSON Datasource UID Mismatch

**What goes wrong:** Dashboard panels reference `"uid": "Prometheus"` in the datasource field, but the actual Prometheus datasource UID in the deployed Grafana may differ.

**Why it happens:** Existing panels in `allchat-grafana-dashboards.yaml` use `"uid": "Prometheus"` (capital P). This was configured when the Grafana instance was set up. New panels must use the exact same UID string.

**How to avoid:** All new dashboard panels must use `"datasourceUid": "prometheus"` (lowercase, matching the existing panels in the file). Audit the existing dashboard JSON for the exact string used before writing new panels.

**Warning signs:** Panels show "No data" immediately after provisioning, even for metrics confirmed present in Prometheus.

### Pitfall 4: Alert Rule UID Collision

**What goes wrong:** New alert rules with UIDs that match existing rules cause Grafana to silently ignore the duplicate or throw a provisioning error.

**Why it happens:** Grafana alert UIDs must be globally unique within a Grafana org. The existing rules use UIDs like `youtube-quota-critical`, `pod-crashloop`.

**How to avoid:** Use descriptive, platform-scoped UIDs for new rules: `listener-disconnected-twitch`, `pipeline-stall`, `websocket-drop`, `error-rate-spike`. Never reuse existing UIDs.

### Pitfall 5: Missing `for` Duration on Message Flow Alerts

**What goes wrong:** A `for: 0` or missing `for` field on pipeline stall alerts triggers immediately on any Prometheus scrape gap (pod restart, network blip), causing alert storms.

**Why it happens:** `for: 1m` means "condition must be true for 1 minute before firing." Pipeline stalls are real at 1min. Listener disconnections need 2min minimum to avoid flapping on normal reconnect cycles.

**How to avoid:** Use `for: 2m` for listener disconnection alerts, `for: 1m` for pipeline stall alerts (matching the locked decisions).

---

## Code Examples

### Wiring RecordX() into twitch-eventsub (no local package, use shared/)

```go
// Source: shared/metrics/listener.go API
// In services/twitch-eventsub-listener/cmd/main.go:
listenerMetrics := metrics.NewListenerMetrics("twitch-eventsub", "twitch-eventsub-listener")

// In the webhook handler (wherever EventSub events are received):
listenerMetrics.RecordMessage("twitch-eventsub", "twitch-eventsub-listener", channelID, "event")
listenerMetrics.RecordPublish("twitch-eventsub", "twitch-eventsub-listener", "success")
listenerMetrics.RecordConnection("twitch-eventsub", "twitch-eventsub-listener", "webhook", true)
```

### Adding to ServiceMonitor (matching existing allchat-listeners pattern)

```yaml
# In servicemonitor.yaml — add to the allchat-listeners matchExpressions values list:
- key: app
  operator: In
  values:
    - twitch-listener
    - kick-listener
    - tiktok-listener
    - youtube-listener
    - discord-listener          # ADD
    - youtube-listener-innertube # ADD
    - twitch-eventsub-listener  # ADD
```

### PromQL for Listener Disconnection Alert

```promql
# Alert: any listener connection_status == 0 for 2min
listener_connection_status{job=~"allchat-.*"} == 0

# Alert: message pipeline stall — no messages consumed while listeners active
rate(processor_messages_consumed_total[5m]) == 0
AND
sum(listener_connection_status) > 0

# Alert: WebSocket connection drop >50% in 5min
(gateway_websocket_connections_active - gateway_websocket_connections_active offset 5m)
/ gateway_websocket_connections_active offset 5m < -0.5

# Alert: error rate spike >5% for any service
rate(listener_errors_total[5m]) / rate(listener_messages_received_total[5m]) > 0.05
```

### Dashboard Panel for Traffic Light Health Grid

```json
// Source: existing allchat-grafana-dashboards.yaml pattern (stat panel with thresholds)
{
  "type": "stat",
  "title": "Discord Connection",
  "datasource": {"type": "prometheus", "uid": "prometheus"},
  "targets": [{"expr": "discord_listener_shard_ownership", "refId": "A"}],
  "fieldConfig": {
    "defaults": {
      "color": {"mode": "thresholds"},
      "thresholds": {
        "steps": [
          {"value": 0, "color": "red"},
          {"value": 1, "color": "green"}
        ]
      },
      "mappings": [
        {"type": "value", "value": "0", "text": "No Leader"},
        {"type": "value", "value": "1", "text": "Active"}
      ]
    }
  },
  "options": {"colorMode": "background", "textMode": "value"}
}
```

---

## Metrics State Per Service (Detailed)

### twitch-listener — MOSTLY WIRED
- ✅ `RecordConnectionAttempt`, `RecordConnection` — irc/connection.go
- ✅ `RecordMessage` — all message types (chat, event, deletion)
- ✅ `RecordPublish` — success/failure after each Redis publish
- ✅ `SetActiveSources` — channels/manager.go
- ❌ `RecordAPICall` — no IRC-specific API calls, acceptable
- ❌ `RecordError` — not wired; errors return as publish failures only
- Status: **Effectively complete for message flow**

### kick-listener — PARTIALLY WIRED (local package)
- ✅ `SetSocketConnected` — websocket/client.go (local: `kick_listener_socket_state`)
- ✅ `IncMessage`, `ObservePublishLatency` — cmd/main.go (local: `kick_listener_messages_total`, `kick_listener_publish_latency_seconds`)
- ✅ `ObserveSubscription`, `SetActiveSubscriptions` — channels/manager.go (local: `kick_listener_subscription_events_total`)
- ❌ `RecordMessage` (shared/) — not wired (local package substitutes)
- Status: **Local package covers core; do NOT add shared/ on top**

### youtube-listener — MINIMAL WIRING
- ✅ `SetQuotaRemaining`, `SetQuotaUsagePercent` — quota/tracker.go
- ❌ `RecordMessage`, `RecordPublish`, `RecordConnection`, `RecordAPICall` — streams/poller.go, publisher/redis.go
- Status: **Message flow and API calls completely missing**

### tiktok-listener — WELL WIRED (prom-client, own package)
- ✅ `heartbeatTimeouts`, `heartbeatLastMessage` — connection health
- ✅ `messagesQueued`, `messagesDropped`, `messageQueueSize` — message processing
- ✅ `circuitBreakerState`, `circuitBreakerTrips` — circuit breaker
- ✅ `pooledConnections`, `connectionSubscribers` — connection pooling
- ✅ `errorsByType` — error classification
- ✅ `backoffCurrentInterval`, `detectionSkippedTotal` — backoff/detection
- ✅ `/metrics` HTTP endpoint — src/index.ts
- Status: **Complete — no wiring work needed**

### discord-listener — PARTIAL (own package, only 4 metrics)
- ✅ `discord_listener_gateway_events_total` — IncGatewayEvent()
- ✅ `discord_listener_active_guilds` — SetActiveGuilds()
- ✅ `discord_listener_shard_ownership` — SetShardOwnership()
- ✅ `discord_listener_resume_attempts_total` — IncResumeAttempt()
- ❌ Messages received, messages published, errors — not wired
- Status: **Connection health covered; message flow missing**

### youtube-listener-innertube — WELL WIRED (own package)
- ✅ `youtube_listener_errors_total` — error tracking
- ✅ `youtube_listener_requests_total` — API requests
- ✅ `youtube_listener_messages_published_total` — message publishing
- ✅ `youtube_listener_redis_publish_attempts_total`, `redis_publish_success_total`, `redis_publish_latency_seconds` — Redis health
- ✅ `youtube_listener_reconnections_total` — reconnection tracking
- ✅ `youtube_listener_deletion_buffer_overflows_total` — deletion buffer
- Status: **Well wired; MISSING from ServiceMonitor**

### twitch-eventsub-listener — ENDPOINT ONLY
- ✅ `/metrics` endpoint registered in cmd/main.go
- ❌ Zero RecordX() calls anywhere in service
- Status: **Zero wiring — highest priority gap for a listener**

### message-processor — WELL WIRED
- ✅ `RecordMessageConsumed` — consumer/stream_consumer.go
- ✅ `RecordMessageProcessed` — multiple stages (normalized, enriched, deduplicated, filtered)
- ✅ `RecordMessagePublished` — per overlay/platform
- ❌ `RecordEmoteLookup`, `RecordEmoteCacheOperation` — enricher not wired
- ❌ `SetStreamLag`, `RecordStreamError` — stream health not wired
- Status: **Pipeline stages covered; emote enrichment and stream lag missing**

### api-gateway — PARTIAL
- ✅ `RecordWebSocketConnectionAttempt` — on connect
- ✅ `RecordWebSocketConnection` — +1/-1 on connect/disconnect
- ✅ `RecordOverlaySubscription` — +1/-1 on subscribe/unsubscribe
- ❌ `RecordMessageReceived` — Redis pub/sub receive not wired
- ❌ `RecordMessageSent` — WebSocket send not wired
- ❌ `RecordMessageDropped` — not wired
- ❌ `RecordHTTPRequest` — HTTP middleware not wired
- Status: **Connection counting works; message delivery telemetry missing**

### auth-service, overlay-manager, token-refresh-service, emote-service — ENDPOINT ONLY
- ✅ `/metrics` endpoint registered
- ❌ Zero custom RecordX() calls
- Status: **Low priority for message flow; business metrics and error tracking completely missing**

### source-manager — SHARDING WIRED
- ✅ Shard metrics via `sharedmetrics.NewShardMetrics()` — coordination/load_monitor.go, coordinator.go, migration_publisher.go
- ❌ Business metrics (active leases, assignment counts) — not wired
- Status: **Sharding observability complete; coordination metrics missing**

---

## Provisioning Mechanism (Confirmed)

The Grafana Helm chart is configured with TWO dashboard delivery mechanisms (both active):

1. **`dashboardsConfigMaps.default: allchat-grafana-dashboards`** — Static mount of the named ConfigMap into `/var/lib/grafana/dashboards/default`. Dashboards are loaded at Grafana startup. Changes require pod restart or file refresh.

2. **Sidecar with `label: grafana_dashboard`** — Dynamic hot-reload. Any ConfigMap with `grafana_dashboard: "1"` label is loaded without restart.

The existing `allchat-grafana-dashboards.yaml` ConfigMap does NOT have `grafana_dashboard: "1"` label — it uses the static mount approach via `dashboardsConfigMaps`. New dashboards should be added as new JSON keys in the same ConfigMap. No ConfigMap label change needed.

For alerts: The sidecar picks up `grafana_alert: "1"` labeled ConfigMaps (hot-reload). The alerts ConfigMap already has this label. Appending new rules to `allchat-alerts.yaml` triggers hot-reload.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual prometheus.MustRegister | promauto.NewCounterVec | Set up in shared/metrics/ | Cannot accidentally forget to register |
| Hardcoded scrape_configs in prometheus.yml | ServiceMonitor CRD | kube-prometheus-stack | Add label `app: my-service` to pod + ServiceMonitor entry |
| Dashboard JSON uploaded via Grafana UI | ConfigMap provisioning | kube-prometheus-stack | Dashboards are version-controlled, reviewable in PRs |
| One large monolithic dashboard | Tiered dashboards by audience | This phase | Operations team can focus on relevant panels |

**Deprecated/outdated:**
- The `docs/METRICS_ROLLOUT_COMPLETE.md` status table lists 9 services — it predates discord-listener, innertube, and twitch-eventsub additions
- The prometheus scrape config example in `shared/metrics/README.md` uses `static_configs` — the production setup uses ServiceMonitor CRDs instead

---

## Open Questions

1. **Live Prometheus audit: are the four "wired" services actually scraped successfully?**
   - What we know: ServiceMonitor exists for them; RecordX() calls exist in code
   - What's unclear: Whether current pod labels match ServiceMonitor selectors in production
   - Recommendation: Run Grafana MCP query for `up{job=~"allchat-.*"}` during the audit plan step to confirm

2. **discord-listener port for ServiceMonitor**
   - What we know: The allchat-listeners ServiceMonitor uses `port: http` endpoint selector
   - What's unclear: What port name discord-listener's Service uses; need to verify `services/discord-listener/cmd/main.go` port and caesar-deployment Service manifest
   - Recommendation: Read the discord-listener-deployment.yaml in caesar-deployment to confirm the service port label matches `http`

3. **InnerTube metric naming collision with youtube-listener**
   - What we know: InnerTube uses `youtube_listener_*` metric names (e.g., `youtube_listener_messages_published_total`) which are the same prefix as the old youtube-listener's shared/metrics names
   - What's unclear: Whether both services are scraped into the same Prometheus job, which would cause label-value collision
   - Recommendation: Check `label_values(youtube_listener_messages_published_total, service)` in live Prometheus to confirm separation by `service` label

4. **allchat-grafana-dashboards ConfigMap size**
   - What we know: Current file is 3617 lines; adding 5 new dashboards will add ~500-800 lines each
   - What's unclear: Whether Kubernetes has a ConfigMap size limit issue (etcd limit is 1MB)
   - Recommendation: Current file is well within limits (~150KB estimated). Split into separate ConfigMaps only if approaching 900KB.

---

## Validation Architecture

> `workflow.nyquist_validation` key is absent from .planning/config.json — treating as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go test (standard library) + existing `*_test.go` files |
| Config file | none — per-service `go test ./...` |
| Quick run command | `cd services/twitch-eventsub-listener && go test ./...` |
| Full suite command | `make test` (root Makefile) |

### Phase Requirements → Test Map

| Behavior | Test Type | Automated Command | Notes |
|----------|-----------|-------------------|-------|
| Metrics package compiles with new wiring | unit (compile) | `cd shared && go build ./...` | Verifies RecordX() signatures |
| twitch-eventsub metrics register without panic | unit | `cd services/twitch-eventsub-listener && go test ./...` | promauto panics on duplicate |
| discord-listener metrics register without panic | unit | `cd services/discord-listener && go test ./...` | Existing metrics_test.go covers this |
| ServiceMonitor YAML is valid | static | `kubectl --dry-run=client apply -f servicemonitor.yaml` | Schema validation |
| Alert rule YAML is valid JSON/YAML | static | `yamllint allchat-alerts.yaml` | Prevents provisioning failure |

### Wave 0 Gaps

None — existing test infrastructure (per-service `go test ./...`, `make test`) is sufficient. No new test files required before implementation begins.

---

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `shared/metrics/*.go` — all metric type definitions and RecordX() signatures confirmed
- Codebase inspection: `services/*/cmd/main.go` — endpoint wiring confirmed per service
- Codebase inspection: `caesar-deployment/apps/platform/kube-prometheus-stack/` — provisioning mechanism confirmed
- Codebase inspection: `services/kick-listener/metrics/*.go` — local package pattern confirmed
- Codebase inspection: `services/discord-listener/metrics/metrics.go` — 4-metric local package confirmed
- Codebase inspection: `services/tiktok-listener/src/metrics/prometheus.ts` — full prom-client implementation confirmed
- Codebase inspection: `servicemonitor.yaml` — ServiceMonitor coverage gaps confirmed

### Secondary (MEDIUM confidence)
- `docs/METRICS_ROLLOUT_COMPLETE.md` — historical wiring status (some services added after this doc was written)
- `shared/metrics/README.md` — usage patterns confirmed match code
- Helm values inspection — provisioning mechanism confirmed

### Tertiary (LOW confidence)
- Alert threshold values (2min disconnection, 1min pipeline stall, 50% WebSocket drop, 5% error rate) — from CONTEXT.md locked decisions; not empirically validated against production traffic patterns

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries confirmed in codebase
- Gap matrix: HIGH — based on direct code inspection per service
- Architecture patterns: HIGH — provisioning mechanism confirmed from Helm values + existing ConfigMaps
- Alert PromQL: MEDIUM — syntax correct, thresholds not production-validated
- Pitfalls: HIGH — promauto duplicate panic and ServiceMonitor selector mismatch are confirmed failure modes from existing code structure

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable infrastructure, 30-day window)
