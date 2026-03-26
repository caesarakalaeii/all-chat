# Phase 04: Confirmed Gap Matrix — Prometheus Metrics & Dashboard Coverage

**Produced:** 2026-03-26
**Method:** Code audit (grep-based, from 04-RESEARCH.md) + ServiceMonitor + Dashboard analysis
**Live Prometheus Status:** Cluster unreachable during audit — live verification pending (see Task 3 checkpoint)

> NOTE: The scrape status column reflects expected state after ServiceMonitor fix in Task 1 of this plan.
> Live confirmation of actual UP status should be verified via Grafana after the ServiceMonitor change rolls out.

---

## Scrape Status

Which services Prometheus is configured to scrape (post-Task-1 ServiceMonitor fix):

| Service | ServiceMonitor | Expected Scrape | Notes |
|---------|---------------|-----------------|-------|
| twitch-listener | allchat-listeners | Expected UP | Was in SM before |
| kick-listener | allchat-listeners | Expected UP | Was in SM before |
| tiktok-listener | allchat-listeners | Expected UP | Was in SM before; Node.js prom-client |
| youtube-listener | allchat-listeners | Expected UP | Was in SM before (covers classic HTTP poller) |
| discord-listener | allchat-listeners | Expected UP (new) | Added in Task 1; Service has `app: discord-listener` label |
| youtube-listener-innertube | allchat-listeners | Expected UP (new) | Added in Task 1; Service label `app: youtube-listener-innertube` added |
| twitch-eventsub-listener | allchat-listeners | Expected UP (new) | Added in Task 1; Service has `app: twitch-eventsub-listener` label |
| message-processor | allchat-services | Expected UP | Was in SM before |
| api-gateway | allchat-services | Expected UP | Was in SM before |
| auth-service | allchat-services | Expected UP | Was in SM before |
| overlay-manager | allchat-services | Expected UP | Was in SM before |
| emote-service | allchat-services | Expected UP | Was in SM before |
| token-refresh-service | allchat-services | Expected UP | Was in SM before |
| source-manager | allchat-source-manager | Expected UP | Separate SM; was before |

**Total scraped:** 14 services (was 11 before Task 1 added 3 new listeners)

---

## Metrics Emission

Whether code actually calls `RecordX()` or equivalent at runtime. Three-way status:

- **WIRED** — RecordX() calls exist in domain logic; metric will have non-zero values
- **LOCAL** — service has its own metrics package (not shared/metrics/) with some wiring; inconsistent naming
- **ENDPOINT ONLY** — `/metrics` endpoint exposed but no RecordX() calls in any domain file
- **MISSING** — no metrics endpoint or no calls at all (unlikely given all services use promhttp)

| Service | Connection Status | Messages Rx | Messages Pub | API Calls | Errors | Special Metrics |
|---------|------------------|-------------|--------------|-----------|--------|-----------------|
| twitch-listener | WIRED (`RecordConnection`) | WIRED (`RecordMessage`) | WIRED (`RecordPublish`) | ENDPOINT ONLY | ENDPOINT ONLY | `SetActiveSources` WIRED |
| kick-listener | LOCAL (`kick_listener_socket_state`) | ENDPOINT ONLY | LOCAL (`kick_listener_messages_published_total`) | ENDPOINT ONLY | ENDPOINT ONLY | Local pkg; dropped msgs LOCAL |
| youtube-listener | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | Quota metrics WIRED (youtube_quota_*) |
| tiktok-listener | LOCAL (prom-client) | LOCAL (prom-client) | LOCAL (prom-client) | ENDPOINT ONLY | LOCAL (prom-client) | Full prom-client setup; own metric names |
| discord-listener | LOCAL (`discord_gateway_events_total`) | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | Active guilds, shard ownership, resume events LOCAL |
| youtube-listener-innertube | ENDPOINT ONLY | LOCAL (`youtube_innertube_messages_published_total`) | LOCAL (same pkg) | LOCAL (api calls pkg) | LOCAL (error counter) | Own metrics pkg; not shared/; batch detection LOCAL |
| twitch-eventsub-listener | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | Zero RecordX() calls anywhere |
| message-processor | N/A | WIRED (`processor_messages_consumed_total`) | WIRED (`processor_messages_published_total`) | N/A | ENDPOINT ONLY | Normalization + enrichment stage timers WIRED |
| api-gateway | LOCAL (connect/disconnect) | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | ENDPOINT ONLY | `gateway_websocket_connections_active` LOCAL; no per-message counters |
| auth-service | N/A | N/A | N/A | ENDPOINT ONLY | ENDPOINT ONLY | Zero custom wiring |
| overlay-manager | N/A | N/A | N/A | ENDPOINT ONLY | ENDPOINT ONLY | Zero custom wiring |
| source-manager | N/A | N/A | N/A | ENDPOINT ONLY | ENDPOINT ONLY | Shard metrics WIRED (`source_manager_*`) |
| token-refresh-service | N/A | N/A | N/A | ENDPOINT ONLY | ENDPOINT ONLY | Zero custom wiring |
| emote-service | N/A | N/A | N/A | ENDPOINT ONLY | ENDPOINT ONLY | Zero custom wiring |

