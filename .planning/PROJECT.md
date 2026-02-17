# All-Chat: Message Deletion Support

## What This Is

All-Chat is a cloud-native platform that aggregates live chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and displays them in unified overlays for streamers. This milestone adds support for honoring message deletion events from platforms, ensuring overlays accurately reflect the current state of chat including when messages are removed by moderators.

## Core Value

When a message is deleted on the streaming platform, it must be removed from connected overlays in real-time so streamers see an accurate representation of chat.

## Requirements

### Validated

<!-- Existing capabilities from the codebase -->

- ✓ Multi-platform chat aggregation (Twitch, YouTube, Kick, TikTok) — existing
- ✓ Real-time message delivery via WebSocket to overlays — existing
- ✓ Message normalization across platforms (unified schema) — existing
- ✓ Emote enrichment (7TV, BTTV, FFZ, platform-native) — existing
- ✓ Overlay configuration with multi-source support — existing
- ✓ OAuth authentication for platform access — existing
- ✓ Redis Streams for durable message queuing — existing
- ✓ Redis Pub/Sub for real-time broadcast to overlays — existing
- ✓ Microservices architecture (Standard Go Layout) — existing
- ✓ Kubernetes-deployable with health checks and graceful shutdown — existing

### Active

<!-- Current milestone scope -->

- [ ] Detect single message deletion events from platforms
- [ ] Detect user message batch deletion events (timeout/ban)
- [ ] Detect full chat clear events
- [ ] Propagate deletion events through message pipeline (Listener → Processor → Gateway)
- [ ] Remove deleted messages from connected overlays in real-time
- [ ] Track message IDs to match deletion events to original messages
- [ ] Support deletion events for Twitch (CLEARMSG, CLEARCHAT commands)
- [ ] Support deletion events for YouTube (redacted messages)
- [ ] Support deletion events for Kick (if available)
- [ ] Support deletion events for TikTok (if available)

### Out of Scope

- Message editing — Platforms don't universally support this; defer
- Moderation context display — Focus only on removal, not reason/metadata
- Historical message deletion — Messages are ephemeral; no database cleanup needed
- Deletion undo/restoration — Not supported by platforms
- Deletion analytics/logging — Core feature only, metrics can be added later

## Context

**Platform Deletion Event Research:**
- Need to investigate what deletion event formats each platform provides
- Platforms may provide different identifiers (message ID, user ID, timestamp)
- Event detection varies by protocol: IRC commands (Twitch), HTTP polling (YouTube), WebSocket (Kick), unofficial library (TikTok)

**Current Message Flow:**
1. Platform → Listener (IRC/HTTP/WebSocket)
2. Listener → Redis Streams (`stream:raw-messages`)
3. Message Processor consumes, normalizes, enriches, routes
4. Message Processor → Redis Pub/Sub per overlay (`overlay:{id}`)
5. API Gateway subscribes, broadcasts via WebSocket
6. Frontend overlay displays messages

**Message Lifecycle:**
- Messages are ephemeral (Redis only, ~1 day retention in Streams)
- No PostgreSQL persistence for chat messages
- Only connected overlays receive messages (real-time only)
- Currently only "add message" events are supported

**Technical Environment:**
- Go 1.25.6 microservices with Standard Go Layout
- Redis 7 (Streams for queuing, Pub/Sub for broadcast)
- Gin web framework, gorilla/websocket
- Platform clients: go-twitch-irc (Twitch), YouTube API v3 (YouTube), WebSocket (Kick), tiktok-live-connector (TikTok)

## Constraints

- **Tech Stack**: Must use Go 1.25.6, existing microservices, Redis 7 — No architectural changes
- **Real-time Only**: No database storage of messages — Deletion only affects in-memory/Redis state
- **Platform APIs**: Limited to what platforms expose — Some platforms may not provide deletion events
- **Backward Compatibility**: Must not break existing message flow — Deletion is additive feature
- **Message ID Tracking**: Need to add ID tracking if not already present — May require changes to message schema

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Research deletion formats first | Each platform provides different event structures and identifiers | — Pending |
| Additive feature | Existing message flow continues unchanged; deletion adds parallel event handling | — Pending |
| Remove completely from overlay | User requested immediate removal without placeholder or fade | — Pending |

---
*Last updated: 2026-02-17 after initialization*
