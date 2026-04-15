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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	maxMessageLength = 500
	rateLimit1Min    = 20
	rateLimit1Hour   = 100
)

// classifySendError returns the appropriate HTTP status code and a user-friendly
// description based on the error message. Prevents returning 502 for errors that
// are not actual gateway failures.
func classifySendError(errMsg string) (int, string) {
	// Stream offline / not live — normal condition, not a server error
	for _, s := range []string{
		"not currently live",
		"no live videos found",
		"no active live chat",
		"no live stream found",
	} {
		if strings.Contains(errMsg, s) {
			return http.StatusUnprocessableEntity, "The streamer is not currently live. Messages can only be sent during an active live stream."
		}
	}
	// Missing account/config — user needs to link their account
	for _, s := range []string{
		"no account linked",
		"no Twitch account",
		"no Kick account",
		"no active YouTube source",
	} {
		if strings.Contains(errMsg, s) {
			return http.StatusUnprocessableEntity, "The streamer has not configured this platform in All-Chat."
		}
	}
	// Auth issues
	for _, s := range []string{
		"no refresh token",
		"failed to decrypt",
		"failed to refresh token",
	} {
		if strings.Contains(errMsg, s) {
			return http.StatusUnauthorized, "Your session has expired. Please log in again."
		}
	}
	// Everything else is an actual upstream failure
	return http.StatusBadGateway, "Failed to deliver your message. Please try again."
}

// ViewerRepositoryInterface defines the interface for viewer repository operations
type ViewerRepositoryInterface interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.ViewerSession, error)
	DecryptAccessToken(token string) (string, error)
	DecryptRefreshToken(token string) (string, error)
	Update(ctx context.Context, session *models.ViewerSession) error
	UpdateRateLimits(ctx context.Context, sessionID uuid.UUID, count1Min, count1Hour int, reset1Min, reset1Hour time.Time) error
	LogMessage(ctx context.Context, log *models.ViewerMessageLog) error
}

// UserRepositoryInterface defines the interface for user repository operations
type UserRepositoryInterface interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error
}

// OAuthTokenRefresher defines the interface for OAuth token refresh operations
type OAuthTokenRefresher interface {
	RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error)
}

// ChatSendHandler handles viewer message sending
type ChatSendHandler struct {
	log             *zap.Logger
	viewerRepo      ViewerRepositoryInterface
	userRepo        UserRepositoryInterface
	db              *pgxpool.Pool
	httpClient      *http.Client
	clientID        string
	twitchProvider  OAuthTokenRefresher
	youtubeProvider OAuthTokenRefresher
	kickProvider    OAuthTokenRefresher
	cipher          StringEncryptor
	youtubeAPIKey   string
	redisClient     *redis.Client
}

// NewChatSendHandler creates a new chat send handler
func NewChatSendHandler(
	log *zap.Logger,
	viewerRepo ViewerRepositoryInterface,
	userRepo UserRepositoryInterface,
	db *pgxpool.Pool,
	clientID string,
	twitchProvider OAuthTokenRefresher,
	youtubeProvider OAuthTokenRefresher,
	kickProvider OAuthTokenRefresher,
	cipher StringEncryptor,
	youtubeAPIKey string,
	redisClient *redis.Client,
) *ChatSendHandler {
	return &ChatSendHandler{
		log:             log.Named("chat-send"),
		viewerRepo:      viewerRepo,
		userRepo:        userRepo,
		db:              db,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		clientID:        clientID,
		twitchProvider:  twitchProvider,
		youtubeProvider: youtubeProvider,
		kickProvider:    kickProvider,
		cipher:          cipher,
		youtubeAPIKey:   youtubeAPIKey,
		redisClient:     redisClient,
	}
}

// SendMessageRequest is the request body for sending a message
type SendMessageRequest struct {
	StreamerUsername string `json:"streamer_username" binding:"required"`
	Message          string `json:"message" binding:"required"`
	Platform         string `json:"platform"`  // Optional: if viewer has multiple platforms
	VideoID          string `json:"video_id"`  // Optional: YouTube video ID from extension (bypasses unreliable search.list discovery)
}

