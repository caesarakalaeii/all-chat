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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/google/uuid"
)

// revokeRecorder stands in for the provider's revoke endpoint and records each
// token it is sent.
type revokeRecorder struct {
	mu     sync.Mutex
	tokens []string
}

func (r *revokeRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	form, _ := url.ParseQuery(string(body))
	r.mu.Lock()
	r.tokens = append(r.tokens, req.URL.Host+":"+form.Get("token"))
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

type prefixCipher struct{}

func (prefixCipher) Encrypt(s string) (string, error) { return "enc:" + s, nil }
func (prefixCipher) Decrypt(s string) (string, error) { return strings.TrimPrefix(s, "enc:"), nil }

func TestHandleUnlinkPlatform_RevokesTheWipedTokensAtTheProvider(t *testing.T) {
	viewerID := uuid.New()
	mock := &mockViewerIdentityRepo{
		unlinkPlatform: func(context.Context, uuid.UUID, string) ([]repository.RemovedToken, error) {
			return []repository.RemovedToken{{AccessToken: "enc:viewer-access", RefreshToken: "enc:viewer-refresh"}}, nil
		},
	}
	recorder := &revokeRecorder{}
	twitch := oauth.NewViewerTwitchOAuth("client", "secret", "http://localhost/cb")
	twitch.TwitchOAuth = twitch.TwitchOAuth.WithHTTPClient(&http.Client{Transport: recorder})

	r, h := newLinkedPlatformsRouter(mock)
	h.twitchProvider = twitch
	h.cipher = prefixCipher{}

	req := httptest.NewRequest(http.MethodDelete, "/viewer/linked-platforms/twitch", nil)
	req.Header.Set("X-Viewer-ID", viewerID.String())
	req.Header.Set("X-Platform", "youtube")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	sort.Strings(recorder.tokens)
	want := []string{"id.twitch.tv:viewer-access", "id.twitch.tv:viewer-refresh"}
	if len(recorder.tokens) != 2 || recorder.tokens[0] != want[0] || recorder.tokens[1] != want[1] {
		t.Errorf("revoked = %v, want the decrypted tokens at Twitch: %v", recorder.tokens, want)
	}
}

// A platform whose viewer OAuth client is not configured is still unlinked; there
// is just nothing to revoke with.
func TestHandleUnlinkPlatform_NoProviderConfigured(t *testing.T) {
	mock := &mockViewerIdentityRepo{
		unlinkPlatform: func(context.Context, uuid.UUID, string) ([]repository.RemovedToken, error) {
			return []repository.RemovedToken{{AccessToken: "enc:x"}}, nil
		},
	}
	r, _ := newLinkedPlatformsRouter(mock)

	req := httptest.NewRequest(http.MethodDelete, "/viewer/linked-platforms/kick", nil)
	req.Header.Set("X-Viewer-ID", uuid.New().String())
	req.Header.Set("X-Platform", "twitch")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}
