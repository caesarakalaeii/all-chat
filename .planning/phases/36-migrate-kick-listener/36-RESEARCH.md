# Phase 36: Migrate kick-listener - Research

**Researched:** 2026-03-17
**Domain:** Go service migration — wiring shared/listener SDK (ListenerBase + LeadershipListener) into kick-listener cmd/main.go
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGRATE-02 | kick-listener `cmd/main.go` migrated to use `ListenerBase` + `LeadershipListener` — both assignment and leadership archetypes exercised via SDK | Both archetypes verified in source: ListenerBase in shared/listener/base.go, LeadershipListener in shared/listener/leadership.go. NewLeadershipListenerFromEnv constructs both. Goroutines to remove identified: heartbeat, assignment refresh, migration subscriber, JWT refresh, manual LeadershipCoordinator construction, SourceManagerClient construction (6 blocks). |
</phase_requirements>

---

## Summary

Phase 36 is a targeted refactor of `services/kick-listener/cmd/main.go`. The shared SDK (`shared/listener`) was built in Phase 34 and proven in Phase 35 (twitch-listener migration). Kick-listener differs from twitch-listener in one key way: it uses `LeadershipListener` (which embeds `ListenerBase`), because kick-listener requires per-stream ownership coordination via `sourcemanager.LeadershipCoordinator`. The `LeadershipListener` is the second SDK archetype — after Phase 36, both archetypes are validated.

The migration removes: four inline goroutine blocks (heartbeat, assignment refresh, migration subscriber, JWT refresh), plus the manual `sourcemanager.NewLeadershipCoordinator` + `sourcemanager.NewClient` + `sourcemanager.NewSigningTokenSource` construction block in main.go. These are replaced by `NewListenerBase` + `NewLeadershipListenerFromEnv` + `base.Start(ctx, channelMgr)` + `listener.ShutdownCoordinator`. A compile-time interface assertion is added to `channels/manager.go`. A goleak smoke test in `cmd/main_sdk_test.go` is added following the exact pattern established in Phase 35.

The kick-listener `channels.Manager` already satisfies all 7 `listener.ChannelManager` interface methods. It holds a `*sourcemanager.LeadershipCoordinator` and calls `m.leader.EnsureLeadership` / `m.leader.Release` / `m.leader.Stop` internally. After migration, `channelMgr` receives the coordinator via `l.LeadershipCoordinator()` — one method call to retrieve the nil-safe pointer.

**Primary recommendation:** Migrate in two plans: Plan 01 adds the compile-time assertion and goleak direct dep (Wave 0 prerequisites). Plan 02 rewrites cmd/main.go using `NewLeadershipListenerFromEnv` and removes all inline wiring, adding the goleak smoke test.

---

## Standard Stack

### Core (already available via shared replace directive)

| Library | Purpose | Location |
|---------|---------|----------|
| `shared/listener.ListenerBase` | Manages heartbeat, assignment refresh, migration subscriber goroutines | `shared/listener/base.go` |
| `shared/listener.LeadershipListener` | Embeds ListenerBase; constructs `LeadershipCoordinator` + `SourceManagerClient` from env | `shared/listener/leadership.go` |
| `shared/listener.NewLeadershipListenerFromEnv` | Factory: reads `SOURCE_MANAGER_SECRET`/`SOURCE_MANAGER_URL`, wires coordinator; nil-safe when secret absent | `shared/listener/leadership.go` line 25 |
| `shared/listener.ListenerConfig` | Config struct with defaults; `DefaultConfig()` = 30s heartbeat, 10s assignment refresh, 30s jitter | `shared/listener/config.go` |
| `shared/listener.Env` | Drop-in for `getEnvOrDefault` — same signature | `shared/listener/config.go` line 58 |
| `shared/listener.ShutdownCoordinator` | Ordered shutdown: parallel channelMgr.Stop + base.Stop → platformDisconnect → 10s HTTP drain | `shared/listener/shutdown.go` |
| `shared/listener.ChannelManager` | 7-method interface that channels.Manager already satisfies | `shared/listener/channel_manager.go` |
| `shared/listener/testutil.MockCoordinator` | In-memory mock coordinator for smoke test | `shared/listener/testutil/mock_coordinator.go` |
| `go.uber.org/goleak v1.3.0` | Goroutine leak detection in smoke test | In go.sum (transitive); must be added as direct dep to kick-listener go.mod |