// SendMessageResponse is the response after sending a message
type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HandleSendMessage handles the POST /chat/send endpoint
func (h *ChatSendHandler) HandleSendMessage(c *gin.Context) {
	// Extract viewer session ID from JWT claims
	sessionIDStr, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session_id not found in token"})
		return
	}

	sessionID, err := uuid.Parse(sessionIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session_id format"})
		return
	}

	// Parse request body
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	h.log.Info("Request parsed",
		zap.String("streamer_username", req.StreamerUsername),
		zap.String("platform", req.Platform),
		zap.String("video_id", req.VideoID))

	// Validate message length
	if len(req.Message) == 0 || len(req.Message) > maxMessageLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("message must be between 1 and %d characters", maxMessageLength)})
		return
	}

	// Get viewer session
	ctx := c.Request.Context()
	session, err := h.viewerRepo.GetByID(ctx, sessionID)
	if err != nil {
		h.log.Error("Failed to get viewer session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get session"})
		return
	}

	// Check if viewer is banned
	if session.IsBanned {
		reason := "No reason provided"
		if session.BannedReason != nil {
			reason = *session.BannedReason
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error":  "You have been banned from sending messages",
			"reason": reason,
		})
		return
	}

	// Check if platform matches (if specified)
	if req.Platform != "" && req.Platform != session.Platform {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform mismatch"})
		return
	}

	// Refresh token if expired or expiring soon
	if err := h.refreshTokenIfNeeded(ctx, session); err != nil {
		h.log.Error("Failed to refresh token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired, please re-authenticate"})
		return
	}

	// Check rate limits
	allowed, resetTime := h.checkRateLimit(session)
	if !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":      "rate limit exceeded",
			"reset_time": resetTime,
		})
		return
	}

	// Get streamer user ID
	streamerUser, err := h.userRepo.GetByUsername(ctx, req.StreamerUsername)
	if err != nil {
		h.log.Error("Failed to get streamer user", zap.Error(err), zap.String("username", req.StreamerUsername))
		c.JSON(http.StatusNotFound, gin.H{"error": "streamer not found"})
		return
	}

	// Check if the resolved user has the required platform configured.
	// If not, try to find an alternate account (duplicate account scenario:
	// same person registered separately via different platforms).
	streamerUser = h.resolveStreamerForPlatform(ctx, streamerUser, session.Platform)

	// Send message based on platform
	h.log.Info("Sending message to platform",
		zap.String("platform", session.Platform),
		zap.String("streamer", streamerUser.Username),
		zap.String("viewer_session_id", sessionID.String()))

	var messageErr error
	switch session.Platform {
	case "twitch":
		messageErr = h.sendTwitchMessage(ctx, session, streamerUser, req.Message)
	case "youtube":
		messageErr = h.sendYouTubeMessage(ctx, session, streamerUser, req.Message, req.VideoID)
		if messageErr != nil {
			h.log.Error("Failed to send YouTube message", zap.Error(messageErr))
		}
	case "kick":
		messageErr = h.sendKickMessage(ctx, session, streamerUser, req.Message)
	case "tiktok":
		c.JSON(http.StatusNotImplemented, gin.H{"error": "TikTok message sending not yet implemented"})
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform"})
		return
	}

	// Update rate limits
	if err := h.updateRateLimits(ctx, session); err != nil {
		h.log.Error("Failed to update rate limits", zap.Error(err))
		// Continue anyway - message was sent
	}

	// Log message
	var channelID string
	if streamerUser.TwitchID != nil {
		channelID = *streamerUser.TwitchID
	}

	streamerUUID, err := uuid.Parse(streamerUser.ID)
	if err != nil {
		h.log.Error("Failed to parse streamer user ID", zap.Error(err))
		// Use nil UUID if parsing fails
		streamerUUID = uuid.Nil
	}

	messageLog := &models.ViewerMessageLog{
		ViewerSessionID: session.ID,
		StreamerUserID:  streamerUUID,
		Platform:        session.Platform,
		ChannelID:       channelID, // Platform-specific channel ID
		ChannelName:     streamerUser.Username,
		MessageText:     req.Message,
		SentAt:          time.Now(),
		Success:         messageErr == nil,
	}

	if messageErr != nil {
		errMsg := messageErr.Error()
		messageLog.ErrorMessage = &errMsg
	}

	if err := h.viewerRepo.LogMessage(ctx, messageLog); err != nil {
		h.log.Error("Failed to log message", zap.Error(err))
		// Continue anyway
	}

	// Return response with appropriate status code based on error type
	if messageErr != nil {
		errMsg := messageErr.Error()
		status, description := classifySendError(errMsg)
		c.JSON(status, gin.H{"error": "failed to send message", "details": errMsg, "description": description})
		return
	}

	c.JSON(http.StatusOK, SendMessageResponse{
		Success: true,
		Message: "Message sent successfully",
	})
}

