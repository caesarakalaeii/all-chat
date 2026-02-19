# Phase 6: Connection Management & Migration Protocol - Research

**Researched:** 2026-02-19
**Domain:** Distributed connection management, zero-loss migration, Redis Pub/Sub coordination
**Confidence:** HIGH

## Summary

Phase 6 integrates Twitch, Kick, and TikTok listener services with the Phase 5 coordinator, implementing zero-loss channel migration through overlap protocol. Listeners query coordinator on startup for assigned channels (not all channels), connect only to assigned subset, and handle graceful migrations when pods scale or rebalance. Migration uses Redis Pub/Sub for 5-20ms notification latency, overlap handoff (new pod connects before old disconnects), and downstream deduplication in Message Processor to prevent duplicate messages during transition period.

The existing codebase provides foundation: Phase 5 coordinator with Kubernetes Lease leader election is deployed, Redis infrastructure operational, listeners already have channel managers with PostgreSQL LISTEN/NOTIFY patterns, and Message Processor implements deduplication via SHA256 fingerprints with 30s TTL.

**Primary recommendation:** Use Redis Pub/Sub `migration:events` channel for migration notifications (all pods subscribe), listener startup blocks until coordinator responds with assignments (pod not ready until connected), overlap protocol with 30s first-message timeout (proof of connection), downstream deduplication via existing Message Processor fingerprint mechanism, platform-specific implementations without forced abstraction layer.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGRATE-01 | System implements overlap migration pattern (new pod connects before old disconnects) | Redis Pub/Sub notification patterns, overlap handoff protocol design |
| MIGRATE-02 | New pod subscribes to channel and waits for first message before signaling ready | Platform connection verification patterns, first-message timeout strategy |
| MIGRATE-03 | Old pod receives migration signal and gracefully disconnects after 45 seconds | Graceful disconnect patterns for IRC PART, Pusher unsubscribe |
| MIGRATE-04 | System guarantees zero message loss during migration (no dropped messages) | Overlap protocol timing, downstream deduplication strategies |
| MIGRATE-05 | System publishes migration events to Redis Streams for observability | Redis Streams append patterns, migration event schema design |
| MIGRATE-06 | System uses sequence numbers per channel to detect message gaps during migration | Platform message ID patterns, gap detection strategies |
| TWITCH-01 | Twitch listener queries shard coordinator for assigned channels on startup | Coordinator assignment API integration, startup blocking patterns |
| TWITCH-02 | Twitch listener connects to IRC only for assigned channels (not all channels) | IRC JOIN filtering, channel manager assignment filtering |
| TWITCH-03 | Twitch listener supports multiple IRC connections when assigned >100 channels | go-twitch-irc multiple client patterns, connection pooling strategies |
| TWITCH-04 | Twitch listener stores IRC JOIN list state in ConnectionSnapshot for migration | Migration state requirements (minimal: channel list only) |
| TWITCH-05 | Twitch listener gracefully parts IRC channels during migration (sends PART command) | IRC PART command patterns, graceful disconnect procedures |
| TWITCH-06 | System allows HPA to scale Twitch listener from 1 to 5 replicas successfully | Kubernetes readiness probe patterns, coordinator-aware health checks |
| TWITCH-07 | All Twitch listener pods report ready status (fixes current 1/5 ready issue) | Readiness probe implementation, assignment-dependent ready status |
| KICK-01 | Kick listener queries shard coordinator for assigned channels on startup | Coordinator assignment API integration, startup blocking patterns |
| KICK-02 | Kick listener connects to Pusher WebSocket only for assigned channels | Pusher subscribe filtering, channel manager assignment filtering |
| KICK-03 | Kick listener stores Pusher subscription IDs in ConnectionSnapshot for migration | Migration state requirements (minimal: chatroom IDs only) |
| KICK-04 | Kick listener gracefully unsubscribes from channels during migration | Pusher unsubscribe patterns, graceful disconnect procedures |
| KICK-05 | System allows HPA to scale Kick listener from 1 to 5 replicas successfully | Kubernetes readiness probe patterns, coordinator-aware health checks |
| TIKTOK-01 | TikTok listener queries shard coordinator for assigned channels on startup | Coordinator assignment API integration, startup blocking patterns |
| TIKTOK-02 | TikTok listener connects via tiktok-live-connector only for assigned channels | Connection filtering for unofficial library |
| TIKTOK-03 | TikTok listener stores connection state in ConnectionSnapshot for migration | Migration state requirements (minimal: channel list only) |
| TIKTOK-04 | TikTok listener handles connection state migration for unofficial library | Unofficial library disconnect/reconnect patterns |
| TIKTOK-05 | System allows HPA to scale TikTok listener from 1 to 3 replicas successfully | Kubernetes readiness probe patterns, coordinator-aware health checks |

