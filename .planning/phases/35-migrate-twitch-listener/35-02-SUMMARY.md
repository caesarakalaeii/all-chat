---
phase: 35-migrate-twitch-listener
plan: 02
subsystem: infra
tags: [go, twitch-listener, listener-sdk, goroutine-leak, goleak, sdk-migration]

requires:
  - phase: 35-migrate-twitch-listener
    provides: "Compile-time ChannelManager assertion and goleak direct dependency in twitch-listener"
  - phase: 34-sdk-definition
    provides: "shared/listener package: ListenerBase, ListenerConfig, ChannelManager interface, ShutdownCoordinator, testutil.MockCoordinator"

provides:
  - "SDK-wired twitch-listener cmd/main.go using listener.NewListenerBase + base.Start + listener.ShutdownCoordinator"
  - "Goroutine leak smoke test cmd/main_sdk_test.go with goleak.VerifyNone — first production SDK validation"
  - "Removal of all inline goroutines (heartbeat, assignment refresh, migration subscriber) from twitch-listener main.go"

affects:
  - 36-migrate-kick-listener (same SDK wiring pattern: NewListenerBase, ShutdownCoordinator, listener.Env)
  - 37-migrate-discord-listener

tech-stack:
  added: []
  patterns:
    - "listener.NewListenerBase replaces 3 inline goroutines + JWT refresh lifecycle in listener main.go"
    - "listener.ShutdownCoordinator replaces manual channelMgr.Stop + platform disconnect + srv.Shutdown sequence"
    - "listener.Env replaces getEnvOrDefault — delete the local helper, use SDK drop-in"
    - "Pass nil assignedSourceIDs to channels.NewManager; SDK populates via UpdateAssignedSourceIDs inside base.Start"
    - "Inline mockChannelManagerForTest in test file — 7 no-op methods, no real DB/Redis required"

key-files:
  created:
    - services/twitch-listener/cmd/main_sdk_test.go
  modified:
    - services/twitch-listener/cmd/main.go

key-decisions:
  - "listener.Env used as drop-in for all getEnvOrDefault calls — getEnvOrDefault function deleted entirely"
  - "nil passed to channels.NewManager for assignedSourceIDs — SDK owns initial assignment query and UpdateAssignedSourceIDs call inside base.Start"
  - "IRC Connect + 2s sleep placed before base.Start — IRC-specific ordering requirement preserved as mandated by plan"
  - "OnFatalError left unset in DefaultConfig — goroutines retry indefinitely with exponential backoff (production-safe default)"

patterns-established:
  - "Smoke test pattern: inline mockChannelManagerForTest struct + testutil.MockCoordinator + nil redisClient + fast intervals (20ms) + goleak.VerifyNone"
  - "SDK migration pattern: add shared/listener import, remove math/rand + strings, replace inline goroutines with NewListenerBase + base.Start + ShutdownCoordinator"

requirements-completed: [MIGRATE-01, VERIFY-02]

duration: 8min
completed: 2026-03-17
---

# Phase 35 Plan 02: Migrate Twitch Listener Summary

**twitch-listener cmd/main.go migrated to ListenerBase SDK — 3 inline goroutines + JWT refresh + manual shutdown replaced by listener.NewListenerBase + base.Start + listener.ShutdownCoordinator, validated by goleak smoke test**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-17T21:17:40Z
- **Completed:** 2026-03-17T21:25:00Z
- **Tasks:** 3 (2 code tasks + 1 verification gate)
- **Files modified:** 2

## Accomplishments

- Created `cmd/main_sdk_test.go` with `TestListenerBase_StartStop_NoGoroutineLeak` — goleak.VerifyNone confirms zero goroutine leaks from SDK wiring; first production-level SDK validation
- Rewrote `cmd/main.go` — removed 8 inline code blocks (heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine, StartJWTRefresh/StopJWTRefresh, startup jitter, initial QueryAssignments, channelMgr.Start, manual shutdown) and replaced with `listener.NewListenerBase` + `base.Start` + `listener.ShutdownCoordinator`
- Replaced `getEnvOrDefault` helper throughout with `listener.Env` and deleted the local function; removed `math/rand` and `strings` imports
- All tests pass: channels/, irc/, publisher/, models/, cmd/ — including with `-race` flag; `make build-all` exits 0 with no cross-module regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Write smoke test stub cmd/main_sdk_test.go** - `fcfd113` (test)
2. **Task 2: Rewrite cmd/main.go using SDK wiring** - `bc914a7` (feat)
3. **Task 3: Full test suite — all existing tests pass** - (verification only, no new commit)

## Files Created/Modified

- `services/twitch-listener/cmd/main_sdk_test.go` - Goroutine leak smoke test; inline mockChannelManagerForTest (7 no-op methods), testutil.MockCoordinator, goleak.VerifyNone
- `services/twitch-listener/cmd/main.go` - SDK-wired startup using ListenerBase; all inline goroutines removed; -120 lines net

## Decisions Made

- `listener.Env` used as a drop-in for all `getEnvOrDefault` calls — the local helper is deleted, no migration shim needed
- `nil` passed to `channels.NewManager` for `assignedSourceIDs` — SDK populates the map via `UpdateAssignedSourceIDs` inside `base.Start`, matching the anti-pattern warning in the plan
- IRC `Connect` + `time.Sleep(2s)` remain before `base.Start` — IRC-specific ordering enforced by code sequence as required
- `OnFatalError` left unset (nil) in `DefaultConfig()` — goroutines retry with exponential backoff, which is the backward-compatible production-safe behavior

## Deviations from Plan

None — plan executed exactly as written. The TDD task went directly GREEN (no RED phase needed) because plan 01 already added goleak to go.mod and the shared listener SDK was fully in place.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- twitch-listener is the first listener migrated to the SDK archetype; the migration pattern is now proven in production code
- Kick-listener migration (phase 36) can follow the same pattern: NewListenerBase/ShutdownCoordinator/listener.Env swap, inline goroutine removal, nil assignedSourceIDs
- The goleak smoke test structure (inline mock, 20ms intervals, nil redisClient) is the canonical test template for all future listener SDK migrations

## Self-Check: PASSED

All files and commits verified present.

---
*Phase: 35-migrate-twitch-listener*
*Completed: 2026-03-17*
