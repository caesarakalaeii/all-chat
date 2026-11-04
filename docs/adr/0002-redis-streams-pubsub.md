# ADR-0002: Redis Streams + Pub/Sub Hybrid

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Architecture Team, Infrastructure Lead

---

## Context and Problem Statement

All-Chat aggregates chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and delivers them to overlay viewers in real-time. The message processing pipeline must:

1. **Buffer messages** from platform listeners (Twitch, YouTube, Kick, TikTok and the beta platform listeners)
2. **Process reliably** through normalization and enrichment pipeline
3. **Deliver to multiple subscribers** (API Gateway replicas broadcasting to WebSocket clients)
4. **Maintain low latency** (<500ms P95 from platform → overlay)

**Problem**: What message transport layer should we use between services?

---

## Decision Drivers

1. **Durability**: Messages must not be lost if processor crashes during handling
2. **Latency**: P95 latency <500ms for entire pipeline (listener → overlay)
3. **Fan-out**: Single message published to N overlays (1:N distribution)
4. **Backpressure**: Processor must handle burst traffic without message loss
5. **Operational Complexity**: Prefer simpler infrastructure (fewer moving parts)
6. **Cost**: Infrastructure costs should scale reasonably with load
7. **Development Velocity**: LLMs must generate integration code accurately

---

## Considered Options

### Option 1: Kafka

**Architecture**:
```
Listeners → Kafka (chat-raw topic) → Message Processor → Kafka (overlay-* topics) → API Gateway
```

**✅ Pros**:
- **Durable**: Messages persisted to disk, replicated across brokers
- **Scalable**: Partitioning supports high throughput (millions msg/s)
- **Backpressure**: Consumer groups handle lag gracefully
- **Battle-tested**: Industry standard for event streaming
- **Replay**: Can reprocess messages from any offset

**❌ Cons**:
- **High latency**: P50 ~50-100ms, P95 ~200-500ms (disk writes, replication)
- **Complex operations**: 3+ Kafka brokers, ZooKeeper (or KRaft), topic management
- **Resource heavy**: Each broker needs 2-4GB RAM, persistent storage
- **Steep learning curve**: Kafka configuration, partitioning strategies complex
- **LLM unfamiliarity**: Fewer Kafka examples, LLMs generate incorrect producer/consumer code
- **Cost**: a multi-broker Kafka cluster's monthly cost, the largest of the options here

**Estimated Latency**: 200-500ms (disk I/O dominant)

---

### Option 2: NATS Streaming (JetStream)

**Architecture**:
```
Listeners → NATS Stream (chat-raw) → Message Processor → NATS Stream (overlay-*) → API Gateway
```

**✅ Pros**:
- **Lower latency**: P95 ~20-50ms (in-memory with optional persistence)
- **Simpler than Kafka**: Single NATS server sufficient for Phase 1
- **Durable**: JetStream provides persistence + replay
- **Lightweight**: ~100MB RAM per NATS server
- **Consumer groups**: Multiple processors can share workload

**❌ Cons**:
- **Less mature**: JetStream released 2020 (vs Kafka 2011)
- **Smaller community**: Fewer examples, less Stack Overflow answers
- **LLM unfamiliarity**: LLMs rarely generate NATS code correctly (requires manual fixes)
- **Operational unknowns**: Team has no production experience with NATS
- **Fan-out complexity**: Need separate stream per overlay (1,000s of streams at scale)

**Estimated Latency**: 50-100ms

---

### Option 3: Redis Streams Only

**Architecture**:
```
Listeners → Redis Stream (chat:raw) → Message Processor → Redis Stream (overlay:{id}) → API Gateway
```

**✅ Pros**:
- **Low latency**: P95 ~5-20ms (in-memory operations)
- **Durable**: Redis Streams with AOF persistence
- **Consumer groups**: Multiple processors share workload (XREADGROUP)
- **Familiar**: Team has Redis experience
- **Simple ops**: Single Redis instance (Phase 1)
- **LLM-friendly**: Many examples, accurate code generation

**❌ Cons**:
- **Fan-out problem**: Need stream per overlay (memory intensive with 1,000s overlays)
- **XREADGROUP for broadcast**: Consumers must XACK messages, adds latency
- **Stream trimming**: MAXLEN limits message retention (need careful tuning)
- **Memory growth**: Each overlay stream consumes memory proportional to message rate

**Estimated Latency**: 20-50ms (but high memory usage)

---

### Option 4: Redis Pub/Sub Only

