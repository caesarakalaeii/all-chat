# ADR-0023: Decoupled YouTube Quota Monitor

**Date**: 2026-06-22
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Infrastructure Lead, Platform Team

## Context

The quota-based `youtube-listener` was the sole owner of YouTube quota observability (the Prometheus gauge `listener_quota_usage_percentage` and `QuotaEvent`s published to the `quota:alerts` Redis channel for the discord-bot). At decision time it was not deployed: the general chat path had moved to `youtube-listener-innertube`, which consumes no official quota. (ADR-0065 later revived `youtube-listener`, but only for opted-in, owner-verified, premium channels.) Two deployed services still spend quota against the shared table: `moderation-service` (bans, 50 units) and `auth-service` (streamer sends, 5 units).

With the listener undeployed: the Prometheus alert rules had no series to evaluate, the discord-bot subscriber was starved, auth-service coordinated sends over HTTP with the not-deployed listener and failed open (every send unaccounted), and nobody ran `cleanup_stale_quota_reservations()`.

## Decision

Treat the shared `youtube_quota_usage` table as the single source of truth for alerting, with one single-replica owner, and make auth-service count its sends directly.

Alternatives considered:

- **Re-deploy the quota-based listener**: reverses the InnerTube decision for the general channel population. Rejected (ADR-0065's later revival is independent).
- **Each consumer publishes/exports**: multi-replica duplicate alerts, alert logic smeared across services, stale gauges, fiddly cross-process dedup. Rejected.
- **Leader-gated monitor inside moderation-service**: couples alerting to moderation's lifecycle and needs leader election it does not have. Rejected.

What shipped:

- **Extracted canonical `shared/quota`**: the quota state machine, `QuotaEvent` + `Notifier` (publishes `quota:alerts`), and the direct-SQL reserve-confirm-rollback `Reserver`. moderation-service imports it; its local copy is deleted.
- **auth-service switched send accounting to the direct-SQL `Reserver`**, keeping the fail-open behaviour (a quota hiccup never blocks a streamer's own chat).
- **New single-replica `youtube-quota-monitor` service**: on an interval (default 30s) reads `get_youtube_quota_with_reserved(today_PT)` and exports `listener_quota_usage_percentage{platform="youtube",service="youtube-quota-monitor",quota_type="daily"}` and `listener_quota_remaining`, publishes deduped state-transition / threshold-crossing / recovery `QuotaEvent`s for the discord-bot, sweeps stale reservations, and serves `GET /quota/status` in the same envelope the listener did (the discord-bot's poll is repointed at it).

One replica (`Recreate`) because alert dedup state is in-memory; quota *accounting* is DB-atomic and safe under N writers.

## Consequences

- Both alert paths restored with a single owner; auth-service sends are counted; canonical `shared/quota` removes divergence between moderation and the listener.
- A single-replica SPOF for *alerting* (not accounting), mitigated by liveness/readiness and the Prometheus path being per-pod.
- `QuotaState` logic is duplicated between `shared/quota` and the listener; consolidate if the official-API mode grows again.

## Implementation

- `shared/quota/` — `state.go`, `event.go`, `notifier.go`, `reserver.go`.
- `services/youtube-quota-monitor/` — `monitor/` (reader + eval loop), `handlers/` (`/health/*`, `/quota/status`), `cmd/main.go`, Dockerfile.
- `services/auth-service/handlers/chat_send.go` — send quota now reserves via `shared/quota`.
- `services/moderation-service/` — imports `shared/quota`.
- Deployment: in `deployments/docker-compose.yml`; production manifests live in the deployment repo.

## Related

- Builds on [ADR-0006](./0006-youtube-quota-tracking.md) (reserve-confirm-rollback over the shared table).
