---
phase: 10-message-pipeline-resilience-fix-silent-failure-modes-across-
plan: "01"
subsystem: twitch-listener
tags: [resilience, atomics, backoff, zombie-detection, health-probes]
dependency_graph:
  requires: []
  provides: [shared/listener/backoff.go, services/twitch-listener/zombie/detector.go, atomic-health-probes]
  affects: [twitch-listener, shared/listener]
tech_stack:
  added: [sync/atomic fields, zombie package, JitteredBackoff utility]
  patterns: [atomic probe reads, received-vs-published drift, full jitter backoff, retry with backoff]
key_files:
  created:
    - shared/listener/backoff.go
    - shared/listener/backoff_test.go
    - services/twitch-listener/zombie/detector.go
    - services/twitch-listener/zombie/detector_test.go
    - services/twitch-listener/status/publisher_test.go
    - docs/adr/0011-zombie-listener-detection.md
  modified:
    - services/twitch-listener/channels/manager.go
    - services/twitch-listener/channels/manager_test.go
    - shared/listener/ring_buffer.go
    - shared/listener/ring_buffer_test.go
    - services/twitch-listener/status/publisher.go
    - services/twitch-listener/irc/connection.go
    - services/twitch-listener/handlers/health.go
    - services/twitch-listener/cmd/main.go
    - docs/adr/README.md
decisions:
  - "Atomic fields shadow mutex-protected values in Manager struct — health probes read atomics, mutation paths write both"
  - "verifyCoverageComplete removes m.mu.RLock — Redis SCAN needs no Manager lock; fallback to m.assignedSourceIDs uses brief read lock only"
  - "JitteredBackoff uses full jitter algorithm (random_between(0, min(cap, base*2^attempt))), base=1s, cap=30s"
  - "Status publisher retries 3 times with JitteredBackoff; context cancellation checked before each sleep"
  - "Zombie detector uses stallWindow/2 snapshot rotation — evaluate delta then rotate, not rotate then evaluate"
  - "RecordPublished called after ring buffer accept (publisher.Publish returns nil), not after XADD — ring buffer absorbs blips"
  - "ZombieDetector wired via interface (zombieTracker) in irc/connection.go for testability without package coupling"
  - "SetZombieDetector setter pattern used for both health.Handler and ConnectionManager — avoids constructor signature changes"
metrics:
  duration: "~30 minutes"
  completed_date: "2026-04-08"
  tasks: 3
  files_modified: 9
  files_created: 6
---

# Phase 10 Plan 01: Twitch Listener Resilience Hardening Summary

Hardened twitch-listener probe isolation, added shared backoff utility, fixed ring buffer drop logging, added status publisher retry, and implemented zombie listener detection.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1a | Atomic flag migration for health probe methods (F-01) | e220ccc | channels/manager.go, channels/manager_test.go |
| 1b | Ring buffer structured drop logging + shared backoff + status publisher retry (F-02/F-03) | 49d8828 | ring_buffer.go, backoff.go, status/publisher.go |
| 2 | Zombie listener detection package + liveness probe wiring (Z-01) | 626ef9a | zombie/detector.go, irc/connection.go, handlers/health.go, cmd/main.go |
| 3 | ADR-0011 for zombie listener detection | e91e952 | docs/adr/0011-zombie-listener-detection.md |

## What Was Built

**Atomic health probe reads (F-01):** Three fields added to `channels.Manager` — `initialSyncDoneAtomic atomic.Bool`, `activeChannelCountAtomic atomic.Int64`, `filteredAssignmentCountAtomic atomic.Int64`. The probe accessor methods (`IsInitialSyncComplete`, `GetActiveChannelCount`, `GetFilteredAssignmentCount`) now do lock-free atomic reads. Atomic stores are placed at every mutation site (joinChannel, partChannel, ClearActiveChannels, handleLeadershipLoss, joinChannelsMultipleConnectionsUnlocked, SyncChannels). The `verifyCoverageComplete` method's global `m.mu.RLock()` is removed — only a brief read lock is taken in the Redis scan error fallback.

**Ring buffer overflow log sentinel (F-02):** The overflow log in `enqueue` is upgraded from `Warn` to `Error` with a stable sentinel message `"ring_buffer_overflow_drop"` and structured fields `service`, `capacity`, `current_depth`. This enables Grafana Loki alert rules to match on a stable string.

