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

package subscription

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MessageHandler is called when a message is received from Redis Pub/Sub
// channel parameter allows distinguishing between main and update channels
type MessageHandler func(overlayID string, channel string, message []byte)

// Subscriber manages Redis Pub/Sub subscriptions for overlays
type Subscriber struct {
	client        *redis.Client
	logger        *zap.Logger
	handler       MessageHandler
	metrics       *metrics.GatewayMetrics
	subscriptions map[string]*redis.PubSub // overlay_id -> subscription
	refCounts     map[string]int           // overlay_id -> number of connections
	// viewerOnly tracks whether each subscription was created by SubscribeViewerOnly
	// (single main channel) vs Subscribe (main + updates channels)
	viewerOnly map[string]bool
	mu         sync.RWMutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewSubscriber creates a new Redis Pub/Sub subscriber.
// The metrics parameter may be nil; if provided, pubsub_reconnect_total is incremented
// on each Pub/Sub reconnect attempt.
func NewSubscriber(client *redis.Client, logger *zap.Logger, handler MessageHandler, m *metrics.GatewayMetrics) *Subscriber {
	return &Subscriber{
		client:        client,
		logger:        logger,
		handler:       handler,
		metrics:       m,
		subscriptions: make(map[string]*redis.PubSub),
		refCounts:     make(map[string]int),
		viewerOnly:    make(map[string]bool),
		stopChan:      make(chan struct{}),
	}
}

// Subscribe subscribes to an overlay channel
// Increments reference count, only subscribes on first connection
func (s *Subscriber) Subscribe(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment reference count
	s.refCounts[overlayID]++

	// If already subscribed, just return
	if _, exists := s.subscriptions[overlayID]; exists {
		s.logger.Debug("Already subscribed to overlay",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]),
		)
		return nil
	}

	// Subscribe to both main and update channels
	// Main channel: regular chat messages and events
	// Update channel: TikTok like aggregate updates
	mainChannel := fmt.Sprintf("overlay:%s", overlayID)
	updateChannel := fmt.Sprintf("overlay:%s:updates", overlayID)
	pubsub := s.client.Subscribe(ctx, mainChannel, updateChannel)

	// Verify subscription
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to channels: %w", err)
	}

	s.subscriptions[overlayID] = pubsub
	s.viewerOnly[overlayID] = false

	s.logger.Info("Subscribed to overlay channels",
		zap.String("overlay_id", overlayID),
		zap.String("main_channel", mainChannel),
		zap.String("update_channel", updateChannel),
	)

	// Start listening for messages
	s.wg.Add(1)
	go s.listen(context.Background(), overlayID, pubsub)

	return nil
}

// Unsubscribe unsubscribes from an overlay channel
// Decrements reference count, only unsubscribes when count reaches 0
func (s *Subscriber) Unsubscribe(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// AG-03: Guard against reference count underflow
	if s.refCounts[overlayID] <= 0 {
		s.logger.Warn("Unsubscribe called with zero ref count",
			zap.String("overlay_id", overlayID),
		)
		delete(s.refCounts, overlayID)
		return nil
	}

	// Decrement reference count
	s.refCounts[overlayID]--

	// If still has connections, don't unsubscribe
	if s.refCounts[overlayID] > 0 {
		s.logger.Debug("Still has connections to overlay",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]),
		)
		return nil
	}

	// Remove from ref counts
	delete(s.refCounts, overlayID)

	// Get subscription
	pubsub, exists := s.subscriptions[overlayID]
	if !exists {
		return nil
	}

	// Unsubscribe
	if err := pubsub.Close(); err != nil {
		s.logger.Warn("Error closing subscription",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
	}

	delete(s.subscriptions, overlayID)
	delete(s.viewerOnly, overlayID)

	s.logger.Info("Unsubscribed from overlay channel",
		zap.String("overlay_id", overlayID),
	)

	return nil
}

// listen listens for messages on a subscription.
// When the channel is closed (ok == false), it triggers resubscribe() in a new
// goroutine and returns so the WaitGroup counter is decremented correctly.
func (s *Subscriber) listen(ctx context.Context, overlayID string, pubsub *redis.PubSub) {
	defer s.wg.Done()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case msg, ok := <-ch:
			if !ok {
				// AG-01: Channel closed — trigger re-subscription.
				// The new goroutine adds itself to wg before this one exits (AG-05).
				s.logger.Warn("Subscription channel closed — re-subscribing",
					zap.String("overlay_id", overlayID),
				)
				go s.resubscribe(overlayID)
				return // Current goroutine exits; wg.Done() fires via defer
			}

			// Call handler with message and channel name
			s.handler(overlayID, msg.Channel, []byte(msg.Payload))
		}
	}
}

