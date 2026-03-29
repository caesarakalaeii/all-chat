# ADR-0009: Ring Buffer Publisher for Listener XADD Resilience

**Date**: 2026-03-29
**Status**: Accepted
**Deciders**: All-Chat Platform Team

---

## Context and Problem Statement

All 6 Go listeners (twitch-listener, kick-listener, youtube-listener, youtube-listener-innertube, discord-listener, twitch-eventsub-listener) silently drop messages when `XADD chat:raw` fails. During a temporary Redis unavailability (pod restart, network blip, rolling update), messages from Twitch IRC, Kick WebSocket, YouTube InnerTube, Discord, and EventSub are lost with no retry and no metric increment.

This is diagnosed as issues LI-01, LI-02, and LI-03 in the Phase 8 resilience audit. The pattern is identical across all 6 listeners: the `Publish` method returns the `XADD` error directly and the caller logs it and moves on — the message is gone.

## Decision Drivers

- **Silent data loss is unacceptable**: Streamers depend on reliable message delivery; dropping messages on a temporary Redis blip is a user-visible failure
- **Shared solution over per-listener duplication**: All 6 listeners share the same failure pattern — a shared SDK component eliminates 6 separate fix sites
- **Bounded memory**: Any buffering solution must have a fixed memory ceiling — unbounded queues are a liability in a long-running Kubernetes pod
- **Opt-in adoption**: Existing listeners should be able to adopt the buffer independently without a flag-day migration
- **Observability**: Operations must be able to see buffer depth and drop rate via existing Prometheus scraping

## Considered Options

1. **Channel-based ring buffer (Go channel as queue)**
   - Use a buffered `chan []byte` of capacity N; retry goroutine reads from it
   - Pros: Go-idiomatic, simple, automatic back-pressure
   - Cons: Channels are FIFO queues, not ring buffers — when full, `send` blocks or the sender must select-default (drop). A true ring buffer that overwrites oldest requires a mutex-protected slice anyway. Channel overhead (goroutine wake, scheduler) is slightly higher for a tight 500ms poll loop.

2. **Persistent disk buffer (WAL-style)**
   - Write failed XADD payloads to a local file; background goroutine replays them
   - Pros: Messages survive pod crash (highest durability)
   - Cons: Adds I/O latency on the hot publish path, requires file management (rotation, cleanup), significantly more complex, and is disproportionate to the problem — Redis outages are typically seconds to tens of seconds, well within the in-memory window

3. **Per-listener inline retry (no shared component)**
   - Each listener wraps its `Publish` with a retry loop locally
   - Pros: No shared dependency, each listener can tune its own policy
   - Cons: Code duplication across 6 listeners, no shared observability, no shared capacity limit, inconsistent behavior

4. **Mutex-protected circular buffer with retry goroutine (chosen)**
   - A slice-backed ring buffer with head/tail/size and a single goroutine retrying on 500ms ticks
   - Pros: O(1) enqueue/dequeue, O(1) capacity check, bounded memory, single shared implementation, clear shutdown via `Stop()`, Prometheus metrics for depth and drop rate
   - Cons: Messages in buffer are lost on pod crash (same as current behavior, but with a time window of recovery)

## Decision Outcome

**Chosen**: Option 4 — Mutex-protected circular buffer with retry goroutine

**Rationale**: The core problem is temporary Redis unavailability lasting seconds, not minutes. A 1000-message in-memory buffer with 500ms retry covers ~8 minutes of a high-volume Twitch channel (2 messages/second) before dropping — well beyond any normal Redis blip. The shared SDK implementation eliminates the 6-listener duplication problem and provides a single place to add metrics and tune behavior.

## Consequences

### Positive

- No silent message drops during brief Redis outages (pod restarts, rolling updates, network blips)
- Bounded memory: 1000 messages maximum; oldest is dropped when full (ring semantics prevent unbounded growth)
- Opt-in: existing listeners add one line to wrap their `Publish` function; no forced migration
- Prometheus metrics (`ring_buffer_depth`, `ring_buffer_drops_total`) provide visibility into buffer utilization under load
- Single implementation in `shared/listener/ring_buffer.go` replaces 6 potential per-listener implementations

### Negative

- Messages remaining in the ring buffer when a pod crashes are lost — this is the same behavior as today, but now only applies to messages that arrive during an extended Redis outage rather than any outage
- The retry goroutine uses `context.Background()` (not the caller's context) so it continues retrying even after the main application context is cancelled, until `Stop()` is called — callers must call `Stop()` during shutdown

## Implementation

- **Files**:
  - `shared/listener/ring_buffer.go` — RingBufferPublisher implementation
  - `shared/listener/ring_buffer_test.go` — Full test suite
- **Type**: `RingBufferPublisher` wraps any `PublishFunc func(ctx context.Context, payload []byte) error`
- **Constructor**: `NewRingBufferPublisher(capacity int, publishFn PublishFunc, logger *zap.Logger, serviceName string) *RingBufferPublisher`
- **Capacity**: 1000 messages (default for all listeners)
- **Retry interval**: 500ms using `time.NewTicker`
- **Overflow policy**: Drop oldest (ring semantics), increment `ring_buffer_drops_total`
- **Shutdown**: `Stop()` closes `stopCh` and calls `wg.Wait()` — safe to call from any goroutine
- **Metrics**: `ring_buffer_depth{service}` gauge, `ring_buffer_drops_total{service}` counter
- **Timeline**: Phase 8 Plan 04

## Related Decisions

- **D-07** (Phase 8 CONTEXT.md): This plan implements the Redis publish retry buffer identified in the Phase 8 resilience audit
- **ADR-0002**: Redis Streams + Pub/Sub Hybrid — the XADD failure mode this ADR addresses is in the Streams write path
- **ADR-0001**: Standard Go Layout — ring_buffer.go follows the same package structure as other `shared/listener/` files
