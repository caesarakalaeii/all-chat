# Hollywood Credit Roll Feature - Complete Roadmap

## Executive Summary

Create a "Hollywood-style" credit roll feature that compiles stream events (subs, follows, bits, gifts, chatters, etc.) from all platforms and displays them with background clips from the stream. This feature will help streamers create professional end-of-stream credits to thank their community.

**Key Philosophy**: **One-Time Setup, Fully Automatic** - Users configure preferences once, then credit rolls are automatically generated after every stream with zero manual intervention.

**Timeline**: 14-16 weeks
**New Services**: 3 (Event Collector, Clip Manager, Credit Roll Generator)
**Database Changes**: 5 new tables
**Platforms**: Twitch, YouTube, Kick, TikTok

---

## 🚀 Automation Philosophy

This feature is designed with **complete hands-off automation** as the core principle:

### The Streaming Experience
This is a **LIVE, end-of-stream feature** that plays DURING the stream (not after):

```
Stream Flow:
1. 🎮 Streamer goes live → Events auto-collected in background
2. 🎬 Streamer ready to end → Switches to "Ending Soon" scene in OBS
3. ✨ Credit roll plays LIVE → Shows today's subs, follows, bits, chatters with clips
4. 👋 Stream ends → Viewers see beautiful Hollywood-style credits
```

**Use Case**: Replace boring "Ending Soon" screens with an engaging recap of today's stream events. Make the final 2-3 minutes memorable and entertaining!

### Set Once, Use Forever
1. **One-time setup** (~5 minutes): User configures:
   - Which event types to include (subs, follows, bits, etc.)
   - Clip preferences (auto-select top clips from past streams)
   - Styling (font, colors, scroll speed)
   - Fallback video for background
   - ✅ Add Browser Source to OBS "Ending Soon" scene

2. **Every stream thereafter**: 100% automatic
   - Events collected during stream (zero action needed)
   - Credit roll overlay updates in REAL-TIME with today's events
   - When streamer switches to "Ending Soon" scene → Credits play with today's data
   - Same OBS source, different content every stream!

3. **User interaction per stream**: Just click "Ending Soon" scene in OBS 🎉

### Key Features
- ✅ **Live, real-time credits**: Shows TODAY'S events as they happened
- ✅ **Automatic updates**: Overlay content updates during stream automatically
- ✅ **Smart clip selection**: Uses clips from previous streams or user-provided videos
- ✅ **Multi-platform**: Works with Twitch, YouTube, Kick, TikTok simultaneously
- ✅ **Persistent URL**: Same overlay URL works forever, content updates per stream
- ✅ **OBS-ready**: Add once to "Ending Soon" scene, use forever

---

