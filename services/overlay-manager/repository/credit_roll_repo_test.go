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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const creditRollTogglesMigration = "105_credit_roll_include_toggles.sql"

// Migration 105 starts enforcing include_channel_points, which used to default to
// FALSE while the board was always shown. Existing rows must come out of it with
// the board still on and the new opt-in boards off, and because the runner
// re-applies every migration on each pod start, a streamer who later turns the
// board off must stay off across a re-run.
func TestCreditRollTogglesMigrationKeepsChannelPointsBoardOnce(t *testing.T) {
	pool, cleanup := setupEventSettingsTestDB(t)
	defer cleanup()
	ctx := context.Background()

	all := loadUpMigrations(t)
	before, from := splitAtMigrations(t, all, creditRollTogglesMigration)
	runMigrations(t, pool, before)

	overlayID := seedOverlayWithEventSettings(t, pool, "credit_roll_toggles_canary")
	_, err := pool.Exec(ctx,
		`INSERT INTO credit_roll_configs (overlay_id) VALUES ($1) ON CONFLICT (overlay_id) DO NOTHING`, overlayID)
	require.NoError(t, err)

	runMigrations(t, pool, from)

	repo := NewCreditRollRepository(pool)
	config, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	assert.True(t, config.IncludeChannelPoints, "the always-shown Top Channel Points board must survive enforcement")
	assert.False(t, config.IncludeTreasureChests)
	assert.False(t, config.IncludeShares)
	assert.False(t, config.IncludeLikes)

	config.IncludeChannelPoints = false
	require.NoError(t, repo.Update(ctx, config))

	runMigrations(t, pool, all)

	reloaded, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	assert.False(t, reloaded.IncludeChannelPoints, "a pod restart re-ran migration 105 and overrode the streamer's choice")
}
