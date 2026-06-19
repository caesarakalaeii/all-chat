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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDiscordClient returns a DiscordClient pointed at a server that records the
// request and replies with the given status.
func newTestDiscordClient(t *testing.T, status int, captured *capturedRequest) (*DiscordClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.auth = r.Header.Get("Authorization")
		captured.client = r.Header.Get("User-Agent")
		w.WriteHeader(status)
	}))
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}
	return c, srv.Close
}

func TestDiscordDeleteMessage(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusNoContent, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "chan-1", "msg-1")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/channels/chan-1/messages/msg-1", got.path)
	assert.Equal(t, "Bot bot-tok", got.auth, "Discord uses the Bot token scheme, not Bearer")
	assert.NotEmpty(t, got.client, "Discord requires a User-Agent header")
}

func TestDiscordDelete_NotFoundIsIdempotentSuccess(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusNotFound, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "chan-1", "gone")
	require.NoError(t, err, "a 404 (already deleted) is treated as success: DELETE is idempotent")
}

func TestDiscordDelete_StatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrDiscordUnauthorized},
		{http.StatusForbidden, ErrDiscordForbidden},
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestDiscordClient(t, tc.status, &got)
		err := c.DeleteMessage(context.Background(), "c", "m")
		assert.ErrorIs(t, err, tc.want, "status %d", tc.status)
		done()
	}

	// Unexpected statuses surface a descriptive error (not a sentinel).
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusInternalServerError, &got)
	defer done()
	err := c.DeleteMessage(context.Background(), "c", "m")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDiscordForbidden)
	assert.NotErrorIs(t, err, ErrDiscordUnauthorized)
}

func TestDiscordGuildIDForChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/channels/chan-9", r.URL.Path)
		assert.Equal(t, "Bot bot-tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"id":"chan-9","guild_id":"guild-42"}`))
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	gid, err := c.GuildIDForChannel(context.Background(), "chan-9")
	require.NoError(t, err)
	assert.Equal(t, "guild-42", gid)
}

func TestDiscordGuildIDForChannel_Forbidden(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusForbidden, &got)
	defer done()
	_, err := c.GuildIDForChannel(context.Background(), "c")
	assert.ErrorIs(t, err, ErrDiscordForbidden)
}

func TestDiscordGuildBotPermissions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/members/@me"):
			_, _ = w.Write([]byte(`{"roles":["r1","r2"]}`))
		case strings.HasSuffix(r.URL.Path, "/roles"):
			// @everyone (id==guildID "g") none; r1 = MANAGE_MESSAGES (8192); r2 = BAN_MEMBERS (4);
			// "other" is not held by the bot and must be ignored.
			_, _ = w.Write([]byte(`[{"id":"g","permissions":"0"},{"id":"r1","permissions":"8192"},{"id":"r2","permissions":"4"},{"id":"other","permissions":"8"}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	perms, err := c.GuildBotPermissions(context.Background(), "g")
	require.NoError(t, err)
	assert.Equal(t, uint64(8192|4), perms, "effective = OR of held roles' permissions; unheld 'other' role excluded")
}

func TestDiscordGuildBotPermissions_NotMemberIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/members/@me") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		t.Fatalf("roles should not be fetched when the bot is not a member")
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	perms, err := c.GuildBotPermissions(context.Background(), "g")
	require.NoError(t, err, "a non-member bot reports zero permissions, not an error")
	assert.Equal(t, uint64(0), perms)
}

func TestDiscordTimeoutMember(t *testing.T) {
	var (
		method, path, body string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	until := time.Now().Add(10 * time.Minute)
	require.NoError(t, c.TimeoutMember(context.Background(), "g", "u", until))
	assert.Equal(t, http.MethodPatch, method)
	assert.Equal(t, "/guilds/g/members/u", path)
	assert.Contains(t, body, "communication_disabled_until")
}

func TestDiscordBanMember(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	require.NoError(t, c.BanMember(context.Background(), "g", "u"))
	assert.Equal(t, http.MethodPut, method)
	assert.Equal(t, "/guilds/g/bans/u", path)
}

func TestDiscordUnbanMember_NotBannedIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	require.NoError(t, c.UnbanMember(context.Background(), "g", "u"), "a 404 (not banned) is idempotent success")
}

func TestDiscordMemberWrite_ForbiddenSurfacesSentinel(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusForbidden, &got)
	defer done()
	// A 403 on a member op means the bot lacks the moderation permission → re-invite.
	assert.ErrorIs(t, c.BanMember(context.Background(), "g", "u"), ErrDiscordForbidden)
}

// countingResolverAPI records how many times each lookup hits the "API" so the cache
// test can assert a miss-then-hit.
type countingResolverAPI struct {
	guildID    string
	perms      uint64
	guildCalls int
	permsCalls int
	guildErr   error
	permsErr   error
}

func (c *countingResolverAPI) GuildIDForChannel(context.Context, string) (string, error) {
	c.guildCalls++
	return c.guildID, c.guildErr
}

func (c *countingResolverAPI) GuildBotPermissions(context.Context, string) (uint64, error) {
	c.permsCalls++
	return c.perms, c.permsErr
}

func TestDiscordGuildResolver_CachesGuildAndPermissions(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	api := &countingResolverAPI{guildID: "guild-1", perms: 8192}
	r := NewDiscordGuildResolver(api, rdb)
	ctx := context.Background()

	// First GuildID call misses the cache (hits the API); the second is served from cache.
	for i := 0; i < 3; i++ {
		gid, gerr := r.GuildID(ctx, "chan-1")
		require.NoError(t, gerr)
		assert.Equal(t, "guild-1", gid)
	}
	assert.Equal(t, 1, api.guildCalls, "channel→guild is resolved once then cached")

	// Same for the effective permissions.
	for i := 0; i < 3; i++ {
		bits, berr := r.GuildBotPermissions(ctx, "guild-1")
		require.NoError(t, berr)
		assert.Equal(t, uint64(8192), bits)
	}
	assert.Equal(t, 1, api.permsCalls, "guild permissions are computed once then cached")
}

func TestDiscordGuildResolver_PropagatesErrorWithoutCaching(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	api := &countingResolverAPI{guildErr: errors.New("boom")}
	r := NewDiscordGuildResolver(api, rdb)

	_, err1 := r.GuildID(context.Background(), "c")
	_, err2 := r.GuildID(context.Background(), "c")
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, 2, api.guildCalls, "an error is not cached; the next call retries")
}
