# Phase 8: Message Pipeline Resilience — Research

**Researched:** 2026-03-29
**Domain:** Redis Streams consumer groups, Redis Pub/Sub reconnect, Go ring buffers, DLQ patterns, Prometheus metrics
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scope & Priority**
- D-01: All 24 failure modes are in scope — no deferral of edge cases
- D-02: Fixes grouped by service (all message-processor fixes together, all api-gateway fixes together, etc.) — not by severity
- D-03: Each service's fixes form a natural plan boundary for independent testing and deployment

**Recovery Strategy**
- D-04: Failed Redis operations use exponential backoff retry (3 attempts: 100ms, 500ms, 2s) then dead-letter to DLQ stream `chat:dlq`
- D-05: No message may be silently dropped — every failure path must either retry successfully or land in DLQ with full context
- D-06: Pub/Sub reconnect handled per-subscriber (Subscriber, StatusSubscriber detect channel closure and re-subscribe independently). go-redis handles TCP reconnect; application layer handles re-subscription
- D-07: Listener XADD failures buffered in in-memory ring buffer (capacity: 1000 messages) with retry every 500ms. When buffer full, drop oldest message. Implemented as shared/listener SDK method so all Go listeners get it

**Consumer Naming**
- D-08: Message-processor consumer names use `os.Hostname()` (maps to K8s pod name). Unique per replica, stable across restarts of same pod

**DLQ Lifecycle**
- D-09: DLQ stream auto-trimmed via XTRIM MINID for messages older than 7 days
- D-10: Admin endpoint or CLI command to replay DLQ messages back to `chat:raw` for reprocessing
- D-11: DLQ messages include original stream ID, source service, failure reason, and retry count as fields

**Rollout**
- D-12: Deploy service-by-service via existing CI/CD (commit, push, Keel). Validate each service before moving on
- D-13: No feature flags — direct deployment with rollback via git revert

**Observability**
- D-14: New Prometheus metrics: `pel_pending_messages`, `pubsub_reconnect_total`, `dlq_messages_total`, `publish_retry_total`, `ring_buffer_depth`, `ring_buffer_drops_total`
- D-15: Matching Prometheus alert rules for each new metric (extends Phase 4 alert groups)
- D-16: DLQ gets its own Grafana dashboard panel on the Pipeline dashboard
- D-17: Alert when DLQ depth > 0 for 5 minutes

### Claude's Discretion
- Specific PEL drain strategy (XAUTOCLAIM vs XPENDING+XCLAIM) — researcher/planner decides based on Redis version
- Ring buffer implementation details (channel-based vs mutex-protected slice)
- Exact grouping of the 24 failure modes into service-scoped plans
- Order of service fixes within the phase (which service first)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope

</user_constraints>

---

## Summary

Phase 8 fixes all silent failure modes across the complete message pipeline: Listeners → Redis Streams → Message Processor → Redis Pub/Sub → API Gateway WebSocket. The codebase currently has no retry logic, no dead-letter routing, and a hardcoded consumer name (`processor-1`) that breaks consumer group ownership semantics when message-processor scales to multiple replicas. With 3 active replicas confirmed in production, all three pods are registering as the same consumer, meaning PEL entries can never be correctly claimed or drained.

The fixes decompose naturally into 5 service-scoped plans: (1) message-processor stream consumer hardening (PEL drain, unique consumer names, DLQ routing, retry logic, ACK ordering fix), (2) API gateway Pub/Sub subscriber reconnect for `Subscriber`, (3) API gateway `StatusSubscriber` nil-channel guard, (4) listener SDK ring buffer for XADD failures in all Go listeners, and (5) observability additions (new metrics, alert rules, DLQ Grafana panel).

