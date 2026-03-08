# Pitfalls Research: Chat Overlay Sharing

**Domain:** Chat overlay sharing for multi-platform streaming
**Researched:** 2026-03-08
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Client-Side Premium Feature Enforcement

**What goes wrong:**
Premium feature checks enforced only in the UI/frontend allow users to bypass restrictions using browser DevTools, modified API clients, or direct API calls with valid session tokens. Users can access premium sharing features without payment by manipulating client-side controls.

**Why it happens:**
Developers implement feature gating in React components (graying out buttons, hiding UI elements) without corresponding server-side authorization checks. The API endpoint accepts any authenticated request without verifying subscription status.

**How to avoid:**
- **Server-side enforcement**: Every API endpoint that creates/accepts shares MUST check `is_premium` flag from database
- **Double-check pattern**: Both frontend (UX) AND backend (security) must validate premium status
- **Audit trail**: Log all premium feature access attempts (successful and denied) with user ID
- **Test coverage**: Automated tests that attempt to bypass client checks with direct API calls

```go
// WRONG - Frontend only
if (!user.isPremium) {
  return <button disabled>Share Overlay</button>
}

// RIGHT - Backend enforcement
func (h *Handler) CreateShare(c *gin.Context) {
  user := getCurrentUser(c)
  if !user.IsPremium {
    c.JSON(403, gin.H{"error": "Premium feature required"})
    return
  }
  // ... create share
}
```

**Warning signs:**
- API endpoints missing premium checks in handler code
- Only UI components checking `isPremium` flag
- No database queries for subscription status in share creation endpoints
- Test suite doesn't include "authenticated non-premium user tries premium feature"

**Phase to address:**
Phase 1 (Foundation) - Implement premium enforcement at API layer before any sharing logic exists

---

### Pitfall 2: Shared Overlay Message Cascade Without Permission Verification

**What goes wrong:**
When User A shares overlay with User B, and overlay contains shared source from User C, messages from User C's chat flow to User B without User C's consent. This creates unauthorized access chains where permissions don't cascade properly through multiple sharing layers.

**Why it happens:**
Message delivery logic publishes to `overlay:{overlay_id}` without checking the source ancestry. The system assumes "if overlay exists, deliver messages" without validating permission chains. Developers focus on direct share permissions but forget recursive/transitive sharing scenarios.

**How to avoid:**
- **Flatten source ownership**: When accepting share, copy sources as "shared from User A" - don't reference nested overlays
- **Permission verification on message delivery**: Message processor checks each source's active permission state before enrichment
- **Explicit permission model**: Database tracks "User A allows User B to access overlay X" - no implicit cascading
- **Depth limits**: Prohibit sharing overlays that contain shared sources (1-layer depth maximum)

```sql
-- Track direct permissions only
CREATE TABLE overlay_shares (
  id UUID PRIMARY KEY,
  owner_user_id UUID NOT NULL,     -- Who owns the overlay
  recipient_user_id UUID NOT NULL, -- Who can access it
  overlay_id UUID NOT NULL,        -- Which overlay is shared
  is_active BOOLEAN DEFAULT true,
  -- Prevent circular/cascading shares
  CONSTRAINT no_shared_overlay_sharing
    CHECK (overlay_id NOT IN (
      SELECT overlay_id FROM overlay_chat_sources
      WHERE source_type = 'shared_overlay'
    ))
);
```

**Warning signs:**
- Message delivery code has no permission checks
- Database schema allows overlays to contain shared overlays (recursive nesting)
- No explicit test: "User A shares to B, B shares to C, does C see A's messages?"
- Redis Pub/Sub channels don't validate subscriber permissions

**Phase to address:**
Phase 2 (Share Acceptance) - Design share model to prevent cascading before acceptance logic is built

---

### Pitfall 3: Race Condition Between Share Revocation and Message Delivery

**What goes wrong:**
User A revokes share with User B. In the 100-500ms window before User B's WebSocket receives the revocation notification, messages continue flowing from User A's overlay to User B's screen. Worse: if User B disconnects during revocation, they reconnect with stale permissions from replay buffer and see messages they shouldn't.

**Why it happens:**
Revocation updates database (`is_active = false`) but doesn't immediately stop message flow. Message processor publishes to Redis Pub/Sub before checking permission state. WebSocket replay buffer stores messages without permission metadata, so reconnection replays messages from revoked sources.

