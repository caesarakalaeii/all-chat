package quota

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	// DefaultDailyQuota is the default YouTube API quota (10,000 units/day)
	DefaultDailyQuota = 10000

	// QuotaCostLiveChatMessages is the quota cost for liveChatMessages.list (5 units)
	QuotaCostLiveChatMessages = 5

	// QuotaCostVideos is the quota cost for videos.list (1 unit)
	QuotaCostVideos = 1

	// QuotaCostSearch is the quota cost for search.list (100 units)
	QuotaCostSearch = 100

	// Default state thresholds (as percentages)
	DefaultHealthyThreshold   = 70.0 // 0-70%: HEALTHY
	DefaultDegradedThreshold  = 85.0 // 70-85%: DEGRADED
	DefaultCriticalThreshold  = 95.0 // 85-95%: CRITICAL
	DefaultExhaustedThreshold = 100.0 // 95-100%: EXHAUSTED
	// 100%+: DEPLETED
)

// QuotaState represents the current state of quota usage
type QuotaState string

const (
	// QuotaStateHealthy - Normal operation (0-70%)
	QuotaStateHealthy QuotaState = "HEALTHY"

	// QuotaStateDegraded - Reduce low-priority operations (70-85%)
	QuotaStateDegraded QuotaState = "DEGRADED"

	// QuotaStateCritical - Stop discovery, active polling only (85-95%)
	QuotaStateCritical QuotaState = "CRITICAL"

	// QuotaStateExhausted - Slow down polling intervals (95-100%)
	QuotaStateExhausted QuotaState = "EXHAUSTED"

	// QuotaStateDepleted - Hard block all requests (100%+)
	QuotaStateDepleted QuotaState = "DEPLETED"
)

// Notifier interface for quota notifications (to avoid circular dependency)
type Notifier interface {
	NotifyStateTransition(ctx context.Context, oldState, newState QuotaState, percentage float64, used, limit int) error
	NotifyThresholdCrossed(ctx context.Context, state QuotaState, threshold float64, percentage float64, used, limit int) error
}

// Tracker tracks YouTube API quota usage
type Tracker struct {
	db         *pgxpool.Pool
	logger     *zap.Logger
	metrics    *metrics.ListenerMetrics
	dailyLimit int
	notifier   Notifier

	mu          sync.RWMutex
	usageToday  int
	currentDate string
	currentState QuotaState
	lastStateTransition time.Time

	// State thresholds (configurable)
	healthyThreshold   float64
	degradedThreshold  float64
	criticalThreshold  float64
	exhaustedThreshold float64

	stopChan chan struct{}
}

// NewTracker creates a new quota tracker
func NewTracker(db *pgxpool.Pool, dailyLimit int, logger *zap.Logger, m *metrics.ListenerMetrics) *Tracker {
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailyQuota
	}

	// Load configurable thresholds from environment variables
	healthyThreshold := getEnvAsFloat("QUOTA_HEALTHY_THRESHOLD", DefaultHealthyThreshold)
	degradedThreshold := getEnvAsFloat("QUOTA_DEGRADED_THRESHOLD", DefaultDegradedThreshold)
	criticalThreshold := getEnvAsFloat("QUOTA_CRITICAL_THRESHOLD", DefaultCriticalThreshold)
	exhaustedThreshold := getEnvAsFloat("QUOTA_EXHAUSTED_THRESHOLD", DefaultExhaustedThreshold)

	logger.Info("Quota tracker thresholds configured",
		zap.Float64("healthy", healthyThreshold),
		zap.Float64("degraded", degradedThreshold),
		zap.Float64("critical", criticalThreshold),
		zap.Float64("exhausted", exhaustedThreshold),
	)

	return &Tracker{
		db:         db,
		logger:     logger,
		metrics:    m,
		dailyLimit: dailyLimit,
		currentState: QuotaStateHealthy,
		lastStateTransition: time.Now(),
		healthyThreshold:   healthyThreshold,
		degradedThreshold:  degradedThreshold,
		criticalThreshold:  criticalThreshold,
		exhaustedThreshold: exhaustedThreshold,
		stopChan:   make(chan struct{}),
	}
}

// getEnvAsFloat gets an environment variable as float64 or returns default
func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// Start initializes the tracker and loads today's usage
func (t *Tracker) Start(ctx context.Context) error {
	t.logger.Info("Starting quota tracker",
		zap.Int("daily_limit", t.dailyLimit),
	)

	// Load today's usage
	if err := t.loadTodayUsage(ctx); err != nil {
		return fmt.Errorf("failed to load today's usage: %w", err)
	}

	// Initialize metrics with current values to ensure Prometheus has accurate data
	// This is critical for pod restarts to avoid showing stale gauge values
	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	remaining := t.dailyLimit - t.usageToday
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)

	// Calculate initial state based on current usage
	t.mu.Lock()
	t.currentState = t.calculateState(percentage)
	t.lastStateTransition = time.Now()
	t.mu.Unlock()

	t.logger.Info("Initialized quota metrics and state",
		zap.Int("usage_today", t.usageToday),
		zap.Int("remaining", remaining),
		zap.Float64("percentage", percentage),
		zap.String("initial_state", string(t.currentState)),
	)

	// Start background goroutine to check for date rollover periodically
	go t.periodicDateCheck()

	return nil
}

