//go:build integration

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

// Chat timers (ADR-0081) against a real Postgres with migration 116 applied.
// Run with: go test -tags=integration ./repository/...
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

func newChatTimer(userID uuid.UUID, name string) models.ChatTimer {
	return models.ChatTimer{UserID: userID, Name: name, Message: "gg everyone",
		IntervalMinutes: new(5), Enabled: true}
}

func seedChatTimer(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name string) models.ChatTimer {
	t.Helper()
	repo := repository.New(pool)
	saved, err := repo.CreateChatTimer(context.Background(), newChatTimer(userID, name))
	require.NoError(t, err)
	return saved
}

// The claim is a conditional UPDATE against the last_fired_at the scheduler's
// decision saw: of N replicas seeing the same row, exactly one wins and every other
// claim returns false, and the winner's fire_count moved by exactly one.
func TestChatTimerClaimSingleWinner(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	// A Twitch streamer carries the chat sources and timer FKs.
	userID := seedTwitchStreamer(t, pool, "ct"+uuid.NewString()[:8], nil, nil)

	// Seed via SQL so last_fired_at is a FIXED past value both replicas "see".
	var timerID uuid.UUID
	var seen time.Time
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO chat_timers
		(user_id, name, message, interval_minutes, enabled, last_fired_at, lines_at_last_fire, fire_count)
		VALUES ($1, $2, 'gg', 5, TRUE, now() - INTERVAL '10 minutes', 0, 0)
		RETURNING id, last_fired_at`,
		userID, "gg"+uuid.NewString()[:8]).Scan(&timerID, &seen))

	// Both replicas loaded the same row, so both hold the stored last_fired_at.
	both := models.ChatTimer{
		ID: timerID, UserID: userID, Name: "x", Message: "gg",
		IntervalMinutes: new(5), Enabled: true, LastFiredAt: &seen,
	}

	// Replica 1 decides "due" against the seen last_fired_at and claims: wins.
	require.True(t, repo.ClaimChatTimer(ctx, both, 7))
	// Replica 2 raced the same tick with the same seen value: loses.
	require.False(t, repo.ClaimChatTimer(ctx, both, 7))

	var fireCount int64
	var linesAtFire int64
	var lastFired time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT fire_count, lines_at_last_fire, last_fired_at FROM chat_timers WHERE id = $1`, timerID).
		Scan(&fireCount, &linesAtFire, &lastFired))
	assert.Equal(t, int64(1), fireCount, "exactly one claimer incremented fire_count")
	assert.Equal(t, int64(7), linesAtFire, "the winner's counter snapshot is recorded")
	assert.WithinDuration(t, time.Now().UTC(), lastFired, 2*time.Second,
		"last_fired_at moved to the claim time, so the second claimer's WHERE no longer matches")

	// A third claim coming later with a STALE seen value also loses.
	require.False(t, repo.ClaimChatTimer(ctx, both, 9))
}

// CRUD round-trip including the case-insensitive unique name and the ownership
// scoping of update/delete.
func TestChatTimersCRUD(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	a := seedTwitchStreamer(t, pool, "cta"+uuid.NewString()[:8], nil, nil)
	b := seedTwitchStreamer(t, pool, "ctb"+uuid.NewString()[:8], nil, nil)

	saved, err := repo.CreateChatTimer(ctx, newChatTimer(a, "Greeting"))
	require.NoError(t, err)
	assert.True(t, saved.Enabled)
	require.NotNil(t, saved.LastFiredAt, "create starts the first period")

	// Case-insensitive duplicate name for the same user.
	_, err = repo.CreateChatTimer(ctx, newChatTimer(a, "greeting"))
	assert.Error(t, err, "lower(name) is unique per user")

	// Same name is fine for another user.
	other, err := repo.CreateChatTimer(ctx, newChatTimer(b, "Greeting"))
	require.NoError(t, err)

	require.NoError(t, repo.DeleteChatTimer(ctx, b, other.ID))
	err = repo.DeleteChatTimer(ctx, b, other.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	// Another user cannot update another's timer.
	steal := newChatTimer(b, "Nobody")
	steal.ID = saved.ID
	_, err = repo.UpdateChatTimer(ctx, steal)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	// Owner disables: the period clears.
	mine, err := repo.ListChatTimers(ctx, a)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	mine[0].Enabled = false
	_, err = repo.UpdateChatTimer(ctx, mine[0])
	require.NoError(t, err)
	mine, err = repo.ListChatTimers(ctx, a)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.False(t, mine[0].Enabled)
	assert.Nil(t, mine[0].LastFiredAt, "disabling clears the period")
}

// The scheduler's per-tick load: enabled timers only, each with its owner's premium
// flag. Every tick depends on this one query, so a SQL error here silences every timer.
func TestEnabledChatTimersCarryOwnerPremium(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()

	owner := seedTwitchStreamer(t, pool, "et"+uuid.NewString()[:8], nil, nil)
	_, err := pool.Exec(ctx, `UPDATE users SET is_premium = TRUE WHERE id = $1`, owner)
	require.NoError(t, err)
	on := seedChatTimer(t, pool, owner, "on")
	off := newChatTimer(owner, "off")
	off.Enabled = false
	_, err = repo.CreateChatTimer(ctx, off)
	require.NoError(t, err)

	all, err := repo.EnabledChatTimers(ctx)
	require.NoError(t, err)
	var mine []models.EnabledChatTimer
	for _, e := range all {
		if e.Timer.UserID == owner {
			mine = append(mine, e)
		}
	}
	require.Len(t, mine, 1, "the disabled timer is not loaded")
	assert.Equal(t, on.ID, mine[0].Timer.ID)
	assert.True(t, mine[0].Premium)
}

// No settings row reads as the All-Chat Bot; a saved choice reads back and can be changed.
func TestChatBotSettingDefaultsToBot(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	owner := seedTwitchStreamer(t, pool, "cs"+uuid.NewString()[:8], nil, nil)

	got, err := repo.GetChatBotSetting(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, "bot", got)

	require.NoError(t, repo.SetChatBotSetting(ctx, owner, "self"))
	got, err = repo.GetChatBotSetting(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, "self", got)

	require.NoError(t, repo.SetChatBotSetting(ctx, owner, "bot"))
	got, err = repo.GetChatBotSetting(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, "bot", got)
}
