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

package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DefaultAlertChannel is the Redis Pub/Sub channel the discord-bot subscribes to.
const DefaultAlertChannel = "quota:alerts"

// Notifier publishes QuotaEvents to a Redis Pub/Sub channel and logs them.
type Notifier struct {
	redisClient *redis.Client
	logger      *zap.Logger
	enabled     bool
	channel     string
}

// NewNotifier creates a quota notifier. An empty channel defaults to "quota:alerts".
func NewNotifier(redisClient *redis.Client, logger *zap.Logger, enabled bool, channel string) *Notifier {
	if channel == "" {
		channel = DefaultAlertChannel
	}
	logger.Info("Quota notifier initialized",
		zap.Bool("enabled", enabled),
		zap.String("channel", channel),
	)
	return &Notifier{
		redisClient: redisClient,
		logger:      logger,
		enabled:     enabled,
		channel:     channel,
	}
}

// NotifyStateTransition notifies about a quota state transition.
func (n *Notifier) NotifyStateTransition(ctx context.Context, oldState, newState QuotaState, percentage float64, used, limit int) error {
	if !n.enabled {
		return nil
	}
	event := QuotaEvent{
		Type:            EventStateChanged,
		Timestamp:       time.Now(),
		GlobalState:     newState,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		PreviousState:   &oldState,
		Message:         fmt.Sprintf("Quota state changed from %s to %s (%.2f%% used)", oldState, newState, percentage),
		Severity:        Severity(newState),
	}
	return n.publishEvent(ctx, event)
}

// NotifyThresholdCrossed notifies when a 5% quota threshold is crossed.
func (n *Notifier) NotifyThresholdCrossed(ctx context.Context, state QuotaState, threshold, percentage float64, used, limit int) error {
	if !n.enabled {
		return nil
	}
	event := QuotaEvent{
		Type:            EventThresholdCrossed,
		Timestamp:       time.Now(),
		GlobalState:     state,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		Message:         fmt.Sprintf("Quota crossed %.0f%% threshold, now at %.2f%%", threshold, percentage),
		Severity:        Severity(state),
	}
	return n.publishEvent(ctx, event)
}

// NotifyQuotaExhausted notifies when quota is nearly exhausted.
func (n *Notifier) NotifyQuotaExhausted(ctx context.Context, percentage float64, used, limit int, affectedChannels []string) error {
	if !n.enabled {
		return nil
	}
	event := QuotaEvent{
		Type:             EventQuotaExhausted,
		Timestamp:        time.Now(),
		GlobalState:      QuotaStateExhausted,
		UsagePercentage:  percentage,
		UnitsUsed:        used,
		UnitsLimit:       limit,
		UnitsRemaining:   limit - used,
		AffectedChannels: affectedChannels,
		Message:          fmt.Sprintf("Quota exhausted: %.2f%% used, %d channels affected", percentage, len(affectedChannels)),
		Severity:         "error",
	}
	return n.publishEvent(ctx, event)
}

// NotifyQuotaDepleted notifies when quota is completely depleted.
func (n *Notifier) NotifyQuotaDepleted(ctx context.Context, used, limit int) error {
	if !n.enabled {
		return nil
	}
	percentage := 0.0
	if limit > 0 {
		percentage = float64(used) / float64(limit) * 100
	}
	event := QuotaEvent{
		Type:            EventQuotaDepleted,
		Timestamp:       time.Now(),
		GlobalState:     QuotaStateDepleted,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  0,
		Message:         fmt.Sprintf("Quota depleted: %d/%d units used, all API requests blocked", used, limit),
		Severity:        "critical",
	}
	return n.publishEvent(ctx, event)
}

// NotifyQuotaRecovered notifies when quota recovers to a healthy state.
func (n *Notifier) NotifyQuotaRecovered(ctx context.Context, percentage float64, used, limit int) error {
	if !n.enabled {
		return nil
	}
	event := QuotaEvent{
		Type:            EventQuotaRecovered,
		Timestamp:       time.Now(),
		GlobalState:     QuotaStateHealthy,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		Message:         fmt.Sprintf("Quota recovered to healthy state: %.2f%% used", percentage),
		Severity:        "info",
	}
	return n.publishEvent(ctx, event)
}

// NotifyChannelQuotaExceeded notifies when a channel exceeds its quota allocation.
func (n *Notifier) NotifyChannelQuotaExceeded(ctx context.Context, channelID string, used, limit int) error {
	if !n.enabled {
		return nil
	}
	percentage := 0.0
	if limit > 0 {
		percentage = float64(used) / float64(limit) * 100
	}
	event := QuotaEvent{
		Type:             EventChannelExceeded,
		Timestamp:        time.Now(),
		GlobalState:      QuotaStateHealthy, // not relevant for channel-specific events
		UsagePercentage:  percentage,
		UnitsUsed:        used,
		UnitsLimit:       limit,
		UnitsRemaining:   limit - used,
		AffectedChannels: []string{channelID},
		Message:          fmt.Sprintf("Channel %s exceeded quota allocation: %d/%d units", channelID, used, limit),
		Severity:         "warning",
	}
	return n.publishEvent(ctx, event)
}

// publishEvent logs the event then publishes it to the Redis Pub/Sub channel.
func (n *Notifier) publishEvent(ctx context.Context, event QuotaEvent) error {
	n.logEvent(event)

	payload, err := json.Marshal(event)
	if err != nil {
		n.logger.Error("Failed to marshal quota event",
			zap.String("event_type", string(event.Type)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := n.redisClient.Publish(ctx, n.channel, payload).Err(); err != nil {
		n.logger.Error("Failed to publish quota event to Redis",
			zap.String("event_type", string(event.Type)),
			zap.String("channel", n.channel),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish event: %w", err)
	}

	n.logger.Debug("Published quota event",
		zap.String("event_type", string(event.Type)),
		zap.String("channel", n.channel),
	)
	return nil
}

// logEvent logs the event at a level matching its severity.
func (n *Notifier) logEvent(event QuotaEvent) {
	fields := []zap.Field{
		zap.String("event_type", string(event.Type)),
		zap.String("global_state", string(event.GlobalState)),
		zap.Float64("usage_percentage", event.UsagePercentage),
		zap.Int("units_used", event.UnitsUsed),
		zap.Int("units_remaining", event.UnitsRemaining),
		zap.String("message", event.Message),
	}
	if len(event.AffectedChannels) > 0 {
		fields = append(fields, zap.Int("affected_channels", len(event.AffectedChannels)))
	}

	switch event.Severity {
	case "critical":
		n.logger.Error("Quota Alert (CRITICAL)", fields...)
	case "error":
		n.logger.Error("Quota Alert (ERROR)", fields...)
	case "warning":
		n.logger.Warn("Quota Alert (WARNING)", fields...)
	default:
		n.logger.Info("Quota Alert", fields...)
	}
}
