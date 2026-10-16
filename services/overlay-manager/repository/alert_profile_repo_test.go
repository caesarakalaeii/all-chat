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

// Runs the real migration set twice (the runner re-applies every migration on
// each pod start, so 107 must be idempotent), then round-trips profiles.
func TestAlertProfileRepository_UpsertRoundTripOnRealMigrations(t *testing.T) {
	pool, cleanup := setupEventSettingsTestDB(t)
	defer cleanup()
	migrations := loadUpMigrations(t)
	runMigrations(t, pool, migrations)
	runMigrations(t, pool, migrations)

	overlayID := seedOverlayWithEventSettings(t, pool, "alert_profiles_canary")
	repo := NewAlertProfileRepository(pool)
	ctx := context.Background()

	got, err := repo.ListAlertProfiles(ctx, overlayID)
	require.NoError(t, err)
	assert.Empty(t, got, "a fresh overlay has no rows: it runs on the Go defaults")

	bits, _ := alerts.DefaultProfile("bits")
	bits.SoundURL = "https://media.allch.at/u/ding.mp3"
	raid, _ := alerts.DefaultProfile("raid")
	require.NoError(t, repo.UpsertAlertProfiles(ctx, overlayID, []alerts.Profile{bits, raid}))

	bits.Volume = 30
	bits.Enabled = false
	require.NoError(t, repo.UpsertAlertProfiles(ctx, overlayID, []alerts.Profile{bits}))

	got, err = repo.ListAlertProfiles(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, []alerts.Profile{bits, raid}, got, "second upsert updates bits in place and leaves raid alone")

	var gate bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_premium FROM feature_gates WHERE feature_key = 'alerts_editor'`).Scan(&gate))
	assert.True(t, gate)
}
