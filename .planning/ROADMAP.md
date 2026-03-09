# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 5-8 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 9-13 (shipped 2026-03-06)
- 🚧 **v1.3 Chat Overlay Sharing** — Phases 14-19 (in progress)

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

**Archive**: [v1.0 details](milestones/) (if archived)

</details>

<details>
<summary>✅ v1.1 Listener Load Balancing (Phases 5-8) — SHIPPED 2026-02-21</summary>

**Milestone Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services, enabling cost-effective scaling and reliable service for high-volume streams.

### Phase 5: Sharding Infrastructure & Coordinator Service
**Goal**: Production-ready consistent hashing and coordinator service with split-brain prevention
**Plans**: 5/5 complete
**Status**: Complete (2026-02-19)

### Phase 6: Connection Management & Migration Protocol
**Goal**: All platform listeners integrate with coordinator and support graceful zero-loss channel migration
**Plans**: 8/8 complete
**Status**: Complete (2026-02-20)

### Phase 7: Dynamic Rebalancing & HPA Integration
**Goal**: Automatic load-aware rebalancing with safeguards against thundering herd and quota exhaustion
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

### Phase 8: Observability & Production Readiness
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
<summary>✅ v1.2 InnerTube YouTube Listener (Phases 9-13) — SHIPPED 2026-03-06</summary>

**Milestone Goal:** Build quota-free YouTube listener using InnerTube API as drop-in replacement for official API listener, maintaining identical downstream behavior while eliminating quota limitations.

### Phase 9: Core Ingestion PoC
**Goal**: Validate InnerTube API viability by establishing basic message flow from InnerTube to Redis Streams
**Plans**: 5/5 complete
**Status**: Complete (2026-02-21)

### Phase 10: Production Minimum
**Goal**: Enable dynamic stream management and production lifecycle behaviors
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 11: Contract Validation
**Goal**: Prove behavioral equivalence with official youtube-listener through comprehensive contract testing
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 12: Production Rollout
**Goal**: Deploy to production with gradual canary rollout, monitoring, and automatic rollback
**Plans**: 3/3 complete
**Status**: Complete (2026-03-05)

### Phase 13: Feature Parity
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

### 🚧 v1.3 Chat Overlay Sharing (In Progress)

**Milestone Goal:** Enable streamers to share their aggregated chat overlays with other streamers, unlocking collaborative streaming experiences as the platform's first premium feature.

