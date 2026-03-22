package coordination

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MigrationSubscriber subscribes to Redis Pub/Sub for migration events
type MigrationSubscriber struct {
	redisClient *redis.Client
	logger      *zap.Logger
	handler     func(*MigrationEvent) error
	platform    string // when non-empty, only events with matching platform are processed
}

// NewMigrationSubscriber creates a new migration event subscriber
func NewMigrationSubscriber(redisClient *redis.Client, handler func(*MigrationEvent) error, logger *zap.Logger) *MigrationSubscriber {
	return &MigrationSubscriber{
		redisClient: redisClient,
		handler:     handler,
		logger:      logger,
	}
}

// WithPlatform restricts this subscriber to only process events for the given platform.
// Events with a different (non-empty) platform field are silently dropped at DEBUG level.
func (s *MigrationSubscriber) WithPlatform(platform string) *MigrationSubscriber {
	s.platform = platform
	return s
}

// Subscribe subscribes to the migration:events Redis Pub/Sub channel
// Implements MIGRATE-01 (overlap migration pattern notification)
// Per CONTEXT.md user decision: "Hybrid Redis Pub/Sub approach - Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)"
func (s *MigrationSubscriber) Subscribe(ctx context.Context) error {
	const channel = "migration:events"

	s.logger.Info("Subscribing to migration events channel",
		zap.String("channel", channel),
	)

	// Subscribe to Redis Pub/Sub channel
	pubsub := s.redisClient.Subscribe(ctx, channel)

	// Wait for subscription confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		s.logger.Error("Failed to confirm subscription",
			zap.String("channel", channel),
			zap.Error(err),
		)
		return fmt.Errorf("failed to subscribe to %s: %w", channel, err)
	}

	s.logger.Info("Successfully subscribed to migration events channel",
		zap.String("channel", channel),
	)

	// Start consuming messages in a goroutine
	go s.consumeMessages(ctx, pubsub)

	return nil
}

// consumeMessages consumes messages from the Pub/Sub subscription
func (s *MigrationSubscriber) consumeMessages(ctx context.Context, pubsub *redis.PubSub) {
	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Migration subscriber context canceled, stopping consumption")
			pubsub.Close()
			return

		case msg, ok := <-ch:
			if !ok {
				s.logger.Warn("Migration events channel closed")
				return
			}

			// Parse migration event
			var event MigrationEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				s.logger.Warn("Failed to unmarshal migration event, skipping",
					zap.String("payload", msg.Payload),
					zap.Error(err),
				)
				continue
			}

			// Drop events for other platforms when platform filter is set.
			if s.platform != "" && event.Platform != s.platform {
				s.logger.Debug("Ignoring migration event for other platform",
					zap.String("event_platform", event.Platform),
					zap.String("listener_platform", s.platform),
					zap.String("migration_id", event.MigrationID),
				)
				continue
			}

			s.logger.Info("Received migration event",
				zap.String("migration_id", event.MigrationID),
				zap.String("channel_id", event.ChannelID),
				zap.String("platform", event.Platform),
				zap.String("from_pod", event.FromPod),
				zap.String("to_pod", event.ToPod),
				zap.String("reason", event.Reason),
			)

			// Call handler with recovered panic protection
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Error("Migration event handler panicked",
							zap.String("migration_id", event.MigrationID),
							zap.Any("panic", r),
						)
					}
				}()

				if err := s.handler(&event); err != nil {
					s.logger.Error("Migration event handler returned error",
						zap.String("migration_id", event.MigrationID),
						zap.Error(err),
					)
					// Do not return — continue processing subsequent events
				}
			}()
		}
	}
}
