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
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StringEncryptor defines the interface for string encryption
type StringEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// ViewerIdentityRepo abstracts the viewer identity repository operations used
// by ViewerAuthHandler. This interface enables unit testing without a real database.
type ViewerIdentityRepo interface {
	GetOrCreateViewerByPlatform(ctx context.Context, platform, platformUserID string) (uuid.UUID, error)
	LinkPlatformToViewer(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) error
	LinkViewerToUser(ctx context.Context, platform, platformUserID, userID string) error
	GetViewerIsPremium(ctx context.Context, viewerID uuid.UUID) (bool, error)
	GetLinkedPlatforms(ctx context.Context, viewerID uuid.UUID) ([]repository.LinkedPlatform, error)
	UnlinkPlatform(ctx context.Context, viewerID uuid.UUID, platform string) error
	MigratePlatformUserID(ctx context.Context, platform, oldPlatformUserID, newPlatformUserID string) error
}

// ViewerAuthHandler handles authentication for viewers who want to send messages
type ViewerAuthHandler struct {
	twitchProvider  *oauth.ViewerTwitchOAuth
	youtubeProvider *oauth.ViewerYouTubeOAuth
	kickProvider    *oauth.ViewerKickOAuth
	viewerRepo      *repository.ViewerRepository
	identityRepo    ViewerIdentityRepo
	userRepo        *repository.UserRepository
	redis           *redis.Client
	userKeyChain    *sharedAuth.KeyChain
	jwtExpiry       time.Duration
	logger          *zap.Logger
	frontendURL     string
	cipher          StringEncryptor
	metrics         *metrics.BusinessMetrics
}

// NewViewerAuthHandler creates a new viewer auth handler
func NewViewerAuthHandler(
	twitchProvider *oauth.ViewerTwitchOAuth,
	youtubeProvider *oauth.ViewerYouTubeOAuth,
	kickProvider *oauth.ViewerKickOAuth,
	viewerRepo *repository.ViewerRepository,
	identityRepo *repository.ViewerIdentityRepository,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	userKeyChain *sharedAuth.KeyChain,
	jwtExpiryHours int,
	frontendURL string,
	cipher StringEncryptor,
	logger *zap.Logger,
) *ViewerAuthHandler {
	return &ViewerAuthHandler{
		twitchProvider:  twitchProvider,
		youtubeProvider: youtubeProvider,
		kickProvider:    kickProvider,
		viewerRepo:      viewerRepo,
		identityRepo:    identityRepo,
		userRepo:        userRepo,
		redis:           redisClient,
		userKeyChain:    userKeyChain,
		jwtExpiry:       time.Duration(jwtExpiryHours) * time.Hour,
		frontendURL:     frontendURL,
		cipher:          cipher,
		logger:          logger,
	}
}

// WithMetrics attaches a BusinessMetrics instance for recording viewer registration events.
func (h *ViewerAuthHandler) WithMetrics(m *metrics.BusinessMetrics) *ViewerAuthHandler {
	h.metrics = m
	return h
}

const authCodeTTL = 60 * time.Second

