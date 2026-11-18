# ADR-0015: Dynamic EventSub Chat-Ownership Claim for the Twitch Chat Partition

**Date**: 2026-06-03
**Status**: Accepted
**Revised**: 2026-11-03
**Deciders**: All-Chat Platform Team

## Context

To relieve the shared Twitch IRC bot's ~100-channel cap, chat for channels whose owner granted the chat scopes was read via EventSub `channel.chat.message` instead of IRC. The partition was set by a static SQL predicate in twitch-listener's channel sync: a channel with a live `user:read:chat` grant was excluded from IRC and owned by EventSub.

The defect: the predicate says nothing about whether EventSub is actually delivering. A channel can satisfy it yet get no EventSub chat (revoked subscription, partial consent: the subscription needs `user:read:chat` + `user:bot` + `channel:bot` but the predicate checked only one; verify failures, token expiry skew, demand races, leader gaps, pod outages). Since the IRC listener ran in enforce mode ([ADR-0026](./0026-twitch-irc-listener-deprecation.md)), IRC joins no channels and cannot act as a fallback for any of these.

Alternatives rejected: gating the exclusion on all three scopes (fixes only the partial-consent case); a leader-driven "intent" claim when the subscription is created (claims a sub before it delivers, reintroducing the startup gap).

## Decision

An ownership claim that means "EventSub is delivering this channel's chat right now":

- The EventSub webhook handler writes `eventsub:chat:owner:{login}` (TTL `EVENTSUB_CHAT_CLAIM_TTL`, default 5m, refresh-write throttled to one per channel per 60s) on each delivered `channel.chat.message`, and releases it on revocation. The claim lapses by TTL whenever delivery stops for any reason.
- The IRC exclusion signal (historical) is the live claim set, `SCAN eventsub:chat:owner:*`, fail-open on Redis error; both sides share the key format and TTL via `shared/twitchchat`, so producer and consumer cannot drift. The brief IRC↔EventSub overlap is made invisible by message-processor dedup on the native Twitch message id (`Tags["id"]`), which both paths set identically.
- The live claim set is exposed as the Prometheus gauge `listener_eventsub_chat_owned` (one series per claimed login, refreshed every 30s), the live "who is serving this chat" signal.

Behaviour today (IRC enforce): revocation, verify/scope failures, or a total EventSub outage still stop chat for the affected channels until they are re-added or the listener recovers; the claim and its metric show exactly which channels are being served. Recovered failure modes resolve within one claim TTL with no restart.

Known limitation: `channel.chat.clear_user_messages` carries no ban duration (unlike IRC's `CLEARCHAT @ban-duration`), so a timeout on an EventSub-owned channel is reported downstream as a permanent ban. Messages are removed identically; only the moderation-log label differs. Fixing it would need a broader `channel.moderate`/`channel.ban` subscription and is deliberately out of scope.

## Implementation

- `shared/twitchchat/claim.go`: `ClaimStore` (`Claim`, `Release`, `ClaimedLogins`), `ClaimKey`, `ClaimTTL`, `ClaimRefreshInterval`; tests in `claim_test.go`.
- twitch-eventsub-listener: `webhooks/handler.go` refreshes the claim on delivery and releases on revocation (constructed in `cmd/main.go`). Because a claimed channel is read only by EventSub, deletion parity lives there too: subscriptions to `channel.chat.message_delete`, `channel.chat.clear_user_messages`, `channel.chat.clear` (created and torn down with the message sub), mapped to the same `message_deletion` raw-event shapes (`single`/`batch`/`clear`) as the IRC parser, plus the `message-processor/registry` population (native id → internal UUID) so single-message deletes resolve. See `handleChatMessageDelete`/`handleChatClearUserMessages`/`handleChatClear` and `eventsub/subscription_manager.go`.
- message-processor: `dedup/dedup.go` (`IsDuplicateNativeID`) drops duplicate Twitch chat by native id in `consumer/stream_consumer.go`.
- twitch-listener (historical): `channels/manager.go` filtered its desired set against `ClaimedLogins` instead of the static SQL exclusion.

Migration UX (layered on the mechanism): `overlay-manager` exposes `chat_via_eventsub` and `is_own_channel` per source on `GET /api/v1/overlays/:id/sources`; the frontend shows a dismissible banner plus per-source CTA (`frontend/src/components/EventSubMigrationBanner.tsx`) linking the add-source re-consent (`GetAuthURLWithChatScopes`, `force_verify=true`), migrating all of the account's own-channel sources on the next sync.

Related: ADR-0026 (IRC deprecation), ADR-0014 (demand linger), ADR-0007 (leadership gates subscription management; claims are deliberately leader-independent), ADR-0009/0002.
