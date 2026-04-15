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
	"strings"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// ViewerJWTAuth returns a middleware that validates viewer JWT tokens
func ViewerJWTAuth(jwtSecret string) gin.HandlerFunc {
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

		// Validate token as viewer JWT
		claims, err := auth.ValidateViewerJWT(token, jwtSecret)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store viewer session ID and other claims in context for use by handlers
		c.Set("session_id", claims.SessionID)
		c.Set("platform", claims.Platform)
		c.Set("platform_user_id", claims.PlatformUserID)
		c.Set("username", claims.Username)
		c.Set("is_viewer", claims.IsViewer)

		c.Next()
	}
}