// storeAuthCode saves a JWT under a random code in Redis and returns the code.
func (h *ViewerAuthHandler) storeAuthCode(ctx context.Context, jwtToken string) (string, error) {
	code := uuid.New().String()
	key := "auth_code:" + code
	if err := h.redis.Set(ctx, key, jwtToken, authCodeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

// StreamerAuthPayload carries the streamer's auth tokens from the OAuth
// callback redirect to the frontend via a short-lived single-use code,
// eliminating token exposure in the URL fragment (audit M1).
type StreamerAuthPayload struct {
	AccessToken       string `json:"access_token"`
	RefreshToken      string `json:"refresh_token"`
	ExpiresIn         int64  `json:"expires_in"`
	TokenType         string `json:"token_type"`
	RedirectTo        string `json:"redirect_to,omitempty"`
	SourceAdded       string `json:"source_added,omitempty"`
	ModerationEnabled string `json:"moderation_enabled,omitempty"`
	// User carries the authenticated streamer's profile for the UI (no tokens).
	// Added in H3 so /exchange returns the user without a separate /auth/me call.
	User *StreamerAuthUser `json:"user,omitempty"`
}

// StreamerAuthUser is the non-secret user profile returned by /exchange.
type StreamerAuthUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	IsAdmin     bool   `json:"is_admin"`
}

const streamerAuthCodeTTL = 60 * time.Second

// storeStreamerAuthCode saves a streamer auth payload as JSON under a random
// single-use code in Redis and returns the code. The frontend exchanges this
// code via POST /exchange to retrieve the tokens, avoiding token exposure in
// the URL fragment (audit M1).
func storeStreamerAuthCode(ctx context.Context, rdb *redis.Client, payload StreamerAuthPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal streamer auth payload: %w", err)
	}
	code := uuid.New().String()
	key := "streamer_auth_code:" + code
	if err := rdb.Set(ctx, key, data, streamerAuthCodeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

// HandleTokenExchange swaps a short-lived auth code for the JWT token.
// POST /viewer/token/exchange { "code": "..." }
func (h *ViewerAuthHandler) HandleTokenExchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	key := "auth_code:" + req.Code
	token, err := h.redis.GetDel(c.Request.Context(), key).Result()
	if err == redis.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
		return
	}
	if err != nil {
		h.logger.Error("Failed to retrieve auth code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// findLinkedStreamer returns the streamer (dashboard user) that shares the given platform identity,
// or nil if no such account exists or the lookup fails.
func (h *ViewerAuthHandler) findLinkedStreamer(ctx context.Context, platform, platformUserID string) *models.User {
	var (
		user *models.User
		err  error
	)
	switch platform {
	case "twitch":
		user, err = h.userRepo.GetByTwitchID(ctx, platformUserID)
	case "youtube":
		user, err = h.userRepo.GetByGoogleID(ctx, platformUserID)
	case "kick":
		user, err = h.userRepo.GetByKickID(ctx, platformUserID)
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	return user
}

// HandleTwitchLogin initiates the OAuth flow for viewers on Twitch
func (h *ViewerAuthHandler) HandleTwitchLogin(c *gin.Context) {
	// Get streamer username from query parameter (optional, for context)
	streamerUsername := c.Query("streamer")

	// Generate random state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Store state in Redis with viewer prefix and optional streamer context
	stateKey := fmt.Sprintf("viewer_oauth_state:twitch:%s", state)
	stateData := map[string]string{
		"platform": "twitch",
	}
	if streamerUsername != "" {
		stateData["streamer"] = streamerUsername
	}
	if redirectTo := sanitizeRedirectPath(c.Query("redirect_to")); redirectTo != "" {
		stateData["redirect_to"] = redirectTo
	}
	// link_viewer_id: when set, the new platform will be linked to the existing
	// viewer instead of creating a fresh identity.
	if linkViewerID := c.Query("link_viewer_id"); linkViewerID != "" {
		stateData["link_viewer_id"] = linkViewerID
	}

	stateJSON, err := json.Marshal(stateData)
	if err != nil {
		h.logger.Error("Failed to marshal state data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	err = h.redis.Set(c.Request.Context(), stateKey, stateJSON, 10*time.Minute).Err()
	if err != nil {
		h.logger.Error("Failed to store state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Generate auth URL
	authURL := h.twitchProvider.GetAuthURL(state)

	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleTwitchCallback handles the OAuth callback for Twitch viewers
func (h *ViewerAuthHandler) HandleTwitchCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		h.redirectToFrontendWithError(c, "Missing code or state parameter")
		return
	}

	// Verify state (atomic Get+Del to prevent TOCTOU, audit L5)
	stateKey := fmt.Sprintf("viewer_oauth_state:twitch:%s", state)
	stateJSON, err := h.redis.GetDel(c.Request.Context(), stateKey).Result()
	if err != nil {
		h.logger.Warn("Invalid or expired state",
			zap.String("state", state),
			zap.Error(err),
		)
		h.redirectToFrontendWithError(c, "Invalid or expired state")
		return
	}

	// Parse state data
	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		h.logger.Error("Failed to parse state data", zap.Error(err))
		h.redirectToFrontendWithError(c, "Invalid state data")
		return
	}

	// Exchange code for token
	token, err := h.twitchProvider.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("Failed to exchange code",
			zap.Error(err),
		)
		h.redirectToFrontendWithError(c, "Failed to exchange code")
		return
	}

	// Get user info
	twitchUser, err := h.twitchProvider.GetUserInfoTwitch(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info",
			zap.Error(err),
		)
		h.redirectToFrontendWithError(c, "Failed to get user info")
		return
	}

	// Encrypt tokens
	encryptedAccessToken, err := h.cipher.Encrypt(token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to encrypt access token", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to save session")
		return
	}

	var encryptedRefreshToken *string
	if token.RefreshToken != "" {
		encrypted, err := h.cipher.Encrypt(token.RefreshToken)
		if err != nil {
			h.logger.Error("Failed to encrypt refresh token", zap.Error(err))
			h.redirectToFrontendWithError(c, "Failed to save session")
			return
		}
		encryptedRefreshToken = &encrypted
	}

	// Get or create viewer session
	session, err := h.getOrCreateViewerSession(c.Request.Context(), "twitch", twitchUser, encryptedAccessToken, encryptedRefreshToken, token.Expiry)
	if err != nil {
		h.logger.Error("Failed to create viewer session", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to create session")
		return
	}

	// Check if a linked streamer account exists and is banned
	linkedStreamer := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)
	if linkedStreamer != nil && linkedStreamer.IsBanned {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", h.frontendURL))
		return
	}

	// Resolve the durable viewer identity.
	// If link_viewer_id is present in state, add this platform to the existing viewer
	// (multi-platform linking flow). Otherwise create/fetch independently.
	viewerID, err := h.resolveViewerID(c.Request.Context(), session.Platform, session.PlatformUserID, stateData)
	if err != nil {
		h.logger.Error("Failed to resolve viewer identity", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to resolve viewer identity")
		return
	}

	if viewerID != uuid.Nil && linkedStreamer != nil {
		// Link viewer session to user account and propagate premium/admin status
		// so the message enricher can read badges without extra lookups.
		if linkErr := h.identityRepo.LinkViewerToUser(
			c.Request.Context(), session.Platform, session.PlatformUserID,
			linkedStreamer.ID,
		); linkErr != nil {
			h.logger.Warn("Failed to link viewer to user account", zap.Error(linkErr))
		}
	}

	// Generate JWT for viewer
	jwtToken, err := h.generateViewerJWT(session, viewerID, linkedStreamer)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate token")
		return
	}

	// Store JWT under a short-lived auth code and redirect with the code

	authCode, err := h.storeAuthCode(c.Request.Context(), jwtToken)
	if err != nil {
		h.logger.Error("Failed to store auth code", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate auth code")
		return
	}

	redirectURL := fmt.Sprintf("%s/chat/auth-success?code=%s", h.frontendURL, authCode)
	if streamer, ok := stateData["streamer"]; ok && streamer != "" {
		redirectURL += fmt.Sprintf("&streamer=%s", streamer)
	}
	if redirectTo, ok := stateData["redirect_to"]; ok && redirectTo != "" {
		redirectURL += fmt.Sprintf("&redirect_to=%s", redirectTo)
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// HandleMe returns the current viewer's information
func (h *ViewerAuthHandler) HandleMe(c *gin.Context) {
	// Get session ID from JWT claims (set by middleware as "session_id")
	sessionIDStr, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse UUID from string
	sessionID, err := uuid.Parse(sessionIDStr.(string))
	if err != nil {
		h.logger.Error("Invalid session ID format", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	// Get viewer session from database
	session, err := h.viewerRepo.GetByID(c.Request.Context(), sessionID)
	if err != nil {
		h.logger.Error("Failed to get viewer session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Return viewer info (without tokens)
	c.JSON(http.StatusOK, models.ViewerInfo{
		ID:          session.ID,
		Platform:    session.Platform,
		Username:    session.Username,
		DisplayName: session.DisplayName,
		AvatarURL:   session.AvatarURL,
	})
}

// HandleLogout deletes the viewer session
func (h *ViewerAuthHandler) HandleLogout(c *gin.Context) {
	// Get session ID from JWT claims
	sessionIDStr, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse UUID from string
	sessionID, err := uuid.Parse(sessionIDStr.(string))
	if err != nil {
		h.logger.Error("Invalid session ID format", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	// Delete viewer session
	err = h.viewerRepo.Delete(c.Request.Context(), sessionID)
	if err != nil {
		h.logger.Error("Failed to delete viewer session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// getOrCreateViewerSession gets or creates a viewer session
func (h *ViewerAuthHandler) getOrCreateViewerSession(
	ctx context.Context,
	platform string,
	twitchUser *models.TwitchUserInfo,
	accessToken string,
	refreshToken *string,
	tokenExpiry time.Time,
) (*models.ViewerSession, error) {
	// Check if session already exists
	existing, err := h.viewerRepo.GetByPlatformUserID(ctx, platform, twitchUser.ID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Update existing session
		existing.Username = twitchUser.Login
		existing.DisplayName = twitchUser.DisplayName
		existing.AvatarURL = &twitchUser.ProfileImageURL
		existing.AccessToken = accessToken
		existing.RefreshToken = refreshToken
		existing.TokenExpiresAt = tokenExpiry

		err = h.viewerRepo.Update(ctx, existing)
		if err != nil {
			return nil, err
		}

		return existing, nil
	}

	// Create new session
	session := &models.ViewerSession{
		Platform:       platform,
		PlatformUserID: twitchUser.ID,
		Username:       twitchUser.Login,
		DisplayName:    twitchUser.DisplayName,
		AvatarURL:      &twitchUser.ProfileImageURL,
		AccessToken:    accessToken,
		RefreshToken:   refreshToken,
		TokenExpiresAt: tokenExpiry,
	}

	err = h.viewerRepo.Create(ctx, session)
	if err != nil {
		return nil, err
	}

	if h.metrics != nil {
		h.metrics.RecordViewerRegistration(platform)
	}

	return session, nil
}

// generateViewerJWT generates a JWT token for a viewer session.
// viewerID is the durable viewer UUID from the viewers table.
// Pass uuid.Nil for pre-Phase-28 compatibility; the viewer_id claim will be an empty string.
// linkedStreamer is the streamer account sharing the same platform identity, or nil if none exists.
func (h *ViewerAuthHandler) generateViewerJWT(session *models.ViewerSession, viewerID uuid.UUID, linkedStreamer *models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(h.jwtExpiry)

	var viewerIDStr string
	if viewerID != uuid.Nil {
		viewerIDStr = viewerID.String()
	}

	var avatarURL string
	if session.AvatarURL != nil {
		avatarURL = *session.AvatarURL
	}

	// Look up viewer-level premium flag.
	// Soft failure: if DB is unavailable, default to false (safe degradation).
	var isPremium bool
	if viewerID != uuid.Nil && h.identityRepo != nil {
		var premErr error
		isPremium, premErr = h.identityRepo.GetViewerIsPremium(context.Background(), viewerID)
		if premErr != nil {
			h.logger.Warn("generateViewerJWT: failed to query is_premium, defaulting to false",
				zap.String("viewer_id", viewerIDStr),
				zap.Error(premErr),
			)
		}
	}

	// Inherit premium and admin from linked streamer account so users don't
	// have to purchase premium twice and admins have full access as viewers.
	var isAdmin bool
	if linkedStreamer != nil {
		isPremium = isPremium || linkedStreamer.IsPremium
		isAdmin = linkedStreamer.IsAdmin
	}

	claims := sharedAuth.ViewerClaims{
		ViewerID:       viewerIDStr,
		SessionID:      session.ID.String(),
		Platform:       session.Platform,
		PlatformUserID: session.PlatformUserID,
		Username:       session.Username,
		DisplayName:    session.DisplayName,
		AvatarURL:      avatarURL,
		IsViewer:       true,
		IsPremium:      isPremium,
		IsAdmin:        isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "all-chat",
		},
	}

	return sharedAuth.GenerateViewerJWTWithKid(h.userKeyChain.LatestKid(), claims, string(h.userKeyChain.LatestSecret()))
}

// redirectToFrontendWithError redirects to frontend with error message
func (h *ViewerAuthHandler) redirectToFrontendWithError(c *gin.Context, errorMsg string) {
	redirectURL := fmt.Sprintf("%s/chat/auth-error?error=%s", h.frontendURL, errorMsg)
	c.Redirect(http.StatusFound, redirectURL)
}

// HandleYouTubeLogin initiates the OAuth flow for viewers on YouTube
func (h *ViewerAuthHandler) HandleYouTubeLogin(c *gin.Context) {
	streamer := c.Query("streamer")

	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login"})
		return
	}
	stateData := map[string]string{
		"type": "viewer_youtube",
	}
	if streamer != "" {
		stateData["streamer"] = streamer
	}
	if redirectTo := sanitizeRedirectPath(c.Query("redirect_to")); redirectTo != "" {
		stateData["redirect_to"] = redirectTo
	}
	if linkViewerID := c.Query("link_viewer_id"); linkViewerID != "" {
		stateData["link_viewer_id"] = linkViewerID
	}

	stateJSON, _ := json.Marshal(stateData)
	if err := h.redis.Set(c.Request.Context(), "oauth_state:"+state, stateJSON, 10*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to store OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login"})
		return
	}

	authURL := h.youtubeProvider.GetAuthURL(state)
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleYouTubeCallback handles the OAuth callback for YouTube viewers
func (h *ViewerAuthHandler) HandleYouTubeCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		h.redirectToFrontendWithError(c, "Missing code or state parameter")
		return
	}

	// Retrieve state data from Redis (atomic Get+Del to prevent TOCTOU, audit L5)
	stateData, err := h.redis.GetDel(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil {
		h.redirectToFrontendWithError(c, "Invalid or expired state")
		return
	}

	var storedState map[string]string
	if err := json.Unmarshal([]byte(stateData), &storedState); err != nil {
		h.redirectToFrontendWithError(c, "Invalid state data")
		return
	}

	// State was already deleted atomically by GetDel (audit L5).

	token, err := h.youtubeProvider.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to exchange code")
		return
	}

	userInfo, err := h.youtubeProvider.GetUserInfo(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to get user info")
		return
	}

	// Resolve the viewer's YouTube channel ID (UC... format).
	// This is the ID that InnerTube embeds as AuthorExternalChannelID on every chat message,
	// so it must be used as platform_user_id — not the Google account ID from /oauth2/v2/userinfo.
	channelID, channelErr := h.youtubeProvider.GetChannelID(c.Request.Context(), token.AccessToken)
	if channelErr != nil {
		h.logger.Warn("Failed to resolve YouTube channel ID; falling back to Google account ID",
			zap.String("google_id", userInfo.ID),
			zap.Error(channelErr),
		)
		// Fallback: use the Google account ID so auth still completes.
		// Viewer matching in the enricher will not work until a channel ID is obtained.
		channelID = userInfo.ID
	}

	// Try to find an existing session by the canonical channel ID first.
	// If none found, also check the legacy Google account ID (backwards-compat migration).
	session, err := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "youtube", channelID)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to get session")
		return
	}
	if session == nil && channelID != userInfo.ID {
		// No session with channel ID — check for a legacy session stored under the Google account ID.
		legacySession, legacyErr := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "youtube", userInfo.ID)
		if legacyErr != nil {
			h.redirectToFrontendWithError(c, "Failed to get session")
			return
		}
		if legacySession != nil {
			// Migrate the legacy session: update platform_user_id from Google ID to channel ID
			// in both viewer_sessions and viewer_platform_identities.
			if migErr := h.viewerRepo.MigratePlatformUserID(c.Request.Context(), "youtube", userInfo.ID, channelID); migErr != nil {
				h.logger.Error("Failed to migrate viewer_sessions to YouTube channel ID",
					zap.String("google_id", userInfo.ID),
					zap.String("channel_id", channelID),
					zap.Error(migErr),
				)
			} else {
				if migErr := h.identityRepo.MigratePlatformUserID(c.Request.Context(), "youtube", userInfo.ID, channelID); migErr != nil {
					h.logger.Error("Failed to migrate viewer_platform_identities to YouTube channel ID",
						zap.String("google_id", userInfo.ID),
						zap.String("channel_id", channelID),
						zap.Error(migErr),
					)
				}
				legacySession.PlatformUserID = channelID
			}
			session = legacySession
		}
	}

	encryptedAccess, _ := h.cipher.Encrypt(token.AccessToken)
	var encryptedRefresh *string
	if token.RefreshToken != "" {
		encrypted, _ := h.cipher.Encrypt(token.RefreshToken)
		encryptedRefresh = &encrypted
	}

	if session == nil {
		session = &models.ViewerSession{
			Platform:       "youtube",
			PlatformUserID: channelID,
			Username:       userInfo.Name,
			DisplayName:    userInfo.Name,
			AccessToken:    encryptedAccess,
			RefreshToken:   encryptedRefresh,
			TokenExpiresAt: token.Expiry,
		}

		if userInfo.Picture != "" {
			session.AvatarURL = &userInfo.Picture
		}

		if err := h.viewerRepo.Create(c.Request.Context(), session); err != nil {
			h.redirectToFrontendWithError(c, "Failed to create session")
			return
		}
		if h.metrics != nil {
			h.metrics.RecordViewerRegistration("youtube")
		}
	} else {
		session.Username = userInfo.Name
		session.DisplayName = userInfo.Name
		session.AccessToken = encryptedAccess
		session.RefreshToken = encryptedRefresh
		session.TokenExpiresAt = token.Expiry

		if userInfo.Picture != "" {
			session.AvatarURL = &userInfo.Picture
		}

		if err := h.viewerRepo.Update(c.Request.Context(), session); err != nil {
			h.redirectToFrontendWithError(c, "Failed to update session")
			return
		}
	}

	// Check if a linked streamer account exists and is banned
	linkedStreamerYT := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)
	if linkedStreamerYT != nil && linkedStreamerYT.IsBanned {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", h.frontendURL))
		return
	}

	// Resolve the durable viewer identity.
	viewerIDYT, err := h.resolveViewerID(c.Request.Context(), session.Platform, session.PlatformUserID, storedState)
	if err != nil {
		h.logger.Error("Failed to resolve viewer identity", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to resolve viewer identity")
		return
	}

	if viewerIDYT != uuid.Nil && linkedStreamerYT != nil {
		if linkErr := h.identityRepo.LinkViewerToUser(
			c.Request.Context(), session.Platform, session.PlatformUserID,
			linkedStreamerYT.ID,
		); linkErr != nil {
			h.logger.Warn("Failed to link viewer to user account", zap.Error(linkErr))
		}
	}

	jwtToken, err := h.generateViewerJWT(session, viewerIDYT, linkedStreamerYT)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to generate token")
		return
	}

	authCode, err := h.storeAuthCode(c.Request.Context(), jwtToken)
	if err != nil {
		h.logger.Error("Failed to store auth code", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate auth code")
		return
	}

	redirectURL := fmt.Sprintf("%s/chat/auth-success?code=%s", h.frontendURL, authCode)
	if streamer, ok := storedState["streamer"]; ok {
		redirectURL += fmt.Sprintf("&streamer=%s", streamer)
	}
	if redirectTo, ok := storedState["redirect_to"]; ok && redirectTo != "" {
		redirectURL += fmt.Sprintf("&redirect_to=%s", redirectTo)
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// HandleKickLogin initiates the OAuth flow for viewers on Kick
func (h *ViewerAuthHandler) HandleKickLogin(c *gin.Context) {
	streamer := c.Query("streamer")

	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login"})
		return
	}

	// Generate PKCE parameters
	authURL, codeVerifier := h.kickProvider.GetAuthURLWithPKCE(state)

	// Store state and code verifier in Redis
	stateData := map[string]string{
		"type":          "viewer_kick",
		"code_verifier": codeVerifier,
	}
	if streamer != "" {
		stateData["streamer"] = streamer
	}
	if redirectTo := sanitizeRedirectPath(c.Query("redirect_to")); redirectTo != "" {
		stateData["redirect_to"] = redirectTo
	}
	if linkViewerID := c.Query("link_viewer_id"); linkViewerID != "" {
		stateData["link_viewer_id"] = linkViewerID
	}

	stateJSON, _ := json.Marshal(stateData)
	if err := h.redis.Set(c.Request.Context(), "oauth_state:"+state, stateJSON, 10*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to store OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleKickCallback handles the OAuth callback for Kick viewers
func (h *ViewerAuthHandler) HandleKickCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		h.redirectToFrontendWithError(c, "Missing code or state parameter")
		return
	}

	// Retrieve state data from Redis (atomic Get+Del to prevent TOCTOU, audit L5)
	stateData, err := h.redis.GetDel(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil {
		h.redirectToFrontendWithError(c, "Invalid or expired state")
		return
	}

	var storedState map[string]string
	if err := json.Unmarshal([]byte(stateData), &storedState); err != nil {
		h.redirectToFrontendWithError(c, "Invalid state data")
		return
	}

	// State was already deleted atomically by GetDel (audit L5).

	// Get code verifier from stored state
	codeVerifier, ok := storedState["code_verifier"]
	if !ok {
		h.redirectToFrontendWithError(c, "Missing code verifier")
		return
	}

	// Exchange code for token using PKCE
	token, err := h.kickProvider.ExchangeCodeWithPKCE(c.Request.Context(), code, codeVerifier)
	if err != nil {
		h.logger.Error("Failed to exchange code", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to exchange code")
		return
	}

	// Get user info
	userInfo, err := h.kickProvider.GetUserInfoKick(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to get user info")
		return
	}

	// Check if session exists
	platformUserID := fmt.Sprintf("%d", userInfo.UserID)
	session, err := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "kick", platformUserID)
	if err != nil {
		h.logger.Error("Failed to get session", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to get session")
		return
	}

	// Encrypt tokens
	encryptedAccess, err := h.cipher.Encrypt(token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to encrypt access token", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to process authentication")
		return
	}

	var encryptedRefresh *string
	if token.RefreshToken != "" {
		encrypted, err := h.cipher.Encrypt(token.RefreshToken)
		if err != nil {
			h.logger.Error("Failed to encrypt refresh token", zap.Error(err))
			// Continue without refresh token
		} else {
			encryptedRefresh = &encrypted
		}
	}

	// Create or update session
	if session == nil {
		// Create new session
		session = &models.ViewerSession{
			Platform:       "kick",
			PlatformUserID: platformUserID,
			Username:       userInfo.Name,
			DisplayName:    userInfo.Name,
			AccessToken:    encryptedAccess,
			RefreshToken:   encryptedRefresh,
			TokenExpiresAt: token.Expiry,
		}

		if userInfo.ProfilePicture != "" {
			session.AvatarURL = &userInfo.ProfilePicture
		}

		if err := h.viewerRepo.Create(c.Request.Context(), session); err != nil {
			h.logger.Error("Failed to create session", zap.Error(err))
			h.redirectToFrontendWithError(c, "Failed to create session")
			return
		}
		if h.metrics != nil {
			h.metrics.RecordViewerRegistration("kick")
		}
	} else {
		// Update existing session
		session.Username = userInfo.Name
		session.DisplayName = userInfo.Name
		session.AccessToken = encryptedAccess
		session.RefreshToken = encryptedRefresh
		session.TokenExpiresAt = token.Expiry

		if userInfo.ProfilePicture != "" {
			session.AvatarURL = &userInfo.ProfilePicture
		}

		if err := h.viewerRepo.Update(c.Request.Context(), session); err != nil {
			h.logger.Error("Failed to update session", zap.Error(err))
			h.redirectToFrontendWithError(c, "Failed to update session")
			return
		}
	}

	// Check if a linked streamer account exists and is banned
	linkedStreamerKick := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)
	if linkedStreamerKick != nil && linkedStreamerKick.IsBanned {
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", h.frontendURL))
		return
	}

	// Resolve the durable viewer identity.
	viewerIDKick, err := h.resolveViewerID(c.Request.Context(), session.Platform, session.PlatformUserID, storedState)
	if err != nil {
		h.logger.Error("Failed to resolve viewer identity", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to resolve viewer identity")
		return
	}

	if viewerIDKick != uuid.Nil && linkedStreamerKick != nil {
		if linkErr := h.identityRepo.LinkViewerToUser(
			c.Request.Context(), session.Platform, session.PlatformUserID,
			linkedStreamerKick.ID,
		); linkErr != nil {
			h.logger.Warn("Failed to link viewer to user account", zap.Error(linkErr))
		}
	}

	// Generate JWT token
	jwtToken, err := h.generateViewerJWT(session, viewerIDKick, linkedStreamerKick)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate token")
		return
	}

	// Store JWT under short-lived code and redirect

	authCode, err := h.storeAuthCode(c.Request.Context(), jwtToken)
	if err != nil {
		h.logger.Error("Failed to store auth code", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate auth code")
		return
	}

	redirectURL := fmt.Sprintf("%s/chat/auth-success?code=%s", h.frontendURL, authCode)
	if streamer, ok := storedState["streamer"]; ok {
		redirectURL += fmt.Sprintf("&streamer=%s", streamer)
	}
	if redirectTo, ok := storedState["redirect_to"]; ok && redirectTo != "" {
		redirectURL += fmt.Sprintf("&redirect_to=%s", redirectTo)
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// resolveViewerID returns the durable viewer UUID for the given platform session.
//
// When stateData contains a "link_viewer_id" key the new platform identity is
// linked to that existing viewer (multi-platform connect flow).  Otherwise the
// standard GetOrCreateViewerByPlatform path is taken (initial sign-in flow).
//
// On error the caller should treat the viewer as anonymous (uuid.Nil) rather
// than aborting the auth entirely — cosmetics can fail gracefully.
func (h *ViewerAuthHandler) resolveViewerID(ctx context.Context, platform, platformUserID string, stateData map[string]string) (uuid.UUID, error) {
	linkViewerIDStr, hasLink := stateData["link_viewer_id"]

	if hasLink && linkViewerIDStr != "" {
		linkViewerID, err := uuid.Parse(linkViewerIDStr)
		if err != nil {
			h.logger.Warn("resolveViewerID: invalid link_viewer_id, falling back to create",
				zap.String("link_viewer_id", linkViewerIDStr),
			)
			// Bad UUID in state — fall through to normal flow rather than error.
		} else {
			linkErr := h.identityRepo.LinkPlatformToViewer(ctx, linkViewerID, platform, platformUserID)
			if linkErr == nil {
				return linkViewerID, nil
			}
			// ErrPlatformAlreadyLinked means the platform belongs to a different viewer.
			// Log and fall back to the normal GetOrCreate path so the user still gets a valid token.
			h.logger.Warn("resolveViewerID: LinkPlatformToViewer failed, falling back",
				zap.String("platform", platform),
				zap.String("platform_user_id", platformUserID),
				zap.Error(linkErr),
			)
		}
	}

	viewerID, err := h.identityRepo.GetOrCreateViewerByPlatform(ctx, platform, platformUserID)
	if err != nil {
		h.logger.Error("resolveViewerID: GetOrCreateViewerByPlatform failed", zap.Error(err))
		return uuid.Nil, err
	}
	return viewerID, nil
}

// HandleGetLinkedPlatforms returns all platforms linked to the authenticated viewer.
//
// GET /viewer/linked-platforms
// Requires viewer JWT. Returns:
//
//	{ "platforms": ["twitch", "youtube"] }
func (h *ViewerAuthHandler) HandleGetLinkedPlatforms(c *gin.Context) {
	viewerIDStr, exists := c.Get("viewer_id")
	if !exists || viewerIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	viewerID, err := uuid.Parse(viewerIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid viewer ID"})
		return
	}

	linked, err := h.identityRepo.GetLinkedPlatforms(c.Request.Context(), viewerID)
	if err != nil {
		h.logger.Error("Failed to get linked platforms", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	platforms := make([]string, 0, len(linked))
	for _, lp := range linked {
		platforms = append(platforms, lp.Platform)
	}

	c.JSON(http.StatusOK, gin.H{"platforms": platforms})
}

// HandleUnlinkPlatform removes a platform identity from the authenticated viewer.
//
// DELETE /viewer/linked-platforms/:platform
// Requires viewer JWT. The current JWT platform cannot be unlinked (you'd lose your session).
// Returns 409 if this is the last platform, 400 if trying to unlink the current JWT platform.
func (h *ViewerAuthHandler) HandleUnlinkPlatform(c *gin.Context) {
	viewerIDStr, exists := c.Get("viewer_id")
	if !exists || viewerIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	viewerID, err := uuid.Parse(viewerIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid viewer ID"})
		return
	}

	targetPlatform := c.Param("platform")
	if targetPlatform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Platform parameter required"})
		return
	}

	// Prevent unlinking the platform the viewer is currently authenticated with —
	// doing so would leave them with no way to re-authenticate this session.
	currentPlatform, _ := c.Get("platform")
	if currentPlatform != nil && currentPlatform.(string) == targetPlatform {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot unlink the platform you are currently signed in with"})
		return
	}

	if err := h.identityRepo.UnlinkPlatform(c.Request.Context(), viewerID, targetPlatform); err != nil {
		switch err {
		case repository.ErrLastPlatform:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case repository.ErrNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Platform not linked"})
		default:
			h.logger.Error("Failed to unlink platform", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Platform unlinked successfully"})
}

// sanitizeRedirectPath ensures redirect_to is a safe relative path (starts with /, no scheme).
// This prevents open redirect attacks.
func sanitizeRedirectPath(path string) string {
	if path == "" {
		return ""
	}
	// Must start with / and must not contain :// (no absolute URLs)
	if len(path) < 1 || path[0] != '/' || len(path) > 200 {
		return ""
	}
	for i := 0; i < len(path)-2; i++ {
		if path[i] == ':' && path[i+1] == '/' && path[i+2] == '/' {
			return ""
		}
	}
	return path
}
