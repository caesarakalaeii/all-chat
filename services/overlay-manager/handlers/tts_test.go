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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	ttspkg "github.com/caesar/all-chat/services/overlay-manager/tts"
	"github.com/caesar/all-chat/shared/encryption"
	sharedMiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---- Test doubles ----------------------------------------------------------

// mockTTSRepo is an in-memory implementation of ttsConfigStore used across
// the handler test suite. Not goroutine-safe; tests are sequential.
type mockTTSRepo struct {
	mu       sync.Mutex
	row      *models.TTSConfig
	notFound bool
}

func (m *mockTTSRepo) GetByOverlayID(_ context.Context, _ string) (*models.TTSConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.notFound || m.row == nil {
		return nil, repository.ErrTTSConfigNotFound
	}
	copied := *m.row
	copied.EncryptedAPIKey = append([]byte(nil), m.row.EncryptedAPIKey...)
	copied.SigningSecret = append([]byte(nil), m.row.SigningSecret...)
	return &copied, nil
}

func (m *mockTTSRepo) CreateOrUpdate(_ context.Context, overlayID string, encryptedKey []byte, voiceID string) (*models.TTSConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		// Mint a deterministic 32-byte signing secret for tests.
		secret := bytes.Repeat([]byte{0x11}, 32)
		m.row = &models.TTSConfig{
			ID:              "row-id",
			OverlayID:       overlayID,
			EncryptedAPIKey: append([]byte(nil), encryptedKey...),
			VoiceID:         voiceID,
			SigningSecret:   secret,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
	} else {
		m.row.EncryptedAPIKey = append([]byte(nil), encryptedKey...)
		m.row.VoiceID = voiceID
		m.row.UpdatedAt = time.Now()
	}
	m.notFound = false
	copied := *m.row
	return &copied, nil
}

func (m *mockTTSRepo) Delete(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		return repository.ErrTTSConfigNotFound
	}
	m.row = nil
	m.notFound = true
	return nil
}

func (m *mockTTSRepo) RotateSigningSecret(_ context.Context, _ string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.row == nil {
		return nil, repository.ErrTTSConfigNotFound
	}
	// Rotate to a well-known byte that differs from the initial secret.
	m.row.SigningSecret = bytes.Repeat([]byte{0x22}, 32)
	m.row.UpdatedAt = time.Now()
	return append([]byte(nil), m.row.SigningSecret...), nil
}

// mockOverlays lets tests control whether ownership checks succeed.
type mockOverlays struct {
	owned bool
}

func (m *mockOverlays) GetByIDAndUserID(_ context.Context, overlayID, _ string) (*models.Overlay, error) {
	if !m.owned {
		return nil, errors.New("overlay not found or unauthorized")
	}
	return &models.Overlay{ID: overlayID, UserID: "user-1", Name: "test"}, nil
}

// mockGateChecker implements sharedMiddleware.GateChecker.
type mockGateChecker struct {
	isPremiumResult bool
}

func (m *mockGateChecker) IsPremium(_ string) bool { return m.isPremiumResult }

// ---- Shared fixtures -------------------------------------------------------

func testCipher(t *testing.T) *encryption.AESEncryptor {
	t.Helper()
	key := bytes.Repeat([]byte{0x01}, 32)
	c, err := encryption.NewAESEncryptor(key)
	require.NoError(t, err)
	return c
}

type tttFixture struct {
	repo       *mockTTSRepo
	overlays   *mockOverlays
	cipher     *encryption.AESEncryptor
	handler    *TTSHandler
	upstreamTS *httptest.Server
}

// newTestHandler builds a TTSHandler with all deps mocked. Caller is
// responsible for closing upstreamTS if it is non-nil.
func newTestHandler(t *testing.T, upstream http.HandlerFunc) *tttFixture {
	t.Helper()
	f := &tttFixture{
		repo:     &mockTTSRepo{},
		overlays: &mockOverlays{owned: true},
		cipher:   testCipher(t),
	}
	if upstream != nil {
		f.upstreamTS = httptest.NewServer(upstream)
	}
	base := ""
	if f.upstreamTS != nil {
		base = f.upstreamTS.URL
	}
	f.handler = NewTTSHandler(f.repo, f.overlays, f.cipher, "https://allch.at", zap.NewNop())
	if base != "" {
		// Override the compile-time ElevenLabs base URL so the handler calls
		// the local mock server for this test.
		f.handler.elevenLabsBaseURL = base
	}
	return f
}

