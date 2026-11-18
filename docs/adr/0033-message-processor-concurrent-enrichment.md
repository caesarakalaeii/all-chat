# ADR-0033: Bounded-Concurrency Enrichment in the Message Processor

**Date**: 2026-07-17
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

The message-processor consumes raw chat from the `chat:raw` Redis stream (`XREADGROUP`, group `message-processor`) and enriches each message through several stages (normalization, avatar, badges, emotes, cheermotes, viewer identity, pronouns) before publishing to per-overlay Pub/Sub channels. Processing was strictly one-at-a-time on the single consume-loop goroutine, and every enrichment stage does blocking I/O — several Redis round-trips plus HTTP calls to the emote-service, Twitch Helix and the Alejo pronouns API, and Postgres queries on cache misses. Serial processing caps per-pod throughput at `1 / per-message-latency`, so any upstream degradation dropped throughput below the arrival rate and accumulated lag with no bulkhead.

Production incident (2026-07-17): at only ~2–3 msg/s input with all pods idle at ~3% CPU and a healthy Redis, lag hit 30–60s. A ~5× spike in emote-service upstream errors plus a cache-miss-heavy traffic period pushed per-message enrichment from ~30ms to 0.5–1s; because processing was serial, the pipeline could not keep pace. Amplifier: the `PronounEnricher` cached nothing on an Alejo API error or timeout, so every message from an uncached user re-issued a call during degradation.

## Decision

1. **Bounded concurrency per read batch.** `readAndProcess` hands each batch to `processBatch`, which runs up to `concurrency` messages in parallel through a semaphore-bounded worker pool and waits for the batch to drain before the next `XREADGROUP` (natural backpressure). Default `DefaultProcessConcurrency = 16`, overridable via `MP_PROCESS_CONCURRENCY`.
2. **Per-message semantics unchanged.** Each message is still independently retried, DLQ-routed and acked inside `processAndAck`; at-least-once delivery, DLQ routing, native-id dedup (ADR-0015) and the deletion buffer behave exactly as before — only the dispatch is parallel.
3. **Race-free enrichers.** The `AvatarEnricher` and `BadgeEnricher` app-access-token fields are guarded by accessors, with the token HTTP refresh run without holding the lock. All other collaborators were audited and already safe (Redis-backed caches, read-only pronoun map, stateless router, pre-existing goroutine-safe `seventvManager`).
4. **Negative-cache pronoun failures.** On an Alejo transport error or non-200/404 status, the empty sentinel is cached for `PronounErrorCacheTTL` (5m) so a degraded upstream is not re-hit on every message, yet still self-heals within minutes.

**Why concurrency is safe.** The pipeline never guaranteed global ordering: the consumer group already fans out across pods with no cross-consumer ordering, and overlays render chat by timestamp. The one ordering-sensitive interaction (deletion vs. the message it deletes) is handled independently by the reorder-tolerant deletion buffer, which already had to tolerate the pair landing on different pods. Per-message dedup and the send-to-all claim use atomic Redis `SETNX`, already correct under multi-pod concurrency.

Alternatives considered: targeted resilience only (tighter timeouts + negative caching, keep serial — leaves the low throughput ceiling; the negative cache is kept as complementary hardening); horizontal scale-out (bottleneck is per-goroutine I/O blocking, not CPU; pods idle at 3%); a persistent worker pool across batches (complicates PEL/ACK bookkeeping and shutdown for marginal gain).

## Consequences

- A pod sustains roughly 16× more throughput, so an upstream latency spike no longer collapses the pipeline into unbounded lag; a slow message no longer stalls its batch.
- Two latent data races on the Twitch app token (masked by the serial loop) removed, verified with `-race`.
- Higher peak concurrent load on downstreams (tunable via `MP_PROCESS_CONCURRENCY`); log lines from a batch may interleave (correlate by `stream_id`); ordering within a batch is explicitly not preserved.
