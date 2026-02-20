---
phase: 08-observability-production-readiness
plan: 04
subsystem: monitoring-observability
tags: [grafana, prometheus, alerts, dashboards, sharding, rebalancing]
dependencies:
  requires: [08-01, 08-03]
  provides: [grafana-sharding-dashboard, shard-alerts]
  affects: [operations, incident-response]
tech_stack:
  added: [grafana-dashboards]
  patterns: [prometheus-alerts, grafana-heatmaps, alert-remediation]
key_files:
  created:
    - deployments/k8s/monitoring/grafana-dashboards/sharding-overview.json
  modified:
    - deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml
    - deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml
decisions:
  - title: Two heatmaps for comprehensive distribution visibility
    rationale: Pod×Platform shows platform-specific imbalances, Pod×Time shows rebalancing effectiveness over time
    alternatives: [single-heatmap, table-view]
  - title: At-a-glance imbalance ratio as primary dashboard metric
    rationale: Operators need immediate visibility into system health without analyzing complex heatmaps
  - title: Relaxed thrashing threshold (>5 in 30min vs >3 in 15min)
    rationale: Reduces alert noise while still catching persistent thrashing patterns
metrics:
  duration: 2
  tasks_completed: 3
  files_created: 1
  files_modified: 2
  commits: 3
  completed_at: 2026-02-20
---

# Phase 08 Plan 04: Grafana Dashboards & Prometheus Alerts Summary

**One-liner:** Comprehensive sharding dashboard with dual heatmaps (Pod×Platform and Pod×Time) and actionable Prometheus alerts for imbalance, split-brain, and thrashing detection.

## What Was Built

Created production-ready monitoring and alerting infrastructure for the dynamic sharding system:

### 1. Grafana Sharding Overview Dashboard

**8-panel comprehensive dashboard** (`sharding-overview.json`):

1. **Load Imbalance Ratio** (stat panel) - Primary at-a-glance metric with color thresholds (green <0.5, yellow <0.7, red ≥0.7)
2. **Healthy Pods** (stat panel) - Count of healthy listener pods
3. **Failed Pods** (stat panel) - Alert-level metric for pod failures
4. **Channel Distribution by Platform: Pod×Platform** (heatmap) - Platform-specific distribution visualization showing which pods handle which platforms
5. **Channel Distribution Over Time: Pod×Time** (heatmap) - Temporal view showing rebalancing effects as load shifts between pods
6. **Rebalancing Events Timeline** (graph) - 5-minute rate with alert annotations showing when imbalance alerts fired
7. **Recent Migration Events** (table) - Migration counts by status and reason
8. **Per-Pod Load Scores** (time series) - Composite load score (70% message rate + 30% channel count) for each pod

**Dashboard features:**
- 30-second auto-refresh for real-time monitoring
- Color-coded thresholds matching alert triggers
- Event markers linking Prometheus alerts to timeline visualization
- Spectral color scheme for heatmaps (high contrast for imbalances)

**Metrics referenced:**
- `shard_imbalance_ratio` (from 08-01)
- `shard_healthy_pods` (from 08-01)
- `shard_failed_pods` (from 08-01)
- `shard_channel_count` (from Phase 7)
- `shard_migration_total` (from Phase 7)
- `shard_pod_load_score` (from 08-01)
- `shard_rebalancing_total` (from Phase 7)

### 2. Prometheus Critical Alerts

Added two critical-level alerts to `allchat-critical-alerts.yaml`:

**ShardLoadImbalance** (warning severity):
- **Trigger:** `shard_imbalance_ratio > 0.7` for 10 minutes
- **Impact:** Uneven resource usage, potential performance degradation
- **Remediation:** 5-step process including cooldown check, rebalancing verification, dashboard review, manual trigger, pod failure check

**ShardCoordinatorSplitBrain** (critical severity):
- **Trigger:** `sum(shard_coordinator_is_leader) > 1` for 1 minute
- **Impact:** Multiple coordinators causing conflicting assignments, duplicate messages, migration conflicts
- **Remediation:** Immediate action required - pod inspection, leader election logs, Kubernetes Lease check, rollout restart, verification

