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

// ambassadorStore is the narrow repository surface the ambassador handler needs.
// Satisfied by *repository.AmbassadorRepository in production and a mock in tests.
type ambassadorStore interface {
	SetUserAmbassador(ctx context.Context, userID string, isAmbassador bool, tagline *string, sortOrder *int) error
	GetShowcase(ctx context.Context, userID string) (*repository.Showcase, error)
	SetFeaturedConsent(ctx context.Context, userID string, consent bool) error
	ListPublic(ctx context.Context) ([]repository.PublicAmbassador, error)
}

type AmbassadorHandler struct {
	repo   ambassadorStore
	logger *zap.Logger
}

func NewAmbassadorHandler(repo ambassadorStore, logger *zap.Logger) *AmbassadorHandler {
	return &AmbassadorHandler{repo: repo, logger: logger}
}

// SetUserAmbassador handles POST /api/v1/admin/ambassadors/users/:id (admin only).
// Body: {"is_ambassador": true, "tagline": "Multistreams to 3 platforms", "sort_order": 10}
// tagline and sort_order are optional and only meaningful when granting; a nil field
// preserves the current value (ADR-0041). Granting force-grants premium + early access.
func (h *AmbassadorHandler) SetUserAmbassador(c *gin.Context) {
	adminUserID := c.GetString("user_id")
	targetUserID := c.Param("id")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user ID required"})
		return
	}

	var req struct {
		IsAmbassador bool    `json:"is_ambassador"`
		Tagline      *string `json:"tagline"`
		SortOrder    *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_ambassador field required"})
		return
	}

	err := h.repo.SetUserAmbassador(c.Request.Context(), targetUserID, req.IsAmbassador, req.Tagline, req.SortOrder)
	if err != nil {
		h.logger.Error("Failed to update ambassador status",
			zap.String("admin_id", adminUserID),
			zap.String("target_id", targetUserID),
			zap.Error(err))
		// Repo returns "user not found: <id>" (fmt.Errorf, no sentinel) — match on prefix.
		if strings.HasPrefix(err.Error(), "user not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update ambassador status"})
		return
	}

	h.logger.Info("Ambassador status updated by admin",
		zap.String("admin_id", adminUserID),
		zap.String("target_id", targetUserID),
		zap.Bool("is_ambassador", req.IsAmbassador))
	c.JSON(http.StatusOK, gin.H{
		"message":       "ambassador status updated",
		"user_id":       targetUserID,
		"is_ambassador": req.IsAmbassador,
	})
}

// showcaseResponse is the shape returned to a streamer for their own showcase.
type showcaseResponse struct {
	IsAmbassador    bool    `json:"is_ambassador"`
	Tagline         *string `json:"tagline"`
	SortOrder       int     `json:"sort_order"`
	FeaturedConsent bool    `json:"featured_consent"`
}

// GetMyShowcase handles GET /api/v1/ambassadors/me/showcase (authenticated).
// It returns the caller's own ambassador showcase state so the Settings toggle can
// render the current consent. Non-ambassadors get 403 — the role is admin-granted.
func (h *AmbassadorHandler) GetMyShowcase(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	sc, err := h.repo.GetShowcase(c.Request.Context(), userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "user not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		h.logger.Error("Failed to read ambassador showcase", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read ambassador showcase"})
		return
	}
	if !sc.IsAmbassador {
		c.JSON(http.StatusForbidden, gin.H{"error": "not an ambassador"})
		return
	}

	c.JSON(http.StatusOK, showcaseResponse{
		IsAmbassador:    true,
		Tagline:         sc.Tagline,
		SortOrder:       sc.SortOrder,
		FeaturedConsent: sc.FeaturedConsent,
	})
}

// UpdateMyConsent handles PUT /api/v1/ambassadors/me/showcase (authenticated).
// Body: {"featured_consent": true}. It lets an ambassador opt in/out of the public
// homepage showcase. Only the streamer's own consent is writable here — the tagline
// and sort_order are admin-curated. Non-ambassadors get 403.
func (h *AmbassadorHandler) UpdateMyConsent(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req struct {
		FeaturedConsent bool `json:"featured_consent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "featured_consent field required"})
		return
	}

	// Only an ambassador may set consent — check the role first (fresh DB read).
	sc, err := h.repo.GetShowcase(c.Request.Context(), userID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "user not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		h.logger.Error("Failed to read ambassador showcase", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read ambassador showcase"})
		return
	}
	if !sc.IsAmbassador {
		c.JSON(http.StatusForbidden, gin.H{"error": "not an ambassador"})
		return
	}

	if err := h.repo.SetFeaturedConsent(c.Request.Context(), userID, req.FeaturedConsent); err != nil {
		h.logger.Error("Failed to update ambassador consent", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update consent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"featured_consent": req.FeaturedConsent})
}

// publicAmbassadorResponse is one card in the public homepage list.
type publicAmbassadorResponse struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   string  `json:"avatar_url"`
	Platform    string  `json:"platform"`
	Tagline     *string `json:"tagline"`
}

// ListPublic handles GET /api/v1/ambassadors (public, no auth). It returns the
// opted-in ambassadors for the "Featured Ambassadors" homepage section.
func (h *AmbassadorHandler) ListPublic(c *gin.Context) {
	list, err := h.repo.ListPublic(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list ambassadors", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ambassadors"})
		return
	}

	resp := make([]publicAmbassadorResponse, len(list))
	for i, a := range list {
		resp[i] = publicAmbassadorResponse{
			Username:    a.Username,
			DisplayName: a.DisplayName,
			AvatarURL:   a.AvatarURL,
			Platform:    a.Platform,
			Tagline:     a.Tagline,
		}
	}
	c.JSON(http.StatusOK, resp)
}
