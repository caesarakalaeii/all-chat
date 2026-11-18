# Scaling & Performance

Honest note first: several numbers below are estimates, not measurements. Where
a number comes from a manifest or metric, its source is named; where it does
not, treat it as a planning input.

## Measured facts

- HPA ranges live in `deployments/k8s/base/*/hpa.yaml`:
  api-gateway 2-10 @ 65%/75% CPU/memory, message-processor 2-8 @ 70%/80%,
  twitch-listener 1-5, kick-listener 1-5, most other listeners 1-5 or
  static. Read the file for each service, not this doc.
- Listener capacity metric exists: `listener_active_sources_total`
  (`shared/metrics/listener.go`), but no HPA currently scales on it; HPA uses
  CPU/memory only.
- Stream lag metric exists: `processor_stream_lag_seconds`
  (`shared/metrics/processor.go` set by message-processor).
- Stream cap: publishers trim `chat:raw` at 100k entries.
- YouTube Data API quota: `QUOTA_LIMIT_DAILY`, default 1,009,000 units/day
  (`services/youtube-listener/cmd/main.go`; docs/quota guide has the
  reserve-confirm-rollback flow, ADR-0006).

## Per-service limits

| Service | Rough ceiling | Basis |
|---|---|---|
| api-gateway | ~2,500 WS connections/pod | memory estimate at 256Mi-1Gi pod; not enforced in code (no per-overlay cap exists) |
| message-processor | few thousand msg/s per pod | enrichment is I/O-bound; parallel 16-way within each 100-msg batch |
| twitch-listener | ~500 channels/pod | Twitch JOIN rate limit, memory per IRC connection |
| kick-listener | ~200 chatrooms/pod | pooled Centrifugo connections, ~100 chatrooms each (ADR-0077) |
| youtube-listener | quota-bound | polling cost dominated by search/discovery, gated by source-manager leader election |

Redis is a single instance and its Pub/Sub fan-out grows with every gateway
replica; that, not the services, is the first real wall if throughput rises
past a few thousand msg/s. Redis Cluster or a separate Pub/Sub instance are the
known-but-undeployed next steps; nothing in this repo implements backpressure
from consumer lag onto publishers.

## Infrastructure

- PostgreSQL runs as a CNPG cluster; scale vertically, and add read replicas
  before sharding. All services share one database (known debt, see
  CLAUDE.md).
- Redis memory is the limiting factor for Redis Streams retention and Pub/Sub
  fan-out; the monitoring stack alerts on it (04-OBSERVABILITY.md).
- Kubernetes namespace: `allchat` (`deployments/k8s/base/kustomization.yaml`).

## Operational knobs

- `MESSAGE_AGE_CUTOFF_SECONDS`: drop stale entries at normalizer stage.
- `MP_PROCESS_CONCURRENCY`: per-batch parallelism in message-processor.
- `PUBSUB_LINGER_SECONDS`: gateway subscription linger (5 min default), which
  also delays source capture shutdown (ADR-0014).
- Emote cache: Redis-backed, 1h fresh / 12h stale grace in
  `services/message-processor/cache/emote_cache.go`; misses hit emote-service
  which caches 1h behind them. Cache hit rate should dominate over external
  7TV/BTTV/FFZ calls.

## Where to look

- Deployment YAML and HPAs: `deployments/k8s/base/` (02-DEPLOYMENT.md).
- Metrics: 04-OBSERVABILITY.md, `shared/metrics/`.
- Quota troubleshooting: `docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md`.
- Listener capacity docs: each `services/*/README.md`.
