---
phase: 33-pre-migration-cleanup
plan: "01"
subsystem: kick-listener, twitch-listener
tags: [source-id-normalization, bug-fix, coordinator, assignment-filtering]
dependency_graph:
  requires: []
  provides: [bare-uuid-assignment-maps-in-kick-and-twitch-listener]
  affects: [channels.Manager, coordinator-assignment-filtering]
tech_stack:
  added: []
  patterns: [strip-at-intake, strings.LastIndexByte]
key_files:
  created:
    - services/kick-listener/channels/manager_test.go
  modified:
    - services/kick-listener/cmd/main.go
    - services/twitch-listener/cmd/main.go
decisions:
  - "Strip platform suffix at the point of intake (cmd/main.go), not inside channels.Manager — keeps manager interface simple and consistent"
  - "Use strings.LastIndexByte for suffix stripping to match the already-correct twitch-listener refresh loop pattern"
metrics:
  duration: "107 minutes"
  completed: "2026-03-17"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 2
---

# Phase 33 Plan 01: Source ID Intake Normalization Summary

**One-liner:** Strip `:kick`/`:twitch` coordinator suffix at assignment extraction in both listeners so `assignedSourceIDs` maps always hold bare UUIDs.

## What Was Done

Both kick-listener and twitch-listener had assignment extraction loops that stored composite coordinator IDs (e.g., `"abc123:kick"`) directly into the `assignedSourceIDs` map. The `channels.Manager` compares these map keys against bare UUIDs from the database, causing all channels to appear unassigned (the filter never matches).

The fix applies the `strings.LastIndexByte`-based strip-at-intake pattern — already correct in the twitch-listener refresh loop — to the three missing locations:

1. `kick-listener/cmd/main.go` — initial extraction (startup path)
2. `kick-listener/cmd/main.go` — refresh loop (60-second periodic update)
3. `twitch-listener/cmd/main.go` — initial extraction (startup path)

The twitch-listener refresh loop was already correct and was not touched.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create kick-listener manager test scaffold | 76848ec | services/kick-listener/channels/manager_test.go |
| 2 | Apply strip-at-intake to all four assignment extraction paths | bde7882 | services/kick-listener/cmd/main.go, services/twitch-listener/cmd/main.go |

## Decisions Made

- **Strip at intake, not inside Manager**: The normalization is applied in `cmd/main.go` at the point where the `assignedSourceIDs` map is built. This keeps `channels.Manager` unaware of coordinator suffix formats and matches the existing twitch-listener refresh loop convention.
- **strings.LastIndexByte**: Used to find the last colon, matching the existing correct implementation in `twitch-listener/cmd/main.go` lines 264-268. This handles UUIDs that might contain colons themselves (though UUIDs do not).

## Deviations from Plan

### Out-of-Scope Issues Discovered

**[Pre-existing] TestRepository_GetActiveChannelsHandlesStringChatroomIDs failure in kick-listener**
- **Found during:** Task 2 verification
- **Issue:** `expected 2 channels, got 0` in `services/kick-listener/channels/repository_test.go:29`
- **Action:** Verified pre-existing via `git stash` (failure existed before any 33-01 changes). Logged to `deferred-items.md`. Not fixed — out of scope per deviation rules.

Otherwise: plan executed exactly as written.

## Verification Results

- `services/kick-listener/channels/manager_test.go` exists — PASS
- `TestManager_SourceIDNormalization` passes — PASS
- `go build ./...` in kick-listener — PASS
- `go build ./...` in twitch-listener — PASS
- All twitch-listener tests pass — PASS
- kick-listener: `TestManager_SourceIDNormalization` passes (pre-existing `TestRepository_GetActiveChannelsHandlesStringChatroomIDs` failure is out of scope, logged to deferred-items.md)

## Self-Check: PASSED
