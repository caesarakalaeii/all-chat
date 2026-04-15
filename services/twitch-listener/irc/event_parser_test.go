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

package irc

import (
	"testing"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUserNotice_Subscription(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "sub",
		Channel:   "#xqc",
		SystemMsg: "viewer123 subscribed at Tier 1",
		User: twitch.User{
			ID:          "12345",
			Name:        "viewer123",
			DisplayName: "Viewer123",
		},
		Tags: map[string]string{
			"msg-param-sub-plan":          "1000",
			"msg-param-cumulative-months": "12",
			"msg-param-streak-months":     "6",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)
	assert.NotNil(t, rawMsg)

	// Verify basic fields
	assert.Equal(t, "twitch", rawMsg.Platform)
	assert.Equal(t, "xqc", rawMsg.ChannelID)
	assert.Equal(t, "12345", rawMsg.UserID)
	assert.Equal(t, "viewer123", rawMsg.Username)

	// Verify event fields
	assert.Equal(t, "subscription", rawMsg.EventType)
	assert.NotNil(t, rawMsg.EventData)

	// Verify event data
	assert.Equal(t, "1000", rawMsg.EventData["tier"])
	assert.Equal(t, 12, rawMsg.EventData["months"])
	assert.Equal(t, 6, rawMsg.EventData["streak_months"])
	assert.Equal(t, false, rawMsg.EventData["is_gift"])
}

func TestParseUserNotice_Resubscription(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "resub",
		Channel:   "#summit1g",
		SystemMsg: "viewer456 subscribed at Tier 2. They've subscribed for 24 months!",
		User: twitch.User{
			ID:          "67890",
			Name:        "viewer456",
			DisplayName: "Viewer456",
		},
		Tags: map[string]string{
			"msg-param-sub-plan":          "2000",
			"msg-param-cumulative-months": "24",
			"msg-param-streak-months":     "18",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "resubscription", rawMsg.EventType)
	assert.Equal(t, "2000", rawMsg.EventData["tier"])
	assert.Equal(t, 24, rawMsg.EventData["months"])
	assert.Equal(t, 18, rawMsg.EventData["streak_months"])
}

func TestParseUserNotice_GiftSubscription(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "subgift",
		Channel:   "#pokimane",
		SystemMsg: "gifter123 gifted a Tier 1 sub to recipient456!",
		User: twitch.User{
			ID:          "11111",
			Name:        "gifter123",
			DisplayName: "Gifter123",
		},
		Tags: map[string]string{
			"msg-param-sub-plan":            "1000",
			"msg-param-recipient-id":        "22222",
			"msg-param-recipient-user-name": "recipient456",
			"msg-param-months":              "1",
			"msg-param-sender-count":        "5",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "gift_subscription", rawMsg.EventType)
	assert.Equal(t, "1000", rawMsg.EventData["tier"])
	assert.Equal(t, true, rawMsg.EventData["is_gift"])
	assert.Equal(t, "22222", rawMsg.EventData["recipient_id"])
	assert.Equal(t, "recipient456", rawMsg.EventData["recipient_name"])
	assert.Equal(t, 1, rawMsg.EventData["months"])
	assert.Equal(t, 5, rawMsg.EventData["sender_total_gifts"])
}

func TestParseUserNotice_MysteryGift(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "submysterygift",
		Channel:   "#shroud",
		SystemMsg: "gifter789 is gifting 50 Tier 1 Subs to the community!",
		User: twitch.User{
			ID:          "99999",
			Name:        "gifter789",
			DisplayName: "Gifter789",
		},
		Tags: map[string]string{
			"msg-param-sub-plan":        "1000",
			"msg-param-mass-gift-count": "50",
			"msg-param-sender-count":    "100",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "mystery_gift", rawMsg.EventType)
	assert.Equal(t, "1000", rawMsg.EventData["tier"])
	assert.Equal(t, 50, rawMsg.EventData["gift_count"])
	assert.Equal(t, 100, rawMsg.EventData["sender_total_gifts"])
}

func TestParseUserNotice_Raid(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "raid",
		Channel:   "#target_channel",
		SystemMsg: "5000 raiders from raider_channel have joined!",
		User: twitch.User{
			ID:          "55555",
			Name:        "raider_channel",
			DisplayName: "RaiderChannel",
		},
		Tags: map[string]string{
			"msg-param-displayName":  "RaiderChannel",
			"msg-param-login":        "raider_channel",
			"msg-param-viewerCount":  "5000",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "raid", rawMsg.EventType)
	assert.Equal(t, "raider_channel", rawMsg.EventData["raider_login"])
	assert.Equal(t, 5000, rawMsg.EventData["viewer_count"])
}

func TestParseUserNotice_Bits(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "bitsbadgetier",
		Channel:   "#ninja",
		SystemMsg: "bits viewer earned a new bits badge!",
		User: twitch.User{
			ID:          "77777",
			Name:        "bitsviewer",
			DisplayName: "BitsViewer",
		},
		Tags: map[string]string{
			"msg-param-threshold": "10000",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "bits", rawMsg.EventType)
	assert.Equal(t, 10000, rawMsg.EventData["badge_tier"])
}

func TestParseUserNotice_Ritual(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:     "ritual",
		Channel:   "#lirik",
		SystemMsg: "newbie123 is new here. Say hello!",
		User: twitch.User{
			ID:          "88888",
			Name:        "newbie123",
			DisplayName: "Newbie123",
		},
		Tags: map[string]string{
			"msg-param-ritual-name": "new_chatter",
		},
		Time: time.Now(),
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	require.NoError(t, err)

	assert.Equal(t, "ritual", rawMsg.EventType)
	assert.Equal(t, "new_chatter", rawMsg.EventData["ritual_name"])
}

func TestParseUserNotice_MissingChannel(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:   "sub",
		Channel: "",
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	assert.Error(t, err)
	assert.Nil(t, rawMsg)
	assert.Contains(t, err.Error(), "missing channel")
}

func TestParseUserNotice_MissingMsgID(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:   "",
		Channel: "#test",
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	assert.Error(t, err)
	assert.Nil(t, rawMsg)
	assert.Contains(t, err.Error(), "missing msg-id")
}

func TestParseUserNotice_UnknownMsgID(t *testing.T) {
	parser := NewParser()

	msg := twitch.UserNoticeMessage{
		MsgID:   "unknown_event_type",
		Channel: "#test",
	}

	rawMsg, err := parser.ParseUserNotice(msg)
	assert.Error(t, err)
	assert.Nil(t, rawMsg)
	assert.Contains(t, err.Error(), "unknown msg-id")
}

func TestMapMsgIDToEventType(t *testing.T) {
	tests := []struct {
		msgID    string
		expected string
	}{
		{"sub", "subscription"},
		{"resub", "resubscription"},
		{"subgift", "gift_subscription"},
		{"anonsubgift", "gift_subscription"},
		{"submysterygift", "mystery_gift"},
		{"raid", "raid"},
		{"unraid", "unraid"},
		{"bitsbadgetier", "bits"},
		{"ritual", "ritual"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.msgID, func(t *testing.T) {
			result := mapMsgIDToEventType(tt.msgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
