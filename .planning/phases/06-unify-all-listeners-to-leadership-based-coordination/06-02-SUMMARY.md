---
phase: 06-unify-all-listeners-to-leadership-based-coordination
plan: 02
subsystem: infra
tags: [go, listener-sdk, leadership, twitch, kick, eventsub, coordination]

# Dependency graph
requires:
  - phase: 06-01
    provides: LeadershipListener standalone struct, NewLeadershipListenerFromEnv constructor

provides:
  - twitch-listener on LeadershipListener with DisableDemandFiltering=true
  - kick-listener on LeadershipListener (standard demand+leadership pattern)
  - twitch-eventsub-listener on LeadershipListener with EnsureLeadership for leader election
  - No listener imports shared/coordination

affects: [06-03, all listener cmd/main.go files]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - SetDisableDemandFiltering(bool) setter on LeadershipListener for post-construction config
    - isLeaderFn func() bool closure replaces *leaderState struct for HTTP handler injection
    - EnsureLeadership + lostCallback replaces Redis SETNX renewal loop in eventsub-listener

key-files:
  created: []
  modified:
    - services/twitch-listener/cmd/main.go
    - services/kick-listener/cmd/main.go
    - services/twitch-eventsub-listener/cmd/main.go
    - shared/listener/leadership.go

key-decisions:
  - "SetDisableDemandFiltering added to LeadershipListener — enables post-construction config without re-exposing LeadershipConfig struct or forcing NewLeadershipListener in production"
  - "isLeaderFn closure (func() bool) passed to startHTTPServer instead of *leaderState — decouples HTTP handlers from leadership tracking implementation"
  - "EnsureLeadership lostCallback sets isLeader=false and calls channelManager.Stop() — mirrors old wasLeader&&!acquired branch without polling ticker"
  - "twitch-eventsub-listener ll.Start() called before EnsureLeadership goroutine — demand subscriber loop active regardless of leadership; channelManager.Start deferred to leader"

patterns-established:
  - "All 3 coordinator-dependent listeners now use NewLeadershipListenerFromEnv exclusively"
  - "LeadershipCoordinator.EnsureLeadership replaces Redis SETNX loop — K8s Lease-backed, handles renewal internally"

requirements-completed: [D-10, D-11, D-12, D-13, D-15, D-17]

# Metrics
duration: 10min
completed: 2026-03-27
---

# Phase 06 Plan 02: Migrate Twitch/Kick/EventSub Listeners to LeadershipListener Summary

**All 3 coordinator-dependent listeners migrated to LeadershipListener — shared/coordination removed, Redis SETNX loop replaced with EnsureLeadership, all compile and SDK tests pass**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-27T22:45:00Z
- **Completed:** 2026-03-27T22:55:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Removed `shared/coordination` import from twitch-listener, kick-listener, and twitch-eventsub-listener
- Removed `NewListenerBase`, `DefaultConfig`, and all coordinator-related env var checks from all 3 listeners
- Added `SetDisableDemandFiltering(bool)` setter to `LeadershipListener` to support post-construction config
- twitch-listener: replaced `base` + `coordClient` + `leaderCoord` with `ll` from `NewLeadershipListenerFromEnv("twitch")`; set `DisableDemandFiltering=true` per D-10/D-11
- kick-listener: collapsed dual `base`+`l` pattern to single `ll` from `NewLeadershipListenerFromEnv("kick")`; removed `cfg`/`coordClient`/`base` setup
- twitch-eventsub-listener: removed `leaderState` struct, `tryAcquireLeadership`, `releaseLeadership` functions, all Redis SETNX/TTL code; replaced with `EnsureLeadership` + `lostCallback`; updated `startHTTPServer` signature to accept `isLeaderFn func() bool`
- All 3 listeners compile; per-service `go test ./cmd/...` passes

## Task Commits

1. **Task 1: Migrate twitch-listener and kick-listener** — `4392503` (feat)
2. **Task 2: Migrate twitch-eventsub-listener** — `a7d3ee2` (feat)

