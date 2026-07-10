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

// Regression tests for the PR #524 round-7 fixes G3 (an All-Chat vote must never
// land on a mirrored twitch_native poll) and H1 (an All-Chat wager must never land
// on a mirrored twitch_native prediction). Twitch owns the votes/wagers on a native
// round, so accepting one here would corrupt the mirrored tally / debit points into a
// round the payout path never touches. Both are silent rejections on the source guard.
// Reuses the newTestDB/seedOverlay/seedViewer helpers in idor_test.go. Needs a
// Postgres with the engagement migrations applied. Run: go test -tags=integration ./repository/...
package repository_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestG3_VoteRejectedOnNativePoll: RecordVote against a mirrored twitch_native poll
// must be a silent no-op (accepted=false, no error) and record no poll_votes row —
// the tally is Twitch-owned (mirror_votes), so an All-Chat vote would corrupt it.
func TestG3_VoteRejectedOnNativePoll(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)

	native, err := repo.UpsertNativePoll(ctx, overlay, "ext-g3", "native poll?", models.PollActive,
		[]repository.NativeOutcomeInput{{ExternalID: "c1", Idx: 1, Label: "yes", Votes: 2}, {ExternalID: "c2", Idx: 2, Label: "no", Votes: 1}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, native)
	require.Equal(t, models.SourceTwitchNative, native.Source)

	accepted, err := repo.RecordVote(ctx, native.ID, viewer, overlay, 1, "twitch", nil, 1)
	require.NoError(t, err)
	assert.False(t, accepted, "an All-Chat vote must never land on a mirrored twitch_native poll")
	n, err := repo.TotalVotes(ctx, native.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n, "no poll_votes row recorded on the native poll")
}

// TestH1_WagerRejectedOnNativePrediction: Wager against a mirrored twitch_native
// prediction must be rejected with reason "native", leave the viewer's balance
// untouched, and record no prediction_entries row — the round uses Twitch Channel
// Points and its payout path never touches the All-Chat economy.
func TestH1_WagerRejectedOnNativePrediction(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	overlay := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)
	_, err := repo.AwardPoints(ctx, viewer, overlay, 1000, "seed", "test", nil, "seed:native:"+viewer.String())
	require.NoError(t, err)

	native, err := repo.UpsertNativePrediction(ctx, overlay, "extp-h1", "native who?", models.PredActive, "",
		[]repository.NativeOutcomeInput{{ExternalID: "o1", Idx: 1, Label: "a", Points: 10, Entrants: 2}, {ExternalID: "o2", Idx: 2, Label: "b", Points: 5, Entrants: 1}}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, native)
	require.Equal(t, models.SourceTwitchNative, native.Source)

	res, err := repo.Wager(ctx, native.ID, viewer, overlay, 1, 100, "twitch", nil, "")
	require.NoError(t, err)
	assert.False(t, res.Accepted, "an All-Chat wager must never land on a mirrored twitch_native prediction")
	assert.Equal(t, "native", res.Reason)

	bal, err := repo.GetBalance(ctx, viewer, overlay)
	require.NoError(t, err)
	assert.EqualValues(t, 1000, bal, "no points debited from the native rejection")
	oid, amt, err := repo.GetViewerEntry(ctx, native.ID, viewer)
	require.NoError(t, err)
	assert.Nil(t, oid, "no prediction_entries row recorded on the native prediction")
	assert.EqualValues(t, 0, amt)
}
