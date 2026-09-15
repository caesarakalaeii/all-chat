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
	"regexp"
	"strings"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Localization handlers (ADR-0063): the beta-tester translation tool.
//
// Contributor routes live under /api/v1/localization and are mounted behind
// RequireEarlyAccess('localization_contribution') — beta testers and
// ambassadors (ADR-0020/0041), open to everyone once the gate graduates.
// Admin routes live under /api/v1/admin/localization behind AdminOnly.

// localizationStore is the narrow repository surface the handlers need.
type localizationStore interface {
	RequestLocale(ctx context.Context, code, englishName, nativeName, requestedBy string) error
	ListApprovedLocales(ctx context.Context) ([]repository.Locale, error)
	UpsertTranslation(ctx context.Context, locale, key, value, submittedBy string) error
	ListTranslations(ctx context.Context, locale string) ([]repository.Translation, error)
}

type LocalizationHandler struct {
	repo   localizationStore
	logger *zap.Logger
}

func NewLocalizationHandler(repo localizationStore, logger *zap.Logger) *LocalizationHandler {
	return &LocalizationHandler{repo: repo, logger: logger}
}

// placeholderPattern extracts every {name} occurrence from a string, matching
// the catalog's single-brace syntax (docs/frontend/I18N.md). Used to check a
// translation carries the same placeholders as its English source: a
// translation that drops {count} renders a visible literal to viewers, and the
// lookup deliberately never throws (overlay hardening), so nothing else would
// catch it.
var placeholderPattern = regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9_]*\}`)

// placeholders returns the set of placeholder names in s, in first-seen order.
func placeholders(s string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range placeholderPattern.FindAllString(s, -1) {
		name := m[1 : len(m)-1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// samePlaceholders reports whether a and b contain exactly the same placeholder
// names, order-insensitive (word order is the first thing a translation
// changes).
func samePlaceholders(a, b string) bool {
	pa, pb := placeholders(a), placeholders(b)
	if len(pa) != len(pb) {
		return false
	}
	set := make(map[string]bool, len(pa))
	for _, p := range pa {
		set[p] = true
	}
	for _, p := range pb {
		if !set[p] {
			return false
		}
	}
	return true
}

// RequestLocale handles POST /api/v1/localization/locales
// Body: {"code": "de", "english_name": "German", "native_name": "Deutsch"}
//
// Any contributor can propose a locale. It stays invisible to other
// contributors until an admin approves it; a duplicate request is a success.
func (h *LocalizationHandler) RequestLocale(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		Code        string `json:"code"`
		EnglishName string `json:"english_name"`
		NativeName  string `json:"native_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code, english_name and native_name required"})
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if !repository.ValidLocaleCode(req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid locale code"})
		return
	}
	if strings.TrimSpace(req.EnglishName) == "" || strings.TrimSpace(req.NativeName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "english_name and native_name required"})
		return
	}

	if err := h.repo.RequestLocale(c.Request.Context(), req.Code, req.EnglishName, req.NativeName, userID); err != nil {
		h.logger.Error("Failed to request locale",
			zap.String("user_id", userID), zap.String("code", req.Code), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to request locale"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "locale requested", "code": req.Code})
}

// ListLocales handles GET /api/v1/localization/locales
//
// Approved locales only — requested ones are invisible to other contributors.
func (h *LocalizationHandler) ListLocales(c *gin.Context) {
	locales, err := h.repo.ListApprovedLocales(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list locales", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list locales"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"locales": locales})
}

// SubmitTranslations handles POST /api/v1/localization/locales/:code/translations
// Body: {"translations": [{"key": "common.save", "value": "Speichern"}, …]}
//
// Batch upsert: each row overwrites the caller's previous submission for that
// key and resets it to 'pending'. Per-row validation failures are reported in
// the response and abort nothing else — a contributor pasting a large batch
// must not lose 40 good rows to one bad one.
func (h *LocalizationHandler) SubmitTranslations(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	locale := c.Param("code")

	var req struct {
		Translations []struct {
			Key         string `json:"key"`
			Value       string `json:"value"`
			SourceValue string `json:"source_value"`
		} `json:"translations"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "translations array required"})
		return
	}
	if len(req.Translations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "translations array must not be empty"})
		return
	}
	if len(req.Translations) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at most 200 translations per batch"})
		return
	}

	type rowError struct {
		Key   string `json:"key"`
		Error string `json:"error"`
	}
	var failed []rowError
	accepted := 0

	for _, tr := range req.Translations {
		if !repository.ValidKey(tr.Key) {
			failed = append(failed, rowError{Key: tr.Key, Error: "invalid key"})
			continue
		}
		if strings.TrimSpace(tr.Value) == "" {
			failed = append(failed, rowError{Key: tr.Key, Error: "value required"})
			continue
		}
		// Placeholder parity against the English source the client sends.
		// The client derives it from the catalog; a mismatch is rejected here
		// so a dropped {placeholder} can never reach review, let alone the
		// repo. The client's source is trusted for parity checking only —
		// the catalog, not the DB, is the key/value source of truth.
		if tr.SourceValue != "" && !samePlaceholders(tr.SourceValue, tr.Value) {
			failed = append(failed, rowError{Key: tr.Key, Error: "placeholders do not match the English source"})
			continue
		}
		if err := h.repo.UpsertTranslation(c.Request.Context(), locale, tr.Key, tr.Value, userID); err != nil {
			if errors.Is(err, repository.ErrLocaleNotApproved) {
				// Unknown or unapproved locale: not the row's fault, and no
				// point continuing the batch.
				c.JSON(http.StatusNotFound, gin.H{"error": "locale not approved"})
				return
			}
			failed = append(failed, rowError{Key: tr.Key, Error: "failed to save"})
			continue
		}
		accepted++
	}

	h.logger.Info("Translation batch submitted",
		zap.String("user_id", userID),
		zap.String("locale", locale),
		zap.Int("accepted", accepted),
		zap.Int("failed", len(failed)))

	c.JSON(http.StatusOK, gin.H{
		"accepted": accepted,
		"failed":   failed,
	})
}

// MyTranslations handles GET /api/v1/localization/locales/:code/translations
//
// Returns the caller's own submissions for a locale (any status), so the
// contributor view can show pending/approved/rejected state and rejection
// notes.
func (h *LocalizationHandler) MyTranslations(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	locale := c.Param("code")

	translations, err := h.repo.ListTranslations(c.Request.Context(), locale)
	if err != nil {
		h.logger.Error("Failed to list translations",
			zap.String("user_id", userID), zap.String("locale", locale), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list translations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"translations": translations})
}
