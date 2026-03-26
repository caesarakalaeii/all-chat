---
status: diagnosed
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md, 04-04-SUMMARY.md, 04-05-SUMMARY.md]
started: 2026-03-26T20:30:00Z
updated: 2026-03-26T21:10:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Message Flow Metrics Emitting
expected: Query `listener_messages_received_total` in Prometheus — should return results for youtube, twitch_eventsub platforms (newly wired in Plan 02). Also check `gateway_messages_sent_total` exists from api-gateway.
result: pass

### 2. Platform Ops HTTP Metrics Emitting
expected: Query `http_requests_total{service="auth-service"}` in Prometheus — should return non-zero counter. Same for overlay-manager, emote-service, token-refresh-service (all newly wired in Plan 03 via Gin middleware).
result: issue
reported: "youtube-listener-innertube has no shared/metrics wiring — Plan 02 never added RecordConnection/RecordMessage to innertube. The Overview dashboard InnerTube stat panel queries listener_connection_status{platform='youtube_innertube'} which doesn't exist. Local youtube_listener_* metrics are present and the Listeners dashboard InnerTube row uses those correctly. HTTP metrics for auth-service, overlay-manager, emote-service all confirmed working."
severity: major

### 3. Business Metrics Emitting
expected: Query `token_refresh_attempts_total` — should show counters per platform. Query `emote_api_calls_total` — should show counters per provider (7tv, bttv, ffz). Query `allchat_active_sources_total` — should show per-platform gauge from source-manager.
result: pass

### 4. Dashboards Provisioned in Grafana
expected: 5 new dashboards visible in Grafana under the allchat folder: "AllChat Overview", "AllChat Listeners", "AllChat Message Processing", "AllChat Delivery", "AllChat Platform Ops". The 6 old dashboards (Listener Health, Listener Observability, Message Pipeline, Platform Overview, Service Health, YouTube Quota) should be gone.
result: pass

### 5. Overview Dashboard Renders
expected: The AllChat Overview dashboard loads without errors. Traffic-light stat panels show green/red status for all 7 listeners, pipeline services, and platform services. Key metrics row shows total messages/min and active WebSocket connections.
result: issue
reported: "Dashboard provisioned and loads (21 panels, all 7 listeners, pipeline, platform services, key metrics). But the InnerTube stat panel queries listener_connection_status{platform='youtube_innertube'} which doesn't exist — will show 'No data'. Same root cause as Test 2 (innertube missing shared/metrics wiring)."
severity: major

### 6. Listeners Dashboard — All 7 Platforms
expected: The AllChat Listeners dashboard has 7 collapsed rows — one for each listener (Twitch, YouTube, Kick, TikTok, Discord, InnerTube, Twitch EventSub). Expanding a row shows platform-specific panels with live data where available.
result: pass

### 7. Alert Rules Provisioned
expected: Grafana Alerting shows 13 total rules across 6 groups. The 4 new groups are: allchat-listener-health (3 rules), allchat-pipeline-health (1 rule), allchat-delivery-health (2 rules), allchat-error-rates (2 rules). All new rules have `team: allchat` label.
result: issue
reported: "All 13 rules provisioned across 6 groups — structure correct. But pipeline-stall alert is FALSE POSITIVE firing: the PromQL uses per-series rate(processor_messages_consumed_total[5m]) and the platform='system' series has rate=0, triggering the $A == 0 && $B > 0 condition even though actual message processing is happening (twitch: 0.14/s, tiktok: 0.10/s, youtube: 0.05/s). Needs sum(rate(...)) instead of per-series evaluation."
severity: major

### 8. Alert Routing to Discord
expected: Alert notification policies route `team: allchat` rules to the discord-allchat contact point. Critical severity alerts should include @lead-dev mention per policy config.
result: pass

## Summary

total: 8
passed: 5
issues: 3
pending: 0
skipped: 0

## Gaps

- truth: "All platform ops services emit metrics via shared/metrics and Gin middleware"
  status: failed
  reason: "youtube-listener-innertube has no shared/metrics wiring — Plan 02 never added RecordConnection/RecordMessage. Overview dashboard InnerTube stat panel queries listener_connection_status{platform='youtube_innertube'} which doesn't exist. Local youtube_listener_* metrics work fine."
  severity: major
  test: 2
  root_cause: "Plan 02 explicitly excluded innertube from shared/metrics wiring (CRITICAL comment in plan: importing shared/metrics causes promauto duplicate registration panic with local metrics package). But no task was created to adapt the Overview dashboard panel to use local metrics instead."
  artifacts:
    - path: "../caesar-deployment/apps/platform/kube-prometheus-stack/dashboards/allchat-grafana-dashboards.yaml"
      issue: "Overview dashboard InnerTube stat panel queries listener_connection_status{platform='youtube_innertube'} which cannot exist"
  missing:
    - "Change Overview dashboard InnerTube panel query to: sum(rate(youtube_listener_messages_published_total{service='youtube-listener-innertube-canary'}[5m])) — non-zero when innertube is actively publishing"
  debug_session: ""

- truth: "Overview dashboard InnerTube stat panel shows connection status"
  status: failed
  reason: "Same root cause as Test 2 — listener_connection_status{platform='youtube_innertube'} doesn't exist because shared/metrics never wired into innertube service."
  severity: major
  test: 5
  root_cause: "Same as gap 1 — Overview dashboard panel uses wrong metric name for innertube. Fix is dashboard-only (no code change needed)."
  artifacts:
    - path: "../caesar-deployment/apps/platform/kube-prometheus-stack/dashboards/allchat-grafana-dashboards.yaml"
      issue: "allchat-overview.json InnerTube panel queries non-existent metric"
  missing:
    - "Change InnerTube stat panel query and value mappings to use youtube_listener_messages_published_total rate"
  debug_session: ""

- truth: "pipeline-stall alert fires only when no messages are being processed"
  status: failed
  reason: "pipeline-stall uses per-series rate(processor_messages_consumed_total[5m]) == 0 but platform='system' series is always 0, causing false positive. Needs sum(rate(...)) to aggregate across platforms before comparing."
  severity: major
  test: 7
  root_cause: "Ref A query returns a vector (one series per platform label). The math expression $A == 0 && $B > 0 evaluates per-series. The platform='system' series always has rate=0, triggering the alert despite active message processing on other platforms. Fix: wrap in sum() to collapse to scalar."
  artifacts:
    - path: "../caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml"
      issue: "pipeline-stall Ref A: rate(processor_messages_consumed_total[5m]) returns vector, needs sum() wrapping"
  missing:
    - "Change Ref A expr from rate(processor_messages_consumed_total[5m]) to sum(rate(processor_messages_consumed_total[5m]))"
  debug_session: ""
