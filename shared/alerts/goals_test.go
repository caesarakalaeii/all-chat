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

package alerts

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoalConfigValidate(t *testing.T) {
	ok := GoalConfig{Metric: "followers", Platform: "twitch", Target: 100}
	require.NoError(t, ok.Validate())

	for name, cfg := range map[string]GoalConfig{
		"unknown metric":     {Metric: "likes", Platform: "twitch", Target: 10},
		"wrong platform":     {Metric: "followers", Platform: "youtube", Target: 10},
		"zero target":        {Metric: "bits", Platform: "twitch", Target: 0},
		"negative start":     {Metric: "bits", Platform: "twitch", Target: 10, StartingValue: -1},
		"title too long":     {Metric: "super_chat", Platform: "youtube", Target: 10, Title: string(make([]byte, 201))},
		"subscribers twitch": {Metric: "subscribers", Platform: "twitch", Target: 10},
	} {
		assert.Error(t, cfg.Validate(), name)
	}
	assert.True(t, IsPolledGoalMetric("followers"))
	assert.True(t, IsPolledGoalMetric("subscribers"))
	assert.False(t, IsPolledGoalMetric("bits"))
}

func TestListConfigValidate(t *testing.T) {
	require.NoError(t, ListConfig{Category: "gifts", Limit: 10}.Validate())
	assert.Error(t, ListConfig{Category: "follows", Limit: 10}.Validate())
	assert.Error(t, ListConfig{Category: "bits", Limit: 0}.Validate())
	assert.Error(t, ListConfig{Category: "bits", Limit: MaxListLimit + 1}.Validate())
}

func TestTopEntriesRanksByScoreWithNames(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	key := LeaderboardKey("ov", "bits")
	rdb.ZAdd(ctx, key, redis.Z{Score: 50, Member: "twitch:1"}, redis.Z{Score: 500, Member: "twitch:2"}, redis.Z{Score: 5, Member: "twitch:3"})
	rdb.HSet(ctx, LeaderboardNamesKey("ov", "bits"), "twitch:2", "Big", "twitch:1", "Mid")

	entries, err := TopEntries(ctx, rdb, "ov", "bits", 2)
	require.NoError(t, err)
	assert.Equal(t, []LeaderboardEntry{
		{Rank: 1, Name: "Big", Platform: "twitch", Amount: 500},
		{Rank: 2, Name: "Mid", Platform: "twitch", Amount: 50},
	}, entries)

	// A member without a stored name falls back to its user id.
	entries, err = TopEntries(ctx, rdb, "ov", "bits", 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "3", entries[2].Name)

	empty, err := TopEntries(ctx, rdb, "ov", "gifts", 10)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.NotNil(t, empty, "an empty board serializes as [] not null")
}
