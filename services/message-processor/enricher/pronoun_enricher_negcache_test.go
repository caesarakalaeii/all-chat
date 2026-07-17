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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// TestPronounEnricher_APIError_NegativeCachesShortTTL verifies that when the Alejo API
// errors (5xx / timeout), the enricher negative-caches the miss so it stops hammering a
// degraded upstream — the amplifier behind MessageProcessorStreamLag. Without this, every
// message from an uncached user makes a fresh (up to 3s) call that never caches, so a
// pronouns-API hiccup collapses processing throughput.
func TestPronounEnricher_APIError_NegativeCachesShortTTL(t *testing.T) {
	mr := miniredis.RunT(t)

	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/pronouns", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(pronounsMapJSON))
	})
	mux.HandleFunc("/users/erroruser", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := newTestPronounEnricher(t, mr, srv.URL)

	// First lookup: upstream 500 → must negative-cache rather than leave the key unset.
	if err := e.Enrich(context.Background(), makeTwitchMsg("erroruser")); err != nil {
		t.Fatalf("unexpected error on first lookup: %v", err)
	}
	// Second lookup within the negative-cache TTL: must be served from cache, NOT re-hit
	// the failing API.
	if err := e.Enrich(context.Background(), makeTwitchMsg("erroruser")); err != nil {
		t.Fatalf("unexpected error on second lookup: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 upstream call (negative cache must suppress the retry), got %d", got)
	}

	// The negative-cache entry must be short-lived so pronouns self-heal once the API
	// recovers — never the 24h happy-path TTL.
	cacheKey := PronounCacheKeyPrefix + "erroruser"
	ttl := mr.TTL(cacheKey)
	if ttl <= 0 || ttl > PronounErrorCacheTTL {
		t.Errorf("expected short negative-cache TTL in (0, %v], got %v", PronounErrorCacheTTL, ttl)
	}
}
