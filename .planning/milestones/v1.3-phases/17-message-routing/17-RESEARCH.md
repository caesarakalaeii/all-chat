# Phase 17: Message Routing - Research

**Researched:** 2026-03-10
**Domain:** Go message-processor routing extension + overlay-manager source activation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Fan-out mechanism**: Extend `FindOverlaysForMessage` in `services/message-processor/router/overlay_router.go` with a SQL UNION. First SELECT is the existing direct platform source lookup (unchanged). Second SELECT is the shared overlay fan-out — finds recipient overlays that have `ocs.platform = 'shared_overlay'` AND `ocs.channel_id = sender_overlay_id` with an inline JOIN against `share_requests WHERE status = 'accepted'`. Single DB call, all routing in one place; fits existing `Router.Route()` signature returning `[]models.OverlayTarget`. No Redis relay, no separate subscriber goroutine.
- **Share status validation**: Inline JOIN in the UNION branch: `JOIN share_requests sr ON sr.sender_overlay_id = ocs.channel_id AND sr.status = 'accepted'`. Only `'accepted'` shares route messages. Fan-out direction: `sender_overlay_id` identifies the source overlay; recipient overlays are those with `ocs.channel_id = sr.sender_overlay_id`.
- **is_active for shared_overlay sources**: Set `is_active = TRUE` immediately on source creation when `platform = 'shared_overlay'`. Modify `HandleAddSource` in `services/overlay-manager/handlers/sources.go` to special-case `shared_overlay` platform (no listener will ever set it active). The routing UNION branch still filters by `ocs.is_active = TRUE`.
- **Revocation cut-off scope**: Phase 17 scope is message routing only. The routing SQL's JOIN on `status = 'accepted'` already stops messages immediately when a share is revoked/expired. Setting `is_active = false` on `overlay_chat_sources` rows when a share is revoked/expired is deferred to Phase 18.

### Claude's Discretion
- Exact SQL query structure for the UNION (aliases, index hints if needed)
- Test approach for the extended router (unit test with mock DB or integration test)
- Whether `FindOverlaysForMessage` stays as one method or splits into two (direct + shared) called from `Route()`

### Deferred Ideas (OUT OF SCOPE)
- Setting `is_active = false` on `overlay_chat_sources` rows when share is revoked/expired — Phase 18
- Bidirectional fan-out (recipient overlay also shares back) — deferred until bidirectional sharing is fully modeled
- Performance optimization with Redis caching of active share relationships — not needed at current scale
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SOURCE-04 | Messages from source overlay's aggregated chat delivered to recipient's overlay | SQL UNION in `FindOverlaysForMessage` fans out platform messages to recipient overlays via `share_requests`; existing `PublishToMultiple` already handles delivery to multiple overlay IDs |
| SOURCE-05 | Display settings (CSS, events) from recipient's overlay apply, not source overlay's | Satisfied automatically: publishing to the recipient's own `overlay:{id}` pub/sub channel means the API Gateway and frontend apply the recipient overlay's settings; no message content changes needed |
</phase_requirements>

---

## Summary

Phase 17 extends message routing in `message-processor` so that a platform message reaching a source (sender) overlay also fans out to every recipient overlay that has added that source overlay as a `shared_overlay` source with an active `share_requests` row. The change is entirely contained in two locations: the router SQL query and the source creation handler.

The routing change is a SQL UNION appended to the existing `FindOverlaysForMessage` query. The UNION branch joins `overlay_chat_sources` (filtered to `platform = 'shared_overlay'` and `is_active = true`) against `share_requests` (filtered to `status = 'accepted'`) to resolve which recipient overlays should receive the message. The returned `[]models.OverlayTarget` list already drives the per-overlay publish loop in `cmd/main.go`; no changes to the publish, deduplication, enrichment, or normalizer layers are required.

The `is_active` fix in `HandleAddSource` is a one-line change: after the existing share-validation block for `platform == "shared_overlay"`, set `source.IsActive = true` before calling `sourceRepo.Create`. Without this, the UNION's `ocs.is_active = true` filter would silently exclude the new source and messages would not route.

