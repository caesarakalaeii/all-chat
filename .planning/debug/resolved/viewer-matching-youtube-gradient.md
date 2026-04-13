---
status: resolved
trigger: "viewer-matching-youtube-gradient: Cross-platform viewer matching fails for YouTube messages. User CaesarLP sends messages in YouTube chat — they appear white (no styling) on overlay e0e469ce-b6f8-4df0-9527-027513027fd7. The same user's Twitch messages show the configured gradient correctly."
created: 2026-04-08T17:10:00Z
updated: 2026-04-08T18:00:00Z
---

## Current Focus

hypothesis: CONFIRMED AND FIXED — viewer YouTube OAuth stored Google account ID (101802728631468199113) as platform_user_id, but InnerTube messages carry YouTube channel ID (UCRs6QcV9kwHu7V0LLlIvwxQ) as AuthorExternalChannelID. Enricher lookup never matched. Fixed in code and DB record migrated.
test: DB updated: viewer_sessions and viewer_platform_identities now have platform_user_id = UCRs6QcV9kwHu7V0LLlIvwxQ for CaesarLP's YouTube identity.
expecting: Awaiting user confirmation that YouTube messages from CaesarLP now show the gradient.
next_action: User to verify gradient appears on YouTube messages

## Symptoms

expected: YouTube messages from CaesarLP should display with the same configured gradient as their Twitch messages on the overlay
actual: YouTube messages appear white (default/unstyled), Twitch messages show the gradient correctly
errors: No error messages reported — the messages appear, just without styling
reproduction: CaesarLP sends a message in YouTube chat while the overlay e0e469ce-b6f8-4df0-9527-027513027fd7 is active
timeline: Current behavior, unclear if it ever worked for YouTube

## Eliminated

- hypothesis: Cache invalidation not clearing YouTube platform cache key
  evidence: Fix already deployed in commit 0c62249 - HandlePatchCosmetics now invalidates ALL linked platform cache keys
  timestamp: 2026-04-08T17:12:00Z

## Evidence

- timestamp: 2026-04-08T17:12:00Z
  checked: services/auth-service/handlers/viewer_auth.go line 545-567
  found: Viewer YouTube OAuth callback calls ViewerYouTubeOAuth.GetUserInfo which calls oauth2/v2/userinfo, returning GoogleID (101802728631468199113). This is stored as platform_user_id.
  implication: platform_user_id for youtube viewers was the Google account ID, not the YouTube channel ID

- timestamp: 2026-04-08T17:13:00Z
  checked: services/youtube-listener-innertube/innertube/parser.go line 164
  found: parseTextMessage sets UserID = renderer.AuthorExternalChannelID — this is the YouTube channel ID (UC... format)
  implication: All YouTube messages have a UC... channel ID, never a Google account ID numeric string

- timestamp: 2026-04-08T17:14:00Z
  checked: DB viewer_platform_identities for CaesarLP
  found: platform=youtube, platform_user_id=101802728631468199113 (Google account ID) — confirmed mismatch
  implication: Enricher lookup WHERE platform_user_id='UC...' will NEVER match '101802728631468199113'

- timestamp: 2026-04-08T17:30:00Z
  checked: DB youtube_oauth_tokens and overlay_chat_sources for CaesarLP
  found: Primary YouTube channel is UCRs6QcV9kwHu7V0LLlIvwxQ (oldest token, configured in overlay sources)
  implication: This is the channel ID that appears as AuthorExternalChannelID in YouTube chat messages

- timestamp: 2026-04-08T17:40:00Z
  checked: DB after migration
  found: viewer_sessions and viewer_platform_identities both updated — youtube platform_user_id = UCRs6QcV9kwHu7V0LLlIvwxQ
  implication: Enricher lookup will now match. Redis cache cleared. Code fix deployed via commit.

## Resolution

root_cause: The viewer YouTube OAuth flow called /oauth2/v2/userinfo which returns a Google account ID (101802728631468199113 — a numeric string). This was stored as platform_user_id for youtube in both viewer_sessions and viewer_platform_identities. The InnerTube parser uses AuthorExternalChannelID (UC... YouTube channel ID format) as msg.User.ID on all YouTube messages. The enricher lookup WHERE vpi.platform = 'youtube' AND vpi.platform_user_id = $2 never matched because the two ID systems are incompatible.

fix: (1) Added GetChannelID method to ViewerYouTubeOAuth that calls youtube/v3/channels?part=id&mine=true to fetch the UC... channel ID. (2) Updated HandleYouTubeCallback to call GetChannelID after token exchange and use the channel ID as platform_user_id for all new sessions. (3) Added legacy migration path: when no session found by channel ID, check by Google account ID and migrate both viewer_sessions and viewer_platform_identities to the channel ID. (4) Added MigratePlatformUserID to ViewerIdentityRepo interface and implemented on ViewerIdentityRepository. (5) Manually migrated CaesarLP's existing DB record: 101802728631468199113 -> UCRs6QcV9kwHu7V0LLlIvwxQ in both tables. (6) Cleared stale Redis cache keys.

verification: DB confirmed: viewer_platform_identities for CaesarLP now has youtube/UCRs6QcV9kwHu7V0LLlIvwxQ. All 14 viewer handler tests pass. Awaiting user confirmation.
files_changed:
  - services/auth-service/oauth/viewer_youtube.go
  - services/auth-service/handlers/viewer_auth.go
  - services/auth-service/repository/viewer_identity_repository.go
  - services/auth-service/repository/viewer_repository.go
  - services/auth-service/handlers/viewer_resolve_test.go
