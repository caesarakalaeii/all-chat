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

// TestP2_2_ActiveAllChatEngagements: the active-flag reconciler must re-arm exactly the
// rounds whose hot-path gate should be set — ACTIVE All-Chat polls and predictions — and
// exclude LOCKED predictions (wagers closed), CLOSED polls, and twitch_native rows.
func TestP2_2_ActiveAllChatEngagements(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	overlay := seedOverlay(t, pool)

	// ACTIVE all-chat poll + prediction: should be listed.
	poll, err := repo.CreatePoll(ctx, overlay, "q", []string{"a", "b"}, true, nil)
	require.NoError(t, err)
	pred, err := repo.CreatePrediction(ctx, overlay, "t", []string{"a", "b"}, nil)
	require.NoError(t, err)

	got, err := repo.ActiveAllChatEngagements(ctx)
	require.NoError(t, err)
	ids := idSet(got, overlay)
	assert.Contains(t, ids, poll.ID, "an ACTIVE all-chat poll must be armed")
	assert.Contains(t, ids, pred.ID, "an ACTIVE all-chat prediction must be armed")

	// LOCKED prediction must drop out (no more wagers → flag intentionally cleared).
	_, err = repo.LockPrediction(ctx, pred.ID, overlay)
	require.NoError(t, err)
	got, err = repo.ActiveAllChatEngagements(ctx)
	require.NoError(t, err)
	ids = idSet(got, overlay)
	assert.Contains(t, ids, poll.ID)
	assert.NotContains(t, ids, pred.ID, "a LOCKED prediction must NOT be re-armed")

	// CLOSED poll must drop out too.
	_, err = repo.ClosePoll(ctx, poll.ID, overlay)
	require.NoError(t, err)
	got, err = repo.ActiveAllChatEngagements(ctx)
	require.NoError(t, err)
	assert.NotContains(t, idSet(got, overlay), poll.ID, "a CLOSED poll must NOT be re-armed")
}

// idSet returns the engagement ids for a specific overlay (the shared test DB may hold
// rows from other tests, so scope to this test's overlay).
func idSet(refs []repository.EngagementRef, overlay uuid.UUID) map[uuid.UUID]bool {
	m := map[uuid.UUID]bool{}
	for _, r := range refs {
		if r.OverlayID == overlay {
			m[r.EngagementID] = true
		}
	}
	return m
}
