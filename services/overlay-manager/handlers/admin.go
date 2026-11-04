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
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// adminOverlayStore and adminSourceStore are the narrow slices of the
// repositories the admin read endpoints use. Interfaces rather than the
// concrete types keep the handler testable without a database, the same way
// every other handler in this package takes its repositories.
type adminOverlayStore interface {
	ListOverlaysWithSourceCount(ctx context.Context, filter repository.OverlayListFilter, limit, offset int) ([]*repository.OverlayWithSourceCount, int, error)
	ListByUserIDWithSourceCount(ctx context.Context, userID string) ([]*repository.OverlayWithSourceCount, error)
}

type adminSourceStore interface {
	ListSourcesWithOverlay(ctx context.Context, filter repository.SourceListFilter, limit, offset int) ([]*repository.SourceWithOverlay, int, error)
	ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error)
}

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	overlayRepo adminOverlayStore
	sourceRepo  adminSourceStore
	logger      *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(overlayRepo adminOverlayStore, sourceRepo adminSourceStore, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		overlayRepo: overlayRepo,
		sourceRepo:  sourceRepo,
		logger:      logger,
	}
}

// pageParams reads the limit/offset pagination query parameters shared by the
// admin list endpoints: 50 rows by default, at most 100, and junk or negative
// values fall back to the defaults.
func pageParams(c *gin.Context) (limit, offset int) {
	limit = 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = min(l, 100)
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o > 0 {
		offset = o
	}
	return limit, offset
}

// ListOverlays returns one page of overlays, newest first, with the total
// number of matches (admin only). ids (comma-separated) restricts the listing
// to those overlays, for the connected-only view and deep links.
// GET /api/v1/admin/overlays?q=<search>&ids=<id,id>&limit=&offset=
func (h *AdminHandler) ListOverlays(c *gin.Context) {
	limit, offset := pageParams(c)
	filter := repository.OverlayListFilter{Query: strings.TrimSpace(c.Query("q"))}
	for _, id := range strings.Split(c.Query("ids"), ",") {
		if id != "" {
			filter.IDs = append(filter.IDs, id)
		}
	}
	overlays, total, err := h.overlayRepo.ListOverlaysWithSourceCount(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.logger.Error("Failed to fetch overlays", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch overlays",
		})
		return
	}

	type OverlayResponse struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		OverlayType      string `json:"overlay_type"`
		UserID           string `json:"user_id"`
		OwnerUsername    string `json:"owner_username"`
		OwnerDisplayName string `json:"owner_display_name"`
		CreatedAt        string `json:"created_at"`
		UpdatedAt        string `json:"updated_at"`
		SourcesCount     int    `json:"sources_count"`
	}

	response := make([]OverlayResponse, len(overlays))

	// Kind() resolves an absent type to chat, matching every other response
	// path; the column is NOT NULL so this only matters for pre-column rows.
	for i, overlay := range overlays {
		response[i] = OverlayResponse{
			ID:               overlay.ID,
			Name:             overlay.Name,
			OverlayType:      overlay.Kind(),
			UserID:           overlay.UserID,
			OwnerUsername:    overlay.OwnerUsername,
			OwnerDisplayName: overlay.OwnerDisplayName,
			CreatedAt:        overlay.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        overlay.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			SourcesCount:     overlay.SourcesCount,
		}
	}

	c.JSON(http.StatusOK, gin.H{"overlays": response, "total": total, "limit": limit, "offset": offset})
}

