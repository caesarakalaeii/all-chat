---
phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
plan: 01
subsystem: infra
tags: [redis-streams, dlq, retry, message-processor, prometheus, go]

# Dependency graph
requires: []
provides:
  - Unique per-pod consumer names via os.Hostname() (MP-01 fixed)
  - PEL drain on startup via XAutoClaim (MP-02 fixed)
  - Retry + DLQ routing + correct ACK ordering (MP-03 fixed)
  - Pub/Sub publish retry with per-overlay isolation (MP-04/MP-05 fixed)
  - Context cancellation exits consume loop cleanly (MP-08 fixed)
  - chat:dlq stream with 7-day auto-trim (DQ-01/DQ-02)
  - POST /admin/dlq/replay endpoint (DQ-03)
  - New Prometheus metrics: pel_pending_messages, dlq_messages_total, publish_retry_total, dlq_write_failures_total
affects: [09, 10, message-processor, api-gateway]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - retryOp/retryPublish pattern: 3 attempts, 100ms/500ms/2s backoff, ctx-aware
    - DLQ pattern: best-effort XAdd to chat:dlq, never blocks message flow
    - processAndAck: retry → DLQ write → ACK (always ACK to leave PEL)

key-files:
  created:
    - services/message-processor/consumer/retry.go
    - services/message-processor/consumer/retry_test.go
    - services/message-processor/consumer/dlq.go
    - services/message-processor/consumer/dlq_test.go
    - services/message-processor/consumer/stream_consumer_test.go
    - services/message-processor/publisher/pubsub_publisher_test.go
    - services/message-processor/handlers/dlq.go
    - services/message-processor/handlers/dlq_test.go
  modified:
    - services/message-processor/consumer/stream_consumer.go
    - services/message-processor/publisher/pubsub_publisher.go
    - services/message-processor/cmd/main.go
    - shared/metrics/processor.go

key-decisions:
  - "processAndAck is the canonical path for all message ACKs — retry then DLQ then ACK ensures PEL is always drained"
  - "DLQ write is best-effort (no retry on writeToDLQ itself) to avoid infinite retry loops"
  - "PublishToMultiple uses individual calls not pipeline — isolates per-overlay failures, returns error only if ALL fail"
  - "consumerName passed explicitly to NewStreamConsumer (not auto-detected inside) — callers control identity, consistent with SDK pattern"
  - "sharedTestMetrics package-level var avoids promauto duplicate registration across tests"

patterns-established:
  - "retryOp: package-private function pattern for retry logic — avoids func-wrapping complexity at call sites"
  - "Sentinel type permanentError deferred to follow-up — current plan uses retryOp for all errors uniformly"

requirements-completed: [D-01, D-04, D-05, D-08, D-09, D-10, D-11]

# Metrics
duration: 10min
completed: 2026-03-29
---

# Phase 08 Plan 01: Message Processor Resilience Summary

**Per-pod unique consumer names, PEL drain on startup, exponential retry with DLQ fallback, Pub/Sub publish retry with per-overlay isolation, and admin DLQ replay endpoint — eliminating all 8 MP-01 through MP-08 silent failure modes**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-29T21:08:35Z
- **Completed:** 2026-03-29T21:18:47Z
- **Tasks:** 2 (both TDD)
- **Files modified:** 12

## Accomplishments

- Eliminated all 8 message-processor silent failure modes (MP-01 through MP-08)
- Built full DLQ infrastructure: write, 7-day trim, and admin replay (DQ-01, DQ-02, DQ-03)
- Added 4 new Prometheus resilience metrics to shared/metrics/processor.go
- All 40+ new tests pass alongside all pre-existing tests (full `go test ./...` green)

## Task Commits

1. **Task 1: Retry helper, DLQ infrastructure, resilience metrics** - `32be6e2` (feat)
2. **Task 2: Harden stream consumer, Pub/Sub retry, DLQ handler** - `ce69796` (feat)

## Files Created/Modified

- `services/message-processor/consumer/retry.go` - retryOp 3-attempt backoff helper (100ms/500ms/2s)
- `services/message-processor/consumer/dlq.go` - writeToDLQ, trimDLQ, drainPEL, formatStreamIDInternal
- `services/message-processor/consumer/stream_consumer.go` - removed ConsumerName constant, added consumerName field, processAndAck helper, drainPEL+trimDLQ in Start(), BUSYGROUP uses strings.Contains
- `services/message-processor/publisher/pubsub_publisher.go` - retryPublish wrapping, per-overlay individual calls, accepts metrics
- `services/message-processor/handlers/dlq.go` - HandleDLQReplay: XRANGE chat:dlq → XADD chat:raw → XDEL
- `services/message-processor/cmd/main.go` - os.Hostname() consumer name, metrics to publisher, /admin/dlq/replay route
- `shared/metrics/processor.go` - PELPendingMessages, DLQMessagesTotal, PublishRetryTotal, DLQWriteFailures added

## Decisions Made

- `processAndAck` is the canonical ACK path — retries first, routes to DLQ on exhaustion, then always ACKs (message leaves PEL regardless)
- DLQ write is single-attempt (best-effort) per D-11 pitfall 5 — recursive retry on DLQ write would infinite-loop
- `PublishToMultiple` replaced pipeline with individual retried calls — error returned only if ALL overlays fail (partial failures logged but succeed)
- `consumerName` passed explicitly into `NewStreamConsumer` by caller — follows existing SDK serviceName pattern, makes intent clear

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] XAutoClaim returns 3 values not 2**
- **Found during:** Task 1 (drainPEL implementation)
- **Issue:** `XAutoClaim(...).Result()` returns `(messages []XMessage, nextCursor string, err error)` — plan pseudo-code used a single result struct
- **Fix:** Used `messages, nextCursor, err :=` destructuring
- **Files modified:** services/message-processor/consumer/dlq.go
- **Committed in:** 32be6e2

**2. [Rule 1 - Bug] Test used closed miniredis addr**
- **Found during:** Task 1 (TestWriteToDLQ_DoesNotPanicOnRedisFailure)
- **Issue:** Test called `newTestConsumer(t, mr)` after `mr.Close()` — `mr.Addr()` panics on closed server
- **Fix:** Captured addr before closing, constructed client directly
- **Files modified:** services/message-processor/consumer/dlq_test.go
- **Committed in:** 32be6e2

---

**Total deviations:** 2 auto-fixed (both Rule 1 - Bug)
**Impact on plan:** Minor implementation details; no scope change.

## Issues Encountered

None beyond the deviations documented above.

## Known Stubs

None — all DLQ infrastructure is fully wired end-to-end.

## Next Phase Readiness

- Phase 08 Plan 02 can now build on top of DLQ infrastructure
- chat:dlq stream is operational for monitoring via Grafana/Redis
- All prior plans still compile: `go build ./...` passes in shared/ and all key services

---
*Phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline*
*Completed: 2026-03-29*
