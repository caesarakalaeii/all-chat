# ADR-0046: Twitch chat notices (watch streaks, announcements) via channel.chat.notification

**Date**: 2026-08-01
**Status**: Accepted
**Deciders**: caesarakalaeii

---

## Context and Problem Statement

Twitch splits chat delivery across two EventSub subscriptions:

- **`channel.chat.message`** — "Any user sends a message to a channel's chat room."
- **`channel.chat.notification`** — "A notification for when an event that appears
  in chat has occurred."

All-Chat subscribed only to the first. `routeEvent` had no case for the second, so
had one ever arrived it would have hit the `default:` branch and been logged as
"Unhandled subscription type" and discarded — but none ever arrived, because no
subscription was created.

The consequence is worse than a missing decoration. Several notice types carry the
**chatter's own message text** in the notice payload (`message.text` + `fragments`),
and Twitch does not additionally send them as `channel.chat.message`:

- **`watch_streak`** — a returning viewer's first message of the stream, delivered
  with their watch-streak milestone. The message is inside the notice.
- **`announcement`** — the body of a `/announce` from the broadcaster or a mod.

So those messages were **silently dropped**: never published to `chat:raw`, never
rendered on any overlay, with no error anywhere in the pipeline. A viewer with a
watch streak would type in chat and simply not appear.

Two earlier decisions turned this from a gap into total loss:

1. **ADR-0015** made EventSub the primary chat path.
2. **ADR-0026** retired the IRC listener; production runs
   `TWITCH_IRC_DEPRECATION_MODE=enforce`, so `twitch-listener` joins no channels
   and serves no chat. EventSub is now the *only* Twitch chat path.

