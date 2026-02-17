# Project Research Summary

**Project:** Message Deletion Events for Chat Aggregation
**Domain:** Streaming chat overlay with moderation features
**Researched:** 2026-02-17
**Confidence:** HIGH

## Executive Summary

Message deletion is a standard moderation feature in streaming chat platforms, but implementing it in a multi-platform aggregation system like All-Chat requires careful handling of platform heterogeneity, message ID tracking, and distributed event ordering. The research reveals that three of four platforms (Twitch, YouTube, Kick) provide deletion events, though with vastly different mechanisms: Twitch delivers immediate IRC commands, YouTube requires polling-based detection with 60-second delays, Kick provides undocumented WebSocket events, and TikTok offers no deletion support at all.

The recommended approach is a phased implementation starting with Twitch (highest confidence, real-time events) using the existing go-twitch-irc v4 library, followed by YouTube (polling-based), then Kick (reverse-engineered), while documenting TikTok as unsupported. The critical architectural requirement is a message ID registry in Redis that maps platform-native message IDs to internal UUIDs, enabling deletion events to match previously published messages. This registry must be implemented before any deletion features, as the current architecture generates new UUIDs without preserving platform IDs.

The highest-risk pitfall is the race condition between message display and deletion events. Deletion events often arrive before original messages finish processing through the enrichment pipeline (emote API calls add 50-200ms latency). Without proper sequence handling or deletion buffering, messages can appear after their deletion event and persist indefinitely on overlays. Additional risks include batch deletion amplification (timing out a user with 5,000 messages), ephemeral architecture lookup failures (messages trimmed from Redis Streams before deletion arrives), and split-brain deletion across multiple WebSocket clients. All require mitigation strategies in Phase 1 foundation work.

## Key Findings

### Recommended Stack

**Summary:** No new dependencies required. All platforms except TikTok can be supported with existing libraries already in use by All-Chat.

**Core technologies:**
- `gempir/go-twitch-irc/v4` (v4.3.1): Twitch IRC client — already supports OnClearMessage and OnClearChatMessage callbacks for single and bulk deletions
- YouTube Data API v3: HTTP polling — messageDeletedEvent type included in standard polling responses, no additional quota cost
- `gorilla/websocket` (in use): Kick Pusher client — handles ChatMessageDeletedEvent over existing WebSocket connection
- Redis hashes with TTL: Message ID registry — map platform IDs to internal UUIDs (O(1) lookup, automatic expiration)

**Platform capabilities:**
- Twitch: Full support (CLEARMSG for single, CLEARCHAT for bulk, <100ms latency)
- YouTube: Full support (messageDeletedEvent via polling, 30-60s latency)
- Kick: Partial support (WebSocket events, undocumented/reverse-engineered)
- TikTok: No support (library does not expose deletion events)

### Expected Features

**Summary:** Users expect standard platform moderation features to work across aggregated chat. Differentiators come from cross-platform consistency and audit capabilities.

**Must have (table stakes):**
- Single message deletion — moderator deletes one message, it disappears from overlay
- User timeout batch deletion — platform times out user, all their messages removed
- User ban batch deletion — platform bans user permanently, all messages removed
- Visual feedback — 200ms fade animation (configurable to instant)
- Race condition handling — deletion events arriving before messages still work
- Already-scrolled message handling — graceful no-op for messages not in DOM

**Should have (competitive):**
- Cross-platform consistency — same deletion behavior across Twitch/YouTube/Kick
- Clear all chat — broadcast deletion (Twitch native, others via API)
- Scroll position preservation — calculate removed heights, adjust scroll offset
- System messages — optionally show "User123 timed out for 10 minutes"

**Defer (v2+):**
- Undo/ghost mode for moderators — strikethrough deleted messages, moderator-only
- Deletion audit log (30-day rolling) — track who deleted what and when in Redis
- Batch delete by keyword — remove all messages matching pattern
- Deletion animations (configurable) — fade/slide/instant removal options

### Architecture Approach

**Summary:** Reuse existing message pipeline (Listeners → Redis Streams → Message Processor → Redis Pub/Sub → API Gateway → Frontend) by adding event_type="deletion" alongside event_type="chat". Critical new component is Message ID Registry (Redis hash with 24h TTL) that maps platform message IDs to internal UUIDs and target overlay IDs for deletion matching.

