---
phase: quick
plan: 260329-u6p
subsystem: metrics, auth-service, monitoring
tags: [prometheus, grafana, auth, metrics, user-growth]
dependency_graph:
  requires: []
  provides: [allchat_user_registrations_total metric, user registrations Grafana dashboard]
  affects: [auth-service, shared/metrics, caesar-deployment monitoring]
tech_stack:
  added: [prometheus/client_golang testutil]
  patterns: [promauto.With(registry) for testable metrics, WithMetrics setter injection]
key_files:
  created:
    - shared/metrics/business_test.go
  modified:
    - shared/metrics/business.go
    - services/auth-service/cmd/main.go
    - services/auth-service/handlers/platform_auth_v2.go
    - services/auth-service/handlers/auth_handler.go
    - ~/git/caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml
decisions:
  - "promauto.With(registry) used in newBusinessMetricsWithRegistry for test isolation — avoids duplicate registration panics in default registry"
  - "WithMetrics setter pattern used on handlers instead of constructor parameter — no call-site changes in main.go beyond wiring"
metrics:
  duration: ~15 minutes
  completed: "2026-03-29"
  tasks_completed: 2
  files_modified: 7
---

# Quick Task 260329-u6p: Add Prometheus Metric for New User Registrations

**One-liner:** `allchat_user_registrations_total` counter with `platform` label wired into all auth-service registration paths, with a 5-panel Grafana dashboard showing growth trends.

## What Was Built

### Task 1: UserRegistrations metric in shared/metrics + auth-service instrumentation

Added `allchat_user_registrations_total` CounterVec (label: `platform`) to `BusinessMetrics`:

- `shared/metrics/business.go`: New `UserRegistrations *prometheus.CounterVec` field, `RecordUserRegistration(platform string)` helper method, and `newBusinessMetricsWithRegistry(reg prometheus.Registerer)` constructor for test isolation. `NewBusinessMetrics()` now delegates to this constructor using `prometheus.DefaultRegisterer`.
- `services/auth-service/handlers/platform_auth_v2.go`: Added `metrics *metrics.BusinessMetrics` field + `WithMetrics` setter. `RecordUserRegistration` called after every successful `h.userRepo.Create()` in the V2 handler (covers Twitch, YouTube, Kick registrations).
- `services/auth-service/handlers/auth_handler.go`: Same pattern — `metrics` field + `WithMetrics` setter. `RecordUserRegistration("twitch")` and `RecordUserRegistration("youtube")` called after the respective legacy callback `Create()` calls.
- `services/auth-service/cmd/main.go`: Changed `_ = metrics.NewBusinessMetrics()` to `businessMetrics := metrics.NewBusinessMetrics()` and wired it into both handlers via `.WithMetrics(businessMetrics)`.

**Tests (TDD):** Three unit tests in `shared/metrics/business_test.go` verify the counter is non-nil, increments for "twitch", and increments for "youtube". Uses `prometheus/client_golang/prometheus/testutil` with isolated registry.

### Task 2: Grafana dashboard in caesar-deployment

Added `allchat-users.json` key to `apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml` ConfigMap with 5 panels:

| # | Type | Title | Query |
|---|------|-------|-------|
| 1 | timeseries (stacked area) | New Registrations Over Time | `sum(increase(allchat_user_registrations_total[1h])) by (platform)` |
| 2 | stat | Total Registrations (24h) | `sum(increase(allchat_user_registrations_total[24h]))` |
| 3 | stat | Total Registrations (7d) | `sum(increase(allchat_user_registrations_total[7d]))` |
| 4 | piechart | Registrations by Platform (30d) | `sum(increase(allchat_user_registrations_total[30d])) by (platform)` |
| 5 | timeseries | Daily Registration Rate | `sum(rate(allchat_user_registrations_total[1d])) by (platform) * 86400` |

## Commits

| Repo | Hash | Message |
|------|------|---------|
| all-chat | c228b44 | test(quick-260329-u6p): add failing tests for UserRegistrations metric |
| all-chat | 7a81716 | feat(quick-260329-u6p): add allchat_user_registrations_total metric to auth-service |
| caesar-deployment | f818dd8 | feat(quick-260329-u6p): add AllChat User Registrations Grafana dashboard |

## Deviations from Plan

### Auto-fixed Issues

None.

### Approach Notes

**WithMetrics setter vs constructor parameter:** The plan suggested adding a `metrics` parameter to constructors. Instead, used the `WithMetrics` setter pattern (fluent/builder style) to avoid changing constructor signatures and all existing test call-sites. This is strictly backward compatible.

**testutil dependency:** Added `github.com/prometheus/client_golang/prometheus/testutil` as a dependency in `shared/go.sum` to enable `testutil.CollectAndCompare`. The `client_golang` package was already a direct dependency; only the `kylelemons/godebug/diff` transitive dep needed adding to `go.sum`.

**Dashboard YAML validation:** `pyyaml` not available in the environment. Validated the JSON using a pure-Python JSON parser extraction from the YAML line instead.

## Known Stubs

None — metric is fully wired and dashboard queries the real metric.

## Self-Check: PASSED

- `shared/metrics/business_test.go`: EXISTS
- `shared/metrics/business.go`: EXISTS (UserRegistrations + RecordUserRegistration + newBusinessMetricsWithRegistry)
- `services/auth-service/handlers/platform_auth_v2.go`: EXISTS (metrics field + WithMetrics + RecordUserRegistration call)
- `services/auth-service/handlers/auth_handler.go`: EXISTS (metrics field + WithMetrics + two RecordUserRegistration calls)
- `services/auth-service/cmd/main.go`: EXISTS (businessMetrics wired)
- `~/git/caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml`: EXISTS (allchat-users.json key with 5 panels)
- all-chat commits c228b44, 7a81716: FOUND
- caesar-deployment commit f818dd8: FOUND
