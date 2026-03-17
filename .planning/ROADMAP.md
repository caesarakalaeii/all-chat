# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Frontend Redesign** — Phases 23-26 (shipped 2026-03-14)
- ✅ **v1.5 Discord Listener** — Phases 27-32 (shipped 2026-03-16)
- 🚧 **v1.6 Listener SDK** — Phases 33-38 (in progress)

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

### 🚧 v1.6 Listener SDK (In Progress)

**Milestone Goal:** Extract load balancing, leader election, and channel management into a shared SDK that all listeners import, eliminating 80–150 lines of duplicated startup wiring from every listener and making future listeners trivial to build.

- [x] **Phase 33: Pre-Migration Cleanup** - Normalize source ID suffix handling and canonicalize HandleMigrationEvent signature across Twitch and Kick before SDK extraction begins (completed 2026-03-17)
- [ ] **Phase 34: SDK Package Definition** - Create shared/listener package with ListenerBase, LeadershipListener, ChannelManager interface, ShutdownCoordinator, and make build-all CI target
- [ ] **Phase 35: Migrate twitch-listener** - First production SDK validation; twitch-listener cmd/main.go reduced to IRC connection and message publishing only
- [ ] **Phase 36: Migrate kick-listener** - Exercise both ListenerBase and LeadershipListener archetypes; both SDK models confirmed in production
- [ ] **Phase 37: Migrate youtube-innertube and discord-listener** - Leadership-only migrations; close leadership archetype with two independent validations
- [ ] **Phase 38: Migrate youtube-listener and twitch-eventsub-listener** - Close migration window; all Go listeners running on shared SDK

## Phase Details

### Phase 33: Pre-Migration Cleanup
**Goal**: All behavioral inconsistencies between existing listeners are resolved and deployed before any SDK code is written
**Depends on**: Phase 32 (v1.5 complete)
**Requirements**: PREP-01, PREP-02
**Success Criteria** (what must be TRUE):
  1. All Go listeners agree on source ID format — source IDs passed to CoordinatorClient either always include or always omit the `:platform` suffix, with no per-listener branching
  2. `HandleMigrationEvent` has the canonical signature `func(event *coordination.MigrationEvent) error` in both twitch-listener and kick-listener channel managers, and both compile and pass existing tests
  3. The `shared/coordination` migration subscriber correctly handles the error return from `HandleMigrationEvent` (logs or ignores) without panicking
**Plans**: 2 plans

Plans:
- [ ] 33-01-PLAN.md — Source ID suffix normalization (strip-at-intake in kick-listener and twitch-listener startup path)
- [ ] 33-02-PLAN.md — HandleMigrationEvent error signature canonicalization across both channel managers and MigrationSubscriber

### Phase 34: SDK Package Definition
**Goal**: The shared/listener package exists, is fully tested, and the build-all CI target verifies all listener modules compile together
**Depends on**: Phase 33
**Requirements**: SDK-01, SDK-02, SDK-03, SDK-04, SDK-05, SDK-06, SDK-07, SDK-08, VERIFY-01
**Success Criteria** (what must be TRUE):
  1. `shared/listener` package compiles with `go build ./...` inside the shared module — four new files present: base.go, leadership.go, channel_manager.go, shutdown.go
  2. `NewCoordinatorClient` in `shared/coordination/client.go` accepts an explicit `serviceName string` parameter — no existing listener has a compilation error after the signature change
  3. Unit tests with a mock coordinator verify that `ListenerBase` goroutines (heartbeat, assignment refresh, migration subscriber) start on `Start()` and stop on `Stop()` with no goroutine leak (`goleak.VerifyNone` passes)
  4. `make build-all` runs in CI and exits 0, building all listener modules in a single command
  5. `ListenerConfig` exposes heartbeat interval, assignment refresh interval, and startup jitter max — setting `LISTENER_STARTUP_JITTER_MAX=0` results in zero jitter delay in tests
**Plans**: TBD

