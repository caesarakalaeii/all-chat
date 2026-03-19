---
phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener
plan: 01
subsystem: infra
tags: [go, listener-sdk, youtube, goroutine-leak, goleak, sourcemanager]

# Dependency graph
requires:
  - phase: 37-migrate-youtube-innertube-and-discord-listener
    provides: LeadershipListener SDK pattern and goleak smoke test pattern

provides:
  - youtube-listener cmd/main.go wired to LeadershipListener SDK via NewLeadershipListenerFromEnv
  - goroutine leak smoke test in cmd/main_sdk_test.go
  - goleak pinned as direct dep in go.mod

affects:
  - 38-02 (twitch-eventsub-listener migration — same SDK, same pattern)

# Tech tracking
tech-stack:
  added: [go.uber.org/goleak@v1.3.0 (direct dep)]
  patterns: [NewLeadershipListenerFromEnv wiring for leadership-only services, listener.Env as drop-in for getEnvOrDefault]

key-files:
  created:
    - services/youtube-listener/cmd/main_sdk_test.go
  modified:
    - services/youtube-listener/cmd/main.go
    - services/youtube-listener/go.mod

key-decisions:
  - "listener.Env used as drop-in replacement for getEnvOrDefault — local helper deleted entirely (matches Phase 35/37 pattern)"
  - "nil passed to NewListenerBase for coordinator client — LeadershipListener is the sole SDK integration point"
  - "base.Start not called — youtube-listener is leadership-only (established Phase 37 pattern)"
  - "parseIntEnv preserved — wraps listener.Env + strconv.Atoi, called 4 times for quota tier config"
  - "nil passed for logger in smoke test NewListenerBase call — established pattern from kick-listener and youtube-innertube"

patterns-established:
  - "Leadership-only service pattern: NewListenerBase(cfg, nil, redisClient, podName, log) + NewLeadershipListenerFromEnv; no Start/Stop calls"
  - "Smoke test pattern: mockChannelManagerForTest inline struct, goleak.VerifyNone(t), nil for logger and redisClient"

requirements-completed: [MIGRATE-03]

# Metrics
duration: 10min
completed: 2026-03-18
---

# Phase 38 Plan 01: Migrate youtube-listener to LeadershipListener SDK Summary

**youtube-listener cmd/main.go migrated to listener.NewLeadershipListenerFromEnv, replacing 12-line manual sourcemanager construction block; goleak smoke test added**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-18T08:37:00Z
- **Completed:** 2026-03-18T08:47:15Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Replaced manual sourcemanager construction (NewSigningTokenSource + NewClient + NewLeadershipCoordinator) with single `listener.NewLeadershipListenerFromEnv(base, "youtube", log)` call
- Replaced all 12 `getEnvOrDefault(...)` call sites with `listener.Env(...)` and deleted the local helper function
- Added `cmd/main_sdk_test.go` with `TestListenerBase_StartStop_NoGoroutineLeak` using goleak, matching established Phase 37 pattern
- `make build-all` cross-module compile gate passes for all listener modules

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin goleak as direct dep and write goroutine leak smoke test** - `e4d403c` (test)
2. **Task 2: Migrate cmd/main.go to LeadershipListener SDK** - `698671c` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `services/youtube-listener/cmd/main_sdk_test.go` - Goroutine leak smoke test with mockChannelManagerForTest, goleak.VerifyNone
- `services/youtube-listener/cmd/main.go` - SDK wiring: NewLeadershipListenerFromEnv, listener.Env throughout, removed sourcemanager import and getEnvOrDefault
- `services/youtube-listener/go.mod` - go.uber.org/goleak@v1.3.0 pinned as direct dep

## Decisions Made
- `listener.Env` used as direct drop-in for `getEnvOrDefault` — identical semantics (returns env value or default), so the local helper was deleted entirely (Phase 35 pattern)
- `parseIntEnv` preserved: it wraps `os.Getenv` + `strconv.Atoi` (not a simple env lookup), called 4 times for YOUTUBE_GLOBAL_DAILY_QUOTA, YOUTUBE_HIGH_TIER_QUOTA, YOUTUBE_STANDARD_TIER_QUOTA, YOUTUBE_LOW_TIER_QUOTA
- `base.Start` not called — youtube-listener is leadership-only; `ListenerBase` is used only as the container for `NewLeadershipListenerFromEnv`
- `nil` passed for coordinator client in `NewListenerBase` — leadership wiring is fully managed by `NewLeadershipListenerFromEnv`
- Daily quota reset goroutine left unchanged — it is leadership-independent (all pods reset, idempotent via PostgreSQL)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- youtube-listener migration complete (MIGRATE-03 closed)
- Phase 38 Plan 02 ready: twitch-eventsub-listener migration to `ListenerBase` SDK (assignment-based archetype, different from leadership-only pattern used here)

---
*Phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener*
*Completed: 2026-03-18*
