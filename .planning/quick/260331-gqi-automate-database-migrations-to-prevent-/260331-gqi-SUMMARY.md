---
phase: quick
plan: 260331-gqi
subsystem: infrastructure/migrations
tags: [migrations, kubernetes, ci, docker, init-containers]
dependency_graph:
  requires: []
  provides: [automated-db-migrations-on-deploy]
  affects: [auth-service, overlay-manager, share-service, source-manager, token-refresh-service, source-controller, message-processor]
tech_stack:
  added: [postgres:16-alpine migration image]
  patterns: [kubernetes-init-containers, ci-matrix-build]
key_files:
  created:
    - migrations/Dockerfile
    - scripts/run-migrations.sh
  modified:
    - .github/workflows/build-and-push.yml
    - ../caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/source-controller-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/message-processor-deployment.yaml
decisions:
  - "postgres:16-alpine base image used for migration container — psql available without extra tooling"
  - "ON_ERROR_STOP=0 with explicit exit 0 — idempotent SQL files (IF NOT EXISTS, ON CONFLICT DO NOTHING) may return psql non-zero; init container must always succeed on re-run"
  - "_down.sql files skipped via case pattern match in sh — alpine uses sh not bash, case/esac is POSIX"
  - "envFrom configMapRef + individual DATABASE_PASSWORD env — matches support-bot precedent, all DB vars in ConfigMap except password in Secret"
  - "Keel auto-updates init container image — existing keel.sh/policy: force + keel.sh/trigger: poll annotations cover init containers automatically"
metrics:
  duration: "~10 minutes"
  completed: "2026-03-31"
  tasks_completed: 2
  files_changed: 9
---

# Quick Task 260331-gqi: Automate Database Migrations Summary

**One-liner:** Migration Docker image (postgres:16-alpine + 66 SQL files) auto-built by CI and run as Kubernetes init containers on all 7 DB-writing service deployments before app start.

## What Was Built

### Task 1: Migration Docker image and runner script

- `migrations/Dockerfile` — builds from `postgres:16-alpine`, copies all 66 `migrations/*.sql` files to `/migrations/` and `scripts/run-migrations.sh` to `/run-migrations.sh`
- `scripts/run-migrations.sh` — POSIX `sh` script that iterates `ls /migrations/[0-9]*.sql | sort`, skips `*_down.sql` via case/esac, runs each via `psql -v ON_ERROR_STOP=0`, exits 0 unconditionally

Image builds successfully from repo root: `docker build -f migrations/Dockerfile -t allchat-migrations:test .`

### Task 2: CI pipeline and Kubernetes init containers

**CI (`.github/workflows/build-and-push.yml`):**
- Added `migrations/**` and `scripts/run-migrations.sh` to `on.push.paths` triggers
- Added `migrations` to `dorny/paths-filter` filters and `detect-changes` outputs
- Added `migrations` entry to build matrix with `path: migrations` — produces `ghcr.io/caesarakalaeii/allchat-migrations:main`

**Kubernetes (caesar-deployment repo, `feature/auto-migrations` branch):**

Added `initContainers` block before `containers:` in all 7 DB-writing service deployments:
- `auth-service-deployment.yaml`
- `overlay-manager-deployment.yaml`
- `share-service-deployment.yaml`
- `source-manager-deployment.yaml`
- `token-refresh-service-deployment.yaml`
- `source-controller-deployment.yaml`
- `message-processor-deployment.yaml`

Each init container:
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

**Not modified:** `support-bot-deployment.yaml` (already has its own inline migration init container), `api-gateway`, `emote-service`, all listener deployments (no DB writes).

## Commits

| Repo | Hash | Description |
|------|------|-------------|
| all-chat | 89813c37 | feat: add migration Docker image and runner script |
| all-chat | d89b0881 | feat: add migrations image to CI build pipeline |
| caesar-deployment | bbbf6ef | feat: add migration init containers to 7 deployments |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all components are fully wired.

## Verification Results

1. `docker build -f migrations/Dockerfile -t allchat-migrations:test .` — succeeds, image contains 66 SQL files
2. `grep -c "run-migrations" auth-service-deployment.yaml` — returns 1
3. CI workflow has `migrations` in detect-changes outputs AND build matrix — confirmed (9 occurrences)
4. `support-bot-deployment.yaml` has no `allchat-migrations` reference — confirmed
5. Runner script skips `_down.sql` and uses `ON_ERROR_STOP=0` — confirmed

## Self-Check: PASSED

- `migrations/Dockerfile` — exists
- `scripts/run-migrations.sh` — exists
- `.github/workflows/build-and-push.yml` — modified with migrations entries
- All 7 K8s deployment files in caesar-deployment have init container — confirmed (7 files match)
- Commits 89813c37, d89b0881 exist in all-chat; bbbf6ef exists in caesar-deployment
