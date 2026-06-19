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
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// ErrDuplicateAccount is returned when a new registration is blocked because
// an existing account already has a matching platform source configured.
type ErrDuplicateAccount struct {
	ExistingUsername string
	Platform         string
	Message          string
}

func (e *ErrDuplicateAccount) Error() string { return e.Message }

// PlatformAuthHandlerV2 handles authentication for any OAuth platform with enhanced state management
type PlatformAuthHandlerV2 struct {
	providers         map[oauth.Platform]oauth.OAuthProvider
	userRepo          *repository.UserRepository
	redis             *redis.Client
	userKeyChain      *auth.KeyChain
	jwtExpiry         time.Duration
	logger            *zap.Logger
	frontendURL       string
	overlayManagerURL string
	metrics           *metrics.BusinessMetrics
}

// NewPlatformAuthHandlerV2 creates a new platform auth handler v2
func NewPlatformAuthHandlerV2(
	providers map[oauth.Platform]oauth.OAuthProvider,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	userKeyChain *auth.KeyChain,
	jwtExpiryHours int,
	frontendURL string,
	overlayManagerURL string,
	logger *zap.Logger,
) *PlatformAuthHandlerV2 {
	return &PlatformAuthHandlerV2{
		providers:         providers,
		userRepo:          userRepo,
		redis:             redisClient,
		userKeyChain:      userKeyChain,
		jwtExpiry:         time.Duration(jwtExpiryHours) * time.Hour,
		frontendURL:       frontendURL,
		overlayManagerURL: overlayManagerURL,
		logger:            logger,
	}
}

