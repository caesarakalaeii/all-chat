# Phase 35: Migrate twitch-listener - Research

**Researched:** 2026-03-17
**Domain:** Go service migration — wiring shared/listener SDK into twitch-listener cmd/main.go
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**IRC startup sequencing**
- IRC-specific setup (`ircConn.Connect()`, `ircConn.SetFirstMessageChan(channelMgr.GetFirstMessageChan())`, and the `time.Sleep(2 * time.Second)`) happens explicitly in `main.go` **before** calling `base.Start()`
- The 2-second sleep stays in main.go — it is IRC-specific behavior needed to let the connection establish before channels are joined
- `base.Start()` receives an already-connected and wired channel manager; SDK is not involved in IRC connection setup
- Shutdown uses `ShutdownCoordinator` from the SDK: `ircConn.Disconnect` is passed as the platform disconnect callback, SDK handles ordering (channelMgr.Stop + base.Stop in parallel → platform disconnect → HTTP drain)

**ENABLE_COORDINATOR_FILTERING flag**
- Env check stays in `main.go`: reads `ENABLE_COORDINATOR_FILTERING`, sets `ListenerConfig{DisableCoordinatorFiltering: !enableFiltering}`
- `channels/manager.go` is not modified beyond adding the compile-time assertion — all filtering logic stays exactly as-is
- This preserves the operational rollback knob and guarantees existing unit tests pass without modification

**JWT refresh lifecycle**
- JWT refresh moves fully inside the SDK: `ListenerBase.Start()` calls `coordClient.StartJWTRefresh()`, `ListenerBase.Stop()` calls `StopJWTRefresh()`
- `main.go` does not create or interact with the coordinator client directly after migration

**Heartbeat interval**
- Adopt the SDK default of 30 seconds — no override in `ListenerConfig`
- The prior 10-second hardcoded value in the old goroutine was arbitrary; 30s is the correct production value

**Test scope**
- Existing unit tests pass without modification (no changes to channels/manager.go logic)
- One new integration smoke test added in `cmd/` (or `cmd/main_sdk_test.go`): constructs `ListenerBase` with `testutil` mock coordinator, calls `Start()`, then `Stop()`, verifies `goleak.VerifyNone` passes — confirms no goroutine leaks from the SDK wiring
- No additional tests beyond the smoke test

### Claude's Discretion
- Exact position of `service-name` string passed to `listener.Env()` or `NewCoordinatorClient` (use `"twitch-listener"`)
- Whether tracing middleware wiring and HTTP server setup remain in `main.go` (yes — service-specific)
- Package-level doc comment for the migrated `main.go`

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGRATE-01 | twitch-listener `cmd/main.go` migrated to use `ListenerBase` — startup wiring reduced to service-specific IRC connection and message publishing only | SDK API fully verified: NewListenerBase, Start, Stop, DefaultConfig, ShutdownCoordinator all confirmed in shared/listener. Goroutines to remove identified: heartbeat, assignment refresh, migration subscriber, JWT refresh (4 goroutine blocks). |
| VERIFY-02 | Each migrated listener has a compile-time interface assertion (`var _ listener.ChannelManager = (*channels.Manager)(nil)`) in its `channels/manager.go` file | channels.Manager already satisfies all 7 ChannelManager methods — assertion is a one-line addition with no logic change. |
</phase_requirements>

---

## Summary

Phase 35 is a targeted refactor of `services/twitch-listener/cmd/main.go`. The shared SDK (`shared/listener`) was fully built in Phase 34 and is available via the existing `replace ../shared` directive in go.mod — no module changes required. The migration removes four inline goroutine blocks (heartbeat, assignment refresh, migration subscriber, JWT refresh) and replaces them with calls to `NewListenerBase`, `base.Start(ctx, channelMgr)`, and `listener.ShutdownCoordinator`. A single compile-time interface assertion is added to `channels/manager.go`.

The main complexity is sequencing: IRC connection setup (`ircConn.Connect`, `SetFirstMessageChan`, 2-second sleep) must happen BEFORE `base.Start()` because the SDK immediately calls `mgr.Start()` internally, which begins channel joins. Shutdown uses `ShutdownCoordinator` which runs `channelMgr.Stop` + `base.Stop` in parallel before calling `ircConn.Disconnect`, then draining the HTTP server.

**Primary recommendation:** Migrate in a single plan: (1) add compile-time assertion to channels/manager.go, (2) rewrite cmd/main.go using SDK wiring, (3) add goleak smoke test in cmd/main_sdk_test.go. One go.mod change is needed to add `go.uber.org/goleak` as a direct test dependency in twitch-listener.

