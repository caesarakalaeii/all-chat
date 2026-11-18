# ADR-0080: Chat Command urlfetch and Images, With a Guarded Outbound HTTP Client

**Date**: 2026-10-22
**Status**: Accepted
**Revised**: 2026-11-15 (chat commands and alerts overlays are free; uploading a sound is premium via `media_upload`)
**Deciders**: caesarakalaeii

## Context

ADR-0079 shipped streamer chat commands but left out Nightbot's `$(urlfetch)`, a
server-side fetch of a user-chosen URL and thus an SSRF surface; streamers also want a
command to put an image or sound on stream, which only the streamer's own overlay can
show (the platforms' send paths carry text only). The cluster leaves egress open
(caesar-deployment network policies), so the control must live in the code that dials the
URL. Constraints: a user-chosen URL never reaches a pod, service, private range or the
metadata endpoint, including after DNS rebinding and redirects; no new writer to a
streamer's chat; chatters cannot pick what is on stream; fetch cost is bounded (they run
on our IP).

## Considered options

- Run urlfetch and `{js}` in the streamer's overlay (browser): no server SSRF, but the
  public overlay socket is anonymous, so anyone holding the URL could make the streamer's
  account post; streamer JS there also runs in every viewer of the overlay (stored XSS
  unless sandboxed, and the CSP allows no `eval`); and chatter-chosen URLs would be
  fetched from the streamer's home network.
- Run on the server behind a guarded client. Chosen.
- Rely on an egress NetworkPolicy alone: rejected, egress is open on purpose and the
  service needs Postgres, Redis and auth-service anyway; it stays defence-in-depth.
- Where the image shows: a pop-up on the alert overlays using the existing profile
  machinery (chosen) over inline chat (no styling/timing) or a chat link (nothing on
  stream). Per-command alerting: the command carries its own sound and alert text
  overriding the shared `chat_command` profile (chosen, migration 123); named profiles in
  the editor were rejected (profiles are per overlay while a command fires on all of them).

## Decision

**`shared/safehttp`** is the client for user-chosen URLs. Its dialer's `Control` hook
refuses every IP that is not public unicast after DNS resolution (private and loopback
ranges, link-local including `169.254.169.254`, CGNAT, `0.0.0.0/8`, NAT64, multicast,
broadcast, and IPv4-mapped forms), covering DNS rebinding and each redirect hop. The
client never uses a proxy and follows at most 3 redirects; `safehttp.DialContext` serves
websocket dialers. The owncast listener (its instance URL is user input, ADR-0058) uses
the client and dialer.

**`{urlfetch URL}`** inserts the response body, trimmed, at most 32 KB. The URL must be
https on the default port, no credentials, at most 2048 bytes, same rules at every
redirect hop; every variable inside is query-escaped so chat text cannot change the host
or path. At most 3 fetches per render, 3 s each, `All-Chat-URLFetch` User-Agent. Inside
`{js}` a `{urlfetch}` is fetched before the code runs and enters as a JSON string literal
(or `null` on failure), so `{js JSON.parse({urlfetch https://...}).value}` works; the JS
budget counts only `Eval` time. The editor preview really fetches, under the same bounds
and the per-user rate limit.

**Images, sounds, alert text.** `chat_commands.image_url` (migration 114) is a template of
at most 1000 chars; `sound_url` and `alert_text` (migration 123) are an absolute https URL
(usually an upload to `media.allch.at`) and a template of at most 500 chars; any one
suffices for the command to be valid. The image is rendered with chatter input withheld
(`chatcmd.ImageURL`, `services/engagement-service/chatcmd/fetch.go`) and must produce an
absolute https URL; the alert text is rendered with chatter input like the reply. When a
command with any of the three fires, engagement-service publishes a `chat_command` alert
on `overlay:{id}:alerts` for each of the owner's alerts overlays, from any chat in the
streamer's overlays. `Alert.image_url`, `Alert.sound_url`, `Alert.text` override the
profile's image, sound and rendered text in `alerts.Resolve`; the profile keeps duration,
volume, custom code, and the text when the command has none (default `{username} used
{amount}` where `{amount}` is the trigger, `{count}` the use count, `{message}` the reply
or the typed message).

## Consequences

- Our servers fetch third-party URLs for streamers; a fetch-heavy command adds up to
  about 9 s to its reply. The User-Agent identifies the feature; cooldowns and the preview
  rate limit bound volume.
- Images need an alerts overlay (free) and sound uploads are premium via `media_upload`
  (migration 128, ADR-0071); pasting a URL is free.
- Any new code dialing a user-chosen URL must use `shared/safehttp`; overlay-manager's
  Owncast lookup is never set in production so it never fetches, but wiring it later must
  use the client.
- A local Owncast instance on a private address is now refused by the listener.
- Staging: owncast-listener runs from `:beta`; urlfetch and images need engagement-service,
  overlay-manager and api-gateway on `:main`.
