# ADR-0076: Credit Roll Clips Play the Clip MP4 from Twitch's Web GQL

**Date**: 2026-10-17
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

The credit roll (`/overlay/:id/credits`) plays the session's Twitch clips
full-screen behind the scrolling credits. The streamer setting "Mute Clips
Audio" (`clips_muted`) has never produced sound when turned off.

The clips came from Twitch's only official clip player, the clip embed
(`clips.twitch.tv/embed?clip=…&autoplay=true&muted=false`). Measured in
Chromium 153 on 2026-10-17 against a real clip:

- With OBS's browser-source autoplay policy
  (`--autoplay-policy=no-user-gesture-required`, set by obs-browser
  `browser-app.cpp`), the embed plays, but its player forces `muted=true` and
  `volume=0` on every load, whatever `muted=` says and whether or not the
  iframe has `allow="autoplay"`.
- The embed ignores the postMessage commands the Twitch Embed SDK uses for its
  player (`setMuted`, `setVolume`).
- Clicking its unmute button works, but the next load is muted again. Every
  clip is a new load, so a one-time "Interact" in OBS does not stick.

The Twitch Embed SDK cannot help: `Twitch.Embed` and `Twitch.Player` take
`channel`, `video` and `collection`, not clips.

## Decision Drivers

- The streamer turned the sound on; the clip should play with it.
- Nothing may get worse when the workaround breaks: clips must still play.
- No new backend surface for a frontend-only playback problem.

## Considered Options

1. **Look up the clip's signed MP4 URL through Twitch's web GQL API from the
   browser, and play it in our own `<video>` (chosen).**
2. Keep the clip embed and tell streamers to click unmute. It resets on every
   clip, so this never yields sound for more than one clip.
3. Do the GQL lookup in overlay-manager and return the URL with the clip list.
   The signed URL would come from a datacenter IP and travel to a different
   one; nothing measured shows that is safe, and it adds a backend change and
   an outbound dependency to a service that does not need it.

## Decision Outcome

Option 1. For the current clip, `fetchClipVideoUrl`
(`frontend/src/lib/twitchClipVideo.ts`) POSTs to `https://gql.twitch.tv/gql`
with Twitch's public web client ID and asks for the clip's
`playbackAccessToken { signature value }` and `videoQualities { sourceURL }`.
The page plays `sourceURL?sig=…&token=…` of the first (best) quality in a
`<video>` and advances on its `ended` event.

- The `<video>` renders `muted` and takes `clips_muted` when it has loaded
  data. OBS then plays it with sound. An ordinary browser tab refuses
  unmuted autoplay, so the clip falls back to muted and still plays.
- If the lookup fails, or the `<video>` errors, that clip plays through the
  clip embed as before: muted, advanced on a timer of its length plus 3 s.
- The lookup needs no user token, and the GQL endpoint answers
  `Access-Control-Allow-Origin: *`. The existing CSP already admits it
  (`connect-src https:`) and the MP4 host (`media-src https:`); `frame-src`
  keeps `https://clips.twitch.tv` for the fallback.

Measured with the real page, a real clip and OBS's autoplay flag: the `<video>`
plays at 1080p with `muted=false`, `volume=1`; without the flag it plays muted.

### Consequences

- Clip audio works in OBS, and clips advance on their real end instead of a
  timer. The clip is shown without Twitch's player chrome or mature-content
  interstitial.
- The GQL API is undocumented. Twitch can change the query, the client ID's
  behaviour or the token format without notice. When it does, the lookup
  returns null and the roll silently falls back to the muted embed; sound is
  lost, playback is not. This is the same class of dependency as InnerTube
  (YouTube) and the unofficial TikTok library.
- Each clip costs one GQL request from the viewer's browser (OBS), one per
  play, including replays of a single clip.

## Related Decisions

- ADR-0052 (self-hosted TikTok signing) and ADR-0065 (InnerTube for every
  unclaimed YouTube channel): other unofficial platform APIs used where the
  official one cannot do the job.
