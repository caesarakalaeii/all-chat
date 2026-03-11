# Phase 14: Foundation - Research

**Researched:** 2026-03-09
**Domain:** User search, share request management, premium feature gating, React dashboard UI
**Confidence:** HIGH

## Summary

Phase 14 establishes the foundation for bidirectional overlay sharing through three core capabilities: user discovery (case-insensitive username search by platform), share request workflow (create/view/respond), and premium feature enforcement (server-side validation with admin override). The project's existing patterns provide strong precedents: migration 028 already implements case-insensitive username indexing, migration 009 demonstrates boolean flag patterns for is_admin, and the admin handler + middleware architecture provides a proven template for premium enforcement.

Research confirms that the user's implementation decisions from CONTEXT.md align with PostgreSQL best practices (LOWER() function with functional indexes for short text fields like usernames), Go ecosystem standards (Gin middleware for cross-cutting concerns), and established project conventions (repository pattern, boolean columns for feature flags, graceful shutdown patterns).

**Primary recommendation:** Leverage existing patterns extensively. The project already has 80% of the infrastructure needed—case-insensitive search pattern proven, admin boolean pattern proven, middleware pattern proven, card-based UI pattern proven. Focus implementation effort on new domain logic (share request lifecycle, expiry background job) rather than infrastructure reinvention.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Query pattern**: Case-insensitive partial match (ILIKE '%query%')
  - Leverages existing case-insensitive username index (migration 028)
  - Most flexible for discovery (finds usernames with query anywhere)
- **Platform filtering**: Platform-specific search (not cross-platform)
  - User selects platform first (Twitch/YouTube/Kick/TikTok), then searches
  - Matches mental model—streamers know which platform someone uses
- **Search columns**: Username only (not display_name)
  - Clean, predictable results using unique identifiers
