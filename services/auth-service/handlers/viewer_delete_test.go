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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type eraseCall struct {
	viewerID       uuid.UUID
	platform, puid string
}

// deleteViewerRequest runs HandleDeleteViewer with the context keys the JWT middleware
// sets for a viewer token (claims == nil models a streamer token: no viewer keys).
func deleteViewerRequest(t *testing.T, claims map[string]string, erase func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error)) (*httptest.ResponseRecorder, *miniredis.Miniredis, []eraseCall) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	var calls []eraseCall
	h := &ViewerAuthHandler{
		identityRepo: &mockViewerIdentityRepo{eraseViewer: func(ctx context.Context, id uuid.UUID, p, u string) ([]repository.LinkedPlatform, error) {
			calls = append(calls, eraseCall{id, p, u})
			return erase(ctx, id, p, u)
		}},
		redis:     rdb,
		jwtExpiry: time.Hour,
		logger:    zap.NewNop(),
	}

	r := gin.New()
	r.DELETE("/viewer/me", func(c *gin.Context) {
		for k, v := range claims {
			c.Set(k, v)
		}
		c.Next()
	}, h.HandleDeleteViewer)
	req := httptest.NewRequest(http.MethodDelete, "/viewer/me", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	// A streamer signed in in the same browser: the gateway forwards that session as
	// X-Access-Token on routes that use AuthCookieForward. It is a different login.
	req.Header.Set("X-Access-Token", "streamer-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, mr, calls
}

func viewerClaims(viewerID string) map[string]string {
	return map[string]string{
		"viewer_id": viewerID, "session_id": uuid.NewString(),
		"platform": "twitch", "platform_user_id": "123",
	}
}

func TestHandleDeleteViewer_ErasesAndRevokes(t *testing.T) {
	viewerID := uuid.New()
	w, mr, calls := deleteViewerRequest(t, viewerClaims(viewerID.String()),
		func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error) {
			return []repository.LinkedPlatform{{Platform: "twitch", PlatformUserID: "123"}, {Platform: "youtube", PlatformUserID: "UC1"}}, nil
		})

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(calls) != 1 || calls[0] != (eraseCall{viewerID, "twitch", "123"}) {
		t.Fatalf("EraseViewer calls = %+v, want one call for the token's viewer and identity", calls)
	}
	if !mr.Exists("blacklist:viewer-token") {
		t.Error("the viewer token must be revoked after erasure")
	}
	if mr.Exists("blacklist:streamer-token") {
		t.Error("deleting viewer data must not sign out a streamer session in the same browser")
	}
}

func TestHandleDeleteViewer_DropsIdentityCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	for _, k := range []string{"viewer:identity:twitch:123", "viewer:identity:youtube:UC1", "viewer:identity:kick:other"} {
		mr.Set(k, "{}")
	}

	dropViewerIdentityCache(context.Background(), rdb, zap.NewNop(), []repository.LinkedPlatform{
		{Platform: "twitch", PlatformUserID: "123"}, {Platform: "youtube", PlatformUserID: "UC1"},
	})

	if mr.Exists("viewer:identity:twitch:123") || mr.Exists("viewer:identity:youtube:UC1") {
		t.Error("erased identities must not keep serving cached cosmetics")
	}
	if !mr.Exists("viewer:identity:kick:other") {
		t.Error("another viewer's cache entry must stay")
	}
}

func TestHandleDeleteViewer_LegacyTokenWithoutViewerID(t *testing.T) {
	w, _, calls := deleteViewerRequest(t, viewerClaims(""),
		func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error) { return nil, nil })
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if len(calls) != 1 || calls[0].viewerID != uuid.Nil {
		t.Fatalf("a token without viewer_id must erase by platform identity, got %+v", calls)
	}
}

func TestHandleDeleteViewer_RejectsNonViewerToken(t *testing.T) {
	w, _, calls := deleteViewerRequest(t, map[string]string{"user_id": uuid.NewString()},
		func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error) { return nil, nil })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for a streamer token, got %d", w.Code)
	}
	if len(calls) != 0 {
		t.Fatal("nothing may be erased without a viewer token")
	}
}

func TestHandleDeleteViewer_RejectsMalformedViewerID(t *testing.T) {
	w, _, calls := deleteViewerRequest(t, viewerClaims("not-a-uuid"),
		func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error) { return nil, nil })
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if len(calls) != 0 {
		t.Fatal("nothing may be erased for a malformed token")
	}
}

func TestHandleDeleteViewer_FailureKeepsToken(t *testing.T) {
	w, mr, _ := deleteViewerRequest(t, viewerClaims(uuid.NewString()),
		func(context.Context, uuid.UUID, string, string) ([]repository.LinkedPlatform, error) {
			return nil, errors.New("db down")
		})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	if mr.Exists("blacklist:viewer-token") {
		t.Error("a failed erasure must not sign the viewer out: they need the token to retry")
	}
}
