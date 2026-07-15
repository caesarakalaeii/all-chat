# TikTok Listener Validation Guide

This guide helps validate all the changes made to fix TikTok stream detection issues.

## Prerequisites

> **Note:** `tiktok-listener` is not in `deployments/docker-compose.yml`. To validate it locally,
> either build and run the container manually from `services/tiktok-listener/Dockerfile`, or run
> it directly with `npm` (`cd services/tiktok-listener && npm install && npm run dev`).
> In Kubernetes it runs as the `tiktok-listener` deployment.

```bash
# Run TikTok listener directly (one option)
cd services/tiktok-listener && npm install && npm run dev

# Or, if running via a manually-built docker image:
docker logs <your-tiktok-container-name>

# Check logs for backoff configuration
docker logs <your-tiktok-container-name> | grep -i "backoff"
```

## 1. Verify Reduced Backoff Parameters

### Check Initialization
> **Note:** `tiktok-listener` in the `docker` commands below is a placeholder (tiktok-listener is
> not in docker-compose; substitute your actual container/binary name, or read the logs of whatever
> you started in Prerequisites).

```bash
# Should see faster backoff parameters in logs/behavior
docker logs tiktok-listener -f
```

**Expected Behavior:**
- Base offline backoff: **20 seconds** (was 60s)
- Max offline backoff: **3 minutes** (was 10min)
- Error backoff: **1 second** (was 2s)
- Max error backoff: **1 minute** (was 5min)

### Progression Test
When a channel is offline, backoff should progress:
- 1st check: 20s
- 2nd check: 40s
- 3rd check: 80s (1min 20s)
- 4th check: 160s (2min 40s)
- 5th+ check: 180s (3min - capped)

## 2. Verify Dynamic Status Cache TTL

### Monitor Cache Behavior
```bash
# Watch status check logs
docker logs -f tiktok-listener | grep "status check"
```

**Expected:**
- Live results cached: **5 seconds** (stream could end)
- Offline results cached: **15 seconds** (less critical)
- Error results cached: **2 seconds** (retry quickly)

### Check Cache Stats
> **Note:** There is no `/stats` endpoint on tiktok-listener, and the cache TTL is not exposed
> over HTTP. The dynamic cache TTL is only observable in the service's debug logs. Look for the
> `Using cached live status (dynamic TTL)` log line, which includes a `ttl_ms` field
> (5000 for live, 15000 for offline, 2000 for error results).

## 3. Verify State Inspection Endpoints

### Get All Channel States
```bash
curl http://localhost:8089/api/channels | jq
```

**Expected Response:**
```json
{
  "channels": [
    {
      "username": "testuser",
      "backoffState": {
        "consecutiveOfflineChecks": 2,
        "consecutiveErrors": 0,
        "currentBackoffMs": 40000,
        "currentBackoffMinutes": 1,
        "nextCheckTime": "2026-02-06T...",
        "lastCheckTime": "2026-02-06T...",
        "lastSeenLive": "2026-02-05T..." or null
      },
      "hasActiveConnection": false,
      "isConnecting": false,
      "isInPoller": true,
      "riskLevel": "low",
      "recommendedAction": ""
    }
  ],
  "summary": {
    "total": 10,
    "active": 2,
    "in_backoff": 8
  }
}
```

### Get Specific Channel
```bash
USERNAME="testuser"
curl "http://localhost:8089/api/channel?username=${USERNAME}" | jq
```

## 4. Verify Manual Control Endpoints

### Force Retry
```bash
# Force immediate retry for a username
USERNAME="testuser"
curl -X POST http://localhost:8089/api/retry \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${USERNAME}\"}"
```

**Expected Response:**
```json
{
  "message": "Retry triggered successfully",
  "username": "testuser",
  "timestamp": "2026-02-06T..."
}
```

**Expected Log:**
```
Manual force retry requested
  username: testuser
  action: admin_force_retry
```

### Reset Backoff (Single Username)
```bash
curl -X POST http://localhost:8089/api/reset-backoff \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"${USERNAME}\"}"
```

### Reset All Backoff (Emergency)
```bash
# Reset backoff for all usernames
curl -X POST http://localhost:8089/api/reset-backoff \
  -H "Content-Type: application/json" \
  -d "{}"
```

**Expected Response:**
```json
{
  "message": "All backoff reset successfully",
  "channels_reset": 8,
  "timestamp": "2026-02-06T..."
}
```

## 5. Verify Automatic Stuck State Recovery

### Monitor Recovery Logs
```bash
# Watch for auto-recovery (runs every polling cycle)
docker logs -f tiktok-listener | grep -i "stuck\|recovery"
```

**Expected Output (when recovery happens):**
```
Detected stuck channel in max backoff, forcing recovery
  username: testuser
  current_backoff_ms: 180000
  time_since_last_check_minutes: 6
  consecutive_offline: 5
  action: auto_recovery

Auto-recovery cycle complete
  stuck_channels_recovered: 1
  usernames: ["testuser"]
```

### Conditions for Auto-Recovery
A channel is considered "stuck" and auto-recovered when:
- Backoff is at max (180000ms = 3 minutes) AND
- Last check was >5 minutes ago

**Action Taken:**
- Backoff state removed (reset to base 20s)
- Will be checked on next polling cycle

## 6. Verify Prometheus Metrics

### Backoff Metrics
```bash
# Current backoff intervals per username (in milliseconds)
curl http://localhost:8089/metrics | grep tiktok_backoff_current_interval_ms

# Usernames stuck in >5min backoff (should be 0 or very low)
curl http://localhost:8089/metrics | grep tiktok_backoff_usernames_stuck

# Detections skipped by reason
curl http://localhost:8089/metrics | grep tiktok_detection_skipped_total
```

