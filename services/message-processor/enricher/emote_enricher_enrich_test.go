package enricher

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

type mockEmoteServiceClient struct {
	emotes      []EmoteServiceEmote
	err         error
	lastChannel string
}

func (m *mockEmoteServiceClient) GetEmotesForChannel(ctx context.Context, channelID string) ([]EmoteServiceEmote, error) {
	m.lastChannel = channelID
	if m.err != nil {
		return nil, m.err
	}
	return m.emotes, nil
}

func TestEnrichAddsEmotesForLaterOccurrences(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{
			{Code: "OMEGALUL", Provider: "7tv", URL: "https://cdn.7tv.app/emote/123/1x.webp"},
		},
	}

	enricher := NewEnricher(client, zap.NewNop())
	msg := &models.UnifiedChatMessage{
		ChannelID: "channel-123",
		Message: models.MessageInfo{
			Text:   "hello OMEGALUL there OMEGALUL again",
			Emotes: []models.Emote{},
		},
	}

	if err := enricher.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if len(msg.Message.Emotes) != 2 {
		t.Fatalf("expected 2 emotes to be added, got %d", len(msg.Message.Emotes))
	}

	for _, emote := range msg.Message.Emotes {
		if emote.Code != "OMEGALUL" {
			t.Fatalf("unexpected emote code %s", emote.Code)
		}
		if len(emote.Positions) != 1 {
			t.Fatalf("expected single position per emote, got %#v", emote.Positions)
		}
	}

	firstPos := msg.Message.Emotes[0].Positions[0][0]
	secondPos := msg.Message.Emotes[1].Positions[0][0]
	if !(firstPos < secondPos) {
		t.Fatalf("expected emote positions to increase, got %d then %d", firstPos, secondPos)
	}
}

func TestEnrichPrefersTwitchRoomID(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{
			{Code: "KEKW", Provider: "bttv", URL: "https://cdn.betterttv.net/emote/1234/1x"},
		},
	}

	enricher := NewEnricher(client, zap.NewNop())
	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "channel-login",
		Metadata: map[string]interface{}{
			"twitch_room_id": "67241623",
		},
		Message: models.MessageInfo{Text: "KEKW"},
	}

	if err := enricher.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if client.lastChannel != "67241623" {
		t.Fatalf("expected enricher to request emotes with room id, got %q", client.lastChannel)
	}
}
