# ADR-0025: InnerTube Listener Caches activeLiveChatId for Streamer Sends

**Date**: 2026-06-22
**Status**: ✅ Accepted
**Deciders**: Caesar

## Context and Problem Statement

Streamers can send chat to YouTube from the monitor view (`/overlay/[id]/view`). auth-service resolves the target `liveChatId` in three strategies, the first being a read of the `youtube:stream:state:<channelID>` Redis cache. In production that cache is never populated: the only deployed YouTube listener is `youtube-listener-innertube`, which polls chat via InnerTube continuation tokens and never obtains the official `activeLiveChatId` (the quota-based `youtube-listener` that used to write the cache is intentionally not deployed). With strategy 1 dead and the extension videoID (strategy 2) absent for monitor sends, every YouTube send fell through to the `search.list` fallback (strategy 3) — unreliable by design, and observed returning HTTP 403 `accountDelegationForbidden`. Net effect: YouTube send-to-all always failed while Twitch/Kick succeeded.

## Decision Drivers

- Streamer sends to YouTube must work in the InnerTube-only deployment.
- Preserve InnerTube's core value: quota-free *message ingestion*.
- Reliability over the flaky `search.list` discovery path.
- Don't reintroduce the decommissioned quota-based listener.

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
   - ❌ Reverses the intentional InnerTube-only decision and reintroduces full Data API quota/OAuth complexity. Rejected.

## Decision Outcome

**Chosen**: Option 1. The InnerTube listener gains an optional `LiveChatResolver` (Data API `videos.list`, gated by `YOUTUBE_API_KEY`). On stream start it resolves `activeLiveChatId` off the hot path, stores it on the `Stream`, and writes `youtube:stream:state` (schema-compatible with the original `youtube-listener` writer and the auth-service / moderation-service readers). A heartbeat re-publishes the entry (well within its TTL) while the stream is live; when the stream ends it is deleted, and the short TTL self-heals any missed cleanup. With no key configured the feature is disabled and the listener stays fully quota-free.

## Consequences

### Positive
- YouTube streamer send-to-all works in the InnerTube-only deployment via the reliable `videos.list` path.
- Reuses the established `youtube:stream:state` contract — no consumer changes in auth-service / moderation-service.
- Heartbeat + TTL means no teardown site must explicitly delete the cache for correctness.

### Negative
- The InnerTube listener now requires `YOUTUBE_API_KEY` (Data API) to enable sends, and spends ~1 quota unit per stream start — a small, intentional break from strict zero-quota.
- The cache is keyed by channel, so a channel running multiple simultaneous live streams resolves to one chat (pre-existing limitation of the contract).
- The `videos.list` call is not yet accounted against the shared `youtube_quota_usage` table (1 unit/stream is negligible); wiring it to `shared/quota` is a possible follow-up.

## Implementation

- **Files**: `services/youtube-listener-innertube/streams/livechat.go` (`StreamState`, `LiveChatResolver`, `DataAPILiveChatResolver`), `streams/repository.go` (`SetStreamState` / `DeleteStreamState`, TTL), `streams/manager.go` (resolve-on-start, heartbeat refresh loop, delete-on-end), `cmd/main.go` (`YOUTUBE_API_KEY` wiring), plus tests in `streams/livechat_test.go`.
- **Configuration**: set `YOUTUBE_API_KEY` (Data API key, reuse the existing `allchat-secrets/youtube-api-key`) on the `youtube-listener-innertube` Deployment. Without it the cache is disabled.
- **Timeline**: 2026-06-22.

## Related Decisions

- [ADR-0024: Streamer Send-to-All Combined Pill](./0024-send-to-all-combined-pill.md) — the badge/UX half of the same feature; its "known limitation" note is what this ADR resolves.
- [ADR-0006: YouTube Quota Reserve-Confirm-Rollback](./0006-youtube-quota-tracking.md) — the accounting the follow-up could plug into.
- Numbering: ADR-0021/0022 live in caesar-deployment; ADR-0023/0024 are taken in all-chat, so this is 0025 (ADR numbers are shared across both repos).
