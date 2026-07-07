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

//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nativePredStateSweep(t *testing.T, pool *pgxpool.Pool, overlay uuid.UUID, ext string) (state string, sweep bool) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT state, sweep_canceled FROM predictions
		 WHERE overlay_id = $1 AND source = 'twitch_native' AND external_id = $2`,
		overlay, ext).Scan(&state, &sweep))
	return state, sweep
}

func nativeOutcomes() []repository.NativeOutcomeInput {
	return []repository.NativeOutcomeInput{
		{ExternalID: "oc-a", Idx: 1, Label: "A"},
		{ExternalID: "oc-b", Idx: 2, Label: "B"},
	}
}

// TestP2_4_SweepCancelOverriddenByGenuineResolve: a LOCKED Twitch prediction has no
// forced-resolution deadline, so the 4h stale-sweep can cancel it before the real
// channel.prediction.end arrives. The synthetic (sweep) cancel must NOT permanently
// absorb the genuine RESOLVED — the real winner has to display (P2-4).
func TestP2_4_SweepCancelOverriddenByGenuineResolve(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	ext := "twitch-pred-" + uuid.NewString()[:8]
	now := time.Now()

	_, err := repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "ACTIVE", "", nativeOutcomes(), nil, nil, nil)
	require.NoError(t, err)
	locked := now
	_, err = repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "LOCKED", "", nativeOutcomes(), nil, &locked, nil)
	require.NoError(t, err)

	// Stale-sweep force-cancels it (ttl=0 → anything created up to now).
	refs, err := repo.ForceCloseStaleNativePredictions(ctx, 0)
	require.NoError(t, err)
	require.NotEmpty(t, refs)
	state, sweep := nativePredStateSweep(t, pool, overlay, ext)
	require.Equal(t, "CANCELED", state)
	require.True(t, sweep, "the sweep cancel must be tagged synthetic")

	// The genuine RESOLVED arrives late and MUST override the synthetic cancel.
	resolvedAt := now.Add(time.Minute)
	pred, err := repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "RESOLVED", "oc-b", nativeOutcomes(), nil, nil, &resolvedAt)
	require.NoError(t, err)
	require.NotNil(t, pred, "the genuine terminal must NOT be absorbed by the synthetic cancel")
	assert.Equal(t, models.PredResolved, pred.State)
	require.NotNil(t, pred.WinningOutcomeID, "the real winner is recorded")

	state, sweep = nativePredStateSweep(t, pool, overlay, ext)
	assert.Equal(t, "RESOLVED", state)
	assert.False(t, sweep, "overriding a synthetic cancel clears the flag → now authoritative")
}

// TestP2_4_GenuineCancelNotOverridden proves the L-C1 guard still holds for REAL
// terminals: a genuine Twitch CANCELED must not be laterally flipped to RESOLVED.
func TestP2_4_GenuineCancelNotOverridden(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	ext := "twitch-pred-" + uuid.NewString()[:8]
	now := time.Now()

	_, err := repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "ACTIVE", "", nativeOutcomes(), nil, nil, nil)
	require.NoError(t, err)
	canceledAt := now
	_, err = repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "CANCELED", "", nativeOutcomes(), nil, nil, &canceledAt)
	require.NoError(t, err)
	state, sweep := nativePredStateSweep(t, pool, overlay, ext)
	require.Equal(t, "CANCELED", state)
	require.False(t, sweep, "a genuine (non-sweep) cancel is authoritative")

	// A later RESOLVED must be BLOCKED (absorbing guard) → upsert is a no-op (nil, nil).
	resolvedAt := now.Add(time.Minute)
	pred, err := repo.UpsertNativePrediction(ctx, overlay, ext, "who wins", "RESOLVED", "oc-b", nativeOutcomes(), nil, nil, &resolvedAt)
	require.NoError(t, err)
	assert.Nil(t, pred, "a genuine RESOLVED must NOT override a genuine CANCELED (L-C1)")
	state, _ = nativePredStateSweep(t, pool, overlay, ext)
	assert.Equal(t, "CANCELED", state)
}
