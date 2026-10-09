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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// overlay_type (ADR-0064): overlays come in four kinds. Only chat overlays have
// chat sources; the responses must say which kind an overlay is so the
// frontend can route its render page.

func decodeOverlayBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	return body
}

func TestHandleCreateOverlay_OverlayType(t *testing.T) {
	tests := []struct {
		name           string
		requestType    any // nil = omit from the request body
		wantType       string
		wantStatusCode int
	}{
		{name: "absent resolves to chat", requestType: nil, wantType: "chat", wantStatusCode: http.StatusCreated},
		{name: "chat", requestType: "chat", wantType: "chat", wantStatusCode: http.StatusCreated},
		{name: "alerts is not available yet", requestType: "alerts", wantStatusCode: http.StatusForbidden},
		{name: "goal is not available yet", requestType: "goal", wantStatusCode: http.StatusForbidden},
		{name: "list is not available yet", requestType: "list", wantStatusCode: http.StatusForbidden},
		{
			name:           "unknown type is rejected before the DB is touched",
			requestType:    "webcam",
			wantStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var created *models.Overlay
			repo := &mockOverlayRepository{
				listByUserIDFunc: func(context.Context, string) ([]*models.Overlay, error) {
					return []*models.Overlay{}, nil
				},
				createFunc: func(_ context.Context, overlay *models.Overlay) error {
					created = overlay
					return nil
				},
			}
			h := NewOverlayHandler(repo, &mockSourceRepository{}, &capturingConfigRepo{})

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("user_id", "user-1") })
			router.POST("/", h.HandleCreateOverlay)

			reqBody := map[string]any{"name": "My Overlay"}
			if tt.requestType != nil {
				reqBody["overlay_type"] = tt.requestType
			}
			raw, err := json.Marshal(reqBody)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)
			if tt.wantStatusCode != http.StatusCreated {
				assert.Nil(t, created, "an invalid kind must not reach the repository")
				return
			}
			body := decodeOverlayBody(t, w)
			assert.Equal(t, tt.wantType, body["overlay_type"])
			require.NotNil(t, created)
			assert.Equal(t, tt.wantType, created.OverlayType)
		})
	}
}

func TestHandleCloneOverlay_CarriesOverlayType(t *testing.T) {
	source := &models.Overlay{
		ID:          "overlay-1",
		UserID:      "user-1",
		Name:        "Alerts overlay",
		OverlayType: models.OverlayTypeAlerts,
	}
	var created *models.Overlay
	repo := &mockOverlayRepository{
		getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
			assert.Equal(t, "overlay-1", id)
			assert.Equal(t, "user-1", userID)
			return source, nil
		},
		listByUserIDFunc: func(context.Context, string) ([]*models.Overlay, error) {
			return nil, nil
		},
		createFunc: func(_ context.Context, overlay *models.Overlay) error {
			created = overlay
			return nil
		},
	}
	h := NewOverlayHandler(repo, &mockSourceRepository{}, &capturingConfigRepo{})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "user-1") })
	router.POST("/:id/clone", h.HandleCloneOverlay)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/overlay-1/clone", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, created)
	assert.Equal(t, models.OverlayTypeAlerts, created.OverlayType,
		"cloning must not silently turn an alerts overlay into a chat overlay")
	assert.Equal(t, models.OverlayTypeAlerts, decodeOverlayBody(t, w)["overlay_type"])
}

// overlayTypeSourceRouter wires a SourcesHandler whose overlay repo returns an
// overlay of the given kind, and a capture hook on source creation.
func overlayTypeSourceRouter(overlayType string, created **models.ChatSource) *gin.Engine {
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(_ context.Context, s *models.ChatSource) error {
				if created != nil {
					*created = s
				}
				return nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Typed overlay", OverlayType: overlayType}, nil
			},
		},
		logger: zap.NewNop(),
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "user-1") })
	router.POST("/overlays/:id/sources", h.HandleAddSource)
	router.POST("/internal/overlays/:id/sources/auto", h.HandleAddSourceAuto)
	return router
}

