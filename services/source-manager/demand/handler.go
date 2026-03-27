package demand

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DemandHandler provides the HTTP handler for the GET /demand endpoint.
// Listeners use this as a fallback poll when Pub/Sub is unavailable.
type DemandHandler struct {
	subscriber *OverlayDemandSubscriber
}

// NewDemandHandler creates a new DemandHandler.
func NewDemandHandler(subscriber *OverlayDemandSubscriber) *DemandHandler {
	return &DemandHandler{subscriber: subscriber}
}

// GetDemand handles GET /demand[?platform=<platform>].
// When a platform query param is provided, only sources for that platform are returned.
// Otherwise all demanded sources are returned.
// Response format: {"sources": [...DemandSource]}
func (h *DemandHandler) GetDemand(c *gin.Context) {
	platform := c.Query("platform")

	var sources []DemandSource
	if platform != "" {
		sources = h.subscriber.GetDemandedSourcesByPlatform(platform)
	} else {
		sources = h.subscriber.GetDemandedSources()
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
	})
}
