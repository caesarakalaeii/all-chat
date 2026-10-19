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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/sessions"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TestAvatarEnricher_ConcurrentEnrichNoTokenRace drives many avatar lookups
// concurrently through the token-refresh path (starting with an empty token so
// multiple goroutines refresh at once). With -race this fails if accessToken is read
// or written without the mu-guarded accessors — the data race that batch concurrency
// (ADR-0033) would otherwise expose.
func TestAvatarEnricher_ConcurrentEnrichNoTokenRace(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/token":
			fmt.Fprint(w, `{"access_token":"app-token-xyz","expires_in":3600,"token_type":"bearer"}`)
		case "/helix/users":
			id := r.URL.Query().Get("id")
			fmt.Fprintf(w, `{"data":[{"id":%q,"login":"u","display_name":"U","profile_image_url":"https://cdn/%s.png"}]}`, id, id)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	e := NewAvatarEnricher(rc, "client-id", "client-secret", zap.NewNop())
	e.tokenURL = srv.URL + "/oauth2/token"
	e.helixUsersURL = srv.URL + "/helix/users"

	const goroutines = 24
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &models.UnifiedChatMessage{
				Platform: "twitch",
				User:     models.UserInfo{ID: fmt.Sprintf("user-%d", idx)},
			}
			errs[idx] = e.Enrich(context.Background(), msg)
			if msg.User.AvatarURL == "" {
				errs[idx] = fmt.Errorf("goroutine %d: avatar not set", idx)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	if got := e.token(); got != "app-token-xyz" {
		t.Errorf("expected token to be set after refresh, got %q", got)
	}
}

// TestAvatarEnricher_TikTokProxyURL pins the two properties the credit roll depends
// on: the rewritten URL must be loadable by a browser on the frontend's origin, and
// the cached bytes behind it must live as long as the leaderboard that stores it.
func TestAvatarEnricher_TikTokProxyURL(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/avatar.webp" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("RIFF....WEBPVP8 "))
	}))
	defer cdn.Close()

	e := NewAvatarEnricher(rc, "", "", zap.NewNop())
	enrich := func(userID, avatarURL string) string {
		msg := &models.UnifiedChatMessage{
			Platform: "tiktok",
			User:     models.UserInfo{ID: userID, AvatarURL: avatarURL},
		}
		if err := e.Enrich(context.Background(), msg); err != nil {
			t.Fatalf("enrich %s: %v", userID, err)
		}
		return msg.User.AvatarURL
	}

	if got := enrich("7001", cdn.URL+"/avatar.webp"); got != "/api/avatars/tiktok/7001" {
		t.Errorf("cached avatar URL = %q, want root-relative proxy path", got)
	}
	if ttl := mr.TTL(AvatarImageCacheKeyPrefix + "tiktok:7001"); ttl < sessions.LeaderboardTTL {
		t.Errorf("image TTL %v shorter than leaderboard TTL %v", ttl, sessions.LeaderboardTTL)
	}

	// A hit renews the TTL instead of letting the first message's clock run out.
	mr.FastForward(sessions.LeaderboardTTL - time.Hour)
	if got := enrich("7001", cdn.URL+"/gone.webp"); got != "/api/avatars/tiktok/7001" {
		t.Errorf("cache hit URL = %q", got)
	}
	if ttl := mr.TTL(AvatarImageCacheKeyPrefix + "tiktok:7001"); ttl < sessions.LeaderboardTTL {
		t.Errorf("TTL not renewed on hit: %v", ttl)
	}

	// Fetch failure keeps the CDN URL so the frontend can still try it, then fall back.
	if got := enrich("7002", cdn.URL+"/gone.webp"); got != cdn.URL+"/gone.webp" {
		t.Errorf("uncached avatar URL = %q, want original", got)
	}
}