---

## Standard Stack

### Core (already in go.mod via shared replace directive)
| Library | Purpose | Location |
|---------|---------|----------|
| `shared/listener.ListenerBase` | Manages heartbeat, assignment refresh, migration subscriber goroutines | `shared/listener/base.go` |
| `shared/listener.ListenerConfig` | Config struct with defaults (HeartbeatInterval: 30s, AssignmentRefreshInterval: 10s, StartupJitterMax: 30s) | `shared/listener/config.go` |
| `shared/listener.Env` | Drop-in for `getEnvOrDefault` pattern | `shared/listener/config.go` line 58 |
| `shared/listener.ShutdownCoordinator` | Ordered shutdown: parallel stop → platform disconnect → HTTP drain | `shared/listener/shutdown.go` |
| `shared/listener.ChannelManager` | Interface that channels.Manager already satisfies (7 methods) | `shared/listener/channel_manager.go` |
| `shared/listener/testutil.MockCoordinator` | In-memory mock coordinator for smoke test | `shared/listener/testutil/mock_coordinator.go` |
| `go.uber.org/goleak v1.3.0` | Goroutine leak detection in smoke test | Must be added as direct dep to twitch-listener go.mod |

### Supporting
| Library | Purpose | Already in go.mod |
|---------|---------|-------------------|
| `shared/coordination.NewCoordinatorClient` | Creates coordinator HTTP client (passed to NewListenerBase) | Yes, via shared replace |
| `go.uber.org/zap` | Logging | Yes |
| `github.com/gin-gonic/gin` | HTTP router | Yes |

**Installation — one addition needed:**
```bash
cd services/twitch-listener && go get go.uber.org/goleak@v1.3.0
```

---

## Architecture Patterns

### SDK Constructor Call Chain

The migrated `main.go` follows this sequence:

```
1. Initialize logger, tracing, DB, Redis, metrics (unchanged)
2. Build IRC components (parser, publisher, msgRegistry, ircConn) — unchanged
3. Read ENABLE_COORDINATOR_FILTERING, build ListenerConfig with DisableCoordinatorFiltering
4. Create coordinator client: coordination.NewCoordinatorClient(url, secret, "twitch-listener", log)
5. Create ListenerBase: listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)
6. Initialize channels.Manager — unchanged constructor call, assignedSourceIDs passed as nil or initial empty map
7. Wire IRC: ircConn.SetFirstMessageChan(channelMgr.GetFirstMessageChan())
8. Connect IRC: ircConn.Connect(ctx) — fatal on error
9. time.Sleep(2 * time.Second) — IRC-specific, stays in main.go
10. base.Start(ctx, channelMgr) — queries assignments, calls mgr.UpdateAssignedSourceIDs, calls mgr.Start, launches 3 goroutines, starts JWT refresh
11. Record shardMetrics.PodChannelCount — service-specific, stays in main.go
12. Set up HTTP router, health handlers, metrics endpoint (unchanged)
13. Start HTTP server goroutine (unchanged)
14. Wait for SIGINT/SIGTERM
15. listener.ShutdownCoordinator(base, channelMgr, ircConn.Disconnect, srv, log)
```

### Pattern 1: ListenerBase Construction

```go
// Source: shared/listener/base.go + config.go (verified in codebase)
cfg := listener.DefaultConfig()
cfg.DisableCoordinatorFiltering = !enableFiltering

coordClient := coordination.NewCoordinatorClient(
    listener.Env("COORDINATOR_URL", "http://source-manager:8088"),
    os.Getenv("SERVICE_JWT_SECRET"),
    "twitch-listener",
    log,
)

base := listener.NewListenerBase(cfg, coordClient, redisClient, podName, log)
```

### Pattern 2: IRC-first, then base.Start

```go
// Source: CONTEXT.md locked decision + base.go Start() internals
ircConn.SetFirstMessageChan(channelMgr.GetFirstMessageChan())
if err := ircConn.Connect(ctx); err != nil {
    log.Fatal("Failed to connect to Twitch IRC", zap.Error(err))
}
time.Sleep(2 * time.Second)

if err := base.Start(ctx, channelMgr); err != nil {
    log.Fatal("Failed to start listener base", zap.Error(err))
}
```

### Pattern 3: ShutdownCoordinator replaces manual shutdown

