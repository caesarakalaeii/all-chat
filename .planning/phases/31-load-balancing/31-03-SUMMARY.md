---
phase: 31-load-balancing
plan: "03"
subsystem: infra
tags: [kubernetes, hpa, autoscaling, prometheus, servicemonitor, discord-listener, kustomize]

requires:
  - phase: 30-outbound-relay
    provides: "discord-listener service with HTTP health + /metrics endpoint on port 8086"

provides:
  - "Kubernetes Deployment + ClusterIP Service for discord-listener on port 8086"
  - "HPA autoscaling/v2 targeting discord-listener with CPU 70% / memory 80%, maxReplicas=3"
  - "ServiceMonitor for Prometheus discovery of /metrics endpoint every 30s"
  - "kustomization.yaml updated with Phase 5 Discord Listener section and image override"

affects: [32-load-balancing, phase-32, discord-listener-ops]

tech-stack:
  added: []
  patterns:
    - "HPA with conservative single-pod-at-a-time scale-up for shard-owning listeners"
    - "Deployment+Service in one YAML file (multi-document) matching kick-listener pattern"
    - "ServiceMonitor with prometheus=kube-prometheus label for Prometheus operator discovery"

key-files:
  created:
    - deployments/k8s/base/discord-listener/deployment.yaml
    - deployments/k8s/base/discord-listener/hpa.yaml
    - deployments/k8s/base/discord-listener/servicemonitor.yaml
  modified:
    - deployments/k8s/base/kustomization.yaml

key-decisions:
  - "maxReplicas=3 (not 5 like kick-listener) — single-shard model; extra pods are fault-tolerance standby only"
  - "scaleUp stabilizationWindowSeconds=30 with type=Pods value=1 — one pod at a time prevents shard ownership races"
  - "DISCORD_BOT_TOKEN sourced from discord-listener-secrets (separate Secret) not allchat-secrets"

patterns-established:
  - "Discord listener HPA uses Pods-count policy (not Percent) for predictable single-replica scale steps"
  - "Phase section comments in kustomization.yaml: '# Phase N - Service Name (vX.Y)'"

requirements-completed:
  - LOAD-02

duration: 2min
completed: "2026-03-16"
---

# Phase 31 Plan 03: Discord Listener Kubernetes Manifests Summary

**Deployment, ClusterIP Service, HPA (CPU 70%/mem 80%, max 3 replicas), and ServiceMonitor for discord-listener registered in kustomization.yaml — all four resources dry-run clean**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-16T09:21:45Z
- **Completed:** 2026-03-16T09:22:59Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Created `deployment.yaml` with Deployment + ClusterIP Service (port 8086) matching kick-listener pattern; env wired from `allchat-config`, `allchat-secrets`, and new `discord-listener-secrets`
- Created `hpa.yaml` with autoscaling/v2, CPU 70% / memory 80%, minReplicas=1 maxReplicas=3, conservative scale-up (one pod per 30s)
- Created `servicemonitor.yaml` scraping /metrics on port `http` every 30s with `prometheus=kube-prometheus` label
- Updated `kustomization.yaml` with Phase 5 Discord Listener section (3 resources) and `allchat-discord-listener` image override

## Task Commits

1. **Task 1: Deployment + Service + HPA manifests** - `a0ebe4a` (feat)
2. **Task 2: ServiceMonitor + kustomization update** - `043b532` (feat)

## Files Created/Modified

- `deployments/k8s/base/discord-listener/deployment.yaml` - Deployment (8086/http) + ClusterIP Service; env from ConfigMap/Secrets including `discord-listener-secrets`
- `deployments/k8s/base/discord-listener/hpa.yaml` - HPA autoscaling/v2, CPU 70% / mem 80%, maxReplicas=3, single-pod scale-up behavior
- `deployments/k8s/base/discord-listener/servicemonitor.yaml` - Prometheus ServiceMonitor, 30s interval, scrapeTimeout 10s
- `deployments/k8s/base/kustomization.yaml` - Added Phase 5 section with 3 resource paths and image override entry

## Decisions Made

- **maxReplicas=3** — discord-listener holds a single WebSocket shard; extra replicas provide fault tolerance only (not horizontal throughput). Matches the single-shard model documented in STATE.md decisions.
- **scaleUp: type=Pods value=1 periodSeconds=30** — Prevents multiple pods racing to acquire the Redis shard ownership lock simultaneously. Conservative matches kick-listener's documented migration protocol concern.
- **discord-listener-secrets** as separate Secret — Bot token lifecycle differs from database/service credentials; isolating it follows principle of least privilege and makes rotation independent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**Operator must create `discord-listener-secrets` Kubernetes Secret** before deploying:
```bash
kubectl -n allchat create secret generic discord-listener-secrets \
  --from-literal=DISCORD_BOT_TOKEN=<your-bot-token>
```
This Secret is referenced in `deployment.yaml` but not created by these manifests (matches sealed-secrets pattern for sensitive credentials).

## Next Phase Readiness

- All four manifests validate cleanly with `kubectl apply --dry-run=client`
- kustomization.yaml intact for all existing services — no regressions
- Ready for Phase 32 (any final load-balancing wiring or e2e validation)

---
*Phase: 31-load-balancing*
*Completed: 2026-03-16*

## Self-Check: PASSED

- FOUND: deployments/k8s/base/discord-listener/deployment.yaml
- FOUND: deployments/k8s/base/discord-listener/hpa.yaml
- FOUND: deployments/k8s/base/discord-listener/servicemonitor.yaml
- FOUND: .planning/phases/31-load-balancing/31-03-SUMMARY.md
- FOUND: a0ebe4a (Task 1 commit)
- FOUND: 043b532 (Task 2 commit)
