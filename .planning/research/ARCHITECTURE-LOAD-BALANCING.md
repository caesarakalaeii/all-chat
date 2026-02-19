# Architecture Research: Distributed Channel Sharding for All-Chat Listeners

**Domain:** Distributed load balancing for real-time chat message aggregation microservices
**Researched:** 2026-02-19
**Confidence:** HIGH

---

## Recommended Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Existing Architecture (DO NOT CHANGE)                │
│  Listener → Redis Streams → Message Processor → Redis Pub/Sub → Gateway │
└─────────────────────────────────────────────────────────────────────────┘

                               ↓ ADD BELOW ↓

┌─────────────────────────────────────────────────────────────────────────┐
│                        Load Balancing Layer                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Shard Coordinator (NEW SERVICE)                              │       │
│  │  • Consistent hashing ring                                    │       │
│  │  • Channel → Pod assignment                                   │       │
│  │  • Rebalancing orchestration                                  │       │
│  │  • API: /assignments, /rebalance                              │       │
│  └────────────────────┬─────────────────────────────────────────┘       │
│                       │ writes assignments to                            │
│                       ▼                                                   │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Redis Assignment Registry (NEW DATA STRUCTURES)              │       │
│  │  • HASH: shard:assignments:{platform}                         │       │
│  │    └─> {channel_id: pod_id}                                   │       │
│  │  • HASH: shard:load:{platform}                                │       │
│  │    └─> {pod_id: channel_count}                                │       │
│  │  • SET: shard:pods:{platform}                                 │       │
│  │    └─> {pod_id, pod_id, ...}                                  │       │
│  │  • STREAM: shard:migrations:{platform}                        │       │
│  │    └─> Migration events for graceful handoff                  │       │
│  └────────────────────┬─────────────────────────────────────────┘       │
│                       │ read assignments from                            │
│                       ▼                                                   │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Enhanced Listener Pods (MODIFY EXISTING)                     │       │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐              │       │
│  │  │ Listener 1 │  │ Listener 2 │  │ Listener 3 │              │       │
│  │  │ Channels:  │  │ Channels:  │  │ Channels:  │              │       │
│  │  │ A, D, G    │  │ B, E, H    │  │ C, F, I    │              │       │
│  │  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘              │       │
│  │         │                │                │                    │       │
│  │         │  Heartbeat every 10s (load metrics)                 │       │
│  │         └────────────────┴────────────────┘                   │       │
│  │                          ▼                                     │       │
│  │  ┌──────────────────────────────────────────────────────────┐ │       │
│  │  │  Load Monitor (NEW COMPONENT IN EACH POD)                 │ │       │
│  │  │  • Tracks: channel count, message rate, memory usage      │ │       │
│  │  │  • Publishes to Redis: shard:metrics:{platform}:{pod_id}  │ │       │
│  │  │  • TTL: 30s (pod considered dead if not refreshed)        │ │       │
│  │  └──────────────────────────────────────────────────────────┘ │       │
│  └──────────────────────────────────────────────────────────────┘       │
│                                                                           │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With | New or Modified |
|-----------|----------------|-------------------|-----------------|
| **Shard Coordinator** | Consistent hashing, assignment computation, rebalancing orchestration | Redis (assignment registry), Listeners (migration API) | **NEW SERVICE** |
| **Assignment Registry (Redis)** | Stores channel→pod mappings, pod load metrics, migration events | Shard Coordinator (writes), Listeners (reads) | **NEW DATA STRUCTURES** |
| **Load Monitor** | Tracks per-pod metrics (channels, messages/sec, memory), heartbeat | Redis (writes metrics), runs inside each listener pod | **NEW COMPONENT** |
| **Enhanced Listener** | Same as before + reads assignments, handles migrations, reports load | Redis Streams (existing), Assignment Registry (new), Shard Coordinator (new) | **MODIFIED** |
| **Source Manager** | YouTube leader election (UNCHANGED) | YouTube Listener only | **UNCHANGED** |

---

## Integration with Existing Architecture

### What Stays the Same

```
Listener → Redis Streams (chat:raw) → Message Processor → Redis Pub/Sub → Gateway
```

**NO CHANGES to message flow.** Sharding only affects which listener pod connects to which channel. Once connected, messages flow identically through existing pipeline.

### What Changes

#### 1. Listener Startup Sequence (NEW)

**Before (current)**:
```
Listener starts → Queries database for all active channels → Joins ALL channels
```

**After (with sharding)**:
```
1. Listener starts → Registers with Shard Coordinator (pod_id, platform)
2. Shard Coordinator assigns subset of channels → Writes to Assignment Registry
3. Listener queries Assignment Registry → Joins ONLY assigned channels
4. Listener starts Load Monitor → Heartbeat every 10s
```

#### 2. Channel Addition (NEW)

