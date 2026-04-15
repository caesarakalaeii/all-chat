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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gatewayEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "discord_listener_gateway_events_total",
		Help: "Total Gateway dispatch events received by type",
	}, []string{"type"})

	activeGuildsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "discord_listener_active_guilds",
		Help: "Number of guilds with at least one configured source",
	})

	shardOwnershipGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "discord_listener_shard_ownership",
		Help: "1 if this pod holds shard ownership, 0 otherwise",
	})

	resumeAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "discord_listener_resume_attempts_total",
		Help: "Gateway RESUME attempts",
	}, []string{"result"})

	messagesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "discord_listener_messages_received_total",
		Help: "Total chat messages received from Discord",
	}, []string{"guild_id", "channel_id"})

	messagesPublishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "discord_listener_messages_published_total",
		Help: "Total messages published to Redis",
	}, []string{"result"})
)

func labelValue(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

// IncGatewayEvent increments the gateway events counter for the given event type.
func IncGatewayEvent(eventType string) {
	gatewayEventsTotal.WithLabelValues(labelValue(eventType)).Inc()
}

// SetActiveGuilds sets the active guilds gauge.
func SetActiveGuilds(count int) {
	activeGuildsGauge.Set(float64(count))
}

// SetShardOwnership sets the shard ownership gauge (1=held, 0=not held).
func SetShardOwnership(held int) {
	shardOwnershipGauge.Set(float64(held))
}

// IncResumeAttempt increments the resume attempts counter with the given result label.
// result values: "success", "fallback_identify".
func IncResumeAttempt(result string) {
	resumeAttemptsTotal.WithLabelValues(labelValue(result)).Inc()
}

// IncMessageReceived increments the messages received counter.
// guildID and channelID identify the source of the message.
func IncMessageReceived(guildID, channelID string) {
	messagesReceivedTotal.WithLabelValues(labelValue(guildID), labelValue(channelID)).Inc()
}

// IncMessagePublished increments the messages published counter.
// result values: "success", "error".
func IncMessagePublished(result string) {
	messagesPublishedTotal.WithLabelValues(labelValue(result)).Inc()
}
