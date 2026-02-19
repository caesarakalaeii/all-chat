---
phase: 06-connection-management-migration-protocol
plan: 01
subsystem: coordination
tags: [coordinator-client, redis-pubsub, migration, http-api, go-redis, jwt-auth]

# Dependency graph
requires:
  - phase: 05-sharding-infrastructure-coordinator-service
    provides: GET /assignments and POST /heartbeat endpoints with JWT authentication
provides:
  - CoordinatorClient library for assignment queries with retry-with-backoff
  - MigrationSubscriber library for Redis Pub/Sub migration events
  - Migration event models with JSON marshaling
affects:
  - 06-02-twitch-listener-integration
  - 06-03-kick-listener-integration
  - 06-04-tiktok-listener-integration
  - 06-05-coordinator-migration-publisher

# Tech tracking
tech-stack:
  added: []
  patterns:
    - exponential-backoff-retry-pattern
    - redis-pubsub-subscription-pattern
    - panic-recovery-in-handlers

key-files:
  created:
    - shared/coordination/client.go
    - shared/coordination/migration_subscriber.go
    - shared/coordination/models.go
  modified: []

key-decisions:
  - "Blocks indefinitely on QueryAssignments until coordinator responds (per CONTEXT.md user decision)"
  - "Exponential backoff with 30s max for coordinator availability during pod startup"
  - "Hybrid Redis Pub/Sub for migration events (5-20ms latency vs 15s polling)"
  - "Handler panic recovery with error logging for migration event processing"

patterns-established:
  - "Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s (max) for HTTP retries on network errors"
  - "Redis Pub/Sub subscription pattern: Subscribe → Receive confirmation → Goroutine for message consumption"
  - "Panic recovery in event handlers: defer/recover to prevent handler panics from crashing subscriber"

requirements-completed: [MIGRATE-01, MIGRATE-05]

# Metrics
duration: 2min
completed: 2026-02-19
---

# Phase 06 Plan 01: Coordinator Client Library Summary

**Reusable Go library for listener pods to query channel assignments on startup and subscribe to real-time migration events via Redis Pub/Sub**

## Performance

- **Duration:** 2 min
- **Started:** 2026-02-19T21:59:02Z
- **Completed:** 2026-02-19T22:01:01Z
- **Tasks:** 3 (2 atomic commits - Task 1 and Task 3 combined due to dependency)
- **Files created:** 3

## Accomplishments

- CoordinatorClient library implements QueryAssignments with exponential backoff (1s to 30s max), blocks indefinitely until coordinator responds
- CoordinatorClient implements PublishHeartbeat for 10-second heartbeat publishing to coordinator
- MigrationSubscriber library subscribes to Redis Pub/Sub `migration:events` channel and invokes handler callbacks
- Migration event models with all 7 required fields (migration_id, channel_id, platform, from_pod, to_pod, timestamp, reason)
- Panic recovery in migration event handlers prevents single handler failure from crashing subscriber
- Ready for integration by Twitch, Kick, and TikTok listeners in Wave 2

## Task Commits

Each task was committed atomically:

1. **Task 1: Create CoordinatorClient** - `02b2434` (feat) - *Combined with Task 3 due to dependency*
2. **Task 2: Create MigrationSubscriber** - `5ebf197` (feat)
3. **Task 3: Create migration event models** - `02b2434` (feat) - *Combined with Task 1 (client.go depends on models.go)*

## Files Created/Modified

**Created:**

1. **shared/coordination/client.go** (205 lines)
   - `CoordinatorClient` struct with baseURL, serviceJWT, httpClient, logger
   - `QueryAssignments(ctx, podID)` - Queries coordinator for channel assignments with exponential backoff retry
   - `PublishHeartbeat(ctx, podID)` - Publishes heartbeat to coordinator endpoint
   - `Assignment` struct matching source-manager models
   - `HeartbeatRequest` struct for heartbeat payload

2. **shared/coordination/migration_subscriber.go** (112 lines)
   - `MigrationSubscriber` struct with redisClient, logger, handler callback
   - `Subscribe(ctx)` - Subscribes to `migration:events` Redis Pub/Sub channel
   - `consumeMessages(ctx, pubsub)` - Goroutine for consuming migration events
   - Panic recovery wrapper around handler invocation
   - Unmarshal error handling (log warning, skip event, continue consuming)

