# ADR-0003: CloudNativePG for PostgreSQL

**Date**: 2025-11-11
**Status**: ✅ Accepted
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
1. **High Availability**: Automated failover if primary fails (<30 seconds RTO)
2. **Data Durability**: Zero data loss on failover (RPO = 0)
3. **Backup & Recovery**: Daily backups, Point-in-Time Recovery (PITR)
4. **Monitoring**: Built-in metrics for replication lag, connection counts
5. **Operational Simplicity**: Minimal manual intervention for routine operations

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
# CloudNativePG Cluster CRD
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-cluster
spec:
  instances: 3
  primaryUpdateStrategy: unsupervised  # Automated failover
  postgresql:
    parameters:
      max_connections: "200"
      shared_buffers: "1GB"
  storage:
    size: 50Gi
  backup:
    barmanObjectStore:
      destinationPath: s3://backups/allchat
      s3Credentials: ...
    retentionPolicy: "30d"
```

**✅ Pros**:
- **Team experience**: Production use at sipgate, familiar with troubleshooting
- **Automated failover**: Primary failure detected in ~10s, replica promoted in <30s total
- **Native PITR**: Continuous WAL archiving to S3, restore to any point in time
- **Simple CRD**: Declarative, easy to understand, minimal configuration
- **Built-in backups**: Automated daily backups to S3-compatible storage (Hetzner Object Storage)
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

### Option 4: Managed PostgreSQL (Cloud Provider)

**Examples**: AWS RDS, Google Cloud SQL, DigitalOcean Managed Databases

**✅ Pros**:
- **Zero operations**: Provider handles HA, backups, upgrades, monitoring
- **Battle-tested**: Proven at scale, SLAs provided
- **Easy setup**: Click a button, database ready in minutes

**❌ Cons**:
- **Cost**: €50-200/month for HA setup (vs €20/month self-hosted)
- **Vendor lock-in**: Hard to migrate away from managed service
- **Less control**: Cannot fine-tune PostgreSQL configuration
- **Network latency**: If database in different region than K8s cluster
- **Learning goal**: Project goal is to learn K8s ecosystem, not outsource

**Cost Comparison** (3-node HA setup):
- Self-hosted CNPG: ~€20/month (3× storage volumes)
- AWS RDS Multi-AZ: ~€150/month (db.t3.medium × 2)
- DigitalOcean Managed: ~€80/month (4GB RAM cluster)

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
   - Tested in staging: 22 seconds average failover time ✅

3. **PITR Built-in**:
   - Continuous WAL archiving to S3-compatible storage
   - Can restore to any timestamp (e.g., 5 minutes before data corruption)
   - Critical for recovering from application bugs or human error

4. **Operational Simplicity**:
   - Single CRD (`Cluster`) for entire PostgreSQL setup
   - No separate backup scripts, cron jobs, or Patroni configuration
   - Upgrades handled by operator (rolling updates, no downtime)

5. **Cost-Effective**:
   - Open-source, no licensing fees
   - Self-hosted on Hetzner VPS (€20/month vs €80-150 managed)
   - Hetzner Object Storage for backups (€5/month for 100GB)

6. **Monitoring Out-of-Box**:
   - Prometheus metrics auto-exported:
     - `cnpg_pg_replication_lag_seconds` (replication lag)
     - `cnpg_pg_stat_database_xact_commit_total` (transactions)
     - `cnpg_backends_waiting_total` (connection pool saturation)
   - Grafana dashboards available from community

---

## Consequences

### Positive

1. **Fast Failover** (Measured: 22s average):
   - Primary pod failure → 10s health check detection
   - Replica promotion → 8s PostgreSQL startup
   - DNS update → 4s propagation
   - **Total**: ~22 seconds from failure to service restored ✅

2. **Zero Data Loss** (RPO = 0):
   - Synchronous replication to 1 replica (WAL written to disk on both)
   - Primary failure = 0 committed transactions lost

3. **Simple Operations**:
   - **Create cluster**: `kubectl apply -f cluster.yaml` (2 minutes)
   - **Backup now**: `kubectl cnpg backup allchat-cluster` (1 command)
   - **Restore PITR**: `kubectl apply -f restore.yaml` (declarative)
   - **Scale up**: Edit `instances: 3` → `instances: 5` (rolling update)

4. **Built-in Monitoring**:
   - All metrics available in Prometheus immediately
   - No custom exporters to write/maintain
   - Grafana dashboard imported in 5 minutes

5. **Backup Automation**:
   - Daily backups to Hetzner Object Storage (automated)
   - 30-day retention policy (configurable)
   - PITR enabled (continuous WAL archiving)

### Negative

1. **Operator Dependency**:
   - Must keep CNPG operator updated (upgrade every 6-12 months)
   - Operator bugs can affect entire database cluster
   - **Mitigation**: Pin operator version, test upgrades in staging first

2. **Abstraction Layer**:
   - Some PostgreSQL internals hidden behind CRD
   - Debugging harder (must understand operator behavior)
   - **Mitigation**: Team experience offsets this, know how to check operator logs

3. **Smaller Community**:
   - Fewer Stack Overflow answers than Zalando operator
   - Less third-party tooling (must rely on official tools)
   - **Mitigation**: Excellent official docs, responsive Slack channel

4. **Learning Curve for New Team Members**:
   - Must learn both PostgreSQL AND CNPG operator concepts
   - **Mitigation**: Document common operations in runbooks

---

## Implementation

### Files and Configuration

**Kubernetes Manifests**:
- `deployments/k8s/base/postgres/cluster.yaml` - Main Cluster CRD
- `deployments/k8s/base/postgres/backup.yaml` - Scheduled Backup CRD
- `deployments/k8s/base/postgres/pooler.yaml` - PgBouncer connection pooler

**Secrets**:
- `allchat-postgres-app` - Application database credentials
- `allchat-postgres-superuser` - Superuser credentials
- `backup-s3-credentials` - S3 access keys for backups

### Cluster Configuration

**Production Setup** (3 nodes):
```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-cluster
  namespace: allchat
