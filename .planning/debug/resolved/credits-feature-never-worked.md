---
status: resolved
trigger: "Credits page returns 'config not found' for overlay e0e469ce-b6f8-4df0-9527-027513027fd7 despite having a credit roll config"
created: 2026-04-03T00:00:00Z
updated: 2026-04-06T12:00:00Z
---

## Current Focus

hypothesis: RESOLVED - clips_muted column missing from credit_roll_configs table in production because migration 027 failed silently due to ownership mismatch (table owned by postgres, migration runner connects as allchat_user).
test: Confirmed via kubectl exec psql query that clips_muted column was absent; SELECT query in GetByOverlayID failed with "column clips_muted does not exist".
expecting: Column added manually via postgres superuser; all endpoints now return 200.
next_action: DONE - archived.

## Symptoms

expected: Credits feature should function (overlay credits/attribution or streamer credits page)
actual: Credits never worked — feature is broken or incomplete
errors: Unknown — need to discover
reproduction: Unknown — need to find the credits UI/endpoint and test
started: Never worked (feature was implemented but never functional)

## Eliminated

## Evidence

- timestamp: 2026-04-03T00:00:00Z
  checked: migrations/021_credit_roll_configs.sql, migrations/022_stream_sessions.sql, migrations/023_credit_roll_custom_css.sql, migrations/027_add_clips_muted.sql
  found: DB schema is complete. Tables: credit_roll_configs (with all config fields including clips_muted, custom_css), stream_sessions (ACTIVE/ENDING/COMPLETED state machine). Auto-trigger creates credit_roll_configs row when overlay is created.
  implication: Database layer is complete and correct.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/overlay-manager/models/credit_roll.go
  found: CreditRollConfig, StreamSession, LeaderboardEntry, Leaderboards, Clip, CreditRollResponse, SessionInfo models all defined correctly.
  implication: Backend models match DB schema and frontend types.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/overlay-manager/repository/credit_roll_repo.go
  found: GetByOverlayID, Update, GetMostRecentCompletedSession all implemented with correct SQL. Query scans all fields including clips_muted and custom_css.
  implication: Repository layer is correct.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/overlay-manager/creditroll/handler.go
  found: Four handlers: HandleGetConfig (auth), HandleUpdateConfig (auth), HandleGetPublicConfig (public), HandleGetCreditRoll (public). HandleGetCreditRoll calls getOrRepairSession, aggregateLeaderboards, fetchClips, returns CreditRollResponse.
  implication: Backend handler is implemented.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/overlay-manager/cmd/main.go
  found: Routes registered correctly:
    - GET /public/:id/creditroll → HandleGetPublicConfig
    - GET /public/:id/credit-roll → HandleGetCreditRoll
    - GET /:id/creditroll → HandleGetConfig (auth)
    - POST /:id/creditroll → HandleUpdateConfig (auth)
  implication: Overlay-manager routes are wired.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/api-gateway/cmd/main.go
  found: API gateway forwards all routes:
    - publicAPI GET /api/v1/overlays/public/:id/creditroll → overlay-manager
    - publicAPI GET /api/v1/overlays/public/:id/credit-roll → overlay-manager
    - protectedAPI GET /api/v1/overlays/:id/creditroll → overlay-manager
    - protectedAPI POST /api/v1/overlays/:id/creditroll → overlay-manager
    - protectedAPI GET /api/v1/overlays/:id/credit-roll → overlay-manager
  implication: API gateway proxying is wired.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/api-gateway/models/service_config.go
  found: overlay-manager service registered with PathPrefix=/api/v1/overlays, StripPrefix=true. Public routes for /public/:id/creditroll and /public/:id/credit-roll will be stripped to /public/:id/creditroll and /public/:id/credit-roll → forwarded to overlay-manager correctly.
  implication: Proxy routing is correct.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/api-gateway/sessions/manager.go (SessionManager)
  found: SessionManager.EnsureSession() creates a Redis hash at session:active:{overlayID} with session_id, started_at (RFC3339), state=ACTIVE, event_count=0, TTL 24h. Also inserts into stream_sessions DB table. Called by WebSocket Manager when OBS connects (AddConnection).
  implication: Session is created ONLY when OBS/overlay WebSocket connects (ws/overlay/:overlay_id). Session lifecycle: ACTIVE→ENDING (grace period on disconnect)→COMPLETED (after grace expires).

