package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MigrationEvent represents a channel migration event
type MigrationEvent struct {
	MigrationID string    `json:"migration_id"`
	ChannelID   string    `json:"channel_id"`
	Platform    string    `json:"platform"`
	FromPod     string    `json:"from_pod"`
	ToPod       string    `json:"to_pod"`
	Timestamp   time.Time `json:"timestamp"`
	Reason      string    `json:"reason"` // "pod_failure", "scale_up", "rebalance"
}

// MigrationConfirmation represents a migration confirmation event
type MigrationConfirmation struct {
	MigrationID    string    `json:"migration_id"`
	ChannelID      string    `json:"channel_id"`
	Platform       string    `json:"platform"`
	PodID          string    `json:"pod_id"`
	Status         string    `json:"status"`          // "connected", "failed"
	SequenceNumber int64     `json:"sequence_number"` // For gap detection (MIGRATE-06)
	Timestamp      time.Time `json:"timestamp"`
	Error          string    `json:"error,omitempty"`
}

// MigrationPublisher publishes migration events to Redis Pub/Sub and Streams
type MigrationPublisher struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewMigrationPublisher creates a new migration publisher instance
func NewMigrationPublisher(redisClient *redis.Client, logger *zap.Logger) *MigrationPublisher {
	return &MigrationPublisher{
		redisClient: redisClient,
		logger:      logger,
	}
}

// PublishMigrationEvent publishes a migration event to both Redis Pub/Sub (for listener notification)
// and Redis Streams (for observability and gap detection) (MIGRATE-02, MIGRATE-06)
func (m *MigrationPublisher) PublishMigrationEvent(ctx context.Context, event *MigrationEvent) error {
	// Marshal event to JSON
	jsonPayload, err := json.Marshal(event)
	if err != nil {
		m.logger.Error("Failed to marshal migration event",
			zap.String("migration_id", event.MigrationID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to marshal migration event: %w", err)
	}

	// Step 1: Publish to Redis Pub/Sub channel for immediate listener notification
	// Per CONTEXT.md: "Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)"
	err = m.redisClient.Publish(ctx, "migration:events", jsonPayload).Err()
	if err != nil {
		m.logger.Error("Failed to publish migration event to Pub/Sub",
			zap.String("migration_id", event.MigrationID),
			zap.String("channel_id", event.ChannelID),
			zap.String("platform", event.Platform),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish to Pub/Sub: %w", err)
	}

	// Step 2: Append to Redis Streams for observability (MIGRATE-06)
	// Per MIGRATE-05: "System publishes migration events to Redis Streams for observability"
	err = m.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "migration:log",
		Values: map[string]interface{}{
			"migration_id": event.MigrationID,
			"channel_id":   event.ChannelID,
			"platform":     event.Platform,
			"from_pod":     event.FromPod,
			"to_pod":       event.ToPod,
			"timestamp":    event.Timestamp.Unix(),
			"reason":       event.Reason,
			"status":       "initiated",
		},
	}).Err()
	if err != nil {
		m.logger.Error("Failed to append migration event to Streams",
			zap.String("migration_id", event.MigrationID),
			zap.String("channel_id", event.ChannelID),
			zap.String("platform", event.Platform),
			zap.Error(err),
		)
		return fmt.Errorf("failed to append to Streams: %w", err)
	}

	m.logger.Info("Published migration event",
		zap.String("migration_id", event.MigrationID),
		zap.String("channel_id", event.ChannelID),
		zap.String("platform", event.Platform),
		zap.String("from_pod", event.FromPod),
		zap.String("to_pod", event.ToPod),
		zap.String("reason", event.Reason),
	)

	return nil
}

// PublishMigrationConfirmation appends a migration confirmation to Redis Streams
// for observability and gap detection (MIGRATE-06)
func (m *MigrationPublisher) PublishMigrationConfirmation(ctx context.Context, confirmation *MigrationConfirmation) error {
	// Append confirmation to Redis Streams
	err := m.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "migration:log",
		Values: map[string]interface{}{
			"migration_id":    confirmation.MigrationID,
			"channel_id":      confirmation.ChannelID,
			"platform":        confirmation.Platform,
			"pod_id":          confirmation.PodID,
			"status":          confirmation.Status, // "connected", "failed"
			"sequence_number": confirmation.SequenceNumber,
			"timestamp":       confirmation.Timestamp.Unix(),
			"error":           confirmation.Error,
		},
	}).Err()
	if err != nil {
		m.logger.Error("Failed to append migration confirmation to Streams",
			zap.String("migration_id", confirmation.MigrationID),
			zap.String("channel_id", confirmation.ChannelID),
			zap.String("status", confirmation.Status),
			zap.Error(err),
		)
		return fmt.Errorf("failed to append confirmation to Streams: %w", err)
	}

	m.logger.Info("Published migration confirmation",
		zap.String("migration_id", confirmation.MigrationID),
		zap.String("channel_id", confirmation.ChannelID),
		zap.String("platform", confirmation.Platform),
		zap.String("pod_id", confirmation.PodID),
		zap.String("status", confirmation.Status),
		zap.Int64("sequence_number", confirmation.SequenceNumber),
	)

	return nil
}
