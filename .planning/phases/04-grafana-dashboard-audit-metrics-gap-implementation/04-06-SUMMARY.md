---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: 06
subsystem: observability
tags: [grafana, dashboards, alerting, gap-closure]
gap_closure: true
dependency_graph:
  requires: ["04-04", "04-05"]
  provides: ["DASH-01-fix", "ALERT-03-fix"]
  affects: []
key_files:
  created: []
  modified:
    - ../caesar-deployment/apps/platform/kube-prometheus-stack/dashboards/allchat-grafana-dashboards.yaml
    - ../caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml
decisions:
  - "InnerTube panel uses rate-based query on local youtube_listener_messages_published_total instead of non-existent shared listener_connection_status — importing shared/metrics into innertube causes promauto panic"
  - "pipeline-stall Ref A wrapped in sum() to aggregate per-platform series to scalar before comparison"
requirements-completed: [DASH-01, ALERT-03]
metrics:
  duration: "120s (~2 min)"
  completed_date: "2026-03-26"
  tasks_completed: 2
  files_modified: 2
---

# Phase 04 Plan 06: Gap Closure — Dashboard + Alert Fixes

**Closes UAT gaps from Tests 2, 5, 7 with two YAML edits.**

## What Was Fixed

### Task 1: InnerTube Overview Dashboard Panel (Tests 2, 5)
- Changed Overview dashboard InnerTube stat panel (id: 6) query from `listener_connection_status{platform="youtube_innertube"}` (non-existent) to `sum(rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]))`
- Updated thresholds: null=red, 0.001=green (any non-zero rate = connected)
- Updated mappings: range-based (0=Disconnected, >0.001=Connected)

### Task 2: pipeline-stall Alert False Positive (Test 7)
- Changed Ref A from `rate(processor_messages_consumed_total[5m])` to `sum(rate(processor_messages_consumed_total[5m]))`
- Eliminates false positive from platform="system" series (always rate=0)

## Commits

| Task | Commit | Repo |
|------|--------|------|
| 1 | `22b6a59` | caesar-deployment |
| 2 | `0a590e1` | caesar-deployment |

## Self-Check: PASSED