3. **shared/coordination/models.go** (32 lines)
   - `MigrationEvent` struct - 7 required fields for migration events
   - `MigrationConfirmation` struct - Used by listeners to confirm successful connection
   - `AssignmentResponse` struct - Matches source-manager response format

**Modified:** None

## Decisions Made

### 1. Block Indefinitely on QueryAssignments Until Coordinator Responds

**Context:** Listener pods need channel assignments before starting to listen

**Decision:** QueryAssignments blocks indefinitely with exponential backoff retry until coordinator responds successfully

**Rationale (from CONTEXT.md user decision):**
- "Block indefinitely until coordinator responds (pod not ready until assignments received)"
- Coordinator availability is critical for pod startup - better to wait than start without assignments
- Exponential backoff (1s → 30s max) prevents overwhelming coordinator during startup
- Kubernetes readiness probe will mark pod not ready until assignments received

**Alternatives considered:**
- Fail fast after N retries - rejected, pod would crash-loop without assignments
- Start with empty assignments - rejected, would miss channels during scale-up

### 2. Hybrid Redis Pub/Sub for Migration Events

**Context:** Listener pods need real-time notification of migrations (per CONTEXT.md)

**Decision:** Subscribe to Redis Pub/Sub `migration:events` channel, coordinator publishes events

**Rationale (from CONTEXT.md user decision):**
- "Hybrid Redis Pub/Sub approach - Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)"
- "All listener pods subscribe to `migration:events` channel"
- "Faster than polling (15s avg), safer than direct HTTP push"

**Why this approach:**
- 5-20ms latency for migration notifications (vs 15s polling)
- No HTTP endpoint needed on listener pods (simpler security model)
- Coordinator doesn't need to track listener pod IPs/endpoints
- Redis Pub/Sub handles fan-out to all listener pods automatically

### 3. Handler Panic Recovery in Migration Event Processing

**Context:** Migration event handler is user-provided callback

**Decision:** Wrap handler invocation in defer/recover to catch panics, log error, continue consuming

**Rationale:**
- Handler panics should not crash entire subscriber goroutine
- Migration events are critical for overlap protocol - must continue processing even if one handler fails
- Error logging provides visibility into handler failures for debugging

**Implementation:**
```go
defer func() {
    if r := recover(); r != nil {
        s.logger.Error("Migration event handler panicked",
            zap.String("migration_id", event.MigrationID),
            zap.Any("panic", r),
        )
    }
}()
s.handler(&event)
```

## Deviations from Plan

None - plan executed exactly as written.

**Clarifications:**
- Task 1 (client.go) and Task 3 (models.go) committed together because client.go depends on AssignmentResponse from models.go
- Both files required for compilation, so combining commits avoided broken build state
- This is a natural dependency ordering, not a deviation from plan intent

## Issues Encountered

None - all files compiled successfully on first attempt.

## Integration Points

**Upstream (Phase 05-04):**
- ✅ Uses GET /assignments endpoint from source-manager (Plan 05-04)
- ✅ Uses POST /heartbeat endpoint from source-manager (Plan 05-04)
- ✅ Uses SERVICE_JWT_AUTH middleware pattern established in Phase 05

**Downstream (Plans 06-02, 06-03, 06-04):**
- ✅ CoordinatorClient ready for Twitch listener startup (Plan 06-02)
- ✅ CoordinatorClient ready for Kick listener startup (Plan 06-03)
- ✅ CoordinatorClient ready for TikTok listener startup (Plan 06-04)
- ✅ MigrationSubscriber ready for all listeners to handle migration events
- ✅ Models ready for migration event publishing (Plan 06-05)

## API Patterns Established

### QueryAssignments Retry Pattern

**Exponential backoff for coordinator availability:**

```go
backoff := time.Second
maxBackoff := 30 * time.Second

for {
    resp, err := c.httpClient.Do(req)
    if err != nil {
        // Network error - retry with backoff
        time.Sleep(backoff)
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
        continue
    }

    // Handle response...
}
```

**When to retry:**
- Network errors (connection refused, timeout) → retry with backoff
- 5xx server errors → retry with backoff
- 4xx client errors → fail immediately (configuration issue)

**Why exponential backoff:**
- Coordinator might be starting up (common during scale-up)
- Prevents overwhelming coordinator with requests
- 30s max backoff balances fast recovery vs reasonable retry frequency

### Redis Pub/Sub Subscription Pattern

**Standard pattern for subscribing to Redis Pub/Sub:**

