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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockSourceRepositoryWithConfig extends the existing mock to include UpdateConfig.
// We use a separate struct here to avoid conflicts with mockSourceRepository in
// sources_shared_overlay_test.go (which does NOT implement UpdateConfig).
type mockSourceRepositoryWithConfig struct {
	createFunc           func(context.Context, *models.ChatSource) error
	listByOverlayFunc    func(context.Context, string) ([]*models.ChatSource, error)
	getByIDFunc          func(context.Context, string) (*models.ChatSource, error)
	deleteFunc           func(context.Context, string) error
	updateConfigFunc     func(context.Context, string, map[string]interface{}) error
}

func (m *mockSourceRepositoryWithConfig) Create(ctx context.Context, source *models.ChatSource) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, source)
	}
	return nil
}

func (m *mockSourceRepositoryWithConfig) ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error) {
	if m.listByOverlayFunc != nil {
		return m.listByOverlayFunc(ctx, overlayID)
	}
	return nil, nil
}

func (m *mockSourceRepositoryWithConfig) GetByID(ctx context.Context, id string) (*models.ChatSource, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockSourceRepositoryWithConfig) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockSourceRepositoryWithConfig) UpdateConfig(ctx context.Context, id string, config map[string]interface{}) error {
	if m.updateConfigFunc != nil {
		return m.updateConfigFunc(ctx, id, config)
	}
	return nil
}

// buildPatchHandler builds a SourcesHandler with the given mock repositories.
func buildPatchHandler(
	srcRepo SourceRepository,
	overlayRepo OverlayRepository,
) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo:  srcRepo,
		overlayRepo: overlayRepo,
		logger:      zap.NewNop(),
	}
}

// setupPatchRouter returns a Gin router wired to HandleUpdateSourceConfig.
func setupPatchRouter(h *SourcesHandler) *gin.Engine {
	router := gin.New()
	router.PATCH("/overlays/:id/sources/:source_id", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		h.HandleUpdateSourceConfig(c)
	})
	return router
}

// TestHandleUpdateSourceConfig_Success verifies that a valid PATCH request with
// ownership and config returns 200 with "config updated" message.
func TestHandleUpdateSourceConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedID string
	var capturedConfig map[string]interface{}

	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{
			updateConfigFunc: func(_ context.Context, id string, cfg map[string]interface{}) error {
				capturedID = id
				capturedConfig = cfg
				return nil
			},
		},
		&mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test"}, nil
			},
		},
	)

	router := setupPatchRouter(h)

	body := map[string]interface{}{
		"config": map[string]interface{}{
			"guild_id":          "123456789",
			"inbound_channel_id": "987654321",
			"relay_enabled":     true,
			"relay_channel_id":  nil,
		},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "config updated", resp["message"])

	assert.Equal(t, "source-id", capturedID)
	assert.Equal(t, "123456789", capturedConfig["guild_id"])
}

// TestHandleUpdateSourceConfig_NonOwner verifies that a PATCH request where the
// overlay does not belong to the user returns 403.
func TestHandleUpdateSourceConfig_NonOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{},
		&mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return nil, errors.New("not found")
			},
		},
	)

	router := setupPatchRouter(h)

	body := map[string]interface{}{
		"config": map[string]interface{}{"guild_id": "123"},
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/other-overlay/sources/source-id", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestHandleUpdateSourceConfig_MissingConfig verifies that a PATCH request with
// no config field returns 400.
func TestHandleUpdateSourceConfig_MissingConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{},
		&mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test"}, nil
			},
		},
	)

	router := setupPatchRouter(h)

	// Body has no "config" key
	body := map[string]interface{}{
		"other_field": "value",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
