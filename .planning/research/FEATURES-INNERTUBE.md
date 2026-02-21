# Feature Landscape: InnerTube YouTube Listener

**Domain:** YouTube Live Chat Listener (Quota-Free Alternative)
**Researched:** 2026-02-21
**Confidence:** HIGH

## Table Stakes

Features users expect. Missing = product feels incomplete or broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Live chat ingestion** | Core purpose - must receive messages in real-time | Medium | masterchat provides iterator API, must handle continuous polling |
| **Message normalization** | Must output same format as official youtube-listener for drop-in replacement | Low | Map masterchat fields to RawChatMessage schema |
| **Super Chat detection** | YouTube users expect super chats to be highlighted | Low | masterchat exposes `addSuperChatItemAction` with amount |
| **Membership events** | YouTube memberships (subscriptions) are key engagement events | Low | masterchat provides `addMembershipItemAction` + milestones |
| **Health checks** | K8s readiness/liveness probes required for production | Low | HTTP endpoints `/health/live` and `/health/ready` |
| **Graceful shutdown** | Must not lose messages on pod restart/scale-down | Medium | Clean up masterchat connections, flush Redis buffer |
| **Stream discovery** | Must detect when channels go live | High | **CRITICAL GAP**: masterchat requires video ID, need channel monitoring |
| **Reconnection on error** | Network failures, stream restarts common | Medium | masterchat emits error events, need retry logic |

## Differentiators

