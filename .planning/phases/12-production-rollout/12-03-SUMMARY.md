---
phase: 12-production-rollout
plan: 03
subsystem: documentation-observability
tags: [grafana, documentation, canary, troubleshooting]
dependency_graph:
  requires:
    - "12-02-argo-rollouts-manifests"
  provides:
    - "grafana-rollout-dashboard"
    - "deployment-runbook"
    - "troubleshooting-guide"
  affects:
    - "operator-workflows"
    - "incident-response"
tech_stack:
  added:
    - "Grafana dashboard JSON"
  patterns:
    - "Operator runbook with fix-in-place workflow"
    - "Issue-based troubleshooting guide"
    - "ToS disclosure for unofficial API usage"
key_files:
  created:
    - "deployments/grafana/dashboards/innertube-rollout.json"
    - "docs/deployment/ROLLOUT_GUIDE.md"
    - "docs/deployment/TROUBLESHOOTING_INNERTUBE.md"
  modified:
    - "services/youtube-listener-innertube/README.md"
decisions:
  - summary: "8-panel dashboard layout: rollout status (top) → metric comparison (middle) → detailed diagnostics (bottom)"
    context: "Research Pattern 3 - hierarchical information architecture for at-a-glance status and drill-down analysis"
  - summary: "Fix-in-place workflow documented as preferred approach (not abort-fix-retry)"
    context: "User requirement from 12-CONTEXT.md - keep rollout at current percentage, deploy fix, resume promotion"
  - summary: "ToS disclosure placed at top of README with warning emoji"
    context: "InnerTube is unofficial API - ensure visibility for operators, not user-facing UI"
  - summary: "6 common issues identified from research pitfalls and Phase 11 findings"
    context: "Automatic rollback, stuck rollout, thundering herd, crashlooping, message deviation, fix-in-place failures"
metrics:
  duration_minutes: 10
  tasks_completed: 3
  files_created: 3
  files_modified: 1
  commits: 3
  completion_date: "2026-03-05"
---

# Phase 12 Plan 03: Grafana Dashboard and Deployment Documentation Summary

**One-liner:** Grafana dashboard for canary observability, deployment runbook with fix-in-place workflow, and comprehensive troubleshooting guide for 6 common rollout issues

## What Was Built

### 1. Grafana Dashboard (innertube-rollout.json)

Created 8-panel dashboard following Research Pattern 3 (hierarchical layout):

**Top Row (Status at a Glance):**
- **Panel 1: Rollout Status** - Stat panel with color-coded phases (Healthy=green, Progressing=blue, Paused=yellow, Degraded=red)
- **Panel 2: Canary Weight** - Gauge showing traffic percentage (0-100%) with threshold markers at 10%, 50%
- **Panel 3: Analysis Run Status** - Table of current and historical analysis runs with success/failure status

**Middle Row (Metric Comparison):**
- **Panel 4: Error Rate Comparison** - Time series comparing InnerTube canary vs official listener baseline
  - Thresholds: <1%=green (promotion gate), 1-5%=yellow, >5%=red (rollback trigger)
- **Panel 5: Message Rate Comparison** - Messages per second comparison
  - 5% deviation threshold for analysis gate

**Bottom Row (Detailed Diagnostics):**
- **Panel 6: Redis Publish Success Rate** - Canary publish reliability (>99% required)
- **Panel 7: Pod Restart Rate** - Restarts per minute (>2 triggers rollback concern)
- **Panel 8: Reconnection Frequency** - InnerTube polling stability monitoring

**Dashboard Configuration:**
- 30s refresh interval for real-time monitoring
- Variables: `$datasource` (Prometheus), `$namespace` (default: allchat)
- All panels use PromQL queries against `argo_rollouts_*` and `youtube_listener_*` metrics

### 2. Deployment Runbook (ROLLOUT_GUIDE.md)

Comprehensive operator guide with 10 sections:

