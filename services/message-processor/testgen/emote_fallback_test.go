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

package testgen

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// fakeEmoteClient records which channel the enricher queried so we can assert
// the test stream resolves emotes against the real fallback channel rather than
// the synthetic test channel (which doesn't exist on any emote provider).
type fakeEmoteClient struct {
	channels       []string
	twitchChannels []string
}

func (f *fakeEmoteClient) GetEmotesForChannel(_ context.Context, channel string) ([]enricher.EmoteServiceEmote, error) {
	f.channels = append(f.channels, channel)
	return []enricher.EmoteServiceEmote{{Code: "PogChamp", URL: "https://e/PogChamp", Provider: "twitch"}}, nil
}

func (f *fakeEmoteClient) GetEmotesForChannelWithUser(_ context.Context, channel, _, _, twitchChannel, _ string) ([]enricher.EmoteServiceEmote, error) {
	f.channels = append(f.channels, channel)
	f.twitchChannels = append(f.twitchChannels, twitchChannel)
	return []enricher.EmoteServiceEmote{{Code: "PogChamp", URL: "https://e/PogChamp", Provider: "twitch"}}, nil
}

// TestEnrichEmotesUsesFallbackChannel verifies that the test-stream generator
// resolves emotes against the real fallback channel (caesarlp) while leaving the
// message's own synthetic channel identity untouched for external tools.
func TestEnrichEmotesUsesFallbackChannel(t *testing.T) {
	fc := &fakeEmoteClient{}
	em := enricher.NewEnricher(fc, nil, zap.NewNop())
	g := NewGenerator(testOverlayID, nil, em, nil, zap.NewNop())

	msg := &models.UnifiedChatMessage{
		OverlayID:   testOverlayID,
		Platform:    "youtube", // non-Twitch: exercises the twitch_channel_hint path too
		ChannelID:   testChannelID,
		ChannelName: testChannelName,
		User:        models.UserInfo{ID: "test-123"},
		Message:     models.MessageInfo{Text: "that was insane PogChamp"},
		Metadata:    map[string]interface{}{"test_stream": true},
	}

	g.enrichEmotes(context.Background(), msg)

	if len(fc.channels) == 0 {
		t.Fatal("emote client was never queried")
	}
	for _, ch := range fc.channels {
		if ch != emoteFallbackChannel {
			t.Fatalf("emote lookup channel = %q, want %q", ch, emoteFallbackChannel)
		}
	}
	for _, tc := range fc.twitchChannels {
		if tc != emoteFallbackChannel {
			t.Fatalf("twitch_channel hint = %q, want %q", tc, emoteFallbackChannel)
		}
	}

	// The message keeps its own channel identity after enrichment.
	if msg.ChannelID != testChannelID {
		t.Fatalf("ChannelID mutated to %q, want %q", msg.ChannelID, testChannelID)
	}
	if msg.ChannelName != testChannelName {
		t.Fatalf("ChannelName mutated to %q, want %q", msg.ChannelName, testChannelName)
	}
	// The fallback hint must not leak into the published payload.
	if _, ok := msg.Metadata["twitch_channel_hint"]; ok {
		t.Fatal("twitch_channel_hint should be cleaned up after enrichment")
	}

	// The fallback channel's emote was actually attached.
	if len(msg.Message.Emotes) != 1 || msg.Message.Emotes[0].Code != "PogChamp" {
		t.Fatalf("expected PogChamp emote attached, got %+v", msg.Message.Emotes)
	}
}
