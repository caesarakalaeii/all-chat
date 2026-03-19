# Phase 34: SDK Package Definition - Research

**Researched:** 2026-03-17
**Domain:** Go shared package design — lifecycle management, interface extraction, goroutine testing
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- `shared/listener` package lives in the existing `shared` module (`shared/go.mod`) — no new Go module, no `go.work`
- Four new files: `base.go`, `leadership.go`, `channel_manager.go`, `shutdown.go`
- `ListenerConfig` includes `OnFatalError func(source string, err error)` callback; nil means retry indefinitely with backoff
- Default intervals: heartbeat 30s, assignment refresh 10s, startup jitter max 30s
- `LISTENER_STARTUP_JITTER_MAX=0` env var disables jitter in tests
- `ChannelManager` interface has 7 methods: `Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`, `GetActiveChannels() []string`, `GetActiveChannelCount() int`
- Compile-time assertions live in each listener's `channels/manager.go` (NOT in the SDK)
- Mock lives in `shared/listener/testutil/` — behavioral with failure modes, tracks call counts
- `NewCoordinatorClient` gains explicit `serviceName string` parameter — hostname auto-detection block removed
- `Env(key, defaultValue string) string` is a package-level function (not a method)
- `LeadershipListener` has nil-safe passthrough when `SOURCE_MANAGER_SECRET` is absent
- `ShutdownCoordinator` uses a fixed 10s HTTP server drain timeout (no context parameter)

### Claude's Discretion

- Exact retry backoff strategy for goroutines (exponential with jitter is standard)
- Internal HTTP client timeout values for coordinator calls
- Package-level doc comments and exported type documentation
- Whether `ShutdownCoordinator` accepts a context or uses a fixed 10s timeout internally

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SDK-01 | `ListenerBase` struct in `shared/listener/base.go` managing full shared lifecycle | Goroutine lifecycle patterns, context cancellation, jitter implementation |
| SDK-02 | `LeadershipListener` in `shared/listener/leadership.go` (embeds `ListenerBase`) | `sourcemanager.NewLeadershipCoordinator` + `NewClient` construction from env, nil-safe passthrough |
| SDK-03 | `ChannelManager` interface in `shared/listener/channel_manager.go` | Both existing managers satisfy all 7 methods — interface extraction is safe |
| SDK-04 | `ShutdownCoordinator` in `shared/listener/shutdown.go` with ordered shutdown | HTTP server `Shutdown` with 10s timeout context, parallel channel+base stop |
| SDK-05 | `ListenerConfig` with configurable intervals and `LISTENER_STARTUP_JITTER_MAX=0` | `os.Getenv` + `strconv.Atoi` pattern, zero-value disables |
| SDK-06 | `ListenerConfig.DisableCoordinatorFiltering bool` | Direct field read in `ListenerBase.Start` before passing assignments to manager |
| SDK-07 | `Env(key, defaultValue string) string` helper | Replaces 4 hand-copied `getEnvOrDefault`/`getEnv` functions across listeners |
| SDK-08 | `NewCoordinatorClient` accepts explicit `serviceName string` | 3 callers to update: twitch-listener, kick-listener, twitch-eventsub-listener |
| VERIFY-01 | `make build-all` Makefile target building all listener modules | Target does not yet exist — add to root Makefile; excludes Node.js tiktok-listener |

</phase_requirements>

---

## Summary

Phase 34 creates the `shared/listener` Go package — a set of four files that extract the lifecycle boilerplate duplicated across every listener's `cmd/main.go` into a reusable SDK. The package is added inside the existing `shared` module (`shared/go.mod`), making it automatically available to all listener services that already have `replace github.com/caesar/all-chat/shared => ../../shared` in their `go.mod`.

The core work is three interlocking pieces: (1) defining the `ChannelManager` interface that both existing concrete managers already satisfy, (2) implementing `ListenerBase` and `LeadershipListener` structs that manage the three recurring background goroutines (heartbeat, assignment refresh, migration subscriber), and (3) updating `NewCoordinatorClient` to accept an explicit `serviceName` parameter instead of deriving it from the pod hostname. Supporting work includes the `testutil/` mock package, the `Env()` helper, and the new `make build-all` CI target.

