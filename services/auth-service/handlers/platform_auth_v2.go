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
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// PlatformAuthHandlerV2 handles authentication for any OAuth platform with enhanced state management
type PlatformAuthHandlerV2 struct {
	providers         map[oauth.Platform]oauth.OAuthProvider
	userRepo          *repository.UserRepository
	redis             *redis.Client
	jwtSecret         string
	jwtExpiry         time.Duration
	logger            *zap.Logger
	frontendURL       string
	overlayManagerURL string
}

// NewPlatformAuthHandlerV2 creates a new platform auth handler v2
func NewPlatformAuthHandlerV2(
	providers map[oauth.Platform]oauth.OAuthProvider,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	jwtSecret string,
	jwtExpiryHours int,
	frontendURL string,
	overlayManagerURL string,
	logger *zap.Logger,
) *PlatformAuthHandlerV2 {
	return &PlatformAuthHandlerV2{
		providers:         providers,
		userRepo:          userRepo,
		redis:             redisClient,
		jwtSecret:         jwtSecret,
		jwtExpiry:         time.Duration(jwtExpiryHours) * time.Hour,
		frontendURL:       frontendURL,
		overlayManagerURL: overlayManagerURL,
		logger:            logger,
	}
}

// HandleLogin initiates the OAuth flow for user login
func (h *PlatformAuthHandlerV2) HandleLogin(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
			return
		}

		// Generate random CSRF token
		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Create login state
		oauthState := oauth.NewLoginState(csrfToken)
		if err := oauthState.Validate(); err != nil {
			h.logger.Error("Invalid OAuth state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		stateStr, err := oauthState.Encode()
		if err != nil {
			h.logger.Error("Failed to encode state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		authURL, err := h.generateAuthURL(c.Request.Context(), provider, platform, stateStr, csrfToken)
		if err != nil {
			h.logger.Error("Failed to generate auth URL", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Store state data in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		err = h.redis.Set(c.Request.Context(), stateKey, stateStr, 10*time.Minute).Err()
		if err != nil {
			h.logger.Error("Failed to store state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// HandleAddSource initiates the OAuth flow to add a source to an overlay
func (h *PlatformAuthHandlerV2) HandleAddSource(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
			return
		}

		overlayID := c.Param("overlay_id")
		if overlayID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id is required"})
			return
		}

		// Get user ID from JWT context (set by middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - please log in first"})
			return
		}

		// Generate random CSRF token
		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Create add-source state with current user_id for account linking
		oauthState := oauth.NewAddSourceState(csrfToken, overlayID, userID.(string))
		if err := oauthState.Validate(); err != nil {
			h.logger.Error("Invalid OAuth state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		stateStr, err := oauthState.Encode()
		if err != nil {
			h.logger.Error("Failed to encode state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		authURL, err := h.generateAuthURL(c.Request.Context(), provider, platform, stateStr, csrfToken)
		if err != nil {
			h.logger.Error("Failed to generate auth URL", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Store state data in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		err = h.redis.Set(c.Request.Context(), stateKey, stateStr, 10*time.Minute).Err()
		if err != nil {
			h.logger.Error("Failed to store state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		h.logger.Info("Generated add-source OAuth URL",
			zap.String("platform", string(platform)),
			zap.String("overlay_id", overlayID),
		)

		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// generateAuthURL generates the OAuth authorization URL with PKCE support for Kick
func (h *PlatformAuthHandlerV2) generateAuthURL(
	ctx context.Context,
	provider oauth.OAuthProvider,
	platform oauth.Platform,
	stateStr string,
	csrfToken string,
) (string, error) {
	var authURL string

	// Handle PKCE for Kick
	if platform == oauth.PlatformKick {
		kickProvider, ok := provider.(*oauth.KickOAuth)
		if !ok {
			return "", fmt.Errorf("kick provider type assertion failed")
		}

		// Generate auth URL with PKCE
		var codeVerifier string
		authURL, codeVerifier = kickProvider.GetAuthURLWithPKCE(stateStr)

		// Store code verifier in Redis for later use during token exchange
		verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, csrfToken)
		err := h.redis.Set(ctx, verifierKey, codeVerifier, 10*time.Minute).Err()
		if err != nil {
			return "", fmt.Errorf("failed to store code verifier: %w", err)
		}
	} else {
		authURL = provider.GetAuthURL(stateStr)
	}

	return authURL, nil
}

// HandleCallback handles the OAuth callback for any platform
func (h *PlatformAuthHandlerV2) HandleCallback(platform oauth.Platform) gin.HandlerFunc {
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

		// Decode state to get action and context
		oauthState, err := oauth.DecodeOAuthState(state)
		if err != nil {
			h.logger.Warn("Failed to decode state",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state format"})
			return
		}

		if err := oauthState.Validate(); err != nil {
			h.logger.Warn("Invalid state",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state"})
			return
		}

		// Verify state in Redis
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, oauthState.CSRFToken)
		storedState, err := h.redis.Get(c.Request.Context(), stateKey).Result()
		if err != nil {
			h.logger.Warn("Invalid or expired state",
				zap.String("platform", string(platform)),
				zap.String("csrf_token", oauthState.CSRFToken),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
			return
		}

		// Verify the stored state matches
		if storedState != state {
			h.logger.Warn("State mismatch",
				zap.String("platform", string(platform)),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "State mismatch"})
			return
		}

		// Delete used state
		h.redis.Del(c.Request.Context(), stateKey)

		// Exchange code for token (handle PKCE for Kick)
		token, err := h.exchangeCodeForToken(c.Request.Context(), provider, platform, code, oauthState.CSRFToken)
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

		var youtubeChannel *oauth.YouTubeChannelInfo
		var sourceDetails *OverlaySourceDetails

		if platform == oauth.PlatformYouTube {
			youtubeProvider, ok := provider.(*oauth.YouTubeOAuth)
			if !ok {
				h.logger.Error("YouTube provider assertion failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "YouTube provider misconfigured"})
				return
			}

			channelInfo, channelErr := youtubeProvider.GetPrimaryChannel(c.Request.Context(), token.AccessToken)
			if channelErr != nil {
				h.logger.Error("Failed to resolve YouTube channel",
					zap.Error(channelErr))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve YouTube channel"})
				return
			}
			youtubeChannel = channelInfo
			sourceDetails = &OverlaySourceDetails{
				ChannelID:   channelInfo.ChannelID,
				ChannelName: channelInfo.Title,
			}
		}

		if platform == oauth.PlatformTikTok {
			username := normalizeTikTokUsername(platformUser.GetUsername())
			if username == "" {
				h.logger.Error("TikTok username missing in OAuth profile")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "TikTok username unavailable"})
				return
			}
			sourceDetails = &OverlaySourceDetails{
				ChannelID:   username,
				ChannelName: platformUser.GetDisplayName(),
			}
		}

		// Get or create user based on platform and context
		var user *models.User
		var jwtToken string

		if oauthState.IsAddSource() && oauthState.UserID != "" {
			// Account linking: link new platform to existing user
			user, err = h.linkPlatformToUser(c.Request.Context(), oauthState.UserID, platform, platformUser, token)
			if err != nil {
				h.logger.Error("Failed to link platform to user",
					zap.String("platform", string(platform)),
					zap.String("user_id", oauthState.UserID),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link account"})
				return
			}

			// Generate JWT for the existing user
			jwtToken, err = auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry)
			if err != nil {
				h.logger.Error("Failed to generate JWT", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
				return
			}

			h.logger.Info("Linked platform to existing user",
				zap.String("platform", string(platform)),
				zap.String("user_id", user.ID),
				zap.String("platform_user_id", platformUser.GetID()),
			)
		} else {
			// Regular login: get or create user
			user, err = h.getOrCreateUser(c.Request.Context(), platform, platformUser, token)
			if err != nil {
				h.logger.Error("Failed to get or create user",
					zap.String("platform", string(platform)),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
				return
			}

			// Generate JWT
			jwtToken, err = auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry)
			if err != nil {
				h.logger.Error("Failed to generate JWT", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
				return
			}
		}

		// Handle different actions
		if oauthState.IsAddSource() {
			// Add source to overlay
			err = h.addSourceToOverlay(c.Request.Context(), user.ID, oauthState.OverlayID, platform, platformUser, sourceDetails, jwtToken)
			if err != nil {
				h.logger.Error("Failed to add source to overlay",
					zap.String("platform", string(platform)),
					zap.String("overlay_id", oauthState.OverlayID),
					zap.Error(err),
				)
				// Redirect with error
				redirectURL := fmt.Sprintf("%s/overlays/%s?error=failed_to_add_source",
					h.frontendURL,
					oauthState.OverlayID,
				)
				c.Redirect(http.StatusFound, redirectURL)
				return
			}

			// Redirect through auth callback to preserve authentication, then to overlay page
			// The frontend auth callback will store the JWT and then redirect to the overlay
			redirectURL := fmt.Sprintf("%s/auth/callback#access_token=%s&refresh_token=%s&expires_in=%d&token_type=Bearer&redirect_to=/overlays/%s&source_added=%s",
				h.frontendURL,
				jwtToken,
				token.RefreshToken,
				int64(h.jwtExpiry.Seconds()),
				oauthState.OverlayID,
				platform,
			)

			h.logger.Info("Source added successfully via OAuth",
				zap.String("platform", string(platform)),
				zap.String("user_id", user.ID),
				zap.String("overlay_id", oauthState.OverlayID),
			)

			c.Redirect(http.StatusFound, redirectURL)
		} else {
			// Regular login - redirect to auth callback with token
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

		if platform == oauth.PlatformYouTube && youtubeChannel != nil {
			if err := h.userRepo.StoreYouTubeToken(c.Request.Context(), user.ID, youtubeChannel.ChannelID, token); err != nil {
				h.logger.Warn("Failed to store YouTube tokens for listener",
					zap.String("user_id", user.ID),
					zap.String("channel_id", youtubeChannel.ChannelID),
					zap.Error(err),
				)
			}
		}
	}
}

// exchangeCodeForToken exchanges the authorization code for an access token
func (h *PlatformAuthHandlerV2) exchangeCodeForToken(
	ctx context.Context,
	provider oauth.OAuthProvider,
	platform oauth.Platform,
	code string,
	csrfToken string,
) (*oauth2.Token, error) {
	if platform == oauth.PlatformKick {
		kickProvider, ok := provider.(*oauth.KickOAuth)
		if !ok {
			return nil, fmt.Errorf("kick provider type assertion failed")
		}

		// Retrieve code verifier from Redis
		verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, csrfToken)
		codeVerifier, err := h.redis.Get(ctx, verifierKey).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve PKCE verifier: %w", err)
		}

		// Delete used verifier
		h.redis.Del(ctx, verifierKey)

		// Exchange code with PKCE verifier
		return kickProvider.ExchangeCodeWithPKCE(ctx, code, codeVerifier)
	}

	return provider.ExchangeCode(ctx, code)
}

// getOrCreateUser gets an existing user or creates a new one
func (h *PlatformAuthHandlerV2) getOrCreateUser(
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

// linkPlatformToUser links a new platform to an existing user account
func (h *PlatformAuthHandlerV2) linkPlatformToUser(
	ctx context.Context,
	userID string,
	platform oauth.Platform,
	platformUser oauth.PlatformUserInfo,
	token *oauth2.Token,
) (*models.User, error) {
	// Get the existing user
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update the user with the new platform ID and tokens
	platformID := platformUser.GetID()

	switch platform {
	case oauth.PlatformTwitch:
		user.TwitchID = &platformID
	case oauth.PlatformYouTube:
		user.GoogleID = &platformID
	case oauth.PlatformKick:
		user.KickID = &platformID
	case oauth.PlatformTikTok:
		user.TikTokOpenID = &platformID
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	// Update tokens (in case they're linking a platform they previously used)
	user.AccessToken = token.AccessToken
	user.RefreshToken = token.RefreshToken
	user.TokenExpiresAt = token.Expiry

	// Update user in database
	if err := h.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// addSourceToOverlay calls the overlay-manager internal API to add a source
type OverlaySourceDetails struct {
	ChannelID   string
	ChannelName string
}

func normalizeTikTokUsername(input string) string {
	username := strings.TrimSpace(strings.TrimSuffix(input, "/"))
	if username == "" {
		return ""
	}

	lower := strings.ToLower(username)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if idx := strings.LastIndex(lower, "/@"); idx != -1 {
			username = username[idx+2:]
		} else if idx := strings.LastIndex(lower, "/"); idx != -1 {
			username = username[idx+1:]
		}
	}

	username = strings.TrimPrefix(username, "@")
	return strings.ToLower(username)
}

func (h *PlatformAuthHandlerV2) addSourceToOverlay(
	ctx context.Context,
	userID string,
	overlayID string,
	platform oauth.Platform,
	platformUser oauth.PlatformUserInfo,
	details *OverlaySourceDetails,
	jwtToken string,
) error {
	// Prepare request body
	channelID := platformUser.GetID()
	channelName := platformUser.GetDisplayName()

	if details != nil {
		if details.ChannelID != "" {
			channelID = details.ChannelID
		}
		if details.ChannelName != "" {
			channelName = details.ChannelName
		}
	}

	if platform == oauth.PlatformTikTok && details == nil {
		if username := normalizeTikTokUsername(platformUser.GetUsername()); username != "" {
			channelID = username
		}
	}

	reqBody := map[string]interface{}{
		"platform":     string(platform),
		"channel_id":   channelID,
		"channel_name": channelName,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make internal API call to overlay-manager
	url := fmt.Sprintf("%s/internal/overlays/%s/sources/auto", h.overlayManagerURL, overlayID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call overlay-manager: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("overlay-manager returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
