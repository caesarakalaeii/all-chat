# Feature Research: Chat Message Deletion

**Domain:** Streaming Chat Overlay Message Deletion
**Researched:** 2026-02-17
**Confidence:** HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Single message deletion | Standard moderation tool on all platforms | MEDIUM | Requires message ID tracking, race condition handling for scrolled-off messages |
| User timeout (batch delete) | Core moderation feature - removes all messages from timed-out user | MEDIUM | Twitch: 10min-24h, YouTube: 10s-24h, Kick: configurable minutes |
| User ban (batch delete) | Permanent removal of user's messages | MEDIUM | Similar to timeout but permanent ban status |
| Clear all chat | Nuclear option for chat emergencies | LOW | Simple broadcast, no message targeting needed |
| Visual feedback | User sees message disappear instantly | LOW | DOM removal, fade animation optional |
| Already-scrolled messages | Handle deletion of messages no longer visible | LOW | Graceful no-op if message not in DOM |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Cross-platform consistency | Same deletion behavior across Twitch/YouTube/Kick/TikTok | MEDIUM | Normalizes platform-specific events into unified format |
| Undo deletion (ghost mode) | Moderators see deleted messages with strikethrough | HIGH | Requires state persistence, moderator-only overlay mode |
| Deletion audit log | Track who deleted what and when | MEDIUM | Useful for multi-mod teams, requires database storage |
| Smart scroll preservation | Keep scroll position when messages removed | LOW | Calculate offset change, adjust scroll |
| Deletion animations | Fade out, slide out, or instant removal configurable | LOW | CSS transitions, user preference |
| Batch delete by keyword | Remove all messages containing specific text | HIGH | Pattern matching across ephemeral Redis messages, potential performance impact |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Store all deletions in database | "Need full history for appeals" | Massive storage cost for ephemeral data, privacy concerns | 30-day rolling audit log in Redis with TTL |
| Real-time deletion sync across all overlays | "Need instant consistency" | Race conditions with multiple WebSocket connections, message ordering issues | Best-effort deletion with 100-500ms acceptable latency |
| Undo for viewers | "Accidental deletions should be reversible" | Creates inconsistent state, violates moderation intent | Only mods see deleted messages (ghost mode) |
| Regex-based auto-deletion | "Want to auto-delete patterns" | Complex regex can freeze UI, False positives harm UX | Use platform native filters (Twitch AutoMod, Kick AI moderation) |

## Feature Dependencies

```
[Single Message Deletion]
    └──requires──> [Message ID Tracking in Frontend]
                       └──requires──> [Unified Message Format]

[User Timeout/Ban] ──requires──> [User ID Tracking]
                                    └──requires──> [Platform User ID Mapping]

[Clear All Chat] ──independent──> (no dependencies)

[Deletion Audit Log] ──enhances──> [Single Message Deletion]
                                    └──requires──> [Database Schema]

[Undo/Ghost Mode] ──enhances──> [All Deletion Types]
                                 └──requires──> [Moderator Auth Context]

[Cross-Platform Consistency] ──requires──> [All Deletion Types]
                                           └──requires──> [Platform Event Normalization]
```

### Dependency Notes

- **Single Message Deletion requires Message ID Tracking:** Frontend must store message IDs in DOM for targeted removal
- **User Timeout/Ban requires User ID Mapping:** Platform-specific user IDs must be consistent across multiple messages
- **Deletion Audit Log enhances Single Message Deletion:** Optional feature that adds traceability without blocking core functionality
- **Undo/Ghost Mode enhances All Deletion Types:** Requires authentication context to distinguish moderators from viewers

## Expected Behaviors by Deletion Type

### 1. Single Message Deletion

