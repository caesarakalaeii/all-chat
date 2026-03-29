package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// stubWSConn implements wsConnectionHealth for testing.
type stubWSConn struct {
	connected      bool
	lastActivityAt time.Time
	stale          bool
}

func (s *stubWSConn) IsConnected() bool        { return s.connected }
func (s *stubWSConn) LastActivityAt() time.Time { return s.lastActivityAt }
func (s *stubWSConn) IsStale() bool             { return s.stale }

// newTestHealthHandler builds a HealthHandler with a stub WS conn and nil publisher/channelMgr.
// Only the liveness probe is exercised in these tests so publisher/channelMgr are not needed.
func newTestHealthHandler(ws wsConnectionHealth) *HealthHandler {
	return &HealthHandler{wsConn: ws}
}

func newGinRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health/live", h.LivenessProbe)
	return r
}

func TestLivenessProbe_Healthy_Returns200(t *testing.T) {
	h := newTestHealthHandler(&stubWSConn{
		connected:      true,
		lastActivityAt: time.Now().Add(-1 * time.Minute),
		stale:          false,
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for healthy connection, got %d", w.Code)
	}
}

func TestLivenessProbe_ZombieConnection_Returns503(t *testing.T) {
	// Simulate a zombie Pusher WebSocket: connection object exists but no messages
	// have been received for well over the staleLivenessThreshold.
	h := newTestHealthHandler(&stubWSConn{
		connected:      true,
		lastActivityAt: time.Now().Add(-10 * time.Minute),
		stale:          true,
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for zombie connection (triggers Kubernetes pod restart), got %d", w.Code)
	}
}

func TestLivenessProbe_NeverConnected_Returns200(t *testing.T) {
	// Startup: lastActivityAt is zero, not stale — pod should not be restarted.
	h := newTestHealthHandler(&stubWSConn{
		connected:      false,
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

func TestLivenessProbe_StaleButNeverConnected_Returns200(t *testing.T) {
	// IsStale() returns false when lastActivityAt is zero, so liveness should pass.
	// This test verifies the handler honours IsStale() directly.
	h := newTestHealthHandler(&stubWSConn{
		connected:      false,
		lastActivityAt: time.Time{},
		stale:          false, // IsStale() is false because zero lastActivityAt
	})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	newGinRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