```go
// Source: shared/listener/shutdown.go (verified)
// Replaces: manual channelMgr.Stop() + ircConn.Disconnect() + srv.Shutdown()
listener.ShutdownCoordinator(
    base,
    channelMgr,
    func() { _ = ircConn.Disconnect() }, // platform disconnect callback
    srv,
    log,
)
```

Note: `ircConn.Disconnect()` returns `error`. The `platformDisconnect` parameter is `func()` (no return). Wrap with an anonymous function. Error logging for IRC disconnect moves inside the wrapper or is dropped (SDK logs the ordering).

### Pattern 4: Compile-time assertion in channels/manager.go

```go
// Source: CONTEXT.md + channel_manager.go interface definition
// Add to channels/manager.go after package declaration and imports
var _ listener.ChannelManager = (*Manager)(nil)
```

Required import: `"github.com/caesar/all-chat/shared/listener"`

### Pattern 5: Smoke test using testutil.MockCoordinator

```go
// Source: shared/listener/base_test.go pattern (verified)
// File: services/twitch-listener/cmd/main_sdk_test.go
package main

import (
    "context"
    "testing"
    "github.com/caesar/all-chat/shared/listener"
    "github.com/caesar/all-chat/shared/listener/testutil"
    "go.uber.org/goleak"
    "go.uber.org/zap"
)

func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    mock := &testutil.MockCoordinator{}
    cfg := listener.ListenerConfig{
        HeartbeatInterval:         20 * time.Millisecond,
        AssignmentRefreshInterval: 20 * time.Millisecond,
        StartupJitterMax:          0,
    }
    base := listener.NewListenerBase(cfg, mock, nil, "test-pod", zap.NewNop())
    mgr := &mockChannelManagerForTest{} // local stub satisfying ChannelManager

    ctx, cancel := context.WithCancel(context.Background())
    if err := base.Start(ctx, mgr); err != nil {
        t.Fatal(err)
    }
    cancel()
    base.Stop()
    // goleak.VerifyNone fires here
}
```

The `mockChannelManagerForTest` stub must be defined in the test file since `channels.Manager` cannot be constructed without a real DB/Redis. It satisfies `listener.ChannelManager` (7 methods, all no-ops).

### Anti-Patterns to Avoid

- **Constructing CoordinatorClient before reading SERVICE_JWT_SECRET**: `NewCoordinatorClient` calls `auth.GenerateServiceJWT` immediately — if the secret is empty it calls `log.Fatal`. Read and validate the secret before constructing the client.
- **Calling base.Start before ircConn.Connect**: `base.Start` calls `mgr.Start` which calls `channelMgr.SyncChannels` which calls `ircConn.Join` — IRC must be connected first.
- **Passing ircConn.Disconnect directly as platformDisconnect**: `Disconnect()` returns `error` but `platformDisconnect` is `func()`. Always wrap.
- **Calling coordClient.StartJWTRefresh or StopJWTRefresh in main.go**: These are now SDK-owned — calling them in main.go after passing coordClient to ListenerBase creates double-call bugs.
- **Keeping any of the 4 goroutine blocks in main.go**: The heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine, and `coordClient.StartJWTRefresh(ctx)` / `defer coordClient.StopJWTRefresh()` calls are ALL replaced by `base.Start` / `base.Stop`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Heartbeat loop | Ticker goroutine in main.go | `ListenerBase` (internal `startHeartbeatLoop`) | SDK has exponential backoff (1s→30s cap), error propagation via OnFatalError |
| Assignment refresh loop | Ticker goroutine in main.go | `ListenerBase` (internal `startAssignmentRefreshLoop`) | SDK handles DisableCoordinatorFiltering flag, backoff, nil-safe |
| Migration subscriber | Goroutine + `coordination.NewMigrationSubscriber` in main.go | `ListenerBase` (internal `startMigrationSubscriberLoop`) | SDK handles nil redis guard, retry loop with backoff |
| JWT refresh lifecycle | `coordClient.StartJWTRefresh` / `StopJWTRefresh` in main.go | `ListenerBase.Start` / `ListenerBase.Stop` | Prevents double-start on migration |
| Ordered shutdown | Manual sequence in main.go | `listener.ShutdownCoordinator` | Handles parallel stop, platform disconnect ordering, 10s HTTP drain |

**Key insight:** The SDK was built specifically to eliminate the duplicated wiring in main.go. Any goroutine or coordination pattern already in `shared/listener/base.go` must NOT be reproduced in main.go.

---

## Common Pitfalls

