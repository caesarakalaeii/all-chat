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

package webhooks

import (
	"encoding/json"
	"testing"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
)

// A watch streak is the reason this path exists: Twitch delivers the viewer's own chat message
// inside the notice, on no other subscription. Dropping the notice drops the message.
func TestBuildChatNoticeWatchStreakCarriesTheViewersMessage(t *testing.T) {
	e := &eventsub.ChatNotificationEvent{
		BroadcasterUserID:    "12345",
		BroadcasterUserLogin: "StreamerName",
		ChatterUserID:        "999",
		ChatterUserLogin:     "ViewerName",
		ChatterUserName:      "ViewerName",
		Color:                "#FF0000",
		Badges:               []eventsub.ChatBadge{{SetID: "subscriber", ID: "12"}},
		SystemMessage:        "ViewerName watched 5 consecutive streams this month!",
		MessageID:            "native-msg-1",
		NoticeType:           "watch_streak",
		Message: eventsub.ChatMessageBody{
			Text: "hey Kappa",
			Fragments: []eventsub.ChatMessageFragment{
				{Type: "text", Text: "hey "},
				{Type: "emote", Text: "Kappa", Emote: &eventsub.ChatEmote{ID: "25"}},
			},
		},
		WatchStreak: &eventsub.ChatNoticeWatchStreak{StreakCount: 5, ChannelPointsAwarded: 350},
	}

	msg := buildChatNotice(e)
	if msg == nil {
		t.Fatal("watch_streak notice was dropped; the viewer's message would be lost")
	}

	if msg.EventType != "watch_streak" {
		t.Errorf("EventType = %q, want watch_streak", msg.EventType)
	}
	// The viewer's text must win over Twitch's system message — that text IS the chat message.
	if msg.Text != "hey Kappa" {
		t.Errorf("Text = %q, want the viewer's message %q", msg.Text, "hey Kappa")
	}
	if msg.ChannelID != "streamername" {
		t.Errorf("ChannelID = %q, want lower-cased broadcaster login", msg.ChannelID)
	}
	if msg.Username != "viewername" {
		t.Errorf("Username = %q, want lower-cased chatter login", msg.Username)
	}
	if msg.UserID != "999" {
		t.Errorf("UserID = %q, want 999", msg.UserID)
	}

	// Chat-path tag parity: without these the message renders without emotes/badges/colour, and
	// per-channel emote lookups break (room-id is the enrichment key).
	if got := msg.Tags["id"]; got != "native-msg-1" {
		t.Errorf(`Tags["id"] = %q, want native-msg-1`, got)
	}
	if got := msg.Tags["room-id"]; got != "12345" {
		t.Errorf(`Tags["room-id"] = %q, want 12345`, got)
	}
	if got := msg.Tags["emotes"]; got != "25:4-8" {
		t.Errorf(`Tags["emotes"] = %q, want 25:4-8`, got)
	}
	if got := msg.Tags["badges"]; got != "subscriber/12" {
		t.Errorf(`Tags["badges"] = %q, want subscriber/12`, got)
	}
	if got := msg.Tags["subscriber"]; got != "1" {
		t.Errorf(`Tags["subscriber"] = %q, want 1`, got)
	}
	if got := msg.Tags["color"]; got != "#FF0000" {
		t.Errorf(`Tags["color"] = %q, want #FF0000`, got)
	}

	if got := msg.EventData["streak_count"]; got != 5 {
		t.Errorf("EventData[streak_count] = %v, want 5", got)
	}
	if got := msg.EventData["channel_points_awarded"]; got != 350 {
		t.Errorf("EventData[channel_points_awarded] = %v, want 350", got)
	}
	if got := msg.EventData["system_message"]; got != e.SystemMessage {
		t.Errorf("EventData[system_message] = %v, want the Twitch rendering", got)
	}
}

