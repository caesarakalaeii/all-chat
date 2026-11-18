# ADR-0016: Per-Link Twitch Credentials for Non-Twitch-Login Accounts

**Date**: 2026-06-13
**Status**: Accepted
**Revised**: 2026-11-18 (linked platform ids persisted and backfilled; disconnect clears them)
**Deciders**: All-Chat Platform Team

## Context

The Twitch chat partition (ADR-0015) resolves a channel's credential from the users row, but that row belongs to the login provider. A streamer signed up via YouTube or Kick has `auth_provider != 'twitch'`, so the partition predicate could never match their channel, and `linkPlatformToUser` had nowhere to persist a Twitch grant: one `granted_scopes` column and one token slot per user, both semantically owned by the login provider (overwriting them was the clobber bug fixed in `d1a5efce`), and `UserRepository.Update` did not persist platform-ID columns at all. Verified in production 2026-06-13: 11 of 46 IRC-partitioned channels had no self-service path to EventSub.

## Decision

Store credentials obtained via the add-source link flow in a dedicated table `twitch_oauth_tokens` (migration 056), keyed by `(user_id, twitch_login)` with `twitch_login`/`twitch_user_id`, AES-encrypted `access_token`/`refresh_token` (shared multi-key cipher, `encryption_version = 1`), `token_expires_at`, `granted_scopes`. auth-service writes there only when `platform == twitch && isAddSource && auth_provider != 'twitch'`; Twitch-login accounts keep the grant on their users row (a second copy would make token-refresh race itself).

Consumers check both credential sources:

- overlay-manager `chat_via_eventsub`: the predicate ORs an exists over `twitch_oauth_tokens` with `user:read:chat` granted and unexpired, so the frontend badge turns green for linked accounts.
- twitch-eventsub-listener `SyncChannels`: a `LEFT JOIN LATERAL` unions both sources per channel and picks one credential, preferring the one that is chat-scoped and unexpired (users row breaks ties).
- token-refresh-service: token type `twitch_link` flows through the standard batch loop (`GetExpiringTwitchLinkTokens`/`UpdateTwitchLinkTokens`/`MarkTwitchLinkTokenPermanentlyFailed`, same 48-hour recovery window and 30-day permanent-fail suppression).

Retention: the pg_cron cleanup that used to delete expired rows was removed (migration 118) and the retention-sweeper (ADR-0089) deliberately leaves them: expiry alone does not prove the credential is dead, and a live row still anchors EventSub eligibility. Rows cascade from `users` on account deletion.

Alternatives rejected: matching on `users.twitch_id` after fixing ID persistence (`ocs.channel_id` stores the login, and the one-slot storage problem remains); per-platform scope/token columns on users (schema bloat; the `youtube_oauth_tokens`/`kick_oauth_tokens` precedent already uses per-platform tables); telling users to create a Twitch-login account (works but unacceptable UX and trips the duplicate-account guard).

## Consequences

- The add-source "Connect Twitch" flow works identically for every account type, and these accounts land on EventSub, the only Twitch chat path under ADR-0026.
- EventSub eligibility is deterministic: valid+scoped first, users row over linked.
- A linked credential dying independently of the account (revoked refresh token: expiry pushed 30 days ahead recorded by token-refresh) removes the channel from the partition; with no IRC fallback its chat goes quiet until the credential is repaired by re-adding with `force_verify`.
- Linking also records the platform id: `UserRepository.SetPlatformID` fills `twitch_id`/`google_id`/`kick_id` only when empty; migration 121 backfills `twitch_id`/`kick_id` from the token tables (YouTube not backfilled: those rows store channel ids, not the Google id). A login with a linked platform then lands on the linking account (before it missed the account and failed creating a second one on the username index). That login authenticates only; profile fields, tokens and scopes stay the login provider's (`linkMayReplacePrimaryCredentials`). Streamer sends as themselves to Twitch still require a Twitch-login account.
- Disconnecting a linked platform in Settings (ADR-0090) clears the id and the tokens together, so that platform's login stops reaching the account.

## Verification

`platform_auth_v2_link_test.go` (storage decision, persistence), `source_repo_test.go` (predicate on linked rows), `query_test.go` (credential selection incl. linked-beats-scope-less), token-refresh-service query-guard and routing tests, migration rerun idempotency (`migrations_rerun_test.go`).

Related: ADR-0015 (partition), ADR-0026 (IRC deprecation), ADR-0089 (retention), ADR-0090 (disconnect).
