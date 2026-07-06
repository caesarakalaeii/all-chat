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

// Regression tests for the PR #524 round-5 fixes: G3 (poll vote overlay binding),
// M3 (owner close must not flip a native mirror poll), M1 (native staleness sweep),
// M7b (concurrent viewer create leaves no orphan rows), U1 (display grace window).
// Reuses the newTestDB/seedOverlay/seedViewer helpers in idor_test.go. Needs a
// Postgres with the engagement migrations applied. Run: go test -tags=integration ./repository/...
package repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestG3_CrossTenantPollVote: a vote naming overlay B in the path while targeting a
// poll owned by overlay A must be rejected (not accepted, no row recorded).
func TestG3_CrossTenantPollVote(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlayA := seedOverlay(t, pool)
	overlayB := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)

	poll, err := repo.CreatePoll(ctx, overlayA, "pizza?", []string{"yes", "no"}, true, nil)
	require.NoError(t, err)

	// Attack: vote on overlay A's poll while naming overlay B.
	accepted, err := repo.RecordVote(ctx, poll.ID, viewer, overlayB, 1, "twitch", nil)
	require.NoError(t, err)
	assert.False(t, accepted, "a vote whose path overlay does not own the poll must be rejected")
	n, err := repo.TotalVotes(ctx, poll.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n, "no vote recorded for the cross-tenant attempt")

	// Positive control: the correctly-scoped vote is accepted and counted.
	accepted, err = repo.RecordVote(ctx, poll.ID, viewer, overlayA, 1, "twitch", nil)
	require.NoError(t, err)
	require.True(t, accepted, "the correctly-scoped vote should be accepted")
	n, err = repo.TotalVotes(ctx, poll.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
}

