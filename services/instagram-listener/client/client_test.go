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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// newTestClient spins a fake Graph API and a client pinned to it.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "v26.0")
	return c, srv
}

func TestLiveMedia_ParsesPathAndFields(t *testing.T) {
	var gotPath, gotFields string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotFields = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "media_live", "media_type": "BROADCAST", "media_product_type": "LIVE"},
			},
		})
	})

	media, err := c.LiveMedia(context.Background(), "ig-user-1", "page-token")
	if err != nil {
		t.Fatalf("LiveMedia: %v", err)
	}
	if gotPath != "/v26.0/ig-user-1/live_media" {
		t.Errorf("path = %q, want /v26.0/ig-user-1/live_media", gotPath)
	}
	if gotFields == "" {
		t.Error("fields param missing")
	}
	if len(media) != 1 {
		t.Fatalf("got %d media, want 1", len(media))
	}
	if !media[0].IsLiveBroadcast() || media[0].ID != "media_live" {
		t.Errorf("media[0] = %+v", media[0])
	}
}

func TestLiveComments_ParamsAndParsing(t *testing.T) {
	var q url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("X-Business-Use-Case-Usage", `{"99":[{"type":"pages","call_count":42}]}`)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":        "c2",
					"text":      "newest",
					"timestamp": "2026-09-01T10:00:05+0000",
					"from":      map[string]string{"id": "u2", "username": "bea_ig"},
				},
				{
					"id":        "c1",
					"text":      "oldest",
					"timestamp": "2026-09-01T10:00:00+0000",
					"from":      map[string]string{"id": "u1", "username": "al_ig"},
					"parent_id": "c0",
					"media":     map[string]string{"id": "media_live"},
				},
			},
			"paging": map[string]interface{}{
				"cursors": map[string]string{"after": "CURSOR1"},
			},
		})
	})

	data, after, usage, err := c.LiveCommentsWithUsage(context.Background(), "ig-user-1", "page-token", LiveCommentsParams{
		CommentID: "c0",
		Limit:     "50",
	})
	if err != nil {
		t.Fatalf("LiveCommentsWithUsage: %v", err)
	}
	if gotPath := q.Get("comment_id"); gotPath != "c0" {
		t.Errorf("comment_id = %q, want c0", gotPath)
	}
	if q.Get("fields") == "" {
		t.Error("fields param missing")
	}
	if q.Get("access_token") != "page-token" {
		t.Errorf("access_token not sent")
	}
	if after != "CURSOR1" {
		t.Errorf("after cursor = %q", after)
	}
	if usage.CallCount != 42 {
		t.Errorf("usage.CallCount = %d, want 42", usage.CallCount)
	}
	if len(data) != 2 {
		t.Fatalf("got %d comments, want 2", len(data))
	}
	if data[1].ParentID != "c0" {
		t.Errorf("reply parent not parsed: %+v", data[1])
	}
	if data[0].From.Username != "bea_ig" {
		t.Errorf("from.username = %q", data[0].From.Username)
	}
	if data[0].Timestamp != "2026-09-01T10:00:05+0000" {
		t.Errorf("timestamp = %q", data[0].Timestamp)
	}
}

func TestLiveComments_ErrorClassification(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   error
	}{
		{401, `{"error":{"code":190,"message":"Invalid OAuth access token"}}`, ErrUnauthorized},
		{400, `{"error":{"code":4,"message":"Application request limit reached"}}`, ErrRateLimited},
		{400, `{"error":{"code":32,"message":"Page request limit reached"}}`, ErrRateLimited},
		{403, `{"error":{"code":200,"message":"Permissions error"}}`, ErrForbidden},
	}
	for _, tc := range cases {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})
		_, _, err := c.LiveComments(context.Background(), "ig-user-1", "tok", LiveCommentsParams{})
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: got %v, want %v", tc.status, err, tc.want)
		}
	}
}
