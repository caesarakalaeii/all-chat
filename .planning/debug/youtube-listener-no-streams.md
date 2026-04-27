---
status: awaiting_human_verify
trigger: "youtube-listener-no-streams: User added a YouTube source to test another feature, but the YouTube listener is not picking up streams. No messages appear."
created: 2026-03-25T00:00:00Z
updated: 2026-03-25T00:00:00Z
---

## Current Focus

hypothesis: Two confirmed structural issues found via code review. (1) The entire detection pipeline requires an overlay to be actively connected (WebSocket open in browser). (2) OAuth token copy uses `expiry > NOW()` which fails silently if admin token is expired - leaving the channel with no token.
test: Need to verify which issue applies to the user's specific test scenario
expecting: One or both of these explains "no messages at all"
next_action: Fix the OAuth token copy to also accept expired tokens (the token-refresh-service will refresh them), and add a fallback for channels without connected overlays

## Symptoms

expected: YouTube listener should detect newly added sources AND poll active streams for live chat messages
actual: No messages appear at all from YouTube listener
errors: Unknown - need to investigate logs
reproduction: Add a YouTube source via the overlay manager
started: Unclear - hasn't been tested recently

## Eliminated

- hypothesis: Circuit breaker blocking detection (in-memory, resets on restart)
  evidence: CircuitBreaker is in-memory only (map in Manager struct). No persistence. Fresh start = CircuitClosed for all channels.
  timestamp: 2026-03-25

- hypothesis: Quota exhaustion for new channel
  evidence: New channel gets daily_quota_limit=100. Full detection costs 100 units. 0+100 > 100 is false, so quota passes for the first detection.
  timestamp: 2026-03-25

- hypothesis: getActiveSources filtering (is_active=true) blocking new inactive sources
  evidence: syncStreams uses GetAllSources (no is_active filter) for the inactiveSourcesWithConnection path, which handles newly added sources.
  timestamp: 2026-03-25

- hypothesis: Recent code changes broke YouTube listener
  evidence: git log shows no youtube-listener changes in recent commits. Last change was cfg.Platform="youtube" in 94dd1ba which is harmless.
  timestamp: 2026-03-25

## Evidence

- timestamp: 2026-03-25
  checked: services/overlay-manager/handlers/sources.go HandleAddSource line 409
  found: IsActive is set to `false` for all non-shared_overlay platforms including YouTube
  implication: New YouTube sources start inactive. Detection only happens via the inactiveSourcesWithConnection code path, which requires a connected overlay.

- timestamp: 2026-03-25
  checked: services/youtube-listener/streams/manager.go syncStreams lines 482-509
  found: connectedSources is filtered to sources where m.connectedOverlays[source.OverlayID] exists. This map is populated only when a browser/frontend is actively viewing the overlay via WebSocket.
  implication: If no frontend is open on the overlay display page, NO YouTube detection runs at all. Not on add, not on periodic sync.

- timestamp: 2026-03-25
  checked: services/overlay-manager/handlers/sources.go copyYouTubeTokenForChannel line 117-134
  found: SELECT query has `AND expiry > NOW()` - only copies non-expired access tokens. OAuth access tokens expire in ~1 hour. If admin token is expired, copy fails silently (warning only), source is created with no token row.
  implication: When youtube-listener later calls CreateYouTubeService → GetToken, it gets "no rows" error. Source gets set is_active=false and detection fails permanently.

- timestamp: 2026-03-25
  checked: services/youtube-listener/streams/backoff_store.go
  found: Backoff state persists to Redis with 24-hour TTL. Negative cache persists 2-10 minutes depending on consecutive offline checks.
  implication: If channel was previously tested when offline, backoff and negative cache in Redis may still block detection.

- timestamp: 2026-03-25
  checked: services/youtube-listener/streams/manager.go Start() line 198-200
  found: "Skip initial sync to avoid quota usage on pod restarts" - initial sync is explicitly disabled.
  implication: On pod start, no detection runs until either (a) PostgreSQL NOTIFY fires, or (b) 30-second periodic sync fires. In both cases, still gated on connected overlays.

## Resolution

root_cause: Two separate issues that together explain "no messages": (1) PRIMARY BUG - OAuth token copy in copyYouTubeTokenForChannel (overlay-manager/handlers/sources.go) uses `AND expiry > NOW()` filter. OAuth access tokens expire in ~1 hour. If the admin token's access_token is expired (even though the refresh_token is still valid), no token is copied to the new channel. The youtube-listener then fails on CreateYouTubeService with "no rows" and sets is_active=false, blocking detection permanently. (2) DESIGN CONSTRAINT - The entire detection pipeline is gated on an overlay being actively connected (browser open on overlay page). No frontend open = no detection. Same issue applies to copyKickTokenForChannel.
fix: Removed `AND expiry > NOW()` from both copyYouTubeTokenForChannel and copyKickTokenForChannel queries. Token is now copied regardless of access_token expiry; youtube-listener's GetToken() handles access_token refresh via refresh_token on first use. Error message updated to indicate "no token at all" rather than "expired token". If persistent Redis backoff state is blocking detection for a previously-tested channel, use admin endpoint POST /admin/detection/channels/{channel_id}/reset-backoff.
verification:
files_changed:
  - services/overlay-manager/handlers/sources.go
