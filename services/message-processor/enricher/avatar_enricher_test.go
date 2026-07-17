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

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
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

	e := NewAvatarEnricher(rc, "client-id", "client-secret", "http://gateway", zap.NewNop())
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