**Note on LeadershipListener:** `LeadershipListener` embeds `*ListenerBase` and exposes `l.LeadershipCoordinator() *sourcemanager.LeadershipCoordinator`. The base's `Start`, `Stop`, and all lifecycle methods are promoted to `LeadershipListener` — the planner calls `base.Start(ctx, channelMgr)` where `base` is `l.ListenerBase` (the embedded field), or alternatively uses the promoted method `l.Start(ctx, channelMgr)` directly since Go promotes embedded struct methods. Using the promoted method is cleaner.

### kick-listener-specific go.mod state

- `go.uber.org/goleak v1.3.0` is in `go.sum` (transitive via shared) but NOT as a direct dep in `go.mod`
- `github.com/caesar/all-chat/shared` is already a direct dep via `replace ../../shared`
- No new module additions needed beyond goleak direct dep

**Installation — one addition needed:**
```bash
cd services/kick-listener && go get go.uber.org/goleak@v1.3.0
```

---

## Architecture Patterns

### How kick-listener differs from twitch-listener

| Aspect | twitch-listener (Phase 35) | kick-listener (Phase 36) |
|--------|---------------------------|--------------------------|
| SDK archetype | `ListenerBase` only | `LeadershipListener` (embeds `ListenerBase`) |
| Leadership coordinator | Not used (leaderCoord = nil) | Required: `sourcemanager.LeadershipCoordinator` |
| Platform disconnect | `ircConn.Disconnect()` (error-returning) | `wsClient.Disconnect()` (error-returning) |
| Pre-connect sleep | 2s (IRC stabilization) | 2s (WebSocket stabilization) |
| Source ID type | string (UUID) | int (chatroomID) — but `assignedSourceIDs map[string]bool` uses string keys |

### String-keyed Channel Manager Convention

kick-listener uses **integer chatroom IDs** internally but the coordinator assigns by **string source UUIDs**. The `assignedSourceIDs map[string]bool` field uses string-keyed source UUIDs from the coordinator. The `chatroomIndex map[int]*trackedChannel` uses integer keys for message routing. These are separate concerns. The compile-time assertion tests the `ChannelManager` interface, which uses `map[string]bool` for `UpdateAssignedSourceIDs` — this is already the signature in `channels.Manager`. The `strconv.Itoa(chatroomID)` pattern is NOT used for `assignedSourceIDs` — source IDs are UUIDs from the coordinator.

The existing source ID stripping (`:kick` suffix) already happens in main.go before passing to `channels.NewManager`. After migration, the SDK's `startAssignmentRefreshLoop` calls `mgr.UpdateAssignedSourceIDs(ids)` where `ids` uses raw `a.SourceID` values — same as the existing assignment refresh goroutine in main.go. The pre-Phase-33 suffix stripping still needs to happen. Since the SDK does NOT strip, the same scenario as twitch-listener applies: if the coordinator returns composite IDs like `uuid:kick`, filtering degrades to the safety fallback (all channels processed). The `ENABLE_COORDINATOR_FILTERING` env var defaults to `"false"` in kick-listener too — so this only matters when explicitly enabled.

### SDK Constructor Call Chain

The migrated `main.go` follows this sequence:

```
1. Initialize logger, tracing (unchanged)
2. DB + Redis connection (unchanged) — use listener.Env instead of getEnvOrDefault
3. Metrics initialization: shardMetrics (unchanged)
4. podName from HOSTNAME (unchanged — still needed as podID arg)
5. Read ENABLE_COORDINATOR_FILTERING, build ListenerConfig with DisableCoordinatorFiltering
6. Create coordinator client: coordination.NewCoordinatorClient(url, secret, "kick-listener", log)
7. base := listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)
8. l, err := listener.NewLeadershipListenerFromEnv(base, "kick", log)  — reads SOURCE_MANAGER_SECRET/URL
9. Configure Pusher: read KICK_PUSHER_APP_KEY, build wsConfig (unchanged)
10. Initialize components: streamPublisher, channelRepo, dbConnWrapper (unchanged)
11. Create messageHandler closure (before channelMgr exists — Kick pattern)
12. Create wsClient with messageHandler; set deletion handler (unchanged)
13. channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, dbConnWrapper,
        l.LeadershipCoordinator(), nil, redisClient, podName, log)
    — pass l.LeadershipCoordinator() instead of manually-built leaderCoord
    — pass nil for assignedSourceIDs (SDK owns via UpdateAssignedSourceIDs inside base.Start)
14. wsClient.Connect() — fatal on error
15. time.Sleep(2 * time.Second) — WebSocket stabilization; stays in main.go
16. l.Start(ctx, channelMgr) — queries assignments, calls mgr.UpdateAssignedSourceIDs, calls mgr.Start,
    launches 3 goroutines, starts JWT refresh
17. Record shardMetrics.PodChannelCount (service-specific, stays in main.go)
18. Start handleReconnections goroutine (service-specific, stays in main.go)
19. HTTP router, handlers, metrics endpoint (unchanged)
20. HTTP server goroutine (unchanged)
21. Wait for SIGINT/SIGTERM
22. listener.ShutdownCoordinator(base, channelMgr,
        func() { _ = wsClient.Disconnect() }, srv, log)
    — NOTE: base is the embedded *ListenerBase from LeadershipListener (l.ListenerBase)
```

### Pattern 1: LeadershipListener Construction

```go
// Source: shared/listener/leadership.go line 25 (verified)
cfg := listener.DefaultConfig()
cfg.DisableCoordinatorFiltering = !enableFiltering

coordClient := coordination.NewCoordinatorClient(
    listener.Env("COORDINATOR_URL", "http://source-manager:8088"),
    listener.Env("SERVICE_JWT_SECRET", "dev-service-secret"),
    "kick-listener",
    log,
)

base := listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)

// NewLeadershipListenerFromEnv reads SOURCE_MANAGER_SECRET/SOURCE_MANAGER_URL from env
// When SOURCE_MANAGER_SECRET is absent, coordinator is nil (nil-safe)
l, err := listener.NewLeadershipListenerFromEnv(base, "kick", log)
if err != nil {
    log.Fatal("Failed to initialize leadership listener", zap.Error(err))
}
```

### Pattern 2: Passing coordinator to channels.NewManager

```go
// Source: channels/manager.go NewManager signature + leadership.go LeadershipCoordinator() method
// l.LeadershipCoordinator() returns nil when SOURCE_MANAGER_SECRET is absent — nil-safe
channelMgr = channels.NewManager(
    channelRepo,
    wsClient,
    streamPublisher,
    dbConnWrapper,
    l.LeadershipCoordinator(),  // replaces manually-built leaderCoord
    nil,                         // assignedSourceIDs: nil — SDK owns via UpdateAssignedSourceIDs
    redisClient,
    podName,
    log,
)
```

### Pattern 3: Start and ShutdownCoordinator

```go
// Source: shared/listener/base.go Start signature (promoted to LeadershipListener)
if err := l.Start(ctx, channelMgr); err != nil {
    log.Fatal("Failed to start listener", zap.Error(err))
}

// ...after signal received...
// Source: shared/listener/shutdown.go line 17 (verified)
// ShutdownCoordinator takes *ListenerBase — pass l.ListenerBase (embedded field)
listener.ShutdownCoordinator(l.ListenerBase, channelMgr,
    func() { _ = wsClient.Disconnect() },
    srv,
    log,
)
```

**Critical note:** `ShutdownCoordinator` takes `*ListenerBase`, not `*LeadershipListener`. Pass `l.ListenerBase` (the embedded pointer).

### Pattern 4: Compile-time assertion in channels/manager.go

```go
// Source: VERIFY-02 requirement + channel_manager.go interface definition
// Add to channels/manager.go after package declaration and imports
var _ listener.ChannelManager = (*Manager)(nil)
```

Required import: `"github.com/caesar/all-chat/shared/listener"`

### Pattern 5: Goleak smoke test (identical structure to Phase 35)

