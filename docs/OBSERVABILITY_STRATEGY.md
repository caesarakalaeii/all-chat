# All-Chat Observability Strategy

## Overview

Comprehensive Prometheus metrics and monitoring strategy for all All-Chat services. Enables real-time monitoring, alerting, debugging, and capacity planning across the entire platform.

## Design Principles

1. **Common Patterns**: Shared metric patterns across similar services (all listeners)
2. **Platform-Agnostic**: Abstract platform differences where possible
3. **Correlation**: Enable tracing flows across services (overlay → source → listener → processor → gateway)
4. **Business Metrics**: Track user-facing metrics, not just technical ones
5. **Low Overhead**: Metric collection must not impact performance (<1ms per recording)
6. **Cardinality Control**: Limit high-cardinality labels to prevent Prometheus overload

---

## Service Metrics Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Request                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│  API Gateway                                                     │
│  - WebSocket connections                                         │
│  - Message throughput (in/out)                                   │
│  - Connection duration                                           │
│  - Overlay subscriptions                                         │
└──────────────────┬──────────────────────────────────────────────┘
                   │
                   ├──► Auth Service
                   │    - OAuth flows
                   │    - Token operations
                   │    - Platform connections
                   │
                   └──► Overlay Manager
                        - CRUD operations
                        - Source management
                        - Active overlays

┌─────────────────────────────────────────────────────────────────┐
│  Listeners (Twitch, YouTube, Kick, TikTok)                      │
│  - Platform API calls                                            │
│  - Connection health                                             │
│  - Message ingestion rate                                        │
│  - Rate limiting / quota                                         │
└──────────────────┬──────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│  Message Processor                                               │
│  - Processing pipeline                                           │
│  - Normalization performance                                     │
│  - Emote enrichment                                              │
│  - Routing efficiency                                            │
└──────────────────┬──────────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│  Redis (Streams + Pub/Sub)                                       │
│  - Stream lag                                                    │
│  - Consumer group status                                         │
│  - Pub/Sub subscriptions                                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Common Listener Metrics Pattern

All listeners (Twitch, YouTube, Kick, TikTok) share these base metrics with platform-specific labels.

### 1.1 Connection Health

#### Gauge: `listener_connection_status`
Connection status to platform (1 = connected, 0 = disconnected).

**Labels**:
- `platform`: `twitch`, `youtube`, `kick`, `tiktok`
- `service`: Service name
- `connection_type`: `irc`, `websocket`, `http_poll`, `grpc`

---

#### Counter: `listener_connection_attempts_total`
Total connection attempts.

**Labels**:
- `platform`, `service`
- `result`: `success`, `failed`, `timeout`, `rate_limited`

---

#### Histogram: `listener_connection_duration_seconds`
Duration of connection uptime before disconnect.

**Labels**:
- `platform`, `service`
- `disconnect_reason`: `normal`, `error`, `rate_limit`, `timeout`

**Buckets**: `[60, 300, 900, 1800, 3600, 7200, 14400, 28800]` (1min to 8hr)

---

### 1.2 Channel/Source Monitoring

#### Gauge: `listener_active_sources_total`
Number of currently monitored channels/sources.

**Labels**:
- `platform`, `service`

---

#### Counter: `listener_source_events_total`
Source lifecycle events.

**Labels**:
- `platform`, `service`
- `event`: `added`, `removed`, `discovered`, `lost`, `reconnected`

---

### 1.3 Message Ingestion

#### Counter: `listener_messages_received_total`
Total messages received from platform.

**Labels**:
- `platform`, `service`
- `channel_id`: Channel/room ID
- `message_type`: `chat`, `subscription`, `donation`, `raid`, `host`, `system`

---

#### Counter: `listener_messages_published_total`
Total messages published to Redis.

**Labels**:
- `platform`, `service`
- `result`: `success`, `failed`

---

#### Histogram: `listener_message_latency_seconds`
Time from receiving message from platform to publishing to Redis.

