# Phase 5: Demand-Driven Polling — Research

**Researched:** 2026-03-27
**Domain:** Redis Pub/Sub demand signaling, Go SDK extension, TypeScript listener refactor
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Demand signal flow
- API Gateway detects overlay WebSocket connect/disconnect → signals source-manager
- Source-manager receives demand signal, evaluates which sources have demand, and pushes demand updates to listeners via existing coordination channels
- Source-manager is the sole authority for "which sources have demand" — listeners do not query `overlay:connected:*` keys directly
- Demand signal includes both source UUID and channel_id (e.g., TikTok username) so listeners can connect immediately without lookups
- Demand signal layers ON TOP of existing coordinator sharding — coordinator still assigns sources to pods, demand signal adds "of your assigned sources, connect to these ones that have viewers"

#### Reaction speed
- Primary path: Pub/Sub signal from source-manager → listener reacts in milliseconds
- Fallback: Listeners keep a polling safety net (longer interval, e.g., 60s) to catch missed Pub/Sub events
- Source-manager receives overlay connection events from API Gateway via Redis Pub/Sub (not polling)

#### Idle behavior
- Full shutdown when zero demand: no live detection, no polling, no connections — service sits idle
- Pod stays running (HPA min replicas = 1) — no scale-to-zero, avoids cold-start latency
- Best-effort startup latency when demand returns (seconds acceptable) — no pre-warming

#### Source discovery
- Source-manager provides the full list of sources with demand (not just "overlay X connected")
- Listeners no longer query the database for source discovery — source-manager is the source of truth
- Existing coordinator assignment flow (consistent hashing, pod ownership) is unchanged — demand is an additional filter layer

#### Graceful transitions
- Rely on API Gateway's existing 60s grace period — listener does NOT add its own grace period
- When `overlay:connected:{id}` TTL expires (no heartbeat refresh), source-manager sends "no demand" signal
- When a TikTok stream ends while viewers are connected: disconnect and report "stream offline" — if streamer restarts, a new demand cycle handles reconnection (no persistent polling for stream resume)

#### Listener scope
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

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

## Summary

This phase adds a demand signal layer to the all-chat listener infrastructure. Rather than always polling/connecting regardless of whether any overlay has viewers, listeners will only activate when the source-manager confirms there is active demand (at least one overlay WebSocket client connected for a given source). The API Gateway already publishes `overlay:connections` Pub/Sub events and maintains `overlay:connected:{overlay_id}` TTL keys — the plumbing exists, it just needs to be wired into source-manager → listener signaling.

The implementation has three independent workstreams: (1) source-manager subscribes to `overlay:connections`, resolves demand per source, and publishes demand signals to a new Redis channel; (2) the shared Go listener SDK gains a demand subscriber loop and a new `DemandListener` interface or hook so Go listeners react to demand changes without touching their own source discovery logic; (3) the TikTok Node.js listener replaces its current `pollActiveStreams()` DB query loop with a Redis Pub/Sub subscriber for demand signals from the source-manager, keeping a 60s safety-net poll as fallback.

