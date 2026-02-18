# Message Deletion Feature

## Overview

All-Chat supports real-time message deletion synchronization across all supported streaming platforms. When a moderator deletes a message on the streaming platform, it is automatically removed from connected overlays, ensuring streamers see an accurate representation of chat.

**Latency:**
- Twitch: <500ms (IRC real-time events)
- YouTube: 30-60s (HTTP polling-based detection)
- Kick: <500ms (Pusher WebSocket real-time events)
- TikTok: Not supported (library limitation)

## Platform Capabilities

| Platform | Single Message | Batch Deletion | Clear Chat | Latency | Event Source |
|----------|----------------|----------------|------------|---------|--------------|
| Twitch   | ✅ Yes         | ✅ Yes (timeout/ban) | ✅ Yes | <500ms  | IRC (CLEARMSG/CLEARCHAT) |
| YouTube  | ✅ Yes         | ✅ Yes (user ban) | ⚠️ Partial | 30-60s  | HTTP polling (liveChatMessages API) |
| Kick     | ✅ Yes         | ⚠️ Unknown     | ⚠️ Unknown | <500ms  | Pusher WebSocket (ChatMessageDeletedEvent) |
| TikTok   | ❌ No          | ❌ No          | ❌ No      | N/A     | Unsupported by library |

**Legend:**
- ✅ Fully supported and tested
- ⚠️ Partial support or unknown (requires production validation)
- ❌ Not supported

## Architecture

### Data Flow

1. **Platform Detection:** Listener service captures deletion event from streaming platform
2. **Registry Lookup:** Platform message ID mapped to internal UUID via Message ID Registry
3. **Buffering:** Deletion events buffered for 60s to handle race conditions (deletion arrives before message)
4. **Normalization:** Platform-specific events normalized to unified schema
5. **Distribution:** Normalized events published to Redis Pub/Sub overlay channels
6. **Delivery:** API Gateway broadcasts to connected WebSocket clients
7. **UI Removal:** Frontend removes messages from overlay display

### Components

**Message ID Registry (Phase 1):**
- Redis hash mapping platform message IDs to internal UUIDs
- 1-hour TTL per channel (refreshed on message add)
- O(1) lookup performance
- Storage: ~50 bytes per message

**Deletion Buffer (Phase 1):**
- Handles race condition where deletion arrives before message
- Redis SET with 60-second TTL
- Automatic expiration by Redis

**Replay Buffer (Phase 3):**
- Enables reconnection resilience
- Redis sorted set with 60-second TTL
- Frontend requests missed deletions on reconnect
- Prevents message loss during disconnect

### Unified Event Schema

All platforms normalize to this schema:

```json
{
  "type": "chat_message",
  "data": {
    "event": {
      "type": "message_deletion",
      "metadata": {
        "deletion_type": "single|batch|clear",
        "target_uuid": "internal-message-uuid",
        "target_msg_id": "platform-message-id",
        "target_user_id": "user-id-for-batch",
        "target_username": "username-for-batch",
        "platform": "twitch|youtube|kick"
      }
    }
  }
}
```

## Deletion Types

### Single Message Deletion

Moderator deletes one specific message.

**Frontend behavior:** Remove message with matching `target_uuid` from display.

**Example:**
- Twitch: `/delete <message-id>` or clicking delete button
- YouTube: Clicking "Remove" on message in YouTube Studio live chat
- Kick: Clicking delete icon on message

### Batch Deletion (User Timeout/Ban)

Moderator times out or bans user, removing all their messages.

**Frontend behavior:** Remove all messages where `message.user.id === target_user_id`.

**Optimization:** React 18 automatic batching groups deletions into single render cycle.

**Example:**
- Twitch: `/timeout <username> <duration>` or `/ban <username>`
- YouTube: Banning user in YouTube Studio (all messages removed)
- Kick: Unknown (requires production validation)

### Clear Chat

Moderator clears entire chat history.

**Frontend behavior:** Remove all messages from display (`setMessages([])`).

**Example:**
- Twitch: `/clear` command
- YouTube: Not supported
- Kick: Unknown (requires production validation)

## Performance

### Load Testing Results

**Test scenario:** 1,000 messages sent, then batch deletion (user ban)

**Target:** <100ms render time for batch deletion

**Results:** [To be filled after load testing execution]

**React 18 Automatic Batching:**
- Single state update for batch deletions
- No manual optimization required (no `unstable_batchedUpdates` needed)
- Frontend filters messages by user ID (not message ID array iteration)

### Memory Usage

**Message ID Registry:**
- ~50 bytes per message
- 1-hour TTL (automatic cleanup)
- Example: 10,000 messages = ~500 KB

**Deletion Buffer:**
- ~100 bytes per buffered deletion
- 60-second TTL (race condition window)
- Minimal memory impact (<1 MB typical)

**Replay Buffer:**
- ~100 bytes per deletion event
- 60-second TTL (reconnection window)
- Per-overlay isolation
- Minimal memory impact (<1 MB per active overlay)

