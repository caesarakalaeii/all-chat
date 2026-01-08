# All-Chat TODO Tracker

**Last Updated**: 2026-01-08

## 🔴 High Priority (Security & Critical)

### CORS Configuration
- [ ] **Update CORS middleware to use CORSFromEnv()**
  - Location: `services/api-gateway/cmd/main.go:201`
  - Current: Uses basic CORS() function
  - Blocker: Waiting for shared module rebuild
  - Impact: Better environment-based CORS configuration
  - Note: Currently using inline function to skip WebSocket routes

### Security Improvements
- [ ] **Harden CORS for browser extensions using manifest key**
  - Location: `all-chat-extension/manifest.json` and `caesar-deployment/apps/workloads/all-chat/configmap.yaml`
  - Current: Allowing `chrome-extension://*` and `moz-extension://*` wildcards
  - Risk: Any malicious extension could make authenticated requests (mitigated by JWT validation)
  - **Implementation Steps**:
    1. Generate RSA keypair: `openssl genrsa 2048 | openssl pkcs8 -topk8 -nocrypt -out key.pem`
    2. Extract public key: `openssl rsa -in key.pem -pubout -outform DER | base64 -w 0`
    3. Add `"key": "<base64_public_key>"` to extension manifest.json
    4. Derive consistent extension ID from key (Chrome will generate same ID for all users)
    5. Update CORS_ORIGIN to specific extension ID: `chrome-extension://<consistent-id>`
    6. Store key.pem securely (DO NOT commit to git!)
  - **Firefox Note**: Still requires `moz-extension://*` wildcard unless published to AMO
  - Impact: High security improvement - only official extension can access API
  - References: Chrome Extension Manifest V3 docs, MDN Web Extensions
  - **Temporary Decision (2025-12-21)**: Using wildcard origins for rapid development, relying on JWT auth + rate limiting for security

- [x] **Implement AES-GCM encryption for OAuth tokens** ✅ COMPLETE
  - Location: `shared/encryption/encryption.go`, `shared/crypto/crypto.go`
  - Status: Implemented and in use by auth-service
  - Migration tool: `services/auth-service/cmd/token-encryption-backfill/main.go`
  - Migration doc: `docs/migrations/2025-02-auth-token-encryption.md`
  - Impact: Critical security improvement achieved
  - Note: Idempotent backfill tool safely migrates existing tokens without invalidation

- [x] **Implement rate limiting** ✅ COMPLETE (2026-01-08)
  - Location: `shared/ratelimit/ratelimit.go`, `services/api-gateway/cmd/main.go`
  - Status: Implemented with Redis-based distributed rate limiting
  - Configuration: `RATE_LIMIT_PER_MINUTE` env var (default: 300 req/min per IP/user)
  - Features: JWT-aware (per-user limits when authenticated, IP-based otherwise)
  - Excludes: Health checks, metrics, WebSocket connections, static files
  - Impact: Prevents abuse, protects resources

- [ ] **Configure CORS for production**
  - Location: `services/api-gateway/cmd/main.go:184`
  - Current: Allows `*` in development
  - Impact: Security hardening
  - Task: Update to `CORSFromEnv()` after shared module rebuild

## 🟡 Medium Priority (Features & Quality)

### Authentication & Authorization
- [ ] **Add display_name field to users table**
  - Location: `services/auth-service/handlers/streamer_info.go:106`
  - Current: Using username as display name
  - Impact: Better user experience

- [ ] **Remove Twitch OAuth workaround**
  - Location: `services/auth-service/handlers/platform_auth_v2.go:140`
  - Current: Workaround for Twitch add-source OAuth regression
  - Impact: Code cleanup
  - Wait for: Twitch to fix regression

### Testing & Observability
- [ ] **Complete YouTube Listener integration tests**
  - Impact: Ensure reliability for YouTube platform

- [ ] **Add comprehensive unit/integration tests**
  - Scope: All services
  - Impact: Code quality, prevent regressions

- [ ] **Add Prometheus metrics endpoints**
  - Scope: All services (currently only some have metrics)
  - Impact: Better monitoring and debugging

- [ ] **Implement distributed tracing with OpenTelemetry**
  - Scope: Cross-service request tracking
  - Impact: Debugging, performance optimization

## 🟢 Low Priority (New Features & Enhancements)

### Platform Support
- [ ] **Add Kick platform listener**
  - Phase: 2
  - Impact: Platform expansion

- [ ] **Add TikTok platform listener**
  - Phase: 2
  - Impact: Platform expansion

### Frontend & UI
- [ ] **Build React + Next.js frontend**
  - Purpose: Overlay display and configuration
  - Impact: Better user experience for streamers

- [ ] **Add overlay management API**
  - Features: CRUD operations for overlays
  - Impact: Self-service overlay configuration

### Scalability
- [ ] **Separate databases per service**
  - Current: Shared PostgreSQL
  - Impact: Service isolation, better scaling

- [ ] **Implement WebSocket connection pooling**
  - Impact: Handle more concurrent connections

- [ ] **Add message queue for high-volume channels**
  - Technology: RabbitMQ or Kafka
  - Impact: Handle channels with 10K+ viewers

---

## Summary Statistics

- **Total Tasks**: 18 (2 recently completed ✅)
- **High Priority**: 2 (Security critical)
- **Medium Priority**: 8 (Quality & features)
- **Low Priority**: 8 (Enhancements & scaling)
- **Completed**: 2 (AES-GCM encryption, Rate limiting)

## Notes

- Extension (all-chat-extension) has no pending TODOs after recent bug fixes
- **Security improvements completed (2026-01-08)**:
  - ✅ AES-GCM encryption implemented for OAuth tokens (with migration tool)
  - ✅ Rate limiting implemented in API Gateway (Redis-based, 300 req/min default)
- Previous fixes:
  - ✅ OAuth login bug fixed
  - ✅ Page layout preservation implemented
  - ✅ JWT middleware corrected to support viewer tokens
  - ✅ Error handling improved