</phase_requirements>

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Startup Integration:**
- Coordinator availability: Block indefinitely until coordinator responds (pod not ready until assignments received)
- Connection strategy: Connect immediately to all assigned channels (no batching delays)
- Ready status: Pod reports ready AFTER successfully connecting to all assigned channels

**Migration Notification & Timing:**
- Notification mechanism: Hybrid Redis Pub/Sub approach
  - Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)
  - All listener pods subscribe to `migration:events` channel
  - Faster than polling (15s avg), safer than direct HTTP push
- Migration event structure: Full context for debugging and metrics
  - Required fields: `channel_id`, `platform`, `from_pod`, `to_pod`, `migration_id`, `timestamp`, `reason`
- Overlap protocol: New pod waits for first message OR 30s timeout (whichever comes first)
  - New pod connects and subscribes to channel
  - Waits up to 30 seconds for first message (proof of successful connection)
  - Publishes confirmation to Redis when connected
- Disconnect trigger: Old pod disconnects immediately after seeing new pod's confirmation
  - No additional grace period - immediate handoff after confirmation
- Duplicate handling: Downstream deduplication
  - Both pods publish to Redis Streams during overlap period
  - Message Processor deduplicates based on platform message IDs
  - Keeps listener implementation simple

**Platform-Specific Implementation:**
- Architecture: Platform-specific implementations (no forced abstraction layer)
  - Twitch listener implements coordinator integration for IRC
  - Kick listener implements coordinator integration for Pusher WebSocket
  - TikTok listener implements coordinator integration for tiktok-live-connector
  - Respects platform quirks, accepts some code duplication
- Connection state migration: Minimal state - just channel assignment list
  - "State" = which channels to connect to (from coordinator)
  - New pod creates fresh connections (new IRC socket, new Pusher subscription)
  - No transfer of connection handles or platform-specific state

**Twitch IRC Specifics:**
- Multiple connections: Claude decides based on channel count
  - <100 channels: Single IRC connection with JOIN commands
  - ≥100 channels: Multiple IRC connections (Requirement TWITCH-03)
  - Balances simplicity vs Twitch's per-connection limits

**Failure Handling & Recovery:**
- New pod connection failure: Quick retry + coordinator fallback
  - New pod attempts connection 3 times with 1-second delays (3s total)
  - If all fail: publish "migration failed" event with error details
  - Old pod continues running (zero message loss)
  - Coordinator immediately reassigns to different pod
- Old pod doesn't disconnect: Coordinator intervention
  - If old pod doesn't send "disconnected" confirmation within 60s timeout
  - Coordinator marks pod as dead, removes from assignments
  - New pod becomes source of truth
- Coordinator crashes mid-migration: Idempotent recovery
  - New coordinator leader reads migration events from Redis Streams
  - Checks current pod states via heartbeat data
  - Resumes or completes any in-progress migrations

### Claude's Discretion

