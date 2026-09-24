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

- Node.js 26.4+ (tls-impersonate's Chrome ClientHello needs the native addon
  and an OpenSSL new enough for the full extension set; see `src/ws/chrome-tls.ts`)
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
TIKTOK_SIGNER_URL=                    # tiktok-signer service URL; empty = no self signer
TIKTOK_SIGNER_AUTH_TOKEN=             # Bearer token if the signer service runs with auth
TIKTOK_SIGNER_TIMEOUT_MS=180000       # Self-signer HTTP timeout; signer rotates lanes inside one request
TIKTOK_EXTENDED_GIFT_INFO=            # Defaults on only under `self` (see below)
SIGN_API_KEY=                         # Euler Stream API key; empty means the free tier

# Heartbeat (silent-failure watchdog). Liveness is wire liveness: any frame
# the connector decodes (e.g. the RoomUserSeq a live-but-quiet stream still
# pushes) resets the timer, so a low-traffic room is not killed for silence.
# A connection whose frames never decode (acks only) is treated as deaf and
# reconnected. The timeout is room liveness, not socket liveness; soften it
# for launch-day streams rather than disabling monitoring.
TIKTOK_HEARTBEAT_INTERVAL_MS=30000     # How often to check (default)
TIKTOK_HEARTBEAT_TIMEOUT_MS=90000      # Silence before a forced reconnect (default)

# WS connect flap (2026-09-16 transport plan): TikTok answering the upgrade
# with HTTP 200 is a transient flap; connect retries immediately up to
# TIKTOK_FLAP_MAX_FAST_RETRIES before falling into normal error backoff.
# Retries 1-2 wait TIKTOK_FLAP_RETRY_DELAY_MS, retries 3+ wait 3x that —
# prod flaps (2026-09-17) are session-scoring artifacts that ease with
# spacing. Defaults: 3 x 1000ms (lab-measured). Metrics: tiktok_ws_flap_total,
# tiktok_ws_flap_retries_total, tiktok_ws_flap_exhausted_total.
TIKTOK_FLAP_MAX_FAST_RETRIES=3
TIKTOK_FLAP_RETRY_DELAY_MS=1000

# Pre-sign lane pinning (WS egress): when the signer answers a pre-sign with
# no proxy lane (direct egress), the pre-sign is skipped for 10 minutes —
# its ~50s capture round-trip cannot pin anything. The first answer that
# does carry a lane re-enables pinning immediately.

# Canary rooms (phase 2): rooms mirrored against the signer's viewer-tab
# relay (signer: SIGNER_RELAY_CANARY_ROOMS). Divergence logs +
# tiktok_canary_divergences_total; never affects the primary connection.
# In pure-node mode the relay has no target-room tab to attach to, so the
# consumer warms one on the signer (POST /v1/sign viewer path) at most once
# per stint — tiktok_canary_warms_total{outcome} audits that budget.
TIKTOK_CANARY_ROOMS=                   # Comma-separated usernames; empty = no canary

# Premium fallback (ADR-0058, off by default): on flap-retry exhaustion a
# premium room's delivery switches to the signer's viewer-tab relay
# (signer: SIGNER_RELAY_FALLBACK=on). A stint ends on stream end, tab death,
# signer refusal, or TIKTOK_FALLBACK_MAX_DURATION_MS (default 6h), and the
# room returns to the primary tier with a fresh flap budget. In pure-node
# mode the promotion warms the target-room tab first (one capture,
# premium-gated). Metrics: tiktok_fallback_promotions_total,
# tiktok_fallback_deliveries_total.
TIKTOK_PREMIUM_FALLBACK=off           # on | off
TIKTOK_FALLBACK_MAX_DURATION_MS=21600000

# Demand poll and rebalancing (ADR-0007)
DEMAND_SAFETY_INTERVAL_MS=25000       # Demand safety-net poll; also re-registers this pod as a
                                      # peer and drives lease rebalancing. Must stay below
                                      # source-manager's 30s peer TTL or the observed peer count
                                      # never stabilizes and rebalancing never fires
                                      # Release selection ranks by wire freshness (heartbeat
                                      # monitor): idle rooms are released first, hot rooms
                                      # (message <60s ago) last — a moved hot room must re-handshake
                                      # on the receiving pod for nothing.
