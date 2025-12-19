package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// StreamerInfoHandler handles requests for streamer information
type StreamerInfoHandler struct {
	log      *zap.Logger
	userRepo *repository.UserRepository
	db       *pgxpool.Pool
}

// NewStreamerInfoHandler creates a new streamer info handler
func NewStreamerInfoHandler(log *zap.Logger, userRepo *repository.UserRepository, db *pgxpool.Pool) *StreamerInfoHandler {
	return &StreamerInfoHandler{
		log:      log.Named("streamer-info"),
		userRepo: userRepo,
		db:       db,
	}
}

// PlatformInfo represents information about a platform for a streamer
type PlatformInfo struct {
	Platform    string `json:"platform"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	IsActive    bool   `json:"is_active"`
}

// StreamerInfoResponse is the response for streamer info
type StreamerInfoResponse struct {
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name,omitempty"`
	Platforms   []PlatformInfo `json:"platforms"`
}

// HandleGetStreamerInfo returns information about a streamer and their active platforms
func (h *StreamerInfoHandler) HandleGetStreamerInfo(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	ctx := c.Request.Context()

	// Get user by username
	user, err := h.userRepo.GetByUsername(ctx, username)
	if err != nil {
		h.log.Error("Failed to get user", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusNotFound, gin.H{"error": "streamer not found"})
		return
	}

	// Query active sources from overlay_chat_sources
	query := `
		SELECT DISTINCT
			ocs.platform,
			ocs.channel_id,
			ocs.channel_name,
			ocs.is_active
		FROM overlay_chat_sources ocs
		INNER JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.user_id = $1 AND ocs.is_active = true
		ORDER BY ocs.platform
	`

	rows, err := h.db.Query(ctx, query, user.ID)
	if err != nil {
		h.log.Error("Failed to query active sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch platform info"})
		return
	}
	defer rows.Close()

	platforms := make([]PlatformInfo, 0)
	for rows.Next() {
		var p PlatformInfo
		if err := rows.Scan(&p.Platform, &p.ChannelID, &p.ChannelName, &p.IsActive); err != nil {
			h.log.Error("Failed to scan platform info", zap.Error(err))
			continue
		}
		platforms = append(platforms, p)
	}

	// Return response
	response := StreamerInfoResponse{
		Username:    user.Username,
		DisplayName: user.Username, // TODO: Add display_name field to users table if needed
		Platforms:   platforms,
	}

	c.JSON(http.StatusOK, response)
}
