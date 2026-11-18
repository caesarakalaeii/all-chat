# ADR-0023: Decoupled YouTube Quota Monitor

**Date**: 2026-06-22
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Infrastructure Lead, Platform Team

---

## Context and Problem Statement

The quota-based `youtube-listener` (ADR-0006) was the single owner of YouTube Data API
quota observability: it ran the in-memory quota `Tracker`, exported the Prometheus gauge
`listener_quota_usage_percentage{platform="youtube"}`, and published `QuotaEvent`s to the
`quota:alerts` Redis Pub/Sub channel that the `discord-bot` renders as Discord embeds.

That service was **not deployed** at the time — the listener path had moved to
`youtube-listener-innertube`, which reads chat via InnerTube and consumes **no** official
quota. (Only ADR-0065 later revived `youtube-listener`, for opted-in, owner-verified,
premium channels: [ADR-0065](./0065-premium-official-youtube-api-listener.md). The
InnerTube listener remains the default path for every other channel.) Two deployed
services still spend official quota against the shared table:

- `moderation-service` — bans (`liveChatBans.insert`, 50 units), counted via direct-SQL
  reserve-confirm-rollback (ADR-0006).
- `auth-service` — streamer sends (`liveChatMessages.insert`, 5 units).

With the quota listener not deployed, the consequences were:

1. **No metric exporter** → the Prometheus quota alert rules on
   `listener_quota_usage_percentage` had no series to evaluate. The deployed rules (in the
   caesar-deployment repo, `apps/platform/allchat-monitoring/`) are `YouTubeQuotaHigh` (>80%),
   `YouTubeQuotaCritical` (>95%) and `YouTubeQuotaExhausted` (>=100%); the copies under
   `deployments/k8s/monitoring/alerts/` in this repo are older (`YouTubeQuotaWarning` >80%,
   `YouTubeQuotaCritical` >90%);
2. **No publisher** → the still-running `discord-bot` `quota:alerts` subscriber was starved.
3. **`auth-service` wasn't even counting sends** — it coordinated quota over HTTP with the
   (then not deployed) `youtube-listener` and *failed open*, so every send went unaccounted.
4. **No owner for `cleanup_stale_quota_reservations()`**, which the listener used to run.

**Problem**: restore quota metrics + alerting (both the Prometheus and discord-bot paths),
driven by the real source of truth (the shared table), without making the general chat
path depend on official-quota listeners (ADR-0065 later revived the listener, but only
for opted-in premium channels; the monitor is what covers every other channel) — and
make `auth-service` actually count its sends so the numbers are correct.

---

## Decision Drivers

1. **Single source of truth**: the shared `youtube_quota_usage` table already aggregates
   all consumers' usage; alerting should read it, not any one consumer's local view.
2. **No duplicate alerts**: state-transition / threshold-crossing dedup needs a single owner;
   the consumers are multi-replica (auth=2, moderation=2).
3. **Reuse proven logic**: the listener's `QuotaState` machine, `QuotaEvent` schema, and
   `Notifier` already match the discord-bot contract and the Prometheus metric name.
4. **Minimal new surface, no coupling** to a consumer's lifecycle.

---

## Considered Options

1. **Re-deploy the quota-based `youtube-listener`** — rejected: for the general channel
   population it reverses the InnerTube decision and drags in stream discovery/polling
   nobody wanted, just for the tracker. (ADR-0065 later revived it, but only for opted-in,
   owner-verified, premium channels, not as the quota-alerting owner.)
2. **Publish + export from each consumer** (auth-service, moderation-service) — rejected:
   multi-replica double-publishing, alert logic smeared across services, gauge goes stale
   between calls, and cross-process dedup (Redis SETNX) is fiddly.
3. **Leader-gated monitor embedded in `moderation-service`** — rejected: couples quota
   alerting to moderation's lifecycle and needs leader election that service doesn't have.
4. **Dedicated single-replica `youtube-quota-monitor` reading the shared table** — chosen.

---

## Decision

**Extract a canonical `shared/quota` package** (the `QuotaState` machine, `QuotaEvent` +
`Notifier` that publishes to `quota:alerts`, and the direct-SQL `Reserver`). `moderation-service`
now imports it (its local copy is deleted), and **`auth-service` switches send accounting from
the dead HTTP coordination to the direct-SQL `Reserver`** — so sends are counted in the shared
table with the same reserve-confirm-rollback semantics as bans, keeping the prior fail-open
behaviour (a quota hiccup never blocks a streamer's own chat).

**Add a dedicated single-replica `youtube-quota-monitor` service** that, on an interval
(default 30s), reads `get_youtube_quota_with_reserved(today_PT)` and:

- exports `listener_quota_usage_percentage{platform="youtube",service="youtube-quota-monitor",quota_type="daily"}`
  and `listener_quota_remaining` so the existing Prometheus rules evaluate fresh data;
- publishes state-transition / 5%-threshold-crossing / recovery `QuotaEvent`s to `quota:alerts`
  (deduped in-memory) for the discord-bot;
- sweeps stale reservations via `cleanup_stale_quota_reservations()`;
- serves `GET /quota/status` in the same envelope the listener did, so the discord-bot's
  periodic poll is repointed at the monitor.

It runs as **one replica** (`Recreate`): the alert-dedup state is in-memory, so a second
replica would double-publish. Quota *accounting* is DB-atomic and remains safe under N writers.

---

## Consequences

**Positive**

- Both alert paths (Prometheus → Alertmanager, and Redis → discord-bot) are restored with a
  single owner and no duplicate alerts.
- `auth-service` sends are now counted, so the quota numbers reflect real total usage.
- The canonical `shared/quota` removes the divergence between moderation-service's and the
  listener's quota code.

**Negative / trade-offs**

- One more (small) deployable, and a single-replica SPOF for *alerting* (not for accounting).
  Mitigated by liveness/readiness + the Prometheus path being independent of any one pod.
- The `QuotaState` logic is duplicated between `shared/quota` and the listener (tech debt;
  consolidate if the listener's official-API mode ever grows again).

**Relationships**

- Builds on **ADR-0006** (reserve-confirm-rollback over `youtube_quota_usage`).
- Restores the metric/alert inputs assumed by the alerting stack (Prometheus/Alertmanager is
  the alerting source of truth; the ADR that defined the original alert rules lives in the
  caesar-deployment repo); the discord-bot Redis path complements, not replaces, Alertmanager.

---

## Implementation

- `shared/quota/` — `state.go` (states/thresholds/`CalculateState`), `event.go` (`QuotaEvent`
  + `Severity`), `notifier.go` (`Notifier` → `quota:alerts`), `reserver.go` (`Reserver`).
- `services/youtube-quota-monitor/` — `monitor/` (DB reader + evaluation loop), `handlers/`
  (`/health/*`, `/quota/status`), `cmd/main.go` (metrics + notifier + cleanup), Dockerfile.
- `services/auth-service/handlers/chat_send.go` — `reserveYouTubeSendQuota`/`settleYouTubeSendQuota`
  now use `shared/quota.Reserver` (direct SQL) instead of HTTP coordination.
- `services/moderation-service/` — imports `shared/quota`; local `quota/` package removed.
- Deployment: the monitor runs in the local dev `deployments/docker-compose.yml` (service
  `youtube-quota-monitor`, and `discord-bot`'s `YOUTUBE_LISTENER_URL` repointed to it). The
  production k8s manifests for this service and the ServiceMonitor/NetworkPolicy wiring live
  in the separate deployment repo, so they are not listed here.
