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
	"net/http"
	"strings"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Localization admin handlers (ADR-0063): the review half of the translation
// tool, mounted under /api/v1/admin/localization behind AdminOnly.
//
// The accepted-translation export ends the pipeline: it returns approved rows
// as JSON, scripts/generate-locale-catalog.mjs turns that into
// frontend/src/lib/i18n/messages/<locale>/ files, and the result is a normal
// reviewed PR. Nothing auto-commits.

// localizationAdminStore is the narrow repository surface the admin handlers need.
type localizationAdminStore interface {
	ListRequestedLocales(ctx context.Context) ([]repository.Locale, error)
	ApproveLocaleRequest(ctx context.Context, code, reviewedBy string) error
	RejectLocaleRequest(ctx context.Context, code string) error
	ListPendingTranslations(ctx context.Context) ([]repository.Translation, error)
	ReviewTranslation(ctx context.Context, locale, key, decision, reviewNote, reviewedBy string) error
	ExportLocale(ctx context.Context, locale string) ([]repository.Translation, error)
	ListLocaleProgress(ctx context.Context) ([]repository.LocaleProgress, error)
}

type LocalizationAdminHandler struct {
	repo   localizationAdminStore
	logger *zap.Logger
}

func NewLocalizationAdminHandler(repo localizationAdminStore, logger *zap.Logger) *LocalizationAdminHandler {
	return &LocalizationAdminHandler{repo: repo, logger: logger}
}

// GetLocaleRequests handles GET /api/v1/admin/localization/locales/requests
func (h *LocalizationAdminHandler) GetLocaleRequests(c *gin.Context) {
	locales, err := h.repo.ListRequestedLocales(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list locale requests", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list locale requests"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"locales": locales})
}

// ReviewLocaleRequest handles POST /api/v1/admin/localization/locales/:code/review
// Body: {"approved": true} — approving makes the locale visible to all
// contributors; rejecting deletes the request.
func (h *LocalizationAdminHandler) ReviewLocaleRequest(c *gin.Context) {
	adminID := c.GetString("user_id")
	code := c.Param("code")

	var req struct {
		Approved bool `json:"approved"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "approved field required"})
		return
	}

	if req.Approved {
		if !repository.ValidLocaleCode(code) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid locale code"})
			return
		}
		if err := h.repo.ApproveLocaleRequest(c.Request.Context(), code, adminID); err != nil {
			h.logger.Error("Failed to approve locale",
				zap.String("admin_id", adminID), zap.String("code", code), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve locale"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "locale approved", "code": code})
		return
	}

	if err := h.repo.RejectLocaleRequest(c.Request.Context(), code); err != nil {
		if strings.HasPrefix(err.Error(), "locale request not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "locale request not found"})
			return
		}
		h.logger.Error("Failed to reject locale",
			zap.String("admin_id", adminID), zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject locale"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "locale request rejected", "code": code})
}

// GetReviewQueue handles GET /api/v1/admin/localization/review
//
// Pending submissions across all approved locales, oldest first.
func (h *LocalizationAdminHandler) GetReviewQueue(c *gin.Context) {
	pending, err := h.repo.ListPendingTranslations(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list pending translations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pending translations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"translations": pending})
}

// ReviewSubmission handles POST /api/v1/admin/localization/review/:locale/:keyHash
// — keyHash is the URL-encoded dotted key; Gin decodes %2E transparently.
// Body: {"approved": true, "review_note": "…"} — review_note is optional and
// only meaningful on rejection (shown to the contributor).
//
// The route carries the key in the path rather than the body so the admin
// action is visible in access logs as a target, matching how every other admin
// mutation in this service names its object.
func (h *LocalizationAdminHandler) ReviewSubmission(c *gin.Context) {
	adminID := c.GetString("user_id")
	locale := c.Param("locale")
	key := c.Param("keyHash")

	var req struct {
		Approved   bool   `json:"approved"`
		ReviewNote string `json:"review_note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "approved field required"})
		return
	}

	decision := "rejected"
	if req.Approved {
		decision = "approved"
	}
	err := h.repo.ReviewTranslation(c.Request.Context(), locale, key, decision, req.ReviewNote, adminID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "pending translation not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "pending translation not found"})
			return
		}
		h.logger.Error("Failed to review translation",
			zap.String("admin_id", adminID),
			zap.String("locale", locale),
			zap.String("key", key),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to review translation"})
		return
	}

	h.logger.Info("Translation reviewed",
		zap.String("admin_id", adminID),
		zap.String("locale", locale),
		zap.String("key", key),
		zap.Bool("approved", req.Approved))

	c.JSON(http.StatusOK, gin.H{
		"message": "translation reviewed",
		"locale":  locale,
		"key":     key,
		"status":  decision,
	})
}

// ExportLocale handles GET /api/v1/admin/localization/export/:code
//
// Returns every approved translation for a locale as
// {"locale": "de", "translations": [{"key": …, "value": …}, …]}. Running
// scripts/generate-locale-catalog.mjs against this JSON produces the catalog
// files for a PR — see the script header for the exact invocation.
func (h *LocalizationAdminHandler) ExportLocale(c *gin.Context) {
	code := c.Param("code")

	translations, err := h.repo.ExportLocale(c.Request.Context(), code)
	if err != nil {
		if err == repository.ErrLocaleNotApproved {
			c.JSON(http.StatusNotFound, gin.H{"error": "locale not approved"})
			return
		}
		h.logger.Error("Failed to export locale", zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export locale"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"locale":       code,
		"translations": translations,
	})
}

// GetProgress handles GET /api/v1/admin/localization/progress
//
// Per-locale pending/approved/rejected counts for the admin review surface.
func (h *LocalizationAdminHandler) GetProgress(c *gin.Context) {
	progress, err := h.repo.ListLocaleProgress(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list locale progress", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list locale progress"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"locales": progress})
}
