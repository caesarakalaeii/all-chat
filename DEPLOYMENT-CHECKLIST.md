# Coordinator Filtering Fix - Deployment Checklist

## Pre-Deployment Verification ✅

- [x] All tests pass (32/32 coordination tests)
- [x] source-manager builds successfully
- [x] twitch-listener builds successfully
- [x] Feature flag defaults to `false` (filtering disabled)
- [x] Metrics properly registered
- [x] Commit created with detailed message

---

## Phase 1: Deploy Heartbeat Cleanup (2-4 hours)

**Objective**: Eliminate ghost pods, reduce cleanup window from 5min to 15s

### Step 1.1: Build and Push Images
```bash
# Build source-manager
cd services/source-manager
docker build -t <your-registry>/source-manager:heartbeat-fix-$(date +%Y%m%d-%H%M%S) .
docker push <your-registry>/source-manager:heartbeat-fix-$(date +%Y%m%d-%H%M%S)

# Update deployment manifest
kubectl set image deployment/source-manager \
  source-manager=<your-registry>/source-manager:heartbeat-fix-$(date +%Y%m%d-%H%M%S) \
  -n allchat
```

### Step 1.2: Monitor Deployment
```bash
# Watch rollout
kubectl rollout status deployment/source-manager -n allchat

# Check logs
kubectl logs -n allchat -l app=source-manager --tail=100 -f
```

### Step 1.3: Verify Heartbeat Cleanup (15 minutes)
```bash
# Monitor heartbeat registry
watch -n 5 "kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:heartbeats 0 -1 WITHSCORES"

# Expected: No entries older than 15 seconds

# Check cleanup metrics
curl -s http://prometheus:9090/api/v1/query?query=shard_stale_heartbeats_removed_total | jq

# Check ghost pod detection
curl -s http://prometheus:9090/api/v1/query?query=shard_ghost_pods_detected_total | jq
```

### Step 1.4: Test Pod Deletion (5 minutes)
```bash
# Get a source-manager pod
POD=$(kubectl get pods -n allchat -l app=source-manager -o jsonpath='{.items[0].metadata.name}')

# Check it has heartbeat
kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:heartbeats 0 -1 | grep "$POD"

# Delete pod
kubectl delete pod -n allchat "$POD"

# Wait 20 seconds
sleep 20

# Verify heartbeat removed
kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:heartbeats 0 -1 | grep "$POD" || echo "✓ Heartbeat removed"
```

### Step 1.5: 24-Hour Monitoring
- [ ] `shard_ghost_pods_detected_total` = 0 for 24 hours
- [ ] `shard_stale_heartbeats_removed_total` increases when pods deleted
- [ ] No heartbeat entries persist >15 seconds
- [ ] No alerts from ghost pod detection

**✅ Phase 1 Complete** when all checks pass for 24 hours

---

## Phase 2: Deploy Coverage Verification (48-72 hours)

**Objective**: Monitor coverage metrics to ensure coordinator assigns all sources

**Prerequisites**:
- [ ] Phase 1 complete and verified for 24 hours
- [ ] No ghost pods detected in last 24 hours

### Step 2.1: Build and Push Images
```bash
# Build twitch-listener
cd services/twitch-listener
docker build -t <your-registry>/twitch-listener:coverage-check-$(date +%Y%m%d-%H%M%S) .
docker push <your-registry>/twitch-listener:coverage-check-$(date +%Y%m%d-%H%M%S)

# Update deployment manifest
kubectl set image deployment/twitch-listener \
  twitch-listener=<your-registry>/twitch-listener:coverage-check-$(date +%Y%m%d-%H%M%S) \
  -n allchat

# Verify feature flag is FALSE
kubectl get deployment twitch-listener -n allchat -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ENABLE_COORDINATOR_FILTERING")].value}'
# Expected output: false (or empty)
```

### Step 2.2: Monitor Deployment
```bash
# Watch rollout
kubectl rollout status deployment/twitch-listener -n allchat

# Check logs - should see filtering DISABLED message
kubectl logs -n allchat -l app=twitch-listener | grep "filtering DISABLED"
# Expected: "Coordinator filtering DISABLED via ENABLE_COORDINATOR_FILTERING environment variable"

# Verify all channels active (no filtering applied)
kubectl logs -n allchat -l app=twitch-listener | grep "total_active"
# Should show all 74 channels (or current total)
```

### Step 2.3: 48-Hour Coverage Monitoring
```bash
# Monitor coverage gaps (should be 0)
watch -n 300 "curl -s http://prometheus:9090/api/v1/query?query=shard_coverage_gaps_detected_total | jq '.data.result[0].value[1]'"

# Monitor unassigned sources (should be 0)
watch -n 300 "curl -s http://prometheus:9090/api/v1/query?query=shard_unassigned_sources | jq '.data.result[0].value[1]'"

# Check logs for coverage verification
kubectl logs -n allchat -l app=twitch-listener | grep "Coverage verification"
# Expected: "Coverage verification passed" (even though filtering disabled)
```

