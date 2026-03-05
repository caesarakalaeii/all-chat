package deletion

import (
	"container/ring"
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"go.uber.org/zap"
)

// Publisher interface for publishing deletion events to Redis Streams
// Abstraction allows for testing without Redis dependency
type Publisher interface {
	Publish(ctx context.Context, msg *innertube.RawChatMessage) error
}

// DeletionBuffer manages per-channel deletion event buffering with 500ms delay
// Purpose: Prevent race condition where deletion arrives before original message is indexed
type DeletionBuffer struct {
	bufferDuration time.Duration                 // Fixed 500ms delay before emission
	maxSize        int                           // Fixed 1000 events per channel buffer
	flushInterval  time.Duration                 // 100ms flush check interval (5x per buffer window)
	channels       map[string]*channelBuffer     // Per-channel buffers
	publisher      Publisher                     // Redis publisher for deletion events
	mu             sync.RWMutex                  // Protect channels map
	logger         *zap.Logger                   // Structured logging
	ctx            context.Context               // Parent context for goroutines
	cancel         context.CancelFunc            // Cancel function for cleanup
	wg             sync.WaitGroup                // Wait group for goroutine synchronization
}

// channelBuffer represents a single channel's deletion event buffer
type channelBuffer struct {
	ring     *ring.Ring        // Circular buffer for fixed-size FIFO storage
	mu       sync.Mutex        // Protect ring access during flush/add
	ticker   *time.Ticker      // Periodic flush ticker (100ms intervals)
	stopChan chan struct{}     // Signal to stop flusher goroutine
	count    int               // Current number of buffered events (for overflow detection)
}

// bufferedEvent wraps a deletion event with its buffer entry timestamp
type bufferedEvent struct {
	message *innertube.RawChatMessage // Deletion event to emit after delay
	addedAt time.Time                 // When added to buffer (for expiration check)
}

