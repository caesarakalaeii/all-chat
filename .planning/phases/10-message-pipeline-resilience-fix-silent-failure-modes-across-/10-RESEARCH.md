# Phase 10: Message Pipeline Resilience — Fix Silent Failure Modes Across Twitch Message Pipeline - Research

**Researched:** 2026-04-08
**Domain:** Go microservices resilience — Redis Streams, Pub/Sub reconnect, atomic flags, zombie detection
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scope & Priority**
- D-01: Fix all 12 identified failure modes (F-01 through F-12), not just High/Medium severity. Full sweep.
- D-02: Add zombie listener detection as a 13th item — this was the pattern behind the April 7 outage and a recurring issue.

**Health Probe Isolation**
- D-03: Replace RLock-based health probe checks with `atomic.Bool`/`atomic.Int64` flags. Zero lock contention — probes must never block on business logic mutexes.
- D-04: Applies to all services where health probe handlers currently acquire any mutex (twitch-listener `verifyCoverageComplete` holds RLock during Redis SCAN, `GetActiveChannelCount`, `IsInitialSyncComplete`).

**Redis Reconnection Strategy**
- D-05: All Redis reconnection paths must use exponential backoff with cap (1s → 2s → 4s → ... cap 30s) plus jitter. Infinite retries until context cancelled.
- D-06: Specifically fix: api-gateway `Subscriber.resubscribe` (currently single-attempt), `StatusSubscriber.reconnect` (currently 3 attempts then permanent give-up), message-processor `XReadGroup` error loop (currently flat 1s sleep).

**Message Durability**
- D-07: Accept that chat messages are ephemeral. When ring buffer fills or DLQ write fails, drop the message but emit a structured log event (not just a metric counter) that can trigger Grafana alerts.
- D-08: No disk-backed buffering, no backpressure to listeners. Keep the architecture simple for recoverable data.

**Zombie Listener Detection**
- D-09: Track received-vs-published message drift. Two atomic counters: `messages_received` (incremented in IRC callback) and `messages_published` (incremented on confirmed XADD/ring buffer accept). If received > 0 but published stalls for N minutes, liveness probe fails → Kubernetes kills and restarts the pod.
- D-10: This avoids false positives on offline channels — when a streamer is offline, both received and published are 0, so no drift is detected.

### Claude's Discretion
- Exact backoff jitter implementation (full jitter vs equal jitter)
- Threshold values for zombie detection (N minutes stall window)
- Whether to use shared/retry utility or per-service retry logic
- PEL drain interval and idle threshold tuning
- Structured log event format for message drops

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

---

## Summary

Phase 10 hardens the Twitch message pipeline against 12 silent failure modes identified in a production resilience audit, plus zombie listener detection added after the April 7 outage. The work spans five services: twitch-listener, message-processor, api-gateway, source-manager, and shared Redis client code.

Phase 8 addressed 24 failure modes in the broader pipeline. Phase 10 targets a distinct, overlapping-but-separate audit of the same pipeline that identified fresh issues. The key difference from Phase 8: Phase 10 focuses specifically on lock-contention paths in health probes (the root cause of the April 7 outage), proper exponential backoff for reconnection paths that Phase 8 only partially fixed, and zombie listener detection as a first-class defense.

Code audit confirms that the Phase 10 F-codes are genuinely unfixed in the current codebase:
- F-01: `verifyCoverageComplete` still holds `m.mu.RLock()` while performing a Redis SCAN [VERIFIED: manager.go line 481-497]
- F-03: `status.Publisher.Publish` has no retry [VERIFIED: status/publisher.go line 40-42]
- F-04: XReadGroup error loop uses `time.Sleep(1 * time.Second)` flat [VERIFIED: stream_consumer.go line 131]
- F-08: `Subscriber.resubscribe` is a single attempt with no backoff [VERIFIED: subscriber.go line 187-240]
- F-09: `StatusSubscriber.reconnect` caps at 3 attempts [VERIFIED: status_subscriber.go line 116-147]
- F-10: twitch-listener and api-gateway create bare `redis.NewClient(&redis.Options{Addr: ...})` without pool tuning [VERIFIED: twitch-listener/cmd/main.go line 90, api-gateway/cmd/main.go line 88]
- F-11: `RenewLeadership` uses non-atomic GET + Expire [VERIFIED: leader.go line 95-122]
- F-12: `RegisterPeer` scans all peer keys on every call [VERIFIED: leader.go line 258-277]

