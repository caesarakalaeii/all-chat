# Requirements: Message Deletion Support

**Defined:** 2026-02-17
**Core Value:** When a message is deleted on the streaming platform, it must be removed from connected overlays in real-time so streamers see an accurate representation of chat.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Message ID Infrastructure

- [ ] **MSGID-01**: System preserves platform-native message IDs alongside internal UUIDs
- [ ] **MSGID-02**: Redis-based Message ID Registry maps platform IDs to internal UUIDs
- [ ] **MSGID-03**: Registry entries have 24-hour TTL to match message retention
- [ ] **MSGID-04**: Registry provides O(1) lookup for deletion event matching
- [ ] **MSGID-05**: Platform IDs flow through entire pipeline (Listener → Processor → Gateway)

### Core Deletion Features

- [ ] **DEL-01**: System detects single message deletion events from platforms
- [ ] **DEL-02**: System detects user batch deletion events (timeout/ban)
- [ ] **DEL-03**: System detects full chat clear events
- [ ] **DEL-04**: Deletion events normalized to unified schema across all platforms
- [ ] **DEL-05**: Deletion events propagate through existing Redis Streams → Pub/Sub pipeline
- [ ] **DEL-06**: Batch deletions use coalesced schema to prevent amplification (single event for multiple messages)

### Race Condition Handling

- [ ] **RACE-01**: System buffers deletion events for messages not yet received (60-second window)
- [ ] **RACE-02**: Deletion events processed after corresponding messages arrive
- [ ] **RACE-03**: Expired deletion events (no matching message after 60s) are discarded without error

### Twitch Integration

- [ ] **TWITCH-01**: Listener detects IRC CLEARMSG events (single message deletion)
- [ ] **TWITCH-02**: Listener detects IRC CLEARCHAT with target-msg-id (user timeout/ban)
- [ ] **TWITCH-03**: Listener detects IRC CLEARCHAT without target (full chat clear)
- [ ] **TWITCH-04**: Twitch deletion events include target-msg-id for message matching

### YouTube Integration

- [ ] **YOUTUBE-01**: Listener polls for messageDeletedEvent message type
- [ ] **YOUTUBE-02**: YouTube deletion events processed within existing polling interval
- [ ] **YOUTUBE-03**: System handles 60-second polling lag gracefully (via deletion buffer)

### Kick Integration

- [ ] **KICK-01**: Listener detects ChatMessageDeletedEvent via WebSocket
- [ ] **KICK-02**: Kick event structure validated in production environment
- [ ] **KICK-03**: Kick deletion events include message ID for matching

### TikTok Handling

- [ ] **TIKTOK-01**: System documents TikTok deletion limitation (unsupported by unofficial library)
- [ ] **TIKTOK-02**: TikTok messages handled gracefully (no deletion support, no errors)

### Frontend Integration

- [ ] **FRONTEND-01**: Frontend tracks platform message IDs in DOM elements
- [ ] **FRONTEND-02**: Frontend receives deletion events via WebSocket
- [ ] **FRONTEND-03**: Frontend removes messages immediately on deletion event (no animation)
- [ ] **FRONTEND-04**: Frontend handles single message deletion
- [ ] **FRONTEND-05**: Frontend handles batch deletion (timeout/ban)
- [ ] **FRONTEND-06**: Frontend handles full chat clear

### Reliability & Edge Cases

- [ ] **REL-01**: Deletion events persisted for 1-minute replay window on reconnect
- [ ] **REL-02**: WebSocket reconnection triggers deletion event replay
- [ ] **REL-03**: System handles Redis Pub/Sub message loss gracefully (best-effort delivery acceptable)
- [ ] **REL-04**: Load testing validates batch deletion performance (1,000+ messages)
- [ ] **REL-05**: DOM update optimization prevents UI blocking during large deletions

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### UX Enhancements

- **UX-01**: Configurable deletion animation (instant vs 200ms fade)
- **UX-02**: Scroll position preservation during deletion
- **UX-03**: System messages for timeout/ban events (e.g., "User timed out for 10 minutes")

### Advanced Features

- **ADV-01**: Moderator ghost mode (view deleted messages)
- **ADV-02**: 30-day deletion audit log in Redis
- **ADV-03**: Batch delete by keyword feature
- **ADV-04**: Deletion metrics for spam detection

### TikTok Workaround

- **TIKTOK-03**: Client-side TTL-based message removal as deletion alternative

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Message editing | Platforms don't universally support editing; different feature entirely |
| Moderation context display | Focus on removal only, not reason/metadata (ban duration, mod name, etc.) |
| Deletion undo/restoration | Not supported by platforms; adds significant complexity |
| Deletion analytics/logging | Metrics can be added later; focus on core feature first |
| Historical message deletion | Messages are ephemeral (Redis only); no database cleanup needed |
| Cross-platform deletion | Deleting message on one platform doesn't delete on others (each platform independent) |
| Moderator attribution | Showing which mod deleted message adds complexity, defer to v2 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| (To be filled during roadmap creation) | | |

**Coverage:**
- v1 requirements: 0 total
- Mapped to phases: 0
- Unmapped: 0 ⚠️

---
*Requirements defined: 2026-02-17*
*Last updated: 2026-02-17 after initial definition*
