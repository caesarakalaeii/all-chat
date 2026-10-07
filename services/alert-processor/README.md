# Alert Processor

The Alert Processor consumes event-typed messages from the `chat:raw` Redis
Stream (with its own consumer group) and routes them to alert-capable overlays
(`overlay_type` IN `alerts`, `goal`, `list`): it normalizes each event with the
message-processor's platform normalizers, publishes an alert envelope to the
overlay's Pub/Sub channel, persists it to `alert_events`, and feeds `list`
overlay leaderboards.

It is a **read-only sibling** of the message-processor: the chat pipeline's
filter/demote path is untouched, and this service's consumer group never sees
the message-processor's pending entries (ADR-0002 consumer-group pattern).

**Port**: 8095
**Deployment**: GitOps repo (`caesar-deployment/apps/workloads/all-chat/alert-processor-deployment.yaml`), two replicas, Prometheus-scraped metrics

---

## Features

- **Redis Streams Consumer**: consumes from `chat:raw` with consumer group
  `alert-processors` (distinct from the message-processor's group)
- **At-least-once delivery**: entries are ACKed only after persist + publish
  succeed; failed alerts are redelivered via a periodic pending-entry reclaim
- **Pure chat never enters the alert path**: `event_type` empty or `"chat"` (and
  moderation `message_deletion` events) are ACKed and dropped
- **Normalization reuse**: imports the message-processor's platform normalizers
  (Twitch, YouTube, TikTok, system) instead of duplicating EventData parsing
- **Alert routing**: direct `overlay_chat_sources` UNION `share_requests`
  fan-out, filtered to alert-capable overlay types
- **History**: every routed alert is persisted to `alert_events`, idempotent on
  `alert_id`
- **Leaderboards**: `list` overlays get Redis sorted sets of summed
  amount-bearing events (bits, gifts, super chats) with a 30-day rolling TTL
- **Health checks**: `/health/live` and `/health/ready` (Postgres + Redis)
- **Metrics**: Prometheus counters by platform and event_type

---

## Architecture

```
Redis Streams (chat:raw)
  ↓ XREADGROUP (consumer group: alert-processors)
Consumer
  ↓ skip: event_type empty | "chat" | "message_deletion"
Message-processor normalizers (shared import, one direction only)
  ↓ EventInfo + user
Routing (overlay_type IN ('alerts','goal','list'))
  ↓ direct sources UNION share_requests fan-out
Per overlay:
  1. INSERT alert_events ... ON CONFLICT (id) DO NOTHING
  2. PUBLISH overlay:{overlay_id}:alerts
  3. 'list' overlays: ZINCRBY overlay:{overlay_id}:leaderboard:{category}
```

---

## Stream and consumer group

| Constant        | Value             | Notes                                        |
|-----------------|-------------------|----------------------------------------------|
| Stream key      | `chat:raw`        | Shared with the message-processor (ADR-0002) |
| Consumer group  | `alert-processors`| Own PEL; ACKs only after the alert lands      |

The group is created with `XGROUP CREATE` (offset `0`, never `MKSTREAM`): a
deployment where no listener has published yet must not have its consumer
fabricate the stream. Until `chat:raw` exists the create failure is logged once
and retried lazily — the pod stays healthy.

Messages are ACKed only after the handler succeeds. A failure leaves the entry
in the group's pending-entries list; a sweep every minute reclaims entries idle
for more than 5 minutes (a processor that died mid-batch) and reprocesses them.
Re-delivery is expected: the alert may reach subscribers twice, and the
`alert_events` write is idempotent so history is not doubled.

---

## Alert envelope

Published as JSON to `overlay:{overlay_id}:alerts` (the same payload is stored
in `alert_events.event_data` as the normalized event):

```json
{
  "alert_id": "2f1c1e2a-...-uuidv5",
  "overlay_id": "overlay-uuid",
  "platform": "twitch",
  "event_type": "bits",
  "event_data": {
    "type": "bits",
    "tier": "high",
    "value": { "amount": 100, "currency": "bits", "display_text": "100 bits" },
    "duration": 0,
    "is_update": false,
    "metadata": { }
  },
  "user": {
    "id": "12345678",
    "name": "SomeChatter",
    "avatar_url": "https://static-cdn.jtvnw.net/jtv_user_pictures/...png"
  },
  "occurred_at": "2026-03-01T12:00:00Z"
}
```

`alert_id` is a **deterministic UUIDv5** of (platform, channel id, message id,
overlay id) — stable across at-least-once re-deliveries so `ON CONFLICT (id) DO
NOTHING` dedupes history, unique per target overlay so one event routed to N
overlays stores N rows (migration `101_alert_events`).

`event_data` is the message-processor's normalized `EventInfo`, so alerts
interpret EventData exactly as the chat pipeline renders the same event.

---

## Leaderboards (`list` overlays only)

| Item    | Value                                                    |
|---------|----------------------------------------------------------|
| Key     | `overlay:{overlay_id}:leaderboard:{category}`            |
| Type    | sorted set, score = summed amount, member = `{platform}:{user_id}` |
| TTL     | 30 days, refreshed on every write                        |

Categories (amount-bearing events only):

| Category     | Event types                                        |
|--------------|----------------------------------------------------|
| `bits`       | `bits`                                             |
| `gifts`      | `mystery_gift`, `gift_subscription`, `membership_gift`, `gift` |
| `super_chat` | `super_chat`                                       |

`goal` overlays receive the same alert events (they may animate on them) but do
not keep leaderboards; authoritative goal counts are polled separately.

---

## Environment variables

### Required

```bash
DATABASE_PASSWORD          # Postgres password
```

### Optional (defaults shown)

```bash
PORT=8095                  # HTTP port (health + metrics)
LOG_LEVEL=info
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=            # empty = AUTH disabled
APP_VERSION=0.1.0
ENVIRONMENT=development
OTEL_ENABLED=false         # OpenTelemetry tracing opt-in
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

---

## Metrics

All labelled by `platform` and `event_type` (`event_type` is `chat` for chat
lines, so the consumed/routed delta is the chat volume the alert path skips):

| Metric                                       | Meaning                                  |
|----------------------------------------------|------------------------------------------|
| `alert_processor_events_consumed_total`      | stream entries parsed and accepted       |
| `alert_processor_events_routed_total`        | alerts fanned out (one per event × overlay) |
| `alert_processor_events_persisted_total`     | `alert_events` write attempts            |
| `alert_processor_publish_errors_total`       | failed publishes (alert is redelivered)  |

---

## Deployment

Compose: `alert-processor` in `deployments/docker-compose.yml` (port 8095).

Kubernetes manifests live in the GitOps repo
(`caesarakalaeii/caesar-deployment`,
`apps/workloads/all-chat/alert-processor-deployment.yaml`) — deliberately not in
this repository. The pod runs the migrations init container like the
message-processor, so `alert_events` is created idempotently on start.