### 3. Prometheus Warning Alerts

Added one warning-level alert to `allchat-warning-alerts.yaml`:

**ShardRebalancingThrashing** (warning severity):
- **Trigger:** `increase(shard_rebalancing_total[30m]) > 5` for 5 minutes
- **Impact:** Connection churn from frequent migrations, potential message loss
- **Remediation:** 5-step investigation process including logs review, migration failure analysis, load calculation verification, cooldown adjustment, oscillation detection
- **Mitigation:** Provides temporary disable option if thrashing persists

**Alert design patterns:**
- Detailed impact statements for triage prioritization
- Step-by-step remediation with exact kubectl commands
- References to Grafana dashboard for visual confirmation
- Temporary mitigation options for emergency situations
- Root cause explanations for operator education

## Deviations from Plan

None - plan executed exactly as written. All requirements met:

- ✅ Dashboard created with 8 panels
- ✅ Two heatmaps (Pod×Platform and Pod×Time) as specified
- ✅ At-a-glance imbalance ratio as primary view
- ✅ Rebalancing timeline with event markers
- ✅ Migration events table
- ✅ ShardLoadImbalance alert with 0.7 threshold for 10 minutes
- ✅ ShardCoordinatorSplitBrain alert with 1-minute trigger
- ✅ ShardRebalancingThrashing alert with >5 in 30min threshold
- ✅ All alerts include detailed remediation steps
- ✅ Valid JSON and YAML confirmed via kubectl dry-run

## Technical Details

### Dashboard Layout (12-column grid)

**Row 1 (y=0, h=4):** At-a-glance metrics
- Imbalance Ratio (x=0, w=6)
- Healthy Pods (x=6, w=3)
- Failed Pods (x=9, w=3)

**Row 2 (y=4, h=8):** Distribution heatmaps
- Pod×Platform (x=0, w=12)
- Pod×Time (x=12, w=12)

**Row 3 (y=12, h=8):** Event analysis
- Rebalancing Timeline (x=0, w=12)
- Migration Events Table (x=12, w=12)

**Row 4 (y=20, h=6):** Detailed metrics
- Per-Pod Load Scores (x=0, w=24, full-width with legend table)

### Alert Severity Mapping

| Alert | Severity | Duration | Justification |
|-------|----------|----------|---------------|
| ShardLoadImbalance | warning | 10m | Degraded performance, not user-visible failures |
| ShardCoordinatorSplitBrain | critical | 1m | Data integrity risk, duplicate messages |
| ShardRebalancingThrashing | warning | 5m | Operational concern, not immediate user impact |

Per CONTEXT.md guidance: Critical = message loss/user-visible failures, Warning = degraded performance.

### PromQL Query Examples

**Imbalance ratio calculation** (from 08-01 metrics):
```promql
shard_imbalance_ratio
```

**Platform-specific distribution** (heatmap aggregation):
```promql
sum(shard_channel_count) by (pod_id, platform)
```

**Rebalancing rate** (5-minute window):
```promql
increase(shard_rebalancing_total[5m])
```

**Split-brain detection** (leader count):
```promql
sum(shard_coordinator_is_leader) > 1
```

**Thrashing detection** (30-minute window):
```promql
increase(shard_rebalancing_total[30m]) > 5
```

## Testing & Validation

### JSON/YAML Validation
```bash
# Dashboard JSON validity
jq . sharding-overview.json > /dev/null
✓ Valid JSON

# Alert YAML validity
kubectl apply --dry-run=client -f allchat-critical-alerts.yaml
✓ prometheusrule.monitoring.coreos.com/allchat-critical-alerts configured (dry run)

kubectl apply --dry-run=client -f allchat-warning-alerts.yaml
✓ prometheusrule.monitoring.coreos.com/allchat-warning-alerts configured (dry run)
```

### Panel Count Verification
```bash
jq '.dashboard.panels | length' sharding-overview.json
# Output: 8 ✓
```

