# ADR-0047: Username Colour Resolution Order

**Date**: 2026-08-01
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

A user reported that All-Chat "is not using the Twitch viewer's set colour". Measured against live traffic (219 raw Twitch messages sampled from `chat:raw`, 2026-08-01), the report was only partly right: 191 (87%) carry an explicit `color` tag, and that colour already flowed intact end to end; 28 (13%) carry no `color` tag at all, because those chatters never picked a colour on Twitch.

The real defects both trace to one design mistake: `ViewerBadgeEnricher` folded its deterministic fallback palette into the same `Color` field that carries authoritative colours, guaranteeing `Color` was never empty. Two consequences:

1. **The streamer's "Username color" setting was dead.** `ColorsGroup.tsx` renders the picker and emits `--chat-username-color`, but the overlay consumed it only as a CSS fallback (`message.user?.color || 'var(--chat-username-color, ...)'`), which could never fire. Setting the picker did nothing.
2. **No room to rank the settings.** With one field the overlay cannot tell whether `Color` came from the platform or from All-Chat's palette, so "platform colour beats the streamer's setting, which beats the fallback" was inexpressible.

A third option was considered and rejected: reproducing Twitch's 15-colour default palette and its per-user assignment for the 13%. All-Chat keeps its 16-colour palette — tuned for legibility on dark overlays and stable across a viewer's linked platforms, which Twitch's per-account default cannot do for YouTube/Kick/TikTok/Discord chatters.

## Decision

Split the concerns into two fields and resolve the priority at render time.

`UserInfo.Color` becomes **authoritative only**: 1. the viewer's manually chosen All-Chat cosmetic colour, else 2. the platform-native colour (Twitch/Kick/Discord), else empty. `UserInfo.AutoColor` (`auto_color` on the wire) always carries the deterministic palette fallback, keyed by the All-Chat viewer UUID for registered viewers and by `platform:userID` otherwise; it is omitted when a gradient is set, since a gradient replaces the username colour outright.

The overlay resolves the chain in `resolveUsernameColor` (`frontend/src/lib/utils/usernameColor.ts`):

1. authoritative colour (`user.color`: manual All-Chat cosmetic, else platform-native);
2. streamer's "Username color" setting (`--chat-username-color`);
3. deterministic auto palette (`user.auto_color`);
4. `#FFFFFF`.

Steps 2 and 3 use CSS custom-property fallback syntax (`var(--chat-username-color, <auto_color>)`) rather than a JavaScript branch: the setting's variable is only emitted when the streamer actually picks one, so an unset setting falls through at paint time.

**Amendment (2026-09): bubble-colour mode resolves usernames statically.** When "Use username colours" for bubbles is on, the bubble itself carries the chatter's colour, so repeating it on the name would hide the name against its own fill. `resolveUsernameColor` gained a `staticColor` option that skips the `user.color` short-circuit and returns the var-chain unconditionally while the mode is on. Gradient usernames are unaffected (callers branch on `name_gradient` first), and colorless chatters whose bubbles fall back to `auto_color` keep a legible name.

## Consequences

- Explicitly-set Twitch/Kick/Discord colours are honoured and are provably distinguishable from generated ones.
- The "Username color" setting works for the first time, applying exactly where a streamer expects: to chatters with no colour of their own, without flattening the ones who do.
- One resolver (`resolveUsernameColor`) backs all four render sites (live overlay, editor preview iframe, `EventContent`, `usernameSpan`), which previously each carried their own fallback expression.
- `auto_color` adds a field to every chat message on the wire (~20 bytes). Clients reading only `color` will see it empty for the ~13% of chatters with no platform colour and must adopt the chain; the API Gateway is unaffected (it forwards the Pub/Sub payload as a generic map).

## Implementation

- `services/message-processor/enricher/viewer_badge_enricher.go` — resolution + defer.
- `services/message-processor/enricher/viewer_color.go` — `AutoColor` palette.
- `frontend/src/lib/utils/usernameColor.ts` — `resolveUsernameColor`.

## References

- [ADR-0008](./0008-feature-gate-infrastructure.md) (feature gates), [ADR-0044](./0044-overlay-text-outline-via-layered-text-shadow.md) (gradient rendering).