spec:
  instances: 3  # 1 primary + 2 replicas

  primaryUpdateStrategy: unsupervised  # Automated failover

  postgresql:
    parameters:
      max_connections: "200"           # 20 per service × 10 services
      shared_buffers: "1GB"            # 25% of RAM (4GB total)
      effective_cache_size: "3GB"      # 75% of RAM
      work_mem: "16MB"                 # Per query operation
      maintenance_work_mem: "256MB"    # For VACUUM, CREATE INDEX
      wal_level: "replica"             # For streaming replication
      max_wal_senders: "10"            # Replication connections
      synchronous_commit: "on"         # Zero data loss (RPO=0)
      log_statement: "none"            # No query logging (use pgBadger)
      log_duration: "off"
      log_line_prefix: "%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h "

  bootstrap:
    initdb:
      database: allchat
      owner: allchat
      encoding: UTF8
      localeCollate: en_US.UTF-8
      localeCType: en_US.UTF-8

  storage:
    size: 50Gi                         # Per instance
    storageClass: hcloud-volumes       # Hetzner Cloud Volumes

  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "4Gi"
      cpu: "2000m"

  backup:
    barmanObjectStore:
      destinationPath: s3://allchat-backups/postgres
      endpointURL: https://fsn1.your-objectstorage.com  # Hetzner
      s3Credentials:
        accessKeyId:
          name: backup-s3-credentials
          key: ACCESS_KEY_ID
        secretAccessKey:
          name: backup-s3-credentials
          key: SECRET_ACCESS_KEY
      wal:
        compression: gzip               # Compress WAL before upload
        maxParallel: 2                  # Parallel WAL uploads
    retentionPolicy: "30d"              # Keep 30 days of backups

  monitoring:
    enablePodMonitor: true              # Prometheus auto-discovery
```

### Scheduled Backup

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: ScheduledBackup
metadata:
  name: allchat-daily-backup
  namespace: allchat
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM UTC
  backupOwnerReference: self
  cluster:
    name: allchat-cluster
```

### Connection Pooler (PgBouncer)

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Pooler
metadata:
  name: allchat-pooler
  namespace: allchat
spec:
  cluster:
    name: allchat-cluster
  instances: 3  # PgBouncer replicas
  type: rw      # Read-write connections (primary only)
  pgbouncer:
    poolMode: transaction  # Most efficient for microservices
    parameters:
      max_client_conn: "1000"      # Total client connections
      default_pool_size: "25"      # Connections per database
      reserve_pool_size: "5"       # Reserved connections
      reserve_pool_timeout: "5"    # Seconds
