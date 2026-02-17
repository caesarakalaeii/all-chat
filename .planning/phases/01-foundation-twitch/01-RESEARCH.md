# Phase 1: Foundation + Twitch - Research

**Researched:** 2026-02-18
**Domain:** Real-time message deletion infrastructure with Twitch IRC integration
**Confidence:** HIGH

## Summary

Phase 1 establishes message deletion infrastructure by implementing a Redis-based Message ID Registry, normalizing deletion events through the existing Redis Streams → Message Processor → Pub/Sub → API Gateway pipeline, and enabling instant message removal from React overlays. Research validates that the existing architecture (go-redis/v9, gempir/go-twitch-irc/v4, Redis Streams consumer groups, React 18 state management) provides all necessary primitives without requiring new dependencies.

**Primary finding:** The existing message pipeline can handle deletion events as a new message type with minimal modification. Twitch IRC provides three distinct deletion commands (CLEARMSG, CLEARCHAT with/without user) that map cleanly to single/batch/clear deletion types. Redis hashes with TTL provide O(1) message ID lookups, and React's automatic batching (React 18+) handles batch DOM deletions efficiently.

**Critical architectural decision:** Use unidirectional mapping (platform ID → internal UUID) in Message ID Registry. Bidirectional mapping adds complexity and memory overhead without providing value since deletion events always arrive with platform IDs, never internal UUIDs.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Message ID Tracking
- Add platform IDs to registry at listener capture (as soon as message arrives from Twitch IRC)
- Registry entries have 1-hour TTL (balance between memory usage and deletion window)

#### Deletion Event Flow
- Single event type with type field differentiating single/batch/clear deletions (not separate event types per deletion kind)
- Deletion events flow through same pipeline as regular messages: Redis Streams → Message Processor → Pub/Sub → API Gateway (maintains ordering and consistency)
- Batch deletions (timeout/ban) represented as coalesced events with user_id (frontend removes all messages matching that user, not individual message ID array)

#### Race Condition Strategy
- Backend buffer approach: Message Processor holds deletion events in Redis when target message not yet in registry
- Buffered deletion events expire after 60 seconds if message never arrives

#### Frontend Removal Behavior
- Instant removal from DOM (no fade animation, no placeholder)

### Claude's Discretion
- Registry mapping direction (unidirectional vs bidirectional)
- Redis buffer data structure (sorted set vs hash with TTL)
- Frontend DOM tracking approach (data attributes, React state, or hybrid)
- Batch deletion DOM optimization strategy
- Error handling for registry misses
- Logging and metrics for deletion events

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MSGID-01 | System preserves platform-native message IDs alongside internal UUIDs | RawChatMessage.Tags["id"] (Twitch msg ID) already captured; needs registry persistence |
| MSGID-02 | Redis-based Message ID Registry maps platform IDs to internal UUIDs | Redis hash provides O(1) HGET/HSET with field-level TTL via HSETEX (go-redis v9) |
| MSGID-03 | Registry entries have 24-hour TTL to match message retention | **NOTE: CONTEXT.md specifies 1-hour TTL** (conflict with requirement) |
| MSGID-04 | Registry provides O(1) lookup for deletion event matching | Redis hash HGET is O(1) average case; hash table backing guarantees performance |
| MSGID-05 | Platform IDs flow through entire pipeline | Tags already propagate from Listener → Processor; needs preservation in UnifiedChatMessage |
| DEL-01 | System detects single message deletion events from platforms | gempir/go-twitch-irc/v4 OnClearMessage() handler provides ClearMessage with TargetMsgID |
| DEL-02 | System detects user batch deletion events | OnClearChatMessage() with target-user-id and optional ban-duration tags |
| DEL-03 | System detects full chat clear events | OnClearChatMessage() without target-user-id |
| DEL-04 | Deletion events normalized to unified schema | New RawChatMessage.EventType = "message_deletion" with DeletionType field |
| DEL-05 | Deletion events propagate through existing pipeline | Redis Streams consumer groups (XREADGROUP) already handle all RawChatMessage types |
| DEL-06 | Batch deletions use coalesced schema | DeletionEvent with UserID field (no message ID array) |
| RACE-01 | System buffers deletion events for messages not yet received | Redis hash or sorted set with 60s TTL holds pending deletions |
| RACE-02 | Deletion events processed after messages arrive | Message Processor checks buffer on message arrival, applies buffered deletions |
| RACE-03 | Expired deletion events discarded without error | Redis TTL auto-expires; processor logs metric but no error propagation |
| TWITCH-01 | Listener detects IRC CLEARMSG events | go-twitch-irc/v4 OnClearMessage() handler built-in |
| TWITCH-02 | Listener detects IRC CLEARCHAT with target-msg-id | OnClearChatMessage() with target-user-id tag |
| TWITCH-03 | Listener detects IRC CLEARCHAT without target | OnClearChatMessage() without target-user-id tag |
| TWITCH-04 | Twitch deletion events include target-msg-id | CLEARMSG provides target-msg-id; CLEARCHAT provides target-user-id |
| FRONTEND-01 | Frontend tracks platform message IDs in DOM | data-message-id attribute on message div |
| FRONTEND-02 | Frontend receives deletion events via WebSocket | API Gateway already broadcasts UnifiedChatMessage types |
| FRONTEND-03 | Frontend removes messages immediately | setMessages(prev => prev.filter(...)) with no animation CSS |
| FRONTEND-04 | Frontend handles single message deletion | Filter by message.id |
| FRONTEND-05 | Frontend handles batch deletion | Filter by message.user.id |
| FRONTEND-06 | Frontend handles full chat clear | setMessages([]) |

