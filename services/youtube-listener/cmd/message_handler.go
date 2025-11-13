package main

import (
	"context"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/publisher"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
)

// MessageHandler handles parsed YouTube chat messages
type MessageHandler struct {
	publisher    *publisher.StreamPublisher
	quotaTracker *quota.Tracker
	logger       *zap.Logger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	publisher *publisher.StreamPublisher,
	quotaTracker *quota.Tracker,
	logger *zap.Logger,
) *MessageHandler {
	return &MessageHandler{
		publisher:    publisher,
		quotaTracker: quotaTracker,
		logger:       logger,
	}
}

// HandleMessages publishes messages to Redis Streams
func (h *MessageHandler) HandleMessages(ctx context.Context, messages []*models.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	// Record quota usage (liveChatMessages.list costs 5 units per API call)
	if err := h.quotaTracker.RecordUsage(ctx, quota.QuotaCostLiveChatMessages); err != nil {
		h.logger.Error("Failed to record quota usage", zap.Error(err))
		// Continue anyway - don't fail the message publishing
	}

	// Publish messages to Redis Streams
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