**Primary recommendation:** Group fixes into four plans — (1) twitch-listener atomic flags + status retry + zombie detection, (2) api-gateway exponential backoff reconnect, (3) message-processor XReadGroup backoff + structured drop logging, (4) source-manager atomic leadership renewal + peer scan optimization.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sync/atomic` (stdlib) | Go 1.25.6 | `atomic.Bool`, `atomic.Int64` for lock-free probe flags | Zero allocation, no contention — matches D-03 exactly |
| `math/rand` (stdlib) | Go 1.25.6 | Jitter for exponential backoff | Used throughout codebase |
| `time` (stdlib) | Go 1.25.6 | Timers, tickers for stall detection windows | Standard |
| `github.com/redis/go-redis/v9` | already in go.mod | Redis GETEX (atomic GET+EXPIRE), Lua scripts | Already used; GETEX available since Redis 6.2 |
| `go.uber.org/zap` | already in go.mod | Structured log events for drop alerts | Project standard |
| `github.com/prometheus/client_golang` | already in go.mod | Counters for zombie detection metrics | Project standard |
| `go.uber.org/goleak` | v1.3.0 (direct in twitch-listener go.mod) | Goroutine leak detection in tests | Already present |

[VERIFIED: go.mod files for all services]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `shared/redis` | local | `NewClientWithTracing` with pool tuning (MaxRetries=3, PoolSize=50) | F-10 fix — replace bare `redis.NewClient` in twitch-listener and api-gateway |

[VERIFIED: shared/redis/client.go — already has MaxRetries=3, PoolSize=50, MinIdleConns=10]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `atomic.Bool` for initialSyncDone | Keep `m.mu.RLock()` with optimized lock | Per D-03, probes must never block on business mutex — atomic is mandatory |
| Shared retry utility in `shared/listener` | Per-service retry loop | Shared utility is cleaner but adds SDK dependency; per D-06 discretion left to planner |
| `GETEX` for atomic RenewLeadership | Lua script GET+EXPIRE | GETEX (Redis 6.2+) is cleaner; Lua script works on all versions. K8s Redis 7 in use — GETEX available [ASSUMED: Redis 7 deployed based on CLAUDE.md "Redis 7" mention] |

---

## Architecture Patterns

### Pattern 1: Atomic Flag Migration for Health Probes

**What:** Replace `m.mu.RLock()` calls in health probe-facing methods with `sync/atomic` flags. The business mutex continues to protect the underlying map/bool values — but the probe-facing accessors read a separately maintained atomic copy.

**When to use:** Any getter called from HTTP health probe handlers that would block if the main mutex is held for 40+ seconds (SyncChannels race).

**Current broken pattern:**
```go
// BROKEN: health probe blocks here during SyncChannels (holds m.mu for 40s)
func (m *Manager) IsInitialSyncComplete() bool {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.initialSyncDone
}
```

**Fixed pattern (per D-03/D-04):**
```go
// Source: Go sync/atomic package documentation [CITED: pkg.go.dev/sync/atomic]
type Manager struct {
    // ... existing fields ...
    initialSyncDoneAtomic atomic.Bool   // probe-safe copy
    activeChannelCountAtomic atomic.Int64 // probe-safe copy
}

// Set atomically when business logic completes (inside or outside mutex — atomic is safe)
m.initialSyncDoneAtomic.Store(true)
m.activeChannelCountAtomic.Store(int64(len(m.activeChans)))

// Health probe reads — zero contention
func (m *Manager) IsInitialSyncComplete() bool {
    return m.initialSyncDoneAtomic.Load()
}

