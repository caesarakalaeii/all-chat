# Rumble Listener Service

## Overview

The Rumble Listener is a microservice that connects to Rumble.com's chat via
Rumble's internal chat pop-up SSE API (`web7.rumble.com`) and publishes raw
chat messages to Redis Streams for processing by the Message Processor
service.

**Port**: 8098

**Decision record**: see [ADR-0061](../../docs/adr/0061-rumble-internal-websocket.md)
for the route decision, accepted risks and maintenance expectations.

## Protocol Sources

The endpoint is undocumented. It was grounded against real sources and
verified live:

- **Social Stream Ninja** (`steveseguin/social_stream`), the primary
  reference implementation:
  - `sources/websocket/rumble.js` — the Live Stream API bridge. Documents
    the creator API (ruled out, see below) and, crucially, the internal
    chat base `https://web7.rumble.com/chat/api` with the chat pop-up id.
  - `docs/agents/08-platform-sources/rumble.md` — SSN's own platform notes,
    including "the API URL is private" (creator-scoped) guidance.
- **Rumble's official chat pop-up page** (`rumble.com/chat/popup/<id>`):
  its bootstrap script calls `RumbleChat("https://web7.rumble.com/chat/api",
  ..., <chat_id>, ...)` and opens an `EventSource` on
  `{base}/chat/<chat_id>/stream`. This is the authoritative wire contract.
- **AxelChat** (`3dproger/AxelChat`) — closed-source; its GitHub
  discussions (#783, #253) corroborate real-world behaviour (channel URLs
  vs. video URLs, region blocks) rather than the wire format.
- **Live capture (2026-09-08)**: anonymous `curl` of
  `web7.rumble.com/chat/api/chat/445201096/stream` returned a live SSE
  feed with `init` + `messages` events. That capture is pinned as test
  fixtures in `websocket/parser_test.go` and
  `services/message-processor/normalizer/rumble_normalizer_test.go`.

## Spike Findings (what was ruled out)

