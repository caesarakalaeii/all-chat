# Architecture Research: Message Deletion Events

**Domain:** Streaming chat aggregation with deletion event handling
**Researched:** 2026-02-17
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Platform Listeners                       │
│  Twitch IRC  │  YouTube API  │  Kick WebSocket  │  TikTok   │
└──────────────┬──────────────┬──────────────┬────────────────┘
               │              │              │
               ▼              ▼              ▼
         ┌──────────────────────────────────────┐
         │  NEW: Message ID Tracking             │
         │  • Assign platform msg ID → UUID      │
         │  • Store in Redis (TTL 24h)          │
         └───────────────┬──────────────────────┘
                         │ publish raw messages + deletion events
                         ▼
                  ┌─────────────┐
                  │ Redis Stream│ (chat:raw)
                  │ XADD        │
                  └──────┬──────┘
                         │ XREADGROUP (consumer group)
                         ▼
              ┌──────────────────────┐
              │ Message Processor     │
              │ • Normalize format    │
              │ • Enrich emotes       │
              │ NEW: Handle deletions │
              └──────────┬────────────┘
                         │ publish enriched messages + deletion events
                         ▼
                  ┌─────────────┐
                  │ Redis Pub/Sub│ (overlay:*)
                  │ PUBLISH      │
                  └──────┬──────┘
                         │ SUBSCRIBE
                         ▼
              ┌──────────────────────┐
              │  API Gateway          │
              │  • WebSocket Hub      │
              │  NEW: Track client    │
              │         message IDs   │
              └──────────┬────────────┘
                         │ WebSocket broadcast
                         ▼
              ┌──────────────────────┐
              │  Overlay (Frontend)   │
              │  • React components   │
              │  NEW: Remove deleted  │
              │         messages      │
              └───────────────────────┘
```

### Component Responsibilities

| Component | Current Responsibility | New Deletion Responsibility |
|-----------|------------------------|----------------------------|
| **Platform Listeners** | Parse platform messages, publish to Redis Streams | Parse deletion events (CLEARMSG, retraction API responses), publish to same stream with event_type="deletion" |
| **Message ID Registry** (NEW) | N/A | Map platform message IDs → internal UUIDs (Redis hash with 24h TTL) |
| **Message Processor** | Normalize, enrich, route to overlays | Handle deletion events: lookup original message UUID, forward deletion to affected overlays |
| **Redis Streams** | Durable queue for raw messages | Also queue deletion events (same stream, different event_type) |
| **Redis Pub/Sub** | Low-latency broadcast of enriched messages | Also broadcast deletion events to overlay channels |
| **API Gateway** | WebSocket management, message broadcast | Track which messages sent to which clients, forward deletions |
| **Frontend Overlay** | Display messages with animations | Remove deleted messages from DOM, optional fade-out animation |

## Recommended Project Structure

### Schema Changes

**New Message ID Registry (Redis Hash)**:
```
Key: msg:ids:{platform}:{platform_msg_id}
Value: {
  "internal_id": "uuid-v4",
  "overlay_ids": ["overlay-uuid-1", "overlay-uuid-2"],
  "timestamp": "2026-02-17T10:00:00Z"
}
TTL: 86400 (24 hours)
```

**Extended RawChatMessage Model** (no changes needed, uses existing event_type field):
```go
type RawChatMessage struct {
    MessageID   string            `json:"message_id"`    // Internal UUID
    Platform    string            `json:"platform"`
    // ... existing fields ...

    // Event support (ALREADY EXISTS - reuse for deletions)
    EventType   string                 `json:"event_type,omitempty"`   // "chat" | "deletion"
    EventData   map[string]interface{} `json:"event_data,omitempty"`   // Deletion: {target_msg_id, deleted_by}
}
```

**Extended UnifiedChatMessage Model** (no changes needed, uses existing Event field):
```go
type UnifiedChatMessage struct {
    ID          string      `json:"id"`           // Internal UUID
    OverlayID   string      `json:"overlay_id"`
    // ... existing fields ...

    // Event support (ALREADY EXISTS - reuse for deletions)
    Event       *EventInfo  `json:"event,omitempty"`  // Type="deletion"
}
```

**New Deletion Event Format**:
```typescript
interface DeletionEvent {
  type: "deletion";
  message_id: string;           // Internal UUID of deleted message
  platform: string;             // "twitch" | "youtube" | "kick" | "tiktok"
  deleted_by: string;           // User ID of moderator (if available)
  timestamp: string;            // ISO 8601 deletion timestamp
}
```

### File Structure

```
services/
├── twitch-listener/
│   ├── handlers/
│   │   └── irc_handler.go            # Handle CLEARMSG/CLEARCHAT IRC messages
│   ├── registry/                      # NEW
│   │   └── message_id_registry.go    # Map Twitch msg-id → internal UUID
│   └── publisher/
│       └── redis.go                   # Publish deletion events to chat:raw
│
├── youtube-listener/
│   ├── handlers/
│   │   └── message_handler.go        # Detect messageDeletededEvent in poll responses
│   ├── registry/                      # NEW
│   │   └── message_id_registry.go    # Map YouTube messageId → internal UUID
│   └── publisher/
│       └── redis.go                   # Publish deletion events to chat:raw
│
├── kick-listener/
│   ├── handlers/
│   │   └── websocket_handler.go      # Handle ChatMessageDeletedEvent
│   ├── registry/                      # NEW
│   │   └── message_id_registry.go    # Map Kick message ID → internal UUID
│   └── publisher/
│       └── redis.go                   # Publish deletion events to chat:raw
│
├── message-processor/
│   ├── handlers/
│   │   └── deletion_handler.go       # NEW: Process deletion events
│   ├── normalizer/
│   │   ├── deletion_normalizer.go    # NEW: Normalize platform deletion formats
│   │   └── *_normalizer.go           # Updated: Track message IDs during normalization
│   └── publisher/
│       └── pubsub.go                  # Publish deletions to overlay:{id}
│
├── api-gateway/
│   ├── websocket/
│   │   ├── connection.go             # Track sent message IDs per connection
│   │   ├── deletion_handler.go       # NEW: Forward deletions to WebSocket clients
│   │   └── manager.go                # Updated: Handle deletion subscriptions
│   └── models/
│       └── ws_message.go             # Add deletion message type
│
└── overlay-frontend/
    ├── components/
    │   ├── ChatMessage.tsx           # Add deletion handler
    │   └── ChatOverlay.tsx           # Track messages by ID, remove on deletion
    └── hooks/
        └── useWebSocket.ts           # Handle deletion events from WebSocket
