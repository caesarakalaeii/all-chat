# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Frontend Redesign** — Phases 23-26 (shipped 2026-03-14)
- ✅ **v1.5 Discord Listener** — Phases 27-32 (shipped 2026-03-16)
- ✅ **v1.6 Listener SDK** — Phases 33-38 (shipped 2026-03-18)

## Phases

<details>
<summary>✅ v1.0 Message Deletion Support (Phases 1-3) — SHIPPED 2026-02-18</summary>

### Phase 1: Foundation + Twitch
**Goal**: Establish message deletion infrastructure with Twitch platform
**Plans**: 5/5 complete
**Status**: Complete (2026-02-18)

### Phase 2: YouTube Integration
**Goal**: YouTube deletion support via polling
**Plans**: 2/2 complete
**Status**: Complete (2026-02-18)

### Phase 3: Kick Integration + Edge Cases
**Goal**: Kick WebSocket deletion events and reconnection handling
**Plans**: 4/4 complete
**Status**: Complete (2026-02-18)

**Note**: Phase 4 (TikTok Integration) was deferred, ending v1.0 at 80% completion.

</details>

<details>
<summary>✅ v1.1 Listener Load Balancing (Phases 4-10) — SHIPPED 2026-02-21</summary>

**Milestone Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services, enabling cost-effective scaling and reliable service for high-volume streams.

### Phase 4: Sharding Infrastructure & Coordinator Service
**Goal**: Production-ready consistent hashing and coordinator service with split-brain prevention
**Plans**: 5/5 complete
**Status**: Complete (2026-02-19)

### Phase 5: Connection Management & Migration Protocol
**Goal**: All platform listeners integrate with coordinator and support graceful zero-loss channel migration
**Plans**: 8/8 complete
**Status**: Complete (2026-02-20)

### Phase 6: Dynamic Rebalancing & HPA Integration
**Goal**: Automatic load-aware rebalancing with safeguards against thundering herd and quota exhaustion
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

### Phase 7: Observability & Production Readiness
**Goal**: Comprehensive metrics, distributed tracing, Grafana dashboards, and alerting for production operations
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

**Delivered:**
- Bounded-load consistent hashing with Kubernetes Lease-based coordinator
- Zero-loss channel migration protocol across all platforms (Twitch, Kick, TikTok)
- Load-aware automatic rebalancing with composite scoring and cooldown safeguards
- Comprehensive observability with 16 distributed tracing spans, Grafana dashboards, and Prometheus alerts

**Archive**: [v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md) | [v1.1-REQUIREMENTS.md](milestones/v1.1-REQUIREMENTS.md) | [v1.1-MILESTONE-AUDIT.md](milestones/v1.1-MILESTONE-AUDIT.md)

</details>

<details>
<summary>✅ v1.2 InnerTube YouTube Listener (Phases 11-22) — SHIPPED 2026-03-06</summary>

**Milestone Goal:** Build quota-free YouTube listener using InnerTube API as drop-in replacement for official API listener, maintaining identical downstream behavior while eliminating quota limitations.

### Phase 11: Core Ingestion PoC
**Goal**: Validate InnerTube API viability by establishing basic message flow from InnerTube to Redis Streams
**Plans**: 5/5 complete
**Status**: Complete (2026-02-21)

### Phase 12: Production Minimum
**Goal**: Enable dynamic stream management and production lifecycle behaviors
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 13: Contract Validation
**Goal**: Prove behavioral equivalence with official youtube-listener through comprehensive contract testing
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 14: Production Rollout
**Goal**: Deploy to production with gradual canary rollout, monitoring, and automatic rollback
**Plans**: 3/3 complete
**Status**: Complete (2026-03-05)

### Phase 15: Feature Parity
**Goal**: Add deletion event detection and advanced metrics leveraging InnerTube advantages
**Plans**: 5/5 complete
**Status**: Complete (2026-03-06)

**Delivered:**
- InnerTube API integration eliminates YouTube API quota constraints (10,000 units/day → unlimited)
- RawChatMessage schema byte-for-byte compatible (drop-in replacement, zero downstream changes)
- All event types supported (messages, Super Chat, Super Sticker, memberships, milestones, deletions)
- Batch deletion detection with time-windowed aggregation (5+ deletions in 100ms)
- Production canary infrastructure (Argo Rollouts, Prometheus metrics, automatic rollback)
- 8,684 LOC InnerTube service with 69 automated tests