func TestBuildChatNoticeAnnouncementCarriesBodyAndColor(t *testing.T) {
	msg := buildChatNotice(&eventsub.ChatNotificationEvent{
		BroadcasterUserLogin: "streamer",
		ChatterUserLogin:     "mod_user",
		MessageID:            "native-msg-2",
		NoticeType:           "announcement",
		Message:              eventsub.ChatMessageBody{Text: "Raid at the end!"},
		Announcement:         &eventsub.ChatNoticeAnnouncement{Color: "PURPLE"},
	})
	if msg == nil {
		t.Fatal("announcement notice was dropped; the announcement body would be lost")
	}
	if msg.EventType != "announcement" {
		t.Errorf("EventType = %q, want announcement", msg.EventType)
	}
	if msg.Text != "Raid at the end!" {
		t.Errorf("Text = %q, want the announcement body", msg.Text)
	}
	if got := msg.EventData["announcement_color"]; got != "PURPLE" {
		t.Errorf("EventData[announcement_color] = %v, want PURPLE", got)
	}
}

// Notices that a dedicated subscription already delivers must never be emitted here, or every sub
// and raid would render twice.
func TestBuildChatNoticeSkipsNoticesCoveredByDedicatedSubscriptions(t *testing.T) {
	for _, noticeType := range []string{
		"sub", "resub", "sub_gift", "community_sub_gift", "raid",
		"shared_chat_sub", "shared_chat_resub", "shared_chat_sub_gift",
		"shared_chat_community_sub_gift", "shared_chat_raid",
	} {
		t.Run(noticeType, func(t *testing.T) {
			msg := buildChatNotice(&eventsub.ChatNotificationEvent{
				BroadcasterUserLogin: "streamer",
				ChatterUserLogin:     "viewer",
				MessageID:            "native-msg",
				NoticeType:           noticeType,
				Message:              eventsub.ChatMessageBody{Text: "thanks for 12 months"},
			})
			if msg != nil {
				t.Errorf("notice %q was emitted (EventType %q); it is already delivered by a dedicated subscription and would double-render",
					noticeType, msg.EventType)
			}
		})
	}
}

func TestBuildChatNoticeEventTypeMapping(t *testing.T) {
	tests := []struct {
		noticeType string
		want       string
	}{
		{"watch_streak", "watch_streak"},
		{"announcement", "announcement"},
		{"bits_badge_tier", "bits"}, // parity with the IRC parser's bitsbadgetier mapping
		{"gift_paid_upgrade", "gift_paid_upgrade"},
		{"prime_paid_upgrade", "prime_paid_upgrade"},
		{"pay_it_forward", "pay_it_forward"},
		{"unraid", "unraid"},
		{"charity_donation", "charity_donation"},
		{"modiversary", "modiversary"},
		// Shared-chat variants collapse onto the base type.
		{"shared_chat_announcement", "announcement"},
		{"shared_chat_pay_it_forward", "pay_it_forward"},
		{"shared_chat_gift_paid_upgrade", "gift_paid_upgrade"},
		{"shared_chat_prime_paid_upgrade", "prime_paid_upgrade"},
		{"shared_chat_modiversary", "modiversary"},
		// Anything Twitch adds later still reaches the overlay rather than vanishing.
		{"unknown", "twitch_notice"},
		{"some_future_notice", "twitch_notice"},
	}

	for _, tt := range tests {
		t.Run(tt.noticeType, func(t *testing.T) {
			msg := buildChatNotice(&eventsub.ChatNotificationEvent{
				BroadcasterUserLogin: "streamer",
				ChatterUserLogin:     "viewer",
				MessageID:            "native-msg",
				SystemMessage:        "something happened",
				NoticeType:           tt.noticeType,
			})
			if msg == nil {
				t.Fatalf("notice %q was dropped", tt.noticeType)
			}
			if msg.EventType != tt.want {
				t.Errorf("EventType = %q, want %q", msg.EventType, tt.want)
			}
			if got := msg.EventData["notice_type"]; got != tt.noticeType {
				t.Errorf("EventData[notice_type] = %v, want %q", got, tt.noticeType)
			}
		})
	}
}

