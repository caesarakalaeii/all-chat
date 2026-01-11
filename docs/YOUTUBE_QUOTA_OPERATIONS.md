# YouTube Quota Operations Guide

Quick reference for managing YouTube API quota in production.

---

## 🔍 Monitoring Endpoints

### 1. Quota Status
```bash
# Current quota usage
curl http://youtube-listener:8086/quota/status | jq
```

**Response:**
```json
{
  "global": {
    "state": "HEALTHY",
    "used": 2500,
    "limit": 10000,
    "remaining": 7500,
    "percentage": 25.0,
    "resets_at": "2026-01-12T00:00:00-08:00"
  }
}
```

### 2. Circuit Breaker Status (NEW!)
```bash
# View all circuit breakers
curl http://youtube-listener:8086/quota/circuit-breakers | jq
```

**Response:**
```json
{
  "circuit_breakers": {
    "UCd2RlRnXhdBKGHw66QWyyCw": {
      "channel_id": "UCd2RlRnXhdBKGHw66QWyyCw",
      "state": "OPEN",
      "consecutive_failures": 3,
      "total_failures": 5,
      "quota_saved": 500,
      "retry_in_seconds": 1234
    }
  },
  "count": 1
}
```

### 3. Prometheus Metrics
```bash
# View all metrics
curl http://youtube-listener:8086/metrics | grep youtube_
```

---

## 🛠️ Admin Operations

### Reset Specific Circuit Breaker
```bash
# Reset circuit breaker for a channel
curl -X POST http://youtube-listener:8086/admin/circuit-breakers/UCd2RlRnXhdBKGHw66QWyyCw/reset

# Response
{
  "success": true,
  "channel_id": "UCd2RlRnXhdBKGHw66QWyyCw",
  "message": "circuit breaker reset successfully"
}
```

### Reset All Circuit Breakers
```bash
# Emergency reset all circuit breakers
curl -X POST http://youtube-listener:8086/admin/circuit-breakers/reset-all

# Response
{
  "success": true,
  "message": "all circuit breakers reset successfully"
}
```

**⚠️ Use Cases:**
- Channel incorrectly marked as offline
- Emergency recovery after quota reset
- Testing/debugging

---

## 📊 Grafana Dashboard Queries

### Circuit Breaker Metrics

```promql
# Number of circuit breakers currently OPEN
sum(youtube_circuit_breaker_state == 2)

# Total quota saved by circuit breakers
sum(youtube_circuit_breaker_quota_saved_units_total)

# Circuit breaker state per channel
youtube_circuit_breaker_state

# Consecutive failures per channel
youtube_circuit_breaker_failures

# Circuit breaker transitions (gauge - shows activity)
rate(youtube_circuit_breaker_transitions_total[5m])
```

### Connection-Aware Polling Metrics

```promql
# Connection checks per second
rate(youtube_poller_connection_checks_total[1m])

# Pollers stopped due to disconnect
rate(youtube_poller_stopped_by_disconnect_total[5m])

# Quota saved by not polling disconnected overlays
rate(youtube_poller_quota_saved_units_total[5m]) * 60

# Percentage of polls that found disconnected overlays
rate(youtube_poller_connection_checks_total{result="disconnected"}[5m])
  /
rate(youtube_poller_connection_checks_total[5m]) * 100
```

### Quota Drift Tracking

```promql
# Drift detection events
rate(youtube_quota_drift_detected_total[5m])

# Current drift (database vs memory)
youtube_quota_drift_units

# Database sync success rate
rate(youtube_quota_database_sync_total{result="success"}[5m])
  /
rate(youtube_quota_database_sync_total[5m]) * 100

# Database sync errors
rate(youtube_quota_database_sync_errors_total[5m])
```

### Emergency Shutoff

```promql
# Emergency shutoff triggers (should be 0)
increase(youtube_emergency_shutoff_triggers_total[24h])

# Blocked API calls
rate(youtube_emergency_shutoff_blocked_total[5m])

# Blocked calls by operation type
sum by (operation) (youtube_emergency_shutoff_blocked_total)
```

---

## 🗄️ Database Queries

### View Audit Log
```sql
-- Last 50 quota operations
SELECT
    timestamp,
    operation_type,
    service_name,
    endpoint,
    units_delta,
    units_after,
    api_success,
    error_type
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
ORDER BY timestamp DESC
LIMIT 50;
```

