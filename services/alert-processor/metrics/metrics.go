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

// Package metrics defines the alert-processor's Prometheus instrumentation.
// All four counters are labelled by platform and event_type so a single
// platform's event source going quiet is visible per event type, not averaged
// into a service-wide rate.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	eventsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alert_processor_events_consumed_total",
		Help: "Stream entries parsed and accepted for alert processing (chat entries included).",
	}, []string{"platform", "event_type"})

	eventsRouted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alert_processor_events_routed_total",
		Help: "Alert events fanned out to an alert-capable overlay (one per event per overlay).",
	}, []string{"platform", "event_type"})

	eventsPersisted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alert_processor_events_persisted_total",
		Help: "alert_events rows written (attempted; re-deliveries are absorbed by ON CONFLICT).",
	}, []string{"platform", "event_type"})

	publishErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alert_processor_publish_errors_total",
		Help: "Failed publishes to overlay:{id}:alerts; the alert is redelivered.",
	}, []string{"platform", "event_type"})
)

// RecordConsumed counts one parsed stream entry. event_type is "chat" for
// chat lines, so the consumed/routed delta is exactly the chat volume the
// alert path deliberately does not touch.
func RecordConsumed(platform, eventType string) {
	eventsConsumed.WithLabelValues(platform, eventType).Inc()
}

// RecordRouted counts one alert delivered to one overlay.
func RecordRouted(platform, eventType string) {
	eventsRouted.WithLabelValues(platform, eventType).Inc()
}

// RecordPersisted counts one alert_events write attempt.
func RecordPersisted(platform, eventType string) {
	eventsPersisted.WithLabelValues(platform, eventType).Inc()
}

// RecordPublishError counts one failed alert publish.
func RecordPublishError(platform, eventType string) {
	publishErrors.WithLabelValues(platform, eventType).Inc()
}
