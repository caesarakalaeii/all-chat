# ADR-0030: Twitch-Native Poll/Prediction Mirroring

**Date**: 2026-07-06
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Issue #523 gives All-Chat its own polls, predictions, and viewer points (ADR-0028 write-path, ADR-0029 economy). A Twitch streamer, though, can *also* run polls/predictions through **Twitch's own native UI**, and their viewers vote/wager there using Twitch's own currency (Channel Points). Those rounds are invisible to All-Chat, so an overlay that combines Twitch with other platforms shows nothing while a native Twitch round is live on screen.

We want the overlay (and the no-install participation surfaces) to **reflect** a native Twitch poll/prediction — its question, options, and live tallies — without pretending All-Chat ran it. The hard constraints:

- **Twitch owns the votes.** EventSub delivers per-option *aggregates*, never individual voters. All-Chat cannot reconstruct `poll_votes`/`prediction_entries` rows for a native round.
- **Two currencies must not mix.** A native prediction settles in Twitch Channel Points. All-Chat viewer points must **never** be credited or debited for a mirrored round (this was already pinned in ADR-0029; this ADR is where it gets enforced end-to-end).
- **Least privilege.** Reading polls/predictions needs `channel:read:polls` / `channel:read:predictions`. These are NOT login scopes — most streamers never grant them — so mirroring must be strictly opt-in and a missing grant must be a non-event, not a failure.
- **Additive only.** Twitch viewers already participate in All-Chat rounds via chat commands (ADR-0028); this feature is a display enhancement, not a new participation path.

## Decision Drivers

- Reuse the existing EventSub listener, its leader-gated per-channel subscription sync, and its at-least-once webhook→Redis pipeline.
- Reuse the existing engagement-service read/broadcast path (same `poll_update`/`prediction_update` snapshots) so overlays/widgets/monitor need no per-source rendering fork — the frontend only branches on `source`.
- Keep the All-Chat write path (chat votes/wagers, points ledger) provably unreachable for native rounds.
- Durability parity with the command stream: a missed `lock`/`end` would strand a mirrored round in a live state on the overlay with no later event to self-heal from.

## Considered Options

### Where the mirror state lives
1. **Reuse the `polls`/`predictions` tables with `source='twitch_native'` + `external_id`, storing aggregates in new `mirror_*` columns.**
   - ✅ One read path, one broadcast path, one set of frontend types; the `source` discriminator (already in the schema since 068/069) drives all divergence. Aggregates coexist with All-Chat's computed tallies because each source only ever populates one side.
   - ❌ Read queries must sum `computed + mirror`; write-path queries must stay `source='allchat'`-scoped.
2. **A separate `native_polls`/`native_predictions` table set.**
   - ❌ Forks the read path, the publisher, the snapshot types, and the frontend; duplicates the state machine. All cost, no isolation benefit that option 1's `source` column doesn't already give.

### How native events reach engagement-service
3. **Durable Redis stream `engagement:twitch-native` (XADD in the listener webhook, XREADGROUP in engagement-service).**
   - ✅ Survives an engagement-service restart; acked; mirrors the ADR-0028 command-stream posture. A lost lifecycle event would otherwise strand a round.
   - ❌ One more stream + consumer group.
4. **Best-effort Pub/Sub (like `engagement:events`).**
   - ❌ A dropped `end` leaves a poll ACTIVE forever on the overlay. Best-effort is fine for earns (a missed earn is an unpaid point); it is not fine for lifecycle state.
5. **Publish native events via the `chat:raw` stream publisher (as chat/event messages).**
   - ❌ These are engagement-domain state, not renderable chat; routing them through the normalizer/enricher is a category error.

## Decision Outcome

**Chosen**: Option 1 (same tables, `source='twitch_native'` + `mirror_*`) + Option 3 (durable stream).

