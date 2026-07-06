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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLC1_TerminalNativePredictionIsAbsorbing: once a mirrored Twitch prediction is
// RESOLVED, a redelivered/out-of-order CANCELED event (or vice versa) must NOT flip
// the terminal outcome — the monotonic guard treats terminal states as absorbing.
func TestLC1_TerminalNativePredictionIsAbsorbing(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlay := seedOverlay(t, pool)
	now := time.Now()
	outcomes := []repository.NativeOutcomeInput{
		{ExternalID: "o1", Idx: 1, Label: "A"},
		{ExternalID: "o2", Idx: 2, Label: "B"},
	}

	resolved, err := repo.UpsertNativePrediction(ctx, overlay, "ext-lc1", "who wins?", models.PredResolved, "o1", outcomes, nil, nil, &now)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, models.PredResolved, resolved.State)

	// A later CANCELED for the same external round must be blocked (returns nil).
	flipped, err := repo.UpsertNativePrediction(ctx, overlay, "ext-lc1", "who wins?", models.PredCanceled, "", outcomes, nil, nil, &now)
	require.NoError(t, err)
	assert.Nil(t, flipped, "a terminal→different-terminal transition must be blocked as a stale event")

	got, err := repo.GetPrediction(ctx, resolved.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PredResolved, got.State, "the round must stay RESOLVED after the stale CANCELED")
}
