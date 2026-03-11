# Architecture Patterns: Chat Overlay Sharing

**Domain:** Chat overlay sharing for All-Chat platform
**Researched:** 2026-03-08

## Recommended Architecture

Chat overlay sharing integrates with the existing All-Chat microservices architecture through a **hybrid approach**: a new `share-service` for share lifecycle management combined with extensions to existing services for message routing and permissions.

### High-Level Integration

```
┌─────────────────────────────────────────────────────────────────────┐
│                     NEW: Share Service (8091)                        │
│  • Share request CRUD (create, accept, revoke)                      │
│  • Share lifecycle management (expiry, stream detection)            │
│  • Permission validation (premium, active status)                   │
│  • User search by platform username                                 │
└───────────────┬─────────────────────────────────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────────────────────────────────┐
│              EXISTING: Overlay-Manager (8082)                        │
│  NEW: Share source type in overlay_chat_sources                     │
│  • platform="share", channel_id=share_id                            │
│  • Config: {source_overlay_id, source_user_id}                      │
└───────────────┬─────────────────────────────────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────────────────────────────────┐
│            MODIFIED: Message Processor (8087)                        │
│  NEW: Share source resolver                                         │
│  • Detects platform="share" sources                                 │
│  • Resolves to actual platform sources from source overlay          │
│  • Publishes to both source and consuming overlay channels          │
└───────────────┬─────────────────────────────────────────────────────┘
                │
                ▼
┌─────────────────────────────────────────────────────────────────────┐
│              EXISTING: Redis Pub/Sub (unchanged)                     │
│  • overlay:{source_overlay_id} (original messages)                  │
│  • overlay:{consuming_overlay_id} (shared messages)                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| **share-service** (NEW) | Share CRUD, expiry, permissions, user search | PostgreSQL, Redis, auth-service (user lookup), twitch-eventsub-listener (stream status) |
| **overlay-manager** (EXTEND) | Add share sources to overlays | share-service (validate share exists), PostgreSQL |
| **message-processor** (EXTEND) | Resolve share sources, duplicate to consuming overlays | PostgreSQL (resolve shares), Redis Pub/Sub |
| **api-gateway** (EXTEND) | Share endpoints proxy | share-service |
| **auth-service** (EXTEND) | Premium flag on users table | PostgreSQL |

### Data Flow

```
Share Request Flow:
1. User A searches for User B by platform username
   → share-service queries users table

2. User A creates share request (overlay_id, recipient_user_id, expiry_type)
   → share-service validates premium status
   → share-service inserts to shares table (status="pending")

3. User B views pending requests
   → share-service queries WHERE recipient_user_id = B AND status="pending"

4. User B accepts share (selects return overlay, expiry)
   → share-service validates both users premium
   → share-service updates share (status="active", return_overlay_id)
   → share-service inserts bidirectional share record

Share Source Activation Flow:
5. User A adds User B's shared overlay as source to their overlay
   → overlay-manager validates share exists and is active
   → overlay-manager inserts overlay_chat_sources (platform="share", channel_id=share_id, config={source_overlay_id})

6. Source activation triggers listeners
   → source-manager detects new share source
   → source-manager publishes control command to message-processor

Message Delivery Flow:
7. Message arrives from platform listener (Twitch, YouTube, etc.)
   → Redis Streams: chat:raw

8. Message processor consumes message
   → Normalize → Enrich → Publish to overlay:{source_overlay_id}
   → NEW: Check if source_overlay_id has active shares
   → Resolve share sources pointing to this overlay
   → Publish to overlay:{consuming_overlay_id} for each share

9. API Gateway subscribes to both channels
   → Broadcast to source overlay WebSocket clients
   → Broadcast to consuming overlay WebSocket clients (shared messages)