**Primary recommendation:** Model the demand signal as a Redis Pub/Sub channel `source:demand` where source-manager publishes full snapshots of `{source_id, channel_id, platform}` tuples that currently have demand. Listeners subscribe, reconcile their active connections against the snapshot, and act. Fallback polling hits source-manager's existing `/assignments` endpoint filtered to "sources with demand" using a new query parameter.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9 (already in use) | Pub/Sub subscribe, key scan | Already used in all Go services |
| `redis` (npm) | ^4.x (already in use) | Pub/Sub subscribe in TikTok listener | Already used; node-redis v4 requires duplicate connection for Pub/Sub |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.uber.org/zap` | v1 (in use) | Structured logging in SDK extension | All Go services already use it |

**No new dependencies required.** All necessary libraries are already in use.

---

## Architecture Patterns

### What Exists Today (State Before This Phase)

**API Gateway** (`services/api-gateway/websocket/manager.go`):
- `AddConnection()` publishes `"connected"` to `overlay:connections` channel on FIRST connection to a pool
- `startDisconnectGracePeriod()` waits 60s, then publishes `"disconnected"` on LAST connection leaving
- `refreshConnectionTTLs()` runs every 2 min, refreshing `overlay:connected:{id}` with 10min TTL
- `OverlayConnectionEvent` struct: `{Type, OverlayID, Timestamp}` — does NOT include source list

**TikTok Listener** (`services/tiktok-listener/src/index.ts`):
- `pollActiveStreams()` runs every 30s (POLL_INTERVAL_MS): scans `overlay:connected:*` keys directly, then queries DB for TikTok sources belonging to those overlays, then connects/disconnects accordingly
- This is the function to replace — currently doing direct Redis key scan + DB query
- `LiveStreamPoller` (`livestream/poller.ts`): a separate layer that polls TikTok's unofficial API to check if a username is live; has `start()`/`stop()`, `addTarget()`/`removeTarget()` — this also needs to be idle when no demand

**Source-Manager** (`services/source-manager/`):
- Has no awareness of overlay connection state — it coordinates pod assignments, not demand
- `registry.Repository.GetActiveSources()` queries DB for sources by platform — does NOT know which overlays have viewers
- The coordinator reconciles on `chat_source_changes` pg_notify, not on overlay connection events

**Go Listener SDK** (`shared/listener/`):
- `ListenerBase`: 3 background loops (heartbeat, assignment refresh, migration subscriber)
- `ChannelManager` interface: 7 methods — `Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`, `GetActiveChannels`, `GetActiveChannelCount`
- No demand awareness — currently starts all assigned channels unconditionally

### New Components Needed

#### 1. Demand Signal Format (new Redis channel: `source:demand`)

Source-manager publishes a demand update whenever overlay connection state changes. Format:

```typescript
// Published to Redis Pub/Sub channel "source:demand"
interface DemandUpdate {
  type: "demand_update";
  sources: Array<{
    source_id: string;      // UUID from overlay_chat_sources
    channel_id: string;     // Platform username (e.g., TikTok username)
    platform: string;       // "tiktok", "kick", "youtube", etc.
    overlay_id: string;     // Which overlay needs this source
  }>;
  timestamp: string;        // ISO 8601
}
```

When `sources` is an empty array, all listeners go idle. This is a full-replacement snapshot, not a delta. Listeners reconcile their active set against the snapshot.

#### 2. Source-Manager: Overlay Connection Subscriber

Source-manager needs a new internal component that:
- Subscribes to `overlay:connections` Pub/Sub channel (already populated by API Gateway)
- On `"connected"` event: looks up overlay's chat sources from DB, adds them to "has demand" set
- On `"disconnected"` event: removes overlay's sources from "has demand" set
- After every change: publishes a `DemandUpdate` to `source:demand`
- On startup: scans existing `overlay:connected:*` keys to hydrate initial demand state (handles source-manager restarts)

```go
// New file: services/source-manager/demand/subscriber.go
type OverlayDemandSubscriber struct {
    redisClient *redis.Client
    sourceRepo  *registry.Repository
    logger      *zap.Logger
    // demand map: overlay_id -> []ActiveSource
}
```

#### 3. Go SDK: Demand Subscriber Loop (new loop in ListenerBase)

Extend `ListenerBase` to add a 4th background loop: `startDemandSubscriberLoop`. This loop:
- Subscribes to `source:demand` Pub/Sub
- Filters the received source list to only this pod's assigned sources
- Calls a new `ChannelManager` method (or existing `UpdateAssignedSourceIDs` variant) to communicate "active demand subset"
- Also runs a 60s safety-net polling goroutine that calls source-manager for current demand state

Two options for the SDK interface extension:
- **Option A (recommended):** Add `UpdateDemandedSourceIDs(ids map[string]bool)` to `ChannelManager` interface — parallel to `UpdateAssignedSourceIDs`; individual listeners implement it to start/stop connections
- **Option B:** Reuse `UpdateAssignedSourceIDs` but pass intersection of assigned + demanded — but this conflates two separate concepts and breaks the "assignment is sharding, demand is activation" separation

**Recommendation: Option A.** Keeps assignment and demand orthogonal. Go listeners implement `UpdateDemandedSourceIDs` to control actual connections; `UpdateAssignedSourceIDs` continues to control ownership.

#### 4. TikTok Listener: Demand Subscriber (TypeScript)

Replace `pollActiveStreams()` DB query + Redis scan with:
- New `DemandSubscriber` class mirroring `MigrationSubscriber` pattern
- Subscribes to `source:demand` Pub/Sub channel using a duplicate Redis connection
- Calls existing connect/disconnect logic based on demand snapshot
- 60s safety-net: re-subscribe to `source:demand` with a fallback poll to source-manager `/demand?pod_id={pod}` endpoint
- Stop `livePoller` entirely when demand is empty; restart when demand arrives

```typescript
// New file: services/tiktok-listener/src/demand/subscriber.ts
export class DemandSubscriber {
  // Mirrors MigrationSubscriber pattern
  // Uses redisClient.duplicate() for Pub/Sub connection
  async subscribe(): Promise<void> { ... }
}
```

### Recommended Project Structure (New Files)

```
services/source-manager/demand/
└── subscriber.go           # OverlayDemandSubscriber — new component

