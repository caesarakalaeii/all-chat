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

// Package consumer reads the chat:raw Redis Stream with the alert-processor's
// own consumer group and hands event-typed messages to the alert pipeline.
package consumer

import (
	"context"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/metrics"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key to consume from. Shared with the
	// message-processor (ADR-0002); this service reads it independently.
	StreamKey = "chat:raw"

	// ConsumerGroup is this service's consumer group. Deliberately distinct
	// from the message-processor's "message-processor" group: both consume the
	// same stream at-least-once without seeing each other's PELs.
	ConsumerGroup = "alert-processors"

	ReadCount = 100

	ReadBlockTime = 5 * time.Second

	// claimMinIdle is how long an entry must sit in this group's PEL before
	// reclaim treats it as orphaned (its processor died mid-batch). Long
	// enough that a slow-but-alive batch is never stolen from itself.
	claimMinIdle = 5 * time.Minute

	// reclaimInterval is how often to sweep the PEL for orphaned entries.
	reclaimInterval = 1 * time.Minute
)

// Handler processes one event-typed raw message. An error means "leave the
// stream entry unacked so it is redelivered"; nil means "done, ACK".
type Handler interface {
	Handle(ctx context.Context, raw *mpmodels.RawChatMessage) error
}

// Consumer consumes event-typed messages from the chat:raw stream.
type Consumer struct {
	client       *redis.Client
	log          *zap.Logger
	handler      Handler
	consumerName string
	stopCh       chan struct{}

	// Tunables with production defaults, fields so tests can shrink them.
	blockFor     time.Duration
	claimMinIdle time.Duration

	// groupErrLogged is only ever touched by the consume loop, which is the
	// sole caller of ensureGroup.
	groupErrLogged bool
}

// New creates a consumer. consumerName should be os.Hostname() so each pod
// owns its own pending entries.
func New(client *redis.Client, log *zap.Logger, handler Handler, consumerName string) *Consumer {
	return &Consumer{
		client:       client,
		log:          log,
		handler:      handler,
		consumerName: consumerName,
		stopCh:       make(chan struct{}),
		blockFor:     ReadBlockTime,
		claimMinIdle: claimMinIdle,
	}
}

// Start launches the consume and reclaim loops. It returns without error even
// when chat:raw does not exist yet — in a fresh deployment the listeners have
// not XADDed anything, and the group is created lazily once the stream appears.
func (c *Consumer) Start(ctx context.Context) {
	c.log.Info("Stream consumer starting",
		zap.String("stream", StreamKey),
		zap.String("group", ConsumerGroup),
		zap.String("consumer", c.consumerName),
	)
	go c.consumeLoop(ctx)
	go c.reclaimLoop(ctx)
}

// Stop stops the consume and reclaim loops.
func (c *Consumer) Stop() {
	close(c.stopCh)
	c.log.Info("Stream consumer stopped")
}

// ensureGroup creates the consumer group starting at offset "0" (not "$": a
// cold start must not silently skip messages the listeners already buffered).
//
// It deliberately uses XGROUP CREATE without MKSTREAM: creating chat:raw is
// the listeners' job, and a consumer that fabricates an empty stream plus a
// second group on it hides a miswired deployment behind a healthy-looking pod.
// Until the stream exists this fails — logged once, retried lazily — and the
// service still serves /health and /metrics.
func (c *Consumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreate(ctx, StreamKey, ConsumerGroup, "0").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}

	if !c.groupErrLogged {
		c.groupErrLogged = true
		c.log.Warn("Cannot create consumer group yet (chat:raw missing?); will retry lazily",
			zap.String("stream", StreamKey),
			zap.String("group", ConsumerGroup),
			zap.Error(err),
		)
	} else {
		c.log.Debug("Consumer group still cannot be created", zap.Error(err))
	}
	return err
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			if err := c.readAndProcess(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				c.log.Error("Error reading messages", zap.Error(err))
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (c *Consumer) reclaimLoop(ctx context.Context) {
	ticker := time.NewTicker(reclaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reclaimStale(ctx)
		}
	}
}

