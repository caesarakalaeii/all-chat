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

package enricher

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

type mockEmoteServiceClient struct {
	emotes      []EmoteServiceEmote
	err         error
	lastChannel string
	calls       int
}

func (m *mockEmoteServiceClient) GetEmotesForChannel(ctx context.Context, channelID string) ([]EmoteServiceEmote, error) {
	m.lastChannel = channelID
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.emotes, nil
}

func (m *mockEmoteServiceClient) GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]EmoteServiceEmote, error) {
	m.lastChannel = channel
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.emotes, nil
}

type mockEmoteCacheStore struct {
	getData  []cache.CachedEmote
	getErr   error
	setCalls int
	setData  []cache.CachedEmote
}

func (m *mockEmoteCacheStore) Get(ctx context.Context, channel string) ([]cache.CachedEmote, error) {
	return m.getData, m.getErr
}

func (m *mockEmoteCacheStore) Set(ctx context.Context, channel string, emotes []cache.CachedEmote) error {
	m.setCalls++
	m.setData = emotes
	return nil
}

func (m *mockEmoteCacheStore) Delete(ctx context.Context, channel string) error {
	return nil
}

func (m *mockEmoteCacheStore) GetWithUser(ctx context.Context, channel, userID string) ([]cache.CachedEmote, error) {
	return m.getData, m.getErr
}

func (m *mockEmoteCacheStore) SetWithUser(ctx context.Context, channel, userID string, emotes []cache.CachedEmote) error {
	m.setCalls++
	m.setData = emotes
	return nil
}

func (m *mockEmoteCacheStore) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

func TestEnrichAddsEmotesForLaterOccurrences(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{
			{Code: "OMEGALUL", Provider: "7tv", URL: "https://cdn.7tv.app/emote/123/1x.webp"},
		},
	}

	enricher := NewEnricher(client, nil, zap.NewNop())
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

	enricher := NewEnricher(client, nil, zap.NewNop())
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

func TestFetchEmotesUsesCache(t *testing.T) {
	client := &mockEmoteServiceClient{}
	store := &mockEmoteCacheStore{
		getData: []cache.CachedEmote{{Code: "LUL", Provider: "twitch", URL: "https://example"}},
	}

	enricher := NewEnricher(client, store, zap.NewNop())
	got, err := enricher.fetchEmotes(context.Background(), "123", "twitch", "", "", "")
	if err != nil {
		t.Fatalf("fetchEmotes returned error: %v", err)
	}
	if len(got) != 1 || got[0].Code != "LUL" {
		t.Fatalf("unexpected cached result: %#v", got)
	}
	if client.calls != 0 {
		t.Fatalf("expected cache hit to skip client call, got %d", client.calls)
	}
	if store.setCalls != 0 {
		t.Fatalf("expected cache hit to skip set, got %d", store.setCalls)
	}
}

func TestFetchEmotesCachesAfterMiss(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "twitch", URL: "https://example"}},
	}
	store := &mockEmoteCacheStore{
		getErr: cache.ErrCacheMiss,
	}

	enricher := NewEnricher(client, store, zap.NewNop())
	got, err := enricher.fetchEmotes(context.Background(), "123", "twitch", "", "", "")
	if err != nil {
		t.Fatalf("fetchEmotes returned error: %v", err)
	}
	if len(got) != 1 || got[0].Code != "KEKW" {
		t.Fatalf("unexpected fetched result: %#v", got)
	}
	if client.calls != 1 {
		t.Fatalf("expected single client call, got %d", client.calls)
	}
	if store.setCalls != 1 {
		t.Fatalf("expected cache set to be invoked once, got %d", store.setCalls)
	}
	if len(store.setData) != 1 || store.setData[0].Code != "KEKW" {
		t.Fatalf("cache stored wrong data: %#v", store.setData)
	}
}
