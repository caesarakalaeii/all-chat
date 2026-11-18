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
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// Synthetic messages (overlay-editor mocks and the public test stream) must
// still enrich with emotes but must not record emote-cache-operation or
// emote-lookup metrics: a short operator test burst is enough to tip
// AllChatEmoteCacheEfficiencyDeclining, which compares hit vs. miss rates over
// a 30m window against low baseline traffic. These tests pin that only real
// messages move the counters.

// promauto registers into the default registry, so the whole test binary
// shares one ProcessorMetrics; each test asserts on per-test deltas.
var (
	syntheticMetricsOnce sync.Once
	syntheticMetrics     *metrics.ProcessorMetrics
)

func sharedProcessorMetrics() *metrics.ProcessorMetrics {
	syntheticMetricsOnce.Do(func() {
		syntheticMetrics = metrics.NewProcessorMetrics()
	})
	return syntheticMetrics
}

type cacheOpDelta struct {
	hits, misses, lookups float64
}

func cacheOps(m *metrics.ProcessorMetrics) cacheOpDelta {
	return cacheOpDelta{
		hits:    testutil.ToFloat64(m.EmoteCacheOperations.WithLabelValues("message-processor", "hit", "all")),
		misses:  testutil.ToFloat64(m.EmoteCacheOperations.WithLabelValues("message-processor", "miss", "all")),
		lookups: testutil.ToFloat64(m.EmoteLookups.WithLabelValues("message-processor", "7tv", "hit")),
	}
}

func TestSyntheticMockMessageDoesNotRecordCacheMetrics(t *testing.T) {
	m := sharedProcessorMetrics()
	e := NewEnricher(&mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}},
	}, &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}, zap.NewNop())
	e.SetMetrics(m)

	before := cacheOps(m)
	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "mock-channel",
		User:      models.UserInfo{ID: "mock-user"},
		Message:   models.MessageInfo{Text: "KEKW"},
		Metadata:  map[string]interface{}{"mock": true},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	after := cacheOps(m)
	if after != before {
		t.Fatalf("mock message must not record cache metrics, delta hits=%v misses=%v lookups=%v",
			after.hits-before.hits, after.misses-before.misses, after.lookups-before.lookups)
	}
	if len(msg.Message.Emotes) != 1 {
		t.Fatalf("mock message must still enrich with emotes, got %#v", msg.Message.Emotes)
	}
}

func TestSyntheticTestStreamMessageDoesNotRecordCacheMetrics(t *testing.T) {
	m := sharedProcessorMetrics()
	e := NewEnricher(&mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}},
	}, &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}, zap.NewNop())
	e.SetMetrics(m)

	before := cacheOps(m)
	msg := &models.UnifiedChatMessage{
		Platform:  "youtube",
		ChannelID: "test-channel",
		User:      models.UserInfo{ID: "testgen-user"},
		Message:   models.MessageInfo{Text: "KEKW"},
		Metadata:  map[string]interface{}{"test_stream": true},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	after := cacheOps(m)
	if after != before {
		t.Fatalf("test stream message must not record cache metrics, delta hits=%v misses=%v lookups=%v",
			after.hits-before.hits, after.misses-before.misses, after.lookups-before.lookups)
	}
	if len(msg.Message.Emotes) != 1 {
		t.Fatalf("test stream message must still enrich with emotes, got %#v", msg.Message.Emotes)
	}
}

func TestRealMessageStillRecordsCacheMetrics(t *testing.T) {
	m := sharedProcessorMetrics()
	e := NewEnricher(&mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}},
	}, &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}, zap.NewNop())
	e.SetMetrics(m)

	before := cacheOps(m)
	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "caesarlp",
		User:      models.UserInfo{ID: "real-user"},
		Message:   models.MessageInfo{Text: "KEKW"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	after := cacheOps(m)
	if d := after.misses - before.misses; d != 1 {
		t.Fatalf("real message must record exactly one cache miss, delta %v", d)
	}
	if d := after.hits - before.hits; d != 0 {
		t.Fatalf("real message must not record cache hit on miss path, delta %v", d)
	}
	if d := after.lookups - before.lookups; d != 1 {
		t.Fatalf("real message must record exactly one provider lookup, delta %v", d)
	}
}

// TestSyntheticHitDoesNotRecordMetrics pins the cache-hit path: a synthetic
// message served from a warm cache must not record the hit either, or editor
// test bursts would inflate the hit side of the efficiency ratio.
func TestSyntheticHitDoesNotRecordMetrics(t *testing.T) {
	m := sharedProcessorMetrics()
	e := NewEnricher(&mockEmoteServiceClient{
		emotes: []EmoteServiceEmote{{Code: "LUL", Provider: "7tv", URL: "https://example"}},
	}, &mockEmoteCacheStore{
		getData: []cache.CachedEmote{{Code: "LUL", Provider: "7tv", URL: "https://example"}},
	}, zap.NewNop())
	e.SetMetrics(m)

	before := cacheOps(m)
	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "mock-channel",
		User:      models.UserInfo{ID: "mock-user"},
		Message:   models.MessageInfo{Text: "LUL"},
		Metadata:  map[string]interface{}{"mock": true},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	after := cacheOps(m)
	if after != before {
		t.Fatalf("synthetic cache hit must not record metrics, delta hits=%v misses=%v lookups=%v",
			after.hits-before.hits, after.misses-before.misses, after.lookups-before.lookups)
	}
	if len(msg.Message.Emotes) != 1 {
		t.Fatalf("synthetic cache hit must still enrich, got %#v", msg.Message.Emotes)
	}
}

