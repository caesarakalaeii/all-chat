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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// The resolver must extract activeLiveChatId from a videos.list response and pass the
// expected query parameters (part=liveStreamingDetails, id, key).
func TestDataAPILiveChatResolver_Resolve(t *testing.T) {
	var gotPart, gotID, gotKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPart = r.URL.Query().Get("part")
		gotID = r.URL.Query().Get("id")
		gotKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"liveStreamingDetails":{"activeLiveChatId":"LC_xyz"}}]}`))
	}))
	defer ts.Close()

	r := NewDataAPILiveChatResolver("test-key", zap.NewNop())
	r.baseURL = ts.URL

	chatID, err := r.Resolve(context.Background(), "vid123")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if chatID != "LC_xyz" {
		t.Errorf("chatID = %q, want LC_xyz", chatID)
	}
	if gotPart != "liveStreamingDetails" || gotID != "vid123" || gotKey != "test-key" {
		t.Errorf("query params wrong: part=%q id=%q key=%q", gotPart, gotID, gotKey)
	}
}

// An empty items array (video not live / no chat) must surface as an error, not "".
func TestDataAPILiveChatResolver_Resolve_NoActiveChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer ts.Close()

	r := NewDataAPILiveChatResolver("k", zap.NewNop())
	r.baseURL = ts.URL
	if _, err := r.Resolve(context.Background(), "vid"); err == nil {
		t.Error("expected error for no active live chat, got nil")
	}
}

// A non-200 (e.g. the observed 403) must error.
func TestDataAPILiveChatResolver_Resolve_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403}}`))
	}))
	defer ts.Close()

	r := NewDataAPILiveChatResolver("k", zap.NewNop())
	r.baseURL = ts.URL
	if _, err := r.Resolve(context.Background(), "vid"); err == nil {
		t.Error("expected error for 403 response, got nil")
	}
}

// SetStreamState must publish the exact youtube:stream:state contract auth-service and
// moderation-service read: an entry with is_live=true and the resolved live_chat_id.
func TestRepository_SetStreamState_ConsumerContract(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	repo := NewRepository(rc, zap.NewNop())
	ctx := context.Background()

	if err := repo.SetStreamState(ctx, "UC1", "vid1", "ov1", "LC_abc"); err != nil {
		t.Fatalf("SetStreamState: %v", err)
	}

	raw, err := rc.Get(ctx, "youtube:stream:state:UC1").Result()
	if err != nil {
		t.Fatalf("expected key to exist: %v", err)
	}
	// Decode into the SAME shape the consumers use (auth-service / moderation-service).
	var consumer struct {
		LiveChatID string `json:"live_chat_id"`
		IsLive     bool   `json:"is_live"`
	}
	if err := json.Unmarshal([]byte(raw), &consumer); err != nil {
		t.Fatalf("consumer cannot parse stream state: %v", err)
	}
	if !consumer.IsLive || consumer.LiveChatID != "LC_abc" {
		t.Errorf("consumer view wrong: is_live=%v live_chat_id=%q", consumer.IsLive, consumer.LiveChatID)
	}
	// TTL must be set so a missed cleanup self-heals.
	if ttl := rc.TTL(ctx, "youtube:stream:state:UC1").Val(); ttl <= 0 {
		t.Errorf("expected positive TTL, got %v", ttl)
	}
}

func TestRepository_DeleteStreamState(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	repo := NewRepository(rc, zap.NewNop())
	ctx := context.Background()

	if err := repo.SetStreamState(ctx, "UC1", "vid1", "ov1", "LC_abc"); err != nil {
		t.Fatalf("SetStreamState: %v", err)
	}
	if err := repo.DeleteStreamState(ctx, "UC1"); err != nil {
		t.Fatalf("DeleteStreamState: %v", err)
	}
	if _, err := rc.Get(ctx, "youtube:stream:state:UC1").Result(); err != redis.Nil {
		t.Errorf("expected key deleted, got err=%v", err)
	}
}

// The heartbeat snapshot must include only streams whose live chat id has been resolved
// (an unresolved stream has nothing to publish yet).
func TestManager_streamStatesToRefresh(t *testing.T) {
	m := &Manager{
		logger: zap.NewNop(),
		activeStreams: map[string]*Stream{
			"vidA": {VideoID: "vidA", ChannelID: "UCA", OverlayID: "ovA", LiveChatID: "LC_A"},
			"vidB": {VideoID: "vidB", ChannelID: "UCB", OverlayID: "ovB", LiveChatID: ""}, // not resolved yet
		},
	}
	states := m.streamStatesToRefresh()
	if len(states) != 1 {
		t.Fatalf("expected 1 refreshable state, got %d", len(states))
	}
	if states[0].ChannelID != "UCA" || states[0].LiveChatID != "LC_A" || states[0].StreamID != "vidA" {
		t.Errorf("unexpected snapshot: %+v", states[0])
	}
}