### Heatmap Verification (CRITICAL)
```bash
jq '.dashboard.panels[] | select(.type == "heatmap") | .title' sharding-overview.json
# Output:
# "Channel Distribution by Platform: Pod × Platform" ✓
# "Channel Distribution Over Time: Pod × Time" ✓
```

### Metric Coverage Check
```bash
grep -o "shard_[a-z_]*" sharding-overview.json | sort -u
# Output: 7 unique shard metrics ✓
```

### Alert Coverage Check
```bash
yq eval '.spec.groups[0].rules[].alert' allchat-critical-alerts.yaml | grep Shard
# ShardLoadImbalance ✓
# ShardCoordinatorSplitBrain ✓

yq eval '.spec.groups[0].rules[].alert' allchat-warning-alerts.yaml | grep Shard
# ShardRebalancingThrashing ✓
```

## Requirements Traceability

| Requirement | Implementation | Verification |
|-------------|----------------|--------------|
| METRICS-04 | shard_rebalancing_total tracked (Phase 7) | Used in dashboard timeline + thrashing alert |
| METRICS-06 | shard_imbalance_ratio tracked (08-01) | Primary dashboard stat + imbalance alert |
| METRICS-07 | Grafana dashboard with channel distribution | Two heatmaps (Pod×Platform, Pod×Time) |
| METRICS-08 | Dashboard shows rebalancing timeline | Timeline graph + alert annotations |
| METRICS-09 | Imbalance ratio alert >0.7 for 10min | ShardLoadImbalance alert implemented |
| METRICS-10 | Split-brain alert on multiple leaders | ShardCoordinatorSplitBrain alert implemented |
| METRICS-11 | Thrashing alert >5 rebalances in 30min | ShardRebalancingThrashing alert implemented |

All requirements satisfied with evidence in commits 2db6199, c73a191, 0e283e1.

## Self-Check: PASSED

**Created files exist:**
```bash
[ -f "deployments/k8s/monitoring/grafana-dashboards/sharding-overview.json" ]
✓ FOUND: sharding-overview.json (11K)
```

**Modified files updated:**
```bash
[ -f "deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml" ]
✓ FOUND: allchat-critical-alerts.yaml (+52 lines)

[ -f "deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml" ]
✓ FOUND: allchat-warning-alerts.yaml (+25 lines)
```

**Commits exist:**
```bash
git log --oneline --all | grep -q "2db6199"
✓ FOUND: 2db6199 feat(08-04): add Grafana sharding overview dashboard

git log --oneline --all | grep -q "c73a191"
✓ FOUND: c73a191 feat(08-04): add sharding critical alerts

git log --oneline --all | grep -q "0e283e1"
✓ FOUND: 0e283e1 feat(08-04): add rebalancing thrashing warning alert
```

**Dashboard validation:**
```bash
jq '.dashboard.panels[] | select(.type == "heatmap")' sharding-overview.json | jq -s 'length'
✓ FOUND: 2 heatmap panels (Pod×Platform and Pod×Time)
```

All artifacts verified. Plan execution complete.

## Next Steps

### Deployment (User Action Required)

**Step 1: Apply Grafana dashboard**
```bash
# Option A: Via kubectl ConfigMap
kubectl create configmap grafana-dashboard-sharding \
  --from-file=sharding-overview.json \
  -n monitoring \
  --dry-run=client -o yaml | kubectl apply -f -

# Option B: Via Grafana UI
# 1. Open Grafana (http://grafana.monitoring.svc.cluster.local)
# 2. Navigate to Dashboards → Import
# 3. Upload sharding-overview.json
# 4. Select Prometheus datasource
```

**Step 2: Apply Prometheus alerts**
```bash
kubectl apply -f deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml
kubectl apply -f deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml

# Verify alerts loaded
kubectl get prometheusrules -n monitoring
```

**Step 3: Verify dashboard rendering**
```bash
# Access Grafana
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80

# Open: http://localhost:3000
# Navigate to: Dashboards → Sharding & Rebalancing Overview
# Verify: Both heatmaps render with sample data
```

