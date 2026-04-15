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

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapEventTypeToColumn_Twitch(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"subscription", "enable_twitch_subs"},
		{"resubscription", "enable_twitch_resubs"},
		{"gift_subscription", "enable_twitch_gift_subs"},
		{"mystery_gift", "enable_twitch_gift_subs"},
		{"bits", "enable_twitch_bits"},
		{"raid", "enable_twitch_raids"},
		{"unraid", "enable_twitch_raids"},
		{"channel_points", "enable_twitch_channel_points"},
		{"follow", "enable_twitch_follows"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("twitch", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_YouTube(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"super_chat", "enable_youtube_super_chat"},
		{"super_sticker", "enable_youtube_super_sticker"},
		{"new_sponsor", "enable_youtube_members"},
		{"member_milestone", "enable_youtube_member_milestones"},
		{"membership_gift", "enable_youtube_member_gifts"},
		{"gift_received", "enable_youtube_member_gifts"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("youtube", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_TikTok(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"like_aggregate", "enable_tiktok_likes"},
		{"gift", "enable_tiktok_gifts"},
		{"follow", "enable_tiktok_follows"},
		{"share", "enable_tiktok_shares"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("tiktok", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_UnknownPlatform(t *testing.T) {
	result := mapEventTypeToColumn("unknown_platform", "subscription")
	assert.Equal(t, "", result)
}
