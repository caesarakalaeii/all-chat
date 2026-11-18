# ADR-0002: Redis Streams + Pub/Sub Hybrid

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: Architecture Team, Infrastructure Lead

## Context

Chat from every platform listener must reach overlay viewers within 500ms P95, survive a processor crash without losing messages, and fan out one message to many overlays. The transport had to be simple to run and easy for agents to generate correctly.

Alternatives rejected: Kafka (durable but heavy: multi-broker cluster, highest latency and cost of the options); NATS JetStream (less mature, agents generate it poorly, per-overlay fan-out awkward); Redis Streams alone (a stream per overlay is memory-hungry with thousands of overlays and needs ACKs on the hot path); Redis Pub/Sub alone (zero durability, no replay).

## Decision

Use both, split at the message pipeline:

```
Listeners → Redis Streams (chat:raw) → Message Processor → Redis Pub/Sub (overlay:{id}) → API Gateway WebSocket

```

Durability sits before processing (XADD is AOF-persisted and consumer groups reprocess after a crash); delivery after processing is ephemeral in exchange for the lowest latency, and a total loss there is a handful of in-flight chat messages, which is acceptable. Runs on one Redis instance, debuggable with redis-cli.

Deployed since 2026: one primary Redis (dev in docker-compose and local k8s, production the Sentinel-managed replication group behind HAProxy). Consumer groups: `message-processor` (chat) and `alert-processors` (alerts, ADR-0068). Failed messages route to a dead-letter stream and the PEL is drained at startup (XAUTOCLAIM of entries idle > 5 minutes). Latency target met: 425ms P95 end to end in decision-time load tests.

## Implementation

Publishers (XADD `chat:raw`, `MAXLEN 100000` approximate trim, wrapped in the `RingBufferPublisher`, `shared/listener/ring_buffer.go` for retry on transient failures, ADR-0009): every listener's `publisher/` package (twitch-listener, twitch-eventsub-listener, youtube-listener, youtube-listener-innertube, kick-listener, discord-listener, facebook-listener, owncast-listener, goodgame-listener, picarto-listener, altstream-listener, instagram-listener, moderation-service deletions, tiktok-listener's `src/index.ts`).

Consume: `services/message-processor/consumer/stream_consumer.go` (XREADGROUP, group `message-processor`); alert-processor consumes the same stream in its own group (`services/alert-processor/consumer/consumer.go`). Publish per overlay: `stream_consumer.go`'s publisher side (`overlay:{id}`, plus `:updates`) and `alert-processor/publisher/publisher.go` (`overlay:{id}:alerts`).

Subscribe: `services/api-gateway/subscription/subscriber.go` (SUBSCRIBE over one shared connection for `overlay:{id}` and the `:updates`, `:poll`, `:prediction`, `:alerts` channels); WebSocket plumbing in `services/api-gateway/websocket/manager.go`.

Config: dev Redis in `deployments/docker-compose.yml` and `deployments/k8s/base/redis/deployment.yaml` (AOF everysec, 512mb allkeys-lru, optional `REDIS_PASSWORD`). Message demand follows the `overlay:connected:*` keys (ADR-0014 lingers them on disconnect).

Known limits: partial durability after Pub/Sub; Pub/Sub fan-out is O(n) subscribers on one primary (scale-out path: Redis Cluster sharded by overlay id); the 100k-entry stream window discards unprocessed messages if the processor lags (monitor with `XPENDING chat:raw <group>`).

Redis outage recovery: [docs/runbooks/recover-redis-outage.md](../runbooks/recover-redis-outage.md).

Related: ADR-0003 (durable config storage in Postgres), ADR-0009 (XADD retry), ADR-0033 (bounded-concurrency enrichment), ADR-0068 (alerts consumer), [01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md).
