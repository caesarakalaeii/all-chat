# ADR-0033: Bounded-Concurrency Enrichment in the Message Processor

**Date**: 2026-07-17
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The message-processor consumes raw chat from the `chat:raw` Redis stream
(`XREADGROUP`, consumer group `message-processor`), enriches each message through a
sequence of stages (normalization, avatar, badges, emotes, cheermotes, viewer identity,
pronouns) and publishes the result to per-overlay Pub/Sub channels.

Until now, `consumer/stream_consumer.go` read a batch of up to `ReadCount` (100)
messages and then processed them **strictly one at a time** on the single consume-loop
goroutine (`readAndProcess` looped over `stream.Messages` calling `processAndAck`
synchronously). Every enrichment stage does blocking I/O — several Redis round-trips per
message (baseline ~50ms RTT in production) plus HTTP calls to the emote-service, Twitch
Helix, and the Alejo pronouns API, and Postgres queries on cache misses.

Serial processing gives a hard per-pod throughput ceiling of `1 / per-message-latency`.
When any upstream degrades, per-message latency rises and throughput falls below the
arrival rate, so unread messages accumulate in the stream and are delivered to viewers
tens of seconds late. There is no bulkhead: a single slow dependency stalls the entire
pipeline.

**Production incident (2026-07-17, `MessageProcessorStreamLagWarning` → 30–60s).** With
input at only ~2–3 msg/s, all three pods idle at ~3% CPU and a healthy Redis
(0 evictions, publish latency for other services flat), the lag was a pure throughput
collapse. It coincided with a ~5× spike in emote-service upstream (7TV/BTTV/FFZ) API
errors and a more cache-miss-heavy traffic period: per-message enrichment latency climbed
from ~30ms to ~0.5–1s (end-to-end p95 pinned at the 1s histogram ceiling), and every
Redis/HTTP/DB-backed stage slowed together. Because processing was serial, the three
pods' combined ~3 msg/s could no longer keep pace and the stream backed up.

A secondary amplifier: the `PronounEnricher` (ADR-0010) did **not** cache anything when
the Alejo API errored or timed out, so during upstream degradation every message from an
uncached user re-issued a fresh call bounded only by the 3s client timeout.

## Decision

1. **Process each read batch with bounded concurrency.** `readAndProcess` now hands the
   batch to `processBatch`, which runs up to `concurrency` messages in parallel through a
   semaphore-bounded worker pool and waits for the batch to finish before the next
   `XREADGROUP`. Default `DefaultProcessConcurrency = 16`, overridable via
   `MP_PROCESS_CONCURRENCY`. This raises the per-pod throughput ceiling ~16× and lets the
   pipeline absorb upstream latency spikes instead of serializing behind them. Waiting for
   the batch to drain before the next read bounds in-flight messages and provides natural
   backpressure.

2. **Per-message semantics are unchanged.** Each message is still independently retried,
   DLQ-routed, and `XACK`ed inside `processAndAck`, so at-least-once delivery, DLQ
   routing, native-id dedup (ADR-0015), and the deletion buffer all behave exactly as
   before — only the dispatch is parallel.

3. **Make the concurrently-invoked enrichers race-free.** The `AvatarEnricher` and
   `BadgeEnricher` app-access-token fields are now guarded by `token()`/`setToken()`
   accessors, and the token HTTP refresh runs without holding the lock (so a slow refresh
   never serializes lookups). All other collaborators were audited and are already safe:
   the emote cache and viewer-identity/pronoun caches are Redis-backed, the pronoun map is
   read-only after construction, the overlay router is stateless over the DB pool, and
   `seventvManager` was already invoked from `go func()`s.

4. **Negative-cache pronoun lookup failures.** On an Alejo API transport error or
   non-200/404 status, `PronounEnricher` now caches the empty sentinel with a short
   `PronounErrorCacheTTL` (5m) so a degraded upstream is not re-hit on every subsequent
   message, while pronouns still self-heal within minutes of recovery.

## Why concurrency is safe here (ordering)

The pipeline never guaranteed global ordering: the consumer group already fans messages
out across multiple pods that process in parallel with no cross-consumer ordering, and
overlays render chat by timestamp. Intra-pod concurrency is the same model applied within
a pod, so it introduces no ordering guarantee that did not already have to hold.

The one ordering-sensitive interaction — a deletion vs. the message it deletes — is
handled independently of processing order by the reorder-tolerant deletion buffer: a
deletion whose target has not been seen is buffered and applied when the target arrives,
and this already had to tolerate the deletion and its target landing on different pods
concurrently. Per-message dedup (`dedup.IsDuplicate`, `IsDuplicateNativeID`) and the
send-to-all claim use atomic Redis `SETNX`, which is likewise already correct under the
existing multi-pod concurrency.

## Consequences

**Positive**
- A pod sustains ~16× more throughput, so an upstream (emote-service, Twitch, Alejo, DB)
  latency spike no longer collapses the pipeline into unbounded lag.
- No bulkhead-less head-of-line blocking: a slow message no longer stalls the rest of its
  batch.
- Two latent data races on the Twitch app token (previously masked by the serial loop)
  are removed, verified with `-race`.
- A pronouns-API outage can no longer amplify into a processing-throughput collapse.

**Negative / trade-offs**
- Higher peak concurrent load on downstream dependencies (Redis pool of 50 is ample;
  Twitch/DB fan-out is bounded by `concurrency`). Tunable via `MP_PROCESS_CONCURRENCY`.
- Log lines for a batch may interleave; per-message correlation is via `stream_id`.
- Ordering within a batch is explicitly not preserved (see above) — acceptable because it
  was never guaranteed.

## Alternatives Considered

- **Targeted resilience only** (tighter enricher timeouts + negative caching, keep serial
  processing): smaller change but leaves the ~1–2 msg/s/pod ceiling, so the pipeline stays
  one upstream hiccup away from lag. Rejected as the sole fix; the pronoun negative-cache
  piece is kept as complementary hardening.
- **Horizontal scale-out (more pods)**: raises the ceiling linearly but wastes resources
  (pods idle at 3% CPU — the bottleneck is per-goroutine I/O blocking, not CPU) and does
  not fix the per-message head-of-line blocking. Concurrency addresses the actual
  I/O-bound shape.
- **Persistent worker pool across batches (pipelined reads)**: marginally higher
  throughput but complicates PEL/ACK bookkeeping and shutdown; per-batch fan-out with a
  drain barrier is simpler and sufficient.