func (m *Manager) GetActiveChannelCount() int {
    return int(m.activeChannelCountAtomic.Load())
}
```

**Scope:** `IsInitialSyncComplete`, `GetActiveChannelCount`, `GetFilteredAssignmentCount`, and `verifyCoverageComplete` in `channels/manager.go`. The `verifyCoverageComplete` RLock + Redis SCAN is the most dangerous path (SCAN can block 100ms+ on large keyspaces).

**For verifyCoverageComplete:** Move Redis SCAN outside of mutex entirely. The method is called within SyncChannels which already holds no global mutex during join/part operations. Remove the RLock inside `verifyCoverageComplete` — it doesn't need it since it only reads from `sourceIDMap` (passed as argument) and `m.assignedSourceIDs` (already protected by the outer SyncChannels flow).

### Pattern 2: Exponential Backoff with Jitter

**What:** Replace flat sleeps and capped retry loops with infinite exponential backoff, cap 30s, plus jitter.

**Full jitter algorithm (Claude's Discretion — recommended):**
```go
// Source: AWS blog "Exponential Backoff And Jitter" [CITED: aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/]
// Full jitter: sleep = random_between(0, min(cap, base * 2^attempt))
func backoffDuration(attempt int) time.Duration {
    const base = 1 * time.Second
    const cap = 30 * time.Second
    exp := time.Duration(1<<uint(attempt)) * base
    if exp > cap {
        exp = cap
    }
    // Full jitter: random fraction of the capped window
    jitter := time.Duration(rand.Int63n(int64(exp)))
    return jitter
}
```

**For `Subscriber.resubscribe` (F-08):** Replace single attempt with infinite loop that exits only when `stopChan` is closed:
```go
func (s *Subscriber) resubscribe(overlayID string) {
    for attempt := 0; ; attempt++ {
        select {
        case <-s.stopChan:
            return
        default:
        }
        // ... try to subscribe ...
        if err == nil { return }
        sleep := backoffDuration(attempt)
        select {
        case <-s.stopChan:
            return
        case <-time.After(sleep):
        }
    }
}
```

**For `StatusSubscriber.reconnect` (F-09):** Same pattern — remove the `attempt <= 3` cap, replace with infinite loop. The 3-attempt limit was the "permanent give-up" that D-06 forbids.

**For message-processor XReadGroup error loop (F-04):** Replace `time.Sleep(1 * time.Second)` in `consumeLoop` with exponential backoff that resets on success:
```go
func (c *StreamConsumer) consumeLoop(ctx context.Context) {
    backoffAttempt := 0
    for {
        // ...
        if err := c.readAndProcess(ctx); err != nil {
            if ctx.Err() != nil { return }
            c.logger.Error("Error reading messages", zap.Error(err))
            sleep := backoffDuration(backoffAttempt)
            backoffAttempt++
            select {
            case <-c.stopCh: return
            case <-ctx.Done(): return
            case <-time.After(sleep):
            }
            continue
        }
        backoffAttempt = 0 // Reset on success
    }
}
```

### Pattern 3: Zombie Listener Detection (D-09/D-10)

**What:** Two `atomic.Int64` counters in the twitch-listener — `messagesReceived` and `messagesPublished`. The liveness probe compares a snapshot taken N minutes ago against the current value. If `messagesReceived` advanced (messages came in from IRC) but `messagesPublished` did not advance in the same window, the pod is a zombie.

**False-positive avoidance (D-10):** When no messages arrive from IRC (streamer is offline), both counters stay at zero. No drift, no false alarm.

**Implementation location:** New `zombie/detector.go` in `services/twitch-listener/zombie/` or inline in the IRC callback and StreamPublisher.

```go
// Counters placed on Manager or a dedicated ZombieDetector struct
var messagesReceived atomic.Int64
var messagesPublished atomic.Int64

// In IRC message callback (already fires for every PRIVMSG):
messagesReceived.Add(1)

// In StreamPublisher.Publish, after successful ring buffer accept:
messagesPublished.Add(1)

