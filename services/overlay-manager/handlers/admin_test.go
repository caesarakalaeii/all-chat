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

// recordingOverlayStore records the listing arguments and answers one row
// with a fixed total, so a test can check what the handler asked for.
type recordingOverlayStore struct {
	stubAdminOverlayStore
	filter        repository.OverlayListFilter
	limit, offset int
}

func (s *recordingOverlayStore) ListOverlaysWithSourceCount(_ context.Context, filter repository.OverlayListFilter, limit, offset int) ([]*repository.OverlayWithSourceCount, int, error) {
	s.filter, s.limit, s.offset = filter, limit, offset
	return []*repository.OverlayWithSourceCount{{Overlay: models.Overlay{ID: "o1", Name: "Main"}}}, 321, nil
}

type recordingSourceStore struct {
	filter        repository.SourceListFilter
	limit, offset int
}

func (s *recordingSourceStore) ListSourcesWithOverlay(_ context.Context, filter repository.SourceListFilter, limit, offset int) ([]*repository.SourceWithOverlay, int, error) {
	s.filter, s.limit, s.offset = filter, limit, offset
	return []*repository.SourceWithOverlay{{ChatSource: models.ChatSource{ID: "s1", ChannelName: "chan"}}}, 654, nil
}

func (s *recordingSourceStore) ListByOverlayID(context.Context, string) ([]*models.ChatSource, error) {
	panic("ListByOverlayID: not expected")
}

func serveAdmin(t *testing.T, route string, handle gin.HandlerFunc, url string) []byte {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(route, handle)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	return w.Body.Bytes()
}

func TestAdminListOverlays_Paginates(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		wantFilter repository.OverlayListFilter
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "/admin/overlays", repository.OverlayListFilter{}, 50, 0},
		{"search, ids and page", "/admin/overlays?q=+main+&ids=a,,b&limit=20&offset=40", repository.OverlayListFilter{Query: "main", IDs: []string{"a", "b"}}, 20, 40},
		{"limit is capped", "/admin/overlays?limit=999", repository.OverlayListFilter{}, 100, 0},
		{"junk paging falls back", "/admin/overlays?limit=0&offset=-1", repository.OverlayListFilter{}, 50, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingOverlayStore{}
			h := NewAdminHandler(store, nil, zap.NewNop())
			raw := serveAdmin(t, "/admin/overlays", h.ListOverlays, tc.url)

			assert.Equal(t, tc.wantFilter, store.filter)
			assert.Equal(t, tc.wantLimit, store.limit)
			assert.Equal(t, tc.wantOffset, store.offset)
			var body struct {
				Overlays []map[string]any `json:"overlays"`
				Total    int              `json:"total"`
			}
			require.NoError(t, json.Unmarshal(raw, &body))
			require.Len(t, body.Overlays, 1)
			assert.Equal(t, "Main", body.Overlays[0]["name"])
			assert.Equal(t, 321, body.Total)
		})
	}
}

func TestAdminListSources_Paginates(t *testing.T) {
	active, inactive := true, false
	cases := []struct {
		name       string
		url        string
		wantFilter repository.SourceListFilter
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "/admin/sources", repository.SourceListFilter{}, 50, 0},
		{"every filter", "/admin/sources?q=+chan+&platform=twitch&status=active&user=u1&limit=10&offset=30", repository.SourceListFilter{Query: "chan", Platform: "twitch", Active: &active, UserID: "u1"}, 10, 30},
		{"inactive", "/admin/sources?status=inactive", repository.SourceListFilter{Active: &inactive}, 50, 0},
		{"unknown status means any", "/admin/sources?status=all", repository.SourceListFilter{}, 50, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingSourceStore{}
			h := NewAdminHandler(nil, store, zap.NewNop())
			raw := serveAdmin(t, "/admin/sources", h.ListAllSources, tc.url)

			assert.Equal(t, tc.wantFilter, store.filter)
			assert.Equal(t, tc.wantLimit, store.limit)
			assert.Equal(t, tc.wantOffset, store.offset)
			var body struct {
				Sources []map[string]any `json:"sources"`
				Total   int              `json:"total"`
			}
			require.NoError(t, json.Unmarshal(raw, &body))
			require.Len(t, body.Sources, 1)
			assert.Equal(t, "chan", body.Sources[0]["channel_name"])
			assert.Equal(t, 654, body.Total)
		})
	}
}