**Before (current)**:
```
New overlay created → All listener pods query database → All pods try to join channel
```

**After (with sharding)**:
```
1. New overlay created → Source Manager notifies (LISTEN/NOTIFY)
2. Shard Coordinator receives notification → Runs consistent hash → Assigns to Pod 2
3. Writes to Assignment Registry: {channel_X: pod_2}
4. Pod 2 polls Assignment Registry → Detects new assignment → Joins channel_X
```

#### 3. Pod Scaling (NEW)

**Before (current)**:
```
HPA scales to 3 pods → Each pod joins ALL channels → N * channels connections
```

**After (with sharding)**:
```
1. HPA scales to 3 pods → New pod registers with Shard Coordinator
2. Shard Coordinator triggers rebalancing:
   - Recomputes consistent hash with 3 nodes
   - Identifies channels moving from Pod 1/2 to Pod 3
   - Publishes migration events to Redis Stream: shard:migrations:{platform}
3. Pod 1/2 consume migration events → Gracefully PART channels (no message loss)
4. Pod 3 receives assignments → Joins channels
5. Rebalancing complete (~60s total)
```

---

## Data Flow

### Assignment Flow (Normal Operation)

```
[Source Manager: Active Source Registry]
                ↓ (queries every 30s)
[Shard Coordinator: Consistent Hash]
                ↓ computes assignments
[Redis Assignment Registry]
  • shard:assignments:twitch = {channel_A: pod_1, channel_B: pod_2}
  • shard:load:twitch = {pod_1: 50, pod_2: 45, pod_3: 48}
                ↓ (polls every 30s)
[Listener Pods]
  • Pod 1 reads assignments → Joins {channel_A, channel_D, channel_G}
  • Pod 2 reads assignments → Joins {channel_B, channel_E, channel_H}
  • Pod 3 reads assignments → Joins {channel_C, channel_F, channel_I}
```

### Rebalancing Flow (Pod Added/Removed)

```
[Kubernetes Event: Pod 4 starts]
                ↓
[Pod 4 registers with Shard Coordinator]
                ↓
[Shard Coordinator: Rebalance Trigger]
  1. Recompute consistent hash (4 nodes now)
  2. Identify moved channels: {channel_A, channel_E, channel_I}
  3. Publish migration events to Redis Stream
                ↓
[Redis Stream: shard:migrations:twitch]
  • {from: pod_1, to: pod_4, channel: channel_A, timestamp: ...}
  • {from: pod_2, to: pod_4, channel: channel_E, timestamp: ...}
  • {from: pod_3, to: pod_4, channel: channel_I, timestamp: ...}
                ↓
[Source Pods: XREADGROUP consume migrations]
  • Pod 1 sees migration → Gracefully PARTs channel_A → Acks event
  • Pod 2 sees migration → Gracefully PARTs channel_E → Acks event
  • Pod 3 sees migration → Gracefully PARTs channel_I → Acks event
                ↓ (after PART complete)
[Destination Pod: Reads updated assignments]
  • Pod 4 polls Assignment Registry → Sees {channel_A, channel_E, channel_I}
  • Pod 4 JOINs new channels → Rebalancing complete
```

### Heartbeat & Load Monitoring Flow

```
[Listener Pod: Load Monitor goroutine]
  Every 10s:
    1. Count active channels
    2. Calculate message rate (messages/sec in last 10s)
    3. Read memory usage (runtime.MemStats)
    4. Publish to Redis:
       SET shard:metrics:twitch:pod_1 "{channels: 50, msg_rate: 120, mem_mb: 45}" EX 30
                ↓
[Shard Coordinator: Health Check]
  Every 30s:
    1. Read all shard:metrics:twitch:* keys
    2. Detect missing pods (no heartbeat in 30s) → Trigger rebalancing
    3. Detect overloaded pods (msg_rate > 500/s) → Trigger rebalancing
    4. Update shard:load:twitch with current channel counts
```

---

## Architectural Patterns

### Pattern 1: Consistent Hashing with Virtual Nodes

**What:** Map channels to listener pods using consistent hashing on a circular ring with virtual nodes (100-200 per physical pod) for even distribution.

**When to use:** All platforms (Twitch, YouTube, Kick, TikTok) with >100 channels and >2 listener pods.

**Trade-offs:**
- **Pros:** Minimizes reassignments on pod changes (only K/N channels move), even load distribution with virtual nodes, deterministic assignments.
- **Cons:** Additional complexity vs round-robin, requires maintaining hash ring state.

