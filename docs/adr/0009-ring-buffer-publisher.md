# ADR-0009: Ring Buffer Publisher for Listener XADD Resilience

**Date**: 2026-03-29
**Status**: Accepted
**Revised**: 2026-11-03
**Deciders**: All-Chat Platform Team

## Context

Listeners silently dropped messages when `XADD chat:raw` failed: `Publish` returned the error directly, the caller logged it and moved on. During a temporary Redis unavailability (pod restart, network blip, rolling update) messages were lost, user-visibly, with no retry and no metric.

## Decision

`RingBufferPublisher` (`shared/listener/ring_buffer.go`): a mutex-protected circular buffer (slice-backed, head/tail/size) of capacity 1000 with a retry goroutine on 500ms ticks. `Publish` enqueues and returns; the goroutine retries `XADD` until the stream accepts or the buffer overflows, dropping oldest and incrementing a drop counter. Memory is bounded; adoption is opt-in per listener (`NewRingBufferPublisher(capacity, publishFn, logger, serviceName)` wrapping any `PublishFunc`); Prometheus metrics `ring_buffer_depth{service}` and `ring_buffer_drops_total{service}` give visibility. The retry goroutine runs on `context.Background()` so it survives main-context cancellation until `Stop()` is called during shutdown.

Accepted trade-off: messages in the buffer are lost on pod crash, which now applies only to messages arriving during an extended Redis outage.

Alternatives rejected: a Go channel as queue (FIFO with blocking or drop on full, giving ring semantics still needs a mutex slice); a persistent disk WAL (disproportionate: Redis outages last seconds to tens of seconds, well within an 8-minute in-memory window at a high-volume channel's 2 msg/s); per-listener inline retries (duplication, no shared metrics or capacity limit).

## Implementation

`shared/listener/ring_buffer.go` (+ tests). Adopting services: twitch-listener, kick-listener, youtube-listener-innertube, twitch-eventsub-listener, discord-listener, owncast-listener, goodgame-listener, picarto-listener, facebook-listener, altstream-listener, moderation-service. Not adopted: youtube-listener (official Data-API mode) and the TypeScript tiktok-listener still publish `chat:raw` directly.

Related: ADR-0002 (the Streams write path this buffers), ADR-0001.
