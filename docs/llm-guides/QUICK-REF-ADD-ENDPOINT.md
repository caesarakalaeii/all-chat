# Quick Reference: Add HTTP Endpoint

**Time Estimate**: 30-60 minutes | **Difficulty**: ⭐ Easy

**Goal**: Add a new HTTP endpoint to an existing service following All-Chat conventions.

---

## Step-by-Step Checklist

### 1. Choose Service and Route

- [ ] Identify which service owns this functionality
- [ ] Choose RESTful route (e.g., `GET /api/v1/overlays/:id`)
- [ ] Document expected request/response formats

### 2. Create Handler Function

**File**: `services/<service>/handlers/<feature>.go`

```go
package handlers

import (
    "net/http"
    "github.com/caesar/all-chat/services/<service>/models"
    "github.com/caesar/all-chat/services/<service>/repository"
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

type FeatureHandler struct {
    repo   *repository.FeatureRepository
    logger *zap.Logger
}

func NewFeatureHandler(repo *repository.FeatureRepository, logger *zap.Logger) *FeatureHandler {
    return &FeatureHandler{repo: repo, logger: logger}
}

func (h *FeatureHandler) GetItem(c *gin.Context) {
    id := c.Param("id")

    // Validate input
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ID required"})
        return
    }

    // Call repository
    item, err := h.repo.GetByID(c.Request.Context(), id)
    if err != nil {
        h.logger.Error("Failed to get item", zap.Error(err), zap.String("id", id))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
        return
    }

    if item == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
        return
    }

    // Return success
    c.JSON(http.StatusOK, item)
}
```

### 3. Register Route

**File**: `services/<service>/cmd/main.go`

```go
// Initialize handler
featureRepo := repository.NewFeatureRepository(db)
featureHandler := handlers.NewFeatureHandler(featureRepo, log)

// Register routes
router := gin.Default()
v1 := router.Group("/api/v1")
{
    v1.GET("/items/:id", featureHandler.GetItem)
    v1.POST("/items", featureHandler.CreateItem)
    v1.PUT("/items/:id", featureHandler.UpdateItem)
    v1.DELETE("/items/:id", featureHandler.DeleteItem)
}
```

### 4. Add Tests

**File**: `services/<service>/handlers/<feature>_test.go`

```go
package handlers

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestGetItem(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Setup
    mockRepo := &MockFeatureRepository{}
    handler := NewFeatureHandler(mockRepo, zap.NewNop())

    // Create request
    router := gin.Default()
    router.GET("/items/:id", handler.GetItem)

    req := httptest.NewRequest("GET", "/items/123", nil)
    w := httptest.NewRecorder()

    // Execute
    router.ServeHTTP(w, req)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)
    // Add more assertions...
}
```

### 5. Test Locally

```bash
# Run tests
go test ./handlers/<feature>_test.go -v

# Start service
go run ./cmd/main.go

# Test endpoint with curl
curl http://localhost:<port>/api/v1/items/123

# Expected: {"id": "123", "name": "...", ...}
```

### 6. Add to Service README

**File**: `services/<service>/README.md`

Update "API Endpoints" section:
```markdown
### Feature Endpoints

\`\`\`bash
# Get item by ID
GET /api/v1/items/:id

# Create item
POST /api/v1/items
Body: { "name": "...", ... }

# Update item
PUT /api/v1/items/:id
Body: { "name": "...", ... }

# Delete item
DELETE /api/v1/items/:id
\`\`\`
```

---

## Common Patterns

### Authentication Required

```go
// Apply JWT middleware to protected routes
authenticated := v1.Group("/")
authenticated.Use(middleware.JWTAuth(jwtSecret))
{
    authenticated.POST("/overlays", overlayHandler.Create)
    authenticated.PUT("/overlays/:id", overlayHandler.Update)
}
```

### Request Body Validation

```go
type CreateItemRequest struct {
    Name        string `json:"name" binding:"required,min=1,max=255"`
    Description string `json:"description" binding:"max=1000"`
}

func (h *Handler) CreateItem(c *gin.Context) {
    var req CreateItemRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // ... process request
}
```

### Pagination

```go
func (h *Handler) ListItems(c *gin.Context) {
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
    offset := (page - 1) * limit

    items, total, _ := h.repo.List(c.Request.Context(), limit, offset)

    c.JSON(http.StatusOK, gin.H{
        "items": items,
        "total": total,
        "page": page,
        "limit": limit,
    })
}
```

---

## Testing Checklist

- [ ] Unit tests for handler functions
- [ ] Test happy path (200 OK)
- [ ] Test validation errors (400 Bad Request)
- [ ] Test not found (404)
- [ ] Test unauthorized (401) if auth required
- [ ] Test database errors (500)
- [ ] Run `go test ./handlers -v`
- [ ] Test with curl/Postman
- [ ] Check logs for errors

---

## Related Documentation

- [CLAUDE.md](../../CLAUDE.md#common-development-patterns) - Development patterns
- Service READMEs for examples (api-gateway, overlay-manager)

---

## Success Criteria

✅ Complete when:
1. Handler function implemented
2. Route registered in cmd/main.go
3. Tests written and passing
4. Endpoint tested manually with curl
5. Service README updated
