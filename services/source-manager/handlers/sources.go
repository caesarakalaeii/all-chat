package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/source-manager/election"
	"github.com/caesar/all-chat/services/source-manager/registry"
	"github.com/gin-gonic/gin"
)

// SourceHandler handles source-related API requests
type SourceHandler struct {
	registry      *registry.Registry
	leaderManager *election.Manager
}

// NewSourceHandler creates a new source handler
func NewSourceHandler(registry *registry.Registry, leaderManager *election.Manager) *SourceHandler {
	return &SourceHandler{
		registry:      registry,
		leaderManager: leaderManager,
	}
}

// GetSources returns active sources for a platform
func (h *SourceHandler) GetSources(c *gin.Context) {
	platform := c.Query("platform")

	if platform == "" {
		// Return all sources
		sources := h.registry.GetAllSources()
		c.JSON(http.StatusOK, gin.H{
			"sources": sources,
			"count":   len(sources),
		})
		return
	}

	// Return sources for specific platform
	sources := h.registry.GetSourcesByPlatform(platform)
	c.JSON(http.StatusOK, gin.H{
		"platform": platform,
		"sources":  sources,
		"count":    len(sources),
	})
}

// ClaimLeadership attempts to claim leadership for a source
func (h *SourceHandler) ClaimLeadership(c *gin.Context) {
	var req struct {
		Platform string `json:"platform" binding:"required"`
		StreamID string `json:"stream_id" binding:"required"`
		CallerID string `json:"caller_id"` // stable identity of the requesting service instance
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	acquired, err := h.leaderManager.TryAcquireLeadership(c.Request.Context(), req.Platform, req.StreamID, req.CallerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to claim leadership",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"acquired":  acquired,
		"platform":  req.Platform,
		"stream_id": req.StreamID,
	})
}

// RenewLeadership renews leadership for a source
func (h *SourceHandler) RenewLeadership(c *gin.Context) {
	var req struct {
		Platform string `json:"platform" binding:"required"`
		StreamID string `json:"stream_id" binding:"required"`
		CallerID string `json:"caller_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	renewed, err := h.leaderManager.RenewLeadership(c.Request.Context(), req.Platform, req.StreamID, req.CallerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to renew leadership",
		})
		return
	}

	if !renewed {
		c.JSON(http.StatusGone, gin.H{
			"error":   "leadership lost",
			"renewed": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"renewed":   true,
		"platform":  req.Platform,
		"stream_id": req.StreamID,
	})
}

// ReleaseLeadership releases leadership for a source
func (h *SourceHandler) ReleaseLeadership(c *gin.Context) {
	var req struct {
		Platform string `json:"platform" binding:"required"`
		StreamID string `json:"stream_id" binding:"required"`
		CallerID string `json:"caller_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
		})
		return
	}

	err := h.leaderManager.ReleaseLeadership(c.Request.Context(), req.Platform, req.StreamID, req.CallerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to release leadership",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"released":  true,
		"platform":  req.Platform,
		"stream_id": req.StreamID,
	})
}

// GetLeadershipStatus returns leadership status
func (h *SourceHandler) GetLeadershipStatus(c *gin.Context) {
	platform := c.Query("platform")
	streamID := c.Query("stream_id")

	if platform == "" || streamID == "" {
		// Return all leadership statuses
		statuses, err := h.leaderManager.GetAllLeadership(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to get leadership status",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"leadership": statuses,
			"count":      len(statuses),
		})
		return
	}

	// Return specific leadership status
	status, err := h.leaderManager.GetLeadership(c.Request.Context(), platform, streamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get leadership status",
		})
		return
	}

	c.JSON(http.StatusOK, status)
}
