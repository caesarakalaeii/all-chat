# Phase 2: YouTube Integration - Research

**Researched:** 2026-02-18
**Domain:** YouTube Live Chat API deletion event detection via polling with 30-60s latency
**Confidence:** HIGH

## Summary

Phase 2 integrates YouTube message deletion support by leveraging the existing YouTube Listener polling architecture and Phase 1's deletion infrastructure. YouTube API provides `messageDeletedEvent` and `userBannedEvent` message types through the same polling mechanism used for chat messages, requiring no additional API calls or quota consumption. The existing parser (lines 98-116 in `api/parser.go`) already detects and captures deletion events, but they are not currently processed by the Message Processor's deletion pipeline.

**Primary finding:** YouTube deletion support requires minimal new code because both the detection mechanism (API polling) and processing infrastructure (Message ID Registry, Deletion Buffer, normalization pipeline) already exist. The work is integration: routing YouTube deletion events through the Phase 1 deletion pipeline and adding YouTube platform-specific message IDs to the registry.

**Key architectural insight:** YouTube's polling-based detection inherently provides 30-60s deletion latency (dictated by `pollingIntervalMillis` from API). The Phase 1 Deletion Buffer's 60-second TTL perfectly accommodates this lag, handling race conditions where deletion events arrive before corresponding messages due to processing delays or overlapping polling intervals.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| YOUTUBE-01 | Listener polls for messageDeletedEvent message type | YouTube API parser (lines 98-103) already extracts `MessageDeletedDetails.DeletedMessageId`; needs routing to deletion pipeline |
| YOUTUBE-02 | YouTube deletion events processed within existing polling interval | Deletion events arrive in same API response as chat messages (no additional quota); processed inline during `ParseBatch()` |
| YOUTUBE-03 | System handles 60-second polling lag gracefully (via deletion buffer) | Phase 1 Deletion Buffer provides 60s TTL, matching max polling interval; handles out-of-order events |

</phase_requirements>

## Standard Stack

### Core (Already in Use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| google.golang.org/api/youtube/v3 | Latest | YouTube Data API v3 client | Official Google Go client; production-proven in codebase |
| github.com/caesar/.../youtube-listener/api | Internal | YouTube API response parser | Existing parser already handles deletion events (lines 98-116) |
| github.com/caesar/.../message-processor/registry | Internal | Message ID Registry (Phase 1) | Redis-backed UUID mapping with 1-hour TTL |
| github.com/caesar/.../message-processor/normalizer | Internal | Platform normalization | Existing `NormalizeDeletion()` function handles all platforms |

### Supporting (No New Dependencies Required)
All features implementable with existing stack. YouTube Listener and Message Processor already contain necessary primitives.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Polling-based detection | YouTube PubSubHubbub webhooks | Webhooks not supported for live chat messages (only for video uploads/comments) |
| Inline detection during polling | Separate deletion polling endpoint | YouTube API doesn't provide separate deletion endpoint; deletions arrive in same `liveChatMessages.list` response |
| Custom deletion event schema | Reuse Phase 1 schema | Phase 1 schema (`DeletionEvent` with `deletion_type`, `target_msg_id`, `target_user_id`) already supports YouTube's two deletion types |

**Installation:**
No new dependencies required. All features use existing `go.mod` entries and Phase 1 infrastructure.

## Architecture Patterns

### Recommended Integration Points

**Location:** Extend existing YouTube Listener and Message Processor

```
services/youtube-listener/
├── api/
│   ├── parser.go           # MODIFY: Route deletion events to RawChatMessage.EventType = "message_deletion"
│   └── parser_test.go      # ADD: Test deletion event parsing
└── models/
    └── raw_message.go      # EXISTING: RawChatMessage already supports EventType and EventData

services/message-processor/
├── consumer/
│   └── stream_consumer.go  # MODIFY: Add YouTube message IDs to registry on arrival
└── normalizer/
    └── normalizer.go       # EXISTING: NormalizeDeletion() already handles YouTube
```

### Pattern 1: YouTube Deletion Detection (EXISTING BUT UNUSED)

**What:** YouTube API parser already extracts deletion events but treats them as regular events

