# ADR-0065: Unlisted and members-only YouTube streams: pinned links and a premium official-API listener

**Date**: 2026-10-10
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
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

Rejected: reviving the Data API listener for every channel (spends quota on
streams InnerTube serves free, and `search.list` discovery only finds public
streams, so it would not fix the unlisted case); a pinned link only (cannot
read members-only chat).

Chosen: a pinned stream link for everyone, plus an opt-in premium
official-API mode. InnerTube stays the default; quota is spent only where it
buys something InnerTube cannot do.

## Decision Outcome## Decision Outcome

**Pinned stream link (free).** A YouTube source can store a video id in `config.stream_id`
(`PATCH /overlays/:id/sources/:source_id`). overlay-manager accepts a watch, `youtu.be`, `/live/`
URL or a bare id, and rejects a video whose watch-page owner (InnerTube `/next`) is a different
channel. innertube tries the pinned video on every discovery attempt before browsing the channel
page, so a stale pin never hides a public stream. Pin changes reach the listener within about a
minute (source-manager's 30s registry refresh plus innertube's 30s sync).

**Official-API mode (premium, gate `youtube_official_api`).** A source can set
`config.official_api: true`. overlay-manager checks the gate and that the caller has a
`youtube_oauth_tokens` row for the channel, and fails closed when the check cannot run; the
listener's own verification is authoritative:

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
- It claims each served channel with a TTL key `youtube:official:claim:{channelID}`
(3 minutes, the ADR-0015 pattern), taking the channel away from innertube for
every overlay on it; innertube skips claimed channels. Claims refresh every
60s from the global-sync leader and release on: a channel ceasing to qualify,
the owner's overlay disconnecting (after the 90s debounce), Critical-or-worse
quota state, a token/auth error on the owner's credentials, or replica
shutdown, so a running stream moves between listeners within about a minute
instead of going dark. Discovery backs off to at most once a minute.

## Consequences

Unlisted streams work for everyone without spending quota; members-only chat
works for premium streamers. Any failure of the official path (listener
stopped, token revoked, premium lapsed, quota pressure) releases the claim or
lets its TTL expire, and InnerTube resumes. Costs: a second YouTube listener
deployed with its own replicas and alerting surface; an opted-in channel costs
quota while its owner's overlay is open and the channel is offline (at most
one `liveBroadcasts.list` a minute plus a `channels.list` per owner every six
hours); a pin is per channel, so with several overlays on the same channel the
earliest-created source with a pin wins.

## Implementation

- **Files**: `services/overlay-manager/handlers/sources.go`, `services/overlay-manager/youtube/resolver.go`,
  `services/youtube-listener-innertube/streams/manager.go`, `services/youtube-listener/`
  (`streams/`, `api/client.go`), `shared/youtubeclaim/`, `shared/featuregates/cache.go`,
  `migrations/102_youtube_official_api_gate.sql`, the YouTube source settings in the overlay editor.
- **Deployment**: the listener runs as `youtube-listener-official` (2 replicas for
  node-reboot coverage, ADR-0021 in caesar-deployment; only the global-sync lease
  holder claims and serves channels, the other is a warm standby) in the `allchat`
  namespace, deployment manifest `apps/workloads/all-chat/youtube-listener-official-deployment.yaml`
  in the GitOps repo. It runs with no Service: nothing calls it, Prometheus scrapes pod
  annotations, and `/admin/*` is reachable only by port-forward. Without opted-in channels it
  claims nothing and InnerTube serves every channel.

## Related Decisions

- ADR-0006: YouTube quota reserve-confirm-rollback (the listener's quota accounting).
- ADR-0008: feature gates.
- ADR-0015: chat-ownership claim with TTL; the claim pattern reused here.
- ADR-0023, ADR-0025: the decision to run InnerTube only, amended by this record for opted-in
  premium channels.
