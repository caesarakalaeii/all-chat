---
phase: 12-production-rollout
plan: 02
subsystem: deployment-automation
tags: [argo-rollouts, canary-deployment, metrics-analysis, kubernetes, kustomize]
dependencies:
  requires: [12-01-metrics-instrumentation]
  provides: [argo-rollout-manifests, metrics-based-promotion]
  affects: [youtube-listener-innertube]
tech-stack:
  added: [Argo Rollouts, AnalysisTemplate, ServiceMonitor]
  patterns: [canary-deployment, metrics-based-rollback, kustomize-overlays]
key-files:
  created:
    - deployments/k8s/youtube-listener-innertube/base/rollout.yaml
    - deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml
    - deployments/k8s/youtube-listener-innertube/base/service.yaml
    - deployments/k8s/youtube-listener-innertube/base/kustomization.yaml
    - deployments/k8s/youtube-listener-innertube/production/kustomization.yaml
    - deployments/k8s/youtube-listener-innertube/production/rollout-patch.yaml
    - deployments/k8s/monitoring/servicemonitor-innertube.yaml
  modified: []
decisions:
  - Indefinite pause between canary steps (metrics-based promotion, not time-based per research Pitfall 2)
  - 240-minute analysis window (4 hours continuous validation per user requirement)
  - Native Kubernetes traffic routing (no service mesh dependency)
  - Kustomize-based deployment (base + production overlay pattern)
metrics:
  tasks_completed: 3
  duration_minutes: 4
  completed_date: 2026-03-05
---

# Phase 12 Plan 02: Argo Rollouts Canary Deployment Summary

**One-liner**: Kubernetes manifests for automated canary deployment with 10%→50%→100% traffic shifting, PromQL-based promotion gates (<1% error), and automatic rollback triggers (>5% error).

## What Was Built

Created complete Argo Rollouts deployment structure for youtube-listener-innertube with:

1. **Argo Rollout CRD** - Replaces standard Deployment with canary strategy
2. **AnalysisTemplate** - 4 Prometheus metrics with automated promotion/rollback logic
3. **Kubernetes Services** - Stable + canary Services for traffic splitting
4. **Kustomize Structure** - Base manifests + production overlay
5. **ServiceMonitor** - Prometheus scrape configuration

## Canary Deployment Strategy

### Traffic Progression

```
10% traffic (1 pod) → Analysis 240min → 50% traffic (5 pods) → Analysis 240min → 100% (full promotion)
```

**Key Features**:
- Indefinite pause between steps (metrics control promotion, not clock)
- 4-hour validation window at each stage (240 x 1-minute samples)
- Automatic rollback on threshold breaches

### AnalysisTemplate Metrics

| Metric | Promotion Gate | Rollback Trigger | Failure Limit |
|--------|----------------|------------------|---------------|
| Error Rate | <1% | >5% | 3 consecutive |
| Message Rate Deviation | <5% vs baseline | >20% vs baseline | 5 consecutive |
| Redis Publish Success | >99% | <95% | 2 consecutive |
| Pod Restart Rate | 0 restarts/min | >2 restarts/min | 1 (immediate) |

**PromQL Queries**: All metrics query Prometheus at `http://prometheus.monitoring.svc.cluster.local:9090` with 5-minute rate windows.

## Architecture Decisions

### 1. Metrics-Based Promotion (Not Time-Based)

**Decision**: Use indefinite `pause: {}` between canary steps, let AnalysisTemplate control promotion.

**Rationale**: Research Pitfall 2 - arbitrary 4-hour clock can promote broken deployments if failure happens after timer expires. Analysis `interval: 1m` × `count: 240` = effective 4-hour soak time with continuous validation.

### 2. Native Kubernetes Traffic Routing

**Decision**: Use Argo Rollouts' built-in Service manipulation instead of service mesh (Istio/Linkerd).

**Implementation**:
- `canaryService: youtube-listener-innertube-canary`
- `stableService: youtube-listener-innertube-stable`
- Rollouts controller updates Service selectors to route traffic by ReplicaSet labels

**Benefit**: No additional infrastructure dependencies, simpler debugging.

### 3. Graceful Shutdown Safeguards

**Configuration**:
- `terminationGracePeriodSeconds: 30` (matches service main.go 25s timeout + 5s buffer)
- `preStop` hook with `sleep 5` (staggers pod terminations during rollback)

**Prevents**: Thundering herd problem during rapid rollback (all pods terminating simultaneously).

### 4. Kustomize Base + Overlay Pattern

**Structure**:
```
base/
  - rollout.yaml          # Core canary configuration
  - service.yaml          # Stable + canary Services
  - analysis-template.yaml # Prometheus metrics
production/
  - kustomization.yaml    # References base, patches replicas + image tag
  - rollout-patch.yaml    # Override: replicas: 10
```

**Benefits**:
- Environment-specific configuration (dev/staging/prod replicas)
- CI/CD updates image tag via `kustomize edit set image`
- Single source of truth in base/, minimal production overrides

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Kustomize patch targeting failure**
- **Found during**: Task 3 verification
- **Issue**: `kustomize build` failed with "no matches for Id Rollout" - patch missing namespace selector
- **Fix**: Added `namespace: allchat` to rollout-patch.yaml metadata
- **Files modified**: `deployments/k8s/youtube-listener-innertube/production/rollout-patch.yaml`
- **Commit**: dc7277b