**Labels**:
- `platform`, `service`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]`

---

#### Gauge: `listener_message_rate_per_second`
Current message ingestion rate (rolling average).

**Labels**:
- `platform`, `service`
- `channel_id`: Channel/room ID

---

### 1.4 Rate Limiting & Quotas

#### Counter: `listener_rate_limit_hits_total`
Number of times rate limit was hit.

**Labels**:
- `platform`, `service`
- `limit_type`: `api_quota`, `connection_rate`, `message_rate`, `join_rate`

---

#### Gauge: `listener_quota_remaining`
Remaining quota/rate limit capacity.

**Labels**:
- `platform`, `service`
- `quota_type`: `daily`, `hourly`, `per_second`
- `limit`: Max limit value

---

#### Gauge: `listener_quota_usage_percentage`
Current quota usage as percentage (0-100).

**Labels**:
- `platform`, `service`
- `quota_type`

---

### 1.5 Platform API Calls

#### Counter: `listener_api_calls_total`
Total API calls to platform.

**Labels**:
- `platform`, `service`
- `operation`: API operation name
- `result`: `success`, `error`
- `error_type`: `auth`, `rate_limit`, `not_found`, `server_error`, `timeout`

---

#### Histogram: `listener_api_call_duration_seconds`
Duration of API calls.

**Labels**:
- `platform`, `service`
- `operation`

**Buckets**: `[0.1, 0.5, 1, 2, 5, 10, 30]`

---

### 1.6 Error Tracking

#### Counter: `listener_errors_total`
Total errors by category.

**Labels**:
- `platform`, `service`
- `error_category`: `connection`, `authentication`, `parsing`, `rate_limit`, `api`, `internal`
- `severity`: `info`, `warning`, `error`, `critical`

---

## 2. Platform-Specific Metrics

### 2.1 Twitch Listener (IRC-based)

#### Gauge: `twitch_irc_channels_joined`
Number of IRC channels currently joined.

**Labels**:
- `service`

---

#### Counter: `twitch_irc_commands_sent_total`
IRC commands sent.

**Labels**:
- `service`
- `command`: `JOIN`, `PART`, `PRIVMSG`, `PING`, `PONG`

---

#### Counter: `twitch_irc_messages_by_type_total`
Messages by IRC message type.

**Labels**:
- `service`
- `type`: `PRIVMSG`, `USERNOTICE`, `CLEARCHAT`, `CLEARMSG`, `RECONNECT`

---

#### Histogram: `twitch_join_rate_limiter_delay_seconds`
Delay imposed by join rate limiter (20 joins per 10s).

**Labels**:
- `service`

**Buckets**: `[0, 0.5, 1, 2, 5, 10, 20]`

---

### 2.2 YouTube Listener (HTTP Polling)

**Use metrics from YOUTUBE_METRICS_PLAN.md**, plus:

#### Histogram: `youtube_poll_interval_seconds`
Actual polling intervals used (from API response).

**Labels**:
- `service`
- `stream_id`

**Buckets**: `[1, 2, 3, 5, 7, 10, 15]`

---

#### Counter: `youtube_stream_lifecycle_events_total`
Stream lifecycle events.

**Labels**:
- `service`
- `event`: `started`, `ended`, `discovered`, `lost`

---

### 2.3 Kick Listener (Pusher WebSocket)

#### Gauge: `kick_websocket_active`
WebSocket connection status (1 = active, 0 = inactive).

**Labels**:
- `service`

---

#### Counter: `kick_pusher_events_total`
Pusher protocol events.

**Labels**:
- `service`
- `event_type`: `subscription_succeeded`, `subscription_error`, `message`, `ping`, `pong`

---

#### Counter: `kick_channel_subscriptions_total`
Chatroom channel subscriptions.

**Labels**:
- `service`
- `result`: `success`, `failed`

---

#### Histogram: `kick_message_receive_delay_seconds`
Delay between message sent (timestamp) and received.

**Labels**:
- `service`

**Buckets**: `[0.05, 0.1, 0.2, 0.5, 1, 2, 5]`

---

### 2.4 TikTok Listener

#### Gauge: `tiktok_websocket_active`
WebSocket connection status.

**Labels**:
- `service`

---

#### Counter: `tiktok_proto_messages_total`
Protocol buffer messages received.

**Labels**:
- `service`
- `message_type`: `chat`, `gift`, `member`, `like`, `social`

---

#### Histogram: `tiktok_connection_stability_seconds`
Time between reconnection events.

**Labels**:
- `service`

**Buckets**: `[60, 300, 900, 1800, 3600, 7200]`

---

## 3. Message Processor Metrics

### 3.1 Processing Pipeline

#### Counter: `processor_messages_consumed_total`
Messages consumed from Redis stream `chat:raw`.

**Labels**:
- `service`
- `platform`: Source platform
- `consumer_group`

---

#### Counter: `processor_messages_processed_total`
Messages processed through pipeline.

**Labels**:
- `service`
- `platform`
- `stage`: `consumed`, `normalized`, `enriched`, `routed`, `published`
- `result`: `success`, `failed`

---

#### Histogram: `processor_message_processing_duration_seconds`
End-to-end message processing time.

**Labels**:
- `service`
- `platform`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]`

