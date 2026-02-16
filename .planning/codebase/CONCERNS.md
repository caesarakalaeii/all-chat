# Codebase Concerns

**Analysis Date:** 2026-02-16

## Tech Debt

### TikTok Listener Unofficial Library Dependency

**Issue:** `services/tiktok-listener/` relies on unofficial TikTok-Live-Connector library based on reverse-engineered WebSocket protocol

**Files:** `services/tiktok-listener/` (Node.js service using `tiktok-live-connector` npm package)

**Impact:**
- Service marked as "BETA" in `services/tiktok-listener/README.md:3`
- Breaking changes if TikTok changes internal APIs
- No official support or guarantees from TikTok
- Service may fail silently in production when TikTok updates protocol

**Fix Approach:**
1. Monitor TikTok official API announcements for official Live Chat API release
2. Implement fallback/circuit-breaker pattern for graceful degradation when library breaks
3. Add monitoring alerts for connection failures specific to TikTok
4. Document exact version of `tiktok-live-connector` locked in `package.json`
5. Plan migration path to official API once available

**Priority:** Medium (impacts one platform only, degradation documented)

---

### YouTube Listener Complexity

**Issue:** `services/youtube-listener/` has significant complexity with 2,220 lines in `streams/manager.go`, 1,063 lines in `quota/tracker.go`, and 751 lines in `streams/poller.go`

**Files:**
- `services/youtube-listener/streams/manager.go` (2220 lines)
- `services/youtube-listener/quota/tracker.go` (1063 lines)
- `services/youtube-listener/streams/poller.go` (751 lines)
- `services/youtube-listener/api/grpc_client.go` (644 lines)

**Impact:**
- Difficult to maintain and test
- Quota coordination logic is fragile (contains CRITICAL DEBUG comments throughout)
- Multiple TODO comments indicate incomplete implementation: `services/youtube-listener/handlers/quota.go:220` "TODO: Implement actual database query"
- Token state management has multiple DEBUG comments suggesting previous issues

**Fix Approach:**
1. Extract quota logic into separate domain-focused package with clear responsibilities
2. Remove DEBUG comments and consolidate logging with proper structured logging
3. Implement comprehensive integration tests for YouTube OAuth flow
4. Add circuit breaker pattern for API failures
5. Document quota decision tree with decision tables instead of inline complexity

**Priority:** High (impacts production reliability, currently incomplete)

---

### Context.Background() Usage in Goroutines

**Issue:** Services use `context.Background()` for long-running goroutines that should be cancellable

**Files:**
- `services/overlay-manager/creditroll/handler.go` (incrementCreditRollDisplay goroutine)
- `services/overlay-manager/cmd/main.go` (multiple locations)
- Other services in initialization paths

**Example:** `services/overlay-manager/creditroll/handler.go:` - `go h.incrementCreditRollDisplay(context.Background(), overlayID, session.SessionID)` spawns goroutine with non-cancellable context

**Impact:**
- Goroutines cannot be cancelled during graceful shutdown
- Potential resource leaks if shutdown is incomplete
- 25-second graceful shutdown timeout may be exceeded if goroutines don't respond to signals

**Fix Approach:**
1. Replace `context.Background()` with parent context from request or application lifecycle
2. Pass `context.WithCancel()` contexts to goroutines with explicit cancellation channels
3. Implement middleware to propagate context through goroutine spawning
4. Add tests for graceful shutdown completion

**Priority:** Medium (affects reliability under shutdown, not critical for running state)

---

### Missing TODO Implementations

**Issue:** Several incomplete implementations marked with TODO comments in production-path code

**Files:**
- `services/api-gateway/cmd/main.go:238` - "TODO: Update to CORSFromEnv() after shared module rebuild" (CORS not environment-configurable)
- `services/youtube-listener/quota/coordinator.go:267` - "TODO: Implement circuit breaker pattern if needed"
- `services/youtube-listener/handlers/quota.go:220` - "TODO: Implement actual database query"
- `services/youtube-listener/handlers/quota.go:497` - "TODO: Get from tracker"
- `services/youtube-listener/api/grpc_client.go` - Multiple DEBUG comments about "CRITICAL DEBUGGING"