// Snapshot struct for drift detection
type snapshot struct {
    received  int64
    published int64
    takenAt   time.Time
}
```

**Liveness probe logic:**
```go
// N minutes ago snapshot is stored; probe checks current values
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
    // Existing IRC stale check...
    if h.zombieDetector.IsZombie() {
        c.JSON(503, gin.H{"status": "dead", "reason": "zombie: messages received but publish stalled"})
        return
    }
    c.JSON(200, gin.H{"status": "alive"})
}
```

**Threshold (Claude's Discretion — recommended 5 minutes):** The IRC connection pings every ~5 minutes. A 5-minute stall window balances responsiveness against false positives during transient Redis blips. The ring buffer provides 500ms retry — a 5-minute stall means the ring buffer is full AND backing off, which is definitively a zombie state.

**Snapshot rotation:** A background goroutine takes a snapshot every N/2 minutes and the probe uses the oldest available snapshot. This avoids edge cases where the snapshot was taken mid-burst.

### Pattern 4: Structured Drop Log Events (D-07)

**What:** When ring buffer overflows or DLQ write fails, emit a `zap.Error`-level structured log that matches a Grafana alert pattern — NOT just a metric counter increment.

**Current state:** Ring buffer drops increment `ring_buffer_drops_total` metric. DLQ write failures call `c.metrics.DLQWriteFailures.Inc()`. Neither emits a structured log that Grafana can alert on without PromQL.

**Fix:** Add a `zap.Error` log alongside the metric for each drop event. The log message string must be stable so Loki alerting can match it:
```go
rb.logger.Error("ring_buffer_overflow_drop",
    zap.String("service", rb.serviceName),
    zap.Int("capacity", rb.capacity),
    zap.Int("current_depth", rb.size),
)
```

### Pattern 5: Redis Client Standardization (F-10)

**What:** Replace bare `redis.NewClient(&redis.Options{Addr: ...})` in twitch-listener and api-gateway with `shared/redis.NewClientWithTracing(addr, password, tracingEnabled)`.

**Why:** The bare constructor has no pool tuning (no `PoolSize`, no `MinIdleConns`, no `MaxRetries`). Under connection churn (Redis restart), these services will exhaust connections and fail silently. The shared client has `MaxRetries=3`, `PoolSize=50`, `MinIdleConns=10`.

**Caveat:** The shared client also sets `DialTimeout=5s`, `ReadTimeout=3s`, `WriteTimeout=3s`, `PoolTimeout=4s`. These are all appropriate for production Kubernetes. [VERIFIED: shared/redis/client.go lines 21-32]

### Pattern 6: Atomic Leadership Renewal (F-11)

**What:** Replace the non-atomic GET+Expire in `RenewLeadership` with a Lua script that checks ownership AND renews in a single atomic operation.

**Current broken pattern:**
```go
// TOCTOU: between GET and Expire, another caller could SetNX and win
currentLeader, err := m.client.Get(ctx, key).Result()  // read
// ... gap ...
err = m.client.Expire(ctx, key, m.lockTTL).Err()       // write
```

**Fixed pattern:**
```go
// Source: Redis documentation on atomic operations [CITED: redis.io/docs/manual/patterns/distributed-locks/]
script := redis.NewScript(`
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("expire", KEYS[1], ARGV[2])
    else
        return 0
    end
`)
// Returns 1 if renewed, 0 if lost leadership
renewed, err := script.Run(ctx, m.client, []string{key}, callerID, int(m.lockTTL.Seconds())).Int()
```

Note: `ReleaseLeadership` already uses this Lua pattern correctly (leader.go line 148-156). `RenewLeadership` needs the same treatment.

### Pattern 7: Peer Count Optimization (F-12)

**What:** Replace Redis SCAN-on-every-call in `RegisterPeer` with Redis INCR/DECR on a peer count key, or use a sorted set `ZADD`+`ZCOUNT`.

**Current:** `RegisterPeer` calls `m.client.Scan(ctx, 0, "peer:platform:*", 0).Iterator()` on every heartbeat. With many pods this scans the full keyspace.

**Recommended fix (ZADD approach):**
```go
// Replace individual peer:platform:callerID keys with a single sorted set
// Score = expiry unix timestamp; ZADD NX with score = now+TTL
// Count = ZCOUNT with score range [now, +inf] (unexpired peers)
func (m *Manager) RegisterPeer(ctx context.Context, platform, callerID string) (int, error) {
    key := fmt.Sprintf("peers:%s", platform)
    expiry := float64(time.Now().Add(PeerTTL).Unix())
    if err := m.client.ZAdd(ctx, key, redis.Z{Score: expiry, Member: callerID}).Err(); err != nil {
        return 0, err
    }
    // Remove expired members first (score < now)
    m.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", time.Now().Unix()))
    count, err := m.client.ZCard(ctx, key).Result()
    return int(count), err
}
```

**Alternative:** Keep existing SCAN but cache the result for 5 seconds. Simpler change, same practical effect since peer count is not latency-sensitive.

### Anti-Patterns to Avoid

- **`atomic.Value` for simple bool/int:** Use `atomic.Bool` and `atomic.Int64` directly (Go 1.19+ types). Cleaner API, no interface{} boxing.
- **Resetting backoff attempt counter on every log line:** Only reset the backoff counter when `readAndProcess` returns nil (success). A partial read that hits a timeout (redis.Nil) is NOT an error and should not increment the backoff counter.
- **Zombie detection on absolute counter values:** Compare delta over window, not absolute counters. A pod that processed 1M messages and then went quiet should still trigger detection.
- **Holding mutex during zombie probe check:** The liveness probe runs in Gin's HTTP handler goroutine. It must read only from atomics — never from mutexes.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff | Custom sleep loop | Pattern 2 above using stdlib `time` and `math/rand` | The pattern is 10 lines — a dependency would be overkill; internal utility function is sufficient |
| Distributed lock atomic renewal | Custom multi-command sequence | Lua script (already used in `ReleaseLeadership`) | Redis guarantees atomicity inside a Lua script — no custom coordination needed |
| Goroutine leak detection in tests | Custom goroutine tracking | `goleak` (already in go.mod) | Catches goroutines left by zombie detectors, retry loops |

**Key insight:** This phase is pure plumbing. Every problem has a well-understood solution using primitives already in the codebase (Lua scripts, atomics, stdlib timers). No new dependencies needed.

---

## Common Pitfalls

### Pitfall 1: Atomic Store Placement for Active Channel Count

**What goes wrong:** `activeChannelCountAtomic.Store()` called before `m.activeChans` is fully updated, or only in the "happy path" but not after `partChannel` calls.

**Why it happens:** The atomic mirror must be updated everywhere `m.activeChans` is modified. Currently that is: `joinChannel`, `partChannel`, and `ClearActiveChannels`. Missing any one site means the atomic drifts from ground truth.

**How to avoid:** Search for all `m.activeChans` map writes and add `m.activeChannelCountAtomic.Store(int64(len(m.activeChans)))` after each one. The store happens inside the existing mutex lock, so it's safe.

**Warning signs:** Test where `GetActiveChannelCount()` returns N but `len(m.activeChans)` is M after a part.

### Pitfall 2: Zombie Detection False Negatives When Ring Buffer Is Full

**What goes wrong:** Ring buffer is full (messages being dropped). `messagesReceived` advances. `messagesPublished` does NOT advance because `ring_buffer.Publish()` returns nil even when buffering. Zombie detector sees no drift and does not fire.

**Why it happens:** `RingBufferPublisher.Publish` returns nil on buffer-full to avoid propagating errors to callers (by design). But `messagesPublished` should count "accepted by ring buffer" not "successfully XADD'd to Redis".

**How to avoid:** Increment `messagesPublished` inside `RingBufferPublisher.Publish` AFTER successful `enqueue()` call, not after successful `publishFn` call. The ring buffer guarantees eventual delivery unless the pod dies — "accepted" is the right signal.

**Warning signs:** Ring buffer depth metric climbing while zombie detector stays silent.

### Pitfall 3: Resubscribe Goroutine Leak on Stop

**What goes wrong:** `resubscribe` goroutine is sleeping in the backoff timer when `Stop()` is called. The goroutine wakes up, sees `stopChan` is closed via select, and exits. But the `wg.Add(1)` was already called before the goroutine started. If `Stop()` closes `stopChan` before the goroutine calls `wg.Add(1)`, the WaitGroup count is wrong.

**Why it happens:** Phase 8 fixed this for the happy path (AG-05 in subscriber.go: `wg.Add(1)` before `go s.listen()`), but the infinite retry loop introduces a new timing window: `wg.Add(1)` in the retry loop must happen before spawning the new `listen` goroutine, and the check for `stopChan` must come before `wg.Add`.

**How to avoid:** Pattern: check `stopChan`, then `wg.Add(1)`, then spawn goroutine. Never reverse this order.

### Pitfall 4: StatusSubscriber Reconnect After Stop

**What goes wrong:** `reconnect()` is called from `listen()` via `go s.reconnect()`. If `Stop()` closes `stopChan` and then `listen` returns, the `reconnect` goroutine may have already started and be sleeping in its first backoff. When it wakes up, it calls `wg.Add(1)` and spawns another goroutine. `Stop()` already returned after `wg.Wait()`.

**Why it happens:** The goroutine spawned by `go s.reconnect()` is NOT tracked in the WaitGroup at spawn time. The current 3-attempt code exits quickly so this window is small. Infinite retry makes it larger.

**How to avoid:** Track `reconnect` goroutine in WaitGroup. Before `go s.reconnect()`, call `s.wg.Add(1)` and pass the done signal into `reconnect(s.wg.Done)`.

### Pitfall 5: PEL Drain Timeout Under Large PEL

**What goes wrong:** `drainPEL` (message-processor) uses `XAutoClaim` with `Count: 100` in a loop. If the PEL has 44,000+ entries (as seen in the March 29 incident), startup takes several minutes. During this time, `Start()` has not returned, the service is not consuming new messages, and the Kubernetes readiness probe may time out.

**Why it happens:** `drainPEL` runs before `consumeLoop` in `Start()`. Large PEL = long startup. This is an existing issue that Phase 10's backoff fix for the consume loop does not make worse, but it's worth documenting.

**How to avoid:** This is a known architectural tradeoff (confirmed in Phase 8). The operational fix is to clean PEL manually before deploying (as done on March 29). The drain limit can be capped (e.g., drain only 1000 messages at startup, then drain the rest incrementally in a separate background goroutine). Out of scope for Phase 10 per CONTEXT.md.

### Pitfall 6: F-10 Shared Redis Client — Tracing Flag

**What goes wrong:** `shared/redis.NewClientWithTracing` enables OpenTelemetry tracing when `tracingEnabled=true`. Enabling tracing inadvertently in a service that was not already using OTEL collector configuration causes startup panics or dropped spans.

**Why it happens:** twitch-listener and api-gateway may or may not have OTEL configured.

**How to avoid:** Pass `tracingEnabled = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""` — the same pattern used in services that already adopted tracing. This is safe: if OTEL is not configured, tracing is disabled.

---

## Code Examples

### Atomic Flag in Manager (Pattern 1)

```go
// Source: Go sync/atomic documentation [CITED: pkg.go.dev/sync/atomic]
// In channels/manager.go — add atomic mirrors
type Manager struct {
    // ... existing fields unchanged ...
    initialSyncDoneAtomic    atomic.Bool   // probe-safe: set after first SyncChannels
    activeChannelCountAtomic atomic.Int64  // probe-safe: updated on join/part/clear
    filteredAssignmentCountAtomic atomic.Int64 // probe-safe: set during SyncChannels
}

