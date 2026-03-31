# Quick Task: Automate Database Migrations - Research

**Researched:** 2026-03-31
**Domain:** Kubernetes init containers, PostgreSQL migrations, CNPG
**Confidence:** HIGH

## Summary

Migrations are currently run manually via `make migrate` (local dev) or not at all in production -- there is no automated migration step in any K8s deployment. The only exception is `support-bot-deployment.yaml`, which has a hand-rolled init container running a single inline SQL migration for `bot_memories`. All other services (auth-service, overlay-manager, share-service, source-manager, token-refresh-service, etc.) have zero migration automation.

**Primary recommendation:** Create a dedicated Kubernetes Job that runs all migrations sequentially using `psql`, triggered before deployments. Alternatively, add a single init container to one "gateway" deployment (e.g., overlay-manager or auth-service) that runs all pending migrations before the app starts.

## Current State

### Migration Files
- **66 SQL files** in `migrations/` (001 through 044, with `_down` variants and some duplicate numbers like 004, 005, 007, 008, 009)
- **31 files** in `migrations/init/` (subset used by `docker-compose.frontend.yml` for fresh dev DBs)
- All migrations use **idempotent patterns**: `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `ON CONFLICT DO NOTHING`
- **No migration tool** (no golang-migrate, goose, atlas, or flyway). Pure raw SQL files.
- Naming is **not strictly sequential** (duplicate prefixes like `004_tiktok_support.sql` and `004_source_change_notifications.sql`)

### Local Dev
- `docker-compose.yml`: Mounts entire `migrations/` into `/docker-entrypoint-initdb.d:ro` -- PostgreSQL runs these on first init only
- `docker-compose.frontend.yml`: Mounts `migrations/init/` subset
- `make migrate`: Only runs `001_initial_schema.sql` -- incomplete, does NOT run all migrations

### Production (K8s)
- **CNPG Cluster** (`allchat-cluster.yaml`): 3 instances, `allchat_user` owner, `uuid-ossp` and `pg_trgm` extensions via `postInitSQL`
- **No migration automation** on any of the 14 service deployments (except support-bot's inline init container)
- **Keel** handles image rollouts (auto-deploys on push to `main`)
- Database credentials: `allchat-config` ConfigMap (host/port/name/user) + `allchat-secrets` (password)

### Services That Use Database
14 deployments reference `DATABASE_HOST`. Key DB-writing services: auth-service, overlay-manager, share-service, source-manager, token-refresh-service, support-bot.

## Recommended Approach: Kubernetes Job

### Why a Job (not init containers on every deployment)

| Approach | Pros | Cons |
|----------|------|------|
| **K8s Job (recommended)** | Runs once, no race conditions, clear success/fail, can be triggered by CI/CD or Keel webhook | Requires ordering before deployments |
| Init container on one service | Simple, runs before app starts | Which service? If that service doesn't deploy, migrations don't run. Multiple replicas = race condition |
| Init container on every service | Guarantees migration runs | 14 services all racing to run migrations simultaneously |

### Implementation Pattern

**1. Migration runner image:** Build a lightweight Docker image (or use `postgres:16-alpine`) that contains all migration SQL files and a runner script.

**2. Runner script pattern:**
```bash
#!/bin/sh
set -e
# Run all migration files in order
for f in /migrations/[0-9]*.sql; do
  # Skip _down files
  case "$f" in *_down.sql) continue ;; esac
  echo "Running: $f"
  PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" \
    -d "$DATABASE_NAME" \
    -v ON_ERROR_STOP=0 \
    -f "$f"
done
echo "All migrations complete"
```

**3. K8s Job manifest:**
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: allchat-migrations-YYYYMMDD-NNN
  namespace: allchat
spec:
  backoffLimit: 3
  ttlSecondsAfterFinished: 86400
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ghcr.io/caesarakalaeii/allchat-migrations:main
          envFrom:
            - configMapRef:
                name: allchat-config
          env:
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: allchat-secrets
                  key: database-password
      restartPolicy: Never
```

### Alternative: Init Container on a Single Service

If a dedicated Job feels like overkill, add an init container to **auth-service** (it starts early, is always deployed, and is a natural DB dependency root):

```yaml
initContainers:
  - name: run-migrations
    image: ghcr.io/caesarakalaeii/allchat-migrations:main
    envFrom:
      - configMapRef:
          name: allchat-config
    env:
      - name: DATABASE_PASSWORD
        valueFrom:
          secretKeyRef:
            name: allchat-secrets
            key: database-password
```

