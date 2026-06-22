# ADR-0024: Streamer Send-to-All Combined Pill via Pre-Register + Reconcile

**Date**: 2026-06-22
**Status**: ✅ Accepted
**Deciders**: Caesar

## Context and Problem Statement

The monitor view (`/overlay/[id]/view`) lets a streamer send one chat message to every connected platform at once ("send to all", shipped in #470). Each platform echoes the streamer's own message back through `chat:raw`, so without intervention the message-processor would emit N copies — one per platform. The feature collapses them into a single message carrying a combined multi-platform pill (`UnifiedChatMessage.Platforms`, rendered by `PlatformGlyphs`).

The original implementation pre-registered the **intended** platform set before fan-out and used it verbatim as the pill. When a platform's send failed (the reported bug: Twitch sent, YouTube failed on live-chat-ID discovery), the pill still showed both platforms — it reflected intent, not delivery. The badge must instead reflect the platforms that actually received the message.

## Decision Drivers

- **Correctness**: the pill must list exactly the platforms a message reached (a platform echoes back only if its send succeeded).
- **No duplicates / no loss on the happy path**: all-success sends must collapse to one message, never a standalone-plus-combined pair, and never drop the streamer's own message.
- **Latency**: the streamer's own message should appear promptly; the processor must not block waiting for echoes that will never arrive (failed platforms never echo).
- **Minimal blast radius**: keep the pill-rendering data-driven (frontend already gates the combined pill on `platforms.length > 1`); avoid moving message enrichment out of the message-processor.

## Considered Options

1. **Pre-register the intended set, reconcile to the success set after sending** (chosen) — keep the pre-registration before fan-out (so any fast echo is recognised and never published standalone), then once outcomes are known rewrite the surviving keys to the success set (≥2 winners) or delete the whole group (<2 winners).
   - ✅ Happy path unchanged: full set pre-registered, zero duplicates, zero loss.
   - ✅ Failed platforms are excluded from the pill in essentially all real timings (failures are fast: not-live / quota / 4xx).
   - ❌ Narrow residual race (see Negative): a fast successful echo arriving before reconcile while another send is *slowly* failing.

2. **Send first, then register only successful platforms** — defer all registration until outcomes are known.
   - ✅ Pill is always exactly the success set.
   - ❌ Registration waits for the slowest send while the first platform's echo races ahead → on the **happy path** the early echo is published standalone and a later echo re-publishes combined: duplicates. Rejected.

3. **Processor waits/polls for a finalized set on first echo** — pre-register a group marker, finalize the success set after sending, have the processor briefly wait for finalization.
   - ✅ Fully race-free pill.
   - ❌ Adds polling + latency to the hot path and contradicts the "never wait for echoes" design intent. Rejected as overkill.

4. **auth-service publishes the combined message directly and suppresses all echoes** — author the unified message in auth-service.
   - ✅ Pill content fully decoupled from echo timing.
   - ❌ Duplicates the message-processor's enrichment (emotes/badges/cosmetics) and overlay-fan-out mapping in auth-service. Rejected as too invasive.

## Decision Outcome

**Chosen**: Option 1 — pre-register the intended set before fan-out, then reconcile to the actual success set.

**Rationale**: It preserves the proven happy-path behavior (one collapsed message, no duplicates, no loss) while making the pill honest about delivery. Reconciliation is cheap (a few Redis `SET`/`DEL`s) and, because send failures are fast and the reconcile runs immediately after the send loop, it wins the race against the platform→listener→processor echo round-trip in all realistic cases. The decision logic is a pure function (`decideSendAllPill`) so it is unit-tested independently of Redis.

Reconcile rules (`decideSendAllPill`):
- **All intended platforms succeed** → no change (pre-registered full set is correct).
- **≥2 succeed, some fail** → `writeSendAllKeys` with the success set: rewrite survivors to the reduced set under the same group id, delete failed platforms' keys.
- **<2 succeed** → `deleteSendAllKeys`: drop the whole group so the lone (or absent) echo renders as an ordinary single-platform message — matching the frontend's `platforms.length > 1` gate for the combined pill.

## Consequences

### Positive
- The combined pill lists exactly the platforms that received the message; a failed platform never appears.
- Per-platform `error_kind` in the send-to-all response is now classified like the single-send path (`sendResultErrorKind`), so a YouTube "not live" surfaces as `stream_offline` instead of a blanket `send_failed`.
- Happy-path collapse, dedup, and no-loss guarantees are unchanged.

### Negative
- **Residual race**: if one successful platform's echo is processed *before* reconcile while a different platform is still *slowly* failing (e.g. a multi-second HTTP timeout), the early echo can publish with the pre-registered (stale) set. This requires a slow failure concurrent with a fast echo and is not observed for the common fast-fail paths (not-live / quota / 4xx, which return in well under the echo round-trip). Mitigation: keep platform sends fast-failing; a future option is the processor-side finalize-wait (Option 3) if the race is ever observed in practice.
- Reconcile issues a few extra Redis ops per partial-failure send (negligible).

## Implementation

- **Files**:
  - `services/auth-service/handlers/chat_send.go` — `decideSendAllPill`, `sendResultErrorKind`, `writeSendAllKeys`, `deleteSendAllKeys`; `handleStreamerSendToAll` now pre-registers with a captured group id and reconciles after the send loop.
  - `services/auth-service/handlers/chat_send_sendall_test.go` — pure-function and miniredis-backed tests (regression guard for the reported bug).
  - `shared/sendall/sendall.go` — unchanged key/registration contract (shared writer/reader).
  - `services/message-processor/` — unchanged; consumes `Registration.Platforms` as before.
- **Configuration**: none.
- **Timeline**: 2026-06-22.

## Related Decisions

- [ADR-0002: Redis Streams + Pub/Sub Hybrid](./0002-redis-streams-pubsub.md) — the echo transport this dedup rides on.
- [ADR-0017: Chat Moderation Write-Path](./0017-chat-moderation-write-path.md) — the advanced-controls consent that also grants send scopes.
- Numbering: ADR-0021 and ADR-0022 live in the caesar-deployment repo (os-patching, alerting) and ADR-0023 is the YouTube quota monitor, so this all-chat ADR is 0024 — ADR numbers are shared across both repos.
- [ADR-0025: InnerTube Listener Caches activeLiveChatId](./0025-innertube-livechatid-cache.md) — resolves the YouTube live-chat-ID gap that caused the original report: the deployed InnerTube listener did not populate the `youtube:stream:state` cache, so YouTube sends fell to the unreliable `search.list` fallback and failed (making YouTube the platform that dropped out of the pill). With the cache now populated, YouTube sends from the monitor succeed.