**Current Implementation (parser.go lines 98-116):**
```go
// Source: /home/caesar/git/all-chat/services/youtube-listener/api/parser.go
} else if msg.Snippet.MessageDeletedDetails != nil {
    // Message deleted (moderation) event
    eventType = "message_deleted"
    text = "Message deleted"
    eventData["deleted_message_id"] = msg.Snippet.MessageDeletedDetails.DeletedMessageId

} else if msg.Snippet.UserBannedDetails != nil {
    // User banned (moderation) event
    eventType = "user_banned"
    text = "User banned"
    if msg.Snippet.UserBannedDetails.BannedUserDetails != nil {
        eventData["banned_user_id"] = msg.Snippet.UserBannedDetails.BannedUserDetails.ChannelId
        eventData["banned_user_name"] = msg.Snippet.UserBannedDetails.BannedUserDetails.DisplayName
    }
    eventData["ban_type"] = msg.Snippet.UserBannedDetails.BanType
    if msg.Snippet.UserBannedDetails.BanDurationSeconds > 0 {
        eventData["ban_duration_seconds"] = msg.Snippet.UserBannedDetails.BanDurationSeconds
    }
}
```

**Required Change:** Convert to `EventType = "message_deletion"` with Phase 1 schema

```go
// NEW: Map YouTube deletion events to Phase 1 deletion schema
} else if msg.Snippet.MessageDeletedDetails != nil {
    // Single message deletion
    eventType = "message_deletion"
    text = "" // Deletion events don't need text
    eventData["deletion_type"] = "single"
    eventData["target_msg_id"] = msg.Snippet.MessageDeletedDetails.DeletedMessageId

} else if msg.Snippet.UserBannedDetails != nil {
    // User ban (batch deletion)
    eventType = "message_deletion"
    text = ""
    eventData["deletion_type"] = "batch"

    if msg.Snippet.UserBannedDetails.BannedUserDetails != nil {
        eventData["target_user_id"] = msg.Snippet.UserBannedDetails.BannedUserDetails.ChannelId
    }

    // Optional metadata for logging/debugging
    eventData["ban_type"] = msg.Snippet.UserBannedDetails.BanType
    if msg.Snippet.UserBannedDetails.BanDurationSeconds > 0 {
        eventData["ban_duration_seconds"] = msg.Snippet.UserBannedDetails.BanDurationSeconds
    }
}
```

**Why this works:** Message Processor already checks for `raw.EventType == "message_deletion"` (main.go line 284) and routes to `NormalizeDeletion()`.

### Pattern 2: YouTube Message ID Registry Integration

**What:** Add YouTube platform message IDs to Phase 1 registry when messages arrive

**Integration Point:** Message Processor stream consumer (existing pattern from Twitch)

**Current Twitch Pattern (would be in consumer):**
```go
// Phase 1 added Twitch message IDs to registry:
platformMsgID := raw.Tags["id"] // Twitch IRC message ID
p.registry.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw.MessageID)
```

**YouTube Pattern (NEW):**
```go
// YouTube messages already have `id` in Snippet
// Parser needs to capture this in Tags map:

// In parser.go ParseChatMessage():
tags["youtube_message_id"] = msg.Id // YouTube's platform message ID

// In message-processor consumer:
if raw.Platform == "youtube" {
    platformMsgID := raw.Tags["youtube_message_id"]
    if platformMsgID != "" {
        p.registry.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw.MessageID)
    }
}
```

**Registry Key Structure:**
```
Key: msgid:registry:youtube:{channel_id}
Fields:
  {youtube_msg_id} → {internal_uuid}|{timestamp}

Example:
  HSET msgid:registry:youtube:UCxxxxxx
       "Cg0KCzkzMTY5..." "550e8400-e29b-41d4-a716-446655440000|1708281600"
```

### Pattern 3: YouTube Deletion Event Flow

**What:** Extend existing deletion pipeline to handle YouTube-specific events

