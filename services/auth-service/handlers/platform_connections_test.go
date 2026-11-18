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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fakeConnectionStore struct {
	connections  []repository.PlatformConnection
	removed      []repository.RemovedToken
	err          error
	disconnected []string
}

func (f *fakeConnectionStore) ListPlatformConnections(context.Context, string) ([]repository.PlatformConnection, error) {
	return f.connections, f.err
}

func (f *fakeConnectionStore) DisconnectPlatform(_ context.Context, _, platform string) ([]repository.RemovedToken, error) {
	f.disconnected = append(f.disconnected, platform)
	return f.removed, f.err
}

type fakeRevoker struct {
	revoked []string
	err     error
}

func (f *fakeRevoker) RevokeToken(_ context.Context, token string) error {
	f.revoked = append(f.revoked, token)
	return f.err
}

func connectionsRouter(h *PlatformConnectionsHandler, ctxValues map[string]string) *gin.Engine {
	r := setupTestRouter()
	r.Use(injectCtx(ctxValues))
	r.GET("/me/connections", h.HandleListConnections)
	r.DELETE("/me/connections/:platform", h.HandleDisconnect)
	return r
}

func serve(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestListConnections_ReturnsTheStoreList(t *testing.T) {
	store := &fakeConnectionStore{connections: []repository.PlatformConnection{
		{Platform: "twitch", Accounts: []string{"me"}, IsLogin: true},
		{Platform: "youtube", Accounts: []string{"UCme"}},
	}}
	h := NewPlatformConnectionsHandler(store, nil, zap.NewNop())

	w := serve(connectionsRouter(h, sessionCtx()), http.MethodGet, "/me/connections")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Connections []repository.PlatformConnection `json:"connections"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Connections) != 2 || !body.Connections[0].IsLogin || body.Connections[1].Platform != "youtube" {
		t.Errorf("connections = %+v", body.Connections)
	}
}

func TestDisconnect_DeletesThenRevokesEveryDistinctToken(t *testing.T) {
	store := &fakeConnectionStore{removed: []repository.RemovedToken{
		{AccessToken: "a1", RefreshToken: "r1"},
		{AccessToken: "a1", RefreshToken: "r1"}, // an admin's token copied to a second channel
		{AccessToken: "a2"},
	}}
	revoker := &fakeRevoker{}
	h := NewPlatformConnectionsHandler(store, map[string]TokenRevoker{"twitch": revoker}, zap.NewNop())

	w := serve(connectionsRouter(h, sessionCtx()), http.MethodDelete, "/me/connections/twitch")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(store.disconnected) != 1 || store.disconnected[0] != "twitch" {
		t.Errorf("disconnected = %v", store.disconnected)
	}
	got := append([]string(nil), revoker.revoked...)
	sort.Strings(got)
	if want := []string{"a1", "a2", "r1"}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("revoked = %v, want each distinct token once: %v", got, want)
	}
}

// Revocation is best effort: the tokens are already gone from our database, and a
// provider that is down or refuses must not turn a completed disconnect into an error.
func TestDisconnect_RevokeFailureStillSucceeds(t *testing.T) {
	store := &fakeConnectionStore{removed: []repository.RemovedToken{{AccessToken: "a1"}}}
	revoker := &fakeRevoker{err: errors.New("provider down")}
	h := NewPlatformConnectionsHandler(store, map[string]TokenRevoker{"kick": revoker}, zap.NewNop())

	if w := serve(connectionsRouter(h, sessionCtx()), http.MethodDelete, "/me/connections/kick"); w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(revoker.revoked) != 1 {
		t.Errorf("revoke attempts = %d, want 1", len(revoker.revoked))
	}
}

// Facebook and Instagram have no revoker: the disconnect deletes locally and succeeds.
func TestDisconnect_PlatformWithoutRevoker(t *testing.T) {
	store := &fakeConnectionStore{removed: []repository.RemovedToken{{AccessToken: "page"}}}
	h := NewPlatformConnectionsHandler(store, map[string]TokenRevoker{}, zap.NewNop())

	if w := serve(connectionsRouter(h, sessionCtx()), http.MethodDelete, "/me/connections/facebook"); w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestDisconnect_StoreErrorsMapToStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{repository.ErrLoginPlatform, http.StatusConflict},
		{repository.ErrUnsupportedPlatform, http.StatusBadRequest},
		{repository.ErrUserNotFound, http.StatusNotFound},
		{errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		revoker := &fakeRevoker{}
		h := NewPlatformConnectionsHandler(&fakeConnectionStore{err: tc.err}, map[string]TokenRevoker{"twitch": revoker}, zap.NewNop())
		w := serve(connectionsRouter(h, sessionCtx()), http.MethodDelete, "/me/connections/twitch")
		if w.Code != tc.want {
			t.Errorf("%v: status = %d, want %d", tc.err, w.Code, tc.want)
		}
		if len(revoker.revoked) != 0 {
			t.Errorf("%v: revoked %v although nothing was deleted", tc.err, revoker.revoked)
		}
	}
}

// Disconnecting is a whole-account action: a personal access token must not reach
// it, and an admin impersonating the user must not strip their connections.
func TestDisconnect_SessionOnly(t *testing.T) {
	impersonating := sessionCtx()
	impersonating["impersonated_by"] = "admin-id"

	for name, ctxValues := range map[string]map[string]string{"pat": patCtx(), "impersonation": impersonating} {
		store := &fakeConnectionStore{}
		h := NewPlatformConnectionsHandler(store, nil, zap.NewNop())
		w := serve(connectionsRouter(h, ctxValues), http.MethodDelete, "/me/connections/twitch")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, w.Code)
		}
		if len(store.disconnected) != 0 {
			t.Errorf("%s: the store was called", name)
		}
	}
}

func TestListConnections_RefusesPAT(t *testing.T) {
	h := NewPlatformConnectionsHandler(&fakeConnectionStore{}, nil, zap.NewNop())
	if w := serve(connectionsRouter(h, patCtx()), http.MethodGet, "/me/connections"); w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestConnections_RequireAUser(t *testing.T) {
	h := NewPlatformConnectionsHandler(&fakeConnectionStore{}, nil, zap.NewNop())
	r := connectionsRouter(h, map[string]string{})
	for _, req := range [][2]string{{http.MethodGet, "/me/connections"}, {http.MethodDelete, "/me/connections/twitch"}} {
		if w := serve(r, req[0], req[1]); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", req[0], req[1], w.Code)
		}
	}
}