**Major components:**
1. **Message ID Registry (NEW)** — Redis hash storing platform_msg_id → {internal_uuid, overlay_ids, timestamp} with 24h TTL for O(1) deletion lookup
2. **Platform Listeners (MODIFIED)** — Parse deletion events (CLEARMSG, messageDeletedEvent, ChatMessageDeletedEvent), lookup internal IDs, publish to chat:raw stream
3. **Message Processor (MODIFIED)** — Handle event_type="deletion", normalize platform formats, route to affected overlay Pub/Sub channels
4. **API Gateway (MODIFIED)** — Forward deletion events to WebSocket clients with {event: {type: "deletion", metadata: {target_message_id}}}
5. **Frontend Overlay (MODIFIED)** — Track messages by ID in DOM, remove on deletion event with optional fade animation

**Architectural patterns:**
- Unified event stream: Same Redis Stream (chat:raw) for messages and deletions, preserves ordering
- Event-driven propagation: Deletions follow existing pipeline, consistent <500ms latency
- Ephemeral ID mapping: Redis hash with TTL, no database schema changes
- Fire-and-forget delivery: Same best-effort model as message delivery

### Critical Pitfalls

1. **Message ID mismatch** — Platform deletion events reference platform-native IDs, but All-Chat generates new UUIDs. Without bidirectional ID mapping (platform_id ↔ internal_uuid), deletion events cannot match messages. Requires Message ID Registry in Phase 1 before any deletion features.

2. **Race condition (deletion before message)** — Deletion events bypass enrichment and arrive faster than original messages (50-200ms difference). Message displays after deletion event and never gets deleted. Mitigate with deletion buffer (frontend queues pending deletions for 200ms) OR route deletions through same enrichment pipeline to preserve ordering.

3. **Batch deletion amplification** — User timeout with 5,000 messages triggers 5,000 individual deletion events, overwhelming API Gateway CPU, network bandwidth, and frontend DOM operations. Requires coalesced deletion event format: {type: "bulk_delete", user_id: "123"} with frontend-side filtering from Phase 1.

4. **Platform-specific deletion gaps** — Twitch provides real-time IRC events (<100ms), YouTube requires polling (60s delay), Kick is undocumented/reverse-engineered, TikTok has no support. Must document capability matrix and implement platform-specific adapters (polling diff for YouTube in Phase 2).

5. **Ephemeral architecture lookup failure** — Deletion arrives after original message trimmed from Redis Streams (MAXLEN ~10000). Cannot determine target overlay_id. Requires short-term ID mapping cache (Redis hash, 15min TTL) to bridge the gap.

6. **Split-brain deletion across clients** — Redis Pub/Sub at-most-once delivery, slow subscribers miss events, creating inconsistent state across multiple overlay instances. Requires deletion event persistence (1min TTL) and reconnection replay logic in Phase 2.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Foundation (Twitch Only)
**Rationale:** Establish core deletion infrastructure with highest-confidence platform (Twitch IRC) before expanding to others. Twitch provides real-time events, official documentation, and full library support. This phase validates the architectural approach and catches ID tracking, race condition, and amplification issues early.

**Delivers:** End-to-end deletion flow (single message and bulk user deletions) working on Twitch streams with <500ms latency.

**Addresses:**
- Single message deletion (table stakes)
- User timeout/ban batch deletion (table stakes)
- Quick fade animation (table stakes)
- Race condition handling (table stakes)

**Implements:**
- Message ID Registry (Redis hash with 24h TTL)
- Twitch Listener: CLEARMSG and CLEARCHAT handlers
- Message Processor: Deletion event normalization and routing
- API Gateway: Deletion event forwarding to WebSocket
- Frontend: Message removal with fade animation
- Coalesced bulk deletion event format (prevents amplification)

**Avoids:**
- Message ID mismatch (registry maps platform IDs to internal UUIDs)
- Batch deletion amplification (single event for user timeouts/bans)
- Race condition (deletion buffer or enrichment routing)