```go
// File: services/kick-listener/cmd/main_sdk_test.go
// Source: Phase 35 pattern from services/twitch-listener/cmd/main_sdk_test.go
package main

import (
    "context"
    "testing"
    "time"

    "github.com/caesar/all-chat/shared/coordination"
    "github.com/caesar/all-chat/shared/listener"
    "github.com/caesar/all-chat/shared/listener/testutil"
    "go.uber.org/goleak"
    "go.uber.org/zap"
)

type mockChannelManagerForTest struct{}

func (m *mockChannelManagerForTest) Start(_ context.Context) error          { return nil }
func (m *mockChannelManagerForTest) Stop()                                  {}
func (m *mockChannelManagerForTest) HandleMigrationEvent(_ *coordination.MigrationEvent) error {
    return nil
}
func (m *mockChannelManagerForTest) UpdateAssignedSourceIDs(_ map[string]bool) {}
func (m *mockChannelManagerForTest) GetFilteredAssignmentCount() int        { return 0 }
func (m *mockChannelManagerForTest) GetActiveChannels() []string            { return nil }
func (m *mockChannelManagerForTest) GetActiveChannelCount() int             { return 0 }

func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    mock := &testutil.MockCoordinator{}
    cfg := listener.ListenerConfig{
        HeartbeatInterval:         20 * time.Millisecond,
        AssignmentRefreshInterval: 20 * time.Millisecond,
        StartupJitterMax:          0,
    }
    base := listener.NewListenerBase(cfg, mock, nil, "test-pod", zap.NewNop())
    mgr := &mockChannelManagerForTest{}

    ctx, cancel := context.WithCancel(context.Background())
    if err := base.Start(ctx, mgr); err != nil {
        t.Fatal(err)
    }
    cancel()
    base.Stop()
}
```

### Anti-Patterns to Avoid

- **Leaving the manual leaderCoord construction in main.go**: The three lines building `tokenSource`, `smClient`, and `leaderCoord` must be deleted — `NewLeadershipListenerFromEnv` owns this.
- **Passing leaderCoord (local variable) to channels.NewManager**: After migration, pass `l.LeadershipCoordinator()` instead.
- **Passing ShutdownCoordinator a LeadershipListener**: `ShutdownCoordinator` requires `*ListenerBase`. Pass `l.ListenerBase` (the embedded field), not `l` itself.
- **Passing non-nil assignedSourceIDs to channels.NewManager**: Pass nil — SDK populates via `UpdateAssignedSourceIDs` inside `l.Start`.
- **Calling coordClient.StartJWTRefresh/StopJWTRefresh in main.go**: SDK owns these — double-call causes panic.
- **Keeping the SOURCE_MANAGER_SECRET env read + nil-check in main.go**: `NewLeadershipListenerFromEnv` handles this — the Warn log on absent secret is inside the SDK.
- **wsClient.Disconnect returns error**: The `platformDisconnect` param is `func()` — wrap with an anonymous function that discards the error.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Heartbeat loop | Ticker goroutine in main.go | `ListenerBase` (internal `startHeartbeatLoop`) | SDK has exponential backoff (1s→30s cap), OnFatalError hook |
| Assignment refresh loop | Ticker goroutine in main.go | `ListenerBase` (internal `startAssignmentRefreshLoop`) | SDK handles DisableCoordinatorFiltering, backoff |
| Migration subscriber | Goroutine + `coordination.NewMigrationSubscriber` in main.go | `ListenerBase` (internal `startMigrationSubscriberLoop`) | SDK handles nil redis guard, retry loop |
| JWT refresh lifecycle | `coordClient.StartJWTRefresh` / `StopJWTRefresh` in main.go | `ListenerBase.Start` / `ListenerBase.Stop` | Prevents double-start on migration |
| LeadershipCoordinator construction | Manual `NewSigningTokenSource` + `NewClient` + `NewLeadershipCoordinator` | `NewLeadershipListenerFromEnv` | Nil-safe when secret absent, matches production kick pattern |
| Ordered shutdown | Manual sequence in main.go | `listener.ShutdownCoordinator` | Handles parallel stop, platform disconnect ordering, 10s HTTP drain |

