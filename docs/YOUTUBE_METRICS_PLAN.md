# YouTube API Quota Monitoring - Prometheus Metrics Plan

## Overview

Implement comprehensive Prometheus metrics for YouTube API quota tracking, enabling real-time monitoring, alerting, and capacity planning without complex service-to-service metric passing.

## Architecture

```
┌─────────────────────┐
│ YouTube Listener    │
│  - API Client       │──► Exposes /metrics endpoint
│  - Quota Tracker    │    (Prometheus format)
└─────────────────────┘
          │
          │ scrape
          ▼
┌─────────────────────┐
│   Prometheus        │──► Queries & Aggregations
└─────────────────────┘
          │
          ├──► Grafana (Dashboards)
          └──► AlertManager (Alerts)
```

## Metrics Design

### 1. Quota Usage Metrics

#### Counter: `youtube_api_quota_total`
Total YouTube API quota units consumed since service start.

**Labels**:
- `operation`: API operation type (`search_list`, `videos_list`, `live_chat_messages_list`, `channels_list`)
- `service`: Service name (`youtube-listener`)
- `result`: `success` or `error`

**Example**:
```
youtube_api_quota_total{operation="search_list",service="youtube-listener",result="success"} 15000
youtube_api_quota_total{operation="live_chat_messages_list",service="youtube-listener",result="success"} 55500
youtube_api_quota_total{operation="search_list",service="youtube-listener",result="error"} 300
```

**Queries**:
- Total daily quota usage: `sum(increase(youtube_api_quota_total[24h]))`
- Quota by operation: `sum by (operation) (increase(youtube_api_quota_total[24h]))`
- Error rate: `rate(youtube_api_quota_total{result="error"}[5m])`

---

#### Gauge: `youtube_api_quota_remaining`
Remaining quota units for today.

**Labels**:
- `service`: Service name
- `limit`: Daily limit (10000 or 1000000)

**Example**:
```
youtube_api_quota_remaining{service="youtube-listener",limit="10000"} 2450
```

**Queries**:
- Current remaining: `youtube_api_quota_remaining`
- Percentage used: `(1 - youtube_api_quota_remaining / youtube_api_quota_remaining{limit}) * 100`

---

#### Gauge: `youtube_api_quota_usage_percentage`
Current quota usage as a percentage (0-100).

**Labels**:
- `service`: Service name

**Example**:
```
youtube_api_quota_usage_percentage{service="youtube-listener"} 75.5
```

---

### 2. Stream-Level Metrics

#### Gauge: `youtube_active_streams_total`
Number of currently active streams being polled.

**Labels**:
- `service`: Service name

**Example**:
```
youtube_active_streams_total{service="youtube-listener"} 5
```

---

#### Counter: `youtube_stream_api_calls_total`
Total API calls per stream.

**Labels**:
- `stream_id`: YouTube video ID
- `channel_id`: YouTube channel ID
- `channel_name`: Channel display name
- `operation`: API operation type
- `result`: `success` or `error`

**Example**:
```
youtube_stream_api_calls_total{stream_id="abc123",channel_id="UCxxx",channel_name="StreamerName",operation="live_chat_messages_list",result="success"} 1200
```

**Queries**:
- API calls per stream: `sum by (stream_id, channel_name) (rate(youtube_stream_api_calls_total[5m]))`
- Top quota consumers: `topk(10, sum by (channel_name) (increase(youtube_stream_api_calls_total[1h])))`

---

#### Histogram: `youtube_stream_poll_duration_seconds`
Duration of stream polling operations.

**Labels**:
- `stream_id`: YouTube video ID
- `operation`: `poll` or `discovery`

**Buckets**: `[0.1, 0.5, 1, 2, 5, 10, 30]`

**Example**:
```
youtube_stream_poll_duration_seconds_bucket{stream_id="abc123",operation="poll",le="1"} 850
youtube_stream_poll_duration_seconds_bucket{stream_id="abc123",operation="poll",le="2"} 980
youtube_stream_poll_duration_seconds_sum{stream_id="abc123",operation="poll"} 1245.3
youtube_stream_poll_duration_seconds_count{stream_id="abc123",operation="poll"} 1000
```

**Queries**:
- Average poll latency: `rate(youtube_stream_poll_duration_seconds_sum[5m]) / rate(youtube_stream_poll_duration_seconds_count[5m])`
- 99th percentile: `histogram_quantile(0.99, rate(youtube_stream_poll_duration_seconds_bucket[5m]))`

