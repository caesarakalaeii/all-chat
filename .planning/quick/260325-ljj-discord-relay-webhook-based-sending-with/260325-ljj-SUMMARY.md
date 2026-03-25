---
phase: quick
plan: 260325-ljj
subsystem: relay
tags: [discord, webhook, relay, go]

requires:
  - phase: v1.5-discord-listener
    provides: discord-listener relay package with channel-based posting
provides:
  - Webhook-based Discord relay with per-message sender identity (avatar + display name)
affects: [discord-listener, overlay-manager]

tech-stack:
  added: []
  patterns:
    - "Discord webhook POST with username/avatar_url fields for per-message identity"

key-files:
  created: []
  modified:
    - services/discord-listener/relay/poster.go
    - services/discord-listener/relay/poster_test.go
    - services/discord-listener/relay/manager.go
    - services/discord-listener/relay/manager_test.go
    - services/discord-listener/relay/repository.go
    - services/discord-listener/cmd/main.go

key-decisions:
  - "Webhook URL passed per-call (not stored in poster) -- allows multi-overlay routing"
  - "avatar_url omitted from JSON when empty (omitempty tag) to avoid Discord showing broken image"
  - "HandleMessage signature expanded to 8 args for full identity passthrough"

patterns-established:
  - "Webhook identity: formatWebhookUsername returns 'displayName [Platform]' with title-cased platform"

requirements-completed: [WEBHOOK-RELAY]

duration: 5min
completed: 2026-03-25
---

# Quick Task 260325-ljj: Discord Relay Webhook-Based Sending Summary

**Webhook-based Discord relay posting with per-message sender avatar, display name, and platform badge via formatWebhookUsername**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-25T14:35:46Z
- **Completed:** 2026-03-25T14:40:26Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Replaced bot-token channel messages with Discord webhook POSTs carrying sender identity
- RelayPayload struct delivers content, username ("alice [Twitch]"), and avatar_url per message
- Repository now queries relay_webhook_url from JSONB config instead of relay_channel_id
- relayMessage parses display_name and avatar_url from Redis Pub/Sub messages
- 15 tests covering webhook posting, format, rate limiting, silent drops, and manager relay

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace channel poster with webhook poster and update DiscordPoster interface** - `1f1b1b6` (feat)
2. **Task 2: Update repository, relayMessage, and manager to use webhook URL and sender identity** - `ed83501` (feat)

## Files Created/Modified
- `services/discord-listener/relay/poster.go` - webhookPoster replaces httpPoster, RelayPayload struct, formatWebhookUsername
- `services/discord-listener/relay/poster_test.go` - 12 tests for webhook posting behavior and username formatting
- `services/discord-listener/relay/manager.go` - relayMessage with display_name/avatar_url, drainOverlay uses webhook identity
- `services/discord-listener/relay/manager_test.go` - 3 tests for manager relay with new interface
- `services/discord-listener/relay/repository.go` - SQL queries relay_webhook_url from JSONB config
- `services/discord-listener/cmd/main.go` - NewWebhookPoster replaces NewHTTPPoster

## Decisions Made
- Webhook URL passed per-call (not stored in poster) -- each overlay can have a different webhook
- avatar_url uses omitempty JSON tag to avoid sending empty string to Discord
- HandleMessage signature expanded with displayName, avatarURL, webhookURL for full identity passthrough

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required. Users must configure `relay_webhook_url` in their Discord chat source config JSONB (instead of `relay_channel_id`) for webhooks to work.

## Next Phase Readiness
- Overlay manager UI will need updating to collect webhook URL instead of channel ID
- Database migration may be needed if existing relay configs use relay_channel_id (graceful skip via IS NOT NULL filter)

---
*Quick Task: 260325-ljj*
*Completed: 2026-03-25*
