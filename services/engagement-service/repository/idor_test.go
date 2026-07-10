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

// Regression tests for the PR #524 review fixes B1 (cross-tenant IDOR on the
// prediction/poll lifecycle) and H1 (cross-economy wager: debit the path overlay
// but pay the prediction's real overlay). They exercise the real overlay_id SQL
// predicates, so they need a Postgres with the engagement migrations (068–074)
// applied. Run with: go test -tags=integration ./repository/...
package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := "postgres://allchat:allchat_dev_password@localhost:5432/allchat"
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("requires DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("requires DB: %v", err)
	}
	// The engagement tables must be present (engagement migrations 068–074).
	var ok bool
	if err := pool.QueryRow(context.Background(),
		`SELECT to_regclass('public.predictions') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		pool.Close()
		t.Skipf("requires engagement migrations (068–074): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedOverlay creates a throwaway user + overlay and returns the overlay id. The
// user (and its overlays, predictions, polls via cascade) are removed on cleanup.
func seedOverlay(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]
	var userID, overlayID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
		 VALUES ($1, $2, $3, 'x', 'x', NOW() + INTERVAL '1 day') RETURNING id`,
		"idor-"+suffix, "idor-"+suffix, "IDOR "+suffix).Scan(&userID)
	require.NoError(t, err)
	err = pool.QueryRow(ctx,
		`INSERT INTO overlays (user_id, name) VALUES ($1, $2) RETURNING id`,
		userID, "idor-overlay-"+suffix).Scan(&overlayID)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })
	return overlayID
}

// seedViewer creates a throwaway viewer, removed on cleanup.
func seedViewer(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var vid uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO viewers DEFAULT VALUES RETURNING id`).Scan(&vid))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM viewers WHERE id = $1`, vid) })
	return vid
}

// TestB1_CrossTenantPredictionLifecycle: an owner of overlay B cannot lock/resolve/
// cancel a prediction that lives on overlay A. Each cross-tenant call must return
// ErrNotFound and leave the prediction unchanged; the legitimate owner still can.
func TestB1_CrossTenantPredictionLifecycle(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlayB := seedOverlay(t, pool)

	// Each subtest seeds its OWN overlayA: uniq_active_pred_per_overlay allows only one
	// live (ACTIVE|LOCKED) All-Chat prediction per overlay, so a prediction left live by
	// one subtest would otherwise block the next subtest's CreatePrediction on a shared
	// overlay. overlayB (the attacker id) is stateless and safely shared.
	t.Run("lock", func(t *testing.T) {
		overlayA := seedOverlay(t, pool)
		pred, err := repo.CreatePrediction(ctx, overlayA, "who wins?", []string{"a", "b"}, nil)
		require.NoError(t, err)

		_, err = repo.LockPrediction(ctx, pred.ID, overlayB)
		assert.ErrorIs(t, err, repository.ErrNotFound, "attacker (overlay B) must not lock overlay A's prediction")
		got, err := repo.GetPrediction(ctx, pred.ID)
		require.NoError(t, err)
		assert.Equal(t, models.PredActive, got.State, "prediction must stay ACTIVE after the cross-tenant lock")

		owned, err := repo.LockPrediction(ctx, pred.ID, overlayA)
		require.NoError(t, err)
		assert.Equal(t, models.PredLocked, owned.State, "the real owner can still lock")
	})

	t.Run("resolve", func(t *testing.T) {
		overlayA := seedOverlay(t, pool)
		pred, err := repo.CreatePrediction(ctx, overlayA, "who wins?", []string{"a", "b"}, nil)
		require.NoError(t, err)
		_, err = repo.LockPrediction(ctx, pred.ID, overlayA)
		require.NoError(t, err)
		winning := pred.Outcomes[0].ID

		_, err = repo.ResolvePrediction(ctx, pred.ID, winning, overlayB)
		assert.ErrorIs(t, err, repository.ErrNotFound, "attacker must not resolve overlay A's prediction")
		got, err := repo.GetPrediction(ctx, pred.ID)
		require.NoError(t, err)
		assert.Equal(t, models.PredLocked, got.State, "prediction must stay LOCKED after the cross-tenant resolve")

		owned, err := repo.ResolvePrediction(ctx, pred.ID, winning, overlayA)
		require.NoError(t, err)
		assert.Equal(t, models.PredResolved, owned.State)
	})

	t.Run("cancel", func(t *testing.T) {
		overlayA := seedOverlay(t, pool)
		pred, err := repo.CreatePrediction(ctx, overlayA, "who wins?", []string{"a", "b"}, nil)
		require.NoError(t, err)

		_, err = repo.CancelPrediction(ctx, pred.ID, overlayB)
		assert.ErrorIs(t, err, repository.ErrNotFound, "attacker must not cancel overlay A's prediction")
		got, err := repo.GetPrediction(ctx, pred.ID)
		require.NoError(t, err)
		assert.Equal(t, models.PredActive, got.State)

		owned, err := repo.CancelPrediction(ctx, pred.ID, overlayA)
		require.NoError(t, err)
		assert.Equal(t, models.PredCanceled, owned.State)
	})
}

