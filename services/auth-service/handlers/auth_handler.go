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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/metrics"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	twitchOAuth  *oauth.TwitchOAuth
	youtubeOAuth *oauth.YouTubeOAuth
	kickOAuth    *oauth.KickOAuth
	userRepo     *repository.UserRepository
	redis        *redis.Client
	userKeyChain *auth.KeyChain
	jwtExpiry    time.Duration
	logger       *zap.Logger
	metrics      *metrics.BusinessMetrics
}

// NewAuthHandler creates a new auth handler. kickOAuth may be nil when Kick is
// not configured; HandleRefresh dispatches the streamer's refresh to the
// provider that minted the session token (audit #6), so all three must be
// passed when available.
func NewAuthHandler(
	twitchOAuth *oauth.TwitchOAuth,
	youtubeOAuth *oauth.YouTubeOAuth,
	kickOAuth *oauth.KickOAuth,
	userRepo *repository.UserRepository,
	redisClient *redis.Client,
	userKeyChain *auth.KeyChain,
	jwtExpiryHours int,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		twitchOAuth:  twitchOAuth,
		youtubeOAuth: youtubeOAuth,
		kickOAuth:    kickOAuth,
		userRepo:     userRepo,
		redis:        redisClient,
		userKeyChain: userKeyChain,
		jwtExpiry:    time.Duration(jwtExpiryHours) * time.Hour,
		logger:       logger,
	}
}