// After setting initialSyncDone under mu.Lock in SyncChannels:
m.mu.Lock()
m.initialSyncDone = true
m.mu.Unlock()
m.initialSyncDoneAtomic.Store(true) // probe reads this — no lock needed

// Probe accessor — zero contention:
func (m *Manager) IsInitialSyncComplete() bool {
    return m.initialSyncDoneAtomic.Load()
}
```

### Exponential Backoff Utility (Pattern 2)

```go
// Source: AWS jitter paper [CITED: aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/]
// Can live in shared/listener/backoff.go or per-service
func jitteredBackoff(attempt int) time.Duration {
    const base = time.Second
    const cap = 30 * time.Second
    exp := base
    for i := 0; i < attempt; i++ {
        exp *= 2
        if exp > cap {
            exp = cap
            break
        }
    }
    // Full jitter: uniform random in [0, exp)
    if exp <= 0 {
        return base
    }
    return time.Duration(rand.Int63n(int64(exp)))
}
```

### Atomic GET+Expire Renewal (Pattern 6)

```go
// Source: Redis documentation [CITED: redis.io/docs/manual/patterns/distributed-locks/]
// In services/source-manager/election/leader.go
var renewScript = redis.NewScript(`
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("expire", KEYS[1], ARGV[2])
    else
        return 0
    end
`)

