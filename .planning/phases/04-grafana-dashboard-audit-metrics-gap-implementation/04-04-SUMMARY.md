---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: 04
subsystem: grafana-dashboards
tags: [grafana, dashboards, observability, prometheus, json]
dependency_graph:
  requires: ["04-02", "04-03"]
  provides: ["DASH-01", "DASH-02", "DASH-03", "DASH-04", "DASH-05"]
  affects: ["grafana-grafana", "04-05-alerting"]
tech_stack:
  added: []
  patterns:
    - "Collapsed row panels in Grafana JSON with sub-panels array for compact listener dashboard"
    - "Traffic-light stat panels using thresholds mode=absolute steps for service health overview"
    - "topk(10, ...) guard for high-cardinality overlay_id labels"
key_files:
  created: []
  modified:
    - ../caesar-deployment/apps/platform/kube-prometheus-stack/dashboards/allchat-grafana-dashboards.yaml
decisions:
  - "Replaced all 6 existing dashboards (not just 4 as originally written) — listener-observability and service-health were also stale/redundant; 5 new tiered dashboards cover all their content with better organization"
  - "Listeners dashboard uses collapsed rows with sub-panels array to keep 7 listeners manageable in Grafana UI without horizontal scrolling"
  - "datasourceUid uses lowercase 'prometheus' (not 'Prometheus') to match the Prometheus datasource UID registered by kube-prometheus-stack"
  - "TikTok connection health uses tiktok_listener_connection_subscribers > 0 (Node.js prom-client metric) — no shared/metrics ListenerMetrics for Node services"
  - "InnerTube row queries youtube_listener_* metric names (local package names from 04-GAP-MATRIX.md audit) rather than shared/metrics names"
  - "YouTube quota monitoring incorporated into Listeners dashboard YouTube row — no separate quota dashboard needed"
metrics:
  duration: "362s (~6 min)"
  completed_date: "2026-03-26"
  tasks_completed: 2
  files_modified: 1
---

# Phase 04 Plan 04: Grafana Dashboards Summary

5 tiered Grafana dashboards replacing 6 stale ones: Overview traffic-light grid, Listeners (all 7 platforms), Message Processing pipeline, Delivery (api-gateway WebSocket), and Platform Ops (HTTP + business metrics).

## What Was Built

### Task 1: Overview and Listeners Dashboards

**allchat-overview.json** (uid: `allchat-overview-v2`):
- Row 1 "Listeners": 7 stat panels — Twitch (listener_connection_status), YouTube (listener_connection_status), Kick (kick_listener_socket_state), TikTok (tiktok_listener_connection_subscribers), Discord (discord_listener_shard_ownership), InnerTube (listener_connection_status{platform="youtube_innertube"}), Twitch EventSub (listener_connection_status{platform="twitch-eventsub"})
- Row 2 "Pipeline": processor (rate(processor_messages_consumed_total) > 0), gateway (gateway_websocket_connections_active)
- Row 3 "Platform Services": 5 stat panels using up{job=~".*<service>.*"} for auth-service, overlay-manager, source-manager, token-refresh, emote-service
- Row 4 "Key Metrics": Total messages/min (sum across all listener platforms), active WebSocket connections, active sources

**allchat-listeners.json** (uid: `allchat-listeners-v2`):
- 7 collapsed rows, one per listener
- Twitch: received/published rate timeseries, connection status stat, active sources stat
- YouTube: received/published rate, quota usage %, API calls rate, connection status stat
- Kick: socket state stat, messages rate, publish latency p95, subscription events rate
- TikTok: messages queued rate, messages dropped stat, circuit breaker state stat, connection subscribers stat
- Discord: gateway events rate by event_type, messages received rate, active guilds stat, shard ownership stat
- InnerTube: messages published rate, errors rate by error_type, reconnections stat
- Twitch EventSub: messages received rate, connection status stat, messages published rate

### Task 2: Message Processing, Delivery, and Platform Ops Dashboards

**allchat-message-processing.json** (uid: `allchat-message-processing-v2`):
- Row "Pipeline Flow": consumed/processed/published rates by platform/stage, processing duration p95
- Row "Emote Enrichment": emote lookups rate by provider+result, cache entries by provider, enrichment duration p95
- Row "Stream Health": stream lag, stream errors rate, deletion buffer buffered/applied rates

**allchat-delivery.json** (uid: `allchat-delivery-v2`):
- Row "WebSocket Connections": active connections gauge, connection rate by result, connection duration p50
- Row "Message Delivery": messages received from Redis by platform, messages sent by result, messages dropped by reason, delivery latency p95
- Row "Overlay Subscriptions": topk(10) active subscriptions, subscription events rate

**allchat-platform-ops.json** (uid: `allchat-platform-ops-v2`):
- Row "HTTP Traffic": request rate by service, error rate by service (5xx %), request duration p95 by service
- Row "Token Refresh": refresh attempts by platform+result, refresh errors by platform+error_category
- Row "Source Management": active sources by platform, source operations rate
- Row "Emote Service": emote API calls by provider+result

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1+2 | `3ef6174` (caesar-deployment) | feat(04-04): replace 6 stale dashboards with 5 tiered all-chat dashboards |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Replaced 6 existing dashboards instead of 4**
- **Found during:** Task 1 analysis of existing file
- **Issue:** Plan stated to replace 4 dashboards but the actual ConfigMap contained 6 (also allchat-listener-observability.json and allchat-service-health.json not mentioned in plan). The acceptance criteria required exactly 5 keys, confirming all old ones should be replaced.
- **Fix:** Removed all 6 existing dashboard keys and replaced with the 5 new tiered ones
- **Files modified:** allchat-grafana-dashboards.yaml
- **Commit:** 3ef6174

## Verification

```
Dashboard keys found: ['allchat-overview.json', 'allchat-listeners.json', 'allchat-message-processing.json', 'allchat-delivery.json', 'allchat-platform-ops.json']
  allchat-overview.json: uid=allchat-overview-v2, panels=21
  allchat-listeners.json: uid=allchat-listeners-v2, panels=7
  allchat-message-processing.json: uid=allchat-message-processing-v2, panels=13
  allchat-delivery.json: uid=allchat-delivery-v2, panels=12
  allchat-platform-ops.json: uid=allchat-platform-ops-v2, panels=12
Total: 5 dashboards
All dashboard JSON valid
All acceptance criteria PASSED
```

All panels use `"uid": "prometheus"` (lowercase) datasource. Old keys confirmed removed.

## Self-Check: PASSED

- /home/moersener/Hobby/caesar-deployment/apps/platform/kube-prometheus-stack/dashboards/allchat-grafana-dashboards.yaml: FOUND (modified)
- Commit 3ef6174: FOUND in caesar-deployment git log

---
*Phase: 04-grafana-dashboard-audit-metrics-gap-implementation*
*Completed: 2026-03-26*
