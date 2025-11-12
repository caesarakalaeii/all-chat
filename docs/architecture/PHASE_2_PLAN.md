# Phase 2: Infrastructure Services - Detailed Plan

**Version**: 1.0
**Created**: 2025-11-12
**Duration**: 2 weeks (Nov 12 - Nov 26)
**Priority**: P0 (blocks Twitch real-time)

---

## Overview

Phase 2 builds the infrastructure services needed for real-time chat aggregation:
1. **Emote Service** - Cache emotes from 7TV, BTTV, FFZ
2. **API Gateway** - HTTP reverse proxy for service communication

These services are **prerequisites** for Phase 3 (Twitch real-time).

---

## Service 1: Emote Service

### Purpose
Fetch and cache third-party emotes to enrich chat messages with emote metadata and URLs.

### Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ GET /emotes/channel/xqc
       ▼
┌─────────────────────┐
│   Emote Service     │
│   (Port 8083)       │
└──────┬──────────────┘
       │
       ├─── Check Redis Cache
       │    └─── emotes:7tv:xqc
       │    └─── emotes:bttv:xqc
       │    └───emotes:ffz:xqc
       │
       └─── (If cache miss) Fetch from APIs
            ├─── 7TV API
            ├─── BTTV API
            └─── FFZ API
```

### Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/emotes/channel/:channel` | All emotes for channel (aggregated) | No |
| GET | `/emotes/7tv/:channel` | 7TV emotes only | No |
| GET | `/emotes/bttv/:channel` | BTTV emotes only | No |
| GET | `/emotes/ffz/:channel` | FFZ emotes only | No |
| GET | `/health/live` | Liveness probe | No |
| GET | `/health/ready` | Readiness (Redis check) | No |

### Data Model

```go
type Emote struct {
    Code     string `json:"code"`     // e.g., "Kappa"
    URL      string `json:"url"`      // Image URL
    Provider string `json:"provider"` // "7tv", "bttv", "ffz", "twitch"
    Channel  string `json:"channel"`  // Channel name
}

type EmoteResponse struct {
    Channel string  `json:"channel"`
    Emotes  []Emote `json:"emotes"`
}
```

### Redis Caching Strategy

**Key Format**: `emotes:{provider}:{channel}`

**Example**:
```
emotes:7tv:xqc       → [{"code":"OMEGALUL","url":"...","provider":"7tv"}]
emotes:bttv:xqc      → [{"code":"widepeepoHappy","url":"...","provider":"bttv"}]
emotes:ffz:xqc       → [{"code":"xqcL","url":"...","provider":"ffz"}]
```

