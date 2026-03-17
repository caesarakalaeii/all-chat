---
phase: 36-migrate-kick-listener
plan: 01
subsystem: infra
tags: [go, kick-listener, goleak, sdk, listener, compile-time-assertion]

# Dependency graph
requires:
  - phase: 34-sdk-definition
    provides: "shared/listener ChannelManager interface (7 methods)"
  - phase: 35-migrate-twitch-listener
    provides: "Pattern: compile-time assertion + goleak direct dep as forward dep"
provides:
  - "compile-time ChannelManager assertion in kick-listener channels/manager.go"
  - "go.uber.org/goleak v1.3.0 as direct dep in kick-listener go.mod"
affects:
  - 36-migrate-kick-listener (plan 02 — smoke test imports goleak)

# Tech tracking
tech-stack:
  added: [go.uber.org/goleak v1.3.0]
  patterns: [compile-time interface assertion, forward direct dep before import]

key-files:
  created: []
  modified:
    - services/kick-listener/channels/manager.go
    - services/kick-listener/go.mod
    - services/kick-listener/go.sum

key-decisions:
  - "goleak placed in direct require block (not indirect) — forward dep for plan 02 smoke test before any .go file imports it"
  - "var _ listener.ChannelManager = (*Manager)(nil) assertion added to channels/manager.go — build fails immediately if Manager drifts from 7-method SDK interface"

patterns-established:
  - "Compile-time assertion placed immediately after imports, before const block — same placement as twitch-listener"
  - "Forward direct dep: add to first require block manually after go mod tidy removes it (no .go file imports goleak yet)"

requirements-completed: [MIGRATE-02]

# Metrics
duration: 5min
completed: 2026-03-17
---

# Phase 36 Plan 01: Migrate Kick Listener (Wave 0 Prerequisites) Summary

**Compile-time ChannelManager assertion and goleak direct dep added to kick-listener — Wave 0 prerequisites unlocking Plan 02 smoke test**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-17T21:50:00Z
- **Completed:** 2026-03-17T21:55:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added `var _ listener.ChannelManager = (*Manager)(nil)` to channels/manager.go — build fails immediately if any of the 7 interface methods drift
- Added `shared/listener` import to channels/manager.go alongside existing coordination/sourcemanager imports
- Registered `go.uber.org/goleak v1.3.0` as a direct (not indirect) dep in kick-listener go.mod for Plan 02 smoke test

## Task Commits

Each task was committed atomically:

1. **Task 1: Add compile-time ChannelManager assertion** - `81af164` (feat)
2. **Task 2: Add goleak as direct dependency** - `fd693a4` (chore)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `services/kick-listener/channels/manager.go` - Added shared/listener import and compile-time assertion line
- `services/kick-listener/go.mod` - Added go.uber.org/goleak v1.3.0 in direct require block
- `services/kick-listener/go.sum` - Updated with goleak checksums

## Decisions Made
- goleak placed in direct require block (not indirect) — forward dep for plan 02 smoke test before any .go file imports it; `go mod tidy` removes it so it must be added manually to the first require block
- Compile-time assertion placed in same position as twitch-listener: after imports, before const block, with comment "Compile-time assertion: Manager must satisfy the SDK ChannelManager interface."

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `go mod tidy` removed goleak from go.mod because no .go file imports it yet. Fixed by manually placing it in the direct require block after tidy ran. This matches the approach documented in STATE.md for twitch-listener Phase 35.
- `TestRepository_GetActiveChannelsHandlesStringChatroomIDs` fails in the channels package — confirmed pre-existing failure (present before this plan's changes). Out of scope per deviation rules. The plan's specified test `TestManager_SourceIDNormalization` passes green.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 02 can now proceed: goleak is a direct dep, Manager satisfies ChannelManager at compile time
- kick-listener builds cleanly with SDK interface assertion active

---
## Self-Check: PASSED

- FOUND: services/kick-listener/channels/manager.go
- FOUND: services/kick-listener/go.mod
- FOUND: .planning/phases/36-migrate-kick-listener/36-01-SUMMARY.md
- FOUND: commit 81af164 (feat: compile-time assertion)
- FOUND: commit fd693a4 (chore: goleak direct dep)
- FOUND: `var _ listener.ChannelManager = (*Manager)(nil)` at line 30
- FOUND: `go.uber.org/goleak v1.3.0` in direct require block of go.mod

---
*Phase: 36-migrate-kick-listener*
*Completed: 2026-03-17*