Features that set InnerTube listener apart from official API listener. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **No quota limits** | Unlimited polling, no daily caps (official API = 1M units/day) | N/A | Built-in advantage of InnerTube |
| **No OAuth required** | Simpler setup, no token refresh | N/A | Built-in advantage of InnerTube |
| **Message deletion events** | Can detect when moderators delete messages (official API doesn't expose) | Low | masterchat `markChatItemAsDeletedAction` available |
| **Ban/timeout detection** | Detect when users are banned/timed out | Low | masterchat `markChatItemsByAuthorAsDeletedAction` |
| **Lower latency** | InnerTube polling can be more frequent than quota-constrained official API | Low | Configure polling interval freely |
| **Sticker support** | Super stickers distinct from super chats | Low | masterchat `addSuperStickerItemAction` |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Quota tracking database** | InnerTube doesn't use official API, no quotas | Remove quota_usage table, tracker, all quota logic |
| **OAuth token storage** | InnerTube works unauthenticated | Remove youtube_oauth_tokens table, refresh logic |
| **Per-channel user credentials** | InnerTube doesn't need per-user auth | Single service credentials (if any) |
| **Adaptive polling slowdown** | No quota pressure to slow down | Use constant polling interval (2-5 seconds) |
| **Quota state machine** | (HEALTHY/DEGRADED/CRITICAL/EXHAUSTED states) | Simple binary: running or stopped |
| **Cross-service quota coordination** | overlay-manager doesn't need to track InnerTube quota | Remove `/quota/record` endpoint, YouTubeQuotaClient |
| **Stream caching for quota savings** | Caching optimized for quota efficiency, not needed | Simpler stream state management |

## Feature Dependencies

```
Stream Discovery → Live Chat Ingestion → Message Normalization → Redis Publishing
                ↓
        Reconnection Handling
                ↓
        Graceful Shutdown
```

**Critical Path:** Stream Discovery is prerequisite for all else. Without channel → video ID resolution, can't initialize masterchat.

## MVP Recommendation

### Phase 1: Proof of Concept (Must Have)

1. **Manual video ID input** - Bypass stream discovery, hardcode video ID for testing
2. **Chat ingestion** - masterchat iterator consuming live chat
3. **Message normalization** - Map to RawChatMessage format
4. **Redis Streams publishing** - XADD to `chat:raw`
5. **Basic health checks** - HTTP 200 on `/health/live` and `/health/ready`

**Deferred:** Stream discovery, OAuth, reconnection, graceful shutdown

**Validation:** Can receive live chat from known stream, message-processor can consume and normalize.

### Phase 2: Production Minimum (Should Have)

1. **Stream discovery via external API** - overlay-manager resolves channel → video ID, passes to innertube-listener
2. **Reconnection logic** - Retry on masterchat error events (exponential backoff)
3. **Graceful shutdown** - SIGTERM handler, flush Redis, close connections
4. **Status endpoint** - `/status` showing active streams, message rate
5. **Structured logging** - Winston with JSON output (matches Go services)

**Deferred:** Advanced error handling, metrics, deletion events

**Validation:** Can run in K8s with HPA, survives pod restarts, integrates with overlay-manager.

### Phase 3: Feature Parity (Nice to Have)

1. **Deletion event publishing** - Publish `markChatItemAsDeletedAction` to Redis Pub/Sub (not chat:raw)
2. **Ban/timeout events** - Detect and publish moderation actions
3. **Prometheus metrics** - Message rate, active streams, error count
4. **Super sticker enrichment** - Full sticker metadata in event_data
5. **Membership milestone parsing** - Extract months/years from milestone message

**Validation:** Feature-complete alternative to official listener with moderation event support.

## Feature Comparison: Official vs InnerTube

| Feature | Official API Listener | InnerTube Listener | Winner |
|---------|----------------------|-------------------|--------|
| **Quota limits** | 1M units/day (hard cap) | None | InnerTube |
| **OAuth required** | Yes (per-user tokens) | No | InnerTube |
| **Setup complexity** | High (Google Cloud Console, OAuth flow) | Low (just video ID) | InnerTube |
| **Reliability** | Official, supported | Unofficial, breaking change risk | Official |
| **Message types** | Chat, super chat, membership | Chat, super chat, membership, deletions, bans | InnerTube |
| **Latency** | 2-5 sec (API-dictated) | 1-5 sec (configurable) | Tie |
| **Legal status** | Terms of Service compliant | Gray area (web scraping) | Official |
| **Maintenance burden** | Quota tracking, token refresh | InnerTube protocol changes | Tie |

**Use Case Fit:**
- **Official Listener:** Production use by streamers within quota, requires official support
- **InnerTube Listener:** Self-hosters, high-volume channels, quota-exceeded scenarios

## Feature Gaps to Address

### Critical Gap: Stream Discovery

**Problem:** masterchat requires video ID at initialization (`Masterchat.init(videoId)`). How to get video ID from channel ID?

**Options:**

1. **Option A: overlay-manager resolves video ID**
   - overlay-manager uses official YouTube API (separate quota pool)
   - POST /streams/monitor with `{channelId, videoId}`
   - Pro: Clean separation, innertube-listener stays quota-free
   - Con: overlay-manager needs YouTube API credentials

2. **Option B: innertube-listener implements channel monitoring**
   - Use InnerTube's browse endpoint to check channel live status
   - Extract video ID from channel's live tab
   - Pro: Self-contained, no external dependencies
   - Con: Higher complexity, need to reverse-engineer InnerTube browse API

3. **Option C: Hybrid - use YouTube RSS feed**
   - YouTube exposes channel RSS feeds (no auth, no quota)
   - Parse `<media:video>` for live streams
   - Pro: Simple, no quota, no auth
   - Con: RSS updates can lag (5-15 minutes)

**Recommendation:** Option A (overlay-manager resolves) for MVP, Option B for full autonomy in Phase 2.

### Moderate Gap: Error Handling

**Problem:** masterchat errors documented ("disabled", "membersOnly", "private", "unavailable") but recovery strategy unclear.

**Solution:** Exponential backoff with max retries:
- "disabled"/"unavailable" → permanent failure, notify overlay-manager
- "membersOnly" → permanent failure (can't bypass)
- "private" → permanent failure
- Network errors → retry with backoff (1s, 2s, 4s, 8s, 16s max)

### Minor Gap: Metrics

**Problem:** Official listener has extensive Prometheus metrics. InnerTube listener should match for observability parity.

**Metrics to implement:**
- `innertube_messages_total` (counter, by channel)
- `innertube_streams_active` (gauge)
- `innertube_errors_total` (counter, by error type)
- `innertube_message_rate` (gauge, messages/sec)

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| **Stream Discovery** | Circular dependency: innertube-listener needs video ID, overlay-manager needs stream ID from listener | Design async flow: overlay-manager resolves video ID first, then calls innertube-listener |
| **Message Normalization** | masterchat message format may differ from official API (field names, types) | Build comprehensive mapping layer with unit tests for each action type |
| **Reconnection** | Infinite retry loops if stream permanently ended | Implement max retry limit, check stream liveness before retrying |
| **Graceful Shutdown** | masterchat connections not cleaned up on SIGTERM | Use Node.js signal handlers, close all connections before process.exit() |

## Sources

- [masterchat features](https://github.com/sigvt/masterchat) - "20+ actions, video comments and transcripts, sending messages and moderating chats"
- [masterchat action types](https://github.com/sigvt/masterchat/blob/master/MANUAL.md) - addChatItemAction, addSuperChatItemAction, markChatItemAsDeletedAction
- [Official youtube-listener README](file:///home/moersener/Hobby/all-chat/services/youtube-listener/README.md) - Quota system, OAuth, health checks
- [RawChatMessage contract](file:///home/moersener/Hobby/all-chat/services/youtube-listener/models/raw_message.go) - JSON schema requirements