- **Result limit**: 10 results maximum
  - Keeps UI manageable, encourages specific queries
  - Reasonable for friend/collaborator discovery
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
- **Check location**: Gin middleware for /api/v1/shares/* routes
  - Centralized enforcement before share handlers run
  - DRY approach with careful route planning (some routes may not need premium checks)
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
- **Display style**: Card view
  - Each request as a card with visual hierarchy
  - Matches existing overlay card pattern in UI
- **Card information**:
  - Requester avatar + username
  - Platform source badges (showing platform icons, source name on hover)
  - Timestamp (when request was sent)
  - Status indicator (pending/accepted/rejected/expired)
  - Overlay name omitted (unnecessary detail)
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

### Deferred Ideas (OUT OF SCOPE)
None—discussion stayed within phase scope.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SHARE-01 | User can search for other users by platform username (Twitch, YouTube, Kick, TikTok) | PostgreSQL LOWER() functional index (migration 028), ILIKE pattern matching, pgx/v5 query patterns |
| SHARE-02 | User can send share request selecting an overlay to share | Repository pattern with share_requests table, Gin handler pattern, overlay-manager integration for overlay lookup |
| SHARE-03 | User can view pending incoming share requests in dashboard | React card-based UI pattern (proven in admin/users), tab filtering pattern, Next.js App Router server/client component patterns |
| PREMIUM-01 | Non-premium users blocked from creating or accepting shares | Gin middleware pattern (existing auth.go), is_premium boolean column (mirrors is_admin pattern), server-side enforcement before handler execution |
| PREMIUM-02 | Admin can mark specific users as premium for testing purposes | Admin handler pattern (existing admin.go), POST /api/v1/admin/users/:id/premium endpoint, AdminHandler struct with repo dependency injection |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.23+ | Backend services | Project standard, proven in all 11 existing services |
| Gin | Latest (gin-gonic/gin) | HTTP routing and middleware | Project standard for all HTTP services, middleware pattern proven in api-gateway |
| pgx/v5 | v5 | PostgreSQL driver | Project standard (pgxpool.Pool in all services), superior performance vs database/sql |
| PostgreSQL | 16 | Database | Project standard (CNPG cluster), functional indexes for case-insensitive search |
| React | 18+ | Frontend UI | Project standard (frontend/src/app/*) |
| Next.js | 14+ (App Router) | Frontend framework | Project standard, server/client components for dashboard pages |
| Tailwind CSS | 4.1+ | Styling | Project standard for all frontend components |
| Zap | go.uber.org/zap | Structured logging | Project standard for all services |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time.Ticker | stdlib | Background job scheduling | Every 5 minutes expiry check (simple, no external dependencies) |
| react-hot-toast | ^2.6.0 | Toast notifications | Error messages, success confirmations (already in package.json) |
| Vitest | ^4.0.18 | Frontend unit tests | Testing React components (already in frontend stack) |
| Playwright | ^1.58.2 | E2E tests | Testing full user workflows (already in frontend stack) |
| testing | stdlib | Go unit tests | Testing repository, handler, middleware layers |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| time.Ticker | River (riverqueue.com) | Ticker is simpler for single 5-minute job, River adds PostgreSQL-backed job queue (overkill for MVP, consider for Phase 19 with multiple jobs) |
| Gin middleware | Handler-level checks | Middleware is DRY and centralized (proven pattern in existing auth.go middleware), handler checks are repetitive and error-prone |
| Boolean column (is_premium) | Separate premium_users table | Boolean column matches is_admin pattern, separate table adds JOIN complexity for minimal benefit at current scale |
| ILIKE '%query%' | pg_trgm GIN index | ILIKE with LOWER() functional index is sufficient for username search (short text, exact substring match), pg_trgm is for fuzzy matching/full-text search |

**Installation:**
```bash
# Backend (Go) - all dependencies already in project
cd services/share-service  # New service or extend existing
go mod init github.com/caesar/all-chat/services/share-service
go get github.com/gin-gonic/gin
go get github.com/jackc/pgx/v5
go get go.uber.org/zap
go get github.com/google/uuid

# Frontend - all dependencies already in package.json
cd frontend
npm install  # Already has react-hot-toast, vitest, playwright
```

## Architecture Patterns

### Recommended Project Structure
```
services/share-service/  # NEW SERVICE (or extend overlay-manager)
├── cmd/main.go          # Entry point: logger, DB, HTTP server, background job
├── handlers/
│   ├── search.go        # GET /api/v1/users/search?platform=twitch&query=xqc
│   ├── shares.go        # POST /api/v1/shares, GET /api/v1/shares/incoming
│   └── admin.go         # POST /api/v1/admin/users/:id/premium
├── middleware/
│   └── premium.go       # RequirePremium() - validates is_premium before handler
├── models/
│   └── share.go         # ShareRequest struct
├── repository/
│   ├── user_search.go   # SearchUsersByPlatform(ctx, platform, query, limit)
│   ├── share_repo.go    # Create/List/UpdateStatus for share_requests
│   └── premium_repo.go  # UpdateUserPremium(ctx, userID, isPremium)
├── jobs/
│   └── expiry.go        # Background ticker to expire old requests
└── Dockerfile

frontend/src/app/dashboard/shares/  # NEW DASHBOARD SECTION
├── page.tsx             # Share request list with tabs (Pending/History)
├── components/
│   ├── ShareRequestCard.tsx   # Individual request card
│   ├── PlatformBadge.tsx      # Platform icon with hover tooltip
│   └── SearchUserModal.tsx    # Search + create request modal
```

### Pattern 1: Case-Insensitive Username Search
**What:** Query users by platform username with partial match, case-insensitive
**When to use:** User types into search box, looking for specific streamer
**Example:**
```go
// Source: Existing pattern in services/auth-service/repository/user_repository.go:133-150
func (r *UserSearchRepository) SearchUsersByPlatform(
    ctx context.Context,
    platform string, // "twitch", "youtube", "kick", "tiktok"
    query string,
    limit int,
) ([]models.User, error) {
    // User decision: ILIKE '%query%' for substring match
    // Migration 028 provides idx_users_username_lower for LOWER(username)
    sqlQuery := `
        SELECT id, username, display_name, profile_image_url
        FROM users
        WHERE LOWER(username) LIKE LOWER($1)
        AND $2::text IS NULL OR (
            (platform = 'twitch' AND twitch_id IS NOT NULL) OR
            (platform = 'youtube' AND google_id IS NOT NULL) OR
            (platform = 'kick' AND kick_id IS NOT NULL) OR
            (platform = 'tiktok' AND tiktok_id IS NOT NULL)
        )
        ORDER BY username
        LIMIT $3
    `

    // Wrap query for ILIKE pattern
    likePattern := "%" + query + "%"

    rows, err := r.db.Query(ctx, sqlQuery, likePattern, platform, limit)
    if err != nil {
        return nil, fmt.Errorf("search query failed: %w", err)
    }
    defer rows.Close()

    // Scan results...
}
```

### Pattern 2: Gin Middleware for Premium Enforcement
**What:** Centralized middleware validates is_premium before executing handler
**When to use:** Routes that require premium subscription (share creation/acceptance)
**Example:**
```go
// Source: Existing pattern in services/api-gateway/middleware/auth.go
// New file: services/share-service/middleware/premium.go

func RequirePremium() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id") // Set by JWTAuth middleware
        if userID == "" {
            c.JSON(401, gin.H{"error": "authentication required"})
            c.Abort()
            return
        }

        // Query database for is_premium column
        var isPremium bool
        err := db.QueryRow(c.Request.Context(),
            "SELECT is_premium FROM users WHERE id = $1", userID).Scan(&isPremium)

        if err != nil {
            c.JSON(500, gin.H{"error": "failed to verify premium status"})
            c.Abort()
            return
        }

        if !isPremium {
            c.JSON(403, gin.H{
                "error": "Premium feature required",
                "upgrade_url": "/upgrade", // User discretion: exact message
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// Usage in main.go:
premiumRoutes := r.Group("/api/v1/shares")
premiumRoutes.Use(middleware.JWTAuth(jwtSecret))
premiumRoutes.Use(middleware.RequirePremium()) // Check premium before handler
premiumRoutes.POST("", handler.CreateShareRequest)
premiumRoutes.POST("/:id/accept", handler.AcceptShareRequest)

// Routes without premium check:
shareRoutes := r.Group("/api/v1/shares")
shareRoutes.Use(middleware.JWTAuth(jwtSecret)) // Still need auth
shareRoutes.GET("/incoming", handler.ListIncomingRequests) // No premium to VIEW
```

### Pattern 3: Background Job with time.Ticker
**What:** Periodic job to expire pending share requests after 7 days
**When to use:** Simple scheduled tasks without complex queue requirements
**Example:**
```go
// Source: Go stdlib time.Ticker pattern + project graceful shutdown pattern
// New file: services/share-service/jobs/expiry.go

type ExpiryJob struct {
    repo   *repository.ShareRepository
    logger *zap.Logger
    ticker *time.Ticker
    done   chan bool
}

func NewExpiryJob(repo *repository.ShareRepository, logger *zap.Logger) *ExpiryJob {
    return &ExpiryJob{
        repo:   repo,
        logger: logger,
        ticker: time.NewTicker(5 * time.Minute), // User decision: every 5 minutes
        done:   make(chan bool),
    }
}

func (j *ExpiryJob) Start(ctx context.Context) {
    go func() {
        for {
            select {
            case <-j.done:
                return
            case <-j.ticker.C:
                j.expireOldRequests(ctx)
            }
        }
    }()
}

func (j *ExpiryJob) Stop() {
    j.ticker.Stop()
    j.done <- true
}

func (j *ExpiryJob) expireOldRequests(ctx context.Context) {
    // User decision: UPDATE query, 7-day expiry
    result, err := j.repo.ExpirePendingRequests(ctx)
    if err != nil {
        j.logger.Error("Failed to expire requests", zap.Error(err))
        return
    }
    if result > 0 {
        j.logger.Info("Expired share requests", zap.Int("count", result))
    }
}

// In cmd/main.go (after HTTP server setup):
expiryJob := jobs.NewExpiryJob(shareRepo, logger)
expiryJob.Start(context.Background())

// In graceful shutdown (before server shutdown):
expiryJob.Stop()
```

### Pattern 4: React Card-Based Dashboard UI
**What:** Card layout for share requests with tab filtering (Pending/History)
**When to use:** Displaying list of actionable items with rich metadata
**Example:**
```typescript
// Source: Existing pattern in frontend/src/app/admin/users/page.tsx (lines 263-379)
// New file: frontend/src/app/dashboard/shares/page.tsx

export default function ShareRequestsPage() {
  const [requests, setRequests] = useState<ShareRequest[]>([]);
  const [filter, setFilter] = useState<'pending' | 'history'>('pending');

  // Fetch requests on mount
  useEffect(() => {
    async function fetchRequests() {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch('/api/v1/shares/incoming', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      const data = await response.json();
      setRequests(data);
    }
    fetchRequests();
  }, []);

  // Filter logic (user decision: pending vs history)
  const displayRequests = requests.filter(r => {
    if (filter === 'pending') return r.status === 'pending';
    return ['accepted', 'rejected', 'expired'].includes(r.status);
  });

  return (
    <div className="px-4 py-6">
      {/* Tab Filters (pattern from admin/users/page.tsx:293-324) */}
      <div className="flex space-x-4 border-b border-gray-200 mb-6">
        <button
          onClick={() => setFilter('pending')}
          className={`pb-2 px-1 text-sm font-medium border-b-2 ${
            filter === 'pending'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-600'
          }`}
        >
          Pending ({requests.filter(r => r.status === 'pending').length})
        </button>
        <button
          onClick={() => setFilter('history')}
          className={`pb-2 px-1 text-sm font-medium border-b-2 ${
            filter === 'history'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-600'
          }`}
        >
          History
        </button>
      </div>

      {/* Card Grid (user decision: card view) */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {displayRequests.map(request => (
          <ShareRequestCard key={request.id} request={request} />
        ))}
      </div>
    </div>
  );
}

