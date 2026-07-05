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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockEntitlementWriter records calls and returns canned errors.
type mockEntitlementWriter struct {
	premiumUserID string
	premiumValue  bool
	premiumTTL    *time.Duration
	premiumCalled bool
	premiumErr    error

	betaUserID string
	betaValue  bool
	betaErr    error
}

func (m *mockEntitlementWriter) UpdateUserPremium(_ context.Context, userID string, isPremium bool, ttl *time.Duration) error {
	m.premiumUserID = userID
	m.premiumValue = isPremium
	m.premiumTTL = ttl
	m.premiumCalled = true
	return m.premiumErr
}

func (m *mockEntitlementWriter) SetUserBetaTester(_ context.Context, userID string, isBetaTester bool) error {
	m.betaUserID = userID
	m.betaValue = isBetaTester
	return m.betaErr
}

// betaRouter wires SetUserBetaTester behind the same param shape as production
// (/admin/beta-tester/users/:id), injecting an admin user_id like JWTAuth would.
func betaRouter(repo userEntitlementWriter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(repo, zap.NewNop())
	r.POST("/admin/beta-tester/users/:id", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Next()
	}, h.SetUserBetaTester)
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSetUserBetaTester_Grant(t *testing.T) {
	repo := &mockEntitlementWriter{}
	w := postJSON(betaRouter(repo), "/admin/beta-tester/users/u-42", `{"is_beta_tester":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u-42", repo.betaUserID)
	assert.True(t, repo.betaValue, "handler should pass is_beta_tester=true to the repo")
}

func TestSetUserBetaTester_Revoke(t *testing.T) {
	repo := &mockEntitlementWriter{}
	w := postJSON(betaRouter(repo), "/admin/beta-tester/users/u-42", `{"is_beta_tester":false}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, repo.betaValue, "handler should pass is_beta_tester=false to the repo")
}

func TestSetUserBetaTester_BadBody(t *testing.T) {
	repo := &mockEntitlementWriter{}
	w := postJSON(betaRouter(repo), "/admin/beta-tester/users/u-42", `not-json`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, repo.betaUserID, "repo must not be called on a malformed body")
}

func TestSetUserBetaTester_NotFound(t *testing.T) {
	repo := &mockEntitlementWriter{betaErr: errors.New("user not found: u-42")}
	w := postJSON(betaRouter(repo), "/admin/beta-tester/users/u-42", `{"is_beta_tester":true}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetUserBetaTester_InternalError(t *testing.T) {
	repo := &mockEntitlementWriter{betaErr: errors.New("db down")}
	w := postJSON(betaRouter(repo), "/admin/beta-tester/users/u-42", `{"is_beta_tester":true}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// premiumRouter wires SetUserPremium behind the production param shape
// (/admin/premium/users/:id), injecting an admin user_id like JWTAuth would.
func premiumRouter(repo userEntitlementWriter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(repo, zap.NewNop())
	r.POST("/admin/premium/users/:id", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Next()
	}, h.SetUserPremium)
	return r
}

// TestSetUserPremium_NotFound locks in the prefix-match fix: the repo returns
// "user not found: <id>", which must map to 404 (previously fell through to 500).
func TestSetUserPremium_NotFound(t *testing.T) {
	repo := &mockEntitlementWriter{premiumErr: errors.New("user not found: u-42")}
	w := postJSON(premiumRouter(repo), "/admin/premium/users/u-42", `{"is_premium":true}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSetUserPremium_PermanentGrant: no duration_seconds => permanent grant (nil ttl).
func TestSetUserPremium_PermanentGrant(t *testing.T) {
	repo := &mockEntitlementWriter{}
	w := postJSON(premiumRouter(repo), "/admin/premium/users/u-42", `{"is_premium":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u-42", repo.premiumUserID)
	assert.True(t, repo.premiumValue)
	assert.Nil(t, repo.premiumTTL, "a grant with no duration must pass a nil ttl (permanent)")
}

// TestSetUserPremium_TimeLimitedGrant: a positive duration_seconds becomes the ttl.
func TestSetUserPremium_TimeLimitedGrant(t *testing.T) {
	repo := &mockEntitlementWriter{}
	w := postJSON(premiumRouter(repo), "/admin/premium/users/u-42", `{"is_premium":true,"duration_seconds":604800}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, repo.premiumValue)
	if assert.NotNil(t, repo.premiumTTL, "a positive duration must be passed as a ttl") {
		assert.Equal(t, 7*24*time.Hour, *repo.premiumTTL)
	}
}

// TestSetUserPremium_RejectsNonPositiveDuration: zero/negative durations are 400.
func TestSetUserPremium_RejectsNonPositiveDuration(t *testing.T) {
	for _, body := range []string{`{"is_premium":true,"duration_seconds":0}`, `{"is_premium":true,"duration_seconds":-5}`} {
		repo := &mockEntitlementWriter{}
		w := postJSON(premiumRouter(repo), "/admin/premium/users/u-42", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.False(t, repo.premiumCalled, "repo must not be called for an invalid duration: %s", body)
	}
}

// TestSetUserPremium_RejectsOverCapDuration: durations beyond the ~10y cap are 400.
func TestSetUserPremium_RejectsOverCapDuration(t *testing.T) {
	repo := &mockEntitlementWriter{}
	// 11 years in seconds, well over the 10y cap.
	w := postJSON(premiumRouter(repo), "/admin/premium/users/u-42", `{"is_premium":true,"duration_seconds":346896000}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, repo.premiumCalled, "repo must not be called for an over-cap duration")
}
