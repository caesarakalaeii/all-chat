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

// Cross-service pipeline test for AutoMod holds: it starts from a verbatim Twitch
// automod.message.hold payload and runs it through the real stages a moderation event crosses —
// listener conversion → the JSON wire format of the chat:raw stream → the message-processor's Twitch
// event normalizer — asserting the held text reaches the unified message the overlay renders.
//
// The normalizer has no case for mod_action, so this also pins the behaviour it relies on: EventData
// is copied verbatim into Event.Metadata and no EventValue is built. The duration assertion is what
// keeps a moderation event off the public OBS alert display.
func TestModerationPipeline_AutoModHoldReachesUnifiedMessage(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	// Stage 1 — the listener decodes the webhook body and converts it.
	if err := h.routeEvent(context.Background(), "automod.message.hold", json.RawMessage(automodHoldPayload), ""); err != nil {
		t.Fatalf("listener failed to handle the AutoMod hold: %v", err)
	}
	rawMsg := firstStreamRawMessage(t, rc)

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

	if unified.Event == nil {
		t.Fatal("unified message carries no event info")
	}
	if unified.Event.Type != "mod_action" {
		t.Errorf("Event.Type = %q, want mod_action", unified.Event.Type)
	}
	// A moderation event must never produce an on-stream alert.
	if unified.Event.Duration != 0 {
		t.Errorf("Event.Duration = %d, want 0 so no OBS alert is shown", unified.Event.Duration)
	}
	if got := unified.Event.Metadata["held_text"]; got != "a message automod did not like" {
		t.Errorf("Metadata[held_text] = %v, want the original held text", got)
	}
	if got := unified.Event.Metadata["held_message_id"]; got != heldMessageID {
		t.Errorf("Metadata[held_message_id] = %v, want %q", got, heldMessageID)
	}
}