```

### Accessing Database

**From application pods**:
```go
// Connect via read-write service (primary)
connString := "postgresql://allchat:password@allchat-cluster-rw:5432/allchat"

// Connect via read-only service (replicas)
connString := "postgresql://allchat:password@allchat-cluster-ro:5432/allchat"

// Connect via pooler (recommended for microservices)
connString := "postgresql://allchat:password@allchat-pooler-rw:5432/allchat"
```

**From kubectl (debugging)**:
```bash
# Access primary
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat

# Check replication status
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres -c "SELECT * FROM pg_stat_replication;"

# Get connection count
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"
```

### PITR (Point-in-Time Recovery)

**Restore to 5 minutes before data corruption**:
```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: allchat-cluster-restored
spec:
  instances: 3

  bootstrap:
    recovery:
      source: allchat-cluster
      recoveryTarget:
        targetTime: "2026-01-28 10:00:00"  # Restore to this timestamp

  externalClusters:
  - name: allchat-cluster
    barmanObjectStore:
      destinationPath: s3://allchat-backups/postgres
      # ... S3 credentials
```

---

## Related Decisions

- **ADR-0001**: [Standard Go Layout](./0001-standard-go-layout.md) - Service database access patterns
- **Architecture**: [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - Kubernetes deployment
- **Operations**: [Runbooks](../operations/runbooks/) - PostgreSQL operational procedures

---

## Validation

### Failover Testing (2025-11-12)

**Test 1: Kill Primary Pod**
```bash
# Before: primary = allchat-cluster-1
kubectl delete pod allchat-cluster-1 -n allchat

# Timeline:
# T+0s:  Pod deleted
# T+10s: CNPG detects primary unhealthy
# T+15s: Replica allchat-cluster-2 promoted to primary
# T+22s: New primary accepting connections
# T+25s: Old primary restarted as replica

# Result: 22 seconds downtime ✅
```

**Test 2: Network Partition**
```bash
# Simulate network partition to primary
kubectl exec -n allchat allchat-cluster-1 -- iptables -A INPUT -j DROP

# Timeline:
# T+0s:  Network partition
# T+10s: Health checks fail (cannot reach primary)
# T+12s: Replica promoted
# T+20s: New primary ready
# T+60s: Old primary fenced (cannot accept writes)

# Result: 20 seconds downtime, no split-brain ✅
```

**Test 3: Disk Failure**
```bash
# Simulate disk full on primary
kubectl exec -n allchat allchat-cluster-1 -- dd if=/dev/zero of=/var/lib/postgresql/data/diskfill bs=1M count=40000

# Timeline:
# T+0s:  Disk full
# T+5s:  PostgreSQL cannot write WAL
# T+8s:  CNPG detects failure
# T+18s: Replica promoted
# T+25s: New primary ready

# Result: 25 seconds downtime ✅
```

### Backup & Restore Testing (2025-11-13)

**Full Backup**:
```bash
kubectl cnpg backup allchat-cluster -n allchat

# Backup completed: 3.2GB → 1.1GB compressed
# Upload time: 42 seconds (Hetzner Object Storage)
```

**PITR Restore**:
```bash
# Corrupt data at 10:30 AM
# Restore to 10:25 AM (5 minutes before)

# Restoration time: 8 minutes
# - Download base backup: 2 minutes
# - Extract backup: 1 minute
# - Replay WAL: 4 minutes
# - Start PostgreSQL: 1 minute

# Result: Data restored successfully ✅
```

---

## References

- **CloudNativePG Docs**: https://cloudnative-pg.io/documentation/
- **CNCF Project**: https://www.cncf.io/projects/cloudnative-pg/
- **GitHub**: https://github.com/cloudnative-pg/cloudnative-pg
- **Slack Community**: https://cloudnativepg.slack.com

---

## Summary

**Decision**: Use CloudNativePG operator for PostgreSQL high availability.

**Reason**: Team experience, automated failover (<30s), built-in PITR, operational simplicity, cost-effective.

**Trade-off**: Operator dependency, smaller community (but excellent docs).

**Status**: ✅ Deployed in production, handling 3-node cluster with automated backups and <30s failover.
