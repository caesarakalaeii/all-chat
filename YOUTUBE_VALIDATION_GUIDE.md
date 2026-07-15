# YouTube Listener Validation Guide

> **Scope / history:** This guide validates the quota-tracking `youtube-listener` (the `/admin/detection/quota-budget` and circuit-breaker endpoints on `:8086`). **Production runs `youtube-listener-innertube` instead**, so the quota-budget/circuit-breaker endpoints and the `youtube-listener` Deployment do not exist in prod. Locally the container is named `allchat-youtube-listener` (a bare `docker logs youtube-listener` / `docker restart youtube-listener` will fail), and the `kubectl rollout undo deployment/youtube-listener` step in Section 10 targets a deployment that is not present in prod. Treat the `kubectl`/`docker` commands below accordingly.

This guide helps validate all the changes made to fix YouTube stream detection issues.

## Prerequisites

```bash
# Start the services
make docker-up

# Check YouTube listener is running
docker logs allchat-youtube-listener

# Check logs for quota budget initialization
docker logs allchat-youtube-listener | grep "Quota budget system configured"
```

## 1. Verify Quota Budgeting System

### Check Initialization
```bash
# Should see quota budget startup in logs
docker logs allchat-youtube-listener | grep -A 10 "Quota budget system configured"
```

**Expected Output:**
```
Quota budget system configured
  high_priority_cap: 100
  standard_priority_cap: 50
  low_priority_cap: 20
  manual_ops_reserve_percent: 20.0
  ...
```

### Check Quota Budget API
```bash
curl http://localhost:8086/admin/detection/quota-budget | jq
```

**Expected Fields:**
- `quota_remaining_percent`: Current quota remaining
- `quota_used`, `quota_remaining`, `quota_total`
- `manual_ops_used_today`, `manual_ops_reserve`
- `emergency_mode`, `prefer_status_checks`, `throttle_low_priority`
- `channels`: High/standard/low priority counts
- `detections_skipped_by_reason`: Reasons for skipped detections

## 2. Verify Circuit Breaker Changes

### Check Configuration
```bash
# Circuit breaker should use new thresholds
docker logs allchat-youtube-listener | grep "circuit"
```

**Expected Behavior:**
- Opens after **5 failures** (not 3)
- Blocks for **10 minutes** (not 30)

### Test Circuit Breaker API
```bash
# Get circuit breakers for all channels
curl http://localhost:8086/quota/circuit-breakers | jq

# Expected: Shows channels with circuit state, failures, retry_in_seconds
```

### Reset Circuit Breaker (Manual Test)
```bash
# Pick a channel with OPEN circuit
CHANNEL_ID="UCxxxxx"

# Reset it
curl -X POST http://localhost:8086/admin/circuit-breakers/${CHANNEL_ID}/reset

# Verify state changed to CLOSED
curl http://localhost:8086/quota/circuit-breakers | jq ".[] | select(.channel_id == \"$CHANNEL_ID\")"
```

## 3. Verify Tiered Backoff Strategy

### Check Channel States
```bash
# Get all channel detection states
curl http://localhost:8086/admin/detection/channels | jq '.channels[] | {channel_id, priority, backoff_state, risk_level}'
```

**Expected:**
- Channels classified as `high`, `standard`, or `low` priority
- Backoff intervals vary based on priority and quota:
  - High priority: 30s-2m (quota available), 2m-10m (quota low)
  - Standard: 1m-5m (quota available), 5m-15m (quota low)
  - Low: 5m-20m (always)

### Test Specific Channel
```bash
CHANNEL_ID="UCxxxxx"
curl http://localhost:8086/admin/detection/channels/${CHANNEL_ID} | jq
```

**Expected Fields:**
- `backoff_state`: Current interval, failure count, last check time
- `circuit_breaker_state`: State, failures, retry time
- `priority`: Channel priority tier
- `detections_today`: Full detections used today
- `quota_cap`: Daily quota cap for this channel
- `risk_level`: high/medium/low
- `recommended_action`: What to do if stuck

## 4. Verify Manual Control Endpoints

### Reset Backoff for Stuck Channel
```bash
# Get stuck channels (backoff >5min with connected overlays)
curl http://localhost:8086/admin/detection/channels?stuck=true | jq

# Reset backoff for a channel
CHANNEL_ID="UCxxxxx"
curl -X POST http://localhost:8086/admin/detection/channels/${CHANNEL_ID}/reset-backoff
```

### Force Detection (Emergency)
```bash
# Force immediate detection (bypasses quota budget)
curl -X POST http://localhost:8086/admin/detection/channels/${CHANNEL_ID}/force-check

# Check logs for manual operation
docker logs allchat-youtube-listener | tail -20
```

**Expected Log:**
```
Manually forced channel detection
  channel_id: UCxxxxx
  action: admin_force_detection
  quota_used: 100
```

### Emergency Reset All
```bash
# Reset all channels (requires confirmation)
curl -X POST "http://localhost:8086/admin/detection/reset-all?confirm=yes"
```

## 5. Verify Automatic Stuck State Recovery

