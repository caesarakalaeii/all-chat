---
phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients
plan: 05
subsystem: infra
tags: [demand-gating, redis-pubsub, go, youtube-listener, discord-listener, gap-closure]

requires:
  - phase: 05-04
    provides: "Direct source:demand Pub/Sub subscription goroutines in leadership-only listeners (innertube, discord, youtube)"

provides:
  - "youtube-listener-innertube: UpdateDemandedChannels stops pollers and cancels discovery for non-demanded channels; syncSources skips non-demanded channels"
  - "discord-listener: DemandChecker interface; HandleMessageCreate drops messages for channels without demand; demandSet wired via SetDemandChecker"
  - "youtube-listener: UpdateDemandedChannels with zero-demand fast-sync; isChannelDemanded gate in syncStreams"

affects: [phase-05-verification, DEMAND-08]

tech-stack:
  added: []
  patterns:
    - "UpdateDemandedChannels method pattern — all three leadership-only listeners now expose this method, matching kick-listener's UpdateDemandedSourceIDs reference implementation"
    - "DemandChecker interface — thin gateway.DemandChecker interface enables demand-aware message filtering without coupling GatewayClient to cmd package types"
    - "demandSet — thread-safe concurrent set used as both DemandChecker and mutable state from demand goroutine"

key-files:
  created: []
  modified:
    - services/youtube-listener-innertube/streams/manager.go
    - services/youtube-listener-innertube/cmd/main.go
    - services/discord-listener/gateway/client.go
    - services/discord-listener/cmd/main.go
    - services/youtube-listener/streams/manager.go
    - services/youtube-listener/cmd/main.go

key-decisions:
  - "youtube-listener-innertube reconcileDemand acquires m.mu.Lock before stopping pollers and cancelling discovery — consistent with existing poller lifecycle management"
  - "youtube-listener demand filter applied to channelSources AFTER inactive validation (not before), so inactive sources with valid tokens that have demand are still processed"
  - "youtube-listener cleanupInactivePollers naturally stops pollers for demand-removed channels since demand filter removes them from channelSources before the cleanup call"
  - "discord demandSet uses nil-as-no-filter sentinel — channels nil means backward-compat pass-through; empty map means zero demand (drop all messages)"
  - "DemandChecker interface placed in gateway package (not cmd) to avoid circular import"
  - "SetDemandChecker setter used instead of changing NewGatewayClient signature — preserves all existing call sites and tests"

patterns-established:
  - "Leadership-only listener demand wiring pattern: demand goroutine in cmd/main.go calls manager.UpdateDemandedChannels(demanded); manager stores and gates all polling/publishing"

requirements-completed: [DEMAND-08]

duration: 4min
completed: 2026-03-27
---

# Phase 05 Plan 05: Gap Closure — Demand Gating for All Leadership-Only Listeners Summary

**Demand gating wired into youtube-listener-innertube (poller/discovery stops), discord-listener (message drop), and youtube-listener (sync skip) — all three leadership-only listeners now gate on source:demand signals**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-27T10:42:36Z
- **Completed:** 2026-03-27T10:46:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- youtube-listener-innertube: UpdateDemandedChannels/reconcileDemand stops pollers and cancels discovery goroutines for channels that lose demand; syncSources filters channelOverlays by demand before starting discovery
- discord-listener: DemandChecker interface + demandSet type + SetDemandChecker; HandleMessageCreate drops messages for non-demanded Discord channels silently
- youtube-listener: UpdateDemandedChannels with zero-demand fast-sync trigger; isChannelDemanded gate in syncStreams filters channelSources before the main polling loop

## Task Commits

1. **Task 1: Wire demand gating into youtube-listener-innertube and discord-listener** - `25fa7de` (feat)
2. **Task 2: Wire demand gating into youtube-listener stream manager** - `567af9c` (feat)

## Files Created/Modified
- `services/youtube-listener-innertube/streams/manager.go` - Added demandedChannels field, UpdateDemandedChannels, reconcileDemand, demand filter in syncSources
- `services/youtube-listener-innertube/cmd/main.go` - Call streamManager.UpdateDemandedChannels in demand goroutine
- `services/discord-listener/gateway/client.go` - DemandChecker interface, demandChecker field, SetDemandChecker, demand check in HandleMessageCreate
- `services/discord-listener/cmd/main.go` - demandSet type, ds creation, SetDemandChecker call, ds.UpdateDemandedChannels in demand goroutine
- `services/youtube-listener/streams/manager.go` - Added demandedChannels field, UpdateDemandedChannels, isChannelDemanded, demand filter in syncStreams
- `services/youtube-listener/cmd/main.go` - Call streamManager.UpdateDemandedChannels in demand goroutine

## Decisions Made
- youtube-listener demand filter placed after inactive source validation so sources with valid OAuth tokens that have demand still get processed for livestream detection
- DemandChecker interface defined in gateway package (not cmd) to avoid circular import; setter pattern (SetDemandChecker) preserves all existing NewGatewayClient call sites
- youtube-listener cleanupInactivePollers naturally handles demand-removed channels since they are deleted from channelSources before the cleanup call — no extra cleanup code needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## Next Phase Readiness
- DEMAND-08 gap is now closed — all non-Twitch listeners (kick, tiktok, innertube, discord, youtube) only connect/poll/publish when overlays are open
- Phase 05 verification plan can now confirm end-to-end demand gating behavior across all listener services

---
*Phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients*
*Completed: 2026-03-27*