**TTL**: 1 hour (emotes don't change frequently)

**Cache Invalidation**:
- Automatic via TTL
- Manual via `/admin/cache/clear/:channel` (future)

### External API Details

#### 7TV API
- **Endpoint**: `GET https://7tv.io/v3/users/twitch/{channel_id}/emotes`
- **Rate Limit**: ~10 req/s
- **Response**: Array of emote objects

#### BTTV API
- **Endpoint**: `GET https://api.betterttv.net/3/cached/users/twitch/{channel_id}`
- **Rate Limit**: ~20 req/s
- **Response**: `channelEmotes` + `sharedEmotes`

#### FFZ API
- **Endpoint**: `GET https://api.frankerfacez.com/v1/room/{channel}`
- **Rate Limit**: ~10 req/s
- **Response**: `sets` object with emote arrays

### File Structure

```
services/emote-service/
├── cmd/
│   └── main.go                # Entry point, wiring
├── handlers/
│   ├── emote.go               # HTTP handlers
│   ├── emote_test.go          # Handler tests
│   └── health.go              # Health check handlers
├── clients/
│   ├── seventv.go             # 7TV API client
│   ├── bttv.go                # BTTV API client
│   ├── ffz.go                 # FFZ API client
│   ├── clients_test.go        # Client unit tests
│   └── mock.go                # Mock clients for testing
├── cache/
│   ├── emote_cache.go         # Redis cache wrapper
│   └── emote_cache_test.go    # Cache tests
├── models/
│   ├── emote.go               # Emote domain model
│   └── emote_test.go          # Model validation tests
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Day 1-2: Setup & Models
- [ ] Initialize `emote-service` module
- [ ] Create `models/emote.go` with validation
- [ ] Write table-driven tests for models
- [ ] Tests pass ✅

#### Day 3-4: External API Clients
- [ ] Implement `clients/seventv.go`
- [ ] Implement `clients/bttv.go`
- [ ] Implement `clients/ffz.go`
- [ ] Write unit tests with HTTP mocks
- [ ] Handle timeouts and errors gracefully
- [ ] Tests pass ✅

#### Day 5: Redis Caching
- [ ] Implement `cache/emote_cache.go`
- [ ] Cache GET with fallback to API
- [ ] Cache SET with TTL
- [ ] Write tests with Redis mock
- [ ] Tests pass ✅

#### Day 6-7: HTTP Handlers
- [ ] Implement `handlers/emote.go`
- [ ] Implement aggregation logic (combine all providers)
- [ ] Implement health checks
- [ ] Write integration tests with Testcontainers (Redis)
- [ ] Wire everything in `cmd/main.go`
- [ ] Manual testing with `curl`
- [ ] All tests pass ✅

### Success Criteria

- [ ] Can fetch emotes from all 3 providers
- [ ] Redis caching works (verify with `redis-cli`)
- [ ] Cache hit rate > 95% after warmup
- [ ] Handles API failures gracefully (returns cached data or empty array)
- [ ] Response time < 100ms (p95) for cached requests
- [ ] Response time < 2s (p95) for cache misses
- [ ] Test coverage ≥ 80%
- [ ] Docker build succeeds
- [ ] Service runs in Docker Compose

### Testing Strategy

**Unit Tests**:
```go
func TestSevenTVClient_FetchEmotes(t *testing.T) {
    tests := []struct {
        name        string
        channel     string
        mockStatus  int
        mockBody    string
        wantEmotes  int
        wantErr     bool
    }{
        {"valid channel", "xqc", 200, `[{"code":"OMEGALUL"}]`, 1, false},
        {"not found", "nonexistent", 404, ``, 0, true},
        {"api error", "xqc", 500, ``, 0, true},
    }
    // ...
}
```

**Integration Tests**:
```go
func TestEmoteService_Integration(t *testing.T) {
    // Start Redis with Testcontainers
    ctx := context.Background()
    redisC, err := testcontainers.GenericContainer(ctx, ...)

    // Test full flow: HTTP → Cache Miss → External API → Cache → HTTP
    // ...
}
```

---

## Service 2: API Gateway (HTTP Proxy)

### Purpose
Single entry point for all client HTTP requests. Routes to appropriate backend services.

### Architecture

```
┌──────────┐
│  Client  │
└────┬─────┘
     │ GET /api/v1/overlays
     ▼
┌──────────────────────┐
│   API Gateway        │
│   (Port 8080)        │
│   ┌───────────────┐  │
│   │  JWT Auth MW  │  │
│   │  CORS MW      │  │
│   │  Logging MW   │  │
│   │  Proxy Handler│  │
│   └───────────────┘  │
└──────┬───────────────┘
       │
       ├─── /api/v1/auth/*      → auth-service:8081
       ├─── /api/v1/overlays/*  → overlay-manager:8082
       └─── /api/v1/emotes/*    → emote-service:8083
```

### Routes

| Path | Target Service | Auth Required |
|------|----------------|---------------|
| `/api/v1/auth/login` | auth-service:8081 | No |
| `/api/v1/auth/callback` | auth-service:8081 | No |
| `/api/v1/auth/refresh` | auth-service:8081 | No |
| `/api/v1/auth/me` | auth-service:8081 | Yes |
| `/api/v1/auth/logout` | auth-service:8081 | Yes |
| `/api/v1/overlays` | overlay-manager:8082 | Yes |
| `/api/v1/overlays/:id` | overlay-manager:8082 | Yes |
| `/api/v1/overlays/:id/sources` | overlay-manager:8082 | Yes |
| `/api/v1/emotes/*` | emote-service:8083 | No |
| `/health` | api-gateway (aggregated) | No |

### Middleware Stack

```go
router := gin.Default()
router.Use(middleware.CORS())        // Allow frontend origin
router.Use(middleware.Logging())     // Log all requests

// Public routes (no auth)
public := router.Group("/api/v1")
public.POST("/auth/login", proxy.ForwardTo("auth-service:8081"))

// Protected routes (JWT required)
protected := router.Group("/api/v1")
protected.Use(middleware.JWTAuth())
protected.GET("/overlays", proxy.ForwardTo("overlay-manager:8082"))
```

### JWT Middleware

```go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Extract token from Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(401, gin.H{"error": "missing authorization header"})
            c.Abort()
            return
        }

        // 2. Validate token
        token := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := jwt.ValidateToken(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }

        // 3. Store user_id in context
        c.Set("user_id", claims.UserID)
        c.Next()
    }
}
```

### Health Aggregation

The `/health` endpoint checks all backend services and returns aggregated status:

```json
{
  "status": "healthy",  // "healthy", "degraded", "unhealthy"
  "services": {
    "auth-service": {
      "status": "up",
      "latency_ms": 5
    },
    "overlay-manager": {
      "status": "up",
      "latency_ms": 8
    },
    "emote-service": {
      "status": "up",
      "latency_ms": 3
    }
  },
  "timestamp": "2025-11-12T10:00:00Z"
}
```

### File Structure

```
services/api-gateway/
├── cmd/
│   └── main.go                # Entry point, wiring
├── handlers/
│   ├── proxy.go               # Reverse proxy handler
│   ├── proxy_test.go          # Proxy tests
│   ├── health.go              # Health aggregation
│   └── health_test.go         # Health tests
├── middleware/
│   ├── auth.go                # JWT middleware
│   ├── auth_test.go           # Auth MW tests
│   ├── cors.go                # CORS middleware
│   └── logging.go             # Request logging
├── models/
│   ├── health.go              # Health response model
│   └── service_config.go      # Service registry
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

### Implementation Checklist

#### Day 1-2: Proxy Setup
- [ ] Initialize `api-gateway` module
- [ ] Create `handlers/proxy.go` with reverse proxy logic
- [ ] Create service registry (map of routes → backend URLs)
- [ ] Write unit tests for proxy handler
- [ ] Tests pass ✅

#### Day 3: Middleware
- [ ] Implement `middleware/auth.go` (JWT validation)
- [ ] Implement `middleware/cors.go`
- [ ] Implement `middleware/logging.go`
- [ ] Write tests for each middleware
- [ ] Tests pass ✅

#### Day 4-5: Health & Integration
- [ ] Implement `handlers/health.go` (aggregate health checks)
- [ ] Wire everything in `cmd/main.go`
- [ ] Write integration tests (start all services in Docker)
- [ ] Test full E2E flow:
  - Login → JWT token
  - Create overlay (with token)
  - Fetch emotes (no token)
- [ ] All tests pass ✅

#### Day 6-7: Docker & Documentation
- [ ] Create Dockerfile
- [ ] Update `docker-compose.yml` with api-gateway
- [ ] Test all services via Gateway
- [ ] Update API documentation
- [ ] README with usage examples

### Success Criteria

- [ ] Routes all requests correctly to backend services
- [ ] JWT middleware validates tokens from auth-service
- [ ] Protected routes return 401 for invalid/missing tokens
- [ ] CORS allows frontend origin (configurable)
- [ ] Health endpoint shows all services
- [ ] Proxy overhead < 10ms (p95)
- [ ] Test coverage ≥ 80%
- [ ] Docker build succeeds
- [ ] All services accessible via Gateway in Docker Compose

### Testing Strategy

**Unit Tests**:
```go
func TestJWTMiddleware(t *testing.T) {
    tests := []struct {
        name       string
        token      string
        wantStatus int
    }{
        {"valid token", "Bearer eyJ...", 200},
        {"invalid token", "Bearer invalid", 401},
        {"missing token", "", 401},
    }
    // ...
}
```

**Integration Tests**:
```go
func TestGateway_E2E(t *testing.T) {
    // 1. Login via Gateway → Auth Service
    resp := httptest.POST("/api/v1/auth/login", body)
    token := resp.Body.Token

    // 2. Create Overlay via Gateway → Overlay Manager
    resp := httptest.POST("/api/v1/overlays",
        headers{"Authorization": "Bearer " + token})

    // 3. Verify overlay created
    // ...
}
```

---

## Phase 2 Integration Testing

### Test Environment Setup

```bash
# Start all services
cd /home/caesar/git/all-chat
make docker-up

# Should start:
# - postgres:16
# - redis:7
# - auth-service:8081
# - overlay-manager:8082
# - emote-service:8083
# - api-gateway:8080
```

### E2E Test Scenarios

#### Scenario 1: Login Flow
```bash
# 1. Start OAuth flow
curl http://localhost:8080/api/v1/auth/login
# → Redirects to Twitch

# 2. After OAuth callback, get token
TOKEN="eyJhbGci..."

# 3. Get current user
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/auth/me
# → {"id":"user-123","username":"testuser"}
```

#### Scenario 2: Overlay Management
```bash
# 1. Create overlay (requires auth)
curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name":"My Overlay"}' \
     http://localhost:8080/api/v1/overlays
# → {"id":"overlay-123","name":"My Overlay"}

# 2. List overlays
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/overlays
# → [{"id":"overlay-123","name":"My Overlay"}]
```

#### Scenario 3: Emote Fetching
```bash
# 1. Fetch emotes for channel (no auth)
curl http://localhost:8080/api/v1/emotes/channel/xqc
# → {"channel":"xqc","emotes":[...]}

# 2. Verify cached (fast response)
time curl http://localhost:8080/api/v1/emotes/channel/xqc
# → Should be < 100ms
```

#### Scenario 4: Health Check
```bash
curl http://localhost:8080/health
# → {"status":"healthy","services":{...}}
```

### Success Criteria

- [ ] All E2E scenarios pass
- [ ] Auth flow works end-to-end via Gateway
- [ ] Emote caching reduces external API calls by 90%+
- [ ] All services report healthy
- [ ] No errors in logs

---

## Docker Compose Updates

### New Services

```yaml
# deployments/docker-compose.yml

services:
  # ... existing services (postgres, redis, auth, overlay) ...

  emote-service:
    build:
      context: ../services/emote-service
      dockerfile: Dockerfile
    ports:
      - "8083:8083"
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - LOG_LEVEL=info
    depends_on:
      - redis
    networks:
      - all-chat

  api-gateway:
    build:
      context: ../services/api-gateway
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - AUTH_SERVICE_URL=http://auth-service:8081
      - OVERLAY_SERVICE_URL=http://overlay-manager:8082
      - EMOTE_SERVICE_URL=http://emote-service:8083
      - JWT_SECRET=${JWT_SECRET}
      - CORS_ORIGIN=http://localhost:3000
      - LOG_LEVEL=info
    depends_on:
      - auth-service
      - overlay-manager
      - emote-service
    networks:
      - all-chat

networks:
  all-chat:
    driver: bridge
```

---

## Makefile Updates

```makefile
# Build Phase 2 services
.PHONY: build-emote
build-emote:
	cd services/emote-service && go build -o emote-service ./cmd

.PHONY: build-gateway
build-gateway:
	cd services/api-gateway && go build -o api-gateway ./cmd

# Test Phase 2 services
.PHONY: test-emote
test-emote:
	cd services/emote-service && go test -v -cover ./...

.PHONY: test-gateway
test-gateway:
	cd services/api-gateway && go test -v -cover ./...

# Run Phase 2 services locally
.PHONY: run-emote
run-emote:
	cd services/emote-service && go run ./cmd

.PHONY: run-gateway
run-gateway:
	cd services/api-gateway && go run ./cmd
```

---

## Timeline

| Week | Days | Tasks | Owner |
|------|------|-------|-------|
| **Week 1** | Mon-Wed | Emote Service implementation | Backend |
| | Thu-Fri | API Gateway proxy setup | Backend |
| **Week 2** | Mon-Tue | Middleware & health checks | Backend |
| | Wed-Thu | Integration testing | Backend + QA |
| | Fri | Docker, docs, cleanup | Backend |

---

## Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **External API rate limits** | Medium | Medium | Implement backoff, cache aggressively |
| **7TV API changes** | Low | High | Abstract clients, add version checks |
| **JWT validation complexity** | Low | Medium | Reuse shared auth package |
| **Integration test flakiness** | Medium | Low | Use Testcontainers, retry logic |

---

## Definition of Done

Phase 2 is complete when:

- [ ] Emote Service deployed and tested
- [ ] API Gateway deployed and tested
- [ ] All unit tests passing (80%+ coverage)
- [ ] All integration tests passing
- [ ] Docker Compose updated with new services
- [ ] Documentation updated (README, API docs)
- [ ] Manual E2E test scenarios verified
- [ ] Ready to start Phase 3 (Twitch Listener)

---

**Next Phase**: [Phase 3: Twitch Real-Time](./PHASE_3_PLAN.md) (Twitch Listener + Message Processor + WebSocket)