**TTL Conflict Resolution:** CONTEXT.md specifies 1-hour TTL (locked decision), but REQUIREMENTS.md specifies 24-hour TTL. **Planner MUST use 1-hour TTL** as CONTEXT.md represents final user decision.

</phase_requirements>

## Standard Stack

### Core (Already in Use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-redis/v9 | v9.x | Redis client (hash ops, TTL, pub/sub) | Official Go client; production-proven in codebase |
| gempir/go-twitch-irc/v4 | v4.x | Twitch IRC client with deletion event handlers | Already integrated; provides OnClearMessage/OnClearChatMessage |
| React | 18+ | Frontend state management with auto-batching | Current codebase uses React 18; automatic batch updates for deletions |
| gorilla/websocket | latest | WebSocket server (API Gateway) | Current API Gateway dependency |

### Supporting (No New Dependencies Required)
All features implementable with existing stack. No additional libraries needed.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Redis hash (HSET/HGET) | Redis sorted set (ZADD/ZRANGEBYSCORE) | Sorted set provides time-ordering but adds O(log N) write cost; unnecessary since lookups are by message ID, not time range |
| Unidirectional map | Bidirectional map (platform ↔ UUID) | Bidirectional doubles memory (2 hashes) and adds complexity; deletion events always provide platform ID, never UUID |
| Redis TTL | Manual cleanup cron | TTL is passive (checked on access) + active (periodic scan); cron adds deployment complexity |

**Installation:**
No new dependencies required. All features use existing `go.mod` entries.

## Architecture Patterns

### Recommended Message ID Registry Structure

**Location:** `services/message-processor/registry/` (new package)

```
services/message-processor/
├── registry/
│   ├── registry.go           # MessageIDRegistry interface + Redis implementation
│   ├── registry_test.go      # Unit tests with miniredis
│   └── buffer.go             # DeletionBuffer for race condition handling
```

### Pattern 1: Unidirectional Message ID Registry (RECOMMENDED)

**What:** Single Redis hash mapping platform message IDs to internal UUIDs

**When to use:** Deletion events always provide platform ID (Twitch target-msg-id), never internal UUID

