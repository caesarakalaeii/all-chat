package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// stubIRCConn implements ircConnectionHealth for testing.
type stubIRCConn struct {
	connected      bool
	lastActivityAt time.Time
	stale          bool
}

func (s *stubIRCConn) IsConnected() bool        { return s.connected }
func (s *stubIRCConn) LastActivityAt() time.Time { return s.lastActivityAt }
func (s *stubIRCConn) IsStale() bool             { return s.stale }

// newTestHealthHandler builds a HealthHandler with a stub IRC conn and nil publisher/chanMgr.
// Only the liveness probe is exercised in these tests so publisher/chanMgr are not needed.
func newTestHealthHandler(irc ircConnectionHealth) *HealthHandler {
	return &HealthHandler{ircConn: irc}
}

func newGinRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health/live", h.LivenessProbe)
	return r
}

func TestLivenessProbe_Healthy_Returns200(t *testing.T) {
	h := newTestHealthHandler(&stubIRCConn{
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
	// Simulate the vr86t pod scenario: IRC joined ironmouse but connection has been
	// zombie for 85+ minutes and watchdog Disconnect() calls don't unblock Connect().
	h := newTestHealthHandler(&stubIRCConn{
		connected:      true,
		lastActivityAt: time.Now().Add(-85 * time.Minute),
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
	h := newTestHealthHandler(&stubIRCConn{
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
	h := newTestHealthHandler(&stubIRCConn{
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
