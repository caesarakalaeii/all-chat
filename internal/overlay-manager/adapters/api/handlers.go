package api

import (
	"net/http"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
	"github.com/caesar/all-chat/internal/overlay-manager/core/ports"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/caesar/all-chat/pkg/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OverlayHandler struct {
	overlayService ports.OverlayService
	jwtSecret      string
}

func NewOverlayHandler(overlayService ports.OverlayService, jwtSecret string) *OverlayHandler {
	return &OverlayHandler{
		overlayService: overlayService,
		jwtSecret:      jwtSecret,
	}
}

func (h *OverlayHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(h.jwtSecret))
	{
		overlays := api.Group("/overlays")
		{
			overlays.GET("", h.ListOverlays)
			overlays.POST("", h.CreateOverlay)
			overlays.GET("/:id", h.GetOverlay)
			overlays.PUT("/:id", h.UpdateOverlay)
			overlays.DELETE("/:id", h.DeleteOverlay)

			overlays.GET("/:id/config", h.GetOverlayConfig)
			overlays.PUT("/:id/config", h.UpdateOverlayConfig)
		}
	}
}

type CreateOverlayRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	TwitchChannel string `json:"twitch_channel" binding:"required"`
}

func (h *OverlayHandler) CreateOverlay(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req CreateOverlayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	overlay, config, err := h.overlayService.CreateOverlay(
		c.Request.Context(),
		userID.(string),
		req.Name,
		req.Description,
		req.TwitchChannel,
	)

	if err != nil {
		logger.Error("Failed to create overlay", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create overlay"})
		return
	}

	logger.Info("Overlay created", zap.String("overlay_id", overlay.ID), zap.String("user_id", userID.(string)))

	c.JSON(http.StatusCreated, gin.H{
		"overlay": overlay,
		"config":  config,
	})
}

func (h *OverlayHandler) ListOverlays(c *gin.Context) {
	userID, _ := c.Get("user_id")

	overlays, err := h.overlayService.GetUserOverlays(c.Request.Context(), userID.(string))
	if err != nil {
		logger.Error("Failed to list overlays", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list overlays"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"overlays": overlays})
}

func (h *OverlayHandler) GetOverlay(c *gin.Context) {
	userID, _ := c.Get("user_id")
	overlayID := c.Param("id")

	overlay, err := h.overlayService.GetOverlay(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Overlay not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"overlay": overlay})
}

type UpdateOverlayRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

func (h *OverlayHandler) UpdateOverlay(c *gin.Context) {
	userID, _ := c.Get("user_id")
	overlayID := c.Param("id")

	var req UpdateOverlayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err := h.overlayService.UpdateOverlay(
		c.Request.Context(),
		overlayID,
		userID.(string),
		req.Name,
		req.Description,
		req.IsActive,
	)

	if err != nil {
		logger.Error("Failed to update overlay", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update overlay"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Overlay updated successfully"})
}

func (h *OverlayHandler) DeleteOverlay(c *gin.Context) {
	userID, _ := c.Get("user_id")
	overlayID := c.Param("id")

	err := h.overlayService.DeleteOverlay(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		logger.Error("Failed to delete overlay", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete overlay"})
		return
	}

	logger.Info("Overlay deleted", zap.String("overlay_id", overlayID))

	c.JSON(http.StatusOK, gin.H{"message": "Overlay deleted successfully"})
}

func (h *OverlayHandler) GetOverlayConfig(c *gin.Context) {
	userID, _ := c.Get("user_id")
	overlayID := c.Param("id")

	config, err := h.overlayService.GetOverlayConfig(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

type UpdateOverlayConfigRequest struct {
	TwitchChannel   string                   `json:"twitch_channel" binding:"required"`
	Enable7TV       bool                     `json:"enable_7tv"`
	EnableBTTV      bool                     `json:"enable_bttv"`
	EnableFFZ       bool                     `json:"enable_ffz"`
	DisplaySettings domain.DisplaySettings   `json:"display_settings"`
	FilterSettings  domain.FilterSettings    `json:"filter_settings"`
}

func (h *OverlayHandler) UpdateOverlayConfig(c *gin.Context) {
	userID, _ := c.Get("user_id")
	overlayID := c.Param("id")

	var req UpdateOverlayConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	config := &domain.OverlayConfig{
		TwitchChannel:   req.TwitchChannel,
		Enable7TV:       req.Enable7TV,
		EnableBTTV:      req.EnableBTTV,
		EnableFFZ:       req.EnableFFZ,
		DisplaySettings: req.DisplaySettings,
		FilterSettings:  req.FilterSettings,
	}

	err := h.overlayService.UpdateOverlayConfig(c.Request.Context(), overlayID, userID.(string), config)
	if err != nil {
		logger.Error("Failed to update overlay config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config updated successfully"})
}
