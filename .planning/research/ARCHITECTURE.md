# Architecture Patterns: Listener SDK

**Domain:** Shared Go SDK for listener microservices in All-Chat platform
**Researched:** 2026-03-17
**Confidence:** HIGH — based entirely on direct code inspection of the live codebase

---

## Recommended Architecture

The Listener SDK lives in `/shared/listener/` as a new package within the existing `github.com/caesar/all-chat/shared` Go module. No new Go module is introduced. No `go.work` file is needed. All listener `go.mod` files already use `replace github.com/caesar/all-chat/shared => ../../shared` to point at the monorepo `shared/` directory. Adding a new package inside `shared/` is automatically available to all listeners that already declare this replace directive.

```
shared/
├── coordination/            # EXISTING: CoordinatorClient, MigrationSubscriber
├── sourcemanager/           # EXISTING: LeadershipCoordinator, Client, LeadershipClient
├── metrics/                 # EXISTING: ShardMetrics, ListenerMetrics
├── listener/                # NEW: Listener SDK
│   ├── base.go              # NEW: ListenerBase struct + Config
│   ├── leadership.go        # NEW: LeadershipListener variant
│   ├── channel_manager.go   # NEW: ChannelManager interface + BaseChannelManager
│   └── shutdown.go          # NEW: ShutdownCoordinator (Gin + platform connections)
├── auth/                    # EXISTING
├── database/                # EXISTING
└── ...
```

The two listener archetypes require different SDK types:

| Archetype | Examples | SDK Type |
|-----------|----------|----------|
| Sharded/assigned listeners | Twitch, Kick, (future Discord) | `ListenerBase` |
| Leadership-per-stream listeners | YouTube InnerTube, Kick (leadership aspect), Discord shard | `LeadershipListener` (embeds `ListenerBase`) |

---

## Package Structure

### `shared/listener/base.go` — ListenerBase

`ListenerBase` replaces the ~120 lines of identical startup wiring found in `twitch-listener/cmd/main.go` and `kick-listener/cmd/main.go`. It encapsulates:

