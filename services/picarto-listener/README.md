# Picarto Listener Service

## Overview

The Picarto Listener connects to Picarto's chat over the same WebSocket the
pop-out chat page uses and publishes raw chat messages to Redis Streams for
processing by the Message Processor service. There is no official chat API;
the endpoint is undocumented and volatile (ADR-0059).

**Port**: 8097

## Architecture

```
┌──────────────────────────┐
│ Picarto chat WebSocket   │
│ wss://chat.picarto.tv    │
│ /chat/token=<jwt>        │
└──────────┬───────────────┘
           │ 1 WS per channel
           ▼
┌──────────────────────────┐
│ Picarto Listener         │
│ - token fetcher (JWT)    │
│ - websocket client       │
│ - channel manager        │
│ - publisher              │
└──────────┬───────────────┘
           │ XADD
           ▼
┌──────────────────────────┐
│ Redis Streams (chat:raw) │
└──────────────────────────┘
```

## Protocol sources

Grounded and live-captured on 2026-09-08 (full evidence in ADR-0059):

- Endpoint `wss://chat.picarto.tv/chat/token=<jwt>`: extracted from Picarto's
  production bundle (`/static/js/main.d543fe27.js`, build constant
  `REACT_APP_CHAT_URL`).
- Anonymous viewer JWT via the GraphQL query
  `generateJwtToken(channel_name: $name) { key }` POSTed to
  `https://ptvintern.picarto.tv/ptvapi` (no authentication; `userId: 0` in
  the returned token).
- Batch frame `{"t":"c","m":[...]}` with per-message fields `c` `u` `n` `rn`
  `i` `m` `id` `d` `k` — cross-checked against the PicaBot adapter
  (github.com/NobreHD/PicaBot) and a live capture.

## Key components

### 1. Token fetcher (`token/fetcher.go`)

Fetches a fresh anonymous chat JWT per channel name over the GraphQL
endpoint. 15s HTTP timeout; failures surface as connect errors and retried
on the manager's next sync.

### 2. WebSocket client (`websocket/client.go`)

Dials one connection per channel and parses frames defensively (ADR-0059):
known shapes (chat batch `t=c`, legacy `type=stream` metadata) are handled;
anything unparseable or unknown is logged at Debug and dropped with a
dropped-message metric. Server pings are answered by gorilla's pong handler;
a silent read resets the deadline at 120s.

### 3. Channel manager (`channels/manager.go`)

Syncs the set of live connections against `overlay_chat_sources` rows with
`platform = 'picarto'` every 30s, filtered by SDK demand. One connection per
channel; many overlays may consume one channel.

### 4. Publisher (`publisher/redis.go`)

Publishes to the shared `chat:raw` Redis Stream — same payload contract as
the other listeners (`RawMessage` in `publisher/redis.go`).

## Risk note

The endpoint and token query are undocumented and can change with any Picarto
deploy, without notice. Expected failure mode: dropped-frame metrics rise
while the reconnect loop redials. The whole wire-format interpretation is
isolated to `websocket/` so a site change is a one-file fix — see the
re-spike instructions below. `platform_picarto` ships as a premium feature
gate (migration 093) so uptake is deliberate until the feed proves stable.

## How to re-spike when it breaks

Verified working sequence (as of 2026-09-08):

1. Confirm the token query still works:
   `curl -s https://ptvintern.picarto.tv/ptvapi -H 'Content-Type: application/json' -H 'Origin: https://picarto.tv' -d '{"query":"query generateToken($channelName: String) { generateJwtToken(channel_name: $channelName) { key } }","variables":{"channelName":"test"}}'`
   — expect `{"data":{"generateJwtToken":{"key":"…"}}}`.
2. Open `https://picarto.tv/chatpopout/<channel>/public` with browser devtools
   open and look at the WS frames on `chat.picarto.tv`: confirm the path and
   the frame shapes (`t:"c"` batches, or whatever has replaced them).
3. Compare against `websocket/types.go` and adjust — the envelope discrimina-
   tor constants and the `ChatEnvelope` field map are the only things that
   normally move.
4. Update the fixtures in
   `services/message-processor/normalizer/picarto_normalizer_test.go` if any
   payload field names changed, then re-run both module builds and tests.

## Configuration

No required environment variables. Standard shared vars apply (`PORT` defaults
to 8097, `DATABASE_*`, `REDIS_*`, `LOG_LEVEL`).
