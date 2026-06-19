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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// newTestTwitchClient wires a TwitchClient to local token/users test servers.
func newTestTwitchClient(tokenURL, usersURL, apiBase string) *TwitchClient {
	c := NewTwitchClient("client-id", "client-secret", zap.NewNop())
	c.tokenURL = tokenURL
	c.usersURL = usersURL
	c.apiBase = apiBase
	return c
}

// TestGetUserIDRefreshesTokenOn401 verifies that a Twitch-invalidated app token
// recovers on the next call: the client refreshes the token and retries the
// request once, instead of staying wedged on the dead token until restart.
func TestGetUserIDRefreshesTokenOn401(t *testing.T) {
	var mu sync.Mutex
	tokenIssued := 0

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		tokenIssued++
		tok := fmt.Sprintf("tok-%d", tokenIssued)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"expires_in":5184000}`, tok)
	}))
	defer tokenSrv.Close()

	usersSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// The first token Twitch issued is treated as invalidated: reject it so
		// the client must refresh. The refreshed token is accepted.
		if auth == "Bearer tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"12345"}]}`)
	}))
	defer usersSrv.Close()

	c := newTestTwitchClient(tokenSrv.URL, usersSrv.URL, "")

	id, err := c.GetUserID(context.Background(), "caesarlp")
	if err != nil {
		t.Fatalf("GetUserID returned error: %v", err)
	}
	if id != "12345" {
		t.Fatalf("got id %q, want 12345", id)
	}

	mu.Lock()
	issued := tokenIssued
	mu.Unlock()
	if issued != 2 {
		t.Fatalf("expected token to be refreshed once after 401 (2 issuances), got %d", issued)
	}
}

// TestGetUserIDDoesNotLoopOnPersistent401 ensures the retry is bounded: a token
// that stays unauthorized surfaces an error rather than refreshing forever.
func TestGetUserIDDoesNotLoopOnPersistent401(t *testing.T) {
	var mu sync.Mutex
	tokenIssued := 0

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		tokenIssued++
		mu.Unlock()
		fmt.Fprint(w, `{"access_token":"always-bad","expires_in":5184000}`)
	}))
	defer tokenSrv.Close()

	usersSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer usersSrv.Close()

	c := newTestTwitchClient(tokenSrv.URL, usersSrv.URL, "")

	_, err := c.GetUserID(context.Background(), "caesarlp")
	if err == nil {
		t.Fatal("expected error on persistent 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}

	mu.Lock()
	issued := tokenIssued
	mu.Unlock()
	// One initial fetch + exactly one refresh after the first 401.
	if issued != 2 {
		t.Fatalf("expected exactly one refresh (2 issuances), got %d", issued)
	}
}