The key technical risk is goroutine leak prevention in tests. `go.uber.org/goleak` is not currently used anywhere in the repo — it must be added to `shared/go.mod` as a test dependency and used with `goleak.VerifyNone(t)` in the `ListenerBase` unit tests. The mock coordinator must be fully controllable (blocking calls, simulated failures) to make these deterministic.

**Primary recommendation:** Extract the interface first (channel_manager.go), verify both concrete managers compile against it, then build `ListenerBase` with mock-backed unit tests, then add the `Env()` helper and `make build-all`. The `NewCoordinatorClient` signature change and its three callers must be done atomically.

---

## Standard Stack

### Core (all already in shared/go.mod)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.uber.org/zap` | v1.27.1 | Structured logging passed into SDK structs | Used by every service in this repo |
| `context` (stdlib) | Go 1.25.6 | Cancellation for `Start(ctx context.Context) error` | Idiomatic Go goroutine lifecycle |
| `os` (stdlib) | Go 1.25.6 | `Env()` helper reads env vars | Standard; no third-party needed |
| `time` (stdlib) | Go 1.25.6 | Ticker for heartbeat/refresh intervals | Standard |
| `sync` (stdlib) | Go 1.25.6 | `sync.WaitGroup` for goroutine tracking | Standard |
| `math/rand` (stdlib) | Go 1.25.6 | Startup jitter calculation | `rand.Int63n` sufficient; no crypto needed |
| `strconv` (stdlib) | Go 1.25.6 | Parse `LISTENER_STARTUP_JITTER_MAX` env int | Standard |
| `net/http` (stdlib) | Go 1.25.6 | `http.Server.Shutdown` in `ShutdownCoordinator` | Standard |

### Test Dependency (new — must be added to shared/go.mod)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection | Idiomatic for goroutine-heavy packages; pairs with `testing.T` |

### Supporting (already in shared/go.mod)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | `MigrationSubscriber` wiring in `ListenerBase` | Already used by `MigrationSubscriber` in coordination package |
| `github.com/caesar/all-chat/shared/coordination` | (local) | `CoordinatorClient`, `MigrationSubscriber` | SDK wraps their construction |
| `github.com/caesar/all-chat/shared/sourcemanager` | (local) | `LeadershipCoordinator`, `Client` for `LeadershipListener` | SDK-02 wires these from env |

**Installation (new test dependency only):**
```bash
cd shared && go get go.uber.org/goleak@v1.3.0
```

---

## Architecture Patterns

### Recommended Package Structure

```
shared/listener/
├── base.go              # ListenerBase struct + Start/Stop + 3 goroutines
├── leadership.go        # LeadershipListener struct (embeds ListenerBase)
├── channel_manager.go   # ChannelManager interface (7 methods)
├── shutdown.go          # ShutdownCoordinator ordered shutdown helper
├── config.go            # ListenerConfig struct + Env() helper  [OR: embed in base.go]
└── testutil/
    └── mock_coordinator.go  # MockCoordinator for unit tests
```

Note: `ListenerConfig` and `Env()` may coexist in `base.go` or a separate `config.go` — the planner may decide. Both compile identically.

### Pattern 1: ListenerBase Goroutine Lifecycle

**What:** Three goroutines started by `Start(ctx)`, tracked with `sync.WaitGroup`, stopped by canceling the context.
**When to use:** Any goroutine that must start on `Start()` and cleanly stop on `Stop()`.

```go
// Idiomatic pattern — goroutine owned by WaitGroup + context
func (b *ListenerBase) startHeartbeatLoop(ctx context.Context) {
    b.wg.Add(1)
    go func() {
        defer b.wg.Done()
        ticker := time.NewTicker(b.config.HeartbeatInterval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                if err := b.client.PublishHeartbeat(ctx, b.podID); err != nil {
                    b.logger.Error("heartbeat failed", zap.Error(err))
                    if b.config.OnFatalError != nil {
                        b.config.OnFatalError("heartbeat", err)
                    }
                    // retry on next tick if callback is nil
                }
            }
        }
    }()
}

func (b *ListenerBase) Stop() {
    b.cancel()     // cancels the context passed to goroutines
    b.wg.Wait()    // waits for all goroutines to exit
}
```