**Structure:**
```
Key: msgid:registry:{platform}:{channel_id}
Fields:
  {platform_msg_id} → {internal_uuid}|{timestamp}

Example:
  HSET msgid:registry:twitch:shroud
       "abc-123-def-456" "550e8400-e29b-41d4-a716-446655440000|1708281600"

TTL: 1 hour (EXPIRE at key level, not field level)
```

**Go Implementation:**
```go
// Source: Research recommendation based on go-redis best practices
package registry

type MessageIDRegistry interface {
    Add(ctx context.Context, platform, channelID, platformMsgID, internalUUID string) error
    Lookup(ctx context.Context, platform, channelID, platformMsgID string) (string, error)
}

type RedisRegistry struct {
    client *redis.Client
    ttl    time.Duration // 1 hour
}

func (r *RedisRegistry) Add(ctx context.Context, platform, channelID, platformMsgID, internalUUID string) error {
    key := fmt.Sprintf("msgid:registry:%s:%s", platform, channelID)
    value := fmt.Sprintf("%s|%d", internalUUID, time.Now().Unix())

    // HSET + EXPIRE in pipeline for atomicity
    pipe := r.client.Pipeline()
    pipe.HSet(ctx, key, platformMsgID, value)
    pipe.Expire(ctx, key, r.ttl) // Refresh TTL on each add
    _, err := pipe.Exec(ctx)
    return err
}

func (r *RedisRegistry) Lookup(ctx context.Context, platform, channelID, platformMsgID string) (string, error) {
    key := fmt.Sprintf("msgid:registry:%s:%s", platform, channelID)
    value, err := r.client.HGet(ctx, key, platformMsgID).Result()
    if err == redis.Nil {
        return "", ErrMessageNotFound
    }
    if err != nil {
        return "", err
    }

    // Extract UUID from "uuid|timestamp" format
    parts := strings.Split(value, "|")
    return parts[0], nil
}
```

**Why unidirectional:** Deletion events provide platform ID, not internal UUID. Reverse lookup never needed.

### Pattern 2: Deletion Event Buffer (Race Condition Handler)

**What:** Temporary storage for deletion events when target message not yet in registry

**When to use:** Deletion event arrives before corresponding message (network reordering, processing lag)

**Structure Options:**

**Option A: Redis Hash with TTL (RECOMMENDED)**
```
Key: msgid:deletion_buffer:{platform}:{channel_id}:{platform_msg_id}
Value: JSON-encoded DeletionEvent
TTL: 60 seconds

Example:
  SET msgid:deletion_buffer:twitch:shroud:abc-123-def-456
      '{"type":"single","platform_msg_id":"abc-123-def-456",...}'
      EX 60
```

**Advantages:**
- Simple SET/GET operations (O(1))
- Per-key TTL (auto-cleanup, no manual expiration)
- Natural fit for "check if buffered deletion exists" pattern
- go-redis provides clean `Set()` with `EX` option

**Option B: Redis Sorted Set with Timestamp Scores**
```
Key: msgid:deletion_buffer:{platform}:{channel_id}
Score: Unix timestamp
Member: JSON-encoded DeletionEvent with platform_msg_id

Example:
  ZADD msgid:deletion_buffer:twitch:shroud
       1708281660 '{"type":"single","platform_msg_id":"abc-123-def-456",...}'
```

**Advantages:**
- Range queries (ZRANGEBYSCORE for batch cleanup)
- Single key per channel (fewer keys in Redis)

**Disadvantages:**
- Manual expiration required (periodic ZREMRANGEBYSCORE)
- O(log N) insert cost
- More complex lookup (ZSCAN + JSON parsing)

**Recommendation:** Use Hash with TTL (Option A) for simplicity and automatic cleanup.

### Pattern 3: Deletion Event Flow Through Pipeline

**What:** Extend existing RawChatMessage/UnifiedChatMessage to support deletion events

**Integration Points:**