### Breakdown by Service
```sql
-- See which service is consuming quota
SELECT
    service_name,
    COUNT(*) as operations,
    SUM(units_delta) as total_units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
GROUP BY service_name
ORDER BY total_units DESC;
```

### Breakdown by Endpoint
```sql
-- See which API endpoints are expensive
SELECT
    endpoint,
    COUNT(*) as calls,
    SUM(units_delta) as total_units,
    AVG(units_delta) as avg_per_call
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
GROUP BY endpoint
ORDER BY total_units DESC;
```

### Hourly Trend
```sql
-- View quota usage by hour
SELECT
    DATE_TRUNC('hour', timestamp) as hour,
    COUNT(*) as api_calls,
    SUM(units_delta) as units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
GROUP BY hour
ORDER BY hour;
```

### Daily Reconciliation
```sql
-- Run and view reconciliation report
SELECT reconcile_youtube_quota_usage(CURRENT_DATE);

-- View stored reconciliation
SELECT * FROM youtube_quota_reconciliation
WHERE date = CURRENT_DATE;
```

---

## 🚨 Alerts (Prometheus/Grafana)

### Critical Alerts

```yaml
# Quota usage above 70% (warning)
- alert: YouTubeQuotaWarning
  expr: youtube_quota_usage_percentage > 70
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "YouTube quota at {{ $value }}%"

# Quota usage above 85% (critical)
- alert: YouTubeQuotaCritical
  expr: youtube_quota_usage_percentage > 85
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "YouTube quota CRITICAL at {{ $value }}%"

# Emergency shutoff triggered
- alert: YouTubeEmergencyShutoff
  expr: increase(youtube_emergency_shutoff_triggers_total[5m]) > 0
  labels:
    severity: critical
  annotations:
    summary: "YouTube emergency shutoff triggered!"

# Circuit breaker opened
- alert: YouTubeCircuitBreakerOpen
  expr: youtube_circuit_breaker_state == 2
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Circuit breaker OPEN for channel {{ $labels.channel_id }}"

# Quota drift detected
- alert: YouTubeQuotaDrift
  expr: increase(youtube_quota_drift_detected_total{severity="major"}[5m]) > 0
  labels:
    severity: warning
  annotations:
    summary: "Major quota drift detected (>50 units)"
```

---

## 🎯 Common Operations

### Check Why Channel Not Polling

```bash
# 1. Check if circuit breaker is open
curl http://youtube-listener:8086/quota/circuit-breakers | \
  jq '.circuit_breakers["UCxxxxxx"]'

# 2. Check quota state
curl http://youtube-listener:8086/quota/status | \
  jq '.global.state'

# 3. Check logs
kubectl logs -n allchat -l app=youtube-listener --tail=100 | \
  grep "UCxxxxxx"
```

### Recover From Circuit Breaker Block

```bash
# Option 1: Wait for automatic recovery (30 minutes)

# Option 2: Manual reset
curl -X POST http://youtube-listener:8086/admin/circuit-breakers/UCxxxxxx/reset

# Option 3: Restart pods (resets all circuit breakers)
kubectl delete pod -n allchat -l app=youtube-listener
```

### Investigate Quota Spike

```sql
-- Step 1: Check today's breakdown
SELECT
    endpoint,
    COUNT(*) as calls,
    SUM(units_delta) as units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
GROUP BY endpoint
ORDER BY units DESC;

-- Step 2: Find time of spike
SELECT
    DATE_TRUNC('hour', timestamp) as hour,
    endpoint,
    COUNT(*) as calls,
    SUM(units_delta) as units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
GROUP BY hour, endpoint
ORDER BY hour DESC, units DESC;

-- Step 3: Check which channel caused it
SELECT
    channel_id,
    COUNT(*) as calls,
    SUM(units_delta) as units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE
  AND operation_type IN ('confirm', 'record')
  AND endpoint = 'Search.List'  -- Most expensive
GROUP BY channel_id
ORDER BY units DESC;
```

---

## 📈 Grafana Dashboard Example

### Panel 1: Quota Usage Gauge
```promql
# Query: Quota usage percentage
listener_quota_usage_percentage{platform="youtube",quota_type="daily"}

# Thresholds:
# Green: 0-70%
# Yellow: 70-85%
# Orange: 85-95%
# Red: 95-100%
```

### Panel 2: Circuit Breaker Summary
```promql
# Count by state
sum by (state) (
  youtube_circuit_breaker_state == 0  # CLOSED
  or
  youtube_circuit_breaker_state == 1  # HALF_OPEN
  or
  youtube_circuit_breaker_state == 2  # OPEN
)
```

