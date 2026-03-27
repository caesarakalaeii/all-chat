---
phase: 06-unify-all-listeners-to-leadership-based-coordination
plan: 01
subsystem: infra
tags: [go, listener-sdk, leadership, redis, pubsub]

# Dependency graph
requires:
  - phase: 05-tiktok-demand-driven-polling
    provides: demand subscriber loop on ListenerBase
  - phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener
    provides: all 6 Go listeners on shared SDK (ListenerBase + LeadershipListener)
provides:
  - LeadershipListener as sole SDK entry point in shared/listener
  - Merged demand subscriber loop on LeadershipListener (platform-only filter)
  - ChannelManager interface with 6 methods (HandleMigrationEvent removed)
  - ShutdownCoordinator accepting interface{ Stop() } instead of *ListenerBase
  - ListenerBase fully deleted
affects: [06-02, 06-03, all listener cmd/main.go files]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - LeadershipListener standalone struct (no embed) owns demand loop + leadership coordination
    - reconcileDemand uses platform-only filter (no assignedSourceIDs intersection)
    - ShutdownCoordinator accepts interface{ Stop() } for type-decoupled callers

key-files:
  created: []
  modified:
    - shared/listener/leadership.go
    - shared/listener/channel_manager.go
    - shared/listener/config.go
    - shared/listener/demand.go
    - shared/listener/shutdown.go
    - shared/listener/base_test.go
    - shared/listener/demand_test.go
    - shared/listener/leadership_test.go
    - shared/listener/shutdown_test.go
    - shared/listener/testutil/redisutil/redis.go
    - services/twitch-listener/channels/manager.go
    - services/twitch-listener/cmd/main_sdk_test.go
    - services/kick-listener/channels/manager.go
    - services/kick-listener/cmd/main_sdk_test.go
    - services/twitch-eventsub-listener/channels/manager.go
    - services/twitch-eventsub-listener/cmd/main_sdk_test.go

key-decisions:
  - "LeadershipListener is standalone (no embed) — eliminates dual ListenerBase/LeadershipListener hierarchy as designed in D-06"
  - "reconcileDemand simplified to platform-only filter — no assignedSourceIDs intersection needed in leadership-only model"
  - "UpdateAssignedSourceIDs kept in ChannelManager interface as no-op slot per research open question 1 (interface stability)"
  - "DeletedMockCoordinator removed from redisutil — coordinatorClient interface no longer exists"
  - "Listener cmd/main.go files intentionally NOT modified — they still reference ListenerBase; fixed in Plan 02"

patterns-established:
  - "NewLeadershipListener(config, redis, logger) for test construction without env reads"
  - "NewLeadershipListenerFromEnv(platform, redis, logger) for production construction"
  - "Demand loop started only if !DisableDemandFiltering && redisClient != nil"

requirements-completed: [D-06, D-07, D-08, D-09, D-14]

# Metrics
duration: 15min
completed: 2026-03-27
---

# Phase 06 Plan 01: Merge ListenerBase into LeadershipListener Summary

**LeadershipListener is now the sole SDK entry type in shared/listener — ListenerBase deleted, ChannelManager reduced to 6 methods, demand loop uses platform-only filter, all 3 listener channel managers have HandleMigrationEvent removed**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-27T22:40:00Z
- **Completed:** 2026-03-27T22:55:00Z
- **Tasks:** 1
- **Files modified:** 18 (2 deleted, 16 modified)

## Accomplishments
- Deleted `shared/listener/base.go` (ListenerBase, coordinatorClient, heartbeat/assignment-refresh/migration loops — all removed)
- Rewrote `leadership.go` as standalone struct: `NewLeadershipListenerFromEnv(platform, redis, logger)` and `NewLeadershipListener(config, redis, logger)` constructors; `Start`/`Stop` methods
- Updated `demand.go`: receivers changed from `*ListenerBase` to `*LeadershipListener`; `reconcileDemand` simplified to platform-only filter (removed `hasInitialAssignments` guard and `assignedSourceIDs` intersection)
- Updated `channel_manager.go`: `HandleMigrationEvent` removed, `shared/coordination` import removed, 6 methods remain
- Updated `config.go`: `ListenerConfig` and `DefaultConfig` removed; only `Env()` helper retained
- Updated `shutdown.go`: first param changed from `*ListenerBase` to `interface{ Stop() }`
- Removed `HandleMigrationEvent` from twitch-listener, kick-listener, twitch-eventsub-listener channel managers and their associated migration helpers
- Updated all 3 listener smoke tests to use `NewLeadershipListener` config-based constructor
- Removed `DelayedMockCoordinator` from redisutil (coordinatorClient interface gone)

## Task Commits

1. **Task 1: Merge ListenerBase into LeadershipListener and update interfaces** - `eaa71e4` (feat)

