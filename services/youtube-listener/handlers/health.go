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

package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/youtube-listener/publisher"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/services/youtube-listener/streams"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	streamManager *streams.Manager
	publisher     *publisher.StreamPublisher
	quotaTracker  *quota.Tracker
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	streamManager *streams.Manager,
	publisher *publisher.StreamPublisher,
	quotaTracker *quota.Tracker,
) *HealthHandler {
	return &HealthHandler{
		streamManager: streamManager,
		publisher:     publisher,
		quotaTracker:  quotaTracker,
	}
}

// LivenessProbe handles the liveness probe
// Always returns 200 OK if the service is running
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// ReadinessProbe handles the readiness probe
// Checks if Redis connection is healthy
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx := c.Request.Context()

	// Check Redis connection
	if err := h.publisher.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "redis connection failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// Status returns detailed status information
func (h *HealthHandler) Status(c *gin.Context) {
	activeStreams := h.streamManager.GetActiveStreams()

	quotaUsed := h.quotaTracker.GetUsageToday()
	quotaRemaining := h.quotaTracker.GetRemainingQuota()
	quotaPercentage := h.quotaTracker.GetUsagePercentage()

	streamInfo := make([]gin.H, 0, len(activeStreams))
	for _, stream := range activeStreams {
		streamInfo = append(streamInfo, gin.H{
			"stream_id":        stream.StreamID,
			"channel_id":       stream.ChannelID,
			"channel_name":     stream.ChannelName,
			"is_live":          stream.IsLive,
			"polling_interval": stream.PollingInterval,
			"last_polled_at":   stream.LastPolledAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "running",
		"streams": gin.H{
			"active_count": len(activeStreams),
			"streams":      streamInfo,
		},
		"quota": gin.H{
			"used":       quotaUsed,
			"remaining":  quotaRemaining,
			"percentage": quotaPercentage,
		},
	})
}
