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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/caesar/all-chat/shared/sendall"
	"github.com/caesar/all-chat/shared/youtubetoken"
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

// YouTubeTokenSource resolves the streamer's own YouTube broadcaster credential for a
// channel (per-channel youtube_oauth_tokens preferred, users row fallback) and refreshes
// it via Google's OAuth endpoint. auth-service uses this for streamer chat sends instead
// of users.access_token: a streamer whose All-Chat login is Twitch has a Twitch token on
// the users row, so sending it to the YouTube Data API 401s with "Invalid Credentials".
// The valid YouTube token lives in youtube_oauth_tokens; this source reads it (shared
// with moderation-service's ban dispatch via shared/youtubetoken so the two paths never
// diverge on which token they use).
type YouTubeTokenSource interface {
	Resolve(ctx context.Context, userID, channelID string) (*youtubetoken.YouTubeCredential, error)
	Refresh(ctx context.Context, cred *youtubetoken.YouTubeCredential) error
}

// WithYouTubeTokenSource wires the shared YouTube broadcaster-credential source used by
// streamer chat sends. Required for YouTube sends to resolve the per-channel YouTube
// token from youtube_oauth_tokens instead of the (possibly cross-platform) users token.
func (h *ChatSendHandler) WithYouTubeTokenSource(ts YouTubeTokenSource) *ChatSendHandler {
	h.ytTokenSource = ts
	return h
}

// sendErrKind classifies a streamer-send failure so HandleStreamerSendMessage can map
// it to the right HTTP status and a machine-readable code the monitor view reacts to
// (e.g. prompt the advanced-controls opt-in on a missing scope).
type sendErrKind string

const (
	sendErrUpstream     sendErrKind = "send_failed"     // platform 5xx / network → 502
	sendErrMissingScope sendErrKind = "missing_scope"   // platform 403 → 403, prompt opt-in
	sendErrReauth       sendErrKind = "reauth_required" // platform 401 → 401, prompt re-login
	sendErrOffline      sendErrKind = "stream_offline"  // not live → 422
	sendErrQuota        sendErrKind = "quota_exhausted" // YouTube send quota depleted → 422
)

// streamerSendError carries a classified streamer-send failure.
type streamerSendError struct {
	kind sendErrKind
	msg  string
}

func (e *streamerSendError) Error() string { return e.msg }

// classifyPlatformStatus maps a platform HTTP error response to a typed send error:
// 401 ⇒ re-auth (token expired/invalid), 403 ⇒ missing scope (prompt the opt-in),
// anything else ⇒ an upstream failure. Previously every platform error surfaced as a
// raw 502, so the monitor could not tell "needs re-consent" from "platform hiccup".
func classifyPlatformStatus(platform string, status int, body string) *streamerSendError {
	switch status {
	case http.StatusUnauthorized:
		return &streamerSendError{kind: sendErrReauth, msg: fmt.Sprintf("%s auth rejected (401): %s", platform, body)}
	case http.StatusForbidden:
		return &streamerSendError{kind: sendErrMissingScope, msg: fmt.Sprintf("%s missing send scope (403): %s", platform, body)}
	default:
		return &streamerSendError{kind: sendErrUpstream, msg: fmt.Sprintf("%s API error: status=%d body=%s", platform, status, body)}
	}
}

// streamerSendHTTPResponse maps a send error to (HTTP status, JSON body) for the
// streamer endpoint. Typed errors map directly; an untyped error falls back to the
// text-based classifier (e.g. "not currently live" → 422).
func streamerSendHTTPResponse(platform string, err error) (int, gin.H) {
	var se *streamerSendError
	if errors.As(err, &se) {
		switch se.kind {
		case sendErrMissingScope:
			return http.StatusForbidden, gin.H{"error": string(sendErrMissingScope), "platform": platform}
		case sendErrReauth:
			return http.StatusUnauthorized, gin.H{"error": string(sendErrReauth), "platform": platform}
		case sendErrOffline:
			return http.StatusUnprocessableEntity, gin.H{"error": string(sendErrOffline), "platform": platform, "details": "The streamer is not currently live."}
		case sendErrQuota:
			return http.StatusUnprocessableEntity, gin.H{"error": string(sendErrQuota), "platform": platform, "details": "YouTube send quota is exhausted. Please try again later."}
		}
	}
	status, desc := classifySendError(err.Error())
	code := string(sendErrUpstream)
	if status == http.StatusUnauthorized {
		code = string(sendErrReauth)
	} else if status == http.StatusUnprocessableEntity {
		code = "unavailable"
	}
	return status, gin.H{"error": code, "platform": platform, "details": desc}
}

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
	// ytTokenSource resolves the streamer's own YouTube broadcaster credential for a
	// channel from youtube_oauth_tokens (shared with moderation-service) and refreshes it
	// via Google's OAuth endpoint. Streamer YouTube sends use this instead of
	// user.AccessToken: a streamer whose All-Chat login is Twitch has a Twitch token on
	// the users row, which 401s against the YouTube Data API. Nil ⇒ YouTube sends surface
	// reauth_required (the monitor prompts Reconnect) rather than using the wrong token.
	ytTokenSource YouTubeTokenSource
	// ytQuota accounts YouTube sends (5 units each) against the shared
	// youtube_quota_usage table via reserve-confirm-rollback (ADR-0006), so the
	// youtube-quota-monitor reflects send usage and a depleted quota blocks sends.
	// Nil ⇒ accounting skipped (fail-open).
	ytQuota *quota.Reserver
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
	youtubeQuotaLimit int,
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
		ytQuota:         quota.NewReserver(db, youtubeQuotaLimit),
	}
}

