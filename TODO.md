# All-Chat TODO Tracker

**Last Updated**: 2026-01-15

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
- [x] **Add display_name field to users table** ✅ COMPLETE
  - Location: `migrations/001_initial_schema.sql:10`
  - Status: Field exists in users table since initial schema
  - Impact: Already providing better user experience

- [ ] **Remove Twitch OAuth workaround**
  - Location: `services/auth-service/handlers/platform_auth_v2.go:140`
  - Current: Workaround for Twitch add-source OAuth regression
  - Impact: Code cleanup
  - Wait for: Twitch to fix regression

### Testing & Observability
- [ ] **Complete YouTube Listener integration tests**
  - Status: Partial - unit tests exist (oauth/manager_test.go, api/parser_test.go, quota/tracker_test.go)
  - Impact: Ensure reliability for YouTube platform

- [ ] **Add comprehensive unit/integration tests**
  - Status: Partial - 40+ test files exist across services
  - Scope: All services (coverage varies by service)
  - Impact: Code quality, prevent regressions

- [ ] **Add Prometheus metrics endpoints**
  - Status: Partial - kick-listener has full implementation (`services/kick-listener/metrics/metrics.go`)
  - Scope: All services (currently only kick-listener has metrics)
  - Impact: Better monitoring and debugging

- [x] **Implement distributed tracing with OpenTelemetry** ✅ COMPLETE (2026-01-15)
  - Location: `shared/tracing/tracing.go`, `services/api-gateway/cmd/main.go`
  - Status: Implemented with OTEL_ENABLED environment variable
  - Features: Optional enablement via env var, integrated in API Gateway
  - Impact: Debugging, performance optimization

## 🟢 Low Priority (New Features & Enhancements)

### Platform Support
- [x] **Add Kick platform listener** ✅ COMPLETE
  - Location: `services/kick-listener/`
  - Status: Fully implemented with Pusher WebSocket Protocol 7
  - Features: Dynamic channel subscription, real-time message reception, auto-reconnection
  - Impact: Platform expansion achieved

- [x] **Add TikTok platform listener** ✅ COMPLETE (Beta)
  - Location: `services/tiktok-listener/`
  - Status: Beta implementation using unofficial TikTok-Live-Connector library (Node.js)
  - Features: Real-time chat capture, message deduplication, dynamic stream management
  - Note: Uses unofficial library, should be replaced when official API available
  - Impact: Platform expansion achieved (beta status)

### Frontend & UI
- [x] **Build React + Next.js frontend** ✅ COMPLETE
  - Location: `frontend/` (Next.js 14+ App Router)
  - Status: Fully implemented with TypeScript and Tailwind CSS
  - Features: Overlay display, admin dashboard, auth pages, settings, legal pages
  - Impact: Better user experience for streamers achieved

- [x] **Add overlay management API** ✅ COMPLETE
  - Location: `services/overlay-manager/handlers/`
  - Status: Fully implemented with CRUD operations
  - Features: Overlay config (config.go), source management (sources.go), mock chat (mock_message.go), YouTube helpers (youtube.go)
  - Impact: Self-service overlay configuration achieved

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

- **Total Tasks**: 18
- **High Priority**: 2 (Security critical)
- **Medium Priority**: 4 (Quality improvements)
- **Low Priority**: 3 (Scaling enhancements)
- **Completed**: 9 (AES-GCM encryption, Rate limiting, OpenTelemetry tracing, Kick listener, TikTok listener, Frontend, Overlay API, display_name field)
- **Partially Complete**: 3 (YouTube tests, Unit tests, Prometheus metrics)

## Notes

- Extension (all-chat-extension) has no pending TODOs after recent bug fixes
- **Major completions (2026-01-15)**:
  - ✅ All 4 platform listeners implemented (Twitch, YouTube, Kick, TikTok)
  - ✅ Frontend fully built with React + Next.js
  - ✅ Overlay management API complete
  - ✅ OpenTelemetry distributed tracing implemented
  - ✅ AES-GCM encryption for OAuth tokens (2026-01-08)
  - ✅ Rate limiting in API Gateway (2026-01-08)
- Previous fixes:
  - ✅ OAuth login bug fixed
  - ✅ Page layout preservation implemented
  - ✅ JWT middleware corrected to support viewer tokens
  - ✅ Error handling improved
  - ✅ display_name field exists in users table (initial schema)
