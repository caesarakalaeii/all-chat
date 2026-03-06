package deletion

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BatchDetector implements time-windowed batch deletion detection
// Distinguishes between single message deletions and batch moderation events (bans/timeouts)
type BatchDetector struct {
	threshold      int                       // Configurable threshold (default 5)
	windowDuration time.Duration             // Fixed 100ms detection window
	channels       map[string]*channelWindow // Per-channel state isolation
	mu             sync.RWMutex              // Protect channels map
	logger         *zap.Logger
}

// channelWindow maintains deletion state for a single channel
type channelWindow struct {
	deletions   []deletionEvent // Deletions in current window
	windowStart time.Time       // Current window start time
	ticker      *time.Ticker    // 100ms ticker for window boundaries
	mu          sync.Mutex      // Protect deletions slice
	stopChan    chan struct{}   // Signal to stop ticker goroutine
}

// deletionEvent represents a single deletion within a time window
type deletionEvent struct {
	targetItemID string
	timestamp    time.Time
}

// BatchResult contains metadata about detected batch or single deletion
type BatchResult struct {
	IsBatch bool   // True if deletion count >= threshold
	Count   int    // Number of deletions in this result
	Reason  string // "ban", "timeout", or "mod" based on count heuristic
}

// NewBatchDetector creates a new batch detector with configurable threshold
// If threshold <= 0, defaults to 5 deletions
func NewBatchDetector(threshold int, logger *zap.Logger) *BatchDetector {
	if threshold <= 0 {
		threshold = 5
	}

	return &BatchDetector{
		threshold:      threshold,
		windowDuration: 100 * time.Millisecond,
		channels:       make(map[string]*channelWindow),
		logger:         logger,
	}
}

// AddDeletion adds a deletion to the current window for the given channel
// Returns BatchResult immediately if threshold crossed, nil otherwise
// Caller must handle the result and emit events accordingly
func (d *BatchDetector) AddDeletion(channelID, targetItemID string, timestamp time.Time) (*BatchResult, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if targetItemID == "" {
		return nil, fmt.Errorf("targetItemID is required")
	}

	d.mu.Lock()
	window, exists := d.channels[channelID]
	if !exists {
		// Create new channel window
		window = &channelWindow{
			deletions:   make([]deletionEvent, 0),
			windowStart: timestamp,
			ticker:      time.NewTicker(d.windowDuration),
			stopChan:    make(chan struct{}),
		}
		d.channels[channelID] = window

		// Start ticker goroutine to process windows
		go d.tickerLoop(channelID, window)
	}
	d.mu.Unlock()

	// Add deletion to current window
	window.mu.Lock()
	window.deletions = append(window.deletions, deletionEvent{
		targetItemID: targetItemID,
		timestamp:    timestamp,
	})

	// Check if threshold crossed with this deletion
	deletionCount := len(window.deletions)
	window.mu.Unlock()

	// Return BatchResult immediately if threshold reached
	if deletionCount >= d.threshold {
		// Classify reason based on count
		var reason string
		if deletionCount >= 20 {
			reason = "ban"
		} else if deletionCount >= 5 {
			reason = "timeout"
		} else {
			reason = "mod"
		}

		return &BatchResult{
			IsBatch: true,
			Count:   deletionCount,
			Reason:  reason,
		}, nil
	}

	// Below threshold, return nil
	return nil, nil
}

// tickerLoop runs in a goroutine to process time windows at regular intervals
// Used for window cleanup and final processing at window boundaries
func (d *BatchDetector) tickerLoop(channelID string, window *channelWindow) {
	for {
		select {
		case <-window.ticker.C:
			// Window boundary reached, reset for next window
			// Note: AddDeletion now returns results immediately when threshold crossed
			// Ticker is kept for window cleanup and edge case handling
			result := d.processWindow(channelID)
			if result != nil {
				d.logger.Debug("Window processed",
					zap.String("channel_id", channelID),
					zap.Bool("is_batch", result.IsBatch),
					zap.Int("count", result.Count),
					zap.String("reason", result.Reason),
				)
			}

		case <-window.stopChan:
			// Cleanup requested, stop ticker
			window.ticker.Stop()
			return
		}
	}
}

// processWindow analyzes the current window and returns batch detection metadata
// Resets the window for the next interval
func (d *BatchDetector) processWindow(channelID string) *BatchResult {
	d.mu.RLock()
	window, exists := d.channels[channelID]
	d.mu.RUnlock()

	if !exists {
		return nil
	}

	window.mu.Lock()
	deletionCount := len(window.deletions)

	// Early exit if window is empty
	if deletionCount == 0 {
		window.mu.Unlock()
		return nil
	}

	// Determine if this is a batch deletion
	isBatch := deletionCount >= d.threshold

	// Determine reason based on deletion count heuristic
	var reason string
	if deletionCount >= 20 {
		reason = "ban"
	} else if deletionCount >= 5 {
		reason = "timeout"
	} else {
		reason = "mod"
	}

	// Reset window for next interval
	window.deletions = make([]deletionEvent, 0)
	window.windowStart = time.Now()
	window.mu.Unlock()

	return &BatchResult{
		IsBatch: isBatch,
		Count:   deletionCount,
		Reason:  reason,
	}
}

// Cleanup removes channel state when stream stops
// Prevents memory leak from abandoned channels
func (d *BatchDetector) Cleanup(channelID string) error {
	if channelID == "" {
		return fmt.Errorf("channelID is required")
	}

	d.mu.Lock()
	window, exists := d.channels[channelID]
	if !exists {
		d.mu.Unlock()
		return nil // Already cleaned up
	}

	// Check deletion count before processing
	window.mu.Lock()
	deletionCount := len(window.deletions)
	window.mu.Unlock()

	// Release lock before calling processWindow to avoid deadlock
	d.mu.Unlock()

	// Process any remaining deletions in the window before cleanup
	if deletionCount > 0 {
		d.logger.Debug("Processing final window during cleanup",
			zap.String("channel_id", channelID),
			zap.Int("deletions", deletionCount),
		)
		// Process the final window
		d.processWindow(channelID)
	}

	// Reacquire lock for cleanup
	d.mu.Lock()
	// Recheck existence in case another goroutine cleaned up
	window, exists = d.channels[channelID]
	if !exists {
		d.mu.Unlock()
		return nil
	}

	// Stop ticker goroutine
	close(window.stopChan)

	// Remove from map
	delete(d.channels, channelID)
	d.mu.Unlock()

	d.logger.Debug("Cleaned up batch detector state",
		zap.String("channel_id", channelID),
	)

	return nil
}

// GetThreshold returns the configured threshold for testing
func (d *BatchDetector) GetThreshold() int {
	return d.threshold
}
