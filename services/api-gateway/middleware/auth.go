package middleware

import (
	"strings"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// JWTAuth returns a middleware that validates JWT tokens

func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(401, gin.H{"error": "invalid authorization format, expected 'Bearer <token>'"})
			c.Abort()
			return
		}

		// Extract token value
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			c.JSON(401, gin.H{"error": "empty token"})
			c.Abort()
			return
		}

		// Validate token using shared auth package
		claims, err := auth.ValidateJWT(token, jwtSecret)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store user ID and other claims in context for use by handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("twitch_id", claims.TwitchID)

		c.Next()
	}
}
