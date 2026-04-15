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

// Tests for POST exchange handlers (Task 1, plan 28-02).
// The exchange handlers return 400 for missing code/state (binding validation),
// and 401 for invalid/expired state (Redis miss).

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"
)

// newTestViewerAuthHandler creates a minimal ViewerAuthHandler for unit testing.
// Only fields needed for the specific test cases are populated.
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d body=%s", w.Code, w.Body.String())
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d body=%s", w.Code, w.Body.String())
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing code, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGenerateViewerJWT_HasViewerID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	h := &ViewerAuthHandler{
		jwtSecret: "test-secret-32-bytes-long-for-jwt",
		jwtExpiry: 24 * time.Hour,
		logger:    logger,
	}

	viewerID := uuid.New()
	session := &models.ViewerSession{
		ID:             uuid.New(),
		Platform:       "twitch",
		PlatformUserID: "12345",
		Username:       "testuser",
		DisplayName:    "Test User",
	}

	tokenStr, err := h.generateViewerJWT(session, viewerID, nil)
	if err != nil {
		t.Fatalf("generateViewerJWT returned error: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("generateViewerJWT returned empty token")
	}

	// JWT is base64url(header).base64url(payload).signature
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// Pad base64url if needed
	payload := parts[1]
	for len(payload)%4 != 0 {
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("failed to decode JWT payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		t.Fatalf("failed to unmarshal JWT payload: %v", err)
	}

	viewerIDClaim, ok := claims["viewer_id"]
	if !ok {
		t.Fatal("JWT payload missing viewer_id claim")
	}
	if viewerIDClaim != viewerID.String() {
		t.Errorf("viewer_id claim = %v, want %v", viewerIDClaim, viewerID.String())
	}
}