**Archive**: [v1.2-ROADMAP.md](milestones/v1.2-ROADMAP.md) | [v1.2-REQUIREMENTS.md](milestones/v1.2-REQUIREMENTS.md) | [v1.2-MILESTONE-AUDIT.md](milestones/v1.2-MILESTONE-AUDIT.md)

</details>

<details>
<summary>✅ v1.3 Frontend Redesign (Phases 23-26) — SHIPPED 2026-03-14</summary>

**Milestone Goal:** Transform frontend from generic Tailwind defaults to professional streaming-focused design system with comprehensive component library and enforceable style rules.

- [x] **Phase 23: Design Token System & Foundation** — Tailwind v4 three-tier @theme tokens, cascade layers, static platform colors, events.css stability contract (3/3 plans, completed 2026-03-10)
- [x] **Phase 24: Component Library Setup & Customization** — @base-ui/react + shadcn/ui, CVA variants, micro-interactions, platform-color components, all Storybook tests pass (5/5 plans, completed 2026-03-11)
- [x] **Phase 25: Page Migration & Split-view Preview** — All 6 pages redesigned, draggable SplitView, Dialog/Toast patterns, zero legacy gray-* classes (8/8 plans, completed 2026-03-11)
- [x] **Phase 26: Enforcement & Quality Gates** — ESLint flat config + Prettier, Husky pre-commit, 7-gate CI pipeline, Chromatic visual regression, marketplace migration guide (4/4 plans, completed 2026-03-14)

**Delivered:**
- Tailwind v4 design token system with cascade layer architecture eliminating all !important overrides
- Full component library (Button, Card, Input, Badge, Dialog, Toast, Skeleton) with CVA variants and platform-color coding
- Professional streaming dark aesthetic across all pages with WCAG 2.1 AA accessibility compliance
- Draggable SplitView live preview component with pointer-capture drag and keyboard navigation
- Automated enforcement: ESLint + Prettier + Husky pre-commit + 7-gate CI + Chromatic baseline
- 45/45 Storybook tests passing with a11y in 'error' mode

**Archive**: [v1.3-ROADMAP.md](milestones/v1.3-ROADMAP.md) | [v1.3-REQUIREMENTS.md](milestones/v1.3-REQUIREMENTS.md)

</details>

<details>
<summary>✅ v1.5 Discord Listener (Phases 27-32) — SHIPPED 2026-03-16</summary>

**Milestone Goal:** Add Discord as a bidirectional chat source — read from Discord channels into overlays, and relay overlay messages back to Discord with loop-safe filtering, full OAuth2 bot authorization UX, and production-grade load balancing.

### Phase 27: Auth and Bot Token Foundation
**Goal**: Users can authorize the Discord bot into their server and the service has a stable Gateway connection with correct session management
**Plans**: 4/4 complete
**Status**: Complete (2026-03-15)

### Phase 28: Inbound Listener Core
**Goal**: Discord channel messages appear in overlays as a real-time, first-class chat source
**Plans**: 2/2 complete
**Status**: Complete (2026-03-15)

### Phase 29: Inbound Enrichment
**Goal**: Discord messages carry deletion events and resolved mention text through the existing platform pipelines
**Plans**: 2/2 complete
**Status**: Complete (2026-03-16)

### Phase 30: Outbound Relay
**Goal**: Non-Discord overlay messages are posted to a user-configured Discord channel with no echo loops
**Plans**: 2/2 complete
**Status**: Complete (2026-03-16)

### Phase 31: Load Balancing
**Goal**: discord-listener runs safely across multiple pods with deterministic shard ownership and auto-scales under load
**Plans**: 3/3 complete
**Status**: Complete (2026-03-16)

### Phase 32: Setup UI
**Goal**: Users can configure Discord sources end-to-end from the frontend without leaving the app
**Plans**: 3/3 complete
**Status**: Complete (2026-03-16)

</details>

<details>
<summary>✅ v1.6 Listener SDK (Phases 33-38) — SHIPPED 2026-03-18</summary>

**Milestone Goal:** Extract load balancing, leader election, and channel management into a shared SDK that all listeners import, eliminating 80-150 lines of duplicated startup wiring from every listener and making future listeners trivial to build.

