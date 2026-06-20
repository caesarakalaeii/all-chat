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

// Package publisher emits moderation-originated message_deletion events onto the
// chat:raw Redis Stream, reusing the exact event shape the read pipeline already
// consumes (message-processor.NormalizeDeletion -> overlay:{id} Pub/Sub -> gateway
// WebSocket -> frontend onDeletion). A successful platform moderation action
// (delete / timeout / ban) publishes one of these so the moderated message
// disappears from every overlay AND the dashboard live, with no new event types.
//
// The shapes intentionally mirror the Twitch EventSub listener's deletion builders
// (services/twitch-eventsub-listener/webhooks/handler.go) and are validated against
// services/message-processor/consumer/stream_consumer.go (processDeletionEvent) and
// services/message-processor/normalizer/normalizer.go (NormalizeDeletion).
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Stream the read pipeline consumes (XREADGROUP group
	// "message-processors"). Identical to every listener's publish target.
	StreamKey = "chat:raw"

	// maxStreamLength bounds the stream as a sliding window (matches the listeners).
	maxStreamLength = 100000

	// ringBufferCapacity is how many events the retry buffer holds before dropping.
	ringBufferCapacity = 1000

	// serviceName labels the ring-buffer metrics for this service.
	serviceName = "moderation-service"
)

// DeletionPublisher writes message_deletion events to chat:raw, buffered through a
// RingBufferPublisher so transient XADD failures are retried rather than dropped.
type DeletionPublisher struct {
	ringBuffer *sharedlistener.RingBufferPublisher
	logger     *zap.Logger
}

// NewDeletionPublisher wires a publisher backed by Redis XADD on the given client.
func NewDeletionPublisher(client *redis.Client, logger *zap.Logger) *DeletionPublisher {
	return newDeletionPublisher(buildXAddFunc(client), logger, prometheus.DefaultRegisterer)
}

// newDeletionPublisher is the internal constructor used by both production code and
// tests (tests inject a capturing PublishFunc and a fresh registry).
func newDeletionPublisher(publishFn sharedlistener.PublishFunc, logger *zap.Logger, reg prometheus.Registerer) *DeletionPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(ringBufferCapacity, publishFn, logger, serviceName, reg)
	return &DeletionPublisher{ringBuffer: rb, logger: logger}
}

// buildXAddFunc returns a PublishFunc that writes the serialised event to chat:raw
// using the "data" field, exactly like the listeners' stream publishers.
func buildXAddFunc(client *redis.Client) sharedlistener.PublishFunc {
	return func(ctx context.Context, payload []byte) error {
		return client.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamKey,
			MaxLen: maxStreamLength,
			Approx: true,
			Values: map[string]interface{}{"data": string(payload)},
		}).Err()
	}
}

// Publish serialises the event and delegates to the ring buffer. A transient XADD
// failure is buffered for retry and nil is returned (the caller is not blocked).
func (p *DeletionPublisher) Publish(ctx context.Context, msg *mpmodels.RawChatMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal deletion event: %w", err)
	}
	return p.ringBuffer.Publish(ctx, payload)
}

// Stop drains the retry goroutine; call during graceful shutdown.
func (p *DeletionPublisher) Stop() {
	if p.ringBuffer != nil {
		p.ringBuffer.Stop()
	}
}

// BuildSingleDeletion builds a "single" message_deletion for one removed message.
// nativeMsgID is the platform's own message id (kept as target_msg_id for debugging
// and as the registry fallback). targetUUID is our internal message id, which the
// dashboard already knows for the message it moderated: passing it lets the
// message-processor consumer match the deletion directly (frontend matches by
// target_uuid), without depending on the msgid registry — which only twitch-listener
// populates. This is what makes reflect-back work for Discord/Kick/YouTube too.
// channelID MUST match the ingest/overlay-routing key for the channel (see
// normalizeChannelID).
func BuildSingleDeletion(platform, channelID, nativeMsgID, targetUUID string) *mpmodels.RawChatMessage {
	data := map[string]interface{}{
		"deletion_type": "single",
		"target_msg_id": nativeMsgID,
	}
	if targetUUID != "" {
		data["target_uuid"] = targetUUID
	}
	return &mpmodels.RawChatMessage{
		MessageID: uuid.NewString(), // id of the deletion event itself
		Platform:  platform,
		ChannelID: normalizeChannelID(platform, channelID),
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion",
		EventData: data,
	}
}

// BuildBatchDeletion builds a "batch" message_deletion covering all of a user's
// messages (timeout or ban). A positive banDurationSeconds marks it a timeout; 0
// marks a permanent ban — the frontend distinguishes purely by the presence of
// ban_duration (see frontend overlayViewModel.ts deletionKind).
func BuildBatchDeletion(platform, channelID, targetUserID, targetUsername string, banDurationSeconds int) *mpmodels.RawChatMessage {
	data := map[string]interface{}{
		"deletion_type":   "batch",
		"target_user_id":  targetUserID,
		"target_username": targetUsername,
	}
	if banDurationSeconds > 0 {
		data["ban_duration"] = banDurationSeconds
	}
	return &mpmodels.RawChatMessage{
		MessageID: uuid.NewString(),
		Platform:  platform,
		ChannelID: normalizeChannelID(platform, channelID),
		UserID:    targetUserID,
		Username:  targetUsername,
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion",
		EventData: data,
	}
}

// normalizeChannelID lower-cases Twitch channel logins to match the chat path and
// the msgid registry key (the EventSub listener does the same). Other platforms key
// on opaque/case-sensitive ids (e.g. YouTube "UC..." channel ids, Kick numeric
// chatroom ids), so they are passed through unchanged.
func normalizeChannelID(platform, channelID string) string {
	if platform == "twitch" {
		return strings.ToLower(channelID)
	}
	return channelID
}
