# ADR-0076: Credit Roll Clips Play the Clip MP4 from Twitch's Web GQL

**Date**: 2026-10-17
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context

The credit roll (`/overlay/:id/credits`) plays the session's Twitch clips full-screen
behind the scrolling credits, and the streamer setting "Mute Clips Audio" has never
produced sound when off. The only official clip player is the clip embed
(`clips.twitch.tv/embed?...`), measured in Chromium against a real clip under OBS's
browser-source autoplay policy: it plays muted (`muted=true`, `volume=0`) whatever
`muted=` says, ignores the Twitch Embed SDK's postMessage commands, and resets a manual
unmute on every load (every clip is a new load, so a one-time "Interact" in OBS does not
stick). The Embed SDK itself takes `channel`, `video` or `collection`, never clips.
Constraints: the streamer turned sound on, so it should play; nothing degrades when the
workaround breaks; no new backend surface for a frontend playback problem.

## Considered options

- Look up the clip's signed MP4 via Twitch's web GQL API from the browser and play it in
  our own `<video>` (chosen).
- Keep the embed and tell streamers to unmute: resets every clip, so never more than one
  clip of sound.
- Do the GQL lookup in overlay-manager and return the URL with the clip list: the signed
  URL would be minted from and travel to different datacenter IPs, with no measured
  signal that is safe, plus backend changes and an outbound dependency in a service that
  does not need it.

## Decision

For the current clip, `fetchClipVideoUrl` (`frontend/src/lib/twitchClipVideo.ts`) POSTs to
`https://gql.twitch.tv/gql` with Twitch's public web client ID asking for the clip's
`playbackAccessToken { signature value }` and `videoQualities { sourceURL }`, then plays
the best quality's `sourceURL?sig=...&token=...` in a `<video>`, advancing on `ended`. The
video renders `muted` and takes `clips_muted` once data has loaded, so OBS plays it with
sound; an ordinary browser tab refuses unmuted autoplay and falls back to muted (verified
with the real page and OBS's autoplay flag: `muted=false, volume=1` there, muted without).
A failed lookup or video error plays that clip through the embed as before: muted,
advanced on a timer of its length plus 3 s. No user token needed, the endpoint answers
`Access-Control-Allow-Origin: *`, and the CSP already admits it (`connect-src https:`)
and the MP4 host (`media-src https:`), keeping `https://clips.twitch.tv` in `frame-src`
for the fallback.

## Consequences

- Clip audio works in OBS and clips advance on their real end; no player chrome or
  mature-content interstitial.
- The GQL API is undocumented: Twitch can change the query, client ID or token format
  without notice, in which case the lookup returns null and the roll silently falls back
  to the muted embed (sound lost, playback kept), the same class of unofficial-API
  dependency as InnerTube and the TikTok signer. Each clip costs one GQL request from
  the viewer's browser, even on replay.