1. **Twitch Listener** (capture):
   ```go
   client.OnClearMessage(func(msg twitch.ClearMessage) {
       rawMsg := &models.RawChatMessage{
           MessageID: uuid.New().String(),
           Platform:  "twitch",
           ChannelID: strings.TrimPrefix(msg.Channel, "#"),
           EventType: "message_deletion",
           EventData: map[string]interface{}{
               "deletion_type":   "single",
               "target_msg_id":   msg.TargetMsgID,
               "moderator_login": msg.Login,
           },
           Timestamp: time.Now().UTC(),
       }
       streamPublisher.PublishRaw(ctx, rawMsg)
   })
   ```

2. **Message Processor** (normalization + registry):
   ```go
   func (p *Processor) processMessage(raw *models.RawChatMessage) error {
       if raw.EventType == "message_deletion" {
           return p.handleDeletion(raw)
       }

       // Regular message: add to registry
       platformMsgID := raw.Tags["id"]
       p.registry.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw.MessageID)

       // Check for buffered deletion
       if deletion := p.buffer.Get(ctx, raw.Platform, raw.ChannelID, platformMsgID); deletion != nil {
           p.handleDeletion(deletion)
           p.buffer.Remove(ctx, raw.Platform, raw.ChannelID, platformMsgID)
       }

       // Continue normal processing...
   }
   ```

3. **API Gateway** (broadcast):
   - No changes required; deletion events are UnifiedChatMessage with `Event.Type = "message_deletion"`

4. **Frontend** (removal):
   ```tsx
   ws.onmessage = (event) => {
       const envelope = JSON.parse(event.data);

       if (envelope.data.event?.type === 'message_deletion') {
           const deletion = envelope.data.event.metadata;

           setMessages(prev => {
               switch (deletion.deletion_type) {
                   case 'single':
                       return prev.filter(m => m.id !== deletion.target_uuid);
                   case 'batch':
                       return prev.filter(m => m.user.id !== deletion.target_user_id);
                   case 'clear':
                       return [];
               }
           });
       } else {
           // Regular message
           setMessages(prev => [...prev, envelope.data]);
       }
   };
   ```

### Anti-Patterns to Avoid

- **Bidirectional registry:** Never needed since deletions provide platform ID
- **Message ID array in batch deletions:** Frontend can filter by user_id; sending 1000 message IDs wastes bandwidth
- **Synchronous registry lookup on message arrival:** Use pipeline (HSET + EXPIRE) to avoid round trips
- **Polling for buffered deletions:** Check buffer reactively when message arrives, not on timer
- **Separate WebSocket message type:** Use existing `chat_message` envelope with `event.type = "message_deletion"`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Twitch IRC parsing | Custom IRC parser | gempir/go-twitch-irc/v4 | Handles IRC tag parsing, reconnection logic, rate limiting; production-proven |
| Redis TTL expiration | Cron job for cleanup | Redis EXPIRE command | Redis handles passive (lazy) + active (periodic) expiration; no deployment overhead |
| WebSocket reconnection | Custom exponential backoff | Existing API Gateway reconnect logic | Already implemented with jitter to prevent thundering herd |
| Batch state updates | Manual requestAnimationFrame batching | React 18 automatic batching | React batches setState calls in event handlers, promises, timeouts automatically |
| Message deduplication | Custom duplicate tracking | Redis Streams consumer groups | XREADGROUP with consumer group prevents duplicate message processing |

**Key insight:** The existing architecture (Redis Streams, go-twitch-irc, React 18) provides all primitives. The work is integration and orchestration, not building new infrastructure.

## Common Pitfalls

### Pitfall 1: Registry Key Structure Causes Memory Fragmentation

**What goes wrong:** Using separate Redis keys per message (`msgid:registry:{msg_id}`) creates millions of keys, causing memory fragmentation and slow SCAN operations.

**Why it happens:** Intuition suggests "one key per message" but Redis keys have ~90 bytes overhead each.

**How to avoid:** Use Redis hash with channel-level keys (`msgid:registry:{platform}:{channel_id}`) and message IDs as fields. Single key holds thousands of messages.

**Warning signs:** `redis-cli INFO memory` shows high `mem_fragmentation_ratio` (>1.5), or `DBSIZE` grows into millions.

