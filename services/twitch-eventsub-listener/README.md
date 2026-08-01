# Twitch EventSub Listener Service

## Overview

The Twitch EventSub Listener connects to Twitch's EventSub WebSocket API to receive real-time events that are not available via IRC, primarily **channel point redemptions**.

**Version:** 1.0.0

**Why a separate service?**
- EventSub uses WebSocket (different from IRC)
- Requires app access tokens (not user OAuth)
- Session-based subscriptions (can't scale horizontally like IRC)
- Optional deployment (only needed if users want channel points displayed)

## Features

- WebSocket connection to `wss://eventsub.wss.twitch.tv/ws`
- App access token authentication (client credentials flow)
- Subscription management for channel point redemptions
- **Leader election** (only one instance creates subscriptions)
- Dynamic channel tracking (syncs from database every 30s)
- Publishes events to Redis Streams (`chat:raw`)
- Automatic reconnection with session resumption
- **Source heartbeat**: bumps `overlay_chat_sources.updated_at` for every channel it holds
  a live `channel.chat.message` subscription for, each sync tick (leader-gated), so the
  source-manager cleanup job never marks an actively-delivered Twitch source stale. This
  honors the heartbeat contract the YouTube/Kick listeners already follow (migration 059,
  ADR-0032); before it, IRC-in-enforce-mode + no EventSub heartbeat meant Twitch sources
  on always-open overlays were deactivated after 24h and their chat silently dropped.
- Health check endpoints

## Supported Events

### Currently Implemented
- ✅ `channel.channel_points_custom_reward_redemption.add` - Channel point redemptions
- ✅ `channel.subscribe` / `channel.subscription.gift` / `channel.subscription.message` - Subscriptions, gifts, resubs
- ✅ `channel.cheer`, `channel.raid`, `channel.follow` - Bits, raids, follows
- ✅ `stream.online` / `stream.offline` - Stream lifecycle (cross-platform discovery + credits)
- ✅ `channel.chat.message` - **Chat reading** (demand-gated; see below)
- ✅ `channel.chat.notification` - **Chat notices** (watch streaks, announcements, and other "events that appear in chat"; see below)
- ✅ `channel.chat.message_delete` / `channel.chat.clear_user_messages` / `channel.chat.clear` - **Chat moderation** (single delete, user timeout/ban, full clear; see below)

> Transport note: subscriptions use **webhook** transport (HTTP callback at `EVENTSUB_CALLBACK_URL`), not the EventSub WebSocket. Twitch verifies each subscription via a `webhook_callback_verification` challenge before it becomes `enabled`.

### Chat Reading (channel.chat.message)

To relieve the IRC bot's ~100-channel cap, channels whose owner granted the chat scopes
(`user:read:chat` + `user:bot` + `channel:bot`) are read via EventSub instead of IRC.

**Dynamic ownership claim (ADR-0015).** The IRC↔EventSub split is NOT a static scope predicate — it
tracks *actual delivery*. On every delivered `channel.chat.message`, the webhook handler writes a
per-channel claim key `eventsub:chat:owner:{login}` (TTL `EVENTSUB_CHAT_CLAIM_TTL`, default 5m,
throttled to one write per channel per 60s) and releases it on revocation. The IRC listener excludes
only channels that hold a **live claim** (`SCAN eventsub:chat:owner:*`). Any channel EventSub is not
currently serving — never subscribed, verification failed, partial scope, revoked, or during an
outage — has no claim and is read by IRC, the always-on fallback. The brief handoff overlap is made
idempotent by message-processor deduplicating on the native Twitch message id (`tags["id"]`, set
identically by both paths). This eliminated the previous permanent-loss modes where a scope-gated
channel was dropped by IRC yet not delivered by EventSub.

The chat subscription is **demand-gated**: it exists only while an overlay using the channel has a
live WebSocket. When demand arrives, the chat subscription is created *before* the always-on event
subscriptions so chat starts flowing with minimal latency.

### Chat Notices (channel.chat.notification)

`channel.chat.message` carries plain chat and nothing else. Twitch delivers "events that appear in
chat" on a separate subscription, and for some of them **the notice is the only carrier of the
chatter's own message text** — most importantly `watch_streak` (a returning viewer's message plus
their milestone) and `announcement` (a `/announce` body). Without this subscription those messages
never arrive at all; they were silently dropped until ADR-0046.

