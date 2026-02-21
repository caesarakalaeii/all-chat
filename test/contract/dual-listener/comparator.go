package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/test/shared"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Comparator consumes messages from both official and InnerTube Redis Streams
// and performs content-based correlation to detect mismatches
type Comparator struct {
	redisClient     *redis.Client
	matcher         *MessageMatcher
	artifactWriter  *ArtifactWriter
	logger          *zap.Logger
	streamPrefix    StreamPrefix

	// State
	officialBuffer  []*shared.RawChatMessage // Rolling window for ±5 context
	innertubeBuffer []*shared.RawChatMessage
	stats           ComparisonStats
}

// StreamPrefix holds the Redis stream prefixes for official and InnerTube
type StreamPrefix struct {
	Official  string
	InnerTube string
}

// ComparisonStats tracks overall comparison metrics
type ComparisonStats struct {
	TotalProcessed        int
	TotalMatched          int
	TotalMissingInner     int
	TotalMissingOfficial  int
	TotalContentMismatch  int
	CurrentMismatchRate   float64
	StartTime             time.Time
	LastProcessedTime     time.Time
}

// MessageMatcher wraps the shared message matcher with time window configuration
type MessageMatcher struct {
	timeWindow time.Duration
}

// NewMessageMatcher creates a matcher with specified time window
func NewMessageMatcher(timeWindow time.Duration) *MessageMatcher {
	return &MessageMatcher{
		timeWindow: timeWindow,
	}
}

// NewComparator creates a new dual-listener comparator
func NewComparator(
	redisClient *redis.Client,
	streamPrefix StreamPrefix,
	artifactWriter *ArtifactWriter,
	logger *zap.Logger,
) *Comparator {
	return &Comparator{
		redisClient:     redisClient,
		matcher:         NewMessageMatcher(time.Second), // 1s time window per user constraint
		artifactWriter:  artifactWriter,
		logger:          logger,
		streamPrefix:    streamPrefix,
		officialBuffer:  make([]*shared.RawChatMessage, 0, 1000),
		innertubeBuffer: make([]*shared.RawChatMessage, 0, 1000),
		stats: ComparisonStats{
			StartTime: time.Now(),
		},
	}
}

// Run starts the comparison process for the specified duration
func (c *Comparator) Run(ctx context.Context, duration time.Duration) error {
	c.logger.Info("Starting dual-listener comparison",
		zap.Duration("duration", duration),
		zap.String("official_stream", c.streamPrefix.Official+":chat:raw"),
		zap.String("innertube_stream", c.streamPrefix.InnerTube+":chat:raw"),
	)

	// Create consumer groups
	officialStream := c.streamPrefix.Official + ":chat:raw"
	innertubeStream := c.streamPrefix.InnerTube + ":chat:raw"
	consumerGroup := "dual-listener-test"

	if err := c.createConsumerGroup(ctx, officialStream, consumerGroup); err != nil {
		return fmt.Errorf("create official consumer group: %w", err)
	}
	if err := c.createConsumerGroup(ctx, innertubeStream, consumerGroup); err != nil {
		return fmt.Errorf("create innertube consumer group: %w", err)
	}

	// Set up ticker for periodic processing
	processTicker := time.NewTicker(10 * time.Second) // Process every 10s
	progressTicker := time.NewTicker(5 * time.Minute) // Log progress every 5m
	defer processTicker.Stop()
	defer progressTicker.Stop()

	// Set deadline for test duration
	deadline := time.Now().Add(duration)
	timeoutCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	c.logger.Info("Comparison started", zap.Time("deadline", deadline))

	for {
		select {
		case <-timeoutCtx.Done():
			c.logger.Info("Test duration complete, finalizing...")
			return c.finalize()

		case <-processTicker.C:
			if err := c.processBatch(ctx, officialStream, innertubeStream, consumerGroup); err != nil {
				c.logger.Error("Batch processing error", zap.Error(err))
				// Continue on error - we want 24h uninterrupted operation
			}

		case <-progressTicker.C:
			c.logProgress()
		}
	}
}

// createConsumerGroup creates a Redis consumer group (idempotent)
func (c *Comparator) createConsumerGroup(ctx context.Context, stream, group string) error {
	err := c.redisClient.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// processBatch consumes messages from both streams and performs correlation
func (c *Comparator) processBatch(ctx context.Context, officialStream, innertubeStream, group string) error {
	// Consume from official stream
	officialMsgs, err := c.consumeMessages(ctx, officialStream, group, "official-consumer")
	if err != nil {
		return fmt.Errorf("consume official messages: %w", err)
	}
	c.officialBuffer = append(c.officialBuffer, officialMsgs...)

	// Consume from InnerTube stream
	innertubeMsgs, err := c.consumeMessages(ctx, innertubeStream, group, "innertube-consumer")
	if err != nil {
		return fmt.Errorf("consume innertube messages: %w", err)
	}
	c.innertubeBuffer = append(c.innertubeBuffer, innertubeMsgs...)

	// Process correlation if we have messages
	if len(c.officialBuffer) > 0 || len(c.innertubeBuffer) > 0 {
		if err := c.correlateMessages(); err != nil {
			return fmt.Errorf("correlate messages: %w", err)
		}
	}

	return nil
}

// consumeMessages reads messages from a Redis Stream using XREADGROUP
func (c *Comparator) consumeMessages(ctx context.Context, stream, group, consumer string) ([]*shared.RawChatMessage, error) {
	// Read with 1s timeout
	streams, err := c.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    100, // Batch size
		Block:    time.Second,
	}).Result()

	if err == redis.Nil {
		// No messages available
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var messages []*shared.RawChatMessage
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			// Parse message
			data, ok := msg.Values["data"].(string)
			if !ok {
				c.logger.Warn("Invalid message format", zap.String("id", msg.ID))
				continue
			}

			var rawMsg shared.RawChatMessage
			if err := json.Unmarshal([]byte(data), &rawMsg); err != nil {
				c.logger.Warn("Failed to parse message", zap.Error(err), zap.String("id", msg.ID))
				continue
			}

			messages = append(messages, &rawMsg)

			// ACK the message
			c.redisClient.XAck(ctx, stream.Stream, group, msg.ID)
		}
	}

	return messages, nil
}

