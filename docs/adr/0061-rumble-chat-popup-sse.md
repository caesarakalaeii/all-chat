# ADR-0061: Rumble chat via the internal chat pop-up SSE endpoint (withdrawn)

**Date**: 2026-09-08
**Status**: Withdrawn (2026-10-08). Rumble support was removed: there is no API that can read arbitrary channels, and Rumble's political affiliations are ones we do not want to support.
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context and Problem Statement

Rumble was the largest platform in the expansion plan without a listener.
The product model requires reading chat from **arbitrary channels**
— any channel a streamer adds as a source — which gated the spike before any
code was written.

Two candidate routes existed:

1. **The official Live Stream API** (API key, documented at
   rumble.support and consumed end-to-end by Social Stream Ninja's
   `sources/websocket/rumble.js` bridge). The API URL is generated per
   creator from `rumble.com/account/livestream-api` and embeds the stream
   key; SSN's own docs warn to treat it like a secret. It reports only the
   creator's own streams. **It cannot address arbitrary channels, so it was
   ruled out by the spike.**
2. **Rumble's internal chat API** — the same one the official
   `rumble.com/chat/popup/<chat_id>` page drives.

## Spike Findings (2026-09-08, live)

Grounded in Social Stream Ninja's adapter source
(`steveseguin/social_stream`, `sources/websocket/rumble.js`) and the
AxelChat issue tracker (discussions #783, #253; AxelChat is closed-source,
so it corroborates behaviour rather than implementation),
and verified directly against production:

- **The endpoint is SSE, not WebSocket.** `GET
  https://web7.rumble.com/chat/api/chat/<chat_id>/stream` with
  `Accept: text/event-stream`. Rumble's own pop-up chat client opens an
  `EventSource` on exactly this URL. There is no WebSocket route for chat
  reads.
- **Anonymous reads work.** A bare request with no cookies returned a live
  SSE stream for a 24/7 channel, complete with the `init` history event
  (50 messages, users, badge config) and live `messages` batches. Rumble's
  own client sends credentials (`EventSource(..., {withCredentials: true})`)
  because the *page* also sends messages; reading does not need them. A
  session cookie is therefore optional (`RUMBLE_SESSION_COOKIE`), not
  required.
- **Message shape** (verified from a live capture, pinned as test fixtures
  in both the listener and the normalizer):
  - `init`: `{chat: {id}, messages[], users[], channels[], config:
    {badges, rants levels}}` — history plus catalog, once per connect.
  - `messages`: `{messages[], users[], channels[]}` — live batches with
    `request_id`. Message entries: `id, time (RFC3339), user_id, text,
    blocks[], type` (`"regular"`), optional `rant {price_cents, duration}`,
    `is_deleted`. User entries: `id, username, link, image.1, is_follower,
    color, badges[]`.
  - The users array is per-batch — sender name, colour, avatar and badges
    live there, not on the message. Deletions arrive as `is_deleted`
    messages.
  - Keepalives are SSE comment lines (`: -1`).

## Decision (2026-09-08, since withdrawn)

Route 2 was adopted: a `rumble-listener` read the internal pop-up SSE
endpoint, using the numeric `chat_id` as the stored channel identifier
(the listener validated the id is numeric and never called a
channel-resolution API, there being none that is not behind Cloudflare for
bots), behind the `platform_rumble` feature gate seeded by migration 095.
Accepted knowingly:

- **ToS grey zone.** The endpoint is undocumented and unauthenticated reads
  of an undocumented internal API sit outside any written permission.
  Read-only, low rate (one long-lived HTTP/2 GET per subscribed channel,
  not one shared multiplexed socket like Kick/Pusher), no message sending,
  standard browser headers: this matches what the broadcast-viewer page
  itself does, but it can break or be shut at any time.
- **Bundled-session identity is not forwarded by default.** A provided
  `RUMBLE_SESSION_COOKIE` is sent as-is to preserve follower-scoped detail;
  without it Rumble may filter some data for anonymous readers.
- **Deletions, rants and raids degrade gracefully.** Rants surface as
  donation events (price/duration); raids, pins, bans and other stray SSE
  event types are logged and dropped.

On 2026-10-08 the support was withdrawn for the reasons in the Status
line, and the listener, its normalizer and migration 095 were removed.

## Current State

Rumble is not a supported platform. There is no `rumble-listener` service in
`services/` and no normalizer in message-processor. The remaining artifacts
of the experiment are this ADR and migration 099, which is idempotent: on
databases that never ran migration 095 the DELETEs are no-ops, and its
down migration is a no-op because the deleted source rows cannot be
restored. Bringing Rumble back would require a new ADR and a new spike; the
credible route for arbitrary channels was the undocumented internal pop-up
endpoint, which sits in a ToS grey zone and can break or be shut at any
time.

## Consequences and Maintenance Expectation

- Rumble sources added by beta users before withdrawal are deleted by
  migration 099; those streamers lose their overlay feed (the down
  migration cannot restore it), which was accepted as the price of the
  withdrawal.
- Adding Instagram (ADR-0062), Altstream (ADR-0082) and the alerts/list/goal
  platform did not depend on any Rumble-planned code; nothing else in the
  tree carries Rumble-specific code or tables.
