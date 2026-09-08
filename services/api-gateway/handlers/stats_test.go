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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStatsTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestGetActiveOverlays_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, _ := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// Explicitly a JSON array, never null, so the frontend can iterate safely.
	assert.Equal(t, "[]", w.Body.String())
	var overlays []ActiveOverlay
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &overlays))
	assert.Empty(t, overlays)
}

func TestGetActiveOverlays_ReturnsConnectedIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, mr := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	// Simulate active overlays in Redis
	mr.Set("overlay:connected:abc-123", "1")
	mr.SetTTL("overlay:connected:abc-123", 10*time.Minute)
	mr.Set("overlay:connected:def-456", "1")
	mr.SetTTL("overlay:connected:def-456", 10*time.Minute)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var overlays []ActiveOverlay
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &overlays))
	assert.Len(t, overlays, 2)

	ids := make([]string, len(overlays))
	for i, o := range overlays {
		ids[i] = o.OverlayID
	}
	assert.ElementsMatch(t, []string{"abc-123", "def-456"}, ids)

	// No session hashes exist, so connected_since is omitted for every overlay.
	for _, o := range overlays {
		assert.Nil(t, o.ConnectedSince, "overlay %s should have no connected_since without a session", o.OverlayID)
	}
}

func TestGetPlatformStats_ShapeAndCounters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, mr := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	// One daily bucket for twitch (today), none for the other platforms —
	// days with no key must read as 0, and discord must appear in the map.
	today := time.Now().UTC().Format("2006-01-02")
	mr.Set("chat:stats:daily:twitch:"+today, "421")
	mr.Set("chat:stats:total", "48291803")
	mr.Set("stats:users:total", "12408")
	mr.Set("overlay:connected:abc-123", "1")
	mr.Set("overlay:connected:def-456", "1")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)

	handler.GetPlatformStats(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats PlatformStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, map[string]int64{
		"twitch": 421, "youtube": 0, "kick": 0, "tiktok": 0, "discord": 0,
	}, stats.Platforms)
	assert.Equal(t, int64(48291803), stats.AllTime)
	assert.Equal(t, int64(12408), stats.Users)
	assert.Equal(t, int64(2), stats.OverlaysLive)
}

func TestGetPlatformStats_MissingCountersAreZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, _ := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)

	handler.GetPlatformStats(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats PlatformStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(0), stats.AllTime)
	assert.Equal(t, int64(0), stats.Users)
	assert.Equal(t, int64(0), stats.OverlaysLive)
	// Fresh handler: memo is empty, so a missing-scan failure path must not
	// leak an error value into the response.
	for _, p := range stats.Platforms {
		assert.Equal(t, int64(0), p)
	}
}

func TestGetActiveOverlays_IncludesConnectedSince(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, mr := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	// Overlay with a session hash carrying a start time.
	startedAt := time.Now().UTC().Add(-90 * time.Minute).Truncate(time.Second)
	mr.Set("overlay:connected:with-session", "1")
	mr.SetTTL("overlay:connected:with-session", 10*time.Minute)
	mr.HSet("session:active:with-session", "started_at", startedAt.Format(time.RFC3339))

	// Overlay connected but without a session (e.g. post-disconnect linger).
	mr.Set("overlay:connected:no-session", "1")
	mr.SetTTL("overlay:connected:no-session", 5*time.Minute)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var overlays []ActiveOverlay
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &overlays))
	require.Len(t, overlays, 2)

	byID := make(map[string]ActiveOverlay, len(overlays))
	for _, o := range overlays {
		byID[o.OverlayID] = o
	}

	withSession, ok := byID["with-session"]
	require.True(t, ok)
	require.NotNil(t, withSession.ConnectedSince)
	assert.WithinDuration(t, startedAt, withSession.ConnectedSince.UTC(), time.Second)

	noSession, ok := byID["no-session"]
	require.True(t, ok)
	assert.Nil(t, noSession.ConnectedSince)
}

// TestGetActiveOverlays_EnrichmentFailureStillReturnsStatus guards the regression
// where a failure of the optional started_at enrichment discarded the entire
// connection-status answer: the endpoint 500'd and the admin UI (which silently
// ignores non-OK responses) showed every overlay as "not connected" even while
// live. A stray/mistyped session key (here a plain string where a hash is
// expected) makes the pipelined HGET return WRONGTYPE, which must degrade to
// "status without durations", not fail the whole request.
func TestGetActiveOverlays_EnrichmentFailureStillReturnsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, mr := setupStatsTestRedis(t)
	handler := NewStatsHandler(client, nil)

	mr.Set("overlay:connected:live-overlay", "1")
	mr.SetTTL("overlay:connected:live-overlay", 10*time.Minute)
	// Wrong type for the session key: HGET session:active:live-overlay -> WRONGTYPE.
	mr.Set("session:active:live-overlay", "not-a-hash")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code, "enrichment failure must not 500 the connection status")
	var overlays []ActiveOverlay
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &overlays))
	require.Len(t, overlays, 1)
	assert.Equal(t, "live-overlay", overlays[0].OverlayID)
	assert.Nil(t, overlays[0].ConnectedSince, "duration is dropped, but the overlay is still reported connected")
}
