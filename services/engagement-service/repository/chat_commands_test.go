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

// Chat commands (ADR-0079) against a real Postgres with migration 113 applied.
// Run with: go test -tags=integration ./repository/...
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

// seedTwitchStreamer creates a Twitch-login user and an overlay carrying channels
// (platform -> channel_id). Returns the user id; cleanup cascades.
func seedTwitchStreamer(t *testing.T, pool *pgxpool.Pool, login string, scopes []string, sources map[string]string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var userID, overlayID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at, auth_provider, granted_scopes)
		 VALUES ($1, $2, $2, 'x', 'x', NOW() + INTERVAL '1 day', 'twitch', $3) RETURNING id`,
		"tid-"+login, login, scopes).Scan(&userID))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) })
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO overlays (user_id, name) VALUES ($1, $2) RETURNING id`, userID, "cc-"+login).Scan(&overlayID))
	for platform, channel := range sources {
		_, err := pool.Exec(ctx, `INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name)
			VALUES ($1, $2, $3, $3)`, overlayID, platform, channel)
		require.NoError(t, err)
	}
	return userID
}

func newChatCmd(userID uuid.UUID, kind, trigger string) models.ChatCommand {
	return models.ChatCommand{UserID: userID, Kind: kind, Trigger: trigger, Response: "hi",
		MinRole: "everyone", CooldownSeconds: 5, Enabled: true}
}

func TestChatCommandsCRUD(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	login := "cc" + uuid.NewString()[:8]
	owner := seedTwitchStreamer(t, pool, login, nil, nil)

	c, err := repo.CreateChatCommand(ctx, newChatCmd(owner, models.TriggerCommand, "!Hello"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), c.UseCount)

	_, err = repo.CreateChatCommand(ctx, newChatCmd(owner, models.TriggerCommand, "!hello"))
	assert.ErrorIs(t, err, repository.ErrDuplicateTrigger, "triggers are unique per user, ignoring case")
	_, err = repo.CreateChatCommand(ctx, newChatCmd(owner, models.TriggerKeyword, "!hello"))
	assert.NoError(t, err, "a keyword may share a command's text")

	c.Response = "changed"
	updated, err := repo.UpdateChatCommand(ctx, c)
	require.NoError(t, err)
	assert.Equal(t, "changed", updated.Response)

	stranger := c
	stranger.UserID = seedTwitchStreamer(t, pool, "cx"+uuid.NewString()[:8], nil, nil)
	_, err = repo.UpdateChatCommand(ctx, stranger)
	assert.ErrorIs(t, err, repository.ErrNotFound, "another streamer cannot edit it")
	assert.ErrorIs(t, repo.DeleteChatCommand(ctx, stranger.UserID, c.ID), repository.ErrNotFound)

	n, err := repo.IncrementChatCommandUse(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	list, err := repo.ListChatCommands(ctx, owner)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	count, err := repo.CountChatCommands(ctx, owner)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, repo.DeleteChatCommand(ctx, owner, c.ID))
}

// A channel resolves to every streamer whose overlay carries it, with Owned true only
// for the streamer it belongs to, and only enabled commands. The forwarded channel set
// follows the same rule.
func TestChatCommandsForChannel(t *testing.T) {
	pool := newTestDB(t)
	repo := repository.New(pool)
	ctx := context.Background()
	login := "cc" + uuid.NewString()[:8]
	kickSlug := "kick-" + login

	owner := seedTwitchStreamer(t, pool, login, []string{"user:write:chat", "clips:edit"},
		map[string]string{"twitch": login, "kick": kickSlug})
	fan := seedTwitchStreamer(t, pool, "fan"+login, nil, map[string]string{"twitch": login})
	silent := seedTwitchStreamer(t, pool, "nil"+login, nil, map[string]string{"twitch": login})

	_, err := repo.CreateChatCommand(ctx, newChatCmd(owner, models.TriggerCommand, "!a"))
	require.NoError(t, err)
	off := newChatCmd(owner, models.TriggerCommand, "!off")
	off.Enabled = false
	_, err = repo.CreateChatCommand(ctx, off)
	require.NoError(t, err)
	_, err = repo.CreateChatCommand(ctx, newChatCmd(fan, models.TriggerKeyword, "clip that"))
	require.NoError(t, err)
	_ = silent // has the channel but no commands: must not appear

	got, err := repo.ChatCommandsForChannel(ctx, "twitch", login)
	require.NoError(t, err)
	byUser := map[uuid.UUID]repository.ChannelCommands{}
	for _, cc := range got {
		byUser[cc.UserID] = cc
	}
	require.Len(t, byUser, 2)
	assert.True(t, byUser[owner].Owned)
	assert.Len(t, byUser[owner].Commands, 1, "disabled commands are left out")
	assert.False(t, byUser[fan].Owned, "the fan's overlay carries the channel but does not own it")

	members, err := repo.ChatCommandChannels(ctx)
	require.NoError(t, err)
	assert.Contains(t, members, "twitch:"+login)
	assert.Contains(t, members, "kick:"+kickSlug)

	st, err := repo.OwnerStatus(ctx, owner)
	require.NoError(t, err)
	assert.True(t, st.TwitchConnected)
	assert.True(t, st.TwitchCanSend)
	assert.True(t, st.TwitchCanClip)
	assert.NotNil(t, st.ConsentOverlayID, "the owner's overlay carries their own Twitch channel")
	st, err = repo.OwnerStatus(ctx, fan)
	require.NoError(t, err)
	assert.False(t, st.TwitchCanClip)
	assert.Nil(t, st.ConsentOverlayID, "the fan's overlay carries someone else's channel")
}