### Monitor Recovery Logs
```bash
# Watch for auto-recovery (runs every 5 minutes)
docker logs -f youtube-listener | grep -i "auto-recover"
```

**Expected Output (when recovery happens):**
```
Auto-recovered stuck channel
  channel_id: UCxxxxx
  reason: circuit_breaker_open_30min OR high_backoff_recently_active
  connected_overlays: 2
  action: auto_recovery
```

### Check Recovery Metrics
```bash
# Should increment when recovery happens
curl http://localhost:8086/metrics | grep youtube_auto_recovery_total
```

## 6. Verify Prometheus Metrics

### Backoff Metrics
```bash
# Current backoff intervals per channel (in seconds)
curl http://localhost:8086/metrics | grep youtube_backoff_current_interval_seconds

# Channels stuck in >5min backoff
curl http://localhost:8086/metrics | grep youtube_backoff_channels_stuck

# Detections skipped by reason
curl http://localhost:8086/metrics | grep youtube_detection_skipped_total
```

### Quota Budget Metrics
```bash
# Per-channel quota remaining
curl http://localhost:8086/metrics | grep youtube_quota_budget_remaining_per_channel

# Detections throttled by quota budget
curl http://localhost:8086/metrics | grep youtube_quota_budget_throttled_total
```

### At-Risk Channels
```bash
# Channels with long backoff + connected overlays
curl http://localhost:8086/metrics | grep youtube_channels_at_risk
```

## 7. Integration Testing Scenarios

### Scenario 1: Channel Goes Live
1. Start with channel offline, backoff at max (10min)
2. Channel goes live
3. **Expected:** Detection within 2-3 minutes for high-priority channel

### Scenario 2: Quota Budget Protection
1. Set a channel to use 100 full detections (cap reached)
2. Try to detect
3. **Expected:**
   - Detection skipped: `channel_quota_cap_reached`
   - Metric incremented: `youtube_quota_budget_throttled_total`
   - Can force via manual endpoint (bypasses budget)

### Scenario 3: Circuit Breaker Opens
1. Channel goes offline
2. 5 consecutive detection failures occur
3. **Expected:**
   - Circuit opens (blocks for 10 minutes)
   - After 10 minutes, transitions to half-open
   - Can be manually reset

### Scenario 4: Auto-Recovery
1. Channel in max backoff (10min) for >30 minutes
2. Channel has connected overlays
3. **Expected:**
   - After 5-minute recovery cycle runs
   - Channel automatically reset
   - Log shows `auto_recovery`
   - Immediate detection attempted

## 8. Performance Validation

### Quota Usage
```bash
# Check quota usage patterns
curl http://localhost:8086/quota/status | jq '{state, percentage, remaining}'

# Should see:
# - Lower usage due to status checks (1 unit) vs full detection (100 units)
# - Per-channel caps prevent runaway usage
```

### Detection Speed
```bash
# Monitor backoff intervals
watch -n 5 'curl -s http://localhost:8086/admin/detection/channels | jq ".summary.quota_budget"'

# Should see:
# - High priority channels: 30s-2m intervals (when quota available)
# - Standard channels: 1m-5m intervals
# - Low priority channels: 5m-20m intervals
```

## 9. Failure Scenarios

### Test Quota Exhaustion
```bash
# Simulate quota exhaustion (if in test environment)
# Manual ops should still work but log warnings

curl -X POST http://localhost:8086/admin/detection/channels/${CHANNEL_ID}/force-check

# Should work but warn about manual reserve usage
```

### Test Emergency Mode
```bash
# When quota <30% remaining
curl http://localhost:8086/admin/detection/quota-budget | jq '.emergency_mode'

# Expected: true
# Low priority channels should be paused
```

## 10. Rollback Testing

If issues are found:

```bash
# Revert to old circuit breaker settings
export CIRCUIT_BREAKER_FAILURE_THRESHOLD=3
export CIRCUIT_BREAKER_OPEN_DURATION_MINUTES=30

# Restart service
docker restart youtube-listener

# Or full rollback
git revert <commit-sha>
kubectl rollout undo deployment/youtube-listener
```

## Success Criteria

✅ **Circuit Breaker:**
- Opens after 5 failures (not 3)
- Recovers in 10 minutes (not 30)

✅ **Backoff Strategy:**
- High priority: <2min retry (quota available)
- Backoff adapts to quota state
- Status checks preferred when quota low

✅ **Quota Budget:**
- Per-channel caps enforced
- Emergency mode activates when quota <30%
- Manual operations bypass budget with logging

✅ **Manual Control:**
- Can inspect channel state via API
- Can reset backoff/circuit for stuck channels
- Can force detection (emergency use)

✅ **Auto-Recovery:**
- Runs every 5 minutes
- Recovers channels stuck >30min (circuit) or >10min (backoff for active)
- Metrics incremented on recovery

✅ **Metrics:**
- All new metrics exposed and updating
- Dashboards show backoff intervals, stuck channels, at-risk channels

✅ **No Missed Streams:**
- Channels with overlays detect streams within 2-3 minutes
- Previously stuck channels now recover automatically
