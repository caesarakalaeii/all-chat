# ADR-0058: TikTok transport tiers by entitlement

**Date**: 2026-09-17
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

TikTok chat ingest has one primary transport: the listener's own Node
WebSocket, signed by the self-hosted signer (ADR-0052) and pinned to the
capture's proxy lane. Lab validation on 2026-09-16 showed it holds sustained
streams, but its connect handshake is exposed to TikTok session scoring —
the "Unexpected server response: 200" flap (transport plan, phase 1). Fast
retries clear most flaps; the ones that do not clear currently park the room
in escalating error backoff with **no delivery at all** for the duration.

The signer already keeps a real viewer tab per captured room — a browser on
the streamer's live page whose own WebSocket receives the same PushFrames.
Phase 2 tapped that tab and relayed its frames over SSE, but only as a
read-only canary for a fixed room set: the frames were counted, never
delivered.

## Decision

**A second transport tier, premium-only, delivered by the signer's viewer-tab
relay.** When a room's primary connect exhausts its flap retry budget and the
room's streamer is premium, the listener switches that room's delivery to the
relay: frames are decoded with the connector's own schemas and replayed into
the room's `TikTokLiveConnection` via `processProtoMessageFetchResult`, so
every chat/gift/social/envelope handler, the dedup layer and the heartbeat
monitor run exactly as on the primary path. One decoder on both sides means a
delivery gap is a wire gap.

Entitlement is `users.is_premium` — the materialized flag the payment service
keeps current (ADR-0018/0027) and the moderation write-path already trusts —
reached through overlay ownership (`overlay_chat_sources` → `overlays` →
`users`), with a short TTL cache and a fail-closed answer.

Premium-only is a capacity decision, not a product one: a viewer tab is
roughly 150MB of renderer in the signer pod, so the tier is reserved for
entitled streamers. A non-premium room keeps today's behaviour (error backoff,
poller retry) unchanged.

Demotion deviates from the plan's first sketch ("demote after X hours of
primary-tier health"). While the fallback is delivering, the primary is not
connected, so its health is unobservable — the sketched condition can never be
evaluated honestly. A fallback stint ends instead on: the room's stream ends
(relay `ws_closed`), the tab dies (`tap_error`), the signer refuses the room
(404 — redeployed without the gate), or a max-duration ceiling
(`TIKTOK_FALLBACK_MAX_DURATION_MS`, default 6h). In every case the room
returns to the poller and retries the primary WS with a fresh flap budget.

## Enforcement

Two independent switches, both off by default:

- `SIGNER_RELAY_FALLBACK=on` (signer): opens `GET /v1/stream/:username` to
  any room with a warm tab, not just the canary set.
- `TIKTOK_PREMIUM_FALLBACK=on` (listener): arms the promotion path at
  flap exhaustion.

Either alone changes nothing observable.

Metrics: `tiktok_fallback_promotions_total{username,outcome}` (promoted /
not_premium / relay_unavailable) and `tiktok_fallback_deliveries_total
{username,outcome}` (delivered / no_messages / decode_failure /
replay_error).

## Consequences

- A premium room's worst case during a flap wall changes from "no messages
  until the flap clears" to "delivery continues on the relay".
- The signer pod carries one pinned tab per fallback room for the stint's
  duration; the max-duration ceiling bounds that exposure.
- Replay duplication between tiers is handled by the existing message
  deduplication layer (same `msgId` keys on both transports).
- The fallback's frames flow through the heartbeat monitor's
  `recordMessage`, so a silently dead relay is caught by the same
  timeout that catches a silently dead WS.
- Phase 2's canary decode was rebuilt while building this tier: the
  consumer called the async `deserializeWebSocketMessage` without awaiting
  it, so every relay frame had counted as an ack and `method_set` /
  `decode_failure_rate` divergences could never fire. The regression test
  for that now feeds a real wire-format PushFrame and asserts the decoded
  method lands in the window.
