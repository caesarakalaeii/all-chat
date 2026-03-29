---
phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
plan: "06"
subsystem: listener-publishers
tags: [ring-buffer, redis-streams, resilience, liveness, xadd-retry]
dependency_graph:
  requires: [08-04]
  provides: [D-07-complete, LI-01-eliminated, LI-02-eliminated, LI-03-eliminated]
  affects:
    - services/twitch-listener/publisher
    - services/kick-listener/publisher
    - services/youtube-listener-innertube/publisher
    - services/discord-listener/publisher
    - services/twitch-eventsub-listener/publisher
tech_stack:
  added: []
  patterns:
    - RingBufferPublisher wrapping XADD in all 5 Go listener services
    - newStreamPublisherWithRingBuffer internal constructor for test injection
    - prometheus.Registerer injection to prevent duplicate metric panics in tests
key_files:
  created:
    - services/twitch-listener/publisher/ring_buffer_integration_test.go
    - services/kick-listener/publisher/ring_buffer_integration_test.go
    - services/youtube-listener-innertube/publisher/ring_buffer_integration_test.go
    - services/discord-listener/publisher/ring_buffer_integration_test.go
    - services/twitch-eventsub-listener/publisher/stream_publisher_test.go
  modified:
    - services/twitch-listener/publisher/stream_publisher.go
    - services/twitch-listener/cmd/main.go
    - services/kick-listener/publisher/redis.go
    - services/kick-listener/cmd/main.go
    - services/youtube-listener-innertube/publisher/redis_publisher.go
    - services/youtube-listener-innertube/cmd/main.go
    - services/discord-listener/publisher/stream_publisher.go
    - services/discord-listener/cmd/main.go
    - services/twitch-eventsub-listener/publisher/stream_publisher.go
    - services/twitch-eventsub-listener/cmd/main.go
decisions:
  - RingBufferPublisher uses data-only "data" field XADD — message-processor reads only the "data" field; per-field twitch-listener fields were unnecessary overhead
  - newStreamPublisherWithRingBuffer internal constructor for test injection — tests inject custom publishFn + isolated prometheus.Registry to prevent duplicate registration panics
  - NewStreamPublisherWithRingBuffer exported for discord-listener tests — external test package (publisher_test) requires exported constructor
  - youtube-innertube deletion buffer preserved — Publish routes deletions through 500ms delay buffer before ring buffer; ring buffer wraps publishToRedis (the raw XADD path)
  - kick-listener stores redis client in struct post-construction — IsHealthy needs the real client; ring buffer captures it in closure for publishFn
  - discord-listener NewStreamPublisherFromCmdable preserved — existing unit test uses mock Cmdable; ring buffer wraps the cmdable XAdd call
  - PublishBatch uses individual ring buffer calls not pipeline — replaces LI-02 problematic pipeline approach; each message independently retried on failure
metrics:
  duration: 442s
  completed_date: "2026-03-29"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 10
  files_created: 5
---

# Phase 08 Plan 06: Wire RingBufferPublisher into All 5 Go Listeners Summary

All 5 Go listener services now wrap their Redis XADD calls with `RingBufferPublisher` from `shared/listener`, eliminating LI-01, LI-02, and LI-03 failure modes (silent XADD drops). Failed publishes are buffered in a 1000-message ring buffer and retried every 500ms.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire RingBufferPublisher into twitch-listener and kick-listener | 21e279a | stream_publisher.go, redis.go, 2 cmd/main.go, 2 test files |
| 2 | Wire RingBufferPublisher into youtube-innertube, discord, and eventsub | b58d0e5 | redis_publisher.go, stream_publisher.go x2, 3 cmd/main.go, 3 test files |

## What Was Built

**Pattern applied to all 5 listeners:**

1. `StreamPublisher` struct gains `ringBuffer *sharedlistener.RingBufferPublisher` field
2. Constructor calls `sharedlistener.NewRingBufferPublisherWithRegisterer(1000, xaddFn, logger, serviceName, reg)`
3. `Publish` serialises message to JSON then calls `p.ringBuffer.Publish(ctx, jsonBytes)` — XADD failures buffer silently
4. `Stop()` method calls `p.ringBuffer.Stop()` for clean shutdown
5. `cmd/main.go` calls `streamPublisher.Stop()` in the graceful shutdown sequence

**Service names registered in Prometheus metrics:**
- twitch-listener: `service="twitch-listener"`
- kick-listener: `service="kick-listener"`
- youtube-listener-innertube: `service="youtube-listener-innertube"`
- discord-listener: `service="discord-listener"`
- twitch-eventsub-listener: `service="twitch-eventsub-listener"`

**youtube-innertube special handling:** The deletion buffer (500ms delay) is preserved. `Publish` routes deletions through `deletionBuffer.Add`, which calls `publishViaRingBuffer` after the delay. Non-deletion messages go directly to `publishViaRingBuffer` → ring buffer.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] twitch-listener multi-field XADD simplified to data-only**
- **Found during:** Task 1
- **Issue:** Original publisher published 8 individual fields (message_id, platform, channel_id, user_id, username, text, timestamp, data). Message-processor reads only the "data" field. The ring buffer wraps a single-payload function, so multi-field XADD required awkward bypass logic.
- **Fix:** Simplified to data-only XADD (matching kick, discord, eventsub pattern). All message content is in the JSON-encoded "data" field as message-processor expects.
- **Files modified:** `services/twitch-listener/publisher/stream_publisher.go`
- **Commit:** 21e279a

**2. [Rule 2 - Missing functionality] PublishBatch replaced pipeline with individual ring buffer calls**
- **Found during:** Task 1
- **Issue:** Original PublishBatch used redis Pipeline which was the LI-02 failure mode identified in the plan. The plan explicitly required replacing the pipeline approach.
- **Fix:** PublishBatch now iterates messages and calls `p.ringBuffer.Publish` individually. Each message is independently retried on failure. PublishBatch always returns nil (partial failures buffered).
- **Files modified:** `services/twitch-listener/publisher/stream_publisher.go`, `services/youtube-listener-innertube/publisher/redis_publisher.go`
- **Commit:** 21e279a, b58d0e5

## Known Stubs

None — all ring buffer wiring is fully functional.

## Self-Check: PASSED

Files created:
- services/twitch-listener/publisher/ring_buffer_integration_test.go ✓
- services/kick-listener/publisher/ring_buffer_integration_test.go ✓
- services/youtube-listener-innertube/publisher/ring_buffer_integration_test.go ✓
- services/discord-listener/publisher/ring_buffer_integration_test.go ✓
- services/twitch-eventsub-listener/publisher/stream_publisher_test.go ✓

Commits: 21e279a (Task 1), b58d0e5 (Task 2) ✓
All 5 services build successfully ✓
All publisher tests pass ✓
