package quota

import (
	"context"
	"fmt"
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
)

// Tracker tracks YouTube API quota usage
type Tracker struct {
	db         *pgxpool.Pool
	logger     *zap.Logger
	metrics    *metrics.ListenerMetrics
	dailyLimit int

	mu          sync.RWMutex
	usageToday  int
	currentDate string
}

// NewTracker creates a new quota tracker
func NewTracker(db *pgxpool.Pool, dailyLimit int, logger *zap.Logger, m *metrics.ListenerMetrics) *Tracker {
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailyQuota
	}

	return &Tracker{
		db:         db,
		logger:     logger,
		metrics:    m,
		dailyLimit: dailyLimit,
	}
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

	return nil
}

// RecordUsage records API quota usage
func (t *Tracker) RecordUsage(ctx context.Context, units int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	today := time.Now().Format("2006-01-02")

	// Check if date rolled over
	if t.currentDate != today {
		t.logger.Info("Date rolled over, resetting quota",
			zap.String("old_date", t.currentDate),
			zap.String("new_date", today),
		)
		t.currentDate = today
		t.usageToday = 0
	}

	// Update in-memory counter
	t.usageToday += units

	// Update database
	query := `
		INSERT INTO youtube_quota_usage (date, units_used, units_limit)
		VALUES ($1, $2, $3)
		ON CONFLICT (date)
		DO UPDATE SET
			units_used = youtube_quota_usage.units_used + EXCLUDED.units_used,
			updated_at = NOW()
	`

	_, err := t.db.Exec(ctx, query, today, units, t.dailyLimit)
	if err != nil {
		t.logger.Error("Failed to record quota usage",
			zap.Int("units", units),
			zap.Error(err),
		)
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Check if approaching limit
	percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
	remaining := t.dailyLimit - t.usageToday

	// Record quota metrics
	t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
	t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)

	if percentage >= 90 {
		t.logger.Error("Quota usage critical",
			zap.Int("used", t.usageToday),
			zap.Int("limit", t.dailyLimit),
			zap.Float64("percentage", percentage),
		)
		t.metrics.RateLimitHits.WithLabelValues("youtube", "youtube-listener", "api_quota_critical").Inc()
	} else if percentage >= 80 {
		t.logger.Warn("Quota usage high",
			zap.Int("used", t.usageToday),
			zap.Int("limit", t.dailyLimit),
			zap.Float64("percentage", percentage),
		)
		t.metrics.RateLimitHits.WithLabelValues("youtube", "youtube-listener", "api_quota_warning").Inc()
	}

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
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.usageToday
}

// GetRemainingQuota returns remaining quota for today
func (t *Tracker) GetRemainingQuota() int {
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
	t.mu.RLock()
	defer t.mu.RUnlock()
	return float64(t.usageToday) / float64(t.dailyLimit) * 100
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
