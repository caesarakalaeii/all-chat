# ADR-0054: Twitch moderation and AutoMod event feed

**Date**: 2026-08-23
**Status**: Accepted
**Deciders**: caesarakalaeii

---

## Context and Problem Statement

All-Chat already reacted to Twitch moderation, but only to its _effects_. The
`channel.chat.message_delete`, `channel.chat.clear_user_messages` and
`channel.chat.clear` subscriptions tell the overlay that a message vanished, that a
user's history was wiped, or that chat was emptied, and the monitor renders those as
moderation-log rows (see `docs/features/message-deletion.md`). What none of them
carries is **who did it**. A streamer looking at their own monitor could see that
`someviewer` had been timed out and had no way to learn which of their five
moderators did it, for how long, or why — the one question a mod log exists to
answer.

The second gap is AutoMod. A message AutoMod holds is never delivered as chat at
all: it is withheld pending review, so from All-Chat's point of view the viewer
simply did not speak. The streamer has to sit in Twitch's own AutoMod queue to see
it, which defeats the single-pane premise the product is built on.

Both gaps are read-only problems — nothing here acts on Twitch. But the AutoMod half
brings a scope requirement and a confidentiality requirement that the chat path has
never had, and those are what make this a decision rather than a patch.

## Decision Drivers

- **Attribution.** A moderation log without the acting moderator is not a log.
- **A held message must be readable.** "AutoMod held something" with no text is a
  notification, not information; a moderator cannot decide anything from it.
- **Forward compatibility.** Twitch adds moderator actions over time. An action this
  build has never seen must render as an unrecognised-but-visible row, never vanish.
- **Held text is confidential.** It is text the platform decided chat should not
  see. The overlay WebSocket is effectively public — an OBS browser source connects
  with no token at all — so delivery cannot be "broadcast like chat".
- **Least privilege (ADR-0012), and honesty about where it is not achievable.** The
  scope set is wide and one of the scopes is a write scope for a read-only feature.
  That has to be stated plainly rather than smuggled through a consent screen.
- **One consent, not two.** A streamer should grant once, not once now and again
  when the next iteration lands.

## Decision Outcome

Three EventSub subscriptions, one internal event type, one consent action, and
owner-only delivery.

### Why three subscriptions, and the wrong turn that made it three

`channel.moderate` v2 is the moderator **action** feed. It reports ban, timeout,
delete, warn, mod/unmod, vip/unvip, raid/unraid, followers-only, slow and more, and
crucially each event carries `moderator_user_id` / `moderator_user_login` — the
identity the pre-existing `channel.chat.*` deletion path does not have. That is the
whole reason this subscription exists: it is what answers "who timed this person
out".

The feed also has an `automod_terms` action, and **an earlier design read that as
"AutoMod holds arrive here"**. It does not. `automod_terms` is an edit to a
blocked/permitted **term list** — its payload
(`services/twitch-eventsub-listener/eventsub/types.go`, `ModerateAutomodTerms`) is
`action: add|remove`, `list: blocked|permitted`, a list of terms, and a
`from_automod` flag. There is no message in it and no chatter. Shipping on that
assumption would have produced a moderation log with the word "automod" in the
title, a row every time the streamer edited their blocked-term list, and **not one
held message anywhere in the feature**. The defect would have been invisible in
review, because the subscription is named after the right thing.

Actual held messages arrive only on `automod.message.hold` v2, which carries the
chatter, the held text, the AutoMod category that tripped and its 1-4 severity
level. How a hold ended — approved, denied, or expired — arrives only on
`automod.message.update` v2, which additionally names the moderator who resolved it.
So the three subscriptions are not three views of one feed; they are three disjoint
feeds, and dropping any one of them removes a capability outright.

All three are created in `SubscribeToChannelModerate`,
`SubscribeToAutoModMessageHold` and `SubscribeToAutoModMessageUpdate` in
`services/twitch-eventsub-listener/eventsub/subscription_manager.go`, which share
`subscribeModeratorScoped`: version `2`, and a condition with **both**
`broadcaster_user_id` and `moderator_user_id` set to the streamer (Twitch answers
400 to a condition carrying only the broadcaster). The streamer moderating their own
channel is the whole authorization model of this iteration — see "Owner-only, not
delegated" below.

### The `moderator:manage:automod` scope

`twitchModerationScopesByAction["modlog"]` in
`services/auth-service/oauth/twitch.go` holds nine scopes, in the order the consent
screen shows them:

