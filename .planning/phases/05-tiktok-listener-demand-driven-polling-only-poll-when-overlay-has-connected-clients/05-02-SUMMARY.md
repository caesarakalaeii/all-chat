---
phase: 05
plan: 02
subsystem: shared/listener SDK, listener ChannelManagers
tags: [demand-driven, sdk, channel-manager, redis-pubsub, go]
dependency_graph:
  requires: [05-01]
  provides: [DEMAND-04, DEMAND-07]
  affects: [kick-listener, twitch-listener, twitch-eventsub-listener, youtube-listener-innertube, discord-listener, youtube-listener]
tech_stack:
  added: [miniredis/v2 (test only, shared module)]
  patterns: [Pub/Sub demand subscriber loop, atomic.Bool for initialization gate, intersection filtering]
key_files:
  created:
    - shared/listener/demand.go
    - shared/listener/demand_test.go
    - shared/listener/testutil/redisutil/redis.go
  modified:
    - shared/listener/channel_manager.go
    - shared/listener/config.go
    - shared/listener/base.go
    - shared/listener/base_test.go
    - services/kick-listener/channels/manager.go
    - services/kick-listener/cmd/main_sdk_test.go
    - services/twitch-listener/channels/manager.go
    - services/twitch-listener/cmd/main.go
    - services/twitch-listener/cmd/main_sdk_test.go
    - services/twitch-eventsub-listener/channels/manager.go
    - services/twitch-eventsub-listener/cmd/main_sdk_test.go
    - services/youtube-listener-innertube/cmd/main_sdk_test.go
    - services/discord-listener/cmd/main_sdk_test.go
    - services/youtube-listener/cmd/main_sdk_test.go
    - shared/go.mod
    - shared/go.sum
decisions:
  - "DemandedSource type added to channel_manager.go — colocated with interface for discoverability"
  - "assignedSourceIDs tracked in ListenerBase (not just ChannelManager) to enable intersection without inter-interface calls"
  - "Redis testutil helpers moved to testutil/redisutil subpackage — prevents miniredis pulling into service-level go.mod files"
  - "trackedChannel.SourceID field added to kick-listener to enable slug->sourceID mapping in reconcileDemand without DB round-trips"
  - "twitch-eventsub-listener UpdateDemandedSourceIDs stores demand state; reconciliation deferred to SyncChannels cycle (leader-managed, stateless webhooks)"
  - "twitch-listener UpdateDemandedSourceIDs is no-op + DisableDemandFiltering=true — IRC push protocol has no per-source connection cost"
  - "delayedMockCoordinator placed in redisutil package (not testutil) — only needed by demand tests, keeps testutil lean"
metrics:
  duration: 1622s
  completed: 2026-03-27
  tasks_completed: 2
  files_changed: 16
---

# Phase 05 Plan 02: SDK Demand Subscriber Loop Summary

Extended the shared Go listener SDK with demand-driven behavior: added `UpdateDemandedSourceIDs` to the ChannelManager interface, implemented a 4th goroutine in ListenerBase that subscribes to `source:demand` Redis Pub/Sub, computes the intersection with assigned sources, and calls the new method; implemented the method in all Go listener ChannelManagers.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Extend ChannelManager interface + demand subscriber loop + TDD tests | 8c449e0 | 9 files |
| 2 | Implement UpdateDemandedSourceIDs in all Go listener ChannelManagers | 1e2c07c | 12 files |

## What Was Built

### SDK Changes (shared/listener/)

**channel_manager.go**: Added `DemandedSource` struct and `UpdateDemandedSourceIDs(demanded map[string]DemandedSource)` to the `ChannelManager` interface. An empty map means disconnect all; nil is never passed.

**config.go**: Added `DisableDemandFiltering bool` to `ListenerConfig`. When true, the demand subscriber loop exits immediately without subscribing — used by twitch IRC which always connects to all assigned channels.

**base.go**:
- Added `hasInitialAssignments atomic.Bool` field to `ListenerBase`
- Added `assignedSourceIDs map[string]bool` field (with mutex) to enable intersection in the demand loop
- Changed `wg.Add(3)` to `wg.Add(4)` and added 4th goroutine: `go b.startDemandSubscriberLoop(internalCtx, mgr)`
- Sets `hasInitialAssignments = true` after initial QueryAssignments completes
- Assignment refreshes also update `b.assignedSourceIDs`

**demand.go**: New file implementing:
- `startDemandSubscriberLoop`: outer retry loop with exponential backoff (1s→30s), matching migration subscriber pattern; exits immediately when DisableDemandFiltering=true or redisClient==nil
- `runDemandSubscriber`: subscribes to `source:demand` channel, processes messages in a select loop
- `reconcileDemand`: applies the hasInitialAssignments gate, computes intersection (assigned ∩ demanded ∩ platform-filtered), calls `mgr.UpdateDemandedSourceIDs`

