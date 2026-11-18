# ADR-0024: Streamer Send-to-All Combined Pill via Pre-Register + Reconcile

**Date**: 2026-06-22
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Caesar

## Context

The monitor view (`/overlay/[id]/view`) lets a streamer send one message to every connected platform at once. Each platform echoes the message back through `chat:raw`, so the message-processor would emit N copies; the feature collapses them into one message carrying a combined multi-platform pill (`UnifiedChatMessage.Platforms`, rendered by `PlatformGlyphs`).

The original implementation pre-registered the *intended* platform set and used it verbatim as the pill. When a platform's send failed (the reported bug: Twitch sent, YouTube failed on live-chat-ID discovery), the pill still showed both platforms — it reflected intent, not delivery. The badge must reflect the platforms that actually received the message.

## Decision

Pre-register the intended set before fan-out (so any fast echo is recognised and never published standalone), then reconcile to the success set once send outcomes are known.

Alternatives considered:

- **Register only after sends complete**: on the happy path the first echo races ahead and is published standalone before registration, then re-published combined — duplicates. Rejected.
- **Processor waits/polls for a finalized set**: adds polling + latency to the hot path. Rejected as overkill.
- **auth-service authors the combined message directly and suppresses echoes**: duplicates the message-processor's enrichment and fan-out mapping. Rejected as too invasive.

The decision logic is the pure function `decideSendAllPill` (unit-tested independently of Redis):

- **All intended platforms succeed** → no change.
- **≥2 succeed, some fail** → rewrite survivors to the success set under the same group id; delete failed platforms' keys.
- **<2 succeed** → delete the whole group so the lone echo renders as an ordinary single-platform message (the frontend gates the combined pill on `platforms.length > 1`).

Because send failures are fast (not-live / quota / 4xx) and reconcile runs immediately after the send loop, it wins the race against the echo round-trip in all realistic cases.

## Consequences

- The pill lists exactly the platforms that received the message; per-platform `error_kind` is classified like the single-send path (`sendResultErrorKind`), so a YouTube "not live" surfaces as `stream_offline` rather than a blanket `send_failed`.
- Happy-path collapse, dedup and no-loss guarantees are unchanged.
- Residual race: one platform's echo processed *before* reconcile while another platform is still *slowly* failing (e.g. a multi-second timeout) can publish with the stale pre-registered set. Not observed for the common fast-fail paths; mitigation is keeping platform sends fast-failing.

## Implementation

- `services/auth-service/handlers/chat_send.go` — `decideSendAllPill`, `sendResultErrorKind`, `writeSendAllKeys`, `deleteSendAllKeys`, reconciled `handleStreamerSendToAll`.
- Tests: `services/auth-service/handlers/chat_send_sendall_test.go`.
- `shared/sendall/sendall.go` — unchanged key/registration contract.
- `services/message-processor/` — unchanged; consumes `Registration.Platforms`.

## Related

- [ADR-0002](./0002-redis-streams-pubsub.md) — the echo transport this dedup rides on.
- [ADR-0025](./0025-innertube-livechatid-cache.md) — resolves the YouTube live-chat-ID gap behind the original report.
- Numbering: ADR-0021/0022 live in the caesar-deployment repo; ADR numbers are shared across both repos.
