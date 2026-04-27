---
phase: 14-secret-rotation-infrastructure
plan: "07"
subsystem: infra
tags:
  - kubernetes
  - secret-rotation
  - deployment-manifests
  - key-chain
  - encryption

dependency_graph:
  requires:
    - 14-04   # encryption callsite migration (TOKEN_ENCRYPTION_KEY_V1 consumers)
    - 14-05   # JWT middleware + kick encryption (SERVICE_JWT_SECRET_V1 consumers)
    - 14-06   # key-rotator binary in auth-service Docker image
  provides:
    - caesar-deployment deployment YAMLs with V1 env entries wired
    - key-rotator-job.yaml (manual-trigger template)
    - key-rotator-cronjob.yaml (weekly Sunday 03:00 UTC)
  affects:
    - 14-08   # rotation runbook: must kubectl patch allchat-secrets before ArgoCD sync

tech-stack:
  added: []
  patterns:
    - "secretKeyRef per-env-var (no envFrom): V1 keys added as N+1 blocks alongside legacy keys"
    - "Job template pattern: key-rotator-job.yaml not in kustomization — operator creates from CronJob"
    - "CronJob registered in kustomization; Job is a documentation-only template"

key-files:
  created:
    - caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml
    - caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml
  modified:
    - caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/kick-listener-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/discord-listener-deployment.yaml
    - caesar-deployment/apps/workloads/all-chat/kustomization.yaml

key-decisions:
  - "api-gateway and share-service already had SERVICE_JWT_SECRET + SERVICE_JWT_SECRET_V1 (wired in a prior session during 14-05 execution); source-manager already had SERVICE_JWT_SECRET_V1 — these three required no changes"
  - "auth-service already had JWT_SECRET_V1 but was missing TOKEN_ENCRYPTION_KEY_V1 — added"
  - "overlay-manager already had JWT_SECRET_V1 but was missing TOKEN_ENCRYPTION_KEY_V1 — added"
  - "key-rotator-job.yaml not registered in kustomization (template only); CronJob is registered"
  - "youtube-listener (non-innertube) is inactive in production — no manifest exists; documented as dead-code-safe"
  - "kubectl context 'default' not present in this environment; youtube preflight ran against k3s-ansible (unavailable) — status inferred from absence of yaml in caesar-deployment"
  - "Dockerfile already builds /app/key-rotator (Plan 14-06 added it)"

requirements-completed: []

metrics:
  duration_seconds: 420
  completed_date: "2026-04-27"
  task_count: 3
  file_count: 12
---

# Phase 14 Plan 07: Deployment Manifests and Sweeper Job Summary

**K8s deployment YAMLs for all 12 all-chat services updated with TOKEN_ENCRYPTION_KEY_V1/JWT_SECRET_V1/SERVICE_JWT_SECRET_V1 env entries; Pitfall 1 renamed ENCRYPTION_KEY to TOKEN_ENCRYPTION_KEY in token-refresh-service and twitch-eventsub-listener; key-rotator Job + CronJob manifests created and CronJob registered in kustomization.yaml.**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-04-27T16:40:00Z
- **Completed:** 2026-04-27T16:46:27Z
- **Tasks:** 3
- **Files modified:** 12 (9 deployment YAMLs + 2 new manifests + kustomization.yaml)

## Accomplishments

- All 12 deployment YAMLs verified and updated with appropriate V1 env entries — services can now receive versioned secret keys on next rolling restart
- Pitfall 1 reconciled: `ENCRYPTION_KEY` → `TOKEN_ENCRYPTION_KEY` in both token-refresh-service and twitch-eventsub-listener (secretKeyRef.key unchanged: `token-encryption-key`)
- key-rotator CronJob manifest created (schedule: `0 3 * * 0`, Sunday 03:00 UTC) and registered in kustomization.yaml; Job manifest created as operator-use-only template (not applied by kustomize)

## youtube-listener (non-innertube) status: inactive

Preflight kubectl output:

```
$ kubectl --context default get deploy -n allchat | grep youtube || echo "NO_DEPLOY_MATCH"
error: context "default" does not exist
NO_DEPLOY_MATCH

$ kubectl --context default get statefulset -n allchat | grep youtube || echo "NO_STATEFULSET_MATCH"
error: context "default" does not exist
NO_STATEFULSET_MATCH
```

