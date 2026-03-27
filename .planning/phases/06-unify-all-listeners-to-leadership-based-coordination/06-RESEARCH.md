# Phase 6: Unify All Listeners to Leadership-Based Coordination - Research

**Researched:** 2026-03-27
**Domain:** Go microservices refactoring — SDK merge, coordinator removal, K8s port consolidation
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Coordinator removal scope**
- D-01: Full removal of coordinator infrastructure — no deprecated fallback, no dead code
- D-02: Delete `shared/coordination/` package entirely (CoordinatorClient, MigrationSubscriber, HeartbeatMonitor, models)
- D-03: Remove coordinator HTTP endpoints from source-manager (`/assignments`, `/heartbeat` on port 8088)
- D-04: Remove `Coordinator.Run()` and all shard-coordinator logic from source-manager
- D-05: Consolidate source-manager to single port 8083 (remove 8088 entirely)

**SDK simplification**
- D-06: Merge ListenerBase into LeadershipListener — single type for all listeners
- D-07: LeadershipListener owns: leadership coordination + demand subscriber loop
- D-08: Remove from SDK: heartbeat loop, assignment refresh loop, migration subscriber loop, `coordinatorClient` interface
- D-09: All listeners import and use LeadershipListener directly — no more ListenerBase

**Twitch IRC handling**
- D-10: Twitch-listener uses LeadershipListener for source discovery only — stays always-connected to IRC channels (no demand gating due to JOIN/PART rate limits)
- D-11: Phase 5's exclusion of Twitch from demand-driven behavior remains — leadership replaces the coordinator's assignment role, nothing more

**Twitch EventSub handling**
- D-12: Twitch-eventsub-listener gets full demand gating — EventSub subscriptions can be created/deleted without rate limits
- D-13: When no overlay is open for a channel, EventSub subscriptions are removed. When demand returns, subscriptions are recreated.

**Migration strategy**
- D-14: Wave 1: Refactor SDK (merge ListenerBase into LeadershipListener)
- D-15: Wave 2: Migrate twitch-listener + twitch-eventsub-listener + clean up kick-listener's dual pattern
- D-16: Wave 3: Remove coordinator from source-manager, consolidate to port 8083, update K8s manifests
- D-17: Safe rollback possible between waves

### Claude's Discretion
- LeadershipListener internal API design (method signatures, config struct)
- How demand subscriber loop integrates into the merged type
- K8s manifest and Service changes for port consolidation
- Test strategy for verifying migration correctness
- Whether kick-listener migration happens in same plan as Twitch or separate

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

---

## Summary

This phase removes the assignment-based coordinator pattern (sharding via HTTP heartbeat/assignment polling) from the SDK and all listeners, replacing it with the leadership-based model already proven by discord, youtube, and innertube listeners. The coordinator infrastructure in source-manager (Coordinator.Run, HeartbeatMonitor, AssignmentRegistry, MigrationPublisher, Rebalancer, Throttler, LoadMonitor) has been superseded by the simpler leadership + demand model introduced in Phases 34-38 and 5. It is now safe to delete.

The phase involves three sequential waves: (1) SDK refactor merging ListenerBase into LeadershipListener and removing coordinator-dependent loops, (2) migrating the three remaining coordinator-dependent listeners (twitch-listener, twitch-eventsub-listener, kick-listener), and (3) gutting source-manager's coordinator subsystem and consolidating it to a single port 8083. This eliminates ~15 files across `shared/coordination/`, `services/source-manager/coordination/`, and listener entry points.

**Primary recommendation:** Follow the three-wave order strictly. Each wave compiles and passes goroutine-leak tests before the next starts. The risk of merge conflicts between waves is low because each wave touches a distinct layer of the stack.

---

## Current Architecture Audit (HIGH confidence — from source code)

### What currently exists

**`shared/coordination/` package (to delete entirely):**
- `client.go` — `CoordinatorClient` struct with `QueryAssignments`, `PublishHeartbeat`, `StartJWTRefresh`, `StopJWTRefresh`
- `migration_subscriber.go` — `MigrationSubscriber` subscribes to `migration:events` Redis channel
- `models.go` — `MigrationEvent`, `MigrationConfirmation`, `AssignmentResponse`, `Assignment` types

