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

	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestP2_1_RoundIndependentWagerDedup: a redelivered chat wager (same Redis stream entry
// id) must never place a fresh bet — or debit again — on a NEW round that opened after the
// original round resolved. The round-independent (overlay, message) ledger dedup key
// catches it even though GetActivePrediction now resolves to a different prediction id.
func TestP2_1_RoundIndependentWagerDedup(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlay := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)
	_, err := repo.AwardPoints(ctx, viewer, overlay, 1000, "seed", "test", nil, "seed:"+uuid.NewString())
	require.NoError(t, err)

	// The stable per-message replay token (a Redis stream entry id survives redelivery).
	const token = "1700000000000-0"

	// Round 1: the chat message wagers 100 on the (only) winning outcome.
	r1, err := repo.CreatePrediction(ctx, overlay, "round 1", []string{"a", "b"}, nil)
	require.NoError(t, err)
	res, err := repo.Wager(ctx, r1.ID, viewer, overlay, 1, 100, "twitch", nil, token)
	require.NoError(t, err)
	require.True(t, res.Accepted)
	bal, err := repo.GetBalance(ctx, viewer, overlay)
	require.NoError(t, err)
	require.EqualValues(t, 900, bal, "the first wager debits 100")

	// R1 resolves (sole entrant on the winning outcome → refunded), then a NEW round opens.
	_, err = repo.LockPrediction(ctx, r1.ID, overlay)
	require.NoError(t, err)
	_, err = repo.ResolvePrediction(ctx, r1.ID, r1.Outcomes[0].ID, overlay)
	require.NoError(t, err)
	balAfterR1, err := repo.GetBalance(ctx, viewer, overlay)
	require.NoError(t, err)

	r2, err := repo.CreatePrediction(ctx, overlay, "round 2", []string{"a", "b"}, nil)
	require.NoError(t, err)

	// The SAME chat message is redelivered and now resolves to R2.
	res2, err := repo.Wager(ctx, r2.ID, viewer, overlay, 1, 100, "twitch", nil, token)
	require.NoError(t, err)
	assert.False(t, res2.Accepted, "a redelivered wager must not place a fresh bet on the new round")
	assert.Equal(t, "duplicate", res2.Reason)

	balAfterReplay, err := repo.GetBalance(ctx, viewer, overlay)
	require.NoError(t, err)
	assert.Equal(t, balAfterR1, balAfterReplay, "the redelivered wager must not debit the new round")

	oid, amt, err := repo.GetViewerEntry(ctx, r2.ID, viewer)
	require.NoError(t, err)
	assert.Nil(t, oid, "no phantom entry left on the new round")
	assert.Zero(t, amt)
}
