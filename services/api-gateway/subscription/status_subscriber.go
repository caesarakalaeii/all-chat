package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Redis channel for platform status updates
	PlatformStatusChannel = "platform:status"
)

// sourceResolver looks up configured sources for an overlay.
type sourceResolver interface {
	GetOverlaySources(ctx context.Context, overlayID string) ([]OverlaySource, error)
}

// StatusSubscriber subscribes to platform status updates from Redis and broadcasts to WebSocket clients
type StatusSubscriber struct {
	redisClient    *redis.Client
	wsManager      *websocket.Manager
	logger         *zap.Logger
	metrics        *metrics.GatewayMetrics
	sourceResolver sourceResolver
	platformState  sync.Map // platform:channelID -> models.PlatformStatusData
	stopChan       chan struct{}
	wg             sync.WaitGroup
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

// SetSourceResolver sets the source resolver used for per-overlay status filtering.
func (s *StatusSubscriber) SetSourceResolver(r sourceResolver) {
	s.sourceResolver = r
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
				select {
				case <-s.stopChan:
					return
				default:
				}
				s.wg.Add(1)
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
// connection drop. It retries indefinitely with jittered exponential backoff
// until stopChan is closed. Increments pubsub_reconnect_total metric on each
// attempt per D-14.
func (s *StatusSubscriber) reconnect() {
	defer s.wg.Done()

	for attempt := 0; ; attempt++ {
		select {
		case <-s.stopChan:
			return
		default:
		}

		s.logger.Info("Status subscriber reconnecting",
			zap.Int("attempt", attempt+1))

		// Increment reconnect metric per D-14
		if s.metrics != nil {
			s.metrics.PubSubReconnectTotal.WithLabelValues("api-gateway", "status").Inc()
		}

		pubsub := s.redisClient.Subscribe(context.Background(), PlatformStatusChannel)
		if _, err := pubsub.Receive(context.Background()); err != nil {
			pubsub.Close()
			s.logger.Warn("Status subscriber reconnect failed",
				zap.Int("attempt", attempt+1), zap.Error(err))

			sleep := listener.JitteredBackoff(attempt)
			select {
			case <-s.stopChan:
				return
			case <-time.After(sleep):
			}
			continue
		}

		ch := pubsub.Channel()
		if ch == nil {
			pubsub.Close()
			s.logger.Warn("Status subscriber reconnect: nil channel",
				zap.Int("attempt", attempt+1))

			sleep := listener.JitteredBackoff(attempt)
			select {
			case <-s.stopChan:
				return
			case <-time.After(sleep):
			}
			continue
		}

		s.logger.Info("Status subscriber reconnected",
			zap.Int("attempt", attempt+1))

		select {
		case <-s.stopChan:
			pubsub.Close()
			return
		default:
		}

		s.wg.Add(1)
		go s.listen(pubsub)
		return
	}
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

	totalSent := s.broadcastStatusToRelevantOverlays(ctx, statusData, msgJSON)

	s.logger.Debug("Broadcasted platform status",
		zap.String("platform", statusData.Platform),
		zap.String("channel_id", statusData.ChannelID),
		zap.String("status", statusData.Status),
		zap.Int("clients_sent", totalSent))
}

// broadcastStatusToRelevantOverlays sends a status message only to overlays
// that have the matching platform+channel configured. Falls back to BroadcastToAll
// if no source resolver is set.
func (s *StatusSubscriber) broadcastStatusToRelevantOverlays(ctx context.Context, statusData models.PlatformStatusData, msgJSON []byte) int {
	if s.sourceResolver == nil {
		return s.wsManager.BroadcastToAll(msgJSON)
	}

	overlayIDs := s.wsManager.GetConnectedOverlayIDs()
	totalSent := 0

	for _, overlayID := range overlayIDs {
		sources, err := s.sourceResolver.GetOverlaySources(ctx, overlayID)
		if err != nil {
			s.logger.Warn("Failed to get overlay sources, sending status anyway",
				zap.String("overlay_id", overlayID),
				zap.Error(err))
			totalSent += s.wsManager.BroadcastToOverlay(overlayID, msgJSON)
			continue
		}

		for _, src := range sources {
			if src.Platform == statusData.Platform {
				totalSent += s.wsManager.BroadcastToOverlay(overlayID, msgJSON)
				break
			}
		}
	}

	return totalSent
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
