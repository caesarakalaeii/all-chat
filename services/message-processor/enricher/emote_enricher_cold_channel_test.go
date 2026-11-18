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
	"sync"
	"testing"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// twoKeyCacheStore is a fake of the real Redis-backed store: it keeps the
// channel entry and each per-user entry under genuinely separate keys, so a
// read only hits what some earlier write actually put there.
//
// The shared mockEmoteCacheStore cannot express this. Its GetEntry serves
// channelGetData regardless of whether anything was ever written, so a test
// using it always starts from a channel entry that exists — which is exactly
// the precondition the cold-channel case does NOT have, and therefore exactly
// the bug it cannot catch.
type twoKeyCacheStore struct {
	mu        sync.Mutex
	channel   []cache.CachedEmote
	perUser   map[string][]cache.CachedEmote
	userWrite int
}

func newTwoKeyCacheStore() *twoKeyCacheStore {
	return &twoKeyCacheStore{perUser: map[string][]cache.CachedEmote{}}
}

func (s *twoKeyCacheStore) GetEntry(ctx context.Context, channel string) (cache.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.channel == nil {
		return cache.Entry{}, cache.ErrCacheMiss
	}
	return cache.Entry{Emotes: s.channel}, nil
}

func (s *twoKeyCacheStore) Set(ctx context.Context, channel string, emotes []cache.CachedEmote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channel = emotes
	return nil
}

func (s *twoKeyCacheStore) Delete(ctx context.Context, channel string) error { return nil }

func (s *twoKeyCacheStore) GetEntryWithUser(ctx context.Context, channel, userID string) (cache.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emotes, ok := s.perUser[userID]
	if !ok {
		return cache.Entry{}, cache.ErrCacheMiss
	}
	return cache.Entry{Emotes: emotes}, nil
}

func (s *twoKeyCacheStore) SetWithUser(ctx context.Context, channel, userID string, emotes []cache.CachedEmote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if emotes == nil {
		emotes = []cache.CachedEmote{}
	}
	s.perUser[userID] = emotes
	s.userWrite++
	return nil
}

func (s *twoKeyCacheStore) DeletePattern(ctx context.Context, pattern string) error { return nil }

func (s *twoKeyCacheStore) channelEntry() []cache.CachedEmote {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channel
}

// TestTwitchOnlyChannelCachesAfterFirstMessage pins the cold-start path for a
// channel whose chatters are all on a platform with personal emotes.
//
// Every such lookup carries a user id, and the per-user write needs a channel
// entry to subtract from. Nothing else writes that entry: the channel-level
// Set runs only on the userID == "" branch, which a Twitch-only channel never
// takes. So without the channel entry being seeded here, the first message
// caches nothing, the second finds nothing, and the channel re-fetches from
// emote-service on every message forever — a 3s upstream call per message in
// place of a cache.
func TestTwitchOnlyChannelCachesAfterFirstMessage(t *testing.T) {
	store := newTwoKeyCacheStore()
	client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{
		{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		{Code: "PogChamp", Provider: "twitch", URL: "https://example/pog"},
	}}
	e := NewEnricher(client, store, zap.NewNop())

	message := func(userID string) *models.UnifiedChatMessage {
		return &models.UnifiedChatMessage{
			Platform:  "twitch",
			ChannelID: "caesarlp",
			User:      models.UserInfo{ID: userID},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  map[string]interface{}{},
		}
	}

	if err := e.Enrich(context.Background(), message("chatter-one")); err != nil {
		t.Fatalf("first Enrich returned error: %v", err)
	}

	if store.channelEntry() == nil {
		t.Fatal("first message on a Twitch-only channel must write the channel emote entry; " +
			"without it every later per-user write is skipped and the channel never caches")
	}

	callsAfterFirst := client.callCount()

	second := message("chatter-two")
	if err := e.Enrich(context.Background(), second); err != nil {
		t.Fatalf("second Enrich returned error: %v", err)
	}
	if len(second.Message.Emotes) != 1 {
		t.Fatalf("second message must still enrich, got %#v", second.Message.Emotes)
	}

	third := message("chatter-two")
	if err := e.Enrich(context.Background(), third); err != nil {
		t.Fatalf("third Enrich returned error: %v", err)
	}
	if len(third.Message.Emotes) != 1 {
		t.Fatalf("third message must still enrich, got %#v", third.Message.Emotes)
	}

	// chatter-two's second message is a full two-key hit, so it must not reach
	// emote-service at all.
	if extra := client.callCount() - callsAfterFirst; extra != 1 {
		t.Fatalf("a repeat chatter must be served from cache: expected 1 upstream fetch "+
			"across the two messages, got %d", extra)
	}
}