```go
pubsub := s.redisClient.Subscribe(ctx, channel)

// Wait for subscription confirmation
_, err := pubsub.Receive(ctx)
if err != nil {
    return fmt.Errorf("failed to subscribe: %w", err)
}

// Start consuming in goroutine
go s.consumeMessages(ctx, pubsub)
```

**Error handling:**
- Subscription failure → return error (caller can retry)
- Unmarshal errors → log warning, skip event, continue
- Handler panics → recover, log error, continue

**Context cancellation:**
- Subscriber goroutine watches `ctx.Done()` channel
- Closes pubsub on context cancellation
- Graceful shutdown when listener pod terminates

## Integration Examples for Wave 2

### Example 1: Query Assignments on Startup (Twitch Listener)

```go
// In twitch-listener cmd/main.go
coordinatorClient := coordination.NewCoordinatorClient(
    "http://source-manager:8088",
    os.Getenv("SERVICE_JWT_SECRET"),
    logger,
)

// Block until assignments received
assignments, err := coordinatorClient.QueryAssignments(ctx, podID)
if err != nil {
    logger.Fatal("Failed to query assignments", zap.Error(err))
}

// Connect to assigned channels
for _, assignment := range assignments {
    channel := getChannelFromDB(assignment.SourceID)
    ircClient.Join(channel.TwitchUsername)
}
```

### Example 2: Subscribe to Migration Events (Kick Listener)

```go
// In kick-listener cmd/main.go
migrationHandler := func(event *coordination.MigrationEvent) {
    logger.Info("Received migration event",
        zap.String("channel_id", event.ChannelID),
        zap.String("from_pod", event.FromPod),
        zap.String("to_pod", event.ToPod),
    )

    // If we're the new pod, connect to channel
    if event.ToPod == podID {
        channel := getChannelFromDB(event.ChannelID)
        connectToKickChannel(channel)
    }

    // If we're the old pod, disconnect after overlap period
    if event.FromPod == podID {
        time.Sleep(30 * time.Second) // Overlap protocol
        disconnectFromKickChannel(event.ChannelID)
    }
}

subscriber := coordination.NewMigrationSubscriber(redisClient, migrationHandler, logger)
if err := subscriber.Subscribe(ctx); err != nil {
    logger.Fatal("Failed to subscribe to migration events", zap.Error(err))
}
```

### Example 3: Publish Heartbeat in Background Goroutine

```go
// In all listeners
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := coordinatorClient.PublishHeartbeat(ctx, podID); err != nil {
                logger.Error("Failed to publish heartbeat", zap.Error(err))
            }
        }
    }
}()
```

## Next Phase Readiness

**Ready for Plan 06-02 (Twitch Listener Integration):**
- ✅ CoordinatorClient can query assignments on startup
- ✅ MigrationSubscriber can handle migration events
- ✅ Models support full migration event context

**Ready for Plan 06-03 (Kick Listener Integration):**
- ✅ Same libraries, no platform-specific changes needed

**Ready for Plan 06-04 (TikTok Listener Integration):**
- ✅ Same libraries, no platform-specific changes needed

**Ready for Plan 06-05 (Coordinator Migration Publisher):**
- ✅ MigrationEvent model ready for coordinator to publish
- ✅ MigrationConfirmation model ready for listeners to confirm

**No blockers identified.**

## Self-Check: PASSED

**Files created:**
```
✅ shared/coordination/client.go (205 lines)
✅ shared/coordination/migration_subscriber.go (112 lines)
✅ shared/coordination/models.go (32 lines)
```

**Commits exist:**
```
✅ 02b2434: feat(06-01): add CoordinatorClient for assignment queries and heartbeat publishing
✅ 5ebf197: feat(06-01): add MigrationSubscriber for Redis Pub/Sub migration events
```

**Build verification:**
```bash
cd shared/coordination && go build .
# ✅ Exit code 0 - all files compile successfully
```

**Requirements met:**
```
✅ MIGRATE-01: MigrationSubscriber subscribes to migration:events channel
✅ MIGRATE-05: MigrationEvent contains all 7 required fields
✅ CoordinatorClient implements QueryAssignments with retry-with-backoff
✅ CoordinatorClient implements PublishHeartbeat
✅ Both methods add JWT Authorization header for SERVICE_JWT_AUTH
```

---
*Phase: 06-connection-management-migration-protocol*
*Completed: 2026-02-19*
