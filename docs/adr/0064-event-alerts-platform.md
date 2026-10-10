# ADR-0064: Event Alerts Platform — Alert Overlay Kinds, a Separate chat:raw Consumer, and Presigned-Upload Alert Media

**Date**: 2026-10-01
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

All-Chat's overlays are chat renderers: the message-processor normalizes `chat:raw` stream
entries and delivers chat lines to `overlay:{id}:poll`. Streamers also want to *celebrate*
platform events — a Twitch sub or cheer, a YouTube Super Chat, a TikTok gift — as on-screen
alerts, goal bars, and top-supporter lists, optionally with their own attached media (sounds,
images, GIFs, short clips).

The event data already exists in the pipeline. Listeners publish event-typed `RawChatMessage`s
into `chat:raw` (EventSub's channel_points/subscription/mystery_gift/resubscription/raid/bits/
follow, YouTube's super_chat/subscriber/membership_gift, TikTok's gift/follow/share), and the
message-processor already normalizes them into `EventInfo` — but then treats them as
chat-line candidates to filter or demote. Nothing routes them to a rendering surface that is
not a chat overlay, nothing keeps event history, and no service can hold uploaded media.

## Decision Drivers

- **The listeners are done.** Event ingestion per platform is complete and working; a new
  event pipeline must reuse it, not re-derive it (per-platform API polling for events is a
  quota and auth cost the listener path has already paid once).
- **The chat pipeline must not grow a second job.** The message-processor's
  filter/demote/drop path is the system's most-trafficked correctness surface; alert
  routing inside it couples a chat regression to a celebration feature.
- **Alerts must survive a restart of anything downstream.** An alert that animates must
  also exist in history, and a half-finished delivery must be safely re-runnable.
- **Alert media bytes must not flow through Go pods** (memory, timeouts, and bandwidth
  for 10 MiB clips on every upload), yet must be playable by OBS without authentication.
- **New overlay kinds must not break the overlays that exist.** Every existing overlay is a
  chat overlay and must stay one by default.

## Considered Options

1. **A separate `alert-processor` service consuming the existing `chat:raw` stream with its
   own consumer group (chosen)**
   - Reuses the stream and the listeners as-is; reuses the message-processor's normalizers
     via a one-directional import; gets at-least-once semantics from the ADR-0002
     consumer-group pattern; fails and scales independently of the chat path.
   - ✅ Pros: zero listener changes, zero chat-path changes, one service owns alert concerns.
   - ❌ Cons: a second reader on `chat:raw` (cheap: it skips pure-chat entries immediately);
     at-least-once redelivery must be made idempotent at every write (history insert,
     leaderboard score).

2. **Extend the message-processor to also route events to alert overlays**
   - ✅ Pros: one less service, one deployment.
   - ❌ Cons: the chat pipeline gains alert persistence, Redis publishing and leaderboard
     concerns — a regression there takes chat down too; scaling for alert fan-out couples to
     chat throughput; the demote/drop logic and the alert history contract diverge in one
     codebase.

3. **Listeners publish a second, dedicated `alerts:raw` stream**
   - ✅ Pros: alert consumers see only events.
   - ❌ Cons: every listener gains a second publish path (three languages, three webhook
     surfaces) — precisely the churn this platform set out to avoid; two copies of each
     event exist in Redis; the duplication must be maintained per platform forever.

4. **Alert overlays poll an events REST API**
   - ✅ Pros: no new consumer.
   - ❌ Cons: alert latency becomes the poll interval; an offline overlay misses events by
     design; the API needs its own store anyway, so the consumer problem is not solved,
     only moved.

## Decision Outcome

**Chosen**: Option 1, together with the pieces it needs:

1. **Overlay kinds.** `overlays.overlay_type` ('chat' | 'alerts' | 'goal' | 'list', default
   'chat', CHECK-constrained — migration 100). Existing overlays keep behaving exactly as
   before; `alerts` is the event alert box, `goal` the goal bar, `list` the ranked list
   (e.g. recent followers, top supporters).
