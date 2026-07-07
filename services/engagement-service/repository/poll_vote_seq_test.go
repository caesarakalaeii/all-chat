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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestP3_3_StaleVoteReplayDoesNotRevert: a 5m-drained redelivery of an OLDER vote must not
// overwrite a viewer's newer vote change. The monotonic seq guard on RecordVote's
// ON CONFLICT ... DO UPDATE blocks the stale re-apply.
func TestP3_3_StaleVoteReplayDoesNotRevert(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlay := seedOverlay(t, pool)
	viewer := seedViewer(t, pool)
	poll, err := repo.CreatePoll(ctx, overlay, "q", []string{"a", "b", "c"}, true, nil)
	require.NoError(t, err)
	opt2, opt3 := poll.Options[1].ID, poll.Options[2].ID

	// Vote option 1 (seq 100), then change to option 2 (seq 200).
	acc, err := repo.RecordVote(ctx, poll.ID, viewer, overlay, 1, "twitch", nil, 100)
	require.NoError(t, err)
	require.True(t, acc)
	_, err = repo.RecordVote(ctx, poll.ID, viewer, overlay, 2, "twitch", nil, 200)
	require.NoError(t, err)
	got, err := repo.GetViewerVote(ctx, poll.ID, viewer)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, opt2, *got, "the newer change wins")

	// A 5m-drained redelivery of the OLD option-1 vote (seq 100) must NOT revert it.
	_, err = repo.RecordVote(ctx, poll.ID, viewer, overlay, 1, "twitch", nil, 100)
	require.NoError(t, err)
	got, err = repo.GetViewerVote(ctx, poll.ID, viewer)
	require.NoError(t, err)
	assert.Equal(t, opt2, *got, "a stale redelivery must not revert the newer vote (P3-3)")

	// A genuinely newer change still applies.
	_, err = repo.RecordVote(ctx, poll.ID, viewer, overlay, 3, "twitch", nil, 300)
	require.NoError(t, err)
	got, err = repo.GetViewerVote(ctx, poll.ID, viewer)
	require.NoError(t, err)
	assert.Equal(t, opt3, *got, "a newer change still wins")
}