It shares the chat subscription's condition, scopes and lifecycle, so it is created on
`subscribe_chat` and removed on `unsubscribe_chat` alongside the deletion subscriptions. A delivered
notice refreshes the ownership claim and the connected indicator, exactly like a delivered message
(but a *revoked* notification subscription does not release the claim — losing notices does not mean
chat is dead).

Notices are routed by `notice_type` (any `shared_chat_` prefix is stripped first, since the payload
arrives under a prefixed key too):

| Notice type | Handling |
|---|---|
| `sub`, `resub`, `sub_gift`, `community_sub_gift`, `raid` | **skipped** — already delivered by `channel.subscribe` / `channel.subscription.message` / `channel.subscription.gift` / `channel.raid` with richer data. Emitting them here would double-render every sub and raid |
| `watch_streak` | event `watch_streak`; text = the viewer's message. Toggle: `enable_twitch_watch_streaks` (off ⇒ the message still renders as plain chat, only the milestone row is suppressed) |
| `announcement` | event `announcement`; text = the announcement body. Not toggleable (it is chat) |
| `bits_badge_tier` | event `bits_badge_tier` — its own type, **not** `bits`: a lifetime badge unlock must not render as "Bits Cheered!" for a cheer that never happened. Rides the bits toggle |
| `gift_paid_upgrade` / `prime_paid_upgrade` / `pay_it_forward` | events of the same name (gift-sub toggle) |
| `unraid` | event `unraid` (raid toggle) |
| `charity_donation` / `modiversary` | events of the same name |
| anything else, incl. Twitch's `unknown` | event `twitch_notice` off `system_message`, logged at Info so a new Twitch notice type degrades instead of vanishing |

Notice tags are built by delegating to the chat path's own `buildChatTags`, so a notice-borne message
enriches identically to ordinary chat (first-party emote positions, badges, colour, `room-id`,
shared-chat provenance) and is registered in the message-ID registry — a moderator can delete a
watch-streak message like any other. Because the notice carries the same native message id a
`channel.chat.message` would, the message-processor's native-id dedup collapses the two should Twitch
ever send both.

### Chat Moderation (deletions)

A claimed channel is read *only* by EventSub, so IRC's `CLEARMSG`/`CLEARCHAT` handling no longer
runs for it. To keep moderation working, the listener subscribes to the three chat-deletion events
**alongside** `channel.chat.message` (same `user:read:chat` scope, same condition, same demand-gated
lifecycle — created on `subscribe_chat`, removed on `unsubscribe_chat`):

| EventSub event | `deletion_type` | Overlay effect |
|----------------|-----------------|----------------|
| `channel.chat.message_delete` | `single` | removes one message (resolved via the message-ID registry) |
| `channel.chat.clear_user_messages` | `batch` | removes all messages from a user (timeout/ban) |
| `channel.chat.clear` | `clear` | removes all messages for the channel |

These are normalized into the same `message_deletion` raw-event shape `twitch-listener` emits, so the
message-processor and overlay handle EventSub- and IRC-sourced deletions identically.

