---
phase: 33-pre-migration-cleanup
plan: 02
subsystem: api
tags: [go, redis, pubsub, migration, error-handling, zap, coordination]

# Dependency graph
requires:
  - phase: 33-pre-migration-cleanup
    provides: "Plan 33-01 source ID normalization — prerequisite cleanup before SDK definition"
provides:
  - "Canonical func(*MigrationEvent) error handler signature in MigrationSubscriber"
  - "HandleMigrationEvent returning error in twitch-listener and kick-listener channel managers"
  - "Error-logging call site in consumeMessages — logs and continues on non-nil handler returns"
  - "Compilation-passing test scaffold TestMigrationSubscriber_ErrorHandling in shared/coordination"
affects:
  - phase-34-sdk-definition
  - twitch-listener
  - kick-listener
  - shared-coordination

# Tech tracking
tech-stack:
  added: [github.com/stretchr/testify (added to shared/go.mod as direct dependency)]
  patterns: [error-return-without-abort, panic-recovery-plus-error-logging, external-test-package-with-qualified-names]

key-files:
  created:
    - shared/coordination/migration_subscriber_test.go
  modified:
    - shared/coordination/migration_subscriber.go
    - services/twitch-listener/channels/manager.go
    - services/kick-listener/channels/manager.go

key-decisions:
  - "HandleMigrationEvent returns nil unconditionally — both managers log internal errors without surfacing them; error return is reserved for future fatal conditions introduced by SDK"
  - "consumeMessages logs non-nil handler errors via zap.Error and continues the event loop — panic recovery defer retained alongside error logging"
  - "Test uses package coordination_test (external) with qualified names — not package coordination (internal) — because test verifies public API surface"
  - "testify added to shared/go.mod as direct dependency to unblock test compilation"

patterns-established:
  - "Error-return-without-abort: handler errors are logged and processing continues; never abort event loops on application-level errors"
  - "Handler type evolution: func(*T) → func(*T) error — Go infers updated method type at wiring sites, no cmd/main.go changes required"

requirements-completed: [PREP-02]

# Metrics
duration: 4min
completed: 2026-03-17
---

# Phase 33 Plan 02: Pre-Migration Cleanup Summary

**MigrationSubscriber updated to func(*MigrationEvent) error handler type with error-logging loop continuation, matching canonical SDK interface pre-requisite for both Twitch and Kick listeners**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-17T16:42:49Z
- **Completed:** 2026-03-17T16:46:49Z
- **Tasks:** 2
- **Files modified:** 4 (plus go.mod)

## Accomplishments
- Updated `MigrationSubscriber` handler field and `NewMigrationSubscriber` parameter from `func(*MigrationEvent)` to `func(*MigrationEvent) error`
- Updated `consumeMessages` call site to log non-nil error returns via `zap.Error` without stopping the event loop
- Updated `twitch-listener` `HandleMigrationEvent` to return `error` (returns `nil` unconditionally)
- Updated `kick-listener` `HandleMigrationEvent` to return `error` (returns `nil` unconditionally)
- Both `cmd/main.go` wiring sites compile without any modification (Go method value type inference)
- Created `TestMigrationSubscriber_ErrorHandling` test scaffold verifying the target signature compiles and documents integration test intent

## Task Commits

Each task was committed atomically:

1. **Task 1: Create MigrationSubscriber error-handling test scaffold** - `c32addc` (test)
2. **Task 2: Update HandleMigrationEvent to return error and update MigrationSubscriber** - `eee90d9` (feat)

## Files Created/Modified
- `shared/coordination/migration_subscriber_test.go` - TestMigrationSubscriber_ErrorHandling using target func(*MigrationEvent) error signature; skips without Redis
- `shared/coordination/migration_subscriber.go` - handler field type updated; error-logging call site added in consumeMessages
- `services/twitch-listener/channels/manager.go` - HandleMigrationEvent returns error; returns nil unconditionally
- `services/kick-listener/channels/manager.go` - HandleMigrationEvent returns error; returns nil unconditionally
- `shared/go.mod` - testify added as direct dependency

## Decisions Made
- `HandleMigrationEvent` returns `nil` unconditionally in both managers — both services log internal errors locally and never need to surface them to the subscriber. The error return is a forward-compatible slot for future SDK-defined fatal conditions.
- `consumeMessages` retains its panic recovery defer alongside the new error-logging block — two separate safety nets for two separate failure modes.
- Test written in `package coordination_test` (external package) to verify the public API surface. Requires qualified names (`coordination.MigrationEvent`, `coordination.NewMigrationSubscriber`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added testify to shared/go.mod as direct dependency**
- **Found during:** Task 1 (test scaffold creation)
- **Issue:** `package coordination_test` referenced `testify/assert` and `zaptest` but testify was not in `go.mod` direct dependencies — `go test` refused to run with `go: updates to go.mod needed`
- **Fix:** Ran `go get github.com/stretchr/testify@latest` and `go mod tidy` in `shared/`
- **Files modified:** `shared/go.mod`, `shared/go.sum`
- **Verification:** `go test ./coordination/... -run TestMigrationSubscriber_ErrorHandling` exits 0
- **Committed in:** `c32addc` (Task 1 commit)

**2. [Rule 3 - Blocking] Fixed external test package qualified name references**
- **Found during:** Task 2 (GREEN phase — running tests after signature update)
- **Issue:** Test file used unqualified `MigrationEvent` and `NewMigrationSubscriber` from `package coordination_test` — external packages must use qualified names
- **Fix:** Added `"github.com/caesar/all-chat/shared/coordination"` import; updated all references to `coordination.MigrationEvent` and `coordination.NewMigrationSubscriber`
- **Files modified:** `shared/coordination/migration_subscriber_test.go`
- **Verification:** `go test ./coordination/... -run TestMigrationSubscriber_ErrorHandling` exits 0
- **Committed in:** `eee90d9` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary for compilation and test execution. No scope creep.

## Issues Encountered

**Pre-existing test failures (out of scope, documented in deferred-items.md):**
- `TestStartStopJWTRefresh` in `shared/coordination` — panic on double-close channel in `StopJWTRefresh` (verified pre-existing before 33-02 changes)
- `TestRepository_GetActiveChannelsHandlesStringChatroomIDs` in `kick-listener/channels` — expected 2 channels, got 0 (verified pre-existing before 33-02 changes; first documented in plan 33-01 deferred items)

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- PREP-02 complete: canonical `func(*MigrationEvent) error` signature established in both listeners and MigrationSubscriber
- Phase 34 (SDK Definition) can now define the `ChannelManager` interface with `HandleMigrationEvent(event *coordination.MigrationEvent) error` knowing both existing implementations already conform
- Both pre-existing test failures need resolution before a full CI green run — document in phase 34 planning

---
*Phase: 33-pre-migration-cleanup*
*Completed: 2026-03-17*

## Self-Check: PASSED

- shared/coordination/migration_subscriber_test.go: FOUND
- shared/coordination/migration_subscriber.go: FOUND
- services/twitch-listener/channels/manager.go: FOUND
- services/kick-listener/channels/manager.go: FOUND
- Commit c32addc (test scaffold): FOUND
- Commit eee90d9 (feat signature update): FOUND
