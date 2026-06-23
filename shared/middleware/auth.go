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

package middleware

import (
	"net/http"
	"strings"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// JWTAuth returns a gin middleware that validates JWT tokens using a KeyChain.
// The KeyChain dispatches by kid header for versioned secrets and falls back to
// the legacy secret for tokens without a kid (D-08). Non-HMAC tokens are
// rejected outright (D-12).
//
// This overload does NOT check the logout blacklist. Use JWTAuthWithRevocation
// to enforce token revocation (audit H2).
func JWTAuth(kc *auth.KeyChain) gin.HandlerFunc {
	return JWTAuthWithRevocation(kc, nil)
}

// JWTAuthWithRevocation is identical to JWTAuth but also checks the Redis-backed
// logout blacklist before accepting a token. If rdb is nil the blacklist check
// is skipped (backward-compatible with callers that have no Redis client).
//
// The blacklist key format is "blacklist:<raw-token>" and is written by
// auth-service HandleLogout / HandleDeleteAccount (audit H2).
func JWTAuthWithRevocation(kc *auth.KeyChain, rdb redis.UniversalClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		// Check logout blacklist (audit H2). Skip when no Redis client is wired.
		if rdb != nil {
			blacklisted, err := rdb.Exists(c.Request.Context(), "blacklist:"+tokenString).Result()
			if err != nil {
				// Fail-open on Redis errors to avoid locking users out; the error
				// is logged but not surfaced to the client.
				_ = err
			} else if blacklisted > 0 {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Token has been revoked",
				})
				c.Abort()
				return
			}
		}

		// Try to validate as viewer token first (more specific)
		viewerClaims, err := auth.ValidateViewerJWTWithKeyChain(tokenString, kc)
		if err == nil && viewerClaims.IsViewer {
			// Viewer token
			c.Set("viewer_id", viewerClaims.ViewerID)
			c.Set("session_id", viewerClaims.SessionID)
			c.Set("username", viewerClaims.Username)
			c.Set("display_name", viewerClaims.DisplayName)
			c.Set("avatar_url", viewerClaims.AvatarURL)
			c.Set("platform", viewerClaims.Platform)
			c.Set("platform_user_id", viewerClaims.PlatformUserID)
			c.Set("is_viewer", viewerClaims.IsViewer)
			c.Set("is_premium", viewerClaims.IsPremium)
			c.Set("is_admin", viewerClaims.IsAdmin)
			c.Next()
			return
		}

		// Try to validate as regular user token
		claims, err := auth.ValidateJWTWithKeyChain(tokenString, kc)
		if err == nil {
			// Regular user token. For impersonation tokens UserID is already the
			// target (effective) user, so ownership checks keyed on "user_id" behave
			// as the impersonated user — which is intended.
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("twitch_id", claims.TwitchID)
			c.Set("roles", claims.Roles)
			// Impersonation provenance (ADR-0017). Empty unless an admin is acting as
			// this user; downstream services (e.g. moderation-service) attribute the
			// real admin in their audit log while the action runs as the target user.
			c.Set("impersonated_by", claims.ImpersonatedBy)
			c.Set("impersonated_user", claims.ImpersonatedUser)
			c.Next()
			return
		}

		// Both validations failed
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired token",
		})
		c.Abort()
	}
}

// AdminOnly middleware checks if the authenticated user has admin role
// Must be used after JWTAuth middleware
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get roles from context (set by JWTAuth middleware)
		rolesInterface, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			c.Abort()
			return
		}

		roles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid roles format",
			})
			c.Abort()
			return
		}

		// Check if user has admin role
		hasAdmin := false
		for _, role := range roles {
			if role == "admin" {
				hasAdmin = true
				break
			}
		}

		if !hasAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