**Key insight:** kick-listener replaces everything from "coordinator construction" downward in the old main.go with three calls: `NewListenerBase`, `NewLeadershipListenerFromEnv`, `l.Start`. Kick-specific logic (Pusher config, WebSocket setup, messageHandler closure, handleReconnections goroutine) stays in main.go unchanged.

---

## What Gets Removed from kick-listener main.go

**Goroutine blocks removed (SDK owns):**
- `coordClient.StartJWTRefresh(ctx)` + `defer coordClient.StopJWTRefresh()` — lines 112-113
- Startup jitter (`rand.Intn(30)` + `time.Sleep(jitter)`) — lines 116-120
- `coordClient.QueryAssignments(ctx, podName)` + `assignedSourceIDs` map construction — lines 123-142
- `channelMgr.Start(ctx)` direct call — line 209 (SDK calls it inside l.Start)
- `migrationSub` goroutine block — lines 222-231
- Heartbeat goroutine block — lines 233-248
- Assignment refresh goroutine block — lines 250-285

**Leadership construction block removed (SDK owns):**
- `sourceManagerURL` + `sourceManagerSecret` reads — lines 158-159
- `var leaderCoord *sourcemanager.LeadershipCoordinator` — line 160
- nil check + `tokenSource` + `smClient` + `leaderCoord` construction — lines 161-170

**Other removals:**
- `getEnvOrDefault` function at bottom of file — replaced by `listener.Env`
- `math/rand` import — no longer needed after removing jitter
- `strings` import — no longer needed (suffix stripping was in goroutine blocks)

**What stays in main.go after migration:**
- Logger, tracing initialization
- DB + Redis connection
- shardMetrics + podName
- SERVICE_JWT_SECRET read (needed for coordClient)
- ENABLE_COORDINATOR_FILTERING + ListenerConfig + coordClient + NewListenerBase + NewLeadershipListenerFromEnv
- Pusher config (KICK_PUSHER_APP_KEY, KICK_PUSHER_CLUSTER, KICK_PUSHER_CLUSTER_FALLBACKS)
- wsConfig, messageHandler closure, wsClient creation, deletion handler wiring
- channelMgr = channels.NewManager(...) with l.LeadershipCoordinator() and nil assignedSourceIDs
- wsClient.Connect() + time.Sleep(2s) — WebSocket-specific
- shardMetrics.PodChannelCount recording after l.Start
- handleReconnections goroutine (Kick-specific reconnect logic)
- HTTP router, handlers, metrics, server goroutine
- dbConnWrapper struct, buildClusterList, parseClusterList, pickKickUsername, buildKickTags, formatKickBadges helpers
- handleChatMessage, handleDeletionEvent, handleReconnections functions

---

## Common Pitfalls

### Pitfall 1: ShutdownCoordinator Receives Wrong Type

**What goes wrong:** `ShutdownCoordinator(l, channelMgr, ...)` fails to compile — `l` is `*LeadershipListener`, not `*ListenerBase`. The function signature requires `*ListenerBase`.

**How to avoid:** Pass `l.ListenerBase` (the embedded field, accessible directly), not `l`.

**Example:**
```go
listener.ShutdownCoordinator(l.ListenerBase, channelMgr, func() { _ = wsClient.Disconnect() }, srv, log)
```

### Pitfall 2: channelMgr Created Before wsClient in Original Code

**What goes wrong:** In the original main.go, `channelMgr` is created AFTER `wsClient` because of the circular dependency (messageHandler needs channelMgr, channelMgr needs wsClient). The migration preserves this sequencing: `messageHandler` closure + `wsClient` creation before `channelMgr`. Do not reorder.

**How to avoid:** Follow the exact sequence from the "SDK Constructor Call Chain" above — wsClient is created before channelMgr.

### Pitfall 3: Source ID Stripping Missing from SDK

**What goes wrong:** Same as Phase 35 (inherited). `ListenerBase.startAssignmentRefreshLoop` uses `a.SourceID` directly without stripping `":kick"` suffix. If coordinator returns composite IDs, filtering degrades to safety fallback (all channels processed).