```

## Patterns to Follow

### Pattern 1: Share as Virtual Platform Source

**What:** Treat shared overlays as a special "platform" type in overlay_chat_sources

**When:** User adds a shared overlay to their overlay configuration

**Implementation:**

```sql
-- In overlay_chat_sources table
INSERT INTO overlay_chat_sources (
    overlay_id,          -- Consuming overlay
    platform,            -- "share" (new virtual platform)
    channel_id,          -- share_id (UUID from shares table)
    channel_name,        -- "{source_user_display_name}'s chat"
    config,              -- {"source_overlay_id": "uuid", "source_user_id": "uuid"}
    is_active
) VALUES (
    '...', 'share', '...', 'xQc's chat', '{"source_overlay_id": "..."}', true
);
```

**Why:**
- Reuses existing source activation flow (source-manager)
- Consistent with other platform sources (Twitch, YouTube, etc.)
- Automatic activation/deactivation on WebSocket connect/disconnect
- No changes to frontend overlay configuration UI (just another source type)

**Advantages:**
- Minimal changes to existing codebase
- Consistent UX (add share like any other source)
- Automatic lifecycle management

**Trade-offs:**
- "platform" field semantics stretched (share is not a platform)
- Additional lookup required in message processor (resolve share → source overlay)

### Pattern 2: Database-per-Service with Share Table Ownership

**What:** share-service owns the `shares` table, other services query via API

**When:** Other services need to validate share existence or status

**Implementation:**

```sql
-- Owned by share-service
CREATE TABLE shares (
    id UUID PRIMARY KEY,
    requester_user_id UUID NOT NULL,
    recipient_user_id UUID NOT NULL,
    requester_overlay_id UUID NOT NULL,
    recipient_overlay_id UUID,              -- NULL until accepted
    status VARCHAR(20) NOT NULL,            -- "pending", "active", "expired", "revoked"
    expiry_type VARCHAR(20) NOT NULL,       -- "this_stream", "duration", "unlimited"
    expiry_duration INTERVAL,               -- NULL if unlimited or this_stream
    expires_at TIMESTAMP,                   -- NULL if unlimited
    created_at TIMESTAMP,
    accepted_at TIMESTAMP,                  -- NULL if pending
    revoked_at TIMESTAMP,                   -- NULL if not revoked
    revoked_by_user_id UUID,                -- Who revoked (NULL if not revoked)
    CONSTRAINT check_status CHECK (status IN ('pending', 'active', 'expired', 'revoked'))
);

-- Queried by overlay-manager, message-processor
-- Via share-service HTTP API (not direct database access)
```

**API Contract:**

```go
// share-service provides HTTP endpoints
GET  /api/v1/shares/:share_id               // Get share details
GET  /api/v1/shares?user_id=xxx&status=active  // List shares
POST /api/v1/shares/validate                // Validate share exists and is active
```

**Why:**
- Follows microservices principle: each service owns its data
- Prevents coupling through shared database tables
- Allows share-service to enforce business rules (premium check, expiry)
- Changes to share schema don't break other services

**Reference:**
- [Database Per Service Pattern](https://microservices.io/patterns/data/database-per-service.html)
- [Microsoft: Data Considerations for Microservices](https://learn.microsoft.com/en-us/azure/architecture/microservices/design/data-considerations)

### Pattern 3: Message Fan-Out at Processor Layer

**What:** Message processor publishes enriched messages to multiple overlay channels when shares exist

**When:** Message arrives for an overlay that has active shares

**Implementation:**

```go
// message-processor: publisher/pubsub_publisher.go
func (p *PubSubPublisher) PublishWithShares(ctx context.Context, msg *models.UnifiedChatMessage) error {
    // Always publish to source overlay
    overlayIDs := []string{msg.OverlayID}

    // Lookup shares pointing to this overlay (cache in Redis)
    shares, _ := p.shareResolver.GetActiveShares(ctx, msg.OverlayID)
    for _, share := range shares {
        overlayIDs = append(overlayIDs, share.ConsumingOverlayID)
    }

    // Batch publish to all overlay channels
    return p.PublishToMultiple(ctx, overlayIDs, msg)
}
```

**Why:**
- Messages enriched once, delivered to multiple overlays
- Avoids duplicate processing (normalization, emote enrichment)
- Low latency (single Redis pipeline for multiple PUBLISH)
- No changes to listener services (still publish to chat:raw)

**Caching Strategy:**

```go
// Redis cache: shares:overlay:{overlay_id} → JSON array of consuming overlay IDs
// TTL: 5 minutes (short TTL to catch revocations quickly)
// Invalidated on share accept/revoke