// Synthetic users (overlay-editor mocks, and the test stream's random
// test-<n> IDs) have no personal emotes on any provider, so their lookups must
// share the channel-level cache entry. Keyed per user, every random ID cached
// its own copy of the full channel set: on 2026-11-15 that was ~12k entries,
// ~1.7 GB of the 2 GB Redis, evicting real users' cache.
func TestSyntheticMessagesUseChannelCacheKey(t *testing.T) {
	for _, meta := range []map[string]interface{}{
		{"test_stream": true, "twitch_channel_hint": "caesarlp"},
		{"mock": true},
	} {
		store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}
		client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}}}
		e := NewEnricher(client, store, zap.NewNop())

		msg := &models.UnifiedChatMessage{
			Platform:  "twitch",
			ChannelID: "caesarlp",
			User:      models.UserInfo{ID: "test-501299"},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  meta,
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("Enrich returned error: %v", err)
		}

		if store.userCalls != 0 {
			t.Errorf("%v: synthetic message used %d per-user cache calls, want 0", meta, store.userCalls)
		}
		if store.setCalls != 1 {
			t.Errorf("%v: synthetic message must still fill the channel cache, got %d sets", meta, store.setCalls)
		}
		if client.lastUserID != "" {
			t.Errorf("%v: synthetic message sent user %q to the emote service, want none", meta, client.lastUserID)
		}
		if len(msg.Message.Emotes) != 1 {
			t.Errorf("%v: synthetic message must still enrich, got %#v", meta, msg.Message.Emotes)
		}
	}
}

// Twitch and Kick chatters can carry personal (7TV) emotes, so they keep
// per-user keys.
//
// The channel entry is primed because a per-user entry is now the chatter's
// emotes minus the channel's, so the write needs a channel entry to split
// against (see splitPersonal).
func TestRealMessageUsesPerUserCacheKey(t *testing.T) {
	for _, platform := range []string{"twitch", "kick"} {
		store := &mockEmoteCacheStore{
			getErr:         cache.ErrCacheMiss,
			channelGetData: []cache.CachedEmote{{Code: "PogChamp", Provider: "twitch", URL: "https://example/pog"}},
		}
		client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}}}
		e := NewEnricher(client, store, zap.NewNop())

		msg := &models.UnifiedChatMessage{
			Platform:  platform,
			ChannelID: "caesarlp",
			User:      models.UserInfo{ID: "real-user"},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  map[string]interface{}{},
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("%s: Enrich returned error: %v", platform, err)
		}
		if store.userCalls != 2 || client.lastUserID != "real-user" {
			t.Errorf("%s: userCalls=%d lastUserID=%q, want 2 and %q", platform, store.userCalls, client.lastUserID, "real-user")
		}
	}
}

// 7TV cannot resolve a TikTok or YouTube chatter, so those share the channel
// key — see hasPersonalEmotes. Keyed per user, each stored a redundant copy of
// the channel set and filled Redis (AllChatRedisMemoryNearLimit, 2026-11-18).
func TestUnresolvableChatterSharesChannelCacheKey(t *testing.T) {
	for _, platform := range []string{"tiktok", "youtube"} {
		store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}
		client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}}}
		e := NewEnricher(client, store, zap.NewNop())

		msg := &models.UnifiedChatMessage{
			Platform:  platform,
			ChannelID: "bardeado_2001",
			User:      models.UserInfo{ID: "_delci0"},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  map[string]interface{}{},
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("%s: Enrich returned error: %v", platform, err)
		}

		if store.userCalls != 0 {
			t.Errorf("%s: chatter used %d per-user cache calls, want 0", platform, store.userCalls)
		}
		if store.setCalls != 1 {
			t.Errorf("%s: channel cache must still be filled, got %d sets", platform, store.setCalls)
		}
		if client.lastUserID != "" {
			t.Errorf("%s: sent user %q to the emote service, want none", platform, client.lastUserID)
		}
		if len(msg.Message.Emotes) != 1 {
			t.Errorf("%s: message must still enrich, got %#v", platform, msg.Message.Emotes)
		}
	}
}

