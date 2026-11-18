# ADR-0003: CloudNativePG for PostgreSQL

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-18 (backups and PITR run through the Barman Cloud plugin)
**Deciders**: Infrastructure Team, Database Lead

## Context

The platform needs a highly available PostgreSQL for user accounts, overlay and source configuration, OAuth token storage, and quota tracking. Requirements: operator-driven failover (not manual promotion), WAL durability, automated backups with point-in-time recovery, and built-in monitoring, with minimal routine ops.

Alternatives rejected:

- Manual StatefulSet: full control, but failover, backup and PITR are hand-written scripts, error-prone and slow (minutes to recover).
- Zalando postgres-operator: mature, but Patroni adds an extra HA layer, PITR needs external tooling, and the team had no production experience with it.

## Decision

Run PostgreSQL via the CloudNativePG (CNPG) operator. The team already has production experience with it; the single `Cluster` CRD gives automated failover (~20-25s measured in early deploys), continuous WAL archiving with PITR, Prometheus metrics through the PodMonitor, and opensource cost. Trade-offs: an operator dependency to keep updated, internals partly hidden behind the CRD, and a smaller community than Zalando (helped by good official docs).

## Implementation

The CNPG `Cluster` manifest and backups are owned and applied by the deployment pipeline, not this repo. What lives here:

- `deployments/k8s/base/postgres/deployment.yaml`: plain single-replica `postgres:16-alpine` Deployment + Service + PVC for local k3d dev (not the CNPG cluster).
- `deployments/k8s/base/configmap.yaml`: sets `DATABASE_HOST: allchat-cluster-rw.allchat.svc.cluster.local`, `DATABASE_USER: allchat_user`; each service reads `DATABASE_PASSWORD` from the `allchat-secrets` Secret. `allchat-cluster-secret` is the cluster's app-user secret.
- `deployments/docker-compose.yml`: dev compose runs `postgres:16-alpine`.

Cluster shape (deployed spec): 3 instances, one per node (`podAntiAffinityType: required`), 20Gi storage, `max_connections: 300` (headroom for the ADR-0039 connection budget), `synchronous_commit` at its default `on` (RPO 0), PodMonitor enabled, extensions `uuid-ossp` and `pg_trgm`.

Connection budget (ADR-0039): every service bounds its pool via `shared/database.NewPostgresPool*` (`DATABASE_MAX_CONNS`/`DATABASE_MIN_CONNS`, defaults 10/1). A Pooler is not deployed: services connect to `allchat-cluster-rw` directly (no read replicas to load-balance). At `max_connections = 200` the per-service pools pinched the ceiling and crashlooped starting pods (SQLSTATE 53300); the pools were shrunk and the cap raised to 300.

Backups (deployment-pipeline-owned):

- The Barman Cloud plugin archives WAL continuously and a `ScheduledBackup` takes a nightly base backup (23:00 UTC) to an `ObjectStore` on Hetzner Object Storage; 30-day retention policy and 5-minute `archive_timeout`.
- A nightly CronJob copies the Barman archive to Backblaze B2 so Hetzner is not the only copy.
- Restore is a new `Cluster` with `bootstrap.recovery` from that `ObjectStore`, never the live cluster.
- Because WAL archiving depends on those stores, a wrong key or missing bucket stalls archiving until the 20Gi disk fills. Alerts `AllChatPostgresWALArchivingFailing` (failing/stalled archiving) and `AllChatPostgresBackupStale` (base backup older than 30h) cover that.

A restore from the Barman archive into a scratch cluster has not been exercised yet; until it is, the backups are unproven.

Debugging: `kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat` (verify the current primary with `kubectl get cluster allchat-cluster -n allchat`).

Related: ADR-0039 (connection budget), ADR-0089 (retention sweeper), [docs/architecture/02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md).

References: https://cloudnative-pg.io/documentation/
