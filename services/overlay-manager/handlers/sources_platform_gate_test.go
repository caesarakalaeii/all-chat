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
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// The ADR-0008 platform rollout gate: adding a source on an expansion platform
// (owncast, goodgame, picarto, facebook, instagram) is refused while the gate is
// closed for the caller, allowed once it graduates, and fails closed on a
// lookup error. A nil gate (direct struct construction) must read as open.

type stubPlatformGate struct {
	allowed map[string]bool
	err     error
}

func (g stubPlatformGate) PlatformSourceAllowed(_ context.Context, _, platform string) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.allowed[platform], nil
}

func gateTestHandler(gate PlatformSourceGate) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(_ context.Context, _ *models.ChatSource) error { return nil },
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		logger:       zap.NewNop(),
		platformGate: gate,
	}
}

func postGoodGameSource(h *SourcesHandler) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.HandleAddSource(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlays/ov-1/sources",
		strings.NewReader(`{"platform":"goodgame","channel_id":"Miker"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestAddSource_PlatformGateClosedRefuses(t *testing.T) {
	h := gateTestHandler(stubPlatformGate{allowed: map[string]bool{}})
	w := postGoodGameSource(h)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "not available for your account")
}

func TestAddSource_PlatformGateOpenAllows(t *testing.T) {
	h := gateTestHandler(stubPlatformGate{allowed: map[string]bool{"goodgame": true}})
	w := postGoodGameSource(h)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAddSource_PlatformGateErrorFailsClosed(t *testing.T) {
	h := gateTestHandler(stubPlatformGate{err: errors.New("db down")})
	w := postGoodGameSource(h)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddSource_NilGateReadsAsOpen(t *testing.T) {
	// Direct construction (as every other test in this package does) leaves the
	// gate nil; a nil gate must read as open, not panic.
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(_ context.Context, _ *models.ChatSource) error { return nil },
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		logger: zap.NewNop(),
	}
	w := postGoodGameSource(h)
	assert.Equal(t, http.StatusCreated, w.Code)
}

// Altstream (ADR-0082) rides the same rollout gate. Its channel is the
// Altstream username, which Altstream treats case-insensitively, so the source
// stores it trimmed and lowercased; anything outside [A-Za-z0-9_-]{1,64} is
// refused before it reaches the listener's URLs.
func postAltstreamSource(t *testing.T, gate PlatformSourceGate, channelID string) (*httptest.ResponseRecorder, *models.ChatSource) {
	t.Helper()
	var created *models.ChatSource
	h := gateTestHandler(gate)
	h.sourceRepo = &mockSourceRepository{
		createFunc: func(_ context.Context, s *models.ChatSource) error { created = s; return nil },
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.HandleAddSource(c)
	})
	body, err := json.Marshal(map[string]string{"platform": "altstream", "channel_id": channelID})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlays/ov-1/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w, created
}

func TestAddSource_Altstream(t *testing.T) {
	open := stubPlatformGate{allowed: map[string]bool{"altstream": true}}

	w, created := postAltstreamSource(t, stubPlatformGate{allowed: map[string]bool{}}, "copygirl")
	assert.Equal(t, http.StatusForbidden, w.Code, "platform_altstream closed for the caller")
	assert.Nil(t, created)

	w, created = postAltstreamSource(t, open, " Utsuho____Reiuji ")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, created)
	assert.Equal(t, "altstream", created.Platform)
	assert.Equal(t, "utsuho____reiuji", created.ChannelID)
	assert.False(t, created.AuthRequired, "Altstream's chat API needs no OAuth")

	for _, ok := range []string{"kanaruhoshino16", "a", "copy-girl", strings.Repeat("a", 64)} {
		w, _ = postAltstreamSource(t, open, ok)
		assert.Equal(t, http.StatusCreated, w.Code, "%q must be accepted", ok)
	}
	for _, bad := range []string{"copy girl", "copygirl/../api", "copygirl?x=1", "kanä", "@copygirl", strings.Repeat("a", 65)} {
		w, created = postAltstreamSource(t, open, bad)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%q must be refused", bad)
		assert.Nil(t, created, "%q must not be stored", bad)
	}
}