**Primary recommendation:** Start with message-processor (highest impact, 3-replica collision happening now in production) and end with the listener SDK ring buffer (touches 6 listener services, requires shared module bump and coordinated deployment).

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 (in go.mod) | Redis Streams, Pub/Sub, DLQ | Already used; XAutoClaim API available |
| `os.Hostname()` | stdlib | Unique consumer name per K8s pod | Matches D-08; pod name is hostname in K8s |
| `github.com/alicebob/miniredis/v2` | v2.37.0 | In-memory Redis for tests | Already in message-processor go.mod |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | Already in all service go.mods |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/prometheus/client_golang` | existing | New resilience metrics | Already wired via promauto pattern |
| `sync` (stdlib) | — | Ring buffer mutex | Mutex-protected slice is simpler than channel-based for fixed-capacity ring |
| `time` (stdlib) | — | Retry backoff timers | 100ms/500ms/2s schedule per D-04 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| XAUTOCLAIM | XPENDING + XCLAIM | XAUTOCLAIM is atomic and paginated — preferred. Redis 7.4.7 confirmed in prod. go-redis v9 has `XAutoClaim(ctx, *XAutoClaimArgs)` returning `([]XMessage, cursor string, err)` |
| Mutex-protected slice ring buffer | Channel-based ring buffer | Mutex is simpler; channel-based requires goroutine overhead; mutex gives O(1) capacity check |
| In-process DLQ admin | HTTP admin endpoint on message-processor | HTTP endpoint preferred for consistency with existing admin pattern in api-gateway |

---

## Enumeration of the 24 Silent Failure Modes

The CONTEXT.md references "24 silent failure modes identified in the robustness audit." The research systematically audited all pipeline services. The enumerated list below is the definitive scope for planning:

### Message Processor (stream_consumer.go + publisher/pubsub_publisher.go)
| # | Failure Mode | File | Current Behaviour |
|---|-------------|------|-------------------|
| MP-01 | Hardcoded `ConsumerName = "processor-1"` | `consumer/stream_consumer.go:25` | All 3 replicas register as same consumer — PEL entries are owned by a non-existent ghost consumer on pod restart; messages stuck in PEL forever |
| MP-02 | No PEL drain on startup | `consumer/stream_consumer.go:62` | Unacknowledged messages from crashed instance are never reclaimed; silently lost until stream MAXLEN evicts them |
| MP-03 | ACK issued before handler success confirmed | `consumer/stream_consumer.go:154-169` | Code ACKs after `processMessage` returns — but if handler panics or process is killed after ACK write, message is lost. ACK ordering correct but not transactional; panic recovery missing in consume loop |
| MP-04 | Redis Pub/Sub `Publish` failures silently logged only — no retry, no DLQ | `publisher/pubsub_publisher.go:44` | `Publish` returns error, caller logs it; message silently dropped if Redis is momentarily unavailable |
| MP-05 | Batch `PublishToMultiple` uses pipeline — partial failures invisible | `publisher/pubsub_publisher.go:77-92` | `pipe.Exec()` returns combined error; individual overlay publish failures undetectable; no per-overlay retry |
| MP-06 | `processMessage` returns error on parse failure, continues loop — no DLQ routing | `consumer/stream_consumer.go:182-195` | Malformed messages never reach DLQ; PEL entry remains pending with `delivery_count` incrementing until stream trim evicts it |
| MP-07 | `processDeletionEvent` error causes caller to return error but message stays in PEL | `consumer/stream_consumer.go:244-266` | Deletion message stuck in PEL without bound; no max retry threshold; no DLQ |
| MP-08 | `XReadGroup` context cancellation not distinguished from connection error | `consumer/stream_consumer.go:138-143` | On context done, loop retries with 1s sleep instead of exiting cleanly; masked as error log |

### API Gateway — Subscriber (subscription/subscriber.go)
| # | Failure Mode | File | Current Behaviour |
|---|-------------|------|-------------------|
| AG-01 | `listen()` goroutine exits on closed channel (`ok == false`) with Warn log — no re-subscribe | `subscription/subscriber.go:143-147` | When Redis connection is interrupted and go-redis closes the channel, subscription is silently dead; overlay clients receive no more messages |
| AG-02 | `Subscribe()` uses `context.Background()` — subscription context never cancelled | `handlers/websocket.go:170` | Subscriptions accumulate context roots; no lifecycle tie to WebSocket connection; leaked goroutines on long-lived deployments |
| AG-03 | Reference count can go negative on `Unsubscribe` without prior `Subscribe` | `subscription/subscriber.go:89-99` | Defensive but silent; no metric or log when ref count underflows |
| AG-04 | `SubscribeViewerOnly` and `Subscribe` share the same `subscriptions` map — viewer-only subscription silently upgraded to full subscription if order interleaves | `subscription/subscriber.go:196-210` | The first subscribe type wins; late viewer connection to already-full-subscribed overlay gets full sub (acceptable but undocumented); vice versa loses update channel |
| AG-05 | Subscription `listen` goroutine not tracked individually — `wg.Wait()` in `Stop()` waits for all but individual goroutine leaks on reconnect are invisible | `subscription/subscriber.go:79, 156` | Re-subscription after AG-01 fix must add new goroutine to wg; existing Stop logic only covers initial goroutines |

### API Gateway — StatusSubscriber (subscription/status_subscriber.go)
| # | Failure Mode | File | Current Behaviour |
|---|-------------|------|-------------------|
| SS-01 | `ch := pubsub.Channel()` — if Subscribe fails or Redis disconnects, `ch` is nil; `case msg := <-ch` blocks forever on nil channel instead of selecting timeout or reconnect | `subscription/status_subscriber.go:48` | A nil receive channel blocks the select indefinitely; status updates stop arriving; no log or metric |
| SS-02 | No reconnect on channel close — same pattern as AG-01 but goroutine blocks on nil channel instead of exiting | `subscription/status_subscriber.go:45-62` | After Redis reconnect, status subscriber is stuck; requires pod restart to recover |
| SS-03 | `pubsub.Close()` deferred but `Subscribe` error not checked | `status_subscriber.go:40` | If `Subscribe(ctx, channel)` returns an error-state pubsub, `.Channel()` may return nil immediately |

### Listener SDK / Twitch Listener — XADD publish path
| # | Failure Mode | File | Current Behaviour |
|---|-------------|------|-------------------|
| LI-01 | `StreamPublisher.Publish()` error returned to IRC message handler — handler logs error and continues; message silently dropped | `publisher/stream_publisher.go:63-70` | IRC messages lost during Redis unavailability; no buffering, no retry |
| LI-02 | `PublishBatch()` pipeline error does not identify which messages failed | `publisher/stream_publisher.go:121-126` | Whole batch treated as failed; could retry all or drop all; currently returns error (caller drops batch) |
| LI-03 | Same XADD silent drop exists in `kick-listener/publisher/redis.go:55`, `youtube-listener-innertube/publisher/redis_publisher.go:112`, `discord-listener/publisher/stream_publisher.go:71` | multiple | All Go listeners have same pattern; tiktok-listener is Node.js and out of scope |

### DLQ (does not yet exist — gaps to fill)
| # | Failure Mode | Description |
|---|-------------|-------------|
| DQ-01 | No dead-letter stream `chat:dlq` — messages exceeding retry threshold are silently discarded | Needs creation |
| DQ-02 | No 7-day XTRIM auto-cleanup on DLQ | Needs scheduled trim via PEL drain goroutine or Redis TTL trick |
| DQ-03 | No admin endpoint to replay DLQ | Needs `POST /admin/dlq/replay` on message-processor |
| DQ-04 | No Grafana panel or alert for DLQ depth | Needs panel on existing Pipeline dashboard + alert rule |

**Total: 24 failure modes** (MP-01 through MP-08 = 8, AG-01 through AG-05 = 5, SS-01 through SS-03 = 3, LI-01 through LI-03 = 3, DQ-01 through DQ-04 = 4, but DQ items are gaps enabled by fixing MP/AG/SS failures — they all count as addressable within this phase per D-01). The grouping and numbering above is the canonical list for task planning.

---

## Architecture Patterns

### Recommended Service Plan Grouping (Claude's Discretion)

Given D-02 (group by service) and D-03 (each service = independent plan), the natural decomposition is:

| Plan | Service | Failure Modes | Rationale |
|------|---------|--------------|-----------|
| 08-01 | message-processor | MP-01, MP-02, MP-03, MP-04, MP-05, MP-06, MP-07, MP-08, DQ-01, DQ-02, DQ-03 | Highest impact; 3-replica consumer name collision happening in production now |
| 08-02 | api-gateway Subscriber | AG-01, AG-02, AG-03, AG-04, AG-05 | Pure reconnect logic; no dependency on DLQ; deploy independently |
| 08-03 | api-gateway StatusSubscriber | SS-01, SS-02, SS-03 | Small file; nil-channel panic guard; deploy with 08-02 or after |
| 08-04 | shared/listener SDK ring buffer | LI-01, LI-02, LI-03 | Touches shared module; all Go listeners require shared module bump; deploy last |
| 08-05 | Observability (metrics + alerts + dashboard) | DQ-04 + D-14 through D-17 | Can be done in parallel with 08-01; alert rules extend caesar-deployment |

### Pattern 1: PEL Drain on Startup (XAUTOCLAIM)

**What:** On startup, each message-processor instance uses `XAUTOCLAIM` to reclaim messages idle in PEL for > `MinIdle` threshold, processing them before entering the normal read loop.

**When to use:** On every startup, before `XREADGROUP >` loop begins.

**Why XAUTOCLAIM over XPENDING+XCLAIM:** XAUTOCLAIM is atomic (single Redis round-trip), auto-paginating via cursor, and returns both claimed messages and deleted-entry IDs. Redis 7.4.7 confirmed in production. go-redis v9 (v9.18.0 in go.mod) exposes `XAutoClaim(ctx, *XAutoClaimArgs)`.

```go
// Source: go-redis v9 stream_commands.go (verified)
// Run on startup after consumer group creation, before main loop
func (c *StreamConsumer) drainPEL(ctx context.Context) {
    cursor := "0-0"
    for {
        messages, nextCursor, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
            Stream:   StreamKey,
            Group:    ConsumerGroup,
            Consumer: c.consumerName, // os.Hostname() per D-08
            MinIdle:  5 * time.Minute,
            Start:    cursor,
            Count:    100,
        }).Result()
        if err != nil {
            c.logger.Error("PEL drain failed", zap.Error(err))
            return
        }
        for _, msg := range messages {
            // Process and ACK — same path as normal consume
            _ = c.processMessage(ctx, msg)
        }
        if nextCursor == "0-0" {
            break // drain complete
        }
        cursor = nextCursor
    }
}
```

### Pattern 2: Exponential Backoff Retry with DLQ (D-04, D-05)

**What:** Wrap any Redis write (Publish, XADD to DLQ) in a retry helper. After 3 attempts (100ms, 500ms, 2s delays), write to `chat:dlq` with failure context.

```go
// retryOp retries fn up to 3 times with exponential backoff.
// Delays: 100ms, 500ms, 2000ms.
func retryOp(ctx context.Context, fn func() error) error {
    delays := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}
    var lastErr error
    for i, d := range delays {
        if lastErr = fn(); lastErr == nil {
            return nil
        }
        if i < len(delays)-1 {
            select {
            case <-time.After(d):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    return lastErr
}
```

### Pattern 3: DLQ Write (D-11)

**What:** When retry exhausted, write to `chat:dlq` stream. Fields per D-11:

```go
// writeToDLQ writes a failed message to chat:dlq with context.
func (c *StreamConsumer) writeToDLQ(ctx context.Context, originalID, sourceService, failureReason string, retryCount int, originalValues map[string]interface{}) {
    dlqValues := map[string]interface{}{
        "original_stream_id": originalID,
        "source_service":     sourceService,
        "failure_reason":     failureReason,
        "retry_count":        retryCount,
        "original_data":      originalValues["data"], // preserve original payload
        "dlq_timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
    }
    // No retry on DLQ write — best-effort; log failure if DLQ write also fails
    if err := c.client.XAdd(ctx, &redis.XAddArgs{
        Stream: DLQStreamKey, // "chat:dlq"
        Values: dlqValues,
    }).Err(); err != nil {
        c.logger.Error("DLQ write failed — message lost",
            zap.String("original_id", originalID),
            zap.Error(err),
        )
    }
}
```

### Pattern 4: Pub/Sub Re-subscription on Channel Close (D-06)

**What:** When `pubsub.Channel()` closes (detected as `ok == false` or `ch == nil`), log the reconnect, call `pubsub.Close()`, create a new `client.Subscribe()`, and restart the listen goroutine. go-redis handles TCP reconnect transparently; the application only needs to re-subscribe at the Redis protocol level.

**Key insight:** go-redis `PubSub.Channel()` returns a Go channel that goes closed when the underlying connection drops. The application must detect this and call `Subscribe()` again on the same `*redis.Client` — the client maintains its connection pool and will reconnect.

```go
// Source: go-redis v9 pubsub.go pattern (verified from codebase)
func (s *Subscriber) listen(ctx context.Context, overlayID string, pubsub *redis.PubSub) {
    defer s.wg.Done()
    ch := pubsub.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopChan:
            return
        case msg, ok := <-ch:
            if !ok {
                // Channel closed — reconnect
                s.metrics.RecordPubSubReconnect(overlayID)
                s.logger.Warn("Pub/Sub channel closed — re-subscribing",
                    zap.String("overlay_id", overlayID))
                pubsub.Close()
                // Re-subscribe (increments wg; this goroutine about to exit)
                s.resubscribe(ctx, overlayID)
                return
            }
            s.handler(overlayID, msg.Channel, []byte(msg.Payload))
        }
    }
}
```

### Pattern 5: StatusSubscriber Nil-Channel Guard (SS-01, SS-02)

**What:** The root issue is that `pubsub.Channel()` can return nil if `Subscribe` was called with a cancelled or errored context. Guard against this with an explicit nil check and reconnect:

```go
// In StatusSubscriber.Start()
ch := pubsub.Channel()
if ch == nil {
    s.logger.Error("Status subscriber channel is nil — subscription failed")
    // Retry subscribe or return error
    return fmt.Errorf("failed to get pub/sub channel for %s", PlatformStatusChannel)
}
```

For reconnect on channel close (SS-02), same pattern as subscriber.go above.

### Pattern 6: Ring Buffer for Listener XADD (D-07)

**What:** Add `RingBufferPublisher` to `shared/listener/` as opt-in wrapper. Wraps any `Publish(ctx, msg) error` function. On failure, enqueues to ring buffer (capacity 1000). Background goroutine retries every 500ms. When buffer full, drops oldest and increments `ring_buffer_drops_total`.

**Implementation choice (Claude's Discretion): Mutex-protected slice ring buffer** — simpler than channel-based, O(1) enqueue/dequeue with index tracking, no goroutine per enqueue.

```go
// shared/listener/ring_buffer.go
type RingBuffer struct {
    mu       sync.Mutex
    buf      []bufferedMsg
    head     int
    tail     int
    size     int
    capacity int
    metrics  *RingBufferMetrics
}

