package creditroll

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// OverlayRepository defines overlay lookup operations
type OverlayRepository interface {
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	GetByIDAndUserID(ctx context.Context, id string, userID string) (*models.Overlay, error)
}

// ConfigRepository defines credit roll config operations
type ConfigRepository interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.CreditRollConfig, error)
	Update(ctx context.Context, config *models.CreditRollConfig) error
}

// Handler handles credit roll HTTP requests
type Handler struct {
	configRepo  ConfigRepository
	overlayRepo OverlayRepository
	redis       *redis.Client
	logger      *zap.Logger
}

// NewHandler creates a new credit roll handler
func NewHandler(
	configRepo ConfigRepository,
	overlayRepo OverlayRepository,
	redis *redis.Client,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		configRepo:  configRepo,
		overlayRepo: overlayRepo,
		redis:       redis,
		logger:      logger,
	}
}

// HandleGetConfig returns credit roll config for an overlay (authenticated)
func (h *Handler) HandleGetConfig(c *gin.Context) {
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
	if _, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get credit roll config
	config, err := h.configRepo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "credit roll config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleUpdateConfig updates credit roll config (authenticated)
func (h *Handler) HandleUpdateConfig(c *gin.Context) {
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
	if _, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get current config
	config, err := h.configRepo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "credit roll config not found"})
		return
	}

	// Bind request body
	var req models.CreditRollConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update config (keep ID and overlay_id from existing)
	req.ID = config.ID
	req.OverlayID = config.OverlayID

	// Update in database
	if err := h.configRepo.Update(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update credit roll config"})
		return
	}

	c.JSON(http.StatusOK, &req)
}

// HandleGetPublicConfig returns credit roll config without authentication
func (h *Handler) HandleGetPublicConfig(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	// Verify overlay exists (no ownership check)
	if _, err := h.overlayRepo.GetByID(c.Request.Context(), overlayID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get credit roll config
	config, err := h.configRepo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "credit roll config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleGetCreditRoll returns aggregated credit roll data (public endpoint)
func (h *Handler) HandleGetCreditRoll(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	ctx := c.Request.Context()

	// Verify overlay exists
	_, err := h.overlayRepo.GetByID(ctx, overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	// Get credit roll config
	config, err := h.configRepo.GetByOverlayID(ctx, overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// Get active session
	session, err := h.getActiveSession(ctx, overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active session", "details": err.Error()})
		return
	}

	// Aggregate events from Redis leaderboards
	leaderboards, err := h.aggregateLeaderboards(ctx, session.SessionID, config)
	if err != nil {
		h.logger.Error("Failed to aggregate leaderboards",
			zap.String("session_id", session.SessionID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to aggregate events"})
		return
	}

	// Build response
	response := models.CreditRollResponse{
		OverlayID:              overlayID,
		SessionID:              session.SessionID,
		SessionStartedAt:       session.StartedAt,
		SessionDurationSeconds: int(time.Since(session.StartedAt).Seconds()),
		Leaderboards:           *leaderboards,
		Clips:                  []models.Clip{}, // Will be populated if clips enabled
		ClipsIsFallback:        false,
	}

	// Increment credit roll display count (fire-and-forget)
	go h.incrementCreditRollDisplay(context.Background(), overlayID, session.SessionID)

	c.JSON(http.StatusOK, response)
}

// getActiveSession retrieves active session from Redis
func (h *Handler) getActiveSession(ctx context.Context, overlayID string) (*models.SessionInfo, error) {
	key := "session:active:" + overlayID

	result, err := h.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no active session")
	}

	session := &models.SessionInfo{
		SessionID: result["session_id"],
		State:     result["state"],
	}

	if startedAtStr, ok := result["started_at"]; ok {
		startedAt, _ := time.Parse(time.RFC3339, startedAtStr)
		session.StartedAt = startedAt
	}

	if eventCountStr, ok := result["event_count"]; ok {
		fmt.Sscanf(eventCountStr, "%d", &session.EventCount)
	}

	if lastEventAtStr, ok := result["last_event_at"]; ok && lastEventAtStr != "" {
		lastEventAt, _ := time.Parse(time.RFC3339, lastEventAtStr)
		session.LastEventAt = lastEventAt
	}

	return session, nil
}

// aggregateLeaderboards retrieves and formats leaderboard data from Redis
func (h *Handler) aggregateLeaderboards(ctx context.Context, sessionID string, config *models.CreditRollConfig) (*models.Leaderboards, error) {
	leaderboards := &models.Leaderboards{}

	// Categories to aggregate
	categories := map[string]*[]models.LeaderboardEntry{
		"subs":        &leaderboards.Subs,
		"bits":        &leaderboards.Bits,
		"raids":       &leaderboards.Raids,
		"super_chats": &leaderboards.SuperChats,
		"follows":     &leaderboards.Follows,
		"gifts":       &leaderboards.Gifts,
	}

	for category, dest := range categories {
		entries, err := h.getLeaderboardEntries(ctx, sessionID, category, config.LeaderboardTopN)
		if err != nil {
			h.logger.Warn("Failed to get leaderboard entries",
				zap.String("category", category),
				zap.Error(err),
			)
			continue
		}
		*dest = entries
	}

	return leaderboards, nil
}

// getLeaderboardEntries retrieves entries for a specific category
func (h *Handler) getLeaderboardEntries(ctx context.Context, sessionID string, category string, topN int) ([]models.LeaderboardEntry, error) {
	key := fmt.Sprintf("session:leaderboard:%s:%s", sessionID, category)

	// Get top N entries from sorted set (highest score first)
	results, err := h.redis.ZRevRangeWithScores(ctx, key, 0, int64(topN-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	entries := make([]models.LeaderboardEntry, 0, len(results))

	for rank, result := range results {
		// Parse member JSON
		var member map[string]interface{}
		if err := json.Unmarshal([]byte(result.Member.(string)), &member); err != nil {
			h.logger.Warn("Failed to parse leaderboard member",
				zap.String("category", category),
				zap.Error(err),
			)
			continue
		}

		entry := models.LeaderboardEntry{
			Rank:        rank + 1,
			UserID:      getString(member, "user_id"),
			DisplayName: getString(member, "display_name"),
			AvatarURL:   getString(member, "avatar_url"),
			Platform:    getString(member, "platform"),
			TotalValue:  result.Score,
			Metadata:    make(map[string]interface{}),
		}

		// Add category-specific metadata
		if tier, ok := member["tier"].(string); ok {
			entry.Metadata["tier"] = tier
		}
		if months, ok := member["months"].(float64); ok {
			entry.Metadata["months"] = int(months)
		}
		if viewerCount, ok := member["viewer_count"].(float64); ok {
			entry.Metadata["viewer_count"] = int(viewerCount)
		}
		if currency, ok := member["currency"].(string); ok {
			entry.Metadata["currency"] = currency
		}
		if displayText, ok := member["display_text"].(string); ok {
			entry.Metadata["display_text"] = displayText
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// incrementCreditRollDisplay increments the display counter
func (h *Handler) incrementCreditRollDisplay(ctx context.Context, overlayID string, sessionID string) {
	// This is fire-and-forget, we don't care if it fails
	// (it's just for analytics)
	_ = h.redis.HIncrBy(ctx, "session:active:"+overlayID, "credit_roll_displayed_count", 1)
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
