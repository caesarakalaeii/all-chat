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
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DLQStreamKey is the Redis Streams key for the dead-letter queue.
const DLQStreamKey = "chat:dlq"

// writeToDLQ writes a failed message to the dead-letter queue with full context.
// This is a best-effort operation — on DLQ write failure the error is logged but not propagated.
// Fields written per D-11: original_stream_id, source_service, failure_reason, retry_count,
// original_data (from originalValues["data"]), dlq_timestamp.
func (c *StreamConsumer) writeToDLQ(ctx context.Context, originalID, sourceService, failureReason string, retryCount int, originalValues map[string]interface{}) {
	originalData, _ := originalValues["data"].(string)

	_, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: DLQStreamKey,
		ID:     "*",
		Values: map[string]interface{}{
			"original_stream_id": originalID,
			"source_service":     sourceService,
			"failure_reason":     failureReason,
			"retry_count":        fmt.Sprintf("%d", retryCount),
			"original_data":      originalData,
			"dlq_timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		// F-05: structured sentinel log so DLQ write failures are searchable.
		c.logger.Error("dlq_write_failure",
			zap.String("stream", DLQStreamKey),
			zap.String("message_id", originalID),
			zap.Error(err),
		)
		c.metrics.DLQWriteFailures.Inc()
		return
	}

	c.metrics.DLQMessagesTotal.WithLabelValues(sourceService, failureReason).Inc()
}

// trimDLQ removes DLQ entries older than 7 days. Runs in a goroutine with a 1-hour ticker.
// Exits when ctx is cancelled.
func (c *StreamConsumer) trimDLQ(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			minID := formatStreamIDInternal(cutoff.UnixMilli())
			if err := c.client.XTrimMinID(ctx, DLQStreamKey, minID).Err(); err != nil {
				c.logger.Warn("Failed to trim DLQ",
					zap.String("stream", DLQStreamKey),
					zap.Error(err),
				)
			} else {
				c.logger.Debug("DLQ trimmed",
					zap.String("min_id", minID),
				)
			}
		}
	}
}

// drainPEL claims and processes messages in the Pending Entries List (PEL) that have been
// idle for more than 5 minutes. This handles PEL orphans left by crashed replicas.
// Called once at startup before entering the normal consume loop (per MP-02 fix).
func (c *StreamConsumer) drainPEL(ctx context.Context) {
	cursor := "0-0"
	for {
		messages, nextCursor, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   StreamKey,
			Group:    ConsumerGroup,
			Consumer: c.consumerName,
			MinIdle:  5 * time.Minute,
			Start:    cursor,
			Count:    100,
		}).Result()
		if err != nil {
			c.logger.Error("Failed to claim PEL messages on startup",
				zap.String("consumer", c.consumerName),
				zap.Error(err),
			)
			return
		}

		c.metrics.PELPendingMessages.WithLabelValues(c.consumerName).Set(float64(len(messages)))

		for _, msg := range messages {
			if err := c.processAndAck(ctx, msg); err != nil {
				c.logger.Error("Failed to process PEL message",
					zap.String("stream_id", msg.ID),
					zap.Error(err),
				)
			}
		}

		// XAutoClaim returns "0-0" when there are no more entries to claim
		if nextCursor == "0-0" {
			break
		}
		cursor = nextCursor
	}

	c.logger.Info("PEL drain complete",
		zap.String("consumer", c.consumerName),
	)
}

// formatStreamIDInternal formats a Unix millisecond timestamp as a Redis stream ID.
func formatStreamIDInternal(ms int64) string {
	return fmt.Sprintf("%d-0", ms)
}
