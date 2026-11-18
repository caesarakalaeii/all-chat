# ADR-0030: Twitch-Native Poll/Prediction Mirroring

**Date**: 2026-07-06
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

A Twitch streamer can also run polls and predictions through Twitch's own UI, with viewers voting in Twitch's currency (Channel Points). Those rounds are invisible to All-Chat, so an overlay combining Twitch with other platforms shows nothing while a native round runs on screen. Constraints: EventSub delivers per-option aggregates, never individual voters, so a native round cannot be reconstructed as votes/entries; a native prediction settles in Channel Points, which All-Chat must never mix with viewer points; reading polls/predictions needs `channel:read:polls` / `channel:read:predictions` (not login scopes), so mirroring must be strictly opt-in and a missing grant must be a non-event; and the feature must be additive — chat commands (ADR-0028) already let Twitch viewers participate.

## Decision

**Reuse the `polls`/`predictions` tables with `source = 'twitch_native'` and mirror columns, and deliver native events over a durable Redis stream.**

Alternatives considered:

- **Separate `native_polls`/`native_predictions` tables**: forks the read path, publisher, snapshot types and frontend, duplicating the state machine. Rejected.
- **Best-effort Pub/Sub for native events**: a dropped `end` leaves a poll ACTIVE forever on the overlay. Rejected for lifecycle state (fine for earns, not for the state machine).
- **Routing native events through `chat:raw`**: engagement-domain state is not renderable chat. Rejected.

### Producer (`twitch-eventsub-listener`)

The leader's per-channel `subscribe` action additionally creates the seven subscriptions `channel.poll.{begin,progress,end}` and `channel.prediction.{begin,progress,lock,end}`. They need the `engagement` scope grant, folded into the existing Twitch re-consent flow (`?actions=engagement` → `channel:read:polls` + `channel:read:predictions`, added to `preservableScopes` so a later plain login cannot silently strip it). A scope error is expected and non-fatal — a channel is simply not mirrored until its owner opts in. Each notification is normalized to a shared `NativeEngagementEvent` (defined in the message-processor models module and imported by both services so the contract cannot drift) and `XADD`ed to `engagement:twitch-native`, with `ChannelID` set to the lowercase broadcaster login (the exact key `overlay_chat_sources.channel_id` stores). On XADD failure the webhook returns an error and skips its dedup key, so Twitch redelivers.

### Consumer (`engagement-service/consumer/native.go`)

A durable `XREADGROUP` (group `engagement-native`) resolves the event's overlays via `OverlaysForChannel('twitch', login)` and upserts a `source='twitch_native'` row per overlay, keyed by the migration-070 partial unique index `(overlay_id, source, external_id)`. State mapping is pure and unit-tested (`begin/progress→ACTIVE`, `lock→LOCKED`, prediction `end→RESOLVED|CANCELED`, poll `end→CLOSED`); aggregates land in `poll_options.mirror_votes` and `prediction_outcomes.mirror_points`/`mirror_entrants`. The refreshed row is broadcast through the existing publisher.

### Currency isolation (the load-bearing invariant)

- Native rounds never set the `engagement:active:*` flag, so the hot path never forwards chat commands for them.
- `GetActivePoll`/`GetActivePrediction` (used by the chat-command and viewer-web write paths) stay `source='allchat'`-only, and `RecordVote`/`Wager` reject any poll/prediction whose source is not `allchat` — native rounds are unreachable by any ledger code.
- Rendering uses `GetActiveDisplayPoll`/`GetActiveDisplayPrediction` (either source). Read tallies are `computed + mirror_*`: an All-Chat row has `mirror_* = 0`, a native row has no votes/entries, so the sum is exact without a branch.

### Precedence

While a native round is live, the All-Chat create endpoints 409. The reverse (a native round beginning while an All-Chat round runs) is resolved at display time: the display queries prefer the All-Chat round, which can hold real wagered points and must stay resolvable from the control panel; once it ends, the native round shows.

### Ordering safety

EventSub does not guarantee ordered delivery and redelivers failed webhooks, so the upserts are monotonic: a state guard blocks backward transitions (a late `progress` cannot reopen a CLOSED/RESOLVED/CANCELED round), tallies use `GREATEST`, and deadline/lock/resolve timestamps `COALESCE`. A blocked stale event is a no-op.

**Synthetic-cancel exception**: the 4h stale-sweep force-CANCELs a native round stuck live, but a LOCKED Twitch prediction has no forced-resolution deadline, so the sweep can fire before the genuine `channel.prediction.end`. A sweep cancel is tagged `sweep_canceled = TRUE`; the guard lets a genuine terminal event override a synthetic cancel (recording the real winner), while genuine RESOLVED↔CANCELED lateral flips stay blocked.

## Consequences

- Overlays reflect native rounds with zero new render/broadcast/type surface — the `source` discriminator does all the work; points integrity is structural, not conventional; the feature is invisible and free for channels that never opt in.
- In the rare both-live state, the native round is hidden until the All-Chat round ends (non-destructive, kept rare by the create-side 409). Because display precedence orders live rounds ahead of terminal ones, a native round going ACTIVE while an All-Chat round resolves/cancels can pre-empt its "who won" frame during the grace window — an accepted, display-only trade-off.
- Mirrored tallies refresh only as fast as Twitch sends `progress` events.
- Known limitation: the seven subscriptions are created in the per-channel `subscribe` action, which fires once when a channel is first tracked; a streamer granting the `engagement` scope after that won't be mirrored until the next channel resync (pre-existing behaviour of the whole event-subscription layer).
- Follow-ups: reconcile event subscriptions on every channel sync; subscription-budget cost tracking; surface "mirroring enabled" in the streamer UI.

## Implementation

- `services/twitch-eventsub-listener/` — subscription creation in the per-channel subscribe action, webhook normalization to `NativeEngagementEvent`.
- `services/message-processor/` — shared `NativeEngagementEvent` model.
- `services/engagement-service/consumer/native.go` — the durable consumer and upsert path.
- Migrations 069/070 carry the `source`, `external_id` and `mirror_*` columns.

## Related

- [ADR-0028](./0028-engagement-chat-command-write-path.md) — the All-Chat participation path beside this one.
- [ADR-0029](./0029-viewer-points-economy-and-prediction-payout.md) — first stated native-point isolation; this ADR enforces it end-to-end.
- ADR-0012 / ADR-0017 — the opt-in re-consent scope-bundle pattern reused for the `engagement` action.
- Numbering is shared with caesar-deployment, where 0021–0022 live.
