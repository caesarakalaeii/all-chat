# ADR-0025: InnerTube Listener Caches activeLiveChatId for Streamer Sends

**Date**: 2026-06-22
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Caesar

## Context

Streamers send chat to YouTube from the monitor view. auth-service resolves the target `liveChatId` in three strategies, the first being the `youtube:stream:state:<channelID>` Redis cache. At decision time that cache was never populated in production: the only deployed YouTube listener was `youtube-listener-innertube`, which polls chat via InnerTube continuation tokens and never obtains the official `activeLiveChatId` (ADR-0065 later revived the quota-based listener only for opted-in, owner-verified, premium channels, which does not cover sends for any other channel). With strategy 1 dead and strategy 2 (extension videoID) absent for monitor sends, every YouTube send fell through to the `search.list` fallback (strategy 3) — unreliable by design and observed returning HTTP 403 `accountDelegationForbidden`. Net effect: YouTube send-to-all always failed while Twitch/Kick succeeded.

## Decision

The InnerTube listener resolves and caches `activeLiveChatId` per stream. On stream start it calls the Data API `videos.list?part=liveStreamingDetails` once (1 quota unit), stores the id on the `Stream`, and writes `youtube:stream:state:<channelID>` — schema-compatible with the original `youtube-listener` writer and the auth-service / moderation-service readers. A heartbeat re-publishes the entry while the stream is live, well within its TTL; the entry is deleted at stream end and the short TTL self-heals missed cleanups. Resolution is gated behind an optional `YOUTUBE_API_KEY`; without a key the feature is disabled and the listener stays fully quota-free.

Alternatives considered:

- **Fix the `search.list` 403 and keep relying on it**: index lag makes it unreliable even when it does not 403. Rejected as the primary fix.
- **Re-deploy the quota-based `youtube-listener`** (it already writes the cache): reverses the InnerTube decision and reintroduces full Data API quota/OAuth complexity. Rejected.

## Consequences

- YouTube streamer sends work on the default InnerTube path; ingestion stays quota-free apart from ~1 unit per stream start.
- Reuses the established cache contract — no consumer changes.
- The InnerTube listener now needs `YOUTUBE_API_KEY` to enable sends; without it sends fall back to the unreliable `search.list` path.
- The cache is keyed by channel, so a channel running multiple simultaneous live streams resolves to one chat.
- The per-stream `videos.list` call is not accounted against `youtube_quota_usage`; it stays negligible (subscriber-alert polling in the same service spends far more and *is* accounted).

## Implementation

- `services/youtube-listener-innertube/streams/livechat.go` (`StreamState`, `LiveChatResolver`, `DataAPILiveChatResolver`), `streams/repository.go` (`SetStreamState` / `DeleteStreamState`), `streams/manager.go` (resolve-on-start, heartbeat, delete-on-end), `cmd/main.go` (`YOUTUBE_API_KEY`).
- Tests: `streams/livechat_test.go`.

## Related

- [ADR-0024](./0024-send-to-all-combined-pill.md) — the badge/UX half of the same feature.
- [ADR-0006](./0006-youtube-quota-tracking.md) — the accounting table the follow-up could plug into.
- Numbering: ADR-0021/0022 live in caesar-deployment; ADR numbers are shared across both repos.
