# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Frontend Redesign** — Phases 23-26 (shipped 2026-03-14)
- 🚧 **v1.5 Discord Listener** — Phases 27-32 (in progress)

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

### 🚧 v1.5 Discord Listener (In Progress)

**Milestone Goal:** Add Discord as a bidirectional chat source — read from Discord channels into overlays, and relay overlay messages back to Discord with loop-safe filtering, full OAuth2 bot authorization UX, and production-grade load balancing.

## Phase Details

### Phase 27: Auth and Bot Token Foundation
**Goal**: Users can authorize the Discord bot into their server and the service has a stable Gateway connection with correct session management
**Depends on**: Nothing (first phase of v1.5)
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Success Criteria** (what must be TRUE):
  1. User can click "Add to Server" and complete the OAuth2 flow, resulting in the bot appearing in their Discord server
  2. After connecting, user sees the list of text channels available in their server (API response from auth-service)
  3. User receives a clear, human-readable error when bot is missing required permissions (VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES)
  4. User can disconnect the bot, which removes it from their account and deletes all associated Discord sources
  5. discord-listener establishes a Gateway WebSocket connection with correct intents bitmask and a startup assertion confirms MESSAGE_CONTENT is non-empty on first READY event
**Plans**: TBD

Plans:
- [ ] 27-01: TBD
- [ ] 27-02: TBD
- [ ] 27-03: TBD

### Phase 28: Inbound Listener Core
**Goal**: Discord channel messages appear in overlays as a real-time, first-class chat source
**Depends on**: Phase 27
**Requirements**: INBD-01, INBD-02
**Success Criteria** (what must be TRUE):
  1. A message sent in a configured Discord channel appears in the overlay within one second
  2. Discord messages display with platform label "discord" and author username, consistent with Twitch and YouTube messages in the overlay
  3. Bot messages (author.bot == true) are silently filtered and never appear in overlays
  4. Only messages from the configured inbound channel appear — messages from other channels in the same server are ignored
**Plans**: TBD

Plans:
- [ ] 28-01: TBD
- [ ] 28-02: TBD

### Phase 29: Inbound Enrichment
**Goal**: Discord messages carry deletion events and resolved mention text through the existing platform pipelines
**Depends on**: Phase 28
**Requirements**: INBD-03, INBD-04
**Success Criteria** (what must be TRUE):
  1. When a Discord message is deleted, a deletion event propagates through the pipeline and the message disappears from active overlays (consistent with Twitch/YouTube deletion behavior)
  2. A message containing @username or #channel mentions renders with resolved names (e.g., "@alice" not "@123456789012345678") in the overlay
**Plans**: TBD

Plans:
- [ ] 29-01: TBD
- [ ] 29-02: TBD

### Phase 30: Outbound Relay
**Goal**: Non-Discord overlay messages are posted to a user-configured Discord channel with no echo loops
**Depends on**: Phase 28
**Requirements**: RELY-01, RELY-02, RELY-03, RELY-04
**Success Criteria** (what must be TRUE):
  1. A Twitch message that appears in an overlay is relayed to the configured Discord channel within two seconds, formatted as "[emoji] username: text"
  2. A message that originated from Discord is never posted back to Discord — verified by a test asserting no REST call is made for platform="discord" pub/sub messages
  3. When relay_enabled is set to false for a source, no messages are relayed for that source even if other sources in the same overlay have relay active
  4. The relay target channel can be configured independently from the inbound channel (same or different channel ID accepted on save)
**Plans**: TBD

Plans:
- [ ] 30-01: TBD
- [ ] 30-02: TBD
- [ ] 30-03: TBD

### Phase 31: Load Balancing
**Goal**: discord-listener runs safely across multiple pods with deterministic shard ownership and auto-scales under load
**Depends on**: Phase 30
**Requirements**: LOAD-01, LOAD-02, LOAD-03
**Success Criteria** (what must be TRUE):
  1. When two discord-listener pods are running, exactly one pod holds the Gateway connection for each shard — the second pod does not connect until the first fails or releases ownership
  2. After a pod restart, the Gateway session resumes (RESUME opcode) using the persisted session_id and resume_gateway_url from Redis, avoiding a full re-IDENTIFY
  3. HPA scales discord-listener pods up when events/sec or active guild count exceeds configured thresholds, and the new pod acquires shard ownership within 60 seconds of the prior pod terminating
**Plans**: TBD

Plans:
- [ ] 31-01: TBD
- [ ] 31-02: TBD

### Phase 32: Setup UI
**Goal**: Users can configure Discord sources end-to-end from the frontend without leaving the app
**Depends on**: Phase 31
**Requirements**: UI-01, UI-02, UI-03, UI-04
**Success Criteria** (what must be TRUE):
  1. Settings page shows a Discord server connect card — clicking it initiates the OAuth2 redirect; after completion the card updates to show the connected server name and icon
  2. Overlay editor allows adding a Discord source by selecting a guild and choosing an inbound channel from a dropdown populated from the channel listing API
  3. Each Discord source card in the overlay editor shows connection status and a visual indicator of whether relay is active or inactive
  4. Per-source relay configuration panel lets the user toggle relay on/off and pick an outbound channel; the visual filter indicator updates immediately on toggle
**Plans**: TBD

Plans:
- [ ] 32-01: TBD
- [ ] 32-02: TBD
- [ ] 32-03: TBD

## Progress

**Execution Order:** 27 → 28 → 29 → 30 → 31 → 32

| Phases | Milestone | Plans Complete | Status | Completed |
|--------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23-26 | v1.3 | 20/20 | Complete | 2026-03-14 |
| 27. Auth and Bot Token Foundation | v1.5 | 0/TBD | Not started | - |
| 28. Inbound Listener Core | v1.5 | 0/TBD | Not started | - |
| 29. Inbound Enrichment | v1.5 | 0/TBD | Not started | - |
| 30. Outbound Relay | v1.5 | 0/TBD | Not started | - |
| 31. Load Balancing | v1.5 | 0/TBD | Not started | - |
| 32. Setup UI | v1.5 | 0/TBD | Not started | - |

---
*Last updated: 2026-03-15 after v1.5 roadmap creation*
