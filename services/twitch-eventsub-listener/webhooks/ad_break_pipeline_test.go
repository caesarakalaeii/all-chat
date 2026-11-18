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
	"context"
	"encoding/json"
	"testing"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/redis/go-redis/v9"
)

// Twitch documents channel.ad_break.begin with an integer duration_seconds and a boolean
// is_automatic, but its own example payload quotes both. Both shapes must produce the same row.
const (
	adBreakManualQuotedPayload = `{
		"duration_seconds": "60",
		"started_at": "2019-11-16T10:11:12.634234626Z",
		"is_automatic": "false",
		"broadcaster_user_id": "1337",
		"broadcaster_user_login": "Cool_User",
		"broadcaster_user_name": "Cool_User",
		"requester_user_id": "42",
		"requester_user_login": "Some_Editor",
		"requester_user_name": "Some_Editor"
	}`
	adBreakAutomaticTypedPayload = `{
		"duration_seconds": 90,
		"started_at": "2019-11-16T10:11:12Z",
		"is_automatic": true,
		"broadcaster_user_id": "1337",
		"broadcaster_user_login": "cool_user",
		"broadcaster_user_name": "Cool_User",
		"requester_user_id": "1337",
		"requester_user_login": "cool_user",
		"requester_user_name": "Cool_User"
	}`
)

// runAdBreakPipeline takes a verbatim webhook event through listener conversion, the chat:raw JSON
// wire format and the message-processor's Twitch event normalizer.
func runAdBreakPipeline(t *testing.T, payload string) *mpmodels.UnifiedChatMessage {
	t.Helper()
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	if err := h.routeEvent(context.Background(), "channel.ad_break.begin", json.RawMessage(payload), ""); err != nil {
		t.Fatalf("listener failed to handle the ad break: %v", err)
	}
	rawMsg := firstStreamRawMessage(t, rc)
	wire, err := rawMsg.ToJSON()
	if err != nil {
		t.Fatalf("failed to marshal the raw message for the stream: %v", err)
	}
	consumed, err := mpmodels.ParseRawMessage(wire)
	if err != nil {
		t.Fatalf("message-processor failed to parse the streamed message: %v", err)
	}
	unified, err := normalizer.NewTwitchNormalizer().NormalizeEvent(consumed, "overlay-1")
	if err != nil {
		t.Fatalf("normalization failed: %v", err)
	}
	if unified.Event == nil {
		t.Fatal("unified message carries no event info")
	}
	if unified.Event.Type != "ad_break" {
		t.Errorf("Event.Type = %q, want ad_break", unified.Event.Type)
	}
	// A cue for the streamer, never an on-stream alert.
	if unified.Event.Duration != 0 {
		t.Errorf("Event.Duration = %d, want 0 so no OBS alert is shown", unified.Event.Duration)
	}
	return unified
}

func TestAdBreakPipeline_ManualBreakFromQuotedPayloadNamesTheRequester(t *testing.T) {
	unified := runAdBreakPipeline(t, adBreakManualQuotedPayload)

	if unified.ChannelID != "cool_user" {
		t.Errorf("ChannelID = %q, want the lowercased broadcaster login the overlays route on", unified.ChannelID)
	}
	if unified.Event.Value == nil || unified.Event.Value.DisplayText != "1m" {
		t.Errorf("Event.Value = %+v, want display text 1m", unified.Event.Value)
	}
	if got := unified.Event.Metadata["is_automatic"]; got != false {
		t.Errorf("Metadata[is_automatic] = %v, want false (quoted \"false\" must decode)", got)
	}
	if got := unified.Event.Metadata["requester_login"]; got != "some_editor" {
		t.Errorf("Metadata[requester_login] = %v, want some_editor", got)
	}
	if got := unified.Event.Metadata["started_at"]; got != "2019-11-16T10:11:12Z" {
		t.Errorf("Metadata[started_at] = %v, want the break's own start time", got)
	}
	if unified.Message.Text != "Ad break started by some_editor" {
		t.Errorf("Message.Text = %q", unified.Message.Text)
	}
}

func TestAdBreakPipeline_AutomaticBreakNamesNoRequester(t *testing.T) {
	unified := runAdBreakPipeline(t, adBreakAutomaticTypedPayload)

	if unified.Event.Value == nil || unified.Event.Value.DisplayText != "1m 30s" {
		t.Errorf("Event.Value = %+v, want display text 1m 30s", unified.Event.Value)
	}
	if got := unified.Event.Metadata["is_automatic"]; got != true {
		t.Errorf("Metadata[is_automatic] = %v, want true", got)
	}
	// Twitch fills the requester on scheduled breaks too; naming the broadcaster would read as if
	// they had started it.
	if _, ok := unified.Event.Metadata["requester_login"]; ok {
		t.Errorf("Metadata[requester_login] = %v, want it absent on an automatic break", unified.Event.Metadata["requester_login"])
	}
	if unified.Message.Text != "Automatic ad break" {
		t.Errorf("Message.Text = %q, want Automatic ad break", unified.Message.Text)
	}
}
