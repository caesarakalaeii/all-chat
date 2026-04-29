# ADR-0012: OAuth Scope Minimisation Post Extension v1.6.0

**Date**: 2026-04-29
**Status**: Accepted

---

## Context and Problem Statement

The AllChat backend's viewer OAuth flows historically requested send-capable scopes — `user:write:chat` (Twitch), `youtube.force-ssl` (YouTube), `chat:write` plus `chat:read`/`channel:read` (Kick) — because the extension routed chat sending through the backend's `/auth/viewer/chat/send` endpoint. The endpoint used the viewer's OAuth token to call each platform's send API.

Extension v1.6.0 (released 2026-04-29) moved chat sending entirely off the backend onto each platform's native session:

- Twitch: GraphQL via the user's `auth-token` cookie
- YouTube: native YouTube live-chat input bar (InnerTube + SAPISIDHASH from page session)
- Kick: REST `messages/send/{chatroomId}` with cookie XSRF token

After v1.6.0 ships and propagates, the backend's send-via-OAuth path is no longer reachable from any current client. The send-capable scopes therefore correspond to dead capability.

Continuing to request them costs:

1. **Consent friction.** Twitch / Google / Kick consent screens display a separate "send messages on your behalf" line, which a non-trivial fraction of viewers refuse. Conversion rate on first sign-in is a real signal we measure on the dashboard.
2. **Token blast radius.** A leaked OAuth token grants strictly the scopes it was issued with. Reducing scope list reduces what an attacker can do with it (e.g. spam-send messages as the user across platforms).
3. **"Only ask for what we need" honesty.** The marketing site's privacy paragraph explicitly invokes this principle. Requesting send scope while never sending is a contradiction.

## Decision

Drop every OAuth scope that has no remaining call site in current backend code, in a single hard cutover (no soft-deprecation phase). Effective immediately on this branch.

**Final scope set per provider × flow:**

| Provider | Flow | Final scopes | Removed |
|---|---|---|---|
| Twitch | viewer | (none — empty slice) | `user:write:chat` |
| Twitch | streamer | `channel:read:redemptions`, `channel:read:subscriptions`, `bits:read`, `moderator:read:followers` | (no change — every scope still used by EventSub) |
| YouTube | viewer | `youtube.readonly`, `userinfo.profile` | `youtube.force-ssl` (replaced with `youtube.readonly` for `GetChannelID`) |
| YouTube | streamer | `youtube.readonly`, `userinfo.profile` | `youtube.force-ssl` |
| Kick | viewer | `user:read` | `chat:read`, `chat:write`, `channel:read` |
| Kick | streamer | `user:read` | `chat:read`, `channel:read` |
| Discord | bot | `bot` + permissions `1024,2048,65536` | (no change — bot capability flags, not user OAuth) |

**Verification basis for each removal:**

- `user:write:chat` (Twitch viewer): only consumer was `helix/chat/messages POST` in auth-service `chat_send` handler, called only from extension's pre-1.6.0 send path. No other caller in repo.
- `youtube.force-ssl` (YouTube viewer): only consumer was `liveChat/messages POST` in viewer-side `sendYouTubeMessage`. `GetChannelID` does `channels?part=id&mine=true` which is satisfied by `youtube.readonly`.
- `youtube.force-ssl` (YouTube streamer): only consumer was `sendStreamerYouTubeMessage`. No production caller of that path remains; streamer-side YouTube sending happens via the streamer's own browser session.
- `chat:write` (Kick viewer): only consumer was `api.kick.com/public/v1/chat POST` in `sendKickMessage`. Same dead-path situation.
- `chat:read` (Kick, both flows): no consumer found anywhere — kick-listener uses Kick's authenticated WebSocket which doesn't gate on this OAuth scope.
- `channel:read` (Kick, both flows): no consumer found — channel info fetched via unauthenticated public Kick API in kick-listener.

Backend-side handler functions (`sendKickMessage`, `sendYouTubeMessage`, etc.) are intentionally left in place but unreachable. They will be removed in a follow-up refactor; keeping them here lets us re-enable temporarily if any of the v1.6.0 native send paths regress.

## Cutover: hard, not soft

We chose a hard cutover (drop scopes immediately) over a soft cutover (deprecate `/auth/viewer/chat/send` first, drop scopes a release later).

**Soft path was rejected because:**

- The v1.6.0 store rollout already started — Chrome Web Store + Firefox Add-ons review queues are in flight. Most users will upgrade naturally over the next 1–2 weeks.
- A v1.5.x extension still in the wild only fails to send when the user re-authenticates after this change deploys. Existing tokens keep their old scopes until natural expiry/refresh.
- The failure mode for an affected user is "send returns an error" — visible, noisy, prompts an upgrade. Not a silent corruption.
- Maintaining two scope sets across a soft-deprecation window adds branching code and consent-screen variability that has to be reconciled later.

The risk we accept: a v1.5.x user who reauths between deploy and their natural extension update will see send failures until they upgrade. We judged this acceptable given the explicit error UX.

## Files Changed

```
services/auth-service/oauth/viewer_twitch.go    — empty Scopes slice
services/auth-service/oauth/viewer_youtube.go   — force-ssl → readonly
services/auth-service/oauth/viewer_kick.go      — only user:read
services/auth-service/oauth/youtube.go          — drop force-ssl
services/auth-service/oauth/kick.go             — only user:read (both PKCE call sites)
docs/adr/0012-oauth-scope-minimisation.md       — this file
```

`go build ./...` clean across the auth-service module; `go test ./oauth/...` passes (existing tests don't assert scope literals, so no test changes were needed).

## Consequences

### Positive

- Smaller consent screens; expected uplift on viewer sign-in conversion.
- Reduced token-leak blast radius across all three platforms.
- Backend's intent matches its marketed privacy promise.
- Removes confusion about why "send messages on your behalf" appears on a dashboard sign-in flow that never sends.

### Negative

- v1.5.x extension users who re-authenticate during the cutover window will see send failures and need to upgrade. We judge this acceptable per the cutover analysis above.
- Backend's `sendKickMessage` / `sendYouTubeMessage` / Twitch viewer-side helix call become dead code. They stay in the repo for a release as a safety hatch but should be removed in a follow-up.

### Reversibility

Trivial — re-add a string to the appropriate `Scopes` slice and redeploy. Existing tokens are unaffected. The change is per-flow and narrowly scoped.

## Related

- Extension v1.6.0 release (commits `b76b1c3` CSP fix for emote APIs, `b98d309` popout SW relay, tag `v1.6.0`)
- Future ADR (TBD): removal of dead viewer-side send handlers from `auth-service`