**Flow Diagram:**
```
YouTube API Poll Response
  ↓ Contains mix of chat messages + deletion events
YouTube Listener Parser (ParseBatch)
  ↓ Separates messageDeletedEvent / userBannedEvent
RawChatMessage (EventType = "message_deletion")
  ↓ Published to Redis Streams (chat:raw)
Message Processor Consumer
  ├─ Regular messages → Registry.Add(youtube_msg_id → uuid)
  └─ Deletion events → NormalizeDeletion() → Lookup target UUID
       ↓
    Redis Pub/Sub (overlay:{overlay_id})
       ↓
    API Gateway WebSocket
       ↓
    Frontend (filter by message.id)
```

**Key Difference from Twitch:** YouTube deletions arrive in same polling response as messages (not separate IRC commands), so ordering is determined by API response order.

### Pattern 4: Deletion Buffer Usage for YouTube

**What:** Phase 1 Deletion Buffer handles YouTube's polling lag

**Scenario 1: Normal Order (Message Before Deletion)**
```
Poll at T+0s: [Message A]
  → Registry: youtube_msg_123 → uuid_A
Poll at T+5s: [Deletion youtube_msg_123]
  → Registry lookup: youtube_msg_123 → uuid_A
  → Publish deletion event with uuid_A
  → Frontend removes message
```

**Scenario 2: Out-of-Order (Deletion Before Message)**
```
Poll at T+0s: [Message A, Deletion youtube_msg_456]
  → Process Message A → Registry.Add(youtube_msg_456 → uuid_B)
  → Process Deletion youtube_msg_456 → Registry.Lookup → ErrMessageNotFound
  → Buffer deletion event with 60s TTL

Poll at T+5s: [Message B with youtube_msg_456]
  → Consumer checks buffer → Found buffered deletion
  → Apply deletion immediately
  → Remove from buffer
```

**Scenario 3: Race Condition (Messages in Different Polling Windows)**
```
Poll at T+0s: [Message A sent, moderator deletes immediately]
Poll at T+2s: [Deletion youtube_msg_789] ← Arrives first due to API batching
  → Registry lookup fails → Buffer deletion (TTL 60s)

Poll at T+5s: [Message A with youtube_msg_789] ← Arrives late
  → Registry.Add(youtube_msg_789 → uuid_C)
  → Check buffer → Found deletion
  → Apply deletion immediately
  → Message never reaches frontend (deleted before publish)
```

**Buffer Expiration:**
- Deletion event buffered at T+0s
- Message never arrives (network error, service restart)
- Buffer entry expires at T+60s
- Metrics track expired deletions (no error logged)

### Anti-Patterns to Avoid

- **Separate API call for deletions:** YouTube doesn't provide separate deletion endpoint; deletions arrive in regular `liveChatMessages.list` response
- **Custom polling interval for deletions:** Use same `pollingIntervalMillis` from API; separate polling wastes quota and increases latency
- **Storing YouTube message IDs in RawChatMessage.MessageID:** This would break internal UUID tracking; store YouTube IDs in `Tags["youtube_message_id"]` only
- **Full chat clear event:** YouTube API doesn't provide "clear all chat" event; only single message deletion and user ban
- **Assuming instant deletion:** YouTube polling has 2-60s latency (typically 5s); frontend must handle delayed deletions gracefully

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YouTube message ID extraction | Custom Snippet parsing | Existing parser.go `msg.Id` field | YouTube API already provides message ID in `LiveChatMessage.Id` field |
| Deletion event detection | Poll separate API endpoint | Existing polling response parsing | Deletion events arrive in same `liveChatMessages.list` response as regular messages |
| Out-of-order handling | Custom timestamp-based ordering | Phase 1 Deletion Buffer | Buffer already handles 60s race condition window with automatic expiration |
| Deletion normalization | YouTube-specific deletion handler | Phase 1 `NormalizeDeletion()` | Function already supports platform-agnostic deletion with `deletion_type` field |
| Registry TTL management | Manual cleanup cron | Redis EXPIRE | Phase 1 registry already uses automatic 1-hour TTL with refresh on access |

**Key insight:** Phase 1 built a complete deletion infrastructure. Phase 2 is purely integration work: capturing YouTube message IDs and routing deletion events through existing pipeline.

## Common Pitfalls

### Pitfall 1: YouTube Message IDs Not Captured in Tags