// StreamerSendMessageRequest is the request body for a streamer sending a message in their own chat
type StreamerSendMessageRequest struct {
	Message  string `json:"message" binding:"required"`
	Platform string `json:"platform" binding:"required"` // Which platform to send to
}

// HandleStreamerSendMessage handles POST /chat/send for authenticated streamers.
// Unlike the viewer flow, this uses the streamer's own stored OAuth tokens.
func (h *ChatSendHandler) HandleStreamerSendMessage(c *gin.Context) {
	// Extract user_id from JWT claims (streamer token)
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found in token"})
		return
	}

	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id format"})
		return
	}

	// Parse request body
	var req StreamerSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: message and platform are required"})
		return
	}

	// Validate message length
	if len(req.Message) == 0 || len(req.Message) > maxMessageLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("message must be between 1 and %d characters", maxMessageLength)})
		return
	}

	// Get streamer user record (includes decrypted OAuth tokens)
	ctx := c.Request.Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get streamer user", zap.Error(err), zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	if user.IsBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "your account is banned"})
		return
	}

	// Refresh token if expired or expiring soon
	if err := h.refreshStreamerTokenIfNeeded(ctx, user); err != nil {
		h.log.Error("Failed to refresh streamer token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired, please re-authenticate"})
		return
	}

	h.log.Info("Streamer sending message",
		zap.String("platform", req.Platform),
		zap.String("user_id", userID),
		zap.String("username", user.Username))

	// Send message based on target platform
	var messageErr error
	switch req.Platform {
	case "twitch":
		messageErr = h.sendStreamerTwitchMessage(ctx, user, req.Message)
	case "youtube":
		messageErr = h.sendStreamerYouTubeMessage(ctx, user, req.Message)
	case "kick":
		messageErr = h.sendStreamerKickMessage(ctx, user, req.Message)
	case "tiktok":
		c.JSON(http.StatusNotImplemented, gin.H{"error": "TikTok message sending not yet implemented"})
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform"})
		return
	}

	if messageErr != nil {
		h.log.Error("Failed to send streamer message", zap.Error(messageErr), zap.String("platform", req.Platform))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send message", "details": messageErr.Error()})
		return
	}

	c.JSON(http.StatusOK, SendMessageResponse{
		Success: true,
		Message: "Message sent successfully",
	})
}

// refreshStreamerTokenIfNeeded refreshes the streamer's OAuth token if expired
func (h *ChatSendHandler) refreshStreamerTokenIfNeeded(ctx context.Context, user *models.User) error {
	if user.TokenExpiresAt.After(time.Now().Add(5 * time.Minute)) {
		return nil
	}

	h.log.Info("Streamer access token expired or expiring soon, refreshing",
		zap.String("auth_provider", user.AuthProvider),
		zap.Time("expires_at", user.TokenExpiresAt))

	if user.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	var newToken *oauth2.Token
	var err error
	switch user.AuthProvider {
	case "twitch":
		newToken, err = h.twitchProvider.RefreshToken(ctx, user.RefreshToken)
	case "youtube":
		newToken, err = h.youtubeProvider.RefreshToken(ctx, user.RefreshToken)
	case "kick":
		newToken, err = h.kickProvider.RefreshToken(ctx, user.RefreshToken)
	default:
		return fmt.Errorf("unsupported auth provider for token refresh: %s", user.AuthProvider)
	}
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	refreshToken := newToken.RefreshToken
	if refreshToken == "" {
		refreshToken = user.RefreshToken
	}

	if err := h.userRepo.UpdateTokens(ctx, user.ID, newToken.AccessToken, refreshToken, newToken.Expiry); err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	// Update in-memory user so the rest of the handler uses the new token
	user.AccessToken = newToken.AccessToken
	user.RefreshToken = refreshToken
	user.TokenExpiresAt = newToken.Expiry

	return nil
}

