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
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

type mockEmoteServiceClient struct {
	mu          sync.Mutex
	emotes      []EmoteServiceEmote
	err         error
	lastChannel string
	calls       int
}

func (m *mockEmoteServiceClient) GetEmotesForChannel(ctx context.Context, channelID string) ([]EmoteServiceEmote, error) {
	m.mu.Lock()
	m.lastChannel = channelID
	m.calls++
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.emotes, nil
}

func (m *mockEmoteServiceClient) GetEmotesForChannelWithUser(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]EmoteServiceEmote, error) {
	m.mu.Lock()
	m.lastChannel = channel
	m.calls++
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.emotes, nil
}

func (m *mockEmoteServiceClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type mockEmoteCacheStore struct {
	mu       sync.Mutex
	getData  []cache.CachedEmote
	getErr   error
	getStale bool
	setCalls int
	setData  []cache.CachedEmote
}

func (m *mockEmoteCacheStore) GetEntry(ctx context.Context, channel string) (cache.Entry, error) {
	if m.getErr != nil {
		return cache.Entry{}, m.getErr
	}
	return cache.Entry{Emotes: m.getData, Stale: m.getStale}, nil
}

func (m *mockEmoteCacheStore) Set(ctx context.Context, channel string, emotes []cache.CachedEmote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls++
	m.setData = emotes
	return nil
}

func (m *mockEmoteCacheStore) Delete(ctx context.Context, channel string) error {
	return nil
}

func (m *mockEmoteCacheStore) GetEntryWithUser(ctx context.Context, channel, userID string) (cache.Entry, error) {
	if m.getErr != nil {
		return cache.Entry{}, m.getErr
	}
	return cache.Entry{Emotes: m.getData, Stale: m.getStale}, nil
}

func (m *mockEmoteCacheStore) SetWithUser(ctx context.Context, channel, userID string, emotes []cache.CachedEmote) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls++
	m.setData = emotes
	return nil
}

func (m *mockEmoteCacheStore) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

func (m *mockEmoteCacheStore) setCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setCalls
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
	if client.callCount() != 0 {
		t.Fatalf("expected cache hit to skip client call, got %d", client.callCount())
	}
	if store.setCallCount() != 0 {
		t.Fatalf("expected cache hit to skip set, got %d", store.setCallCount())
	}
}

// TestFetchEmotesDoesNotCacheEmptyResult guards against a transient/cold empty
// response from the emote service poisoning the cache. The stale-while-revalidate
// envelope keeps entries alive for softTTL + 12h grace and serves them through
// their freshness window without revalidating, so caching an empty result here
// would suppress emotes for that channel/user for hours after the upstream
// recovered (the "no emotes render in the preview" regression). Empty results
// must be left uncached so the next message re-fetches.
func TestFetchEmotesDoesNotCacheEmptyResult(t *testing.T) {
	client := &mockEmoteServiceClient{emotes: nil} // upstream returns no emotes
	store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}

	enricher := NewEnricher(client, store, zap.NewNop())

	// Channel-level lookup (no user id).
	got, err := enricher.fetchEmotes(context.Background(), "caesarlp", "twitch", "", "", "")
	if err != nil {
		t.Fatalf("fetchEmotes returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %#v", got)
	}
	if store.setCallCount() != 0 {
		t.Fatalf("empty result must not be cached, got %d set calls", store.setCallCount())
	}

	// User-specific lookup (the mock-message path that triggered the regression).
	if _, err := enricher.fetchEmotes(context.Background(), "caesarlp", "twitch", "mock-user", "", ""); err != nil {
		t.Fatalf("fetchEmotes (with user) returned error: %v", err)
	}
	if store.setCallCount() != 0 {
		t.Fatalf("empty user-specific result must not be cached, got %d set calls", store.setCallCount())
	}
}

func TestFetchEmotesStaleServesImmediatelyAndRefreshes(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example/fresh"}},
	}
	store := &mockEmoteCacheStore{
		getData:  []cache.CachedEmote{{Code: "LUL", Provider: "twitch", URL: "https://example/stale"}},
		getStale: true,
	}

	enricher := NewEnricher(client, store, zap.NewNop())
	got, err := enricher.fetchEmotes(context.Background(), "123", "twitch", "", "", "")
	if err != nil {
		t.Fatalf("fetchEmotes returned error: %v", err)
	}

	// The stale entry is served immediately, without blocking on the client.
	if len(got) != 1 || got[0].Code != "LUL" {
		t.Fatalf("expected stale entry served immediately, got %#v", got)
	}

	// A background refresh should re-fetch from the service and repopulate the cache.
	if !waitFor(func() bool { return client.callCount() == 1 && store.setCallCount() == 1 }) {
		t.Fatalf("expected one background refresh fetch+set, got calls=%d sets=%d",
			client.callCount(), store.setCallCount())
	}
}

func TestFetchEmotesStaleRefreshIsRateLimited(t *testing.T) {
	client := &mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example/fresh"}},
	}
	store := &mockEmoteCacheStore{
		getData:  []cache.CachedEmote{{Code: "LUL", Provider: "twitch", URL: "https://example/stale"}},
		getStale: true,
	}

	enricher := NewEnricher(client, store, zap.NewNop())

	// Mark a refresh as already in flight for this key; a concurrent stale read
	// must not kick off a second one.
	enricher.refreshing.Store("123\x00", struct{}{})

	if _, err := enricher.fetchEmotes(context.Background(), "123", "twitch", "", "", ""); err != nil {
		t.Fatalf("fetchEmotes returned error: %v", err)
	}

	// Give any (incorrectly) spawned goroutine a chance to run.
	time.Sleep(50 * time.Millisecond)
	if client.callCount() != 0 {
		t.Fatalf("expected in-flight refresh to suppress duplicate fetch, got %d", client.callCount())
	}
}

// waitFor polls cond for up to a second, returning true once it holds.
func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
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
	if client.callCount() != 1 {
		t.Fatalf("expected single client call, got %d", client.callCount())
	}
	if store.setCallCount() != 1 {
		t.Fatalf("expected cache set to be invoked once, got %d", store.setCallCount())
	}
	if len(store.setData) != 1 || store.setData[0].Code != "KEKW" {
		t.Fatalf("cache stored wrong data: %#v", store.setData)
	}
}