---

#### Histogram: `processor_stage_duration_seconds`
Duration of each processing stage.

**Labels**:
- `service`
- `platform`
- `stage`: `normalization`, `emote_enrichment`, `routing`

**Buckets**: `[0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1]`

---

### 3.2 Emote Enrichment

#### Counter: `processor_emote_lookups_total`
Emote provider API lookups.

**Labels**:
- `service`
- `provider`: `7tv`, `bttv`, `ffz`, `twitch`, `youtube`
- `result`: `success`, `cached`, `failed`, `timeout`

---

#### Gauge: `processor_emote_cache_entries`
Number of entries in emote cache.

**Labels**:
- `service`
- `provider`

---

#### Counter: `processor_emote_cache_operations_total`
Cache operations.

**Labels**:
- `service`
- `operation`: `hit`, `miss`, `eviction`
- `provider`

---

#### Histogram: `processor_emote_enrichment_duration_seconds`
Time to enrich message with emotes.

**Labels**:
- `service`
- `emote_count_bucket`: `0`, `1-5`, `6-10`, `11+`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1]`

---

### 3.3 Stream Health

#### Gauge: `processor_stream_lag_seconds`
Lag in consuming from Redis stream (time since last message).

**Labels**:
- `service`
- `stream`: `chat:raw`
- `consumer_group`

---

#### Counter: `processor_stream_errors_total`
Stream consumption errors.

**Labels**:
- `service`
- `error_type`: `connection`, `timeout`, `parse`, `claim`

---

### 3.4 Routing & Publishing

#### Counter: `processor_messages_published_total`
Messages published to overlay-specific pub/sub channels.

**Labels**:
- `service`
- `overlay_id`
- `platform`
- `result`: `success`, `failed`

---

#### Histogram: `processor_fanout_duration_seconds`
Time to publish message to all overlay subscribers.

**Labels**:
- `service`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5]`

---

## 4. API Gateway Metrics

### 4.1 WebSocket Connections

#### Gauge: `gateway_websocket_connections_active`
Number of active WebSocket connections.

**Labels**:
- `service`
- `connection_type`: `overlay`, `admin`

---

#### Counter: `gateway_websocket_connections_total`
Total WebSocket connection attempts.

**Labels**:
- `service`
- `result`: `success`, `auth_failed`, `rate_limited`, `invalid`

---

#### Histogram: `gateway_websocket_connection_duration_seconds`
Duration of WebSocket connections.

**Labels**:
- `service`
- `disconnect_reason`: `normal`, `timeout`, `error`, `client_close`