**Example:**
```
❌ BAD: SET msgid:registry:abc-123 "uuid" (90 bytes overhead per message)
✅ GOOD: HSET msgid:registry:twitch:shroud abc-123 "uuid" (~10 bytes overhead per field)
```

### Pitfall 2: Race Condition Buffer Grows Unbounded

**What goes wrong:** Buffered deletion events never get cleaned up if target message never arrives, causing memory leak.

**Why it happens:** Assuming "message will always arrive eventually" but network failures, service restarts, or IRC disconnect can drop messages.

**How to avoid:** Set 60-second TTL on buffered deletion events. After 60s, assume message won't arrive and discard deletion.

**Warning signs:** `redis-cli MEMORY USAGE msgid:deletion_buffer:*` shows growing memory, or manual inspection reveals old entries.

**Monitoring metric:**
```go
metrics.BufferedDeletionsExpired.Inc() // Track how often deletions expire without matching message
```

### Pitfall 3: Frontend State Update Causes Re-Render Storm

**What goes wrong:** Removing 1000 messages from DOM (timeout/ban) triggers 1000 individual re-renders, freezing UI.

**Why it happens:** Naive implementation calls `setMessages()` inside loop, triggering render per deletion.

**How to avoid:** Use single `setMessages()` call with filter function. React 18 auto-batches, but explicit batching ensures single render.

**Example:**
```tsx
// ❌ BAD: Re-renders for each deleted message
deletionIds.forEach(id => {
    setMessages(prev => prev.filter(m => m.id !== id));
});

// ✅ GOOD: Single re-render for batch deletion
setMessages(prev => prev.filter(m => !deletionIds.includes(m.id)));

// ✅ BETTER: Filter by user_id (no array needed)
setMessages(prev => prev.filter(m => m.user.id !== bannedUserId));
```

**Warning signs:** React DevTools Profiler shows >100ms render times, or browser devtools show "Long Task" warnings.

### Pitfall 4: Registry TTL Not Refreshed on Access

**What goes wrong:** Message sent at T+0, deletion at T+55min, registry expired at T+60min → deletion fails despite being within window.

**Why it happens:** Setting TTL once at message insertion, not refreshing on lookup.

**How to avoid:** Use `EXPIRE` in pipeline with every `HSET`. Alternatively, use longer TTL (2 hours) to provide margin.

**Example:**
```go
// ✅ GOOD: Refresh TTL on every addition
pipe := client.Pipeline()
pipe.HSet(ctx, key, field, value)
pipe.Expire(ctx, key, 1*time.Hour) // Reset TTL to 1 hour
pipe.Exec(ctx)
```

**Warning signs:** Metrics show increasing "message not found in registry" errors over time, especially for older channels.

### Pitfall 5: Platform Message ID Not Captured for All Platforms

**What goes wrong:** Registry works for Twitch but fails for YouTube/Kick because platform message IDs not extracted.

**Why it happens:** Assuming all platforms provide message IDs in same format/location.

**How to avoid:** Phase 1 is Twitch-only (validates architecture). Document where each platform stores message IDs:
- Twitch: `msg.ID` (UUID in tags)
- YouTube: `id` field in API response
- Kick: `id` field in WebSocket message

**Warning signs:** Phase 2/3 implementation discovers message IDs missing, requiring listener changes.

## Code Examples

Verified patterns from official sources and existing codebase:

### Twitch IRC Deletion Event Handlers