**`shared/listener/` SDK (to refactor):**
- `base.go` — `ListenerBase` owns 4 goroutine loops: heartbeat, assignment-refresh, migration-subscriber, demand-subscriber. All 4 will be removed or relocated.
- `leadership.go` — `LeadershipListener` embeds `*ListenerBase`. After the merge, it will stand alone (no embed).
- `channel_manager.go` — `ChannelManager` interface includes `HandleMigrationEvent(*coordination.MigrationEvent) error` — this method must be removed from the interface since it will no longer be called.
- `testutil/mock_coordinator.go` — `MockCoordinator` implements the `coordinatorClient` private interface — delete when interface goes away.

**Listeners on assignment-based (coordinator) pattern (to migrate):**
- `services/twitch-listener/cmd/main.go` — uses `ListenerBase` only; has `coordination.NewCoordinatorClient`, `SERVICE_JWT_SECRET`, `COORDINATOR_URL` env vars
- `services/twitch-eventsub-listener/cmd/main.go` — uses `ListenerBase` only; has `coordination.NewCoordinatorClient`, `SERVICE_JWT_SECRET`, `COORDINATOR_URL` env vars; also has its own Redis-SETNX leader election (`leader:twitch-eventsub`) that will be replaced by `LeadershipListener`
- `services/kick-listener/cmd/main.go` — uses both `ListenerBase` AND `LeadershipListener` (dual pattern); has both `COORDINATOR_URL`/`SERVICE_JWT_SECRET` for coordinator AND `SOURCE_MANAGER_URL`/`SOURCE_MANAGER_SECRET` for leadership

**Listeners already on leadership-only pattern (reference implementations):**
- `services/discord-listener/cmd/main.go` — constructs `ListenerBase` as empty shell, `NewLeadershipListenerFromEnv`, never calls `base.Start/Stop`, runs demand subscriber manually in goroutine
- `services/youtube-listener/cmd/main.go` — same empty-shell ListenerBase pattern (leadership-only)
- `services/youtube-listener-innertube/cmd/main.go` — same pattern

**`services/source-manager/` (coordinator subsystem to remove):**
- `coordination/` directory: `coordinator.go`, `assigner.go`, `assigner_test.go`, `heartbeat.go`, `heartbeat_test.go`, `load_monitor.go`, `load_monitor_test.go`, `migration_publisher.go`, `platform_filtering_test.go`, `rebalancer.go`, `rebalancer_test.go`, `registry.go`, `registry_test.go`, `throttler.go`, `throttler_test.go`, `coordination_lock.go`, `coordination_lock_test.go` (16 files)
- `cmd/main.go` — initialises and starts coordinator, also starts on port 8088; must keep port 8083 startup, remove 8088 and coordinator init
- `handlers/assignment_handler.go` — `/assignments` and `/heartbeat` endpoints (delete)
- HTTP routes `GET /assignments` and `POST /heartbeat` in main.go

**K8s manifests (to update):**
- `deployments/k8s/base/source-manager/deployment.yaml` — port 8088 → 8083 for containerPort, liveness/readiness probes, Service port
- `deployments/k8s/base/configmap.yaml` — `SOURCE_MANAGER_URL` value references port 8088 → 8083
- `deployments/k8s/base/kick-listener/deployment.yaml` — remove `COORDINATOR_URL` and `SERVICE_JWT_SECRET` env vars
- `deployments/k8s/base/twitch-listener/deployment.yaml` — remove `SERVICE_JWT_SECRET`; `SOURCE_MANAGER_URL` and `SOURCE_MANAGER_SECRET` already present
- `deployments/k8s/base/twitch-eventsub-listener/deployment.yaml` — add `SOURCE_MANAGER_URL` and `SOURCE_MANAGER_SECRET`, remove `COORDINATOR_URL`/`SERVICE_JWT_SECRET`

---

## Standard Stack

### Core (no new dependencies required)
| Component | Location | Version | Status |
|-----------|----------|---------|--------|
| `shared/listener` SDK | `shared/go.mod` | local | Refactored in Wave 1 |
| `shared/sourcemanager` | `shared/go.mod` | local | Already used by LeadershipListener |
| `go-redis/v9` | `shared/go.mod` | existing | Retained for demand subscriber loop |
| `go.uber.org/zap` | `shared/go.mod` | existing | No change |
| `go.uber.org/goleak` | per-service `go.mod` | existing | Tests use it already |