// newRouter mounts the full set of TTS routes with a tiny auth-shim that
// injects user_id and (optionally) runs the real RequirePremium middleware.
func newRouter(t *testing.T, h *TTSHandler, gates sharedMiddleware.GateChecker, userPremium bool, userID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authShim := func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	}

	premium := sharedMiddleware.RequirePremiumWithQuerier(gates, "tts", func(_ context.Context, _ string) (bool, error) {
		return userPremium, nil
	}, nil)

	r.GET("/:id/tts-config", authShim, h.HandleGetTTSConfig)

	gated := r.Group("/")
	gated.Use(authShim, premium)
	{
		gated.POST("/:id/tts-config", h.HandleSaveTTSConfig)
		gated.DELETE("/:id/tts-config", h.HandleDeleteTTSConfig)
		gated.POST("/:id/tts-config/rotate-token", h.HandleRotateToken)
		gated.GET("/:id/tts-voices", h.HandleGetVoices)
		gated.POST("/:id/tts-voices/preview", h.HandleGetVoicesPreview)
		gated.POST("/:id/tts-config/test", h.HandleTestKey)
	}

	// POST /tts uses tts_token JWT auth inside the handler, not user JWT.
	r.POST("/:id/tts", h.HandleTTS)
	return r
}

// ---- Tests -----------------------------------------------------------------

// TestSaveTTSConfigRequiresPremium — non-premium user hitting POST
// /tts-config gets 403 from the middleware.
func TestSaveTTSConfigRequiresPremium(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, false, "user-1")

	body := strings.NewReader(`{"api_key":"sk_test","voice_id":"v1"}`)
	req := httptest.NewRequest(http.MethodPost, "/overlay-1/tts-config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestSaveTTSConfigEncryptsKey — premium user saves a key; the blob written
// to the repo is AES-GCM ciphertext and decrypting it yields the original
// plaintext.
func TestSaveTTSConfigEncryptsKey(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	body := strings.NewReader(`{"api_key":"sk_secret_key","voice_id":"v-main"}`)
	req := httptest.NewRequest(http.MethodPost, "/overlay-abc/tts-config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.NotNil(t, f.repo.row)
	require.NotEmpty(t, f.repo.row.EncryptedAPIKey)

	// Round-trip through the cipher.
	plaintext, err := f.cipher.DecryptString(string(f.repo.row.EncryptedAPIKey))
	require.NoError(t, err)
	assert.Equal(t, "sk_secret_key", plaintext)
	assert.NotContains(t, string(f.repo.row.EncryptedAPIKey), "sk_secret_key",
		"ciphertext must not contain plaintext substring")
}

// TestSaveTTSConfigRejectsNonOwner — ownership check rejects other users.
func TestSaveTTSConfigRejectsNonOwner(t *testing.T) {
	f := newTestHandler(t, nil)
	f.overlays.owned = false
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	body := strings.NewReader(`{"api_key":"sk","voice_id":"v"}`)
	req := httptest.NewRequest(http.MethodPost, "/overlay-stranger/tts-config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDeleteTTSConfig — DELETE returns 204 and the row is gone.
func TestDeleteTTSConfig(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	// First save so there's something to delete.
	save := httptest.NewRequest(http.MethodPost, "/overlay-del/tts-config",
		strings.NewReader(`{"api_key":"k","voice_id":"v"}`))
	save.Header.Set("Content-Type", "application/json")
	saveW := httptest.NewRecorder()
	r.ServeHTTP(saveW, save)
	require.Equal(t, http.StatusOK, saveW.Code)

	// Delete.
	del := httptest.NewRequest(http.MethodDelete, "/overlay-del/tts-config", nil)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, del)
	assert.Equal(t, http.StatusNoContent, delW.Code)

	// Now GET /tts-config reflects the deletion.
	get := httptest.NewRequest(http.MethodGet, "/overlay-del/tts-config", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, get)
	require.Equal(t, http.StatusOK, getW.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["has_elevenlabs_config"])
}

// TestRotateTokenReturnsNewURL — rotate returns a new obs_url whose token
// verifies against the (new) signing secret and fails against the old one.
func TestRotateTokenReturnsNewURL(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	save := httptest.NewRequest(http.MethodPost, "/overlay-rot/tts-config",
		strings.NewReader(`{"api_key":"k","voice_id":"v"}`))
	save.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), save)

	oldSecret := append([]byte(nil), f.repo.row.SigningSecret...)

	rot := httptest.NewRequest(http.MethodPost, "/overlay-rot/tts-config/rotate-token", nil)
	rotW := httptest.NewRecorder()
	r.ServeHTTP(rotW, rot)

	require.Equal(t, http.StatusOK, rotW.Code, "body=%s", rotW.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rotW.Body.Bytes(), &resp))
	obsURL, ok := resp["obs_url"].(string)
	require.True(t, ok, "obs_url missing")
	assert.Contains(t, obsURL, "tts_token=")
	assert.Contains(t, obsURL, "/overlay/overlay-rot")

	// Parse the token out of the URL.
	idx := strings.Index(obsURL, "tts_token=")
	require.Greater(t, idx, -1)
	token := obsURL[idx+len("tts_token="):]

	// Should verify against the NEW secret.
	newSecret := f.repo.row.SigningSecret
	require.NotEqual(t, oldSecret, newSecret)
	assert.NoError(t, ttspkg.VerifyOverlayToken(token, "overlay-rot", newSecret))
	assert.Error(t, ttspkg.VerifyOverlayToken(token, "overlay-rot", oldSecret),
		"new token must NOT verify against the old secret")
}