```

### Structure Rationale

- **registry/:** Separate package for message ID mapping (reusable across listeners)
- **handlers/deletion_handler.go:** Dedicated handlers for deletion-specific logic (single responsibility)
- **deletion_normalizer.go:** Platform-agnostic deletion format (follows existing normalizer pattern)
- **No database changes:** Message IDs stored in Redis only (ephemeral, 24h TTL matches message lifecycle)

## Architectural Patterns

### Pattern 1: Message ID Registry (Redis Hash)

**What:** Map platform-specific message IDs to internal UUIDs for deletion matching

**When to use:** When platform provides message ID in deletion event (Twitch CLEARMSG, YouTube retraction, Kick deletion)

**Trade-offs:**
- **Pros:** Fast O(1) lookup, automatic expiration (TTL), no database schema changes
- **Cons:** Lost on Redis restart (mitigated by 24h TTL matching message lifetime), memory overhead (~100 bytes per message)

**Example:**
```go
// services/twitch-listener/registry/message_id_registry.go
type MessageIDRegistry struct {
    redis *redis.Client
}

func (r *MessageIDRegistry) Register(ctx context.Context, platformID, internalID string, overlayIDs []string) error {
    key := fmt.Sprintf("msg:ids:twitch:%s", platformID)
    data := map[string]interface{}{
        "internal_id": internalID,
        "overlay_ids": overlayIDs,
        "timestamp": time.Now().Format(time.RFC3339),
    }
    pipe := r.redis.Pipeline()
    pipe.HSet(ctx, key, data)
    pipe.Expire(ctx, key, 24*time.Hour)
    _, err := pipe.Exec(ctx)
    return err
}

