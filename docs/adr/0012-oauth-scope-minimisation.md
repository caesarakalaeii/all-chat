# ADR-0012: OAuth Scope Minimisation Post Extension v1.6.0

**Date**: 2026-04-29
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: Auth team

## Context

The backend's viewer OAuth flows historically requested send-capable scopes (`user:write:chat` on Twitch, `youtube.force-ssl` on YouTube, `chat:write`/`chat:read`/`channel:read` on Kick) because extension v1.5.x routed chat sending through the backend's `/auth/viewer/chat/send`. Extension v1.6.0 (2026-04-29) moved sending onto each platform's native session (Twitch GraphQL via the `auth-token` cookie, YouTube's native live-chat input via InnerTube, Kick's REST send with the XSRF cookie), leaving those scopes as dead capability.

Continuing to request them costs consent friction ("send messages on your behalf" lines refused by many viewers at sign-in) and widens the blast radius of a leaked token, and contradicts the "only ask for what we need" privacy promise.

## Decision

Drop every OAuth scope with no remaining call site, as a hard cutover (no deprecation window). Final scopes per flow:

| Provider | Flow | Scopes |
|---|---|---|
| Twitch | viewer/login | none |
| Twitch | streamer | `channel:read:redemptions`, `channel:read:subscriptions`, `bits:read`, `moderator:read:followers` (all used by EventSub) |
| YouTube | viewer | `youtube.readonly`, `userinfo.profile` (replaces `force-ssl` for `GetChannelID`) |
| YouTube | streamer | `youtube.readonly`, `userinfo.profile` |
| Kick | streams | `user:read` |
| Kick | add-source | `user:read`, `channel:read` (2026-09-26 addendum) |
| Discord | bot | `bot` + permissions `1024,2048,65536` (capability flags, not user OAuth) |

Addendum (2026-09-26): Kick streamer `channel:read` is restored for the add-source flow only. The originally claimed "no consumer" no longer holds: add-source must resolve the streamer's actual channel slug via Kick's authenticated `GET /public/v1/channels`, because `/public/v1/users` `name` is the display name, not the slug. Storing the name made every slug lookup 404 and silently auto-deactivated the source (prod incident, overlay `36847b00`). See `KickOAuth.GetAuthURLWithChannelScopePKCE`; login and viewer flows stay at `user:read`.

Addendum (2026-06-19): send scopes returned where they have a real caller, via explicit re-consent flows, never autoplay: the moderation re-consent requests `TwitchSendScope` for Twitch and `youtube.force-ssl` for the `timeout`/`ban` actions (`youtubeModerationScopeByAction` in `services/auth-service/handlers/chat_bot_send.go`, ADR-0017). Kick `chat:write` similarly returns through moderation re-consent. The flows below stay minimal.

Hard-cut rationale: rolled-out extensions upgrade naturally over 1-2 weeks; existing tokens keep their old scopes until refresh; failures are loud ("send returns an error"), prompting upgrade, rather than silent; and a soft window means maintaining two scope sets. Accepted risk: a v1.5.x user reauthing before upgrading loses send until they upgrade.

## Consequences

Smaller consent screens, smaller token blast radius, backend intent matching the privacy promise. Cost: the noted v1.5.x window; and backend viewer send code becomes dead capability kept as a safety hatch (send paths reappeared as callers via moderation and streamer sends). Reversibility is trivial: re-add a scope string to the relevant flow's `Scopes` slice.

## Implementation

- Scope lists live in `services/auth-service/oauth/` (`viewer_twitch.go`, `viewer_youtube.go`, `viewer_kick.go`, `youtube.go`, `kick.go`).
- Backend send paths that remain: `/chat/send` and `/viewer/chat/send` in `services/auth-service/handlers/chat_send.go` (viewer sends via backend were removed from the extension's flow; the bridge endpoints remain for the chat-bot and streamer paths), plus `chat_bot_send.go` and `chat_announce.go`.

Related: ADR-0017 (moderation write path and the moderation re-consent scopes).
