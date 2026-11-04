# All-Chat: Scaling & Performance

**Version**: 2.1
**Last Updated**: 2026-11-04
**Status**: Production Ready (with known limitations)

---

## Table of Contents

1. [Introduction](#introduction)
2. [Scalability Analysis](#scalability-analysis)
3. [Service Scaling Strategies](#service-scaling-strategies)
4. [Infrastructure Scaling](#infrastructure-scaling)
5. [Performance Bottlenecks](#performance-bottlenecks)
6. [Capacity Planning](#capacity-planning)
7. [Optimization Techniques](#optimization-techniques)

---

## Introduction

All-Chat is designed to scale horizontally across all components. This document provides scaling strategies, performance optimization techniques, capacity planning guidance, and **honest assessment of current limitations**. Replicas, HPAs and pools described here match the production manifests in the ArgoCD GitOps repo (`caesar-deployment/apps/workloads/all-chat/`, not in this repo); local dev (`deployments/docker-compose.yml`, `deployments/k8s/` k3d) is thinner and not treated as scaling evidence here.

**See the system map in [00-OVERVIEW.md](./00-OVERVIEW.md#system-overview)**.

### Scaling Principles

- **Horizontal scaling**: Add more replicas (preferred for stateless services)
- **Vertical scaling**: Increase CPU/memory (for databases, Redis)
- **Caching**: Reduce database load (emote cache, Redis)
- **Backpressure**: Bounded per-batch concurrency and stream trimming prevent overload
- **Replica 1 caps**: Singleton services (discord-bot, discord-listener, support-bot, token-refresh-service, tiktok-signer, youtube-quota-monitor) deliberately run 1 replica to prevent duplicate side effects; leader-election services keep an active/standby split instead

### Current Capacity

Sustained throughput has never been the observed constraint at current traffic: producer (listeners) and consumer (message-processor) scale horizontally, and the message-processor pods idle far below their capacity at present volumes. No measured messages/second figure exists — the numbers below are mechanisms and limits, not measurements.

**Bottlenecks** (see [Scalability Analysis](#scalability-analysis) for detail):
1. **Redis HA** — Sentinel failover + HAProxy routing, no sharding/cluster; a single master at the moment of failure blocks writes for the failover window
2. **PostgreSQL single-primary writes** (read replicas exist but the app reads through the pool on the primary)
3. **Per-platform rate limits** (Twitch IRC/EventSub, Kick subscription cap, YouTube Data API quota)

---

## Scalability Analysis

### Message Flow

The listeners scale differently because of their connection model: connection-count brokers that join channels per chatroom scale out (with per-stream Redis locks, ADR-0007), while account-bound or bot-token-bound listeners cannot run more than one effective replica.

```
Listeners (per platform; see services/)
         ↓
              Redis Streams (chat:raw, MAXLEN 100,000)
                        ↓
              Message Processor
              (3 replicas, consumer group, per-batch concurrency 16)
                        ↓
              Redis Pub/Sub (overlay:{id})
                        ↓
              API Gateway WebSocket
              (3 replicas + 1 beta, per-overlay buffers carry sessions across pod restarts)
```

**Notes**:

- Twitch chat is ingested via EventSub (`twitch-eventsub-listener`, 2 replicas, leader election: 1 active + 1 standby). The legacy IRC listener (`twitch-listener`, also 2 replicas with its own HPA) runs in `"enforce"` mode per ADR-0026 and joins no channels.
- **Redis Streams trimming**: all listeners publish `chat:raw` with `MaxLen 100,000` (engagement streams cap at 10,000). The consumer reads batches of 100 (`ReadCount`) with a 5s `XREADGROUP` block, per-batch concurrency 16 (ADR-0033, `MP_PROCESS_CONCURRENCY`). A stalled consumer discards messages oldest-first beyond the cap; `MESSAGE_AGE_CUTOFF_SECONDS` (default 60) also drops stale messages on read, so a lagging consumer does not replay stale chat after recovery.
- **Redis Pub/Sub fan-out**: each message is delivered once per subscribed gateway pod, so delivery work grows linearly with replica count. At 3-4 gateway pods this bounds the multiplication; gateway pods buffer per overlay so a pod restart replays recent messages (ADR-0069, session spans).
- **Backpressure**: the per-batch semaphore blocks the consumer between reads, providing natural backpressure when enrichment is slow; trimming + age cutoff bound the stream size so a burst does not grow memory without limit.

### Realistic Capacity

No measured sustained msg/s figure exists for this stack and traffic today. Rough mechanism-level ceilings:

- Single Redis pub/sub delivery capacity comfortably exceeds current need at the ~3 gateway replicas in place.
- A message-processor pod sustains roughly `concurrency / per-message-latency` messages per second under ADR-0033 (concurrency 16, I/O-bound: Redis round-trips plus HTTP calls on cache miss), reading batches of 100 per `XREADGROUP`.
- Per-message enrichment latency on cache hit is single-digit ms; on miss it is bounded by upstream HTTP calls (emote-service, pronoun, avatars) with negative caching of upstream failures so a degraded upstream degrades throughput per-message rather than collapsing it.

---

## Service Scaling Strategies

### API Gateway

**Prod**: 3 replicas; **beta**: 1 replica (`api-gateway-beta-deployment.yaml`). No HPA exists for the gateway; scale via the GitOps repo.

**Capacity rules**:

- Single-pod WebSocket connection capacity is bounded by memory (limit 1Gi, cpu 1000m). Per-connection memory is not measured; the historical "2,500 connections per pod" figure was modelled, not measured, and is not kept.
- Gateway replicas are stateless for session lookup: sessions replay from Redis/DB-backed buffers across pod restarts (ADR-0069), so scale-out is safe and reconnects do not need sticky routing.

**Scaling**: production is ArgoCD/GitOps-managed; replica changes go through the caesar-deployment repo (see [02-DEPLOYMENT.md](./02-DEPLOYMENT.md)). No per-overlay connection cap exists in api-gateway code (the "1,000 connections per overlay" limit in the previous revision was never verified).

### Message Processor

**Prod**: 3 replicas. No HPA. Scales horizontally; add pods via the GitOps repo.

**Mechanisms**:

- Consumes `chat:raw` via XREADGROUP consumer group `message-processor`, batches of 100, 5s read block.
- **Per-batch concurrency 16** (`DefaultProcessConcurrency`), overridable per pod via `MP_PROCESS_CONCURRENCY` (ADR-0033). Semaphore-bounded over the batch, and the batch completes before the next read, bounding in-flight messages.
- Lag visibility: `XPENDING` counts on the PEL are logged and exported for dashboards (see [04-OBSERVABILITY.md](./04-OBSERVABILITY.md)); no lag-based HPA is configured.
- Resource requests/limits (raised per the deployment manifest note "memory-heavy under concurrent emote enrichment, ADR-0033"): requests 256Mi/200m, limits 1Gi/1000m.

**Scaling triggers in practice**: sustained consumer lag growth, or enrichment-latency amplification from a slow upstream (mitigated by negative caching in the enrichers). Multiple pods partition the same stream via the consumer group; no coordination is needed.

### Listeners

**General pattern**: 1–2 replicas depending on the platform's connection model, each with its own affinity via Redis-locked stream split or leader election (ADR-0007).

| Service | Prod replicas | HPA | Notes |
|---|---|---|---|
| `twitch-eventsub-listener` | 2 | none (pinned) | Leader election: 1 active, 1 standby |
| `twitch-listener` | 2 | `twitch-listener-hpa.yaml` (min 2, max 5, CPU 70% / mem 80%) | ADR-0026 `"enforce"` mode — joins no channels |
| `youtube-listener-innertube` | 2 | none | Default YouTube path (quota-free InnerTube polling); canary coordination sidecar (ADR-0065) |
| `youtube-listener-official` | 2 | none | Premium-only Data API listener (ADR-0065), leader-coordinated; consumed only for claimed channels |
| `kick-listener` | 1 | `kick-listener-hpa.yaml` (min 1, max 5, CPU 70% / mem 80%) | Centrifugo or Pusher; connection pool ≤100 chatrooms/conn (ADR-0077) |
| `tiktok-listener` | 2 | none | Leadership election for discovery; signer (`tiktok-signer`) is a separate 1-replica deployment |
| `discord-listener` | 1 | none (pinned) | One Gateway connection per bot token — cannot run >1 effective replica |
| `discord-bot` | 1 | none | Duplicate-message prevention |
| `owncast-listener` | 1 | platform-listeners-hpa (owncast/goodgame/picarto/altstream, min 1 max 5, CPU 70% / mem 80%) | One websocket per instance URL (ADR-0058) |
| `goodgame-listener` | 1 | (same HPA file) | Beta platform |
| `picarto-listener` | 1 | (same HPA file) | Beta platform |
| `altstream-listener` | 1 | (same HPA file) | Early access, ADR-0082 |
| `facebook-listener` | 1 | none | Beta |
| `tiktok-signer` | 1 | none | Headless-browser signer; scale adds concurrency cost per replica |
| `youtube-quota-monitor` | 1 | none | Single owner of quota state machine (ADR-0023) |

Note: all listener HPAs (`kick-listener-hpa.yaml`, `twitch-listener-hpa.yaml`, `platform-listeners-hpa.yaml`) use identical resource metrics (CPU 70% / memory 80% utilization). Custom pod metrics exposed by the services — `gateway_websocket_connections_active`, `listener_active_sources_total` (`shared/metrics`) — are exported for Prometheus/Grafana but are not configured as HPA metrics today.

### YouTube Listener (quota-related scale)

Two listeners coexist after ADR-0065:

- **`youtube-listener-innertube`** (2 replicas): default for every unclaimed channel; polls via InnerTube, costs **zero Data API quota** (except optional subscriber-alert polling, which spends accounted Data API units). Uses leadership (source-manager SDK leadership, sidecar canary coordinator `youtube-canary`) to avoid duplicating discovery across replicas.
- **`youtube-listener-official`** (the `youtube-listener` service, 2 replicas): premium-only Data API path for owner-opted-in channels (unlisted + members-only streams). Discovery takes the channel away from innertube via claims (refreshed by the global-sync leader every 60s); for streams innertube cannot access, pinned stream links replace quota-costly `search.list` calls.

**Quota**:

- Limit: 1,009,000 units/day (Google-increased project quota; default in `shared/quota/state.go` as `DefaultDailyQuota`, injected as `QUOTA_LIMIT_DAILY` from the cluster secret).
- Tracking: reserve-confirm-rollback in `shared/quota/reserver.go` (Postgres-backed `youtube_quota_usage` table, per-day Pacific time window).
- Observability/sweeper: `youtube-quota-monitor` (1 replica) is now the single owner of the quota state machine, emits `listener_quota_*` Prometheus metrics and `quota:alerts` for the discord-bot, and sweeps abandoned reservations (ADR-0023).
- The historical `10,000 units/day` bottleneck applied when the Data API listener was the only YouTube path. With InnerTube as default, quota is no longer the scaling ceiling; it recedes to a constraint for the premium official path only.

### Kick Listener

- **Prod**: 1 replica, HPA to max 5 (CPU 70% / mem 80%).
- Realtime provider chosen by Kick at startup: Centrifugo or Pusher (ADR-0077).
- **Centrifugo mode** scales out via a **connection pool of at most 100 chatrooms per connection** (below Kick's hard limit of 128 subscriptions per connection, error `106 limit exceeded` on the 129th), each connection with its own guest token and refresh cycle. Roughly one pool connection per 100 chatrooms; ADR-0077 cites ~600 chatrooms currently watched.
- Pod memory limit: 512Mi (kick-listener deployment). Connection count, not memory, is the driver of pods-per-chatroom-count.

---

## Infrastructure Scaling

### PostgreSQL (CloudNativePG)

**Current**: 3-instance CNPG cluster (`allchat-cluster` in caesar-deployment), 1 primary + 2 replicas, pod anti-affinity required per node, local-path PVCs (node-pinned; see the caution notes in the manifest for re-provisioning co-located instances).

**Capacity**:

- `max_connections: "300"` (raised from 200 per ADR-0039), `shared_buffers 256MB`, `work_mem 2MB`, `effective_cache_size 1GB`.
- Resource requests: 500m CPU/512Mi, limits 2 CPU/2Gi per instance. The manifest notes the primary used 815Mi/2Gi at 200 connections, so 300 stays memory-safe.
- Storage: 20Gi per instance (PVC); WAL and backups archive to Hetzner Object Storage via `backup-objectstore.yaml` / `backup-offsite-mirror-cronjob.yaml`.

**Scaling**:

- **Connection budget**: shared/pgx pools default `MaxConns 10` / `MinConns 1` (env-overridable per pod via `DATABASE_MAX_CONNS` / `DATABASE_MIN_CONNS`), with `application_name` tagging for `pg_stat_activity` attribution (ADR-0039). `Σ(instance count × MaxConns)` must stay under 300. PgBouncer is not deployed; the budget replaces external pooling.
- **Read scaling**: 2 CNPG replicas exist for HA, not read load-balancing; all reads route through the pgx pool to the primary today. Read-only service routing is available via CNPG's `-rw`/`-r` services but is not wired into app code.
- **Writes** scale vertically via instance resources and up-front tuning; no horizontal write scaling exists in Postgres.

### Redis HA (Sentinel + HAProxy)

**Current**: Sentinel-managed HA, not single-instance and not Redis Cluster:

- **Data nodes** (`redis-ha-node.yaml`): 3 nodes, 1 master + 2 replicas, `min-replicas-to-write 1` / `min-replicas-max-lag 10`, AOF persistence on, 8Gi storage per node, 1 CPU/2560Mi resource shape.
- **Sentinel** (`redis-ha-sentinel.yaml`): 3 sentinels, failover orchestration, `podManagementPolicy: Parallel`.
- **HAProxy** (`redis-ha-haproxy.yaml`): 2 replicas TCP-proxying `:6379` to whichever node currently reports `role:master` via inline `info replication` checks every 1s. Clients keep using the `redis` Service unchanged (`REDIS_HOST` unmodified); long-lived pub/sub subscriptions and blocking `XREADGROUP` reads are not severed by the routing change, per the HAProxy config notes.

**Implications**:

- No **Redis Cluster** (no sharding, no cross-node key distribution). Multi-instance throughput still rides a single master's CPU; scaling under very high message volume means raising this `redis-node` resource profile, moving to a sharded topology, or splitting Streams and Pub/Sub onto separate deployments (not implemented).
- **Failover window**: Sentinel detection + promotion adds a bounded write-gap under failover. Streams tail behind after reconnect; gateway pods buffer per overlay so overlays replay recent messages across brief drops (ADR-0069). No durable broker (Kafka/NATS) replaces this today.

---

## Performance Bottlenecks

### 1. Redis Pub/Sub Fan-out (⚠️ moderate)

**Problem**: each message is delivered once per gateway-pod subscriber; SQL/load grows O(pods). Not currently capacity-limiting at 3–4 gateway pods, but it grows linearly.

**Mitigation**: pod-count stays small; per-overlay Redis channels are short-lived (created/pinned at subscribe time); further scale would need a fan-out layer or sharded pub/sub, neither of which exists today.

### 2. Single-master Redis

**Problem**: all Streams writes and Pub/Sub publishes land on one master node; a failover drops writes for the promotion window.

**Mitigation**: Sentinel + HAProxy bounds the gap; `min-replicas-to-write 1` prevents split-brain writes. No cluster/sharding deployed (see [Infrastructure Scaling](#infrastructure-scaling)).

### 3. Emote Enrichment Latency (largely mitigated, ADR-0033)

**Problem**: enrichment calls 7TV/BTTV/FFZ (via emote-service), Alejo (pronouns), avatar/badge fetchers. On cache miss these are HTTP round-trips; during a prior upstream degradation incident, serial per-message processing dropped 3 pods' combined throughput to ~3 msg/s (ADR-0033 motivation), now mitigated by:

- ✅ Bounded per-batch concurrency (16/pod), so a single slow upstream no longer collapses throughput.
- ✅ Emote cache (Redis, ~1-hour TTL), 7TV EventAPI WebSocket updates.
- ✅ Negative caching for pronoun lookup failures (5m) so erroring upstreams are not re-hit per message.
- ✅ Avatar/badge and viewer badge caches, Twitch bits/cheermote Redis cache.

### 4. PostgreSQL Single-Primary Writes

**Problem**: all writes go through the primary; replica instances serve HA only (see above).

**Mitigation**: batch writes in hot paths (bulk inserts in engagement/alert processors), Redis caches for frequently-read data, and the connection budget (ADR-0039) keeps post-outage restart storms from pinning the DB.

### 5. Twitch Platform Limits

**Problem**: Twitch chat no longer rides the IRC rate limits (ADR-0026), but EventSub brings per-app webhook limits and subscription management overhead.

**Mitigation**: eventsub listener is leader-elected (1 active/1 standby); the IRC listener runs in ADR-0026 `enforce` mode and joins nothing, so IRC JOIN limits no longer apply. See [QUICK-REF-ADD-PLATFORM](../llm-guides/QUICK-REF-ADD-PLATFORM.md) for platform limits in general.

---

## Capacity Planning

### Prod replica / resource summary

Current shape (caesar-deployment, verified at this doc's audit date):

| Component | Replicas | Shape |
|---|---|---|
| api-gateway | 3 | 200m/256Mi req, 1 CPU/1Gi limit |
| api-gateway-beta | 1 | |
| frontend | 2 | |
| frontend-beta | 1 | |
| message-processor | 3 | 256Mi req → 1Gi limit, 1 CPU |
| alert-processor | 2 | |
| engagement-service | 2 | |
| moderation-service | 2 | |
| overlay-manager | 2 | |
| auth-service | 2 | |
| emote-service | 2 | |
| share-service | 2 | |
| source-manager | 2 | |
| payment-service | 1 | |
| token-refresh-service | 1 | (singleton: no duplicate refreshes) |
| media-service | 2 | |
| twitch-eventsub-listener | 2 | |
| twitch-listener | 2 | (enforce mode) |
| youtube-listener-innertube | 2 | |
| youtube-listener-official | 2 | |
| youtube-quota-monitor | 1 | |
| kick-listener | 1 → HPA max 5 | |
| tiktok-listener / tiktok-signer | 2 / 1 | |
| discord-listener / discord-bot / support-bot | 1 each | (singleton) |
| owncast / goodgame / picarto / altstream / facebook / instagram listeners | 1 each | (HPA files cover first four) |
| redis (data + sentinel + haproxy + legacy `redis` Service) | 3/3/2 | |
| postgres (CNPG) | 3 instances | 20Gi each |
| retention-sweeper / key-rotator | CronJobs | |

Current workers: `caesar2`, `caesar3`, `caesar4` plus a 3-node K3s HA control plane (see the caesar-deployment README). The legacy `redis-deployment.yaml` Service remains and targets the HA nodes; see `REDIS-HA-RUNBOOK.md` in caesar-deployment's `apps/workloads/all-chat/`.

### Scaling out

Practical steps, in order of confidence:

1. **Add replicas to stateless services** (overlay-manager, auth-service, emote-service, share-service, moderation-service) via the GitOps repo. These have no HPA; scale by hand or add HPAs there.
2. **Add message-processor pods** when consumer lag is sustained. They attach to the consumer group; stream partitioning is automatic, nothing else needs configuring.
3. **Raise kick-listener HPA max** if Kick pool connections per pod saturate (owncast/goodgame/picarto/altstream HPA file shares the same pattern).
4. **Add gateway pods** when CPU or connection load demands it; memory is the tighter frame (replay buffers). Keep the count low to bound Pub/Sub fan-out.
5. **Redis/Postgres vertical resources** in the deployment repo; horizontal write scaling is not currently supported for either without further work (sharding / read routing).

### Cost profile and infrastructure identity

Infrastructure is a K3s cluster with a 3-node HA control plane plus 3 workers (caesar-deployment README); off-site backups go to Hetzner Object Storage. Node sizing and maintenance flows live in the caesar-deployment repo; no cost tables are kept here.

**Beyond the current shape**: neither Redis Cluster, a persistent message queue (Kafka/NATS) nor multi-region is deployed or planned in code. Any such step needs a new ADR.

---

## Optimization Techniques

### Caching Strategy

1. **Emote Cache** (Redis, ~1-hour TTL): 7TV, BTTV, FFZ emotes cached per channel; real-time updates via 7TV EventAPI WebSocket.
2. **OAuth Token Cache** (Redis, until expiry): YouTube OAuth tokens cached per user; automatic refresh when expired (`shared/youtubetoken`).
3. **Channel metadata caches**: Kick chatroom resolutions, subscriber-alert counts, InnerTube live-chat-ID resolutions (ADR-0025).
4. **Avatar/badge/cheermote caches** (Redis); viewer badges resolved from Postgres with caching (ADR-0033 references).

### Connection Pooling

1. **PostgreSQL** (pgx, shared/database + ADR-0039):
   - `MaxConns: 10` default per instance (env `DATABASE_MAX_CONNS`)
   - `MinConns: 1` (env `DATABASE_MIN_CONNS`)
   - Recycle: 1h lifetime, 10m idle-time, 1m health check
   - `application_name` tagging for per-service attribution
   - Cluster budget: `Σ(instance count × MaxConns)` ≤ 300 (CNPG `max_connections`)
2. **Redis** (go-redis via `shared/redis`):
   - `PoolSize: 50` per instance
   - `MinIdleConns: 10`

### Message Age Filtering

- **Message Processor**: ignores messages older than 60s (`MESSAGE_AGE_CUTOFF_SECONDS`, default 60).
- Stale messages from a lagging consumer are dropped rather than replayed; combined with `MaxLen` trimming, this keeps a recovered consumer from flooding overlays with minutes-old chat.

### Leader Election / Stream Locks

- **twitch-eventsub-listener**: leader election, 1 active + 1 standby replica — prevents duplicate EventSub webhook/API churn.
- **youtube listeners**: leadership coordination (source-manager SDK `LeadershipListener`) for stream discovery/discovery claims; innertube adds a sidecar canary coordinator (`youtube-canary`).
- **source-manager** (2 replicas): coordinates the active-source registry and per-stream leadership claims/renewals/releases via Redis-backed locks (endpoints `/leadership/claim`, `/leadership/renew`, `/leadership/release`). Peer-aware rebalancing redistributes streams on scale-out instead of leaving new pods at zero (ADR-0007).
- **Singletons**: discord-bot, discord-listener, support-bot, token-refresh-service, tiktok-signer, youtube-quota-monitor run 1 replica each by design (duplicate side effects otherwise).

---

## Summary

**Current Architecture**:
- WebSocket delivery via 3 prod + 1 beta gateway pods, replay-buffered across pod restarts (ADR-0069)
- Internal message flow scales horizontally via consumer groups and per-stream leadership locking (ADR-0007)
- Per-batch concurrency 16 (ADR-0033) keeps a degraded upstream from collapsing throughput; negative caching bounds retry amplification
- Postgres connection budget (ADR-0039): 10/1 pool defaults across ~40 service instances under CNPG `max_connections: 300`
- Redis HA via Sentinel (3) + HAProxy routing (2) + data nodes (3), `min-replicas-to-write 1`
- YouTube scale is decoupled from Data API quota for the default path (InnerTube); quota (1,009,000 units/day) applies to the premium official path, tracked reserve-confirm-rollback and monitored/exported by `youtube-quota-monitor` (ADR-0023)

**Known Limitations**:
- ⚠️ Redis Pub/Sub fan-out grows linearly with gateway pods (3–4 replicas today)
- ⚠️ Single-master Redis; failover is bounded write-gap, not zero-downtime; no Redis Cluster sharding
- ⚠️ Postgres writes single-primary; replicas are HA-only, no read routing wired into code
- ⚠️ Postgres `max_connections 300` is a real budget with the ~40-instance footprint; per-service overrides (`DATABASE_MAX_CONNS`) must be accounted (ADR-0039)

**For detailed deployment steps**, see:
- [02-DEPLOYMENT.md](./02-DEPLOYMENT.md) - Kubernetes deployment (production is ArgoCD/GitOps in caesar-deployment)
- [04-OBSERVABILITY.md](./04-OBSERVABILITY.md) - Metrics and monitoring
- [QUICK-REF-SCALING.md](../llm-guides/QUICK-REF-SCALING.md) - Quick reference for scaling operations
