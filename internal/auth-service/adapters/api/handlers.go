package api

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/internal/auth-service/core/ports"
	"github.com/caesar/all-chat/internal/auth-service/core/services"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authService ports.AuthService
	redisClient *redis.Client
}

func NewAuthHandler(authService ports.AuthService, redisClient *redis.Client) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		redisClient: redisClient,
	}
}

func (h *AuthHandler) RegisterRoutes(router *gin.Engine) {
	auth := router.Group("/api/v1/auth")
	{
		auth.GET("/login", h.Login)
		auth.GET("/callback", h.Callback)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/logout", h.Logout)
		auth.GET("/me", h.GetMe)
	}
}

// Login redirects to Twitch OAuth
func (h *AuthHandler) Login(c *gin.Context) {
	state, err := services.GenerateStateToken()
	if err != nil {
		logger.Error("Failed to generate state token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state token"})
		return
	}

	// Store state in Redis with 5-minute expiration
	err = h.redisClient.Set(c.Request.Context(), "oauth_state:"+state, "1", 5*time.Minute).Err()
	if err != nil {
		logger.Error("Failed to store state in Redis", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate login"})
		return
	}

	authURL := h.authService.GetAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles OAuth callback from Twitch
func (h *AuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state
	result, err := h.redisClient.GetDel(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil || result != "1" {
		logger.Warn("Invalid OAuth state", zap.String("state", state))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state parameter"})
		return
	}

	// Exchange code for tokens
	user, tokenPair, err := h.authService.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		logger.Error("Failed to exchange code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate"})
		return
	}

	logger.Info("User logged in", zap.String("user_id", user.ID), zap.String("username", user.Username))

	// In a real application, you might redirect to frontend with tokens
	// For now, we'll return JSON
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"twitch_id":    user.TwitchID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"avatar_url":   user.AvatarURL,
		},
		"tokens": tokenPair,
	})
}

// RefreshToken generates a new access token from a refresh token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tokenPair, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokenPair})
}

// Logout invalidates the user's session
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	err := h.authService.Logout(c.Request.Context(), userID.(string))
	if err != nil {
		logger.Error("Failed to logout", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetMe returns the current user's information
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	user, err := h.authService.GetUserInfo(c.Request.Context(), userID.(string))
	if err != nil {
		logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           user.ID,
			"twitch_id":    user.TwitchID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"avatar_url":   user.AvatarURL,
		},
	})
}
