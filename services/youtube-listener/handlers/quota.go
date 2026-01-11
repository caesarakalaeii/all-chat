package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// QuotaHandler handles quota-related admin API requests
type QuotaHandler struct {
	coordinator       *quota.Coordinator
	globalTracker     *quota.Tracker
	perChannelTracker *quota.PerChannelTracker
	logger            *zap.Logger
}

// NewQuotaHandler creates a new quota handler
func NewQuotaHandler(
	coordinator *quota.Coordinator,
	globalTracker *quota.Tracker,
	perChannelTracker *quota.PerChannelTracker,
	logger *zap.Logger,
) *QuotaHandler {
	return &QuotaHandler{
		coordinator:       coordinator,
		globalTracker:     globalTracker,
		perChannelTracker: perChannelTracker,
		logger:            logger,
	}
}

// GlobalQuotaStatus represents the global quota status response
type GlobalQuotaStatus struct {
	State            string    `json:"state"`
	Used             int       `json:"used"`
	Limit            int       `json:"limit"`
	Remaining        int       `json:"remaining"`
	Percentage       float64   `json:"percentage"`
	LastTransition   time.Time `json:"last_transition"`
	ResetsAt         string    `json:"resets_at"`
	PollingMultiplier float64  `json:"polling_multiplier"`
}

// ChannelQuotaStatus represents per-channel quota status
type ChannelQuotaStatus struct {
	ChannelID  string  `json:"channel_id"`
	Used       int     `json:"used"`
	Limit      int     `json:"limit"`
	Remaining  int     `json:"remaining"`
	Percentage float64 `json:"percentage"`
}

// QuotaStatusResponse represents the complete quota status
type QuotaStatusResponse struct {
	Global   GlobalQuotaStatus    `json:"global"`
	Channels []ChannelQuotaStatus `json:"channels"`
}

// GetQuotaStatus returns the current quota status (global + channels)
// GET /quota/status
func (h *QuotaHandler) GetQuotaStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Get global quota info
	state, percentage, lastTransition := h.globalTracker.GetStateInfo()
	used := h.globalTracker.GetUsageToday()
	remaining := h.globalTracker.GetRemainingQuota()
	limit := used + remaining

	// Calculate reset time (midnight PST - YouTube's quota reset timezone)
	now := time.Now().In(quota.YouTubePST)
	tomorrow := now.AddDate(0, 0, 1)
	midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, quota.YouTubePST)
	resetsAt := midnight.Format(time.RFC3339)

	// Get polling multiplier
	pollingMultiplier := h.coordinator.GetPollingDelayMultiplier()

	global := GlobalQuotaStatus{
		State:             string(state),
		Used:              used,
		Limit:             limit,
		Remaining:         remaining,
		Percentage:        percentage,
		LastTransition:    lastTransition,
		ResetsAt:          resetsAt,
		PollingMultiplier: pollingMultiplier,
	}

	// Get per-channel quota status by querying database directly
	channelStatuses := make([]ChannelQuotaStatus, 0)

	query := `
		SELECT channel_id, daily_quota_used, daily_quota_limit
		FROM youtube_channel_quota
		WHERE daily_quota_used > 0
		ORDER BY daily_quota_used DESC
		LIMIT 100
	`

	rows, err := h.perChannelTracker.GetDB().Query(ctx, query)
	if err != nil {
		h.logger.Error("Failed to query channel quotas", zap.Error(err))
	} else {
		defer rows.Close()

		for rows.Next() {
			var channelID string
			var used, limit int
			if err := rows.Scan(&channelID, &used, &limit); err != nil {
				h.logger.Error("Failed to scan channel quota", zap.Error(err))
				continue
			}

			remaining := limit - used
			if remaining < 0 {
				remaining = 0
			}
			percentage := float64(used) / float64(limit) * 100

			channelStatuses = append(channelStatuses, ChannelQuotaStatus{
				ChannelID:  channelID,
				Used:       used,
				Limit:      limit,
				Remaining:  remaining,
				Percentage: percentage,
			})
		}
	}

	response := QuotaStatusResponse{
		Global:   global,
		Channels: channelStatuses,
	}

	c.JSON(http.StatusOK, response)

	h.logger.Debug("Quota status retrieved",
		zap.String("request_from", c.ClientIP()),
	)
}