`moderator:read:blocked_terms`, `moderator:read:chat_settings`,
`moderator:read:unban_requests`, `moderator:read:banned_users`,
`moderator:read:chat_messages`, `moderator:read:warnings`,
`moderator:read:moderators`, `moderator:read:vips`, `moderator:manage:automod`.

The eight `moderator:read:*` scopes are what `channel.moderate` v2 requires. Twitch
accepts the corresponding `moderator:manage:*` as a superset for some of them; we
request the narrow read form.

The ninth is the awkward one, and it is deliberate. **This iteration only reads.**
There is no Approve/Deny UI and no `POST /helix/moderation/automod/message` call
anywhere in this codebase. Twitch nonetheless requires `moderator:manage:automod` to
create an `automod.message.hold` subscription at all and offers no read-only
alternative. That is Twitch's scope design, not ours. Dropping the scope does not
degrade the feature gracefully; it leaves the AutoMod half with no events to show.

We ask for it **now**, in the same consent round as the read scopes, rather than
deferring it to a second consent round when Approve/Deny lands. A second round is
not free: it is another banner, another redirect, another `force_verify` prompt, and
a population of streamers who granted the first round and never see the second. One
consent for a coherent capability is the better trade even though it means asking for
a write scope we do not exercise. Both the code comment on the scope list and the
consent banner in the monitor say so in plain words, because a "manage" permission on
a read-only feature looks like a mistake and gets declined.

### One event type, not five

Every event from all three subscriptions is published to `chat:raw` as a
`RawChatMessage` with `EventType: "mod_action"` (the single constant
`modActionEventType` in `services/twitch-eventsub-listener/webhooks/handler.go`).
The specific action lives in `EventData["action"]`: Twitch's own action string for
`channel.moderate`, and the two synthesized names `automod_hold` and
`automod_resolved` for the AutoMod feeds.

Twitch's action string is passed through **verbatim** rather than matched against a
whitelist. Twitch adds actions over time, and an unrecognised one must reach the
monitor as a visible row rather than disappear; `buildModerationAction` extracts
target details only for the actions All-Chat models, and an unmodelled action simply
carries no target. `modActionKind` in `frontend/src/lib/utils/overlayViewModel.ts`
mirrors that: anything it does not recognise renders as kind `action`, built from
whatever the frame gave it.

Two consequences follow, and both were the point:

- `mapEventTypeToColumn` in `services/message-processor/filter/event_filter.go`
  needs exactly **one** entry for the whole feature, and its value is
  `columnAlwaysEnabled` — a moderator cannot switch off their own audit log, so
  there is no per-overlay toggle, no migration, and no database round-trip. Five
  event types would have meant five entries and a decision about each.
- The frontend matches **one** string. `classifyEnvelope`
  (`frontend/src/lib/utils/overlayStreamCore.ts`) routes `mod_action` to the
  moderation log instead of the feed, and the api-gateway's owner-only gate keys on
  the same single string.

The classifier gives `mod_action` tier `low` with duration **0**
(`services/message-processor/classifier/tier.go`): duration 0 is how "produce no
alert" is expressed, because these belong in the moderator's view and never on the
public OBS overlay.

### Owner-only delivery — the load-bearing security property

An `automod_hold` frame's `EventData` carries `held_text`: the full text of a message
Twitch withheld from chat. Nobody in that chat ever saw it.

The overlay WebSocket is effectively public. `HandleOverlayConnection`
(`services/api-gateway/handlers/websocket.go`) accepts a socket with no token at all
— that is the anonymous OBS browser-source path, and it must keep working. So
mod frames are broadcast **only** to sockets that presented a valid JWT, cleared the
logout-revocation blacklist, and passed `VerifyOverlayOwnership`. That conjunction is
recorded on the connection as `isOwner` and enforced in `Pool.BroadcastFiltered`
(`services/api-gateway/websocket/pool.go`) via `BroadcastFilter.OwnerOnly`, which
also excludes viewer sockets — a distinct public path that never proves ownership.
Ownership is never inferred from a user id; `"obs"` is a sentinel string, not an
authorization decision.

The gate has a second half that is just as necessary: mod frames are **excluded from
the chat replay buffer**. `shouldBufferForReplay`
(`services/api-gateway/cmd/main.go`) refuses them, because the buffer is replayed to
every socket on connect — including anonymous ones — so buffering a held message
would leak later, out of context, exactly the text the owner-only broadcast had just
protected.

