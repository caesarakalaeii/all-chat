# Quick Reference: Debug YouTube Quota Issues

**Time Estimate**: 30-60 minutes | **Difficulty**: ⭐⭐ Easy-Moderate

**Goal**: Diagnose and resolve YouTube API quota exhaustion or tracking issues in the YouTube Listener service.

---

## Quick Diagnosis

### Step 1: Check Current Quota Status

```bash
# From local development
curl http://localhost:8086/quota/status | jq .

# From Kubernetes pod
kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .
```

**Expected Response:**
```json
{
  "global": {
    "state": "HEALTHY",            // HEALTHY | DEGRADED | CRITICAL | EXHAUSTED | DEPLETED
    "used": 2500,                  // Units consumed today
    "reserved": 15,                // In-flight API calls
    "remaining": 7485,             // Available units
    "percentage": 25.15,           // Quota usage percentage
    "resets_at": "2026-01-29T00:00:00-08:00",  // Midnight Pacific Time
    "polling_multiplier": 1.0      // Adaptive slowdown (1.0 = normal)
  },
  "channels": [
    {
      "channel_id": "UCxxxxxx",
      "used": 500,
      "limit": 2000,
      "percentage": 25.0,
      "state": "HEALTHY"
    }
  ]
}
```

### Step 2: Identify State

| State | Percentage | Immediate Action |
|-------|------------|------------------|
| **HEALTHY** | 0-70% | ✅ No action needed |
| **DEGRADED** | 70-85% | ⚠️ Monitor closely, reduce non-critical polling |
| **CRITICAL** | 85-95% | 🚨 Only high-priority channels polled |
| **EXHAUSTED** | 95-100% | ⛔ New discoveries stopped, existing polling continues |
| **DEPLETED** | >100% | 🛑 ALL POLLING STOPPED, wait for reset |

---

## Common Issues & Solutions

### Issue 1: Quota State is EXHAUSTED or DEPLETED

**Symptom**: `/quota/status` shows `state: "EXHAUSTED"` or `state: "DEPLETED"`

**Root Cause**: Daily quota limit (10,000 units) reached or exceeded.

**Immediate Fix** (Wait for Reset):
1. **Check reset time**:
   ```bash
   curl http://localhost:8086/quota/status | jq .global.resets_at
   # Output: "2026-01-29T00:00:00-08:00"  (Midnight Pacific Time)
   ```

2. **Calculate time remaining**:
   - Quota resets at **00:00 PST** (or 00:00 PDT during daylight saving)
   - Use `date -u` to check current UTC time
   - PST = UTC-8, PDT = UTC-7

3. **Service behavior during exhaustion**:
   - **EXHAUSTED (95-100%)**: Existing streams continue polling, no new stream discovery
   - **DEPLETED (>100%)**: All polling stopped, service enters sleep mode
   - After midnight PT: Service automatically resumes normal operation

**Long-Term Fix** (Request Quota Increase):
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to: APIs & Services → Enabled APIs → YouTube Data API v3
3. Click "Quotas" tab
4. Find "Queries per day" quota
5. Click "Edit Quotas" → Request increase to **1,000,000 units/day**
6. Provide justification: "Multi-platform chat aggregation service for live streamers"

**File Reference**: `services/youtube-listener/quota/tracker.go:GetGlobalState()`

---

### Issue 2: Quota Tracking Drift

**Symptom**: Database quota usage doesn't match YouTube API Console

**Expected Accuracy**: With reserve-confirm-rollback pattern, drift should be <±5 units (99.95%+ accuracy)

**Check Database Tracking**:
```sql
-- Connect to database
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat

-- Check today's usage
SELECT date, units_used, units_reserved, units_limit,
       ROUND((units_used::numeric / units_limit) * 100, 2) as percentage
FROM youtube_quota_usage
WHERE date = CURRENT_DATE;

-- Expected output:
--    date    | units_used | units_reserved | units_limit | percentage
-- -----------+------------+----------------+-------------+------------
-- 2026-01-28 |       2500 |             15 |       10000 |      25.00
```

**Check for Stale Reservations**:
```sql
-- View current reservations
SELECT operation_type, units, created_at
FROM youtube_quota_reservations
WHERE date = CURRENT_DATE
ORDER BY created_at DESC
LIMIT 20;

-- Clean up stale reservations (>5 minutes old)
SELECT cleanup_stale_quota_reservations();

-- Expected: "Cleaned up N stale reservations" in logs
```

