# ADR-0032: Source-Liveness Heartbeat Contract and Overlay Self-Heal

**Date**: 2026-07-13
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

`overlay_chat_sources.is_active` is the on/off switch for a chat source across the whole
pipeline: the message-processor routes a message only to overlays where the matching source
is `is_active = true` (`message-processor/router/overlay_router.go` `FindOverlaysForMessage`),
listeners scope their upstream work to active sources, and the admin panel shows it as the
live polling state.

The source-manager cleanup job (`source-manager/cleanup/cleanup.go`) marks a source
`is_active = false` once its `updated_at` is older than a 24h stale threshold. This exists to
stop upstream work (YouTube polling quota, listener load) for overlays nobody watches. It is
correct **only if every actively-used source keeps its `updated_at` fresh** — i.e. only if
each listener heartbeats. Migration 059 documents this contract and deliberately scopes the
`chat_source_changes` NOTIFY trigger so that an `updated_at`-only heartbeat write does **not**
fire a notification (which would cause demand-refresh storms / source flapping).

The contract was silently incomplete for Twitch. The YouTube (InnerTube) and Kick listeners
heartbeat via source-manager's `ActivateSource` every poll cycle, but:

- The **Twitch IRC listener** used to keep Twitch sources fresh, but it is now in enforce mode
  (ADR-0015 deprecation cutoff) and joins nothing (`suppressed_sources`, `status_updates: 0`).
- The **Twitch EventSub listener** only ever *read* `overlay_chat_sources`; it never
  heartbeated.

Net effect (production incident, 2026-07-13, streamer caesarlp, and at least one other overlay
the same hour): a Twitch source on an always-open (24/7 OBS) overlay had nothing refreshing its
`updated_at`, so after 24h the cleanup job flipped it to `is_active = false`. The
message-processor then **silently dropped all Twitch messages** for that overlay. The client
never recovered on its own, because the overlay WebSocket transport stayed perfectly alive: the
20s app-level heartbeat pongs and other sources' frames (e.g. YouTube) kept the client's silence
watchdog satisfied, so it never reconnected. Only a **manual page refresh** — which re-runs the
gateway's connect-time `ActivateSourcesForOverlay` — flipped the source back to active and
restored chat. The cycle then repeated every ~24h.

Two independent gaps combine here: (1) an actively-watched source can be deactivated because the
heartbeat contract is not honored by all listeners, and (2) a per-source content stop is
invisible to a client whose transport is healthy, so it cannot self-heal.

## Decision Drivers

- Never deactivate a source whose overlay is actively connected, regardless of which listener
  (if any) owns it — the failure must not depend on per-listener heartbeat coverage.
- Keep the cleanup job's intent (reap genuinely-abandoned overlays so upstream work / quota
  stops) and keep its `updated_at`-based mechanism (so migration 059's NOTIFY scoping holds).
- Restore chat without a manual refresh when a source silently stops and then recovers.
- No new NOTIFY storms: heartbeat writes must stay `updated_at`-only.

## Decision

**1. Listener-agnostic heartbeat at the API gateway (primary).**
The gateway already refreshes `overlays.last_connected_at` for every demand-bearing connected
overlay on each 2-minute heartbeat tick (`api-gateway/websocket/manager.go` `refreshConnectionTTLs`).
It now *also* refreshes `overlay_chat_sources.updated_at` for those overlays' active sources
(`bumpActiveSourcesUpdatedAt`, an `updated_at`-only write). Because chat delivery is demand-gated
on a live overlay connection, any actively-watched source is heartbeated here every 2 minutes and
can never cross the 24h stale threshold — for **all** platforms, independent of listener behavior.

**2. EventSub listener self-heartbeat (defense-in-depth + contract compliance).**
The Twitch EventSub listener now heartbeats the `updated_at` of every source it holds a live
`channel.chat.message` subscription for, on each sync tick, leader-gated
(`twitch-eventsub-listener/channels/manager.go` `heartbeatActiveSources`). This brings Twitch in
line with the YouTube/Kick heartbeat contract that migration 059 already assumes.

**3. Cleanup job unchanged.** It keeps keying staleness off `updated_at`. With the heartbeat
contract now actually honored (1 + 2), its `updated_at` mechanism is correct, and migration 059's
guarantee that `updated_at`-only writes don't notify keeps both heartbeats storm-free.

**4. Client self-heal on source recovery.**
The overlay client (`useOverlayStream` + `overlayStreamCore.isSourceRecovery`) treats a
`platform_status` transition from a known-down state (`offline`/`error`/`paused`/`reconnecting`)
back to `connected`, for a configured source, as a signal that it may have silently missed
messages. It forces a `?since=` replay reconnect (debounced to one per
`RECOVERY_REPLAY_COOLDOWN_MS` so a burst of recoveries or a flapping source collapses to a single
replay) to backfill the gap — no manual refresh. A first-ever `connected` (no prior status) is
deliberately not a recovery, so the initial connect does not trigger a spurious reconnect.

## Consequences

- An overlay left open indefinitely keeps its sources active for as long as it is connected; the
  reported 24h Twitch-chat dropout cannot recur.
- Abandoned overlays are still reaped: once the last connection is gone, nothing heartbeats the
  source and the 24h cleanup deactivates it as before, so YouTube quota / listener load still wind
  down.
- Heartbeat writes remain `updated_at`-only, so migration 059's NOTIFY scoping continues to
  prevent demand-refresh storms. If a future change makes an `is_active` flip part of the
  heartbeat path, that guarantee must be re-checked.
- The client self-heal depends on listeners publishing a `platform_status` `connected` event when
  a source recovers (the EventSub webhook handler and other listeners already do). It is a
  best-effort backfill bounded by the gateway replay buffer window; it complements, and does not
  replace, the transport-silence watchdog.
- The client watchdog is still content-blind (it cannot distinguish "source quiet" from "source
  dead" without a status event). A content-aware watchdog is intentionally out of scope; the
  heartbeat (1 + 2) removes the actual cause, and the status-driven replay (4) covers recovery.

## Related

- ADR-0007 (leadership rebalancing), ADR-0015 (Twitch IRC → EventSub migration), migration 059
  (`059_scope_source_change_notify.sql`).
- Confirmed root cause + production timeline: `.planning/debug/` and the incident on 2026-07-13.