func (r *MessageIDRegistry) Lookup(ctx context.Context, platformID string) (string, []string, error) {
    key := fmt.Sprintf("msg:ids:twitch:%s", platformID)
    result, err := r.redis.HGetAll(ctx, key).Result()
    if err != nil {
        return "", nil, err
    }
    return result["internal_id"], parseOverlayIDs(result["overlay_ids"]), nil
}
```

### Pattern 2: Unified Event Stream (Same Redis Stream)

**What:** Publish both chat messages and deletion events to the same Redis Stream (chat:raw)

**When to use:** When events share similar processing pipeline and delivery requirements

**Trade-offs:**
- **Pros:** Single consumer group, simpler architecture, preserves message order
- **Cons:** Message Processor must handle multiple event types (mitigated by event_type field)

**Example:**
```go
// services/twitch-listener/handlers/irc_handler.go
func (h *IRCHandler) handleCLEARMSG(msg twitch.ClearMessage) {
    // Lookup original message UUID
    internalID, overlayIDs, err := h.registry.Lookup(ctx, msg.TargetMsgID)
    if err != nil {
        logger.Warn("Deletion for unknown message", zap.String("target_msg_id", msg.TargetMsgID))
        return
    }

    // Publish deletion event to Redis Stream
    deletionEvent := models.RawChatMessage{
        MessageID: uuid.New().String(),
        Platform:  "twitch",
        EventType: "deletion",
        EventData: map[string]interface{}{
            "target_internal_id": internalID,
            "target_platform_id": msg.TargetMsgID,
            "deleted_by": msg.Login,
            "overlay_ids": overlayIDs,
        },
        Timestamp: time.Now(),
    }
    h.publisher.Publish(ctx, deletionEvent)
}
```

### Pattern 3: Event-Driven Deletion Propagation

**What:** Forward deletion events through existing message pipeline (Streams → Processor → Pub/Sub → Gateway → Client)

**When to use:** When deletion latency requirements match message latency (<500ms P95)

**Trade-offs:**
- **Pros:** Reuses existing infrastructure, no new communication channels, consistent latency
- **Cons:** Deletions delayed by processing pipeline (acceptable for chat moderation use case)

**Example:**
```go
// services/message-processor/handlers/deletion_handler.go
func (h *DeletionHandler) Process(ctx context.Context, rawMsg models.RawChatMessage) error {
    targetID := rawMsg.EventData["target_internal_id"].(string)
    overlayIDs := rawMsg.EventData["overlay_ids"].([]string)

    // Create unified deletion event
    deletion := models.UnifiedChatMessage{
        ID:        rawMsg.MessageID,
        Platform:  rawMsg.Platform,
        Timestamp: rawMsg.Timestamp,
        Event: &models.EventInfo{
            Type: "deletion",
            Metadata: map[string]interface{}{
                "target_message_id": targetID,
                "deleted_by": rawMsg.EventData["deleted_by"],
            },
        },
    }

    // Publish to overlay-specific channels
    for _, overlayID := range overlayIDs {
        deletion.OverlayID = overlayID
        h.publisher.Publish(ctx, fmt.Sprintf("overlay:%s", overlayID), deletion)
    }
    return nil
}
```

## Data Flow

### Request Flow: Message Creation → Deletion

```
1. Platform Message Arrives
   Twitch IRC: "PRIVMSG #channel :Hello world" (msg-id=abc-123-def)
   ↓
2. Listener: Register Message ID
   Redis: SET msg:ids:twitch:abc-123-def {internal_id: uuid-v4, overlays: [overlay-1]} EX 86400
   ↓
3. Listener → Redis Streams
   XADD chat:raw {message_id: uuid-v4, platform: "twitch", text: "Hello world", event_type: "chat"}
   ↓
4. Message Processor: Normalize + Enrich
   (existing pipeline, no changes)
   ↓
5. Pub/Sub → Gateway → Client
   WebSocket: {id: uuid-v4, text: "Hello world", ...}
   Frontend: Display message with ID=uuid-v4
   ↓
   ────────────────────────────────────────────────────────────────
6. Platform Deletion Event
   Twitch IRC: "CLEARMSG #channel :Hello world" (target-msg-id=abc-123-def)
   ↓
7. Listener: Lookup Original Message
   Redis: GET msg:ids:twitch:abc-123-def → {internal_id: uuid-v4, overlays: [overlay-1]}
   ↓
8. Listener → Redis Streams
   XADD chat:raw {message_id: deletion-uuid, platform: "twitch", event_type: "deletion",
                  event_data: {target_internal_id: uuid-v4, overlay_ids: [overlay-1]}}
   ↓
9. Message Processor: Handle Deletion
   Normalize deletion event, no enrichment needed
   ↓
10. Pub/Sub → Gateway → Client
    WebSocket: {event: {type: "deletion", metadata: {target_message_id: uuid-v4}}}
    Frontend: Remove message ID=uuid-v4 from DOM
```

### State Management

```
Message ID Registry (Redis Hash)
    ↓ (register on message create)
Platform Listener ←→ Redis Hash ← (lookup on deletion event)
    ↓ (publish deletion to stream)
