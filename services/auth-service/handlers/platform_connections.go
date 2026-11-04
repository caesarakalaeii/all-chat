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

// Connected platforms (Settings → Connected platforms): list the platforms whose
// OAuth tokens the account holds, and disconnect a linked one. Disconnecting deletes
// the tokens first and only then revokes them at the provider, best effort, so a
// provider outage can never leave a token in our database. What a disconnect does
// to sources is decided per platform in repository.DisconnectPlatform.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TokenRevoker revokes one access or refresh token at the provider that issued it.
type TokenRevoker interface {
	RevokeToken(ctx context.Context, token string) error
}

// PlatformConnectionStore is the subset of the user repository this handler needs.
type PlatformConnectionStore interface {
	ListPlatformConnections(ctx context.Context, userID string) ([]repository.PlatformConnection, error)
	DisconnectPlatform(ctx context.Context, userID, platform string) ([]repository.RemovedToken, error)
}

// PlatformConnectionsHandler serves GET /me/connections and DELETE /me/connections/:platform.
type PlatformConnectionsHandler struct {
	store    PlatformConnectionStore
	revokers map[string]TokenRevoker
	logger   *zap.Logger
}

// NewPlatformConnectionsHandler wires the handler. revokers maps a platform to its
// provider's revoke call; a platform without one (Facebook, Instagram: Meta offers
// no revoke for the stored Page token) is deleted locally only.
func NewPlatformConnectionsHandler(store PlatformConnectionStore, revokers map[string]TokenRevoker, logger *zap.Logger) *PlatformConnectionsHandler {
	return &PlatformConnectionsHandler{store: store, revokers: revokers, logger: logger}
}

// HandleListConnections returns {"connections": [...]}.
func (h *PlatformConnectionsHandler) HandleListConnections(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !RefuseAPIToken(c, "Connected platforms can only be viewed from a signed-in session, not with a token.") {
		return
	}
	connections, err := h.store.ListPlatformConnections(c.Request.Context(), userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if err != nil {
		h.logger.Error("Failed to list platform connections", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list connected platforms"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connections": connections})
}

// HandleDisconnect deletes every token the account holds for a linked platform and
// then revokes them at the provider. Session-only, and refused while impersonating:
// an admin acting as a user must not strip their connections.
func (h *PlatformConnectionsHandler) HandleDisconnect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if c.GetString("impersonated_by") != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Platforms cannot be disconnected while impersonating"})
		return
	}
	if !RefuseAPIToken(c, "Disconnecting a platform requires a signed-in session, not a personal access token.") {
		return
	}

	platform := c.Param("platform")
	removed, err := h.store.DisconnectPlatform(c.Request.Context(), userID, platform)
	switch {
	case errors.Is(err, repository.ErrLoginPlatform):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "login_platform",
			"message": "You sign in with this platform. Its tokens are deleted when you delete your account.",
		})
		return
	case errors.Is(err, repository.ErrUnsupportedPlatform):
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform"})
		return
	case errors.Is(err, repository.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	case err != nil:
		h.logger.Error("Failed to disconnect platform", zap.String("user_id", userID), zap.String("platform", platform), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect"})
		return
	}

	revokeAtProvider(c.Request.Context(), h.revokers[platform], removed, h.logger.With(zap.String("platform", platform)))
	h.logger.Info("Platform disconnected", zap.String("user_id", userID), zap.String("platform", platform),
		zap.Int("credentials_deleted", len(removed)))
	c.JSON(http.StatusOK, gin.H{"message": "Platform disconnected"})
}

// revokeAtProvider revokes each distinct token once, refresh tokens first (for
// Google that revokes the whole grant). It runs after the local delete and only
// logs failures: an expired or already revoked token is refused by the provider,
// and that is not the user's problem.
func revokeAtProvider(ctx context.Context, revoker TokenRevoker, tokens []repository.RemovedToken, logger *zap.Logger) {
	if revoker == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	seen := map[string]bool{}
	for _, t := range tokens {
		for _, token := range []string{t.RefreshToken, t.AccessToken} {
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			if err := revoker.RevokeToken(ctx, token); err != nil {
				logger.Info("Provider did not confirm token revocation", zap.Error(err))
			}
		}
	}
}
