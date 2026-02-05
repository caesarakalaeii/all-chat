package streams

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Channel priority tiers based on recent activity
type ChannelPriority int

const (
	PriorityHigh     ChannelPriority = iota // Recently live < 24h
	PriorityStandard                         // 24h to 7 days
	PriorityLow                              // > 7 days
)

// QuotaBudget manages per-channel quota caps and adaptive throttling
// to prevent quota exhaustion from aggressive detection
type QuotaBudget struct {
	quotaTracker  *quota.Tracker
	redisClient   *redis.Client
	logger        *zap.Logger
	ytMetrics     *metrics.YouTubeMetrics
	mu            sync.RWMutex
	stopChan      chan struct{}

	// Per-channel daily caps (full detections)
	highPriorityCap     int // Default: 100 full detections/day (10,000 units)
	standardPriorityCap int // Default: 50 full detections/day (5,000 units)
	lowPriorityCap      int // Default: 20 full detections/day (2,000 units)

	// Global quota reserves
	manualOpsReservePercent   float64 // Default: 20% for manual operations
	peakHoursReservePercent   float64 // Default: 10% for evening peak
	manualOpsUsedToday        int     // Track manual ops usage
	manualOpsQuotaLogWarnings int     // Count of warnings logged today

	// Adaptive throttling thresholds
	normalOperationThreshold  float64 // Default: 70% - normal operation
	preferStatusCheckThreshold float64 // Default: 50% - prefer status checks
	criticalThreshold         float64 // Default: 30% - emergency mode

	// Channel activity tracking (in-memory cache from Redis)
	channelLastLive       map[string]time.Time // channelID -> last known live time
	channelDetectionsToday map[string]int      // channelID -> full detection count today
	currentDate           string

	// Metrics tracking
	detectionSkippedByReason map[string]int // reason -> count
}

// NewQuotaBudget creates a new quota budgeting system
func NewQuotaBudget(
	quotaTracker *quota.Tracker,
	redisClient *redis.Client,
	ytMetrics *metrics.YouTubeMetrics,
	logger *zap.Logger,
) *QuotaBudget {
	// Load configuration from environment variables with defaults
	highPriorityCap := getEnvAsInt("QUOTA_BUDGET_HIGH_PRIORITY_CAP", 100)
	standardPriorityCap := getEnvAsInt("QUOTA_BUDGET_STANDARD_PRIORITY_CAP", 50)
	lowPriorityCap := getEnvAsInt("QUOTA_BUDGET_LOW_PRIORITY_CAP", 20)

	manualOpsReservePercent := getEnvAsFloat64("QUOTA_BUDGET_MANUAL_RESERVE_PERCENT", 20.0)
	peakHoursReservePercent := getEnvAsFloat64("QUOTA_BUDGET_PEAK_RESERVE_PERCENT", 10.0)

	normalOperationThreshold := getEnvAsFloat64("QUOTA_BUDGET_NORMAL_THRESHOLD", 70.0)
	preferStatusCheckThreshold := getEnvAsFloat64("QUOTA_BUDGET_STATUS_CHECK_THRESHOLD", 50.0)
	criticalThreshold := getEnvAsFloat64("QUOTA_BUDGET_CRITICAL_THRESHOLD", 30.0)

	logger.Info("Quota budget system configured",
		zap.Int("high_priority_cap", highPriorityCap),
		zap.Int("standard_priority_cap", standardPriorityCap),
		zap.Int("low_priority_cap", lowPriorityCap),
		zap.Float64("manual_ops_reserve_percent", manualOpsReservePercent),
		zap.Float64("peak_hours_reserve_percent", peakHoursReservePercent),
		zap.Float64("normal_operation_threshold", normalOperationThreshold),
		zap.Float64("prefer_status_check_threshold", preferStatusCheckThreshold),
		zap.Float64("critical_threshold", criticalThreshold),
	)

	return &QuotaBudget{
		quotaTracker:              quotaTracker,
		redisClient:               redisClient,
		ytMetrics:                 ytMetrics,
		logger:                    logger,
		stopChan:                  make(chan struct{}),
		highPriorityCap:           highPriorityCap,
		standardPriorityCap:       standardPriorityCap,
		lowPriorityCap:            lowPriorityCap,
		manualOpsReservePercent:   manualOpsReservePercent,
		peakHoursReservePercent:   peakHoursReservePercent,
		normalOperationThreshold:  normalOperationThreshold,
		preferStatusCheckThreshold: preferStatusCheckThreshold,
		criticalThreshold:         criticalThreshold,
		channelLastLive:           make(map[string]time.Time),
		channelDetectionsToday:    make(map[string]int),
		detectionSkippedByReason:  make(map[string]int),
		manualOpsUsedToday:        0,
		manualOpsQuotaLogWarnings: 0,
	}
}