Classification and routing live in one function, `overlayBroadcastFilter`, on
purpose: recognising a `mod_action` frame and then broadcasting it with `OwnerOnly`
unset leaks held text to every anonymous browser source, and a test of the
classification alone cannot see that. Returning the filter the broadcaster consumes
leaves the caller nothing to get wrong. Frames whose event cannot be decoded fail
open to "not a mod frame", which is safe: a frame that does not parse cannot carry
`held_text` any consumer would render.

**This is the load-bearing security property of the feature.** If mod frames reach a
tokenless socket, All-Chat publishes pre-moderation content to whatever is pointed at
the overlay URL, and the URL alone already grants chat read to anyone who has it.

### Owner-only, not delegated

`modlog` is deliberately absent from `delegatableActions` in
`services/auth-service/handlers/mod_consent.go`, so a delegated moderator's consent
screen can never request these scopes. ADR-0048 draws the line this respects: a
volunteer acts with their **own** platform credential and is asked for the minimum
their delegated actions need. Nine scopes over the broadcaster's channel — one of
them a "manage" scope — is not that, and a volunteer must never be asked for AutoMod
permissions on a channel they merely help with. The feed reads the _channel's_ own
moderation history, which is the owner's record, not an action a moderator can be
handed.

The consent flow is correspondingly the owner's: `?actions=modlog` against
`/api/v1/auth/twitch/moderation/{overlay_id}`, reached from
`moderationApi.getTwitchModLogConsentUrl`, which unions the nine scopes with the
scopes already granted for Twitch so the issued token is a superset and never trips
the downgrade guard.

## Considered Options

1. **Enrich the existing `channel.chat.*` deletion path instead of adding
   subscriptions.** ✅ No new scopes, no new consent. ❌ Impossible: those payloads
   contain no moderator identity and no held message, so neither question this ADR
   exists to answer can be answered from them. **Rejected as not a solution.**

2. **`channel.moderate` alone, reading `automod_terms` as the AutoMod feed.** ✅ One
   subscription, and only the eight read scopes — no `moderator:manage:automod` on
   the consent screen. ❌ Factually wrong, as above: a mod log that says "automod"
   and contains blocked-term-list edits instead of held messages. **Rejected once
   the payload was actually read** — recorded here because it is the mistake this
   design nearly shipped.

3. **`channel.moderate` plus `automod.message.hold`, without
   `automod.message.update`.** ✅ Held messages appear, one fewer subscription. ❌
   Every hold stays "held" forever; the panel accumulates rows that are already
   resolved on Twitch, which is worse than not showing holds at all because it reads
   as a queue that needs attention. **Rejected.**

4. **All three subscriptions, one `mod_action` event type, owner-only delivery
   (chosen).** ✅ Attribution, held text, and resolutions, with one filter entry and
   one string for the frontend to match. ❌ Nine scopes including a write scope we
   do not use, and a new delivery class that must never be broadcast like chat.
   **Chosen.**

5. **Ship Approve/Deny in the same iteration, so the write scope is honest.** ✅ The
   consent screen would ask for nothing unused. ❌ Doubles the surface — a write
   path to Twitch, its idempotency, its audit rows, its failure copy — for a feature
   whose value is already delivered by reading. And it is safely additive later; see
   below. **Rejected as scope creep.**

## Explicitly out of scope

- **Approve/Deny actions and the `POST /helix/moderation/automod/message` path.**
  Deferring is safe because `automod.message.update` already delivers the
  resolution _and_ the resolving actor. When Approve/Deny lands, the row it produces
  is the row already rendered today, arriving through the same subscription with the
  same join key. It is an additive change — a button and one Helix call — not a
  rework of the feed, and the scope it needs is already granted.
- **Delegated-moderator access to the feed.** Per "Owner-only, not delegated" above.
  If it is ever wanted, it is a new decision about a much wider scope set on a
  volunteer's account, not an extension of this one.
- **Any non-Twitch platform.** No other platform All-Chat ingests exposes a
  moderator-action feed or a held-message queue; there is nothing to normalize
  against. The event type is deliberately generic (`mod_action`, not
  `twitch_mod_action`) so a second platform would not need a new taxonomy, but
  nothing has been built or designed for one.

## Consequences

### Positive

- The monitor's Activity & Events panel finally answers "who did this", with the
  reason and the timeout duration when Twitch supplies them.
- AutoMod holds are visible in All-Chat with their text, category and severity, and
  a hold folds into one row when its resolution arrives (`mergeAutoModResolution`),
  so "how many are still waiting" is readable.
- Twitch actions this build has never seen still produce a row, on both the listener
  and the frontend side.
- Zero settings surface: one `columnAlwaysEnabled` filter entry, no migration, no
  toggle to get wrong.
