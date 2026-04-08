package creditroll

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// parseSessionTime safely parses RFC3339 time with validation
func parseSessionTime(timeStr string, fieldName string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("%s is empty", fieldName)
	}

	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse %s: %w", fieldName, err)
	}

	if parsed.IsZero() {
		return time.Time{}, fmt.Errorf("%s is zero value", fieldName)
	}

	return parsed, nil
}

// validateStartedAt checks if time is valid for duration calculation
func validateStartedAt(t time.Time) error {
	if t.IsZero() {
		return fmt.Errorf("started_at is zero value")
	}

	// Sanity check: reject times before 2020 or in future
	minTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Now().UTC().Add(1 * time.Hour)

	if t.Before(minTime) {
		return fmt.Errorf("started_at before 2020: %s", t.Format(time.RFC3339))
	}

	if t.After(maxTime) {
		return fmt.Errorf("started_at in future: %s", t.Format(time.RFC3339))
	}

	return nil
}

// calculateSessionDuration safely calculates duration with bounds checking
func calculateSessionDuration(startedAt time.Time) int {
	if startedAt.IsZero() {
		return 0
	}

	duration := time.Since(startedAt)

	// Sanity checks
	if duration < 0 {
		return 0
	}

	// Cap at 30 days (reasonable max stream duration)
	maxDuration := 30 * 24 * time.Hour
	if duration > maxDuration {
		return int(maxDuration.Seconds())
	}

	return int(duration.Seconds())
}

// OverlayRepository defines overlay lookup operations
type OverlayRepository interface {
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	GetByIDAndUserID(ctx context.Context, id string, userID string) (*models.Overlay, error)
}

// ConfigRepository defines credit roll config operations
type ConfigRepository interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.CreditRollConfig, error)
	GetOrCreate(ctx context.Context, overlayID string) (*models.CreditRollConfig, error)
	Update(ctx context.Context, config *models.CreditRollConfig) error
	GetMostRecentCompletedSession(ctx context.Context, overlayID string) (*models.SessionInfo, error)
}

// SourceRepository defines chat source lookup operations
type SourceRepository interface {
	ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error)
}

// Handler handles credit roll HTTP requests
type Handler struct {
	configRepo  ConfigRepository
	overlayRepo OverlayRepository
	sourceRepo  SourceRepository
	redis       *redis.Client
	logger      *zap.Logger
	clipsClient *clients.TwitchClipsClient
}

