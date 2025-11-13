package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Ensure pgxpool is available for type assertions
var _ *pgxpool.Pool

// Mock database pool
type mockDBPool struct {
	pingFunc func(context.Context) error
}

func (m *mockDBPool) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

func (m *mockDBPool) Close() {}

// Mock Redis client
type mockRedisClient struct {
	pingFunc func(context.Context) *redis.StatusCmd
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
	router := setupTestRouter()
	handler := NewHealthHandler(nil, nil)

	router.GET("/health/live", handler.HandleLiveness)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "alive")
}

func TestHealthHandler_HandleReadiness(t *testing.T) {
	tests := []struct {
		name           string
		mockDB         func(context.Context) error
		mockRedis      func(context.Context) *redis.StatusCmd
		wantStatusCode int
		wantBody       string
	}{
		{
			name: "all healthy",
			mockDB: func(ctx context.Context) error {
				return nil
			},
			mockRedis: func(ctx context.Context) *redis.StatusCmd {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetVal("PONG")
				return cmd
			},
			wantStatusCode: http.StatusOK,
			wantBody:       "ready",
		},
		{
			name: "database unhealthy",
			mockDB: func(ctx context.Context) error {
				return errors.New("connection refused")
			},
			mockRedis: func(ctx context.Context) *redis.StatusCmd {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetVal("PONG")
				return cmd
			},
			wantStatusCode: http.StatusServiceUnavailable,
			wantBody:       "database unreachable",
		},
		{
			name: "redis unhealthy",
			mockDB: func(ctx context.Context) error {
				return nil
			},
			mockRedis: func(ctx context.Context) *redis.StatusCmd {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetErr(errors.New("connection refused"))
				return cmd
			},
			wantStatusCode: http.StatusServiceUnavailable,
			wantBody:       "redis unreachable",
		},
		{
			name: "both unhealthy",
			mockDB: func(ctx context.Context) error {
				return errors.New("db error")
			},
			mockRedis: func(ctx context.Context) *redis.StatusCmd {
				cmd := redis.NewStatusCmd(ctx)
				cmd.SetErr(errors.New("redis error"))
				return cmd
			},
			wantStatusCode: http.StatusServiceUnavailable,
			wantBody:       "database unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()

			db := &mockDBPool{pingFunc: tt.mockDB}
			redisClient := &mockRedisClient{pingFunc: tt.mockRedis}

			handler := NewHealthHandler(db, redisClient)

			router.GET("/health/ready", handler.HandleReadiness)

			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBody)
		})
	}
}

func TestHealthHandler_RegisterRoutes(t *testing.T) {
	router := setupTestRouter()
	handler := NewHealthHandler(nil, nil)

	handler.RegisterRoutes(router)

	routes := router.Routes()
	assert.NotEmpty(t, routes)

	// Check health routes are registered
	expectedRoutes := []string{
		"GET /health/live",
		"GET /health/ready",
	}

	for _, expected := range expectedRoutes {
		found := false
		for _, r := range routes {
			if r.Method+" "+r.Path == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Route %s should be registered", expected)
	}
}