**Impact:** Low — `ENABLE_COORDINATOR_FILTERING` defaults to `"false"` in kick-listener's main.go, so filtering is disabled by default. Only affects explicit opt-in.

**Warning signs:** Coverage verification log entries at ERROR level. Check with `ENABLE_COORDINATOR_FILTERING=true`.

### Pitfall 4: Double JWT Refresh

**What goes wrong:** If `coordClient.StartJWTRefresh(ctx)` and `defer coordClient.StopJWTRefresh()` are left in main.go AND the SDK calls them in `l.Start` / `l.Stop`, double-start / double-close panic occurs.

**How to avoid:** Remove both lines (currently at kick-listener main.go lines 112-113).

### Pitfall 5: goleak Not a Direct Dependency

**What goes wrong:** go.sum has goleak (transitive via shared) but go.mod does not list it directly. `go test ./cmd/...` fails with "cannot find module providing package go.uber.org/goleak".

**How to avoid:** Run `go get go.uber.org/goleak@v1.3.0` in services/kick-listener before writing the test (Wave 0 prerequisite).

### Pitfall 6: LeadershipCoordinator.Stop Called Twice

**What goes wrong:** `channels.Manager.Stop()` calls `m.leader.Stop()` (verified at manager.go line 178-180). `ShutdownCoordinator` calls `mgr.Stop()` and `base.Stop()` in parallel. The LeadershipCoordinator lives inside channels.Manager — its Stop is called by channelMgr.Stop(). This is fine: the coordinator is not separately stopped by the SDK. The old main.go also calls leaderCoord.Stop() indirectly through channelMgr.Stop(). No double-stop risk here.

### Pitfall 7: handleReconnections Goroutine Must Stay

**What goes wrong:** `handleReconnections` is kick-specific reconnect logic — it re-subscribes channels after WebSocket disconnection. This is NOT equivalent to any SDK goroutine. It must remain in main.go.

**How to avoid:** Only remove the 4 SDK-owned goroutine blocks (heartbeat, assignment refresh, migration subscriber, JWT refresh). The handleReconnections goroutine is service-specific and stays.

---

## Code Examples

Verified from codebase inspection:

### NewLeadershipListenerFromEnv signature
```go
// Source: shared/listener/leadership.go line 25 (verified)
func NewLeadershipListenerFromEnv(
    base *ListenerBase,
    platform string,       // "kick" — used as platform in NewLeadershipCoordinator
    logger *zap.Logger,
) (*LeadershipListener, error)
// Returns error only from sourcemanager.NewClient (network config validation)
// Returns nil coordinator (not nil *LeadershipListener) when SOURCE_MANAGER_SECRET is absent
```

### LeadershipCoordinator accessor
```go
// Source: shared/listener/leadership.go line 51 (verified)
func (l *LeadershipListener) LeadershipCoordinator() *sourcemanager.LeadershipCoordinator
// Returns nil when SOURCE_MANAGER_SECRET was absent — all LeadershipCoordinator methods are nil-safe
// channels.Manager.syncChannels already nil-checks: if m.leader != nil { ... }
```

### channels.Manager ChannelManager interface compliance (7 methods — verified)
```go
// Source: services/kick-listener/channels/manager.go (all 7 methods verified present)
func (m *Manager) Start(_ context.Context) error          // line 144
func (m *Manager) Stop()                                  // line 174
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) error  // line 676
func (m *Manager) UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)          // line 944
func (m *Manager) GetFilteredAssignmentCount() int        // line 919
func (m *Manager) GetActiveChannels() []string            // line 926
func (m *Manager) GetActiveChannelCount() int             // line 937
// No compile-time assertion yet — that is Wave 0 for Phase 36
```

### channels.NewManager signature (verified)
```go
// Source: services/kick-listener/channels/manager.go line 86 (verified)
func NewManager(
    repo *Repository,
    wsClient WebSocketClient,
    publisher *publisher.StreamPublisher,
    dbConn DBConnInterface,
    leader *sourcemanager.LeadershipCoordinator,  // nil-safe — used in syncChannels nil-check
    assignedSourceIDs map[string]bool,             // pass nil — SDK populates
    redisClient *redis.Client,
    podID string,
    logger *zap.Logger,
) *Manager
```