**Implementation (Go):**
```go
// shard-coordinator/hashing/ring.go
type ConsistentHashRing struct {
    ring        map[uint32]string // hash position → pod_id
    sortedKeys  []uint32          // sorted hash positions
    virtualNodes int              // 150 virtual nodes per pod
}

func (r *ConsistentHashRing) GetAssignment(channelID string) string {
    hash := crc32.ChecksumIEEE([]byte(channelID))
    idx := sort.Search(len(r.sortedKeys), func(i int) bool {
        return r.sortedKeys[i] >= hash
    })
    if idx == len(r.sortedKeys) {
        idx = 0 // wrap around
    }
    return r.ring[r.sortedKeys[idx]]
}

func (r *ConsistentHashRing) AddNode(podID string) {
    for i := 0; i < r.virtualNodes; i++ {
        virtualKey := fmt.Sprintf("%s#%d", podID, i)
        hash := crc32.ChecksumIEEE([]byte(virtualKey))
        r.ring[hash] = podID
        r.sortedKeys = append(r.sortedKeys, hash)
    }
    sort.Slice(r.sortedKeys, func(i, j int) bool {
        return r.sortedKeys[i] < r.sortedKeys[j]
    })
}
```

**Why this pattern:**
- Industry standard (Cassandra, DynamoDB, Kafka)
- Proven to minimize disruption during scaling (only ~K/N keys reassigned)
- Virtual nodes solve uneven distribution problem

