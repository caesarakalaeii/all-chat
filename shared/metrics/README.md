# All-Chat Metrics Implementation

## Overview

This package provides Prometheus metrics for the All-Chat platform following the observability strategy outlined in `/docs/OBSERVABILITY_STRATEGY.md`.

## Current Implementation Status

### ✅ Completed
- **Metrics Package Created**: Common metrics patterns for listeners, processors, gateway, and business metrics
- **Twitch Listener**: `/metrics` endpoint added

### 🚧 In Progress
- Adding `/metrics` endpoints to remaining services
- Integrating metrics recording into service components

### 📋 TODO
- Wire up metrics recording in all listener components (IRC, WebSocket, HTTP polling)
- Add metrics recording to Message Processor pipeline
- Add metrics recording to API Gateway WebSocket handling
- Implement business metrics tracking (active overlays, users, etc.)
- Add platform-specific metrics (Twitch IRC commands, YouTube quota, etc.)

## Quick Start

### Adding Metrics to a Service

1. **Import the metrics package in your service's `cmd/main.go`:**

```go
import (
    "github.com/caesar/all-chat/shared/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)
```

2. **Initialize metrics (choose the appropriate type):**

```go
// For listeners (Twitch, YouTube, Kick, TikTok)
listenerMetrics := metrics.NewListenerMetrics("platform-name", "service-name")

// For message processor
processorMetrics := metrics.NewProcessorMetrics()

// For API gateway
gatewayMetrics := metrics.NewGatewayMetrics()

// For business metrics
businessMetrics := metrics.NewBusinessMetrics()
```

3. **Add the /metrics endpoint to your HTTP router:**

```go
router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

4. **Record metrics in your components:**

```go
// Example: Recording a message received
listenerMetrics.RecordMessage("twitch", "twitch-listener", channelID, "chat")

// Example: Recording connection status
listenerMetrics.RecordConnection("twitch", "twitch-listener", "irc", true)

// Example: Recording API call
listenerMetrics.RecordAPICall("youtube", "youtube-listener", "list_videos", "success", "")
```

## Metric Types

### Listener Metrics

All platform listeners share these common metrics:

- **Connection Health**: Connection status, attempts, duration
- **Source Monitoring**: Active sources, lifecycle events
- **Message Ingestion**: Messages received/published, latency, rate
- **Rate Limiting**: Rate limit hits, quota remaining/usage
- **API Calls**: Total calls, duration by operation
- **Errors**: Error counts by category and severity

### Processor Metrics

Message processor metrics:

- **Processing Pipeline**: Messages consumed/processed, duration by stage
- **Emote Enrichment**: Lookups, cache operations, enrichment duration
- **Stream Health**: Stream lag, errors
- **Routing**: Messages published to overlays, fanout duration

### Gateway Metrics

API Gateway metrics:

- **WebSocket Connections**: Active connections, total attempts, duration
- **Overlay Subscriptions**: Active subscriptions, lifecycle events
- **Message Distribution**: Messages received/sent/dropped, delivery latency
- **HTTP Endpoints**: Request counts, duration

### Business Metrics

Platform-level business metrics:

- **User Engagement**: Active overlays, views, session duration
- **Platform Usage**: Messages by platform, connected platforms per user
- **Source Management**: Active sources, operations

## Prometheus Configuration

Add these scrape configs to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'twitch-listener'
    static_configs:
      - targets: ['twitch-listener:8085']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'youtube-listener'
    static_configs:
      - targets: ['youtube-listener:8086']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'kick-listener'
    static_configs:
      - targets: ['kick-listener:8089']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'message-processor'
    static_configs:
      - targets: ['message-processor:8087']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'api-gateway'
    static_configs:
      - targets: ['api-gateway:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'source-manager'
    static_configs:
      - targets: ['source-manager:8088']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Grafana Dashboards

See `/grafana-dashboards/` directory for:

- **Platform Overview**: High-level health and activity
- **Listener Health**: Per-platform listener metrics
- **Message Pipeline**: Processing performance
- **Business Metrics**: User engagement and platform usage

## Implementation Priority

Follow this order for rolling out metrics:

### Phase 1: Critical Path (Week 1)
1. ✅ Metrics package created
2. ✅ Twitch Listener `/metrics` endpoint
3. 🚧 Add `/metrics` to all services
4. Wire up connection status metrics in all listeners
5. Wire up message flow metrics (received → processed → delivered)

### Phase 2: Platform Health (Week 2)
1. YouTube quota tracking metrics
2. Twitch rate limit metrics
3. Kick/TikTok connection stability metrics
4. Error tracking across all services

### Phase 3: Performance (Week 3)
1. End-to-end latency tracking
2. Processing stage duration
3. Emote enrichment performance
4. Database query performance

### Phase 4: Business Insights (Week 4)
1. Active overlay tracking
2. Platform usage distribution
3. User engagement metrics
4. Capacity planning metrics

## Testing

Test that metrics are exposed:

```bash
# Check if endpoint is available
curl http://localhost:8085/metrics

# Verify specific metrics exist
curl http://localhost:8085/metrics | grep listener_connection_status

# Check metric values
curl http://localhost:8085/metrics | grep listener_messages_received_total
```

## Performance Considerations

- Metrics are designed for <1ms overhead per recording
- Use histogram buckets tuned for expected latencies
- Limit high-cardinality labels (avoid user IDs, message IDs, etc.)
- Pre-aggregate metrics using Prometheus recording rules if needed

## Support

For questions or issues with metrics implementation:
1. Review `/docs/OBSERVABILITY_STRATEGY.md` for full metric specifications
2. Check existing implementations in `services/twitch-listener/`
3. Refer to Prometheus best practices: https://prometheus.io/docs/practices/naming/
