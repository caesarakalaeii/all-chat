# Owncast Listener Service

## Overview

The Owncast Listener connects to self-hosted [Owncast](https://owncast.online) instances and publishes raw chat messages to Redis Streams for processing by the Message Processor service.

**Port**: 8095

## Instance URL as Channel (ADR-0058)

An Owncast instance serves **one stream**. Unlike every other platform, where a "channel" is a channel name or numeric ID, an Owncast **channel is the instance URL itself** (e.g. `https://watch.example.org`), stored in `channel_identifier` and normalized to a bare `scheme://host` (lowercase host, no path, no trailing slash).

Overlay-manager enforces this at source-add time: the `channel_id` must be an http(s) base URL (400 otherwise), and the display name is resolved from the instance's own `GET /api/config` (`name` field). If the instance is unreachable at add time, the add still succeeds and the URL hostname is used as the channel name — instances go offline independently and that must never block adding a source.

## Architecture

One WebSocket connection **per instance URL** (not one connection with many subscriptions, as in kick-listener):

```
┌─────────────────────┐
│  Owncast instance   │
│  (self-hosted)      │
│  POST /api/chat/    │
│    register → token │
│  WS /ws?accessToken=│
└──────────┬──────────┘
           │ WebSocket
           ▼
┌─────────────────────┐
│  Owncast Listener   │
│  - one client per   │
│    instance URL     │
│  - Manager (30s DB  │
│    sync, demand)    │
│  - Publisher        │
└──────────┬──────────┘
           │ XADD
           ▼
┌─────────────────────┐
│  Redis Streams      │
│  (chat:raw)         │
└─────────────────────┘
```

## Chat flow (per instance)

1. Manager syncs active `overlay_chat_sources` rows for `platform='owncast'` every 30s (demand-filtered like other listeners).
2. For each instance URL: `POST <url>/api/chat/register` (unauthenticated, anonymous chat registration) → `{id, accessToken, displayName}`.
3. Dial `<ws|wss>://<host>/ws?accessToken=<token>` — the token is passed as a query parameter and re-registered on every reconnect (token lifetime is tied to the instance's session, so an instance restart invalidates it).
4. Read events. A single frame may carry **multiple newline-separated JSON events** (the server coalesces queued events); each line is decoded independently.
5. Only `type: "CHAT"` messages are forwarded to `chat:raw`; joins, parts, name changes, system messages and actions are logged-and-dropped (chat is read-only anyway).

## Offline instance behavior

An unreachable or offline instance is a **normal state**, not an error (ADR-0058). Each connection runs its own retry loop with capped exponential backoff: 2s → 4s → 8s → … → capped at **5 minutes**, then steady. The instance is marked `offline` in `platform:status`, its `overlay_chat_sources.is_active` is flipped false, and retries are logged at Warn for the first attempts and Debug thereafter — never Error-spam, never crash-loop.

On success the source is marked `connected`, published to `platform:status` (which the overlay client uses for ADR-0032 self-heal replay), and `is_active` is set true.

## Message payload to `chat:raw`

```json
{
  "platform": "owncast",
  "overlay_id": "uuid",
  "channel_id": "https://watch.example.org",
  "channel_name": "https://watch.example.org",
  "text": "<p>hello world</p>",
  "raw_message": {
    "id": "aBcDeFgHi",
    "timestamp": "2026-09-08T12:34:56.789Z",
    "type": "CHAT",
    "body": "<p>hello world</p>",
    "visible": true,
    "user": {
      "id": "bFzGqYvXc",
      "displayName": "gabek",
      "displayColor": 3,
      "isBot": false,
      "authenticated": false
    }
  }
}
```

Notes:

- `body` is **pre-rendered HTML** (Owncast renders markdown + emoji server-side and sanitizes); it is passed through as text with no client-side emote extraction — Owncast has no emote/badge system over chat.
- `displayColor` is an index into Owncast's user color palette, not a CSS color; the normalizer stores it as `owncast:<index>` and the frontend resolves it.

## Health endpoints

- `GET /health/live` — always 200 while the process runs. An all-instances-down world is **not** unhealthy (see above).
- `GET /health/ready` — 200 when Redis is healthy; zero connections is valid (no demanded instances, or every instance currently down).
- `GET /status` — connected URLs and connection count.
- `GET /metrics` — Prometheus (`owncast_listener_*` family: per-instance socket state, reconnects, subscriptions, messages, dropped).

## Environment Variables

No Owncast-specific required variables. Shared vars only:

```bash
PORT=8095                    # HTTP server port (health/metrics)
LOG_LEVEL=info               # debug, info, warn, error

DATABASE_HOST=localhost      # PostgreSQL
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=...
DATABASE_NAME=allchat

REDIS_HOST=localhost         # Redis (chat:raw + platform:status)
REDIS_PORT=6379

SOURCE_MANAGER_URL=http://localhost:8088   # leadership/demand SDK
SOURCE_MANAGER_SECRET=...

APP_VERSION=dev
```

## Protocol sources

Grounded on the Owncast server source (`owncast/owncast`, `develop` branch) and official API docs:

- `services/chat/events/userMessageEvent.go` — outbound CHAT payload `{id, timestamp, body, user, type, visible}`
- `models/user.go` — user shape serialized to chat clients (`id`, `displayName`, `displayColor`, `isBot`, `authenticated`)
- `services/chat/server.go` + `services/chat/chatclient.go` — `/ws` requires `accessToken` query param, 60s read deadline/pong, events coalesced per frame with `\n`
- `webserver/handlers/chat.go` + `test/automated/api/003_chat.test.js` + `test/automated/api/lib/chat.js` — `POST /api/chat/register` returns `{id, accessToken, displayName}`
- `webserver/handlers/config.go` — `GET /api/config` returns `webConfigResponse` including `name`
- `models/eventType.go` — `MessageSent = "CHAT"`; join/part/name-change/system/event types deliberately dropped

## Development

```bash
# Build
go build ./...     # from within services/owncast-listener

# Test
go test ./...

# Docker
docker build -f services/owncast-listener/Dockerfile -t owncast-listener:dev .
```

## Deployment

Kubernetes: `deployments/k8s/base/owncast-listener/` (Deployment + HPA, registered in `base/kustomization.yaml`).
Docker Compose: `deployments/docker-compose.yml` (service `owncast-listener`, port 127.0.0.1:8095).