### Pattern 2: ListenerBase.Start with Jitter

**What:** Read `LISTENER_STARTUP_JITTER_MAX` env var (integer seconds). If > 0, `time.Sleep(rand.Int63n(max) * time.Second)`. If 0, skip.
**When to use:** All listeners to prevent thundering herd on scale-up.

```go
func readJitterMax() time.Duration {
    raw := os.Getenv("LISTENER_STARTUP_JITTER_MAX")
    if raw == "" {
        return 30 * time.Second // default
    }
    n, err := strconv.Atoi(raw)
    if err != nil || n <= 0 {
        return 0 // disables jitter (used in tests)
    }
    return time.Duration(n) * time.Second
}

// In Start():
if jitterMax := readJitterMax(); jitterMax > 0 {
    jitter := time.Duration(rand.Int63n(int64(jitterMax)))
    b.logger.Info("applying startup jitter", zap.Duration("jitter", jitter))
    time.Sleep(jitter)
}
```

Note: `ListenerConfig.StartupJitterMax` can override the env var — config takes precedence over env. Tests set `StartupJitterMax: 0` directly without needing env var manipulation.

### Pattern 3: ChannelManager Interface Extraction

**What:** Interface whose method set is the exact union of methods already on both `twitch-listener/channels.Manager` and `kick-listener/channels.Manager`.
**When to use:** SDK-03 — defined once in `shared/listener/channel_manager.go`.

