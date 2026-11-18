# ADR-0054: Twitch moderation and AutoMod event feed

**Date**: 2026-08-23
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

The `channel.chat.message_delete` / `clear_user_messages` / `clear`
subscriptions show only the *effect* of moderation, not the actor: a streamer
saw a viewer timed out but not by which mod, for how long, or why. And a
message AutoMod holds is never delivered as chat at all, so the streamer had to
sit in Twitch's own AutoMod queue.

## Decision

Three EventSub subscriptions, one internal event type, owner-only delivery.
`modlog` is deliberately absent from `delegatableActions`
(`services/auth-service/handlers/mod_consent.go`): the feed reads the
channel's own moderation history, which is the owner's record, and a volunteer
must never be asked for these scopes (ADR-0048).

- **Subscriptions** (`services/twitch-eventsub-listener/eventsub/subscription_manager.go`,
  `subscribeModeratorScoped`): `channel.moderate` v2 (the moderator-action
  feed, carrying `moderator_user_id` — the identity the deletion path lacks),
  `automod.message.hold` v2 (chatter, held text, category, severity) and
  `automod.message.update` v2 (resolution and the resolving moderator). Three
  disjoint feeds: dropping any one removes a capability. The condition sets
  both `broadcaster_user_id` and `moderator_user_id` to the streamer (Twitch
  answers 400 to a broadcaster-only condition). Do not read the feed's
  `automod_terms` action as "holds arrive here" — it is a term-list edit with
  no message in it; an earlier design nearly shipped on that mistake.
- **The scope**: `twitchModerationScopesByAction["modlog"]`
  (`services/auth-service/oauth/twitch.go`) holds eight `moderator:read:*`
  scopes plus `moderator:manage:automod`. That ninth is a write scope the code
  never exercises and there is deliberately no Approve/Deny UI — but Twitch
  requires it to create an `automod.message.hold` subscription at all and
  offers no read-only alternative. It is requested now, in one consent round
  (a second round is another banner and a population who never sees it), with
  copy explaining the "manage" name on a read-only feature. Consent URL:
  `moderationApi.getTwitchModLogConsentUrl`, reached via
  `/api/v1/auth/twitch/moderation/{overlay_id}`.
- **One event type**: everything publishes to `chat:raw` as
  `EventType: "mod_action"`, the action in `EventData["action"]` — Twitch's
  own string passed through verbatim (unknown actions must render as a row,
  never vanish; `modActionKind` in `frontend/src/lib/utils/overlayViewModel.ts`
  renders unrecognised ones as kind `action`), plus synthesized
  `automod_hold` / `automod_resolved`. One filter entry
  (`columnAlwaysEnabled` — a moderator cannot switch off their own audit log;
  no toggle, no migration), one frontend match string (`classifyEnvelope` in
  `frontend/src/core/overlayStreamCore.ts`). Classifier tier `low` with
  duration 0: these never reach the public OBS overlay.
- **Owner-only delivery** is the load-bearing security property: an
  `automod_hold` frame carries `held_text`, text nobody in chat ever saw, and
  the overlay WebSocket accepts anonymous OBS browser-source sockets
  (`HandleOverlayConnection`, `services/api-gateway/handlers/websocket.go`).
  Mod frames are broadcast only to sockets with a valid JWT, not blacklisted,
  that passed `VerifyOverlayOwnership` (`isOwner`, enforced by
  `Pool.BroadcastFiltered` via `BroadcastFilter.OwnerOnly`; ownership is never
  inferred from `user_id`). They are also excluded from the chat replay buffer
  (`shouldBufferForReplay` in `services/api-gateway/cmd/main.go`): the buffer
  replays to every socket on connect, so buffering held text would leak later,
  out of context. Classification and routing live in one function,
  `overlayBroadcastFilter`, so the caller cannot class a frame and then
  broadcast it unfiltered; undecodable frames fail open to "not a mod frame"
  (an undecodable frame cannot carry renderable `held_text`).

## Consequences

- The monitor answers "who did this"; AutoMod holds are visible with text,
  category and severity, folding into one row on resolution
  (`mergeAutoModResolution`).
- The consent requests nine scopes including an unused write scope — Twitch's
  design, but streamers see it as ours and a declined consent is the expected
  failure mode; mitigated only by copy.
- The feed is invisible until the next channel (re)sync (subscriptions are
  created by the listener's per-channel `subscribe` action, ADR-0030's known
  limitation), and a missing grant is silent by design (`isScopeError` logs at
  Info with a `scope_errors` count).
- Live-only: no persistence, capped at `MAX_MOD_LOG` rows in the browser;
  frames advance the replay watermark by wall-clock.
- Tests: `webhooks/handler_moderation_test.go` (wire format, unmodelled action
  pass-through, target extraction, absolute-`expires_at`, no moderator keys on
  a hold), `moderation_pipeline_test.go` (verbatim payloads), gateway pool and
  main tests pinning the owner-only filter and replay exclusion
  (`applyOverlaySocketFlags` / `ownershipVerified`), plus
  `overlayViewModel.test.ts` / `overlayStreamCore.test.ts`. No end-to-end test
  is possible: AutoMod must actually trip on a real message.

## References

- [channel.moderate v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-moderate-event),
  [automod.message.hold v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-hold-v2-event),
  [automod.message.update v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-update-v2-event)
- ADR-0012 (the ninth scope is measured against it), ADR-0017 (re-consent flow
  reused), ADR-0030 (subscribe-on-first-track), ADR-0046 (pass-through +
  `columnAlwaysEnabled` patterns), ADR-0048 (why not delegatable)
- [Twitch moderation events guide](../features/twitch-moderation-events.md),
  [Message deletion feature](../features/message-deletion.md)
