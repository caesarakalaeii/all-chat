# ADR-0072: Goal and List Overlays — Demand-Gated Goal Polls in the Alert-Processor and Debounced Leaderboard Pushes

**Date**: 2026-10-16
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

ADR-0068 created the `goal` and `list` overlay kinds and decided the split:
goal counts that only a platform knows (followers, subscribers) are polled from
its API, counts that the event stream carries (bits, Super Chats) are summed
from events, and list overlays rank supporters from the alert-processor's Redis
sorted sets. Both kinds stayed unreleased (no renderer). Issue #955 asks for
the renderers, their configs, and the data paths behind them.

Five questions needed an answer:

1. **Where do goal polls run, and when?** A poll costs an API call per overlay
   per interval; on YouTube it costs Data API quota (ADR-0006).
2. **How do two alert-processor replicas avoid polling the same goal twice?**
3. **Whose credential reads the counts?**
4. **How does a live list learn about a new ranking** without a frame per
   cheer?
5. **How does the unauthenticated OBS page get its initial state?**

## Decision Drivers

- A poll nobody sees is waste, and on YouTube it burns shared quota.
- No new scheduler, no new leader election, no new service.
- Every YouTube Data API call goes through the shared reserve/confirm/rollback
  ledger, so youtube-quota-monitor (ADR-0023) sees it.
- An at-least-once stream must not double-count a goal or lose a list's last
  change.

## Decision

1. **Polls live in the alert-processor and only run while the overlay is
   connected.** A ticker (`GOAL_POLL_SECONDS`, default 60) lists active goal
   overlays with a `followers`/`subscribers` goal and an active source on the
   goal's platform, and polls only those whose `overlay:connected:{id}` key
   exists. That key is the api-gateway's demand signal, the one source-manager
   builds listener demand from; editor previews connect `passive` and never set
   it, so an open editor polls nothing.
2. **A per-overlay Redis lock, not leader election.** Before polling, a replica
   takes `SET overlay:{id}:goal-poll-lock NX EX (interval − 1s)`. Whichever
   replica ticks first polls; the other skips. The lock is one second shorter
   than the interval so its holder reliably wins it again on its next tick.
3. **The owner's own credential.** Twitch: `moderation-service/tokens.TwitchSource`
   (the ADR-0016 users-row-or-linked-token resolution, imported rather than
   copied), Helix `GET /channels/followers?broadcaster_id=…&first=1`, reading
   `total`. `total` needs only a user token; `moderator:read:followers` (part of
   every Twitch login since EventSub follows) only adds the follower list. A
   401 refreshes once and retries. YouTube: `shared/youtubetoken`, then
   `channels.list?part=statistics&id=…` (1 unit, the cheapest call that carries
   a subscriber count), reserved, confirmed or rolled back through
   `shared/quota.Reserver` exactly like the innertube subscriber poller. No
   credential, a revoked one, or a 401 after the refresh writes
   `overlay:{id}:goal-status = reconnect`; a hidden subscriber count writes
   `hidden`. The editor shows both; the bar keeps its last count.
4. **Counter goals count once per alert.** For a goal overlay whose metric is
   `bits` or `super_chat`, the alert-processor claims and `INCRBYFLOAT`s
   `overlay:{id}:goal:{metric}` in one Lua script (the leaderboard's per-alert
   claim pattern) and pushes a `goal` frame with `starting_value + total`. Only
   events after the goal was configured count; "reset progress" deletes the
   counter.
5. **Lists push debounced `list_update` frames.** A scored board is marked
   dirty; a flush every second claims `SET overlay:{id}:list-push:{category}
   NX PX 2000` and, if won, publishes the board's top 25. A replica that loses
   the window keeps the board dirty and sends on a later tick, so the cluster
   sends at most one frame per board per two seconds and the final change is
   never dropped. Display names live in `overlay:{id}:leaderboard:{category}:names`,
   written in the scoring script, with the board's TTL.
6. **Public reads seed the renderer.** `GET /overlays/public/:id/goal` and
   `GET /overlays/:id/list/top` (both unauthenticated, 404 for other kinds) give
   the OBS page its first state; the WebSocket frames keep it current. Both
   frames ride `overlay:{id}:alerts`, and the gateway forwards them under their
   own WS types (`goal`, `list_update`) without an alert-profile lookup. The
   page fetches them only once the public config names the kind, so a chat
   overlay pays nothing.
7. **Release.** Creating goal and list overlays is gated like alerts
   (ADR-0008): `goal_overlay` (migration 110) and `list_overlay` (migration 111),
   seeded `is_premium=TRUE`. Configuring an existing one is free.

## Consequences

### Positive

- No poll runs for an overlay nobody has open in OBS, and none runs twice.
- YouTube goal polls appear in the shared quota table, the
  `listener_quota_usage_percentage` gauge and its alerts with no monitor change.
- Goal and list state survives a pod restart: it lives in Redis, not in a
  replica's memory.

### Negative

- The alert-processor now needs the token cipher and the Twitch and YouTube
  app credentials. Without the cipher no goal is polled (logged at start).
- It imports `moderation-service/tokens` across a service boundary, as it
  already imports the message-processor's normalizers. Moving `TwitchSource`
  to `shared/` is the upgrade if a third caller appears.
- YouTube rounds public subscriber counts to three significant figures, so a
  large channel's bar moves in steps.
- Counter goals sum Super Chat amounts across currencies, like the
  leaderboard does.
- A goal counter has no TTL; a Redis wipe resets counted progress to the
  starting value.

## Alternatives Considered

- **Source-manager leadership or a dedicated poller service.** Correct, but a
  new deployment or a new coordination path for one call per minute per
  connected goal. The per-overlay lock is ten lines.
- **Poll on `overlay:connections` connect events.** Misses overlays connected
  before a pod started and needs its own state; the key is already the source
  of truth (source-manager reconciles against it for the same reason).
- **`subscriptions.list?myRecentSubscribers` for the YouTube count.** Returns
  recent subscribers, not a total.
- **A `list_update` per scored event.** A cheer storm would flood every list
  socket; the issue asks for at most one per board per two seconds.

## Related

- ADR-0068: overlay kinds, the alert-processor, leaderboards.
- ADR-0070: the `overlay:{id}:alerts` delivery path.
- ADR-0006 / ADR-0023: YouTube quota accounting and its monitor.
- ADR-0016: linked Twitch credential resolution.
- ADR-0008: feature gates.