// SendMessageRequest is the request body for sending a message
type SendMessageRequest struct {
	StreamerUsername string `json:"streamer_username" binding:"required"`
	Message          string `json:"message" binding:"required"`
	Platform         string `json:"platform"` // Optional: if viewer has multiple platforms
	VideoID          string `json:"video_id"` // Optional: YouTube video ID from extension (bypasses unreliable search.list discovery)
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

	// "all" fans out to every connected platform and returns per-platform results.
	if req.Platform == "all" {
		h.handleStreamerSendToAll(c, ctx, user, req.Message)
		return
	}

	// Single-platform send. Rate-limit against the streamer's own limits (one unit).
	if ok, retryAfter := h.reserveStreamerRate(ctx, user.ID, 1); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "retry_after_seconds": retryAfter})
		return
	}

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
		status, body := streamerSendHTTPResponse(req.Platform, messageErr)
		h.log.Warn("Failed to send streamer message", zap.Error(messageErr), zap.String("platform", req.Platform), zap.Int("status", status))
		c.JSON(status, body)
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
		return classifyPlatformStatus("twitch", resp.StatusCode, string(body))
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

	// Resolve the streamer's YouTube access token from youtube_oauth_tokens (shared
	// source), NOT user.AccessToken: a streamer whose All-Chat login is Twitch has a
	// Twitch token on the users row, which YouTube rejects with 401 "Invalid
	// Credentials". A missing credential surfaces as reauth_required so the monitor
	// prompts Reconnect.
	accessToken, err := h.resolveYouTubeAccessToken(ctx, user.ID, channelID)
	if err != nil {
		return err
	}

	// Account the send against the shared youtube_quota_usage table (a send costs 5
	// units) via reserve-confirm-rollback, so the youtube-quota-monitor reflects send
	// usage and a depleted quota blocks the send. Reserve before sending, then confirm
	// on success / roll back on failure. Fails open when accounting is unavailable.
	ok := h.reserveYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend)
	if !ok {
		return &streamerSendError{kind: sendErrQuota, msg: "youtube send blocked: daily quota exhausted"}
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
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := "https://www.googleapis.com/youtube/v3/liveChat/messages?part=snippet"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return classifyPlatformStatus("youtube", resp.StatusCode, string(body))
	}

	h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, true)
	return nil
}

// resolveYouTubeAccessToken resolves a valid YouTube access token for the streamer's
// active channel via the shared token source (youtube_oauth_tokens, refreshed in place),
// surfacing reauth_required when no credential is linked or the source is unconfigured.
// This replaces the old use of user.AccessToken, which for a non-YouTube-login streamer is
// a different platform's token and 401s against the YouTube Data API. A token expiring
// within 5 minutes is proactively refreshed so a foreseeable expiry doesn't cost the send.
func (h *ChatSendHandler) resolveYouTubeAccessToken(ctx context.Context, userID, channelID string) (string, error) {
	if h.ytTokenSource == nil {
		return "", &streamerSendError{kind: sendErrReauth, msg: "youtube token source not configured"}
	}
	cred, err := h.ytTokenSource.Resolve(ctx, userID, channelID)
	if err != nil {
		if errors.Is(err, youtubetoken.ErrNoCredential) {
			return "", &streamerSendError{kind: sendErrReauth, msg: "no YouTube credential linked for this channel; reconnect YouTube"}
		}
		return "", fmt.Errorf("resolve youtube credential: %w", err)
	}
	if !cred.ExpiresAt.IsZero() && time.Until(cred.ExpiresAt) < 5*time.Minute {
		if rerr := h.ytTokenSource.Refresh(ctx, cred); rerr != nil {
			h.log.Warn("proactive youtube token refresh failed; attempting with current token", zap.Error(rerr))
		}
	}
	return cred.AccessToken, nil
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
		return classifyPlatformStatus("kick", resp.StatusCode, string(body))
	}

	return nil
}