**Platform Sources:**
- **Twitch IRC:** `CLEARMSG` command with `target-msg-id` tag ([Twitch IRC Docs](https://dev.twitch.tv/docs/chat/irc/))
- **Twitch EventSub:** `channel.chat.message_delete` subscription type ([Twitch Developers](https://dev.twitch.tv/docs/chat/irc-migration/))
- **YouTube API:** `liveChatMessages.delete` endpoint ([YouTube API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete))
- **Kick:** Message deletion events via Pusher WebSocket ([Kick Help Center](https://help.kick.com/en/articles/10162074-moderation-features-guide))
- **TikTok:** No official API, likely via event stream in unofficial library

**Expected Behavior:**
1. Moderator deletes single message on platform
2. Platform sends deletion event with message ID
3. Listener receives event → publishes to Redis Streams
4. Message Processor normalizes event → publishes to Redis Pub/Sub `overlay:{id}`
5. API Gateway broadcasts to connected WebSocket clients
6. Frontend receives deletion event:
   - Finds message in DOM by `data-message-id` attribute
   - Removes element (instant) OR fades out over 200ms
   - If message not found (scrolled off): gracefully ignore
7. Total latency: 100-500ms (acceptable for moderation)

**Edge Cases:**
- **Message already scrolled off screen:** No-op, log debug message
- **Message received after deletion event (race condition):** Store deletions in 60-second buffer, check on message render
- **Duplicate deletion events:** Idempotent - second deletion is no-op
- **Deletion during disconnect:** On reconnect, don't replay deleted messages (requires deletion event persistence)

**Complexity Factors:**
- Need stable message IDs from platform (Twitch provides, YouTube provides, others may need generation)
- Race condition handling requires temporary deletion buffer
- Cross-platform event format normalization

---

### 2. User Timeout (Batch Deletion)

**Platform Sources:**
- **Twitch IRC:** `CLEARCHAT` with `ban-duration` tag ([Twitch Mod Commands](https://www.streamscheme.com/twitch-moderator-chat-commands/))
- **YouTube API:** `liveChatBans.insert` with `duration` ([YouTube Bans API](https://developers.google.com/youtube/v3/live/docs/liveChatBans))
- **Kick:** `/timeout <username> <minutes>` command ([Kick Chat Commands](https://help.kick.com/en/articles/7112979-kick-chat-commands))
- **TikTok:** Mute/timeout via LIVE Studio moderators ([TikTok LIVE Studio](https://www.tiktok.com/live/studio/help/article/Boost-viewer-engagement/Add-moderators-to-manage-LIVE-chat-comments?lang=en))

**Expected Behavior:**
1. Moderator times out user for duration (e.g., 10 minutes)
2. Platform sends timeout event with user ID and duration
3. Listener receives event → publishes to Redis Streams
4. Message Processor:
   - Normalizes event
   - Publishes deletion command: `{ type: "user_timeout", user_id: "xyz", duration: 600 }`
5. Frontend receives timeout event:
   - Queries DOM for all messages with `data-user-id="xyz"`
   - Removes all matching messages (batch operation)
   - Optionally displays system message: "User123 timed out for 10 minutes"
6. During timeout period, don't render new messages from user

**Edge Cases:**
- **Timeout during message burst:** Some messages may arrive after timeout event (deletion buffer catches these)
- **Multiple overlays, different channels:** User timeout on Twitch channel A shouldn't affect their messages on YouTube channel B
- **Timeout cancellation:** Twitch/Kick allow manual untimeout - send "untimeout" event to re-enable user
- **Timeout expiration:** After duration, allow user messages again (time-based check on frontend)

**Complexity Factors:**
- Requires tracking all message elements by user ID
- Temporal logic: disable user rendering during timeout window
- Platform differences: Twitch default 10min, YouTube 10s-24h, Kick variable

---

### 3. User Ban (Permanent Batch Deletion)

**Platform Sources:**
- **Twitch IRC:** `CLEARCHAT` without `ban-duration` tag ([Twitch IRC](https://dev.twitch.tv/docs/chat/irc/))
- **YouTube API:** `liveChatBans.insert` without `duration` ([YouTube Bans API](https://developers.google.com/youtube/v3/live/docs/liveChatBans))
- **Kick:** `/ban <username>` command ([Kick Moderation](https://help.kick.com/en/articles/10162074-moderation-features-guide))
- **TikTok:** Block user feature ([TikTok Creator Tools](https://newsroom.tiktok.com/en-us/new-tools-for-creators))

**Expected Behavior:**
1. Moderator permanently bans user
2. Platform sends ban event with user ID (no duration)
3. Listener receives event → publishes to Redis Streams
4. Message Processor:
   - Normalizes event
   - Publishes deletion command: `{ type: "user_ban", user_id: "xyz" }`
5. Frontend receives ban event:
   - Queries DOM for all messages with `data-user-id="xyz"`
   - Removes all matching messages
   - Adds user ID to permanent ban list (in-memory Set)
   - Future messages from banned user are dropped before rendering
6. System message optional: "User123 has been banned"

**Edge Cases:**
- **Unban:** Platform sends unban event, remove from ban list
- **Cross-channel bans:** User banned on Twitch channel A can still appear from YouTube channel B (different identity)
- **Shared username across platforms:** Handle per-platform user IDs, not display names

**Complexity Factors:**
- Permanent in-memory ban list per overlay (doesn't persist across page refresh - OK for ephemeral)
- Similar to timeout but no expiration logic

---

### 4. Clear All Chat

**Platform Sources:**
- **Twitch:** `/clear` command ([Twitch Mod Guide](https://www.hollyland.com/blog/tips/remove-messages-and-manage-chat))
- **YouTube:** Not a native feature (would need custom implementation)
- **Kick:** Not documented as native feature
- **TikTok:** Not documented as native feature

**Expected Behavior:**
1. Moderator issues clear chat command (Twitch only native)
2. Platform sends `CLEARCHAT` event without user ID or ban-duration
3. Listener receives event → publishes to Redis Streams
4. Message Processor:
   - Normalizes event
   - Publishes deletion command: `{ type: "clear_all" }`
5. Frontend receives clear_all event:
   - Removes all chat message elements from DOM
   - Clears any cached message state
   - Optionally displays system message: "Chat has been cleared by moderator"
6. No impact on future messages

**Edge Cases:**
- **Messages in flight:** Messages sent during clear may appear after (acceptable - not a strong consistency requirement)
- **Multiple overlays:** Each overlay clears independently
- **Non-Twitch platforms:** May need custom broadcaster command or API call

**Complexity Factors:**
- Simplest deletion type (no targeting)
- Only Twitch has native support via IRC
- Other platforms may need API-based implementation or skip feature

---

## UX Considerations

### Immediate Removal vs. Fade Animation

**Options:**
1. **Instant removal** (0ms): Jarring but clear
2. **Quick fade** (200ms): Smooth, still responsive
3. **Slide out** (300ms): Animated but may feel slow
4. **Strikethrough + fade** (500ms): Clear indication before removal

**Recommendation:** Quick fade (200ms) by default, with config option for instant removal

**Rationale:**
- 200ms is perceptually instant but smoother than 0ms
- Moderators prefer clear feedback over subtle transitions
- Streamers may want instant for high-volume chat

---

### Race Condition Handling

**Scenario:** Deletion event arrives before message appears in overlay

**Strategies:**
1. **Deletion buffer (60s TTL):** Store recent deletion events in memory
   - On message render, check if message ID is in deletion buffer
   - If yes, skip rendering
   - Expire deletions after 60 seconds (messages older than 60s are scrolled off anyway)

2. **Backend deduplication:** Message Processor checks Redis for recent deletions before publishing
   - Adds latency (~5-10ms)
   - Reduces frontend complexity
   - Better for multi-client scenarios

**Recommendation:** Backend deduplication in Message Processor

**Implementation:**
```
Redis Key: deletions:recent:{overlay_id}
Type: Sorted Set
Members: message IDs
Scores: deletion timestamp
TTL: 60 seconds

On message processing:
1. Check if message ID exists in deletions:recent:{overlay_id}
2. If yes, drop message
3. If no, publish to overlay
4. Atomic operation ensures consistency
```

---

### Scroll Position Preservation

**Problem:** Removing messages shifts content, disrupting user reading flow

**Solution:** Calculate removed message heights and adjust scroll offset

**Algorithm:**
```javascript
function deleteMessage(messageId) {
  const element = document.querySelector(`[data-message-id="${messageId}"]`);
  if (!element) return; // Already scrolled off

  const container = element.parentElement;
  const wasScrolledToBottom = (
    container.scrollHeight - container.scrollTop - container.clientHeight < 50
  );

  const messageHeight = element.offsetHeight;

  element.remove();

  // If user was at bottom, stay at bottom
  if (wasScrolledToBottom) {
    container.scrollTop = container.scrollHeight;
  } else {
    // Adjust scroll to preserve position
    container.scrollTop -= messageHeight;
  }
}
```

---

## MVP Recommendation

### Launch With (v1)

Minimum viable product — what's needed to validate the concept.

- [x] **Single message deletion** — Core moderation tool, expected by all users
- [x] **User timeout batch deletion** — Standard moderation action across all platforms
- [x] **User ban batch deletion** — Permanent removal, standard moderation
- [x] **Quick fade animation (200ms)** — Smooth feedback without delay
- [x] **Race condition handling (deletion buffer)** — Prevents deleted messages from appearing
- [x] **Already-scrolled message handling** — Graceful no-op for missing messages

**Implementation Priority:**
1. Backend: Normalize deletion events in Message Processor
2. Backend: Redis deletion buffer (sorted set, 60s TTL)
3. Frontend: Message ID tracking in DOM
4. Frontend: Deletion event handlers (single, timeout, ban)
5. Frontend: Fade animation CSS transitions

---

### Add After Validation (v1.x)

Features to add once core is working.

- [ ] **Clear all chat** — Trigger: Broadcaster request (Twitch native, others via API)
- [ ] **Deletion animations configurable** — Trigger: User feedback prefers instant or slower fade
- [ ] **Scroll position preservation** — Trigger: Users complain about "jumping" chat
- [ ] **System message for timeouts/bans** — Trigger: Transparency request from streamers

---

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] **Undo/Ghost mode for moderators** — Trigger: Multi-mod teams need accountability
- [ ] **Deletion audit log (30-day)** — Trigger: Moderation transparency requirements
- [ ] **Batch delete by keyword** — Trigger: Spam attack scenarios
- [ ] **Strikethrough animation** — Trigger: A/B test shows better moderator confidence

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Single message deletion | HIGH | MEDIUM | P1 |
| User timeout batch deletion | HIGH | MEDIUM | P1 |
| User ban batch deletion | HIGH | MEDIUM | P1 |
| Quick fade animation | MEDIUM | LOW | P1 |
| Race condition handling | HIGH | MEDIUM | P1 |
| Clear all chat | MEDIUM | LOW | P2 |
| Scroll position preservation | MEDIUM | LOW | P2 |
| Deletion animations (config) | LOW | LOW | P2 |
| System messages | LOW | LOW | P2 |
| Undo/Ghost mode | MEDIUM | HIGH | P3 |
| Deletion audit log | MEDIUM | MEDIUM | P3 |
| Batch delete by keyword | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (MVP)
- P2: Should have, add when possible
- P3: Nice to have, future consideration

---

## Platform Behavior Comparison

| Feature | Twitch | YouTube | Kick | TikTok |
|---------|--------|---------|------|--------|
| **Single message delete** | ✅ CLEARMSG via IRC | ✅ API delete | ✅ Via WebSocket | ⚠️ Unofficial lib |
| **Timeout duration** | 1s - 14 days (default 10min) | 10s - 24 hours | Custom minutes | Not documented |
| **Permanent ban** | ✅ /ban command | ✅ API ban | ✅ /ban command | ✅ Block user |
| **Clear all chat** | ✅ /clear command | ❌ Not native | ❌ Not native | ❌ Not native |
| **Deletion events** | ✅ Real-time IRC | ✅ Polling required | ✅ WebSocket | ⚠️ Library-dependent |
| **Untimeout/Unban** | ✅ /untimeout, /unban | ✅ API delete ban | ✅ Commands | ⚠️ Unknown |

**Legend:**
- ✅ Natively supported
- ⚠️ Limited or unofficial support
- ❌ Not available

---

## Technical Constraints

### Redis Ephemeral Architecture

**Current State:** Messages exist only in Redis (no database)

**Implications:**
- Deletion events must be processed immediately (no replay from DB)
- Deletion buffer in Redis with TTL handles race conditions
- Audit logs require separate storage or TTL-based retention

**Tradeoff:** Fast, scalable ephemeral chat vs. no historical replay

---

### WebSocket Message Ordering

**Challenge:** Deletion event may arrive via different Redis Pub/Sub subscription than original message

**Mitigation:**
- Single Redis Pub/Sub channel per overlay ensures ordering
- Message Processor publishes all events to same channel: `overlay:{overlay_id}`
- Sequential processing in API Gateway before WebSocket broadcast

**Guarantee:** FIFO ordering within single overlay channel

---

### Multi-Overlay Scenarios

**Complexity:** User may appear in multiple overlays from different channels

**Example:**
- Overlay A: Twitch #shroud + YouTube @shroud
- Overlay B: Twitch #ninja + Kick @shroud

User "shroud" banned on Twitch #shroud should only affect Overlay A, not Overlay B

**Solution:** Deletion events include platform + channel_id + user_id triple, not just user_id

---

## What NOT to Build (Anti-Features)

### 1. Real-Time Sync Across Multiple Tabs

**Why Avoid:** Adds complexity without user value. Each overlay tab is independent, no need for cross-tab coordination.

**Alternative:** Each tab maintains its own state, receives same Redis Pub/Sub events.

---

### 2. Persistent Deletion History in Database

**Why Avoid:**
- Massive storage cost for ephemeral data (messages already not stored)
- Privacy concerns (deleted messages should be forgotten)
- No replay needed (overlays are live-only)

**Alternative:** 30-day audit log in Redis with TTL for moderation accountability (optional feature)

---

### 3. Deletion Confirmation Modal

**Why Avoid:** Slows down moderation, creates friction

**Alternative:** Instant deletion with optional undo window (5 seconds) for moderators

---

### 4. AI-Based Auto-Deletion

**Why Avoid:**
- False positives harm UX
- Requires ML model training/hosting
- Platforms already provide native filters (Twitch AutoMod, Kick AI moderation)

**Alternative:** Recommend users enable platform native moderation tools

---

## Sources

### Platform Documentation (HIGH Confidence)
- [Twitch IRC Concepts](https://dev.twitch.tv/docs/chat/irc/)
- [Twitch IRC Migration Guide](https://dev.twitch.tv/docs/chat/irc-migration/)
- [Twitch Mod Commands List](https://www.streamscheme.com/twitch-moderator-chat-commands/)
- [YouTube Live Chat Moderation](https://support.google.com/youtube/answer/9826490?hl=en)
- [YouTube LiveChatBans API](https://developers.google.com/youtube/v3/live/docs/liveChatBans)
- [YouTube LiveChatMessages Delete API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/delete)
- [Kick Moderation Features Guide](https://help.kick.com/en/articles/10162074-moderation-features-guide)
- [Kick Chat Commands](https://help.kick.com/en/articles/7112979-kick-chat-commands)
- [TikTok LIVE Studio Moderators](https://www.tiktok.com/live/studio/help/article/Boost-viewer-engagement/Add-moderators-to-manage-LIVE-chat-comments?lang=en)
- [TikTok Creator Tools Announcement](https://newsroom.tiktok.com/en-us/new-tools-for-creators)

### Community/Technical (MEDIUM Confidence)
- [Twitch Developer Forums - Message Deletion](https://discuss.dev.twitch.com/t/message-deletion-confusion/19311)
- [Twitch Developer Forums - CLEARMSG Support](https://github.com/tmijs/tmi.js/issues/317)
- [Twitch Mod Guide - Remove Messages](https://www.hollyland.com/blog/tips/remove-messages-and-manage-chat)
- [Chat Overlay Widget Documentation](https://docs.vdo.ninja/updates/updates-social-stream-and-chat-overlay)

---

*Feature research for: Chat Message Deletion in Streaming Overlays*
*Researched: 2026-02-17*
*Confidence: HIGH (official platform documentation + community implementation patterns)*
