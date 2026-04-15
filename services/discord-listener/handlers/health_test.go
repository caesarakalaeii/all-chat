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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// stubGatewayConn implements gatewayConnectionHealth for testing.
type stubGatewayConn struct {
	lastActivityAt time.Time
	stale          bool
}

func (s *stubGatewayConn) LastActivityAt() time.Time { return s.lastActivityAt }
func (s *stubGatewayConn) IsStale() bool             { return s.stale }

func newTestHealthHandler(gw gatewayConnectionHealth) *HealthHandler {
	return &HealthHandler{gatewayConn: gw}
}

func newGinRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health/live", h.CheckLive)
	r.GET("/health/ready", h.CheckReady)
	return r
}

func TestCheckLive_Healthy_Returns200(t *testing.T) {
	h := newTestHealthHandler(&stubGatewayConn{
		lastActivityAt: time.Now().Add(-1 * time.Minute),
		stale:          false,
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for healthy gateway, got %d", w.Code)
	}
}

func TestCheckLive_ZombieConnection_Returns503(t *testing.T) {
	// Simulate a zombie connection: last heartbeat ACK was >3 minutes ago.
	h := newTestHealthHandler(&stubGatewayConn{
		lastActivityAt: time.Now().Add(-10 * time.Minute),
		stale:          true,
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for zombie gateway connection (triggers pod restart), got %d", w.Code)
	}
}

func TestCheckLive_NeverConnected_Returns200(t *testing.T) {
	// Startup: lastActivityAt is zero, not stale — pod should not be restarted.
	h := newTestHealthHandler(&stubGatewayConn{
		lastActivityAt: time.Time{},
		stale:          false,
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 during startup (never connected), got %d", w.Code)
	}
}

func TestCheckLive_NilGatewayConn_Returns200(t *testing.T) {
	// When gatewayConn is nil (e.g. test env without a gateway client),
	// the liveness probe must still return 200 — not panic.
	h := &HealthHandler{gatewayConn: nil}
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when gatewayConn is nil, got %d", w.Code)
	}
}

func TestCheckReady_NilRedis_Returns503(t *testing.T) {
	// When redis is nil the ready check must not panic and should return 503.
	h := &HealthHandler{redis: nil}
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when redis is nil, got %d", w.Code)
	}
}
