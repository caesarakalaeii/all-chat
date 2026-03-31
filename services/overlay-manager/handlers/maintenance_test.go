package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockMaintenanceRepository implements MaintenanceRepository for testing.
type mockMaintenanceRepository struct {
	createFunc       func(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error)
	listAllFunc      func(ctx context.Context) ([]models.MaintenanceWindow, error)
	listUpcomingFunc func(ctx context.Context) ([]models.MaintenanceWindow, error)
	deleteFunc       func(ctx context.Context, id string) error
}

func (m *mockMaintenanceRepository) Create(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req, createdBy)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMaintenanceRepository) ListAll(ctx context.Context) ([]models.MaintenanceWindow, error) {
	if m.listAllFunc != nil {
		return m.listAllFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMaintenanceRepository) ListUpcoming(ctx context.Context) ([]models.MaintenanceWindow, error) {
	if m.listUpcomingFunc != nil {
		return m.listUpcomingFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMaintenanceRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return errors.New("not implemented")
}

func newTestMaintenanceHandler(repo MaintenanceRepository) *MaintenanceHandler {
	logger, _ := zap.NewDevelopment()
	return NewMaintenanceHandler(repo, logger)
}

func TestHandleCreateMaintenance(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	startsAt := now.Add(1 * time.Hour)
	endsAt := now.Add(2 * time.Hour)

	t.Run("success", func(t *testing.T) {
		createdWindow := &models.MaintenanceWindow{
			ID:          uuid.New().String(),
			Title:       "Planned downtime",
			Description: "Upgrading database",
			StartsAt:    startsAt,
			EndsAt:      endsAt,
			CreatedBy:   "user-123",
			CreatedAt:   now,
		}

		repo := &mockMaintenanceRepository{
			createFunc: func(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error) {
				assert.Equal(t, "Planned downtime", req.Title)
				assert.Equal(t, "user-123", createdBy)
				return createdWindow, nil
			},
		}

		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(models.CreateMaintenanceRequest{
			Title:       "Planned downtime",
			Description: "Upgrading database",
			StartsAt:    startsAt,
			EndsAt:      endsAt,
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/maintenance", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		r.POST("/admin/maintenance", func(c *gin.Context) {
			c.Set("user_id", "user-123")
			handler.HandleCreateMaintenance(c)
		})
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusCreated, w.Code)

		var result models.MaintenanceWindow
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		assert.Equal(t, createdWindow.ID, result.ID)
		assert.Equal(t, "Planned downtime", result.Title)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		repo := &mockMaintenanceRepository{}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodPost, "/admin/maintenance", bytes.NewReader([]byte(`{invalid`)))
		c.Request.Header.Set("Content-Type", "application/json")

		r.POST("/admin/maintenance", func(c *gin.Context) {
			c.Set("user_id", "user-123")
			handler.HandleCreateMaintenance(c)
		})
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("starts_at not before ends_at", func(t *testing.T) {
		repo := &mockMaintenanceRepository{}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(models.CreateMaintenanceRequest{
			Title:    "Bad window",
			StartsAt: endsAt,
			EndsAt:   startsAt,
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/maintenance", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		r.POST("/admin/maintenance", func(c *gin.Context) {
			c.Set("user_id", "user-123")
			handler.HandleCreateMaintenance(c)
		})
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "starts_at must be before ends_at")
	})

	t.Run("missing user_id", func(t *testing.T) {
		repo := &mockMaintenanceRepository{}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(models.CreateMaintenanceRequest{
			Title:    "Window",
			StartsAt: startsAt,
			EndsAt:   endsAt,
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/maintenance", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		r.POST("/admin/maintenance", func(c *gin.Context) {
			// no user_id set
			handler.HandleCreateMaintenance(c)
		})
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			createFunc: func(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error) {
				return nil, errors.New("db connection lost")
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		body, _ := json.Marshal(models.CreateMaintenanceRequest{
			Title:    "Window",
			StartsAt: startsAt,
			EndsAt:   endsAt,
		})
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/maintenance", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		r.POST("/admin/maintenance", func(c *gin.Context) {
			c.Set("user_id", "user-123")
			handler.HandleCreateMaintenance(c)
		})
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleListMaintenance(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		windows := []models.MaintenanceWindow{
			{ID: uuid.New().String(), Title: "Window 1"},
			{ID: uuid.New().String(), Title: "Window 2"},
		}

		repo := &mockMaintenanceRepository{
			listAllFunc: func(ctx context.Context) ([]models.MaintenanceWindow, error) {
				return windows, nil
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodGet, "/admin/maintenance", nil)

		r.GET("/admin/maintenance", handler.HandleListMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)

		var result []models.MaintenanceWindow
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		assert.Len(t, result, 2)
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			listAllFunc: func(ctx context.Context) ([]models.MaintenanceWindow, error) {
				return nil, errors.New("db error")
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodGet, "/admin/maintenance", nil)

		r.GET("/admin/maintenance", handler.HandleListMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleDeleteMaintenance(t *testing.T) {
	validID := uuid.New().String()

	t.Run("success", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			deleteFunc: func(ctx context.Context, id string) error {
				assert.Equal(t, validID, id)
				return nil
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodDelete, "/admin/maintenance/"+validID, nil)

		r.DELETE("/admin/maintenance/:id", handler.HandleDeleteMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		repo := &mockMaintenanceRepository{}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodDelete, "/admin/maintenance/not-a-uuid", nil)

		r.DELETE("/admin/maintenance/:id", handler.HandleDeleteMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "valid UUID")
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			deleteFunc: func(ctx context.Context, id string) error {
				return repository.ErrMaintenanceNotFound
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodDelete, "/admin/maintenance/"+validID, nil)

		r.DELETE("/admin/maintenance/:id", handler.HandleDeleteMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "not found")
	})

	t.Run("database error returns 500", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			deleteFunc: func(ctx context.Context, id string) error {
				return errors.New("connection refused")
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodDelete, "/admin/maintenance/"+validID, nil)

		r.DELETE("/admin/maintenance/:id", handler.HandleDeleteMaintenance)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Failed to delete")
	})
}

func TestHandleListUpcoming(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		windows := []models.MaintenanceWindow{
			{ID: uuid.New().String(), Title: "Upcoming 1"},
		}

		repo := &mockMaintenanceRepository{
			listUpcomingFunc: func(ctx context.Context) ([]models.MaintenanceWindow, error) {
				return windows, nil
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodGet, "/maintenance/upcoming", nil)

		r.GET("/maintenance/upcoming", handler.HandleListUpcoming)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)

		var result []models.MaintenanceWindow
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		assert.Len(t, result, 1)
		assert.Equal(t, "Upcoming 1", result[0].Title)
	})

	t.Run("returns empty array when no upcoming windows", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			listUpcomingFunc: func(ctx context.Context) ([]models.MaintenanceWindow, error) {
				return []models.MaintenanceWindow{}, nil
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodGet, "/maintenance/upcoming", nil)

		r.GET("/maintenance/upcoming", handler.HandleListUpcoming)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]", w.Body.String())
	})

	t.Run("repository error", func(t *testing.T) {
		repo := &mockMaintenanceRepository{
			listUpcomingFunc: func(ctx context.Context) ([]models.MaintenanceWindow, error) {
				return nil, errors.New("db error")
			},
		}
		handler := newTestMaintenanceHandler(repo)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, r := gin.CreateTestContext(w)

		c.Request = httptest.NewRequest(http.MethodGet, "/maintenance/upcoming", nil)

		r.GET("/maintenance/upcoming", handler.HandleListUpcoming)
		r.ServeHTTP(w, c.Request)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
