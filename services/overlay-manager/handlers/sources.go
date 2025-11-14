package handlers

import (
	"context"
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)

// SourceRepository defines the interface for source persistence
type SourceRepository interface {
	Create(ctx context.Context, source *models.ChatSource) error
	ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error)
	GetByID(ctx context.Context, id string) (*models.ChatSource, error)
	Delete(ctx context.Context, id string) error
}

// SourcesHandler handles HTTP requests for overlay chat sources
type SourcesHandler struct {
	sourceRepo  SourceRepository
	overlayRepo OverlayRepository
}

// NewSourcesHandler creates a new sources handler
func NewSourcesHandler(sourceRepo SourceRepository, overlayRepo OverlayRepository) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo:  sourceRepo,
		overlayRepo: overlayRepo,
	}
}

// HandleListSources handles GET /:id/sources
func (h *SourcesHandler) HandleListSources(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	// Verify user owns this overlay
	_, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get sources
	sources, err := h.sourceRepo.ListByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sources"})
		return
	}

	// Ensure we always return an array, even if empty
	if sources == nil {
		sources = []*models.ChatSource{}
	}

	c.JSON(http.StatusOK, sources)
}

// HandleAddSource handles POST /:id/sources
func (h *SourcesHandler) HandleAddSource(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	// Verify user owns this overlay
	_, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	var req struct {
		Platform    string `json:"platform" binding:"required"`
		ChannelID   string `json:"channel_id" binding:"required"`
		ChannelName string `json:"channel_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use channel_id as channel_name if not provided
	channelName := req.ChannelName
	if channelName == "" {
		channelName = req.ChannelID
	}

	source := &models.ChatSource{
		OverlayID:    overlayID,
		Platform:     req.Platform,
		ChannelID:    req.ChannelID,
		ChannelName:  channelName,
		AuthRequired: req.Platform == "youtube", // YouTube requires OAuth
		Config:       make(map[string]interface{}),
		IsActive:     true,
	}

	// Validate
	if err := source.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create in database
	if err := h.sourceRepo.Create(c.Request.Context(), source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create source"})
		return
	}

	c.JSON(http.StatusCreated, source)
}

// HandleDeleteSource handles DELETE /:id/sources/:source_id
func (h *SourcesHandler) HandleDeleteSource(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")
	sourceID := c.Param("source_id")

	// Verify user owns this overlay
	_, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Verify source belongs to this overlay
	source, err := h.sourceRepo.GetByID(c.Request.Context(), sourceID)
	if err != nil || source.OverlayID != overlayID {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found"})
		return
	}

	// Delete source
	if err := h.sourceRepo.Delete(c.Request.Context(), sourceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete source"})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegisterRoutes registers source routes
// Note: Must be registered on the overlay detail routes (/:id/sources)
func (h *SourcesHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/sources", h.HandleListSources)
	router.POST("/sources", h.HandleAddSource)
	router.DELETE("/sources/:source_id", h.HandleDeleteSource)
}