- Observability event design (lifecycle events, retries, failures)
- Exact batching strategy if platform rate limits require it
- Kubernetes pod restart mechanics (delete pod vs update deployment)
- Migration retry backoff tuning if 1-second intervals prove suboptimal

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/redis/go-redis/v9 | v9.17.3+ | Redis client (already in project) | Pub/Sub subscription, message publishing, existing infrastructure |
| github.com/gempir/go-twitch-irc/v4 | v4.0.0+ | Twitch IRC client (already in project) | Standard Twitch IRC library, supports JOIN/PART, multiple connections |
| github.com/gorilla/websocket | v1.5.3+ | WebSocket client (already in project for Kick) | Industry standard WebSocket library, used by Kick Pusher client |
| k8s.io/client-go | v0.30.2 | Kubernetes client (already in project) | Pod lifecycle queries, readiness probe configuration |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/prometheus/client_golang | v1.23.2+ | Metrics (already in project) | Track migration events, connection counts, failure rates |
| go.uber.org/zap | v1.27.1+ | Logging (already in project) | Structured logging for migration operations |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Redis Pub/Sub | HTTP webhooks | HTTP requires listener pod discovery and retry logic; Pub/Sub broadcast to all subscribers simpler |
| Redis Pub/Sub | Polling coordinator API | 15s polling latency vs 5-20ms Pub/Sub; higher coordinator load |
| Downstream deduplication | Listener-side coordination lock | Adds complexity to listeners; duplicate prevention more fragile |
| Overlap protocol | Drain-then-connect pattern | Guaranteed message loss during handoff window; unacceptable for "zero message loss" requirement |

**Installation:**

All dependencies already present in project (Phase 1-5 implementations).

## Architecture Patterns

### Recommended Integration Structure

```
Listener Service Startup Flow:
├── main.go initialization
│   ├── Connect to Redis (existing)
│   ├── Connect to PostgreSQL (existing)
│   ├── Initialize coordinator client (NEW)
│   ├── Subscribe to migration:events channel (NEW)
│   └── Query assignments (NEW, blocks until response)
├── Connection initialization
│   ├── Filter assigned channels from all sources (NEW)
│   ├── Connect to platform (IRC/Pusher/TikTok)
│   └── Wait for successful connection
├── Health check ready state
│   ├── Check platform connection (existing)
│   └── Check assignment count > 0 (NEW)
└── Start message processing loop
```

### Pattern 1: Coordinator Assignment Query on Startup

**What:** Listener pods query coordinator for assigned channels on startup, block until response, connect only to assigned subset.

**When to use:** Pod initialization (before connecting to platform), readiness probe health check.

**Example:**
```go
// Source: Existing Phase 5 coordinator GET /assignments endpoint
// services/source-manager/handlers/assignments.go

type CoordinatorClient struct {
    baseURL string
    client  *http.Client
}

// QueryAssignments blocks until coordinator responds with assignments
func (c *CoordinatorClient) QueryAssignments(ctx context.Context, podID string) ([]Assignment, error) {
    url := fmt.Sprintf("%s/assignments?pod_id=%s", c.baseURL, podID)

    // Retry with exponential backoff (coordinator might not be ready)
    backoff := time.Second
    for {
        resp, err := c.client.Get(url)
        if err == nil && resp.StatusCode == 200 {
            var result AssignmentResponse
            json.NewDecoder(resp.Body).Decode(&result)
            return result.Assignments, nil
        }

        // Log and retry
        log.Warn("Coordinator not ready, retrying", zap.Duration("backoff", backoff))
        time.Sleep(backoff)
        backoff = min(backoff*2, 30*time.Second)
    }
}

// In listener main.go:
assignments, err := coordClient.QueryAssignments(ctx, podName)
// Pod stays NotReady until this returns

// Filter channels
assignedChannelIDs := make(map[string]bool)
for _, a := range assignments {
    assignedChannelIDs[a.ChannelID] = true
}
```

**User constraint enforcement:**
- Blocks indefinitely until coordinator responds (pod not ready until assignments received)
- No timeout on initial query (coordinator availability critical)

### Pattern 2: Redis Pub/Sub Migration Notification

**What:** All listener pods subscribe to `migration:events` Redis Pub/Sub channel for real-time migration notifications.

