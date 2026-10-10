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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type recordedRequest struct {
	path  string
	query url.Values
}

// newTestClient serves body for every request and records what the client asked for.
func newTestClient(t *testing.T, body interface{}) (*Client, func() []recordedRequest) {
	t.Helper()
	var (
		mu   sync.Mutex
		reqs []recordedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, recordedRequest{path: r.URL.Path, query: r.URL.Query()})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)

	service, err := youtube.NewService(context.Background(),
		option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("youtube.NewService: %v", err)
	}
	return NewClient(service, srv.Client(), nil, zap.NewNop()), func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

func broadcast(videoID, channelID, chatID, privacy string) map[string]interface{} {
	return map[string]interface{}{
		"id":      videoID,
		"snippet": map[string]string{"channelId": channelID, "liveChatId": chatID, "title": "title " + videoID},
		"status":  map[string]string{"privacyStatus": privacy, "lifeCycleStatus": "live"},
	}
}

func TestGetActiveBroadcasts_Request(t *testing.T) {
	client, requests := newTestClient(t, map[string]interface{}{"items": []interface{}{}})
	if _, err := client.GetActiveBroadcasts(context.Background(), "UCowner", nil); err != nil {
		t.Fatalf("GetActiveBroadcasts: %v", err)
	}

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1 (no search.list, no per-video lookups)", len(reqs))
	}
	r := reqs[0]
	if !strings.HasSuffix(r.path, "/liveBroadcasts") {
		t.Fatalf("path = %q, want .../liveBroadcasts", r.path)
	}
	if got := r.query.Get("broadcastStatus"); got != "active" {
		t.Fatalf("broadcastStatus = %q, want active", got)
	}
	if got := r.query.Get("broadcastType"); got != "all" {
		t.Fatalf("broadcastType = %q, want all (persistent \"Stream now\" broadcasts are type persistent)", got)
	}
	var parts []string
	for _, v := range r.query["part"] {
		parts = append(parts, strings.Split(v, ",")...)
	}
	for _, want := range []string{"id", "snippet", "status"} {
		found := false
		for _, p := range parts {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("part = %v, missing %q", parts, want)
		}
	}
}

func TestGetActiveBroadcasts_FiltersForeignChannel(t *testing.T) {
	client, _ := newTestClient(t, map[string]interface{}{"items": []interface{}{
		broadcast("ownVideo001", "UCowner", "chat-own", "public"),
		broadcast("otherVideo1", "UCbrandAccount", "chat-other", "public"),
	}})
	got, err := client.GetActiveBroadcasts(context.Background(), "UCowner", nil)
	if err != nil {
		t.Fatalf("GetActiveBroadcasts: %v", err)
	}
	if len(got) != 1 || got[0].StreamID != "ownVideo001" {
		t.Fatalf("got %+v, want only ownVideo001", got)
	}
	if got[0].LiveChatID != "chat-own" || got[0].ChannelID != "UCowner" || !got[0].IsLive {
		t.Fatalf("stream = %+v, want live chat-own on UCowner", *got[0])
	}
}

func TestGetActiveBroadcasts_IncludesUnlisted(t *testing.T) {
	client, _ := newTestClient(t, map[string]interface{}{"items": []interface{}{
		broadcast("unlisted001", "UCowner", "chat-u", "unlisted"),
		broadcast("private0001", "UCowner", "chat-p", "private"),
	}})
	got, err := client.GetActiveBroadcasts(context.Background(), "UCowner", nil)
	if err != nil {
		t.Fatalf("GetActiveBroadcasts: %v", err)
	}
	if len(got) != 2 || got[0].StreamID != "unlisted001" || got[1].StreamID != "private0001" {
		t.Fatalf("got %+v, want unlisted001 and private0001", got)
	}
}

func TestGetActiveBroadcasts_EmptyNotLive(t *testing.T) {
	client, _ := newTestClient(t, map[string]interface{}{"items": []interface{}{}})
	got, err := client.GetActiveBroadcasts(context.Background(), "UCowner", nil)
	if err != nil {
		t.Fatalf("GetActiveBroadcasts: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want no streams", got)
	}
}

func TestTokenChannelID_Request(t *testing.T) {
	client, requests := newTestClient(t, map[string]interface{}{"items": []interface{}{
		map[string]string{"id": "UCtokenChannel"},
	}})
	got, err := client.TokenChannelID(context.Background())
	if err != nil {
		t.Fatalf("TokenChannelID: %v", err)
	}
	if got != "UCtokenChannel" {
		t.Fatalf("TokenChannelID = %q, want UCtokenChannel", got)
	}

	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(reqs))
	}
	if !strings.HasSuffix(reqs[0].path, "/channels") {
		t.Fatalf("path = %q, want .../channels", reqs[0].path)
	}
	if got := reqs[0].query.Get("mine"); got != "true" {
		t.Fatalf("mine = %q, want true", got)
	}
}

// The streams package decides from these errors whether an owner token was rejected
// (isOwnerAuthError), so the googleapi.Error with its code and reason must survive the wrapping.
func TestGetActiveBroadcastsAuthError_KeepsGoogleAPIError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reason string
	}{
		{"401", http.StatusUnauthorized, "authError"},
		{"403_forbidden", http.StatusForbidden, "forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]interface{}{
					"code": tc.status, "message": "rejected",
					"errors": []map[string]string{{"reason": tc.reason, "message": "rejected"}},
				}})
			}))
			t.Cleanup(srv.Close)
			service, err := youtube.NewService(context.Background(),
				option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
			if err != nil {
				t.Fatalf("youtube.NewService: %v", err)
			}
			client := NewClient(service, srv.Client(), nil, zap.NewNop())

			_, err = client.GetActiveBroadcasts(context.Background(), "UCowner", nil)
			var apiErr *googleapi.Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("GetActiveBroadcasts error %v does not wrap *googleapi.Error", err)
			}
			if apiErr.Code != tc.status {
				t.Fatalf("code = %d, want %d", apiErr.Code, tc.status)
			}
			if len(apiErr.Errors) != 1 || apiErr.Errors[0].Reason != tc.reason {
				t.Fatalf("reasons = %+v, want [%s]", apiErr.Errors, tc.reason)
			}
		})
	}
}
