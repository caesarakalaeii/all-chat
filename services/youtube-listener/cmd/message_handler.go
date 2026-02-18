package main

import (
	"context"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/publisher"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
)

// MessageHandler handles parsed YouTube chat messages
type MessageHandler struct {
	publisher    *publisher.StreamPublisher
	quotaTracker *quota.Tracker
	registry     registry.MessageIDRegistry
	logger       *zap.Logger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	publisher *publisher.StreamPublisher,
	quotaTracker *quota.Tracker,
	registry registry.MessageIDRegistry,
	logger *zap.Logger,
) *MessageHandler {
	return &MessageHandler{
		publisher:    publisher,
		quotaTracker: quotaTracker,
		registry:     registry,
		logger:       logger,
	}
}

// HandleMessages publishes messages to Redis Streams
func (h *MessageHandler) HandleMessages(ctx context.Context, messages []*models.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	// Quota is now tracked in api.Client.GetChatMessages() before the API call
	// This ensures we don't exceed quota and can fail fast if quota is exhausted

	// Add to registry IMMEDIATELY at capture point (follows Twitch pattern)
	// This happens BEFORE publishing to Redis Streams to ensure registry is populated first
	for _, rawMsg := range messages {
		if platformMsgID := rawMsg.Tags["youtube_message_id"]; platformMsgID != "" {
			if err := h.registry.Add(ctx, rawMsg.Platform, rawMsg.ChannelID, platformMsgID, rawMsg.MessageID); err != nil {
				h.logger.Warn("Failed to add YouTube message to registry",
					zap.Error(err),
					zap.String("youtube_msg_id", platformMsgID),
					zap.String("internal_uuid", rawMsg.MessageID),
				)
				// Continue - registry is best-effort, don't block message publishing
			}
		}
	}

	// Publish messages to Redis Streams (AFTER registry)
	if err := h.publisher.PublishBatch(ctx, messages); err != nil {
		h.logger.Error("Failed to publish messages",
			zap.Int("count", len(messages)),
			zap.Error(err),
		)
		return err
	}

	h.logger.Debug("Successfully published messages",
		zap.Int("count", len(messages)),
	)

	return nil
}