### Step 2.4: Verification Checklist (48 hours)
- [ ] `shard_coverage_gaps_detected_total` = 0 for 48 continuous hours
- [ ] `shard_unassigned_sources` = 0 for 48 continuous hours
- [ ] All pods publishing heartbeats successfully
- [ ] No coverage gap alerts fired
- [ ] All channels remain active (74 total or current count)

**✅ Phase 2 Complete** when all checks pass for 48 hours

---

## Phase 3: Enable Filtering (Canary) - 24 hours

**Objective**: Enable filtering on single pod, verify zero message loss

**Prerequisites**:
- [ ] Phase 2 complete and verified for 48 hours
- [ ] Coverage metrics = 0 for 48 hours
- [ ] **User approval obtained to proceed**

### Step 3.1: Scale to Canary (1 pod)
```bash
# Scale down to single pod
kubectl scale deployment/twitch-listener --replicas=1 -n allchat

# Wait for scale-down
kubectl rollout status deployment/twitch-listener -n allchat
```

### Step 3.2: Enable Filtering
```bash
# Enable feature flag
kubectl set env deployment/twitch-listener ENABLE_COORDINATOR_FILTERING=true -n allchat

# Wait for pod restart
kubectl rollout status deployment/twitch-listener -n allchat

# Wait for assignment refresh (60 seconds)
sleep 60
```

### Step 3.3: Verify Filtering Enabled
```bash
# Check logs for coverage verification
kubectl logs -n allchat -l app=twitch-listener --tail=100 | grep "Coverage verified"
# Expected: "Coverage verified - filtering enabled"

# Verify channel count matches database
kubectl logs -n allchat -l app=twitch-listener | grep "total_active"
# Should still show 74 channels (with 1 pod, it gets all assignments)

# Check for coverage gaps
kubectl logs -n allchat -l app=twitch-listener | grep "Coverage verification FAILED"
# Expected: NO OUTPUT (no gaps detected)
```

### Step 3.4: Monitor Canary (24 hours)
```bash
# Monitor coverage gaps
watch -n 60 "curl -s http://prometheus:9090/api/v1/query?query=shard_coverage_gaps_detected_total | jq"

# Monitor message rate (should remain constant)
watch -n 60 "curl -s http://prometheus:9090/api/v1/query?query=rate(chat_messages_received_total[5m]) | jq"

# Check coordinator assignments
curl http://source-manager:8088/assignments?pod_id=<pod-name>
```

### Step 3.5: Canary Success Criteria (24 hours)
- [ ] Coverage gaps = 0 for 24 hours
- [ ] Message rate unchanged from baseline
- [ ] All 74 channels remain active
- [ ] No coverage verification failures in logs
- [ ] Coordinator assignments match database source count

### Step 3.6: Emergency Rollback (if needed)
```bash
# If ANY coverage gaps detected:
kubectl set env deployment/twitch-listener ENABLE_COORDINATOR_FILTERING=false -n allchat

# Verify all channels active after rollback
kubectl logs -n allchat -l app=twitch-listener | grep "total_active"
```

**✅ Phase 3 Complete** when canary runs successfully for 24 hours

---

## Phase 4: Full Rollout - 72 hours

**Objective**: Enable filtering across all pods, verify even distribution

**Prerequisites**:
- [ ] Phase 3 complete and verified for 24 hours
- [ ] Canary showed zero coverage gaps
- [ ] **User approval obtained to proceed**

### Step 4.1: Scale to Production Replicas
```bash
# Scale back to normal (typically 2 replicas)
kubectl scale deployment/twitch-listener --replicas=2 -n allchat

# Wait for scale-up
kubectl rollout status deployment/twitch-listener -n allchat

# Wait for coordinator to redistribute (60s)
sleep 60
```

### Step 4.2: Verify Distribution
```bash
# Get pod names
POD1=$(kubectl get pods -n allchat -l app=twitch-listener -o jsonpath='{.items[0].metadata.name}')
POD2=$(kubectl get pods -n allchat -l app=twitch-listener -o jsonpath='{.items[1].metadata.name}')

# Check assigned channels per pod
kubectl logs -n allchat "$POD1" | grep "assigned_channels"
kubectl logs -n allchat "$POD2" | grep "assigned_channels"
# Expected: Roughly equal split (e.g., 37 + 37 = 74)

# Verify coverage on each pod
kubectl logs -n allchat "$POD1" | grep "Coverage verified"
kubectl logs -n allchat "$POD2" | grep "Coverage verified"
# Expected: Both show "Coverage verified - filtering enabled"
```

### Step 4.3: Monitor Distribution (72 hours)
```bash
# Monitor per-pod channel count
watch -n 60 "curl -s http://prometheus:9090/api/v1/query?query=shard_channel_count | jq"

# Monitor coverage gaps (should be 0)
watch -n 300 "curl -s http://prometheus:9090/api/v1/query?query=shard_coverage_gaps_detected_total | jq"

# Monitor message rate
watch -n 60 "curl -s http://prometheus:9090/api/v1/query?query=rate(chat_messages_received_total[5m]) | jq"
```

