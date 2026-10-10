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

package streams

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// A claimed channel is held away from innertube, so every gate in front of discovery bounds how
// late a go-live is noticed. Discovery costs one liveBroadcasts.list unit, so none may exceed a minute.
func TestDetectionBackoff_CappedAtOneMinute(t *testing.T) {
	t.Setenv("CIRCUIT_BREAKER_OPEN_DURATION_MINUTES", "")
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	m := NewManager(nil, nil, nil, nil, nil, nil, nil, rc, nil, nil, nil, func() bool { return false }, zap.NewNop())
	ctx := context.Background()

	for range 10 {
		m.updateDetectionBackoff("UCoffline")
		m.increaseDetectionBackoff("UCerroring")
	}

	for _, channelID := range []string{"UCoffline", "UCerroring"} {
		state, err := m.backoffStore.LoadBackoffState(ctx, channelID)
		if err != nil || state == nil {
			t.Fatalf("%s: backoff state = %v, %v", channelID, state, err)
		}
		if state.CurrentInterval > time.Minute {
			t.Errorf("%s: detection backoff = %v, want <= 1m", channelID, state.CurrentInterval)
		}
	}
	if !mr.Exists("youtube:negative:UCoffline") {
		t.Fatal("negative cache key youtube:negative:UCoffline not set; its TTL cannot be checked")
	}
	if ttl := mr.TTL("youtube:negative:UCoffline"); ttl <= 0 || ttl > time.Minute {
		t.Errorf("negative cache TTL = %v, want in (0, 1m]", ttl)
	}

	breaker := m.getOrCreateCircuitBreaker("UCoffline")
	for range 10 {
		breaker.RecordFailure()
	}
	if breaker.openDuration > time.Minute {
		t.Errorf("circuit breaker open window = %v, want <= 1m", breaker.openDuration)
	}

	// An env override must not bring back a long blind window, nor disable the breaker's wait.
	for _, minutes := range []string{"10", "0", "-5"} {
		t.Setenv("CIRCUIT_BREAKER_OPEN_DURATION_MINUTES", minutes)
		if got := NewCircuitBreaker("UCx", zap.NewNop(), nil).openDuration; got != maxDetectionBackoff {
			t.Errorf("CIRCUIT_BREAKER_OPEN_DURATION_MINUTES=%s: openDuration = %v, want %v", minutes, got, maxDetectionBackoff)
		}
	}
}

func TestCircuitBreakerStats_QuotaSavedAtDiscoveryCost(t *testing.T) {
	breaker := NewCircuitBreaker("UCx", zap.NewNop(), nil)
	for range 3 {
		breaker.RecordFailure()
	}
	if got, want := breaker.GetStats()["quota_saved"], 3*quota.QuotaCostLiveBroadcasts; got != want {
		t.Errorf("quota_saved = %v, want %d (one liveBroadcasts.list per offline result)", got, want)
	}
}
