package handlers

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/source-manager/coordination"
	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AssignmentHandler handles assignment-related API requests
type AssignmentHandler struct {
	registry         *coordination.AssignmentRegistry
	heartbeatMonitor *coordination.HeartbeatMonitor
	metrics          *metrics.ShardMetrics
	logger           *zap.Logger
}

// NewAssignmentHandler creates a new assignment handler
func NewAssignmentHandler(
	registry *coordination.AssignmentRegistry,
	heartbeatMonitor *coordination.HeartbeatMonitor,
	shardMetrics *metrics.ShardMetrics,
	logger *zap.Logger,
) *AssignmentHandler {
	return &AssignmentHandler{
		registry:         registry,
		heartbeatMonitor: heartbeatMonitor,
		metrics:          shardMetrics,
		logger:           logger,
	}
}

// GetAssignments returns all channel assignments for the specified pod
// GET /assignments?pod_id={pod_id}
func (h *AssignmentHandler) GetAssignments(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.metrics.AssignmentQueryLatency.Observe(time.Since(start).Seconds())
	}()

	podID := c.Query("pod_id")
	if podID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pod_id query parameter required"})
		return
	}

	// Query all assignments for this pod from registry
	assignmentPointers, err := h.registry.GetAssignmentsForPod(c.Request.Context(), podID)
	if err != nil {
		h.logger.Error("Failed to query assignments",
			zap.String("pod_id", podID),
			zap.Error(err),
		)
		h.metrics.AssignmentErrors.Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query assignments"})
		return
	}

	// Convert pointers to values for response
	assignments := make([]models.Assignment, len(assignmentPointers))
	for i, ptr := range assignmentPointers {
		assignments[i] = *ptr
	}

	// Temporary DEBUG logging to diagnose assignment retrieval issue
	h.logger.Info("Queried assignments for pod",
		zap.String("pod_id", podID),
		zap.Int("count", len(assignments)),
	)

	c.JSON(http.StatusOK, models.AssignmentResponse{
		Assignments: assignments,
		Count:       len(assignments),
	})
}

// PublishHeartbeat publishes a heartbeat for a listener pod
// POST /heartbeat
func (h *AssignmentHandler) PublishHeartbeat(c *gin.Context) {
	var req models.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	err := h.heartbeatMonitor.PublishHeartbeat(c.Request.Context(), req.PodID)
	if err != nil {
		h.logger.Error("Failed to publish heartbeat",
			zap.String("pod_id", req.PodID),
			zap.Error(err),
		)
		h.metrics.HeartbeatErrors.Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish heartbeat"})
		return
	}

	h.metrics.HeartbeatsPublished.Inc()
	h.logger.Debug("Published heartbeat", zap.String("pod_id", req.PodID))

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
