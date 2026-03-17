---
phase: 34-sdk-package-definition
plan: 02
subsystem: shared/listener
tags: [sdk, listener, goroutine, leadership, shutdown, testutil, goleak, coordinator]

# Dependency graph
requires:
  - phase: 34-01
    provides: ChannelManager interface and updated CoordinatorClient with explicit serviceName
provides:
  - shared/listener/config.go (ListenerConfig, DefaultConfig, Env helper)
  - shared/listener/base.go (ListenerBase with 3-goroutine lifecycle, coordinatorClient interface)
  - shared/listener/leadership.go (LeadershipListener embedding ListenerBase)
  - shared/listener/shutdown.go (ShutdownCoordinator ordered shutdown function)
  - shared/listener/env_test.go (3 Env() behavioral tests — wave 2 Nyquist check)
  - shared/listener/testutil/mock_coordinator.go (MockCoordinator with call tracking)
  - Makefile build-all target for monorepo-wide compile verification
affects:
  - 35-twitch-listener-migration
  - 36-kick-listener-migration
  - 37-discord-listener-migration
  - 38-youtube-listener-migration

# Tech tracking
tech-stack:
  added:
    - go.uber.org/goleak v1.3.0 (goroutine leak detection for listener tests)
  patterns:
    - "coordinatorClient private interface in base.go enables mock injection without public API surface"
    - "Ticker + select pattern with exponential backoff (1s→2s→4s→30s cap) for all 3 goroutine loops"
    - "nil redis.Client guard in startMigrationSubscriberLoop — test-safe without real Redis"
    - "DisableCoordinatorFiltering flag passes empty map to UpdateAssignedSourceIDs for rollback"
    - "LeadershipListener nil coordinator pattern — all LeadershipCoordinator methods are nil-safe"
    - "ShutdownCoordinator: parallel stop (mgr+base) → platformDisconnect → 10s HTTP drain"
    - "TDD RED-GREEN cycle: env_test.go committed failing first, implementation committed second"

key-files:
  created:
    - shared/listener/config.go
    - shared/listener/base.go
    - shared/listener/leadership.go
    - shared/listener/shutdown.go
    - shared/listener/env_test.go
    - shared/listener/testutil/mock_coordinator.go
  modified:
    - shared/go.mod
    - Makefile

key-decisions:
  - "coordinatorClient is a private interface (lowercase) in base.go — avoids public API surface while enabling mock injection"
  - "LeadershipListener construction uses sourcemanager.NewSigningTokenSource (15min TTL) — matches kick-listener production pattern"
  - "sourcemanager.NewClient signature is (rawURL, tokenSource, opts...) not (url, secret, logger) — plan description was simplified; actual API used"
  - "NewLeadershipCoordinator signature is (platform, client, interval, logger) — plan had wrong argument order; fixed at implementation time"
  - "make build-all uses absolute paths for cd commands — prevents relative path failures when make is invoked from any directory"

patterns-established:
  - "All listener goroutine loops: ticker+select with backoff, OnFatalError callback, context cancellation"
  - "Nil redis guard in migration loop: if b.redisClient == nil { return } — tests pass nil safely"
  - "env_test.go as wave 2 Nyquist behavioral checkpoint — 3 tests covering absent/set/empty cases"

requirements-completed: [SDK-01, SDK-02, SDK-04, SDK-05, SDK-06, SDK-07, VERIFY-01]

# Metrics
duration: 15min
completed: 2026-03-17
---

# Phase 34 Plan 02: SDK Package Implementation Summary

**ListenerBase with 3-goroutine lifecycle, LeadershipListener with env-based nil-safe construction, and ShutdownCoordinator with parallel stop + 10s HTTP drain — full shared/listener SDK core delivered.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-17T20:30:00Z
- **Completed:** 2026-03-17T20:45:00Z
- **Tasks:** 3 (Task 2 has TDD RED+GREEN commits)
- **Files modified:** 8

## Accomplishments

