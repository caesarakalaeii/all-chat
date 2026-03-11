# Phase 16: Shared Overlay Sources - Research

**Researched:** 2026-03-10
**Domain:** Overlay source management — extending existing `overlay_chat_sources` schema and UI for a new `shared_overlay` platform type
**Confidence:** HIGH

---

## Summary

Phase 16 connects the share-acceptance plumbing from Phase 15 to the overlay source configuration system. The core work is small and well-bounded: a new `shared_overlay` platform type needs to be recognized in three places — the database `supported_platforms` table, the `overlay-manager` source model/handler, and the frontend overlay editor.

The `overlay_chat_sources` table already exists and is general-purpose. Adding `shared_overlay` as a new platform requires: (1) a migration inserting it into `supported_platforms`, (2) a new backend endpoint (or extension of the existing `POST /overlays/:id/sources` handler) that validates the `shared_overlay_id` against the `share_requests` table, and (3) frontend changes in two places — the AddSourceModal stub in `shares/` (which already has the TODO comment pointing at Phase 16) and the overlay editor's "Add Source" panel.

The "browse available shared overlays" requirement (SOURCE-02) means the frontend needs a way to list which shared overlays the user is entitled to add. This data lives in `share_requests` (accepted rows where `recipient_user_id = user`). The share-service already queries this; it just needs a new endpoint that returns the list in a shape the overlay editor can consume.

**Primary recommendation:** Add `shared_overlay` as a new platform in `supported_platforms`, extend the source creation endpoint to validate share access, add a `GET /api/v1/shares/accepted` list endpoint, wire everything through api-gateway, and update the two frontend entry points (AddSourceModal stub + overlay editor).

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SOURCE-01 | New "Shared Overlays" source type available alongside platform sources | Requires `shared_overlay` in `supported_platforms` migration + model validation update |
| SOURCE-02 | User can browse list of available shared overlays when adding source | Requires `GET /api/v1/shares/accepted` endpoint listing share_requests with status=accepted where current user is recipient; share-service repository already has the querying pattern |
| SOURCE-03 | User can add shared overlay as source to any overlay via configuration UI | Requires completing the AddSourceModal TODO stub (Phase 15 deferred this), updating `POST /overlays/:id/sources` to accept `platform=shared_overlay` + `channel_id=<share_request_id or overlay_id>`, and validating the caller has an accepted share |
</phase_requirements>

---

## Standard Stack

### Core (no new libraries needed)

| Component | Version | Purpose | Pattern Source |
|-----------|---------|---------|----------------|
| Go / Gin | 1.23 / existing | New handler logic in share-service and overlay-manager | Consistent with all other services |
| pgx/v5 | existing | DB queries for share validation | Matches all repository code |
| Next.js / React | 14+ / 18+ | Frontend modal and overlay editor changes | Existing `overlaysApi`, `sharesApi` patterns |
| Tailwind CSS | existing | Styling the new "Shared Overlays" section in the overlay editor | Matches existing `getPlatformColor` / icon patterns |

No new packages required for this phase.

**Installation:** none

---

## Architecture Patterns

### How the Existing Source System Works

```
POST /api/v1/overlays/:id/sources
  → api-gateway (proxy, JWT auth)
  → overlay-manager HandleAddSource
      validates user owns overlay
      validates platform in validPlatforms map
      persists to overlay_chat_sources
      RETURNS ChatSource JSON
```

The `validPlatforms` map in `services/overlay-manager/models/chat_source.go` is the hard gate:

```go
// services/overlay-manager/models/chat_source.go  (lines 24-29)
var validPlatforms = map[string]bool{
    "twitch":  true,
    "youtube": true,
    "kick":    true,
    "tiktok":  true,
}
```

`overlay_chat_sources.platform` has a FK constraint to `supported_platforms(platform)`, so both the map AND the migration row must be added together.

### Pattern 1: Adding a New Platform Type

**What:** Insert into `supported_platforms`, add to `validPlatforms` map, extend source handler to allow the new type, adjust Validate() to not require `channel_name` for `shared_overlay` (the overlay name can be fetched from the overlays table).

**When to use:** Every new source type follows this path.