## Files Created/Modified
- `shared/listener/leadership.go` - Standalone LeadershipListener struct, both constructors, Start/Stop/demand loop
- `shared/listener/base.go` - DELETED
- `shared/listener/channel_manager.go` - HandleMigrationEvent removed, 6-method interface
- `shared/listener/config.go` - Only Env() helper remains
- `shared/listener/demand.go` - Receivers updated to *LeadershipListener, reconcileDemand simplified
- `shared/listener/shutdown.go` - ShutdownCoordinator accepts interface{ Stop() }
- `shared/listener/testutil/mock_coordinator.go` - DELETED
- `shared/listener/testutil/redisutil/redis.go` - DelayedMockCoordinator removed
- `shared/listener/base_test.go` - Rewritten to test LeadershipListener
- `shared/listener/demand_test.go` - Rewritten for platform-only filter semantics
- `shared/listener/leadership_test.go` - Updated for new constructor signatures
- `shared/listener/shutdown_test.go` - Updated to use LeadershipListener
- `services/twitch-listener/channels/manager.go` - HandleMigrationEvent and migration helpers removed
- `services/twitch-listener/cmd/main_sdk_test.go` - Updated to NewLeadershipListener
- `services/kick-listener/channels/manager.go` - HandleMigrationEvent and migration helpers removed
- `services/kick-listener/cmd/main_sdk_test.go` - Updated to NewLeadershipListener
- `services/twitch-eventsub-listener/channels/manager.go` - HandleMigrationEvent removed
- `services/twitch-eventsub-listener/cmd/main_sdk_test.go` - Updated to NewLeadershipListener

## Decisions Made
- `UpdateAssignedSourceIDs` kept in ChannelManager interface as a no-op slot — no SDK code calls it, but removing would be an unnecessary breaking change in one step; Plan 02 can remove it
- `reconcileDemand` now passes ALL platform-matching sources through without checking assignment state — leadership-based model doesn't need assignment intersection
- `ShutdownCoordinator` now accepts `interface{ Stop() }` — decouples from concrete type, allows both `*LeadershipListener` and future implementations

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] Removed unused imports after HandleMigrationEvent deletion**
- **Found during:** Task 1 (cleaning twitch-listener and kick-listener managers)
- **Issue:** After removing HandleMigrationEvent and migration helpers, several imports (otel, trace, attribute, propagation, os, shared/coordination) became unused
- **Fix:** Removed all unused imports from twitch-listener/channels/manager.go and kick-listener/channels/manager.go
- **Files modified:** services/twitch-listener/channels/manager.go, services/kick-listener/channels/manager.go
- **Committed in:** eaa71e4

**2. [Rule 2 - Missing] Made logger nil-safe in ShutdownCoordinator**
- **Found during:** Task 1 (writing shutdown_test.go)
- **Issue:** Tests pass nil logger; `logger.Error(...)` in error path would panic
- **Fix:** Added nil-guard `if logger != nil` around the error log in srv.Shutdown error path
- **Files modified:** shared/listener/shutdown.go
- **Committed in:** eaa71e4

**3. [Rule 1 - Bug] Removed DelayedMockCoordinator from redisutil**
- **Found during:** Task 1 (verifying `grep -r "shared/coordination" shared/listener/`)
- **Issue:** redisutil/redis.go imported `shared/coordination` for DelayedMockCoordinator, which was only needed for the `TestDemandBeforeAssignments` test that relied on `hasInitialAssignments` (now removed). The grep check would fail.
- **Fix:** Removed DelayedMockCoordinator type and its `shared/coordination` import from redisutil
- **Files modified:** shared/listener/testutil/redisutil/redis.go
- **Committed in:** eaa71e4

---

**Total deviations:** 3 auto-fixed (1 missing imports cleanup, 1 nil-safety, 1 stale test helper)
**Impact on plan:** All auto-fixes necessary for correctness and compliance with acceptance criteria. No scope creep.

## Issues Encountered
None - plan executed cleanly.

## Known Stubs
None — the listener SDK changes are structural refactors; no data flows are stubbed.

## Next Phase Readiness
- `shared/listener` package compiles and all tests pass with race detector
- `LeadershipListener` is the sole SDK entry point; `ListenerBase` fully deleted
- All 3 listener channel managers have `HandleMigrationEvent` removed
- Listener `cmd/main.go` files still reference `ListenerBase` and will fail to compile — Plan 02 migrates them

---
*Phase: 06-unify-all-listeners-to-leadership-based-coordination*
*Completed: 2026-03-27*

## Self-Check: PASSED

- shared/listener/leadership.go: FOUND
- shared/listener/channel_manager.go: FOUND
- shared/listener/config.go: FOUND
- shared/listener/demand.go: FOUND
- shared/listener/shutdown.go: FOUND
- shared/listener/base.go: CONFIRMED DELETED
- shared/listener/testutil/mock_coordinator.go: CONFIRMED DELETED
- Commit eaa71e4: FOUND
