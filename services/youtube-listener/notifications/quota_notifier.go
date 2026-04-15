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

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// QuotaEventType represents the type of quota event
type QuotaEventType string

const (
	EventStateChanged      QuotaEventType = "state_changed"
	EventThresholdCrossed  QuotaEventType = "threshold_crossed"
	EventQuotaExhausted    QuotaEventType = "quota_exhausted"
	EventQuotaDepleted     QuotaEventType = "quota_depleted"
	EventQuotaRecovered    QuotaEventType = "quota_recovered"
	EventChannelExceeded   QuotaEventType = "channel_quota_exceeded"
)

// QuotaEvent represents a quota notification event
type QuotaEvent struct {
	Type             QuotaEventType  `json:"type"`
	Timestamp        time.Time       `json:"timestamp"`
	GlobalState      quota.QuotaState `json:"global_state"`
	UsagePercentage  float64         `json:"usage_percentage"`
	UnitsUsed        int             `json:"units_used"`
	UnitsLimit       int             `json:"units_limit"`
	UnitsRemaining   int             `json:"units_remaining"`
	PreviousState    *quota.QuotaState `json:"previous_state,omitempty"`
	AffectedChannels []string        `json:"affected_channels,omitempty"`
	Message          string          `json:"message"`
	Severity         string          `json:"severity"` // info, warning, error, critical
}

// QuotaNotifier handles quota event notifications
type QuotaNotifier struct {
	redisClient *redis.Client
	logger      *zap.Logger
	enabled     bool
	channel     string
}

// NewQuotaNotifier creates a new quota notifier
func NewQuotaNotifier(redisClient *redis.Client, logger *zap.Logger, enabled bool, channel string) *QuotaNotifier {
	if channel == "" {
		channel = "quota:alerts"
	}

	logger.Info("Quota notifier initialized",
		zap.Bool("enabled", enabled),
		zap.String("channel", channel),
	)

	return &QuotaNotifier{
		redisClient: redisClient,
		logger:      logger,
		enabled:     enabled,
		channel:     channel,
	}
}

// NotifyStateTransition notifies about quota state transitions
func (n *QuotaNotifier) NotifyStateTransition(
	ctx context.Context,
	oldState, newState quota.QuotaState,
	percentage float64,
	used, limit int,
) error {
	if !n.enabled {
		return nil
	}

	// Determine severity based on new state
	severity := n.getSeverity(newState)

	// Create message
	message := fmt.Sprintf("Quota state changed from %s to %s (%.2f%% used)",
		oldState, newState, percentage)

	event := QuotaEvent{
		Type:            EventStateChanged,
		Timestamp:       time.Now(),
		GlobalState:     newState,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		PreviousState:   &oldState,
		Message:         message,
		Severity:        severity,
	}

	return n.publishEvent(ctx, event)
}

// NotifyThresholdCrossed notifies when a quota threshold is crossed
func (n *QuotaNotifier) NotifyThresholdCrossed(
	ctx context.Context,
	state quota.QuotaState,
	threshold float64,
	percentage float64,
	used, limit int,
) error {
	if !n.enabled {
		return nil
	}

	severity := n.getSeverity(state)
	message := fmt.Sprintf("Quota crossed %.0f%% threshold, now at %.2f%%", threshold, percentage)

	event := QuotaEvent{
		Type:            EventThresholdCrossed,
		Timestamp:       time.Now(),
		GlobalState:     state,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		Message:         message,
		Severity:        severity,
	}

	return n.publishEvent(ctx, event)
}

