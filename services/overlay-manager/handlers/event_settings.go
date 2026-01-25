package handlers

import (
	"context"
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)

// EventSettingsRepository defines persistence operations for event settings
type EventSettingsRepository interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.EventSettings, error)
	Update(ctx context.Context, settings *models.EventSettings) error
}

// EventSettingsHandler manages event settings routes
type EventSettingsHandler struct {
	repo     EventSettingsRepository
	overlays OverlayRepository
}

// NewEventSettingsHandler creates a new event settings handler
func NewEventSettingsHandler(repo EventSettingsRepository, overlays OverlayRepository) *EventSettingsHandler {
	return &EventSettingsHandler{
		repo:     repo,
		overlays: overlays,
	}
}

// HandleGetEventSettings returns event settings for an overlay owned by the requester
func (h *EventSettingsHandler) HandleGetEventSettings(c *gin.Context) {
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

	// Verify overlay ownership
	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get event settings
	settings, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event settings not found"})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// HandleUpdateEventSettings updates event settings for an overlay owned by the requester
func (h *EventSettingsHandler) HandleUpdateEventSettings(c *gin.Context) {
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

	// Verify overlay ownership
	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get current settings
	settings, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event settings not found"})
		return
	}

	// Bind request body
	var req models.EventSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update settings (keep overlay_id and id from existing)
	req.ID = settings.ID
	req.OverlayID = settings.OverlayID

	// Update in database
	if err := h.repo.Update(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update event settings"})
		return
	}

	c.JSON(http.StatusOK, &req)
}

// HandleGetPublicEventSettings returns event settings without authentication (for overlay display)
func (h *EventSettingsHandler) HandleGetPublicEventSettings(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	// Verify overlay exists (no ownership check for public endpoint)
	if _, err := h.overlays.GetByID(c.Request.Context(), overlayID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get event settings
	settings, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event settings not found"})
		return
	}

	c.JSON(http.StatusOK, settings)
}
