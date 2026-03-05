# YouTube Listener InnerTube Troubleshooting Guide

Comprehensive issue diagnosis and resolution guide for InnerTube canary rollout operations.

## Issue 1: Automatic Rollback Triggered

### Symptoms

- Rollout status changes from "Progressing" to "Degraded"
- Canary traffic returns to 0%
- AnalysisRun shows "Failed" status
- Grafana dashboard shows red status panels

### Diagnosis

**Step 1: Identify which metric breached threshold**

```bash
kubectl describe analysisrun <name> -n allchat | grep -A 10 "Failed"
```

Look for:
- `error-rate` metric failed: Error rate exceeded 5%
- `message-rate-deviation` metric failed: Message rate >20% different from baseline

**Step 2: Review Grafana dashboard for spike**

Navigate to "YouTube Listener InnerTube Rollout" dashboard:
- Check "Error Rate Comparison" panel: Look for red spikes above 5% threshold
- Check "Message Rate Comparison" panel: Look for orange/red zones (>5% deviation)

**Step 3: Check canary pod logs**

```bash
# Get canary pod template hash
kubectl get rs -n allchat -l app=youtube-listener-innertube

# View canary logs
kubectl logs -n allchat -l app=youtube-listener-innertube,rollouts-pod-template-hash=<canary-hash> --tail=200
```

Search for:
- HTTP error codes (401, 403, 429, 500)
- Redis connection errors
- Panic/crash messages

### Resolution

**If error rate >5%:**

1. **Check for InnerTube API changes**
   - Review HTTP response codes in logs
   - Look for new error patterns (e.g., "invalid continuation token")
   - Check YouTube's public status page for API issues

2. **Investigate authentication failures**
   - Verify API key still valid
   - Check for rate limiting (429 responses)
   - Review recent YouTube ToS changes

3. **Deploy fix**
   ```bash
   # Fix code, rebuild image
   docker build -t allchat/youtube-listener-innertube:v1.2.1 .
   docker push allchat/youtube-listener-innertube:v1.2.1

   # Update kustomization
   vim deployments/k8s/youtube-listener-innertube/production/kustomization.yaml

   # Restart rollout
   kubectl argo rollouts restart youtube-listener-innertube -n allchat
   ```

**If message rate drops >20%:**

1. **Check Redis connectivity**
   ```bash
   kubectl logs -n allchat -l app=youtube-listener-innertube | grep -i redis
   ```
   Look for: "connection refused", "timeout", "publish failed"

2. **Verify channel assignments**
   ```bash
   # Check source-manager logs for assignment changes
   kubectl logs -n allchat -l app=source-manager --tail=100
   ```
   Ensure canary is receiving expected channel assignments.

3. **Verify network policies**
   ```bash
   kubectl get networkpolicy -n allchat
   kubectl describe networkpolicy youtube-listener-innertube -n allchat
   ```
   Confirm canary pods can reach Redis.

**If Redis publish failures:**

1. **Check Redis health**
   ```bash
   kubectl logs -n allchat -l app=redis
   kubectl exec -it redis-0 -n allchat -- redis-cli INFO
   ```

2. **Verify connection pool settings**
   - Check `REDIS_POOL_SIZE` environment variable
   - Review connection pool exhaustion errors in logs

3. **Deploy fix and restart rollout** (see commands above)

---

## Issue 2: Rollout Stuck at 10% for >6 Hours

### Symptoms

- Canary weight remains at 10% for extended period
- Analysis runs continuously show "Running" status (not passing or failing)
- Grafana dashboard shows metrics hovering near thresholds
- No automatic promotion to 50%

### Diagnosis

**Step 1: Check analysis success rate**

```bash
kubectl get analysisrun -n allchat
```

Look for:
- Phase: "Running"
- Status: "Progressing" (not "Successful")
- Age: >6 hours

**Step 2: Verify metrics return valid data**

Test each AnalysisTemplate query manually:

```bash
# Test error rate query
curl 'http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=sum(rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m]))/sum(rate(youtube_listener_requests_total{service="youtube-listener-innertube-canary"}[5m]))'

# Test message rate query
curl 'http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=sum(rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]))'
```

