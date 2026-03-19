# Phase 35: Migrate twitch-listener - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Refactor `services/twitch-listener/cmd/main.go` to use `ListenerBase` from the shared SDK. All coordinator wiring (startup jitter, assignment query, heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine) is removed from `cmd/main.go` and delegated to the SDK. Add compile-time interface assertion to `channels/manager.go`. No changes to channels.Manager logic beyond the assertion. No other listener migrations happen in this phase.

</domain>

<decisions>
## Implementation Decisions

### IRC startup sequencing
- IRC-specific setup (`ircConn.Connect()`, `ircConn.SetFirstMessageChan(channelMgr.GetFirstMessageChan())`, and the `time.Sleep(2 * time.Second)`) happens explicitly in `main.go` **before** calling `base.Start()`
- The 2-second sleep stays in main.go — it is IRC-specific behavior needed to let the connection establish before channels are joined
- `base.Start()` receives an already-connected and wired channel manager; SDK is not involved in IRC connection setup
- Shutdown uses `ShutdownCoordinator` from the SDK: `ircConn.Disconnect` is passed as the platform disconnect callback, SDK handles ordering (channelMgr.Stop + base.Stop in parallel → platform disconnect → HTTP drain)

### ENABLE_COORDINATOR_FILTERING flag
- Env check stays in `main.go`: reads `ENABLE_COORDINATOR_FILTERING`, sets `ListenerConfig{DisableCoordinatorFiltering: !enableFiltering}`
- `channels/manager.go` is not modified beyond adding the compile-time assertion — all filtering logic stays exactly as-is
- This preserves the operational rollback knob and guarantees existing unit tests pass without modification

### JWT refresh lifecycle
- JWT refresh moves fully inside the SDK: `ListenerBase.Start()` calls `coordClient.StartJWTRefresh()`, `ListenerBase.Stop()` calls `StopJWTRefresh()`
- `main.go` does not create or interact with the coordinator client directly after migration

### Heartbeat interval
- Adopt the SDK default of 30 seconds (battle-tested value from Phase 34 context) — no override in `ListenerConfig`
- The prior 10-second hardcoded value in the old goroutine was arbitrary; 30s is the correct production value

### Test scope
- Existing unit tests pass without modification (no changes to channels/manager.go logic)
- One new integration smoke test added in `cmd/` (or `cmd/main_test.go`): constructs `ListenerBase` with `testutil` mock coordinator, calls `Start()`, then `Stop()`, verifies `goleak.VerifyNone` passes — confirms no goroutine leaks from the SDK wiring
- No additional tests beyond the smoke test

### Claude's Discretion
- Exact position of `service-name` string passed to `listener.Env()` or `NewCoordinatorClient` (use `"twitch-listener"`)
- Whether tracing middleware wiring and HTTP server setup remain in `main.go` (yes — service-specific)
- Package-level doc comment for the migrated `main.go`

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/listener.ListenerBase`: Replaces the jitter, assignment query, heartbeat goroutine, assignment refresh goroutine, and migration subscriber in `cmd/main.go`
- `shared/listener.ShutdownCoordinator`: Replaces the manual shutdown sequence; `ircConn.Disconnect` becomes the platform disconnect callback
- `shared/listener.ListenerConfig`: Carries `DisableCoordinatorFiltering`, `OnFatalError`, and interval overrides
- `shared/listener/testutil`: Mock coordinator for the new smoke test — `testutil.NewMockCoordinator()` satisfies the internal coordinator interface
- `listener.Env()` helper: Drop-in replacement for `getEnvOrDefault` — can be adopted throughout `cmd/main.go` to remove the local function

### Established Patterns
- `channels/manager.go` already satisfies all 7 `listener.ChannelManager` interface methods — no method additions needed
- `irc.ConnectionManager` implements `channels.JoinParterInterface` — this type relationship is unchanged
- `channels.NewManager` constructor signature unchanged — still accepts the same args; `assignedSourceIDs` may be nil (when filtering disabled)

### Integration Points
- `services/twitch-listener/go.mod` already has `replace ../shared` directive — `shared/listener` package is available without `go.mod` changes
- `channels/manager.go` line for compile-time assertion: `var _ listener.ChannelManager = (*Manager)(nil)` (import `shared/listener`)
- New smoke test file: `services/twitch-listener/cmd/main_sdk_test.go` (or similar) — uses `testutil.NewMockCoordinator()` + `testutil.NewMockChannelManager()`

</code_context>

<specifics>
## Specific Ideas

- The `getEnvOrDefault` local function can be replaced wholesale by `listener.Env()` — the signatures are identical. Adopt it across all env reads in the migrated `main.go` so the local function is deleted entirely.
- The `dbConnWrapper` struct at the bottom of the current `main.go` is service-specific infrastructure (needed by channels.NewManager) — it stays in `main.go` unchanged.
- `shardMetrics.PodChannelCount` metric recording after `base.Start()` stays in `main.go` — it's service-specific observability, not SDK responsibility.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 35-migrate-twitch-listener*
*Context gathered: 2026-03-17*