This approach is simpler but has the caveat that migrations only run when auth-service restarts/deploys.

## Common Pitfalls

### Pitfall 1: Race Conditions with Multiple Replicas
**What goes wrong:** auth-service has `replicas: 2`. Both pods start simultaneously, both init containers run migrations at the same time.
**How to avoid:** All existing migrations are idempotent (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`). This means concurrent execution is safe -- both will succeed, the second is a no-op. No advisory lock needed given current migration patterns.

### Pitfall 2: Non-Sequential File Ordering
**What goes wrong:** `migrations/` has duplicate number prefixes (004, 005, 007, 008, 009 each have two files). Shell glob ordering (`[0-9]*.sql`) sorts lexicographically, which could run files in unexpected order.
**How to avoid:** The existing files are all idempotent and the duplicates don't conflict (e.g., `004_tiktok_support.sql` and `004_source_change_notifications.sql` touch different tables). Lexicographic ordering works fine. For future migrations, enforce unique numeric prefixes.

### Pitfall 3: `ON_ERROR_STOP` Behavior
**What goes wrong:** Using `ON_ERROR_STOP=1` causes the entire migration to abort on first error (e.g., a `CREATE TYPE` that already exists without `IF NOT EXISTS`).
**How to avoid:** The support-bot precedent uses `ON_ERROR_STOP=0`. This is safe given the idempotent pattern. Use `ON_ERROR_STOP=0` for the runner script.

### Pitfall 4: Keel Auto-Deploy Timing
**What goes wrong:** Keel deploys new service images immediately when pushed. If new code requires a new migration but the migration hasn't run yet, the service starts with a missing table/column.
**How to avoid:** The migration image should be built and deployed (Job or init container) BEFORE or alongside service images. If using init container approach, it naturally runs before the app container. If using Job approach, CI/CD must ensure the Job completes before service deployments.

### Pitfall 5: Down Migrations Mixed In
**What goes wrong:** The glob `[0-9]*.sql` matches `*_down.sql` rollback files.
**How to avoid:** Filter out `_down.sql` files explicitly in the runner script.

## Integration Points

### CNPG Connection
- RW endpoint: `allchat-cluster-rw.allchat.svc.cluster.local:5432` (already in ConfigMap as `DATABASE_HOST`)
- Migrations MUST target the RW endpoint (already configured)
- User: `allchat_user` (the CNPG owner, has full DDL permissions)

### Keel Deployment Pipeline
- All services use `keel.sh/policy: force` + `keel.sh/trigger: poll`
- Migration image can use the same Keel annotations for automatic rollout
- Or: trigger migration Job via GitHub Actions before Keel picks up service images

### Existing Precedent (support-bot)
- The support-bot already demonstrates the init container + `postgres:16-alpine` + inline SQL pattern
- Phase 03 decision: `ON_ERROR_STOP=0` is intentional for idempotent migrations

## Recommendation

**Simplest effective approach:** Add an init container to **one** service deployment (auth-service recommended) that runs ALL migration files. This:

1. Requires no new Docker image (uses `postgres:16-alpine`)
2. Requires no CI/CD changes
3. Runs automatically on every deployment
4. Is safe with multiple replicas (idempotent SQL)
5. Follows the existing support-bot pattern

The init container mounts the migration files from a ConfigMap (or baked into a small image). Since there are 66 SQL files, inlining them is impractical -- a dedicated migration image containing the SQL files is the cleanest path. This image can be a simple `postgres:16-alpine` base with the migration directory COPYed in and a shell script entrypoint.

**Dockerfile for migration image:**
```dockerfile
FROM postgres:16-alpine
COPY migrations/ /migrations/
COPY scripts/run-migrations.sh /run-migrations.sh
RUN chmod +x /run-migrations.sh
ENTRYPOINT ["/run-migrations.sh"]
```

## Sources

### Primary (HIGH confidence)
- Direct inspection of `caesar-deployment/apps/workloads/all-chat/` K8s manifests
- Direct inspection of `all-chat/migrations/` SQL files
- Direct inspection of `support-bot-deployment.yaml` init container pattern (existing precedent)
- CNPG cluster config in `allchat-cluster.yaml`

## Metadata

**Confidence breakdown:**
- Current state analysis: HIGH - direct file inspection
- Recommended approach: HIGH - follows existing project pattern (support-bot)
- Pitfall analysis: HIGH - based on actual migration file contents

**Research date:** 2026-03-31
**Valid until:** 2026-04-30