### Wiring Gaps by Priority (message flow first)

**P1 — Message flow pipeline (blocks end-to-end visibility):**
| Gap | Service | Metric needed | Action |
|-----|---------|---------------|--------|
| No messages-received counter | kick-listener | `listener_messages_received_total{platform="kick"}` | Wire RecordMessage() via shared/metrics |
| No messages-received counter | youtube-listener | `listener_messages_received_total{platform="youtube"}` | Wire RecordMessage() |
| No messages-received counter | discord-listener | `listener_messages_received_total{platform="discord"}` | Wire RecordMessage() via shared/metrics |
| No messages-received counter | twitch-eventsub-listener | `listener_messages_received_total{platform="twitch_eventsub"}` | Wire all RecordX() calls |
| No connection status | youtube-listener | `listener_connection_status{platform="youtube"}` | Wire RecordConnection() |
| No connection status | youtube-listener-innertube | `listener_connection_status{platform="youtube_innertube"}` | Wire RecordConnection() |
| No connection status | twitch-eventsub-listener | `listener_connection_status{platform="twitch_eventsub"}` | Wire RecordConnection() |
| No per-message metrics | api-gateway | `gateway_messages_delivered_total` | Wire message delivery counter |
| No error counters | message-processor | `processor_errors_total` | Wire error recording |

**P2 — Connection health (enables listener disconnect alerting):**
| Gap | Service | Action |
|-----|---------|--------|
| Kick uses local `kick_listener_socket_state` not shared/ | kick-listener | Acceptable (keep local; dashboard query adapts) |
| Discord gateway events in local pkg not shared/ | discord-listener | Acceptable (keep local; expose via /metrics) |
| InnerTube metrics in local pkg not shared/ | youtube-listener-innertube | Acceptable (keep local; expose via /metrics) |

**P3 — Platform ops (lowest priority):**
| Gap | Services | Action |
|-----|---------|--------|
| Zero custom metrics | auth-service, overlay-manager, token-refresh-service, emote-service | Wire HTTP request counters + error rates via standard Go HTTP middleware |

---

## Dashboard Gaps

### Existing Dashboards (6 total in `allchat-grafana-dashboards.yaml`)

| Dashboard | Key | Has Discord | Has InnerTube | Has twitch-eventsub | Status |
|-----------|-----|-------------|---------------|---------------------|--------|
| All-Chat Listener Health | `allchat-listener-health.json` | NO | NO | NO | Stale — missing 3 new listeners |
| AllChat Listener Observability | `allchat-listener-observability.json` | NO | NO | NO | Stale — missing 3 new listeners |
| All-Chat Message Pipeline | `allchat-message-pipeline.json` | Partial | Partial | NO | Mostly OK; no platform breakdown |
| All-Chat Platform Overview | `allchat-platform-overview.json` | Via deployment | Via deployment | Via deployment | OK — uses `up` metric |
| AllChat Service Health | `allchat-service-health.json` | Via deployment | Via deployment | Via deployment | OK — uses deployment availability |
| All-Chat YouTube Quota Monitoring | `allchat-youtube-quota.json` | N/A | N/A | N/A | OK — YouTube-specific |

### Existing Panel Inventory (key panels)

**allchat-listener-health.json** (covers: Twitch, YouTube, Kick only):
- Twitch Connection Status / Active Channels / Message Rate / Latency P95
- YouTube Quota Usage / Remaining / Active Streams
- Kick Connection Status / Active Channels / Message Rate / Latency
- Connection Health Matrix (cross-platform)

**allchat-listener-observability.json** (covers: Kick only with detail):
- Kick Socket State / Publish p95 / Message Rate / Dropped Messages / Reconnect Attempts
- Source Manager Active Leases / Leadership Events

**allchat-message-pipeline.json** (covers: pipeline stages):
- Message Funnel (End-to-End)
- Processing Stage Duration P95
- Normalization/Enrichment/Publish Success Rate
- Stream Consumption Errors / Messages Dropped

**allchat-platform-overview.json** (covers: all services via `up`):
- Active Overlays / WebSocket Connections
- YouTube Quota / Sources by Platform
- Message Rate by Platform / Listener Status
- P95 Latency by Service / Error Rate by Service

