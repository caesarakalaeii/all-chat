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

package websocket

import (
	"encoding/json"
	"testing"
)

// Fixture pinned to owncast/owncast develop, services/chat/events/userMessageEvent.go
// GetBroadcastPayload: the outbound CHAT event is a flat JSON object with
// id/timestamp/body/user/type and visible.
func TestUnmarshalChatMessage(t *testing.T) {
	raw := `{
		"id": "aBcDeFgHi",
		"timestamp": "2026-09-08T12:34:56.789012345Z",
		"type": "CHAT",
		"body": "<p>hello world</p>",
		"visible": true,
		"user": {
			"id": "bFzGqYvXc",
			"displayName": "gabek",
			"displayColor": 3,
			"isBot": false,
			"authenticated": false,
			"createdAt": "2026-01-02T03:04:05Z",
			"previousNames": ["oldname"]
		}
	}`

	var msg OwncastChatMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg.ID != "aBcDeFgHi" || msg.Type != "CHAT" || !msg.Visible {
		t.Fatalf("unexpected envelope: %+v", msg)
	}
	if msg.Body != "<p>hello world</p>" {
		t.Fatalf("body mismatch: %q", msg.Body)
	}
	if msg.User.ID != "bFzGqYvXc" || msg.User.DisplayName != "gabek" || msg.User.DisplayColor != 3 {
		t.Fatalf("user mismatch: %+v", msg.User)
	}
}

// Server coalesces queued outbound events into one websocket frame separated
// by newlines (chatclient.go writePump). handleEvent must split and drop
// non-CHAT events.
func TestHandleEvent_SplitsBatchedFrames(t *testing.T) {
	var got []string
	c := NewClient("https://watch.example.org", "tk", func(_ string, msg *OwncastChatMessage) {
		got = append(got, msg.Body)
	}, nil)

	batch := `{"id":"1","type":"USER_JOINED","user":{}}
{"id":"2","type":"CHAT","body":"hi","visible":true}
{"id":"3","type":"CHAT_ACTION","body":"x"}
{"id":"4","type":"CHAT","body":"yo","visible":true}`
	c.handleEvent(batch)

	if len(got) != 2 || got[0] != "hi" || got[1] != "yo" {
		t.Fatalf("expected only CHAT bodies [hi yo], got %v", got)
	}
}

func TestHandleEvent_DropsGarbageAndEmpty(t *testing.T) {
	var got int
	c := NewClient("https://watch.example.org", "tk", func(_ string, _ *OwncastChatMessage) {
		got++
	}, nil)

	c.handleEvent("not json")
	c.handleEvent(`{"id":"","body":"","type":"CHAT"}`)
	c.handleEvent(`{"id":"7","type":"CHAT","body":"real","visible":false}`)

	if got != 1 {
		t.Fatalf("expected exactly 1 forwarded message, got %d", got)
	}
}
