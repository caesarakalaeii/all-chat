# ADR-0015: Dynamic EventSub Chat-Ownership Claim for the Twitch Chat Partition

**Date**: 2026-06-03
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: All-Chat Platform Team

---

## Context and Problem Statement

To relieve the shared Twitch IRC bot's ~100-channel cap, chat for channels whose owner granted the
chat scopes is read via EventSub `channel.chat.message` instead of IRC (see the feature commit
`feat(twitch): read chat via EventSub, demand-gated, to relieve IRC cap`). The two paths were
**partitioned by a static SQL predicate**: `twitch-listener` (IRC) excluded any channel where
`'user:read:chat' = ANY(users.granted_scopes) AND token_expires_at > NOW()`, expecting EventSub to
own it.

The defect: that predicate is **decoupled from whether EventSub is actually delivering chat**. A
channel can satisfy the exclusion predicate yet receive no EventSub chat for many reasons, and in
every one of them IRC has *also* dropped the channel — so chat is lost with **no fallback**:

- **Revocation** (observed in production, broadcaster `196691714`): a `channel.chat.message`
  subscription is revoked; the listener only logged it, never recreated the sub or cleared state —
  permanent loss until pod restart.
- **Partial consent**: the `channel.chat.message` subscription requires `user:read:chat` **+
  `user:bot` + `channel:bot`**, but the IRC exclusion checked only `user:read:chat`. A channel with
  the partial grant is excluded from IRC yet cannot get an EventSub sub — permanent loss.
- **Verification pending/failed, token expiry skew, demand races, leader gaps, pod outage,
  startup/reconnect latency** — each a window where the predicate says "EventSub owns it" but
  EventSub is not delivering.

Since the two-phase deprecation of the IRC listener landed ([ADR-0026](./0026-twitch-irc-listener-deprecation.md)), the IRC
listener runs `TWITCH_IRC_DEPRECATION_MODE=enforce` in production and joins no channels at all, so
EventSub is the only Twitch chat path and the fallback below covers none of these failures any more.
But the claim mechanism is what the enforce phase stands on: the ownership claim is still how the
system decides, per channel and in real time, that EventSub is the one delivering its chat.
The text below describes the mechanism as designed at decision time, when IRC was still live.

## Decision Drivers

- **No silent message loss is paramount.** A channel must always be read by *someone*.
- **Preserve the cap-relief benefit** for the channels that actually matter (active chat).
- **Self-healing:** recovery must not require a pod restart or manual action.
- **Don't reintroduce divergent in-memory cross-pod state** (webhooks land on any pod; subscription
  management is leader-gated) — coordinate through Redis, consistent with ADR-0002 / ADR-0014.
- **Minimal blast radius and reversible.**

## Considered Options

1. **Dynamic traffic-driven ownership claim; IRC excluded a channel only while it was provably
   delivering chat (chosen).**
   - The EventSub webhook handler writes a per-channel claim key `eventsub:chat:owner:{login}` (TTL
     `EVENTSUB_CHAT_CLAIM_TTL`, default 5m) on each delivered `channel.chat.message`, throttled to one
     write per channel per `ClaimRefreshInterval` (60s). It releases the claim on revocation. IRC
     excluded only channels with a **live claim**, fetched via `SCAN eventsub:chat:owner:*`.
  - ✅ Pros: IRC is the fallback sink — a channel leaves IRC *only while EventSub is actually
    delivering chat*, and returns within the TTL the instant EventSub stops (revoke, verify-fail,
    scope gap, pod outage, leader gap). Verify-fail/scope-fail produce **zero** loss (no message →
    no claim → IRC never dropped it). No leader/cross-pod coordination on the hot path. Cap relief
    tracks activity: busy channels stay on EventSub, idle ones cost a cheap IRC slot.
  - ❌ Cons: A brief IRC↔EventSub overlap during handoff (≤ one IRC sync) and a possible ≤TTL fallback
    delay in a total-EventSub-outage case; relies on message-processor native-id dedup to make the
    overlap invisible.

2. **Gate the IRC exclusion on all three required scopes (`user:read:chat`+`user:bot`+`channel:bot`).**
   - ✅ Pros: Fixes the partial-consent case with a one-line predicate change.
   - ❌ Cons: Leaves every other failure mode (revocation, verify-fail, demand/leader gaps, outage)
     as permanent loss. Not self-healing.

3. **Leader-driven "intent" claim (claim when the leader creates the sub).**
   - ✅ Pros: Keeps idle-but-eligible channels off IRC (better steady-state cap relief).
   - ❌ Cons: Claims a sub before it is confirmed delivering → reintroduces the startup gap and the
     verify-fail loss; requires a revocation→leader signal to avoid resurrecting a revoked claim;
     more cross-pod state. Higher complexity for marginal cap benefit.