// sendStreamerTwitchMessage sends a message to Twitch chat as the streamer (broadcaster)
func (h *ChatSendHandler) sendStreamerTwitchMessage(ctx context.Context, user *models.User, message string) error {
	if user.TwitchID == nil || *user.TwitchID == "" {
		return fmt.Errorf("no Twitch account linked")
	}
	broadcasterID := *user.TwitchID

	reqBody := map[string]string{
		"broadcaster_id": broadcasterID,
		"sender_id":      broadcasterID, // streamer sends as themselves
		"message":        message,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.twitch.tv/helix/chat/messages", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.AccessToken))
	req.Header.Set("Client-Id", h.clientID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("twitch API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// sendStreamerYouTubeMessage sends a message to YouTube live chat as the streamer
func (h *ChatSendHandler) sendStreamerYouTubeMessage(ctx context.Context, user *models.User, message string) error {
	channelID, err := h.getActiveYouTubeChannelID(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to get YouTube channel ID: %w", err)
	}

	liveChatID, err := h.getYouTubeLiveChatID(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get live chat ID: %w", err)
	}

	reqBody := map[string]interface{}{
		"snippet": map[string]interface{}{
			"liveChatId": liveChatID,
			"type":       "textMessageEvent",
			"textMessageDetails": map[string]string{
				"messageText": message,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := "https://www.googleapis.com/youtube/v3/liveChat/messages?part=snippet"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("youtube API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// sendStreamerKickMessage sends a message to Kick chat as the streamer
func (h *ChatSendHandler) sendStreamerKickMessage(ctx context.Context, user *models.User, message string) error {
	if user.KickID == nil || *user.KickID == "" {
		return fmt.Errorf("no Kick account linked")
	}

	reqBody := map[string]interface{}{
		"type":                "user",
		"content":             message,
		"broadcaster_user_id": *user.KickID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.kick.com/public/v1/chat", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.AccessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("kick API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// checkRateLimit checks if the viewer is within rate limits
func (h *ChatSendHandler) checkRateLimit(session *models.ViewerSession) (allowed bool, resetTime time.Time) {
	now := time.Now()

	// Check 1-minute rate limit
	if session.RateLimitReset1Min != nil && now.Before(*session.RateLimitReset1Min) {
		if session.MessageCount1Min >= rateLimit1Min {
			return false, *session.RateLimitReset1Min
		}
	}

	// Check 1-hour rate limit
	if session.RateLimitReset1Hour != nil && now.Before(*session.RateLimitReset1Hour) {
		if session.MessageCount1Hour >= rateLimit1Hour {
			return false, *session.RateLimitReset1Hour
		}
	}

	return true, time.Time{}
}

// updateRateLimits updates the rate limit counters after sending a message
func (h *ChatSendHandler) updateRateLimits(ctx context.Context, session *models.ViewerSession) error {
	now := time.Now()

	// Update 1-minute counter
	count1Min := session.MessageCount1Min
	reset1Min := session.RateLimitReset1Min
	if reset1Min == nil || now.After(*reset1Min) {
		count1Min = 1
		newReset := now.Add(1 * time.Minute)
		reset1Min = &newReset
	} else {
		count1Min++
	}

	// Update 1-hour counter
	count1Hour := session.MessageCount1Hour
	reset1Hour := session.RateLimitReset1Hour
	if reset1Hour == nil || now.After(*reset1Hour) {
		count1Hour = 1
		newReset := now.Add(1 * time.Hour)
		reset1Hour = &newReset
	} else {
		count1Hour++
	}

	return h.viewerRepo.UpdateRateLimits(ctx, session.ID, count1Min, count1Hour, *reset1Min, *reset1Hour)
}

// refreshTokenIfNeeded checks if the access token is expired and refreshes it if needed
func (h *ChatSendHandler) refreshTokenIfNeeded(ctx context.Context, session *models.ViewerSession) error {
	// Check if token is expired or will expire in the next 5 minutes
	if session.TokenExpiresAt.After(time.Now().Add(5 * time.Minute)) {
		return nil // Token is still valid
	}

	h.log.Info("Access token expired or expiring soon, refreshing",
		zap.String("platform", session.Platform),
		zap.Time("expires_at", session.TokenExpiresAt))

	// Check if refresh token exists
	if session.RefreshToken == nil {
		return fmt.Errorf("no refresh token available")
	}

	// Decrypt refresh token
	refreshToken, err := h.viewerRepo.DecryptRefreshToken(*session.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	// Refresh token using the appropriate provider
	var newToken *oauth2.Token
	switch session.Platform {
	case "twitch":
		newToken, err = h.twitchProvider.RefreshToken(ctx, refreshToken)
	case "youtube":
		newToken, err = h.youtubeProvider.RefreshToken(ctx, refreshToken)
	case "kick":
		newToken, err = h.kickProvider.RefreshToken(ctx, refreshToken)
	default:
		return fmt.Errorf("unsupported platform for token refresh: %s", session.Platform)
	}

	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// Encrypt new tokens
	encryptedAccess, err := h.cipher.Encrypt(newToken.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt new access token: %w", err)
	}

	var encryptedRefresh *string
	if newToken.RefreshToken != "" {
		encrypted, err := h.cipher.Encrypt(newToken.RefreshToken)
		if err != nil {
			h.log.Warn("Failed to encrypt new refresh token", zap.Error(err))
		} else {
			encryptedRefresh = &encrypted
		}
	}

	// Update session with new tokens
	session.AccessToken = encryptedAccess
	session.RefreshToken = encryptedRefresh
	session.TokenExpiresAt = newToken.Expiry

	if err := h.viewerRepo.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session with new tokens: %w", err)
	}

	h.log.Info("Successfully refreshed access token",
		zap.String("platform", session.Platform),
		zap.Time("new_expiry", newToken.Expiry))

	return nil
}

// resolveStreamerForPlatform checks if the resolved streamer user has the
// required platform configured. If not, it searches for an alternate user
// account with the same display name that does have the platform configured.
// This handles the duplicate-account scenario where a streamer registered
// separately via different platforms (e.g., Twitch and YouTube as two accounts).
func (h *ChatSendHandler) resolveStreamerForPlatform(ctx context.Context, streamer *models.User, platform string) *models.User {
	if h.streamerHasPlatformCtx(ctx, streamer, platform) {
		return streamer
	}

	h.log.Warn("Streamer missing platform, searching for alternate account",
		zap.String("streamer_username", streamer.Username),
		zap.String("streamer_display_name", streamer.DisplayName),
		zap.String("required_platform", platform))

	// Search for another user with the same display name who has the required
	// platform configured and an active public overlay with a source for it.
	query := `
		SELECT u.id, u.twitch_id, u.google_id, u.kick_id, u.auth_provider,
		       u.username, u.display_name, u.profile_image_url,
		       u.is_admin, u.is_premium, u.is_banned,
		       u.banned_at, u.banned_reason, u.banned_by,
		       u.access_token, u.refresh_token, u.token_expires_at,
		       u.created_at, u.updated_at
		FROM users u
		JOIN overlays o ON o.user_id = u.id
		JOIN overlay_chat_sources ocs ON ocs.overlay_id = o.id
		WHERE LOWER(u.display_name) = LOWER($1)
		  AND u.id != $2
		  AND u.is_banned = false
		  AND o.is_active = true
		  AND o.is_public_for_viewers = true
		  AND ocs.platform = $3
		  AND ocs.is_active = true
		LIMIT 1
	`

	row := h.db.QueryRow(ctx, query, streamer.DisplayName, streamer.ID, platform)

	alt := &models.User{}
	err := row.Scan(
		&alt.ID, &alt.TwitchID, &alt.GoogleID, &alt.KickID, &alt.AuthProvider,
		&alt.Username, &alt.DisplayName, &alt.ProfileImageURL,
		&alt.IsAdmin, &alt.IsPremium, &alt.IsBanned,
		&alt.BannedAt, &alt.BannedReason, &alt.BannedBy,
		&alt.AccessToken, &alt.RefreshToken, &alt.TokenExpiresAt,
		&alt.CreatedAt, &alt.UpdatedAt,
	)
	if err != nil {
		h.log.Warn("No alternate account found for platform",
			zap.String("display_name", streamer.DisplayName),
			zap.String("platform", platform),
			zap.Error(err))
		return streamer
	}

	h.log.Info("Resolved alternate streamer account for platform",
		zap.String("original_username", streamer.Username),
		zap.String("alternate_username", alt.Username),
		zap.String("platform", platform))

	return alt
}

// streamerHasPlatform checks if a streamer user has the given platform configured.
// For YouTube, it checks overlay_chat_sources rather than google_id since the
// active channel may differ from the user's Google account ID.
func (h *ChatSendHandler) streamerHasPlatformCtx(ctx context.Context, streamer *models.User, platform string) bool {
	switch platform {
	case "twitch":
		return streamer.TwitchID != nil && *streamer.TwitchID != ""
	case "youtube":
		// YouTube is checked via overlay sources in sendYouTubeMessage,
		// so we verify the user has an active YouTube source configured
		query := `
			SELECT EXISTS(
				SELECT 1 FROM overlay_chat_sources ocs
				JOIN overlays o ON ocs.overlay_id = o.id
				WHERE o.user_id = $1
				  AND ocs.platform = 'youtube'
				  AND ocs.is_active = true
				  AND o.is_public_for_viewers = true
			)
		`
		var exists bool
		if err := h.db.QueryRow(ctx, query, streamer.ID).Scan(&exists); err != nil {
			return false
		}
		return exists
	case "kick":
		return streamer.KickID != nil && *streamer.KickID != ""
	default:
		return false
	}
}

// sendTwitchMessage sends a message to Twitch chat using the Helix API
func (h *ChatSendHandler) sendTwitchMessage(ctx context.Context, session *models.ViewerSession, streamer *models.User, message string) error {
	// Decrypt access token
	accessToken, err := h.viewerRepo.DecryptAccessToken(session.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// Get broadcaster ID (streamer's Twitch user ID)
	if streamer.TwitchID == nil || *streamer.TwitchID == "" {
		return fmt.Errorf("streamer has no Twitch account linked")
	}
	broadcasterID := *streamer.TwitchID

	// Prepare request body
	reqBody := map[string]string{
		"broadcaster_id": broadcasterID,
		"sender_id":      session.PlatformUserID,
		"message":        message,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.twitch.tv/helix/chat/messages", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Client-Id", h.clientID)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("twitch API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// sendYouTubeMessage sends a message to YouTube live chat using the Live Chat API.
// videoID is an optional YouTube video ID provided by the extension (extracted from the page URL).
// When provided, the backend can look up the liveChatId via a single videos.list call (1 quota unit)
// instead of relying on the unreliable search.list API (100 quota units).
func (h *ChatSendHandler) sendYouTubeMessage(ctx context.Context, session *models.ViewerSession, streamer *models.User, message string, videoID string) error {
	// Decrypt access token
	accessToken, err := h.viewerRepo.DecryptAccessToken(session.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// Get the active YouTube channel ID from overlay_chat_sources
	// This is more accurate than using the users.google_id because it reflects
	// the currently active YouTube source, which may change when streamers switch channels
	channelID, err := h.getActiveYouTubeChannelID(ctx, streamer.ID)
	if err != nil {
		return fmt.Errorf("failed to get YouTube channel ID: %w", err)
	}

	// Get the streamer's active livestream liveChatId.
	// Strategy order:
	//   1. Redis cache from youtube-listener (free, fast, reliable)
	//   2. videos.list with video_id from extension (1 quota unit, reliable)
	//   3. search.list + videos.list fallback (100+ quota units, unreliable)
	liveChatID, err := h.getYouTubeLiveChatIDWithVideoID(ctx, channelID, videoID)
	if err != nil {
		return fmt.Errorf("failed to get live chat ID: %w", err)
	}

	// Insert the message into the live chat
	reqBody := map[string]interface{}{
		"snippet": map[string]interface{}{
			"liveChatId": liveChatID,
			"type":       "textMessageEvent",
			"textMessageDetails": map[string]string{
				"messageText": message,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	url := "https://www.googleapis.com/youtube/v3/liveChat/messages?part=snippet"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("youtube API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// getYouTubeLiveChatIDWithVideoID gets the live chat ID for a streamer's active broadcast.
//
// Strategy (in order):
//  1. Check Redis for stream state cached by the youtube-listener service.
//     This is fast, free (no API quota), and the most reliable source because
//     the youtube-listener already monitors liveness continuously.
//  2. If the extension provided a video ID, use videos.list to look up the
//     liveChatId directly (1 quota unit, 100% reliable for active streams).
//  3. Fall back to the YouTube Data API search.list + videos.list with a
//     server-side API key. This path is unreliable (YouTube's search index
//     lags behind reality) but serves as a safety net.
func (h *ChatSendHandler) getYouTubeLiveChatIDWithVideoID(ctx context.Context, channelID string, videoID string) (string, error) {
	// --- Strategy 1: Redis lookup (youtube-listener cached state) ---
	if h.redisClient != nil {
		liveChatID, err := h.getYouTubeLiveChatIDFromRedis(ctx, channelID)
		if err != nil {
			h.log.Warn("Redis stream state lookup failed",
				zap.String("channel_id", channelID),
				zap.Error(err))
		} else if liveChatID != "" {
			h.log.Info("Got live chat ID from Redis (youtube-listener cache)",
				zap.String("channel_id", channelID),
				zap.String("live_chat_id", liveChatID))
			return liveChatID, nil
		} else {
			h.log.Info("No stream state in Redis for channel",
				zap.String("channel_id", channelID))
		}
	}

	// --- Strategy 2: Extension-provided video ID (cheap videos.list call) ---
	if videoID != "" {
		h.log.Info("Trying extension-provided video ID for liveChatId lookup",
			zap.String("video_id", videoID))
		liveChatID, err := h.getYouTubeLiveChatIDFromVideoID(ctx, videoID)
		if err != nil {
			h.log.Warn("Video ID lookup failed, falling back to search.list",
				zap.String("video_id", videoID),
				zap.Error(err))
		} else {
			return liveChatID, nil
		}
	}

	// --- Strategy 3: YouTube Data API search.list fallback (unreliable) ---
	return h.getYouTubeLiveChatIDFromAPI(ctx, channelID)
}

// getYouTubeLiveChatID is the legacy entry point without video ID support.
// Kept for backwards compatibility with callers that don't have a video ID.
func (h *ChatSendHandler) getYouTubeLiveChatID(ctx context.Context, channelID string) (string, error) {
	return h.getYouTubeLiveChatIDWithVideoID(ctx, channelID, "")
}

// getYouTubeLiveChatIDFromVideoID looks up the liveChatId for a specific video
// using the YouTube Data API videos.list endpoint. This costs only 1 quota unit
// (vs 100 for search.list) and is 100% reliable for active live streams.
func (h *ChatSendHandler) getYouTubeLiveChatIDFromVideoID(ctx context.Context, videoID string) (string, error) {
	authParam := ""
	if h.youtubeAPIKey != "" {
		authParam = "&key=" + h.youtubeAPIKey
	}

	videoURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=liveStreamingDetails&id=%s%s", videoID, authParam)
	videoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create video request: %w", err)
	}

	videoResp, err := h.httpClient.Do(videoReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch video details: %w", err)
	}
	defer videoResp.Body.Close()

	if videoResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(videoResp.Body, 1024))
		return "", fmt.Errorf("youtube videos API error: status=%d body=%s", videoResp.StatusCode, string(body))
	}

	var videoResult struct {
		Items []struct {
			LiveStreamingDetails struct {
				ActiveLiveChatID string `json:"activeLiveChatId"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(videoResp.Body).Decode(&videoResult); err != nil {
		return "", fmt.Errorf("failed to decode video response: %w", err)
	}

	if len(videoResult.Items) == 0 || videoResult.Items[0].LiveStreamingDetails.ActiveLiveChatID == "" {
		return "", fmt.Errorf("no active live chat found for video %s", videoID)
	}

	h.log.Info("Got live chat ID from video ID lookup",
		zap.String("video_id", videoID),
		zap.String("live_chat_id", videoResult.Items[0].LiveStreamingDetails.ActiveLiveChatID))

	return videoResult.Items[0].LiveStreamingDetails.ActiveLiveChatID, nil
}

// getYouTubeLiveChatIDFromRedis reads the live chat ID from the youtube-listener's
// stream state cache in Redis. Returns ("", nil) if no state exists.
func (h *ChatSendHandler) getYouTubeLiveChatIDFromRedis(ctx context.Context, channelID string) (string, error) {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)
	data, err := h.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // No state — channel may not be live or youtube-listener hasn't cached it
	}
	if err != nil {
		return "", fmt.Errorf("redis GET failed: %w", err)
	}

	var state struct {
		LiveChatID string `json:"live_chat_id"`
		IsLive     bool   `json:"is_live"`
	}
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return "", fmt.Errorf("failed to unmarshal stream state: %w", err)
	}

	if !state.IsLive || state.LiveChatID == "" {
		return "", nil
	}

	return state.LiveChatID, nil
}

// getYouTubeLiveChatIDFromAPI uses the YouTube Data API (search.list + videos.list)
// with a server-side API key to discover the live chat ID. This is the fallback path;
// the search.list endpoint is unreliable due to YouTube's search index lag.
func (h *ChatSendHandler) getYouTubeLiveChatIDFromAPI(ctx context.Context, channelID string) (string, error) {
	// Build authentication parameter: prefer API key, fall back to nothing (will likely 401)
	authParam := ""
	if h.youtubeAPIKey != "" {
		authParam = "&key=" + h.youtubeAPIKey
	}

	// Step 1: Search for live videos on the channel using the server-side API key
	searchURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&type=video&eventType=live&maxResults=1%s", channelID, authParam)
	searchReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create search request: %w", err)
	}

	searchResp, err := h.httpClient.Do(searchReq)
	if err != nil {
		return "", fmt.Errorf("failed to search for live videos: %w", err)
	}
	defer searchResp.Body.Close()

	if searchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(searchResp.Body, 1024))
		return "", fmt.Errorf("youtube search API error: status=%d body=%s", searchResp.StatusCode, string(body))
	}

	var searchResult struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}

	if err := json.NewDecoder(searchResp.Body).Decode(&searchResult); err != nil {
		return "", fmt.Errorf("failed to decode search response: %w", err)
	}

	h.log.Info("YouTube Search API response",
		zap.String("channel_id", channelID),
		zap.Int("num_results", len(searchResult.Items)))

	if len(searchResult.Items) == 0 {
		return "", fmt.Errorf("streamer is not currently live on YouTube (no live videos found for channel %s)", channelID)
	}

	videoID := searchResult.Items[0].ID.VideoID

	h.log.Info("Found live video", zap.String("video_id", videoID))

	// Step 2: Get video details to extract liveChatId using the server-side API key
	videoURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?part=liveStreamingDetails&id=%s%s", videoID, authParam)
	videoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create video request: %w", err)
	}

	videoResp, err := h.httpClient.Do(videoReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch video details: %w", err)
	}
	defer videoResp.Body.Close()

	if videoResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(videoResp.Body, 1024))
		return "", fmt.Errorf("youtube videos API error: status=%d body=%s", videoResp.StatusCode, string(body))
	}

	var videoResult struct {
		Items []struct {
			LiveStreamingDetails struct {
				ActiveLiveChatID string `json:"activeLiveChatId"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(videoResp.Body).Decode(&videoResult); err != nil {
		return "", fmt.Errorf("failed to decode video response: %w", err)
	}

	if len(videoResult.Items) == 0 || videoResult.Items[0].LiveStreamingDetails.ActiveLiveChatID == "" {
		return "", fmt.Errorf("no active live chat found for stream")
	}

	return videoResult.Items[0].LiveStreamingDetails.ActiveLiveChatID, nil
}

// getActiveYouTubeChannelID gets the channel ID of the active YouTube source for a streamer
// This is retrieved from overlay_chat_sources rather than users.google_id because the
// active channel may change when streamers switch between different YouTube channels
func (h *ChatSendHandler) getActiveYouTubeChannelID(ctx context.Context, streamerUserID string) (string, error) {
	query := `
		SELECT ocs.channel_id
		FROM overlay_chat_sources ocs
		INNER JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.user_id = $1
		  AND ocs.platform = 'youtube'
		  AND ocs.is_active = true
		  AND o.is_public_for_viewers = true
		LIMIT 1
	`

	var channelID string
	err := h.db.QueryRow(ctx, query, streamerUserID).Scan(&channelID)
	if err != nil {
		h.log.Error("Failed to get active YouTube channel ID",
			zap.Error(err),
			zap.String("streamer_user_id", streamerUserID))
		return "", fmt.Errorf("no active YouTube source found for streamer")
	}

	h.log.Info("Retrieved active YouTube channel ID",
		zap.String("streamer_user_id", streamerUserID),
		zap.String("channel_id", channelID))

	return channelID, nil
}

// sendKickMessage sends a message to Kick chat using the Kick API
func (h *ChatSendHandler) sendKickMessage(ctx context.Context, session *models.ViewerSession, streamer *models.User, message string) error {
	// Decrypt access token
	accessToken, err := h.viewerRepo.DecryptAccessToken(session.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// Get broadcaster ID (streamer's Kick user ID)
	if streamer.KickID == nil || *streamer.KickID == "" {
		return fmt.Errorf("streamer has no Kick account linked")
	}

	// Prepare request body
	// Kick API expects: {"type": "user", "content": "message", "broadcaster_user_id": 123456}
	reqBody := map[string]interface{}{
		"type":                "user",
		"content":             message,
		"broadcaster_user_id": *streamer.KickID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.kick.com/public/v1/chat", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("kick API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}
