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

package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBotID is the snowflake the fake /users/@me hands back.
const testBotID = "1483074909046444173"

// newDiscordMemberProbe serves /users/@me plus the guild-member route, recording every
// member path so a test can prove "@me" never lands in a {user_id} position.
func newDiscordMemberProbe(t *testing.T, memberStatus int, seen *[]string) (*DiscordOAuth, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/@me":
			_, _ = w.Write([]byte(`{"id":"` + testBotID + `"}`))
		case strings.Contains(r.URL.Path, "/members/"):
			*seen = append(*seen, r.URL.Path)
			w.WriteHeader(memberStatus)
			if memberStatus == http.StatusOK {
				_, _ = w.Write([]byte(`{"roles":[]}`))
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb").WithBotToken("bot-tok")
	d.apiBase = srv.URL
	return d, srv.Close
}

// TestCheckBotPermissions_UsesBotSnowflakeNotAtMe pins the contract Discord actually
// enforces: GET /guilds/{g}/members/{user_id} coerces its last segment to a snowflake and
// answers 400 NUMBER_TYPE_COERCE for "@me" (measured against the production application),
// so the membership check silently never worked.
func TestCheckBotPermissions_UsesBotSnowflakeNotAtMe(t *testing.T) {
	var memberPaths []string
	d, done := newDiscordMemberProbe(t, http.StatusOK, &memberPaths)
	defer done()

	missing, err := d.CheckBotPermissions(context.Background(), "guild-1")
	require.NoError(t, err)
	assert.Empty(t, missing, "a 200 member record means the bot joined")
	require.Len(t, memberPaths, 1)
	assert.Equal(t, "/guilds/guild-1/members/"+testBotID, memberPaths[0])
	assert.NotContains(t, memberPaths[0], "@me", "\"@me\" is valid only on /users/@me, never as a {user_id}")
}

// TestCheckBotPermissions_NotAMemberIsReported: the 404 branch is the whole point of the
// check — it is how a cancelled or immediately-removed invite gets noticed.
func TestCheckBotPermissions_NotAMemberIsReported(t *testing.T) {
	var memberPaths []string
	d, done := newDiscordMemberProbe(t, http.StatusNotFound, &memberPaths)
	defer done()

	missing, err := d.CheckBotPermissions(context.Background(), "guild-1")
	require.NoError(t, err)
	assert.Equal(t, []string{"Bot is not a member of this server"}, missing)
}

// TestDiscordBotUserID_CachedAcrossCalls: a bot user id is immutable, so it must not cost
// an API call on every Discord connect.
func TestDiscordBotUserID_CachedAcrossCalls(t *testing.T) {
	var identityCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/users/@me", r.URL.Path)
		identityCalls++
		_, _ = w.Write([]byte(`{"id":"` + testBotID + `"}`))
	}))
	defer srv.Close()
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb").WithBotToken("bot-tok")
	d.apiBase = srv.URL

	for i := 0; i < 3; i++ {
		id, err := d.botUserID(context.Background())
		require.NoError(t, err)
		assert.Equal(t, testBotID, id)
	}
	assert.Equal(t, 1, identityCalls)
}

// TestDiscordBotUserID_ErrorIsNotCached: one bad response must not permanently disable the
// membership check for the pod's lifetime.
func TestDiscordBotUserID_ErrorIsNotCached(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + testBotID + `"}`))
	}))
	defer srv.Close()
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb").WithBotToken("bot-tok")
	d.apiBase = srv.URL

	_, err := d.botUserID(context.Background())
	require.Error(t, err)
	id, err := d.botUserID(context.Background())
	require.NoError(t, err, "the next call retries rather than returning the cached failure")
	assert.Equal(t, testBotID, id)
}

// TestDiscordOAuth_DefaultAPIBase guards the seam: production must keep talking to Discord,
// so an unconfigured apiBase is the real API base, not the empty string.
func TestDiscordOAuth_DefaultAPIBase(t *testing.T) {
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")
	assert.Equal(t, discordAPIBase, d.apiBase)
}
