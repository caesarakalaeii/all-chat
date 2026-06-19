# ADR-0017: Two-Phase Deprecation of the Twitch IRC Listener

**Date**: 2026-06-20
**Status**: Accepted
**Deciders**: Engineering team

---

## Context and Problem Statement

The Twitch IRC listener (`services/twitch-listener`) is being retired in favour of the EventSub listener (`services/twitch-eventsub-listener`). ADR-0015 already makes EventSub the primary chat path and IRC the universal fallback via per-channel ownership claims. The remaining problem is the *cutover*: a streamer's source only moves from IRC to EventSub when they (re-)complete the Twitch add-source consent. We cannot silently kill IRC — channels still served only by IRC would lose chat with no warning and no obvious fix. We need a controlled, reversible rollout that informs affected users in-product before chat stops, and a hard switch to actually stop serving once the grace period ends.

## Decision Drivers

- **No silent chat loss**: users must be told *in their overlay* before IRC stops, with a concrete action ("re-add your Twitch source").
- **Reversible, no-code rollout**: operators must be able to advance and roll back phases without code changes.
- **Reuse existing infrastructure**: the system already has a system-event pipeline (`token_expiration_warning`, `source_permission_error`) that renders in the overlay activity feed. Do not invent a new delivery channel.
- **No duplicate spam across replicas**: the listener is horizontally scaled; a notice must be sent once per connected source, not once per replica.
- **Safe default**: a misconfigured gate must never accidentally stop the listener from serving chat.

## Considered Options

1. **Hard cutover on a date** — flip EventSub on, delete the IRC listener.
   - ✅ Simplest to ship.
   - ❌ Any channel still on IRC loses chat instantly with no warning; no rollback; support load.
   - **Rejected**: violates the no-silent-loss driver.

2. **External email/dashboard banner campaign only** — notify out of band, then hard cutover.
   - ✅ Reaches users outside the overlay.
   - ❌ Low in-context signal; streamers live in the overlay, not their inbox; still a hard cutover at the end.
   - **Rejected** as the sole mechanism (may complement it).

3. **Two-phase env-var gate with in-overlay migration notices (chosen)** — a single `TWITCH_IRC_DEPRECATION_MODE` env var with `off` → `warn` → `enforce`. `warn` keeps serving chat and publishes a `listener_deprecation_notice` system event to every connected source on a fixed interval; `enforce` stops the listener from joining any channel.
   - ✅ Informs users exactly where they look; reuses the system-event pipeline end-to-end; reversible per phase by changing one env var; notices are scoped to connected sources (dedup via leadership-by-join).
   - ❌ Env-var change requires a pod restart to take effect (acceptable — phase changes are rare, deliberate operator actions); `enforce` reached without a prior `warn` window would surprise users (operator runbook responsibility).
   - **Chosen**.

## Decision Outcome

**Chosen**: Option 3 — a two-phase deprecation gate driven by `TWITCH_IRC_DEPRECATION_MODE`.

- `off` (default): the listener behaves exactly as before.
- `warn`: the listener still joins channels and serves chat, **and** the channel manager runs a notice loop that, every `TWITCH_IRC_DEPRECATION_NOTICE_INTERVAL` (default 5m), emits one `listener_deprecation_notice` system event per `(overlay, connected channel)` onto the `chat:raw` Redis Stream. The message-processor `SystemNormalizer` normalizes it and the overlay renders it in the activity feed with a re-add call-to-action.
- `enforce`: `SyncChannels` empties the desired-channel set, so the normal PART path disconnects everything and no JOINs are issued. Chat via IRC stops; users restore it by re-adding their Twitch source, which routes them to EventSub (ADR-0015 / ADR-0016 semantics).

Notices are scoped to **connected** channels (the pod's `activeChans`), not the full desired set. Because a channel only appears in a pod's `activeChans` after that pod won its leadership lease and joined it (ADR-0007), this naturally sends exactly one notice per connected source across the whole replica set — no cross-replica coordination needed.

Unknown/empty `TWITCH_IRC_DEPRECATION_MODE` values resolve to `off` (fail-safe): a typo can never stop the listener from serving chat.

## Consequences

### Positive

- Operators run `warn` for a grace period, then `enforce`, then remove the listener — each step reversible by env var.
- Users are warned in the overlay, where they will see it, with a one-click path to migrate.
- Zero new delivery infrastructure: the notice rides the existing system-event pipeline.
- Notice fan-out is duplicate-free by construction (leadership-by-join).

### Negative

- Phase changes require a pod restart (env-var driven, by design — phases are deliberate, infrequent operator actions; not a runtime feature toggle like ADR-0008).
- Jumping straight to `enforce` skips the user-facing warning; the rollout runbook must sequence `warn` before `enforce`.
- A new system event type (`listener_deprecation_notice`) is added to the message-processor classifier/normalizer and the frontend event union.

## Implementation

- **Gate**: `services/twitch-listener/channels/deprecation.go` — `DeprecationMode` (`off`/`warn`/`enforce`), `ParseDeprecationMode`, `DeprecationConfig`, `DeprecationNoticePublisher` interface, `RedisNoticePublisher` (publishes to `chat:raw`).
- **Manager**: `services/twitch-listener/channels/manager.go` — `SetDeprecationConfig`; enforce gating in `SyncChannels`; `deprecationNoticeLoop` + `publishDeprecationNotices` started from `Start()` during `warn`.
- **Wiring**: `services/twitch-listener/cmd/main.go` — parses `TWITCH_IRC_DEPRECATION_MODE` and `TWITCH_IRC_DEPRECATION_NOTICE_INTERVAL`.
- **Pipeline**: `services/message-processor/normalizer/system_normalizer.go` + `classifier/tier.go` — `listener_deprecation_notice` (high tier, 60s).
- **Frontend**: `frontend/src/lib/types/message.ts`, `components/overlay/CompactEvent.tsx`, `lib/utils/overlayViewModel.ts` (system bucket), `app/overlay/[id]/page.tsx`, `styles/events.css`.

## Related Decisions

- ADR-0007: Leadership rebalancing — the leadership-by-join property that makes notice fan-out duplicate-free.
- ADR-0008: Feature gate infrastructure — *not* used here; deprecation phases are deliberate operator actions, not runtime user-facing toggles, so a restart-scoped env var is appropriate.
- ADR-0015: EventSub chat-ownership claim — the IRC↔EventSub partition this deprecation completes.
- ADR-0016: Per-link Twitch credentials — ensures every account type can re-add a Twitch source and land on EventSub.