// WithMetrics attaches a BusinessMetrics instance for recording user registration events.
func (h *AuthHandler) WithMetrics(m *metrics.BusinessMetrics) *AuthHandler {
	h.metrics = m
	return h
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

	// Generate auth URL with PKCE (audit L4)
	authURL, codeVerifier := h.twitchOAuth.GetAuthURLWithPKCE(state)
	// Store PKCE verifier in Redis for the callback
	if err := h.redis.Set(c.Request.Context(), "oauth_verifier:twitch:"+state, codeVerifier, 10*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to store PKCE verifier", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// HandleYouTubeLogin initiates the YouTube OAuth flow
func (h *AuthHandler) HandleYouTubeLogin(c *gin.Context) {
	// Generate random state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		h.logger.Error("Failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Store state in Redis with 10 minute expiry
	err = h.redis.Set(c.Request.Context(), "oauth_state:youtube:"+state, "1", 10*time.Minute).Err()
	if err != nil {
		h.logger.Error("Failed to store state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Generate auth URL with PKCE (audit L4)
	authURL, codeVerifier := h.youtubeOAuth.GetAuthURLWithPKCE(state)
	// Store PKCE verifier in Redis for the callback
	if err := h.redis.Set(c.Request.Context(), "oauth_verifier:youtube:"+state, codeVerifier, 10*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to store PKCE verifier", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

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

	// Verify state (atomic Get+Del to prevent TOCTOU, audit L5)
	exists, err := h.redis.GetDel(c.Request.Context(), "oauth_state:"+state).Result()
	if err != nil || exists == "" {
		h.logger.Warn("Invalid or expired state", zap.String("state", state))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
		return
	}

	// Exchange code for token (with PKCE, audit L4)
	codeVerifier, verifierErr := h.redis.GetDel(c.Request.Context(), "oauth_verifier:twitch:"+state).Result()
	var token *oauth2.Token
	if verifierErr == nil && codeVerifier != "" {
		token, err = h.twitchOAuth.ExchangeCodeWithVerifier(c.Request.Context(), code, codeVerifier)
	} else {
		token, err = h.twitchOAuth.ExchangeCode(c.Request.Context(), code)
	}
	if err != nil {
		h.logger.Error("Failed to exchange code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info from Twitch (use the specific Twitch method)
	twitchUser, err := h.twitchOAuth.GetUserInfoTwitch(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Check if platform ID is banned
	platformBanned, err := h.userRepo.IsPlatformIDBanned(c.Request.Context(), "twitch", twitchUser.ID)
	if err != nil {
		h.logger.Error("Failed to check platform ban", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
		return
	}
	if platformBanned {
		h.logger.Warn("Banned platform ID attempted login",
			zap.String("platform", "twitch"),
			zap.String("platform_id", twitchUser.ID))
		frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", frontendURL))
		return
	}

	// Check if user exists
	user, err := h.userRepo.GetByTwitchID(c.Request.Context(), twitchUser.ID)
	if err != nil {
		// Create new user
		twitchID := twitchUser.ID
		user = &models.User{
			TwitchID:        &twitchID,
			AuthProvider:    "twitch",
			Username:        twitchUser.Login,
			DisplayName:     twitchUser.DisplayName,
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

		if h.metrics != nil {
			h.metrics.RecordUserRegistration("twitch")
		}
	} else {
		// Check if existing user is banned
		if user.IsBanned {
			h.logger.Warn("Banned user attempted login",
				zap.String("user_id", user.ID),
				zap.String("username", user.Username))
			frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
			c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", frontendURL))
			return
		}

		// Update existing user
		user.Username = twitchUser.Login
		user.DisplayName = twitchUser.DisplayName
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
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Redirect to frontend with a short-lived single-use auth code (not the token).
	// The frontend exchanges the code via POST /exchange to retrieve the tokens,
	// eliminating token exposure in the URL fragment (audit M1).
	frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
	code, storeErr := storeStreamerAuthCode(c.Request.Context(), h.redis, StreamerAuthPayload{
		AccessToken:  jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    int64(h.jwtExpiry.Seconds()),
		TokenType:    "Bearer",
		User: &StreamerAuthUser{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			IsAdmin:     user.IsAdmin,
		},
	})
	if storeErr != nil {
		// L4: log storeErr (the storeStreamerAuthCode error), not err (the stale
		// token-exchange error from above).
		h.logger.Error("Failed to store streamer auth code", zap.Error(storeErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate redirect"})
		return
	}
	redirectURL := fmt.Sprintf("%s/auth/callback?code=%s", frontendURL, code)

	// Track refresh token for reuse detection (audit M2).
	if token.RefreshToken != "" {
		rtKey := "refresh_token:" + refreshTokenHash(token.RefreshToken)
		if err := h.redis.Set(c.Request.Context(), rtKey, user.ID, 14*24*time.Hour).Err(); err != nil {
			h.logger.Warn("Failed to track refresh token for reuse detection", zap.Error(err))
		}
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// HandleYouTubeCallback handles the YouTube OAuth callback
func (h *AuthHandler) HandleYouTubeCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state (atomic Get+Del to prevent TOCTOU, audit L5)
	exists, err := h.redis.GetDel(c.Request.Context(), "oauth_state:youtube:"+state).Result()
	if err != nil || exists == "" {
		h.logger.Warn("Invalid or expired state", zap.String("state", state))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
		return
	}

	// Exchange code for token (with PKCE, audit L4)
	ytCodeVerifier, ytVerifierErr := h.redis.GetDel(c.Request.Context(), "oauth_verifier:youtube:"+state).Result()
	var token *oauth2.Token
	if ytVerifierErr == nil && ytCodeVerifier != "" {
		token, err = h.youtubeOAuth.ExchangeCodeWithVerifier(c.Request.Context(), code, ytCodeVerifier)
	} else {
		token, err = h.youtubeOAuth.ExchangeCode(c.Request.Context(), code)
	}
	if err != nil {
		h.logger.Error("Failed to exchange code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info from Google (use the specific YouTube method)
	youtubeUser, err := h.youtubeOAuth.GetUserInfoYouTube(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Error("Failed to get user info", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Check if platform ID is banned
	platformBanned, err := h.userRepo.IsPlatformIDBanned(c.Request.Context(), "youtube", youtubeUser.ID)
	if err != nil {
		h.logger.Error("Failed to check platform ban", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
		return
	}
	if platformBanned {
		h.logger.Warn("Banned platform ID attempted login",
			zap.String("platform", "youtube"),
			zap.String("platform_id", youtubeUser.ID))
		frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
		c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", frontendURL))
		return
	}

	// Check if user exists by Google ID
	user, err := h.userRepo.GetByGoogleID(c.Request.Context(), youtubeUser.ID)
	if err != nil {
		// Create new YouTube user
		googleID := youtubeUser.ID
		youtubeUsername := fmt.Sprintf("youtube_%s", youtubeUser.ID)
		user = &models.User{
			GoogleID:        &googleID,
			AuthProvider:    "youtube",
			Username:        youtubeUsername,
			DisplayName:     youtubeUser.Name,
			ProfileImageURL: youtubeUser.Picture,
			AccessToken:     token.AccessToken,
			RefreshToken:    token.RefreshToken,
			TokenExpiresAt:  token.Expiry,
		}

		if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
			h.logger.Error("Failed to create YouTube user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		if h.metrics != nil {
			h.metrics.RecordUserRegistration("youtube")
		}

		h.logger.Info("Created new YouTube user",
			zap.String("user_id", user.ID),
			zap.String("google_id", googleID),
		)
	} else {
		// Check if existing user is banned
		if user.IsBanned {
			h.logger.Warn("Banned user attempted login",
				zap.String("user_id", user.ID),
				zap.String("username", user.Username))
			frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
			c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/banned", frontendURL))
			return
		}

		// Update existing user
		user.DisplayName = youtubeUser.Name
		user.ProfileImageURL = youtubeUser.Picture
		user.AccessToken = token.AccessToken
		user.RefreshToken = token.RefreshToken
		user.TokenExpiresAt = token.Expiry

		if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
			h.logger.Error("Failed to update YouTube user", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	channelInfo, err := h.youtubeOAuth.GetPrimaryChannel(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.logger.Warn("Failed to resolve YouTube channel (non-fatal, skipping token store)", zap.Error(err))
	} else {
		// Store YouTube OAuth tokens for YouTube Listener
		if err := h.userRepo.StoreYouTubeToken(c.Request.Context(), user.ID, channelInfo.ChannelID, token, oauth.ExtractGrantedScopes(token)); err != nil {
			h.logger.Error("Failed to store YouTube tokens", zap.Error(err))
		}
	}

	// Generate JWT
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Redirect to frontend with a short-lived single-use auth code (audit M1).
	frontendURL := getEnvOrDefault("FRONTEND_URL", "http://localhost:3000")
	code, storeErr := storeStreamerAuthCode(c.Request.Context(), h.redis, StreamerAuthPayload{
		AccessToken:  jwtToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    int64(h.jwtExpiry.Seconds()),
		TokenType:    "Bearer",
		User: &StreamerAuthUser{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			IsAdmin:     user.IsAdmin,
		},
	})
	if storeErr != nil {
		// L4: log storeErr (the storeStreamerAuthCode error), not err (the stale
		// channel/token-exchange error from above).
		h.logger.Error("Failed to store streamer auth code", zap.Error(storeErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate redirect"})
		return
	}
	redirectURL := fmt.Sprintf("%s/auth/callback?code=%s", frontendURL, code)

	// Track refresh token for reuse detection (audit M2).
	if token.RefreshToken != "" {
		rtKey := "refresh_token:" + refreshTokenHash(token.RefreshToken)
		if err := h.redis.Set(c.Request.Context(), rtKey, user.ID, 14*24*time.Hour).Err(); err != nil {
			h.logger.Warn("Failed to track refresh token for reuse detection", zap.Error(err))
		}
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// HandleStreamerTokenExchange swaps a short-lived auth code for the streamer's
// token payload (access + refresh tokens). POST /exchange { "code": "..." }
// (audit M1 — replaces URL-fragment token exposure with code+POST exchange).
func (h *AuthHandler) HandleStreamerTokenExchange(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	key := "streamer_auth_code:" + req.Code
	data, err := h.redis.GetDel(c.Request.Context(), key).Result()
	if err == redis.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
		return
	}
	if err != nil {
		h.logger.Error("Failed to retrieve streamer auth code", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var payload StreamerAuthPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		h.logger.Error("Failed to unmarshal streamer auth payload", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Issue httpOnly cookies (audit H3). Tokens live in cookies, not the body,
	// so they are not exposed to frontend JavaScript or logged in responses.
	auth.SetAuthCookies(c, payload.AccessToken, payload.RefreshToken,
		time.Duration(payload.ExpiresIn)*time.Second, 14*24*time.Hour)

	// C2: seed the refresh-token reuse-detection key on initial cookie issue.
	// HandleRefresh does GetDel("refresh_token:"+hash) and treats a miss as token
	// theft → 401 + ClearAuthCookies. The production login path (HandleCallback →
	// /exchange → here) previously never seeded that key (only the legacy
	// HandleCallback/HandleYouTubeCallback seeded it), so the first /refresh after
	// any real login was misclassified as reuse and force-logged-out every user.
	// Mirror the seeding in HandleCallback so the first refresh succeeds.
	if payload.RefreshToken != "" {
		rtKey := "refresh_token:" + refreshTokenHash(payload.RefreshToken)
		if err := h.redis.Set(c.Request.Context(), rtKey, payload.User.ID, 14*24*time.Hour).Err(); err != nil {
			h.logger.Warn("Failed to track refresh token for reuse detection on exchange",
				zap.Error(err))
		}
	}

	// Body carries only non-secret data for the UI.
	c.JSON(http.StatusOK, gin.H{
		"expires_in":         payload.ExpiresIn,
		"token_type":         payload.TokenType,
		"redirect_to":        payload.RedirectTo,
		"source_added":       payload.SourceAdded,
		"moderation_enabled": payload.ModerationEnabled,
		"user":               payload.User,
	})
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return defaultValue
}

// HandleRefresh rotates the session by exchanging a Twitch refresh token for
// a fresh access token and reissuing httpOnly cookies (audit H3). The refresh
// token is read from the X-Refresh-Token header (forwarded by the gateway
// AuthCookieForward middleware; the raw Cookie is stripped by L17 before it
// reaches auth-service). A JSON-body fallback is kept for backward compat
// during the rollout (deprecated).
//
// HandleRefresh resolves the session owner from the reuse-key's stored user ID,
// dispatches the refresh to the provider that minted the session (Twitch /
// YouTube / Kick — audit #6), persists + rotates, and reissues httpOnly cookies.
//
// Failure discipline (audit #2 + #5): the rule is that whenever a 5xx is
// returned the refresh token in the client's cookie MUST still have a live
// Redis reuse key, so a transient blip is retryable rather than a permanent
// lockout. A genuine reuse-miss or provider 400/401 (revoked token) clears the
// cookies; transient Redis/DB/provider errors restore the key and return 503.
func (h *AuthHandler) HandleRefresh(c *gin.Context) {
	// audit #21: refuse to silently rotate an impersonation session into the
	// admin's own session. The access token is forwarded as X-Access-Token by
	// the gateway AuthCookieForward middleware; if it carries an impersonation
	// claim, end the impersonation explicitly instead of issuing a plain admin
	// JWT (which would leave the UI showing "impersonating" while acting as admin).
	// Use 403 (not 409): the client reserves a 409 from /refresh for the
	// concurrent-refresh retry path (review M1); a 403 is treated as a terminal
	// refresh failure → the session is re-derived from /auth/me, which is exactly
	// the resync we want here.
	if at := c.GetHeader("X-Access-Token"); at != "" {
		if claims, vErr := auth.ValidateJWTWithKeyChain(at, h.userKeyChain); vErr == nil && claims.ImpersonatedBy != "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "impersonation_session_ended",
				"impersonation_ended": true,
			})
			return
		}
	}

	// Read refresh token from the X-Refresh-Token header (forwarded by the
	// gateway AuthCookieForward middleware; the raw Cookie is stripped by L17
	// before reaching auth-service). Fallback to JSON body for backward compat
	// during rollout (deprecated).
	refreshToken := c.GetHeader("X-Refresh-Token")
	if refreshToken == "" {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}
	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token required"})
		return
	}

	// Refresh-token reuse detection (audit M2).
	// Each refresh token is stored in Redis as refresh_token:<sha256> when issued,
	// with the owning user's ID as the value. GetDel atomically retrieves+deletes.
	tokenHash := refreshTokenHash(refreshToken)
	rtKey := "refresh_token:" + tokenHash
	// Concurrency guard (PR #478 review M1): browsers routinely fire near-
	// simultaneous /refresh calls (multiple tabs, a retried slow request,
	// StrictMode double-invoke). Without this lock exactly one wins the single-use
	// GetDel below and every other concurrent request mis-classifies as theft →
	// ClearAuthCookies + 401, force-logging-out a legitimate user. Serialize per
	// token hash: only the lock holder rotates; a concurrent duplicate gets a
	// retryable 409 and keeps its cookies (the holder's Set-Cookie updates the
	// shared cookie jar, so the duplicate's client just retries the original
	// request). Theft detection is preserved — a stolen token replayed when no
	// rotation is in flight finds no lock and still hits the GetDel-miss reuse
	// path. The short TTL self-heals if a rotation crashes mid-flight. Redis
	// errors fail open to the pre-existing behaviour.
	lockKey := "refresh_lock:" + tokenHash
	if locked, lockErr := h.redis.SetNX(c.Request.Context(), lockKey, "1", 15*time.Second).Result(); lockErr == nil {
		if !locked {
			h.logger.Info("Concurrent refresh for same token — returning retryable 409",
				zap.String("refresh_token_hash", tokenHash[:16]))
			c.JSON(http.StatusConflict, gin.H{"error": "refresh already in progress, please retry"})
			return
		}
		// We hold the lock; release it on every exit path (best-effort, the TTL is
		// the backstop). Use a detached context so cleanup isn't skipped when the
		// request context is already done.
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.redis.Del(ctx, lockKey)
		}()
	}

	storedUserID, err := h.redis.GetDel(c.Request.Context(), rtKey).Result()
	if err == redis.Nil || (err == nil && storedUserID == "") {
		// Genuine miss: the token is not in the active set → reuse/theft. Terminal.
		h.logger.Warn("Refresh token reuse detected — token not in active set",
			zap.String("refresh_token_hash", tokenHash[:16]))
		auth.ClearAuthCookies(c) // clear stale cookies so the client re-auths (audit H3)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token already used or invalid"})
		return
	}
	if err != nil {
		// Transient Redis error (the documented single-replica SPOF). Do NOT treat
		// a connection blip as theft — that would force-log-out every refreshing
		// user during a Redis outage. The key was NOT consumed (GetDel errored), so
		// the client's cookie token is still valid; return a retryable 503.
		h.logger.Warn("Refresh reuse-key lookup failed (transient) — not treating as reuse",
			zap.String("refresh_token_hash", tokenHash[:16]),
			zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Refresh temporarily unavailable, please retry"})
		return
	}

	// restoreReuseKey re-seeds the OLD reuse key so a transient downstream failure
	// (before cookie rotation) leaves the client's existing cookie token retryable.
	restoreReuseKey := func() {
		if setErr := h.redis.Set(c.Request.Context(), rtKey, storedUserID, 14*24*time.Hour).Err(); setErr != nil {
			h.logger.Warn("Failed to restore refresh-token reuse key", zap.Error(setErr))
		}
	}

	// Resolve the session owner up front from the reuse-key value. This both
	// removes the fragile GetUserInfo/GetByTwitchID round-trips (audit #2/#5) and
	// lets us dispatch to the correct OAuth provider (audit #6).
	user, err := h.userRepo.GetByID(c.Request.Context(), storedUserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			h.logger.Warn("Refresh for unknown user — terminal", zap.String("user_id", storedUserID))
			auth.ClearAuthCookies(c) // audit H3
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		// Transient DB error (CNPG failover, pool exhaustion). Restore the key and
		// surface a retryable status — do NOT clear cookies (audit #5).
		h.logger.Warn("Transient DB error resolving refresh user — restoring reuse key",
			zap.String("user_id", storedUserID), zap.Error(err))
		restoreReuseKey()
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable, please retry"})
		return
	}

	// Dispatch the refresh to the provider that minted the session (audit #6).
	// Sending a Google/Kick refresh token to Twitch (the old unconditional path)
	// produced a 400 invalid_grant → terminal force-logout for every YouTube/Kick
	// streamer at each token expiry.
	token, err := h.refreshForProvider(c.Request.Context(), user.AuthProvider, refreshToken)
	if err != nil {
		// Terminal provider rejection (400/401: token revoked / invalid_grant) →
		// clear cookies. Anything else (5xx, network, unsupported provider) is
		// transient/retryable → restore key + 503 (audit L2 + #5).
		var retrErr *oauth2.RetrieveError
		if errors.As(err, &retrErr) && retrErr.Response != nil &&
			(retrErr.Response.StatusCode == http.StatusBadRequest || retrErr.Response.StatusCode == http.StatusUnauthorized) {
			h.logger.Warn("Refresh token rejected by provider (terminal)",
				zap.String("provider", user.AuthProvider),
				zap.String("refresh_token_hash", tokenHash[:16]),
				zap.Int("status", retrErr.Response.StatusCode),
				zap.Error(err))
			auth.ClearAuthCookies(c) // audit H3
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to refresh token"})
			return
		}
		h.logger.Warn("Transient refresh failure — restoring reuse key for retry",
			zap.String("provider", user.AuthProvider),
			zap.String("refresh_token_hash", tokenHash[:16]),
			zap.Error(err))
		restoreReuseKey()
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Refresh provider temporarily unavailable, please retry"})
		return
	}

	// Issue the new access JWT before persisting so a rare JWT-gen failure can
	// restore the old key (the old provider refresh token may already be spent,
	// but the retry then fails cleanly rather than locking out on a transient).
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
	if err != nil {
		h.logger.Error("Failed to generate JWT", zap.Error(err))
		restoreReuseKey()
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to issue session, please retry"})
		return
	}

	// Audit #2: seed the NEW reuse key AND rotate the cookies up front, BEFORE the
	// DB write. From here the client holds a token whose reuse key exists, so a
	// failed UpdateTokens is a retryable inconsistency, not a lockout.
	newRTKey := "refresh_token:" + refreshTokenHash(token.RefreshToken)
	if err := h.redis.Set(c.Request.Context(), newRTKey, user.ID, 14*24*time.Hour).Err(); err != nil {
		h.logger.Warn("Failed to track new refresh token for reuse detection", zap.Error(err))
	}
	// Rotate cookies (audit H3). Tokens live in httpOnly cookies, not the body.
	auth.SetAuthCookies(c, jwtToken, token.RefreshToken, h.jwtExpiry, 14*24*time.Hour)

	// Persist the rotated provider tokens (best-effort: the session is already
	// valid via the rotated cookie + Redis key; a stale DB token self-heals on the
	// next successful refresh).
	if err := h.userRepo.UpdateTokens(c.Request.Context(), user.ID, token.AccessToken, token.RefreshToken, token.Expiry); err != nil {
		h.logger.Warn("Failed to persist rotated tokens (session still valid)",
			zap.String("user_id", user.ID), zap.Error(err))
	}

	// Body carries only non-secret data for the UI.
	c.JSON(http.StatusOK, gin.H{
		"expires_in": int(h.jwtExpiry.Seconds()),
		"token_type": "Bearer",
		"user":       gin.H{"id": user.ID, "username": user.Username, "is_admin": user.IsAdmin},
	})
}

// refreshForProvider dispatches an OAuth refresh to the provider that issued the
// session (audit #6). An unsupported/unconfigured provider returns a non-terminal
// error so the caller treats it as retryable (503) rather than clearing cookies.
func (h *AuthHandler) refreshForProvider(ctx context.Context, provider, refreshToken string) (*oauth2.Token, error) {
	switch provider {
	case "youtube", "google":
		return h.youtubeOAuth.RefreshToken(ctx, refreshToken)
	case "kick":
		if h.kickOAuth == nil {
			return nil, fmt.Errorf("kick refresh not configured")
		}
		return h.kickOAuth.RefreshToken(ctx, refreshToken)
	default: // "twitch" and legacy empty provider
		return h.twitchOAuth.RefreshToken(ctx, refreshToken)
	}
}

// HandleGetMe returns current user info. When the JWT carries an
// ImpersonatedBy claim (set by JWTAuthWithRevocation into the context as
// "impersonated_by"), the response includes an "impersonating" flag + the
// admin's identifier so the frontend can restore isImpersonating across page
// reloads (audit H3).
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

	// Marshal the user to a map to preserve the existing response shape while
	// allowing the impersonation fields to be appended only when set.
	resp := gin.H{}
	if raw, err := json.Marshal(user); err == nil {
		_ = json.Unmarshal(raw, &resp)
	}
	if ib, ok := c.Get("impersonated_by"); ok {
		if ibStr, ok := ib.(string); ok && ibStr != "" {
			resp["impersonating"] = true
			resp["impersonated_by"] = ibStr
		}
	}
	c.JSON(http.StatusOK, resp)
}

// HandleLogout logs out the user
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	// Read token from X-Access-Token (gateway AuthCookieForward) or
	// Authorization (backward compat). A personal access token is not a session and is
	// never blacklisted here — see blacklistableSessionToken.
	token := blacklistableSessionToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Blacklist the access JWT (expires after JWT expiry).
	if err := h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry).Err(); err != nil {
		h.logger.Error("Failed to blacklist token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Revoke the refresh token family entry (audit H3).
	if rt := c.GetHeader("X-Refresh-Token"); rt != "" {
		rtKey := "refresh_token:" + refreshTokenHash(rt)
		if err := h.redis.Del(c.Request.Context(), rtKey).Err(); err != nil {
			h.logger.Warn("Failed to revoke refresh token on logout", zap.Error(err))
		}
	}

	// Clear the auth cookies.
	auth.ClearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// HandleDeleteAccount deletes the authenticated user's account and cascades associated data
func (h *AuthHandler) HandleDeleteAccount(c *gin.Context) {
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Account deletion is session-only. A personal access token is scoped to chat and
	// engagement writes; it must never be able to irreversibly delete the account and
	// cascade every overlay. Checked before any mutation so a refusal changes nothing.
	if !RefuseAPIToken(c, "Deleting your account requires a signed-in session, not a personal access token.") {
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), userID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		h.logger.Error("Failed to delete user account",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	// Best-effort blacklist of the session token used for this request (never a
	// personal access token — see blacklistableSessionToken). The account row is gone,
	// so its api_tokens rows went with it via ON DELETE CASCADE (migration 086).
	token := blacklistableSessionToken(c)
	if token != "" {
		if err := h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry).Err(); err != nil {
			h.logger.Warn("Failed to blacklist token after account deletion",
				zap.String("user_id", userID),
				zap.Error(err),
			)
		}
	}

	// L3: bring account deletion to parity with /logout — revoke the refresh-token
	// reuse key (forwarded as X-Refresh-Token by AuthCookieForward) and clear the
	// auth cookies so the browser drops the now-orphaned session.
	if rt := c.GetHeader("X-Refresh-Token"); rt != "" {
		rtKey := "refresh_token:" + refreshTokenHash(rt)
		if err := h.redis.Del(c.Request.Context(), rtKey).Err(); err != nil {
			h.logger.Warn("Failed to revoke refresh token on account deletion",
				zap.String("user_id", userID),
				zap.Error(err))
		}
	}
	auth.ClearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{"message": "Account deleted successfully"})
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// blacklistableSessionToken returns the raw session token to blacklist, or "" when
// there is nothing blacklistable.
//
// It reads X-Access-Token (forwarded by the gateway's AuthCookieForward, since the raw
// Cookie is stripped by L17) and falls back to the Authorization bearer.
//
// A personal access token (ADR-0051) is deliberately excluded. The blacklist key is
// "blacklist:<raw-token>", so blacklisting a PAT would write the plaintext secret into
// Redis — and it would achieve nothing anyway, because the PAT branch in
// shared/middleware never consults the blacklist: PAT revocation is api_tokens.revoked_at,
// read live on every request. Revoking a PAT is DELETE /me/api-tokens/:id, not logout.
func blacklistableSessionToken(c *gin.Context) string {
	token := c.GetHeader("X-Access-Token")
	if token == "" {
		if ah := c.GetHeader("Authorization"); strings.HasPrefix(ah, "Bearer ") {
			token = strings.TrimPrefix(ah, "Bearer ")
		}
	}
	if sharedmiddleware.IsAPIToken(token) {
		return ""
	}
	return token
}

// refreshTokenHash returns a hex-encoded SHA-256 hash of the refresh token for
// use as a Redis key (audit M2). Hashing avoids storing raw tokens in Redis.
func refreshTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
