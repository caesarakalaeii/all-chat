---
phase: 02-youtube-integration
plan: 01
subsystem: youtube-listener
tags: [deletion-events, parser, youtube-api, phase-1-integration]
requires: [01-01-message-registry, 01-03-message-processor]
provides: [youtube-deletion-event-mapping]
affects: [message-processor]
tech_stack:
  added: []
  patterns: [youtube-api-event-parsing, phase-1-schema-mapping]
key_files:
  created: []
  modified:
    - services/youtube-listener/api/parser.go
    - services/youtube-listener/api/parser_test.go
decisions:
  - id: youtube-deletion-schema
    summary: Map YouTube deletion events to Phase 1 unified schema
    rationale: YouTube API provides messageDeletedEvent and userBannedEvent natively - converting to Phase 1 schema (EventType=message_deletion, deletion_type field) enables direct integration with existing message-processor pipeline
    alternatives_considered:
      - Keep YouTube-specific event types and add special handling in message-processor (rejected - increases coupling)
    impact: YouTube deletion events now flow through Phase 1 deletion pipeline without additional processor changes
metrics:
  duration_minutes: 2.5
  completed_date: 2026-02-18
  tasks_completed: 2
  files_modified: 2
  tests_added: 3
  commits: 2
---

# Phase 2 Plan 1: YouTube Deletion Event Parser Mapping Summary

**One-liner:** YouTube deletion events (messageDeletedEvent, userBannedEvent) mapped to Phase 1 unified deletion schema with EventType=message_deletion

## What Was Built

Converted YouTube API deletion events to Phase 1 deletion schema in the YouTube Listener parser:

1. **messageDeletedEvent → Single Deletion**
   - EventType: "message_deletion" (was "message_deleted")
   - deletion_type: "single"
   - target_msg_id: DeletedMessageId from YouTube API

2. **userBannedEvent → Batch Deletion**
   - EventType: "message_deletion" (was "user_banned")
   - deletion_type: "batch"
   - target_user_id: BannedUserDetails.ChannelId
   - target_username: BannedUserDetails.DisplayName
   - Preserved ban metadata (ban_type, ban_duration_seconds) for logging

3. **Registry Integration**
   - Added youtube_message_id to tags map for all messages
   - Enables registry Add() calls in Plan 02-02

## Implementation Details

### Modified Files

**services/youtube-listener/api/parser.go** (Lines 98-123)
- Changed messageDeletedEvent to Phase 1 single deletion schema
- Changed userBannedEvent to Phase 1 batch deletion schema
- Added youtube_message_id to tags map
- Removed text field for deletion events (empty string)

**services/youtube-listener/api/parser_test.go** (Lines 257-362)
- Added TestParseYouTubeDeletionEvents with 3 test cases
- MessageDeletedEvent test validates single deletion schema
- UserBannedEvent_Permanent test validates batch deletion (permanent ban)
- UserBannedEvent_Temporary test validates batch deletion with timeout duration

### Phase 1 Schema Compliance

YouTube deletion events now match the Phase 1 schema expected by message-processor:

```go
// Single deletion (messageDeletedEvent)
{
  EventType: "message_deletion",
  EventData: {
    "deletion_type": "single",
    "target_msg_id": "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JB..."
  },
  Tags: {
    "youtube_message_id": "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JB..."
  }
}

// Batch deletion (userBannedEvent)
{
  EventType: "message_deletion",
  EventData: {
    "deletion_type": "batch",
    "target_user_id": "UC_banned_user_id",
    "target_username": "BannedUser",
    "ban_type": "permanent",         // metadata
    "ban_duration_seconds": 600      // metadata (if temporary)
  },
  Tags: {
    "youtube_message_id": "CjoKGkNKem93WjJkQkljQ0ZRcWFRZ29kSnU4T0JB..."
  }
}
```

### Integration Points

**With Message Processor** (services/message-processor/consumer/stream_consumer.go:284)
- EventType "message_deletion" triggers processDeletionEvent()
- deletion_type field routes to single/batch handling
- target_msg_id used for registry lookups (single deletion)
- target_user_id used for batch filtering (user ban/timeout)