// resubscribe creates a new Pub/Sub subscription for the given overlay after
// the previous one was closed (e.g., on Redis reconnect). It retries
// indefinitely with jittered exponential backoff until stopChan is closed.
// Increments pubsub_reconnect_total metric on each attempt (D-14).
//
// AG-05: The new listen goroutine is added to wg before resubscribe returns,
// ensuring Stop() waits for it.
func (s *Subscriber) resubscribe(overlayID string) {
	for attempt := 0; ; attempt++ {
		// AG-01: Do not re-subscribe if Stop has already been signalled
		select {
		case <-s.stopChan:
			s.logger.Info("resubscribe cancelled — stop signal received",
				zap.String("overlay_id", overlayID))
			return
		default:
		}

		s.mu.Lock()

		// D-14: Increment reconnect metric
		if s.metrics != nil {
			s.metrics.PubSubReconnectTotal.WithLabelValues("api-gateway", overlayID).Inc()
		}

		// Close the stale subscription on first attempt
		if attempt == 0 {
			if oldPubsub, exists := s.subscriptions[overlayID]; exists {
				oldPubsub.Close()
			}
		}

		// Determine whether this was a viewer-only subscription (main channel only)
		// or a full subscription (main + updates channels).
		isViewerOnly := s.viewerOnly[overlayID]

		var pubsub *redis.PubSub
		if isViewerOnly {
			channel := fmt.Sprintf("overlay:%s", overlayID)
			pubsub = s.client.Subscribe(context.Background(), channel)
		} else {
			mainChannel := fmt.Sprintf("overlay:%s", overlayID)
			updateChannel := fmt.Sprintf("overlay:%s:updates", overlayID)
			pubsub = s.client.Subscribe(context.Background(), mainChannel, updateChannel)
		}

		if _, err := pubsub.Receive(context.Background()); err != nil {
			pubsub.Close()
			s.mu.Unlock()

			s.logger.Warn("resubscribe attempt failed",
				zap.String("overlay_id", overlayID),
				zap.Int("attempt", attempt+1),
				zap.Error(err))

			sleep := listener.JitteredBackoff(attempt)
			select {
			case <-s.stopChan:
				return
			case <-time.After(sleep):
			}
			continue
		}

		s.subscriptions[overlayID] = pubsub
		s.logger.Info("Re-subscribed to overlay channel after close",
			zap.String("overlay_id", overlayID),
			zap.Int("attempt", attempt+1),
		)

		// Check stopChan BEFORE wg.Add to prevent WaitGroup leak (Pitfall 3)
		select {
		case <-s.stopChan:
			pubsub.Close()
			s.mu.Unlock()
			return
		default:
		}

		// AG-05: Track the new goroutine in WaitGroup before spawning it
		s.wg.Add(1)
		go s.listen(context.Background(), overlayID, pubsub)
		s.mu.Unlock()
		return
	}
}

// Stop stops all subscriptions and waits for all goroutines to finish.
func (s *Subscriber) Stop() {
	close(s.stopChan)

	s.mu.Lock()
	for overlayID, pubsub := range s.subscriptions {
		pubsub.Close()
		s.logger.Info("Closed subscription",
			zap.String("overlay_id", overlayID),
		)
	}
	s.subscriptions = make(map[string]*redis.PubSub)
	s.refCounts = make(map[string]int)
	s.viewerOnly = make(map[string]bool)
	s.mu.Unlock()

	s.wg.Wait()

	s.logger.Info("All subscriptions stopped")
}

// GetSubscriptionCount returns the number of active subscriptions
func (s *Subscriber) GetSubscriptionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.subscriptions)
}

// IsSubscribed checks if subscribed to an overlay
func (s *Subscriber) IsSubscribed(overlayID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.subscriptions[overlayID]
	return exists
}

// SubscribeViewerOnly subscribes to an overlay channel for viewer connections.
// Same as Subscribe() but does NOT publish connection events to avoid triggering
// YouTube polling. This is critical for viewer-only WebSocket connections at
// /ws/chat/{streamer}.
//
// AG-04 (interleaving): If a full Subscribe() already exists for the overlay,
// viewer connections share that existing Pub/Sub subscription (ref count is
// incremented). The viewer-only flag only governs which Redis channels to use
// when a brand-new subscription must be created for this overlay.
func (s *Subscriber) SubscribeViewerOnly(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment reference count
	s.refCounts[overlayID]++

	// If already subscribed, just return (shared subscription)
	if _, exists := s.subscriptions[overlayID]; exists {
		s.logger.Debug("Already subscribed to overlay (viewer connection)",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]),
		)
		return nil
	}

	// Subscribe to Redis channel (viewer-only: main channel only, no updates channel)
	channel := fmt.Sprintf("overlay:%s", overlayID)
	pubsub := s.client.Subscribe(ctx, channel)

	// Verify subscription
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to channel: %w", err)
	}

	s.subscriptions[overlayID] = pubsub
	s.viewerOnly[overlayID] = true

	s.logger.Info("Subscribed to overlay channel (viewer-only, no polling trigger)",
		zap.String("overlay_id", overlayID),
		zap.String("channel", channel),
	)

	// Start listening for messages
	s.wg.Add(1)
	go s.listen(context.Background(), overlayID, pubsub)

	return nil
}

// UnsubscribeViewerOnly unsubscribes from an overlay channel (viewer connection)
// Same as Unsubscribe() but does NOT publish disconnection events
func (s *Subscriber) UnsubscribeViewerOnly(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// AG-03: Guard against reference count underflow
	if s.refCounts[overlayID] <= 0 {
		s.logger.Warn("UnsubscribeViewerOnly called with zero ref count",
			zap.String("overlay_id", overlayID),
		)
		delete(s.refCounts, overlayID)
		return nil
	}

	// Decrement reference count
	s.refCounts[overlayID]--

	// If still has connections, don't unsubscribe
	if s.refCounts[overlayID] > 0 {
		s.logger.Debug("Still has connections to overlay (viewer)",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]),
		)
		return nil
	}

	// Remove from ref counts
	delete(s.refCounts, overlayID)

	// Get subscription
	pubsub, exists := s.subscriptions[overlayID]
	if !exists {
		return nil
	}

	// Unsubscribe
	if err := pubsub.Close(); err != nil {
		s.logger.Warn("Error closing subscription (viewer)",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
	}

	delete(s.subscriptions, overlayID)
	delete(s.viewerOnly, overlayID)

	s.logger.Info("Unsubscribed from overlay channel (viewer-only)",
		zap.String("overlay_id", overlayID),
	)

	return nil
}
