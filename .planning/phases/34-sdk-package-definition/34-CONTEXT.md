# Phase 34: SDK Package Definition - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Create the `shared/listener` Go package with four new files (base.go, leadership.go, channel_manager.go, shutdown.go), a `ListenerConfig` struct with configurable intervals, an `Env()` helper, and change `NewCoordinatorClient` to accept an explicit `serviceName string` parameter. Also create a `shared/listener/testutil/` package with a reusable mock coordinator. The `shared/listener` package must compile and all listener services must continue to build. No listener migrations happen in this phase — that is Phases 35-38.

</domain>

<decisions>
## Implementation Decisions

### Goroutine error propagation
- `ListenerConfig` includes `OnFatalError func(source string, err error)` callback
- When a background goroutine (heartbeat, assignment refresh, migration subscriber) fails permanently, call `OnFatalError(sourceName, err)` — `sourceName` is the goroutine name (e.g. `"heartbeat"`, `"assignment-refresh"`, `"migration-subscriber"`)
- If `OnFatalError` is nil, goroutine retries indefinitely with backoff — no escalation (silent mode)
- This preserves backward compatibility: listeners that don't set the callback keep the current log-and-retry behavior

### Default interval values
- Heartbeat interval: **30 seconds** (matches v1.1 battle-tested production value)
- Assignment refresh interval: **10 seconds** (good balance for rebalancing responsiveness)
- Startup jitter max: **30 seconds** (matches v1.1 implementation, `LISTENER_STARTUP_JITTER_MAX=0` disables it in tests)

### ChannelManager interface scope
- Interface includes **7 methods**: the 5 required by SDK-03 (`Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`) plus 2 read methods (`GetActiveChannels() []string`, `GetActiveChannelCount() int`)
- Reasoning: health handlers in migrated listeners may query channel state via the interface without knowing the concrete type
- Compile-time assertions (`var _ listener.ChannelManager = (*channels.Manager)(nil)`) live in each listener's `channels/manager.go` — assertion is about the concrete type, lives with the implementation

### Test mock design
- Mock is **behavioral with failure modes** — can simulate: coordinator down, 401 auth, request timeout
- Mock lives in `shared/listener/testutil/` package — reusable by all listener migration phases (35-38) without code duplication
- Mock **tracks call counts** — `mock.HeartbeatCallCount()`, `mock.AssignmentCallCount()` etc. allow tests to assert goroutines actually fired (not just started)
- Primary test assertion: goroutines start on `Start()` and stop on `Stop()` with `goleak.VerifyNone` passing

### Claude's Discretion
- Exact retry backoff strategy for goroutines (exponential with jitter is standard)
- Internal HTTP client timeout values for coordinator calls
- Whether `ShutdownCoordinator` accepts a context or uses a fixed 10s timeout internally (requirements specify 10s)
- Package-level doc comments and exported type documentation

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/coordination/client.go`: `CoordinatorClient` — SDK wraps its construction; `NewCoordinatorClient` signature changes to accept explicit `serviceName string` (SDK-08)
- `shared/coordination/migration_subscriber.go`: `MigrationSubscriber` — already exists, SDK wires its lifecycle
- `shared/coordination/models.go`: `MigrationEvent` — used in `ChannelManager.HandleMigrationEvent` signature
- `shared/sourcemanager`: `LeadershipCoordinator`, `SourceManagerClient` — `LeadershipListener` constructs these from env
- `services/twitch-listener/channels/manager.go`: Existing `Manager` — satisfies the new `ChannelManager` interface; `GetActiveChannels`, `GetActiveChannelCount`, `GetFilteredAssignmentCount`, `UpdateAssignedSourceIDs`, `HandleMigrationEvent` all present
- `services/kick-listener/channels/manager.go`: Same pattern, second reference implementation

### Established Patterns
- Standard Go Layout: `cmd/main.go` for entry points, packages for domain logic
- Dependency injection via constructor functions (`NewXxx(deps...) *Xxx`)
- `go.uber.org/zap` structured logging — pass logger into SDK structs
- Explicit `context.Context` for cancellation in `Start(ctx context.Context) error`
- `getEnvOrDefault(key, default)` pattern in every listener — SDK-07 `Env()` helper replaces this

### Integration Points
- `shared/go.mod` — new `shared/listener` package lives here (no new module)
- Each listener's `go.mod` already has a `replace` directive pointing to `../shared` — new package is automatically available
- `shared/coordination/client.go` signature change (`NewCoordinatorClient` + `serviceName string` param) — all current callers must be updated in the same phase

</code_context>

<specifics>
## Specific Ideas

- The `NewCoordinatorClient` hostname-prefix auto-detection block (the `if strings.HasPrefix(hostname, ...)` switch in client.go:50-60) is replaced by the explicit `serviceName string` parameter — callers pass their known service name, no more hostname parsing
- `Env(key, defaultValue string) string` is a package-level function (not a method), usable as a drop-in for the `getEnvOrDefault` helper that is copy-pasted in every listener today

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 34-sdk-package-definition*
*Context gathered: 2026-03-17*
