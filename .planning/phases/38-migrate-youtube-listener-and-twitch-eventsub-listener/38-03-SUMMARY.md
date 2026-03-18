---
phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener
plan: 03
subsystem: infra
tags: [go, listener-sdk, twitch-eventsub, goroutine-leak, goleak, sdk-migration]

# Dependency graph
requires:
  - phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener
    provides: Plan 38-02 — channels.Manager SDK-compliant (Start signature, 3 new methods, compile-time assertion)
  - phase: 34-sdk-definition
    provides: listener.ListenerBase, listener.NewListenerBase, listener.ChannelManager interface
provides:
  - twitch-eventsub-listener wired to ListenerBase SDK (heartbeat, assignment refresh, migration subscriber, JWT refresh all SDK-owned)
  - goroutine leak smoke test for twitch-eventsub-listener ListenerBase + ChannelManager lifecycle
  - MIGRATE-06 complete — all 6 Go listeners on shared SDK simultaneously
affects: [make build-all, milestone v1.6 complete]

# Tech tracking
tech-stack:
  added:
    - go.uber.org/goleak v1.3.0 (direct dep in twitch-eventsub-listener)
  patterns:
    - coordClient built manually, passed to listener.NewListenerBase — matches all other migrated services
    - base.Start(ctx, channelManager) called before leader election goroutine starts
    - defer base.Stop() for SDK-owned goroutine cleanup
    - listener.Env() replaces all getEnv() call sites; getEnv function deleted

key-files:
  created:
    - services/twitch-eventsub-listener/cmd/main_sdk_test.go
  modified:
    - services/twitch-eventsub-listener/go.mod
    - services/twitch-eventsub-listener/cmd/main.go

key-decisions:
  - "coordClient still built manually with coordination.NewCoordinatorClient — NewListenerBaseFromEnv does not exist in SDK; all other migrated services use the same direct pattern"
  - "base.Start called before leader election goroutine — ensures UpdateAssignedSourceIDs is available when channelManager.Start fires on leadership acquisition"
  - "Custom Redis SetNX leader election (leaderState, tryAcquireLeadership, releaseLeadership) preserved unchanged — separate concern from SDK-owned coordinator loop"
  - "goleak pinned as direct require in go.mod before any .go file imports it — matches Phase 37 pattern"

patterns-established:
  - "Pattern: coordClient + NewListenerBase wiring — same as twitch-listener and kick-listener (assignment-based listeners)"
  - "Pattern: inline mockChannelManagerForTest stub in test file avoids importing channels.Manager DB dependency"

requirements-completed: [MIGRATE-06]

# Metrics
duration: 4min
completed: 2026-03-18
---

# Phase 38 Plan 03: twitch-eventsub-listener SDK Migration Summary

**twitch-eventsub-listener cmd/main.go wired to ListenerBase SDK — manual heartbeat, assignment refresh, migration subscriber, and JWT refresh goroutines replaced by base.Start(ctx, channelManager); goleak smoke test added; MIGRATE-06 complete; all 6 Go listeners on shared SDK**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-18T09:03:29Z
- **Completed:** 2026-03-18T09:07:20Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- go.uber.org/goleak v1.3.0 pinned as direct dep in twitch-eventsub-listener go.mod
- cmd/main_sdk_test.go created with TestListenerBase_StartStop_NoGoroutineLeak (passes, no leaks)
- cmd/main.go migrated: manual startup jitter, initial assignments block, SetAssignedSourceIDs, migration subscriber goroutine, heartbeat goroutine, assignment refresh goroutine, JWT refresh all removed
- listener.NewListenerBase(cfg, coordClient, redisClient, podName, log) wired
- base.Start(ctx, channelManager) called before leader election goroutine; defer base.Stop() added
- All getEnv() call sites replaced with listener.Env(); getEnv function deleted
- Custom Redis SetNX leader election (leaderState, tryAcquireLeadership, releaseLeadership) preserved unchanged
- go build ./... passes in twitch-eventsub-listener
- make build-all exits 0 — all 6 Go listeners build on shared SDK

## Task Commits

1. **Task 1: Pin goleak as direct dep and write goroutine leak smoke test** - `bc51109` (test)
2. **Task 2: Migrate cmd/main.go to ListenerBase SDK wiring** - `df58889` (feat)

## Files Created/Modified

- `services/twitch-eventsub-listener/go.mod` - Added go.uber.org/goleak v1.3.0 to direct require block
- `services/twitch-eventsub-listener/cmd/main_sdk_test.go` - New: goroutine leak smoke test with mockChannelManagerForTest stub
- `services/twitch-eventsub-listener/cmd/main.go` - SDK migration: removed 7 manual blocks, added NewListenerBase + base.Start + defer base.Stop, replaced all getEnv with listener.Env, deleted getEnv function

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] NewListenerBaseFromEnv does not exist in SDK**
- **Found during:** Task 2
- **Issue:** Plan interfaces block references `NewListenerBaseFromEnv(cfg, redis, podID, log)` which does not exist in shared/listener. Must_have truth "no manual coordination.NewCoordinatorClient call" conflicts with actual SDK pattern used by all 5 previously migrated listeners.
- **Fix:** Used `coordination.NewCoordinatorClient(...)` + `listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)` — the established pattern from twitch-listener (Phase 35), kick-listener (Phase 36), youtube-listener (Phase 38-01). All migrated listeners use this same pattern. The coordClient is passed to NewListenerBase, which owns JWT refresh, heartbeat, assignment refresh, and migration subscriber goroutines.
- **Files modified:** services/twitch-eventsub-listener/cmd/main.go
- **Commit:** df58889

## Issues Encountered

None beyond the plan deviation above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All 6 Go listeners (twitch, kick, youtube, youtube-innertube, discord, twitch-eventsub) are on shared SDK
- v1.6 Listener SDK milestone complete: MIGRATE-06 checked off
- make build-all passes cleanly as the cross-module compile gate

---
*Phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener*
*Completed: 2026-03-18*
