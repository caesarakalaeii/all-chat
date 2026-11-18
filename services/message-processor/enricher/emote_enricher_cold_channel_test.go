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
	"errors"
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
// The shared mockEmoteCacheStore cannot express this: its GetEntry serves
// channelGetData regardless of whether anything was ever written, so a test
// using it always starts from a channel entry that exists — the one precondition
// the cold-channel case does not have.
type twoKeyCacheStore struct {
	mu      sync.Mutex
	channel []cache.CachedEmote
	perUser map[string][]cache.CachedEmote
	// channelWrites counts Set calls. A rewrite that happens to store the same
	// emotes still resets the key's TTL in Redis, so contents alone cannot show
	// whether the channel entry was left alone.
	channelWrites int
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
	s.channelWrites++
	return nil
}

func (s *twoKeyCacheStore) channelWriteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channelWrites
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
	return nil
}

func (s *twoKeyCacheStore) DeletePattern(ctx context.Context, pattern string) error { return nil }

// failingChannelReadStore serves the channel entry but fails every read of it
// with a transport error, the way a Redis blip does. The entry is still there;
// only the read of it failed.
type failingChannelReadStore struct {
	*twoKeyCacheStore
	readErr error
}

func (s *failingChannelReadStore) GetEntry(ctx context.Context, channel string) (cache.Entry, error) {
	return cache.Entry{}, s.readErr
}

// TestChannelEntryIsNotReseededAfterATransientReadError pins which GetEntry
// failures mean "no channel entry yet".
//
// Only ErrCacheMiss does. A transport or decode error says nothing about
// whether the entry exists, and on a blip it does: overwriting it there
// republishes a channel set from a user-aware response and resets the entry's
// freshness window, on a channel that was already warm.
func TestChannelEntryIsNotReseededAfterATransientReadError(t *testing.T) {
	inner := newTwoKeyCacheStore()
	inner.channel = []cache.CachedEmote{
		{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
	}
	store := &failingChannelReadStore{
		twoKeyCacheStore: inner,
		readErr:          errors.New("dial tcp 10.0.0.1:6379: connect: connection refused"),
	}
	client := &userAwareEmoteClient{
		channel: []EmoteServiceEmote{
			{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		},
		personal: map[string][]EmoteServiceEmote{
			"chatter-one": {{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"}},
		},
	}
	e := NewEnricher(client, store, zap.NewNop())

	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "caesarlp",
		User:      models.UserInfo{ID: "chatter-one"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	// Counted, not compared: a reseed would write the same channel-only emotes
	// this entry already holds, but it would also reset the key's TTL on a
	// channel that was already warm. Only ErrCacheMiss means the entry is absent.
	if writes := inner.channelWriteCount(); writes != 0 {
		t.Fatalf("a failed channel read rewrote the channel entry %d time(s); "+
			"a transport error says nothing about whether the entry exists", writes)
	}

	// The message is still served: the failure skips the cache write, not the
	// enrichment.
	if len(msg.Message.Emotes) != 2 {
		t.Fatalf("resolved %d emotes (%#v), want the channel's and the chatter's",
			len(msg.Message.Emotes), msg.Message.Emotes)
	}
}

func (s *twoKeyCacheStore) channelEntry() []cache.CachedEmote {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channel
}

// userAwareEmoteClient models the one way the real emote service differs
// between its two routes: a request carrying a user id gets that chatter's
// personal 7TV set merged into the response (FetchCombinedEmotes in
// services/emote-service/clients/seventv.go), a request without one does not.
//
// mockEmoteServiceClient returns the same slice either way, so it cannot tell a
// channel-only response apart from a user-aware one — which is the distinction
// the channel key's contents depend on.
type userAwareEmoteClient struct {
	mu       sync.Mutex
	channel  []EmoteServiceEmote
	personal map[string][]EmoteServiceEmote
	calls    int
	userCall int
}

func (c *userAwareEmoteClient) GetEmotesForChannel(ctx context.Context, channel string) ([]EmoteServiceEmote, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.channel, nil
}

func (c *userAwareEmoteClient) GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]EmoteServiceEmote, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if userID == "" {
		return c.channel, nil
	}
	c.userCall++
	merged := make([]EmoteServiceEmote, 0, len(c.channel)+len(c.personal[userID]))
	merged = append(merged, c.channel...)
	merged = append(merged, c.personal[userID]...)
	return merged, nil
}

func (c *userAwareEmoteClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// TestChannelSeedExcludesTheSeedingChattersOwnEmotes pins what may be written to
// the SHARED channel key.
//
// The seed runs on a cold channel whose lookups all carry a user id, and the
// response to such a lookup has that chatter's personal 7TV set merged in. Had
// it been stored as-is, the first chatter to speak would publish their own
// emotes as channel emotes to every other viewer for the channel entry's whole
// lifetime (softTTL + stale grace, up to 18h).
func TestChannelSeedExcludesTheSeedingChattersOwnEmotes(t *testing.T) {
	store := newTwoKeyCacheStore()
	client := &userAwareEmoteClient{
		channel: []EmoteServiceEmote{
			{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		},
		personal: map[string][]EmoteServiceEmote{
			"chatter-one": {{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"}},
		},
	}
	e := NewEnricher(client, store, zap.NewNop())

	first := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "caesarlp",
		User:      models.UserInfo{ID: "chatter-one"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), first); err != nil {
		t.Fatalf("first Enrich returned error: %v", err)
	}

	for _, emote := range store.channelEntry() {
		if emote.Code == "MyPersonalEmote" {
			t.Fatalf("the channel entry carries the seeding chatter's personal emote %q; "+
				"every other viewer of this channel would be served it as a channel emote", emote.Code)
		}
	}

	// The seeding chatter must still see their own emote on the message that
	// triggered the seed: it is served from the upstream response, not the cache.
	if len(first.Message.Emotes) != 2 {
		t.Fatalf("seeding chatter resolved %d emotes (%#v), want the channel's and their own",
			len(first.Message.Emotes), first.Message.Emotes)
	}

	// A second chatter reads the channel entry the first one seeded.
	second := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "caesarlp",
		User:      models.UserInfo{ID: "chatter-two"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), second); err != nil {
		t.Fatalf("second Enrich returned error: %v", err)
	}
	for _, emote := range second.Message.Emotes {
		if emote.Code == "MyPersonalEmote" {
			t.Fatalf("chatter-two was served chatter-one's personal emote %q", emote.Code)
		}
	}
}

// TestTwitchOnlyChannelCachesAfterFirstMessage pins the cold-start path for a
// channel whose chatters are all on a platform with personal emotes.
//
// Every such lookup carries a user id, and the per-user write needs a channel
// entry to subtract from. Nothing else writes that entry: the channel-level
// Set runs only on the userID == "" branch, which a Twitch-only channel never
// takes. Without the seed, the channel re-fetches from emote-service on every
// message forever.
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
