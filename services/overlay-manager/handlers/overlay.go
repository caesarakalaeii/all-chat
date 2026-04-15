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

// OverlayRepository defines the interface for overlay persistence
type OverlayRepository interface {
	Create(ctx context.Context, overlay *models.Overlay) error
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error)
	ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error)
	Update(ctx context.Context, overlay *models.Overlay) error
	Delete(ctx context.Context, id string) error
	UnsetAllPublicForUser(ctx context.Context, userID, excludeID string) error
}

// OverlayHandler handles HTTP requests for overlays
type OverlayHandler struct {
	repo       OverlayRepository
	sourceRepo SourceRepository
	configRepo OverlayConfigRepository
}

// NewOverlayHandler creates a new overlay handler
func NewOverlayHandler(repo OverlayRepository, sourceRepo SourceRepository, configRepo OverlayConfigRepository) *OverlayHandler {
	return &OverlayHandler{repo: repo, sourceRepo: sourceRepo, configRepo: configRepo}
}

// HandleCreateOverlay handles POST /overlays
func (h *OverlayHandler) HandleCreateOverlay(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		IsActive    *bool  `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default is_active to true if not provided
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Build overlay struct and validate before any DB calls.
	// is_public_for_viewers is resolved below once we know whether this is the user's first overlay.
	overlay := &models.Overlay{
		UserID:      userID.(string),
		Name:        req.Name,
		Description: req.Description,
		IsActive:    isActive,
	}

	// Validate early — catches input errors cheaply before hitting the DB.
	if err := overlay.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only set is_public_for_viewers=true automatically if the user has no existing overlays.
	// This preserves the designated extension overlay when a user creates additional overlays.
	existingOverlays, err := h.repo.ListByUserID(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create overlay"})
		return
	}
	overlay.IsPublicForViewers = len(existingOverlays) == 0

	// Create in database
	if err := h.repo.Create(c.Request.Context(), overlay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create overlay"})
		return
	}

	c.JSON(http.StatusCreated, overlay)
}

// HandleListOverlays handles GET /overlays
func (h *OverlayHandler) HandleListOverlays(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlays, err := h.repo.ListByUserID(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list overlays"})
		return
	}

	// Ensure we always return an array, even if empty (not null)
	if overlays == nil {
		overlays = []*models.Overlay{}
	}

	c.JSON(http.StatusOK, overlays)
}

// HandleGetOverlay handles GET /overlays/:id
func (h *OverlayHandler) HandleGetOverlay(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	overlay, err := h.repo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	c.JSON(http.StatusOK, overlay)
}

// HandleUpdateOverlay handles PUT /overlays/:id
func (h *OverlayHandler) HandleUpdateOverlay(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	// First check if overlay exists and belongs to user
	overlay, err := h.repo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Bind update request
	var req struct {
		Name                *string `json:"name"`
		Description         *string `json:"description"`
		IsActive            *bool   `json:"is_active"`
		IsPublicForViewers  *bool   `json:"is_public_for_viewers"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.Name != nil {
		overlay.Name = *req.Name
	}
	if req.Description != nil {
		overlay.Description = *req.Description
	}
	if req.IsActive != nil {
		overlay.IsActive = *req.IsActive
	}
	if req.IsPublicForViewers != nil {
		overlay.IsPublicForViewers = *req.IsPublicForViewers
	}

	// Validate
	if err := overlay.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If making this overlay public, unset all other public overlays for this user
	// (only one public overlay per user is allowed)
	if overlay.IsPublicForViewers {
		if err := h.repo.UnsetAllPublicForUser(c.Request.Context(), userID.(string), overlayID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update overlay"})
			return
		}
	}

	// Update in database
	if err := h.repo.Update(c.Request.Context(), overlay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update overlay"})
		return
	}

	c.JSON(http.StatusOK, overlay)
}

// HandleDeleteOverlay handles DELETE /overlays/:id
func (h *OverlayHandler) HandleDeleteOverlay(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	// First check if overlay exists and belongs to user
	target, err := h.repo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	wasPublic := target.IsPublicForViewers

	// Delete from database
	if err := h.repo.Delete(c.Request.Context(), overlayID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete overlay"})
		return
	}

	// If the deleted overlay was the public extension overlay, promote the oldest
	// remaining active overlay so viewers always have a valid target.
	if wasPublic {
		remaining, err := h.repo.ListByUserID(c.Request.Context(), userID.(string))
		if err == nil {
			// ListByUserID orders by created_at DESC; iterate to find the oldest active one.
			var oldest *models.Overlay
			for i := len(remaining) - 1; i >= 0; i-- {
				if remaining[i].IsActive {
					oldest = remaining[i]
					break
				}
			}
			if oldest != nil {
				oldest.IsPublicForViewers = true
				// Ignore promotion error — the delete already succeeded; a best-effort
				// promotion failure should not roll back or change the response code.
				_ = h.repo.Update(c.Request.Context(), oldest)
			}
		}
	}

	c.Status(http.StatusNoContent)
}

// HandleCloneOverlay handles POST /overlays/:id/clone
func (h *OverlayHandler) HandleCloneOverlay(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	source, err := h.repo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	newOverlay := &models.Overlay{
		UserID:             userID.(string),
		Name:               source.Name + " (copy)",
		Description:        source.Description,
		IsActive:           source.IsActive,
		IsPublicForViewers: false,
	}

	if err := h.repo.Create(c.Request.Context(), newOverlay); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clone overlay"})
		return
	}

	// Copy config if it exists
	if sourceConfig, err := h.configRepo.GetByOverlayID(c.Request.Context(), overlayID); err == nil {
		newConfig := &models.OverlayConfig{
			OverlayID:       newOverlay.ID,
			DisplaySettings: sourceConfig.DisplaySettings,
			FilterSettings:  sourceConfig.FilterSettings,
			Enable7TV:       sourceConfig.Enable7TV,
			EnableBTTV:      sourceConfig.EnableBTTV,
			EnableFFZ:       sourceConfig.EnableFFZ,
			CustomCSS:       sourceConfig.CustomCSS,
			VisualSettings:  sourceConfig.VisualSettings,
		}
		_ = h.configRepo.Update(c.Request.Context(), newConfig)
	}

	// Copy sources (skip shared_overlay)
	if sourceSources, err := h.sourceRepo.ListByOverlayID(c.Request.Context(), overlayID); err == nil {
		for _, s := range sourceSources {
			if s.Platform == "shared_overlay" {
				continue
			}
			cloned := &models.ChatSource{
				OverlayID:     newOverlay.ID,
				Platform:      s.Platform,
				ChannelID:     s.ChannelID,
				ChannelName:   s.ChannelName,
				ChannelHandle: s.ChannelHandle,
				AuthRequired:  s.AuthRequired,
				IsActive:      s.IsActive,
				Config:        s.Config,
			}
			_ = h.sourceRepo.Create(c.Request.Context(), cloned)
		}
	}

	c.JSON(http.StatusCreated, newOverlay)
}

// RegisterRoutes registers overlay routes (no /overlays prefix - API Gateway strips /api/v1/overlays)
func (h *OverlayHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/", h.HandleCreateOverlay)
	router.GET("/", h.HandleListOverlays)
	router.GET("/:id", h.HandleGetOverlay)
	router.PUT("/:id", h.HandleUpdateOverlay)
	router.DELETE("/:id", h.HandleDeleteOverlay)
	router.POST("/:id/clone", h.HandleCloneOverlay)
}