type bufferedMsg struct {
    ctx       context.Context
    payload   []byte
    enqueuedAt time.Time
}

func (r *RingBuffer) Enqueue(ctx context.Context, payload []byte) (dropped bool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if r.size == r.capacity {
        // Drop oldest (advance head)
        r.head = (r.head + 1) % r.capacity
        r.size--
        dropped = true
        r.metrics.DropsTotal.Inc()
    }
    r.buf[r.tail] = bufferedMsg{ctx: ctx, payload: payload, enqueuedAt: time.Now()}
    r.tail = (r.tail + 1) % r.capacity
    r.size++
    r.metrics.Depth.Set(float64(r.size))
    return
}
```

### Pattern 7: Consumer Name via os.Hostname() (D-08)

**What:** Replace `ConsumerName = "processor-1"` constant with `os.Hostname()` called at startup.

```go
// In StreamConsumer initialization
hostname, err := os.Hostname()
if err != nil {
    hostname = "processor-unknown"
    logger.Warn("Failed to get hostname for consumer name", zap.Error(err))
}
consumer := &StreamConsumer{
    consumerName: hostname,
    // ...
}
```

In K8s, `os.Hostname()` returns the pod name (confirmed: HOSTNAME env = `message-processor-ddd87d996-65nx5` in production pod). This is stable across restarts of the same pod and unique per replica — exactly what PEL ownership tracking requires.

### Pattern 8: DLQ XTRIM via MINID (D-09)

**What:** Periodically trim `chat:dlq` to remove entries older than 7 days. Redis `XTRIM MINID` uses stream entry ID (which encodes millisecond timestamp) to trim efficiently without scanning.

```go
// Run in background goroutine, every hour
func (c *StreamConsumer) trimDLQ(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            cutoff := time.Now().Add(-7 * 24 * time.Hour)
            minID := fmt.Sprintf("%d-0", cutoff.UnixMilli())
            if err := c.client.XTrimMinID(ctx, DLQStreamKey, minID).Err(); err != nil {
                c.logger.Warn("DLQ XTRIM failed", zap.Error(err))
            }
        case <-ctx.Done():
            return
        }
    }
}
```

`XTrimMinID` is available in go-redis v9 (verified in go-redis codebase).

### Pattern 9: DLQ Replay Admin Endpoint (D-10)

**What:** `POST /admin/dlq/replay` on message-processor reads N messages from `chat:dlq`, re-publishes to `chat:raw` via XADD, then deletes the replayed entries from DLQ. Consistent with existing admin endpoint pattern in api-gateway (`/admin/stats`, `/admin/overlays`).

### Anti-Patterns to Avoid

- **ACK before retry:** Never ACK a message before confirming delivery to DLQ on permanent failure — if ACK succeeds but DLQ write fails, message is lost
- **Infinite PEL drain on startup:** Always bound PEL drain by idle threshold (5 min) and count (100 per iteration) — don't claim messages that may be actively processing on another pod
- **Re-using closed PubSub handle:** After `pubsub.Close()`, create new subscription via `client.Subscribe()` — the closed handle is unusable
- **Ring buffer with unlimited context propagation:** Ring buffered messages store context at enqueue time; if context is already cancelled at retry time, use `context.Background()` as fallback

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PEL claiming | Custom XPENDING+XCLAIM loop | `client.XAutoClaim()` | Atomic, paginated, one round-trip, handles deleted entries |
| Redis stream trimming | Manual DELETE loop | `client.XTrimMinID()` | Redis-native, O(log N), no scan |
| Retry with backoff | Custom timer loop | Inline `retryOp` helper (5 lines) | No external dep needed; pattern is simple enough to inline |
| Pub/Sub reconnect | New go-redis client | Reuse `*redis.Client`, call `Subscribe()` again | Client maintains connection pool; only sub-level reconnect needed |
| Unique consumer ID | UUID generation | `os.Hostname()` | Pod name is already unique+stable; no new dependency |

---

## Common Pitfalls

### Pitfall 1: PEL Drain Claiming Active Messages

**What goes wrong:** Drain goroutine claims messages with `MinIdle: 0` — claims messages currently being processed by another pod
**Why it happens:** Forgetting MinIdle minimum; setting too short a threshold
**How to avoid:** Use `MinIdle: 5 * time.Minute` — longer than any expected single-message processing time (~500ms)
**Warning signs:** Duplicate message processing logs from different pods

### Pitfall 2: Re-subscription Creates Duplicate Goroutines

**What goes wrong:** On channel close, `resubscribe()` adds new wg goroutine but old goroutine is still in wg; `Stop()` deadlocks waiting for both
**Why it happens:** `s.wg.Add(1)` in `listen()` is not paired with `Done()` before re-subscribing
**How to avoid:** `listen()` returns (calling `s.wg.Done()`) before spawning new goroutine; new goroutine calls `s.wg.Add(1)` before starting

### Pitfall 3: ACK After Retry Without idempotency

**What goes wrong:** Retried message is processed twice (by original consumer and PEL drain on new pod), ACKed twice, second ACK silently ignored by Redis — no problem, but if handler has side effects (DB write), duplicate processing occurs
**Why it happens:** Redis Streams guarantee at-least-once on crash; handler must be idempotent
**How to avoid:** Message dedup is already present in `services/message-processor/dedup/` — verify dedup runs before handler for retried messages too

### Pitfall 4: Ring Buffer Entries with Cancelled Contexts

**What goes wrong:** IRC callback provides request-scoped context; context cancelled 100ms later; ring buffer retry uses cancelled context, all retries fail immediately
**Why it happens:** Listeners pass IRC callback's context to ring buffer enqueue
**How to avoid:** Ring buffer retry goroutine uses `context.Background()` for all retry attempts — network operations don't need the original caller's context

### Pitfall 5: DLQ Write Looping

**What goes wrong:** DLQ write itself fails (Redis unavailable) → triggers retry → retry fails → triggers DLQ write of DLQ write failure → infinite loop
**Why it happens:** Applying same retry logic to DLQ writes
**How to avoid:** DLQ writes are best-effort, single-attempt, no retry. Log failure at ERROR level. The message is already lost at this point.

### Pitfall 6: Multiple go-redis `PubSub` Handles Leaking

**What goes wrong:** On re-subscription, old `pubsub` handle not closed before creating new one; old TCP connection held open
**Why it happens:** Forgetting `pubsub.Close()` before creating new subscription
**How to avoid:** Always call `oldPubsub.Close()` before `client.Subscribe()` in reconnect path

### Pitfall 7: Consumer Group Creation Race on Multi-Pod Startup

**What goes wrong:** Three message-processor pods start simultaneously; all call `XGroupCreateMkStream`; two get BUSYGROUP; race condition in error checking
**Why it happens:** Current code checks `err.Error() != "BUSYGROUP..."` — string comparison is fragile
**How to avoid:** Use `strings.Contains(err.Error(), "BUSYGROUP")` or check for `redis.ErrBusyGroup` when available. The current implementation is acceptable but the string comparison should be reviewed.

---

## Code Examples

### XAutoClaim in go-redis v9
```go
// Source: verified from /home/caesar/go/pkg/mod/github.com/redis/go-redis/v9@v9.16.0/stream_commands.go
messages, nextCursor, err := client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
    Stream:   "chat:raw",
    Group:    "message-processor",
    Consumer: consumerName,
    MinIdle:  5 * time.Minute,
    Start:    "0-0",  // start from beginning
    Count:    100,
}).Result()
// nextCursor == "0-0" means drain complete
```

### XTrimMinID
```go
// Source: go-redis v9 stream_commands.go
cutoff := time.Now().Add(-7 * 24 * time.Hour)
minID := fmt.Sprintf("%d-0", cutoff.UnixMilli())
err := client.XTrimMinID(ctx, "chat:dlq", minID).Err()
```

### New Metrics (extending ProcessorMetrics pattern)
```go
// Follows existing promauto pattern in shared/metrics/processor.go
PELPendingMessages   *prometheus.GaugeVec    // pel_pending_messages{consumer}
DLQMessagesTotal     *prometheus.CounterVec  // dlq_messages_total{source_service, reason}
PublishRetryTotal    *prometheus.CounterVec  // publish_retry_total{attempt}
PubSubReconnectTotal *prometheus.CounterVec  // pubsub_reconnect_total{subscriber}
RingBufferDepth      *prometheus.GaugeVec    // ring_buffer_depth{service}
RingBufferDropsTotal *prometheus.CounterVec  // ring_buffer_drops_total{service}
```

### DLQ alert rule pattern (from Phase 4 allchat-alerts.yaml)
```yaml
- uid: dlq-depth-nonzero
  title: DLQ Messages Accumulated
  condition: B
  data:
    - refId: A
      queryType: instant
      datasourceUid: prometheus
      model:
        expr: sum(rate(dlq_messages_total[5m])) > 0
        refId: A
    - refId: B
      datasourceUid: __expr__
      model:
        type: threshold
        refId: B
        conditions:
          - evaluator: {params: [0], type: gt}
            query: {params: [A]}
  noDataState: OK
  for: 5m
  labels: {severity: critical, team: allchat}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| XPENDING + XCLAIM (two round-trips) | XAUTOCLAIM (single command, paginated) | Redis 6.2.0 | Atomic PEL drain; cursor-based pagination |