Expected: JSON response with numeric result value.

**Step 3: Check if error rate oscillating**

Review Grafana "Error Rate Comparison" panel:
- Is error rate bouncing between 0.8% and 1.2%?
- Are there intermittent spikes above 1% threshold?

### Resolution

**If metrics unavailable:**

1. **Verify ServiceMonitor scraping**
   ```bash
   kubectl get servicemonitor youtube-listener-innertube -n allchat
   kubectl describe servicemonitor youtube-listener-innertube -n allchat
   ```

2. **Check Prometheus targets**
   Navigate to Prometheus UI > Targets:
   - Look for `youtube-listener-innertube` target
   - Verify "UP" status

3. **Restart canary pods to re-expose metrics**
   ```bash
   kubectl rollout restart rollout/youtube-listener-innertube -n allchat
   ```

**If error rate hovering at 0.9-1.1%:**

- **This is expected behavior:** Analysis requires 240 consecutive minutes below 1%
- **Wait for metrics to stabilize:** Intermittent spikes reset the counter
- **Monitor for patterns:** If spikes correlate with specific events (channel joins, API rate limits), investigate root cause

**If stuck indefinitely:**

1. **Review analysis template configuration**
   ```bash
   kubectl get analysistemplate youtube-listener-innertube-analysis -n allchat -o yaml
   ```
   Verify `count: 240` (4 hours of 1-minute intervals)

2. **Manual promotion (emergency only)**
   ```bash
   kubectl argo rollouts promote youtube-listener-innertube -n allchat
   ```
   **Caution:** Only use if metrics are clearly stable but analysis not progressing due to technical issue.

---

## Issue 3: Thundering Herd During Rollback

### Symptoms

- Official listener CPU spikes to >80% immediately after rollback
- Redis connection errors in official listener logs
- High latency or message delivery delays
- Official listener HPA scaling up rapidly but not fast enough

### Diagnosis

**Step 1: Check official listener resource usage**

```bash
kubectl top pods -n allchat -l app=youtube-listener
```

Look for:
- CPU >80% across all pods
- Memory approaching limits
- Multiple pods in high utilization

**Step 2: Review official listener HPA**

```bash
kubectl get hpa youtube-listener -n allchat
kubectl describe hpa youtube-listener -n allchat
```

Check:
- Current replicas vs desired replicas
- Scaling events (recent scale-up actions)
- Target CPU percentage (should be 50-60%, not 80%)

**Step 3: Check Redis connection pool**

```bash
kubectl exec -it redis-0 -n allchat -- redis-cli INFO clients | grep connected_clients
```

Compare to pre-rollback baseline:
- Spike in connections indicates thundering herd
- Connection limit approaching max

### Resolution

**Immediate: Manual scaling**

If HPA not responsive, scale manually:

```bash
kubectl scale deployment youtube-listener -n allchat --replicas=15
```

This provides immediate relief while HPA catches up.

**Long-term: Adjust HPA configuration**

Edit HPA to use lower CPU threshold:

```bash
kubectl edit hpa youtube-listener -n allchat
```

Change `targetAverageUtilization` from 80% to 50-60%:

```yaml
spec:
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60  # Lower threshold for faster scaling
```

**Prevention: Verify termination grace period**

Ensure Rollout manifest has proper grace period:

```bash
kubectl get rollout youtube-listener-innertube -n allchat -o yaml | grep terminationGracePeriodSeconds
```

Expected: `terminationGracePeriodSeconds: 30`

This allows gradual traffic shift during rollback.

---

## Issue 4: Canary Pods Crashlooping

### Symptoms

- Canary pods restart every 30-60 seconds
- Readiness probe failures
- Rollout stuck at 0% (never progresses to 10%)
- Pod status shows "CrashLoopBackOff"

### Diagnosis

**Step 1: Check pod events**

```bash
kubectl describe pod <canary-pod> -n allchat
```

Look for:
- `Back-off restarting failed container`
- `Readiness probe failed`
- `Liveness probe failed`
- `OOMKilled` (out of memory)

**Step 2: Review pod logs (previous container)**

```bash
kubectl logs <canary-pod> -n allchat --previous
```

