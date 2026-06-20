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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockEntitlementWriter records calls and returns canned errors.
type mockEntitlementWriter struct {
	premiumUserID string
	premiumValue  bool
	premiumErr    error

	betaUserID string
	betaValue  bool
	betaErr    error
}

func (m *mockEntitlementWriter) UpdateUserPremium(_ context.Context, userID string, isPremium bool) error {
	m.premiumUserID = userID
	m.premiumValue = isPremium
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

// TestSetUserPremium_NotFound locks in the prefix-match fix: the repo returns
// "user not found: <id>", which must map to 404 (previously fell through to 500).
func TestSetUserPremium_NotFound(t *testing.T) {
	repo := &mockEntitlementWriter{premiumErr: errors.New("user not found: u-42")}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(repo, zap.NewNop())
	r.POST("/admin/premium/users/:id", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Next()
	}, h.SetUserPremium)

	w := postJSON(r, "/admin/premium/users/u-42", `{"is_premium":true}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