### Pitfall 1: Source ID Stripping Gap in SDK

**What goes wrong:** `ListenerBase.startAssignmentRefreshLoop` calls `ids[a.SourceID] = true` without stripping the `:twitch` platform suffix. If coordinator returns composite IDs like `"abc123:twitch"`, channels.Manager would compare against bare UUIDs from the database and coverage verification would fail, causing filtering to be disabled (safety fallback).

**Why it happens:** Phase 33 established that stripping is done at intake in `cmd/main.go`. The SDK was built to delegate assignment handling but does not replicate the stripping logic from the old goroutines.

**How to avoid:** Verify at migration time whether the coordinator actually returns composite IDs or bare UUIDs for twitch-listener. If composite IDs are still returned by source-manager, the planner must decide: either (a) accept that `DisableCoordinatorFiltering=true` is always used in production (the rollback knob), or (b) add source ID stripping inside `channels.Manager.UpdateAssignedSourceIDs` (but CONTEXT.md says channels/manager.go is not modified), or (c) document that filtering is expected to degrade gracefully via the coverage check fallback. The `ENABLE_COORDINATOR_FILTERING=false` flag (default) means filtering is disabled by default anyway — so this only matters when the feature is explicitly enabled.

**Warning signs:** Coverage verification log entries at `ERROR` level: "Coverage verification FAILED - filtering disabled for safety". Check `channels.Manager.verifyCoverageComplete` output in logs after deployment with `ENABLE_COORDINATOR_FILTERING=true`.

### Pitfall 2: Double JWT Refresh

**What goes wrong:** If `coordClient.StartJWTRefresh(ctx)` and `defer coordClient.StopJWTRefresh()` are left in main.go AND the SDK also calls them in `base.Start` / `base.Stop`, the JWT refresh goroutine runs twice and `StopJWTRefresh` (which closes a channel) may panic on double-close.

**Why it happens:** The old main.go had explicit JWT refresh lifecycle management. Easy to forget to remove.

**How to avoid:** Delete lines 126-127 of the current main.go (`coordClient.StartJWTRefresh(ctx)` and `defer coordClient.StopJWTRefresh()`) as part of the migration. Grep for both calls as a verification step.

### Pitfall 3: Leaving the Signal Handler for Manual Shutdown

**What goes wrong:** The current main.go has a manual shutdown sequence after `<-quit`. After migration, `ShutdownCoordinator` replaces the manual `channelMgr.Stop()`, `ircConn.Disconnect()`, and `srv.Shutdown()` calls. If any of the manual calls are left in addition to `ShutdownCoordinator`, double-stop panics occur (e.g., `close(m.stopChan)` called twice on channels.Manager).

**How to avoid:** Replace the entire post-`<-quit` block with a single `listener.ShutdownCoordinator(...)` call.

### Pitfall 4: goleak Missing from twitch-listener go.mod

**What goes wrong:** Smoke test file imports `go.uber.org/goleak` but it is only in `go.sum` (transitive via shared). `go test ./cmd/...` fails with "cannot find module providing package go.uber.org/goleak".

**Why it happens:** goleak is a direct dependency of `shared/go.mod` but transitive in twitch-listener. Test files require explicit direct deps.

**How to avoid:** Run `go get go.uber.org/goleak@v1.3.0` in services/twitch-listener before writing the test.

### Pitfall 5: channels.Manager Constructor Receives Non-nil assignedSourceIDs Too Early

**What goes wrong:** In the old main.go, `assignedSourceIDs` was populated by the startup assignment query before `channels.NewManager` was called. In the new flow, `channels.NewManager` is called with `nil` or empty `assignedSourceIDs`, and the SDK calls `mgr.UpdateAssignedSourceIDs` inside `base.Start`. If the constructor receives a non-nil map, and then `UpdateAssignedSourceIDs` overwrites it — that is fine. But if the constructor is called with the old manually-queried map (which the SDK will also query), the initial assignments are queried twice.

**How to avoid:** Pass `nil` or an empty `map[string]bool{}` for `assignedSourceIDs` in `channels.NewManager(...)`. Let the SDK own all assignment state via `UpdateAssignedSourceIDs`.

---

## Code Examples

Verified from codebase inspection:

### NewListenerBase signature
```go
// Source: shared/listener/base.go line 37 (verified)
func NewListenerBase(
    config ListenerConfig,
    client coordinatorClient,  // *coordination.CoordinatorClient satisfies this
    redisClient *redis.Client, // may be nil — disables migration subscriber
    podID string,
    logger *zap.Logger,
) *ListenerBase
```

