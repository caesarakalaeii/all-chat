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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	socketState = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "goodgame_listener_socket_state",
		Help: "Current GoodGame WebSocket connection state (1=connected, 0=disconnected)",
	})

	reconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goodgame_listener_reconnects_total",
		Help: "Number of WebSocket reconnect attempts",
	}, []string{"reason"})

	subscriptionEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goodgame_listener_subscription_events_total",
		Help: "Subscription lifecycle events processed by the channel manager",
	}, []string{"action"})

	activeSubscriptionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "goodgame_listener_active_subscriptions",
		Help: "Number of active GoodGame channel subscriptions",
	})

	channelViewersGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "goodgame_listener_channel_viewers",
		Help: "Live client count per channel from channel_counters frames",
	}, []string{"channel_id"})

	messagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goodgame_listener_messages_total",
		Help: "Count of messages handled by result",
	}, []string{"status", "reason"})

	publishLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "goodgame_listener_publish_latency_seconds",
		Help:    "Latency from frame receipt to Redis publish",
		Buckets: prometheus.DefBuckets,
	})

	droppedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goodgame_listener_dropped_messages_total",
		Help: "Messages dropped before publishing with reason labels",
	}, []string{"reason"})
)

func labelValue(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// SetSocketConnected updates the socket state gauge.
func SetSocketConnected(connected bool) {
	if connected {
		socketState.Set(1)
	} else {
		socketState.Set(0)
	}
}

// IncReconnect increments the reconnect counter.
func IncReconnect(reason string) {
	reconnectsTotal.WithLabelValues(labelValue(reason)).Inc()
}

// ObserveSubscription records a subscription lifecycle event.
func ObserveSubscription(action string) {
	subscriptionEvents.WithLabelValues(labelValue(action)).Inc()
}

// SetActiveSubscriptions sets the active subscription gauge.
func SetActiveSubscriptions(count int) {
	activeSubscriptionsGauge.Set(float64(count))
}

// SetChannelViewers sets the per-channel viewer gauge from channel_counters frames.
func SetChannelViewers(channelID string, viewers int64) {
	channelViewersGauge.WithLabelValues(labelValue(channelID)).Set(float64(viewers))
}

// ObservePublishLatency records the publish latency histogram.
func ObservePublishLatency(duration time.Duration) {
	publishLatency.Observe(duration.Seconds())
}

// IncMessage increments the generic message counter.
func IncMessage(status, reason string) {
	messagesTotal.WithLabelValues(labelValue(status), labelValue(reason)).Inc()
}

// IncDropped increments the dropped message counter.
func IncDropped(reason string) {
	droppedMessages.WithLabelValues(labelValue(reason)).Inc()
}