func postSource(router *gin.Engine, path string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(map[string]any{"platform": "twitch", "channel_id": "somechannel"})
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHandleAddSource_RejectsNonChatOverlayTypes(t *testing.T) {
	// Only chat overlays carry chat sources: an alerts/goal/list overlay has no
	// source pipeline, so a source row would be inert data that still shows up
	// in admin listings and source counts.
	for _, overlayType := range []string{models.OverlayTypeAlerts, models.OverlayTypeGoal, models.OverlayTypeList} {
		t.Run(overlayType, func(t *testing.T) {
			var created *models.ChatSource
			router := overlayTypeSourceRouter(overlayType, &created)

			w := postSource(router, "/overlays/overlay-1/sources")

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Contains(t, body["error"], overlayType, "the error must name the kind so the streamer can act on it")
			assert.Contains(t, body["error"], "chat", "the error must say chat sources are the problem")
			assert.Nil(t, created, "no source row may be created on a non-chat overlay")
		})
	}

	t.Run("chat overlay still accepts sources", func(t *testing.T) {
		var created *models.ChatSource
		router := overlayTypeSourceRouter(models.OverlayTypeChat, &created)

		w := postSource(router, "/overlays/overlay-1/sources")

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.NotNil(t, created)
	})
}

func TestHandleAddSourceAuto_RejectsNonChatOverlayTypes(t *testing.T) {
	var created *models.ChatSource
	router := overlayTypeSourceRouter(models.OverlayTypeGoal, &created)

	w := postSource(router, "/internal/overlays/overlay-1/sources/auto")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Nil(t, created,
		"the internal OAuth callback must not strand a chat source on a goal overlay either")
}

func TestHandleGetConfig_ReturnsOverlayType(t *testing.T) {
	overlays := &stubOverlayRepo{owned: true}
	h := NewConfigHandler(&capturingConfigRepo{cfg: storedConfig(nil)}, overlays, &stubSourceRepo{}, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
	router.GET("/:id/config", h.HandleGetConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/o1/config", nil))

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeOverlayBody(t, w)
	assert.Equal(t, models.OverlayTypeChat, body["overlay_type"],
		"the editor needs the kind to route its settings surfaces")
}

func TestHandleGetPublicConfig_ReturnsOverlayType(t *testing.T) {
	overlays := &stubOverlayRepo{owned: true}
	h := NewConfigHandler(&capturingConfigRepo{cfg: storedConfig(nil)}, overlays, &stubSourceRepo{}, nil, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/public/:id/config", h.HandleGetPublicConfig)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public/o1/config", nil))

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeOverlayBody(t, w)
	assert.Equal(t, models.OverlayTypeChat, body["overlay_type"],
		"the unauthenticated render page routes on the kind and has no other way to learn it")
}

func TestHandleGetConfig_TypedOverlayCarriesItsKind(t *testing.T) {
	for _, kind := range []string{
		models.OverlayTypeAlerts, models.OverlayTypeGoal, models.OverlayTypeList,
	} {
		t.Run(kind, func(t *testing.T) {
			overlays := &stubOverlayRepo{
				owned:   true,
				overlay: &models.Overlay{ID: "o1", UserID: "u1", OverlayType: kind},
			}
			h := NewConfigHandler(&capturingConfigRepo{cfg: storedConfig(nil)}, overlays, &stubSourceRepo{}, nil, nil)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
			router.GET("/:id/config", h.HandleGetConfig)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/o1/config", nil))

			require.Equal(t, http.StatusOK, w.Code)
			body := decodeOverlayBody(t, w)
			assert.Equal(t, kind, body["overlay_type"],
				"a typed overlay's config must carry its actual kind, not the chat default")
		})
	}
}

func TestHandleGetPublicConfig_TypedOverlayCarriesItsKind(t *testing.T) {
	for _, kind := range []string{
		models.OverlayTypeAlerts, models.OverlayTypeGoal, models.OverlayTypeList,
	} {
		t.Run(kind, func(t *testing.T) {
			overlays := &stubOverlayRepo{
				overlay: &models.Overlay{ID: "o1", UserID: "u1", OverlayType: kind},
			}
			h := NewConfigHandler(&capturingConfigRepo{cfg: storedConfig(nil)}, overlays, &stubSourceRepo{}, nil, nil)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/public/:id/config", h.HandleGetPublicConfig)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public/o1/config", nil))

			require.Equal(t, http.StatusOK, w.Code)
			body := decodeOverlayBody(t, w)
			assert.Equal(t, kind, body["overlay_type"],
				"the unauthenticated render page routes on the kind and must see the real one")
		})
	}
}

// stubAdminOverlayStore hands the admin handler a fixed overlay list for both
// of its listing routes, standing in for the owner-join repository queries.
type stubAdminOverlayStore struct {
	overlays []*repository.OverlayWithSourceCount
}

func (s *stubAdminOverlayStore) GetAllOverlaysWithSourceCount(context.Context) ([]*repository.OverlayWithSourceCount, error) {
	return s.overlays, nil
}

func (s *stubAdminOverlayStore) ListByUserIDWithSourceCount(_ context.Context, _ string) ([]*repository.OverlayWithSourceCount, error) {
	return s.overlays, nil
}

// The admin listing responses must say each overlay's kind: they are the only
// responses with no test of their own, so a refactor could drop the field with
// every gate still green. An empty type (a row written before the column
// existed, should one ever surface) must resolve to chat, like every other
// response path does.
func TestHandleAdminListOverlays_OverlayType(t *testing.T) {
	tests := []struct {
		name     string
		rowType  string
		wantType string
	}{
		{name: "chat", rowType: models.OverlayTypeChat, wantType: models.OverlayTypeChat},
		{name: "alerts", rowType: models.OverlayTypeAlerts, wantType: models.OverlayTypeAlerts},
		{name: "goal", rowType: models.OverlayTypeGoal, wantType: models.OverlayTypeGoal},
		{name: "list", rowType: models.OverlayTypeList, wantType: models.OverlayTypeList},
		{name: "empty resolves to chat", rowType: "", wantType: models.OverlayTypeChat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubAdminOverlayStore{overlays: []*repository.OverlayWithSourceCount{{
				Overlay: models.Overlay{ID: "overlay-1", UserID: "user-1", Name: "Their overlay", OverlayType: tt.rowType},
			}}}
			h := NewAdminHandler(repo, nil, zap.NewNop())

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/admin/overlays", h.ListOverlays)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/overlays", nil))

			require.Equal(t, http.StatusOK, w.Code)
			var body []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
			require.Len(t, body, 1)
			assert.Equal(t, tt.wantType, body[0]["overlay_type"])
		})
	}
}

func TestHandleAdminGetUserOverlays_OverlayType(t *testing.T) {
	repo := &stubAdminOverlayStore{overlays: []*repository.OverlayWithSourceCount{{
		Overlay: models.Overlay{ID: "overlay-1", UserID: "user-1", Name: "Their overlay", OverlayType: models.OverlayTypeAlerts},
	}}}
	h := NewAdminHandler(repo, nil, zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/user-overlays/:id", h.GetUserOverlays)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/user-overlays/user-1", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	require.Len(t, body, 1)
	assert.Equal(t, models.OverlayTypeAlerts, body[0]["overlay_type"])
}
