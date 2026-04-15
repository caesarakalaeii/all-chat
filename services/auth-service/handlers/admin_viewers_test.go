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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/repository"
)

// mockViewerRepo implements the subset of ViewerRepository used by AdminViewerHandler.
type mockViewerRepo struct {
	repository.ViewerRepository
	listAllFn         func(limit, offset int) ([]models.ViewerSession, error)
	setViewerPremiumFn func(sessionID uuid.UUID, isPremium bool) error
}

func setupAdminViewerRouter(t *testing.T, mock *mockViewerRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	logger := zap.NewNop()
	handler := &AdminViewerHandler{log: logger, viewerRepo: &mock.ViewerRepository}
	// We can't easily mock the repo through the real handler since it takes a concrete type.
	// Instead, test via HTTP against the real handler with a test DB.
	// For now, test request validation.
	r.POST("/admin/viewers/:session_id/premium", handler.HandleSetViewerPremium)
	return r
}

func TestHandleSetViewerPremium_InvalidSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	logger := zap.NewNop()
	// Create handler with nil repo — we expect validation to fail before DB access
	handler := &AdminViewerHandler{log: logger, viewerRepo: nil}
	r.POST("/admin/viewers/:session_id/premium", handler.HandleSetViewerPremium)

	body := `{"is_premium": true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/viewers/not-a-uuid/premium", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid session ID" {
		t.Errorf("unexpected error: %s", resp["error"])
	}
}

func TestHandleSetViewerPremium_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	logger := zap.NewNop()
	handler := &AdminViewerHandler{log: logger, viewerRepo: nil}
	r.POST("/admin/viewers/:session_id/premium", handler.HandleSetViewerPremium)

	sessionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/admin/viewers/"+sessionID+"/premium", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