// An event-only notice carries no chatter text, so it must fall back to Twitch's system message
// instead of rendering as a blank row.
func TestBuildChatNoticeFallsBackToSystemMessage(t *testing.T) {
	msg := buildChatNotice(&eventsub.ChatNotificationEvent{
		BroadcasterUserLogin: "streamer",
		ChatterUserLogin:     "viewer",
		MessageID:            "native-msg",
		SystemMessage:        "viewer is now a bits badge tier 1000 holder!",
		NoticeType:           "bits_badge_tier",
		BitsBadgeTier:        &eventsub.ChatNoticeBitsBadgeTier{Tier: 1000},
	})
	if msg == nil {
		t.Fatal("bits_badge_tier notice was dropped")
	}
	if msg.Text != "viewer is now a bits badge tier 1000 holder!" {
		t.Errorf("Text = %q, want the system message", msg.Text)
	}
	// "badge_tier" is the key the message-processor's existing "bits" case reads.
	if got := msg.EventData["badge_tier"]; got != 1000 {
		t.Errorf("EventData[badge_tier] = %v, want 1000", got)
	}
}

func TestBuildChatNoticeSharedChatProvenance(t *testing.T) {
	msg := buildChatNotice(&eventsub.ChatNotificationEvent{
		BroadcasterUserLogin:       "hoststreamer",
		ChatterUserLogin:           "viewer",
		MessageID:                  "native-msg",
		NoticeType:                 "shared_chat_announcement",
		Message:                    eventsub.ChatMessageBody{Text: "hello from the other channel"},
		SharedChatAnnouncement:     &eventsub.ChatNoticeAnnouncement{Color: "BLUE"},
		SourceBroadcasterUserID:    "777",
		SourceBroadcasterUserLogin: "sourcestreamer",
		SourceMessageID:            "source-msg",
	})
	if msg == nil {
		t.Fatal("shared-chat announcement was dropped")
	}
	if msg.EventType != "announcement" {
		t.Errorf("EventType = %q, want announcement", msg.EventType)
	}
	if got := msg.EventData["is_shared_chat"]; got != true {
		t.Errorf("EventData[is_shared_chat] = %v, want true", got)
	}
	if got := msg.EventData["source_channel"]; got != "sourcestreamer" {
		t.Errorf("EventData[source_channel] = %v, want sourcestreamer", got)
	}
	// The payload arrives under the "shared_chat_"-prefixed key, and must still be read.
	if got := msg.EventData["announcement_color"]; got != "BLUE" {
		t.Errorf("EventData[announcement_color] = %v, want BLUE", got)
	}
	if got := msg.Tags["source-room-id"]; got != "777" {
		t.Errorf(`Tags["source-room-id"] = %q, want 777`, got)
	}
}

func TestBuildChatNoticeCharityDonationAmount(t *testing.T) {
	msg := buildChatNotice(&eventsub.ChatNotificationEvent{
		BroadcasterUserLogin: "streamer",
		ChatterUserLogin:     "viewer",
		MessageID:            "native-msg",
		SystemMessage:        "viewer donated 12.34 EUR",
		NoticeType:           "charity_donation",
		CharityDonation: &eventsub.ChatNoticeCharityDonation{
			CharityName: "Some Charity",
			Amount:      eventsub.ChatNoticeMoneyAmt{Value: 1234, DecimalPlaces: 2, Currency: "EUR"},
		},
	})
	if msg == nil {
		t.Fatal("charity_donation notice was dropped")
	}
	if got := msg.EventData["charity_name"]; got != "Some Charity" {
		t.Errorf("EventData[charity_name] = %v, want Some Charity", got)
	}
	if got := msg.EventData["amount_value"]; got != 1234 {
		t.Errorf("EventData[amount_value] = %v, want 1234", got)
	}
	if got := msg.EventData["amount_decimal_places"]; got != 2 {
		t.Errorf("EventData[amount_decimal_places] = %v, want 2", got)
	}
	if got := msg.EventData["amount_currency"]; got != "EUR" {
		t.Errorf("EventData[amount_currency] = %v, want EUR", got)
	}
}