**Step 4: Test alerting pipeline**
```bash
# Manually trigger imbalance (testing only)
# This would require temporarily skewing channel distribution
# OR wait for natural imbalance to occur during load testing

# Verify alert fires
kubectl get prometheus -n monitoring -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

### Monitoring Production System

**Day 1-7: Baseline establishment**
- Monitor dashboard daily for normal load patterns
- Document typical imbalance ratio range
- Observe rebalancing frequency under normal operations
- Verify alert thresholds match production characteristics

**Week 2+: Alert tuning**
- Adjust ShardLoadImbalance threshold if too sensitive (currently 0.7)
- Modify thrashing window if alert fires frequently (currently >5 in 30min)
- Review remediation steps with ops team for accuracy

**Ongoing:**
- Use Pod×Platform heatmap to identify platform-specific imbalances
- Use Pod×Time heatmap to verify rebalancing effectiveness
- Monitor split-brain alert for Kubernetes Lease API issues
- Track migration success rates in table view

## Operations Reference

### Remediation Quick Reference

**High Imbalance Ratio:**
1. Check `kubectl logs -n allchat -l app=source-manager | grep "cooldown"`
2. Verify rebalancing enabled in config
3. Open Grafana → Sharding Overview → Heatmaps
4. Manual trigger: `curl -X POST http://source-manager:8088/admin/rebalance`
5. Check for pod failures: `kubectl get pods -n allchat | grep -v Running`

**Split-Brain Detected:**
1. Immediate: `kubectl get pods -n allchat -l app=source-manager`
2. Check logs: `kubectl logs -n allchat -l app=source-manager | grep "leader"`
3. Verify Lease: `kubectl get lease -n allchat coordinator-leader`
4. Restart: `kubectl rollout restart -n allchat deployment/source-manager`
5. Verify resolution: Check alert + metric

**Thrashing Detected:**
1. Review logs: `kubectl logs -n allchat -l app=source-manager | grep "rebalancing"`
2. Grafana → Migration Events timeline
3. Check Prometheus for pod message rates
4. Increase cooldown: `kubectl set env -n allchat deployment/source-manager REBALANCING_COOLDOWN=10m`
5. Emergency: `kubectl set env -n allchat deployment/source-manager REBALANCING_ENABLED=false`

### Dashboard Panels Explained

| Panel | Purpose | When to Check |
|-------|---------|---------------|
| Imbalance Ratio | System health at-a-glance | Every check-in, alert triage |
| Pod Health | Pod availability | During scale events, after deploys |
| Pod×Platform | Platform-specific imbalances | Platform outages, adding new platforms |
| Pod×Time | Rebalancing effectiveness | After rebalancing, during tuning |
| Timeline | Rebalancing frequency | Investigating thrashing, capacity planning |
| Migration Events | Migration success rates | Debugging failed migrations |
| Load Scores | Per-pod workload | Capacity planning, HPA tuning |

## Phase 8 Completion Status

With this plan complete, Phase 8 (Observability & Production Readiness) is **100% complete**:

- ✅ 08-01: Shard & Migration Metrics (foundational telemetry)
- ✅ 08-02: Health Check Enhancements (readiness/liveness improvements)
- ✅ 08-03: Rebalancing & Assignment Tracing (distributed tracing)
- ✅ 08-04: Grafana Dashboards & Prometheus Alerts (this plan)

**Production readiness checklist:**
- [x] Comprehensive metrics collection (shard, migration, rebalancing)
- [x] Enhanced health checks (filtered channel tracking)
- [x] Distributed tracing (rebalancing operations, channel migrations)
- [x] Grafana dashboards (sharding overview)
- [x] Prometheus alerts (imbalance, split-brain, thrashing)
- [x] Remediation documentation (embedded in alerts)

**Next milestone: v1.1 Listener Load Balancing - COMPLETE**

All 31 plans across 8 phases delivered. System is production-ready for dynamic sharding with comprehensive observability.