### Phase 35: Migrate twitch-listener
**Goal**: twitch-listener uses the shared SDK in production and validates the ListenerBase lifecycle against live Twitch IRC traffic
**Depends on**: Phase 34
**Requirements**: MIGRATE-01, VERIFY-02
**Success Criteria** (what must be TRUE):
  1. twitch-listener `cmd/main.go` contains only IRC connection setup and message publishing — all coordinator wiring, heartbeat, assignment refresh, and migration subscriber goroutines are gone from the file
  2. `var _ listener.ChannelManager = (*channels.Manager)(nil)` compile-time assertion is present in twitch-listener `channels/manager.go` and the build succeeds
  3. `messages_published_total` metric for twitch-listener shows no drop greater than 10% sustained for 5 minutes after the SDK-backed deployment replaces the prior deployment
  4. All existing twitch-listener unit tests pass without modification
**Plans**: TBD

### Phase 36: Migrate kick-listener
**Goal**: kick-listener uses both ListenerBase and LeadershipListener from the SDK, confirming both archetypes work correctly in production
**Depends on**: Phase 35
**Requirements**: MIGRATE-02
**Success Criteria** (what must be TRUE):
  1. kick-listener `cmd/main.go` uses `ListenerBase` for assignment coordination and `LeadershipListener` for per-stream ownership — manual construction of both CoordinatorClient and LeadershipCoordinator is removed from the file
  2. kick-listener compiles with string-keyed channel manager (`strconv.Itoa(chatroomID)` convention documented in code) and all existing Kick channel manager tests pass
  3. Both SDK archetypes (assignment-based and leadership-based) have been exercised against live Kick traffic with `messages_published_total` showing no regression
**Plans**: TBD

### Phase 37: Migrate youtube-innertube and discord-listener
**Goal**: Both leadership-only listeners use LeadershipListener from the SDK, validating the archetype without assignment coordinator complexity
**Depends on**: Phase 36
**Requirements**: MIGRATE-04, MIGRATE-05
**Success Criteria** (what must be TRUE):
  1. youtube-listener-innertube `cmd/main.go` uses `LeadershipListener` — manual `sourcemanager.NewLeadershipCoordinator` wiring is removed; SDK nil-safe passthrough is active when `SOURCE_MANAGER_SECRET` is absent
  2. discord-listener `cmd/main.go` uses `LeadershipListener` — shard ownership coordination via Redis lock pattern is unchanged in behavior; Gateway RESUME protocol is unaffected
  3. Both services deploy without regression: InnerTube message rate is stable and Discord relay continues to function with loop-safety filter intact
**Plans**: TBD

### Phase 38: Migrate youtube-listener and twitch-eventsub-listener
**Goal**: Migration window is closed — all Go listeners run on the shared SDK and the mixed-fleet monitoring period ends
**Depends on**: Phase 37
**Requirements**: MIGRATE-03, MIGRATE-06
**Success Criteria** (what must be TRUE):
  1. youtube-listener `cmd/main.go` uses `LeadershipListener` — quota tracker behavior is unchanged; all existing quota-related tests pass against the SDK-backed assignment implementation
  2. twitch-eventsub-listener `cmd/main.go` uses `ListenerBase` — stateless webhook receiver gains standardized heartbeat and health wiring; no existing EventSub webhook test requires modification
  3. All 5 Go listener services (twitch, kick, youtube, youtube-innertube, discord) plus twitch-eventsub-listener are running SDK-backed code in production simultaneously with no active mixed-fleet monitoring alerts
**Plans**: TBD

## Progress

**Execution Order:** 33 → 34 → 35 → 36 → 37 → 38

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23-26 | v1.3 | 20/20 | Complete | 2026-03-14 |
| 27-32 | v1.5 | 16/16 | Complete | 2026-03-16 |
| 33. Pre-Migration Cleanup | 2/2 | Complete   | 2026-03-17 | - |
| 34. SDK Package Definition | v1.6 | 0/TBD | Not started | - |
| 35. Migrate twitch-listener | v1.6 | 0/TBD | Not started | - |
| 36. Migrate kick-listener | v1.6 | 0/TBD | Not started | - |
| 37. Migrate youtube-innertube and discord-listener | v1.6 | 0/TBD | Not started | - |
| 38. Migrate youtube-listener and twitch-eventsub-listener | v1.6 | 0/TBD | Not started | - |

---
*Last updated: 2026-03-17 after Phase 33 planning*
