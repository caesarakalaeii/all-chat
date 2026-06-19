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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestModerationScopesForActions(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    []string
	}{
		{"delete only", []string{"delete"}, []string{"moderator:manage:chat_messages"}},
		{"ban only", []string{"ban"}, []string{"moderator:manage:banned_users"}},
		{"timeout/ban/unban share one scope", []string{"timeout", "ban", "unban"}, []string{"moderator:manage:banned_users"}},
		{"delete + ban requests both", []string{"delete", "ban"}, []string{"moderator:manage:chat_messages", "moderator:manage:banned_users"}},
		{"unknown actions are ignored", []string{"nuke", "delete"}, []string{"moderator:manage:chat_messages"}},
		{"empty input yields nothing", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, ModerationScopesForActions(tt.actions))
		})
	}
}

func TestGetAuthURLWithScopes(t *testing.T) {
	o := NewTwitchOAuth("cid", "secret", "http://localhost/cb")
	// channel:read:redemptions duplicates a base login scope — it must be deduped.
	extra := []string{"user:read:chat", "moderator:manage:banned_users", "channel:read:redemptions"}

	raw := o.GetAuthURLWithScopes("state123", extra)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()

	assert.Equal(t, "true", q.Get("force_verify"), "force_verify is required to actually re-prompt")
	assert.Equal(t, "state123", q.Get("state"))

	scopes := strings.Fields(q.Get("scope"))
	assert.Contains(t, scopes, "moderator:read:followers", "base login scopes are still requested")
	assert.Contains(t, scopes, "user:read:chat", "extra scopes are requested")
	assert.Contains(t, scopes, "moderator:manage:banned_users")

	count := 0
	for _, s := range scopes {
		if s == "channel:read:redemptions" {
			count++
		}
	}
	assert.Equal(t, 1, count, "a scope present in both base and extra must appear once")
}

func TestExtractGrantedScopes(t *testing.T) {
	withScope := func(v interface{}) *oauth2.Token {
		return (&oauth2.Token{AccessToken: "x"}).WithExtra(map[string]interface{}{"scope": v})
	}

	tests := []struct {
		name  string
		token *oauth2.Token
		want  []string
	}{
		{name: "nil token", token: nil, want: nil},
		{name: "no scope extra", token: &oauth2.Token{AccessToken: "x"}, want: nil},
		{
			name:  "json array as twitch returns it",
			token: withScope([]interface{}{"user:read:chat", "user:bot", "channel:bot"}),
			want:  []string{"user:read:chat", "user:bot", "channel:bot"},
		},
		{
			name:  "space-delimited string is split",
			token: withScope("user:read:chat user:bot"),
			want:  []string{"user:read:chat", "user:bot"},
		},
		{
			name:  "non-string and empty entries are skipped",
			token: withScope([]interface{}{"a", 42, "", "b"}),
			want:  []string{"a", "b"},
		},
		{
			name:  "empty string returns nil",
			token: withScope(""),
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractGrantedScopes(tt.token)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractGrantedScopes() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