- **Official Live Stream API: out.** The API URL is generated per creator
  from `rumble.com/account/livestream-api`, embeds the stream key, and
  reports only the creator's own streams. The All-Chat product model
  requires sources to be **arbitrary channels**; the official API cannot
  address them. (Social Stream Ninja's own docs warn the URL is a secret.)
- **WebSocket: there is none for chat reads.** The chat pop-up uses
  SSE (`EventSource`). The listener therefore holds one HTTP/2 GET per
  chat room instead of one multiplexed socket.
- **No session cookie needed for reads.** Anonymous requests receive the
  full chat. Rumble's own client sends credentials because the page also
  *writes*; an optional `RUMBLE_SESSION_COOKIE` env is supported and
  forwarded verbatim if follower-scoped detail ever gets filtered.

## Architecture

```
┌───────────────────────┐
│  Rumble chat pop-up   │
│  SSE endpoint         │
│  (web7.rumble.com)    │
└──────────┬────────────┘
           │ HTTP/2 SSE (one stream per chat room)
           ▼
┌───────────────────────┐
│  Rumble Listener      │
│  - SSE client         │
│  - parser (1 file)    │
│  - Channel Manager    │
│  - Publisher          │
└──────────┬────────────┘
           │ XADD
           ▼
┌───────────────────────┐
│  Redis Streams        │
│  (chat:raw)           │
└───────────────────────┘
```

## Key Components

### 1. SSE Client + Parser (`websocket/`)

`client.go` manages one SSE stream per subscribed chat room (subscribe,
unsubscribe, reconnect signalling, stale-connection detection).
`parser.go` is **the single point of contact with Rumble's protocol**
(ADR-0061): endpoint construction, SSE framing and payload decoding all
live there. A rumble.com wire-format change is a one-file fix.

**Endpoint:**
```
GET https://web7.rumble.com/chat/api/chat/{chat_id}/stream
Accept: text/event-stream
```

**Event types** (unknown types are logged and dropped, never crash):

- `init` — history snapshot + chat config: `chat {id}`, `messages[]`,
  `users[]`, `channels[]`, `config {badges, rants}`.
- `messages` — live batch: `messages[]`, `users[]`, `channels[]`.

**Message shape** (live capture, 2026-09-08):
```json
{
  "id": "2737847970726173781",
  "time": "2026-09-08T14:51:03+00:00",
  "user_id": "7211755",
  "text": "…",
  "blocks": [{"type": "text.1", "data": {"text": "…"}}],
  "type": "regular",
  "rant": {"price_cents": 500, "duration": 300},
  "is_deleted": false
}
```

`users[]` entries carry the sender profile the message omits:
`id, username, link, "image.1" (avatar), is_follower, color, badges[]`.
Deletions arrive as messages with `is_deleted: true`.

### 2. Channel Manager (`channels/manager.go`)

Manages one SSE stream per active Rumble source:

- Syncs active channels from the database every 30 seconds (plus
  LISTEN/NOTIFY debounced syncs on `chat_source_changes`).
- The stored channel value **is** the numeric chat pop-up id; no
  channel-resolution API is involved (there is none reachable without a
  browser). Rumble sources are read-only and anonymous, so unlike
  kick-listener there is no OAuth token path and no channel API call.
- Demand-based subscription via the source-manager SDK (demand = assigned
  sources with an active overlay viewer).
- Message routing: chat id → overlay targets.

### 3. Redis Publisher (`publisher/redis.go`)

Publishes raw chat messages to Redis Streams for the Message Processor.

**Stream key:** `chat:raw`. Platform field: `"rumble"`.

### 4. Health Handlers (`handlers/health.go`)

- `GET /health/live` - Liveness probe (503 when every stream has been
  silent for 5 minutes - zombie connection, pod restart).
- `GET /health/ready` - Readiness probe (SSE client ready + Redis healthy
  + subscriptions match demand).
- `GET /status` - Detailed status with subscriptions.

## Environment Variables

```bash
PORT=8098                    # HTTP server port
LOG_LEVEL=info

DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

REDIS_HOST=localhost
REDIS_PORT=6379

SOURCE_MANAGER_URL=http://localhost:8088
SOURCE_MANAGER_SECRET=dev-service-secret

# Optional. Forwarded verbatim as the Cookie header on chat stream
# requests (ADR-0061). Anonymous reads work without it; set it only if
# Rumble starts filtering follower-scoped detail for anonymous readers.
# RUMBLE_SESSION_COOKIE=session=your_session_value_here
```

## Database Schema

The listener reads `overlay_chat_sources`:

```sql
SELECT
  ocs.id as source_id,
  ocs.overlay_id,
  COALESCE(ocs.channel_handle, ocs.channel_name) as channel_slug,
  ocs.config->>'chatroom_id' as chatroom_id,
  ocs.is_active
FROM overlay_chat_sources ocs
JOIN overlays o ON ocs.overlay_id = o.id
WHERE ocs.platform = 'rumble'
  AND o.is_active = true
```

**Metadata structure:**
```json
{
  "chatroom_id": 445201096
}
```

The chatroom_id duplicates the channel value (it IS the channel value) so
the readiness probe and the manager share one judgment of validity.

## Channel Discovery Flow

1. User adds a Rumble source by pasting the chat pop-up id (the integer in
   `rumble.com/chat/popup/<chat_id>`; also visible as the chat id the
   video page's chat bootstrap references).
2. Record created in `overlay_chat_sources`
   (platform='rumble', channel value = chat id).
3. Rumble Listener syncs from the database, validates the id is numeric.
4. Listener opens `web7.rumble.com/chat/api/chat/{chat_id}/stream`.
5. Messages route to every overlay consuming the source.

## Re-spike Instructions (when Rumble breaks it)

The chat pop-up id and message shapes can change without notice. To
re-spike:

1. Open a Rumble channel's chat pop-up (`rumble.com/chat/popup/<id>`) in a
   browser with devtools, or `curl -N -H "Accept: text/event-stream"`
   the stream URL directly.
2. Compare against the fixtures in `websocket/parser_test.go` and
   `services/message-processor/normalizer/rumble_normalizer_test.go`.
3. Edit **only** `websocket/parser.go` (and `types.go` if fields moved) —
   the plan's one-file isolation requirement. Update the fixtures and the
   normalizer if payloads changed.
4. Run `go test ./websocket/` in this module and
   `go test ./normalizer/ -run TestRumble` in message-processor.

## Development

### Build

```bash
cd services/rumble-listener
go build ./...
```

### Test

```bash
go test ./websocket/ ./channels/ ./handlers/ ./publisher/
# Or everything:
go test ./...
```

### Run Locally

```bash
export DATABASE_HOST=localhost REDIS_HOST=localhost LOG_LEVEL=debug
go run ./cmd
```

### Docker

```bash
docker build -f services/rumble-listener/Dockerfile -t rumble-listener:dev .
docker run -p 8098:8098 \
  -e DATABASE_HOST=postgres \
  -e REDIS_HOST=redis \
  rumble-listener:dev
```

## Deployment

### Kubernetes

- **Deployment + Service**: `deployments/k8s/base/rumble-listener/`
  (1 replica, HPA to 5).
- Probes: `/health/live` (stale-stream zombie detection),
  `/health/ready` (client + Redis + demand match).

### Limitations

- **ToS grey zone accepted** (ADR-0061): undocumented internal endpoint,
  read-only. Rumble can change or shut it at any time; expect a one-file
  fix, and plan the maintenance budget accordingly.
- Rants (donations) are surfaced as donation events; raids, pins, bans,
  gift events and other stray SSE event types are logged and dropped.
- Badges are carried as slugs; the per-chat icon catalog from `init` is
  parsed but not yet mapped to media (empty icon URLs render fine).

## Support

- GitHub Issues: https://github.com/caesarakalaeii/all-chat/issues
