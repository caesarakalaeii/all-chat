# Feature Landscape

**Domain:** YouTube Live Chat Listener (InnerTube-based)
**Researched:** 2026-02-21

## Table Stakes

Features users expect. Missing = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Stream discovery** | Must find latest live stream for a channel | Medium | InnerTube uses `browse` endpoint with channel ID. No search quota costs. Filter live streams from browse results. |
| **Regular chat messages** | Core functionality - text messages from viewers | Low | InnerTube returns `liveChatTextMessageRenderer` with message, author, timestamp, badges |
| **Super Chat events** | Monetization events must be captured | Low | InnerTube provides `liveChatPaidMessageRenderer` with amount, currency, tier, message |
| **Super Sticker events** | Monetization events must be captured | Low | InnerTube provides `liveChatPaidStickerRenderer` with sticker metadata, amount |
| **Membership events** | New memberships, milestones, gifts | Medium | InnerTube supports new sponsor, milestone chat, gift purchase renderers |
| **Single message deletion** | Moderator actions must reflect in overlay | Medium | InnerTube sends `markChatItemAsDeletedAction` with target message ID |
| **Batch deletion (ban)** | User bans must remove all their messages | Medium | InnerTube sends `markChatItemsByAuthorAsDeletedAction` with author ID |
| **Continuation tokens** | Resume streaming without missing messages | Low | InnerTube provides continuation token in response, used in next request |
| **User metadata** | Display name, badges, profile image, verified status | Low | Included in InnerTube message renderers (`authorBadges`, `authorPhoto`, etc.) |
| **Emote parsing** | YouTube custom emotes (membership emotes) | Medium | InnerTube provides emoji metadata in message `runs` (image URL, alt text, custom emoji flag) |
| **Live/offline detection** | Know when stream ends | Low | InnerTube returns `offlineAt` timestamp or error when stream ends |
| **Connection gating** | Stop streaming when overlay disconnected (quota efficiency) | Low | Already implemented in official listener, reuse pattern |

## Differentiators