**Primary recommendation:** Implement as two coordinated changes — SQL UNION in `overlay_router.go` and `is_active = true` override in `sources.go` — with a unit test for the router using a mock/stub and a manual smoke test via mock-messages endpoint.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `jackc/pgx/v5` | v5.8.0 | PostgreSQL queries in message-processor | Already used in `Repository.FindOverlaysForMessage` |
| `stretchr/testify` | v1.11.1 | Test assertions | Used in all existing message-processor and overlay-manager tests |
| `alicebob/miniredis/v2` | v2.37.0 | In-process Redis for unit tests | Already in message-processor `go.mod`; used in dedup and registry tests |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testcontainers/testcontainers-go` | v0.40.0 | Real PostgreSQL for integration tests | overlay-manager uses it for repo tests; NOT available in message-processor module |

**Installation:** No new dependencies required for either service.

---

## Architecture Patterns

### Existing Router Structure
```
services/message-processor/router/
└── overlay_router.go    # Repository + Router types; FindOverlaysForMessage is the only query
```

The `Repository` struct holds a `*pgxpool.Pool`. `FindOverlaysForMessage(ctx, platform, channelID)` returns `[]models.OverlayTarget`. `Router.Route()` is a thin wrapper that adds a debug log. The message handler in `cmd/main.go` calls `overlayRouter.Route()` and iterates the returned slice.

### Pattern 1: SQL UNION for Fan-out
**What:** Append a second SELECT to `FindOverlaysForMessage` using SQL UNION (deduplication built-in). The second branch joins across `overlay_chat_sources` and `share_requests` to find recipient overlays.
**When to use:** Single DB round-trip; fits existing query/scan pattern; UNION semantics prevent duplicate overlay IDs if the same overlay appears in both branches.
**Example (directional — Claude optimizes exact form):**
```sql
-- Direct platform sources (existing, unchanged)
SELECT DISTINCT o.id, o.user_id
FROM overlays o
JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
WHERE o.is_active = true
  AND ocs.is_active = true
  AND ocs.platform = $1
  AND ocs.channel_id = $2

UNION

-- Shared overlay fan-out (new)
SELECT DISTINCT o.id, o.user_id
FROM overlays o
JOIN overlay_chat_sources ocs
    ON o.id = ocs.overlay_id
    AND ocs.platform = 'shared_overlay'
    AND ocs.is_active = true
JOIN share_requests sr
    ON sr.sender_overlay_id = ocs.channel_id
    AND sr.status = 'accepted'
WHERE o.is_active = true
  AND sr.sender_overlay_id IN (
      SELECT o2.id
      FROM overlays o2
      JOIN overlay_chat_sources ocs2 ON o2.id = ocs2.overlay_id
      WHERE o2.is_active = true
        AND ocs2.is_active = true
        AND ocs2.platform = $1
        AND ocs2.channel_id = $2
  )
