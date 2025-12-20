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

// ViewerAuthHandler handles authentication for viewers who want to send messages
type ViewerAuthHandler struct {
	twitchProvider  *oauth.ViewerTwitchOAuth
	youtubeProvider *oauth.ViewerYouTubeOAuth
	viewerRepo      *repository.ViewerRepository
	redis           *redis.Client
	jwtSecret       string
	jwtExpiry       time.Duration
	logger          *zap.Logger
	frontendURL     string
	cipher          StringEncryptor
}

// NewViewerAuthHandler creates a new viewer auth handler
func NewViewerAuthHandler(
	twitchProvider *oauth.ViewerTwitchOAuth,
	youtubeProvider *oauth.ViewerYouTubeOAuth,
	viewerRepo *repository.ViewerRepository,
	redisClient *redis.Client,
	jwtSecret string,
	jwtExpiryHours int,
	frontendURL string,
	cipher StringEncryptor,
	logger *zap.Logger,
) *ViewerAuthHandler {
	return &ViewerAuthHandler{
		twitchProvider:  twitchProvider,
		youtubeProvider: youtubeProvider,
		viewerRepo:      viewerRepo,
		redis:           redisClient,
		jwtSecret:       jwtSecret,
		jwtExpiry:       time.Duration(jwtExpiryHours) * time.Hour,
		frontendURL:     frontendURL,
		cipher:          cipher,
		logger:          logger,
	}
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

	// Verify state
	stateKey := fmt.Sprintf("viewer_oauth_state:twitch:%s", state)
	stateJSON, err := h.redis.Get(c.Request.Context(), stateKey).Result()
	if err != nil {
		h.logger.Warn("Invalid or expired state",
			zap.String("state", state),
			zap.Error(err),
		)
		h.redirectToFrontendWithError(c, "Invalid or expired state")
		return
	}

	// Delete used state
	h.redis.Del(c.Request.Context(), stateKey)

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

	// Generate JWT for viewer
	jwtToken, err := h.generateViewerJWT(session)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		h.redirectToFrontendWithError(c, "Failed to generate token")
		return
	}

	// Redirect to frontend with JWT token and optional streamer context
	redirectURL := fmt.Sprintf("%s/chat/auth-success?token=%s", h.frontendURL, jwtToken)
	if streamer, ok := stateData["streamer"]; ok && streamer != "" {
		redirectURL += fmt.Sprintf("&streamer=%s", streamer)
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

	return session, nil
}

// generateViewerJWT generates a JWT token for a viewer session
func (h *ViewerAuthHandler) generateViewerJWT(session *models.ViewerSession) (string, error) {
	now := time.Now()
	expiresAt := now.Add(h.jwtExpiry)

	claims := sharedAuth.ViewerClaims{
		SessionID:      session.ID.String(),
		Platform:       session.Platform,
		PlatformUserID: session.PlatformUserID,
		Username:       session.Username,
		IsViewer:       true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "all-chat",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
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

	stateData, err := h.redis.Get(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil {
		h.redirectToFrontendWithError(c, "Invalid or expired state")
		return
	}

	var storedState map[string]string
	if err := json.Unmarshal([]byte(stateData), &storedState); err != nil {
		h.redirectToFrontendWithError(c, "Invalid state data")
		return
	}

	h.redis.Del(c.Request.Context(), "oauth_state:"+state)

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

	session, err := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "youtube", userInfo.ID)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to get session")
		return
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
			PlatformUserID: userInfo.ID,
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

	jwtToken, err := h.generateViewerJWT(session)
	if err != nil {
		h.redirectToFrontendWithError(c, "Failed to generate token")
		return
	}

	redirectURL := fmt.Sprintf("%s/chat/auth-success?token=%s", h.frontendURL, jwtToken)
	if streamer, ok := storedState["streamer"]; ok {
		redirectURL += fmt.Sprintf("&streamer=%s", streamer)
	}

	c.Redirect(http.StatusFound, redirectURL)
}
