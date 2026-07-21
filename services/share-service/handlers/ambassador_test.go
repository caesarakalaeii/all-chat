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
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockAmbassadorStore records calls and returns canned values.
type mockAmbassadorStore struct {
	setCalled  bool
	setUserID  string
	setValue   bool
	setTagline *string
	setSort    *int
	setErr     error

	showcase    *repository.Showcase
	showcaseErr error

	consentCalled bool
	consentUserID string
	consentValue  bool
	consentErr    error

	listResult []repository.PublicAmbassador
	listErr    error
}

func (m *mockAmbassadorStore) SetUserAmbassador(_ context.Context, userID string, isAmbassador bool, tagline *string, sortOrder *int) error {
	m.setCalled = true
	m.setUserID = userID
	m.setValue = isAmbassador
	m.setTagline = tagline
	m.setSort = sortOrder
	return m.setErr
}

func (m *mockAmbassadorStore) GetShowcase(_ context.Context, _ string) (*repository.Showcase, error) {
	return m.showcase, m.showcaseErr
}

func (m *mockAmbassadorStore) SetFeaturedConsent(_ context.Context, userID string, consent bool) error {
	m.consentCalled = true
	m.consentUserID = userID
	m.consentValue = consent
	return m.consentErr
}

func (m *mockAmbassadorStore) ListPublic(_ context.Context) ([]repository.PublicAmbassador, error) {
	return m.listResult, m.listErr
}

func ambassadorAdminRouter(repo ambassadorStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAmbassadorHandler(repo, zap.NewNop())
	r.POST("/admin/ambassadors/users/:id", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Next()
	}, h.SetUserAmbassador)
	return r
}

// selfRouter wires the self-service showcase routes, injecting a caller user_id
// (empty string => simulate an unauthenticated caller).
func selfRouter(repo ambassadorStore, callerID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAmbassadorHandler(repo, zap.NewNop())
	inject := func(c *gin.Context) {
		if callerID != "" {
			c.Set("user_id", callerID)
		}
		c.Next()
	}
	r.GET("/ambassadors/me/showcase", inject, h.GetMyShowcase)
	r.PUT("/ambassadors/me/showcase", inject, h.UpdateMyConsent)
	return r
}

func doReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func strptr(s string) *string { return &s }

// --- Admin assign/revoke -----------------------------------------------------

func TestSetUserAmbassador_GrantWithCard(t *testing.T) {
	repo := &mockAmbassadorStore{}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42",
		`{"is_ambassador":true,"tagline":"Multistreams to 3 platforms","sort_order":10}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u-42", repo.setUserID)
	assert.True(t, repo.setValue)
	if assert.NotNil(t, repo.setTagline) {
		assert.Equal(t, "Multistreams to 3 platforms", *repo.setTagline)
	}
	if assert.NotNil(t, repo.setSort) {
		assert.Equal(t, 10, *repo.setSort)
	}
}

// A grant that omits tagline/sort_order must pass nil (preserve existing), not zero.
func TestSetUserAmbassador_GrantOmittedCardFieldsAreNil(t *testing.T) {
	repo := &mockAmbassadorStore{}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42",
		`{"is_ambassador":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, repo.setValue)
	assert.Nil(t, repo.setTagline, "omitted tagline must be nil so the repo preserves it")
	assert.Nil(t, repo.setSort, "omitted sort_order must be nil so the repo preserves it")
}

func TestSetUserAmbassador_Revoke(t *testing.T) {
	repo := &mockAmbassadorStore{}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42",
		`{"is_ambassador":false}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, repo.setValue)
}

func TestSetUserAmbassador_BadBody(t *testing.T) {
	repo := &mockAmbassadorStore{}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42", `not-json`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, repo.setCalled, "repo must not be called on a malformed body")
}

func TestSetUserAmbassador_NotFound(t *testing.T) {
	repo := &mockAmbassadorStore{setErr: errors.New("user not found: u-42")}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42",
		`{"is_ambassador":true}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetUserAmbassador_InternalError(t *testing.T) {
	repo := &mockAmbassadorStore{setErr: errors.New("db down")}
	w := doReq(ambassadorAdminRouter(repo), http.MethodPost, "/admin/ambassadors/users/u-42",
		`{"is_ambassador":true}`)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Self-service read -------------------------------------------------------

func TestGetMyShowcase_Ambassador(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{
		IsAmbassador: true, Tagline: strptr("hi"), SortOrder: 5, FeaturedConsent: true,
	}}
	w := doReq(selfRouter(repo, "u-1"), http.MethodGet, "/ambassadors/me/showcase", "")

	assert.Equal(t, http.StatusOK, w.Code)
	var got showcaseResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.True(t, got.IsAmbassador)
	assert.True(t, got.FeaturedConsent)
	assert.Equal(t, 5, got.SortOrder)
}

func TestGetMyShowcase_NotAmbassador(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{IsAmbassador: false}}
	w := doReq(selfRouter(repo, "u-1"), http.MethodGet, "/ambassadors/me/showcase", "")

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetMyShowcase_Unauthenticated(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{IsAmbassador: true}}
	w := doReq(selfRouter(repo, ""), http.MethodGet, "/ambassadors/me/showcase", "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Self-service consent ----------------------------------------------------

func TestUpdateMyConsent_Ambassador(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{IsAmbassador: true}}
	w := doReq(selfRouter(repo, "u-1"), http.MethodPut, "/ambassadors/me/showcase", `{"featured_consent":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, repo.consentCalled)
	assert.Equal(t, "u-1", repo.consentUserID)
	assert.True(t, repo.consentValue)
}

func TestUpdateMyConsent_NotAmbassadorForbidden(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{IsAmbassador: false}}
	w := doReq(selfRouter(repo, "u-1"), http.MethodPut, "/ambassadors/me/showcase", `{"featured_consent":true}`)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, repo.consentCalled, "a non-ambassador must never reach SetFeaturedConsent")
}

func TestUpdateMyConsent_BadBody(t *testing.T) {
	repo := &mockAmbassadorStore{showcase: &repository.Showcase{IsAmbassador: true}}
	w := doReq(selfRouter(repo, "u-1"), http.MethodPut, "/ambassadors/me/showcase", `not-json`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, repo.consentCalled)
}

// --- Public list -------------------------------------------------------------

func TestListPublic(t *testing.T) {
	repo := &mockAmbassadorStore{listResult: []repository.PublicAmbassador{
		{Username: "alice", DisplayName: "Alice", AvatarURL: "http://a", Platform: "twitch", Tagline: strptr("t")},
		{Username: "bob", DisplayName: "Bob", Platform: "kick"},
	}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAmbassadorHandler(repo, zap.NewNop())
	r.GET("/ambassadors", h.ListPublic)

	w := doReq(r, http.MethodGet, "/ambassadors", "")
	assert.Equal(t, http.StatusOK, w.Code)

	var got []publicAmbassadorResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
	assert.Equal(t, "Alice", got[0].DisplayName)
	assert.Equal(t, "twitch", got[0].Platform)
}

func TestListPublic_Error(t *testing.T) {
	repo := &mockAmbassadorStore{listErr: errors.New("db down")}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAmbassadorHandler(repo, zap.NewNop())
	r.GET("/ambassadors", h.ListPublic)

	w := doReq(r, http.MethodGet, "/ambassadors", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