```go
// Source: inspection of both concrete managers (lines confirmed)
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

**Important:** Kick's `Start()` has signature `Start() error` (no ctx parameter — confirmed from source). Twitch's has `Start(ctx context.Context) error`. The interface must match the TWITCH signature for forward compatibility. **The kick-listener `channels.Manager.Start` method will need a context parameter added** to satisfy the interface — this is a necessary same-phase change alongside SDK-03.

### Pattern 4: ShutdownCoordinator with Parallel Stop

**What:** Ordered shutdown — channel manager stop AND base stop run in parallel (`sync.WaitGroup`), then platform-specific disconnect, then HTTP server drain with fixed 10s context timeout.

```go
func ShutdownCoordinator(base *ListenerBase, mgr ChannelManager, platformDisconnect func(), srv *http.Server, logger *zap.Logger) {
    // Phase 1: stop SDK-managed goroutines in parallel
    var wg sync.WaitGroup
    wg.Add(2)
    go func() { defer wg.Done(); mgr.Stop() }()
    go func() { defer wg.Done(); base.Stop() }()
    wg.Wait()

    // Phase 2: platform-specific disconnect (IRC PART, WebSocket close, etc.)
    if platformDisconnect != nil {
        platformDisconnect()
    }

    // Phase 3: drain HTTP server with fixed 10s timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        logger.Error("HTTP server shutdown error", zap.Error(err))
    }
}
```

### Pattern 5: LeadershipListener Nil-Safe Passthrough

**What:** `LeadershipListener` reads `SOURCE_MANAGER_SECRET` from env. If absent, `LeadershipCoordinator` and `SourceManagerClient` are `nil` — both types already have nil-safe guards (confirmed from `sourcemanager/coordinator.go`: `if c == nil || c.client == nil { return true, nil }`).

```go
func NewLeadershipListenerFromEnv(base *ListenerBase, platform string, logger *zap.Logger) (*LeadershipListener, error) {
    secret := os.Getenv("SOURCE_MANAGER_SECRET")
    if secret == "" {
        logger.Info("SOURCE_MANAGER_SECRET not set — leadership coordination disabled")
        return &LeadershipListener{base: base}, nil // nil coordinator/client
    }
    // ... construct client and coordinator
}
```

### Pattern 6: goleak Goroutine Leak Verification

**What:** `goleak.VerifyNone(t)` at the end of each test (or via `TestMain`) asserts no goroutines leaked.
**When to use:** Any test that calls `ListenerBase.Start()`.

```go
// Per-test approach (preferred when tests are independent)
func TestListenerBase_StartStop(t *testing.T) {
    defer goleak.VerifyNone(t)

    mock := testutil.NewMockCoordinator()
    base := listener.NewListenerBase(listener.ListenerConfig{
        StartupJitterMax: 0, // disable jitter in tests
        HeartbeatInterval: 50 * time.Millisecond,
    }, mock, logger)

    ctx, cancel := context.WithCancel(context.Background())
    require.NoError(t, base.Start(ctx, mockChannelManager))

    cancel() // triggers goroutine shutdown
    base.Stop()
    // goleak.VerifyNone fires after Stop() — must be clean
}
```

### Anti-Patterns to Avoid

- **Do NOT use `time.Sleep` in tests for goroutine synchronization:** Use `goleak.VerifyNone` + `wg.Wait()` instead. Sleep-based assertions produce flaky tests.
- **Do NOT make `ListenerBase.Stop()` idempotent via sync.Once initially:** The WaitGroup pattern is sufficient and simpler. Add Once only if double-Stop is a production concern.
- **Do NOT put the compile-time assertion in `channel_manager.go`:** Per locked decision, assertions live in each listener's `channels/manager.go`. SDK file only declares the interface.
- **Do NOT use `context.Background()` inside goroutines:** The goroutine context must be the one passed to `Start()` — canceling it is the stop signal.
- **Do NOT `go mod tidy` across all listeners in a single step:** Each service has its own `go.mod`; tidy must run per-module. `make build-all` uses `go build ./...` in each module, not a workspace.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Goroutine leak detection | Custom goroutine counter | `go.uber.org/goleak` | Detects leaked goroutines from any source, not just ones you explicitly track |
| JWT generation for `NewCoordinatorClient` | New auth code | Existing `auth.GenerateServiceJWT` (already in shared/auth) | Already tested, already used by existing `NewCoordinatorClient` |
| Exponential backoff | Custom sleep loop | Inline `backoff *= 2; if backoff > max { backoff = max }` | Pattern already used in `CoordinatorClient.QueryAssignments` — stay consistent |
| env-with-default | Copy-paste per service | `listener.Env(key, default)` (SDK-07) | This is what SDK-07 exists to provide |
| Nil-safe leadership | If-nil guards per listener | `LeadershipCoordinator` nil guards already in `sourcemanager/coordinator.go` | `EnsureLeadership` and `Stop` both guard `if c == nil || c.client == nil` |

**Key insight:** The codebase already has production-tested implementations of every non-trivial piece (JWT refresh, exponential backoff, leadership renewal with grace period). The SDK's job is composition and configuration, not re-implementation.

---

## Common Pitfalls

### Pitfall 1: Kick ChannelManager.Start Signature Mismatch

**What goes wrong:** `kick-listener/channels.Manager.Start()` has no `context.Context` parameter (confirmed from source: line 143 `func (m *Manager) Start() error`). The SDK interface requires `Start(ctx context.Context) error`.
**Why it happens:** Kick's Manager uses an internal `m.ctx` created in `NewManager`, while Twitch's Manager takes ctx from the caller.
**How to avoid:** In the same plan wave that defines the interface, add the `ctx context.Context` parameter to kick's `Start` method. The caller (currently `cmd/main.go`) must pass its context. The internal `m.ctx` / `m.cancel` pair remains for use by background goroutines.
**Warning signs:** Compilation error on `var _ listener.ChannelManager = (*channels.Manager)(nil)` in kick-listener.

### Pitfall 2: client_jwt_test.go Breaks After SDK-08 Signature Change

**What goes wrong:** `shared/coordination/client_jwt_test.go` calls `NewCoordinatorClient(baseURL, secret, logger)` directly with the old 3-arg signature. After adding `serviceName string`, the test must also pass a service name.
**Why it happens:** The test is in the `coordination` package (internal) and calls the constructor directly.
**How to avoid:** Update `client_jwt_test.go` in the same task that modifies `NewCoordinatorClient`. Use `"test-service"` as the service name.
**Warning signs:** Compile error in `shared/coordination/client_jwt_test.go`.

### Pitfall 3: Double-Close of stopRefresh Channel in CoordinatorClient

**What goes wrong:** `StopJWTRefresh()` calls `close(c.stopRefresh)`. If called twice, it panics. The existing test (`TestStartStopJWTRefresh`) already calls it twice and does not panic — this works because the goroutine reads from the channel only once, but the double-close of a closed channel is a latent panic.
**Why it happens:** The existing implementation lacks a `sync.Once` guard on close.
**How to avoid:** Do not introduce additional `StopJWTRefresh()` calls in SDK code. The SDK manages `ListenerBase` lifecycle separately from the `CoordinatorClient`'s JWT refresh lifecycle.
**Warning signs:** `panic: close of closed channel` in tests.

### Pitfall 4: Assignment Refresh Goroutine Races with ChannelManager

**What goes wrong:** `ListenerBase`'s assignment refresh goroutine calls `mgr.UpdateAssignedSourceIDs(...)` on a ticker. If `mgr.Stop()` races with an in-progress `UpdateAssignedSourceIDs`, the manager's mutex must protect the map.
**Why it happens:** Both `twitch-listener` and `kick-listener` managers already hold a mutex in `UpdateAssignedSourceIDs` — but the SDK goroutine must not hold a reference to a stopped manager.
**How to avoid:** Stop order: `base.Stop()` (which cancels the refresh goroutine) BEFORE or concurrent with `mgr.Stop()`. The `ShutdownCoordinator` runs both in parallel, which is safe because `mgr.Stop()` closes the manager's own stop channel and the SDK goroutine stops on ctx cancel — they don't wait for each other.
**Warning signs:** Data race detector output during `go test -race`.

### Pitfall 5: goleak False Positives from go-redis Background Goroutines

**What goes wrong:** `go-redis` starts internal goroutines when a client is created. If the mock coordinator spawns a real redis client, `goleak.VerifyNone` fails.
**Why it happens:** Third-party library goroutines are not under test control.
**How to avoid:** The `MockCoordinator` in `testutil/` must NOT use a real Redis client. It simulates coordinator behavior in-memory. For the `MigrationSubscriber`, use a channel-based fake in tests.
**Warning signs:** `goleak.VerifyNone` reports goroutines owned by `github.com/redis/go-redis`.

### Pitfall 6: LISTENER_STARTUP_JITTER_MAX env var Bleeds Between Tests

**What goes wrong:** Setting `os.Setenv("LISTENER_STARTUP_JITTER_MAX", "0")` in one test leaves the env var set for other tests in the same process.
**Why it happens:** `os.Setenv` is process-global.
**How to avoid:** Use `ListenerConfig.StartupJitterMax` field directly in tests (set to `0` explicitly), not env var manipulation. The `Env()` helper is for runtime configuration, not test control.
**Warning signs:** Tests that rely on default jitter max fail when run after a test that sets the env var.

---

## Code Examples

Verified patterns from actual codebase:

### NewCoordinatorClient After SDK-08

```go
// File: shared/coordination/client.go
// Change: remove hostname auto-detection block, accept serviceName directly
func NewCoordinatorClient(baseURL, serviceSecret, serviceName string, logger *zap.Logger) *CoordinatorClient {
    jwt, err := auth.GenerateServiceJWT(serviceName, serviceSecret, 24*time.Hour)
    if err != nil {
        logger.Fatal("Failed to generate service JWT", zap.Error(err))
    }
    return &CoordinatorClient{
        baseURL:       baseURL,
        serviceSecret: serviceSecret,
        serviceJWT:    jwt,
        serviceName:   serviceName,
        httpClient:    &http.Client{Timeout: 10 * time.Second},
        logger:        logger,
        stopRefresh:   make(chan struct{}),
    }
}
```

### Callers to Update (3 files)

```go
// services/twitch-listener/cmd/main.go:122
coordClient := coordination.NewCoordinatorClient(coordinatorURL, serviceJWT, "twitch-listener", log)

