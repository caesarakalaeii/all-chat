# Stack Research: Message Deletion Events

**Domain:** Streaming chat aggregation message deletion
**Researched:** 2026-02-17
**Confidence:** HIGH

## Executive Summary

All four streaming platforms provide mechanisms to detect and handle message deletion events, but with varying approaches:

- **Twitch**: Native IRC commands (CLEARMSG, CLEARCHAT) with full support in go-twitch-irc v4
- **YouTube**: API-native deletion events via polling with dedicated message type
- **Kick**: Pusher WebSocket events (unofficial, reverse-engineered)
- **TikTok**: **NO DELETION SUPPORT** in tiktok-live-connector library

The existing All-Chat stack requires NO new libraries - all platforms except TikTok can be supported with current dependencies. TikTok message deletion is not feasible without official API support.

---

## Platform-Specific Stack Requirements

### 1. Twitch (IRC)

**Status:** ✅ **FULLY SUPPORTED** - No new libraries needed

**Library:** `gempir/go-twitch-irc/v4` v4.3.1 (already in use)

**Deletion Mechanisms:**

| Event Type | IRC Command | Purpose | Identifier |
|------------|-------------|---------|------------|
| Single Message Deletion | `CLEARMSG` | Moderator deletes one message | `target-msg-id` (UUID) |
| User Timeout/Ban | `CLEARCHAT` | Remove all messages from user | `target-user-id` |
| Full Chat Clear | `CLEARCHAT` | Clear entire chat | None (channel-wide) |

**Implementation Details:**

```go
// Callback registration (gempir/go-twitch-irc/v4)
client.OnClearMessage(func(msg twitch.ClearMessage) {
    // Single message deletion
    // msg.TargetMsgID - UUID of deleted message
    // msg.Channel - channel name
    // msg.Login - username of message author
})

client.OnClearChatMessage(func(msg twitch.ClearChatMessage) {
    // User timeout/ban or full chat clear
    // msg.TargetUsername - user whose messages to clear (empty for full clear)
    // msg.TargetUserID - user ID
    // msg.BanDuration - timeout duration in seconds (0 for permanent ban)
    // msg.Channel - channel name
})
```

**Data Structures:**

**ClearMessage** (single deletion):
- `TargetMsgID` (string): UUID of deleted message - **PRIMARY IDENTIFIER**
- `Channel` (string): Channel name
- `Login` (string): Username of original author
- `Tags` (map[string]string): Full IRC tags
- `Time` (time.Time): Timestamp

**ClearChatMessage** (bulk deletion):
- `TargetUsername` (string): Username (empty = full clear)
- `TargetUserID` (string): User ID (empty = full clear)
- `BanDuration` (int): Seconds (0 = permanent, empty = full clear)
- `RoomID` (string): Channel ID
- `Channel` (string): Channel name
- `Tags` (map[string]string): Full IRC tags
- `Time` (time.Time): Timestamp

**IRC Tag Details:**

CLEARMSG tags:
- `target-msg-id`: UUID format (e.g., "94e6c7ff-bf98-4faa-af5d-7ad633a158a9")
- `login`: Username
- `room-id`: Channel ID
- `tmi-sent-ts`: Unix timestamp (milliseconds)

CLEARCHAT tags:
- `target-user-id`: User ID (optional)
- `ban-duration`: Timeout seconds (optional, only for timeouts)
- `room-id`: Channel ID
- `tmi-sent-ts`: Unix timestamp (milliseconds)

**Confidence:** HIGH - Official Twitch IRC protocol, well-documented in go-twitch-irc v4

