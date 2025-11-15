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