## Decision Outcome

**Chosen: Option 1.** A chat-ownership claim means "EventSub is delivering this channel's chat right
now." It is created/refreshed by the webhook handler on real delivered chat and released on
revocation; it lapses on its own (TTL) whenever delivery stops for any reason. The IRC listener
treats the presence of a claim as the sole exclusion signal, replacing the static scope predicate, so
IRC (while it was live) served any channel EventSub was not currently serving (in the two-phase
deprecation of [ADR-0026](./0026-twitch-irc-listener-deprecation.md) this became "served by nothing but EventSub, in enforce
mode"). The unavoidable brief handoff overlap is made idempotent by deduplicating in
message-processor on the **native Twitch message id** (`tags["id"]`), which both the IRC parser and
the EventSub handler set to the identical value.

The claim key format and TTL live in a single shared package (`shared/twitchchat`) imported by both
listeners, so the producer (EventSub) and consumer (IRC) of the claim cannot drift — the same
discipline the previous byte-identical SQL predicate aimed for.

Beyond the partition boundary, the live claim set is also exposed as the Prometheus gauge
`listener_eventsub_chat_owned` (one series per claimed channel's login, refreshed every 30s) so
dashboards can tell which Twitch channels EventSub is serving right now; without it the only
Prometheus signal for "on IRC" was message activity, which counts brief claim-lapse flaps.

## Consequences

### Positive
- Eliminates every identified permanent-loss mode; the worst case degrades to a bounded fallback
  delay, never silent permanent loss (while IRC was live; since the enforce phase of ADR-0026 there
  is no fallback, and an uncovered failure stops a channel's chat until the channel is re-added or
  the EventSub listener recovers).
- Self-healing with no restart: revoke/outage/leader-loss all resolve within the claim TTL (with the
  same qualification above: today "resolves" means "the claim clears", not "chat returns").
- Cap relief is preserved for active channels (the ones that pressure the cap) — historically the
  IRC cap motivated the split; today the activity-scoped claim still bounds the EventSub
  subscription set to channels that actually talk.

### Negative
- Since the IRC listener's enforce phase ([ADR-0026](./0026-twitch-irc-listener-deprecation.md)), the
  IRC fallback path this mechanism was built around is disabled. Failures that a live claim used to
  cover (revocation, verify-fail, partial consent, total outage) instead stop chat for the affected
  channels until they are re-added or EventSub recovers. The ownership claim and its Prometheus
  mirror remain the live "who is serving this chat" signal.
- **Deploy transient.** Immediately after a deploy, claims have not yet been (re)written, so the
  IRC desired set briefly included scope-eligible channels until each received an EventSub message
  and the claim formed (≤ the 60s refresh interval for active channels; quiet channels carry no
  claim and have no chat to lose in the first place).
- Introduces a dependency on message-processor native-id dedup; without it, viewers would briefly
  see duplicated Twitch chat during the (IRC-era) handoff overlap.
- **Timeout vs. ban is not distinguishable on EventSub-owned channels.** `channel.chat.clear_user_messages`
  carries no duration (unlike IRC's `CLEARCHAT` `@ban-duration`), so a timeout is reported downstream
  as a permanent ban. The messages are removed identically either way; only the moderation-log label
  differs. Restoring the distinction would require an additional `channel.moderate`/`channel.ban`
  subscription (broader scope) and is deliberately out of scope here.
- `SCAN eventsub:chat:owner:*` per IRC sync (cheap at current channel counts; bounded by channel
  count, on the twitch-listener's existing 30s sync cadence while it still ran).

### Failure-mode behaviour (as designed, while IRC still served a partition)
| Mode | Claim outcome | Result while IRC served the channel | Result today (IRC enforce) |
|------|---------------|--------|--------|
| Verify-fail / scope-fail / partial consent | never created (no message) | IRC serves from the start | no chat until re-add |
| Revocation | released by handler (or expires ≤TTL) | IRC resumes | no chat until re-add |
| EventSub pod/total outage | expires ≤TTL | IRC resumes | no chat until the listener recovers or the channel is re-added |
| Leader gap | unaffected (claim is traffic-driven, not leader-driven) | continuous | chat may pause during the gap |
| Startup / reconnect | absent until first EventSub message | IRC covers, then handoff (deduped) | chat may pause until the first message arrives |

What survives in the current implementation is the *ownership model*: a channel is counted, per
channel, as "EventSub is delivering chat right now" via the live claim set (and its Prometheus
mirror). The IRC-side fallback rows above are historical; the claim set itself is still live.

## Implementation

- **New**: `shared/twitchchat/claim.go` — `ClaimStore` (`Claim`, `Release`, `ClaimedLogins`),
  `ClaimKey`, `ClaimTTL`, `ClaimRefreshInterval`. Tests in `claim_test.go` (miniredis).
- **twitch-eventsub-listener**: `webhooks/handler.go` refreshes the claim on `channel.chat.message`
  delivery (throttled) and releases it on revocation; `cmd/main.go` constructs the `ClaimStore` and
  injects it into the handler.
- **twitch-eventsub-listener (deletion parity)**: because a claimed channel is read *only* by
  EventSub, IRC's `CLEARMSG`/`CLEARCHAT` handling and its message-ID registry population no longer
  run for it — so EventSub must reproduce both, or moderation silently fails to reach the overlay.
  The listener therefore also subscribes to `channel.chat.message_delete`,
  `channel.chat.clear_user_messages`, and `channel.chat.clear` (created/torn down together with the
  `channel.chat.message` subscription — same scope + condition), maps them to the existing
  `message_deletion` raw-event shape (`single` / `batch` / `clear`, identical to
  `twitch-listener`'s parser), and populates the shared `message-processor/registry`
  (native id → internal UUID) on each delivered chat message so single-message deletes resolve to
  the displayed message. See `webhooks/handler.go` (`handleChatMessageDelete` /
  `handleChatClearUserMessages` / `handleChatClear`) and `eventsub/subscription_manager.go`.
- **twitch-listener**: `channels/repository.go` drops the static `eventSubOwnedExclusion` SQL;
  `channels/manager.go` filters the desired-channel set against live claims via `ClaimedLogins`
  (fail-open on a Redis error: while the IRC listener ran, it read every channel and
  message-processor deduped the overlap; under ADR-0026 enforce mode the filter is moot).
- **message-processor**: `dedup/dedup.go` adds `IsDuplicateNativeID`; `consumer/stream_consumer.go`
  drops duplicate Twitch chat by native id (`Tags["id"]`, via the shared `platformMessageID`
  predicate) before the handler.
- **Configuration**: `EVENTSUB_CHAT_CLAIM_TTL` (Go duration, default `5m`), set per deployment and
  documented in `shared/twitchchat/claim.go` as also bounding the fallback delay in the
  (historical) total-outage case.
- **Timeline**: 2026-06-03.

## Migration UX

The claim mechanism makes the partition *safe*, but a channel only leaves IRC once its owner has
granted the chat scopes (`user:read:chat` + `user:bot` + `channel:bot`). Channels added before the
chat-scope consent flow existed never re-consented, so the majority of registered Twitch channels
remained on the IRC side — which mattered because IRC's shared bot was capped (~100 channels) and
silently dropped the overflow once more than that were live at once. The migration is therefore
driven from the product UI, not automatically:

- `overlay-manager` exposes two per-source booleans on `GET /api/v1/overlays/:id/sources`:
  `chat_via_eventsub` (any owner consent satisfies the partition predicate) and `is_own_channel`
  (the *requesting* user owns this channel and can re-consent — computed for both Twitch-login and
  ADR-0016 linked-credential owners via `ListByOverlayIDForUser`).
- The frontend surfaces a dismissible dashboard banner plus a per-source CTA for sources where
  `is_own_channel && !chat_via_eventsub`, linking the existing add-source re-consent reflow
  (`auth-service` `GetAuthURLWithChatScopes`, `force_verify=true`). One re-consent migrates all of
  that account's own-channel sources on the next sync. The banner behavior remains live in
  `frontend/src/components/EventSubMigrationBanner.tsx`.

This UX is layered on the existing partition; it does not change the claim mechanism above.
Successfully re-consenting sources now land on EventSub (which they were already eligible for);
per ADR-0026, with IRC in enforce mode there is no further fallback path.

## Related Decisions

- ADR-0026 (Two-phase IRC deprecation) — the enforce phase retired the IRC side of this partition;
  the claim mechanism and its ownership model survive as the live "who delivers this chat" signal.
- ADR-0014 (Demand linger) — the demand window during which a chat sub exists; this ADR made the
  IRC↔EventSub boundary resilient to *every* gap, not just demand (while the boundary was still a
  handoff between two live paths).
- ADR-0009 (Ring buffer publisher) — the "no silent drops" resilience philosophy, extended to the
  partition boundary.
- ADR-0007 (Leadership rebalancing) — leader gates subscription management; claims are deliberately
  leader-independent so leader transitions never drop chat.
- ADR-0002 (Redis Streams + Pub/Sub) — claims coordinate through Redis keys, like `overlay:connected:*`.
