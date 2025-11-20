# YouTube API Quota Increase Request

This document guides you through requesting a YouTube Data API v3 quota increase from Google.

## Current Status

- **Current Quota**: 10,000 units/day (default)
- **Target Quota**: 1,000,000 units/day
- **Reason**: Production streaming platform with multiple concurrent live streams

## Why We Need This

### API Call Costs
| Operation | Cost (units) | Usage Pattern |
|-----------|-------------|---------------|
| `search.list` | 100 | Every 30s per active channel |
| `videos.list` | 1 | Per live stream found |
| `liveChatMessages.list` | 5 | Every 2-5s per active stream |

### Example Calculations

**Single Active Stream (2-second polling interval):**
- Discovery: 100 units (search.list) every 30s = ~290 units/hour
- Video details: 1 unit per check = ~120 units/hour
- Chat messages: 5 units every 2s = 9,000 units/hour
- **Total: ~9,410 units/hour for one stream**

With the default 10,000 units/day quota, we can only monitor **one active stream for about 1 hour per day**.

### Production Requirements

For a production streaming platform supporting:
- 10 concurrent streams
- 8 hours of streaming per day average

**Required quota: ~753,000 units/day**

## How to Request Quota Increase

### Step 1: Access Google Cloud Console

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project (the one with YouTube Data API v3 enabled)
3. Navigate to: **APIs & Services** → **YouTube Data API v3** → **Quotas**

Direct link: `https://console.cloud.google.com/apis/api/youtube.googleapis.com/quotas`

### Step 2: Request Quota Increase

1. Find **"Queries per day"** in the quota list
2. Click on the quota name
3. Click **"EDIT QUOTAS"** or **"Request a higher quota"**
4. Fill out the quota increase form:

**Requested Quota**: `1000000` (1 million units/day)

### Step 3: Justification Template

Use this template for your justification:

```
Application Type: Production Streaming Platform

Use Case:
We operate a multi-platform chat aggregation service for live streamers. Our
service monitors YouTube live streams and aggregates chat messages from multiple
platforms (Twitch, YouTube, Kick, TikTok) into unified overlays.

Current Usage:
- Production service with active users
- Monitoring multiple concurrent live streams
- Average of 10-20 active streams during peak hours
- Each stream requires continuous polling of live chat messages every 2-5 seconds

Why We Need Increased Quota:
With the default 10,000 units/day quota, we can only monitor approximately
one active stream for one hour. Our production requirements demand:
- 10-20 concurrent streams during peak hours
- 8+ hours of daily operation
- Estimated daily usage: 750,000+ units

The quota increase is critical for:
1. Supporting our existing user base
2. Maintaining service reliability
3. Scaling to meet growing demand
4. Providing real-time chat aggregation for live streamers

We implement quota tracking, monitoring, and have safeguards in place to prevent
abuse or unnecessary API calls.
```

### Step 4: Additional Information

**Business Impact**:
```
Without the quota increase, our service cannot operate at production capacity.
We have implemented comprehensive quota monitoring and efficient API usage
patterns, but the default quota is insufficient for a production streaming
platform.
```

**Request Timeline**:
- High Priority
- Production service impacted by current quota limits

### Step 5: Submit and Wait

- Submit the form
- Google typically responds within 2-5 business days
- You may be asked for additional information
- Check your email for updates from Google Cloud

## Monitoring Quota Usage

Once approved, monitor your quota usage:

### Via Google Cloud Console

1. Go to: APIs & Services → YouTube Data API v3 → Quotas
2. View real-time usage graphs
3. Set up alerts for high usage

### Via Application Logs

Check the YouTube listener logs:
```bash
kubectl -n allchat logs -l app=youtube-listener --tail=100 | grep quota
```

### Via Database

Query the `youtube_quota_usage` table:
```sql
SELECT date, units_used, units_limit,
       ROUND(100.0 * units_used / units_limit, 2) as percentage
FROM youtube_quota_usage
ORDER BY date DESC
LIMIT 7;
```

## Quota Management Best Practices

### 1. Efficient Polling
- Use API-recommended polling intervals (typically 2-5 seconds)
- Don't poll more frequently than necessary
- Stop polling when streams end (now fixed)

### 2. Monitoring & Alerts
- Set alerts at 50%, 75%, 90% usage
- Log all API calls with quota costs
- Track quota per stream/channel

### 3. Graceful Degradation
- When quota approaches limit:
  - Increase polling intervals
  - Prioritize high-traffic streams
  - Queue low-priority requests

### 4. Quota Reset
- Quota resets daily at midnight Pacific Time (UTC-8 or UTC-7)
- Plan maintenance and testing around reset times

## Troubleshooting

### "Quota exceeded" Errors

**Check current usage**:
```bash
kubectl -n allchat exec allchat-cluster-1 -- psql -U postgres -d allchat -c \
  "SELECT * FROM youtube_quota_usage WHERE date = CURRENT_DATE;"
```

**Restart listeners** (clears zombie pollers):
```bash
kubectl -n allchat delete pods -l app=youtube-listener
```

### Quota Increase Denied

If your request is denied:
1. Review and improve your justification
2. Provide more details about your use case
3. Show evidence of legitimate usage
4. Demonstrate quota management practices
5. Consider starting with a smaller increase (e.g., 100,000 units/day)

### Alternative: Multiple Projects

If you need immediate relief:
- Create multiple Google Cloud projects
- Distribute streams across projects
- Each project gets 10,000 units/day
- **Not recommended for long-term**: Adds complexity

## Cost Implications

**YouTube Data API v3 is FREE** up to the quota limit. Quota increases don't incur additional costs. Google provides generous quotas for legitimate use cases.

## Related Documentation

- [YouTube Data API Quota Calculator](https://developers.google.com/youtube/v3/determine_quota_cost)
- [All-Chat YouTube Listener README](../services/youtube-listener/README.md)
- [Quota Tracking Implementation](../services/youtube-listener/quota/tracker.go)

## Support

If you encounter issues with the quota increase request:
- Google Cloud Support: https://cloud.google.com/support
- YouTube API Forum: https://stackoverflow.com/questions/tagged/youtube-data-api

---

**Last Updated**: 2025-11-20
**Status**: Requires quota increase to 1,000,000 units/day for production
