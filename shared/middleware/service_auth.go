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
)

// ServiceJWTAuth enforces service-to-service authentication using signed JWTs.
// Optionally accepts a list of allowed service names. If provided, requests from
// other services will receive a 403 response.
func ServiceJWTAuth(secret string, allowedServices ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, svc := range allowedServices {
		if svc == "" {
			continue
		}
		allowed[svc] = struct{}{}
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		claims, err := auth.ValidateServiceJWT(tokenString, secret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired service token",
			})
			c.Abort()
			return
		}

		if len(allowed) > 0 {
			if _, ok := allowed[claims.ServiceName]; !ok {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "service not permitted",
				})
				c.Abort()
				return
			}
		}

		c.Set("service_name", claims.ServiceName)
		c.Next()
	}
}