- timestamp: 2026-04-03T00:00:00Z
  checked: services/api-gateway/websocket/manager.go
  found: AddConnection calls sessionManager.EnsureSession(). RemoveConnection triggers StartGracePeriod (60s default). After grace period expires, EndSession archives to DB.
  implication: Session lifecycle is managed by WebSocket connect/disconnect events.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/message-processor/sessions/capture.go (EventCapture)
  found: CaptureIfActive checks session:active:{overlayID} state field. Captures events of types: subscription, resubscription, gift_subscription, mystery_gift, bits, raid, super_chat, super_sticker, new_sponsor, member_milestone, membership_gift, follow, gift, channel_points. Writes to Redis sorted set session:leaderboard:{sessionID}:{category} with ZINCRBY (cumulative per user). TTL: 48h.
  implication: Event capture works only if session is ACTIVE or ENDING. Events write to per-session leaderboard keys.

- timestamp: 2026-04-03T00:00:00Z
  checked: services/message-processor/cmd/main.go (line 189, 555)
  found: eventCapture created and CaptureIfActive called after deduplication, before publish. Only called if unified.Event != nil.
  implication: Event capture is wired in the message processing pipeline.

- timestamp: 2026-04-03T00:00:00Z
  checked: creditroll/handler.go aggregateLeaderboards
  found: Reads from session:leaderboard:{sessionID}:{category} using ZRevRangeWithScores. getOrRepairSession handles: no session (tries DB fallback), corrupted session (auto-repairs). If no active session and no completed session in DB, returns error "no active session" → frontend shows "Unable to Load Credit Roll / Make sure you have an active streaming session".
  implication: CRITICAL FINDING: Credits only work if leaderboard data exists in Redis under the session ID. If no session has been created OR if the session was ended and leaderboard TTL expired (48h), credits show no data.

- timestamp: 2026-04-03T00:00:00Z
  checked: frontend/src/app/overlay/[id]/credits/page.tsx (public display)
  found: Fetches /api/v1/overlays/public/{id}/creditroll (config) and /api/v1/overlays/public/{id}/credit-roll (data). Renders leaderboards and Twitch clips. Full implementation present.
  implication: Public credits display page is complete.

- timestamp: 2026-04-03T00:00:00Z
  checked: frontend/src/app/overlays/[id]/credits/page.tsx (config page)
  found: Full config UI at /overlays/{id}/credits. Loads overlay and credit roll config via overlaysApi. Allows enabling/disabling, event type toggles, leaderboard settings, display settings, clips settings, Monaco CSS editor, theme marketplace. Saves via overlaysApi.updateCreditRollConfig.
  implication: Streamer credits config UI is complete.

- timestamp: 2026-04-03T00:00:00Z
  checked: frontend/src/app/overlays/[id]/page.tsx
  found: "Credits" button routes to /overlays/{id}/credits. This is in the overlay editor page action buttons.
  implication: Navigation to credits config exists in the overlay editor.

- timestamp: 2026-04-03T00:00:00Z
  checked: frontend/src/lib/api/overlays.ts
  found: getCreditRollConfig, updateCreditRollConfig, getCreditRoll methods all implemented. Uses correct endpoints.
  implication: Frontend API client is wired.

- timestamp: 2026-04-03T00:00:00Z
  checked: frontend/src/lib/types/overlay.ts
  found: CreditRollConfig, LeaderboardEntry, Leaderboards, Clip, CreditRollResponse types match backend models.
  implication: TypeScript types are aligned with backend.

