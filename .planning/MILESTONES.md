# Milestones

## v1.3 Chat Overlay Sharing (Shipped: 2026-03-11)

**Phases completed:** 6 phases (14-19), 23 plans
**Timeline:** 3 days (2026-03-09 → 2026-03-11)
**Files changed:** 246 files, 28,236 insertions

**Key accomplishments:**
- New share-service with PostgreSQL schema, premium enforcement middleware, admin controls, and background expiry job
- Share acceptance with DFS cycle detection, atomic SELECT FOR UPDATE transactions, bidirectional AddSource prompts, and overlay-specific deduplication
- Shared overlay platform type (`shared_overlay`) wired end-to-end: DB migration, Go model validPlatforms map, TypeScript union types, and overlay editor UI
- SQL UNION fan-out in `FindOverlaysForMessage` routes messages from source overlay to all recipient overlays in real-time
- Atomic revocation endpoint with dual-UPDATE transaction, real-time WS notifications, and inactive source rendering in overlay editor (greyed out at 50% opacity with StatusBadge)
- Multi-platform stream lifecycle detection: Twitch EventSub stream.offline, YouTube HandleStreamOffline publish, TikTok stream_end in disconnected handler — all with 60s debounce via LifecycleSubscriber; Kick gracefully disabled in AcceptModal

**Known Gaps:**
- SHARE-05: On acceptance, both users can optionally add shared source to an overlay immediately — AddSourceModal built but requirement not verified through full flow test

---

## v1.0: Message Deletion Support (Partial)

**Status:** Archived (80% complete - 3 of 4 phases)
**Completed:** 2026-02-18
**Last Phase:** Phase 3

### What Shipped

**Phases Completed:**
- ✓ Phase 1: Core Deletion Infrastructure (5 plans)
  - Message ID registry (Redis-based, 1-hour TTL)
  - Twitch deletion event capture (CLEARMSG, CLEARCHAT)
  - Message processor deletion handling
  - Frontend deletion event handling (React)
  - End-to-end integration verified

- ✓ Phase 2: YouTube Integration (2 plans)
  - YouTube deletion event parser mapping
  - YouTube registry integration

- ✓ Phase 3: Kick Integration Edge Cases (4 plans)
  - Kick deletion event handler
  - WebSocket reconnection replay buffer (Redis sorted sets)
  - Batch deletion load testing infrastructure
  - Replay buffer test compilation fix

**Requirements Delivered:**
- ✓ Message ID tracking across platforms (Twitch, YouTube, Kick)
- ✓ Single message deletion propagation
- ✓ Batch deletion (timeout/ban) propagation
- ✓ Real-time removal from overlays
- ✓ Replay buffer for reconnection scenarios
- ✓ 88.5% test coverage for deletion pipeline

**Not Completed:**
- ✗ Phase 4: TikTok Integration (deferred)
  - TikTok deletion event support
  - Final cross-platform verification

### Metrics

**Performance:**
- Total plans: 11
- Total time: 0.95 hours
- Average: 4.6 minutes per plan
- Velocity trend: Improving (Phase 3 avg: 3.5 min)

**Test Coverage:**
- Message processor: 88.5%
- API Gateway deletion: Covered
- Frontend deletion: Implemented

---
*Last updated: 2026-02-19*

## v1.1 Listener Load Balancing (Shipped: 2026-02-21)

**Phases completed:** 7 phases, 32 plans, 29 tasks

**Key accomplishments:**
- (none recorded)

---


## v1.2 InnerTube YouTube Listener (Shipped: 2026-03-06)

**Phases completed:** 12 phases, 53 plans, 50 tasks

**Key accomplishments:**
- (none recorded)

---

