# YouTube Listener InnerTube Canary Rollout Guide

Operator runbook for deploying the InnerTube listener using Argo Rollouts canary strategy with automatic promotion/rollback.

## Prerequisites

Before deploying the InnerTube canary, ensure:

- **Argo Rollouts controller** installed in cluster (`kubectl get deployment argo-rollouts -n argo-rollouts`)
- **kubectl-argo-rollouts CLI plugin** installed locally (`kubectl argo rollouts version`)
- **Prometheus and Grafana** operational (metrics-based promotion requires Prometheus)
- **Official YouTube listener** running and healthy (provides baseline for comparison)

## Pre-Deployment Checklist

Verify system health before starting rollout:

- [ ] **Verify official listener message rate baseline**
  ```bash
  kubectl top pods -n allchat -l app=youtube-listener
  ```
  Expected: Stable CPU/memory, no crashloops

- [ ] **Check Prometheus scraping InnerTube metrics**
  ```bash
  kubectl get servicemonitor youtube-listener-innertube -n allchat
  curl http://youtube-listener-innertube.allchat.svc.cluster.local:8080/metrics
  ```
  Expected: ServiceMonitor exists, metrics endpoint returns data

- [ ] **Confirm AnalysisTemplate created**
  ```bash
  kubectl get analysistemplate youtube-listener-innertube-analysis -n allchat
  ```
  Expected: AnalysisTemplate exists with error rate and message rate queries

- [ ] **Review recent changes**
  ```bash
  git log --oneline services/youtube-listener-innertube/ | head -5
  ```
  Verify you're deploying expected version

## Deployment Steps

### 1. Apply Argo Rollouts Manifests

```bash
kubectl apply -k deployments/k8s/youtube-listener-innertube/production/
```

This creates:
- Rollout resource (replaces Deployment)
- Service with canary and stable selectors
- ServiceMonitor for Prometheus scraping
- AnalysisTemplate for metrics-based promotion

### 2. Verify Rollout Created

```bash
kubectl get rollout youtube-listener-innertube -n allchat
```

Expected output:
```
NAME                          DESIRED   CURRENT   UP-TO-DATE   AVAILABLE
youtube-listener-innertube    5         5         5            5
```

### 3. Watch Rollout Progress (Real-Time)

```bash
kubectl argo rollouts get rollout youtube-listener-innertube -n allchat --watch
```

This shows:
- Current rollout phase (Progressing, Paused, Healthy, Degraded)
- Canary weight (traffic percentage)
- Stable and canary pod counts
- Analysis run status

### 4. Open Grafana Dashboard

Navigate to: **Dashboards > YouTube Listener InnerTube Rollout**

Monitor:
- Error rate comparison (InnerTube vs official)
- Message rate comparison
- Redis publish success rate
- Pod restart rate

## Rollout Timeline (Expected)

| Time Window | Phase | Canary Weight | Analysis Duration | Promotion Gate |
|-------------|-------|---------------|-------------------|----------------|
| 0-5 min | Initial deployment | 0% → 10% | - | Pods ready |
| 5 min - 4 hours | First soak | 10% | 240 minutes | Error rate <1%, message rate within 5% |
| 4 hours - 8 hours | Second soak | 50% | 240 minutes | Error rate <1%, message rate within 5% |
| 8 hours - 8.5 hours | Full promotion | 100% | - | Analysis passed |

**Total expected rollout time:** 8 hours (if all analysis gates pass)

**Note:** Rollout pauses indefinitely at each soak if metrics don't stabilize. Manual intervention may be required.

## Monitoring During Rollout

### Watch Analysis Runs

```bash
kubectl get analysisrun -n allchat
```

Expected output during soak:
```
NAME                                              STATUS      AGE
youtube-listener-innertube-<hash>-1-1            Running     45m
```

### Check Canary Weight

```bash
kubectl argo rollouts status youtube-listener-innertube -n allchat
```

Shows current traffic percentage to canary.

### Review Grafana Dashboard

Monitor these panels:
- **Error Rate Comparison:** Must be <1% for promotion (green zone)
- **Message Rate Comparison:** Must be within 5% of baseline
- **Rollout Status:** Shows Progressing/Paused/Healthy/Degraded state

### Spot-Check Canary Logs

```bash
# Get canary pod template hash
kubectl get rs -n allchat -l app=youtube-listener-innertube

# View canary logs
kubectl logs -n allchat -l app=youtube-listener-innertube,rollouts-pod-template-hash=<canary-hash> --tail=100
```

Look for:
- Successful Redis stream publishing
- No authentication errors
- Correct channel assignments from source-manager

## Manual Promotion (Emergency Use)

**Caution:** Manual promotion bypasses analysis gates. Use only in emergency situations.

### Promote to Next Step Immediately

```bash
kubectl argo rollouts promote youtube-listener-innertube -n allchat
```

This skips the current soak and advances to the next canary weight.

**When to use:**
- Analysis is stuck but metrics are clearly healthy
- Need to expedite rollout for critical bug fix
- Prometheus temporarily unavailable (metrics will backfill)

**Do not use if:**
- Error rate is elevated (let rollback trigger automatically)
- Canary pods are crashlooping
- Unsure of stability (wait for analysis to complete)

## Manual Rollback

If issues are detected, abort the rollout and revert to stable version:

### Abort Rollout

```bash
kubectl argo rollouts abort youtube-listener-innertube -n allchat
```

