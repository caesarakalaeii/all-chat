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
	"os"
	"strconv"
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

// Subscriber manages Redis Pub/Sub subscriptions for overlays.
//
// Subscription lifetime is decoupled from connection lifetime: when the last
// connection for an overlay drops, the Pub/Sub subscription is held open for
// lingerDuration (default 5 min) so messages arriving during the disconnect
// gap continue to flow into the chat replay buffer for replay on reconnect.
// If a new connection arrives within the window, the linger timer is cancelled
// and the existing subscription is reused — no break in capture.
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
	// lingerTimers holds pending deferred-close timers. A non-nil entry means
	// refCount is 0 but the subscription is intentionally kept alive to capture
	// messages for the chat replay buffer during a connection gap.
	lingerTimers   map[string]*time.Timer
	lingerDuration time.Duration
	mu             sync.RWMutex
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// defaultLingerDuration is how long we keep a Pub/Sub subscription alive after
// the last WebSocket connection drops, so messages arriving during a brief
// disconnect are captured by the replay buffer and replayed on reconnect.
// Must match the chat replay buffer's TTL.
const defaultLingerDuration = 5 * time.Minute

// NewSubscriber creates a new Redis Pub/Sub subscriber.
// The metrics parameter may be nil; if provided, pubsub_reconnect_total is incremented
// on each Pub/Sub reconnect attempt.
//
// PUBSUB_LINGER_SECONDS env var overrides the default 5-minute linger window.
// Set to 0 to disable lingering (revert to immediate unsubscribe).
func NewSubscriber(client *redis.Client, logger *zap.Logger, handler MessageHandler, m *metrics.GatewayMetrics) *Subscriber {
	linger := defaultLingerDuration
	if v := os.Getenv("PUBSUB_LINGER_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			linger = time.Duration(secs) * time.Second
		}
	}
	logger.Info("Subscriber initialised", zap.Duration("linger", linger))
	return &Subscriber{
		client:         client,
		logger:         logger,
		handler:        handler,
		metrics:        m,
		subscriptions:  make(map[string]*redis.PubSub),
		refCounts:      make(map[string]int),
		viewerOnly:     make(map[string]bool),
		lingerTimers:   make(map[string]*time.Timer),
		lingerDuration: linger,
		stopChan:       make(chan struct{}),
	}
}

// cancelLingerLocked cancels any pending linger timer for overlayID.
// Caller MUST hold s.mu.
func (s *Subscriber) cancelLingerLocked(overlayID string) {
	if timer, exists := s.lingerTimers[overlayID]; exists {
		timer.Stop()
		delete(s.lingerTimers, overlayID)
		s.logger.Debug("Cancelled pending pubsub linger close",
			zap.String("overlay_id", overlayID))
	}
}

// scheduleLingerCloseLocked schedules a deferred unsubscribe. Caller MUST hold s.mu.
// If lingerDuration is 0, closes immediately (legacy behaviour).
func (s *Subscriber) scheduleLingerCloseLocked(overlayID string) {
	if s.lingerDuration <= 0 {
		// Lingering disabled — close right now.
		s.closeSubscriptionLocked(overlayID)
		return
	}
	// Cancel any older timer so we don't double-fire.
	if old, exists := s.lingerTimers[overlayID]; exists {
		old.Stop()
	}
	timer := time.AfterFunc(s.lingerDuration, func() {
		s.fireLingerClose(overlayID)
	})
	s.lingerTimers[overlayID] = timer
	s.logger.Info("Last connection dropped — keeping pubsub open during linger window",
		zap.String("overlay_id", overlayID),
		zap.Duration("linger", s.lingerDuration))
}

// fireLingerClose runs in a timer goroutine. Re-checks refCount under lock —
// a connection that arrived after the timer fired but before we acquired the
// lock will have already cancelled the timer (so we wouldn't be here), so
// when we see refCount == 0 here we can safely close.
func (s *Subscriber) fireLingerClose(overlayID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Has the timer been cancelled? Map entry will be gone if so.
	if _, exists := s.lingerTimers[overlayID]; !exists {
		return
	}
	delete(s.lingerTimers, overlayID)

	// Re-check refCount: a new connection could have arrived while we were
	// waiting for the lock and bumped the count back above 0. In that case
	// the new Subscribe would have cancelled the timer (clearing the map
	// entry above), so reaching here means the count is genuinely 0.
	if s.refCounts[overlayID] > 0 {
		s.logger.Debug("Skipping linger close: connection arrived",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]))
		return
	}

	s.logger.Info("Linger window expired — closing pubsub subscription",
		zap.String("overlay_id", overlayID))
	s.closeSubscriptionLocked(overlayID)
}

