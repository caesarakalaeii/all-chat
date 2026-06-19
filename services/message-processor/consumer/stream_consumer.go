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

package consumer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key to consume from
	StreamKey = "chat:raw"

	// ConsumerGroup is the consumer group name
	ConsumerGroup = "message-processor"

	// ReadCount is how many messages to read in one batch
	ReadCount = 100

	// ReadBlockTime is how long to block waiting for messages
	ReadBlockTime = 5 * time.Second
)

// MessageHandler is called for each consumed message
type MessageHandler func(ctx context.Context, msg *models.RawChatMessage) error

// NativeDeduplicator drops messages whose platform-native id has already been processed. Both the
// IRC and EventSub Twitch chat paths stamp the same native id into Tags["id"], so this makes the
// brief IRC↔EventSub handoff overlap (and Twitch webhook retries) idempotent (ADR-0015).
type NativeDeduplicator interface {
	IsDuplicateNativeID(ctx context.Context, platform, nativeID string) (bool, error)
}

// StreamConsumer consumes messages from Redis Streams
type StreamConsumer struct {
	client         *redis.Client
	logger         *zap.Logger
	metrics        *metrics.ProcessorMetrics
	handler        MessageHandler
	stopCh         chan struct{}
	msgIDRegistry  registry.MessageIDRegistry
	deletionBuffer registry.DeletionBuffer
	nativeDedup    NativeDeduplicator // nil disables native-id dedup
	consumerName   string
}

// NewStreamConsumer creates a new Redis Streams consumer.
// consumerName should be set to os.Hostname() by the caller for unique per-pod identification.
func NewStreamConsumer(client *redis.Client, logger *zap.Logger, m *metrics.ProcessorMetrics, handler MessageHandler, msgIDRegistry registry.MessageIDRegistry, deletionBuffer registry.DeletionBuffer, consumerName string) *StreamConsumer {
	return &StreamConsumer{
		client:         client,
		logger:         logger,
		metrics:        m,
		handler:        handler,
		stopCh:         make(chan struct{}),
		msgIDRegistry:  msgIDRegistry,
		deletionBuffer: deletionBuffer,
		consumerName:   consumerName,
	}
}

// SetNativeDeduplicator injects the native-id deduplicator used to collapse the IRC↔EventSub
// handoff overlap. Safe to leave unset (dedup disabled).
func (c *StreamConsumer) SetNativeDeduplicator(d NativeDeduplicator) {
	c.nativeDedup = d
}

// Start begins consuming messages from the stream
func (c *StreamConsumer) Start(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	if err := c.createConsumerGroup(ctx); err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// MP-02: Drain PEL messages idle > 5 minutes before entering normal read loop.
	// This reclaims orphaned entries from crashed/restarted replicas.
	c.drainPEL(ctx)
	c.logger.Info("PEL drain complete, starting consume loop")

	// DQ-02: Launch hourly DLQ trim goroutine
	go c.trimDLQ(ctx)

	c.logger.Info("Stream consumer started",
		zap.String("stream", StreamKey),
		zap.String("group", ConsumerGroup),
		zap.String("consumer", c.consumerName),
	)

	// Start consuming
	go c.consumeLoop(ctx)

	return nil
}

// Stop gracefully stops the consumer
func (c *StreamConsumer) Stop() {
	close(c.stopCh)
	c.logger.Info("Stream consumer stopped")
}

// createConsumerGroup creates the consumer group if it doesn't exist
func (c *StreamConsumer) createConsumerGroup(ctx context.Context) error {
	// Try to create the consumer group.
	// Offset "0" (not "$") so pre-existing messages in the stream are not silently
	// skipped on the first start — F-07 from phase 10 (message pipeline resilience).
	// "$" would mean "only new messages", causing data loss on cold start when the
	// listeners have already buffered chat into the stream.
	err := c.client.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "0").Err()
	if err != nil {
		// BUSYGROUP error means group already exists, which is fine
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
		c.logger.Debug("Consumer group already exists",
			zap.String("group", ConsumerGroup),
		)
	} else {
		c.logger.Info("Created consumer group",
			zap.String("group", ConsumerGroup),
		)
	}

	return nil
}

// consumeLoop continuously reads and processes messages
func (c *StreamConsumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			if err := c.readAndProcess(ctx); err != nil {
				// MP-08: Context cancellation is not an error — exit cleanly
				if ctx.Err() != nil {
					return
				}
				c.logger.Error("Error reading messages", zap.Error(err))
				time.Sleep(1 * time.Second) // Back off on error
			}
		}
	}
}

