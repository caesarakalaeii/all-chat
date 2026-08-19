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

package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// connectedPayload decodes the `data` object of a connected frame.
func connectedPayload(t *testing.T, msg *WSMessage) map[string]any {
	t.Helper()

	raw, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialise connected frame: %v", err)
	}

	var envelope struct {
		Type WSMessageType  `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to decode connected frame %q: %v", raw, err)
	}
	if envelope.Type != WSMessageTypeConnected {
		t.Fatalf("expected type %q, got %q", WSMessageTypeConnected, envelope.Type)
	}
	return envelope.Data
}

// The truncation warning rides the connected frame both clients already
// receive, so no new message type is needed on either surface.
func TestConnectedFrames_CarryReplayTruncated(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *WSMessage
	}{
		{"owner", NewConnected("overlay-1", true)},
		{"viewer", NewViewerConnected(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := connectedPayload(t, tc.msg)
			if data["replay_truncated"] != true {
				t.Errorf("expected replay_truncated true, got %#v", data["replay_truncated"])
			}
		})
	}
}

// Absent must resolve to false on the client, so a not-truncated replay is
// byte-for-byte what an older gateway would have sent. Omitting the field
// rather than sending `false` is what makes an old client and a new gateway
// behave exactly as they do today.
func TestConnectedFrames_OmitReplayTruncatedWhenFalse(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *WSMessage
	}{
		{"owner", NewConnected("overlay-1", false)},
		{"viewer", NewViewerConnected(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := connectedPayload(t, tc.msg)
			if _, present := data["replay_truncated"]; present {
				t.Errorf("replay_truncated must be omitted when false, got %#v", data["replay_truncated"])
			}
		})
	}
}

// The viewer frame must not start leaking the overlay ID just because it gained
// a field.
func TestViewerConnected_StillHidesOverlayID(t *testing.T) {
	raw, err := NewViewerConnected(true).ToJSON()
	if err != nil {
		t.Fatalf("failed to serialise viewer connected frame: %v", err)
	}
	if strings.Contains(string(raw), "overlay_id") {
		t.Errorf("viewer connected frame must not expose overlay_id: %s", raw)
	}
}
