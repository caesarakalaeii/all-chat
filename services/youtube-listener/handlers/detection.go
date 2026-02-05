package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/services/youtube-listener/streams"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DetectionManager interface for accessing stream manager detection state
type DetectionManager interface {
	GetChannelDetectionState(channelID string) (*streams.ChannelDetectionState, error)
	GetAllChannelStates() ([]*streams.ChannelDetectionState, error)
	ResetChannelBackoff(ctx context.Context, channelID string) error
	ForceChannelDetection(ctx context.Context, channelID string) error
	ResetAllBackoff(ctx context.Context) error
	GetQuotaBudgetSummary() map[string]interface{}
}

// DetectionHandler handles detection control and inspection endpoints
type DetectionHandler struct {
	manager      DetectionManager
	quotaBudget  *streams.QuotaBudget
	quotaTracker *quota.Tracker
	logger       *zap.Logger
}

// NewDetectionHandler creates a new detection handler
func NewDetectionHandler(
	manager DetectionManager,
	quotaBudget *streams.QuotaBudget,
	quotaTracker *quota.Tracker,
	logger *zap.Logger,
) *DetectionHandler {
	return &DetectionHandler{
		manager:      manager,
		quotaBudget:  quotaBudget,
		quotaTracker: quotaTracker,
		logger:       logger,
	}
}

// GetChannelState returns the detection state for a specific channel
// GET /admin/detection/channels/:channel_id
func (h *DetectionHandler) GetChannelState(c *gin.Context) {
	channelID := c.Param("channel_id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel_id is required"})
		return
	}

	state, err := h.manager.GetChannelDetectionState(channelID)
	if err != nil {
		h.logger.Error("Failed to get channel detection state",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, state)
}

// ListAllChannelStates returns detection state for all tracked channels
// GET /admin/detection/channels
func (h *DetectionHandler) ListAllChannelStates(c *gin.Context) {
	// Optional filters
	filterRisk := c.Query("risk") // high/medium/low
	filterStuck := c.Query("stuck") == "true"

	states, err := h.manager.GetAllChannelStates()
	if err != nil {
		h.logger.Error("Failed to get all channel states", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply filters
	filtered := make([]*streams.ChannelDetectionState, 0)
	for _, state := range states {
		if filterRisk != "" && state.RiskLevel != filterRisk {
			continue
		}

		if filterStuck {
			// Consider "stuck" if backoff > 5min and has connected overlays
			isStuck := false
			if state.BackoffState != nil && state.ConnectedOverlays > 0 {
				if state.BackoffState.CurrentInterval > 5*time.Minute {
					isStuck = true
				}
			}
			if !isStuck {
				continue
			}
		}

		filtered = append(filtered, state)
	}

	// Add summary
	response := gin.H{
		"channels": filtered,
		"summary": gin.H{
			"total":          len(states),
			"filtered":       len(filtered),
			"quota_budget":   h.quotaBudget.GetBudgetSummary(),
			"global_quota": gin.H{
				"state":      h.quotaTracker.GetState(),
				"remaining":  h.quotaTracker.GetRemainingQuota(),
				"percentage": h.quotaTracker.GetUsagePercentage(),
			},
		},
	}

	c.JSON(http.StatusOK, response)
}

// ResetChannelBackoff resets backoff for a specific channel
// POST /admin/detection/channels/:channel_id/reset-backoff
func (h *DetectionHandler) ResetChannelBackoff(c *gin.Context) {
	channelID := c.Param("channel_id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel_id is required"})
		return
	}

	ctx := c.Request.Context()
	if err := h.manager.ResetChannelBackoff(ctx, channelID); err != nil {
		h.logger.Error("Failed to reset channel backoff",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Manually reset channel backoff",
		zap.String("channel_id", channelID),
		zap.String("action", "admin_reset_backoff"),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Backoff reset successfully",
		"channel_id": channelID,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// ForceChannelDetection forces immediate detection for a channel
// POST /admin/detection/channels/:channel_id/force-check
// NOTE: This bypasses quota budget but logs and counts as manual operation
func (h *DetectionHandler) ForceChannelDetection(c *gin.Context) {
	channelID := c.Param("channel_id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel_id is required"})
		return
	}

	// Check if quota is critically low
	remaining := h.quotaTracker.GetRemainingQuota()
	if remaining < 200 {
		h.logger.Warn("Force detection requested but quota critically low",
			zap.String("channel_id", channelID),
			zap.Int("remaining_quota", remaining),
		)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":           "Quota critically low - manual operations disabled",
			"remaining_quota": remaining,
		})
		return
	}

	ctx := c.Request.Context()
	if err := h.manager.ForceChannelDetection(ctx, channelID); err != nil {
		h.logger.Error("Failed to force channel detection",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Record manual operation in quota budget (100 units for full detection)
	h.quotaBudget.RecordManualOperation(ctx, 100, fmt.Sprintf("force_detection:%s", channelID))

	h.logger.Info("Manually forced channel detection",
		zap.String("channel_id", channelID),
		zap.String("action", "admin_force_detection"),
		zap.Int("quota_used", 100),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Detection triggered successfully (bypassed quota budget)",
		"channel_id": channelID,
		"quota_used": 100,
		"timestamp":  time.Now().Format(time.RFC3339),
		"warning":    "Manual operation counted against manual quota reserve",
	})
}

// ResetAllBackoff resets backoff for all channels (emergency use)
// POST /admin/detection/reset-all
func (h *DetectionHandler) ResetAllBackoff(c *gin.Context) {
	// Require confirmation parameter for safety
	confirm := c.Query("confirm")
	if confirm != "yes" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Confirmation required",
			"message": "Add ?confirm=yes to confirm reset of all channels",
		})
		return
	}

	ctx := c.Request.Context()
	if err := h.manager.ResetAllBackoff(ctx); err != nil {
		h.logger.Error("Failed to reset all backoff", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Warn("Manually reset ALL channel backoff",
		zap.String("action", "admin_reset_all_backoff"),
		zap.String("confirmed", "yes"),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":   "All channel backoff reset successfully",
		"timestamp": time.Now().Format(time.RFC3339),
		"warning":   "All channels will check immediately on next detection cycle",
	})
}

// GetQuotaBudgetStatus returns quota budget status
// GET /admin/detection/quota-budget
func (h *DetectionHandler) GetQuotaBudgetStatus(c *gin.Context) {
	summary := h.quotaBudget.GetBudgetSummary()

	c.JSON(http.StatusOK, gin.H{
		"quota_budget": summary,
		"timestamp":    time.Now().Format(time.RFC3339),
	})
}