// Example cache value:
// shares:overlay:source-123 → ["consuming-456", "consuming-789"]
```

**Performance:**
- Cache hit: <5ms to resolve shares
- Cache miss: 10-20ms (database query to shares table)
- Expected hit rate: >95% (overlays rarely change share config)

### Pattern 4: Stream Lifecycle Detection for Expiry

**What:** Detect stream start/end events to expire "this stream" shares

**When:** Share has `expiry_type="this_stream"`

**Implementation:**

```go
// share-service subscribes to Twitch EventSub, YouTube activity, Kick/TikTok stream status

// Twitch: Use existing twitch-eventsub-listener
// - stream.online → record stream session start
// - stream.offline → expire shares for this user

// YouTube: Use existing stream history tracking (migrations/010_stream_history_tracking.sql)
// - Check stream_sessions table for active streams

// Kick: Research needed (see PITFALLS.md)

// TikTok: Use existing tiktok-listener stream detection
```

**Expiry Check Cron:**

```go
// share-service: runs every 5 minutes
func (s *ShareService) ExpireThisStreamShares(ctx context.Context) {
    // Find shares with expiry_type="this_stream" AND status="active"
    shares, _ := s.repo.GetActiveThisStreamShares(ctx)

    for _, share := range shares {
        // Check if requester's stream is offline
        isLive := s.streamDetector.IsStreamLive(ctx, share.RequesterUserID)
        if !isLive {
            s.repo.ExpireShare(ctx, share.ID)
        }

        // Check if recipient's stream is offline
        isLive = s.streamDetector.IsStreamLive(ctx, share.RecipientUserID)
        if !isLive {
            s.repo.ExpireShare(ctx, share.ID)
        }
    }
}
```

**Why:**
- Automatic expiry without manual user action
- Leverages existing stream detection infrastructure
- Respects user intent ("share for this stream only")

## Anti-Patterns to Avoid

### Anti-Pattern 1: Direct Database Joins Across Services

**What:** Message processor directly joining `overlay_chat_sources` with `shares` table

**Why bad:**
- Tight coupling between services
- Schema changes in share-service break message-processor
- Violates microservices principle (service autonomy)
- Hard to scale independently (shared database bottleneck)

**Instead:** API calls or Redis cache for cross-service data

### Anti-Pattern 2: Synchronous Share Validation in Message Path

**What:** Message processor calls share-service API for every message to validate share is active

**Why bad:**
- Adds 10-50ms latency per message
- Share service becomes bottleneck (3,000 msg/s × 10ms = 30 concurrent requests)
- Single point of failure (share-service down = no messages)

**Instead:** Cache active shares in Redis, refresh on share lifecycle events

```go
// BAD: Synchronous validation
for each message:
    shares := callShareServiceAPI(overlayID)  // 10-50ms per message!
    publishToShares(shares)

// GOOD: Cached shares with event invalidation
shares := getFromRedisCache(overlayID)  // <5ms
publishToShares(shares)