### At-Risk Usernames
```bash
# Usernames with long backoff by risk level
curl http://localhost:8089/metrics | grep tiktok_usernames_at_risk
```

### Auto-Recovery Metrics
```bash
# Auto-recovery events
curl http://localhost:8089/metrics | grep tiktok_auto_recovery_total
```

## 7. Integration Testing Scenarios

### Scenario 1: Username Goes Live
1. **Setup:** Username offline, backoff at 3min (max)
2. **Action:** Username goes live on TikTok
3. **Expected:**
   - Detection within 3-4 minutes (faster than old 10min)
   - Backoff resets when connection established
   - Stream connects successfully

### Scenario 2: Rapid Error Recovery
1. **Setup:** Connection error occurs
2. **Expected:**
   - Error backoff: 1s, 2s, 4s, 8s, 16s, 32s, 60s (max)
   - Much faster recovery than old (2s → 5min)
   - Successful retry within 1-2 minutes typically

### Scenario 3: Offline Backoff Progression
1. **Setup:** Username not streaming
2. **Expected Progression:**
   - Check 1: 20s wait
   - Check 2: 40s wait
   - Check 3: 80s wait
   - Check 4: 160s wait
   - Check 5+: 180s wait (capped at 3min)

### Scenario 4: Auto-Recovery of Stuck Username
1. **Setup:**
   - Username in max backoff (3min)
   - Last check was >5 minutes ago
2. **Expected:**
   - Auto-recovery detects stuck state
   - Backoff reset to 20s (base)
   - Immediate retry in next cycle
   - Log shows recovery action

### Scenario 5: Cache TTL Behavior
1. **Test Live Result:**
   - Check username status (live)
   - Check again within 5s → cached
   - Check after 5s → fresh check

2. **Test Offline Result:**
   - Check username status (offline)
   - Check again within 15s → cached
   - Check after 15s → fresh check

3. **Test Error Result:**
   - API error occurs
   - Check again within 2s → cached
   - Check after 2s → fresh check

## 8. Performance Validation

### Detection Speed Comparison

| Scenario | Old Behavior | New Behavior | Improvement |
|----------|--------------|--------------|-------------|
| Max offline backoff | 10 minutes | 3 minutes | **70% faster** |
| Base offline backoff | 60 seconds | 20 seconds | **67% faster** |
| Max error backoff | 5 minutes | 1 minute | **80% faster** |
| Stuck recovery | Manual only | Auto (5min check) | **Automatic** |

### Monitor Detection Times
```bash
# Watch channel states and backoff intervals
watch -n 5 'curl -s http://localhost:8089/api/channels | jq ".summary"'

# Expected: Most channels in 20s-3min backoff range
```

## 9. Failure Scenarios

### Test Network Errors
```bash
# Simulate by forcing retry during network issue
# Errors should backoff exponentially: 1s, 2s, 4s, 8s, 16s, 32s, 60s
# Much faster than old: 2s, 4s, 8s, ... 5min

# Monitor logs
docker logs -f tiktok-listener | grep -i "error"
```

### Test Rapid State Changes
```bash
# Username goes live → offline → live quickly
# Should handle gracefully with dynamic cache TTL
# Live cached 5s, offline cached 15s
```

## 10. Rollback Testing

If issues are found:

```bash
# Environment variables to revert to old behavior
export TIKTOK_BASE_OFFLINE_BACKOFF_MS=60000  # 1 minute
export TIKTOK_MAX_OFFLINE_BACKOFF_MS=600000  # 10 minutes
export TIKTOK_ERROR_BACKOFF_MS=2000          # 2 seconds
export TIKTOK_MAX_ERROR_BACKOFF_MS=300000    # 5 minutes

# Restart service
docker restart tiktok-listener

# Or full rollback
git revert <commit-sha>
kubectl rollout undo deployment/tiktok-listener
```

## Success Criteria

✅ **Reduced Backoff:**
- Base: 20s (not 60s) ✓
- Max: 3min (not 10min) ✓
- Progression: 20s → 40s → 80s → 160s → 180s ✓

✅ **Dynamic Cache TTL:**
- Live: 5s cache ✓
- Offline: 15s cache ✓
- Error: 2s cache ✓
- Cache stats show distribution ✓

✅ **Manual Control:**
- Can view channel/all states via API ✓
- Can force retry specific username ✓
- Can reset backoff (single or all) ✓
- Operations logged with action tag ✓

✅ **Auto-Recovery:**
- Detects max backoff >5min ✓
- Auto-resets to base (20s) ✓
- Runs every polling cycle ✓
- Logs recovery events ✓

✅ **Metrics:**
- Backoff intervals tracked per username ✓
- Stuck usernames counted ✓
- At-risk usernames by level ✓
- Auto-recovery events counted ✓

✅ **No Missed Streams:**
- Usernames detected within 3-4 minutes (not 10+) ✓
- Error recovery within 1-2 minutes (not 5+) ✓
- Stuck channels auto-recover ✓

## Quick Smoke Test

Run this to quickly verify all components:

```bash
#!/bin/bash
echo "=== TikTok Listener Validation ==="

echo "1. Check service status"
docker ps | grep tiktok-listener

echo "2. Get all channels"
curl -s http://localhost:8089/api/channels | jq '.summary'

echo "3. Check metrics endpoint"
curl -s http://localhost:8089/metrics | grep -E "tiktok_backoff|tiktok_auto_recovery" | head -5

echo "4. Test force retry"
curl -s -X POST http://localhost:8089/api/retry \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser"}' | jq

echo "5. Check recent logs"
docker logs tiktok-listener --tail 20

echo "=== Validation Complete ==="
```
