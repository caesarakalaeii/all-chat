---
phase: 37-migrate-youtube-innertube-and-discord-listener
plan: 03
subsystem: infra
tags: [go, sdk, listener, sourcemanager, leadership, discord, goleak]

# Dependency graph
requires:
  - phase: 37-01
    provides: LeadershipListener SDK with NewLeadershipListenerFromEnv and listener.Env
  - phase: 36-migrate-kick-listener
    provides: established smoke test pattern for LeadershipListener services
provides:
  - discord-listener cmd/main.go wired via SDK LeadershipListener
  - goleak smoke test confirming zero goroutine leaks in ListenerBase lifecycle
  - MIGRATE-05 requirement satisfied
affects: [phase-38, discord-listener, listener-sdk]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - LeadershipListener gateway goroutine without outer nil guard (EnsureLeadership always called, nil-safe passthrough)
    - listener.Env drop-in replacing local getEnv helper
    - goleak smoke test in cmd package using MockCoordinator and inline mockChannelManagerForTest

key-files:
  created:
    - services/discord-listener/cmd/main_sdk_test.go
  modified:
    - services/discord-listener/cmd/main.go

key-decisions:
  - "discord-listener is leadership-only: ll.Start() and ll.Stop() NOT called — ListenerBase is container only for NewLeadershipListenerFromEnv; no ShutdownCoordinator used"
  - "Gateway goroutine outer nil guard removed — EnsureLeadership called unconditionally via nil-safe passthrough; only metrics.SetShardOwnership calls remain guarded"
  - "Smoke test uses nil logger (matching kick-listener Phase 36 pattern) not zap.NewNop() — nil is acceptable per NewListenerBase signature"

patterns-established:
  - "LeadershipListener production pattern: NewLeadershipListenerFromEnv + unconditional EnsureLeadership + guarded metrics calls"

requirements-completed: [MIGRATE-05]

# Metrics
duration: 3min
completed: 2026-03-17
---

# Phase 37 Plan 03: Discord-Listener SDK Migration Summary

**discord-listener migrated to LeadershipListener SDK via NewLeadershipListenerFromEnv, removing manual sourcemanager construction and simplifying the gateway goroutine nil-guard pattern**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-17T22:49:59Z
- **Completed:** 2026-03-17T22:52:50Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced 14-line manual sourcemanager block with 4-line SDK call in cmd/main.go
- Gateway goroutine now calls `ll.LeadershipCoordinator().EnsureLeadership(...)` unconditionally — no outer nil guard needed
- Added cmd/main_sdk_test.go with `TestListenerBase_StartStop_NoGoroutineLeak` confirming zero goroutine leaks
- Deleted local `getEnv` helper; all env reads use `listener.Env` consistently
- `make build-all` and all non-gateway tests pass with race detector

## Task Commits

Each task was committed atomically:

1. **Task 1: Write goleak smoke test cmd/main_sdk_test.go** - `47cb60b` (test)
2. **Task 2: Rewrite cmd/main.go leadership block and gateway goroutine using SDK** - `4176f02` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `services/discord-listener/cmd/main_sdk_test.go` - Goroutine leak smoke test using MockCoordinator and inline channel manager stub
- `services/discord-listener/cmd/main.go` - Leadership block replaced with SDK, gateway goroutine nil-guard simplified, getEnv deleted

## Decisions Made

- `ll.Start()` and `ll.Stop()` are NOT called in production — discord-listener is leadership-only and the existing custom shutdown sequence (gwClient.Close, relayMgr.Stop, srv.Shutdown) stays unchanged
- Smoke test uses `nil` logger matching the established kick-listener Phase 36 pattern; plan template suggested `zap.NewNop()` but nil is equally valid
- Gateway goroutine calls `EnsureLeadership` unconditionally without outer nil guard — nil-safe passthrough in SDK returns `acquired=true, err=nil` when coordinator is absent

## Deviations from Plan

None — plan executed exactly as written. Pre-existing gateway test failures (out of scope) documented in `deferred-items.md`.

## Issues Encountered

**Pre-existing gateway package test failures (out of scope):**
- `services/discord-listener/gateway` tests fail to compile because `mockChannelRegistry` stubs in those test files are missing the `ListConfiguredChannels` method that was added to the `gateway.ChannelRegistry` interface
- Confirmed pre-existing by stash verification — identical failures before any Plan 37-03 changes
- Documented in `.planning/phases/37-migrate-youtube-innertube-and-discord-listener/deferred-items.md`
- All non-gateway packages pass (`cmd`, `metrics`, `publisher`, `relay`)

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- discord-listener is the fourth SDK-backed listener in production
- MIGRATE-05 satisfied — all listeners in the LeadershipListener archetype (Kick, InnerTube, Discord) now use the SDK
- Phase 38 (youtube-listener + twitch-eventsub) can proceed
- Pre-existing gateway test failures should be addressed as a cleanup task (add `ListConfiguredChannels` stub to gateway mock types)

---
*Phase: 37-migrate-youtube-innertube-and-discord-listener*
*Completed: 2026-03-17*
