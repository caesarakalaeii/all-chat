---
phase: 31-load-balancing
plan: 02
subsystem: discord-listener/metrics
tags: [prometheus, promauto, metrics, shard-ownership, leader-election, sourcemanager]

requires:
  - phase: 31-load-balancing/31-01
    provides: "Gateway RESUME protocol (BuildResumePayload call sites in OpHello branch)"
provides:
  - LOAD-01
  - LOAD-02
  - "discord_listener_* Prometheus metrics package"
  - "Shard ownership gate via LeadershipCoordinator in Gateway goroutine"
  - "/metrics HTTP endpoint for Prometheus scraping"
affects: [31-03-PLAN]

tech-stack:
  added:
    - github.com/prometheus/client_golang v1.23.2
    - github.com/caesar/all-chat/shared (replace directive to ../../shared)
  patterns:
    - "promauto package-level vars with unexported metrics + exported setter/inc funcs (same as kick-listener)"
    - "LeadershipCoordinator nil-guard for graceful degradation when SOURCE_MANAGER_URL absent"

key-files:
  created:
    - services/discord-listener/metrics/metrics.go
    - services/discord-listener/metrics/metrics_test.go
  modified:
    - services/discord-listener/gateway/client.go
    - services/discord-listener/cmd/main.go
    - services/discord-listener/go.mod

key-decisions:
  - "lostCallback in EnsureLeadership does NOT call gwClient.Close() — calling Close() from the callback would be re-entrant and race with the running Connect() goroutine; next loop iteration re-checks EnsureLeadership instead"
  - "Graceful degradation when SOURCE_MANAGER_URL/SECRET absent: WARN log + direct connect (consistent with kick-listener pattern)"

patterns-established:
  - "Prometheus metrics: promauto-registered package-level vars, exported helpers only (labelValue guards empty strings)"
  - "Ownership gate: leaderCoord != nil guard wraps EnsureLeadership — nil coordinator bypasses gate"

requirements-completed: [LOAD-01, LOAD-02]

duration: 3min
completed: "2026-03-16"
---

# Phase 31 Plan 02: Shard Ownership Gating and Prometheus Metrics Summary

**Prometheus metrics package (4 discord_listener_* metrics) + LeadershipCoordinator shard ownership gate in Gateway goroutine + /metrics scraping endpoint**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-16T09:25:35Z
- **Completed:** 2026-03-16T09:28:39Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Created `metrics` package with 4 promauto-registered Prometheus metrics: `discord_listener_gateway_events_total`, `discord_listener_active_guilds`, `discord_listener_shard_ownership`, `discord_listener_resume_attempts_total`
- Wired `IncResumeAttempt("success")` and `IncResumeAttempt("fallback_identify")` into gateway/client.go RESUME/IDENTIFY branch
- Added `LeadershipCoordinator` ownership gate in Gateway goroutine — only the pod holding `shard:0` calls `gwClient.Connect()`; graceful degradation when SOURCE_MANAGER_URL absent
- Exposed `/metrics` endpoint via `promhttp.Handler()` for Prometheus scraping

## Task Commits

Each task was committed atomically:

1. **Task 1: discord-listener metrics package** - `786317a` (feat)
2. **Task 2: metrics call sites + ownership gating + /metrics route** - `46a63e8` (feat)

## Files Created/Modified

- `services/discord-listener/metrics/metrics.go` - 4 promauto metrics + exported IncGatewayEvent, SetActiveGuilds, SetShardOwnership, IncResumeAttempt
- `services/discord-listener/metrics/metrics_test.go` - TestMetricRegistration, TestShardOwnershipToggle
- `services/discord-listener/gateway/client.go` - metrics import + IncResumeAttempt call sites in OpHello branch
- `services/discord-listener/cmd/main.go` - LeadershipCoordinator setup, ownership-gated Gateway goroutine, /metrics route
- `services/discord-listener/go.mod` - added prometheus/client_golang v1.23.2, shared v0.0.0, replace directive

## Decisions Made

- `lostCallback` in `EnsureLeadership` does NOT call `gwClient.Close()` — calling `Close()` from the callback would be re-entrant and create a race with the running `Connect()` goroutine. Instead the next loop iteration calls `EnsureLeadership` which returns `(false, nil)`, causing the pod to sleep and wait before reconnecting without ownership.
- Graceful degradation: when `SOURCE_MANAGER_URL` or `SOURCE_MANAGER_SECRET` is unset, `leaderCoord` stays `nil` and the WARN log fires — consistent with kick-listener's degradation pattern.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None — `go mod tidy` temporarily removed manually added deps before `metrics.go` existed; resolved by creating `metrics.go` first (which imports prometheus), then running `go get` + `go mod tidy` in sequence.

## Next Phase Readiness

- Plan 31-03 (Kubernetes HPA manifests) can now reference `discord_listener_shard_ownership` and other metrics for HPA scaling policy configuration.
- `/metrics` endpoint on port 8086 is ready for Prometheus ServiceMonitor scraping.

---
*Phase: 31-load-balancing*
*Completed: 2026-03-16*

## Self-Check: PASSED

Files confirmed present:
- services/discord-listener/metrics/metrics.go ✓
- services/discord-listener/metrics/metrics_test.go ✓
- services/discord-listener/gateway/client.go (contains IncResumeAttempt) ✓
- services/discord-listener/cmd/main.go (contains EnsureLeadership) ✓

Commits confirmed:
- 786317a — feat(31-02): add discord-listener metrics package with Prometheus counters/gauges
- 46a63e8 — feat(31-02): wire shard ownership gating and metrics into discord-listener