// services/kick-listener/cmd/main.go:109
coordClient := coordination.NewCoordinatorClient(coordinatorURL, serviceJWT, "kick-listener", log)

// services/twitch-eventsub-listener/cmd/main.go:163
coordClient := coordination.NewCoordinatorClient(coordinatorURL, serviceJWT, "twitch-eventsub-listener", log)
```

### Test also to Update

```go
// shared/coordination/client_jwt_test.go — all 3 NewCoordinatorClient calls
client := NewCoordinatorClient(baseURL, secret, "test-service", logger)
```

### Env() Helper

```go
// File: shared/listener/base.go (or config.go)
// Drop-in replacement for getEnvOrDefault() / getEnv() across all listeners
func Env(key, defaultValue string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultValue
}
```

### make build-all Target (VERIFY-01)

```makefile
# Add to root Makefile
build-all:
	@echo "Building all Go listener modules..."
	cd shared && go build ./...
	cd services/twitch-listener && go build ./...
	cd services/kick-listener && go build ./...
	cd services/twitch-eventsub-listener && go build ./...
	cd services/youtube-listener && go build ./...
	cd services/youtube-listener-innertube && go build ./...
	cd services/discord-listener && go build ./...
	@echo "All listener modules built successfully"
```

Note: `tiktok-listener` is Node.js — excluded from `build-all` per project decision.

### MockCoordinator (testutil skeleton)

```go
// File: shared/listener/testutil/mock_coordinator.go
package testutil

