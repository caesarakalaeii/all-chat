---
phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients
plan: 04
subsystem: infra
tags: [redis, pubsub, demand-driven, go, listener-sdk]

# Dependency graph
requires:
  - phase: 05-01
    provides: source:demand Redis Pub/Sub channel from source-manager
  - phase: 05-02
    provides: SDK demand subscriber loop in ListenerBase.Start (for kick and twitch-eventsub)
provides:
  - kick-listener demand filtering via Platform="kick" in SDK config
  - twitch-eventsub-listener demand filtering via Platform="twitch-eventsub" in SDK config
  - youtube-listener-innertube source:demand Pub/Sub subscription goroutine
  - discord-listener source:demand Pub/Sub subscription goroutine
  - youtube-listener source:demand Pub/Sub subscription goroutine
affects:
  - e2e verification of full demand signal flow

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Direct Redis Pub/Sub subscription goroutine in main.go for leadership-only listeners that don't call base.Start

key-files:
  created: []
  modified:
    - services/youtube-listener-innertube/cmd/main.go
    - services/discord-listener/cmd/main.go
    - services/youtube-listener/cmd/main.go

key-decisions:
  - "Leadership-only listeners (innertube, discord, youtube) add direct source:demand Pub/Sub goroutine in main.go — pragmatic minimum since they don't call base.Start and therefore don't get the SDK demand loop automatically"
  - "kick-listener and twitch-eventsub-listener already had Platform set from prior migration phases (35/36/38) — no code changes required for Task 1"
  - "Demand update logs (Demand update received) are the minimum observable behavior; connect/disconnect gating into stream managers deferred to follow-up iteration"

patterns-established:
  - "Pattern: Leadership-only listeners subscribe to source:demand via goroutine in main.go, filter by platformFilter const, log platform_sources count"

requirements-completed:
  - DEMAND-08
  - DEMAND-09

# Metrics
duration: 5min
completed: 2026-03-27
---

# Phase 05 Plan 04: Wire Demand Signals into All Remaining Go Listeners Summary

**source:demand Pub/Sub subscription added to youtube-listener-innertube, discord-listener, and youtube-listener; kick and twitch-eventsub already had Platform set from prior phases**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-27T11:24:00Z
- **Completed:** 2026-03-27T11:26:47Z
- **Tasks:** 2 of 3 (Task 3 is checkpoint:human-verify — stopping for user E2E verification)
- **Files modified:** 3

## Accomplishments
- Verified kick-listener has `cfg.Platform = "kick"` (already set in Phase 36)
- Verified twitch-eventsub-listener has `cfg.Platform = "twitch-eventsub"` (already set in Phase 38)
- Added direct Redis Pub/Sub goroutine to youtube-listener-innertube, discord-listener, and youtube-listener
- All services compile via `make build-all`

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire SDK demand config for kick and twitch-eventsub listeners** - no commit needed (already done in prior phases)
2. **Task 2: Add demand subscription to leadership-only listeners** - `972b5fb` (feat)
3. **Task 3: End-to-end demand signal verification** - checkpoint:human-verify (awaiting user)

## Files Created/Modified
- `services/youtube-listener-innertube/cmd/main.go` - Added encoding/json import and source:demand Pub/Sub goroutine (platform=youtube)
- `services/discord-listener/cmd/main.go` - Added source:demand Pub/Sub goroutine (platform=discord)
- `services/youtube-listener/cmd/main.go` - Added encoding/json import and source:demand Pub/Sub goroutine (platform=youtube)

## Decisions Made
- Leadership-only listeners (innertube, discord, youtube) add direct source:demand Pub/Sub goroutine in main.go — pragmatic minimum since they don't call base.Start and therefore don't get the SDK demand loop automatically
- kick-listener and twitch-eventsub-listener already had Platform set from prior migration phases (35/36/38) — no code changes required for Task 1
- Demand update logs are the minimum observable behavior; connect/disconnect gating into stream managers deferred to follow-up iteration

## Deviations from Plan

None - plan executed exactly as written. Task 1 required zero code changes because prior phases (36, 38) already set Platform on those services.

## Issues Encountered
None - all services compiled on first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
Tasks 1 and 2 complete. Task 3 (checkpoint:human-verify) requires user to:
1. Start full environment: `make docker-up`
2. Check source-manager logs for demand subscriber initialization
3. Open an overlay in browser to trigger demand signal
4. Check tiktok-listener and other listener logs for demand update received
5. Close overlay and confirm grace period deactivation after 60s

---
*Phase: 05-tiktok-listener-demand-driven-polling*
*Completed: 2026-03-27*