**Verify Reservation Age**:
- Reservations should be <30 seconds old (typical API call duration)
- If `units_reserved` is >50, check for:
  - Service crashes (reservations not confirmed/rolled back)
  - Network timeouts to YouTube API
  - Database connection issues

**Automatic Cleanup**:
- Runs every 60 seconds via background goroutine
- Rolls back reservations older than 5 minutes
- Logs: `"Cleaned up stale quota reservations" count=N`

**File References**:
- `services/youtube-listener/quota/tracker.go:ReserveQuota()`
- `services/youtube-listener/quota/tracker.go:ConfirmQuota()`
- `services/youtube-listener/quota/tracker.go:RollbackQuota()`
- `migrations/008_quota_reservations.sql` (reservation schema)

---

### Issue 3: Untracked API Calls from Other Services

**Symptom**: Quota usage higher than expected based on polling activity

**Check overlay-manager Integration**:

The `overlay-manager` service resolves YouTube channels when creating sources, consuming 100 units per `search.list` call. These calls must be tracked.

**Verify Integration**:
```bash
# Check overlay-manager logs for quota tracking
kubectl logs -n allchat deployment/overlay-manager | grep -i quota

# Expected: "Recording YouTube quota usage" units=100 operation="channel_resolve"
```

**Test Channel Resolution**:
```bash
# Trigger channel resolution (creates quota usage)
curl -X POST http://localhost:8081/api/v1/overlays/{overlay_id}/sources \
  -H "Content-Type: application/json" \
  -d '{
    "platform": "youtube",
    "channel_identifier": "@examplechannel"
  }'

# Check quota increased by 100 units
curl http://localhost:8086/quota/status | jq .global.used
```

**Configuration**:
- overlay-manager uses `YouTubeQuotaClient` HTTP client wrapper
- Calls `/quota/record` endpoint on youtube-listener
- Fail-open circuit breaker: quota tracking failure doesn't block users

**File Reference**:
- `services/overlay-manager/youtube/quota_client.go` (HTTP client wrapper)
- `services/youtube-listener/handlers/quota.go` (`POST /quota/record`)

---

### Issue 4: Polling Not Resuming After Reset

**Symptom**: Quota resets at midnight PT, but service doesn't resume polling

**Check Service Logs**:
```bash
kubectl logs -n allchat deployment/youtube-listener -f | grep -E "(quota|reset|resume)"

# Expected after midnight PT:
# "Quota reset detected" previous_date=2026-01-28 current_date=2026-01-29
# "Resuming quota-limited operations"
```

**Verify Quota State Transition**:
```bash
# Before midnight (DEPLETED)
curl http://localhost:8086/quota/status | jq .global.state
# Output: "DEPLETED"

# After midnight (HEALTHY)
curl http://localhost:8086/quota/status | jq .global.state
# Output: "HEALTHY"
```

**Manual Reset** (if automatic reset fails):
```sql
-- Reset today's quota usage (DANGER: Only use for debugging)
UPDATE youtube_quota_usage
SET units_used = 0, units_reserved = 0, updated_at = NOW()
WHERE date = CURRENT_DATE;

-- Verify reset
SELECT * FROM youtube_quota_usage WHERE date = CURRENT_DATE;
```

**Restart Service** (force quota state refresh):
```bash
kubectl rollout restart deployment/youtube-listener -n allchat
kubectl rollout status deployment/youtube-listener -n allchat
```

**File Reference**: `services/youtube-listener/quota/tracker.go:checkDailyReset()`

---

### Issue 5: High Reserved Units (Not Releasing)

**Symptom**: `units_reserved` in `/quota/status` is high (>50) and not decreasing

**Root Cause**: API calls timing out or service crashing before confirming/rolling back

**Diagnosis**:
```bash
# Check reserved units
curl http://localhost:8086/quota/status | jq .global.reserved

# View pending reservations
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat -c "
  SELECT operation_type, units, created_at,
         EXTRACT(EPOCH FROM (NOW() - created_at)) as age_seconds
  FROM youtube_quota_reservations
  WHERE date = CURRENT_DATE
  ORDER BY created_at DESC;
"
```