**Architecture**:
```
Listeners → Redis Pub/Sub (chat) → Message Processor → Redis Pub/Sub (overlay:{id}) → API Gateway
```

**✅ Pros**:
- **Lowest latency**: P95 ~2-10ms (pure in-memory, no persistence)
- **Natural fan-out**: Single PUBLISH → N subscribers (O(n) distribution)
- **Simple**: No consumer groups, no ACKs, fire-and-forget
- **LLM-friendly**: Very simple API (PUBLISH, SUBSCRIBE)

**❌ Cons**:
- **Zero durability**: Messages lost if subscriber disconnected
- **No backpressure**: Slow subscribers cause memory bloat in Redis
- **No replay**: Cannot reprocess messages (lost = lost forever)
- **Fragile**: Processor crash = messages lost during downtime

**Estimated Latency**: 5-10ms (but unreliable)

---

### Option 5: Redis Streams + Pub/Sub Hybrid (CHOSEN)

**Architecture**:
```
Listeners → Redis Streams (chat:raw) → Message Processor → Redis Pub/Sub (overlay:{id}) → API Gateway
              [Durable, backpressure]                      [Low-latency, fan-out]
```

**✅ Pros**:
- **Best of both**: Durability WHERE needed (pre-processing) + low latency WHERE needed (post-processing)
- **Durable ingress**: Listeners publish to Streams (AOF persisted, consumer groups)
- **Fast egress**: Processor publishes to Pub/Sub (in-memory, instant fan-out)
- **Simple**: Single Redis instance, no Kafka/NATS complexity
- **Cost-effective**: A small single-instance Redis suffices
- **LLM-friendly**: Redis is well-documented, LLMs generate correct code
- **Backpressure-capable**: Streams support consumer lag monitoring (XPENDING)

**❌ Cons**:
- **Partial durability**: Messages lost after Pub/Sub if API Gateway crashes (acceptable trade-off)
- **Fan-out bottleneck**: Pub/Sub scales O(n) with subscribers (mitigated by Redis Cluster in Phase 2)
- **Streams MAXLEN**: Must tune retention to avoid trimming unprocessed messages

**Estimated Latency**: **100-500ms total P95** (within <500ms target), dominated by the
processor's normalize + enrich step (emote API calls; then cached).

---

## Decision Outcome

**Chosen**: **Option 5 - Redis Streams + Pub/Sub Hybrid**

**Rationale**:

1. **Latency Target Met** (100-500ms P95):
   - Faster than Kafka (200-500ms)
   - Acceptable for real-time chat (users tolerate <1s delay)

2. **Right Durability Trade-off**:
   - **Before processing**: Durable (Streams with AOF)
     - If Message Processor crashes, messages reprocessed from stream
   - **After processing**: Ephemeral (Pub/Sub)
     - If API Gateway crashes, only in-flight messages lost (<10 messages)
     - Acceptable: Chat messages are transient, not business-critical

3. **Operational Simplicity** (Key Driver for Phase 1):
   - Single Redis instance vs 3-node Kafka cluster
   - Team familiar with Redis (production experience)
   - Easier debugging (Redis CLI for manual inspection)

4. **LLM Development Velocity**:
   - Redis examples abundant, LLMs generate correct code

5. **Cost-Effective**:
   - Single Redis instance is cheap to run

6. **Scalability Path**:
   - Phase 1: Single Redis
   - Phase 2: Redis Cluster if sustained throughput demands it
   - Phase 3+: Migrate to Kafka if needed

---

## Consequences

### Positive

1. **Low Latency Achieved** (~100-500ms P95):
   - Measured in testing: 425ms P95 end to end
   - Users perceive as "real-time" (<1s feels instant)

2. **Simple Operations**:
   - Single-primary Redis (docker-compose for dev, k8s Deployment + PVC in this repo's dev manifests; production runs the Sentinel-managed replication group behind HAProxy, see the deployment pipeline's Redis HA runbook)
   - No Kafka brokers, no ZooKeeper, no complex topic management
   - Redis CLI for debugging (XREAD, XINFO, PUBSUB CHANNELS)

3. **Development Velocity**:
   - LLMs generated the Redis integration code correctly

4. **Backpressure Monitoring**:
   - `XPENDING chat:raw <group>` shows consumer lag (group `message-processor` for chat, `alert-processors` for alerts)
   - Failed messages are routed to a dead-letter stream and the PEL is drained at processor startup (XAUTOCLAIM of entries idle >5 minutes), so a crashed replica's unacknowledged messages are reprocessed rather than lost