func (m *Manager) RenewLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
    key := m.leaderKey(platform, streamID)
    if callerID == "" {
        callerID = m.instanceID
    }
    result, err := renewScript.Run(ctx, m.client, []string{key},
        callerID, int(m.lockTTL.Seconds())).Int()
    if err != nil {
        return false, fmt.Errorf("failed to renew leadership: %w", err)
    }
    return result == 1, nil
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SyncChannels held write-lock during IRC joins | IRC joins happen outside mutex; lock held only for brief map writes | April 7 fix (commit 5041775) | Eliminates pod cycling during startup |
| `Subscriber.resubscribe` single attempt | **Still single attempt** (Phase 10 target) | Not yet changed | Overlay permanently loses subscription on Redis blip |
| `StatusSubscriber.reconnect` 3 attempts | **Still 3 attempts** (Phase 10 target) | Not yet changed | Status updates permanently lost after brief Redis downtime |
| XReadGroup error: flat 1s sleep | **Still flat 1s** (Phase 10 target) | Not yet changed | Thundering herd on Redis restart |
| GET+Expire in RenewLeadership | **Still non-atomic** (Phase 10 target) | Not yet changed | TOCTOU race window on leadership expiry boundary |
| 24 failure modes fixed | Ring buffer, DLQ, PEL drain, unique consumer names, PubSub reconnect | Phase 8 (March 2026) | Bulk of pipeline resilience delivered |