No new external dependencies are introduced in this phase. All deletions reduce the dependency surface.

**Verification:** No `npm install` or `go get` needed. This is a pure refactor/delete phase.

---

## Architecture Patterns

### Wave 1: New LeadershipListener API (after merge)

After merging ListenerBase into LeadershipListener, the merged type will own:
- Leadership coordination (EnsureLeadership / source discovery via source-manager API at port 8083)
- Demand subscriber loop (subscribes to `source:demand` Redis Pub/Sub)
- JWT token refresh (via `sourcemanager.NewSigningTokenSource`)
- `Start(ctx, mgr ChannelManager) error` — triggers demand subscriber loop + mgr.Start
- `Stop()` — cancels context, waits for goroutines
- `SMClient() *sourcemanager.Client` — nil-safe accessor
- `LeadershipCoordinator() *sourcemanager.LeadershipCoordinator` — nil-safe accessor

**Construction signatures (recommended):**

```go
// Primary constructor — reads SOURCE_MANAGER_SECRET and SOURCE_MANAGER_URL from env.
// When SOURCE_MANAGER_SECRET is absent, coordination/demand is disabled (nil coordinator).
func NewLeadershipListenerFromEnv(platform string, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error)

// Config-based constructor for tests (avoids env reads).
func NewLeadershipListener(config LeadershipConfig, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error)
```

Note: The `base *ListenerBase` parameter in the existing `NewLeadershipListenerFromEnv` signature is removed because ListenerBase is being merged in.

**Key design decisions for the merger:**
- `redisClient` passed directly to `LeadershipListener` (previously was routed through `ListenerBase`); when nil, demand subscriber loop is disabled (nil-safe, same behavior as before)
- `DisableDemandFiltering` config field moves from `ListenerConfig` to new `LeadershipConfig` (or kept as a field on the merged struct)
- `DisableCoordinatorFiltering` field and coordinator-based `assignedSourceIDs` map are removed entirely — no longer needed
- `hasInitialAssignments` atomic guard is removed — no initial assignment query needed
- `podID` (HOSTNAME) is removed — only used for heartbeat/assignment, both gone
- `StartupJitterMax` is removed — was for spreading pod startup to reduce coordinator load; no coordinator means no thundering herd concern

**Demand reconciliation (simplified):**

Without coordinator assignments, the demand reconciler no longer intersects `assignedSourceIDs` with demanded sources. The platform filter (`b.config.Platform`) is the only filter. All sources matching the platform in a demand update are passed directly to `mgr.UpdateDemandedSourceIDs`.

For `DisableDemandFiltering = true` (Twitch IRC): demand loop exits immediately without subscribing (same behavior as today).

### Wave 2: Listener Migration Pattern

**twitch-listener migration:**

The `twitch-listener` used `DisableCoordinatorFiltering = true` (as rollback knob) and `DisableDemandFiltering = true`. After migration:
- Coordinator client and related env vars are deleted from `cmd/main.go`
- `LeadershipListener` is constructed (discovery via source-manager at port 8083)
- `DisableDemandFiltering = true` is preserved (Twitch IRC stays always-connected)
- `ll.Start(ctx, channelMgr)` replaces `base.Start(ctx, channelMgr)`
- `listener.ShutdownCoordinator` signature updated — takes `*LeadershipListener` instead of `*ListenerBase`

The twitch-listener `channels.Manager.HandleMigrationEvent` method and all migration-related internal logic can be deleted once the `ChannelManager` interface no longer requires it.

**twitch-eventsub-listener migration:**

Currently has a custom Redis-SETNX leader election loop (`leader:twitch-eventsub` key, 10s TTL). This is replaced by `LeadershipListener` with `EnsureLeadership`. The `leaderState` struct and the goroutine with `tryAcquireLeadership` / `releaseLeadership` are deleted.

The `SetSubscriptionCallback` is called conditionally based on `isLeader` (from the old `leaderState` struct). After migration, the subscription callback checks leadership via `ll.LeadershipCoordinator().EnsureLeadership` or the service can call `ll.LeadershipCoordinator().IsLeader()` as a nil-safe check.

D-12/D-13 demand gating: `channelManager.UpdateDemandedSourceIDs` triggers subscription create/delete. This requires twitch-eventsub-listener's `channels.Manager` to implement `UpdateDemandedSourceIDs` that drives subscription lifecycle.

