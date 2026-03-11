# Phase 19: Lifecycle & Expiry - Research

**Researched:** 2026-03-11
**Domain:** Share expiry logic, stream lifecycle detection (Twitch Helix, YouTube/TikTok reuse, Kick investigation)
**Confidence:** HIGH (core patterns), MEDIUM (Kick), LOW (Kick webhook availability in local dev)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EXPIRY-01 | User can choose expiry option when accepting: "This stream", "n hours", "Unlimited" | UI already exists (AcceptModal.tsx); backend receives expiry_option but does not persist it to DB — needs migration + persistence |
| EXPIRY-02 | Stream lifecycle detected for Twitch via Helix API | Twitch EventSub `stream.online` / `stream.offline` events need to be subscribed in twitch-eventsub-listener; no subscription_manager method exists for these types yet |
| EXPIRY-03 | Share auto-expires when either user's stream ends (if "This stream" selected) | ExpiryJob pattern exists in share-service; needs new method to expire `this_stream` shares when a stream-end event fires |
| EXPIRY-04 | Time-based expiry checked via background job every 5 minutes | ExpiryJob already exists and runs every 5 minutes — needs extension to handle accepted shares with time-based expiry (currently only handles pending request expiry) |
| EXPIRY-05 | YouTube and TikTok lifecycle already tracked (reuse existing detection) | YouTube `streams/manager.go` detects offline via poller; TikTok `status-checker.ts` detects offline — both need to publish to a Redis channel that share-service subscribes to |
| EXPIRY-06 | Kick stream lifecycle detection researched (defer implementation if complex) | Kick has webhook `stream.online` / `stream.offline` events in official API (added 2025-04); implementation via webhook endpoint is possible but requires kick-listener refactor |
</phase_requirements>

---

## Summary

Phase 19 implements the lifecycle side of the share expiry system. Three mechanisms are needed: (1) persist the expiry option chosen at acceptance time, (2) expire time-based shares via background job, and (3) expire stream-scoped shares when a stream ends. The existing `ExpiryJob` in share-service handles pending-request expiry but does not yet handle *accepted-share* expiry — both a schema change and new job logic are required.

The most significant new piece is Twitch stream lifecycle detection. The project already has `twitch-eventsub-listener` with a `SubscriptionManager` that handles chat events. Adding `stream.online` / `stream.offline` EventSub subscriptions requires no OAuth scopes — only an app access token — making them the simplest event types to add. The webhook handler already dispatches on subscription type; a new case emits a Redis pub/sub lifecycle event that the share-service subscribes to.

YouTube and TikTok already detect stream end internally; they just do not publish a standardised lifecycle event that other services can consume. A lightweight Redis Pub/Sub channel (`lifecycle:stream_end`) needs to be added to those pollers' offline handling. Kick has an official webhook API with `stream.online`/`stream.offline` (added April 2025) but the kick-listener is Pusher-WebSocket only; adding Kick webhook support is non-trivial and should be researched further with a graceful disable path.

**Primary recommendation:** Persist `expiry_option` + `expiry_at` on `share_requests`, extend ExpiryJob for accepted shares, add Twitch EventSub stream.online/offline, publish Redis `lifecycle:stream_end` events from YouTube/TikTok pollers, and gracefully disable "This stream" expiry for Kick.

---

## Standard Stack

### Core (all from existing services)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| pgx/v5 | existing | DB updates in share-service | Already used everywhere |
| go-redis/v9 | existing | Redis Pub/Sub for lifecycle events | Already used in all Go services |
| gin | existing | HTTP routing | Already used in all Go services |
| go.uber.org/zap | existing | Structured logging | Already used everywhere |
| time.Ticker (stdlib) | existing | Background expiry job | ExpiryJob already uses this pattern |

No new dependencies required.

---

## Architecture Patterns

### Recommended New Components

```
share-service/
├── jobs/
│   ├── expiry.go                  # EXTEND: add ExpireTimedShares + lifecycle trigger
│   └── lifecycle_subscriber.go   # NEW: Redis subscriber for stream_end events

services/twitch-eventsub-listener/
├── eventsub/
│   └── subscription_manager.go   # EXTEND: SubscribeToStreamOnline/Offline methods
└── webhooks/
    └── handler.go                 # EXTEND: handle stream.online / stream.offline notifications

services/youtube-listener/
└── streams/
    └── poller.go (or manager.go)  # EXTEND: publish lifecycle event on stream end

services/tiktok-listener/
└── src/livestream/
    └── status-checker.ts          # EXTEND: publish lifecycle event on offline detection

migrations/
└── 034_share_expiry_fields.sql   # NEW: add expiry_option + expiry_at columns
```

