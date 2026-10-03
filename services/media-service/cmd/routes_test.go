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

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/media-service/handlers"
	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/caesar/all-chat/services/media-service/storage"
	sharedauth "github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// These tests pin the production router wiring through registerMediaRoutes:
// that every media route sits behind JWTAuthWithRevocation (a valid JWT is the
// ONLY way in — issue #949 forbids any auth bypass) and behind RequireStore
// (no MinIO means 503). The handler tests in handlers/ inject user_id in a
// parallel router, so they cannot see a missing guard here; these can.
//
// The refusal cases assert on the middleware's error body, not just the 401:
// the handlers also answer 401 when user_id is absent, so without the body an
// unguarded router looks exactly like a guarded one on those requests. The
// accept cases are the real discriminator — with the guard deleted, a valid
// token never gets user_id into context and every route answers 401.

const (
	routeUserID      = "user-1"
	routeSeededKey   = "user-1/0d3f1d6a-0000-4000-8000-000000000001/horn.mp3"
	routePresignBody = `{"filename":"horn.mp3","content_type":"audio/mpeg","size":12345}`
	routePresignURL  = "https://minio.example/upload/horn.mp3"
	routeWrongSecret = "a-different-signing-secret-entirely-01"
)

var routeMediaConfig = handlers.MediaConfig{
	PublicBaseURL:     "https://media.allch.at",
	PresignExpiry:     5 * time.Minute,
	MaxObjectsPerUser: 50,
}

// fakeRegistry is an in-memory handlers.MediaRegistry: just enough state for
// the three routes to answer successfully.
type fakeRegistry struct {
	objects []models.MediaObject
}

func (f *fakeRegistry) Create(_ context.Context, obj *models.MediaObject) error {
	f.objects = append(f.objects, *obj)
	return nil
}

