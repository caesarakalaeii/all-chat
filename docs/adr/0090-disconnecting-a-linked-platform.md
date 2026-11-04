# ADR-0090: Disconnecting a linked platform deletes its tokens

**Date**: 2026-11-02
**Status**: Accepted
**Revised**: 2026-11-04 (a disconnect also clears the platform id that linking records on the account; viewer unlink deletes the unlinked session instead of keeping it with wiped tokens)
**Deciders**: caesarakalaeii

## Context and Problem Statement

The privacy policy says OAuth tokens are deleted when you disconnect a platform,
but for most platforms there was nothing to press. Patreon, custom chat bots and
Discord had a disconnect; the per-link token tables (`twitch_oauth_tokens`,
`youtube_oauth_tokens`, `kick_oauth_tokens`, `facebook_oauth_tokens`,
`instagram_oauth_tokens`) and delegated-moderator credentials
(`mod_oauth_credentials`, ADR-0048) did not. Removing an overlay source never
touched a token, and unlinking a platform from a viewer left that session's
tokens in `viewer_sessions`.

A disconnect has to say what happens to the overlay sources that read chat with
those tokens, and what happens to the platform the account signs in with.

## Decision Drivers

- After a disconnect, no token for that platform is left in our database, whatever
  the provider does.
- A source must never be left in a state where a listener retries it forever.
- The account must stay usable: you cannot disconnect your way out of signing in.

## Decision

`DELETE /api/v1/auth/me/connections/:platform` (auth-service, routed by the
api-gateway, session-only, refused while impersonating) and
`GET /api/v1/auth/me/connections` back a "Connected platforms" card on
`/settings`.

1. **Delete first, then revoke.** One transaction deletes every row the user holds
   in the platform's token table and, for Twitch, YouTube and Kick, their
   `mod_oauth_credentials` row, and clears the platform's id column on `users`
   (`twitch_id`, `google_id` or `kick_id`), which linking fills so that platform's login
   reaches the account (ADR-0016). Only then are the deleted tokens sent to the
   provider's revoke endpoint (Twitch `id.twitch.tv/oauth2/revoke`, Google
   `oauth2.googleapis.com/revoke`, Kick `id.kick.com/oauth/revoke`), each distinct
   token once, refresh token first. A provider that refuses or is down is logged;
   the disconnect still succeeds. Meta has no revoke for the stored Page token, so
   Facebook and Instagram are deleted locally only and the dialog tells the
   streamer to remove the app on Facebook.
2. **Sources follow what they need.**

   | Platform | Sources after a disconnect |
   |---|---|
   | Twitch | Stay. The EventSub listener finds no credential for the channel on its next sync (about 30 s), unsubscribes and releases the claim; chat stops until the streamer connects again (IRC is off, ADR-0026). |
   | YouTube | Stay. They drop out of official-API mode (ADR-0065) within a claim round and keep reading public streams through InnerTube, which needs no token. |
   | Kick | Stay. The Kick listener subscribes without a token. |
   | Facebook, Instagram | Removed from the user's overlays, in the same transaction. These sources cannot exist without the Page token (overlay-manager refuses to create one without it), and their listeners would otherwise poll a missing token every 10 s forever. |

3. **The sign-in platform is refused** (`409 login_platform`). Its tokens on the
   `users` row are how the account exists, and its per-link rows are written back
   on the next sign-in. They are deleted with the account (`DELETE /me`). The card
   lists it, marked as the sign-in, with that explanation.
4. **Viewer unlink deletes the session.** `UnlinkPlatform` runs in one transaction:
   it removes the platform identity and deletes that platform account's
   `viewer_sessions` rows of the viewer (tokens, ban flags and message history with
   them), as signing out does. The handler then revokes the deleted tokens with the
   viewer OAuth client.
5. **Refresh write-backs never insert.** youtube-listener's token store used an
   UPSERT to save a refreshed token, so a refresh in flight during a disconnect
   could recreate the deleted row. It now updates only; a missing row is an error.

## Considered Alternatives

1. **Delete every source of the platform.** Rejected for Twitch, YouTube and Kick:
   those sources work, or come back with one reconnect, without the streamer
   rebuilding their overlays.
2. **Deactivate sources instead of deleting them.** Rejected: the listeners'
   heartbeat sets `is_active` back to true, and the Facebook listener does not
   filter on it.
3. **Revoke before deleting.** Rejected: a provider outage would block the delete,
   which is the half the privacy policy promises.
4. **Allow disconnecting the sign-in platform when another login exists.** Rejected
   for now: it would mean moving `auth_provider`, which the EventSub predicate,
   send paths and duplicate-account guard all key on.
5. **Keep the unlinked session row with its tokens wiped and marked expired** (the
   first version of this ADR, so the row kept its ban flags and message history).
   Replaced on 2026-11-04: nothing would ever remove the row. The pg_cron cleanup it
   relied on never ran and is gone (ADR-0089), the retention sweeper keeps viewer
   sessions, and viewer erasure (ADR-0088) follows the viewer and its linked
   identities, which a detached row no longer belongs to. A session ban is lifted the
   same way by signing out, and viewer messages are swept after an hour anyway.

## Consequences

- Each linked platform has a working disconnect, and the privacy policy is true for
  them. The sign-in platform's tokens go only with the account.
- A disconnected platform no longer signs in to the account: its id column on `users`
  is cleared with the tokens, so the account stops claiming that platform account and
  another account may link it. Reconnecting through add-source stores new tokens and
  records the id again. A login with the disconnected platform is then a login with an
  unknown account; it creates a new one, or fails on the username index when the name
  is taken (as before linked ids were recorded).
- Account deletion (ADR-0088) erases the viewer identity of every platform account the
  account is linked to when it is deleted. A platform disconnected before that is no
  longer one of them; its viewer data is erased from the viewer settings instead.
- A Twitch EventSub subscription created by a pod that restarted before the
  disconnect is not tracked by the new leader and is not deleted by this sync
  (pre-existing; the listener has no orphan sweep).

## Verification

- `services/auth-service/repository/platform_connections_test.go` and
  `viewer_unlink_test.go` — real PostgreSQL with the full migration set.
- `services/auth-service/handlers/platform_connections_test.go`,
  `viewer_unlink_revoke_test.go`, `oauth/revoke_test.go` — status mapping,
  session-only guard, revoke endpoints and parameters.
- `services/youtube-listener/oauth/store_test.go` — a refresh write-back cannot
  recreate a deleted row.
- `frontend/tests/e2e/settings-connections.spec.ts` and
  `src/components/settings/__tests__/ConnectedPlatformsCard.test.tsx`.
