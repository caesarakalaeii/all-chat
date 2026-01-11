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

// YouTubePST is the Pacific timezone where YouTube quota resets at midnight
var YouTubePST *time.Location

func init() {
	var err error
	YouTubePST, err = time.LoadLocation("America/Los_Angeles")
	if err != nil {
		// Fallback to UTC if PST is not available
		YouTubePST = time.UTC
	}
}

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
	lastNotifiedThreshold float64 // Last 5% threshold that triggered a notification

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
		lastNotifiedThreshold: 0.0,
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
	// Set lastNotifiedThreshold to the highest 5% threshold already crossed
	// This prevents re-notifying on restart for thresholds we've already passed
	t.lastNotifiedThreshold = (float64(int(percentage/5.0)) * 5.0)
	t.mu.Unlock()

	t.logger.Info("Initialized quota metrics and state",
		zap.Int("usage_today", t.usageToday),
		zap.Int("remaining", remaining),
		zap.Float64("percentage", percentage),
		zap.String("initial_state", string(t.currentState)),
		zap.Float64("last_notified_threshold", t.lastNotifiedThreshold),
	)

	// Start background goroutines
	go t.periodicDateCheck()
	go t.periodicReservationCleanup(ctx)
	go t.periodicDatabaseSync(ctx)

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

// periodicDatabaseSync runs in a background goroutine and syncs in-memory cache with database every 5 minutes
// This is CRITICAL to prevent drift when multiple pods or external services make API calls
func (t *Tracker) periodicDatabaseSync(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	t.logger.Info("Started periodic database sync (every 5 minutes)")

	for {
		select {
		case <-ticker.C:
			t.syncWithDatabase(ctx)
		case <-t.stopChan:
			t.logger.Info("Periodic database sync stopped")
			return
		}
	}
}

