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
	"reflect"
	"testing"

	"golang.org/x/oauth2"
)

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
