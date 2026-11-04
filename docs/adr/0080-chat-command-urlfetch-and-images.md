# ADR-0080: Chat Command urlfetch and Images, With a Guarded Outbound HTTP Client

**Date**: 2026-10-22
**Status**: Accepted
**Revised**: 2026-11-04 (each command carries its own alert sound and text, migration 123)
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0079 shipped streamer chat commands but left out Nightbot's `$(urlfetch)`:
a server-side fetch of a URL the user picks is an SSRF surface. Streamers use
that feature for things the template variables cannot do, such as quotes, jokes
and stats from their own APIs, or a random dog picture.

Streamers also want a command to put an image on stream, for example `!hype`
showing a GIF. Twitch, Kick and YouTube only accept text from the send paths we
post through, so a picture can never reach their chats. It can only appear on
the streamer's own overlay.

The cluster leaves egress open (`caesar-deployment`
`apps/workloads/all-chat/network-policies.yaml`: "Egress is left open"). Any pod
can reach any service, the nodes' private networks and the cloud metadata
endpoint. Whatever stops a user-chosen URL from going there has to live in the
code that dials it.

## Decision Drivers

- **No internal reach.** A user-chosen URL must never reach a pod, a service, a
  private range or the metadata endpoint. That includes after DNS rebinding and
  after a redirect.
- **No new writer to a streamer's chat.** Only the server may decide what is
  posted from a streamer's account.
- **Chatters cannot pick what is on stream.** An image comes from the streamer's
  template, never from chat text.
- **Bounded cost.** Fetches add latency and run on our IP, so their number,
  time and size are capped.

## Considered Options

### Where urlfetch and `{js}` run