// syncWithDatabase reloads today's usage from database and updates in-memory cache
// This prevents drift when multiple pods or external services make API calls
func (t *Tracker) syncWithDatabase(ctx context.Context) {
	// Get current date in PST
	today := time.Now().In(YouTubePST).Format("2006-01-02")

	query := `
		SELECT units_used
		FROM youtube_quota_usage
		WHERE date = $1
	`

	var dbUsage int
	err := t.db.QueryRow(ctx, query, today).Scan(&dbUsage)
	if err != nil {
		// If no row found, usage is 0
		if err.Error() == "no rows in result set" {
			dbUsage = 0
		} else {
			t.logger.Error("Failed to sync with database",
				zap.Error(err),
				zap.String("date", today),
			)
			return
		}
	}

	// Compare with in-memory cache
	t.mu.Lock()
	memoryUsage := t.usageToday
	drift := dbUsage - memoryUsage

	// Only update if there's a drift > 5 units (to avoid log spam from tiny differences)
	if abs(drift) > 5 {
		t.logger.Warn("Quota drift detected, syncing with database",
			zap.Int("memory_usage", memoryUsage),
			zap.Int("database_usage", dbUsage),
			zap.Int("drift", drift),
			zap.String("date", today),
		)

		// Update in-memory cache to match database
		t.usageToday = dbUsage
		percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100

		// Update state based on new usage
		t.updateStateAndNotify(percentage)

		// Update metrics
		remaining := t.dailyLimit - t.usageToday
		t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
		t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)

		t.logger.Info("In-memory cache synced with database",
			zap.Int("new_usage", t.usageToday),
			zap.Float64("percentage", percentage),
			zap.String("state", string(t.currentState)),
		)
	}
	t.mu.Unlock()
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// checkDateRollover checks if the date has changed and resets quota if needed
// This method must be called WITHOUT holding the lock, as it will acquire a write lock if needed
// NOTE: Uses Pacific Time (PST/PDT) because YouTube quota resets at midnight PST
func (t *Tracker) checkDateRollover() {
	// Get current date in PST (YouTube's quota reset timezone)
	today := time.Now().In(YouTubePST).Format("2006-01-02")

	// Fast path: check with read lock first
	t.mu.RLock()
	if t.currentDate == today {
		t.mu.RUnlock()
		return
	}
	t.mu.RUnlock()

	// Slow path: date changed, reload from database
	t.logger.Info("Date rolled over (PST timezone), reloading quota from database",
		zap.String("old_date", t.currentDate),
		zap.String("new_date", today),
		zap.String("timezone", "America/Los_Angeles"),
	)

	// CRITICAL FIX: Load today's usage from database instead of just resetting to 0
	// This handles cases where:
	// 1. Another pod made API calls between midnight and now
	// 2. Database has pre-existing data for today
	// 3. This pod crashed/restarted and missed the rollover
	ctx := context.Background()
	if err := t.loadTodayUsage(ctx); err != nil {
		t.logger.Error("Failed to reload today's usage after date rollover",
			zap.Error(err),
			zap.String("date", today),
		)
		// Fallback: reset to 0 if database load fails
		t.mu.Lock()
		t.currentDate = today
		t.usageToday = 0
		t.lastNotifiedThreshold = 0.0
		t.mu.Unlock()
		return
	}

	// Update state based on loaded usage
	t.mu.Lock()
	defer t.mu.Unlock()

	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	t.lastNotifiedThreshold = (float64(int(percentage/5.0)) * 5.0)

	// Calculate new state based on current usage
	newState := t.calculateState(percentage)
	if t.currentState != newState {
		oldState := t.currentState
		t.currentState = newState
		t.lastStateTransition = time.Now()
		t.logger.Info("Quota state updated after date rollover",
			zap.String("old_state", string(oldState)),
			zap.String("new_state", string(newState)),
			zap.Int("usage_today", t.usageToday),
			zap.Float64("percentage", percentage),
		)
	}

	// Update metrics with loaded values
	remaining := t.dailyLimit - t.usageToday
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)

	t.logger.Info("Date rollover complete",
		zap.Int("usage_today", t.usageToday),
		zap.Int("remaining", remaining),
		zap.Float64("percentage", percentage),
		zap.String("state", string(t.currentState)),
	)
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

	// Check for 5% threshold crossings (75%, 80%, 85%, 90%, 95%, 100%, etc.)
	// Only notify for thresholds >= 75% to avoid spam
	currentThreshold := float64(int(percentage/5.0)) * 5.0
	if currentThreshold >= 75.0 && currentThreshold > t.lastNotifiedThreshold {
		t.lastNotifiedThreshold = currentThreshold

		t.logger.Info("Crossed 5% quota threshold",
			zap.Float64("threshold", currentThreshold),
			zap.Float64("percentage", percentage),
			zap.Int("used", t.usageToday),
		)

		// Emit threshold crossed notification
		if t.notifier != nil {
			ctx := context.Background()
			if err := t.notifier.NotifyThresholdCrossed(ctx, newState, currentThreshold, percentage, t.usageToday, t.dailyLimit); err != nil {
				t.logger.Warn("Failed to send threshold crossed notification",
					zap.Float64("threshold", currentThreshold),
					zap.Error(err),
				)
			}
		}
	}

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
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check for date rollover WHILE HOLDING LOCK (fixes race condition)
	today := time.Now().In(YouTubePST).Format("2006-01-02")
	if t.currentDate != today {
		t.logger.Info("Date rollover detected during RecordUsage",
			zap.String("old_date", t.currentDate),
			zap.String("new_date", today),
			zap.Int("old_usage", t.usageToday),
		)
		t.currentDate = today
		t.usageToday = 0
		t.lastNotifiedThreshold = 0.0 // Reset threshold notifications for new day

		// Reset state to healthy on new day
		if t.currentState != QuotaStateHealthy {
			t.currentState = QuotaStateHealthy
			t.lastStateTransition = time.Now()
			t.logger.Info("Quota state reset to HEALTHY on date rollover")
		}
	}

	// Update in-memory counter
	t.usageToday += units

	// Update database with retry logic
	if err := t.recordUsageWithRetry(ctx, units, 3); err != nil {
		t.logger.Error("Failed to record quota usage after retries",
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

// recordUsageWithRetry writes quota usage to database with exponential backoff retry
// This prevents quota loss from transient database connection issues
func (t *Tracker) recordUsageWithRetry(ctx context.Context, units int, maxRetries int) error {
	query := `
		INSERT INTO youtube_quota_usage (date, units_used, units_limit)
		VALUES ($1, $2, $3)
		ON CONFLICT (date)
		DO UPDATE SET
			units_used = youtube_quota_usage.units_used + EXCLUDED.units_used,
			updated_at = NOW()
	`

	backoff := 100 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := t.db.Exec(ctx, query, t.currentDate, units, t.dailyLimit)
		if err == nil {
			if attempt > 1 {
				t.logger.Info("Quota recorded successfully after retry",
					zap.Int("attempt", attempt),
					zap.Int("units", units),
				)
			}
			return nil
		}

		t.logger.Warn("Failed to record quota usage to database, retrying",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Int("units", units),
			zap.Error(err),
		)

		// Don't sleep on last attempt
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
				backoff *= 2  // Exponential backoff: 100ms, 200ms, 400ms
			}
		}
	}

	return fmt.Errorf("failed to record quota after %d retries", maxRetries)
}