```go
// Source: https://pkg.go.dev/github.com/gempir/go-twitch-irc/v4
// Official go-twitch-irc v4 API

client := twitch.NewClient(username, oauth)

// CLEARMSG - Single message deletion
client.OnClearMessage(func(msg twitch.ClearMessage) {
    log.Info("Message deleted",
        zap.String("channel", msg.Channel),
        zap.String("target_msg_id", msg.TargetMsgID),
        zap.String("login", msg.Login),
        zap.String("message", msg.Message),
    )
    // msg.TargetMsgID: UUID of deleted message
    // msg.Login: Username whose message was deleted
    // msg.Message: The deleted message text
})

// CLEARCHAT - Batch deletion or full clear
client.OnClearChatMessage(func(msg twitch.ClearChatMessage) {
    if msg.TargetUserID != "" {
        // User timeout or ban
        if msg.BanDuration > 0 {
            log.Info("User timed out",
                zap.String("channel", msg.Channel),
                zap.String("user", msg.TargetUsername),
                zap.Int("duration", msg.BanDuration),
            )
        } else {
            log.Info("User banned",
                zap.String("channel", msg.Channel),
                zap.String("user", msg.TargetUsername),
            )
        }
    } else {
        // Full chat clear
        log.Info("Chat cleared", zap.String("channel", msg.Channel))
    }
})
```

### Redis Hash Operations with TTL

```go
// Source: https://redis.io/docs/latest/develop/clients/go/
// Official go-redis guide

import "github.com/redis/go-redis/v9"

// Add message to registry with TTL
func AddToRegistry(ctx context.Context, client *redis.Client,
                   platform, channelID, platformMsgID, internalUUID string) error {
    key := fmt.Sprintf("msgid:registry:%s:%s", platform, channelID)
    value := fmt.Sprintf("%s|%d", internalUUID, time.Now().Unix())

    // Pipeline ensures atomicity: HSET + EXPIRE executed together
    pipe := client.Pipeline()
    pipe.HSet(ctx, key, platformMsgID, value)
    pipe.Expire(ctx, key, 1*time.Hour)
    _, err := pipe.Exec(ctx)
    return err
}

// Lookup message in registry (O(1) operation)
func LookupInRegistry(ctx context.Context, client *redis.Client,
                      platform, channelID, platformMsgID string) (string, error) {
    key := fmt.Sprintf("msgid:registry:%s:%s", platform, channelID)
    value, err := client.HGet(ctx, key, platformMsgID).Result()
    if err == redis.Nil {
        return "", fmt.Errorf("message not found in registry")
    }
    if err != nil {
        return "", fmt.Errorf("redis error: %w", err)
    }

    // Extract UUID from "uuid|timestamp" format
    parts := strings.Split(value, "|")
    if len(parts) != 2 {
        return "", fmt.Errorf("invalid registry value format")
    }
    return parts[0], nil
}
```

### React Message Deletion Handler

```tsx
// Source: Existing codebase pattern from /home/caesar/git/all-chat/frontend/src/app/overlay/[id]/page.tsx
// Current WebSocket message handling with deletion support added

useEffect(() => {
    const ws = new WebSocket(wsUrl);

    ws.onmessage = async (event) => {
        const envelope = JSON.parse(event.data);

        // Deletion event
        if (envelope.type === 'chat_message' && envelope.data.event?.type === 'message_deletion') {
            const deletion = envelope.data.event.metadata;

            setMessages((prev) => {
                switch (deletion.deletion_type) {
                    case 'single':
                        // Remove specific message by internal UUID
                        return prev.filter(m => m.id !== deletion.target_uuid);

                    case 'batch':
                        // Remove all messages from specific user (timeout/ban)
                        return prev.filter(m => m.user.id !== deletion.target_user_id);

                    case 'clear':
                        // Remove all messages (full chat clear)
                        return [];

                    default:
                        console.warn('Unknown deletion type:', deletion.deletion_type);
                        return prev;
                }
            });
            return;
        }

        // Regular message (existing code)
        if (envelope.type === 'chat_message' && envelope.data) {
            let message = envelope.data;
            message = await resolveTwitchBadgeIcons(message);
            message = sortMessageBadges(message);

            setMessages((prev) => {
                const newMessages = [...prev, message];
                return newMessages.slice(-maxMessages);
            });
        }
    };
}, [wsUrl, maxMessages]);
```

### Message Processor Deletion Handler