// WithMetrics attaches a BusinessMetrics instance for recording user registration events.
func (h *PlatformAuthHandlerV2) WithMetrics(m *metrics.BusinessMetrics) *PlatformAuthHandlerV2 {
	h.metrics = m
	return h
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

		// Login keeps the minimal scope set (withChatScopes=false).
		authURL, err := h.generateAuthURL(c.Request.Context(), provider, platform, stateStr, csrfToken, false)
		if err != nil {
			h.logger.Error("Failed to generate auth URL", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Store state data in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		err = h.redis.Set(c.Request.Context(), stateKey, stateStr, 30*time.Minute).Err()
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

		userIDStr, ok := userID.(string)
		if !ok || userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			return
		}

		// Short-circuit Twitch add-source when the user already authenticated via Twitch
		// AND already granted the EventSub chat scopes. When the chat scope is missing we
		// intentionally fall through to the full OAuth flow below (withChatScopes=true) so
		// the streamer is prompted to grant it — that is what moves their channel from the
		// IRC listener to the EventSub listener.
		if platform == oauth.PlatformTwitch {
			user, err := h.userRepo.GetByID(c.Request.Context(), userIDStr)
			if err != nil {
				if errors.Is(err, repository.ErrUserNotFound) {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
					return
				}
				h.logger.Error("Failed to fetch user for Twitch add-source", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}

			// granted_scopes is not populated by GetByID (scanUser), so read it directly.
			// On error, treat as "no chat scope" and fall through to the OAuth flow.
			grantedScopes, scopeErr := h.userRepo.GetGrantedScopes(c.Request.Context(), userIDStr)
			if scopeErr != nil {
				h.logger.Warn("Failed to read granted scopes for Twitch add-source short-circuit",
					zap.String("user_id", userIDStr), zap.Error(scopeErr))
			}
			hasChatScope := containsScope(grantedScopes, "user:read:chat")

			if user.TwitchID != nil && *user.TwitchID != "" && user.AuthProvider == string(oauth.PlatformTwitch) && hasChatScope {
				// Check if token is expired and refresh if needed
				if time.Now().After(user.TokenExpiresAt) {
					h.logger.Info("Token expired, refreshing before adding source",
						zap.String("user_id", user.ID),
						zap.Time("expired_at", user.TokenExpiresAt),
					)

					// Refresh the token
					twitchProvider, ok := provider.(*oauth.TwitchOAuth)
					if !ok {
						h.logger.Error("Failed to get Twitch provider")
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
						return
					}

					newToken, err := twitchProvider.RefreshToken(c.Request.Context(), user.RefreshToken)
					if err != nil {
						h.logger.Error("Failed to refresh expired token",
							zap.String("user_id", user.ID),
							zap.Error(err),
						)
						// Token refresh failed - force OAuth flow instead of short-circuit
						c.JSON(http.StatusUnauthorized, gin.H{
							"error":         "OAuth token expired and refresh failed",
							"requires_auth": true,
						})
						return
					}

					// Update user with new token
					user.AccessToken = newToken.AccessToken
					user.RefreshToken = newToken.RefreshToken
					user.TokenExpiresAt = newToken.Expiry

					if err := h.userRepo.UpdateTokens(c.Request.Context(), user.ID, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry); err != nil {
						h.logger.Error("Failed to save refreshed token",
							zap.String("user_id", user.ID),
							zap.Error(err),
						)
						// Continue anyway - we have the token in memory
					}

					h.logger.Info("Successfully refreshed expired token",
						zap.String("user_id", user.ID),
						zap.Time("new_expiry", newToken.Expiry),
					)
				}

				authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
				jwtToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
				if jwtToken == "" {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
					return
				}

				platformUser := &oauth.TwitchUserInfoWrapper{
					ID:              *user.TwitchID,
					Login:           user.Username,
					DisplayName:     user.DisplayName,
					ProfileImageURL: user.ProfileImageURL,
				}

				if err := h.addSourceToOverlay(c.Request.Context(), user.ID, overlayID, platform, platformUser, nil, jwtToken); err != nil {
					h.logger.Error("Failed to add Twitch source via existing session",
						zap.String("user_id", user.ID),
						zap.String("overlay_id", overlayID),
						zap.Error(err),
					)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add source"})
					return
				}

				h.logger.Info("Added Twitch source without OAuth reflow",
					zap.String("user_id", user.ID),
					zap.String("overlay_id", overlayID),
				)

				c.JSON(http.StatusOK, gin.H{
					"source_added":                string(platform),
					"reused_existing_credentials": true,
				})
				return
			}
		}

		// Generate random CSRF token
		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Create add-source state with current user_id for account linking
		oauthState := oauth.NewAddSourceState(csrfToken, overlayID, userIDStr)
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

		// Add-source requests the EventSub chat scopes (withChatScopes=true) so a
		// streamer authorizing their own channel can be read via EventSub.
		authURL, err := h.generateAuthURL(c.Request.Context(), provider, platform, stateStr, csrfToken, true)
		if err != nil {
			h.logger.Error("Failed to generate auth URL", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Store state data in Redis with platform prefix
		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		err = h.redis.Set(c.Request.Context(), stateKey, stateStr, 30*time.Minute).Err()
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

// HandleEnableModeration initiates the opt-in moderation re-consent flow (ADR-0017).
// Unlike login/add-source it requests the platform moderation scopes for exactly the
// actions the streamer is enabling (?actions=delete,ban) — minimized to those actions —
// on TOP of the scopes already granted, so the issued token is a SUPERSET of the stored
// grant and never trips the downgrade guard. It reuses the add-source state + callback,
// so overlay linking and token/scope persistence (incl. the linked-token path for
// YouTube/Kick-login accounts) need no changes. Supports Twitch and Kick; Kick uses
// PKCE, so its consent URL also stashes a code verifier for the shared callback.
func (h *PlatformAuthHandlerV2) HandleEnableModeration(platform oauth.Platform) gin.HandlerFunc {
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

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - please log in first"})
			return
		}
		userIDStr, ok := userID.(string)
		if !ok || userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			return
		}

		// Minimal scopes for exactly the actions being enabled, mapped per platform.
		// Twitch splits delete vs ban/timeout/unban; Kick gates ban/timeout/unban behind
		// one scope and has no single-message delete. Unsupported platforms are rejected.
		actions := splitActions(c.Query("actions"))
		var modScopes []string
		switch provider.(type) {
		case *oauth.TwitchOAuth:
			modScopes = oauth.ModerationScopesForActions(actions)
		case *oauth.KickOAuth:
			modScopes = oauth.KickModerationScopesForActions(actions)
		case *oauth.YouTubeOAuth:
			modScopes = oauth.YouTubeModerationScopesForActions(actions)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("moderation is not supported for %s", platform)})
			return
		}
		if len(modScopes) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no valid moderation actions; expected ?actions=delete,timeout,ban,unban"})
			return
		}

		// Union with the scopes already granted FOR THIS PLATFORM so the new token is a
		// SUPERSET; the downgrade guard then preserves (never clobbers) the chat /
		// prior-mod grants. Reading platform-scoped (not the cross-platform users row) is
		// essential for LINKED accounts: a YouTube-login streamer enabling Twitch
		// moderation must not get their YouTube scopes injected into the Twitch consent
		// URL (the provider would reject the unknown scopes).
		existingScopes, scopeErr := h.userRepo.GetPlatformGrantedScopes(c.Request.Context(), userIDStr, string(platform))
		if scopeErr != nil {
			h.logger.Warn("Failed to read platform granted scopes for moderation re-consent; requesting action scopes only",
				zap.String("user_id", userIDStr), zap.String("platform", string(platform)), zap.Error(scopeErr))
		}
		extra := unionScopes(existingScopes, modScopes)

		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		oauthState := oauth.NewAddSourceState(csrfToken, overlayID, userIDStr)
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

		// Build the consent URL per platform. Kick uses PKCE, so its verifier is stored
		// in Redis under the same key the add-source callback reads — the re-consent
		// reuses the add-source state + callback for token/scope persistence.
		var authURL string
		switch p := provider.(type) {
		case *oauth.TwitchOAuth:
			authURL = p.GetAuthURLWithScopes(stateStr, extra)
		case *oauth.YouTubeOAuth:
			authURL = p.GetAuthURLWithScopes(stateStr, extra)
		case *oauth.KickOAuth:
			var codeVerifier string
			authURL, codeVerifier = p.GetAuthURLWithScopesPKCE(stateStr, extra)
			verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, csrfToken)
			if err := h.redis.Set(c.Request.Context(), verifierKey, codeVerifier, 10*time.Minute).Err(); err != nil {
				h.logger.Error("Failed to store PKCE verifier", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}
		}

		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		if err := h.redis.Set(c.Request.Context(), stateKey, stateStr, 30*time.Minute).Err(); err != nil {
			h.logger.Error("Failed to store state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		h.logger.Info("Generated moderation re-consent OAuth URL",
			zap.String("platform", string(platform)),
			zap.String("overlay_id", overlayID),
			zap.Strings("mod_scopes", modScopes),
		)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// splitActions parses a comma-separated ?actions= value into trimmed, non-empty items.
func splitActions(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// unionScopes returns the deduped union of two scope lists, preserving order (a, then b).
func unionScopes(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, s := range list {
			if s != "" && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// generateAuthURL generates the OAuth authorization URL with PKCE support for Kick.
//
// withChatScopes is set by the Twitch add-source flow to additionally request the
// EventSub chat scopes (TwitchChatScopes); the login flow passes false so logins
// keep the minimal scope set.
func (h *PlatformAuthHandlerV2) generateAuthURL(
	ctx context.Context,
	provider oauth.OAuthProvider,
	platform oauth.Platform,
	stateStr string,
	csrfToken string,
	withChatScopes bool,
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
	} else if platform == oauth.PlatformTwitch && withChatScopes {
		twitchProvider, ok := provider.(*oauth.TwitchOAuth)
		if !ok {
			return "", fmt.Errorf("twitch provider type assertion failed")
		}
		authURL = twitchProvider.GetAuthURLWithChatScopes(stateStr)
	} else {
		authURL = provider.GetAuthURL(stateStr)
	}

	return authURL, nil
}

// containsScope reports whether scopes contains want.
func containsScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
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
			// Check for idempotency tombstone — handles double-callbacks from iOS Safari
			// or Google issuing multiple auth codes for the same consent session.
			// The tombstone stores the original redirect URL for 60 seconds after state consumption.
			usedKey := fmt.Sprintf("oauth_state:%s:used:%s", platform, oauthState.CSRFToken)
			if usedRedirectURL, tombErr := h.redis.Get(c.Request.Context(), usedKey).Result(); tombErr == nil {
				h.logger.Info("Idempotent OAuth callback replay — replaying original redirect",
					zap.String("platform", string(platform)),
					zap.String("csrf_token", oauthState.CSRFToken),
				)
				c.Redirect(http.StatusFound, usedRedirectURL)
				return
			}
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

		// Check if platform ID is banned (prevent multi-account abuse)
		platformBanned, err := h.userRepo.IsPlatformIDBanned(c.Request.Context(), string(platform), platformUser.GetID())
		if err != nil {
			h.logger.Error("Failed to check platform ban",
				zap.String("platform", string(platform)),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
			return
		}
		if platformBanned {
			h.logger.Warn("Banned platform ID attempted login",
				zap.String("platform", string(platform)),
				zap.String("platform_id", platformUser.GetID()),
			)
			h.redirectWithTombstone(c, platform, oauthState.CSRFToken, fmt.Sprintf("%s/auth/banned", h.frontendURL))
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

			// Only fetch channel info when adding a source, not during login
			// This saves YouTube API quota and allows login without a channel
			if oauthState.IsAddSource() {
				channelInfo, channelErr := youtubeProvider.GetPrimaryChannel(c.Request.Context(), token.AccessToken)
				if channelErr != nil {
					h.logger.Error("Failed to resolve YouTube channel for add-source",
						zap.Error(channelErr),
						zap.String("platform_user_id", platformUser.GetID()))
					c.JSON(http.StatusInternalServerError, gin.H{
						"error":   "Unable to resolve YouTube channel. Please ensure your Google account has a YouTube channel.",
						"details": channelErr.Error(),
					})
					return
				}
				youtubeChannel = channelInfo
				sourceDetails = &OverlaySourceDetails{
					ChannelID:     channelInfo.ChannelID,
					ChannelName:   channelInfo.Title,
					ChannelHandle: channelInfo.Handle,
				}
			} else {
				// For login flow, skip channel resolution to save quota
				h.logger.Info("Skipping YouTube channel resolution during login (will fetch when adding source)",
					zap.String("platform_user_id", platformUser.GetID()))
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
			jwtToken, err = auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
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
			user, err = h.getOrCreateUser(c.Request.Context(), platform, platformUser, token, youtubeChannel)
			if err != nil {
				var dupErr *ErrDuplicateAccount
				if errors.As(err, &dupErr) {
					h.logger.Warn("Duplicate account registration blocked",
						zap.String("platform", string(platform)),
						zap.String("existing_username", dupErr.ExistingUsername),
					)
					redirectURL := fmt.Sprintf("%s/auth/callback?error=duplicate_account&existing_username=%s&platform=%s",
						h.frontendURL,
						dupErr.ExistingUsername,
						dupErr.Platform,
					)
					h.redirectWithTombstone(c, platform, oauthState.CSRFToken, redirectURL)
					return
				}
				h.logger.Error("Failed to get or create user",
					zap.String("platform", string(platform)),
					zap.Error(err),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
				return
			}

			// Generate JWT
			jwtToken, err = auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
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
				h.redirectWithTombstone(c, platform, oauthState.CSRFToken, redirectURL)
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

			h.redirectWithTombstone(c, platform, oauthState.CSRFToken, redirectURL)
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

			h.redirectWithTombstone(c, platform, oauthState.CSRFToken, redirectURL)
		}

		if platform == oauth.PlatformYouTube && youtubeChannel != nil {
			// granted_scopes carries the opt-in youtube.force-ssl grant (ADR-0017) for
			// the moderation service; merged (not replaced) so a plain add-source never
			// drops a prior moderation grant.
			if err := h.userRepo.StoreYouTubeToken(c.Request.Context(), user.ID, youtubeChannel.ChannelID, token, oauth.ExtractGrantedScopes(token)); err != nil {
				h.logger.Warn("Failed to store YouTube tokens for listener",
					zap.String("user_id", user.ID),
					zap.String("channel_id", youtubeChannel.ChannelID),
					zap.Error(err),
				)
			}
		}

		// Persist linked Kick credentials for non-Kick-login accounts so they can
		// moderate their connected Kick channel (ADR-0017). Mirrors the linked Twitch
		// path: a Kick-login account keeps its grant on the users row (linkPlatformToUser
		// reflow), so storing it here too would have token-refresh racing two copies of
		// the same rotating refresh token. The row carries the numeric broadcaster id
		// (platformUser.GetID()) the Kick moderation API keys on, keyed by the slug.
		if shouldStoreLinkedKickCredentials(user.AuthProvider, platform, oauthState.IsAddSource()) {
			scopes := oauth.ExtractGrantedScopes(token)
			if err := h.userRepo.StoreKickToken(c.Request.Context(),
				user.ID, platformUser.GetUsername(), platformUser.GetID(), token, scopes); err != nil {
				h.logger.Warn("Failed to store linked Kick credentials",
					zap.String("user_id", user.ID),
					zap.String("kick_slug", platformUser.GetUsername()),
					zap.Error(err),
				)
			} else {
				h.logger.Info("Stored linked Kick credentials",
					zap.String("user_id", user.ID),
					zap.String("kick_slug", platformUser.GetUsername()),
					zap.Strings("scopes", scopes),
				)
			}
		}

		// Persist linked Twitch credentials for non-Twitch-login accounts so
		// their channel can flip to the EventSub listener (ADR-0016). The
		// partition predicate and the EventSub listener match this table by
		// twitch_login; token-refresh-service keeps it fresh.
		if shouldStoreLinkedTwitchCredentials(user.AuthProvider, platform, oauthState.IsAddSource()) {
			scopes := oauth.ExtractGrantedScopes(token)
			if err := h.userRepo.StoreTwitchToken(c.Request.Context(),
				user.ID, platformUser.GetID(), platformUser.GetUsername(), token, scopes); err != nil {
				h.logger.Warn("Failed to store linked Twitch credentials",
					zap.String("user_id", user.ID),
					zap.String("twitch_login", platformUser.GetUsername()),
					zap.Error(err),
				)
			} else {
				h.logger.Info("Stored linked Twitch credentials",
					zap.String("user_id", user.ID),
					zap.String("twitch_login", platformUser.GetUsername()),
					zap.Strings("scopes", scopes),
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

// getOrCreateUser gets an existing user or creates a new one.
// youtubeChannel is optional (nil for non-YouTube platforms or when channel
// resolution was skipped during login). When present, its ChannelID is used
// for duplicate account detection against existing overlay sources.
func (h *PlatformAuthHandlerV2) getOrCreateUser(
	ctx context.Context,
	platform oauth.Platform,
	platformUser oauth.PlatformUserInfo,
	token *oauth2.Token,
	youtubeChannel *oauth.YouTubeChannelInfo,
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
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}

	if err != nil {
		// For YouTube login flow, channel info was skipped to save quota.
		// Now that we know this is a NEW user, fetch it for duplicate detection.
		if platform == oauth.PlatformYouTube && youtubeChannel == nil {
			if ytProvider, ok := h.providers[oauth.PlatformYouTube].(*oauth.YouTubeOAuth); ok {
				channelInfo, channelErr := ytProvider.GetPrimaryChannel(ctx, token.AccessToken)
				if channelErr != nil {
					h.logger.Info("Could not resolve YouTube channel for new user duplicate check",
						zap.Error(channelErr),
						zap.String("platform_user_id", platformUser.GetID()))
				} else {
					youtubeChannel = channelInfo
				}
			}
		}

		// User doesn't exist — before creating, check if a duplicate account
		// exists by looking for an overlay source with the same channel_id.
		// This catches the case where a streamer already registered via a
		// different platform and has the current platform configured as a source.
		if sourceChannelID := h.getSourceChannelID(platform, platformUser, youtubeChannel); sourceChannelID != "" {
			existingUsername, lookupErr := h.userRepo.FindExistingUserBySource(ctx, string(platform), sourceChannelID)
			if lookupErr != nil {
				h.logger.Warn("Failed to check for duplicate account by source",
					zap.Error(lookupErr),
					zap.String("platform", string(platform)),
					zap.String("channel_id", sourceChannelID))
				// Continue with creation — don't block on a lookup failure
			} else if existingUsername != "" {
				h.logger.Warn("Blocked duplicate account creation — existing account has source configured",
					zap.String("platform", string(platform)),
					zap.String("channel_id", sourceChannelID),
					zap.String("existing_username", existingUsername))
				return nil, &ErrDuplicateAccount{
					ExistingUsername: existingUsername,
					Platform:         string(platform),
					Message: fmt.Sprintf(
						"a channel matching your %s account is already configured on the account '%s'. "+
							"Please log in with that account and link %s from your account settings",
						platform, existingUsername, platform),
				}
			}
		}

		// Create new user
		platformID := platformUser.GetID()
		user = &models.User{
			AuthProvider:    string(platform),
			Username:        platformUser.GetUsername(),
			DisplayName:     platformUser.GetDisplayName(),
			ProfileImageURL: platformUser.GetProfileImageURL(),
			AccessToken:     token.AccessToken,
			RefreshToken:    token.RefreshToken,
			TokenExpiresAt:  token.Expiry,
			GrantedScopes:   oauth.ExtractGrantedScopes(token),
		}

		// Set the appropriate platform ID
		switch platform {
		case oauth.PlatformTwitch:
			user.TwitchID = &platformID
		case oauth.PlatformYouTube:
			user.GoogleID = &platformID
		case oauth.PlatformKick:
			user.KickID = &platformID
		}

		if err := h.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		if h.metrics != nil {
			h.metrics.RecordUserRegistration(string(platform))
		}

		h.logger.Info("Created new user",
			zap.String("platform", string(platform)),
			zap.String("user_id", user.ID),
			zap.String("username", user.Username),
		)
	} else {
		// Check if existing user is banned
		if user.IsBanned {
			return nil, fmt.Errorf("user account is banned")
		}

		// Update existing user profile fields.
		user.DisplayName = platformUser.GetDisplayName()
		user.ProfileImageURL = platformUser.GetProfileImageURL()

		// Decide whether this login would downgrade the stored OAuth scopes. A plain
		// Twitch login only requests the login scopes (withChatScopes=false); if the
		// streamer previously granted the EventSub chat scopes via add-source, replacing
		// their token here would knock their channel back to the IRC listener on every
		// login. So when the stored credentials already carry user:read:chat and this
		// token does not, keep the existing token + granted_scopes and refresh only the
		// profile fields. The user is still authenticated and issued a JWT.
		newScopes := oauth.ExtractGrantedScopes(token)
		existingScopes, _ := h.userRepo.GetGrantedScopes(ctx, user.ID)
		preserveScopes := wouldDowngradeScopes(existingScopes, newScopes)

		if !preserveScopes {
			user.AccessToken = token.AccessToken
			user.RefreshToken = token.RefreshToken
			user.TokenExpiresAt = token.Expiry
		}

		if err := h.userRepo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		// Keep granted_scopes in sync with the freshly issued token (unless we deliberately
		// preserved a broader stored grant above). Kept separate from Update so unrelated
		// user updates can never clobber the scope set.
		if !preserveScopes {
			if err := h.userRepo.UpdateGrantedScopes(ctx, user.ID, newScopes); err != nil {
				h.logger.Warn("Failed to update granted scopes on login",
					zap.String("user_id", user.ID), zap.Error(err))
			}
		}
	}

	return user, nil
}

// getSourceChannelID returns the channel identifier that would be stored in
// overlay_chat_sources for this platform. For Twitch and Kick this is the
// platform username; for YouTube it's the channel ID (UCxxx) from the
// YouTubeChannelInfo if available.
func (h *PlatformAuthHandlerV2) getSourceChannelID(platform oauth.Platform, user oauth.PlatformUserInfo, ytChannel *oauth.YouTubeChannelInfo) string {
	switch platform {
	case oauth.PlatformTwitch:
		return user.GetUsername() // Twitch login (lowercase)
	case oauth.PlatformKick:
		return user.GetUsername() // Kick username
	case oauth.PlatformYouTube:
		if ytChannel != nil {
			return ytChannel.ChannelID // UCxxx format
		}
		return ""
	default:
		return ""
	}
}

// linkMayReplacePrimaryCredentials decides whether a platform-link callback may
// overwrite the user's primary OAuth credentials (users.access_token /
// refresh_token / token_expires_at) and the granted_scopes record.
//
// The primary credentials always belong to the user's auth_provider. Linking a
// DIFFERENT platform (e.g. a Twitch-login streamer connecting YouTube or Kick
// as an additional source) must never touch them: those platforms persist their
// tokens in their own tables (youtube_oauth_tokens / kick_oauth_tokens), and
// overwriting granted_scopes here erased the user:read:chat grant — silently
// demoting the streamer's channel from the EventSub listener back to IRC.
//
// For a same-platform reflow (the Twitch add-source consent that upgrades a
// login-scoped token to one carrying the chat scopes) the replacement is the
// whole point. The downgrade guard mirrors the login path: a token WITHOUT
// user:read:chat must not replace a stored grant that has it.
func linkMayReplacePrimaryCredentials(authProvider string, platform oauth.Platform, existingScopes, newScopes []string) bool {
	if string(platform) != authProvider {
		return false
	}
	if wouldDowngradeScopes(existingScopes, newScopes) {
		return false
	}
	return true
}

// preservableScopes are grants obtained ONLY through an explicit opt-in consent flow:
// the EventSub chat scope (add-source) and the moderation scopes (moderation
// re-consent, ADR-0017). A narrower token from a plain login or an unrelated platform
// link must never drop them — doing so silently demotes the streamer's channel to the
// IRC listener (chat scope) or disables their moderation controls until they
// re-authorize. The opt-in flows always request a SUPERSET, so a genuine upgrade is
// never blocked by this guard.
var preservableScopes = []string{
	"user:read:chat",
	"moderator:manage:chat_messages",
	"moderator:manage:banned_users",
	"moderation:ban",                                    // Kick moderation grant (opt-in re-consent, ADR-0017)
	"https://www.googleapis.com/auth/youtube.force-ssl", // YouTube moderation grant (ADR-0017)
}

// wouldDowngradeScopes reports whether replacing existing with incoming would drop any
// preservable (opt-in) scope. When true, callers keep the stored token + granted_scopes
// and refresh only non-credential profile fields.
func wouldDowngradeScopes(existing, incoming []string) bool {
	for _, s := range preservableScopes {
		if containsScope(existing, s) && !containsScope(incoming, s) {
			return true
		}
	}
	return false
}

// shouldStoreLinkedTwitchCredentials decides whether a Twitch consent must be
// persisted to twitch_oauth_tokens (ADR-0016). That table exists for accounts
// whose login provider is NOT Twitch (YouTube/Kick signups): their users row
// can never satisfy the IRC↔EventSub partition predicate (it matches
// username + auth_provider='twitch'), so the per-link table is the only place
// their channel's chat grant can live. Twitch-login accounts keep their grant
// on the users row (linkPlatformToUser reflow) — storing it twice would have
// token-refresh racing two copies of the same refresh token.
func shouldStoreLinkedTwitchCredentials(authProvider string, platform oauth.Platform, isAddSource bool) bool {
	return platform == oauth.PlatformTwitch && isAddSource && authProvider != "twitch"
}

// shouldStoreLinkedKickCredentials decides whether a Kick consent must be persisted to
// kick_oauth_tokens with its moderation columns (kick_user_id + granted_scopes,
// migration 062). Like the Twitch sibling this is for accounts whose login provider is
// NOT Kick: a Kick-login account keeps its grant on the users row (the
// linkPlatformToUser same-platform reflow), and the moderation service prefers that
// users row — storing a second copy here would have token-refresh racing two copies of
// the same rotating Kick refresh token.
func shouldStoreLinkedKickCredentials(authProvider string, platform oauth.Platform, isAddSource bool) bool {
	return platform == oauth.PlatformKick && isAddSource && authProvider != "kick"
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

	// Check if user is banned
	if user.IsBanned {
		return nil, fmt.Errorf("user account is banned")
	}

	// Update the user with the new platform ID
	platformID := platformUser.GetID()

	switch platform {
	case oauth.PlatformTwitch:
		user.TwitchID = &platformID
	case oauth.PlatformYouTube:
		user.GoogleID = &platformID
	case oauth.PlatformKick:
		user.KickID = &platformID
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	newScopes := oauth.ExtractGrantedScopes(token)
	existingScopes, err := h.userRepo.GetGrantedScopes(ctx, user.ID)
	if err != nil {
		// Without the stored scopes we cannot prove the replacement is safe;
		// keep the existing credentials rather than risk wiping a chat grant.
		h.logger.Warn("Failed to read granted scopes during platform link; preserving credentials",
			zap.String("user_id", user.ID), zap.Error(err))
		return user, nil
	}

	if !linkMayReplacePrimaryCredentials(user.AuthProvider, platform, existingScopes, newScopes) {
		h.logger.Info("Platform link kept primary credentials and granted scopes",
			zap.String("user_id", user.ID),
			zap.String("auth_provider", user.AuthProvider),
			zap.String("linked_platform", string(platform)))
		return user, nil
	}

	// Same-platform reflow: replace the primary token (it carries at least the
	// scopes of the old one, see linkMayReplacePrimaryCredentials).
	user.AccessToken = token.AccessToken
	user.RefreshToken = token.RefreshToken
	user.TokenExpiresAt = token.Expiry

	// Update user in database
	if err := h.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Persist the granted scopes. For the Twitch add-source reflow the token
	// carries the chat scopes (user:read:chat, user:bot, channel:bot) — recording
	// them here is what flips the channel from the IRC listener to the EventSub
	// listener on the next sync. Kept separate from Update (see UpdateGrantedScopes).
	if err := h.userRepo.UpdateGrantedScopes(ctx, user.ID, newScopes); err != nil {
		h.logger.Warn("Failed to persist granted scopes after platform link",
			zap.String("user_id", user.ID), zap.Error(err))
	}

	return user, nil
}

// addSourceToOverlay calls the overlay-manager internal API to add a source
type OverlaySourceDetails struct {
	ChannelID     string
	ChannelName   string
	ChannelHandle string
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
	// IMPORTANT: For Twitch, use username (login) not display_name for channel_id
	// Twitch usernames are lowercase alphanumeric only (e.g., "shahin200x")
	// Display names can have Unicode, mixed case (e.g., "شوشو")
	channelID := platformUser.GetUsername() // For Twitch this is the "login" field
	channelName := platformUser.GetDisplayName()
	channelHandle := ""

	if details != nil {
		if details.ChannelID != "" {
			channelID = details.ChannelID
		}
		if details.ChannelName != "" {
			channelName = details.ChannelName
		}
		if details.ChannelHandle != "" {
			channelHandle = details.ChannelHandle
		}
	}

	reqBody := map[string]interface{}{
		"platform":       string(platform),
		"channel_id":     channelID,
		"channel_name":   channelName,
		"channel_handle": channelHandle,
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

// redirectWithTombstone performs an OAuth redirect and stores an idempotency tombstone in Redis.
// The tombstone keeps the redirect URL for 60 seconds so that duplicate callbacks (e.g. from iOS
// Safari prefetch or Google issuing multiple auth codes for the same consent session) are replayed
// with the original redirect instead of returning "Invalid or expired state".
func (h *PlatformAuthHandlerV2) redirectWithTombstone(c *gin.Context, platform oauth.Platform, csrfToken, redirectURL string) {
	usedKey := fmt.Sprintf("oauth_state:%s:used:%s", platform, csrfToken)
	if err := h.redis.Set(c.Request.Context(), usedKey, redirectURL, 60*time.Second).Err(); err != nil {
		h.logger.Warn("Failed to store OAuth callback tombstone",
			zap.String("platform", string(platform)),
			zap.String("csrf_token", csrfToken),
			zap.Error(err),
		)
	}
	c.Redirect(http.StatusFound, redirectURL)
}
