# ADR-0014: Linger Upstream Capture Demand Symmetric with the Downstream Pub/Sub Linger

**Date**: 2026-06-02
**Status**: Accepted
**Deciders**: All-Chat Platform Team

---

## Context and Problem Statement

Demand-gated listeners (twitch-eventsub chat, youtube) only capture a channel's chat while an
overlay that uses it has a live WebSocket. source-manager derives the demanded set from the
`overlay:connected:{overlay_id}` keys api-gateway maintains (see ADR-0002 and the source-manager
demand model). When the last WebSocket for an overlay disconnected, api-gateway **deleted** that key
immediately (after a 60s disconnect grace period) and source-manager eagerly dropped the overlay
from demand. The eventsub listener then tore down the `channel.chat.message` subscription.

This created an asymmetry with the downstream delivery path, which is explicitly built to tolerate
brief reconnects: the api-gateway pub/sub subscriber **lingers for `PUBSUB_LINGER_SECONDS` (default 5
minutes)** after the last connection drops, buffering incoming messages and replaying them when the
overlay reconnects. But because upstream capture stopped ~60s after disconnect, there was nothing to
buffer or replay — any overlay reconnect gap between the grace period and the linger window lost chat
**permanently**.

Symptom (recurring, e.g. channel `caesarlp`, twitch_id `67241623`): an OBS browser-source reconnect
(scene switch, source refresh, network blip) longer than the grace period silently dropped all chat
that arrived during the gap, defeating the entire buffer/replay design.

## Decision Drivers

- **End-to-end resilience must be symmetric.** A reconnect the gateway can replay for is only useful
  if the upstream listener actually captured the messages during the gap.
- **OBS reconnects are normal and frequent**, not exceptional — the system is already designed to
  absorb them downstream; upstream must match.
- **Keep the source-of-truth model from the demand rework** (key-derived demand + 15s reconcile) —
  do not reintroduce divergent in-memory state.
- **Bounded cost.** A genuinely-gone overlay must still release demand so idle channels stop costing
  Twitch/YouTube API work.
- **Reversible.** Operators must be able to revert to immediate teardown.

## Considered Options

1. **Linger the `overlay:connected` key on disconnect, symmetric with the pub/sub linger (chosen)**
   - On disconnect, api-gateway shortens the key's TTL to the linger window instead of deleting it;
     source-manager stops eagerly dropping demand on the `disconnected` event and lets the 15s
     reconcile release demand once the key expires.
   - ✅ Pros: Upstream capture survives exactly the reconnect window the downstream can replay; reuses
     the existing key-as-source-of-truth + reconcile model; one shared knob (`PUBSUB_LINGER_SECONDS`)
     governs both windows; `=0` reverts to immediate teardown.
   - ❌ Cons: A genuinely-gone overlay keeps its upstream subscription for up to the linger window
     (bounded, identical to the cost the downstream linger already accepts).

2. **Buffer upstream messages during the gap (ring buffer at the listener) instead of keeping the subscription**
   - ✅ Pros: No idle subscription cost.
   - ❌ Cons: EventSub/YouTube do not deliver missed messages after a subscription is torn down — the
     data simply never arrives, so there is nothing to buffer. Does not solve the problem.

3. **Shorten/remove the downstream linger to match the old 60s upstream teardown**
   - ✅ Pros: Symmetric.
   - ❌ Cons: Wrong direction — makes brief reconnects worse for every viewer; throws away working
     buffer/replay resilience (ADR-0009 lineage).

4. **Eager demand drop but re-add via reconcile**
   - ❌ Cons: With a lingering key, the next reconcile re-adds demand, producing a flap that tears the
     subscription down and back up within ~15s — still loses chat and churns subscriptions.

## Decision Outcome

Chosen: **Option 1.** On disconnect, api-gateway sets `overlay:connected:{id}` to a TTL equal to the
disconnect linger window (read from `PUBSUB_LINGER_SECONDS`, default 5 minutes, `0` disables and
reverts to immediate deletion) rather than deleting it. source-manager's `disconnected` handler no
longer drops demand; demand is released only when the key expires, detected by the periodic
reconcile. A reconnect within the window refreshes the key back to the full connection TTL, so
capture is continuous and the downstream replay has messages to replay.

## Consequences

- Brief overlay reconnects (up to the linger window) no longer lose chat; buffer/replay works end to
  end.
- Demand for a truly-gone overlay falls away within `PUBSUB_LINGER_SECONDS` + one reconcile interval
  (≤ ~5m15s by default) instead of ~grace period — an intentional, bounded trade.
- `overlay:connected:*` key count (used by stats) now counts overlays in their linger window as
  connected; treat it as "connected or recently-connected".
- The two windows are governed by one env var, keeping them symmetric by construction.

## Implementation

- `services/api-gateway/websocket/manager.go`: `disconnectLingerTTL` field, initialised from
  `PUBSUB_LINGER_SECONDS`; `publishConnectionEvent` lingers the key on `disconnected` (or deletes when
  the TTL is 0).
- `services/source-manager/demand/subscriber.go`: `handleConnectionEvent` `disconnected` branch is
  now a no-op (reconcile-driven release).
- Tests: `services/api-gateway/websocket/manager_linger_test.go`,
  `services/source-manager/demand/subscriber_test.go` (`TestOverlayDemandSubscriber_Disconnected`).

## Related Decisions

- ADR-0002 (Redis Streams + Pub/Sub) — the `overlay:connected` keys and `source:demand` channel.
- ADR-0009 (Ring Buffer Publisher) — the downstream resilience philosophy this extends upstream.
- ADR-0007 (Leadership rebalancing) — demand gates which channels a listener acquires leadership for.
