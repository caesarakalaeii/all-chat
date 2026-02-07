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
// Note: overlay_id is intentionally NOT included to prevent viewers from
// accessing the secret overlay ID or triggering YouTube listener polling
type StreamerInfoResponse struct {
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name,omitempty"`
	Platforms   []PlatformInfo `json:"platforms"`
}

// HandleGetStreamerInfo returns information about a streamer and their active platforms
// Supports lookup by:
// - Username (case-insensitive) - works for Twitch, Kick, TikTok
// - Channel handle/ID - works for YouTube (@handle or channel ID)
func (h *StreamerInfoHandler) HandleGetStreamerInfo(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	ctx := c.Request.Context()

	// Try Method 1: Get user by username (case-insensitive)
	user, err := h.userRepo.GetByUsername(ctx, username)

	// If not found by username, try Method 2: Look up by channel_id in overlay_chat_sources
	// This handles YouTube handles/channel IDs which aren't stored as usernames
	if err != nil {
		h.log.Debug("User not found by username, trying channel_id lookup",
			zap.String("username", username))

		// Query to find user by channel_id (case-insensitive for handles)
		// Note: Don't filter by is_active - we want to find streamers who have
		// All-Chat configured even if they're not currently live
		channelQuery := `
			SELECT DISTINCT u.id, u.username
			FROM users u
			INNER JOIN overlays o ON o.user_id = u.id
			INNER JOIN overlay_chat_sources ocs ON ocs.overlay_id = o.id
			WHERE LOWER(ocs.channel_id) = LOWER($1)
			LIMIT 1
		`

		var userID, foundUsername string
		err = h.db.QueryRow(ctx, channelQuery, username).Scan(&userID, &foundUsername)
		if err != nil {
			h.log.Error("Streamer not found by username or channel_id",
				zap.Error(err),
				zap.String("lookup_value", username))
			c.JSON(http.StatusNotFound, gin.H{"error": "streamer not found"})
			return
		}

		// Now get the full user object
		user, err = h.userRepo.GetByUsername(ctx, foundUsername)
		if err != nil {
			h.log.Error("Failed to get user after channel_id lookup", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
			return
		}
	}

	// Query all configured sources from overlay_chat_sources
	// Note: Return all configured sources regardless of is_active status
	// so the extension can show the badge even when the streamer isn't live
	query := `
		SELECT DISTINCT
			ocs.platform,
			ocs.channel_id,
			ocs.channel_name,
			ocs.is_active
		FROM overlay_chat_sources ocs
		INNER JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.user_id = $1
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

	// Return response (overlay_id intentionally excluded for security)
	response := StreamerInfoResponse{
		Username:    user.Username,
		DisplayName: user.Username,
		Platforms:   platforms,
	}

	c.JSON(http.StatusOK, response)
}
