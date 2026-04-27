---
status: verifying
trigger: "TikTok likes are not aggregated but sent as individual messages"
created: 2026-04-02T00:00:00Z
updated: 2026-04-02T00:00:00Z
---

## Current Focus

hypothesis: The aggregation key `${username}:${userId}` separates likes by individual user (TikTok @handle). Each user who likes generates their own 30-second window and their own overlay message. Multiple users liking produces multiple separate "Sent N likes" messages — appearing as "individual messages" rather than a single total.
test: Read aggregation key construction in handleLike, verify userId is populated from data.user?.uniqueId
expecting: Fix by changing aggregation key to per-stream (remove userId from key), so all users' likes are merged into one window
next_action: Apply fix to index.ts — change aggregation key from `${username}:${userId}` to just `${username}:${overlayId}`

## Symptoms

expected: TikTok likes should be aggregated over a 30-second window and delivered as a single "X likes" message, not individual like events.
actual: Each TikTok like arrives as a separate individual message in the overlay.
errors: Haven't checked logs yet — please investigate logs and code.
reproduction: Reproducible — can trigger TikTok likes on a test stream and see individual messages.
timeline: Unknown whether it ever worked correctly.

## Eliminated

- hypothesis: deduplication blocking updates
  evidence: Updates have different text (different like count) so dedup fingerprint differs; dedup is not the cause
  timestamp: 2026-04-02

- hypothesis: message_update not handled in OBS overlay
  evidence: /overlay/[id]/page.tsx has explicit message_update handling with aggregation_id lookup
  timestamp: 2026-04-02

- hypothesis: overlay_id routing issue
  evidence: overlay_id is in tags (not top-level), but routing via DB works correctly for tiktok platform
  timestamp: 2026-04-02

- hypothesis: SubscribeViewerOnly missing updates channel
  evidence: OBS overlay uses Subscribe (not SubscribeViewerOnly), gets both channels correctly
  timestamp: 2026-04-02

- hypothesis: userId always 'unknown' due to wrong field name
  evidence: TikTok's User type (from ./data.d.ts) has BOTH uniqueId and userId fields. uniqueId IS populated for real users, so aggregation key correctly uses per-user IDs.
  timestamp: 2026-04-02

## Evidence

- timestamp: 2026-04-02T00:01:00Z
  checked: services/tiktok-listener/src/index.ts handleLike()
  found: Aggregation key is `${username}:${userId}` where userId = data.user?.uniqueId || 'unknown'. TikTok User interface has uniqueId: string (the @handle). This separates likes by individual user.
  implication: Each liking user creates their own 30-second window → their own overlay message.

- timestamp: 2026-04-02T00:02:00Z
  checked: node_modules/tiktok-live-connector/dist/types/tiktok/data.d.ts User interface
  found: User interface has both `userId: string` and `uniqueId: string` (the @username handle). uniqueId is populated for real TikTok users.
  implication: The aggregation key is effectively per-user, not per-stream.

- timestamp: 2026-04-02T00:03:00Z
  checked: Like aggregation timer (startLikeAggregationPublisher)
  found: Timer runs every 5s. First publish (is_update=false) goes to main channel as chat_message (new overlay message). Updates (is_update=true) go to :updates channel as message_update (updates existing). Window closes after 30s.
  implication: Per-user aggregation produces one overlay entry per user per 30s window.

- timestamp: 2026-04-02T00:04:00Z
  checked: TikTok WebcastLikeMessage — when does TikTok send LIKE events?
  found: TikTok sends WebcastLikeMessage per user interaction. likeCount = number of likes in this event batch, totalLikeCount = cumulative stream total.
  implication: With per-user aggregation, busy streams with many users liking produce many overlay messages.

- timestamp: 2026-04-02T00:05:00Z
  checked: overlay_event_settings migration (017_event_settings.sql)
  found: tiktok_like_aggregation_window_seconds column exists with DEFAULT 30. But tiktok-listener hardcodes LIKE_AGGREGATION_WINDOW_MS = 30000 and never reads from DB.
  implication: Secondary bug: DB config is ignored. Not the root cause but should be fixed.

- timestamp: 2026-04-02T00:06:00Z
  checked: WebSocketClient in frontend/src/lib/api/websocket.ts
  found: WebSocketClient only handles chat_message, silently ignores message_update. Preview page uses WebSocketClient.
  implication: Preview page (/overlays/[id]/preview) shows only first-publish like message per window, updates not shown. Not the root cause for OBS overlay.

## Resolution

root_cause: TikTok like aggregation key is `${username}:${userId}` (per-user per-stream). When multiple users like simultaneously, each user gets their own 30-second aggregation window and their own overlay message, producing many "individual" messages rather than a single total-likes counter. The user expects ONE message showing total likes from all viewers combined.

fix: Change aggregation key from `${username}:${userId}` to `${username}` (per-stream, all users' likes merged into one window). The aggregation data already tracks overlay_id, so use `${username}:${overlayId}` to properly isolate per-overlay if the same channel appears on multiple overlays. Also remove per-user identity from the aggregation (likes are stream-wide, not per-user).

verification: Fix applied. Changed aggregation key from `${username}:${userId}` to just `${username}` (per-stream). Removed user_id and user_nickname from LikeAggregation struct. Publisher now uses channel identity (agg.username) for user_id/username fields. TypeScript compiles without new errors.
files_changed: [services/tiktok-listener/src/index.ts]
