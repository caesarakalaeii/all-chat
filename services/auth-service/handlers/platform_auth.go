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
	providers   map[oauth.Platform]oauth.OAuthProvider
	userRepo    *repository.UserRepository
	redis       *redis.Client
	jwtSecret   string
	jwtExpiry   time.Duration
	logger      *zap.Logger
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

		// Handle PKCE for Kick
		var authURL string
		if platform == oauth.PlatformKick {
			kickProvider, ok := provider.(*oauth.KickOAuth)
			if !ok {
				h.logger.Error("Kick provider type assertion failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}

			// Generate auth URL with PKCE
			var codeVerifier string
			authURL, codeVerifier = kickProvider.GetAuthURLWithPKCE(state)

			// Store code verifier in Redis for later use during token exchange
			verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, state)
			err = h.redis.Set(c.Request.Context(), verifierKey, codeVerifier, 10*time.Minute).Err()
			if err != nil {
				h.logger.Error("Failed to store code verifier", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}
		} else {
			authURL = provider.GetAuthURL(state)
		}

		// Store state in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, state)
		err = h.redis.Set(c.Request.Context(), stateKey, "1", 10*time.Minute).Err()
		if err != nil {
			h.logger.Error("Failed to store state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

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

		// Exchange code for token (handle PKCE for Kick)
		var token *oauth2.Token
		if platform == oauth.PlatformKick {
			kickProvider, ok := provider.(*oauth.KickOAuth)
			if !ok {
				h.logger.Error("Kick provider type assertion failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}

			// Retrieve code verifier from Redis
			verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, state)
			codeVerifier, err := h.redis.Get(c.Request.Context(), verifierKey).Result()
			if err != nil {
				h.logger.Error("Failed to retrieve code verifier",
					zap.String("platform", string(platform)),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve PKCE verifier"})
				return
			}

			// Delete used verifier
			h.redis.Del(c.Request.Context(), verifierKey)

			// Exchange code with PKCE verifier
			token, err = kickProvider.ExchangeCodeWithPKCE(c.Request.Context(), code, codeVerifier)
			if err != nil {
				h.logger.Error("Failed to exchange code with PKCE",
					zap.String("platform", string(platform)),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
				return
			}
		} else {
			var err error
			token, err = provider.ExchangeCode(c.Request.Context(), code)
			if err != nil {
				h.logger.Error("Failed to exchange code",
					zap.String("platform", string(platform)),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
				return
			}
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

		if platform == oauth.PlatformYouTube {
			youtubeProvider, ok := provider.(*oauth.YouTubeOAuth)
			if !ok {
				h.logger.Error("YouTube provider assertion failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "YouTube provider misconfigured"})
				return
			}

			channelInfo, channelErr := youtubeProvider.GetPrimaryChannel(c.Request.Context(), token.AccessToken)
			if channelErr != nil {
				h.logger.Error("Failed to resolve YouTube channel", zap.Error(channelErr))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve YouTube channel"})
				return
			}

			if err := h.userRepo.StoreYouTubeToken(c.Request.Context(), user.ID, channelInfo.ChannelID, token); err != nil {
				h.logger.Warn("Failed to store YouTube tokens for listener",
					zap.String("user_id", user.ID),
					zap.String("channel_id", channelInfo.ChannelID),
					zap.Error(err),
				)
			}
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
	case oauth.PlatformKick:
		user, err = h.userRepo.GetByKickID(ctx, platformUser.GetID())
	case oauth.PlatformTikTok:
		user, err = h.userRepo.GetByTikTokID(ctx, platformUser.GetID())
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
		case oauth.PlatformKick:
			user.KickID = &platformID
		case oauth.PlatformTikTok:
			user.TikTokOpenID = &platformID
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

	return user, nil
}