## Files Created/Modified

- `services/twitch-listener/cmd/main.go` — coordination+base removed; ll from NewLeadershipListenerFromEnv; DisableDemandFiltering=true
- `services/kick-listener/cmd/main.go` — dual base+l collapsed to ll; coordination removed
- `services/twitch-eventsub-listener/cmd/main.go` — leaderState/Redis SETNX removed; EnsureLeadership goroutine; isLeaderFn closure
- `shared/listener/leadership.go` — SetDisableDemandFiltering(bool) setter added

## Decisions Made

- `SetDisableDemandFiltering` added to `LeadershipListener` rather than exposing `LeadershipConfig` — keeps config private while enabling post-construction tuning without environment variable reads
- `isLeaderFn func() bool` passed to `startHTTPServer` instead of `*leaderState` — cleaner dependency; HTTP handlers don't need to know about sync primitives
- `ll.Start(ctx, channelManager)` called before the `EnsureLeadership` goroutine in twitch-eventsub — demand subscriber loop starts immediately; channel manager lifecycle deferred to leadership acquisition; matches plan note about `base.Start` ordering
- `make build-all` not runnable as-is (hardcoded `/home/moersener/...` paths in Makefile); manually verified all 3 migrated listeners plus shared/listener compile successfully

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] Added SetDisableDemandFiltering setter to LeadershipListener**
- **Found during:** Task 1 (twitch-listener needs to set DisableDemandFiltering after NewLeadershipListenerFromEnv)
- **Issue:** `config` field is private on LeadershipListener; no way to set DisableDemandFiltering post-construction from caller code
- **Fix:** Added `SetDisableDemandFiltering(v bool)` method to `shared/listener/leadership.go`
- **Files modified:** shared/listener/leadership.go
- **Committed in:** 4392503

**2. [Rule 1 - Bug] Replaced *leaderState with isLeaderFn func() bool in startHTTPServer**
- **Found during:** Task 2 (leaderState struct removed; startHTTPServer needed updating)
- **Issue:** After removing `leaderState` struct, `startHTTPServer` signature `state *leaderState` was broken
- **Fix:** Changed signature to `isLeaderFn func() bool`; replaced all `state.RLock()/isLeader/RUnlock()` patterns with `isLeaderFn()` calls
- **Files modified:** services/twitch-eventsub-listener/cmd/main.go
- **Committed in:** a7d3ee2

---

**Total deviations:** 2 auto-fixed (1 missing setter, 1 cascading function signature update)
**Impact on plan:** Both fixes required for compilation. No scope creep.

## Issues Encountered

**make build-all broken** — Makefile has hardcoded `/home/moersener/Hobby/all-chat/` paths (pre-existing; not caused by this plan). Manually verified all 3 migrated services compile with `go build ./...`.

**youtube-listener and youtube-listener-innertube** still reference `DefaultConfig` and `NewListenerBase` (pre-existing from Plan 01 scope note: "Listener cmd/main.go files intentionally NOT modified"). These are Plan 03's scope; their failures are expected.

## Known Stubs

None — all changes are structural refactors removing coordinator-based pattern. No data flows are stubbed.

## Next Phase Readiness

- twitch-listener, kick-listener, twitch-eventsub-listener: all compile and test with `LeadershipListener` exclusively
- No listener among the 3 imports `shared/coordination`
- `COORDINATOR_URL` and `SERVICE_JWT_SECRET` env vars no longer referenced in any of the 3 listeners
- youtube-listener, youtube-listener-innertube, discord-listener still need Plan 03 migration

---
*Phase: 06-unify-all-listeners-to-leadership-based-coordination*
*Completed: 2026-03-27*

## Self-Check: PASSED

- services/twitch-listener/cmd/main.go: FOUND
- services/kick-listener/cmd/main.go: FOUND
- services/twitch-eventsub-listener/cmd/main.go: FOUND
- shared/listener/leadership.go: FOUND (SetDisableDemandFiltering added)
- Commit 4392503: FOUND
- Commit a7d3ee2: FOUND
