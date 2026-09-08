# ADR-0061: Rumble chat via the internal chat pop-up SSE endpoint

**Date**: 2026-09-08
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Rumble is the largest of the five platforms in the expansion plan without a
listener. The product model requires reading chat from **arbitrary channels**
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
(`steveseguin/social_stream`, `sources/websocket/rumble.js` and
`docs/agents/08-platform-sources/rumble.md`), the AxelChat issue tracker
(discussions #783, #253 — AxelChat is closed-source, so it corroborates
behaviour rather than implementation), and verified directly against
production:

- **The endpoint is SSE, not WebSocket.** `GET
  https://web7.rumble.com/chat/api/chat/<chat_id>/stream` with
  `Accept: text/event-stream`. Rumble's own pop-up chat client opens an
  `EventSource` on exactly this URL. There is no WebSocket route for chat
  reads.
- **The `<chat_id>` is the numeric chat room id**, e.g. `445201096`. The
  official pop-up page accepts it as
  `rumble.com/chat/popup/445201096`, and the page's bootstrap script
  (`RumbleChat("https://web7.rumble.com/chat/api", ..., 445201096, ...)`)
  passes the same number to the same base URL.
- **Anonymous reads work.** A bare request with no cookies returned a live
  SSE stream for a 24/7 channel, complete with the `init` history event
  (50 messages, users, badge config) and live `messages` batches. Rumble's
  own client sends credentials (`EventSource(..., {withCredentials: true})`)
  because the *page* also sends messages; reading does not need them. A
  session cookie is therefore optional (`RUMBLE_SESSION_COOKIE`), not
  required.
- **Message shape** (verified from a live capture, pinned as test fixtures
  in both the listener and the normalizer):
  - `init`: `{chat: {id}, messages[], users[], channels[], config:` 
    `{badges, rants levels}``}` — history plus catalog, once per connect.
  - `messages`: `{messages[], users[], channels[]}` — live batches with
    `request_id`. Message entries: `id, time (RFC3339), user_id, text,
    blocks[], type` (`"regular"`), optional `rant {price_cents, duration}`,
    `is_deleted`. User entries: `id, username, link, image.1, is_follower,
    color, badges[]`.
  - The users array is per-batch — sender name, colour, avatar and badges
    live there, not on the message. Deletions arrive as `is_deleted`
    messages.
  - Keepalives are SSE comment lines (`: -1`).

## Decision

**Route 2, the internal chat pop-up SSE endpoint, adopted for reads.** The
chat pop-up id (`chat_id`) becomes the stored channel identifier; the
listener validates it is numeric and never calls a channel-resolution API
(there is none that is not behind Cloudflare for bots).

Accepted knowingly:

- **ToS grey zone.** The endpoint is undocumented and unauthenticated reads
  of an undocumented internal API sit outside any written permission.
  Read-only, low rate (one long-lived GET per channel), no message sending,
  standard browser headers: this matches what the broadcast-viewer page
  itself does, but it can break or be shut at any time.
- **Bundled-session identity is not forwarded by default.** A provided
  `RUMBLE_SESSION_COOKIE` is sent as-is to preserve follower-scoped detail;
  without it Rumble may filter some data for anonymous readers.
- **Deletions, rants and raids degrade gracefully.** Rants surface as
  donation events (price/duration); raids, pins, bans and other stray SSE
  event types are logged and dropped.

## Parser Isolation

Everything protocol-specific — endpoint construction, SSE framing, payload
decoding — lives in ONE file: `services/rumble-listener/websocket/parser.go`.
A rumble.com wire-format change is a one-file fix; tests pin the shapes
against the live capture, so a re-spike means re-running
`services/rumble-listener/websocket` tests against a fresh capture and
editing only that file plus fixtures.

## Consequences and Maintenance Expectation

- One HTTP/2 GET per subscribed channel (not one shared multiplexed
  socket like Kick/Pusher). Cheap, but a 100-channel pod holds 100
  streams; fine at the listener's HPA ceiling.
- No auth service changes, no OAuth, no overlay-manager write gating beyond
  the coordinator's central feature-gate enforcement (ADR-0008 seed in
  migration 095).
- **Expect breakage without notice**: when Rumble ships a chat upgrade this
  listener can go silent. The stale-connection liveness probe restarts
  pods, but a shape change needs the one-file re-spike above. Budget
  maintenance accordingly — this is the price of the largest unserved
  audience in the plan, and it was accepted deliberately.
- Verification status: protocol shape verified live on 2026-09-08 against
  production (`init` + `messages` for PSB's chat). Full end-to-end
  message-processor round trip verified in unit tests, not against live
  Rumble in CI. Re-verify with a live capture when touching the parser.