**Deprecated/outdated:**
- Non-atomic `RenewLeadership`: Was acceptable when source-manager was single-pod; now that multiple pods run concurrently, the TOCTOU window causes intermittent dual-leadership.
- Flat sleep in XReadGroup loop: Pre-dates exponential backoff adoption in Phase 8; was accidentally missed.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Redis 7 is deployed in production (GETEX available since 6.2) | Standard Stack | Low — Lua script alternative always works; verify with `redis-server --version` |
| A2 | OTEL is configured conditionally via `OTEL_EXPORTER_OTLP_ENDPOINT` in twitch-listener and api-gateway | Pitfall 6 | Low — safe default is `tracingEnabled=false` |
| A3 | Zombie detection threshold of 5 minutes is appropriate given IRC ping cadence | Pattern 3 | Medium — too short = false positives during Redis restarts; too long = outage drags on. Configurable via env var recommended |

---

## Open Questions (RESOLVED)

1. **Zombie detection threshold configuration** — RESOLVED: Default 5 minutes, configurable via `ZOMBIE_STALL_WINDOW_MINUTES` env var. Documented in Plan 10-01 Task 2.

2. **Backoff utility placement: shared or per-service?** — RESOLVED: `shared/listener/backoff.go` with `JitteredBackoff` function. Three callers (api-gateway subscriber, api-gateway status-subscriber, message-processor consume loop) justify shared extraction. Implemented in Plan 10-01 Task 1b.

3. **F-02 structured log format** — RESOLVED: Upgrade to Error level with stable sentinel `"ring_buffer_overflow_drop"`. Grafana Loki alerts on this exact string. Implemented in Plan 10-01 Task 1b.

---

## Environment Availability

