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
	"encoding/json"
	"net/http"
	"strings"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HandleStopImpersonation restores the admin session after an impersonation.
// POST /auth/stop-impersonation (protected; requires an impersonated JWT).
// Reads the current access token from X-Access-Token (forwarded by the
// gateway AuthCookieForward middleware; the raw Cookie is stripped before it
// reaches auth-service). If the token carries an impersonated_by claim, looks
// up the stashed admin identity (impersonation:<jti>), issues a fresh admin
// access JWT, and sets it as the access cookie. The stash is consumed
// (GetDel) so a stop is single-use. (audit H3)
func (h *AuthHandler) HandleStopImpersonation(c *gin.Context) {
	token := c.GetHeader("X-Access-Token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	claims, err := auth.ValidateJWTWithKeyChain(token, h.userKeyChain)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if claims.ImpersonatedBy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not currently impersonating"})
		return
	}

	// Look up the stashed admin identity (single-use: GetDel consumes it).
	stashKey := "impersonation:" + claims.ID
	data, err := h.redis.GetDel(c.Request.Context(), stashKey).Result()
	if err != nil {
		h.logger.Warn("Impersonation stash missing on stop", zap.String("jti", claims.ID), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "impersonation session expired"})
		return
	}
	var stash struct {
		AdminUserID   string `json:"admin_user_id"`
		AdminUsername string `json:"admin_username"`
	}
	if err := json.Unmarshal([]byte(data), &stash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Fetch the admin user (re-validates existence + is_admin) and issue a
	// fresh admin access JWT.
	adminUser, err := h.userRepo.GetUserByID(c.Request.Context(), stash.AdminUserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "admin user not found"})
		return
	}
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), adminUser.ID, adminUser.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, adminUser.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     auth.CookieAccessToken,
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.jwtExpiry.Seconds()),
	})

	// I3: blacklist the impersonation JWT so it can't be replayed after the
	// admin stops impersonating. Previously the impersonation token stayed
	// valid until natural JWT expiry — a leaked/stolen impersonation token
	// remained usable to act as the target user for the full access-token
	// lifetime. Mirrors HandleLogout's blacklist (TTL = jwtExpiry).
	if token != "" {
		if err := h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry).Err(); err != nil {
			h.logger.Warn("Failed to blacklist impersonation JWT on stop",
				zap.String("jti", claims.ID),
				zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": adminUser.ID, "username": adminUser.Username, "is_admin": adminUser.IsAdmin},
	})
}