// NewDeletionBuffer creates a new deletion buffer with default configuration
// Default: 500ms delay, 1000 event buffer per channel, 100ms flush interval
func NewDeletionBuffer(publisher Publisher, logger *zap.Logger) *DeletionBuffer {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeletionBuffer{
		bufferDuration: 500 * time.Millisecond,
		maxSize:        1000,
		flushInterval:  100 * time.Millisecond,
		channels:       make(map[string]*channelBuffer),
		publisher:      publisher,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Add adds a deletion event to the specified channel's buffer
// Creates channel buffer lazily on first event
// FIFO overflow strategy: drops oldest event when buffer full
func (db *DeletionBuffer) Add(channelID string, deletionEvent *innertube.RawChatMessage) error {
	// Get or create channel buffer
	db.mu.Lock()
	cb, exists := db.channels[channelID]
	if !exists {
		// Create new channel buffer
		cb = &channelBuffer{
			ring:     ring.New(db.maxSize),
			stopChan: make(chan struct{}),
			count:    0,
		}
		db.channels[channelID] = cb

		// Start flusher goroutine for this channel
		db.startFlusher(channelID, cb)
	}
	db.mu.Unlock()

	// Add event to ring buffer
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check for overflow (ring slot has non-nil value)
	if cb.ring.Value != nil {
		// Buffer full, dropping oldest event (FIFO strategy)
		oldEvent := cb.ring.Value.(*bufferedEvent)
		db.logger.Warn("Deletion buffer overflow, dropping oldest event",
			zap.String("channel_id", channelID),
			zap.String("dropped_message_id", oldEvent.message.MessageID),
			zap.Int("buffer_size", db.maxSize),
		)
	} else {
		cb.count++
	}

	// Store new event in current ring position
	cb.ring.Value = &bufferedEvent{
		message: deletionEvent,
		addedAt: time.Now(),
	}

	// Move to next position (automatic wraparound)
	cb.ring = cb.ring.Next()

	return nil
}

// startFlusher starts a background goroutine that periodically flushes expired events
// Runs every 100ms to check for events older than 500ms
func (db *DeletionBuffer) startFlusher(channelID string, cb *channelBuffer) {
	cb.ticker = time.NewTicker(db.flushInterval)

	db.wg.Add(1)
	go func() {
		defer db.wg.Done()
		defer cb.ticker.Stop()

		for {
			select {
			case <-cb.stopChan:
				// Cleanup signal received, flush remaining and exit
				db.flushExpired(channelID, cb)
				return
			case <-db.ctx.Done():
				// Global shutdown, flush remaining and exit
				db.flushExpired(channelID, cb)
				return
			case <-cb.ticker.C:
				// Periodic flush check
				db.flushExpired(channelID, cb)
			}
		}
	}()
}

// flushExpired walks the ring buffer and publishes events older than bufferDuration (500ms)
// Events are published in ring order (insertion order preserved)
func (db *DeletionBuffer) flushExpired(channelID string, cb *channelBuffer) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	flushedCount := 0

	// Walk entire ring to check all buffered events
	cb.ring.Do(func(v interface{}) {
		if v == nil {
			return // Empty slot, skip
		}

		event := v.(*bufferedEvent)
		age := now.Sub(event.addedAt)

		if age >= db.bufferDuration {
			// Event expired, publish to Redis
			ctx := context.Background() // Use background context for publish
			if err := db.publisher.Publish(ctx, event.message); err != nil {
				// Log error but continue flushing (eventual consistency per user decision)
				db.logger.Error("Failed to publish deletion event from buffer",
					zap.String("channel_id", channelID),
					zap.String("message_id", event.message.MessageID),
					zap.Duration("age", age),
					zap.Error(err),
				)
				// Drop event on failure per eventual consistency decision
			} else {
				db.logger.Debug("Published deletion event from buffer",
					zap.String("channel_id", channelID),
					zap.String("message_id", event.message.MessageID),
					zap.Duration("age", age),
				)
				flushedCount++
			}

			// Mark slot as flushed (set to nil)
			// Note: We're inside cb.ring.Do(), so we need to modify via pointer
			// Since Do() doesn't provide direct access to modify, we'll clear after walk
		}
	})

	// Second pass: clear flushed slots
	// Walk ring again and clear expired slots
	if flushedCount > 0 {
		cb.ring.Do(func(v interface{}) {
			if v == nil {
				return
			}

			event := v.(*bufferedEvent)
			age := now.Sub(event.addedAt)

			if age >= db.bufferDuration {
				// Clear this slot
				// Since Do() gives us value not pointer, we need different approach
				// We'll track flushed and clear by recreating ring with non-flushed events
			}
		})

		// Simpler approach: mark ring positions as nil during single walk
		// Create a slice to track which values to clear
		var toClear []*ring.Ring
		r := cb.ring
		for i := 0; i < cb.ring.Len(); i++ {
			if r.Value != nil {
				event := r.Value.(*bufferedEvent)
				age := now.Sub(event.addedAt)
				if age >= db.bufferDuration {
					toClear = append(toClear, r)
				}
			}
			r = r.Next()
		}

		// Clear the marked positions
		for _, ringPos := range toClear {
			ringPos.Value = nil
			cb.count--
		}

		if flushedCount > 0 {
			db.logger.Debug("Flushed expired deletion events",
				zap.String("channel_id", channelID),
				zap.Int("flushed", flushedCount),
				zap.Int("remaining", cb.count),
			)
		}
	}
}

// Cleanup stops the flusher, flushes remaining events, and removes channel buffer
// Called when stream goes offline to prevent memory leaks
func (db *DeletionBuffer) Cleanup(channelID string) {
	db.mu.Lock()
	cb, exists := db.channels[channelID]
	if !exists {
		db.mu.Unlock()
		return
	}

	// Remove from map immediately to prevent new additions
	delete(db.channels, channelID)
	db.mu.Unlock()

	// Stop flusher goroutine
	close(cb.stopChan)

	// Flush all remaining events (don't wait for expiration)
	db.flushAll(channelID, cb)

	db.logger.Info("Cleaned up deletion buffer",
		zap.String("channel_id", channelID),
	)
}

// flushAll flushes all buffered events regardless of age
// Used during cleanup to emit remaining events before shutdown
func (db *DeletionBuffer) flushAll(channelID string, cb *channelBuffer) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	flushedCount := 0
	ctx := context.Background()

	// Walk ring and publish all non-nil events
	cb.ring.Do(func(v interface{}) {
		if v == nil {
			return
		}

		event := v.(*bufferedEvent)
		if err := db.publisher.Publish(ctx, event.message); err != nil {
			db.logger.Error("Failed to publish deletion event during cleanup",
				zap.String("channel_id", channelID),
				zap.String("message_id", event.message.MessageID),
				zap.Error(err),
			)
		} else {
			flushedCount++
		}
	})

	if flushedCount > 0 {
		db.logger.Info("Flushed remaining deletion events during cleanup",
			zap.String("channel_id", channelID),
			zap.Int("flushed", flushedCount),
		)
	}
}

// Shutdown gracefully shuts down the deletion buffer
// Stops all flushers, flushes remaining events, waits for goroutines
func (db *DeletionBuffer) Shutdown() {
	db.logger.Info("Shutting down deletion buffer")

	// Cancel context to signal all flushers
	db.cancel()

	// Get all channel IDs
	db.mu.RLock()
	channelIDs := make([]string, 0, len(db.channels))
	for channelID := range db.channels {
		channelIDs = append(channelIDs, channelID)
	}
	db.mu.RUnlock()

	// Cleanup all channels
	for _, channelID := range channelIDs {
		db.Cleanup(channelID)
	}

	// Wait for all flusher goroutines to complete
	db.wg.Wait()

	db.logger.Info("Deletion buffer shutdown complete")
}