// GetChannelQuota returns quota status for a specific channel
// GET /quota/channels/:channel_id
func (h *QuotaHandler) GetChannelQuota(c *gin.Context) {
	ctx := c.Request.Context()
	channelID := c.Param("channel_id")

	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "channel_id is required",
		})
		return
	}

	quota, err := h.perChannelTracker.GetChannelQuota(ctx, channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get channel quota",
		})
		return
	}

	used := quota.DailyQuotaUsed
	limit := quota.DailyQuotaLimit
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	percentage := float64(used) / float64(limit) * 100

	response := ChannelQuotaStatus{
		ChannelID:  channelID,
		Used:       used,
		Limit:      limit,
		Remaining:  remaining,
		Percentage: percentage,
	}

	c.JSON(http.StatusOK, response)

	h.logger.Debug("Channel quota retrieved",
		zap.String("channel_id", channelID),
		zap.String("request_from", c.ClientIP()),
	)
}

// QuotaHistoryEntry represents a historical quota data point
type QuotaHistoryEntry struct {
	Date       string  `json:"date"`
	UnitsUsed  int     `json:"units_used"`
	UnitsLimit int     `json:"units_limit"`
	Percentage float64 `json:"percentage"`
}

// QuotaHistoryResponse represents historical quota usage
type QuotaHistoryResponse struct {
	History []QuotaHistoryEntry `json:"history"`
}

// GetQuotaHistory returns historical quota usage data
// GET /quota/history?days=7
func (h *QuotaHandler) GetQuotaHistory(c *gin.Context) {
	// Get days parameter (default: 7 days)
	days := 7
	if daysParam := c.Query("days"); daysParam != "" {
		if parsedDays, err := parseIntParam(daysParam); err == nil && parsedDays > 0 && parsedDays <= 30 {
			days = parsedDays
		}
	}

	// Query database for historical data
	// For now, return mock data since we need to implement the database query
	// TODO: Implement actual database query
	history := make([]QuotaHistoryEntry, 0)

	// Generate last N days
	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		// This is mock data - should query from youtube_quota_usage table
		history = append(history, QuotaHistoryEntry{
			Date:       date,
			UnitsUsed:  0,
			UnitsLimit: 10000,
			Percentage: 0.0,
		})
	}

	response := QuotaHistoryResponse{
		History: history,
	}

	c.JSON(http.StatusOK, response)

	h.logger.Debug("Quota history retrieved",
		zap.Int("days", days),
		zap.String("request_from", c.ClientIP()),
	)
}

// QuotaPredictionResponse represents quota prediction/forecast
type QuotaPredictionResponse struct {
	CurrentUsage        int       `json:"current_usage"`
	CurrentPercentage   float64   `json:"current_percentage"`
	EstimatedDailyUsage int       `json:"estimated_daily_usage"`
	EstimatedDepletion  *string   `json:"estimated_depletion"` // ISO timestamp or null
	WillDeplete         bool      `json:"will_deplete"`
	Recommendation      string    `json:"recommendation"`
}

