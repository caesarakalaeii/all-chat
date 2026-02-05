package streams

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"go.uber.org/zap"
)

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	// CircuitClosed - Normal operation, allow requests
	CircuitClosed CircuitState = "CLOSED"

	// CircuitHalfOpen - Testing if service recovered, allow limited requests
	CircuitHalfOpen CircuitState = "HALF_OPEN"

	// CircuitOpen - Service unavailable, block expensive requests
	CircuitOpen CircuitState = "OPEN"
)

// CircuitBreaker prevents repeated expensive API calls for offline channels
type CircuitBreaker struct {
	channelID string
	logger    *zap.Logger
	metrics   *metrics.YouTubeMetrics

	mu                  sync.RWMutex
	state               CircuitState
	failureCount        int
	consecutiveFailures int
	successCount        int
	lastFailureTime     time.Time
	lastCheckTime       time.Time
	lastStateChange     time.Time

	// Configuration
	failureThreshold    int           // Open circuit after N consecutive failures (default: 5, was 3)
	successThreshold    int           // Close circuit after N consecutive successes in half-open (default: 2)
	openDuration        time.Duration // How long to keep circuit open (default: 10 minutes, was 30)
	halfOpenMaxAttempts int           // Max attempts in half-open state (default: 3)
}

// NewCircuitBreaker creates a new circuit breaker for a channel
func NewCircuitBreaker(channelID string, logger *zap.Logger, ytMetrics *metrics.YouTubeMetrics) *CircuitBreaker {
	// Load configurable parameters from environment variables with defaults
	// CHANGED FROM 3 to 5: Allow more failures before opening circuit
	failureThreshold := getEnvAsIntCB("CIRCUIT_BREAKER_FAILURE_THRESHOLD", 5)

	// CHANGED FROM 30 to 10: Recover 3x faster (10min vs 30min)
	openDurationMinutes := getEnvAsIntCB("CIRCUIT_BREAKER_OPEN_DURATION_MINUTES", 10)

	successThreshold := getEnvAsIntCB("CIRCUIT_BREAKER_SUCCESS_THRESHOLD", 2)
	halfOpenMaxAttempts := getEnvAsIntCB("CIRCUIT_BREAKER_HALF_OPEN_MAX_ATTEMPTS", 3)

	cb := &CircuitBreaker{
		channelID:           channelID,
		logger:              logger,
		metrics:             ytMetrics,
		state:               CircuitClosed,
		failureThreshold:    failureThreshold,
		successThreshold:    successThreshold,
		openDuration:        time.Duration(openDurationMinutes) * time.Minute,
		halfOpenMaxAttempts: halfOpenMaxAttempts,
		lastStateChange:     time.Now(),
	}

	// Initialize metrics
	if ytMetrics != nil {
		ytMetrics.CircuitBreakerState.WithLabelValues(channelID).Set(0) // CLOSED = 0
		ytMetrics.CircuitBreakerFailures.WithLabelValues(channelID).Set(0)
	}

	return cb
}

// getEnvAsIntCB gets an environment variable as int or returns default
func getEnvAsIntCB(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// CanAttemptDiscovery checks if expensive channel discovery should be attempted
func (cb *CircuitBreaker) CanAttemptDiscovery() (bool, string) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		// Normal operation - allow all requests
		return true, "circuit closed"

	case CircuitOpen:
		// Check if enough time has passed to transition to half-open
		timeSinceFailure := time.Since(cb.lastFailureTime)
		if timeSinceFailure >= cb.openDuration {
			// Transition to half-open (done in RecordFailure/Success with write lock)
			return true, "circuit transitioning to half-open"
		}

		// Still in open state - block expensive discovery
		remainingTime := cb.openDuration - timeSinceFailure
		return false, string("circuit open, retry in " + remainingTime.Round(time.Second).String())

	case CircuitHalfOpen:
		// In half-open state - allow limited attempts to test if service recovered
		return true, "circuit half-open, testing recovery"

	default:
		return false, "unknown circuit state"
	}
}