Note: The `default` context does not exist in this environment (sipgate contexts are named e.g. `k3s-ansible`, `tooling01-live-ml01`). Additional verification via manifest audit: `caesar-deployment/apps/workloads/all-chat/` contains only `youtube-listener-innertube-deployment.yaml` — no `youtube-listener-deployment.yaml` exists, and kustomization.yaml does not list one. Conclusion: **youtube-listener (non-innertube) is inactive in production.**

The encryption-callsite migration in Plan 14-04 (which touches `services/youtube-listener/cmd/main.go` so the binary requires `TOKEN_ENCRYPTION_KEY_V1` if ever re-deployed) is therefore dead-code-safe — the binary update compiled and ships in the Docker image but will not be reached at runtime until/unless the deployment is reactivated. Re-activation MUST go through Phase 14 manifest discipline first (add a `youtube-listener-deployment.yaml` with all six standard env entries).

## Per-Service Env Var Delta

| Service | Already present before 14-07 | Added in 14-07 | Renamed |
|---------|-------------------------------|----------------|---------|
| auth-service | JWT_SECRET_V1 | TOKEN_ENCRYPTION_KEY_V1 | — |
| api-gateway | JWT_SECRET_V1, SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — (complete) | — |
| overlay-manager | JWT_SECRET_V1 | TOKEN_ENCRYPTION_KEY_V1 | — |
| share-service | JWT_SECRET_V1, SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — (complete) | — |
| source-manager | SERVICE_JWT_SECRET_V1 | — (complete) | — |
| token-refresh-service | — | TOKEN_ENCRYPTION_KEY_V1 | ENCRYPTION_KEY → TOKEN_ENCRYPTION_KEY |
| twitch-eventsub-listener | — | TOKEN_ENCRYPTION_KEY_V1, SERVICE_JWT_SECRET_V1 | ENCRYPTION_KEY → TOKEN_ENCRYPTION_KEY |
| twitch-listener | — | SERVICE_JWT_SECRET_V1 | — |
| kick-listener | — | SERVICE_JWT_SECRET_V1, TOKEN_ENCRYPTION_KEY (NEW), TOKEN_ENCRYPTION_KEY_V1 | — |
| tiktok-listener | — | SERVICE_JWT_SECRET_V1 | — |
| youtube-listener-innertube | — | SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — |
| discord-listener | — | SERVICE_JWT_SECRET, SERVICE_JWT_SECRET_V1 | — |

tiktok-listener intentionally does NOT get TOKEN_ENCRYPTION_KEY — Node.js scope deferral (D-17, Plan 14-03/14-05 confirmed).

## Dockerfile Multi-Binary Status

`services/auth-service/Dockerfile` already builds `/app/key-rotator` (added in Plan 14-06):

```
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/key-rotator ./cmd/key-rotator
COPY --from=builder /app/key-rotator /app/key-rotator
```

No Dockerfile changes required in this plan.

## Key-Rotator Job/CronJob Shape

- **Job**: `key-rotator-job.yaml` — manual template, NOT in kustomization. Operator creates a run via `kubectl create job --from=cronjob/key-rotator-weekly key-rotator-manual-$(date +%s) -n allchat` per Plan 14-08 runbook. Uses `restartPolicy: OnFailure`, `backoffLimit: 2`, `activeDeadlineSeconds: 3600`, `ttlSecondsAfterFinished: 86400`.
- **CronJob**: `key-rotator-cronjob.yaml` — registered in kustomization.yaml. `schedule: "0 3 * * 0"` (Sunday 03:00 UTC), `concurrencyPolicy: Forbid`, `ttlSecondsAfterFinished: 604800` (7 days for log retention).
- **Env contract** (both Job and CronJob): DATABASE_HOST/PORT/NAME/USER/PASSWORD via configMapKeyRef + secretKeyRef → DATABASE_URL composed via K8s variable substitution; TOKEN_ENCRYPTION_KEY_V1 (key: token-encryption-key-v1); TOKEN_ENCRYPTION_KEY (key: token-encryption-key); YOUTUBE_TOKEN_ENCRYPTION_KEY (key: youtube-token-encryption-key) for D-04 legacy decryption.