// NotifyQuotaExhausted notifies when quota is nearly exhausted
func (n *QuotaNotifier) NotifyQuotaExhausted(
	ctx context.Context,
	percentage float64,
	used, limit int,
	affectedChannels []string,
) error {
	if !n.enabled {
		return nil
	}

	message := fmt.Sprintf("Quota exhausted: %.2f%% used, %d channels affected",
		percentage, len(affectedChannels))

	event := QuotaEvent{
		Type:             EventQuotaExhausted,
		Timestamp:        time.Now(),
		GlobalState:      quota.QuotaStateExhausted,
		UsagePercentage:  percentage,
		UnitsUsed:        used,
		UnitsLimit:       limit,
		UnitsRemaining:   limit - used,
		AffectedChannels: affectedChannels,
		Message:          message,
		Severity:         "error",
	}

	return n.publishEvent(ctx, event)
}

// NotifyQuotaDepleted notifies when quota is completely depleted
func (n *QuotaNotifier) NotifyQuotaDepleted(
	ctx context.Context,
	used, limit int,
) error {
	if !n.enabled {
		return nil
	}

	message := fmt.Sprintf("Quota depleted: %d/%d units used, all API requests blocked",
		used, limit)

	event := QuotaEvent{
		Type:            EventQuotaDepleted,
		Timestamp:       time.Now(),
		GlobalState:     quota.QuotaStateDepleted,
		UsagePercentage: float64(used) / float64(limit) * 100,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  0,
		Message:         message,
		Severity:        "critical",
	}

	return n.publishEvent(ctx, event)
}

// NotifyQuotaRecovered notifies when quota recovers to healthy state
func (n *QuotaNotifier) NotifyQuotaRecovered(
	ctx context.Context,
	percentage float64,
	used, limit int,
) error {
	if !n.enabled {
		return nil
	}

	message := fmt.Sprintf("Quota recovered to healthy state: %.2f%% used", percentage)

	event := QuotaEvent{
		Type:            EventQuotaRecovered,
		Timestamp:       time.Now(),
		GlobalState:     quota.QuotaStateHealthy,
		UsagePercentage: percentage,
		UnitsUsed:       used,
		UnitsLimit:      limit,
		UnitsRemaining:  limit - used,
		Message:         message,
		Severity:        "info",
	}

	return n.publishEvent(ctx, event)
}

// NotifyChannelQuotaExceeded notifies when a channel exceeds its quota allocation
func (n *QuotaNotifier) NotifyChannelQuotaExceeded(
	ctx context.Context,
	channelID string,
	used, limit int,
) error {
	if !n.enabled {
		return nil
	}

	message := fmt.Sprintf("Channel %s exceeded quota allocation: %d/%d units",
		channelID, used, limit)

	event := QuotaEvent{
		Type:             EventChannelExceeded,
		Timestamp:        time.Now(),
		GlobalState:      quota.QuotaStateHealthy, // Not relevant for channel-specific events
		UsagePercentage:  float64(used) / float64(limit) * 100,
		UnitsUsed:        used,
		UnitsLimit:       limit,
		UnitsRemaining:   limit - used,
		AffectedChannels: []string{channelID},
		Message:          message,
		Severity:         "warning",
	}

	return n.publishEvent(ctx, event)
}

// publishEvent publishes an event to Redis and logs it
func (n *QuotaNotifier) publishEvent(ctx context.Context, event QuotaEvent) error {
	// Log the event
	n.logEvent(event)

	// Publish to Redis pub/sub channel
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

// logEvent logs the event with appropriate log level
func (n *QuotaNotifier) logEvent(event QuotaEvent) {
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
	case "info":
		n.logger.Info("Quota Alert (INFO)", fields...)
	default:
		n.logger.Info("Quota Alert", fields...)
	}
}

// getSeverity determines severity based on quota state
func (n *QuotaNotifier) getSeverity(state quota.QuotaState) string {
	switch state {
	case quota.QuotaStateDepleted:
		return "critical"
	case quota.QuotaStateExhausted:
		return "error"
	case quota.QuotaStateCritical:
		return "error"
	case quota.QuotaStateDegraded:
		return "warning"
	case quota.QuotaStateHealthy:
		return "info"
	default:
		return "info"
	}
}