// streamerSendResult is one platform's outcome in a send-to-all response.
type streamerSendResult struct {
	Platform  string `json:"platform"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
}

// sendResultErrorKind classifies a per-platform send failure for the send-to-all response.
// A typed *streamerSendError carries its kind directly; an untyped error (e.g. YouTube
// live-chat-ID discovery failing) is run through the same text classifier the single-send
// path uses, so the monitor shows a meaningful kind (stream_offline, reauth_required)
// instead of a blanket send_failed.
func sendResultErrorKind(err error) sendErrKind {
	var se *streamerSendError
	if errors.As(err, &se) {
		return se.kind
	}
	switch status, _ := classifySendError(err.Error()); status {
	case http.StatusUnauthorized:
		return sendErrReauth
	case http.StatusUnprocessableEntity:
		return sendErrOffline
	default:
		return sendErrUpstream
	}
}

// sendAllAction is how the send-to-all dedup registration must be reconciled once the real
// per-platform outcomes are known. The combined pill is pre-registered with the full
// intended set before sending (so fast echoes are always recognised); afterwards it has to
// be corrected, because a platform echoes its copy back ONLY if its send actually succeeded.
type sendAllAction int

const (
	sendAllNoChange sendAllAction = iota // every intended platform succeeded; full set is correct
	sendAllRewrite                       // partial success with ≥2 winners; shrink the pill to them
	sendAllDelete                        // <2 winners; drop the group so the lone echo shows normally
)

// sendAllReconcile is the decision produced by decideSendAllPill: the action to apply and
// the platform set (the successes, in the caller's deterministic order) the pill should show.
type sendAllReconcile struct {
	action    sendAllAction
	platforms []string
}

// decideSendAllPill derives the reconcile decision from the per-platform results. The pill
// must list exactly the platforms whose send succeeded (and thus will echo back): all
// succeeded ⇒ leave the pre-registered set; ≥2 succeeded but some failed ⇒ rewrite to the
// winners; ≤1 succeeded ⇒ delete the group so the survivor renders as an ordinary message
// (the combined pill is only meaningful for >1 platform — see ChatRow's length>1 gate).
func decideSendAllPill(intended []string, results []streamerSendResult) sendAllReconcile {
	ok := make(map[string]bool, len(results))
	for _, r := range results {
		if r.Success {
			ok[r.Platform] = true
		}
	}
	success := make([]string, 0, len(intended))
	for _, p := range intended {
		if ok[p] {
			success = append(success, p)
		}
	}
	switch {
	case len(success) == len(intended):
		return sendAllReconcile{action: sendAllNoChange, platforms: success}
	case len(success) >= 2:
		return sendAllReconcile{action: sendAllRewrite, platforms: success}
	default:
		return sendAllReconcile{action: sendAllDelete, platforms: success}
	}
}

// handleStreamerSendToAll fans the message out to every platform the streamer is
// connected on (Twitch/YouTube/Kick — TikTok has no send API, Discord no streamer
// path), pre-registers the dedup group so the echoed-back copies collapse into one
// combined-pill message, and returns per-platform results. Partial success is normal
// (e.g. YouTube quota-blocked while Twitch + Kick succeed).
func (h *ChatSendHandler) handleStreamerSendToAll(c *gin.Context, ctx context.Context, user *models.User, message string) {
	// Candidate platforms: those the streamer has a platform identity on. senderID is
	// the platform-native id that appears as the author on the echoed-back message.
	idsByPlatform := map[string]string{}
	if user.TwitchID != nil && *user.TwitchID != "" {
		idsByPlatform["twitch"] = *user.TwitchID
	}
	if user.KickID != nil && *user.KickID != "" {
		idsByPlatform["kick"] = *user.KickID
	}
	if ytChannelID, ytErr := h.getActiveYouTubeChannelID(ctx, user.ID); ytErr == nil && ytChannelID != "" {
		idsByPlatform["youtube"] = ytChannelID
	}

	if len(idsByPlatform) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no_sendable_platform", "details": "No connected platform is configured for sending."})
		return
	}

	// Rate-limit per platform (the plan's send-to-all accounting): N platforms = N units.
	if ok, retryAfter := h.reserveStreamerRate(ctx, user.ID, len(idsByPlatform)); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "retry_after_seconds": retryAfter})
		return
	}

	// Deterministic platform order so the combined pill / primary platform is stable.
	platforms := make([]string, 0, len(idsByPlatform))
	for _, p := range []string{"twitch", "youtube", "kick"} {
		if _, ok := idsByPlatform[p]; ok {
			platforms = append(platforms, p)
		}
	}

	// Pre-register the echoes (only meaningful for ≥2 targets) BEFORE sending, so the
	// message-processor recognises each platform's echo and collapses them into one. The
	// set is provisional: it lists every intended platform, and is reconciled below to the
	// platforms that actually succeed (a platform echoes back only if its send did).
	groupID := ""
	if len(platforms) >= 2 {
		groupID = uuid.NewString()
		h.writeSendAllKeys(ctx, idsByPlatform, message, platforms, groupID)
	}

	results := make([]streamerSendResult, 0, len(platforms))
	anySuccess := false
	for _, platform := range platforms {
		var sendErr error
		switch platform {
		case "twitch":
			sendErr = h.sendStreamerTwitchMessage(ctx, user, message)
		case "kick":
			sendErr = h.sendStreamerKickMessage(ctx, user, message)
		case "youtube":
			sendErr = h.sendStreamerYouTubeMessage(ctx, user, message)
		}
		if sendErr != nil {
			kind := sendResultErrorKind(sendErr)
			results = append(results, streamerSendResult{Platform: platform, Success: false, Error: string(kind), ErrorKind: string(kind)})
			h.log.Warn("send-to-all platform failed", zap.String("platform", platform), zap.String("error_kind", string(kind)), zap.Error(sendErr))
		} else {
			anySuccess = true
			results = append(results, streamerSendResult{Platform: platform, Success: true})
		}
	}

	// Reconcile the dedup registration with reality so the combined pill lists ONLY the
	// platforms that actually received the message. Without this, a platform that failed
	// (e.g. YouTube not live / quota-blocked) would still render in the pill, because the
	// set was pre-registered before any outcome was known.
	if groupID != "" {
		switch d := decideSendAllPill(platforms, results); d.action {
		case sendAllRewrite:
			h.writeSendAllKeys(ctx, idsByPlatform, message, d.platforms, groupID)
		case sendAllDelete:
			h.deleteSendAllKeys(ctx, idsByPlatform, message)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": anySuccess, "results": results})
}

// writeSendAllKeys (re)writes the send-to-all dedup keys for a message so the collapsed
// combined pill lists exactly includePlatforms, all sharing one group id. For every
// candidate platform identity in idsByPlatform: if it is in includePlatforms its key is
// SET (group id + the includePlatforms set); otherwise its key is DELETED — a platform
// that did not receive the message must never appear in the pill and will never echo.
// Used twice per send-to-all: first with the full intended set (pre-register, so fast
// echoes are recognised), then with the success set (reconcile). Best-effort: a Redis
// error only degrades dedup for that one platform. See shared/sendall.
func (h *ChatSendHandler) writeSendAllKeys(ctx context.Context, idsByPlatform map[string]string, message string, includePlatforms []string, groupID string) {
	if h.redisClient == nil {
		return
	}
	include := make(map[string]bool, len(includePlatforms))
	for _, p := range includePlatforms {
		include[p] = true
	}
	payload, err := json.Marshal(sendall.Registration{GroupID: groupID, Platforms: includePlatforms})
	if err != nil {
		h.log.Warn("send-to-all: failed to marshal registration", zap.Error(err))
		return
	}
	for platform, senderID := range idsByPlatform {
		if senderID == "" {
			continue
		}
		key := sendall.Key(platform, senderID, message)
		if include[platform] {
			if err := h.redisClient.Set(ctx, key, payload, sendall.TTL).Err(); err != nil {
				h.log.Warn("send-to-all: failed to write echo key", zap.String("platform", platform), zap.Error(err))
			}
		} else if err := h.redisClient.Del(ctx, key).Err(); err != nil {
			h.log.Warn("send-to-all: failed to drop echo key", zap.String("platform", platform), zap.Error(err))
		}
	}
}

// deleteSendAllKeys removes every send-to-all dedup key for this message across the given
// platform identities. Used when fewer than two platforms ultimately succeeded, so there is
// no combined pill to render: the single surviving echo (if any) then flows through as an
// ordinary single-platform message rather than a one-glyph "combined" pill. Best-effort.
func (h *ChatSendHandler) deleteSendAllKeys(ctx context.Context, idsByPlatform map[string]string, message string) {
	if h.redisClient == nil {
		return
	}
	for platform, senderID := range idsByPlatform {
		if senderID == "" {
			continue
		}
		if err := h.redisClient.Del(ctx, sendall.Key(platform, senderID, message)).Err(); err != nil {
			h.log.Warn("send-to-all: failed to clear echo key", zap.String("platform", platform), zap.Error(err))
		}
	}
}

// reserveStreamerRate enforces the streamer's send rate limits (rateLimit1Min per
// minute, rateLimit1Hour per hour) using fixed Redis windows, charging n units (a
// send-to-all charges one per target platform). Returns (allowed, retryAfterSeconds).
// Fails open if Redis is unavailable — rate limiting is a safeguard, not a gate.
func (h *ChatSendHandler) reserveStreamerRate(ctx context.Context, userID string, n int) (bool, int) {
	if h.redisClient == nil || n <= 0 {
		return true, 0
	}
	check := func(key string, limit int, window time.Duration) (bool, int) {
		val, err := h.redisClient.IncrBy(ctx, key, int64(n)).Result()
		if err != nil {
			return true, 0 // fail open
		}
		if val == int64(n) {
			h.redisClient.Expire(ctx, key, window)
		}
		if val > int64(limit) {
			ttl, _ := h.redisClient.TTL(ctx, key).Result()
			secs := int(ttl.Seconds())
			if secs < 1 {
				secs = 1
			}
			return false, secs
		}
		return true, 0
	}
	if ok, retry := check("streamer_send_rl:min:"+userID, rateLimit1Min, time.Minute); !ok {
		return false, retry
	}
	if ok, retry := check("streamer_send_rl:hr:"+userID, rateLimit1Hour, time.Hour); !ok {
		return false, retry
	}
	return true, 0
}

// reserveYouTubeSendQuota reserves units for a send against the shared
// youtube_quota_usage table (ADR-0006). Returns false only when the daily quota is
// genuinely exhausted (the send must be BLOCKED). FAILS OPEN (returns true) when
// accounting is not configured or the DB errors — a quota hiccup must never block a
// streamer's own chat, and a send is cheap vs. the daily limit.
func (h *ChatSendHandler) reserveYouTubeSendQuota(ctx context.Context, units int) bool {
	if h.ytQuota == nil {
		return true
	}
	ok, err := h.ytQuota.Reserve(ctx, units)
	if err != nil {
		h.log.Warn("youtube quota reserve failed; allowing send", zap.Error(err))
		return true
	}
	return ok
}

// settleYouTubeSendQuota confirms (success=true) or rolls back (success=false) a prior
// reservation. Best-effort: a settle failure only skews the quota counter slightly.
func (h *ChatSendHandler) settleYouTubeSendQuota(ctx context.Context, units int, success bool) {
	if h.ytQuota == nil {
		return
	}
	var err error
	if success {
		err = h.ytQuota.Confirm(ctx, units)
	} else {
		err = h.ytQuota.Rollback(ctx, units)
	}
	if err != nil {
		h.log.Warn("youtube quota settle failed", zap.Bool("success", success), zap.Error(err))
	}
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

// getActiveYouTubeChannelID gets the channel ID of the active YouTube source for a streamer.
// This is retrieved from overlay_chat_sources rather than users.google_id because the
// active channel may change when streamers switch between different YouTube channels (and a
// streamer whose All-Chat login is not YouTube has no google_id at all).
//
// It deliberately does NOT gate on overlays.is_public_for_viewers: chat send is now only
// driven by the streamer's own monitor view, where they send to their own live chat, so
// whether the overlay is shared with viewers is irrelevant. Requiring public previously made
// a streamer with a private overlay get the misleading "The streamer has not configured this
// platform in All-Chat." error despite YouTube being configured.
func (h *ChatSendHandler) getActiveYouTubeChannelID(ctx context.Context, streamerUserID string) (string, error) {
	query := `
		SELECT ocs.channel_id
		FROM overlay_chat_sources ocs
		INNER JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.user_id = $1
		  AND ocs.platform = 'youtube'
		  AND ocs.is_active = true
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
