# Pitfalls Research: Distributed Channel Sharding

**Domain:** Distributed Load Balancing for Real-Time Messaging Microservices
**Researched:** 2026-02-19
**Confidence:** MEDIUM-HIGH

## Critical Pitfalls

### Pitfall 1: Split-Brain During Channel Assignment

**What goes wrong:**
Network partitions cause multiple pods to simultaneously believe they own the same channel, resulting in duplicate connections to the platform (e.g., two pods both join the same Twitch IRC channel) and duplicate message delivery to overlays.

**Why it happens:**
Leader election or distributed lock implementations fail to handle network partitions correctly. Most systems use Redis or etcd for coordination, but without proper fencing tokens or quorum-based decisions, two partitions can both believe they're authoritative.

**How to avoid:**
- Use Kubernetes Lease API (coordination.k8s.io/v1) with proper TTLs and renewal intervals
- Implement fencing tokens (monotonic counters) that increase with each channel assignment
- Require majority quorum for channel ownership decisions
- Add platform-level duplicate detection: track message IDs and filter duplicates before publishing to Redis

**Warning signs:**
- Duplicate messages appearing in overlays (same message ID delivered twice)
- Platform rate limit errors (two pods connecting to same channel)
- Metrics showing: sum(channels_per_pod) > total_active_channels
- Multiple pods logging "acquired channel X" simultaneously

**Phase to address:**
Phase 1 (Sharding Infrastructure) - Critical to get right from the start. Cannot be retrofitted easily once in production.

**Severity:** CRITICAL - Causes duplicate charges for API quota (YouTube), rate limit bans (all platforms), and breaks message deduplication guarantees to users.