// GetQuotaPrediction returns quota usage predictions
// GET /quota/predictions
func (h *QuotaHandler) GetQuotaPrediction(c *gin.Context) {
	used := h.globalTracker.GetUsageToday()
	limit := used + h.globalTracker.GetRemainingQuota()
	percentage := h.globalTracker.GetUsagePercentage()

	// Simple prediction based on current time of day
	now := time.Now()
	minutesSinceMidnight := now.Hour()*60 + now.Minute()
	if minutesSinceMidnight == 0 {
		minutesSinceMidnight = 1 // Avoid division by zero
	}

	// Estimate daily usage based on current rate
	estimatedDailyUsage := int(float64(used) / float64(minutesSinceMidnight) * 1440) // 1440 minutes in a day

	willDeplete := estimatedDailyUsage > limit
	var depletionTime *string
	var recommendation string

	if willDeplete {
		// Estimate when quota will be depleted
		remaining := limit - used
		minutesUntilDepletion := float64(remaining) / (float64(used) / float64(minutesSinceMidnight))
		depletionEstimate := now.Add(time.Duration(minutesUntilDepletion) * time.Minute).Format(time.RFC3339)
		depletionTime = &depletionEstimate
		recommendation = "Current usage rate will exceed daily quota. Consider reducing polling frequency or pausing low-priority channels."
	} else {
		recommendation = "Current usage rate is sustainable for the rest of the day."
	}

	response := QuotaPredictionResponse{
		CurrentUsage:        used,
		CurrentPercentage:   percentage,
		EstimatedDailyUsage: estimatedDailyUsage,
		EstimatedDepletion:  depletionTime,
		WillDeplete:         willDeplete,
		Recommendation:      recommendation,
	}

	c.JSON(http.StatusOK, response)

	h.logger.Debug("Quota prediction retrieved",
		zap.String("request_from", c.ClientIP()),
	)
}

// RecordQuotaRequest represents a request to record quota usage from external services
type RecordQuotaRequest struct {
	Units int `json:"units" binding:"required,min=1"`
}

// RecordQuota records quota usage from external services (e.g., overlay-manager)
// POST /quota/record
func (h *QuotaHandler) RecordQuota(c *gin.Context) {
	var req RecordQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: units required and must be >= 1",
		})
		return
	}

	ctx := c.Request.Context()

	// Record in global tracker
	if err := h.globalTracker.RecordUsage(ctx, req.Units); err != nil {
		h.logger.Error("Failed to record quota usage from external service",
			zap.Int("units", req.Units),
			zap.String("client_ip", c.ClientIP()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to record quota usage",
		})
		return
	}

	h.logger.Info("Recorded quota usage from external service",
		zap.Int("units", req.Units),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"units_recorded": req.Units,
	})
}

// parseIntParam safely parses an integer parameter
func parseIntParam(param string) (int, error) {
	var result int
	_, err := fmt.Sscanf(param, "%d", &result)
	return result, err
}

// =============== CROSS-SERVICE QUOTA COORDINATION API ===============

// CheckQuotaRequest represents a request to check if quota is available
type CheckQuotaRequest struct {
	Units         int    `json:"units" binding:"required,min=1"`
	Service       string `json:"service" binding:"required"`        // "auth-service", "overlay-manager"
	Operation     string `json:"operation" binding:"required"`      // "search.list", "liveBroadcasts.list", etc.
	AllowCritical bool   `json:"allow_critical"`                    // Bypass emergency shutoff for OAuth
}

// CheckQuotaResponse represents the quota check response
type CheckQuotaResponse struct {
	Allowed   bool   `json:"allowed"`
	Remaining int    `json:"remaining"`
	Reason    string `json:"reason,omitempty"`
}

// CheckQuota checks if quota is available for an operation WITHOUT reserving it
// POST /api/v1/quota/check
func (h *QuotaHandler) CheckQuota(c *gin.Context) {
	var req CheckQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
			"details": err.Error(),
		})
		return
	}

	// Check if emergency threshold would be breached
	percentage := h.globalTracker.GetUsagePercentage()
	emergencyThreshold := 90.0 // TODO: Get from tracker

	if percentage >= emergencyThreshold && !req.AllowCritical {
		c.JSON(http.StatusOK, CheckQuotaResponse{
			Allowed:   false,
			Remaining: h.globalTracker.GetRemainingQuota(),
			Reason:    fmt.Sprintf("emergency shutoff at %.1f%%", percentage),
		})
		return
	}

	// Check if sufficient quota remaining
	remaining := h.globalTracker.GetRemainingQuota()
	allowed := remaining >= req.Units

	response := CheckQuotaResponse{
		Allowed:   allowed,
		Remaining: remaining,
	}

	if !allowed {
		response.Reason = "insufficient quota"
	}

	c.JSON(http.StatusOK, response)

	h.logger.Debug("Quota check from external service",
		zap.String("service", req.Service),
		zap.String("operation", req.Operation),
		zap.Int("units", req.Units),
		zap.Bool("allowed", allowed),
	)
}