**Phase Numbering:**
- Integer phases (14, 15, 16, etc.): Planned milestone work
- Decimal phases (14.1, 14.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

#### Phase 14: Foundation
**Goal**: Users can search for streamers and create share requests with premium enforcement
**Depends on**: Nothing (first phase in v1.3)
**Requirements**: SHARE-01, SHARE-02, SHARE-03, PREMIUM-01, PREMIUM-02
**Success Criteria** (what must be TRUE):
  1. User can search for other users by platform username (Twitch, YouTube, Kick, TikTok)
  2. Premium users can send share requests selecting an overlay to share
  3. Non-premium users are blocked from sending share requests (server-side enforcement)
  4. Users can view list of pending incoming share requests in dashboard
  5. Admin can mark specific users as premium for testing purposes
**Plans**: 4 plans

Plans:
- [ ] 14-01-PLAN.md — Database schema and models (Wave 1)
- [ ] 14-02-PLAN.md — User search and share request creation (Wave 2)
- [ ] 14-03-PLAN.md — Premium enforcement and admin controls (Wave 2)
- [ ] 14-04-PLAN.md — Dashboard UI and background expiry job (Wave 3)

#### Phase 15: Share Acceptance
**Goal**: Users can accept share requests, establishing bidirectional overlay access with cycle prevention
**Depends on**: Phase 14
**Requirements**: SHARE-04, SHARE-05, SHARE-08
**Success Criteria** (what must be TRUE):
  1. User can accept share request, choosing which overlay to share back and expiry option
  2. Both users can optionally add shared overlay source immediately on acceptance
  3. Share status indicators show active, expired, or revoked state in dashboard
  4. System prevents circular share dependencies (cycle detection blocks acceptance)
**Plans**: TBD

#### Phase 16: Shared Overlay Sources
**Goal**: Users can browse and add shared overlays as chat sources to their overlays
**Depends on**: Phase 15
**Requirements**: SOURCE-01, SOURCE-02, SOURCE-03
**Success Criteria** (what must be TRUE):
  1. "Shared Overlays" source type appears alongside platform sources (Twitch, YouTube, etc.)
  2. User can browse list of available shared overlays when adding source
  3. User can add shared overlay as source to any overlay via configuration UI
  4. Shared overlay source persists in configuration like platform sources
**Plans**: TBD

#### Phase 17: Message Routing
**Goal**: Messages from source overlay's aggregated chat are delivered to recipient's overlay with display settings isolation
**Depends on**: Phase 16
**Requirements**: SOURCE-04, SOURCE-05
**Success Criteria** (what must be TRUE):
  1. Messages from all sources in shared overlay appear in recipient's overlay in real-time
  2. Recipient's display settings (CSS, event filters) apply to shared messages, not source overlay's settings
  3. Shared messages are visually indistinguishable from platform source messages (unified rendering)
  4. Message enrichment (emotes, badges) works for shared messages identically to platform messages
**Plans**: TBD

#### Phase 18: Revocation
**Goal**: Users can revoke shares instantly with inactive source marking
**Depends on**: Phase 17
**Requirements**: SHARE-06, SHARE-07
**Success Criteria** (what must be TRUE):
  1. Either user can revoke share at any time from dashboard
  2. Revoked shares stop delivering messages within 1 second (cache invalidation)
  3. Revoked or expired shares are marked as inactive in overlay configuration (not deleted)
  4. User can distinguish active vs inactive shared sources in overlay editor
**Plans**: TBD

#### Phase 19: Lifecycle & Expiry
**Goal**: Shares auto-expire based on stream lifecycle or time duration
**Depends on**: Phase 18
**Requirements**: EXPIRY-01, EXPIRY-02, EXPIRY-03, EXPIRY-04, EXPIRY-05, EXPIRY-06
**Success Criteria** (what must be TRUE):
  1. User can choose expiry option when accepting: "This stream", "n hours", "Unlimited"
  2. Share auto-expires when either user's stream ends (if "This stream" selected)
  3. Share auto-expires after configured duration (if time-based selected)
  4. Twitch stream lifecycle detected via Helix API polling
  5. YouTube and TikTok stream lifecycle reuses existing listener detection (no new code needed)
  6. Kick stream lifecycle detection implemented or gracefully disabled (per research outcome)
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation + Twitch | v1.0 | 5/5 | Complete | 2026-02-18 |
| 2. YouTube Integration | v1.0 | 2/2 | Complete | 2026-02-18 |
| 3. Kick Integration + Edge Cases | v1.0 | 4/4 | Complete | 2026-02-18 |
| 5. Sharding Infrastructure & Coordinator | v1.1 | 5/5 | Complete | 2026-02-19 |
| 6. Connection Management & Migration | v1.1 | 8/8 | Complete | 2026-02-20 |
| 7. Dynamic Rebalancing & HPA | v1.1 | 4/4 | Complete | 2026-02-20 |
| 8. Observability & Production Readiness | v1.1 | 4/4 | Complete | 2026-02-20 |
| 9. Core Ingestion PoC | v1.2 | 5/5 | Complete | 2026-02-21 |
| 10. Production Minimum | v1.2 | 4/4 | Complete | 2026-02-21 |
| 11. Contract Validation | v1.2 | 4/4 | Complete | 2026-02-21 |
| 12. Production Rollout | v1.2 | 3/3 | Complete | 2026-03-05 |
| 13. Feature Parity | v1.2 | 5/5 | Complete | 2026-03-06 |
| 14. Foundation | v1.3 | 0/4 | Not started | - |
| 15. Share Acceptance | v1.3 | 0/TBD | Not started | - |
| 16. Shared Overlay Sources | v1.3 | 0/TBD | Not started | - |
| 17. Message Routing | v1.3 | 0/TBD | Not started | - |
| 18. Revocation | v1.3 | 0/TBD | Not started | - |
| 19. Lifecycle & Expiry | v1.3 | 0/TBD | Not started | - |

---
*Last updated: 2026-03-09 after Phase 14 planning*