**Buckets**: `[60, 300, 900, 1800, 3600, 7200, 14400, 28800, 43200, 86400]`

---

### 4.2 Overlay Subscriptions

#### Gauge: `gateway_overlay_subscriptions_active`
Number of active overlay subscriptions.

**Labels**:
- `service`
- `overlay_id`

---

#### Counter: `gateway_overlay_subscription_events_total`
Subscription lifecycle events.

**Labels**:
- `service`
- `event`: `subscribed`, `unsubscribed`, `resubscribed`

---

### 4.3 Message Distribution

#### Counter: `gateway_messages_received_total`
Messages received from Redis pub/sub.

**Labels**:
- `service`
- `overlay_id`
- `platform`

---

#### Counter: `gateway_messages_sent_total`
Messages sent to WebSocket clients.

**Labels**:
- `service`
- `overlay_id`
- `result`: `success`, `failed`, `dropped`

---

#### Histogram: `gateway_message_delivery_latency_seconds`
Time from receiving message from Redis to sending via WebSocket.

**Labels**:
- `service`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]`

---

#### Counter: `gateway_messages_dropped_total`
Messages dropped (client disconnected, buffer full, etc.).

**Labels**:
- `service`
- `reason`: `client_disconnected`, `buffer_full`, `send_timeout`, `marshal_error`

---

### 4.4 HTTP Endpoints

#### Counter: `gateway_http_requests_total`
HTTP requests to gateway endpoints.

**Labels**:
- `service`
- `method`: HTTP method
- `path`: Request path
- `status`: HTTP status code

---

#### Histogram: `gateway_http_request_duration_seconds`
HTTP request duration.

**Labels**:
- `service`
- `method`
- `path`

**Buckets**: `[0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]`

---

## 5. Auth Service Metrics

### 5.1 OAuth Flows

#### Counter: `auth_oauth_flows_total`
OAuth flow attempts.

**Labels**:
- `service`
- `platform`: `twitch`, `youtube`, `kick`, `tiktok`
- `flow_type`: `login`, `add_source`, `refresh`
- `result`: `success`, `failed`, `abandoned`

---

#### Histogram: `auth_oauth_flow_duration_seconds`
Duration of OAuth flows (initiate to callback).

**Labels**:
- `service`
- `platform`
- `flow_type`

**Buckets**: `[1, 5, 10, 30, 60, 120, 300]`

---

#### Counter: `auth_oauth_errors_total`
OAuth errors by type.

**Labels**:
- `service`
- `platform`
- `error_type`: `invalid_state`, `invalid_code`, `token_exchange_failed`, `user_info_failed`, `quota_exceeded`

---

### 5.2 Token Management

#### Counter: `auth_tokens_issued_total`
JWT tokens issued.

**Labels**:
- `service`
- `token_type`: `access`, `refresh`
- `platform`

---

#### Counter: `auth_token_operations_total`
Token operations.

**Labels**:
- `service`
- `operation`: `validate`, `refresh`, `revoke`
- `result`: `success`, `expired`, `invalid`, `revoked`

---

### 5.3 User Operations

#### Counter: `auth_user_operations_total`
User CRUD operations.

**Labels**:
- `service`
- `operation`: `create`, `read`, `update`, `delete`, `link_platform`
- `result`: `success`, `failed`

---

#### Gauge: `auth_active_users_total`
Number of users with valid tokens.

**Labels**:
- `service`

---

## 6. Overlay Manager Metrics

### 6.1 Overlay Operations

#### Counter: `overlay_operations_total`
Overlay CRUD operations.

**Labels**:
- `service`
- `operation`: `create`, `read`, `update`, `delete`, `list`
- `result`: `success`, `failed`

---

#### Gauge: `overlay_total`
Total number of overlays.

**Labels**:
- `service`
- `is_active`: `true`, `false`

---

#### Histogram: `overlay_operation_duration_seconds`
Duration of overlay operations.

**Labels**:
- `service`
- `operation`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1]`