## Table of Contents
1. [Automation Philosophy](#-automation-philosophy)
2. [Platform Analysis & OAuth Scopes](#platform-analysis--oauth-scopes)
3. [Event Types & Collection Methods](#event-types--collection-methods)
4. [Clip Management Strategy](#clip-management-strategy)
5. [Architecture Design](#architecture-design)
6. [Database Schema](#database-schema)
7. [New Services](#new-services)
8. [User Flows](#user-flows)
9. [Implementation Phases](#implementation-phases)
10. [Technical Challenges](#technical-challenges)
11. [Future Enhancements](#future-enhancements)

---

## Platform Analysis & OAuth Scopes

### Twitch (Fully Supported)

**Current Status**: Most mature platform with comprehensive EventSub system

**Events We Can Track**:
- ✅ Follows
- ✅ Subscriptions (new, resub, gift subs)
- ✅ Bits/Cheers
- ✅ Raids (incoming/outgoing)
- ✅ Channel Point Redemptions
- ✅ Hype Train events
- ✅ Chat participation (from IRC)

**Required OAuth Scopes** (in addition to existing):
```
channel:read:subscriptions      # Read subscription events
bits:read                         # Read bits events
moderator:read:followers          # Read follower information
channel:read:redemptions          # Read channel point redemptions
moderator:read:chatters           # Read list of active chatters
channel:read:hype_train           # Read hype train events
```

**API Methods**:
- **EventSub** (Webhook or WebSocket) - Real-time events
  - Subscribe to event types via POST `/eventsub/subscriptions`
  - Receive events in real-time (< 1 second latency)
  - Requires webhook endpoint with signature validation OR WebSocket connection
- **Helix API** - Historical data
  - `/chat/chatters` - List current chatters
  - `/subscriptions` - Get subscription list
  - `/clips` - Fetch clips (no special scope needed for public clips)

**Clips API**:
- `GET /clips?broadcaster_id={id}&first=100` - Get clips
- Filter by date range: `started_at`, `ended_at` parameters
- Sort by: `views`, `trending`, `time`
- Response includes: `url`, `embed_url`, `view_count`, `duration`, `thumbnail_url`

**Implementation Complexity**: 🟢 Low (well-documented, mature APIs)

---

### YouTube (OAuth per User)

**Current Status**: Already polling Live Chat API for messages

**Events We Can Track**:
- ✅ Super Chats (from Live Chat API)
- ✅ Super Stickers (from Live Chat API)
- ✅ New members (from Live Chat API - membership badges)
- ✅ Chat participation (already tracking)
- ⚠️ New subscribers (requires Analytics API - delayed data)
- ⚠️ Channel memberships (requires separate API)

**Required OAuth Scopes** (in addition to existing):
```
https://www.googleapis.com/auth/youtube.readonly                        # Already have
https://www.googleapis.com/auth/yt-analytics.readonly                  # Analytics data (subscribers, views)
https://www.googleapis.com/auth/youtube.channel-memberships.creator     # Membership information
```

**API Methods**:
- **Live Chat API** (already using) - Real-time Super Chats, members
  - Poll interval: 2-5 seconds
  - Extract `superChatDetails`, `superStickerDetails` from messages
  - Member badges: `authorDetails.isChatSponsor`
- **YouTube Analytics API** - Historical subscriber data
  - `GET /youtube/analytics/v2/reports?dimensions=day&metrics=subscribersGained`
  - Delayed by 24-48 hours
  - No real-time subscriber events available
- **Members API** - Current membership list
  - `GET /youtube/v3/members?part=snippet&maxResults=1000`
  - Paginated results

**Clips Strategy**:
- ❌ YouTube doesn't have native "clips" like Twitch
- **Alternatives**:
  1. User-provided timestamps in VOD (e.g., "1:23:45 - Epic moment")
  2. Video chapters (if enabled)
  3. Full VOD as background
  4. User-provided fallback video URL
- **Recommendation**: Rely on user-provided URLs or full VOD

**Implementation Complexity**: 🟡 Medium (no EventSub equivalent, limited clip support)

**Limitations**:
- No real-time subscriber notifications
- No webhook/EventSub system (polling only)
- Quota limits: 10,000 units/day (each analytics call = 7-10 units)

---

### Kick (New Platform)

**Current Status**: OAuth and WebSocket listener implemented, production-ready

**Events We Can Track**:
- ✅ Chat messages (via Pusher WebSocket)
- ✅ Subscriptions (via WebSocket events)
- ✅ Gifts (via WebSocket events)
- ⚠️ Follows (need to verify API availability)
- ⚠️ Raids (need to verify)

**Required OAuth Scopes**:
```
Currently implemented scopes should cover basic functionality.
Need to investigate Kick API documentation for:
- Historical follower data
- Subscription history
- Gift history
```

**API Methods**:
- **Pusher WebSocket** (already using) - Real-time events
  - Subscribe to `chatrooms.{chatroom_id}` channel
  - Event types: `ChatMessageEvent`, `SubscriptionEvent`, `GiftedSubscriptionsEvent`
- **Kick REST API** - Historical data
  - Need to investigate endpoints for:
    - `/api/v2/channels/{slug}/followers` (?)
    - `/api/v2/channels/{slug}/subscriptions` (?)
  - Documentation is limited/unofficial

**Clips API**:
- Kick has clips feature visible on website
- **Need to investigate**:
  - Kick API endpoint for clips (likely `/api/v2/channels/{slug}/clips`)
  - Authentication requirements
  - Filtering/sorting options
- **Assumption**: Similar structure to Twitch clips API

**Implementation Complexity**: 🟡 Medium (limited public documentation, newer platform)

**Challenges**:
- Limited official API documentation
- May need to reverse-engineer endpoints
- API stability unknown

---

### TikTok (In Development)

**Current Status**: OAuth implemented, Listener service not yet built

**Events We Can Track**:
- ✅ Gifts (roses, etc.) - TikTok Live Events API
- ✅ Likes - TikTok Live Events API
- ✅ Follows - TikTok Live Events API
- ✅ Shares - TikTok Live Events API
- ✅ Chat messages - WebCast WebSocket (when implemented)

**Required OAuth Scopes**:
```
Currently using: user.info.basic, user.info.profile

Additional scopes needed:
- TBD: Need to review TikTok Live Events API documentation
- May need: live.room.info, live.room.events
```

**API Methods**:
- **TikTok Live Events API** (Webhook-based)
  - Register webhook URL for live room events
  - Receive events for: gifts, likes, follows, shares
  - Real-time delivery
- **WebCast WebSocket** (undocumented)
  - Alternative: reverse-engineered protocol
  - Used by community libraries (e.g., TikTok-Live-Connector)
  - Higher risk of breaking changes

**Clips Strategy**:
- ❌ TikTok is a short-form video platform (15-60s videos)
- ❌ Live stream replays exist but no "clip" concept
- **Alternatives**:
  1. Use highlights from live stream replay (if available)
  2. User-provided fallback video URL
  3. Stream thumbnail/poster image

**Implementation Complexity**: 🔴 High (platform in development, limited clip support)

**Challenges**:
- Listener service not yet implemented
- Live Events API may require additional approval
- No native clip functionality

---

## Event Types & Collection Methods

### Event Type Taxonomy

We'll normalize all platform events into these categories:

| Category | Description | Platforms | Priority |
|----------|-------------|-----------|----------|
| **Follow** | New follower | Twitch, YouTube, Kick, TikTok | HIGH |
| **Subscription** | New subscriber (paid) | Twitch, YouTube, Kick | HIGH |
| **Gift Subscription** | Gifted subs to others | Twitch, Kick | HIGH |
| **Super Chat / Bits** | Monetary support | Twitch (bits), YouTube (super chat), Kick (?) | HIGH |
| **Raid** | Incoming raid from another streamer | Twitch, Kick (?) | MEDIUM |
| **Channel Points** | Channel point redemptions | Twitch | MEDIUM |
| **Hype Train** | Hype train milestones | Twitch | LOW |
| **Chat Participation** | Unique chatters | All platforms | HIGH |
| **Membership** | Channel membership | YouTube | HIGH |
| **TikTok Gift** | Virtual gifts (roses, etc.) | TikTok | HIGH |
| **Share/Like** | Social engagement | TikTok, YouTube (?) | LOW |

### Unified Event Model

All events will be normalized to this structure:

```json
{
  "id": "uuid",
  "stream_session_id": "uuid",
  "user_id": "uuid",
  "platform": "twitch|youtube|kick|tiktok",
  "event_type": "follow|sub|bits|raid|gift_sub|super_chat|channel_points|chatter|membership|tiktok_gift|share",
  "event_subtype": "new_sub|resub|gift_sub|tier_1|tier_2|tier_3",

  "platform_user": {
    "id": "platform-specific-user-id",
    "username": "username",
    "display_name": "Display Name",
    "avatar_url": "https://..."
  },

  "metadata": {
    "amount": 500,              // bits, super chat amount (cents), gift count
    "currency": "USD",          // for super chats
    "message": "Great stream!",  // attached message
    "tier": "1000|2000|3000",   // sub tier (Twitch)
    "months": 12,               // cumulative months (resubs)
    "streak": 6,                // streak months
    "recipient_count": 5,       // for gift subs
    "recipients": [...],        // gift sub recipients
    "raid_viewer_count": 50,    // for raids
    "reward_title": "Hydrate",  // for channel points
    "gift_type": "Rose",        // for TikTok gifts
    "gift_count": 10            // for TikTok gifts
  },

  "occurred_at": "2025-11-19T12:34:56Z",
  "created_at": "2025-11-19T12:34:57Z"
}
```

### Collection Strategies by Platform

#### Twitch: EventSub + Helix API

**Real-time Collection (EventSub WebSocket)**:
```
1. Establish WebSocket connection to wss://eventsub.wss.twitch.tv/ws
2. Subscribe to event types:
   - channel.follow (v2)
   - channel.subscribe
   - channel.subscription.gift
   - channel.subscription.message
   - channel.cheer
   - channel.raid
   - channel.channel_points_custom_reward_redemption.add
   - channel.hype_train.begin/progress/end
3. Receive events on WebSocket
4. Normalize and store in database
```

**Periodic Collection (Helix API)**:
```
1. Every 60 seconds: Fetch active chatters from /chat/chatters
2. Store unique chatters for session (deduplicate)
3. On stream end: Compile full chatter list
```

**Historical Backfill** (if user starts mid-stream):
```
1. Fetch recent followers: /users/follows?to_id={broadcaster_id}&first=100
2. Filter by timestamp > stream_start_time
3. Store with "backfilled" flag
```

#### YouTube: Live Chat API + Analytics API

**Real-time Collection (Live Chat API)**:
```
Already polling every 2-5 seconds:
1. Extract superChatDetails from messages → super_chat event
2. Extract superStickerDetails from messages → super_sticker event
3. Track authorDetails.isChatSponsor=true → membership event (first occurrence)
4. Track unique author IDs → chatter event
```

**Daily Collection (Analytics API)**:
```
1. Daily cron job: Fetch subscriber gains for previous day
2. Query: dimensions=day&metrics=subscribersGained&startDate=yesterday&endDate=yesterday
3. Store as subscriber events (no individual user attribution - limitation)
4. Note: 24-48 hour delay is inherent to YouTube Analytics
```

**Limitation**: YouTube doesn't provide real-time individual subscriber notifications via API.

#### Kick: Pusher WebSocket + REST API

**Real-time Collection (Pusher WebSocket)**:
```
Already connected to Pusher:
1. Listen for subscription events → subscription event
2. Listen for gift events → gift_subscription event
3. Track chat message authors → chatter event
4. (Need to verify) Listen for follow events → follow event
```

**Periodic Collection (REST API)**:
```
1. Need to investigate Kick API endpoints:
   - Recent followers (if available)
   - Subscription history (if available)
2. Poll every 60 seconds as backup to WebSocket
3. Deduplicate against WebSocket events
```

#### TikTok: Events API (Webhook) - Future Implementation

**Webhook Collection**:
```
1. Register webhook URL with TikTok: POST /live/events/webhook/
2. Receive webhook events for:
   - Gift events (roses, etc.)
   - Follow events
   - Like events
   - Share events
3. Validate webhook signatures
4. Normalize and store events
```

**Real-time Collection (WebCast WebSocket)** - Alternative:
```
If official API insufficient:
1. Use community libraries (e.g., TikTok-Live-Connector)
2. Connect to WebCast WebSocket
3. Parse protobuf messages
4. Risk: Unofficial, may break
```

---

## Clip Management Strategy

### Clip Sources by Platform

#### Twitch Clips (Best Support)

**Fetching Strategy**:
```
1. On stream end (or on-demand):
   GET /clips?broadcaster_id={id}&started_at={stream_start}&ended_at={stream_end}&first=100

2. Ranking algorithm:
   - Primary: View count (more views = better)
   - Secondary: Creation time during stream (first 20% of clips tend to be highlights)
   - Tertiary: Duration (prefer 15-45 second clips)

3. Filtering:
   - Exclude clips < 10 seconds (too short)
   - Exclude clips > 60 seconds (too long for background)
   - Require minimum view count (e.g., 10 views)

4. Storage:
   - Cache clip metadata (URL, embed URL, views, duration)
   - Store thumbnail for preview
   - Refresh view counts daily for 7 days post-stream
```

**Fallback**: If < 3 clips available, fetch clips from past 30 days and use highest-viewed.

#### YouTube (No Native Clips)

**Strategy**: User-provided content

```
Options for users:
1. Paste YouTube video URL with timestamps (e.g., https://youtube.com/watch?v=xxx&t=123)
   - Parse timestamp, create 30-second segments
   - Use YouTube Player API to seek to timestamp

2. Use full VOD as background
   - Auto-detect stream VOD from channel
   - Play random segments throughout credit roll

3. Provide fallback video URL (see below)

4. Use stream thumbnail as static background
```

#### Kick Clips (Needs Investigation)

**Assumed API** (similar to Twitch):
```
1. Investigate endpoint: GET /api/v2/channels/{slug}/clips
2. Expected parameters: start_date, end_date, sort_by
3. Ranking: Same algorithm as Twitch (views, recency, duration)
4. If API unavailable: User-provided fallback only
```

#### TikTok (No Clip Concept)

**Strategy**: Fallback only
```
1. Check if stream replay available (TikTok saves recent live streams)
2. If available: Use random segments from replay
3. If not: Use user-provided fallback video URL
4. Last resort: Static background with TikTok logo/colors
```

### Fallback Video System

**User-Provided Fallback**:
```
1. User uploads video file to S3/Cloud Storage
2. OR user provides YouTube/Vimeo URL
3. System validates:
   - Video is accessible
   - Duration > 60 seconds (for looping)
   - Resolution >= 720p
4. Use as background when platform clips unavailable
5. Loop video if duration < credit roll duration
```

**Default Fallback**:
```
If no user-provided fallback:
1. Generate abstract background with particles/gradient
2. Use platform brand colors
3. Overlay platform logo watermark
4. Smooth color transitions between platforms
```

### Clip Selection Algorithm

**Inputs**:
- Stream duration
- Available clips per platform
- User preferences (clip count, diversity)

**Algorithm**:
```python
def select_clips(stream_session, user_prefs):
    clips = []

    # Fetch clips from all platforms used in stream
    for platform in stream_session.platforms:
        platform_clips = fetch_clips(platform, stream_session.start, stream_session.end)
        clips.extend(platform_clips)

    # Rank clips
    clips.sort(key=lambda c: (
        c.view_count * 0.6 +          # 60% weight on views
        c.recency_score * 0.3 +       # 30% weight on recency (during stream)
        c.duration_score * 0.1        # 10% weight on ideal duration (30s)
    ), reverse=True)

    # Diversify: Don't show clips back-to-back from same platform
    selected = []
    last_platform = None
    for clip in clips:
        if clip.platform != last_platform or len(selected) == 0:
            selected.append(clip)
            last_platform = clip.platform
            if len(selected) >= user_prefs.max_clips:
                break

    # Fill with fallback if needed
    if len(selected) < user_prefs.min_clips:
        selected.extend(get_fallback_videos(user_prefs))

    return selected
```

---

## Architecture Design

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Frontend (React/Next.js)                     │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌─────────────────┐  │
│  │ Credit Roll      │  │ Event Dashboard  │  │ Clip Management │  │
│  │ Configuration UI │  │ (History view)   │  │ UI              │  │
│  └──────────────────┘  └──────────────────┘  └─────────────────┘  │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │         Credit Roll Overlay Display (WebGL/Canvas)           │  │
│  │  - Scrolling credits    - Background video player            │  │
│  │  - Section rendering    - Smooth transitions                 │  │
│  └──────────────────────────────────────────────────────────────┘  │
└───────────────────────────────┬──────────────────────────────────┬─┘
                                │ HTTP/REST                        │ WebSocket
                                ▼                                  ▼
┌────────────────────────────────────────────────────────────────────┐
│                       API Gateway (Port 8080)                       │
│  - HTTP routing         - WebSocket management                     │
│  - Authentication       - Rate limiting                            │
└────┬─────────┬─────────┬─────────┬─────────┬─────────────────────┘
     │         │         │         │         │
     ▼         ▼         ▼         ▼         ▼
┌─────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐ ┌──────────────┐
│ Event   │ │   Clip   │ │  Credit  │ │  Existing  │ │   Existing   │
│Collector│ │ Manager  │ │   Roll   │ │  Services  │ │   Services   │
│ Service │ │ Service  │ │Generator │ │  (Twitch   │ │  (YouTube,   │
│         │ │          │ │ Service  │ │  Listener, │ │  Kick, etc.) │
│ :8090   │ │  :8091   │ │  :8092   │ │  etc.)     │ │              │
└─────────┘ └──────────┘ └──────────┘ └────────────┘ └──────────────┘
     │            │            │             │
     │            │            │             │
     ▼            ▼            ▼             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         PostgreSQL Database                          │
│  ┌────────────────┐  ┌─────────────┐  ┌──────────────┐            │
│  │ stream_events  │  │    clips    │  │ credit_rolls │            │
│  │ stream_sessions│  │clip_prefs   │  │ event_stats  │            │
│  └────────────────┘  └─────────────┘  └──────────────┘            │
└─────────────────────────────────────────────────────────────────────┘
     ▲
     │ Store events
     │
┌────┴────────────────────────────────────────────────────────────────┐
│                    Platform Event Sources                            │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐ │
│  │ Twitch EventSub  │  │ YouTube Live Chat│  │ Kick Pusher WS   │ │
│  │   (WebSocket)    │  │   API (Polling)  │  │  (WebSocket)     │ │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘ │
│                                                                      │
│  ┌──────────────────┐                                               │
│  │ TikTok Events API│                                               │
│  │   (Webhooks)     │                                               │
│  └──────────────────┘                                               │
└──────────────────────────────────────────────────────────────────────┘
```

### Data Flow

#### 1. Event Collection Flow

```
Stream Live → Platform Events Occur → Event Collector Service
                                             │
                                             ▼
                                     Normalize Event
                                             │
                                             ▼
                                   Store in stream_events table
                                             │
                                             ▼
                                   Aggregate statistics
                                             │
                                             ▼
                                   Update event_stats table
                                   (real-time leaderboards)
```

**Concrete Example (Twitch Follow)**:
```
1. User "Alice" follows broadcaster
2. Twitch sends EventSub notification to Event Collector webhook
3. Event Collector receives:
   {
     "subscription": { "type": "channel.follow" },
     "event": {
       "user_id": "12345",
       "user_login": "alice",
       "user_name": "Alice",
       "broadcaster_user_id": "67890",
       "followed_at": "2025-11-19T12:34:56Z"
     }
   }
4. Event Collector normalizes to:
   {
     "stream_session_id": "current-session-uuid",
     "platform": "twitch",
     "event_type": "follow",
     "platform_user": {
       "id": "12345",
       "username": "alice",
       "display_name": "Alice"
     },
     "occurred_at": "2025-11-19T12:34:56Z"
   }
5. Stores in stream_events table
6. Updates event_stats.follow_count += 1 for session
```

#### 2. Clip Management Flow

```
Stream Ends → Clip Manager Service Triggered
                      │
                      ▼
              Fetch Clips from Platforms
              (Twitch API, Kick API)
                      │
                      ▼
              Rank Clips by Algorithm
                      │
                      ▼
              Store in clips table
                      │
                      ▼
              User Reviews/Approves
              (via Configuration UI)
                      │
                      ▼
              Selected clips stored in
              credit_roll.config.clips[]
```

#### 3. Credit Roll Generation Flow

```
User Initiates Generation → Credit Roll Generator Service
                                       │
                                       ▼
                            Query stream_events for date range
                                       │
                                       ▼
                            Group events by type
                            (subs, follows, bits, etc.)
                                       │
                                       ▼
                            Sort within groups
                            (by amount for bits, alphabetically for names)
                                       │
                                       ▼
                            Select clips from clips table
                                       │
                                       ▼
                            Generate timeline:
                            [
                              { type: "video", clip_id: "...", duration: 30s },
                              { type: "section", title: "Subscribers", names: [...], duration: 45s },
                              { type: "video", clip_id: "...", duration: 30s },
                              { type: "section", title: "Top Bits", entries: [...], duration: 30s },
                              ...
                            ]
                                       │
                                       ▼
                            Store in credit_rolls table
                                       │
                                       ▼
                            Return preview URL to user
```

#### 4. Overlay Display Flow

```
Browser Loads Overlay → Fetch credit roll data from API
                                 │
                                 ▼
                        Preload video clips
                                 │
                                 ▼
                        Initialize Canvas/WebGL
                                 │
                                 ▼
                        Start playback:
                        - Video plays in background
                        - Credits scroll over video
                        - On video end → crossfade to next video
                        - On credits end → show outro
                                 │
                                 ▼
                        Loop or end based on config
```

---

## Database Schema

### New Tables

```sql
-- ============================================================================
-- Stream Sessions: Track individual streaming sessions
-- ============================================================================
CREATE TABLE stream_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Session metadata
    title TEXT,
    description TEXT,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,

    -- Platform information (JSONB for flexibility)
    -- Example: {"twitch": {"channel_id": "12345", "game": "Just Chatting"}, "youtube": {...}}
    platform_info JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Status: live, ended, archived
    status VARCHAR(50) NOT NULL DEFAULT 'live',

    -- Cached statistics (updated in real-time by Event Collector)
    stats JSONB DEFAULT '{
        "total_events": 0,
        "followers": 0,
        "subscribers": 0,
        "bits_total": 0,
        "super_chat_total": 0,
        "unique_chatters": 0,
        "peak_viewers": 0
    }'::jsonb,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stream_sessions_user_id ON stream_sessions(user_id);
CREATE INDEX idx_stream_sessions_started_at ON stream_sessions(started_at DESC);
CREATE INDEX idx_stream_sessions_status ON stream_sessions(status);

-- ============================================================================
-- Stream Events: All platform events (subs, follows, bits, etc.)
-- ============================================================================
CREATE TABLE stream_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stream_session_id UUID NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Platform & event type
    platform VARCHAR(50) NOT NULL, -- twitch, youtube, kick, tiktok
    event_type VARCHAR(50) NOT NULL, -- follow, sub, bits, raid, gift_sub, super_chat, etc.
    event_subtype VARCHAR(50), -- new_sub, resub, tier_1, tier_2, tier_3, etc.

    -- User who triggered the event
    platform_user_id VARCHAR(255),
    platform_username VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,

    -- Event-specific data (flexible JSONB structure)
    metadata JSONB DEFAULT '{}'::jsonb,
    -- Examples:
    -- Bits: {"amount": 500, "message": "Great stream!"}
    -- Sub: {"tier": "1000", "months": 12, "streak": 6}
    -- Gift: {"recipient_count": 5, "recipients": [...]}
    -- Raid: {"raid_viewer_count": 50, "from_broadcaster": "..."}
    -- Super Chat: {"amount": 1000, "currency": "USD", "message": "..."}

    -- Timestamps
    occurred_at TIMESTAMP NOT NULL, -- When event happened on platform
    created_at TIMESTAMP NOT NULL DEFAULT NOW(), -- When we recorded it

    -- Flags
    is_test BOOLEAN DEFAULT FALSE, -- For testing
    is_backfilled BOOLEAN DEFAULT FALSE -- If added retroactively
);

CREATE INDEX idx_stream_events_session ON stream_events(stream_session_id);
CREATE INDEX idx_stream_events_user ON stream_events(user_id);
CREATE INDEX idx_stream_events_type ON stream_events(event_type, platform);
CREATE INDEX idx_stream_events_occurred_at ON stream_events(occurred_at DESC);
CREATE INDEX idx_stream_events_platform_user ON stream_events(platform_user_id);

-- Composite index for common queries (events by session and type)
CREATE INDEX idx_stream_events_session_type ON stream_events(stream_session_id, event_type);

-- ============================================================================
-- Clips: Platform clips and user-provided videos
-- ============================================================================
CREATE TABLE clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_session_id UUID REFERENCES stream_sessions(id) ON DELETE SET NULL,

    -- Platform information
    platform VARCHAR(50) NOT NULL, -- twitch, kick, youtube, user_upload
    platform_clip_id VARCHAR(255), -- Platform's ID for the clip

    -- URLs
    clip_url TEXT NOT NULL, -- Direct link to clip
    embed_url TEXT, -- Embeddable player URL
    thumbnail_url TEXT,

    -- Metadata
    title TEXT,
    duration_seconds INTEGER, -- Clip duration
    view_count INTEGER DEFAULT 0,
    created_at_platform TIMESTAMP, -- When clip was created on platform

    -- User-provided clips
    is_user_provided BOOLEAN DEFAULT FALSE,
    user_notes TEXT,

    -- Ranking (computed by Clip Manager)
    rank_score FLOAT, -- Computed ranking score (views, recency, duration)

    -- Timestamps
    fetched_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(platform, platform_clip_id)
);

CREATE INDEX idx_clips_user ON clips(user_id);
CREATE INDEX idx_clips_session ON clips(stream_session_id);
CREATE INDEX idx_clips_platform ON clips(platform);
CREATE INDEX idx_clips_rank ON clips(rank_score DESC NULLS LAST);

-- ============================================================================
-- User Credit Roll Settings: One-time user configuration for automatic generation
-- ============================================================================
CREATE TABLE user_credit_roll_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

    -- Auto-generation settings
    auto_generate_enabled BOOLEAN DEFAULT TRUE, -- Automatically generate after each stream
    auto_publish_enabled BOOLEAN DEFAULT TRUE, -- Automatically publish (skip review)

    -- Event sections configuration
    sections_config JSONB DEFAULT '{
        "subscribers": {"enabled": true, "title": "Thank You Subscribers", "sort": "alphabetical"},
        "bits": {"enabled": true, "title": "Top Bits Supporters", "sort": "amount_desc"},
        "followers": {"enabled": true, "title": "New Followers", "sort": "alphabetical"},
        "chatters": {"enabled": true, "title": "Amazing Chatters", "sort": "message_count"},
        "raids": {"enabled": true, "title": "Raiders", "sort": "viewer_count"}
    }'::jsonb,

    -- Clip selection settings
    clip_selection_mode VARCHAR(50) DEFAULT 'auto', -- auto, manual (manual requires user review)
    max_clips INTEGER DEFAULT 5,
    min_clips INTEGER DEFAULT 1,
    prefer_recent BOOLEAN DEFAULT TRUE,
    min_duration_seconds INTEGER DEFAULT 10,
    max_duration_seconds INTEGER DEFAULT 60,

    -- Fallback video
    fallback_video_url TEXT,
    fallback_video_start_time INTEGER,

    -- Default background (if no clips/fallback)
    default_background_type VARCHAR(50) DEFAULT 'gradient',
    default_background_config JSONB DEFAULT '{
        "colors": ["#6366f1", "#8b5cf6", "#d946ef"]
    }'::jsonb,

    -- Styling (applied to all auto-generated rolls)
    styling_config JSONB DEFAULT '{
        "font_family": "Inter",
        "text_color": "#ffffff",
        "background_overlay": "rgba(0,0,0,0.4)",
        "scroll_speed": "medium"
    }'::jsonb,

    -- Music settings
    music_enabled BOOLEAN DEFAULT FALSE,
    music_url TEXT,
    music_volume FLOAT DEFAULT 0.7,

    -- Notification settings
    notify_on_generation BOOLEAN DEFAULT TRUE, -- Send notification when credit roll ready
    notification_email TEXT,
    notification_discord_webhook TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- Credit Rolls: Generated credit roll configurations
-- ============================================================================
CREATE TABLE credit_rolls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_session_id UUID REFERENCES stream_sessions(id) ON DELETE SET NULL,

    -- Metadata
    name VARCHAR(255) NOT NULL, -- User-friendly name (e.g., "Nov 19 Stream Credits")
    description TEXT,

    -- Time range for events (can be different from session start/end)
    event_start_time TIMESTAMP NOT NULL,
    event_end_time TIMESTAMP NOT NULL,

    -- Configuration (sections, styling, clips)
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Example structure:
    -- {
    --   "sections": [
    --     {"type": "subscribers", "enabled": true, "title": "Thank You Subscribers", "sort": "alphabetical"},
    --     {"type": "bits", "enabled": true, "title": "Top Bits Supporters", "sort": "amount_desc"},
    --     {"type": "followers", "enabled": true, "title": "New Followers", "sort": "alphabetical"}
    --   ],
    --   "styling": {
    --     "font_family": "Inter",
    --     "text_color": "#ffffff",
    --     "background_overlay": "rgba(0,0,0,0.4)",
    --     "scroll_speed": "medium"
    --   },
    --   "clips": {
    --     "enabled": true,
    --     "selected_clip_ids": ["uuid1", "uuid2", "uuid3"],
    --     "transition": "crossfade",
    --     "duration_per_clip": 30
    --   },
    --   "music": {
    --     "enabled": true,
    --     "url": "https://...",
    --     "volume": 0.7
    --   }
    -- }

    -- Generated data (cached)
    generated_data JSONB,
    -- Timeline structure returned by Credit Roll Generator:
    -- {
    --   "timeline": [
    --     {"type": "video", "clip_id": "uuid", "start": 0, "end": 30},
    --     {"type": "section", "section_type": "subscribers", "start": 5, "end": 50, "entries": [...]},
    --     {"type": "video", "clip_id": "uuid2", "start": 30, "end": 60},
    --     ...
    --   ],
    --   "total_duration_seconds": 180,
    --   "event_counts": {"subscribers": 15, "followers": 42, "bits": 8}
    -- }

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'draft', -- draft, generated, published, archived

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP,

    -- Stats (views, shares)
    view_count INTEGER DEFAULT 0,
    last_viewed_at TIMESTAMP
);

CREATE INDEX idx_credit_rolls_user ON credit_rolls(user_id);
CREATE INDEX idx_credit_rolls_session ON credit_rolls(stream_session_id);
CREATE INDEX idx_credit_rolls_status ON credit_rolls(status);
CREATE INDEX idx_credit_rolls_created_at ON credit_rolls(created_at DESC);

-- ============================================================================
-- Triggers: Auto-update timestamps
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_stream_sessions_updated_at BEFORE UPDATE ON stream_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_credit_roll_settings_updated_at BEFORE UPDATE ON user_credit_roll_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_credit_rolls_updated_at BEFORE UPDATE ON credit_rolls
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_clips_last_updated BEFORE UPDATE ON clips
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Auto-Generation Trigger: Create credit roll job when stream ends
-- ============================================================================

CREATE OR REPLACE FUNCTION trigger_credit_roll_generation()
RETURNS TRIGGER AS $$
BEGIN
    -- Only trigger if session status changed to 'ended' and user has auto-generation enabled
    IF NEW.status = 'ended' AND OLD.status != 'ended' THEN
        -- Check if user has auto-generation enabled
        IF EXISTS (
            SELECT 1 FROM user_credit_roll_settings
            WHERE user_id = NEW.user_id
            AND auto_generate_enabled = TRUE
        ) THEN
            -- Publish message to Redis for async processing
            -- (In practice, this would be handled by a background worker)
            -- Redis PUBLISH credit_roll:generate '{"session_id": "...", "user_id": "..."}'

            -- Alternative: Insert into a job queue table
            -- INSERT INTO credit_roll_jobs (session_id, user_id, status)
            -- VALUES (NEW.id, NEW.user_id, 'pending');

            -- For now, just log that generation should be triggered
            RAISE NOTICE 'Credit roll generation triggered for session %', NEW.id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER auto_generate_credit_roll AFTER UPDATE ON stream_sessions
    FOR EACH ROW EXECUTE FUNCTION trigger_credit_roll_generation();
```

### Schema Highlights

**Key Design Decisions**:

1. **JSONB for Flexibility**: Use JSONB for `metadata`, `config`, `generated_data` to support platform-specific data without rigid schema
2. **Denormalized Statistics**: Cache event counts in `stream_sessions.stats` for fast dashboard queries (updated in real-time)
3. **Soft Foreign Keys**: `stream_session_id` in `clips` and `credit_rolls` is nullable (SET NULL on delete) so clips/rolls persist even if session is deleted
4. **Composite Indexes**: Optimize common queries (e.g., events by session and type)
5. **Timestamps**: Track both `occurred_at` (platform time) and `created_at` (our time) for events
6. **Auto-Generation Settings**: `user_credit_roll_settings` table stores one-time configuration that applies to ALL future streams (set once, use forever)
7. **Database Trigger**: Automatically publishes Redis job when `stream_sessions.status` changes to 'ended' (enables hands-off generation)

**Storage Estimates** (for 3-hour stream with 100 viewers):
- `stream_events`: ~1,000 rows (follows, subs, bits, chatters) × 1 KB = 1 MB
- `clips`: ~20 rows × 500 bytes = 10 KB
- `credit_rolls`: 1 row × 50 KB (with generated data) = 50 KB
- **Total per stream**: ~1 MB

---

## New Services

### Service 1: Event Collector Service (Port 8090)

**Responsibility**: Collect, normalize, and store platform events in real-time

**Key Components**:
```
services/event-collector/
├── cmd/main.go                  # Service entry point
├── handlers/
│   ├── webhooks.go              # Twitch EventSub, TikTok webhook receiver
│   ├── health.go                # Health checks
│   └── stats.go                 # Event statistics API
├── collectors/
│   ├── twitch_eventsub.go       # Twitch EventSub WebSocket client
│   ├── youtube_poller.go        # YouTube Live Chat event extraction
│   ├── kick_listener.go         # Kick WebSocket event listener
│   └── tiktok_webhook.go        # TikTok webhook handler
├── normalizers/
│   ├── normalizer.go            # Interface & base implementation
│   ├── twitch_normalizer.go     # Twitch event → unified event
│   ├── youtube_normalizer.go    # YouTube event → unified event
│   ├── kick_normalizer.go       # Kick event → unified event
│   └── tiktok_normalizer.go     # TikTok event → unified event
├── repository/
│   ├── events.go                # CRUD for stream_events table
│   └── sessions.go              # CRUD for stream_sessions table
├── aggregator/
│   └── stats.go                 # Real-time statistics aggregation
└── models/
    └── event.go                 # Unified event model
```

**API Endpoints**:

```
POST   /webhooks/twitch          # Twitch EventSub webhook receiver
POST   /webhooks/tiktok          # TikTok events webhook receiver

GET    /api/v1/sessions/:id/events              # Get events for session
GET    /api/v1/sessions/:id/events/stats        # Get aggregated stats
GET    /api/v1/sessions/:id/events/leaderboard  # Top bits, subs, etc.

GET    /health/live              # Liveness probe
GET    /health/ready             # Readiness probe (DB + Redis)
```

**Core Logic**:

1. **Twitch EventSub (WebSocket)**:
   ```go
   // Establish WebSocket connection
   conn, err := websocket.Dial("wss://eventsub.wss.twitch.tv/ws")

   // Subscribe to event types
   subscriptions := []string{
       "channel.follow",
       "channel.subscribe",
       "channel.subscription.gift",
       "channel.cheer",
       "channel.raid",
   }

   for _, eventType := range subscriptions {
       subscribeToEventType(conn, eventType, broadcasterID)
   }

   // Listen for events
   for {
       var msg EventSubMessage
       conn.ReadJSON(&msg)

       // Normalize and store
       event := twitchNormalizer.Normalize(msg)
       repository.CreateEvent(event)

       // Update session stats
       aggregator.UpdateStats(event)
   }
   ```

2. **YouTube Event Extraction** (integrate with existing YouTube Listener):
   ```go
   // Hook into YouTube Listener's message polling
   func ExtractEvents(liveChatMessage youtube.LiveChatMessage) []Event {
       events := []Event{}

       // Check for Super Chat
       if liveChatMessage.Snippet.SuperChatDetails != nil {
           events = append(events, Event{
               Type: "super_chat",
               Metadata: map[string]interface{}{
                   "amount": liveChatMessage.Snippet.SuperChatDetails.AmountMicros / 10000,
                   "currency": liveChatMessage.Snippet.SuperChatDetails.Currency,
                   "message": liveChatMessage.Snippet.DisplayMessage,
               },
           })
       }

       // Check for new member (first time seeing membership badge)
       if liveChatMessage.AuthorDetails.IsChatSponsor && !seenMember(userID) {
           events = append(events, Event{
               Type: "membership",
           })
       }

       return events
   }
   ```

3. **Statistics Aggregation**:
   ```go
   func (a *Aggregator) UpdateStats(event Event) error {
       // Atomic increment in database
       query := `
           UPDATE stream_sessions
           SET stats = jsonb_set(
               stats,
               '{` + event.Type + `_count}',
               (COALESCE((stats->>` + event.Type + `_count)::int, 0) + 1)::text::jsonb
           )
           WHERE id = $1
       `
       _, err := a.db.Exec(query, event.StreamSessionID)

       // Also update Redis for real-time leaderboards
       if event.Type == "bits" {
           a.redis.ZIncrBy(ctx,
               fmt.Sprintf("leaderboard:bits:%s", event.StreamSessionID),
               float64(event.Metadata["amount"].(int)),
               event.PlatformUserID,
           )
       }

       return err
   }
   ```

**Environment Variables**:
```
DATABASE_HOST=localhost
DATABASE_PORT=5432
REDIS_HOST=localhost
REDIS_PORT=6379
PORT=8090

TWITCH_CLIENT_ID=...
TWITCH_CLIENT_SECRET=...
TIKTOK_CLIENT_KEY=...
TIKTOK_CLIENT_SECRET=...
```

**Deployment**:
- Dockerfile: `services/event-collector/Dockerfile`
- Kubernetes: `deployments/k8s/base/event-collector/`
- HPA: Scale based on event ingestion rate (CPU > 70%)

---

### Service 2: Clip Manager Service (Port 8091)

**Responsibility**: Fetch, rank, store, and serve clips from platforms

**Key Components**:
```
services/clip-manager/
├── cmd/main.go                  # Service entry point
├── handlers/
│   ├── clips.go                 # Clip CRUD API
│   ├── preferences.go           # User preferences API
│   └── fetch.go                 # Trigger clip fetching
├── fetchers/
│   ├── twitch_fetcher.go        # Fetch Twitch clips via Helix API
│   ├── kick_fetcher.go          # Fetch Kick clips (if API available)
│   └── youtube_fetcher.go       # Handle YouTube fallback logic
├── ranker/
│   └── clip_ranker.go           # Ranking algorithm
├── repository/
│   ├── clips.go                 # CRUD for clips table
│   └── preferences.go           # CRUD for user_clip_preferences table
└── models/
    └── clip.go                  # Clip model
```

**API Endpoints**:

```
# Clip Management
POST   /api/v1/clips/fetch/:session_id      # Trigger clip fetching for session
GET    /api/v1/clips?session_id=X           # Get clips for session
GET    /api/v1/clips/:id                    # Get single clip
POST   /api/v1/clips                        # Upload user-provided clip
DELETE /api/v1/clips/:id                    # Delete clip

# Preferences
GET    /api/v1/clips/preferences            # Get user preferences
PUT    /api/v1/clips/preferences            # Update preferences

# Selection
POST   /api/v1/clips/select                 # Run selection algorithm
       Body: { session_id, max_clips, preferences }
       Returns: [ {clip_id, rank_score}, ... ]

# Health
GET    /health/live
GET    /health/ready
```

**Core Logic**:

1. **Fetch Twitch Clips**:
   ```go
   func (f *TwitchFetcher) FetchClips(sessionID uuid.UUID) ([]Clip, error) {
       session, _ := f.repo.GetSession(sessionID)

       // Get broadcaster ID from session
       broadcasterID := session.PlatformInfo["twitch"]["channel_id"]

       // Fetch clips from Twitch Helix API
       resp, err := f.twitchClient.GetClips(&helix.ClipsParams{
           BroadcasterID: broadcasterID,
           StartedAt:     session.StartedAt,
           EndedAt:       session.EndedAt,
           First:         100, // Max per request
       })

       clips := []Clip{}
       for _, twitchClip := range resp.Data.Clips {
           clips = append(clips, Clip{
               UserID:            session.UserID,
               StreamSessionID:   sessionID,
               Platform:          "twitch",
               PlatformClipID:    twitchClip.ID,
               ClipURL:           twitchClip.URL,
               EmbedURL:          twitchClip.EmbedURL,
               ThumbnailURL:      twitchClip.ThumbnailURL,
               Title:             twitchClip.Title,
               DurationSeconds:   int(twitchClip.Duration),
               ViewCount:         twitchClip.ViewCount,
               CreatedAtPlatform: twitchClip.CreatedAt,
           })
       }

       // Store clips
       for _, clip := range clips {
           f.repo.CreateOrUpdateClip(clip)
       }

       return clips, nil
   }
   ```

2. **Ranking Algorithm**:
   ```go
   func (r *Ranker) RankClips(clips []Clip, session StreamSession) []Clip {
       for i, clip := range clips {
           score := 0.0

           // View count score (normalized 0-100)
           maxViews := findMaxViews(clips)
           viewScore := (float64(clip.ViewCount) / float64(maxViews)) * 100
           score += viewScore * 0.6 // 60% weight

           // Recency score (clips created early in stream rank higher)
           timeSinceStart := clip.CreatedAtPlatform.Sub(session.StartedAt).Seconds()
           streamDuration := session.EndedAt.Sub(session.StartedAt).Seconds()
           recencyScore := (1 - (timeSinceStart / streamDuration)) * 100
           score += recencyScore * 0.3 // 30% weight

           // Duration score (prefer 15-45 second clips)
           idealDuration := 30
           durationDiff := math.Abs(float64(clip.DurationSeconds - idealDuration))
           durationScore := math.Max(0, 100 - (durationDiff * 3))
           score += durationScore * 0.1 // 10% weight

           clips[i].RankScore = score
       }

       // Sort by score descending
       sort.Slice(clips, func(i, j int) bool {
           return clips[i].RankScore > clips[j].RankScore
       })

       return clips
   }
   ```

3. **Selection Algorithm**:
   ```go
   func (r *Ranker) SelectClips(sessionID uuid.UUID, maxClips int) ([]Clip, error) {
       // Get all clips for session
       clips, _ := r.repo.GetClipsBySession(sessionID)

       // Rank them
       rankedClips := r.RankClips(clips, session)

       // Diversify: alternate platforms if multi-platform stream
       selected := []Clip{}
       lastPlatform := ""

       for _, clip := range rankedClips {
           // Skip if same platform as last (unless we're desperate)
           if clip.Platform == lastPlatform && len(rankedClips) > maxClips*2 {
               continue
           }

           selected = append(selected, clip)
           lastPlatform = clip.Platform

           if len(selected) >= maxClips {
               break
           }
       }

       // If we don't have enough clips, fill with user fallback
       if len(selected) < maxClips {
           prefs, _ := r.repo.GetUserPreferences(session.UserID)
           if prefs.FallbackVideoURL != "" {
               // Add fallback as pseudo-clip
               selected = append(selected, Clip{
                   Platform:   "user_upload",
                   ClipURL:    prefs.FallbackVideoURL,
                   IsUserProvided: true,
               })
           }
       }

       return selected, nil
   }
   ```

**Environment Variables**:
```
DATABASE_HOST=localhost
DATABASE_PORT=5432
PORT=8091

TWITCH_CLIENT_ID=...
TWITCH_CLIENT_SECRET=...
KICK_API_KEY=... (if needed)
```

**Deployment**:
- Dockerfile: `services/clip-manager/Dockerfile`
- Kubernetes: `deployments/k8s/base/clip-manager/`
- Cron Job: Daily job to refresh view counts for recent clips

---

### Service 3: Credit Roll Generator Service (Port 8092)

**Responsibility**: Generate credit roll timeline from events and clips

**Key Components**:
```
services/credit-roll-generator/
├── cmd/main.go                  # Service entry point
├── handlers/
│   ├── generate.go              # Generate credit roll
│   ├── preview.go               # Preview endpoint
│   ├── crud.go                  # CRUD operations
│   └── export.go                # Export to JSON/video
├── generator/
│   ├── timeline.go              # Timeline generation logic
│   ├── sections.go              # Section builders (subs, follows, etc.)
│   └── formatter.go             # Text formatting and layout
├── repository/
│   ├── credit_rolls.go          # CRUD for credit_rolls table
│   ├── events.go                # Query events
│   └── clips.go                 # Query clips
└── models/
    ├── credit_roll.go           # Credit roll model
    └── timeline.go              # Timeline structure
```

**API Endpoints**:

```
# Generation
POST   /api/v1/credit-rolls/generate           # Generate new credit roll
       Body: { session_id, config }
       Returns: { credit_roll_id, preview_url }

GET    /api/v1/credit-rolls/:id/preview        # Get preview data
GET    /api/v1/credit-rolls/:id/timeline       # Get full timeline JSON

# CRUD
GET    /api/v1/credit-rolls                    # List user's credit rolls
GET    /api/v1/credit-rolls/:id                # Get credit roll
PUT    /api/v1/credit-rolls/:id                # Update configuration
DELETE /api/v1/credit-rolls/:id                # Delete credit roll

# Publishing
POST   /api/v1/credit-rolls/:id/publish        # Mark as published
GET    /api/v1/credit-rolls/:id/embed          # Get embed code/URL

# Export
GET    /api/v1/credit-rolls/:id/export?format=json   # Export as JSON
GET    /api/v1/credit-rolls/:id/export?format=pdf    # Export as PDF (future)

# Health
GET    /health/live
GET    /health/ready
```

**Core Logic**:

1. **Generate Credit Roll**:
   ```go
   func (g *Generator) Generate(req GenerateRequest) (*CreditRoll, error) {
       // 1. Fetch events for time range
       events, err := g.eventRepo.GetEventsByTimeRange(
           req.SessionID,
           req.EventStartTime,
           req.EventEndTime,
       )

       // 2. Group events by type
       grouped := g.groupEvents(events)

       // 3. Select clips
       clips, err := g.clipRepo.SelectClips(req.SessionID, req.Config.MaxClips)

       // 4. Build timeline
       timeline := g.buildTimeline(grouped, clips, req.Config)

       // 5. Create credit roll record
       creditRoll := CreditRoll{
           UserID:          req.UserID,
           StreamSessionID: req.SessionID,
           Name:            req.Name,
           EventStartTime:  req.EventStartTime,
           EventEndTime:    req.EventEndTime,
           Config:          req.Config,
           GeneratedData: map[string]interface{}{
               "timeline":       timeline,
               "total_duration": timeline.TotalDuration(),
               "event_counts":   g.getEventCounts(grouped),
           },
           Status: "generated",
       }

       g.repo.Create(&creditRoll)

       return &creditRoll, nil
   }
   ```

2. **Build Timeline**:
   ```go
   func (g *Generator) buildTimeline(events map[string][]Event, clips []Clip, config Config) Timeline {
       timeline := Timeline{}
       currentTime := 0.0 // seconds

       // Intro section (static)
       timeline.AddEntry(TimelineEntry{
           Type:     "title",
           Start:    currentTime,
           Duration: 5.0,
           Content: map[string]interface{}{
               "text":      "Stream Highlights",
               "subtitle":  formatDate(config.EventStartTime),
               "style":     "large",
           },
       })
       currentTime += 5.0

       // Interleave clips and sections
       clipIndex := 0
       for _, section := range config.Sections {
           if !section.Enabled {
               continue
           }

           // Add clip before section (if available)
           if clipIndex < len(clips) {
               clip := clips[clipIndex]
               timeline.AddEntry(TimelineEntry{
                   Type:     "video",
                   Start:    currentTime,
                   Duration: float64(clip.DurationSeconds),
                   Content: map[string]interface{}{
                       "clip_id":   clip.ID,
                       "embed_url": clip.EmbedURL,
                   },
               })
               currentTime += float64(clip.DurationSeconds)
               clipIndex++
           }

           // Build section
           sectionEntry := g.buildSection(section.Type, events[section.Type], config)
           sectionEntry.Start = currentTime
           timeline.AddEntry(sectionEntry)
           currentTime += sectionEntry.Duration
       }

       // Outro
       timeline.AddEntry(TimelineEntry{
           Type:     "title",
           Start:    currentTime,
           Duration: 5.0,
           Content: map[string]interface{}{
               "text":  "Thanks for Watching!",
               "style": "large",
           },
       })

       return timeline
   }
   ```

3. **Build Section (e.g., Subscribers)**:
   ```go
   func (g *Generator) buildSection(sectionType string, events []Event, config Config) TimelineEntry {
       switch sectionType {
       case "subscribers":
           // Sort alphabetically or by tier
           sort.Slice(events, func(i, j int) bool {
               return events[i].DisplayName < events[j].DisplayName
           })

           // Format entries
           entries := []string{}
           for _, event := range events {
               tier := event.Metadata["tier"]
               months := event.Metadata["months"]
               entries = append(entries, fmt.Sprintf(
                   "%s (Tier %s, %d months)",
                   event.DisplayName,
                   tier,
                   months,
               ))
           }

           // Calculate duration (5 seconds per 10 names)
           duration := math.Max(10.0, float64(len(entries)) * 0.5)

           return TimelineEntry{
               Type:     "section",
               Duration: duration,
               Content: map[string]interface{}{
                   "section_type": "subscribers",
                   "title":        "Thank You Subscribers",
                   "entries":      entries,
                   "scroll_speed": config.Styling.ScrollSpeed,
               },
           }

       case "bits":
           // Sort by amount descending
           sort.Slice(events, func(i, j int) bool {
               return events[i].Metadata["amount"].(int) > events[j].Metadata["amount"].(int)
           })

           entries := []string{}
           for _, event := range events {
               entries = append(entries, fmt.Sprintf(
                   "%s - %d bits",
                   event.DisplayName,
                   event.Metadata["amount"],
               ))
           }

           duration := math.Max(10.0, float64(len(entries)) * 0.5)

           return TimelineEntry{
               Type:     "section",
               Duration: duration,
               Content: map[string]interface{}{
                   "section_type": "bits",
                   "title":        "Top Bits Supporters",
                   "entries":      entries,
               },
           }

       // ... similar logic for follows, raids, chatters, etc.
       }
   }
   ```

4. **Timeline Structure** (returned to frontend):
   ```json
   {
     "timeline": [
       {
         "type": "title",
         "start": 0,
         "duration": 5,
         "content": {
           "text": "Stream Highlights",
           "subtitle": "November 19, 2025",
           "style": "large"
         }
       },
       {
         "type": "video",
         "start": 5,
         "duration": 30,
         "content": {
           "clip_id": "uuid",
           "embed_url": "https://clips.twitch.tv/embed?clip=...",
           "thumbnail_url": "https://..."
         }
       },
       {
         "type": "section",
         "start": 35,
         "duration": 45,
         "content": {
           "section_type": "subscribers",
           "title": "Thank You Subscribers",
           "entries": [
             "Alice (Tier 1, 12 months)",
             "Bob (Tier 3, 24 months)",
             "Charlie (Tier 1, 1 month)"
           ],
           "scroll_speed": "medium"
         }
       },
       {
         "type": "video",
         "start": 80,
         "duration": 28,
         "content": {
           "clip_id": "uuid2",
           "embed_url": "https://..."
         }
       },
       {
         "type": "section",
         "start": 108,
         "duration": 30,
         "content": {
           "section_type": "bits",
           "title": "Top Bits Supporters",
           "entries": [
             "Dave - 1000 bits",
             "Eve - 500 bits",
             "Frank - 250 bits"
           ]
         }
       },
       {
         "type": "title",
         "start": 138,
         "duration": 5,
         "content": {
           "text": "Thanks for Watching!",
           "style": "large"
         }
       }
     ],
     "total_duration_seconds": 143,
     "event_counts": {
       "subscribers": 3,
       "followers": 15,
       "bits": 3,
       "unique_chatters": 87
     }
   }
   ```

**Environment Variables**:
```
DATABASE_HOST=localhost
DATABASE_PORT=5432
PORT=8092
```

**Deployment**:
- Dockerfile: `services/credit-roll-generator/Dockerfile`
- Kubernetes: `deployments/k8s/base/credit-roll-generator/`

**Real-Time Generation** (on-demand when overlay loads):
- No background jobs or pre-generation needed
- Timeline built in ~50-200ms when OBS scene activates
- Uses current stream session (status='live') or most recent session
- Clips fetched from cache (populated by Clip Manager background task)

---

## User Flows

### 🎯 TL;DR - The Live Credit Roll Experience

**The Magic**: User configures settings ONCE and adds Browser Source to OBS, then:
1. ✅ **During stream**: Events automatically collected in real-time
2. ✅ **Ready to end**: Streamer switches to "Ending Soon" scene in OBS
3. ✅ **Credit roll plays LIVE**: Shows today's subs, follows, bits, clips (2-3 min)
4. ✅ **Viewers engaged**: Beautiful Hollywood-style credits instead of boring "BRB" screen
5. ✅ **Stream ends**: Streamer raids or ends stream

**User interaction per stream**: Just switch to "Ending Soon" scene in OBS 🎉

**Setup vs. Daily Use**:
- **First time**: ~5 minutes to configure + add to OBS
- **Every stream**: Just click scene in OBS when ready to end
- **Zero maintenance**: Same URL shows different content every stream

---

### Flow 1: One-Time Setup + OBS Integration

**Goal**: User configures preferences ONCE and adds Browser Source to OBS "Ending Soon" scene

**Key Principle**: ⚡ **Set up once → Use at end of every stream** ⚡

```
┌─────────────┐
│   User      │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────┐
│ 1. Navigate to "Credit Roll" settings page  │
│    (ONE TIME ONLY)                           │
└──────┬──────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 2. Grant additional OAuth scopes (if needed):    │
│    ✓ Twitch: channel:read:subscriptions, etc.   │
│    ✓ YouTube: yt-analytics.readonly             │
│    ✓ Kick: (existing scopes sufficient)         │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 3. Configure AUTO-GENERATION settings:           │
│    ┌──────────────────────────────────────────┐ │
│    │ 🤖 Automatic Generation:                 │ │
│    │  [✓] Auto-generate credit roll after    │ │
│    │      each stream ends                    │ │
│    │  [✓] Auto-publish (skip manual review)  │ │
│    │  [✓] Send notification when ready        │ │
│    └──────────────────────────────────────────┘ │
│    ┌──────────────────────────────────────────┐ │
│    │ Event Types to Include:                  │ │
│    │  [x] Subscribers                         │ │
│    │  [x] Followers                           │ │
│    │  [x] Bits/Super Chats                    │ │
│    │  [x] Raids                               │ │
│    │  [x] Unique Chatters                     │ │
│    │  [ ] Channel Point Redemptions           │ │
│    └──────────────────────────────────────────┘ │
│    ┌──────────────────────────────────────────┐ │
│    │ Clip Settings:                           │ │
│    │  • Selection mode: [Auto ▼]             │ │
│    │  • Max clips: [5]                        │ │
│    │  • Prefer recent: [✓]                    │ │
│    │  • Fallback URL: [________________]      │ │
│    │                  (YouTube/Vimeo link)    │ │
│    └──────────────────────────────────────────┘ │
│    ┌──────────────────────────────────────────┐ │
│    │ Styling (applied to all credit rolls):  │ │
│    │  • Font: [Inter ▼]                       │ │
│    │  • Text color: [#FFFFFF]                 │ │
│    │  • Scroll speed: [Medium ▼]             │ │
│    │  • Background overlay: [40% opacity]     │ │
│    └──────────────────────────────────────────┘ │
│    ┌──────────────────────────────────────────┐ │
│    │ 📧 Notifications:                        │ │
│    │  • Email: [user@example.com]            │ │
│    │  • Discord webhook: [____________]      │ │
│    └──────────────────────────────────────────┘ │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 4. Click "Save Settings"                         │
│    → API: PUT /api/v1/credit-roll/settings       │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 5. System generates overlay URL and shows:       │
│                                                  │
│    ✅ Settings saved!                            │
│                                                  │
│    📺 Your Credit Roll Overlay URL:             │
│    https://allchat.live/overlay/credit-roll/    │
│    user-abc123                                   │
│                                                  │
│    This URL will show TODAY'S events when you   │
│    play it during your stream!                  │
│                                                  │
│    [Copy URL] [Setup OBS →]                     │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 6. Add to OBS "Ending Soon" Scene:               │
│                                                  │
│    ┌──────────────────────────────────────┐    │
│    │ 1. Open OBS Studio                   │    │
│    │ 2. Select "Ending Soon" scene        │    │
│    │    (or create new scene)             │    │
│    │ 3. Add Source → Browser              │    │
│    │ 4. Paste URL from All-Chat           │    │
│    │ 5. Set size: 1920x1080               │    │
│    │ 6. ✓ Shutdown when not visible       │    │
│    │ 7. ✓ Refresh when scene activates    │    │
│    └──────────────────────────────────────┘    │
└──────┬───────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────┐
│ 7. Done! You're ready to use it!                 │
│                                                  │
│    💡 How to use during stream:                  │
│    1. When ready to end stream →                │
│       Switch to "Ending Soon" scene in OBS      │
│    2. Credit roll plays with TODAY'S events     │
│    3. After 2-3 min → Raid or end stream        │
│                                                  │
│    ✨ Same URL works every stream with          │
│       different content!                        │
└──────────────────────────────────────────────────┘
```

**Backend Actions**:
1. Store settings in `user_credit_roll_settings` table
2. Generate persistent overlay URL: `/overlay/credit-roll/{user_id}`
3. Event Collector automatically tracks events during ALL future streams
4. Overlay fetches TODAY'S events when loaded (real-time query)

**User Benefit**: ⭐ **Set once + add to OBS once → Use at end of every stream** - Just switch scenes!

---

### Flow 2: During Stream (Automatic Event Collection)

**Goal**: Transparently collect events in the background

```
┌──────────────┐
│ User streams │
└──────┬───────┘
       │ (User does nothing - automatic)
       │
       ▼
┌────────────────────────────────────────────────────────┐
│ Event Collector Service (Background Process)           │
│                                                         │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 1. Detect stream start:                         │  │
│  │    • Listen to Twitch EventSub: stream.online   │  │
│  │    • Or user manually starts session via UI     │  │
│  └────────────────┬────────────────────────────────┘  │
│                   ▼                                    │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 2. Create stream_session record in DB          │  │
│  │    • session_id: uuid                           │  │
│  │    • started_at: now()                          │  │
│  │    • status: 'live'                             │  │
│  └────────────────┬────────────────────────────────┘  │
│                   ▼                                    │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 3. Collect events in real-time:                │  │
│  │    • Twitch: EventSub WebSocket                 │  │
│  │    • YouTube: Extract from Live Chat API       │  │
│  │    • Kick: Pusher WebSocket                     │  │
│  │    • TikTok: Webhook/WebSocket                  │  │
│  └────────────────┬────────────────────────────────┘  │
│                   ▼                                    │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 4. For each event:                              │  │
│  │    • Normalize to unified format                │  │
│  │    • Store in stream_events table               │  │
│  │    • Update session.stats (real-time counters)  │  │
│  │    • Update Redis leaderboards                  │  │
│  └────────────────┬────────────────────────────────┘  │
│                   ▼                                    │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 5. User can view live stats (optional):        │  │
│  │    • Dashboard shows: "15 new followers today!" │  │
│  │    • "87 unique chatters"                       │  │
│  │    • "Total bits: 1,250"                        │  │
│  └─────────────────────────────────────────────────┘  │
│                                                         │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 6. Detect stream end:                           │  │
│  │    • EventSub: stream.offline                   │  │
│  │    • Or user manually ends session              │  │
│  │    • Update: session.ended_at = now()           │  │
│  │    • Update: session.status = 'ended'           │  │
│  └─────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**User Experience**:
- ✅ Zero interaction required during stream
- ✅ Optional real-time dashboard to see stats
- ✅ Fully transparent background collection

---

### Flow 3: Using Credit Roll LIVE During Stream

**Goal**: Play credit roll at end of stream to engage viewers while wrapping up

**Key Principle**: 🎬 **Switch to scene → Credits play with TODAY'S events** 🎬

```
┌────────────────────────────────┐
│ Streamer is live for 3 hours  │
│ (Events collected automatically)│
└──────┬─────────────────────────┘
       │
       │ [Events collected so far: 15 subs, 42 follows, 1,250 bits, 87 chatters]
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ 🎮 Streamer: "Alright chat, we're wrapping up!"        │
│    "Let's see who made today awesome!"                  │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ 📺 Streamer clicks "Ending Soon" scene in OBS          │
│    (Browser Source loads overlay URL)                   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ OVERLAY LOADS (Real-time query)                         │
│                                                          │
│  ┌───────────────────────────────────────────────────┐ │
│  │ 1. Browser Source activates                       │ │
│  │    → GET /api/v1/overlay/credit-roll/user-123    │ │
│  └────────────────┬──────────────────────────────────┘ │
│                   ▼                                     │
│  ┌───────────────────────────────────────────────────┐ │
│  │ 2. Backend queries TODAY'S events                 │ │
│  │    • Get current stream_session_id (status=live)  │ │
│  │    • Query stream_events WHERE session_id=...     │ │
│  │    • Group by type (subs, follows, bits, etc.)    │ │
│  │    Duration: ~100ms                               │ │
│  └────────────────┬──────────────────────────────────┘ │
│                   ▼                                     │
│  ┌───────────────────────────────────────────────────┐ │
│  │ 3. Fetch clips for background                     │ │
│  │    • Use clips from PREVIOUS streams (cached)     │ │
│  │    • OR use user's fallback video                 │ │
│  │    • OR generate gradient background              │ │
│  │    Duration: ~50ms (clips pre-fetched)            │ │
│  └────────────────┬──────────────────────────────────┘ │
│                   ▼                                     │
│  ┌───────────────────────────────────────────────────┐ │
│  │ 4. Generate timeline on-the-fly                   │ │
│  │    • Apply user's saved section config            │ │
│  │    • Interleave clips and credit sections         │ │
│  │    • Build timeline JSON                          │ │
│  │    Duration: ~50ms                                │ │
│  └────────────────┬──────────────────────────────────┘ │
│                   ▼                                     │
│  ┌───────────────────────────────────────────────────┐ │
│  │ 5. Return timeline to overlay frontend           │ │
│  │    Total load time: ~200ms                        │ │
│  └───────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ 🎬 CREDIT ROLL PLAYS LIVE ON STREAM                     │
│                                                          │
│  [0:00 - 0:05] Intro: "Today's Stream Highlights"      │
│                                                          │
│  [0:05 - 0:35] 🎥 Clip from previous stream plays      │
│                                                          │
│  [0:35 - 1:20] 📜 Subscribers Section:                  │
│                "Thank You Subscribers!"                  │
│                • Alice (Tier 1, 12 months)              │
│                • Bob (Tier 3, 24 months)                │
│                • Charlie (Tier 1, 1 month)              │
│                ... [scrolling credits]                   │
│                                                          │
│  [1:20 - 1:48] 🎥 Another clip plays                    │
│                                                          │
│  [1:48 - 2:18] 📜 Top Bits Supporters:                  │
│                • Dave - 1,000 bits                       │
│                • Eve - 500 bits                          │
│                • Frank - 250 bits                        │
│                                                          │
│  [2:18 - 2:48] 📜 New Followers (42 people):            │
│                [Names scrolling in 2 columns...]        │
│                                                          │
│  [2:48 - 3:00] 📜 Amazing Chatters (87 people)          │
│                                                          │
│  [3:00 - 3:05] Outro: "Thanks for Watching!"           │
│                                                          │
│  💬 Chat reacts:                                         │
│     "Ayy I'm in there!" "PogChamp" "GGs everyone!"     │
└─────────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ 🎮 Streamer: "Thanks everyone! Let's raid XYZ!"        │
│    Clicks raid button or ends stream                    │
└─────────────────────────────────────────────────────────┘
```

**Timeline**:
- Streamer clicks scene: `00:00`
- Overlay loads: `+0.2s` (real-time query)
- Credits start playing: `+0.3s`
- Credits finish: `+3:00` (depends on event count)
- Streamer raids/ends: `+3:05`

**User Actions Required**: **ONE CLICK** (switch to scene in OBS) ⭐

**What Viewers See**:
1. Beautiful Hollywood-style credits
2. TODAY'S subscribers, followers, bits, chatters
3. Clips from previous streams playing in background
4. Professional end-of-stream experience

**What Streamer Does**:
- Click "Ending Soon" scene in OBS
- Chat with viewers while credits roll
- Raid or end stream when done

**Technical Magic**:
- Same overlay URL every stream
- Content updates in real-time based on TODAY'S active session
- No pre-generation needed - built on-demand when scene activates
- ~200ms load time (feels instant)

---

## Overlay Frontend Implementation

**Technical Details** (Overlay URL: `/overlay/credit-roll/{user_id}`):

```javascript
// Overlay page: /overlay/credit-roll/:userId
// Built with React + Canvas/WebGL
// Loads LIVE data when scene activates in OBS

function CreditRollOverlay({ userId }) {
  const [timeline, setTimeline] = useState(null);
  const [loading, setLoading] = useState(true);
  const canvasRef = useRef(null);

  useEffect(() => {
    // Fetch TODAY'S events and generate timeline in real-time
    async function loadCreditRoll() {
      try {
        // API queries current stream session and builds timeline on-the-fly
        const response = await fetch(`/api/v1/overlay/credit-roll/${userId}`);
        const data = await response.json();

        // Response includes:
        // - timeline: Complete timeline with clips + credit sections
        // - session_info: Stream title, date, stats
        // - clips: Pre-selected clips from previous streams

        setTimeline(data);
        setLoading(false);

        // Start playback immediately
        startCreditRoll(data.timeline);
      } catch (error) {
        console.error('Failed to load credit roll:', error);
        // Show error state or fallback
      }
    }

    loadCreditRoll();
  }, [userId]);

  useEffect(() => {
    if (!timeline) return;

    // Preload video clips for smooth playback
    timeline.clips.forEach(clip => {
      const video = document.createElement('video');
      video.src = clip.embed_url;
      video.preload = 'auto';
    });
  }, [timeline]);

  const startCreditRoll = (timelineData) => {
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');

    // Initialize video players, text renderers, animation loop
    // - Background video playback with crossfades
    // - Scrolling credit text over video
    // - Synchronized timeline playback
  };

  if (loading) {
    return <div className="loading">Loading today's highlights...</div>;
  }

  return (
    <div className="credit-roll-overlay">
      <canvas ref={canvasRef} width={1920} height={1080} />
    </div>
  );
}
```

**Key API Endpoint** (real-time generation):

```go
// GET /api/v1/overlay/credit-roll/:userId
func (h *Handler) GetCreditRollOverlay(c *gin.Context) {
    userID := c.Param("userId")

    // 1. Get current active stream session for this user
    session, err := h.repo.GetActiveSession(userID)
    if err != nil {
        // No active stream - use most recent ended session
        session, _ = h.repo.GetMostRecentSession(userID)
    }

    // 2. Get user's credit roll settings
    settings, _ := h.repo.GetUserSettings(userID)

    // 3. Query events for this session
    events, _ := h.repo.GetEventsBySession(session.ID)

    // 4. Get cached clips (from previous streams)
    clips, _ := h.clipRepo.GetTopClips(userID, settings.MaxClips)

    // 5. Build timeline on-the-fly
    timeline := h.generator.BuildTimeline(events, clips, settings)

    // 6. Return complete overlay data
    c.JSON(200, gin.H{
        "timeline": timeline,
        "session_info": session,
        "clips": clips,
        "duration_seconds": timeline.TotalDuration(),
    })
}
```

**Performance**:
- Query time: ~100ms (indexed DB queries)
- Timeline generation: ~50ms (in-memory processing)
- Total response time: ~150-200ms (feels instant to user)
- No pre-generation or background jobs needed for basic functionality

---

## Implementation Phases

### Phase 1: Foundation & Twitch Events (Weeks 1-3)

**Objective**: Build core event collection infrastructure with Twitch as first platform

**Tasks**:
1. Database setup
   - [ ] Create migration with 5 new tables
   - [ ] Add indexes and constraints
   - [ ] Test with seed data

2. Event Collector Service - Skeleton
   - [ ] Create service structure (`services/event-collector/`)
   - [ ] Add health checks
   - [ ] DB + Redis connection setup

3. Twitch EventSub Integration
   - [ ] Implement EventSub WebSocket client
   - [ ] Subscribe to event types (follow, sub, cheer, raid)
   - [ ] Webhook receiver (alternative to WebSocket)
   - [ ] Signature validation

4. Event Normalization - Twitch
   - [ ] Implement `TwitchNormalizer`
   - [ ] Map all Twitch event types to unified model
   - [ ] Handle edge cases (gift subs, resubs, etc.)

5. Event Storage & Statistics
   - [ ] CRUD repository for `stream_events`
   - [ ] Real-time stats aggregation (`stream_sessions.stats`)
   - [ ] Redis leaderboards (bits, subs)

6. API Endpoints
   - [ ] `GET /api/v1/sessions/:id/events`
   - [ ] `GET /api/v1/sessions/:id/events/stats`
   - [ ] `POST /webhooks/twitch`

7. Testing
   - [ ] Unit tests for normalizer
   - [ ] Integration tests with mock EventSub
   - [ ] Load testing (1000 events/sec)

**Deliverables**:
- ✅ Event Collector Service collecting Twitch events
- ✅ Events stored in database with unified format
- ✅ API to query events and stats

**Milestone**: Can track all Twitch events for a live stream

---

### Phase 2: Multi-Platform Events (Weeks 4-6)

**Objective**: Extend event collection to YouTube, Kick, TikTok

**Tasks**:
1. YouTube Event Extraction
   - [ ] Integrate with existing YouTube Listener
   - [ ] Extract Super Chats from Live Chat messages
   - [ ] Extract memberships (first occurrence of sponsor badge)
   - [ ] Track unique chatters
   - [ ] (Future) YouTube Analytics API for subscriber data

2. Kick Event Collection
   - [ ] Integrate with existing Kick Listener
   - [ ] Listen for subscription events on Pusher
   - [ ] Listen for gift events
   - [ ] Investigate follower events

3. TikTok Event Collection (if Listener ready)
   - [ ] Implement webhook receiver for TikTok Events API
   - [ ] Handle gift events, follows, likes, shares
   - [ ] Signature validation

4. Normalizers for All Platforms
   - [ ] `YouTubeNormalizer`
   - [ ] `KickNormalizer`
   - [ ] `TikTokNormalizer`
   - [ ] Ensure consistent metadata structure

5. Stream Session Management
   - [ ] Auto-detect stream start/end (via EventSub)
   - [ ] Manual session creation API
   - [ ] Multi-platform session support (one session, multiple platforms)

6. Event Dashboard UI (Frontend)
   - [ ] List recent streams
   - [ ] Real-time event feed (WebSocket)
   - [ ] Statistics cards (subs, follows, bits, etc.)
   - [ ] Leaderboards (top bits, top chatters)

7. Testing
   - [ ] Test each platform normalizer
   - [ ] End-to-end test: multi-platform stream
   - [ ] Performance testing

**Deliverables**:
- ✅ All 4 platforms (Twitch, YouTube, Kick, TikTok) collecting events
- ✅ Frontend dashboard showing live stats
- ✅ Unified event format across platforms

**Milestone**: Can track events from all platforms simultaneously

---

### Phase 3: Clip Management (Weeks 7-9)

**Objective**: Fetch, rank, and manage clips from platforms

**Tasks**:
1. Clip Manager Service - Skeleton
   - [ ] Create service structure (`services/clip-manager/`)
   - [ ] DB connection setup
   - [ ] Health checks

2. Twitch Clip Fetcher
   - [ ] Implement Helix API client
   - [ ] Fetch clips for session (by date range)
   - [ ] Store clip metadata in `clips` table
   - [ ] Handle pagination (100 clips per request)

3. Kick Clip Fetcher
   - [ ] Investigate Kick API for clips
   - [ ] Implement fetcher (if API available)
   - [ ] Fallback: user-provided clips only

4. Clip Ranking Algorithm
   - [ ] Implement ranking logic (views, recency, duration)
   - [ ] Store `rank_score` in database
   - [ ] Selection algorithm with diversification

5. User Clip Preferences
   - [ ] CRUD API for `user_clip_preferences`
   - [ ] Fallback video URL support
   - [ ] Validation (check if URL is accessible)

6. Clip Management UI (Frontend)
   - [ ] List clips for session (with thumbnails)
   - [ ] Play clip preview
   - [ ] Manual selection (add/remove clips)
   - [ ] Upload custom clip (user-provided video)
   - [ ] Set fallback video URL

7. API Endpoints
   - [ ] `POST /api/v1/clips/fetch/:session_id`
   - [ ] `GET /api/v1/clips?session_id=X`
   - [ ] `POST /api/v1/clips/select` (run selection algorithm)
   - [ ] `POST /api/v1/clips` (upload custom clip)
   - [ ] `GET/PUT /api/v1/clips/preferences`

8. Testing
   - [ ] Test clip fetching from Twitch
   - [ ] Test ranking algorithm (various scenarios)
   - [ ] Test selection with edge cases (0 clips, 1 clip, 100 clips)

**Deliverables**:
- ✅ Clip Manager Service fetching and ranking clips
- ✅ Frontend UI for reviewing and selecting clips
- ✅ User preferences for clip behavior

**Milestone**: Can automatically select best clips for a stream

---

### Phase 4: Credit Roll Generator (Weeks 10-12)

**Objective**: Generate credit roll timeline from events and clips

**Tasks**:
1. Credit Roll Generator Service - Skeleton
   - [ ] Create service structure (`services/credit-roll-generator/`)
   - [ ] DB connection setup
   - [ ] Health checks

2. Section Builders
   - [ ] `SubscribersSection` (sorted alphabetically or by tier)
   - [ ] `BitsSection` (sorted by amount)
   - [ ] `FollowersSection` (alphabetical)
   - [ ] `ChattersSection` (top chatters by message count)
   - [ ] `RaidsSection` (sorted by viewer count)
   - [ ] Generic section builder interface

3. Timeline Generation Logic
   - [ ] Interleave clips and sections
   - [ ] Calculate durations (based on entry count)
   - [ ] Add intro/outro
   - [ ] Generate JSON timeline structure

4. Credit Roll CRUD
   - [ ] `POST /api/v1/credit-rolls/generate`
   - [ ] `GET /api/v1/credit-rolls/:id`
   - [ ] `PUT /api/v1/credit-rolls/:id` (update config)
   - [ ] `DELETE /api/v1/credit-rolls/:id`
   - [ ] `POST /api/v1/credit-rolls/:id/publish`

5. Configuration System
   - [ ] Section enable/disable
   - [ ] Section reordering (user-defined)
   - [ ] Styling options (font, colors, speed)
   - [ ] Music settings

6. Credit Roll Configuration UI (Frontend)
   - [ ] Wizard/modal for generating roll
   - [ ] Section selection checkboxes
   - [ ] Drag-and-drop section reordering
   - [ ] Styling customization form
   - [ ] Clip selection integration

7. Preview API
   - [ ] `GET /api/v1/credit-rolls/:id/timeline` (full JSON)
   - [ ] `GET /api/v1/credit-rolls/:id/preview` (summary)

8. Testing
   - [ ] Test timeline generation with various configs
   - [ ] Test section builders (edge cases: 0 events, 1 event, 1000 events)
   - [ ] Test ordering and interleaving logic

**Deliverables**:
- ✅ Credit Roll Generator Service creating timelines
- ✅ Frontend UI for configuring credit rolls
- ✅ JSON timeline structure ready for frontend rendering

**Milestone**: Can generate complete credit roll configuration

---

### Phase 5: Overlay Display (Weeks 13-15)

**Objective**: Build frontend overlay to display credit roll

**Tasks**:
1. Overlay Page Setup
   - [ ] Create `/overlay/credit-roll/:id` route
   - [ ] Fetch timeline data from API
   - [ ] Preload video clips and assets

2. Video Background Player
   - [ ] HTML5 `<video>` player for clips
   - [ ] Crossfade transitions between clips
   - [ ] Loop handling (if clips shorter than credits)
   - [ ] Fallback to user-provided video
   - [ ] Fallback to gradient background

3. Credit Roll Renderer (Canvas/WebGL)
   - [ ] Scrolling text animation
   - [ ] Section headers (larger text)
   - [ ] Smooth 60 FPS scrolling
   - [ ] Text shadows/outlines for readability
   - [ ] Configurable scroll speed

4. Timeline Playback Engine
   - [ ] Parse timeline JSON
   - [ ] Synchronize video playback with credit scroll
   - [ ] Handle transitions (video → section → video)
   - [ ] Intro/outro screens

5. Styling System
   - [ ] Apply user-defined fonts
   - [ ] Apply user-defined colors
   - [ ] Background overlay opacity
   - [ ] Responsive to resolution (1920x1080, 1280x720)

6. Music Integration
   - [ ] Audio player for background music
   - [ ] Volume control
   - [ ] Crossfade with video audio (if clips have audio)

7. Preview Player (Dashboard)
   - [ ] Embedded preview in configuration UI
   - [ ] Timeline scrubber
   - [ ] Play/pause controls
   - [ ] Full-screen preview mode

8. OBS Integration Guide
   - [ ] Documentation page: "How to Add Credit Roll to OBS"
   - [ ] Copy embed URL button
   - [ ] Copy embed code button (HTML iframe)
   - [ ] Recommended OBS settings

9. Testing
   - [ ] Test with various timeline structures
   - [ ] Test with 0 clips (fallback video only)
   - [ ] Test with long credit lists (100+ names)
   - [ ] Performance testing (60 FPS maintained)
   - [ ] Cross-browser testing (Chrome, Firefox, Safari)

**Deliverables**:
- ✅ Fully functional credit roll overlay
- ✅ Smooth animations and transitions
- ✅ OBS-compatible browser source
- ✅ Preview player in dashboard

**Milestone**: Can display credit roll in OBS

---

### Phase 6: Polish, Testing, Launch (Weeks 16-18)

**Objective**: Production readiness, documentation, launch

**Tasks**:
1. End-to-End Testing
   - [ ] Full user flow: setup → stream → generate → display
   - [ ] Multi-platform test (Twitch + YouTube + Kick)
   - [ ] Edge cases: 0 events, 1000+ events, no clips
   - [ ] Performance testing: 3-hour stream, 1000 viewers

2. Security Audit
   - [ ] Review OAuth scope requirements
   - [ ] Webhook signature validation
   - [ ] SQL injection prevention (parameterized queries)
   - [ ] XSS prevention in overlay (sanitize user input)
   - [ ] Rate limiting on APIs

3. Error Handling & Logging
   - [ ] Graceful degradation (missing clips → fallback)
   - [ ] User-friendly error messages
   - [ ] Structured logging (Zap) in all services
   - [ ] Error tracking (Sentry or similar)

4. Documentation
   - [ ] API documentation (Swagger/OpenAPI)
   - [ ] User guide: "Getting Started with Credit Rolls"
   - [ ] Video tutorials (YouTube)
   - [ ] FAQ page
   - [ ] Developer docs (architecture, contributing)

5. UI/UX Polish
   - [ ] Consistent styling across pages
   - [ ] Loading states and spinners
   - [ ] Empty states ("No clips found")
   - [ ] Success/error toast notifications
   - [ ] Mobile-responsive admin UI

6. Performance Optimization
   - [ ] Database query optimization (indexes, EXPLAIN)
   - [ ] Redis caching for frequently accessed data
   - [ ] CDN for static assets (clips, thumbnails)
   - [ ] Lazy loading in frontend

7. Monitoring & Observability
   - [ ] Prometheus metrics for all services
   - [ ] Grafana dashboards (event ingestion rate, API latency)
   - [ ] Alerts (high error rate, DB connection failures)
   - [ ] Distributed tracing (OpenTelemetry)

8. Beta Testing
   - [ ] Recruit 10-20 beta testers (streamers)
   - [ ] Gather feedback on UX
   - [ ] Fix critical bugs
   - [ ] Iterate on features

9. Launch Preparation
   - [ ] Marketing page: feature overview
   - [ ] Demo video
   - [ ] Blog post announcement
   - [ ] Social media posts (Twitter, Reddit)

10. Production Deployment
    - [ ] Deploy all services to Kubernetes
    - [ ] Set up autoscaling (HPA)
    - [ ] Database backups
    - [ ] Monitor for 48 hours post-launch

**Deliverables**:
- ✅ Production-ready feature
- ✅ Comprehensive documentation
- ✅ Monitoring and alerts
- ✅ Public launch

**Milestone**: Credit Roll feature live in production!

---

## Technical Challenges & Solutions

### Challenge 1: Real-Time Event Ingestion at Scale

**Problem**: During large streams (10,000+ concurrent viewers), events can spike (e.g., 100 follows/minute during a raid). System must handle bursts without data loss.

**Solutions**:
1. **Redis Streams as Buffer**:
   - Event Collector publishes to Redis Stream (fast writes)
   - Worker processes consume from stream asynchronously
   - Provides backpressure handling

2. **Consumer Groups**:
   - Multiple Event Collector instances in consumer group
   - Horizontal scaling during high load

3. **Batch Inserts**:
   - Accumulate events in memory (max 100 or 1 second)
   - Batch insert to database (reduces DB load)

```go
func (c *Collector) BatchInsert(ctx context.Context) {
    batch := []Event{}
    ticker := time.NewTicker(1 * time.Second)

    for {
        select {
        case event := <-c.eventChan:
            batch = append(batch, event)
            if len(batch) >= 100 {
                c.flushBatch(batch)
                batch = []Event{}
            }
        case <-ticker.C:
            if len(batch) > 0 {
                c.flushBatch(batch)
                batch = []Event{}
            }
        }
    }
}
```

---

### Challenge 2: YouTube Lacks Real-Time Subscriber Events

**Problem**: YouTube Analytics API has 24-48 hour delay. No real-time subscriber notifications.

**Solutions**:
1. **Accept Limitation**:
   - Document that YouTube subscribers appear with delay
   - Focus on real-time events (Super Chats, members)

2. **Daily Batch Job**:
   - Cron job runs daily at 3 AM
   - Fetches previous day's subscriber gains
   - Backfills events with `is_backfilled=true` flag

3. **Alternative: Manual Input**:
   - Allow users to manually add subscriber count
   - "We gained 42 subscribers today!" (user-entered)

**Recommendation**: Accept limitation, focus on Super Chats (which are real-time and more visible)

---

### Challenge 3: Clip Synchronization with Credit Scroll

**Problem**: Need to synchronize video playback with scrolling credits. If video ends before credits finish, need seamless transition to next video.

**Solutions**:
1. **Timeline Absolute Positioning**:
   - Each timeline entry has `start` and `duration` in seconds
   - Video player and credit scroller both reference global playback time

```javascript
function syncPlayback(timeline, currentTime) {
    // Find current video
    const videoEntry = timeline.find(
        entry => entry.type === 'video' &&
        currentTime >= entry.start &&
        currentTime < entry.start + entry.duration
    );

    if (videoEntry) {
        const videoTime = currentTime - videoEntry.start;
        videoPlayer.currentTime = videoTime;
    }

    // Update credit scroll position
    const scrollSpeed = 50; // pixels per second
    const scrollPosition = currentTime * scrollSpeed;
    creditsCanvas.scrollY = scrollPosition;
}
```

2. **Crossfade Transitions**:
   - Preload next video 5 seconds before transition
   - Fade out current video (opacity 1 → 0 over 1 second)
   - Fade in next video (opacity 0 → 1 over 1 second)
   - Overlap videos during transition

3. **Loop Handling**:
   - If credits longer than total video duration, loop last video
   - OR: Transition to fallback video
   - OR: Fade to gradient background

---

### Challenge 4: Large Event Datasets (100+ Followers in One Stream)

**Problem**: Displaying 100+ names in a scrolling credit roll can take 2+ minutes. Too long.

**Solutions**:
1. **Configurable Limits**:
   - User can set "Top N" (e.g., "Top 50 Chatters")
   - Or group by category: "...and 50 other amazing chatters!"

2. **Smart Grouping**:
   - Group followers by first letter: "A: Alice, Andy, Anna | B: Bob, Bella"
   - Reduces visual clutter

3. **Multiple Columns**:
   - Display names in 2-3 columns instead of single column
   - Triples throughput (show 3 names per scroll position)

```javascript
function formatChatters(chatters, maxDisplay = 50) {
    if (chatters.length <= maxDisplay) {
        return chatters.map(c => c.displayName);
    }

    const top = chatters.slice(0, maxDisplay);
    const remaining = chatters.length - maxDisplay;

    return [
        ...top.map(c => c.displayName),
        `...and ${remaining} other amazing chatters!`
    ];
}
```

4. **Scrolling Speed Options**:
   - Slow (40px/s) - leisurely pace
   - Medium (60px/s) - default
   - Fast (80px/s) - for long lists
   - Turbo (120px/s) - speed run mode

---

### Challenge 5: Kick and TikTok Limited Clip Support

**Problem**: Kick clip API may not exist or be undocumented. TikTok doesn't have clips.

**Solutions**:
1. **Fallback Video Priority**:
   - For Kick/TikTok-only streams, emphasize fallback video in UI
   - "Upload a highlight reel or provide a YouTube link"

2. **Stream Replay Segments**:
   - If platform stores VODs, extract random 30-second segments
   - Kick stores VODs (investigate API)
   - TikTok stores recent live replays

3. **Screenshot Slideshows**:
   - As last resort: generate slideshow from stream thumbnails
   - Not ideal, but better than solid color

4. **User-Generated Content**:
   - Encourage users to create clips manually
   - Upload to All-Chat platform
   - Build community clip library

**Recommendation**: Focus on Twitch clips first (best support), then fallback to user content

---

## Future Enhancements

### Phase 2 Features (Post-Launch)

1. **Video Export**:
   - Generate downloadable MP4 file of credit roll
   - Use headless browser (Puppeteer) to record Canvas
   - Store in S3, provide download link
   - Use case: Share on social media

2. **Custom Sections**:
   - Allow users to add custom text sections
   - "Thanks to my mods: Alice, Bob, Charlie"
   - Free-form text editor

3. **Multi-Stream Compilation**:
   - Generate credit roll for entire week/month
   - "Thanks to everyone who joined this month!"
   - Aggregate events across multiple sessions

4. **Template Library**:
   - Pre-made templates (themes): "Retro", "Cyberpunk", "Minimal"
   - One-click apply styling
   - Community-submitted templates

5. **Social Media Integration**:
   - Auto-post to Twitter: "Thanks to my 42 new followers today!"
   - Generate Twitter card image (top supporters)

6. **Streamer Shoutouts**:
   - Section for other streamers (raids, hosts)
   - Display their logos/avatars
   - Link to their channels

7. **Charity Tracking**:
   - Special section for charity donations (via Tiltify, etc.)
   - Highlight top charity donors
   - Show total raised

8. **Live Credit Roll**:
   - Real-time updating credit roll during stream
   - "New follower: Alice" appears immediately
   - Use as continuous overlay, not just end-of-stream

9. **Animation Effects**:
   - Particle effects (confetti for subs)
   - Ken Burns effect on clips (slow zoom/pan)
   - 3D text effects (WebGL shaders)

10. **Accessibility**:
    - Screen reader support
    - High contrast mode
    - Dyslexia-friendly fonts

### Integration Opportunities

1. **StreamElements/Streamlabs**:
   - Export events to StreamElements for alerts
   - Import StreamElements event history

2. **Discord Bot**:
   - Post credit roll link in Discord after stream
   - "Today's stream stats: 42 followers, 15 subs!"

3. **Spotify Integration**:
   - Auto-detect music played during stream
   - Add "Music Credits" section
   - Handle DMCA compliance

4. **Patreon Integration**:
   - Include Patreon supporters in credits
   - Fetch from Patreon API

---

## Success Metrics

### KPIs to Track Post-Launch

1. **Adoption Rate**:
   - % of active users who enable credit roll feature
   - Target: 25% within 3 months

2. **Generation Rate**:
   - Average credit rolls generated per week
   - Target: 1,000 rolls/week after 3 months

3. **Completion Rate**:
   - % of users who complete full setup (preferences → generate → publish)
   - Target: 60% completion

4. **Overlay Usage**:
   - % of generated rolls actually displayed in OBS
   - Track via view_count in credit_rolls table
   - Target: 70% of generated rolls viewed at least once

5. **User Satisfaction**:
   - Survey after generation: "How satisfied are you?"
   - Target: 4.5/5 average rating

6. **Technical Performance**:
   - Event ingestion latency: < 500ms (p99)
   - Credit roll generation time: < 10 seconds
   - Overlay load time: < 2 seconds

---

## Conclusion

This roadmap provides a comprehensive plan for implementing the Hollywood Credit Roll feature across **18 weeks** with **3 new microservices**, **5 database tables**, and **full multi-platform support**.

**Key Highlights**:
- ✅ Supports all 4 platforms (Twitch, YouTube, Kick, TikTok)
- ✅ Real-time event collection with minimal latency
- ✅ Intelligent clip selection and ranking
- ✅ Highly customizable credit roll generation
- ✅ Professional-quality overlay display
- ✅ OBS-compatible browser source
- ✅ Scalable architecture (horizontal scaling)

**Next Steps**:
1. Review and approve roadmap
2. Prioritize phases based on business goals
3. Assign engineers to Phase 1 tasks
4. Set up project board (Jira, GitHub Projects, etc.)
5. Begin implementation with Twitch events (Phase 1)

**Estimated Effort**:
- Backend: 2 engineers × 18 weeks = 36 engineer-weeks
- Frontend: 1 engineer × 18 weeks = 18 engineer-weeks
- **Total**: ~54 engineer-weeks (~13 months for solo dev, ~3 months for team of 3)

Let's build an amazing credit roll feature! 🎬🎉