**Producer** (`twitch-eventsub-listener`):
- The leader's per-channel `subscribe` action additionally creates the 7 subscriptions `channel.poll.{begin,progress,end}` and `channel.prediction.{begin,progress,lock,end}`. They need the broadcaster's `channel:read:polls` / `channel:read:predictions` grant; a scope error is expected and **non-fatal** (`scopeErrorCount`), exactly like the moderation subs — a channel simply isn't mirrored until its owner opts in.
- **Opt-in grant** is an `engagement` action folded into the existing Twitch re-consent flow (`?actions=engagement` → `channel:read:polls`+`channel:read:predictions`, added to `preservableScopes` so a later plain login can't silently strip it). Least privilege, additive to any existing grant, Twitch-only.
- Each notification is normalized to a shared `NativeEngagementEvent` (defined in the `message-processor` models module, imported by both services so the contract can't drift) and `XADD`ed to `engagement:twitch-native`. `ChannelID` is the **lowercase broadcaster login** — the exact key `overlay_chat_sources.channel_id` stores for Twitch — so the consumer fans out with no Helix id→login lookup. On XADD failure the webhook returns an error and skips its dedup key, so Twitch redelivers (at-least-once).

**Consumer** (`engagement-service/consumer/native.go`):
- A durable `XREADGROUP` (group `engagement-native`) resolves the event's overlays via `OverlaysForChannel('twitch', login)` and upserts a `source='twitch_native'` row **per overlay**, keyed by the migration-070 partial unique index `(overlay_id, source, external_id)` — one Twitch round fans out to every overlay sourcing that channel. State mapping is pure and unit-tested (`begin/progress→ACTIVE`, `lock→LOCKED`, `end→RESOLVED`|`CANCELED`, poll `end→CLOSED`). Aggregates land in `poll_options.mirror_votes` / `prediction_outcomes.mirror_points`/`mirror_entrants`. The refreshed row is broadcast through the **existing** publisher, so the overlay/monitor/widgets update with no new plumbing.

**Currency isolation (the load-bearing invariant):**
- Native rounds **never** set the `engagement:active:{platform}:{channel}` flag, so the message-processor hot path never forwards chat commands for them.
- The chat-command and viewer-web write paths resolve their target via `GetActivePoll`/`GetActivePrediction`, which stay **`source='allchat'`-only** — a native round is unreachable by any vote/wager/ledger code.
- `RecordVote` (like `Wager`) rejects any poll whose `source != 'allchat'`, so even a hand-crafted `WebVote` at a native poll id can't insert a `poll_votes` row and corrupt the mirrored tally.
- Public rendering uses new `GetActiveDisplayPoll`/`GetActiveDisplayPrediction` (either source), keeping the write path and the display path deliberately separate.
- Read tallies are `computed_from_votes/entries + mirror_*`: an All-Chat row has `mirror_* = 0`, a native row has no vote/entry rows, so the sum is exact for both without a branch.

**Precedence.** While a native round is live, the All-Chat create endpoints 409 (`HasLiveNativePoll`/`HasLiveNativePrediction`). The reverse ordering — a Twitch round *beginning* while an All-Chat round is already running (Twitch is external, so this needs no race) — is resolved at **display** time: the display queries prefer the **All-Chat** round. That round can hold real wagered viewer points and MUST stay resolvable from the control panel, so it can never be shadowed by a mirror; once it ends, the native round shows. (Native votes/wagers happen in Twitch's own UI regardless of what the overlay shows.)

**Ordering safety.** EventSub does not guarantee ordered delivery, and a transiently-failed webhook is redelivered later, so the mirror upserts are **monotonic**: a state guard blocks a backward transition (a late `progress` can't reopen a `CLOSED`/`RESOLVED`/`CANCELED` round — which would otherwise strand a phantom live round that blocks All-Chat creation and is preferred for display forever), tallies use `GREATEST` (Twitch counts only grow within a round), and deadline/lock/resolve timestamps `COALESCE` so a later event can't null an earlier one. A blocked stale event is a no-op and is not re-broadcast.

**Synthetic-cancel exception (P2-4).** The 4h stale-sweep force-`CANCEL`s a native round stuck live past the TTL, but a `LOCKED` Twitch prediction has no forced-resolution deadline — only its *betting window* is capped — so the sweep can fire *before* the genuine `channel.prediction.end`. Such a sweep cancel is tagged `sweep_canceled = TRUE` (synthetic); a real mirror event always carries `sweep_canceled = FALSE`. The absorbing guard therefore lets a *genuine* terminal override a *synthetic* cancel (re-tagging the row authoritative and recording the real winner), so the overlay shows the true result instead of a permanent wrong `CANCELED`. Genuine `RESOLVED`↔`CANCELED` lateral flips stay blocked.

### Consequences

- **Positive**: Overlays reflect native Twitch rounds with zero new render/broadcast/type surface — the `source` discriminator does all the work. Points integrity is structural (native rows are unreachable by ledger code), not merely conventional. Opt-in + non-fatal scope errors mean the feature is invisible to streamers who don't want it and costs nothing on channels that haven't granted it.
- **Negative / trade-offs**: In the rare both-live state, the simultaneously-live *native* round is hidden on the overlay until the All-Chat round ends (display prefers the points-bearing All-Chat round); this is non-destructive and the create-side 409 keeps it rare. Mirrored tallies refresh only as fast as Twitch sends `progress` events. The 7 extra subscriptions per opted-in channel add to the Twitch subscription budget (uncosted in-code today, same as the existing 8 types).
- **Known limitation**: the 7 subscriptions are created in the listener's per-channel `subscribe` action, which — like every other event subscription (subs/bits/follows) — fires once when a channel is first tracked. A streamer who grants the `engagement` scope *after* their channel is already tracked won't be mirrored until the next channel (re)sync (leader change, pod restart, or the channel being re-added). This is the pre-existing behavior of the whole event-subscription layer, not new to this feature.
- **Follow-ups**: reconcile event subscriptions (not just the chat sub) on every channel sync so a late scope grant activates without a restart; cost/`max_total_cost` tracking for the subscription budget; surfacing "mirroring enabled" state in the streamer UI beyond the consent link.

## Related

- **ADR-0028** — engagement chat-command write-path (the All-Chat participation path native mirroring sits beside).
- **ADR-0029** — viewer points economy; first stated that native predictions mirror state only. This ADR enforces that end-to-end.
- **ADR-0012 / ADR-0017** — the opt-in Twitch re-consent scope-bundle pattern reused for the `engagement` action.
- **ADR-0015** — IRC↔EventSub split; the EventSub listener this rides on.

(ADR numbering is shared with caesar-deployment, where 0020–0022 live, so this is 0029.)