1. **Prerequisites** - Argo Rollouts, kubectl plugin, Prometheus/Grafana operational
2. **Pre-Deployment Checklist** - 4-step verification (baseline metrics, ServiceMonitor, recent changes)
3. **Deployment Steps** - kubectl apply, verification, real-time monitoring
4. **Rollout Timeline** - Expected 8-hour progression (0%→10%→50%→100%)
5. **Monitoring During Rollout** - Analysis runs, canary weight, Grafana dashboard, log checks
6. **Manual Promotion** - Emergency override (skip soak when needed)
7. **Manual Rollback** - Abort procedure and rollback progress monitoring
8. **Post-Deployment Validation** - 4-step verification (full promotion, message counts, offline detection, 24-hour monitoring)
9. **Fix-in-Place Workflow** - 6-step procedure for deploying fixes without aborting rollout (user requirement)
10. **Troubleshooting Quick Reference** - 6 common issues with symptoms/causes/resolutions

**Key Features:**
- Step-by-step kubectl commands with expected outputs
- Detailed timeline table (time window, phase, canary weight, analysis duration, promotion gates)
- Fix-in-place workflow emphasized (build → tag → update kustomization → apply → verify → auto-resume)
- Success criteria checklist

### 3. Troubleshooting Guide (TROUBLESHOOTING_INNERTUBE.md)

6 common issues with full diagnosis and resolution:

**Issue 1: Automatic Rollback Triggered**
- Symptoms: Rollout status "Degraded", canary traffic 0%, AnalysisRun "Failed"
- Diagnosis: Identify breached metric (error rate >5% or message rate >20%), review Grafana, check logs
- Resolution: Investigate InnerTube API changes, check Redis connectivity, verify channel assignments, deploy fix

**Issue 2: Rollout Stuck at 10% for >6 Hours**
- Symptoms: Canary weight remains at 10%, analysis continuously "Running" (not passing)
- Diagnosis: Check analysis success rate, verify metrics return valid data, check oscillating error rate
- Resolution: Verify ServiceMonitor scraping, check Prometheus targets, wait for stabilization, manual promotion (emergency)

**Issue 3: Thundering Herd During Rollback** (Research Pitfall 1)
- Symptoms: Official listener CPU >80%, Redis connection errors, HPA not scaling fast enough
- Diagnosis: Check resource usage, review HPA configuration, check Redis connection pool
- Resolution: Manual scaling (15 replicas), adjust HPA to 50-60% threshold, verify termination grace period

**Issue 4: Canary Pods Crashlooping**
- Symptoms: Restarts every 30-60s, readiness probe failures, "CrashLoopBackOff" status
- Diagnosis: Check pod events, review logs (previous container), verify environment variables
- Resolution: Check Redis connection, verify source-manager secret, fix image pull errors

**Issue 5: Message Rate Deviation >5% but <20%**
- Symptoms: Grafana shows 7-15% difference, analysis continues (not failing), deviation persists >1 hour
- Diagnosis: Check consistency (steady vs intermittent), verify channel assignments, compare per-channel rates
- Resolution: Document InnerTube parsing differences (Phase 13 research), wait for variance to smooth, manual channel assignment

**Issue 6: Fix-in-Place Not Applying to Canary Pods**
- Symptoms: Updated image deployed, canary pods still running old version
- Diagnosis: Check Rollout image, verify Kustomize applied, check pod template hash
- Resolution: Reapply with `--force`, manual rollout restart, abort/fix stable/restart (fallback)

**Additional Sections:**
- Common Commands Reference (copy-paste ready)
- Escalation Checklist (what to collect before escalating)
- Related Documentation links

### 4. README Updates (youtube-listener-innertube/README.md)

**ToS Disclosure Added:**
```markdown
> ⚠️ Terms of Service Disclosure: This service uses the InnerTube API, which is an unofficial YouTube API not documented or supported by Google. Use of this API may violate YouTube's Terms of Service. This implementation is intended for personal use and small-scale deployments. For production use at scale, consider YouTube's official Data API with proper quota management.
```

**Updated Sections:**
- Status changed from "Phase 9 Proof of Concept" to "Phase 12 - Production Rollout"
- Added "Key Differences from Official Listener" comparison table
- Updated Deployment section with canary rollout reference
- Added Monitoring section with Grafana dashboard and key metrics
- Added Troubleshooting section with guide reference