// New file: frontend/src/app/dashboard/shares/components/ShareRequestCard.tsx
interface ShareRequestCardProps {
  request: ShareRequest;
}

export function ShareRequestCard({ request }: ShareRequestCardProps) {
  return (
    <div className="bg-white rounded-lg shadow hover:shadow-md transition-shadow p-4">
      {/* User info (user decision: avatar + username) */}
      <div className="flex items-center mb-3">
        <img
          src={request.sender.profile_image_url}
          alt={request.sender.username}
          className="w-10 h-10 rounded-full"
        />
        <div className="ml-3">
          <p className="font-medium">{request.sender.display_name}</p>
          <p className="text-sm text-gray-500">@{request.sender.username}</p>
        </div>
      </div>

      {/* Platform badges (user decision: icons with hover) */}
      <div className="flex gap-2 mb-3">
        {request.overlay_sources.map(source => (
          <PlatformBadge key={source.id} source={source} />
        ))}
      </div>

      {/* Timestamp (user decision: when request sent) */}
      <p className="text-xs text-gray-500">
        {formatDistanceToNow(new Date(request.created_at))} ago
      </p>

      {/* Status indicator */}
      <div className="mt-3 pt-3 border-t">
        <StatusBadge status={request.status} />
      </div>
    </div>
  );
}
```

### Pattern 5: Database Migration with Foreign Keys
**What:** Create share_requests table with proper constraints
**When to use:** New tables that reference existing tables (users, overlays)
**Example:**
```sql
-- New file: migrations/030_share_requests.sql
-- Pattern: ON DELETE RESTRICT (project standard, see migration 001)