// getCurrentDate returns the current date in PST timezone
func (t *Tracker) getCurrentDate() string {
	return time.Now().In(YouTubePST).Format("2006-01-02")
}

// ReserveQuota atomically reserves quota BEFORE making a YouTube API call
// Returns reservation ID on success, error if insufficient quota
// This is the first step in the reserve-confirm-rollback pattern
func (t *Tracker) ReserveQuota(ctx context.Context, units int) (string, error) {
	t.checkDateRollover()

	today := t.getCurrentDate()

	// Atomic reservation in database with row-level locking
	var canReserve bool
	err := t.db.QueryRow(ctx,
		`SELECT reserve_youtube_quota($1, $2, $3)`,
		today, units, t.dailyLimit,
	).Scan(&canReserve)

	if err != nil {
		t.logger.Error("Failed to reserve quota (database error)",
			zap.Int("units", units),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to reserve quota: %w", err)
	}

	if !canReserve {
		t.logger.Warn("Cannot reserve quota - insufficient available",
			zap.Int("units", units),
		)
		return "", fmt.Errorf("insufficient quota available")
	}

	// Generate unique reservation ID
	reservationID := fmt.Sprintf("%s-%d-%d", today, time.Now().UnixNano(), units)

	t.logger.Debug("Reserved quota",
		zap.String("reservation_id", reservationID),
		zap.Int("units", units),
	)

	return reservationID, nil
}

// ConfirmReservation confirms a reservation after successful YouTube API call
// Moves units from reserved -> used
func (t *Tracker) ConfirmReservation(ctx context.Context, reservationID string, units int) error {
	today := t.getCurrentDate()

	// Confirm in database with retry
	if err := t.confirmReservationWithRetry(ctx, today, units, 3); err != nil {
		t.logger.Error("Failed to confirm reservation after retries",
			zap.String("reservation_id", reservationID),
			zap.Int("units", units),
			zap.Error(err),
		)
		return err
	}

	// Update in-memory state
	t.mu.Lock()
	t.usageToday += units
	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	remaining := t.dailyLimit - t.usageToday

	// Update state machine
	t.updateStateAndNotify(percentage)

	// Update metrics
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)
	t.mu.Unlock()

	t.logger.Debug("Confirmed reservation",
		zap.String("reservation_id", reservationID),
		zap.Int("units", units),
		zap.Int("total_used", t.usageToday),
	)

	return nil
}

