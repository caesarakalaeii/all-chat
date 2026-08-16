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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// fakeAPITokenStore records what the handler asked for and returns canned results, so
// the request-shaping and guard logic can be tested without a database.
type fakeAPITokenStore struct {
	created    *repository.APIToken
	createErr  error
	lastHash   []byte
	lastName   string
	lastScopes []string
	lastExpiry *time.Time

	list    []repository.APIToken
	listErr error

	revoked      *repository.APIToken
	revokeErr    error
	revokedID    string
	revokeCalled bool
}

func (f *fakeAPITokenStore) CreateAPIToken(_ context.Context, _, name string, tokenHash []byte, scopes []string, expiresAt *time.Time) (*repository.APIToken, error) {
	f.lastName, f.lastHash, f.lastScopes, f.lastExpiry = name, tokenHash, scopes, expiresAt
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &repository.APIToken{ID: "11111111-1111-1111-1111-111111111111", Name: name, Scopes: scopes, CreatedAt: time.Now(), ExpiresAt: expiresAt}, nil
}

func (f *fakeAPITokenStore) ListAPITokensByUser(_ context.Context, _ string) ([]repository.APIToken, error) {
	return f.list, f.listErr
}

func (f *fakeAPITokenStore) RevokeAPIToken(_ context.Context, _, tokenID string) (*repository.APIToken, error) {
	f.revokeCalled, f.revokedID = true, tokenID
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	if f.revoked != nil {
		return f.revoked, nil
	}
	now := time.Now()
	return &repository.APIToken{ID: tokenID, Name: "revoked", Scopes: []string{}, CreatedAt: now, RevokedAt: &now}, nil
}

const testUserID = "99999999-9999-9999-9999-999999999999"

// apiTokenRouter mounts the three management routes behind a stub middleware that
// injects the given context values (standing in for JWTAuthWithRevocation).
func apiTokenRouter(store apiTokenStore, ctxValues map[string]string) *gin.Engine {
	router := setupTestRouter()
	h := newAPITokenHandlerWithStore(store, zap.NewNop())
	inject := func(c *gin.Context) {
		for k, v := range ctxValues {
			c.Set(k, v)
		}
		c.Next()
	}
	router.POST("/me/api-tokens", inject, h.HandleCreateAPIToken)
	router.GET("/me/api-tokens", inject, h.HandleListAPITokens)
	router.DELETE("/me/api-tokens/:id", inject, h.HandleRevokeAPIToken)
	return router
}

func sessionCtx() map[string]string {
	return map[string]string{"user_id": testUserID, middleware.CtxAuthMethod: middleware.AuthMethodJWT}
}