---

### 3. User/Channel Metrics

#### Counter: `youtube_channel_discovery_total`
Number of channel discovery attempts (search.list calls).

**Labels**:
- `channel_id`: YouTube channel ID
- `channel_name`: Channel display name
- `result`: `found_streams` or `no_streams` or `error`

**Example**:
```
youtube_channel_discovery_total{channel_id="UCxxx",channel_name="StreamerName",result="found_streams"} 150
youtube_channel_discovery_total{channel_id="UCxxx",channel_name="StreamerName",result="no_streams"} 2800
```

**Queries**:
- Channels with most discoveries: `topk(10, sum by (channel_name) (increase(youtube_channel_discovery_total[24h])))`
- Discovery success rate: `rate(youtube_channel_discovery_total{result="found_streams"}[5m]) / rate(youtube_channel_discovery_total[5m])`

---

### 4. Error & Health Metrics

#### Counter: `youtube_api_errors_total`
Total API errors by type.

**Labels**:
- `error_type`: `quota_exceeded`, `live_chat_ended`, `invalid_credentials`, `not_found`, `rate_limited`, `other`
- `operation`: API operation type
- `stream_id`: YouTube video ID (optional)

**Example**:
```
youtube_api_errors_total{error_type="live_chat_ended",operation="live_chat_messages_list",stream_id="abc123"} 1
youtube_api_errors_total{error_type="quota_exceeded",operation="search_list"} 5
```

**Queries**:
- Error rate by type: `sum by (error_type) (rate(youtube_api_errors_total[5m]))`
- Quota exceeded events: `increase(youtube_api_errors_total{error_type="quota_exceeded"}[1h])`

---

#### Gauge: `youtube_poller_active`
Whether a specific poller is active (1) or stopped (0).

**Labels**:
- `stream_id`: YouTube video ID
- `channel_id`: YouTube channel ID

**Example**:
```
youtube_poller_active{stream_id="abc123",channel_id="UCxxx"} 1
youtube_poller_active{stream_id="def456",channel_id="UCyyy"} 0
```

**Queries**:
- Active pollers: `sum(youtube_poller_active)`
- Zombie pollers (should be 0 after fix): Pollers active longer than expected

---

### 5. Message Processing Metrics

#### Counter: `youtube_messages_received_total`
Total chat messages received from YouTube.

**Labels**:
- `stream_id`: YouTube video ID
- `channel_id`: YouTube channel ID

**Example**:
```
youtube_messages_received_total{stream_id="abc123",channel_id="UCxxx"} 15420
```

**Queries**:
- Messages per second: `rate(youtube_messages_received_total[1m])`
- Top chatty streams: `topk(10, rate(youtube_messages_received_total[5m]))`

---

#### Histogram: `youtube_message_processing_duration_seconds`
Time to process messages after receiving from API.

