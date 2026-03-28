---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: Milestone complete
stopped_at: Completed 06-03-PLAN.md — Phase 06 complete
last_updated: "2026-03-27T23:10:32.498Z"
last_activity: 2026-03-27
progress:
  total_phases: 11
  completed_phases: 11
  total_plans: 48
  completed_plans: 48
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-18)

**Core value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.
**Current focus:** Phase 06 complete — unify-all-listeners-to-leadership-based-coordination

## Current Position

Phase: 06 (complete)
Plan: 3/3 complete

## Performance Metrics

**Velocity (prior milestones):**

- v1.0: 11 plans (3 phases)
- v1.1: 21 plans (7 phases)
- v1.2: 21 plans (12 phases)
- v1.3: 20 plans (4 phases)
- v1.5: 16 plans (6 phases)

**By Milestone:**

| Milestone | Phases | Plans | Status |
|-----------|--------|-------|--------|
| v1.0 Message Deletion | 1-3 | 11 | Complete |
| v1.1 Load Balancing | 4-10 | 21 | Complete |
| v1.2 InnerTube Listener | 11-22 | 21 | Complete |
| v1.3 Frontend Redesign | 23-26 | 20 | Complete |
| v1.5 Discord Listener | 27-32 | 16 | Complete |
| v1.6 Listener SDK | 33-38 | TBD | Not started |
| Phase 33 P01 | 107 | 2 tasks | 3 files |
| Phase 33-pre-migration-cleanup P02 | 4 | 2 tasks | 5 files |
| Phase 34 P01 | 4m | 3 tasks | 8 files |
| Phase 34 P02 | 15m | 3 tasks | 8 files |
| Phase 34 P03 | 10 | 3 tasks | 3 files |
| Phase 35-migrate-twitch-listener P01 | 3m | 2 tasks | 3 files |
| Phase 35-migrate-twitch-listener P02 | 5m | 3 tasks | 2 files |
| Phase 36-migrate-kick-listener P01 | 5m | 2 tasks | 3 files |
| Phase 36-migrate-kick-listener P02 | 3m | 2 tasks | 2 files |
| Phase 37-migrate-youtube-innertube-and-discord-listener P01 | 2m | 2 tasks | 5 files |
| Phase 37 P03 | 3min | 2 tasks | 2 files |
| Phase 37-migrate-youtube-innertube-and-discord-listener P02 | 5m | 2 tasks | 2 files |
| Phase 38-migrate-youtube-listener-and-twitch-eventsub-listener P01 | 10min | 2 tasks | 3 files |
| Phase 38-migrate-youtube-listener-and-twitch-eventsub-listener P02 | 8min | 2 tasks | 2 files |
| Phase 38 P03 | 4min | 2 tasks | 3 files |
| Phase 01 P01 | 3 | 2 tasks | 11 files |
| Phase 01 P02 | 248s | 2 tasks | 5 files |
| Phase 01 P03 | 3min | 1 tasks | 3 files |
| Phase 02 P03 | 1min | 2 tasks | 3 files |
| Phase 02 P01 | 3min | 1 tasks | 4 files |
| Phase 02 P02 | 15min | 1 tasks | 4 files |
| Phase 03 P01 | 282 | 1 tasks | 5 files |
| Phase 03 P02 | 489 | 2 tasks | 7 files |
| Phase 03 P03 | 1min | 1 tasks | 1 files |
| Phase 03 P03 | 1min | 2 tasks | 1 files |
| Phase 04 P01 | 658 | 2 tasks | 3 files |
| Phase 04 P01 | 65min | 3 tasks | 3 files |
| Phase 04 P02 | 34min | 3 tasks | 10 files |
| Phase 04 P03 | 38min | 2 tasks | 10 files |
| Phase 04 P05 | 310s | 2 tasks | 1 files |
| Phase 04 P04 | 362s | 2 tasks | 1 files |
| Phase 05 P01 | 226s | 2 tasks | 5 files |
| Phase 05 P02 | 1622 | 2 tasks | 16 files |
| Phase 05 P03 | 380 | 2 tasks | 6 files |
| Phase 05 P05 | 4min | 2 tasks | 6 files |
| Phase 06 P01 | 15min | 1 tasks | 18 files |
| Phase 06 P02 | 10min | 2 tasks | 4 files |
| Phase 06 P03 | 381 | 2 tasks | 13 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Key decisions relevant to v1.6:

- SDK lives in `shared/listener/` — four new files in existing shared module; no new Go modules or go.work required
- Two SDK archetypes: `ListenerBase` (assignment-based: Twitch, YouTube, twitch-eventsub) and `LeadershipListener` embeds ListenerBase (per-stream ownership: Kick, InnerTube, Discord)
- Pre-migration cleanup is a hard prerequisite — source ID suffix inconsistency and HandleMigrationEvent signature mismatch must be fixed before SDK code is written
- Migration order: twitch → kick → (innertube + discord in parallel) → (youtube + twitch-eventsub) — simplest first, one listener at a time
- tiktok-listener explicitly excluded — Node.js service cannot use the Go SDK
- `make build-all` CI target (not go.work) chosen for monorepo-wide compile verification — avoids go mod tidy side-effect risks
- [Phase 33]: Strip platform suffix at intake in cmd/main.go (not inside Manager) to keep channels.Manager interface simple; bare-UUID maps passed to NewManager and UpdateAssignedSourceIDs in both kick-listener and twitch-listener
- [Phase 33-pre-migration-cleanup]: HandleMigrationEvent returns nil unconditionally in both managers — error return is a forward-compatible slot for future SDK-defined fatal conditions
- [Phase 33-pre-migration-cleanup]: consumeMessages retains panic recovery defer alongside new error-logging — two separate safety nets for panic vs application error
- [Phase 34]: serviceName passed explicitly by each listener caller — no hostname auto-detection in NewCoordinatorClient
- [Phase 34]: ChannelManager interface defined in shared/listener with 7 methods; compile-time assertions deferred to Phase 35
- [Phase 34]: coordinatorClient is a private interface in base.go — enables mock injection without public API surface
- [Phase 34]: LeadershipListener uses sourcemanager.NewSigningTokenSource (15min TTL) — matches kick-listener production pattern
- [Phase 34]: mockChannelManager defined in test file — satisfies ChannelManager interface without adding public SDK surface
- [Phase 35-migrate-twitch-listener]: goleak placed in direct require block (not indirect) — forward dep for plan 02 smoke test before any .go file imports it
- [Phase 35-migrate-twitch-listener]: Compile-time assertion var _ listener.ChannelManager = (*Manager)(nil) added to channels/manager.go — build fails immediately if Manager drifts from 7-method SDK interface
- [Phase 35-migrate-twitch-listener]: listener.Env used as drop-in for getEnvOrDefault — local helper deleted entirely
- [Phase 35-migrate-twitch-listener]: nil passed to channels.NewManager for assignedSourceIDs — SDK owns via UpdateAssignedSourceIDs inside base.Start
- [Phase 36-migrate-kick-listener]: goleak placed in direct require block (not indirect) — forward dep for plan 02 smoke test before any .go file imports it
- [Phase 36-migrate-kick-listener]: Compile-time assertion var _ listener.ChannelManager = (*Manager)(nil) added to kick-listener channels/manager.go — build fails immediately if Manager drifts from 7-method SDK interface
- [Phase 36-migrate-kick-listener]: nil passed to NewListenerBase for logger in smoke test — matches established twitch-listener smoke test pattern from Phase 35
- [Phase 37-migrate-youtube-innertube-and-discord-listener]: go mod edit -require + go mod download used to pin goleak as direct dep before any .go imports it — avoids go mod tidy stripping unused dep
- [Phase 37-migrate-youtube-innertube-and-discord-listener]: SMClient() accessor mirrors LeadershipCoordinator() nil-safety pattern — doc comment warns callers to nil-check
- [Phase 37]: discord-listener is leadership-only: ll.Start() and ll.Stop() NOT called — ListenerBase used as container only for NewLeadershipListenerFromEnv; custom shutdown sequence unchanged
- [Phase 37]: Gateway goroutine outer nil guard removed — EnsureLeadership called unconditionally via nil-safe passthrough; only metrics.SetShardOwnership calls remain guarded
- [Phase 37-migrate-youtube-innertube-and-discord-listener]: nil passed for logger in NewListenerBase smoke test — matches kick-listener pattern (not zap.NewNop())
- [Phase 37-migrate-youtube-innertube-and-discord-listener]: ListenerBase used as container only in youtube-innertube production main.go — Start/Stop not called for leadership-only service
- [Phase 38-migrate-youtube-listener-and-twitch-eventsub-listener]: [Phase 38-01]: listener.Env used as drop-in for getEnvOrDefault — local helper deleted entirely; base.Start not called for leadership-only youtube-listener; parseIntEnv preserved for 4 quota tier config call sites
- [Phase 38-migrate-youtube-listener-and-twitch-eventsub-listener]: [Phase 38-02]: syncInterval stored as Manager field, passed to NewManager as 5th arg; ChannelSyncInterval constant stays in cmd/main.go as documentation
- [Phase 38-migrate-youtube-listener-and-twitch-eventsub-listener]: [Phase 38-02]: Old map-returning GetActiveChannels renamed to GetActiveChannelMap; new GetActiveChannels() []string satisfies SDK interface without breaking callers
- [Phase 38]: coordClient still built manually with coordination.NewCoordinatorClient — NewListenerBaseFromEnv does not exist in SDK; all other migrated services use the same direct pattern
- [Phase 38]: [Phase 38-03]: base.Start called before leader election goroutine — ensures UpdateAssignedSourceIDs is available when channelManager.Start fires on leadership acquisition
- [Phase 01-01]: Use execa subprocess (claude -p) not @anthropic-ai/claude-agent-sdk — reuses user's Claude.ai subscription via CLAUDE_CODE_OAUTH_TOKEN
- [Phase 01-01]: allowedTools restricted to Read,Glob,Grep only — bot can never write/edit code
- [Phase 01-01]: PROPOSE_ISSUE:repo|||title|||body protocol for structured issue creation from LLM output
- [Phase 01]: Mention regex uses \d+ (numeric snowflake IDs only) — Discord IDs are always numeric
- [Phase 01]: sendResponse standalone function taking channel — separates message handler from editReply slash command path
- [Phase 01]: InteractionCreate calls fetchReply before editReply to get message reference for thread creation
- [Phase 01-03]: npm ci (not --production) in Dockerfile — tsx is a devDependency but required at runtime for TypeScript execution
- [Phase 01-03]: Init containers use $GITHUB_TOKEN shell variable syntax inside sh -c — NOT $(GITHUB_TOKEN) which is Kubernetes command substitution
- [Phase 01-03]: emptyDir volumes for cloned repos with readOnly: true mounts — ephemeral fresh clone on each pod start, bot reads only
- [Phase quick-260325-lwo]: Webhook name 'AllChat Relay' as idempotency key; pg_notify after StoreWebhookURL for immediate sync
- [Phase 02]: kubectl installed via apk curl + install to /usr/local/bin before USER node; mcp-grafana via tar.gz extraction with 0755 permissions — both available to non-root node user at runtime
- [Phase 02]: RBAC split into two roles: support-bot-secret-patcher (existing write to secrets) and support-bot-cluster-reader (new read-only to pods/events/deployments/replicasets/metrics)
- [Phase 02-01]: Read GRAFANA_URL and GRAFANA_SERVICE_ACCOUNT_TOKEN directly from process.env in agent.ts (not passed as function params) — avoids signature change, keeps callers clean
- [Phase 02-01]: Bash(kubectl:*) always included in allowedTools (not conditional) — infra checks always useful even without Grafana
- [Phase 02-01]: INFRA_VERDICT stripped from answer before PROPOSE_ISSUE is parsed — correct ordering since INFRA_VERDICT appears at end of response
- [Phase 02]: shouldPingLeadDev is a derived boolean evaluated after issue creation so one mention covers both infraVerdict and issueProposal conditions
- [Phase 02]: enqueue returns Promise<void> instead of void for testable async Discord event handlers — Discord.js ignores return value
- [Phase 02]: [Phase 02-02]: LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN moved to required env var Record — fail-fast consistent with other vars
- [Phase 03]: pg installed as runtime dep (not devDep) — used at runtime for DB connection
- [Phase 03]: [Phase 03-01]: MemoryRepository constructor takes pg.Pool for dependency injection — enables vi.mock('pg') pattern in tests
- [Phase 03]: [Phase 03-01]: pruneIfNeeded uses two queries (COUNT then conditional DELETE) — cleaner for testability with mocked pool
- [Phase 03]: STORE_MEMORY and UPDATE_MEMORY parsed after INFRA_VERDICT and PROPOSE_ISSUE — tail markers strip cleanly in sequence
- [Phase 03]: [Phase 03-02]: System prompt instruction uses 'Relevant memories' (no ## prefix) to avoid colliding with injected block assertion in tests
- [Phase 03]: [Phase 03-02]: bot.test.ts mocks MemoryRepository module entirely — preserves isolation without pg dependency in unit tests
- [Phase 03]: DATABASE_URL constructed via K8s variable substitution from individual DATABASE_HOST/PORT/NAME/USER/PASSWORD vars — avoids hardcoding URL, matches allchat-secrets/database-password key used by other services
- [Phase 03]: DO $$ block for memory_type ENUM — PostgreSQL CREATE TYPE has no IF NOT EXISTS support, requires workaround via pg_type catalog check
- [Phase 03]: ON_ERROR_STOP=0 in migration init container — allows pod to start even if some idempotent statements fail (safe since CREATE TABLE/INDEX use IF NOT EXISTS)
- [Phase 03]: DATABASE_URL constructed via K8s variable substitution from individual DATABASE_HOST/PORT/NAME/USER/PASSWORD vars — avoids hardcoding URL, matches allchat-secrets/database-password key used by other services
- [Phase 03]: DO $$ block for memory_type ENUM — PostgreSQL CREATE TYPE has no IF NOT EXISTS support, requires workaround via pg_type catalog check
- [Phase 03]: ON_ERROR_STOP=0 in migration init container — allows pod to start even if some idempotent statements fail (safe since CREATE TABLE/INDEX use IF NOT EXISTS)
- [Phase 04]: youtube-listener-innertube Service had no app label — added app: youtube-listener-innertube so the ServiceMonitor matchExpressions selector can match it (was invisible to Prometheus before)
- [Phase 04]: [Phase 04-01]: Gap matrix produced from code audit (live Prometheus unreachable during execution) — scrape status is expected state post-SM-fix; live verification via Grafana checkpoint
- [Phase 04]: [Phase 04-01]: Live Prometheus audit via Grafana MCP was not possible during automated execution; gap matrix produced from code audit instead — live verification confirmed via user checkpoint (all 14 services up=1)
- [Phase 04]: [Phase 04-02]: youtube-listener RecordMessage/RecordPublish wired in MessageHandler (cmd package) not in poller — keeps shared/metrics import out of streams package, publish point is in cmd anyway
- [Phase 04]: [Phase 04-02]: twitch-eventsub-listener metrics injected into webhooks.Handler; RecordMessage per notification before routing, RecordPublish after routeEvent success/failure
- [Phase 04]: [Phase 04-02]: discord-listener uses local package-level functions for new message metrics — no shared/metrics import to avoid promauto duplicate registration
- [Phase 04]: [Phase 04-02]: api-gateway RecordMessageReceived fires once per Redis pub/sub message; RecordMessageSent fires per WebSocket client connection in BroadcastToOverlay result
- [Phase 04]: [Phase 04-02]: message-processor emote enricher uses SetMetrics() post-construction injection to avoid changing NewEnricher signature; stream lag computed from Redis stream ID millisecond prefix
- [Phase 04]: [Phase 04-03]: httpMetricsMiddleware defined locally in each service's cmd/main.go — avoids importing GatewayMetrics (semantically wrong for non-gateway) while keeping metric names consistent
- [Phase 04]: [Phase 04-03]: categorizeRefreshError helper uses string matching consistent with existing isNonRetryableError pattern — classifies into token_revoked, invalid_client, network_error, other
- [Phase 04]: [Phase 04-05]: listener-disconnected rules use separate UIDs for kick and discord because they expose local metric packages instead of shared/metrics — one rule per distinct metric name
- [Phase 04]: [Phase 04-05]: pipeline-stall alert uses math expression type (not threshold) to express  == 0 &&  > 0 multi-condition logic; noDataState: OK on all new rules to avoid noise during quiet periods
- [Phase 04]: [Phase 04-05]: websocket-connections-zero severity: warning (not critical) — zero connections during off-stream hours is expected; websocket-connections-drop >50% is critical
- [Phase 04]: [Phase 04-04]: Replaced all 6 existing dashboards (not just 4) with 5 tiered ones — listener-observability and service-health were also stale/redundant
- [Phase 04]: [Phase 04-04]: Listeners dashboard uses collapsed rows with sub-panels array for 7 listeners; datasourceUid lowercase 'prometheus' matches kube-prometheus-stack registration
- [Phase 05]: sourceRepository interface in demand package allows mock injection in tests without importing registry package
- [Phase 05]: hydrate() called before subscribeLoop() in Start() prevents empty DemandUpdate snapshot on source-manager restart
- [Phase 05]: GetDemandedSources() returns make([]DemandSource, 0) not nil — ensures JSON marshals as [] not null
- [Phase 05]: assignedSourceIDs tracked in ListenerBase for intersection logic in demand loop without inter-interface calls
- [Phase 05]: Redis testutil moved to testutil/redisutil subpackage to avoid miniredis dependency in service-level go.mod files
- [Phase 05]: trackedChannel.SourceID added to kick-listener for O(1) slug-to-sourceID lookup in reconcileDemand
- [Phase 05-04]: Leadership-only listeners (innertube, discord, youtube) add direct source:demand Pub/Sub goroutine in main.go — pragmatic minimum since they don't call base.Start; connect/disconnect gating into stream managers deferred to follow-up
- [Phase 05-04]: kick-listener and twitch-eventsub-listener already had Platform set from prior migration phases (36, 38) — no code changes required
- [Phase 05]: assignedSourceIDs changed from Map<string,boolean> to Set<string> to align with DemandSubscriber constructor signature
- [Phase 05]: getDemand() added to CoordinatorClient to avoid exposing raw auth headers; uses existing axios interceptor for JWT
- [Phase 05]: livePollerRunning boolean guards LiveStreamPoller start/stop — poller does not start until first non-empty demand update
- [Phase 05]: youtube-listener demand filter placed after inactive source validation; DemandChecker interface in gateway package avoids circular import; SetDemandChecker setter preserves existing call sites; cleanupInactivePollers naturally handles demand-removed channels without extra code
- [Phase 06]: LeadershipListener is standalone (no embed) — eliminates dual ListenerBase/LeadershipListener hierarchy per D-06
- [Phase 06]: reconcileDemand simplified to platform-only filter — assignedSourceIDs intersection removed in leadership model
- [Phase 06]: UpdateAssignedSourceIDs kept as no-op slot in ChannelManager interface for stability; Plan 02 can remove if needed
- [Phase 06]: SetDisableDemandFiltering added to LeadershipListener — enables post-construction config without re-exposing LeadershipConfig struct
- [Phase 06]: isLeaderFn func() bool closure replaces *leaderState struct in startHTTPServer — decouples HTTP handlers from leadership tracking implementation
- [Phase 06]: EnsureLeadership lostCallback replaces Redis SETNX renewal loop in twitch-eventsub-listener per D-12/D-13
- [Phase 06]: shared/coordination fully deleted per D-01/D-02 — no deprecated fallback; Plans 01+02 already removed all callers
- [Phase 06]: source-manager port changed from 8088 to 8083 per D-05 — coordinator-only port eliminated, consolidates to single leadership/demand API port

### Roadmap Evolution

- Phase 1 added: Discord support bot that answers user questions with codebase awareness, proposes code changes without making them, and uses Claude Code login to avoid additional charges
- Phase 2 added: Support bot operational awareness — Grafana logs and K8s cluster state access with leak prevention, infrastructure error detection, and lead developer pinging
- Phase 3 added: Discord support bot persistent memory storage for learning and improvement over time
- Phase 4 added: Grafana Dashboard Audit & Metrics Gap Implementation
- Phase 5 added: TikTok listener demand-driven polling — only poll when overlay has connected clients
- Phase 6 added: Unify all listeners to leadership-based coordination — remove assignment-based pattern, single resilient architecture

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 33 (Pre-Migration Cleanup — research flags):**

- Source ID normalization strategy: Validate whether coordinator `/assignments` response can be normalized server-side vs. normalizing all listeners client-side before choosing the fix direction
- HandleMigrationEvent error return: Confirm `shared/coordination/migration_subscriber.go` handles the returned error gracefully (logs/ignores) before deploying the canonical signature

**Phase 34 (SDK Definition — research flags):**

- discordgo version: Run `go get github.com/bwmarrin/discordgo@latest` before wiring — expected v0.28.1 or higher; verify before pinning

**Phase 37 (Discord migration — research flag):**

- Integration test shard acquisition and release with SDK-backed LeadershipListener before deploying to confirm Discord Gateway RESUME protocol is unaffected

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260325-ljj | Discord relay webhook-based sending with sender identity | 2026-03-25 | f7ab0eb | [260325-ljj-discord-relay-webhook-based-sending-with](./quick/260325-ljj-discord-relay-webhook-based-sending-with/) |
| 260325-lwo | Auto-create Discord webhook when relay enabled | 2026-03-25 | 5b776b3 | [260325-lwo-auto-create-discord-webhook-when-relay-e](./quick/260325-lwo-auto-create-discord-webhook-when-relay-e/) |
| 260326-poh | Update Open Graph and meta embed tags for allch.at | 2026-03-26 | f544bab | [260326-poh-update-open-graph-and-meta-embed-tags-fo](./quick/260326-poh-update-open-graph-and-meta-embed-tags-fo/) |
| 260326-r4m | Fix closed dependabot PRs after force push — update all Go and npm deps | 2026-03-26 | 38225be | [260326-r4m-fix-closed-dependabot-prs-after-force-pu](./quick/260326-r4m-fix-closed-dependabot-prs-after-force-pu/) |
| 260328-tqh | Enable tracing in the tiktok-listener service | 2026-03-28 | 01effa3 | [260328-tqh-enable-tracing-in-the-tiktok-listener-se](./quick/260328-tqh-enable-tracing-in-the-tiktok-listener-se/) |
| 260328-v03 | Fix Grafana dashboard legends to show meaningful labels | 2026-03-28 | cc3982f (caesar-deployment) | [260328-v03-fix-grafana-dashboard-legends-to-show-me](./quick/260328-v03-fix-grafana-dashboard-legends-to-show-me/) |

## Session Continuity

Last session: 2026-03-28T21:26:47Z
Last activity: 2026-03-28
Stopped at: Completed quick task 260328-v03 — fix Grafana dashboard legends
Resume file: None

**Next action:** Phase 05 Plan 04 Task 3 — E2E demand signal verification: make docker-up, open overlay, check logs.