**Sources:**
- [Twitch IRC Tags Documentation](https://dev.twitch.tv/docs/irc/tags/)
- [Twitch IRC Commands](https://dev.twitch.tv/docs/irc/commands)
- [go-twitch-irc v4 Package Documentation](https://pkg.go.dev/github.com/gempir/go-twitch-irc/v4)
- [go-twitch-irc GitHub Repository](https://github.com/gempir/go-twitch-irc)

---

### 2. YouTube (HTTP Polling API)

**Status:** ✅ **FULLY SUPPORTED** - No new libraries needed

**Library:** YouTube Data API v3 (already in use via HTTP client)

**Deletion Mechanism:**

| Event Type | API Message Type | Purpose | Identifier |
|------------|------------------|---------|------------|
| Message Deletion | `messageDeletedEvent` | Moderator/owner deletes message | `deletedMessageId` |

**Implementation Details:**

YouTube deletions are detected via the same polling mechanism used for chat messages:

```go
// Existing polling flow (services/youtube-listener/streams/poller.go)
// GET /youtube/v3/liveChat/messages?liveChatId={id}&pageToken={token}

// Response includes messageDeletedEvent types
{
  "snippet": {
    "type": "messageDeletedEvent",  // Check this field
    "authorChannelId": "UCxxxxxxx",  // Moderator who deleted
    "publishedAt": "2026-02-17T10:00:00Z",
    "hasDisplayContent": false,
    "messageDeletedDetails": {
      "deletedMessageId": "original_message_id"  // PRIMARY IDENTIFIER
    }
  }
}
```

**API Response Structure:**

**snippet.type Values:**
- `"textMessageEvent"` - Normal chat message
- `"messageDeletedEvent"` - **Message deletion event**
- `"memberMilestoneChatEvent"` - Membership milestone
- `"newSponsorEvent"` - New sponsor
- etc.

**messageDeletedDetails Object:**
- `deletedMessageId` (string): ID of the original deleted message - **PRIMARY IDENTIFIER**

**Additional Context (snippet):**
- `authorChannelId` (string): Channel ID of moderator who deleted
- `publishedAt` (string): ISO 8601 timestamp of deletion
- `hasDisplayContent` (boolean): Always `false` for deletions

**API Quota Cost:**
- Same as regular message polling: **5 units per request**
- No additional quota consumption for deletion events

**Detection Flow:**
1. Poll `liveChatMessages.list` (existing behavior)
2. For each message in response, check `snippet.type`
3. If `type == "messageDeletedEvent"`, extract `messageDeletedDetails.deletedMessageId`
4. Match against cached messages using `deletedMessageId` (same format as original message `id`)
5. Publish deletion event to Redis

**Confidence:** HIGH - Official YouTube API, well-documented

**Sources:**
- [YouTube LiveChatMessages API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages)
- [YouTube LiveChatMessages.list Method](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/list)

---

### 3. Kick (Pusher WebSocket)

**Status:** ⚠️ **PARTIALLY SUPPORTED** - Unofficial, reverse-engineered

**Library:** `gorilla/websocket` (already in use)

**Deletion Mechanism:**

| Event Type | Pusher Event | Purpose | Identifier |
|------------|--------------|---------|------------|
| Message Deletion | `App\\Events\\ChatMessageDeletedEvent` | Message removed | `id` (UUID) |

**Implementation Details:**

```go
// Pusher WebSocket message format (services/kick-listener/websocket/client.go)
{
  "event": "App\\Events\\ChatMessageDeletedEvent",
  "channel": "chatrooms.123456",
  "data": {
    "id": "e4a2cc71-5e99-49e5-9112-2eafb30b414d",  // Deletion event ID
    "message": {
      "id": "original_message_uuid"  // PRIMARY IDENTIFIER
    },
    // OR (alternate structure)
    "deletedMessage": {
      "id": "original_message_uuid",  // PRIMARY IDENTIFIER
      "deleted_by": 12345,             // User ID of moderator
      "chatroom_id": 123456
    }
  }
}
```

**Event Structure:**

**ChatMessageDeletedEvent:**
- `event` (string): `"App\\Events\\ChatMessageDeletedEvent"`
- `channel` (string): `"chatrooms.{chatroom_id}"`
- `data.id` (string): UUID of deletion event (NOT the deleted message)
- `data.message.id` OR `data.deletedMessage.id` (string): UUID of deleted message - **PRIMARY IDENTIFIER**
- `data.deletedMessage.deleted_by` (int): User ID of moderator (optional)
- `data.deletedMessage.chatroom_id` (int): Chatroom ID

**Message ID Format:**
- UUID format: `"e4a2cc71-5e99-49e5-9112-2eafb30b414d"`
- Same format as original message IDs in `ChatMessageSentEvent`

**Implementation Notes:**
- Event arrives over existing WebSocket connection (no additional connections needed)
- Must track original message IDs to match deletions
- Structure may vary (two possible formats observed in community libraries)
- Event is pushed in real-time (sub-100ms latency)

**Confidence:** MEDIUM - Unofficial/reverse-engineered, but widely used in community libraries

**Limitations:**
- ⚠️ Unofficial event structure (Kick may change without notice)
- ⚠️ No official documentation
- ⚠️ Two conflicting structures observed (`message.id` vs `deletedMessage`)
- ⚠️ May break if Kick changes Pusher implementation

**Sources:**
- [KickLib C# Library](https://github.com/Bukk94/KickLib) (NuGet package with deletion support)
- [Kick Chat Wrapper (Go)](https://pkg.go.dev/github.com/johanvandegriff/kick-chat-wrapper)
- [Kick Alerts GitHub](https://github.com/Jake4-CX/Kick-Alerts) (mentions ChatMessageDeletedEvent structure)

---

### 4. TikTok (Unofficial WebSocket)

**Status:** ❌ **NOT SUPPORTED** - No deletion events available

**Library:** `tiktok-live-connector` (already in use via Node.js)

**Deletion Support:** **NONE**

**Available Events:**
- `chat` - Chat messages
- `member` - Members joining
- `gift` - Gifts
- `social` - Follows/shares
- `like` - Likes
- `roomUser` - Viewer count
- `emote` - Emote reactions
- `envelope` - Red envelope events
- `questionNew` - Q&A questions
- `linkMicBattle` - Battle events

**Message Deletion:** Not supported by library or TikTok's WebSocket API

**Rationale:**
- TikTok-Live-Connector is reverse-engineered (unofficial)
- No deletion events observed in community-maintained libraries
- TikTok's internal WebSocket API likely doesn't expose deletion events
- No official TikTok Live Chat API exists

**Confidence:** HIGH - Verified via library documentation and source code inspection

**Recommendations:**
1. Document limitation in user-facing documentation
2. Mark TikTok deletion support as "Not Available (Platform Limitation)"
3. Consider adding fallback: client-side timeout-based removal after X minutes
4. Monitor for official TikTok Live API release

**Sources:**
- [TikTok-Live-Connector GitHub](https://github.com/zerodytrash/TikTok-Live-Connector)
- [tiktok-live-connector npm package](https://www.npmjs.com/package/tiktok-live-connector)

---

## Implementation Strategy

### Phase 1: Twitch + YouTube (High Confidence)

**No new dependencies required.**

**Twitch Implementation:**
1. Add `OnClearMessage` and `OnClearChatMessage` handlers to `services/twitch-listener/irc/client.go`
2. Publish deletion events to Redis Streams (`chat:deletions` or reuse `chat:raw` with type flag)
3. Message Processor handles deletion events
4. API Gateway forwards to WebSocket clients
5. Frontend removes messages from overlay

**YouTube Implementation:**
1. Modify `services/youtube-listener/streams/poller.go` to check `snippet.type`
2. When `type == "messageDeletedEvent"`, extract `deletedMessageId`
3. Publish deletion event to Redis Streams
4. Same processing flow as Twitch

**Estimated Effort:** 2-3 days (both platforms)

### Phase 2: Kick (Medium Confidence)

**No new dependencies required.**

**Implementation:**
1. Add `ChatMessageDeletedEvent` handler to `services/kick-listener/websocket/client.go`
2. Handle both structure variations (`message.id` vs `deletedMessage`)
3. Publish deletion event to Redis Streams
4. Test with live Kick streams to verify structure

**Estimated Effort:** 1-2 days

**Risks:**
- Structure may differ from community documentation
- Kick may change event format without notice
- Requires production testing for validation

### Phase 3: TikTok (Not Feasible)

**Recommendation:** Skip or implement client-side fallback

**Options:**
1. **Document limitation** - "TikTok does not support message deletion"
2. **Client-side TTL** - Auto-remove messages after 5 minutes (configurable)
3. **Manual moderation** - Admin panel to hide specific messages
4. **Wait for official API** - Monitor TikTok developer announcements

---

## Redis Streams Message Format

### Unified Deletion Event Schema

Publish to existing `chat:raw` stream with new message type:

```json
{
  "message_id": "deletion_event_uuid",
  "platform": "twitch|youtube|kick|tiktok",
  "event_type": "deletion",  // NEW FIELD
  "deletion_type": "single|user|chat",  // single message, user ban, or full clear
  "channel_id": "channel_identifier",
  "timestamp": "2026-02-17T10:00:00Z",
  "deleted_message_id": "original_message_uuid",  // For single deletions
  "deleted_user_id": "user_id",  // For user bans/timeouts (optional)
  "deleted_username": "username",  // For user bans/timeouts (optional)
  "ban_duration": 600,  // Timeout seconds (0 = permanent, null = not applicable)
  "moderator_id": "mod_user_id",  // Who performed deletion (optional)
  "tags": {
    // Platform-specific metadata
  }
}
```

**Deletion Types:**
- `"single"` - One message deleted (Twitch CLEARMSG, YouTube messageDeletedEvent, Kick ChatMessageDeletedEvent)
- `"user"` - All messages from user (Twitch CLEARCHAT with target)
- `"chat"` - Full chat clear (Twitch CLEARCHAT without target)

---

## Alternatives Considered

### Alternative 1: Twitch EventSub Instead of IRC

**Why Not:**
- ❌ Requires webhook infrastructure (extra complexity)
- ❌ Would need to migrate entire Twitch integration
- ❌ IRC already working, tested, and reliable
- ✅ IRC CLEARMSG/CLEARCHAT fully supported in go-twitch-irc v4

**When to Consider:**
- If Twitch deprecates IRC (not announced)
- If webhook latency becomes acceptable
- If other EventSub features are needed

### Alternative 2: YouTube EventSub/WebSocket

**Why Not:**
- ❌ YouTube doesn't offer WebSocket for live chat
- ❌ Polling is official approach (documented in API)
- ❌ No EventSub equivalent for YouTube Live Chat

### Alternative 3: Official Kick API

**Why Not:**
- ❌ Kick has no official chat API
- ❌ All integrations are reverse-engineered via Pusher
- ✅ Pusher WebSocket is de facto standard

### Alternative 4: Wait for TikTok Official API

**Why Consider:**
- ✅ Would be production-ready and supported
- ✅ Would likely include deletion events
- ❌ No timeline or announcement from TikTok
- ❌ Current library is only option for real-time events

---

## Version Compatibility

| Package | Current Version | Deletion Support | Notes |
|---------|----------------|------------------|-------|
| `gempir/go-twitch-irc/v4` | v4.3.1 | ✅ Full | `OnClearMessage`, `OnClearChatMessage` |
| YouTube Data API v3 | v3 | ✅ Full | `messageDeletedEvent` type |
| `gorilla/websocket` | (in use) | ✅ Compatible | No changes needed |
| `tiktok-live-connector` | (in use) | ❌ None | No deletion events exposed |

**No version upgrades required for deletion support.**

---

## What NOT to Use

### ❌ Twitch PubSub API for Deletions

**Why Avoid:**
- Deprecated in favor of EventSub
- Requires WebSocket connection per channel (doesn't scale)
- IRC provides same functionality with better performance

### ❌ YouTube API v2 (Legacy)

**Why Avoid:**
- Deprecated since 2015
- No live chat support
- Use YouTube Data API v3

### ❌ Unofficial Kick REST API for Deletions

**Why Avoid:**
- No deletion event endpoints in REST API
- Pusher WebSocket is only real-time option
- REST API cannot provide immediate notification

### ❌ Screen Scraping / Browser Automation

**Why Avoid:**
- Extremely fragile (DOM changes break implementation)
- High latency (seconds vs milliseconds)
- Platform TOS violations
- Use official/unofficial APIs instead

---

## Production Considerations

### Message ID Storage

**Problem:** Need to match deletion events to original messages

**Solutions:**
1. **Redis Cache** (Recommended)
   - Store `message_id → overlay_id` mapping
   - TTL: 5-10 minutes (messages older than this rarely deleted)
   - Key format: `msg:{platform}:{message_id} → overlay_id`

2. **PostgreSQL** (Not Recommended)
   - Would require schema change (messages currently not persisted)
   - Adds database load
   - Slower than Redis

3. **Frontend Only** (Fallback)
   - Frontend tracks message IDs
   - Matches deletion events locally
   - Works even if backend cache expires

### Race Conditions

**Scenario:** Deletion event arrives before original message

**Mitigation:**
1. Buffer deletion events in Redis for 10 seconds
2. If matching message arrives late, apply deletion immediately
3. If no match after 10 seconds, discard (message never sent to overlay)

### API Gateway WebSocket Protocol

**New Message Type:** Add `deletion` event type

```json
{
  "type": "deletion",
  "payload": {
    "deletion_type": "single|user|chat",
    "deleted_message_id": "uuid",  // For single
    "deleted_user_id": "user_id"   // For user bans
  }
}
```

Frontend listens for `deletion` events and removes matching messages from DOM.

---

## Testing Strategy

### Unit Tests

**Twitch:**
- Mock IRC CLEARMSG/CLEARCHAT messages
- Verify callback registration and parsing
- Test `TargetMsgID` and `TargetUserID` extraction

**YouTube:**
- Mock API responses with `messageDeletedEvent`
- Verify `deletedMessageId` extraction
- Test polling flow with mixed message types

**Kick:**
- Mock Pusher WebSocket messages
- Test both structure variations
- Verify UUID parsing

### Integration Tests

**E2E Flow:**
1. Publish test message to overlay
2. Trigger deletion event (mock or real)
3. Verify deletion event published to Redis
4. Verify Message Processor handles deletion
5. Verify API Gateway sends deletion to WebSocket
6. Verify frontend removes message

### Production Validation

**Twitch:**
- Delete message via Twitch chat moderation
- Verify overlay updates immediately

**YouTube:**
- Delete message via YouTube Studio live dashboard
- Verify overlay updates within polling interval (2-5 seconds)

**Kick:**
- Delete message via Kick chat moderation
- Verify overlay updates immediately
- Confirm event structure matches documentation

**TikTok:**
- Document that deletion is not supported
- Test client-side TTL fallback if implemented

---

## Sources

### Official Documentation
- [Twitch IRC Commands](https://dev.twitch.tv/docs/irc/commands)
- [Twitch IRC Tags](https://dev.twitch.tv/docs/irc/tags/)
- [YouTube LiveChatMessages API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages)
- [YouTube LiveChatMessages.list Method](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/list)

### Library Documentation
- [go-twitch-irc v4 Package](https://pkg.go.dev/github.com/gempir/go-twitch-irc/v4)
- [go-twitch-irc GitHub Repository](https://github.com/gempir/go-twitch-irc)
- [TikTok-Live-Connector GitHub](https://github.com/zerodytrash/TikTok-Live-Connector)
- [tiktok-live-connector npm](https://www.npmjs.com/package/tiktok-live-connector)

### Community Resources (Kick)
- [KickLib C# Library](https://github.com/Bukk94/KickLib)
- [Kick Chat Wrapper (Go)](https://pkg.go.dev/github.com/johanvandegriff/kick-chat-wrapper)
- [Kick Alerts GitHub](https://github.com/Jake4-CX/Kick-Alerts)

---

**Research Date:** 2026-02-17
**Researcher:** Claude Code (GSD Project Researcher Agent)
**Overall Confidence:** HIGH (Twitch, YouTube), MEDIUM (Kick), HIGH (TikTok = Not Supported)
