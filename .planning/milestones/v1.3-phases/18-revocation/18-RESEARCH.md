# Phase 18: Revocation - Research

**Researched:** 2026-03-10
**Domain:** Share revocation — Go backend transaction, PostgreSQL schema, WebSocket notification, React frontend modal + inactive source display
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Revoke button lives in **both** the dashboard (History tab cards) AND the overlay editor (on shared_overlay source rows)
- Dashboard: Revoke button shown only on cards with `status='accepted'` — already-revoked/expired cards show status badge only
- Overlay editor: Revoke action on active shared_overlay source rows only
- Confirmation dialog required before revoking: "Revoke share with [User]? This will stop message delivery immediately." Buttons: Revoke (destructive red) / Cancel — modal pattern same as AcceptModal
- After confirmed revocation: dashboard card stays in History tab, status badge updates in place to Revoked (red ✗) — optimistic update on API success, no page reload
- Both sender_user_id AND recipient_user_id can call the revoke endpoint (403 if neither)
- No premium check on revocation
- Greyed-out row (50% opacity) with colored status badge: Revoked = red ✗, Expired = gray ⏱ — badge only on `platform='shared_overlay'` sources
- User CAN still remove an inactive shared_overlay source (standard Remove button still works)
- Share-service updates `overlay_chat_sources` directly in shared PostgreSQL DB — no HTTP call to overlay-manager
- Single transaction: `share_requests.status='revoked'` AND `overlay_chat_sources.is_active=false` for all rows where `channel_id = share_id AND platform = 'shared_overlay'` on both sides
- If transaction fails, both updates roll back — no partial state
- WebSocket notification fired to the other user (fire-and-forget, 5s timeout) — same pattern as `notifyShareAccepted`
- Event type: `share_revoked`, payload: `{share_id, revoked_by_user_id, revoked_by_username}`
- User B's frontend: global toast ("Your share with [User A] was revoked") + affected source row greyed out in real time

### Claude's Discretion
- Exact SQL for the revoke transaction (UPDATE share_requests + UPDATE overlay_chat_sources in one txn)
- Revoke endpoint path (`DELETE /api/v1/shares/:id` or `POST /api/v1/shares/:id/revoke`)
- WebSocket event routing through API Gateway (internal endpoint pattern from Phase 15)
- Frontend: how the WebSocket `share_revoked` event is broadcast to all open tabs/windows

### Deferred Ideas (OUT OF SCOPE)
- Re-share after revocation ("Share again" button on revoked cards)
- Revocation reason (free-text)
- Bulk revocation
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SHARE-06 | Either user can revoke share at any time | Revoke endpoint with both-user auth check; transaction pattern confirmed in AcceptShareRequest |
| SHARE-07 | Revoked or expired shares are marked as inactive (not deleted from config) | `is_active` column exists on `overlay_chat_sources` (migration 001); no schema migration needed to set it false; share_requests needs 'revoked' status added to CHECK constraint |
</phase_requirements>

## Summary

Phase 18 adds a revoke action to an existing, well-structured share system. The codebase already has all the primitives: `overlay_chat_sources.is_active` (migration 001), the `NotifyUser` internal WebSocket endpoint (api-gateway `handlers/websocket.go:297`), the transaction-with-SELECT-FOR-UPDATE pattern (AcceptShareRequest), the `notifyShareAccepted` fire-and-forget goroutine, and a `StatusBadge` component that already renders `'revoked'` correctly. The `ShareRequest` TypeScript type already includes `'revoked'` in its status union.

The two gaps that require new code: (1) 'revoked' is not in the `share_requests.status` CHECK constraint (migration 030 only allows `pending, accepted, rejected, expired`) and (2) `StatusRevoked` constant is missing from models/share_request.go. Both require a migration (033) and a one-line models change. Everything else is additive: a new handler, a new repo method, new frontend modal component, and overlay-editor conditional rendering.