// correlateMessages performs content-based matching on buffered messages
func (c *Comparator) correlateMessages() error {
	// Match messages using content fingerprinting
	result := shared.MatchMessages(c.officialBuffer, c.innertubeBuffer, c.matcher.timeWindow)

	// Update overall stats
	c.stats.TotalProcessed += result.TotalMessages
	c.stats.TotalMatched += result.Matched
	c.stats.TotalMissingInner += result.MissingInInnerTube
	c.stats.TotalMissingOfficial += result.MissingInOfficial
	c.stats.TotalContentMismatch += result.ContentMismatches
	c.stats.LastProcessedTime = time.Now()

	// Recalculate overall mismatch rate
	if c.stats.TotalProcessed > 0 {
		totalMismatches := c.stats.TotalMissingInner + c.stats.TotalMissingOfficial + c.stats.TotalContentMismatch
		c.stats.CurrentMismatchRate = float64(totalMismatches) / float64(c.stats.TotalProcessed)
	}

	// Write artifacts for mismatches
	for _, mismatch := range result.Mismatches {
		if err := c.captureArtifact(mismatch); err != nil {
			c.logger.Error("Failed to capture artifact", zap.Error(err))
		}
	}

	// Clear buffers after processing
	c.officialBuffer = make([]*shared.RawChatMessage, 0, 1000)
	c.innertubeBuffer = make([]*shared.RawChatMessage, 0, 1000)

	return nil
}

// captureArtifact writes a mismatch artifact with ±5 message context
func (c *Comparator) captureArtifact(mismatch shared.MismatchDetail) error {
	// Build context from buffers
	// Note: For PoC, we use the current buffer state
	// In production, we'd maintain a longer rolling window
	context := SurroundingContext{
		Before: c.getSurroundingMessages(-5, mismatch),
		After:  c.getSurroundingMessages(5, mismatch),
	}

	return c.artifactWriter.CaptureArtifact(mismatch, context)
}

// getSurroundingMessages retrieves context messages around a mismatch
func (c *Comparator) getSurroundingMessages(offset int, mismatch shared.MismatchDetail) []*shared.RawChatMessage {
	// Simplified: return last 5 from buffer
	// Production would maintain indexed rolling window
	var buffer []*shared.RawChatMessage
	if mismatch.OfficialMessage != nil {
		buffer = c.officialBuffer
	} else {
		buffer = c.innertubeBuffer
	}

	count := 5
	if offset < 0 {
		count = -offset
	}

	start := len(buffer) - count
	if start < 0 {
		start = 0
	}

	if start >= len(buffer) {
		return []*shared.RawChatMessage{}
	}

	return buffer[start:len(buffer)]
}

// logProgress logs current comparison statistics
func (c *Comparator) logProgress() {
	c.logger.Info("Comparison progress",
		zap.Int("processed", c.stats.TotalProcessed),
		zap.Int("matched", c.stats.TotalMatched),
		zap.Int("missing_innertube", c.stats.TotalMissingInner),
		zap.Int("missing_official", c.stats.TotalMissingOfficial),
		zap.Int("content_mismatches", c.stats.TotalContentMismatch),
		zap.Float64("mismatch_rate_pct", c.stats.CurrentMismatchRate*100),
		zap.Duration("elapsed", time.Since(c.stats.StartTime)),
	)
}

// finalize completes the comparison and writes final report
func (c *Comparator) finalize() error {
	c.logger.Info("Finalizing comparison results")

	// Process any remaining messages
	if len(c.officialBuffer) > 0 || len(c.innertubeBuffer) > 0 {
		if err := c.correlateMessages(); err != nil {
			return fmt.Errorf("final correlation: %w", err)
		}
	}

	// Write final report
	if err := c.artifactWriter.WriteFinalReport(c.stats); err != nil {
		return fmt.Errorf("write final report: %w", err)
	}

	c.logger.Info("Comparison complete",
		zap.Int("total_messages", c.stats.TotalProcessed),
		zap.Float64("mismatch_rate_pct", c.stats.CurrentMismatchRate*100),
		zap.Duration("duration", time.Since(c.stats.StartTime)),
	)

	return nil
}

// GetStats returns current comparison statistics
func (c *Comparator) GetStats() ComparisonStats {
	return c.stats
}
