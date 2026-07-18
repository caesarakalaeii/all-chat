# ADR-0039: Database connection budget (bounded pools per service)

**Date**: 2026-07-18
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Every Go service builds its own PostgreSQL pool through the shared
`shared/database.NewPostgresPool*` helpers. That helper hardcoded:

```go
config.MaxConns = 20   // per service instance
config.MinConns = 5    // keep warm
```

`MinConns` is a floor the pool keeps open permanently. With ~40 service
instances (≈18 services, several replicated) that is `40 × 5 = ~200`
connections held **idle** at all times — and the shared CNPG cluster
(`allchat-cluster`) was configured with `max_connections: "200"`.

The result: Postgres sat pinned at its ceiling with ~197 idle connections and
**zero headroom**. Running pods were fine (they already held their
connections), but any event that started new pods — a node failure, a rolling
deploy, or manual pod moves — made the new pool's connection attempts fail with:

```
FATAL: remaining connection slots are reserved for roles with the
SUPERUSER attribute (SQLSTATE 53300)
```

so the pod hit a fatal DB-ping at startup and `CrashLoopBackOff`ed until older
connections churned. This was observed on 2026-07-18 during the
`caesar-control-3` OOM incident (ADR-0038 in caesar-deployment): the reschedule
churn crashlooped `twitch-eventsub-listener` several times before it recovered.

A secondary problem: all connections were opened with an empty
`application_name`, so `pg_stat_activity` could not attribute connections to a
service, making the leak impossible to diagnose from the database side.

## Decision

Treat database connections as a **budget**: `Σ(instances × MaxConns)` must stay
comfortably under the cluster's `max_connections`. Concretely:

1. **Shrink the pool defaults** in `shared/database`: `MaxConns 20 → 10`,
   `MinConns 5 → 1`. This drops the permanent idle footprint from ~200 to ~40.
2. **Make pool sizes env-configurable** via `DATABASE_MAX_CONNS` /
   `DATABASE_MIN_CONNS`, so a genuinely DB-heavy service can be raised without a
   code change (and the budget stays explicit).
3. **Set `application_name`** on every connection, resolved from
   `DATABASE_APP_NAME` → `OTEL_SERVICE_NAME` → `HOSTNAME` (the pod name is
   always present), restoring per-service attribution in `pg_stat_activity`.
4. **Raise `max_connections` 200 → 300** on the `allchat-cluster` CNPG cluster
   (caesar-deployment) for headroom. This is memory-safe: the primary used
   815Mi/2Gi at 200 connections and `work_mem` is only 2MB, so 300 stays well
   under the 2Gi limit. The higher ceiling also gives a safe window to roll out
   the smaller pools without hitting the ceiling mid-deploy.

## Consequences

Positive:
- Steady-state DB connections drop from ~197 to ~40, and the ceiling rises to
  300 — from **zero** headroom to ~260 slots. Mass restarts no longer crashloop.
- `pg_stat_activity.application_name` now names the owning pod, so future pool
  growth or leaks are diagnosable.
- Pool sizing is tunable per service via env, no rebuild required.

Negative / trade-offs:
- `MinConns = 1` means a service that has been idle keeps only one warm
  connection; the first concurrent query after a quiet period pays a
  connection-open (~ms). Negligible for these workloads.
- The budget is still additive: if the instance count grows enough that
  `instances × MaxConns` again approaches `max_connections`, per-pool tuning
  stops scaling.

Follow-up:
- If the service/replica count keeps growing, put a **PgBouncer** in front of
  the cluster (CNPG `Pooler`, transaction mode) so client pool sizes are
  decoupled from actual backend connections. Not done now because pgx defaults
  to prepared statements (extended protocol), which needs care in transaction
  mode; the pool-budget fix resolves the observed problem without that risk.