**Message-ID registry.** Single-message deletes carry only the *native* Twitch message id, so the
listener registers `native id → internal UUID` for every delivered chat message (shared
`message-processor/registry`, 1h TTL — identical to IRC's capture-point registration). For
chat-scoped channels EventSub is the sole writer; without this the message-processor can't resolve
which displayed message to remove and buffers the deletion until it expires.

> **Limitation:** `channel.chat.clear_user_messages` carries no duration, so a timeout is reported as
> a ban (the messages are removed either way; only the moderation-log label differs). See ADR-0015.

### Platform Status Indicators

The listener publishes the chat channel's connection state to the `platform:status` Redis Pub/Sub
channel (consumed by api-gateway → overlay indicators), keyed by the lowercased login to match
`overlay_chat_sources.channel_id`:

- **`connected`** — published by the webhook handler when Twitch verifies the `channel.chat.message`
  subscription (it is then `enabled` and will deliver). The login is resolved from `users.twitch_id`,
  so any pod receiving the verification can publish it.
- **`offline`** — published by the channel manager when the chat subscription is torn down (demand
  lost or channel removed), and by the webhook handler on subscription revocation.

Without this, channels moved off IRC to EventSub would show no Twitch indicator.

## Architecture

```
┌─────────────────────────────────────┐
│ Twitch EventSub WebSocket           │
│ wss://eventsub.wss.twitch.tv/ws     │
└──────────────┬──────────────────────┘
               │
               │ WebSocket messages
               │ (Welcome, Notification, Keepalive, Reconnect)
               ↓
┌─────────────────────────────────────┐
│ EventSub Client                     │
│ - Session management                │
│ - Message parsing                   │
│ - Notification handling             │
└──────────────┬──────────────────────┘
               │
               │ onNotification()
               ↓
┌─────────────────────────────────────┐
│ Event Handler                       │
│ - Parse channel points redemption   │
│ - Create RawChatMessage             │
└──────────────┬──────────────────────┘
               │
               ↓
┌─────────────────────────────────────┐
│ Stream Publisher                    │
│ - Publish to Redis Stream           │
│ - Stream: chat:raw                  │
└──────────────┬──────────────────────┘
               │
               ↓
      Message Processor
      (same flow as IRC events)
```

## Leader Election

A single leader manages all EventSub subscriptions (one webhook callback serves the cluster), so
only one pod may create/delete subscriptions at a time. Leadership is a single `shard:0` lease
acquired through the shared `LeadershipCoordinator` (source-manager), per ADR-0007.

- **Leader:** Its subscription callback creates/deletes subscriptions and reads chat.
- **Standby:** The channel manager still runs (started by `LeadershipListener.Start`) but its
  callback is a no-op; it keeps trying to acquire leadership.
- **Acquisition:** `EnsureLeadership` is called **repeatedly** (every ~10s) until acquired, and
  re-acquired after loss. A one-shot attempt is wrong — after a rolling deploy the new pods race
  the outgoing leader's still-held lease, and without retry no pod ever becomes leader.
- **On acquire:** the manager's tracking map is reset (`ResetTracking`) and a sync is triggered,
  so a freshly promoted pod (re)creates every subscription rather than skipping stale entries.

The webhook handler runs on every pod (any pod may receive Twitch's callback); only subscription
*management* is leader-gated.

## Event Flow

1. **Channel Manager** syncs active Twitch channels from database (every 30s)
2. **Subscription Manager** creates EventSub subscription for each channel
3. **EventSub Client** receives notification when user redeems channel points
4. **Event Handler** parses redemption and creates `RawChatMessage`
5. **Publisher** publishes to Redis Stream `chat:raw`
6. **Message Processor** consumes, normalizes, enriches, publishes to overlay Pub/Sub
7. **API Gateway** broadcasts to WebSocket clients
8. **Frontend** displays redemption on overlay

## Configuration

### Environment Variables

```bash
# Twitch Application (required)
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret

# Database (required)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis (required)
REDIS_HOST=localhost
REDIS_PORT=6379

# Service
PORT=8090
LOG_LEVEL=info  # debug, info, warn, error
```

### Twitch Application Setup

1. Go to https://dev.twitch.tv/console/apps
2. Create or select your application
3. Copy Client ID and Client Secret
4. **No redirect URI needed** (app access token doesn't require user auth)

## Deployment

### Local Development

```bash
# Set environment variables
export TWITCH_CLIENT_ID=your_client_id
export TWITCH_CLIENT_SECRET=your_client_secret

# Build
go build -o bin/twitch-eventsub-listener ./cmd

# Run
./bin/twitch-eventsub-listener
```

### Docker

```bash
docker build -t twitch-eventsub-listener:latest .

docker run -e TWITCH_CLIENT_ID=... \
           -e TWITCH_CLIENT_SECRET=... \
           -e DATABASE_HOST=... \
           -e REDIS_HOST=... \
           twitch-eventsub-listener:latest
```

### Kubernetes

```bash
# Create secret with Twitch credentials
kubectl create secret generic twitch-eventsub-creds \
  --from-literal=client-id=your_client_id \
  --from-literal=client-secret=your_client_secret \
  -n allchat

# Deploy service
kubectl apply -f deployments/k8s/base/twitch-eventsub-listener/
```

## Health Checks

### Liveness Probe
```
GET /health/live
→ 200 OK (always)
```

### Readiness Probe
```
GET /health/ready
→ 200 OK if leader AND connected to EventSub
→ 503 otherwise
```

### Status Endpoint
```
GET /status
→ {
  "is_leader": true,
  "connected": true,
  "session_id": "AQoQexAW..."
}
```

## Monitoring

### Logs

**Leadership Acquired:**
```json
{
  "level": "info",
  "message": "Acquired leadership",
  "instance_id": "uuid"
}
```

**Channel Points Redemption:**
```json
{
  "level": "info",
  "message": "Published channel points redemption",
  "channel": "xqc",
  "username": "viewer123",
  "reward": "Hydrate",
  "cost": 500
}
```

**Session Established:**
```json
{
  "level": "info",
  "message": "EventSub session established",
  "session_id": "AQoQ..."
}
```

### Metrics

- `eventsub_notifications_received{type}` - Notifications received
- `eventsub_subscriptions_active` - Active subscriptions
- `eventsub_websocket_reconnects` - Reconnection count
- `eventsub_leadership_status` - Current leadership status (1=leader, 0=follower)

## Troubleshooting

### WebSocket Connection Fails

**Check credentials:**
```bash
curl -X POST "https://id.twitch.tv/oauth2/token?client_id=YOUR_ID&client_secret=YOUR_SECRET&grant_type=client_credentials"
```

Should return:
```json
{
  "access_token": "...",
  "expires_in": 5184000,
  "token_type": "bearer"
}
```

### Subscriptions Not Created

**Check logs:**
```bash
kubectl logs -n allchat -l app=twitch-eventsub-listener | grep subscription
```

**Verify leader election:**
```bash
kubectl exec -n allchat redis-0 -- redis-cli GET leader:twitch-eventsub
```

### Channel Points Not Appearing

1. **Verify event settings enabled:**
   ```sql
   SELECT enable_twitch_channel_points
   FROM overlay_event_settings
   WHERE overlay_id = '{id}';
   ```

2. **Check Redis Stream:**
   ```bash
   kubectl exec -n allchat redis-0 -- redis-cli \
     XREAD COUNT 10 STREAMS chat:raw 0 | grep channel_points
   ```

3. **Check EventSub session:**
   ```bash
   curl http://localhost:8090/status
   ```

## Event Message Format

**Channel Points Redemption:**
```json
{
  "message_id": "uuid",
  "platform": "twitch",
  "channel_id": "xqc",
  "user_id": "12345",
  "username": "viewer123",
  "text": "Redeemed Hydrate",
  "timestamp": "2026-01-25T...",
  "tags": {...},
  "event_type": "channel_points",
  "event_data": {
    "reward_id": "uuid",
    "reward_title": "Hydrate",
    "reward_cost": 500,
    "reward_prompt": "Drink some water!",
    "user_input": "I'm hydrating!",
    "status": "unfulfilled"
  }
}
```

## Performance

- **Memory:** ~50MB baseline
- **CPU:** <5% under normal load
- **Network:** WebSocket kept alive with periodic keepalives
- **EventSub Limits:** Max 3 WebSocket connections per app (we use 1)

## References

- [Twitch EventSub WebSocket Documentation](https://dev.twitch.tv/docs/eventsub/handling-websocket-events/)
- [EventSub Subscription Types](https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/)
- [App Access Tokens](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#client-credentials-grant-flow)
