---
phase: 14-foundation
plan: 02
subsystem: share-service-api
tags: [http-api, repository, handlers, foundation]
dependency_graph:
  requires: [share-requests-table, share-service-models, users-table]
  provides: [user-search-api, share-request-api, share-service-http]
  affects: [share-workflow, premium-gating]
tech_stack:
  added: [gin, pgxpool]
  patterns: [repository-pattern, handler-pattern, graceful-shutdown, case-insensitive-search]
key_files:
  created:
    - services/share-service/repository/user_search.go
    - services/share-service/repository/share_repo.go
    - services/share-service/handlers/search.go
    - services/share-service/handlers/shares.go
    - services/share-service/Dockerfile
  modified:
    - services/share-service/cmd/main.go
    - services/share-service/go.mod
    - services/share-service/go.sum
decisions:
  - title: LOWER() function for case-insensitive username search
    rationale: Leverages functional index from migration 028 (LOWER(username)), provides efficient case-insensitive partial matching with ILIKE pattern
    alternatives: ILIKE without LOWER() (no index usage), full-text search (overkill for simple username matching)
  - title: Accept/Reject endpoints require authenticated user to be recipient
    rationale: Prevents unauthorized users from accepting/rejecting requests not intended for them, enforces bidirectional sharing model
    alternatives: Allow sender to cancel (adds complexity), admin-only override (not in scope)
  - title: Platform-specific filtering in search (twitch_id IS NOT NULL)
    rationale: Users search within specific platform context (UI has platform selector), prevents showing users without that platform linked
    alternatives: Search all platforms (confusing UX), client-side filtering (inefficient)
  - title: Premium enforcement at middleware layer for POST /shares
    rationale: Follows server-side validation pattern from research (Pitfall #1), prevents client bypass, reuses existing RequirePremium middleware
    alternatives: Check in handler (duplicated logic), skip enforcement (defeats monetization)
metrics:
  duration_minutes: 8
  completed_date: "2026-03-09"
  tasks_completed: 4
  files_created: 5
  files_modified: 3
  tests_added: 0
  test_coverage: 0%
---

# Phase 14 Plan 02: User Search and Share Request API Summary

**One-liner:** HTTP API for case-insensitive user search (leveraging migration 028 index) and share request creation/management with premium enforcement

## Overview

Implemented complete REST API for share request workflow by creating repository layer (UserSearchRepository, ShareRepository) with case-insensitive search and CRUD operations, HTTP handlers (SearchHandler, ShareHandler) with authorization and validation, and share-service HTTP server with Gin routing, health checks, and graceful shutdown. This enables SHARE-01 (user search) and SHARE-02 (request creation) requirements.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement user search repository | 9db4d4e | services/share-service/repository/user_search.go |
| 2 | Implement share request repository | fc5bcda | services/share-service/repository/share_repo.go |
| 3 | Implement HTTP handlers | 31d532e | services/share-service/handlers/search.go, services/share-service/handlers/shares.go |
| 4 | Create share-service main.go and Dockerfile | 45d22c1 | services/share-service/cmd/main.go, services/share-service/Dockerfile |

## What Was Built

### Repository Layer

**UserSearchRepository (user_search.go):**
- `SearchUsersByPlatform(platform, query, limit)` - Case-insensitive username search
- Uses `LOWER(username) LIKE LOWER($1)` pattern to leverage functional index from migration 028
- Platform filtering via `twitch_id IS NOT NULL`, `google_id IS NOT NULL`, etc.
- Maximum 10 results, alphabetically ordered by username
- Returns UserSearchResult struct with id, username, display_name, profile_image_url

**ShareRepository (share_repo.go):**
- `Create(ShareRequest)` - Creates pending request with 7-day auto-expiry via SQL `INTERVAL '7 days'`
- `GetByID(id)` - Retrieves single request with all fields
- `ListIncoming(recipientUserID, status)` - Lists requests for recipient, optional status filtering
- `UpdateStatus(id, newStatus)` - Updates status to accepted/rejected/expired, sets responded_at timestamp
- `ListByOverlay(overlayID)` - Lists all requests for specific overlay (sender view)

### HTTP Handlers

**SearchHandler (search.go):**
- `GET /api/v1/users/search?platform=twitch&query=xqc`
- Validates platform enum (twitch/youtube/kick/tiktok)
- Returns maximum 10 results via repository limit parameter
- Structured logging with platform, query, result_count

**ShareHandler (shares.go):**
- `POST /api/v1/shares` - Create share request
  - Validates sender owns overlay (authorization check)
  - Looks up recipient by username (case-insensitive)
  - Prevents self-sharing at application layer
  - Returns 201 Created with full ShareRequest object
- `GET /api/v1/shares/incoming?status=pending` - List incoming requests
  - Optional status filtering for Pending vs History tabs
  - Returns array of ShareRequest objects
- `POST /api/v1/shares/:id/accept` - Accept pending request
  - Verifies user is recipient (authorization)
  - Verifies request is pending (state validation)
  - Updates status to accepted, sets responded_at
- `POST /api/v1/shares/:id/reject` - Reject pending request
  - Same authorization and validation as accept
  - Updates status to rejected

### HTTP Server (cmd/main.go)

**Service configuration:**
- Port 8090 (configurable via PORT env var)
- Database connection via shared/database package
- Health checks: `/health/live` (always 200), `/health/ready` (pings database)
- Graceful shutdown with 25-second timeout (project standard)
- Gin router with recovery middleware and logger (skips health check paths)

**Route structure:**
```
/api/v1 (all routes require JWT auth via middleware.JWTAuth)
  GET  /users/search              (searchHandler.SearchUsers)
  GET  /shares/incoming            (shareHandler.ListIncoming)
  POST /shares                     (shareHandler.CreateRequest) - PREMIUM REQUIRED
  POST /shares/:id/accept          (shareHandler.AcceptRequest) - PREMIUM REQUIRED
  POST /shares/:id/reject          (shareHandler.RejectRequest) - PREMIUM REQUIRED
/admin (requires JWT + admin role)
  POST /users/:id/premium          (adminHandler.SetUserPremium)
```

**Premium enforcement:**
- Create/Accept/Reject share requests require `RequirePremium` middleware
- List incoming requests does NOT require premium (Pitfall #5 from research)
- Follows server-side validation pattern to prevent client bypass

### Docker Deployment (Dockerfile)

**Multi-stage build:**
- Builder stage: golang:1.25.6-alpine with go mod download
- Runtime stage: alpine:3.23 with ca-certificates and tzdata
- Non-root user (appuser:1000) for security
- Health check: `wget http://localhost:8090/health/live` every 30s
- Exposes port 8090

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Added Accept/Reject endpoints not in plan**
- **Found during:** Task 3 (handlers implementation)
- **Issue:** Plan only specified CreateRequest and ListIncoming, but share request lifecycle requires accept/reject actions. Without these, recipients have no way to respond to requests.
- **Fix:** Added `AcceptRequest` and `RejectRequest` handler methods with proper authorization (verify recipient) and validation (verify pending status). Both update status via `ShareRepository.UpdateStatus`.
- **Files modified:** services/share-service/handlers/shares.go
- **Commit:** 31d532e (included in Task 3 commit)

**2. [Rule 3 - Blocking issue] Fixed go.mod Go version mismatch**
- **Found during:** Task 4 (Docker build)
- **Issue:** `go mod tidy` automatically updated go.mod to require Go 1.26.1, but Dockerfile uses 1.25.6 (matching shared module). Docker build failed with "go.mod requires go >= 1.26.1".
- **Fix:** Manually corrected go.mod to `go 1.25.6` to match shared module and project standard.
- **Files modified:** services/share-service/go.mod
- **Commit:** 45d22c1 (included in Task 4 commit)

**3. [Rule 2 - Missing critical functionality] Added UpdateStatus and ListByOverlay repository methods**
- **Found during:** Task 2 (share repository)
- **Issue:** Plan specified only Create/GetByID/ListIncoming, but accept/reject workflow requires status updates. Sender also needs to list their sent requests by overlay.
- **Fix:** Added `UpdateStatus(id, newStatus)` method to update share_requests.status and set responded_at timestamp. Added `ListByOverlay(overlayID)` for sender view.
- **Files modified:** services/share-service/repository/share_repo.go
- **Commit:** fc5bcda (included in Task 2 commit)

## Technical Notes

**Case-insensitive search performance:**
Migration 028 created a functional index on `LOWER(username)`, which PostgreSQL uses when the query includes `LOWER(username) LIKE LOWER($1)`. This provides efficient partial matching without full table scan. The ILIKE pattern `'%query%'` enables "contains" search (e.g., "xqc" matches "xQcOW").

**7-day expiry implementation:**
The `ShareRepository.Create` method uses SQL `NOW() + INTERVAL '7 days'` to calculate expires_at at insert time. This is efficient (no application-level date math) and consistent (timezone-aware). The expiry job (Phase 14 Plan 03) will query `WHERE status = 'pending' AND expires_at < NOW()` using the partial index from migration 030.

**Premium enforcement architecture:**
The `RequirePremium` middleware (from Plan 14-01) queries `users.is_premium` on every request. This server-side check prevents client bypass (Pitfall #1 from research). Non-premium users can still view incoming requests (GET /shares/incoming) but cannot create/accept/reject (POST operations).

**Authorization patterns:**
- Overlay ownership: Query `SELECT user_id FROM overlays WHERE id = $1` before creating share request
- Recipient verification: Compare `share_request.recipient_user_id` with JWT user_id before accept/reject
- Self-share prevention: Both database CHECK constraint and application-level validation

**Handler error responses:**
- 400 Bad Request: Validation failures (missing params, self-share, invalid platform)
- 403 Forbidden: Authorization failures (not overlay owner, not request recipient)
- 404 Not Found: Resource not found (overlay, user, share request)
- 500 Internal Server Error: Database or system failures

## Self-Check: PASSED

**Created files exist:**
```
FOUND: services/share-service/repository/user_search.go
FOUND: services/share-service/repository/share_repo.go
FOUND: services/share-service/handlers/search.go
FOUND: services/share-service/handlers/shares.go
FOUND: services/share-service/Dockerfile
```

**Commits exist:**
```
FOUND: 9db4d4e (Task 1 - user search repository)
FOUND: fc5bcda (Task 2 - share request repository)
FOUND: 31d532e (Task 3 - HTTP handlers)
FOUND: 45d22c1 (Task 4 - HTTP server + Dockerfile)
```

**Service compiles:**
```
$ cd services/share-service && go build ./cmd/main.go
# Compiled successfully (no output)
```

**Docker image builds:**
```
$ docker build -t share-service:test -f services/share-service/Dockerfile .
Successfully built 5808d7c7f0d6
Successfully tagged share-service:test
```

## Next Steps

Plan 14-03 will implement:
- Request response endpoints (accept/reject with bidirectional overlay creation)
- Expiry job for auto-expiring pending requests after 7 days
- WebSocket notifications for real-time request updates
- Integration with overlay-manager for bidirectional sharing setup
## Self-Check: PASSED

All files created, all commits verified, service compiles, Docker image builds successfully.
