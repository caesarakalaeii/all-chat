# Testing YouTube Listener Replica Quota Optimization

## Overview
This document describes how to verify that the global sync leader election prevents quota waste when running multiple YouTube Listener replicas.

## Problem
When running 3 replicas of the YouTube Listener, all replicas independently perform expensive stream discovery API calls (100 quota units per search). This causes 3x quota waste (300 units vs 100 units every 30 seconds).

## Solution
Global sync leader election ensures only ONE replica performs stream discovery at any time, reducing quota usage by ~66% with 3 replicas.

## Testing Procedure

### 1. Deploy Multiple Replicas

Scale the YouTube Listener deployment to 3 replicas:

```bash
kubectl scale deployment youtube-listener -n allchat --replicas=3
```

Verify all 3 pods are running:

```bash
kubectl get pods -n allchat -l app=youtube-listener
```

Expected output:
```
NAME                                READY   STATUS    RESTARTS   AGE
youtube-listener-7d8f9c5b4d-abc12   1/1     Running   0          30s
youtube-listener-7d8f9c5b4d-def34   1/1     Running   0          30s
youtube-listener-7d8f9c5b4d-ghi56   1/1     Running   0          30s
```

### 2. Check Logs for Leader Election

Monitor logs from all 3 replicas:

```bash
# Terminal 1
kubectl logs -n allchat -l app=youtube-listener --tail=100 -f | grep "global sync"
```

**Expected Behavior:**
- Only ONE pod logs: `"Global sync leader, performing periodic sync"`
- The other TWO pods log: `"Not global sync leader, skipping periodic sync"`

Example:
```
youtube-listener-7d8f9c5b4d-abc12: {"level":"debug","msg":"Global sync leader, performing periodic sync"}
youtube-listener-7d8f9c5b4d-def34: {"level":"debug","msg":"Not global sync leader, skipping periodic sync"}
youtube-listener-7d8f9c5b4d-ghi56: {"level":"debug","msg":"Not global sync leader, skipping periodic sync"}
```

### 3. Verify Leadership in Redis

Check the Redis lock for global sync leadership:

```bash
# Connect to Redis pod
kubectl exec -n allchat -it redis-0 -- redis-cli

# Check global sync leader lock
GET leader:youtube:global-sync
```

**Expected Output:**
- A UUID representing the instance ID of the current leader
- TTL should be ~10 seconds

```
# Check TTL
TTL leader:youtube:global-sync
```

### 4. Monitor Quota Usage

Check the YouTube Listener status endpoint to verify quota usage:

```bash
# Port forward to one of the pods
kubectl port-forward -n allchat svc/youtube-listener 8086:8086

# Check status
curl http://localhost:8086/status | jq '.quota'
```

**Expected Behavior:**
- Quota usage should show a steady, controlled increase
- With 3 replicas and global sync election: ~100 units per sync cycle
- Without global sync election: ~300 units per sync cycle (3x waste)

### 5. Test Leadership Failover

Kill the current leader pod and verify another replica takes over:

```bash
# Identify current leader from logs
LEADER_POD=$(kubectl logs -n allchat -l app=youtube-listener --tail=50 | grep "Global sync leader" | head -1 | awk '{print $1}')

# Delete the leader pod
kubectl delete pod -n allchat $LEADER_POD

# Watch logs to see leadership transfer
kubectl logs -n allchat -l app=youtube-listener --tail=100 -f | grep "global sync"
```

**Expected Behavior:**
- Within 10 seconds (lock TTL), another replica should acquire leadership
- New leader starts logging: `"Global sync leader, performing periodic sync"`
- The deleted pod is recreated by Kubernetes and becomes a follower

### 6. Verify No Quota Waste

Compare quota usage graphs before and after the fix:

**Before Fix (3 replicas without global sync election):**
- 3 replicas × 100 units/search = 300 units per sync
- ~1,080 units per hour (every 30s)
- Graph shows sharp spikes every 30 seconds

**After Fix (3 replicas with global sync election):**
- 1 replica × 100 units/search = 100 units per sync
- ~360 units per hour (every 30s)
- Graph shows controlled, steady increase
- **66% quota savings**

## Troubleshooting

### All Replicas Claim Leadership
**Symptom:** All 3 pods log "Global sync leader"

**Cause:** Source Manager is not running or Redis is not accessible

**Solution:**
```bash
# Check Source Manager is running
kubectl get pods -n allchat -l app=source-manager

# Check Redis is accessible
kubectl get pods -n allchat -l app=redis

# Verify SOURCE_MANAGER_URL and SOURCE_MANAGER_SECRET are set
kubectl get configmap -n allchat allchat-config -o yaml | grep SOURCE_MANAGER
kubectl get secret -n allchat allchat-secrets -o yaml
```

### No Replica Claims Leadership
**Symptom:** All 3 pods log "Not global sync leader" and no syncing happens

**Cause:** Unable to acquire Redis lock (Source Manager issues)

**Solution:**
```bash
# Check Source Manager logs
kubectl logs -n allchat -l app=source-manager

# Manually clear the lock in Redis if stuck
kubectl exec -n allchat -it redis-0 -- redis-cli DEL leader:youtube:global-sync
```

### Leadership Changes Too Frequently
**Symptom:** Leadership transfers every few seconds

**Cause:** Lock TTL too short or heartbeat failures

**Solution:**
- Check Source Manager logs for renewal errors
- Verify network connectivity between YouTube Listener and Source Manager
- Default TTL is 10 seconds with 5-second renewal - should be stable

## Success Criteria

✅ **Pass**: Only ONE replica performs stream discovery at any time
✅ **Pass**: Leadership transfers when the leader pod is deleted
✅ **Pass**: Quota usage reduced by ~66% compared to before the fix
✅ **Pass**: All replicas can become leader (no permanent follower)
✅ **Pass**: No duplicate API calls observed in YouTube API quota graphs

## Cleanup

Scale back to 1 replica if testing is complete:

```bash
kubectl scale deployment youtube-listener -n allchat --replicas=1
```
