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
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// sharedChatHelixServer stands in for Twitch. helixHandler answers /helix/users so a
// test can decide per case what the origin broadcaster lookup returns; helixCalls
// counts those lookups so a test can assert the cache was used.
type sharedChatHelixServer struct {
	server     *httptest.Server
	helixCalls atomic.Int32
}

func newSharedChatHelixServer(t *testing.T, helixHandler http.HandlerFunc) *sharedChatHelixServer {
	t.Helper()
	s := &sharedChatHelixServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/token":
			fmt.Fprint(w, `{"access_token":"app-token","expires_in":3600,"token_type":"bearer"}`)
		case "/helix/users":
			s.helixCalls.Add(1)
			helixHandler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.server.Close)
	return s
}

func newSharedChatEnricher(t *testing.T, srv *sharedChatHelixServer) (*AvatarEnricher, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	e := NewAvatarEnricher(rc, "client-id", "client-secret", "http://gateway", zap.NewNop())
	e.tokenURL = srv.server.URL + "/oauth2/token"
	e.helixUsersURL = srv.server.URL + "/helix/users"
	return e, rc
}

// sharedChatMessage is a Twitch message that already carries an avatar for the
// chatter, so only the origin-channel lookup is under test.
func sharedChatMessage(sourceRoomID string) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		Platform: "twitch",
		User: models.UserInfo{
			ID:        "chatter-1",
			Username:  "alice",
			AvatarURL: "https://cdn/alice.png",
		},
		Metadata: map[string]interface{}{
			"is_shared_chat": true,
			"source_room_id": sourceRoomID,
		},
	}
}

func TestEnrichTwitch_SharedChatSetsSourceAvatarAndDisplayName(t *testing.T) {
	srv := newSharedChatHelixServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":%q,"login":"originchan","display_name":"OriginChan","profile_image_url":"https://cdn/origin.png"}]}`,
			r.URL.Query().Get("id"))
	})
	e, _ := newSharedChatEnricher(t, srv)

	msg := sharedChatMessage("origin-42")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich: %v", err)
	}

	if got := msg.Metadata["source_avatar_url"]; got != "https://cdn/origin.png" {
		t.Errorf("source_avatar_url = %v, want https://cdn/origin.png", got)
	}
	if got := msg.Metadata["source_display_name"]; got != "OriginChan" {
		t.Errorf("source_display_name = %v, want OriginChan", got)
	}
	if msg.User.AvatarURL != "https://cdn/alice.png" {
		t.Errorf("chatter avatar was overwritten: %q", msg.User.AvatarURL)
	}
}

func TestEnrichTwitch_SharedChatReusesAvatarCache(t *testing.T) {
	srv := newSharedChatHelixServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":%q,"login":"originchan","display_name":"OriginChan","profile_image_url":"https://cdn/origin.png"}]}`,
			r.URL.Query().Get("id"))
	})
	e, rc := newSharedChatEnricher(t, srv)

	for i := 0; i < 2; i++ {
		if err := e.Enrich(context.Background(), sharedChatMessage("origin-42")); err != nil {
			t.Fatalf("Enrich #%d: %v", i, err)
		}
	}

	if calls := srv.helixCalls.Load(); calls != 1 {
		t.Errorf("helix lookups = %d, want 1 (second message should hit the cache)", calls)
	}
	cached, err := rc.Get(context.Background(), AvatarCacheKeyPrefix+"origin-42").Result()
	if err != nil {
		t.Fatalf("reading %s%s: %v", AvatarCacheKeyPrefix, "origin-42", err)
	}
	if cached != "https://cdn/origin.png" {
		t.Errorf("cached avatar URL = %q, want https://cdn/origin.png", cached)
	}
}

func TestEnrichTwitch_SharedChatLookupFailureLeavesKeysUnset(t *testing.T) {
	srv := newSharedChatHelixServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	e, _ := newSharedChatEnricher(t, srv)

	msg := sharedChatMessage("origin-42")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("a failed origin lookup must not fail the message: %v", err)
	}

	if _, ok := msg.Metadata["source_avatar_url"]; ok {
		t.Errorf("source_avatar_url should be unset, got %v", msg.Metadata["source_avatar_url"])
	}
	if _, ok := msg.Metadata["source_display_name"]; ok {
		t.Errorf("source_display_name should be unset, got %v", msg.Metadata["source_display_name"])
	}
}

// The grey Twitch default is not a picture of anyone; rendering it for every origin
// channel would be worse than the text pill the frontend falls back to.
func TestEnrichTwitch_SharedChatDefaultAvatarLeavesURLUnset(t *testing.T) {
	srv := newSharedChatHelixServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":%q,"login":"originchan","display_name":"OriginChan","profile_image_url":%q}]}`,
			r.URL.Query().Get("id"),
			"https://static-cdn.jtvnw.net/jtv_user_pictures/-profile_image-70x70.png")
	})
	e, _ := newSharedChatEnricher(t, srv)

	msg := sharedChatMessage("origin-42")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich: %v", err)
	}

	if _, ok := msg.Metadata["source_avatar_url"]; ok {
		t.Errorf("source_avatar_url should be unset for the default avatar, got %v", msg.Metadata["source_avatar_url"])
	}
	if got := msg.Metadata["source_display_name"]; got != "OriginChan" {
		t.Errorf("source_display_name = %v, want OriginChan even without a picture", got)
	}
}

func TestEnrichTwitch_NotSharedChatSkipsOriginLookup(t *testing.T) {
	srv := newSharedChatHelixServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":[{"id":%q,"login":"u","display_name":"U","profile_image_url":"https://cdn/u.png"}]}`,
			r.URL.Query().Get("id"))
	})
	e, _ := newSharedChatEnricher(t, srv)

	msg := sharedChatMessage("origin-42")
	msg.Metadata["is_shared_chat"] = false

	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("Enrich: %v", err)
	}

	if srv.helixCalls.Load() != 0 {
		t.Errorf("helix lookups = %d, want 0", srv.helixCalls.Load())
	}
	if _, ok := msg.Metadata["source_avatar_url"]; ok {
		t.Errorf("source_avatar_url should be unset when is_shared_chat is false")
	}
}
