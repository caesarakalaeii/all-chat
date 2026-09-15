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

// mockLocalizationStore records calls and replays canned errors.
type mockLocalizationStore struct {
	requestedCode string
	requestedBy   string
	upserts       []string // keys handed to UpsertTranslation
	localeErr     error    // returned by UpsertTranslation
	listedLocale  string
	locales       []repository.Locale
	translations  []repository.Translation
}

func (m *mockLocalizationStore) RequestLocale(_ context.Context, code, _, _, requestedBy string) error {
	m.requestedCode = code
	m.requestedBy = requestedBy
	return nil
}

func (m *mockLocalizationStore) ListApprovedLocales(_ context.Context) ([]repository.Locale, error) {
	return m.locales, nil
}

func (m *mockLocalizationStore) UpsertTranslation(_ context.Context, locale, key, _, submittedBy string) error {
	m.upserts = append(m.upserts, locale+"/"+key+"@"+submittedBy)
	return m.localeErr
}

func (m *mockLocalizationStore) ListTranslations(_ context.Context, locale string) ([]repository.Translation, error) {
	m.listedLocale = locale
	return m.translations, nil
}

// localizationRouter wires the contributor routes with a user_id injected the
// way JWTAuth would, matching the production param shape.
func localizationRouter(repo localizationStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewLocalizationHandler(repo, zap.NewNop())
	g := r.Group("/localization", func(c *gin.Context) {
		c.Set("user_id", "u-1")
		c.Next()
	})
	{
		g.GET("/locales", h.ListLocales)
		g.POST("/locales", h.RequestLocale)
		g.GET("/locales/:code/translations", h.MyTranslations)
		g.POST("/locales/:code/translations", h.SubmitTranslations)
	}
	return r
}

