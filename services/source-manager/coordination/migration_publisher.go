package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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
	TraceParent string    `json:"traceparent,omitempty"` // W3C Trace Context propagation
	TraceState  string    `json:"tracestate,omitempty"`  // W3C Trace Context state
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
	metrics     *metrics.ShardMetrics
	logger      *zap.Logger
}

// NewMigrationPublisher creates a new migration publisher instance
func NewMigrationPublisher(redisClient *redis.Client, metrics *metrics.ShardMetrics, logger *zap.Logger) *MigrationPublisher {
	return &MigrationPublisher{
		redisClient: redisClient,
		metrics:     metrics,
		logger:      logger,
	}
}

// PublishMigrationEvent publishes a migration event to both Redis Pub/Sub (for listener notification)
// and Redis Streams (for observability and gap detection) (MIGRATE-02, MIGRATE-06)
func (m *MigrationPublisher) PublishMigrationEvent(ctx context.Context, event *MigrationEvent) error {
	// Record migration duration
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		m.metrics.MigrationDuration.Observe(duration)
	}()

	// Create parent span for the entire migration publish operation
	tracer := otel.Tracer("source-manager")
	ctx, span := tracer.Start(ctx, "publish-migration-event",
		trace.WithAttributes(
			attribute.String("migration_id", event.MigrationID),
			attribute.String("channel_id", event.ChannelID),
			attribute.String("platform", event.Platform),
			attribute.String("from_pod", event.FromPod),
			attribute.String("to_pod", event.ToPod),
			attribute.String("reason", event.Reason),
		),
	)
	defer span.End()

	// Inject trace context into carrier for propagation through Redis
	carrier := make(propagation.MapCarrier)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	// Add trace context to event for propagation
	event.TraceParent = carrier.Get("traceparent")
	event.TraceState = carrier.Get("tracestate")

	// Marshal event to JSON
	jsonPayload, err := json.Marshal(event)
	if err != nil {
		m.logger.Error("Failed to marshal migration event",
			zap.String("migration_id", event.MigrationID),
			zap.Error(err),
		)
		m.metrics.MigrationTotal.WithLabelValues("failure", event.Reason).Inc()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal migration event")
		return fmt.Errorf("failed to marshal migration event: %w", err)
	}

	// Step 1: Publish to Redis Pub/Sub channel for immediate listener notification
	// Per CONTEXT.md: "Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)"
	ctx, pubSpan := tracer.Start(ctx, "redis-publish-notification")
	err = m.redisClient.Publish(ctx, "migration:events", jsonPayload).Err()
	if err != nil {
		m.logger.Error("Failed to publish migration event to Pub/Sub",
			zap.String("migration_id", event.MigrationID),
			zap.String("channel_id", event.ChannelID),
			zap.String("platform", event.Platform),
			zap.Error(err),
		)
		m.metrics.MigrationTotal.WithLabelValues("failure", event.Reason).Inc()
		pubSpan.RecordError(err)
		pubSpan.SetStatus(codes.Error, err.Error())
		pubSpan.End()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish migration event")
		return fmt.Errorf("failed to publish to Pub/Sub: %w", err)
	}
	pubSpan.SetStatus(codes.Ok, "")
	pubSpan.End()

	// Step 2: Append to Redis Streams for observability (MIGRATE-06)
	// Per MIGRATE-05: "System publishes migration events to Redis Streams for observability"
	ctx, streamSpan := tracer.Start(ctx, "redis-stream-log")
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
			"traceparent":  carrier.Get("traceparent"), // W3C Trace Context
			"tracestate":   carrier.Get("tracestate"),
		},
	}).Err()
	if err != nil {
		m.logger.Error("Failed to append migration event to Streams",
			zap.String("migration_id", event.MigrationID),
			zap.String("channel_id", event.ChannelID),
			zap.String("platform", event.Platform),
			zap.Error(err),
		)
		m.metrics.MigrationTotal.WithLabelValues("failure", event.Reason).Inc()
		streamSpan.RecordError(err)
		streamSpan.SetStatus(codes.Error, err.Error())
		streamSpan.End()
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to log to streams")
		return fmt.Errorf("failed to append to Streams: %w", err)
	}
	streamSpan.SetStatus(codes.Ok, "")
	streamSpan.End()

	// Record successful migration
	m.metrics.MigrationTotal.WithLabelValues("success", event.Reason).Inc()

	m.logger.Info("Published migration event",
		zap.String("migration_id", event.MigrationID),
		zap.String("channel_id", event.ChannelID),
		zap.String("platform", event.Platform),
		zap.String("from_pod", event.FromPod),
		zap.String("to_pod", event.ToPod),
		zap.String("reason", event.Reason),
	)

	span.SetStatus(codes.Ok, "migration event published")
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