**Impact:**
- Circuit breaker pattern missing for YouTube API failures
- Quota tracking may not persist state to database as intended
- Debug code left in production paths suggests incomplete development

**Fix Approach:**
1. Implement CORS environment configuration (CORSFromEnv)
2. Add circuit breaker using resilience pattern (exponential backoff, failure threshold)
3. Complete quota database persistence implementation
4. Remove all DEBUG comments and replace with structured logging
5. Add tests for each TODO implementation

**Priority:** High (affects production readiness)

---

## Known Issues

### Admin Routes Lack Role-Based Authorization

**Issue:** Admin routes exist but lack role-based access control (RBAC) checks

**Files:** `services/api-gateway/cmd/main.go:362`

**Code Comment:** `// Admin routes (protected - TODO: add admin role check)`

**Symptoms:**
- Any authenticated user with valid JWT can access admin endpoints
- No differentiation between regular users and administrators
- Potential for unauthorized administrative actions

**Trigger:**
1. Authenticate as regular user
2. Call any admin endpoint
3. Endpoint will succeed even though user lacks admin role

**Workaround:**
- Implement admin role check before rolling to production
- Currently relies on JWT `roles` claim which exists but is not validated

**Fix Priority:** Critical (security issue, blocks production deployment)

---

### JWT Tokens Stored in localStorage

**Issue:** Frontend stores JWT tokens in localStorage which is vulnerable to XSS attacks

**Files:**
- `frontend/src/app/auth/callback/page.tsx:52` - `localStorage.setItem('refresh_token', refreshToken)`
- `frontend/src/components/ImpersonationBanner.tsx` - Admin token stored in localStorage
- Multiple hooks use `localStorage.getItem('jwt_token')`

**Symptoms:**
- If any XSS vulnerability exists on the page, attacker can steal tokens
- Tokens persist across browser sessions (convenience vs. security tradeoff)
- No automatic invalidation when browser closes

**Trigger:**
- Any DOM-based XSS vulnerability in frontend components
- Malicious script can call `localStorage.getItem('jwt_token')`

**Workaround:**
- Ensure strict Content Security Policy headers are set
- Never store sensitive data in localStorage without encryption
- Frontend already protects against common XSS by using React (auto-escaping), but custom dangerouslySetInnerHTML usage could introduce vulnerabilities

**Fix Approach:**
1. Move JWT to HttpOnly cookies (requires backend changes to set cookies)
2. Keep refresh tokens in HttpOnly secure cookies
3. Update auth flow to use cookie-based sessions for automatic cleanup
4. Keep localStorage only for non-sensitive data (preferences, theme)

**Priority:** High (security best practice, affects all authenticated users)

---

### TikTok Listener Node.js TypeScript Integration Debt

**Issue:** TikTok listener is implemented in Node.js/TypeScript while all other services are Go, creating:
- Different deployment and scaling patterns
- Separate logging and monitoring concerns
- Different error handling strategies

**Files:** `services/tiktok-listener/` (entire service)

**Impact:**
- Operational overhead managing two language runtimes
- Inconsistent observability across services
- Harder to troubleshoot cross-service message flow
- Deployment complexity with Docker Compose and Kubernetes

**Fix Approach:**
1. Migrate TikTok listener to Go once official or more stable unofficial library available
2. Use same base service template as other listeners for consistency
3. Consolidate logging format and error handling patterns
4. Benefits: Single runtime, consistent deployments, unified monitoring

**Priority:** Low (not blocking, but technical hygiene issue)

---

## Security Considerations

### CORS Configuration Allows Wildcards

**Issue:** CORS configuration uses `*` for browser extensions which allows any extension to make requests

**Files:** `services/api-gateway/cmd/main.go:184-238`

