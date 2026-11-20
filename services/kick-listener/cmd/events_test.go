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

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
)

type capturePublisher struct{ got []*publisher.RawMessage }

func (c *capturePublisher) Publish(_ context.Context, msg *publisher.RawMessage) error {
	c.got = append(c.got, msg)
	return nil
}

// The message-processor routes a raw message down its event path on the
// top-level event_type and reads the numbers from event_data.
func TestPublishKickEvent_WireFormat(t *testing.T) {
	pub := &capturePublisher{}
	event := &websocket.KickEvent{
		Type: "kick_gift_subscription", ID: "kick-gift-pi_1", ChatroomID: 42,
		Username: "gifter", Data: map[string]any{"gift_count": 5},
	}
	publishKickEvent(event, "overlay-1", "somestreamer", time.Unix(1700000000, 0).UTC(), pub, zap.NewNop())

	if len(pub.got) != 1 {
		t.Fatalf("published %d messages, want 1", len(pub.got))
	}
	wire, err := json.Marshal(pub.got[0])
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		MessageID string         `json:"message_id"`
		Platform  string         `json:"platform"`
		OverlayID string         `json:"overlay_id"`
		ChannelID string         `json:"channel_id"`
		Username  string         `json:"username"`
		EventType string         `json:"event_type"`
		EventData map[string]any `json:"event_data"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.EventType != "kick_gift_subscription" || decoded.EventData["gift_count"] != float64(5) {
		t.Errorf("event = %q %v", decoded.EventType, decoded.EventData)
	}
	if decoded.Platform != "kick" || decoded.OverlayID != "overlay-1" || decoded.ChannelID != "somestreamer" ||
		decoded.Username != "gifter" || decoded.MessageID != "kick-gift-pi_1" {
		t.Errorf("envelope = %+v", decoded)
	}
}
