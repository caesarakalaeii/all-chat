# Phase 37: Migrate youtube-innertube and discord-listener - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate `services/youtube-listener-innertube/cmd/main.go` and `services/discord-listener/cmd/main.go` to use `LeadershipListener` from the shared SDK. These are leadership-only listeners — no ChannelManager, no assignment loops, no migration subscriber. The SDK handles env wiring for `LeadershipCoordinator` and `sourcemanager.Client` construction. No changes to domain packages (`streams/`, `gateway/`, `relay/`, etc.) beyond what is needed to accept the SDK-provided types. Both services deploy without regression.

</domain>

<decisions>
## Implementation Decisions

### SMClient exposure in LeadershipListener
- Add `SMClient() *sourcemanager.Client` accessor to `shared/listener/leadership.go` — symmetric with `LeadershipCoordinator()`
- Returns nil when `SOURCE_MANAGER_SECRET` is absent (consistent with `LeadershipCoordinator()` nil-return behavior)
- `streams.NewManager` signature unchanged; `main.go` calls `ll.LeadershipCoordinator()` and `ll.SMClient()` and passes both through as before
- `streams.Manager` already nil-checks `smClient` before calling `ActivateSource`/`GetSources` — existing nil guards cover the absent-secret case

### youtube-innertube migration pattern
- Replace manual `sourcemanager.NewSigningTokenSource` + `sourcemanager.NewClient` + `sourcemanager.NewLeadershipCoordinator` block with `listener.NewLeadershipListenerFromEnv(base, "youtube", logger)`
- `getEnv` local function deleted entirely; replaced by `listener.Env()` throughout `cmd/main.go` (same pattern as phase 35)
- All other main.go structure (HTTP server, metrics, deletion buffer, streamManager setup) unchanged

### Discord gateway goroutine
- Remove `if leaderCoord != nil` nil-guard from `EnsureLeadership` call — always call `ll.LeadershipCoordinator().EnsureLeadership(...)` (nil-safe passthrough returns `acquired=true, err=nil` when coordinator is absent)
- Keep `if ll.LeadershipCoordinator() != nil` guard specifically for `metrics.SetShardOwnership` calls — avoids spurious shard ownership metrics when leadership is disabled
- Remove the manual `log.Warn("SOURCE_MANAGER_URL or SOURCE_MANAGER_SECRET not set...")` — SDK already logs `SOURCE_MANAGER_SECRET not set — leadership coordination disabled` in `NewLeadershipListenerFromEnv`
- `getEnv` local function replaced by `listener.Env()` in discord `cmd/main.go` as well

### Shutdown ordering
- Both services keep fully custom shutdown sequences — `ShutdownCoordinator` does not apply (no ChannelManager)
- youtube-innertube: `streamManager.Shutdown(shutdownCtx)` → `deletionBuffer.Shutdown()` → `srv.Shutdown(shutdownCtx)` unchanged
- discord-listener: `signal.NotifyContext`-driven shutdown (ctx.Done()), `gwClient.Close()` + `relayMgr.Stop()` + `srv.Shutdown(shutdownCtx)` unchanged

### Test scope
- Goleak smoke test in `cmd/main_sdk_test.go` for **both** services
- Test constructs `ListenerBase` with `testutil.NewMockCoordinator()`, wraps in `LeadershipListener` via `NewLeadershipListenerFromEnv`, calls `base.Start(ctx, nil)` (no ChannelManager), then `base.Stop()`, verifies `goleak.VerifyNone` passes
- SDK-only scope — no gateway client or stream manager in the test
- No additional tests beyond the smoke test per service

### Claude's Discretion
- Exact position of `"youtube"` / `"discord-listener"` platform string passed to `NewLeadershipListenerFromEnv`
- Whether `base.Start(ctx, nil)` is valid for leadership-only services or if a no-op ChannelManager stub is needed (Claude to verify)
- Package-level doc comment updates for migrated `main.go` files

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/listener.LeadershipListener` + `NewLeadershipListenerFromEnv`: Drop-in replacement for manual `sourcemanager.LeadershipCoordinator` construction in both services
- `shared/listener.Env()`: Replaces local `getEnv` in both `cmd/main.go` files
- `shared/listener/testutil.NewMockCoordinator()`: Used in both goleak smoke tests
- `SMClient()` accessor (to be added): Exposes `*sourcemanager.Client` from `LeadershipListener` for youtube-innertube's `streams.NewManager`

### Established Patterns
- Both services already have `replace ../../shared` directives in `go.mod` — no module changes needed
- `streams.Manager` uses nil-checks for `leaderCoord` and `smClient` throughout — existing guards handle absent-secret case
- `sourcemanager.LeadershipCoordinator` methods are nil-safe on nil receiver — discord goroutine can call without nil-guard

### Integration Points
- `services/youtube-listener-innertube/streams/manager.go:NewManager` — takes `(*sourcemanager.LeadershipCoordinator, *sourcemanager.Client, ...)` — unchanged, receives values extracted from `LeadershipListener`
- `services/discord-listener/gateway.GatewayClient` — connected in a goroutine in `main.go` gated by `EnsureLeadership` — goroutine structure unchanged, just wiring simplified
- `shared/listener/leadership.go` — add `SMClient()` accessor alongside existing `LeadershipCoordinator()` accessor

</code_context>

<specifics>
## Specific Ideas

- The `SMClient()` accessor on `LeadershipListener` should mirror the `LeadershipCoordinator()` accessor exactly — same nil-return contract, same placement in `leadership.go`
- The discord gateway goroutine guard pattern after migration: `if ll.LeadershipCoordinator() != nil { metrics.SetShardOwnership(1) }` — only metrics calls are guarded, not the EnsureLeadership call itself

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 37-migrate-youtube-innertube-and-discord-listener*
*Context gathered: 2026-03-17*