// A per-user entry must hold only what is actually per-user: the chatter's own
// emotes. It used to hold the whole merged response, so every Twitch or Kick
// chatter stored a second copy of the channel set (384-1486 emotes, ~100-230 KB)
// for 13h — the family behind AllChatRedisEvictingKeys on 2026-11-18.
func TestPerUserCacheEntryExcludesChannelEmotes(t *testing.T) {
	channelEmotes := []cache.CachedEmote{
		{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		{Code: "PogChamp", Provider: "twitch", URL: "https://example/pog"},
	}
	// What emote-service returns for a user-aware lookup: the channel set with
	// the chatter's personal emotes merged in.
	merged := []EmoteServiceEmote{
		{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		{Code: "PogChamp", Provider: "twitch", URL: "https://example/pog"},
		{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"},
	}

	store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss, channelGetData: channelEmotes}
	client := &mockEmoteServiceClient{emotes: merged}
	e := NewEnricher(client, store, zap.NewNop())

	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "jtaz",
		User:      models.UserInfo{ID: "605199714"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if len(store.userSetData) != 1 || store.userSetData[0].Code != "MyPersonalEmote" {
		t.Errorf("per-user entry stored %d emotes (%#v), want only the chatter's own", len(store.userSetData), store.userSetData)
	}
	if len(msg.Message.Emotes) != 2 {
		t.Errorf("message must resolve both the channel and the personal emote, got %#v", msg.Message.Emotes)
	}
}

// A cached per-user entry holds only the chatter's own emotes, so serving it
// means putting it back together with the channel entry. Without the merge a
// returning chatter would lose every channel emote.
func TestPerUserCacheHitMergesChannelEmotes(t *testing.T) {
	store := &mockEmoteCacheStore{
		channelGetData: []cache.CachedEmote{
			{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		},
		getData: []cache.CachedEmote{
			{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"},
		},
	}
	client := &mockEmoteServiceClient{}
	e := NewEnricher(client, store, zap.NewNop())

	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "jtaz",
		User:      models.UserInfo{ID: "605199714"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if len(msg.Message.Emotes) != 2 {
		t.Errorf("cache hit resolved %d emotes (%#v), want the channel's and the chatter's", len(msg.Message.Emotes), msg.Message.Emotes)
	}
	if client.callCount() != 0 {
		t.Errorf("both halves were cached, but the emote service was called %d times", client.callCount())
	}
}

// Either half of a split entry can be evicted on its own — they carry separate
// TTLs. Serving the personal half alone would silently strip the channel's
// emotes from the message, so a half hit must count as a miss and re-fetch.
func TestPerUserCacheHitWithoutChannelEntryRefetches(t *testing.T) {
	store := &mockEmoteCacheStore{
		getErr: cache.ErrCacheMiss, // channel entry gone; GetEntryWithUser also misses
		getData: []cache.CachedEmote{
			{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"},
		},
	}
	client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{
		{Code: "KEKW", Provider: "7tv", URL: "https://example/kekw"},
		{Code: "MyPersonalEmote", Provider: "7tv", URL: "https://example/mine"},
	}}
	e := NewEnricher(client, store, zap.NewNop())

	msg := &models.UnifiedChatMessage{
		Platform:  "twitch",
		ChannelID: "jtaz",
		User:      models.UserInfo{ID: "605199714"},
		Message:   models.MessageInfo{Text: "KEKW MyPersonalEmote"},
		Metadata:  map[string]interface{}{},
	}
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if client.callCount() != 1 {
		t.Errorf("emote service called %d times, want 1 re-fetch", client.callCount())
	}
	if len(msg.Message.Emotes) != 2 {
		t.Errorf("re-fetch resolved %d emotes (%#v), want the channel's and the chatter's", len(msg.Message.Emotes), msg.Message.Emotes)
	}
}

// Dropping the chatter id for the cache key must not also drop the platform
// from the outbound request: GetEmotesForChannel carries no platform argument,
// so routing such a chatter down it degrades emote resolution silently rather
// than failing (see fetchFromService). Only the platforms that lose their user
// id can reach that route; the ones that keep it are already user-aware.
func TestUnresolvableChatterStillSendsPlatformToEmoteService(t *testing.T) {
	for _, platform := range []string{"tiktok", "youtube"} {
		store := &mockEmoteCacheStore{getErr: cache.ErrCacheMiss}
		client := &mockEmoteServiceClient{emotes: []EmoteServiceEmote{{Code: "KEKW", Provider: "7tv", URL: "https://example"}}}
		e := NewEnricher(client, store, zap.NewNop())

		msg := &models.UnifiedChatMessage{
			Platform:  platform,
			ChannelID: "bardeado_2001",
			User:      models.UserInfo{ID: "_delci0"},
			Message:   models.MessageInfo{Text: "KEKW"},
			Metadata:  map[string]interface{}{},
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("%s: Enrich returned error: %v", platform, err)
		}

		if client.plainCalls != 0 {
			t.Errorf("%s: used the platform-less channel route %d times, want 0", platform, client.plainCalls)
		}
		if client.lastPlatform != platform {
			t.Errorf("%s: emote service got platform %q, want %q", platform, client.lastPlatform, platform)
		}
	}
}
