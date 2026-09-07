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

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The nine-scope count is a contract with auth-service's consent flow, which is where the
// list is authored (services/auth-service/oauth/twitch.go). A tenth scope appearing there
// without appearing here would silently hide the opt-in banner from streamers whose grant
// is one scope short of being able to subscribe.
func TestModLogScopesCount(t *testing.T) {
	assert.Len(t, ModLogTwitchScopes, 9,
		"modlog must map to the nine channel.moderate + AutoMod scopes")
}

func TestModLogGranted(t *testing.T) {
	require.Len(t, ModLogTwitchScopes, 9)
	all := append([]string(nil), ModLogTwitchScopes...)

	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"no scopes at all", nil, false},
		{"login scopes only", []string{"user:read:chat", "channel:bot"}, false},
		{"all nine", all, true},
		{"all nine mixed with unrelated grants", append([]string{"user:read:chat", ScopeTwitchSend}, all...), true},
		{"eight of nine, missing moderator:manage:automod", all[:len(all)-1], false},
		{"eight of nine, missing a read scope", append([]string{}, all[1:]...), false},
		{"duplicated subset does not stand in for the rest", []string{all[0], all[0], all[1]}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ModLogGranted(tt.scopes))
		})
	}
}