Search for:
- Panic stack traces
- `fatal error` messages
- Connection failures
- Missing environment variables

**Step 3: Verify environment variables**

```bash
kubectl get rollout youtube-listener-innertube -n allchat -o yaml | grep -A 20 env:
```

Confirm required variables:
- `REDIS_HOST`
- `REDIS_PORT`
- `SOURCE_MANAGER_URL`
- `LOG_LEVEL`

### Resolution

**If missing Redis connection:**

1. **Check REDIS_HOST environment variable**
   ```bash
   kubectl get rollout youtube-listener-innertube -n allchat -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="REDIS_HOST")].value}'
   ```

2. **Verify Redis service**
   ```bash
   kubectl get svc redis -n allchat
   ```
   Expected: Service exists with ClusterIP.

3. **Update Rollout manifest** if missing:
   ```yaml
   env:
   - name: REDIS_HOST
     value: "redis.allchat.svc.cluster.local"
   ```

**If source-manager authentication failure:**

1. **Verify SOURCE_MANAGER_SECRET**
   ```bash
   kubectl get secret source-manager-secret -n allchat
   ```

2. **Check source-manager logs**
   ```bash
   kubectl logs -n allchat -l app=source-manager | grep "authentication"
   ```

3. **Regenerate secret if expired/invalid**

**If image pull errors:**

1. **Verify image tag exists**
   ```bash
   docker images | grep youtube-listener-innertube
   ```

2. **Check image pull secrets**
   ```bash
   kubectl get rollout youtube-listener-innertube -n allchat -o jsonpath='{.spec.template.spec.imagePullSecrets}'
   ```

3. **Push image to registry if missing**
   ```bash
   docker push allchat/youtube-listener-innertube:v1.2.0
   ```

---

## Issue 5: Message Rate Deviation >5% but <20%

### Symptoms

- Grafana shows 7-15% message rate difference between InnerTube and official
- Analysis continues (not failing, not passing quickly)
- No automatic rollback triggered
- Deviation persists for >1 hour

### Diagnosis

**Step 1: Check if difference is consistent**

Review Grafana "Message Rate Comparison" panel:
- Is deviation steady at 10%?
- Or does it spike intermittently?

**Step 2: Verify both services processing same channels**

```bash
# Check official listener channel assignments
kubectl logs -n allchat -l app=youtube-listener --tail=50 | grep "channel_id"

# Check InnerTube channel assignments
kubectl logs -n allchat -l app=youtube-listener-innertube --tail=50 | grep "channel_id"
```

Compare channel lists: Should be identical (or proportional based on traffic split).

**Step 3: Compare per-channel message rates**

Run Prometheus query grouped by channel:

```promql
sum by (channel_id) (rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]))
```

vs

```promql
sum by (channel_id) (rate(youtube_listener_messages_published_total{service="youtube-listener"}[5m]))
```

Identify which channels have discrepancies.

### Resolution

**If consistent 7-15% deviation:**

