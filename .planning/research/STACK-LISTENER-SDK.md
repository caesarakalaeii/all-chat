# Technology Stack: Listener SDK Shared Library

**Project:** All-Chat v1.6 — Listener SDK
**Researched:** 2026-03-17
**Scope:** Stack additions and patterns required for extracting a shared Go SDK into `/shared/listener` that all platform listeners import.

---

## Context: What Already Exists (Do Not Re-Introduce)

Verified from actual go.mod files in this repository. The SDK lives in the `shared` module at `github.com/caesar/all-chat/shared` and all listeners already import it via `replace` directives.

| Dependency | Version | Source |
|------------|---------|--------|
| Go | 1.25.6 | All go.mod files |
| `redis/go-redis/v9` | v9.18.0 | All listeners |
| `go.uber.org/zap` | v1.27.1 | All listeners |
| `prometheus/client_golang` | v1.23.2 | All listeners |
| `go.opentelemetry.io/otel` | v1.42.0 | All listeners |
| `google/uuid` | v1.6.0 | twitch-listener, shared/sourcemanager |
| `stretchr/testify` | v1.11.1 | twitch-listener, youtube-listener |
| `gin-gonic/gin` | v1.12.0 | All services (HTTP/health) |
| `golang.org/x/time` | v0.15.0 | twitch-listener (rate limiter) |
| `golang-jwt/jwt/v5` | v5.3.1 | shared/auth, coordination client |
| `jackc/pgx/v5` | v5.8.0 | All listeners with DB access |

The shared SDK already has all of these in `shared/go.mod`. No new runtime dependencies are needed.

---

## New Dependencies: None Required

The Listener SDK extraction introduces **zero new runtime dependencies**. Every dependency the SDK needs is already in `shared/go.mod`:

- Startup jitter: `math/rand` (stdlib)
- Backoff retry: manual implementation already in `coordination/client.go` (1s→2s→4s→8s→16s→30s)
- Assignment query + heartbeat: `shared/coordination` (already exists)
- Migration subscriber: `shared/coordination` (already exists)
- Leadership coordination: `shared/sourcemanager` (already exists)
- Metrics: `shared/metrics` (already exists)
- Logging: `go.uber.org/zap` (already in shared/go.mod)

**Confidence:** HIGH — verified by reading all listener go.mod files and shared/go.mod directly.

---

## Module Organization: Replace Directives (Keep Current Pattern)

### Decision: Keep `replace` directives, do NOT introduce `go.work`

The repository already uses `replace` directives in each service's `go.mod` to point at `../../shared`. This pattern is working and correct. Do not migrate to `go.work`.

**Rationale:**

| Concern | Replace Directives | go.work |
|---------|-------------------|---------|
| CI/CD reproducibility | Each module builds independently — `go build ./...` works without workspace | go.work is ignored in production builds (GOWORK=off default in many CI setups); can cause confusion |
| Committed to VCS | Already in .gitignore (`go.work` line confirmed in root .gitignore) — would need team convention if introduced | Typically not committed; each dev sets up locally |
| New module (shared/listener) | Add `replace github.com/caesar/all-chat/shared => ../../shared` to new listener go.mod — same as existing pattern | Would need go.work updated |
| Discoverability | Explicit in each go.mod — any developer sees exactly what's local | Requires knowing to run `go work init` |
| Existing tooling (make, Docker) | Already works | Requires `GOWORK=off` for Docker builds |

**The only reason to adopt go.work in a monorepo is DX convenience when developing multiple modules simultaneously.** This project already has that solved: Docker Compose mounts source directories and services rebuild on change. The replace directive pattern costs nothing.

**What changes when adding shared/listener sub-package:**

The new `shared/listener` package lives inside the existing `shared` module (`github.com/caesar/all-chat/shared`). It is NOT a new module — it is a new directory within the existing module. No new `go.mod` is needed and no new `replace` directives are needed. All six listener services already import `github.com/caesar/all-chat/shared` and will automatically resolve the new sub-package.

**Confidence:** HIGH — module path `github.com/caesar/all-chat/shared/listener` resolves within the existing `shared` module. Confirmed by reading `shared/go.mod` (module declaration at line 1) and all listener `replace` directives.

---

## SDK Ergonomics: Struct Embedding, Not Interface-Only

### Decision: `ListenerBase` as an embedded struct with a required `PlatformHandler` interface

The SDK should expose a concrete `ListenerBase` struct that callers embed, combined with a `PlatformHandler` interface that callers implement for their platform-specific logic.