**When to use:** Pod startup (subscribe), ongoing operation (handle notifications).

**Example:**
```go
// Source: Existing Redis Pub/Sub patterns in message-processor/publisher/pubsub_publisher.go
import "github.com/redis/go-redis/v9"

type MigrationSubscriber struct {
    client *redis.Client
    logger *zap.Logger
}

func (s *MigrationSubscriber) Subscribe(ctx context.Context, handler func(*MigrationEvent)) {
    pubsub := s.client.Subscribe(ctx, "migration:events")
    defer pubsub.Close()

    // Receive messages
    for {
        msg, err := pubsub.ReceiveMessage(ctx)
        if err != nil {
            s.logger.Error("Pub/Sub receive error", zap.Error(err))
            continue
        }

        var event MigrationEvent
        if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
            s.logger.Error("Invalid migration event", zap.Error(err))
            continue
        }

        handler(&event)
    }
}

type MigrationEvent struct {
    ChannelID   string    `json:"channel_id"`
    Platform    string    `json:"platform"`
    FromPod     string    `json:"from_pod"`
    ToPod       string    `json:"to_pod"`
    MigrationID string    `json:"migration_id"`
    Timestamp   time.Time `json:"timestamp"`
    Reason      string    `json:"reason"`
}
```

**Benefits over alternatives:**
- 5-20ms latency vs 15s polling (user constraint: "very fast handovers")
- All pods receive notification simultaneously (broadcast pattern)
- Redis already deployed and operational (no new infrastructure)

### Pattern 3: Overlap Protocol with First-Message Confirmation

**What:** New pod connects, waits for first message (or 30s timeout), publishes confirmation, old pod disconnects immediately.

**When to use:** Migration event received by new pod (connection phase) and old pod (disconnect phase).

**Example (Twitch IRC):**
```go
// New pod receives migration event for channel "xqc"
func (m *Manager) handleMigrationEvent(event *MigrationEvent) {
    if event.ToPod != m.podID {
        return // Not for us
    }

    // Step 1: Connect and join channel
    m.ircClient.Join(event.ChannelID)

    // Step 2: Wait for first message (proof of connection)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    select {
    case <-m.firstMessageChan(event.ChannelID):
        // Success! Publish confirmation
        m.publishMigrationConfirmation(event.MigrationID, "connected")
        m.logger.Info("Migration successful", zap.String("channel", event.ChannelID))

    case <-ctx.Done():
        // Timeout - connection failed
        m.publishMigrationConfirmation(event.MigrationID, "failed")
        m.logger.Error("Migration timeout", zap.String("channel", event.ChannelID))
    }
}

// Old pod receives migration confirmation
func (m *Manager) handleMigrationConfirmation(event *MigrationConfirmation) {
    if event.Status == "connected" {
        // New pod ready, disconnect immediately
        m.ircClient.Depart(event.ChannelID)
        m.logger.Info("Migration handoff complete", zap.String("channel", event.ChannelID))
    }
    // If "failed", keep running (zero message loss)
}
```

**Timing characteristics:**
- Typical handoff: 100-500ms (IRC JOIN latency + first message)
- Worst case: 30s timeout + 1s retries = 33s
- User constraint: "very fast handovers" met by typical case, worst case acceptable fallback

### Pattern 4: Downstream Deduplication in Message Processor

**What:** Message Processor deduplicates based on SHA256 fingerprint (platform|channel|user|text|timestamp), 30s TTL window.

**When to use:** Migration overlap period (both old and new pods publishing), network retries.

**Example:**
```go
// Source: Existing implementation in services/message-processor/dedup/dedup.go

// Message Processor receives messages from Redis Streams (published by listeners)
func (p *Processor) processMessage(rawMsg *RawMessage) {
    // Check if duplicate using existing deduplicator
    isDup, err := p.deduplicator.IsDuplicate(ctx,
        rawMsg.Platform,
        rawMsg.ChannelID,
        rawMsg.UserID,
        rawMsg.Text,
        rawMsg.Timestamp)

    if isDup {
        p.logger.Debug("Duplicate message detected during migration",
            zap.String("channel", rawMsg.ChannelID),
            zap.String("message_id", rawMsg.MessageID))
        return // Skip processing
    }

    // Process normally
    p.normalize(rawMsg)
    p.enrich(rawMsg)
    p.publish(rawMsg)
}
```

