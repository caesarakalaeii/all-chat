# 🎬 3-Hour Coding Stream Guide: Overlay Manager Service

**Goal**: Build Overlay Manager microservice using Test-Driven Development (TDD)

**Status**: ✅ All tests pre-written - You implement code to make them pass!

---

## 📋 Quick Start Checklist

Before stream:
- [ ] Open IDE to `/home/caesar/git/all-chat/services/overlay-manager/`
- [ ] Have Docker running (`make docker-up` in separate terminal)
- [ ] Have database migrated (`make migrate`)
- [ ] Auth Service working (test at http://localhost:8081/health/live)
- [ ] Postman/Bruno ready for manual API testing

---

## ⏱️ Timeline (3 Hours)

### Hour 1: Models + Repository (90 min total)

#### Task 1: Overlay Model (30 min)
**File to create**: `models/overlay.go`

**What to implement**:
```go
package models

import (
	"errors"
	"time"
)

type Overlay struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TODO: Implement Validate() method
// Requirements:
// - UserID cannot be empty
// - Name must be 1-100 characters
// - Description max 500 characters (optional field)
func (o *Overlay) Validate() error {
	// YOUR CODE HERE
	return nil
}
```

**Test file**: `models/overlay_test.go` ✅ Already written
**Run tests**: `go test ./models/... -v`
**Expected**: Tests FAIL (red) - that's good!

**Stream tip**: Show viewers the failing tests, explain what needs to be implemented

---

#### Task 2: ChatSource Model (20 min)
**File to create**: `models/chat_source.go`

**What to implement**:
```go
package models

import "time"

type ChatSource struct {
	ID           string                 `json:"id"`
	OverlayID    string                 `json:"overlay_id"`
	Platform     string                 `json:"platform"`      // "twitch", "youtube", "kick", "tiktok"
	ChannelID    string                 `json:"channel_id"`
	ChannelName  string                 `json:"channel_name"`
	AuthRequired bool                   `json:"auth_required"`
	Config       map[string]interface{} `json:"config"`
	IsActive     bool                   `json:"is_active"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// TODO: Implement Validate() method
// Requirements:
// - OverlayID cannot be empty
// - Platform must be one of: "twitch", "youtube", "kick", "tiktok"
// - ChannelID cannot be empty
// - ChannelName cannot be empty
func (c *ChatSource) Validate() error {
	// YOUR CODE HERE
	return nil
}

// TODO: Implement IsValidPlatform() helper
func (c *ChatSource) IsValidPlatform() bool {
	// YOUR CODE HERE
	return false
}
```

**Test file**: `models/chat_source_test.go` ✅ Already written
**Run tests**: `go test ./models/... -v`

---

#### Task 3: Overlay Repository (40 min)
**File to create**: `repository/overlay_repo.go`

**What to implement**:
```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOverlayNotFound = errors.New("overlay not found")
	ErrUnauthorized    = errors.New("unauthorized")
)

type OverlayRepository struct {
	pool *pgxpool.Pool
}

func NewOverlayRepository(connString string) (*OverlayRepository, error) {
	// TODO: Create PostgreSQL connection pool
	// Use shared/database package helper
	return &OverlayRepository{}, nil
}

// TODO: Implement Create
// Requirements:
// - Generate UUID for overlay.ID
// - Set CreatedAt and UpdatedAt
// - INSERT into overlays table
// - Return error if validation fails
func (r *OverlayRepository) Create(ctx context.Context, overlay *models.Overlay) error {
	// YOUR CODE HERE
	return nil
}

// TODO: Implement GetByID
func (r *OverlayRepository) GetByID(ctx context.Context, id string) (*models.Overlay, error) {
	// YOUR CODE HERE
	return nil, ErrOverlayNotFound
}

// TODO: Implement GetByIDAndUserID (for authorization check)
// This ensures users can only access their own overlays
func (r *OverlayRepository) GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error) {
	// YOUR CODE HERE
	return nil, ErrOverlayNotFound
}

// TODO: Implement ListByUserID
func (r *OverlayRepository) ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error) {
	// YOUR CODE HERE
	return []*models.Overlay{}, nil
}

// TODO: Implement Update
func (r *OverlayRepository) Update(ctx context.Context, overlay *models.Overlay) error {
	// YOUR CODE HERE
	return nil
}

// TODO: Implement Delete
func (r *OverlayRepository) Delete(ctx context.Context, id string) error {
	// YOUR CODE HERE
	return nil
}
```

**Test file**: `repository/overlay_repo_test.go` ✅ Already written (uses Testcontainers!)
**Run tests**: `go test ./repository/... -v`

**Important**: Testcontainers will start a real PostgreSQL instance. Make sure Docker is running!

---

### Hour 2: HTTP Handlers (60 min)

#### Task 4: Overlay Handlers (60 min)
**File to create**: `handlers/overlay.go`

**What to implement**:
```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OverlayHandler struct {
	repo   OverlayRepository
	logger *zap.Logger
}

