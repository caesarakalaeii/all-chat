# Phase 1: Foundation + Twitch - Context

**Gathered:** 2026-02-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the core message deletion infrastructure by implementing a Message ID Registry (Redis hash mapping platform IDs to internal UUIDs), normalizing deletion events through the existing message pipeline (Redis Streams → Message Processor → Pub/Sub → API Gateway), and enabling real-time removal of messages from overlays when Twitch moderators delete them. Supports three deletion types: single message (CLEARMSG), user timeout/ban (CLEARCHAT with target), and full chat clear (CLEARCHAT without target). Foundation phase validates architecture before expanding to other platforms.

</domain>

<decisions>
## Implementation Decisions

### Message ID Tracking
- Add platform IDs to registry at listener capture (as soon as message arrives from Twitch IRC)
- Registry entries have 1-hour TTL (balance between memory usage and deletion window)
- Registry mapping direction: Claude's discretion (choose between unidirectional platform ID → UUID or bidirectional based on implementation needs)

### Deletion Event Flow
- Single event type with type field differentiating single/batch/clear deletions (not separate event types per deletion kind)
- Deletion events flow through same pipeline as regular messages: Redis Streams → Message Processor → Pub/Sub → API Gateway (maintains ordering and consistency)
- Batch deletions (timeout/ban) represented as coalesced events with user_id (frontend removes all messages matching that user, not individual message ID array)

### Race Condition Strategy
- Backend buffer approach: Message Processor holds deletion events in Redis when target message not yet in registry
- Buffered deletion events expire after 60 seconds if message never arrives
- Redis buffer structure: Claude's discretion (choose between sorted set with timestamp scores or hash with TTL based on access patterns)

### Frontend Removal Behavior
- Instant removal from DOM (no fade animation, no placeholder)
- Message ID tracking in DOM: Claude's discretion (choose between data attributes, React state, or hybrid based on React patterns)
- Batch deletion optimization: Claude's discretion (choose between batched DOM updates, virtual list updates, or progressive removal based on overlay implementation)

### Claude's Discretion
- Registry mapping direction (unidirectional vs bidirectional)
- Redis buffer data structure (sorted set vs hash with TTL)
- Frontend DOM tracking approach (data attributes, React state, or hybrid)
- Batch deletion DOM optimization strategy
- Error handling for registry misses
- Logging and metrics for deletion events

</decisions>

<specifics>
## Specific Ideas

- User emphasized immediate removal: "Remove completely from overlay" (no animation, no [deleted] placeholder)
- Phase focuses on Twitch only to validate architecture before expanding to YouTube/Kick
- Research identified critical dependency: Message ID Registry must exist before any deletion features work

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation-twitch*
*Context gathered: 2026-02-17*