// Stop gracefully stops the quota tracker background goroutine
func (t *Tracker) Stop() {
	close(t.stopChan)
	t.logger.Info("Quota tracker stopped")
}

// SetNotifier sets the notifier for quota events (optional)
func (t *Tracker) SetNotifier(notifier Notifier) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifier = notifier
	t.logger.Info("Quota notifier attached to tracker")
}

// periodicDateCheck runs in a background goroutine and checks for date rollover every minute
func (t *Tracker) periodicDateCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	t.logger.Info("Started periodic date rollover check (every 1 minute)")

	for {
		select {
		case <-ticker.C:
			t.checkDateRollover()
		case <-t.stopChan:
			t.logger.Info("Periodic date rollover check stopped")
			return
		}
	}
}

// checkDateRollover checks if the date has changed and resets quota if needed
// This method must be called WITHOUT holding the lock, as it will acquire a write lock if needed
func (t *Tracker) checkDateRollover() {
	today := time.Now().Format("2006-01-02")

	// Fast path: check with read lock first
	t.mu.RLock()
	if t.currentDate == today {
		t.mu.RUnlock()
		return
	}
	t.mu.RUnlock()

	// Slow path: date changed, acquire write lock and reset
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have reset already)
	if t.currentDate == today {
		return
	}

	t.logger.Info("Date rolled over, resetting quota",
		zap.String("old_date", t.currentDate),
		zap.String("new_date", today),
	)
	t.currentDate = today
	t.usageToday = 0

	// Reset state to HEALTHY
	if t.currentState != QuotaStateHealthy {
		oldState := t.currentState
		t.currentState = QuotaStateHealthy
		t.lastStateTransition = time.Now()
		t.logger.Info("Quota state reset to HEALTHY after date rollover",
			zap.String("old_state", string(oldState)),
		)
	}

	// Immediately update metrics to reflect the reset (prevents stale gauge values)
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(t.dailyLimit))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", 0.0)
}

// calculateState determines the current quota state based on usage percentage
func (t *Tracker) calculateState(percentage float64) QuotaState {
	if percentage >= 100.0 {
		return QuotaStateDepleted
	} else if percentage >= t.exhaustedThreshold {
		return QuotaStateExhausted
	} else if percentage >= t.criticalThreshold {
		return QuotaStateCritical
	} else if percentage >= t.degradedThreshold {
		return QuotaStateDegraded
	}
	return QuotaStateHealthy
}

// updateStateAndNotify updates the quota state and emits transition events if state changed
// Must be called WITH the lock held
func (t *Tracker) updateStateAndNotify(percentage float64) {
	newState := t.calculateState(percentage)

	// Check if state changed
	if newState != t.currentState {
		oldState := t.currentState
		t.currentState = newState
		t.lastStateTransition = time.Now()

		t.logger.Warn("Quota state transition",
			zap.String("old_state", string(oldState)),
			zap.String("new_state", string(newState)),
			zap.Float64("percentage", percentage),
			zap.Int("used", t.usageToday),
			zap.Int("limit", t.dailyLimit),
		)

		// Record state transition metric
		t.metrics.RateLimitHits.WithLabelValues("youtube", "youtube-listener", fmt.Sprintf("quota_state_%s", string(newState))).Inc()

		// Emit notification if notifier is configured
		if t.notifier != nil {
			ctx := context.Background()
			if err := t.notifier.NotifyStateTransition(ctx, oldState, newState, percentage, t.usageToday, t.dailyLimit); err != nil {
				t.logger.Warn("Failed to send quota state transition notification",
					zap.Error(err),
				)
			}
		}

		// Log actionable warnings based on new state
		switch newState {
		case QuotaStateDegraded:
			t.logger.Warn("Quota degraded - reducing low-priority discovery operations",
				zap.Float64("percentage", percentage),
			)
		case QuotaStateCritical:
			t.logger.Error("Quota critical - stopping all discovery, active polling only",
				zap.Float64("percentage", percentage),
			)
		case QuotaStateExhausted:
			t.logger.Error("Quota exhausted - slowing down polling intervals",
				zap.Float64("percentage", percentage),
			)
		case QuotaStateDepleted:
			t.logger.Error("Quota depleted - blocking all API requests",
				zap.Float64("percentage", percentage),
			)
		case QuotaStateHealthy:
			t.logger.Info("Quota recovered to healthy state",
				zap.Float64("percentage", percentage),
			)
		}
	}
}

