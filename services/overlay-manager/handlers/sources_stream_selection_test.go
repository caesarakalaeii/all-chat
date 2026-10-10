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

// fakeStreamSelectionGate answers for the stream_selection gate and the caller's premium
// status together, and counts calls so a test can prove a free save checks nothing.
type fakeStreamSelectionGate struct {
	allowed bool
	err     error
	calls   int
}

func (f *fakeStreamSelectionGate) Allowed(context.Context, string) (bool, error) {
	f.calls++
	return f.allowed, f.err
}

type streamSelectionResult struct {
	code    int
	body    map[string]interface{}
	updated bool
	saved   map[string]interface{}
}

func patchStreamSelection(t *testing.T, gate *fakeStreamSelectionGate, stored, config map[string]interface{}) streamSelectionResult {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var res streamSelectionResult
	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{
			getByIDFunc: func(_ context.Context, id string) (*models.ChatSource, error) {
				return &models.ChatSource{ID: id, OverlayID: "overlay-id", Platform: "youtube", ChannelID: "UCabcdefghijklmnopqrstuv", Config: stored}, nil
			},
			updateConfigFunc: func(_ context.Context, _ string, cfg map[string]interface{}) error {
				res.updated = true
				res.saved = cfg
				return nil
			},
		},
		&mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test"}, nil
			},
		},
	)
	h.SetStreamSelectionGate(gate)

	bodyBytes, err := json.Marshal(map[string]interface{}{"config": config})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	setupPatchRouter(h).ServeHTTP(w, req)

	res.code = w.Code
	_ = json.Unmarshal(w.Body.Bytes(), &res.body)
	return res
}

func TestStreamSelectionGate_NonPremiumStrategyRefused(t *testing.T) {
	gate := &fakeStreamSelectionGate{allowed: false}
	res := patchStreamSelection(t, gate, map[string]interface{}{}, map[string]interface{}{"stream_select": "most_viewers"})

	assert.Equal(t, http.StatusForbidden, res.code)
	assert.False(t, res.updated)
	assert.Equal(t, "Premium feature required", res.body["error"])
	assert.Equal(t, "/upgrade", res.body["upgrade_url"])
	assert.NotEmpty(t, res.body["message"])
}

func TestStreamSelectionGate_PremiumStrategyAllowed(t *testing.T) {
	gate := &fakeStreamSelectionGate{allowed: true}
	res := patchStreamSelection(t, gate, map[string]interface{}{},
		map[string]interface{}{"stream_select": "title_match", "stream_match": "main"})

	assert.Equal(t, http.StatusOK, res.code)
	assert.Equal(t, "title_match", res.saved["stream_select"])
	assert.Equal(t, 1, gate.calls)
}

func TestStreamSelectionGate_FreeStrategyNeedsNoPremium(t *testing.T) {
	for name, config := range map[string]map[string]interface{}{
		"first_found":       {"stream_select": "first_found"},
		"strategy key gone": {"other": true},
	} {
		t.Run(name, func(t *testing.T) {
			gate := &fakeStreamSelectionGate{allowed: false}
			res := patchStreamSelection(t, gate, map[string]interface{}{"stream_select": "most_viewers"}, config)

			assert.Equal(t, http.StatusOK, res.code, "going back to the free default must work")
			assert.Equal(t, 0, gate.calls)
		})
	}
}

func TestStreamSelectionGate_LapsedUserKeepsStoredStrategy(t *testing.T) {
	// The frontend PATCHes the whole config, so an untouched premium strategy rides along on
	// every other save; a lapsed user must still be able to save the rest.
	t.Run("unchanged title match", func(t *testing.T) {
		gate := &fakeStreamSelectionGate{allowed: false}
		stored := map[string]interface{}{"stream_select": "title_match", "stream_match": "main"}
		res := patchStreamSelection(t, gate, stored,
			map[string]interface{}{"stream_select": "title_match", "stream_match": "main", "other": true})

		assert.Equal(t, http.StatusOK, res.code)
		assert.Equal(t, "title_match", res.saved["stream_select"])
		assert.Equal(t, 0, gate.calls)
	})
	t.Run("match term the strategy ignores", func(t *testing.T) {
		gate := &fakeStreamSelectionGate{allowed: false}
		stored := map[string]interface{}{"stream_select": "most_viewers", "stream_match": "old"}
		res := patchStreamSelection(t, gate, stored,
			map[string]interface{}{"stream_select": "most_viewers", "stream_match": "new"})

		assert.Equal(t, http.StatusOK, res.code, "most_viewers never reads stream_match")
		assert.Equal(t, 0, gate.calls)
	})
}

func TestStreamSelectionGate_LapsedUserCannotChangeMatchTerm(t *testing.T) {
	gate := &fakeStreamSelectionGate{allowed: false}
	res := patchStreamSelection(t, gate, map[string]interface{}{"stream_select": "title_match", "stream_match": "main"},
		map[string]interface{}{"stream_select": "title_match", "stream_match": "vertical"})

	assert.Equal(t, http.StatusForbidden, res.code)
	assert.False(t, res.updated)
}

func TestStreamSelectionGate_LookupErrorFailsClosed(t *testing.T) {
	gate := &fakeStreamSelectionGate{err: errors.New("db down")}
	res := patchStreamSelection(t, gate, map[string]interface{}{}, map[string]interface{}{"stream_select": "all"})

	assert.Equal(t, http.StatusServiceUnavailable, res.code)
	assert.False(t, res.updated)
}

func TestStreamSelectionGate_AddSourceNonPremiumRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	created := false
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(context.Context, *models.ChatSource) error {
				created = true
				return nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		logger: zap.NewNop(),
	}
	h.SetStreamSelectionGate(&fakeStreamSelectionGate{allowed: false})

	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.HandleAddSource(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlays/ov-1/sources", bytes.NewBufferString(
		`{"platform":"youtube","channel_id":"UCabcdefghijklmnopqrstuv","config":{"stream_select":"fewest_viewers"}}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.False(t, created)
}