The key technical decision (Claude's discretion): use `POST /api/v1/shares/:id/revoke` (not DELETE) to match the existing action-verb pattern (`/accept`, `/reject`, `/mark-seen`). DELETE semantics imply resource deletion, which contradicts the "mark inactive, never delete" requirement.

**Primary recommendation:** Use `POST /api/v1/shares/:id/revoke`, reuse the AcceptShareRequest transaction pattern (SELECT FOR UPDATE + two UPDATEs in one txn), and fire notification with `notifyShareRevoked` mirroring `notifyShareAccepted`.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| pgx/v5 | existing | PostgreSQL transaction (BEGIN, FOR UPDATE, COMMIT) | Already in use across all services |
| gin | existing | HTTP handler, param extraction, JSON binding | Service router |
| zap | existing | Structured logging | Project standard |
| react-hot-toast | existing | Global toast notifications | Already used in ShareRequestCard and AcceptModal |
| Tailwind CSS | existing | Greyed-out opacity classes (`opacity-50`) | Project standard |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| net/http | stdlib | HTTP client for notifyShareRevoked (same as notifyShareAccepted) | Firing internal WS notify |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| POST /revoke | DELETE /shares/:id | DELETE is semantically wrong here — resource is not deleted, only status changes |
| Direct DB update in handler | Repo method | Repo method keeps handlers thin, consistent with project pattern |

**Installation:** No new packages required.

## Architecture Patterns

### Recommended Project Structure

No new packages. Changes are additive within existing packages:

```
services/share-service/
├── handlers/shares.go          # + RevokeShareRequest handler + notifyShareRevoked
├── repository/share_repo.go    # + RevokeShare(ctx, shareID, revokerUserID) method
├── models/share_request.go     # + StatusRevoked constant
migrations/
└── 033_revoke_status.sql       # + 'revoked' to CHECK constraint

frontend/src/
├── app/dashboard/shares/
│   ├── components/
│   │   ├── ShareRequestCard.tsx        # + Revoke button for accepted cards
│   │   └── RevocationConfirmModal.tsx  # new modal (mirrors AcceptModal structure)
│   └── page.tsx                        # + filter 'revoked' into History tab
├── app/overlays/[id]/page.tsx          # + inactive source rendering + WS handler
└── lib/api/shares.ts                   # + revokeShare(shareId) function
```

### Pattern 1: Transaction with SELECT FOR UPDATE (existing, replicate)

**What:** Lock the share_request row, verify status='accepted' and caller authorization, then atomically update share_requests status + overlay_chat_sources is_active.
**When to use:** Any mutation that must be atomic across two tables and prevent concurrent races.

```go
// Source: services/share-service/handlers/shares.go AcceptShareRequest (lines 207-313)
tx, err := h.db.Begin(c.Request.Context())
defer tx.Rollback(c.Request.Context())

// Lock the row
query := `SELECT id, sender_user_id, recipient_user_id, status
          FROM share_requests WHERE id = $1 FOR UPDATE`
tx.QueryRow(ctx, query, shareID).Scan(...)

// Verify caller is sender or recipient (not just recipient like Accept)
if share.SenderUserID != callerID && share.RecipientUserID != callerID {
    c.JSON(403, ...)
    return
}

// Verify revocable (must be accepted)
if share.Status != models.StatusAccepted {
    c.JSON(409, gin.H{"error": "share is not active"})
    return
}

// UPDATE 1: mark share revoked
tx.Exec(ctx, `UPDATE share_requests SET status='revoked', responded_at=NOW() WHERE id=$1`, shareID)

// UPDATE 2: deactivate sources on both sides
tx.Exec(ctx, `UPDATE overlay_chat_sources SET is_active=false
              WHERE channel_id=$1 AND platform='shared_overlay'`, shareID)

tx.Commit(ctx)
```

### Pattern 2: Fire-and-Forget WebSocket Notification (existing, replicate)

**What:** Goroutine with 5s context timeout calling `POST http://api-gateway:8080/internal/ws/notify`.
**When to use:** Notifying the peer user of state change without blocking the HTTP response.

```go
// Source: services/share-service/handlers/shares.go notifyShareAccepted (lines 337-371)
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    h.notifyShareRevoked(ctx, otherUserID, shareID, revokerUserID, revokerUsername)
}()

func (h *ShareHandler) notifyShareRevoked(ctx context.Context, targetUserID, shareID, revokerUserID, revokerUsername string) {
    payload := map[string]interface{}{
        "user_id": targetUserID,
        "type":    "share_revoked",
        "data": map[string]string{
            "share_id":           shareID,
            "revoked_by_user_id": revokerUserID,
            "revoked_by_username": revokerUsername,  // needed for toast message
        },
    }
    // ... same http.Client pattern as notifyShareAccepted
}
```

### Pattern 3: Frontend RevocationConfirmModal (new, mirrors AcceptModal)

**What:** Fixed-overlay modal with backdrop, title, consequence message, Cancel + Revoke (red) buttons. Stateless — just calls API and propagates result via callbacks.

```typescript
// Mirrors: frontend/src/app/dashboard/shares/components/AcceptModal.tsx structure
interface RevocationConfirmModalProps {
  partnerName: string;    // "Revoke share with [partnerName]?"
  shareId: string;
  onClose: () => void;
  onRevoked: () => void;  // Called on successful API response
}
```

### Pattern 4: Overlay Editor — Inactive Source Conditional Rendering

**What:** In the sources map at `frontend/src/app/overlays/[id]/page.tsx:750`, add a conditional branch for `source.platform === 'shared_overlay' && !source.is_active` to render greyed-out row with badge.

The `ChatSource` type must have `is_active: boolean` added. Currently, the overlay editor renders sources without checking `is_active` — this phase adds the first conditional inactive display.

```typescript
// Add to ChatSource type in frontend/src/lib/types/overlay.ts
is_active: boolean;

// In sources.map():
const isInactiveSharedOverlay = source.platform === 'shared_overlay' && !source.is_active;

<div className={`... ${isInactiveSharedOverlay ? 'opacity-50' : ''}`}>
  {/* existing content */}
  {isInactiveSharedOverlay && (
    <StatusBadge status={source.shareStatus} size="sm" />  // 'revoked' or 'expired'
  )}
  {/* Revoke button only when active shared_overlay */}
  {source.platform === 'shared_overlay' && source.is_active && (
    <button onClick={() => setShowRevokeModal(source)}>Revoke</button>
  )}
</div>
```

Note: The overlay editor will need to know the share status ('revoked'/'expired') for the badge. This requires the sources endpoint to return this data for shared_overlay sources. Two options:
- Option A: Add `share_status` field to the `ChatSource` model returned by overlay-manager (JOIN on share_requests)
- Option B: The frontend reads share status from a separate call to share-service

**Recommendation (Claude's discretion):** Option A is cleaner — overlay-manager already queries overlay_chat_sources and can JOIN share_requests ON share_requests.id = overlay_chat_sources.channel_id WHERE platform='shared_overlay'. The `share_status` field is nullable/optional so non-shared sources are unaffected.

### Pattern 5: WebSocket share_revoked Event Handler (frontend)

**What:** The overlay editor page (`overlays/[id]/page.tsx`) needs a WebSocket connection to handle `share_revoked` events. However, the overlay editor does NOT currently have a WebSocket connection — it uses REST API calls only.

The dashboard page also has no WebSocket connection.

**Options for receiving share_revoked:**
1. **Add a user-scoped WebSocket to the overlay editor/dashboard** — the existing `/ws/overlay/:id` endpoint is overlay-scoped, not user-scoped. A user would need to be connected to at least one overlay to receive the notification.
2. **Use the existing overlay WS connection** — user B's overlay is open in OBS/browser, which is connected to `/ws/overlay/:id`. The `share_revoked` event would arrive on that connection. But the overlay page (OBS display) is not the same page as the overlay editor (dashboard).
3. **Poll on page focus** — simpler fallback: re-fetch sources when the window regains focus.

**Existing infrastructure:** `NotifyUser` in api-gateway finds connections by `GetConnectionsByUser(userID)` across ALL overlay pools (manager.go:393). This means any open overlay WebSocket connection (OBS, overlay editor if connected) will receive the notification. The overlay editor page currently has NO WebSocket — it is pure REST.

**Recommendation (Claude's discretion):** For real-time update in the overlay editor, add a lightweight user-notification WebSocket hook that connects to `/ws/overlay/:some_owned_overlay_id` and listens for `share_revoked` events. The first overlay owned by the user can serve this purpose. Alternatively: accept that real-time greying in the overlay editor is a best-effort feature — the revocation is immediate at the DB level (message routing already stops), and the UI updates on next page load. The toast notification works because user B's OBS overlay connection receives it via `GetConnectionsByUser`.

**Simpler path:** The WS notification currently only reaches users who have an open overlay WebSocket (OBS tab). If user B has no overlay open in OBS, they miss the real-time toast. This is acceptable for MVP — the CONTEXT.md says "fire-and-forget, 5s timeout" acknowledging best-effort delivery.

### Anti-Patterns to Avoid
- **HTTP call to overlay-manager to update sources:** Share-service connects to the same PostgreSQL DB — use direct SQL in transaction, not service-to-service HTTP.
- **Separate transactions for share_requests and overlay_chat_sources:** Must be a single transaction per CONTEXT.md — partial failure must roll back both.
- **Using DELETE HTTP verb:** Use POST /revoke — DELETE implies resource removal, share records are preserved.
- **Blocking HTTP response on WebSocket notification:** Always fire notification in goroutine (fire-and-forget pattern).
- **Checking only one side's sources:** The UPDATE must use `channel_id = $1 AND platform = 'shared_overlay'` which covers ALL overlays of BOTH users who added this share as a source — correct by design since channel_id stores the share_id.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic multi-table update | Application-level saga with retry | Single PostgreSQL transaction (BEGIN/COMMIT) | DB transactions are atomic — no saga needed |
| User-to-user WS notification | Custom pub/sub channel per user | Existing `NotifyUser` + `GetConnectionsByUser` | Already implemented in api-gateway, tested |
| Revoked status display | Custom badge component | Existing `StatusBadge` with `status='revoked'` | Already handles 'revoked': red ✗, 'Revoked' label |
| Confirmation modal structure | New modal from scratch | Mirror `AcceptModal.tsx` structure | Same pattern: backdrop, title, buttons, loading state |

## Common Pitfalls

### Pitfall 1: Missing 'revoked' in DB CHECK Constraint
**What goes wrong:** `UPDATE share_requests SET status='revoked'` fails with constraint violation at runtime, even though the Go code is correct.
**Why it happens:** Migration 030 defines `CHECK (status IN ('pending', 'accepted', 'rejected', 'expired'))` — 'revoked' is absent.
**How to avoid:** Add migration 033 to extend the constraint: `ALTER TABLE share_requests DROP CONSTRAINT share_requests_status_check; ALTER TABLE share_requests ADD CONSTRAINT share_requests_status_check CHECK (status IN ('pending', 'accepted', 'rejected', 'expired', 'revoked'));`
**Warning signs:** Test fails with `ERROR: new row for relation "share_requests" violates check constraint`.

### Pitfall 2: UpdateStatus Repo Method Rejects 'revoked'
**What goes wrong:** `repository/share_repo.go:UpdateStatus` has its own `validStatuses` map (line 148) that only allows `accepted`, `rejected`, `expired`. Calling it with `'revoked'` returns an error before touching the DB.
**Why it happens:** The repo method validates before querying. A new `RevokeShare` repo method should bypass this validation map and run the full two-table transaction directly.
**How to avoid:** Do NOT reuse `UpdateStatus` for revocation. Write a dedicated `RevokeShare(ctx, shareID)` method that runs the full transaction internally.
**Warning signs:** Handler returns 500 with "invalid status: revoked" even before hitting DB.

### Pitfall 3: channel_id Stores share_id (Verify Before Querying)
**What goes wrong:** Developer writes `WHERE overlay_id = $1 AND platform = 'shared_overlay'` (targeting one specific overlay) instead of targeting by share_id.
**Why it happens:** overlay_chat_sources.channel_id stores the share_id for shared_overlay rows (from Phase 16). Both the sender's and recipient's overlays have a row with channel_id = share_id. A single `WHERE channel_id = share_id AND platform = 'shared_overlay'` deactivates all rows on both sides in one statement — no need to know overlay_ids.
**How to avoid:** Use `UPDATE overlay_chat_sources SET is_active=false WHERE channel_id=$1 AND platform='shared_overlay'` where $1 is the share_id.
**Warning signs:** Only one user's overlay goes inactive (missed the other side).

### Pitfall 4: History Tab Filter Misses 'revoked' Cards
**What goes wrong:** Revoked cards disappear from the dashboard entirely after revocation.
**Why it happens:** `dashboard/shares/page.tsx:84` filters History as `['accepted', 'rejected', 'expired']` — 'revoked' is not included.
**How to avoid:** Add 'revoked' to the History filter array.
**Warning signs:** After optimistic update, card vanishes from UI instead of showing Revoked badge.

### Pitfall 5: Frontend Type Missing 'revoked' in ChatSource Status
**What goes wrong:** TypeScript compile error or runtime display issue when trying to show `StatusBadge` with share_status from the source.
**Why it happens:** The `ChatSource` type in `frontend/src/lib/types/overlay.ts` may not have `share_status` or `is_active` fields — these need to be added.
**How to avoid:** Extend `ChatSource` interface with `is_active: boolean` and `share_status?: 'accepted' | 'revoked' | 'expired'` (optional, only for shared_overlay sources).

### Pitfall 6: Revoke Button Appears on Non-Accepted Cards
**What goes wrong:** Revoke button shows on already-revoked or expired cards, leading to confusing double-revoke attempts.
**Why it happens:** Conditional rendering logic checks `platform === 'shared_overlay'` but forgets to also check `is_active`.
**How to avoid:** Gate revoke button on `platform === 'shared_overlay' && source.is_active` in overlay editor, and `status === 'accepted'` in dashboard card.

### Pitfall 7: NotifyUser Returns 404 When User Has No Open WS
**What goes wrong:** `notifyShareRevoked` logs "WebSocket notification failed" with 404.
**Why it happens:** `NotifyUser` returns 404 if `GetConnectionsByUser` returns empty slice — user B has no overlay open in OBS or browser.
**How to avoid:** Treat this as expected behavior — fire-and-forget means notification is best-effort. Log at Info level, not Error. Do NOT retry.
**Warning signs:** Spurious error alerts in monitoring. Fix by downgrading log level on 404 response from notify endpoint.

## Code Examples

### Revoke Transaction SQL
```sql
-- UPDATE 1: mark share revoked (in transaction)
UPDATE share_requests
SET status = 'revoked', responded_at = NOW()
WHERE id = $1
RETURNING sender_user_id, recipient_user_id;

-- UPDATE 2: deactivate all overlay sources on both sides (same transaction)
-- channel_id stores share_id for shared_overlay rows (set in Phase 16)
UPDATE overlay_chat_sources
SET is_active = false
WHERE channel_id = $1 AND platform = 'shared_overlay';
```

### Migration 033 — Add 'revoked' to CHECK Constraint
```sql
-- Migration 033: Add 'revoked' status to share_requests
-- PostgreSQL does not support adding values to CHECK constraints directly;
-- drop and recreate.
ALTER TABLE share_requests
    DROP CONSTRAINT IF EXISTS share_requests_status_check;

ALTER TABLE share_requests
    ADD CONSTRAINT share_requests_status_check
    CHECK (status IN ('pending', 'accepted', 'rejected', 'expired', 'revoked'));
```

### RevokeShare Repository Method Signature
```go
// In repository/share_repo.go
// RevokeShare atomically revokes a share and deactivates overlay sources.
// Returns the share's sender_user_id and recipient_user_id for notification routing.
func (r *ShareRepository) RevokeShare(ctx context.Context, db *pgxpool.Pool, shareID string) (senderUserID, recipientUserID string, err error)
```

Note: The handler already has `h.db` (pgxpool.Pool) for transactions. The repo method can accept the pool or the handler can manage the transaction inline (as AcceptShareRequest does). Given that AcceptShareRequest manages its own transaction in the handler, revoke should follow the same pattern for consistency.

### Endpoint Registration (share-service cmd/main.go)
```go
// No premium middleware — revocation should always be allowed
api.POST("/shares/:id/revoke", shareHandler.RevokeShareRequest)
```

### API Gateway Route Registration
```go
// In services/api-gateway/cmd/main.go protectedAPI group
protectedAPI.POST("/shares/:id/revoke", proxyHandler.ForwardRequest)
```

### Frontend sharesApi Extension
```typescript
// Add to frontend/src/lib/api/shares.ts
async revokeShare(shareId: string): Promise<void> {
  await apiClient.post(`/api/v1/shares/${shareId}/revoke`, {});
},
```

### RevocationConfirmModal Structure
```typescript
// frontend/src/app/dashboard/shares/components/RevocationConfirmModal.tsx
export function RevocationConfirmModal({ partnerName, shareId, onClose, onRevoked }) {
  const [loading, setLoading] = useState(false);
  const handleRevoke = async () => {
    setLoading(true);
    try {
      await sharesApi.revokeShare(shareId);
      toast.success('Share revoked');
      onRevoked();
      onClose();
    } catch (err) {
      toast.error('Failed to revoke share');
    } finally { setLoading(false); }
  };
  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h2 className="text-xl font-semibold mb-4">Revoke share with {partnerName}?</h2>
        <p className="text-gray-600 mb-6">This will stop message delivery immediately.</p>
        <div className="flex gap-3">
          <button onClick={onClose} disabled={loading} className="flex-1 ...">Cancel</button>
          <button onClick={handleRevoke} disabled={loading} className="flex-1 bg-red-500 ...">
            {loading ? 'Revoking...' : 'Revoke'}
          </button>
        </div>
      </div>
    </div>
  );
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate service HTTP call to update sources | Direct DB access in same transaction | Phase 18 design decision | Eliminates partial-state risk, simpler code |
| Status-only badge on cards | Badge + inactive source row in overlay editor | Phase 18 | Users can see revoked sources contextually in both places |

**What already exists (no changes needed):**
- `StatusBadge`: Already handles 'revoked' (red ✗) — zero changes needed
- `ShareRequest` TypeScript type: Already has `'revoked'` in the status union
- `GetConnectionsByUser`: Already implemented in WS manager
- `NotifyUser` internal endpoint: Fully functional since Phase 15
- `overlay_chat_sources.is_active`: Column exists since migration 001

**What needs to be added:**
- Migration 033: 'revoked' in CHECK constraint
- `StatusRevoked = "revoked"` constant in models/share_request.go
- `RevokeShareRequest` handler method
- `notifyShareRevoked` notification function
- `RevocationConfirmModal` frontend component
- Revoke button in ShareRequestCard (accepted cards only)
- Revoke button in overlay editor (active shared_overlay rows only)
- Inactive shared_overlay row rendering in overlay editor
- `is_active` and `share_status` fields on `ChatSource` type
- History tab filter inclusion of 'revoked'
- `sharesApi.revokeShare()` API function
- API gateway route: `POST /shares/:id/revoke`

## Open Questions

1. **share_status in ChatSource — overlay-manager JOIN or separate fetch?**
   - What we know: overlay_chat_sources.channel_id = share_id for shared_overlay rows; overlay-manager serves the sources endpoint
   - What's unclear: Whether overlay-manager should JOIN share_requests to add share_status, or frontend fetches share status separately
   - Recommendation: Add JOIN in overlay-manager's GetSources query — returns `share_status` only when platform='shared_overlay'. Frontend can read it from the source object directly. This avoids a second network round-trip.

2. **Real-time greying in overlay editor — WS connection needed?**
   - What we know: The overlay editor has no WebSocket today; NotifyUser sends to all open overlay WS connections for the user
   - What's unclear: Whether the user B will have an overlay WS open when they're on the overlay editor page
   - Recommendation: For MVP, skip adding a WS connection to the overlay editor. The toast arrives via OBS/overlay page if open. The editor re-fetches on next load. The revocation is immediate at DB level regardless.

3. **Endpoint: DELETE /api/v1/shares/:id vs POST /api/v1/shares/:id/revoke**
   - Recommendation: Use `POST /shares/:id/revoke` — consistent with `/accept`, `/reject`, `/mark-seen` action verbs in the same router group. DELETE implies resource deletion.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `testify/assert` + `testify/require` |
| Config file | none — `go test ./...` per service |
| Quick run command | `cd services/share-service && go test ./handlers/... -v -run TestRevoke` |
| Full suite command | `cd services/share-service && go test ./... && cd ../../frontend && npm test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SHARE-06 | Caller must be sender or recipient (403 otherwise) | unit | `go test ./handlers/... -run TestRevokeShareRequest_AuthCheck` | ❌ Wave 0 |
| SHARE-06 | Revoke only works on accepted shares (409 if revoked/pending/expired) | unit | `go test ./handlers/... -run TestRevokeShareRequest_StatusCheck` | ❌ Wave 0 |
| SHARE-06 | Transaction atomicity (share + sources update together) | integration (manual-only for MVP — no DB in unit tests) | manual | n/a |
| SHARE-07 | share_requests.status = 'revoked' after API call | unit (mock repo) | `go test ./handlers/... -run TestRevokeShareRequest_StatusUpdate` | ❌ Wave 0 |
| SHARE-07 | overlay_chat_sources.is_active = false after revoke | unit (mock repo) | `go test ./handlers/... -run TestRevokeShareRequest_SourceDeactivation` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/share-service && go test ./handlers/... -v`
- **Per wave merge:** `cd services/share-service && go test ./... && cd frontend && npm test -- --testPathPattern=shares --passWithNoTests`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/share-service/handlers/shares_revoke_test.go` — covers SHARE-06 auth check + status check + mock repo assertions

*(Frontend tests follow the TDD pattern established in Phase 15-02 — RevocationConfirmModal test file to be added in Wave 0)*

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/share-service/handlers/shares.go` — AcceptShareRequest transaction pattern, notifyShareAccepted pattern
- Direct code inspection: `services/share-service/repository/share_repo.go` — UpdateStatus validStatuses map (pitfall identified)
- Direct code inspection: `services/api-gateway/handlers/websocket.go:297` — NotifyUser implementation
- Direct code inspection: `services/api-gateway/websocket/manager.go:393` — GetConnectionsByUser
- Direct code inspection: `migrations/030_share_requests.sql` — CHECK constraint missing 'revoked'
- Direct code inspection: `migrations/001_initial_schema.sql:80` — is_active column confirmed on overlay_chat_sources
- Direct code inspection: `frontend/src/app/dashboard/shares/components/StatusBadge.tsx` — 'revoked' already handled
- Direct code inspection: `frontend/src/lib/types/share.ts` — 'revoked' in ShareRequest status union
- Direct code inspection: `services/share-service/cmd/main.go` — router groups, no premium on revoke path confirmed
- Direct code inspection: `services/api-gateway/cmd/main.go:449-452` — internal/ws/notify route

### Secondary (MEDIUM confidence)
- CONTEXT.md decisions — confirmed against codebase patterns during research

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, verified in go.mod and package.json
- Architecture: HIGH — transaction and notification patterns directly read from existing code
- Pitfalls: HIGH — pitfalls verified against actual code (UpdateStatus validStatuses map, CHECK constraint, History filter, channel_id semantics all confirmed by reading source)

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (stable codebase, no fast-moving dependencies)