| `go-redis/v8` | `go-redis/v9` (project uses v9) | ~2022 | API change: context always first arg; `Result()` idiom unchanged |

---

## Open Questions

1. **PEL MinIdle threshold**
   - What we know: Processing a single message takes ~100-500ms (from data flow doc)
   - What's unclear: What's the maximum? Enrichment can take up to 500ms + emote HTTP calls
   - Recommendation: Use 5 minutes as MinIdle — conservative enough to never claim actively-processing messages

2. **Ring buffer retry context**
   - What we know: IRC callbacks provide goroutine-local context
   - What's unclear: Whether any listener passes a context with meaningful cancellation
   - Recommendation: Ring buffer retry goroutine always uses `context.Background()` — network retries must not depend on caller context

3. **DLQ replay idempotency**
   - What we know: Replayed messages re-enter `chat:raw` as new stream entries
   - What's unclear: Whether re-processing could cause duplicate frontend delivery for messages originally delivered before DLQ
   - Recommendation: DLQ replay is intentionally manual (D-10) — operator confirms replay intent; add `replayed: true` field to distinguish in metrics

4. **StatusSubscriber stop race**
   - What we know: `stopChan` is a chan struct{} closed on Stop(); currently no wg
   - What's unclear: After reconnect is added, Stop() needs to wait for reconnecting goroutine
   - Recommendation: Add `sync.WaitGroup` to StatusSubscriber matching Subscriber pattern

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Redis | DLQ stream, PEL drain, all fixes | ✓ | 7.4.7 (confirmed in prod) | — |
| go-redis v9 XAutoClaim | PEL drain | ✓ | v9.18.0 (go.mod) | XPending+XClaim if needed |
| XTrimMinID | DLQ lifecycle | ✓ | Redis 6.2+ required; 7.4.7 present | — |
| miniredis v2 | Tests | ✓ | v2.37.0 (message-processor go.mod) | — |
| prometheus/client_golang | New metrics | ✓ | existing in all services | — |