**How to avoid:**
- **Permission cache invalidation**: When share revoked, publish `revocation:{share_id}` event to Redis Pub/Sub
- **Message processor checks**: Before enrichment, verify share is still active (with Redis cache, 1-second TTL)
- **Replay buffer metadata**: Store `source_share_id` with each message, validate on replay
- **Idempotent revocation**: Client-side immediately stops displaying messages from revoked source (don't wait for server confirmation)

```go
// Message processor checks share validity
func (p *Processor) ProcessMessage(msg RawChatMessage) {
  sources := p.getOverlaySources(msg.OverlayID)
  for _, source := range sources {
    if source.Type == "shared_overlay" {
      // Check Redis cache first (fast path)
      isActive := p.redis.Get(ctx, fmt.Sprintf("share:active:%s", source.ShareID))
      if isActive == "false" {
        continue // Skip revoked share
      }
      // Fallback to database (cache miss)
      if !p.db.IsShareActive(source.ShareID) {
        p.redis.Set(ctx, fmt.Sprintf("share:active:%s", source.ShareID), "false", 60*time.Second)
        continue
      }
    }
  }
}
```

**Warning signs:**
- Message processor doesn't check share status before publishing
- Revocation handler only updates database, doesn't invalidate caches
- Replay buffer implementation has no permission validation
- No test: "Revoke share, send message, verify recipient doesn't see it"

**Phase to address:**
Phase 4 (Revocation) - Implement permission checks before building revocation logic

---

### Pitfall 4: Stream Lifecycle Detection Inconsistency Across Platforms

**What goes wrong:**
"Expire when stream ends" feature works reliably for YouTube (tracked via InnerTube), sporadically for Twitch (IRC doesn't signal offline), and not at all for Kick (no lifecycle API). Users expect consistent behavior but experience platform-dependent expiry, leading to shares that never expire or expire prematurely.

**Why it happens:**
Each platform has different lifecycle signals: YouTube sends explicit stream end events, Twitch requires polling StreamSchedule API or detecting zero viewers, Kick has no official stream status API. Developers implement for one platform and assume others work the same.

**How to avoid:**
- **Fallback expiry**: Always add 24-hour maximum expiry as backup (even if "this stream" selected)
- **Platform capability flags**: Database column `supports_stream_lifecycle` - disable "this stream" option for Kick
- **Polling hybrid**: For Twitch, poll Helix API every 5 minutes to detect offline (expensive but necessary)
- **User communication**: UI clearly shows "Twitch stream end detection may be delayed up to 5 minutes"

```go
// Platform-specific lifecycle strategies
type LifecycleDetector interface {
  SupportsStreamEnd() bool
  CheckStreamStatus(channelID string) (isLive bool, err error)
}

type TwitchDetector struct{}
func (d *TwitchDetector) SupportsStreamEnd() bool { return true }
func (d *TwitchDetector) CheckStreamStatus(channelID string) (bool, error) {
  // Poll Helix API /streams endpoint
  return d.helixClient.IsStreamLive(channelID)
}

type KickDetector struct{}
func (d *KickDetector) SupportsStreamEnd() bool { return false } // No API
func (d *KickDetector) CheckStreamStatus(channelID string) (bool, error) {
  return false, ErrNotSupported
}
```

**Warning signs:**
- Share expiry logic assumes all platforms emit stream end events
- No platform-specific lifecycle handlers
- UI offers "this stream" expiry for all platforms equally
- No 24-hour maximum expiry backstop

**Phase to address:**
Phase 5 (Stream Lifecycle) - Research platform capabilities before implementing expiry

---

### Pitfall 5: Soft Delete Without Cascade Control (Database Constraint Violation)

**What goes wrong:**
When implementing "mark as inactive" instead of hard delete, foreign key constraints with `ON DELETE CASCADE` cause database errors. Attempting to set `is_active = false` fails because child records expect CASCADE DELETE. Alternatively, developers disable foreign keys entirely, losing referential integrity.

**Why it happens:**
Mixing soft-delete patterns with database-level cascade deletes. Developers copy foreign key patterns from platform sources (which use CASCADE) to shared sources without adapting for soft delete semantics.

**How to avoid:**
- **No CASCADE on share tables**: Use `ON DELETE RESTRICT` or `ON DELETE SET NULL` for share relationships
- **Application-level cascade**: When marking share inactive, application code marks dependent records inactive
- **Separate lifecycle**: Share lifecycle is independent of overlay lifecycle (overlay deletion doesn't cascade to shares)
- **Clear separation**: Hard delete for technical resources (sessions, tokens), soft delete for user data (shares, overlays)

```sql
-- WRONG - Cascade delete incompatible with soft delete
CREATE TABLE overlay_shares (
  id UUID PRIMARY KEY,
  overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE, -- PROBLEM
  is_active BOOLEAN DEFAULT true
);

-- RIGHT - No cascade, application handles cleanup
CREATE TABLE overlay_shares (
  id UUID PRIMARY KEY,
  overlay_id UUID REFERENCES overlays(id) ON DELETE RESTRICT,
  is_active BOOLEAN DEFAULT true,
  deleted_at TIMESTAMPTZ NULL
);

-- Application code handles soft delete cascade
func (r *Repository) DeactivateShare(shareID uuid.UUID) error {
  tx := r.db.Begin()
  defer tx.Rollback()

  // Mark share inactive
  tx.Exec("UPDATE overlay_shares SET is_active = false, deleted_at = NOW() WHERE id = ?", shareID)

  // Mark associated source configs inactive
  tx.Exec("UPDATE overlay_chat_sources SET is_active = false WHERE share_id = ?", shareID)

  tx.Commit()
}
```

**Warning signs:**
- Foreign keys use `ON DELETE CASCADE` on tables with `is_active` flags
- Test failures with "foreign key constraint violation" during soft delete
- Inconsistent deletion strategy across similar tables
- No explicit soft delete cascade logic in application code

**Phase to address:**
Phase 1 (Foundation) - Design database schema before implementing any share logic

---

### Pitfall 6: Circular Share Dependency (Bidirectional Resource Loop)

**What goes wrong:**
User A shares overlay X with User B. User B shares overlay Y back to User A. Both overlays now contain each other as sources, creating infinite message loops where the same message circulates endlessly through Redis Pub/Sub channels, causing message duplication storms and Redis memory exhaustion.

**Why it happens:**
Bidirectional sharing is a feature (both users want each other's chat), but without cycle detection, the system allows A→B→A relationships. Message delivery doesn't track message ancestry, so messages circulate without deduplication.

**How to avoid:**
- **Directed acyclic graph (DAG) validation**: Before accepting share, verify it doesn't create cycles
- **Message ID deduplication**: Message processor tracks seen message IDs (Redis set with 60s TTL)
- **Source type separation**: Shared sources deliver messages directly to overlay (not through overlay re-publishing)
- **Depth tracking**: Messages carry `share_depth` metadata (max depth = 1, drop if exceeded)

```go
// Cycle detection before accepting share
func (s *ShareService) AcceptShare(shareID, recipientOverlayID uuid.UUID) error {
  share := s.repo.GetShare(shareID)

  // Check if accepting creates a cycle
  if s.wouldCreateCycle(share.OverlayID, recipientOverlayID) {
    return ErrCyclicShare
  }

  // Safe to accept
  return s.repo.AcceptShare(shareID, recipientOverlayID)
}

func (s *ShareService) wouldCreateCycle(sourceOverlayID, targetOverlayID uuid.UUID) bool {
  // Check if sourceOverlayID already has targetOverlayID as a source (directly or transitively)
  visited := make(map[uuid.UUID]bool)
  return s.hasCycleDFS(sourceOverlayID, targetOverlayID, visited)
}

// Message deduplication
func (p *Processor) ProcessMessage(msg RawChatMessage) {
  dedupKey := fmt.Sprintf("msg:seen:%s", msg.ID)

  // Check if message already processed (Redis SET with 60s TTL)
  exists := p.redis.SetNX(ctx, dedupKey, "1", 60*time.Second).Val()
  if !exists {
    return // Duplicate, skip
  }

  // Process message...
}
```

**Warning signs:**
- No cycle detection logic in share acceptance code
- Message processor doesn't deduplicate by message ID
- Database schema allows unlimited bidirectional relationships
- No test: "User A shares to B, B shares back to A, verify no loops"

**Phase to address:**
Phase 2 (Share Acceptance) - Implement cycle detection before acceptance logic

---

### Pitfall 7: Time-Based Expiry Without UTC Normalization (Timezone Edge Cases)

**What goes wrong:**
User sets "expire in 2 hours" at 11:00 PM local time. Daylight Saving Time ends at midnight, clocks fall back 1 hour. Share expires 3 hours later in wall-clock time (2:00 AM) instead of 1:00 AM, or expires 1 hour early. Time zone conversions introduce ±1 hour errors around DST transitions.

**Why it happens:**
Storing expiry timestamps in user's local timezone or using client-provided timestamps without normalization. Comparison logic uses server's local time instead of UTC. Go's `time.Now()` respects server timezone, creating inconsistencies across Kubernetes pods in different zones.

**How to avoid:**
- **UTC everywhere**: Store all timestamps as `TIMESTAMPTZ` in PostgreSQL, always use `time.Now().UTC()` in Go
- **Duration-based expiry**: Store `expires_at = created_at + INTERVAL '2 hours'` (UTC), not absolute timestamps
- **Client sends durations**: Frontend sends `expiryDuration: 7200` (seconds), not target timestamp
- **Test DST transitions**: Automated tests with `time.Local = time.FixedZone("EST", -5*3600)` and DST boundary timestamps

```go
// WRONG - Uses local time, DST-sensitive
func (s *ShareService) CreateShare(req ShareRequest) error {
  expiresAt := time.Now().Add(time.Duration(req.ExpiryHours) * time.Hour)
  // Problem: time.Now() uses server's local timezone
}

// RIGHT - UTC throughout
func (s *ShareService) CreateShare(req ShareRequest) error {
  createdAt := time.Now().UTC()
  expiresAt := createdAt.Add(time.Duration(req.ExpirySeconds) * time.Second)

  share := &Share{
    CreatedAt: createdAt,
    ExpiresAt: expiresAt, // Both UTC
  }
  return s.repo.Create(share)
}

// Expiry check - always compare UTC
func (s *ShareService) IsExpired(share *Share) bool {
  if share.ExpiresAt == nil {
    return false // Unlimited
  }
  return time.Now().UTC().After(*share.ExpiresAt)
}
```

**Warning signs:**
- Code uses `time.Now()` instead of `time.Now().UTC()`
- Database columns are `TIMESTAMP` instead of `TIMESTAMPTZ`
- Client sends absolute timestamps instead of durations
- No tests covering DST transitions (March/November)

**Phase to address:**
Phase 3 (Expiry Options) - Use UTC from the start, before implementing time-based expiry

---

### Pitfall 8: OAuth Token Revocation Without Share Cascade Cleanup

**What goes wrong:**
User A shares overlay (requires YouTube OAuth token). User A revokes YouTube OAuth token in Google account settings. Share remains active in database, but message delivery fails silently. User B sees no messages but UI shows source as "active", creating confusion.

**Why it happens:**
Share lifecycle is independent of OAuth token lifecycle. Token refresh service handles expiry/refresh, but doesn't notify overlay-manager when tokens are permanently revoked. Listeners fail to fetch messages but don't report failures to overlay-manager.

**How to avoid:**
- **Token validation on share creation**: Verify all required OAuth tokens exist and are valid before accepting share
- **Health check endpoint**: Each listener exposes `/sources/{source_id}/health` endpoint (returns token validity)
- **Periodic validation**: Background job checks share sources every 15 minutes, marks unhealthy sources as `is_active = false`
- **Failure notifications**: Listeners publish `source:failed:{source_id}` events to Redis when token errors occur

```go
// Background job validates share health
func (s *ShareHealthChecker) Run(ctx context.Context) {
  ticker := time.NewTicker(15 * time.Minute)
  defer ticker.Stop()

  for {
    select {
    case <-ctx.Done():
      return
    case <-ticker.C:
      s.checkAllShares(ctx)
    }
  }
}

func (s *ShareHealthChecker) checkAllShares(ctx context.Context) {
  shares := s.repo.GetActiveShares()

  for _, share := range shares {
    sources := s.repo.GetOverlaySources(share.OverlayID)

    for _, source := range sources {
      // Check if listener can access this source
      healthy := s.checkSourceHealth(source)

      if !healthy {
        log.Warn("Share source unhealthy, deactivating",
          zap.String("share_id", share.ID.String()),
          zap.String("source_id", source.ID.String()),
          zap.String("reason", "oauth_token_invalid"))

        s.repo.DeactivateSource(source.ID)
      }
    }
  }
}
```

**Warning signs:**
- No health monitoring for share sources
- Token revocation doesn't trigger share cleanup
- Listeners fail silently without notifying overlay-manager
- No test: "Revoke OAuth token, verify share becomes inactive"

**Phase to address:**
Phase 6 (Health Monitoring) - After basic sharing works, add health validation

---

### Pitfall 9: Replay Buffer State Desynchronization After Share Acceptance

**What goes wrong:**
User A accepts share from User B at 3:15 PM. User A's WebSocket has 50-message replay buffer from 3:10-3:15 PM (doesn't include new source). User A disconnects/reconnects, replay buffer delivers old 50 messages but not messages from newly accepted source. User sees incomplete history.

**Why it happens:**
Replay buffer stores messages per overlay at buffer-fill time. When share accepted, new source is added but buffer isn't backfilled. On reconnection, client receives buffer + live messages, but buffer predates share acceptance.

**How to avoid:**
- **Buffer invalidation**: When source added to overlay, clear that overlay's replay buffer
- **Buffer metadata**: Store `buffer_created_at` timestamp, clients send `last_seen_at`, only replay if `buffer_created_at >= last_seen_at`
- **Source-aware replay**: Buffer stores `source_id` with each message, filter by active sources at replay time
- **Reconnection protocol**: Client sends `last_message_id`, server replays from that point forward (not time-based)

```go
// Invalidate replay buffer when sources change
func (h *OverlayHandler) AcceptShare(c *gin.Context) {
  // ... accept share logic ...

  // Invalidate replay buffer for affected overlays
  h.replayBuffer.Invalidate(recipientOverlayID)

  // Notify connected clients to refetch state
  h.websocketHub.PublishEvent(recipientOverlayID, Event{
    Type: "source_added",
    Data: map[string]interface{}{
      "source_id": newSourceID,
      "source_type": "shared_overlay",
    },
  })
}

// Client reconnection with last seen message ID
type ReconnectRequest struct {
  OverlayID     uuid.UUID `json:"overlay_id"`
  LastMessageID uuid.UUID `json:"last_message_id,omitempty"`
}

func (h *WebSocketHub) HandleReconnect(conn *WebSocket, req ReconnectRequest) {
  if req.LastMessageID != uuid.Nil {
    // Replay from specific message forward
    messages := h.buffer.GetMessagesAfter(req.OverlayID, req.LastMessageID)
    for _, msg := range messages {
      conn.Send(msg)
    }
  } else {
    // Full buffer replay
    messages := h.buffer.GetAll(req.OverlayID)
    for _, msg := range messages {
      conn.Send(msg)
    }
  }
}
```

**Warning signs:**
- Share acceptance doesn't invalidate replay buffers
- Replay logic uses timestamps instead of message IDs
- No test: "Accept share, disconnect, reconnect, verify new source messages appear"
- Buffer stores messages without source metadata

**Phase to address:**
Phase 2 (Share Acceptance) - Design replay buffer invalidation before acceptance

---

### Pitfall 10: Permission State Testing Without Comprehensive Edge Case Coverage

**What goes wrong:**
Tests cover "happy path" (create share, accept, messages flow) but miss edge cases: expired shares, denied shares that weren't cleaned up, shares revoked mid-stream, user deleted but shares persist, circular permission chains. Production incidents occur from untested state transitions.

**Why it happens:**
Developers write tests for intended workflows but don't systematically enumerate state space. Permission systems have combinatorial complexity (pending × active × expired × revoked × denied for each share, multiplied by multiple users). Manual test design misses combinations.

**How to avoid:**
- **State machine testing**: Explicitly model share states, generate tests for all valid transitions
- **Property-based testing**: Use Go's `testing/quick` or `github.com/leanovate/gopter` to generate random state sequences
- **Edge case matrix**: Document all permission states × user actions × timing scenarios, ensure tests cover 100%
- **Negative tests**: For every "should allow" test, write corresponding "should deny" test

```go
// Comprehensive permission state testing
func TestSharePermissionStates(t *testing.T) {
  tests := []struct {
    name           string
    shareState     ShareState
    userAction     string
    expectedResult string
  }{
    // Happy paths
    {name: "active share allows messages", shareState: StateActive, userAction: "view", expectedResult: "allowed"},
    {name: "pending share denies messages", shareState: StatePending, userAction: "view", expectedResult: "denied"},

    // Edge cases
    {name: "expired share denies messages", shareState: StateExpired, userAction: "view", expectedResult: "denied"},
    {name: "revoked share denies messages", shareState: StateRevoked, userAction: "view", expectedResult: "denied"},
    {name: "denied share marked inactive", shareState: StateDenied, userAction: "view", expectedResult: "denied"},
    {name: "active share with deleted owner", shareState: StateActive, userAction: "view_deleted_owner", expectedResult: "denied"},
    {name: "active share with deleted recipient", shareState: StateActive, userAction: "view_deleted_recipient", expectedResult: "denied"},
    {name: "active share with revoked oauth", shareState: StateActive, userAction: "view_revoked_oauth", expectedResult: "denied"},

    // Timing edge cases
    {name: "share expires during message delivery", shareState: StateExpiringNow, userAction: "view", expectedResult: "denied"},
    {name: "share revoked during reconnection", shareState: StateRevokedDuringReconnect, userAction: "reconnect", expectedResult: "denied"},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      // Test implementation
    })
  }
}

// Property-based testing for state transitions
func TestShareStateTransitions(t *testing.T) {
  properties := gopter.NewProperties(nil)

  properties.Property("no state allows access after revocation", prop.ForAll(
    func(initialState ShareState, actions []UserAction) bool {
      share := setupShare(initialState)

      for _, action := range actions {
        applyAction(share, action)
      }

      share.Revoke()

      // After revocation, no action should allow access
      return !share.AllowsAccess()
    },
    gen.AnyShareState(),
    gen.SliceOf(gen.AnyUserAction()),
  ))

  properties.TestingRun(t)
}
```

**Warning signs:**
- Test suite has >80% happy path coverage, <20% edge case coverage
- No state transition diagram documenting valid share states
- Permission bugs found in production that weren't covered by tests
- No property-based or fuzz testing for permission logic

**Phase to address:**
All phases - Add edge case tests for each feature as it's built

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store expiry as duration instead of timestamp | Simpler mental model | Can't display "expires in X minutes" accurately, no audit trail of when expiry was set | Never (always use UTC timestamp) |
| Skip cycle detection "users won't do that" | Faster implementation | Message loops cause production outages, requires database cleanup | Never (cycle detection is 10 lines of code) |
| Check premium status once at share creation | Avoid database query on every message | Users can exploit by subscribing, sharing, then unsubscribing | Only if premium checks are hourly background job |
| Allow unlimited share depth | Don't have to explain limits | Permission chains become unauditable, security risk | Never (enforce 1-layer depth) |
| Store replay buffer in-memory only | No Redis dependency | Pod restarts lose buffers, reconnections fail | Only for MVP testing (migrate to Redis after) |
| Soft delete with CASCADE constraints | Reuse existing foreign key patterns | Database errors on deactivation, requires schema migration | Never (use RESTRICT or SET NULL) |
| Accept shares without OAuth validation | Faster acceptance UX | Shares fail silently when tokens invalid | Only if health monitoring added within same sprint |
| Frontend-only premium checks | Faster development | Security vulnerability, revenue loss | Never (always enforce server-side) |

---

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Redis Pub/Sub for share revocation | Assuming all subscribers receive event instantly | Use cache invalidation + polling fallback (subscribers may miss events) |
| PostgreSQL NOTIFY for source changes | Not handling connection drops (missed notifications) | Combine LISTEN/NOTIFY with periodic polling (every 60s) |
| WebSocket replay buffer | Storing messages without permission metadata | Include `source_id` and `share_id` with each buffered message |
| OAuth token refresh | Not handling permanent revocation (user removes app access) | Listener publishes `oauth:revoked:{source_id}` event, overlay-manager marks share inactive |
| Twitch Helix API stream status | Polling too frequently (rate limit 800 req/min) | Poll every 5 minutes per channel, cache results in Redis (60s TTL) |
| YouTube InnerTube stream end | Assuming immediate offline detection | Stream end may be delayed 30-60s, use 2-minute grace period before expiry |
| Message processor enrichment | Processing revoked share messages | Check share status BEFORE enrichment (not after Redis publish) |
| Kubernetes pod DNS resolution | Hardcoding service URLs (`youtube-listener:8086`) | Use environment variables for service discovery, supports local dev + k8s |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| N+1 queries checking share status per message | Message processor CPU at 80%, latency >1s | Use Redis cache for share status (1s TTL), batch database queries | >100 messages/sec per overlay |
| No pagination on share list endpoint | `/shares` endpoint timeout after 30s | Paginate (50 shares per page), add `created_at` index | >500 shares per user |
| Polling every platform for stream status | Listener pod CPU 60%, rate limit errors | Only poll platforms that support lifecycle detection (skip Kick), 5-minute intervals | >200 active shares with "this stream" expiry |
| Replay buffer stores full message JSON | Redis memory 12GB, eviction warnings | Store message IDs only, fetch full messages from PostgreSQL on replay | >50,000 concurrent WebSocket connections |
| Checking permission on every message delivery | Message processor throughput 500 msg/sec | Cache share permissions in Redis (60s TTL), invalidate on revocation | >1,000 messages/sec aggregate |
| Linear search for cycle detection | Share acceptance timeout >10s | Use adjacency list + DFS with visited set (O(V+E) instead of O(V²)) | >100 shares per user |
| Broadcasting share creation to all users | WebSocket hub CPU 90%, fan-out 10,000 connections | Only broadcast to affected users (share participants), not global | >5,000 concurrent users |
| Deduplication using database queries | Message processor latency >2s per message | Use Redis SET with 60s TTL (`msg:seen:{msg_id}`), O(1) lookups | >500 messages/sec |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Not validating `recipient_overlay_id` ownership on share acceptance | User A accepts share into User B's overlay (unauthorized access) | Verify `SELECT user_id FROM overlays WHERE id = ?` matches authenticated user |
| Exposing all overlay details in share search results | User searches for streamer, sees private overlay names/configs | Return only public profile info (username, avatar), not overlay metadata |
| Allowing share acceptance without explicit user action | Automatic acceptance of shares creates unauthorized access | Require explicit "Accept" button click, log acceptance with IP/timestamp |
| No rate limiting on share creation | User spams 1,000 share requests, DoS attack on recipients | Rate limit: 10 shares per hour per user, 50 pending shares maximum |
| Storing `is_premium` flag only in JWT | User gets premium JWT, cancels subscription, JWT still valid for 24 hours | Always check database `users.is_premium` on premium actions, don't trust JWT |
| Broadcasting revocation reasons in WebSocket events | User B learns User A revoked because "inappropriate content" (privacy leak) | Send generic "share_revoked" event without reason to recipient |
| Not sanitizing overlay names in share notifications | XSS via overlay name in email notification "User accepted share of <script>..." | Sanitize all user-generated content in notifications, use plain text emails |
| Allowing infinite share renewals | User creates "1 hour" share, renews every 55 minutes to bypass premium limits | Limit renewals to 3 per share, require new share creation after |

---

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Not showing why share is inactive | User sees source grayed out, no explanation (expired? revoked? denied?) | Display status badge: "Expired 2 hours ago", "Revoked by owner", "OAuth token invalid" |
| Allowing "this stream" expiry for Kick | User selects "this stream", share never expires (Kick has no lifecycle API) | Disable option with tooltip: "Kick doesn't support stream end detection, use time-based expiry" |
| No confirmation before revoking share | User accidentally clicks "Revoke", other streamer's overlay breaks mid-stream | Confirmation modal: "This will immediately stop sharing with @username. Continue?" |
| Sharing overlay by ID instead of search | User needs to ask recipient "what's your overlay ID?" | Search by Twitch/YouTube username, select from results, system resolves overlay |
| Not showing expiry countdown | User wonders "how long until this expires?" | Display countdown: "Expires in 1h 23m" with auto-update every minute |
| Accepting share without preview | User accepts, sees 10,000 messages/min spam overlay | Show preview: "This share will add ~X messages/min from Y sources" |
| No notification when share expires | User's overlay suddenly missing chat source mid-stream, no explanation | Browser notification + WebSocket event: "Share from @username expired" |
| Hiding pending shares in separate tab | User forgets about pending requests, other streamer waits indefinitely | Show badge with count on main dashboard: "Shares (3 pending)" |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Share creation**: Often missing server-side premium check — verify API endpoint returns 403 for non-premium users
- [ ] **Share acceptance**: Often missing cycle detection — verify A shares to B, B shares back to A, second share rejected
- [ ] **Time-based expiry**: Often missing UTC normalization — verify tests include DST transition boundaries
- [ ] **Stream end expiry**: Often missing platform capability check — verify Kick disables option, Twitch polls API
- [ ] **Revocation**: Often missing cache invalidation — verify messages stop flowing within 1 second of revocation
- [ ] **Message delivery**: Often missing permission verification — verify revoked shares don't deliver messages
- [ ] **OAuth integration**: Often missing token revocation handling — verify share deactivated when OAuth revoked
- [ ] **Replay buffer**: Often missing permission filtering — verify disconnected user doesn't see revoked share messages on reconnect
- [ ] **Database schema**: Often missing soft delete without CASCADE — verify foreign keys use RESTRICT or SET NULL
- [ ] **WebSocket events**: Often missing revocation broadcast — verify all connected clients receive share_revoked event
- [ ] **Rate limiting**: Often missing share creation limits — verify >10 shares/hour returns 429 Too Many Requests
- [ ] **Search functionality**: Often missing ownership validation — verify user can't accept share into someone else's overlay

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Client-side premium bypass discovered | MEDIUM | 1. Deploy server-side enforcement hotfix, 2. Audit logs for unauthorized shares (query `overlay_shares WHERE created_by IN (SELECT id FROM users WHERE is_premium = false)`), 3. Mark unauthorized shares inactive, 4. Email affected users (premium feature violation) |
| Message cascade permission leak | HIGH | 1. Disable share acceptance immediately (feature flag), 2. Audit share chains (recursive query), 3. Delete nested shares, 4. Redeploy with 1-layer depth limit, 5. Re-enable acceptance |
| Race condition after revocation | LOW | 1. Deploy cache invalidation fix, 2. No data cleanup needed (messages already delivered), 3. Monitor metrics for reduced post-revocation message delivery |
| Stream lifecycle inconsistency | LOW | 1. Add 24-hour backstop expiry to all existing shares (database UPDATE), 2. UI update to show platform capabilities, 3. No immediate fix needed (cosmetic issue) |
| Soft delete CASCADE violation | MEDIUM | 1. Disable share creation (feature flag), 2. Migrate schema (ALTER TABLE... DROP CONSTRAINT, ADD CONSTRAINT ON DELETE RESTRICT), 3. Deploy application-level cascade logic, 4. Re-enable creation |
| Circular share dependency | HIGH | 1. Identify cycles (graph query), 2. Break cycles (deactivate newer share in each cycle), 3. Deploy cycle detection, 4. Message deduplication for existing cycles (prevent loops while fixing) |
| Timezone DST error | LOW | 1. Migrate timestamps to UTC (UPDATE overlay_shares SET expires_at = expires_at AT TIME ZONE 'UTC'), 2. Deploy UTC enforcement, 3. No user impact (transparent fix) |
| OAuth revocation undetected | MEDIUM | 1. Run health check job immediately (mark unhealthy shares inactive), 2. Deploy periodic health monitoring, 3. Email affected users (share deactivated due to OAuth issue) |
| Replay buffer desync | LOW | 1. Invalidate all replay buffers (Redis FLUSHDB on buffer keyspace), 2. Deploy buffer invalidation logic, 3. Connected clients reconnect automatically |
| Permission state testing gaps | HIGH | 1. Incident post-mortem (identify untested edge case), 2. Write regression test, 3. Generate full state matrix, 4. Add property-based tests, 5. Run fuzz testing for 24 hours |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Client-side premium bypass | Phase 1 (Foundation) | Automated test: Non-premium user curl POST /shares → 403 Forbidden |
| Message cascade permission leak | Phase 2 (Share Acceptance) | Integration test: A→B→C chain rejected, only A→B allowed |
| Race condition after revocation | Phase 4 (Revocation) | Timing test: Revoke share, send message 100ms later, verify not delivered |
| Stream lifecycle inconsistency | Phase 5 (Stream Lifecycle) | Platform tests: Twitch stream ends → share expires, Kick option disabled |
| Soft delete CASCADE violation | Phase 1 (Foundation) | Schema validation: `pg_constraint` query verifies RESTRICT not CASCADE |
| Circular share dependency | Phase 2 (Share Acceptance) | Graph test: Create A→B, attempt B→A, verify rejection with cycle error |
| Timezone DST error | Phase 3 (Expiry Options) | DST boundary test: Set 2hr expiry at 11PM before DST end, verify expires at 1AM UTC |
| OAuth revocation undetected | Phase 6 (Health Monitoring) | OAuth test: Revoke token via Google, run health check, verify share inactive |
| Replay buffer desync | Phase 2 (Share Acceptance) | Reconnect test: Accept share, disconnect, reconnect, verify new source messages in buffer |
| Permission state testing gaps | All phases | Coverage report: Permission edge cases >90% test coverage |

---

## Sources

### Premium Feature Enforcement
- [Subscription Bypass Leading to Full Access to Paid Features](https://medium.com/@hossam_hamada/subscription-bypass-leading-to-full-access-to-paid-features-7c3a1bf6487c) - Business logic flaws in subscription enforcement
- [Bypassing Paywalls: How Subscription Fraud is Costing Businesses Millions](https://bureau.id/resources/blog/bypassing-paywalls-how-subscription-fraud-is-costing-businesses-millions) - Financial impact of bypass vulnerabilities
- [Pentest Findings: Bypassing Freemium with client-side security controls](https://onsecurity.io/article/pentest-findings-bypassing-freemium-through-client-side-security-controls/) - Client-side vs server-side enforcement patterns
- [Paywall Bypass: How Client-Side Trust Led to a Free Premium Upgrade](https://medium.com/@default_Ox/paywall-bypass-how-client-side-trust-led-to-a-free-premium-upgrade-f54e65699628) - Real-world bypass exploitation

### Permission Systems and Revocation
- [CVE-2026-29061: Gokapi privilege escalation via incomplete API-key permission revocation](https://advisories.gitlab.com/pkg/golang/github.com/forceu/gokapi/CVE-2026-29061/) - Permission revocation edge case (2026)
- [OAuth 2.0 Token Revocation (RFC 7009)](https://datatracker.ietf.org/doc/html/rfc7009) - Token revocation cascade behavior
- [Security Consideration: Delegation Chain Splicing in RFC 8693 Token Exchange](http://www.mail-archive.com/oauth@ietf.org/msg25725.html) - Delegation chain edge cases (Feb 2026)
- [How OAuth 2.0 Token Revocation Works & Why It Matters](https://curity.io/resources/learn/oauth-revoke/) - Cascade revocation patterns

### Database Patterns
- [Cascade Delete - EF Core](https://learn.microsoft.com/en-us/ef/core/saving/cascade-delete) - Cascade delete fundamentals
- [Foreign Keys vs Performance (Part 3): The CASCADE DELETE Story](https://medium.com/@thyagodoliveiraperez/foreign-keys-vs-performance-part-3-the-cascade-delete-story-aac5cabd843b) - Soft delete performance (2026 benchmarks)
- [Soft delete on delete cascade foreign key · Issue #26](https://github.com/yii2tech/ar-softdelete/issues/26) - Cascade conflicts with soft delete
- [Circular dependency detected on resource](https://learn.microsoft.com/en-us/answers/questions/1095117/circular-dependency-detected-on-resource) - Circular dependency resolution
- [Handling Circular Reference of JPA Bidirectional Relationships](https://hellokoding.com/handling-circular-reference-of-jpa-hibernate-bidirectional-entity-relationships-with-jackson-jsonignoreproperties/) - Bidirectional relationship patterns

### Real-Time Systems and WebSockets
- [Handling Race Conditions in Real-Time Apps](https://dev.to/mattlewandowski93/handling-race-conditions-in-real-time-apps-49c8) - Event cache pattern for race conditions (2026)
- [WebSockets: The Complete Guide for 2026](https://devtoolbox.dedyn.io/blog/websocket-complete-guide) - Sub-10ms latency patterns
- [Message delivery race condition causes gateway timeouts](https://github.com/openclaw/openclaw/issues/9471) - Message delivery race conditions (Feb 2026)
- [How to Implement Reconnection Logic for WebSockets](https://oneuptime.com/blog/post/2026-01-27-websocket-reconnection-logic/view) - Reconnection patterns (2026)
- [Day 13: The Replay Buffer](https://javatsc.substack.com/p/day-13-the-replay-buffer-engineering) - Per-user replay buffer architecture (2026)

### Stream Lifecycle and Platform APIs
- [Twitch Product Lifecycle](https://dev.twitch.tv/docs/product-lifecycle/) - Twitch API lifecycle
- [How to Download Kick Streams in 2026](https://streamrecorder.io/blog/how-to-download-kick-streams) - Kick VOD retention issues
- [Twitch vs YouTube vs Kick vs TikTok: Streaming Platform Comparison 2026](https://www.streamups.com/blog/platform-comparison-2026) - Platform capability comparison
- [Twitch Viewer Bot and Kick Viewbot Guide 2026](https://www.theviewbot.com/blog/twitch-kick-viewbot-guide/) - Detection system challenges

### Time Zones and Temporal Edge Cases
- [Daylight Saving Time Changes 2026 in UTC](https://www.timeanddate.com/time/change/timezone/utc) - UTC has no DST changes
- [Date and Time Testing Across Multiple Time Zones](https://www.thegreenreport.blog/articles/date-and-time-testing-across-multiple-time-zones/date-and-time-testing-across-multiple-time-zones.html) - DST testing strategies
- [Handling Local Timezones, UTC, Daylight Savings Time, and Leap Units](https://support.safe.com/hc/en-us/articles/25407590058765-Handling-Local-Timezones-UTC-Daylight-Savings-Time-and-Leap-Units) - UTC best practices

### Security and API Enforcement
- [Professional API Security Best Practices in 2026](https://www.trustedaccounts.org/blog/post/professional-api-security-best-practices) - OWASP API Security Top 10 alignment
- [Server side enforcement — OWASP AASVS](https://owasp-aasvs.readthedocs.io/en/latest/requirement-2.4.html) - Server-side validation requirements
- [CWE-602: Client-Side Enforcement of Server-Side Security](https://cwe.mitre.org/data/definitions/602.html) - Client-side enforcement anti-pattern
- [Authorization - OWASP Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html) - Authorization edge cases

### Testing and Validation
- [Testing Runtime Permissions: Lessons Learned](https://proandroiddev.com/testing-runtime-permissions-lessons-learned-17642a4c5652) - Permission state testing patterns
- [Testing Login & Authentication Flows, Edge Cases People Forget](https://www.frugaltesting.com/blog/testing-login-authentication-flows-edge-cases-people-forget) - Authentication edge cases
- [Edge Cases in Software Development: Guide to Testing](https://testomat.io/blog/edge-cases-in-software-development/) - Systematic edge case enumeration

---

*Pitfalls research for: Chat overlay sharing for streaming platforms*
*Researched: 2026-03-08*
*Confidence: HIGH - Based on production security research (2026), real-world platform comparisons, and All-Chat architecture analysis*