// share-service publishes to Redis Pub/Sub on share lifecycle events
// message-processor subscribes and invalidates cache
```

### Anti-Pattern 3: Nested Share Chains

**What:** Allowing User A to share their overlay (which contains User B's shared overlay) with User C

**Why bad:**
- Infinite loop potential (A shares to B, B shares to A)
- Permission explosion (transitive sharing violates intent)
- Complexity in expiry (if B's share expires, does C's share also expire?)
- Hard to reason about ownership and revocation

**Instead:** Shares resolve to platform sources only (one level deep)

```sql
-- Enforce constraint: shares cannot reference other shares
-- In overlay_chat_sources: if platform="share", verify channel_id points to share with platform sources only
```

### Anti-Pattern 4: Share Source as Separate Overlay Type

**What:** Creating a new `shared_overlays` table separate from `overlay_chat_sources`

**Why bad:**
- Duplicates source management logic (activation, deactivation, lifecycle)
- Frontend needs separate UI for share sources vs platform sources
- Source-manager must handle two different data models
- More complex codebase (two paths for similar functionality)

**Instead:** Treat shares as virtual platform sources (Pattern 1)

## Scalability Considerations

| Concern | At 100 shares | At 10K shares | At 1M shares |
|---------|---------------|---------------|--------------|
| **Share lookup** | In-memory cache (Redis) | Redis cache with partitioning | Redis Cluster with sharding by overlay_id |
| **Message fan-out** | Single Redis PUBLISH pipeline | Same (pipeline supports 100s of overlays) | Split hot overlays to dedicated Pub/Sub instances |
| **Expiry checks** | Cron every 5 minutes | Background worker pool (10 workers) | Distributed task queue (BullMQ, Temporal) |
| **User search** | PostgreSQL LIKE query | PostgreSQL full-text search index | ElasticSearch index on usernames |

### Fan-Out Amplification

**Scenario:** Popular streamer (100K viewers) shares their chat with 1,000 other streamers

**Impact:**
- Message rate: 500 msg/s (popular channel)
- Fan-out: 1,000 overlays × 500 msg/s = 500K messages/s published to Redis Pub/Sub
- Redis Pub/Sub capacity: ~500K msg/s per instance (acceptable, but close to limit)

**Mitigation:**
1. **Cap shares per overlay:** Limit to 10 active shares per overlay (prevent abuse)
2. **Hot overlay detection:** If overlay has >100 shares, flag for manual review (likely spam)
3. **Rate limiting:** Throttle share requests (5 requests/hour per user)
4. **Premium tier limits:** Free tier = 1 share, Premium = 10 shares, Partner = 100 shares

### Share Cache Invalidation

**Challenge:** Cache must be invalidated within seconds of share revocation

**Solution:** Redis Pub/Sub for cache invalidation

```go
// share-service publishes on share lifecycle events
redis.Publish("shares:invalidate", json.Marshal({
    "source_overlay_id": "...",
    "consuming_overlay_id": "...",
    "action": "revoke"  // or "activate", "expire"
}))

// message-processor subscribes
func (p *MessageProcessor) handleShareInvalidation(msg ShareInvalidationEvent) {
    // Delete cache entries
    p.cache.Delete("shares:overlay:" + msg.SourceOverlayID)
    p.cache.Delete("shares:overlay:" + msg.ConsumingOverlayID)
}
```

**Fallback:** If message arrives before cache invalidation, worst case is one extra message delivered to revoked share (acceptable trade-off for low latency)

## Database Schema Extensions

### New Table: shares

```sql
CREATE TABLE shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Requester (initiator of share request)
    requester_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requester_overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Recipient (receives share request)
    recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE,  -- NULL until accepted

    -- Status and lifecycle
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expiry_type VARCHAR(20) NOT NULL,
    expiry_duration INTERVAL,           -- Used if expiry_type="duration"
    expires_at TIMESTAMP,                -- Computed expiry timestamp

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    accepted_at TIMESTAMP,               -- When recipient accepted
    revoked_at TIMESTAMP,                -- When revoked
    revoked_by_user_id UUID REFERENCES users(id),  -- Who revoked

    -- Constraints
    CONSTRAINT check_status CHECK (status IN ('pending', 'active', 'expired', 'revoked')),
    CONSTRAINT check_expiry_type CHECK (expiry_type IN ('this_stream', 'duration', 'unlimited')),
    CONSTRAINT check_not_self_share CHECK (requester_user_id != recipient_user_id)
);

-- Indexes
CREATE INDEX idx_shares_requester_user_id ON shares(requester_user_id);
CREATE INDEX idx_shares_recipient_user_id ON shares(recipient_user_id);
CREATE INDEX idx_shares_status ON shares(status);
CREATE INDEX idx_shares_requester_overlay_id ON shares(requester_overlay_id) WHERE status = 'active';
CREATE INDEX idx_shares_recipient_overlay_id ON shares(recipient_overlay_id) WHERE status = 'active';