### Pattern 1: Schema — Persist Expiry Option

The `share_requests` table needs two new columns:
- `expiry_option VARCHAR(20)` — values: `this_stream`, `custom`, `unlimited`
- `share_expires_at TIMESTAMP NULL` — populated for `custom` option; NULL for `unlimited` and `this_stream`

The existing `expires_at` column is **only for pending request expiry** (7-day acceptance window). A separate column is needed for the *active share's* expiry after acceptance.

```sql
-- Migration 034
ALTER TABLE share_requests
  ADD COLUMN expiry_option VARCHAR(20) DEFAULT 'unlimited',
  ADD COLUMN share_expires_at TIMESTAMP NULL;

CREATE INDEX idx_share_requests_share_expires
  ON share_requests(share_expires_at, status)
  WHERE status = 'accepted' AND share_expires_at IS NOT NULL;
```

The accept handler (`AcceptShareRequest` in `handlers/shares.go`) already receives `expiry_option` and `expiry_hours` from the request body. It persists `recipient_overlay_id` but silently discards the expiry values. The UPDATE query must be extended to set `expiry_option` and `share_expires_at = NOW() + INTERVAL 'N hours'` for custom.

### Pattern 2: Extend ExpiryJob for Accepted Shares (EXPIRY-04)

The existing ExpiryJob calls `ExpirePendingRequests` — a single query that sets `status = 'expired'` for pending requests past `expires_at`. A second method is needed: `ExpireTimedAcceptedShares` that targets accepted shares where `share_expires_at < NOW()`.

```go
// In repository/share_repo.go
func (r *ShareRepository) ExpireTimedAcceptedShares(ctx context.Context) (int, error) {
    query := `
        UPDATE share_requests
        SET status = $1, responded_at = NOW()
        WHERE status = $2
          AND expiry_option = 'custom'
          AND share_expires_at IS NOT NULL
          AND share_expires_at < NOW()
    `
    result, err := r.db.Exec(ctx, query, models.StatusExpired, models.StatusAccepted)
    // ... deactivate overlay_chat_sources atomically
}
```

Deactivating `overlay_chat_sources` must be done in the same transaction as the status update — follow the exact pattern from `RevokeShareRequest` which does `UPDATE overlay_chat_sources SET is_active = false WHERE channel_id = $1 AND platform = 'shared_overlay'`.

The ExpiryJob `expireOldRequests` method should call both in sequence.

### Pattern 3: Stream Lifecycle Redis Channel

**Design:** All listener services publish to a single Redis Pub/Sub channel: `lifecycle:stream_end`

**Message format:**
```json
{
  "platform": "twitch",
  "user_id": "<all-chat user UUID>",
  "broadcaster_id": "<platform-specific ID>",
  "timestamp": "2026-03-11T12:00:00Z"
}
```

The `user_id` (all-chat UUID) is needed for the share-service to query which accepted shares to expire. The `broadcaster_id` is the platform-specific ID (Twitch broadcaster ID, YouTube channel ID, TikTok username).

**Lookup pattern:** The share-service lifecycle subscriber receives the event, then queries:
```sql
SELECT id FROM share_requests
WHERE status = 'accepted'
  AND expiry_option = 'this_stream'
  AND (sender_user_id = $1 OR recipient_user_id = $1)
```
Then expires each found share (set status=expired, deactivate overlay sources).

### Pattern 4: Twitch EventSub stream.online / stream.offline (EXPIRY-02)

**Confirmed from official docs:** `stream.online` and `stream.offline` require NO OAuth scopes. Only an app access token (`client_credentials`) is needed — the same token the `SubscriptionManager` already obtains.

**What to add in `subscription_manager.go`:**
```go
// No user token needed - app token suffices
func (sm *SubscriptionManager) SubscribeToStreamOnline(ctx context.Context, broadcasterID string) (string, error) {
    return sm.subscribeWithAppToken(ctx, "stream.online", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToStreamOffline(ctx context.Context, broadcasterID string) (string, error) {
    return sm.subscribeWithAppToken(ctx, "stream.offline", broadcasterID, "1")
}
```

