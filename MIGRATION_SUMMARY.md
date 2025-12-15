# Database Migration Summary - Robust Stream Detection

## ✅ Migrations Applied Successfully

### Migration 007: YouTube Channel Quota Tracking
- **Table Created:** `youtube_channel_quota` with 6 indexes
- **Functions Created:**
  - `record_youtube_quota_usage(channel_id, units)`
  - `reset_youtube_daily_quotas()`
  - `promote_youtube_channel_tier(channel_id)`
  - `demote_inactive_youtube_channels()`
  - `update_youtube_channel_quota_updated_at()` (trigger function)
- **Initial Data:** 2 YouTube channels populated with default `standard` tier

### Migration 010: Stream History Tracking
- **Table Created:** `stream_history` with 5 indexes
- **Functions Created:**
  - `update_stream_history_on_detection(platform, channel_id, channel_name, is_live)`
  - `analyze_streaming_patterns()` (placeholder for future)
  - `get_likely_live_channels(platform)` (pattern-based predictions)
- **Initial Data:** 10 active sources populated (2 YouTube, 6 Twitch, 1 Kick, 1 TikTok)

### Database Details
- **Cluster:** `allchat-cluster` in namespace `allchat`
- **Current Primary:** `allchat-cluster-2`
- **Database:** `allchat`
- **User:** `allchat_user` / postgres
- **Status:** Healthy (3/3 instances ready)

---

## 🔧 Required Environment Variable Updates

**IMPORTANT:** These environment variables need to be added to the caesar-deployment repository for the following services:

### YouTube Listener Service

Add to `caesar-deployment/services/youtube-listener/config.yaml` (or equivalent):

```yaml
# YouTube Listener - Hybrid Detection & Quota Management
ENABLE_HYBRID_DETECTION: "true"

# Global daily quota limit (default: 10000 units/day from YouTube)
YOUTUBE_GLOBAL_DAILY_QUOTA: "10000"

# Per-tier quota limits (units per channel per day)
YOUTUBE_HIGH_TIER_QUOTA: "200"
YOUTUBE_STANDARD_TIER_QUOTA: "100"
YOUTUBE_LOW_TIER_QUOTA: "50"

# Detection intervals per tier
YOUTUBE_HIGH_TIER_INTERVAL: "30s"
YOUTUBE_STANDARD_TIER_INTERVAL: "2m"
YOUTUBE_LOW_TIER_INTERVAL: "10m"

# Backoff configuration
YOUTUBE_GENTLE_BACKOFF_MULTIPLIER: "1.2"
YOUTUBE_GENTLE_BACKOFF_CAP: "2.0"
YOUTUBE_AGGRESSIVE_BACKOFF_MULTIPLIER: "2.0"
YOUTUBE_AGGRESSIVE_BACKOFF_CAP: "60.0"

# Activity thresholds
YOUTUBE_HIGH_TIER_DURATION: "24h"
YOUTUBE_DEMOTION_THRESHOLD: "168h"  # 7 days
YOUTUBE_RECENT_ACTIVITY_WINDOW: "24h"

# Detection fallback thresholds
YOUTUBE_STATUS_CHECK_FAILURE_THRESHOLD: "3"
YOUTUBE_OFFLINE_CACHE_CLEAR_THRESHOLD: "3"
```

### TikTok Listener Service

Add to `caesar-deployment/services/tiktok-listener/config.yaml` (or equivalent):

```yaml
# TikTok Listener - Live Detection & Backoff
TIKTOK_STATUS_CHECK_CACHE_TTL_MS: "10000"  # 10 seconds
TIKTOK_POLLER_INTERVAL_MS: "30000"  # 30 seconds

# Offline backoff configuration
TIKTOK_BASE_OFFLINE_BACKOFF_MS: "60000"   # 1 minute
TIKTOK_MAX_OFFLINE_BACKOFF_MS: "600000"   # 10 minutes

# Error backoff configuration
TIKTOK_ERROR_BACKOFF_MS: "2000"           # 2 seconds
TIKTOK_MAX_ERROR_BACKOFF_MS: "300000"     # 5 minutes
```

---

## 📊 Current State

### YouTube Channels Being Tracked
```
Channel ID                    | Priority Tier | Quota Limit
------------------------------|---------------|------------
UCRs6QcV9kwHu7V0LLlIvwxQ     | standard      | 100 units/day
UCTJyWF-A_kbFVV4aFh8HnvQ     | standard      | 100 units/day
```