### Test Infrastructure (shared/listener/testutil/redisutil/)

New `redisutil` subpackage (separate from `testutil`) provides:
- `StartTestRedisWithClient(t)` — starts miniredis + returns go-redis client; caller controls lifecycle for goleak compatibility
- `DelayedMockCoordinator` — delays QueryAssignments to simulate slow initial assignment loading (used by TestDemandBeforeAssignments)

**5 TDD tests** in demand_test.go, all passing:
- `TestDemandFiltering`: intersection {A,B,C} ∩ {A,D} = {A}
- `TestDemandBeforeAssignments`: demand update ignored before hasInitialAssignments=true
- `TestDemandWithDisableFiltering`: DisableDemandFiltering=true → UpdateDemandedSourceIDs never called
- `TestDemandEmptySources`: empty sources → empty demanded map (disconnect all)
- `TestDemandSubscriberReconnect`: after stop+restart, new demand update processed

### ChannelManager Implementations

**kick-listener/channels/manager.go** (full demand):
- Added `demandedSourceIDs map[string]listener.DemandedSource` field
- Added `SourceID string` field to `trackedChannel` struct; populated in `buildChannelPlans`
- `UpdateDemandedSourceIDs`: stores demanded set, calls `reconcileDemand()`
- `reconcileDemand`: iterates active subscriptions, unsubscribes channels whose SourceID is not in demanded, releases leadership

**twitch-listener/channels/manager.go** (no-op):
- `UpdateDemandedSourceIDs` is a no-op — Twitch IRC always connected to all assigned channels
- `DisableDemandFiltering: true` set in cmd/main.go so the SDK loop never calls this method

**twitch-eventsub-listener/channels/manager.go** (store only):
- Added `demandedSourceIDs map[string]listener.DemandedSource` field
- `UpdateDemandedSourceIDs`: stores the demanded set; reconciliation deferred to SyncChannels cycle (EventSub subscriptions are leader-managed and stateless)

**All 7 mock ChannelManagers** in service-level SDK smoke tests updated with `UpdateDemandedSourceIDs(_ map[string]listener.DemandedSource) {}` stub.

## Verification

```
cd shared/listener && go test ./... -v -count=1    # 18 tests, all PASS
make build-all                                      # All 6 listener services compile
cd services/kick-listener && go test ./cmd/... -v  # PASS
cd services/twitch-eventsub-listener && go test ./cmd/... -v # PASS
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Moved Redis testutil to separate redisutil subpackage**
- **Found during:** Task 2 — service tests failed with "missing go.sum entry for miniredis"
- **Issue:** `testutil` package imported miniredis; service-level go.mod files don't have this dependency
- **Fix:** Created `testutil/redisutil` subpackage for Redis helpers; services only import `testutil` (MockCoordinator) without pulling in miniredis
- **Files modified:** `shared/listener/testutil/redisutil/redis.go` (created), `shared/listener/demand_test.go` (import updated)
- **Commit:** 1e2c07c

**2. [Rule 2 - Missing Critical] Added SourceID field to kick-listener trackedChannel**
- **Found during:** Task 2 — `reconcileDemand` needed slug→sourceID mapping without DB calls
- **Issue:** `trackedChannel` struct didn't have `SourceID`; the only way to cross-reference demanded map was DB round-trip
- **Fix:** Added `SourceID string` field to `trackedChannel`, populated in `buildChannelPlans` from `ActiveChannel.SourceID`
- **Files modified:** `services/kick-listener/channels/manager.go`
- **Commit:** 1e2c07c

**3. [Rule 2 - Missing Critical] Added kick-listener and twitch-eventsub-listener SDK smoke tests to UpdateDemandedSourceIDs update scope**
- **Found during:** Task 2 — `cmd/main_sdk_test.go` in kick-listener and twitch-eventsub-listener also had mock channel managers not listed in the plan
- **Issue:** Plan listed 3 mock files but there are 5 service-level mock files
- **Fix:** Updated all 5 mock files (kick-listener/cmd, twitch-listener/cmd, twitch-eventsub-listener/cmd, youtube-listener-innertube/cmd, discord-listener/cmd, youtube-listener/cmd — 6 total)
- **Commit:** 1e2c07c

## Self-Check: PASSED

Files exist:
- shared/listener/demand.go ✓
- shared/listener/demand_test.go ✓
- shared/listener/channel_manager.go (contains UpdateDemandedSourceIDs) ✓
- shared/listener/testutil/redisutil/redis.go ✓

Commits exist:
- 8c449e0 ✓
- 1e2c07c ✓
