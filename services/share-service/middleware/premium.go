package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RequirePremium returns middleware that validates user has premium subscription
// User decision: No caching, query database on every request for MVP simplicity
func RequirePremium(db *pgxpool.Pool, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user_id from context (set by JWTAuth middleware)
		userID := c.GetString("user_id")
		if userID == "" {
			logger.Warn("Premium check failed: no user_id in context")
			c.JSON(401, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Query database for is_premium column (no caching per user decision)
		var isPremium bool
		err := db.QueryRow(c.Request.Context(),
			"SELECT is_premium FROM users WHERE id = $1", userID).Scan(&isPremium)

		if err != nil {
			logger.Error("Failed to verify premium status",
				zap.String("user_id", userID),
				zap.Error(err))
			c.JSON(500, gin.H{
				"error": "failed to verify premium status",
			})
			c.Abort()
			return
		}

		if !isPremium {
			logger.Info("Premium feature access denied",
				zap.String("user_id", userID),
				zap.String("path", c.Request.URL.Path))
			c.JSON(403, gin.H{
				"error":       "Premium feature required",
				"message":     "Share requests are a premium feature. Upgrade your account to access this functionality.",
				"upgrade_url": "/upgrade", // Placeholder for future billing page
			})
			c.Abort()
			return
		}

		// User is premium, continue to handler
		logger.Debug("Premium check passed", zap.String("user_id", userID))
		c.Next()
	}
}
