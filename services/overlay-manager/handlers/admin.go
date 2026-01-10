package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	overlayRepo *repository.OverlayRepository
	sourceRepo  *repository.SourceRepository
	logger      *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(overlayRepo *repository.OverlayRepository, sourceRepo *repository.SourceRepository, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		overlayRepo: overlayRepo,
		sourceRepo:  sourceRepo,
		logger:      logger,
	}
}

// ListOverlays returns all overlays in the system (admin only)
// GET /api/v1/admin/overlays
func (h *AdminHandler) ListOverlays(c *gin.Context) {
	overlays, err := h.overlayRepo.GetAllOverlays(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to fetch overlays", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch overlays",
		})
		return
	}

	// For each overlay, get source count
	type OverlayResponse struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		UserID       string `json:"user_id"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		SourcesCount int    `json:"sources_count"`
	}

	response := make([]OverlayResponse, len(overlays))
	for i, overlay := range overlays {
		sources, _ := h.sourceRepo.ListByOverlayID(c.Request.Context(), overlay.ID)
		response[i] = OverlayResponse{
			ID:           overlay.ID,
			Name:         overlay.Name,
			UserID:       overlay.UserID,
			CreatedAt:    overlay.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    overlay.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			SourcesCount: len(sources),
		}
	}

	h.logger.Info("Listed overlays", zap.Int("count", len(overlays)))
	c.JSON(http.StatusOK, response)
}

// ListAllSources returns all sources across all overlays (admin only)
// GET /api/v1/admin/sources
func (h *AdminHandler) ListAllSources(c *gin.Context) {
	sources, err := h.sourceRepo.GetAllSources(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to fetch sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch sources",
		})
		return
	}

	// Include overlay information for each source
	type SourceResponse struct {
		ID          string `json:"id"`
		OverlayID   string `json:"overlay_id"`
		OverlayName string `json:"overlay_name"`
		Platform    string `json:"platform"`
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		IsActive    bool   `json:"is_active"`
		CreatedAt   string `json:"created_at"`
		UserID      string `json:"user_id"`
	}

	response := make([]SourceResponse, 0)
	for _, source := range sources {
		overlay, err := h.overlayRepo.GetByID(c.Request.Context(), source.OverlayID)
		if err != nil {
			// Skip if overlay not found
			h.logger.Warn("Overlay not found for source",
				zap.String("source_id", source.ID),
				zap.String("overlay_id", source.OverlayID),
			)
			continue
		}

		response = append(response, SourceResponse{
			ID:          source.ID,
			OverlayID:   source.OverlayID,
			OverlayName: overlay.Name,
			Platform:    source.Platform,
			ChannelID:   source.ChannelID,
			ChannelName: source.ChannelName,
			IsActive:    source.IsActive,
			CreatedAt:   source.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UserID:      overlay.UserID,
		})
	}

	h.logger.Info("Listed all sources", zap.Int("count", len(response)))
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
