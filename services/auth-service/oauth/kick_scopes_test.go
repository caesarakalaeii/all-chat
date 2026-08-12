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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKickModerationScopesForActions(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    []string
	}{
		{"ban only", []string{"ban"}, []string{"moderation:ban"}},
		{"timeout/ban/unban share one scope", []string{"timeout", "ban", "unban"}, []string{"moderation:ban"}},
		{"delete has its own scope", []string{"delete"}, []string{"moderation:chat_message:manage"}},
		{
			"delete + ban asks for both, since Kick grants them separately",
			[]string{"delete", "ban"},
			[]string{"moderation:chat_message:manage", "moderation:ban"},
		},
		{"unknown actions are ignored", []string{"engagement"}, []string{}},
		{"empty input yields nothing", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, KickModerationScopesForActions(tt.actions))
		})
	}
}

func TestGetAuthURLWithScopesPKCE(t *testing.T) {
	o := NewKickOAuth("cid", "secret", "http://localhost/cb")
	// user:read duplicates the base login scope — it must be deduped.
	extra := []string{"user:read", "moderation:ban"}

	raw, verifier := o.GetAuthURLWithScopesPKCE("state123", extra)
	require.NotEmpty(t, verifier, "the PKCE verifier must be returned for the caller to store")

	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()

	assert.Equal(t, "state123", q.Get("state"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"), "Kick requires PKCE")
	assert.NotEmpty(t, q.Get("code_challenge"))

	scopes := strings.Fields(q.Get("scope"))
	assert.Contains(t, scopes, "user:read", "base identity scope is still requested")
	assert.Contains(t, scopes, "moderation:ban", "the moderation scope is requested")

	count := 0
	for _, s := range scopes {
		if s == "user:read" {
			count++
		}
	}
	assert.Equal(t, 1, count, "a scope present in both base and extra must appear once")
}

// The Kick token exchange must surface the granted scope where ExtractGrantedScopes reads it.
//
// This was a live bug rather than a hypothetical: the response field was parsed and dropped, so
// every Kick grant was stored with an empty granted_scopes and Kick moderation could never be
// enabled — the consent completed, the capability endpoint still said missing_scope, and the
// dispatcher's pre-check refused every action. A refresh grant never widens scopes, so nothing
// downstream could repair it.
func TestKickExchangeCarriesTheGrantedScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "verifier-123", r.Form.Get("code_verifier"), "PKCE verifier must be sent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"acc","refresh_token":"ref","expires_in":3600,
			"token_type":"Bearer","scope":"user:read moderation:ban"
		}`))
	}))
	defer srv.Close()

	o := NewKickOAuth("cid", "secret", "http://localhost/cb")
	o.tokenURL = srv.URL

	token, err := o.ExchangeCodeWithPKCE(context.Background(), "code-1", "verifier-123")
	require.NoError(t, err)
	assert.Equal(t, "acc", token.AccessToken)
	assert.Equal(t, []string{"user:read", "moderation:ban"}, ExtractGrantedScopes(token),
		"the granted scopes must survive the exchange, or no Kick grant is ever recorded")
}

// A response without a scope field must not fabricate one: granted_scopes is a record of what
// Kick confirmed, and inventing the requested set would claim a grant we cannot prove. The
// visible consequence (missing_scope) is the correct outcome.
func TestKickExchangeWithoutScopeRecordsNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"acc","refresh_token":"ref","expires_in":3600}`))
	}))
	defer srv.Close()

	o := NewKickOAuth("cid", "secret", "http://localhost/cb")
	o.tokenURL = srv.URL

	token, err := o.ExchangeCodeWithPKCE(context.Background(), "code-1", "v")
	require.NoError(t, err)
	assert.Empty(t, ExtractGrantedScopes(token))
}

// The seam must default to the real endpoint — a zero tokenURL would silently POST to a relative
// path and fail every exchange.
func TestKickOAuthDefaultsToTheRealTokenEndpoint(t *testing.T) {
	assert.Equal(t, kickTokenURL, NewKickOAuth("cid", "secret", "http://localhost/cb").tokenURL)
}