**Sources:**
- [Split-brain in distributed systems (DZone)](https://dzone.com/articles/split-brain-in-distributed-systems)
- [Understanding Split-Brain Scenarios with etcd](https://sithara-wanigasooriya.medium.com/understanding-split-brain-scenarios-in-distributed-systems-and-how-etcd-mitigates-them-e3007acd506d)
- [Leader Election in Distributed Systems 2026](https://www.devahmedali.click/post/leader-election-in-distributed-systems-complete-guide)

---

### Pitfall 2: Message Loss During Channel Migration

**What goes wrong:**
When a channel moves from Pod A to Pod B during rebalancing (scaling, rolling update, or pod crash), messages sent during the handoff window are lost. Old pod disconnects before new pod establishes connection, creating a 1-10 second gap where messages aren't captured.

**Why it happens:**
Migration implementations focus on state transfer but forget that real-time connections are stateful. IRC connections take 2-5 seconds to establish (TCP handshake → TLS → IRC NICK/PASS → JOIN channel). WebSocket connections (Kick) have similar overhead. During this window, the old pod has already disconnected.

**How to avoid:**
- Implement overlap migration: new pod connects and confirms receiving messages BEFORE old pod disconnects
- Use Redis Streams offset tracking: new pod starts consuming from old pod's last confirmed offset
- Add replay buffer in Message Processor: keep last 60 seconds of messages per channel, replay on reconnect
- For critical channels, implement dual-connection during migration (both pods listen, deduplicate downstream)

**Warning signs:**
- Users report "missed messages" during deployments
- Gaps in Redis Streams consumer group offsets
- Connection timing metrics show disconnect → reconnect gaps > 1 second
- Increased "reconnect_events" metric correlated with "migration_events" metric

**Phase to address:**
Phase 2 (Connection Management) - Build overlap migration before going to production. Message loss is unacceptable for streaming overlays.

**Severity:** CRITICAL - Directly violates product promise. Streamers will notice missed chats during live streams.

**Sources:**
- [Stateful Microservice Migration Challenges in Kubernetes](https://cloudnativenow.com/features/stateful-microservice-migration-the-live-state-challenge-in-kubernetes/)
- [Migrate Stateful Workloads with Zero Downtime](https://cast.ai/blog/how-to-migrate-stateful-workloads-on-kubernetes-with-zero-downtime/)
- [How to Handle Graceful Shutdown for WebSocket Servers 2026](https://oneuptime.com/blog/post/2026-02-02-websocket-graceful-shutdown/view)

---

### Pitfall 3: Thundering Herd on HPA Scale-Up

**What goes wrong:**
When Kubernetes HPA adds 5 new pods simultaneously (traffic spike), all 5 pods attempt channel rebalancing at the exact same moment. This triggers:
- Redis lock contention (all pods competing for same channels)
- Platform rate limits (YouTube quota exhausted by simultaneous API calls)
- Cascading failures (rebalancing failures trigger more HPA scaling)
- Election storms (leadership changes multiple times in 10 seconds)

**Why it happens:**
HPA scale-up is synchronous - all new pods reach "ready" state within 1-2 seconds of each other. Each pod's startup logic immediately attempts to:
1. Register in source-manager
2. Participate in leader election
3. Request channel assignments
4. Establish platform connections

Without jitter or rate limiting, this creates a stampede.

**How to avoid:**
- Add startup jitter: sleep(random(0, pod_ordinal * 2)) before registration
- Implement gradual rebalancing: leader assigns channels in batches (10/second) not all-at-once
- Use token bucket rate limiter for platform API calls: shared across all pods via Redis
- Set HPA scaleUp behavior: stabilizationWindowSeconds: 60, policies: limit 2 pods/min
- Defer non-critical channels during rebalancing: prioritize high-traffic channels first

**Warning signs:**
- Spiking "redis_lock_timeout" errors during scale-up
- Platform API 429 (rate limit) errors correlated with HPA events
- "leader_election_changed" metric shows > 3 elections in 60 seconds
- Multiple pods logging "failed to acquire channel" simultaneously

**Phase to address:**
Phase 3 (Scaling & Resilience) - Test with simulated scale-ups before production. Add chaos engineering tests.

**Severity:** CRITICAL - Can cause service-wide outages. YouTube quota exhaustion affects all users for 24 hours.

**Sources:**
- [Distributed Systems Horror Stories: The Thundering Herd Problem](https://encore.dev/blog/thundering-herd-problem)
- [The Thundering Herd Problem and Solutions](https://singhajit.com/thundering-herd-problem/)
- [Kubernetes Leader Election in Pods 2026](https://oneuptime.com/blog/post/2026-01-19-kubernetes-leader-election-pods/view)

---

### Pitfall 4: Inconsistent Hashing Key Selection Breaks Channel Affinity

**What goes wrong:**
Poor choice of consistent hashing key causes:
- Hot spots (all popular channels hash to same pod)
- Migration churn (different components use different keys, causing repeated migrations)
- State loss (channel metadata stored with one key, but channel assigned with different key)

**Why it happens:**
Team uses overlayID as hash key instead of channelID. When a channel is used by multiple overlays, it gets assigned to different pods depending on which overlay's consumer group processes it first. Channel bounces between pods every few minutes.

Alternatively, team uses composite key like "platform:channelID:overlayID" which creates N connections for a channel used by N overlays, wasting resources and hitting platform connection limits.

**How to avoid:**
- Always hash on channelID (or "platform:channelID" for global uniqueness)
- Document hash key selection in ADR with rationale
- Use virtual nodes (150-200 per physical pod) to distribute load evenly
- Implement "hot key detection" using Count-Min Sketch: track requests/sec per channel, move 1% hottest channels to dedicated pods
- Add hash key validation in tests: ensure same channel always maps to same pod

**Warning signs:**
- Highly variable load across pods (1 pod at 90% CPU, others at 20%)
- "channel_migration_events" metric shows frequent migrations for same channel
- Platform connection limits reached despite total channels < limit
- Duplicate connection errors from platforms

**Phase to address:**
Phase 1 (Sharding Infrastructure) - Hash key selection is foundational. Wrong choice requires full rewrite.

**Severity:** MAJOR - Doesn't cause immediate failure but degrades performance and causes cascading issues under load.

**Sources:**
- [How to Build Consistent Hashing Implementation](https://oneuptime.com/blog/post/2026-01-30-consistent-hashing-implementation/view)
- [The Hot Key Crisis in Consistent Hashing](https://systemdr.substack.com/p/the-hot-key-crisis-in-consistent)
- [Understanding Consistent Hashing](https://www.pubnub.com/blog/consistent-hashing-in-distributed-systems/)

---

### Pitfall 5: Platform-Specific Connection State Not Migrated

**What goes wrong:**
Different platforms have different stateful connection requirements:
- **IRC (Twitch):** Channels must be explicitly JOINed, state includes active channel list
- **HTTP Polling (YouTube):** Polling offset (pageToken) resets on migration, causing duplicate/missed messages
- **WebSocket (Kick):** Subscription IDs must match original connection, can't resume on different pod

When channels migrate, new pod establishes "clean" connection without transferring platform-specific state, causing:
- Twitch: Pod connects but doesn't JOIN channels → no messages
- YouTube: Polling restarts from beginning → duplicates or skipped messages
- Kick: Subscription fails → connection alive but no messages

**Why it happens:**
Generic sharding implementation treats all platforms identically. Source-manager tracks "channelID → podID" mapping but doesn't track platform-specific connection metadata (YouTube pageToken, Kick subscription ID, IRC join state).

**How to avoid:**
- Design platform-specific "connection snapshot" interface:
  ```go
  type ConnectionSnapshot interface {
      Capture() ([]byte, error)  // Serialize current state
      Restore([]byte) error       // Restore state on new pod
  }
  ```
- Store snapshots in Redis with TTL: `connection_state:{platform}:{channelID}`
- Implement per-platform migration handlers:
  - Twitch: Capture active JOIN list, replay on new connection
  - YouTube: Store last pageToken + timestamp, resume from that point
  - Kick: Store subscription IDs, re-subscribe with same IDs
- Add integration tests: migrate channel mid-stream, verify no message loss/duplication

**Warning signs:**
- After migrations, message rate drops to zero (connection alive but not receiving)
- Duplicate messages appear after migration (YouTube polling restarted)
- Platform error logs: "Not subscribed to channel" (Kick), "Not in channel" (Twitch)
- Manual intervention required after migrations (restart pods to fix connections)

**Phase to address:**
Phase 2 (Connection Management) - Must be implemented before production. Each platform needs custom migration logic.

**Severity:** CRITICAL - Without this, migration = downtime. Every rolling update loses messages.

**Sources:**
- [Effective Strategies for Managing WebSockets in Kubernetes](https://wafatech.sa/blog/devops/kubernetes/effective-strategies-for-managing-websockets-in-kubernetes-environments/)
- [How to Handle WebSocket Connection Pooling 2026](https://oneuptime.com/blog/post/2026-01-24-websocket-connection-pooling/view)

---

### Pitfall 6: Message Ordering Violations During Rebalancing

**What goes wrong:**
During channel migration, messages from the same channel arrive out-of-order at Message Processor:
- Old pod publishes message A (timestamp: T1) to Redis Streams
- New pod connects, receives message B (timestamp: T2) and publishes immediately
- Old pod finishes graceful shutdown, message A arrives AFTER message B
- Overlay displays: "message B", then "message A" (backwards)

**Why it happens:**
Redis Streams provides ordering within a single producer, but not across producers. During migration, a channel temporarily has TWO producers (old pod + new pod), breaking ordering guarantees.

**How to avoid:**
- Use sequence numbers per channel: each message includes channel-specific sequence number
- Message Processor validates sequence: buffer out-of-order messages, deliver when gap fills
- During migration, old pod stops publishing BEFORE new pod starts (brief gap acceptable, out-of-order not)
- Alternative (zero-gap): Use "migration coordinator" - old pod routes messages through new pod during overlap
- Add ordering verification in tests: inject messages A, B, C during migration, verify delivery order

**Warning signs:**
- Users report "messages appearing in wrong order" during deployments
- Message Processor logs "sequence gap detected" warnings
- Metrics show increased "buffered_messages" during migration events
- E2E tests fail with "ordering violation" errors

**Phase to address:**
Phase 2 (Connection Management) - Build sequence number system into message format from start.

**Severity:** MAJOR - Breaks user experience. Chat messages out-of-order is confusing but not catastrophic (unlike message loss).

**Sources:**
- [How to Guarantee Message Order in Kafka 2026](https://oneuptime.com/blog/post/2026-01-26-kafka-message-ordering/view)
- [Ordering, Grouping and Consistency in Messaging systems](https://www.architecture-weekly.com/p/ordering-grouping-and-consistency)
- [How to Fix Message Ordering Issues in Event-Driven Systems 2026](https://oneuptime.com/blog/post/2026-01-24-message-ordering-event-driven/view)

---

### Pitfall 7: Redlock Anti-Pattern for Channel Ownership

**What goes wrong:**
Team uses Redlock (distributed Redis locks across multiple Redis instances) for channel assignment, believing it provides stronger guarantees. In practice:
- Adds operational complexity (must run 3+ Redis instances)
- Doesn't solve fundamental problems (clock skew causes false lock acquisitions)
- No fencing tokens (can't prevent stale pod from acting on expired lock)
- Fails during network partitions (minority partition loses locks immediately)

**Why it happens:**
Redlock marketing suggests it's "safer" than single-instance locks. Martin Kleppmann's famous critique explains why it's actually worse than alternatives (etcd with fencing tokens, or single Redis with proper TTLs).

**How to avoid:**
- Use Kubernetes Lease API with fencing tokens (built-in, battle-tested)
- If using Redis: single instance with proper lock patterns:
  - SET with NX (only if not exists) + PX (expiry)
  - Store unique token (UUID) as value
  - Delete only if value matches (Lua script for atomicity)
  - Renew lock periodically (before expiry)
- Add fencing tokens: monotonic counter that increments with each lock acquisition, pods include token in all operations, operations with stale tokens are rejected
- Avoid Redlock entirely - it solves problems you don't have and introduces ones you do

**Warning signs:**
- Multiple Redis instances in architecture for "distributed locking"
- No fencing token implementation
- Locks being acquired by multiple pods simultaneously (clock skew)
- Operations succeeding with expired locks (zombie pods)

**Phase to address:**
Phase 1 (Sharding Infrastructure) - Choose locking mechanism correctly from start. Redlock is a rewrite-level mistake.

**Severity:** MAJOR - Doesn't fail immediately but causes subtle split-brain scenarios under network partitions or clock skew.

**Sources:**
- [How to do distributed locking - Martin Kleppmann](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html)
- [Distributed Locks with Redis (Official Docs)](https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/)
- [10 Hidden Pitfalls of Using Redis Distributed Locks](https://leapcell.medium.com/10-hidden-pitfalls-of-using-redis-distributed-locks-b5234ddd6349)
- [How to Implement Distributed Locks with Redis 2026](https://oneuptime.com/blog/post/2026-01-21-redis-distributed-locks/view)

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip overlap migration, use disconnect→reconnect | Simpler implementation, faster to ship | Message loss during every migration. Users complain. | NEVER - core product requirement is zero message loss |
| Use simple mod-N hashing instead of consistent hashing | Easy to implement, no library needed | Adding/removing pods causes 90%+ channels to migrate, causing thundering herd | Only if: single pod only, no autoscaling planned |
| Single Redis instance (no clustering) | Simple setup, no cluster coordination | Single point of failure, scaling bottleneck at ~50k channels | Acceptable for MVP if: <10k channels, staged rollout plan exists |
| Skip platform-specific state migration | Generic implementation works for all platforms | Silent failures after migration, requires manual intervention | NEVER - each platform is different, generic approach will break |
| Store channel→pod mapping in memory (not Redis) | Lower latency, no network calls | Lost on pod restart, no coordination during split-brain | Only if: single pod only (defeats purpose of sharding) |
| Use leader election without fencing tokens | Simpler implementation, one less counter | Split-brain possible during network partition, duplicate connections | NEVER - fencing tokens are critical for safety |
| Skip observability (metrics, traces, logs) | Faster to ship, less code | Impossible to debug production issues, blind to problems until users complain | Only for: proof-of-concept (not MVP) |

---

## Integration Gotchas

Common mistakes when connecting to external platforms during sharding.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| **Twitch IRC** | Connecting to each channel as separate IRC connection | Single IRC connection per pod, JOIN multiple channels on same connection (Twitch allows ~50 channels/connection) |
| **YouTube API** | Each pod has independent quota tracking | Centralized quota tracking in Redis, pods reserve quota before making API call (reserve→use→confirm or rollback) |
| **YouTube API** | Not handling quota exceeded gracefully | Implement quota circuit breaker: when approaching limit (90%), throttle polling rate globally, prioritize high-traffic channels |
| **Kick WebSocket** | Reconnecting with new subscription IDs after migration | Store subscription IDs in Redis, reuse same IDs on new pod (Kick may maintain state server-side) |
| **Kick WebSocket** | Not handling Pusher channel limits | Track total subscribed channels across all pods, stay under Pusher plan limits (varies by plan) |
| **TikTok (unofficial)** | Assuming API stability | Implement aggressive error handling + graceful degradation, TikTok may block/change endpoints without notice |
| **All Platforms** | Not implementing exponential backoff on reconnect | Use exponential backoff with jitter: 1s, 2s, 4s, 8s, max 30s (prevents thundering herd on platform outages) |
| **All Platforms** | Storing OAuth tokens in pod memory | Store tokens in PostgreSQL, refresh proactively before expiry, use existing token-refresh-service |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| **N+1 Redis calls during rebalancing** | Rebalancing takes 30+ seconds for 1000 channels | Use Redis pipelines: batch channel assignments into single multi-exec transaction | >500 channels per rebalance |
| **Polling every channel at same interval** | YouTube quota exhausted in first hour of day (quota resets midnight PT) | Implement staggered polling: offset each channel by (channelIndex * interval / totalChannels) | >100 YouTube channels |
| **No connection pooling for HTTP clients** | Memory usage spikes during migrations, TCP connections exhausted | Reuse http.Client instances, set MaxIdleConns and MaxConnsPerHost limits | >200 concurrent HTTP connections |
| **Synchronous channel assignment** | HPA scale-up blocked waiting for channels to connect (30+ seconds) | Async channel assignment: pod becomes ready immediately, establishes connections in background | >50 channels assigned at once |
| **Full channel list broadcast on any change** | Redis pub/sub saturated, pods can't keep up with updates | Use incremental updates: only broadcast changed channels, not entire list | >1000 active channels |
| **Leader doing all coordination work** | Leader pod at 100% CPU while followers idle | Distribute work: leader assigns, followers validate and report back | Leader managing >2000 channels |
| **No caching of platform metadata** | Emote API rate limits hit (7TV, BTTV, FFZ) | Cache emote sets in Redis with TTL, share across all pods | >500 unique channels (each queries emotes) |

---

## Observability Gaps

Critical metrics/logs needed for debugging distributed sharding in production.

| What to Measure | Why Critical | How to Detect Problems |
|-----------------|--------------|------------------------|
| **Per-pod channel count** | Detect uneven distribution (hot spots) | Alert: max(channels_per_pod) > 2 * avg(channels_per_pod) |
| **Channel migration events** | Track migration frequency and success rate | Alert: migration_failure_rate > 5% OR migration_events > 10/min (thrashing) |
| **Connection establishment time** | Detect platform slowdowns or network issues | Alert: p95(connection_time) > 10s (should be <3s) |
| **Message gap duration** | Detect message loss during migration | Alert: gap_duration > 5s (overlap migration should eliminate gaps) |
| **Redis lock contention** | Detect thundering herd or lock timeouts | Alert: lock_timeout_rate > 1% OR lock_wait_time p95 > 1s |
| **Platform API errors per pod** | Detect quota issues or rate limits | Alert: api_error_rate > 5% OR error_count_diff between pods > 50 (uneven load) |
| **Leader election changes** | Detect split-brain or network instability | Alert: leader_changes > 2 per hour (should be ~0 in steady state) |
| **Message sequence gaps** | Detect ordering violations | Alert: sequence_gap_events > 0 (should never happen with proper implementation) |
| **Graceful shutdown duration** | Detect pods not draining properly | Alert: shutdown_duration p95 > 20s (K8s terminationGracePeriod is 30s) |
| **Distributed trace for message flow** | Debug end-to-end latency issues | Use OpenTelemetry: platform→listener→Redis→processor→gateway→overlay |

**Missing Observability Consequences:**
- Without per-pod metrics: Can't detect uneven sharding (hot spots)
- Without migration tracking: Blind to message loss during deployments
- Without distributed tracing: Can't debug "why is overlay laggy?" questions
- Without lock contention metrics: Thundering herd appears as "pods crashing randomly"

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| **Split-brain (duplicate connections)** | MEDIUM | 1. Identify duplicate channels (check platform connection logs)<br>2. Force leader re-election: delete Lease resource in K8s<br>3. Implement fencing tokens before restarting pods<br>4. Clear Redis channel assignments: `DEL channel_assignments`<br>5. Rolling restart all pods to reassign cleanly |
| **Message loss during migration** | LOW | 1. Implement replay buffer in Message Processor (60s retention)<br>2. For affected overlays: trigger replay from last known offset<br>3. If no replay buffer: message loss is permanent (notify users) |
| **Thundering herd (quota exhausted)** | HIGH | 1. YouTube quota exhausted = 24h lockout (no recovery)<br>2. Temporarily disable autoscaling to prevent more pods joining<br>3. Implement rate limiter before re-enabling<br>4. Reduce polling frequency for all channels until quota resets<br>5. Request quota increase from Google (takes 2-5 business days) |
| **Hot spots (uneven sharding)** | LOW | 1. Identify hot channels (sort by message rate)<br>2. Manually reassign hot channels to underloaded pods<br>3. Implement virtual nodes (150-200 per pod) for better distribution<br>4. Use "hot key eviction" pattern: detect hot keys, move to dedicated pods |
| **Platform connection state lost** | MEDIUM | 1. Implement connection snapshot/restore (see Pitfall 5)<br>2. Short-term: monitor for "zombie connections" (connected but not receiving)<br>3. Add health check: if message_rate=0 for 60s, force reconnect<br>4. Store pageToken/subscription IDs in Redis before next migration |
| **Out-of-order messages** | LOW | 1. Add sequence numbers to messages<br>2. Message Processor buffers out-of-order (max 30s buffer)<br>3. If gap doesn't fill: log warning + deliver what we have (stale better than stuck) |
| **Redlock causing split-brain** | HIGH (REWRITE) | 1. Immediate: switch to single-Redis locks with proper patterns<br>2. Long-term: migrate to Kubernetes Lease API<br>3. No in-place fix - Redlock is fundamentally broken design |

**Prevention > Recovery:**
Most critical pitfalls (split-brain, message loss, thundering herd) have HIGH recovery costs or no recovery at all. Prevention through proper design is mandatory.

---

## Phase-Specific Warnings

How roadmap phases should address these pitfalls.

| Phase | Topic | Pitfalls to Prevent | How to Verify |
|-------|-------|---------------------|---------------|
| **Phase 1** | Sharding Infrastructure | Split-brain, Redlock anti-pattern, Inconsistent hashing | Chaos test: network partition between pods, verify only one pod owns each channel |
| **Phase 1** | Leader Election | Election storms, missing fencing tokens | Verify: `kubectl delete pod <leader>` → new leader elected in <5s, no duplicate assignments |
| **Phase 2** | Connection Migration | Message loss, platform state not migrated, out-of-order messages | E2E test: migrate channel mid-stream, verify zero message loss and ordering preserved |
| **Phase 2** | Graceful Shutdown | Connections not drained, message gaps | Test: send SIGTERM to pod, verify continues processing for 20s, clean handoff to new pod |
| **Phase 3** | HPA Scaling | Thundering herd, quota exhaustion | Load test: trigger HPA scale-up 2→10 pods, verify staggered startup, no lock contention |
| **Phase 3** | Load Distribution | Hot spots, uneven sharding | Run with 1000 channels for 24h, verify: max/avg channel count per pod <1.5x |
| **Phase 4** | Observability | Blind spots in metrics/logs | Simulate failure scenarios, verify: can identify root cause from metrics alone in <5 min |
| **Phase 4** | Debugging | Missing distributed traces | Generate artificial lag, verify: can trace message from platform→overlay with OpenTelemetry |

---

## "Looks Done But Isn't" Checklist

Features that appear complete but are missing critical pieces.

- [ ] **Channel migration:** Works in happy path but not tested with:
  - Network partition during migration (simulated with tc/iptables)
  - Pod crash mid-migration (kill -9 during handoff)
  - Multiple simultaneous migrations (HPA scaling 2→10 pods)

- [ ] **Leader election:** Works but missing:
  - Fencing tokens (pods must include token in all operations)
  - Monitoring for election frequency (should alert if >2/hour)
  - Automatic leader re-election on leader pod crash (<5s recovery)

- [ ] **Connection management:** Connects successfully but missing:
  - Platform-specific state migration (YouTube pageToken, Kick subscription IDs)
  - Exponential backoff on reconnect failures
  - Circuit breaker for quota/rate limit protection

- [ ] **Message ordering:** Messages delivered but not tested with:
  - Out-of-order scenarios (late-arriving messages during migration)
  - Sequence gap detection and buffering
  - Timeout for gaps that never fill (deliver what we have after 30s)

- [ ] **Observability:** Basic metrics exist but missing:
  - Per-pod channel count distribution (detect hot spots)
  - Migration success/failure rate per platform
  - Distributed tracing for end-to-end message flow
  - Lock contention and wait time metrics

- [ ] **Error handling:** Basic retries but missing:
  - Graceful degradation when quota exhausted (throttle polling, prioritize channels)
  - Platform-specific error handling (YouTube quota vs network error vs auth failure)
  - Dead letter queue for messages that can't be processed

- [ ] **Load testing:** Works with 10 channels but not validated:
  - 1000+ channels across 10 pods
  - HPA scale up/down (2→10→2 pods)
  - 24+ hour soak test (detect memory leaks, connection exhaustion)
  - Platform outage recovery (all channels reconnect simultaneously)

---

## All-Chat Specific Warnings

Pitfalls unique to All-Chat's architecture.

### YouTube Quota is Non-Negotiable

**Problem:** YouTube quota is a HARD LIMIT. Once exhausted, ALL users are affected for 24 hours.

**Why critical for sharding:**
- Thundering herd during HPA scale-up can exhaust quota in minutes
- Duplicate connections (split-brain) double quota consumption
- Naive sharding: each pod independently polls = N*quota usage

**Prevention specific to All-Chat:**
- Centralized quota tracking in Redis (existing quota-manager service)
- Before any YouTube API call: `RESERVE_QUOTA → USE → CONFIRM or ROLLBACK`
- Circuit breaker at 90% quota: throttle polling rate globally
- During rebalancing: pause new YouTube connections until complete

**Phase to address:** Phase 1 (integrate with existing quota-manager), Phase 3 (test under load)

### IRC Connection Limits (Twitch)

**Problem:** Twitch allows ~50 channels per IRC connection, but limits total connections per IP.

**Why critical for sharding:**
- Naive approach: one connection per channel = hit connection limit at 50 channels
- Multiple pods from same IP (Kubernetes node) count toward same limit

**Prevention specific to All-Chat:**
- Existing twitch-listener uses single connection per pod (correct approach)
- Sharding must preserve this: each pod has 1 IRC connection, JOINs multiple channels
- During migration: new pod JOINs channel BEFORE old pod PARTs (brief overlap, not duplicate connection)

**Phase to address:** Phase 2 (connection migration must handle multi-channel IRC connections)

### Redis Streams Consumer Groups

**Problem:** All-Chat uses Redis Streams consumer groups for message delivery. Sharding adds coordination layer.

**Why critical for sharding:**
- Each listener pod publishes to same Redis Stream (`chat:raw`)
- Message Processor consumes via consumer group (exactly-once delivery)
- During migration: old pod stops publishing, new pod starts → no coordination needed at Redis level

**Simplification:** Redis Streams consumer groups already handle multiple publishers correctly. Sharding doesn't complicate this.

**What to watch:** Message ordering during migration (see Pitfall 6)

### Emote Service Load

**Problem:** Emote enrichment (7TV, BTTV, FFZ) queries external APIs per channel.

**Why critical for sharding:**
- More pods = more emote API calls (each pod independently caches)
- Emote APIs have rate limits (lower than platform limits)

**Prevention specific to All-Chat:**
- Centralized emote cache in Redis (shared across all pods)
- TTL: 5 minutes (emotes rarely change)
- Cache key: `emotes:{platform}:{channelID}`
- Only leader pod refreshes cache (followers read-only)

**Phase to address:** Phase 1 (shared cache architecture), Phase 3 (test under load)

---

## Sources

### Core Distributed Systems Concepts
- [A Guide to Large-Scale Distributed Systems (2026)](https://www.systemdesignhandbook.com/blog/large-scale-distributed-systems/)
- [Distributed System Distributed Messaging](https://www.meegle.com/en_us/topics/distributed-system/distributed-system-distributed-messaging)
- [Distributed Messaging System | System Design - GeeksforGeeks](https://www.geeksforgeeks.org/system-design/distributed-messaging-system-system-design/)

### Split-Brain & Leader Election
- [Split-Brain in Distributed Systems (DZone)](https://dzone.com/articles/split-brain-in-distributed-systems)
- [Split brain in distributed systems | Medium](https://medium.com/nerd-for-tech/split-brain-in-distributed-systems-252b0d4d122e)
- [Understanding Split-Brain Scenarios with etcd](https://sithara-wanigasooriya.medium.com/understanding-split-brain-scenarios-in-distributed-systems-and-how-etcd-mitigates-them-e3007acd506d)
- [Leader Election in Distributed Systems: Complete Guide 2026](https://www.devahmedali.click/post/leader-election-in-distributed-systems-complete-guide)
- [How to Implement Leader Election in Kubernetes Pods](https://oneuptime.com/blog/post/2026-01-19-kubernetes-leader-election-pods/view)
- [How to Implement Kubernetes Leader Election](https://oneuptime.com/blog/post/2026-01-30-kubernetes-leader-election/view)

### Consistent Hashing & Sharding
- [How to Build Consistent Hashing Implementation](https://oneuptime.com/blog/post/2026-01-30-consistent-hashing-implementation/view)
- [Consistent Hashing 101: How Modern Systems Handle Growth](https://blog.bytebytego.com/p/consistent-hashing-101-how-modern)
- [The "Hot Key" Crisis in Consistent Hashing](https://systemdr.substack.com/p/the-hot-key-crisis-in-consistent)
- [Understanding Consistent Hashing](https://www.pubnub.com/blog/consistent-hashing-in-distributed-systems/)

### Stateful Migration & Zero Downtime
- [Stateful Microservice Migration in Kubernetes](https://cloudnativenow.com/features/stateful-microservice-migration-the-live-state-challenge-in-kubernetes/)
- [Migrate Stateful Workloads with Zero Downtime](https://cast.ai/blog/how-to-migrate-stateful-workloads-on-kubernetes-with-zero-downtime/)
- [Container Live Migration: Zero Downtime](https://cast.ai/blog/introducing-container-live-migration-zero-downtime-for-stateful-kubernetes-workloads/)
- [How to Migrate Workloads Between Kubernetes Clusters](https://oneuptime.com/blog/post/2026-01-06-kubernetes-migrate-workloads-zero-downtime/view)

### WebSocket & Connection Management
- [How to Handle Graceful Shutdown for WebSocket Servers](https://oneuptime.com/blog/post/2026-02-02-websocket-graceful-shutdown/view)
- [How to Handle WebSocket Connection Pooling](https://oneuptime.com/blog/post/2026-01-24-websocket-connection-pooling/view)
- [Effective Strategies for Managing WebSockets in Kubernetes](https://wafatech.sa/blog/devops/kubernetes/effective-strategies-for-managing-websockets-in-kubernetes-environments/)
- [Kubernetes: zero-downtime rolling updates](https://www.driftrock.com/blog/kubernetes-zero-downtime-rolling-updates)

### Distributed Locking
- [How to do distributed locking — Martin Kleppmann](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html)
- [Distributed Locks with Redis (Official Docs)](https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/)
- [10 Hidden Pitfalls of Using Redis Distributed Locks](https://leapcell.medium.com/10-hidden-pitfalls-of-using-redis-distributed-locks-b5234ddd6349)
- [How to Implement Distributed Locks with Redis (Redlock)](https://oneuptime.com/blog/post/2026-01-21-redis-distributed-locks/view)
- [Distributed Locking: A Practical Guide](https://www.architecture-weekly.com/p/distributed-locking-a-practical-guide)

### Thundering Herd & Scaling
- [Distributed Systems Horror Stories: The Thundering Herd Problem](https://encore.dev/blog/thundering-herd-problem)
- [The Thundering Herd Problem and Its Solutions](https://singhajit.com/thundering-herd-problem/)
- [The Thundering Herd Problem Explained 2025](https://medium.com/@work.dhairya.singla/the-thundering-herd-problem-explained-causes-examples-and-solutions-7166b7e26c0c)
- [Thundering Herds: The Scalability Killer](https://docs.aonnis.com/blog/thundering-herds-the-scalability-killer)

### Message Ordering & Duplicates
- [How to Guarantee Message Order in Kafka 2026](https://oneuptime.com/blog/post/2026-01-26-kafka-message-ordering/view)
- [Ordering, Grouping and Consistency in Messaging systems](https://www.architecture-weekly.com/p/ordering-grouping-and-consistency)
- [How to Fix Message Ordering Issues in Event-Driven Systems 2026](https://oneuptime.com/blog/post/2026-01-24-message-ordering-event-driven/view)
- [Handling Duplicate Messages in Distributed Systems](https://www.geeksforgeeks.org/system-design/handling-duplicate-messages-in-distributed-systems/)
- [Idempotency in Distributed Systems](https://medium.com/javarevisited/idempotency-in-distributed-systems-preventing-duplicate-operations-85ce4468d161)

### Observability
- [Kubernetes Observability and Monitoring Trends in 2026](https://www.usdsi.org/data-science-insights/kubernetes-observability-and-monitoring-trends-in-2026)
- [Building a Production Ready Observability Stack: 2026](https://medium.com/@krishnafattepurkar/building-a-production-ready-observability-stack-the-complete-2026-guide-9ec6e7e06da2)
- [11 Key Observability Best Practices 2026](https://spacelift.io/blog/observability-best-practices)
- [Distributed Systems Observability Explained 2025](https://edgedelta.com/company/knowledge-center/distributed-systems-observability)

### Rate Limiting & Load Balancing
- [How to Implement Load Balancer Rate Limiting](https://oneuptime.com/blog/post/2026-01-27-load-balancer-rate-limiting/view)
- [Rate Limiting Fundamentals](https://blog.bytebytego.com/p/rate-limiting-fundamentals)
- [Scale Your Live Streaming App for 1 Million Users: 2026](https://scalevista.com/blog/scaling-live-streaming-app/)

---

*Pitfalls research for: All-Chat Distributed Channel Sharding*
*Researched: 2026-02-19*
*Focus: Subsequent milestone adding load balancing to existing real-time messaging system*