1. Startup jitter (`rand.Intn(30)` seconds — prevents thundering herd)
2. Assignment query (blocks indefinitely with backoff via `CoordinatorClient.QueryAssignments`)
3. JWT refresh background task (`CoordinatorClient.StartJWTRefresh`)
4. Heartbeat loop (10-second interval, `CoordinatorClient.PublishHeartbeat`)
5. Assignment refresh loop (60-second interval, re-queries coordinator)
6. Migration subscriber (wires `coordination.NewMigrationSubscriber` to the platform's `HandleMigrationEvent`)

```go
// Package path: github.com/caesar/all-chat/shared/listener

type Config struct {
    ServiceName     string
    CoordinatorURL  string
    ServiceSecret   string
    PodID           string
    JitterMaxSec    int           // default: 30
    HeartbeatInterval time.Duration // default: 10s
    AssignmentRefreshInterval time.Duration // default: 60s
    EnableFiltering bool
    Logger          *zap.Logger
    RedisClient     *redis.Client
}

type ListenerBase struct {
    Config
    coordClient *coordination.CoordinatorClient
    // unexported fields for goroutine lifecycle
}

// Start performs jitter + assignment query. Returns initial assigned source IDs.
// Caller MUST call Stop() on shutdown.
func (b *ListenerBase) Start(ctx context.Context) (map[string]bool, error)

// Run starts background loops: heartbeat, assignment refresh, migration subscriber.
// migrationHandler is the platform-specific HandleMigrationEvent method.
func (b *ListenerBase) Run(ctx context.Context, migrationHandler func(*coordination.MigrationEvent))

// Stop signals all background loops to exit gracefully.
func (b *ListenerBase) Stop()

// GetFilteredAssignedSourceIDs returns the current assignment map (updated by refresh loop).
func (b *ListenerBase) GetFilteredAssignedSourceIDs() map[string]bool
```

### `shared/listener/leadership.go` — LeadershipListener

`LeadershipListener` embeds `ListenerBase` and adds:

- Wiring for `sourcemanager.LeadershipCoordinator` (claim/renew/release per stream)
- Source-manager client creation (reuses `sourcemanager.NewSigningTokenSource` + `sourcemanager.NewClient`)
- Nil-safe operation: coordinator is optional (logs warning if `SOURCE_MANAGER_SECRET` is empty)

```go
type LeadershipConfig struct {
    Config                           // embeds ListenerBase config
    Platform            string       // "youtube", "kick", "discord"
    SourceManagerURL    string
    SourceManagerSecret string
    RenewalInterval     time.Duration // default: 5s
}

type LeadershipListener struct {
    ListenerBase
    LeaderCoord *sourcemanager.LeadershipCoordinator
    SMClient    *sourcemanager.Client
}

// New creates the coordinator + client (or nil if secret is empty) and returns a LeadershipListener.
func NewLeadershipListener(cfg LeadershipConfig) (*LeadershipListener, error)
```

Usage: youtube-listener-innertube and kick-listener embed `LeadershipListener` instead of wiring their own `sourcemanager.NewLeadershipCoordinator` calls (currently duplicated in both services).

### `shared/listener/channel_manager.go` — ChannelManager

`ChannelManager` is an **interface** that both Twitch and Kick channel managers already satisfy (they have matching method sets). Extracting the interface to `/shared/listener/` enables `ListenerBase.Run` to accept any channel manager without importing platform-specific packages.

```go
// ChannelManager is the interface all platform channel managers must implement.
type ChannelManager interface {
    // Start begins periodic sync and DB LISTEN. Called after platform connection is established.
    Start(ctx context.Context) error

    // Stop gracefully stops sync loops and releases resources.
    Stop()

    // HandleMigrationEvent processes a channel migration event from the coordinator.
    HandleMigrationEvent(event *coordination.MigrationEvent)

    // UpdateAssignedSourceIDs replaces the current assignment filter map.
    UpdateAssignedSourceIDs(ids map[string]bool)

    // GetFilteredAssignmentCount returns the number of assigned sources with active channels.
    GetFilteredAssignmentCount() int
}
```

The existing `twitch-listener/channels.Manager` and `kick-listener/channels.Manager` already implement all these methods. Migration requires only adding the `ChannelManager` interface import — no structural changes to either manager.

**Where ChannelManager lives relative to existing packages:**

- `shared/coordination/` remains unchanged — it owns `CoordinatorClient`, `MigrationSubscriber`, `MigrationEvent`
- `shared/sourcemanager/` remains unchanged — it owns `LeadershipCoordinator`, `Client`, `LeadershipClient`
- `shared/listener/channel_manager.go` introduces the `ChannelManager` **interface** that ties them together at the SDK layer. The concrete implementations stay in each listener's `channels/` package.

This preserves the existing package boundaries while enabling `ListenerBase.Run` to accept a `ChannelManager` and wire migration events without importing platform-specific code.

### `shared/listener/shutdown.go` — ShutdownCoordinator

Both Twitch and Kick implement identical shutdown sequences (channel manager stop → platform disconnect → HTTP server graceful shutdown). `ShutdownCoordinator` extracts this into a reusable helper.

```go
// PlatformConnector is the minimal interface for platform connections that need cleanup.
type PlatformConnector interface {
    Disconnect() error
}

type ShutdownConfig struct {
    Logger       *zap.Logger
    HTTPServer   *http.Server
    HTTPTimeout  time.Duration // default: 10s
}

type ShutdownCoordinator struct {
    cfg ShutdownConfig
}

// Wait blocks until SIGINT or SIGTERM, then calls Stop on the ChannelManager,
// Disconnect on the platform connector, and Shutdown on the HTTP server.
func (s *ShutdownCoordinator) Wait(
    ctx context.Context,
    base *ListenerBase,
    channelMgr ChannelManager,
    conn PlatformConnector, // nil if no platform disconnect needed
)
```

---

## Go Module and Workspace Strategy

**Decision: No `go.work` file. Use existing `replace` directives.**

Rationale from code inspection:

1. Every listener `go.mod` already has: `replace github.com/caesar/all-chat/shared => ../../shared`
2. Adding `shared/listener/*.go` to the existing `shared` module is automatically visible — no `go.mod` or `go.work` changes needed
3. A `go.work` file would enable cross-module tooling (e.g., `go build ./...` from root), but the existing per-service build model (each service's `Dockerfile` runs `go build ./cmd/...`) does not require it
4. Go workspace (`go work`) is useful when multiple modules need simultaneous development. Since the SDK lives in `shared/` (already an existing module), and listeners already depend on it via `replace`, there is no cross-module cycle to solve.

**What must be added to `shared/go.mod`:**

The new `shared/listener/` package imports `github.com/google/uuid` (already in `shared/go.mod` as indirect via `sourcemanager`) and `github.com/redis/go-redis/v9` (already direct). No new dependencies are required.

**If a `go.work` is later desired** (e.g., for monorepo-wide `go vet` in CI), the minimal workspace file would be:

```
// go.work — root of monorepo
go 1.25.6

use (
    ./shared
    ./services/twitch-listener
    ./services/kick-listener
    ./services/youtube-listener
    ./services/youtube-listener-innertube
    ./services/discord-listener
    ./services/message-processor
    // ... all Go services
)
```

This is a future improvement, not a prerequisite for the SDK milestone.

---

## Component Boundaries

| Component | Responsibility | NEW or MODIFIED |
|-----------|---------------|-----------------|
| `shared/listener/base.go` | Startup sequence: jitter, assignment query, JWT refresh, heartbeat loop, assignment refresh loop | NEW |
| `shared/listener/leadership.go` | LeadershipListener: embeds ListenerBase, adds leader election wiring | NEW |
| `shared/listener/channel_manager.go` | ChannelManager interface definition | NEW |
| `shared/listener/shutdown.go` | Graceful shutdown: channel manager stop + platform disconnect + HTTP server shutdown | NEW |
| `shared/coordination/` | CoordinatorClient, MigrationSubscriber, MigrationEvent, Assignment | UNCHANGED |
| `shared/sourcemanager/` | LeadershipCoordinator, Client, LeadershipClient | UNCHANGED |
| `shared/metrics/` | ShardMetrics, ListenerMetrics | UNCHANGED |
| `twitch-listener/channels/manager.go` | Concrete ChannelManager implementation, Twitch-specific IRC join/part logic | MODIFY: add interface compliance, remove duplicated SDK wiring from cmd/main.go |
| `kick-listener/channels/manager.go` | Concrete ChannelManager implementation, Kick-specific Pusher subscribe/unsubscribe | MODIFY: same as above |
| `twitch-listener/cmd/main.go` | Platform setup: IRC connect, Gin routes, health handlers | MODIFY: replace ~120 lines of coordination wiring with ListenerBase + ShutdownCoordinator |
| `kick-listener/cmd/main.go` | Platform setup: Pusher connect, Gin routes, health handlers | MODIFY: same as above |
| `youtube-listener-innertube/cmd/main.go` | Platform setup: InnerTube client, stream manager, Gin routes | MODIFY: replace LeadershipCoordinator wiring with LeadershipListener |
| `discord-listener/cmd/main.go` | Platform setup: Gateway shard, relay worker, Gin routes | MODIFY: use LeadershipListener for shard ownership (already exists as a service) |

---

## Graceful Shutdown Coordination

The existing shutdown pattern (from both Twitch and Kick main.go) is:

```
SIGINT/SIGTERM received
  → channelMgr.Stop()          (drains sync loops, stops DB LISTEN, waits WaitGroup)
  → ircConn.Disconnect() / wsClient.Disconnect()   (closes platform connection)
  → srv.Shutdown(ctx with 10s timeout)             (HTTP server drains in-flight requests)
```

`ShutdownCoordinator.Wait` encapsulates this sequence. The critical ordering constraint is:

1. **Channel manager MUST stop before platform disconnect.** The channel manager's `Stop()` closes `stopChan` and calls `wg.Wait()`, ensuring all sync goroutines have exited before the IRC/WebSocket connection is torn down. If the platform connection is closed first, running goroutines that attempt to `Join`/`Subscribe` channels will panic or log errors.

2. **HTTP server shutdown is always last.** Health probes must remain responsive during the channel manager drain phase (Kubernetes readiness probe may be polled during pod termination). The HTTP server is shut down only after all platform state has been cleaned up.

3. **ListenerBase.Stop() runs concurrently with ChannelManager.Stop().** The background loops (heartbeat, assignment refresh) do not interact with the channel manager and can be stopped in parallel. `ShutdownCoordinator.Wait` stops both concurrently, then waits for both, before closing the platform connection.

For `LeadershipListener`-based services (YouTube InnerTube, Discord), the `LeadershipCoordinator.Stop()` must be called **before** the HTTP server shutdown but **after** platform connections are released. The `LeadershipCoordinator.Stop()` fires `ReleaseLeadership` calls for all active leases via goroutines — these are best-effort (non-blocking), so they do not extend the shutdown deadline.

---

## Data Flow: How SDK Wires Into Platform Message Path

The SDK does not touch the message publishing path. It only manages the lifecycle (startup, assignment, migration, shutdown). Platform-specific code remains responsible for:

```
Platform connection (IRC/WebSocket/HTTP polling)
  → Platform-specific message handler
  → publisher.Publish(ctx, rawMsg)   [platform-specific publisher package]
  → Redis Streams chat:raw
```

The ChannelManager interface methods (`Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`) are all lifecycle and assignment operations. The SDK never calls `Publish` directly.

```
ListenerBase.Start()
  ├─ Jitter
  ├─ QueryAssignments → returns map[string]bool (assignedSourceIDs)
  └─ Returns to caller

Caller (cmd/main.go):
  ├─ Initializes platform connection (IRC/WebSocket)
  ├─ Initializes ChannelManager with assignedSourceIDs
  ├─ Calls channelMgr.Start(ctx)

ListenerBase.Run(ctx, channelMgr.HandleMigrationEvent)
  ├─ goroutine: JWT refresh (12h interval)
  ├─ goroutine: heartbeat (10s interval)
  └─ goroutine: assignment refresh (60s interval)
        └─ on tick: QueryAssignments → channelMgr.UpdateAssignedSourceIDs(newIDs)

ShutdownCoordinator.Wait(ctx, base, channelMgr, conn)
  ├─ waits SIGINT/SIGTERM
  ├─ parallel: base.Stop() + channelMgr.Stop()
  ├─ conn.Disconnect()
  └─ srv.Shutdown(10s timeout)
```

---

## Patterns to Follow

### Pattern 1: Interface-First for ChannelManager

Define `ChannelManager` as an interface in `shared/listener/` with only the methods the SDK needs. Do not define a concrete `BaseChannelManager` struct in the shared package — the Twitch and Kick managers have platform-specific constructor arguments (IRC client, WebSocket client, Pusher publisher) that cannot be generalized without reflection or `interface{}`.

Both existing managers already have matching method signatures. The migration is additive: add `_ listener.ChannelManager = (*channels.Manager)(nil)` compile-time assertion to each platform's `channels/manager.go`.

### Pattern 2: Config Structs Over Long Parameter Lists

The existing `channels.NewManager` in Kick has 9 parameters. Future ChannelManager constructors in new listeners should use a config struct. The SDK should not dictate the constructor signature — only the interface.

### Pattern 3: Nil-Safe LeadershipCoordinator

The `sourcemanager.LeadershipCoordinator` already implements nil-safe methods (`EnsureLeadership` and `Release` both check `c == nil`). `LeadershipListener` should preserve this: if `SOURCE_MANAGER_SECRET` is empty, `LeaderCoord` is nil, and leadership operations are no-ops. This allows local development without a source-manager.

### Pattern 4: CoordinatorClient.NewCoordinatorClient Retains Service Name Detection

The existing `NewCoordinatorClient` in `shared/coordination/client.go` auto-detects the service name from `HOSTNAME` (pod name prefix). When listeners migrate to `ListenerBase`, the `Config.ServiceName` field should override this auto-detection by being passed explicitly. The `NewCoordinatorClient` constructor will need an optional `serviceName` parameter, or `ListenerBase.Start` builds the client with the explicit name.

**Preferred approach:** Add a `serviceName string` parameter to `NewCoordinatorClient` and remove the hostname auto-detection logic (which only covers `twitch-listener`, `twitch-eventsub-listener`, `kick-listener`, `tiktok-listener`). The SDK passes `Config.ServiceName` directly.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Shared `BaseChannelManager` Struct

**What goes wrong:** Trying to provide a concrete `BaseChannelManager` in the shared package requires generics or `interface{}` for the platform connection dependency (IRC client, WebSocket client). This makes the SDK harder to use and test than the existing service-specific managers.

**Instead:** Interface only. Each platform keeps its concrete manager in `services/<listener>/channels/`.

### Anti-Pattern 2: Moving Channel DB Repository to Shared

**What goes wrong:** The Twitch `channels.Repository` and Kick `channels.Repository` query platform-specific tables/columns (`platform = 'twitch'`, Kick chatroom IDs). Sharing a generic repository adds conditional logic that obscures platform intent and makes the shared package aware of platform specifics.

**Instead:** Each service keeps its own `channels/repository.go`. The `ChannelManager` interface abstracts the channel sync behavior, not the database access pattern.

### Anti-Pattern 3: Adding `go.work` Without Understanding Build Impact

**What goes wrong:** `go.work` causes `go mod tidy` in any workspace member to modify other modules' checksums. CI pipelines that run per-service `go mod tidy` break. Docker builds using `COPY go.mod go.sum ./` patterns require the workspace file to be present in the build context.

**Instead:** Use the existing `replace` directives. Introduce `go.work` only if CI needs monorepo-wide `go vet` or `go build ./...`.

### Anti-Pattern 4: Inlining SDK Logic into `cmd/main.go` of New Listeners

**What goes wrong:** Adding new listeners (e.g., a future Twitch EventSub listener) by copy-pasting the startup sequence from existing `main.go` files perpetuates the duplication the SDK is meant to eliminate.

**Instead:** All new Go listener services after v1.6 MUST embed `ListenerBase` or `LeadershipListener` in their `cmd/main.go`.

---

## Build Order: SDK First, Then Per-Listener Migration

The SDK is developed in phases. Each phase is independently deployable. Existing listeners continue to use their current startup wiring until migrated.

### Phase 1 — Define SDK Package (no listener changes)

**Files created:**
- `shared/listener/channel_manager.go` — `ChannelManager` interface + `PlatformConnector` interface
- `shared/listener/base.go` — `ListenerBase` struct + `Config`
- `shared/listener/leadership.go` — `LeadershipListener` struct + `LeadershipConfig`
- `shared/listener/shutdown.go` — `ShutdownCoordinator`

**Files modified:**
- `shared/coordination/client.go` — add explicit `serviceName` parameter to `NewCoordinatorClient`, remove hostname auto-detection

**Dependencies:** None. The `shared` module compiles independently.

**Validation:** `cd shared && go build ./...` passes. Unit tests for `ListenerBase.Start` (mock coordinator) pass.

### Phase 2 — Migrate twitch-listener

**Files modified:**
- `services/twitch-listener/cmd/main.go` — replace coordination wiring with `ListenerBase.Start()`, `ListenerBase.Run()`, `ShutdownCoordinator.Wait()`
- `services/twitch-listener/channels/manager.go` — add `_ listener.ChannelManager = (*Manager)(nil)` compile-time assertion

**Files unchanged:** `channels/repository.go`, `irc/`, `handlers/`, `publisher/`

**Validation:** `cd services/twitch-listener && go build ./...` passes. Integration smoke test: listener starts, queries coordinator, joins channels.

### Phase 3 — Migrate kick-listener

**Files modified:**
- `services/kick-listener/cmd/main.go` — same pattern as twitch migration, plus `LeadershipListener` for leader coordination
- `services/kick-listener/channels/manager.go` — add compile-time assertion

**Files unchanged:** `channels/repository.go`, `websocket/`, `handlers/`, `publisher/`, `metrics/`

**Dependency on Phase 2:** None — kick and twitch migrations are independent. Can run in parallel.

**Validation:** `cd services/kick-listener && go build ./...` passes.

### Phase 4 — Migrate youtube-listener-innertube

**Files modified:**
- `services/youtube-listener-innertube/cmd/main.go` — replace manual `sourcemanager.NewLeadershipCoordinator` wiring with `LeadershipListener`

**Files unchanged:** `streams/`, `innertube/`, `deletion/`, `handlers/`, `publisher/`

**Validation:** `cd services/youtube-listener-innertube && go build ./...` passes.

### Phase 5 — Migrate remaining listeners

**discord-listener:** Uses `LeadershipListener` for shard ownership (already implemented as a service, wires its own `sourcemanager.NewLeadershipCoordinator`). Migration replaces that manual wiring.

**youtube-listener (quota-based):** Uses `ListenerBase` (no leadership required — coordinator assigns streams). Has its own assignment + heartbeat loop in `cmd/main.go`.

**Note:** `tiktok-listener` is Node.js — out of scope for the Go SDK.

---

## Service Interface Contracts (Unchanged by SDK Migration)

The SDK migration is internal to each listener's `cmd/main.go`. The following external interfaces are unaffected:

```
GET  /health/live     → 200 always
GET  /health/ready    → 200 if platform connected + coordinator reachable
GET  /status          → JSON: channel count, assignment count, platform connection state
GET  /metrics         → Prometheus metrics
```

Redis Streams key (`chat:raw`), message schema (`RawChatMessage`), and coordinator HTTP API (`/assignments`, `/heartbeat`) are not modified.

---

## New Files vs. Modified Files Summary

### New Files (all in `shared/listener/`)

| File | Contains |
|------|---------|
| `shared/listener/base.go` | `Config`, `ListenerBase`, `Start()`, `Run()`, `Stop()`, `GetFilteredAssignedSourceIDs()` |
| `shared/listener/leadership.go` | `LeadershipConfig`, `LeadershipListener`, `NewLeadershipListener()` |
| `shared/listener/channel_manager.go` | `ChannelManager` interface, `PlatformConnector` interface |
| `shared/listener/shutdown.go` | `ShutdownConfig`, `ShutdownCoordinator`, `Wait()` |

### Modified Files

| File | Change |
|------|--------|
| `shared/coordination/client.go` | Add explicit `serviceName` parameter, remove hostname prefix auto-detection logic |
| `services/twitch-listener/cmd/main.go` | Replace ~120 lines of coordination wiring with SDK calls |
| `services/twitch-listener/channels/manager.go` | Add compile-time interface assertion |
| `services/kick-listener/cmd/main.go` | Replace coordination + leadership wiring with SDK calls |
| `services/kick-listener/channels/manager.go` | Add compile-time interface assertion |
| `services/youtube-listener-innertube/cmd/main.go` | Replace manual leadership wiring with `LeadershipListener` |
| `services/discord-listener/cmd/main.go` | Replace manual leadership wiring with `LeadershipListener` |
| `services/youtube-listener/cmd/main.go` | Replace assignment loop wiring with `ListenerBase` |

### Unchanged Files

All `channels/repository.go`, `handlers/`, `publisher/`, and platform connection packages (IRC, WebSocket, InnerTube) remain untouched. No database schema changes. No Kubernetes manifests changes. No Redis key changes.

---

## Sources

Research based on direct inspection of:
- `/home/moersener/Hobby/all-chat/services/twitch-listener/cmd/main.go` — full startup sequence
- `/home/moersener/Hobby/all-chat/services/twitch-listener/channels/manager.go` — ChannelManager method set
- `/home/moersener/Hobby/all-chat/services/kick-listener/cmd/main.go` — startup sequence, leadership wiring
- `/home/moersener/Hobby/all-chat/services/kick-listener/channels/manager.go` — ChannelManager method set
- `/home/moersener/Hobby/all-chat/services/youtube-listener-innertube/cmd/main.go` — leadership coordinator pattern
- `/home/moersener/Hobby/all-chat/services/discord-listener/cmd/main.go` — leadership pattern, no coordinator-based assignment
- `/home/moersener/Hobby/all-chat/shared/coordination/client.go` — CoordinatorClient API, hostname auto-detection
- `/home/moersener/Hobby/all-chat/shared/coordination/migration_subscriber.go` — MigrationSubscriber API
- `/home/moersener/Hobby/all-chat/shared/sourcemanager/coordinator.go` — LeadershipCoordinator API
- `/home/moersener/Hobby/all-chat/shared/sourcemanager/client.go` — Client API, LeadershipClient interface
- `/home/moersener/Hobby/all-chat/shared/metrics/shard_metrics.go` — ShardMetrics (unchanged)
- `/home/moersener/Hobby/all-chat/services/twitch-listener/go.mod` — replace directive pattern
- `/home/moersener/Hobby/all-chat/services/kick-listener/go.mod` — replace directive pattern
- `/home/moersener/Hobby/all-chat/services/discord-listener/go.mod` — replace directive pattern
- `/home/moersener/Hobby/all-chat/shared/go.mod` — existing shared module dependencies
- `/home/moersener/Hobby/all-chat/.planning/PROJECT.md` — milestone requirements and constraints
