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

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
)

// Cross-service pipeline test for the dropped-watch-streak bug (ADR-0046): it starts from a verbatim
// Twitch channel.chat.notification payload and runs it through the real stages a message crosses —
// listener conversion → the JSON wire format of the chat:raw stream → the message-processor's Twitch
// event normalizer — asserting the viewer's message survives all the way to the unified message the
// overlay renders.
//
// A true end-to-end test (real Twitch → webhook → Redis → gateway → browser overlay) is not
// achievable for this event: a watch streak cannot be triggered on demand, because Twitch only emits
// it when a real viewer's multi-stream watch history crosses a milestone. There is no sandbox or API
// to synthesize one (the EventSub CLI's event set does not include chat notices), so the wire payload
// above — taken from Twitch's own reference — is the furthest upstream a test can start.
func TestChatNoticePipeline_WatchStreakReachesUnifiedMessage(t *testing.T) {
	const twitchPayload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"chatter_user_id": "49912639",
		"chatter_user_login": "loyalviewer",
		"chatter_user_name": "LoyalViewer",
		"chatter_is_anonymous": false,
		"color": "#00FF7F",
		"badges": [{"set_id": "subscriber", "id": "6", "info": "6"}],
		"system_message": "LoyalViewer watched 4 consecutive streams this month and sparked a watch streak!",
		"message_id": "b1a2c3d4-0000-4000-8000-00000000abcd",
		"message": {
			"text": "back again Kappa",
			"fragments": [
				{"type": "text", "text": "back again "},
				{"type": "emote", "text": "Kappa", "emote": {"id": "25", "emote_set_id": "0"}}
			]
		},
		"notice_type": "watch_streak",
		"watch_streak": {"streak_count": 4, "channel_points_awarded": 400}
	}`

	// Stage 1 — the listener decodes the webhook body and converts it.
	var event eventsub.ChatNotificationEvent
	if err := json.Unmarshal([]byte(twitchPayload), &event); err != nil {
		t.Fatalf("listener failed to decode the Twitch payload: %v", err)
	}
	rawMsg := buildChatNotice(&event)
	if rawMsg == nil {
		t.Fatal("listener dropped the watch-streak notice; the viewer's message is lost")
	}

	// Stage 2 — the message crosses the chat:raw Redis Stream as JSON. Round-tripping it here is
	// what turns Go ints in EventData into float64s downstream, the exact shape the normalizer must
	// tolerate.
	wire, err := rawMsg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal the raw message for the stream: %v", err)
	}
	consumed, err := mpmodels.ParseRawMessage(wire)
	if err != nil {
		t.Fatalf("message-processor failed to parse the streamed message: %v", err)
	}

	// Stage 3 — normalization into the unified message the API Gateway broadcasts.
	unified, err := normalizer.NewTwitchNormalizer().NormalizeEvent(consumed, "overlay-1")
	if err != nil {
		t.Fatalf("normalization failed: %v", err)
	}

	if unified.Message.Text != "back again Kappa" {
		t.Errorf("Message.Text = %q, want the viewer's message %q", unified.Message.Text, "back again Kappa")
	}
	if unified.Event == nil {
		t.Fatal("unified message carries no event info")
	}
	if unified.Event.Type != "watch_streak" {
		t.Errorf("Event.Type = %q, want watch_streak", unified.Event.Type)
	}
	if unified.Event.Value == nil || unified.Event.Value.DisplayText != "4-stream watch streak" {
		t.Errorf("Event.Value = %+v, want display text %q", unified.Event.Value, "4-stream watch streak")
	}
	if unified.Event.Tier != "low" || unified.Event.Duration != 12 {
		t.Errorf("Event tier/duration = %q/%d, want low/12", unified.Event.Tier, unified.Event.Duration)
	}

	// The viewer's first-party emote must render, not appear as literal "Kappa" text.
	if len(unified.Message.Emotes) != 1 {
		t.Fatalf("Emotes = %+v, want the one Twitch emote from the notice message", unified.Message.Emotes)
	}
	if unified.Message.Emotes[0].Code != "Kappa" {
		t.Errorf("Emotes[0].Code = %q, want Kappa", unified.Message.Emotes[0].Code)
	}

	// Identity and channel routing must match the chat path so the row renders like any message.
	if unified.User.Username != "loyalviewer" {
		t.Errorf("User.Username = %q, want loyalviewer", unified.User.Username)
	}
	if unified.User.DisplayName != "LoyalViewer" {
		t.Errorf("User.DisplayName = %q, want LoyalViewer", unified.User.DisplayName)
	}
	if unified.ChannelID != "streamer" {
		t.Errorf("ChannelID = %q, want streamer", unified.ChannelID)
	}
	// The native id must survive so a later channel.chat.message_delete can remove this row and the
	// message-processor can dedup it against a same-id chat message.
	if got := consumed.Tags["id"]; got != "b1a2c3d4-0000-4000-8000-00000000abcd" {
		t.Errorf(`Tags["id"] = %q, want the native Twitch message id`, got)
	}
}

// The announcement body is likewise carried only by the notice, and must arrive as message text.
func TestChatNoticePipeline_AnnouncementReachesUnifiedMessage(t *testing.T) {
	const twitchPayload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"chatter_user_id": "555",
		"chatter_user_login": "modperson",
		"chatter_user_name": "ModPerson",
		"badges": [{"set_id": "moderator", "id": "1", "info": ""}],
		"system_message": "",
		"message_id": "c2b3d4e5-0000-4000-8000-00000000dcba",
		"message": {"text": "Follow the raid target at the end!", "fragments": [{"type": "text", "text": "Follow the raid target at the end!"}]},
		"notice_type": "announcement",
		"announcement": {"color": "ORANGE"}
	}`

	var event eventsub.ChatNotificationEvent
	if err := json.Unmarshal([]byte(twitchPayload), &event); err != nil {
		t.Fatalf("failed to decode the Twitch payload: %v", err)
	}
	rawMsg := buildChatNotice(&event)
	if rawMsg == nil {
		t.Fatal("listener dropped the announcement; the announcement body is lost")
	}

	wire, err := rawMsg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal for the stream: %v", err)
	}
	consumed, err := mpmodels.ParseRawMessage(wire)
	if err != nil {
		t.Fatalf("failed to parse the streamed message: %v", err)
	}

	unified, err := normalizer.NewTwitchNormalizer().NormalizeEvent(consumed, "overlay-1")
	if err != nil {
		t.Fatalf("normalization failed: %v", err)
	}

	if unified.Event.Type != "announcement" {
		t.Errorf("Event.Type = %q, want announcement", unified.Event.Type)
	}
	if unified.Message.Text != "Follow the raid target at the end!" {
		t.Errorf("Message.Text = %q, want the announcement body", unified.Message.Text)
	}
	if got := unified.Event.Metadata["announcement_color"]; got != "ORANGE" {
		t.Errorf("Metadata[announcement_color] = %v, want ORANGE", got)
	}
	// Moderator badge must survive so the announcement renders with its author's badges.
	if got := consumed.Tags["mod"]; got != "1" {
		t.Errorf(`Tags["mod"] = %q, want 1`, got)
	}
}
