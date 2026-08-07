// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SourceRepository defines the interface for source persistence
type SourceRepository interface {
	Create(ctx context.Context, source *models.ChatSource) error
	CreateOrUpdateAuto(ctx context.Context, source *models.ChatSource) error
	ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error)
	ListByOverlayIDForUser(ctx context.Context, overlayID, requestingUserID string) ([]*models.ChatSource, error)
	GetByID(ctx context.Context, id string) (*models.ChatSource, error)
	Delete(ctx context.Context, id string) error
	UpdateConfig(ctx context.Context, id string, config map[string]interface{}) error
}

// discordChannelGuildResolver reports which guild a Discord channel belongs to, as Discord
// itself sees it. A client-supplied guild id is never a substitute: the whole point is to stop
// a caller pairing a channel they do not control with a guild they do.
type discordChannelGuildResolver interface {
	GuildIDForChannel(ctx context.Context, channelID string) (string, error)
}

// discordGuildOwnership reports whether a user has connected a guild — i.e. invited the
// shared bot to it, which Discord only offers to members holding Manage Server.
type discordGuildOwnership interface {
	UserOwnsGuild(ctx context.Context, userID, guildID string) (bool, error)
}

// SourcesHandler handles HTTP requests for overlay chat sources
type SourcesHandler struct {
	sourceRepo  SourceRepository
	overlayRepo OverlayRepository
	db          *pgxpool.Pool
	redis       redis.Cmdable
	logger      *zap.Logger
	bm          *metrics.BusinessMetrics
	cipher      *encryption.MultiKeyEncryptor // Encrypts kick_oauth_tokens on write (D-16; nil = no encryption)

	// Discord source guard (ADR-0048). Both nil means Discord sources cannot be validated,
	// and adding one fails closed.
	discordChannels discordChannelGuildResolver
	discordGuilds   discordGuildOwnership
}

// SetDiscordGuard wires the Discord source guard. Injected separately from the constructor
// because it is optional configuration (DISCORD_BOT_TOKEN): when it is absent, Discord chat
// cannot work at all, and Discord sources must be refused rather than accepted unvalidated.
func (h *SourcesHandler) SetDiscordGuard(channels discordChannelGuildResolver, guilds discordGuildOwnership) {
	h.discordChannels = channels
	h.discordGuilds = guilds
}

// discordChannelEntry is the JSON value stored at discord:channels:{channel_id}.
type discordChannelEntry struct {
	OverlayID string `json:"overlay_id"`
	SourceID  string `json:"source_id"`
}

// NewSourcesHandler creates a new sources handler.
// redisClient is used to maintain the Discord channel registry keys.
// It accepts redis.Cmdable for testability; *redis.Client implements this interface.
// bm is the shared business metrics instance (may be nil — metrics are skipped if nil).
// cipher is used to encrypt kick_oauth_tokens on write (D-16); may be nil when
// TOKEN_ENCRYPTION_KEY_V1 is not configured (tokens stored as plaintext with encryption_version=0).
func NewSourcesHandler(sourceRepo SourceRepository, overlayRepo OverlayRepository, db *pgxpool.Pool, logger *zap.Logger, redisClient redis.Cmdable, bm *metrics.BusinessMetrics, cipher *encryption.MultiKeyEncryptor) *SourcesHandler {
	return &SourcesHandler{
		sourceRepo:  sourceRepo,
		overlayRepo: overlayRepo,
		db:          db,
		redis:       redisClient,
		logger:      logger,
		bm:          bm,
		cipher:      cipher,
	}
}

// discordConfigChannelKeys are the config fields that name a Discord channel All-Chat will
// act on. Each is attacker-controlled and each is exercised by the SHARED bot, so Discord
// authorizes the bot and never the caller:
//   - inbound_channel_id is the discord:channels:{id} registry key that routes a guild's chat
//     into an overlay (a read capability over that channel).
//   - relay_channel_id is an outbound webhook target that discord-listener provisions and
//     posts chat into (a write capability).
//
// channel_id itself is checked alongside these, since it is what the moderation write-path
// acts on.
var discordConfigChannelKeys = []string{"inbound_channel_id", "relay_channel_id"}

