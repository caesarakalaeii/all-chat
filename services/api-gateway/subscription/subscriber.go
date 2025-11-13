package subscription

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MessageHandler is called when a message is received from Redis Pub/Sub
type MessageHandler func(overlayID string, message []byte)

// Subscriber manages Redis Pub/Sub subscriptions for overlays
type Subscriber struct {
	client        *redis.Client
	logger        *zap.Logger
	handler       MessageHandler
	subscriptions map[string]*redis.PubSub // overlay_id -> subscription
	refCounts     map[string]int           // overlay_id -> number of connections
	mu            sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewSubscriber creates a new Redis Pub/Sub subscriber
func NewSubscriber(client *redis.Client, logger *zap.Logger, handler MessageHandler) *Subscriber {
	return &Subscriber{
		client:        client,
		logger:        logger,
		handler:       handler,
		subscriptions: make(map[string]*redis.PubSub),
		refCounts:     make(map[string]int),
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

	// Subscribe to Redis channel
	channel := fmt.Sprintf("overlay:%s", overlayID)
	pubsub := s.client.Subscribe(ctx, channel)

	// Verify subscription
	if _, err := pubsub.Receive(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to channel: %w", err)
	}

	s.subscriptions[overlayID] = pubsub

	s.logger.Info("Subscribed to overlay channel",
		zap.String("overlay_id", overlayID),
		zap.String("channel", channel),
	)

	// Start listening for messages
	s.wg.Add(1)
	go s.listen(ctx, overlayID, pubsub)

	return nil
}

// Unsubscribe unsubscribes from an overlay channel
// Decrements reference count, only unsubscribes when count reaches 0
func (s *Subscriber) Unsubscribe(ctx context.Context, overlayID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	s.logger.Info("Unsubscribed from overlay channel",
		zap.String("overlay_id", overlayID),
	)

	return nil
}

// listen listens for messages on a subscription
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
				s.logger.Warn("Subscription channel closed",
					zap.String("overlay_id", overlayID),
				)
				return
			}

			// Call handler with message
			s.handler(overlayID, []byte(msg.Payload))
		}
	}
}

// Stop stops all subscriptions
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
