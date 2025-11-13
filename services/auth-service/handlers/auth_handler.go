package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	oauthClient *oauth.TwitchOAuth
	userRepo    *repository.UserRepository
	redis       *redis.Client
	jwtSecret   string
	jwtExpiry   time.Duration
	logger      *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	oauthClient *oauth.TwitchOAuth,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	jwtSecret string,
	jwtExpiryHours int,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		oauthClient: oauthClient,
		userRepo:    userRepo,
		redis:       redisClient,
		jwtSecret:   jwtSecret,
		jwtExpiry:   time.Duration(jwtExpiryHours) * time.Hour,
		logger:      logger,
	}
}

// HandleLogin initiates the OAuth flow
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	// Generate random state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Store state in Redis with 10 minute expiry
	err = h.redis.Set(c.Request.Context(), "oauth_state:"+state, "1", 10*time.Minute).Err()
	if err != nil {
		h.logger.Error("Failed to store state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	authURL := h.oauthClient.GetAuthURL(state)
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleCallback handles the OAuth callback
func (h *AuthHandler) HandleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state
	exists, err := h.redis.Get(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil || exists == "" {
		h.logger.Warn("Invalid or expired state", zap.String("state", state))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
		return
	}

	// Delete used state
	h.redis.Del(c.Request.Context(), "oauth_state:"+state)

	// Exchange code for token
	token, err := h.oauthClient.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("Failed to exchange code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info from Twitch
	twitchUser, err := h.oauthClient.GetUserInfo(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Check if user exists
	user, err := h.userRepo.GetByTwitchID(c.Request.Context(), twitchUser.ID)
	if err != nil {
		// Create new user
		user = &models.User{
			TwitchID:        twitchUser.ID,
			Username:        twitchUser.Login,
			DisplayName:     twitchUser.DisplayName,
			Email:           twitchUser.Email,
			ProfileImageURL: twitchUser.ProfileImageURL,
			AccessToken:     token.AccessToken,
			RefreshToken:    token.RefreshToken,
			TokenExpiresAt:  token.Expiry,
		}

		if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
			h.logger.Error("Failed to create user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	} else {
		// Update existing user
		user.Username = twitchUser.Login
		user.DisplayName = twitchUser.DisplayName
		user.Email = twitchUser.Email
		user.ProfileImageURL = twitchUser.ProfileImageURL
		user.AccessToken = token.AccessToken
		user.RefreshToken = token.RefreshToken
		user.TokenExpiresAt = token.Expiry

		if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
			h.logger.Error("Failed to update user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	// Generate JWT
	jwtToken, err := auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.TokenResponse{
		AccessToken:  jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    int64(h.jwtExpiry.Seconds()),
		TokenType:    "Bearer",
	})
}

// HandleRefresh refreshes an expired JWT using Twitch refresh token
func (h *AuthHandler) HandleRefresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Refresh OAuth token
	token, err := h.oauthClient.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh token", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to refresh token"})
		return
	}

	// Get user info to find user ID
	twitchUser, err := h.oauthClient.GetUserInfo(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Get user from database
	user, err := h.userRepo.GetByTwitchID(c.Request.Context(), twitchUser.ID)
	if err != nil {
		h.logger.Error("User not found", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Update tokens in database
	err = h.userRepo.UpdateTokens(c.Request.Context(), user.ID, token.AccessToken, token.RefreshToken, token.Expiry)
	if err != nil {
		h.logger.Error("Failed to update tokens", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tokens"})
		return
	}

	// Generate new JWT
	jwtToken, err := auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.TokenResponse{
		AccessToken:  jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    int64(h.jwtExpiry.Seconds()),
		TokenType:    "Bearer",
	})
}

// HandleGetMe returns current user info
func (h *AuthHandler) HandleGetMe(c *gin.Context) {
	// Extract user ID from JWT (set by middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(string))
	if err != nil {
		h.logger.Error("User not found", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// HandleLogout logs out the user
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
		return
	}

	// Add token to blacklist in Redis (expires after JWT expiry time)
	err := h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry).Err()
	if err != nil {
		h.logger.Error("Failed to blacklist token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