## Deviations from Plan

None - plan executed exactly as written.

## Decisions Made

### 1. Dashboard Panel Ordering
**Decision:** Place rollout status panels at top, metric comparisons in middle, detailed diagnostics at bottom.

**Rationale:** Research Pattern 3 suggests hierarchical information architecture - operators need quick status check (Healthy/Degraded?) before drilling into metrics.

**Alternative Considered:** Chronological layout (deployment flow: status → canary weight → metrics). Rejected because operators may check dashboard mid-rollout when they already know phase.

### 2. Fix-in-Place Emphasis
**Decision:** Dedicate full section to fix-in-place workflow in runbook, not just mention as option.

**Rationale:** User requirement from 12-CONTEXT.md - "keep rollout at current percentage, deploy fix, resume promotion" is preferred pattern. Abort-fix-retry resets progress.

**Implementation:** 6-step procedure in runbook (build → tag → update → apply → verify → auto-resume) with troubleshooting issue dedicated to failures.

### 3. ToS Disclosure Placement
**Decision:** Top of README with warning emoji, not buried in "Known Limitations" section.

**Rationale:** InnerTube is unofficial API - operators must be aware before deployment. Prominent placement (above Overview) ensures visibility.

**Considered:** User-facing disclosure in UI. Rejected per 12-CONTEXT.md - "internal note only in README."

### 4. Troubleshooting Issue Selection
**Decision:** Document 6 issues: automatic rollback, stuck rollout, thundering herd, crashlooping, message deviation, fix-in-place failures.

**Rationale:** Selected based on:
- Research pitfalls (thundering herd from 12-RESEARCH.md)
- Phase 11 contract testing findings (message deviation)
- Standard Kubernetes rollout issues (crashlooping, image update failures)
- Argo Rollouts patterns (stuck analysis, automatic rollback)

**Deferred:** InnerTube-specific issues (rate limiting, API changes) will be documented post-rollout based on observed incidents.

## Technical Details

### Grafana Dashboard PromQL Queries

**Error Rate Comparison:**
```promql
# InnerTube Canary
sum(rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m])) /
sum(rate(youtube_listener_requests_total{service="youtube-listener-innertube-canary"}[5m]))

# Official Listener Baseline
sum(rate(youtube_listener_errors_total{service="youtube-listener"}[5m])) /
sum(rate(youtube_listener_requests_total{service="youtube-listener"}[5m]))
```

**Message Rate Comparison:**
```promql
# InnerTube Canary
sum(rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]))

# Official Listener Baseline
sum(rate(youtube_listener_messages_published_total{service="youtube-listener"}[5m]))
```

**Rollout Status:**
```promql
argo_rollouts_info{name="youtube-listener-innertube", namespace="allchat"}
```

### Fix-in-Place Workflow

1. **Keep rollout paused** (do not abort) - current canary weight preserved
2. **Build fixed image** - `docker build -t allchat/youtube-listener-innertube:v1.2.1 .`
3. **Update Kustomization** - `newTag: v1.2.1` in `production/kustomization.yaml`
4. **Apply manifests** - `kubectl apply -k deployments/k8s/youtube-listener-innertube/production/`
5. **Verify canary pods updated** - Check image tag via `kubectl get pods`
6. **Analysis auto-resumes** - If fix successful, rollout proceeds to next step

**Argo Rollouts behavior:**
- Detects image change in Rollout spec
- Creates new ReplicaSet for canary pods
- Keeps canary weight unchanged (still 10% or 50%)
- Continues running AnalysisTemplate with new canary version
- Promotes if analysis passes, rolls back if analysis fails

## Testing Performed

### Verification 1: Dashboard Structure
```bash
cat deployments/grafana/dashboards/innertube-rollout.json | jq '.dashboard.panels | length'
# Result: 8 (all panels defined)
```

### Verification 2: Dashboard Queries
```bash
cat deployments/grafana/dashboards/innertube-rollout.json | grep -c "argo_rollouts_info"
# Result: 1 (rollout status panel configured)
```

