# Phase 6: Unify All Listeners to Leadership-Based Coordination - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Eliminate the dual coordinator/leadership architecture by migrating all listeners to use a single LeadershipListener type. Remove the coordinator HTTP API (source-manager:8088), CoordinatorClient, ListenerBase, shared/coordination package, and consolidate source-manager to a single port (8083). After this phase, every listener uses LeadershipListener for source discovery and (where applicable) demand gating.

</domain>

<decisions>
## Implementation Decisions

### Coordinator removal scope
- **D-01:** Full removal of coordinator infrastructure — no deprecated fallback, no dead code
- **D-02:** Delete `shared/coordination/` package entirely (CoordinatorClient, MigrationSubscriber, HeartbeatMonitor, models)
- **D-03:** Remove coordinator HTTP endpoints from source-manager (`/assignments`, `/heartbeat` on port 8088)
- **D-04:** Remove `Coordinator.Run()` and all shard-coordinator logic from source-manager
- **D-05:** Consolidate source-manager to single port 8083 (remove 8088 entirely)

### SDK simplification
- **D-06:** Merge ListenerBase into LeadershipListener — single type for all listeners
- **D-07:** LeadershipListener owns: leadership coordination + demand subscriber loop
- **D-08:** Remove from SDK: heartbeat loop, assignment refresh loop, migration subscriber loop, `coordinatorClient` interface
- **D-09:** All listeners import and use LeadershipListener directly — no more ListenerBase

### Twitch IRC handling
- **D-10:** Twitch-listener uses LeadershipListener for source discovery only — stays always-connected to IRC channels (no demand gating due to JOIN/PART rate limits)
- **D-11:** Phase 5's exclusion of Twitch from demand-driven behavior remains — leadership replaces the coordinator's assignment role, nothing more

### Twitch EventSub handling
- **D-12:** Twitch-eventsub-listener gets full demand gating — EventSub subscriptions can be created/deleted without rate limits
- **D-13:** When no overlay is open for a channel, EventSub subscriptions are removed. When demand returns, subscriptions are recreated.

### Migration strategy
- **D-14:** Wave 1: Refactor SDK (merge ListenerBase into LeadershipListener)
- **D-15:** Wave 2: Migrate twitch-listener + twitch-eventsub-listener + clean up kick-listener's dual pattern
- **D-16:** Wave 3: Remove coordinator from source-manager, consolidate to port 8083, update K8s manifests
- **D-17:** Safe rollback possible between waves

### Claude's Discretion
- LeadershipListener internal API design (method signatures, config struct)
- How demand subscriber loop integrates into the merged type
- K8s manifest and Service changes for port consolidation
- Test strategy for verifying migration correctness
- Whether kick-listener migration happens in same plan as Twitch or separate

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Listener SDK (being refactored)
- `shared/listener/base.go` — ListenerBase struct, Start/Stop lifecycle, 4 background loops (to be removed/merged)
- `shared/listener/leadership.go` — LeadershipListener struct (target single type)
- `shared/listener/channel_manager.go` — ChannelManager interface (7 methods)
- `shared/listener/config.go` — ListenerConfig struct
- `shared/listener/demand.go` — Demand subscriber loop (stays)
- `shared/listener/shutdown.go` — ShutdownCoordinator helper

### Coordinator infrastructure (being removed)
- `shared/coordination/client.go` — CoordinatorClient HTTP client (DELETE)
- `shared/coordination/migration_subscriber.go` — MigrationSubscriber (DELETE)
- `shared/coordination/models.go` — Assignment, MigrationEvent models (DELETE)
- `services/source-manager/coordination/` — Coordinator, HeartbeatMonitor, shard logic (DELETE)
- `services/source-manager/cmd/main.go` — Coordinator startup, port 8088 (MODIFY)

### Listeners being migrated
- `services/twitch-listener/cmd/main.go` — Currently ListenerBase only, needs LeadershipListener
- `services/twitch-eventsub-listener/cmd/main.go` — Currently ListenerBase only, needs LeadershipListener + demand gating
- `services/kick-listener/cmd/main.go` — Uses both ListenerBase + LeadershipListener, remove coordinator dependency

### Listeners already on LeadershipListener (reference implementations)
- `services/discord-listener/cmd/main.go` — LeadershipListener pattern
- `services/youtube-listener-innertube/cmd/main.go` — LeadershipListener pattern
- `services/youtube-listener/cmd/main.go` — LeadershipListener pattern

### Phase 5 demand-driven context
- `.planning/phases/05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients/05-CONTEXT.md` — Demand signal architecture decisions

### K8s manifests
- `k8s/source-manager/` — Deployment, Service (port consolidation)
- `k8s/twitch-listener/` — Env vars referencing COORDINATOR_URL (remove)
- `k8s/twitch-eventsub-listener/` — Env vars referencing COORDINATOR_URL (remove)
- `k8s/kick-listener/` — Env vars referencing COORDINATOR_URL (remove)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/listener/leadership.go`: LeadershipListener already works for discord, innertube, youtube — proven pattern
- `shared/sourcemanager/`: LeadershipCoordinator, Client — source discovery API used by all leadership listeners
- `shared/listener/demand.go`: Demand subscriber loop — already in ListenerBase, needs to move to LeadershipListener

### Established Patterns
- LeadershipListener constructed via `NewLeadershipListenerFromEnv(base, platform, logger)` — will need new signature without base
- `SOURCE_MANAGER_SECRET` env var enables/disables leadership (nil-safe when absent)
- `ChannelManager` interface (7 methods) remains unchanged — all listeners implement it

### Integration Points
- source-manager port 8083: leadership API, demand API, health checks — stays
- source-manager port 8088: coordinator API — removed
- K8s Service definitions for source-manager — port mapping changes
- COORDINATOR_URL env var in listener deployments — removed
- SERVICE_SECRET env var in listener deployments — may change if auth approach changes

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-unify-all-listeners-to-leadership-based-coordination*
*Context gathered: 2026-03-27*
