---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: 03
subsystem: infra
tags: [prometheus, metrics, gin, promauto, http-metrics, business-metrics]

requires:
  - phase: 04-01
    provides: ServiceMonitor wiring and gap matrix confirming which services had ENDPOINT ONLY metrics

provides:
  - HTTP request count and duration metrics in auth-service, overlay-manager, emote-service, and token-refresh-service
  - Source operation business metrics in overlay-manager (create/delete per platform)
  - Emote provider API call metrics in emote-service (per provider, cache-miss API hits)
  - Token refresh attempt and error metrics in token-refresh-service (per platform, result, error category)
  - Active sources per platform and assignment operation metrics in source-manager coordinator

affects:
  - 04-04 (Listener dashboards can now query platform ops metrics from Prometheus)
  - 04-05 (Alerting rules for error-rate-spike and platform ops can reference these metrics)

tech-stack:
  added: []
  patterns:
    - "httpMetricsMiddleware pattern: promauto CounterVec + HistogramVec registered once in cmd/main.go, passed to Gin middleware function"
    - "Nil-safe business metrics injection: bm *metrics.BusinessMetrics passed to handlers, all call sites guard with nil check"
    - "categorizeRefreshError function classifies OAuth errors into token_revoked, invalid_client, network_error, other for label safety"

key-files:
  created: []
  modified:
    - services/auth-service/cmd/main.go
    - services/overlay-manager/cmd/main.go
    - services/overlay-manager/handlers/sources.go
    - services/emote-service/cmd/main.go
    - services/emote-service/handlers/emote.go
    - services/emote-service/handlers/emote_test.go
    - services/token-refresh-service/cmd/main.go
    - services/token-refresh-service/refresher/manager.go
    - services/source-manager/cmd/main.go
    - services/source-manager/coordination/coordinator.go

key-decisions:
  - "httpMetricsMiddleware defined locally in each service's cmd/main.go — avoids importing GatewayMetrics (semantically wrong for non-gateway) while keeping metric names consistent (http_requests_total, http_request_duration_seconds)"
  - "BusinessMetrics injected nil-safely into SourcesHandler with new bm field — constructor updated but guard ensures nil bm never panics"
  - "EmoteHandler apiCalls counter passed as *prometheus.CounterVec (not interface) — simplest approach; nil if not provided, which satisfies test callers"
  - "Coordinator businessMetrics field added to Coordinator struct; SetActiveSourcesTotal called per-platform after sources query; RecordSourceOperation called per successful assignment store"
  - "categorizeRefreshError uses string matching (not error type assertions) — consistent with existing isNonRetryableError pattern in the same file"

requirements-completed:
  - WIRE-06
  - WIRE-07
  - WIRE-08
  - WIRE-09
  - WIRE-10

duration: 38min
completed: 2026-03-26
---

# Phase 04 Plan 03: Platform Ops Metrics Wiring Summary

**HTTP request metrics plus business operation counters wired into all 5 platform ops services using promauto Gin middleware, completing Prometheus coverage for auth-service, overlay-manager, emote-service, token-refresh-service, and source-manager.**

## Performance

- **Duration:** 38 min
- **Started:** 2026-03-26T18:30:30Z
- **Completed:** 2026-03-26T19:08:30Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- All 5 platform ops services now emit `http_requests_total` and `http_request_duration_seconds` via a shared Gin middleware pattern
- overlay-manager emits `allchat_source_operations_total` on every add/delete source operation (platform + result labels)
- emote-service emits `emote_api_calls_total` per provider (7tv, bttv, ffz, twitch) tracking cache-miss API calls
- token-refresh-service emits `token_refresh_attempts_total` and `token_refresh_errors_total` per platform with error categorization
- source-manager coordinator emits `allchat_active_sources_total` per platform after each reconcile and `allchat_source_operations_total` per assignment

## Task Commits

1. **Task 1: Wire auth-service, overlay-manager, and emote-service HTTP metrics** - `07e8e73` (feat)
2. **Task 2: Wire token-refresh-service and source-manager business metrics** - included in `c4030fb` (dependency update commit that swept in uncommitted changes)

## Files Created/Modified

- `services/auth-service/cmd/main.go` - Added httpMetricsMiddleware and promauto counter/histogram init
- `services/overlay-manager/cmd/main.go` - Added httpMetricsMiddleware; bm passed to NewSourcesHandler
- `services/overlay-manager/handlers/sources.go` - Added bm *metrics.BusinessMetrics field; RecordSourceOperation calls in HandleAddSource/HandleDeleteSource
- `services/emote-service/cmd/main.go` - Added httpMetricsMiddleware and emote_api_calls_total counter
- `services/emote-service/handlers/emote.go` - Added apiCalls *prometheus.CounterVec field; recording on cache-miss API calls
- `services/emote-service/handlers/emote_test.go` - Updated NewEmoteHandler calls to pass nil for apiCalls
- `services/token-refresh-service/cmd/main.go` - Added httpMetricsMiddleware; fixed getInt bug (fmt.Sscan missing target pointer)
- `services/token-refresh-service/refresher/manager.go` - Added refreshTotal and refreshErrors counters; categorizeRefreshError helper
- `services/source-manager/cmd/main.go` - Changed `_ = metrics.NewBusinessMetrics()` to capture businessMetrics; passed to NewCoordinator
- `services/source-manager/coordination/coordinator.go` - Added businessMetrics field; SetActiveSourcesTotal per platform after sources query; RecordSourceOperation per successful assignment

## Decisions Made

- Used locally-defined `httpMetricsMiddleware` per service rather than importing `GatewayMetrics.RecordHTTPRequest` — gateway metrics use `gateway_http_requests_total` which is semantically wrong for non-gateway services; separate binaries with same metric name `http_requests_total` is safe
- Nil-safe injection pattern for BusinessMetrics and CounterVec — allows nil in tests without panicking

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed getInt in token-refresh-service cmd/main.go**
- **Found during:** Task 2 (token-refresh-service HTTP metrics wiring)
- **Issue:** `fmt.Sscan(value)` called without a target variable — incorrect API usage, return value misread as count not int
- **Fix:** Changed to `fmt.Sscan(value, &i)` with proper scanned variable
- **Files modified:** services/token-refresh-service/cmd/main.go
- **Verification:** go build passes
- **Committed in:** c4030fb (swept into dependency update commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 bug)
**Impact on plan:** Pre-existing bug in getInt caught during wiring. No scope creep.

## Issues Encountered

Task 2 changes were swept into `c4030fb` (quick-plan dependency update commit) rather than getting their own atomic commit — the git add/commit step ran while the working tree still showed no untracked changes because the quick-plan task ran between my edits and the commit attempt. All Task 2 code is present and compiled correctly.

## Next Phase Readiness

- All 14 services (excluding support-bot per deferred decision) now emit operational metrics to Prometheus
- Platform Ops dashboard (Plan 04) can query auth-service and overlay-manager HTTP metrics
- Error rate alerting (Plan 05) can reference all 5 services
- Token refresh reliability visible via token_refresh_attempts_total{result="error"} alerts

## Self-Check: PASSED

All created files exist. Task commits found (07e8e73 for Task 1; c4030fb swept Task 2).

---
*Phase: 04-grafana-dashboard-audit-metrics-gap-implementation*
*Completed: 2026-03-26*