// NewHandler creates a new credit roll handler
func NewHandler(
	configRepo ConfigRepository,
	overlayRepo OverlayRepository,
	sourceRepo SourceRepository,
	redis *redis.Client,
	logger *zap.Logger,
	clipsClient *clients.TwitchClipsClient,
) *Handler {
	return &Handler{
		configRepo:  configRepo,
		overlayRepo: overlayRepo,
		sourceRepo:  sourceRepo,
		redis:       redis,
		logger:      logger,
		clipsClient: clipsClient,
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

	// Get or create credit roll config (auto-creates default if missing)
	config, err := h.configRepo.GetOrCreate(c.Request.Context(), overlayID)
	if err != nil {
		h.logger.Error("Failed to get or create credit roll config",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get credit roll config"})
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

	// Get or create credit roll config (auto-creates default if missing)
	config, err := h.configRepo.GetOrCreate(ctx, overlayID)
	if err != nil {
		h.logger.Error("Failed to get or create credit roll config",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get credit roll config"})
		return
	}

	// Get active session (with auto-repair if corrupted)
	session, err := h.getOrRepairSession(ctx, overlayID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session", "details": err.Error()})
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

	// Fetch clips if enabled
	clips, clipsIsFallback, err := h.fetchClips(ctx, overlayID, session, config)
	if err != nil {
		h.logger.Error("Failed to fetch clips",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		clips = []models.Clip{}
		clipsIsFallback = false
	}

	// Build response
	response := models.CreditRollResponse{
		OverlayID:              overlayID,
		SessionID:              session.SessionID,
		SessionStartedAt:       session.StartedAt,
		SessionDurationSeconds: calculateSessionDuration(session.StartedAt),
		Leaderboards:           *leaderboards,
		Clips:                  clips,
		ClipsIsFallback:        clipsIsFallback,
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

	// A hash with no session_id is a zombie left by a background analytics write
	// (incrementCreditRollDisplay) that fired after EndSession deleted the key.
	// Treat it the same as an absent key so the DB-fallback path is taken, not the
	// corruption-repair path (which would reset startedAt to time.Now() → duration 0).
	sessionID, hasSessionID := result["session_id"]
	if !hasSessionID || sessionID == "" {
		return nil, fmt.Errorf("no active session")
	}

	session := &models.SessionInfo{
		SessionID: sessionID,
		State:     result["state"],
	}

	// started_at is required
	startedAtStr, ok := result["started_at"]
	if !ok {
		return nil, fmt.Errorf("started_at missing from session")
	}

	startedAt, err := parseSessionTime(startedAtStr, "started_at")
	if err != nil {
		h.logger.Error("Invalid started_at in session",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("invalid started_at: %w", err)
	}

	if err := validateStartedAt(startedAt); err != nil {
		return nil, fmt.Errorf("started_at validation failed: %w", err)
	}

	session.StartedAt = startedAt

	if eventCountStr, ok := result["event_count"]; ok {
		fmt.Sscanf(eventCountStr, "%d", &session.EventCount)
	}

	if lastEventAtStr, ok := result["last_event_at"]; ok && lastEventAtStr != "" {
		lastEventAt, _ := time.Parse(time.RFC3339, lastEventAtStr)
		session.LastEventAt = lastEventAt
	}

	return session, nil
}

// getOrRepairSession attempts to get session, repairs if corrupted
func (h *Handler) getOrRepairSession(ctx context.Context, overlayID string) (*models.SessionInfo, error) {
	session, err := h.getActiveSession(ctx, overlayID)

	if err == nil {
		// Session valid
		return session, nil
	}

	// Check if error is due to invalid started_at
	if strings.Contains(err.Error(), "started_at") || strings.Contains(err.Error(), "parse") {
		h.logger.Warn("Detected corrupted session, attempting repair",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)

		// Delete corrupted session
		key := "session:active:" + overlayID
		if delErr := h.redis.Del(ctx, key).Err(); delErr != nil {
			h.logger.Error("Failed to delete corrupted session",
				zap.String("overlay_id", overlayID),
				zap.Error(delErr),
			)
		}

		// Create new session
		sessionID := uuid.New().String()
		startedAt := time.Now().UTC()

		pipe := h.redis.Pipeline()
		pipe.HSet(ctx, key, "session_id", sessionID)
		pipe.HSet(ctx, key, "started_at", startedAt.Format(time.RFC3339))
		pipe.HSet(ctx, key, "state", "ACTIVE")
		pipe.HSet(ctx, key, "event_count", 0)
		pipe.Expire(ctx, key, 24*time.Hour)

		if _, execErr := pipe.Exec(ctx); execErr != nil {
			return nil, fmt.Errorf("failed to create new session: %w", execErr)
		}

		h.logger.Info("Repaired corrupted session",
			zap.String("overlay_id", overlayID),
			zap.String("new_session_id", sessionID),
		)

		// Return new session
		return &models.SessionInfo{
			SessionID:  sessionID,
			StartedAt:  startedAt,
			State:      "ACTIVE",
			EventCount: 0,
		}, nil
	}

	// Check if error is "no active session" - try database fallback
	if strings.Contains(err.Error(), "no active session") {
		h.logger.Info("No active session in Redis, checking database for completed session",
			zap.String("overlay_id", overlayID),
		)

		// Try to get most recent completed session from database
		dbSession, dbErr := h.configRepo.GetMostRecentCompletedSession(ctx, overlayID)
		if dbErr == nil && dbSession != nil {
			h.logger.Info("Found completed session in database",
				zap.String("overlay_id", overlayID),
				zap.String("session_id", dbSession.SessionID),
				zap.Time("started_at", dbSession.StartedAt),
			)
			// Return completed session from database
			// Leaderboard data should still be in Redis (48h TTL)
			return dbSession, nil
		}

		h.logger.Warn("No completed session found in database",
			zap.String("overlay_id", overlayID),
			zap.Error(dbErr),
		)
	}

	// Error not related to corruption or no session available
	return nil, err
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
		"points":      &leaderboards.Points,
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
	metaKey := fmt.Sprintf("session:leaderboard:meta:%s:%s", sessionID, category)

	// Get top N entries from sorted set (highest score first)
	results, err := h.redis.ZRevRangeWithScores(ctx, key, 0, int64(topN-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	// Batch-fetch metadata for all members
	memberKeys := make([]string, 0, len(results))
	for _, result := range results {
		memberKeys = append(memberKeys, result.Member.(string))
	}

	metaValues := make([]interface{}, len(memberKeys))
	if len(memberKeys) > 0 {
		metaValues, _ = h.redis.HMGet(ctx, metaKey, memberKeys...).Result()
	}

	entries := make([]models.LeaderboardEntry, 0, len(results))

	for rank, result := range results {
		memberStr := result.Member.(string)

		// Try metadata hash first (new format: stable key with companion hash)
		var member map[string]interface{}
		if rank < len(metaValues) && metaValues[rank] != nil {
			if metaStr, ok := metaValues[rank].(string); ok {
				if err := json.Unmarshal([]byte(metaStr), &member); err != nil {
					h.logger.Warn("Failed to parse leaderboard metadata",
						zap.String("category", category),
						zap.String("member_key", memberStr),
						zap.Error(err),
					)
					continue
				}
			}
		}

		// Fallback: try parsing member itself as JSON (legacy format)
		if member == nil {
			if err := json.Unmarshal([]byte(memberStr), &member); err != nil {
				// Member is a stable key (platform:user_id) with no metadata — skip
				h.logger.Warn("Leaderboard entry missing metadata",
					zap.String("category", category),
					zap.String("member_key", memberStr),
				)
				continue
			}
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

// incrementCreditRollDisplay increments the display counter only when the session
// key already exists.  Using a plain HIncrBy on an absent key would create a zombie
// hash containing only "credit_roll_displayed_count", which caused getActiveSession
// to trigger the corruption-repair path and reset startedAt to time.Now() (→ duration 0).
func (h *Handler) incrementCreditRollDisplay(ctx context.Context, overlayID string, sessionID string) {
	key := "session:active:" + overlayID
	// Lua script: increment only if the key exists (KEYS[1] present in Redis).
	// This is atomic and avoids creating a zombie hash when the session has already
	// been deleted by EndSession.
	script := redis.NewScript(`
		if redis.call("EXISTS", KEYS[1]) == 1 then
			return redis.call("HINCRBY", KEYS[1], "credit_roll_displayed_count", 1)
		end
		return 0
	`)
	_ = script.Run(ctx, h.redis, []string{key}).Err()
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// fetchClips retrieves clips for the credit roll
func (h *Handler) fetchClips(ctx context.Context, overlayID string, session *models.SessionInfo, config *models.CreditRollConfig) ([]models.Clip, bool, error) {
	if !config.ClipsEnabled {
		return []models.Clip{}, false, nil
	}

	// Get broadcaster Twitch ID
	broadcasterID, err := h.getBroadcasterTwitchID(ctx, overlayID)
	if err != nil {
		return nil, false, fmt.Errorf("no twitch broadcaster found: %w", err)
	}

	// Try fetching clips from current session timeframe
	endTime := time.Now().UTC()
	clips, err := h.clipsClient.GetClips(ctx, broadcasterID, session.StartedAt, endTime, config.ClipsMaxCount)
	if err != nil {
		h.logger.Warn("Failed to fetch clips for session", zap.Error(err))
		clips = []clients.ClipData{}
	}

	isFallback := false

	// If no clips from session, try fallback period
	if len(clips) == 0 && config.ClipsFallbackDays > 0 {
		fallbackStart := time.Now().UTC().AddDate(0, 0, -config.ClipsFallbackDays)
		clips, err = h.clipsClient.GetClips(ctx, broadcasterID, fallbackStart, endTime, config.ClipsMaxCount)
		if err != nil {
			h.logger.Warn("Failed to fetch fallback clips", zap.Error(err))
			return []models.Clip{}, false, nil
		}
		isFallback = len(clips) > 0
	}

	// Convert to models.Clip
	result := make([]models.Clip, len(clips))
	for i, clip := range clips {
		createdAt, _ := time.Parse(time.RFC3339, clip.CreatedAt)
		result[i] = models.Clip{
			ID:           clip.ID,
			URL:          clip.URL,
			EmbedURL:     clip.EmbedURL,
			Title:        clip.Title,
			ViewCount:    clip.ViewCount,
			CreatedAt:    createdAt,
			ThumbnailURL: clip.ThumbnailURL,
			Duration:     clip.Duration,
		}
	}

	return result, isFallback, nil
}

// getBroadcasterTwitchID retrieves the Twitch broadcaster ID for an overlay
func (h *Handler) getBroadcasterTwitchID(ctx context.Context, overlayID string) (string, error) {
	sources, err := h.sourceRepo.ListByOverlayID(ctx, overlayID)
	if err != nil {
		return "", fmt.Errorf("failed to list sources: %w", err)
	}

	// Find first Twitch source
	for _, source := range sources {
		if source.Platform == "twitch" {
			// Convert username to broadcaster ID
			broadcasterID, err := h.clipsClient.GetUserID(ctx, source.ChannelID)
			if err != nil {
				return "", fmt.Errorf("failed to get broadcaster ID for %s: %w", source.ChannelID, err)
			}
			return broadcasterID, nil
		}
	}

	return "", fmt.Errorf("no twitch source found for overlay")
}