// TestB1_CrossTenantClosePoll: an owner of overlay B cannot close overlay A's poll.
func TestB1_CrossTenantClosePoll(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlayA := seedOverlay(t, pool)
	overlayB := seedOverlay(t, pool)

	poll, err := repo.CreatePoll(ctx, overlayA, "pizza?", []string{"yes", "no"}, true, nil)
	require.NoError(t, err)

	_, err = repo.ClosePoll(ctx, poll.ID, overlayB)
	assert.ErrorIs(t, err, repository.ErrNotFound, "attacker (overlay B) must not close overlay A's poll")
	got, err := repo.GetPoll(ctx, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PollActive, got.State, "poll must stay ACTIVE after the cross-tenant close")

	owned, err := repo.ClosePoll(ctx, poll.ID, overlayA)
	require.NoError(t, err)
	assert.Equal(t, models.PollClosed, owned.State, "the real owner can still close")
}

// TestH1_WagerBindsDebitToPredictionOverlay: a wager naming overlay B in the path
// while betting on a prediction that lives on overlay A must be rejected — the
// debit economy (path) and the payout economy (prediction row) must never diverge.
func TestH1_WagerBindsDebitToPredictionOverlay(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlayA := seedOverlay(t, pool)
	overlayB := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)

	// Fund the viewer in BOTH economies so a mis-bound debit would visibly move points.
	_, err := repo.AwardPoints(ctx, viewer, overlayA, 1000, "seed", "test", nil, "seed:a:"+viewer.String())
	require.NoError(t, err)
	_, err = repo.AwardPoints(ctx, viewer, overlayB, 1000, "seed", "test", nil, "seed:b:"+viewer.String())
	require.NoError(t, err)

	pred, err := repo.CreatePrediction(ctx, overlayA, "who wins?", []string{"a", "b"}, nil)
	require.NoError(t, err)

	// Attack: wager on overlay A's prediction while naming overlay B in the path.
	res, err := repo.Wager(ctx, pred.ID, viewer, overlayB, 1, 100, "twitch", nil, "")
	require.NoError(t, err)
	assert.False(t, res.Accepted, "a wager whose path overlay does not own the prediction must be rejected")
	assert.Equal(t, "not_found", res.Reason)

	balA, _ := repo.GetBalance(ctx, viewer, overlayA)
	balB, _ := repo.GetBalance(ctx, viewer, overlayB)
	assert.EqualValues(t, 1000, balA, "no debit should land in the prediction's real economy")
	assert.EqualValues(t, 1000, balB, "no debit should land in the named (wrong) economy")

	// Positive control: the correctly-scoped wager debits the prediction's own economy.
	res, err = repo.Wager(ctx, pred.ID, viewer, overlayA, 1, 100, "twitch", nil, "")
	require.NoError(t, err)
	require.True(t, res.Accepted, "the correctly-scoped wager should be accepted; reason=%q", res.Reason)
	balA, _ = repo.GetBalance(ctx, viewer, overlayA)
	assert.EqualValues(t, 900, balA, "the stake is debited from the prediction's economy (overlay A)")
}
