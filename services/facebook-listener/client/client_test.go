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

func TestLiveVideos_ParsesAndFiltersPath(t *testing.T) {
	var gotPath, gotFields string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotFields = r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "vid_live", "status": "LIVE_NOW", "title": "Evening stream", "created_time": "2026-09-01T10:00:00Z"},
				{"id": "vid_sched", "status": "SCHEDULED_LIVE", "title": "Later", "created_time": "2026-09-01T11:00:00Z"},
			},
		})
	})

	videos, err := c.LiveVideos(context.Background(), "page-1", "page-token")
	if err != nil {
		t.Fatalf("LiveVideos: %v", err)
	}
	if gotPath != "/v26.0/page-1/live_videos" {
		t.Errorf("path = %q, want /v26.0/page-1/live_videos", gotPath)
	}
	if gotFields == "" {
		t.Error("fields param missing")
	}
	if len(videos) != 2 {
		t.Fatalf("got %d videos, want 2", len(videos))
	}
	if videos[0].Status != "LIVE_NOW" || videos[0].ID != "vid_live" {
		t.Errorf("videos[0] = %+v", videos[0])
	}
}

func TestComments_ParamsAndParsing(t *testing.T) {
	var q url.Values
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("X-Business-Use-Case-Usage", `{"99":[{"type":"pages","call_count":42}]}`)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":           "c2",
					"message":      "newest",
					"created_time": "2026-09-01T10:00:05Z",
					"from":         map[string]string{"id": "u2", "name": "Bea"},
				},
				{
					"id":           "c1",
					"message":      "oldest",
					"created_time": "2026-09-01T10:00:00Z",
					"from":         map[string]string{"id": "u1", "name": "Al"},
					"parent":       map[string]string{"id": "c0"},
				},
			},
			"paging": map[string]interface{}{
				"cursors": map[string]string{"after": "CURSOR1"},
			},
		})
	})

	data, after, usage, err := c.CommentsWithUsage(context.Background(), "vid_live", "page-token", CommentsParams{
		Order: "reverse_chronological",
		Since: "2026-09-01T09:00:00+0000",
		Limit: "100",
	})
	if err != nil {
		t.Fatalf("CommentsWithUsage: %v", err)
	}
	if q.Get("order") != "reverse_chronological" {
		t.Errorf("order = %q", q.Get("order"))
	}
	if q.Get("since") != "2026-09-01T09:00:00+0000" {
		t.Errorf("since = %q", q.Get("since"))
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
	if data[1].Parent.ID != "c0" {
		t.Errorf("reply parent not parsed: %+v", data[1])
	}
	if data[0].From.Name != "Bea" {
		t.Errorf("from.name = %q", data[0].From.Name)
	}
}

func TestGraphErrorClassification(t *testing.T) {
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
		_, _, err := c.Comments(context.Background(), "vid", "tok", CommentsParams{})
		if !errors.Is(err, tc.want) {
			t.Errorf("status %d: got %v, want %v", tc.status, err, tc.want)
		}
	}
}

func TestMyPages(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v26.0/me/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "p1", "name": "My Page", "access_token": "EAAC-page-token", "tasks": []string{"ANALYZE", "MODERATE", "CREATE_CONTENT"}},
			},
		})
	})
	pages, err := c.MyPages(context.Background(), "user-token")
	if err != nil {
		t.Fatalf("MyPages: %v", err)
	}
	if len(pages) != 1 || pages[0].ID != "p1" {
		t.Fatalf("pages = %+v", pages)
	}
	if !pages[0].HasModerateTask() {
		t.Error("MODERATE task should satisfy HasModerateTask")
	}
}
