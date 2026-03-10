---
phase: 18-revocation
verified: 2026-03-10T21:59:03Z
status: human_needed
score: 15/15 must-haves verified
re_verification: false
human_verification:
  - test: "Dashboard revocation flow end-to-end"
    expected: "Accepted share card shows Revoke button; clicking opens modal with copy 'Revoke share with [User]? This will stop message delivery immediately.'; confirming updates badge to Revoked without page reload; card remains in History tab"
    why_human: "Visual rendering of conditional Revoke button, modal copy, optimistic status badge update, and absence of page reload cannot be verified by static analysis"
  - test: "Overlay editor inactive shared_overlay source appearance"
    expected: "Revoked shared_overlay source row renders at 50% opacity with red Revoked StatusBadge; active shared_overlay source shows Revoke button (not Remove); platform sources (Twitch, YouTube, etc.) are unaffected — no opacity change, no badge"
    why_human: "CSS class application (opacity-50) and conditional badge rendering require visual inspection; platform source isolation cannot be confirmed without running the UI"
  - test: "Real-time share_revoked notification (User B in overlay editor)"
    expected: "While User B has the overlay editor open, User A revokes a share; User B sees error notification 'Your share with [User A username] was revoked' within seconds; the revoked source row greys out and badge appears without page reload"
    why_human: "WebSocket event delivery, notification timing, and source list refresh are runtime behaviors that require two browser sessions to test"
---

# Phase 18: Revocation Verification Report

**Phase Goal:** Users can revoke shares instantly with inactive source marking
**Verified:** 2026-03-10T21:59:03Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | POST /api/v1/shares/:id/revoke returns 200 when caller is sender or recipient of an accepted share | VERIFIED | `RevokeShareRequest` in shares.go lines 469-586; all 4 unit tests pass GREEN |
| 2 | POST /api/v1/shares/:id/revoke returns 403 when caller is neither sender nor recipient | VERIFIED | Auth check at line 524-528; `TestRevokeShareRequest_AuthCheck` PASS |
| 3 | POST /api/v1/shares/:id/revoke returns 409 when share status is not accepted | VERIFIED | Status check at line 531-534; `TestRevokeShareRequest_StatusCheck` PASS |
| 4 | Revocation atomically sets share_requests.status='revoked' AND overlay_chat_sources.is_active=false in one transaction | VERIFIED | Lines 536-561: two UPDATEs inside single `tx`, committed together with `tx.Commit`; `TestRevokeShareRequest_SourceDeactivation` PASS |
| 5 | WebSocket notification fired to the other user (fire-and-forget, 5s timeout) | VERIFIED | `go func()` goroutine with 5s context at lines 579-583; `notifyShareRevoked` sends `share_revoked` type to the other participant |
| 6 | GET /api/v1/overlays/:id/sources returns share_status field for shared_overlay sources | VERIFIED | `ListByOverlayID` LEFT JOIN at source_repo.go lines 75-79; `ShareStatus *string` field on ChatSource at models line 19 |
| 7 | share_status is null/absent for non-shared_overlay sources | VERIFIED | LEFT JOIN condition `ocs.platform = 'shared_overlay'` ensures NULL for other platforms; `omitempty` JSON tag omits from response |
| 8 | Dashboard accepted share cards have a Revoke button | VERIFIED | `ShareRequestCard.tsx` line 70: `onClick={() => setShowRevokeModal(true)}` inside `{request.status === 'accepted' && ...}` |
| 9 | Clicking Revoke opens confirmation modal with correct copy | VERIFIED | `RevocationConfirmModal.tsx` exists, exports `RevocationConfirmModal`; copy "Revoke share with {partnerName}?" and "This will stop message delivery immediately." present in JSX |
| 10 | Confirming calls revokeShare() and updates card status | VERIFIED | `handleRevoke` in RevocationConfirmModal.tsx calls `sharesApi.revokeShare(shareId)`; `onRevoked()` callback triggers `onUpdate()` |
| 11 | Revoked cards appear in History tab | VERIFIED | `page.tsx` line 84: `['accepted', 'rejected', 'expired', 'revoked'].includes(r.status)` |
| 12 | Inactive shared_overlay sources appear greyed out in overlay editor | VERIFIED | `overlays/[id]/page.tsx` line 793-800: `isInactiveSharedOverlay` flag; `opacity-50` class applied conditionally |
| 13 | Active shared_overlay sources have Revoke button in overlay editor | VERIFIED | Lines 821-827: `{isActiveSharedOverlay && (<button onClick={() => setRevokeTarget(source)}>Revoke</button>)}` |
| 14 | Overlay editor handles share_revoked WS event | VERIFIED | `useEffect` at line 89 opens `/ws/overlay/{id}?token={token}`; `envelope.type === 'share_revoked'` handler at line 100 sets notification and refreshes sources |
| 15 | ActivateSourcesForOverlay does not reactivate revoked/expired shared_overlay sources | VERIFIED | `subscription/repository.go` lines 90-97: NOT EXISTS subquery excludes `shared_overlay` sources with `status IN ('revoked', 'expired')` |