### Step 4.4: Full Rollout Success Criteria (72 hours)
- [ ] Coverage gaps = 0 for 72 hours
- [ ] Even distribution across pods (±10% variance acceptable)
- [ ] No message loss (message rate unchanged)
- [ ] No coverage verification failures
- [ ] Total channels = database source count (no duplicates, no gaps)
- [ ] HPA scale events handled correctly

### Step 4.5: Test HPA Scale Events
```bash
# Test scale-up
kubectl scale deployment/twitch-listener --replicas=3 -n allchat
sleep 60
# Verify channels redistributed evenly (~25 each)

# Test scale-down
kubectl scale deployment/twitch-listener --replicas=2 -n allchat
sleep 60
# Verify channels redistributed evenly (~37 each)
```

**✅ Phase 4 Complete** when all checks pass for 72 hours

---

## Phase 5: Apply to Other Listeners (1-2 weeks)

**Objective**: Roll out filtering to kick-listener, tiktok-listener, youtube-listener

**Prerequisites**:
- [ ] Phase 4 complete for twitch-listener
- [ ] 72-hour stability demonstrated
- [ ] No issues observed in production

### Listeners to Update
1. **kick-listener** (same pattern as twitch-listener)
2. **tiktok-listener** (same pattern as twitch-listener)
3. **youtube-listener** (verify if already uses UUIDs)

### Process per Listener
Repeat Phases 1-4 for each listener with same verification steps.

---

## Alerts to Configure

### Critical (Page On-Call)
```yaml
- alert: CoverageGapDetected
  expr: shard_unassigned_sources > 0
  for: 5m
  severity: critical
  annotations:
    summary: "Sources without coordinator assignments"
    description: "{{ $value }} sources lack assignments"
    runbook: "Check coordinator logs, verify all pods healthy, check database source count"

- alert: FilteringDisabledAutomatically
  expr: increase(shard_coverage_gaps_detected_total[5m]) > 0
  for: 1m
  severity: critical
  annotations:
    summary: "Filtering auto-disabled due to coverage gaps"
    description: "Coverage verification failed, system reverted to full coverage"
```

### Warning (Notify)
```yaml
- alert: GhostPodsPresent
  expr: rate(shard_ghost_pods_detected_total[10m]) > 0
  for: 15m
  severity: warning
  annotations:
    summary: "Ghost pods in heartbeat registry"
    description: "Pods with heartbeats but not in Kubernetes API"

- alert: HeartbeatCleanupSlow
  expr: increase(shard_stale_heartbeats_removed_total[5m]) > 10
  for: 15m
  severity: warning
  annotations:
    summary: "High stale heartbeat cleanup rate"
    description: "More than 10 stale heartbeats removed in 5 minutes"
```

---

## Rollback Procedures

### Automatic Rollback
System will automatically fallback if:
- Coverage verification detects gaps
- Filtering disabled in-process
- All channels processed (100% duplication)
- Alert fires for investigation

### Manual Rollback: Disable Filtering
```bash
# Instant disable (< 30 seconds)
kubectl set env deployment/twitch-listener ENABLE_COORDINATOR_FILTERING=false -n allchat

# Verify all channels active
kubectl logs -n allchat -l app=twitch-listener | grep "total_active"
```

### Full Revert: Code Rollback
```bash
# Revert commit
git revert f88ebef
git push origin main

# Wait for CI/CD to rebuild and deploy
# Or manually rollback deployment:
kubectl rollout undo deployment/source-manager -n allchat
kubectl rollout undo deployment/twitch-listener -n allchat
```

---

## Success Criteria Summary

### Phase 1 (Heartbeat Cleanup)
✅ Ghost pods cleared within 15s
✅ Metrics showing zero ghost pods for 24 hours

### Phase 2 (Coverage Verification)
✅ Coverage gaps = 0 for 48 hours
✅ All sources have assignments

### Phase 3 (Canary)
✅ Single pod filtering successfully for 24 hours
✅ Zero message loss

### Phase 4 (Production)
✅ All pods filtering for 72 hours
✅ Even distribution
✅ Zero coverage gaps

### Phase 5 (All Listeners)
✅ All listener services using accurate filtering
✅ Resource usage optimized
✅ Monitoring and alerts operational

---

## Timeline

| Phase | Duration | Start After |
|-------|----------|-------------|
| Phase 1 | 24 hours | Immediate |
| Phase 2 | 48 hours | Phase 1 complete |
| Phase 3 | 24 hours | Phase 2 complete + user approval |
| Phase 4 | 72 hours | Phase 3 complete + user approval |
| Phase 5 | 1-2 weeks | Phase 4 complete |

**Total Time to Full Production**: ~6-7 days minimum

---

## Notes

- Each phase requires explicit completion before proceeding
- User approval required before Phase 3 and Phase 4
- Emergency rollback available at any time (< 30 seconds)
- Automatic fallback provides zero-downtime safety
- All verification can be automated with scripts
