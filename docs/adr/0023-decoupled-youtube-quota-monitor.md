# ADR-0023: Decoupled YouTube Quota Monitor

**Date**: 2026-06-22
**Status**: ✅ Accepted
**Deciders**: Infrastructure Lead, Platform Team

---

## Context and Problem Statement

The quota-based `youtube-listener` (ADR-0006) was the single owner of YouTube Data API
quota observability: it ran the in-memory quota `Tracker`, exported the Prometheus gauge
`listener_quota_usage_percentage{platform="youtube"}`, and published `QuotaEvent`s to the
`quota:alerts` Redis Pub/Sub channel that the `discord-bot` renders as Discord embeds.

That service is **no longer deployed** — the listener path moved to `youtube-listener-innertube`,
which reads chat via InnerTube and consumes **no** official quota. But two deployed services
still spend official quota against the shared `youtube_quota_usage` table:

- `moderation-service` — bans (`liveChatBans.insert`, 50 units), counted via direct-SQL
  reserve-confirm-rollback (ADR-0006).
- `auth-service` — streamer sends (`liveChatMessages.insert`, 5 units).

With the listener gone, the consequences were:

1. **No metric exporter** → the Prometheus alert rules `YouTubeQuotaHigh/Critical/Exhausted`
   (ADR-0022) had no series to evaluate; YouTube quota alerting was silently dark.
2. **No publisher** → the still-running `discord-bot` `quota:alerts` subscriber was starved.
3. **`auth-service` wasn't even counting sends** — it coordinated quota over HTTP with the
   (now absent) `youtube-listener` and *failed open*, so every send went unaccounted.
4. **No owner for `cleanup_stale_quota_reservations()`**, which the listener used to run.

**Problem**: restore quota metrics + alerting (both the Prometheus and discord-bot paths),
driven by the real source of truth (the shared table), without reviving the listener — and
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

1. **Re-deploy the quota-based `youtube-listener`** — rejected: reverses the InnerTube
   decision and drags in stream discovery/polling we no longer want, just for the tracker.
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
  listener's quota code (Pathfinder cleanup).

**Negative / trade-offs**

- One more (small) deployable, and a single-replica SPOF for *alerting* (not for accounting).
  Mitigated by liveness/readiness + the Prometheus path being independent of any one pod.
- The `QuotaState` logic is duplicated between `shared/quota` and the undeployed listener
  (tech debt; consolidate if the listener is ever revived). The canonical version also fixes
  a latent bug in the listener's `calculateState` (the `healthy`/`exhausted` thresholds were
  unused/unreachable), so it now matches the documented state ranges.

**Relationships**

- Builds on **ADR-0006** (reserve-confirm-rollback over `youtube_quota_usage`).
- Restores the metric/alert inputs assumed by **ADR-0022** (Prometheus/Alertmanager is the
  alerting source of truth); the discord-bot Redis path complements, not replaces, Alertmanager.

---

## Implementation

- `shared/quota/` — `state.go` (states/thresholds/`CalculateState`), `event.go` (`QuotaEvent`
  + `Severity`), `notifier.go` (`Notifier` → `quota:alerts`), `reserver.go` (`Reserver`).
- `services/youtube-quota-monitor/` — `monitor/` (DB reader + evaluation loop), `handlers/`
  (`/health/*`, `/quota/status`), `cmd/main.go` (metrics + notifier + cleanup), Dockerfile.
- `services/auth-service/handlers/chat_send.go` — `reserveYouTubeSendQuota`/`settleYouTubeSendQuota`
  now use `shared/quota.Reserver` (direct SQL) instead of HTTP coordination.
- `services/moderation-service/` — imports `shared/quota`; local `quota/` package removed.
- Deployment (`caesar-deployment/apps/workloads/all-chat/`): `youtube-quota-monitor-deployment.yaml`
  (1 replica, `Recreate`), added to `kustomization.yaml`, the `allchat-listeners` ServiceMonitor,
  and a NetworkPolicy; the `discord-bot` `YOUTUBE_LISTENER_URL` is repointed to the monitor.
