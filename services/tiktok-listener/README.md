# TikTok Listener Service

⚠️ **BETA SERVICE** - Uses unofficial TikTok-Live-Connector library

Real-time TikTok LIVE chat monitoring service for All-Chat. Connects to TikTok live streams and publishes chat messages to Redis Streams for processing.

> **Note**: This service is currently in beta development and uses an unofficial library.

## ⚠️ Important Notice

This service uses the **unofficial** [TikTok-Live-Connector](https://github.com/zerodytrash/TikTok-Live-Connector) library, which is based on reverse engineering TikTok's internal WebSocket service.

**Limitations:**
- Not production-ready according to library authors
- May break if TikTok changes their internal APIs
- No official support from TikTok
- Should be replaced when TikTok releases an official Live Chat API

## Features

- ✅ Monitor multiple TikTok live streams simultaneously
- ✅ Real-time chat message capture
- ✅ **Message deduplication** (prevents replay on reconnect using native TikTok message IDs)
- ✅ **Native timestamp preservation** (uses TikTok's original message timestamps)
- ✅ Publishes to Redis Streams (`chat:raw`)
- ✅ Audience events alongside chat: gifts, follows, shares, aggregated likes, and coin chests
- ✅ Dynamic stream management (polls database for active channels)
- ✅ Health check endpoints
- ✅ Graceful shutdown handling
- ✅ TypeScript implementation
- ✅ Resource-optimized with deduplication cache limits

## Architecture

```
┌─────────────────────────────────────────┐
│  TikTok Listener Service (Node.js)      │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  TikTok-Live-Connector Library     │ │
│  │  (Unofficial WebSocket Client)     │ │
│  └────────────────┬───────────────────┘ │
│                   │                      │
│                   ▼                      │
│  ┌────────────────────────────────────┐ │
│  │   Message Handler                  │ │
│  │   - Normalizes to RawChatMessage   │ │
│  │   - Adds overlay_id tagging        │ │
│  └────────────────┬───────────────────┘ │
│                   │                      │
│                   ▼                      │
│  ┌────────────────────────────────────┐ │
│  │   Redis Streams Publisher          │ │
│  │   Stream: chat:raw                 │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Prerequisites

- Node.js 18+
- Redis (for message publishing)
- PostgreSQL (for active stream tracking)

## Installation

```bash
cd services/tiktok-listener
npm install
```

## Configuration

Set the following environment variables:

```bash
# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# PostgreSQL
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Service
PORT=8089
LOG_LEVEL=info           # debug, info, warn, error
LOG_FORMAT=json          # json (default, for production/K8s) or simple (for development)
POLL_INTERVAL_MS=30000   # Poll for active streams every 30 seconds

# Message Deduplication (prevents replay on reconnect)
TIKTOK_DEDUP_TTL_MS=300000           # Keep dedup cache for 5 minutes (default)
TIKTOK_DEDUP_CLEANUP_INTERVAL_MS=60000  # Cleanup interval: 1 minute (default)
TIKTOK_DEDUP_MAX_CACHE_SIZE=10000    # Max messages in dedup cache (default)

# Coin chest (ENVELOPE) classification tracing — unset by default
TIKTOK_ENVELOPE_TRACE=                # Set to any value to log businessType, the display-text
                                      # key and the resulting chest/not-a-chest decision per frame

# WebSocket signing / Euler Stream retirement (see ADR-0052)
TIKTOK_DISABLE_EULER_FALLBACKS=true   # Skip Euler's leg of the room-id and is-live composites
TIKTOK_SIGNER_MODE=euler              # euler | shadow | self
TIKTOK_SELF_SIGN_FALLBACK=true        # Under `self`, fall back to Euler when our signer fails
TIKTOK_SIGNER_URL=                    # Empty = sign in-process; a URL points at a sign service
TIKTOK_EXTENDED_GIFT_INFO=            # Defaults on only under `self` (see below)
SIGN_API_KEY=                         # Euler Stream API key; empty means the free tier
```

### WebSocket signing

`tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed URL, and by default
it gets that signature from **Euler Stream**. Their free tier caps how many rooms we can hold
concurrently and paywalls the gift list, so we are working to sign for ourselves. See
[ADR-0052](../../docs/adr/0052-retiring-euler-stream-for-tiktok-signing.md) for the full
rationale and the trade-off involved.

There are **two independent levers**, deliberately separate because they carry very different
risk:

**`TIKTOK_DISABLE_EULER_FALLBACKS`** (default `true`, safe). Room ID and is-live are resolved by
composites that try TikTok directly — HTML scrape, then the API endpoint — and reach for Euler
only when both have already failed. Turning that last leg off reduces free-tier consumption
immediately and cannot lose a capability.

**`TIKTOK_SIGNER_MODE`** (default `euler`, risky). The signature has no direct-to-TikTok route in
the library, so this is the part we have to build:

| Mode | Who signs the connection | Purpose |
|---|---|---|
| `euler` | Euler Stream | Unchanged behaviour. Our code is not on the connect path. |
| `shadow` | Euler | Our signer runs in parallel against the same room; its outcome is recorded and discarded. Cannot change connection behaviour. |
| `self` | Us | Euler catches failures while `TIKTOK_SELF_SIGN_FALLBACK` is on. |

Walk them in that order. `shadow` exists so the success rate of our own signer can be measured
against Euler's, on live rooms, before anything depends on it — after cutover, a TikTok change to
the signing algorithm takes TikTok ingest down until we fix it, where today it is Euler's problem.

Until the signer itself is implemented, `shadow` and `self` log a warning and fall back to Euler,
so the flag can be set ahead of the code.

`TIKTOK_EXTENDED_GIFT_INFO` defaults on **only** under `self`, because the direct `gift/list/`
route must itself be signed — enabling it any earlier just reinstates Euler's Business-plan error
on every connect.

Signature outcomes are exported as `tiktok_sign_attempts_total{signer,outcome,reason,load_bearing}`
and `tiktok_sign_duration_seconds`. Filter to `load_bearing="true"` for real availability, and to
`load_bearing="false"` for the shadow experiment:

```promql
# Our signer's success rate on live rooms, before we trust it
sum(rate(tiktok_sign_attempts_total{signer="self",outcome="success",load_bearing="false"}[1h]))
  / sum(rate(tiktok_sign_attempts_total{signer="self",load_bearing="false"}[1h]))

# Are we still hitting Euler's free-tier ceiling?
sum(rate(tiktok_sign_attempts_total{reason="rate_limit"}[5m]))
```

## Development

```bash
# Run in development mode with ts-node
npm run dev

# Build TypeScript to JavaScript
npm run build

# Run production build
npm start
```

## Message Format

The service publishes messages to Redis Stream `chat:raw` in the following format:

```json
{
  "message_id": "tiktok_native_msg_id",
  "platform": "tiktok",
  "channel_id": "tiktok_username",
  "stream_id": null,
  "user_id": "unique_user_id",
  "username": "Display Name",
  "text": "Hello from TikTok!",
  "timestamp": "2025-11-15T12:34:56.789Z",
  "tags": {
    "overlay_id": "uuid",
    "user_unique_id": "@username",
    "profile_picture_url": "https://...",
    "is_follower": "true",
    "is_subscriber": "false",
    "badge_level": "0",
    "native_msg_id": "tiktok_native_msg_id",
    "native_create_time": "1731675296"
  }
}
```

**Note**: 
- `message_id` now uses TikTok's native message ID for accurate deduplication
- `timestamp` uses TikTok's original `createTime` (converted from Unix timestamp)
- This prevents duplicate messages from appearing when the service reconnects
    "badge_level": "0"
  }
}
```

## Health Endpoints

- `GET /health/live` - Liveness probe (always returns 200)
- `GET /health/ready` - Readiness probe (checks Redis connection)
- `GET /status` - Service status with active stream count

## Database Requirements

The service queries the `overlay_chat_sources` table to determine which TikTok streams to monitor:

```sql
SELECT DISTINCT
  ocs.overlay_id,
  ocs.channel_id as tiktok_username,
  ocs.is_active
