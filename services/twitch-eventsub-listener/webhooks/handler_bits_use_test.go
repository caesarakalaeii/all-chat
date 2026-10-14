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

// Payload shape from the Twitch EventSub reference (Channel Bits Use Event).
func unmarshalBitsUse(t *testing.T, payload string) *eventsub.BitsUseEvent {
	t.Helper()
	var e eventsub.BitsUseEvent
	if err := json.Unmarshal([]byte(payload), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &e
}

func TestBuildPowerUpCelebration(t *testing.T) {
	msg := buildPowerUp(unmarshalBitsUse(t, `{
		"user_id": "1", "user_login": "Viewer", "user_name": "Viewer",
		"broadcaster_user_id": "2", "broadcaster_user_login": "Streamer", "broadcaster_user_name": "Streamer",
		"bits": 500, "type": "power_up",
		"message": {"text": "gg", "fragments": [{"type": "text", "text": "gg"}]},
		"power_up": {"type": "celebration", "emote": null, "message_effect_id": null}
	}`))
	if msg == nil {
		t.Fatal("celebration was dropped")
	}
	if msg.EventType != "power_up" || msg.EventData["power_up_type"] != "celebration" || msg.EventData["bits"] != 500 {
		t.Errorf("got type %q data %v", msg.EventType, msg.EventData)
	}
	if msg.ChannelID != "streamer" || msg.Username != "viewer" {
		t.Errorf("channel/user not lowercased: %q/%q", msg.ChannelID, msg.Username)
	}
	// No power_ups_* chat message accompanies a celebration, so its message must stay on the event.
	if msg.Text != "gg" {
		t.Errorf("Text = %q, want the viewer's message", msg.Text)
	}
}

// Message effects and gigantified emotes also arrive on channel.chat.message; repeating the
// text on the event would show the viewer's message twice.
func TestBuildPowerUpOmitsTextAlreadyDeliveredAsChat(t *testing.T) {
	for _, tt := range []struct{ powerUp, wantName string }{
		{`{"type": "message_effect", "message_effect_id": "fx"}`, ""},
		{`{"type": "gigantify_an_emote", "emote": {"id": "25", "name": "Kappa"}}`, "Kappa"},
	} {
		msg := buildPowerUp(unmarshalBitsUse(t, `{
			"user_login": "viewer", "broadcaster_user_login": "streamer", "bits": 100, "type": "power_up",
			"message": {"text": "hello Kappa"}, "power_up": `+tt.powerUp+`}`))
		if msg == nil {
			t.Fatalf("%s was dropped", tt.powerUp)
		}
		if msg.Text != "" {
			t.Errorf("%s: Text = %q, want empty", tt.powerUp, msg.Text)
		}
		if msg.EventData["power_up_name"] != tt.wantName {
			t.Errorf("%s: power_up_name = %v, want %q", tt.powerUp, msg.EventData["power_up_name"], tt.wantName)
		}
	}
}

func TestBuildPowerUpCustom(t *testing.T) {
	msg := buildPowerUp(unmarshalBitsUse(t, `{
		"user_login": "viewer", "broadcaster_user_login": "streamer", "bits": 300, "type": "custom_power_up",
		"custom_power_up": {"title": "Spin the Wheel", "reward_id": "r1"}
	}`))
	if msg == nil {
		t.Fatal("custom Power-up was dropped")
	}
	if msg.EventData["power_up_type"] != "custom" || msg.EventData["power_up_name"] != "Spin the Wheel" {
		t.Errorf("EventData = %v", msg.EventData)
	}
}

// channel.cheer already publishes cheers; publishing them from channel.bits.use too would double
// every cheer. A payload whose type and object disagree is unusable and also skipped.
func TestBuildPowerUpSkipsNonPowerUps(t *testing.T) {
	for _, payload := range []string{
		`{"user_login": "viewer", "broadcaster_user_login": "streamer", "bits": 100, "type": "cheer", "message": {"text": "Cheer100"}}`,
		`{"user_login": "viewer", "broadcaster_user_login": "streamer", "bits": 100, "type": "power_up"}`,
		`{"user_login": "viewer", "broadcaster_user_login": "streamer", "bits": 100, "type": "custom_power_up"}`,
	} {
		if msg := buildPowerUp(unmarshalBitsUse(t, payload)); msg != nil {
			t.Errorf("payload %s produced %+v, want nil", payload, msg)
		}
	}
}