**What goes wrong:** Deletion events reference YouTube message IDs, but registry lookup fails because message IDs were never added.

**Why it happens:** Current parser extracts YouTube message ID (`msg.Id`) but doesn't add it to `tags` map for registry consumption.

**How to avoid:** Modify `ParseChatMessage()` to add `tags["youtube_message_id"] = msg.Id` before creating `RawChatMessage`.

**Warning signs:** Message Processor logs show "message not found in registry" for YouTube deletions immediately after message arrival.

**Example:**
```go
// In parser.go ParseChatMessage():
tags["youtube_message_id"] = msg.Id // ADD THIS LINE

rawMsg := &models.RawChatMessage{
    MessageID: uuid.New().String(), // Internal UUID
    Platform:  "youtube",
    // ... rest of fields
    Tags: tags, // Now includes youtube_message_id
}
```

### Pitfall 2: Treating YouTube Deletions as Display Events

**What goes wrong:** Parser sets `EventType = "message_deleted"` (display event) instead of `"message_deletion"` (internal deletion event), causing deletions to appear in overlay as notification banners instead of removing messages.

**Why it happens:** Confusion between user-facing events (Super Chat, membership) and internal deletion events.

**How to avoid:** Use `EventType = "message_deletion"` for all deletion events. Message Processor checks this exact string (line 284 in main.go).

**Warning signs:** Frontend logs show "Unknown event type: message_deleted" or deletion events appear as visible messages instead of removing content.

**Fix:**
```go
// ❌ WRONG: User-facing event
eventType = "message_deleted" // Would trigger event display

// ✅ CORRECT: Internal deletion event
eventType = "message_deletion" // Triggers NormalizeDeletion()
```

### Pitfall 3: Polling Interval Assumptions

**What goes wrong:** Assuming YouTube polling is always 2-5 seconds, but API can recommend up to 60 seconds during low activity.

**Why it happens:** API's `pollingIntervalMillis` varies based on chat activity (2s for active, 60s for quiet).

**How to avoid:** Always respect `response.PollingIntervalMillis` from API. Phase 1 Deletion Buffer's 60s TTL accommodates max interval.

**Warning signs:** Deletion events expire from buffer before messages arrive, causing missed deletions in quiet chats.

**Monitoring:**
```go
metrics.DeletionsBuffered.Inc()        // Track buffered deletions
metrics.BufferedDeletionsApplied.Inc() // Track successful applications
metrics.BufferedDeletionsExpired.Inc() // Track expiration (should be rare)
```

### Pitfall 4: Ban Events Not Mapped to Batch Deletions

**What goes wrong:** YouTube `userBannedEvent` detected but not converted to batch deletion, leaving banned user's messages visible.

**Why it happens:** Parser captures ban event but doesn't set `deletion_type = "batch"` or `target_user_id`.

**How to avoid:** Map `userBannedEvent` to Phase 1 batch deletion schema with `target_user_id` from `BannedUserDetails.ChannelId`.

**Example:**
```go
// YouTube userBannedEvent → Phase 1 batch deletion
eventType = "message_deletion"
eventData["deletion_type"] = "batch"
eventData["target_user_id"] = msg.Snippet.UserBannedDetails.BannedUserDetails.ChannelId

// Frontend filters: setMessages(prev => prev.filter(m => m.user.id !== target_user_id))
```

**Warning signs:** User gets timed out/banned but their messages remain visible in overlay.

### Pitfall 5: Full Chat Clear Expectation

**What goes wrong:** Implementation attempts to detect YouTube "clear all chat" event, but YouTube API doesn't provide this event type.

**Why it happens:** Assuming YouTube has feature parity with Twitch's `/clear` command.

**How to avoid:** Document limitation. YouTube doesn't support full chat clear (API provides only `messageDeletedEvent` and `userBannedEvent`).

**Phase scope:** Phase 2 supports single and batch deletions only. Full clear is out of scope (YouTube platform limitation, not implementation gap).

**Future consideration:** If YouTube adds full clear support, can be added as `deletion_type = "clear"` using existing schema.

## Code Examples

Verified patterns from official sources and existing codebase:

### YouTube API Deletion Event Structure

```go
// Source: https://developers.google.com/youtube/v3/live/docs/liveChatMessages
// Official YouTube Data API v3 LiveChatMessage resource

type LiveChatMessage struct {
    Kind    string   `json:"kind"`
    Etag    string   `json:"etag"`
    Id      string   `json:"id"` // Platform message ID
    Snippet *Snippet `json:"snippet"`
    // ... other fields
}

type Snippet struct {
    Type                    string                   `json:"type"` // "messageDeletedEvent" or "userBannedEvent"
    MessageDeletedDetails   *MessageDeletedDetails   `json:"messageDeletedDetails,omitempty"`
    UserBannedDetails       *UserBannedDetails       `json:"userBannedDetails,omitempty"`
    // ... other fields for regular messages
}

type MessageDeletedDetails struct {
    DeletedMessageId string `json:"deletedMessageId"` // YouTube message ID of deleted message
}

type UserBannedDetails struct {
    BannedUserDetails   *BannedUserDetails `json:"bannedUserDetails,omitempty"`
    BanType             string             `json:"banType"` // "permanent" or "temporary"
    BanDurationSeconds  uint64             `json:"banDurationSeconds,omitempty"` // Only for temporary
}

type BannedUserDetails struct {
    ChannelId       string `json:"channelId"`       // YouTube user channel ID
    ChannelUrl      string `json:"channelUrl"`
    DisplayName     string `json:"displayName"`
    ProfileImageUrl string `json:"profileImageUrl"`
}
```

### Modified Parser for Deletion Events

```go
// Source: Research recommendation based on existing parser.go
// Modify ParseChatMessage() to handle deletion events with Phase 1 schema

func (p *Parser) ParseChatMessage(msg *youtube.LiveChatMessage, channelID, streamID string) (*models.RawChatMessage, error) {
    // ... existing validation ...

    // Determine event type and extract message text and event data
    eventType := ""
    eventData := make(map[string]interface{})
    text := ""

    if msg.Snippet.TextMessageDetails != nil {
        // Regular chat message (existing code)
        text = msg.Snippet.TextMessageDetails.MessageText

    } else if msg.Snippet.MessageDeletedDetails != nil {
        // MODIFIED: Single message deletion → Phase 1 schema
        eventType = "message_deletion"
        eventData["deletion_type"] = "single"
        eventData["target_msg_id"] = msg.Snippet.MessageDeletedDetails.DeletedMessageId

    } else if msg.Snippet.UserBannedDetails != nil {
        // MODIFIED: User ban → Phase 1 batch deletion schema
        eventType = "message_deletion"
        eventData["deletion_type"] = "batch"

        if msg.Snippet.UserBannedDetails.BannedUserDetails != nil {
            eventData["target_user_id"] = msg.Snippet.UserBannedDetails.BannedUserDetails.ChannelId
        }

        // Optional metadata for logging/metrics
        eventData["ban_type"] = msg.Snippet.UserBannedDetails.BanType
        if msg.Snippet.UserBannedDetails.BanDurationSeconds > 0 {
            eventData["ban_duration_seconds"] = msg.Snippet.UserBannedDetails.BanDurationSeconds
        }

    } else if msg.Snippet.SuperChatDetails != nil {
        // Super Chat event (existing code unchanged)
        eventType = "super_chat"
        // ... existing Super Chat handling ...

    } // ... other event types unchanged ...

    // Build tags map with YouTube-specific metadata
    tags := make(map[string]string)

    // NEW: Add YouTube message ID for registry
    tags["youtube_message_id"] = msg.Id // Platform message ID for registry lookup

    // Existing tags
    tags["channel_id"] = msg.AuthorDetails.ChannelId
    tags["channel_url"] = msg.AuthorDetails.ChannelUrl
    // ... rest of existing tags ...

    // Create RawChatMessage with event fields
    rawMsg := &models.RawChatMessage{
        MessageID: uuid.New().String(), // Internal UUID
        Platform:  "youtube",
        ChannelID: channelID,
        StreamID:  streamID,
        UserID:    msg.AuthorDetails.ChannelId,
        Username:  displayName,
        Text:      text,
        Timestamp: timestamp,
        Tags:      tags,             // Now includes youtube_message_id
        EventType: eventType,        // "message_deletion" for deletions
        EventData: eventData,        // Contains deletion_type and target IDs
    }

    return rawMsg, nil
}
```

