package streams

import (
	"math"
	"time"
)

// BackoffStrategy implements context-aware exponential backoff for stream detection
// It adjusts backoff intervals based on channel history and activity patterns
type BackoffStrategy struct {
	// Channel history
	LastSeenLive      time.Time
	TotalStreamsFound int
	ConsecutiveOffline int

	// Current state
	CurrentInterval   time.Duration
	FailureCount     int

	// Configuration
	PriorityTier     string // "high", "standard", "low"
	BaseInterval     time.Duration
	MaxInterval      time.Duration
}

// BackoffConfig holds configuration for backoff strategies
type BackoffConfig struct {
	// Base intervals per tier
	HighTierInterval     time.Duration
	StandardTierInterval time.Duration
	LowTierInterval      time.Duration

	// Backoff multipliers
	GentleBackoffMultiplier     float64 // For recently active channels
	GentleBackoffCap            float64 // Max multiplier for gentle backoff
	AggressiveBackoffMultiplier float64 // For cold channels
	AggressiveBackoffCap        float64 // Max multiplier for aggressive backoff

	// Activity thresholds
	RecentActivityWindow time.Duration // Consider channel "active" if live in this window
	ColdChannelThreshold time.Duration // Consider channel "cold" after this period
}

// DefaultBackoffConfig returns the default configuration
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		HighTierInterval:            30 * time.Second,
		StandardTierInterval:        2 * time.Minute,
		LowTierInterval:             10 * time.Minute,
		GentleBackoffMultiplier:     1.2,
		GentleBackoffCap:            2.0,
		AggressiveBackoffMultiplier: 2.0,
		AggressiveBackoffCap:        60.0,
		RecentActivityWindow:        24 * time.Hour,
		ColdChannelThreshold:        7 * 24 * time.Hour,
	}
}

// NewBackoffStrategy creates a new backoff strategy for a channel
func NewBackoffStrategy(tier string, config BackoffConfig) *BackoffStrategy {
	baseInterval := config.StandardTierInterval
	switch tier {
	case "high":
		baseInterval = config.HighTierInterval
	case "low":
		baseInterval = config.LowTierInterval
	}

	return &BackoffStrategy{
		PriorityTier:    tier,
		BaseInterval:    baseInterval,
		MaxInterval:     10 * time.Minute, // Hard cap at 10 minutes
		CurrentInterval: baseInterval,
	}
}

// NextInterval calculates the next backoff interval based on channel context
func (b *BackoffStrategy) NextInterval(config BackoffConfig) time.Duration {
	now := time.Now()

	// Reset conditions: should we reset to base interval?
	if b.shouldResetBackoff(now, config) {
		b.FailureCount = 0
		b.CurrentInterval = b.BaseInterval
		return b.BaseInterval
	}

	// Recent activity: slower backoff growth (more frequent checks)
	if b.wasRecentlyActive(now, config) {
		return b.getGentleBackoff(config)
	}

	// Cold channel: aggressive backoff (less frequent checks)
	if b.isColdChannel(now, config) {
		return b.getAggressiveBackoff(config)
	}

	// Standard exponential backoff
	return b.getStandardBackoff()
}

// shouldResetBackoff determines if backoff should be reset to base interval
func (b *BackoffStrategy) shouldResetBackoff(now time.Time, config BackoffConfig) bool {
	// Reset if channel was recently live (within 30 minutes)
	if !b.LastSeenLive.IsZero() && now.Sub(b.LastSeenLive) < 30*time.Minute {
		return true
	}

	// Reset if this is the first offline check after being live
	if !b.LastSeenLive.IsZero() && b.ConsecutiveOffline == 1 {
		return true
	}

	return false
}

// wasRecentlyActive checks if channel was live in the recent activity window
func (b *BackoffStrategy) wasRecentlyActive(now time.Time, config BackoffConfig) bool {
	if b.LastSeenLive.IsZero() {
		return false
	}
	return now.Sub(b.LastSeenLive) < config.RecentActivityWindow
}

// isColdChannel checks if channel hasn't been active for a long time
func (b *BackoffStrategy) isColdChannel(now time.Time, config BackoffConfig) bool {
	// Never seen live, or not live for >7 days
	if b.TotalStreamsFound == 0 {
		return true
	}
	if b.LastSeenLive.IsZero() {
		return true
	}
	return now.Sub(b.LastSeenLive) > config.ColdChannelThreshold
}

// getGentleBackoff calculates backoff for recently active channels
// Uses slower growth rate and lower cap for more responsive detection
func (b *BackoffStrategy) getGentleBackoff(config BackoffConfig) time.Duration {
	// Gentle backoff: 1.2^n growth, capped at 2x base interval
	multiplier := math.Min(
		math.Pow(config.GentleBackoffMultiplier, float64(b.FailureCount)),
		config.GentleBackoffCap,
	)

	b.CurrentInterval = time.Duration(float64(b.BaseInterval) * multiplier)
	if b.CurrentInterval > b.MaxInterval {
		b.CurrentInterval = b.MaxInterval
	}

	return b.CurrentInterval
}

// getAggressiveBackoff calculates backoff for cold channels
// Uses faster growth rate and higher cap to reduce unnecessary API calls
func (b *BackoffStrategy) getAggressiveBackoff(config BackoffConfig) time.Duration {
	// Aggressive backoff: 2^n growth, capped at 60x base interval
	multiplier := math.Min(
		math.Pow(config.AggressiveBackoffMultiplier, float64(b.FailureCount)),
		config.AggressiveBackoffCap,
	)

	b.CurrentInterval = time.Duration(float64(b.BaseInterval) * multiplier)
	if b.CurrentInterval > b.MaxInterval {
		b.CurrentInterval = b.MaxInterval
	}

	return b.CurrentInterval
}

// getStandardBackoff calculates standard exponential backoff
func (b *BackoffStrategy) getStandardBackoff() time.Duration {
	// Standard exponential backoff: 1.5^n growth
	multiplier := math.Pow(1.5, float64(b.FailureCount))
	b.CurrentInterval = time.Duration(float64(b.BaseInterval) * multiplier)

	// Cap at max interval
	if b.CurrentInterval > b.MaxInterval {
		b.CurrentInterval = b.MaxInterval
	}

	return b.CurrentInterval
}

// Reset resets the backoff strategy to initial state
func (b *BackoffStrategy) Reset() {
	b.FailureCount = 0
	b.CurrentInterval = b.BaseInterval
	b.ConsecutiveOffline = 0
}

// RecordFailure increments the failure count (stream not found)
func (b *BackoffStrategy) RecordFailure() {
	b.FailureCount++
	b.ConsecutiveOffline++
}

// RecordSuccess resets failure count (stream found)
func (b *BackoffStrategy) RecordSuccess() {
	b.FailureCount = 0
	b.ConsecutiveOffline = 0
	b.LastSeenLive = time.Now()
	b.TotalStreamsFound++
}

// RecordStreamEnd records that a stream ended gracefully
// This triggers backoff reset for quick re-detection
func (b *BackoffStrategy) RecordStreamEnd() {
	b.FailureCount = 0
	b.ConsecutiveOffline = 0
	b.CurrentInterval = b.BaseInterval
}

// GetCurrentInterval returns the current backoff interval
func (b *BackoffStrategy) GetCurrentInterval() time.Duration {
	return b.CurrentInterval
}

// ShouldCheckNow determines if enough time has passed to check again
func (b *BackoffStrategy) ShouldCheckNow(lastCheckTime time.Time) bool {
	if lastCheckTime.IsZero() {
		return true // Never checked, check now
	}
	return time.Since(lastCheckTime) >= b.CurrentInterval
}
