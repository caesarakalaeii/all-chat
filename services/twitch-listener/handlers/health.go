package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/channels"
	"github.com/caesar/all-chat/services/twitch-listener/irc"
	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	ircConn   *irc.ConnectionManager
	publisher *publisher.StreamPublisher
	chanMgr   *channels.Manager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	ircConn *irc.ConnectionManager,
	pub *publisher.StreamPublisher,
	chanMgr *channels.Manager,
) *HealthHandler {
	return &HealthHandler{
		ircConn:   ircConn,
		publisher: pub,
		chanMgr:   chanMgr,
	}
}

// LivenessProbe checks if the service is alive (HTTP 200 = alive)
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "twitch-listener",
	})
}

// ReadinessProbe checks if the service is ready to handle requests
// Checks: IRC connection, Redis connection, coordinator assignments, active channels
// Per CONTEXT.md: Pod reports ready AFTER successfully connecting to all assigned channels
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	ready := true
	reason := ""

	// Check 1: IRC connection established
	ircConnected := h.ircConn.IsConnected()
	checks["irc_connected"] = ircConnected
	if !ircConnected {
		ready = false
		reason = "IRC not connected"
	}

	// Expose last activity for observability (not a readiness gate — quiet channels
	// may legitimately have no messages for extended periods, and go-twitch-irc handles
	// PING/PONG internally without exposing a callback we could track).
	lastActivity := h.ircConn.LastActivityAt()
	if !lastActivity.IsZero() {
		checks["irc_last_activity_seconds_ago"] = int(time.Since(lastActivity).Seconds())
	}

	// Check 2: Redis connection
	redisErr := h.publisher.Ping(ctx)
	checks["redis_connected"] = redisErr == nil
	if redisErr != nil {
		ready = false
		checks["redis_error"] = redisErr.Error()
		if reason == "" {
			reason = "Redis not connected"
		}
	}

	// Check 3: Assignments received from coordinator (TWITCH-06, TWITCH-07)
	// Skip when coordination is disabled (SOURCE_MANAGER_SECRET not set) — in that case
	// Twitch IRC operates without assignment filtering (all channels always joined).
	coordinationEnabled := h.chanMgr.IsCoordinationEnabled()
	checks["coordination_enabled"] = coordinationEnabled
	if coordinationEnabled {
		assignmentCount := h.chanMgr.GetAssignmentCount()
		checks["assignments"] = assignmentCount
		if assignmentCount == 0 {
			ready = false
			if reason == "" {
				reason = "no assignments from coordinator"
			}
		}
	}

	// Check 4: Verify IRC channels are initialised.
	//
	// When leadership election is enabled (SOURCE_MANAGER_SECRET set) multiple pods
	// split channels between them via Redis locks.  A pod that wins 0 locks is still
	// healthy — the peer holds them all.  Requiring activeChannelCount >= filteredAssignmentCount
	// would permanently fail the second pod, preventing the rolling deploy from completing.
	//
	// Therefore: when leadership is enabled, require only that the initial SyncChannels
	// call has finished (i.e. the pod attempted leadership election and set its activeChans).
	// When leadership is disabled (single-pod mode), keep the strict channel count check.
	activeChannelCount := h.chanMgr.GetActiveChannelCount()
	filteredAssignmentCount := h.chanMgr.GetFilteredAssignmentCount()
	checks["active_channels"] = activeChannelCount
	if h.chanMgr.IsLeadershipEnabled() {
		// Leadership mode: ready once the initial sync completed (0 active channels is valid).
		if !h.chanMgr.IsInitialSyncComplete() {
			ready = false
			if reason == "" {
				reason = "initial channel sync not yet complete"
			}
		}
	} else {
		// Single-pod mode: all filtered channels must be active.
		if activeChannelCount < filteredAssignmentCount {
			ready = false
			if reason == "" {
				reason = "channels connecting"
			}
			checks["expected"] = filteredAssignmentCount
			checks["connected"] = activeChannelCount
		}
	}

	status := "ready"
	statusCode := http.StatusOK
	if !ready {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
		checks["reason"] = reason
	}

	c.JSON(statusCode, gin.H{
		"status": status,
		"checks": checks,
	})
}

// Status provides detailed status information
func (h *HealthHandler) Status(c *gin.Context) {
	activeChannels := h.chanMgr.GetActiveChannels()

	lastAct := h.ircConn.LastActivityAt()
	var lastActivityAgo string
	if !lastAct.IsZero() {
		lastActivityAgo = time.Since(lastAct).Truncate(time.Second).String()
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"irc": gin.H{
			"connected":         h.ircConn.IsConnected(),
			"active_channels":   len(activeChannels),
			"channels":          activeChannels,
			"last_activity_ago": lastActivityAgo,
		},
	})
}
