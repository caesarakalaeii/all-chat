# Admin Dashboard Implementation Plan

## ✅ Completed

### Frontend (100% Complete)
- ✅ Admin layout with navigation (`/admin/layout.tsx`)
- ✅ Dashboard home page (`/admin/page.tsx`)
- ✅ Users management page (`/admin/users/page.tsx`)
- ✅ Overlays management page (`/admin/overlays/page.tsx`)
- ✅ Sources management page (`/admin/sources/page.tsx`)
- ✅ Professional UI with Tailwind CSS
- ✅ TypeScript type safety
- ✅ Builds successfully
- ✅ Mock data for demonstration

### Backend (Partially Complete)
- ✅ Admin handler created (`services/auth-service/handlers/admin.go`)
- ✅ Repository methods added:
  - `GetAllUsers(ctx) ([]*models.User, error)`
  - `GetUserByID(ctx, userID) (*models.User, error)`
  - `scanUserFromRows(rows) (*models.User, error)`

## 🚧 Remaining Work

### 1. Auth Service - Register Admin Routes

Add to `services/auth-service/cmd/main.go` after line 172:

```go
// Create admin handler
adminHandler := handlers.NewAdminHandler(userRepo, log)
```

Then after the protected routes (around line 230), add:

```go
// Admin routes (JWT protected - admin role check needed)
admin := router.Group("/admin")
admin.Use(middleware.JWTAuth(jwtSecret))
// TODO: Add middleware.AdminOnly() check
{
	admin.GET("/users", adminHandler.ListUsers)
	admin.GET("/users/:id", adminHandler.GetUser)
}
```

### 2. Overlay Manager - Add Admin Endpoints

Create `services/overlay-manager/handlers/admin.go`:

```go
package handlers

import (
	"net/http"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminHandler struct {
	overlayRepo *repository.OverlayRepository
	sourceRepo  *repository.SourceRepository
	logger      *zap.Logger
}

func NewAdminHandler(overlayRepo *repository.OverlayRepository, sourceRepo *repository.SourceRepository, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		overlayRepo: overlayRepo,
		sourceRepo:  sourceRepo,
		logger:      logger,
	}
}

// ListOverlays returns all overlays in the system
// GET /admin/overlays
func (h *AdminHandler) ListOverlays(c *gin.Context) {
	overlays, err := h.overlayRepo.GetAllOverlays(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to fetch overlays", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch overlays"})
		return
	}

	// For each overlay, get source count
	type OverlayResponse struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		UserID       string `json:"user_id"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		SourcesCount int    `json:"sources_count"`
	}

	response := make([]OverlayResponse, len(overlays))
	for i, overlay := range overlays {
		sources, _ := h.sourceRepo.GetByOverlayID(c.Request.Context(), overlay.ID)
		response[i] = OverlayResponse{
			ID:           overlay.ID,
			Name:         overlay.Name,
			UserID:       overlay.UserID,
			CreatedAt:    overlay.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    overlay.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			SourcesCount: len(sources),
		}
	}

	c.JSON(http.StatusOK, response)
}