CREATE TABLE share_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    sender_overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE RESTRICT,
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'accepted', 'rejected', 'expired')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    CONSTRAINT no_self_share CHECK (sender_user_id != recipient_user_id)
);

CREATE INDEX idx_share_requests_recipient ON share_requests(recipient_user_id, status);
CREATE INDEX idx_share_requests_sender ON share_requests(sender_user_id);
CREATE INDEX idx_share_requests_expiry ON share_requests(status, expires_at)
    WHERE status = 'pending'; -- Partial index for expiry job

-- Add is_premium column to users table (pattern from migration 009)
ALTER TABLE users ADD COLUMN is_premium BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_users_is_premium ON users(is_premium) WHERE is_premium = TRUE;
COMMENT ON COLUMN users.is_premium IS 'Whether the user has premium subscription';
```

### Anti-Patterns to Avoid
- **Client-side premium enforcement:** Always validate is_premium on server-side, never trust client to hide premium features (security bypass risk)
- **Caching premium status:** Don't cache is_premium flag—query is fast, cache invalidation is complex, stale data causes support issues
- **Global middleware for all routes:** Don't apply RequirePremium() globally—viewing requests doesn't need premium, only creating/accepting does
- **Nested share requests:** Don't allow recipient to share sender's overlay to third party (permission complexity, cascading load issues)
- **ILIKE without LOWER():** Don't use `ILIKE '%query%'` directly—use `LOWER(username) LIKE LOWER($1)` to leverage functional index

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT middleware | Custom token validation | Existing middleware.JWTAuth() in api-gateway | Already proven, handles Bearer token extraction, claim validation, context injection |
| Admin endpoint pattern | Custom admin handler structure | Extend services/auth-service/handlers/admin.go | Proven AdminHandler struct with repo/db/logger/jwtSecret dependencies |
| Case-insensitive search | String manipulation in Go | PostgreSQL LOWER() + functional index (migration 028) | Database indexes are faster than Go string ops, already optimized for users table |
| Background job queue | Custom job processing system | time.Ticker for simple periodic job | For single 5-minute expiry job, Ticker is simpler than River/gocraft/work (save complexity for Phase 19 multi-job scenarios) |
| Toast notifications | Custom notification system | react-hot-toast (already in package.json) | Proven in existing frontend, accessible, customizable |
| Card component styling | Custom CSS classes | Tailwind utility classes (project standard) | Matches existing admin/users page pattern, responsive by default |

**Key insight:** The project has established patterns for 80% of this phase's needs. The "new" work is domain logic (share request lifecycle, platform filtering), not infrastructure. Reusing proven patterns reduces bugs, accelerates development, and maintains consistency with existing codebase.

## Common Pitfalls

### Pitfall 1: Forgetting Server-Side Premium Enforcement
**What goes wrong:** Frontend hides premium features, but API endpoints remain accessible via direct HTTP requests
**Why it happens:** Developer focuses on UI implementation, forgets to add middleware to backend routes
**How to avoid:**
- Add RequirePremium() middleware to ALL premium routes during initial implementation
- Write integration test that attempts to call premium endpoint without premium flag (should return 403)
- Code review checklist: "Are all share creation/acceptance routes protected?"
**Warning signs:**
- Postman/curl can create share requests for non-premium users
- Frontend shows "Premium required" but backend returns 200 OK

### Pitfall 2: Case-Sensitive Search Breaking Functional Index
**What goes wrong:** Query uses `username ILIKE $1` instead of `LOWER(username) LIKE LOWER($1)`, bypassing functional index from migration 028
**Why it happens:** ILIKE implies case-insensitivity, developer doesn't realize functional index requires explicit LOWER()
**How to avoid:**
- Always use `LOWER(username) LIKE LOWER($1)` pattern (not ILIKE directly)
- Run EXPLAIN ANALYZE on search query to verify index usage (should show "Index Scan using idx_users_username_lower")
- Add comment in repository code referencing migration 028
**Warning signs:**
- Search is slow with > 1000 users (sequential scan instead of index scan)
- EXPLAIN plan shows "Seq Scan on users" instead of index usage

### Pitfall 3: Background Job Blocking Server Shutdown
**What goes wrong:** Ticker continues running after HTTP server shutdown signal, delaying graceful shutdown or causing panic
**Why it happens:** Ticker runs in separate goroutine, not integrated into shutdown flow
**How to avoid:**
- Create ExpiryJob struct with Start()/Stop() methods (see Pattern 3)
- Call expiryJob.Stop() BEFORE srv.Shutdown() in main.go
- Use project's standard 25-second shutdown timeout to ensure job can complete current iteration
**Warning signs:**
- Server takes > 30 seconds to shutdown
- "context deadline exceeded" errors in logs during shutdown
- Docker container killed with SIGKILL instead of SIGTERM

### Pitfall 4: Missing Foreign Key Constraint on share_requests
**What goes wrong:** User or overlay deleted, share_requests table has orphaned records, JOIN queries return NULL
**Why it happens:** Developer forgets ON DELETE clause or uses CASCADE instead of RESTRICT
**How to avoid:**
- Always use ON DELETE RESTRICT (project standard, see migration 001)
- Add CHECK constraint to prevent self-sharing (sender != recipient)
- Write integration test that attempts to delete user with pending share request (should fail with FK constraint error)
**Warning signs:**
- Share request cards show blank user info
- COUNT(*) on share_requests doesn't match JOIN with users

### Pitfall 5: Middleware Applied to Wrong Routes
**What goes wrong:** RequirePremium() applied to GET /api/v1/shares/incoming, blocking non-premium users from viewing their requests
**Why it happens:** Developer applies middleware to entire route group instead of specific endpoints
**How to avoid:**
- Apply middleware at method level, not group level: `premiumRoutes.POST("", handler.CreateRequest)`
- Document which routes need premium check in code comments
- User story test: "As non-premium user, I can VIEW incoming requests but CANNOT create/accept"
**Warning signs:**
- Non-premium users get 403 when viewing their own share requests
- Support tickets: "I can't see my share requests even though I didn't create any"

### Pitfall 6: Expiry Job Missing Database Context Timeout
**What goes wrong:** ExpirePendingRequests() query runs indefinitely during database slowdown, ticker accumulates blocked goroutines
**Why it happens:** Background job uses context.Background() without timeout
**How to avoid:**
- Use context.WithTimeout() for expiry job queries: `ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)`
- Log slow queries that exceed threshold (> 5 seconds)
- Monitor goroutine count to detect ticker goroutine leaks
**Warning signs:**
- Goroutine count increases over time (memory leak)
- Database connection pool exhausted during normal operation
- Expiry job stops running (all goroutines blocked)

## Code Examples

Verified patterns from official sources and project codebase:

### User Search Handler (Platform-Specific)
```go
// services/share-service/handlers/search.go
// GET /api/v1/users/search?platform=twitch&query=xqc

