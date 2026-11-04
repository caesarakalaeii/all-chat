# ADR-0025: InnerTube Listener Caches activeLiveChatId for Streamer Sends

**Date**: 2026-06-22
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Caesar

## Context and Problem Statement

Streamers can send chat to YouTube from the monitor view (`/overlay/[id]/view`). auth-service resolves the target `liveChatId` in three strategies, the first being a read of the `youtube:stream:state:<channelID>` Redis cache. At the time this ADR was written that cache was never populated in production: the only deployed YouTube listener was `youtube-listener-innertube`, which polls chat via InnerTube continuation tokens and never obtains the official `activeLiveChatId` (the quota-based `youtube-listener` that used to write the cache was intentionally not deployed; ADR-0065 later revived it only for opted-in, owner-verified, premium channels, which does not cover sends for any other channel). With strategy 1 dead and the extension videoID (strategy 2) absent for monitor sends, every YouTube send fell through to the `search.list` fallback (strategy 3) — unreliable by design, and observed returning HTTP 403 `accountDelegationForbidden`. Net effect: YouTube send-to-all always failed while Twitch/Kick succeeded.

## Decision Drivers

- Streamer sends to YouTube must work on the default InnerTube chat path.
- Preserve InnerTube's core value: quota-free *message ingestion*.
- Reliability over the flaky `search.list` discovery path.
- Don't reintroduce the general-population quota-based listener (the ADR-0065 revival for opted-in premium channels is orthogonal).

## Considered Options

1. **InnerTube listener resolves and caches `activeLiveChatId` per stream** (chosen) — when a stream starts, call the Data API `videos.list?part=liveStreamingDetails` once (1 quota unit) and publish `youtube:stream:state:<channelID>` (the existing cache contract), refreshed on a heartbeat and TTL'd.
   - ✅ Revives auth-service strategy 1 with the reliable `videos.list` path; works for all monitor sends.
   - ✅ Ingestion stays quota-free; cost is ~1 unit per stream start (negligible).
   - ❌ Introduces a Data API key dependency on a service that had none; a small, bounded departure from "zero quota".

2. **Fix the `search.list` 403 / API-key project** — keep relying on strategy 3.
   - ✅ No listener change.
   - ❌ `search.list` is explicitly unreliable (index lag) even when not 403'ing; doesn't give a dependable send path. Rejected as the primary fix (still worth checking as ops hygiene).

3. **Redeploy the quota-based `youtube-listener`** — it already writes the cache.
   - ✅ Reuses existing code.
   - ❌ Reverses the intentional InnerTube-only decision and reintroduces full Data API quota/OAuth complexity. Rejected. (ADR-0065 later revived `youtube-listener` for opted-in, owner-verified, premium channels; that does not change this choice, because this ADR's cache written by the InnerTube listener is what makes sends work for every other channel.)

## Decision Outcome

**Chosen**: Option 1. The InnerTube listener gains an optional `LiveChatResolver` (Data API `videos.list`, gated by `YOUTUBE_API_KEY`). On stream start it resolves `activeLiveChatId` off the hot path, stores it on the `Stream`, and writes `youtube:stream:state` (schema-compatible with the original `youtube-listener` writer and the auth-service / moderation-service readers). A heartbeat re-publishes the entry (well within its TTL) while the stream is live; when the stream ends it is deleted, and the short TTL self-heals any missed cleanup. With no key configured the feature is disabled and the listener stays fully quota-free.

## Consequences

### Positive
- YouTube streamer send-to-all works without depending on the quota listener, via the reliable `videos.list` path (InnerTube stays the default chat path; ADR-0065's listener revival covers only opted-in premium channels).
- Reuses the established `youtube:stream:state` contract — no consumer changes in auth-service / moderation-service.
- Heartbeat + TTL means no teardown site must explicitly delete the cache for correctness.

### Negative
- The InnerTube listener requires `YOUTUBE_API_KEY` (Data API) to enable sends, and spends ~1 quota unit per stream start — a small, intentional break from strict zero-quota. Without the key the cache is disabled and streamer sends stay broken (they fall back to the unreliable `search.list` path).
- The cache is keyed by channel, so a channel running multiple simultaneous live streams resolves to one chat (a limitation of the contract).
- The `videos.list` call (1 unit per stream start) is not accounted against the shared `youtube_quota_usage` table. It is a single call per stream, so the cost stays negligible; subscriber-alert polling in the same service, which spends far more, *is* accounted (see the service README).

## Implementation

- **Files**: `services/youtube-listener-innertube/streams/livechat.go` (`StreamState`, `LiveChatResolver`, `DataAPILiveChatResolver`), `streams/repository.go` (`SetStreamState` / `DeleteStreamState`, TTL), `streams/manager.go` (resolve-on-start, heartbeat refresh loop, delete-on-end), `cmd/main.go` (`YOUTUBE_API_KEY` wiring), plus tests in `streams/livechat_test.go`.
- **Configuration**: set `YOUTUBE_API_KEY` (Data API key) on the `youtube-listener-innertube` deployment. Without it the cache is disabled and sends fall back to the unreliable `search.list` path. See the env-var table in the service README.
- **Timeline**: 2026-06-22.

## Related Decisions

- [ADR-0024: Streamer Send-to-All Combined Pill](./0024-send-to-all-combined-pill.md) — the badge/UX half of the same feature; its "known limitation" note is what this ADR resolves.
- [ADR-0006: YouTube Quota Reserve-Confirm-Rollback](./0006-youtube-quota-tracking.md) — the accounting the follow-up could plug into.
- Numbering: ADR-0021/0022 live in caesar-deployment; ADR-0023/0024 are taken in all-chat, so this is 0025 (ADR numbers are shared across both repos). [ADR-0065](./0065-premium-official-youtube-api-listener.md) later revived the quota-based listener for opted-in premium channels; [ADR-0023](./0023-decoupled-youtube-quota-monitor.md) is the quota monitor.
