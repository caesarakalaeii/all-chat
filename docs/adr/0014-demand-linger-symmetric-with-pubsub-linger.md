# ADR-0014: Linger Upstream Capture Demand Symmetric with the Downstream Pub/Sub Linger

**Date**: 2026-06-02
**Status**: Accepted
**Revised**: 2026-11-03
**Deciders**: All-Chat Platform Team

## Context

Demand-gated listeners (twitch-eventsub chat, youtube) capture a channel's chat only while an overlay using it holds a live WebSocket. source-manager derives demand from the `overlay:connected:{overlay_id}` keys api-gateway maintains (ADR-0002). When the last WebSocket disconnected, api-gateway deleted that key after the disconnect grace period, source-manager dropped demand, and the listener tore down the subscription. With the IRC listener in enforce mode (ADR-0026) there is no fallback Twitch chat path, so that teardown closed the channel's chat entirely.

This was asymmetric with the downstream: the api-gateway pub/sub subscriber lingers `PUBSUB_LINGER_SECONDS` (default 5 minutes) after the last connection, buffering messages and replaying them on reconnect. But upstream capture had already stopped, so there was nothing to replay; an OBS browser-source reconnect (scene switch, source refresh) longer than the grace period silently and permanently dropped the chat in between (recurring, e.g. channel `caesarlp`).

Alternatives rejected: buffering the gap at the listener (EventSub/YouTube never redeliver after a teardown, so there is nothing to buffer); shortening the downstream linger to match (wrong direction, worsens every reconnect); eager drop with reconcile re-add (with a lingering key the next reconcile re-adds demand, a flap that still loses chat while churning subscriptions).

## Decision

Make the two windows symmetric, governed by one knob. On disconnect, api-gateway sets `overlay:connected:{id}` TTL to the linger window (from `PUBSUB_LINGER_SECONDS`, default 5 minutes; `0` reverts to immediate deletion) instead of deleting it. source-manager no longer drops demand on the `disconnected` event; demand releases only when the key expires, detected by the periodic reconcile. A reconnect within the window refreshes the key to the full connection TTL, so capture is continuous and the downstream replay has messages to replay.

Cost: a gone overlay keeps its upstream subscription up to the linger window, the same bounded cost the downstream linger already accepts. Note: the `overlay:connected:*` key count now includes recently-disconnected overlays; read it as "connected or recently connected".

## Implementation

- `services/api-gateway/websocket/manager.go`: `disconnectLingerTTL` from `PUBSUB_LINGER_SECONDS`; `publishConnectionEvent` lingers the key on `disconnected`, deletes when TTL is 0.
- `services/source-manager/demand/subscriber.go`: the `disconnected` branch is a no-op (reconcile-driven release).
- Tests: `manager_linger_test.go`, `demand/subscriber_test.go` (`TestOverlayDemandSubscriber_Disconnected`).

Related: ADR-0002 (demand keys and `source:demand`), ADR-0009 (downstream resilience this extends upstream), ADR-0007 (leadership over the demand-gated channels).