// ListAllSources returns one page of sources across all overlays, newest
// first, with the total number of matches (admin only). status is "active" or
// "inactive" (anything else means either); user scopes to one overlay owner.
// GET /api/v1/admin/sources?q=&platform=&status=&user=&limit=&offset=
func (h *AdminHandler) ListAllSources(c *gin.Context) {
	limit, offset := pageParams(c)
	filter := repository.SourceListFilter{
		Query:    strings.TrimSpace(c.Query("q")),
		Platform: c.Query("platform"),
		UserID:   c.Query("user"),
	}
	switch c.Query("status") {
	case "active":
		filter.Active = new(true)
	case "inactive":
		filter.Active = new(false)
	}
	sources, total, err := h.sourceRepo.ListSourcesWithOverlay(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.logger.Error("Failed to fetch sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch sources",
		})
		return
	}

	// Include overlay information for each source
	type SourceResponse struct {
		ID               string  `json:"id"`
		OverlayID        string  `json:"overlay_id"`
		OverlayName      string  `json:"overlay_name"`
		Platform         string  `json:"platform"`
		ChannelID        string  `json:"channel_id"`
		ChannelName      string  `json:"channel_name"`
		ChannelHandle    *string `json:"channel_handle,omitempty"`
		IsActive         bool    `json:"is_active"`
		CreatedAt        string  `json:"created_at"`
		UserID           string  `json:"user_id"`
		OwnerUsername    string  `json:"owner_username"`
		OwnerDisplayName string  `json:"owner_display_name"`
	}

	response := make([]SourceResponse, 0, len(sources))
	for _, source := range sources {
		response = append(response, SourceResponse{
			ID:               source.ID,
			OverlayID:        source.OverlayID,
			OverlayName:      source.OverlayName,
			Platform:         source.Platform,
			ChannelID:        source.ChannelID,
			ChannelName:      source.ChannelName,
			ChannelHandle:    source.ChannelHandle,
			IsActive:         source.IsActive,
			CreatedAt:        source.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UserID:           source.UserID,
			OwnerUsername:    source.OwnerUsername,
			OwnerDisplayName: source.OwnerDisplayName,
		})
	}

	c.JSON(http.StatusOK, gin.H{"sources": response, "total": total, "limit": limit, "offset": offset})
}

// GetUserOverlays returns all overlays for a specific user (admin only)
// GET /admin/user-overlays/:id
func (h *AdminHandler) GetUserOverlays(c *gin.Context) {
	userID := c.Param("id")

	overlays, err := h.overlayRepo.ListByUserIDWithSourceCount(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to fetch user overlays", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch overlays",
		})
		return
	}

	type OverlayResponse struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		OverlayType      string `json:"overlay_type"`
		UserID           string `json:"user_id"`
		OwnerUsername    string `json:"owner_username"`
		OwnerDisplayName string `json:"owner_display_name"`
		CreatedAt        string `json:"created_at"`
		UpdatedAt        string `json:"updated_at"`
		SourcesCount     int    `json:"sources_count"`
	}

	response := make([]OverlayResponse, len(overlays))
	for i, overlay := range overlays {
		response[i] = OverlayResponse{
			ID:               overlay.ID,
			Name:             overlay.Name,
			OverlayType:      overlay.Kind(),
			UserID:           overlay.UserID,
			OwnerUsername:    overlay.OwnerUsername,
			OwnerDisplayName: overlay.OwnerDisplayName,
			CreatedAt:        overlay.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        overlay.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			SourcesCount:     overlay.SourcesCount,
		}
	}

	h.logger.Info("Listed user overlays", zap.String("user_id", userID), zap.Int("count", len(overlays)))
	c.JSON(http.StatusOK, response)
}

// GetOverlaySources returns all sources for a specific overlay (admin only)
// GET /api/v1/admin/overlays/:id/sources
func (h *AdminHandler) GetOverlaySources(c *gin.Context) {
	overlayID := c.Param("id")

	sources, err := h.sourceRepo.ListByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		h.logger.Error("Failed to fetch overlay sources", zap.String("overlay_id", overlayID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch sources",
		})
		return
	}

	h.logger.Info("Listed overlay sources", zap.String("overlay_id", overlayID), zap.Int("count", len(sources)))
	c.JSON(http.StatusOK, sources)
}
