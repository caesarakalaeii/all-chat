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

// Package metrics mirrors kick-listener's Prometheus surface, per instance
// instead of per cluster.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	socketState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "owncast_listener_socket_state",
		Help: "Current Owncast WebSocket connection state per instance (1=connected, 0=disconnected)",
	}, []string{"instance"})

	reconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "owncast_listener_reconnects_total",
		Help: "Number of Owncast WebSocket reconnect attempts per instance",
	}, []string{"instance", "reason"})

	subscriptionEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "owncast_listener_subscription_events_total",
		Help: "Owncast source subscription lifecycle events processed by the channel manager",
	}, []string{"action"})

	activeSubscriptionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "owncast_listener_active_subscriptions",
		Help: "Number of active Owncast instance connections",
	})

	messagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "owncast_listener_messages_total",
		Help: "Count of messages handled by result",
	}, []string{"status", "reason"})

	publishLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "owncast_listener_publish_latency_seconds",
		Help:    "Latency from socket receipt to Redis publish",
		Buckets: prometheus.DefBuckets,
	})

	droppedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "owncast_listener_dropped_messages_total",
		Help: "Messages dropped before publishing with reason labels",
	}, []string{"reason"})
)

// SetSocketConnected updates the per-instance socket state gauge.
func SetSocketConnected(instanceURL string, connected bool) {
	v := 0.0
	if connected {
		v = 1.0
	}
	socketState.WithLabelValues(instanceURL).Set(v)
}

// IncReconnect increments the per-instance reconnect counter.
func IncReconnect(instanceURL, reason string) {
	reconnectsTotal.WithLabelValues(instanceURL, reason).Inc()
}

// ObserveSubscription records a subscription lifecycle event.
func ObserveSubscription(action string) {
	subscriptionEvents.WithLabelValues(action).Inc()
}

// SetActiveSubscriptions sets the active subscription gauge.
func SetActiveSubscriptions(count int) {
	activeSubscriptionsGauge.Set(float64(count))
}

// ObservePublishLatency records the publish latency histogram.
func ObservePublishLatency(duration time.Duration) {
	publishLatency.Observe(duration.Seconds())
}

// IncMessage increments the generic message counter.
func IncMessage(status, reason string) {
	messagesTotal.WithLabelValues(status, reason).Inc()
}

// IncDropped increments the dropped message counter.
func IncDropped(reason string) {
	droppedMessages.WithLabelValues(reason).Inc()
}
