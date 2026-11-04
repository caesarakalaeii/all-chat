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

package oauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// revokeCapture records the one request a RevokeToken call makes and answers it
// with status, so a test can pin the exact provider endpoint and parameters.
type revokeCapture struct {
	status int
	method string
	url    *url.URL
	ctype  string
	form   url.Values
}

func (r *revokeCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	r.method = req.Method
	r.url = req.URL
	r.ctype = req.Header.Get("Content-Type")
	body, _ := io.ReadAll(req.Body)
	r.form, _ = url.ParseQuery(string(body))
	return &http.Response{StatusCode: r.status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func (r *revokeCapture) client() *http.Client { return &http.Client{Transport: r} }

func TestRevokeToken_ProviderEndpoints(t *testing.T) {
	cases := []struct {
		name      string
		revoke    func(c *http.Client) error
		wantURL   string
		wantForm  url.Values
		wantQuery url.Values
	}{
		{
			name: "twitch posts client_id and token to id.twitch.tv",
			revoke: func(c *http.Client) error {
				return NewTwitchOAuth("twitch-client", "s", "r").WithHTTPClient(c).RevokeToken(context.Background(), "tw-token")
			},
			wantURL:  "https://id.twitch.tv/oauth2/revoke",
			wantForm: url.Values{"client_id": {"twitch-client"}, "token": {"tw-token"}},
		},
		{
			name: "viewer twitch uses the same endpoint",
			revoke: func(c *http.Client) error {
				v := NewViewerTwitchOAuth("twitch-client", "s", "r")
				v.TwitchOAuth = v.TwitchOAuth.WithHTTPClient(c)
				return v.RevokeToken(context.Background(), "tw-token")
			},
			wantURL:  "https://id.twitch.tv/oauth2/revoke",
			wantForm: url.Values{"client_id": {"twitch-client"}, "token": {"tw-token"}},
		},
		{
			name: "youtube posts the token to Google",
			revoke: func(c *http.Client) error {
				y := NewYouTubeOAuth("g", "s", "r")
				y.client = c
				return y.RevokeToken(context.Background(), "g-token")
			},
			wantURL:  "https://oauth2.googleapis.com/revoke",
			wantForm: url.Values{"token": {"g-token"}},
		},
		{
			name: "viewer youtube posts the token to Google",
			revoke: func(c *http.Client) error {
				y := NewViewerYouTubeOAuth("g", "s", "r")
				y.client = c
				return y.RevokeToken(context.Background(), "g-token")
			},
			wantURL:  "https://oauth2.googleapis.com/revoke",
			wantForm: url.Values{"token": {"g-token"}},
		},
		{
			// Kick documents the token as a query parameter on a form-encoded POST.
			name: "kick passes the token as a query parameter",
			revoke: func(c *http.Client) error {
				return NewKickOAuth("k", "s", "r").WithHTTPClient(c).RevokeToken(context.Background(), "k-token")
			},
			wantURL:   "https://id.kick.com/oauth/revoke",
			wantQuery: url.Values{"token": {"k-token"}},
		},
		{
			name: "viewer kick passes the token as a query parameter",
			revoke: func(c *http.Client) error {
				k := NewViewerKickOAuth("k", "s", "r")
				k.client = c
				return k.RevokeToken(context.Background(), "k-token")
			},
			wantURL:   "https://id.kick.com/oauth/revoke",
			wantQuery: url.Values{"token": {"k-token"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := &revokeCapture{status: http.StatusOK}
			if err := tc.revoke(rc.client()); err != nil {
				t.Fatalf("RevokeToken: %v", err)
			}
			if rc.method != http.MethodPost {
				t.Errorf("method = %s, want POST", rc.method)
			}
			gotURL := rc.url.Scheme + "://" + rc.url.Host + rc.url.Path
			if gotURL != tc.wantURL {
				t.Errorf("url = %s, want %s", gotURL, tc.wantURL)
			}
			if rc.ctype != "application/x-www-form-urlencoded" {
				t.Errorf("content-type = %q", rc.ctype)
			}
			for k, v := range tc.wantForm {
				if rc.form.Get(k) != v[0] {
					t.Errorf("form %s = %q, want %q", k, rc.form.Get(k), v[0])
				}
			}
			for k, v := range tc.wantQuery {
				if rc.url.Query().Get(k) != v[0] {
					t.Errorf("query %s = %q, want %q", k, rc.url.Query().Get(k), v[0])
				}
			}
		})
	}
}

// A provider that refuses the revoke (already revoked, expired, unknown token) is
// reported as an error so the caller can log it; it must never panic or hang.
func TestRevokeToken_ProviderRefusalIsAnError(t *testing.T) {
	rc := &revokeCapture{status: http.StatusBadRequest}
	err := NewTwitchOAuth("c", "s", "r").WithHTTPClient(rc.client()).RevokeToken(context.Background(), "dead")
	if err == nil {
		t.Fatal("a 400 from the provider must be returned as an error")
	}
}

func TestRevokeToken_EmptyTokenIsNotSent(t *testing.T) {
	rc := &revokeCapture{status: http.StatusOK}
	if err := NewKickOAuth("k", "s", "r").WithHTTPClient(rc.client()).RevokeToken(context.Background(), ""); err != nil {
		t.Fatalf("empty token: %v", err)
	}
	if rc.method != "" {
		t.Error("an empty token must not be sent to the provider")
	}
}
