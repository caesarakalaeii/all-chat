# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.3 — Chat Overlay Sharing

**Shipped:** 2026-03-11
**Phases:** 6 (14-19) | **Plans:** 23 | **Commits:** 45

### What Was Built
- New share-service microservice with PostgreSQL schema, premium enforcement middleware, admin controls, background expiry job, and WebSocket notification relay
- Share acceptance flow: DFS cycle detection, SELECT FOR UPDATE transactions, bidirectional AddSourceModal, overlay-specific message deduplication, and color-coded status badges
- Shared overlay platform type wired end-to-end (DB migration → Go model → TypeScript → overlay editor UI)
- SQL UNION fan-out in `FindOverlaysForMessage` routes messages from source overlay to all recipient overlays without duplicate enrichment
- Atomic revocation: dual-UPDATE transaction (share_requests + overlay_chat_sources), real-time WS notifications, inactive source rendering in overlay editor
- Multi-platform stream lifecycle detection with 60s debounce: Twitch EventSub stream.offline, YouTube HandleStreamOffline publish, TikTok disconnected handler; Kick gracefully disabled

### What Worked
- **3-day delivery for a complex cross-service feature** — 6 phases, 23 plans, 246 files changed. The phased approach (foundation → acceptance → sources → routing → revocation → lifecycle) kept complexity manageable
- **Nyquist Wave 0 stubs (Phase 15+)** — RED test scaffolding before implementation caught integration gaps early and created clear test coverage from the start
- **Stub-based tests without pgxmock** — Phase 17 router tests used in-process stubs (no testcontainers/pgxmock dependency), keeping the test suite fast and dependency-free
- **Feature flag via graceful disable** — Rather than blocking progress on Kick lifecycle uncertainty, the AcceptModal gracefully disables "This stream" for Kick with a clear UI message. Correct decision.
- **Redis pub/sub for lifecycle relay** — Listeners publish lifecycle:stream_end; share-service subscribes. Redis ping failure is non-fatal. Loose coupling kept listeners unaware of share-service.

### What Was Inefficient
- **SHARE-05 gap** — AddSourceModal was built (15-02) but the full flow wasn't end-to-end verified as a complete user journey. The requirement remained unchecked at milestone close.
- **STATE.md progress percent** — Progress tracking showed 93% at milestone close (stale from earlier session). The state tracking needs to be updated more frequently during execution.
- **Phase 19 plan 02 outlier** — A 525,609-second duration entry in STATE.md performance tracking was clearly a leftover from a paused session, not real execution time.

### Patterns Established
- **Service-per-concern for premium features** — share-service owns all sharing logic (search, requests, acceptance, expiry, lifecycle) as a clean separation from overlay-manager
- **ON DELETE RESTRICT + application-layer cascade** — Explicit cleanup logic preferred over CASCADE deletes for audit trail and data safety
- **Redis lifecycle relay pattern** — Listeners publish domain events (lifecycle:stream_end) to Redis; dependent services (share-service) subscribe. Zero direct coupling.
- **60s debounce for stream lifecycle expiry** — Prevents false positive expiry during Twitch stream restarts/category changes; pattern reusable for any stream-lifecycle-triggered action

### Key Lessons
1. **Premium enforcement belongs at the API middleware layer** — Client-side bypass is trivial; server validates on every mutation, not just the initial gate
2. **UNION (not UNION ALL) for fan-out deduplication** — When overlays appear in both direct and shared branches, UNION ensures exactly-once delivery
3. **Shared overlay sources are immediately active** — `shared_overlay` platform type doesn't go through listener activation; set `is_active=true` at creation time (share already accepted)
4. **Revocation bypass on WS reconnect** — `ActivateSourcesForOverlay` must filter `shared_overlay` sources where `share_request.status IN (revoked, expired)` to prevent revoked sources from reactivating on reconnect

### Cost Observations
- Model mix: ~100% sonnet (balanced profile configured)
- Sessions: ~6 sessions across 3 days
- Notable: Fine granularity setting kept individual plans small and focused; wave parallelization kept execution fast

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Days | Key Change |
|-----------|--------|-------|------|------------|
| v1.0 | 3 | 11 | 1 | Initial deletion support |
| v1.1 | 4 | 21 | 3 | Load balancing infrastructure |
| v1.2 | 5 | 21 | 13 | InnerTube + production rollout |
| v1.3 | 6 | 23 | 3 | Cross-service feature with new microservice |

### Cumulative Quality

| Milestone | New Tests | Pattern |
|-----------|-----------|---------|
| v1.0 | 88.5% deletion coverage | Integration tests for pipeline |
| v1.1 | 16 tracing spans | Observability-first approach |
| v1.2 | 69 automated tests | Contract validation before prod |
| v1.3 | Nyquist RED stubs + modal TDD | Test scaffolding before implementation |

### Top Lessons (Verified Across Milestones)

1. **Drop-in replacement beats migration** — v1.2 InnerTube succeeded because schema compatibility was a hard constraint from day one. Applying this to v1.3: `shared_overlay` is a virtual platform type, not a new routing tier.
2. **Nyquist Wave 0 RED stubs prevent integration gaps** — First introduced in v1.2, proven essential in v1.3 for catching missing test coverage before implementation begins.
3. **Loose coupling via Redis pub/sub scales across milestones** — Used in v1.1 (load balancing), v1.2 (InnerTube integration), and v1.3 (lifecycle events). The pattern is battle-tested.
