# ADR-0032: Source-Liveness Heartbeat Contract and Overlay Self-Heal

**Date**: 2026-07-13
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

`overlay_chat_sources.is_active` is the on/off switch for a chat source across the whole pipeline: the message-processor routes a message only to overlays where the matching source is active, listeners scope their upstream work to active sources, and the admin panel shows it as the live polling state. The source-manager cleanup job marks a source inactive once its `updated_at` is older than the 24h stale threshold, which exists to stop upstream work (YouTube polling quota, listener load) for overlays nobody watches. That is correct only if every actively-used source stays fresh, i.e. only if each listener heartbeats. Migration 059 deliberately scopes the `chat_source_changes` NOTIFY trigger so that an `updated_at`-only heartbeat write does not fire a notification (avoiding demand-refresh storms and flapping).

The contract was incomplete for Twitch. The YouTube (InnerTube) and Kick listeners heartbeat via source-manager's `ActivateSource` every poll cycle, but the Twitch IRC listener (formerly the Twitch heartbeat) had been put into enforce mode by ADR-0026 and joins nothing, and the Twitch EventSub listener only ever read the table — it never heartbeated. Net effect (production incident, 2026-07-13): a Twitch source on a 24/7-open OBS overlay had nothing refreshing its `updated_at`, so after 24h the cleanup job deactivated it and the message-processor silently dropped all Twitch messages for that overlay. The client never recovered: the WebSocket transport stayed healthy (20s pongs plus other sources' frames kept the silence watchdog satisfied), so only a manual page refresh — which re-runs the gateway's connect-time `ActivateSourcesForOverlay` — restored chat; the cycle then repeated every ~24h.

Two independent gaps: an actively-watched source can be deactivated because not every listener heartbeats, and a per-source content stop is invisible to a client whose transport is healthy, so it cannot self-heal.

## Decision

1. **Listener-agnostic heartbeat at the API gateway (primary).** On each 2-minute heartbeat tick the gateway, which already refreshes `overlays.last_connected_at` for connected overlays, also refreshes `overlay_chat_sources.updated_at` for those overlays' active sources (`bumpActiveSourcesUpdatedAt`, `updated_at`-only). Any actively-watched source is heartbeated here every 2 minutes and can never cross the 24h threshold, for all platforms and independent of listener behaviour.
2. **EventSub listener self-heartbeat (defense in depth).** The Twitch EventSub listener heartbeats the `updated_at` of every source it holds a live `channel.chat.message` subscription for, on each sync tick, leader-gated (`channels/manager.go` `heartbeatActiveSources`) — bringing Twitch in line with the YouTube/Kick contract migration 059 assumes.
3. **Cleanup job unchanged.** It keeps keying staleness off `updated_at`, which is now correct, and migration 059's guarantee that `updated_at`-only writes don't notify keeps both heartbeats storm-free.
4. **Client self-heal on source recovery.** `useOverlayStream` / `overlayStreamCore.isSourceRecovery` treats a `platform_status` transition from a known-down state (`offline`/`error`/`paused`/`reconnecting`) back to `connected` as a signal that messages may have been silently missed, and forces a debounced `?since=` replay reconnect (`RECOVERY_REPLAY_COOLDOWN_MS`) to backfill. A first-ever `connected` is deliberately not a recovery, so the initial connect does not trigger a spurious reconnect.

## Consequences

- An overlay left open indefinitely keeps its sources active while connected; the reported 24h dropout cannot recur. Abandoned overlays are still reaped: once the last connection is gone nothing heartbeats the source and the 24h cleanup deactivates it.
- If a future change makes an `is_active` flip part of the heartbeat path, migration 059's no-notify guarantee must be re-checked.
- The self-heal depends on listeners publishing a `platform_status` `connected` event when a source recovers; it is a best-effort backfill bounded by the gateway replay buffer, complementing (not replacing) the transport-silence watchdog.
- The client watchdog remains content-blind (it cannot distinguish "source quiet" from "source dead" without a status event); heartbeats remove the cause and status-driven replay covers recovery.

## Implementation

- Gateway heartbeat: `services/api-gateway/websocket/manager.go` (`refreshConnectionTTLs`, `bumpActiveSourcesUpdatedAt`).
- EventSub heartbeat: `services/twitch-eventsub-listener/channels/manager.go`.
- Client replay: the overlay stream hook and core (`useOverlayStream`, `overlayStreamCore.isSourceRecovery`).
- Migration 059 (`059_scope_source_change_notify.sql`) defines the NOTIFY scoping.

## Related

- [ADR-0007](./0007-leadership-rebalancing.md) — leadership; [ADR-0026](./0026-twitch-irc-listener-deprecation.md) — removed the IRC listener's heartbeat role.
