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
