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
	connectedStreams = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "altstream_listener_connected_streams",
		Help: "Altstream SSE streams currently open (one per watched channel)",
	})

	reconnectsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_reconnects_total",
		Help: "SSE stream reconnects by reason (stale, eof, read_error, connect_error, http_<status>)",
	}, []string{"reason"})

	subscriptionEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_subscription_events_total",
		Help: "Subscription lifecycle events processed by the channel manager",
	}, []string{"action"})

	activeSubscriptionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "altstream_listener_active_subscriptions",
		Help: "Number of Altstream channels this pod watches",
	})

	messagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_messages_total",
		Help: "Count of messages handled by result",
	}, []string{"status", "reason"})

	publishLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "altstream_listener_publish_latency_seconds",
		Help:    "Latency from stream receipt to Redis publish",
		Buckets: prometheus.DefBuckets,
	})

	droppedMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_dropped_messages_total",
		Help: "Events dropped before publishing with reason labels",
	}, []string{"reason"})

	ignoredEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_ignored_events_total",
		Help: "Altstream event types outside the v1 scope (chat, highlight, delete, clear), dropped",
	}, []string{"type"})

	restRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "altstream_listener_rest_requests_total",
		Help: "Altstream REST API requests (lastId bootstrap, emote catalogs) by result",
	}, []string{"endpoint", "result"})
)

// knownIgnoredTypes bounds the ignored_events type label.
var knownIgnoredTypes = map[string]bool{
	"redemption": true, "raid": true, "system": true, "shoutout": true, "tip": true, "subscription": true,
}

func labelValue(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// AddConnectedStreams moves the open-stream gauge by delta.
func AddConnectedStreams(delta int) {
	connectedStreams.Add(float64(delta))
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

// IncIgnored counts an out-of-scope event type; unknown types share "other".
func IncIgnored(eventType string) {
	if !knownIgnoredTypes[eventType] {
		eventType = "other"
	}
	ignoredEvents.WithLabelValues(eventType).Inc()
}

// IncRESTRequest counts one REST call by endpoint and result.
func IncRESTRequest(endpoint, result string) {
	restRequests.WithLabelValues(endpoint, labelValue(result)).Inc()
}
