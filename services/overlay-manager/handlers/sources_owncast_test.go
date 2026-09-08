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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ADR-0058: an Owncast "channel" is an instance URL. Everything here guards
// that contract: URL-shaped channel_id, normalization, and display-name
// resolution that must survive a down instance.

func owncastTestHandler(captured **models.ChatSource, configServerURL string) *SourcesHandler {
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			createFunc: func(_ context.Context, s *models.ChatSource) error {
				if captured != nil {
					*captured = s
				}
				return nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(_ context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		db:     nil,
		logger: zap.NewNop(),
	}
	if configServerURL != "" {
		// Route /api/config through the fake instance regardless of the
		// instance URL in the request body.
		h.httpClient = &http.Client{Transport: rewriteTransport{target: configServerURL}}
	}
	return h
}

// rewriteTransport sends every request to the fake Owncast instance, standing
// in for the user-supplied instance URL.
type rewriteTransport struct {
	target string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target[:len("http")]
	req.URL.Host = t.target[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}

func postOwncastSource(h *SourcesHandler, body map[string]interface{}) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.HandleAddSource(c)
	})
	bodyBytes, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/overlays/overlay-id/sources", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestHandleAddSource_Owncast_NormalizesURL(t *testing.T) {
	var captured *models.ChatSource
	h := owncastTestHandler(&captured, "")

	w := postOwncastSource(h, map[string]interface{}{
		"platform":   "owncast",
		"channel_id": "HTTPS://Watch.Example.ORG/",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, captured)
	assert.Equal(t, "https://watch.example.org", captured.ChannelID, "host lowercased, trailing slash stripped")
}

// A non-URL channel_id is rejected with 400 — Owncast has no channel names,
// only instances.
func TestHandleAddSource_Owncast_RejectsNonURL(t *testing.T) {
	var captured *models.ChatSource
	h := owncastTestHandler(&captured, "")

	for _, bad := range []string{"somechannel", "ftp://x.example.org", "https://", "https://watch.example.org/embed", "https://watch.example.org/#frag"} {
		w := postOwncastSource(h, map[string]interface{}{
			"platform":   "owncast",
			"channel_id": bad,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code, "channel_id %q must be rejected", bad)
		assert.Nil(t, captured, "no source row may be created for %q", bad)
	}
}

// /api/config name resolution: a live instance name becomes channel_name.
func TestHandleAddSource_Owncast_ResolvesNameFromConfig(t *testing.T) {
	var captured *models.ChatSource
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/config", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": "Gabek's Corner"})
	}))
	defer srv.Close()

	h := owncastTestHandler(&captured, srv.URL)
	w := postOwncastSource(h, map[string]interface{}{
		"platform":   "owncast",
		"channel_id": "https://watch.example.org",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, captured)
	assert.Equal(t, "Gabek's Corner", captured.ChannelName)
}

// An OFFLINE instance must not fail the add (ADR-0058): name falls back to
// the URL hostname.
func TestHandleAddSource_Owncast_OfflineInstanceStillAddable(t *testing.T) {
	var captured *models.ChatSource
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := owncastTestHandler(&captured, srv.URL)
	w := postOwncastSource(h, map[string]interface{}{
		"platform":   "owncast",
		"channel_id": "https://watch.example.org",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, captured)
	assert.Equal(t, "watch.example.org", captured.ChannelName, "hostname fallback on config fetch failure")
}

// A refused connection (server closed before use) also degrades to the
// hostname rather than failing the add.
func TestHandleAddSource_Owncast_ConnectionRefusedFallsBack(t *testing.T) {
	var captured *models.ChatSource
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	target := srv.URL
	srv.Close() // port now closed — every request fails at dial

	h := owncastTestHandler(&captured, target)
	w := postOwncastSource(h, map[string]interface{}{
		"platform":   "owncast",
		"channel_id": "https://watch.example.org",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, captured)
	assert.Equal(t, "watch.example.org", captured.ChannelName)
}

// Explicit channel_name provided by the caller wins over /api/config.
func TestHandleAddSource_Owncast_ExplicitNameWins(t *testing.T) {
	var captured *models.ChatSource
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": "Server Name"})
	}))
	defer srv.Close()

	h := owncastTestHandler(&captured, srv.URL)
	w := postOwncastSource(h, map[string]interface{}{
		"platform":     "owncast",
		"channel_id":   "https://watch.example.org",
		"channel_name": "My Own Name",
	})

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.NotNil(t, captured)
	assert.Equal(t, "My Own Name", captured.ChannelName)
}

// The listener is read-only: no auth, no OAuth.
func TestHandleAddSource_Owncast_NoOAuthRequired(t *testing.T) {
	var captured *models.ChatSource
	h := owncastTestHandler(&captured, "")

	w := postOwncastSource(h, map[string]interface{}{
		"platform":   "owncast",
		"channel_id": "https://watch.example.org",
	})

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, captured)
	assert.False(t, captured.AuthRequired)
}

func TestNormalizeOwncastInstanceURL_Unit(t *testing.T) {
	got, err := normalizeOwncastInstanceURL("  HTTP://Watch.Example.ORG  ")
	require.NoError(t, err)
	assert.Equal(t, "http://watch.example.org", got)

	_, err = normalizeOwncastInstanceURL("")
	assert.Error(t, err)
	_, err = normalizeOwncastInstanceURL("https://u:p@watch.example.org")
	assert.Error(t, err, "embedded credentials must be rejected")
}