**Payload for `stream.offline`** (confirmed from Twitch EventSub docs):
- `broadcaster_user_id` — Twitch broadcaster ID

**Challenge:** The share-service needs the all-chat `user_id` to match against `share_requests`. The webhook handler in `twitch-eventsub-listener` must look up the user by `twitch_id` (users table column), then publish the lifecycle event with the all-chat UUID.

**Where to wire it:** In `channels/manager.go`, where the subscription callback already calls `subscriptionMgr.SubscribeChannelPoints(...)` etc. The `SetSubscriptionCallback` invocation in `cmd/main.go` adds `stream.online` and `stream.offline` to the subscribe block.

**Webhook handler dispatch:** In `webhooks/handler.go`, `handleNotification` dispatches on subscription type. A new case handles `stream.offline` and publishes to Redis `lifecycle:stream_end`.

### Pattern 5: YouTube Lifecycle Reuse (EXPIRY-05)

YouTube's `streams/poller.go` detects stream end via `DetectOffline()` and calls `HandleStreamOffline()`. That function currently only clears the Redis video mapping. It needs to also publish to `lifecycle:stream_end`.

The `Manager` in `streams/manager.go` already has `statusPublisher *status.Publisher`. A second publisher or an extended publisher can handle lifecycle events. Alternatively, add a direct `redis.Publish` call in the offline handler with the channel ID.

**Lookup challenge for YouTube:** The `broadcaster_id` in the lifecycle event will be the YouTube `channel_id`. The share-service must JOIN `users(google_id)` to get the all-chat `user_id`. The `users` table has `google_id` column.

### Pattern 6: TikTok Lifecycle Reuse (EXPIRY-05)

TikTok is a Node.js service. The `TikTokStatusChecker.checkLiveStatus()` returning `isLive: false` triggers the offline path in the poller. The `poller.ts` (or `pool-manager.ts`) needs to publish to Redis `lifecycle:stream_end` when a connected stream goes offline.

**Lookup challenge for TikTok:** TikTok users are identified by username in the listeners but the all-chat `users` table has `tiktok_id`. A lookup via `tiktok_id` or `username` is needed.

### Anti-Patterns to Avoid

- **Do NOT expire immediately on stream.offline receipt:** Add a 60-second debounce timer before expiring shares. Short interruptions (sub-broadcast restarts) can cause false stream.offline events on Twitch.
- **Do NOT forget to deactivate overlay_chat_sources** when expiring a share. Revocation does this atomically; expiry must do the same.
- **Do NOT run expiry AND lifecycle triggers from two sources simultaneously** — use the lifecycle subscriber as the authoritative path for `this_stream` expiry; the 5-minute job only handles `custom` time-based expiry.
- **Do NOT reuse `expires_at` for share-active-expiry** — that column is the pending-request acceptance window. Use the new `share_expires_at` column to avoid confusion.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Stream end detection (Twitch) | Custom IRC disconnect monitoring | EventSub `stream.offline` webhook | IRC disconnects ≠ stream end; EventSub is the authoritative signal |
| Stream end detection (YouTube) | New detection loop | Existing `HandleStreamOffline()` path in poller | Already detects offline via continuation API; just add publish step |
| Stream end detection (TikTok) | New connectivity probe | Existing `checkLiveStatus()` returning `isLive: false` | Already polls at 5-15s intervals; just add publish step |
| Transactional expiry + deactivate | Two separate DB calls | Single pgx transaction | Race condition: share marked expired but sources still active |

---

## Common Pitfalls

### Pitfall 1: expiry_option Not Persisted
**What goes wrong:** `AcceptShareRequest` in `handlers/shares.go` already receives and validates `expiry_option` and `expiry_hours` but the UPDATE query only sets `status`, `responded_at`, and `recipient_overlay_id`. The expiry choice is lost after acceptance — this is a guaranteed Phase 19 bug.
**Why it happens:** Phase 15 implemented the accept flow before Phase 19's expiry persistence was needed.
**How to avoid:** The migration (034) adding `expiry_option` + `share_expires_at` must be Wave 0. The accept handler UPDATE query must be extended in Wave 1.

