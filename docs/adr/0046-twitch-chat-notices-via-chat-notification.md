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
moderation subscriptions. Creation is best-effort: a failure leaves chat working.
Recovery is *not* a periodic retry — `reconcileChatLocked` only calls `subscribe_chat`
on the `want && !ChatActive` transition, so a channel whose notice subscription
failed or was revoked while chat stays active keeps working chat with no notices
until demand cycles (the last overlay using it disconnects and reconnects) or the pod
restarts. Acceptable because overlays disconnect routinely, but it means a
continuously-connected overlay can sit without notices for a long time; a periodic
reconcile of the companion subscriptions would close that gap.
Notice delivery also refreshes the EventSub chat
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
| `bits_badge_tier` | event `bits_badge_tier` (see below) |
| `gift_paid_upgrade`, `prime_paid_upgrade`, `pay_it_forward` | events of the same name |
| `unraid`, `charity_donation`, `modiversary` | events of the same name |
| anything else (incl. Twitch's own `unknown`) | event `twitch_notice`, text = `system_message`, logged at Info |

The skip-list is the EventSub twin of `twitch-listener`'s `isCoveredByEventSub`,
which does the same job for IRC USERNOTICEs.

Text selection is: the chatter's `message.text` when present, else Twitch's
`system_message`. That keeps message-bearing notices as real messages and stops
event-only notices from rendering blank.

**`bits_badge_tier` gets its own event type**, rather than folding into `bits` as
IRC's `bitsbadgetier` did. The IRC mapping renders a *lifetime badge unlock* as
"💎 Bits Cheered! / 1000 bits" at cheer prominence — an alert for a cheer that never
happened, tiered by the badge threshold as though it were the amount spent. IRC
parity was not worth preserving: since the IRC listener serves no channels, keeping
that mapping would have newly activated a misleading alert rather than matched any
behaviour users see. It renders as "🏅 Bits Badge Unlocked! / 1,000-bit badge" at
medium tier and still rides the existing bits toggle.

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

**The buffered-deletion drain is widened the same way**, and must be kept in lockstep
with the dedup predicate above. A moderator can delete a watch-streak message or an
announcement like any other message, and when the deletion races ahead of the message
it is buffered and drained on the message's arrival. That drain was gated on plain
chat only (`EventType == "" || "chat_message"`), so a notice — which arrives *with* an
event type set — never drained its buffered deletion and the removed message stayed
visible on every overlay until the buffer entry expired. Both predicates now read
"any Twitch message that is not a deletion", and a regression test asserts the drain
fires for `watch_streak`, `announcement` and plain chat alike.

### Settings and rendering

`watch_streak` gets one new per-overlay toggle, `enable_twitch_watch_streaks`
(migration 079, default TRUE) — it fires once per returning viewer per stream, so
busy overlays need an off switch. The sub-adjacent conversions
(`gift_paid_upgrade`, `prime_paid_upgrade`, `pay_it_forward`) ride the existing
gift-sub toggle; `unraid` rides the existing raid toggle; `bits` and
`bits_badge_tier` ride the bits toggle.

**A disabled toggle suppresses the decoration, never the message.** This matters
specifically because of what these notices are. The message-processor's event filter
used to `continue` on a disabled event — fine for a follow or a raid, but for a
watch streak the event *is* the viewer's chat message, so switching the toggle off
would have deleted real chat and silently re-created the very bug this ADR fixes,
this time triggered from the settings UI.

So per-overlay filtering now runs *before* the event/chat branch in the
message-processor. When an event is disabled and `filter.CarriesChatterMessage`
reports that its text is the chatter's own message (Twitch `watch_streak` and
`announcement`), the message is demoted to the chat path — `Normalize` instead of
`NormalizeEvent` — and renders as an ordinary chat line with its emotes, badges,
colour and GIFs, just without the milestone row. Everything else still drops as
before. The demotion is counted as `filtered_event/demoted_to_chat`.

`resubscription` is deliberately excluded: its text is an *optional* message
attached to a subscription event, so a streamer disabling "Resubscriptions"
reasonably means the whole notice, and including it would change long-standing
behaviour rather than fix a regression.

`announcement`, `charity_donation`, `modiversary` and `twitch_notice` are
deliberately **not** toggleable, marked with a `columnAlwaysEnabled` sentinel that
returns enabled without a database round-trip and without the "unknown event type"
warning. An announcement is chat content the broadcaster chose to highlight —
hiding it would hide chat.

The frontend adds icons, titles and CompactEvent labels for each new type. No CSS
work is required: the `.event-type-{type}` class is generated from `event.type`, and
the overlay's event partitioning is allowlist-free, so new types flow through
automatically.

**The event renderer renders the message like chat.** `EventContent` printed
`message.message.text` as a bare string, so emote codes appeared as literal words —
enrichment ran and produced the emote list, but nothing consumed it. For a notice
whose payload *is* a chat message that defeated the point, so `EventContent` now
renders through the shared `renderMessageContent` used by chat rows, and includes
`MessageAttachments` so an event-borne GIF appears instead of vanishing. To make
those attachments exist, `NormalizeEvent` also runs the chat path's chat-GIF
extraction (ADR-0037): strip the bracketed alt caption, re-anchor emote offsets,
surface the GIF. This benefits every event with user text (resub messages,
Super Chats, channel-point redemptions), not just notices.

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
  condition, so no additional subscription cost, but more state to reconcile) — and
  it recovers only on a demand cycle, as described above.
- Two predicates in the message-processor (native-id dedup and the buffered-deletion
  drain) must stay in lockstep. They already drifted once: widening only the first
  left moderator-deleted notices stuck on screen.
- The demote-to-chat path means the same raw message can render as an event on one
  overlay and as plain chat on another. That is correct per-overlay behaviour, but it
  is the first place where the event/chat distinction is not a property of the message
  alone.
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
