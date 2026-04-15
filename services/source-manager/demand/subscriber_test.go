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

package demand_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/source-manager/demand"
	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockRepository is a test double for the repository
type mockRepository struct {
	sources map[string][]*models.ActiveSource
}

func (m *mockRepository) GetSourcesForOverlays(ctx context.Context, overlayIDs []string) ([]*models.ActiveSource, error) {
	result := make([]*models.ActiveSource, 0)
	for _, id := range overlayIDs {
		if sources, ok := m.sources[id]; ok {
			result = append(result, sources...)
		}
	}
	return result, nil
}

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

// TestOverlayDemandSubscriber_Connected tests that connecting an overlay adds its sources to demand
func TestOverlayDemandSubscriber_Connected(t *testing.T) {
	mr, client := newTestRedis(t)
	_ = mr

	repo := &mockRepository{
		sources: map[string][]*models.ActiveSource{
			"overlay-abc": {
				{
					ID:        "source-1",
					OverlayID: "overlay-abc",
					Platform:  "tiktok",
					ChannelID: "creator123",
				},
			},
		},
	}

	sub := demand.NewOverlayDemandSubscriber(client, repo, zap.NewNop())

	// Simulate a connected event
	event := map[string]interface{}{
		"type":       "connected",
		"overlay_id": "overlay-abc",
		"timestamp":  time.Now(),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = sub.HandleConnectionEventForTest(context.Background(), string(payload))
	require.NoError(t, err)

	sources := sub.GetDemandedSources()
	require.Len(t, sources, 1)
	assert.Equal(t, "source-1", sources[0].SourceID)
	assert.Equal(t, "overlay-abc", sources[0].OverlayID)
	assert.Equal(t, "tiktok", sources[0].Platform)
	assert.Equal(t, "creator123", sources[0].ChannelID)
}

// TestOverlayDemandSubscriber_Disconnected tests that disconnecting removes overlay sources from demand
func TestOverlayDemandSubscriber_Disconnected(t *testing.T) {
	mr, client := newTestRedis(t)
	_ = mr

	repo := &mockRepository{
		sources: map[string][]*models.ActiveSource{
			"overlay-abc": {
				{
					ID:        "source-1",
					OverlayID: "overlay-abc",
					Platform:  "tiktok",
					ChannelID: "creator123",
				},
			},
		},
	}

	sub := demand.NewOverlayDemandSubscriber(client, repo, zap.NewNop())

	// First connect
	connectEvent := map[string]interface{}{
		"type":       "connected",
		"overlay_id": "overlay-abc",
		"timestamp":  time.Now(),
	}
	connectPayload, err := json.Marshal(connectEvent)
	require.NoError(t, err)
	err = sub.HandleConnectionEventForTest(context.Background(), string(connectPayload))
	require.NoError(t, err)

	// Verify connected
	sources := sub.GetDemandedSources()
	require.Len(t, sources, 1)

	// Now disconnect
	disconnectEvent := map[string]interface{}{
		"type":       "disconnected",
		"overlay_id": "overlay-abc",
		"timestamp":  time.Now(),
	}
	disconnectPayload, err := json.Marshal(disconnectEvent)
	require.NoError(t, err)
	err = sub.HandleConnectionEventForTest(context.Background(), string(disconnectPayload))
	require.NoError(t, err)

	// Verify removed
	sources = sub.GetDemandedSources()
	assert.Empty(t, sources)
}

// TestStartupHydration tests that Start() hydrates demand from existing overlay:connected:* keys
func TestStartupHydration(t *testing.T) {
	mr, client := newTestRedis(t)

	// Pre-seed two overlay keys in Redis
	mr.Set("overlay:connected:overlay-1", "1")
	mr.Set("overlay:connected:overlay-2", "1")

	repo := &mockRepository{
		sources: map[string][]*models.ActiveSource{
			"overlay-1": {
				{
					ID:        "source-1",
					OverlayID: "overlay-1",
					Platform:  "tiktok",
					ChannelID: "channel-1",
				},
			},
			"overlay-2": {
				{
					ID:        "source-2",
					OverlayID: "overlay-2",
					Platform:  "twitch",
					ChannelID: "channel-2",
				},
			},
		},
	}

	sub := demand.NewOverlayDemandSubscriber(client, repo, zap.NewNop())

	err := sub.HydrateForTest(context.Background())
	require.NoError(t, err)

	sources := sub.GetDemandedSources()
	require.Len(t, sources, 2)

	ids := make([]string, 0, 2)
	for _, s := range sources {
		ids = append(ids, s.SourceID)
	}
	assert.ElementsMatch(t, []string{"source-1", "source-2"}, ids)
}

// TestDemandHandler_GetDemand tests that GET /demand?platform=tiktok returns only tiktok sources
func TestDemandHandler_GetDemand(t *testing.T) {
	_, client := newTestRedis(t)

	repo := &mockRepository{
		sources: map[string][]*models.ActiveSource{
			"overlay-abc": {
				{
					ID:        "source-1",
					OverlayID: "overlay-abc",
					Platform:  "tiktok",
					ChannelID: "creator123",
				},
				{
					ID:        "source-2",
					OverlayID: "overlay-abc",
					Platform:  "twitch",
					ChannelID: "twitchdude",
				},
			},
		},
	}

	sub := demand.NewOverlayDemandSubscriber(client, repo, zap.NewNop())

	// Connect overlay
	connectEvent := map[string]interface{}{
		"type":       "connected",
		"overlay_id": "overlay-abc",
		"timestamp":  time.Now(),
	}
	connectPayload, err := json.Marshal(connectEvent)
	require.NoError(t, err)
	err = sub.HandleConnectionEventForTest(context.Background(), string(connectPayload))
	require.NoError(t, err)

	// Filter by platform
	tiktokSources := sub.GetDemandedSourcesByPlatform("tiktok")
	require.Len(t, tiktokSources, 1)
	assert.Equal(t, "source-1", tiktokSources[0].SourceID)
	assert.Equal(t, "tiktok", tiktokSources[0].Platform)

	twitchSources := sub.GetDemandedSourcesByPlatform("twitch")
	require.Len(t, twitchSources, 1)
	assert.Equal(t, "source-2", twitchSources[0].SourceID)
}

// TestGetSourcesForOverlays tests the mockRepository directly (documents the interface contract)
func TestGetSourcesForOverlays(t *testing.T) {
	repo := &mockRepository{
		sources: map[string][]*models.ActiveSource{
			"overlay-1": {
				{ID: "source-1", OverlayID: "overlay-1", ChannelID: "chan-1", Platform: "tiktok"},
			},
			"overlay-2": {
				{ID: "source-2", OverlayID: "overlay-2", ChannelID: "chan-2", Platform: "twitch"},
			},
		},
	}

	ctx := context.Background()
	sources, err := repo.GetSourcesForOverlays(ctx, []string{"overlay-1", "overlay-2"})
	require.NoError(t, err)
	require.Len(t, sources, 2)

	ids := []string{sources[0].ID, sources[1].ID}
	assert.ElementsMatch(t, []string{"source-1", "source-2"}, ids)
}

// TestEmptyDemand tests that when no overlays are connected, DemandUpdate.Sources is empty (not nil)
func TestEmptyDemand(t *testing.T) {
	_, client := newTestRedis(t)

	repo := &mockRepository{sources: map[string][]*models.ActiveSource{}}
	sub := demand.NewOverlayDemandSubscriber(client, repo, zap.NewNop())

	sources := sub.GetDemandedSources()
	assert.NotNil(t, sources)
	assert.Empty(t, sources)
}
