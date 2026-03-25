---
phase: quick
plan: 260325-lwo
subsystem: relay
tags: [discord, webhooks, rest-api, auto-provisioning]

requires:
  - phase: quick-260325-ljj
    provides: Discord relay webhook-based posting infrastructure
provides:
  - WebhookProvisioner that auto-creates Discord webhooks via REST API
  - GetPendingRelayConfigs and StoreWebhookURL repository methods
  - Provisioning integration in Manager.SyncRelayConfigs
affects: [discord-listener, relay]

tech-stack:
  added: []
  patterns: [idempotent webhook provisioning with existing-check-before-create]

key-files:
  created:
    - services/discord-listener/relay/webhook_provisioner.go
  modified:
    - services/discord-listener/relay/repository.go
    - services/discord-listener/relay/manager.go
    - services/discord-listener/cmd/main.go
    - services/discord-listener/relay/manager_test.go

key-decisions:
  - "Webhook name 'AllChat Relay' used as idempotency key -- existing webhooks with that name are reused"
  - "pg_notify('chat_source_changes') issued after StoreWebhookURL so LISTEN/NOTIFY watcher triggers immediate sync"
  - "WebhookProvisioner is optional (nil-safe) in Manager for backwards compatibility with tests"

patterns-established:
  - "Idempotent Discord API provisioning: list existing resources before creating"

requirements-completed: [auto-create-discord-webhook]

duration: 6min
completed: 2026-03-25
---

# Quick Task 260325-lwo: Auto-Create Discord Webhook Summary

**WebhookProvisioner auto-creates Discord webhooks when relay is enabled with channel ID but no webhook URL, eliminating manual webhook setup**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-25T14:54:10Z
- **Completed:** 2026-03-25T15:00:22Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- WebhookProvisioner auto-creates Discord webhooks for relay-enabled sources missing a webhook URL
- Existing webhooks named "AllChat Relay" are reused (no duplicates created)
- Provisioning runs before each sync cycle so newly-provisioned sources are immediately picked up
- Per-source error handling: failures do not block other overlays

## Task Commits

Each task was committed atomically:

1. **Task 1: Add WebhookProvisioner and repository methods** - `ea34e1a` (feat)
2. **Task 2: Wire provisioner into Manager and main.go** - `5b776b3` (feat)

## Files Created/Modified
- `services/discord-listener/relay/webhook_provisioner.go` - WebhookProvisioner with Discord REST API integration (list/create webhooks, rate limit handling)
- `services/discord-listener/relay/repository.go` - Added pendingRelayConfig struct, GetPendingRelayConfigs, StoreWebhookURL with NOTIFY
- `services/discord-listener/relay/manager.go` - Added provisioner field, updated NewManager signature, ProvisionPending call in SyncRelayConfigs
- `services/discord-listener/cmd/main.go` - Creates WebhookProvisioner with bot token and passes to NewManager
- `services/discord-listener/relay/manager_test.go` - Updated stubRepository to satisfy expanded RepositoryInterface

## Decisions Made
- Webhook name "AllChat Relay" used as idempotency key -- existing webhooks with that name are reused to avoid duplicates
- pg_notify('chat_source_changes') issued after StoreWebhookURL so the LISTEN/NOTIFY watcher triggers an immediate sync cycle
- WebhookProvisioner is nil-safe in Manager for backwards compatibility with tests

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated stubRepository in manager_test.go**
- **Found during:** Task 1
- **Issue:** Adding GetPendingRelayConfigs and StoreWebhookURL to RepositoryInterface broke the test stub
- **Fix:** Added no-op implementations of both methods to stubRepository
- **Files modified:** services/discord-listener/relay/manager_test.go
- **Verification:** go test ./relay/... passes
- **Committed in:** ea34e1a (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to keep tests compiling after interface expansion. No scope creep.

## Issues Encountered
- Pre-existing compilation error in gateway_test.go (mockChannelRegistry missing ListConfiguredChannels) -- out of scope, unrelated to relay changes

## User Setup Required
None - no external service configuration required. The bot must already have MANAGE_WEBHOOKS permission in target Discord channels.

## Next Phase Readiness
- Webhook auto-provisioning is fully wired and will activate on next relay sync cycle
- Users only need to set relay_enabled=true and relay_channel_id -- webhook creation is automatic

---
*Plan: quick-260325-lwo*
*Completed: 2026-03-25*
