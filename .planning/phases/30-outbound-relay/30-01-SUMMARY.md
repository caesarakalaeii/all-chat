---
phase: 30-outbound-relay
plan: "01"
subsystem: discord-listener/relay
tags: [relay, discord, pubsub, tdd, loop-safety]
dependency_graph:
  requires: []
  provides: [relay.Manager, relay.DiscordPoster, relay.Repository]
  affects: [services/discord-listener/relay]
tech_stack:
  added: [github.com/jackc/pgx/v5]
  patterns: [interface-injection, tdd-red-green, repository-pattern, pub-sub-drain]
key_files:
  created:
    - services/discord-listener/relay/poster.go
    - services/discord-listener/relay/repository.go
    - services/discord-listener/relay/manager.go
    - services/discord-listener/relay/poster_test.go
    - services/discord-listener/relay/manager_test.go
  modified:
    - services/discord-listener/go.mod
    - services/discord-listener/go.sum
decisions:
  - "httpPoster.baseURL field overridable for httptest.Server injection — no HTTP client interface extraction needed"
  - "HandleMessage exported on Manager for synchronous unit test injection, avoiding Redis Pub/Sub mocking"
  - "pgxpool added via go get before RED phase so tests compile from the start"
  - "doPost(isRetry bool) private helper enforces single-retry contract without exposing retry logic in interface"
metrics:
  duration_seconds: 129
  completed_date: "2026-03-16"
  tasks_completed: 2
  files_created: 5
  files_modified: 2
---

# Phase 30 Plan 01: relay package — loop-safe Discord outbound relay Summary

**One-liner:** TDD-built relay package with loop-safety filter, 429 Retry-After single retry, and dynamic Redis Pub/Sub subscribe/unsubscribe via SyncRelayConfigs.

## Tasks Completed

| Task | Type | Commit | Description |
|------|------|--------|-------------|
| RED | test | f1f8ffb | Failing tests for poster, manager (loop-safety + relay format) |
| GREEN | feat | 80627e6 | poster.go, repository.go, manager.go — all 7 tests passing |

## Test Coverage

| Test | Result |
|------|--------|
| TestFormatRelayContent | PASS — "🟣 alice: hello" |
| TestFormatRelayContent_UnknownPlatform | PASS — "💬 alice: hello" |
| TestRelayManager_DiscordPlatformFiltered | PASS — 0 poster.Post calls |
| TestRelayManager_NonDiscordRelayed | PASS — 1 call with correct channelID + format |
| TestHTTPPoster_UsesCorrectChannelID | PASS — POST path contains relay_channel_id |
| TestHTTPPoster_429RetriesOnce | PASS — exactly 2 HTTP calls for 429+201 sequence |
| TestHTTPPoster_403Drops | PASS — 403 returns nil (silent drop) |

## Verification

```
go test ./relay/... exits 0 — all 7 relay tests pass
go test ./...     exits 0 — no regressions in gateway/ or publisher/
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Missing pgxpool go.sum entry**
- **Found during:** GREEN phase compilation
- **Issue:** `go get github.com/jackc/pgx/v5` added pgx but not pgxpool transitive dep `github.com/jackc/puddle/v2`
- **Fix:** Ran `go get github.com/jackc/pgx/v5/pgxpool@v5.8.0` to populate go.sum
- **Files modified:** services/discord-listener/go.sum
- **Commit:** 80627e6

## Key Decisions

1. **httpPoster.baseURL overridable** — allows `httptest.Server` injection in `TestHTTPPoster_UsesCorrectChannelID` without extracting an HTTP client interface; keeps implementation simpler.

2. **HandleMessage exported** — `Manager.HandleMessage(ctx, platform, username, text, overlayID, relayChannelID)` provides synchronous test injection that avoids mocking Redis Pub/Sub channels entirely.

3. **doPost(isRetry bool)** — private helper enforces the single-retry invariant at compile time; the public `Post` method always passes `isRetry=false` and recursive call passes `true`.

## Self-Check

- [x] services/discord-listener/relay/poster.go exists
- [x] services/discord-listener/relay/repository.go exists
- [x] services/discord-listener/relay/manager.go exists
- [x] services/discord-listener/relay/poster_test.go exists
- [x] services/discord-listener/relay/manager_test.go exists
- [x] Commits f1f8ffb and 80627e6 exist in git log

## Self-Check: PASSED