**Labels**:
- `stream_id`: YouTube video ID

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]`

---

### 6. Quota Efficiency Metrics

#### Gauge: `youtube_quota_efficiency_ratio`
Messages received per quota unit spent.

**Calculation**: `messages_received / quota_spent`

**Labels**:
- `stream_id`: YouTube video ID

**Example**:
```
youtube_quota_efficiency_ratio{stream_id="abc123"} 2.8
```

**Queries**:
- Most efficient streams: `topk(10, youtube_quota_efficiency_ratio)`
- Least efficient streams: `bottomk(10, youtube_quota_efficiency_ratio)`

---

## Grafana Dashboards

### Dashboard 1: YouTube Quota Overview

**Panels**:

1. **Quota Usage Gauge** (top-left, large)
   - Current percentage used
   - Color zones: Green (0-50%), Yellow (50-80%), Orange (80-90%), Red (90-100%)

2. **Remaining Quota** (top-right, large)
   - Current remaining units
   - Shows daily limit

3. **Quota Usage Over Time** (full-width)
   - Line graph showing quota consumption throughout the day
   - Query: `sum(increase(youtube_api_quota_total[5m]))`

4. **Quota by Operation** (pie chart)
   - Breakdown: search.list, videos.list, liveChatMessages.list
   - Query: `sum by (operation) (increase(youtube_api_quota_total[24h]))`

5. **Projected Time to Quota Exhaustion** (single stat)
   - Calculates based on current consumption rate
   - Query: `(youtube_api_quota_remaining) / (rate(youtube_api_quota_total[1h]) * 3600)`

6. **API Call Rate** (graph)
   - Calls per minute by operation
   - Query: `sum by (operation) (rate(youtube_api_quota_total[1m]) / 5)` (divide by cost)

---

### Dashboard 2: Stream Monitoring

**Panels**:

1. **Active Streams Count** (single stat)
   - Query: `youtube_active_streams_total`

2. **Active Streams Table**
   - Columns: Stream ID, Channel Name, Messages/min, API Calls/min, Uptime
   - Sortable by activity

3. **Top Quota Consumers** (bar chart)
   - Top 10 streams by quota usage
   - Query: `topk(10, sum by (stream_id, channel_name) (increase(youtube_stream_api_calls_total[1h]) * 5))`

4. **Stream Message Rate** (graph)
   - Messages per second per stream
   - Query: `sum by (stream_id, channel_name) (rate(youtube_messages_received_total[1m]))`

5. **Polling Latency** (graph)
   - P50, P95, P99 latencies
   - Query: `histogram_quantile(0.99, rate(youtube_stream_poll_duration_seconds_bucket[5m]))`

6. **Quota Efficiency** (heatmap)
   - Efficiency ratio per stream over time

---

### Dashboard 3: Error Monitoring

**Panels**:

1. **Error Rate** (single stat)
   - Errors per minute
   - Query: `sum(rate(youtube_api_errors_total[5m]))`

2. **Errors by Type** (bar chart)
   - Query: `sum by (error_type) (increase(youtube_api_errors_total[1h]))`

3. **Error Timeline** (graph)
   - Stacked area chart showing error types over time

4. **Quota Exceeded Events** (single stat)
   - Count of quota exceeded errors in last 24h
   - Query: `sum(increase(youtube_api_errors_total{error_type="quota_exceeded"}[24h]))`

5. **Stream End Events** (graph)
   - liveChatEnded errors (should trigger poller stop)
   - Query: `increase(youtube_api_errors_total{error_type="live_chat_ended"}[5m])`

6. **Failed API Calls** (table)
   - Recent failed operations with details

---

### Dashboard 4: Capacity Planning

**Panels**:

1. **Daily Quota Trend** (7-day)
   - Daily quota consumption for past week
   - Helps identify patterns

2. **Peak Usage Hours** (heatmap)
   - Hour of day vs. day of week

3. **Concurrent Streams Capacity** (calculation)
   - Shows how many streams can be supported with current quota
   - Formula: `remaining_quota / (5 units * 1800 calls/hour)`

4. **Quota Runway** (gauge)
   - Days until quota runs out at current rate
   - Shows when to increase polling intervals

5. **Cost per Stream** (table)
   - Average quota units consumed per stream per hour

6. **Forecasted Usage** (graph)
   - Projected quota usage for next 24h based on trends

---

## Prometheus Alerts

### Alert 1: High Quota Usage

```yaml
- alert: YouTubeQuotaHigh
  expr: youtube_api_quota_usage_percentage > 75
  for: 5m
  labels:
    severity: warning
    component: youtube-listener
  annotations:
    summary: "YouTube API quota usage is high"
    description: "Quota usage is at {{ $value }}%. Consider increasing polling intervals or requesting quota increase."
```

---

### Alert 2: Critical Quota Usage

```yaml
- alert: YouTubeQuotaCritical
  expr: youtube_api_quota_usage_percentage > 90
  for: 2m
  labels:
    severity: critical
    component: youtube-listener
  annotations:
    summary: "YouTube API quota usage is critical"
    description: "Quota usage is at {{ $value }}%. Service degradation imminent. Increase polling intervals immediately."
```

---

### Alert 3: Quota Exceeded

```yaml
- alert: YouTubeQuotaExceeded
  expr: increase(youtube_api_errors_total{error_type="quota_exceeded"}[5m]) > 0
  labels:
    severity: critical
    component: youtube-listener
  annotations:
    summary: "YouTube API quota exceeded"
    description: "YouTube API quota has been exceeded. All API calls will fail until quota resets at midnight PT."