### Negative

1. **Partial Durability**:
   - **Messages lost if**:
     - API Gateway crashes after subscribing but before WebSocket send
   - **Mitigation**: Health checks detect crashes quickly, Redis Pub/Sub reconnects automatically

2. **Fan-out Bottleneck**:
   - **Problem**: Pub/Sub scales O(n) with subscribers
   - **Current Limit**: single Redis instance
   - **Mitigation**: Redis Cluster (Phase 2, sharding by overlay ID)

3. **Streams MAXLEN Tuning**:
   - **Problem**: the 100,000-entry sliding window fills quickly under burst load
   - **Risk**: Trimming discards unprocessed messages if processor lags
   - **Mitigation**: Monitor consumer lag (XPENDING), alert on lag

---

## Implementation

### Files and Configuration

**Listeners and other chat:raw publishers** (XADD to chat:raw):
- `services/twitch-listener/publisher/stream_publisher.go`
- `services/twitch-eventsub-listener/publisher/stream_publisher.go`
- `services/youtube-listener/publisher/stream_publisher.go`
- `services/youtube-listener-innertube/publisher/redis_publisher.go`
- `services/kick-listener/publisher/redis.go`
- `services/discord-listener/publisher/stream_publisher.go`
- `services/facebook-listener/publisher/stream_publisher.go`
- `services/owncast-listener/publisher/redis.go`
- `services/goodgame-listener/publisher/redis.go`
- `services/picarto-listener/publisher/redis.go`
- `services/altstream-listener/publisher/redis.go`
- `services/instagram-listener/publisher/stream_publisher.go`
- `services/moderation-service/publisher/deletion.go` (message_deletion events)
- TikTok listener (`services/tiktok-listener`, TypeScript): XADD to `chat:raw` from `src/index.ts`

All use a bounded stream (MAXLEN ~100,000, approximate trimming) and wrap the
XADD call in a `RingBufferPublisher` (`shared/listener/ring_buffer.go`) so
transient publish failures are retried rather than dropped (ADR-0009).

**Message Processor** (consume from Streams, publish to Pub/Sub):
- `services/message-processor/consumer/stream_consumer.go` (XREADGROUP, consumer group `message-processor`, PEL drain at startup via XAUTOCLAIM)
- `services/message-processor/publisher/pubsub_publisher.go` (PUBLISH overlay:{id} and overlay:{id}:updates)

**alert-processor** is a second `chat:raw` consumer group (`alert-processors`,
`services/alert-processor/consumer/consumer.go`) that publishes event-typed
messages to `overlay:{id}:alerts` via `services/alert-processor/publisher/publisher.go`.

**API Gateway** (subscribe to Pub/Sub):
- `services/api-gateway/subscription/subscriber.go` (SUBSCRIBE overlay:{id},
  plus `:updates`, `:poll`, `:prediction` and for alert overlays `:alerts`)
  over one shared connection; the per-connection WebSocket plumbing lives in
  `services/api-gateway/websocket/manager.go`

### Redis Configuration

**docker-compose.yml** (`deployments/docker-compose.yml`, dev):
```yaml
redis:
  image: redis:7-alpine
  # REDIS_PASSWORD empty = AUTH disabled; non-empty enables --requirepass
  command: |
    if [ -n "$REDIS_PASSWORD" ]; then
      exec redis-server --requirepass "$REDIS_PASSWORD" --appendonly yes --appendfsync everysec
        --maxmemory 512mb --maxmemory-policy allkeys-lru;
    else
      exec redis-server --appendonly yes --appendfsync everysec
        --maxmemory 512mb --maxmemory-policy allkeys-lru;
    fi
  ports:
    - "127.0.0.1:6379:6379"
  volumes:
    - redis_data:/data
```

**Kubernetes Deployment** (`deployments/k8s/base/redis/deployment.yaml`, dev/local k3d; production runs the Sentinel + HAProxy stack from the deployment pipeline instead):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command: ["sh", "-c"]
        args:
          - |
            if [ -n "$REDIS_PASSWORD" ]; then
              exec redis-server --requirepass "$REDIS_PASSWORD" --appendonly yes
                --appendfsync everysec --maxmemory 512mb --maxmemory-policy allkeys-lru;
            else
              exec redis-server --appendonly yes --appendfsync everysec
                --maxmemory 512mb --maxmemory-policy allkeys-lru;
            fi
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### Code Examples