Step 2.6: SKIPPED — this phase is purely code changes with no new external dependencies.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` with `testify/assert`, `testify/require`, `goleak` |
| Config file | none — standard `go test ./...` per service |
| Quick run command | `go test ./... -count=1 -timeout 30s` (per service dir) |
| Full suite command | `make test` (project root) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| F-01 | Health probe methods do not block during SyncChannels | unit | `go test ./channels/... -run TestManager_SyncChannels_DoesNotBlockHealthProbe -v` | ✅ manager_test.go line 538 |
| F-02 | Ring buffer overflow emits Error-level structured log | unit | `go test ./publisher/... -run TestRingBuffer_OverflowLog -v` | ❌ Wave 0 |
| F-03 | status.Publisher retries on Redis failure | unit | `go test ./status/... -run TestPublisher_RetryOnFailure -v` | ❌ Wave 0 |
| F-04 | XReadGroup error loop uses exponential backoff | unit | `go test ./consumer/... -run TestConsumeLoop_ExponentialBackoff -v` | ❌ Wave 0 |
| F-08 | Subscriber.resubscribe retries indefinitely until stopChan | unit | `go test ./subscription/... -run TestSubscriber_ResubscribeRetries -v` | ❌ Wave 0 |
| F-09 | StatusSubscriber.reconnect retries indefinitely | unit | `go test ./subscription/... -run TestStatusSubscriber_ReconnectInfinite -v` | ❌ Wave 0 |
| F-10 | twitch-listener uses shared Redis client pool settings | integration | verify via build + Redis client options inspection | ❌ Wave 0 (smoke test) |
| F-11 | RenewLeadership is atomic (Lua script) | unit | `go test ./election/... -run TestRenewLeadership_Atomic -v` | ❌ Wave 0 |
| F-12 | RegisterPeer does not SCAN on every call | unit | `go test ./election/... -run TestRegisterPeer_NoScan -v` | ❌ Wave 0 |
| Z-01 | Zombie detector fires when received > 0 and published stalled N min | unit | `go test ./zombie/... -run TestZombieDetector_DetectsStall -v` | ❌ Wave 0 |
| Z-02 | Zombie detector does not fire when both counters are zero (offline channel) | unit | `go test ./zombie/... -run TestZombieDetector_NoFalsePositiveWhenOffline -v` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `cd services/{service} && go test ./... -count=1 -timeout 30s`
- **Per wave merge:** `make test`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `services/twitch-listener/publisher/ring_buffer_overflow_log_test.go` — covers F-02
- [ ] `services/twitch-listener/status/publisher_retry_test.go` — covers F-03
- [ ] `services/twitch-listener/zombie/detector.go` + `zombie/detector_test.go` — covers Z-01, Z-02
- [ ] `services/message-processor/consumer/backoff_test.go` — covers F-04
- [ ] `services/api-gateway/subscription/subscriber_retry_test.go` — covers F-08
- [ ] `services/api-gateway/subscription/status_subscriber_retry_test.go` — covers F-09
- [ ] `services/source-manager/election/leader_atomic_test.go` — covers F-11, F-12
- [ ] `shared/listener/backoff.go` + `shared/listener/backoff_test.go` — shared utility (if chosen per open question 2)

---

## Security Domain

V5 Input Validation: not applicable (internal services, no user input in this phase).
V6 Cryptography: not applicable (no new cryptographic operations).

The only relevant security consideration is the Lua script for atomic leadership renewal — Lua scripts run server-side in Redis with no injection risk since arguments are passed as ARGV not concatenated into the script string. [VERIFIED: leader.go existing Lua pattern in ReleaseLeadership follows this correctly]

---

## Sources

### Primary (HIGH confidence)

- [VERIFIED: codebase] `services/twitch-listener/channels/manager.go` — confirmed RLock in probe-facing methods, verifyCoverageComplete holds lock during SCAN
- [VERIFIED: codebase] `services/api-gateway/subscription/subscriber.go` — confirmed single-attempt resubscribe
- [VERIFIED: codebase] `services/api-gateway/subscription/status_subscriber.go` — confirmed 3-attempt cap in reconnect
- [VERIFIED: codebase] `services/message-processor/consumer/stream_consumer.go` — confirmed flat 1s sleep in consumeLoop
- [VERIFIED: codebase] `services/twitch-listener/cmd/main.go`, `services/api-gateway/cmd/main.go` — confirmed bare redis.NewClient without pool tuning
- [VERIFIED: codebase] `services/source-manager/election/leader.go` — confirmed GET+Expire TOCTOU in RenewLeadership, SCAN in RegisterPeer
- [VERIFIED: codebase] `shared/redis/client.go` — confirmed pool tuning in NewClientWithTracing
- [VERIFIED: codebase] `shared/listener/ring_buffer.go` — confirmed existing ring buffer implementation
- [VERIFIED: codebase] `.planning/debug/resolved/twitch-messages-not-reaching-overlay.md` — April 7 outage root cause and fix
- [VERIFIED: codebase] `.planning/debug/resolved/twitch-messages-not-reaching-overlay-hesplayingroblox.md` — March 29 PEL incident

### Secondary (MEDIUM confidence)

- [CITED: pkg.go.dev/sync/atomic] Go 1.19+ `atomic.Bool` and `atomic.Int64` types
- [CITED: aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/] Full jitter algorithm recommendation
- [CITED: redis.io/docs/manual/patterns/distributed-locks/] Lua script for atomic GET+EXPIRE

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verified from codebase, all dependencies already present
- Architecture: HIGH — code audit confirms each F-code is unaddressed; patterns are well-established
- Pitfalls: HIGH — derived from actual production incidents (April 7, March 29) and code inspection

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (stable Go/Redis ecosystem; only caveat is Redis version for GETEX)