```go
// Migration (services pattern)
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth)
VALUES ('shared_overlay', 'Shared Overlay', TRUE, FALSE);

// Model update
var validPlatforms = map[string]bool{
    "twitch":         true,
    "youtube":        true,
    "kick":           true,
    "tiktok":         true,
    "shared_overlay": true,   // NEW
}
```

### Pattern 2: Validate Share Access Before Creating Source

**What:** The `POST /overlays/:id/sources` handler (or a new dedicated handler) must verify that an accepted share relationship exists between the requesting user and the overlay being referenced as `channel_id` / `shared_overlay_id`.

**When to use:** Any time a `shared_overlay` source is being added.

The simplest approach: add a branch in `HandleAddSource` for `platform == "shared_overlay"`. Query `share_requests` to confirm:
- A row exists with `status = 'accepted'`
- The `sender_overlay_id` = the requested shared overlay id
- The `recipient_user_id` = the requesting user (they are adding someone else's overlay as a source)

OR the user is the sender and `recipient_overlay_id` was stored (Note: the current schema does NOT store `recipient_overlay_id` in `share_requests` — see Critical Finding #1 below).

```go
// Validation pseudocode in HandleAddSource (overlay-manager)
if req.Platform == "shared_overlay" {
    // Verify the calling user has an accepted share that grants access to req.ChannelID (the sender_overlay_id)
    var count int
    err := db.QueryRow(ctx,
        `SELECT COUNT(*) FROM share_requests
         WHERE sender_overlay_id = $1
         AND recipient_user_id = $2
         AND status = 'accepted'`,
        req.ChannelID, userID,
    ).Scan(&count)
    if count == 0 {
        c.JSON(403, gin.H{"error": "no accepted share for this overlay"})
        return
    }
    // Use overlay name as channel_name (fetch from overlays table)
}
```

### Pattern 3: List Available Shared Overlays (SOURCE-02)

**What:** New endpoint `GET /api/v1/shares/accepted` in share-service returns all accepted shares where current user is the recipient. The frontend "Add Source" panel calls this to populate the "Shared Overlays" section.

**Response shape:**
```json
{
  "shares": [
    {
      "share_id": "uuid",
      "sender_overlay_id": "uuid",
      "sender_display_name": "xqc",
      "share_status": "accepted"
    }
  ]
}
```

This is a small addition to `share_repo.go` — a query that selects accepted shares and joins the overlays table for the overlay name.

### Pattern 4: Frontend "Add Shared Overlay" Flow

**Two entry points both need the real API call:**

1. **AddSourceModal** (`frontend/src/app/dashboard/shares/components/AddSourceModal.tsx`) — the Phase 15 stub has a TODO comment at lines 55-66 that must be replaced with an actual `overlaysApi.addSource()` call.

2. **Overlay editor** (`frontend/src/app/overlays/[id]/page.tsx`) — needs a new "Shared Overlays" section alongside Twitch/YouTube/Kick/TikTok. This section fetches from `GET /api/v1/shares/accepted` and presents a list to pick from.

```typescript
// Replace the Phase 15 TODO stub in AddSourceModal.tsx
await overlaysApi.addSource(selectedOverlay, {
  platform: 'shared_overlay',
  channel_id: senderOverlayId,  // sender_overlay_id from the share request
});
```

### Recommended Project Structure (changes only)

```
migrations/
  032_shared_overlay_platform.sql   # Insert into supported_platforms
  032_shared_overlay_platform_down.sql

services/overlay-manager/
  models/chat_source.go             # Add 'shared_overlay' to validPlatforms
  handlers/sources.go               # Branch in HandleAddSource for shared_overlay validation

services/share-service/
  handlers/shares.go                # New GetAcceptedShares handler
  repository/share_repo.go          # New GetAcceptedSharesByCurrentUser method
  cmd/main.go                       # Wire new route

services/api-gateway/
  cmd/main.go                       # Add GET /api/v1/shares/accepted route

frontend/src/
  lib/types/overlay.ts              # Add 'shared_overlay' to ChatSource.platform union
  lib/types/share.ts                # Add SharedOverlaySource type if needed
  lib/api/shares.ts                 # Add getAcceptedShares() method
  app/dashboard/shares/components/AddSourceModal.tsx   # Replace TODO with real API call
  app/overlays/[id]/page.tsx        # Add "Shared Overlays" section to Add Source panel
```

### Anti-Patterns to Avoid

- **Bypassing the `validPlatforms` check without also adding the migration row:** The FK constraint on `overlay_chat_sources.platform` will reject inserts if `supported_platforms` doesn't have the row. Both changes must land in the same wave.
- **Storing the overlay name as `channel_id`:** Use `channel_id = sender_overlay_id` (a UUID) and populate `channel_name` with the overlay's display name. This keeps the data model consistent with how Twitch/YouTube store IDs vs names.
- **Skipping the share validation in the source handler:** Without checking that the user actually has an accepted share, any user could add any overlay as a source by guessing a UUID.
- **Adding a completely new endpoint for shared overlay sources:** The existing `POST /overlays/:id/sources` endpoint already handles source creation generically. Extend it rather than creating a parallel route.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Share access validation | Custom auth middleware | SQL query in source handler against share_requests | Access is data-driven, not role-based — a SQL check is the right tool |
| Available overlays list | In-memory cache | Direct DB query via new repo method | Low cardinality data (a user has O(10) accepted shares at most) |
| Source type registry | Dynamic plugin system | Add to `validPlatforms` map and `supported_platforms` table | The pattern is established; follow it exactly |

---

## Critical Findings

### Finding 1: `recipient_overlay_id` Is NOT Stored in `share_requests` (HIGH confidence)

The `share_requests` table (migration 030) does not have a `recipient_overlay_id` column. The accept endpoint receives `recipient_overlay_id` in the request body but currently only uses it for the WebSocket notification payload — it is never persisted to the database.

**Impact on Phase 16:** When the sender wants to add the recipient's overlay as a source (the other direction of the bidirectional flow), there is nowhere to look up "which overlay did the recipient share back with me?" The current data model only records the sender's overlay (`sender_overlay_id`).

**Recommendation:** Phase 16 scope (SOURCE-01, SOURCE-02, SOURCE-03) only requires the recipient to add the sender's overlay. The sender adding the recipient's overlay back is SHARE-05 territory (already marked complete in the requirements traceability, which seems to be the AddSourceModal prompt). However, if the AddSourceModal prompt for the sender is supposed to result in an actual source addition, we need to know which overlay the recipient shared back.

**Resolution options:**
1. Add `recipient_overlay_id` column to `share_requests` in migration 032, populate it in `AcceptShareRequest` handler, and the sender's AddSourceModal can use it.
2. Phase 16 only implements the recipient adding the sender's overlay (one direction), and the sender's direction is deferred to Phase 17 or addressed separately.

Given the AddSourceModal TODO in Phase 15 has `senderOverlayId` passed as a prop (from the sender's perspective: this is the overlay that was just shared WITH you by the recipient), and the acceptance flow also prompts both parties, **the simplest path for Phase 16 is**:
- Implement the recipient adding the sender's overlay (primary flow, requires no schema change)
- Implement the sender adding the recipient's overlay by storing `recipient_overlay_id` in the acceptance (requires migration 032 to add the column + update AcceptShareRequest handler)

### Finding 2: The `channel_handle` Field Is Optional — Shared Overlays Don't Need It (HIGH confidence)

`overlay_chat_sources.channel_handle` is nullable. The source model uses `ChannelHandle *string`. For `shared_overlay` sources, this field should be nil — no special handling needed.

### Finding 3: api-gateway Does Not Currently Route `/api/v1/shares/*` (HIGH confidence)

Looking at `services/api-gateway/cmd/main.go`, the protected API section does NOT include any `/api/v1/shares/*` routes. The share-service routes are missing from the gateway entirely.

**The routes in share-service's main.go that exist but have no gateway counterpart:**
- `GET /api/v1/users/search`
- `GET /api/v1/shares/incoming`
- `POST /api/v1/shares`
- `POST /api/v1/shares/:id/accept`
- `POST /api/v1/shares/:id/reject`
- `GET /api/v1/shares/unseen-acceptances`
- `POST /api/v1/shares/:id/mark-seen`
- `POST /api/v1/admin/users/:id/premium`

The frontend `sharesApi` calls these endpoints (e.g., `/api/v1/shares/incoming`, `/api/v1/shares/:id/accept`) but they would fail at the gateway unless the gateway is forwarding them. This is likely a Phase 14 oversight that was somehow working (perhaps the frontend dev server proxies directly to the share-service, or the gateway has wildcard routing not shown).

**For Phase 16:** The new `GET /api/v1/shares/accepted` endpoint must be added to BOTH the share-service AND the api-gateway routing. Phase 16 research recommends auditing all share-service routes and adding the missing gateway registrations as a Wave 0 task.

### Finding 4: `supported_platforms.is_enabled` Feature Flag (HIGH confidence)

The `supported_platforms` table has `is_enabled BOOLEAN DEFAULT FALSE`. When adding `shared_overlay`, it must be inserted with `is_enabled = TRUE`, otherwise the platform is disabled by the feature flag (matching Twitch which is `TRUE`, and the initial insert pattern in migration 001).

### Finding 5: `ChatSource.platform` Type Union in Frontend Must Be Extended (HIGH confidence)

`frontend/src/lib/types/overlay.ts` line 58:
```typescript
platform: 'twitch' | 'youtube' | 'kick' | 'tiktok';
```

This union must gain `'shared_overlay'`. It appears in both `ChatSource` and `AddSourceRequest`. Both must be updated together, or TypeScript will reject the new source type at compile time.

---

## Common Pitfalls

### Pitfall 1: Missing api-gateway Route Registration
**What goes wrong:** New `GET /api/v1/shares/accepted` endpoint in share-service is unreachable from the frontend because api-gateway has no routing entry for it.
**Why it happens:** Share-service routes are not in the gateway's service registry (see Finding 3). Each new share-service endpoint must be manually registered in api-gateway `cmd/main.go`.
**How to avoid:** Add the route in api-gateway alongside the share-service handler work. Check the existing share routes are also registered (they appear to be missing — this should be fixed in Wave 0).
**Warning signs:** 404 responses from the gateway when calling share endpoints; the share-service health check passes but API calls fail.

### Pitfall 2: FK Violation on `shared_overlay` Insert Without Migration
**What goes wrong:** `INSERT INTO overlay_chat_sources ... platform = 'shared_overlay'` fails with FK violation against `supported_platforms`.
**Why it happens:** `overlay_chat_sources.platform` has `REFERENCES supported_platforms(platform)`. If the migration hasn't run, the FK constraint rejects the insert.
**How to avoid:** Migration 032 must run before any source with `platform = 'shared_overlay'` is created. Wave 0 must include migration 032.
**Warning signs:** `ERROR: insert or update on table "overlay_chat_sources" violates foreign key constraint "overlay_chat_sources_platform_fkey"`

### Pitfall 3: Share Validation Bypassed Because overlay-manager Doesn't Know About share_requests
**What goes wrong:** A user adds an arbitrary overlay UUID as a `shared_overlay` source without having an accepted share.
**Why it happens:** overlay-manager doesn't query share_requests (different service). If validation is skipped, there is no enforcement.
**How to avoid:** The validation query must be in the source handler (overlay-manager has a pgxpool and can directly query `share_requests` since they share the same Postgres database). This is the same database cross-query pattern already used in `HandleAddSource` for overlay ownership (`SELECT user_id FROM overlays WHERE id = $1`).
**Warning signs:** Sources with `platform = 'shared_overlay'` appearing in overlays for users who have no accepted shares.

### Pitfall 4: TypeScript Union Mismatch Causing Silent Failures
**What goes wrong:** Frontend passes `platform: 'shared_overlay'` to the API but TypeScript rejects it at compile time (or at runtime if type assertions are used).
**Why it happens:** `AddSourceRequest.platform` is typed as a union of only the four existing platforms.
**How to avoid:** Update both `ChatSource.platform` and `AddSourceRequest.platform` unions before implementing the UI.
**Warning signs:** TypeScript error `Type '"shared_overlay"' is not assignable to type '"twitch" | "youtube" | "kick" | "tiktok"'`

### Pitfall 5: The `channel_name` Field for `shared_overlay` Sources
**What goes wrong:** Source displays blank name in the overlay editor because `channel_name` was not populated.
**Why it happens:** For platform sources, `channel_name` comes from the OAuth flow or user input. For `shared_overlay`, there is no user input — the name must be fetched from the `overlays` table using the `sender_overlay_id`.
**How to avoid:** In the source handler, when `platform == 'shared_overlay'`, query `SELECT name FROM overlays WHERE id = $1` and set it as `channel_name` before persisting. Alternatively, the frontend can pass `channel_name` in the request body (populated from the shares list response).

---

## Code Examples

### Backend: shared_overlay Branch in HandleAddSource
```go
// services/overlay-manager/handlers/sources.go
// In HandleAddSource, after validating overlay ownership:

if req.Platform == "shared_overlay" {
    // Validate the user has an accepted share granting access to this overlay
    var shareCount int
    err := h.db.QueryRow(ctx,
        `SELECT COUNT(*) FROM share_requests
         WHERE sender_overlay_id = $1
         AND recipient_user_id = $2
         AND status = 'accepted'`,
        req.ChannelID, userID.(string),
    ).Scan(&shareCount)
    if err != nil || shareCount == 0 {
        c.JSON(http.StatusForbidden, gin.H{
            "error": "no accepted share relationship for this overlay",
        })
        return
    }

    // Fetch the overlay name for channel_name
    var overlayName string
    err = h.db.QueryRow(ctx,
        `SELECT name FROM overlays WHERE id = $1`, req.ChannelID,
    ).Scan(&overlayName)
    if err != nil {
        overlayName = req.ChannelName // Fallback to provided name
    }
    req.ChannelName = overlayName
}
```

### Backend: GetAcceptedShares Endpoint in share-service
```go
// GET /api/v1/shares/accepted
// Returns accepted shares where current user is recipient (they can add these as sources)
func (h *ShareHandler) GetAcceptedShares(c *gin.Context) {
    userID := c.GetString("user_id")

    shares, err := h.shareRepo.GetAcceptedSharesByRecipient(c.Request.Context(), userID)
    if err != nil {
        h.logger.Error("Failed to get accepted shares", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"shares": shares})
}
// Note: GetAcceptedSharesByRecipient already exists in repository (added for cycle detection in Phase 15)
```

### Frontend: Replace AddSourceModal TODO with Real Call
```typescript
// frontend/src/app/dashboard/shares/components/AddSourceModal.tsx
// Replace lines 55-66 (the console.log + TODO block):

await overlaysApi.addSource(selectedOverlay, {
  platform: 'shared_overlay',
  channel_id: senderOverlayId,
  channel_name: `${senderName}'s overlay`,
});
```

### Frontend: sharesApi.getAcceptedShares
```typescript
// frontend/src/lib/api/shares.ts
async getAcceptedShares(): Promise<ShareRequest[]> {
    const response = await apiClient.get<{ shares: ShareRequest[] }>(
        '/api/v1/shares/accepted'
    );
    return response.shares || [];
},
```

### Migration 032
```sql
-- migrations/032_shared_overlay_platform.sql
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth)
VALUES ('shared_overlay', 'Shared Overlay', TRUE, FALSE)
ON CONFLICT (platform) DO NOTHING;
```

---

## State of the Art

| Old State (End of Phase 15) | Phase 16 State | Impact |
|-----------------------------|----------------|--------|
| AddSourceModal logs to console (TODO stub) | AddSourceModal calls `overlaysApi.addSource` with `platform: 'shared_overlay'` | SOURCE-03 fulfilled |
| `validPlatforms` = 4 platforms | `validPlatforms` = 5 platforms including `shared_overlay` | SOURCE-01 fulfilled |
| No endpoint to browse available shared overlays | `GET /api/v1/shares/accepted` returns accepted shares | SOURCE-02 fulfilled |
| Overlay editor has no "Shared Overlays" section | Overlay editor shows "Shared Overlays" alongside platform sources | SOURCE-01 + SOURCE-03 fulfilled |
| `recipient_overlay_id` not persisted in `share_requests` | Added as nullable column in migration 032 | Enables bidirectional add-source (sender flow) |

---

## Open Questions

1. **Should `recipient_overlay_id` be added to `share_requests` in Phase 16 or deferred?**
   - What we know: The sender's AddSourceModal prompts the sender to add the recipient's overlay. Without `recipient_overlay_id` stored, there is no way to know which overlay the recipient shared back.
   - What's unclear: Is the sender's add-source flow strictly in Phase 16's scope or is it implicit from SHARE-05 (marked complete but the API call was deferred)?
   - Recommendation: Add `recipient_overlay_id` as a nullable column in migration 032, update `AcceptShareRequest` to store it, and implement the sender's add-source flow as part of Phase 16. This completes SHARE-05 properly.

2. **Are the missing api-gateway routes for share-service a bug that must be fixed in Phase 16 Wave 0?**
   - What we know: api-gateway `cmd/main.go` has NO routes under `/api/v1/shares/*` or `/api/v1/users/search`. These endpoints are called by the frontend (as seen in `shares.ts` and `sharesApi`).
   - What's unclear: Is this intentional (dev proxy bypasses gateway) or an oversight?
   - Recommendation: Treat as an oversight. Add all missing share-service routes to api-gateway in Wave 0. This makes the system consistent and prevents prod deployment failures.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go: standard `testing` + `testify` (backend); vitest 1.x (frontend) |
| Config file | `frontend/vitest.config.ts` (created in Phase 15) |
| Quick run command | `cd frontend && npm test -- --run src/app/dashboard/shares/` |
| Full suite command | `cd frontend && npm test -- --run src/` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SOURCE-01 | `shared_overlay` accepted as valid platform in source model | unit | `go test ./services/overlay-manager/models/...` | ✅ (`chat_source_test.go` exists) |
| SOURCE-02 | `GET /api/v1/shares/accepted` returns accepted shares for user | unit | `go test ./services/share-service/handlers/...` | ❌ Wave 0 |
| SOURCE-03 | `POST /overlays/:id/sources` with `platform=shared_overlay` validates share access | unit | `go test ./services/overlay-manager/handlers/...` | ❌ Wave 0 |
| SOURCE-03 | AddSourceModal calls `overlaysApi.addSource` when Add clicked | unit | `cd frontend && npm test -- --run src/app/dashboard/shares/components/AddSourceModal.test.tsx` | ✅ (test file exists, needs new test case) |

### Sampling Rate
- **Per task commit:** `go test ./services/overlay-manager/models/... && go test ./services/share-service/handlers/...`
- **Per wave merge:** `cd frontend && npm test -- --run src/ && go test ./services/...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/overlay-manager/handlers/sources_shared_overlay_test.go` — covers SOURCE-03 (shared overlay validation in HandleAddSource)
- [ ] `services/share-service/handlers/shares_accepted_test.go` — covers SOURCE-02 (GetAcceptedShares handler)
- [ ] `AddSourceModal.test.tsx` — needs new test case: "calls overlaysApi.addSource when Add clicked" (currently only tests that onAdded and onClose are called — the actual API call is not tested since Phase 15 deferred it)

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/overlay-manager/models/chat_source.go` — validPlatforms map
- Direct code inspection: `services/overlay-manager/repository/source_repo.go` — Create/ListByOverlayID
- Direct code inspection: `services/overlay-manager/handlers/sources.go` — HandleAddSource full logic
- Direct code inspection: `migrations/001_initial_schema.sql` — overlay_chat_sources schema and FK constraint
- Direct code inspection: `migrations/030_share_requests.sql` — share_requests schema
- Direct code inspection: `services/api-gateway/cmd/main.go` — full route table (confirmed missing share routes)
- Direct code inspection: `services/share-service/repository/share_repo.go` — GetAcceptedSharesByRecipient already exists
- Direct code inspection: `frontend/src/app/dashboard/shares/components/AddSourceModal.tsx` — Phase 15 TODO stub at lines 55-66
- Direct code inspection: `frontend/src/app/overlays/[id]/page.tsx` — existing "Add Source" panel structure
- Direct code inspection: `frontend/src/lib/types/overlay.ts` — ChatSource platform union

### Secondary (MEDIUM confidence)
- Phase 15 summaries (15-01, 15-02, 15-03) — confirmed what was built and what was deferred

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all existing libraries, no new dependencies
- Architecture: HIGH — patterns are established and code-verified
- Critical findings: HIGH — all verified by direct code inspection
- Pitfalls: HIGH — derived from actual code constraints (FK, type unions, missing routes)

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (stable domain)