// readAndProcess reads messages and processes them
func (c *StreamConsumer) readAndProcess(ctx context.Context) error {
	// Read messages from the stream
	// ">" means only new messages not yet delivered to this consumer
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: c.consumerName,
		Streams:  []string{StreamKey, ">"},
		Count:    ReadCount,
		Block:    ReadBlockTime,
	}).Result()

	if err != nil {
		// redis.Nil means no messages available (timeout)
		if err == redis.Nil {
			return nil
		}
		c.metrics.RecordStreamError("message-processor", "read_error")
		return fmt.Errorf("failed to read from stream: %w", err)
	}

	// Process each stream (we only have one)
	for _, stream := range streams {
		for _, message := range stream.Messages {
			// Calculate and record stream lag from Redis stream entry timestamp
			if lag, ok := streamEntryLag(message.ID); ok {
				c.metrics.SetStreamLag("message-processor", StreamKey, ConsumerGroup, lag)
			}

			// MP-03: processAndAck handles retry, DLQ routing, and ACK ordering.
			// Messages are ACKed regardless of processing outcome (after DLQ write on failure).
			if err := c.processAndAck(ctx, message); err != nil {
				c.logger.Error("Failed to process message (sent to DLQ)",
					zap.String("stream_id", message.ID),
					zap.Error(err),
				)
				// Continue processing other messages
			}
		}
	}

	return nil
}

// processMessage processes a single message
func (c *StreamConsumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	// Extract the "data" field which contains the full JSON
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		c.metrics.RecordStreamError("message-processor", "invalid_data")
		return fmt.Errorf("missing or invalid 'data' field in message")
	}

	// Parse the raw message
	rawMsg, err := models.ParseRawMessage([]byte(dataStr))
	if err != nil {
		c.metrics.RecordStreamError("message-processor", "parse_error")
		return fmt.Errorf("failed to parse raw message: %w", err)
	}

	// Record message consumed
	c.metrics.RecordMessageConsumed("message-processor", rawMsg.Platform, ConsumerGroup)

	c.logger.Debug("Processing message",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("platform", rawMsg.Platform),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("user", rawMsg.Username),
		zap.String("event_type", rawMsg.EventType),
	)

	// Handle deletion events specially
	if rawMsg.EventType == "message_deletion" {
		return c.processDeletionEvent(ctx, rawMsg)
	}

	// For regular messages, check if deletion was buffered
	if rawMsg.EventType == "" || rawMsg.EventType == "chat_message" {
		// Extract platform message ID from tags. Twitch sets Tags["id"] (IRC tag),
		// YouTube sets Tags["youtube_message_id"] (InnerTube renderer ID); without
		// the YouTube fallback, buffered deletions for YouTube messages would
		// never drain (#284).
		platformMsgID := rawMsg.Tags["id"]
		if platformMsgID == "" {
			platformMsgID = rawMsg.Tags["youtube_message_id"]
		}
		if platformMsgID != "" {
			// NOTE: registry.Add() happens in twitch-listener per user decision (CONTEXT.md)
			// We only CHECK the buffer here for pending deletions

			// Check if deletion was buffered for this message
			deletion, err := c.deletionBuffer.Get(ctx, rawMsg.Platform, rawMsg.ChannelID, platformMsgID)
			if err != nil {
				c.logger.Error("Failed to check deletion buffer", zap.Error(err))
			} else if deletion != nil {
				// Apply buffered deletion
				c.logger.Info("Applying buffered deletion",
					zap.String("platform_msg_id", platformMsgID),
					zap.String("deletion_type", deletion.EventData["deletion_type"].(string)),
				)

				// Process deletion event immediately
				if err := c.processDeletionEvent(ctx, deletion); err != nil {
					c.logger.Error("Failed to process buffered deletion", zap.Error(err))
				}

				// Remove from buffer
				if err := c.deletionBuffer.Remove(ctx, rawMsg.Platform, rawMsg.ChannelID, platformMsgID); err != nil {
					c.logger.Error("Failed to remove from buffer", zap.Error(err))
				}

				c.metrics.BufferedDeletionsApplied.Inc()
			}
		}
	}

	// Native-id dedup (ADR-0015): the IRC and EventSub Twitch paths both stamp the identical
	// native Twitch message id into Tags["id"], so a channel handed off between them (or a Twitch
	// webhook retry) can present the same message twice. Drop the second copy before enrichment so
	// viewers never see doubled chat. Scoped to Twitch regular chat, where Tags["id"] is the native,
	// globally-unique message id; deletion events and other platforms are unaffected.
	if c.nativeDedup != nil && rawMsg.Platform == "twitch" &&
		(rawMsg.EventType == "" || rawMsg.EventType == "chat_message") {
		if nativeID := rawMsg.Tags["id"]; nativeID != "" {
			if dup, derr := c.nativeDedup.IsDuplicateNativeID(ctx, rawMsg.Platform, nativeID); derr == nil && dup {
				c.logger.Debug("Dropping duplicate Twitch message by native id",
					zap.String("native_id", nativeID),
					zap.String("channel", rawMsg.ChannelID),
				)
				if c.metrics != nil {
					c.metrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "native_dedup", "duplicate")
				}
				return nil // ACK + drop the duplicate
			}
		}
	}

	// Call the handler (which does normalization + enrichment + publishing)
	start := time.Now()
	if err := c.handler(ctx, rawMsg); err != nil {
		c.metrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "handler", "failed")
		return fmt.Errorf("handler failed: %w", err)
	}

	// Record successful processing and duration
	c.metrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "handler", "success")
	c.metrics.ProcessingDuration.WithLabelValues("message-processor", rawMsg.Platform).Observe(time.Since(start).Seconds())

	return nil
}