**Score:** 15/15 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|---------|--------|---------|
| `migrations/033_revoke_status.sql` | 'revoked' added to share_requests CHECK constraint | VERIFIED | Contains `share_requests_status_check` with all 5 statuses including 'revoked' |
| `migrations/033_revoke_status_down.sql` | Down migration reverts constraint | VERIFIED | Exists; drops and recreates constraint without 'revoked' |
| `services/share-service/models/share_request.go` | StatusRevoked constant | VERIFIED | Line 29: `StatusRevoked = "revoked"`; Validate() validStatuses map updated at line 61 |
| `services/share-service/handlers/shares.go` | RevokeShareRequest handler + notifyShareRevoked | VERIFIED | Both methods present; substantive implementation with SELECT FOR UPDATE, dual UPDATE, goroutine notify |
| `services/share-service/handlers/shares_revoke_test.go` | 4 test functions covering auth, status, success, deactivation | VERIFIED | All 4 tests pass: AuthCheck (403), StatusCheck (409), Success (200), SourceDeactivation (mock assertion) |
| `services/share-service/cmd/main.go` | Revoke route registered (non-premium) | VERIFIED | Line 137: `api.POST("/shares/:id/revoke", shareHandler.RevokeShareRequest)` |
| `services/api-gateway/cmd/main.go` | Revoke route registered (protectedAPI) | VERIFIED | Line 422: `protectedAPI.POST("/shares/:id/revoke", proxyHandler.ForwardRequest)` |
| `services/overlay-manager/models/chat_source.go` | ShareStatus field on ChatSource | VERIFIED | Line 19: `ShareStatus *string` with `json:"share_status,omitempty"` |
| `services/overlay-manager/repository/source_repo.go` | ListByOverlayID with LEFT JOIN on share_requests | VERIFIED | Lines 75-79: LEFT JOIN with `sr.id::text = ocs.channel_id` cast (uuid->text to avoid cast errors) |
| `services/api-gateway/subscription/repository.go` | ActivateSourcesForOverlay excludes revoked/expired shared_overlay sources | VERIFIED | Lines 90-97: NOT EXISTS subquery guards against revoked/expired reactivation |
| `frontend/src/lib/types/overlay.ts` | ChatSource extended with is_active and share_status | VERIFIED | Lines 64-65: `is_active: boolean` and `share_status?: 'accepted' \| 'revoked' \| 'expired'` |
| `frontend/src/lib/api/shares.ts` | revokeShare(shareId) API function | VERIFIED | Line 90: `async revokeShare(shareId: string): Promise<void>` calling POST `/api/v1/shares/${shareId}/revoke` |
| `frontend/src/app/dashboard/shares/components/RevocationConfirmModal.tsx` | Confirmation modal component | VERIFIED | Exports `RevocationConfirmModal`; calls `sharesApi.revokeShare`; loading state; toast success/error; Cancel + Revoke buttons |
| `frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx` | Revoke button on accepted cards | VERIFIED | `showRevokeModal` state; Revoke button conditional on `request.status === 'accepted'`; RevocationConfirmModal wired |
| `frontend/src/app/dashboard/shares/page.tsx` | History tab includes 'revoked' status | VERIFIED | Line 84: filter array includes 'revoked' |
| `frontend/src/app/overlays/[id]/page.tsx` | isInactiveSharedOverlay + opacity + StatusBadge + Revoke button + WS handler | VERIFIED | All 5 features present: flag, opacity-50 class, StatusBadge, Revoke button (isActiveSharedOverlay gated), share_revoked useEffect |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `services/share-service/cmd/main.go` | `handlers.RevokeShareRequest` | `api.POST /shares/:id/revoke` | WIRED | Line 137 |
| `services/api-gateway/cmd/main.go` | `share-service POST /shares/:id/revoke` | `protectedAPI.POST /shares/:id/revoke` | WIRED | Line 422 |
| `source_repo.go ListByOverlayID` | `share_requests.status` | `LEFT JOIN share_requests ON sr.id::text = ocs.channel_id` | WIRED | Lines 77-79; scan includes `&source.ShareStatus` at line 105 |
| `RevocationConfirmModal` | `sharesApi.revokeShare` | `handleRevoke async call` | WIRED | RevocationConfirmModal.tsx line 22 |
| `dashboard/shares/page.tsx History filter` | `revoked status` | `includes('revoked') in filter array` | WIRED | page.tsx line 84 |
| `overlays/[id]/page.tsx source row` | `RevocationConfirmModal` | `showRevokeModal state + conditional render` | WIRED | lines 821-827 (button), 866-875 (modal render) |
| `source.is_active` | `opacity-50 CSS class` | `isInactiveSharedOverlay conditional` | WIRED | line 800: `${isInactiveSharedOverlay ? ' opacity-50' : ''}` |
| `WebSocket onmessage` | `share_revoked handler` | `envelope.type === 'share_revoked'` | WIRED | line 100 |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| SHARE-06 | 18-00, 18-01, 18-03 | Either user can revoke share at any time | SATISFIED | RevokeShareRequest checks `senderUserID == callerUserID OR recipientUserID == callerUserID`; route is non-premium (always accessible); dashboard Revoke button present on accepted cards |
| SHARE-07 | 18-00, 18-01, 18-02, 18-03, 18-04 | Revoked or expired shares are marked as inactive (not deleted from config) | SATISFIED | Atomic dual-UPDATE sets `status='revoked'` AND `is_active=false`; overlay_chat_sources rows are NOT deleted; ActivateSourcesForOverlay excludes revoked sources; overlay editor shows greyed rows with status badge; `share_status` field returned from overlay sources API |