### ShutdownCoordinator signature
```go
// Source: shared/listener/shutdown.go line 17 (verified)
func ShutdownCoordinator(
    base *ListenerBase,
    mgr ChannelManager,
    platformDisconnect func(), // nil-safe
    srv *http.Server,
    logger *zap.Logger,
)
```

### ChannelManager interface (7 methods — all satisfied by channels.Manager)
```go
// Source: shared/listener/channel_manager.go (verified)
type ChannelManager interface {
    Start(ctx context.Context) error
    Stop()
    HandleMigrationEvent(event *coordination.MigrationEvent) error
    UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
    GetFilteredAssignmentCount() int
    GetActiveChannels() []string
    GetActiveChannelCount() int
}
```

### Compile-time assertion (channels/manager.go)
```go
// Import added: "github.com/caesar/all-chat/shared/listener"
var _ listener.ChannelManager = (*Manager)(nil)
```

### Env helper (replaces getEnvOrDefault)
```go
// Source: shared/listener/config.go line 58 (verified)
// Usage in main.go — same signature:
dbHost := listener.Env("DATABASE_HOST", "localhost")
port   := listener.Env("PORT", "8085")
```

### DefaultConfig with filtering flag
```go
// Source: shared/listener/config.go (verified)
enableFiltering := listener.Env("ENABLE_COORDINATOR_FILTERING", "false") == "true"
cfg := listener.DefaultConfig()
cfg.DisableCoordinatorFiltering = !enableFiltering
```

---

## What Stays in main.go After Migration

The following blocks are NOT replaced by the SDK and must remain:

| Block | Reason |
|-------|--------|
| Logger initialization | Service-specific |
| Tracing initialization | Service-specific |
| Required env validation (TWITCH_BOT_USERNAME, TWITCH_BOT_OAUTH) | IRC-specific |
| DB and Redis connection | Service-specific infrastructure |
| Metrics initialization (listenerMetrics, shardMetrics) | Service-specific |
| podName from HOSTNAME | Still needed as podID argument to NewListenerBase |
| SERVICE_JWT_SECRET validation + coordClient construction | coordClient passed to NewListenerBase |
| ENABLE_COORDINATOR_FILTERING + ListenerConfig construction | SDK-06 rollback knob |
| IRC component construction (parser, publisher, msgRegistry, ircConn) | IRC-specific |
| leaderCoord = nil + channels.NewManager call | Service-specific |
| ircConn.SetFirstMessageChan(...) | IRC migration wiring |
| ircConn.Connect(ctx) | IRC-specific |
| time.Sleep(2 * time.Second) | IRC-specific connection stabilization |
| shardMetrics.PodChannelCount recording after base.Start | Service-specific observability |
| HTTP router, handlers, metrics endpoint | Service-specific |
| HTTP server goroutine | Service-specific |
| dbConnWrapper struct | Service-specific (used by channels.NewManager) |

**Removed from main.go:**
- `coordClient.StartJWTRefresh(ctx)` + `defer coordClient.StopJWTRefresh()` — SDK owns
- Startup jitter (`rand.Intn(30)` + `time.Sleep`) — SDK owns
- `coordClient.QueryAssignments(ctx, podName)` startup call — SDK owns
- `assignedSourceIDs` map construction — SDK owns
- `channelMgr.Start(ctx)` — SDK owns (called inside base.Start)
- `migrationSub` goroutine — SDK owns
- Heartbeat goroutine — SDK owns
- Assignment refresh goroutine — SDK owns
- `math/rand` and `strings` imports — no longer needed
- `getEnvOrDefault` function — replaced by `listener.Env`
- Manual shutdown sequence (`channelMgr.Stop()`, `ircConn.Disconnect()`, `srv.Shutdown()`) — replaced by ShutdownCoordinator

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| Inline goroutines in main.go for heartbeat/refresh/migration | Encapsulated in ListenerBase | All three goroutines + backoff logic centralized |
| Manual JWT refresh in main.go | SDK-owned in ListenerBase.Start/Stop | Cannot double-start |
| Manual shutdown sequence in main.go | ShutdownCoordinator handles ordering | Parallel stop + 10s drain guaranteed |
| getEnvOrDefault local function | listener.Env helper | Eliminated copy-paste boilerplate |
| No compile-time assertion | var _ listener.ChannelManager = (*Manager)(nil) | Build fails immediately if interface drifts |

---

## Open Questions