// processAndAck processes a single stream message and ACKs it after completion.
// On handler error, retries via retryOp. On exhaustion, routes to DLQ then ACKs
// (message must leave PEL regardless of processing outcome).
func (c *StreamConsumer) processAndAck(ctx context.Context, msg redis.XMessage) error {
	var processErr error
	err := retryOp(ctx, func() error {
		processErr = c.processMessage(ctx, msg)
		return processErr
	})

	if err != nil {
		// All retries exhausted — route to DLQ then ACK
		c.writeToDLQ(ctx, msg.ID, "message-processor", err.Error(), 3, msg.Values)
	}

	// Always ACK to remove from PEL (even on failure — message is in DLQ)
	if ackErr := c.client.XAck(ctx, StreamKey, ConsumerGroup, msg.ID).Err(); ackErr != nil {
		c.logger.Warn("Failed to ACK message after DLQ routing",
			zap.String("stream_id", msg.ID),
			zap.Error(ackErr),
		)
	}

	return processErr
}

// GetPendingCount returns the number of pending messages for this consumer
func (c *StreamConsumer) GetPendingCount(ctx context.Context) (int64, error) {
	pending, err := c.client.XPending(ctx, StreamKey, ConsumerGroup).Result()
	if err != nil {
		return 0, err
	}
	return pending.Count, nil
}

// processDeletionEvent handles deletion events from Redis Streams
func (c *StreamConsumer) processDeletionEvent(ctx context.Context, raw *models.RawChatMessage) error {
	deletionType, ok := raw.EventData["deletion_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid deletion_type in event data")
	}

	switch deletionType {
	case "single":
		// A moderation-originated deletion already carries the internal UUID (the
		// dashboard knows the id of the rendered message it moderated), so trust it and
		// skip the registry lookup. This is the only resolution path for platforms whose
		// listener does not populate the msgid registry — only twitch-listener does — so
		// it is what makes moderation reflect-back work for Discord/Kick/YouTube.
		if internalUUID, ok := raw.EventData["target_uuid"].(string); ok && internalUUID != "" {
			break
		}

		// Otherwise (a native platform deletion, e.g. Twitch EventSub, which carries only
		// the platform message id) reverse-resolve it to our internal UUID via the registry.
		platformMsgID, ok := raw.EventData["target_msg_id"].(string)
		if !ok {
			return fmt.Errorf("missing target_msg_id for single deletion")
		}

		internalUUID, err := c.msgIDRegistry.Lookup(ctx, raw.Platform, raw.ChannelID, platformMsgID)
		if err != nil {
			// Message not in registry yet - buffer deletion
			c.logger.Debug("Buffering deletion for message not yet in registry",
				zap.String("platform_msg_id", platformMsgID),
			)

			if err := c.deletionBuffer.Add(ctx, raw.Platform, raw.ChannelID, platformMsgID, raw); err != nil {
				c.logger.Error("Failed to buffer deletion", zap.Error(err))
				return err
			}

			c.metrics.DeletionsBuffered.Inc()
			return nil // Successfully buffered
		}

		// Message found - proceed with deletion
		raw.EventData["target_uuid"] = internalUUID

	case "batch", "clear":
		// No registry lookup needed - frontend filters by user ID or clears all
		// EventData already contains target_user_id for batch, nothing for clear

	default:
		c.logger.Warn("Unknown deletion type", zap.String("type", deletionType))
		return fmt.Errorf("unknown deletion type: %s", deletionType)
	}

	// Continue with normal processing (normalize, enrich, route, publish)
	// Handler will call normalizer which handles deletion events specially
	start := time.Now()
	if err := c.handler(ctx, raw); err != nil {
		c.metrics.RecordMessageProcessed("message-processor", raw.Platform, "deletion_handler", "failed")
		return fmt.Errorf("deletion handler failed: %w", err)
	}

	c.metrics.RecordMessageProcessed("message-processor", raw.Platform, "deletion_handler", "success")
	c.metrics.ProcessingDuration.WithLabelValues("message-processor", raw.Platform).Observe(time.Since(start).Seconds())

	return nil
}

// streamEntryLag parses the Redis stream entry ID to compute the age of the message.
// Redis stream IDs have the format "{unix_ms}-{sequence}". Returns lag in seconds and true on success.
func streamEntryLag(streamID string) (float64, bool) {
	parts := strings.SplitN(streamID, "-", 2)
	if len(parts) == 0 {
		return 0, false
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	msgTime := time.UnixMilli(ms)
	lagSeconds := time.Since(msgTime).Seconds()
	if lagSeconds < 0 {
		lagSeconds = 0
	}
	return lagSeconds, true
}
