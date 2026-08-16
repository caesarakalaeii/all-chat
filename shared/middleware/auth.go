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
	"sync"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// revocationLogger is the logger used by JWTAuthWithRevocation for blacklist
// check failures. Defaults to a no-op logger (backward compat with callers that
// never wire one); services call SetLogger at startup to surface failures.
// (audit L1 — previously the blacklist Redis error was silently dropped via
// `_ = err` despite the comment claiming it was logged.)
var (
	revocationLoggerMu sync.RWMutex
	revocationLogger   = zap.NewNop()
)

// SetLogger wires the logger used by JWT blacklist-check failure paths. Services
// should call it once at startup (after their own logger is initialized) so
// revocation Redis errors are emitted instead of silently dropped (audit L1).
func SetLogger(l *zap.Logger) {
	if l == nil {
		l = zap.NewNop()
	}
	revocationLoggerMu.Lock()
	revocationLogger = l
	revocationLoggerMu.Unlock()
}

func revocationLog() *zap.Logger {
	revocationLoggerMu.RLock()
	defer revocationLoggerMu.RUnlock()
	return revocationLogger
}

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

		// Personal access token path (Stream Deck / StreamController desktop plugins).
		// A bearer carrying the allchat_pat_ prefix is a PAT, never a JWT: it is hashed
		// and looked up in api_tokens, and on success populates the SAME context identity
		// the JWT branch below sets, so downstream handlers need no changes.
		//
		// The branch is taken BEFORE the logout blacklist check on purpose: that key is
		// "blacklist:<raw-token>", so routing a PAT through it would write the plaintext
		// token into a Redis command. PAT revocation is a column on the token row and is
		// read live by the resolver, so nothing is lost by skipping the JWT blacklist.
		//
		// Anything that is not a PAT falls through completely unchanged.
		if IsAPIToken(tokenString) {
			if authenticateAPIToken(c, tokenString) {
				c.Next()
			}
			return
		}

		// Check logout blacklist (audit H2). Skip when no Redis client is wired.
		if rdb != nil {
			blacklisted, err := rdb.Exists(c.Request.Context(), "blacklist:"+tokenString).Result()
			if err != nil {
				// Fail-open on Redis errors to avoid locking users out (audit L1).
				// The error is now actually emitted (it was previously dropped via
				// `_ = err` despite the comment claiming it was logged); consider
				// failing closed for admin routes in a future pass.
				revocationLog().Warn("Blacklist check failed (fail-open)",
					zap.Error(err))
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
			c.Set(CtxAuthMethod, AuthMethodJWT)
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
			// Marks the request as session-authenticated so RequireAPITokenScope knows
			// this identity is not scope-limited (a browser session is authorized by the
			// surrounding gates, exactly as before).
			c.Set(CtxAuthMethod, AuthMethodJWT)
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
// Must be used after JWTAuth middleware.
//
// Admin surfaces are SESSION-ONLY: a personal access token is refused here even when it
// belongs to an admin (ADR-0051). A PAT's scopes cover chat and engagement writes, and
// ADR-0049's least-privilege clause says a device credential is "rejected on any route
// outside" its scope — so a token minted for a Stream Deck button must not also reach
// user bans, impersonation or feature-gate flips. This is enforced in one place rather
// than per admin route group, and holds in services that never wire a resolver too
// (where a PAT cannot authenticate at all).
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(CtxAuthMethod) == AuthMethodAPIToken {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Session required",
				"message": "Admin actions require a signed-in session, not a personal access token.",
			})
			c.Abort()
			return
		}

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
