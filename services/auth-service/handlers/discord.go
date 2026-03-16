package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// DiscordOAuthProvider is the interface DiscordHandler depends on (allows mocking in tests).
type DiscordOAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	CheckBotPermissions(ctx context.Context, guildID string) ([]string, error)
}

// DiscordGuildRepo is the interface for guild persistence (allows mocking in tests).
type DiscordGuildRepo interface {
	UpsertGuild(ctx context.Context, guild *models.DiscordGuild) error
	DeleteGuild(ctx context.Context, userID, guildID string) error
	ListGuildsByUser(ctx context.Context, userID string) ([]*models.DiscordGuild, error)
	GetGuild(ctx context.Context, userID, guildID string) (*models.DiscordGuild, error)
	DeleteDiscordSourcesByGuildID(ctx context.Context, guildID string) error
}

// stateStorer abstracts Redis state key operations for testability.
type stateStorer interface {
	Get(ctx context.Context, state string) (string, error)
	Set(ctx context.Context, state, userID string, ttl time.Duration) error
	Del(ctx context.Context, state string) error
}

// redisStateStore implements stateStorer using Redis.
type redisStateStore struct {
	client *redis.Client
}

func (r *redisStateStore) Get(ctx context.Context, state string) (string, error) {
	return r.client.Get(ctx, "discord:oauth:state:"+state).Result()
}

func (r *redisStateStore) Set(ctx context.Context, state, userID string, ttl time.Duration) error {
	return r.client.Set(ctx, "discord:oauth:state:"+state, userID, ttl).Err()
}

func (r *redisStateStore) Del(ctx context.Context, state string) error {
	return r.client.Del(ctx, "discord:oauth:state:"+state).Err()
}

// DiscordHandler handles Discord bot authorization and guild management HTTP endpoints.
type DiscordHandler struct {
	oauth          DiscordOAuthProvider
	repo           DiscordGuildRepo
	redis          *redis.Client
	stateStore     stateStorer
	botToken       string
	frontendURL    string
	log            *zap.Logger
	httpClient     *http.Client
	discordAPIBase string // overridable for tests
}

// NewDiscordHandler creates a new DiscordHandler with production dependencies.
func NewDiscordHandler(
	oauthProvider DiscordOAuthProvider,
	repo DiscordGuildRepo,
	redisClient *redis.Client,
	botToken, frontendURL string,
	log *zap.Logger,
) *DiscordHandler {
	return &DiscordHandler{
		oauth:          oauthProvider,
		repo:           repo,
		redis:          redisClient,
		stateStore:     &redisStateStore{client: redisClient},
		botToken:       botToken,
		frontendURL:    frontendURL,
		log:            log,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		discordAPIBase: "https://discord.com/api/v10",
	}
}

// newTestDiscordHandlerNoRedis creates a DiscordHandler suitable for unit tests.
// The Redis client field is nil; stateStore defaults to a no-op in-memory store.
// Tests that need custom state behaviour should set handler.stateStore directly after creation.
func newTestDiscordHandlerNoRedis(
	oauthProvider DiscordOAuthProvider,
	repo DiscordGuildRepo,
	frontendURL string,
) *DiscordHandler {
	log, _ := zap.NewDevelopment()
	return &DiscordHandler{
		oauth:          oauthProvider,
		repo:           repo,
		redis:          nil,
		stateStore:     newMemStateStore(),
		botToken:       "test-bot-token",
		frontendURL:    frontendURL,
		log:            log,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		discordAPIBase: "https://discord.com/api/v10",
	}
}

// memStateStore is an in-memory stateStorer used in unit tests.
type memStateStore struct {
	states map[string]string
}

func newMemStateStore() *memStateStore {
	return &memStateStore{states: make(map[string]string)}
}

func (m *memStateStore) Get(_ context.Context, state string) (string, error) {
	v, ok := m.states[state]
	if !ok {
		return "", fmt.Errorf("state not found: %s", state)
	}
	return v, nil
}

func (m *memStateStore) Set(_ context.Context, state, userID string, _ time.Duration) error {
	m.states[state] = userID
	return nil
}

func (m *memStateStore) Del(_ context.Context, state string) error {
	delete(m.states, state)
	return nil
}

