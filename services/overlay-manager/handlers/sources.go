package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
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
	db          *pgxpool.Pool
	logger      *zap.Logger
}

// NewSourcesHandler creates a new sources handler
func NewSourcesHandler(sourceRepo SourceRepository, overlayRepo OverlayRepository, db *pgxpool.Pool, logger *zap.Logger) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo:  sourceRepo,
		overlayRepo: overlayRepo,
		db:          db,
		logger:      logger,
	}
}

// copyYouTubeTokenForChannel copies the admin's YouTube OAuth token to a new channel
// This allows admins to add YouTube channels manually (by link) without OAuth flow
func (h *SourcesHandler) copyYouTubeTokenForChannel(ctx context.Context, adminUserID, newChannelID string) error {
	// Step 1: Find the best YouTube token for this admin
	// Prefer non-expired tokens, then most recently updated
	var existingToken struct {
		AccessToken        string
		RefreshToken       string
		TokenType          string
		Expiry             string // Store as string to avoid timestamp parsing issues
		EncryptionVersion  int
	}

	query := `
		SELECT access_token, refresh_token, token_type,
		       expiry::text, encryption_version
		FROM youtube_oauth_tokens
		WHERE user_id = $1
		  AND expiry > NOW()  -- Only select non-expired tokens
		ORDER BY expiry DESC  -- Prefer tokens that expire furthest in the future
		LIMIT 1
	`

	err := h.db.QueryRow(ctx, query, adminUserID).Scan(
		&existingToken.AccessToken,
		&existingToken.RefreshToken,
		&existingToken.TokenType,
		&existingToken.Expiry,
		&existingToken.EncryptionVersion,
	)

	if err != nil {
		return fmt.Errorf("admin has no valid (non-expired) YouTube OAuth token - please re-authorize YouTube: %w", err)
	}

	// Step 2: Copy token to new channel_id (insert or update)
	insertQuery := `
		INSERT INTO youtube_oauth_tokens (
			user_id, channel_id, access_token, refresh_token,
			token_type, expiry, encryption_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::timestamp, $7, NOW(), NOW())
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			encryption_version = EXCLUDED.encryption_version,
			updated_at = NOW()
	`

	_, err = h.db.Exec(ctx, insertQuery,
		adminUserID,
		newChannelID,
		existingToken.AccessToken,
		existingToken.RefreshToken,
		existingToken.TokenType,
		existingToken.Expiry,
		existingToken.EncryptionVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to copy YouTube token: %w", err)
	}

	h.logger.Info("Copied YouTube OAuth token for new channel",
		zap.String("admin_user_id", adminUserID),
		zap.String("new_channel_id", newChannelID),
	)

	return nil
}