# Connection ceiling per pod
TIKTOK_MAX_STREAMS_PER_POD=20         # Hard cap on concurrent WebSocket connections. The Euler
                                      # free tier proxies every connection and caps concurrent ones
                                      # (~25); above that it accepts the handshake but withholds live
                                      # push (2026-09-14 incident). Also caps the leases a pod
                                      # holds during rebalancing: a leased-but-unconnectable stream
                                      # is deaf everywhere, since no other pod may claim it.
                                      # Raise with a paid plan; remove when self-signing retires
                                      # Euler (ADR-0052)
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

This one is verified end to end against live TikTok, not just asserted:

```bash
TIKTOK_LIVE_TESTS=1 npx vitest run src/sign/euler-free.live.test.ts
```

`src/sign/euler-free.live.test.ts` sets the skip flags and then points
`SignConfig.basePath` at a **closed port**, so any surviving Euler dependency fails loudly
instead of quietly working. It resolves room IDs for three accounts and answers is-live with
Euler black-holed. It is opt-in and skipped by default: CI must not depend on tiktok.com being
reachable, or a TikTok outage reads as our regression.
**`TIKTOK_SIGNER_MODE`** (default `euler`, risky). The signature has no direct-to-TikTok route in
the library, so this is the part we had to build. The signer now exists:
[`services/tiktok-signer`](../tiktok-signer/) signs and executes `/webcast/im/fetch/` with
TikTok's own SDK in a headless browser. Set `TIKTOK_SIGNER_URL` to its address (the k8s
deployment defaults it to `http://tiktok-signer:8092`) and the mode's signer is constructed at
startup; without the URL, `shadow`, `self` and `pure-node` log a warning and stay on Euler.

| Mode | Who signs the connection | Purpose |
|---|---|---|
| `euler` | Euler Stream | Unchanged behaviour. Our code is not on the connect path. |
| `shadow` | Euler | Our signer runs in parallel against the same room; its outcome is recorded and discarded. Cannot change connection behaviour. |
| `self` | Us | Euler catches failures while `TIKTOK_SELF_SIGN_FALLBACK` is on. |
| `pure-node` | Us (leased session) | No Euler, no per-room page capture: the signer's warm classic room leases its WS session (`GET /v1/session`), and every room enters on it cross-room. |

Walk them in that order. `shadow` exists so the success rate of our own signer can be measured
against Euler's, on live rooms, before anything depends on it — after cutover, a TikTok change to
the signing algorithm takes TikTok ingest down until we fix it, where today it is Euler's problem.
That break-fix cycle lands on the signer service; see its README for the update procedure.

`pure-node` details (PR 2 of the pure-Node transport, measured 2026-09-18):
the signer leases a **shared WS session** — one warm classic room's
webcast URL + cookie jar, freshness-stamped — and the listener
synthesizes the connector's initial fetch result in-process
(`src/sign/pure-node.ts`): every recorded identity param is forwarded
(they are handshake-load-bearing; the bare push origin is rejected),
`cursor=0` rides the WS query (validated on the wire), and the room's
chat arrives via the connector's own `im_enter_room`. Two budgets pace
it (constraint: ~15 WS connects/hour flags the session): successful
lease fetches ≤ 8/hour (10-min cache, single-flight) and WS connect
signs ≤ 12/hour **per pod** — the session's flag budget is shared by
every pod leasing it, so keep ONE connect-capable replica in pure-node
rollouts or divide the cap accordingly. The premium **fallback tier is
ON** in this mode (PR 3 of the pure-Node transport): promotion warms the
target-room tab on the signer (`POST /v1/sign` viewer path, one capture
per attempt, premium-gated) before attaching the relay, and the canary
mirror attaches the same way — one warm per canary stint, paced by the
signer's per-room capture breaker (`tiktok_canary_warms_total{outcome}`).
Gifts stay off (`enableExtendedGiftInfo` is self-only — the URL-signing
seam keys on the self signer).

When `TIKTOK_SIGNER_URL` is set the listener also fetches the signer's browser identity
(`GET /v1/identity`) once at startup and pins every connection's device presets to it, so the
signed fetch, the signature and the WebSocket handshake all describe the same browser. The
WebSocket egress goes one step further: its TLS handshake mirrors Chrome's ClientHello
(`src/ws/chrome-tls.ts`, verified against a reference fingerprint service through the CONNECT
tunnel), so the connection is Chrome at every layer TikTok can see — TLS, HTTP headers, and
signed payload. On runtimes where the impersonation addon cannot load, the plain Node
handshake is used, which is the pre-hardening behaviour. Under
`self`, the second Euler seam (`fetchWebcastSignatureFromProvider`, generic HTTP URL signing) is
repointed at the signer service too.

