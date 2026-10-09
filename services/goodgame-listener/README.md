# GoodGame Listener Service

## Overview

The GoodGame Listener is a microservice that connects to goodgame.ru's (GG, CIS
streaming platform) chat system via WebSocket and publishes raw chat messages
to Redis Streams for processing by the Message Processor service.

**Port**: 8096

## Architecture

```
┌─────────────────────┐
│  GoodGame Chat      │
│  WebSocket Server   │
│  (chat.goodgame.ru) │
└──────────┬──────────┘
           │ WebSocket
           │ JSON {type,data} frames
           ▼
┌─────────────────────┐
│  GoodGame Listener  │
│  - WebSocket Client │
│  - Channel Resolver │
│  - Channel Manager  │
│  - Publisher        │
└──────────┬──────────┘
           │ XADD
           ▼
┌─────────────────────┐
│  Redis Streams      │
│  (chat:raw)         │
└─────────────────────┘
```

## Protocol sources

Grounded on real sources (no invented shapes; fixtures in
`websocket/client_test.go` and
`services/message-processor/normalizer/goodgame_normalizer_test.go` are live
captures or verbatim protocol examples):

- **Official chat protocol**: [GoodGame/API Chat/protocol.md](https://github.com/GoodGame/API/blob/master/Chat/protocol.md)
  (note: `protocol2.md` is the newer draft; the deployed raw-WS endpoint still
  speaks the documented v1.1 shape). Frame envelope `{"type": "...", "data":
  {...}}`, `welcome` on connect, `join`/`success_join`, `message`,
  `remove_message`, `error`, rights ladder for badges.
- **Endpoint**: `wss://chat.goodgame.ru/chat/websocket` — from the official
  protocol doc, and used by every maintained third-party client:
  [drewoko/peka.online `goodgame/chat.go`](https://github.com/drewoko/peka.online) (gorilla/websocket, dial + JSON frames + `ping`),
  [drewoko/p3ka `chats/goodgame.go`](https://github.com/drewoko/p3ka) (same shape), and
  [tommsawyer/goodgamechatapi `lib/Connection.js`](https://github.com/tommsawyer/goodgamechatapi).
- **Legacy SockJS**: the Norgat gist
  ([b082d6f62bcc2a0b4142](https://gist.github.com/b082d6f62bcc2a0b4142), 2015)
  connects via SockJS to `http://chat.goodgame.ru:8081/chat`. That is the
  2015-era transport; the maintained clients and the live check (2026-09)
  confirm the raw WebSocket endpoint — so **no SockJS framing layer was
  needed**. A raw dial to `wss://chat.goodgame.ru/chat/websocket` receives a
  plaintext `welcome` frame immediately (verified live).
- **Heartbeat**: application-level JSON `{"type":"ping","data":{}}`
  (client → server) rather than protocol-level ping/pong frames; this is what
  peka.online and p3ka do. This listener pings every 25 s and answers a
  server-initiated `ping` with `pong`.
- **Social Stream Ninja** ([steveseguin/social_stream
  `sources/goodgame.js`](https://github.com/steveseguin/social_stream)) scrapes
  the embedded chat iframe DOM rather than the socket — useful cross-check for
  message fields (name, colour, badges-as-tooltips) but not the transport.

## Key Components

### 1. WebSocket Client (`websocket/client.go`)

Raw WebSocket client speaking the GoodGame chat protocol (no Pusher, no SockJS).

**Features:**
- JSON envelope framing in both directions
- `welcome` handshake, join/unjoin per numeric channel id
- Application-level `ping` every 25 s, `pong` answering
- Re-join of all recorded channels on reconnect (welcome replay)
- Unknown frame types are logged and dropped (`users_list`, `payment`,
  `update_rights`, ...) — log-and-drop per the shared contract

**Chat message frame example** (live capture, verbatim field shapes):
```json
{
  "type": "message",
  "data": {
    "channel_id": 5,
    "user_id": 21011,
    "user_name": "runi.",
    "user_rights": 0,
    "premium": 0,
    "color": "simple",
    "icon": "none",
    "role": "",
    "mobile": 0,
    "message_id": 1788879813866,
    "timestamp": 1788879814,
    "text": "hello from the spike"
  }
}
```
Numeric-vs-string encoding varies per field even inside one frame, so every
number-ish field decodes as `json.Number`.

### 2. Channel Resolver (`channels/resolver.go`)

GoodGame identifies channels by a symbolic key ("Miker") on the site/API but by
a numeric id on the chat socket. The DB stores only the slug the streamer
entered (`overlay_chat_sources.channel_identifier`); the listener resolves it
per sync cycle.

**Endpoint**: `GET https://goodgame.ru/api/4/stream/{key}` → `{"id": 5, "channelkey": "Miker", ...}` (verified live; `id` is the chat channel id).
Legacy equivalent: `GET https://goodgame.ru/api/getchannelstatus?id=Miker&fmt=json` (official stream_api.md; `stream_id`).

**Cache**: in-process map keyed by slug with a 24 h TTL — channel ids are
effectively permanent; the TTL bounds how long a rename can be shadowed. A
permanent 404 (removed channel) marks the source inactive; transient API
failures skip the cycle and keep existing subscriptions.

### 3. Channel Manager (`channels/manager.go`)

Same lifecycle as the kick-listener manager:

- Syncs active `goodgame` channels from the database every 30 s
- PostgreSQL LISTEN/NOTIFY on `chat_source_changes` for immediate syncs
- Source-manager leadership coordination + demand-based filtering (identical
  SDK contract: `UpdateDemandedSourceIDs`, `GetFilteredAssignmentCount`, ...)
- Maps numeric chat ids to overlay targets for message routing

### 4. Redis Publisher (`publisher/redis.go`)

Publishes raw chat messages to Redis Streams for the Message Processor.

**Stream Key:** `chat:raw` (identical envelope to kick: `platform`,
`overlay_id`, `channel_id` (= slug), `raw_message`, `timestamp`; GoodGame
message timestamps are unix seconds and are converted to UTC).

Deletions (`remove_message` frames) are published as unified
`message_deletion` events with `deletion_type=single`, matching the Kick flow.

### 5. Health Handlers (`handlers/health.go`)

- `GET /health/live` — liveness (503 on zombie connection, 5 min silence)
- `GET /health/ready` — WebSocket + Redis + subscription-count checks
- `GET /status` — detailed status with subscribed channels
- `GET /metrics` — Prometheus (`goodgame_listener_*`)

## Environment Variables

```bash
# Server Configuration
PORT=8096
LOG_LEVEL=info

# Database Connection (PostgreSQL)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis Connection
REDIS_HOST=localhost
REDIS_PORT=6379

# Source Manager (leadership + demand)
SOURCE_MANAGER_URL=http://localhost:8088
SOURCE_MANAGER_SECRET=dev-service-secret

# GoodGame specifics — none. No OAuth, no API keys: the chat endpoint accepts
# guest reads on public channels and the stream-status API is public.
```

## Database Schema

Reads from `overlay_chat_sources` with `platform = 'goodgame'`:

```sql
SELECT
  ocs.id as source_id,
  ocs.overlay_id,
  COALESCE(ocs.channel_handle, ocs.channel_name) as channel_slug,
  ocs.is_active
FROM overlay_chat_sources ocs
JOIN overlays o ON ocs.overlay_id = o.id
WHERE ocs.platform = 'goodgame'
  AND o.is_active = true
```

Channel identifiers are the streamer's GoodGame channel key (e.g. `Miker`).
Unlike the kick-listener, the resolved numeric chat id is **not** written back
to the DB (`config.chatroom_id`): resolution is cached in-process and
self-heals, so keeping the DB slug-only avoids a second write path on source
add/transfer.

Registration lives in `migrations/092_goodgame_support.sql` (supported_platforms
row + `platform_goodgame` feature gate per ADR-0008).

## Development

### Build

```bash
cd services/goodgame-listener
go build ./...
go test ./...
```

### Testing

The WebSocket test-suite runs a mock chat server (upgrade + `welcome` +
frame relay) over `httptest`; resolver tests run a fake
`/api/4/stream/<key>` HTTP server. No live GoodGame calls in tests.

```bash
go test -v ./services/goodgame-listener/...
```

### Docker

```bash
docker build -f services/goodgame-listener/Dockerfile -t goodgame-listener:dev .
docker run -p 8096:8096 -e DATABASE_HOST=postgres -e REDIS_HOST=redis goodgame-listener:dev
```

## Deployment

- **Deployment + Service**: `deployments/k8s/base/goodgame-listener/deployment.yaml`
- **HPA**: 1–3 replicas (chat fan-out is light; lower than Kick's 5)
- **docker-compose**: `goodgame-listener` service, port 8096

## Live verification status

Protocol shapes above are grounded in the official protocol doc, three
independent client implementations, and a live 2026-09 spike (raw WS dial +
join + real `message`/`success_join`/`channel_counters` captures). Continuous
live verification was NOT possible in this environment: run the built binary
against the production endpoint with one known channel (e.g. `Miker`, id `5`)
before first deploy.
