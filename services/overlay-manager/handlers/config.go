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

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)

// OverlayConfigRepository defines persistence operations needed by the handler.
// Having an interface keeps handlers easy to test.
type OverlayConfigRepository interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.OverlayConfig, error)
	Update(ctx context.Context, config *models.OverlayConfig) error
}

// ConfigHandler manages overlay configuration routes.
type ConfigHandler struct {
	repo     OverlayConfigRepository
	overlays OverlayRepository
	sources  SourceRepository
}

// NewConfigHandler returns a ConfigHandler.
func NewConfigHandler(repo OverlayConfigRepository, overlays OverlayRepository, sources SourceRepository) *ConfigHandler {
	return &ConfigHandler{repo: repo, overlays: overlays, sources: sources}
}

// HandleGetConfig returns the configuration for an overlay owned by the requester.
func (h *ConfigHandler) HandleGetConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleUpdateConfig updates the overlay configuration for the owner.
func (h *ConfigHandler) HandleUpdateConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	var req struct {
		DisplaySettings map[string]any `json:"display_settings"`
		FilterSettings  map[string]any `json:"filter_settings"`
		Enable7TV       *bool          `json:"enable_7tv"`
		EnableBTTV      *bool          `json:"enable_bttv"`
		EnableFFZ       *bool          `json:"enable_ffz"`
		CustomCSS       *string        `json:"custom_css"`
		VisualSettings  map[string]any `json:"visual_settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DisplaySettings != nil {
		config.DisplaySettings = req.DisplaySettings
	}
	if req.FilterSettings != nil {
		config.FilterSettings = req.FilterSettings
	}
	if req.Enable7TV != nil {
		config.Enable7TV = *req.Enable7TV
	}
	if req.EnableBTTV != nil {
		config.EnableBTTV = *req.EnableBTTV
	}
	if req.EnableFFZ != nil {
		config.EnableFFZ = *req.EnableFFZ
	}
	if req.CustomCSS != nil {
		config.CustomCSS = *req.CustomCSS
	}
	if req.VisualSettings != nil {
		config.VisualSettings = req.VisualSettings
	}

	if err := h.repo.Update(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update overlay config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleGetPublicConfig exposes safe configuration fields without authentication.
func (h *ConfigHandler) HandleGetPublicConfig(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByID(c.Request.Context(), overlayID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	// Get sources with their active status
	sources, err := h.sources.ListByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		// Log error but don't fail the request
		sources = []*models.ChatSource{}
	}

	// Build simplified source status for public consumption
	sourceStatus := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		sourceStatus = append(sourceStatus, map[string]interface{}{
			"platform":     source.Platform,
			"channel_id":   source.ChannelID,
			"channel_name": source.ChannelName,
			"is_active":    source.IsActive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"display_settings": config.DisplaySettings,
		"filter_settings":  config.FilterSettings,
		"custom_css":       config.CustomCSS,
		"visual_settings":  config.VisualSettings,
		"sources":          sourceStatus,
	})
}
