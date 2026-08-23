# Twitch Moderation & AutoMod Events

## Overview

All-Chat can show a Twitch channel's **moderator action feed** and its **AutoMod
held-message queue** in the monitor view's Activity & Events panel, live. It is a
read-only feed: All-Chat never bans, unbans, approves or denies anything on the
strength of these subscriptions.

This complements [message deletion](./message-deletion.md), which reports the same
channel's moderation _effects_ (a message vanished, a user's history was wiped, chat
was cleared) but carries no acting moderator. This feature is where the name comes
from.

The design decisions — and one wrong turn worth knowing about — are in
[ADR-0054](../adr/0054-twitch-moderation-and-automod-event-feed.md).

**Platform**: Twitch only. No other platform All-Chat ingests exposes a
moderator-action feed or a held-message queue.

**Latency**: Twitch webhook delivery, so sub-second in practice.

**Persistence**: none. The feed is live-only, capped at 200 rows in the browser, and
gone on reload. There is no table, no migration and no history endpoint.

**Opt-in**: yes, and separate from the moderation write-path grant. A streamer who
enabled moderation before this shipped still has to consent here.

## What a streamer sees

In the monitor view (`/overlay/{id}/view`), the right-hand **Activity & Events**
panel merges audience events, system notices and moderation rows into one newest-first
list. A moderation row shows the local time, an icon, a `mod` badge, and a one-line
summary:

| Row              | Example text                                                                                                        |
| ---------------- | ------------------------------------------------------------------------------------------------------------------- |
| Timeout          | `Timed out someviewer for 600s by somemod`                                                                          |
| Ban              | `Banned someviewer by somemod`                                                                                      |
| Message delete   | `Message deleted by somemod`                                                                                        |
| Chat cleared     | `Chat cleared by somemod`                                                                                           |
| AutoMod hold     | `AutoMod held a message from someviewer (aggressive)` plus a `held` badge and the held text quoted on a second line |
| AutoMod resolved | the same row, its badge now `approved` / `denied` / `expired`, and `AutoMod hold approved by somemod`               |
| Anything else    | the frame's own words: moderator, Twitch's action name, target — e.g. `somemod warn someviewer`                     |

The last row is not a fallback for bugs. Twitch adds moderator actions over time and
All-Chat passes the action name through verbatim, so an action this build has never
seen still produces a visible row rather than disappearing.

Rows only ever appear for the overlay **owner**, in the authenticated monitor. They
never reach an OBS browser source, a viewer participate tab, or the public overlay
render — and they raise no on-stream alert (the classifier gives them duration 0).

### Turning it on

The panel is empty until the streamer grants the Twitch mod-log scopes. While the
overlay has a Twitch source and the viewer is its owner, the monitor shows a banner
above the panels with an **Info** icon:

> Show Twitch moderation actions and AutoMod holds in this activity feed. Twitch
> requires an AutoMod "manage" permission to send us held messages at all — All-Chat
> only reads them; there are no approve/deny buttons yet.
>
> **Show moderation & AutoMod events**

Clicking that redirects to Twitch consent via
`GET /api/v1/auth/twitch/moderation/{overlay_id}?actions=modlog`
(`moderationApi.getTwitchModLogConsentUrl`). Nine scopes are requested — the eight
`moderator:read:*` scopes `channel.moderate` v2 needs, plus
`moderator:manage:automod` — unioned with the scopes already granted for Twitch, so
the resulting token is always a superset of the stored grant.

The wording about the "manage" permission is not padding. A write scope on a
read-only feature looks like a mistake and gets declined; Twitch requires it to
create an `automod.message.hold` subscription and offers no read-only alternative.
See ADR-0054 for why it is requested now rather than in a second consent round when
Approve/Deny lands.

The banner has no "already granted" state: `GET /capabilities` carries no flag for
this grant, so the CTA stays visible rather than pretending to know. Re-consenting is
harmless.

## End-to-end trace

Every hop, with the file that owns it.

