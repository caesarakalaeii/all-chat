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
	"testing"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalize_TwitchBits: the dispatch must produce the message-processor's
// normalized EventInfo (value, tier, display text) and sender — the alert path
// must not invent its own interpretation of EventData, or alerts would drift
// from how the same event renders in chat.
func TestNormalize_TwitchBits(t *testing.T) {
	raw := &mpmodels.RawChatMessage{
		MessageID: "msg-1",
		Platform:  "twitch",
		ChannelID: "12345",
		UserID:    "u1",
		Username:  "somechatter",
		Tags:      map[string]string{"display-name": "Some Chatter"},
		EventType: "bits",
		EventData: map[string]interface{}{"badge_tier": 100},
	}

	info, user, err := Normalize(raw)
	require.NoError(t, err)

	require.NotNil(t, info)
	assert.Equal(t, "bits", info.Type)
	require.NotNil(t, info.Value)
	assert.Equal(t, float64(100), info.Value.Amount)
	assert.Equal(t, "bits", info.Value.Currency)
	assert.Equal(t, "100 bits", info.Value.DisplayText)

	assert.Equal(t, "u1", user.ID)
	assert.Equal(t, "Some Chatter", user.DisplayName)
}

// TestNormalize_YouTubeSuperChat: same for the YouTube path — the amount is
// carried as micros and must reach the alert unchanged.
func TestNormalize_YouTubeSuperChat(t *testing.T) {
	raw := &mpmodels.RawChatMessage{
		MessageID: "msg-2",
		Platform:  "youtube",
		ChannelID: "chan-1",
		UserID:    "u2",
		Username:  "MemberName",
		Tags:      map[string]string{"profile_image": "https://yt.example/a.png"},
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros":  float64(5000000),
			"currency":       "USD",
			"amount_display": "$5.00",
		},
	}

	info, user, err := Normalize(raw)
	require.NoError(t, err)

	require.NotNil(t, info)
	assert.Equal(t, "super_chat", info.Type)
	require.NotNil(t, info.Value)
	assert.Equal(t, float64(5000000), info.Value.Amount)
	assert.Equal(t, "$5.00", info.Value.DisplayText)

	assert.Equal(t, "u2", user.ID)
	assert.Equal(t, "https://yt.example/a.png", user.AvatarURL)
}

// TestNormalize_UnsupportedPlatform: a platform whose normalizer has no event
// path (Kick, Discord today) must return a deterministic error the processor
// can treat as permanent — not a panic, and not an invented EventInfo.
func TestNormalize_UnsupportedPlatform(t *testing.T) {
	raw := &mpmodels.RawChatMessage{
		Platform:  "kick",
		EventType: "gift",
		EventData: map[string]interface{}{"count": 5},
	}

	_, _, err := Normalize(raw)
	assert.Error(t, err, "platforms without an event normalizer must not produce alerts")
}