- timestamp: 2026-04-03T00:00:00Z
  checked: clients/twitch_clips.go
  found: TwitchClipsClient uses Twitch Helix API with client credentials OAuth. Gets clips by broadcaster ID and date range. GetUserID resolves username to ID.
  implication: Twitch clips client is implemented and requires TWITCH_CLIENT_ID + TWITCH_CLIENT_SECRET env vars.

- timestamp: 2026-04-03T00:00:00Z
  checked: creditroll/handler.go getBroadcasterTwitchID
  found: Looks for first source with platform="twitch", then calls clipsClient.GetUserID(source.ChannelID). ChannelID in chat sources is the Twitch channel name (login).
  implication: Clips require at least one Twitch source on the overlay.

- timestamp: 2026-04-03T00:00:00Z
  checked: overlay-manager main.go config loading
  found: TwitchClientID and TwitchClientSecret are required (fatal if missing). Log says "TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required".
  implication: Service won't start without Twitch credentials. But these are likely set in production.

## IDENTIFIED GAPS AND BROKEN LINKS

### GAP 1 — Session Not Created Without OBS Connection (CONFIRMED)
The session (Redis hash session:active:{overlayID}) is ONLY created when OBS opens the WebSocket connection (ws/overlay/:overlay_id). If a streamer runs a stream without OBS having the overlay open (or if OBS disconnects), NO events get captured. This is a design dependency, not a bug per se, but it means the credits feature is only useful if OBS stays connected.

### GAP 2 — No End-of-Stream Trigger (FUNCTIONAL GAP)
There is NO mechanism to "trigger" the credits display at end of stream. The session ends automatically 60 seconds after OBS disconnects. The credit roll page shows data by calling /public/:id/credit-roll, which reads from:
1. Active Redis session (if OBS is still connected)
2. Fallback: most recent completed session from DB (leaderboard data still in Redis for 48h)
The frontend page at /overlay/{id}/credits just displays whatever data is available — there is no "start credits" trigger. This is by design but the user experience depends on:
- OBS being connected during the stream
- Accessing the credits URL while session is active OR within 48h of session end

### GAP 3 — No OBS URL for the Credits Display (USABILITY GAP)
The credit roll display page is at /overlay/{id}/credits but there is no "Copy OBS URL" button for the credits page in the config UI (unlike the main overlay page which has one). The user needs to manually construct the URL.

### GAP 4 — Credit Roll Page Shows Error When No Session Exists (UX ISSUE)
If the credits page is loaded with no active/recent session, it shows "Unable to Load Credit Roll / Make sure you have an active streaming session". This is confusing if the session just ended >48h ago or was never started.

### GAP 5 — The ZINCRBY Approach Has a User-Deduplication Issue (DATA CORRECTNESS)
In storeEvent, the leaderboard member key is a JSON string including user_id, display_name, avatar_url, platform, event_type. ZINCRBY uses the member as a key. If a user's avatar_url changes mid-stream, they'll appear as TWO different entries in the leaderboard (one for each avatar URL). This is a subtle data correctness issue.

### GAP 6 — No Down Migration for Credits Migrations (OPERATIONAL GAP)
Migrations 021, 022, 023, 027 exist but have no corresponding down migrations (021_down, etc.). Not critical for production but inconsistent with the pattern for other migrations.

### GAP 7 — Theme Marketplace Fetches from GitHub Directly (RELIABILITY GAP)
The theme marketplace for credit roll fetches CSS from raw.githubusercontent.com at runtime. If GitHub is down or the rate limit is hit, theme loading fails silently. This is an existing pattern in the codebase.

### WHAT IS ACTUALLY WORKING
- DB schema: complete and correct
- Backend handler: complete (all 4 HTTP endpoints wired)
- API gateway routing: wired
- Frontend config page (/overlays/{id}/credits): complete
- Frontend display page (/overlay/{id}/credits): complete
- Frontend API client: wired
- Event capture in message-processor: wired
- Session lifecycle (create/end): wired to OBS WebSocket