**allchat-service-health.json** (covers: all deployments):
- Ready/Not Ready Pods / Restarts (6h)
- CPU Usage / Memory Usage / HTTP 5xx Rate

**allchat-youtube-quota.json** (covers: YouTube quota depth):
- Quota Usage % / Remaining / Limit / Trend
- API Calls by Operation / Duration P95
- Rate Limit Hits / Time to Exhaustion
- Circuit Breaker Status / Quota Saved

### Stale or Broken Panels (likely broken without live confirmation)

| Panel | Dashboard | Issue | Severity |
|-------|-----------|-------|----------|
| Kick panels (all) | allchat-listener-health | Uses `kick_listener_*` names; correct but non-standard | LOW |
| "Active Twitch Channels" | allchat-listener-health | Uses `listener_active_sources` — WIRED; should work | OK |
| "Active YouTube Streams" | allchat-listener-health | Uses YouTube-specific quota metrics — WIRED; should work | OK |
| Any Discord panel | allchat-listener-health | None exists — gap | HIGH |
| Any InnerTube panel | allchat-listener-health | None exists — gap | HIGH |
| Any twitch-eventsub panel | allchat-listener-health | None exists — gap | HIGH |

### New Dashboards Needed (per Phase 4 plan)

Per the tiered dashboard strategy from CONTEXT.md:

| Dashboard | Plan | Services covered | Priority |
|-----------|------|-----------------|---------|
| Listeners (all 7) | Plan 03 | twitch, kick, youtube, tiktok, discord, innertube, eventsub | HIGH |
| Message Processing | Plan 04 | message-processor, emote-service | MEDIUM |
| Delivery | Plan 04 | api-gateway (WebSocket) | MEDIUM |
| Platform Ops | Plan 05 | auth, overlay-manager, source-manager, token-refresh | LOW |

---

## Wiring Work Plan (feeds Plans 02-05)

### Plan 02: Wire missing RecordX() calls in Go listeners

Services requiring new wiring:
1. **kick-listener** — add `RecordMessage()` for messages received; existing local pkg handles publish + socket state
2. **youtube-listener** — add `RecordConnection()`, `RecordMessage()`, `RecordPublish()`
3. **discord-listener** — add `RecordMessage()` via shared/metrics; existing local pkg keeps gateway events
4. **twitch-eventsub-listener** — add full wiring: connection, messages received, messages published
5. **youtube-listener-innertube** — add `RecordConnection()` via shared/metrics; keep existing local pkg for innertube-specific metrics
6. **api-gateway** — add per-message delivery counter

### Plan 03: Listener dashboards

Update `allchat-listener-health.json` and `allchat-listener-observability.json` to add panels for:
- Discord: gateway events, active guilds, shard ownership
- InnerTube: messages published, error rate, API calls
- twitch-eventsub: webhook events, connection status, messages received

### Plan 04: Pipeline + Delivery dashboards

Add to `allchat-message-pipeline.json`:
- Per-platform message breakdown
- Gateway delivery counter panel

### Plan 05: Alerting

Add to `allchat-alerts.yaml`:
1. Listener disconnection alert (all 7 listeners)
2. Message pipeline stall alert (processor + gateway)
3. WebSocket connection drop alert
4. Error rate spike alert

---

## Confirmed Facts vs Research Assumptions

| Claim from RESEARCH.md | Confirmed? | Notes |
|-----------------------|------------|-------|
| All services expose `/metrics` | CONFIRMED via code (promhttp.Handler in every cmd/main.go) | |
| Twitch-listener fully wired | CONFIRMED via code grep (RecordConnection, RecordMessage, RecordPublish found) | |
| youtube-listener quota only | CONFIRMED via code grep (quota/tracker.go has RecordX calls, streams/ does not) | |
| message-processor pipeline wired | CONFIRMED via code grep (consumer/stream_consumer.go has RecordX calls) | |
| kick-listener local pkg | CONFIRMED via code grep (kick_listener_* names in websocket/client.go) | |
| discord-listener local pkg | CONFIRMED via code grep (gateway/client.go has metrics calls; local pkg) | |
| innertube local pkg | CONFIRMED via code grep (innertube/client.go, streams/manager.go have local metrics calls) | |
| twitch-eventsub zero wiring | CONFIRMED via code grep (no RecordX() calls anywhere) | |
| tiktok-listener Node.js prom-client | CONFIRMED via dist/index.js metrics calls | |
| api-gateway connect/disconnect only | CONFIRMED via websocket/manager.go and health.go | |
| SM missing 3 listeners | CONFIRMED (was missing; fixed in Task 1) | |
| 6 dashboards exist | CONFIRMED via allchat-grafana-dashboards.yaml (6 JSON keys) | |
| No Discord/InnerTube/eventsub panels | CONFIRMED — none found in panel title scan | |