```go
// In shared/listener/base.go

// PlatformHandler is what each listener implements — platform-specific logic only.
type PlatformHandler interface {
    // Platform-specific channel connect/disconnect
    Connect(ctx context.Context, channel string) error
    Disconnect(ctx context.Context, channel string) error
    // Called after all assignments are loaded and the listener is ready
    OnReady(ctx context.Context) error
}

// Config is passed to NewListenerBase — no functional options needed at this scale.
type Config struct {
    ServiceName    string
    PodID          string
    CoordinatorURL string
    ServiceJWT     string
    Platform       string
    Logger         *zap.Logger
    RedisClient    *redis.Client
    // Optional: override defaults
    HeartbeatInterval  time.Duration // default 10s
    AssignmentInterval time.Duration // default 60s
    StartupJitterMax   time.Duration // default 30s
}

// ListenerBase wires the full startup sequence.
// Embed this in your platform-specific listener struct.
type ListenerBase struct {
    config Config
    // ... internal fields
}
```

**Rationale for embed over pure interface:**

- The startup sequence (jitter → query assignments → start migration subscriber → start heartbeat → start assignment refresh) is identical across all 6 listeners. That is ~150 lines of `cmd/main.go` that should run once from the SDK, not be re-implemented.
- Go struct embedding promotes `ListenerBase` methods to the embedding struct — `l.Start(ctx)` calls the SDK's `Start`, which internally calls `l.handler.OnReady(ctx)` for the platform hook.
- Interface-only approaches require each listener to re-implement the orchestration loop, which defeats the purpose of the SDK.
- Pure embedding without an interface produces tight coupling — the `PlatformHandler` interface keeps the boundary explicit.

**Rationale for `Config` struct over functional options:**

Functional options add syntactic overhead that is only worth it when the option space is large (>8 configurable fields) or when options must be shared across packages. The SDK has a bounded, stable config set. A plain struct is simpler to read, simpler to test, and simpler to validate (all required fields visible at a glance).

**Confidence:** HIGH — this pattern is idiomatic Go SDK design ("accept interfaces, return structs") and directly matches the existing codebase's style (`NewCoordinatorClient`, `NewLeadershipCoordinator`, `NewManager` all take plain config args).

---

## Leadership Variant: `LeadershipListenerBase`

### Decision: Separate type via embedding, not a boolean flag

Listeners that use the stream-ownership model (YouTube, Kick, YouTube-InnerTube, Discord) need `sourcemanager.LeadershipCoordinator` wired in. Rather than a `UseLeadership bool` flag on `ListenerBase`, expose a `LeadershipListenerBase` that embeds `ListenerBase` and adds the leadership lifecycle:

```go
// LeadershipListenerBase extends ListenerBase for stream-per-pod ownership.
type LeadershipListenerBase struct {
    ListenerBase
    leaderCoord *sourcemanager.LeadershipCoordinator
}
```

**Why:** Twitch-listener explicitly sets `leaderCoord = nil` today (confirmed in `cmd/main.go` line 179). A boolean flag creates dead code paths and confusion. Two types with clear names are self-documenting. The embedding means all `ListenerBase` methods are still available.

**Confidence:** HIGH — confirmed from reading twitch-listener `cmd/main.go` (nil leader coord explicitly commented) and `shared/sourcemanager/coordinator.go` (nil-safe guard on all methods already).

---

## Shared ChannelManager: Interface-Driven Extraction

### Decision: Extract into `shared/listener/channels` with platform-specific hooks via interface