shared/listener/
└── demand.go               # Demand subscriber loop + DemandedSourceIDs tracking

services/tiktok-listener/src/demand/
└── subscriber.ts           # DemandSubscriber for TikTok listener
```

### Modified Files

| File | Change |
|------|--------|
| `services/source-manager/cmd/main.go` | Instantiate and start OverlayDemandSubscriber |
| `services/source-manager/registry/repository.go` | Add `GetSourcesForOverlays(ctx, []string) []ActiveSource` |
| `shared/listener/base.go` | Add 4th loop: demand subscriber |
| `shared/listener/channel_manager.go` | Add `UpdateDemandedSourceIDs` to interface |
| `services/tiktok-listener/src/index.ts` | Remove `pollActiveStreams()` DB loop, add demand subscriber |
| `services/tiktok-listener/src/index.ts` | Stop/start `livePoller` based on demand |

### Anti-Patterns to Avoid

- **Listeners querying `overlay:connected:*` keys directly:** Violates locked decision — source-manager is the sole authority
- **Delta-based demand signals:** Source-manager should publish full snapshots, not diffs. Diffs require ordered delivery guarantees; Redis Pub/Sub does not guarantee order
- **DB query on every demand event in listeners:** Defeats the purpose. Demand signal must include `channel_id` so listeners can act immediately
- **Adding grace period in listeners:** API Gateway already provides 60s grace; double grace periods cause unnecessary latency for stream shutdown
- **Stopping/restarting the LiveStreamPoller per-target while demand changes:** Stop the entire poller when demand is empty; only add/remove individual targets when demand is non-empty

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pub/Sub reconnection | Custom reconnect loop | `redis.Client.Subscribe()` with context cancellation + outer retry loop (established pattern in `startMigrationSubscriberLoop`) | Already battle-tested in this codebase |
| Demand state persistence across source-manager restart | Redis HSET demand state | Scan `overlay:connected:*` keys on startup | TTL keys are the ground truth; no separate state store needed |
| Channel-level demand tracking across pods | Per-source demand Redis keys | Full snapshot Pub/Sub + listener-side filtering | Simpler; listeners already own their assignment filtering |

---

## Common Pitfalls

### Pitfall 1: Race Between Demand Signal and Assignment Query
**What goes wrong:** Listener pod starts, receives a demand signal before it has queried its assignments. It sees sources in the demand signal but doesn't know which ones are assigned to it yet.

**Why it happens:** Source-manager publishes demand on overlay connect; new listener pod may not yet have assignments.

**How to avoid:** In SDK demand subscriber loop, only act on demand after `UpdateAssignedSourceIDs` has been called at least once (i.e., after `ListenerBase.Start()` has completed the initial assignment query). The demand loop should check `b.hasInitialAssignments` flag before reconciling.

**Warning signs:** Listener connects to sources assigned to a different pod right after startup.

### Pitfall 2: TikTok Listener Loses Demand Signal During Redis Reconnect
**What goes wrong:** Redis disconnects; listener misses a demand update. When Redis reconnects, listener has stale demand state (either stuck active or stuck idle).

**Why it happens:** Redis Pub/Sub does not replay missed messages after reconnect.

**How to avoid:** The 60s safety-net polling loop requests current demand from source-manager. After reconnect, the next 60s tick restores correct state. Keep `livePoller` state consistent with last known demand.

**Warning signs:** Listener is idle despite overlays being open; streams connected with no viewers.

### Pitfall 3: Source-Manager Publishes Empty Demand on Restart
**What goes wrong:** Source-manager restarts, initializes empty demand state, immediately publishes `{sources: []}`, causing all listeners to disconnect.

**Why it happens:** In-memory demand state is cleared on restart before scanning `overlay:connected:*`.

**How to avoid:** Source-manager MUST scan existing `overlay:connected:*` keys and hydrate demand state BEFORE publishing any demand update. Only publish after hydration is complete.

**Warning signs:** Listeners disconnect on source-manager rollout.

### Pitfall 4: ChannelManager Interface Change Breaks Compile-Time Assertions
**What goes wrong:** Adding `UpdateDemandedSourceIDs` to the `ChannelManager` interface breaks all Go listeners that have `var _ listener.ChannelManager = (*Manager)(nil)` compile-time assertions.

**Why it happens:** All 5 migrated Go listeners (twitch, kick, innertube, discord, youtube, twitch-eventsub) implement the interface with compile-time assertions.

**How to avoid:** When adding to the interface, immediately implement the method in all 6 ChannelManagers before landing the SDK change. The compile-time assertions will catch any gap at build time via `make build-all`.

### Pitfall 5: TikTok `pollActiveStreams()` and Demand Subscriber Running Concurrently
**What goes wrong:** During migration, both the old DB-polling loop and the new demand subscriber are active simultaneously, causing duplicate connects/disconnects.

**Why it happens:** Incremental migration of `index.ts` that doesn't fully remove the old path.

**How to avoid:** Delete `startPolling()` and `startDatabaseListener()` in the same commit that adds the demand subscriber. Feature flag or dead-code path is not appropriate here.

---

## Code Examples

Verified patterns from existing codebase:

### Existing Pub/Sub Subscribe Pattern (Go) — Migration Subscriber
```go
// Source: shared/listener/base.go startMigrationSubscriberLoop
subscriber := coordination.NewMigrationSubscriber(b.redisClient, mgr.HandleMigrationEvent, b.logger)
if err := subscriber.Subscribe(ctx); err != nil {
    // backoff and retry
}
<-ctx.Done()
return
```
The demand subscriber loop in `base.go` should follow this exact retry pattern.

### Existing Pub/Sub Subscribe Pattern (TypeScript) — Migration Subscriber
```typescript
// Source: services/tiktok-listener/src/coordination/subscriber.ts
const subscriber = this.redisClient.duplicate();
await subscriber.connect();
await subscriber.subscribe(channel, async (message: string) => {
    await this.handleMessage(message);
});
```
The new `DemandSubscriber` should use the identical `redisClient.duplicate()` pattern.

### Existing Overlay Connection Event (API Gateway → Redis)
```go
// Source: services/api-gateway/websocket/manager.go publishConnectionEvent
event := OverlayConnectionEvent{
    Type:      eventType,   // "connected" or "disconnected"
    OverlayID: overlayID,
    Timestamp: time.Now(),
}
// Published to: "overlay:connections" Pub/Sub channel
// TTL key set/deleted: "overlay:connected:{overlay_id}"
```

### Existing TTL Key Scan for Startup Hydration
```typescript
// Source: services/tiktok-listener/src/index.ts pollActiveStreams (to be replaced)
const keys = await this.redis.keys('overlay:connected:*');
const connectedOverlays = keys.map(key => key.replace('overlay:connected:', ''));
```
Source-manager should use this pattern on startup to hydrate its demand state, not individual listeners.

### ChannelManager Interface (current, to be extended)
```go
// Source: shared/listener/channel_manager.go
type ChannelManager interface {
    Start(ctx context.Context) error
    Stop()
    HandleMigrationEvent(event *coordination.MigrationEvent) error
    UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
    GetFilteredAssignmentCount() int
    GetActiveChannels() []string
    GetActiveChannelCount() int
    // New: UpdateDemandedSourceIDs(demanded map[string]bool)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| TikTok polls DB every 30s always | Will poll only when demand confirmed by source-manager | Phase 5 | Zero DB/TikTok API calls when no overlays open |
| All Go listeners connect all assigned sources immediately | Will connect only sources with active demand | Phase 5 | Reduces TikTok/Kick/YouTube connections by ~100% during off-stream hours |
| Source discovery: listener queries DB directly | Source discovery: source-manager is authority | Phase 5 | DB query eliminated from listener hot path |

---

## Open Questions

1. **Does source-manager need a new HTTP endpoint for demand state?**
   - What we know: Demand is published via Pub/Sub; fallback is a 60s safety-net poll
   - What's unclear: Should the fallback poll source-manager for current demand, or should listeners scan Redis keys as fallback?
   - Recommendation: Add `GET /demand?pod_id={pod_id}` to source-manager — returns sources currently in demand that are assigned to this pod. Avoids listeners having direct Redis key dependency. Source-manager already has `/assignments` as the pattern.

2. **Should demand signal be per-pod or broadcast to all pods?**
   - What we know: Source-manager knows which sources are assigned to which pods
   - What's unclear: Publishing a full unfiltered snapshot vs. pod-targeted messages
   - Recommendation: Broadcast full snapshot to all pods; each pod filters by its own assigned sources. Simpler fan-out; avoids source-manager needing to track which pods are subscribed.

3. **How does the SDK demand loop interact with `DisableCoordinatorFiltering`?**
   - What we know: Some listeners set `DisableCoordinatorFiltering = true` (skips assignment filtering)
   - What's unclear: Should demand filtering also be skippable?
   - Recommendation: Add `DisableDemandsFiltering bool` to `ListenerConfig` parallel to `DisableCoordinatorFiltering`. When true, all assigned sources are treated as having demand (maintains backward compat for Twitch, which is excluded from this phase anyway).

---

## Validation Architecture

> `workflow.nyquist_validation` key is absent from `.planning/config.json` — treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go: standard `testing` + `testify` / TS: Vitest (existing in tiktok-listener) |
| Config file | Go: none (go test ./...); TS: vitest.config.ts if present |
| Quick run command | `cd services/source-manager && go test ./demand/... -v` / `cd services/tiktok-listener && npm test` |
| Full suite command | `make build-all` + per-service `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command |
|--------|----------|-----------|-------------------|
| DEMAND-01 | Source-manager subscribes to `overlay:connections` and updates demand set | unit | `go test ./demand/... -run TestOverlayDemandSubscriber` |
| DEMAND-02 | Source-manager publishes correct `DemandUpdate` on connect/disconnect | unit | `go test ./demand/... -run TestDemandPublish` |
| DEMAND-03 | Source-manager hydrates demand from `overlay:connected:*` keys on startup | unit | `go test ./demand/... -run TestStartupHydration` |
| DEMAND-04 | Go SDK demand loop filters by assigned sources | unit | `go test ../../shared/listener/... -run TestDemandFiltering` |
| DEMAND-05 | TikTok listener connects/disconnects based on demand signal | unit | `npm test -- --grep DemandSubscriber` |
| DEMAND-06 | TikTok listener goes idle (stops livePoller) when demand is empty | unit | `npm test -- --grep idle` |
| DEMAND-07 | SDK demand loop does not act before initial assignments are loaded | unit | `go test ../../shared/listener/... -run TestDemandBeforeAssignments` |

### Wave 0 Gaps
- [ ] `services/source-manager/demand/subscriber_test.go` — covers DEMAND-01, DEMAND-02, DEMAND-03
- [ ] `services/tiktok-listener/src/demand/subscriber.test.ts` — covers DEMAND-05, DEMAND-06
- [ ] `shared/listener/demand_test.go` — covers DEMAND-04, DEMAND-07

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/api-gateway/websocket/manager.go` — overlay connection event publication, TTL keys, grace period
- Direct code inspection: `services/api-gateway/websocket/pool.go` — pool size tracking
- Direct code inspection: `services/tiktok-listener/src/index.ts` — `pollActiveStreams()` current implementation
- Direct code inspection: `services/tiktok-listener/src/coordination/subscriber.ts` — Pub/Sub pattern to mirror
- Direct code inspection: `services/tiktok-listener/src/livestream/poller.ts` — LiveStreamPoller start/stop
- Direct code inspection: `shared/listener/base.go` — ListenerBase, 3 existing loops
- Direct code inspection: `shared/listener/channel_manager.go` — current 7-method interface
- Direct code inspection: `shared/listener/leadership.go` — LeadershipListener pattern
- Direct code inspection: `services/source-manager/coordination/registry.go` — AssignmentRegistry patterns
- Direct code inspection: `.planning/phases/05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients/05-CONTEXT.md` — locked decisions

### Secondary (MEDIUM confidence)
- `services/source-manager/cmd/main.go` — wiring pattern for new source-manager components
- `services/source-manager/registry/repository.go` — DB query patterns to follow for new demand query

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies, all libraries in use
- Architecture: HIGH — full code inspection of all canonical refs
- Pitfalls: HIGH — derived from actual code paths and established SDK patterns
- Interface design: MEDIUM — discretion areas; specific naming left to planner

**Research date:** 2026-03-27
**Valid until:** 2026-04-27 (stable architecture; 30-day window)
