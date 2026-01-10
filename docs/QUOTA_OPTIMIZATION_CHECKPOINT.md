# YouTube Quota Optimization - Checkpoint

**Date**: 2026-01-10
**Status**: Changes deployed, monitoring required
**Commit**: `9342b24` - fix(youtube): eliminate quota waste through health checker fix and persistent backoff

---

## Problem Summary

YouTube API quota was burning at an alarming rate, reaching **9,503 / 10,000 units (95% - CRITICAL state)** within a day.

### Root Causes Identified

1. **API Gateway Health Checker Event Spam** (CRITICAL)
   - Health checker used unmaintained Redis SET `overlay:connected`
   - Manager used individual TTL keys `overlay:connected:{id}`
   - Mismatch caused false positives every 5 minutes
   - Each false positive triggered YouTube listener sync (100+ units)
   - **Impact**: ~20,000 wasted units/day

2. **Non-Persistent Backoff State** (HIGH)
   - Backoff state stored in-memory, lost on pod restarts
   - Every restart/deploy/scale reset backoff → all channels searched again
   - No negative caching for offline channels
   - **Impact**: 300 units per restart + 180 units/day from repeats

3. **567-Unit Spike Event**
   - 5-6 concurrent `search.list` API calls (100 units each)
   - Symptom of the above issues, not root cause

---

## Changes Deployed

### Priority 1: API Gateway Health Checker Fix

**Files Modified**:
- `services/api-gateway/websocket/health.go` (lines 87-177)
- `services/api-gateway/cmd/main.go` (added migration function)

**Changes**:
- Replaced `SMembers("overlay:connected")` with pipelined EXISTS checks on `overlay:connected:{id}` keys
- Removed stale detection logic (TTL handles cleanup automatically)
- Added one-time migration to delete legacy SET

**Expected Result**: Zero "Recovered missing overlay" messages during normal operation

---

### Priority 2: Persistent Backoff + Negative Caching

**Files Modified/Created**:
- `services/youtube-listener/streams/backoff_store.go` (NEW - 164 lines)
- `services/youtube-listener/streams/manager.go` (replaced in-memory backoff with persistent store)

**Changes**:
- Created `BackoffStore` with Redis-backed persistence
- Backoff state survives pod restarts (24h TTL)
- Negative cache with tiered TTL (5-30min) for offline channels
- Removed in-memory maps: `channelLastCheck`, `channelBackoff`

**Redis Keys**:
- `youtube:backoff:{channel_id}` - Persistent backoff state (24h TTL)
- `youtube:negative:{channel_id}` - Negative cache (5-30min TTL based on consecutive offline)

**Expected Result**: Backoff persists across restarts, offline channels cached

---

## Monitoring Plan - Tomorrow (2026-01-11)

### Priority Checks (5 minutes)

#### 1. Verify Deployment Success
```bash
# Check pod status
kubectl get pods -n allchat | grep -E "api-gateway|youtube-listener"

# Should see pods running with uptime > 12 hours
```

#### 2. Check Quota Status (CRITICAL)
```bash
# Get current quota usage
kubectl exec -n allchat $(kubectl get pods -n allchat -l app=youtube-listener -o jsonpath='{.items[0].metadata.name}') -- \
  wget -qO- http://localhost:8086/quota/status | jq '.global'

# Expected output:
# {
#   "state": "HEALTHY",          # Should be HEALTHY (not DEGRADED or CRITICAL)
#   "used": 1500-2500,           # Should be dramatically lower than 9,500
#   "limit": 10000,
#   "percentage": 15-25,         # Should be well below 70%
#   "remaining": 7500-8500
# }
```

#### 3. Verify Health Checker Fix
```bash
# Should return ZERO results (no false positives):
kubectl logs -n allchat -l app=api-gateway --since=12h | grep "Recovered missing overlay"

# If you see any "Recovered" messages, the fix didn't work
```

#### 4. Check Backoff Store Working
```bash
# Should see backoff keys (means persistence is working):
kubectl exec -n allchat redis-0 -- redis-cli KEYS "youtube:backoff:*"

# Should see negative cache keys (means offline caching is working):
kubectl exec -n allchat redis-0 -- redis-cli KEYS "youtube:negative:*"

# Check logs for backoff activity:
kubectl logs -n allchat -l app=youtube-listener --since=1h | grep -E "backoff|negative cache" | tail -20
```

---

## Success Criteria

### ✅ PASS Criteria (Fix Working)
- [ ] Quota state: HEALTHY (0-70%) or DEGRADED (70-85%)
- [ ] Daily quota usage: **2,000-3,000 units** (down from 9,500+)
- [ ] Zero "Recovered missing overlay" messages in API Gateway logs
- [ ] Backoff keys exist in Redis: `youtube:backoff:*`
- [ ] Negative cache keys exist: `youtube:negative:*`
- [ ] No CRITICAL quota warnings

### ❌ FAIL Criteria (Needs Investigation)
- [ ] Quota state: CRITICAL (95%+) or EXHAUSTED (100%)
- [ ] Daily quota usage: **>5,000 units**
- [ ] "Recovered missing overlay" messages still appearing
- [ ] No Redis backoff keys (persistence not working)
- [ ] Pods crash-looping

