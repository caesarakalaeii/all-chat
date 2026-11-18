# ADR-0039: Database Connection Budget (Bounded Pools per Service)

**Date**: 2026-07-18
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

Every Go service builds its PostgreSQL pool through the shared `shared/database.NewPostgresPool*` helpers, which hardcoded `MaxConns = 20` and `MinConns = 5`. `MinConns` is a floor kept open permanently, so ~40 service instances held ~200 idle connections against a CNPG cluster (`allchat-cluster`) configured with `max_connections: "200"` — zero headroom. Running pods were fine, but any event that started new pods (node failure, rolling deploy, manual moves) hit new pools with `FATAL: remaining connection slots are reserved for roles with the SUPERUSER attribute (SQLSTATE 53300)` at the startup DB ping and crashlooped until old connections churned; observed during the 2026-07-18 node-OOM incident. A secondary problem: connections were opened with an empty `application_name`, so `pg_stat_activity` could not attribute connections to a service.

## Decision

Treat connections as a budget: `Σ(instances × MaxConns)` must stay comfortably under `max_connections`.

1. **Shrink the defaults** in `shared/database`: `MaxConns 20 → 10`, `MinConns 5 → 1`. Permanent idle footprint drops from ~200 to ~40.
2. **Env-configurable sizing** via `DATABASE_MAX_CONNS` / `DATABASE_MIN_CONNS`, so a DB-heavy service can be raised without a code change.
3. **Set `application_name`** on every connection, resolved `DATABASE_APP_NAME` → `OTEL_SERVICE_NAME` → `HOSTNAME`, restoring per-service attribution in `pg_stat_activity`.
4. **Raise `max_connections` 200 → 300** on the `allchat-cluster` CNPG cluster (deployment repo) for headroom. Memory-safe: the primary used 815Mi/2Gi at 200 with `work_mem` at 2MB, and the higher ceiling gives a safe window to roll out the smaller pools mid-deploy.

## Consequences

- Steady-state connections drop to ~40 against a ceiling of 300, so mass restarts no longer crashloop; `pg_stat_activity` names the owning pod; pool sizing is tunable per service.
- `MinConns = 1` means the first concurrent query after a quiet period pays a connection open (~ms) — negligible.
- The budget is additive; if the instance count grows enough, per-pool tuning stops scaling.
- Applications connect to `allchat-cluster-rw` directly; no CNPG `Pooler` is deployed, and the per-pod budget is the only control in place. If the service/replica count keeps growing, the follow-up is a PgBouncer `Pooler` (transaction mode) to decouple client pools from backend connections — not done now because pgx defaults to prepared statements (extended protocol), which needs care in transaction mode.
