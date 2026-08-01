# ADR-0047: Username colour resolution order

## Status

Accepted

## Context

A user reported that All-Chat "is not using the Twitch viewer's set colour".

Measured against live production traffic (219 raw Twitch messages sampled from
`chat:raw`, 2026-08-01), the report was only partly right:

- **191 / 219 (87%)** of Twitch messages carry an explicit `color` tag, and that
  colour *was* already flowing intact: EventSub `channel.chat.message.color` →
  `buildChatTags` → `tags["color"]` → `TwitchNormalizer.extractUserInfo` →
  `UserInfo.Color` → overlay.
- **28 / 219 (13%)** carry no `color` tag at all — those chatters never picked a
  colour on Twitch.

The real defects were narrower, and both trace to one design mistake:
`ViewerBadgeEnricher` folded its deterministic fallback palette **into the same
`Color` field** that carries authoritative colours.

1. **The streamer's "Username color" setting was dead.** `ColorsGroup.tsx`
   renders the picker and `visualSettingsToCss` emits `--chat-username-color`,
   but the overlay only consumed that variable as a CSS fallback:
   `message.user?.color || 'var(--chat-username-color, #FFFFFF)'`. Because the
   auto-colour `defer` guaranteed `Color` was never empty, the fallback could
   never fire. Setting the picker did nothing at all.
2. **No room to rank the settings.** With one field there is no way to express
   "platform colour beats the streamer's setting, which beats the fallback",
   because by the time the overlay sees the message it cannot tell whether
   `Color` came from Twitch or from All-Chat's palette.

A third option was considered and **rejected**: reproducing Twitch's own 15-colour
default palette and its per-user assignment so the 13% match Twitch chat exactly.
We kept All-Chat's 16-colour palette instead — it is tuned for legibility on dark
overlays and stays stable across a viewer's linked platforms, which Twitch's
per-account default cannot do for YouTube/Kick/TikTok/Discord chatters.

## Decision

Split the two concerns into two fields and resolve the priority at render time.

`UserInfo.Color` becomes **authoritative only**:

1. the viewer's manually chosen All-Chat cosmetic colour, else
2. the platform-native colour (Twitch/Kick/Discord),
3. otherwise empty.

`UserInfo.AutoColor` (new, `auto_color` on the wire) always carries the
deterministic palette fallback, keyed by the All-Chat viewer UUID when the
chatter is a registered viewer and by `platform:userID` otherwise. It is omitted
when a gradient is set, since a gradient replaces the username colour outright.

The overlay resolves the full chain in `resolveUsernameColor`:

```
viewer's manual All-Chat colour        ─┐
                                        ├─ user.color (server-resolved)
platform-native colour                 ─┘
  > streamer's "Username color" setting     (--chat-username-color)
    > deterministic auto palette            (user.auto_color)
      > #FFFFFF
```

The middle two steps use CSS custom-property fallback syntax
(`var(--chat-username-color, <auto_color>)`) rather than a JavaScript branch:
`visualSettingsToCss` only emits `--chat-username-color` when the streamer
actually picks one, so an unset setting falls through to the auto colour at paint
time with no extra plumbing.

## Consequences

**Positive**

- Explicitly-set Twitch/Kick/Discord colours are honoured, and are now provably
  distinguishable from generated ones.
- The "Username color" overlay setting works for the first time, and applies
  exactly where a streamer would expect: to the chatters who have no colour of
  their own, without flattening the ones who do.
- Chatters keep a stable, legible colour instead of collapsing to white.
- One resolver (`frontend/src/lib/utils/usernameColor.ts`) backs all four render
  sites (live overlay, editor preview iframe, `EventContent`, `usernameSpan`),
  which previously each carried their own copy of the fallback expression.

**Negative / risks**

- `auto_color` adds a field to every chat message on the wire (~20 bytes).
- Clients that read only `color` will now see it empty for the ~13% of chatters
  with no platform colour, and must adopt the chain. The API Gateway is not
  affected: it forwards the pub/sub payload as a generic map (`stripOverlayID`),
  so the field passes through without a struct change.
- The 13% still will not match the colour Twitch's own chat shows them, by
  choice. Revisit only if streamers ask for exact Twitch parity.

## References

- `services/message-processor/enricher/viewer_badge_enricher.go` — resolution + defer
- `services/message-processor/enricher/viewer_color.go` — `AutoColor` palette
- `frontend/src/lib/utils/usernameColor.ts` — `resolveUsernameColor`
- ADR-0008 (feature gates), ADR-0044 (gradient rendering)
