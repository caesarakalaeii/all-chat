# ADR-0016: Per-Link Twitch Credentials for Non-Twitch-Login Accounts

**Date**: 2026-06-13
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: All-Chat Platform Team

---

## Context and Problem Statement

The IRC↔EventSub partition (ADR-0015) decides that a Twitch channel is read via EventSub when its
owner granted the chat scopes (`user:read:chat` + `user:bot` + `channel:bot`) and holds a valid
token on the Twitch-login users row, or when linked credentials exist in `twitch_oauth_tokens`. There
was no linked-token path before this ADR: the grant was stored exclusively on the **users row**:

```sql
EXISTS (SELECT 1 FROM users u
        WHERE LOWER(u.username) = LOWER(ocs.channel_id)
          AND u.auth_provider = 'twitch'
          AND 'user:read:chat' = ANY(u.granted_scopes)
          AND u.token_expires_at > NOW())
```

The users row belongs to the **login provider**. A streamer who signed up via YouTube or Kick has
`username = 'youtube_…'` / a Kick username and `auth_provider != 'twitch'`, so the predicate can
**never** match their channel — regardless of what they consent to. When such a user completed the
Twitch add-source OAuth (which requests the chat scopes with `force_verify`), the callback landed
in `linkPlatformToUser`, which had nowhere to persist the grant:

- there is one `granted_scopes` column and one access/refresh-token slot per user, both
  semantically owned by the auth_provider (overwriting them was the clobber bug fixed in
  `d1a5efce`);
- `UserRepository.Update` never persisted the platform-ID columns, so even the link itself
  (`google_id`/`kick_id`/`twitch_id`) was silently dropped.

Net effect (verified in production 2026-06-13): 11 of 46 IRC-partitioned channels belonged to
YouTube/Kick-login accounts with **no self-service path to EventSub at all**. Twitch held their
grant server-side; All-Chat discarded every artifact of it.

## Decision

Store Twitch credentials obtained via the **add-source link flow** in a dedicated table,
`twitch_oauth_tokens` (migration 056), keyed by `(user_id, twitch_login)`:

- `twitch_login` (= channel name), `twitch_user_id`, encrypted `access_token`/`refresh_token`
  (shared multi-key cipher, `encryption_version = 1`), `token_expires_at`, `granted_scopes`.
- Written by auth-service **only when `platform == twitch && isAddSource && auth_provider !=
  'twitch'`** (`shouldStoreLinkedTwitchCredentials`). Twitch-login accounts keep their grant on
  the users row — storing it twice would have token-refresh racing two copies of the same refresh
  token.

Consumers extend their lookups to both credential sources:

- **overlay-manager** `chat_via_eventsub`: `… OR EXISTS (SELECT 1 FROM twitch_oauth_tokens t WHERE
  LOWER(t.twitch_login) = LOWER(ocs.channel_id) AND 'user:read:chat' = ANY(t.granted_scopes) AND
  t.token_expires_at > NOW())` — the frontend badge turns green for linked accounts.
- **twitch-eventsub-listener** `SyncChannels`: a `LEFT JOIN LATERAL` unions both sources per
  channel and picks ONE credential, preferring whichever is chat-scoped AND unexpired (users row
  breaks ties). The subscription then uses that token exactly as before.
- **token-refresh-service**: new token type `twitch_link` flows through the standard batch loop
  (`GetExpiringTwitchLinkTokens` / `UpdateTwitchLinkTokens` /
  `MarkTwitchLinkTokenPermanentlyFailed`, same bounded 48-hour recovery window and 30-day
  permanent-fail suppression).
- **DSGVO cleanup**: `cleanup_expired_oauth_tokens()` was extended by migration 063 to cover the
  linked tables; it was later removed by migration 118 together with the other pg_cron-scheduled
  cleanups, so no automatic expired-token deletion runs for `twitch_oauth_tokens` today. The
  `retention-sweeper` CronJob ([ADR-0089](./0089-retention-sweeper-cronjob.md)) deliberately leaves
  these rows in place, because expiry alone does not prove the credential is dead and a live row
  still anchors the channel's EventSub eligibility; they cascade from `users` on account deletion.

## Considered Alternatives

1. **Match the partition predicate on `users.twitch_id` after fixing platform-ID persistence.**
   Rejected: `ocs.channel_id` stores the Twitch *login*, not the numeric ID, so this still needs a
   login column; and it leaves the token/scope storage problem unsolved (one slot per user).
2. **Per-platform scope/token columns on users.** Rejected: schema bloat, and the YouTube/Kick
   precedent (`youtube_oauth_tokens`, `kick_oauth_tokens`) already established per-platform token
   tables.
3. **Tell affected users to create a second, Twitch-login account.** Rejected: works (the
   predicate matches any Twitch-login row by channel name) but is an unacceptable UX and risks the
   duplicate-account guard.

## Consequences

- The "Connect Twitch" add-source button now works identically for every account type; the
  outreach instruction "re-add your Twitch source" is truthful for all users. (The add-source flow
  now lands these accounts on the EventSub listener, which is what ADR-0026's enforce phase left as
  the only Twitch chat path.)
- A channel can be EventSub-eligible through either credential source; the LATERAL preference
  order makes the choice deterministic (valid+scoped first, users row over linked).
- Linked credentials die independently of the account: if their refresh token is revoked,
  token-refresh-service records the rejection by pushing the expiry 30 days ahead, the listener's
  `has_chat_scope`/`token_expires_at` checks stop selecting the row, and the channel leaves the
  EventSub partition. Under ADR-0015 that was a fallback to IRC; since ADR-0026's enforce phase
  dropped IRC, the channel's chat goes quiet until the credential is repaired (re-add with
  `force_verify`).
- `linkPlatformToUser` still does not persist `google_id`/`kick_id`/`twitch_id` on users (known
  gap, unchanged by this ADR): account *linking* as an identity feature remains unimplemented;
  this ADR only fixes credential storage for chat reading.

## Verification

- `services/auth-service/handlers/platform_auth_v2_link_test.go` — storage decision + persistence
  (testcontainers).
- `services/overlay-manager/repository/source_repo_test.go` — predicate flips on linked rows,
  ignores expired ones.
- `services/twitch-eventsub-listener/channels/query_test.go` — credential selection incl. the
  "linked beats scope-less users row" case.
- `services/token-refresh-service` — source-level query guards + refresh/mark routing tests.
- Migration idempotency: `services/auth-service/repository/migrations_rerun_test.go`.
