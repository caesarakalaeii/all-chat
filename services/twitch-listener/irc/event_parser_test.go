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

// TestParseUserNotice_EventSubCoveredMsgIDsAreSkipped asserts that USERNOTICEs
// for events the EventSub listener already produces (subs, resubs, gift subs,
// mystery gifts, raids) are dropped silently — both nil rawMsg and nil error,
// so handleUserNotice publishes nothing. Without this, every Twitch
// subscription would surface twice (once via IRC, once via EventSub), and the
// IRC copy would also miss the months tenure that EventSub carries (#254).
func TestParseUserNotice_EventSubCoveredMsgIDsAreSkipped(t *testing.T) {
	parser := NewParser()

	covered := []string{"sub", "resub", "subgift", "anonsubgift", "submysterygift", "raid"}
	for _, msgID := range covered {
		t.Run(msgID, func(t *testing.T) {
			msg := twitch.UserNoticeMessage{
				MsgID:   msgID,
				Channel: "#somechannel",
				User: twitch.User{ID: "1", Name: "u", DisplayName: "U"},
				Tags:    map[string]string{},
				Time:    time.Now(),
			}
			rawMsg, err := parser.ParseUserNotice(msg)
			require.NoError(t, err, "EventSub-covered USERNOTICEs must not error — they are simply skipped")
			assert.Nil(t, rawMsg, "rawMsg must be nil so handleUserNotice does not publish")
		})
	}
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