func postJSONLocal(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestRequestLocale_Valid(t *testing.T) {
	repo := &mockLocalizationStore{}
	w := postJSONLocal(localizationRouter(repo), "/localization/locales",
		`{"code":"de","english_name":"German","native_name":"Deutsch"}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "de", repo.requestedCode)
	assert.Equal(t, "u-1", repo.requestedBy)
}

func TestRequestLocale_BadCode(t *testing.T) {
	repo := &mockLocalizationStore{}
	// '../etc' is filesystem-hostile and must fail shape validation.
	w := postJSONLocal(localizationRouter(repo), "/localization/locales",
		`{"code":"../etc","english_name":"X","native_name":"X"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, repo.requestedCode)
}

func TestSubmitTranslations_BatchWithPlaceholderMismatch(t *testing.T) {
	repo := &mockLocalizationStore{}
	// One good row, one that drops the {count} placeholder: the bad row is
	// reported, the good row is saved — a large paste must not be all-or-nothing.
	w := postJSONLocal(localizationRouter(repo), "/localization/locales/de/translations",
		`{"translations":[
			{"key":"common.save","value":"Speichern","source_value":"Save"},
			{"key":"errors.count","value":"Fehler","source_value":"{count} errors"}
		]}`)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Accepted int `json:"accepted"`
		Failed   []struct {
			Key   string `json:"key"`
			Error string `json:"error"`
		} `json:"failed"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Accepted)
	assert.Len(t, resp.Failed, 1)
	assert.Equal(t, "errors.count", resp.Failed[0].Key)
	assert.Len(t, repo.upserts, 1, "only the valid row reaches the repo")
}

func TestSubmitTranslations_LocaleNotApproved(t *testing.T) {
	repo := &mockLocalizationStore{localeErr: repository.ErrLocaleNotApproved}
	w := postJSONLocal(localizationRouter(repo), "/localization/locales/fr/translations",
		`{"translations":[{"key":"common.save","value":"Enregistrer"}]}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSubmitTranslations_EmptyBatch(t *testing.T) {
	repo := &mockLocalizationStore{}
	w := postJSONLocal(localizationRouter(repo), "/localization/locales/de/translations",
		`{"translations":[]}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, repo.upserts)
}

func TestPlaceholderParity(t *testing.T) {
	// Same set, different order — word order is the point of translation.
	assert.True(t, samePlaceholders("{count} errors", "Fehler: {count}"))
	// Missing placeholder.
	assert.False(t, samePlaceholders("{count} errors", "Fehler"))
	// Extra placeholder.
	assert.False(t, samePlaceholders("Save", "Speichern {count}"))
	// No placeholders on either side.
	assert.True(t, samePlaceholders("Save", "Speichern"))
	// Repeated occurrence still matches a single one in the other string.
	assert.True(t, samePlaceholders("{word} and {word}", "et {word}"))
}

func TestValidKey(t *testing.T) {
	assert.True(t, repository.ValidKey("common.save"))
	assert.True(t, repository.ValidKey("overlayEditor.appearance.title"))
	assert.False(t, repository.ValidKey("save"))          // no namespace
	assert.False(t, repository.ValidKey("a.b.c.d.e"))     // > 3 levels
	assert.False(t, repository.ValidKey("common..save"))  // empty segment
	assert.False(t, repository.ValidKey("common.0save"))  // segment starts with digit
	assert.False(t, repository.ValidKey("../etc/passwd")) // filesystem hostile
}

func TestValidLocaleCode(t *testing.T) {
	assert.True(t, repository.ValidLocaleCode("de"))
	assert.True(t, repository.ValidLocaleCode("pt-BR"))
	assert.False(t, repository.ValidLocaleCode("de-DE-u-co-phonebk-x-foo")) // too long
	assert.False(t, repository.ValidLocaleCode("../etc"))
	assert.False(t, repository.ValidLocaleCode(""))
}

func TestMyTranslations(t *testing.T) {
	repo := &mockLocalizationStore{translations: []repository.Translation{
		{Key: "common.save", Value: "Speichern", Status: "pending"},
	}}
	r := localizationRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/localization/locales/de/translations", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "de", repo.listedLocale)

	var resp struct {
		Translations []repository.Translation `json:"translations"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Translations, 1)
}

// --- Admin handlers ---

type mockLocalizationAdminStore struct {
	requested    []repository.Locale
	approvedCode string
	rejectedCode string
	reviewed     map[string]string // "locale/key" -> decision
	reviewErr    error
	exportErr    error
	exported     []repository.Translation
	progress     []repository.LocaleProgress
}

func (m *mockLocalizationAdminStore) ListRequestedLocales(_ context.Context) ([]repository.Locale, error) {
	return m.requested, nil
}

func (m *mockLocalizationAdminStore) ApproveLocaleRequest(_ context.Context, code, _ string) error {
	m.approvedCode = code
	return nil
}

func (m *mockLocalizationAdminStore) RejectLocaleRequest(_ context.Context, code string) error {
	m.rejectedCode = code
	return nil
}

func (m *mockLocalizationAdminStore) ListPendingTranslations(_ context.Context) ([]repository.Translation, error) {
	return m.exported, nil
}

func (m *mockLocalizationAdminStore) ReviewTranslation(_ context.Context, locale, key, decision, _, _ string) error {
	if m.reviewErr != nil {
		return m.reviewErr
	}
	if m.reviewed == nil {
		m.reviewed = map[string]string{}
	}
	m.reviewed[locale+"/"+key] = decision
	return nil
}

func (m *mockLocalizationAdminStore) ExportLocale(_ context.Context, locale string) ([]repository.Translation, error) {
	if m.exportErr != nil {
		return nil, m.exportErr
	}
	return m.exported, nil
}

func (m *mockLocalizationAdminStore) ListLocaleProgress(_ context.Context) ([]repository.LocaleProgress, error) {
	return m.progress, nil
}

func localizationAdminRouter(repo localizationAdminStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewLocalizationAdminHandler(repo, zap.NewNop())
	g := r.Group("/admin/localization", func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Next()
	})
	{
		g.GET("/locales/requests", h.GetLocaleRequests)
		g.POST("/locales/:code/review", h.ReviewLocaleRequest)
		g.GET("/review", h.GetReviewQueue)
		g.POST("/review/:locale/:keyHash", h.ReviewSubmission)
		g.GET("/export/:code", h.ExportLocale)
		g.GET("/progress", h.GetProgress)
	}
	return r
}

func TestReviewLocaleRequest_Approve(t *testing.T) {
	repo := &mockLocalizationAdminStore{}
	w := postJSONLocal(localizationAdminRouter(repo), "/admin/localization/locales/de/review",
		`{"approved":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "de", repo.approvedCode)
}

func TestReviewLocaleRequest_Reject(t *testing.T) {
	repo := &mockLocalizationAdminStore{}
	w := postJSONLocal(localizationAdminRouter(repo), "/admin/localization/locales/xx/review",
		`{"approved":false}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "xx", repo.rejectedCode)
}

func TestReviewSubmission_KeyInPath(t *testing.T) {
	repo := &mockLocalizationAdminStore{}
	// The dotted key travels URL-encoded in the path.
	w := postJSONLocal(localizationAdminRouter(repo), "/admin/localization/review/de/common%2Esave",
		`{"approved":true}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "approved", repo.reviewed["de/common.save"])
}

func TestReviewSubmission_NotPending(t *testing.T) {
	repo := &mockLocalizationAdminStore{reviewErr: errors.New("pending translation not found: de/common.save")}
	w := postJSONLocal(localizationAdminRouter(repo), "/admin/localization/review/de/common%2Esave",
		`{"approved":true}`)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExportLocale_NotApproved(t *testing.T) {
	repo := &mockLocalizationAdminStore{exportErr: repository.ErrLocaleNotApproved}
	r := localizationAdminRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/localization/export/xx", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExportLocale_Ok(t *testing.T) {
	repo := &mockLocalizationAdminStore{exported: []repository.Translation{
		{Key: "common.save", Value: "Speichern", Status: "approved"},
	}}
	r := localizationAdminRouter(repo)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/localization/export/de", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Locale       string                   `json:"locale"`
		Translations []repository.Translation `json:"translations"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "de", resp.Locale)
	assert.Len(t, resp.Translations, 1)
	assert.Equal(t, "Speichern", resp.Translations[0].Value)
}