No orphaned requirements: all phase 18 requirements (SHARE-06, SHARE-07) are accounted for in plan frontmatter and verified in codebase.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No TODOs, FIXMEs, placeholder returns, stub handlers, or empty implementations found in any phase 18 artifacts. The single `return null` in `overlays/[id]/page.tsx` (line 420) is inside a `getPlatformIcon()` switch default case — legitimate behavior, not a stub.

### Human Verification Required

#### 1. Dashboard revocation flow end-to-end

**Test:** Navigate to /dashboard/shares, find an accepted share card in the History tab. Verify a Revoke button is visible (red pill, below the StatusBadge). Click it. Verify the modal appears with text "Revoke share with [Username]? This will stop message delivery immediately." and Cancel + red Revoke buttons. Click Cancel — modal closes with no change. Click Revoke again, then click the red Revoke button in the modal — the card's status badge should change to Revoked (red) without a page reload. The card must remain in the History tab.

**Expected:** Full flow completes; no page reload; badge updates; card stays visible in History.

**Why human:** Conditional button rendering, modal copy, optimistic badge update, and absence of page reload cannot be confirmed by static analysis alone.

#### 2. Overlay editor inactive shared_overlay source appearance

**Test:** Open an overlay that has an active shared_overlay source in the editor. Verify the source row shows a Revoke button (separate from Remove). Revoke the share — the source row should go to 50% opacity and show a red Revoked StatusBadge. Verify the Remove button is still present on the greyed row. Verify a Twitch/YouTube source on the same overlay shows no opacity change, no badge, no Revoke button.

**Expected:** Inactive shared_overlay row: opacity-50 + Revoked badge. Active shared_overlay row: Revoke button visible. Platform sources: unaffected.

**Why human:** CSS class application and visual isolation of platform sources requires the running UI.

#### 3. Real-time share_revoked notification (User B in overlay editor)

**Test:** Open the overlay editor in one browser tab as User B (the recipient of a share). In a second tab or via CLI as User A (sender), revoke the share: `curl -X POST http://localhost:8080/api/v1/shares/{share_id}/revoke -H "Authorization: Bearer {jwt_user_a}"`. Observe User B's editor tab.

**Expected:** Within seconds: a red error notification appears ("Your share with [User A username] was revoked") and the shared_overlay source row greys out + shows Revoked badge — all without page reload.

**Why human:** WebSocket event delivery, notification timing, and cross-tab real-time behavior cannot be verified by static code inspection.

### Gaps Summary

No gaps found. All 15 observable truths are verified by direct codebase inspection. Three items are flagged for human verification due to runtime/visual requirements (these are not blockers; automated checks all pass).

---

_Verified: 2026-03-10T21:59:03Z_
_Verifier: Claude (gsd-verifier)_