1. **Source ID stripping in SDK**
   - What we know: Phase 33 normalized stripping to main.go. The SDK's `base.go` uses `a.SourceID` directly without stripping. The coordinator may still return `"uuid:twitch"` composite IDs.
   - What's unclear: Whether source-manager was also updated to return bare IDs server-side, or whether the stripping is now expected to be inside the SDK.
   - Recommendation: The planner should check whether `ENABLE_COORDINATOR_FILTERING` defaults to `false` in production (it does — current main.go line 160 defaults to "false"). Since filtering is disabled by default, this affects only the explicit opt-in path. Accept the current SDK behavior; document that filtering with composite IDs degrades to the safety fallback (all channels processed). If needed in a future phase, add stripping inside `UpdateAssignedSourceIDs`.

2. **Smoke test mock channel manager location**
   - What we know: The test is in `cmd/main_sdk_test.go` (package `main`). It needs a `ChannelManager` stub. `channels.Manager` cannot be constructed without DB/Redis.
   - What's unclear: Whether to use an inline `mockChannelManagerForTest` struct or import something from testutil.
   - Recommendation: Define a local `mockChannelManagerForTest` struct in the test file (same pattern as `shared/listener/base_test.go` which defines `mockChannelManager` locally). No testutil.NewMockChannelManager exists — testutil only exports `MockCoordinator`.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 + goleak v1.3.0 |
| Config file | none — standard `go test` |
| Quick run command | `cd services/twitch-listener && go test ./... -count=1` |
| Full suite command | `cd services/twitch-listener && go test ./... -race -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIGRATE-01 | main.go contains only IRC setup + publishing — SDK owns lifecycle goroutines | build + goleak smoke | `go build ./cmd/... && go test ./cmd/... -count=1` | ❌ Wave 0: `cmd/main_sdk_test.go` |
| MIGRATE-01 | Existing unit tests pass without modification | unit | `go test ./channels/... ./irc/... ./publisher/... ./models/... -count=1` | ✅ existing |
| VERIFY-02 | Compile-time assertion present and build succeeds | build | `go build ./...` in twitch-listener | ❌ Wave 0: assertion line in channels/manager.go |

### Sampling Rate
- **Per task commit:** `cd services/twitch-listener && go build ./... && go test ./... -count=1`
- **Per wave merge:** `cd services/twitch-listener && go test ./... -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/twitch-listener/cmd/main_sdk_test.go` — goleak smoke test covering MIGRATE-01
- [ ] `go.uber.org/goleak` direct dependency in `services/twitch-listener/go.mod` — needed by smoke test
- [ ] `var _ listener.ChannelManager = (*Manager)(nil)` in `channels/manager.go` — covers VERIFY-02

*(No new test infrastructure beyond goleak dep and the smoke test file.)*

---

## Sources

### Primary (HIGH confidence)
- `/home/moersener/Hobby/all-chat/shared/listener/base.go` — NewListenerBase, Start, Stop, goroutine internals verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/config.go` — ListenerConfig fields, DefaultConfig, Env helper verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/shutdown.go` — ShutdownCoordinator signature and ordering verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/channel_manager.go` — 7-method interface definition verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/testutil/mock_coordinator.go` — MockCoordinator fields and methods verified directly
- `/home/moersener/Hobby/all-chat/shared/listener/base_test.go` — smoke test pattern (mockChannelManager local struct, goleak.VerifyNone) verified directly
- `/home/moersener/Hobby/all-chat/services/twitch-listener/cmd/main.go` — current goroutines and shutdown sequence inventoried directly
- `/home/moersener/Hobby/all-chat/services/twitch-listener/channels/manager.go` — 7 interface methods confirmed present, no assertion yet
- `/home/moersener/Hobby/all-chat/services/twitch-listener/go.mod` — replace directive confirmed, goleak absent as direct dep
- `.planning/phases/33-pre-migration-cleanup/33-01-SUMMARY.md` — source ID strip-at-intake decision confirmed

### Secondary (MEDIUM confidence)
- `.planning/phases/34-sdk-package-definition/34-VERIFICATION.md` — Phase 34 build verification results

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all SDK APIs verified by reading source files directly
- Architecture: HIGH — migration sequence derived from locked CONTEXT.md decisions + SDK source
- Pitfalls: HIGH (except source ID gap: MEDIUM) — most pitfalls verified against actual code; source ID gap is a logic inference from two separate source files

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable internal codebase; only at risk from changes to shared/listener or coordinator)