- Held pre-moderation content is confined to authenticated owner sockets, and the
  same predicate keeps it out of the replay buffer.

### Negative

- The consent screen asks for nine scopes, one of which is a write scope the code
  never exercises. That is Twitch's design, but the streamer sees it as ours, and a
  declined consent is the expected failure mode. Mitigated only by copy.
- Three more EventSub subscriptions per Twitch channel to create and reconcile.
- **The feed is invisible until the next channel (re)sync.** Like every other event
  subscription, these are created in the listener's per-channel `subscribe` action,
  which fires once when a channel is first tracked. A streamer who grants `modlog`
  after their channel is already tracked gets nothing until a leader change, a pod
  restart, or the channel being re-added. This is the pre-existing behaviour of the
  whole event-subscription layer (ADR-0030's known limitation), not new here, but it
  is the first feature where a _user-initiated_ grant is what the user is waiting on,
  so it is the first one where the delay is confusing rather than invisible.
- A missing grant is silent by design: `isScopeError` classifies Twitch's 403 and the
  listener logs it at **Info** with a `scope_errors` count, not as an error. Correct
  — an ungranted opt-in is not a fault — but it means "I enabled it and see nothing"
  has no loud signal to search for. The troubleshooting section of
  `docs/features/twitch-moderation-events.md` exists for exactly this.
- `mod_action` frames are the first delivery class the api-gateway must _withhold_
  from some sockets in a pool. Every future frame type now carries an implicit
  question ("who may see this?") that chat never posed.
- `moderationSummary` sets `Text` on every mod frame and nothing renders it — the
  frontend builds its own copy from the metadata. It exists because every other
  handler sets `Text` and an empty one trips the message-processor's empty-text
  metric. Dead-ish weight, kept for that reason and no other.

### Neutral

- No database change of any kind: no migration, no new column, no persistence. The
  feed is live-only, capped at `MAX_MOD_LOG` rows in the browser, and gone on reload.
- `mod_action` frames advance the client's replay watermark by wall-clock, like
  deletions, because they carry no message timestamp of their own.

## Testing

`services/twitch-eventsub-listener/webhooks/handler_moderation_test.go` covers the
three handlers' wire format — action pass-through for an unmodelled action, target
extraction per sub-object, the absolute-`expires_at`-to-duration conversion, and the
absence of moderator keys on a hold (AutoMod is not a person, and a placeholder there
would later read as a real moderator having acted).
`moderation_pipeline_test.go` drives verbatim Twitch payloads through listener
conversion and the `chat:raw` wire format.
`services/api-gateway/websocket/pool_test.go` and
`services/api-gateway/cmd/main_test.go` pin the owner-only filter and the
replay-buffer exclusion; `applyOverlaySocketFlags` and `ownershipVerified` were
extracted as functions specifically so the wiring that feeds `isOwner` is testable
without a live WebSocket upgrade — hard-coding it false takes the streamer's mod log
dark, and hard-coding it true publishes held text to every anonymous browser source.
On the frontend, `overlayViewModel.test.ts` covers `toModActionEntry` and the
hold/resolution merge, and `overlayStreamCore.test.ts` covers the `mod_action`
classification.

An end-to-end test is not possible: an AutoMod hold requires AutoMod to actually trip
on a real message in a real channel, and no sandbox or CLI event synthesizes one. The
verbatim-payload pipeline test is the furthest upstream a test can start.

## References

- [Twitch: channel.moderate v2 event reference](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-moderate-event)
- [Twitch: automod.message.hold v2 event reference](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-hold-v2-event)
- [Twitch: automod.message.update v2 event reference](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#automod-message-update-v2-event)
- [ADR-0012](./0012-oauth-scope-minimisation.md) — the minimisation stance the nine-scope
  request is measured against, and why the ninth is called out rather than buried.
- [ADR-0017](./0017-chat-moderation-write-path.md) — the opt-in moderation re-consent
  flow this reuses with a new action.
- [ADR-0030](./0030-twitch-native-engagement-mirroring.md) — the subscribe-on-first-track
  limitation that governs when a fresh `modlog` grant takes effect.
- [ADR-0046](./0046-twitch-chat-notices-via-chat-notification.md) — the
  pass-through-the-unrecognised and `columnAlwaysEnabled` patterns reused here.
- [ADR-0048](./0048-delegated-overlay-moderators.md) — why `modlog` is not delegatable.
- [Message deletion feature](../features/message-deletion.md) — the effects-only path
  this complements.
- [Twitch moderation events feature guide](../features/twitch-moderation-events.md)