// RecordUsage records API quota usage
func (t *Tracker) RecordUsage(ctx context.Context, units int) error {
	// Check for date rollover before acquiring lock
	t.checkDateRollover()

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update in-memory counter
	t.usageToday += units

	// Update database (use currentDate which is guaranteed to be today after checkDateRollover)
	query := `
		INSERT INTO youtube_quota_usage (date, units_used, units_limit)
		VALUES ($1, $2, $3)
		ON CONFLICT (date)
		DO UPDATE SET
			units_used = youtube_quota_usage.units_used + EXCLUDED.units_used,
			updated_at = NOW()
	`

	_, err := t.db.Exec(ctx, query, t.currentDate, units, t.dailyLimit)
	if err != nil {
		t.logger.Error("Failed to record quota usage",
			zap.Int("units", units),
			zap.Error(err),
		)
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Calculate percentage and update state
	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	remaining := t.dailyLimit - t.usageToday

	// Update state machine and emit transition events
	t.updateStateAndNotify(percentage)

	// Record quota metrics
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)

	t.logger.Debug("Recorded quota usage",
		zap.Int("units", units),
		zap.Int("total_used", t.usageToday),
		zap.Int("limit", t.dailyLimit),
		zap.Float64("percentage", percentage),
	)

	return nil
}

// GetUsageToday returns today's quota usage
func (t *Tracker) GetUsageToday() int {
	t.checkDateRollover()
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.usageToday
}

// GetRemainingQuota returns remaining quota for today
func (t *Tracker) GetRemainingQuota() int {
	t.checkDateRollover()
	t.mu.RLock()
	defer t.mu.RUnlock()
	remaining := t.dailyLimit - t.usageToday
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CanMakeRequest checks if we have enough quota for a request
func (t *Tracker) CanMakeRequest(units int) bool {
	return t.GetRemainingQuota() >= units
}

// GetUsagePercentage returns usage as a percentage
func (t *Tracker) GetUsagePercentage() float64 {
	t.checkDateRollover()
	t.mu.RLock()
	defer t.mu.RUnlock()
	return float64(t.usageToday) / float64(t.dailyLimit) * 100
}

// GetState returns the current quota state
func (t *Tracker) GetState() QuotaState {
	t.checkDateRollover()
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentState
}

// GetStateInfo returns detailed quota state information
func (t *Tracker) GetStateInfo() (QuotaState, float64, time.Time) {
	t.checkDateRollover()
	t.mu.RLock()
	defer t.mu.RUnlock()
	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	return t.currentState, percentage, t.lastStateTransition
}

// ShouldAllowDiscovery checks if discovery operations should be allowed based on state
func (t *Tracker) ShouldAllowDiscovery() bool {
	state := t.GetState()
	return state == QuotaStateHealthy || state == QuotaStateDegraded
}

// ShouldAllowLowPriorityDiscovery checks if low-priority discovery should be allowed
func (t *Tracker) ShouldAllowLowPriorityDiscovery() bool {
	return t.GetState() == QuotaStateHealthy
}

// ShouldSlowDownPolling checks if polling should be slowed down
func (t *Tracker) ShouldSlowDownPolling() bool {
	state := t.GetState()
	return state == QuotaStateExhausted || state == QuotaStateDepleted
}

// ShouldBlockAllRequests checks if all API requests should be blocked
func (t *Tracker) ShouldBlockAllRequests() bool {
	return t.GetState() == QuotaStateDepleted
}

// loadTodayUsage loads today's usage from database
func (t *Tracker) loadTodayUsage(ctx context.Context) error {
	today := time.Now().Format("2006-01-02")

	query := `
		SELECT units_used
		FROM youtube_quota_usage
		WHERE date = $1
	`

	var usageToday int
	err := t.db.QueryRow(ctx, query, today).Scan(&usageToday)
	if err != nil {
		// If no row found, that's fine (start from 0)
		if err.Error() == "no rows in result set" {
			usageToday = 0
			t.logger.Info("No usage record for today, starting fresh",
				zap.String("date", today),
			)
		} else {
			return fmt.Errorf("failed to load today's usage: %w", err)
		}
	}

	t.mu.Lock()
	t.currentDate = today
	t.usageToday = usageToday
	t.mu.Unlock()

	t.logger.Info("Loaded today's quota usage",
		zap.String("date", today),
		zap.Int("used", usageToday),
		zap.Int("limit", t.dailyLimit),
		zap.Float64("percentage", float64(usageToday)/float64(t.dailyLimit)*100),
	)

	return nil
}