func (h *SearchHandler) SearchUsers(c *gin.Context) {
    platform := c.Query("platform")
    query := c.Query("query")

    // Validation
    if platform == "" || query == "" {
        c.JSON(400, gin.H{"error": "platform and query required"})
        return
    }

    validPlatforms := map[string]bool{
        "twitch": true, "youtube": true, "kick": true, "tiktok": true,
    }
    if !validPlatforms[platform] {
        c.JSON(400, gin.H{"error": "invalid platform"})
        return
    }

    // User decision: 10 result limit
    users, err := h.repo.SearchUsersByPlatform(
        c.Request.Context(), platform, query, 10)

    if err != nil {
        h.logger.Error("Search failed", zap.Error(err))
        c.JSON(500, gin.H{"error": "search failed"})
        return
    }

    c.JSON(200, users)
}
```

### Share Request Creation Handler
```go
// services/share-service/handlers/shares.go
// POST /api/v1/shares
// Body: {"recipient_username": "xqc", "overlay_id": "uuid"}

func (h *ShareHandler) CreateRequest(c *gin.Context) {
    senderUserID := c.GetString("user_id") // From JWTAuth middleware

    var req struct {
        RecipientUsername string `json:"recipient_username" binding:"required"`
        OverlayID         string `json:"overlay_id" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // Verify sender owns overlay
    overlay, err := h.overlayRepo.GetByID(c.Request.Context(), req.OverlayID)
    if err != nil || overlay.UserID != senderUserID {
        c.JSON(403, gin.H{"error": "overlay not found or not owned"})
        return
    }

    // Lookup recipient (case-insensitive)
    recipient, err := h.userRepo.GetByUsername(
        c.Request.Context(), req.RecipientUsername)
    if err != nil {
        c.JSON(404, gin.H{"error": "user not found"})
        return
    }

    // Prevent self-share (also enforced by DB constraint)
    if recipient.ID == senderUserID {
        c.JSON(400, gin.H{"error": "cannot share with yourself"})
        return
    }

    // Create share request (expires in 7 days)
    shareRequest := &models.ShareRequest{
        SenderUserID:    senderUserID,
        SenderOverlayID: req.OverlayID,
        RecipientUserID: recipient.ID,
        Status:          "pending",
        ExpiresAt:       time.Now().Add(7 * 24 * time.Hour),
    }

    err = h.shareRepo.Create(c.Request.Context(), shareRequest)
    if err != nil {
        h.logger.Error("Failed to create share request", zap.Error(err))
        c.JSON(500, gin.H{"error": "failed to create request"})
        return
    }

    c.JSON(201, shareRequest)
}
```

### Admin Premium Management Handler
```go
// services/share-service/handlers/admin.go
// POST /api/v1/admin/users/:id/premium
// Body: {"is_premium": true}

func (h *AdminHandler) SetUserPremium(c *gin.Context) {
    adminUserID := c.GetString("user_id") // From JWTAuth middleware
    targetUserID := c.Param("id")

    var req struct {
        IsPremium bool `json:"is_premium"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // Update users.is_premium column (pattern from migration 009)
    err := h.repo.UpdateUserPremium(
        c.Request.Context(), targetUserID, req.IsPremium)

    if err != nil {
        h.logger.Error("Failed to update premium status",
            zap.String("admin_id", adminUserID),
            zap.String("target_id", targetUserID),
            zap.Error(err))
        c.JSON(500, gin.H{"error": "failed to update premium status"})
        return
    }

    h.logger.Info("Premium status updated",
        zap.String("admin_id", adminUserID),
        zap.String("target_id", targetUserID),
        zap.Bool("is_premium", req.IsPremium))

    c.JSON(200, gin.H{
        "message": "premium status updated",
        "user_id": targetUserID,
        "is_premium": req.IsPremium,
    })
}
```

### Expiry Job Repository Method
```go
// services/share-service/repository/share_repo.go

func (r *ShareRepository) ExpirePendingRequests(ctx context.Context) (int, error) {
    // User decision: UPDATE query with 7-day expiry check
    result, err := r.db.Exec(ctx, `
        UPDATE share_requests
        SET status = 'expired', responded_at = NOW()
        WHERE status = 'pending' AND expires_at < NOW()
    `)

    if err != nil {
        return 0, fmt.Errorf("failed to expire requests: %w", err)
    }

    return int(result.RowsAffected()), nil
}
```

### Platform Badge Component (React)
```typescript
// frontend/src/app/dashboard/shares/components/PlatformBadge.tsx

interface PlatformBadgeProps {
  source: OverlaySource; // { platform: string, channel_name: string }
}

const PLATFORM_ICONS = {
  twitch: '🟣',   // Or import actual icon
  youtube: '🔴',
  kick: '🟢',
  tiktok: '⚫',
};

export function PlatformBadge({ source }: PlatformBadgeProps) {
  return (
    <div
      className="inline-flex items-center px-2 py-1 rounded bg-gray-100 text-xs"
      title={`${source.channel_name} on ${source.platform}`}
    >
      <span className="mr-1">{PLATFORM_ICONS[source.platform]}</span>
      <span className="capitalize">{source.platform}</span>
    </div>
  );
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| pg_trgm for all text search | Functional index (LOWER()) for exact substring | PostgreSQL 9.1+ (2011) | Simpler for username search (short text, exact match), pg_trgm still best for fuzzy/full-text |
| Separate premium_users table | Boolean column with partial index | Project standard (migration 009 is_admin) | Simpler queries (no JOIN), faster lookups, matches existing pattern |
| Cron jobs for background tasks | In-process time.Ticker | Go 1.0+ (2012) | Simpler deployment (no cron daemon), easier testing, graceful shutdown support |
| Redux for state management | Zustand | Frontend package.json (2024+) | Simpler API, less boilerplate, already in project |
| Pages Router (Next.js) | App Router | Next.js 13+ (2023) | Server/client components, better data fetching, already in project |

**Deprecated/outdated:**
- **Class-based React components:** Project uses functional components with hooks (React 16.8+, 2019)
- **database/sql driver:** Project uses pgx/v5 for superior PostgreSQL performance (2020+)
- **context.TODO() for production code:** Project uses proper context propagation with timeouts

## Open Questions

1. **Should share-service be a new microservice or extend overlay-manager?**
   - What we know: Project has 11 existing microservices, share logic relates to overlays
   - What's unclear: Scaling implications of adding to overlay-manager vs separate service
   - Recommendation: Start as separate share-service for domain isolation, can merge later if overhead becomes issue (easier to merge than split)

2. **What happens to share_requests when user is deleted?**
   - What we know: ON DELETE RESTRICT prevents deletion, project uses application-level cascade
   - What's unclear: Should admin be able to delete user with pending shares? Should shares auto-reject?
   - Recommendation: Implement application-level cascade in auth-service—before user deletion, auto-reject all pending shares where user is sender/recipient (log for audit)

3. **Should expired requests be deleted or marked as expired?**
   - What we know: User decision is "expired" status (not deletion), keeps history for debugging
   - What's unclear: Retention policy—do we ever hard-delete expired requests?
   - Recommendation: Keep expired requests indefinitely for MVP (storage is cheap, debugging value is high), add retention policy in Phase 19 if needed

4. **How to handle race condition when premium status changes during request creation?**
   - What we know: No caching, query database on every request
   - What's unclear: If admin removes premium between request initiation and completion, should request succeed or fail?
   - Recommendation: Use database transaction with SELECT FOR UPDATE on users.is_premium—lock user row, verify premium, create request atomically (prevents TOCTOU race)

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + Vitest ^4.0.18 (frontend) |
| Config file | None for Go (stdlib), vitest.config.ts (frontend) |
| Quick run command | `go test -run TestPremiumMiddleware ./services/share-service/...` (backend), `npm test` (frontend) |
| Full suite command | `make test` (all Go services), `npm test` (frontend) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SHARE-01 | Username search returns case-insensitive partial matches for selected platform | unit | `go test -run TestSearchUsersByPlatform ./services/share-service/repository/...` | ❌ Wave 0 |
| SHARE-01 | Search uses functional index (LOWER) not sequential scan | integration | Manual EXPLAIN ANALYZE verification | ❌ Wave 0 |
| SHARE-02 | Share request creation validates sender owns overlay | unit | `go test -run TestCreateRequest ./services/share-service/handlers/...` | ❌ Wave 0 |
| SHARE-02 | Share request prevents self-sharing via DB constraint | integration | `go test -run TestSelfShareConstraint ./services/share-service/repository/...` | ❌ Wave 0 |
| SHARE-03 | Dashboard fetches and displays pending requests in card view | e2e | `npm run test:e2e -- shares.spec.ts` | ❌ Wave 0 |
| SHARE-03 | Tab filtering separates pending from history requests | unit | `npm test -- ShareRequestsPage.test.tsx` | ❌ Wave 0 |
| PREMIUM-01 | RequirePremium middleware blocks non-premium POST /api/v1/shares | integration | `go test -run TestPremiumMiddleware ./services/share-service/middleware/...` | ❌ Wave 0 |
| PREMIUM-01 | Non-premium users can view requests (no middleware on GET) | integration | `go test -run TestNonPremiumCanView ./services/share-service/handlers/...` | ❌ Wave 0 |
| PREMIUM-02 | Admin endpoint updates is_premium column and returns success | unit | `go test -run TestSetUserPremium ./services/share-service/handlers/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -run Test{TaskName} ./services/share-service/...` (< 10s for relevant tests)
- **Per wave merge:** `make test` (full Go suite) + `npm test` (frontend suite)
- **Phase gate:** Full suite green + manual EXPLAIN ANALYZE for search query index usage

### Wave 0 Gaps
- [ ] `services/share-service/handlers/search_test.go` — covers SHARE-01 (search validation)
- [ ] `services/share-service/repository/share_repo_test.go` — covers SHARE-02 (FK constraints, self-share)
- [ ] `services/share-service/middleware/premium_test.go` — covers PREMIUM-01 (middleware enforcement)
- [ ] `services/share-service/handlers/admin_test.go` — covers PREMIUM-02 (admin endpoint)
- [ ] `frontend/src/app/dashboard/shares/__tests__/ShareRequestsPage.test.tsx` — covers SHARE-03 (tab filtering)
- [ ] `frontend/tests/e2e/shares.spec.ts` — covers SHARE-03 (full user flow)
- [ ] Test fixtures: Mock ShareRepository, mock UserRepository with premium flags
- [ ] Integration test database setup: Apply migration 030 (share_requests + is_premium)

## Sources

### Primary (HIGH confidence)
- Existing codebase patterns:
  - `migrations/028_case_insensitive_username_index.sql` - LOWER() functional index pattern
  - `migrations/009_add_admin_role.sql` - Boolean column with partial index pattern
  - `services/auth-service/repository/user_repository.go:133-150` - GetByUsername case-insensitive query
  - `services/api-gateway/middleware/auth.go` - Gin middleware pattern
  - `services/auth-service/handlers/admin.go` - AdminHandler pattern with repo/db/logger dependencies
  - `frontend/src/app/admin/users/page.tsx:263-379` - Card-based UI with tab filtering
  - `frontend/package.json` - Vitest ^4.0.18, Playwright ^1.58.2, react-hot-toast ^2.6.0
- PostgreSQL official documentation:
  - https://www.postgresql.org/docs/current/indexes-types.html - Functional indexes for case-insensitive search
- Go stdlib documentation:
  - https://pkg.go.dev/time#Ticker - Periodic task pattern

### Secondary (MEDIUM confidence)
- [How to Create Custom Middleware for Gin in Go](https://oneuptime.com/blog/post/2026-01-30-how-to-create-custom-middleware-for-gin-in-go/view) - 2026 middleware patterns
- [Optimizing LIKE and ILIKE Queries in PostgreSQL](https://medium.com/@anonrongbo/optimizing-like-and-ilike-queries-and-index-usage-in-postgresql-833d726702ef) - Functional index vs pg_trgm tradeoffs
- [Go Background Job Processing](https://oneuptime.com/blog/post/2026-01-30-go-background-job-processing/view) - time.Ticker vs job queue libraries
- [PostgreSQL More Performance for LIKE/ILIKE](https://www.cybertec-postgresql.com/en/postgresql-more-performance-for-like-and-ilike-statements/) - Short text field optimization
- [River - Fast, reliable background jobs in Go](https://riverqueue.com/) - Database-backed job queue (defer to Phase 19)

### Tertiary (LOW confidence)
None—all findings verified with project codebase or official documentation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in project (Go 1.23+, Gin, pgx/v5, React 18+, Next.js 14+, Tailwind)
- Architecture: HIGH - Patterns proven in existing services (middleware, repository, admin handler, card UI)
- Pitfalls: HIGH - Derived from project codebase review (ON DELETE RESTRICT, functional index usage, graceful shutdown)
- Validation: MEDIUM - Test framework identified, specific tests need creation (Wave 0 gaps listed)

**Research date:** 2026-03-09
**Valid until:** 2026-04-09 (30 days - stable stack, established patterns)
