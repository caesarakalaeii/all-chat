# Phase 17: Message Routing - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend message routing in message-processor so that when a platform message routes to a source overlay, it also fans out to any recipient overlays that have that source overlay configured as a `shared_overlay` source. Display settings isolation (SOURCE-05) is handled naturally — publishing to the recipient's own pub/sub channel means the frontend applies the recipient overlay's CSS and event filters automatically.

This phase does NOT cover revocation/expiry handling (Phase 18) or source deactivation on revoke (also Phase 18).

</domain>

<decisions>
## Implementation Decisions

### Fan-out mechanism
- Extend `FindOverlaysForMessage` in `services/message-processor/router/overlay_router.go` with a SQL UNION
- First SELECT: existing direct platform source lookup (unchanged)
- Second SELECT: shared overlay fan-out — finds recipient overlays that have `ocs.platform = 'shared_overlay'` AND `ocs.channel_id = sender_overlay_id` with an inline JOIN against `share_requests WHERE status = 'accepted'`
- Single DB call, all routing in one place; fits existing `Router.Route()` signature returning `[]models.OverlayTarget`
- No Redis relay, no separate subscriber goroutine — SQL UNION is the approach

### Share status validation
- Inline JOIN in the UNION branch: `JOIN share_requests sr ON sr.sender_overlay_id = ocs.channel_id AND sr.status = 'accepted'`
- Only `'accepted'` shares route messages — revoked/expired shares stop routing immediately when their status changes (no propagation lag)
- Fan-out direction: `sender_overlay_id` identifies the source overlay; recipient overlays are those with `ocs.channel_id = sr.sender_overlay_id`

### is_active for shared_overlay sources
- Set `is_active = TRUE` immediately on source creation when `platform = 'shared_overlay'`
- Modify the `HandleAddSource` handler in `services/overlay-manager/handlers/sources.go` to special-case `shared_overlay` platform (no listener will ever set it active)
- The routing UNION branch still filters by `ocs.is_active = TRUE` (consistent with the other branch)

### Revocation cut-off scope
- Phase 17 scope: message routing only
- The routing SQL's JOIN on `status = 'accepted'` already stops messages immediately when a share is revoked/expired
- Setting `is_active = false` on `overlay_chat_sources` rows when a share is revoked/expired is deferred to Phase 18
- Phase 18 will own the full revocation logic including source deactivation

### Claude's Discretion
- Exact SQL query structure for the UNION (aliases, index hints if needed)
- Test approach for the extended router (unit test with mock DB or integration test)
- Whether `FindOverlaysForMessage` stays as one method or splits into two (direct + shared) called from `Route()`

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- **overlay_router.go** (`services/message-processor/router/overlay_router.go`): `FindOverlaysForMessage` is the sole routing query — extend with UNION here
- **pubsub_publisher.go** (`services/message-processor/publisher/pubsub_publisher.go`): `PublishToMultiple` already handles batched publish to multiple overlay IDs — no change needed
- **HandleAddSource** (`services/overlay-manager/handlers/sources.go`): Already has `if req.Platform == "shared_overlay"` branch at line ~294 — add `is_active = true` override in this branch
- **OverlayTarget model** (`services/message-processor/models/message.go`): `{OverlayID, UserID}` struct — returned by extended router, no schema change needed

### Established Patterns
- **Per-overlay deduplication** (Phase 15-03): `IsDuplicateForOverlay` already handles the same raw message going to multiple overlays — no change needed for fan-out
- **SQL UNION pattern**: Not yet used in this codebase but idiomatic PostgreSQL; fits the existing pgx/v5 query pattern
- **is_active boolean**: Platform listeners set this via source-manager activation; shared_overlay is the only platform that sets it at creation time

### Integration Points
- **overlay_router.go UNION**: The extended query needs `JOIN share_requests sr ON sr.sender_overlay_id = ocs.channel_id` — `share_requests` is in the same PostgreSQL database (no cross-service call)
- **overlay-manager HandleAddSource**: The `shared_overlay` branch already exists; add one line to set `source.IsActive = true` before persisting
- **No new services**: All changes are within message-processor (router) and overlay-manager (source creation)

</code_context>

<specifics>
## Specific Ideas

- The UNION SQL should look roughly like:
  ```sql
  -- Direct platform sources (existing)
  SELECT DISTINCT o.id, o.user_id
  FROM overlays o
  JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
  WHERE o.is_active = true AND ocs.is_active = true
    AND ocs.platform = $1 AND ocs.channel_id = $2
  UNION
  -- Shared overlay fan-out (new)
  SELECT DISTINCT o.id, o.user_id
  FROM overlays o
  JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id AND ocs.platform = 'shared_overlay' AND ocs.is_active = true
  JOIN share_requests sr ON sr.sender_overlay_id = ocs.channel_id AND sr.status = 'accepted'
  WHERE o.is_active = true
    AND sr.sender_overlay_id IN (
      SELECT DISTINCT id FROM overlays o2
      JOIN overlay_chat_sources ocs2 ON o2.id = ocs2.overlay_id
      WHERE o2.is_active = true AND ocs2.is_active = true
        AND ocs2.platform = $1 AND ocs2.channel_id = $2
    )
  ```
  (Claude should optimize/simplify the exact SQL — this is directional, not prescriptive)

</specifics>

<deferred>
## Deferred Ideas

- Setting `is_active = false` on `overlay_chat_sources` rows when share is revoked/expired — Phase 18
- Bidirectional fan-out (recipient overlay also shares back) — the current model uses sender_overlay_id; reverse routing deferred until bidirectional sharing is fully modeled
- Performance optimization with Redis caching of active share relationships — not needed at current scale

</deferred>

---

*Phase: 17-message-routing*
*Context gathered: 2026-03-10*
