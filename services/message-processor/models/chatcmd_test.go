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
)

func TestChatRoles(t *testing.T) {
	cases := []struct {
		name string
		raw  RawChatMessage
		want []string
	}{
		{"twitch badges", RawChatMessage{Platform: "twitch", Tags: map[string]string{"badges": "broadcaster/1,subscriber/12"}}, []string{"broadcaster", "subscriber"}},
		{"twitch founder is a sub", RawChatMessage{Platform: "twitch", Tags: map[string]string{"badges": "founder/0"}}, []string{"subscriber"}},
		{"twitch lead mod", RawChatMessage{Platform: "twitch", Tags: map[string]string{"badges": "lead_moderator/1,vip/1"}}, []string{"moderator", "vip"}},
		{"kick badges", RawChatMessage{Platform: "kick", Tags: map[string]string{"badges": "moderator,og"}}, []string{"moderator"}},
		{"youtube owner", RawChatMessage{Platform: "youtube", Tags: map[string]string{"is_owner": "true", "is_moderator": "false"}}, []string{"broadcaster"}},
		{"youtube mod member", RawChatMessage{Platform: "youtube", Tags: map[string]string{"is_moderator": "true", "badge_member_url": "https://x"}}, []string{"moderator", "subscriber"}},
		{"youtube legacy sponsor", RawChatMessage{Platform: "youtube", Tags: map[string]string{"is_sponsor": "true"}}, []string{"subscriber"}},
		{"no tags", RawChatMessage{Platform: "twitch"}, nil},
		{"tiktok has no roles", RawChatMessage{Platform: "tiktok", Tags: map[string]string{"badges": "moderator"}}, nil},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ChatRoles(&c.raw), c.name)
	}
}

func TestChatCommandChannelMember_CaseNormalized(t *testing.T) {
	assert.Equal(t, "twitch:caesarlp", ChatCommandChannelMember("Twitch", "CaesarLP"))
}

func TestChatDisplayName(t *testing.T) {
	assert.Equal(t, "CaesarLP", ChatDisplayName(&RawChatMessage{Username: "caesarlp", Tags: map[string]string{"display-name": "CaesarLP"}}))
	assert.Equal(t, "kickuser", ChatDisplayName(&RawChatMessage{Username: "kickuser"}))
}
