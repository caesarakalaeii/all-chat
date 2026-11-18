# ADR-0090: Disconnecting a linked platform deletes its tokens

**Date**: 2026-11-02
**Status**: Accepted
**Revised**: 2026-11-04 (a disconnect also clears the platform id that linking records on the account; viewer unlink deletes the unlinked session instead of keeping it with wiped tokens)
**Deciders**: caesarakalaeii

## Context

The privacy policy says OAuth tokens are deleted on disconnect, but only Patreon, custom
chat bots and Discord had a disconnect: the per-link token tables and
`mod_oauth_credentials` (ADR-0048) had nothing to press, removing an overlay source never
touched a token, and viewer unlink left session tokens behind. A disconnect must also say
what happens to the sources reading chat with those tokens and to the sign-in platform.
Constraints: no token for that platform left in the database whatever the provider does;
no source left in a state a listener retries forever; the account stays usable.

## Considered options

- Delete every source of the platform: rejected for Twitch, YouTube and Kick, whose
  sources work or return with one reconnect.
- Deactivate sources instead: rejected, the listeners' heartbeat sets `is_active` back
  and the Facebook listener does not filter on it.
- Revoke before deleting: rejected, a provider outage would block the delete, the half
  the policy promises.
- Allow disconnecting the sign-in platform when another login exists: deferred; it means
  moving `auth_provider`, which the EventSub predicate, send paths and the duplicate-
  account guard key on.
- Keep the unlinked viewer session with wiped tokens (first version): replaced on
  2026-11-04, nothing would ever remove the row (the pg_cron cleanup never ran, ADR-0089;
  the retention sweeper keeps viewer sessions; viewer erasure follows linked identities a
  detached row no longer belongs to).

## Decision

`DELETE /api/v1/auth/me/connections/:platform` (auth-service, gateway-routed,
session-only, refused while impersonating) plus `GET .../connections` back a "Connected
platforms" card on `/settings`.

Delete first, then revoke: one transaction deletes every row the user holds in the
platform's token table plus, for Twitch/YouTube/Kick, their `mod_oauth_credentials` row,
and clears the id column (`twitch_id`/`google_id`/`kick_id`) that linking filled (ADR-0016).
Only then the deleted tokens go to the provider revoke endpoint, each distinct token once,
refresh first. A refusing or down provider is logged and the disconnect succeeds; Meta
has no revoke for the stored Page token, so Facebook/Instagram are deleted locally and
the dialog says to remove the app on Facebook.

Sources follow what they need: Twitch stays (EventSub loses the credential at the next
sync, unsubscribes and releases the claim; chat stops until reconnect); YouTube stays and
drops out of official-API mode within a claim round, reading public streams via
InnerTube; Kick stays (subscribes without a token); Facebook and Instagram sources are
removed in the same transaction, since they cannot exist without the Page token and their
listeners would poll a missing token forever.

The sign-in platform is refused (`409 login_platform`): its ids on the `users` row are how
the account exists and its per-link rows are rewritten on the next sign-in; they go with
the account. The card lists it marked as the sign-in.

Viewer unlink (`UnlinkPlatform`) runs in one transaction: removes the platform identity
and deletes that platform account's `viewer_sessions` rows (tokens, ban flags and message
history with them, like signing out), then revokes the deleted tokens with the viewer
OAuth client. youtube-listener's refresh write-back no longer UPSERTs (a refresh in flight
could otherwise recreate the deleted row; a missing row is an error).

## Consequences

- Every linked platform has a working disconnect and the policy is true for them; the
  sign-in platform's tokens go only with the account.
- A disconnected platform stops signing in (its id column is cleared), so the account
  stops claiming that platform account and another may link it. Reconnect through
  add-source stores new tokens and the id again; a login with it then creates a new
  account or fails on the username index if taken.
- Account deletion (ADR-0088) erases the viewer identity of platform accounts linked at
  deletion; one disconnected beforehand is erased from the viewer settings instead.
- A Twitch EventSub subscription created by a pod that restarted before the disconnect is
  not tracked by the new leader and survives this sync (pre-existing; no orphan sweep).
