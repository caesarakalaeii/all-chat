# ADR-0003: CloudNativePG for PostgreSQL

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation; backups and PITR wired through the Barman Cloud plugin)
**Deciders**: Infrastructure Team, Database Lead

---

## Context and Problem Statement

All-Chat requires a **highly available PostgreSQL database** for:
- User accounts and authentication
- Overlay configurations
- Multi-source chat source mappings
- YouTube quota tracking (critical for rate limiting)
- OAuth token storage

**Production Requirements**:
1. **High Availability**: failover driven by the operator, not manual promotion
2. **Data Durability**: WAL durability on failover
3. **Backup & Recovery**: automated backups, Point-in-Time Recovery (PITR)
4. **Monitoring**: built-in metrics for replication lag, connection counts
5. **Operational Simplicity**: minimal manual intervention for routine operations

**Problem**: How do we deploy PostgreSQL in Kubernetes with HA, automated failover, and backups?

---

## Decision Drivers

1. **Team Experience**: Team has production experience with CloudNativePG (CNPG)
2. **Operational Simplicity**: Prefer declarative configuration (Kubernetes CRDs) over imperative scripts
3. **Automated Failover**: Must detect and promote replica automatically (<30s)
4. **PITR Support**: Point-in-Time Recovery for data corruption or human error
5. **Backup Automation**: No manual backup scripts, integrated with S3-compatible storage
6. **Cost**: Prefer open-source over managed services (cost optimization)
7. **Community Support**: Active development, good documentation, responsive maintainers

---

## Considered Options

### Option 1: Manual PostgreSQL StatefulSet

**Architecture**:
```yaml
# Manual setup with streaming replication
StatefulSet (3 replicas):
  - postgres-0 (primary)
  - postgres-1 (sync replica)
  - postgres-2 (async replica)

# Custom scripts for:
- Primary election (etcd or Consul)
- Failover detection and promotion
- Backup scheduling (cron jobs)
- PITR setup (WAL archiving to S3)
```

**✅ Pros**:
- **Full control**: No operator abstraction, direct PostgreSQL config
- **No dependencies**: No operator to maintain/upgrade
- **Flexible**: Can customize every aspect of PostgreSQL behavior
- **Zero learning curve**: Standard PostgreSQL knowledge applies

**❌ Cons**:
- **High operational burden**: Must write and maintain failover scripts
- **Error-prone**: Manual steps for promotion, replication setup, backup/restore
- **No automated PITR**: Must implement WAL archiving, recovery scripts manually
- **Slow failover**: Detecting primary failure + promoting replica = 2-5 minutes manual process
- **No monitoring**: Must build custom metrics exporters for replication lag
- **Risk of split-brain**: Without proper leader election, can have two primaries

**Estimated Setup Time**: 2-3 weeks (including testing failover scenarios)

---

### Option 2: Zalando PostgreSQL Operator

**Architecture**:
```yaml
# Zalando's postgres-operator
apiVersion: acid.zalan.do/v1
kind: postgresql
metadata:
  name: allchat-postgres
spec:
  numberOfInstances: 3
  patroni:
    initdb: ...
  volume:
    size: 50Gi
```

**✅ Pros**:
- **Mature**: Released 2016, battle-tested at Zalando (large scale)
- **Patroni-based**: Uses Patroni for HA (industry standard)
- **Connection pooling**: Built-in PgBouncer support
- **Logical backups**: pg_dump/restore integration
- **Large community**: Many production users, Stack Overflow answers

**❌ Cons**:
- **Patroni complexity**: Extra component to understand (DCS, leader election)
- **No native PITR**: Physical backups require external tools (WAL-G)
- **Team unfamiliar**: No production experience with Zalando operator
- **Configuration verbosity**: Many nested YAML sections, hard to understand
- **Backup limitations**: Primarily logical backups (slower to restore, no PITR)

**Estimated Setup Time**: 1-2 weeks (learning curve for Patroni + operator)

---

### Option 3: CloudNativePG (CNPG) Operator

**Architecture**:
```yaml
# CloudNativePG Cluster CRD (sketch)
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-cluster
spec:
  instances: 3
  storage:
    size: 20Gi
  backup:
    barmanObjectStore:
      destinationPath: s3://backups/allchat
      s3Credentials: ...
```

**✅ Pros**:
- **Team experience**: Production use at sipgate, familiar with troubleshooting
- **Automated failover**: Primary failure detected in ~10s, replica promoted in <30s total
- **Native PITR**: Continuous WAL archiving to S3, restore to any point in time
- **Simple CRD**: Declarative, easy to understand, minimal configuration
- **Built-in backups**: automated backups to S3-compatible object storage
- **Excellent monitoring**: Prometheus metrics out-of-box (replication lag, connections, queries)
- **No Patroni**: Self-contained operator, no external DCS (etcd/Consul) needed
- **Active development**: Graduated CNCF project, frequent releases, responsive maintainers

