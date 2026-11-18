# ADR-0059: Picarto chat via the unofficial pop-out WebSocket feed

**Date**: 2026-09-08
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Picarto support (Phase 3 of the platform expansion plan) shipped without an
official chat API. Picarto
publishes a REST API for channel metadata (https://api.picarto.tv) but no
public way to follow chat. What exists is the pop-out chat page
(https://picarto.tv/chatpopout/<channel>) which, like every Picarto client,
reads chat over a WebSocket.

Two reference implementations parse this same feed, both by riding the
browser client rather than a documented protocol:

- Social Stream Ninja opens the pop-out page and scrapes the rendered DOM
  (`sources/picarto.js` — a MutationObserver over `[class*='ChannelChat__MessageBoxWrapper']`).
- PicaBot (github.com/NobreHD/PicaBot) connects to the chat WebSocket itself
  and documents the server's destructure: the batch frame `{"t":"c","m":[…]}`,
  and the per-message fields `c` `rn` `rc` `a` `id` `m` `u` `n` `k` `i`.

Neither is official, and Picarto ships its client as one minified React
bundle, so the protocol surface can change in any deploy without deprecation
notice or a changelog.

## Spike findings (2026-09-08, verified same day)

- **Endpoint**: `REACT_APP_CHAT_URL = "wss://chat.picarto.tv/chat/token=:token"`,
  a build constant in Picarto's production bundle.
- **Token**: the pop-out obtains a read token by POSTing a GraphQL query
  (`generateJwtToken(channel_name) { key }`) to Picarto's internal API
  `https://ptvintern.picarto.tv/ptvapi`; no authentication needed, and the
  returned HS256 JWT carries `userId: 0` (anonymous viewer). Verified live.
- **Handshake**: a raw HTTP Upgrade returns `101 Switching Protocols` (uWebSockets
  behind Cloudflare); no client hello needed, the server starts pushing
  immediately.
- **Message shapes** from a live capture: chat batches
  `{"t":"c","m":[{"t":"c","c","u":<user id>,"n":<username>,"rn":<display name>,
  "i":<avatar>,"m":<text>,"id","d","k","rc":""}]}` (fields matching
  PicaBot's destructure; emotes arrive as `:name:` shortcodes inside `m`),
  legacy stream metadata frames, and user-list updates `{"t":"ur","m":...}`.
- **Write path**: anonymous sends are rejected (`INVALID_USER`); reading is
  free. All-Chat only reads.

## Decision

**Picarto chat is read over the unofficial pop-out WebSocket**, grounded as
above, with four load-bearing constraints:

1. **One small client, one file for the parser.** The entire protocol
   interpretation lives in `services/picarto-listener/websocket/` (the
   envelope switch in `client.go`, the shapes in `types.go`). When Picarto
   changes the feed, the fix is a one-file change plus a re-run of the spike —
   nothing elsewhere in the service knows the wire format. The token fetcher
   (`token/fetcher.go`) is the only other file that touches a Picarto endpoint.
2. **Defensive parsing, log-and-drop.** Any frame that fails to unmarshal, or
   carries an unknown discriminator, is dropped with a Debug log and a
   dropped-message metric — never a panic, never a crash loop. Extending the
   listener to a new message type is additive: a new case in one switch.
3. **Emotes are out of scope.** Shortcodes pass through to the normalizer,
   which strips them to plain text. Rendering them would mean following
   Picarto's emoji CDN contract — a second undocumented surface — for casing
   value.
4. **The risk is accepted knowingly, and priced into the feature gate.**
   `platform_picarto` seeded `is_premium = TRUE` (migration 093): uptake was
   held back until the feed had been observed stable in production, because a
   silent Picarto change degrades a *paying* overlay's chat silently. The gate
   flips via the admin endpoint without redeploy when confidence is earned.
   The frontend still hides Picarto outside beta.allch.at
   (`BETA_ONLY_PLATFORMS` in `frontend/src/lib/platform-availability.ts`);
   release means removing it there and turning the gate's `is_premium` off in
   `/admin/features`.

## Consequences

- A Picarto client deploy can break chat with no warning. Detection is by
  dropped-message metrics and the listener's reconnect loop, not by an error
  surfaced to the streamer. This is the accepted maintenance cost of the
  unofficial route.
- The listener polls `overlay_chat_sources` every 30 s (no LISTEN/NOTIFY
  watcher): source changes take up to 30 s to connect, which is fine for a
  chat source joined at overlay-edit time.
- Live verification was performed only through the spike captures; the
  service's own end-to-end run against a live channel is pending (see the
  service README's re-spike instructions).