```
The subquery in the UNION branch re-resolves which overlays are the direct recipients of the platform message; this drives the sender_overlay_id lookup. Claude may simplify with a JOIN or CTE depending on query plan.

### Pattern 2: is_active Override at Creation
**What:** In `HandleAddSource`, after the existing share-validation block for `platform == "shared_overlay"` (lines ~294-325), set `source.IsActive = true` before calling `sourceRepo.Create`. All other platforms leave `IsActive = false` and rely on listeners.
**When to use:** `shared_overlay` is the only platform where no external listener activates the source. The pattern is consistent with `HandleAddSourceAuto` which sets `IsActive: true` directly.
**Example:**
```go
// In HandleAddSource, inside the shared_overlay block, after share validation:
source := &models.ChatSource{
    // ...existing fields...
    IsActive: req.Platform == "shared_overlay", // true for shared_overlay, false for others
}
```
Or equivalently, keep the `IsActive: false` default and add an explicit override after the `shared_overlay` validation block:
```go
if req.Platform == "shared_overlay" {
    source.IsActive = true
}
```

### Per-overlay Processing Loop (No Changes Needed)
The `cmd/main.go` message handler already iterates `for _, overlay := range overlays` — every `OverlayTarget` from the extended router goes through the full normalize → enrich → dedup → publish pipeline independently. The existing per-overlay deduplication (`IsDuplicateForOverlay`) handles the case where the same raw message would be delivered to both the direct recipient and a shared recipient (if any overlap existed). No changes to this loop are needed.

### Anti-Patterns to Avoid
- **Filtering `shared_overlay` from the platform lookup**: The UNION approach handles this naturally. Do NOT add `AND ocs.platform != 'shared_overlay'` to the first branch — the first branch already only matches on `ocs.platform = $1` where `$1` is the actual platform (e.g., `twitch`), so `shared_overlay` rows are never matched by the first branch.
- **Calling `FindOverlaysForMessage` twice**: One combined UNION query is the locked decision. Avoid splitting into two repository calls from `Route()`.
- **Setting `is_active` via a separate UPDATE after Create**: The `HandleAddSource` handler must set `IsActive = true` on the struct before the `sourceRepo.Create` call. A post-create UPDATE is unnecessary and introduces a race window.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Message fan-out to multiple overlays | Custom Redis pub/sub relay or goroutine per share | SQL UNION in existing router | `PublishToMultiple` already handles batched publish; routing is already the DB's job |
| Duplicate detection across fan-out overlays | Custom message ID tracking for shared messages | Existing `IsDuplicateForOverlay` | Per-overlay Redis dedup keys already isolate overlays; Phase 15-03 proved this works |
| Share status caching | Redis cache for active shares | Direct DB JOIN in routing query | Query hits indexed columns; caching deferred by design |

---

## Common Pitfalls

### Pitfall 1: shared_overlay sources left with is_active = false
**What goes wrong:** The UNION branch filters `ocs.is_active = true`. If `HandleAddSource` does not override `IsActive = true` for `shared_overlay`, the source is created but never matches the router query. Messages silently do not fan out.
**Why it happens:** The default `IsActive: false` exists because all other platforms wait for a listener to activate the source. `shared_overlay` has no listener.
**How to avoid:** Add `source.IsActive = true` override in the `shared_overlay` branch of `HandleAddSource`, after the share validation block completes.
**Warning signs:** Source appears in overlay source list, user sees it as added, but no messages arrive. No errors logged.

### Pitfall 2: UNION query returns duplicate overlay IDs
**What goes wrong:** If a recipient overlay has the sender overlay added as `shared_overlay` AND the recipient overlay also happens to have the same platform/channel as a direct source, the overlay appears in both UNION branches. The `for _, overlay := range overlays` loop processes it twice.
**Why it happens:** SQL `UNION` deduplicates identical rows, but only by exact column value match. If the recipient overlay's `user_id` differs between branches (shouldn't happen for the same overlay), dedup fails.
**How to avoid:** Use `UNION` (not `UNION ALL`) — this is the default and deduplicates on `(o.id, o.user_id)`. The existing per-overlay dedup (`IsDuplicateForOverlay`) also catches any double-publish within the 5-second window.
**Warning signs:** Same message appears twice in recipient overlay frontend.

### Pitfall 3: Revoked share still routes messages in Phase 17
**What goes wrong:** Phase 18 sets `is_active = false` on `overlay_chat_sources` rows when shares are revoked. Phase 17's UNION already handles revocation correctly via `sr.status = 'accepted'` — once a share is revoked (status changes), the UNION branch returns no rows for that recipient. No additional work needed in Phase 17.
**Why it matters:** Understanding this prevents over-engineering. The `is_active` flag on `overlay_chat_sources` is a redundant guard that Phase 18 adds; the status check is the primary cut-off.
**Warning signs:** Would only be a problem if someone removed the `sr.status = 'accepted'` filter.

### Pitfall 4: SQL parameter index conflict in UNION
**What goes wrong:** The existing query uses `$1` (platform) and `$2` (channelID). The new UNION branch subquery also needs these same parameters. PostgreSQL parameterized queries in pgx reuse parameter indices — `$1` and `$2` refer to the same bound values throughout the entire query string, including UNION branches and subqueries.
**How to avoid:** Use `$1` and `$2` in the UNION subquery as well. Do NOT introduce new parameter indices for the same values.

---

## Code Examples

Verified patterns from existing codebase:

### Current FindOverlaysForMessage (baseline)
```go
// Source: services/message-processor/router/overlay_router.go
func (r *Repository) FindOverlaysForMessage(ctx context.Context, platform, channelID string) ([]models.OverlayTarget, error) {
    query := `
        SELECT DISTINCT o.id, o.user_id
        FROM overlays o
        JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
        WHERE o.is_active = true
          AND ocs.platform = $1
          AND ocs.channel_id = $2
    `
    rows, err := r.db.Query(ctx, query, platform, channelID)
    // ... scan loop ...
}
```

### is_active Override Pattern (from HandleAddSourceAuto)
```go
// Source: services/overlay-manager/handlers/sources.go (HandleAddSourceAuto)
source := &models.ChatSource{
    // ...
    IsActive: true,  // Set directly at creation for auto-added sources
}
```
The `HandleAddSource` equivalent sets `IsActive: false` by default and relies on listeners. The `shared_overlay` branch must override this to `true`.

### pgx UNION scan (existing scan pattern applies unchanged)
```go
// Source: services/message-processor/router/overlay_router.go — scan loop pattern
for rows.Next() {
    var target models.OverlayTarget
    if err := rows.Scan(&target.OverlayID, &target.UserID); err != nil {
        return nil, fmt.Errorf("failed to scan overlay row: %w", err)
    }
    overlays = append(overlays, target)
}
```
UNION queries in pgx return a flat result set; the same scan loop works unchanged.

### Existing share_requests schema (verified from migration 030)
```sql
-- share_requests columns relevant to routing
sender_overlay_id UUID NOT NULL REFERENCES overlays(id)
recipient_user_id UUID NOT NULL REFERENCES users(id)
status VARCHAR(20) CHECK (status IN ('pending', 'accepted', 'rejected', 'expired'))
```
The UNION branch joins on `sr.sender_overlay_id = ocs.channel_id` — `ocs.channel_id` stores the sender overlay's UUID when `ocs.platform = 'shared_overlay'` (established in Phase 16).

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Global dedup (single key per message) | Per-overlay dedup (`IsDuplicateForOverlay`) | Phase 15-03 | Enables same raw message to route to multiple overlays without false positives |
| `is_active` set by listeners only | `is_active = true` at creation for `shared_overlay` | Phase 17 | No listener activation step needed for virtual platform sources |

---

## Open Questions

1. **Split vs. unified `FindOverlaysForMessage`**
   - What we know: Claude's discretion; both approaches compile and produce correct results
   - What's unclear: Whether splitting into `findDirectOverlays` + `findSharedOverlays` with a merge in `Route()` improves testability enough to justify the added complexity
   - Recommendation: Keep as one method. The SQL UNION is self-documenting and aligns with the decision to use a single DB call. Splitting adds a merge step and requires two mock-DB setups in tests.

2. **Index coverage for the UNION branch**
   - What we know: `idx_overlay_chat_sources_overlay_platform` covers `(overlay_id, platform, is_active)`. `idx_share_requests_sender` covers `(sender_user_id)` but not `(sender_overlay_id)`.
   - What's unclear: Whether a missing index on `share_requests(sender_overlay_id)` will matter at current scale (small number of shares)
   - Recommendation: No new index needed for Phase 17. Document as a Phase 18/19 performance consideration if share counts grow.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none — standard `go test ./...` |
| Quick run command | `cd services/message-processor && go test ./router/... -v` |
| Full suite command | `cd services/message-processor && go test ./... && cd ../overlay-manager && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SOURCE-04 | `FindOverlaysForMessage` returns recipient overlays for shared_overlay fan-out | unit (mock DB) | `cd services/message-processor && go test ./router/... -run TestFindOverlaysForMessage -v` | ❌ Wave 0 |
| SOURCE-04 | `FindOverlaysForMessage` returns only direct overlays when no accepted share exists | unit (mock DB) | `cd services/message-processor && go test ./router/... -run TestFindOverlaysForMessage -v` | ❌ Wave 0 |
| SOURCE-04 | `HandleAddSource` sets `is_active = true` for `shared_overlay` platform | unit (existing mock pattern) | `cd services/overlay-manager && go test ./handlers/... -run TestHandleAddSource_SharedOverlay -v` | ❌ Wave 0 (extend existing test file) |
| SOURCE-05 | Recipient overlay receives message on its own `overlay:{id}` channel | smoke/manual | `make frontend-messages` + observe in browser | N/A (manual) |