**Registry Ready** (Plan 02-02)
- youtube_message_id in tags enables registry.Add() calls
- Format: `chat:registry:{platform}:{channel_id}` → {youtube_message_id}:{uuid}

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Missing go.sum entry**
- **Found during:** Task 1 build verification
- **Issue:** go.sum missing entry for github.com/exaring/otelpgx dependency
- **Fix:** Ran `go mod download github.com/exaring/otelpgx` to update go.sum
- **Files modified:** services/youtube-listener/go.sum
- **Commit:** d00c700 (included in Task 1 commit)

**2. [Rule 3 - Blocking] YouTube API type naming**
- **Found during:** Task 2 test compilation
- **Issue:** Used incorrect type name `youtube.LiveChatUserBannedDetails` (does not exist in API)
- **Fix:** Changed to correct type `youtube.LiveChatUserBannedMessageDetails`
- **Files modified:** services/youtube-listener/api/parser_test.go
- **Commit:** 4f4f000 (included in Task 2 commit)

No architectural decisions required - all issues were straightforward type/dependency fixes.

## Test Results

All tests passed successfully:

```bash
=== RUN   TestParseYouTubeDeletionEvents
=== RUN   TestParseYouTubeDeletionEvents/MessageDeletedEvent
=== RUN   TestParseYouTubeDeletionEvents/UserBannedEvent_Permanent
=== RUN   TestParseYouTubeDeletionEvents/UserBannedEvent_Temporary
--- PASS: TestParseYouTubeDeletionEvents (0.00s)
    --- PASS: TestParseYouTubeDeletionEvents/MessageDeletedEvent (0.00s)
    --- PASS: TestParseYouTubeDeletionEvents/UserBannedEvent_Permanent (0.00s)
    --- PASS: TestParseYouTubeDeletionEvents/UserBannedEvent_Temporary (0.00s)
PASS
```

**Coverage:** ParseChatMessage function coverage increased to 75.8% with deletion event tests.

## Verification

### Schema Verification
- ✅ EventType = "message_deletion" (2 occurrences: messageDeletedEvent + userBannedEvent)
- ✅ deletion_type field present for single/batch routing
- ✅ target_msg_id present for registry lookup
- ✅ target_user_id present for batch filtering
- ✅ youtube_message_id in tags for registry integration

### Build Verification
- ✅ `go build ./cmd/` succeeds with no errors
- ✅ All existing tests still pass
- ✅ 3 new deletion event tests pass

### Integration Readiness
- ✅ EventType matches message-processor check (line 284)
- ✅ Schema fields match Phase 1 expectations
- ✅ Ready for registry integration in Plan 02-02

## Success Criteria Met

1. ✅ YouTube parser converts messageDeletedEvent to Phase 1 single deletion schema
2. ✅ YouTube parser converts userBannedEvent to Phase 1 batch deletion schema
3. ✅ Parser adds youtube_message_id to tags map for every message
4. ✅ Three deletion event tests pass with Phase 1 schema validation
5. ✅ Build succeeds with no compilation errors
6. ✅ Deletion events ready to flow through message-processor pipeline

## Key Decisions

**Decision: Map YouTube events to Phase 1 schema in parser (not processor)**
- **Context:** YouTube API provides deletion events natively in polling responses
- **Decision:** Convert to Phase 1 schema in YouTube Listener parser rather than adding YouTube-specific handling in message-processor
- **Rationale:** Maintains platform-agnostic processor design, YouTube Listener already has platform-specific parsing logic
- **Impact:** Zero processor changes needed, deletion events flow through existing pipeline

## Next Steps

**Plan 02-02: YouTube Registry Integration**
- Add registry.Add() calls for YouTube messages in message handler
- Implement registry integration tests
- Verify end-to-end deletion flow with YouTube events

**Dependencies satisfied for Plan 02-02:**
- ✅ YouTube deletion events produce EventType = "message_deletion"
- ✅ youtube_message_id available in tags for registry Add()
- ✅ Schema matches Phase 1 expectations

## Commits

| Commit | Type | Description |
|--------|------|-------------|
| d00c700 | feat | Convert YouTube deletion events to Phase 1 schema |
| 4f4f000 | test | Add comprehensive YouTube deletion event test coverage |

## Self-Check: PASSED

**Created files verified:**
- N/A (no new files created)

**Modified files verified:**
```bash
FOUND: services/youtube-listener/api/parser.go
FOUND: services/youtube-listener/api/parser_test.go
```

**Commits verified:**
```bash
FOUND: d00c700
FOUND: 4f4f000
```

All artifacts validated successfully.