**Shared JitteredBackoff utility:** New `shared/listener/backoff.go` exports `JitteredBackoff(attempt int) time.Duration` using full jitter algorithm (random between 0 and min(cap, base×2^attempt)), with base=1s and cap=30s. Used by status publisher retry and available to all future plans in this phase.

**Status publisher retry (F-03):** `status.Publisher.Publish` now retries up to 3 times (`maxPublishAttempts=3`) on Redis failure with `JitteredBackoff`. Context cancellation is honoured between retries. After all attempts are exhausted, logs `status_publish_exhausted` at Error level.

**Zombie detector (Z-01):** New `services/twitch-listener/zombie/` package with `Detector` struct using two `atomic.Int64` counters. `RecordReceived()` is called on each IRC PRIVMSG; `RecordPublished()` is called after each successful ring buffer accept. `IsZombie()` evaluates drift every `stallWindow/2` (default 2.5 minutes) and returns true when received advances but published stalls. Both-zero check (D-10) prevents false positives on offline channels. Wired into `irc.ConnectionManager` via `SetZombieDetector` setter; liveness probe returns HTTP 503 when zombie detected.

**ADR-0011:** Documents the zombie detector design rationale — 5-minute stall window choice, false positive avoidance, snapshot rotation, and rejected alternatives (heartbeat, external monitoring, readiness probe, XADD counter).

## Deviations from Plan

**1. [Rule 1 - Bug] Zombie detector snapshot rotation logic needed inversion**
- **Found during:** Task 2 — first test run after implementing detector.go
- **Issue:** The plan's provided code snippet rotated the snapshot and returned `false` immediately, preventing `IsZombie()` from ever returning `true`. Tests `TestDetector_StallDetected_IsZombie` and `TestDetector_NotEnoughTimeElapsed_NotZombie` failed.
- **Fix:** Inverted the logic: if `elapsed < stallWindow/2`, return false (not enough data). If `elapsed >= stallWindow/2`, evaluate delta then rotate. This matches the spec intent (evaluate, then reset the window).
- **Files modified:** `services/twitch-listener/zombie/detector.go`
- **Commit:** 626ef9a

**2. [Rule 2 - Missing critical functionality] Status publisher test used internal helper approach**
- **Found during:** Task 1b — the test needed to simulate Redis failures without miniredis (not available in twitch-listener go.mod)
- **Fix:** Extracted `marshalMessage` and `maxPublishAttempts` as package-level symbols; wrote tests against a `testPublisher` struct with an injectable `publisherFunc`. Tests validate the retry loop logic without a real Redis client.
- **Files modified:** `services/twitch-listener/status/publisher.go`, `services/twitch-listener/status/publisher_test.go`
- **Commit:** 49d8828

**3. [Rule 2 - Missing critical functionality] ConnectionManager needed interface for zombieTracker**
- **Found during:** Task 2 — wiring zombie detector into irc/connection.go
- **Fix:** Defined `zombieTracker` interface in connection.go so the IRC connection manager doesn't need to import `zombie.Detector` directly in tests. `SetZombieDetector` accepts the concrete `*zombie.Detector` type (only called from main.go).
- **Files modified:** `services/twitch-listener/irc/connection.go`
- **Commit:** 626ef9a

## Known Stubs

None — all functionality is fully wired in production paths.

## Threat Flags

None — all surfaces are internal to the Kubernetes pod (health probe HTTP handlers behind ClusterIP). The zombie probe status message is not exposed to end users (T-10-02: accepted per threat model).

## Self-Check: PASSED

Files verified:
- `shared/listener/backoff.go` — FOUND
- `shared/listener/backoff_test.go` — FOUND
- `services/twitch-listener/zombie/detector.go` — FOUND
- `services/twitch-listener/zombie/detector_test.go` — FOUND
- `docs/adr/0011-zombie-listener-detection.md` — FOUND

Commits verified:
- `e220ccc` — feat(10-01): atomic flag migration (Task 1a)
- `49d8828` — feat(10-01): ring buffer + backoff + status publisher (Task 1b)
- `626ef9a` — feat(10-01): zombie detection (Task 2)
- `e91e952` — docs(10-01): ADR-0011 (Task 3)
