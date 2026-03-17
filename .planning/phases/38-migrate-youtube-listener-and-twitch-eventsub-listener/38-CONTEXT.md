# Phase 38: Migrate youtube-listener and twitch-eventsub-listener - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate `services/youtube-listener/cmd/main.go` to use `LeadershipListener` and `services/twitch-eventsub-listener/cmd/main.go` to use `ListenerBase`. Both migrations close the v1.6 SDK window — all 6 Go listeners run on shared SDK code simultaneously. youtube-listener has no ChannelManager (leadership-only pattern). twitch-eventsub-listener has a ChannelManager that needs interface gap fixes before the SDK can adopt it. No changes to domain packages beyond what is needed for SDK wiring.

</domain>

<decisions>
## Implementation Decisions

### twitch-eventsub: Redis leader election coexistence
- SDK `ListenerBase` handles coordinator/heartbeat/assignments boilerplate
- Custom Redis SetNX leader election (`leader:twitch-eventsub`) stays for subscription management — controls which pod registers/deletes EventSub webhooks on Twitch's API
- Two separate leadership concerns, both valid, both remain
- Redis lock TTL (10s) and renewal interval (5s) unchanged
- Leader goroutine structure unchanged: `channelManager.Start()` on acquire, `channelManager.Stop()` on loss
- Shutdown sequence unchanged: `releaseLeadership()` + `channelManager.Stop()` before HTTP server shutdown
- `getEnv` local function replaced by `listener.Env()` throughout cmd/main.go

### twitch-eventsub: ChannelManager interface gap fixes
- `Start` signature changes: `Start(ctx context.Context, interval time.Duration)` → `Start(ctx context.Context) error` (SDK interface)
- `ChannelSyncInterval` (currently 30s constant) pre-configured at construction: `channels.NewManager(..., syncInterval time.Duration)` stores it as a field; `Start(ctx)` uses stored value
- `GetFilteredAssignmentCount() int` added: returns `len(assignedSourceIDs)` (matches twitch-listener / kick-listener semantics)
- `GetActiveChannelCount() int` added: returns count of channels with active EventSub subscriptions
- `GetActiveChannels() []string` return type changed from `map[string]*Channel` to `[]string` of broadcaster IDs; old map-returning method renamed `GetActiveChannelMap()` if still needed internally, or deleted if no callers remain
- Compile-time assertion added to `channels/manager.go`: `var _ listener.ChannelManager = (*Manager)(nil)`

### youtube-listener: LeadershipListener migration
- `NewLeadershipListenerFromEnv(base, "youtube", logger)` replaces manual `sourcemanager.NewSigningTokenSource` + `sourcemanager.NewClient` + `sourcemanager.NewLeadershipCoordinator` block
- `getEnvOrDefault` local function replaced by `listener.Env()` throughout cmd/main.go; `parseIntEnv` stays (wraps Env + Atoi, not a drop-in)
- `streams.NewManager` signature unchanged; main.go extracts `ll.LeadershipCoordinator()` and passes it through — exact Phase 37 youtube-innertube pattern
- Daily quota reset goroutine stays leadership-independent: all pods reset (idempotent via PostgreSQL); no leadership gate added
- `base.Start(ctx, nil)` not called — leadership-only service (established Phase 37 pattern)

### Plan structure
- 3 plans: 38-01 (youtube-listener), 38-02 (eventsub ChannelManager gap fixes), 38-03 (eventsub ListenerBase wiring)
- youtube-listener migrated first (simpler, validates pattern before the more complex eventsub migration)
- Mixed-fleet monitoring period ends with successful Phase 38 deploy — no separate monitoring phase

### Test scope
- youtube-listener: goleak smoke test in `cmd/main_sdk_test.go` — constructs `LeadershipListener` with `testutil.NewMockCoordinator()`, verifies `goleak.VerifyNone`. No quota tracker or stream manager in test. Phase 37 pattern.
- twitch-eventsub: goleak smoke test includes ChannelManager — `base.Start(ctx, channelManager)` + `base.Stop()` + `goleak.VerifyNone`. Uses real `channels.NewManager` (or minimal stub) with mock coordinator. Matches Phase 35 twitch-listener pattern.

### Claude's Discretion
- Whether `GetActiveChannelMap()` is kept or deleted (depends on whether any internal eventsub code calls the old map-returning method)
- Exact position of `"youtube"` platform string passed to `NewLeadershipListenerFromEnv`
- Package-level doc comment updates for both migrated `cmd/main.go` files

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/listener.LeadershipListener` + `NewLeadershipListenerFromEnv`: Drop-in for manual `sourcemanager.LeadershipCoordinator` construction in youtube-listener
- `shared/listener.ListenerBase` + `NewListenerBaseFromEnv`: Replaces manual coordinator/jitter/assignments/heartbeat/migration-subscriber block in twitch-eventsub
- `shared/listener.Env()`: Replaces `getEnvOrDefault` (youtube-listener) and `getEnv` (twitch-eventsub) in both `cmd/main.go` files
- `shared/listener/testutil.NewMockCoordinator()`: Used in goleak smoke tests for both services

### Established Patterns
- youtube-listener: `leaderCoord` is passed to `streams.NewManager(...)` — after migration, `ll.LeadershipCoordinator()` is passed instead; streams package unchanged
- twitch-eventsub: `coordClient.StartJWTRefresh(ctx)` / `StopJWTRefresh()` and the assignment-refresh goroutine are replaced by `base.Start()` — SDK owns these lifecycle calls
- twitch-eventsub: Both services already have `replace ../../shared` directives in `go.mod` — no module changes needed
- goleak pinned as direct dep before .go files import it: `go mod edit -require` + `go mod download` (Phase 37 pattern)

### Integration Points
- `services/youtube-listener/streams/manager.go:NewManager` — accepts `*sourcemanager.LeadershipCoordinator`; receives value extracted from `ll.LeadershipCoordinator()` in migrated main.go
- `services/twitch-eventsub-listener/channels/manager.go` — needs 3 new methods + Start signature change before SDK can wire it
- `services/twitch-eventsub-listener/cmd/main.go` lines 563–608 — coordinator construction + jitter + assignments block replaced by `NewListenerBaseFromEnv`
- `services/twitch-eventsub-listener/cmd/main.go` lines 840–910 — migration subscriber + heartbeat + assignment refresh goroutines replaced by `base.Start(ctx, channelManager)`
- `shared/listener/channel_manager.go` — `ChannelManager` interface is the target; twitch-eventsub Manager must satisfy it after gap fixes

</code_context>

<specifics>
## Specific Ideas

- twitch-eventsub `channels.NewManager` constructor gains a `syncInterval time.Duration` parameter — callers pass `ChannelSyncInterval` (30s constant). The constant can be kept in `cmd/main.go` for documentation, even though it's no longer passed to `Start`.
- The `leaderState` struct + `tryAcquireLeadership` + `releaseLeadership` functions in twitch-eventsub `cmd/main.go` are intentionally preserved — they serve the EventSub subscription management concern, not the coordinator/SDK concern.

</specifics>

<deferred>
## Deferred Ideas

- twitch-eventsub: Replace Redis SetNX leader election with source-manager `LeadershipCoordinator` — would change the archetype to `LeadershipListener` and eliminate the `leaderState` struct entirely. Meaningful simplification but out of scope for this phase.

</deferred>

---

*Phase: 38-migrate-youtube-listener-and-twitch-eventsub-listener*
*Context gathered: 2026-03-18*
