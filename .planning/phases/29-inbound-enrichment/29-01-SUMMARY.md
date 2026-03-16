---
phase: 29-inbound-enrichment
plan: "01"
subsystem: messaging
tags: [discord, gateway, redis-streams, message-deletion, tdd]

# Dependency graph
requires:
  - phase: 28-inbound-listener-core
    provides: HandleMessageCreate, ChannelRegistry, MessagePublisher, GatewayClient struct
provides:
  - MessageDeleteData and MessageDeleteBulkData structs in gateway/types.go
  - HandleMessageDelete method with channel filter and deletion event publish
  - HandleMessageDeleteBulk method dispatching per-ID via HandleMessageDelete
  - MESSAGE_DELETE and MESSAGE_DELETE_BULK dispatch wiring in Connect() loop
affects: [message-processor, 29-inbound-enrichment]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HandleMessageDelete mirrors HandleMessageCreate channel filter — same registry, same silent-drop-on-unknown behavior"
    - "Deletion event uses msg.ID + ':del' as message_id to distinguish from create events in Redis Streams"
    - "Bulk deletes decomposed into individual HandleMessageDelete calls — single-path logic, no duplication"

key-files:
  created:
    - services/discord-listener/gateway/message_delete_test.go
  modified:
    - services/discord-listener/gateway/types.go
    - services/discord-listener/gateway/client.go

key-decisions:
  - "HandleMessageDeleteBulk delegates to HandleMessageDelete per ID — consistent channel filter, no duplicated registry lookup path"
  - "message_id in deletion event uses snowflake + ':del' suffix to prevent Redis Stream key collision with original create event"
  - "capturePayloadPublisher defined separately from capturePublisher — delete tests need payload inspection, create tests do not"

patterns-established:
  - "Deletion event schema: event_type=message_deletion, event_data.deletion_type=single, event_data.target_msg_id=snowflake"
  - "Registry error handling: log WARN, return nil — identical to HandleMessageCreate pattern"

requirements-completed: [INBD-03]

# Metrics
duration: 2min
completed: 2026-03-16
---

# Phase 29 Plan 01: MESSAGE_DELETE Dispatch Summary

**Discord MESSAGE_DELETE and MESSAGE_DELETE_BULK dispatch with channel filter publishing deletion events (event_type=message_deletion, target_msg_id=snowflake) to chat:raw Redis Stream**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-16T07:40:05Z
- **Completed:** 2026-03-16T07:42:27Z
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files modified:** 3

## Accomplishments
- Added `MessageDeleteData` and `MessageDeleteBulkData` structs to gateway/types.go
- Implemented `HandleMessageDelete` with identical channel filter to `HandleMessageCreate` — only configured channels emit deletion events
- Implemented `HandleMessageDeleteBulk` decomposing bulk deletes into per-ID `HandleMessageDelete` calls
- Wired both dispatch branches into the `Connect()` OpDispatch switch immediately after MESSAGE_CREATE
- 5 TDD tests covering: unknown channel, happy path (payload assertions), registry error, bulk happy path (3 distinct target_msg_ids), bulk unknown channel

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add delete types + write failing tests** - `767e18b` (test)
2. **Task 2 (GREEN): Implement HandleMessageDelete + dispatch wiring** - `a2e8077` (feat)

## Files Created/Modified
- `services/discord-listener/gateway/types.go` - Added MessageDeleteData and MessageDeleteBulkData structs
- `services/discord-listener/gateway/client.go` - Added HandleMessageDelete, HandleMessageDeleteBulk methods; MESSAGE_DELETE and MESSAGE_DELETE_BULK dispatch wiring in Connect()
- `services/discord-listener/gateway/message_delete_test.go` - 5 tests with capturePayloadPublisher for payload inspection

## Decisions Made
- `HandleMessageDeleteBulk` delegates entirely to `HandleMessageDelete` per ID — avoids duplicating registry lookup logic, single code path for channel filter
- `message_id` uses `snowflake + ":del"` suffix to distinguish deletion events from create events in Redis Stream consumer group processing
- `capturePayloadPublisher` defined in message_delete_test.go (separate from `capturePublisher` in message_create_test.go) — deletion tests require payload field assertions; create tests only count calls

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None — TDD cycle was clean. RED confirmed with "undefined: HandleMessageDelete" build errors; GREEN all 5 tests passed on first implementation.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- MESSAGE_DELETE dispatch complete and tested; downstream message-processor can now receive and handle Discord deletion events via the existing chat:raw Redis Stream consumer group
- Phase 29 plan 02+ can build on this foundation for further inbound enrichment

---
*Phase: 29-inbound-enrichment*
*Completed: 2026-03-16*

## Self-Check: PASSED

- FOUND: services/discord-listener/gateway/types.go
- FOUND: services/discord-listener/gateway/client.go
- FOUND: services/discord-listener/gateway/message_delete_test.go
- FOUND: .planning/phases/29-inbound-enrichment/29-01-SUMMARY.md
- FOUND commit: 767e18b (test RED)
- FOUND commit: a2e8077 (feat GREEN)
