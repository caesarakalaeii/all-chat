# ADR-0037: Twitch Chat GIFs as Media Attachments

**Date**: 2026-07-18
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Twitch shipped native chat GIFs (Giphy-backed). A viewer picks a GIF and it is
delivered to chat as a **text-replacement token, like an emote**: the message body
carries a bracketed alt caption (e.g. `[Y A Y Yes GIF by Djemilah Birnie]`) and a
side-channel names the GIF that stands in for that span. Twitch renders the GIF in
place of the caption and hides the caption text.

Both Twitch chat transports carry it (documented on `dev.twitch.tv`, and confirmed in
[PR #576](https://github.com/caesarakalaeii/all-chat/pull/576#issuecomment-5007998475)):

- **EventSub `channel.chat.message`**: a new fragment `{"type":"gif","text":"[…]","gif":{"gif_id":"…","url":"…"}}`.
- **IRC PRIVMSG**: a new `gifs` tag — a comma-separated list of `start-end|gif_id|url`,
  where `start-end` are inclusive, zero-based **byte** offsets into the message text
  marking the alt-caption span.

Until now our Twitch pipeline ignored the GIF entirely: the message surfaced as bare
`[…]` caption text with no image. We want Twitch chat GIFs to render on overlays.

[PR #576](https://github.com/caesarakalaeii/all-chat/pull/576) had already added a
media path end to end for Discord (image/GIF/video uploads and Tenor/Giphy link
previews), *explicitly* as groundwork for Twitch chat GIFs once their mechanism was
known: `MessageInfo.attachments []Attachment` on the wire, a source-agnostic
`MessageAttachments` React component wired into all three render surfaces (live
overlay, editor preview, in-app monitor), a WCAG 2.2.2 hide/show control for animated
GIFs on the in-scope monitor, and `media-src` in the CSP. This ADR records how Twitch
chat GIFs plug into that existing path.

## Decision Drivers

- **Reuse the #576 media path.** It already renders `image/gif` attachments on every
  surface with the a11y controls and CSP in place. Twitch GIFs *are* Giphy GIFs — the
  same media kind Discord's Tenor/Giphy previews already flow through.
- **One parse site for both transports.** EventSub is the primary/only forward chat
  path (ADR-0015, ADR-0026), but the IRC listener still passes native tags through, so
  both should converge on one representation rather than two parsers.
- **Faithful to Twitch's rendering.** Show the GIF, not the `[…]` caption — Twitch
  hides the caption. Keep the caption as accessible alt text.
- **No regressions to first-party emotes.** A message may in principle carry both a GIF
  and native Twitch emotes; stripping the caption must not desync emote byte offsets.

## Considered Options

1. **Render the GIF as a large inline "emote" (text-replacement in `renderMessage`).**
   Closest to Twitch's literal mechanism and needs no text stripping. **Rejected**: an
   inline `<img>` in `renderMessage` has *no* WCAG 2.2.2 control on the in-scope monitor,
   directly contradicting the position #576 just established (full-size GIFs get a
   hide/show control there); it also duplicates media rendering that `MessageAttachments`
   already does, and emote-sized inline layout fights a full-size GIF.

2. **Map the GIF to an `attachment`, leave the caption in the body.** Simplest, but the
   overlay then shows `[Y A Y Yes GIF by …]` as literal caption text *above* a duplicate
   GIF thumbnail — redundant and unlike Twitch. **Rejected.**

3. **Map the GIF to an `attachment` and strip the caption span from the text (chosen).**
   Reuses the whole #576 render + a11y + CSP path unchanged; matches Twitch's hidden-caption
   look; preserves the caption as the attachment's alt text.

## Decision Outcome

**Chosen: Option 3.** Twitch chat GIFs become `Attachment{type:"image", content_type:"image/gif"}`
entries on `MessageInfo.attachments`, and the alt caption is stripped from the visible text.

Concretely, both transports converge on the **`gifs` tag** as the internal wire
representation (`start-end|gif_id|url`, Twitch's own IRC format):

- **twitch-eventsub-listener** (`buildGifsTag`, wired into `buildChatTags`): synthesizes
  the `gifs` tag from `gif` fragments, with byte offsets accumulated across fragments —
  exactly mirroring how it already synthesizes the `emotes` tag from `emote` fragments.
  A new `eventsub.ChatGif` type and a `Gif` field on `ChatMessageFragment` carry the payload.
- **twitch-listener (IRC)**: the PRIVMSG parser builds its tag map from an explicit allowlist
  (it does not blindly copy `msg.Tags`), so it forwards the native `gifs` tag with a one-line
  addition next to the existing `source-*` forwarding. Positions in the native IRC tag follow the
  same convention as the IRC `emotes` tag, which the normalizer already byte-slices — so a
  multibyte caption on the IRC path inherits the same pre-existing offset caveat as IRC emotes
  (the EventSub path, which synthesizes byte offsets on both sides, is exact).
- **message-processor** (`extractTwitchGifs` in the Twitch normalizer): parses the `gifs`
  tag into attachments (URL, `content_type:"image/gif"`, caption → `filename`), strips each
  caption span from the visible text, and **re-anchors first-party emote byte offsets** to
  the stripped text (`remapEmotePositions`), dropping any emote occurrence that overlapped a
  removed span. A GIF whose URL is present but whose span is malformed still yields an
  attachment (the image renders) and is simply not stripped. Capped at 4 GIFs per message.
- **frontend**: **no code change.** `MessageAttachments` is platform-agnostic and already
  renders `image/gif` attachments on all three surfaces; Twitch GIFs are images, so the
  existing `img-src https:` CSP already permits them (no `media-src` change needed).

### Consequences

- **Positive**: Twitch chat GIFs render on overlays with zero frontend change; a11y and
  CSP are inherited from #576; one parser serves EventSub and IRC; first-party emotes stay
  correct alongside GIFs.
- **Negative / limitations**: Like the existing emote/attachment offset handling, positions
  are Go byte offsets; the strip logic reduces spans to a sorted, disjoint set so a degenerate
  overlapping tag can't desync the emote-offset shift. The EventSub `gif` fragment shape follows
  Twitch's published contract; if Twitch changes field names before GA, `ChatGif`/`buildGifsTag`
  are the single place to adjust. Known scoped-out cases, each consistent with existing behaviour
  and safe to defer:
  - **Event-path GIFs** (a GIF inside a resub/shared-sub *message*) are not rendered: the event
    handlers build no `gifs` tag and `NormalizeEvent` does not extract one — the same reason
    those messages already don't carry tag-based first-party emotes. Uniform, pre-existing gap.
  - **Zero-caption GIFs** are dropped: the tag format is position-based (`start-end`), so a GIF
    with no alt-caption span in the body has nothing to key on. Twitch always sends a bracketed
    caption today.
  - **Comma-in-URL** would break the comma-separated tag round-trip, but this matches Twitch's own
    IRC `gifs`-tag format constraint (URL last, comma-separated), so Twitch never emits an
    unencoded comma in a GIF URL.
- **Not premium-gated.** This is parity with what Twitch native chat already shows (matching
  #576's ungated Discord media), not a premium add-on, so it ships as a base feature with no
  `feature_gates` entry and no onboarding-tour entry. A Patreon note is still worthwhile once
  deployed.

## References

- ADR-0015 (EventSub chat-ownership claim), ADR-0026 (IRC listener deprecation)
- PR #576 (Discord image/GIF/video attachments — the media groundwork)
- Twitch docs: EventSub `channel.chat.message`; IRC `gifs` tag on PRIVMSG