**Why downstream deduplication:**
- Keeps listener implementation simple (no coordination locks)
- Reuses existing Message Processor deduplication (already implemented for network retries)
- Migration overlap window typically <1s, worst case 30s (within existing 30s TTL)
- User constraint: "Both pods publish to Redis Streams during overlap period, Message Processor deduplicates based on platform message IDs"

### Pattern 5: Platform-Specific Connection Management

**What:** Each listener implements coordinator integration respecting platform quirks (IRC JOIN limits, Pusher subscription patterns, unofficial TikTok library).

**When to use:** Implementing coordinator integration for each listener service.

**Twitch IRC specifics:**
```go
// Twitch IRC rate limit: 20 channels per 10 seconds (authenticated)
// Source: Existing implementation in services/twitch-listener/channels/manager.go

type Manager struct {
    rateLimiter *rate.Limiter // Existing: 20 channels per 10s
    ircConn     *irc.ConnectionManager
}

// For <100 channels: Single IRC connection
func (m *Manager) joinAssignedChannels(assignments []Assignment) {
    for _, a := range assignments {
        m.rateLimiter.Wait(context.Background()) // Respect rate limit
        m.ircConn.Join(a.ChannelID)
    }
}

// For ≥100 channels: Multiple IRC connections (user constraint: Claude decides threshold)
func (m *Manager) joinAssignedChannelsMultiConn(assignments []Assignment) {
    // Create additional IRC connections (go-twitch-irc supports multiple clients)
    clientCount := (len(assignments) / 90) + 1 // 90 channels per connection (safe margin)

    for i := 0; i < clientCount; i++ {
        client := twitch.NewClient(m.username, m.oauth)
        client.OnPrivateMessage(m.handleMessage)
        go client.Connect()

        // Distribute channels across connections
        start := i * 90
        end := min((i+1)*90, len(assignments))
        for _, a := range assignments[start:end] {
            client.Join(a.ChannelID)
        }
    }
}
```

**Kick Pusher specifics:**
```go
// Kick uses Pusher WebSocket with chatroom subscriptions
// Source: Existing implementation in services/kick-listener/websocket/client.go

// Subscribe to assigned chatrooms only
func (m *Manager) subscribeAssignedChannels(assignments []Assignment) {
    for _, a := range assignments {
        // Convert channel slug to chatroom ID (from database)
        chatroomID := m.getChatroomID(a.ChannelID)

        // Existing Pusher subscribe method
        m.wsClient.Subscribe(chatroomID)
    }
}

// Graceful unsubscribe during migration
func (m *Manager) unsubscribeChannel(channelID string) {
    chatroomID := m.getChatroomID(channelID)
    m.wsClient.Unsubscribe(chatroomID)
}
```

**TikTok specifics:**
Note: TikTok listener not yet implemented in codebase. Pattern based on CONTEXT.md requirements.

```go
// TikTok uses unofficial tiktok-live-connector library
// Connection management follows similar pattern to Kick

type Manager struct {
    connections map[string]*TikTokLiveClient // Per-channel connections
}

func (m *Manager) connectAssignedChannels(assignments []Assignment) {
    for _, a := range assignments {
        client := NewTikTokLiveClient(a.ChannelID)
        client.OnMessage(m.handleMessage)
        go client.Connect()
        m.connections[a.ChannelID] = client
    }
}

func (m *Manager) disconnectChannel(channelID string) {
    if client, ok := m.connections[channelID]; ok {
        client.Disconnect()
        delete(m.connections, channelID)
    }
}
```