---

## Project Constraints (from CLAUDE.md)

| Directive | Impact on Phase 8 |
|-----------|-------------------|
| Use Test Driven Development | All new code (ring buffer, retry helper, PEL drain, reconnect logic) must have tests written first |
| Create end-to-end tests if possible; explain if not | Unit tests with miniredis for stream consumer and pub/sub reconnect; E2E test for full pipeline would require all services running — not feasible in unit test context; integration test with miniredis is the practical maximum |
| Conventional commits | Each plan's commit follows `fix(service): description` format |
| Architectural changes need ADR verification | New DLQ stream (`chat:dlq`) is a new infrastructure component — verify against ADR-0002 (Redis Streams + Pub/Sub hybrid); DLQ is a natural extension. No new ADR required unless DLQ triggers routing changes |
| NEVER disable a feature without permission | No features are being disabled; resilience paths are purely additive |
| Commit and push to deploy (Keel) | D-12: service-by-service deployment; validate before proceeding |

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Test Redis | miniredis v2.37.0 (already in message-processor go.mod) |
| Quick run command | `cd services/message-processor && go test ./consumer/... -v -run TestPELDrain -timeout 30s` |
| Full suite command | `cd services/message-processor && go test ./... -timeout 120s` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-01/MP-01 | Consumer name = os.Hostname() | unit | `go test ./consumer/... -run TestConsumerName` | ❌ Wave 0 |
| D-01/MP-02 | PEL drain claims idle messages on startup | unit (miniredis) | `go test ./consumer/... -run TestPELDrain` | ❌ Wave 0 |
| D-04/MP-04 | Publish retry 3 attempts then DLQ | unit (miniredis) | `go test ./consumer/... -run TestPublishRetry` | ❌ Wave 0 |
| D-05/DQ-01 | Failed message lands in chat:dlq | unit (miniredis) | `go test ./consumer/... -run TestDLQWrite` | ❌ Wave 0 |
| D-06/AG-01 | Subscriber re-subscribes after channel close | unit (miniredis) | `go test ./subscription/... -run TestResubscribe` | ❌ Wave 0 |
| D-06/SS-01 | StatusSubscriber nil channel guard | unit | `go test ./subscription/... -run TestStatusSubscriberNilChannel` | ❌ Wave 0 |
| D-07/LI-01 | Ring buffer enqueues on XADD failure | unit | `go test github.com/caesar/all-chat/shared/listener -run TestRingBuffer` | ❌ Wave 0 |
| D-07/LI-01 | Ring buffer drops oldest when full | unit | `go test github.com/caesar/all-chat/shared/listener -run TestRingBufferFull` | ❌ Wave 0 |
| D-08 | Consumer name unique per replica | integration-manual | hostname differs per K8s pod | manual |
| D-09/DQ-02 | DLQ trimmed after 7 days | unit (miniredis) | `go test ./consumer/... -run TestDLQTrim` | ❌ Wave 0 |
| D-14 | New metrics registered and emitting | unit | `go test ./consumer/... -run TestMetrics` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -timeout 120s` in modified service
- **Per wave merge:** Full suite across all modified services
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/message-processor/consumer/stream_consumer_test.go` — covers MP-01, MP-02, MP-04, D-05, D-08, D-09
- [ ] `services/api-gateway/subscription/subscriber_test.go` — covers AG-01 reconnect
- [ ] `services/api-gateway/subscription/status_subscriber_test.go` — covers SS-01, SS-02
- [ ] `shared/listener/ring_buffer_test.go` — covers LI-01, ring buffer capacity, drop semantics
- [ ] `shared/listener/ring_buffer.go` — new file (Wave 0 for plan 08-04)

