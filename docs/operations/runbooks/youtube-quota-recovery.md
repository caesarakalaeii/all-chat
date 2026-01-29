# Runbook: YouTube Quota Recovery

**Scenario**: YouTube API quota exhausted, polling stopped

**Time Estimate**: 5-15 minutes (or wait until midnight PT)
**Risk**: Low (quota automatically resets, no data loss)

---

## Immediate Actions

### 1. Confirm Quota Exhaustion

```bash
# Check quota status
kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .global

# Expected if exhausted:
# {
#   "state": "EXHAUSTED" or "DEPLETED",
#   "used": 9500+,
#   "remaining": <500,
#   "percentage": 95.0+,
#   "resets_at": "2026-01-29T00:00:00-08:00"
# }
```

### 2. Check Reset Time

```bash
# Quota resets at midnight Pacific Time
# Calculate hours until reset
TZ='America/Los_Angeles' date
```

---

## Recovery Options

### Option 1: Wait for Automatic Reset (Recommended)

**Best for**: Normal quota exhaustion, midnight PT is within 12 hours

**Timeline**:
- Quota resets at **00:00:00 Pacific Time** (PST/PDT)
- Service automatically detects reset and resumes polling
- No manual intervention required

**What Happens**:
1. T+0: Midnight PT, YouTube resets quota
2. T+1min: YouTube Listener detects reset in next quota check
3. T+2min: Polling resumes for all active streams
4. T+5min: All overlays receiving messages again

**Monitor**:
```bash
# Watch quota status (wait for midnight PT)
watch 'kubectl exec -n allchat deployment/youtube-listener -- \
  wget -qO- http://localhost:8086/quota/status | jq .global.state'

# After midnight, should change: DEPLETED → HEALTHY
```

---

### Option 2: Request Emergency Quota Increase (Google)

**Best for**: Critical production issue, cannot wait until midnight

**Steps**:
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to: APIs & Services → YouTube Data API v3 → Quotas
3. Click "Edit Quotas" → Select "Queries per day"
4. Request increase to **1,000,000 units/day** (100× increase)
5. Provide justification: "Live streaming chat aggregation service"

**Timeline**: Google approval takes 1-3 business days

---

### Option 3: Reduce Active Streams (Temporary)

**Best for**: Quota exhausted early in day, need to continue with reduced capacity

```bash
# Disable low-priority channels in database
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat -c "
  UPDATE overlay_chat_sources
  SET is_active = false
  WHERE platform = 'youtube'
    AND priority > 1;  -- Keep only priority 1 (high-priority) channels
"

# Check quota freed up
curl http://localhost:8086/quota/status | jq .global
```

**Re-enable after reset**:
```bash
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat -c "
  UPDATE overlay_chat_sources
  SET is_active = true
  WHERE platform = 'youtube';
"
```

---

## Prevention

### Long-Term Solution: Request Quota Increase

**Target**: 1,000,000 units/day (100× increase)

**Justification Template**:
```
Project: All-Chat - Multi-platform streaming chat aggregation
Use Case: Aggregate live chat from YouTube, Twitch, Kick, TikTok for streamers

Expected Usage:
- 50-100 concurrent live streams
- Polling every 2-5 seconds (per YouTube API recommendation)
- 5 units per liveChatMessages.list call
- Estimated: 100,000-200,000 units/day

Current Quota: 10,000 units/day (insufficient)
Requested Quota: 1,000,000 units/day

Justification: Educational platform for multi-streaming creators.
```

### Monitor Quota Daily

**Alerts**:
```yaml
# Warning at 70%
alert: YouTubeQuotaHigh
expr: listener_quota_usage_percentage{platform="youtube"} > 70

# Critical at 85%
alert: YouTubeQuotaCritical
expr: listener_quota_usage_percentage{platform="youtube"} > 85
```

---

## Verification After Recovery

```bash
# 1. Check quota state is HEALTHY
curl http://localhost:8086/quota/status | jq .global.state
# Expected: "HEALTHY"

# 2. Check polling resumed
kubectl logs -n allchat deployment/youtube-listener | tail -50
# Expected: "Polling chat messages" logs

# 3. Check messages appearing in overlays
redis-cli XREAD COUNT 10 STREAMS chat:raw 0 | grep youtube
# Should see recent YouTube messages
```

---

## Related Documentation

- [QUICK-REF-DEBUG-QUOTA.md](../../llm-guides/QUICK-REF-DEBUG-QUOTA.md) - Comprehensive quota debugging
- [youtube-listener/README.md](../../../services/youtube-listener/README.md) - Service documentation
- [ADR-0006](../../adr/0006-youtube-quota-tracking.md) - Quota tracking architecture