// OverlayRepository interface (for testing)
type OverlayRepository interface {
	Create(ctx context.Context, overlay *models.Overlay) error
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error)
	ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error)
	Update(ctx context.Context, overlay *models.Overlay) error
	Delete(ctx context.Context, id string) error
}

func NewOverlayHandler(repo OverlayRepository) *OverlayHandler {
	return &OverlayHandler{
		repo:   repo,
		logger: logger.NewLogger("overlay-handler", "info"),
	}
}

// TODO: Implement HandleCreateOverlay
// Requirements:
// - Extract user_id from context (c.Get("user_id"))
// - Parse JSON request body (name, description)
// - Create overlay with user_id
// - Validate overlay
// - Call repo.Create()
// - Return 201 with created overlay
func (h *OverlayHandler) HandleCreateOverlay(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement HandleListOverlays
func (h *OverlayHandler) HandleListOverlays(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement HandleGetOverlay
// Requirements:
// - Extract overlay ID from URL param (c.Param("id"))
// - Extract user_id from context
// - Call repo.GetByIDAndUserID() (authorization check!)
// - Return 200 with overlay
func (h *OverlayHandler) HandleGetOverlay(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement HandleUpdateOverlay
func (h *OverlayHandler) HandleUpdateOverlay(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement HandleDeleteOverlay
func (h *OverlayHandler) HandleDeleteOverlay(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement RegisterRoutes
func (h *OverlayHandler) RegisterRoutes(router *gin.Engine) {
	// YOUR CODE HERE
	// Routes:
	// POST   /overlays      - Create
	// GET    /overlays      - List
	// GET    /overlays/:id  - Get
	// PUT    /overlays/:id  - Update
	// DELETE /overlays/:id  - Delete
}
```

**Test file**: `handlers/overlay_test.go` ✅ Already written
**Run tests**: `go test ./handlers/... -v -run TestOverlayHandler`

---

#### Task 5: Health Handlers (15 min)
**File to create**: `handlers/health.go`

**What to implement**:
```go
package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// DBHealthChecker interface for database health checks
type DBHealthChecker interface {
	Ping(context.Context) error
}

// RedisHealthChecker interface for Redis health checks
type RedisHealthChecker interface {
	Ping(context.Context) *redis.StatusCmd
}

type HealthHandler struct {
	db    DBHealthChecker
	redis RedisHealthChecker
}

func NewHealthHandler(db DBHealthChecker, redis RedisHealthChecker) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// TODO: Implement HandleLiveness
// Always return 200 OK with {"status": "alive"}
func (h *HealthHandler) HandleLiveness(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement HandleReadiness
// Check database.Ping() and redis.Ping()
// Return 200 if both healthy, 503 if either fails
func (h *HealthHandler) HandleReadiness(c *gin.Context) {
	// YOUR CODE HERE
}

// TODO: Implement RegisterRoutes
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	// YOUR CODE HERE
	// GET /health/live
	// GET /health/ready
}
```

**Test file**: `handlers/health_test.go` ✅ Already written
**Run tests**: `go test ./handlers/... -v -run TestHealthHandler`

---

### Hour 3: Integration (60 min)

#### Task 6: Main Entry Point (30 min)
**File to create**: `cmd/main.go`

**What to implement**:
```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/handlers"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// TODO: Initialize logger
	log := logger.NewLogger("overlay-manager", os.Getenv("LOG_LEVEL"))
	defer log.Sync()

	// TODO: Build database connection string
	// Format: postgresql://user:pass@host:port/dbname

	// TODO: Connect to PostgreSQL

	// TODO: Connect to Redis

	// TODO: Create repository

	// TODO: Create handlers

	// TODO: Set up Gin router
	router := gin.Default()
	router.Use(middleware.CORS())

	// TODO: Register routes

	// TODO: Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// TODO: Implement graceful shutdown
	// - Start server in goroutine
	// - Listen for SIGINT/SIGTERM
	// - Shutdown with 25-second timeout

	log.Info("Overlay Manager started", zap.String("port", port))
}
```

---

#### Task 7: Dockerfile (10 min)
**File to create**: `Dockerfile`

Copy from auth-service and modify:
```dockerfile
# YOUR CODE HERE
# Hint: Multi-stage build
# Stage 1: Build
# Stage 2: Runtime (alpine, non-root user)
```

---

#### Task 8: Test End-to-End (20 min)

**Manual testing with curl/Postman**:

```bash
# 1. Start services
make docker-up

# 2. Login to get JWT token
curl http://localhost:8081/twitch/login
# Follow OAuth flow, get JWT token

# 3. Create overlay
curl -X POST http://localhost:8082/overlays \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My First Overlay", "description": "Test overlay"}'

# 4. List overlays
curl http://localhost:8082/overlays \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# 5. Get specific overlay
curl http://localhost:8082/overlays/OVERLAY_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# 6. Update overlay
curl -X PUT http://localhost:8082/overlays/OVERLAY_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Updated Name", "description": "New description"}'

# 7. Delete overlay
curl -X DELETE http://localhost:8082/overlays/OVERLAY_ID \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

---

## 🧪 Test Commands

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./models/...
go test -v ./repository/...  # Requires Docker for Testcontainers
go test -v ./handlers/...

# Run with coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Watch mode (if you have gotestsum)
gotestsum --watch
```

---

## 📊 Expected Test Results

### Before Implementation (Red):
```
FAIL models/overlay_test.go       - Validate() not implemented
FAIL models/chat_source_test.go   - Validate() not implemented
FAIL repository/overlay_repo_test.go - Functions not implemented
FAIL handlers/overlay_test.go     - Handlers not implemented
FAIL handlers/health_test.go      - Handlers not implemented
```

### After Implementation (Green):
```
PASS models/overlay_test.go       - ✅ 8/8 tests
PASS models/chat_source_test.go   - ✅ 13/13 tests
PASS repository/overlay_repo_test.go - ✅ 12/12 tests
PASS handlers/overlay_test.go     - ✅ 12/12 tests
PASS handlers/health_test.go      - ✅ 3/3 tests

Total: 48 tests, coverage ≥80%
```

---

## 💡 Implementation Tips

### Tip 1: Start Simple
Implement minimal code to make tests pass. Don't over-engineer!

### Tip 2: Use Auth Service as Reference
The auth-service in `services/auth-service/` has similar patterns:
- Check `models/user.go` for Validate() pattern
- Check `repository/user_repo.go` for pgx queries
- Check `handlers/auth.go` for Gin handler patterns

### Tip 3: Copy-Paste Patterns
These patterns are the same across services:
- Error handling: Log, then return HTTP error
- JWT extraction: `userID, exists := c.Get("user_id")`
- JSON binding: `c.ShouldBindJSON(&req)`
- Validation: Call `.Validate()` before database ops

### Tip 4: Testing Workflow
```bash
# Terminal 1: Watch tests
watch -n 2 go test ./...

# Terminal 2: Code
# Save file → tests re-run automatically
```

---

## 🎯 Stream Milestones

### Milestone 1 (30 min): ✅ Models Pass
- `models/overlay.go` and `models/chat_source.go` implemented
- All model tests green
- Viewers learn Go structs, validation, error handling

### Milestone 2 (90 min): ✅ Repository Pass
- `repository/overlay_repo.go` implemented
- Testcontainers in action (viewers see real PostgreSQL!)
- All repository tests green
- Viewers learn pgx, SQL queries, database patterns

### Milestone 3 (150 min): ✅ Handlers Pass
- `handlers/overlay.go` and `handlers/health.go` implemented
- All handler tests green
- Viewers learn Gin, HTTP patterns, JWT validation

### Milestone 4 (180 min): ✅ E2E Working
- `cmd/main.go` wired up
- Docker Compose running
- Manual API testing successful
- **Victory! Phase 1 complete (2/8 services done)**

---

## 🚨 Common Issues & Solutions

### Issue 1: Tests can't find packages
**Solution**: Run `go mod tidy` in `services/overlay-manager/`

### Issue 2: Testcontainers fails
**Solution**: Ensure Docker is running: `docker ps`

### Issue 3: Database connection fails
**Solution**: Check `.env` file exists and has correct credentials

### Issue 4: Imports not resolving
**Solution**: Check `go.work` includes overlay-manager

---

## 📈 Success Criteria

At end of stream:
- ✅ All 48+ tests passing
- ✅ Coverage ≥ 80%
- ✅ Service runs in Docker Compose
- ✅ Can create/list/update/delete overlays via API
- ✅ Auth + Overlay services communicate (JWT validation)
- ✅ Phase 1 foundation complete!

---

## 🎁 Bonus Challenges (If Time Remains)

- [ ] Add chat source repository and handlers
- [ ] Add config repository (overlay display settings)
- [ ] Add Prometheus metrics middleware
- [ ] Write E2E test that spans both services
- [ ] Deploy to Kubernetes (single node)

---

## 📝 Notes Section (Use During Stream)

**Timecode | What you learned**
```
00:00 - Stream start, explain TDD approach
00:15 - First test fails (good!), start implementing
00:30 - Overlay model done
00:45 - ChatSource model done
01:30 - Repository with Testcontainers working
02:30 - All handlers implemented
03:00 - E2E test with real API calls - SUCCESS!
```

---

**Good luck with your stream! Remember: Red → Green → Refactor** 🎯
