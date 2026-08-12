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