```

---

### Alert 4: High Error Rate

```yaml
- alert: YouTubeAPIHighErrorRate
  expr: rate(youtube_api_errors_total[5m]) > 1
  for: 10m
  labels:
    severity: warning
    component: youtube-listener
  annotations:
    summary: "High YouTube API error rate"
    description: "YouTube API error rate is {{ $value }} errors/sec. Check service health and API status."
```

---

### Alert 5: Zombie Poller Detected

```yaml
- alert: YouTubeZombiePollerDetected
  expr: |
    sum by (stream_id) (
      increase(youtube_api_errors_total{error_type="live_chat_ended"}[5m])
    ) > 10
  labels:
    severity: warning
    component: youtube-listener
  annotations:
    summary: "Potential zombie poller detected"
    description: "Stream {{ $labels.stream_id }} has received 10+ liveChatEnded errors in 5 minutes. Poller may not have stopped properly."
```

---

### Alert 6: No Active Streams (Optional)

```yaml
- alert: YouTubeNoActiveStreams
  expr: youtube_active_streams_total == 0 and on() hour() >= 18 and hour() <= 23
  for: 30m
  labels:
    severity: info
    component: youtube-listener
  annotations:
    summary: "No active YouTube streams during peak hours"
    description: "No streams are being monitored during typical streaming hours (6PM-11PM). This may be expected."
```

---

### Alert 7: High Polling Latency

```yaml
- alert: YouTubeHighPollingLatency
  expr: |
    histogram_quantile(0.95,
      rate(youtube_stream_poll_duration_seconds_bucket[5m])
    ) > 5
  for: 10m
  labels:
    severity: warning
    component: youtube-listener
  annotations:
    summary: "High YouTube polling latency"
    description: "95th percentile polling latency is {{ $value }}s. YouTube API may be slow or service is overloaded."
```

---

### Alert 8: Rapid Quota Consumption

```yaml
- alert: YouTubeQuotaConsumptionRapid
  expr: |
    (
      increase(youtube_api_quota_total[1h]) /
      youtube_api_quota_remaining{limit}
    ) * 100 > 10
  labels:
    severity: warning
    component: youtube-listener
  annotations:
    summary: "Rapid YouTube quota consumption"
    description: "Consumed 10% of remaining quota in last hour. At this rate, quota will be exhausted in {{ $value }} hours."
```

---

## Implementation Plan

### Phase 1: Core Metrics (Week 1)

**Tasks**:
1. Create `metrics` package in `services/youtube-listener/metrics/`
2. Implement Prometheus metrics using `prometheus/client_golang`
3. Add metrics to API client:
   - Instrument `GetLiveStreams()`, `GetChatMessages()`, `GetVideoDetails()`
   - Record quota costs with labels
4. Add `/metrics` endpoint to YouTube listener HTTP server
5. Update quota tracker to export metrics instead of just DB writes

**Files to Create/Modify**:
- `services/youtube-listener/metrics/metrics.go` (new)
- `services/youtube-listener/api/client.go` (modify)
- `services/youtube-listener/cmd/main.go` (add /metrics endpoint)

**Example Code Structure**:
```go
// services/youtube-listener/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    QuotaTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "youtube_api_quota_total",
            Help: "Total YouTube API quota units consumed",
        },
        []string{"operation", "service", "result"},
    )

    QuotaRemaining = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "youtube_api_quota_remaining",
            Help: "Remaining YouTube API quota units",
        },
        []string{"service", "limit"},
    )

    // ... more metrics
)

