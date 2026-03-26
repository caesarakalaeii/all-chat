---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: 05
subsystem: alerting
tags: [grafana, alerting, prometheus, kubernetes, discord-notifications]
dependency_graph:
  requires: ["04-02", "04-03"]
  provides: ["listener-disconnection-alerts", "pipeline-stall-alerts", "websocket-drop-alerts", "error-rate-alerts", "ALERT-01", "ALERT-02", "ALERT-03", "ALERT-04", "ALERT-05"]
  affects: ["grafana-alert-evaluation", "discord-notifications"]
tech_stack:
  added: []
  patterns:
    - "Grafana alert rules as Kubernetes ConfigMap with grafana_alert: 1 label for auto-provisioning"
    - "Two-query alert pattern: PromQL query (refId A) + threshold expression (refId B)"
    - "Three-query pipeline stall: rate query (A) + active listener count (B) + math condition (C: $A == 0 && $B > 0)"
    - "noDataState: OK for all new rules — avoids false positive noise during quiet periods"
key_files:
  created: []
  modified:
    - ../caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml
key-decisions:
  - "listener-disconnected-shared uses listener_connection_status == 0 threshold pattern with type: lt evaluator against 0 — fires when metric equals 0 (disconnected) for any service using shared metrics"
  - "kick uses separate kick_listener_socket_state rule because it has its own local metric package rather than shared/metrics"
  - "discord uses separate discord_listener_shard_ownership rule for same reason as kick"
  - "pipeline-stall uses math expression type ($A == 0 && $B > 0) not threshold — requires multi-condition logic unavailable in threshold evaluator"
  - "websocket-connections-zero has severity: warning not critical — zero connections during off-stream hours is not an emergency; critical is for drop >50%"
  - "error-rate rules use execErrState: OK — if PromQL query errors (e.g. metric doesn't exist), do not alert; avoid noise from newly deployed services"
  - "All rules noDataState: OK — avoids false positive when platform is down and no metrics are being scraped"
  - "operator: type: and added to threshold conditions to match existing rule structure exactly"
metrics:
  duration: "310s (~5 min)"
  completed_date: "2026-03-26"
  tasks_completed: 2
  files_modified: 1
---

# Phase 04 Plan 05: Grafana Alerting Rules Summary

4 new alert groups with 8 rules added to allchat-alerts.yaml ConfigMap covering listener disconnections, pipeline stalls, WebSocket drops, and error rate spikes — all with inline kubectl remediation steps and correct severity routing to Discord.

## What Was Built

### Task 1: Listener Disconnection and Pipeline Stall Alerts

**allchat-listener-health group (3 rules):**

- `listener-disconnected-shared` — fires when `listener_connection_status == 0` for any service using shared metrics (Twitch, YouTube, twitch-eventsub). Severity: critical. `for: 2m`.
- `listener-disconnected-kick` — fires when `kick_listener_socket_state == 0`. Kick uses its own local metrics package so requires a separate rule. Severity: critical. `for: 2m`.
- `listener-disconnected-discord` — fires when `discord_listener_shard_ownership == 0`. Discord uses local metrics package; shard ownership losing means no Gateway connection. Severity: critical. `for: 2m`.

**allchat-pipeline-health group (1 rule):**

- `pipeline-stall` — fires when `rate(processor_messages_consumed_total[5m]) == 0` while `sum(listener_connection_status) + sum(kick_listener_socket_state) + sum(discord_listener_shard_ownership) > 0`. Uses `type: math` expression `$A == 0 && $B > 0`. Severity: critical. `for: 1m`.

### Task 2: WebSocket Drop and Error Rate Spike Alerts

**allchat-delivery-health group (2 rules):**

- `websocket-connections-zero` — fires when `sum(gateway_websocket_connections_active) < 1`. Severity: warning (off-stream hours may legitimately have zero). `for: 2m`.
- `websocket-connections-drop` — fires when `(current - 5m_ago) / 5m_ago < -0.5` (>50% drop). `execErrState: OK` since offset query errors when no baseline exists. Severity: critical. `for: 2m`.

**allchat-error-rates group (2 rules):**

- `error-rate-spike-listeners` — fires when `rate(listener_errors_total[5m]) / rate(listener_messages_received_total[5m]) > 0.05` per service. `execErrState: OK` to avoid noise from services without listener_errors_total. Severity: warning. `for: 3m`.
- `error-rate-spike-http` — fires when `rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05` per service. Severity: warning. `for: 3m`.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1+2 | `62df5fa` | feat(04-05): add 4 new alert groups to allchat-alerts.yaml |

Note: Tasks 1 and 2 both modify the same single file (allchat-alerts.yaml) and were written together atomically. A single commit covers both tasks.

## Alert Rule Summary

| Group | Rule UID | Severity | For | Metric |
|-------|----------|----------|-----|--------|
| allchat-listener-health | listener-disconnected-shared | critical | 2m | listener_connection_status |
| allchat-listener-health | listener-disconnected-kick | critical | 2m | kick_listener_socket_state |
| allchat-listener-health | listener-disconnected-discord | critical | 2m | discord_listener_shard_ownership |
| allchat-pipeline-health | pipeline-stall | critical | 1m | processor_messages_consumed_total + connection metrics |
| allchat-delivery-health | websocket-connections-zero | warning | 2m | gateway_websocket_connections_active |
| allchat-delivery-health | websocket-connections-drop | critical | 2m | gateway_websocket_connections_active (offset) |
| allchat-error-rates | error-rate-spike-listeners | warning | 3m | listener_errors_total / listener_messages_received_total |
| allchat-error-rates | error-rate-spike-http | warning | 3m | http_requests_total{status=~"5.."} / http_requests_total |

**Existing rules preserved:** youtube-quota-critical, youtube-quota-exhausted, youtube-quota-high, pod-crashloop, pod-not-running (5 rules unchanged).

**Total:** 13 unique UIDs across 6 alert groups.

## Deviations from Plan

None — plan executed exactly as written.

The plan-specified YAML for each rule was followed exactly. One structural addition was made: `operator: type: and` was included in threshold conditions to match the exact structure of existing rules in the file (the plan's abbreviated examples omitted this field but the existing rules have it). This is not a deviation — it's correct conformance to the existing schema.

## Notification Routing

All new rules carry `team: allchat` label which routes to the `discord-allchat` contact point via `allchat-notification-policies.yaml`. `severity: critical` rules will trigger with `@lead-dev` mention (per policy config). `severity: warning` rules route to channel without mention.

## Self-Check: PASSED

- `/home/moersener/Hobby/caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml` — FOUND, 6 groups, 13 UIDs verified via python3 yaml parse
- Commit `62df5fa` — FOUND in caesar-deployment repo git log
- All acceptance criteria validated: group names, rule UIDs, `for` durations, severity labels, `team: allchat` labels, kubectl commands in descriptions, existing rules preserved
