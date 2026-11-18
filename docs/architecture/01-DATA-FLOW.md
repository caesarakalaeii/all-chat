# Data Flow Architecture

How a chat message travels from a streaming platform to an overlay.

```
Platform (Twitch/YouTube/Kick/TikTok/...)
  -> listener (one service per platform, services/*-listener)
      XADD chat:raw (cap MAXLEN ~100000)
  -> Redis Stream chat:raw
      XREADGROUP, consumer group "message-processor" (alert-processor has its
      own "alert-processors" group on the same stream; moderation-service
      publishes message_deletion events into it)
  -> message-processor: normalize + enrich
      PUBLISH overlay:{overlay_id}        (chat, on :updates for message updates)
      PUBLISH overlay:{overlay_id}:alerts (alert-processor)
      PUBLISH overlay:{overlay_id}:poll / :prediction (engagement-service)
  -> api-gateway subscribes per connected overlay, broadcasts over WebSocket
  -> overlay frontend renders
```

## Streams

### `chat:raw`

Every listener publishes raw messages here as a single JSON blob in one field,
`data` (see `services/message-processor/consumer/stream_consumer.go`, which
extracts and unmarshals it). The blob is `RawChatMessage`
(`services/message-processor/models/message.go`): `message_id`, `platform`,
`overlay_id`, `channel_id`, `user_id`, `username`, `text`, `timestamp`, `tags`
(free-form key/value incl. platform-native emote data), `attachments`,
`event_type`/`event_data` for events (subscription, raid, bits, like
aggregates) and `is_update` for corrections.

Every publisher (Go listeners, services/tiktok-listener in Node,
moderation-service) trims the stream with `MAXLEN ~100000`. moderation-service
also writes `message_deletion` events into `chat:raw` so the deletion reaches
every overlay through the normal pipeline.

Duplicate delivery never happens within one group, but a consumer crash loses
unacked work; failed entries are written to `chat:dlq` with `original_stream_id`,
`source_service`, `failure_reason` and `retry_count`
(`services/message-processor/consumer/dlq.go`) and the admin re-queue endpoint in
`services/message-processor/handlers/dlq.go` republishes them back to `chat:raw`.

### Other streams

| Stream | Producer | Consumer |
|---|---|---|
| `chat:raw` | all listeners, moderation-service | message-processor, alert-processor |
| `chat:dlq` | message-processor | DLQ admin endpoint |
| `chatcmd:jobs` | message-processor | engagement-service (streamer chat commands) |
| `engagement:commands` | engagement-service | message-processor (replies as chat messages) |
| `engagement:twitch-native` | engagement-service | message-processor |

Capacities: the engagement streams trim at 10000 (`services/message-processor/consumer/engagement.go`).

## Consumer groups

| Group | Stream | Service | Note |
|---|---|---|---|
| `message-processor` | `chat:raw` | message-processor | reads batches of 100, block 5s |
| `alert-processors` | `chat:raw` | alert-processor | same entries, independent PEL |
| per-pod consumers | `chatcmd:jobs`, `engagement:*` | engagement-service | created with XGroupCreateMkStream `"$"` (newest onward) |

Inside one batch the message-processor enriches and publishes in parallel
(`MP_PROCESS_CONCURRENCY`, default 16 in
`services/message-processor/consumer/stream_consumer.go`).

## Overlay routing

After enrichment the message-processor asks PostgreSQL which overlays care
(`services/message-processor/router/overlay_router.go`): a `UNION` over
(a) `overlay_chat_sources` rows matching `platform` + `channel_id` for active
`overlay_type = 'chat'` overlays, and (b) accepted `share_requests` fan-out to
recipient overlays via `shared_overlay` sources.

## Pub/Sub channels

Channel names are built in two places: message-processor publishes
(`services/message-processor/publisher/pubsub_publisher.go`), api-gateway
subscribes (`services/api-gateway/subscription/subscriber.go`).

| Channel | Content |
|---|---|
| `overlay:{id}` | chat messages |
| `overlay:{id}:updates` | `message_update` replacements (late media, TikTok like aggregates) |
| `overlay:{id}:alerts` | alerts and goal/list overlays |
| `overlay:{id}:poll` / `:prediction` | engagement snapshots |
| `overlay:connections` | gateway publishes attach/detach; source-manager and listeners reconcile demand |
| `source:demand` | source-manager publishes full demand snapshots; youtube-listener, youtube-listener-innertube and discord-listener subscribe |
| `platform:status` | listeners publish up/down; gateway and overlay-manager subscribe |

Pub/Sub is at-most-once and nondurable. The gateway keeps each overlay
subscription alive for a linger window (`PUBSUB_LINGER_SECONDS`, default 5 min)
after the last WebSocket disconnect and replays its buffered frames to
reconnecting clients (`services/api-gateway/websocket/manager.go`); demand
linger mirrors this (`docs/adr/0014-demand-linger-symmetric-with-pubsub-linger.md`).

## Unified message format

`UnifiedChatMessage` in `services/message-processor/models/message.go` is the
shape published to `overlay:*`:

- `id`, `overlay_id`, `platform`, `channel_id`, `channel_name`, `timestamp`
- `platforms` (only for streamer "send to all" collapsed multi-platform messages)
- `user`: `id`, `username`, `display_name`, `avatar_url`, `badges[]`
  (`name`, `version`, `icon_url`), `color` (authoritative), `auto_color`,
  `name_gradient`, `source_badges`, `pronouns`, `avatar_frame_url`, `avatar_flair_url`
- `message`: `text`, `emotes[]` (`code`, `provider`, `url`, `positions`,
  `zero_width`), `attachments[]` (`type`, `url`, `content_type`, `width`,
  `height`, `thumb_url`, `spoiler`, `filename`)
- `event` (nil for chat): `type`, `tier`, `value` (`amount`, `currency`,
  `display_text`), `duration`, `aggregation_id`, `is_update`

Platform-specific fields become `tags` on the raw message and metadata on the
unified message. Normalizers live per platform in
`services/message-processor/normalizer/`.

## Emote enrichment

Cached in Redis under the `mp:emotes:v3:` namespace
(`services/message-processor/cache/emote_cache.go`): 1h fresh TTL plus a 12h
stale grace with background refresh. Misses go to the emote-service HTTP API
(`GET /emotes/channel/:channel`, `GET /emotes/:provider/:channel` in
`services/emote-service/handlers/emote.go`), which aggregates 7TV, BTTV and FFZ
and caches per channel for 1h.

## Failure handling

- **Stale entries**: consumers reclaim PEL entries idle 5 minutes
  (`services/alert-processor/consumer/consumer.go`; message-processor similar).
- **Rejects**: write to `chat:dlq`, do not re-enqueue.
- **Old messages**: entries older than 60s are dropped (`MESSAGE_AGE_CUTOFF_SECONDS`).
- **Publish failures**: bounded retry per overlay; partial failures logged
  (`services/message-processor/publisher/pubsub_publisher.go`).
- **Stream lag**: exported as `processor_stream_lag_seconds`
  (`shared/metrics/processor.go`).
- **Ring buffer**: each listener buffers XADD payloads in-process and retries
  instead of dropping on Redis blips (`shared/listener`, ADR-0009).

## Relevant ADRs

- ADR-0002: Redis Streams + Pub/Sub hybrid (the whole pipeline).
- ADR-0009: ring-buffer publisher for listener XADD resilience.
- ADR-0011: zombie listener detection.
- ADR-0015: EventSub chat ownership claim (Twitch partition).