**User constraint enforcement:**
- Platform-specific implementations (no forced abstraction layer)
- Respects platform quirks (IRC rate limits, Pusher subscription IDs, unofficial TikTok library)
- Accepts code duplication across listeners

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis Pub/Sub pattern | Custom polling loop | github.com/redis/go-redis/v9 Subscribe() | Built-in reconnection, message buffering, error handling |
| Message deduplication | Custom cache with manual expiry | Existing Message Processor dedup/dedup.go | Already implemented with SHA256 fingerprinting, tested |
| IRC connection pooling | Custom connection manager | github.com/gempir/go-twitch-irc/v4 multiple clients | Library handles reconnection, rate limiting, JOIN/PART |
| Migration coordination lock | Custom distributed lock | Overlap protocol + downstream dedup | Simpler, no lock coordination overhead |

**Key insight:** Existing infrastructure (Phase 5 coordinator, Message Processor deduplication, Redis Pub/Sub, platform clients) provides foundation. Phase 6 integrates listeners without reinventing wheels.

## Common Pitfalls

### Pitfall 1: Blocking Readiness Probe on Coordinator

**What goes wrong:** Pod readiness probe fails if coordinator temporarily unavailable during deployment, causing cascading failures.

**Why it happens:** Readiness probe timeout (default 1s) shorter than coordinator startup time.

**How to avoid:**
- Separate startup assignment query (blocks indefinitely) from readiness probe (quick check)
- Readiness probe checks: (1) platform connected, (2) assignments count > 0
- Readiness probe does NOT query coordinator (uses cached assignments)

**Warning signs:** Pods stuck in NotReady state, logs show "coordinator timeout" errors.

**Example:**
```go
// BAD: Readiness probe queries coordinator (slow, unreliable)
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
    assignments, err := h.coordClient.QueryAssignments(ctx, h.podID)
    if err != nil {
        c.JSON(503, gin.H{"status": "not ready"})
        return
    }
    c.JSON(200, gin.H{"status": "ready"})
}

// GOOD: Readiness probe checks cached assignments
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
    if !h.platformConnected() {
        c.JSON(503, gin.H{"status": "platform not connected"})
        return
    }
    if h.assignmentCount() == 0 {
        c.JSON(503, gin.H{"status": "no assignments"})
        return
    }
    c.JSON(200, gin.H{"status": "ready"})
}
```

### Pitfall 2: Race Condition in Overlap Protocol Confirmation

**What goes wrong:** Old pod disconnects before new pod connected, causing message loss during migration.

**Why it happens:** Migration confirmation published before first message received (premature handoff).

**How to avoid:**
- New pod MUST wait for first message (proof of connection) before publishing confirmation
- 30s timeout prevents indefinite blocking
- Old pod disconnects ONLY after seeing confirmation (not on migration event)

**Warning signs:** Message gaps detected during migrations, migration success rate <100%.

**Example:**
```go
// BAD: Publish confirmation immediately after JOIN
m.ircClient.Join(channelID)
m.publishConfirmation("connected") // Premature!

// GOOD: Wait for first message proof
m.ircClient.Join(channelID)
select {
case <-m.firstMessageChan(channelID):
    m.publishConfirmation("connected") // Safe
case <-time.After(30 * time.Second):
    m.publishConfirmation("failed") // Retry elsewhere
}
```

### Pitfall 3: Deduplication Window Too Short

**What goes wrong:** Duplicate messages published during migration overlap if deduplication TTL expires before Message Processor processes both messages.

**Why it happens:** Message Processor consumer lag + short dedup TTL = missed duplicates.

**How to avoid:**
- Use existing 30s dedup TTL (services/message-processor/dedup/dedup.go)
- Migration overlap window typically <1s, worst case 30s (within TTL)
- Monitor Message Processor consumer lag (should be <5s)

**Warning signs:** Duplicate messages appearing in overlays during migrations, user reports.

**Example:**
```go
// Existing Message Processor deduplication (don't change)
const dedupTTL = 30 * time.Second // CRITICAL: Must exceed migration overlap window

// Migration overlap window
const migrationTimeout = 30 * time.Second // Matches dedup TTL
```