Features that set product apart. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Zero quota consumption** | Unlimited polling, no YouTube API quota limits | HIGH VALUE | InnerTube has no quota (unofficial API). Official listener limited to 1,009,000 units/day. |
| **Faster deletion detection** | Real-time deletion events vs polling delay | MEDIUM VALUE | InnerTube sends deletions immediately in stream. Official API requires polling every 2-5s to detect. |
| **Lower polling latency** | Faster message delivery to overlay | MEDIUM VALUE | InnerTube continuation timeout can be lower (sub-second possible). Official API enforces `pollingIntervalMillis` (2-5s). |
| **No OAuth required** | Simpler setup, no token management | HIGH VALUE | InnerTube works without authentication. Official listener requires OAuth per streamer. |
| **Ticker events** | Paid message ticker announcements | LOW VALUE | InnerTube provides `addLiveChatTickerItemAction` for ticker items. Nice-to-have, not critical. |
| **Viewer leaderboard rank** | Top contributor crown tags (#1, #2, #3) | LOW VALUE | InnerTube exposes YouTube points ranking. Community feature, not essential. |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Sending messages** | Requires authentication, out of scope, increases complexity | READ ONLY - InnerTube listener only consumes chat, never sends |
| **Polls/Creator goals** | Unstable InnerTube schema, not core chat functionality | Defer to future phase if user demand emerges |
| **Historical chat replay** | Different API endpoint, separate use case | Stream live chat only. Replay is separate feature. |
| **Multiple stream tracking per channel** | Complexity, edge case (official listener tracks latest only) | Track latest live stream only (drop-in replacement behavior) |
| **Quota tracking** | InnerTube has no quota | Remove quota tracking entirely (simplification over official listener) |

## Feature Dependencies

```
Stream discovery → Live chat streaming (need stream/video ID to fetch chat)
Continuation tokens → Resume streaming (prevents message loss on reconnect)
Connection gating → Stop streaming (prevents wasted resources when overlay offline)
User metadata → Message rendering (overlay needs display name, badges, avatar)
Single deletion → Batch deletion (both use same removal mechanism in overlay)
```

## MVP Recommendation

**Phase 1: Core parity (drop-in replacement)**
Prioritize:
1. Stream discovery (find latest live stream for channel)
2. Live chat streaming with continuation tokens
3. Regular messages, Super Chat, Super Sticker, membership events
4. Single + batch deletion events
5. User metadata (name, badges, avatar, verified)
6. Live/offline detection
7. Connection gating (stop when overlay disconnected)

**Phase 2: Differentiators (beyond official listener)**
8. YouTube custom emotes (membership emotes in message runs)
9. Ticker events (nice-to-have)

Defer:
- **Polls/Creator goals**: Unstable schema, not core functionality
- **Viewer leaderboard rank**: Low value, community feature
- **Sending messages**: Out of scope (read-only listener)

## Complexity Assessment

### Low Complexity (1-2 days)
- Regular chat messages (straightforward InnerTube renderer parsing)
- Super Chat/Sticker events (similar to regular messages)
- Continuation tokens (provided by InnerTube, just pass through)
- User metadata (included in renderers)
- Live/offline detection (check `offlineAt` field)
- Connection gating (reuse official listener pattern)

### Medium Complexity (3-5 days)
- **Stream discovery** (find live stream from channel browse results, filter logic)
- Membership events (multiple renderer types: new sponsor, milestone, gift, received)
- Single message deletion (match `targetItemId` to message ID, remove from overlay)
- Batch deletion (find all messages from author, remove)
- YouTube custom emotes (parse message `runs`, extract emoji metadata)

### High Complexity (5-10 days)
- **InnerTube protocol reverse engineering** (no official docs, must inspect network traffic)
- **Stability handling** (InnerTube schema can change without notice)
- **Error handling** (undocumented error codes, must infer from responses)

## Drop-In Replacement Feature Parity Checklist

Can InnerTube match official listener behavior exactly?

| Official Listener Feature | InnerTube Equivalent | Parity Status | Notes |
|---------------------------|----------------------|---------------|-------|
| OAuth-based stream discovery | Browse endpoint (no auth) | ✅ YES | InnerTube `browse` returns live streams, no quota cost |
| Poll live chat (5 units/poll) | Continuation-based streaming | ✅ YES (BETTER) | Zero quota, no polling cost |
| Regular messages | `liveChatTextMessageRenderer` | ✅ YES | Direct mapping |
| Super Chat | `liveChatPaidMessageRenderer` | ✅ YES | Amount, currency, tier, message |
| Super Sticker | `liveChatPaidStickerRenderer` | ✅ YES | Sticker metadata, amount |
| New membership | `liveChatSponsorshipsGiftPurchaseAnnouncementRenderer` | ✅ YES | New sponsor event |
| Membership milestone | `liveChatMembershipItemRenderer` | ✅ YES | Milestone months |
| Membership gifts | `liveChatSponsorshipsGiftPurchaseAnnouncementRenderer` | ✅ YES | Gift count, level |
| Single deletion | `markChatItemAsDeletedAction` | ✅ YES | Target message ID |
| Batch deletion (ban) | `markChatItemsByAuthorAsDeletedAction` | ✅ YES | Target author ID |
| User badges | `authorBadges` in renderer | ✅ YES | Moderator, verified, owner, member |
| Profile image | `authorPhoto` in renderer | ✅ YES | Avatar URL |
| Polling interval | Continuation timeout | ✅ YES (BETTER) | InnerTube can be faster (sub-second) |
| Live/offline detection | `offlineAt` timestamp | ✅ YES | Stream end notification |
| Quota tracking | N/A (no quota) | ✅ YES (SIMPLER) | Remove quota tracking code |
| Connection gating | Reuse pattern | ✅ YES | Stop when overlay disconnected |
| Token refresh | N/A (no auth) | ✅ YES (SIMPLER) | Remove OAuth token management |

**Verdict:** InnerTube can provide **100% feature parity** with official listener, with **improvements** in quota elimination, deletion latency, and setup simplicity.

## Key Risks

### Stability Risk: HIGH
- InnerTube is **unofficial, undocumented** API
- YouTube can change schema **without notice** (breaking changes possible)
- No SLA, no support, no version guarantees
- **Mitigation**: Robust error handling, schema validation, fallback to official listener on breakage

### Rate Limiting Risk: MEDIUM
- InnerTube has "request limits" but "so high you'll likely never hit them" (source: GitHub discussions)
- No documented rate limits, must discover empirically
- **Mitigation**: Implement exponential backoff, monitor for rate limit errors

### Detection Risk: LOW-MEDIUM
- YouTube could detect/block InnerTube usage from non-browser clients
- No evidence of widespread blocking (libraries actively used in 2026)
- **Mitigation**: User-Agent spoofing, request header matching browser behavior

### Go Library Availability Risk: HIGH
- **Most InnerTube libraries are JavaScript/Python** (.NET, Python dominate)
- **Go library found: `github.com/nezbut/innertube-go`** but **does NOT support live chat**
- **Go library found: `github.com/abhinavxd/youtube-live-chat-downloader/v2`** (Go-specific live chat)
- **Mitigation**: Evaluate `youtube-live-chat-downloader` or implement InnerTube protocol directly

## Dependencies on Existing Features

| Existing Feature | InnerTube Reuse | Modification Required |
|------------------|-----------------|----------------------|
| Message normalization (message-processor) | ✅ Reuse | None - same `RawChatMessage` schema |
| Redis Streams publishing | ✅ Reuse | None - same `chat:raw` stream |
| Connection gating pattern | ✅ Reuse | Port from official listener poller |
| Leader election (source-manager) | ✅ Reuse | None - same leader election pattern |
| Health checks | ✅ Reuse | None - same `/health/live` and `/health/ready` |
| Metrics (Prometheus) | ✅ Reuse | Remove quota metrics, add InnerTube-specific |
| Database schema | ⚠️ Partial | Remove `youtube_quota_usage`, `youtube_oauth_tokens` tables |
| OAuth manager | ❌ Remove | InnerTube requires no authentication |
| Quota tracker | ❌ Remove | InnerTube has no quota |

## Sources

### High Confidence (Official Documentation)
- [YouTube Live Streaming API - LiveChatMessages](https://developers.google.com/youtube/v3/live/docs/liveChatMessages)
- [YouTube API Quota Calculator](https://developers.google.com/youtube/v3/determine_quota_cost)

### Medium Confidence (Verified Libraries)
- [YTLiveChat (C#/.NET InnerTube library)](https://github.com/Agash/YTLiveChat)
- [YouTube.js (JavaScript InnerTube library)](https://github.com/LuanRT/YouTube.js)
- [innertube-go (Go InnerTube library - no live chat)](https://pkg.go.dev/github.com/nezbut/innertube-go)
- [youtube-live-chat-downloader (Go live chat library)](https://pkg.go.dev/github.com/abhinavxd/youtube-live-chat-downloader/v2)

### Medium Confidence (Community Research)
- [YouTube API Quota Breakdown 2026](https://www.contentstats.io/blog/youtube-api-quota-tracking)
- [YouTube API Quota Exceeded Fix 2026](https://getlate.dev/blog/youtube-api-limits-how-to-calculate-api-usage-cost-and-fix-exceeded-api-quota)
- [VTuber LiveChat Dataset (InnerTube moderation events)](https://repopython.com/r/holodata/vtuber-livechat-dataset)

### Low Confidence (GitHub Issues, Discussions)
- [Retract Message Issue #263](https://github.com/youtube/api-samples/issues/263)
- [Parse LiveChat Parameters Issue #192](https://github.com/Tyrrrz/YoutubeExplode/issues/192)
