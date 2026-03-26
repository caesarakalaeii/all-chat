---
phase: 02-support-bot-operational-awareness
plan: 03
subsystem: infra
tags: [kubectl, mcp-grafana, kubernetes, rbac, docker, discord-bot]

requires:
  - phase: 02-support-bot-operational-awareness
    provides: Support bot TypeScript source and base Dockerfile

provides:
  - kubectl v1.33.5 binary available in support-bot container at /usr/local/bin/kubectl
  - mcp-grafana v0.11.3 binary available in support-bot container at /usr/local/bin/mcp-grafana
  - LEAD_DEVELOPER_DISCORD_ID env var in Kubernetes deployment (198569499228766208)
  - GRAFANA_URL env var in Kubernetes deployment (https://grafana.caes.ar)
  - GRAFANA_SERVICE_ACCOUNT_TOKEN env var sourced from allchat-secrets
  - support-bot-cluster-reader RBAC Role with read-only access to pods, events, deployments, replicasets, pod metrics
  - support-bot-cluster-reader RoleBinding linking Role to support-bot ServiceAccount

affects:
  - 02-support-bot-operational-awareness (future plans that build on kubectl/grafana subprocess access)

tech-stack:
  added: [kubectl v1.33.5, mcp-grafana v0.11.3]
  patterns: [Binary installation as root before USER node in alpine Dockerfile, RBAC least-privilege read-only cluster reader pattern]

key-files:
  created: []
  modified:
    - services/support-bot/Dockerfile
    - ../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml
    - ../caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml

key-decisions:
  - "kubectl installed via apk curl + install to /usr/local/bin before USER node — binaries available in PATH for non-root node user at runtime"
  - "mcp-grafana installed via tar.gz extraction, binaries owned by root but world-executable (0755) — non-root user can execute"
  - "RBAC split into two roles: secret-patcher (existing, write to secrets) and cluster-reader (new, read-only to workload resources)"
  - "metrics.k8s.io/pods included in RBAC for pod CPU/memory visibility during health checks"

patterns-established:
  - "Binary installation pattern: root-install before USER node in alpine-based Dockerfiles"
  - "RBAC least-privilege: separate Role per concern (secret access vs cluster inspection)"

requirements-completed: [OPS-07, OPS-08, OPS-09]

duration: 1min
completed: 2026-03-26
---

# Phase 02 Plan 03: Infrastructure Binaries and RBAC Summary

**kubectl v1.33.5 and mcp-grafana v0.11.3 binaries installed in support-bot container, with RBAC cluster-reader Role granting read-only access to pods, events, deployments, replicasets, and pod metrics in the allchat namespace**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-03-26T08:55:02Z
- **Completed:** 2026-03-26T08:55:49Z
- **Tasks:** 3 of 3 complete (Task 3 checkpoint approved by user)
- **Files modified:** 3

## Accomplishments

- Dockerfile extended with kubectl v1.33.5 and mcp-grafana v0.11.3 binary installations (both as root before USER node)
- Deployment manifest extended with three new env vars: LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN
- RBAC extended with new support-bot-cluster-reader Role and RoleBinding for read-only cluster inspection

## Task Commits

Each task was committed atomically:

1. **Task 1: Install kubectl and mcp-grafana binaries in Dockerfile** - `3d21232` (feat)
2. **Task 2: Add env vars to deployment and extend RBAC for kubectl read access** - `880d744` (feat, caesar-deployment repo)

3. **Task 3: Human review checkpoint** - Approved by user; `support-bot-grafana-token` added to SOPS-encrypted `allchat-secret.enc.yaml`

## Files Created/Modified

- `/home/moersener/Hobby/all-chat/services/support-bot/Dockerfile` - Added kubectl v1.33.5 and mcp-grafana v0.11.3 binary installation steps before USER node
- `/home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` - Added LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN env vars
- `/home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml` - Added support-bot-cluster-reader Role (pods/events/deployments/replicasets/metrics) and RoleBinding

## Decisions Made

- kubectl installed via `apk add curl` + `install -o root -g root -m 0755` — matches standard Kubernetes binary installation pattern, binary accessible to non-root user via /usr/local/bin PATH
- mcp-grafana installed via tar.gz extraction with chmod 0755 — world-executable so the non-root `node` user can invoke it as subprocess
- RBAC cluster-reader kept separate from secret-patcher — least-privilege principle: two concerns, two roles
- `metrics.k8s.io/pods` included so Claude subprocess can inspect pod CPU/memory during health assessments

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Completed

The `support-bot-grafana-token` key was added to the SOPS-encrypted `allchat-secret.enc.yaml` by the user during the checkpoint review. No further manual steps required before deployment.

## Next Phase Readiness

- Container has kubectl and mcp-grafana binaries at /usr/local/bin — available to Claude subprocess via PATH
- Deployment has Grafana credentials and lead developer Discord ID as env vars
- RBAC grants read-only cluster inspection in allchat namespace
- Grafana token added to SOPS-encrypted allchat-secret.enc.yaml — ready for deployment

---
*Phase: 02-support-bot-operational-awareness*
*Completed: 2026-03-26*

## Self-Check: PASSED

- [x] `services/support-bot/Dockerfile` exists with kubectl and mcp-grafana
- [x] `support-bot-deployment.yaml` contains LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN
- [x] `support-bot-rbac.yaml` contains support-bot-cluster-reader Role and RoleBinding
- [x] Commit `3d21232` (Dockerfile) exists in all-chat repo
- [x] Commit `880d744` (deployment + RBAC) exists in caesar-deployment repo