- **May indicate InnerTube parsing differences** (not a failure)
- **Investigate further:** Check message content for missing/extra messages
- **Compare event types:** InnerTube may capture deletion events (official doesn't)
- **Document findings:** Update research notes for Phase 13

**If intermittent spikes:**

- **Normal variance:** YouTube chat is bursty (super chats, spam)
- **Wait for analysis to accumulate more samples:** 5-minute rate smooths out spikes
- **Continue monitoring:** If spikes don't exceed 20%, rollout will proceed

**If specific channels missing from InnerTube:**

1. **Check source-manager channel assignment**
   ```bash
   kubectl logs -n allchat -l app=source-manager | grep "assign.*youtube-listener-innertube"
   ```

2. **Verify stream discovery**
   - Check if InnerTube correctly resolved channel → video ID
   - Review HTML parsing logs for failures

3. **Manual channel assignment** (temporary workaround):
   ```bash
   # Force reassignment via source-manager API
   curl -X POST http://source-manager.allchat.svc.cluster.local/api/channels/<channel-id>/reassign
   ```

---

## Issue 6: Fix-in-Place Not Applying to Canary Pods

### Symptoms

- Updated image deployed (new tag in kustomization)
- Canary pods still running old version
- `kubectl describe rollout` shows old image
- Fix not taking effect during soak

### Diagnosis

**Step 1: Check Rollout image**

```bash
kubectl get rollout youtube-listener-innertube -n allchat -o yaml | grep image:
```

Expected: Should show new image tag (e.g., `v1.2.1`).

**Step 2: Verify Kustomize applied**

```bash
kubectl kustomize deployments/k8s/youtube-listener-innertube/production/ | grep image:
```

Compare to Rollout image: Should match.

**Step 3: Check pod template hash**

```bash
kubectl get rs -n allchat -l app=youtube-listener-innertube
```

Look for:
- Multiple ReplicaSets (old and new)
- Pod counts per ReplicaSet
- Image tags per ReplicaSet

### Resolution

**If image not updated in Rollout:**

1. **Reapply Kustomize with force**
   ```bash
   kubectl apply -k deployments/k8s/youtube-listener-innertube/production/ --force
   ```

2. **Verify Rollout updated**
   ```bash
   kubectl get rollout youtube-listener-innertube -n allchat -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```

**If ReplicaSet not rolling:**

1. **Manual restart rollout**
   ```bash
   kubectl argo rollouts restart youtube-listener-innertube -n allchat
   ```

   This triggers new ReplicaSet creation with updated image.

2. **Watch rollout progress**
   ```bash
   kubectl argo rollouts get rollout youtube-listener-innertube -n allchat --watch
   ```

   Verify canary pods are recreated with new image.

**If persistent issue:**

1. **Abort rollout**
   ```bash
   kubectl argo rollouts abort youtube-listener-innertube -n allchat
   ```

2. **Apply fix to stable version**
   Update stable image tag, apply, wait for stable pods to roll.

3. **Restart rollout from 0%**
   ```bash
   kubectl argo rollouts restart youtube-listener-innertube -n allchat
   ```

---

## Common Commands Reference

Quick copy-paste commands for troubleshooting:

### View Rollout Status

```bash
kubectl argo rollouts get rollout youtube-listener-innertube -n allchat
kubectl argo rollouts status youtube-listener-innertube -n allchat
```

### Check Analysis Runs

```bash
kubectl get analysisrun -n allchat
kubectl describe analysisrun <name> -n allchat
kubectl logs -n allchat analysisrun/<name>
```

### View Canary Pod Logs

```bash
# Get canary pods
kubectl get pods -n allchat -l app=youtube-listener-innertube,rollouts-pod-template-hash=<canary-hash>

# View logs
kubectl logs -n allchat -l app=youtube-listener-innertube,rollouts-pod-template-hash=<canary-hash> --tail=100 -f
```

### Abort and Rollback

```bash
kubectl argo rollouts abort youtube-listener-innertube -n allchat
kubectl argo rollouts get rollout youtube-listener-innertube -n allchat --watch
```

### Manual Promotion

```bash
kubectl argo rollouts promote youtube-listener-innertube -n allchat
```

### Check Metrics in Prometheus

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Then visit http://localhost:9090 and query:
# - sum(rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m]))
# - sum(rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]))
```

### Scale Official Listener Manually

```bash
kubectl scale deployment youtube-listener -n allchat --replicas=15
```

---

## Escalation Checklist

Before escalating to senior engineer:

- [ ] Collected canary pod logs (`kubectl logs`)
- [ ] Collected analysis run details (`kubectl describe analysisrun`)
- [ ] Checked Grafana dashboard for threshold breaches
- [ ] Verified Prometheus metrics returning data
- [ ] Reviewed recent code changes (`git log`)
- [ ] Tested manual promotion (if safe to do so)
- [ ] Documented timeline of events (when rollout started, when issue appeared)

## Related Documentation

- [Deployment Runbook](./ROLLOUT_GUIDE.md) - Step-by-step deployment guide
- [InnerTube Service README](../../services/youtube-listener-innertube/README.md) - Service architecture
- [Grafana Dashboard](../../deployments/grafana/dashboards/innertube-rollout.json) - Monitoring configuration
- [Argo Rollouts Docs](https://argoproj.github.io/argo-rollouts/) - Official Argo documentation