import (
    "context"
    "sync/atomic"
)

type MockCoordinator struct {
    heartbeatCount   atomic.Int64
    assignmentCount  atomic.Int64
    ShouldFailHeartbeat bool
    ShouldFail401       bool
}

func (m *MockCoordinator) PublishHeartbeat(ctx context.Context, podID string) error {
    m.heartbeatCount.Add(1)
    if m.ShouldFail401 { return errors.New("401 unauthorized") }
    if m.ShouldFailHeartbeat { return errors.New("coordinator down") }
    return nil
}

func (m *MockCoordinator) HeartbeatCallCount() int64 { return m.heartbeatCount.Load() }
func (m *MockCoordinator) AssignmentCallCount() int64 { return m.assignmentCount.Load() }
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hostname prefix auto-detection in `NewCoordinatorClient` | Explicit `serviceName string` parameter | Phase 34 (now) | Eliminates fragile hostname parsing; enables SDK to pass known service name |
| Copy-paste `getEnvOrDefault` in every listener | `listener.Env()` package function | Phase 34 (now) | Single source of truth; compile-checked |
| Goroutine lifecycle in `cmd/main.go` | `ListenerBase.Start/Stop` | Phase 34 (now) | Extracted into tested SDK; MIGRATE phases (35-38) consume it |

**Not deprecated yet:**
- The concrete `channels.Manager` in twitch/kick — still exists; SDK defines the interface they satisfy. Migration (Phases 35-38) switches `cmd/main.go` to use the SDK.

---

## Open Questions

1. **Kick Manager.Start context parameter**
   - What we know: Current signature is `func (m *Manager) Start() error` (no ctx)
   - What's unclear: Whether the kick team intended to add ctx later or deliberately excluded it
   - Recommendation: Add `ctx context.Context` parameter to kick's `Start` in the same plan as SDK-03. The ctx is ignored internally (kick uses `m.ctx` created in `NewManager`) but the method signature matches the interface. Document this in the plan.

2. **LeadershipListener env wiring for SOURCE_MANAGER_URL**
   - What we know: `sourcemanager.NewClient` requires a base URL. The env var name is not standardized — kick uses `getEnvOrDefault("SOURCE_MANAGER_URL", ...)` implicitly
   - What's unclear: Whether `SOURCE_MANAGER_URL` is the canonical env var name across all listeners
   - Recommendation: Verify from kick-listener and discord-listener cmd/main.go before implementing `LeadershipListener.fromEnv`. Use `SOURCE_MANAGER_URL` as the env var name; default to `http://source-manager:8083`.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + testify v1.11.1 + goleak v1.3.0 (new) |
