package handlers

// Wave 0: tests fail RED until plan 02 implements the exchange handlers.
// The stub handlers return 501 Not Implemented; the tests assert 400 Bad Request,
// so every test below will FAIL until real validation logic is added in plan 02.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestViewerAuthHandler creates a minimal ViewerAuthHandler for testing
// (all providers and repos are nil; only used to reach the stub exchange handlers).
func newTestViewerAuthHandler() *ViewerAuthHandler {
	return &ViewerAuthHandler{}
}

func TestHandleTwitchExchange_MissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := newTestViewerAuthHandler()
	router.POST("/viewer/twitch/exchange", h.HandleTwitchExchange)

	req := httptest.NewRequest(http.MethodPost, "/viewer/twitch/exchange", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// RED: stub returns 501; plan 02 must make this return 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d", w.Code)
	}
}

func TestHandleYouTubeExchange_MissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := newTestViewerAuthHandler()
	router.POST("/viewer/youtube/exchange", h.HandleYouTubeExchange)

	req := httptest.NewRequest(http.MethodPost, "/viewer/youtube/exchange", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// RED: stub returns 501; plan 02 must make this return 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d", w.Code)
	}
}

func TestHandleKickExchange_MissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := newTestViewerAuthHandler()
	router.POST("/viewer/kick/exchange", h.HandleKickExchange)

	req := httptest.NewRequest(http.MethodPost, "/viewer/kick/exchange", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// RED: stub returns 501; plan 02 must make this return 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d", w.Code)
	}
}
