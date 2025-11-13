package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Mock repository for testing
type mockOverlayRepository struct {
	createFunc           func(context.Context, *models.Overlay) error
	getByIDFunc          func(context.Context, string) (*models.Overlay, error)
	getByIDAndUserIDFunc func(context.Context, string, string) (*models.Overlay, error)
	listByUserIDFunc     func(context.Context, string) ([]*models.Overlay, error)
	updateFunc           func(context.Context, *models.Overlay) error
	deleteFunc           func(context.Context, string) error
}

func (m *mockOverlayRepository) Create(ctx context.Context, overlay *models.Overlay) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, overlay)
	}
	return nil
}

func (m *mockOverlayRepository) GetByID(ctx context.Context, id string) (*models.Overlay, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOverlayRepository) GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error) {
	if m.getByIDAndUserIDFunc != nil {
		return m.getByIDAndUserIDFunc(ctx, id, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOverlayRepository) ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error) {
	if m.listByUserIDFunc != nil {
		return m.listByUserIDFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockOverlayRepository) Update(ctx context.Context, overlay *models.Overlay) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, overlay)
	}
	return nil
}

func (m *mockOverlayRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

// Test setup helper
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestOverlayHandler_HandleCreateOverlay(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		userID         string
		mockRepo       *mockOverlayRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful creation",
			requestBody: map[string]interface{}{
				"name":        "My Overlay",
				"description": "Test overlay",
			},
			userID: uuid.New().String(),
			mockRepo: &mockOverlayRepository{
				createFunc: func(ctx context.Context, overlay *models.Overlay) error {
					overlay.ID = uuid.New().String()
					return nil
				},
			},
			wantStatusCode: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotNil(t, response["id"])
				assert.Equal(t, "My Overlay", response["name"])
			},
		},
		{
			name: "missing name",
			requestBody: map[string]interface{}{
				"description": "Test overlay",
			},
			userID:         uuid.New().String(),
			mockRepo:       &mockOverlayRepository{},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "name too long",
			requestBody: map[string]interface{}{
				"name": "a very long name that exceeds the maximum allowed length of 100 characters for overlay names which should fail",
			},
			userID:         uuid.New().String(),
			mockRepo:       &mockOverlayRepository{},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "database error",
			requestBody: map[string]interface{}{
				"name": "My Overlay",
			},
			userID: uuid.New().String(),
			mockRepo: &mockOverlayRepository{
				createFunc: func(ctx context.Context, overlay *models.Overlay) error {
					return errors.New("database error")
				},
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "missing user_id in context (no auth)",
			requestBody: map[string]interface{}{
				"name": "My Overlay",
			},
			userID:         "", // No user ID
			mockRepo:       &mockOverlayRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewOverlayHandler(tt.mockRepo)

			router.POST("/overlays", func(c *gin.Context) {
				// Mock auth middleware - set user_id if provided
				if tt.userID != "" {
					c.Set("user_id", tt.userID)
				}
				handler.HandleCreateOverlay(c)
			})

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/overlays", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestOverlayHandler_HandleListOverlays(t *testing.T) {
	userID := uuid.New().String()

	tests := []struct {
		name           string
		userID         string
		mockRepo       *mockOverlayRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "successful list",
			userID: userID,
			mockRepo: &mockOverlayRepository{
				listByUserIDFunc: func(ctx context.Context, uid string) ([]*models.Overlay, error) {
					return []*models.Overlay{
						{
							ID:          uuid.New().String(),
							UserID:      uid,
							Name:        "Overlay 1",
							Description: "First overlay",
							IsActive:    true,
						},
						{
							ID:          uuid.New().String(),
							UserID:      uid,
							Name:        "Overlay 2",
							Description: "Second overlay",
							IsActive:    true,
						},
					}, nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response []map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, 2)
			},
		},
		{
			name:   "empty list",
			userID: userID,
			mockRepo: &mockOverlayRepository{
				listByUserIDFunc: func(ctx context.Context, uid string) ([]*models.Overlay, error) {
					return []*models.Overlay{}, nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response []map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, 0)
			},
		},
		{
			name:           "missing user_id (unauthorized)",
			userID:         "",
			mockRepo:       &mockOverlayRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewOverlayHandler(tt.mockRepo)

			router.GET("/overlays", func(c *gin.Context) {
				if tt.userID != "" {
					c.Set("user_id", tt.userID)
				}
				handler.HandleListOverlays(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/overlays", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestOverlayHandler_HandleGetOverlay(t *testing.T) {
	userID := uuid.New().String()
	overlayID := uuid.New().String()

	tests := []struct {
		name           string
		overlayID      string
		userID         string
		mockRepo       *mockOverlayRepository
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "successful fetch",
			overlayID: overlayID,
			userID:    userID,
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return &models.Overlay{
						ID:          id,
						UserID:      uid,
						Name:        "My Overlay",
						Description: "Test overlay",
						IsActive:    true,
					}, nil
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, overlayID, response["id"])
				assert.Equal(t, "My Overlay", response["name"])
			},
		},
		{
			name:      "overlay not found",
			overlayID: uuid.New().String(),
			userID:    userID,
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return nil, errors.New("overlay not found")
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "missing user_id (unauthorized)",
			overlayID:      overlayID,
			userID:         "",
			mockRepo:       &mockOverlayRepository{},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewOverlayHandler(tt.mockRepo)

			router.GET("/overlays/:id", func(c *gin.Context) {
				if tt.userID != "" {
					c.Set("user_id", tt.userID)
				}
				handler.HandleGetOverlay(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/overlays/"+tt.overlayID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestOverlayHandler_HandleUpdateOverlay(t *testing.T) {
	userID := uuid.New().String()
	overlayID := uuid.New().String()

	tests := []struct {
		name           string
		overlayID      string
		userID         string
		requestBody    map[string]interface{}
		mockRepo       *mockOverlayRepository
		wantStatusCode int
	}{
		{
			name:      "successful update",
			overlayID: overlayID,
			userID:    userID,
			requestBody: map[string]interface{}{
				"name":        "Updated Name",
				"description": "Updated description",
			},
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return &models.Overlay{
						ID:          id,
						UserID:      uid,
						Name:        "Old Name",
						Description: "Old description",
						IsActive:    true,
					}, nil
				},
				updateFunc: func(ctx context.Context, overlay *models.Overlay) error {
					return nil
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:      "overlay not found",
			overlayID: uuid.New().String(),
			userID:    userID,
			requestBody: map[string]interface{}{
				"name": "Updated Name",
			},
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return nil, errors.New("overlay not found")
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:      "invalid request body",
			overlayID: overlayID,
			userID:    userID,
			requestBody: map[string]interface{}{
				"name": "", // Empty name should fail validation
			},
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return &models.Overlay{
						ID:     id,
						UserID: uid,
						Name:   "Old Name",
					}, nil
				},
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:      "database update error",
			overlayID: overlayID,
			userID:    userID,
			requestBody: map[string]interface{}{
				"name": "Updated Name",
			},
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return &models.Overlay{
						ID:     id,
						UserID: uid,
						Name:   "Old Name",
					}, nil
				},
				updateFunc: func(ctx context.Context, overlay *models.Overlay) error {
					return errors.New("database error")
				},
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewOverlayHandler(tt.mockRepo)

			router.PUT("/overlays/:id", func(c *gin.Context) {
				if tt.userID != "" {
					c.Set("user_id", tt.userID)
				}
				handler.HandleUpdateOverlay(c)
			})

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/overlays/"+tt.overlayID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)
		})
	}
}

func TestOverlayHandler_HandleDeleteOverlay(t *testing.T) {
	userID := uuid.New().String()
	overlayID := uuid.New().String()

	tests := []struct {
		name           string
		overlayID      string
		userID         string
		mockRepo       *mockOverlayRepository
		wantStatusCode int
	}{
		{
			name:      "successful deletion",
			overlayID: overlayID,
			userID:    userID,
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return &models.Overlay{
						ID:     id,
						UserID: uid,
						Name:   "To Delete",
					}, nil
				},
				deleteFunc: func(ctx context.Context, id string) error {
					return nil
				},
			},
			wantStatusCode: http.StatusNoContent,
		},
		{
			name:      "overlay not found",
			overlayID: uuid.New().String(),
			userID:    userID,
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return nil, errors.New("overlay not found")
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:      "unauthorized (different user)",
			overlayID: overlayID,
			userID:    uuid.New().String(), // Different user
			mockRepo: &mockOverlayRepository{
				getByIDAndUserIDFunc: func(ctx context.Context, id, uid string) (*models.Overlay, error) {
					return nil, errors.New("unauthorized")
				},
			},
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewOverlayHandler(tt.mockRepo)

			router.DELETE("/overlays/:id", func(c *gin.Context) {
				if tt.userID != "" {
					c.Set("user_id", tt.userID)
				}
				handler.HandleDeleteOverlay(c)
			})

			req := httptest.NewRequest(http.MethodDelete, "/overlays/"+tt.overlayID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)
		})
	}
}

func TestOverlayHandler_RegisterRoutes(t *testing.T) {
	router := setupTestRouter()
	handler := NewOverlayHandler(&mockOverlayRepository{})

	handler.RegisterRoutes(router)

	// Test that routes are registered
	routes := router.Routes()
	assert.NotEmpty(t, routes)

	// Check for expected routes
	expectedRoutes := map[string]string{
		"POST /overlays":      "CreateOverlay",
		"GET /overlays":       "ListOverlays",
		"GET /overlays/:id":   "GetOverlay",
		"PUT /overlays/:id":   "UpdateOverlay",
		"DELETE /overlays/:id": "DeleteOverlay",
	}

	for route := range expectedRoutes {
		found := false
		for _, r := range routes {
			if r.Method+" "+r.Path == route {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s should be registered", route)
	}
}
