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

package clients

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"go.uber.org/zap"
)

// newTestTwitchClient points every Twitch request (token and Helix) at srv.
func newTestTwitchClient(t *testing.T, srv *httptest.Server) *TwitchClipsClient {
	t.Helper()
	target, _ := url.Parse(srv.URL)
	c := NewTwitchClipsClient("id", "secret", zap.NewNop())
	c.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})}
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetStreamStartedAt(t *testing.T) {
	streams := `{"data":[{"started_at":"2026-10-16T12:57:00Z"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
		case "/helix/streams":
			if r.URL.Query().Get("user_login") != "lejoe_ttv" {
				t.Errorf("unexpected user_login %q", r.URL.Query().Get("user_login"))
			}
			_, _ = w.Write([]byte(streams))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newTestTwitchClient(t, srv)

	got, err := c.GetStreamStartedAt(t.Context(), "lejoe_ttv")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 16, 12, 57, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("live stream start = %v, want %v", got, want)
	}

	streams = `{"data":[]}`
	got, err = c.GetStreamStartedAt(t.Context(), "lejoe_ttv")
	if err != nil || !got.IsZero() {
		t.Fatalf("offline channel must return zero time, got %v, %v", got, err)
	}
}
