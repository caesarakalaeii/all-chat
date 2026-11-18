# ADR-0028: Engagement Chat-Command Write-Path

**Date**: 2026-07-01
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

Issue #523 adds cross-platform polls, predictions and viewer points (ADR-0029). The feature only pays off if the default viewer can participate, and the default viewer has not installed the optional, low-adoption browser extension. The universal, zero-install participation channel is chat commands ("`!vote 2`", a bare "`2`", "`!predict 1 500`") — every ingested message already carries a stable per-platform user id. The question is how a chat message becomes a validated vote/wager without burdening the message-processor hot path, and how feedback is delivered. The baseline must not require the opt-in send scope (ADR-0012 posture), and the existing `viewer_platform_identities` + `GetOrCreateViewerByPlatform` (migration 035) already resolve a durable `viewer_id`.

## Decision

The message-processor detects candidate commands on the hot path and forwards them to a durable stream; engagement-service does the parsing and DB work off the hot path.

- **Hot-path hook** (`message-processor/consumer/engagement.go`): for a message passing a cheap in-process pre-check (`looksLikeCommand`) whose channel has a live engagement (`EXISTS engagement:active:{platform}:{channel}`, a refcounted key engagement-service maintains while a poll/prediction is ACTIVE), forward a `CommandJob` to the approx-`MAXLEN`-capped `engagement:commands` stream. Ordinary chat pays only the in-process check.
- **Off-hot-path consumer** (`engagement-service/consumer/command.go`): parse, resolve `viewer_id` via `GetOrCreateViewerByPlatform`, apply the vote/wager with per-(poll|prediction, viewer) idempotency, ack after the DB write.
- **No platform-chat spam.** Confirmations are silent by default; feedback reaches the viewer via the overlay tally broadcast and the authenticated pull endpoint `GET /viewers/me/engagement`.
- **Earning is separate and best-effort.** Event-driven earns (subs/bits/donations/gifts) ride the Pub/Sub channel `engagement:events` — a missed earn is an unpaid point, not a corrupted balance. Votes/wagers, which can corrupt balances, get the durable stream.
- **Multi-overlay channels**: a `CommandJob` carries `(platform, channel_id)`; engagement-service applies to every overlay sourcing the channel that has a live engagement.

Alternatives considered: engagement-service consuming `chat:raw` directly (a second consumer on the whole firehose, re-implementing dedup/DLQ/PEL machinery); bidirectional frames on the overlay WebSocket (an anonymous one-way broadcast for OBS; large net-new surface). Both rejected.

**Premium gate on starting a round (2026-07-07 amendment).** *Starting* a poll/prediction posts the question and participate link to chat (`announce_on_start`), which spends the streamer's send quota, so `POST …/polls` and `POST …/predictions` are per-route gated behind `shared/middleware.RequirePremium("engagement")` (ADR-0008); a non-premium owner gets 403. Managing an already-open round, the earn config, viewer participation and earning are deliberately not gated. The `GateEngagement` gate is seeded premium in migration 076 and can graduate to free via the feature-gate admin endpoint with no redeploy; a cache-load failure leaves the feature premium (fail closed).

## Consequences

- The extension and the no-install web page become enhancements, not gates; Twitch viewers additionally get native poll/prediction mirroring (ADR-0030) with points semantics from ADR-0029.
- One new stream (`engagement:commands`), one Pub/Sub channel (`engagement:events`), one flag-key family (`engagement:active:*`).
- Chat-participation *points* stay deferred in v1 (the earn consumer supports the `engagement:chat` signal, but nothing publishes it yet).
- The durable-stream transport survives an engagement-service restart; worker restarts re-deliver at-least-once and idempotency keys make that safe.

## Related

- [ADR-0029](./0029-viewer-points-economy-and-prediction-payout.md) — points semantics the votes/wagers debit against.
- [ADR-0030](./0030-twitch-native-engagement-mirroring.md) — native mirroring sits beside this path.

Runbook-level help: `docs/runbooks/fixing-service-issues.md` and `docs/troubleshooting/decision-tree.md`.