## Note for Plan 14-08

**Before applying these manifests via ArgoCD:** The runbook MUST `kubectl patch allchat-secrets` to add the new secret keys (`token-encryption-key-v1`, `jwt-secret-v1`, `service-jwt-secret-v1`) — pods will fail `ImagePullBackOff`/`CreateContainerConfigError` otherwise because the secretKeyRef mounts will fail. The patch must run before the ArgoCD sync.

The legacy keys (`token-encryption-key`, `jwt-secret`, `service-jwt-secret`) already exist in `allchat-secrets` (they are in the live secret and verified by existing deployments running). Only the V1 variants need to be added.

## Task Commits (in caesar-deployment repo)

1. **Tasks 1+2: V1 env entries + Pitfall 1 fix** — `bd48cd8` (feat)
2. **Task 3: key-rotator Job + CronJob + kustomization** — `cb7d995` (feat)

## Deviations from Plan

### Pre-existing state (not deviations — informational)

Some deployments already had V1 entries applied during a prior session (api-gateway, share-service, source-manager were already complete). The plan's acceptance criteria checked for presence, so these passed without additional edits. auth-service had JWT_SECRET_V1 but was missing TOKEN_ENCRYPTION_KEY_V1 (added). overlay-manager had JWT_SECRET_V1 but was missing TOKEN_ENCRYPTION_KEY_V1 (added).

No Rule 1/2/3 auto-fixes required. No architectural changes (Rule 4). Plan executed exactly as written for all items needing changes.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or trust-boundary surface introduced. Threat T-14-07-04 (DATABASE_URL with embedded password in logs) is mitigated: all database credentials use `valueFrom.secretKeyRef` or `configMapKeyRef`; the DATABASE_URL env var is composed via K8s variable substitution (`$(DATABASE_USER)` etc.) so the actual password is never present as a literal value in the manifest.

## Self-Check

Files verified to exist:
- `caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml` — FOUND
- `caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml` — FOUND

Commits verified:
- `bd48cd8` in caesar-deployment — FOUND
- `cb7d995` in caesar-deployment — FOUND

Acceptance criteria verified:
- `grep -q "name: TOKEN_ENCRYPTION_KEY_V1" auth-service-deployment.yaml` — PASS
- `grep -q "name: TOKEN_ENCRYPTION_KEY_V1" overlay-manager-deployment.yaml` — PASS
- `! grep -q "name: ENCRYPTION_KEY$" token-refresh-service-deployment.yaml` — PASS (Pitfall 1 fixed)
- `! grep -q "name: ENCRYPTION_KEY$" twitch-eventsub-listener-deployment.yaml` — PASS (Pitfall 1 fixed)
- `grep -q "name: TOKEN_ENCRYPTION_KEY$" token-refresh-service-deployment.yaml` — PASS
- `grep -q "name: TOKEN_ENCRYPTION_KEY$" twitch-eventsub-listener-deployment.yaml` — PASS
- `grep -q "name: SERVICE_JWT_SECRET_V1" twitch-listener-deployment.yaml` — PASS
- `grep -q "name: TOKEN_ENCRYPTION_KEY_V1" kick-listener-deployment.yaml` — PASS
- `grep -q "name: SERVICE_JWT_SECRET_V1" tiktok-listener-deployment.yaml` — PASS
- `! grep -q "name: TOKEN_ENCRYPTION_KEY" tiktok-listener-deployment.yaml` — PASS (Node.js scope)
- `grep -q "name: SERVICE_JWT_SECRET_V1" youtube-listener-innertube-deployment.yaml` — PASS
- `grep -q "name: SERVICE_JWT_SECRET_V1" discord-listener-deployment.yaml` — PASS
- `grep -q "kind: Job" key-rotator-job.yaml` — PASS
- `grep -q "kind: CronJob" key-rotator-cronjob.yaml` — PASS
- `grep -q 'schedule: "0 3 * * 0"' key-rotator-cronjob.yaml` — PASS
- `grep -q "key-rotator-cronjob.yaml" kustomization.yaml` — PASS
- `! grep -q "key-rotator-job.yaml" kustomization.yaml` — PASS

## Self-Check: PASSED