func TestBuildChatNoticeRejectsUnusablePayloads(t *testing.T) {
	tests := []struct {
		name  string
		event *eventsub.ChatNotificationEvent
	}{
		{
			name:  "missing broadcaster login cannot be routed to an overlay",
			event: &eventsub.ChatNotificationEvent{NoticeType: "watch_streak", ChatterUserLogin: "viewer"},
		},
		{
			name:  "missing notice type cannot be classified",
			event: &eventsub.ChatNotificationEvent{BroadcasterUserLogin: "streamer", ChatterUserLogin: "viewer"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if msg := buildChatNotice(tt.event); msg != nil {
				t.Errorf("expected nil, got %+v", msg)
			}
		})
	}
}

func TestBuildChatNoticeAnonymousChatter(t *testing.T) {
	msg := buildChatNotice(&eventsub.ChatNotificationEvent{
		BroadcasterUserLogin: "streamer",
		ChatterIsAnonymous:   true,
		MessageID:            "native-msg",
		SystemMessage:        "An anonymous user paid it forward!",
		NoticeType:           "pay_it_forward",
		PayItForward:         &eventsub.ChatNoticePayItForward{GifterIsAnonymous: true},
	})
	if msg == nil {
		t.Fatal("anonymous pay_it_forward notice was dropped")
	}
	if got := msg.EventData["is_anonymous"]; got != true {
		t.Errorf("EventData[is_anonymous] = %v, want true", got)
	}
	if got := msg.EventData["gifter_is_anonymous"]; got != true {
		t.Errorf("EventData[gifter_is_anonymous] = %v, want true", got)
	}
}

// Guards the wire contract: a verbatim Twitch watch_streak payload must decode into the fields the
// builder reads. A silently renamed field would reintroduce the dropped-message bug.
func TestChatNotificationEventUnmarshalsTwitchWatchStreakPayload(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"chatter_user_id": "49912639",
		"chatter_user_login": "viewer",
		"chatter_user_name": "Viewer",
		"chatter_is_anonymous": false,
		"color": "#00FF7F",
		"badges": [{"set_id": "subscriber", "id": "3", "info": "3"}],
		"system_message": "Viewer watched 3 consecutive streams this month and sparked a watch streak!",
		"message_id": "d3e8c4f1-0000-4000-8000-000000000001",
		"message": {"text": "morning all", "fragments": [{"type": "text", "text": "morning all"}]},
		"notice_type": "watch_streak",
		"watch_streak": {"streak_count": 3, "channel_points_awarded": 300}
	}`

	var e eventsub.ChatNotificationEvent
	if err := json.Unmarshal([]byte(payload), &e); err != nil {
		t.Fatalf("failed to unmarshal Twitch watch_streak payload: %v", err)
	}
	if e.NoticeType != "watch_streak" {
		t.Errorf("NoticeType = %q, want watch_streak", e.NoticeType)
	}
	if e.Message.Text != "morning all" {
		t.Errorf("Message.Text = %q, want the viewer's message", e.Message.Text)
	}
	if e.WatchStreak == nil {
		t.Fatal("WatchStreak payload did not decode")
	}
	if e.WatchStreak.StreakCount != 3 {
		t.Errorf("StreakCount = %d, want 3", e.WatchStreak.StreakCount)
	}
	if e.WatchStreak.ChannelPointsAwarded != 300 {
		t.Errorf("ChannelPointsAwarded = %d, want 300", e.WatchStreak.ChannelPointsAwarded)
	}

	msg := buildChatNotice(&e)
	if msg == nil || msg.Text != "morning all" {
		t.Fatalf("verbatim payload did not produce the viewer's message: %+v", msg)
	}
}
