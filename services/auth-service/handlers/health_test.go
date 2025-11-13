package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Mock database pool
type mockPgxPool struct {
	pingFunc func(ctx context.Context) error
}

func (m *mockPgxPool) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

// Mock Redis client
type mockRedisClient struct {
	pingFunc func(ctx context.Context) *redis.StatusCmd
}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")
	return cmd
}

func TestHealthHandler_HandleLiveness(t *testing.T) {
	tests := []struct {
		name           string
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "liveness always returns OK",
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Body.String() != `{"status":"ok"}` {
					t.Errorf("HandleLiveness() body = %v, want {\"status\":\"ok\"}", w.Body.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewHealthHandler(nil, nil) // Liveness doesn't need dependencies

			router.GET("/health/live", handler.HandleLiveness)

			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleLiveness() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestHealthHandler_HandleReadiness(t *testing.T) {
	tests := []struct {
		name           string
		mockDB         *mockPgxPool
		mockRedis      *mockRedisClient
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "all services healthy",
			mockDB: &mockPgxPool{
				pingFunc: func(ctx context.Context) error {
					return nil
				},
			},
			mockRedis: &mockRedisClient{
				pingFunc: func(ctx context.Context) *redis.StatusCmd {
					cmd := redis.NewStatusCmd(ctx)
					cmd.SetVal("PONG")
					return cmd
				},
			},
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["status"] != "ok" || response["database"] != "ok" || response["redis"] != "ok" {
					t.Errorf("HandleReadiness() unexpected values: %v", response)
				}
			},
		},
		{
			name: "database unhealthy",
			mockDB: &mockPgxPool{
				pingFunc: func(ctx context.Context) error {
					return errors.New("connection failed")
				},
			},
			mockRedis: &mockRedisClient{
				pingFunc: func(ctx context.Context) *redis.StatusCmd {
					cmd := redis.NewStatusCmd(ctx)
					cmd.SetVal("PONG")
					return cmd
				},
			},
			wantStatusCode: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["status"] != "unhealthy" || response["database"] != "error" || response["redis"] != "ok" {
					t.Errorf("HandleReadiness() unexpected values: %v", response)
				}
			},
		},
		{
			name: "redis unhealthy",
			mockDB: &mockPgxPool{
				pingFunc: func(ctx context.Context) error {
					return nil
				},
			},
			mockRedis: &mockRedisClient{
				pingFunc: func(ctx context.Context) *redis.StatusCmd {
					cmd := redis.NewStatusCmd(ctx)
					cmd.SetErr(errors.New("redis connection failed"))
					return cmd
				},
			},
			wantStatusCode: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["status"] != "unhealthy" || response["database"] != "ok" || response["redis"] != "error" {
					t.Errorf("HandleReadiness() unexpected values: %v", response)
				}
			},
		},
		{
			name: "all services unhealthy",
			mockDB: &mockPgxPool{
				pingFunc: func(ctx context.Context) error {
					return errors.New("database error")
				},
			},
			mockRedis: &mockRedisClient{
				pingFunc: func(ctx context.Context) *redis.StatusCmd {
					cmd := redis.NewStatusCmd(ctx)
					cmd.SetErr(errors.New("redis error"))
					return cmd
				},
			},
			wantStatusCode: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["status"] != "unhealthy" || response["database"] != "error" || response["redis"] != "error" {
					t.Errorf("HandleReadiness() unexpected values: %v", response)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()

			// We'll need to adapt the handler to accept our mock interfaces
			// For now, this test structure shows what we're testing
			// The actual implementation will need interface adaptation
			handler := NewHealthHandler(
				&struct{ *mockPgxPool }{tt.mockDB},
				&struct{ *mockRedisClient }{tt.mockRedis},
			)

			router.GET("/health/ready", handler.HandleReadiness)

			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleReadiness() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestHealthHandler_RegisterRoutes(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		wantFound bool
	}{
		{
			name:      "liveness route registered",
			endpoint:  "/health/live",
			wantFound: true,
		},
		{
			name:      "readiness route registered",
			endpoint:  "/health/ready",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			handler := NewHealthHandler(nil, nil)

			healthGroup := router.Group("/health")
			handler.RegisterRoutes(healthGroup)

			routes := router.Routes()
			found := false
			for _, route := range routes {
				if route.Path == tt.endpoint {
					found = true
					break
				}
			}

			if found != tt.wantFound {
				t.Errorf("RegisterRoutes() route %s found = %v, want %v", tt.endpoint, found, tt.wantFound)
			}
		})
	}
}
