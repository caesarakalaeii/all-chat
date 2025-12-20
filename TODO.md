# All-Chat TODO Tracker

**Last Updated**: 2025-12-20

## 🔴 High Priority (Security & Critical)

### Security Improvements
- [ ] **Implement AES-GCM encryption for OAuth tokens**
  - Location: Token storage across all services
  - Current: Basic encryption
  - Impact: Critical security improvement
  - Reference: `CLAUDE.md:440`

- [ ] **Implement rate limiting**
  - Location: API Gateway
  - Current: No rate limiting
  - Impact: Prevent abuse, protect resources
  - Reference: `CLAUDE.md:441`, `CLAUDE.md:456`

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

- [ ] **Implement API Gateway rate limiting**
  - Impact: Prevent abuse, protect backend services

---

## Summary Statistics

- **Total Tasks**: 19
- **High Priority**: 3 (Security critical)
- **Medium Priority**: 8 (Quality & features)
- **Low Priority**: 8 (Enhancements & scaling)

## Notes

- Extension (all-chat-extension) has no pending TODOs after recent bug fixes
- Recent fixes completed:
  - ✅ OAuth login bug fixed
  - ✅ Page layout preservation implemented
  - ✅ JWT middleware corrected to support viewer tokens
  - ✅ Error handling improved