---

### 6.2 Source Management

#### Counter: `overlay_source_operations_total`
Source add/remove operations.

**Labels**:
- `service`
- `operation`: `add`, `remove`, `update`
- `platform`
- `result`: `success`, `failed`

---

#### Gauge: `overlay_sources_total`
Total chat sources configured.

**Labels**:
- `service`
- `platform`
- `is_active`: `true`, `false`

---

## 7. Source Manager Metrics

### 7.1 Leader Election

#### Gauge: `source_manager_leadership_status`
Leadership status (1 = leader, 0 = follower).

**Labels**:
- `service`
- `resource_type`: `youtube_polling`, `global`

---

#### Counter: `source_manager_leadership_changes_total`
Leadership change events.

**Labels**:
- `service`
- `event`: `claimed`, `lost`, `released`, `stolen`

---

#### Histogram: `source_manager_leadership_duration_seconds`
Duration of leadership hold.

**Labels**:
- `service`

**Buckets**: `[60, 300, 900, 1800, 3600, 7200, 14400]`

---

### 7.2 Source Registry

#### Gauge: `source_manager_active_sources_total`
Number of active sources in registry.

**Labels**:
- `service`
- `platform`

---

#### Counter: `source_manager_registry_operations_total`
Registry operations.

**Labels**:
- `service`
- `operation`: `register`, `deregister`, `sync`, `heartbeat`
- `result`: `success`, `failed`

---

## 8. Cross-Service Correlation Metrics

### 8.1 End-to-End Latency

#### Histogram: `platform_message_e2e_latency_seconds`
End-to-end latency: platform → listener → processor → gateway → client.

**Labels**:
- `platform`
- `overlay_id`

**Buckets**: `[0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10]`

**Implementation**: Inject timestamp in listener, track through pipeline.

---

### 8.2 Message Journey

#### Counter: `platform_message_journey_total`
Track message through pipeline stages.

**Labels**:
- `stage`: `received`, `normalized`, `enriched`, `published`, `delivered`
- `platform`
- `result`: `success`, `dropped`, `failed`

---

## 9. Business Metrics

### 9.1 User Engagement

#### Gauge: `allchat_active_overlays_total`
Number of actively used overlays (with WebSocket connections).

**Labels**:
- None (aggregate metric)

---

#### Counter: `allchat_overlay_views_total`
Overlay page views.

**Labels**:
- `overlay_id`
- `view_type`: `config`, `preview`, `live`

---

#### Histogram: `allchat_overlay_session_duration_seconds`
How long overlays are actively viewed/used.

**Labels**:
- None

**Buckets**: `[60, 300, 900, 1800, 3600, 7200, 14400, 28800]`

---

#### Gauge: `allchat_active_users`
Distinct streamers who actually used an overlay within a rolling window — the
DAU/WAU/MAU of real product usage, as opposed to sign-ups.

**Labels**:
- `window`: `24h`, `7d`, `30d`

**Emitted by**: auth-service, polled from the database by the sampler in
`services/auth-service/usage/` every `USAGE_SAMPLE_INTERVAL_SECONDS` (default
120). Every replica reports the same fleet-wide value, so aggregate with
`max(...)`, never `sum(...)`.

**Calculation**: distinct non-banned owners of an overlay whose
`overlays.last_connected_at` (bumped by api-gateway on demand-bearing WebSocket
attach and on each ~2min heartbeat tick) falls inside the window, excluding
overlays that were created but never opened. Definition lives in
`services/auth-service/repository/usage_repository.go`, shared with the admin
dashboard's active-user tiles.

**Graphed by**: *All-Chat: User Growth & Actual Usage* (dashboard uid
`allchat-user-growth`), provisioned from the GitOps repo at
`caesar-deployment: apps/platform/allchat-monitoring/`. See
`deployments/k8s/monitoring/grafana-dashboards/README.md`.