`TIKTOK_EXTENDED_GIFT_INFO` defaults on **only** under `self`. The reason is narrower than it
looks: the library already fetches the gift list from TikTok directly (`gift/list/`), but that
request is signed, and signing an HTTP URL goes through a *second* Euler seam
(`fetchWebcastSignatureFromProvider`). Under `self` mode that seam routes to our signer service,
so the gift list unblocks; enabling it any earlier reinstates Euler's Business-plan error on
every connect.

Signature outcomes are exported as `tiktok_sign_attempts_total{signer,outcome,reason,load_bearing}`
and `tiktok_sign_duration_seconds`. Filter to `load_bearing="true"` for real availability, and to
`load_bearing="false"` for the shadow experiment:

```promql
# Our signer's success rate on live rooms, before we trust it
sum(rate(tiktok_sign_attempts_total{signer="self",outcome="success",load_bearing="false"}[1h]))
  / sum(rate(tiktok_sign_attempts_total{signer="self",load_bearing="false"}[1h]))

# Are we still hitting an external rate limit (Euler's free tier, TikTok)?
sum(rate(tiktok_sign_attempts_total{reason="rate_limit"}[5m]))

# How often are we refusing ourselves on the pure-node hourly budgets
# (WS connects, leases)? A refused room is parked until the window
# slides — this is capacity signal, not an external fault.
sum(rate(tiktok_sign_attempts_total{reason="budget"}[5m]))
```

### Transport tiers and the premium fallback (ADR-0058)

The primary transport is this service's own Node WebSocket. Two auxiliary
paths share the signer's viewer-tab relay (`GET /v1/stream/:username`, SSE):

- **Canary (phase 2)**: rooms in `TIKTOK_CANARY_ROOMS` are mirrored —
  relay frames are decoded and *compared* against the primary WS per
  method; divergence logs and counts (`tiktok_canary_divergences_total`)
  but never affects the connection. In pure-node mode the consumer warms
  the room's tab on the signer when the relay answers 409 (no warm tab):
  at most once per stint, `tiktok_canary_warms_total{outcome}`.
- **Premium fallback (ADR-0058)**: with `TIKTOK_PREMIUM_FALLBACK=on`, a
  room whose flap retries are exhausted **and** whose streamer is premium
  (`users.is_premium` via overlay ownership, TTL-cached, fail-closed)
  switches delivery to the relay. Frames are decoded with the connector's
  own schemas and replayed into the room's connection, so all handlers,
  dedup and heartbeat run exactly as on the primary. A stint ends on
  stream end, tab death, signer refusal, or
  `TIKTOK_FALLBACK_MAX_DURATION_MS` (default 6h) — not on "primary
  health", which is unobservable while the fallback delivers — and the
  room returns to the poller with a fresh flap budget. Metrics:
  `tiktok_fallback_promotions_total{outcome}`, `tiktok_fallback_deliveries_total{outcome}`.
  In pure-node mode the promotion first warms the target-room tab on the
  signer (`POST /v1/sign` viewer path, one capture, premium-gated); a
  failed warm reports `relay_unavailable` and the room stays on the
  primary tier's error backoff.

  The signer side needs `SIGNER_RELAY_FALLBACK=on` for the endpoint to
  serve non-canary rooms.

Every promotion alerts (`TikTokFallbackPromoted` in
`deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml`, fires on
any `outcome="promoted"`): the promoted users are the breakage canary for
the whole transport — they are the first to feel whatever TikTok changed,
before non-premium rooms go dark. Treat an alert as "investigate the
primary tier", not "fallback working as intended".


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
7. **Gift Enrichment Off By Default**: `enableExtendedGiftInfo` is disabled because
   `fetchAvailableGifts()` fails with "requires a Business plan" — Euler paywalls the *URL
   signing* the direct `gift/list/` request needs, not the gift data. It turns on automatically
   under `TIKTOK_SIGNER_MODE=self`.

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