**Estimated effort:** 3-5 days

**Research flag:** Standard patterns, skip research-phase (Twitch IRC well-documented)

---

### Phase 2: YouTube Integration
**Rationale:** Second-highest confidence platform with official API support. YouTube's polling-based approach (60s delay) differs from Twitch's real-time events, validating the architecture handles both push and pull models. Reuses Phase 1 infrastructure (Message ID Registry, Message Processor) with platform-specific adapter.

**Delivers:** YouTube message deletions working via polling with 30-60s latency (acceptable for moderation use case).

**Uses:**
- YouTube Data API v3 (already in use)
- Message ID Registry (from Phase 1)
- Message Processor deletion handler (from Phase 1)

**Implements:**
- YouTube Listener: Poll diff logic (compare message IDs between polls to detect deletions)
- Platform-specific normalization for messageDeletedEvent
- Testing: Deletion during live stream via YouTube Studio

**Addresses:**
- Platform-specific deletion gaps (mitigates with polling adapter)
- Cross-platform consistency (second platform validates unified event format)

**Estimated effort:** 2-3 days

**Research flag:** Standard patterns, skip research-phase (YouTube API official)

---

### Phase 3: Kick Integration + Edge Cases
**Rationale:** Lower confidence platform (undocumented/reverse-engineered) comes after core infrastructure proven with Twitch/YouTube. Validates handling of unofficial APIs. Also addresses advanced edge cases discovered during Phase 1-2.

**Delivers:** Kick message deletions working via WebSocket events, plus clear-all-chat, scroll preservation, and reconnection handling.

**Uses:**
- `gorilla/websocket` (already in use)
- Message ID Registry (from Phase 1)
- Platform capability matrix (from research)

**Implements:**
- Kick Listener: ChatMessageDeletedEvent handler (test both structure variations)
- Clear all chat support (Twitch native, others via custom implementation)
- Scroll position preservation (calculate removed message heights)
- Deletion event persistence (1min TTL in Redis)
- WebSocket reconnection replay logic

**Addresses:**
- Platform-specific gaps (Kick reverse-engineered, requires production testing)
- Split-brain deletion (persistence + replay)
- Clear all chat (should-have feature)
- Scroll preservation (should-have feature)

**Estimated effort:** 3-4 days

**Research flag:** Needs validation — Kick event structure is reverse-engineered, requires live testing

---

### Phase 4: Advanced Features (Optional)
**Rationale:** Competitive differentiators deferred until core deletion proven. These enhance value but are not table stakes.

**Delivers:** Moderator ghost mode, deletion audit log, system messages, configurable animations.

**Addresses:**
- Undo/ghost mode (differentiator, v2+ feature)
- Deletion audit log (differentiator, v2+ feature)
- System messages (should-have feature)
- Deletion animations configurable (should-have feature)

**Estimated effort:** 3-5 days

**Research flag:** Skip research-phase (UX features, implementation straightforward)

---

### Phase Ordering Rationale

- **Twitch first:** Highest confidence, real-time events, validates foundation before expanding
- **YouTube second:** Official API, different event model (polling vs push), reuses infrastructure
- **Kick third:** Unofficial/reverse-engineered, lower risk after core proven
- **TikTok deferred:** No deletion support, document limitation only
- **Advanced features last:** Competitive differentiators, not blocking MVP

**Dependency chain:**
- Message ID Registry (Phase 1) required for all deletion features
- Race condition mitigation (Phase 1) prevents production bugs
- Batch deletion handling (Phase 1) prevents performance collapse
- Platform adapters (Phase 2-3) isolate platform-specific complexity

**Pitfall prevention:**
- Phase 1 addresses ID mismatch, race conditions, amplification before scaling to other platforms
- Phase 2 validates polling-based model for platforms without real-time events
- Phase 3 handles edge cases (reconnection, split-brain) after core stable

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 3 (Kick):** Reverse-engineered WebSocket events — requires live testing to confirm structure, may differ from community documentation
- **Phase 4 (Advanced):** Audit log storage strategy — need to decide between Redis TTL vs database persistence based on requirements

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Twitch):** Official IRC protocol, well-documented in go-twitch-irc library and Twitch developer docs
- **Phase 2 (YouTube):** Official API with messageDeletedEvent documented in YouTube Data API v3 reference

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All required libraries already in use, no new dependencies, version compatibility confirmed |
| Features | HIGH | Table stakes identified from official platform documentation, MVP scope clear |
| Architecture | HIGH | Reuses existing pipeline with minimal modifications, Message ID Registry pattern proven in similar systems |
| Pitfalls | HIGH | Critical pitfalls (ID mismatch, race conditions, amplification) identified from distributed systems research and platform API limitations |

