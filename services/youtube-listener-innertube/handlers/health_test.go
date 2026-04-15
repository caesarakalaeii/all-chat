// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// mockRedisHealthChecker mocks the Redis health checker
type mockRedisHealthChecker struct {
	shouldFail bool
}

func (m *mockRedisHealthChecker) Ping(ctx context.Context) error {
	if m.shouldFail {
		return errors.New("redis connection failed")
	}
	return nil
}

// mockInnerTubeClientChecker mocks the InnerTube client checker
type mockInnerTubeClientChecker struct {
	initialized bool
}

func (m *mockInnerTubeClientChecker) IsInitialized() bool {
	return m.initialized
}

// TestLivenessProbe_AlwaysReturns200 verifies liveness probe always returns 200 OK
func TestLivenessProbe_AlwaysReturns200(t *testing.T) {
	logger := zap.NewNop()

	// Create handler with failing dependencies
	handler := NewHealthHandler(
		&mockRedisHealthChecker{shouldFail: true},
		&mockInnerTubeClientChecker{initialized: false},
		logger,
	)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health/live", handler.LivenessProbe)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"status":"alive"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

// TestReadinessProbe_Returns503_WhenRedisUnavailable verifies readiness returns 503 when Redis fails
func TestReadinessProbe_Returns503_WhenRedisUnavailable(t *testing.T) {
	logger := zap.NewNop()

	handler := NewHealthHandler(
		&mockRedisHealthChecker{shouldFail: true},
		&mockInnerTubeClientChecker{initialized: true},
		logger,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health/ready", handler.ReadinessProbe)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

// TestReadinessProbe_Returns503_WhenInnerTubeNotInitialized verifies readiness returns 503 when client not ready
func TestReadinessProbe_Returns503_WhenInnerTubeNotInitialized(t *testing.T) {
	logger := zap.NewNop()

	handler := NewHealthHandler(
		&mockRedisHealthChecker{shouldFail: false},
		&mockInnerTubeClientChecker{initialized: false},
		logger,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health/ready", handler.ReadinessProbe)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

// TestReadinessProbe_Returns200_WhenAllChecksPass verifies readiness returns 200 when all checks pass
func TestReadinessProbe_Returns200_WhenAllChecksPass(t *testing.T) {
	logger := zap.NewNop()

	handler := NewHealthHandler(
		&mockRedisHealthChecker{shouldFail: false},
		&mockInnerTubeClientChecker{initialized: true},
		logger,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health/ready", handler.ReadinessProbe)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"status":"ready"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

// TestReadinessProbe_NilInnerTubeChecker verifies readiness handles nil InnerTube checker
func TestReadinessProbe_NilInnerTubeChecker(t *testing.T) {
	logger := zap.NewNop()

	handler := NewHealthHandler(
		&mockRedisHealthChecker{shouldFail: false},
		nil, // No InnerTube checker
		logger,
	)

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health/ready", handler.ReadinessProbe)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should still return 200 if Redis is healthy
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
