package sourcemanager

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LeadershipCoordinator maintains leadership leases per stream ID.
type LeadershipCoordinator struct {
	platform string
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
func NewLeadershipCoordinator(platform string, client LeadershipClient, interval time.Duration, logger *zap.Logger) *LeadershipCoordinator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	coord := &LeadershipCoordinator{
		platform: platform,
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

	acquired, err := c.client.ClaimLeadership(ctx, c.platform, streamID)
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
		if err := c.client.ReleaseLeadership(context.Background(), c.platform, streamID); err != nil {
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
			if err := c.client.ReleaseLeadership(context.Background(), c.platform, streamID); err != nil {
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
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-entry.stopCh:
			return
		case <-ticker.C:
			ok, err := c.client.RenewLeadership(context.Background(), c.platform, entry.streamID)
			if err != nil {
				// CRITICAL FIX: Treat renewal errors as leadership loss to prevent duplicate polling
				// When renewal fails, the Redis key will expire and another pod will acquire leadership
				// If we continue polling without successful renewal, we get duplicate polling!
				c.observe("renew_error")
				if c.logger != nil {
					c.logger.Error("Failed to renew leadership, treating as leadership loss",
						zap.String("platform", c.platform),
						zap.String("stream_id", entry.streamID),
						zap.Error(err),
					)
				}

				// Clean up lease and trigger callback (same as leadership loss)
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
			if !ok {
				c.observe("lost")
				if c.logger != nil {
					c.logger.Warn("Leadership lost",
						zap.String("platform", c.platform),
						zap.String("stream_id", entry.streamID),
					)
				}
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
		}
	}
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
