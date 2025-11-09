package api

import (
	"net/http"

	"github.com/caesar/all-chat/internal/emote-service/core/ports"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type EmoteHandler struct {
	emoteService ports.EmoteService
}

func NewEmoteHandler(emoteService ports.EmoteService) *EmoteHandler {
	return &EmoteHandler{
		emoteService: emoteService,
	}
}

func (h *EmoteHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/emotes")
	{
		api.GET("/global/:provider", h.GetGlobalEmotes)
		api.GET("/channel/:channel/:provider", h.GetChannelEmotes)
		api.POST("/refresh", h.RefreshCache)
	}
}

func (h *EmoteHandler) GetGlobalEmotes(c *gin.Context) {
	provider := c.Param("provider")

	emotes, err := h.emoteService.GetGlobalEmotes(c.Request.Context(), provider)
	if err != nil {
		logger.Error("Failed to get global emotes", zap.String("provider", provider), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch emotes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"emotes": emotes})
}

func (h *EmoteHandler) GetChannelEmotes(c *gin.Context) {
	channel := c.Param("channel")
	provider := c.Param("provider")

	emotes, err := h.emoteService.GetChannelEmotes(c.Request.Context(), channel, provider)
	if err != nil {
		logger.Error("Failed to get channel emotes", zap.String("channel", channel), zap.String("provider", provider), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch emotes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"emotes": emotes})
}

func (h *EmoteHandler) RefreshCache(c *gin.Context) {
	if err := h.emoteService.RefreshCache(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache refreshed successfully"})
}