### Message Processor Registry Integration

```go
// Source: Research recommendation based on Phase 1 consumer pattern
// Extend stream_consumer.go to add YouTube message IDs to registry

func (c *Consumer) processMessage(ctx context.Context, raw *models.RawChatMessage) error {
    // Existing deletion handling (unchanged)
    if raw.EventType == "message_deletion" {
        return c.handleDeletion(ctx, raw)
    }

    // REGULAR MESSAGE PATH

    // EXISTING: Twitch message IDs already added to registry
    if raw.Platform == "twitch" {
        platformMsgID := raw.Tags["id"] // Twitch IRC message ID
        if platformMsgID != "" {
            c.registry.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw.MessageID)
        }
    }

    // NEW: Add YouTube message IDs to registry
    if raw.Platform == "youtube" {
        platformMsgID := raw.Tags["youtube_message_id"] // YouTube API message ID
        if platformMsgID != "" {
            if err := c.registry.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw.MessageID); err != nil {
                c.logger.Warn("Failed to add YouTube message to registry",
                    zap.Error(err),
                    zap.String("youtube_msg_id", platformMsgID),
                    zap.String("internal_uuid", raw.MessageID),
                )
                // Continue processing; registry is best-effort
            }

            // Check if deletion was buffered for this message
            if deletion := c.deletionBuffer.Get(ctx, raw.Platform, raw.ChannelID, platformMsgID); deletion != nil {
                c.handleDeletion(ctx, deletion)
                c.deletionBuffer.Remove(ctx, raw.Platform, raw.ChannelID, platformMsgID)
                c.metrics.BufferedDeletionsApplied.Inc()
            }
        }
    }

    // Continue with normal processing (enrichment, routing, publishing)
    return c.enrichAndPublish(ctx, raw)
}

func (c *Consumer) handleDeletion(ctx context.Context, raw *models.RawChatMessage) error {
    deletionType := raw.EventData["deletion_type"].(string)

    switch deletionType {
    case "single":
        // Lookup internal UUID from platform message ID
        platformMsgID := raw.EventData["target_msg_id"].(string)
        internalUUID, err := c.registry.Lookup(ctx, raw.Platform, raw.ChannelID, platformMsgID)

        if err != nil {
            // Message not in registry yet - buffer deletion
            c.deletionBuffer.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw)
            c.metrics.DeletionsBuffered.Inc()
            return nil
        }

        // Publish deletion event with internal UUID
        return c.publishDeletion(ctx, raw.ChannelID, "single", internalUUID, "", nil)

    case "batch":
        // User timeout/ban - provide user_id, frontend filters
        targetUserID := raw.EventData["target_user_id"].(string)
        return c.publishDeletion(ctx, raw.ChannelID, "batch", "", targetUserID, nil)

    case "clear":
        // Full chat clear (YouTube doesn't support this)
        return c.publishDeletion(ctx, raw.ChannelID, "clear", "", "", nil)
    }

    return nil
}
```

### Test: YouTube Deletion Event Parsing