// ReserveQuotaRequest represents a request to reserve quota
type ReserveQuotaRequest struct {
	Units         int    `json:"units" binding:"required,min=1"`
	Service       string `json:"service" binding:"required"`
	Operation     string `json:"operation" binding:"required"`
	AllowCritical bool   `json:"allow_critical"` // Bypass emergency shutoff
}

// ReserveQuotaResponse represents the reservation response
type ReserveQuotaResponse struct {
	Success       bool   `json:"success"`
	ReservationID string `json:"reservation_id,omitempty"`
	Remaining     int    `json:"remaining"`
	Reason        string `json:"reason,omitempty"`
}

// ReserveQuota reserves quota for an external service before making a YouTube API call
// POST /api/v1/quota/reserve
func (h *QuotaHandler) ReserveQuota(c *gin.Context) {
	var req ReserveQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
			"details": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	// Reserve quota with priority support
	reservationID, err := h.globalTracker.ReserveQuotaWithPriority(ctx, req.Units, req.AllowCritical)
	if err != nil {
		h.logger.Warn("External service quota reservation failed",
			zap.String("service", req.Service),
			zap.String("operation", req.Operation),
			zap.Int("units", req.Units),
			zap.Bool("allow_critical", req.AllowCritical),
			zap.Error(err),
		)

		c.JSON(http.StatusOK, ReserveQuotaResponse{
			Success:   false,
			Remaining: h.globalTracker.GetRemainingQuota(),
			Reason:    err.Error(),
		})
		return
	}

	h.logger.Info("Reserved quota for external service",
		zap.String("service", req.Service),
		zap.String("operation", req.Operation),
		zap.Int("units", req.Units),
		zap.String("reservation_id", reservationID),
	)

	c.JSON(http.StatusOK, ReserveQuotaResponse{
		Success:       true,
		ReservationID: reservationID,
		Remaining:     h.globalTracker.GetRemainingQuota(),
	})
}

// ConfirmQuotaRequest represents a request to confirm a quota reservation
type ConfirmQuotaRequest struct {
	ReservationID string `json:"reservation_id" binding:"required"`
	Units         int    `json:"units" binding:"required,min=1"`
	Service       string `json:"service" binding:"required"`
	Success       bool   `json:"success"` // True = confirm, False = rollback
}

// ConfirmQuota confirms or rolls back a quota reservation
// POST /api/v1/quota/confirm
func (h *QuotaHandler) ConfirmQuota(c *gin.Context) {
	var req ConfirmQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request",
			"details": err.Error(),
		})
		return
	}

	ctx := c.Request.Context()

	var err error
	if req.Success {
		err = h.globalTracker.ConfirmReservation(ctx, req.ReservationID, req.Units)
		if err != nil {
			h.logger.Error("Failed to confirm quota reservation",
				zap.String("service", req.Service),
				zap.String("reservation_id", req.ReservationID),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to confirm reservation",
			})
			return
		}

		h.logger.Info("Confirmed quota reservation from external service",
			zap.String("service", req.Service),
			zap.Int("units", req.Units),
			zap.String("reservation_id", req.ReservationID),
		)
	} else {
		err = h.globalTracker.RollbackReservation(ctx, req.ReservationID, req.Units)
		if err != nil {
			h.logger.Error("Failed to rollback quota reservation",
				zap.String("service", req.Service),
				zap.String("reservation_id", req.ReservationID),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to rollback reservation",
			})
			return
		}

		h.logger.Info("Rolled back quota reservation from external service",
			zap.String("service", req.Service),
			zap.Int("units", req.Units),
			zap.String("reservation_id", req.ReservationID),
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
