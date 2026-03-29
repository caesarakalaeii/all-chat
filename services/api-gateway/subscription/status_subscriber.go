package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Redis channel for platform status updates
	PlatformStatusChannel = "platform:status"
)

// StatusSubscriber subscribes to platform status updates from Redis and broadcasts to WebSocket clients
type StatusSubscriber struct {
	redisClient   *redis.Client
	wsManager     *websocket.Manager
	logger        *zap.Logger
	metrics       *metrics.GatewayMetrics
	platformState sync.Map // platform:channelID -> models.PlatformStatusData
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewStatusSubscriber creates a new status subscriber
func NewStatusSubscriber(redisClient *redis.Client, wsManager *websocket.Manager, logger *zap.Logger, m *metrics.GatewayMetrics) *StatusSubscriber {
	return &StatusSubscriber{
		redisClient: redisClient,
		wsManager:   wsManager,
		logger:      logger,
		metrics:     m,
		stopChan:    make(chan struct{}),
	}
}

// Start begins subscribing to platform status updates.
// Returns an error if the initial subscription to Redis fails.
func (s *StatusSubscriber) Start(ctx context.Context) error {
	pubsub := s.redisClient.Subscribe(ctx, PlatformStatusChannel)

	// SS-03: check Subscribe error before proceeding
	if _, err := pubsub.Receive(ctx); err != nil {
		pubsub.Close()
		return fmt.Errorf("failed to subscribe to %s: %w", PlatformStatusChannel, err)
	}

	// SS-01: guard against nil channel
	ch := pubsub.Channel()
	if ch == nil {
		pubsub.Close()
		return fmt.Errorf("pub/sub channel is nil for %s", PlatformStatusChannel)
	}

	s.logger.Info("Status subscriber started",
		zap.String("channel", PlatformStatusChannel))

	s.wg.Add(1)
	go s.listen(pubsub)

	return nil
}

// listen is the internal goroutine that reads messages from the pubsub channel.
// SS-02: re-subscribes when the channel closes instead of blocking forever.
func (s *StatusSubscriber) listen(pubsub *redis.PubSub) {
	defer s.wg.Done()
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				s.logger.Warn("Status subscriber channel closed — re-subscribing")
				go s.reconnect()
				return
			}
			s.handleStatusMessage(context.Background(), msg.Payload)
		case <-s.stopChan:
			s.logger.Info("Status subscriber stopped")
			return
		}
	}
}

// reconnect attempts to re-subscribe to the platform status channel after a
// connection drop. It retries up to 3 times with exponential backoff and
// increments the pubsub_reconnect_total metric on each attempt per D-14.
func (s *StatusSubscriber) reconnect() {
	// If Stop has already been called, do not attempt to reconnect.
	select {
	case <-s.stopChan:
		return
	default:
	}

	for attempt := 1; attempt <= 3; attempt++ {
		s.logger.Info("Status subscriber reconnecting",
			zap.Int("attempt", attempt))

		// Increment reconnect metric per D-14
		if s.metrics != nil {
			s.metrics.PubSubReconnectTotal.WithLabelValues("api-gateway", "status").Inc()
		}

		pubsub := s.redisClient.Subscribe(context.Background(), PlatformStatusChannel)
		if _, err := pubsub.Receive(context.Background()); err != nil {
			pubsub.Close()
			s.logger.Warn("Status subscriber reconnect failed",
				zap.Int("attempt", attempt), zap.Error(err))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		ch := pubsub.Channel()
		if ch == nil {
			pubsub.Close()
			s.logger.Warn("Status subscriber reconnect: nil channel",
				zap.Int("attempt", attempt))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		s.logger.Info("Status subscriber reconnected")
		s.wg.Add(1)
		go s.listen(pubsub)
		return
	}

	s.logger.Error("Status subscriber failed to reconnect after 3 attempts")
}

// Stop stops the status subscriber and waits for the goroutine to exit.
func (s *StatusSubscriber) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	s.logger.Info("Status subscriber fully stopped")
}

// handleStatusMessage processes incoming status messages
func (s *StatusSubscriber) handleStatusMessage(ctx context.Context, payload string) {
	var statusData models.PlatformStatusData
	if err := json.Unmarshal([]byte(payload), &statusData); err != nil {
		s.logger.Error("Failed to unmarshal status message",
			zap.String("payload", payload),
			zap.Error(err))
		return
	}

	// Update in-memory state
	key := statusData.Platform + ":" + statusData.ChannelID
	s.platformState.Store(key, statusData)

	s.logger.Debug("Received platform status update",
		zap.String("platform", statusData.Platform),
		zap.String("channel_id", statusData.ChannelID),
		zap.String("status", statusData.Status))

	// Broadcast to all WebSocket clients
	if s.wsManager == nil {
		return
	}
	wsMsg := models.NewPlatformStatus(statusData)
	msgJSON, err := wsMsg.ToJSON()
	if err != nil {
		s.logger.Error("Failed to marshal WebSocket message",
			zap.String("platform", statusData.Platform),
			zap.String("channel_id", statusData.ChannelID),
			zap.Error(err))
		return
	}

	s.wsManager.BroadcastToAll(msgJSON)

	s.logger.Debug("Broadcasted platform status to all clients",
		zap.String("platform", statusData.Platform),
		zap.String("channel_id", statusData.ChannelID),
		zap.String("status", statusData.Status))
}

// GetPlatformStatus retrieves the current status for a platform and channel
func (s *StatusSubscriber) GetPlatformStatus(platform, channelID string) (*models.PlatformStatusData, bool) {
	key := platform + ":" + channelID
	if value, ok := s.platformState.Load(key); ok {
		if status, ok := value.(models.PlatformStatusData); ok {
			return &status, true
		}
	}
	return nil, false
}