### Panel 3: Quota Saved
```promql
# Total quota saved in last 24h
increase(youtube_circuit_breaker_quota_saved_units_total[24h])
  +
increase(youtube_poller_quota_saved_units_total[24h])
```

### Panel 4: API Call Breakdown
```promql
# API calls by endpoint
sum by (endpoint) (
  rate(youtube_api_calls_total[5m])
)
```

---

## 🐛 Troubleshooting Scenarios

### Scenario 1: Quota Depleted Before End of Day

**Check:**
```bash
# 1. View hourly trend
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres -d allchat <<SQL
SELECT DATE_TRUNC('hour', timestamp) as hour, SUM(units_delta) as units
FROM youtube_quota_audit_log
WHERE date = CURRENT_DATE AND operation_type IN ('confirm','record')
GROUP BY hour ORDER BY hour;
SQL

# 2. Check circuit breakers
curl http://youtube-listener:8086/quota/circuit-breakers

# 3. Check for emergency shutoff
kubectl logs -n allchat -l app=youtube-listener | grep "EMERGENCY"
```

**Fix:**
- If circuit breakers disabled: They're working as intended
- If legitimate traffic: Consider quota increase request to Google
- If runaway polling: Check connection debounce settings

### Scenario 2: Channel Not Polling Despite Being Live

**Check:**
```bash
# 1. Circuit breaker status
curl http://youtube-listener:8086/quota/circuit-breakers | \
  jq '.circuit_breakers["UCxxxxxx"]'

# 2. If state = "OPEN":
curl -X POST http://youtube-listener:8086/admin/circuit-breakers/UCxxxxxx/reset
```

### Scenario 3: Drift Between Database and Memory

**Check:**
```bash
# Should see in logs every 5 minutes if drift > 5 units
kubectl logs -n allchat -l app=youtube-listener --tail=100 | grep "drift"

# Verify periodic sync is running
kubectl logs -n allchat -l app=youtube-listener | grep "Started periodic database sync"
```

**Fix:**
- Drift will auto-correct within 5 minutes
- If persistent: Check database connectivity
- If major drift: Run reconciliation report

---

## 📝 Maintenance Tasks

### Daily (Automated)
- ✅ Quota resets at midnight PST
- ✅ Per-channel quotas reset
- ✅ Stale reservations cleaned up (every 1 min)
- ✅ Database sync (every 5 minutes)
- ✅ Connection cleanup (every 2 minutes)

### Weekly (Manual)
```sql
-- Clean up old audit logs (keeps 30 days)
SELECT cleanup_old_youtube_audit_logs();

-- Review quota usage trends
SELECT date, units_used, units_limit
FROM youtube_quota_usage
WHERE date >= CURRENT_DATE - 7
ORDER BY date DESC;
```

### Monthly (Manual)
```sql
-- Review reconciliation history
SELECT date, status, drift_db_vs_audit, by_service
FROM youtube_quota_reconciliation
WHERE date >= CURRENT_DATE - 30
ORDER BY date DESC;

-- Check for persistent issues
SELECT channel_id, COUNT(*) as circuit_opens
FROM youtube_quota_audit_log
WHERE timestamp >= NOW() - INTERVAL '30 days'
  AND operation_type = 'circuit_open'
GROUP BY channel_id
ORDER BY circuit_opens DESC;
```

---

## 🎛️ Configuration

### Environment Variables
```bash
# Quota thresholds
EMERGENCY_QUOTA_THRESHOLD=90.0          # Hard block at 90%
QUOTA_HEALTHY_THRESHOLD=70.0            # 0-70% normal
QUOTA_DEGRADED_THRESHOLD=85.0           # 70-85% reduced discovery
QUOTA_CRITICAL_THRESHOLD=95.0           # 85-95% polling only
QUOTA_EXHAUSTED_THRESHOLD=100.0         # 95-100% slow polling

# Connection management
OVERLAY_DISCONNECT_DEBOUNCE_SECONDS=90  # Grace period
```

### Circuit Breaker Settings (Code)
To change circuit breaker behavior, modify `streams/circuit_breaker.go`:
```go
failureThreshold: 3              // Open after N failures
openDuration: 30 * time.Minute   // Block duration
successThreshold: 2              // Successes needed to close
```

---

## 📞 Quick Reference

