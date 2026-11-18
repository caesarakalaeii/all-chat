# All-Chat: Observability

Metrics, logs, tracing, alerting, and where they are configured.

## Stack

| Component | Role | Config |
|---|---|---|
| Prometheus | Metrics store (15 day retention) | kube-prometheus-stack; scraping via pod annotations and ServiceMonitors |
| Grafana | Dashboards + alert rules | GitOps repo, below |
| Alertmanager | Routing Prometheus rules | `deployments/k8s/monitoring/alerts/alertmanager-config.yaml` |
| Loki + Promtail | Log aggregation from pod stdout | GitOps repo (7 day retention) |
| OpenTelemetry | Tracing SDK in every Go service (`shared/tracing/`) | per-service env vars |

Mimir and Tempo are not deployed. Tracing SDK is live in every Go service
(21 services call `tracing.InitTracer`) but exports to OTLP only when
`OTEL_ENABLED=true` and an `OTEL_EXPORTER_OTLP_ENDPOINT` is reachable; no
collector/Tempo backend runs today, so traces go nowhere in production.

Dashboards are not stored in this repo. They live in the GitOps repo
(`apps/platform/allchat-monitoring/`, ConfigMaps with `grafana_dashboard: "1"`
picked up by Grafana's sidecar, synced by the `allchat-monitoring` ArgoCD app).
The dashboards this repo feeds are documented in
`deployments/k8s/monitoring/grafana-dashboards/README.md`.

## Services and metrics

Metric helper package: [`shared/metrics/`](../../shared/metrics/README.md)
(recorders for listeners, processor, gateway, shards, business metrics).
Most main services expose `/metrics` (api-gateway, auth-service, twitch-listener,
youtube-listener, youtube-listener-innertube, kick-listener, message-processor,
overlay-manager, source-manager, emote-service, ...). Discord-listener has its
own local metrics package (`services/discord-listener/metrics/`), TikTok listener's
are in `services/tiktok-listener/src/metrics/prometheus.ts` (`tiktok_*`).
discord-listener, share-service and support-bot do not yet expose `/metrics`.

Naming follows `<service>_<subsystem>_<metric>_<unit>`.

### Listeners (shared helpers, per-platform labels)

`shared/metrics/listener.go`, registered as `metrics.NewListenerMetrics(platform, service)`.
All follow the pattern `<subsystem>_<metric>` with `platform`,`service` labels:

- `listener_connection_status` (1/0), `listener_connection_attempts_total{result}`,
  `listener_connection_duration_seconds{disconnect_reason}`
- `listener_active_sources_total`, `listener_source_events_total{event}`,
  `listener_invalid_channels_total`
- `listener_messages_received_total{channel_id,message_type}`,
  `listener_messages_published_total{result}`, `listener_message_latency_seconds`,
  `listener_message_rate_per_second{channel_id}`
- `listener_rate_limit_hits_total{limit_type}`,
  `listener_quota_remaining{quota_type,limit}`, `listener_quota_usage_percentage{quota_type}`
- `listener_api_calls_total{operation,result,error_type}`,
  `listener_api_call_duration_seconds`
- `listener_errors_total{error_category,severity}`

Quota gauges for YouTube come from `youtube-quota-monitor`
(`metrics.NewListenerMetrics("youtube", "youtube-quota-monitor")`), not from a
listener; the quota-based youtube-listener is not deployed (ADR-0065) except for
opted-in premium channels.

### Platform-specific

- Kick: `services/kick-listener/metrics/` (`kick_listener_socket_state`,
  `kick_listener_messages_total`, `kick_listener_active_subscriptions`, ...)
  plus `shard_*` from `shared/metrics/shard_metrics.go`.
- TikTok (TypeScript): `tiktok_canary_divergences_total`,
  `tiktok_ws_flap_*`, `tiktok_fallback_promotions_total`, etc.
- InnerTube listener: `services/youtube-listener-innertube/metrics/innertube_metrics.go`:
  `youtube_listener_messages_published_total{channel_id}`,
  `youtube_listener_errors_total{error_type}` (`http|parse|rate_limit|redis|network`),
  `youtube_listener_deletion_buffer_overflows_total` (buffer max 1000 events),
  canary/rediscovery metrics. Scraped at 1m by
  `deployments/k8s/monitoring/servicemonitor-innertube.yaml`.

### Message processor

`shared/metrics/processor.go`: `processor_messages_consumed_total`,
`processor_messages_processed_total{stage,result}`,
`processor_message_processing_duration_seconds`, `processor_stage_duration_seconds{stage}`,
emote metrics (`processor_emote_lookups_total{provider,result}`,
`processor_emote_cache_*`), `processor_stream_lag_seconds`,
`processor_stream_errors_total{error_type}`, DLQ/PEL/publish-retry metrics, and
`processor_engagement_forward_total`.

### API gateway

`shared/metrics/gateway.go`: `gateway_websocket_connections_active{connection_type}`,
`gateway_websocket_connections_total{result}`,
`gateway_overlay_subscriptions_active`, `gateway_messages_received_total` /
`gateway_messages_sent_total{result}` / `gateway_messages_dropped_total{reason}`,
`gateway_message_delivery_latency_seconds`, `gateway_http_requests_total{status}`,
`pubsub_reconnect_total`.

### Sharding and business

`shard_*` metrics (25 of them) from `shared/metrics/shard_metrics.go`: coverage,
ghost-pod detection, imbalance ratio (`shard_imbalance_ratio`), migrations.
Business metrics from `shared/metrics/business.go`, mostly registered by
auth-service and overlay-manager: `allchat_user_registrations_total{platform}`,
`allchat_active_users{window}` (definition in
`services/auth-service/repository/usage_repository.go`; aggregate with `max()`
because every auth-service replica reports the same fleet-wide value),
`allchat_active_overlays_total`, `allchat_source_operations_total`.

## Scraping

Prometheus scrapes via pod annotations (kube-prometheus-stack) plus
ServiceMonitors declared in this repo:
`deployments/k8s/monitoring/servicemonitor-innertube.yaml` (1m interval for the
InnerTube listener) and `deployments/k8s/base/discord-listener/servicemonitor.yaml`.
Default scrape interval is 30s, retention 15 days. There is no long-term metrics
store.

## Logging

Every Go service logs JSON via `shared/logger/logger.go` (Zap, ISO8601 ts,
`service` and `version` fields). Promtail ships pod stdout/stderr to Loki with
namespace/pod labels; query in Grafana with LogQL, e.g.
`{namespace="allchat"} |= "level\":\"error\"" | json` or
`{service="youtube-listener"} |= "quota"`.
Loki and Tempo retention is 7 days (GitOps `apps/platform/loki-stack/loki-values.yaml`);
the privacy policy states these periods, so change them together.

## Tracing

OpenTelemetry lives in [`shared/tracing/`](../../shared/tracing/README.md):
OTLP gRPC exporter, W3C trace-context propagation, Gin server-span
middleware (`tracing.GinMiddleware`), and an instrumented HTTP client
(`tracing.NewInstrumentedClient`). Setup in a service `main.go`:

```go
shutdown, err := tracing.InitTracer(tracing.Config{
    ServiceName: "my-service",
    Enabled:     getEnv("OTEL_ENABLED", "false") == "true",
    OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
}, log)
defer shutdown(context.Background())
```

Env vars: `OTEL_ENABLED` (off by default), `OTEL_EXPORTER_OTLP_ENDPOINT`,
`OTEL_SAMPLING_RATE` (0.0-1.0, default 1.0; error traces sample anyway via
`AlwaysSampleErrorsSampler`).

Adding a custom span:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("youtube-listener.quota")

func (t *Tracker) ReserveQuotaWithPriority(ctx context.Context, units int) (string, error) {
    ctx, span := tracer.Start(ctx, "quota.reserve",
        trace.WithAttributes(attribute.Int("quota.units", units)))
    defer span.End()

    if err := ...; err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "reservation failed")
        return "", err
    }
    span.SetAttributes(attribute.String("quota.reservation_id", id))
    return id, nil
}
```

For Gin handlers, prefer the helpers in `shared/tracing/middleware.go`
(`AddSpanAttribute`, `AddSpanEvent`, `RecordError`), which operate on the
recording span of the request. `shared/tracing/http_client.go` wraps outbound
HTTP calls automatically; it is exported but currently unused by services.

Useful pipeline spans if you need deeper tracing: `redis.xreadgroup`
(message-processor, `services/message-processor/consumer/stream_consumer.go`),
`quota.reserve/confirm/rollback`
(`services/youtube-listener/quota/tracker.go`), `websocket.broadcast`
(`services/api-gateway/websocket/`). There is no Tempo backend to query yet;
traces are only meaningful with an OTLP collector attached.

## Alerting

Rules are PrometheusRule resources in this repo:

- critical (`deployments/k8s/monitoring/alerts/allchat-critical-alerts.yaml`, 6 rules):
  `AllChatServiceDown` (up=0 for 2m), `YouTubeQuotaCritical` (>90% for 1m),
  `AllChatListenerDisconnected` (status=0 for 2m), `AllChatHighMessageDropRate`
  (>5% loss), `ShardLoadImbalance` (ratio>0.7 for 10m), `ShardCoordinatorSplitBrain`.
- warning (`deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml`, 11 rules):
  `YouTubeQuotaWarning` (>80%), `AllChatHighProcessingLatency` (p95 >500ms),
  `AllChatMessageBacklog` (`processor_stream_lag_seconds` >5000 means pending
  messages, the alert's expr), `AllChatListenerReconnecting`, `AllChatHighStreamErrors`,
  `AllChatPodCrashLoop`, `AllChatLowEmoteCacheHitRate` (<80%), shard thrashing,
  and the three TikTok transport alerts (`TikTokCanaryDivergence`,
  `TikTokWsFlapExhausted`, `TikTokFallbackPromoted`).
- Grafana-side rules: `deployments/k8s/monitoring/grafana-alerts/grafana-allchat-alerts.yaml`
  (Grafana alert ConfigMap, e.g. YouTube Quota Critical >95%, Pod Not Running).

Routing (`deployments/k8s/monitoring/alerts/alertmanager-config.yaml`):
`team=allchat` criticalGo to the `allchat-critical` receiver, warnings go to `allchat-warnings`
both Discord plus email for critical. Webhook URL and email are placeholders
(`YOUR_DISCORD_WEBHOOK_URL_HERE`); real values live in a cluster secret
(`allchat-discord-webhook`), which wins. Lower severities are inhibited while a
higher-severity alert for the same alert is firing.

The dashboards these rules reference live in the GitOps repo, including
`/d/allchat-youtube-quota` referenced from alert annotations.

## Availability and status page

Service availability is measured by the public status page
[status.allch.at](https://status.allch.at): Gatus in the `status` namespace,
configured in the GitOps repo (`apps/workloads/status/config.yaml`, the ADR-0085
cluster). Each row is checked every minute (sign-in every two), uptime is the
share of passing checks over 1h/24h/7d/30d, and nothing is excluded by hand.
Rows measure public requests through Cloudflare + Envoy (website, sign-in,
overlay page, overlay WebSocket), a `status-prober` end-to-end chat run against
the fixed test overlay (see `services/message-processor/README.md`), and
Prometheus-backed signals (ready replicas, `processor_stream_lag_seconds < 60`,
listener ready + no critical platform alert firing, Prometheus `/-/ready`).

Limits of that measurement: requests that never reach the cluster are only seen
by public checks, and a down cluster takes the status page down with it, so that
time is not recorded. Silence from a low-traffic platform (Kick, Discord, beta
platforms) is never treated as an outage.

## Operations

Health endpoints per service: `/health/live`, `/health/ready`, `/metrics`
(plus `/status` on some services). Port-forward into Grafana
(`kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80`,
login `grafana-admin-credentials` secret) and Prometheus (`svc/kube-prometheus-stack-prometheus 9090`)
to inspect targets (`/targets`) and alerts (`/alerts`).

Runbooks: [troubleshooting/decision-tree.md](../troubleshooting/decision-tree.md),
[runbooks/](../runbooks/) (scale-api-gateway, recover-redis-outage,
youtube-quota-recovery, platform-listener-deployment).

Also here: [Deployment](./02-DEPLOYMENT.md) and [Scaling](./03-SCALING.md).