-- Index for expiry checks
CREATE INDEX idx_shares_expiry ON shares(expires_at) WHERE status = 'active' AND expires_at IS NOT NULL;
```

### Modified Table: users

```sql
-- Add premium flag
ALTER TABLE users ADD COLUMN is_premium BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN premium_expires_at TIMESTAMP;

-- Index for premium checks
CREATE INDEX idx_users_is_premium ON users(is_premium) WHERE is_premium = TRUE;
```

### Modified Table: overlay_chat_sources

```sql
-- No schema changes needed, but add new "share" platform to supported_platforms

INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth)
VALUES ('share', 'Shared Overlay', TRUE, FALSE)
ON CONFLICT (platform) DO NOTHING;

-- Config JSONB for share sources:
-- {
--   "source_overlay_id": "uuid",
--   "source_user_id": "uuid",
--   "share_id": "uuid"
-- }
```

## Service Interface Contracts

### share-service API

```go
// Share Management
POST   /api/v1/shares                 // Create share request (requires premium)
GET    /api/v1/shares                 // List shares (filter by status, user)
GET    /api/v1/shares/:id             // Get share details
PUT    /api/v1/shares/:id/accept      // Accept share request (requires premium)
DELETE /api/v1/shares/:id             // Revoke share

// User Search
GET    /api/v1/users/search?platform=twitch&username=xqc  // Search by platform username

// Internal APIs (service-to-service)
GET    /internal/shares/validate/:share_id              // Validate share exists and is active
GET    /internal/shares/resolve/:overlay_id             // Get consuming overlays for source overlay
POST   /internal/shares/expire                          // Bulk expire shares (cron job)
```

### overlay-manager Extensions

```go
// Existing endpoint, modified behavior
POST /api/v1/overlays/:id/sources
// New validation: if platform="share", verify share exists and is active via share-service API

// Request body:
{
  "platform": "share",
  "channel_id": "share-uuid-123",        // Share ID from shares table
  "channel_name": "xQc's chat",
  "config": {
    "source_overlay_id": "overlay-uuid",
    "source_user_id": "user-uuid",
    "share_id": "share-uuid-123"
  }
}
```

### message-processor Extensions

```go
// Internal: Share resolver
type ShareResolver interface {
    GetActiveShares(ctx context.Context, sourceOverlayID string) ([]Share, error)
    InvalidateCache(ctx context.Context, overlayID string) error
}

