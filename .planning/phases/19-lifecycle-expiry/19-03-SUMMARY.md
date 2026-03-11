---
phase: 19-lifecycle-expiry
plan: "03"
subsystem: lifecycle-expiry
tags: [youtube, tiktok, share-service, frontend, lifecycle, redis, expiry]
dependency_graph:
  requires: [19-02]
  provides: [EXPIRY-05, EXPIRY-06]
  affects: [share-service, youtube-listener-innertube, tiktok-listener, frontend]
tech_stack:
  added: []
  patterns:
    - LifecyclePublisher interface for testable Redis publish
    - 4-arg constructor pattern for adding DB dependency without breaking existing tests
    - RedisPublisher interface in TypeScript for cross-client compatibility
    - useEffect switching default state when platform constraint applies
key_files:
  created: []
  modified:
    - services/youtube-listener-innertube/poller/lifecycle.go
    - services/youtube-listener-innertube/poller/lifecycle_test.go
    - services/youtube-listener-innertube/poller/poller.go
    - services/share-service/jobs/lifecycle_subscriber.go
    - services/share-service/jobs/lifecycle_subscriber_test.go
    - services/share-service/cmd/main.go
    - services/tiktok-listener/src/livestream/poller.ts
    - services/tiktok-listener/src/index.ts
    - frontend/src/app/dashboard/shares/components/AcceptModal.tsx
    - frontend/src/app/dashboard/shares/components/AcceptModal.test.tsx
decisions:
  - key: youtube-null-client-guard
    summary: "Added nil guard to Repository.DeleteChannelVideoMapping to prevent panic in unit tests with nil redis client"
  - key: tiktok-stream-end-publish-location
    summary: "TikTok stream_end publish placed in index.ts disconnected handler (live→offline transition) using livePoller.publishStreamEnd, not in the poller polling cycle (which only handles offline→live)"
  - key: tiktok-redis-publisher-interface
    summary: "RedisPublisher interface defined in poller.ts accepts both ioredis and node-redis client shapes via duck typing"
metrics:
  duration: 6 min
  completed_date: "2026-03-11"
  tasks_completed: 2
  files_changed: 10
---

# Phase 19 Plan 03: Multi-Platform Stream End Detection Summary

YouTube and TikTok now publish lifecycle:stream_end to Redis when streams go offline. Share-service LifecycleSubscriber updated to 4-arg form with google_id lookup for YouTube. AcceptModal gracefully disables "This stream" for Kick users with test coverage.

## What Was Built

### Task 1: YouTube HandleStreamOffline + 4-arg LifecycleSubscriber

**YouTube lifecycle publish (EXPIRY-05):**
- Added `LifecyclePublisher` interface and `StreamEndPayload` type to `poller/lifecycle.go`
- `HandleStreamOffline` now accepts an optional `LifecyclePublisher` parameter — nil-safe
- When non-nil, publishes `{platform:"youtube", user_id:"", broadcaster_id:channelID, timestamp:...}` to `lifecycle:stream_end`
- `Poller` struct gains `publisher LifecyclePublisher` field wired through `PollerOptions.Publisher`
- Both call sites in `poller.go` updated to pass `p.publisher`
- Wave 0 RED stub replaced with real `TestHandleStreamOffline_PublishesLifecycleEvent` using `mockLifecyclePublisher`
- Added nil guard to `Repository.DeleteChannelVideoMapping` to prevent panic with nil client in tests

**LifecycleSubscriber 4-arg update (share-service):**
- `LifecycleSubscriber` struct gains `db *pgxpool.Pool` field
- `NewLifecycleSubscriber` updated to 4-arg form: `(repo, redis, db *pgxpool.Pool, logger)`
- `debounceExpire` resolves `google_id → user_id` via `SELECT id FROM users WHERE google_id = $1` when `UserID == ""` and `platform == "youtube"`
- `lifecycle_subscriber_test.go` updated to 4-arg call (`nil, nil, nil, log.Sugar()`)
- `cmd/main.go` passes `dbPool` as third argument to `NewLifecycleSubscriber`

### Task 2: TikTok stream_end publish + AcceptModal Kick disable

