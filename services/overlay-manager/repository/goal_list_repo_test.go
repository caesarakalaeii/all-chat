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

// Migrations 110/111 applied twice (the runner re-applies every migration on
// pod start), then goal and list configs round-trip through their upserts.
func TestGoalListRepository_RoundTripOnRealMigrations(t *testing.T) {
	pool, cleanup := setupEventSettingsTestDB(t)
	defer cleanup()
	migrations := loadUpMigrations(t)
	runMigrations(t, pool, migrations)
	runMigrations(t, pool, migrations)

	overlayID := seedOverlayWithEventSettings(t, pool, "goal_list_canary")
	repo := NewGoalListRepository(pool)
	ctx := context.Background()

	goal, err := repo.GetGoalConfig(ctx, overlayID)
	require.NoError(t, err)
	assert.Nil(t, goal)
	want := alerts.GoalConfig{Metric: "bits", Platform: "twitch", Target: 1000, StartingValue: 50, Title: "Bits"}
	require.NoError(t, repo.UpsertGoalConfig(ctx, overlayID, want))
	want.Target = 2000
	require.NoError(t, repo.UpsertGoalConfig(ctx, overlayID, want))
	goal, err = repo.GetGoalConfig(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, &want, goal)

	list, err := repo.GetListConfig(ctx, overlayID)
	require.NoError(t, err)
	assert.Nil(t, list)
	wantList := alerts.ListConfig{Category: "gifts", Limit: 5, Title: "Top"}
	require.NoError(t, repo.UpsertListConfig(ctx, overlayID, wantList))
	list, err = repo.GetListConfig(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, &wantList, list)
}
