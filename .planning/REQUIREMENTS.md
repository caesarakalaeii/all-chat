# Requirements: All-Chat

**Defined:** 2026-03-15
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## v1.5 Requirements — Discord Listener

Requirements for the Discord Listener milestone. Each maps to roadmap phases.

### AUTH — Bot Setup

- [x] **AUTH-01**: User can connect a Discord server to All-Chat via OAuth2 "Add to Server" flow
- [x] **AUTH-02**: After connecting, user can view a list of readable text channels in the connected server
- [x] **AUTH-03**: Bot permissions are validated on connect (VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES) with user-visible errors on failure
- [x] **AUTH-04**: User can disconnect the bot from their server, removing all associated Discord sources

### INBD — Inbound (Discord → Overlay)

- [x] **INBD-01**: Discord channel messages appear in overlays as a first-class chat source
- [x] **INBD-02**: Discord messages are normalized to the unified RawChatMessage schema via a discord normalizer in message-processor
- [x] **INBD-03**: Discord message deletions are propagated through the existing deletion pipeline
- [x] **INBD-04**: Discord @user and #channel mentions are resolved to human-readable names in message text

### RELY — Outbound Relay (Overlay → Discord)

- [x] **RELY-01**: Overlay messages from non-Discord sources are relayed to a configured Discord channel, with `platform == "discord"` messages unconditionally filtered to prevent loops
- [x] **RELY-02**: Each Discord source has a relay_enabled toggle so inbound-only (read-only) mode is supported
- [x] **RELY-03**: Relay target channel (outbound) is configurable per-source, can be the same or different from the inbound channel
- [x] **RELY-04**: Relayed messages are posted as plain text `[emoji] username: text`

### LOAD — Load Balancing

- [x] **LOAD-01**: Gateway shard ownership is coordinated via source-manager leader election — one pod holds each shard's connection
- [x] **LOAD-02**: discord-listener scales via HPA on Prometheus metrics (events/sec, active guilds)
- [x] **LOAD-03**: Gateway session state (session_id + resume_gateway_url) is persisted in Redis so pod restarts resume the session instead of full re-IDENTIFY

### UI — Setup UI

- [x] **UI-01**: Settings page includes a Discord server connect card showing OAuth2 flow and connected server name/icon
- [x] **UI-02**: Overlay editor allows adding a Discord source with guild selector and inbound channel dropdown (from channel listing API)
- [x] **UI-03**: Per-source relay configuration panel: toggle relay, pick outbound channel, visual indicator of active filter
- [x] **UI-04**: Discord source cards in the overlay editor display connection status and relay active/inactive indicator

---

## v1.6 Requirements — Listener SDK

Requirements for the Listener SDK milestone. Each maps to roadmap phases.

### PREP — Pre-Migration Cleanup

- [x] **PREP-01**: Source ID suffix handling is normalized across all Go listeners — Twitch and Kick agree on whether the `:platform` suffix is present or stripped before SDK extraction begins
- [x] **PREP-02**: `HandleMigrationEvent` signature is canonicalized to `func(event *coordination.MigrationEvent) error` in both Twitch and Kick channel managers and deployed before SDK extraction begins

### SDK — Shared Listener Package

- [x] **SDK-01**: `ListenerBase` struct in `shared/listener/base.go` manages the full shared lifecycle: startup jitter, CoordinatorClient construction, initial assignment query, heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine, and graceful stop
- [x] **SDK-02**: `LeadershipListener` struct in `shared/listener/leadership.go` (embeds `ListenerBase`) constructs `LeadershipCoordinator` + `SourceManagerClient` from environment, with nil-safe passthrough when `SOURCE_MANAGER_SECRET` is absent
- [x] **SDK-03**: `ChannelManager` interface in `shared/listener/channel_manager.go` defines the `Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount` contract that both Twitch and Kick channel managers satisfy
- [x] **SDK-04**: `ShutdownCoordinator` in `shared/listener/shutdown.go` implements ordered shutdown: channel manager stop + base stop (parallel) → platform disconnect → HTTP server drain with 10s timeout
- [x] **SDK-05**: `ListenerConfig` exposes configurable intervals — heartbeat interval, assignment refresh interval, startup jitter max — so each listener can override defaults and tests can disable jitter via `LISTENER_STARTUP_JITTER_MAX=0`
- [x] **SDK-06**: `ListenerConfig` includes `DisableCoordinatorFiltering bool` to preserve the operational rollback mechanism currently in twitch-listener
- [x] **SDK-07**: `shared/listener` package exposes `Env(key, defaultValue string) string` helper used by all migrated listeners (eliminates copy-paste env-with-default boilerplate)
- [x] **SDK-08**: `NewCoordinatorClient` in `shared/coordination/client.go` accepts an explicit `serviceName string` parameter replacing hostname-prefix auto-detection

### MIGRATE — Listener Migrations