- [x] **Phase 33: Pre-Migration Cleanup** — Source ID suffix normalization + HandleMigrationEvent error signature canonicalized (2/2 plans, completed 2026-03-17)
- [x] **Phase 34: SDK Package Definition** — shared/listener package: ListenerBase, LeadershipListener, ChannelManager, ShutdownCoordinator, make build-all CI target (3/3 plans, completed 2026-03-17)
- [x] **Phase 35: Migrate twitch-listener** — First production SDK validation; ListenerBase archetype confirmed (2/2 plans, completed 2026-03-17)
- [x] **Phase 36: Migrate kick-listener** — Both ListenerBase + LeadershipListener archetypes exercised (2/2 plans, completed 2026-03-17)
- [x] **Phase 37: Migrate youtube-innertube and discord-listener** — Leadership-only archetype validated with two independent services (3/3 plans, completed 2026-03-17)
- [x] **Phase 38: Migrate youtube-listener and twitch-eventsub-listener** — Migration window closed; all Go listeners on shared SDK (3/3 plans, completed 2026-03-18)

**Delivered:**
- Shared `shared/listener` SDK (797 LOC) with two archetypes: `ListenerBase` (Twitch, youtube, twitch-eventsub) and `LeadershipListener` (kick, youtube-innertube, discord)
- All 6 Go listeners migrated — `cmd/main.go` reduced to service-specific connection + publishing only
- Compile-time `ChannelManager` assertions and goroutine leak smoke tests in every migrated listener
- `make build-all` CI target catching cross-module version drift

**Archive**: [v1.6-ROADMAP.md](milestones/v1.6-ROADMAP.md) | [v1.6-REQUIREMENTS.md](milestones/v1.6-REQUIREMENTS.md)

</details>

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23-26 | v1.3 | 20/20 | Complete | 2026-03-14 |
| 27-32 | v1.5 | 16/16 | Complete | 2026-03-16 |
| 33-38 | v1.6 | 15/15 | Complete | 2026-03-18 |
| 1-6 | milestone | 45+3/48 | In Progress | - |

### Phase 1: Discord Support Bot

**Goal:** Deploy a Discord support bot (services/support-bot) that answers user questions about All-Chat and All-Chat-Extension with codebase awareness via Claude Agent SDK, proposes code changes as GitHub issues, and responds to @mentions and /support slash commands.
**Requirements**: BOT-01, BOT-02, BOT-03, BOT-04, BOT-05, BOT-06, BOT-07
**Depends on:** None
**Plans:** 2/3 plans executed

Plans:
- [ ] 01-01-PLAN.md — Project scaffold, types, Claude agent wrapper, GitHub issues module, env validation
- [ ] 01-02-PLAN.md — Discord bot event handlers (@mention + slash command), handleQuestion orchestrator, command registration
- [ ] 01-03-PLAN.md — Dockerfile, Kubernetes deployment manifests, live Discord verification

### Phase 2: Support Bot Operational Awareness

**Goal:** Give the support bot access to Grafana logs and Kubernetes cluster state to detect ongoing infrastructure errors (missing secrets, crashed pods, OOMKills) so it can distinguish code bugs from operational issues. Bot must NEVER leak raw logs to Discord users. Recognize lead developer (caesarlp, configured via LEAD_DEVELOPER_DISCORD_ID env var) and ping them when infrastructure issues are detected.
**Requirements**: OPS-01, OPS-02, OPS-03, OPS-04, OPS-05, OPS-06, OPS-07, OPS-08, OPS-09
**Depends on:** Phase 1
**Plans:** 2/3 plans executed

Plans:
- [ ] 02-01-PLAN.md — Extend agent.ts with Grafana MCP config, kubectl tools, leak-prevention prompt, INFRA_VERDICT parsing
- [ ] 02-02-PLAN.md — Lead dev @mention logic in bot.ts, new env var validation in index.ts
- [ ] 02-03-PLAN.md — Dockerfile binary installs (kubectl, mcp-grafana), K8s deployment env vars, RBAC for cluster read access

### Phase 3: Support Bot Persistent Memory

**Goal:** Add persistent memory storage to the support bot so it learns from past interactions -- stores common error patterns, user corrections, and codebase insights in PostgreSQL, retrieves relevant memories via tag matching, and injects them into the Claude prompt. Memory creation is auto-detected via STORE_MEMORY marker protocol.
**Requirements**: MEM-01, MEM-02, MEM-03, MEM-04, MEM-05, MEM-06, MEM-07, MEM-08, MEM-09
**Depends on:** Phase 2
**Plans:** 2/3 plans executed

Plans:
- [ ] 03-01-PLAN.md — Types, migration SQL, pg dependency, MemoryRepository class with tests
- [ ] 03-02-PLAN.md — Agent.ts memory injection + marker parsing, bot.ts wiring, index.ts DB init
- [ ] 03-03-PLAN.md — Kubernetes deployment DATABASE_URL env var and migration init container