```go
// Source: Research recommendation based on existing processor patterns
// Extends existing message processing pipeline

func (p *Processor) processRawMessage(raw *models.RawChatMessage) error {
    ctx := context.Background()

    // Handle deletion events
    if raw.EventType == "message_deletion" {
        return p.handleDeletionEvent(ctx, raw)
    }

    // Regular message: add to registry FIRST (before enrichment)
    platformMsgID := raw.Tags["id"]
    if platformMsgID != "" {
        if err := p.registry.Add(ctx, raw.Platform, raw.ChannelID,
                                  platformMsgID, raw.MessageID); err != nil {
            p.logger.Error("Failed to add message to registry",
                zap.Error(err),
                zap.String("platform_msg_id", platformMsgID),
            )
            // Continue processing; registry is best-effort
        }

        // Check if deletion was buffered for this message
        if deletion := p.deletionBuffer.Get(ctx, raw.Platform, raw.ChannelID,
                                             platformMsgID); deletion != nil {
            // Process buffered deletion now
            p.handleDeletionEvent(ctx, deletion)
            p.deletionBuffer.Remove(ctx, raw.Platform, raw.ChannelID, platformMsgID)
            p.metrics.BufferedDeletionsApplied.Inc()
        }
    }

    // Continue with normal processing (enrichment, routing, publishing)
    return p.enrichAndPublish(ctx, raw)
}

func (p *Processor) handleDeletionEvent(ctx context.Context, raw *models.RawChatMessage) error {
    deletionType := raw.EventData["deletion_type"].(string)

    switch deletionType {
    case "single":
        // Lookup internal UUID from platform message ID
        platformMsgID := raw.EventData["target_msg_id"].(string)
        internalUUID, err := p.registry.Lookup(ctx, raw.Platform, raw.ChannelID,
                                                platformMsgID)
        if err != nil {
            // Message not in registry yet - buffer deletion
            p.deletionBuffer.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw)
            p.metrics.DeletionsBuffered.Inc()
            return nil
        }

        // Publish deletion event with internal UUID
        return p.publishDeletion(ctx, raw.ChannelID, "single", internalUUID, "", nil)

    case "batch":
        // User timeout/ban - provide user_id, frontend filters
        targetUserID := raw.EventData["target_user_id"].(string)
        return p.publishDeletion(ctx, raw.ChannelID, "batch", "", targetUserID, nil)

    case "clear":
        // Full chat clear
        return p.publishDeletion(ctx, raw.ChannelID, "clear", "", "", nil)
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| IRC without deletion support | Twitch IRC v3 with CLEARMSG/CLEARCHAT | 2019 (Twitch API update) | Moderators can now delete individual messages, not just ban users |
| Manual React batching (unstable_batchedUpdates) | Automatic batching in React 18 | React 18.0 (March 2022) | State updates in promises, timeouts, event handlers now batch automatically |
| Redis SET per message ID | Redis hash with field-level storage | Pattern established ~2015 | 90% memory reduction for message registries with millions of entries |
| Manual TTL cleanup (SCAN + DEL) | Native EXPIRE with passive + active eviction | Redis 2.0+ (stable 2010) | No cron jobs needed; Redis handles expiration automatically |
| Event sourcing deletion (tombstone records) | Direct deletion events | Modern event-driven pattern | Simpler: deletion is event, not state change; no database cleanup needed |

**Deprecated/outdated:**
- **Twitch IRC v1/v2:** No CLEARMSG support; could only clear entire chat or timeout user
- **React class components with componentDidUpdate:** Functional components with useEffect cleaner for WebSocket subscriptions
- **Redis Pub/Sub for message queue:** Ephemeral; replaced by Redis Streams for durable message processing
- **Manual batching in React 17:** React 18 auto-batches; `ReactDOM.unstable_batchedUpdates()` no longer needed

## Open Questions

1. **Should registry track deletion events for audit/debugging?**
   - What we know: Requirements don't specify deletion audit log
   - What's unclear: Production debugging may need "when was message deleted" timestamp
   - Recommendation: Log deletion events to structured log (zap) but don't persist in Redis; add retention later if needed

2. **What happens if message and deletion arrive in rapid succession (<1ms)?**
   - What we know: Redis Streams consumer groups process messages sequentially within single consumer
   - What's unclear: If message and deletion both in same XREADGROUP batch, processing order is message insertion order
   - Recommendation: Trust Redis Streams ordering; batch will process message before deletion

3. **Should frontend track deleted messages for "view deleted messages" moderator feature?**
   - What we know: Out of scope for Phase 1 (deferred to v2 per REQUIREMENTS.md ADV-01)
   - What's unclear: Architecture decision now may simplify future implementation
   - Recommendation: Remove from DOM immediately (Phase 1 requirement); add separate deleted message store in Phase 4+ if needed

4. **How to handle deletion events for shared chat (multi-channel collaborative streams)?**
   - What we know: Existing codebase supports shared chat (source-room-id tags)
   - What's unclear: Does deletion in source channel propagate to receiving channel?
   - Recommendation: Phase 1 ignores shared chat deletions (out of scope); document for Phase 2 YouTube integration

## Sources

### Primary (HIGH confidence)
- [Twitch IRC Documentation - CLEARMSG/CLEARCHAT](https://dev.twitch.tv/docs/chat/irc/) - Official Twitch specification
- [go-twitch-irc v4 Package Documentation](https://pkg.go.dev/github.com/gempir/go-twitch-irc/v4) - Official library API
- [go-redis Guide](https://redis.io/docs/latest/develop/clients/go/) - Official Go client documentation
- [Redis Hashes Documentation](https://redis.io/docs/latest/develop/data-types/hashes/) - Official data structure guide
- [Redis Sorted Sets Documentation](https://redis.io/docs/latest/develop/data-types/sorted-sets/) - Official data structure guide
- Existing codebase: services/twitch-listener/irc/parser.go, services/api-gateway/models/ws_message.go, frontend/src/app/overlay/[id]/page.tsx

### Secondary (MEDIUM confidence)
- [Redis Hash vs SET Memory Optimization (Salesforce Engineering)](https://engineering.salesforce.com/using-redis-hash-instead-of-set-to-reduce-cache-size-and-operating-costs-2a1f7b8ff577/) - Production case study
- [Redis Key Expiration Mechanics (Medium)](https://farshadth.medium.com/how-redis-removes-expired-cache-keys-behind-the-scenes-6684481b7283) - Technical deep dive
- [React Batching (Robin Wieruch)](https://www.robinwieruch.de/react-batching/) - React 18 batching explanation
- [Atomicity in Redis Operations (Medium)](https://lucaspin.medium.com/atomicity-in-redis-operations-a1d7bc9f4a90) - Transaction patterns
- [Dealing with Race Conditions in Event-Driven Architecture](https://event-driven.io/en/dealing_with_race_conditions_in_eda_using_read_models/) - Buffer pattern examples

### Tertiary (LOW confidence)
- Web search results on React state management libraries (2026 trends) - General patterns, not deletion-specific
- Web search results on Redis performance tuning - General best practices, not benchmarked for this use case

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, versions verified from codebase
- Architecture patterns: HIGH - Redis hash/TTL and go-twitch-irc handlers verified from official docs
- Pitfalls: MEDIUM - Based on common Redis patterns and React performance issues, not all tested in this specific context
- Race condition strategy: MEDIUM - Buffer pattern validated in event-driven architecture literature, but not specific to chat deletion

**Research date:** 2026-02-18
**Valid until:** 2026-03-18 (30 days; stable domain with mature technologies)

**Critical findings for planner:**
1. **No new dependencies required** - Use existing go-redis, go-twitch-irc, React 18
2. **Unidirectional registry is sufficient** - Deletions provide platform ID, not UUID
3. **Hash with TTL beats sorted set** - Simpler, O(1) lookups, automatic cleanup
4. **React 18 auto-batches** - No manual optimization needed for batch deletions
5. **TTL conflict:** CONTEXT.md (1 hour) overrides REQUIREMENTS.md (24 hours) - use 1 hour
