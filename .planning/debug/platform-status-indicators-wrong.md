---
status: awaiting_human_verify
trigger: "platform-status-indicators-wrong: Overlay with one YouTube source shows all 4 platforms as connected"
created: 2026-03-31T00:00:00Z
updated: 2026-03-31T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — Two-layer bug: (1) backend broadcasts all platform statuses to all overlays via BroadcastToAll, (2) frontend filter condition has logical inversion that passes unconfigured platforms
test: Verified by reading status_subscriber.go (BroadcastToAll on line 181) and overlay page.tsx (filter condition lines 343-345)
expecting: Fix frontend filter to reject platforms not in configuredChannels
next_action: Apply fix to frontend filter logic

## Symptoms

expected: Only YouTube platform indicator should show as connected (overlay has one YouTube source)
actual: All 4 platform indicators (Twitch, YouTube, Kick, TikTok) show as connected
errors: none
reproduction: Correct initially on page load, but gets confused over longer WebSocket connections
started: Always been like this — platform indicators have never correctly reflected actual sources

## Eliminated

- hypothesis: Status tracking accumulates positives from message metadata
  evidence: Platform status comes from dedicated platform_status envelope type, not chat messages
  timestamp: 2026-03-31

## Evidence

- timestamp: 2026-03-31
  checked: services/api-gateway/subscription/status_subscriber.go line 181
  found: s.wsManager.BroadcastToAll(msgJSON) — broadcasts all platform status updates to every connected overlay WebSocket client, with no per-overlay filtering
  implication: Every overlay receives status updates for ALL platforms/channels in the entire system, not just its own

- timestamp: 2026-03-31
  checked: frontend/src/app/overlay/[id]/page.tsx lines 340-365
  found: Filter condition: `if (!channelId || !platformChannels || platformChannels.has(channelId))` — the `!platformChannels` branch passes when the platform has NO configured channels on this overlay (configuredChannels.get('twitch') returns undefined for a YouTube-only overlay). This means unconfigured platforms pass the filter.
  implication: For overlay with only YouTube: twitch/kick/tiktok have no entry in configuredChannels, so !platformChannels is true for all of them, causing all their status messages to be accepted and all 4 indicators to light up

- timestamp: 2026-03-31
  checked: frontend/src/app/overlay/[id]/page.tsx lines 152-159
  found: configuredChannels is a Map<string, Set<string>> populated from data.sources. For a YouTube-only overlay it only contains { youtube: Set<channel_id> }. No entries exist for twitch, kick, tiktok.
  implication: The filter's !platformChannels fallthrough is reached for every non-YouTube platform, making the check vacuously true

## Resolution

root_cause: Two-layer bug. Primary: backend status_subscriber.go broadcasts all platform statuses to all overlay WebSocket clients indiscriminately via BroadcastToAll(). Secondary (what triggers the symptom): frontend filter in overlay/[id]/page.tsx has inverted logic — the condition `!platformChannels` is true for platforms with no configured channels on the overlay (exactly the platforms that should be rejected), causing unconfigured platform statuses to pass through and activate their indicators.

fix: Split the filter into two explicit boolean conditions. `isPlatformConfigured = configuredChannels.size === 0 || platformChannels !== undefined` — rejects platforms not in configuredChannels once config has loaded. `isChannelMatch` is the existing channel-ID check, unchanged. Both must be true to accept a status message. Committed on branch fix/platform-status-indicators, commit d5bf7d31.

verification: awaiting human confirmation
files_changed: [frontend/src/app/overlay/[id]/page.tsx]
