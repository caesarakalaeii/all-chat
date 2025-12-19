package middleware

import (
	"net/http"
	"strings"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// JWTAuth returns a gin middleware that validates JWT tokens
func JWTAuth(secret string) gin.HandlerFunc {
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

		// Try to validate as regular user token first
		claims, err := auth.ValidateJWT(tokenString, secret)
		if err == nil {
			// Regular user token
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("twitch_id", claims.TwitchID)
			c.Set("roles", claims.Roles)
			c.Next()
			return
		}

		// Try to validate as viewer token
		viewerClaims, err := auth.ValidateViewerJWT(tokenString, secret)
		if err == nil {
			// Viewer token
			c.Set("session_id", viewerClaims.SessionID)
			c.Set("username", viewerClaims.Username)
			c.Set("platform", viewerClaims.Platform)
			c.Set("platform_user_id", viewerClaims.PlatformUserID)
			c.Set("is_viewer", viewerClaims.IsViewer)
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
