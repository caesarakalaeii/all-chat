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
		{"engagement requests both read scopes", []string{"engagement"}, []string{"channel:read:polls", "channel:read:predictions"}},
		{"engagement + moderation unions", []string{"engagement", "delete", "ban"}, []string{"channel:read:polls", "channel:read:predictions", "moderator:manage:chat_messages", "moderator:manage:banned_users"}},
		{"duplicate engagement dedupes", []string{"engagement", "engagement"}, []string{"channel:read:polls", "channel:read:predictions"}},
		{"modlog requests the channel.moderate v2 reads plus automod manage", []string{"modlog"}, modlogScopes},
		{
			"modlog + delete unions and dedupes",
			[]string{"delete", "modlog"},
			append([]string{"moderator:manage:chat_messages"}, modlogScopes...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, ModerationScopesForActions(tt.actions))
		})
	}
}

// modlogScopes is the scope set the "modlog" re-consent must request: the eight
// moderator:read:* scopes channel.moderate v2 requires, plus moderator:manage:automod for
// the AutoMod hold/update subscriptions.
var modlogScopes = []string{
	"moderator:read:blocked_terms",
	"moderator:read:chat_settings",
	"moderator:read:unban_requests",
	"moderator:read:banned_users",
	"moderator:read:chat_messages",
	"moderator:read:warnings",
	"moderator:read:moderators",
	"moderator:read:vips",
	"moderator:manage:automod",
}

// The declaration order is part of the contract: the consent screen lists scopes in the order
// requested, so a reshuffle changes what the streamer reads on Twitch's page.
func TestModlogScopesAreReturnedInDeclarationOrderWithoutDuplicates(t *testing.T) {
	assert.Equal(t, modlogScopes, ModerationScopesForActions([]string{"modlog"}))
	assert.Equal(t, modlogScopes, ModerationScopesForActions([]string{"modlog", "modlog"}),
		"a repeated action must not duplicate its scopes")
}

// moderator:manage:automod looks wrong on a read-only feature and a future cleanup will try to
// remove it. Twitch requires it to create an automod.message.hold subscription; there is no
// read-only alternative, and without it the AutoMod panel has no events to show.
func TestModlogKeepsAutomodManageBecauseTheHoldSubscriptionNeedsIt(t *testing.T) {
	assert.Contains(t, ModerationScopesForActions([]string{"modlog"}), "moderator:manage:automod",
		"automod.message.hold cannot be subscribed to without moderator:manage:automod, "+
			"even though this feature only reads AutoMod events")
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
