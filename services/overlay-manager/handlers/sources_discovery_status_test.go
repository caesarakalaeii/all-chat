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

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// listSourcesWithStatusSnapshots serves GET /overlays/:id/sources for the given sources,
// against a Redis pre-loaded with the platform:status snapshots in snapshots, and returns
// the decoded response body.
func listSourcesWithStatusSnapshots(
	t *testing.T,
	sources []*models.ChatSource,
	snapshots map[string]string,
) []map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)

	server := miniredis.RunT(t)
	for key, value := range snapshots {
		require.NoError(t, server.Set(key, value))
	}
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	handler := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			listByOverlayFunc: func(context.Context, string) ([]*models.ChatSource, error) {
				return sources, nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(ctx context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		redis:  redisClient,
		logger: zap.NewNop(),
	}

	router := gin.New()
	router.GET("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.HandleListSources(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/overlays/overlay-1/sources", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func youtubeSource(channelID string) *models.ChatSource {
	return &models.ChatSource{
		ID:          "source-" + channelID,
		OverlayID:   "overlay-1",
		Platform:    "youtube",
		ChannelID:   channelID,
		ChannelName: channelID,
		IsActive:    true,
	}
}

// The dashboard reads discovery_status off the source list it already fetches, which is
// the only way a streamer who never opens the chat monitor learns their channel is parked.
func TestListSourcesReportsPausedDiscovery(t *testing.T) {
	body := listSourcesWithStatusSnapshots(t,
		[]*models.ChatSource{youtubeSource("UCparked")},
		map[string]string{
			"platform:status:youtube:UCparked": `{"platform":"youtube","channel_id":"UCparked","status":"paused","error_message":"No live stream found after 1h"}`,
		},
	)

	require.Len(t, body, 1)
	assert.Equal(t, "paused", body[0]["discovery_status"])
}

// An absent snapshot means "we do not know", not "everything is fine". Omitting the field
// is what stops the dashboard rendering a reassuring badge it has no evidence for.
func TestListSourcesOmitsDiscoveryStatusWithoutSnapshot(t *testing.T) {
	body := listSourcesWithStatusSnapshots(t,
		[]*models.ChatSource{youtubeSource("UCunknown")},
		nil,
	)

	require.Len(t, body, 1)
	assert.NotContains(t, body[0], "discovery_status")
}

// Only a parked channel is actionable. A connected or offline channel is ordinary
// lifecycle, and surfacing it on a dashboard card would be noise the streamer learns
// to ignore.
func TestListSourcesOmitsDiscoveryStatusForNonPausedSnapshot(t *testing.T) {
	body := listSourcesWithStatusSnapshots(t,
		[]*models.ChatSource{youtubeSource("UClive"), youtubeSource("UCoff")},
		map[string]string{
			"platform:status:youtube:UClive": `{"platform":"youtube","channel_id":"UClive","status":"connected"}`,
			"platform:status:youtube:UCoff":  `{"platform":"youtube","channel_id":"UCoff","status":"offline"}`,
		},
	)

	require.Len(t, body, 2)
	assert.NotContains(t, body[0], "discovery_status")
	assert.NotContains(t, body[1], "discovery_status")
}

// A source list is more useful without discovery state than not at all: Redis is HA and
// can refuse reads under node loss, and the sources themselves come from Postgres.
func TestListSourcesStillServesSourcesWhenRedisIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &SourcesHandler{
		sourceRepo: &mockSourceRepository{
			listByOverlayFunc: func(context.Context, string) ([]*models.ChatSource, error) {
				return []*models.ChatSource{youtubeSource("UCparked")}, nil
			},
		},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(ctx context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		redis:  nil, // REDIS_URL unset, or the client failed to construct
		logger: zap.NewNop(),
	}

	router := gin.New()
	router.GET("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		handler.HandleListSources(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/overlays/overlay-1/sources", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body []map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, "UCparked", body[0]["channel_id"])
	assert.NotContains(t, body[0], "discovery_status")
}
