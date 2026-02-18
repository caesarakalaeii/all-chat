# Roadmap: Message Deletion Support

## Overview

This milestone adds message deletion event support to All-Chat's multi-platform chat aggregation system. The journey progresses from establishing core infrastructure with the highest-confidence platform (Twitch IRC with real-time events), expanding to YouTube's polling-based model, adding Kick's reverse-engineered WebSocket events, and finally addressing TikTok limitations while implementing advanced reliability features. Each phase builds on proven infrastructure before expanding scope, ensuring deletion events flow reliably through the existing Redis Streams → Message Processor → Pub/Sub → WebSocket pipeline.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation + Twitch** - Message ID registry and end-to-end Twitch deletion flow (completed 2026-02-18)
- [x] **Phase 2: YouTube Integration** - Polling-based deletion detection for YouTube streams (completed 2026-02-18)
- [ ] **Phase 3: Kick Integration + Edge Cases** - Kick WebSocket events and reconnection handling
- [ ] **Phase 4: TikTok Documentation + Polish** - Document limitations and add reliability improvements

## Phase Details

### Phase 1: Foundation + Twitch
**Goal**: Establish message deletion infrastructure with Twitch platform, enabling single and bulk message deletion with <500ms latency
**Depends on**: Nothing (first phase)
**Requirements**: MSGID-01, MSGID-02, MSGID-03, MSGID-04, MSGID-05, DEL-01, DEL-02, DEL-03, DEL-04, DEL-05, DEL-06, RACE-01, RACE-02, RACE-03, TWITCH-01, TWITCH-02, TWITCH-03, TWITCH-04, FRONTEND-01, FRONTEND-02, FRONTEND-03, FRONTEND-04, FRONTEND-05, FRONTEND-06
**Success Criteria** (what must be TRUE):
  1. When Twitch moderator deletes single message, it disappears from overlay within 500ms
  2. When Twitch moderator times out user, all user's messages disappear from overlay as single batch event
  3. When Twitch moderator clears entire chat, all messages disappear from overlay
  4. Deletion events arriving before corresponding messages still remove messages correctly (no orphaned messages persist)
  5. Frontend receives and processes deletion events via WebSocket with message removal working across all event types
**Plans**: 5 plans in 3 waves

Plans:
- [x] 01-01-PLAN.md — Message ID Registry infrastructure (Redis hash, O(1) lookups, 1-hour TTL)
- [x] 01-02-PLAN.md — Twitch deletion event capture (IRC handlers for CLEARMSG/CLEARCHAT)
- [x] 01-03-PLAN.md — Message processor deletion handling (buffer, normalization, publishing)
- [x] 01-04-PLAN.md — Frontend deletion handling (WebSocket events, DOM removal)
- [x] 01-05-PLAN.md — End-to-end integration testing and verification

### Phase 2: YouTube Integration
**Goal**: Add YouTube message deletion support via polling-based detection with 30-60s latency
**Depends on**: Phase 1
**Requirements**: YOUTUBE-01, YOUTUBE-02, YOUTUBE-03
**Success Criteria** (what must be TRUE):
  1. When YouTube moderator deletes message, it disappears from overlay within 60 seconds
  2. YouTube deletion detection operates within existing quota limits (no additional API cost)
  3. YouTube deletion events flow through same Message ID Registry and pipeline established in Phase 1
**Plans**: 2 plans in 2 waves

Plans:
- [x] 02-01-PLAN.md — YouTube parser deletion event mapping to Phase 1 schema
- [x] 02-02-PLAN.md — YouTube registry integration and end-to-end verification

### Phase 3: Kick Integration + Edge Cases
**Goal**: Add Kick WebSocket deletion events and implement reconnection handling for all platforms
**Depends on**: Phase 2
**Requirements**: KICK-01, KICK-02, KICK-03, REL-01, REL-02, REL-03, REL-04, REL-05
**Success Criteria** (what must be TRUE):
  1. When Kick moderator deletes message, it disappears from overlay in real-time via WebSocket event
  2. When overlay WebSocket disconnects and reconnects, deletion events during disconnect window are replayed (1-minute buffer)
  3. Batch deletions of 1,000+ messages complete without blocking frontend UI (measured via load testing)
  4. All three platforms (Twitch, YouTube, Kick) handle deletion events consistently through unified pipeline
**Plans**: 3 plans in 2 waves

Plans:
- [ ] 03-01-PLAN.md — Kick deletion event handler (Pusher WebSocket, registry integration)
- [ ] 03-02-PLAN.md — Deletion replay buffer (Redis sorted set, reconnection resilience)
- [ ] 03-03-PLAN.md — Load testing and feature documentation (Artillery, platform matrix)

### Phase 4: TikTok Documentation + Polish
**Goal**: Document TikTok deletion limitation and validate production readiness across all platforms
**Depends on**: Phase 3
**Requirements**: TIKTOK-01, TIKTOK-02
**Success Criteria** (what must be TRUE):
  1. Documentation clearly states TikTok deletion events are unsupported (library limitation)
  2. TikTok messages display without errors despite lack of deletion support
  3. All platforms tested in production environment with real streams and moderation actions
  4. Message deletion feature documented in user-facing guides with platform capability matrix
**Plans**: TBD

Plans:
- [ ] 04-01: TBD during planning

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Twitch | 5/5 | Complete   | 2026-02-18 |
| 2. YouTube Integration | 2/2 | Complete   | 2026-02-18 |
| 3. Kick Integration + Edge Cases | 0/3 | Not started | - |
| 4. TikTok Documentation + Polish | 0/1 | Not started | - |
