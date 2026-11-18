# ADR-0026: Two-Phase Deprecation of the Twitch IRC Listener

**Date**: 2026-06-20
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Engineering team

## Context

The Twitch IRC listener (`services/twitch-listener`) is retired in favour of the EventSub listener; ADR-0015 makes EventSub the primary chat path. The problem at decision time was the cutover: a source only moves from IRC to EventSub when the streamer re-completes the Twitch add-source consent, so IRC could not be killed silently — channels still served only by IRC would lose chat with no warning and no obvious fix. A hard cutover on a date violates the no-silent-loss requirement; an out-of-band email campaign has low in-context signal and still ends in a hard switch.

Production now runs `enforce`: the listener joins no channels and EventSub is the only Twitch chat path (`../caesar-deployment/apps/workloads/all-chat/twitch-listener-deployment.yaml:61-63`).

## Decision

A two-phase gate driven by the `TWITCH_IRC_DEPRECATION_MODE` env var:

- `off` (default): unchanged behaviour.
- `warn`: the listener still serves chat, and the channel manager publishes one `listener_deprecation_notice` system event per `(overlay, connected channel)` to the `chat:raw` Redis Stream every `TWITCH_IRC_DEPRECATION_NOTICE_INTERVAL` (default 5m). The message-processor normalizes it and the overlay renders it in the activity feed with a re-add call-to-action.
- `enforce`: `SyncChannels` empties the desired-channel set, so the normal PART path disconnects everything and no JOINs are issued. Users restore chat by re-adding their Twitch source, which routes them to EventSub (ADR-0015 / ADR-0016).

Notices are scoped to the pod's connected channels. Because a channel only appears there after that pod won its leadership lease and joined it (ADR-0007), exactly one notice goes out per connected source across the replica set with no cross-replica coordination. Unknown/empty values resolve to `off` (fail-safe): a typo can never stop the listener serving chat. Phase changes require a pod restart — phases are deliberate, infrequent operator actions, not a runtime toggle (this is why ADR-0008 feature gates are *not* used here).

## Consequences

- Each step is reversible by env var; users are warned where they look, with a one-click migration path; zero new delivery infrastructure (the notice rides the system-event pipeline).
- Jumping straight to `enforce` skips the user-facing warning; the rollout must sequence `warn` first.
- A new system event type (`listener_deprecation_notice`) lives in the message-processor classifier/normalizer and the frontend event union.

## Implementation

- `services/twitch-listener/channels/deprecation.go` — `DeprecationMode`, `ParseDeprecationMode`, `DeprecationConfig`, notice publishers.
- `services/twitch-listener/channels/manager.go` — `SetDeprecationConfig`, enforce gating in `SyncChannels`, the notice loop.
- `services/twitch-listener/cmd/main.go` — env wiring.
- `services/message-processor/normalizer/system_normalizer.go` + `classifier/tier.go` — the event type (high tier, 60s).
- Frontend: `frontend/src/lib/types/message.ts`, `components/overlay/CompactEvent.tsx`, `lib/utils/overlayViewModel.ts`, `components/overlay/EventContent.tsx`, `app/overlay/[id]/page.tsx`, `app/overlays/[id]/preview/embed/page.tsx`, `styles/events.css`.

## Related

- [ADR-0007](./0007-leadership-rebalancing.md) — leadership-by-join makes notice fan-out duplicate-free.
- [ADR-0015](./0015-eventsub-chat-ownership-claim.md) — the IRC↔EventSub partition this completes.
- [ADR-0016](./0016-linked-twitch-credentials.md) — every account type can re-add a Twitch source and land on EventSub.
