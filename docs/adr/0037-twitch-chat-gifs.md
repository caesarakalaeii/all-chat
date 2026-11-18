# ADR-0037: Twitch Chat GIFs as Media Attachments

**Date**: 2026-07-18
**Status**: Accepted
**Revised**: 2026-11-18 (trimmed; record the current state of the event-path gap)
**Deciders**: caesarakalaeii

## Context

Twitch ships native chat GIFs (Giphy-backed). A viewer's GIF is delivered like an emote: the message body carries a bracketed alt caption (e.g. `[Y A Y Yes GIF by …]`) and a side-channel names the GIF standing in for that span; Twitch renders the GIF and hides the caption. Both Twitch transports carry it: EventSub `channel.chat.message` has a `gif` fragment, and IRC PRIVMSG has a `gifs` tag listing `start-end|gif_id|url` spans (inclusive, zero-based *byte* offsets).

Until this ADR the pipeline ignored the GIF entirely: messages surfaced as bare caption text with no image. PR #576 had already built a source-agnostic media path end to end for Discord uploads and Tenor/Giphy link previews — `MessageInfo.attachments []Attachment` on the wire, the `MessageAttachments` React component on all three render surfaces (live overlay, editor preview, monitor), a WCAG 2.2.2 hide/show control for animated GIFs, and CSP media directives — explicitly as groundwork for Twitch GIFs.

## Decision

**Twitch chat GIFs become `Attachment{type:"image", content_type:"image/gif"}` entries on `MessageInfo.attachments`, with the alt caption stripped from the visible text and kept as the attachment's alt text.** This reuses the whole #576 render + a11y + CSP path unchanged and matches Twitch's hidden-caption look. Alternatives rejected: rendering the GIF as a large inline emote (no a11y control on the monitor, duplicates `MessageAttachments`); attaching the GIF but leaving the caption visible (shows `[Y A Y …]` plus a redundant thumbnail).

Both transports converge on Twitch's own **`gifs` tag** as the internal representation:

- **twitch-eventsub-listener** (`buildGifsTag`, wired into `buildChatTags`) synthesizes the tag from `gif` fragments with byte offsets accumulated across fragments, mirroring how it already synthesizes `emotes`. A new `eventsub.ChatGif` type and a `Gif` field on `ChatMessageFragment` carry the payload (`services/twitch-eventsub-listener/eventsub/types.go`).
- **twitch-listener (IRC)**: the PRIVMSG parser builds its tag map from an explicit allowlist and forwards the native `gifs` tag with a one-line addition. The IRC path inherits the same multibyte-offset caveat as IRC emotes (Go byte offsets); the EventSub path is exact.
- **message-processor** (`extractTwitchGifs` in the Twitch normalizer): parses the tag into attachments, strips each caption span from the visible text, and re-anchors first-party emote byte offsets (`remapEmotePositions`), dropping any emote occurrence that overlapped a removed span. Spans are reduced to a sorted, disjoint set so a degenerate overlapping tag cannot desync the shift. A GIF with a present URL but a malformed span still yields an attachment. Capped at 4 GIFs per message.
- **frontend**: no code change — `MessageAttachments` is platform-agnostic and `img-src https:` already permits the URLs.

## Consequences

- Twitch chat GIFs render on overlays with zero frontend change; a11y and CSP are inherited from #576; one representation serves both transports; first-party emotes stay correct alongside GIFs.
- If Twitch changes the published `gif` fragment field names before GA, `ChatGif` / `buildGifsTag` is the single place to adjust.
- **Event-path scope**: `NormalizeEvent` now runs the same GIF extraction the chat path does (`services/message-processor/normalizer/twitch_normalizer.go:441`), so a notice whose message carries a `gifs` tag renders its GIF. The `resubscription` handler still attaches no tags (`services/twitch-eventsub-listener/webhooks/handler.go:782`), so a GIF inside a dedicated-resub message remains unrendered.
- **Zero-caption GIFs are dropped**: the position-based tag format has nothing to key on without a caption span; Twitch always sends one today.
- **Comma-in-URL** would break the tag round-trip, but that is Twitch's own format constraint (URL last, comma-separated) and Twitch never emits an unencoded comma.
- **Not premium-gated** and no onboarding-tour entry: this is parity with what Twitch native chat already shows, not a paid add-on. GIF *search* is a separate feature with its own providers and entitlements (ADR-0071 editor GIF search; the Klipy alert-overlay provider under ADR-0073/0074).

## Related

- [ADR-0015](./0015-eventsub-chat-ownership-claim.md) and [ADR-0026](./0026-twitch-irc-listener-deprecation.md) — the transports this rides on.
- ADR-0071, ADR-0073, ADR-0074 — the later, separate GIF-search features.