| Task | Command |
|------|---------|
| Check quota | `curl youtube-listener:8086/quota/status` |
| View circuit breakers | `curl youtube-listener:8086/quota/circuit-breakers` |
| Reset circuit breaker | `curl -X POST youtube-listener:8086/admin/circuit-breakers/:id/reset` |
| View audit log | `psql: SELECT * FROM youtube_quota_audit_log` |
| Run reconciliation | `psql: SELECT reconcile_youtube_quota_usage(CURRENT_DATE)` |
| Force quota reload | `kubectl delete pod -l app=youtube-listener` |

---

## 🔐 Security Notes

**⚠️ TODO for Production:**
- Add authentication to admin endpoints
- Restrict circuit breaker reset to authorized IPs
- Add audit logging for admin actions
- Consider rate limiting admin endpoints

**Current State:**
- Admin endpoints are UNAUTHENTICATED
- Only accessible within cluster (ClusterIP service)
- External access requires port-forward or ingress

---

## 📊 Sample Grafana Dashboard

```yaml
# YouTube Quota Management Dashboard
panels:
  - title: "Quota Usage"
    type: gauge
    targets:
      - expr: listener_quota_usage_percentage{platform="youtube"}
    thresholds:
      - value: 70
        color: yellow
      - value: 85
        color: orange
      - value: 95
        color: red

  - title: "Circuit Breakers"
    type: stat
    targets:
      - expr: sum(youtube_circuit_breaker_state == 2)
        legend: "OPEN"
      - expr: sum(youtube_circuit_breaker_state == 0)
        legend: "CLOSED"

  - title: "Quota Saved (24h)"
    type: stat
    targets:
      - expr: increase(youtube_circuit_breaker_quota_saved_units_total[24h])
        legend: "Circuit Breakers"
      - expr: increase(youtube_poller_quota_saved_units_total[24h])
        legend: "Connection-Aware"

  - title: "API Calls by Endpoint"
    type: bar
    targets:
      - expr: sum by (endpoint) (increase(youtube_api_calls_total[1h]))

  - title: "Drift Detection"
    type: graph
    targets:
      - expr: rate(youtube_quota_drift_detected_total[5m])
      - expr: youtube_quota_drift_units
```

---

## ✅ Health Checks

### Everything Working Correctly

```bash
# Quota status: HEALTHY (< 70%)
curl http://youtube-listener:8086/quota/status | jq '.global.state'

# No open circuit breakers
curl http://youtube-listener:8086/quota/circuit-breakers | jq '.count'

# No emergency shutoff triggers
curl http://youtube-listener:8086/metrics | grep emergency_shutoff_triggers

# No major drift detected
kubectl logs -n allchat -l app=youtube-listener --tail=100 | grep "drift" | grep "major"
```

### Daily Usage on Track

```bash
# Should be < 3,500 units by end of day
# At noon: ~1,250 units (50%)
# At 6pm: ~2,500 units (83%)
# At midnight: ~3,000 units (100%)

curl http://youtube-listener:8086/quota/status | \
  jq '.global | "Used: \(.used)/\(.limit) (\(.percentage)%)"'
```

---

## 🆘 Emergency Procedures

### Emergency Shutoff Triggered (90% quota)

**What it means:** System at 9,000+ units, all non-critical operations blocked

**What still works:**
- ✅ OAuth/login (allowCritical = true)
- ✅ Health checks
- ✅ Existing WebSocket connections

**What's blocked:**
- ❌ New channel discovery (Search.List)
- ❌ Chat polling (LiveChatMessages.List)
- ❌ Status checks

**Recovery:**
1. Wait for midnight PST quota reset (automatic)
2. Or: Identify and stop runaway operations
3. Or: Request quota increase from Google

### Complete Quota Exhaustion (100%)

**Actions:**
1. Check audit log for cause
2. Reset all circuit breakers to free up channels
3. Wait for midnight reset
4. Review and adjust thresholds

### Database Connectivity Issues

**Symptoms:**
- Logs show "Failed to sync with database"
- Metrics show high `youtube_quota_database_sync_errors_total`

**Fix:**
```bash
# Check database connectivity
kubectl exec -n allchat youtube-listener-xxx -- \
  wget -qO- http://localhost:8086/health/ready

# If database is down, pods will restart automatically
# Quota tracker will reload from database on restart
```

---

For full implementation details, see:
- `/home/caesar/git/all-chat/YOUTUBE_QUOTA_FIX_SUMMARY.md`
- `/home/caesar/.claude/plans/binary-percolating-widget.md`