The twitch-listener `channels/manager.go` (741 lines) and kick-listener `channels/manager.go` (1011 lines) share: assignment filtering, migration event handling, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`, `GetAssignmentCount`, heartbeat-adjacent state. Platform-specific parts: join/part mechanism (IRC vs WebSocket), channel-to-source-ID mapping (slug vs UUID), leadership calls, chatroom ID resolution (Kick-specific).

The right boundary:

```go
// ChannelOps is what the shared ChannelManager calls into the platform for.
type ChannelOps interface {
    // AddChannel connects to the platform channel. Called after assignment filter passes.
    AddChannel(ctx context.Context, sourceID, channelIdentifier string) error
    // RemoveChannel disconnects. Called during migration or sync.
    RemoveChannel(ctx context.Context, sourceID, channelIdentifier string) error
    // ChannelIdentifier maps a sourceID to its platform-specific string (slug, username, etc.)
    ChannelIdentifier(ctx context.Context, sourceID string) (string, error)
}
```

The shared `ChannelManager` owns: assignment map, migration event protocol, first-message channel map, `UpdateAssignedSourceIDs`, filtering, and `SyncChannels` orchestration. The platform provides `ChannelOps`.

**Confidence:** MEDIUM — the two channel managers share the migration protocol verbatim (both have `handleMigrationAsNewPod`, `handleMigrationAsOldPod`, `publishMigrationConfirmation`) which is the strongest signal for extraction. The platform-specific surface is small and well-defined. However, kick's `channels/manager.go` is 270 lines longer due to chatroom ID resolution logic; verification during implementation will be needed to confirm the interface boundary is sufficient.

---

## Testing Shared SDK Packages

### Decision: Test in `shared/listener` package directly; consumer tests use hand-rolled mocks

**Pattern:**

1. Unit tests for `ListenerBase` and `ChannelManager` live in `shared/listener/*_test.go` — same module, same package or `_test` suffix for black-box.
2. Each listener's test mocks the `PlatformHandler` and `ChannelOps` interfaces by hand (as twitch-listener already does for `JoinParterInterface` and `RepositoryInterface` in `channels/manager_test.go`).
3. No shared test helpers package needed at this point — the interfaces are small enough that each consumer defines its own mock in 20-40 lines.
4. Use `go test ./shared/listener/...` from the repo root to run all SDK tests (works because `shared` is a single module).

**`testify/mock` vs hand-rolled:**

The existing codebase uses hand-rolled mocks (confirmed: `MockJoinParter` in twitch-listener). Introducing `testify/mock` codegen (mockery) is a separate tooling decision that is out of scope for the SDK itself. The SDK tests should follow the existing convention: define a `fakeHandler` struct in the `_test.go` file that implements `PlatformHandler`.

**`export_test.go` is not needed** here because the SDK's public API surface is what gets tested — there are no internal methods that need white-box exposure for consumer tests.

**Confidence:** HIGH — the pattern matches exactly what already exists in `shared/coordination/client_jwt_test.go` (tests internal JWT state via exported-enough fields) and `services/twitch-listener/channels/manager_test.go` (hand-rolled mock implements interface).

---

## What NOT to Add

| Temptation | Why to Avoid |
|------------|-------------|
| `go.work` file | Already gitignored; replace directives work; adds CI complexity for no DX gain given Docker Compose dev setup |
| `testify/mock` + `mockery` codegen | Out of scope; existing hand-rolled mock pattern is consistent and sufficient |
| Generic constraints on `ListenerBase[T PlatformHandler]` | Go generics add complexity without benefit here — the interface dispatch is fine and makes zero-allocation concerns irrelevant for a startup-once lifecycle |
| Separate `go.mod` for `shared/listener` | The SDK should live inside the existing `shared` module; a new module would require another `replace` directive in every listener |
| `buraksezer/consistent` in SDK | Already in `source-manager` and `source-manager`'s logic; listeners consume assignments via HTTP from coordinator, they do not run the consistent hash ring themselves |
| Dependency injection framework (wire, dig) | Overkill; the Config struct + constructor pattern already used is idiomatic and sufficient |
| `golang.org/x/sync/errgroup` | The startup sequence is sequential-then-concurrent; the existing `go func()` + `chan os.Signal` pattern in `cmd/main.go` is correct and should be preserved in `ListenerBase.Start()` |

---

## Integration Points with Existing go.mod Files

When a service's `cmd/main.go` migrates to use `ListenerBase`:

1. **Import path:** `github.com/caesar/all-chat/shared/listener` — already resolvable via the existing `replace github.com/caesar/all-chat/shared => ../../shared` directive in each service's `go.mod`. No go.mod changes needed.

2. **Services that need both `ListenerBase` AND platform-specific packages:** The embed pattern means `cmd/main.go` imports `shared/listener` plus its own platform packages. The separation is: SDK wires lifecycle, service wires platform connection.

3. **Services without a `go.mod` `replace` for shared yet:** All six target listeners already have the `replace` directive (verified: twitch-listener, kick-listener, youtube-listener, tiktok-listener all confirmed). youtube-listener-innertube and discord-listener inherit the pattern.

---

## Sources

- [Go Modules Reference — replace directive](https://go.dev/ref/mod) — HIGH confidence, official documentation
- [Go Workspace proposal #53502 — whether to commit go.work](https://github.com/golang/go/issues/53502) — HIGH confidence, official Go issue tracker
- [How to Use Go Workspaces for Monorepos (Feb 2026)](https://oneuptime.com/blog/post/2026-02-01-go-workspaces-monorepos/view) — MEDIUM confidence, current blog post
- [Embedding in Go: Part 1 - structs in structs (Eli Bendersky)](https://eli.thegreenplace.net/2020/embedding-in-go-part-1-structs-in-structs/) — HIGH confidence, authoritative Go explanation
- [Embedding in Go: Part 3 - interfaces in structs (Eli Bendersky)](https://eli.thegreenplace.net/2020/embedding-in-go-part-3-interfaces-in-structs/) — HIGH confidence
- [Go Unit Testing: Structure & Best Practices (Dec 2025)](https://www.glukhov.org/post/2025/11/unit-tests-in-go/) — MEDIUM confidence, current
- [Advanced unit testing patterns in Go (LogRocket)](https://blog.logrocket.com/advanced-unit-testing-patterns-go/) — MEDIUM confidence
- Codebase evidence: `shared/go.mod`, `services/twitch-listener/go.mod`, `services/kick-listener/go.mod`, `shared/coordination/client.go`, `shared/sourcemanager/coordinator.go`, `services/twitch-listener/cmd/main.go`, `services/twitch-listener/channels/manager.go`, `services/kick-listener/channels/manager.go` — HIGH confidence, direct inspection