1. **Twitch → webhook.** `POST /webhooks/eventsub` on `twitch-eventsub-listener`,
   signature-verified, then dispatched by `routeEvent` in
   `services/twitch-eventsub-listener/webhooks/handler.go`. Three cases matter here:
   `channel.moderate`, `automod.message.hold`, `automod.message.update`.
2. **Handler → wire format.** `handleChannelModerate` → `buildModerationAction`,
   `handleAutoModHold` → `buildAutoModHold`, `handleAutoModUpdate` →
   `buildAutoModResolution` (same file). Each produces a
   `RawChatMessage{EventType: "mod_action"}` with the specific action in
   `EventData["action"]`. Payload structs live in
   `services/twitch-eventsub-listener/eventsub/types.go`.
3. **`chat:raw`.** `publisher.Publish`
   (`services/twitch-eventsub-listener/publisher/stream_publisher.go`) XADDs to the
   `chat:raw` Redis stream through the shared ring-buffered publisher, exactly like
   chat.
4. **Per-overlay filter.** `message-processor` consumes `chat:raw`
   (`services/message-processor/consumer/stream_consumer.go`) and asks
   `EventFilter.IsEventEnabled` whether this overlay wants the event
   (`services/message-processor/filter/event_filter.go`). `mapEventTypeToColumn`
   answers `columnAlwaysEnabled` for `mod_action` — a moderator's own audit log is
   not toggleable, so this costs no database round-trip.
5. **Normalization.** `TwitchNormalizer.NormalizeEvent`
   (`services/message-processor/normalizer/twitch_normalizer.go`) builds the
   `UnifiedChatMessage`. `EventInfo.Metadata` is `raw.EventData` passed through
   unchanged — that is why the key table below is the frontend's contract too.
   `classifier.ClassifyEvent` (`services/message-processor/classifier/tier.go`)
   returns tier `low`, duration **0**: no on-stream alert.
6. **`overlay:{id}` pub/sub.** `publisher/pubsub_publisher.go` publishes to
   `overlay:{overlayID}`.
7. **api-gateway owner-only broadcast.** The subscriber in
   `services/api-gateway/cmd/main.go` decodes the event type, and
   `overlayBroadcastFilter` returns `BroadcastFilter{OwnerOnly: true}` for
   `mod_action`. `Pool.BroadcastFiltered`
   (`services/api-gateway/websocket/pool.go`) then skips every socket that is a
   viewer socket or has not proved ownership. `isOwner` is set in
   `services/api-gateway/handlers/websocket.go` only after a presented JWT has
   cleared validation, the logout-revocation blacklist and `VerifyOverlayOwnership`.
   `shouldBufferForReplay` also refuses the frame, keeping it out of the chat replay
   buffer — that buffer is replayed to every socket on connect, anonymous ones
   included.
8. **Client classification.** `classifyEnvelope`
   (`frontend/src/lib/utils/overlayStreamCore.ts`) sees a `chat_message` envelope
   whose `data.event.type === 'mod_action'` and returns `{ kind: 'modAction' }` —
   never a feed item. `useOverlayStream`
   (`frontend/src/hooks/useOverlayStream.ts`) advances the replay watermark by
   wall-clock (these frames carry no message timestamp of their own) and calls
   `onModAction(metadata, 'live')`.
9. **View model.** `toModActionEntry`
   (`frontend/src/lib/utils/overlayViewModel.ts`) turns the untyped metadata map into
   a `ModEntryData`, and `mergeAutoModResolution` appends it — folding a resolution
   into the hold it closes.
10. **Render.** `ActivityPanel`
    (`frontend/src/components/overlay/ActivityPanel.tsx`) merges the moderation log
    with events and system notices into one newest-first list; `ModRow` renders the
    icon, badges, summary line and held text.

Note that step 8 always passes `'live'`: mod frames are never in the replay buffer,
so a reconnecting monitor starts its moderation log empty rather than replaying held
text.

## `EventData` keys