func postCreate(router *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/me/api-tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

// The plaintext is returned exactly once, only here, and only a digest is handed to
// the store.
func TestHandleCreateAPIToken_ReturnsPlaintextOnceAndStoresOnlyHash(t *testing.T) {
	store := &fakeAPITokenStore{}
	router := apiTokenRouter(store, sessionCtx())

	w := postCreate(router, `{"name":"Stream Deck","scopes":["chat:write","engagement:write"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
		Token  string   `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !middleware.IsAPIToken(resp.Token) {
		t.Fatalf("create response token %q is not an allchat_pat_ token", resp.Token)
	}
	if resp.Name != "Stream Deck" || len(resp.Scopes) != 2 {
		t.Fatalf("unexpected metadata: %+v", resp)
	}

	// What reached persistence must be the digest of the returned plaintext, and the
	// plaintext itself must appear nowhere in the stored fields.
	want := middleware.HashAPIToken(resp.Token)
	if string(store.lastHash) != string(want) {
		t.Fatalf("stored hash is not sha256(plaintext)")
	}
	if strings.Contains(store.lastName, resp.Token) {
		t.Fatalf("plaintext token leaked into a stored field")
	}
	// The response must not carry the digest under any name.
	if strings.Contains(w.Body.String(), "token_hash") {
		t.Fatalf("create response exposes token_hash: %s", w.Body.String())
	}
}

func TestHandleCreateAPIToken_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"scopes":["chat:write"]}`},
		{"blank name", `{"name":"   ","scopes":["chat:write"]}`},
		{"name too long", `{"name":"` + strings.Repeat("x", 121) + `","scopes":["chat:write"]}`},
		{"missing scopes", `{"name":"deck"}`},
		{"empty scopes", `{"name":"deck","scopes":[]}`},
		{"blank scope only", `{"name":"deck","scopes":["  "]}`},
		{"unknown scope", `{"name":"deck","scopes":["admin:*"]}`},
		{"past expiry", `{"name":"deck","scopes":["chat:write"],"expires_at":"2000-01-01T00:00:00Z"}`},
		{"absurd expiry", `{"name":"deck","scopes":["chat:write"],"expires_at":"2999-01-01T00:00:00Z"}`},
		{"garbage body", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeAPITokenStore{}
			w := postCreate(apiTokenRouter(store, sessionCtx()), tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if store.lastHash != nil {
				t.Fatalf("invalid request reached persistence")
			}
		})
	}
}

func TestHandleCreateAPIToken_DeduplicatesScopes(t *testing.T) {
	store := &fakeAPITokenStore{}
	w := postCreate(apiTokenRouter(store, sessionCtx()),
		`{"name":"deck","scopes":["chat:write"," chat:write ","engagement:write"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if len(store.lastScopes) != 2 {
		t.Fatalf("expected 2 de-duplicated scopes, got %v", store.lastScopes)
	}
}

func TestHandleCreateAPIToken_LimitReached(t *testing.T) {
	store := &fakeAPITokenStore{createErr: repository.ErrAPITokenLimitReached}
	w := postCreate(apiTokenRouter(store, sessionCtx()), `{"name":"deck","scopes":["chat:write"]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleAPITokens_Unauthenticated(t *testing.T) {
	store := &fakeAPITokenStore{}
	router := apiTokenRouter(store, nil)

	if w := postCreate(router, `{"name":"deck","scopes":["chat:write"]}`); w.Code != http.StatusUnauthorized {
		t.Errorf("create status = %d, want 401", w.Code)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me/api-tokens", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("list status = %d, want 401", w.Code)
	}
}

// Impersonation must not be able to mint a credential that outlives the session, nor
// to revoke the user's tokens.
func TestHandleAPITokens_ImpersonationForbidden(t *testing.T) {
	ctx := sessionCtx()
	ctx["impersonated_by"] = "22222222-2222-2222-2222-222222222222"
	store := &fakeAPITokenStore{}
	router := apiTokenRouter(store, ctx)

	if w := postCreate(router, `{"name":"deck","scopes":["chat:write"]}`); w.Code != http.StatusForbidden {
		t.Errorf("create status = %d, want 403", w.Code)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/me/api-tokens/11111111-1111-1111-1111-111111111111", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("revoke status = %d, want 403", w.Code)
	}
	if store.revokeCalled {
		t.Fatalf("impersonated revoke reached persistence")
	}
}

// A PAT must not be able to manage PATs: token management is a session-only surface,
// so a leaked token cannot mint more tokens or lock the owner out.
func TestHandleAPITokens_PATCannotManageTokens(t *testing.T) {
	ctx := map[string]string{
		"user_id":                testUserID,
		middleware.CtxAuthMethod: middleware.AuthMethodAPIToken,
		middleware.CtxAPITokenID: "11111111-1111-1111-1111-111111111111",
	}
	store := &fakeAPITokenStore{}
	router := apiTokenRouter(store, ctx)

	if w := postCreate(router, `{"name":"deck","scopes":["chat:write"]}`); w.Code != http.StatusForbidden {
		t.Errorf("create status = %d, want 403", w.Code)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me/api-tokens", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("list status = %d, want 403", w.Code)
	}
	if store.lastHash != nil || store.revokeCalled {
		t.Fatalf("PAT-authenticated management reached persistence")
	}
}

// The list response may only ever carry metadata.
func TestHandleListAPITokens_MetadataOnly(t *testing.T) {
	used := time.Now().Add(-time.Hour)
	store := &fakeAPITokenStore{list: []repository.APIToken{{
		ID:         "11111111-1111-1111-1111-111111111111",
		Name:       "Stream Deck",
		Scopes:     []string{"chat:write"},
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		LastUsedAt: &used,
	}}}
	router := apiTokenRouter(store, sessionCtx())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me/api-tokens", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, forbidden := range []string{"token_hash", middleware.APITokenPrefix, `"token"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("list response contains %q: %s", forbidden, body)
		}
	}
	for _, expected := range []string{"Stream Deck", "chat:write", "last_used_at", "created_at"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("list response missing %q: %s", expected, body)
		}
	}
}

func TestHandleRevokeAPIToken(t *testing.T) {
	t.Run("revokes own token", func(t *testing.T) {
		store := &fakeAPITokenStore{}
		router := apiTokenRouter(store, sessionCtx())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete,
			"/me/api-tokens/11111111-1111-1111-1111-111111111111", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
		if store.revokedID != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("revoked the wrong id: %q", store.revokedID)
		}
	})

	t.Run("malformed id is 400", func(t *testing.T) {
		store := &fakeAPITokenStore{}
		router := apiTokenRouter(store, sessionCtx())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/me/api-tokens/not-a-uuid", nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		if store.revokeCalled {
			t.Fatalf("malformed id reached persistence")
		}
	})

	t.Run("someone else's token is 404", func(t *testing.T) {
		store := &fakeAPITokenStore{revokeErr: repository.ErrNotFound}
		router := apiTokenRouter(store, sessionCtx())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete,
			"/me/api-tokens/33333333-3333-3333-3333-333333333333", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}

func TestNormalizeAPITokenScopes(t *testing.T) {
	if _, err := normalizeAPITokenScopes([]string{"chat:write", "nope"}); err == nil {
		t.Fatalf("expected an unknown scope to be rejected")
	}
	scopes, err := normalizeAPITokenScopes([]string{" engagement:write "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 1 || scopes[0] != middleware.ScopeEngagementWrite {
		t.Fatalf("unexpected normalization result: %v", scopes)
	}
}