### Compile-time assertion (channels/manager.go)
```go
// Add after package declaration and imports
// New import required: "github.com/caesar/all-chat/shared/listener"
var _ listener.ChannelManager = (*Manager)(nil)
```

### ENABLE_COORDINATOR_FILTERING pattern (from twitch-listener migration)
```go
// Source: Phase 35 established pattern; kick-listener main.go currently has no filtering flag
// Check kick-listener main.go — it does NOT currently read ENABLE_COORDINATOR_FILTERING
// IMPORTANT: Do NOT add this flag if kick-listener does not have it — keep behavior identical
```

**Note:** The current kick-listener main.go does NOT read `ENABLE_COORDINATOR_FILTERING`. Unlike twitch-listener, kick-listener has no rollback knob. The SDK default `DisableCoordinatorFiltering = false` means filtering IS active (assignments are queried). The planner must decide: either add `ENABLE_COORDINATOR_FILTERING` for consistency, or pass `cfg.DisableCoordinatorFiltering = false` (default) and skip the env read. The simplest correct approach: use `listener.DefaultConfig()` without setting `DisableCoordinatorFiltering` — coordinator filtering will be active (assignments used). This matches the intent of kick-listener's existing code which always queries assignments.

---

## State of the Art

| Old Approach | Current Approach (after Phase 36) | Impact |
|--------------|-----------------------------------|--------|
| Manual `sourcemanager.NewSigningTokenSource` + `NewClient` + `NewLeadershipCoordinator` in main.go | `listener.NewLeadershipListenerFromEnv(base, "kick", log)` | One call replaces 6-10 lines + nil-check boilerplate |
| Inline heartbeat goroutine in main.go | Encapsulated in ListenerBase | Exponential backoff, OnFatalError hook |
| Inline assignment refresh goroutine in main.go | Encapsulated in ListenerBase | Handles DisableCoordinatorFiltering, backoff |
| Inline migration subscriber goroutine in main.go | Encapsulated in ListenerBase | nil redis guard, retry loop |
| Manual JWT refresh in main.go | SDK-owned in ListenerBase.Start/Stop | Cannot double-start |
| Manual shutdown sequence in main.go | ShutdownCoordinator handles ordering | Parallel stop + 10s drain guaranteed |
| getEnvOrDefault local function | listener.Env helper | Eliminated copy-paste boilerplate |
| No compile-time assertion | `var _ listener.ChannelManager = (*Manager)(nil)` | Build fails immediately if interface drifts |

**Deprecated patterns removed in this phase:**
- Manual `LeadershipCoordinator` construction in main.go (kick-listener specific)
- Source ID suffix stripping in the assignment refresh goroutine (SDK does not strip — but ENABLE_COORDINATOR_FILTERING is off by default in kick so filtering behavior is preserved)

---

## Open Questions

1. **ENABLE_COORDINATOR_FILTERING flag**
   - What we know: twitch-listener main.go reads this flag and sets `DisableCoordinatorFiltering`. kick-listener main.go does NOT read it (always queries assignments).
   - What's unclear: Should Phase 36 add the flag to kick-listener for operational consistency, or keep kick-listener without the rollback knob?
   - Recommendation: Use `listener.DefaultConfig()` without `DisableCoordinatorFiltering` override. kick-listener always had coordinator filtering active and there's no documented need for a rollback knob. The planner should add a comment in main.go noting that filtering is always active (no env override). If the project later needs the knob, it can be added.