**❌ Cons**:
- **Younger project**: First stable release 2021 (vs Zalando 2016)
- **Smaller community**: Fewer Stack Overflow answers (but great official docs)
- **Operator dependency**: Must maintain operator version, upgrade carefully
- **Some abstractions**: Hides PostgreSQL internals (can make debugging harder)

**Estimated Setup Time**: 2-3 days (team already familiar)

---

## Decision Outcome

**Chosen**: **Option 3 - CloudNativePG Operator**

**Rationale**:

1. **Team Experience** (Primary Driver):
   - Production use at sipgate (parent organization)
   - Team knows how to troubleshoot failures, tune configuration
   - Faster incident response (no learning curve during outage)

2. **Automated Failover** (<30s RTO):
   - Primary failure detected automatically (health checks every 10s)
   - Replica promoted without manual intervention
   - Measured in early deploys: ~22 seconds average failover time

3. **PITR Capability in the Operator**:
   - CNPG supports continuous WAL archiving to S3-compatible storage and
     restore to any timestamp; since CNPG 1.26 this goes through the Barman
     Cloud plugin rather than the deprecated in-tree `barmanObjectStore`
   - Wired for this deployment: see Backups below

4. **Operational Simplicity**:
   - Single CRD (`Cluster`) for entire PostgreSQL setup
   - No separate backup scripts, cron jobs, or Patroni configuration
   - Upgrades handled by operator (rolling updates, no downtime)

5. **Cost-Effective**:
   - Open-source, no licensing fees
   - Self-hosted, far below the cost of a managed database

6. **Monitoring Out-of-Box**:
   - Prometheus metrics auto-exported via the CNPG PodMonitor
     (replication lag, transactions, connections)
   - Grafana dashboards available from community

---

## Consequences

### Positive

1. **Fast Failover**:
   - Primary failure detected automatically, replica promoted without manual intervention
   - Measured in early deploys: ~22 seconds from failure to service restored

2. **Data Durability** on failover (RPO = 0) while `synchronous_commit` stays at
   its default `on`, which the cluster manifest leaves untouched.

3. **Simple Operations**:
   - **Create cluster**: apply the CNPG `Cluster` resource (declarative)
   - **Scale up**: Edit `instances: 3` → `instances: 5` (rolling update)

4. **Built-in Monitoring**:
   - All metrics available in Prometheus immediately (PodMonitor enabled on the cluster)
   - No custom exporters to write/maintain
   - Grafana dashboard imported in 5 minutes

5. **Connection Budget** (ADR-0039):
   - Every Go service bounds its pool via `shared/database.NewPostgresPool*`
     (`DATABASE_MAX_CONNS`/`DATABASE_MIN_CONNS`, defaults 10/1), so
     `Σ(instances × MaxConns)` stays under the cluster's `max_connections`

### Negative

1. **Operator Dependency**:
   - Must keep the CNPG operator updated
   - Operator bugs can affect the entire database cluster
   - **Mitigation**: Pin operator version, test upgrades in staging first

2. **Abstraction Layer**:
   - PostgreSQL internals hidden behind the CRD; debugging takes operator knowledge
   - **Mitigation**: Team experience, know how to check operator logs

3. **Smaller Community**:
   - Fewer Stack Overflow answers than the Zalando operator
   - **Mitigation**: Excellent official docs

4. **Backups Depend on Two External Stores**:
   - WAL and base backups go to Hetzner Object Storage, and a nightly copy to
     Backblaze B2 (see Backups below). A wrong key or a missing bucket stops
     WAL archiving, and Postgres then keeps every unarchived segment until the
     20Gi volume fills
   - **Mitigation**: `AllChatPostgresWALArchivingFailing` pages on failing or
     stalled archiving long before the disk fills; `AllChatPostgresBackupStale`
     warns when the newest base backup is older than 30h

5. **Connection Ceiling**:
   - At `max_connections = 200` the per-service pools pinned Postgres at its
     ceiling and crashlooped newly-starting pods (SQLSTATE 53300); ADR-0039
     shrank the per-service pools and the manifest raises the cap to 300

---

## Implementation

### Files and Configuration

The CNPG `Cluster` resource is owned and applied by the deployment pipeline
(caesar-deployment), not from this repository. What lives in this repo:

**Kubernetes manifests** (this repo, dev/local):
- `deployments/k8s/base/postgres/deployment.yaml` - plain single-replica `postgres:16-alpine` Deployment + Service + PVC (local k3d dev; NOT the CNPG cluster)
- `deployments/k8s/base/postgres/configmap.yaml` - migration ConfigMap stub
- `deployments/docker-compose.yml` - dev compose, `postgres:16-alpine`

**Connection config** (this repo):
- `deployments/k8s/base/configmap.yaml` sets `DATABASE_HOST: allchat-cluster-rw.allchat.svc.cluster.local`
  and `DATABASE_USER: allchat_user`; each service reads `DATABASE_PASSWORD` from the
  `allchat-secrets` Secret. Services build their pools via `shared/database.NewPostgresPool*`
  (connection budget per ADR-0039).