**Expected Age**:
- Most reservations: <30 seconds (normal API call)
- Stale threshold: 5 minutes (300 seconds)
- Auto-cleanup: Runs every 60 seconds

**Manual Cleanup**:
```sql
-- Force cleanup of all stale reservations
SELECT cleanup_stale_quota_reservations();

-- Expected output in logs:
-- INFO: Cleaned up stale quota reservations  count=N total_units=XYZ
```

**Check for Repeated Failures**:
```bash
# Check logs for timeout patterns
kubectl logs -n allchat deployment/youtube-listener --tail=500 | \
  grep -E "(timeout|context deadline|rollback)"

# Common issues:
# - "context deadline exceeded" → YouTube API slow/unreachable
# - "connection refused" → Network issues
# - "rollback failed" → Database connection problems
```

**File Reference**: `services/youtube-listener/quota/tracker.go:cleanupStaleReservations()`

---

## Quota State Machine

Understanding the 5 quota states and automatic behavior:

```
HEALTHY (0-70%)
│   Normal operation
│   Polling multiplier: 1.0x
│   All features enabled
│
├─> DEGRADED (70-85%)
│   Warning state
│   Polling multiplier: 1.5x (slow down)
│   Non-critical operations reduced
│
├─> CRITICAL (85-95%)
│   Critical state
│   Polling multiplier: 2.0x (significant slowdown)
│   Only high-priority channels polled
│
├─> EXHAUSTED (95-100%)
│   Quota nearly exhausted
│   New stream discovery STOPPED
│   Existing streams continue polling
│   Prevents exceeding 100% limit
│
└─> DEPLETED (>100%)
    Quota exceeded (should not happen with reserve-confirm-rollback)
    ALL POLLING STOPPED
    Service enters sleep mode
    Waits for midnight PT reset
```

**Automatic Behaviors**:
- **HEALTHY → DEGRADED**: Slow polling intervals by 50%
- **DEGRADED → CRITICAL**: Slow polling intervals by 100%, prioritize channels
- **CRITICAL → EXHAUSTED**: Stop new `search.list` calls (100 units each)
- **EXHAUSTED → DEPLETED**: Stop all API calls
- **Midnight PT**: Automatic reset to HEALTHY

**File Reference**: `services/youtube-listener/quota/states.go`

---

## Quota Cost Breakdown

**API Call Costs (per YouTube API v3 documentation):**

| Operation | Cost | Frequency | Daily Impact |
|-----------|------|-----------|--------------|
| `search.list` (find live streams) | 100 units | Per channel check | 100-400 units/channel/day |
| `videos.list` (stream details) | 1 unit | Per stream discovery | 1 unit/stream start |
| `liveChatMessages.list` (fetch messages) | 5 units | Every 2-5 seconds | ~2,000 units/stream/hour |

**Example Quota Usage**:
- **10 active streams** polled at 3-second intervals for 8 hours:
  - Messages: 10 × (8 hours × 1200 calls/hour) × 5 units = **480,000 units** ❌ EXCEEDS LIMIT
- **1 active stream** polled at 3-second intervals for 6 hours:
  - Messages: 1 × (6 hours × 1200 calls/hour) × 5 units = **36,000 units** ❌ EXCEEDS LIMIT
- **Realistic usage** with optimizations:
  - Stream discovery: 10 channels × 100 units = 1,000 units
  - Chat polling: 2-3 concurrent streams × 2-hour average × (varies by chat activity) = 1,000-2,000 units
  - **Total: 2,000-3,000 units/day** ✅ Within 10,000 limit

**Quota Waste Elimination** (~9,000 units/day savings):
- Stop on exhaustion (not retry every 5 min): 1,440 units/day saved
- Immediate cache clearing on stream end: 200+ units/event saved
- Enhanced status check (returns liveChatID): 2,880 units/day saved
- Smart disconnect (stops polling on last overlay): 75-90 units/event saved
- Connection batching (5-second debounce): 400+ units saved on rapid connects

---

## Diagnostic Commands Reference

### Check Service Health
```bash
# Kubernetes
kubectl get pods -n allchat -l app=youtube-listener
kubectl logs -n allchat deployment/youtube-listener --tail=100 -f

# Local
curl http://localhost:8086/health/live
curl http://localhost:8086/health/ready
curl http://localhost:8086/status
```

