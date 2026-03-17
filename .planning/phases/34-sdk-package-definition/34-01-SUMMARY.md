---
phase: 34-sdk-package-definition
plan: 01
subsystem: shared/coordination, shared/listener, kick-listener, twitch-listener, twitch-eventsub-listener
tags: [sdk, interface, coordinator-client, channel-manager]
dependency_graph:
  requires: []
  provides:
    - shared/listener/channel_manager.go (ChannelManager interface)
    - shared/coordination/client.go (explicit serviceName parameter)
  affects:
    - services/twitch-listener/cmd/main.go
    - services/kick-listener/cmd/main.go
    - services/kick-listener/channels/manager.go
    - services/twitch-eventsub-listener/cmd/main.go
    - services/twitch-eventsub-listener/channels/manager.go
tech_stack:
  added: []
  patterns:
    - "Explicit serviceName parameter — caller responsibility, no hostname auto-detection"
    - "sync.Once for idempotent channel close (StopJWTRefresh)"
    - "Interface compliance via blank identifier assertions deferred to Phase 35"
key_files:
  created:
    - shared/listener/channel_manager.go
  modified:
    - shared/coordination/client.go
    - shared/coordination/client_jwt_test.go
    - services/kick-listener/channels/manager.go
    - services/kick-listener/cmd/main.go
    - services/twitch-listener/cmd/main.go
    - services/twitch-eventsub-listener/cmd/main.go
    - services/twitch-eventsub-listener/channels/manager.go
decisions:
  - "serviceName passed explicitly by each listener caller — no hostname auto-detection"
  - "ChannelManager interface defined in shared/listener package with 7 methods"
  - "kick-listener Start accepts ctx context.Context but ignores it (uses internal m.ctx)"
  - "compile-time assertions deferred to Phase 35 per CONTEXT.md lock"
metrics:
  duration: "4m"
  completed_date: "2026-03-17"
  tasks_completed: 3
  files_changed: 8
---

# Phase 34 Plan 01: SDK Contract Foundations Summary

**One-liner:** Explicit-serviceName coordinator client and 7-method ChannelManager interface enabling type-safe listener SDK migration.

## What Was Built

### Task 1: Update NewCoordinatorClient to accept explicit serviceName

`shared/coordination/client.go` now accepts `serviceName string` as the third parameter (before `logger`). The hostname auto-detection block (`os.Getenv("HOSTNAME")` + `HasPrefix` chain) is removed entirely. The `os` import was removed; `strings` is retained because `strings.NewReader` is used in `PublishHeartbeat`.

All 3 call sites in `client_jwt_test.go` updated to pass `"test-service"` as the third argument.

### Task 2: Define ChannelManager interface and fix kick-listener Start signature

Created `shared/listener/channel_manager.go` with the `ChannelManager` interface:

```go
type ChannelManager interface {
    Start(ctx context.Context) error
    Stop()
    HandleMigrationEvent(event *coordination.MigrationEvent) error
    UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
    GetFilteredAssignmentCount() int
    GetActiveChannels() []string
    GetActiveChannelCount() int
}
```

kick-listener `channels/manager.go`:
- `Start() error` changed to `Start(_ context.Context) error` with explanatory comment
- Added `GetActiveChannels() []string` — returns slugs from `m.subscriptions`
- Added `GetActiveChannelCount() int` — returns `len(m.subscriptions)`
- `cmd/main.go` updated to call `channelMgr.Start(ctx)`

### Task 3: Update NewCoordinatorClient callers in all 3 listener services

- `twitch-listener/cmd/main.go`: passes `"twitch-listener"` as serviceName
- `kick-listener/cmd/main.go`: passes `"kick-listener"` as serviceName
- `twitch-eventsub-listener/cmd/main.go`: passes `"twitch-eventsub-listener"` as serviceName

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed HandleMigrationEvent return type in twitch-eventsub-listener**
- **Found during:** Task 3 — build verification
- **Issue:** `channels/manager.go HandleMigrationEvent` had no return value, but `NewMigrationSubscriber` expects `func(*MigrationEvent) error` — pre-existing type mismatch preventing compilation
- **Fix:** Added `error` return type; returns `nil` unconditionally (matches Phase 33 decision for forward-compatible error slot)
- **Files modified:** `services/twitch-eventsub-listener/channels/manager.go`
- **Commit:** `61c30e0`

**2. [Rule 1 - Bug] Fixed StopJWTRefresh double-close panic**
- **Found during:** Final verification — `TestStartStopJWTRefresh` panicked on `close of closed channel`
- **Issue:** `StopJWTRefresh` closed `stopRefresh` channel without guarding against multiple calls; test explicitly calls it twice to verify idempotency
- **Fix:** Added `sync.Once stopOnce` field to `CoordinatorClient`; `StopJWTRefresh` now calls `c.stopOnce.Do(func() { close(c.stopRefresh) })`
- **Files modified:** `shared/coordination/client.go`
- **Commit:** `2ea1d12`

### Out-of-Scope (Deferred)

- `TestRepository_GetActiveChannelsHandlesStringChatroomIDs` in kick-listener — pre-existing test failure requiring live database; not caused by these changes; logged for awareness

## Commits

| Hash | Description |
|------|-------------|
| `f8aa171` | feat(34-01): update NewCoordinatorClient to accept explicit serviceName |
| `16363b7` | feat(34-01): define ChannelManager interface and fix kick-listener Start signature |
| `61c30e0` | feat(34-01): update NewCoordinatorClient callers in all 3 listener services |
| `2ea1d12` | fix(34-01): prevent StopJWTRefresh panic on double close |

## Verification Results

All exit 0:
- `cd shared && go build ./... && go test ./coordination/... -v -count=1` — 3 PASS, 1 SKIP (Redis integration)
- `cd services/twitch-listener && go build ./...` — PASS
- `cd services/kick-listener && go build ./...` — PASS
- `cd services/twitch-eventsub-listener && go build ./...` — PASS

## Self-Check: PASSED

All files verified present. All commits verified in git log.
