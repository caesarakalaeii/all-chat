# Phase 15: Share Acceptance - Context

**Gathered:** 2026-03-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can accept share requests, establishing bidirectional overlay access with cycle prevention. This phase delivers: acceptance flow with overlay selection and expiry choice, optional immediate source addition for both users, share status indicators (active/expired/revoked), and cycle detection to prevent circular dependencies.

</domain>

<decisions>
## Implementation Decisions

### Acceptance flow & overlay selection
- **UI pattern**: Modal with form (not inline card expansion)
  - Opens on Accept button click
  - Contains: overlay dropdown + expiry options + platform badges + Accept/Cancel buttons
  - Clean separation from dashboard, doesn't disrupt card layout
- **Overlay selection**: Simple dropdown showing overlay names
  - Display all user's overlays (no filtering by active status or source count)
  - Clean, familiar pattern for 2-10 overlays
- **No overlay error**: Block acceptance if user has no overlays
  - Error message: "Create an overlay first to accept shares"
  - Prevents incomplete bidirectional flow
- **Platform context**: Show platform badges in modal
  - Same badges from card (Twitch/YouTube/Kick/TikTok logos)
  - Helps user decide which overlay to share back

### Expiry option UX
- **Input pattern**: Radio buttons with custom duration
  - ○ This stream (expires when your stream ends)
  - ○ Custom duration → [number input] hours (1-168 range)
  - ○ Unlimited (never expires)
- **Default selection**: "This stream" pre-selected
  - Most common use case for live streaming collaborations
- **Not live handling**: Allow "This stream" even if user not currently streaming
  - Share becomes active immediately, expires on next stream end
- **Custom duration validation**: Inline validation (not on-submit)
  - Red border + error text if value < 1 or > 168
  - Disable Accept button until valid
  - Clear feedback before user tries to submit

### Immediate source addition
- **Prompt timing**: Second modal immediately after acceptance
  - First modal closes (acceptance complete)
  - Second modal opens: "Add [User]'s overlay to one of yours?"
  - Dropdown: all user's overlays + Add/Skip buttons
- **Overlay dropdown**: All user's overlays (no filtering)
  - No restriction by active status or existing sources
  - User has full flexibility
- **Bidirectional prompt**: Both sender AND recipient get add-source prompt
  - Symmetric experience for both users
  - After recipient accepts → both see add-source modal
- **Sender notification strategy**: Realtime if online + prompt on visit if offline
  - If sender online: WebSocket notification triggers add-source modal immediately
  - If sender offline: Next dashboard visit shows add-source prompt
  - Requires tracking: "has_seen_acceptance" flag per share request

### Cycle detection behavior
- **Detection timing**: On acceptance submission (in modal, before closing)
  - Check when user clicks Accept button
  - If cycle detected: show error in modal, user stays in modal
  - If no cycle: proceed with acceptance
- **Error message**: Technical but clear
  - "Can't accept: This would create a circular share (You → [User] → [Other] → You). Messages would loop infinitely."
  - Explains the problem and why it's blocked
- **Algorithm depth**: Full depth graph traversal
  - DFS or BFS to detect cycles of any length
  - Not just direct cycles (A→B→A), but also A→B→C→A
