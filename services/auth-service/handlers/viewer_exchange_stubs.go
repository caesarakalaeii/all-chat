package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleTwitchExchange handles POST /viewer/twitch/exchange.
// Wave 0 stub: not yet implemented. Plan 02 will provide the real implementation.
func (h *ViewerAuthHandler) HandleTwitchExchange(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// HandleYouTubeExchange handles POST /viewer/youtube/exchange.
// Wave 0 stub: not yet implemented. Plan 02 will provide the real implementation.
func (h *ViewerAuthHandler) HandleYouTubeExchange(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// HandleKickExchange handles POST /viewer/kick/exchange.
// Wave 0 stub: not yet implemented. Plan 02 will provide the real implementation.
func (h *ViewerAuthHandler) HandleKickExchange(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