### Pitfall 2: stream.offline False Positives
**What goes wrong:** Twitch briefly sends `stream.offline` during category changes, network flaps, or sub-broadcasts. Expiring shares immediately on receipt will cause phantom expiries.
**Why it happens:** EventSub stream.offline fires on any broadcast end, including temporary ones.
**How to avoid:** Debounce the expiry: after receiving `stream.offline`, wait 60 seconds, then re-check if the share is still marked `this_stream` and the stream is still offline before expiring. A simple `time.AfterFunc` pattern in the lifecycle subscriber suffices for MVP.

### Pitfall 3: Missing Broadcaster-ID to User-ID Mapping
**What goes wrong:** The lifecycle event carries a platform broadcaster ID (Twitch: `broadcaster_user_id`, YouTube: `channel_id`). The share-service stores all-chat `user_id` UUIDs, not platform IDs. Without a mapping, the subscriber cannot know which user's shares to expire.
**Why it happens:** The share-service works entirely with all-chat UUIDs; platform IDs live in the `users` table.
**How to avoid:** The lifecycle event publisher (twitch-eventsub-listener webhook handler) must do the user lookup (via `SELECT id FROM users WHERE twitch_id = $1`) and include `user_id` in the event payload. The share-service subscriber then queries `share_requests` using this UUID.

### Pitfall 4: Expiry Skips overlay_chat_sources Deactivation
**What goes wrong:** Setting `share_requests.status = 'expired'` without deactivating `overlay_chat_sources` means revoked sources keep delivering messages. Frontend and backend will show inconsistent state.
**Why it happens:** Easy to forget in the expiry job — the revocation handler makes this transactional but the expiry job was written before revocation existed.
**How to avoid:** Both `ExpireTimedAcceptedShares` and the stream-end expiry path must deactivate `overlay_chat_sources WHERE channel_id = share_id AND platform = 'shared_overlay'` in the same transaction.

### Pitfall 5: Kick Lifecycle Complexity
**What goes wrong:** Kick does not have an IRC/IRC-adjacent protocol for stream lifecycle. The existing kick-listener uses Pusher WebSocket for chat only. Adding webhook support requires: a public HTTPS endpoint, webhook registration in the Kick developer portal, signature verification, and a new HTTP server in kick-listener (or a new service).
**Why it happens:** Kick's official webhook API (stream.online, stream.offline) was only added April 2025 and requires a completely different ingestion path than the Pusher chat WebSocket.
**How to avoid:** Defer Kick `this_stream` expiry. The requirements explicitly allow this: EXPIRY-06 says "defer implementation if complex." Use a graceful disable: if user's stream platform is Kick, show a UI note that "This stream" expiry is not yet supported for Kick and default to "Unlimited" or disable the option.

### Pitfall 6: ExpiryJob Transaction Gap for Accepted Shares
**What goes wrong:** The existing `ExpirePendingRequests` does a bare `UPDATE ... WHERE status = 'pending'`. For accepted shares, a two-step update (share_requests + overlay_chat_sources) requires a transaction. A non-transactional approach risks partial updates.
**How to avoid:** The new `ExpireTimedAcceptedShares` method should use `pgxpool.BeginTx` and batch-update in one transaction, following the `RevokeShareRequest` pattern exactly.

---

## Code Examples

### Extend UpdateStatus to Support 'expired' for Accepted Shares

The existing `UpdateStatus` method in `share_repo.go` only allows `accepted`, `rejected`, `expired`. Expiring an accepted share is a different operation because it must also deactivate `overlay_chat_sources`. Use a dedicated method, not `UpdateStatus`.

```go
// Source: pattern from handlers/shares.go RevokeShareRequest
func (r *ShareRepository) ExpireAcceptedShare(ctx context.Context, shareID string) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to start transaction: %w", err)
    }
    defer tx.Rollback(ctx)

    _, err = tx.Exec(ctx,
        `UPDATE share_requests SET status = 'expired', responded_at = NOW() WHERE id = $1 AND status = 'accepted'`,
        shareID)
    if err != nil {
        return fmt.Errorf("failed to expire share: %w", err)
    }

    _, err = tx.Exec(ctx,
        `UPDATE overlay_chat_sources SET is_active = false
         WHERE channel_id = $1 AND platform = 'shared_overlay'`,
        shareID)
    if err != nil {
        return fmt.Errorf("failed to deactivate sources: %w", err)
    }

    return tx.Commit(ctx)
}
```

