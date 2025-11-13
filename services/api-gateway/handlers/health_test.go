package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_CheckHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		serviceResponses map[string]httpResponse
		expectedStatus   int
		checkResponse    func(*testing.T, *models.HealthResponse)
	}{
		{
			name: "all services healthy",
			serviceResponses: map[string]httpResponse{
				"auth-service":    {statusCode: 200, delay: 5 * time.Millisecond},
				"overlay-manager": {statusCode: 200, delay: 8 * time.Millisecond},
				"emote-service":   {statusCode: 200, delay: 3 * time.Millisecond},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.HealthResponse) {
				assert.Equal(t, "healthy", resp.Status)
				assert.Len(t, resp.Services, 3)
				assert.Equal(t, "up", resp.Services["auth-service"].Status)
				assert.Equal(t, "up", resp.Services["overlay-manager"].Status)
				assert.Equal(t, "up", resp.Services["emote-service"].Status)
				assert.True(t, resp.Services["auth-service"].LatencyMs >= 5)
			},
		},
		{
			name: "one service down - degraded",
			serviceResponses: map[string]httpResponse{
				"auth-service":    {statusCode: 200, delay: 5 * time.Millisecond},
				"overlay-manager": {statusCode: 500, delay: 8 * time.Millisecond},
				"emote-service":   {statusCode: 200, delay: 3 * time.Millisecond},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.HealthResponse) {
				assert.Equal(t, "degraded", resp.Status)
				assert.Len(t, resp.Services, 3)
				assert.Equal(t, "up", resp.Services["auth-service"].Status)
				assert.Equal(t, "down", resp.Services["overlay-manager"].Status)
				assert.Equal(t, "up", resp.Services["emote-service"].Status)
			},
		},
		{
			name: "all services down - unhealthy",
			serviceResponses: map[string]httpResponse{
				"auth-service":    {statusCode: 500, delay: 0},
				"overlay-manager": {statusCode: 500, delay: 0},
				"emote-service":   {statusCode: 500, delay: 0},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *models.HealthResponse) {
				assert.Equal(t, "unhealthy", resp.Status)
				assert.Len(t, resp.Services, 3)
				assert.Equal(t, "down", resp.Services["auth-service"].Status)
				assert.Equal(t, "down", resp.Services["overlay-manager"].Status)
				assert.Equal(t, "down", resp.Services["emote-service"].Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock backend servers
			servers := make(map[string]*httptest.Server)
			for serviceName, response := range tt.serviceResponses {
				resp := response // Capture loop variable
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(resp.delay)
					w.WriteHeader(resp.statusCode)
				}))
				defer server.Close()
				servers[serviceName] = server
			}

			// Create service registry with mock server URLs
			registry := &models.ServiceRegistry{
				Services: make(map[string]*models.ServiceConfig),
			}
			for serviceName, server := range servers {
				registry.Services[serviceName] = &models.ServiceConfig{
					Name:       serviceName,
					BaseURL:    server.URL,
					HealthPath: "/health",
				}
			}

			// Create health handler
			handler := NewHealthHandler(registry)

			// Create test router
			router := gin.New()
			router.GET("/health", handler.CheckHealth)

			// Make request
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)

			var healthResp models.HealthResponse
			err := json.Unmarshal(w.Body.Bytes(), &healthResp)
			require.NoError(t, err)

			tt.checkResponse(t, &healthResp)
			assert.False(t, healthResp.Timestamp.IsZero())
		})
	}
}

// httpResponse represents a mock HTTP response for testing
type httpResponse struct {
	statusCode int
	delay      time.Duration
}
