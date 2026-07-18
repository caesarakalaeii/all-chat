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
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/google/uuid"
)

// TestGifWireSeam_EventSubToNormalizedAttachment exercises the whole Twitch chat-GIF path end
// to end across the service boundary (ADR-0037): a channel.chat.message "gif" fragment →
// buildChatTags → the exact RawChatMessage JSON published on the wire → message-processor
// normalization. It proves the GIF surfaces as a rendered attachment and the hidden alt
// caption is stripped, without either service reaching into the other's internals.
func TestGifWireSeam_EventSubToNormalizedAttachment(t *testing.T) {
	const gifURL = "https://media4.giphy.com/media/joSNxeswxuc74Juo8X/giphy.gif?cid=abc&ct=g"

	event := &eventsub.ChatMessageEvent{
		BroadcasterUserLogin: "Streamer",
		BroadcasterUserID:    "42",
		ChatterUserID:        "1001",
		ChatterUserLogin:     "Viewer",
		ChatterUserName:      "Viewer",
		MessageID:            "native-msg-1",
		Message: eventsub.ChatMessageBody{
			Text: "[Y A Y Yes GIF by Djemilah Birnie]",
			Fragments: []eventsub.ChatMessageFragment{
				{Type: "gif", Text: "[Y A Y Yes GIF by Djemilah Birnie]", Gif: &eventsub.ChatGif{GifID: "joSNxeswxuc74Juo8X", URL: gifURL}},
			},
		},
	}

	// Producer side: shape the RawChatMessage exactly as handleChatMessage does.
	raw := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: "streamer",
		UserID:    event.ChatterUserID,
		Username:  "viewer",
		Text:      event.Message.Text,
		Timestamp: time.Now().UTC(),
		Tags:      buildChatTags(event),
		EventType: "",
	}

	// Wire hop: marshal on the producer, unmarshal on the consumer.
	wire, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal RawChatMessage: %v", err)
	}
	consumed, err := mpmodels.ParseRawMessage(wire)
	if err != nil {
		t.Fatalf("parse wire RawChatMessage: %v", err)
	}

	// Consumer side: normalize.
	msg, err := normalizer.NewTwitchNormalizer().Normalize(consumed, "overlay-1")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if len(msg.Message.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Message.Attachments))
	}
	att := msg.Message.Attachments[0]
	if att.Type != "image" || att.ContentType != "image/gif" {
		t.Fatalf("attachment type/content_type = %q/%q, want image/image/gif", att.Type, att.ContentType)
	}
	if att.URL != gifURL {
		t.Fatalf("attachment URL = %q, want %q", att.URL, gifURL)
	}
	if att.Filename != "Y A Y Yes GIF by Djemilah Birnie" {
		t.Fatalf("attachment Filename (alt) = %q, want caption without brackets", att.Filename)
	}
	if msg.Message.Text != "" {
		t.Fatalf("message text = %q, want empty (caption stripped)", msg.Message.Text)
	}
}