**kick-listener migration:**

Currently creates both a `ListenerBase` (coordinator) AND a `LeadershipListener`. After migration, the `ListenerBase` construction and `coordination.NewCoordinatorClient` are removed. Only `NewLeadershipListenerFromEnv` remains. This simplifies `cmd/main.go` by ~15 lines and removes the `COORDINATOR_URL`/`SERVICE_JWT_SECRET` env dependencies.

### Wave 3: Source-Manager Cleanup

**Source-manager port consolidation:**

The source-manager currently has a single HTTP server on port 8088. The leadership API (`/sources`, `/leadership/*`, `/demand`) all live on 8088 (per source code audit). Port 8083 is referenced in env vars (`SOURCE_MANAGER_URL: http://source-manager...:8083`) but is NOT the actual port the source-manager is running on today — the manifest and configmap both use 8088.

Wait — this is a critical discrepancy. Let me state the actual current state precisely:
- source-manager `cmd/main.go`: `port := getEnvOrDefault("PORT", "8088")` — serves everything on one port
- `configmap.yaml`: `SOURCE_MANAGER_URL: http://source-manager.allchat.svc.cluster.local:8088` — points to 8088
- `LeadershipListener` SDK default: `Env("SOURCE_MANAGER_URL", "http://source-manager:8083")` — defaults to 8083
- K8s Service for source-manager: exposes port 8088

So the `SOURCE_MANAGER_URL` configmap value (`...:8088`) is what running listeners use. The SDK default of `8083` is only the fallback when the env var is absent (local dev without configmap). After this phase, source-manager must move to port 8083 — both the container/service port AND the configmap value must change to 8083.

**Source-manager internal removals:**
- Delete `services/source-manager/coordination/` directory (16 Go files)
- Remove `handlers/assignment_handler.go` (handles `/assignments` and `/heartbeat`)
- In `cmd/main.go`: remove coordinator construction block (~50 lines), `coordinator.Run(ctx)` goroutine, `coordinator.Stop()` in shutdown, assignment handler routes
- Keep: sourceRegistry, leaderManager, cleanupJob, demandSubscriber, sourceHandler (leadership), demandHandler

**ChannelManager interface change:**

`HandleMigrationEvent(*coordination.MigrationEvent) error` is removed from the `ChannelManager` interface in `shared/listener/channel_manager.go`. All 3 listener managers that implement it (twitch-listener, twitch-eventsub-listener, kick-listener) have their `HandleMigrationEvent` method deleted. The compile-time assertions `var _ listener.ChannelManager = (*Manager)(nil)` will catch any drift.

### Recommended Project Structure After Phase 6

```
shared/listener/
├── leadership.go         # LeadershipListener (merged, standalone — no ListenerBase embed)
├── channel_manager.go    # ChannelManager interface (HandleMigrationEvent REMOVED)
├── config.go             # LeadershipConfig (simplified — coordinator fields removed)
├── demand.go             # demand subscriber loop (moved into leadership.go or kept as helpers)
├── shutdown.go           # ShutdownCoordinator (signature updated)
└── testutil/
    └── redisutil/        # miniredis helpers (remove mock_coordinator.go)

shared/coordination/      # DELETED ENTIRELY

services/source-manager/
├── cmd/main.go           # No coordinator init, port 8083
├── coordination/         # DELETED ENTIRELY
├── handlers/             # assignment_handler.go DELETED
│   ├── health_handler.go
│   ├── source_handler.go
│   └── demand_handler.go (unchanged)
├── demand/               # Unchanged
├── election/             # Unchanged
├── registry/             # Unchanged
└── cleanup/              # Unchanged
```

