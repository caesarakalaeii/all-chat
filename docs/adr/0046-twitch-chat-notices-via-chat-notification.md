# ADR-0046: Twitch Chat Notices (Watch Streaks, Announcements) via channel.chat.notification

**Date**: 2026-08-01
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

Twitch splits chat delivery across two EventSub subscriptions: `channel.chat.message` and `channel.chat.notification`. All-Chat subscribed only to the first. Several notice types carry the chatter's own message text (`message.text` + `fragments`) — `watch_streak` (a returning viewer's first message of the stream) and `announcement` (the body of a `/announce`) — and Twitch does not additionally send them as `channel.chat.message`. Those messages were therefore **silently dropped**: never published to `chat:raw`, never rendered, no error anywhere. Since ADR-0026 put the IRC listener into enforce mode, EventSub is the only Twitch chat path, so there was no fallback. (IRC would not have helped: its USERNOTICE parser maps `viewermilestone` and `announcement` to "unknown" and drops them, just more noisily.)

Beyond the two message-bearing notices the whole notice feed was missing: `bits_badge_tier`, `charity_donation`, `pay_it_forward`, `gift_paid_upgrade`, `prime_paid_upgrade`, `unraid`, `modiversary`, and every `shared_chat_*` variant.

The complication: the notice feed overlaps subscriptions All-Chat already has — `sub`, `resub`, `sub_gift`, `community_sub_gift` and `raid` also appear as notice types while being delivered (with richer data) by dedicated subscriptions. Naively forwarding every notice would double-render every sub and raid.

## Decision

**Subscribe to `channel.chat.notification` alongside chat, and route by notice type: skip the types a dedicated subscription already delivers, map the rest onto All-Chat's event taxonomy, and emit anything unrecognised generically.**

- **Subscription lifecycle.** `channel.chat.notification` is created and torn down with the chat subscription (same condition and authorization), best-effort — a failure leaves chat working.
- **Route or skip** (`buildChatNotice`, after stripping any `shared_chat_` prefix): skip `sub`, `resub`, `sub_gift`, `community_sub_gift`, `raid`; emit `watch_streak` and `announcement` (text = the chatter's message); emit `bits_badge_tier`, `gift_paid_upgrade`, `prime_paid_upgrade`, `pay_it_forward`, `unraid`, `charity_donation`, `modiversary` as same-named events; anything else becomes a generic `twitch_notice` event with `system_message` as text plus an Info log, with the skip-list the EventSub twin of the IRC listener's `isCoveredByEventSub`. Text falls back to `system_message` so event-only notices never render blank.
- **`bits_badge_tier` gets its own event type** rather than folding into `bits`: IRC parity would have rendered a lifetime badge unlock as a cheer alert at cheer prominence. It renders as "Bits Badge Unlocked" at medium tier and rides the existing bits toggle.
- **Chat parity for notice-borne messages.** Tags are built by delegating to the chat path's `buildChatTags` via a `ChatMessageEvent` view, so watch-streak messages get identical emote/badge/colour treatment. The native message id lands in `Tags["id"]` and is registered in the message-ID registry, so a later `channel.chat.message_delete` removes the row like any other.
- **Duplicate protection.** Rather than trust Twitch's disjoint-subscriptions guarantee, the message-processor's native-id dedup (ADR-0015) is widened to any Twitch message carrying `Tags["id"]` except deletions, collapsing a hypothetical twin pair in either arrival order. Deletions are excluded (they carry no `Tags`).
- **The buffered-deletion drain shares the dedup predicate.** The drain was gated on plain chat only, so a deletion racing ahead of a notice never drained. Both call sites now go through a single `platformMessageID` helper (a duplicated predicate had already gone stale once); a table test pins it.
- **Subscription recovery.** A revocation notification now evicts the cache (`SubscriptionManager.ForgetSubscription`) so a stale entry cannot make every future re-subscribe a silent no-op, and a leader-gated periodic repair pass (`reconcileChatSubscriptions`, 5m`ChatSubscriptionReconcileInterval`) re-asserts the chat subscription set for every chat-active channel, cheaply guarded by `HasSubscription`. The repair pass and `subscribe_chat` share one code path in `main.go`. A delivered notice also refreshes the chat ownership claim and the "connected" indicator; a revoked notification subscription does not release the claim.
- **Settings.** `watch_streak` gets a per-overlay toggle (migration 079, default TRUE). The gift/prime/pay-it-forward conversions ride the gift-sub toggle, `unraid` rides raid, bits badge rides bits. **A disabled toggle suppresses the decoration, never the message**: per-overlay filtering runs before the event/chat branch, and when a disabled event's text is the chatter's own message (`CarriesChatterMessage`: watch streak, announcement) it is demoted to the chat path and renders as an ordinary chat line. Resubscription is deliberately excluded (its message is optional; disabling "Resubscriptions" reasonably means the whole notice). `announcement`, `charity_donation`, `modiversary` and `twitch_notice` are not toggleable.
- **Rendering.** The event renderer now renders notice messages through the shared chat row renderer and includes `MessageAttachments`, so emotes, badges and event-borne GIFs (ADR-0037 extraction runs in `NormalizeEvent`) appear instead of literal emote codes. New event types need no CSS work: the class is generated from the type and overlay partitioning is allowlist-free.

Alternatives considered: forwarding every notice (double-renders every sub and raid); emitting only the two message-bearing types (leaves seven types and all shared-chat variants dropped, so the same bug recurs elsewhere).

## Consequences

- Watch-streak and announcement messages reach overlays; a moderator can delete them like any other message; future Twitch notice types degrade visibly instead of vanishing.
- One more subscription per chat-active channel, and a periodic pass that can talk to Twitch (bounded: leader-only, `HasSubscription`-guarded, 5-minute ticker; a permanently un-granted scope costs one TTL call per channel per pass).
- `ForgetSubscription` does not verify against Twitch before evicting; a spurious revocation causes one redundant re-create that the existing reconcile path heals — strictly better than never recovering.
- The demote-to-chat path means the same raw message can render as an event on one overlay and as plain chat on another — correct per-overlay behaviour, and the first place the event/chat distinction is not a property of the message alone.
- The skip-list couples the handler to the set of dedicated subscriptions: dropping one without updating `noticeCoveredByDedicatedSubscription` would silently lose those events.
- Testing: unit tests cover mapping, skip-list, shared-chat prefixes, tag parity and filter columns; a pipeline test drives a verbatim Twitch payload through listener → `chat:raw` → the event normalizer. A true end-to-end test is not possible: Twitch emits watch streaks only on real multi-stream watch history, with no sandbox event.

## References

- [Twitch: channel.chat.notification event](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-chat-notification-event)
- [ADR-0015](./0015-eventsub-chat-ownership-claim.md), [ADR-0026](./0026-twitch-irc-listener-deprecation.md), [ADR-0037](./0037-twitch-chat-gifs.md)