// HandleConnect generates a CSRF state token, stores it in Redis (state -> userID), and
// returns the Discord bot invite URL for the authenticated user.
// Route: GET /discord/connect (JWT required)
func (h *DiscordHandler) HandleConnect(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	state, err := generateRandomString(32)
	if err != nil {
		h.log.Error("discord: failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if err := h.stateStore.Set(c.Request.Context(), state, userID, 10*time.Minute); err != nil {
		h.log.Error("discord: failed to store OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	inviteURL := h.oauth.GetAuthURL(state)
	c.JSON(http.StatusOK, gin.H{"bot_invite_url": inviteURL})
}

// HandleCallback processes the Discord OAuth callback.
// It validates the CSRF state from Redis, exchanges the code, checks bot permissions,
// and stores the guild in the database. If the bot lacks required permissions, it returns
// 403 Forbidden with a human-readable error listing the missing permissions.
// Route: GET /discord/callback (PUBLIC — no JWT)
func (h *DiscordHandler) HandleCallback(c *gin.Context) {
	ctx := c.Request.Context()
	state := c.Query("state")
	code := c.Query("code")
	guildID := c.Query("guild_id")
	guildName := c.Query("guild_name")

	if state == "" || code == "" || guildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	// Validate CSRF state — retrieve and delete atomically
	userID, err := h.stateStore.Get(ctx, state)
	if err != nil {
		h.log.Warn("discord: invalid or expired OAuth state", zap.String("state", state), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
		return
	}
	// Delete state regardless of subsequent outcome
	if delErr := h.stateStore.Del(ctx, state); delErr != nil {
		h.log.Warn("discord: failed to delete OAuth state", zap.Error(delErr))
	}

	// Exchange code for token (stored for audit only — not used for subsequent calls)
	_, err = h.oauth.ExchangeCode(ctx, code)
	if err != nil {
		h.log.Error("discord: code exchange failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange authorization code"})
		return
	}

	// Check bot permissions — never trust the `permissions` query param from the callback
	missing, err := h.oauth.CheckBotPermissions(ctx, guildID)
	if err != nil {
		h.log.Error("discord: permission check failed", zap.String("guild_id", guildID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify bot permissions"})
		return
	}
	if len(missing) > 0 {
		h.log.Warn("discord: bot missing required permissions",
			zap.String("guild_id", guildID),
			zap.Strings("missing", missing),
		)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Bot is missing: " + strings.Join(missing, ", "),
		})
		return
	}

	// Store the guild
	guild := &models.DiscordGuild{
		UserID:    userID,
		GuildID:   guildID,
		GuildName: guildName,
	}
	if err := h.repo.UpsertGuild(ctx, guild); err != nil {
		h.log.Error("discord: failed to upsert guild", zap.String("guild_id", guildID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save guild"})
		return
	}

	h.log.Info("discord: guild connected",
		zap.String("user_id", userID),
		zap.String("guild_id", guildID),
		zap.String("guild_name", guildName),
	)

	redirectURL := strings.TrimSuffix(h.frontendURL, "/") + "/settings?discord=connected"
	c.Redirect(http.StatusFound, redirectURL)
}

// HandleGetGuilds returns a JSON array of the authenticated user's connected Discord guilds.
// Route: GET /guilds (JWT required)
func (h *DiscordHandler) HandleGetGuilds(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	guilds, err := h.repo.ListGuildsByUser(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("discord: failed to list guilds", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve guilds"})
		return
	}

	// Return empty array rather than null when no guilds connected
	if guilds == nil {
		guilds = []*models.DiscordGuild{}
	}
	c.JSON(http.StatusOK, guilds)
}

// discordChannel is a single channel object from the Discord channels API.
type discordChannel struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     int     `json:"type"`
	Position int     `json:"position"`
	ParentID *string `json:"parent_id"`
}

// channelCategory is the response category object grouping text channels.
type channelCategory struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Channels []channelSummary    `json:"channels"`
}

// channelSummary is the minimal channel data returned to the frontend.
type channelSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// HandleGetGuildChannels fetches text channels for a guild and returns them grouped by category.
// Only type=0 (GUILD_TEXT) channels are included — voice and other types are excluded.
// Route: GET /guilds/:guild_id/channels (JWT required)
func (h *DiscordHandler) HandleGetGuildChannels(c *gin.Context) {
	ctx := c.Request.Context()
	guildID := c.Param("guild_id")

	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	// Verify guild belongs to the authenticated user
	if _, err := h.repo.GetGuild(ctx, userID, guildID); err != nil {
		h.log.Warn("discord: guild not found for user",
			zap.String("user_id", userID),
			zap.String("guild_id", guildID),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Guild not found"})
		return
	}

	// Fetch channels from Discord API
	url := fmt.Sprintf("%s/guilds/%s/channels", h.discordAPIBase, guildID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		h.log.Error("discord: failed to create channels request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	req.Header.Set("Authorization", "Bot "+h.botToken)
	req.Header.Set("User-Agent", "AllChat (https://allch.at, 1.0)")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.log.Error("discord: channels request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch channels from Discord"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		h.log.Error("discord: channels API returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Discord channels API error"})
		return
	}

	var rawChannels []discordChannel
	if err := json.Unmarshal(body, &rawChannels); err != nil {
		h.log.Error("discord: failed to decode channels response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse channels response"})
		return
	}

	// Build category map: id -> name
	categoryNames := make(map[string]string)
	for _, ch := range rawChannels {
		if ch.Type == 4 { // GUILD_CATEGORY
			categoryNames[ch.ID] = ch.Name
		}
	}

	// Collect text channels (type=0) grouped by parent_id
	type textChannelWithParent struct {
		channel  channelSummary
		parentID string // empty string means uncategorized
	}
	var textChannels []textChannelWithParent
	for _, ch := range rawChannels {
		if ch.Type != 0 { // Only GUILD_TEXT
			continue
		}
		parentID := ""
		if ch.ParentID != nil {
			parentID = *ch.ParentID
		}
		textChannels = append(textChannels, textChannelWithParent{
			channel:  channelSummary{ID: ch.ID, Name: ch.Name, Position: ch.Position},
			parentID: parentID,
		})
	}

	// Sort text channels by position within each category
	sort.Slice(textChannels, func(i, j int) bool {
		return textChannels[i].channel.Position < textChannels[j].channel.Position
	})

	// Group channels by parent category. Preserve category order from Discord.
	// We'll use an ordered list of category IDs seen.
	categoryOrder := []string{}
	seen := make(map[string]bool)
	for _, ch := range rawChannels {
		if ch.Type == 4 {
			if !seen[ch.ID] {
				categoryOrder = append(categoryOrder, ch.ID)
				seen[ch.ID] = true
			}
		}
	}

	// Build category map: parentID -> channels
	categoryChannels := make(map[string][]channelSummary)
	for _, tc := range textChannels {
		categoryChannels[tc.parentID] = append(categoryChannels[tc.parentID], tc.channel)
	}

	// Build response: known categories in order, then uncategorized
	var categories []channelCategory
	for _, catID := range categoryOrder {
		channels := categoryChannels[catID]
		if len(channels) == 0 {
			continue
		}
		categories = append(categories, channelCategory{
			ID:       catID,
			Name:     categoryNames[catID],
			Channels: channels,
		})
	}

	// Add "Uncategorized" for channels with no parent_id
	if orphans, ok := categoryChannels[""]; ok && len(orphans) > 0 {
		categories = append(categories, channelCategory{
			ID:       "",
			Name:     "Uncategorized",
			Channels: orphans,
		})
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// HandleDisconnect removes a connected Discord guild (best-effort Leave Guild + always delete local records).
// The Discord REST call failure is logged but does NOT prevent local cleanup.
// Route: DELETE /guilds/:guild_id (JWT required)
func (h *DiscordHandler) HandleDisconnect(c *gin.Context) {
	ctx := c.Request.Context()
	guildID := c.Param("guild_id")

	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	// Verify guild belongs to the authenticated user
	if _, err := h.repo.GetGuild(ctx, userID, guildID); err != nil {
		h.log.Warn("discord: guild not found for disconnect",
			zap.String("user_id", userID),
			zap.String("guild_id", guildID),
			zap.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Guild not found"})
		return
	}

	// Best-effort: call Discord Leave Guild API
	leaveURL := fmt.Sprintf("%s/users/@me/guilds/%s", h.discordAPIBase, guildID)
	leaveReq, err := http.NewRequestWithContext(ctx, "DELETE", leaveURL, nil)
	if err != nil {
		h.log.Error("discord: failed to create leave request", zap.Error(err))
		// Continue — still clean up local records
	} else {
		leaveReq.Header.Set("Authorization", "Bot "+h.botToken)
		leaveReq.Header.Set("User-Agent", "AllChat (https://allch.at, 1.0)")

		leaveResp, leaveErr := h.httpClient.Do(leaveReq)
		if leaveErr != nil {
			h.log.Warn("discord: leave guild request failed (proceeding with local cleanup)",
				zap.String("guild_id", guildID),
				zap.Error(leaveErr),
			)
		} else {
			defer leaveResp.Body.Close()
			if leaveResp.StatusCode != http.StatusNoContent && leaveResp.StatusCode != http.StatusNotFound {
				h.log.Warn("discord: leave guild API returned unexpected status (proceeding with local cleanup)",
					zap.String("guild_id", guildID),
					zap.Int("status", leaveResp.StatusCode),
				)
			}
		}
	}

	// Always delete local guild record
	if err := h.repo.DeleteGuild(ctx, userID, guildID); err != nil {
		h.log.Error("discord: failed to delete guild record",
			zap.String("user_id", userID),
			zap.String("guild_id", guildID),
			zap.Error(err),
		)
		// Still try to clean up sources
	}

	// Always delete associated chat sources
	if err := h.repo.DeleteDiscordSourcesByGuildID(ctx, guildID); err != nil {
		h.log.Error("discord: failed to delete discord sources",
			zap.String("guild_id", guildID),
			zap.Error(err),
		)
	}

	h.log.Info("discord: guild disconnected",
		zap.String("user_id", userID),
		zap.String("guild_id", guildID),
	)

	c.Status(http.StatusNoContent)
}