// RollbackReservation rolls back a reservation after failed YouTube API call
// Releases reserved units back to available pool
func (t *Tracker) RollbackReservation(ctx context.Context, reservationID string, units int) error {
	today := t.getCurrentDate()

	// Rollback in database with retry
	if err := t.rollbackReservationWithRetry(ctx, today, units, 3); err != nil {
		t.logger.Error("Failed to rollback reservation after retries",
			zap.String("reservation_id", reservationID),
			zap.Int("units", units),
			zap.Error(err),
		)
		return err
	}

	t.logger.Info("Rolled back reservation (API call failed before reaching YouTube)",
		zap.String("reservation_id", reservationID),
		zap.Int("units", units),
	)

	return nil
}

// confirmReservationWithRetry confirms reservation with exponential backoff retry
func (t *Tracker) confirmReservationWithRetry(ctx context.Context, date string, units int, maxRetries int) error {
	backoff := 100 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := t.db.Exec(ctx, `SELECT confirm_youtube_quota($1, $2)`, date, units)
		if err == nil {
			if attempt > 1 {
				t.logger.Info("Confirmed reservation after retry", zap.Int("attempt", attempt))
			}
			return nil
		}

		t.logger.Warn("Failed to confirm reservation, retrying",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Error(err),
		)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return fmt.Errorf("failed after %d retries", maxRetries)
}

// rollbackReservationWithRetry rolls back reservation with exponential backoff retry
func (t *Tracker) rollbackReservationWithRetry(ctx context.Context, date string, units int, maxRetries int) error {
	backoff := 100 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := t.db.Exec(ctx, `SELECT rollback_youtube_quota($1, $2)`, date, units)
		if err == nil {
			if attempt > 1 {
				t.logger.Info("Rolled back reservation after retry", zap.Int("attempt", attempt))
			}
			return nil
		}

		t.logger.Warn("Failed to rollback reservation, retrying",
			zap.Int("attempt", attempt),
			zap.Int("max_retries", maxRetries),
			zap.Error(err),
		)

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return fmt.Errorf("failed after %d retries", maxRetries)
}

// periodicReservationCleanup periodically cleans up stale reservations
// Recovers quota from crashed processes that didn't confirm or rollback
func (t *Tracker) periodicReservationCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	t.logger.Info("Started periodic reservation cleanup",
		zap.Duration("interval", 1*time.Minute),
	)

	for {
		select {
		case <-ticker.C:
			var recoveredUnits int
			err := t.db.QueryRow(ctx, `SELECT cleanup_stale_quota_reservations()`).Scan(&recoveredUnits)
			if err != nil {
				t.logger.Warn("Failed to cleanup stale reservations", zap.Error(err))
			} else if recoveredUnits > 0 {
				t.logger.Info("Recovered stale reserved quota units",
					zap.Int("units_recovered", recoveredUnits),
				)
			}
		case <-t.stopChan:
			t.logger.Info("Stopping periodic reservation cleanup")
			return
		case <-ctx.Done():
			t.logger.Info("Context cancelled, stopping reservation cleanup")
			return
		}
	}
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
// NOTE: Uses Pacific Time (PST/PDT) because YouTube quota resets at midnight PST
func (t *Tracker) loadTodayUsage(ctx context.Context) error {
	// Get current date in PST (YouTube's quota reset timezone)
	today := time.Now().In(YouTubePST).Format("2006-01-02")

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
			t.logger.Info("No usage record for today (PST), starting fresh",
				zap.String("date", today),
				zap.String("timezone", "America/Los_Angeles"),
			)
		} else {
			return fmt.Errorf("failed to load today's usage: %w", err)
		}
	}

	t.mu.Lock()
	t.currentDate = today
	t.usageToday = usageToday
	t.mu.Unlock()

	t.logger.Info("Loaded today's quota usage (PST timezone)",
		zap.String("date", today),
		zap.Int("used", usageToday),
		zap.Int("limit", t.dailyLimit),
		zap.Float64("percentage", float64(usageToday)/float64(t.dailyLimit)*100),
		zap.String("timezone", "America/Los_Angeles"),
	)

	return nil
}