**TikTok lifecycle publish (EXPIRY-06 partial):**
- `LiveStreamPoller` gains optional `RedisPublisher` interface parameter in constructor
- `publishStreamEnd(username)` public method: fire-and-forget publish to `lifecycle:stream_end`
- `redisPublish` private helper: nil-safe, logs debug if no client configured
- `setRedisClient(client)` method to wire redis after construction
- `index.ts`: calls `this.livePoller.setRedisClient(this.redis)` after redis connects
- `index.ts`: calls `this.livePoller.publishStreamEnd(username)` in `disconnected` event handler (live→offline transition point)
- `user_id=""` is acceptable MVP behavior — users table has no tiktok_id column

**AcceptModal Kick graceful disable:**
- `senderPlatform?: string` prop added (defaults to `undefined` — no behavioral change for existing callers)
- `isKickUser = senderPlatform === 'kick'` computed boolean
- `useEffect` switches expiryOption to `'unlimited'` when `isKickUser && expiryOption === 'this_stream'`
- "This stream" radio: `disabled={isKickUser}`, label gets `opacity-50 cursor-not-allowed`
- Explanatory note: "(not available for Kick — stream detection not yet supported)"
- 5 new tests in `AcceptModal.test.tsx` covering: kick disables radio, note display, non-kick does not disable, undefined does not disable, kick selects unlimited by default
- All 12 AcceptModal tests pass

## Verification Results

```
youtube-listener-innertube: BUILD_OK
youtube-listener-innertube: go test ./poller/ -run TestHandleStreamOffline PASS
share-service: BUILD_OK
share-service: go test ./... -short PASS (all packages)
frontend: tsc --noEmit → 0 errors
frontend: vitest AcceptModal.test.tsx → 12/12 tests pass
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added nil guard to Repository.DeleteChannelVideoMapping**
- **Found during:** Task 1 — `TestHandleStreamOffline_PublishesLifecycleEvent` panicked with nil pointer dereference
- **Issue:** `Repository.DeleteChannelVideoMapping` directly accessed `r.client.Del()` without nil check; test creates `&Repository{client: nil}` which causes panic
- **Fix:** Added early return `if r.client == nil { return fmt.Errorf("redis client is nil") }` at top of method
- **Files modified:** `services/youtube-listener-innertube/poller/lifecycle.go`
- **Commit:** 436d399

**2. [Architectural note] TikTok publish location**
- The plan suggested adding the publish call to the LiveStreamPoller's polling cycle (checkTarget). However, the LiveStreamPoller polls *offline* users waiting to go live — it does not track live→offline transitions.
- The correct publish point is the `disconnected` event handler in `index.ts` where the actual live→offline transition occurs.
- Resolution: Added `publishStreamEnd` to `LiveStreamPoller` as specified, called from `index.ts` disconnected handler via `this.livePoller.publishStreamEnd(username)`. This follows the plan's interface spec while placing the call at the architecturally correct point.

## Checkpoint Status

Task 3 (human-verify) reached — automation complete, awaiting human verification.

## Self-Check: PASSED

Files created/modified:
- [x] `services/youtube-listener-innertube/poller/lifecycle.go` — LifecyclePublisher interface added
- [x] `services/youtube-listener-innertube/poller/lifecycle_test.go` — Wave 0 stub replaced with real test
- [x] `services/youtube-listener-innertube/poller/poller.go` — publisher field wired
- [x] `services/share-service/jobs/lifecycle_subscriber.go` — 4-arg constructor with db field
- [x] `services/share-service/jobs/lifecycle_subscriber_test.go` — updated to 4-arg
- [x] `services/share-service/cmd/main.go` — dbPool passed to NewLifecycleSubscriber
- [x] `services/tiktok-listener/src/livestream/poller.ts` — publishStreamEnd added
- [x] `services/tiktok-listener/src/index.ts` — disconnected handler calls publishStreamEnd
- [x] `frontend/src/app/dashboard/shares/components/AcceptModal.tsx` — senderPlatform prop + Kick disable
- [x] `frontend/src/app/dashboard/shares/components/AcceptModal.test.tsx` — 5 new Kick tests

Commits:
- [x] 436d399 — Task 1: YouTube HandleStreamOffline + 4-arg LifecycleSubscriber
- [x] c7c72b9 — Task 2: TikTok stream_end + AcceptModal Kick disable