## Reconnection Handling

When WebSocket disconnects and reconnects:

1. Frontend tracks `last_seen` timestamp in localStorage
2. On reconnect, sends `replay_request` with timestamp
3. API Gateway queries replay buffer for missed deletions
4. Gateway sends `replay_response` with batch of events
5. Frontend applies deletions identically to real-time events

**Window:** 60 seconds (deletions older than 60s not replayed)

**Rationale:** Chat is ephemeral. >60s disconnect = acceptable message loss per requirements.

## Error Handling

### Registry Lookup Failure

**Scenario:** Deletion event references message ID not in registry

**Behavior:** Deletion buffered for 60 seconds. If message arrives, deletion applied. If not, buffer expires silently.

**Why:** Message may not have arrived yet (race condition), or message too old (>1 hour TTL expired).

### WebSocket Disconnect >60s

**Scenario:** Frontend disconnected for longer than replay buffer TTL

**Behavior:** Missed deletions not replayed. Overlay continues with messages that should have been deleted.

**Mitigation:** Reload page to fetch fresh state (existing messages only, deleted messages excluded from API response).

### Malformed Deletion Event

**Scenario:** Deletion event JSON invalid or missing required fields

**Behavior:** Event skipped, error logged. Other deletions continue processing.

**Why:** Resilient parsing prevents single bad event from blocking pipeline.

## Platform-Specific Notes

### Twitch

**Strengths:**
- Real-time IRC events (<500ms latency)
- All deletion types supported (single, batch, clear)
- Production-tested and stable

**Limitations:** None

### YouTube

**Strengths:**
- Supports single and batch deletions
- No additional API quota cost (uses existing liveChatMessages polling)

**Limitations:**
- 30-60s latency (polling-based)
- Clear chat not supported by YouTube API
- Requires increased quota (1,000,000 units/day) for high-traffic streams

### Kick

**Strengths:**
- Real-time Pusher WebSocket events (<500ms latency)
- Follows existing chat message pattern

**Limitations:**
- Unofficial API (no official documentation)
- Event structure validated via third-party libraries (MEDIUM confidence)
- Batch deletion and clear chat support unknown (requires production testing)

**Production validation required:** Event name and structure confirmed via logging.

### TikTok

**Limitation:** Deletion events not supported by unofficial TikTok Live library.

**Workaround:** None available. Messages persist on overlay even if deleted on TikTok.

**Future:** Monitor library updates for deletion event support.

## Configuration

No configuration required - deletion support enabled automatically for all platforms.

**Optional:** Disable deletion handling per overlay via `overlay_settings` table (future enhancement).

## Monitoring

**Metrics (Prometheus):**
- `deletions_buffered`: Count of out-of-order deletions buffered
- `buffered_deletions_applied`: Count of buffered deletions applied when message arrived
- `replay_requests`: Count of frontend replay requests
- `replay_events_sent`: Count of events replayed on reconnect

**Logs (structured):**
- Deletion events received from platforms
- Registry lookup results (hit/miss)
- Buffer operations (add/apply/expire)
- Replay requests and responses

**Alerts:**
- High buffered deletion rate (>10% of messages) = potential registry issue
- Replay buffer memory growth = TTL not expiring correctly
- Deletion event processing errors = platform API changes

## Testing

**Unit Tests:**
- Message ID Registry: 87.5% coverage
- Deletion Buffer: 95% coverage
- Replay Buffer: >85% coverage
- Message Processor normalization: 90% coverage

**Integration Tests:**
- End-to-end Twitch deletion flow (Phase 1)
- YouTube deletion detection (Phase 2)
- Kick deletion event handler (Phase 3)

**Load Tests:**
- 1,000 message batch deletion (Artillery)
- Frontend render time validation (<100ms target)

**Manual Testing:**
- Twitch: Moderator deletes message via IRC command
- YouTube: Moderator deletes message in YouTube Studio
- Kick: Moderator deletes message in Kick chat (requires live stream)

## Future Enhancements

- [ ] Per-overlay deletion handling toggle
- [ ] Deletion event history log (audit trail)
- [ ] Soft delete with "Message deleted by moderator" placeholder
- [ ] Animated fade-out transition (currently instant removal)
- [ ] TikTok deletion support (when library adds support)
- [ ] Increased replay buffer window (5 minutes instead of 60 seconds)

## References

- [Phase 1 Summary](../../.planning/phases/01-foundation-twitch/01-05-SUMMARY.md)
- [Phase 2 Summary](../../.planning/phases/02-youtube-integration/02-02-SUMMARY.md)
- [Phase 3 Research](../../.planning/phases/03-kick-integration-edge-cases/03-RESEARCH.md)
- [Message ID Registry Architecture](../../services/message-processor/registry/README.md)
- [API Gateway WebSocket Protocol](../../services/api-gateway/README.md)
