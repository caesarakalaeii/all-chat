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

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// nativeStreamMaxLen bounds the engagement:twitch-native stream. Lifecycle
// events are low-volume (a handful per poll/prediction), so this is weeks of
// backlog headroom for a temporarily-down engagement-service.
const nativeStreamMaxLen = 10000

// handlePollEvent normalizes a channel.poll.begin/progress/end notification
// into the shared NativeEngagementEvent contract and hands it to the durable
// engagement:twitch-native stream. These are engagement-domain state events,
// not chat render events, so they bypass the chat:raw publisher entirely.
func (h *Handler) handlePollEvent(ctx context.Context, eventData json.RawMessage, phase string) error {
	var event eventsub.PollEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("unmarshal channel.poll event: %w", err)
	}
	if event.ID == "" || event.BroadcasterUserLogin == "" {
		h.logger.Warn("channel.poll event missing id or broadcaster login")
		return nil
	}
	outcomes := make([]mpmodels.NativeOutcome, 0, len(event.Choices))
	for i, choice := range event.Choices {
		outcomes = append(outcomes, mpmodels.NativeOutcome{
			ExternalID: choice.ID,
			Idx:        i + 1,
			Label:      choice.Title,
			Votes:      choice.Votes,
		})
	}
	return h.publishNative(ctx, mpmodels.NativeEngagementEvent{
		Kind:       mpmodels.NativeKindPoll,
		Event:      phase,
		Platform:   "twitch",
		ChannelID:  strings.ToLower(event.BroadcasterUserLogin),
		ExternalID: event.ID,
		Title:      event.Title,
		Outcomes:   outcomes,
		Status:     event.Status,
		EndsAt:     event.EndsAt,
		Timestamp:  time.Now(),
	})
}

// handlePredictionEvent normalizes a channel.prediction.begin/progress/lock/end
// notification into the shared NativeEngagementEvent contract. Only aggregates
// cross this boundary — mirrored predictions run on Twitch channel points and
// never touch All-Chat viewer points.
func (h *Handler) handlePredictionEvent(ctx context.Context, eventData json.RawMessage, phase string) error {
	var event eventsub.PredictionEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("unmarshal channel.prediction event: %w", err)
	}
	if event.ID == "" || event.BroadcasterUserLogin == "" {
		h.logger.Warn("channel.prediction event missing id or broadcaster login")
		return nil
	}
	outcomes := make([]mpmodels.NativeOutcome, 0, len(event.Outcomes))
	for i, out := range event.Outcomes {
		outcomes = append(outcomes, mpmodels.NativeOutcome{
			ExternalID: out.ID,
			Idx:        i + 1,
			Label:      out.Title,
			Color:      out.Color,
			Points:     out.ChannelPoints,
			Users:      out.Users,
		})
	}
	return h.publishNative(ctx, mpmodels.NativeEngagementEvent{
		Kind:              mpmodels.NativeKindPrediction,
		Event:             phase,
		Platform:          "twitch",
		ChannelID:         strings.ToLower(event.BroadcasterUserLogin),
		ExternalID:        event.ID,
		Title:             event.Title,
		Outcomes:          outcomes,
		Status:            event.Status,
		WinningExternalID: event.WinningOutcomeID,
		LocksAt:           event.LocksAt,
		Timestamp:         time.Now(),
	})
}

// publishNative XADDs a normalized native engagement event to the durable
// stream. An error propagates to the webhook layer, which then skips setting
// the message dedup key so Twitch redelivers — the same at-least-once
// semantics the chat:raw path gets from its ring-buffered publisher.
func (h *Handler) publishNative(ctx context.Context, ev mpmodels.NativeEngagementEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal native engagement event: %w", err)
	}
	if err := h.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: mpmodels.StreamEngagementTwitchNative,
		MaxLen: nativeStreamMaxLen,
		Approx: true,
		Values: map[string]interface{}{mpmodels.FieldEngagementData: string(data)},
	}).Err(); err != nil {
		return fmt.Errorf("xadd native engagement event: %w", err)
	}
	h.logger.Debug("Published native engagement event",
		zap.String("kind", ev.Kind),
		zap.String("event", ev.Event),
		zap.String("channel", ev.ChannelID),
		zap.String("external_id", ev.ExternalID))
	return nil
}