Redis Streams (chat:raw)
    ↓ (process deletion)
Message Processor ←→ Deletion Handler → Pub/Sub
    ↓ (forward to clients)
API Gateway ←→ WebSocket Manager → Frontend
    ↓ (remove from DOM)
React State (message list) - message removed
```

### Key Data Flows

1. **Message ID Registration Flow:** Listener receives platform message → generates internal UUID → stores mapping in Redis (platform_id → internal_id + overlay_ids) → publishes to stream
2. **Deletion Event Flow:** Listener receives deletion event → looks up internal UUID from platform message ID → publishes deletion event with internal UUID → Message Processor routes to affected overlays → Frontend removes message

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-1k messages/s | Single Redis instance, registry hash stores ~86K messages (24h * 1msg/s), ~8MB memory overhead |
| 1k-10k messages/s | Redis Cluster, shard registry by platform+channel (consistent hashing), ~80MB memory overhead |
| 10k+ messages/s | Separate Redis instance for message ID registry (dedicated memory allocation), consider shorter TTL (12h instead of 24h) |

### Scaling Priorities

1. **First bottleneck:** Message ID registry memory (at 10k msg/s, 24h retention = 864M entries × 100 bytes = ~86GB)
   - **Fix:** Reduce TTL to 6 hours (messages rarely deleted after 6h), drops to ~21GB
2. **Second bottleneck:** Redis Streams MAXLEN (deletion events add to stream length)
   - **Fix:** Increase MAXLEN from 50K to 100K messages (accommodates 10% deletion rate)

## Anti-Patterns

### Anti-Pattern 1: Database Storage for Message IDs

**What people might do:** Store message ID mappings in PostgreSQL instead of Redis

**Why it's wrong:**
- **Performance:** Database query adds 5-20ms per deletion (vs <1ms for Redis hash lookup)
- **Unnecessary durability:** Message IDs only needed for 24h (Redis TTL handles cleanup automatically)
- **Schema bloat:** Would require new table, indices, and migrations for ephemeral data

**Do this instead:** Use Redis hash with 24h TTL (matches message lifecycle)

### Anti-Pattern 2: Separate Deletion Stream

**What people might do:** Create dedicated Redis Stream for deletion events (e.g., chat:deletions)

**Why it's wrong:**
- **Ordering issues:** Deletion event might process before original message (race condition)
- **Complexity:** Message Processor must consume from two streams, manage ordering
- **Fan-out duplication:** Both streams need consumer groups, monitoring, backpressure handling

**Do this instead:** Use same Redis Stream (chat:raw) with event_type field (preserves order)

### Anti-Pattern 3: Synchronous Deletion Confirmation

**What people might do:** Wait for deletion to propagate to all clients before ACK'ing deletion event

**Why it's wrong:**
- **Latency spike:** Blocks message processing pipeline waiting for WebSocket delivery
- **Unnecessary guarantee:** Chat deletion is best-effort (acceptable if client offline/disconnected)
- **Complexity:** Requires tracking per-client delivery state, timeout handling

**Do this instead:** Fire-and-forget deletion (same model as existing message delivery)

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Twitch IRC | Parse CLEARMSG/CLEARCHAT | target-msg-id maps to Twitch-provided msg-id tag on PRIVMSG |
| YouTube API | Poll for messageDeletededEvent | YouTube provides deletedMessageId in response, poll-based (no WebSocket) |
| Kick Pusher | Subscribe to ChatMessageDeletedEvent | Real-time WebSocket event with message_id field |
| TikTok Live | Library provides deletion callback | Unofficial library, API unstable (low confidence) |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Listener ↔ Redis | HSET (register), XADD (publish) | Message ID registry + event stream |
| Processor ↔ Redis | XREADGROUP (consume), PUBLISH (forward) | Deletion handler reuses existing pipeline |
| Gateway ↔ Frontend | WebSocket JSON | New message type: {event: {type: "deletion"}} |

## Platform-Specific Deletion Formats

### Twitch IRC CLEARMSG

**Example:**
```
@login=baduser;room-id=12345;target-msg-id=abc-123-def;tmi-sent-ts=1642720582342 :tmi.twitch.tv CLEARMSG #channel :offensive message
```

**Fields:**
- `target-msg-id`: Message ID to delete (maps to `id` tag from PRIVMSG)
- `login`: Username of deleted message author
- `room-id`: Channel ID

**Confidence:** HIGH - Official Twitch IRC documentation

### Twitch IRC CLEARCHAT

**Example:**
```
@ban-duration=600;room-id=12345;target-user-id=67890;tmi-sent-ts=1642720582342 :tmi.twitch.tv CLEARCHAT #channel :baduser
```

**Use case:** Deletes ALL messages from user (timeout/ban)

**Implementation:** Loop through message ID registry, delete all entries where user_id=67890

**Confidence:** HIGH - Official Twitch IRC documentation

### YouTube API Message Retraction

**Example Response:**
```json
{
  "kind": "youtube#liveChatMessageListResponse",
  "items": [
    {
      "kind": "youtube#liveChatMessage",
      "id": "msg-456",
      "snippet": {
        "type": "messageDeletedEvent",
        "messageDeletedDetails": {
          "deletedMessageId": "msg-123"
        }
      }
    }
  ]
}
```

**Fields:**
- `deletedMessageId`: Message ID to delete
- No information about who deleted (API limitation)

**Confidence:** MEDIUM - Official YouTube API docs, but sparse examples

### Kick Pusher WebSocket

**Example:**
```json
{
  "event": "App\\Events\\ChatMessageDeletedEvent",
  "data": {
    "message": {
      "id": "msg-789",
      "chatroom_id": 12345
    }
  }
}
```

**Fields:**
- `id`: Message ID to delete
- `chatroom_id`: Channel ID
- No information about moderator (unofficial API)

**Confidence:** LOW - Reverse-engineered, no official documentation

### TikTok Live

**Status:** Library provides `onMessageDeleted` callback with message ID

**Confidence:** LOW - Unofficial library (zerodytrash/TikTok-Live-Connector), API unstable

## Implementation Build Order

### Phase 1: Foundation (Twitch Only)
**Dependencies:** None (standalone feature)

**Components:**
1. Message ID registry (Redis hash operations)
2. Twitch Listener: Handle CLEARMSG, publish deletion events
3. Message Processor: Deletion handler
4. API Gateway: Forward deletions to WebSocket
5. Frontend: Remove messages from DOM

**Validation:** Moderator deletes message in Twitch chat → message removed from overlay within 500ms

**Estimated Effort:** 3-5 days

### Phase 2: YouTube Integration
**Dependencies:** Phase 1 (reuses registry + processor)

**Components:**
1. YouTube Listener: Parse messageDeletededEvent
2. Message ID registry: Add YouTube platform support
3. Testing: Deletion during live stream

**Validation:** Moderator deletes message in YouTube chat → message removed from overlay

**Estimated Effort:** 2-3 days

### Phase 3: Kick + TikTok Integration
**Dependencies:** Phase 1 (reuses registry + processor)

**Components:**
1. Kick Listener: Handle ChatMessageDeletedEvent
2. TikTok Listener: Subscribe to onMessageDeleted (if stable)
3. Message ID registry: Add Kick/TikTok platform support

**Validation:** Deletions work across all 4 platforms

**Estimated Effort:** 2-3 days

### Phase 4: Advanced Features (Optional)
**Dependencies:** Phases 1-3

**Components:**
1. CLEARCHAT support (delete all messages from user)
2. Deletion animations (fade-out instead of instant removal)
3. Deletion history (audit log in database)
4. Moderator attribution (display who deleted message)

**Estimated Effort:** 3-5 days

## Sources

### Official Documentation
- [Twitch IRC Concepts](https://dev.twitch.tv/docs/chat/irc/) - CLEARMSG/CLEARCHAT message formats
- [Twitch IRC Tags](https://dev.twitch.tv/docs/irc/tags/) - target-msg-id tag specification
- [YouTube Live Chat API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages) - Message deletion endpoints
- [YouTube Streaming Live Chat](https://developers.google.com/youtube/v3/live/streaming-live-chat) - Polling for deletion events

### Community Documentation
- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) - Go library supporting CLEARMSG parsing
- [Kick API GitHub Issues](https://github.com/KickEngineering/KickDevDocs/issues/20) - Websocket-based events discussion
- [TikTok-Live-Connector](https://github.com/zerodytrash/TikTok-Live-Connector) - Node.js library with deletion support

### Existing Architecture
- [ADR-0002: Redis Streams + Pub/Sub](../docs/adr/0002-redis-streams-pubsub.md) - Explains hybrid messaging architecture
- [01-DATA-FLOW.md](../docs/architecture/01-DATA-FLOW.md) - Complete message pipeline documentation

---
*Architecture research for: Message deletion events in streaming chat aggregation*
*Researched: 2026-02-17*
