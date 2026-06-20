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

// Package handlers contains the HTTP handlers for payment-service.
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// stateTTL is how long a started Patreon connect flow remains valid.
const stateTTL = 30 * time.Minute

// OAuthHandler serves the "Connect Patreon" flow.
type OAuthHandler struct {
	oauth       *patreon.OAuth
	redis       *redis.Client
	tokenRepo   *repository.TokenRepository
	entitlement *entitlement.Service
	campaignID  string
	frontendURL string
	logger      *zap.Logger
}

// NewOAuthHandler builds an OAuthHandler.
func NewOAuthHandler(oauth *patreon.OAuth, rdb *redis.Client, tokenRepo *repository.TokenRepository, ent *entitlement.Service, campaignID, frontendURL string, logger *zap.Logger) *OAuthHandler {
	return &OAuthHandler{oauth: oauth, redis: rdb, tokenRepo: tokenRepo, entitlement: ent, campaignID: campaignID, frontendURL: frontendURL, logger: logger}
}

func stateKey(csrf string) string { return "oauth_state:patreon:" + csrf }

// Connect starts the Patreon OAuth flow. Requires a user JWT. Returns the consent
// URL as JSON so the frontend can redirect the browser to it.
func (h *OAuthHandler) Connect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	csrf, err := randomToken()
	if err != nil {
		h.logger.Error("Failed to generate OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start connect flow"})
		return
	}

	// Bind the CSRF token to this user server-side; the callback reads the user id
	// from here (authoritative), not from the URL, so the state cannot be tampered.
	if err := h.redis.Set(c.Request.Context(), stateKey(csrf), userID, stateTTL).Err(); err != nil {
		h.logger.Error("Failed to persist OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start connect flow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auth_url": h.oauth.GetAuthURL(csrf)})
}

// Callback completes the Patreon OAuth flow. Public route: the browser arrives here
// from Patreon with no JWT, so the trust anchor is the Redis-stored state (consumed
// one-time with GETDEL). Redirects to the frontend premium settings page.
func (h *OAuthHandler) Callback(c *gin.Context) {
	ctx := c.Request.Context()
	code := c.Query("code")
	csrf := c.Query("state")
	if code == "" || csrf == "" {
		h.redirect(c, "error")
		return
	}

	userID, err := h.redis.GetDel(ctx, stateKey(csrf)).Result()
	if err != nil || userID == "" {
		h.logger.Warn("Patreon callback with invalid/expired state", zap.Error(err))
		h.redirect(c, "error")
		return
	}

	token, err := h.oauth.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Error("Patreon code exchange failed", zap.String("user_id", userID), zap.Error(err))
		h.redirect(c, "error")
		return
	}

	snap, err := h.oauth.GetIdentityWithMembership(ctx, token.AccessToken, h.campaignID)
	if err != nil {
		h.logger.Error("Patreon identity fetch failed", zap.String("user_id", userID), zap.Error(err))
		h.redirect(c, "error")
		return
	}

	if err := h.tokenRepo.Upsert(ctx, repository.PatreonToken{
		UserID:        userID,
		PatreonUserID: snap.PatreonUserID,
		AccessToken:   token.AccessToken,
		RefreshToken:  token.RefreshToken,
		ExpiresAt:     token.Expiry,
		Scopes:        h.oauth.Scopes(),
	}); err != nil {
		h.logger.Error("Failed to store Patreon token", zap.String("user_id", userID), zap.Error(err))
		h.redirect(c, "error")
		return
	}

	if _, _, err := h.entitlement.Apply(ctx, snap, &userID, nil); err != nil {
		h.logger.Error("Failed to apply Patreon membership", zap.String("user_id", userID), zap.Error(err))
		h.redirect(c, "error")
		return
	}

	h.redirect(c, "connected")
}

func (h *OAuthHandler) redirect(c *gin.Context, result string) {
	c.Redirect(http.StatusFound, fmt.Sprintf("%s/settings/premium?patreon=%s", h.frontendURL, result))
}

// randomToken returns a 32-byte hex CSRF token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
