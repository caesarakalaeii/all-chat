# ADR-0065: Unlisted and members-only YouTube streams: pinned links and a premium official-API listener

**Date**: 2026-10-10
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

YouTube chat is ingested by `youtube-listener-innertube`, which finds a channel's live stream by
browsing the channel's `/streams` tab. Two kinds of stream never work there:

- **Unlisted streams** are not listed on the channel page, so discovery never finds them, even
  though their chat is readable by anyone who has the video id.
- **Members-only streams** are found, but their chat is only readable by a signed-in member.
  InnerTube polls anonymously.

A user reported the unlisted case. The quota-based Data API listener (`services/youtube-listener`)
was taken out of the deployment in March 2026 and ADR-0023/ADR-0025 record the decision not to
bring it back for every channel. The project's YouTube Data API quota is now 1,009,000 units per
day and mostly unused.

## Decision Drivers

- Unlisted streams should work for everyone, at no quota cost.
- Members-only chat can only be read with the channel owner's own credentials, which means the
  Data API.
- The InnerTube decision stays the default: quota is spent only where it buys something InnerTube
  cannot do.
- A streamer who stops qualifying must fall back to today's behaviour, not lose chat.
- One channel is served by exactly one listener at a time.

## Considered Options

1. **Revive the Data API listener for every channel.** Rejected for the reasons in ADR-0023: it
   spends quota on streams InnerTube serves for free, and its `search.list` discovery (100 units)
   only finds public streams, so it would not even fix the unlisted case.
2. **Pinned stream link only.** Free and enough for unlisted streams, but it cannot read
   members-only chat.
3. **Pinned stream link for everyone, plus an opt-in premium official-API mode.** Chosen.

## Decision Outcome

**Pinned stream link (free).** A YouTube source can store a video id in `config.stream_id`
(`PATCH /overlays/:id/sources/:source_id`). overlay-manager accepts a watch, `youtu.be`, `/live/`
URL or a bare id, and rejects a video that oEmbed attributes to a different channel. innertube
tries the pinned video on every discovery attempt before browsing the channel page, so a stale pin
never hides a public stream. Pin changes reach the listener within about a minute (source-manager's
30s registry refresh plus innertube's 30s sync).

**Official-API mode (premium, gate `youtube_official_api`).** A source can set
`config.official_api: true`. overlay-manager checks the gate and that the caller has a YouTube
token row for the channel; that check is for user feedback only. The revived `youtube-listener`
is authoritative:

- It only considers sources that opted in, whose overlay owner holds a token row for the channel
  and is premium (or the gate is free), on an active overlay of a non-banned user. Eligibility is
  re-read on every sync, so losing premium drops the channel back to InnerTube with the opt-in
  kept for later.
- It proves ownership with `channels.list(mine=true)` on the owner's token. A token row alone is
  not proof: overlay-manager's add-by-link path copies an admin's own token under the channel id
  added, and before that was restricted to admin sessions it did so for every user.
- It discovers broadcasts with `liveBroadcasts.list broadcastStatus=active broadcastType=all` on
  the owner's token, which returns unlisted, members-only and "Stream now" broadcasts, and reads
  chat over the gRPC `liveChatMessages.streamList`.
- It claims each channel it serves with a TTL key `youtube:official:claim:{channelID}` (3 minutes,
  the ADR-0015 pattern), but only while one of the opted-in owner's overlays is connected: a claim
  takes the channel away from innertube for every overlay on it. innertube skips claimed channels
  and stops what it was polling on them. Claims are refreshed by the global-sync leader every 60s
  and released when a channel stops qualifying, when the owner's overlay disconnects (after the
  listener's 90s disconnect debounce), while the global quota state is Critical or worse, when a
  sync hits a token or auth error on the owner's credentials (the cached ownership verdict is
  dropped with it), and when a replica shuts down. The next sync on each side then hands the
  channel over, so a running stream moves between the listeners within about a minute instead of
  going dark. Discovery backs off to at most once a minute, so a go-live on a claimed channel is
  seen about as fast as innertube would see it.

## Consequences

### Positive

- Unlisted streams work for every user without spending quota.
- Members-only chat works for premium streamers.
- Any failure of the official path (listener stopped or crashed, token revoked or rejected, premium
  lapsed, quota pressure) releases the claim, or lets its 3-minute TTL expire after a crash, and
  InnerTube resumes.

### Negative

- A second YouTube listener is deployed again, with its own replicas and alerting surface.
- An opted-in channel costs quota while its owner's overlay is open and the channel is offline:
  at most one `liveBroadcasts.list` a minute, plus a `channels.list` per owner every six hours.
- A pin is per channel: when several overlays use the same channel, the earliest-created source
  with a pin wins.

## Implementation

- **Files**: `services/overlay-manager/handlers/sources.go`, `services/overlay-manager/youtube/resolver.go`,
  `services/youtube-listener-innertube/streams/manager.go`, `services/youtube-listener/`
  (`streams/`, `api/client.go`), `shared/youtubeclaim/`, `shared/featuregates/cache.go`,
  `migrations/102_youtube_official_api_gate.sql`, the YouTube source settings in the overlay editor.
- **Deployment**: the listener's Deployment has to be re-added in the deployment repository. Until
  it runs, no claims exist and InnerTube serves every channel.

## Related Decisions

- ADR-0006: YouTube quota reserve-confirm-rollback (the listener's quota accounting).
- ADR-0008: feature gates.
- ADR-0015: chat-ownership claim with TTL; the claim pattern reused here.
- ADR-0023, ADR-0025: the decision to run InnerTube only, amended by this record for opted-in
  premium channels.
