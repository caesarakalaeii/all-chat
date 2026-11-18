# ADR-0072: Goal and List Overlays: demand-gated goal polls and debounced leaderboard pushes

**Date**: 2026-10-16
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context

ADR-0068 created the `goal` and `list` overlay kinds and the split: platform-only counts
(followers, subscribers) poll the platform API, event-carried counts (bits, Super Chats)
sum from events, and lists rank supporters from the alert-processor's Redis sorted sets.
Both kinds needed renderers, configs and the data paths. Constraints: a poll nobody sees
is waste and on YouTube burns shared quota (all Data API calls run the
reserve/confirm/rollback ledger, visible to youtube-quota-monitor, ADR-0023); no new
scheduler, leader election or service; at-least-once must not double a goal or lose a
list's last change.

## Considered options

- Source-manager leadership or a dedicated poller service: a new deployment or
  coordination path for one call per minute per connected goal; the per-overlay lock is
  ten lines.
- Poll on `overlay:connections` connect events: misses overlays connected before a pod
  started; the demand key is already the source of truth (source-manager reconciles
  against it).
- `subscriptions.list?myRecentSubscribers` for YouTube: returns recent subscribers, not a
  total.
- A `list_update` per scored event: a cheer storm floods every list socket; one per board
  per two seconds is the ask.

## Decision

1. Polls run in the alert-processor only while the overlay is connected: a ticker
   (`GOAL_POLL_SECONDS`, default 60) lists active goal overlays with a
   `followers`/`subscribers` goal and an active source on that platform and polls only
   those with `overlay:connected:{id}` (the gateway's demand key; editor previews connect
   passive and never set it).
2. A per-overlay Redis lock, not leader election: `SET overlay:{id}:goal-poll-lock NX EX
   (interval - 1s)`; the first ticker wins, the next one shorter than the interval so its
   holder reliably renews.
3. The owner's own credential: Twitch via `moderation-service/tokens.TwitchSource` (the
   ADR-0016 users-row-or-linked-token resolution), Helix `GET /channels/followers`
   reading `total` (a user token suffices; a 401 refreshes once and retries). YouTube via
   `shared/youtubetoken` then `channels.list?part=statistics` (1 unit) reserved and
   confirmed or rolled back through `shared/quota.Reserver`. No credential, a revoked
   one, or a post-refresh 401 writes `overlay:{id}:goal-status = reconnect`; a hidden
   count writes `hidden`; the editor shows both and the bar keeps its last count.
4. Counter goals (`bits`, `super_chat`) count once per alert: the alert-processor claims
   and `INCRBYFLOAT`s `overlay:{id}:goal:{metric}` in one Lua script (the leaderboard's
   per-alert claim) and pushes `starting_value + total`; only events after configuration
   count; "reset progress" deletes the counter.
5. Lists push debounced `list_update` frames: a scored board is marked dirty; a 1s flush
   claims `SET overlay:{id}:list-push:{category} NX PX 2000` and publishes the top 25; a
   loser keeps the board dirty and sends on a later tick, so the cluster sends at most one
   frame per board per two seconds and the final change is never dropped. Display names
   live in `overlay:{id}:leaderboard:{category}:names` with the board's TTL.
6. Public reads seed the renderer: `GET /overlays/public/:id/goal` and
   `GET /overlays/:id/list/top` (unauthenticated, 404 for other kinds) give the OBS page
   its first state; both frames ride `overlay:{id}:alerts` and the gateway forwards them
   under their own WS types (`goal`, `list_update`) without a profile lookup. The page
   fetches only when the public config names the kind, so chat overlays pay nothing.
7. Creation is gated like alerts (ADR-0008): `goal_overlay` (migration 110) and
   `list_overlay` (migration 111), seeded premium; configuring an existing one is free.

## Consequences

- No poll runs for an overlay nobody has open in OBS, and none runs twice; goal/list
  state lives in Redis, so it survives a pod restart.
- The alert-processor now needs the token cipher and Twitch/YouTube app credentials
  (without the cipher no goal is polled, logged at start) and imports
  `moderation-service/tokens` across a service boundary (move `TwitchSource` to `shared/`
  if a third caller appears).
- YouTube rounds public subscriber counts to three significant figures; counter goals sum
  Super Chat amounts across currencies, like the leaderboard; a goal counter has no TTL,
  so a Redis wipe resets counted progress.

## Related

ADR-0068 (overlay kinds, leaderboards), ADR-0070 (delivery path), ADR-0006/0023 (quota
accounting and monitor), ADR-0016 (linked credentials), ADR-0008 (feature gates).