- ListenerBase manages 3 goroutine loops (heartbeat, assignment-refresh, migration-subscriber) with exponential backoff and OnFatalError callback
- LeadershipListener embeds ListenerBase with nil-safe coordinator construction from env (SOURCE_MANAGER_SECRET absent → disabled, nil-safe)
- ShutdownCoordinator performs ordered shutdown: parallel stop → platformDisconnect → 10s HTTP drain
- MockCoordinator in testutil tracks call counts atomically, supports failure simulation, goleak-safe (no real HTTP/Redis)
- `make build-all` target verifies all 6 Go listener modules build from a single CI command
- All 3 Env() behavioral tests pass (wave 2 Nyquist compliance confirmed)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add goleak test dependency** - `386a5cc` (chore)
2. **Task 2 RED: env_test.go failing tests** - `ac47dce` (test)
3. **Task 2 GREEN: config, base, leadership, shutdown** - `ff5a9a3` (feat)
4. **Task 3: testutil MockCoordinator + make build-all** - `7b41df4` (feat)

## Files Created/Modified

- `shared/listener/config.go` — ListenerConfig (5 fields), DefaultConfig(), Env() helper
- `shared/listener/base.go` — ListenerBase struct, coordinatorClient private interface, Start/Stop, 3 goroutine loops
- `shared/listener/leadership.go` — LeadershipListener embedding ListenerBase, NewLeadershipListenerFromEnv
- `shared/listener/shutdown.go` — ShutdownCoordinator with parallel stop + 10s HTTP drain
- `shared/listener/env_test.go` — 3 behavioral tests for Env() helper
- `shared/listener/testutil/mock_coordinator.go` — MockCoordinator with atomic call count tracking
- `shared/go.mod` — added go.uber.org/goleak v1.3.0
- `Makefile` — build-all target for all 6 Go listener modules

## Decisions Made

**coordinatorClient is a private interface** — enables mock injection in tests without adding public API surface. `*coordination.CoordinatorClient` satisfies it automatically via Go's structural typing.

**Actual sourcemanager API differs from plan description** — `sourcemanager.NewClient(rawURL, tokenSource, opts...)` and `NewLeadershipCoordinator(platform, client, interval, logger)` are the actual signatures (not the simplified forms shown in the plan context). Used `NewSigningTokenSource` with 15min TTL matching kick-listener production pattern.

**make build-all uses absolute paths** — prevents `cd` failures when invoked from directories other than repo root.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected sourcemanager constructor signatures**
- **Found during:** Task 2 (leadership.go implementation)
- **Issue:** Plan showed `sourcemanager.NewClient(smURL, secret, logger)` and `sourcemanager.NewLeadershipCoordinator(smClient, platform, logger)` — neither matches the actual package API
- **Fix:** Used actual API: `NewClient(rawURL, tokenSource)` with `NewSigningTokenSource(platform+"-listener", secret, 15*time.Minute)`, and `NewLeadershipCoordinator(platform, smClient, 5*time.Second, logger)`
- **Files modified:** shared/listener/leadership.go
- **Verification:** `go build ./listener/...` exits 0
- **Committed in:** `ff5a9a3` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — constructor signature mismatch between plan description and actual code)
**Impact on plan:** Fix was necessary for compilation. No scope change. All must-haves met.

## Issues Encountered

`go mod tidy` removed the goleak entry after `go get` since no source files imported it yet. Fixed by adding the require entry directly to go.mod before creating implementation files.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- shared/listener SDK is complete and importable by phases 35–38
- All 6 Go listener modules build via `make build-all`
- MockCoordinator available for unit tests in migration phases
- Compile-time interface assertions deferred to Phase 35 (per CONTEXT.md lock)

---

## Self-Check: PASSED

Files verified:
- FOUND: shared/listener/config.go
- FOUND: shared/listener/base.go
- FOUND: shared/listener/leadership.go
- FOUND: shared/listener/shutdown.go
- FOUND: shared/listener/env_test.go
- FOUND: shared/listener/testutil/mock_coordinator.go

Commits verified in git log: 386a5cc, ac47dce, ff5a9a3, 7b41df4

*Phase: 34-sdk-package-definition*
*Completed: 2026-03-17*