**Overall confidence:** HIGH

### Gaps to Address

**Platform capability uncertainty:**
- **Kick event structure:** Two conflicting formats observed in community libraries (message.id vs deletedMessage). Resolve during Phase 3 by capturing real deletion event from Kick stream in browser DevTools.
- **TikTok support timeline:** No official API announced. Monitor for updates, document as unsupported in v1.

**Performance tuning:**
- **Message ID Registry memory:** At 10k msg/s, 24h retention = ~86GB. May need to reduce TTL to 6 hours (sufficient for 99% of deletions) or shard across Redis instances. Defer optimization until load testing in Phase 3.
- **Batch deletion threshold:** Unknown at what message count (100? 1000? 10000?) frontend DOM operations become blocking. Requires load testing in Phase 1 to establish limits.

**Race condition mitigation choice:**
- **Buffer vs routing:** Two approaches identified (frontend deletion buffer with 200ms delay OR route deletions through enrichment pipeline). Recommend testing both in Phase 1 to measure latency/complexity tradeoffs before committing.

**YouTube quota impact:**
- **Deletion detection polling:** Diff-based polling adds no quota cost (same requests as message polling), but increased poll frequency (30s instead of 60s) would double quota consumption. Validate quota headroom before adjusting polling interval.

## Sources

### Primary (HIGH confidence)
- [Twitch IRC Commands](https://dev.twitch.tv/docs/irc/commands) — CLEARMSG and CLEARCHAT message specifications
- [Twitch IRC Tags](https://dev.twitch.tv/docs/irc/tags) — target-msg-id and ban-duration tag formats
- [go-twitch-irc v4 Package](https://pkg.go.dev/github.com/gempir/go-twitch-irc/v4) — OnClearMessage and OnClearChatMessage callback documentation
- [YouTube LiveChatMessages API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages) — messageDeletedEvent response structure
- [YouTube LiveChatMessages.list Method](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/list) — polling endpoint specification

### Secondary (MEDIUM confidence)
- [KickLib C# Library](https://github.com/Bukk94/KickLib) — ChatMessageDeletedEvent structure from reverse-engineering
- [Kick Chat Wrapper (Go)](https://pkg.go.dev/github.com/johanvandegriff/kick-chat-wrapper) — Community implementation of Kick Pusher events
- [TikTok-Live-Connector GitHub](https://github.com/zerodytrash/TikTok-Live-Connector) — Confirms no deletion event support in library
- [Causal Consistency in Distributed Systems](https://systemd.imshawan.dev/1-Fundamentals/1.5-Consistency-Models-Strong-Eventual-Causal/4-Causal-Consistency/) — Message ordering guarantees for race condition handling
- [Broadcasting - websockets documentation](https://websockets.readthedocs.io/en/stable/topics/broadcast.html) — Efficient WebSocket broadcast patterns

### Tertiary (LOW confidence)
- [Kick Alerts GitHub](https://github.com/Jake4-CX/Kick-Alerts) — Mentions ChatMessageDeletedEvent structure (needs validation)
- [Twitch Developer Forum: Message Deletion Confusion](https://discuss.dev.twitch.com/t/message-deletion-confusion/19311) — Community discussion on CLEARMSG edge cases

### Internal (All-Chat codebase)
- `/services/message-processor/models/message.go` — Current UUID generation without platform ID preservation
- `/docs/architecture/01-DATA-FLOW.md` — Existing message pipeline architecture
- `/docs/adr/0002-redis-streams-pubsub.md` — Hybrid messaging architecture rationale

---
*Research completed: 2026-02-17*
*Ready for roadmap: yes*
