# Pitfalls Research: Message Deletion in Streaming Chat Aggregation

**Domain:** Chat Aggregation - Message Deletion Events
**Researched:** 2026-02-17
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Message ID Mismatch Between Platform and System

**What goes wrong:**
Platform sends deletion event with **platform-native message ID** (e.g., Twitch's `target-msg-id`), but your system has already transformed messages to use **internal UUIDs**. Deletion events fail to match any messages because the ID tracking is broken.

**Why it happens:**
All-Chat generates new UUIDs when messages enter the system (`MessageID: uuid.New().String()` in `/services/message-processor/models/message.go`). Platform deletion events reference **original platform IDs** that were never preserved. The current architecture has no bidirectional ID mapping.

**How to avoid:**
- **Dual ID tracking**: Store both `platform_message_id` (original) and `internal_id` (UUID) in message models
- **Add to RawChatMessage**: New field `PlatformMessageID string` to preserve original ID
- **Add to UnifiedChatMessage**: Field `PlatformMessageID string` for deletion matching
- **Implement ID registry**: Redis hash `msg:platform_id_to_internal` with 15-minute TTL (messages older than this unlikely to be deleted)

**Warning signs:**
- Deletion events arrive but no messages disappear from overlays
- Logs show "message not found" errors when processing deletion events
- Users manually refreshing overlays to remove deleted messages

**Phase to address:**
Phase 1 (Foundation) - MUST establish ID tracking before implementing deletion events, or all deletion features will fail.

---

### Pitfall 2: Race Condition Between Message Display and Deletion Event

**What goes wrong:**
Deletion event arrives **before** the original message reaches overlay clients via WebSocket. Overlay stores deletion event, original message then displays, and **never gets deleted** (message persists indefinitely on screen).

**Why it happens:**
Message flow: Listener → Redis Streams → Message Processor (enrichment takes 50-200ms with emote API calls) → Redis Pub/Sub → API Gateway → WebSocket. Deletion events often bypass enrichment and arrive faster. This is a classic **out-of-order delivery** problem in distributed systems.

Reference: [Causal Consistency in Distributed Systems](https://systemd.imshawan.dev/1-Fundamentals/1.5-Consistency-Models-Strong-Eventual-Causal/4-Causal-Consistency/) - "causal consistency ensures that the reply always appears after the original message, regardless of the delivery order."

**How to avoid:**
- **Tombstone buffer**: Overlay frontend maintains 5-second buffer of "pending deletions" for messages not yet received
- **Sequence numbers**: Add monotonic counter per channel (`channel:sequence_number` in Redis) to messages and deletion events
- **Deletion event enrichment**: Route deletion events through same Message Processor pipeline (DO NOT fast-track) to preserve ordering
- **Client-side queue**: Frontend queues incoming messages for 200ms before display (allows late deletions to arrive first)

**Warning signs:**
- Messages "flicker" (appear then immediately disappear)
- Deleted messages visible on some overlay instances but not others
- Moderators report deletions "not working" intermittently (race condition is timing-dependent)

**Phase to address:**
Phase 2 (Event Processing) - After basic deletion works, validate ordering guarantees with stress testing.

---

### Pitfall 3: Batch Deletion Amplification (Ban/Timeout Events)

**What goes wrong:**
Moderator times out user with 5,000 messages in chat history. System attempts to **broadcast 5,000 individual deletion events** to all overlay WebSocket clients. This triggers:
- API Gateway CPU spike (5,000 JSON serializations)
- Network saturation (5,000 × number of connected clients WebSocket frames)
- Frontend JavaScript hangs (5,000 DOM manipulations)
- Redis Pub/Sub backlog accumulation

**Why it happens:**
Platforms send `CLEARCHAT` (Twitch) or batch deletion events referencing **all messages by user**, not individual message IDs. Naive implementation: "for each message in history by user: send deletion event." With ephemeral messages (Redis-only, no database), you must scan Redis Streams backlog or frontend's in-memory message buffer.

Reference: [Broadcasting WebSockets Messages](https://websockets.readthedocs.io/en/stable/topics/broadcast.html) - "Calling broadcast() once is more efficient than calling send() in a loop."

**How to avoid:**
- **Coalesced deletion events**: Single event type `{type: "bulk_delete", user_id: "123", reason: "timeout"}` instead of N individual deletions
- **Frontend-side filtering**: Overlay client removes all messages matching `user_id` with single filter operation
- **Limit historical scope**: Only delete messages in frontend buffer (last 100-200 messages displayed), ignore older messages
- **Throttling**: Batch deletion events trigger cooldown (prevent rapid consecutive bulk deletes)

**Warning signs:**
- API Gateway CPU spikes correlate with timeouts/bans in logs
- WebSocket disconnections during moderation actions (clients can't keep up)
- Frontend performance degradation after bulk deletions (DOM thrashing)
- Redis Pub/Sub subscribers lag (`PUBSUB NUMSUB` shows delayed delivery)

**Phase to address:**
Phase 1 (Foundation) - Design deletion event schema to support bulk operations from the start, or refactoring later is expensive.

---

### Pitfall 4: Platform-Specific Deletion Event Gaps

**What goes wrong:**
Assume all platforms provide real-time deletion events. Reality:
- **Twitch IRC**: `CLEARMSG` (single), `CLEARCHAT` (user/full chat) - **immediate**
- **YouTube Live Chat API**: Polling-based, **no push notifications for deletions** - must detect via absence in next poll (60-second delay)
- **Kick Pusher**: Deletion events exist but **undocumented** - reverse-engineer WebSocket payloads
- **TikTok**: Unofficial library - **deletions not supported** at all

Result: Inconsistent deletion experience across platforms. Twitch deletions are instant, YouTube deletions lag 60 seconds, TikTok deletions never work.

**Why it happens:**
Each platform has different moderation APIs and event notification systems. All-Chat must adapt to **lowest common denominator** or implement platform-specific workarounds. Current architecture assumes uniform event delivery (wrong assumption).

Reference: [Twitch IRC Documentation](https://dev.twitch.tv/docs/chat/irc/) - CLEARMSG and CLEARCHAT specifications
Reference: [YouTube Live Chat API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete) - DELETE operation, no webhook push

**How to avoid:**
- **Platform capability matrix**: Document which platforms support real-time deletions (Phase 1)
- **Polling fallback**: YouTube Listener fetches message list on each poll, diffs with previous poll to detect deletions (Phase 2)
- **Graceful degradation**: Display notice "YouTube deletions delayed 60s" in overlay settings
- **Event normalization layer**: Standardize deletion events regardless of how platform delivers them

**Warning signs:**
- Feature works perfectly on Twitch test streams, fails on YouTube
- User reports "deletions don't work" but only for specific platforms
- Logs show deletion events for some platforms, silence for others

**Phase to address:**
Phase 1 (Foundation) - Document platform capabilities before implementation. Phase 2 (Platform Integration) - Implement platform-specific adapters.

---

### Pitfall 5: Ephemeral Architecture Message Lookup Failure

**What goes wrong:**
Deletion event arrives: `{platform_message_id: "abc123"}`. To delete, need to find `overlay_id` to know which Redis Pub/Sub channel to publish deletion to. But messages are **ephemeral** (Redis Streams with `MAXLEN`, no database persistence). Message `abc123` was already trimmed from stream. Cannot determine target overlay. **Deletion event is dropped.**

**Why it happens:**
All-Chat uses Redis Streams with `MAXLEN ~10000` (approximate trimming) for `chat:raw`. Old messages are evicted. Deletion events can arrive **minutes later** (e.g., moderator reviewing logs and deleting old messages). By then, original message is gone from Redis Streams, and there's no database to query.

Reference: Current architecture in `/docs/architecture/01-DATA-FLOW.md` - "Redis Streams: Durable message queues" with MAXLEN limits, no database persistence for message content.

**How to avoid:**
- **Short-term ID mapping cache**: Redis hash `deletion:msg_map` stores `{platform_msg_id: overlay_id}` for 15 minutes after message published
- **Optimistic deletion**: If overlay unknown, broadcast deletion to **all active overlays** for this channel (may delete from overlays that never received it, but harmless)
- **Deletion event TTL**: Ignore deletion events older than 10 minutes (stale deletions unlikely to have visible messages)
- **Accept partial success**: Some deletions may fail for old messages - document this limitation

**Warning signs:**
- Deletion success rate decreases over time (newer deletions work, old deletions fail)
- Cannot delete messages older than ~5 minutes
- Logs show "overlay_id not found for message" errors

**Phase to address:**
Phase 1 (Foundation) - Implement ID mapping cache before deletion feature launches, or deletion reliability will be poor.

---

### Pitfall 6: Multiple Overlay Instances Race (Split-Brain Deletion)

**What goes wrong:**
User has 3 overlays displaying same Twitch channel. Deletion event arrives. API Gateway has 3 connection pools, each with multiple WebSocket clients. Race occurs:
- Pool 1: Receives deletion, removes message
- Pool 2: Receives deletion, removes message
- Pool 3: Pub/Sub subscriber lags, message still visible

OR

- Pool 1-3: All receive deletion, but different clients render at different times (network latency). Viewer sees message deleted on one screen, still visible on another screen for 2-5 seconds.

**Why it happens:**
Redis Pub/Sub has **at-most-once delivery** semantics. If API Gateway subscriber is slow or reconnecting, it may miss deletion events. Also, broadcasting to multiple WebSocket clients is **not atomic** - each client processes deletion independently.

Reference: [Redis Pub/Sub Documentation](https://redis.io/docs/interact/pubsub/) - "At most once delivery" - messages can be lost if no subscribers at publish time.
Reference: [WebSocket Broadcasting](https://tutorialedge.net/projects/chat-system-in-go-and-react/part-4-handling-multiple-clients/) - "Calling broadcast() once is more efficient than calling send() in a loop."

**How to avoid:**
- **Deletion event persistence**: Store deletion events in Redis with 1-minute TTL (`deletion_events:{overlay_id}` Set). New WebSocket connections replay recent deletions on connect.
- **Idempotent deletions**: Frontend safely handles duplicate deletion events (delete message by ID, if not found, silently ignore)
- **Subscriber health checks**: Monitor Redis Pub/Sub subscriber lag (`PUBSUB CHANNELS` and `PUBSUB NUMSUB`), alert if subscriber count drops
- **Deletion acknowledgment**: Track which clients acknowledged deletion (complex, defer to Phase 3+)

**Warning signs:**
- "Message still shows on overlay" reports after confirmed deletion
- Deletion works on dashboard but not on viewer overlay (different WebSocket connection pools)
- Refreshing overlay makes deleted messages disappear (catches up on reconnect)

**Phase to address:**
Phase 2 (Event Processing) - Validate broadcast reliability with multiple client testing.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Use internal UUID only, ignore platform message ID | Simpler data model | Deletion events cannot match messages | **Never** - breaks core feature |
| Skip sequence numbering, assume in-order delivery | Faster initial implementation | Race conditions between message and deletion | Only for Phase 0 prototype |
| Broadcast individual deletions for batch events | Simpler deletion logic | API Gateway and frontend performance collapse at scale | Only for MVP with <50 messages/overlay |
| No deletion event persistence, rely on live Pub/Sub | Less Redis storage | Clients miss deletions during reconnect | Only if reconnect rate <0.1% |
| Single deletion event type, no platform-specific handling | Unified event schema | Cannot leverage platform-specific optimizations | Acceptable for Phase 1, refactor in Phase 2 |
| Frontend-only deletion (no backend tracking) | No backend changes needed | Cannot audit deletions, inconsistent state | Only for Phase 0 prototype |

---

## Integration Gotchas

| Platform | Common Mistake | Correct Approach |
|----------|----------------|------------------|
| **Twitch IRC** | Parse `CLEARMSG` but miss `CLEARCHAT` (different event types for single vs bulk) | Handle both: `CLEARMSG` → single deletion, `CLEARCHAT` → user bulk or full clear |
| **Twitch IRC** | Use `msg.ID` from `go-twitch-irc` library as message ID | Wrong - that's IRC message ID. Use `id` tag from IRC tags (platform message ID) |
| **YouTube** | Poll API every 5 seconds to detect deletions | Exceeds API quota (10,000 units/day). Poll every 30-60 seconds, batch operations |
| **YouTube** | Assume deletion API returns deleted message content | Wrong - DELETE returns 204 No Content. Must track messages client-side to know what was deleted |
| **Kick Pusher** | Assume Pusher events match Twitch IRC event names | Wrong - Kick uses different event names (`ChatMessageDeleted` not `CLEARMSG`). Reverse-engineer from browser WebSocket |
| **TikTok** | Expect deletion events from unofficial library | Not supported - TikTok library provides message create only. May need to implement "hide message" client-side for reported spam |
| **All Platforms** | Use message text for matching deletions | Fragile - text may be truncated, contain emotes, or be modified. Always use message ID |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| **Linear scan for message lookup** | Deletion latency increases with message volume | Use Redis hash for O(1) ID lookup, not O(n) stream scan | >1,000 messages in buffer |
| **DOM thrashing on bulk delete** | Frontend freezes during timeouts | Batch DOM updates: `requestAnimationFrame()` + `DocumentFragment` | >100 simultaneous deletions |
| **Pub/Sub channel per message** | Redis connection exhaustion | Single channel per overlay, multiplex deletion events within | >10,000 active overlays |
| **No deletion event aggregation** | Network bandwidth saturation | Coalesce deletions: send `{deleted_ids: [1,2,3]}` not 3 separate events | >50 deletions/second/overlay |
| **Synchronous WebSocket broadcast** | API Gateway blocks on slow clients | Async broadcast with timeout (drop slow clients after 5 seconds) | Any slow client blocks all others |
| **Unbounded deletion history** | Memory leak in frontend | Limit deletion buffer to 1,000 most recent, evict older | After 24 hours uptime |

---

## Platform-Specific Edge Cases

### Twitch IRC Quirks

**CLEARCHAT with no target user**: Clears entire chat
```
@room-id=12345;tmi-sent-ts=1642715695392 :tmi.twitch.tv CLEARCHAT #channel
```
**Action**: Broadcast `{type: "clear_all"}` to overlay

**CLEARCHAT with ban duration**: Timeout (temporary) vs ban (permanent)
```
@ban-duration=600;room-id=12345;target-user-id=98765 :tmi.twitch.tv CLEARCHAT #channel :username
```
**Action**: If `ban-duration` tag exists = timeout, else = permanent ban

**CLEARMSG target-msg-id format**: UUID string
```
@target-msg-id=94e6c7ff-bf98-4faa-af5d-7ad633a158a9 :tmi.twitch.tv CLEARMSG #channel :message text
```
**Action**: Extract `target-msg-id` tag value, map to internal ID

**Missing message text in CLEARMSG**: Text may be empty or truncated
```
@target-msg-id=abc :tmi.twitch.tv CLEARMSG #channel :
```
**Action**: Never rely on message text for deletion matching, only use `target-msg-id`

Reference: [Twitch IRC CLEARMSG Documentation](https://discuss.dev.twitch.com/t/message-deletion-confusion/19311)

### YouTube API Quirks

**No real-time deletion events**: Must poll and diff to detect deletions
**Action**: Cache previous poll's message IDs, compare with current poll, deleted IDs = difference

**Rate limit on DELETE operation**: 100 units per call (daily quota 10,000 units)
**Action**: All-Chat **cannot initiate deletions** on YouTube (only display deletions initiated by moderators via YouTube interface)

**liveChatMessages.delete requires OAuth**: Must use streamer's access token, not bot token
**Action**: YouTube sources in overlays must have OAuth consent from channel owner

**Deleted message not removed from list immediately**: API caching delay (30-60 seconds)
**Action**: Display "Deletion requested" in UI, don't expect instant confirmation

Reference: [YouTube liveChatMessages: delete API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete)

### Kick Platform Quirks

**Undocumented Pusher events**: Kick uses private Pusher WebSocket, events not in public API docs
**Action**: Reverse-engineer by monitoring browser DevTools WebSocket traffic

**Event name guess**: Likely `ChatMessageDeleted` or `message.deleted` (not confirmed)
**Action**: Phase 1 research task - capture real deletion event from Kick stream

**Chatroom ID vs Channel ID**: Kick has two identifiers - ensure using correct one for Pusher channel name
**Action**: Test deletion on Kick staging environment before production

### TikTok Platform Quirks

**No deletion support in unofficial library**: `TikTokLiveClient` does not emit deletion events
**Action**: Consider implementing client-side "hide message" feature for spam reports (user-initiated, not platform-initiated)

**Connection stability issues**: Library drops connection frequently, may miss events
**Action**: Focus TikTok implementation on message display, deprioritize deletion feature (low ROI)

---

## "Looks Done But Isn't" Checklist

- [ ] **Deletion works on Twitch test stream**: Did you test YouTube, Kick, TikTok? (Platform-specific bugs)
- [ ] **Deletion works with single message**: Did you test timeout/ban (bulk deletion)? (Amplification issue)
- [ ] **Deletion works in dashboard**: Did you test viewer overlay? (WebSocket connection pools differ)
- [ ] **Deletion event arrives in logs**: Did you verify frontend received and processed it? (Pub/Sub delivery gap)
- [ ] **Message disappears immediately**: Did you test with 200ms delay race condition? (Out-of-order delivery)
- [ ] **ID matching works now**: Did you test after message evicted from Redis Streams? (Ephemeral architecture lookup failure)
- [ ] **Deletion works in browser DevTools**: Did you test in OBS browser source (CEF)? (Chromium version differences)
- [ ] **Deletion works with 10 messages**: Did you test with 1,000 messages on screen? (Performance scaling)
- [ ] **Single overlay test passes**: Did you test with 5 overlays on same channel? (Split-brain deletion)
- [ ] **Deletion works during normal operation**: Did you test during API Gateway restart? (Reconnection gaps)

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| **No platform ID preserved** | **HIGH** - requires data model change + migration | 1. Add `PlatformMessageID` to models, 2. Deploy new listeners to populate field, 3. Update processor to preserve it, 4. Deploy deletion feature |
| **Race condition in production** | **MEDIUM** - frontend hotfix | 1. Add 200ms message queue buffer in frontend, 2. Implement tombstone logic for pending deletions, 3. Redeploy frontend |
| **Batch deletion performance collapse** | **MEDIUM** - schema change + frontend optimization | 1. Deploy coalesced deletion event format, 2. Update frontend to handle bulk events, 3. Add throttling to API Gateway |
| **YouTube deletions don't work** | **LOW** - expected limitation | 1. Document "YouTube deletions delayed 60s", 2. Add polling diff logic, 3. Display notice in UI |
| **Ephemeral message lookup fails** | **LOW** - add caching layer | 1. Deploy Redis hash for ID mapping, 2. Set 15-minute TTL, 3. Accept old messages may not delete |
| **Split-brain deletion across clients** | **HIGH** - architecture refactor | 1. Implement deletion event persistence, 2. Add reconnection replay logic, 3. Monitor Pub/Sub subscriber lag |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| **Message ID mismatch** | Phase 1 (Foundation) | Unit test: deletion event matches message after ID transformation |
| **Race condition (message vs deletion)** | Phase 2 (Event Processing) | Integration test: deletion arrives before message, overlay still deletes correctly |
| **Batch deletion amplification** | Phase 1 (Foundation) | Load test: 1,000 messages deleted, measure CPU/network/frontend latency |
| **Platform-specific gaps** | Phase 1 (Foundation) + Phase 2 | Feature matrix: document which platforms support real-time deletion |
| **Ephemeral lookup failure** | Phase 1 (Foundation) | Chaos test: delete message 10 minutes after publish, measure success rate |
| **Split-brain deletion** | Phase 2 (Event Processing) | Multi-client test: 10 WebSocket connections to same overlay, all receive deletion |

---

## Testing Requirements for Deletion Feature

### Unit Tests (Phase 1)
- Parse Twitch `CLEARMSG` IRC message → extraction of `target-msg-id`
- Parse Twitch `CLEARCHAT` IRC message → extraction of `target-user-id` and `ban-duration`
- Map platform message ID to internal UUID via Redis hash
- Serialize/deserialize deletion event JSON schema

### Integration Tests (Phase 2)
- Send message → send deletion → verify overlay receives both in order
- Send deletion → send message → verify overlay queues deletion and applies when message arrives
- Send 100 messages by user → timeout user → verify single bulk deletion event (not 100 individual)
- Delete message from Redis cache → deletion event arrives → verify graceful failure (log warning, don't crash)

### E2E Tests (Phase 3)
- Twitch IRC: `/delete <message>` in chat → overlay removes message within 2 seconds
- Twitch IRC: `/timeout <user> 600` → all user messages removed from overlay
- YouTube: Delete message in YouTube Studio → overlay removes message within 60 seconds
- Multi-overlay: 3 overlays on same channel → delete message → all overlays update consistently

### Load Tests (Phase 3)
- 10,000 messages/minute + 100 deletions/minute → measure API Gateway CPU/memory
- 1,000 WebSocket clients → broadcast deletion event → measure P95 delivery latency
- Bulk delete 5,000 messages → measure frontend render time (should be <100ms)

### Chaos Tests (Phase 3)
- API Gateway restart during deletion event → reconnect → verify deletion replayed
- Redis Pub/Sub network partition → verify subscriber lag detection and alert
- Message Processor crash after message published, before deletion → verify deletion still works (ID mapping cache survives)

---

## Sources

**Platform API Documentation:**
- [Twitch IRC Concepts](https://dev.twitch.tv/docs/chat/irc/) - CLEARMSG and CLEARCHAT specifications
- [Twitch IRC Migration Guide](https://dev.twitch.tv/docs/chat/irc-migration/) - EventSub alternatives
- [YouTube liveChatMessages: delete API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete) - DELETE operation details
- [Twitch Developer Forum: Message Deletion Confusion](https://discuss.dev.twitch.com/t/message-deletion-confusion/19311) - Community discussion on CLEARMSG

**Distributed Systems Consistency:**
- [Causal Consistency in Distributed Systems](https://systemd.imshawan.dev/1-Fundamentals/1.5-Consistency-Models-Strong-Eventual-Causal/4-Causal-Consistency/) - Message ordering guarantees
- [Eventual Consistency in Distributed Systems](https://www.geeksforgeeks.org/system-design/eventual-consistency-in-distributive-systems-learn-system-design/) - Consistency models for chat
- [Consistency Patterns in Distributed Systems](https://www.designgurus.io/blog/consistency-patterns-distributed-systems) - Strong vs eventual consistency tradeoffs

**WebSocket Broadcasting:**
- [Broadcasting - websockets documentation](https://websockets.readthedocs.io/en/stable/topics/broadcast.html) - Efficient broadcast patterns
- [Broadcasting WebSockets Messages across FastAPI instances](https://medium.com/@philipokiokio/broadcasting-websockets-messages-across-instances-and-workers-with-fastapi-9a66d42cb30a) - Multi-instance challenges
- [WebSockets with Elixir - How to Sync Multiple Clients](https://www.viget.com/articles/websockets-with-elixir-how-to-sync-multiple-clients) - Client synchronization strategies

**Ephemeral Messaging:**
- [Ephemeral Chat Messages](https://getstream.io/blog/ephemeral-chat-messages/) - Self-destructing message patterns
- [Ephemeral group chat patent](https://patents.google.com/patent/WO2016179235A1/en) - Race conditions in group chat with deletion triggers
- [Compliance Challenges of Ephemeral Messaging](https://mco.mycomplianceoffice.com/blog/the-compliance-challenges-of-ephemeral-messaging) - Audit and retention concerns

**Chat Moderation APIs:**
- [StreamElements Nuke Command](https://docs.streamelements.com/chatbot/commands/default/nuke) - Bulk moderation patterns
- [Moderation for Chat - React Docs](https://getstream.io/chat/docs/react/moderation/) - Client-side moderation implementation
- [YouTube Live Chat Bans API](https://developers.google.com/youtube/v3/live/docs/liveChatBans) - Ban/timeout API specifications

**IRC Protocol Libraries:**
- [go-twitch-irc on GitHub](https://github.com/gempir/go-twitch-irc) - Go IRC client used by All-Chat
- [dank-twitch-irc on GitHub](https://github.com/robotty/dank-twitch-irc) - Node.js IRC client with CLEARMSG support
- [ClearchatMessage Documentation](https://robotty.github.io/dank-twitch-irc/classes/clearchatmessage.html) - CLEARCHAT message structure

**All-Chat Codebase:**
- `/services/message-processor/models/message.go` - Current message data models (UUID generation)
- `/services/api-gateway/websocket/pool.go` - WebSocket connection pool and broadcasting
- `/docs/architecture/01-DATA-FLOW.md` - Complete message flow architecture

---

*Pitfalls research for: Message Deletion in Streaming Chat Aggregation*
*Researched: 2026-02-17*