### Anti-Patterns to Avoid
- **Soft deprecation via feature flags:** D-01 mandates full removal. Do not add `ENABLE_COORDINATOR_FILTERING=false` or similar rollback switches.
- **Keeping ListenerBase as a private field in LeadershipListener:** The merge means LeadershipListener IS the base — no embedding. Embedding creates a confusing API surface.
- **Calling `base.Start` / `base.Stop` in leadership-only listeners after the merge:** Discord, youtube, innertube currently use ListenerBase as an empty shell and never call Start/Stop. After the merge, they should call `ll.Start(ctx, mgr)` only when they have a ChannelManager, or skip Start entirely if demand filtering is not needed (no-op path).
- **Leaving `migration:events` subscription in any service:** The MigrationSubscriber Redis channel is only published to by the coordinator being removed. After deletion, no service publishes to `migration:events`. Any lingering subscriber would silently do nothing but waste a Redis subscription.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Leader election for twitch-eventsub | Custom Redis SETNX loop (`leader:twitch-eventsub`) | `LeadershipListener.EnsureLeadership` via source-manager API | Eliminates divergent leader election logic; K8s Lease-backed via source-manager |
| JWT token refresh | New refresh goroutine | `sourcemanager.NewSigningTokenSource` (already in LeadershipListener) | Already handles 15min TTL, signing, and nil-safety |
| Platform filtering in demand reconciler | Custom platform filtering logic | Existing `Platform` field in `LeadershipConfig` (moved from ListenerConfig) | Already implemented in `demand.go`, just move it |

---

## Common Pitfalls

### Pitfall 1: Port 8083 vs 8088 Confusion
**What goes wrong:** The `LeadershipListener` SDK defaults to `SOURCE_MANAGER_URL=http://source-manager:8083`. The configmap currently sets it to `...:8088`. Both the configmap AND the source-manager server must change in the same Wave 3 deployment.
**Why it happens:** Port 8083 was the originally-planned leadership port; 8088 ended up as the actual deployed port.
**How to avoid:** In Wave 3, update configmap.yaml and source-manager deployment.yaml port + probes in a single commit. Do not deploy source-manager on 8083 before updating configmap (all SDK-based listeners read SOURCE_MANAGER_URL from configmap).
**Warning signs:** LeadershipListener log `Failed to query sources from source-manager` after deploying source-manager change.

### Pitfall 2: ChannelManager Interface Drift
**What goes wrong:** After removing `HandleMigrationEvent` from the `ChannelManager` interface, compile-time assertions catch mismatches — but only if the assertions exist in the channel manager files.
**Why it happens:** All three migrating listeners have `var _ listener.ChannelManager = (*Manager)(nil)` assertions. If `HandleMigrationEvent` is deleted from the interface but NOT from a manager (or vice versa), the assertion will fail at compile time.
**How to avoid:** Remove `HandleMigrationEvent` from the interface and from all 3 manager files in the same plan. Run `make build-all` after Wave 1 to catch any file that still imports coordination.
**Warning signs:** `cannot use (*Manager)(nil) as type listener.ChannelManager: Manager.HandleMigrationEvent does not implement` at compile time.

### Pitfall 3: Leadership-Only Listeners Don't Call Start
**What goes wrong:** Discord, youtube, and innertube listeners currently use ListenerBase as an empty shell and never call `Start`. After the merge, they need to know whether to call `ll.Start(ctx, mgr)` or not.
**Why it happens:** These listeners don't have a `ChannelManager` in the SDK sense — they manage channels directly in main.go. The demand subscriber loop was added as a standalone goroutine (Phase 5, D-14 from that phase).
**How to avoid:** These three listeners SHOULD call `ll.Start(ctx, mgr)` if they have a ChannelManager that satisfies the interface, OR keep the standalone demand goroutine if they don't (acceptable but inconsistent). The cleanest approach: if the listener has no ChannelManager, keep the standalone goroutine from Phase 5 as-is — don't force-fit them into a ChannelManager for this phase.
**Warning signs:** Demand filtering stops working for discord/youtube/innertube after the merge.

### Pitfall 4: twitch-eventsub Subscription Storm on Demand Recovery
**What goes wrong:** When demand returns for all channels simultaneously (e.g., source-manager restart), twitch-eventsub-listener tries to create EventSub subscriptions for all channels at once, potentially hitting Twitch rate limits.
**Why it happens:** `UpdateDemandedSourceIDs` fires with all demanded sources in one batch.
**How to avoid:** The existing `subscriptionMgr` already handles "subscription already exists" gracefully. The bulk create path can be rate-limited by the existing sequential loop. No new rate limiting needed.
**Warning signs:** Log spam of `Failed to subscribe to channel points` with 429 errors.

