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

// Integration coverage for payout/refund idempotency + whole-economy conservation (P3-8).
// The pure engine is unit-tested in engine/payout_test.go; these drive the real repository
// transactions and prove that a DOUBLE resolve/cancel never pays winners (or refunds) twice
// — the guarded state transition plus the per-viewer payout:/refund: ledger dedup keys.
package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedFunded(t *testing.T, repo *repository.Repository, viewer, overlay uuid.UUID, amount int64) {
	t.Helper()
	_, err := repo.AwardPoints(context.Background(), viewer, overlay, amount, "seed", "test", nil, "seed:"+uuid.NewString())
	require.NoError(t, err)
}

func TestP3_8_ResolveIdempotentAndConserved(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	v1, v2 := seedViewer(t, pool), seedViewer(t, pool)
	seedFunded(t, repo, v1, overlay, 1000)
	seedFunded(t, repo, v2, overlay, 1000)

	pred, err := repo.CreatePrediction(ctx, overlay, "who wins", []string{"a", "b"}, nil)
	require.NoError(t, err)
	// v1 wagers 100 on the winning outcome, v2 wagers 40 on the losing one.
	r, err := repo.Wager(ctx, pred.ID, v1, overlay, 1, 100, "web", nil, "")
	require.NoError(t, err)
	require.True(t, r.Accepted)
	r, err = repo.Wager(ctx, pred.ID, v2, overlay, 2, 40, "web", nil, "")
	require.NoError(t, err)
	require.True(t, r.Accepted)

	_, err = repo.LockPrediction(ctx, pred.ID, overlay)
	require.NoError(t, err)
	_, err = repo.ResolvePrediction(ctx, pred.ID, pred.Outcomes[0].ID, overlay)
	require.NoError(t, err)

	b1, _ := repo.GetBalance(ctx, v1, overlay)
	b2, _ := repo.GetBalance(ctx, v2, overlay)
	assert.EqualValues(t, 1040, b1, "winner: 900 remaining + stake 100 + losers' pool 40")
	assert.EqualValues(t, 960, b2, "loser keeps their 960")
	assert.EqualValues(t, 2000, b1+b2, "whole economy conserved (started 2000)")

	// Double-resolve must be a no-op — the guarded LOCKED->RESOLVED transition affects 0
	// rows and the payout: dedup keys backstop it. Winners are NOT paid twice.
	_, err = repo.ResolvePrediction(ctx, pred.ID, pred.Outcomes[0].ID, overlay)
	require.NoError(t, err)
	b1b, _ := repo.GetBalance(ctx, v1, overlay)
	b2b, _ := repo.GetBalance(ctx, v2, overlay)
	assert.EqualValues(t, 1040, b1b, "double-resolve must not pay the winner twice")
	assert.EqualValues(t, 960, b2b)
}

func TestP3_8_CancelIdempotentRefund(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	v1 := seedViewer(t, pool)
	seedFunded(t, repo, v1, overlay, 1000)

	pred, err := repo.CreatePrediction(ctx, overlay, "who wins", []string{"a", "b"}, nil)
	require.NoError(t, err)
	r, err := repo.Wager(ctx, pred.ID, v1, overlay, 1, 100, "web", nil, "")
	require.NoError(t, err)
	require.True(t, r.Accepted)
	bal, _ := repo.GetBalance(ctx, v1, overlay)
	require.EqualValues(t, 900, bal)

	_, err = repo.CancelPrediction(ctx, pred.ID, overlay)
	require.NoError(t, err)
	bal, _ = repo.GetBalance(ctx, v1, overlay)
	assert.EqualValues(t, 1000, bal, "cancel refunds the full stake")

	// Double-cancel must be a no-op — no second refund.
	_, err = repo.CancelPrediction(ctx, pred.ID, overlay)
	require.NoError(t, err)
	bal, _ = repo.GetBalance(ctx, v1, overlay)
	assert.EqualValues(t, 1000, bal, "double-cancel must not refund twice")
}
