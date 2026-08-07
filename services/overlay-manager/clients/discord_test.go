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

package clients

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestDiscordClient_GuildIDForChannel_Success asserts the happy path: the bot token is
// sent as an Authorization header and the guild_id is read off the channel object.
func TestDiscordClient_GuildIDForChannel_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chan-1","guild_id":"guild-9","type":0}`))
	}))
	defer srv.Close()

	c := NewDiscordClient("bot-secret", zap.NewNop()).WithBaseURL(srv.URL)

	guildID, err := c.GuildIDForChannel(context.Background(), "chan-1")
	require.NoError(t, err)
	assert.Equal(t, "guild-9", guildID)
	assert.Equal(t, "/channels/chan-1", gotPath)
	assert.Equal(t, "Bot bot-secret", gotAuth,
		"must authenticate as the bot, not with a user token")
}

// TestDiscordClient_GuildIDForChannel_NotFound maps Discord's 404 to a distinct error so
// the caller can answer "that channel does not exist / the bot cannot see it" rather than
// treating it as a transient failure.
func TestDiscordClient_GuildIDForChannel_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Unknown Channel","code":10003}`))
	}))
	defer srv.Close()

	c := NewDiscordClient("bot-secret", zap.NewNop()).WithBaseURL(srv.URL)

	_, err := c.GuildIDForChannel(context.Background(), "nope")
	assert.ErrorIs(t, err, ErrDiscordChannelNotFound)
}

// TestDiscordClient_GuildIDForChannel_Forbidden covers a channel in a guild the bot is in
// but cannot view. It must NOT be reported as "not found" — the distinction drives whether
// the user is told to re-invite the bot.
func TestDiscordClient_GuildIDForChannel_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewDiscordClient("bot-secret", zap.NewNop()).WithBaseURL(srv.URL)

	_, err := c.GuildIDForChannel(context.Background(), "chan-1")
	assert.ErrorIs(t, err, ErrDiscordForbidden)
}

// TestDiscordClient_GuildIDForChannel_DMHasNoGuild guards the case that would otherwise
// return an empty guild id and let an empty-string ownership check through: DM and group-DM
// channels carry no guild_id at all.
func TestDiscordClient_GuildIDForChannel_DMHasNoGuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"dm-1","type":1}`))
	}))
	defer srv.Close()

	c := NewDiscordClient("bot-secret", zap.NewNop()).WithBaseURL(srv.URL)

	_, err := c.GuildIDForChannel(context.Background(), "dm-1")
	assert.ErrorIs(t, err, ErrDiscordNoGuild)
}

// TestDiscordClient_GuildIDForChannel_ServerError must surface a non-sentinel error so the
// caller fails closed rather than mistaking it for "unowned".
func TestDiscordClient_GuildIDForChannel_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewDiscordClient("bot-secret", zap.NewNop()).WithBaseURL(srv.URL)

	_, err := c.GuildIDForChannel(context.Background(), "chan-1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDiscordChannelNotFound))
	assert.False(t, errors.Is(err, ErrDiscordForbidden))
}