func RecordQuotaUsage(operation string, units int, err error) {
    result := "success"
    if err != nil {
        result = "error"
    }
    QuotaTotal.WithLabelValues(operation, "youtube-listener", result).Add(float64(units))
}
```

---

### Phase 2: Stream Metrics (Week 1-2)

**Tasks**:
1. Add stream-level metrics to poller
2. Track API calls per stream
3. Add message rate metrics
4. Implement polling duration histograms

**Files to Modify**:
- `services/youtube-listener/streams/poller.go`
- `services/youtube-listener/streams/manager.go`

---

### Phase 3: Prometheus Configuration (Week 2)

**Tasks**:
1. Add Prometheus scrape config for YouTube listener
2. Configure service discovery (Kubernetes annotations)
3. Set up recording rules for aggregations
4. Test metric collection

**Files to Create**:
- `deployments/k8s/base/youtube-listener/servicemonitor.yaml` (if using Prometheus Operator)
- `deployments/prometheus/youtube-rules.yaml` (recording rules)

**Example ServiceMonitor**:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: youtube-listener
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: youtube-listener
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

---

### Phase 4: Grafana Dashboards (Week 2-3)

**Tasks**:
1. Create JSON dashboard definitions
2. Import into Grafana
3. Set up variables for filtering (stream, channel, time range)
4. Create dashboard links between related views

**Files to Create**:
- `deployments/grafana/dashboards/youtube-quota-overview.json`
- `deployments/grafana/dashboards/youtube-streams.json`
- `deployments/grafana/dashboards/youtube-errors.json`
- `deployments/grafana/dashboards/youtube-capacity.json`

---

### Phase 5: AlertManager Rules (Week 3)

**Tasks**:
1. Create alert rule definitions
2. Configure AlertManager routing
3. Set up notification channels (Slack, PagerDuty, email)
4. Test alert firing and resolution

**Files to Create**:
- `deployments/prometheus/youtube-alerts.yaml`
- `deployments/alertmanager/youtube-routes.yaml`

---

### Phase 6: Documentation & Runbook (Week 3-4)

**Tasks**:
1. Document metrics and their meanings
2. Create runbook for alert responses
3. Add example PromQL queries
4. Create troubleshooting guide

**Files to Create**:
- `docs/YOUTUBE_METRICS.md`
- `docs/runbooks/youtube-quota-exceeded.md`
- `docs/runbooks/youtube-high-error-rate.md`

---

## Testing Strategy

### Unit Tests
- Test metric recording logic
- Verify labels are correct
- Ensure counters increment properly

### Integration Tests
- Start listener with test streams
- Query Prometheus for expected metrics
- Verify values match actual API calls

### Load Tests
- Simulate 20 concurrent streams
- Measure metric overhead (should be < 1ms per recording)
- Verify no metric cardinality explosion

---

## Metric Cardinality Management

### High Cardinality Concerns

**Potential Issues**:
- `stream_id`: Could be hundreds of unique values
- `channel_name`: Could be hundreds of unique channels

**Mitigation**:
1. Use recording rules to pre-aggregate high-cardinality metrics
2. Set retention limits on per-stream metrics (7 days)
3. Use `limit` label aggregation where possible
4. Consider dropping inactive stream labels after 24h

**Recording Rules Example**:
```yaml
groups:
- name: youtube_aggregations
  interval: 30s
  rules:
  - record: youtube:quota:rate5m
    expr: rate(youtube_api_quota_total[5m])

  - record: youtube:quota:daily
    expr: sum(increase(youtube_api_quota_total[24h]))

  - record: youtube:streams:active
    expr: count(youtube_poller_active == 1)
```

---

## Success Criteria

### Phase 1-2 Success
- [ ] All core metrics exposed on `/metrics` endpoint
- [ ] Quota usage tracked accurately
- [ ] Metrics match database quota records

### Phase 3-4 Success
- [ ] Prometheus scraping metrics every 30s
- [ ] Grafana dashboards show real-time data
- [ ] Can drill down from overview to specific streams

### Phase 5 Success
- [ ] Alerts fire correctly when thresholds exceeded
- [ ] No false positives in 1 week of monitoring
- [ ] Alert notifications delivered to correct channels

### Overall Success
- [ ] Can identify quota issues within 5 minutes
- [ ] Historical data enables capacity planning
- [ ] Reduced MTTD (Mean Time To Detect) for issues from hours to minutes

---

## Maintenance & Evolution

### Regular Reviews
- **Weekly**: Review top quota consumers, adjust polling if needed
- **Monthly**: Analyze capacity trends, plan for growth
- **Quarterly**: Review alert thresholds, update dashboards

### Metric Lifecycle
- **New Metrics**: Add via feature flags, measure overhead
- **Deprecated Metrics**: Mark deprecated, remove after 2 releases
- **Changed Metrics**: Use new metric name, keep old for 1 release

---

## References

- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
- [YouTube Data API Costs](https://developers.google.com/youtube/v3/determine_quota_cost)
- [RED Method Monitoring](https://grafana.com/blog/2018/08/02/the-red-method-how-to-instrument-your-services/)

---

**Last Updated**: 2025-11-20
**Status**: Plan ready for implementation
**Estimated Effort**: 3-4 weeks for complete implementation