### Pitfall 5: twitch-eventsub Leader Election State in Redis
**What goes wrong:** The old `leader:twitch-eventsub` Redis key (used by the custom SETNX loop) may persist after migration. The new LeadershipListener uses a different key format (`source:leader:{platform}:{shard}`).
**Why it happens:** Redis keys are not cleaned up on service restart.
**How to avoid:** After migrating twitch-eventsub-listener and deploying, run `redis-cli DEL leader:twitch-eventsub` to clean up the stale key. Document this as a post-deployment step.
**Warning signs:** None — the stale key won't conflict with the new system. But it's dead weight in Redis.

### Pitfall 6: ShutdownCoordinator Signature Change
**What goes wrong:** `listener.ShutdownCoordinator(base, channelMgr, ...)` currently takes `*ListenerBase` as first arg. After Wave 1, this must change to accept `*LeadershipListener` (or be polymorphic via an interface).
**Why it happens:** The shutdown helper was designed for the base type.
**How to avoid:** Simplest fix — change `ShutdownCoordinator` to take `interface{ Stop() }` as the first argument, or replace with a simpler inline shutdown pattern in each listener's main.go.

---

## Code Examples

### Current LeadershipListener construction (kick-listener pattern)
```go
// Source: services/kick-listener/cmd/main.go (current dual pattern)
base := listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)
l, err := listener.NewLeadershipListenerFromEnv(base, "kick", log)
// ...
if err := l.Start(ctx, channelMgr); err != nil { ... }
```

### Target LeadershipListener construction (after merge)
```go
// Source: decision D-06/D-09 — single type, no base
ll, err := listener.NewLeadershipListenerFromEnv("kick", redisClient, log)
if err != nil { log.Fatal(...) }
// ...
if err := ll.Start(ctx, channelMgr); err != nil { ... }
```

### ChannelManager interface after change
```go
// Source: shared/listener/channel_manager.go (after Wave 1)
// HandleMigrationEvent is REMOVED — no longer called by any SDK code
type ChannelManager interface {
    Start(ctx context.Context) error
    Stop()
    // HandleMigrationEvent DELETED
    UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
    UpdateDemandedSourceIDs(demanded map[string]DemandedSource)
    GetFilteredAssignmentCount() int
    GetActiveChannels() []string
    GetActiveChannelCount() int
}
```

Note: `UpdateAssignedSourceIDs` may also be removable since the coordinator assignment model is gone. However, if any listener channel manager internally uses `assignedSourceIDs` for filtering, the method stays as a no-op until that filtering is also removed. Recommend keeping it as a no-op stub in Wave 1 and removing it only when all managers are confirmed clean.

### twitch-eventsub leader election replacement
```go
// Before (current): custom Redis SETNX loop
acquired, err := tryAcquireLeadership(ctx, redisClient, instanceID)
state.Lock()
state.isLeader = acquired
state.Unlock()

// After: EnsureLeadership via LeadershipListener (same as discord-listener pattern)
acquired, err := ll.LeadershipCoordinator().EnsureLeadership(ctx, "shard:0", func() {
    log.Warn("Lost EventSub leadership — stopping subscription management")
    channelManager.Stop()
})
```

### Source-manager main.go after Wave 3 (coordinator sections removed)
```go
// Lines to DELETE from source-manager/cmd/main.go:
//   assignmentRegistry := coordination.NewAssignmentRegistry(redisClient)
//   assigner := coordination.NewAssigner([]string{})
//   heartbeatMonitor := coordination.NewHeartbeatMonitor(redisClient, log, shardMetrics)
//   migrationPublisher := coordination.NewMigrationPublisher(redisClient, shardMetrics, log)
//   loadMonitor := coordination.NewLoadMonitor(...)
//   rebalancer := coordination.NewRebalancer(...)
//   throttler := coordination.NewThrottler(...)
//   coordinator := coordination.NewCoordinator(...)
//   go coordinator.Run(ctx)
//   coordinator.Stop()
//   protected.GET("/assignments", assignmentHandler.GetAssignments)
//   protected.POST("/heartbeat", assignmentHandler.PublishHeartbeat)
//
// port := getEnvOrDefault("PORT", "8083")  ← changed from "8088"
```