// closeSubscriptionLocked closes and removes the subscription for overlayID.
// Caller MUST hold s.mu.
func (s *Subscriber) closeSubscriptionLocked(overlayID string) {
	pubsub, exists := s.subscriptions[overlayID]
	if !exists {
		return
	}
	if err := pubsub.Close(); err != nil {
		s.logger.Warn("Error closing subscription",
			zap.String("overlay_id", overlayID),
			zap.Error(err))
	}
	delete(s.subscriptions, overlayID)
	delete(s.viewerOnly, overlayID)
}

// Subscribe subscribes to an overlay channel
// Increments reference count, only subscribes on first connection
func (s *Subscriber) Subscribe(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment reference count
	s.refCounts[overlayID]++

	// If a linger timer is pending for this overlay, the subscription is still
	// alive — cancel the timer and reuse it. This is the reconnect-within-window
	// fast path: no Pub/Sub re-subscribe, no message loss between drops.
	s.cancelLingerLocked(overlayID)

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
	// Engagement snapshots (issue #523): aggregate poll/prediction state.
	pollChannel := fmt.Sprintf("overlay:%s:poll", overlayID)
	predictionChannel := fmt.Sprintf("overlay:%s:prediction", overlayID)
	pubsub := s.client.Subscribe(ctx, mainChannel, updateChannel, pollChannel, predictionChannel)

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

// Unsubscribe unsubscribes from an overlay channel.
// Decrements reference count; when count hits zero, schedules a deferred close
// (lingerDuration) so the Pub/Sub subscription stays alive long enough to
// capture messages for the replay buffer during a brief disconnect.
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

	// No subscription? Nothing to do.
	if _, exists := s.subscriptions[overlayID]; !exists {
		return nil
	}

	// Schedule deferred close — keep capturing messages for the buffer during
	// the linger window. If a connection arrives within the window, the timer
	// is cancelled and the subscription is reused.
	s.scheduleLingerCloseLocked(overlayID)

	return nil
}

// listen listens for messages on a subscription.
// When the channel is closed (ok == false), it triggers resubscribe() in a new
// goroutine and returns so the WaitGroup counter is decremented correctly.
// If the subscription was closed intentionally (linger window expired or Stop
// called), we detect that by checking whether the overlay is still in the
// subscriptions map and exit without re-subscribing.
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
				// Was the close intentional (linger expired / Stop called)?
				// If our pubsub object is no longer in the map, yes — give up.
				s.mu.RLock()
				current, stillTracked := s.subscriptions[overlayID]
				s.mu.RUnlock()
				if !stillTracked || current != pubsub {
					s.logger.Debug("Listen goroutine exiting after intentional close",
						zap.String("overlay_id", overlayID))
					return
				}

				// AG-01: Unexpected channel close — trigger re-subscription.
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
		// Engagement snapshots (issue #523) reach both OBS overlays and viewers.
		pollChannel := fmt.Sprintf("overlay:%s:poll", overlayID)
		predictionChannel := fmt.Sprintf("overlay:%s:prediction", overlayID)
		if isViewerOnly {
			channel := fmt.Sprintf("overlay:%s", overlayID)
			pubsub = s.client.Subscribe(context.Background(), channel, pollChannel, predictionChannel)
		} else {
			mainChannel := fmt.Sprintf("overlay:%s", overlayID)
			updateChannel := fmt.Sprintf("overlay:%s:updates", overlayID)
			pubsub = s.client.Subscribe(context.Background(), mainChannel, updateChannel, pollChannel, predictionChannel)
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
	for overlayID, timer := range s.lingerTimers {
		timer.Stop()
		delete(s.lingerTimers, overlayID)
	}
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

	// Cancel any pending linger close — we're reusing the subscription.
	s.cancelLingerLocked(overlayID)

	// If already subscribed, just return (shared subscription)
	if _, exists := s.subscriptions[overlayID]; exists {
		s.logger.Debug("Already subscribed to overlay (viewer connection)",
			zap.String("overlay_id", overlayID),
			zap.Int("ref_count", s.refCounts[overlayID]),
		)
		return nil
	}

	// Subscribe to Redis channel (viewer-only: main channel, plus engagement
	// snapshots so viewers see live poll/prediction state; no TikTok updates channel).
	channel := fmt.Sprintf("overlay:%s", overlayID)
	pollChannel := fmt.Sprintf("overlay:%s:poll", overlayID)
	predictionChannel := fmt.Sprintf("overlay:%s:prediction", overlayID)
	pubsub := s.client.Subscribe(ctx, channel, pollChannel, predictionChannel)

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
// Same as Unsubscribe() but does NOT publish disconnection events.
// Honours the same linger window so messages keep flowing into the replay
// buffer while the viewer reconnects.
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

	// No subscription? Nothing to do.
	if _, exists := s.subscriptions[overlayID]; !exists {
		return nil
	}

	// Defer the actual close so messages keep landing in the replay buffer.
	s.scheduleLingerCloseLocked(overlayID)

	return nil
}