| Config file | none — `go test` invoked per module |
| Quick run command | `cd shared && go test ./listener/... -v -race` |
| Full suite command | `cd shared && go test ./... -race` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SDK-01 | `ListenerBase` goroutines start on `Start()`, stop on `Stop()` with no leak | unit | `cd shared && go test ./listener/... -run TestListenerBase -race` | Wave 0 |
| SDK-01 | `ListenerBase` startup jitter: zero jitter when `StartupJitterMax=0` | unit | `cd shared && go test ./listener/... -run TestListenerBase_NoJitter -race` | Wave 0 |
| SDK-02 | `LeadershipListener` nil-safe when `SOURCE_MANAGER_SECRET` absent | unit | `cd shared && go test ./listener/... -run TestLeadershipListener_NilSafe -race` | Wave 0 |
| SDK-03 | `ChannelManager` interface satisfied by twitch manager (compile-time assertion) | compile | `cd services/twitch-listener && go build ./...` | Wave 0 |
| SDK-05 | `ListenerConfig` jitter max: `LISTENER_STARTUP_JITTER_MAX=0` → zero delay | unit | `cd shared && go test ./listener/... -run TestListenerConfig_JitterEnv -race` | Wave 0 |
| SDK-07 | `Env()` returns default when key absent, value when key set | unit | `cd shared && go test ./listener/... -run TestEnv -race` | Wave 0 |
| SDK-08 | `NewCoordinatorClient` with explicit serviceName: JWT uses correct name | unit | `cd shared && go test ./coordination/... -run TestJWTRefresh -race` | exists (needs update) |
| VERIFY-01 | `make build-all` exits 0 across all listener modules | build smoke | `make build-all` | Wave 0 (Makefile target missing) |

### Sampling Rate

- **Per task commit:** `cd shared && go test ./listener/... -race`
- **Per wave merge:** `cd shared && go test ./... -race && make build-all`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `shared/listener/` directory — does not exist yet; create with 4 files + config
- [ ] `shared/listener/testutil/mock_coordinator.go` — mock for unit tests
- [ ] `shared/listener/base_test.go` — covers SDK-01, SDK-05 (goroutine start/stop, jitter)
- [ ] `shared/listener/leadership_test.go` — covers SDK-02 (nil-safe passthrough)
- [ ] `shared/listener/env_test.go` — covers SDK-07 (Env helper)
- [ ] `shared/go.mod` — add `go.uber.org/goleak` test dependency
- [ ] `Makefile` — add `build-all` target (covers VERIFY-01)

*(Existing `shared/coordination/client_jwt_test.go` needs update to pass `serviceName` arg — not a new file, existing update.)*

---

## Sources

### Primary (HIGH confidence)

- Direct source inspection: `shared/coordination/client.go` — confirmed 3-arg `NewCoordinatorClient` signature, hostname auto-detection block at lines 48-57, `serviceName` field already on struct
- Direct source inspection: `services/twitch-listener/channels/manager.go` — confirmed all 7 interface methods present with correct signatures
- Direct source inspection: `services/kick-listener/channels/manager.go` — confirmed `Start() error` (no ctx), all other 7 methods present
- Direct source inspection: `shared/sourcemanager/coordinator.go` — confirmed nil-safe guards at `if c == nil || c.client == nil`
- Direct source inspection: `shared/go.mod` — confirmed `go.uber.org/goleak` is NOT currently in dependencies
- Direct source inspection: `root Makefile` — confirmed `build-all` target does not exist; `build` target exists but calls Docker build, not `go build ./...`
- Direct source inspection: `shared/coordination/client_jwt_test.go` — confirmed 3 call sites of `NewCoordinatorClient` that will break after SDK-08

### Secondary (MEDIUM confidence)

- `go.uber.org/goleak` v1.3.0: standard goroutine leak test library, widely used in Go projects with background goroutines; documentation at https://pkg.go.dev/go.uber.org/goleak

### Tertiary (LOW confidence)

- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod except goleak; goleak version from pkg.go.dev
- Architecture: HIGH — derived from direct reading of concrete implementations; interface extraction is provably safe
- Pitfalls: HIGH — Kick's Start() signature mismatch is a confirmed compile-time issue from source; other pitfalls are proven patterns

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable — no fast-moving external dependencies; only internal code)