### Phase 4: Grafana Dashboard Audit & Metrics Gap Implementation

**Goal:** Audit all Grafana dashboards and Prometheus metrics across all 14 services, wire missing RecordX() calls into the message pipeline and platform ops services, add 3 listeners to ServiceMonitor, create 5 tiered dashboards as code, and add 4 alert groups covering listener disconnections, pipeline stalls, WebSocket drops, and error rate spikes.
**Requirements**: AUDIT-01, AUDIT-02, SM-01, WIRE-01, WIRE-02, WIRE-03, WIRE-04, WIRE-05, WIRE-06, WIRE-07, WIRE-08, WIRE-09, WIRE-10, DASH-01, DASH-02, DASH-03, DASH-04, DASH-05, ALERT-01, ALERT-02, ALERT-03, ALERT-04, ALERT-05
**Depends on:** Phase 3
**Plans:** 6 plans (5 original + 1 gap closure)

Plans:
- [ ] 04-01-PLAN.md — ServiceMonitor update + live Prometheus audit + confirmed gap matrix
- [ ] 04-02-PLAN.md — Message flow metrics wiring (youtube-listener, twitch-eventsub, discord, api-gateway, message-processor)
- [ ] 04-03-PLAN.md — Platform ops metrics wiring (auth, overlay-manager, token-refresh, emote-service, source-manager)
- [ ] 04-04-PLAN.md — 5 tiered Grafana dashboards as code in ConfigMap
- [ ] 04-05-PLAN.md — 4 alert groups as code (listener disconnect, pipeline stall, WebSocket drop, error rate)
- [ ] 04-06-PLAN.md — Gap closure: fix InnerTube dashboard panel query + pipeline-stall alert false positive

### Phase 5: TikTok listener demand-driven polling — only poll when overlay has connected clients

**Goal:** Make all listeners except Twitch IRC demand-driven: source-manager subscribes to overlay connection events, resolves which sources have demand, and publishes demand signals via Redis Pub/Sub. Go listener SDK gains a demand subscriber loop. TikTok listener replaces DB polling with demand-driven activation. All non-Twitch listeners only connect when overlays are open.
**Requirements**: DEMAND-01, DEMAND-02, DEMAND-03, DEMAND-04, DEMAND-05, DEMAND-06, DEMAND-07, DEMAND-08, DEMAND-09
**Depends on:** Phase 4
**Plans:** 6/6 complete
**Status**: Complete (2026-03-27)

Plans:
- [x] 05-01-PLAN.md — Source-manager demand subscriber, DemandUpdate Pub/Sub, GET /demand endpoint, repository extension
- [x] 05-02-PLAN.md — Go SDK demand loop, ChannelManager UpdateDemandedSourceIDs, all listener implementations
- [x] 05-03-PLAN.md — TikTok listener DemandSubscriber replacing pollActiveStreams, LiveStreamPoller idle control
- [x] 05-04-PLAN.md — Wire demand into kick/twitch-eventsub SDK config + leadership-only listener subscriptions + E2E verify
- [x] 05-05-PLAN.md — Gap closure: wire demand gating into innertube, discord, and youtube-listener stream managers
- [x] 05-06-PLAN.md — Gap closure: E2E demand signal verification checkpoint

### Phase 6: Unify all listeners to leadership-based coordination

**Goal:** Eliminate the dual coordinator/leadership architecture by merging ListenerBase into LeadershipListener, migrating twitch-listener, twitch-eventsub-listener, and kick-listener to the unified type, then removing all coordinator infrastructure (shared/coordination, source-manager coordination subsystem, port 8088) and consolidating source-manager to port 8083.
**Requirements**: D-01, D-02, D-03, D-04, D-05, D-06, D-07, D-08, D-09, D-10, D-11, D-12, D-13, D-14, D-15, D-16, D-17
**Depends on:** Phase 5
**Plans:** 3/3 plans complete
**Status**: Complete (2026-03-27)

Plans:
- [x] 06-01-PLAN.md — SDK refactor: merge ListenerBase into LeadershipListener, update ChannelManager interface, remove coordinator loops
- [x] 06-02-PLAN.md — Migrate twitch-listener + twitch-eventsub-listener + kick-listener to LeadershipListener
- [x] 06-03-PLAN.md — Remove coordinator from source-manager, delete shared/coordination, consolidate to port 8083, update K8s manifests