- **Implementation location**: Both backend and frontend
  - Frontend: Pre-check for instant feedback (better UX)
  - Backend: Authoritative enforcement (can't be bypassed)
  - Same cycle detection logic in both places

### Message deduplication (Twitch Shared Chat overlap)
- **Problem**: User A shares overlay with B + Twitch Shared Chat enabled → B sees A's Twitch messages twice
  - Once from Twitch listener (native shared chat)
  - Once from shared overlay source
- **Solution**: Deduplicate in message-processor
  - Track: platform + message ID per overlay
  - Time window: 5 seconds
  - If duplicate seen within window: drop second occurrence
  - Works for any platform overlap (not just Twitch)
- **Phase placement**: Claude's Discretion
  - Could be Phase 15 (with acceptance) for immediate correct behavior
  - Could be Phase 17 (with message routing) as routing enhancement
  - Trade-off: Earlier implementation vs phase complexity

### Claude's Discretion
- Acceptance modal styling (spacing, shadows, animations)
- Cycle detection algorithm choice (DFS vs BFS)
- "has_seen_acceptance" flag storage (database column vs Redis key)
- WebSocket notification payload structure for sender
- Deduplication data structure (in-memory map vs Redis set)
- Whether to implement deduplication in Phase 15 or Phase 17

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- **ShareRequestCard component**: Already has Accept/Reject buttons (Phase 14)
  - `frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx`
  - onClick handlers ready to implement: lines 63-64 (Accept), 70-74 (Reject)
- **PlatformBadge component**: Shows platform icons with hover tooltips
  - Can reuse in acceptance modal
- **Dashboard page with tab filtering**: Pending/History tabs already working
  - `frontend/src/app/dashboard/shares/page.tsx`
  - Sort: most recent first, filter by status
- **sharesApi client**: API client for share requests
  - `frontend/src/lib/api/shares.ts` (assumed location)
  - Has fetchIncoming() method, need to add accept() and reject()
- **message-processor service**: Normalization and enrichment layer
  - `services/message-processor/` (Standard Go Layout)
  - Processes messages from Redis Streams before Pub/Sub
  - Natural location for deduplication logic

### Established Patterns
- **Modal components**: Frontend likely has modal primitives from Phase 14 work
  - Overlay backdrop, centered dialog, close behavior
  - Can build acceptance modal using same patterns
- **Form validation**: Inline validation with error states
  - Red border + error text pattern from existing forms
  - Apply to custom duration input
- **WebSocket notifications**: API Gateway delivers realtime events
  - Can send "share_accepted" event to sender
  - Frontend subscribes to user-specific channel
- **Database foreign keys**: ON DELETE RESTRICT pattern (Phase 14 decision)
  - share_requests table has foreign keys to users, overlays
  - Application-level cascade for cleanup logic

### Integration Points
- **share-service acceptance endpoint**: New endpoint needed
  - POST /api/v1/shares/:id/accept
  - Body: {recipient_overlay_id, expiry_option, expiry_hours?}
  - Returns: {accepted_share, sender_overlay_id_for_prompt}
- **Cycle detection service method**: New method in share-service
  - Input: sender_user_id, recipient_user_id
  - Output: bool (has_cycle), optional path description
  - Called before creating bidirectional share records
- **Frontend cycle check API**: Optional pre-check endpoint
  - GET /api/v1/shares/:id/check-cycle
  - Returns: {has_cycle: bool, message?: string}
  - For instant feedback before opening modal
- **Message-processor deduplication**: New deduplication layer
  - Before publishing to Redis Pub/Sub overlay channels
  - Track: map[overlayID]map[platformMessageID]timestamp
  - Cleanup: Remove entries older than 5 seconds
- **Sender notification**: WebSocket event + database flag
  - Event: {type: "share_accepted", share_id, recipient_overlay_id}
  - Flag: has_seen_acceptance column on share_requests or Redis key

</code_context>

<specifics>
## Specific Ideas

- Acceptance modal title: "[Sender Name] wants to share with you"
- Platform badges in modal should match card style (logos with hover tooltips)
- Expiry radio buttons vertical layout with clear spacing between options
- Custom duration input should have placeholder: "e.g., 24"
- Add-source modal title: "Add [User]'s overlay to one of yours?"
- Add-source modal should show a preview: "[User]'s overlay ([N] platforms)"
- Skip button in add-source modal should be subtle (gray, not prominent)
- Cycle error should appear as inline error in modal (not separate error modal)
- "has_seen_acceptance" flag approach: Add column to share_requests table (persistent, survives restarts)
- Deduplication message ID format: platform-specific (Twitch uses IRC message ID, YouTube uses liveChatId, etc.)

</specifics>

<deferred>
## Deferred Ideas

- Email/push notifications for share acceptance (future notification system)
- Share acceptance history log (full audit trail beyond status changes)
- "Suggest overlays" based on common platforms (ML-based recommendations)
- Bulk accept/reject for multiple pending requests (not MVP need)
- Custom cycle error messages per cycle length ("Direct loop", "3-hop loop", etc.)
- Redis-backed distributed deduplication (MVP can use in-memory per message-processor pod)

</deferred>

---

*Phase: 15-share-acceptance*
*Context gathered: 2026-03-09*
