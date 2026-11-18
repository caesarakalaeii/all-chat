# ADR-0068: Event Alerts Platform

**Date**: 2026-10-01
**Status**: Accepted
**Revised**: 2026-11-13 (`overlay:{id}:updates` also carries Discord messages republished with late link-preview media); 2026-11-03 (brought in line with the current implementation; alert media now backed up off-cluster; the consumer passes only alert event types, so the owner-only moderation log never reaches alert overlays)
**Deciders**: All-Chat Platform Team

## Context

Overlays were chat renderers only: message-processor normalizes `chat:raw` into
`overlay:{id}` (plus `overlay:{id}:updates` for TikTok like aggregates and Discord
messages with late link-preview media). Streamers want platform events (Twitch subs and
cheers, YouTube Super Chats, TikTok gifts) as alerts, goal bars and top-supporter lists,
optionally with attached media; the data already flowed through `chat:raw` as event-typed
messages the processor normalized to `EventInfo` but then treated as chat lines to filter
or demote. Constraints: reuse the completed per-platform listeners; do not couple the
chat pipeline (the most-trafficked correctness surface) to a celebration feature;
deliveries must be re-runnable after a crash; media bytes must not flow through Go pods
yet stay playable by anonymous OBS; existing overlays keep behaving as chat.

## Considered options

- A separate `alert-processor` consuming the existing `chat:raw` with its own consumer
  group (chosen): zero listener and chat-path changes, normalizers reused through a
  one-directional import, at-least-once semantics from ADR-0002. Cost: a cheap second
  reader on `chat:raw` (skips pure chat immediately) and idempotency required at every
  write.
- Extend the message-processor: one less service, but alert persistence, publishing and
  leaderboard concerns join the chat path, so a regression there takes chat down and
  scaling couples to chat throughput.
- Listeners publish a dedicated `alerts:raw` stream: consumers see only events, but every
  listener (three languages) gains a second publish path to maintain forever, with two
  copies of each event in Redis.
- Alert overlays poll a REST API: latency becomes the poll interval, offline overlays
  miss by design, and the API still needs its own store.

## Decision

Option 1, with four parts:

1. `overlays.overlay_type` ('chat' | 'alerts' | 'goal' | 'list', default 'chat',
   CHECK-constrained, migration 100); existing overlays keep chat behavior, `alerts` is
   the event alert box, `goal` the bar, `list` the ranked list.
2. The alert-processor consumes `chat:raw` (group `alert-processors`), passes only alert
   event types (an allowlist: profiled types plus the public events rendered without a
   profile) and ACKs the rest, so pure chat, deletions, the owner-only moderation log
   (`mod_action`, which carries AutoMod-held text) and system warnings never reach an
   alert overlay; normalizes via the message-processor's normalizer dispatch
   (one-directional); routes through a UNION of `overlay_chat_sources` and accepted
   `share_requests` filtered to the alert kinds; per overlay persists to `alert_events`,
   publishes to `overlay:{id}:alerts`, and scores amount-bearing events on `list`
   leaderboards (`overlay:{id}:leaderboard:{bits|gifts|super_chat}`, 30-day rolling TTL).
3. Idempotency: ACK only after persist and publish; the alert id is a deterministic
   UUIDv5 of (platform, channel, message id, overlay id) with `ON CONFLICT DO NOTHING`,
   and the leaderboard claim is a per-alert-id atomic Lua script (SET NX + ZINCRBY), so
   redelivery double-writes nothing.
4. Media as a presigned-upload registry: single-node MinIO (bucket `allchat-media`,
   public-read behind `media.allch.at`) holds the bytes; media-service issues presigned
   PUTs that go straight to MinIO and indexes keys in `media_objects`; keys are
   `{user_id}/{uuid}/{filename}` (unguessable via the uuid, ownership-scoped by the
   prefix). Distribution and replication stay follow-ups. A nightly `mc mirror` CronJob
   mirrors the bucket to Hetzner Object Storage and Backblaze B2 (ADR-0003).

## Consequences

- Alert features land without touching listeners or the chat pipeline; a broken alert-
  processor degrades to "no alerts", never "no chat".
- `alert_events` (one row per routed overlay delivery) makes goal counts, lists and
  post-hoc questions answerable from Postgres; it has no FK to `overlays` (append-only,
  survives overlay deletion), rows for gone overlays are tolerated.
- The delivery contract reuses the gateway's channel convention, so the frontend is a
  subscription change. Uploads bypass pods: media memory is independent of clip size.
- A new event type stays invisible to alerts until profiled or allowlisted (deliberate:
  anything passing is published to sockets including anonymous OBS and share
  recipients). Redelivery may animate an alert twice in a crash window (cosmetic).
  Single-node MinIO is a SPOF and capacity ceiling until demand says otherwise.

## Implementation

- `migrations/100_overlay_type.sql`, `106_alert_events.sql` (index
  `(overlay_id, occurred_at DESC)`), `099_media_objects.sql`.
- `services/alert-processor/` (port 8098; consumer, normalizer bridge, routing repo,
  publisher, leaderboard, goal polls and list pushes per ADR-0072, health, metrics;
  compose entry in `deployments/docker-compose.yml`; k8s manifests in caesar-deployment).
- `services/media-service/`; MinIO statefulset and ingress in the GitOps repo (uploads
  capped 10 MiB, ingress `proxy-body-size 12m`).
- Delivery channel: `overlay:{overlay_id}:alerts`.
