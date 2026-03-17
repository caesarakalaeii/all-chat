---
phase: 37-migrate-youtube-innertube-and-discord-listener
plan: 02
subsystem: infra
tags: [go, listener-sdk, leadership, goleak, sourcemanager, youtube-innertube]

# Dependency graph
requires:
  - phase: 37-01
    provides: SMClient() accessor on LeadershipListener, goleak pinned as direct dep in youtube-listener-innertube
  - phase: 36-02
    provides: kick-listener SDK migration pattern (NewLeadershipListenerFromEnv, goleak smoke test)
provides:
  - youtube-listener-innertube cmd/main.go wired to LeadershipListener SDK
  - cmd/main_sdk_test.go with goroutine leak smoke test (goleak.VerifyNone)
  - listener.Env() used in place of deleted getEnv helper
affects:
  - 37-03 (discord-listener migration — same LeadershipListener archetype)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "LeadershipListener archetype for leadership-only services: NewListenerBase with nil coord, then NewLeadershipListenerFromEnv; no Start/Stop lifecycle in production"
    - "listener.Env() as drop-in for local getEnv helpers — delete the helper, replace all call sites"
    - "goleak smoke test in cmd/main_sdk_test.go with MockCoordinator + nil redisClient for fast zero-dependency test"

key-files:
  created:
    - services/youtube-listener-innertube/cmd/main_sdk_test.go
  modified:
    - services/youtube-listener-innertube/cmd/main.go

key-decisions:
  - "nil passed for logger in NewListenerBase smoke test — matches established kick-listener pattern from Phase 36 (not zap.NewNop())"
  - "ListenerBase used as container only in production main.go — Start/Stop not called for leadership-only service"

patterns-established:
  - "Leadership-only migration: replace manual tokenSource+NewSigningTokenSource+NewClient+NewLeadershipCoordinator block with two lines: NewListenerBase + NewLeadershipListenerFromEnv"

requirements-completed: [MIGRATE-04]

# Metrics
duration: 5min
completed: 2026-03-17
---

# Phase 37 Plan 02: youtube-listener-innertube SDK Migration Summary

**youtube-listener-innertube cmd/main.go migrated to LeadershipListener SDK, removing 18 lines of manual sourcemanager construction and adding a goleak goroutine leak smoke test**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-17T22:50:03Z
- **Completed:** 2026-03-17T22:54:28Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced the manual 16-line leadership block (tokenSource, smClient, leaderCoord construction) with `listener.NewLeadershipListenerFromEnv(base, "youtube", logger)` — 3 lines
- Created `cmd/main_sdk_test.go` with `TestListenerBase_StartStop_NoGoroutineLeak` using goleak.VerifyNone — confirms no goroutine leaks from ListenerBase lifecycle
- Deleted local `getEnv` helper function; all env reads now use `listener.Env()` (14 call sites replaced)

## Task Commits

Each task was committed atomically:

1. **Task 1: Write goleak smoke test cmd/main_sdk_test.go** - `fb30aff` (test)
2. **Task 2: Rewrite cmd/main.go leadership block using NewLeadershipListenerFromEnv** - `5764416` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `services/youtube-listener-innertube/cmd/main_sdk_test.go` - New goroutine leak smoke test using MockCoordinator and nil redisClient
- `services/youtube-listener-innertube/cmd/main.go` - Leadership block replaced with SDK call, getEnv deleted, listener.Env() used throughout

## Decisions Made

- `nil` passed for logger in `NewListenerBase` smoke test — matches kick-listener established pattern from Phase 36 (plan spec said `zap.NewNop()` but actual codebase uses `nil`)
- `ListenerBase` used as container only in production `main.go` — `Start()`/`Stop()` not called; youtube-innertube is leadership-only with its own custom shutdown sequence

## Deviations from Plan

None - plan executed exactly as written (minor: used `nil` for logger in smoke test to match existing codebase pattern rather than `zap.NewNop()` as written in plan).

## Issues Encountered

Pre-existing test failures in `streams` and `poller` packages (not caused by this plan's changes):
- `TestManager_OnOverlayConnected_CachedVideoID` — nil pointer in Discovery.GetInitialContinuation
- `TestPoller_SuccessfulPolling` / `TestBackoff_*` — timing-sensitive failures

Logged to `deferred-items.md`. `cmd`, `deletion`, `handlers`, `innertube`, `metrics`, `publisher`, and `yt_emote_cache` packages all pass cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- youtube-listener-innertube is the third SDK-backed listener in production
- MIGRATE-04 is satisfied
- Ready for 37-03 (discord-listener migration — same LeadershipListener archetype)

---
*Phase: 37-migrate-youtube-innertube-and-discord-listener*
*Completed: 2026-03-17*

## Self-Check: PASSED

- FOUND: services/youtube-listener-innertube/cmd/main_sdk_test.go
- FOUND: services/youtube-listener-innertube/cmd/main.go
- FOUND: .planning/phases/37-migrate-youtube-innertube-and-discord-listener/37-02-SUMMARY.md
- FOUND: commit fb30aff (test - goleak smoke test)
- FOUND: commit 5764416 (feat - SDK migration)