---

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | `leader:twitch-eventsub` Redis key — SETNX lock from old leader election | Manual cleanup: `redis-cli DEL leader:twitch-eventsub` post-deploy |
| Stored data | `assignment:*` Redis keys — set by `AssignmentRegistry` | Coordinator removal stops new writes; keys expire or can be flushed with `redis-cli DEL` pattern |
| Stored data | `heartbeat:*` Redis keys — set by `HeartbeatMonitor` | Same as above — expire naturally |
| Stored data | `rebalance:*` / `coordinator:lock` Redis keys — used by Throttler/CoordinatorLock | Expire naturally; no migration needed |
| Live service config | K8s `allchat-config` ConfigMap `SOURCE_MANAGER_URL` value uses port 8088 | Update configmap value to port 8083 in Wave 3 |
| Live service config | K8s Service for source-manager exposes port 8088 | Change Service port to 8083 in Wave 3 |
| OS-registered state | None — no OS-level registrations | None |
| Secrets/env vars | `SERVICE_JWT_SECRET` secret key used by coordinator auth in twitch-listener, twitch-eventsub, kick-listener | Secret key itself is unchanged; env var references in deployment.yaml are removed. The secret in K8s (`allchat-secrets`) is not deleted — still used for other services. |
| Secrets/env vars | `COORDINATOR_URL` env var in kick-listener and twitch-eventsub deployments | Remove from deployment.yaml in Wave 3 |
| Build artifacts | Binary artifacts in Docker images — will be rebuilt by CI | No manual action — handled by normal build/deploy |

**Nothing found in category:** OS-registered state — verified by code audit, no systemd/launchd/scheduler references.

---

## Environment Availability

Step 2.6: SKIPPED — this phase is a pure Go code refactor/delete with no new external tool dependencies. All tools (`go`, `kubectl`, `redis-cli`) are already required by the project and assumed present.

---

## Validation Architecture

`workflow.nyquist_validation` is not set in `.planning/config.json` → treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (`go test ./...`) + goleak |
| Config file | none (standard Go testing) |
| Quick run command | `cd services/{service} && go test ./cmd/... -count=1 -timeout 30s` |
| Full suite command | `make build-all` (cross-module compile check) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-06/D-09 | LeadershipListener Start/Stop has no goroutine leak | unit + goleak | `cd shared && go test github.com/caesar/all-chat/shared/listener -run TestLeadershipListener -count=1` | ❌ Wave 0 (new test) |
| D-08 | No heartbeat/assignment-refresh/migration loops in SDK | unit | `cd shared && go test github.com/caesar/all-chat/shared/listener -count=1 -race` | partial (existing tests check goroutine count) |
| D-09 | twitch-listener compiles with LeadershipListener only | compile | `cd services/twitch-listener && go build ./...` | ✅ |
| D-09 | twitch-eventsub compiles with LeadershipListener only | compile | `cd services/twitch-eventsub-listener && go build ./...` | ✅ |
| D-09 | kick-listener compiles with LeadershipListener only | compile | `cd services/kick-listener && go build ./...` | ✅ |
| D-01/D-02 | shared/coordination is not imported by any service | compile | `make build-all` | N/A — verified by build succeeding |
| D-03/D-04/D-05 | source-manager serves on port 8083, no coordinator routes | smoke | manual `curl http://source-manager:8083/health/live` | N/A |
| goroutine safety | Each listener's SDK smoke test passes goleak | unit + goleak | `go test ./cmd/... -run TestLeadershipListener` per listener | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/{service} && go build ./...` (compile check)
- **Per wave merge:** `make build-all` (cross-module compile verification)
- **Phase gate:** Full goroutine-leak smoke tests across all 3 migrated listeners before Wave 3

### Wave 0 Gaps
- [ ] `shared/listener/leadership_merged_test.go` — goroutine-leak test for merged LeadershipListener `Start/Stop` (covers D-06/D-08)
- [ ] `services/twitch-listener/cmd/main_sdk_test.go` — update mock to remove `HandleMigrationEvent` and update `LeadershipListener` smoke test
- [ ] `services/twitch-eventsub-listener/cmd/main_sdk_test.go` — same as above
- [ ] `services/kick-listener/cmd/main_sdk_test.go` — same as above

