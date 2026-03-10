# Phase 18: Revocation - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Either user can revoke an active share at any time. Revoking immediately stops message delivery (Phase 17's routing SQL already gates on `status='accepted'`), marks the share as revoked, and sets `overlay_chat_sources.is_active=false` on both users' overlays — without deleting the source from their config. The overlay editor shows inactive shared_overlay sources as greyed-out with a status badge. The other user receives a real-time notification when their share is revoked.

This phase does NOT cover expiry lifecycle (Phase 19) — but inactive source display applies to both revoked and expired states.

</domain>

<decisions>
## Implementation Decisions

### Revocation entry points
- Revoke button lives in **both** the dashboard (History tab cards) AND the overlay editor (on shared_overlay source rows)
- Dashboard: Revoke button shown only on cards with `status='accepted'` — already-revoked/expired cards show status badge only
- Overlay editor: Revoke action on active shared_overlay source rows only

### Revocation UX & confirmation
- Confirmation dialog required before revoking
  - Message: "Revoke share with [User]? This will stop message delivery immediately."
  - Buttons: Revoke (destructive red) / Cancel
  - Modal pattern — same as AcceptModal, not inline
- After confirmed revocation: dashboard card stays in History tab, status badge updates in place from Active (green ✓) to Revoked (red ✗) — optimistic update on API success
- No page reload needed — UI updates reactively

### Authorization
- Both sender_user_id AND recipient_user_id can call the revoke endpoint
- Auth check: caller must be one of those two users (otherwise 403)
- No premium check needed on revocation (unblocking a share should always be allowed)

### Inactive source display in overlay editor
- Greyed-out row (reduced opacity) with a colored status badge (Revoked = red ✗, Expired = gray ⏱)
- Status badge only on `platform='shared_overlay'` sources — platform sources (Twitch, YouTube, etc.) get no badge
- All inactive shared_overlay sources (revoked + expired) use the same greyed-out appearance; the badge communicates the specific reason
- User CAN remove an inactive shared_overlay source from their overlay config (standard Remove button still works)

### Cross-side source deactivation
- Share-service updates `overlay_chat_sources` directly in the shared PostgreSQL DB — no HTTP call to overlay-manager
- Single transaction: `share_requests.status='revoked'` AND `overlay_chat_sources.is_active=false` for all rows where `channel_id = share_id AND platform = 'shared_overlay'` on both sides
- If transaction fails, both updates roll back — no partial state

### Peer notification
- WebSocket notification fired to the other user (fire-and-forget, 5s timeout — same pattern as `notifyShareAccepted` in Phase 15)
- Event type: `share_revoked`, payload: `{share_id, revoked_by_user_id}`
- User B's frontend: global toast ("Your share with [User A] was revoked") + affected source row greyed out in real time
- Toast shows on any page the user is currently on (not page-specific)

### Claude's Discretion
- Exact SQL for the revoke transaction (UPDATE share_requests + UPDATE overlay_chat_sources in one txn)
- Revoke endpoint path (DELETE /api/v1/shares/:id or POST /api/v1/shares/:id/revoke)
- WebSocket event routing through API Gateway (internal endpoint pattern from Phase 15)
- Frontend: how the WebSocket `share_revoked` event is broadcast to all open tabs/windows

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- **StatusBadge** (`frontend/src/app/dashboard/shares/components/StatusBadge.tsx`): Already handles `'revoked'` (red ✗) and `'expired'` (gray ⏱) — no changes needed for dashboard badges
- **ShareRequestCard** (`frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx`): Has Accept/Reject buttons at lines 63-64 and 70-74 — same pattern for Revoke button
- **AcceptModal** (`frontend/src/app/dashboard/shares/components/AcceptModal.tsx`): Existing modal pattern for confirmation dialogs — reuse structure for RevocationConfirmModal
- **notifyShareAccepted** (`services/share-service/handlers/shares.go` line 338): Fire-and-forget WebSocket notification — replicate for `notifyShareRevoked`
- **Dashboard History tab**: Already filters `['accepted', 'rejected', 'expired']` (line 84) — add `'revoked'` to this filter
- **Overlay editor source list** (`frontend/src/app/overlays/[id]/page.tsx`): Renders sources at line 752+ — add conditional rendering for `shared_overlay` + `is_active=false` case

### Established Patterns
- **AcceptRequest handler**: Both-user auth check pattern — replicate for revoke (sender_user_id OR recipient_user_id)
- **Transaction pattern**: Phase 15's `AcceptShareRequest` uses `SELECT FOR UPDATE` in a transaction — same approach for revoke
- **ON DELETE RESTRICT FK pattern**: Share requests stay in DB after revocation, not deleted
- **Shared PostgreSQL DB**: All services connect to the same DB — share-service can update `overlay_chat_sources` directly without cross-service HTTP

### Integration Points
- **New revoke endpoint**: `DELETE /api/v1/shares/:id` or `POST /api/v1/shares/:id/revoke` in share-service
  - Auth: JWT middleware (existing) + both-user check in handler
  - No premium middleware needed
- **overlay_chat_sources UPDATE**: `UPDATE overlay_chat_sources SET is_active=false WHERE channel_id=$1 AND platform='shared_overlay'` — runs in same transaction as share status update
- **WebSocket notification**: Internal endpoint on API Gateway (same as `notifyShareAccepted`) — `POST /internal/notify/:user_id` with `share_revoked` event
- **Frontend WebSocket subscriber**: Existing WS connection in overlay editor / dashboard — add `share_revoked` event handler

</code_context>

<specifics>
## Specific Ideas

- Revoke confirmation dialog: "Revoke share with [User]? This will stop message delivery immediately." — clear consequence statement, not just "Are you sure?"
- Overlay editor: inactive shared_overlay source row should show the share partner's name + "Revoked" badge. E.g.: "xQc's overlay (Revoked)" at 50% opacity with red badge
- Toast message for user B: "[User A] revoked their share with you" — identify the revoker by name so user B knows which share ended
- The `share_revoked` WebSocket event should carry enough info to update UI without a refetch: `{share_id, revoked_by_user_id, revoked_by_username}`

</specifics>

<deferred>
## Deferred Ideas

- Re-share after revocation (quick "Share again" button on revoked cards) — could be a Phase 19+ enhancement
- Revocation reason (free-text "why are you revoking?") — not needed for MVP
- Bulk revocation — no use case identified, deferred

</deferred>

---

*Phase: 18-revocation*
*Context gathered: 2026-03-10*
