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

package repository

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/shared/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoalConfig_ReadsTheOverlaysRow(t *testing.T) {
	db := &fakeQuerier{rows: fakeRows{rows: [][]any{{"bits", "twitch", int64(1000), int64(5), "Bits"}}}}
	cfg, err := NewRepository(db).GoalConfig(context.Background(), "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, &alerts.GoalConfig{Metric: "bits", Platform: "twitch", Target: 1000, StartingValue: 5, Title: "Bits"}, cfg)
	db.assertIssued(t, "FROM goal_configs", []any{"overlay-1"})
}

func TestGoalConfig_MissingRowIsNil(t *testing.T) {
	cfg, err := NewRepository(&fakeQuerier{}).GoalConfig(context.Background(), "overlay-1")
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

// Only active goal overlays with an API-polled metric and an active source on
// the goal's platform are polled; the filter lives in the SQL.
func TestPolledGoals_SelectsPolledMetricsWithASource(t *testing.T) {
	db := &fakeQuerier{rows: fakeRows{rows: [][]any{
		{"overlay-1", "user-1", "followers", "twitch", "somestreamer", int64(100), "Followers"},
	}}}
	goals, err := NewRepository(db).PolledGoals(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []PolledGoal{{
		OverlayID: "overlay-1", OwnerUserID: "user-1", Metric: "followers", Platform: "twitch",
		ChannelID: "somestreamer", Target: 100, Title: "Followers",
	}}, goals)
	db.assertIssued(t, "gc.metric IN ('followers','subscribers')", nil)
	q := db.queries[0].sql
	assert.Contains(t, q, "o.overlay_type = 'goal'")
	assert.Contains(t, q, "o.is_active")
	assert.Contains(t, q, "ocs.platform = gc.platform")
	assert.Contains(t, q, "ocs.is_active")
}