**2. [Rule 3 - Blocking] Deprecated Kustomize syntax**
- **Found during**: Task 3 verification
- **Issue**: Warning "patchesStrategicMerge is deprecated" and patch not applying correctly
- **Fix**: Replaced `patchesStrategicMerge` with `patches` syntax including explicit target kind/name
- **Files modified**: `deployments/k8s/youtube-listener-innertube/production/kustomization.yaml`
- **Commit**: dc7277b

## Deployment Instructions

### Prerequisites

1. Argo Rollouts controller installed:
   ```bash
   kubectl create namespace argo-rollouts
   kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
   ```

2. Prometheus Operator with ServiceMonitor CRD:
   ```bash
   kubectl get crd servicemonitors.monitoring.coreos.com
   ```

3. Official youtube-listener running for baseline comparison:
   ```bash
   kubectl get deploy youtube-listener -n allchat
   ```

### Apply Manifests

```bash
# Build and preview (dry-run)
kubectl kustomize deployments/k8s/youtube-listener-innertube/production/

# Apply to cluster
kubectl apply -k deployments/k8s/youtube-listener-innertube/production/

# Verify resources created
kubectl get rollout,svc,analysistemplate,servicemonitor -n allchat | grep innertube
```

### Monitor Rollout Progress

```bash
# Watch rollout status (requires Argo Rollouts kubectl plugin)
kubectl argo rollouts get rollout youtube-listener-innertube -n allchat --watch

# Check analysis run status
kubectl get analysisrun -n allchat

# View Prometheus metrics (forward port if needed)
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Open: http://localhost:9090
```

### Manual Controls

```bash
# Promote to next step (skip analysis wait)
kubectl argo rollouts promote youtube-listener-innertube -n allchat

# Abort rollout (immediate rollback to stable)
kubectl argo rollouts abort youtube-listener-innertube -n allchat

# Restart rollout (useful after fixing issues)
kubectl argo rollouts restart youtube-listener-innertube -n allchat
```

## Verification Commands

```bash
# 1. Rollout exists and is healthy
kubectl get rollout youtube-listener-innertube -n allchat

# 2. Services route to correct pods
kubectl get svc youtube-listener-innertube-stable youtube-listener-innertube-canary -n allchat -o wide

# 3. Analysis template ready
kubectl get analysistemplate innertube-metrics-analysis -n allchat

# 4. ServiceMonitor discovered by Prometheus
kubectl get servicemonitor youtube-listener-innertube -n allchat

# 5. Prometheus scraping metrics
curl http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=youtube_listener_messages_published_total
```

## Success Criteria

- [x] Argo Rollout manifest replaces standard Deployment with canary strategy
- [x] Canary steps progress from 10% → 50% → 100% with indefinite pause between stages
- [x] AnalysisTemplate defines 4 metrics: error rate, message rate deviation, Redis publish success, pod restarts
- [x] Promotion gates require <1% error rate and <5% message deviation for 240 consecutive minutes
- [x] Rollback triggers activate on >5% error rate or >20% message deviation
- [x] Stable and canary Services enable traffic splitting without service mesh
- [x] ServiceMonitor configures Prometheus to scrape /metrics every 1 minute
- [x] Kustomize structure provides base manifests with production overlay for replica count and image tags
- [x] terminationGracePeriodSeconds: 30 and preStop hook prevent thundering herd during rollback

## Commits

| Hash | Type | Description |
|------|------|-------------|
| a7ac52c | feat | Add Argo Rollout manifest with canary strategy |
| 80fea96 | feat | Add AnalysisTemplate with Prometheus queries |
| 01fbe8d | feat | Add Services, Kustomize structure, and ServiceMonitor |
| dc7277b | fix | Fix Kustomize patch targeting and deprecation |

## Next Steps

1. **Phase 12 Plan 03**: Pre-rollout validation checklist
   - Verify official listener metrics baseline
   - Confirm Prometheus scraping InnerTube metrics (from 12-01)
   - Test AnalysisTemplate queries return valid data

2. **Phase 12 Plan 04**: Execute canary deployment
   - Apply manifests to production cluster
   - Monitor analysis runs during 10% and 50% stages
   - Document promotion decision timeline

## Self-Check: PASSED

**Created files verified**:
```bash
FOUND: deployments/k8s/youtube-listener-innertube/base/rollout.yaml
FOUND: deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml
FOUND: deployments/k8s/youtube-listener-innertube/base/service.yaml
FOUND: deployments/k8s/youtube-listener-innertube/base/kustomization.yaml
FOUND: deployments/k8s/youtube-listener-innertube/production/kustomization.yaml
FOUND: deployments/k8s/youtube-listener-innertube/production/rollout-patch.yaml
FOUND: deployments/k8s/monitoring/servicemonitor-innertube.yaml
```

**Commits verified**:
```bash
FOUND: a7ac52c
FOUND: 80fea96
FOUND: 01fbe8d
FOUND: dc7277b
```

**Kustomize build**: Successfully generates 228 lines of valid Kubernetes YAML.