### Active Sources in Stream History
```
Platform | Channel ID              | Channel Name
---------|-------------------------|-------------
kick     | 86306498                | allch_at
tiktok   | _caesarlp               | Caesar
twitch   | Papaplatte              | Papaplatte
twitch   | bims_sh                 | bims_sh
twitch   | caesarlp                | CaesarLP
twitch   | crypticdude1            | crypticdude1
twitch   | salmmus                 | salmmus
twitch   | youngdabo               | youngdabo
youtube  | UCRs6QcV9kwHu7V0LLlIvwxQ | Caesar LP
youtube  | UCTJyWF-A_kbFVV4aFh8HnvQ | Cryptic
```

---

## 🚀 Next Steps

### 1. Update caesar-deployment Repository
- [ ] Add YouTube Listener environment variables
- [ ] Add TikTok Listener environment variables
- [ ] Commit and push changes
- [ ] Apply to cluster: `kubectl apply -f <deployment-files>`

### 2. Build and Deploy Updated Services
```bash
# YouTube Listener (Go)
cd services/youtube-listener
go build -o bin/youtube-listener ./cmd
docker build -t <registry>/youtube-listener:latest .
docker push <registry>/youtube-listener:latest

# TikTok Listener (TypeScript)
cd services/tiktok-listener
npm run build
docker build -t <registry>/tiktok-listener:latest .
docker push <registry>/tiktok-listener:latest
```

### 3. Restart Services in Kubernetes
```bash
# Restart YouTube Listener
kubectl -n allchat rollout restart deployment youtube-listener

# Restart TikTok Listener
kubectl -n allchat rollout restart deployment tiktok-listener

# Monitor rollout
kubectl -n allchat rollout status deployment youtube-listener
kubectl -n allchat rollout status deployment tiktok-listener
```

### 4. Verify Migrations
```bash
# Check tables exist
kubectl -n allchat exec allchat-cluster-2 -- psql -U postgres -d allchat -c "\dt" | grep -E "youtube_channel_quota|stream_history"

# Check functions exist
kubectl -n allchat exec allchat-cluster-2 -- psql -U postgres -d allchat -c "\df" | grep -E "youtube|stream"

# Monitor logs for new features
kubectl -n allchat logs -f deployment/youtube-listener --tail=50
kubectl -n allchat logs -f deployment/tiktok-listener --tail=50
```

---

## 🔍 Monitoring & Troubleshooting

### Check Quota Usage
```bash
kubectl -n allchat exec allchat-cluster-2 -- psql -U postgres -d allchat -c "
SELECT
  channel_id,
  priority_tier,
  daily_quota_used,
  daily_quota_limit,
  last_seen_live_at
FROM youtube_channel_quota
ORDER BY priority_tier, daily_quota_used DESC;"
```

### Check Stream History
```bash
kubectl -n allchat exec allchat-cluster-2 -- psql -U postgres -d allchat -c "
SELECT
  platform,
  channel_name,
  last_seen_live,
  last_seen_offline,
  consecutive_offline_checks,
  total_streams_detected
FROM stream_history
ORDER BY platform, last_seen_live DESC NULLS LAST;"
```

### Watch Service Logs
```bash
# YouTube Listener - Look for "hybrid detection" logs
kubectl -n allchat logs -f deployment/youtube-listener | grep -i "quota\|detection\|backoff"

# TikTok Listener - Look for "live detection" logs
kubectl -n allchat logs -f deployment/tiktok-listener | grep -i "live\|backoff\|poller"
```

---

## 📝 Rollback Instructions (If Needed)

If something goes wrong, rollback using the provided down migrations:

```bash
# Rollback migration 010
cat migrations/010_stream_history_tracking_down.sql | \
  kubectl -n allchat exec -i allchat-cluster-2 -- psql -U postgres -d allchat

# Rollback migration 007
cat migrations/007_youtube_channel_quota_down.sql | \
  kubectl -n allchat exec -i allchat-cluster-2 -- psql -U postgres -d allchat
```

---

## ✨ Expected Improvements

### YouTube Listener
- **80 channels** supported (vs 10-20 currently)
- **90-94% quota reduction** per channel
- **30s-3m detection latency** (vs 5-10m currently)
- Automatic tier management based on activity
- Graceful degradation when quota exhausted

### TikTok Listener
- **Near-instant detection** for active streams
- **80%+ reduction** in failed connection attempts
- Smart backoff prevents API spam
- Automatic recovery when streams return

---

**Migration completed successfully on:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**Applied to cluster:** allchat-cluster (namespace: allchat)
