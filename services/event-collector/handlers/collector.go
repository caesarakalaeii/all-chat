package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/event-collector/collectors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CollectorHandler handles collector management endpoints
type CollectorHandler struct {
	manager *collectors.CollectorManager
	logger  *zap.Logger
}

// NewCollectorHandler creates a new collector handler
func NewCollectorHandler(manager *collectors.CollectorManager, logger *zap.Logger) *CollectorHandler {
	return &CollectorHandler{
		manager: manager,
		logger:  logger,
	}
}

// StartTwitchCollectorRequest represents the request body for starting a collector
type StartTwitchCollectorRequest struct {
	UserID            string `json:"user_id" binding:"required"`
	TwitchID          string `json:"twitch_id" binding:"required"`
	TwitchAccessToken string `json:"twitch_access_token" binding:"required"`
}

// StartTwitchCollector handles POST /api/v1/collectors/twitch/start
func (h *CollectorHandler) StartTwitchCollector(c *gin.Context) {
	var req StartTwitchCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	mapping := &collectors.UserBroadcasterMapping{
		UserID:            userID,
		TwitchID:          req.TwitchID,
		TwitchAccessToken: req.TwitchAccessToken,
	}

	if err := h.manager.StartTwitchCollector(c.Request.Context(), mapping); err != nil {
		h.logger.Error("Failed to start Twitch collector", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "started",
		"user_id":       userID,
		"broadcaster_id": req.TwitchID,
	})
}

// StopTwitchCollector handles POST /api/v1/collectors/twitch/stop
func (h *CollectorHandler) StopTwitchCollector(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query parameter required"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.manager.StopTwitchCollector(userID); err != nil {
		h.logger.Error("Failed to stop Twitch collector", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "stopped",
		"user_id": userID,
	})
}

// ListActiveCollectors handles GET /api/v1/collectors/active
func (h *CollectorHandler) ListActiveCollectors(c *gin.Context) {
	userIDs := h.manager.ListActiveCollectors()

	c.JSON(http.StatusOK, gin.H{
		"active_collectors": userIDs,
		"count":             len(userIDs),
	})
}

// GetCollectorStatus handles GET /api/v1/collectors/twitch/:user_id
func (h *CollectorHandler) GetCollectorStatus(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	client, exists := h.manager.GetTwitchCollector(userID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "not_running",
			"user_id": userID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "running",
		"user_id":    userID,
		"session_id": client.GetSessionID(),
		"broadcaster_id": client.BroadcasterID,
	})
}
