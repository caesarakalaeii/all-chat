package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	maxMessageLength = 500
	rateLimit1Min    = 20
	rateLimit1Hour   = 100
)

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
	GetByUsername(ctx context.Context, username string) (*models.User, error)
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
	httpClient      *http.Client
	clientID        string
	twitchProvider  OAuthTokenRefresher
	youtubeProvider OAuthTokenRefresher
	kickProvider    OAuthTokenRefresher
	cipher          StringEncryptor
}

// NewChatSendHandler creates a new chat send handler
func NewChatSendHandler(
	log *zap.Logger,
	viewerRepo ViewerRepositoryInterface,
	userRepo UserRepositoryInterface,
	clientID string,
	twitchProvider OAuthTokenRefresher,
	youtubeProvider OAuthTokenRefresher,
	kickProvider OAuthTokenRefresher,
	cipher StringEncryptor,
) *ChatSendHandler {
	return &ChatSendHandler{
		log:             log.Named("chat-send"),
		viewerRepo:      viewerRepo,
		userRepo:        userRepo,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		clientID:        clientID,
		twitchProvider:  twitchProvider,
		youtubeProvider: youtubeProvider,
		kickProvider:    kickProvider,
		cipher:          cipher,
	}
}

// SendMessageRequest is the request body for sending a message
type SendMessageRequest struct {
	StreamerUsername string `json:"streamer_username" binding:"required"`
	Message          string `json:"message" binding:"required"`
	Platform         string `json:"platform"` // Optional: if viewer has multiple platforms
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

	// Send message based on platform
	var messageErr error
	switch session.Platform {
	case "twitch":
		messageErr = h.sendTwitchMessage(ctx, session, streamerUser, req.Message)
	case "youtube":
		messageErr = h.sendYouTubeMessage(ctx, session, streamerUser, req.Message)
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

	// Return response
	if messageErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send message", "details": messageErr.Error()})
		return
	}

	c.JSON(http.StatusOK, SendMessageResponse{
		Success: true,
		Message: "Message sent successfully",
	})
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

// sendYouTubeMessage sends a message to YouTube live chat using the Live Chat API
func (h *ChatSendHandler) sendYouTubeMessage(ctx context.Context, session *models.ViewerSession, streamer *models.User, message string) error {
	// Decrypt access token
	accessToken, err := h.viewerRepo.DecryptAccessToken(session.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// Get streamer's Google ID
	if streamer.GoogleID == nil || *streamer.GoogleID == "" {
		return fmt.Errorf("streamer has no YouTube account linked")
	}

	// First, get the streamer's active livestream to find the liveChatId
	// This requires querying the YouTube API for active broadcasts
	liveChatID, err := h.getYouTubeLiveChatID(ctx, accessToken, *streamer.GoogleID)
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

// getYouTubeLiveChatID gets the live chat ID for a streamer's active broadcast
func (h *ChatSendHandler) getYouTubeLiveChatID(ctx context.Context, accessToken, channelID string) (string, error) {
	// Query for active livestreams for this channel
	url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/liveBroadcasts?part=snippet&broadcastStatus=active&maxResults=1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch broadcasts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("youtube broadcasts API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Snippet struct {
				LiveChatID string `json:"liveChatId"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Items) == 0 || result.Items[0].Snippet.LiveChatID == "" {
		return "", fmt.Errorf("streamer is not currently live on YouTube")
	}

	return result.Items[0].Snippet.LiveChatID, nil
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