### Pitfall 4: Multiple IRC Connections Without Load Balancing

**What goes wrong:** Creating 10+ IRC connections for large channel sets exceeds Twitch rate limits, causes connection failures.

**Why it happens:** User constraint allows multiple connections for >100 channels, but no guidance on distribution strategy.

**How to avoid:**
- Distribute channels evenly across connections (90 channels per connection, safe margin below 100)
- Respect global rate limit: 20 JOINs per 10 seconds across ALL connections
- Use existing rate limiter in channel manager

**Warning signs:** IRC connection errors, JOIN failures, "rate limit exceeded" logs.

**Example:**
```go
// GOOD: Distribute channels across connections
clientCount := (len(assignments) / 90) + 1
channelsPerClient := len(assignments) / clientCount

for i := 0; i < clientCount; i++ {
    client := twitch.NewClient(username, oauth)
    start := i * channelsPerClient
    end := min((i+1)*channelsPerClient, len(assignments))

    for _, a := range assignments[start:end] {
        rateLimiter.Wait(ctx) // Global rate limiter (20 per 10s)
        client.Join(a.ChannelID)
    }
}
```

## Code Examples

Verified patterns from official sources and existing codebase:

### Redis Pub/Sub Subscription Pattern
```go
// Source: https://redis.uptrace.dev/guide/go-redis-pubsub.html
// Existing: services/message-processor/publisher/pubsub_publisher.go

import "github.com/redis/go-redis/v9"

pubsub := redisClient.Subscribe(ctx, "migration:events")
defer pubsub.Close()

// Wait for confirmation that subscription is created
_, err := pubsub.Receive(ctx)
if err != nil {
    panic(err)
}

// Consume messages
ch := pubsub.Channel()
for msg := range ch {
    var event MigrationEvent
    json.Unmarshal([]byte(msg.Payload), &event)
    handleMigrationEvent(&event)
}
```

### Twitch IRC Multiple Connections
```go
// Source: https://github.com/gempir/go-twitch-irc (v4)
// Existing: services/twitch-listener/irc/connection.go

import "github.com/gempir/go-twitch-irc/v4"

// Create multiple clients for load distribution
clients := make([]*twitch.Client, clientCount)
for i := 0; i < clientCount; i++ {
    client := twitch.NewClient(username, oauth)

    // Set up handlers
    client.OnPrivateMessage(handleMessage)
    client.OnConnect(handleConnect)

    // Connect in goroutine
    go client.Connect()

    clients[i] = client
}

// Distribute channels across clients
for i, channelID := range assignments {
    clientIdx := i % len(clients)
    clients[clientIdx].Join(channelID)
}
```

### Kick Pusher Subscribe/Unsubscribe
```go
// Source: Existing services/kick-listener/websocket/client.go

// Subscribe to chatroom
func (c *Client) Subscribe(chatroomID int) error {
    channel := fmt.Sprintf("chatrooms.%d.v2", chatroomID)

    event := &pusherEvent{
        Event: pusherSubscribe,
        Data: map[string]interface{}{
            "channel": channel,
        },
    }

    return c.sendMessage(event)
}

// Unsubscribe during migration
func (c *Client) Unsubscribe(chatroomID int) error {
    channel := fmt.Sprintf("chatrooms.%d.v2", chatroomID)

    event := &pusherEvent{
        Event: pusherUnsubscribe,
        Data: map[string]interface{}{
            "channel": channel,
        },
    }

    return c.sendMessage(event)
}
```