func (f *fakeRegistry) CountByUser(_ context.Context, userID string) (int, error) {
	n := 0
	for _, o := range f.objects {
		if o.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRegistry) ListByUser(_ context.Context, userID string) ([]models.MediaObject, error) {
	var out []models.MediaObject
	for _, o := range f.objects {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeRegistry) DeleteByOwner(_ context.Context, userID, objectKey string) (bool, error) {
	for i, o := range f.objects {
		if o.UserID == userID && o.ObjectKey == objectKey {
			f.objects = append(f.objects[:i], f.objects[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// fakeStore is an in-memory handlers.ObjectStore that always presigns the
// same URL.
type fakeStore struct{}

func (f *fakeStore) Available() bool { return true }

func (f *fakeStore) PresignPut(_ context.Context, _ string, _ time.Duration) (string, error) {
	return routePresignURL, nil
}

func (f *fakeStore) Remove(_ context.Context, _ string) error { return nil }

// newTestKeyChain builds the key chain shape shared/middleware's own tests
// use: one versioned secret plus a legacy fallback.
func newTestKeyChain(t *testing.T) *sharedauth.KeyChain {
	t.Helper()
	return sharedauth.NewKeyChain(
		map[string][]byte{"v1": []byte("test-secret-v1-0123456789abcdef")},
		[]byte("test-secret-legacy-0123456789ab"),
		"v1",
	)
}

// mintUserToken signs a user JWT for routeUserID with the given secret.
func mintUserToken(t *testing.T, kid, secret string) string {
	t.Helper()
	token, err := sharedauth.GenerateTokenWithKid(kid, routeUserID, "testuser", secret, time.Hour, false)
	require.NoError(t, err)
	return token
}

// newTestRouter wires the media routes exactly as main() does — through
// registerMediaRoutes, not a parallel router — over an in-memory registry and
// the given object store.
func newTestRouter(t *testing.T, kc *sharedauth.KeyChain, store handlers.ObjectStore) (*gin.Engine, *fakeRegistry) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registry := &fakeRegistry{objects: []models.MediaObject{{
		UserID:      routeUserID,
		ObjectKey:   routeSeededKey,
		Filename:    "horn.mp3",
		ContentType: "audio/mpeg",
		SizeBytes:   12345,
	}}}
	router := gin.New()
	registerMediaRoutes(router, kc, nil,
		handlers.NewMediaHandler(registry, store, routeMediaConfig, zap.NewNop()))
	return router, registry
}

func doRouteRequest(router *gin.Engine, method, path, authHeader, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// mediaRouteCases enumerates the three registered routes.
var mediaRouteCases = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"presign", http.MethodPost, "/api/v1/media/presign", routePresignBody},
	{"list", http.MethodGet, "/api/v1/media", ""},
	{"delete", http.MethodDelete, "/api/v1/media/" + routeSeededKey, ""},
}

func TestRegisterMediaRoutes_RefusesRequestsWithoutAValidJWT(t *testing.T) {
	kc := newTestKeyChain(t)
	router, _ := newTestRouter(t, kc, &fakeStore{})

	// Signed with the right shape but a secret the key chain does not hold:
	// must be refused like a garbage token, not admitted as some user.
	wrongSecretToken := mintUserToken(t, kc.LatestKid(), routeWrongSecret)

	refusals := []struct {
		name     string
		auth     string
		wantBody string
	}{
		{"no authorization header", "", "Authorization header required"},
		{"not a bearer token", "Basic dXNlcjpwYXNz", "Invalid authorization header format"},
		{"garbage bearer token", "Bearer not-even-a-jwt", "Invalid or expired token"},
		{"token signed with a foreign secret", "Bearer " + wrongSecretToken, "Invalid or expired token"},
	}

	for _, route := range mediaRouteCases {
		for _, refusal := range refusals {
			t.Run(route.name+"/"+refusal.name, func(t *testing.T) {
				w := doRouteRequest(router, route.method, route.path, refusal.auth, route.body)

				assert.Equal(t, http.StatusUnauthorized, w.Code,
					"every media route must refuse requests without a valid JWT")
				assert.Contains(t, w.Body.String(), refusal.wantBody,
					"the refusal must come from the JWT middleware, not the handler's own empty-user_id guard")
			})
		}
	}
}

func TestRegisterMediaRoutes_GatesMediaRoutesOnObjectStore(t *testing.T) {
	kc := newTestKeyChain(t)
	token := mintUserToken(t, kc.LatestKid(), string(kc.LatestSecret()))
	// storage.DisabledStore is exactly what main() serves with MINIO_ENDPOINT
	// unset; every media route must answer 503 through RequireStore.
	router, _ := newTestRouter(t, kc, storage.DisabledStore{})

	for _, route := range mediaRouteCases {
		t.Run(route.name, func(t *testing.T) {
			w := doRouteRequest(router, route.method, route.path, "Bearer "+token, route.body)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code,
				"media routes must 503 when the object store is not configured")
			assert.Contains(t, w.Body.String(), "media storage is not configured")
		})
	}
}

func TestRegisterMediaRoutes_AuthenticatedRequestsReachHandlers(t *testing.T) {
	kc := newTestKeyChain(t)
	token := mintUserToken(t, kc.LatestKid(), string(kc.LatestSecret()))
	router, _ := newTestRouter(t, kc, &fakeStore{})

	// Each expectation is the registered handler's success answer. A router
	// missing the JWT guard answers 401 here (no user_id reaches the handler),
	// a router missing a route answers 404, and a router missing RequireStore
	// fails the 503 cases above.
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"presign", http.MethodPost, "/api/v1/media/presign", routePresignBody,
			http.StatusCreated, `"object_key":"` + routeUserID + "/"},
		{"list", http.MethodGet, "/api/v1/media", "",
			http.StatusOK, `"media"`},
		{"delete", http.MethodDelete, "/api/v1/media/" + routeSeededKey, "",
			http.StatusOK, `"deleted"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRouteRequest(router, tc.method, tc.path, "Bearer "+token, tc.body)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tc.wantBody)
		})
	}
}