# Phase 15: Share Acceptance - Research

**Researched:** 2026-03-09
**Domain:** Bidirectional share acceptance with cycle prevention, React modal forms, WebSocket notifications
**Confidence:** HIGH

## Summary

Phase 15 implements the acceptance flow for share requests, establishing bidirectional overlay access between users. The core technical challenges are: (1) React modal forms with inline validation for overlay selection and expiry options, (2) cycle detection using DFS graph traversal to prevent circular dependencies, (3) WebSocket notifications for realtime sender updates, and (4) optional message deduplication in message-processor to handle Twitch Shared Chat overlap.

**Primary recommendation:** Use existing modal patterns from Phase 14 (BanModal.tsx), implement DFS cycle detection in share-service with dual-check (frontend pre-validation + backend enforcement), leverage existing deduplicator in message-processor with overlay-specific fingerprints for Phase 15 or defer to Phase 17.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Acceptance flow & overlay selection:**
- UI pattern: Modal with form (not inline card expansion)
- Opens on Accept button click
- Contains: overlay dropdown + expiry options + platform badges + Accept/Cancel buttons
- Clean separation from dashboard, doesn't disrupt card layout
- Overlay selection: Simple dropdown showing overlay names (all user's overlays, no filtering)
- No overlay error: Block acceptance if user has no overlays with error message "Create an overlay first to accept shares"
- Platform context: Show platform badges in modal (same badges from card)

**Expiry option UX:**
- Input pattern: Radio buttons with custom duration
  - "This stream" (expires when your stream ends)
  - Custom duration → [number input] hours (1-168 range)
  - Unlimited (never expires)
- Default selection: "This stream" pre-selected
- Not live handling: Allow "This stream" even if user not currently streaming
- Custom duration validation: Inline validation (not on-submit)
  - Red border + error text if value < 1 or > 168
  - Disable Accept button until valid

**Immediate source addition:**
- Prompt timing: Second modal immediately after acceptance
- First modal closes (acceptance complete)
- Second modal opens: "Add [User]'s overlay to one of yours?"
- Dropdown: all user's overlays (no filtering)
- Bidirectional prompt: Both sender AND recipient get add-source modal
- Sender notification strategy: Realtime if online + prompt on visit if offline
  - If sender online: WebSocket notification triggers add-source modal immediately
  - If sender offline: Next dashboard visit shows add-source prompt
  - Requires tracking: "has_seen_acceptance" flag per share request

**Cycle detection behavior:**
- Detection timing: On acceptance submission (in modal, before closing)
- Error message: "Can't accept: This would create a circular share (You → [User] → [Other] → You). Messages would loop infinitely."
- Algorithm depth: Full depth graph traversal (DFS or BFS)
- Not just direct cycles (A→B→A), but also A→B→C→A
- Implementation location: Both backend and frontend
  - Frontend: Pre-check for instant feedback (better UX)
  - Backend: Authoritative enforcement (can't be bypassed)

**Message deduplication (Twitch Shared Chat overlap):**
- Problem: User A shares overlay with B + Twitch Shared Chat enabled → B sees A's Twitch messages twice
- Solution: Deduplicate in message-processor
  - Track: platform + message ID per overlay
  - Time window: 5 seconds
  - If duplicate seen within window: drop second occurrence
- Phase placement: Claude's Discretion (Phase 15 or Phase 17)

### Claude's Discretion

- Acceptance modal styling (spacing, shadows, animations)
- Cycle detection algorithm choice (DFS vs BFS)
- "has_seen_acceptance" flag storage (database column vs Redis key)
- WebSocket notification payload structure for sender
- Deduplication data structure (in-memory map vs Redis set)
- Whether to implement deduplication in Phase 15 or Phase 17

### Deferred Ideas (OUT OF SCOPE)

- Email/push notifications for share acceptance (future notification system)
- Share acceptance history log (full audit trail beyond status changes)
- "Suggest overlays" based on common platforms (ML-based recommendations)
- Bulk accept/reject for multiple pending requests (not MVP need)
- Custom cycle error messages per cycle length ("Direct loop", "3-hop loop", etc.)
- Redis-backed distributed deduplication (MVP can use in-memory per message-processor pod)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SHARE-04 | User can accept share request, choosing overlay to share back and expiry option | React modal form patterns, radio button validation, overlay dropdown from database |
| SHARE-05 | On acceptance, both users can optionally add shared source to an overlay immediately | Sequential modal pattern, WebSocket notifications for sender, has_seen_acceptance flag |
| SHARE-08 | Share status indicators show active, expired, or revoked state | Status badge component exists (Phase 14), acceptance updates status to "accepted" |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React 19.2.4 | Latest | Modal component state management | Project uses React 19, hooks for form state |
| Tailwind CSS 4.1.18 | Latest | Modal styling, validation states | Project standard for all UI styling |
| Go 1.23+ | Latest | Backend cycle detection, acceptance logic | Project backend standard |
| PostgreSQL pgx/v5 | v5.8.0 | Share request status updates, bidirectional records | Project database driver |
| Redis go-redis/v9 | Latest | WebSocket notifications, optional deduplication | Project realtime infrastructure |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| date-fns 4.1.0 | Current | Timestamp formatting for acceptance | Already in project for Phase 14 |
| react-hot-toast 2.6.0 | Current | Success/error notifications | Project standard for user feedback |
| gorilla/websocket | Latest | WebSocket notifications to sender | Already used in API Gateway |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| DFS cycle detection | BFS cycle detection | DFS simpler for directed graphs, BFS better for shortest path (not needed) |
| Database flag for has_seen_acceptance | Redis key | Database persists across restarts (better for MVP), Redis faster but ephemeral |
| Inline validation | On-submit validation | Inline provides instant feedback (user decision), better UX |

**Installation:**
```bash
# Frontend dependencies already installed (Phase 14)
# Backend dependencies already installed (share-service exists)
# No new packages required
```

## Architecture Patterns

### Recommended Project Structure
```
services/share-service/
├── handlers/            # Acceptance endpoint
│   └── accept.go        # POST /api/v1/shares/:id/accept
├── repository/          # Database operations
│   ├── shares.go        # Share request CRUD
│   └── cycles.go        # Cycle detection queries
├── cycles/              # NEW: Cycle detection logic
│   └── detector.go      # DFS graph traversal
├── models/              # Data models (exists from Phase 14)
└── cmd/main.go          # Service entry point

frontend/src/app/dashboard/shares/
├── components/
│   ├── ShareRequestCard.tsx       # Exists (Phase 14)
│   ├── AcceptModal.tsx            # NEW: Acceptance form modal
│   ├── AddSourceModal.tsx         # NEW: Add-source prompt modal
│   └── PlatformBadge.tsx          # Exists (Phase 14)
└── page.tsx                        # Dashboard page (exists)

services/message-processor/
└── dedup/                          # Exists (v1.2)
    └── dedup.go                    # Extend for overlay-specific deduplication
```

### Pattern 1: Modal Form with Inline Validation
**What:** React modal with form state, inline validation, disabled submit button
**When to use:** User input with validation before server submission
**Example:**
```typescript
// Source: Existing BanModal.tsx pattern (Phase 14)
export function AcceptModal({ request, onClose, onAccept }) {
  const [selectedOverlay, setSelectedOverlay] = useState('');
  const [expiryOption, setExpiryOption] = useState('this_stream');
  const [customHours, setCustomHours] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Inline validation for custom hours
  const isValidCustomHours = () => {
    if (expiryOption !== 'custom') return true;
    const hours = parseInt(customHours);
    return hours >= 1 && hours <= 168;
  };

  const canSubmit = selectedOverlay && isValidCustomHours() && !loading;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg p-6 max-w-md w-full">
        <h2 className="text-xl font-bold mb-4">
          Accept share from {request.sender.display_name}
        </h2>
        {/* Form fields */}
        <button
          disabled={!canSubmit}
          onClick={handleAccept}
        >
          Accept
        </button>
      </div>
    </div>
  );
}
```

### Pattern 2: Sequential Modal Flow
**What:** First modal closes, second modal opens with data from first action
**When to use:** Multi-step user flows where each step is independent
**Example:**
```typescript
// Source: Common React modal pattern
const [showAcceptModal, setShowAcceptModal] = useState(false);
const [showAddSourceModal, setShowAddSourceModal] = useState(false);
const [acceptedShare, setAcceptedShare] = useState(null);

async function handleAccept(overlayId, expiryOption) {
  const share = await sharesApi.accept(requestId, overlayId, expiryOption);
  setAcceptedShare(share);
  setShowAcceptModal(false);
  setShowAddSourceModal(true); // Immediately show second modal
}
```

### Pattern 3: DFS Cycle Detection in Directed Graph
**What:** Depth-first search with recursion stack to detect cycles in share relationships
**When to use:** Preventing circular dependencies in bidirectional relationships
**Example:**
```go
// Source: Standard DFS cycle detection pattern
// Reference: https://www.geeksforgeeks.org/dsa/detect-cycle-in-a-graph/

type CycleDetector struct {
    repo *repository.ShareRepository
}

func (d *CycleDetector) HasCycle(ctx context.Context, fromUserID, toUserID string) (bool, error) {
    visited := make(map[string]bool)
    recStack := make(map[string]bool)

    return d.dfs(ctx, fromUserID, toUserID, visited, recStack)
}

func (d *CycleDetector) dfs(ctx context.Context, current, target string, visited, recStack map[string]bool) (bool, error) {
    visited[current] = true
    recStack[current] = true

    // If we reach the target in our recursion stack, cycle exists
    if current == target && recStack[target] {
        return true, nil
    }

    // Get all users that current user shares with (accepted shares only)
    shares, err := d.repo.GetAcceptedSharesByRecipient(ctx, current)
    if err != nil {
        return false, err
    }

    for _, share := range shares {
        nextUser := share.SenderUserID

        if !visited[nextUser] {
            if hasCycle, err := d.dfs(ctx, nextUser, target, visited, recStack); err != nil {
                return false, err
            } else if hasCycle {
                return true, nil
            }
        } else if recStack[nextUser] {
            // Back edge found
            return true, nil
        }
    }

    recStack[current] = false
    return false, nil
}
```

### Pattern 4: WebSocket Notification with Modal Trigger
**What:** Server sends WebSocket event to trigger client-side modal
**When to use:** Realtime notifications requiring user action
**Example:**
```typescript
// Source: Common WebSocket notification pattern
// Reference: https://blog.logrocket.com/websocket-tutorial-socket-io/

useEffect(() => {
  const ws = new WebSocket('ws://localhost:8080/ws/notifications');

  ws.onmessage = (event) => {
    const notification = JSON.parse(event.data);

    if (notification.type === 'share_accepted') {
      // Trigger add-source modal
      setAcceptedShareData({
        shareId: notification.share_id,
        senderName: notification.sender_name,
        overlayId: notification.sender_overlay_id,
      });
      setShowAddSourceModal(true);
    }
  };

  return () => ws.close();
}, []);
```

### Pattern 5: Message Deduplication with Time Window
**What:** Redis-based duplicate detection using message fingerprint + TTL
**When to use:** Preventing duplicate message delivery from overlapping sources
**Example:**
```go
// Source: Existing message-processor/dedup/dedup.go (extended)

func (d *Deduplicator) IsDuplicateForOverlay(ctx context.Context, overlayID, platform, channelID, userID, text string, timestamp time.Time) (bool, error) {
    // Create fingerprint including overlay ID for overlay-specific deduplication
    fingerprint := d.createFingerprintWithOverlay(overlayID, platform, channelID, userID, text, timestamp)

    key := fmt.Sprintf("%s:%s", dedupPrefix, fingerprint)
    wasSet, err := d.client.SetNX(ctx, key, "1", 5*time.Second).Result()
    if err != nil {
        return false, err // Fail open
    }

    return !wasSet, nil // If wasSet is false, key existed = duplicate
}

func (d *Deduplicator) createFingerprintWithOverlay(overlayID, platform, channelID, userID, text string, timestamp time.Time) string {
    truncatedTime := timestamp.Truncate(time.Second).Unix()
    message := fmt.Sprintf("%s|%s|%s|%s|%s|%d", overlayID, platform, channelID, userID, text, truncatedTime)
    hash := sha256.Sum256([]byte(message))
    return hex.EncodeToString(hash[:])
}
```

### Anti-Patterns to Avoid

- **Client-only cycle detection:** Frontend checks are for UX only, backend MUST enforce (security bypass risk)
- **Synchronous cycle detection in HTTP path:** Cycle detection can be expensive (O(V+E)), use with timeout, fail safe if timeout
- **Global deduplication without overlay scope:** Message deduplication must be overlay-specific (User A sees message, User B shouldn't be blocked)
- **Modal state in parent component:** Each modal should manage its own open/close state, parent triggers via props
- **Forgetting to cleanup WebSocket listeners:** Always return cleanup function in useEffect to prevent memory leaks

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Graph cycle detection | Custom adjacency list traversal | Standard DFS with visited + recursion stack | Edge cases: self-loops, disconnected components, infinite recursion without proper base case |
| Message deduplication | Custom in-memory map with manual expiry | Existing dedup.Deduplicator with Redis TTL | Handles race conditions, automatic expiry, distributed-safe with Redis |
| WebSocket connection management | Raw WebSocket with manual reconnection | Existing API Gateway WebSocket Manager | Handles connection pooling, auto-reconnection, graceful shutdown |
| Form validation library | useForm hook library (react-hook-form) | Simple useState with inline validation | Phase 15 has minimal validation (overlay dropdown + number range), useState is simpler for 2-3 fields |
| Modal component library | Headless UI, Radix UI | Tailwind-styled custom modal | Project pattern (BanModal.tsx), no additional dependencies, full control over styling |

**Key insight:** Graph algorithms (cycle detection) have well-established patterns that are easy to get wrong (infinite recursion, missed edge cases). Message deduplication requires distributed coordination (Redis atomic operations). Use proven patterns and existing infrastructure.

## Common Pitfalls

### Pitfall 1: Cycle Detection Race Condition
**What goes wrong:** Two users simultaneously accept shares with each other (A accepts B's request, B accepts A's request), both cycle checks pass, both acceptances succeed, cycle created
**Why it happens:** Database transactions don't lock across multiple queries, cycle check and acceptance are separate operations
**How to avoid:** Use database transaction with SELECT FOR UPDATE on share_requests table during cycle detection + acceptance, or use optimistic locking with version column
**Warning signs:** Race condition tests (concurrent acceptance) fail, cycles appear in production after simultaneous acceptances

### Pitfall 2: Frontend Cycle Detection Out of Sync
**What goes wrong:** Frontend cache/state is stale, shows "no cycle" pre-check, backend rejects with "cycle detected", user sees confusing error
**Why it happens:** Frontend doesn't refetch active shares before cycle check, or another user accepts during modal open
**How to avoid:** Always refetch active shares immediately before opening acceptance modal, use short cache TTL (< 10 seconds), backend is source of truth
**Warning signs:** Users report "modal said OK but server rejected", error rate on /accept endpoint higher than expected

### Pitfall 3: WebSocket Notification Lost (Sender Offline)
**What goes wrong:** Sender offline when recipient accepts, notification never delivered, sender never sees add-source prompt
**Why it happens:** WebSocket notifications are ephemeral (not persisted), no fallback mechanism
**How to avoid:** Use has_seen_acceptance flag in database, check on dashboard page load, show add-source prompt if flag is false
**Warning signs:** Users report "I got a share accepted but never saw the add-source option", asymmetric experience between online/offline senders

### Pitfall 4: Deduplication False Positives (Legitimate Duplicate Messages)
**What goes wrong:** User legitimately sends same message twice (e.g., "gg" twice in a row), second message incorrectly dropped as duplicate
**Why it happens:** Deduplication fingerprint includes message text + user + timestamp (truncated to second), identical messages within same second flagged as duplicates
**How to avoid:** Include platform-specific message ID in fingerprint (Twitch IRC message ID, YouTube liveChatId), not just text content
**Warning signs:** Users report "my messages randomly disappear", higher than expected deduplication rate (> 1% of messages)

### Pitfall 5: Modal Z-Index Conflicts
**What goes wrong:** Add-source modal appears behind acceptance modal or dashboard elements, unclickable
**Why it happens:** Sequential modals don't increment z-index, CSS specificity conflicts
**How to avoid:** Use distinct z-index values (acceptance modal z-50, add-source modal z-60), or ensure first modal fully unmounts before second renders
**Warning signs:** Visual testing reveals modal overlap, users can't click add-source buttons

### Pitfall 6: Custom Duration Input Without Min/Max Attributes
**What goes wrong:** User enters 0, negative number, or 10000 hours, client-side validation catches but input allows typing
**Why it happens:** HTML input type="number" allows any number without min/max attributes
**How to avoid:** Set min="1" max="168" attributes on input element, enforce in both HTML and React validation
**Warning signs:** Users can type invalid values (even if submission blocked), poor UX with delayed validation feedback

## Code Examples

Verified patterns from existing codebase:

### Acceptance Endpoint (share-service)
```go
// Source: Standard Go service pattern (services/auth-service/handlers/)
// Location: services/share-service/handlers/accept.go

type AcceptShareRequest struct {
    RecipientOverlayID string `json:"recipient_overlay_id" binding:"required"`
    ExpiryOption       string `json:"expiry_option" binding:"required"` // "this_stream", "custom", "unlimited"
    ExpiryHours        *int   `json:"expiry_hours,omitempty"`           // Required if expiry_option = "custom"
}

func (h *ShareHandler) AcceptShare(c *gin.Context) {
    shareID := c.Param("id")
    userID := c.GetString("user_id") // From JWT middleware

    var req AcceptShareRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Validate expiry option
    if req.ExpiryOption == "custom" {
        if req.ExpiryHours == nil || *req.ExpiryHours < 1 || *req.ExpiryHours > 168 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "expiry_hours must be between 1 and 168"})
            return
        }
    }

    ctx := c.Request.Context()

    // Start transaction for atomic acceptance
    tx, err := h.db.Begin(ctx)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
        return
    }
    defer tx.Rollback(ctx)

    // Get share request with row lock
    share, err := h.repo.GetShareRequestForUpdate(ctx, tx, shareID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "share request not found"})
        return
    }

    // Verify recipient owns this request
    if share.RecipientUserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "not your share request"})
        return
    }

    // Check if already responded
    if share.Status != models.StatusPending {
        c.JSON(http.StatusConflict, gin.H{"error": "share request already responded to"})
        return
    }

    // Cycle detection (with transaction context to see pending changes)
    hasCycle, err := h.cycleDetector.HasCycle(ctx, tx, share.SenderUserID, share.RecipientUserID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for cycles"})
        return
    }
    if hasCycle {
        c.JSON(http.StatusConflict, gin.H{
            "error": "Can't accept: This would create a circular share (You → User → Other → You). Messages would loop infinitely.",
        })
        return
    }

    // Update share request status
    now := time.Now()
    share.Status = models.StatusAccepted
    share.RespondedAt = &now

    if err := h.repo.UpdateShareRequest(ctx, tx, share); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update share request"})
        return
    }

    // Create bidirectional share record
    // (Implementation in Phase 16 - creates share relationship in both directions)

    // Commit transaction
    if err := tx.Commit(ctx); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit transaction"})
        return
    }

    // Send WebSocket notification to sender (fire-and-forget)
    go h.notifyShareAccepted(share.SenderUserID, shareID, req.RecipientOverlayID)

    c.JSON(http.StatusOK, gin.H{
        "share": share,
        "sender_overlay_id": share.SenderOverlayID, // For add-source prompt
    })
}
```

### Frontend API Client Method
```typescript
// Source: Existing frontend/src/lib/api/shares.ts pattern
// Location: frontend/src/lib/api/shares.ts

export const sharesApi = {
  // ... existing methods (Phase 14)

  /**
   * Accept a share request
   */
  async acceptRequest(
    shareId: string,
    recipientOverlayId: string,
    expiryOption: 'this_stream' | 'custom' | 'unlimited',
    expiryHours?: number
  ): Promise<{ share: ShareRequest; sender_overlay_id: string }> {
    return apiClient.post(`/api/v1/shares/${shareId}/accept`, {
      recipient_overlay_id: recipientOverlayId,
      expiry_option: expiryOption,
      expiry_hours: expiryHours,
    });
  },

  /**
   * Check if accepting a share would create a cycle
   */
  async checkCycle(shareId: string): Promise<{ has_cycle: boolean; message?: string }> {
    return apiClient.get(`/api/v1/shares/${shareId}/check-cycle`);
  },

  /**
   * Reject a share request
   */
  async rejectRequest(shareId: string): Promise<void> {
    await apiClient.post(`/api/v1/shares/${shareId}/reject`);
  },
};
```

### AcceptModal Component
```typescript
// Source: Existing BanModal.tsx pattern
// Location: frontend/src/app/dashboard/shares/components/AcceptModal.tsx

'use client';

import { useState, useEffect } from 'react';
import { ShareRequest } from '@/lib/types/share';
import { overlaysApi } from '@/lib/api/overlays';
import { sharesApi } from '@/lib/api/shares';
import { PlatformBadge } from './PlatformBadge';
import toast from 'react-hot-toast';

interface AcceptModalProps {
  request: ShareRequest;
  onClose: () => void;
  onAccepted: (senderOverlayId: string) => void;
}

export function AcceptModal({ request, onClose, onAccepted }: AcceptModalProps) {
  const [overlays, setOverlays] = useState([]);
  const [selectedOverlay, setSelectedOverlay] = useState('');
  const [expiryOption, setExpiryOption] = useState('this_stream');
  const [customHours, setCustomHours] = useState('24');
  const [loading, setLoading] = useState(false);
  const [loadingOverlays, setLoadingOverlays] = useState(true);

  useEffect(() => {
    fetchOverlays();
  }, []);

  async function fetchOverlays() {
    try {
      const data = await overlaysApi.fetchAll();
      setOverlays(data);
      if (data.length > 0) {
        setSelectedOverlay(data[0].id); // Auto-select first overlay
      }
    } catch (error) {
      toast.error('Failed to load overlays');
    } finally {
      setLoadingOverlays(false);
    }
  }

  const isValidCustomHours = () => {
    if (expiryOption !== 'custom') return true;
    const hours = parseInt(customHours);
    return !isNaN(hours) && hours >= 1 && hours <= 168;
  };

  const canSubmit = selectedOverlay && isValidCustomHours() && !loading;

  async function handleAccept() {
    if (!canSubmit) return;

    setLoading(true);
    try {
      const hours = expiryOption === 'custom' ? parseInt(customHours) : undefined;
      const response = await sharesApi.acceptRequest(
        request.id,
        selectedOverlay,
        expiryOption,
        hours
      );

      toast.success(`Share accepted from ${request.sender.display_name}!`);
      onAccepted(response.sender_overlay_id);
      onClose();
    } catch (error: any) {
      if (error.response?.data?.error?.includes('circular share')) {
        toast.error('Cannot accept: This would create a circular share dependency');
      } else {
        toast.error(error.response?.data?.error || 'Failed to accept share');
      }
    } finally {
      setLoading(false);
    }
  }

  if (loadingOverlays) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
        <div className="bg-white rounded-lg p-6">
          <p>Loading overlays...</p>
        </div>
      </div>
    );
  }

  if (overlays.length === 0) {
    return (
      <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
        <div className="bg-white rounded-lg p-6 max-w-md">
          <h2 className="text-xl font-bold mb-4">No Overlays Found</h2>
          <p className="text-gray-600 mb-4">
            Create an overlay first to accept shares
          </p>
          <button
            onClick={onClose}
            className="w-full px-4 py-2 bg-gray-200 rounded hover:bg-gray-300"
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg p-6 max-w-md w-full" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-xl font-bold mb-4">
          {request.sender.display_name} wants to share with you
        </h2>

        {/* Platform badges */}
        {request.overlay_sources && (
          <div className="flex gap-2 mb-4 flex-wrap">
            {request.overlay_sources.map((source, idx) => (
              <PlatformBadge key={idx} source={source} />
            ))}
          </div>
        )}

        {/* Overlay selection */}
        <div className="mb-4">
          <label className="block text-sm font-medium mb-2">
            Share back which overlay? *
          </label>
          <select
            value={selectedOverlay}
            onChange={(e) => setSelectedOverlay(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {overlays.map(overlay => (
              <option key={overlay.id} value={overlay.id}>
                {overlay.name}
              </option>
            ))}
          </select>
        </div>

        {/* Expiry option */}
        <div className="mb-4">
          <label className="block text-sm font-medium mb-2">
            Expiry option *
          </label>
          <div className="space-y-2">
            <label className="flex items-center">
              <input
                type="radio"
                name="expiry"
                value="this_stream"
                checked={expiryOption === 'this_stream'}
                onChange={(e) => setExpiryOption(e.target.value)}
                className="mr-2"
              />
              This stream (expires when your stream ends)
            </label>
            <label className="flex items-center">
              <input
                type="radio"
                name="expiry"
                value="custom"
                checked={expiryOption === 'custom'}
                onChange={(e) => setExpiryOption(e.target.value)}
                className="mr-2"
              />
              <span className="mr-2">Custom duration</span>
              <input
                type="number"
                value={customHours}
                onChange={(e) => setCustomHours(e.target.value)}
                disabled={expiryOption !== 'custom'}
                min="1"
                max="168"
                placeholder="e.g., 24"
                className={`px-2 py-1 border rounded w-20 ${
                  expiryOption === 'custom' && !isValidCustomHours()
                    ? 'border-red-500'
                    : 'border-gray-300'
                }`}
              />
              <span className="ml-1">hours (1-168)</span>
            </label>
            {expiryOption === 'custom' && !isValidCustomHours() && (
              <p className="text-red-500 text-sm ml-6">
                Must be between 1 and 168 hours
              </p>
            )}
            <label className="flex items-center">
              <input
                type="radio"
                name="expiry"
                value="unlimited"
                checked={expiryOption === 'unlimited'}
                onChange={(e) => setExpiryOption(e.target.value)}
                className="mr-2"
              />
              Unlimited (never expires)
            </label>
          </div>
        </div>

        {/* Action buttons */}
        <div className="flex justify-end space-x-3">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded disabled:opacity-50"
            disabled={loading}
          >
            Cancel
          </button>
          <button
            onClick={handleAccept}
            disabled={!canSubmit}
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Accepting...' : 'Accept'}
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
| Modal libraries (Headless UI, Radix UI) | Custom Tailwind-styled modals | Phase 14 (2026-03) | Zero dependencies, full styling control, matches BanModal pattern |
| Form libraries (react-hook-form, Formik) | Simple useState with inline validation | Phase 14 (2026-03) | Simpler for 2-3 field forms, no library overhead |
| Global deduplication | Per-overlay deduplication | v1.2 (2026-02) | Prevents false positives across overlays, existing dedup.Deduplicator extended |
| Manual WebSocket management | WebSocket Manager (api-gateway) | v1.0 (2026-01) | Handles pooling, reconnection, graceful shutdown |

**Deprecated/outdated:**
- None - Phase 15 builds on existing Phase 14 patterns (modals, dashboard, API client)

## Open Questions

1. **Deduplication Phase Placement**
   - What we know: User decision allows Phase 15 or Phase 17, deduplication logic exists in message-processor
   - What's unclear: Performance impact of adding overlay-specific fingerprint (SHA256 hash includes overlay ID)
   - Recommendation: Implement in Phase 15 (5-line change to existing deduplicator), test with benchmark, defer optimization to Phase 17 if needed

2. **Cycle Detection Performance on Large Graphs**
   - What we know: DFS is O(V+E), shares are bidirectional (doubles edges), typical user has < 10 active shares
   - What's unclear: Worst-case performance with 100+ users in dense share network
   - Recommendation: Set 5-second timeout on cycle detection query, fail safe (allow acceptance) if timeout, add database index on share status

3. **has_seen_acceptance Flag Cleanup**
   - What we know: Flag marks if sender has seen acceptance notification
   - What's unclear: When to clear flag (after modal shown? after modal closed? never?)
   - Recommendation: Clear flag when sender opens add-source modal (onOpen), persist if they skip (can prompt again later)

## Validation Architecture

> Nyquist validation enabled in .planning/config.json (workflow.nyquist_validation not set to false)

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Frontend: Vitest 4.0.18 + Playwright 1.58.2, Backend: Go testing stdlib |
| Config file | Frontend: vitest.config.ts, Backend: *_test.go files |
| Quick run command | Frontend: `npm test`, Backend: `go test ./services/share-service/...` |
| Full suite command | Frontend: `npm test && npm run test:e2e`, Backend: `make test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SHARE-04 | User can accept share request choosing overlay and expiry | integration | `go test ./services/share-service/handlers -run TestAcceptShare` | ❌ Wave 0 |
| SHARE-04 | Cycle detection blocks acceptance if circular dependency exists | unit | `go test ./services/share-service/cycles -run TestCycleDetection` | ❌ Wave 0 |
| SHARE-04 | Inline validation for custom duration (1-168 hours) | unit | `npm test -- AcceptModal.test.tsx` | ❌ Wave 0 |
| SHARE-05 | Add-source modal appears after acceptance for recipient | e2e | `npm run test:e2e -- share-acceptance.spec.ts` | ❌ Wave 0 |
| SHARE-05 | WebSocket notification triggers add-source modal for online sender | integration | `go test ./services/api-gateway/handlers -run TestShareAcceptedNotification` | ❌ Wave 0 |
| SHARE-05 | has_seen_acceptance flag shows prompt on dashboard if sender offline | e2e | `npm run test:e2e -- share-acceptance.spec.ts -g "offline sender"` | ❌ Wave 0 |
| SHARE-08 | Status badge shows "accepted" after successful acceptance | unit | `npm test -- ShareRequestCard.test.tsx` | ✅ Exists (Phase 14) |

### Sampling Rate
- **Per task commit:** `go test ./services/share-service/... -short` (< 30s)
- **Per wave merge:** `make test` (full backend suite) + `npm test` (frontend unit tests)
- **Phase gate:** Full suite green (backend + frontend unit + e2e) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/share-service/handlers/accept_test.go` — covers SHARE-04 (acceptance endpoint, validation)
- [ ] `services/share-service/cycles/detector_test.go` — covers SHARE-04 (DFS cycle detection, edge cases)
- [ ] `frontend/src/app/dashboard/shares/components/AcceptModal.test.tsx` — covers SHARE-04 (inline validation, form state)
- [ ] `frontend/src/app/dashboard/shares/components/AddSourceModal.test.tsx` — covers SHARE-05 (modal trigger, overlay selection)
- [ ] `frontend/e2e/share-acceptance.spec.ts` — covers SHARE-05 (end-to-end acceptance flow, WebSocket notification)
- [ ] `services/message-processor/dedup/dedup_test.go` — extend existing tests for overlay-specific deduplication (optional if Phase 15)

## Sources

### Primary (HIGH confidence)
- Existing codebase patterns:
  - `frontend/src/components/admin/BanModal.tsx` - Modal form pattern with inline validation
  - `services/message-processor/dedup/dedup.go` - Deduplication with Redis TTL
  - `services/api-gateway/handlers/websocket.go` - WebSocket notification pattern
  - `migrations/030_share_requests.sql` - Share request schema from Phase 14
  - `services/share-service/models/share_request.go` - Share request model from Phase 14
- Official documentation:
  - React 19 documentation (hooks, form state management)
  - Go standard library testing documentation
  - PostgreSQL pgx/v5 transaction documentation

### Secondary (MEDIUM confidence)
- [Detect Cycle in a Directed Graph - GeeksforGeeks](https://www.geeksforgeeks.org/dsa/detect-cycle-in-a-graph/) - DFS cycle detection algorithm with recursion stack
- [Building Real-Time Notifications with React, Socket.IO & Node.js](https://medium.com/@sasindusathiska/building-real-time-notifications-with-react-socket-io-node-js-12757a032e0d) - WebSocket notification patterns
- [How to Implement Request Deduplication with Redis](https://oneuptime.com/blog/post/2026-01-21-redis-request-deduplication/view) - Redis-based deduplication with time window
- [Tailwind CSS Modal - Flowbite](https://flowbite.com/docs/components/modal/) - Modal styling patterns with Tailwind CSS

### Tertiary (LOW confidence)
- [Graphs 101: Cycle Detection in Directed Graphs using DFS](https://medium.com/@shrutipokale2016/graphs-101-cycle-detection-in-directed-graphs-using-dfs-095265e61f9f) - Conceptual overview, no Go implementation
- [PostgreSQL Bi-Directional Logical Replication](https://severalnines.com/blog/postgresql-bi-directional-logical-replication-deep-dive/) - Replication patterns (not application-level relationships)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in project (Phase 14), versions verified in package.json and go.mod
- Architecture: HIGH - Patterns verified in existing codebase (BanModal.tsx, dedup.go, websocket.go), Phase 14 established conventions
- Pitfalls: MEDIUM - Based on common patterns and logical analysis, not specific to this codebase

**Research date:** 2026-03-09
**Valid until:** 2026-04-09 (30 days for stable stack, React/Go patterns change slowly)

---

## Research Complete

**Key Findings:**
- Phase 15 builds directly on Phase 14 foundation (modal patterns, dashboard, API client, share-service)
- DFS cycle detection is standard graph algorithm with clear implementation pattern (visited + recursion stack)
- Message deduplication already exists in message-processor, can be extended with overlay-specific fingerprints
- WebSocket notifications follow existing API Gateway pattern for realtime updates
- Sequential modal flow (AcceptModal → AddSourceModal) matches user flow requirements

**Confidence Assessment:**
| Area | Level | Reason |
|------|-------|--------|
| Frontend patterns | HIGH | BanModal.tsx provides exact pattern for AcceptModal, existing dashboard page for integration |
| Backend patterns | HIGH | Standard Go service structure, existing share-service from Phase 14, database schema ready |
| Cycle detection | HIGH | Standard DFS algorithm with clear implementation, well-documented edge cases |
| WebSocket notifications | HIGH | Existing websocket.go handler shows pattern, API Gateway manages connections |
| Deduplication | MEDIUM | Existing deduplicator works, overlay-specific extension is logical but untested |

**Ready for Planning:**
Research complete. Planner has all technical context for creating PLAN.md files covering acceptance endpoint, frontend modals, cycle detection, WebSocket notifications, and optional message deduplication.