### Twitch EventSub stream.offline — no scope required

```go
// Source: Twitch EventSub docs (stream.offline, no auth scope)
// Condition: { "broadcaster_user_id": "<id>" }
// Token: app access token (client_credentials) — same as SubscribeChannelPoints
func (sm *SubscriptionManager) SubscribeToStreamOffline(ctx context.Context, broadcasterID string) (string, error) {
    token, err := sm.getAccessToken(ctx) // Already exists, uses client_credentials
    if err != nil {
        return "", fmt.Errorf("failed to get access token: %w", err)
    }
    cacheKey := broadcasterID + ":stream.offline"
    condition := map[string]string{"broadcaster_user_id": broadcasterID}
    return sm.subscribeWithCondition(ctx, "stream.offline", broadcasterID, token, "1", condition, cacheKey)
}
```

### Lifecycle Event Payload

```go
// To be published on Redis channel "lifecycle:stream_end"
type StreamEndEvent struct {
    Platform      string    `json:"platform"`        // "twitch", "youtube", "tiktok"
    UserID        string    `json:"user_id"`         // all-chat user UUID
    BroadcasterID string    `json:"broadcaster_id"`  // platform-specific ID
    Timestamp     time.Time `json:"timestamp"`
}
```

### Webhook Handler: stream.offline Dispatch

```go
// In webhooks/handler.go handleNotification, add case:
case "stream.offline":
    h.handleStreamOffline(c, payload)
```

```go
func (h *Handler) handleStreamOffline(c *gin.Context, payload WebhookPayload) {
    broadcasterID, _ := payload.Event["broadcaster_user_id"].(string)
    if broadcasterID == "" {
        return
    }
    // Look up all-chat user_id from users table via twitch_id
    var userID string
    err := h.db.QueryRow(c.Request.Context(),
        "SELECT id FROM users WHERE twitch_id = $1", broadcasterID).Scan(&userID)
    if err != nil {
        h.logger.Warn("No user found for broadcaster", zap.String("broadcaster_id", broadcasterID))
        return
    }
    event := StreamEndEvent{
        Platform: "twitch", UserID: userID,
        BroadcasterID: broadcasterID, Timestamp: time.Now(),
    }
    data, _ := json.Marshal(event)
    h.redis.Publish(c.Request.Context(), "lifecycle:stream_end", string(data))
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Twitch stream detection via IRC disconnect | Twitch EventSub stream.offline webhook | EventSub stable 2022, stream.offline needs no scope | IRC disconnect ≠ stream end; webhook is the correct signal |
| Custom webhook handler per event type | Single `handleNotification` dispatcher in webhook handler | Phase 14+ | Add case to existing switch; no new HTTP endpoint needed |
| Kick lifecycle via Pusher events | Kick official webhook stream.online/offline (April 2025) | Kick DevDocs 2025-04 | Official but requires new webhook endpoint; Pusher has no stream lifecycle events |

**Deprecated/outdated:**
- Checking Twitch IRC connection state to detect stream end: IRC channel membership does not reflect stream live status.
- Kick: The Pusher WebSocket carries chat events only; no Pusher event signals stream on/off.

---

## Open Questions

1. **Kick webhook feasibility in local dev**
   - What we know: Official Kick webhook API exists (stream.offline added April 2025). Requires a public HTTPS callback URL. kick-listener has no HTTP server for webhooks.
   - What's unclear: Can the existing docker-compose/API gateway route forward Kick webhook callbacks? Are Kick webhook subscriptions manageable via API or only via developer portal?
   - Recommendation: Defer Kick `this_stream` expiry. Show a UI note and default to "Unlimited" for Kick users. Research as a follow-on to Phase 19.

2. **Debounce duration for stream.offline**
   - What we know: Twitch can emit stream.offline on category changes and sub-broadcast restarts.
   - What's unclear: How long is the typical gap between stream.offline and stream.online during a brief restart?
   - Recommendation: Use 60-second debounce for MVP. Can be tuned based on real-world observation.

3. **TikTok user_id lookup**
   - What we know: Users table has `tiktok_id` column. TikTok listener uses username strings.
   - What's unclear: Does `tiktok_id` match the username or the platform user ID? The TikTok status checker uses username (e.g., `@user`).
   - Recommendation: Verify the users table schema for tiktok identity columns before implementing the lifecycle subscriber lookup.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing + testify (share-service), Go testing (twitch-eventsub-listener) |
| Config file | none (standard `go test ./...`) |
| Quick run command | `cd services/share-service && go test ./... -short` |
| Full suite command | `cd services/share-service && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXPIRY-01 | expiry_option persisted on accept | unit | `go test ./handlers/ -run TestAcceptShareRequest -short` | Partial (TestAcceptShareRequest_ExpiryValidation exists; DB persistence not tested) |
| EXPIRY-02 | stream.online/offline EventSub subscribed | unit | `go test ./eventsub/ -run TestSubscribeToStream -short` | Wave 0 |
| EXPIRY-03 | share expires on stream end event | unit | `go test ./jobs/ -run TestLifecycleSubscriber -short` | Wave 0 |
| EXPIRY-04 | time-based expiry via 5-min job | unit | `go test ./jobs/ -run TestExpiryJob_TimedAccepted -short` | Wave 0 |
| EXPIRY-05 | YouTube/TikTok publish lifecycle event | unit | `go test ./streams/ -run TestStreamOfflinePublishesLifecycle -short` | Wave 0 |
| EXPIRY-06 | Kick gracefully disabled | unit | `go test ./handlers/ -run TestAcceptModal_KickDisabled -short` | Wave 0 (frontend) |

