# Altstream Listener Service

## Overview

The Altstream Listener reads live chat from [Altstream](https://altstream.online)
through its public developer API (documented at altstream.online/documentation,
External Developer API tab) and publishes raw chat events to Redis Streams for
the Message Processor. No login or OAuth is needed: a channel is identified by
its Altstream username (ADR-0082).

**Port**: 8101

## Architecture

```
┌──────────────────────────────┐   ┌──────────────────────────────┐
│ SSE  /api/chat/stream        │   │ REST (60 req/min per IP)     │
│ ?channel=<u>&lastId=<id>     │   │ /api/chat?channel=<u>&limit=1│
│ one stream per channel       │   │ /api/v1/channels/<u>/emotes  │
└──────────────┬───────────────┘   └──────────────┬───────────────┘
               │                                  │ ≤30 req/min, shared
               ▼                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│ Altstream Listener                                               │
│ - altstream/  SSE client, lastId resume, dedupe, catch-up filter,│
│               stale detection, emote catalogs                    │
│ - channels/   sources from Postgres, demand, per-channel leader  │
│ - cmd/        event to chat:raw mapping                          │
└──────────────┬───────────────────────────────────────────────────┘
               │ XADD
               ▼
┌──────────────────────────────┐
│ Redis Streams (chat:raw)     │
└──────────────────────────────┘
```

## Protocol facts (verified live 2026-10-31)

- Live chat is SSE: `GET /api/chat/stream?channel=<u>&lastId=<id>`. Frames are
  single `data: [ <event>, ... ]` lines (a JSON array, usually one event) and a
  `: ping` comment about every 25s. Response headers only arrive with the first
  byte, so a stream shows as connected at its first ping or event.
- Events: `{"type","id","author" (display name),"authorUsername","authorImage",
  "authorPronouns","text","sentAt" (epoch ms),"channel","userColor","roles",
  "badges",...}`. Ids are numeric strings and **not monotonic**. There is no
  numeric user id; `authorUsername` is the stable identity. REST-shaped events
  (and the catch-up replay) lack `authorUsername`, so the listener falls back
  to `author`.
- Emotes appear in text as `:name:` tokens. A viewer can use any emote of any
  channel they follow, and there is no anonymous global emote lookup.

### The lastId gotcha

`lastId` is **mandatory and undocumented**. Without it, or with it empty, the
stream sends `: ping` comments forever and never a message. The site's own
client always sends it. So the listener:

1. Bootstraps it from the newest id of `GET /api/chat?channel=<u>&limit=1`
   (oldest first, so the last element). An empty history or a failed lookup
   uses the sentinel `none`.
2. Tracks the id of every event it receives (any type) and reconnects with it.

With a `lastId` that matches the newest message, the stream sends only new
messages. With one that does not (an old id, or the sentinel), it first replays
the newest 1 to 3 messages, then goes live. It does not replay the full gap.

### Replays, duplicates and silent streams

- **Catch-up filter**: on the first connect, events with `sentAt` older than
  connect time minus 10s are replays and are dropped, so old messages (and an
  old `clear`) never hit overlays again. On a reconnect the cutoff is the last
  time the old stream heard anything, minus 10s, so messages sent during the
  gap still come through. The message-processor drops anything older than 60s
  regardless.
- **Dedupe**: each channel remembers its last 256 event ids. This is what stops
  reconnect replays from showing twice; the processor's own dedup only
  remembers 5s.
- **Stale detection**: a stream that delivers nothing at all, not even a ping,
  for 75s is cancelled and reconnected with jittered backoff (1s base, 30s cap;
  reset after a stream that stayed up a minute). Non-200 answers, such as a
  502 during an Altstream deploy, are treated the same way. Liveness therefore
  stays 200: the pod never needs a restart to recover a stream. No per-IP
  stream limit is known (twelve parallel streams from one IP were fine).

### Emotes

Each watched channel's catalog (`/api/v1/channels/<u>/emotes`, site-relative
URLs made absolute) is loaded before its first connect and refreshed every
10 minutes. All catalogs of the channels this pod watches merge into one
name-to-URL map, and a channel's catalog leaves the map when its stream stops.
Only the `:name:` tokens that resolve go into `raw_message.emotes`; unknown
tokens stay plain text. The message-processor computes the positions.

### REST rate limit

The API allows 60 REST requests per minute per IP. One shared limiter
(30 per minute, burst 10) covers the lastId bootstrap and all emote fetches.
SSE connects do not count.

## Event mapping (v1 scope)