// Start initializes the quota budget system
func (qb *QuotaBudget) Start(ctx context.Context) error {
	qb.logger.Info("Starting quota budget system")

	// Load today's per-channel detection counts from Redis
	if err := qb.loadTodayDetections(ctx); err != nil {
		qb.logger.Warn("Failed to load today's detection counts, starting fresh",
			zap.Error(err),
		)
	}

	// Start background goroutines
	go qb.periodicDateCheck()
	go qb.periodicChannelActivityRefresh(ctx)

	return nil
}

// Stop gracefully stops the quota budget system
func (qb *QuotaBudget) Stop() {
	close(qb.stopChan)
	qb.logger.Info("Quota budget system stopped")
}

// getEnvAsInt gets an environment variable as int or returns default
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsFloat64 gets an environment variable as float64 or returns default
func getEnvAsFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// GetChannelPriority determines a channel's priority tier based on last live time
func (qb *QuotaBudget) GetChannelPriority(channelID string) ChannelPriority {
	qb.mu.RLock()
	lastLive, exists := qb.channelLastLive[channelID]
	qb.mu.RUnlock()

	if !exists {
		// Unknown activity - treat as low priority
		return PriorityLow
	}

	timeSinceLive := time.Since(lastLive)

	if timeSinceLive < 24*time.Hour {
		return PriorityHigh
	} else if timeSinceLive < 7*24*time.Hour {
		return PriorityStandard
	}
	return PriorityLow
}

// GetChannelQuotaCap returns the daily quota cap for a channel based on priority
func (qb *QuotaBudget) GetChannelQuotaCap(channelID string) int {
	priority := qb.GetChannelPriority(channelID)
	switch priority {
	case PriorityHigh:
		return qb.highPriorityCap
	case PriorityStandard:
		return qb.standardPriorityCap
	case PriorityLow:
		return qb.lowPriorityCap
	default:
		return qb.lowPriorityCap
	}
}

// CanChannelUseFullDetection checks if a channel can use full detection (100 units)
// Returns (allowed, reason) where reason explains why if not allowed
func (qb *QuotaBudget) CanChannelUseFullDetection(channelID string) (bool, string) {
	qb.mu.RLock()
	detectionsToday := qb.channelDetectionsToday[channelID]
	qb.mu.RUnlock()

	cap := qb.GetChannelQuotaCap(channelID)

	if detectionsToday >= cap {
		priority := qb.GetChannelPriority(channelID)
		reason := fmt.Sprintf("channel_quota_cap_reached (priority=%v, cap=%d, used=%d)",
			priority, cap, detectionsToday)

		// Emit throttled metric
		if qb.ytMetrics != nil {
			qb.ytMetrics.QuotaBudgetThrottled.WithLabelValues(channelID, "channel_quota_cap_reached").Inc()
		}

		return false, reason
	}

	return true, ""
}