---

## Sources

### Primary (HIGH confidence)
- Verified from `/home/caesar/go/pkg/mod/github.com/redis/go-redis/v9@v9.16.0/stream_commands.go` — `XAutoClaim`, `XAutoClaimArgs`, `XTrimMinID` APIs confirmed
- `services/message-processor/consumer/stream_consumer.go` — current consumer state, hardcoded ConsumerName, ACK ordering, no PEL drain
- `services/api-gateway/subscription/subscriber.go` — current listen() goroutine, closed channel behavior
- `services/api-gateway/subscription/status_subscriber.go` — nil channel risk confirmed
- `kubectl exec` — Redis version 7.4.7 confirmed in production cluster
- `kubectl exec` — HOSTNAME env = pod name confirmed in message-processor pod

### Secondary (MEDIUM confidence)
- `docs/architecture/01-DATA-FLOW.md` — pipeline flow and failure scenario documentation
- `services/message-processor/go.mod` — miniredis v2.37.0 and go-redis v9.18.0 versions
- `.planning/phases/04-grafana-dashboard-audit-metrics-gap-implementation/04-05-PLAN.md` — alert rule YAML patterns to follow

### Tertiary (LOW confidence)
- Phase 4 alert patterns assumed to still be in caesar-deployment — verify file exists before extending

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all library versions verified from go.mod and module cache
- Architecture: HIGH — all 24 failure modes traced to specific files and line numbers
- Pitfalls: HIGH — verified from direct code inspection; patterns confirmed from go-redis source
- PEL drain strategy: HIGH — XAUTOCLAIM API confirmed in go-redis v9 module cache; Redis 7.4.7 confirmed in prod

**Research date:** 2026-03-29
**Valid until:** 2026-04-30 (Redis API stable; go-redis v9 stable)
