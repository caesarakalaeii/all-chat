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
	"github.com/stretchr/testify/require"
)

// capturingConfigRepo records what Update was asked to persist, which is the only
// way to tell "the gated keys were carried over" from "they were accepted".
type capturingConfigRepo struct {
	cfg   *models.OverlayConfig
	saved *models.OverlayConfig
}

func (r *capturingConfigRepo) GetByOverlayID(context.Context, string) (*models.OverlayConfig, error) {
	if r.cfg == nil {
		return nil, errors.New("not found")
	}
	return r.cfg, nil
}

func (r *capturingConfigRepo) Update(_ context.Context, cfg *models.OverlayConfig) error {
	r.saved = cfg
	return nil
}

type stubBubbleGate struct {
	enabled bool
	err     error
}

func (g stubBubbleGate) BubbleColorsEnabled(context.Context, string) (bool, error) {
	return g.enabled, g.err
}

func bubbleRouter(repo OverlayConfigRepository, gate BubbleColorsGate) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewConfigHandler(repo, &stubOverlayRepo{owned: true}, &stubSourceRepo{}, nil, gate)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
	r.GET("/:id/config", h.HandleGetConfig)
	r.PUT("/:id/config", h.HandleUpdateConfig)
	return r
}

func storedConfig(visual map[string]any) *models.OverlayConfig {
	return &models.OverlayConfig{
		ID:             "c1",
		OverlayID:      "o1",
		VisualSettings: visual,
	}
}

func putConfig(t *testing.T, r *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/o1/config", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBubbleColorsGateReportedOnRead(t *testing.T) {
	for _, tc := range []struct {
		name       string
		gate       BubbleColorsGate
		wantLocked bool
	}{
		// The seeded state: gate row is not premium, so the feature is open.
		{name: "gate open", gate: stubBubbleGate{enabled: true}, wantLocked: false},
		{name: "gate closed", gate: stubBubbleGate{enabled: false}, wantLocked: true},
		// Fail closed. Handing back an editable control whose value the save path
		// will drop is the exact failure this feature was built to remove.
		{name: "gate lookup error", gate: stubBubbleGate{err: errors.New("db down")}, wantLocked: true},
		// A service wired without a gate behaves as the seeded state does.
		{name: "no gate wired", gate: nil, wantLocked: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := bubbleRouter(&capturingConfigRepo{cfg: storedConfig(nil)}, tc.gate)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/o1/config", nil))
			require.Equal(t, http.StatusOK, w.Code)

			var got map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, tc.wantLocked, got["bubble_colors_locked"])
			// The flag rides alongside the config, it does not replace it.
			assert.Equal(t, "o1", got["overlay_id"])
		})
	}
}

func TestBubbleColorsAcceptedWhenGateOpen(t *testing.T) {
	repo := &capturingConfigRepo{cfg: storedConfig(nil)}
	r := bubbleRouter(repo, stubBubbleGate{enabled: true})

	w := putConfig(t, r, map[string]any{
		"visual_settings": map[string]any{
			"bubblePalette":  []string{"#111111", "#222222"},
			"twitchBubbleBg": "#2a1b3d",
			"fontFamily":     "Inter",
		},
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.saved)
	assert.Equal(t, "#2a1b3d", repo.saved.VisualSettings["twitchBubbleBg"])
	assert.Len(t, repo.saved.VisualSettings["bubblePalette"], 2)
}

// The save must not fail: this route persists the whole config, so rejecting it
// would block unrelated edits (theme, fonts, filters) over a cosmetic setting.
func TestBubbleColorsDroppedButRestOfConfigSavedWhenLocked(t *testing.T) {
	repo := &capturingConfigRepo{cfg: storedConfig(nil)}
	r := bubbleRouter(repo, stubBubbleGate{enabled: false})

	w := putConfig(t, r, map[string]any{
		"visual_settings": map[string]any{
			"bubblePalette":   []string{"#111111", "#222222"},
			"twitchBubbleBg":  "#2a1b3d",
			"discordBubbleBg": "#22253d",
			"fontFamily":      "Inter",
		},
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.saved)
	for _, key := range bubbleColorSettings {
		assert.NotContains(t, repo.saved.VisualSettings, key, key+" must not be settable while locked")
	}
	assert.Equal(t, "Inter", repo.saved.VisualSettings["fontFamily"])
}

// Carrying over rather than deleting matters for a lapsed subscriber: an
// unrelated save must not silently wipe colours configured while the gate was
// open, and must not let them be changed either.
func TestBubbleColorsCarriedOverWhenLocked(t *testing.T) {
	repo := &capturingConfigRepo{cfg: storedConfig(map[string]any{
		"twitchBubbleBg": "#OLD",
		"bubblePalette":  []any{"#aaaaaa", "#bbbbbb"},
	})}
	r := bubbleRouter(repo, stubBubbleGate{enabled: false})

	w := putConfig(t, r, map[string]any{
		"visual_settings": map[string]any{
			"twitchBubbleBg": "#NEW",
			"fontFamily":     "Inter",
		},
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.saved)
	assert.Equal(t, "#OLD", repo.saved.VisualSettings["twitchBubbleBg"])
	assert.Len(t, repo.saved.VisualSettings["bubblePalette"], 2)
	assert.Equal(t, "Inter", repo.saved.VisualSettings["fontFamily"])
}

// Every key the frontend can send has to be covered, or the gate leaks. Names
// mirror VisualSettings in frontend/src/lib/types/visual-settings.ts.
func TestBubbleColorSettingsCoversEveryGatedField(t *testing.T) {
	assert.ElementsMatch(t, []string{
		"bubblePalette",
		"twitchBubbleBg",
		"youtubeBubbleBg",
		"kickBubbleBg",
		"tiktokBubbleBg",
		"discordBubbleBg",
	}, bubbleColorSettings)
}
