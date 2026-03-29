---
phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
plan: 04
subsystem: infra
tags: [redis, resilience, ring-buffer, prometheus, go, shared-sdk]

# Dependency graph
requires:
  - phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
    provides: shared/listener SDK — all 6 Go listeners use this package

provides:
  - RingBufferPublisher in shared/listener/ring_buffer.go — opt-in XADD retry buffer for all Go listeners
  - ADR-0009 documenting the ring buffer publisher architectural decision

affects:
  - twitch-listener
  - kick-listener
  - youtube-listener
  - youtube-listener-innertube
  - discord-listener
  - twitch-eventsub-listener

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RingBufferPublisher: wrap any PublishFunc to get automatic buffering + 500ms retry"
    - "NewRingBufferPublisherWithRegisterer for isolated Prometheus registry in tests"
    - "TDD: RED commit (failing tests) → GREEN commit (passing implementation)"

key-files:
  created:
    - shared/listener/ring_buffer.go
    - shared/listener/ring_buffer_test.go
    - docs/adr/0009-ring-buffer-publisher.md
  modified:
    - docs/adr/README.md

key-decisions:
  - "Metrics use prometheus.NewGauge/NewCounter (not promauto) to allow per-registry injection for tests"
  - "NewRingBufferPublisherWithRegisterer exposes registerer parameter; NewRingBufferPublisher uses DefaultRegisterer"
  - "drainOneTick calls requeue on failure — head item stays at front for next tick, avoiding N failures per tick"
  - "dropsCount int64 field mirrors dropsTotal counter for test assertions (prometheus counters are not readable)"

patterns-established:
  - "Ring buffer pattern: capacity=1000, 500ms retry, oldest-drop on overflow — standard for all listener XADD failures"

requirements-completed: [D-07]

# Metrics
duration: 4m
completed: 2026-03-29
---

# Phase 08 Plan 04: Ring Buffer Publisher Summary

**Opt-in RingBufferPublisher added to shared/listener SDK — 1000-msg circular buffer with 500ms retry goroutine eliminates silent XADD drops across all 6 Go listeners**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T21:08:25Z
- **Completed:** 2026-03-29T21:12:53Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- ADR-0009 documents the ring buffer architectural decision with alternatives (channel-based, disk buffer, per-listener inline retry) and their trade-offs
- RingBufferPublisher wraps any `PublishFunc` with a mutex-protected circular buffer that buffers failed XADD operations instead of dropping them
- Retry goroutine polls every 500ms using `context.Background()` — decoupled from caller's context lifecycle
- Ring semantics: when buffer reaches capacity (1000), oldest message dropped, `ring_buffer_drops_total` incremented
- `Stop()` closes `stopCh` and waits for retry goroutine to exit cleanly — safe for graceful shutdown
- 7 tests covering all behaviors: success passthrough, failure buffering, retry drain, capacity overflow, clean stop, background context, FIFO ordering

## Task Commits

1. **Task 1: ADR-0009** - `9f3d9fe` (docs)
2. **Task 2: RED** - `e101b87` (test — failing tests)
3. **Task 2: GREEN** - `c3a920e` (feat — passing implementation)

## Files Created/Modified

- `shared/listener/ring_buffer.go` — RingBufferPublisher with enqueue, dequeue, retryLoop, requeue, Stop
- `shared/listener/ring_buffer_test.go` — 7 TestRingBuffer* tests using isolated prometheus.NewRegistry()
- `docs/adr/0009-ring-buffer-publisher.md` — ADR documenting the architectural decision
- `docs/adr/README.md` — Updated index with ADR-0009 entry, total count updated to 9

## Decisions Made

- **Metrics via prometheus.NewGauge/NewCounter (not promauto):** `promauto` auto-registers with `DefaultRegisterer` — multiple test instances would panic on duplicate registration. Using `prometheus.NewGauge` + explicit `reg.Register()` with best-effort error allows `NewRingBufferPublisherWithRegisterer` to accept a per-test `prometheus.NewRegistry()`.
- **dropsCount int64 field:** Prometheus counter values are not directly readable from the counter object; added `dropsCount int64` under the same mutex for test assertions via `getDropsTotal()`.
- **drainOneTick uses requeue on failure:** When a retry attempt fails, the item is put back at the head of the buffer (not lost, not re-appended at tail). This preserves FIFO ordering and stops processing for the current tick to avoid thundering herd.
- **Best-effort `reg.Register()` (ignoring error):** Allows the same service name to create multiple publishers in tests without panicking — duplicate registration silently reuses the first registration.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Used per-test Prometheus registry to prevent duplicate registration panics**
- **Found during:** Task 2 (test writing)
- **Issue:** Tests create multiple `RingBufferPublisher` instances. Using `promauto` with `DefaultRegisterer` would panic on the second creation with the same `serviceName` label. Plan spec used `promauto.NewGauge` directly in constructor.
- **Fix:** Changed implementation to use `prometheus.NewGauge/NewCounter` with a `prometheus.Registerer` interface. `NewRingBufferPublisher` uses `DefaultRegisterer`; tests use `NewRingBufferPublisherWithRegisterer(prometheus.NewRegistry())`. This is more correct than the plan's approach.
- **Files modified:** shared/listener/ring_buffer.go, shared/listener/ring_buffer_test.go
- **Verification:** All 7 tests pass without panic
- **Committed in:** c3a920e

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Auto-fix necessary for test correctness. Public API is a superset of plan spec. No scope creep.

## Issues Encountered

None — implementation was straightforward. The only adjustment was the Prometheus registry injection pattern for test isolation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `RingBufferPublisher` is ready for opt-in adoption in all 6 Go listeners
- Listeners wrap their existing `Publish` method: `rb := listener.NewRingBufferPublisher(1000, publisher.Publish, logger, "twitch")`
- Prometheus metrics `ring_buffer_depth` and `ring_buffer_drops_total` will appear automatically once a listener adopts the wrapper
- ADR-0009 is in place; no further architectural review needed before adoption

---
*Phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline*
*Completed: 2026-03-29*
