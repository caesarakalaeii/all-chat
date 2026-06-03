# ADR-0015: Dynamic EventSub Chat-Ownership Claim for the IRC↔EventSub Partition

**Date**: 2026-06-03
**Status**: Accepted
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

## Decision Drivers

- **No silent message loss is paramount.** A channel must always be read by *someone*.
- **Preserve the cap-relief benefit** for the channels that actually matter (active chat).
- **Self-healing:** recovery must not require a pod restart or manual action.
- **Don't reintroduce divergent in-memory cross-pod state** (webhooks land on any pod; subscription
  management is leader-gated) — coordinate through Redis, consistent with ADR-0002 / ADR-0014.
- **Minimal blast radius and reversible.**

## Considered Options

1. **Dynamic traffic-driven ownership claim; IRC is the default, EventSub claims a channel only
   while it is provably delivering chat (chosen).**
   - The EventSub webhook handler writes a per-channel claim key `eventsub:chat:owner:{login}` (TTL
     `EVENTSUB_CHAT_CLAIM_TTL`, default 5m) on each delivered `channel.chat.message`, throttled to one
     write per channel per `ClaimRefreshInterval` (60s). It releases the claim on revocation. IRC
     excludes only channels with a **live claim**, fetched via `SCAN eventsub:chat:owner:*`.
   - ✅ Pros: IRC is the universal safety net — a channel leaves IRC *only while EventSub is actually
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
IRC automatically resumes any channel EventSub is not currently serving. The unavoidable brief
handoff overlap is made idempotent by deduplicating in message-processor on the **native Twitch
message id** (`tags["id"]`), which both the IRC parser and the EventSub handler set to the identical
value.

The claim key format and TTL live in a single shared package (`shared/twitchchat`) imported by both
listeners, so the producer (EventSub) and consumer (IRC) of the claim cannot drift — the same
discipline the previous byte-identical SQL predicate aimed for.

## Consequences

### Positive
- Eliminates every identified permanent-loss mode; the worst case degrades to a bounded fallback
  delay, never silent permanent loss.
- Verify-fail / scope-fail / partial-consent channels are served by IRC with **zero** gap.
- Self-healing with no restart: revoke/outage/leader-loss all resolve within the claim TTL.
- Cap relief is preserved for active channels (the ones that pressure the cap).

### Negative
- A channel idle (no chat) for longer than the claim TTL falls back to an IRC slot until chat
  resumes — cap relief is activity-scoped, not permanent.
- **Deploy transient.** Immediately after a deploy, claims have not yet been (re)written, so IRC's
  desired set briefly includes scope-eligible channels until each receives an EventSub message and
  the claim forms (≤ the 60s refresh interval for active channels; quiet channels stay on IRC but
  have no chat to lose). If the total exceeds the bot cap, JOINs are rate-limited, not dropped, and
  the set converges within ~1–2 minutes. No chat is lost — EventSub continues delivering the
  channels it owns, and message-processor dedupes any overlap.
- Introduces a dependency on message-processor native-id dedup to hide the handoff overlap; without
  it, viewers would briefly see duplicated Twitch chat during a handoff.
- `SCAN eventsub:chat:owner:*` per IRC sync (cheap at current channel counts; bounded by channel
  count, runs on the existing 30s-ish sync cadence).

### Failure-mode behaviour (all → IRC serves, no loss)
| Mode | Claim outcome | Result |
|------|---------------|--------|
| Verify-fail / scope-fail / partial consent | never created (no message) | IRC serves from the start |
| Revocation | released by handler (or expires ≤TTL) | IRC resumes |
| EventSub pod/total outage | expires ≤TTL | IRC resumes |
| Leader gap | unaffected (claim is traffic-driven, not leader-driven) | continuous |
| Startup / reconnect | absent until first EventSub message | IRC covers, then handoff (deduped) |

## Implementation

- **New**: `shared/twitchchat/claim.go` — `ClaimStore` (`Claim`, `Release`, `ClaimedLogins`),
  `ClaimKey`, `ClaimTTL`, `ClaimRefreshInterval`. Tests in `claim_test.go` (miniredis).
- **twitch-eventsub-listener**: `webhooks/handler.go` refreshes the claim on `channel.chat.message`
  delivery (throttled) and releases it on revocation; `cmd/main.go` constructs the `ClaimStore` and
  injects it into the handler.
- **twitch-listener**: `channels/repository.go` drops the static `eventSubOwnedExclusion` SQL;
  `channels/manager.go` filters the desired-channel set against live claims via `ClaimedLogins`
  (fail-open on Redis error → IRC reads everything, deduped).
- **message-processor**: `dedup/dedup.go` adds `IsDuplicateNativeID`; `consumer/stream_consumer.go`
  drops duplicate Twitch chat by `platform|tags["id"]` before the handler.
- **Configuration**: `EVENTSUB_CHAT_CLAIM_TTL` (Go duration, default `5m`; the IRC fallback delay in a
  total-outage case is bounded by this).
- **Timeline**: 2026-06-03.

## Related Decisions

- ADR-0014 (Demand linger) — the demand window during which a chat sub exists; this ADR makes the
  IRC↔EventSub boundary resilient to *every* gap, not just demand.
- ADR-0009 (Ring buffer publisher) — the "no silent drops" resilience philosophy, extended to the
  partition boundary.
- ADR-0007 (Leadership rebalancing) — leader gates subscription management; claims are deliberately
  leader-independent so leader transitions never drop chat.
- ADR-0002 (Redis Streams + Pub/Sub) — claims coordinate through Redis keys, like `overlay:connected:*`.