### Check Quota Status
```bash
# Current status (formatted)
kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .

# Quota history (last 7 days)
curl http://localhost:8086/quota/history?days=7 | jq .

# Per-channel quota
curl http://localhost:8086/quota/channels/UCxxxxxx | jq .
```

### Check Database State
```bash
# Access database
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat

# Check today's quota
SELECT * FROM youtube_quota_usage WHERE date = CURRENT_DATE;

# Check reservations
SELECT * FROM youtube_quota_reservations WHERE date = CURRENT_DATE;

# Check per-channel quota
SELECT * FROM youtube_channel_quota
WHERE channel_id = 'UCxxxxxx'
ORDER BY date DESC LIMIT 7;

# Clean up stale reservations
SELECT cleanup_stale_quota_reservations();
```

### Check Active Pollers
```bash
# Service status (includes active streams)
curl http://localhost:8086/status | jq .

# Expected:
# {
#   "status": "running",
#   "streams": {
#     "active_count": 2,
#     "streams": [...]
#   },
#   "quota": {...}
# }
```

---

## Monitoring & Alerts

### Recommended Alerts

**Grafana/Prometheus Alerts**:
```yaml
# Quota High Usage (Warning)
- alert: YouTubeQuotaHigh
  expr: youtube_quota_percentage > 70
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "YouTube quota usage is high ({{ $value }}%)"

# Quota Critical (Critical)
- alert: YouTubeQuotaCritical
  expr: youtube_quota_percentage > 85
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "YouTube quota usage is critical ({{ $value }}%)"

# Stale Reservations (Warning)
- alert: YouTubeQuotaStaleReservations
  expr: youtube_quota_reserved_units > 50
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High number of stale quota reservations ({{ $value }} units)"
```

### Key Metrics to Monitor

- **Global Quota Usage** (`youtube_quota_percentage`): Should be <70%
- **Reserved Units** (`youtube_quota_reserved_units`): Should be <50
- **Polling Multiplier** (`youtube_polling_multiplier`): 1.0 = normal, >1.0 = throttled
- **Active Streams** (`youtube_active_streams`): Number of live streams being polled
- **API Call Duration** (`youtube_api_duration_seconds`): P99 should be <2 seconds

---

## Recovery Procedures

### Procedure 1: Emergency Quota Reset (Development Only)

**⚠️ WARNING: Only use in development environments**

```sql
-- Reset quota to 0 (restart service afterward)
UPDATE youtube_quota_usage
SET units_used = 0, units_reserved = 0, updated_at = NOW()
WHERE date = CURRENT_DATE;

-- Clear all reservations
DELETE FROM youtube_quota_reservations WHERE date = CURRENT_DATE;
```

### Procedure 2: Graceful Service Restart

```bash
# Kubernetes rolling restart (zero downtime)
kubectl rollout restart deployment/youtube-listener -n allchat

# Wait for pods to be ready
kubectl rollout status deployment/youtube-listener -n allchat

# Verify quota state after restart
kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .global
```

### Procedure 3: Scale Down to Reduce Quota Usage

```bash
# Temporarily reduce replicas to 1 (reduce quota consumption)
kubectl scale deployment/youtube-listener --replicas=1 -n allchat

# Monitor quota usage
watch 'kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .global.percentage'

# Scale back up when quota allows
kubectl scale deployment/youtube-listener --replicas=2 -n allchat
```

---

## Related Documentation

- [YouTube Listener README](../../services/youtube-listener/README.md) - Complete service documentation
- [Quota Implementation](../../services/youtube-listener/quota/tracker.go) - Reserve-confirm-rollback code
- [Migration 008](../../migrations/008_quota_reservations.sql) - Quota reservation schema
- [ARCHITECTURE: Data Flow](../architecture/01-DATA-FLOW.md) - Message processing pipeline

---

## Success Criteria

✅ Issue resolved when:
1. `/quota/status` shows state is `HEALTHY` or `DEGRADED`
2. `units_reserved` is <50 (no stale reservations)
3. Quota tracking drift is <±5 units (database vs expected)
4. All active streams polling normally
5. No quota-related errors in logs

**Next Steps**: Monitor quota usage over 24 hours to ensure sustainable consumption rate.