// RecordSuccess records a successful channel discovery
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastCheckTime = time.Now()
	cb.consecutiveFailures = 0 // Reset consecutive failures

	oldState := cb.state

	switch cb.state {
	case CircuitClosed:
		// Already closed, nothing to do
		cb.successCount++

	case CircuitHalfOpen:
		// In half-open, increment success count
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			// Enough successes - close the circuit
			cb.state = CircuitClosed
			cb.successCount = 0
			cb.failureCount = 0
			cb.lastStateChange = time.Now()

			cb.logger.Info("Circuit breaker closed after successful recovery",
				zap.String("channel_id", cb.channelID),
				zap.String("old_state", string(oldState)),
				zap.String("new_state", string(cb.state)),
			)

			// Update metrics
			if cb.metrics != nil {
				cb.metrics.CircuitBreakerTransitions.WithLabelValues(cb.channelID, string(oldState), string(cb.state)).Inc()
				cb.metrics.CircuitBreakerState.WithLabelValues(cb.channelID).Set(0) // CLOSED
				cb.metrics.CircuitBreakerFailures.WithLabelValues(cb.channelID).Set(0)
			}
		}

	case CircuitOpen:
		// Shouldn't happen (CanAttemptDiscovery should transition to half-open)
		// but handle it gracefully
		cb.state = CircuitHalfOpen
		cb.successCount = 1
		cb.lastStateChange = time.Now()

		cb.logger.Info("Circuit breaker transitioned to half-open on success",
			zap.String("channel_id", cb.channelID),
			zap.String("old_state", string(oldState)),
		)

		// Update metrics
		if cb.metrics != nil {
			cb.metrics.CircuitBreakerTransitions.WithLabelValues(cb.channelID, string(oldState), string(cb.state)).Inc()
			cb.metrics.CircuitBreakerState.WithLabelValues(cb.channelID).Set(1) // HALF_OPEN
		}
	}
}

// RecordFailure records a failed channel discovery (no stream found)
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastCheckTime = time.Now()
	cb.lastFailureTime = time.Now()
	cb.consecutiveFailures++
	cb.failureCount++
	cb.successCount = 0 // Reset success count

	oldState := cb.state

	// Update failure metrics
	if cb.metrics != nil {
		cb.metrics.CircuitBreakerFailures.WithLabelValues(cb.channelID).Set(float64(cb.consecutiveFailures))
	}

	switch cb.state {
	case CircuitClosed:
		// Check if we should open the circuit
		if cb.consecutiveFailures >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()

			quotaSaved := cb.failureCount * 100 // 100 units per Search.List

			cb.logger.Warn("Circuit breaker opened - channel appears offline",
				zap.String("channel_id", cb.channelID),
				zap.Int("consecutive_failures", cb.consecutiveFailures),
				zap.Int("total_failures", cb.failureCount),
				zap.Duration("block_duration", cb.openDuration),
				zap.Int("quota_saved_so_far", quotaSaved),
			)

			// Update metrics - state transition and quota saved
			if cb.metrics != nil {
				cb.metrics.CircuitBreakerTransitions.WithLabelValues(cb.channelID, string(oldState), string(cb.state)).Inc()
				cb.metrics.CircuitBreakerState.WithLabelValues(cb.channelID).Set(2) // OPEN
				cb.metrics.CircuitBreakerQuotaSaved.WithLabelValues(cb.channelID).Add(float64(quotaSaved))
			}
		}

	case CircuitHalfOpen:
		// Failed in half-open - go back to open
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()

		cb.logger.Warn("Circuit breaker reopened - channel still offline",
			zap.String("channel_id", cb.channelID),
			zap.String("old_state", string(oldState)),
			zap.Duration("block_duration", cb.openDuration),
		)

		// Update metrics
		if cb.metrics != nil {
			cb.metrics.CircuitBreakerTransitions.WithLabelValues(cb.channelID, string(oldState), string(cb.state)).Inc()
			cb.metrics.CircuitBreakerState.WithLabelValues(cb.channelID).Set(2) // OPEN
		}

	case CircuitOpen:
		// Already open, just log if this is unexpected
		// (shouldn't happen if CanAttemptDiscovery is used correctly)
		cb.logger.Debug("Failure recorded while circuit already open",
			zap.String("channel_id", cb.channelID),
		)
	}
}

// GetState returns the current circuit state and failure stats
func (cb *CircuitBreaker) GetState() (state CircuitState, failures int, lastFailure time.Time) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state, cb.consecutiveFailures, cb.lastFailureTime
}

// GetStats returns detailed circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stats := map[string]interface{}{
		"channel_id":           cb.channelID,
		"state":                string(cb.state),
		"consecutive_failures": cb.consecutiveFailures,
		"total_failures":       cb.failureCount,
		"success_count":        cb.successCount,
		"last_check":           cb.lastCheckTime.Format(time.RFC3339),
		"last_state_change":    cb.lastStateChange.Format(time.RFC3339),
		"quota_saved":          cb.failureCount * 100, // 100 units per blocked Search.List
	}

	if cb.state == CircuitOpen {
		remainingTime := cb.openDuration - time.Since(cb.lastFailureTime)
		if remainingTime > 0 {
			stats["retry_in_seconds"] = int(remainingTime.Seconds())
		} else {
			stats["retry_in_seconds"] = 0
		}
	}

	return stats
}

// Reset manually resets the circuit breaker to closed state
// This should only be used for manual intervention (e.g., admin command)
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = CircuitClosed
	cb.consecutiveFailures = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()

	if oldState != CircuitClosed {
		cb.logger.Info("Circuit breaker manually reset",
			zap.String("channel_id", cb.channelID),
			zap.String("old_state", string(oldState)),
		)
	}
}
