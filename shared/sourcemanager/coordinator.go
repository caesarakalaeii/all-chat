package sourcemanager

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LeadershipCoordinator maintains leadership leases per stream ID.
type LeadershipCoordinator struct {
	platform string
	callerID string // stable identity for this process, passed to source-manager
	client   LeadershipClient
	logger   *zap.Logger

	mu       sync.Mutex
	leases   map[string]*leaseEntry
	interval time.Duration
}

type leaseEntry struct {
	streamID     string
	stopCh       chan struct{}
	lostCallback func()
}

// NewLeadershipCoordinator returns a coordinator for a specific platform.
// A stable callerID is generated once per process so claim/renew/release always
// carry the same identity regardless of which source-manager pod handles the request.
func NewLeadershipCoordinator(platform string, client LeadershipClient, interval time.Duration, logger *zap.Logger) *LeadershipCoordinator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	coord := &LeadershipCoordinator{
		platform: platform,
		callerID: uuid.New().String(),
		client:   client,
		logger:   logger,
		interval: interval,
		leases:   make(map[string]*leaseEntry),
	}
	setLeadershipActive(platform, 0)
	return coord
}

// EnsureLeadership tries to acquire leadership for streamID. When leadership is lost,
// lostCallback is invoked asynchronously to allow the caller to react.
func (c *LeadershipCoordinator) EnsureLeadership(ctx context.Context, streamID string, lostCallback func()) (bool, error) {
	if c == nil || c.client == nil {
		return true, nil
	}

	c.mu.Lock()
	if lease, ok := c.leases[streamID]; ok {
		if lostCallback != nil {
			lease.lostCallback = lostCallback
		}
		c.mu.Unlock()
		return true, nil
	}
	c.mu.Unlock()

	acquired, err := c.client.ClaimLeadership(ctx, c.platform, streamID, c.callerID)
	if err != nil {
		c.observe("claim_error")
		return false, err
	}
	if !acquired {
		c.observe("claim_skipped")
		return false, nil
	}
	c.observe("claim_success")

	entry := &leaseEntry{
		streamID:     streamID,
		stopCh:       make(chan struct{}),
		lostCallback: lostCallback,
	}

	c.mu.Lock()
	c.leases[streamID] = entry
	c.setActiveGaugeLocked()
	c.mu.Unlock()

	go c.heartbeat(entry)

	return true, nil
}

// Release relinquishes leadership for streamID.
func (c *LeadershipCoordinator) Release(streamID string) {
	if c == nil || c.client == nil {
		return
	}

	var entry *leaseEntry
	c.mu.Lock()
	if le, ok := c.leases[streamID]; ok {
		entry = le
		delete(c.leases, streamID)
		c.setActiveGaugeLocked()
		close(le.stopCh)
	}
	c.mu.Unlock()

	if entry == nil {
		return
	}

	go func() {
		if err := c.client.ReleaseLeadership(context.Background(), c.platform, streamID, c.callerID); err != nil {
			if c.logger != nil {
				c.logger.Warn("Failed to release leadership",
					zap.String("platform", c.platform),
					zap.String("stream_id", streamID),
					zap.Error(err),
				)
			}
			c.observe("release_error")
			return
		}
		c.observe("released")
	}()
}

// Stop releases all leases and stops the coordinator.
func (c *LeadershipCoordinator) Stop() {
	if c == nil || c.client == nil {
		return
	}

	c.mu.Lock()
	streams := make([]string, 0, len(c.leases))
	for id, lease := range c.leases {
		streams = append(streams, id)
		close(lease.stopCh)
	}
	c.leases = make(map[string]*leaseEntry)
	c.setActiveGaugeLocked()
	c.mu.Unlock()

	for _, id := range streams {
		go func(streamID string) {
			if err := c.client.ReleaseLeadership(context.Background(), c.platform, streamID, c.callerID); err != nil {
				if c.logger != nil {
					c.logger.Warn("Failed to release leadership",
						zap.String("platform", c.platform),
						zap.String("stream_id", streamID),
						zap.Error(err),
					)
				}
				c.observe("release_error")
			} else {
				c.observe("released")
			}
		}(id)
	}
}

