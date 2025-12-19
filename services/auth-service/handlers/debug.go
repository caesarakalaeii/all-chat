package handlers

import (
	"net/http"
	"strings"

	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DebugHandler provides debug endpoints for testing
type DebugHandler struct {
	log       *zap.Logger
	jwtSecret string
}

// NewDebugHandler creates a new debug handler
func NewDebugHandler(log *zap.Logger, jwtSecret string) *DebugHandler {
	return &DebugHandler{
		log:       log.Named("debug"),
		jwtSecret: jwtSecret,
	}
}

// HandleTestViewerJWT tests viewer JWT validation
func (h *DebugHandler) HandleTestViewerJWT(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization header required"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Try user token
	userClaims, userErr := sharedAuth.ValidateJWT(tokenString, h.jwtSecret)

	// Try viewer token
	viewerClaims, viewerErr := sharedAuth.ValidateViewerJWT(tokenString, h.jwtSecret)

	c.JSON(http.StatusOK, gin.H{
		"user_validation": gin.H{
			"success": userErr == nil,
			"error":   errToString(userErr),
			"claims":  userClaims,
		},
		"viewer_validation": gin.H{
			"success": viewerErr == nil,
			"error":   errToString(viewerErr),
			"claims":  viewerClaims,
		},
	})
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