1. **In the streamer's overlay (browser).** The overlay would run the code and
   send the reply text back.
   - ✅ The fetch never leaves the streamer's machine, so the server has no SSRF.
   - ❌ `/overlay/[id]` is public and its socket is anonymous (#1094). Anyone
     holding the overlay URL could answer a job and make the streamer's account
     post anything. Overlay URLs leak on stream and in share links. Closing that
     needs a new secret executor token, and leaking that token is then account
     impersonation.
   - ❌ The overlay runs on allch.at. Streamer JS there runs in the browser of
     everyone else who loads that overlay: the dashboard preview, delegated
     moderators (ADR-0048), admin impersonation. That is stored XSS. A safe
     version needs an opaque-origin sandbox iframe plus a Worker to kill
     runaway code, and the CSP allows no `'unsafe-eval'`.
   - ❌ Commands only answer while OBS has the overlay open. Two open copies
     both run, so duplicate replies have to be dropped.
   - ❌ A chatter-chosen URL is fetched from the streamer's home network. It
     can reach local tools there, such as a Streamer.bot HTTP server.
2. **On the server, behind a guarded client** (chosen).
3. **Rely on an egress NetworkPolicy.** Rejected as the only control: egress is
   open on purpose today. A policy is per pod and engagement-service still
   needs Postgres, Redis and auth-service, so it does not help an in-code
   fetch. It stays a defence-in-depth follow-up (RESIDUALS ENG2).

### Where a command's image shows

1. **A pop-up on the streamer's alerts overlays** (chosen). This reuses
   ADR-0070/ADR-0075: per-event-type duration, sound, text and custom code.
2. **Inline in the chat overlay.** The text reply also echoes back from the
   platform, so the image would be its own entry next to it. There is no
   styling or timing control for it.
3. **A link in the chat reply.** This already works with `{urlfetch}` and
   needs no feature, but it puts nothing on stream.

### How one command's alert differs from another's (added 2026-11-04)

Streamers want each command to be its own alert (`!dog` barks, `!hype` plays
an air horn), and a sound alone to be enough. One `chat_command` profile per
overlay cannot do that.

1. **The command carries its alert** (chosen). Next to its image, a command
   stores a sound and an alert text, and the alert envelope overrides the
   profile's, exactly as the image already did. The `chat_command` profile
   stays the shared style: duration, volume, custom code, and the text when a
   command has none. One migration, no change to the profile table, the
   gateway or the overlay.
2. **Named `chat_command` profiles in the alerts editor**, picked per command.
   Rejected: profiles are per overlay while a command fires on every alerts
   overlay of the streamer, so the link would have to span overlays, and the
   profile table's `(overlay_id, event_type)` key would change.

## Decision Outcome

### `shared/safehttp`

`shared/safehttp` is the client for user-chosen URLs.

- **Dial-time check.** Its dialer's `Control` hook refuses every IP that is not
  public unicast, after DNS resolution. That covers DNS rebinding and each
  redirect hop. Refused addresses are:
  - private and loopback ranges
  - link-local, which includes `169.254.169.254`
  - CGNAT, `0.0.0.0/8`, NAT64, multicast and broadcast
  - IPv4-mapped forms of all of these
- **Client.** `safehttp.Client` uses that dialer, never uses a proxy (through a
  proxy, the check would see the proxy's address instead of the target's) and
  follows at most 3 redirects. `safehttp.DialContext` serves websocket dialers.
- **Owncast listener.** It uses the client for `POST /api/chat/register` and
  the dialer for the chat socket. The Owncast instance URL is user input
  (ADR-0058) and was dialed unguarded.

### `{urlfetch URL}`

- **What it inserts.** The response body, trimmed, at most 32 KB.
- **URL rules** (`chatcmd.CheckFetchURL`). The URL must be https on the default
  port, with no credentials and at most 2048 bytes. The same rules apply to
  every redirect hop. Port 443 only keeps the nodes' own public IPs, which are
  public unicast, to the ingress everyone can already reach.
- **Escaping.** Every variable inside the URL is query-escaped, so chat text
  cannot change the host, the path or add parameters.
- **Limits.** At most 3 fetches per render, 3 s each, sent with an
  `All-Chat-URLFetch` User-Agent.
- **Inside `{js}`.** A `{urlfetch}` in a `{js}` block is fetched before the
  code runs. It is inserted as a JSON string literal, so the body stays data,
  or as `null` when the fetch failed. So
  `{js JSON.parse({urlfetch https://…}).value}` works. The QuickJS VM still
  has no host functions.
- **JS budget.** The budget now counts only time spent in `Eval`, so a slow
  fetch between blocks does not use up the 100 ms.
- **Preview.** The editor preview really fetches, bounded the same way and by
  the authenticated route's per-user rate limit (`ENGAGEMENT_RATE_PER_MIN`).

### Images, sounds and alert text

- **Storage.** `chat_commands.image_url` (migration 114) is a template of at
  most 1000 characters. `sound_url` (migration 123) is a plain absolute https
  URL of at most 1000 characters (usually an upload to `media.allch.at`), and
  `alert_text` (migration 123) is a template of at most 500 characters. Any one
  of the three alone is enough for a command to be valid.
- **Rendering.** It is rendered with the chatter's input withheld: `{user}`,
  `{query}` and the `{1}`…`{9}`/`{touser}` arguments and their JS globals are
  empty (`chatcmd.ImageURL` in
  `services/engagement-service/chatcmd/fetch.go`). It must come out as an
  absolute https URL or nothing is shown. `{urlfetch}` and
  `{js}` work, so `!dog` can show an image from an API. The alert text is
  rendered like the reply, with the chatter's input, and sent final. A sound
  is not a template.
- **Delivery.** When a command with an image, a sound or an alert text fires,
  engagement-service
  publishes a `chat_command` alert (`shared/alerts`) on `overlay:{id}:alerts`
  for each of the owner's alerts overlays. Like a clip, it fires from any chat
  in the streamer's overlays, including TikTok and Discord, because it shows on
  the streamer's own stream.
  An image that does not render to https drops only the image; the alert
  still fires when it has a sound or a text.
- **The alert.** `Alert.image_url`, `Alert.sound_url` and `Alert.text`
  override the profile's image, sound and rendered text in `alerts.Resolve`.
  The profile still sets duration, volume and custom code, and the text when
  the command has none. The default text is `{username} used {amount}`:
  - `{amount}` is the trigger.
  - `{count}` is the use count.
  - `{message}` is the reply, or what the chatter typed when there is no reply.
- **Visibility.** The alerts editor offers the `chat_command` event type only
  where the chat commands feature is available: every host for early-access
  users, and beta.allch.at for everyone (`EARLY_ACCESS_FEATURES`).

## Consequences

- **Fetches from our IP.** Our servers fetch third-party URLs for streamers. A
  fetch-heavy command adds up to about 9 s to its reply. Abuse complaints would
  name our IP. The User-Agent identifies the feature, and per-command cooldowns
  plus the preview rate limit bound the volume.
- **Images need an alerts overlay.** Creating one is behind the
  `alerts_overlay` premium gate (migration 108), so for most accounts images
  are effectively premium today.
- **The guard is in code.** Any new code that dials a user-chosen URL must use
  `shared/safehttp`. overlay-manager's Owncast name lookup
  (`SourcesHandler.httpClient`) is never set in production, so it never
  fetches. Wiring it later must use `safehttp.Client`.
- **Local development.** A local Owncast instance on a private address is now
  refused by the listener.
- **Staging.**
  - The owncast-listener runs from `:beta`, so its fix ships with the beta
    merge.
  - urlfetch and images need engagement-service, overlay-manager (it saves the
    `chat_command` alert profile) and the api-gateway (it resolves the image
    through `alerts.Resolve`) on `:main`, like the rest of ADR-0079.
