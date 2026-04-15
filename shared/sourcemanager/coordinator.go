// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package sourcemanager

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DefaultRebalanceStabilizationPeriod is how long the peer count must be stable
// before leases are released during rebalancing. This prevents oscillation when
// pods scale up/down rapidly: without a stabilization window, a new pod appearing
// causes incumbents to shed leases, which the newcomer immediately acquires,
// then the newcomer also rebalances and releases some back — creating churn.
const DefaultRebalanceStabilizationPeriod = 30 * time.Second

// LeadershipCoordinator maintains leadership leases per stream ID.
type LeadershipCoordinator struct {
	platform string
	callerID string // stable identity for this process, passed to source-manager
	client   LeadershipClient
	logger   *zap.Logger

	mu       sync.Mutex
	leases   map[string]*leaseEntry
	interval time.Duration

	// stabilizationPeriod is how long the peer count must be stable before
	// leases are released. Configurable so tests can set it to 0.
	stabilizationPeriod time.Duration

	// Rebalance stabilization: track the last observed peer count and when it
	// was first seen so we only shed leases after the count has been stable.
	lastPeerCount     int
	peerCountStableAt time.Time
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
		platform:            platform,
		callerID:            uuid.New().String(),
		client:              client,
		logger:              logger,
		interval:            interval,
		leases:              make(map[string]*leaseEntry),
		stabilizationPeriod: DefaultRebalanceStabilizationPeriod,
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

// LeaseCount returns the number of active leases held by this coordinator.
func (c *LeadershipCoordinator) LeaseCount() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.leases)
}

// HeldStreamIDs returns the stream IDs currently held by this coordinator.
func (c *LeadershipCoordinator) HeldStreamIDs() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.leases))
	for id := range c.leases {
		ids = append(ids, id)
	}
	return ids
}

// Rebalance checks the number of active peers for this platform and releases
// excess leases so that each pod holds at most ceil(totalStreams/peerCount).
// It returns the stream IDs that were released so the caller can disconnect them.
//
// To prevent oscillation during pod scaling events, leases are only released
// after the peer count has been stable for rebalanceStabilizationPeriod. If the
// peer count just changed this call registers the new count and returns without
// releasing anything; the next call (after the stabilization window) will act.
func (c *LeadershipCoordinator) Rebalance(ctx context.Context, totalStreams int) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}

	peerCount, err := c.client.RegisterPeer(ctx, c.platform, c.callerID)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("Failed to register peer for rebalancing",
				zap.String("platform", c.platform),
				zap.Error(err),
			)
		}
		return nil, err
	}

	if peerCount <= 0 {
		peerCount = 1
	}
	leadershipPeerCount.WithLabelValues(sanitizeLabel(c.platform)).Set(float64(peerCount))
	leadershipDesired.WithLabelValues(sanitizeLabel(c.platform)).Set(float64(totalStreams))

	// Stabilization gate: only shed leases once the peer count has been stable
	// for stabilizationPeriod. This prevents the release→re-acquire oscillation
	// that occurs when pods scale up/down. When stabilizationPeriod == 0 (tests)
	// the gate is bypassed entirely.
	c.mu.Lock()
	now := time.Now()
	if c.stabilizationPeriod > 0 {
		if c.lastPeerCount != peerCount {
			// Peer count just changed — start (or restart) the stabilization timer.
			c.lastPeerCount = peerCount
			c.peerCountStableAt = now.Add(c.stabilizationPeriod)
			c.mu.Unlock()
			if c.logger != nil {
				c.logger.Info("Peer count changed, waiting for stabilization before rebalancing",
					zap.String("platform", c.platform),
					zap.Int("peer_count", peerCount),
					zap.Duration("stabilization_period", c.stabilizationPeriod),
				)
			}
			return nil, nil
		}
		if now.Before(c.peerCountStableAt) {
			// Still within the stabilization window.
			c.mu.Unlock()
			return nil, nil
		}
	}

	// ceil(totalStreams / peerCount)
	maxPerPod := (totalStreams + peerCount - 1) / peerCount

	currentCount := len(c.leases)
	excess := currentCount - maxPerPod
	if excess <= 0 {
		c.mu.Unlock()
		return nil, nil
	}

	// Collect stream IDs and sort alphabetically for deterministic release
	streamIDs := make([]string, 0, currentCount)
	for id := range c.leases {
		streamIDs = append(streamIDs, id)
	}
	sort.Strings(streamIDs)

	// Release the last `excess` streams (keep the first maxPerPod alphabetically)
	toRelease := streamIDs[maxPerPod:]
	for _, id := range toRelease {
		if entry, ok := c.leases[id]; ok {
			close(entry.stopCh)
			delete(c.leases, id)
		}
	}
	c.setActiveGaugeLocked()
	c.mu.Unlock()

	// Release leadership on source-manager asynchronously
	for _, id := range toRelease {
		go func(streamID string) {
			if err := c.client.ReleaseLeadership(context.Background(), c.platform, streamID, c.callerID); err != nil {
				if c.logger != nil {
					c.logger.Warn("Failed to release leadership during rebalance",
						zap.String("platform", c.platform),
						zap.String("stream_id", streamID),
						zap.Error(err),
					)
				}
				c.observe("rebalance_release_error")
				return
			}
			c.observe("rebalance_released")
		}(id)
	}

	leadershipRebalanceReleased.WithLabelValues(sanitizeLabel(c.platform)).Add(float64(len(toRelease)))

	if c.logger != nil {
		c.logger.Info("Rebalanced leadership leases",
			zap.String("platform", c.platform),
			zap.Int("peer_count", peerCount),
			zap.Int("total_streams", totalStreams),
			zap.Int("max_per_pod", maxPerPod),
			zap.Int("had", currentCount),
			zap.Int("released", len(toRelease)),
			zap.Int("kept", maxPerPod),
		)
	}

	return toRelease, nil
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
