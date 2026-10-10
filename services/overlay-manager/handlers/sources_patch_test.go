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
	createFunc        func(context.Context, *models.ChatSource) error
	listByOverlayFunc func(context.Context, string) ([]*models.ChatSource, error)
	getByIDFunc       func(context.Context, string) (*models.ChatSource, error)
	deleteFunc        func(context.Context, string) error
	updateConfigFunc  func(context.Context, string, map[string]interface{}) error
}

func (m *mockSourceRepositoryWithConfig) Create(ctx context.Context, source *models.ChatSource) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, source)
	}
	return nil
}

func (m *mockSourceRepositoryWithConfig) CreateOrUpdateAuto(ctx context.Context, source *models.ChatSource) error {
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

func (m *mockSourceRepositoryWithConfig) ListByOverlayIDForUser(ctx context.Context, overlayID, _ string) ([]*models.ChatSource, error) {
	return m.ListByOverlayID(ctx, overlayID)
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
//
// Since ADR-0048 the handler also requires the source to exist on the overlay named in the
// path, and — for a Discord source — that every channel in the config sits in a guild the
// caller has connected. The stored source and the approving guard below satisfy both.
func TestHandleUpdateSourceConfig_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedID string
	var capturedConfig map[string]interface{}

	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{
			getByIDFunc: func(_ context.Context, id string) (*models.ChatSource, error) {
				return &models.ChatSource{
					ID:        id,
					OverlayID: "overlay-id",
					Platform:  "discord",
					ChannelID: "987654321",
				}, nil
			},
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
	h.SetDiscordGuard(
		&mockDiscordChannelResolver{guilds: map[string]string{"987654321": "123456789"}},
		&mockDiscordGuildOwnership{owned: map[string]bool{"test-user-id|123456789": true}},
	)

	router := setupPatchRouter(h)

	body := map[string]interface{}{
		"config": map[string]interface{}{
			"guild_id":           "123456789",
			"inbound_channel_id": "987654321",
			"relay_enabled":      true,
			"relay_channel_id":   nil,
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

const (
	pinSourceChannel  = "UCsourcesourcesourcesour"
	pinForeignChannel = "UCforeignforeignforeignf"
	pinVideo          = "dQw4w9WgXcQ"
)

// fakeYouTubeVideoChannels maps video ids to their uploading channel and counts lookups,
// so a test can prove an unchanged pin never reaches the network.
type fakeYouTubeVideoChannels struct {
	channels map[string]string
	err      error
	calls    int
}

func (f *fakeYouTubeVideoChannels) VideoChannelID(_ context.Context, videoID string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	ch, ok := f.channels[videoID]
	if !ok {
		return "", errors.New("video not found")
	}
	return ch, nil
}

type pinPatchResult struct {
	w       *httptest.ResponseRecorder
	saved   map[string]interface{}
	updated bool
}

// patchYouTubePin PATCHes a YouTube source whose stored config is `stored` with `config`.
func patchYouTubePin(t *testing.T, videos *fakeYouTubeVideoChannels, stored, config map[string]interface{}) pinPatchResult {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var res pinPatchResult
	h := buildPatchHandler(
		&mockSourceRepositoryWithConfig{
			getByIDFunc: func(_ context.Context, id string) (*models.ChatSource, error) {
				return &models.ChatSource{
					ID:        id,
					OverlayID: "overlay-id",
					Platform:  "youtube",
					ChannelID: pinSourceChannel,
					Config:    stored,
				}, nil
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
	h.SetYouTubeVideoResolver(videos)

	bodyBytes, err := json.Marshal(map[string]interface{}{"config": config})
	assert.NoError(t, err)
	res.w = httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	setupPatchRouter(h).ServeHTTP(res.w, req)
	return res
}

func TestPatchYouTubePin_StoresNormalized(t *testing.T) {
	videos := &fakeYouTubeVideoChannels{channels: map[string]string{pinVideo: pinSourceChannel}}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{"stream_select": "first_found"},
		map[string]interface{}{"stream_select": "first_found", "stream_id": "https://youtu.be/" + pinVideo + "?si=share"},
	)

	assert.Equal(t, http.StatusOK, res.w.Code, res.w.Body.String())
	assert.True(t, res.updated)
	assert.Equal(t, pinVideo, res.saved["stream_id"])
	assert.Equal(t, "first_found", res.saved["stream_select"])
	assert.Equal(t, 1, videos.calls)
}

func TestPatchYouTubePin_MismatchedChannel422(t *testing.T) {
	videos := &fakeYouTubeVideoChannels{channels: map[string]string{pinVideo: pinForeignChannel}}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{},
		map[string]interface{}{"stream_id": "https://www.youtube.com/watch?v=" + pinVideo},
	)

	assert.Equal(t, http.StatusUnprocessableEntity, res.w.Code, res.w.Body.String())
	assert.False(t, res.updated)
	assert.Equal(t, 1, videos.calls)
}

func TestPatchYouTubePin_Garbage400(t *testing.T) {
	videos := &fakeYouTubeVideoChannels{channels: map[string]string{pinVideo: pinSourceChannel}}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{},
		map[string]interface{}{"stream_id": "https://www.youtube.com/channel/" + pinSourceChannel},
	)

	assert.Equal(t, http.StatusBadRequest, res.w.Code, res.w.Body.String())
	assert.False(t, res.updated)
	assert.Equal(t, 0, videos.calls)
}

func TestPatchYouTubePin_ClearRemovesKey(t *testing.T) {
	videos := &fakeYouTubeVideoChannels{err: errors.New("must not be called")}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{"stream_id": pinVideo, "stream_select": "first_found"},
		map[string]interface{}{"stream_id": "", "stream_select": "first_found"},
	)

	assert.Equal(t, http.StatusOK, res.w.Code, res.w.Body.String())
	assert.True(t, res.updated)
	_, present := res.saved["stream_id"]
	assert.False(t, present, "cleared pin must be removed from the stored config")
	assert.Equal(t, "first_found", res.saved["stream_select"])
	assert.Equal(t, 0, videos.calls)
}

func TestPatchYouTubePin_UnchangedPinNoLookup(t *testing.T) {
	// The frontend PATCHes the whole config on every settings save, so an untouched pin
	// riding along with another change must not cost (or fail on) a lookup.
	videos := &fakeYouTubeVideoChannels{err: errors.New("must not be called")}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{"stream_id": pinVideo, "stream_select": "first_found"},
		map[string]interface{}{"stream_id": pinVideo, "stream_select": "most_viewers"},
	)

	assert.Equal(t, http.StatusOK, res.w.Code, res.w.Body.String())
	assert.True(t, res.updated)
	assert.Equal(t, pinVideo, res.saved["stream_id"])
	assert.Equal(t, "most_viewers", res.saved["stream_select"])
	assert.Equal(t, 0, videos.calls)
}

func TestPatchYouTubeConfig_UnverifiableLink422(t *testing.T) {
	// oEmbed refusing the video (private, embedding disabled, outage) must not store an
	// unchecked pin.
	videos := &fakeYouTubeVideoChannels{err: errors.New("oembed returned status 401")}
	res := patchYouTubePin(t, videos,
		map[string]interface{}{},
		map[string]interface{}{"stream_id": pinVideo},
	)

	assert.Equal(t, http.StatusUnprocessableEntity, res.w.Code, res.w.Body.String())
	assert.False(t, res.updated)
	assert.Equal(t, 1, videos.calls)
}

// Without its collaborators wired, the handler must refuse to store a new pin rather
// than accept it unchecked.
func TestPatchYouTubeConfig_UnwiredFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, config := range map[string]map[string]interface{}{
		"new pin": {"stream_id": pinVideo},
	} {
		t.Run(name, func(t *testing.T) {
			updated := false
			h := buildPatchHandler(
				&mockSourceRepositoryWithConfig{
					getByIDFunc: func(_ context.Context, id string) (*models.ChatSource, error) {
						return &models.ChatSource{ID: id, OverlayID: "overlay-id", Platform: "youtube", ChannelID: pinSourceChannel}, nil
					},
					updateConfigFunc: func(context.Context, string, map[string]interface{}) error {
						updated = true
						return nil
					},
				},
				&mockOverlayRepository{
					getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
						return &models.Overlay{ID: id, UserID: userID, Name: "Test"}, nil
					},
				},
			)

			bodyBytes, err := json.Marshal(map[string]interface{}{"config": config})
			assert.NoError(t, err)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PATCH", "/overlays/overlay-id/sources/source-id", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			setupPatchRouter(h).ServeHTTP(w, req)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assert.False(t, updated)
		})
	}
}
