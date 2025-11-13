package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/source-controller/election"
	"github.com/caesar/all-chat/services/source-controller/registry"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	registry      *registry.Registry
	leaderManager *election.Manager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(registry *registry.Registry, leaderManager *election.Manager) *HealthHandler {
	return &HealthHandler{
		registry:      registry,
		leaderManager: leaderManager,
	}
}

// LivenessProbe handles the liveness probe
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// ReadinessProbe handles the readiness probe
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	// Check if registry has synced at least once
	stats := h.registry.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"registry": stats,
	})
}

// Status returns detailed status information
func (h *HealthHandler) Status(c *gin.Context) {
	stats := h.registry.GetStats()

	leadership, err := h.leaderManager.GetAllLeadership(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get leadership status",
		})
		return
	}

	// Count leadership by status
	leaderCount := 0
	for _, status := range leadership {
		if status.IsLeader {
			leaderCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "running",
		"instance_id": h.leaderManager.GetInstanceID(),
		"registry":    stats,
		"leadership": gin.H{
			"total_streams": len(leadership),
			"leader_count":  leaderCount,
			"streams":       leadership,
		},
	})
}
