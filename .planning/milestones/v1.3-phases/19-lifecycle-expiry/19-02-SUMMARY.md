---
phase: 19-lifecycle-expiry
plan: "02"
subsystem: api
tags: [twitch, eventsub, redis, pubsub, lifecycle, share-expiry]

# Dependency graph
requires:
  - phase: 19-01
    provides: ExpireAcceptedShare + ExpiryJob foundation for share expiry

provides:
  - SubscribeToStreamOffline and SubscribeToStreamOnline in twitch-eventsub-listener
  - stream.offline webhook handler publishing StreamEndEvent to Redis lifecycle:stream_end
  - LifecycleSubscriber in share-service consuming lifecycle:stream_end with 60s debounce
  - GetThisStreamShares query for accepted shares with expiry_option='this_stream'

affects:
  - 19-03 (YouTube/TikTok stream end detection will publish to same lifecycle:stream_end channel)

# Tech tracking
tech-stack:
  added: [github.com/redis/go-redis/v9 added to share-service]
  patterns: [lifecycle:stream_end Redis pub/sub channel, 60s debounce before stream expiry, db field on webhook Handler for user_id lookup]

key-files:
  created:
    - services/share-service/jobs/lifecycle_subscriber.go
  modified:
    - services/twitch-eventsub-listener/eventsub/subscription_manager.go
    - services/twitch-eventsub-listener/webhooks/handler.go
    - services/twitch-eventsub-listener/cmd/main.go
    - services/share-service/repository/share_repo.go
    - services/share-service/jobs/lifecycle_subscriber_test.go
    - services/share-service/cmd/main.go
    - services/share-service/go.mod

key-decisions:
  - "Handler.db field added to webhooks Handler struct for twitch_id -> user_id lookup without passing db through routeEvent"
  - "60s debounce in LifecycleSubscriber.debounceExpire prevents phantom expiry on stream restart/category change"
  - "Redis ping failure in share-service is non-fatal: lifecycle events disabled but service continues (nil guard in LifecycleSubscriber.run)"
  - "stream.online case is no-op in webhook handler (debounce already handles race in subscriber)"

patterns-established:
  - "lifecycle:stream_end channel: JSON payload with platform/user_id/broadcaster_id/timestamp"
  - "SubscribeToStreamOffline uses app token (no user OAuth scope) via subscribeWithCondition pattern"
  - "expireThisStreamShares: GetThisStreamShares + ExpireAcceptedShare per share (follows existing ExpireTimedAcceptedShares pattern)"

requirements-completed: [EXPIRY-02, EXPIRY-03]

# Metrics
duration: 8min
completed: 2026-03-11
---

# Phase 19 Plan 02: Twitch Stream Lifecycle Detection and Share Expiry Summary

**Twitch stream.offline EventSub detection with Redis lifecycle relay and LifecycleSubscriber consuming lifecycle:stream_end to expire this_stream shares after 60s debounce**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-11T17:47:00Z
- **Completed:** 2026-03-11T17:55:42Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- SubscribeToStreamOffline and SubscribeToStreamOnline added to twitch-eventsub-listener using app-token subscribeWithCondition pattern
- Webhook handler now relays stream.offline to Redis lifecycle:stream_end after twitch_id -> user_id lookup from DB
- LifecycleSubscriber in share-service subscribes to lifecycle:stream_end with 60s debounce before expiring this_stream shares
- GetThisStreamShares query added to ShareRepository for efficient this_stream share lookup

## Task Commits

Each task was committed atomically:

1. **Task 1: SubscribeToStreamOffline/Online + webhook handler stream.offline dispatch** - `f27767b` (feat)
2. **Task 2: LifecycleSubscriber in share-service** - `c3b92e5` (feat)

## Files Created/Modified
- `services/twitch-eventsub-listener/eventsub/subscription_manager.go` - Added SubscribeToStreamOffline and SubscribeToStreamOnline methods
- `services/twitch-eventsub-listener/webhooks/handler.go` - Added db field, StreamEndEvent type, stream.offline case, handleStreamOffline method
- `services/twitch-eventsub-listener/cmd/main.go` - Pass db to NewHandler; add SubscribeToStreamOffline to subscription callback
- `services/share-service/jobs/lifecycle_subscriber.go` - Full LifecycleSubscriber with Redis pub/sub, 60s debounce, expireThisStreamShares
- `services/share-service/jobs/lifecycle_subscriber_test.go` - GREEN test: constructor + StreamEndEvent marshaling
- `services/share-service/repository/share_repo.go` - Added GetThisStreamShares query
- `services/share-service/cmd/main.go` - Added Redis connection; LifecycleSubscriber started alongside ExpiryJob
- `services/share-service/go.mod` - Added github.com/redis/go-redis/v9 dependency

## Decisions Made
- Handler.db field added directly to webhooks.Handler struct so handleStreamOffline can look up user_id without threading db through the event routing chain
- 60s debounce in LifecycleSubscriber.debounceExpire prevents phantom expiry on stream restart or category change — Twitch can emit stream.offline during brief restarts
- Redis ping failure in share-service is non-fatal (non-critical path): LifecycleSubscriber receives nil redis and returns early, service continues normal operation
- stream.online case in routeEvent is a no-op for MVP (debounce in subscriber already handles the Twitch restart race condition)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added go-redis dependency to share-service**
- **Found during:** Task 2 (LifecycleSubscriber implementation)
- **Issue:** share-service go.mod had no Redis dependency; lifecycle_subscriber.go needs redis.Client
- **Fix:** Ran `go get github.com/redis/go-redis/v9@v9.18.0` in share-service
- **Files modified:** services/share-service/go.mod, services/share-service/go.sum
- **Verification:** go build ./... passes
- **Committed in:** c3b92e5 (Task 2 commit)

**2. [Rule 1 - Bug] Corrected lifecycle_subscriber.go Wave 0 stub constructor signature**
- **Found during:** Task 2 (reading existing Wave 0 stub)
- **Issue:** Existing stub had `NewLifecycleSubscriber(redisClient interface{}, repo, logger)` but test calls `NewLifecycleSubscriber(nil, nil, log.Sugar())` expecting `(repo, redis, logger)` order
- **Fix:** Rewrote constructor with correct `(repo *ShareRepository, rdb *redis.Client, logger)` signature
- **Files modified:** services/share-service/jobs/lifecycle_subscriber.go
- **Verification:** TestLifecycleSubscriber_StreamEnd passes
- **Committed in:** c3b92e5 (Task 2 commit)

**3. [Rule 2 - Missing Critical] Added nil-safe Redis connection handling in share-service main.go**
- **Found during:** Task 2 (wiring LifecycleSubscriber in main.go)
- **Issue:** Share-service had no Redis connection; Close() defer on nil client would panic
- **Fix:** Redis ping failure sets redisClientForJobs=nil (non-fatal); LifecycleSubscriber.run() has nil guard
- **Files modified:** services/share-service/cmd/main.go
- **Verification:** go build ./... passes
- **Committed in:** c3b92e5 (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 blocking dependency, 1 bug fix, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
None — all issues resolved via deviation rules.

## Next Phase Readiness
- Twitch stream.offline lifecycle events fully wired end-to-end
- lifecycle:stream_end channel available for Plan 03 (YouTube/TikTok stream end detection)
- LifecycleSubscriber is platform-agnostic: any service can publish StreamEndEvent with a valid user_id

---
*Phase: 19-lifecycle-expiry*
*Completed: 2026-03-11*