---

## Expected Timeline

| Time | Expected State | Check |
|------|---------------|-------|
| **T+0 (Deploy)** | Pods restarting | Migration runs, removes legacy SET |
| **T+10min** | Stable | No more false positive events |
| **T+1hr** | Improving | Quota usage rate drops to ~100 units/hr |
| **T+12hr (Tomorrow AM)** | Stable | Daily usage on track for 2,000-3,000 units |
| **T+24hr** | Confirmed | Total usage <3,000 units, state HEALTHY |

---

## Rollback Plan (If Needed)

### Emergency Rollback
```bash
# Revert both services to previous version
kubectl rollout undo deployment/api-gateway -n allchat
kubectl rollout undo deployment/youtube-listener -n allchat
```

### Temporary Mitigation (If rollback not desired)
```bash
# Disable health checker temporarily (reduces events)
kubectl set env deployment/api-gateway WEBSOCKET_HEALTH_CHECK_INTERVAL_SECONDS=3600 -n allchat

# Check logs for root cause
kubectl logs -n allchat -l app=youtube-listener --tail=500 | grep -i error
kubectl logs -n allchat -l app=api-gateway --tail=500 | grep -i error
```

---

## Investigation Commands (If Issues)

### Deep Dive Quota Analysis
```bash
# Get detailed quota breakdown
kubectl exec -n allchat $(kubectl get pods -n allchat -l app=youtube-listener -o jsonpath='{.items[0].metadata.name}') -- \
  wget -qO- http://localhost:8086/quota/channels | jq '.channels[:10]'

# Check for search.list calls (expensive - 100 units each)
kubectl logs -n allchat -l app=youtube-listener --since=2h | grep "search.list" | wc -l

# Check for API errors
kubectl logs -n allchat -l app=youtube-listener --since=2h | grep -i "quota\|exceeded\|error" | tail -50
```

### Redis State Inspection
```bash
# Check all YouTube-related keys
kubectl exec -n allchat redis-0 -- redis-cli KEYS "youtube:*"

# Inspect a backoff state
kubectl exec -n allchat redis-0 -- redis-cli GET "youtube:backoff:UC..."

# Check TTLs
kubectl exec -n allchat redis-0 -- redis-cli TTL "youtube:backoff:UC..."
```

### Connection Event Analysis
```bash
# Check if overlay connection events are still spamming
kubectl logs -n allchat -l app=youtube-listener --since=30m | \
  grep "Received overlay connection event" | wc -l

# Should be minimal (only on actual connects/disconnects)
# If hundreds/thousands, health checker fix didn't work
```

---

## Next Steps (If Successful)

1. **Monitor for 48 hours** - Ensure stability
2. **Document savings** - Calculate actual quota reduction
3. **Update documentation** - Add to architecture docs
4. **Consider Priority 3** - Rate limiter (optional, only if spikes continue)

---

## Next Steps (If Failed)

1. **Rollback immediately** if quota hits EXHAUSTED
2. **Collect logs** from both services for analysis
3. **Check Redis connectivity** - Backoff store requires Redis
4. **Verify environment** - Health check interval, quota limits
5. **Contact team** - May need deeper investigation

---

## Key Files Reference

### Source Code
- `services/api-gateway/websocket/health.go` - Health checker logic
- `services/api-gateway/cmd/main.go` - Migration function
- `services/youtube-listener/streams/backoff_store.go` - Persistent backoff
- `services/youtube-listener/streams/manager.go` - Backoff integration

### Documentation
- `/home/caesar/.claude/plans/merry-wibbling-muffin.md` - Full implementation plan
- `GETTING_STARTED.md` - Architecture overview
- `services/youtube-listener/README.md` - Quota tracking details

---

## Contact Info

**Implementation**: Claude Code (commit 9342b24)
**Date**: 2026-01-10
**Expected Resolution**: Within 24 hours
**Status**: ⏳ Awaiting monitoring results

---

## Quick Health Check (30 seconds)

```bash
# One-liner to check if fix is working:
echo "=== QUOTA STATUS ===" && \
kubectl exec -n allchat $(kubectl get pods -n allchat -l app=youtube-listener -o jsonpath='{.items[0].metadata.name}') -- \
  wget -qO- http://localhost:8086/quota/status 2>/dev/null | jq -r '.global | "State: \(.state), Used: \(.used)/\(.limit) (\(.percentage)%)"' && \
echo "" && \
echo "=== FALSE POSITIVES (should be 0) ===" && \
kubectl logs -n allchat -l app=api-gateway --since=12h 2>/dev/null | grep -c "Recovered missing overlay" && \
echo "" && \
echo "=== BACKOFF KEYS ===" && \
kubectl exec -n allchat redis-0 -- redis-cli KEYS "youtube:backoff:*" 2>/dev/null | wc -l | awk '{print $1 " channels with backoff state"}'
```

**Expected Output**:
```
=== QUOTA STATUS ===
State: HEALTHY, Used: 2000-3000/10000 (20-30%)

=== FALSE POSITIVES (should be 0) ===
0

=== BACKOFF KEYS ===
3-5 channels with backoff state
```

---

**END OF CHECKPOINT** - Review tomorrow morning (2026-01-11)
