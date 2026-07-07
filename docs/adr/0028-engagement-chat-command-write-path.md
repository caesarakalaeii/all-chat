# ADR-0028: Engagement Chat-Command Write-Path

**Date**: 2026-07-01
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Issue #523 adds cross-platform **polls, predictions, and viewer points**. The feature only pays off if the *default* viewer can participate — and the default viewer has not installed the (explicitly optional, low-adoption) browser extension. Gating votes/wagers behind the extension produces empty polls and a dead feature.

Every ingested chat message already carries a stable per-platform user id (`UnifiedChatMessage.User.ID`) across Twitch/YouTube/Kick/TikTok. So the universal, zero-install participation channel is **chat commands** ("`!vote 2`", a bare "`2`", "`!predict 1 500`"). The question is *how* a chat message becomes a validated vote/wager without regressing the message-processor hot path (a tight normalize→enrich→publish loop with per-message retry/DLQ), and how feedback is delivered.

## Decision Drivers

- **Universal reach.** Must work for every chatter on every platform, no login, no install.
- **Hot-path safety.** message-processor is latency-sensitive and central; it must not take on DB work or a second firehose consumer.
- **Durability of participation.** A dropped vote/wager is worse than a dropped earn — the participation transport must survive a consumer restart.
- **No chat spam.** Posting confirmations back to the platform requires the opt-in send scope (ADR-0012 posture); the baseline must not depend on it.
- **Reuse existing identity.** `viewer_platform_identities` + `GetOrCreateViewerByPlatform` (migration 035) already resolve a durable `viewer_id` from `(platform, platform_user_id)`.

## Considered Options

1. **message-processor detects a candidate command and `XADD`s a job to a new durable `engagement:commands` stream; engagement-service consumes it via its own group.**
   - ✅ Hot path adds only a byte-cheap text pre-check plus, on the rare hit, a single Redis `EXISTS` and `XADD`. All parsing/validation/DB writes happen off the hot path. Durable (survives restart), acked, DLQ-free reuse.
   - ❌ One new stream + a small hot-path hook.
2. **engagement-service runs a second `XREADGROUP` consumer group directly on `chat:raw`.**
   - ✅ No message-processor change.
   - ❌ A second consumer on the whole firehose: every message deserialized twice, engagement scaling coupled to total chat volume, and it re-implements the native-dedup/DLQ/PEL machinery message-processor already owns.
3. **Bidirectional command frames on the overlay WebSocket.**
   - ❌ That socket is an anonymous one-way broadcast for OBS; adding authenticated upstream command semantics is large net-new surface and doesn't reach non-extension viewers anyway.

## Decision Outcome

**Chosen**: Option 1.

- **Hot-path hook** (`message-processor/consumer/engagement.go`): for a chat message that passes a cheap in-process pre-check (`looksLikeCommand`) *and* whose channel currently has a live engagement (`EXISTS engagement:active:{platform}:{channel}` — a refcounted SET the engagement-service maintains while a poll/prediction is ACTIVE), forward a `CommandJob` to `engagement:commands` (approx-`MAXLEN`-capped). Ordinary chat pays only the in-process check.
- **Off-hot-path processing** (`engagement-service/consumer/command.go`): parse the grammar, resolve `viewer_id` via `GetOrCreateViewerByPlatform`, look up the overlay's active poll/prediction, and apply the vote/wager with per-`(poll,viewer)` / per-`(prediction,viewer)` idempotency. Ack after the DB write (message-processor's proven pattern).
- **No platform-chat spam.** Confirmations/rejections are silent on the platform by default. Feedback reaches the viewer through the aggregate overlay tally (broadcast) and the authenticated web page's pull endpoint (`GET /viewers/me/engagement`). A future opt-in "confirm in chat" can reuse the moderation send-scope check.
- **Earning is separate and best-effort.** Event-driven earning (subs/bits/donations/gifts) is republished on the Pub/Sub channel `engagement:events` (not the durable stream): a missed earn is an unpaid point, never a corrupted balance. Votes/wagers, which *can* corrupt balances, get the durable stream.

## Consequences

- The extension and the no-install web page become *enhancements* (richer UI, private balance display, wager confirmation), not gates. Twitch viewers additionally get their native poll/prediction UI mirrored (see ADR-0029 for the points semantics).
- One new Redis stream (`engagement:commands`), one Pub/Sub channel (`engagement:events`), and one refcounted flag key family (`engagement:active:*`). The hot path gains a pre-check + at most one `EXISTS`/`XADD` for command-shaped messages on channels with a live engagement.
- Chat-participation *points* (as opposed to votes) are deferred in v1 to keep the hot path free of a per-message Redis op; the earn consumer already supports the `engagement:chat` signal when a future revision decides the cost is worth it.
- Multi-overlay channels: a `CommandJob` carries `(platform, channel_id)`; engagement-service resolves every overlay that carries the channel and applies to those with a live engagement.

## Update (2026-07-07): premium gate on starting a round

*Starting* a poll/prediction is gated behind `shared/middleware.RequirePremium("engagement")` (ADR-0008), applied per-route to `POST …/polls` and `POST …/predictions` in engagement-service `main.go`. Rationale: opening a round posts the question + participate link to chat (`announce_on_start`), which consumes the streamer's **send quota** — a paid capability, consistent with the send-scope posture in this ADR's drivers. A non-premium owner gets `403` (the dashboard also surfaces the message via the returned `error`). Managing an already-open round (close/lock/resolve/cancel), the earn config, viewer participation, and points earning are deliberately **not** gated — a viewer never needs premium to take part, and points accrual sends no messages. The gate (`GateEngagement`) is seeded premium in migration 076 and can graduate to free via the feature-gate admin endpoint with no redeploy (mirrors moderation, ADR-0017/061). Unknown-key fail-closed (FeatureGateCache) means a cache-load failure keeps the feature premium rather than opening it to everyone.