| Altstream type | Published as |
|---|---|
| `message` | chat message |
| `highlight` | chat message with `data.text`; `tags.message_type=highlight` |
| `delete` | `message_deletion`, `deletion_type=single`, `target_msg_id=targetId` |
| `clear` | `message_deletion`, `deletion_type=clear` |
| anything else (`redemption`, `raid`, `system`, `shoutout`, `tip`, `subscription`) | dropped, counted in `altstream_listener_ignored_events_total{type}` |

One entry is published per overlay consuming the channel. Chat message:

```json
{"message_id":"615594708","platform":"altstream","overlay_id":"<uuid>",
 "channel_id":"copygirl","channel_name":"copygirl",
 "user_id":"vildravn","username":"vildravn","text":"choo choo :copyPeek:",
 "tags":{"message_type":"message"},
 "raw_message":{"type":"message","id":"615594708","author":"Vildravn", "...":"event verbatim",
   "emotes":[{"name":"copyPeek","url":"https://altstream.online/api/media/stickers/copygirl_1792322555448_b51313fd3353.png"}]},
 "timestamp":"2026-10-31T15:31:50Z"}
```

Deletion (discord-listener's shape; the processor resolves `target_msg_id`
through its msgid registry):

```json
{"message_id":"169457509","platform":"altstream","overlay_id":"<uuid>",
 "channel_id":"copygirl","channel_name":"copygirl",
 "event_type":"message_deletion",
 "event_data":{"deletion_type":"single","target_msg_id":"840417196"},
 "timestamp":"2026-10-31T15:52:27Z"}
```

`clear` is the same with `"event_data":{"deletion_type":"clear"}`. The exact
payloads are pinned by `cmd/main_test.go`.

## Channels and scaling

The manager reads `overlay_chat_sources` rows with `platform = 'altstream'` of
active overlays every 30s (and on every demand update), lowercases the
username, skips anything that is not `[a-z0-9_-]{1,64}`, and runs one stream
per channel for every overlay consuming it. It claims per-channel leadership
through source-manager (`SOURCE_MANAGER_SECRET`), so HPA replicas split
channels instead of double-ingesting. Readiness only checks Redis, so a new pod
is not held unready behind the leases of the pod it replaces.

## Endpoints

- `GET /health/live`: always 200
- `GET /health/ready`: 200 when Redis answers
- `GET /status`: subscribed, connected and demanded channels
- `GET /metrics`: Prometheus

## Metrics

`altstream_listener_connected_streams`, `altstream_listener_reconnects_total{reason}`,
`altstream_listener_subscription_events_total{action}`,
`altstream_listener_active_subscriptions`, `altstream_listener_messages_total{status,reason}`,
`altstream_listener_publish_latency_seconds`, `altstream_listener_dropped_messages_total{reason}`
(`catch_up`, `duplicate`, `malformed_frame`, `malformed_event`, `unknown_channel`, ...),
`altstream_listener_ignored_events_total{type}`, `altstream_listener_rest_requests_total{endpoint,result}`,
plus the shared `ring_buffer_*{service="altstream-listener"}` and leadership metrics.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `ALTSTREAM_BASE_URL` | `https://altstream.online` | API base; tests point it at a fake server |
| `PORT` | `8101` | health and metrics port |
| `SOURCE_MANAGER_URL`, `SOURCE_MANAGER_SECRET` | | leadership; without the secret every replica streams every channel |

Standard shared vars apply (`DATABASE_*` with `DATABASE_PASSWORD` required,
`REDIS_*`, `LOG_LEVEL`, `OTEL_*`).

## Known gaps

- Sticker posts arrive as a `message` with empty `text` and a site-relative
  `stickerUrl` (the field Altstream's own chat client renders). They are
  published, and the message-processor renders the sticker as an image
  attachment. The anonymous REST history omits `stickerUrl`, so whether the
  SSE stream carries it has not been seen live yet. A message with neither text
  nor `stickerUrl` is dropped (`dropped_messages_total{reason="empty"}`).

## How to re-check when it breaks

1. `curl -s 'https://altstream.online/api/chat?channel=<live user>&limit=1'`
   returns a JSON array with one event.
2. `curl -N 'https://altstream.online/api/chat/stream?channel=<live user>&lastId=none'`
   prints 1 to 3 replayed `data: [...]` lines, then `: ping` every ~25s and
   live messages. If it prints only pings, check what `lastId` the site's own
   chat client sends now (browser devtools on a channel page).
3. Compare an event against `altstream/types.go` and the fixtures in
   `altstream/testdata/`, then rerun `go test ./...` here and in
   `services/message-processor`.
