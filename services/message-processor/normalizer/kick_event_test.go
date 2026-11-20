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
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

func kickEvent(eventType, username, text string, data map[string]interface{}) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: "kick-evt-1",
		Platform:  "kick",
		ChannelID: "somestreamer",
		Username:  username,
		Text:      text,
		Timestamp: time.Unix(1700000000, 0).UTC(),
		EventType: eventType,
		EventData: data,
	}
}

func TestKickNormalizeEvent(t *testing.T) {
	tests := []struct {
		name        string
		raw         *models.RawChatMessage
		wantAmount  float64
		wantUnit    string
		wantDisplay string
		wantTier    string
	}{
		{
			name:        "new subscription has no tenure to show",
			raw:         kickEvent("kick_subscription", "alice", "", map[string]interface{}{"months": float64(1)}),
			wantAmount:  1,
			wantUnit:    "months",
			wantDisplay: "New subscriber",
			wantTier:    "high",
		},
		{
			name:        "renewal shows the month count",
			raw:         kickEvent("kick_subscription", "alice", "", map[string]interface{}{"months": float64(7)}),
			wantAmount:  7,
			wantUnit:    "months",
			wantDisplay: "7 months",
			wantTier:    "high",
		},
		{
			name:        "gifted subs count the recipients",
			raw:         kickEvent("kick_gift_subscription", "bob", "", map[string]interface{}{"gift_count": float64(5)}),
			wantAmount:  5,
			wantUnit:    "gifts",
			wantDisplay: "5 gift subs",
			wantTier:    "high",
		},
		{
			name:        "host carries the viewer count",
			raw:         kickEvent("kick_host", "carol", "hello chat", map[string]interface{}{"viewers": float64(250)}),
			wantAmount:  250,
			wantUnit:    "viewers",
			wantDisplay: "250 viewers",
			wantTier:    "high",
		},
		{
			name:        "KICKs gift names the gift and its price",
			raw:         kickEvent("kick_kicks", "dave", "gg", map[string]interface{}{"kicks": float64(100), "gift_name": "Rage Quit"}),
			wantAmount:  100,
			wantUnit:    "kicks",
			wantDisplay: "Rage Quit (100 KICKs)",
			wantTier:    "medium",
		},
		{
			name:        "follow",
			raw:         kickEvent("kick_follow", "erin", "", nil),
			wantAmount:  1,
			wantUnit:    "follow",
			wantDisplay: "New follower",
			wantTier:    "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := NewKickNormalizer().NormalizeEvent(tt.raw, "overlay-1")
			if err != nil {
				t.Fatalf("NormalizeEvent: %v", err)
			}
			if unified.Event == nil || unified.Event.Value == nil {
				t.Fatalf("event or value missing: %+v", unified.Event)
			}
			if unified.Event.Type != tt.raw.EventType {
				t.Errorf("type = %q, want %q", unified.Event.Type, tt.raw.EventType)
			}
			v := unified.Event.Value
			if v.Amount != tt.wantAmount || v.Currency != tt.wantUnit || v.DisplayText != tt.wantDisplay {
				t.Errorf("value = %+v, want {%v %s %s}", *v, tt.wantAmount, tt.wantUnit, tt.wantDisplay)
			}
			if unified.Event.Tier != tt.wantTier {
				t.Errorf("tier = %q, want %q", unified.Event.Tier, tt.wantTier)
			}
			if unified.User.Username != tt.raw.Username || unified.Platform != "kick" || unified.OverlayID != "overlay-1" {
				t.Errorf("envelope = %+v", unified)
			}
			if unified.Message.Text != tt.raw.Text {
				t.Errorf("text = %q, want %q", unified.Message.Text, tt.raw.Text)
			}
		})
	}
}

func TestKickNormalizeEventRejectsUnknownAndForeign(t *testing.T) {
	n := NewKickNormalizer()
	if _, err := n.NormalizeEvent(kickEvent("kick_teleport", "x", "", nil), "o"); err == nil {
		t.Error("unknown event type: want error, got nil")
	}
	foreign := kickEvent("kick_follow", "x", "", nil)
	foreign.Platform = "twitch"
	if _, err := n.NormalizeEvent(foreign, "o"); err == nil {
		t.Error("foreign platform: want error, got nil")
	}
}