2. **Source ID stripping scope**
   - What we know: kick-listener main.go strips `":kick"` suffix at lines 137-141 (in the startup assignment query) and lines 269-273 (in the assignment refresh goroutine). Both of these blocks are removed by the migration.
   - What's unclear: After migration, the SDK's `startAssignmentRefreshLoop` does NOT strip the suffix. The initial assignment query in `base.Start` also does not strip.
   - Recommendation: Since `DisableCoordinatorFiltering = false` (default), the SDK WILL use assignments for filtering via `UpdateAssignedSourceIDs`. If coordinator returns `uuid:kick` and channels.Manager compares against bare UUIDs, filtering will match nothing and all channels will be processed (the SDK sets `assignedIDs[a.SourceID] = true` — composite key won't match bare UUID from DB). The existing `TestManager_SourceIDNormalization` test covers the strip-at-intake convention. The planner should verify: does `channels.syncChannels` check `m.assignedSourceIDs[ch.SourceID]` where `ch.SourceID` is a bare UUID? If yes, and coordinator returns composite IDs, filtering will be broken. This may require adding a stripping step inside `channels.Manager.UpdateAssignedSourceIDs`, or keeping the SDK's assignment storage in a way that handles this. The plan should include a verification step checking what the coordinator actually returns for Kick sources.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (indirect via shared) + goleak v1.3.0 |
| Config file | none — standard `go test` |
| Quick run command | `cd services/kick-listener && go test ./... -count=1` |
| Full suite command | `cd services/kick-listener && go test ./... -race -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIGRATE-02 | main.go uses ListenerBase + LeadershipListener — no inline goroutines | build + goleak smoke | `go build ./cmd/... && go test ./cmd/... -count=1` | ❌ Wave 0: `cmd/main_sdk_test.go` |
| MIGRATE-02 | Existing channel manager tests pass (SourceIDNormalization) | unit | `go test ./channels/... -count=1` | ✅ existing |
| MIGRATE-02 | Compile-time assertion present and build succeeds | build | `go build ./...` in kick-listener | ❌ Wave 0: assertion line in channels/manager.go |

### Sampling Rate
- **Per task commit:** `cd services/kick-listener && go build ./... && go test ./... -count=1`
- **Per wave merge:** `cd services/kick-listener && go test ./... -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/kick-listener/channels/manager.go` — add `var _ listener.ChannelManager = (*Manager)(nil)` compile-time assertion
- [ ] `services/kick-listener/go.mod` — add `go.uber.org/goleak@v1.3.0` as direct dep (`go get go.uber.org/goleak@v1.3.0`)
- [ ] `services/kick-listener/cmd/main_sdk_test.go` — goroutine leak smoke test covering MIGRATE-02

---

## Sources

### Primary (HIGH confidence)
- `/home/moersener/Hobby/all-chat/shared/listener/base.go` — NewListenerBase, Start, Stop, goroutine internals verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/leadership.go` — NewLeadershipListenerFromEnv, LeadershipCoordinator() accessor, embedding pattern verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/config.go` — ListenerConfig fields, DefaultConfig, Env helper verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/shutdown.go` — ShutdownCoordinator signature (*ListenerBase, not *LeadershipListener) verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/channel_manager.go` — 7-method interface definition verified directly
- `/home/moersener/Hobby/all-chat/services/kick-listener/cmd/main.go` — all goroutine blocks, leadership construction, shutdown sequence inventoried directly (684 lines)
- `/home/moersener/Hobby/all-chat/services/kick-listener/channels/manager.go` — all 7 interface methods confirmed present, no compile-time assertion yet (verified lines 144, 174, 676, 919, 926, 937, 944)
- `/home/moersener/Hobby/all-chat/services/kick-listener/channels/manager_test.go` — existing test (SourceIDNormalization) confirmed; no other tests
- `/home/moersener/Hobby/all-chat/services/kick-listener/go.mod` — replace directive confirmed, goleak absent as direct dep, in go.sum as transitive
- `/home/moersener/Hobby/all-chat/.planning/phases/35-migrate-twitch-listener/35-02-SUMMARY.md` — established patterns, deviations, and decisions documented

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` — key decisions re: LeadershipListener archetype for Kick
- `.planning/REQUIREMENTS.md` — MIGRATE-02 requirement text, string-keyed convention noted

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all SDK APIs verified by reading source files directly; leadership.go fully read
- Architecture: HIGH — migration sequence derived from Phase 35 established pattern + kick-listener source inspection; LeadershipListener embedding mechanics verified
- Pitfalls: HIGH (except source ID stripping open question: MEDIUM) — most pitfalls verified against actual code; stripping gap is a logic inference about coordinator return format

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable internal codebase; only at risk from changes to shared/listener or sourcemanager package)
