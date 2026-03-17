---
phase: 36-migrate-kick-listener
plan: "02"
subsystem: kick-listener
tags: [sdk, migration, leadershiplistener, goroutine-leak, goleak]
dependency_graph:
  requires: [36-01, shared/listener]
  provides: [kick-listener SDK wiring, LeadershipListener archetype validation]
  affects: [kick-listener deployment, SDK migration confidence]
tech_stack:
  added: []
  patterns: [ListenerBase.Start, NewLeadershipListenerFromEnv, ShutdownCoordinator, listener.Env, goleak.VerifyNone]
key_files:
  created:
    - services/kick-listener/cmd/main_sdk_test.go
  modified:
    - services/kick-listener/cmd/main.go
decisions:
  - "nil passed to NewListenerBase for logger (not zap.NewNop()) — matches established twitch-listener smoke test pattern from Phase 35"
  - "Pre-existing TestRepository_GetActiveChannelsHandlesStringChatroomIDs failure logged as deferred — not caused by this plan, scope boundary applies"
metrics:
  duration: "3m"
  completed_date: "2026-03-17"
  tasks_completed: 2
  files_modified: 2
---

# Phase 36 Plan 02: Migrate Kick-Listener SDK Wiring Summary

**One-liner:** SDK-wired kick-listener using NewLeadershipListenerFromEnv + l.Start + ShutdownCoordinator, removing ~120 lines of inline goroutine blocks and manual LeadershipCoordinator construction.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Write goleak smoke test cmd/main_sdk_test.go | e6d117d | services/kick-listener/cmd/main_sdk_test.go |
| 2 | Rewrite cmd/main.go using SDK wiring (NewLeadershipListenerFromEnv) | 33a8315 | services/kick-listener/cmd/main.go |

## What Was Built

**cmd/main_sdk_test.go** — Goroutine leak smoke test using goleak.VerifyNone. Uses testutil.MockCoordinator and nil redisClient (no real Redis/DB required). Fast intervals (20ms heartbeat/refresh, 0 jitter) keep test under 100ms. Confirms zero leaked goroutines from the SDK-wired ListenerBase lifecycle.

**cmd/main.go (rewritten)** — Full SDK migration:
- `NewLeadershipListenerFromEnv(base, "kick", log)` replaces manual `tokenSource` + `smClient` + `leaderCoord` construction
- `l.Start(ctx, channelMgr)` replaces: startup jitter, initial QueryAssignments, channelMgr.Start, heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine, StartJWTRefresh
- `listener.ShutdownCoordinator(l.ListenerBase, channelMgr, func() { _ = wsClient.Disconnect() }, srv, log)` replaces manual shutdown sequence
- `listener.Env(...)` replaces deleted `getEnvOrDefault` function
- `l.LeadershipCoordinator()` passed to channels.NewManager (was manual `leaderCoord`)
- `nil` passed for assignedSourceIDs to channels.NewManager (SDK populates via UpdateAssignedSourceIDs in l.Start)
- Kick-specific `handleReconnections` goroutine preserved unchanged

**Net reduction: ~120 lines** (41 added, 162 removed in Task 2 commit).

## Verification Results

- `go build ./...` exits 0
- `go test ./cmd/... -race -count=1` exits 0 (both tests pass)
- `TestListenerBase_StartStop_NoGoroutineLeak` passes, goleak.VerifyNone confirms zero leaks
- `make build-all` exits 0 (no cross-module regressions)
- All done criteria confirmed:
  - `grep "NewLeadershipListenerFromEnv" cmd/main.go` — match found (line 115)
  - `grep "getEnvOrDefault" cmd/main.go` — no match
  - `grep "StartJWTRefresh" cmd/main.go` — no match
  - `grep "leaderCoord" cmd/main.go` — no match

## Deviations from Plan

### Out-of-Scope Pre-existing Issue

**Pre-existing test failure: TestRepository_GetActiveChannelsHandlesStringChatroomIDs**
- **Found during:** Task 2 verification
- **Status:** Pre-existed before this plan (confirmed via git stash test)
- **Action:** Logged to deferred items — not caused by this plan, scope boundary applies
- **Commit:** Not fixed (out of scope)

### Minor Deviation

**logger argument: nil vs zap.NewNop()**
- **Plan specified:** `zap.NewNop()` in the test
- **Actual:** `nil` — matches the established twitch-listener/cmd/main_sdk_test.go pattern from Phase 35
- **Reason:** `NewListenerBase` accepts `nil` logger (safe), and consistency with the established pattern is preferable

## Deferred Items

- `TestRepository_GetActiveChannelsHandlesStringChatroomIDs` in `services/kick-listener/channels/` — pre-existing failure, not related to SDK migration

## Self-Check: PASSED

- `services/kick-listener/cmd/main_sdk_test.go` — exists
- `services/kick-listener/cmd/main.go` — exists, modified
- Commit `e6d117d` — exists (Task 1)
- Commit `33a8315` — exists (Task 2)
