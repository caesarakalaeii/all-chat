# Phase 14: Foundation - Context

**Gathered:** 2026-03-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can search for streamers and create share requests with premium enforcement. This phase establishes the foundation for bidirectional overlay sharing: discovery mechanism (search by platform username), share request workflow (send/view/respond), and premium feature gating (server-side enforcement with admin testing overrides).

</domain>

<decisions>
## Implementation Decisions

### User search implementation
- **Query pattern**: Case-insensitive partial match (ILIKE '%query%')
  - Leverages existing case-insensitive username index (migration 028)
  - Most flexible for discovery (finds usernames with query anywhere)
- **Platform filtering**: Platform-specific search (not cross-platform)
  - User selects platform first (Twitch/YouTube/Kick/TikTok), then searches
  - Matches mental model — streamers know which platform someone uses
- **Search columns**: Username only (not display_name)
  - Clean, predictable results using unique identifiers
- **Result limit**: 10 results maximum
  - Keeps UI manageable, encourages specific queries
  - Reasonable for friend/collaborator discovery

### Share request model & lifecycle
- **Status states**: pending / accepted / rejected / expired
  - Explicit expired state separates timeout from manual rejection
  - Clear intent for each terminal state
- **Overlay selection timing**: At request creation (sender picks upfront)
  - Recipient sees: "User X wants to share their [platform badges] with you"
  - Clear intent, no deferred decisions
- **Auto-expiry**: 7-day expiry for pending requests
  - Prevents stale requests accumulating
  - Background job checks expiry every 5 minutes (matches Phase 19 pattern)
  - Transitions pending → expired after 7 days
- **Database structure**: Single share_requests table
  - Columns: id, sender_user_id, sender_overlay_id, recipient_user_id, status, created_at, responded_at, expires_at
  - Simple, normalized, no audit history (not needed for MVP)

### Premium enforcement pattern
- **Check location**: Gin middleware for /api/v1/shares/* routes
  - Centralized enforcement before share handlers run
  - DRY approach with careful route planning (some routes may not need premium checks, e.g., viewing your own requests)
- **Premium storage**: is_premium boolean column on users table
  - Matches existing is_admin pattern (simple, familiar)
  - Admin sets via UPDATE users SET is_premium = TRUE WHERE id = '...'
- **Caching**: No caching, query database on every request
  - Premium checks are infrequent (only on share creation/acceptance)
  - Database query is fast, no stale data risk
  - Simple for MVP, can optimize later if needed
- **Admin endpoint**: POST /api/v1/admin/users/:id/premium
  - Body: {"is_premium": true/false}
  - Clean REST design, matches existing /api/v1/admin/users pattern
  - Dedicated endpoint for clarity (not generic PATCH)

### Dashboard UX for incoming requests
- **Display style**: Card view
  - Each request as a card with visual hierarchy
  - Matches existing overlay card pattern in UI
- **Card information**:
  - Requester avatar + username
  - Platform source badges (showing platform icons, source name on hover)
  - Timestamp (when request was sent)
  - Status indicator (pending/accepted/rejected/expired)
  - **Note**: Overlay name omitted (unnecessary detail)
- **Sort order**: Most recent first
  - Newest requests at top (natural expectation for notifications)
- **Status filtering**: Separate tabs (Pending / History)
  - Default view: Pending requests only (actionable items)
  - History tab: Accepted/rejected/expired requests
  - Keeps main view focused

### Claude's Discretion
- Exact SQL query structure for platform-specific search (JOIN strategy, index usage)
- Background job implementation for request expiry (cron vs service internal loop)
- Error messages for premium enforcement ("Premium feature required" vs more specific)
- Card styling details (spacing, shadows, hover effects)
- Middleware exemption logic (which share routes don't need premium checks)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- **auth-service admin handlers**: Existing pattern for admin endpoints (`/api/v1/admin/users`)
  - AdminHandler struct with repo, db, logger, jwtSecret
  - Can extend with premium management endpoint
- **users table structure**: Already has is_admin boolean column (migration 009)
  - Same pattern can be used for is_premium column
  - Case-insensitive username index exists (migration 028) for search
- **overlay-manager service**: Handles overlays and chat sources
  - Overlay model: id, user_id, name, description, is_active
  - ChatSource model: overlay_id, platform, channel_id, channel_name, is_active
  - Can query to fetch sender's overlay sources for badge display
- **Standard Go Layout**: All services follow handlers → domain → repository pattern
  - Dependency injection for testability
  - Graceful shutdown, health checks (/health/live, /health/ready)

### Established Patterns
- **Admin boolean flags**: is_admin column pattern proven (migration 009, indexed WHERE is_admin = TRUE)
  - Apply same pattern for is_premium
- **Gin middleware**: API Gateway uses middleware for auth
  - Can create similar premium enforcement middleware
- **Repository pattern**: All services use repository layer for database access
  - ShareRequestRepository interface for share CRUD operations
- **Foreign key constraints**: Project uses ON DELETE RESTRICT (not CASCADE)
  - Application-level cascade logic to prevent data loss

### Integration Points
- **New service needed**: share-service (or extend overlay-manager)
  - Handlers for: user search, create request, list requests, accept/reject
  - Repository for share_requests table CRUD
  - Admin handler for premium management
- **Database migration**: New tables needed
  - share_requests table (id, sender_user_id, sender_overlay_id, recipient_user_id, status, created_at, responded_at, expires_at)
  - Add is_premium column to users table
- **API Gateway routing**: Add routes for /api/v1/shares/* and /api/v1/admin/users/:id/premium
  - Premium middleware applied to share routes
- **Frontend dashboard**: New UI components
  - User search interface (platform selector + search input)
  - Share request cards (pending/history tabs)
  - Accept/reject action buttons

</code_context>

<specifics>
## Specific Ideas

- Platform source badges on request cards should show platform icons (Twitch, YouTube, Kick, TikTok logos)
  - Hover reveals source name (e.g., "xQc on Twitch")
  - Helps recipient quickly assess if requester is on platforms they care about
- No overlay name displayed on request cards — platform sources are more relevant for decision-making
- Middleware-based premium enforcement requires careful route design:
  - Premium required: POST /api/v1/shares (create request), POST /api/v1/shares/:id/accept
  - No premium check: GET /api/v1/shares/incoming (view your own requests)
- Background job for expiry should run every 5 minutes (matches Phase 19 time-based share expiry pattern)
  - Query: UPDATE share_requests SET status = 'expired' WHERE status = 'pending' AND expires_at < NOW()

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 14-foundation*
*Context gathered: 2026-03-09*