**Risk:**
- Any Chrome/Firefox extension can make authenticated requests to the API
- If JWT token stored in localStorage is stolen, any extension can impersonate user
- No way to restrict to specific trusted extensions in development

**Current Mitigation:**
- JWT validation still required (tokens can't be forged)
- Rate limiting prevents abuse at scale
- Extension token approach requires separate browser tokens

**Recommendations:**
1. For production: Implement extension manifest key pinning (see TODO.md for steps)
2. Generate RSA keypair for extension signing
3. Store public key in manifest, derive consistent extension ID
4. Update CORS to specific extension ID: `chrome-extension://<id-only>`
5. Keep Firefox moz-extension wildcard only if not published to AMO

**Priority:** High (incomplete security hardening)

---

### No Service-to-Service Authentication

**Issue:** Services communicate without mutual TLS or authentication headers

**Files:** All inter-service communication paths

**Risk:**
- If Kubernetes NetworkPolicies are misconfigured, any pod can call any service
- No audit trail of which service called which
- No encryption for internal communication

**Current Mitigation:**
- Kubernetes NetworkPolicies restrict at network layer
- Services run in isolated namespace
- No sensitive data passed in service-to-service calls (mostly UUIDs and references)

**Recommendations:**
1. Implement mTLS between services using cert-manager
2. Or: Use JWT tokens for service-to-service auth (slower but simpler)
3. Add authorization checks: verify calling service is allowed to call target service
4. Log all service-to-service calls for audit trails

**Priority:** Medium (mitigated by NetworkPolicies, but should be addressed)

---

### Extension Impersonation Risk

**Issue:** Frontend allows admin impersonation via localStorage tokens without proper audit

**Files:** `frontend/src/components/ImpersonationBanner.tsx`

**Code:**
```typescript
const adminToken = localStorage.getItem('admin_token');
if (adminToken) {
  localStorage.setItem('jwt_token', adminToken);
  localStorage.removeItem('admin_token');
}
```

**Risk:**
- Admin token stored in localStorage can be stolen
- Impersonation leaves no audit trail beyond `impersonating` localStorage flag
- No rate limiting on impersonation changes

**Recommendations:**
1. Implement backend audit log for all impersonation events
2. Require admin re-authentication (TOTP or password) for impersonation
3. Add session ID to differentiate impersonation sessions
4. Set auto-expire on impersonation sessions (15 minutes)
5. Log impersonation token source and target user

**Priority:** High (impacts security audit compliance)

---

## Performance Bottlenecks

### Single Redis Instance Bottleneck

**Issue:** Single Redis instance handles both Streams (raw messages) and Pub/Sub (overlay delivery)

**Files:** Architecture-level (all services depend on Redis)

**Problem:**
- At 10,000 msg/s with 26 API Gateway pods = 260,000 message deliveries/sec
- Redis Pub/Sub does not persist messages (lost if subscriber crashes)
- Single point of failure for entire message pipeline
- Memory usage grows O(n) with number of subscribed pods

**Symptoms:**
- Message loss during high-volume chat events
- Latency spikes when delivering to many pods
- Cannot scale beyond ~1,000 msg/s sustainably

**Improvement Path:**
1. Separate Redis: Streams instance (persistent) vs. Pub/Sub instance (ephemeral)
2. Implement Redis Sentinel for automatic failover
3. Consider Redis Cluster for horizontal scaling
4. Add message queue (RabbitMQ/Kafka) for high-volume channels
5. Implement circuit breaker if Redis becomes unavailable

**Priority:** Medium (current scale acceptable, but limits growth)

---

### YouTube Quota Tracking Reliability

**Issue:** Quota tracking uses in-memory state that could be lost on pod restart

**Files:** `services/youtube-listener/quota/tracker.go` (1063 lines)

**Problem:**
- Quota state stored in memory, not persisted to database
- Pod restart loses quota estimation
- No distributed quota coordination between replicas
- Could lead to quota exhaustion if multiple replicas don't coordinate

**Symptoms:**
- After pod restart, quota state reset incorrectly
- Potential for quota exhaustion in multi-replica scenarios
- No visibility into quota decisions across all replicas

**Improvement Path:**
1. Persist quota state to PostgreSQL with per-channel tracking
2. Implement distributed quota coordinator across all replicas
3. Add metrics for quota state visibility
4. Implement quota leasing: reserve quota before request, confirm after success
5. Add fallback quotas for error recovery

**Priority:** Medium (quota tracking is critical but current implementation is incomplete)

---

### Large Go Files Difficult to Test

**Issue:** YouTube listener has several >600 line files with complex interdependencies

**Files:**
- `services/youtube-listener/streams/manager.go` (2220 lines)
- `services/youtube-listener/quota/tracker.go` (1063 lines)
- `services/youtube-listener/streams/poller.go` (751 lines)

**Problem:**
- Difficult to unit test due to size and complexity
- Integration tests required for most functionality
- Mocking dependencies is complex due to tight coupling
- Changes require understanding large code sections

**Fix Approach:**
1. Extract quota logic into separate, testable domain package
2. Split manager.go into smaller focused files (one responsibility each)
3. Implement interfaces for all external dependencies (Database, API, Redis)
4. Create test helpers for common setup patterns
5. Aim for 70%+ unit test coverage per file

**Priority:** Medium (affects maintainability)

---

## Fragile Areas

### YouTube OAuth Token Encryption Migration

**Issue:** OAuth tokens encrypted with AES-GCM but migration incomplete and fragile

**Files:**
- `services/youtube-listener/cmd/token_backfill/main.go` (migration tool)
- `shared/encryption/encryption.go` (encryption implementation)
- Migrations tracking `encryption_version` flag

**Why Fragile:**
- Manual migration tool must be run separately after secret deployment
- Mixing encrypted and plaintext tokens during migration window
- If migration fails partway, inconsistent state with no automated recovery
- Encryption key rotation requires additional manual steps
- No automated tests for encryption/decryption with different keys

**Safe Modification:**
1. Always run migration in dry-run mode first: verify counts match
2. Verify encryption key is set before any operations
3. Check `encryption_version` column has data
4. Use database transaction for migration: `BEGIN; UPDATE...; COMMIT;` or rollback
5. Verify token decryption works in new environment before cutover
6. Keep plaintext tokens backed up until 30 days post-migration

**Test Coverage Gaps:**
- No integration test for encrypt/decrypt roundtrip
- No test for key rotation
- No test for mixed encrypted/plaintext state
- No test for migration rollback

**Priority:** High (security-critical, but currently working)

---

### Message Processor Consumer Group State

**Issue:** Message Processor consumer group reads from Redis Streams but state management is implicit

**Files:** `services/message-processor/cmd/main.go` (769 lines)

**Why Fragile:**
- Consumer group offset not explicitly managed in code review
- If processor crashes, recovery behavior depends on Redis state
- No backpressure mechanism if downstream processing slows
- Redis Streams auto-trim could discard unprocessed messages if processor stalls
- No monitoring of consumer lag

**Safe Modification:**
1. Always check consumer group status: `XINFO GROUPS chat:raw`
2. Monitor processing latency and lag before changes
3. Test recovery: kill processor mid-stream, verify messages aren't lost
4. Document assumptions about max stream size (50,000 messages default)
5. Add metrics for consumer lag and pending messages

**Test Coverage Gaps:**
- No test for message loss on processor crash
- No test for consumer group recovery
- No test for backpressure behavior
- No test for stream trim edge cases

**Priority:** High (data loss risk)

---

### Admin Route Security

**Issue:** Admin endpoints exist but authorization checks are missing (marked TODO)

**Files:** `services/api-gateway/cmd/main.go:362`

**Why Fragile:**
- Current check: JWT exists (any authenticated user)
- Missing check: User has admin role in JWT claims
- No audit log of admin action attempts
- No rate limiting specific to admin endpoints
- Easy to accidentally grant admin access to regular users

**Safe Modification:**
1. Never call admin endpoints in tests without explicit admin role setup
2. Verify JWT `roles` claim contains "admin" before allowing access
3. Log all admin endpoint access attempts with user ID
4. Add rate limiting: max 10 admin actions/minute per user
5. Consider requiring re-authentication for sensitive operations

**Test Coverage Gaps:**
- No test for authorization failure (non-admin users)
- No test for missing role claim
- No test for audit logging
- No test for rate limiting

**Priority:** Critical (security blocker for production)

---

## Scaling Limits

### YouTube API Quota Per User

**Issue:** Each user has separate YouTube OAuth credentials with 1,009,000 units/day quota

**Current Capacity:** ~1,009,000 units/day per user account

**Limit:** Hits quota if user monitors 100+ live channels simultaneously (varies by polling interval)

**Scaling Path:**
1. For users hitting quota: recommend OAuth key registration for higher quotas
2. Implement quota sharing: multiple overlays from same user share auth tokens
3. Add quota tier system: free users get 100 units/day shared pool
4. Consider YouTube Service Account with enterprise quota (requires YouTube approval)
5. Implement smart polling: reduce frequency for channels with low chat activity

**Priority:** Low (limit depends on usage patterns, mitigation options available)

---

### Database Connection Pool Saturation

**Issue:** Services connect to shared PostgreSQL via connection pooling

**Current State:** PgBouncer pooling enabled in Kubernetes (documented in `docs/phase-reports/CRITICAL_ARCHITECTURE_ANALYSIS.md`)

**Limit:** Single shared database becomes bottleneck as services scale

**Scaling Path:**
1. Separate database per service: reduces lock contention and improves isolation
2. Implement read replicas for reporting queries
3. Add caching layer (Redis) for frequently read data (overlays, configurations)
4. Implement connection pooling per service (PgBouncer or pgx pool tuning)
5. Archive old messages to separate analytical database

**Priority:** Low (not blocking current scale, but planning concern)

---

## Dependencies at Risk

### Unofficial TikTok Library

**Issue:** `tiktok-live-connector` npm package depends on unofficial WebSocket protocol reverse engineering

**Files:** `services/tiktok-listener/package.json`

**Risk:**
- No maintenance guarantees from library authors
- Could break anytime TikTok changes internal protocol
- No official support channel
- Security vulnerabilities may not be patched

**Impact:** If TikTok platform breaks, entire service becomes non-functional

**Migration Plan:**
1. Monitor `tiktok-live-connector` GitHub for maintenance status
2. Track TikTok official API announcements
3. When official API available: implement new service module
4. Maintain backward compatibility with existing overlays during migration
5. Plan 3-month migration window with deprecation notice

**Priority:** Medium (beta service, documented limitation)

---

### Go Module Pinning

**Issue:** All services use Go 1.25.6 but go.mod versions may drift

**Files:** All `services/*/go.mod`

**Risk:**
- Dependency vulnerabilities not automatically patched
- Build drift if different developers run on different versions
- Kubernetes deployments could use different build versions

**Recommendations:**
1. Use `go.mod` version locking with specific patch versions
2. Implement `govulncheck ./...` in CI pipeline
3. Use Dependabot or Renovate for automated dependency updates
4. Regular `go get -u` followed by testing before merge
5. Pin base images in Dockerfile to specific Go versions

**Priority:** Low (version consistency good, needs CI automation)

---

## Missing Critical Features

### Feature: End-to-End Encryption for Overlays

**Issue:** WebSocket connection from API Gateway to frontend is not encrypted (relies on TLS only)

**Problem:** TLS terminates at Kubernetes Ingress, but internal TLS between services is incomplete

**Blocks:** Compliance with HIPAA/PCI if handling sensitive data

**Workaround:** Currently only handles chat messages which are public, so acceptable

**Priority:** Low (not applicable to current use case)

---

### Feature: Message Deduplication at Scale

**Issue:** Message deduplication only implemented at TikTok listener level, not system-wide

**Files:** `services/tiktok-listener/` has deduplication cache

**Problem:**
- Kick/YouTube/Twitch could theoretically deliver duplicate messages on reconnect
- API Gateway broadcasts all messages without deduplication
- Frontend could render duplicates if WebSocket reconnects mid-message

**Improvement:**
1. Add message ID to unified format from all platforms
2. Implement deduplication in Message Processor before pub/sub
3. Add bloom filter for message IDs seen in last 24 hours
4. Detect and log duplicate deliveries for monitoring

**Priority:** Medium (not blocking, but quality improvement)

---

## Test Coverage Gaps

### YouTube Listener Integration Tests Incomplete

**Issue:** YouTube listener has unit tests but integration tests are partial

**Files:** `services/youtube-listener/` contains unit tests for `oauth/`, `api/`, `quota/` but missing end-to-end flow

**What's Not Tested:**
- Full OAuth flow with real tokens (uses mocks)
- Message polling pipeline from API to Redis Streams
- Quota tracking during actual polling
- Stream manager leader election failover
- API client reconnection logic under network failures

**Risk:**
- Edge cases in production that unit tests don't catch
- OAuth regressions not caught until production
- Message loss scenarios not verified

**Fix Approach:**
1. Use testcontainers for integration tests (PostgreSQL, Redis)
2. Mock YouTube API responses for realistic scenarios
3. Test full message flow: OAuth → Stream Poll → Redis Publish
4. Test quota exhaustion recovery
5. Test leader election with multiple replicas

**Priority:** High (affects production reliability)

---

### E2E Tests Missing for Critical Paths

**Issue:** No end-to-end tests for full message pipeline

**Files:** Tests exist but don't cover: Listener → Redis Streams → Processor → Pub/Sub → API Gateway → WebSocket → Frontend

**What's Tested:** Individual services tested separately

**What's Not Tested:**
- Message loss between services
- WebSocket connection recovery
- Overlay connection and subscription flow
- Multi-platform message aggregation
- Rate limiting under load
- Redis failover scenarios

**Priority:** Medium (development blocker for new platform additions)

---

### Error Handling Inconsistency

**Issue:** Error handling patterns vary across services

**Examples:**
- Some services use `log.Fatal()` for startup errors (correct)
- Some services silently ignore errors in goroutines
- Some services use `panic()` (should use proper logging)
- No standardized error types for domain errors

**Fix Approach:**
1. Define error interfaces in `shared/errors/`
2. Use consistent error wrapping: `fmt.Errorf("operation failed: %w", err)`
3. Never silently ignore errors in goroutines
4. Return errors from goroutines via channels or error context
5. Log errors at appropriate level (warn, error, critical)

**Priority:** Medium (maintainability and debugging)

---

## Summary

**Critical Issues (Block Production):**
1. Admin routes lack RBAC checks (security)
2. YouTube listener TODO implementations incomplete (reliability)
3. JWT tokens in localStorage (XSS vulnerability)

**High Priority Issues (Address Before Scale):**
1. TikTok listener uses unofficial library (maintenance risk)
2. YouTube OAuth token encryption migration (fragile)
3. Message processor consumer group state (data loss risk)
4. CORS wildcard configuration (incomplete security)

**Medium Priority Issues (Next Quarter):**
1. Single Redis bottleneck (scalability limit)
2. YouTube quota tracking reliability (incomplete persistence)
3. Large Go files difficult to test (maintainability)
4. Context.Background() in goroutines (shutdown safety)
5. Incomplete test coverage for critical paths (quality)

**Low Priority Issues (Nice to Have):**
1. TikTok listener Node.js/TypeScript debt (operational overhead)
2. Missing service-to-service authentication (mitigated by NetworkPolicies)
3. Database connection pooling saturation (future concern)
4. Dependency version management (needs CI automation)

---

*Concerns audit: 2026-02-16*
