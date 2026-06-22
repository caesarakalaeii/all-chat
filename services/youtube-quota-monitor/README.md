# youtube-quota-monitor

Single-owner observability for the shared YouTube Data API quota.

## Why

The quota-based `youtube-listener` is no longer deployed (the listener path moved to
InnerTube, which costs no quota). But two services still spend official quota against the
shared `youtube_quota_usage` table: `moderation-service` (bans, 50 units) and
`auth-service` (streamer sends, 5 units), both via reserve-confirm-rollback (ADR-0006).

Nothing was left to *observe* that usage — the listener used to export the Prometheus
quota gauge and publish alert events. This service restores both, reading the shared
table as the single source of truth. See **ADR-0023**.

## What it does

On an interval (`QUOTA_MONITOR_INTERVAL`, default 30s) it reads today's quota via the
`get_youtube_quota_with_reserved` SQL function and:

1. **Exports Prometheus gauges** `listener_quota_usage_percentage{platform="youtube",service="youtube-quota-monitor",quota_type="daily"}`
   and `listener_quota_remaining{...}` — the metrics the alert rules
   `YouTubeQuotaHigh/Critical/Exhausted` (in `caesar-deployment`) evaluate.
2. **Publishes `QuotaEvent`s** to the `quota:alerts` Redis Pub/Sub channel on state
   transitions (HEALTHY→DEGRADED→CRITICAL→EXHAUSTED→DEPLETED) and 5% threshold crossings
   (≥75%), plus a recovery event at the midnight-PT reset. The `discord-bot` subscribes
   to this channel and renders the events as Discord embeds.
3. **Sweeps stale quota reservations** (`cleanup_stale_quota_reservations()`) left by
   crashed processes — the housekeeping the listener used to own.

It also serves `GET /quota/status` in the same envelope the old listener did, so the
discord-bot's periodic status poll (`YOUTUBE_LISTENER_URL`) can target this service.

> **Deploy as a single replica.** The alert-dedup state (last state, last notified
> threshold) is in-memory; a second replica would double-publish alerts. Quota
> *accounting* itself is DB-atomic and safe under N writers — that is unaffected.

## Endpoints

| Method | Path             | Purpose                                            |
|--------|------------------|----------------------------------------------------|
| GET    | `/health/live`   | Liveness (always 200).                             |
| GET    | `/health/ready`  | Readiness — 200 only when Postgres + Redis reachable. |
| GET    | `/metrics`       | Prometheus metrics.                                |
| GET    | `/quota/status`  | `{global:{...}, channels:[]}` for the discord-bot poll. |

## Configuration

| Env | Default | Purpose |
|-----|---------|---------|
| `PORT` | `8093` | HTTP port |
| `GIN_MODE` | `debug` | `release` in production |
| `LOG_LEVEL` | `info` | Zap log level |
| `DATABASE_HOST/PORT/USER/PASSWORD/NAME` | localhost/5432/allchat/.../allchat | Postgres |
| `REDIS_HOST/REDIS_PORT` | localhost/6379 | Redis (alert publishing) |
| `QUOTA_MONITOR_INTERVAL` | `30s` | Table poll interval (`time.ParseDuration`) |
| `QUOTA_CLEANUP_INTERVAL` | `5m` | Stale-reservation sweep interval |
| `QUOTA_NOTIFIER_ENABLED` | `true` | Toggle `quota:alerts` publishing |
| `QUOTA_ALERT_CHANNEL` | `quota:alerts` | Redis Pub/Sub channel |
| `QUOTA_HEALTHY/DEGRADED/CRITICAL/EXHAUSTED_THRESHOLD` | `70/85/95/100` | State thresholds (%) |

## Tests

`go test ./...` — the monitor's state/threshold/recovery logic is unit-tested with a fake
reader + fake notifier (no DB/Redis); the `/quota/status` envelope and the `QuotaEvent`
JSON contract (in `shared/quota`) are locked by tests.
