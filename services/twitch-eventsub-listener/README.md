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
- Health check endpoints

## Supported Events

### Currently Implemented
- ✅ `channel.channel_points_custom_reward_redemption.add` - Channel point redemptions
- ✅ `channel.subscribe` / `channel.subscription.gift` / `channel.subscription.message` - Subscriptions, gifts, resubs
- ✅ `channel.cheer`, `channel.raid`, `channel.follow` - Bits, raids, follows
- ✅ `stream.online` / `stream.offline` - Stream lifecycle (cross-platform discovery + credits)
- ✅ `channel.chat.message` - **Chat reading** (demand-gated; see below)

> Transport note: subscriptions use **webhook** transport (HTTP callback at `EVENTSUB_CALLBACK_URL`), not the EventSub WebSocket. Twitch verifies each subscription via a `webhook_callback_verification` challenge before it becomes `enabled`.

### Chat Reading (channel.chat.message)

To relieve the IRC bot's ~100-channel cap, channels whose owner granted the chat scopes
(`user:read:chat` + `user:bot`) are read via EventSub instead of IRC. The IRC listener and the
EventSub listener use a byte-identical `granted_scopes` predicate so each channel lands on exactly
one path (no double-read, no gap).

The chat subscription is **demand-gated**: it exists only while an overlay using the channel has a
live WebSocket. When demand arrives, the chat subscription is created *before* the always-on event
subscriptions so chat starts flowing with minimal latency.

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

Uses Redis-based distributed lock:

- **Leader:** Connects to EventSub, creates subscriptions, processes events
- **Follower:** Standby, ready to take over if leader fails
- **Lock TTL:** 10 seconds
- **Renewal Interval:** 5 seconds
- **Key:** `leader:twitch-eventsub`

Only the leader maintains the EventSub WebSocket connection to prevent duplicate event notifications.

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