// readAndProcess reads one batch of new entries and processes each.
func (c *Consumer) readAndProcess(ctx context.Context) error {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: c.consumerName,
		Streams:  []string{StreamKey, ">"},
		Count:    ReadCount,
		Block:    c.blockFor,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // timeout, no messages
		}
		// NOGROUP: stream or group missing (fresh deployment, or Redis was
		// reset). Recreate lazily — but first wait out the block time: XREADGROUP
		// validates the group before honoring Block, so this error returns
		// immediately and without the explicit wait the consume loop would
		// busy-spin at full CPU until the stream first appears.
		if strings.Contains(err.Error(), "NOGROUP") {
			_ = c.ensureGroup(ctx)
			select {
			case <-time.After(c.blockFor):
			case <-c.stopCh:
			case <-ctx.Done():
			}
			return nil
		}
		return err
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			if err := c.processMessage(ctx, msg); err != nil {
				// Left unacked on purpose: redelivery is the recovery. Logged
				// and the rest of the batch still runs.
				c.log.Warn("Alert processing failed; entry stays pending for redelivery",
					zap.String("stream_id", msg.ID),
					zap.Error(err),
				)
			}
		}
	}
	return nil
}

// reclaimStale re-delivers entries this group has left unacked past
// claimMinIdle — the entries of a processor that died mid-batch. Only this
// group's pending entries are touched; the message-processor's group owns its
// own PEL.
func (c *Consumer) reclaimStale(ctx context.Context) {
	start := "0-0"
	for {
		messages, next, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   StreamKey,
			Group:    ConsumerGroup,
			Consumer: c.consumerName,
			MinIdle:  c.claimMinIdle,
			Start:    start,
			Count:    ReadCount,
		}).Result()
		if err != nil {
			if !strings.Contains(err.Error(), "NOGROUP") {
				c.log.Warn("Reclaim sweep failed", zap.Error(err))
			}
			return
		}

		for _, msg := range messages {
			if err := c.processMessage(ctx, msg); err != nil {
				c.log.Warn("Reclaimed alert failed again; stays pending",
					zap.String("stream_id", msg.ID),
					zap.Error(err),
				)
			}
		}

		if next == "0-0" || len(messages) == 0 {
			return
		}
		start = next
	}
}

// processMessage handles one stream entry: parse, drop what is not an alert
// event, run the pipeline, and ACK only on success.
func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	data, ok := msg.Values["data"].(string)
	if !ok {
		// No payload can ever become valid — ACK and drop rather than poison
		// the PEL forever. The message-processor parks its copy of the same
		// entry in a DLQ; this service deliberately does not duplicate that:
		// the entry is already parked for humans to inspect, and an alert-side
		// DLQ would only ever replay the same undecodable bytes.
		c.log.Warn("Dropping stream entry without a data field", zap.String("stream_id", msg.ID))
		return c.ack(ctx, msg.ID)
	}

	raw, err := mpmodels.ParseRawMessage([]byte(data))
	if err != nil {
		// Same reasoning: the chat pipeline's DLQ already holds the bytes.
		c.log.Warn("Dropping undecodable stream entry",
			zap.String("stream_id", msg.ID),
			zap.Error(err),
		)
		return c.ack(ctx, msg.ID)
	}

	eventTypeLabel := raw.EventType
	if eventTypeLabel == "" {
		eventTypeLabel = "chat"
	}
	metrics.RecordConsumed(raw.Platform, eventTypeLabel)

	// Pure chat never enters the alert path, and neither do moderation
	// deletions: deletions are reflect-back plumbing for the chat pipeline
	// (they remove already-rendered lines), not something an alert overlay can
	// show. Both are ACKed so the PEL only ever holds real alert work.
	if raw.EventType == "" || raw.EventType == "chat" || raw.EventType == "message_deletion" {
		return c.ack(ctx, msg.ID)
	}

	if err := c.handler.Handle(ctx, raw); err != nil {
		return err // NOT acked — redelivery is the recovery
	}
	return c.ack(ctx, msg.ID)
}

func (c *Consumer) ack(ctx context.Context, streamID string) error {
	return c.client.XAck(ctx, StreamKey, ConsumerGroup, streamID).Err()
}