FROM overlay_chat_sources ocs
WHERE ocs.platform = 'tiktok'
  AND ocs.is_active = true
```

**Requirements:**
- `channel_id` should contain the TikTok username (e.g., `@officialgeilegisela` or `officialgeilegisela`)
- `platform` must be `'tiktok'`
- `is_active` must be `true`

## How It Works

1. **Polling**: Every 30 seconds (configurable), the service polls PostgreSQL for active TikTok channels
2. **Connection**: For each active channel, creates a TikTok-Live-Connector instance
3. **Event Listening**: Subscribes to `WebcastEvent.CHAT` events from the library
4. **Message Deduplication**: Uses TikTok's native message ID to detect and skip duplicate messages (important during reconnects)
5. **Timestamp Preservation**: Extracts TikTok's original `createTime` timestamp instead of generating new ones
6. **Message Publishing**: Normalizes chat messages to `RawChatMessage` format and publishes to Redis Stream
7. **Dynamic Management**: Automatically connects to new channels and disconnects from removed ones

## Resource Optimizations

### Message Deduplication

The service implements message deduplication to prevent replayed messages on reconnection:

- **Native Message ID Tracking**: Uses TikTok's `msgId` from the message's `common` property
- **TTL-based Cache**: Keeps track of seen messages for 5 minutes (configurable via `TIKTOK_DEDUP_TTL_MS`)
- **Automatic Cleanup**: Expired entries are cleaned up every minute to prevent memory leaks
- **Size Limits**: Cache limited to 10,000 messages by default (`TIKTOK_DEDUP_MAX_CACHE_SIZE`)

When the service restarts or reconnects, TikTok may replay recent messages. The deduplicator detects these replays and prevents them from being published to Redis.

### Memory Management

- **Increased Memory Limits**: Default memory limit increased to 1GB for better stability
- **Bounded Cache**: Deduplication cache has hard limits to prevent unbounded growth
- **Periodic Cleanup**: Automatic cleanup of expired cache entries every minute

## Limitations & Known Issues

1. **Unofficial Library**: May break if TikTok changes internal APIs
2. **No Authentication**: Library doesn't require OAuth (connects anonymously)
3. **Username-Based**: Requires TikTok username, not live stream ID
4. **Limited Metadata**: Some data (like stream_id) not available via unofficial library
5. **Rate Limits**: Unknown rate limits from TikTok's side. Separately, the **Euler Stream** sign
   service imposes its own ceiling: twelve concurrent connection attempts exhausted the free tier
   on 2026-08-14, and the connector does not surface that cleanly — it throws
   `Cannot read properties of undefined (reading 'retry-after')` while reading the 429. See
   ADR-0052 and the "WebSocket signing" section above.
6. **Connection Stability**: May experience disconnections during long streams
7. **Gift Enrichment Off By Default**: `enableExtendedGiftInfo` is disabled because Euler
   paywalls `fetchAvailableGifts()` behind a Business plan. It turns on automatically under
   `TIKTOK_SIGNER_MODE=self`.

## Troubleshooting

### Connection Fails

```
Error: Failed to connect to TikTok stream
```

**Possible causes:**
- Username is incorrect
- User is not currently live
- TikTok has blocked the connection
- Network connectivity issues

**Solutions:**
- Verify the username is correct (without @ symbol)
- Ensure the user is actively streaming
- Check network connectivity
- Try again later if rate limited

### Messages Not Appearing

**Check:**
1. Redis connection: `redis-cli PING`
2. Stream exists: `redis-cli XLEN chat:raw`
3. Service logs for errors
4. User is actually live on TikTok
5. Chat is enabled on the stream

### A Coin Chest Did Not Appear

Coin chests ("treasure boxes") ride on TikTok's `ENVELOPE` message, which multiplexes several
unrelated products, so a chest that never surfaced has more than one possible cause. Every
`ENVELOPE` frame is logged at `info` (`"Received ENVELOPE frame"`) with its decoded
`business_type`, `display` and `coins`, and counted by outcome:

```promql
# Why envelope frames did not become chests
sum by (outcome) (tiktok_envelope_frames_total)
```

`outcome` is one of `published`, `super_fan_box` (a Super Fan Box, not a chest), `not_a_drop`
(the HIDE frame for a chest that expired or was fully claimed), `no_chest_payload` (an envelope
announcing no chest — these must not render), `duplicate`, or `error`.

**If there is no log line and no counter movement at all, the frame never reached the service.**
Check what TikTok is actually sending with:

```promql
# Every protobuf message that decoded, by wire name
sum by (method) (tiktok_wire_messages_total)
```

If `WebcastEnvelopeMessage` is missing from that metric while chests are visibly dropping in the
stream, the envelope is not reaching us in decodable form.

**That narrows it to three causes without separating them**, because this metric counts only frames
that decoded: TikTok stopped sending the message, TikTok renamed it, or it no longer decodes.
`tiktok-live-connector` skips a method absent from its schema *silently*, and drops one that throws
while decoding, so neither leaves a trace. To name an unknown method you need the connector's
`DEBUG_DESERIALIZE_XD` env var, which `console.log`s the method plus a base64 payload for every
frame it cannot place. That is noisy and unstructured, so treat it as a deliberate short-lived
investigation rather than something to leave enabled.

A renamed or undecodable message means the unofficial protocol has drifted and the library needs
bumping (as in PR #539). **For coin chests specifically, that has been tested and ruled out.** On
2026-08-14, `DEBUG_DESERIALIZE_XD` on two live rooms showed TikTok does send methods the library
cannot decode (`WebcastLinkScreenChangeMessage`, `RoomMessage`,
`WebcastUpdateShareRevenueNoticeMessage`, `WebcastAnchorToolModificationMessage`,
`WebcastGiftGalleryMessage`, `WebcastPrivilegeAdvanceMessage`), so the check works, and **none of
them resembles an envelope**. The chest is not arriving under a different name; it is not arriving.
No envelope frame appeared in ~75 room-minutes across eight live rooms, one with 61 gifts.

### Dead ends, so they are not re-investigated

- **`room_auth` capability flags are not a signal.** `fetchRoomInfo()` is unauthenticated and
  returns `data.room_auth.GoldenEnvelope` and `data.room_auth.anchor_level_permission.treasure_box`,
  which look like exactly the per-room switch you want. They are not: for anonymous fetches **all 30
  `anchor_level_permission` entries read 0**, including `share` for a room that sent us 62 social
  frames in the same window. The map is uniformly zeroed, so a `0` says nothing about the feature.
  Do not gate UI or listener behaviour on it.
- **Polling the gift catalogue is not free.** `fetchAvailableGifts()` returns *"This endpoint
  requires a Business plan"* from the Euler Stream sign server on our tier.
- **Bumping `tiktok-live-connector` will not fix it**, per the ruled-out drift above, and 2.4.3
  relicenses to a modified AGPL restricting hosted SaaS use. See the note in PR #695.

The leading remaining theory is that TikTok pushes envelopes only to **authenticated** sessions
(this service connects anonymously; the library does support `session` + `authenticateWs`, though it
forwards the session cookie to the sign server, which is a credential decision, not a config
change). Chests being region or creator-tier gated, and simply absent from every room sampled, is
not excluded either.

Probe caveat: cap concurrent probe connections at ~5. Twelve at once exhausts the Euler Stream
free-tier sign limit, which surfaces as the connector throwing on `Cannot read properties of
undefined (reading 'retry-after')` rather than a clean 429.

## Migration Path (Future)

When TikTok releases an official Live Chat API:

1. Update OAuth scopes in auth service (add live chat permissions)
2. Replace `TikTok-Live-Connector` with official TikTok API client
3. Update authentication to use OAuth tokens from database
4. Modify message handler to parse official API response format
5. Update health checks to validate OAuth token status
6. Remove BETA labels from UI

## Dependencies

- `tiktok-live-connector` - Unofficial TikTok LIVE WebSocket client
- `redis` - Redis client for stream publishing
- `pg` - PostgreSQL client for active stream queries
- `winston` - Logging
- `typescript` - Type safety

## Docker

See `Dockerfile` for containerization details.

## License

MIT (same as All-Chat project)