// Modified publisher
func (p *PubSubPublisher) PublishWithShares(ctx context.Context, msg *models.UnifiedChatMessage) error
```

## Build Order and Dependencies

### Phase 1: Database and Core Share Service (Milestone Foundation)

**Goal:** Share CRUD without message delivery

**Components:**
1. Database migration: `shares` table, `users.is_premium`
2. share-service: CRUD endpoints, premium validation
3. share-service: User search endpoint
4. API Gateway: Route share endpoints

**Dependencies:** None (can build in parallel with other work)

**Validation:** Can create, accept, revoke shares via API

### Phase 2: Overlay Source Integration (Share as Source)

**Goal:** Add shared overlays to overlay configuration

**Components:**
1. overlay-manager: Validate share exists when adding share source
2. Frontend: UI to add share source (select from accepted shares)
3. share-service: Internal validate endpoint

**Dependencies:** Phase 1 complete

**Validation:** Can add share source to overlay, shows in source list

### Phase 3: Message Routing (Actual Chat Delivery)

**Goal:** Messages from shared overlays appear in consuming overlay

**Components:**
1. message-processor: Share resolver (database query, Redis cache)
2. message-processor: Modified publisher (fan-out to consuming overlays)
3. share-service: Cache invalidation events (Redis Pub/Sub)

**Dependencies:** Phase 2 complete

**Validation:** Messages appear in both source and consuming overlay

### Phase 4: Lifecycle and Expiry (Production Ready)

**Goal:** Automatic share expiry based on rules

**Components:**
1. share-service: Stream lifecycle detection (Twitch EventSub integration)
2. share-service: Expiry cron job (5-minute interval)
3. share-service: Manual revocation (mark inactive, invalidate cache)

**Dependencies:** Phase 3 complete

**Validation:** Shares expire when stream ends, manually revoked shares stop delivering messages

### Phase 5: Premium Enforcement (Business Logic)

**Goal:** Enforce premium requirements, admin overrides

**Components:**
1. auth-service: Premium flag management (admin endpoint)
2. share-service: Premium checks on create/accept
3. Frontend: Premium warning UI

**Dependencies:** Phase 1-4 complete

**Validation:** Non-premium users blocked from creating shares, admins can mark users premium

## New vs Modified Components

### New Components (Build from Scratch)

| Component | Lines of Code (Estimate) | Complexity | Notes |
|-----------|--------------------------|------------|-------|
| **share-service** | ~2,000 LOC | Medium | Standard Go service, follows existing patterns |
| **share-service/handlers** | ~500 LOC | Low | CRUD handlers, premium validation |
| **share-service/repository** | ~400 LOC | Low | PostgreSQL queries, standard CRUD |
| **share-service/expiry** | ~300 LOC | Medium | Cron job, stream detection integration |
| **share-service/search** | ~200 LOC | Low | User search by platform username |
| **Database migration** | ~100 LOC | Low | CREATE TABLE shares, ALTER users |

**Total New Code:** ~3,500 LOC

### Modified Components (Extend Existing)

| Component | File | Modification | Complexity |
|-----------|------|--------------|------------|
| **overlay-manager** | `handlers/sources.go` | Add share validation on source create | Low (~50 LOC) |
| **message-processor** | `publisher/pubsub_publisher.go` | Add share resolver, fan-out logic | Medium (~200 LOC) |
| **message-processor** | `share/resolver.go` | NEW FILE: Share resolver with Redis cache | Medium (~300 LOC) |
| **api-gateway** | `cmd/main.go` | Route /api/v1/shares/* to share-service | Low (~20 LOC) |
| **frontend** | Multiple files | Share management UI, source selection | High (~1,000 LOC) |

**Total Modified Code:** ~1,570 LOC

### No Changes Required

- Listener services (Twitch, YouTube, Kick, TikTok) - unchanged
- Emote service - unchanged
- Auth service - minor addition (premium flag endpoints)
- Source manager - works with share sources automatically (platform agnostic)
- Redis Streams/Pub/Sub - no schema changes

## Integration Testing Strategy

### Test 1: End-to-End Share Flow

```
1. User A creates share request for User B
2. User B accepts share request
3. User A adds User B's shared overlay to their overlay
4. Message sent in User B's Twitch chat
5. Message appears in BOTH User B's overlay AND User A's overlay
6. User A revokes share
7. Message sent in User B's Twitch chat
8. Message appears ONLY in User B's overlay (not User A's)
```

### Test 2: Expiry Validation

```
1. User A creates share with expiry_type="this_stream"
2. User B accepts
3. User A's stream goes offline (Twitch EventSub: stream.offline)
4. share-service expires share (status="expired")
5. Message sent in User B's Twitch chat
6. Message appears ONLY in User B's overlay (share expired)
```

### Test 3: Premium Enforcement

```
1. User A (non-premium) attempts to create share request
2. share-service returns 403 Forbidden (premium required)
3. Admin marks User A as premium (is_premium=true)
4. User A creates share request
5. Request succeeds
```

### Test 4: Fan-Out Performance

```
1. Create 10 shares pointing to same source overlay
2. Send 100 messages to source overlay Twitch chat
3. Verify all 100 messages appear in all 11 overlays (source + 10 consumers)
4. Measure latency: <500ms P95 (same as current system)
```

## Sources

Research informed by:
- Existing All-Chat architecture (services/*/README.md, docs/architecture/*)
- [Microservices Pattern: Database per service](https://microservices.io/patterns/data/database-per-service.html)
- [Microsoft: Data Considerations for Microservices](https://learn.microsoft.com/en-us/azure/architecture/microservices/design/data-considerations)
- [8 Essential Example Microservices Architecture Patterns for 2026](https://www.wondermentapps.com/blog/example-microservices-architecture/)
- [Data Management Patterns for Microservices Architecture](https://www.dataversity.net/articles/data-management-patterns-for-microservices-architecture/)
