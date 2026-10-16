# ADR-0069: A stream session ends only when no gateway pod holds the overlay, after a 15-minute grace

**Date**: 2026-10-16
**Status**: Accepted
**Deciders**: caesarakalaeii

(ADR numbering is shared with the caesar-deployment repo. If 0069 is already
taken there, renumber this file; the slug is the stable identifier.)

## Context and Problem Statement

The credit roll measures a stream by its session, the `session:active:{overlay}`
Redis hash that api-gateway creates on the first WebSocket connect. Message
processor captures leaderboard events under the same session ID. A streamer
reported "Duration: 2 hours 48 minutes" on a 4h18 stream.

The session ended mid-stream in two ways:

- **Per-pod grace timers on a shared session.** api-gateway runs at least three
  replicas. Each pod starts a 60-second grace timer when *its own* pool for an
  overlay empties, then deleted the shared hash, even while the OBS browser
  source stayed connected to a different pod. Closing a dashboard tab was
  enough to end the stream's session.
- **The 60-second grace is shorter than ordinary interruptions.** An OBS scene
  switch with "shutdown source when not visible", a PC sleep or a network drop
  longer than a minute started a new session.

Each split also restarted the leaderboards, so supporters from earlier in the
stream were missing from the credits.

## Decision

1. Each pod records that it holds an overlay in a Redis sorted set
   `session:holders:{overlay}` (member: pod name from `HOSTNAME`, score: last
   heartbeat). It adds itself on connect and on every connection heartbeat, and
   removes itself when its pool empties. A holder older than 10 minutes, the
   default `overlay:connected` TTL, is pruned, so a pod that crashed without
   cleaning up stops counting.
2. A pod only marks the session `ENDING` and only ends it when no holder remains.
3. Ending the session is decoupled from the 60-second WebSocket grace, which
   still publishes the `disconnected` event. The session ends after
   `SESSION_END_GRACE_SECONDS` (default 900) without a connection on any pod.
4. `ENDING` records `ending_at`. If the pod that owned the end timer died, the
   next connect after the grace ends the old session and starts a new one
   instead of resuming it into the next stream.
5. The credit roll measures a live session from the earlier of the session
   start and the overlay's live Twitch stream start (Helix `GET /streams`, with
   the app token overlay-manager already uses for clips). An overlay that was not
   loaded for part of the stream still reports the stream's length, and clips
   cover the whole stream. Leaderboards stay per session. Other platforms are not
   consulted yet.

## Considered Options

- **Use only the platform's stream start time** (Twitch `stream.online`, YouTube
  `actualStartTime`). Fixes the displayed number but not the leaderboard split,
  and multistream overlays have several start times. Adopted as a complement,
  see Decision 5.
- **Cross-pod check only, keep 60 seconds.** Fixes the multi-replica bug but
  still splits a stream on every scene switch longer than a minute.

## Consequences

- A stream keeps one session across tabs, pods and short drops; credits cover
  the whole stream.
- Two streams less than 15 minutes apart merge into one session. Tune
  `SESSION_END_GRACE_SECONDS` if that becomes a problem.
- Credits shown from a completed session (database fallback) now measure to
  `ended_at` instead of growing with the time since the stream started.