// authorizeDiscordChannels verifies that every Discord channel a request would put to work
// belongs to a guild the user has connected. It returns (0, "") when the request is allowed,
// or an HTTP status plus message when it must be refused.
//
// It fails closed on every uncertainty: an unconfigured guard, a Discord API error and a
// database error all refuse, because "cannot verify" must never read as "allowed". A guild id
// supplied in config is not trusted as the binding — Discord's own answer for the channel is
// authoritative — but a value that contradicts it is refused rather than stored, since a
// mislabelled source would misinform the owner about what they attached. (ADR-0048.)
//
// channelID is the source's primary channel, and is validated (and used for the guild_id
// claim check) only when non-empty. Reconfiguring an existing source passes "" so that a
// primary channel deleted on Discord's side cannot strand the owner: they must still be able
// to turn a relay off, and the stored channel is not what this request changes.
func (h *SourcesHandler) authorizeDiscordChannels(ctx context.Context, userID, channelID string, config map[string]interface{}) (int, string) {
	if h.discordChannels == nil || h.discordGuilds == nil {
		h.logger.Error("Discord source guard unconfigured — refusing Discord source",
			zap.String("user_id", userID))
		return http.StatusServiceUnavailable, "Discord source validation is unavailable, please try again later"
	}

	// channel_id first: its resolved guild is also what a supplied guild_id must match.
	var candidates []string
	if channelID != "" {
		candidates = append(candidates, channelID)
	}
	for _, key := range discordConfigChannelKeys {
		if v, ok := config[key].(string); ok && v != "" && v != channelID {
			candidates = append(candidates, v)
		}
	}

	claimedGuild, _ := config["guild_id"].(string)
	if channelID == "" {
		claimedGuild = "" // no primary channel to compare a claim against
	}
	seen := make(map[string]bool, len(candidates))

	for i, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true

		guildID, err := h.discordChannels.GuildIDForChannel(ctx, candidate)
		switch {
		case errors.Is(err, clients.ErrDiscordChannelNotFound), errors.Is(err, clients.ErrDiscordNoGuild):
			return http.StatusBadRequest, fmt.Sprintf("Discord channel '%s' does not exist or is not a server channel", candidate)
		case errors.Is(err, clients.ErrDiscordForbidden):
			return http.StatusBadRequest, fmt.Sprintf("All-Chat's bot cannot see Discord channel '%s' — re-invite the bot with access to it", candidate)
		case err != nil:
			h.logger.Error("Failed to resolve Discord channel guild",
				zap.String("channel_id", candidate), zap.Error(err))
			return http.StatusServiceUnavailable, "Could not verify the Discord channel right now, please try again"
		}

		// Only the primary channel_id's guild is compared against the claim; the other fields
		// are validated by ownership alone (they may legitimately sit in the same guild).
		if i == 0 && claimedGuild != "" && claimedGuild != guildID {
			return http.StatusBadRequest, "The Discord server does not match the selected channel"
		}

		owns, err := h.discordGuilds.UserOwnsGuild(ctx, userID, guildID)
		if err != nil {
			h.logger.Error("Failed to check Discord guild ownership",
				zap.String("user_id", userID), zap.String("guild_id", guildID), zap.Error(err))
			return http.StatusServiceUnavailable, "Could not verify the Discord server right now, please try again"
		}
		if !owns {
			h.logger.Warn("Refused Discord channel in a guild the user has not connected",
				zap.String("user_id", userID),
				zap.String("channel_id", candidate),
				zap.String("guild_id", guildID))
			return http.StatusForbidden, "You have not connected that Discord server to All-Chat"
		}
	}

	return 0, ""
}