The full wire contract, read off the three builders in
`services/twitch-eventsub-listener/webhooks/handler.go`. Logins are lowercased.

| Key                | Type         | Set by                                           | Always present?                                                                                                                                                                                  |
| ------------------ | ------------ | ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `action`           | string       | all three                                        | **Yes.** Twitch's own action name for `channel.moderate`; the literal `automod_hold` / `automod_resolved` for the two AutoMod feeds                                                              |
| `moderator_id`     | string       | `channel.moderate`, `automod.message.update`     | Optional — omitted when Twitch sends no moderator id, which is every `expired` AutoMod resolution (nobody acted)                                                                                 |
| `moderator_login`  | string       | `channel.moderate`, `automod.message.update`     | Optional, same condition                                                                                                                                                                         |
| `target_user_id`   | string       | all three                                        | Optional on `channel.moderate` (absent for actions with no user target, e.g. `clear`, `slow`, `automod_terms`, and for any action All-Chat does not model). Always set by both AutoMod feeds     |
| `target_login`     | string       | all three                                        | Same as `target_user_id`                                                                                                                                                                         |
| `reason`           | string       | `channel.moderate`                               | Optional — only when the action carries one and it is non-empty (`timeout`, `ban`, `warn`; Twitch does not require a reason)                                                                     |
| `ban_duration`     | int, seconds | `channel.moderate`                               | Optional — **timeout only**. Twitch sends an absolute `expires_at`; `timeoutSeconds` converts it to remaining whole seconds, floored at 1 so a nearly-elapsed timeout does not read as permanent |
| `held_message_id`  | string       | `automod.message.hold`, `automod.message.update` | Always on both. The join key; byte-identical across the pair                                                                                                                                     |
| `held_text`        | string       | `automod.message.hold`                           | Always on a hold, never on a resolution. **This is the message Twitch withheld from chat** — the reason the frame is owner-only                                                                  |
| `automod_category` | string       | `automod.message.hold`                           | Always on a hold (`"aggressive"`, `"sexual"`, …). Not re-sent on the resolution                                                                                                                  |
| `automod_level`    | int, 1-4     | `automod.message.hold`                           | Always on a hold; AutoMod's severity. Not re-sent on the resolution                                                                                                                              |
| `resolution`       | string       | `automod.message.update`                         | Always on a resolution: `approved`, `denied` or `expired`                                                                                                                                        |
| `resolved_by`      | string       | `automod.message.update`                         | Always present, but **empty when the hold expired** — nobody acted. The frontend treats an empty string as absent                                                                                |

No moderator keys are set on a hold at all: AutoMod is not a person, and a
placeholder there would later read as a real moderator having acted.

`EventData` is passed through verbatim to `event.metadata` on the wire, so these are
exactly the keys a client reads.

## Joining a hold to its resolution

A hold and its outcome arrive as two separate webhooks, possibly minutes apart, and
must render as **one** row — two would double-count them and make "how many are still
waiting" unreadable.

`mergeAutoModResolution` (`frontend/src/lib/utils/overlayViewModel.ts`) does it:

1. An entry whose `action` is not `automod_resolved`, or which carries no
   `heldMessageId`, is simply appended.
2. Otherwise the log is searched for an existing entry with the same
   `heldMessageId`.
3. On a hit, that entry is replaced in place with `resolution` and `resolvedBy`
   copied onto it. **Its original `at` is kept**, so the row does not jump to the top
   of the panel when the resolution lands.
4. On a miss — the hold was never seen, e.g. it predates this browser session — the
   resolution is appended as its own row. It renders as an AutoMod row with an
   outcome badge and no held text, which is the honest rendering of what is known.

`held_message_id` must be byte-identical between the two frames or step 2 fails and
the panel shows a stale `held` row forever. Both builders take it from Twitch's
`message_id` unmodified for that reason.

## Troubleshooting

### The streamer granted nothing and sees nothing

Expected. There is no error anywhere, by design — an ungranted opt-in is not a fault.
What you will find:

- **In the listener**, at **Info** level, once per subscription type per channel
  sync: `Moderation-log events require re-authentication with the mod-log scopes`
  with `broadcaster_id` and `type`. Twitch answers the subscription-create call with
  a 403; `isScopeError` (`services/twitch-eventsub-listener/cmd/main.go`) recognises
  it and the channel's `scope_errors` count in the following
  `EventSub subscription sync complete` line goes up by three.
- **Not** a `Failed to subscribe to moderation-log event` warning. That line means a
  real failure (network, malformed request) and is a different problem.
- **Nothing in the frontend.** The panel simply has no mod rows, and the consent
  banner is still offered.

Fix: click **Show moderation & AutoMod events** in the monitor and complete Twitch
consent. Then read the next section, because that is usually not enough on its own.

### The streamer granted it and _still_ sees nothing

This is the common report, and it is ADR-0030's known limitation, not a bug in this
feature. The three subscriptions are created in the listener's per-channel
`subscribe` action, which fires **once, when a channel is first tracked** — exactly
like subs, bits and follows. A grant made after the channel is already tracked
therefore takes effect on the next channel **(re)sync**:

- a leader change in `twitch-eventsub-listener`,
- a pod restart,
- or the channel being removed and re-added to the overlay.

The last is the one a streamer can do themselves. Until one of those happens, the
grant is stored and unused, and the listener keeps logging the Info line above
because its cached subscription state has not been re-derived.

### Moderation rows appear but never name a moderator

Check which subscription the row came from. `automod_hold` rows carry no moderator by
design. A `channel.moderate` row with no moderator means Twitch sent no moderator
identity on that event, which is rare; a `resolution: expired` row carries no
`resolved_by` because nobody acted.

If _deletion_ rows never name a moderator, that is the other feature: rows built from
`channel.chat.message_delete` and friends have no moderator field at all (see
[message deletion](./message-deletion.md)). The whole point of `channel.moderate` is
that its delete action does.

### A held message shows in the browser but should not be visible to X

It is not. Held text reaches only sockets that presented a valid JWT and passed
`VerifyOverlayOwnership`, and it is never written to the chat replay buffer. If you
have evidence of `held_text` on an anonymous socket, treat it as a security incident
and start at `overlayBroadcastFilter` and `shouldBufferForReplay` in
`services/api-gateway/cmd/main.go`.

### An action renders as a bare `moderator action target` line

Working as intended: Twitch shipped an action All-Chat does not model. The row is
built from whatever the frame carried. If the action deserves better rendering, add a
case to `modActionKind` and `modText` in the frontend and, if it has a target,
to `moderationTarget` in the listener — no new subscription, no new event type and no
filter change is involved.

## Not built

- **Approve / Deny.** There is no UI and no `POST /helix/moderation/automod/message`
  call in the codebase. `automod.message.update` already delivers both the resolution
  and the resolving actor, so adding the buttons later is additive — the row it
  produces is the row rendered today — and the scope it needs
  (`moderator:manage:automod`) is already granted.
- **Delegated-moderator access.** `modlog` is deliberately absent from
  `delegatableActions` in `services/auth-service/handlers/mod_consent.go`; see
  [ADR-0048](../adr/0048-delegated-overlay-moderators.md).
- **Any non-Twitch platform.**
- **History.** Nothing is persisted; a reload starts the log empty.

## References

- [ADR-0054: Twitch moderation and AutoMod event feed](../adr/0054-twitch-moderation-and-automod-event-feed.md)
- [ADR-0030: Twitch-native engagement mirroring](../adr/0030-twitch-native-engagement-mirroring.md) — the subscribe-on-first-track limitation
- [ADR-0048: Delegated overlay moderators](../adr/0048-delegated-overlay-moderators.md)
- [Message deletion feature](./message-deletion.md)
- [Twitch: channel.moderate v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-moderate-event)
- [Twitch: automod.message.hold v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-hold-v2-event)
- [Twitch: automod.message.update v2](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-update-v2-event)
