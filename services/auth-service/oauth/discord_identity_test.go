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
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The identify flow is a genuinely different Discord flow from the bot invite, and the two must
// not be conflated: the invite asks a server admin to add a bot and returns a guild_id, while
// this asks a person who they are and returns nothing else at all.

// TestGetIdentityAuthURL_RequestsIdentifyOnly is the least-privilege guard (ADR-0012). A
// volunteer moderator is being asked only "which Discord account are you"; anything more on that
// screen — guilds, email, or the bot scope — would be a scope grab.
func TestGetIdentityAuthURL_RequestsIdentifyOnly(t *testing.T) {
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")

	raw := d.GetIdentityAuthURL("state-1")
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()

	assert.Equal(t, "identify", q.Get("scope"), "identify and nothing else")
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "client-id", q.Get("client_id"))
	assert.Equal(t, "state-1", q.Get("state"))
	assert.Equal(t, "https://example.com/cb", q.Get("redirect_uri"),
		"the identify flow reuses the registered bot-invite redirect_uri, so linking needs no Discord dashboard change")
	assert.Empty(t, q.Get("permissions"), "a permission bitfield belongs to the bot invite, not to an identity link")
}

// TestGetIdentityAuthURL_IsNotTheBotInvite: the two URLs must differ in scope, or a user would be
// shown a server picker when asked to identify themselves.
func TestGetIdentityAuthURL_IsNotTheBotInvite(t *testing.T) {
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")

	identity, err := url.Parse(d.GetIdentityAuthURL("s"))
	require.NoError(t, err)
	invite, err := url.Parse(d.GetAuthURL("s"))
	require.NoError(t, err)

	assert.Equal(t, "identify", identity.Query().Get("scope"))
	assert.Equal(t, "bot", invite.Query().Get("scope"))
}

func TestGetIdentity(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_, _ = w.Write([]byte(`{"id":"198569499228766208","username":"volunteer","global_name":"A Volunteer"}`))
	}))
	defer srv.Close()
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")
	d.apiBase = srv.URL

	id, err := d.GetIdentity(context.Background(), "user-access-token")
	require.NoError(t, err)
	assert.Equal(t, "198569499228766208", id.ID)
	assert.Equal(t, "volunteer", id.Username)
	assert.Equal(t, "/users/@me", gotPath)
	assert.Equal(t, "Bearer user-access-token", gotAuth,
		"an identify grant is a USER token and uses Bearer; only the bot token uses the Bot scheme")
}

// TestGetIdentity_RejectsAnEmptyID: a 200 with no id would otherwise be stored as a link to the
// empty snowflake, which every permission read would then answer "not a member" for — an
// unexplainable dead end rather than a failure at link time.
func TestGetIdentity_RejectsAnEmptyID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"username":"nameless"}`))
	}))
	defer srv.Close()
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")
	d.apiBase = srv.URL

	_, err := d.GetIdentity(context.Background(), "tok")
	require.Error(t, err)
}

func TestGetIdentity_PropagatesFailure(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb")
		d.apiBase = srv.URL

		_, err := d.GetIdentity(context.Background(), "tok")
		require.Error(t, err, "status %d must not yield a silent empty identity", status)
		srv.Close()
	}
}

// TestGetIdentity_DoesNotUseTheBotToken pins the separation: the bot token must never be sent on
// the identify read, or the "identity" returned would be the bot's own and every user would link
// to the same account.
func TestGetIdentity_DoesNotUseTheBotToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"1","username":"u"}`))
	}))
	defer srv.Close()
	d := NewDiscordOAuth("client-id", "secret", "https://example.com/cb").WithBotToken("the-bot-token")
	d.apiBase = srv.URL

	_, err := d.GetIdentity(context.Background(), "the-user-token")
	require.NoError(t, err)
	assert.NotContains(t, gotAuth, "the-bot-token")
	assert.Equal(t, "Bearer the-user-token", gotAuth)
}