// setDiscordChannelRegistry writes the channel registry Redis key and publishes an invalidation event.
// The key is set BEFORE the Pub/Sub publish so discord-listener always sees a consistent state.
func (h *SourcesHandler) setDiscordChannelRegistry(ctx context.Context, channelID, overlayID, sourceID string) {
	if h.redis == nil {
		return
	}
	entry := discordChannelEntry{OverlayID: overlayID, SourceID: sourceID}
	data, err := json.Marshal(entry)
	if err != nil {
		h.logger.Warn("Failed to marshal discord channel registry entry",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return
	}
	key := "discord:channels:" + channelID
	if err := h.redis.Set(ctx, key, string(data), 0).Err(); err != nil {
		h.logger.Warn("Failed to set discord channel registry key",
			zap.String("key", key),
			zap.Error(err),
		)
	}
	if err := h.redis.Publish(ctx, "discord:channel:invalidation", channelID).Err(); err != nil {
		h.logger.Warn("Failed to publish discord channel invalidation",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}
}

// delDiscordChannelRegistry removes the channel registry Redis key and publishes an invalidation event.
func (h *SourcesHandler) delDiscordChannelRegistry(ctx context.Context, channelID string) {
	if h.redis == nil {
		return
	}
	key := "discord:channels:" + channelID
	if err := h.redis.Del(ctx, key).Err(); err != nil {
		h.logger.Warn("Failed to delete discord channel registry key",
			zap.String("key", key),
			zap.Error(err),
		)
	}
	if err := h.redis.Publish(ctx, "discord:channel:invalidation", channelID).Err(); err != nil {
		h.logger.Warn("Failed to publish discord channel invalidation",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}
}

// copyYouTubeTokenForChannel copies the admin's YouTube OAuth token to a new channel
// This allows admins to add YouTube channels manually (by link) without OAuth flow
func (h *SourcesHandler) copyYouTubeTokenForChannel(ctx context.Context, adminUserID, newChannelID string) error {
	// Step 1: Find the best YouTube token for this admin.
	// Prefer non-expired tokens first, but fall back to any token because the
	// youtube-listener will use the refresh_token to get a fresh access_token on
	// first use. Excluding expired tokens here prevents copy when the access_token
	// has expired but the refresh_token is still valid — blocking detection permanently.
	var existingToken struct {
		AccessToken       string
		RefreshToken      string
		TokenType         string
		Expiry            string // Store as string to avoid timestamp parsing issues
		EncryptionVersion int
	}

	query := `
		SELECT access_token, refresh_token, token_type,
		       expiry::text, encryption_version
		FROM youtube_oauth_tokens
		WHERE user_id = $1
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
		return fmt.Errorf("admin has no YouTube OAuth token - please authorize YouTube first: %w", err)
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

// copyKickTokenForChannel copies the admin's Kick OAuth token to a new channel.
// If a cipher is configured, the access_token is encrypted before write and
// encryption_version=1 is stored; otherwise plaintext is written with encryption_version=0.
func (h *SourcesHandler) copyKickTokenForChannel(ctx context.Context, adminUserID, newChannelID string) error {
	// Step 1: Find the best Kick token for this admin.
	// Prefer non-expired tokens first, but fall back to any token because the
	// kick-listener will use the refresh_token to get a fresh access_token on
	// first use. Excluding expired tokens here prevents copy when the access_token
	// has expired but the refresh_token is still valid — blocking detection permanently.
	var existingToken struct {
		AccessToken       string
		RefreshToken      string
		TokenType         string
		Expiry            string // Store as string to avoid timestamp parsing issues
		EncryptionVersion int
	}

	query := `
		SELECT access_token, refresh_token, token_type, expiry::text, encryption_version
		FROM kick_oauth_tokens
		WHERE user_id = $1
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
		return fmt.Errorf("admin has no Kick OAuth token - please authorize Kick first: %w", err)
	}

	// Decrypt the source token if it was stored encrypted (so we always work with plaintext).
	plainAccessToken := existingToken.AccessToken
	if existingToken.EncryptionVersion >= 1 {
		if h.cipher == nil {
			return fmt.Errorf("source kick_oauth_token has encryption_version=%d but no cipher configured", existingToken.EncryptionVersion)
		}
		plainAccessToken, err = h.cipher.DecryptString(existingToken.AccessToken)
		if err != nil {
			return fmt.Errorf("failed to decrypt source Kick token: %w", err)
		}
	}

	// Encrypt for the destination row (D-16): use cipher when available.
	writeToken := plainAccessToken
	encryptionVersion := 0
	if h.cipher != nil {
		writeToken, err = h.cipher.EncryptString(plainAccessToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt Kick token for write: %w", err)
		}
		encryptionVersion = 1
	}

	// Step 2: Copy token to new channel_id (insert or update)
	insertQuery := `
		INSERT INTO kick_oauth_tokens (
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
		writeToken,
		existingToken.RefreshToken,
		existingToken.TokenType,
		existingToken.Expiry,
		encryptionVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to copy Kick token: %w", err)
	}

	h.logger.Info("Copied Kick OAuth token for new channel",
		zap.String("admin_user_id", adminUserID),
		zap.String("new_channel_id", newChannelID),
		zap.Int("encryption_version", encryptionVersion),
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

	// Get sources, computing IsOwnChannel for the authenticated user so the frontend can
	// surface the IRC→EventSub re-consent nudge for channels this user can re-authorize.
	sources, err := h.sourceRepo.ListByOverlayIDForUser(c.Request.Context(), overlayID, userID.(string))
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
		Platform      string                 `json:"platform" binding:"required"`
		ChannelID     string                 `json:"channel_id" binding:"required"`
		ChannelName   string                 `json:"channel_name"`
		ChannelHandle string                 `json:"channel_handle"`
		Config        map[string]interface{} `json:"config"`
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

	// Discord sources are acted on by the shared bot, which Discord authorizes instead of the
	// caller — so nothing upstream refuses a channel the caller has no claim to. Verify guild
	// ownership here, fail closed. (ADR-0048.)
	if req.Platform == "discord" {
		if status, msg := h.authorizeDiscordChannels(c.Request.Context(), userID.(string), channelID, req.Config); status != 0 {
			c.JSON(status, gin.H{"error": msg})
			return
		}
	}

	var channelHandle *string
	if req.ChannelHandle != "" {
		channelHandle = &req.ChannelHandle
	}

	sourceConfig := make(map[string]interface{})
	if req.Config != nil {
		sourceConfig = req.Config
	}

	source := &models.ChatSource{
		OverlayID:     overlayID,
		Platform:      req.Platform,
		ChannelID:     channelID,
		ChannelName:   channelName,
		ChannelHandle: channelHandle,
		AuthRequired:  req.Platform == "youtube", // YouTube requires OAuth
		Config:        sourceConfig,
		IsActive:      req.Platform == "shared_overlay", // shared_overlay is immediately active (share already accepted); other platforms activated by listeners
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

	// For Discord sources, register the channel in Redis so discord-listener
	// can route incoming messages to the correct overlay.
	// The registry key is written BEFORE the Pub/Sub invalidation event.
	if req.Platform == "discord" {
		// Extract inbound_channel_id from the source config (or fall back to channel_id).
		inboundChannelID := channelID
		if v, ok := source.Config["inbound_channel_id"].(string); ok && v != "" {
			inboundChannelID = v
		}
		h.setDiscordChannelRegistry(c.Request.Context(), inboundChannelID, overlayID, source.ID)
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

	if h.bm != nil {
		h.bm.RecordSourceOperation("create", req.Platform, "success")
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

	// For Discord sources, remove the channel registry key from Redis so
	// discord-listener stops routing messages to the now-deleted overlay.
	if source.Platform == "discord" {
		inboundChannelID := source.ChannelID
		if v, ok := source.Config["inbound_channel_id"].(string); ok && v != "" {
			inboundChannelID = v
		}
		h.delDiscordChannelRegistry(c.Request.Context(), inboundChannelID)
	}

	if h.bm != nil {
		h.bm.RecordSourceOperation("delete", source.Platform, "success")
	}

	c.Status(http.StatusNoContent)
}

// HandleUpdateSourceConfig handles PATCH /:id/sources/:source_id
// It verifies overlay ownership, then updates the config JSONB field.
func (h *SourcesHandler) HandleUpdateSourceConfig(c *gin.Context) {
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
		c.JSON(http.StatusForbidden, gin.H{"error": "overlay not found or access denied"})
		return
	}

	var req struct {
		Config map[string]interface{} `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Config == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config is required"})
		return
	}

	// Owning the overlay in the path is not enough: UpdateConfig is keyed on source_id alone,
	// so without this the caller could patch a source belonging to someone else's overlay.
	source, err := h.sourceRepo.GetByID(c.Request.Context(), sourceID)
	if err != nil || source == nil || source.OverlayID != overlayID {
		c.JSON(http.StatusNotFound, gin.H{"error": "source not found on this overlay"})
		return
	}

	// A Discord relay/inbound target can be introduced by PATCH just as easily as at create
	// time, so it needs the same ownership guard. Only the channels named in THIS request are
	// validated — the stored primary channel was checked when the source was created. (ADR-0048.)
	if source.Platform == "discord" {
		if status, msg := h.authorizeDiscordChannels(c.Request.Context(), userID.(string), "", req.Config); status != 0 {
			c.JSON(status, gin.H{"error": msg})
			return
		}
	}

	// Validate stream_select if present (YouTube stream selection strategy)
	if strategy, ok := req.Config["stream_select"].(string); ok && strategy != "" {
		validStrategies := map[string]bool{
			"first_found":     true,
			"most_viewers":    true,
			"fewest_viewers":  true,
			"title_match":     true,
			"title_match_all": true,
			"all":             true,
		}
		if !validStrategies[strategy] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid stream_select strategy: %s", strategy)})
			return
		}
		if strategy == "title_match" || strategy == "title_match_all" {
			match, _ := req.Config["stream_match"].(string)
			if strings.TrimSpace(match) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "stream_match is required when stream_select uses title matching"})
				return
			}
		}
	}

	if err := h.sourceRepo.UpdateConfig(c.Request.Context(), sourceID, req.Config); err != nil {
		h.logger.Error("Failed to update source config",
			zap.String("source_id", sourceID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
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

	// This endpoint serves the OAuth callback (twitch/youtube/kick) and has no legitimate
	// Discord caller — Discord connects through auth-service's bot-invite flow, which writes
	// discord_guilds rather than overlay sources. A source row alone is enough to make a
	// channel moderatable, so refuse outright instead of running an add-source guard here.
	if req.Platform == "discord" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "discord sources cannot be added through this endpoint"})
		return
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

	// Idempotent create-or-update: re-connecting an already-configured channel (e.g. to grant
	// the EventSub chat scopes) must succeed, not fail on the UNIQUE(overlay_id, platform,
	// channel_id) constraint. Otherwise the OAuth flow stores the new scopes but the frontend
	// shows a spurious "failed to add source" error.
	if err := h.sourceRepo.CreateOrUpdateAuto(c.Request.Context(), source); err != nil {
		h.logger.Error("Failed to upsert source (auto)",
			zap.String("overlay_id", overlayID),
			zap.String("platform", source.Platform),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add source"})
		return
	}

	c.JSON(http.StatusOK, source)
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