The existing `TestListenerBase_StartStop_NoGoroutineLeak` tests in all three listeners will need to be renamed and updated to use `NewLeadershipListenerFromEnv` / mock (no coordinator mock needed).

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HTTP heartbeat to coordinator | K8s Lease leader election via source-manager | Phase 34 (SDK) | Coordinator heartbeats become unnecessary |
| HTTP assignment polling | Source discovery via leadership API (`GET /sources`) | Phase 34-38 | Coordinator assignment endpoint superseded |
| `migration:events` Redis channel for pod migration | Source-manager leadership model handles source ownership directly | Phase 34-38 | Migration subscriber becomes dead code |
| Dual-port source-manager (8088 coordinator + 8083 leadership) | Single-port on 8083 (leadership only) | This phase | Simpler networking, no split brain risk |

**Deprecated/outdated patterns after this phase:**
- `shared/coordination` package: superseded entirely by `shared/sourcemanager`
- `ListenerBase` struct: superseded by `LeadershipListener` (merged)
- `coordinatorClient` private interface: removed with the base
- `HandleMigrationEvent` method: removed from `ChannelManager` interface

---

## Open Questions

1. **`UpdateAssignedSourceIDs` removal**
   - What we know: The method is called by `ListenerBase.Start` and `startAssignmentRefreshLoop`. After coordinator removal, no SDK code calls it. Channel managers currently use it to filter which channels to connect to.
   - What's unclear: Do any managers use `assignedSourceIDs` for filtering after coordinator is gone, or can this map always be nil/empty post-migration?
   - Recommendation: Keep `UpdateAssignedSourceIDs(map[string]bool)` in the `ChannelManager` interface as a no-op in Wave 1. Remove it in a follow-up pass once all managers are confirmed to not use it internally.

2. **`GetFilteredAssignmentCount` metric**
   - What we know: `shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(filteredCount))` uses this count in twitch-listener and kick-listener.
   - What's unclear: What is the semantically correct value post-coordinator? "Filtered by assignments" has no meaning without an assigner.
   - Recommendation: `GetFilteredAssignmentCount()` can return `GetActiveChannelCount()` (all active channels, since no filtering applies). The metric becomes "active channel count" rather than "filtered assignment count". Rename the metric label in a follow-up.

3. **discord/youtube/innertube demand subscriber goroutines**
   - What we know: These listeners added a standalone `source:demand` subscription goroutine in Phase 5 (not via SDK) because they don't call `base.Start`. After the merge, `ll.Start(ctx, mgr)` would run the demand subscriber via SDK if they have a `ChannelManager`.
   - What's unclear: Whether it's cleaner to give these listeners a minimal ChannelManager wrapper (consolidation) or keep standalone goroutines (minimal change scope for this phase).
   - Recommendation: Out of scope for Phase 6. These listeners are not in the migration list (D-15 specifies twitch + twitch-eventsub + kick). Keep standalone goroutines in Phase 6; consolidate in a follow-up if desired.

---

## Sources

### Primary (HIGH confidence)
- Direct code audit of `/home/caesar/git/all-chat/shared/listener/` — all 8 SDK files read in full
- Direct code audit of `services/twitch-listener/cmd/main.go`, `services/twitch-eventsub-listener/cmd/main.go`, `services/kick-listener/cmd/main.go` — read in full
- Direct code audit of `services/discord-listener/cmd/main.go` — reference implementation, read in full
- Direct code audit of `services/source-manager/cmd/main.go` — read in full
- Direct code audit of `shared/coordination/` — client.go, migration_subscriber.go, models.go read in full
- Direct code audit of `deployments/k8s/base/` YAML files — deployment, service, configmap read in full
- Direct code audit of `shared/listener/testutil/mock_coordinator.go` — read in full

### Secondary (MEDIUM confidence)
- `.planning/phases/06-unify-all-listeners-to-leadership-based-coordination/06-CONTEXT.md` — user decisions (verbatim decisions replicated above)
- `.planning/STATE.md` — accumulated decisions from Phases 33-38 and Phase 5

### Tertiary (LOW confidence)
- None — all findings are from direct code inspection, not web search

---

## Metadata

**Confidence breakdown:**
- What to delete: HIGH — enumerated from direct code audit
- Wave sequencing: HIGH — from CONTEXT.md decisions D-14/D-15/D-16
- New API signatures: MEDIUM — recommended based on existing patterns; exact signatures are Claude's discretion per CONTEXT.md
- K8s port change risk: HIGH — port mismatch between configmap (8088) and SDK default (8083) documented; must change together

**Research date:** 2026-03-27
**Valid until:** 2026-04-27 (stable Go codebase; no external API dependencies change)
