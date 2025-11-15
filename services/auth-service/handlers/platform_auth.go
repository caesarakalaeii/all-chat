package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// PlatformAuthHandler handles authentication for any OAuth platform
type PlatformAuthHandler struct {
	providers  map[oauth.Platform]oauth.OAuthProvider
	userRepo   *repository.UserRepository
	redis      *redis.Client
	jwtSecret  string
	jwtExpiry  time.Duration
	logger     *zap.Logger
	frontendURL string
}

// NewPlatformAuthHandler creates a new platform auth handler
func NewPlatformAuthHandler(
	providers map[oauth.Platform]oauth.OAuthProvider,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	jwtSecret string,
	jwtExpiryHours int,
	frontendURL string,
	logger *zap.Logger,
) *PlatformAuthHandler {
	return &PlatformAuthHandler{
		providers:   providers,
		userRepo:    userRepo,
		redis:       redisClient,
		jwtSecret:   jwtSecret,
		jwtExpiry:   time.Duration(jwtExpiryHours) * time.Hour,
		frontendURL: frontendURL,
		logger:      logger,
	}
}

// HandleLogin initiates the OAuth flow for any platform
func (h *PlatformAuthHandler) HandleLogin(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
			return
		}

		// Generate random state for CSRF protection
		state, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Store state in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, state)
		err = h.redis.Set(c.Request.Context(), stateKey, "1", 10*time.Minute).Err()
		if err != nil {
			h.logger.Error("Failed to store state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		authURL := provider.GetAuthURL(state)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// HandleCallback handles the OAuth callback for any platform
func (h *PlatformAuthHandler) HandleCallback(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
			return
		}

		code := c.Query("code")
		state := c.Query("state")

		if code == "" || state == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
			return
		}

		// Verify state
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, state)
		_, err := h.redis.Get(c.Request.Context(), stateKey).Result()
		if err != nil {
			h.logger.Warn("Invalid or expired state",
				zap.String("platform", string(platform)),
				zap.String("state", state),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
			return
		}

		// Delete used state
		h.redis.Del(c.Request.Context(), stateKey)

		// Exchange code for token
		token, err := provider.ExchangeCode(c.Request.Context(), code)
		if err != nil {
			h.logger.Error("Failed to exchange code",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
			return
		}

		// Get user info
		platformUser, err := provider.GetUserInfo(c.Request.Context(), token.AccessToken)
		if err != nil {
			h.logger.Error("Failed to get user info",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		// Get or create user based on platform
		user, err := h.getOrCreateUser(c.Request.Context(), platform, platformUser, token)
		if err != nil {
			h.logger.Error("Failed to get or create user",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
			return
		}

		// Generate JWT
		jwtToken, err := auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry)
		if err != nil {
			h.logger.Error("Failed to generate JWT", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Redirect to frontend with token
		redirectURL := fmt.Sprintf("%s/auth/callback#access_token=%s&refresh_token=%s&expires_in=%d&token_type=Bearer",
			h.frontendURL,
			jwtToken,
			token.RefreshToken,
			int64(h.jwtExpiry.Seconds()),
		)

		h.logger.Info("User authenticated successfully",
			zap.String("platform", string(platform)),
			zap.String("user_id", user.ID),
			zap.String("username", user.Username),
		)

		c.Redirect(http.StatusFound, redirectURL)
	}
}

// getOrCreateUser gets an existing user or creates a new one
func (h *PlatformAuthHandler) getOrCreateUser(
	ctx context.Context,
	platform oauth.Platform,
	platformUser oauth.PlatformUserInfo,
	token *oauth2.Token,
) (*models.User, error) {
	var user *models.User
	var err error

	// Try to get existing user based on platform
	switch platform {
	case oauth.PlatformTwitch:
		user, err = h.userRepo.GetByTwitchID(ctx, platformUser.GetID())
	case oauth.PlatformYouTube:
		user, err = h.userRepo.GetByGoogleID(ctx, platformUser.GetID())
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	if err != nil {
		// User doesn't exist, create new one
		platformID := platformUser.GetID()
		user = &models.User{
			AuthProvider:    string(platform),
			Username:        platformUser.GetUsername(),
			DisplayName:     platformUser.GetDisplayName(),
			Email:           platformUser.GetEmail(),
			ProfileImageURL: platformUser.GetProfileImageURL(),
			AccessToken:     token.AccessToken,
			RefreshToken:    token.RefreshToken,
			TokenExpiresAt:  token.Expiry,
		}

		// Set the appropriate platform ID
		switch platform {
		case oauth.PlatformTwitch:
			user.TwitchID = &platformID
		case oauth.PlatformYouTube:
			user.GoogleID = &platformID
		}

		if err := h.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		h.logger.Info("Created new user",
			zap.String("platform", string(platform)),
			zap.String("user_id", user.ID),
			zap.String("username", user.Username),
		)
	} else {
		// Update existing user
		user.DisplayName = platformUser.GetDisplayName()
		user.Email = platformUser.GetEmail()
		user.ProfileImageURL = platformUser.GetProfileImageURL()
		user.AccessToken = token.AccessToken
		user.RefreshToken = token.RefreshToken
		user.TokenExpiresAt = token.Expiry

		if err := h.userRepo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// For YouTube, also store tokens in youtube_oauth_tokens table for listener
	if platform == oauth.PlatformYouTube {
		if err := h.userRepo.StoreYouTubeToken(ctx, user.ID, token); err != nil {
			h.logger.Warn("Failed to store YouTube tokens for listener",
				zap.String("user_id", user.ID),
				zap.Error(err),
			)
			// Don't fail the login
		}
	}

	return user, nil
}