**Secrets** (service-affecting names only):
- `allchat-cluster-secret` - the cluster's app-user secret (referenced by the Cluster spec)
- `allchat-secrets` - per-service `DATABASE_PASSWORD`, JWT_SECRET and friends

### Cluster Configuration

The CNPG `Cluster` manifest is owned by the deployment pipeline
(caesar-deployment, `allchat-cluster.yaml`); it is the deployed spec:
```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-cluster
  namespace: allchat
spec:
  instances: 3
  monitoring:
    enablePodMonitor: true              # Prometheus auto-discovery
  plugins:                              # WAL archiving + base backups (see Backups)
    - name: barman-cloud.cloudnative-pg.io
      isWALArchiver: true
      parameters:
        barmanObjectName: allchat-hetzner
  affinity:
    podAntiAffinityType: required       # one instance per node
    topologyKey: kubernetes.io/hostname
  storage:
    size: 20Gi
  postgresql:
    parameters:
      max_connections: "300"           # headroom for the ADR-0039 connection budget
      shared_buffers: "256MB"
      work_mem: "2MB"                  # low work_mem under the connection budget
  bootstrap:
    initdb:
      database: allchat
      owner: allchat_user
      secret:
        name: allchat-cluster-secret
      postInitSQL:
      - CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
      - CREATE EXTENSION IF NOT EXISTS "pg_trgm";
  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "2"
```
The manifest sets no `primaryUpdateStrategy` override (CNPG's default
applies); `synchronous_commit` stays at its default `on`.

### Backups

Owned by the deployment pipeline (caesar-deployment), like the Cluster:

- **Postgres**: the Barman Cloud plugin (`argocd/apps/plugin-barman-cloud.yaml`)
  archives WAL continuously and takes a base backup every night at 23:00 UTC
  (`ScheduledBackup` `allchat-cluster-daily`) to the `allchat-hetzner`
  `ObjectStore` on Hetzner Object Storage (`apps/workloads/all-chat/backup-objectstore.yaml`).
  The recovery window is 30 days (`retentionPolicy`), to within CNPG's 5-minute
  `archive_timeout`.
- **Second copy**: the `backup-offsite-mirror` CronJob copies the Barman
  archive to Backblaze B2 every night, so Hetzner does not hold the only copy.
- **Restore**: a new `Cluster` with `bootstrap.recovery` from that
  `ObjectStore` (`externalClusters[].plugin`, `serverName: allchat-cluster`),
  never the live cluster itself. The header of `backup-objectstore.yaml` has
  the details.

### PgBouncer Pooler (not deployed)

A Pooler is unnecessary: services connect to `allchat-cluster-rw`
directly (the manifest disables the `ro`/`r` services; there are no read
replicas to load-balance) and cap their own pools via the connection budget
(ADR-0039).

### Accessing Database

**From application pods**:
```go
// DATABASE_HOST is allchat-cluster-rw.allchat.svc.cluster.local (primary)
// DATABASE_USER is allchat_user; DATABASE_PASSWORD comes from the allchat-secrets Secret
// Pools are built via shared/database.NewPostgresPool (bounded, see ADR-0039)
```

**From kubectl (debugging)**:
```bash
# Access primary (verify the current primary with `kubectl get cluster allchat-cluster -n allchat`)
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat

# Get connection count
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"
```

---

## Related Decisions

- **ADR-0001**: [Standard Go Layout](./0001-standard-go-layout.md) - Service database access patterns
- **ADR-0039**: [Database Connection Budget](./0039-database-connection-budget.md) - bounded per-service pools against this cluster's max_connections
- **Architecture**: [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - Kubernetes deployment
- **ADR-0089**: [Enforce data retention with a sweeper CronJob](./0089-retention-sweeper-cronjob.md) - data retention enforcement against this database

---

## Validation

Failover was exercised directly against the operator during the early deployment
phase and measured at roughly 20-25 seconds per scenario (pod deletion, network
partition, WAL disk full). A restore from the Barman archive into a scratch
cluster has not been exercised yet; until it has, the backups are unproven.

---

## References

- **CloudNativePG Docs**: https://cloudnative-pg.io/documentation/
- **CNCF Project**: https://www.cncf.io/projects/cloudnative-pg/
- **GitHub**: https://github.com/cloudnative-pg/cloudnative-pg
- **Slack Community**: https://cloudnativepg.slack.com

---

## Summary

**Decision**: Use CloudNativePG operator for PostgreSQL high availability.

**Reason**: Team experience, automated failover (<30s), operational simplicity, cost-effectiveness.

**Trade-off**: Operator dependency, smaller community (but excellent docs); backups depend on Hetzner Object Storage and Backblaze B2.

**Status**: Deployed in production: 3-instance CNPG cluster with automated failover,
continuous WAL archiving and nightly base backups (30-day PITR window).