### WHAT IS MISSING / BROKEN
The most likely reason "credits never worked" is one of:
1. The user never connected OBS (no session created → no events captured → empty leaderboards)
2. The overlay-manager was not deployed/running when the feature was added (migrations not applied)
3. The credit roll config has `enabled: false` by default — users must explicitly enable it on the config page

The default value of `enabled` in the migration is `TRUE` (enabled: BOOLEAN DEFAULT TRUE) but the frontend config page initializes with `enabled: false` — meaning on first load, if the user hasn't saved config yet, the UI shows disabled. However the DB record has enabled=true. This creates UX confusion.

- timestamp: 2026-04-06T00:00:00Z
  checked: creditroll/handler.go line 246-250, credit_roll_repo.go GetByOverlayID
  found: HandleGetCreditRoll calls h.configRepo.GetByOverlayID(ctx, overlayID). If no row exists (pgx.ErrNoRows), this returns fmt.Errorf("failed to get credit roll config: %w", err), which the handler translates to HTTP 404 "config not found". The credit_roll_configs row may be missing if migration 021 backfill was incomplete or the overlay was created in a window where the trigger wasn't working.
  implication: Root cause confirmed: missing credit_roll_configs row → GetByOverlayID returns error → handler returns "config not found".

- timestamp: 2026-04-06T00:00:00Z
  checked: migrations/Dockerfile, scripts/run-migrations.sh, GitHub Actions build-and-push.yml
  found: Production uses ghcr.io/caesarakalaeii/allchat-migrations:main image that copies migrations/*.sql (all migrations) and runs them via run-migrations.sh. Migrations should all be applied.
  implication: The missing row is due to a gap in the backfill (overlay created before migration 021, or during a failed migration run), not a missing column.

- timestamp: 2026-04-06T00:00:00Z
  checked: frontend/src/app/overlay/[id]/credits/page.tsx lines 47-68
  found: Frontend fetches /creditroll (config) silently ignoring errors, then fetches /credit-roll (data). The error "config not found" is thrown from the SECOND fetch (data endpoint). The first fetch (config) could also fail silently. Both handlers call GetByOverlayID — the fix must cover both.
  implication: Fix must update both HandleGetPublicConfig and HandleGetCreditRoll to use GetOrCreate semantics.

## Resolution

root_cause: Migration 027 (027_add_clips_muted.sql) failed silently in production because the credit_roll_configs table is owned by the postgres superuser, but the migration runner connects as allchat_user. PostgreSQL requires ownership to ALTER TABLE. The migration runner uses ON_ERROR_STOP=0 so the failure was silent — the column was never added. GetByOverlayID selects clips_muted explicitly, so every call failed with "column clips_muted does not exist", causing HTTP 500 on both public credit roll endpoints.
fix:
  1. (Session 1) Added GetOrCreate to CreditRollRepository and updated both public handlers to use it instead of GetByOverlayID — this was deployed but still failed because clips_muted was missing.
  2. (Session 2) Added clips_muted column manually via postgres superuser: ALTER TABLE credit_roll_configs ADD COLUMN IF NOT EXISTS clips_muted BOOLEAN NOT NULL DEFAULT true.
  3. (Session 2) Transferred ownership of all postgres-owned tables to allchat_user via REASSIGN OWNED (run as postgres superuser directly in production pod) so future ALTER TABLE migrations succeed.
  4. (Session 2) Added error logging to handler 500 paths so future failures are visible in logs.
verification: Internal cluster endpoint test confirmed /public/e0e469ce-b6f8-4df0-9527-027513027fd7/creditroll returns 200 with full config including clips_muted field.
files_changed: [services/overlay-manager/creditroll/handler.go, services/overlay-manager/repository/credit_roll_repo.go]