The IRC listener would not have helped anyway: `mapMsgIDToEventType` maps
`viewermilestone` (IRC's watch streak) and `announcement` to `"unknown"`, and
`ParseUserNotice` returns an error for unknown msg-ids — so those USERNOTICEs were
dropped there too, just noisily rather than silently.

Beyond the two message-bearing notices, the whole notice feed was missing:
`bits_badge_tier`, `charity_donation`, `pay_it_forward`, `gift_paid_upgrade`,
`prime_paid_upgrade`, `unraid`, `modiversary`, and every `shared_chat_*` variant.

The complication is that the notice feed **overlaps** subscriptions all-chat already
has: `sub`, `resub`, `sub_gift`, `community_sub_gift` and `raid` also appear as
notice types, while being delivered (with strictly richer data) by
`channel.subscribe`, `channel.subscription.message`, `channel.subscription.gift`
and `channel.raid`. Naively forwarding every notice would double-render every sub
and raid.

## Decision Drivers

- **No silently dropped chat.** A message a viewer typed must reach the overlay.
- **No double-rendering.** Adding a subscription must not duplicate events that
  already arrive on a dedicated one.
- **Notice-borne messages must render as chat, not as bare system text** — with
  their emotes, badges, colour and per-channel enrichment.
- **Forward compatibility.** Twitch adds notice types over time (`modiversary`
  post-dates the original enum). An unrecognised notice must degrade, not vanish.
- **Deletability.** A moderator can delete a watch-streak message; the overlay must
  be able to remove it like any other message.

## Considered Options

1. **Subscribe to `channel.chat.notification`, forward every notice.**
   - ✅ Nothing is lost.
   - ❌ Every sub, resub, gift and raid renders twice.
   - **Rejected.**

2. **Subscribe, and emit only the two message-bearing notices (`watch_streak`,
   `announcement`).**
   - ✅ Fixes the dropped-message bug with the smallest surface.
   - ❌ Leaves seven event types and all shared-chat variants dropped, so the same
     bug report recurs for charity donations, bits badges, pay-it-forward, etc.
   - **Rejected** as knowingly incomplete.

3. **Subscribe, and route by notice type: skip the types a dedicated subscription
   already delivers, map the rest onto all-chat's event taxonomy, emit anything
   unrecognised generically (chosen).**
   - ✅ Nothing is dropped and nothing duplicates; new Twitch notice types degrade
     to a visible generic event plus a log line rather than disappearing.
   - ❌ The skip-list is a closed set that must be revisited if all-chat ever drops
     one of the dedicated subscriptions (a notice would then go missing).
   - **Chosen.**

## Decision Outcome

**Chosen**: Option 3.

### Subscription lifecycle

`channel.chat.notification` is created and torn down **with** the chat
subscription. It shares the chat condition (`broadcaster_user_id == user_id`) and
authorization (`user:read:chat` + `user:bot`, broadcaster == chatter), so it joins
the existing `subscribe_chat` / `unsubscribe_chat` bundle alongside the three
moderation subscriptions. Creation is best-effort: a failure leaves chat working
and is retried on the next sync. Notice delivery also refreshes the EventSub chat
ownership claim and the "connected" indicator, exactly as a chat message does — a
delivered notice is equally proof the channel's chat is live.

Unlike `channel.chat.message`, a **revoked** notification subscription does not
release the chat ownership claim: losing notices does not mean chat is dead.

### Notice routing

`buildChatNotice` strips any `shared_chat_` prefix (the payload arrives under a
prefixed key too, e.g. `shared_chat_announcement`), then:

| Base notice type | Outcome |
|---|---|
| `sub`, `resub`, `sub_gift`, `community_sub_gift`, `raid` | **skipped** — already delivered by a dedicated subscription |
| `watch_streak` | event `watch_streak`, text = the viewer's message |
| `announcement` | event `announcement`, text = the announcement body |
| `bits_badge_tier` | event `bits` (parity with IRC's `bitsbadgetier`, reuses the bits classifier/toggle/CSS) |
| `gift_paid_upgrade`, `prime_paid_upgrade`, `pay_it_forward` | events of the same name |
| `unraid`, `charity_donation`, `modiversary` | events of the same name |
| anything else (incl. Twitch's own `unknown`) | event `twitch_notice`, text = `system_message`, logged at Info |

The skip-list is the EventSub twin of `twitch-listener`'s `isCoveredByEventSub`,
which does the same job for IRC USERNOTICEs.

Text selection is: the chatter's `message.text` when present, else Twitch's
`system_message`. That keeps message-bearing notices as real messages and stops
event-only notices from rendering blank.

### Chat parity for notice-borne messages

Tags are built by delegating to the **chat path's own** `buildChatTags`, via a
`ChatMessageEvent` view of the notice. A watch-streak message therefore gets
identical treatment to ordinary chat: first-party emote positions, badges (with the
prediction-badge version fix), colour, `room-id` (the enrichment key every
enricher uses), and shared-chat provenance tags. The native Twitch message id lands
in `Tags["id"]` and is registered in the message-ID registry, so a later
`channel.chat.message_delete` removes the row like any other message.

### Duplicate protection

Twitch documents the two subscriptions as disjoint, and the observed production
behaviour (watch-streak messages absent from overlays entirely) confirms notices are
not also sent as `channel.chat.message`. Rather than depend on that, the
message-processor's existing native-id dedup (ADR-0015) is **widened** from "Twitch
regular chat" to "any Twitch message carrying `Tags["id"]`, except deletions". Since
a notice and its hypothetical chat-message twin carry the same native id, the two
collapse to one rendered message in either arrival order. The dedup is a single
atomic Redis `SETNX` that already ran for every chat message, so this costs nothing
and removes the assumption entirely.

Deletion events are excluded and unaffected: they carry no `Tags` at all (the target
id lives in `EventData.target_msg_id`).

### Settings and rendering

`watch_streak` gets one new per-overlay toggle, `enable_twitch_watch_streaks`
(migration 079, default TRUE) — it fires once per returning viewer per stream, so
busy overlays need an off switch. The sub-adjacent conversions
(`gift_paid_upgrade`, `prime_paid_upgrade`, `pay_it_forward`) ride the existing
gift-sub toggle; `unraid` rides the existing raid toggle; `bits` rides the bits
toggle.

`announcement`, `charity_donation`, `modiversary` and `twitch_notice` are
deliberately **not** toggleable, marked with a `columnAlwaysEnabled` sentinel that
returns enabled without a database round-trip and without the "unknown event type"
warning. An announcement is chat content the broadcaster chose to highlight —
hiding it would hide chat.

The frontend adds icons, titles and CompactEvent labels for each new type. No CSS
work is required: the `.event-type-{type}` class is generated from `event.type`, and
the overlay's event partitioning is allowlist-free, so new types flow through
automatically.

## Consequences

### Positive

- Watch-streak and announcement messages reach overlays at all — the reported bug.
- Seven further Twitch event types and all shared-chat notice variants now render.
- Notice-borne messages render with full emote/badge/colour enrichment, and are
  moderatable (deletable) like ordinary chat.
- Future Twitch notice types degrade to a visible generic event plus an Info log,
  instead of disappearing.
- Native-id dedup now guarantees one rendered message per native Twitch message id
  regardless of which subscription delivered it.

### Negative

- One more EventSub subscription per chat-active channel (created with a user-scoped
  condition, so no additional subscription cost, but more state to reconcile).
- The skip-list couples this handler to the set of dedicated subscriptions: dropping
  `channel.raid` (say) without updating `noticeCoveredByDedicatedSubscription` would
  silently lose raids.
- `twitch-listener`'s IRC USERNOTICE parser still maps `viewermilestone` and
  `announcement` to `"unknown"`. Deliberately not fixed: IRC serves no channels in
  `enforce` mode and the service is slated for removal (ADR-0026). If IRC is ever
  reactivated, its watch streaks would still be dropped.

### Testing

Unit tests cover notice→event mapping, the skip-list, shared-chat prefix handling,
tag parity, per-type event data, the classifier tiers and the filter columns. A
cross-service pipeline test drives a **verbatim Twitch payload** through listener
conversion → the JSON wire format of `chat:raw` → the message-processor's event
normalizer, asserting the viewer's message and emotes survive to the unified
message.

A true end-to-end test is **not possible** for this event: a watch streak cannot be
triggered on demand — Twitch only emits it when a real viewer's multi-stream watch
history crosses a milestone, and no sandbox or CLI event exists to synthesize a chat
notice. The verbatim-payload pipeline test is the furthest upstream a test can
start.

## References

- [Twitch: channel.chat.notification event reference](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-chat-notification-event)
- ADR-0015: EventSub chat ownership claim (native-id dedup, IRC↔EventSub handoff)
- ADR-0026: Two-phase deprecation of the Twitch IRC listener
- ADR-0037: Twitch chat GIFs (the tag-parity pattern reused here)
