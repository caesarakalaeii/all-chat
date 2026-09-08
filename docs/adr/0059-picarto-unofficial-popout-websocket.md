# ADR-0059: Picarto chat via the unofficial pop-out WebSocket feed

**Date**: 2026-09-08
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Picarto support (plan Phase 3) shipped without an official chat API. Picarto
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

## Spike findings (2026-09-08)

Everything below was verified the same day, from Picarto's own production
bundle and a live anonymous capture — not inferred.

- **Endpoint.** The production bundle (`/static/js/main.d543fe27.js`) carries
  the full socket map as build constants:
  `REACT_APP_CHAT_URL = "wss://chat.picarto.tv/chat/token=:token"`, alongside
  stream/feed/widget/dm/cms/notify sockets on the same hosts. Chat uses the
  chat path.
- **Token.** The pop-out obtains its read token by POSTing a GraphQL query to
  Picarto's internal API `https://ptvintern.picarto.tv/ptvapi`:
  `generateJwtToken(channel_name: $name) { key }`. No authentication is
  needed; the returned HS256 JWT carries `userId: 0` (anonymous viewer) and
  the target `channelName`. Verified live.
- **Handshake.** A raw HTTP Upgrade to `wss://chat.picarto.tv/chat/token=<jwt>`
  returns `101 Switching Protocols` (uWebSockets behind Cloudflare). No
  client hello is required; the server immediately starts pushing feed data.
- **Message shapes.** Live capture (60 s on a streaming channel) received:
  - chat batches: `{"t":"c","m":[{"t":"c","c":"<channel id>","u":"<user id>",
    "n":"<username>","rn":"<display name>","i":"<avatar path>","m":"<text>",
    "id":"<uuid>","d":1788880502781,"k":"ffc2dd","rc":"ffc2dd"}]}` — fields
    match PicaBot's destructure exactly. Emotes arrive as `:name:` shortcodes
    inside `m`.
  - stream metadata: `{"type":"stream","messages":{…viewers, multistream…}}`
    (legacy full-word type field, alongside the short `t` field).
  - user-list updates: `{"t":"ur","m":{"u":"…","n":"…"}}`.
- **Write path.** Anonymous sends are rejected with
  `{"success":false,"code":"INVALID_USER"}` — reading is free, posting needs
  an account. All-Chat only reads.

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
   `platform_picarto` seeds `is_premium = TRUE` (migration 093): uptake is
   held back until the feed has been observed stable in production, because a
   silent Picarto change degrades a * paying * overlay's chat silently. The
   gate flips via the admin endpoint without redeploy when confidence is
   earned.

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
