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

package normalizer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKickBadgeIconURL_KnownTypes(t *testing.T) {
	tests := []struct {
		badgeType   string
		wantPrefix  string
		wantContain string
	}{
		{"broadcaster", "data:image/svg+xml,", "FF1CD2"},
		{"moderator", "data:image/svg+xml,", "00C7FF"},
		{"vip", "data:image/svg+xml,", "FFC900"},
		{"subscriber", "data:image/svg+xml,", "E1FF00"},
		{"founder", "data:image/svg+xml,", "FFC900"},
		{"verified", "data:image/svg+xml,", "1EFF00"},
	}

	for _, tt := range tests {
		t.Run(tt.badgeType, func(t *testing.T) {
			url := kickBadgeIconURL(tt.badgeType)
			assert.True(t, strings.HasPrefix(url, tt.wantPrefix), "badge %q should have SVG data URI", tt.badgeType)
			assert.Contains(t, url, tt.wantContain, "badge %q should contain expected color", tt.badgeType)
		})
	}
}

func TestKickBadgeIconURL_Aliases(t *testing.T) {
	broadcasterURL := kickBadgeIconURL("broadcaster")
	subscriberURL := kickBadgeIconURL("subscriber")

	assert.Equal(t, broadcasterURL, kickBadgeIconURL("host"), "host should resolve to broadcaster")
	assert.Equal(t, broadcasterURL, kickBadgeIconURL("streamer"), "streamer should resolve to broadcaster")
	assert.Equal(t, subscriberURL, kickBadgeIconURL("sub"), "sub should resolve to subscriber")
}

func TestKickBadgeIconURL_CaseInsensitive(t *testing.T) {
	assert.NotEmpty(t, kickBadgeIconURL("Moderator"))
	assert.NotEmpty(t, kickBadgeIconURL("BROADCASTER"))
	assert.NotEmpty(t, kickBadgeIconURL("VIP"))
}

func TestKickBadgeIconURL_UnknownType(t *testing.T) {
	assert.Empty(t, kickBadgeIconURL("unknown_badge"))
	assert.Empty(t, kickBadgeIconURL(""))
}

func TestKickNormalizer_ExtractBadges_WithIcons(t *testing.T) {
	n := NewKickNormalizer()
	kickPayload := map[string]interface{}{
		"id":          "test-123",
		"chatroom_id": 42,
		"content":     "hello",
		"sender": map[string]interface{}{
			"id":       1,
			"username": "testuser",
			"slug":     "testuser",
			"identity": map[string]interface{}{
				"color": "#FF0000",
				"badges": []map[string]interface{}{
					{"type": "moderator", "text": "Moderator"},
					{"type": "subscriber", "text": "3-Month Sub"},
				},
			},
		},
	}

	rawJSON, err := json.Marshal(kickPayload)
	require.NoError(t, err)

	raw := &models.RawChatMessage{
		MessageID:  "kick-msg-1",
		Platform:   "kick",
		ChannelID:  "test-channel",
		UserID:     "1",
		Username:   "testuser",
		Text:       "hello",
		Timestamp:  time.Now(),
		Tags:       map[string]string{},
		RawMessage: rawJSON,
	}

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	modBadge := findBadge(unified.User.Badges, "moderator")
	require.NotNil(t, modBadge, "moderator badge should be present")
	assert.True(t, strings.HasPrefix(modBadge.IconURL, "data:image/svg+xml,"), "moderator badge should have SVG icon")
	assert.Contains(t, modBadge.IconURL, "00C7FF", "moderator badge should be cyan")

	subBadge := findBadge(unified.User.Badges, "subscriber")
	require.NotNil(t, subBadge, "subscriber badge should be present")
	assert.True(t, strings.HasPrefix(subBadge.IconURL, "data:image/svg+xml,"), "subscriber badge should have SVG icon")
	assert.Equal(t, "3-Month Sub", subBadge.Version)
}

func TestKickNormalizer_ExtractBadges_BroadcasterFromTags(t *testing.T) {
	n := NewKickNormalizer()
	raw := &models.RawChatMessage{
		MessageID: "kick-msg-2",
		Platform:  "kick",
		ChannelID: "test-channel",
		UserID:    "2",
		Username:  "streamer",
		Text:      "hello chat",
		Timestamp: time.Now(),
		Tags:      map[string]string{"badges": "broadcaster"},
	}

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	badge := findBadge(unified.User.Badges, "broadcaster")
	require.NotNil(t, badge, "broadcaster badge should be present")
	assert.True(t, strings.HasPrefix(badge.IconURL, "data:image/svg+xml,"), "broadcaster badge should have SVG icon")
	assert.Contains(t, badge.IconURL, "FF1CD2", "broadcaster badge should be pink")
}