2. **The alert-processor** (`services/alert-processor/`, issue #951): consumes `chat:raw`
   with consumer group `alert-processors` (distinct from the message-processor's group),
   skips pure chat, normalizes via the message-processor's own normalizer dispatch
   (one-directional import; the message-processor never imports alert-processor), routes
   through a UNION of `overlay_chat_sources` and accepted `share_requests` filtered to
   `overlay_type IN ('alerts','goal','list')`, and per overlay: persists to `alert_events`,
   publishes the alert envelope to Pub/Sub `overlay:{id}:alerts` (the `:poll`/`:prediction`
   convention extended), and scores amount-bearing events on `list` leaderboards
   (`overlay:{id}:leaderboard:{bits|gifts|super_chat}`, 30-day rolling TTL).
3. **Idempotency as a first-class contract.** ACK only after persist + publish succeed;
   the alert id is a deterministic UUIDv5 of (platform, channel, message id, overlay id)
   with `ON CONFLICT DO NOTHING`, so a redelivered entry cannot double-write history; the
   leaderboard write claims its contribution per alert id inside one atomic Lua script
   (SET NX + ZINCRBY), so redelivery cannot double-count a board either.
4. **Alert media as a presigned-upload registry** (issues #948, #949): a single-node
   MinIO instance (bucket `allchat-media`, public-read behind `media.allch.at`) holds the
   bytes; media-service issues presigned PUTs so uploads go **directly** to MinIO, and
   indexes issued keys in `media_objects` (migration 099). Object keys are
   `{user_id}/{uuid}/{filename}` — the random uuid segment makes public-read URLs
   unguessable, the `user_id` prefix scopes ownership checks on delete. Single-node MinIO
   defers distribution, replication and lifecycle management; they are follow-ups.

## Consequences

### Positive

- Alert features (new overlay kinds, new event types, new media) land without touching the
  chat pipeline or any listener; a broken alert-processor degrades to "no alerts", never to
  "no chat".
- Event history (`alert_events`, one row per routed overlay delivery) makes goal counts,
  list overlays and post-hoc questions answerable from Postgres rather than from a replay.
- The delivery contract is the same channel naming convention the gateway already
  subscribes overlays with; wiring the frontend is a subscription change, not a protocol.
- Media uploads bypass service pods entirely; the service's memory profile is independent
  of clip size.

### Negative

- `chat:raw` carries a second reader; pure-chat entries are read and skipped per delivery
  (bounded work: parse, check `event_type`, ACK).
- At-least-once redelivery means subscribers may see the same alert twice in a crash
  window; accepted for live overlays (an alert animating twice is cosmetic) and prevented
  at the data layer (persist, history) and the leaderboard layer (per-alert claim).
- Single-node MinIO is a single point of failure and a capacity ceiling for stored media;
  accepted until demand says otherwise.
- `alert_events` has no FK to `overlays` (append-only history must survive overlay
  deletion); consumers must tolerate rows for gone overlays.

## Implementation

- **Overlays kinds**: `migrations/100_overlay_type.sql`; overlay-manager models/CRUD/render
  routing (`services/overlay-manager/`), frontend kind routing.
- **Alert-processor**: `services/alert-processor/` (consumer, normalizer bridge, routing
  repository, publisher, leaderboard, health, Prometheus metrics; port 8095; Dockerfile;
  compose entry `deployments/docker-compose.yml`). Kubernetes manifests in the GitOps repo
  (`caesar-deployment/apps/workloads/all-chat/alert-processor-deployment.yaml`).
- **History**: `migrations/101_alert_events.sql` (idempotent; index
  `(overlay_id, occurred_at DESC)`).
- **Media**: `migrations/099_media_objects.sql`; `services/media-service/`;
  `minio-statefulset.yaml`, `minio-ingress.yaml` in the GitOps repo (uploads capped at
  10 MiB, ingress `proxy-body-size 12m`).
- **Delivery channel**: `overlay:{overlay_id}:alerts` for every routed alert.

## Related Decisions

- ADR-0002 (Redis Streams + Pub/Sub) — the consumer-group pattern and the per-overlay
  channel convention this extends.
- ADR-0009 (Ring Buffer Publisher) — the downstream reconnect tolerance the gateway's
  alerts subscription inherits.
- ADR-0056 (shadcn token vocabulary) — the frontend surface the alert overlays render on.