### Sampling Rate

- **Per task commit:** `cd services/share-service && go test ./... -short`
- **Per wave merge:** `cd services/share-service && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `services/share-service/jobs/lifecycle_subscriber_test.go` — covers EXPIRY-03
- [ ] `services/share-service/jobs/expiry_test.go` — add TestExpiryJob_TimedAcceptedShares for EXPIRY-04
- [ ] `services/twitch-eventsub-listener/eventsub/subscription_manager_test.go` — covers EXPIRY-02
- [ ] `services/youtube-listener/streams/lifecycle_test.go` — covers EXPIRY-05 (YouTube publish)
- [ ] Migration `034_share_expiry_fields.sql` — needed before all tests that touch DB

---

## Sources

### Primary (HIGH confidence)
- Twitch EventSub official docs (stream.online, stream.offline): https://dev.twitch.tv/docs/eventsub/eventsub-subscription-types/#streamonline — confirmed no OAuth scope required, app token sufficient, payload fields verified
- Codebase: `services/share-service/jobs/expiry.go` — ExpiryJob pattern confirmed (5-minute ticker, immediate run)
- Codebase: `services/share-service/handlers/shares.go` RevokeShareRequest — transactional revoke + deactivate pattern confirmed
- Codebase: `services/share-service/repository/share_repo.go` — current schema confirmed (no expiry_option column)
- Codebase: `services/twitch-eventsub-listener/eventsub/subscription_manager.go` — existing subscription infrastructure
- Codebase: `services/twitch-eventsub-listener/webhooks/handler.go` — webhook dispatch pattern confirmed
- Codebase: `services/youtube-listener-innertube/poller/lifecycle.go` — HandleStreamOffline pattern
- Codebase: `services/tiktok-listener/src/livestream/status-checker.ts` — offline detection pattern
- Codebase: `frontend/src/app/dashboard/shares/components/AcceptModal.tsx` — confirmed UI sends expiry_option/expiry_hours to backend

### Secondary (MEDIUM confidence)
- Kick DevDocs GitHub (KickEngineering/KickDevDocs): stream.online/stream.offline webhook events added April 2025 — confirmed official but implementation details require further reading

### Tertiary (LOW confidence)
- WebSearch: Kick stream lifecycle detection — general pointers to KickDevDocs; no detailed payload documentation retrieved

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all existing libraries; no new dependencies
- Architecture (share-service expiry extension): HIGH — ExpiryJob and RevokeShareRequest patterns are direct templates
- Architecture (Twitch EventSub stream.offline): HIGH — confirmed no scope required; subscription_manager pattern is clear
- Architecture (YouTube/TikTok lifecycle publish): MEDIUM — the offline detection path is clear; the exact publisher wiring requires tracing manager/poller coordination
- Architecture (Kick): LOW — official API exists but local dev feasibility unverified

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (Kick DevDocs are actively changing; re-verify if implementing Kick)