### Verification 3: Runbook Completeness
```bash
cat docs/deployment/ROLLOUT_GUIDE.md | grep -E "Prerequisites|Deployment Steps|Manual Rollback|Fix-in-Place" | wc -l
# Result: 5 (all sections present, extra match for "Fix-in-Place Workflow")
```

### Verification 4: Troubleshooting Issues
```bash
cat docs/deployment/TROUBLESHOOTING_INNERTUBE.md | grep -c "^## Issue"
# Result: 6 (all issues documented)
```

### Verification 5: ToS Disclosure
```bash
grep "Terms of Service" services/youtube-listener-innertube/README.md
# Result: Found - disclosure added at top of README
```

## Self-Check: PASSED

### Files Created
- `deployments/grafana/dashboards/innertube-rollout.json` - FOUND (603 lines)
- `docs/deployment/ROLLOUT_GUIDE.md` - FOUND (345 lines)
- `docs/deployment/TROUBLESHOOTING_INNERTUBE.md` - FOUND (666 lines)

### Files Modified
- `services/youtube-listener-innertube/README.md` - FOUND (ToS disclosure added, sections updated)

### Commits
- `1743adf` - feat(12-03): add Grafana dashboard for InnerTube canary rollout - FOUND
- `2286447` - docs(12-03): add deployment runbook for InnerTube canary rollout - FOUND
- `f9daed7` - docs(12-03): add troubleshooting guide and ToS disclosure - FOUND

All claims verified. No discrepancies.

## Lessons Learned

### What Went Well

1. **Research-driven dashboard design** - Pattern 3 hierarchical layout directly applicable from 12-RESEARCH.md
2. **Fix-in-place workflow clarity** - User requirement well-documented, easy to translate to runbook steps
3. **Issue categorization** - 6 issues cover research pitfalls + standard Kubernetes issues comprehensively

### What Could Be Improved

1. **Grafana panel ordering** - Could add annotations linking panels to troubleshooting sections (e.g., "If error rate red, see Issue 1")
2. **Troubleshooting frequency data** - No empirical data on issue frequency (will update post-rollout with actual incident counts)
3. **Dashboard export testing** - Dashboard JSON not imported to live Grafana (manual verification needed)

## Next Steps

### Immediate (Phase 12)
- [ ] Import dashboard to Grafana and verify panel rendering
- [ ] Test PromQL queries against Prometheus (ensure metrics available)
- [ ] Perform dry-run of deployment steps in staging environment

### Phase 13 (Deletion Events)
- [ ] Update dashboard with deletion event metrics
- [ ] Add troubleshooting section for deletion event parsing issues
- [ ] Document deletion event rate comparison (InnerTube vs official)

### Post-Rollout
- [ ] Update troubleshooting guide with actual incident data
- [ ] Add frequency indicators to issues (Common/Rare/Edge Case)
- [ ] Document any new issues discovered during production rollout

## References

- [12-02-SUMMARY.md](./12-02-SUMMARY.md) - Argo Rollouts manifests (ServiceMonitor, AnalysisTemplate)
- [12-RESEARCH.md](./12-RESEARCH.md) - Pattern 3 dashboard layout, canary pitfalls
- [12-CONTEXT.md](./12-CONTEXT.md) - Fix-in-place workflow requirement, ToS disclosure
- [deployments/k8s/youtube-listener-innertube/base/rollout.yaml](../../../deployments/k8s/youtube-listener-innertube/base/rollout.yaml) - Rollout configuration
- [deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml](../../../deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml) - Analysis metrics

## Metadata

**Plan Duration:** 10 minutes
**Tasks Completed:** 3/3
**Files Created:** 3
**Files Modified:** 1
**Total Lines Added:** 1,614
**Commits:** 3 (feat, docs, docs)
**Completion Date:** 2026-03-05

**Task Breakdown:**
- Task 1 (Grafana Dashboard): ~3 minutes - 603 lines JSON
- Task 2 (Deployment Runbook): ~3 minutes - 345 lines Markdown
- Task 3 (Troubleshooting Guide + README): ~4 minutes - 666 lines + 24 lines modified