- [ ] **MIGRATE-01**: twitch-listener `cmd/main.go` migrated to use `ListenerBase` — startup wiring reduced to service-specific IRC connection and message publishing only
- [ ] **MIGRATE-02**: kick-listener `cmd/main.go` migrated to use `ListenerBase` + `LeadershipListener` — both assignment and leadership archetypes exercised via SDK
- [ ] **MIGRATE-03**: youtube-listener `cmd/main.go` migrated to use `LeadershipListener` — quota tracker behavior unchanged; all existing tests pass
- [ ] **MIGRATE-04**: youtube-listener-innertube `cmd/main.go` migrated to use `LeadershipListener` — no CoordinatorClient; SDK leadership wiring is the sole integration point
- [ ] **MIGRATE-05**: discord-listener `cmd/main.go` migrated to use `LeadershipListener` — shard ownership coordination via existing Redis lock pattern unchanged
- [ ] **MIGRATE-06**: twitch-eventsub-listener `cmd/main.go` migrated to use `ListenerBase` — stateless webhook receiver gains standardized heartbeat and health wiring

### VERIFY — Build and Interface Verification

- [x] **VERIFY-01**: `make build-all` Makefile target builds all listener modules in one command, run in CI on every PR to catch `replace`-directive version drift
- [ ] **VERIFY-02**: Each migrated listener has a compile-time interface assertion (`var _ listener.ChannelManager = (*channels.Manager)(nil)`) in its `channels/manager.go` file

## Future Requirements

### Extended SDK

- **SDK-09**: Generic `ChannelManager[K comparable]` — defer until string-key convention proves insufficient
- **SDK-10**: `HealthChecker` interface in SDK for health route registration — defer until migration validates API shape
- **SDK-11**: `go.work` at repo root — `make build-all` is sufficient for v1.6; workspace adds `go mod tidy` risk

### TikTok Go Rewrite

- **TIKTOK-01**: Rewrite tiktok-listener in Go (currently Node.js) — prerequisite for SDK migration; defer to v1.7+
- **TIKTOK-02**: Migrate Go tiktok-listener to SDK — follows TIKTOK-01

## Out of Scope

| Feature | Reason |
|---------|--------|
| tiktok-listener SDK migration | Node.js service — cannot use Go SDK |
| go.work at repo root | replace directives + make build-all sufficient for v1.6 scope |
| Generic ChannelManager[K] | String-keyed convention sufficient; generics add complexity before interface is proven |
| Discord slash commands | Not a chat aggregation concern |
| Voice channel transcription | High complexity, separate domain |
| Discord DMs / private channels | Privacy concerns, not a streaming use case |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

### v1.5 Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 27 | Complete |
| AUTH-02 | Phase 27 | Complete |
| AUTH-03 | Phase 27 | Complete |
| AUTH-04 | Phase 27 | Complete |
| INBD-01 | Phase 28 | Complete |
| INBD-02 | Phase 28 | Complete |
| INBD-03 | Phase 29 | Complete |
| INBD-04 | Phase 29 | Complete |
| RELY-01 | Phase 30 | Complete |
| RELY-02 | Phase 30 | Complete |
| RELY-03 | Phase 30 | Complete |
| RELY-04 | Phase 30 | Complete |
| LOAD-01 | Phase 31 | Complete |
| LOAD-02 | Phase 31 | Complete |
| LOAD-03 | Phase 31 | Complete |
| UI-01 | Phase 32 | Complete |
| UI-02 | Phase 32 | Complete |
| UI-03 | Phase 32 | Complete |
| UI-04 | Phase 32 | Complete |

### v1.6 Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PREP-01 | Phase 33 | Complete |
| PREP-02 | Phase 33 | Complete |
| SDK-01 | Phase 34 | Complete |
| SDK-02 | Phase 34 | Complete |
| SDK-03 | Phase 34 | Complete |
| SDK-04 | Phase 34 | Complete |
| SDK-05 | Phase 34 | Complete |
| SDK-06 | Phase 34 | Complete |
| SDK-07 | Phase 34 | Complete |
| SDK-08 | Phase 34 | Complete |
| MIGRATE-01 | Phase 35 | Pending |
| MIGRATE-02 | Phase 36 | Pending |
| MIGRATE-03 | Phase 38 | Pending |
| MIGRATE-04 | Phase 37 | Pending |
| MIGRATE-05 | Phase 37 | Pending |
| MIGRATE-06 | Phase 38 | Pending |
| VERIFY-01 | Phase 34 | Complete |
| VERIFY-02 | Phase 35 | Pending |

**Coverage:**
- v1.6 requirements: 18 total
- Mapped to phases: 18/18 ✓
- Unmapped: 0

---
*Requirements defined: 2026-03-15 (v1.5), 2026-03-17 (v1.6)*
*Last updated: 2026-03-17 after v1.6 roadmap creation*