---

### 9.2 Platform Usage

#### Counter: `allchat_messages_by_platform_total`
Total messages delivered by platform.

**Labels**:
- `platform`

**Aggregation**: Sum of all listener messages_received_total

---

#### Gauge: `allchat_connected_platforms_per_user`
Average platforms connected per user.

**Calculation**: `count(distinct platform per user) / count(users)`

---

## 10. Infrastructure Metrics

### 10.1 Redis

#### Gauge: `redis_stream_length`
Length of Redis streams.

**Labels**:
- `stream`: `chat:raw`, `overlay:{id}`

---

#### Gauge: `redis_consumer_group_lag`
Consumer group lag (pending messages).

**Labels**:
- `stream`
- `consumer_group`: `message-processors`

---

#### Counter: `redis_pubsub_messages_total`
Pub/sub messages published.

**Labels**:
- `channel_pattern`: `overlay:*`

---

### 10.2 Database

#### Histogram: `database_query_duration_seconds`
Database query duration.

**Labels**:
- `service`
- `operation`: `select`, `insert`, `update`, `delete`
- `table`

**Buckets**: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5]`

---

#### Gauge: `database_connections_active`
Active database connections.

**Labels**:
- `service`
- `state`: `idle`, `active`, `waiting`

---

## Implementation Priority

### Phase 1: Critical Path (Week 1-2) 🔥
**Goal**: Monitor core message flow and detect outages

1. **All Listeners**:
   - Connection status
   - Message ingestion rate
   - Error tracking

2. **Message Processor**:
   - Processing pipeline metrics
   - Stream lag
   - Message throughput

3. **API Gateway**:
   - WebSocket connections
   - Message delivery
   - Latency

**Success Criteria**: Can detect service outages within 1 minute

---

### Phase 2: Platform Health (Week 2-3) 📊
**Goal**: Monitor platform-specific quotas and rate limits

1. **YouTube Listener**: Full quota monitoring (per existing plan)
2. **Twitch Listener**: IRC rate limits, join throttling
3. **Kick/TikTok Listeners**: Connection stability

**Success Criteria**: Can predict quota/rate limit exhaustion

---

### Phase 3: Performance & Optimization (Week 3-4) ⚡
**Goal**: Identify bottlenecks and optimize

1. **End-to-end latency tracking**
2. **Emote enrichment performance**
3. **Database query performance**
4. **Redis stream lag**

**Success Criteria**: P95 latency < 500ms

---

### Phase 4: Business Insights (Week 4-5) 💼
**Goal**: Understand usage patterns and growth

1. **Active overlay tracking**
2. **Platform usage distribution**
3. **User engagement metrics**
4. **Capacity planning metrics**

**Success Criteria**: Monthly usage reports automated

---

## Grafana Dashboard Structure

### 1. Platform Overview Dashboard
**Panels**:
- Active overlays (gauge)
- Total message rate (graph, all platforms)
- Service health matrix (all services green/red)
- P95 end-to-end latency (gauge)
- Messages by platform (pie chart)
- Error rate (graph, all services)

---

### 2. Listener Health Dashboard
**Panels**:
- Connection status (4 panels, one per platform)
- Message ingestion rate by platform (graph)
- Active sources by platform (bar chart)
- Rate limit / quota usage (4 gauges)
- Top message producers (table)

---

### 3. Message Pipeline Dashboard
**Panels**:
- Pipeline stages (funnel visualization)
- Processing duration by stage (graph)
- Stream lag (gauge)
- Emote enrichment performance (graph)
- Message drop rate (graph)
- Consumer group status (table)

---

### 4. API Gateway Dashboard
**Panels**:
- Active WebSocket connections (gauge)
- Connection duration histogram
- Message delivery latency (graph)
- Overlay subscriptions (table)
- Message throughput in/out (graph)
- Dropped messages (counter)

---

### 5. Business Metrics Dashboard
**Panels**:
- Active overlays trend (7-day graph)
- Messages delivered by platform (stacked area)
- User engagement (session duration histogram)
- Growth metrics (week-over-week)
- Top overlays by activity (table)
- Platform adoption (pie chart)

---

### 6. Capacity Planning Dashboard
**Panels**:
- Resource utilization trends
- Growth projections
- Quota consumption forecasts
- Infrastructure scaling recommendations

---

## Alert Rules Summary

### Critical Alerts 🚨
- Service down (any service unreachable)
- Message pipeline broken (stream lag > 60s)
- WebSocket gateway overloaded (>1000 connections)
- Database connection pool exhausted
- YouTube quota exceeded
- End-to-end latency > 5s

### Warning Alerts ⚠️
- High error rate (>1% errors)
- YouTube quota > 75%
- Stream lag > 30s
- Emote cache hit rate < 80%
- Twitch rate limit approaching
- Connection instability (>5 reconnects/hour)

### Info Alerts ℹ️
- New overlay created
- Leadership changed
- Large message spike detected
- Long-running overlay session (>8h)

---

## Metrics Collection Implementation

### Shared Metrics Package

Create `shared/metrics/` package with common patterns:

```go
// shared/metrics/listener.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type ListenerMetrics struct {
    ConnectionStatus     *prometheus.GaugeVec
    ConnectionAttempts   *prometheus.CounterVec
    MessagesReceived     *prometheus.CounterVec
    MessagesPublished    *prometheus.CounterVec
    MessageLatency       *prometheus.HistogramVec
    APICallsTotal        *prometheus.CounterVec
    ErrorsTotal          *prometheus.CounterVec
    QuotaRemaining       *prometheus.GaugeVec
    ActiveSources        *prometheus.GaugeVec
}

