# Phase 5: Demand-Driven Polling — Only Poll When Overlay Has Connected Clients - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Make all listeners (except Twitch) demand-driven: only connect to streams when at least one streamer has an overlay open in OBS (or browser) with a WebSocket connection to the API Gateway. When no overlay is open for a given source, the listener does zero work for that source. TikTok listener is the primary target but the demand signal infrastructure and Go SDK integration cover all non-Twitch listeners.

**Twitch excluded:** IRC connection/disconnection is rate-limited by Twitch. Twitch listener keeps its current always-connected behavior.

</domain>

<decisions>
## Implementation Decisions

### Demand signal flow
- API Gateway detects overlay WebSocket connect/disconnect → signals source-manager
- Source-manager receives demand signal, evaluates which sources have demand, and pushes demand updates to listeners via existing coordination channels
- Source-manager is the sole authority for "which sources have demand" — listeners do not query `overlay:connected:*` keys directly
- Demand signal includes both source UUID and channel_id (e.g., TikTok username) so listeners can connect immediately without lookups
- Demand signal layers ON TOP of existing coordinator sharding — coordinator still assigns sources to pods, demand signal adds "of your assigned sources, connect to these ones that have viewers"

### Reaction speed
- Primary path: Pub/Sub signal from source-manager → listener reacts in milliseconds
- Fallback: Listeners keep a polling safety net (longer interval, e.g., 60s) to catch missed Pub/Sub events
- Source-manager receives overlay connection events from API Gateway via Redis Pub/Sub (not polling)

### Idle behavior
- Full shutdown when zero demand: no live detection, no polling, no connections — service sits idle
- Pod stays running (HPA min replicas = 1) — no scale-to-zero, avoids cold-start latency
- Best-effort startup latency when demand returns (seconds acceptable) — no pre-warming

### Source discovery
- Source-manager provides the full list of sources with demand (not just "overlay X connected")
- Listeners no longer query the database for source discovery — source-manager is the source of truth
- Existing coordinator assignment flow (consistent hashing, pod ownership) is unchanged — demand is an additional filter layer

### Graceful transitions
- Rely on API Gateway's existing 60s grace period — listener does NOT add its own grace period
- When `overlay:connected:{id}` TTL expires (no heartbeat refresh), source-manager sends "no demand" signal
- When a TikTok stream ends while viewers are connected: disconnect and report "stream offline" — if streamer restarts, a new demand cycle handles reconnection (no persistent polling for stream resume)

### Listener scope
- All listeners except Twitch get demand-driven behavior
- TikTok listener (Node.js): direct implementation
- Go listeners (Kick, YouTube, YouTube InnerTube, Discord, Twitch EventSub): demand-driven behavior added to shared Go listener SDK
- Implementation must not duplicate logic — SDK handles demand-driven behavior, individual listeners do not re-implement it
- Twitch listener excluded: IRC JOIN/PART is rate-limited, always-connected model stays

### Claude's Discretion
- Exact Redis Pub/Sub channel naming for demand signals
- Source-manager internal implementation of demand tracking
- SDK interface design for demand-driven callbacks
- Polling fallback interval (suggested ~60s but flexible)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Demand signal infrastructure
- `services/api-gateway/websocket/manager.go` — Existing overlay connection tracking, TTL keys, heartbeat, `publishConnectionEvent()`
- `services/api-gateway/websocket/pool.go` — Per-overlay connection pool, `Size()` for connection count
- `services/source-manager/coordination/registry.go` — Assignment registry, Redis-backed O(1) lookups

### TikTok listener (primary target)
- `services/tiktok-listener/src/index.ts` — Main service, `pollActiveStreams()` with existing demand-checking logic to be replaced
- `services/tiktok-listener/src/livestream/poller.ts` — Live detection poller (to be stopped when idle)
- `services/tiktok-listener/src/coordination/client.ts` — Coordinator client, assignment queries
- `services/tiktok-listener/src/coordination/subscriber.ts` — Migration event subscriber

### Go shared listener SDK
- `shared/listener/base.go` — ListenerBase struct, Start/Stop lifecycle
- `shared/listener/leadership.go` — LeadershipListener variant
- `shared/listener/channel_manager.go` — ChannelManager interface (7 methods)

### Architecture
- `docs/architecture/01-DATA-FLOW.md` — Message flow, Redis Streams/Pub/Sub patterns
- `docs/architecture/00-OVERVIEW.md` — Service map, system design

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `overlay:connected:{overlay_id}` Redis TTL keys — already maintained by API Gateway (10min TTL, 2min heartbeat refresh)
- `publishConnectionEvent()` in api-gateway/websocket/manager.go — already fires on first connect/last disconnect
- `coordination/subscriber.ts` in TikTok listener — existing Redis Pub/Sub subscription pattern for migration events
- Shared listener SDK (`shared/listener/`) — ListenerBase and LeadershipListener archetypes ready for extension

### Established Patterns
- Source-manager as coordination hub — leader election, assignment registry, migration events
- Redis Pub/Sub for real-time inter-service communication (migration events, overlay messages)
- Coordinator consistent hashing for pod-level sharding — demand signal layers on top
- API Gateway 60s grace period for brief overlay reconnects

### Integration Points
- API Gateway → source-manager: new demand signal (overlay connected/disconnected with source list)
- Source-manager → listeners: new demand signal (sources with active demand, including channel_id)
- Go SDK: new demand-driven lifecycle hooks in ListenerBase/LeadershipListener
- TikTok listener: replace `pollActiveStreams()` DB queries with source-manager demand signals

</code_context>

<specifics>
## Specific Ideas

- "Viewer" means the streamer's OBS browser source or browser tab with the overlay open — NOT stream chat viewers
- Twitch excluded because IRC JOIN/PART rate limits would cause connection issues with demand-driven connect/disconnect
- "Ensure the listener doesn't implement this behaviour twice" — SDK owns the demand-driven logic, individual Go listeners inherit it

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients*
*Context gathered: 2026-03-27*