This immediately:
- Sets canary weight to 0%
- Routes all traffic to stable pods
- Marks rollout as "Degraded"

### Check Rollback Progress

```bash
kubectl argo rollouts get rollout youtube-listener-innertube -n allchat --watch
```

Watch for:
- Canary weight returns to 0%
- Stable pods remain at 5 replicas
- Rollout status becomes "Degraded"

### Restart Rollout (After Fix)

After deploying a fix (see Fix-in-Place workflow below):

```bash
kubectl argo rollouts restart youtube-listener-innertube -n allchat
```

This starts a new rollout from 0% canary weight.

## Post-Deployment Validation

After rollout completes (100% traffic on InnerTube):

### 1. Verify Full Promotion

```bash
kubectl argo rollouts status youtube-listener-innertube -n allchat
```

Expected:
- Rollout status: "Healthy"
- Canary weight: 100%
- All 5 pods running InnerTube version

### 2. Compare Message Counts

Check Grafana "Message Rate Comparison" panel:
- InnerTube and official listener should have similar message rates
- Deviation should be <5% over 5-minute window

**Note:** Official listener can be scaled down after successful validation.

### 3. Validate Offline Detection

Stop a live YouTube stream, confirm InnerTube stops polling:

```bash
# Watch logs for offline detection
kubectl logs -n allchat -l app=youtube-listener-innertube --tail=20 -f | grep "offline"
```

Expected: "Stream detected offline, stopping poll" after 2-3 empty continuation checks.

### 4. Monitor for 24 Hours

Continue monitoring:
- Error rate remains <1%
- No unexpected pod restarts
- Redis publish success rate >99%
- Reconnection frequency remains low (<0.5 per second)

## Fix-in-Place Workflow

**User requirement:** Keep rollout at current percentage, deploy fix, resume promotion (do NOT abort).

If issues discovered during canary soak:

### 1. Keep Rollout at Current Percentage

**Do not abort.** Argo Rollouts will pause at current weight (10% or 50%).

### 2. Build and Tag Fixed Image

```bash
cd services/youtube-listener-innertube
docker build -t allchat/youtube-listener-innertube:v1.2.1 .
docker push allchat/youtube-listener-innertube:v1.2.1
```

### 3. Update Image Tag in Kustomization

Edit `deployments/k8s/youtube-listener-innertube/production/kustomization.yaml`:

```yaml
images:
  - name: allchat/youtube-listener-innertube
    newTag: v1.2.1  # Update to new version
```

### 4. Apply Updated Manifests

```bash
kubectl apply -k deployments/k8s/youtube-listener-innertube/production/
```

Argo Rollouts will:
- Detect image change
- Update canary pods with new image
- Keep traffic percentage unchanged (still at 10% or 50%)
- Continue running analysis with new canary version

### 5. Verify Fix Applied to Canary

```bash
# Check canary pod image
kubectl get pods -n allchat -l app=youtube-listener-innertube -o jsonpath='{.items[*].spec.containers[0].image}'
```

Expected: Canary pods show `v1.2.1`, stable pods show original version.

### 6. Resume Promotion

If fix successful and analysis passes:
- Rollout automatically resumes promotion to next step
- No manual intervention required

If fix unsuccessful:
- Analysis will continue to fail
- Automatic rollback will trigger after consecutive failures

## Troubleshooting Quick Reference

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| Rollout stuck in Paused | Analysis waiting for metrics to stabilize | Wait for 240 consecutive successes, or manual promote if metrics clearly healthy |
| Automatic rollback triggered | Error rate >5% or message rate >20% deviation | Check Grafana for threshold breach, investigate canary logs, deploy fix |
| Canary pods crashlooping | Missing env vars, Redis connection failure | Check pod events, verify environment variables, test Redis connectivity |
| Promotion not progressing | AnalysisTemplate queries not returning data | Verify ServiceMonitor scraping, check Prometheus targets |
| Thundering herd on rollback | Official listener HPA not scaling fast enough | Manually scale official listener: `kubectl scale deployment youtube-listener -n allchat --replicas=15` |
| Fix-in-place not applying | Image tag not updated in Rollout | Verify Kustomize applied, force reapply with `--force`, or restart rollout |

For detailed troubleshooting, see [TROUBLESHOOTING_INNERTUBE.md](./TROUBLESHOOTING_INNERTUBE.md).

## Success Criteria

Rollout is successful when:

- [ ] Rollout status is "Healthy"
- [ ] Canary weight is 100%
- [ ] Error rate <1% for 24 hours post-rollout
- [ ] Message rate within 5% of official listener baseline
- [ ] No pod restarts (except during image updates)
- [ ] Offline detection working (streams stop polling when ended)

## Next Steps

After successful rollout:

1. **Scale down official listener** (optional): Reduce replicas to 1 for backup
2. **Monitor for 1 week:** Watch for long-tail issues (rate limiting, API changes)
3. **Phase 13 preparation:** Plan deletion event support (advanced InnerTube feature)
4. **Update runbooks:** Document any issues encountered during rollout

## References

- [Argo Rollouts Documentation](https://argoproj.github.io/argo-rollouts/)
- [InnerTube Service README](../../services/youtube-listener-innertube/README.md)
- [Troubleshooting Guide](./TROUBLESHOOTING_INNERTUBE.md)
- [Grafana Dashboard](../../deployments/grafana/dashboards/innertube-rollout.json)