**Source:** [Consistent Hashing Explained](https://ably.com/blog/implementing-efficient-consistent-hashing), [Toptal Guide to Consistent Hashing](https://www.toptal.com/big-data/consistent-hashing), [OneUpTime Implementation](https://oneuptime.com/blog/post/2026-01-30-consistent-hashing-implementation/view)

### Pattern 2: Redis-Based Assignment Registry (Client-Side Sharding)

**What:** Store all channel→pod assignments in Redis HASHes, with listeners polling for updates every 30s. Coordinator writes, listeners read.

**When to use:** When avoiding complex leader election and preferring stateless coordinator.

**Trade-offs:**
- **Pros:** Simple to implement, no coordinator failure = no reassignments, listeners remain autonomous, Redis provides atomic updates.
- **Cons:** Eventual consistency (30s polling delay), all listeners must poll Redis, stale assignments if Redis fails.

**Implementation (Redis data structures):**
```
# Channel assignments (platform-specific)
HSET shard:assignments:twitch channel_xqc pod_1
HSET shard:assignments:twitch channel_shroud pod_2
HGET shard:assignments:twitch channel_xqc  → "pod_1"

# Load tracking (for rebalancing decisions)
HSET shard:load:twitch pod_1 50
HSET shard:load:twitch pod_2 45
HINCRBY shard:load:twitch pod_1 1  # Increment when channel assigned

# Active pods (for detecting pod failures)
SADD shard:pods:twitch pod_1
SADD shard:pods:twitch pod_2
SREM shard:pods:twitch pod_1  # Remove on pod shutdown

# Metrics (with TTL for health checks)
SET shard:metrics:twitch:pod_1 '{"channels":50,"msg_rate":120,"mem_mb":45}' EX 30
```

**Why this pattern:**
- Matches existing source-manager pattern (leader election uses Redis locks)
- Scales horizontally (Redis Cluster supports HASH sharding)
- Atomic operations prevent split-brain assignments

**Source:** [Redis Ring for Consistent Hashing](https://redis.uptrace.dev/guide/ring.html), [Redis Sharding Best Practices 2026](https://www.dragonflydb.io/guides/redis-sharding-how-it-works-pros-cons-best-practices)

### Pattern 3: Cooperative Rebalancing (Kafka-Style)

**What:** When rebalancing, only migrate channels that MUST move (due to consistent hash changes). Listeners consume migration events from Redis Stream, gracefully PART channels, then acknowledge completion.

**When to use:** During pod scaling (up/down), pod failures (detected via missing heartbeat).

**Trade-offs:**
- **Pros:** Minimizes disruption (only affected channels pause), cooperative (no forced disconnects), preserves message ordering.
- **Cons:** Slower rebalancing (~60s vs instant), requires migration protocol, complex failure handling.

**Implementation (migration event flow):**
```go
// shard-coordinator/rebalancer.go
type MigrationEvent struct {
    FromPod   string    `json:"from_pod"`
    ToPod     string    `json:"to_pod"`
    ChannelID string    `json:"channel_id"`
    Platform  string    `json:"platform"`
    Timestamp time.Time `json:"timestamp"`
}

func (r *Rebalancer) PublishMigrations(migrations []MigrationEvent) {
    for _, m := range migrations {
        payload, _ := json.Marshal(m)
        r.redis.XAdd(ctx, &redis.XAddArgs{
            Stream: fmt.Sprintf("shard:migrations:%s", m.Platform),
            Values: map[string]interface{}{"event": payload},
        })
    }
}

// twitch-listener/migration_handler.go
func (l *Listener) ConsumeMigrations(ctx context.Context) {
    for {
        streams, _ := l.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
            Group:    "listener-group",
            Consumer: l.podID,
            Streams:  []string{"shard:migrations:twitch", ">"},
            Count:    10,
            Block:    5 * time.Second,
        })

        for _, msg := range streams[0].Messages {
            var event MigrationEvent
            json.Unmarshal([]byte(msg.Values["event"].(string)), &event)

            if event.FromPod == l.podID {
                l.ircClient.Depart(event.ChannelID) // Graceful PART
                l.redis.XAck(ctx, "shard:migrations:twitch", "listener-group", msg.ID)
            }
        }
    }
}
```

**Why this pattern:**
- Prevents message loss during rebalancing (PART before new pod JOINs)
- Follows Kafka's cooperative rebalancing model (introduced Kafka 2.4)
- Graceful degradation (if migration fails, channel simply stays on old pod)

**Source:** [Kafka Partition Rebalancing](https://oneuptime.com/blog/post/2026-02-02-kafka-partition-rebalancing/view), [Dynamic Work Rebalancing in Dataflow](https://docs.cloud.google.com/dataflow/docs/dynamic-work-rebalancing)

### Pattern 4: Heartbeat-Based Failure Detection

**What:** Each listener pod publishes load metrics to Redis every 10s with 30s TTL. Shard Coordinator considers pod dead if no heartbeat for 30s, triggering automatic rebalancing.

**When to use:** For detecting pod crashes, network partitions, or unresponsive listeners.

**Trade-offs:**
- **Pros:** Simple to implement, no additional infrastructure (uses Redis TTL), fast detection (30s), prevents split-brain (expired keys mean dead pod).
- **Cons:** False positives if Redis overloaded, requires clock synchronization, network partitions cause unnecessary rebalancing.

**Implementation:**
```go
// twitch-listener/load_monitor.go
func (l *Listener) StartLoadMonitor(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            metrics := LoadMetrics{
                Channels: l.channelManager.Count(),
                MsgRate:  l.messageCounter.RatePer10s(),
                MemoryMB: l.getMemoryUsage(),
            }
            payload, _ := json.Marshal(metrics)

            l.redis.Set(ctx,
                fmt.Sprintf("shard:metrics:%s:%s", l.platform, l.podID),
                payload,
                30*time.Second, // TTL
            )
        case <-ctx.Done():
            return
        }
    }
}

// shard-coordinator/health_checker.go
func (c *Coordinator) DetectFailedPods(ctx context.Context) []string {
    activePods, _ := c.redis.SMembers(ctx, "shard:pods:twitch")
    failedPods := []string{}

    for _, podID := range activePods {
        key := fmt.Sprintf("shard:metrics:twitch:%s", podID)
        exists, _ := c.redis.Exists(ctx, key)

        if exists == 0 {
            failedPods = append(failedPods, podID)
            c.redis.SRem(ctx, "shard:pods:twitch", podID)
        }
    }

    return failedPods
}
```

**Why this pattern:**
- Matches Kubernetes liveness probe pattern (pods self-report health)
- Redis TTL provides automatic cleanup (no manual cleanup needed)
- Prevents "zombie pods" from holding channel assignments

**Source:** [Redis Kubernetes Pod Stability](https://redis.io/docs/latest/operate/kubernetes/recommendations/pod-stability/), [StatefulSet Management](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)

---

## Integration Points with Existing Services

### Source Manager Integration

**Current Role:** YouTube leader election (prevents duplicate polling), active source registry (syncs from database).

**New Role (EXTENDED):**
1. **UNCHANGED:** YouTube leader election (remains same)
2. **NEW:** Publishes source changes to Shard Coordinator
   - When new source added → Notify Shard Coordinator via HTTP POST `/sources/added`
   - When source removed → Notify Shard Coordinator via HTTP POST `/sources/removed`
3. **NEW:** Provides full source list to Shard Coordinator on startup (bootstrap)

**Why extend vs replace:** Source Manager already has database sync logic (30s polling + LISTEN/NOTIFY). Reusing this avoids duplicate database queries and maintains single source of truth.

**API contract (new endpoints on Source Manager):**
```go
// POST /webhook/subscribe
// Shard Coordinator registers for source change notifications
type WebhookSubscription struct {
    URL      string   `json:"url"`       // http://shard-coordinator:8090/sources/changed
    Platforms []string `json:"platforms"` // ["twitch", "kick", "tiktok"]
}

// Source Manager sends POST to webhook URL when sources change:
type SourceChangeEvent struct {
    Action    string `json:"action"`     // "added" | "removed"
    Platform  string `json:"platform"`   // "twitch"
    ChannelID string `json:"channel_id"` // "xqc"
    OverlayID string `json:"overlay_id"` // "uuid"
}
```

### Listener Service Integration

**Modified Components:**

1. **Startup sequence (NEW):**
   - `cmd/main.go`: Register with Shard Coordinator on startup
   - `channels/manager.go`: Query Assignment Registry instead of database
   - `channels/syncer.go`: Poll Assignment Registry every 30s (not database)

2. **Load monitoring (NEW):**
   - `monitoring/load_monitor.go`: New package for heartbeat and metrics
   - Runs as goroutine in `main()`
   - Publishes to Redis every 10s

3. **Migration handling (NEW):**
   - `migration/handler.go`: New package for consuming migration events
   - XREADGROUP on `shard:migrations:{platform}`
   - Gracefully PART channels when migration event received

4. **Shutdown (MODIFIED):**
   - `cmd/main.go`: On SIGTERM, notify Shard Coordinator of shutdown
   - Shard Coordinator immediately triggers rebalancing (don't wait for heartbeat timeout)

**Code changes (estimated):**
```
twitch-listener/
├── cmd/main.go                    [MODIFIED: Add registration, load monitor, migration handler]
├── channels/manager.go            [MODIFIED: Read from Redis, not database]
├── channels/syncer.go             [MODIFIED: Poll Assignment Registry]
├── monitoring/load_monitor.go     [NEW: Heartbeat and metrics]
├── migration/handler.go           [NEW: Migration event consumer]
└── client/shard_coordinator.go    [NEW: HTTP client for Shard Coordinator API]
```

**Same changes for:** `youtube-listener`, `kick-listener`, `tiktok-listener`.

### Database Integration

**NO CHANGES to database schema.** `overlay_chat_sources` table remains single source of truth.

**Data flow:**
```
Database (overlay_chat_sources)
  ↓ (Source Manager polls every 30s)
Source Manager (active source registry)
  ↓ (Webhook on changes)
Shard Coordinator (computes assignments)
  ↓ (Writes to Redis)
Assignment Registry
  ↓ (Listeners poll every 30s)
Listener Pods
```

**Why not listeners query database directly?**
- Reduces database load (N listeners × 30s polling → 1 Source Manager × 30s polling)
- Maintains separation of concerns (Source Manager owns source registry)
- Enables centralized assignment logic (Shard Coordinator computes once, all listeners read)

---

## Build Order (Dependency Graph)

**Phase 1: Foundation (No dependencies)**
1. Redis data structures design (document keys, data types, TTLs)
2. Consistent hashing library (`shard-coordinator/hashing/ring.go`)
3. Load monitoring component (`monitoring/load_monitor.go`)

**Phase 2: Coordinator Service (Depends on Phase 1)**
4. Shard Coordinator skeleton (HTTP server, health checks)
5. Assignment computation logic (consistent hash → Redis writes)
6. Rebalancing orchestrator (detect pod changes, compute migrations)
7. Health checker (read heartbeats, detect failures)

**Phase 3: Listener Integration (Depends on Phase 2)**
8. Listener registration logic (POST to Shard Coordinator on startup)
9. Assignment polling (read from Redis Assignment Registry)
10. Migration handler (XREADGROUP consume, graceful PART)
11. Load monitor integration (publish metrics every 10s)

**Phase 4: Source Manager Integration (Depends on Phase 3)**
12. Source Manager webhook endpoints (subscribe, notify)
13. Shard Coordinator webhook client (receive source changes)

**Phase 5: Testing & Observability (Depends on Phase 4)**
14. Integration tests (simulate pod scaling, failures)
15. Prometheus metrics (assignment counts, rebalancing duration)
16. Grafana dashboards (load distribution, migration events)

**Parallel tracks:**
- Phase 1 + Phase 4 can run in parallel (independent codebases)
- Load monitoring (Phase 1.3) can be developed alongside coordinator (Phase 2)

**Critical path:** Phase 2 (Coordinator) blocks everything. Must complete first.

---

## State Management Approach

### Redis Data Structures (Detailed)

#### 1. Assignment Registry (HASH)

**Key:** `shard:assignments:{platform}` (e.g., `shard:assignments:twitch`)

**Structure:** HASH (field = channel_id, value = pod_id)

**Operations:**
```
HSET shard:assignments:twitch xqc pod_1       # Assign channel to pod
HGET shard:assignments:twitch xqc             # Get assignment
HDEL shard:assignments:twitch xqc             # Remove assignment
HGETALL shard:assignments:twitch              # Get all assignments
```

**TTL:** None (persistent until explicitly deleted)

**Why HASH:** O(1) lookups, atomic updates, supports partial updates (HSET single channel).

#### 2. Load Tracking (HASH)

**Key:** `shard:load:{platform}` (e.g., `shard:load:twitch`)

**Structure:** HASH (field = pod_id, value = channel_count)

**Operations:**
```
HSET shard:load:twitch pod_1 50               # Set load
HINCRBY shard:load:twitch pod_1 1             # Increment (new channel assigned)
HINCRBY shard:load:twitch pod_1 -1            # Decrement (channel removed)
HGETALL shard:load:twitch                     # Get all loads
```

**TTL:** None (updated atomically with assignments)

**Why HASH:** Atomic increments (HINCRBY), efficient for coordinator to compute rebalancing.

#### 3. Active Pods (SET)

**Key:** `shard:pods:{platform}` (e.g., `shard:pods:twitch`)

**Structure:** SET (members = pod_id)

**Operations:**
```
SADD shard:pods:twitch pod_1                  # Register pod
SREM shard:pods:twitch pod_1                  # Unregister pod
SMEMBERS shard:pods:twitch                    # Get all active pods
SISMEMBER shard:pods:twitch pod_1             # Check if pod active
```

**TTL:** None (pods explicitly remove themselves on shutdown)

**Why SET:** Fast membership checks, automatic deduplication, supports set operations.

#### 4. Metrics (STRING with TTL)

**Key:** `shard:metrics:{platform}:{pod_id}` (e.g., `shard:metrics:twitch:pod_1`)

**Structure:** STRING (JSON payload)

**Payload:**
```json
{
  "channels": 50,
  "msg_rate": 120,
  "mem_mb": 45,
  "last_updated": "2026-02-19T10:30:00Z"
}
```

**Operations:**
```
SET shard:metrics:twitch:pod_1 '{"channels":50,...}' EX 30  # Set with 30s TTL
GET shard:metrics:twitch:pod_1                              # Get metrics
EXISTS shard:metrics:twitch:pod_1                           # Check if pod alive
```

**TTL:** 30 seconds (auto-expires if pod doesn't refresh)

**Why STRING + TTL:** Automatic cleanup, simple to implement, pods self-report.

#### 5. Migration Events (STREAM)

**Key:** `shard:migrations:{platform}` (e.g., `shard:migrations:twitch`)

**Structure:** STREAM (ordered log of migration events)

**Operations:**
```
# Coordinator publishes migration
XADD shard:migrations:twitch * event '{"from_pod":"pod_1","to_pod":"pod_2","channel":"xqc"}'

# Listeners consume with consumer groups
XREADGROUP GROUP listener-group pod_1 COUNT 10 BLOCK 5000 STREAMS shard:migrations:twitch >

# Listener acknowledges completion
XACK shard:migrations:twitch listener-group 1234567890-0
```

**TTL:** Stream trimmed to last 10,000 entries (MAXLEN ~ 10000)

**Why STREAM:** Ordered event log, consumer groups prevent duplicate processing, acknowledgment ensures completion.

### Failure Scenarios

#### Scenario 1: Listener Pod Crashes

**Detection:** Heartbeat missing for 30s → Shard Coordinator detects via `EXISTS shard:metrics:twitch:pod_1` returning 0.

**Recovery:**
1. Coordinator removes pod from `shard:pods:twitch` (SREM)
2. Triggers rebalancing: reassigns crashed pod's channels to remaining pods
3. Publishes migration events for newly assigned channels
4. Remaining pods consume migration events → JOIN new channels
5. Recovery time: ~30s (detection) + ~30s (rebalancing) = **60s total**

**Message loss:** None (channels remain unmonitored for 60s, but no messages lost from Redis Streams).

#### Scenario 2: Shard Coordinator Crashes

**Impact:** No new assignments, but existing assignments remain valid (stored in Redis).

**Recovery:**
1. Kubernetes restarts coordinator pod (~10s)
2. Coordinator reads Assignment Registry from Redis (bootstraps state)
3. Resumes health checking and rebalancing

**Degradation:** Listeners continue operating normally. New overlays not assigned until coordinator recovers.

**Message loss:** None.

#### Scenario 3: Redis Failure

**Impact:** Assignment Registry unavailable, listeners cannot read assignments.

**Recovery (Redis persistence enabled):**
1. Redis restarts, loads from RDB/AOF (~10s for small datasets)
2. Assignment Registry restored
3. Listeners resume polling, rejoin channels

**Recovery (Redis persistence disabled):**
1. Redis restarts, all assignments lost
2. Shard Coordinator detects empty `shard:assignments:*` keys
3. Recomputes assignments from Source Manager's active source registry
4. Publishes full assignment set to Redis
5. Listeners poll, rejoin all channels
6. Recovery time: ~60s

**Message loss:** Potential (if Redis down >60s and messages published to offline channels).

#### Scenario 4: Network Partition (Coordinator ↔ Redis)

**Impact:** Coordinator cannot write assignments, listeners read stale assignments.

**Behavior:**
- Listeners continue operating with existing assignments (eventual consistency)
- New pods cannot register (registration API fails)
- Pod failures not detected (coordinator cannot read heartbeats)

**Recovery:** Partition heals → Coordinator resumes operations, rebalances if pods added/removed during partition.

**Message loss:** None (existing channels continue operating).

---

## Scalability Considerations

### Platform-Specific Constraints

#### Twitch IRC

**Constraints:**
- 100 channels per connection (broadcaster must mod bot or grant permission)
- Rate limit: 20 JOIN commands per 10 seconds per connection

**Scaling strategy:**
- **<100 channels:** 1 pod, 1 IRC connection (no sharding needed)
- **100-1000 channels:** 2-10 pods, shard across pods, each pod maintains 1 connection
- **>1000 channels:** Multiple IRC connections per pod (not supported by go-twitch-irc out-of-box, requires custom client)

**Rebalancing impact:**
- Twitch IRC JOIN/PART is fast (~50ms per command)
- Rebalancing 1000 channels = 50 commands × 50ms = 2.5s per pod
- Total rebalancing time: ~10s (including coordinator computation)

**Source:** [Twitch IRC Join Limits](https://discuss.dev.twitch.com/t/giving-broadcasters-control-concurrent-join-limits-for-irc-and-eventsub/54997)

#### YouTube Polling

**Constraints:**
- Quota: 1,009,000 units/day (was 10,000 before increase request)
- Each poll costs 5 units
- Polling interval: 10 seconds (6 polls/minute)
- Max streams per day: 1,009,000 / (5 × 6 × 60 × 24) ≈ 23 concurrent streams

**Scaling strategy:**
- YouTube already uses leader election (Source Manager) to prevent duplicate polling
- Sharding not applicable (poll count is bottleneck, not connection count)
- **Use existing leader election, DO NOT shard YouTube**

**Rebalancing impact:** None (leader election handles failover, no sharding needed).

#### Kick WebSocket

**Constraints:**
- Pusher WebSocket: 100 channels per connection (Pusher free tier)
- Rate limit: Unknown (community reports suggest generous limits)

**Scaling strategy:**
- **<100 channels:** 1 pod, 1 WebSocket connection
- **100-1000 channels:** 2-10 pods, shard across pods
- **>1000 channels:** Multiple WebSocket connections per pod (connection pooling)

**Rebalancing impact:**
- WebSocket subscribe/unsubscribe is fast (~100ms)
- Similar to Twitch (2-10s for 1000 channels)

#### TikTok Unofficial Library

**Constraints:**
- Uses unofficial WebRTC-based library (may break with TikTok changes)
- Connection limits unknown (community reports suggest 10-20 concurrent streams)

**Scaling strategy:**
- **<10 streams:** 1 pod
- **10-50 streams:** 2-5 pods, shard across pods
- **>50 streams:** Not recommended (unofficial API unreliable at scale)

**Rebalancing impact:** Fast (WebRTC connection setup ~500ms per stream).

### Load Distribution Metrics

| Scale | Twitch Pods | Kick Pods | TikTok Pods | Coordinator Overhead | Rebalancing Time |
|-------|-------------|-----------|-------------|----------------------|------------------|
| 50 channels | 1 | 1 | 1 | Negligible (<1% CPU) | 5s |
| 500 channels | 5 | 5 | N/A | Low (5% CPU) | 30s |
| 1000 channels | 10 | 10 | N/A | Medium (10% CPU) | 60s |
| 5000 channels | 50 | 50 | N/A | High (25% CPU) | 5min |

**Coordinator resource usage (estimated):**
- **CPU:** 0.1% per 100 channels (consistent hash computation)
- **Memory:** 1 KB per channel (assignment registry)
- **Network:** 10 KB/s per 1000 channels (heartbeat ingestion)

**At 1000 channels:**
- Coordinator: 0.1 vCPU, 1 MB RAM, 10 KB/s network
- Total listener pods: 10 (Twitch) + 10 (Kick) + 1 (TikTok) = 21 pods
- Redis memory: 1 MB (assignments) + 500 KB (load tracking) + 100 KB (heartbeats) = 1.6 MB

**Bottleneck:** Platform connection limits (Twitch 100 channels, Kick 100 channels), NOT coordinator or Redis.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Global Lock During Rebalancing

**What people do:** Coordinator acquires global lock, computes assignments, releases lock. All listeners block until lock released.

**Why it's wrong:** Listeners cannot process messages during rebalancing. Global lock causes 60s downtime for all channels (not just migrating channels).

**Do this instead:** Use cooperative rebalancing (Pattern 3). Only migrating channels pause, others continue operating. Publish migration events to Redis Stream, listeners consume asynchronously.

### Anti-Pattern 2: Aggressive Rebalancing on Minor Load Imbalance

**What people do:** Trigger rebalancing when any pod has ±5% more channels than others. Rebalance every 5 minutes to maintain perfect balance.

**Why it's wrong:** Rebalancing has cost (PART/JOIN commands, network overhead, brief message loss). Small imbalances (<10%) don't justify rebalancing overhead. Thrashing occurs (constant migrations).

**Do this instead:** Rebalance only when:
- New pod added/removed (topology change)
- Pod failure detected (heartbeat timeout)
- Extreme imbalance (any pod has >150% of average load)

**Threshold:** If average load is 100 channels/pod, trigger rebalancing only if any pod has >150 or <50 channels.

### Anti-Pattern 3: Synchronous Assignment Updates

**What people do:** When new channel added, Coordinator synchronously updates all listeners via HTTP POST `/channels/add`. Listeners immediately JOIN channel.

**Why it's wrong:** Coordinator becomes single point of failure. If coordinator is slow (high load), all channel additions blocked. HTTP requests fail if listener pod restarting.

**Do this instead:** Use eventual consistency (Pattern 2). Coordinator writes to Redis Assignment Registry. Listeners poll every 30s. Eventual consistency is acceptable (30s delay for new channels is fine).

### Anti-Pattern 4: Stateful Assignment History

**What people do:** Store full migration history in Redis (`shard:migrations:history`) to debug rebalancing issues. Never trim stream (MAXLEN ∞).

**Why it's wrong:** Migration stream grows unbounded (1 GB after 1M migrations). Redis memory exhausted. XREADGROUP slows down (scans entire stream).

**Do this instead:** Trim migration stream to last 10,000 entries (MAXLEN ~ 10000). Store critical debugging info in Prometheus metrics (rebalancing count, duration, failed migrations). Use Loki for logs (query migration events from logs, not Redis).

### Anti-Pattern 5: Per-Channel Heartbeats

**What people do:** Each listener publishes heartbeat for every channel (`shard:heartbeat:twitch:xqc`, `shard:heartbeat:twitch:shroud`, etc.). Coordinator checks all channel heartbeats.

**Why it's wrong:** Redis overwhelmed (1000 channels × 10s interval = 100 heartbeats/sec). Coordinator must scan 1000 keys every 30s (O(N) scan). Network overhead (100 KB/s for 1000 channels).

**Do this instead:** Per-pod heartbeats (Pattern 4). Each pod publishes ONE heartbeat with aggregated load (`shard:metrics:twitch:pod_1`). Coordinator scans only P keys (P = pod count, typically 10-50). Network overhead: 1 KB/s for 50 pods.

---

## Sources

**Consistent Hashing:**
- [Consistent Hashing Explained (Ably)](https://ably.com/blog/implementing-efficient-consistent-hashing)
- [Ultimate Guide to Consistent Hashing (Toptal)](https://www.toptal.com/big-data/consistent-hashing)
- [How to Build Consistent Hashing Implementation](https://oneuptime.com/blog/post/2026-01-30-consistent-hashing-implementation/view)
- [Consistent Hashing - GeeksforGeeks](https://www.geeksforgeeks.org/system-design/consistent-hashing/)

**Redis Sharding and State Management:**
- [Redis Ring for Consistent Hashing](https://redis.uptrace.dev/guide/ring.html)
- [Redis Sharding Best Practices 2026](https://www.dragonflydb.io/guides/redis-sharding-how-it-works-pros-cons-best-practices)
- [Hash Slot vs Consistent Hashing in Redis](https://severalnines.com/blog/hash-slot-vs-consistent-hashing-redis/)
- [Managing Redis Clusters on Kubernetes (CNCF)](https://www.cncf.io/blog/2024/12/17/managing-large-scale-redis-clusters-on-kubernetes-with-an-operator-kuaishous-approach/)

**Rebalancing and Migration:**
- [Kafka Partition Rebalancing](https://oneuptime.com/blog/post/2026-02-02-kafka-partition-rebalancing/view)
- [Dynamic Work Rebalancing (Google Cloud Dataflow)](https://docs.cloud.google.com/dataflow/docs/dynamic-work-rebalancing)
- [StatefulSet Management on Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Migrate Stateful Workloads with Zero Downtime (Cast AI)](https://cast.ai/blog/how-to-migrate-stateful-workloads-on-kubernetes-with-zero-downtime/)

**WebSocket Scaling and Sharding:**
- [WebSocket Sharding Patterns](https://tsh.io/blog/how-to-scale-websocket)
- [WebSocket Scaling for Virtual Events (Ably)](https://ably.com/topic/scaling-websockets-virtual-events)
- [10 WebSocket Scaling Patterns](https://medium.com/@sparknp1/10-websocket-scaling-patterns-for-real-time-dashboards-1e9dc4681741)

**Platform-Specific Constraints:**
- [Twitch IRC Join Limits](https://discuss.dev.twitch.com/t/giving-broadcasters-control-concurrent-join-limits-for-irc-and-eventsub/54997)
- [Twitch Chat & Chatbots Documentation](https://dev.twitch.tv/docs/chat/)

---

*Architecture research for: All-Chat Distributed Listener Load Balancing*
*Researched: 2026-02-19*
*Confidence: HIGH (verified with official docs, multiple credible sources, industry patterns)*