func (c *LeadershipCoordinator) heartbeat(entry *leaseEntry) {
	// Perform initial renewal immediately to prevent 5-second gap
	// This prevents leadership loss due to ticker not firing immediately
	if !c.tryRenewWithRetry(entry) {
		return // Lost leadership on initial renewal
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	consecutiveFailures := 0
	const failureThreshold = 2 // Allow 2 failures (10s grace period)

	for {
		select {
		case <-entry.stopCh:
			return
		case <-ticker.C:
			if !c.tryRenewWithRetry(entry) {
				consecutiveFailures++

				if consecutiveFailures >= failureThreshold {
					// Lost leadership after grace period
					c.observe("renew_error")
					if c.logger != nil {
						c.logger.Error("Failed to renew leadership after grace period, treating as leadership loss",
							zap.String("platform", c.platform),
							zap.String("stream_id", entry.streamID),
							zap.Int("consecutive_failures", consecutiveFailures),
						)
					}

					// Clean up lease and trigger callback
					c.mu.Lock()
					if current, ok := c.leases[entry.streamID]; ok && current == entry {
						delete(c.leases, entry.streamID)
						c.setActiveGaugeLocked()
					}
					c.mu.Unlock()

					if entry.lostCallback != nil {
						go entry.lostCallback()
					}
					return
				}

				if c.logger != nil {
					c.logger.Warn("Leadership renewal failed, retrying within grace period",
						zap.String("platform", c.platform),
						zap.String("stream_id", entry.streamID),
						zap.Int("consecutive_failures", consecutiveFailures),
						zap.Int("threshold", failureThreshold),
					)
				}
				continue
			}

			// Reset failure counter on success
			if consecutiveFailures > 0 {
				if c.logger != nil {
					c.logger.Info("Leadership renewal recovered after failures",
						zap.String("platform", c.platform),
						zap.String("stream_id", entry.streamID),
						zap.Int("previous_failures", consecutiveFailures),
					)
				}
				consecutiveFailures = 0
			}
		}
	}
}

// tryRenewWithRetry attempts to renew leadership with exponential backoff retry.
// Returns true if renewal succeeded, false if all retries failed.
func (c *LeadershipCoordinator) tryRenewWithRetry(entry *leaseEntry) bool {
	const maxRetries = 3
	const baseDelay = 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		ok, err := c.client.RenewLeadership(context.Background(), c.platform, entry.streamID, c.callerID)

		if err == nil {
			if !ok {
				// Leadership genuinely lost (another pod has it)
				return false
			}
			// Success
			if attempt > 0 && c.logger != nil {
				c.logger.Info("Leadership renewal succeeded after retry",
					zap.String("platform", c.platform),
					zap.String("stream_id", entry.streamID),
					zap.Int("attempts", attempt+1),
				)
			}
			c.observe("renew_success")
			return true
		}

		// Error occurred - retry with backoff
		if attempt < maxRetries-1 {
			delay := baseDelay * (1 << uint(attempt)) // 100ms, 200ms, 400ms
			if c.logger != nil {
				c.logger.Warn("Leadership renewal failed, retrying",
					zap.String("platform", c.platform),
					zap.String("stream_id", entry.streamID),
					zap.Error(err),
					zap.Int("attempt", attempt+1),
					zap.Int("max_retries", maxRetries),
					zap.Duration("retry_delay", delay),
				)
			}
			time.Sleep(delay)
		}
	}

	// All retries failed
	c.observe("renew_error_after_retries")
	if c.logger != nil {
		c.logger.Error("Leadership renewal failed after all retries",
			zap.String("platform", c.platform),
			zap.String("stream_id", entry.streamID),
			zap.Int("max_retries", maxRetries),
		)
	}
	return false
}

func (c *LeadershipCoordinator) observe(event string) {
	if c == nil {
		return
	}
	observeLeadershipEvent(c.platform, event)
}

func (c *LeadershipCoordinator) setActiveGaugeLocked() {
	setLeadershipActive(c.platform, len(c.leases))
}