// ListAllSources returns all sources across all overlays
// GET /admin/sources
func (h *AdminHandler) ListAllSources(c *gin.Context) {
	sources, err := h.sourceRepo.GetAllSources(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to fetch sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sources"})
		return
	}

	// Include overlay information for each source
	type SourceResponse struct {
		ID          string `json:"id"`
		OverlayID   string `json:"overlay_id"`
		OverlayName string `json:"overlay_name"`
		Platform    string `json:"platform"`
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
	}

	response := make([]SourceResponse, 0)
	for _, source := range sources {
		overlay, err := h.overlayRepo.GetByID(c.Request.Context(), source.OverlayID)
		if err != nil {
			continue // Skip if overlay not found
		}

		response = append(response, SourceResponse{
			ID:          source.ID,
			OverlayID:   source.OverlayID,
			OverlayName: overlay.Name,
			Platform:    source.Platform,
			ChannelID:   source.ChannelID,
			ChannelName: source.ChannelName,
			IsActive:    source.IsActive,
			CreatedAt:   source.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, response)
}
```

### 3. Add Repository Methods

In `services/overlay-manager/repository/overlay_repository.go`, add:

```go
// GetAllOverlays returns all overlays (admin only)
func (r *OverlayRepository) GetAllOverlays(ctx context.Context) ([]*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, created_at, updated_at
		FROM overlays
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query overlays: %w", err)
	}
	defer rows.Close()

	var overlays []*models.Overlay
	for rows.Next() {
		var overlay models.Overlay
		if err := rows.Scan(&overlay.ID, &overlay.UserID, &overlay.Name, &overlay.CreatedAt, &overlay.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan overlay: %w", err)
		}
		overlays = append(overlays, &overlay)
	}

	return overlays, nil
}
```

In `services/overlay-manager/repository/source_repository.go`, add:

```go
// GetAllSources returns all sources across all overlays (admin only)
func (r *SourceRepository) GetAllSources(ctx context.Context) ([]*models.Source, error) {
	query := `
		SELECT id, overlay_id, platform, channel_id, channel_name, is_active, created_at, updated_at
		FROM chat_sources
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sources: %w", err)
	}
	defer rows.Close()

	var sources []*models.Source
	for rows.Next() {
		var source models.Source
		if err := rows.Scan(&source.ID, &source.OverlayID, &source.Platform, &source.ChannelID, &source.ChannelName, &source.IsActive, &source.CreatedAt, &source.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sources = append(sources, &source)
	}

	return sources, nil
}
```

### 4. Register Routes in Overlay Manager

In `services/overlay-manager/cmd/main.go`, add:

```go
// Create admin handler
adminHandler := handlers.NewAdminHandler(overlayRepo, sourceRepo, log)

// Admin routes
admin := router.Group("/admin")
admin.Use(middleware.JWTAuth(jwtSecret))
// TODO: Add middleware.AdminOnly() check
{
	admin.GET("/overlays", adminHandler.ListOverlays)
	admin.GET("/sources", adminHandler.ListAllSources)
}
```

### 5. API Gateway - Add Admin Routes

In `services/api-gateway/cmd/main.go`, add to protected routes:

```go
// Admin routes (proxied to respective services)
protectedAPI.GET("/admin/users", proxyHandler.ForwardRequest)  // -> auth-service
protectedAPI.GET("/admin/users/:id", proxyHandler.ForwardRequest)
protectedAPI.GET("/admin/overlays", proxyHandler.ForwardRequest)  // -> overlay-manager
protectedAPI.GET("/admin/sources", proxyHandler.ForwardRequest)
```

### 6. Update Frontend to Use Real APIs

Update `frontend/src/app/admin/users/page.tsx`:

```typescript
// Replace mock data with:
const response = await fetch('/api/v1/admin/users', {
  headers: {
    'Authorization': `Bearer ${token}`,
  },
});
const data = await response.json();
setUsers(data);
```

Update `frontend/src/app/admin/overlays/page.tsx`:

```typescript
const response = await fetch('/api/v1/admin/overlays', {
  headers: {
    'Authorization': `Bearer ${token}`,
  },
});
```

Update `frontend/src/app/admin/sources/page.tsx`:

```typescript
const response = await fetch('/api/v1/admin/sources', {
  headers: {
    'Authorization': `Bearer ${token}`,
  },
});
```

### 7. Admin Authorization (Future Enhancement)

Create `shared/middleware/admin.go`:

```go
package middleware

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

// AdminOnly middleware checks if the user has admin role
// This requires adding an is_admin field to the users table
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from JWT claims (set by JWTAuth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// TODO: Check if user has admin role
		// For now, allow all authenticated users
		// In production, query database to check user.is_admin

		c.Next()
	}
}
```

## Testing

Once implemented, test the admin dashboard:

```bash
# Start all services
make docker-up

# Navigate to admin dashboard
open http://localhost:3000/admin

# Test endpoints directly
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/users
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/overlays
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/sources
```

## Deployment

After implementation:

```bash
# Build all services
make build

# Or build with Docker
docker-compose build auth-service overlay-manager api-gateway frontend

# Deploy to Kubernetes
kubectl apply -f deployments/k8s/
```

## Security Considerations

1. **Admin Role**: Add `is_admin` boolean column to `users` table
2. **Middleware**: Implement `AdminOnly()` middleware to check admin role
3. **Audit Logging**: Log all admin actions for security
4. **Rate Limiting**: Add rate limiting to admin endpoints
5. **IP Whitelisting**: Consider restricting admin access by IP

## Summary

**Status**: Frontend complete, backend 40% complete

**Estimated Time to Complete**: 2-3 hours

**Priority Items**:
1. Add repository methods (30 min)
2. Create admin handlers (30 min)
3. Register routes (20 min)
4. Update frontend API calls (20 min)
5. Testing (30 min)