**Listener (Publish to Streams)**:
```go
// services/twitch-listener/publisher/stream_publisher.go
// Wrapped in a RingBufferPublisher so transient XADD failures are retried.
func buildXAddFunc(client *redis.Client) sharedlistener.PublishFunc {
    return func(ctx context.Context, data []byte) error {
        args := &redis.XAddArgs{
            Stream: StreamKey,      // "chat:raw"
            MaxLen: MaxStreamLength, // 100000 (sliding window, approximate trim)
            Approx: true,
            Values: map[string]interface{}{"data": data},
        }
        _, err := client.XAdd(ctx, args).Result()
        return err
    }
}
```

**Message Processor (Consume Streams, Publish Pub/Sub)**:
```go
// services/message-processor/consumer/stream_consumer.go
func (c *StreamConsumer) readAndProcess(ctx context.Context) error {
    streams, _ := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    ConsumerGroup, // "message-processor"
        Consumer: c.consumerName,
        Streams:  []string{StreamKey, ">"},
        Count:    ReadCount,     // 100
        Block:    ReadBlockTime, // 5s
    }).Result()

    // Process batch with bounded concurrency (ADR-0033)
    c.processBatch(ctx, streams[0].Messages)
    // Each message: normalize + enrich + Publish to overlay:{id},
    // then XAck (after DLQ routing on repeated failure).
    return nil
}
```

**API Gateway (Subscribe Pub/Sub)**:
```go
// services/api-gateway/subscription/subscriber.go
func (s *Subscriber) Subscribe(ctx context.Context, overlayID string, withAlerts bool) error {
    channels := channelsFor(overlayID, false, withAlerts)
    // [overlay:{id}, overlay:{id}:updates, overlay:{id}:poll,
    //  overlay:{id}:prediction, (overlay:{id}:alerts)]
    pubsub := s.client.Subscribe(ctx, channels...)

    for msg := range pubsub.Channel() {
        // Deliver to the WebSocket connections for this overlay
        s.deliver(overlayID, msg.Payload)
    }
}
```

---

## Future Considerations

**Redis Cluster (Phase 2)**: if sustained throughput outgrows a single primary,
shard by `overlay:{id}`. Kafka migration remains the Phase 3+ fallback path if
streams outgrow what Redis Cluster can handle.

---

## Related Decisions

- **ADR-0003**: [CloudNativePG](./0003-cloudnative-postgres.md) - PostgreSQL for durable configuration storage
- **ADR-0009**: [Ring Buffer Publisher for Listener XADD Resilience](./0009-ring-buffer-publisher.md) - XADD failure retry for the stream publishers
- **ADR-0033**: [Bounded-Concurrency Enrichment in the Message Processor](./0033-message-processor-concurrent-enrichment.md) - bounded concurrency in the consumer
- **ADR-0068**: [Event Alerts Platform](./0068-event-alerts-platform.md) - the second `chat:raw` consumer group (`alert-processors`)
- **Architecture**: [01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md) - Complete message flow documentation
- **Operations**: [Runbooks](../operations/runbooks/) - Redis outage recovery

---

## Validation

### Load Testing Results (2025-11-14)

**Test Setup**:
- 10 Twitch channels, 1,000 msg/s sustained
- 3 Message Processor replicas
- 5 API Gateway replicas
- Single Redis instance (1GB RAM)

**Results**:
| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| **P50 Latency** | 180ms | <250ms | ✅ |
| **P95 Latency** | 425ms | <500ms | ✅ |
| **P99 Latency** | 680ms | <1000ms | ✅ |
| **Message Loss** | 0.01% | <0.1% | ✅ |
| **Redis CPU** | 35% | <70% | ✅ |
| **Redis Memory** | 420MB | <1GB | ✅ |

**Verdict**: Latency targets met, system stable at 1,000 msg/s.

---

## References

- **Redis Streams**: https://redis.io/docs/latest/develop/data-types/streams/
- **Redis Pub/Sub**: https://redis.io/docs/latest/develop/pubsub/

---

## Summary

**Decision**: Use Redis Streams (chat:raw) for durable message queue + Redis Pub/Sub (overlay:*) for low-latency broadcast.

**Reason**: Achieves <500ms P95 latency, operational simplicity, LLM-friendly, cost-effective.

**Trade-off**: Partial durability (Pub/Sub messages lost on crash), fan-out bottleneck at scale (mitigated by Redis Cluster in Phase 2).

**Status**: Implemented and validated in production.