// RecordFullDetection records that a channel used a full detection (100 units)
func (qb *QuotaBudget) RecordFullDetection(ctx context.Context, channelID string) error {
	today := time.Now().In(quota.YouTubePST).Format("2006-01-02")

	qb.mu.Lock()
	qb.channelDetectionsToday[channelID]++
	count := qb.channelDetectionsToday[channelID]
	qb.mu.Unlock()

	// Persist to Redis
	key := fmt.Sprintf("youtube:quota_budget:detections:%s:%s", today, channelID)
	err := qb.redisClient.Incr(ctx, key).Err()
	if err != nil {
		qb.logger.Warn("Failed to persist detection count to Redis",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		// Don't fail the operation, in-memory count is primary
	} else {
		// Set expiry to 48 hours to auto-cleanup
		qb.redisClient.Expire(ctx, key, 48*time.Hour)
	}

	cap := qb.GetChannelQuotaCap(channelID)
	priority := qb.GetChannelPriority(channelID)

	qb.logger.Debug("Recorded full detection for channel",
		zap.String("channel_id", channelID),
		zap.Int("count_today", count),
		zap.Int("cap", cap),
		zap.String("priority", fmt.Sprintf("%v", priority)),
	)

	return nil
}

// RecordManualOperation records quota usage from a manual operation (bypasses budget)
func (qb *QuotaBudget) RecordManualOperation(ctx context.Context, units int, operation string) {
	qb.mu.Lock()
	qb.manualOpsUsedToday += units
	totalManual := qb.manualOpsUsedToday
	qb.mu.Unlock()

	dailyLimit := qb.quotaTracker.GetUsageToday() + qb.quotaTracker.GetRemainingQuota()
	manualReserve := int(float64(dailyLimit) * qb.manualOpsReservePercent / 100.0)

	qb.logger.Info("Manual operation used quota (bypassed budget)",
		zap.String("operation", operation),
		zap.Int("units", units),
		zap.Int("total_manual_today", totalManual),
		zap.Int("manual_reserve", manualReserve),
		zap.Float64("reserve_percent_used", float64(totalManual)/float64(manualReserve)*100.0),
	)

	// Warn if manual operations are consuming too much of the reserve
	if totalManual > manualReserve && qb.manualOpsQuotaLogWarnings < 5 {
		qb.mu.Lock()
		qb.manualOpsQuotaLogWarnings++
		warningCount := qb.manualOpsQuotaLogWarnings
		qb.mu.Unlock()

		qb.logger.Warn("Manual operations exceeded reserve budget",
			zap.Int("total_manual_today", totalManual),
			zap.Int("manual_reserve", manualReserve),
			zap.Int("overage", totalManual-manualReserve),
			zap.Int("warning_count", warningCount),
		)
	}
}

// ShouldPreferStatusCheck determines if status checks (1 unit) should be preferred over full detection
func (qb *QuotaBudget) ShouldPreferStatusCheck() bool {
	remainingPercent := qb.GetRemainingQuotaPercent()
	return remainingPercent < qb.preferStatusCheckThreshold
}

// ShouldThrottleLowPriority determines if low-priority channels should be throttled
func (qb *QuotaBudget) ShouldThrottleLowPriority() bool {
	remainingPercent := qb.GetRemainingQuotaPercent()
	return remainingPercent < qb.preferStatusCheckThreshold
}

// IsEmergencyMode checks if we're in emergency quota mode (critical channels only)
func (qb *QuotaBudget) IsEmergencyMode() bool {
	remainingPercent := qb.GetRemainingQuotaPercent()
	return remainingPercent < qb.criticalThreshold
}

// GetRemainingQuotaPercent returns the percentage of quota remaining
func (qb *QuotaBudget) GetRemainingQuotaPercent() float64 {
	remaining := qb.quotaTracker.GetRemainingQuota()
	used := qb.quotaTracker.GetUsageToday()
	total := used + remaining
	if total == 0 {
		return 100.0
	}
	return float64(remaining) / float64(total) * 100.0
}

// UpdateChannelLastLive updates when a channel was last seen live
func (qb *QuotaBudget) UpdateChannelLastLive(ctx context.Context, channelID string, liveTime time.Time) {
	qb.mu.Lock()
	qb.channelLastLive[channelID] = liveTime
	qb.mu.Unlock()

	// Persist to Redis for cross-pod sharing
	key := fmt.Sprintf("youtube:quota_budget:last_live:%s", channelID)
	err := qb.redisClient.Set(ctx, key, liveTime.Unix(), 30*24*time.Hour).Err()
	if err != nil {
		qb.logger.Warn("Failed to persist last live time to Redis",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	qb.logger.Debug("Updated channel last live time",
		zap.String("channel_id", channelID),
		zap.Time("live_time", liveTime),
		zap.String("priority", fmt.Sprintf("%v", qb.GetChannelPriority(channelID))),
	)
}

// GetBudgetSummary returns a summary of quota budget usage
func (qb *QuotaBudget) GetBudgetSummary() map[string]interface{} {
	qb.mu.RLock()
	defer qb.mu.RUnlock()

	highPriorityChannels := 0
	standardPriorityChannels := 0
	lowPriorityChannels := 0

	for channelID := range qb.channelLastLive {
		switch qb.GetChannelPriority(channelID) {
		case PriorityHigh:
			highPriorityChannels++
		case PriorityStandard:
			standardPriorityChannels++
		case PriorityLow:
			lowPriorityChannels++
		}
	}

	quotaRemaining := qb.quotaTracker.GetRemainingQuota()
	quotaUsed := qb.quotaTracker.GetUsageToday()
	quotaTotal := quotaUsed + quotaRemaining

	return map[string]interface{}{
		"quota_remaining_percent": qb.GetRemainingQuotaPercent(),
		"quota_used":              quotaUsed,
		"quota_remaining":         quotaRemaining,
		"quota_total":             quotaTotal,
		"manual_ops_used_today":   qb.manualOpsUsedToday,
		"manual_ops_reserve":      int(float64(quotaTotal) * qb.manualOpsReservePercent / 100.0),
		"emergency_mode":          qb.IsEmergencyMode(),
		"prefer_status_checks":    qb.ShouldPreferStatusCheck(),
		"throttle_low_priority":   qb.ShouldThrottleLowPriority(),
		"channels": map[string]interface{}{
			"high_priority":     highPriorityChannels,
			"standard_priority": standardPriorityChannels,
			"low_priority":      lowPriorityChannels,
		},
		"detections_skipped_by_reason": qb.detectionSkippedByReason,
	}
}

// RecordDetectionSkipped records that a detection was skipped for a specific reason
func (qb *QuotaBudget) RecordDetectionSkipped(reason string) {
	qb.mu.Lock()
	qb.detectionSkippedByReason[reason]++
	qb.mu.Unlock()

	// Emit Prometheus metric
	if qb.ytMetrics != nil {
		qb.ytMetrics.DetectionSkippedTotal.WithLabelValues(reason).Inc()
	}
}

// loadTodayDetections loads today's per-channel detection counts from Redis
func (qb *QuotaBudget) loadTodayDetections(ctx context.Context) error {
	today := time.Now().In(quota.YouTubePST).Format("2006-01-02")
	qb.currentDate = today

	pattern := fmt.Sprintf("youtube:quota_budget:detections:%s:*", today)
	iter := qb.redisClient.Scan(ctx, 0, pattern, 100).Iterator()

	loadedCount := 0
	for iter.Next(ctx) {
		key := iter.Val()

		// Extract channel ID from key
		// Key format: youtube:quota_budget:detections:2025-01-01:UC...
		parts := iter.Val()[len(fmt.Sprintf("youtube:quota_budget:detections:%s:", today)):]
		channelID := parts

		count, err := qb.redisClient.Get(ctx, key).Int()
		if err != nil {
			qb.logger.Warn("Failed to load detection count for channel",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			continue
		}

		qb.mu.Lock()
		qb.channelDetectionsToday[channelID] = count
		qb.mu.Unlock()

		loadedCount++
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("error iterating detection keys: %w", err)
	}

	qb.logger.Info("Loaded today's detection counts from Redis",
		zap.Int("channels", loadedCount),
		zap.String("date", today),
	)

	return nil
}

// periodicDateCheck checks for date rollover and resets daily counters
func (qb *QuotaBudget) periodicDateCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	qb.logger.Info("Started periodic date check for quota budget")

	for {
		select {
		case <-ticker.C:
			qb.checkDateRollover()
		case <-qb.stopChan:
			qb.logger.Info("Stopped periodic date check for quota budget")
			return
		}
	}
}

// checkDateRollover checks if the date has changed and resets counters
func (qb *QuotaBudget) checkDateRollover() {
	today := time.Now().In(quota.YouTubePST).Format("2006-01-02")

	qb.mu.Lock()
	if qb.currentDate != today {
		oldDate := qb.currentDate
		qb.currentDate = today

		// Reset daily counters
		qb.channelDetectionsToday = make(map[string]int)
		qb.detectionSkippedByReason = make(map[string]int)
		qb.manualOpsUsedToday = 0
		qb.manualOpsQuotaLogWarnings = 0

		qb.logger.Info("Quota budget date rollover complete",
			zap.String("old_date", oldDate),
			zap.String("new_date", today),
		)
	}
	qb.mu.Unlock()
}

// periodicChannelActivityRefresh periodically refreshes channel last live times from Redis
func (qb *QuotaBudget) periodicChannelActivityRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	qb.logger.Info("Started periodic channel activity refresh")

	for {
		select {
		case <-ticker.C:
			qb.refreshChannelActivity(ctx)
		case <-qb.stopChan:
			qb.logger.Info("Stopped periodic channel activity refresh")
			return
		}
	}
}

// refreshChannelActivity loads channel last live times from Redis
func (qb *QuotaBudget) refreshChannelActivity(ctx context.Context) {
	pattern := "youtube:quota_budget:last_live:*"
	iter := qb.redisClient.Scan(ctx, 0, pattern, 100).Iterator()

	refreshedCount := 0
	for iter.Next(ctx) {
		key := iter.Val()

		// Extract channel ID from key
		channelID := key[len("youtube:quota_budget:last_live:"):]

		timestamp, err := qb.redisClient.Get(ctx, key).Int64()
		if err != nil {
			qb.logger.Warn("Failed to load last live time for channel",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			continue
		}

		liveTime := time.Unix(timestamp, 0)

		qb.mu.Lock()
		qb.channelLastLive[channelID] = liveTime
		qb.mu.Unlock()

		refreshedCount++
	}

	if err := iter.Err(); err != nil {
		qb.logger.Warn("Error refreshing channel activity from Redis",
			zap.Error(err),
		)
		return
	}

	qb.logger.Debug("Refreshed channel activity from Redis",
		zap.Int("channels", refreshedCount),
	)
}

// GetState returns the detection state for a channel (for handlers)
type ChannelDetectionBudgetState struct {
	DetectionsToday int
	QuotaCap        int
	Priority        ChannelPriority
}

// GetState returns detection budget state for a channel
func (qb *QuotaBudget) GetState(channelID string) *ChannelDetectionBudgetState {
	qb.mu.RLock()
	detectionsToday := qb.channelDetectionsToday[channelID]
	qb.mu.RUnlock()

	return &ChannelDetectionBudgetState{
		DetectionsToday: detectionsToday,
		QuotaCap:        qb.GetChannelQuotaCap(channelID),
		Priority:        qb.GetChannelPriority(channelID),
	}
}

// GetAllChannels returns all tracked channel IDs
func (qb *QuotaBudget) GetAllChannels() []string {
	qb.mu.RLock()
	defer qb.mu.RUnlock()

	// Collect unique channel IDs from both maps
	channelMap := make(map[string]struct{})
	
	for channelID := range qb.channelLastLive {
		channelMap[channelID] = struct{}{}
	}
	
	for channelID := range qb.channelDetectionsToday {
		channelMap[channelID] = struct{}{}
	}

	channels := make([]string, 0, len(channelMap))
	for channelID := range channelMap {
		channels = append(channels, channelID)
	}

	return channels
}