### Sampling Rate
- **Per task commit:** `cd services/message-processor && go test ./router/... -v`
- **Per wave merge:** `cd services/message-processor && go test ./... && cd ../overlay-manager && go test ./handlers/... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/message-processor/router/overlay_router_test.go` — covers SOURCE-04 (router UNION logic with table-driven cases: direct only, shared only, both, no match, revoked share excluded)
- [ ] Extend `services/overlay-manager/handlers/sources_shared_overlay_test.go` — add `TestHandleAddSource_SharedOverlay_IsActiveTrue` asserting `source.IsActive == true` after creation (currently only the 403 forbidden case is tested)

*(No new framework installs needed — testify and go test already available in both modules)*

---

## Sources

### Primary (HIGH confidence)
- Direct code read: `services/message-processor/router/overlay_router.go` — full file, baseline query
- Direct code read: `services/message-processor/cmd/main.go` — message handler loop, how Route() result is consumed
- Direct code read: `services/overlay-manager/handlers/sources.go` — HandleAddSource shared_overlay branch (lines ~292-325)
- Direct code read: `services/message-processor/publisher/pubsub_publisher.go` — PublishToMultiple confirmed unchanged
- Direct code read: `services/message-processor/models/message.go` — OverlayTarget struct confirmed
- Direct code read: `migrations/030_share_requests.sql` — share_requests schema
- Direct code read: `migrations/032_shared_overlay_platform.sql` — shared_overlay platform registration
- Direct code read: `services/overlay-manager/handlers/sources_shared_overlay_test.go` — existing test patterns

### Secondary (MEDIUM confidence)
- PostgreSQL UNION semantics (deduplicates on full row; UNION ALL does not) — standard SQL, HIGH confidence
- pgx/v5 parameter binding: `$1`, `$2` indices reused across UNION branches in same query string — verified by pgx documentation behavior

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, no new dependencies
- Architecture: HIGH — changes directly derived from reading the actual source files
- Pitfalls: HIGH — derived from reading code and schema, not speculation
- SQL UNION approach: HIGH — locked decision in CONTEXT.md; standard PostgreSQL behavior

**Research date:** 2026-03-10
**Valid until:** 2026-06-10 (stable domain — no fast-moving dependencies)
