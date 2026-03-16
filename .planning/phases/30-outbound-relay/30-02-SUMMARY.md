---
phase: 30-outbound-relay
plan: "02"
subsystem: api
tags: [discord, relay, pgx, redis, pubsub, go]

# Dependency graph
requires:
  - phase: 30-outbound-relay plan 01
    provides: relay package (manager, repository, poster) with all interfaces

provides:
  - pgx/v5 as direct dependency in discord-listener go.mod
  - buildDatabaseDSN() helper reading DATABASE_* env vars with defaults
  - DB connection pool (pgxpool) wired in cmd/main.go
  - relay.Manager constructed and started on service boot
  - relay.Manager.Stop() called in graceful shutdown before srv.Shutdown
affects: [31-scaling, ops-monitoring, discord-listener-deployment]

# Tech tracking
tech-stack:
  added: [github.com/jackc/pgx/v5 v5.8.0 (promoted to direct dependency)]
  patterns: [DB pool + relay manager wired at service entry point following existing service patterns; relay manager shutdown ordered before HTTP server shutdown]

key-files:
  created: []
  modified:
    - services/discord-listener/cmd/main.go
    - services/discord-listener/go.mod
    - services/discord-listener/go.sum

key-decisions:
  - "relay.NewHTTPPoster called with logger parameter — actual signature has 3 params (token, client, logger) unlike plan spec which showed 2 params"
  - "pgxpool.New called after signal.NotifyContext so ctx cancellation propagates to connection pool creation on shutdown"
  - "relayMgr.Stop() called before srv.Shutdown to ensure no relay goroutines outlive the service"

patterns-established:
  - "DB pool created immediately after Redis client, before any domain objects — same order as other services"
  - "Relay manager goroutine wraps Start() with ctx.Err() nil guard to suppress error on clean shutdown"

requirements-completed: [RELY-01, RELY-02, RELY-03, RELY-04]

# Metrics
duration: 5min
completed: 2026-03-16
---

# Phase 30 Plan 02: Outbound Relay Wiring Summary

**pgx/v5 DB pool + relay.Manager wired into discord-listener cmd/main.go; service now starts and stops outbound Discord relay on boot/shutdown**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-16
- **Completed:** 2026-03-16
- **Tasks:** 1 of 1 automated tasks complete (checkpoint pending human approval)
- **Files modified:** 3

## Accomplishments

- Promoted `github.com/jackc/pgx/v5` from indirect to direct dependency in go.mod
- Added `buildDatabaseDSN()` reading `DATABASE_{HOST,PORT,NAME,USER,PASSWORD}` with same defaults as other services
- Wired `pgxpool.New`, `relay.NewRepository`, `relay.NewHTTPPoster`, and `relay.NewManager` in `main()`
- Started `relayMgr.Start(ctx)` as a background goroutine after the gateway reconnect goroutine
- Added `relayMgr.Stop()` to the shutdown block before `srv.Shutdown`
- All 38 tests pass: 7 relay package tests + 28 gateway tests + 1 publisher test + build clean

## Task Commits

1. **Task 1: Add pgx/v5 dependency and wire relay.Manager into cmd/main.go** - `d70cda8` (feat)

**Plan metadata:** (pending)

## Files Created/Modified

- `services/discord-listener/cmd/main.go` - Added relay imports, buildDatabaseDSN(), DB pool, relayMgr construction, Start goroutine, Stop in shutdown
- `services/discord-listener/go.mod` - pgx/v5 promoted to direct dependency
- `services/discord-listener/go.sum` - Updated checksums

## Decisions Made

- `relay.NewHTTPPoster` takes a third `logger *zap.Logger` parameter not shown in the plan spec — used the actual signature from poster.go
- `pgxpool.New` is called after `signal.NotifyContext` so the context used for DB connection is the same signal-aware context used throughout startup
- Shutdown order: `gwClient.Close()` → `relayMgr.Stop()` → `srv.Shutdown()` ensures all active relay goroutines drain before the HTTP server closes

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] relay.NewHTTPPoster signature mismatch**
- **Found during:** Task 1 (reading actual poster.go)
- **Issue:** Plan showed `relay.NewHTTPPoster(token, client)` but actual function signature is `relay.NewHTTPPoster(token, client, logger)` (added in Plan 01 per STATE.md decision)
- **Fix:** Called with all three arguments: `relay.NewHTTPPoster(botToken, &http.Client{Timeout: 10 * time.Second}, log)`
- **Files modified:** services/discord-listener/cmd/main.go
- **Verification:** `go build ./...` exits 0
- **Committed in:** d70cda8 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — signature mismatch discovered on read)
**Impact on plan:** Fix required for compilation. No scope change.

## Issues Encountered

None — build and all 38 tests green on first attempt.

## User Setup Required

None - no external service configuration required beyond existing `DISCORD_BOT_TOKEN` and `DATABASE_*` env vars.

## Next Phase Readiness

- Full outbound relay stack is live: relay package + DB wiring + cmd/main.go startup/shutdown
- discord-listener now subscribes to `overlay:{overlay_id}` Redis Pub/Sub channels for relay-enabled Discord sources and POSTs formatted messages to Discord REST on boot
- Ready for Phase 31 (scaling / HPA) or live integration testing with a bot token

---
*Phase: 30-outbound-relay*
*Completed: 2026-03-16*