func NewListenerMetrics(platform string) *ListenerMetrics {
    return &ListenerMetrics{
        ConnectionStatus: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "listener_connection_status",
                Help: "Connection status to platform",
            },
            []string{"platform", "service", "connection_type"},
        ),
        // ... more metrics
    }
}

func (m *ListenerMetrics) RecordMessage(channelID, messageType string) {
    m.MessagesReceived.WithLabelValues(
        "platform", "service", channelID, messageType,
    ).Inc()
}
```

---

## Testing Strategy

### Metric Validation
- Unit tests: Verify metrics increment correctly
- Integration tests: Check metrics exposed on `/metrics`
- Load tests: Verify low overhead (<1ms per recording)

### Cardinality Testing
- Monitor unique label combinations
- Alert if cardinality > 10,000 per metric
- Use recording rules to pre-aggregate

### Dashboard Testing
- Verify all panels render
- Check PromQL queries return expected results
- Test alert expressions

---

## Success Metrics

### Operational Excellence
- ✅ MTTD (Mean Time To Detect) < 2 minutes
- ✅ MTTR (Mean Time To Resolve) < 30 minutes
- ✅ 99.9% uptime visibility (no monitoring blind spots)

### Performance
- ✅ Metric collection overhead < 1ms per operation
- ✅ Prometheus scrape time < 5s per service
- ✅ Dashboard load time < 2s

### Adoption
- ✅ All services expose metrics within 4 weeks
- ✅ 6 Grafana dashboards operational
- ✅ 20+ alert rules configured
- ✅ Daily usage of dashboards by team

---

## Documentation Deliverables

1. **Metrics Catalog**: Comprehensive list of all metrics
2. **Dashboard User Guide**: How to use each dashboard
3. **Runbooks**: Response procedures for each alert
4. **Integration Guide**: How to add metrics to new services
5. **Best Practices**: Dos and don'ts for metrics

---

**Last Updated**: 2025-11-20
**Status**: Design complete, ready for implementation
**Estimated Effort**: 4-5 weeks for full implementation
**Priority**: Phase 1 (Critical Path) should start immediately