// copyKickTokenForChannel copies the admin's Kick OAuth token to a new channel
// This allows admins to add Kick channels manually without OAuth flow
func (h *SourcesHandler) copyKickTokenForChannel(ctx context.Context, adminUserID, newChannelID string) error {
	// Step 1: Find the best Kick token for this admin
	// Prefer non-expired tokens, then most recently updated
	var existingToken struct {
		AccessToken  string
		RefreshToken string
		TokenType    string
		Expiry       string // Store as string to avoid timestamp parsing issues
	}

	query := `
		SELECT access_token, refresh_token, token_type, expiry::text
		FROM kick_oauth_tokens
		WHERE user_id = $1
		  AND expiry > NOW()  -- Only select non-expired tokens
		ORDER BY expiry DESC  -- Prefer tokens that expire furthest in the future
		LIMIT 1
	`

	err := h.db.QueryRow(ctx, query, adminUserID).Scan(
		&existingToken.AccessToken,
		&existingToken.RefreshToken,
		&existingToken.TokenType,
		&existingToken.Expiry,
	)

	if err != nil {
		return fmt.Errorf("admin has no valid (non-expired) Kick OAuth token - please re-authorize Kick: %w", err)
	}

	// Step 2: Copy token to new channel_id (insert or update)
	insertQuery := `
		INSERT INTO kick_oauth_tokens (
			user_id, channel_id, access_token, refresh_token,
			token_type, expiry, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::timestamp, NOW(), NOW())
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			updated_at = NOW()
	`

	_, err = h.db.Exec(ctx, insertQuery,
		adminUserID,
		newChannelID,
		existingToken.AccessToken,
		existingToken.RefreshToken,
		existingToken.TokenType,
		existingToken.Expiry,
	)

	if err != nil {
		return fmt.Errorf("failed to copy Kick token: %w", err)
	}

	h.logger.Info("Copied Kick OAuth token for new channel",
		zap.String("admin_user_id", adminUserID),
		zap.String("new_channel_id", newChannelID),
	)

	return nil
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
		h.logger.Error("Failed to list sources", zap.Error(err), zap.String("overlay_id", overlayID))
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
		Platform      string `json:"platform" binding:"required"`
		ChannelID     string `json:"channel_id" binding:"required"`
		ChannelName   string `json:"channel_name"`
		ChannelHandle string `json:"channel_handle"`
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

	// For Twitch, validate that channel_id is a valid username (lowercase alphanumeric + underscore)
	// This prevents display names from being stored (e.g., "شوشو" instead of "shahin200x")
	channelID := req.ChannelID
	if req.Platform == "twitch" {
		// Twitch usernames must be lowercase alphanumeric + underscore only
		channelID = strings.ToLower(strings.TrimSpace(channelID))

		// Basic validation: only allow alphanumeric and underscore
		for _, r := range channelID {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Invalid Twitch username: '%s'. Twitch usernames can only contain lowercase letters, numbers, and underscores.", channelID),
				})
				return
			}
		}
	}

	// For Kick, validate that channel_id is a valid slug (not a numeric ID)
	// Kick channel IDs should be usernames like "xqc", not numeric IDs like "52390613"
	if req.Platform == "kick" {
		channelID = strings.TrimSpace(channelID)

		// Check if it's purely numeric (invalid for Kick slugs)
		isNumeric := true
		for _, r := range channelID {
			if r < '0' || r > '9' {
				isNumeric = false
				break
			}
		}

		if isNumeric {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid Kick channel ID: '%s'. Kick requires a username/slug (like 'xqc'), not a numeric ID. Please provide the channel username.", channelID),
			})
			return
		}
	}

	// For shared_overlay, validate the caller has an accepted share granting access to this overlay.
	// Uses a direct DB query against share_requests (same DB, no cross-service call needed).
	if req.Platform == "shared_overlay" {
		if h.db == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no accepted share relationship for this overlay",
			})
			return
		}

		var shareCount int
		dbErr := h.db.QueryRow(c.Request.Context(),
			`SELECT COUNT(*) FROM share_requests
			 WHERE sender_overlay_id = $1
			   AND recipient_user_id = $2
			   AND status = 'accepted'`,
			channelID, userID.(string),
		).Scan(&shareCount)
		if dbErr != nil || shareCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no accepted share relationship for this overlay",
			})
			return
		}

		// Fetch overlay name to use as channel_name (not the raw UUID)
		var overlayName string
		if nameErr := h.db.QueryRow(c.Request.Context(),
			`SELECT name FROM overlays WHERE id = $1`, channelID,
		).Scan(&overlayName); nameErr == nil {
			channelName = overlayName
		}
		// If fetch fails, channelName retains the fallback value (channel_id) — acceptable
	}

	var channelHandle *string
	if req.ChannelHandle != "" {
		channelHandle = &req.ChannelHandle
	}

	source := &models.ChatSource{
		OverlayID:     overlayID,
		Platform:      req.Platform,
		ChannelID:     channelID,
		ChannelName:   channelName,
		ChannelHandle: channelHandle,
		AuthRequired:  req.Platform == "youtube", // YouTube requires OAuth
		Config:        make(map[string]interface{}),
		IsActive:      false, // Will be set to true by listeners when they connect
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

	// CRITICAL: For YouTube sources added manually, copy admin's OAuth token
	// This allows admins to add YouTube channels by link without OAuth flow
	// The admin's token will be used to poll the new channel
	if req.Platform == "youtube" && h.db != nil {
		if err := h.copyYouTubeTokenForChannel(c.Request.Context(), userID.(string), channelID); err != nil {
			// Log error but don't fail the request - source was created successfully
			// Admin will need to authenticate via OAuth if token copy fails
			h.logger.Warn("Failed to copy YouTube token for new channel",
				zap.String("user_id", userID.(string)),
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		} else {
			h.logger.Info("Successfully copied YouTube token for manual channel addition",
				zap.String("user_id", userID.(string)),
				zap.String("channel_id", channelID),
			)
		}
	}

	// CRITICAL: For Kick sources added manually, copy admin's OAuth token
	// This allows admins to add Kick channels without OAuth flow
	// The admin's token will be used to connect to the new channel
	if req.Platform == "kick" && h.db != nil {
		if err := h.copyKickTokenForChannel(c.Request.Context(), userID.(string), channelID); err != nil {
			// Log error but don't fail the request - source was created successfully
			// Admin will need to authenticate via OAuth if token copy fails
			h.logger.Warn("Failed to copy Kick token for new channel",
				zap.String("user_id", userID.(string)),
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		} else {
			h.logger.Info("Successfully copied Kick token for manual channel addition",
				zap.String("user_id", userID.(string)),
				zap.String("channel_id", channelID),
			)
		}
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

// HandleAddSourceAuto handles POST /internal/overlays/:id/sources/auto
// This is an internal endpoint called by auth-service after OAuth flow
func (h *SourcesHandler) HandleAddSourceAuto(c *gin.Context) {
	// Get user ID from JWT context (set by auth middleware)
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
		Platform      string `json:"platform" binding:"required"`
		ChannelID     string `json:"channel_id" binding:"required"`
		ChannelName   string `json:"channel_name"`
		ChannelHandle string `json:"channel_handle"`
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

	// For Twitch, validate that channel_id is a valid username (lowercase alphanumeric + underscore)
	// This prevents display names from being stored (e.g., "شوشو" instead of "shahin200x")
	channelID := req.ChannelID
	if req.Platform == "twitch" {
		// Twitch usernames must be lowercase alphanumeric + underscore only
		channelID = strings.ToLower(strings.TrimSpace(channelID))

		// Basic validation: only allow alphanumeric and underscore
		for _, r := range channelID {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Invalid Twitch username: '%s'. Twitch usernames can only contain lowercase letters, numbers, and underscores.", channelID),
				})
				return
			}
		}
	}

	// For Kick, validate that channel_id is a valid slug (not a numeric ID)
	// Kick channel IDs should be usernames like "xqc", not numeric IDs like "52390613"
	if req.Platform == "kick" {
		channelID = strings.TrimSpace(channelID)

		// Check if it's purely numeric (invalid for Kick slugs)
		isNumeric := true
		for _, r := range channelID {
			if r < '0' || r > '9' {
				isNumeric = false
				break
			}
		}

		if isNumeric {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid Kick channel ID: '%s'. Kick requires a username/slug (like 'xqc'), not a numeric ID. Please provide the channel username.", channelID),
			})
			return
		}
	}

	var channelHandle *string
	if req.ChannelHandle != "" {
		channelHandle = &req.ChannelHandle
	}

	source := &models.ChatSource{
		OverlayID:     overlayID,
		Platform:      req.Platform,
		ChannelID:     channelID,
		ChannelName:   channelName,
		ChannelHandle: channelHandle,
		AuthRequired:  req.Platform == "youtube" || req.Platform == "kick",
		Config:        make(map[string]interface{}),
		IsActive:      true,
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

// RegisterRoutes registers source routes
// Note: Must be registered on the overlay detail routes (/:id/sources)
func (h *SourcesHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/sources", h.HandleListSources)
	router.POST("/sources", h.HandleAddSource)
	router.DELETE("/sources/:source_id", h.HandleDeleteSource)
}

// RegisterInternalRoutes registers internal source routes (called by other services)
func (h *SourcesHandler) RegisterInternalRoutes(router gin.IRouter) {
	router.POST("/sources/auto", h.HandleAddSourceAuto)
}