// TestGetVoicesProxies — voice list is fetched from the (mocked) upstream
// with the xi-api-key header and returned verbatim to the client.
func TestGetVoicesProxies(t *testing.T) {
	var gotHeader string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/voices" {
			gotHeader = r.Header.Get("xi-api-key")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"voices":[{"voice_id":"v1"},{"voice_id":"v2"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	// Seed a saved key.
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-voices/tts-config",
			strings.NewReader(`{"api_key":"sk_voices","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodGet, "/overlay-voices/tts-voices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "sk_voices", gotHeader, "decrypted key should be forwarded")
	assert.Contains(t, w.Body.String(), "voice_id")
}

// TestGetVoicesPreviewProxies — POST /:id/tts-voices/preview with a body
// {api_key:"sk_typed"} forwards that key as xi-api-key without persisting,
// and returns the upstream voices payload verbatim. Crucially, the repo is
// untouched so the chicken-and-egg with HandleSaveTTSConfig is broken.
func TestGetVoicesPreviewProxies(t *testing.T) {
	var gotHeader string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/voices" {
			gotHeader = r.Header.Get("xi-api-key")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"voices":[{"voice_id":"vp1"},{"voice_id":"vp2"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	body := strings.NewReader(`{"api_key":"sk_typed"}`)
	req := httptest.NewRequest(http.MethodPost, "/overlay-prev/tts-voices/preview", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "sk_typed", gotHeader, "typed key must be forwarded as xi-api-key")
	assert.Contains(t, w.Body.String(), "voice_id")
	assert.Nil(t, f.repo.row, "preview must not persist anything to the repo")
}

// TestGetVoicesPreviewRequiresAPIKey — empty body or missing api_key returns
// 400 without calling upstream.
func TestGetVoicesPreviewRequiresAPIKey(t *testing.T) {
	calls := 0
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	cases := []string{`{"api_key":""}`, `{"api_key":"   "}`, `{}`}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/overlay-bad/tts-voices/preview",
			strings.NewReader(c))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body case=%q", c)
	}
	assert.Equal(t, 0, calls, "upstream must not be called when api_key is missing")
}

// TestGetVoicesPreviewMapsUpstream401 — invalid key surfaces as 401 with the
// human-readable copy the frontend toasts on.
func TestGetVoicesPreviewMapsUpstream401(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	req := httptest.NewRequest(http.MethodPost, "/overlay-401/tts-voices/preview",
		strings.NewReader(`{"api_key":"sk_bad"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid API key")
}

// TestGetVoicesPreviewRequiresPremium — the new endpoint must sit behind the
// same RequirePremium gate as the rest of the TTS surface.
func TestGetVoicesPreviewRequiresPremium(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, false, "user-1")

	req := httptest.NewRequest(http.MethodPost, "/overlay-1/tts-voices/preview",
		strings.NewReader(`{"api_key":"sk"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestGetVoicesPreviewRejectsNonOwner — ownership check still applies.
func TestGetVoicesPreviewRejectsNonOwner(t *testing.T) {
	f := newTestHandler(t, nil)
	f.overlays.owned = false
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	req := httptest.NewRequest(http.MethodPost, "/overlay-stranger/tts-voices/preview",
		strings.NewReader(`{"api_key":"sk"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestTestKeyHandler_Success — upstream returns 200 for GET
// /v1/user/subscription AND audio/mpeg for the sample POST — handler returns
// audio/mpeg + x-characters-* headers.
func TestTestKeyHandler_Success(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/user/subscription":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"character_count":1000,"character_limit":10000}`))
		case strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/"):
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-mp3-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-testk/tts-config",
			strings.NewReader(`{"api_key":"sk_ok","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodPost, "/overlay-testk/tts-config/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "9000", w.Header().Get("x-characters-remaining"))
	assert.Equal(t, "10000", w.Header().Get("x-characters-limit"))
	assert.Equal(t, "fake-mp3-bytes", w.Body.String())
}

// TestTestKeyHandler_401 — verbose error copy (D-39).
func TestTestKeyHandler_401(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/user/subscription" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-401/tts-config",
			strings.NewReader(`{"api_key":"sk_bad","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodPost, "/overlay-401/tts-config/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid API key")
}

// TestTestKeyHandler_429 — verbose error copy (D-39).
func TestTestKeyHandler_429(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/user/subscription" {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-429/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodPost, "/overlay-429/tts-config/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Rate-limited — try again in a minute")
}

// TestTestKeyHandler_5xx — verbose error copy (D-39).
func TestTestKeyHandler_5xx(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-5xx/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodPost, "/overlay-5xx/tts-config/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "ElevenLabs service unavailable")
}

// TestHandleTTSStreamsAudioMpeg — happy path streaming proxy.
func TestHandleTTSStreamsAudioMpeg(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("streamed-audio-bytes"))
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	// Save config so the signing secret exists.
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-stream/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	// Sign a valid tts_token for this overlay.
	token, err := ttspkg.SignOverlayToken("overlay-stream", f.repo.row.SigningSecret)
	require.NoError(t, err)

	url := fmt.Sprintf("/overlay-stream/tts?text=hello&voice=v-foo&tts_token=%s", token)
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "streamed-audio-bytes", w.Body.String())
}

// TestHandleTTSRequiresToken — missing or invalid tts_token → 401.
func TestHandleTTSRequiresToken(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-noauth/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	// No token.
	req := httptest.NewRequest(http.MethodPost, "/overlay-noauth/tts?text=hi", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Garbage token.
	req2 := httptest.NewRequest(http.MethodPost, "/overlay-noauth/tts?text=hi&tts_token=garbage", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

// TestHandleTTSRateLimited — >60 calls in 60s returns 429 on the 61st.
func TestHandleTTSRateLimited(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-rl/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	token, err := ttspkg.SignOverlayToken("overlay-rl", f.repo.row.SigningSecret)
	require.NoError(t, err)
	url := fmt.Sprintf("/overlay-rl/tts?text=hi&tts_token=%s", token)

	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodPost, url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i+1)
	}

	// 61st must be 429.
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestHandleTTSStillWorksForDowngradedPremium — POST /tts has NO premium
// gate; a user with is_premium=false can still serve TTS as long as the
// tts_token validates. This is the graceful-permanence contract.
func TestHandleTTSStillWorksForDowngradedPremium(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("still-playing"))
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	// Step 1: save as premium.
	rPremium := newRouter(t, f.handler, gates, true, "user-1")
	rPremium.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-graceful/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	token, err := ttspkg.SignOverlayToken("overlay-graceful", f.repo.row.SigningSecret)
	require.NoError(t, err)

	// Step 2: user downgrades (userPremium=false) — POST /tts still serves
	// because POST /tts does NOT go through the premium gate.
	rDowngraded := newRouter(t, f.handler, gates, false, "user-1")
	url := fmt.Sprintf("/overlay-graceful/tts?text=hi&tts_token=%s", token)
	req := httptest.NewRequest(http.MethodPost, url, nil)
	w := httptest.NewRecorder()
	rDowngraded.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "still-playing", w.Body.String())
}

// TestHandleTTSUsesCfgVoiceIDWhenQueryParamMissing — if ?voice= is absent the
// handler falls back to cfg.VoiceID.
func TestHandleTTSUsesCfgVoiceIDWhenQueryParamMissing(t *testing.T) {
	var gotPath string
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-voicefb/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"xyz"}`)))

	token, err := ttspkg.SignOverlayToken("overlay-voicefb", f.repo.row.SigningSecret)
	require.NoError(t, err)

	// No ?voice= in query — handler must use cfg.VoiceID = "xyz".
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/overlay-voicefb/tts?text=hello&tts_token=%s", token), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, gotPath, "/v1/text-to-speech/xyz/stream",
		"upstream request must target /v1/text-to-speech/xyz/stream, got %s", gotPath)
}

// TestHandleTTSCancelPropagates — cancel the client context mid-flight; the
// upstream sees the connection close.
func TestHandleTTSCancelPropagates(t *testing.T) {
	upstreamGotDone := make(chan bool, 1)
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait until client disconnects (r.Context().Done closes).
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("part1"))
		if flusher != nil {
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			upstreamGotDone <- true
		case <-time.After(5 * time.Second):
			upstreamGotDone <- false
		}
	})
	f := newTestHandler(t, upstream)
	defer f.upstreamTS.Close()

	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")
	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-cancel/tts-config",
			strings.NewReader(`{"api_key":"sk","voice_id":"v1"}`)))

	token, err := ttspkg.SignOverlayToken("overlay-cancel", f.repo.row.SigningSecret)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/overlay-cancel/tts?text=hi&tts_token=%s", token), nil).WithContext(ctx)

	w := httptest.NewRecorder()
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	r.ServeHTTP(w, req)

	select {
	case got := <-upstreamGotDone:
		assert.True(t, got, "upstream should see client context cancellation")
	case <-time.After(6 * time.Second):
		t.Fatal("upstream timeout without seeing cancellation")
	}
}

// TestGetTTSConfigHidesKey — GET /tts-config response never includes
// api_key, encrypted_api_key, or tts_signing_secret.
func TestGetTTSConfigHidesKey(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	r.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/overlay-hide/tts-config",
			strings.NewReader(`{"api_key":"sk_super_secret","voice_id":"v1"}`)))

	req := httptest.NewRequest(http.MethodGet, "/overlay-hide/tts-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "has_elevenlabs_config")
	assert.Contains(t, body, "voice_id")
	assert.Contains(t, body, "obs_url")
	assert.NotContains(t, body, "api_key", "response must not include api_key")
	assert.NotContains(t, body, "encrypted_api_key", "response must not include encrypted_api_key")
	assert.NotContains(t, body, "tts_signing_secret", "response must not include tts_signing_secret")
	assert.NotContains(t, body, "sk_super_secret", "response must not include plaintext key")
}

// TestGetTTSConfigWhenAbsent — returns 200 {has_elevenlabs_config:false}
// instead of 404 (valid "no key yet" state — D-24).
func TestGetTTSConfigWhenAbsent(t *testing.T) {
	f := newTestHandler(t, nil)
	gates := &mockGateChecker{isPremiumResult: true}
	r := newRouter(t, f.handler, gates, true, "user-1")

	req := httptest.NewRequest(http.MethodGet, "/overlay-empty/tts-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["has_elevenlabs_config"])
}

// TestPublicConfigHidesTTSKey — Regression for T-13-06. The public config
// endpoint's response schema must never include TTS-related fields. This
// calls the EXISTING HandleGetPublicConfig on a ConfigHandler; TTS data
// lives in a separate table, so no accidental leak can happen unless a
// future developer couples the two.
//
// Implemented as a schema assertion on the json keys of the existing
// handler's response: we build a minimal ConfigHandler with a stub repo
// that returns an OverlayConfig with DisplaySettings containing a
// tts_-prefixed field, call HandleGetPublicConfig, and assert neither
// api_key, encrypted_api_key, nor tts_signing_secret appear in the
// response body.
func TestPublicConfigHidesTTSKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Stub config repo that returns a config with a tts-named display
	// setting — the leak risk would be if some code path copied the
	// tts_signing_secret / encrypted_api_key values into the public response.
	cfg := &models.OverlayConfig{
		ID:              "c1",
		OverlayID:       "overlay-pub",
		DisplaySettings: map[string]any{"tts_enabled": true, "tts_volume": 0.8},
		FilterSettings:  map[string]any{},
		VisualSettings:  map[string]any{},
	}

	cfgRepo := &stubConfigRepo{cfg: cfg}
	overlays := &stubOverlayRepo{owned: true}
	sources := &stubSourceRepo{}
	ch := NewConfigHandler(cfgRepo, overlays, sources)

	r := gin.New()
	r.GET("/public/:id/config", ch.HandleGetPublicConfig)

	req := httptest.NewRequest(http.MethodGet, "/public/overlay-pub/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "encrypted_api_key")
	assert.NotContains(t, body, "tts_signing_secret")
	assert.NotContains(t, body, "api_key")
}

// ---- Stubs for TestPublicConfigHidesTTSKey ---------------------------------
//
// These stubs implement the full OverlayRepository / SourceRepository /
// OverlayConfigRepository interfaces so NewConfigHandler accepts them. Only
// the methods the public-config path exercises do useful work; the rest
// return zero values.

type stubConfigRepo struct{ cfg *models.OverlayConfig }

func (s *stubConfigRepo) GetByOverlayID(_ context.Context, _ string) (*models.OverlayConfig, error) {
	if s.cfg == nil {
		return nil, errors.New("not found")
	}
	return s.cfg, nil
}
func (s *stubConfigRepo) Update(_ context.Context, _ *models.OverlayConfig) error {
	return nil
}

type stubOverlayRepo struct{ owned bool }

func (s *stubOverlayRepo) Create(_ context.Context, _ *models.Overlay) error { return nil }
func (s *stubOverlayRepo) GetByID(_ context.Context, id string) (*models.Overlay, error) {
	return &models.Overlay{ID: id, UserID: "u1"}, nil
}
func (s *stubOverlayRepo) GetByIDAndUserID(_ context.Context, id, _ string) (*models.Overlay, error) {
	if !s.owned {
		return nil, errors.New("not owned")
	}
	return &models.Overlay{ID: id, UserID: "u1"}, nil
}
func (s *stubOverlayRepo) ListByUserID(_ context.Context, _ string) ([]*models.Overlay, error) {
	return nil, nil
}
func (s *stubOverlayRepo) Update(_ context.Context, _ *models.Overlay) error { return nil }
func (s *stubOverlayRepo) Delete(_ context.Context, _ string) error          { return nil }
func (s *stubOverlayRepo) UnsetAllPublicForUser(_ context.Context, _, _ string) error {
	return nil
}

type stubSourceRepo struct{}

func (s *stubSourceRepo) Create(_ context.Context, _ *models.ChatSource) error { return nil }
func (s *stubSourceRepo) ListByOverlayID(_ context.Context, _ string) ([]*models.ChatSource, error) {
	return nil, nil
}
func (s *stubSourceRepo) GetByID(_ context.Context, _ string) (*models.ChatSource, error) {
	return nil, errors.New("not impl")
}
func (s *stubSourceRepo) Delete(_ context.Context, _ string) error { return nil }
func (s *stubSourceRepo) UpdateConfig(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}

// Ensure the file compiles under the handlers package even if downstream
// consumers rename interfaces — referencing io.Copy keeps the dep.
var _ = io.Copy