// TestM3_CloseDoesNotFlipNativeMirrorPoll: an owner close on a mirrored twitch_native
// poll id must be a no-op (source guard), and the mirror must stay updatable.
func TestM3_CloseDoesNotFlipNativeMirrorPoll(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)

	mirror, err := repo.UpsertNativePoll(ctx, overlay, "ext-m3", "pineapple?", models.PollActive,
		[]repository.NativeOutcomeInput{{ExternalID: "c1", Idx: 1, Label: "yes", Votes: 5}, {ExternalID: "c2", Idx: 2, Label: "no", Votes: 3}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mirror)
	require.Equal(t, models.SourceTwitchNative, mirror.Source)
	require.Equal(t, models.PollActive, mirror.State)

	got, err := repo.ClosePoll(ctx, mirror.ID, overlay)
	require.NoError(t, err)
	assert.Equal(t, models.PollActive, got.State, "closing a native mirror poll must not flip it to CLOSED")
	reread, err := repo.GetPoll(ctx, mirror.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PollActive, reread.State)

	// Regression: later Twitch tally events must still be accepted (mirror not frozen
	// by a premature CLOSED via UpsertNativePoll's monotonic guard).
	updated, err := repo.UpsertNativePoll(ctx, overlay, "ext-m3", "pineapple?", models.PollActive,
		[]repository.NativeOutcomeInput{{ExternalID: "c1", Idx: 1, Label: "yes", Votes: 9}, {ExternalID: "c2", Idx: 2, Label: "no", Votes: 4}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated, "later native events must still be accepted (mirror not frozen)")
}

// TestM1_ForceCloseStaleNativePolls: a native poll stuck ACTIVE past the TTL is
// force-closed; a fresh native poll and an all-chat poll are left alone.
func TestM1_ForceCloseStaleNativePolls(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	overlay2 := seedOverlay(t, pool)

	stale, err := repo.UpsertNativePoll(ctx, overlay, "ext-stale", "q?", models.PollActive,
		[]repository.NativeOutcomeInput{{ExternalID: "c1", Idx: 1, Label: "a", Votes: 1}, {ExternalID: "c2", Idx: 2, Label: "b", Votes: 1}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, stale)
	_, err = pool.Exec(ctx, `UPDATE polls SET created_at = NOW() - INTERVAL '5 hours' WHERE id=$1`, stale.ID)
	require.NoError(t, err)

	fresh, err := repo.UpsertNativePoll(ctx, overlay2, "ext-fresh", "q2?", models.PollActive,
		[]repository.NativeOutcomeInput{{ExternalID: "d1", Idx: 1, Label: "a", Votes: 0}, {ExternalID: "d2", Idx: 2, Label: "b", Votes: 0}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, fresh)

	// An old all-chat poll must NOT be swept (native sweep is source-scoped).
	acPoll, err := repo.CreatePoll(ctx, overlay2, "ac?", []string{"y", "n"}, true, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE polls SET created_at = NOW() - INTERVAL '9 hours' WHERE id=$1`, acPoll.ID)
	require.NoError(t, err)

	refs, err := repo.ForceCloseStaleNativePolls(ctx, 4*time.Hour)
	require.NoError(t, err)
	closed := map[uuid.UUID]bool{}
	for _, r := range refs {
		closed[r.PollID] = true
	}
	assert.True(t, closed[stale.ID], "the stranded native poll must be force-closed")
	assert.False(t, closed[fresh.ID], "a fresh native poll must not be swept")
	assert.False(t, closed[acPoll.ID], "an all-chat poll must not be swept by the native sweep")

	live, err := repo.HasLiveNativePoll(ctx, overlay)
	require.NoError(t, err)
	assert.False(t, live, "the stranded overlay no longer 409-blocks all-chat polls")
	liveFresh, err := repo.HasLiveNativePoll(ctx, overlay2)
	require.NoError(t, err)
	assert.True(t, liveFresh, "the fresh native poll still blocks (genuinely live)")
	acGot, err := repo.GetPoll(ctx, acPoll.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PollActive, acGot.State, "the all-chat poll is untouched")
}

// TestM1_ForceCloseStaleNativePredictions: a native prediction stuck LOCKED past the
// TTL is CANCELED (no known winner); a fresh one is left alone.
func TestM1_ForceCloseStaleNativePredictions(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	lockedAt := time.Now()

	pred, err := repo.UpsertNativePrediction(ctx, overlay, "extp-stale", "t?", models.PredLocked, "",
		[]repository.NativeOutcomeInput{{ExternalID: "o1", Idx: 1, Label: "a", Points: 10, Entrants: 2}, {ExternalID: "o2", Idx: 2, Label: "b", Points: 5, Entrants: 1}}, nil, &lockedAt, nil)
	require.NoError(t, err)
	require.NotNil(t, pred)
	_, err = pool.Exec(ctx, `UPDATE predictions SET created_at = NOW() - INTERVAL '5 hours' WHERE id=$1`, pred.ID)
	require.NoError(t, err)

	refs, err := repo.ForceCloseStaleNativePredictions(ctx, 4*time.Hour)
	require.NoError(t, err)
	found := false
	for _, r := range refs {
		if r.PredictionID == pred.ID {
			found = true
		}
	}
	assert.True(t, found, "the stranded native prediction must be force-canceled")
	got, err := repo.GetPrediction(ctx, pred.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PredCanceled, got.State, "a stale native prediction becomes CANCELED (no known winner)")
	live, err := repo.HasLiveNativePrediction(ctx, overlay)
	require.NoError(t, err)
	assert.False(t, live)
}

// TestM7b_ConcurrentViewerCreateNoOrphan: many concurrent GetOrCreateViewerByPlatform
// for the same identity resolve one viewer with no orphaned viewers rows.
func TestM7b_ConcurrentViewerCreateNoOrphan(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	platform := "twitch"
	platformUserID := "m7b-" + uuid.NewString()[:12]

	orphanQuery := `SELECT count(*) FROM viewers v WHERE NOT EXISTS (SELECT 1 FROM viewer_platform_identities i WHERE i.viewer_id = v.id)`
	var orphansBefore int64
	require.NoError(t, pool.QueryRow(ctx, orphanQuery).Scan(&orphansBefore))

	const n = 20
	ids := make([]uuid.UUID, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ids[i], errs[i] = repo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		require.NoErrorf(t, e, "goroutine %d", i)
	}
	first := ids[0]
	require.NotEqual(t, uuid.Nil, first)
	for _, id := range ids {
		assert.Equal(t, first, id, "all concurrent callers must resolve the same viewer")
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM viewers WHERE id = $1`, first) })

	var identCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM viewer_platform_identities WHERE platform=$1 AND platform_user_id=$2`,
		platform, platformUserID).Scan(&identCount))
	assert.EqualValues(t, 1, identCount, "exactly one identity row")

	var orphansAfter int64
	require.NoError(t, pool.QueryRow(ctx, orphanQuery).Scan(&orphansAfter))
	assert.EqualValues(t, orphansBefore, orphansAfter, "no orphan viewers rows created under contention")
}

// TestU1_DisplayPollServedDuringClosedGrace: a just-closed poll is still served by
// GetActiveDisplayPoll for the grace window, then clears.
func TestU1_DisplayPollServedDuringClosedGrace(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)

	poll, err := repo.CreatePoll(ctx, overlay, "grace?", []string{"y", "n"}, true, nil)
	require.NoError(t, err)
	_, err = repo.ClosePoll(ctx, poll.ID, overlay)
	require.NoError(t, err)

	disp, err := repo.GetActiveDisplayPoll(ctx, overlay)
	require.NoError(t, err)
	assert.Equal(t, models.PollClosed, disp.State, "a just-closed poll is served during the grace window (final reveal)")

	_, err = pool.Exec(ctx, `UPDATE polls SET closed_at = NOW() - INTERVAL '1 hour' WHERE id=$1`, poll.ID)
	require.NoError(t, err)
	_, err = repo.GetActiveDisplayPoll(ctx, overlay)
	assert.ErrorIs(t, err, repository.ErrNotFound, "past the grace window the poll clears")
}

// TestU1_DisplayPredictionServedDuringResolvedGrace: a just-resolved prediction is
// served for the reveal; a live round outranks it; past the window it clears.
func TestU1_DisplayPredictionServedDuringResolvedGrace(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)
	_, err := repo.AwardPoints(ctx, viewer, overlay, 1000, "seed", "test", nil, "u1seed:"+viewer.String())
	require.NoError(t, err)

	pred, err := repo.CreatePrediction(ctx, overlay, "who?", []string{"a", "b"}, nil)
	require.NoError(t, err)
	res, err := repo.Wager(ctx, pred.ID, viewer, overlay, 1, 100, "twitch", nil)
	require.NoError(t, err)
	require.True(t, res.Accepted, "reason=%q", res.Reason)
	_, err = repo.LockPrediction(ctx, pred.ID, overlay)
	require.NoError(t, err)
	_, err = repo.ResolvePrediction(ctx, pred.ID, pred.Outcomes[0].ID, overlay)
	require.NoError(t, err)

	disp, err := repo.GetActiveDisplayPrediction(ctx, overlay)
	require.NoError(t, err)
	assert.Equal(t, models.PredResolved, disp.State, "a just-resolved prediction is served during the grace window")
	require.NotNil(t, disp.WinningOutcomeID)

	// A fresh live round on the same overlay outranks the resolved grace-window one.
	pred2, err := repo.CreatePrediction(ctx, overlay, "next?", []string{"x", "y"}, nil)
	require.NoError(t, err)
	disp2, err := repo.GetActiveDisplayPrediction(ctx, overlay)
	require.NoError(t, err)
	assert.Equal(t, pred2.ID, disp2.ID, "a live round outranks a resolved grace-window round")
	assert.Equal(t, models.PredActive, disp2.State)

	// Remove the live round and age both terminal rows past the window → cleared.
	_, err = repo.CancelPrediction(ctx, pred2.ID, overlay)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE predictions SET resolved_at = NOW() - INTERVAL '1 hour' WHERE overlay_id=$1`, overlay)
	require.NoError(t, err)
	_, err = repo.GetActiveDisplayPrediction(ctx, overlay)
	assert.ErrorIs(t, err, repository.ErrNotFound, "past the grace window the round clears")
}
