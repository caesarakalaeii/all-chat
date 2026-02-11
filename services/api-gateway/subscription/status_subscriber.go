package subscription

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/websocket"
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
	platformState sync.Map // platform:channelID -> models.PlatformStatusData
	stopChan      chan struct{}
}

// NewStatusSubscriber creates a new status subscriber
func NewStatusSubscriber(redisClient *redis.Client, wsManager *websocket.Manager, logger *zap.Logger) *StatusSubscriber {
	return &StatusSubscriber{
		redisClient: redisClient,
		wsManager:   wsManager,
		logger:      logger,
		stopChan:    make(chan struct{}),
	}
}

// Start begins subscribing to platform status updates
func (s *StatusSubscriber) Start(ctx context.Context) error {
	pubsub := s.redisClient.Subscribe(ctx, PlatformStatusChannel)

	s.logger.Info("Status subscriber started",
		zap.String("channel", PlatformStatusChannel))

	go func() {
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case msg := <-ch:
				s.handleStatusMessage(ctx, msg.Payload)
			case <-s.stopChan:
				s.logger.Info("Status subscriber stopped")
				return
			case <-ctx.Done():
				s.logger.Info("Status subscriber context done")
				return
			}
		}
	}()

	return nil
}

// Stop stops the status subscriber
func (s *StatusSubscriber) Stop() {
	close(s.stopChan)
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
