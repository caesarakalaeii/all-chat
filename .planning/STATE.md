---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: unknown
stopped_at: Completed 04-01-PLAN.md
last_updated: "2026-03-26T17:52:05.358Z"
last_activity: 2026-03-26
progress:
  total_phases: 7
  completed_phases: 6
  total_plans: 25
  completed_plans: 21
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-18)

**Core value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.
**Current focus:** Phase 04 — grafana-dashboard-audit-metrics-gap-implementation

## Current Position

Phase: 04 (grafana-dashboard-audit-metrics-gap-implementation) — EXECUTING
Plan: 1 of 5

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

### Roadmap Evolution

- Phase 1 added: Discord support bot that answers user questions with codebase awareness, proposes code changes without making them, and uses Claude Code login to avoid additional charges
- Phase 2 added: Support bot operational awareness — Grafana logs and K8s cluster state access with leak prevention, infrastructure error detection, and lead developer pinging
- Phase 3 added: Discord support bot persistent memory storage for learning and improvement over time
- Phase 4 added: Grafana Dashboard Audit & Metrics Gap Implementation

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

## Session Continuity

Last session: 2026-03-26T17:52:05.354Z
Last activity: 2026-03-26
Stopped at: Completed 04-01-PLAN.md
Resume file: None

**Next action:** Phase 02 complete.