```go
// NEW: services/youtube-listener/api/parser_test.go
// Verify deletion events convert to Phase 1 schema

func TestParseYouTubeDeletionEvents(t *testing.T) {
    parser := NewParser()

    t.Run("MessageDeletedEvent", func(t *testing.T) {
        msg := &youtube.LiveChatMessage{
            Id: "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgB",
            Snippet: &youtube.LiveChatMessageSnippet{
                Type: "messageDeletedEvent",
                MessageDeletedDetails: &youtube.LiveChatMessageDeletedDetails{
                    DeletedMessageId: "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgA",
                },
            },
            AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
                ChannelId:   "UC_mod_channel_id",
                DisplayName: "ModeratorName",
            },
        }

        raw, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")
        require.NoError(t, err)

        // Verify Phase 1 deletion schema
        assert.Equal(t, "message_deletion", raw.EventType)
        assert.Equal(t, "single", raw.EventData["deletion_type"])
        assert.Equal(t, "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgA",
                     raw.EventData["target_msg_id"])

        // Verify YouTube message ID in tags
        assert.Equal(t, "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgB",
                     raw.Tags["youtube_message_id"])
    })

    t.Run("UserBannedEvent_Permanent", func(t *testing.T) {
        msg := &youtube.LiveChatMessage{
            Id: "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgC",
            Snippet: &youtube.LiveChatMessageSnippet{
                Type: "userBannedEvent",
                UserBannedDetails: &youtube.LiveChatUserBannedMessageDetails{
                    BanType: "permanent",
                    BannedUserDetails: &youtube.ChannelProfileDetails{
                        ChannelId:   "UC_banned_user_id",
                        DisplayName: "BannedUser",
                    },
                },
            },
            AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
                ChannelId:   "UC_mod_channel_id",
                DisplayName: "ModeratorName",
            },
        }

        raw, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")
        require.NoError(t, err)

        // Verify Phase 1 batch deletion schema
        assert.Equal(t, "message_deletion", raw.EventType)
        assert.Equal(t, "batch", raw.EventData["deletion_type"])
        assert.Equal(t, "UC_banned_user_id", raw.EventData["target_user_id"])
        assert.Equal(t, "permanent", raw.EventData["ban_type"])
    })

    t.Run("UserBannedEvent_Temporary", func(t *testing.T) {
        msg := &youtube.LiveChatMessage{
            Id: "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JBEhxDTXV4c2YyZEJJY0NGUWVQdXdvZDFjd0JRQSgD",
            Snippet: &youtube.LiveChatMessageSnippet{
                Type: "userBannedEvent",
                UserBannedDetails: &youtube.LiveChatUserBannedMessageDetails{
                    BanType:            "temporary",
                    BanDurationSeconds: 600, // 10 minute timeout
                    BannedUserDetails: &youtube.ChannelProfileDetails{
                        ChannelId:   "UC_timed_out_user_id",
                        DisplayName: "TimedOutUser",
                    },
                },
            },
            AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
                ChannelId:   "UC_mod_channel_id",
                DisplayName: "ModeratorName",
            },
        }

        raw, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")
        require.NoError(t, err)

        // Verify batch deletion with timeout metadata
        assert.Equal(t, "message_deletion", raw.EventType)
        assert.Equal(t, "batch", raw.EventData["deletion_type"])
        assert.Equal(t, "UC_timed_out_user_id", raw.EventData["target_user_id"])
        assert.Equal(t, "temporary", raw.EventData["ban_type"])
        assert.Equal(t, 600, raw.EventData["ban_duration_seconds"])
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate deletion polling | Deletion events in same response | YouTube API design (2015+) | No additional quota cost; deletions arrive with regular messages |
| Manual out-of-order handling | Deletion Buffer with TTL | Phase 1 (Feb 2026) | Handles 60s polling lag automatically; no manual timestamp comparison |
| Platform-specific deletion schemas | Unified deletion schema | Phase 1 (Feb 2026) | Single `NormalizeDeletion()` function handles all platforms |
| YouTube message IDs not tracked | Redis-based Message ID Registry | Phase 1 (Feb 2026) | O(1) UUID lookup for deletion matching |
| Display deletion events to users | Internal deletion events | Phase 1 (Feb 2026) | Deletions remove messages instead of showing notification banners |

**Deprecated/outdated:**
- **Custom deletion event types:** Phase 1 standardized on `EventType = "message_deletion"` with `deletion_type` field
- **Separate deletion API endpoints:** YouTube API doesn't provide these; deletions arrive in `liveChatMessages.list` response
- **Manual expiration handling:** Redis EXPIRE handles automatic cleanup of registry and buffer entries
- **Full chat clear for YouTube:** Not supported by YouTube API (only single/batch deletions available)

## Open Questions

1. **Should we track deletion events per YouTube channel for quota debugging?**
   - What we know: YouTube quota tracking exists (1,009,000 units/day limit)
   - What's unclear: Does tracking deletion event frequency help optimize quota usage?
   - Recommendation: Add Prometheus metric `youtube_deletion_events_total{channel_id, deletion_type}` for monitoring; no persistent storage needed

2. **What happens if YouTube message arrives in one polling window and deletion in next?**
   - What we know: Deletion Buffer has 60s TTL, matching max polling interval
   - What's unclear: If polling interval is 60s and message arrives at T+59s, will deletion at T+61s find it?
   - Recommendation: Registry TTL (1 hour) >> polling interval (max 60s), so registry lookup succeeds even across polling windows

3. **Should frontend display ban duration for temporary bans?**
   - What we know: YouTube provides `ban_duration_seconds` in event metadata
   - What's unclear: Phase scope includes UI notification or just message removal?
   - Recommendation: Phase 2 focuses on message removal only; defer ban duration display to v2 (UX-03 requirement)

4. **How to handle YouTube quota exhaustion during active deletion events?**
   - What we know: Poller stops entirely when quota exceeded (line 297-302 in poller.go)
   - What's unclear: Do buffered deletions expire if quota exhausted for >60s?
   - Recommendation: Document limitation - deletion events during quota exhaustion are lost; quota reset at midnight PT resumes normal operation

5. **Should Phase 2 include integration tests with real YouTube API?**
   - What we know: Phase 1 tested Twitch integration end-to-end (human verification)
   - What's unclear: Can we simulate YouTube deletion events without consuming quota?
   - Recommendation: Use mock YouTube API responses for unit tests; defer live API testing to Phase 5 (end-to-end validation) to avoid quota waste

## Sources

### Primary (HIGH confidence)
- [YouTube Data API v3 - LiveChatMessages](https://developers.google.com/youtube/v3/live/docs/liveChatMessages) - Official documentation for LiveChatMessage resource
- [YouTube Live Streaming API Reference](https://developers.google.com/youtube/v3/live/docs) - Complete API reference
- Existing codebase: `/home/caesar/git/all-chat/services/youtube-listener/api/parser.go` (lines 98-116) - Deletion event parsing already implemented
- Existing codebase: `/home/caesar/git/all-chat/services/message-processor/cmd/main.go` (lines 284-294) - Deletion event routing
- Existing codebase: `/home/caesar/git/all-chat/services/message-processor/registry/registry.go` - Message ID Registry implementation
- Phase 1 Research: `/home/caesar/git/all-chat/.planning/phases/01-foundation-twitch/01-RESEARCH.md` - Deletion infrastructure architecture

### Secondary (MEDIUM confidence)
- [LiveChatMessageSnippet - Java API Docs](https://developers.google.com/resources/api-libraries/documentation/youtube/v3/java/latest/com/google/api/services/youtube/model/LiveChatMessageSnippet.html) - Field-level documentation
- [Python YouTube API Client](https://googleapis.github.io/google-api-python-client/docs/dyn/youtube_v3.liveChatMessages.html) - Alternative language reference
- WebSearch verification (Feb 2026): YouTube API docs updated February 2026, confirming current event structure

### Tertiary (LOW confidence)
- None - all findings verified from official documentation or existing codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All components already in use (Phase 1 infrastructure + existing YouTube Listener)
- Architecture patterns: HIGH - Parser already extracts deletion events; integration points clearly defined
- Pitfalls: MEDIUM - Based on common polling and registry patterns, not YouTube-specific deletion production data
- Quota impact: HIGH - Deletion events arrive in same response, verified from API docs (zero additional quota cost)

**Research date:** 2026-02-18
**Valid until:** 2026-03-18 (30 days; YouTube API stable, Phase 1 infrastructure recently completed)

**Critical findings for planner:**
1. **Minimal new code required** - Parser already detects deletions; needs Phase 1 schema mapping only
2. **Zero quota impact** - Deletion events arrive in existing polling response (no additional API calls)
3. **Deletion Buffer handles polling lag** - 60s TTL matches YouTube's max polling interval
4. **Two deletion types only** - Single message deletion and user ban; no full chat clear (API limitation)
5. **Registry integration straightforward** - Add `tags["youtube_message_id"]` in parser, register in consumer