### Message Deduplication Check
```go
// Source: Existing services/message-processor/dedup/dedup.go

// Check if message is duplicate during migration overlap
isDup, err := deduplicator.IsDuplicate(ctx,
    rawMsg.Platform,      // "twitch", "kick", etc.
    rawMsg.ChannelID,     // "xqc", "trainwreckstv"
    rawMsg.UserID,        // "12345"
    rawMsg.Text,          // "PogChamp"
    rawMsg.Timestamp)     // time.Now()

if isDup {
    // Skip processing - already handled by other pod during migration
    return
}

// Process normally
processMessage(rawMsg)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HTTP polling for assignments | Coordinator query on startup (blocking) | Phase 6 (2026-02) | Pod initialization validates assignments before ready |
| All pods connect to all channels | Coordinator assigns subset to each pod | Phase 6 (2026-02) | Enables horizontal scaling without duplicate connections |
| Drain-then-connect migration | Overlap protocol with confirmation | Phase 6 (2026-02) | Zero message loss during migrations |
| Listener-side deduplication | Downstream deduplication in Message Processor | Phase 1 (2024) | Simpler listener implementation, centralized dedup logic |

**Deprecated/outdated:**
- **Global channel sync:** All pods previously synced all channels from database, now query coordinator for assigned subset only
- **Immediate disconnection on scale-down:** Previous approach lost messages during pod termination, now uses overlap protocol

## Open Questions

1. **TikTok Listener Implementation**
   - What we know: Requirements defined (TIKTOK-01 through TIKTOK-05), unofficial library mentioned
   - What's unclear: Exact tiktok-live-connector API patterns, connection lifecycle
   - Recommendation: Research tiktok-live-connector library during TikTok listener implementation task, follow same patterns as Kick listener (connection manager, assignment filtering)

2. **Migration Retry Strategy**
   - What we know: 3 retries with 1-second delays (3s total), coordinator reassigns on failure
   - What's unclear: Should coordinator try multiple pods in parallel or sequentially?
   - Recommendation: Sequential first (simpler), monitor failure rates in production, add parallel fallback if needed

3. **Multiple IRC Connection Threshold**
   - What we know: User constraint says "<100 channels: single connection, ≥100: multiple connections"
   - What's unclear: Optimal threshold (90? 95? 100?) balancing simplicity vs safety margin
   - Recommendation: Use 90 channels per connection (10% safety margin), configurable via env var for tuning

## Sources

### Primary (HIGH confidence)

- **Existing codebase**: services/source-manager/coordination/coordinator.go - Phase 5 coordinator implementation
- **Existing codebase**: services/source-manager/handlers/assignments.go - GET /assignments endpoint
- **Existing codebase**: services/message-processor/dedup/dedup.go - Deduplication implementation
- **Existing codebase**: services/twitch-listener/irc/connection.go - IRC connection patterns
- **Existing codebase**: services/kick-listener/websocket/client.go - Pusher WebSocket patterns
- **GitHub**: [github.com/gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) - Official Twitch IRC client
- **Official docs**: [Redis Pub/Sub guide](https://redis.uptrace.dev/guide/go-redis-pubsub.html) - go-redis Pub/Sub patterns

### Secondary (MEDIUM confidence)

- [GeeksforGeeks: Zero Downtime Deployments in Distributed Systems](https://www.geeksforgeeks.org/system-design/zero-downtime-deployments-in-distributed-systems/) - Zero downtime patterns
- [Architecture Weekly: Deduplication in Distributed Systems](https://www.architecture-weekly.com/p/deduplication-in-distributed-systems) - Deduplication strategies
- [Redis Docs: XADD command](https://redis.io/docs/latest/commands/xadd/) - Redis Streams message IDs
- [OneUpTime: Exactly-Once Processing with Pub/Sub](https://oneuptime.com/blog/post/2026-01-27-pubsub-exactly-once/view) - Pub/Sub reliability patterns

### Tertiary (LOW confidence)

- None - all findings verified with codebase or official documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All dependencies already in project (Phase 1-5)
- Architecture: HIGH - Patterns verified in existing codebase (coordinator, dedup, Redis Pub/Sub)
- Pitfalls: MEDIUM-HIGH - Based on distributed systems best practices, verified with codebase patterns

**Research date:** 2026-02-19
**Valid until:** 60 days (stack mature, no breaking changes expected)
