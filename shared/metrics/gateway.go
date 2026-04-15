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

// GatewayMetrics provides metrics for the API Gateway service
type GatewayMetrics struct {
	// WebSocket Connections
	WebSocketConnections       *prometheus.GaugeVec
	WebSocketConnectionsTotal  *prometheus.CounterVec
	WebSocketConnectionDuration *prometheus.HistogramVec

	// Overlay Subscriptions
	OverlaySubscriptions       *prometheus.GaugeVec
	OverlaySubscriptionEvents  *prometheus.CounterVec

	// Message Distribution
	MessagesReceived           *prometheus.CounterVec
	MessagesSent               *prometheus.CounterVec
	MessageDeliveryLatency     *prometheus.HistogramVec
	MessagesDropped            *prometheus.CounterVec

	// HTTP Endpoints
	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDuration        *prometheus.HistogramVec

	// PubSub Reconnects
	PubSubReconnectTotal *prometheus.CounterVec // "pubsub_reconnect_total" labels: ["service", "overlay_id"]
}

// NewGatewayMetrics creates a new set of gateway metrics
func NewGatewayMetrics() *GatewayMetrics {
	return &GatewayMetrics{
		WebSocketConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_websocket_connections_active",
				Help: "Number of active WebSocket connections",
			},
			[]string{"service", "connection_type"},
		),
		WebSocketConnectionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_websocket_connections_total",
				Help: "Total WebSocket connection attempts",
			},
			[]string{"service", "result"},
		),
		WebSocketConnectionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_websocket_connection_duration_seconds",
				Help:    "Duration of WebSocket connections",
				Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800, 43200, 86400},
			},
			[]string{"service", "disconnect_reason"},
		),
		OverlaySubscriptions: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_overlay_subscriptions_active",
				Help: "Number of active overlay subscriptions",
			},
			[]string{"service", "overlay_id"},
		),
		OverlaySubscriptionEvents: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_overlay_subscription_events_total",
				Help: "Subscription lifecycle events",
			},
			[]string{"service", "event"},
		),
		MessagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_received_total",
				Help: "Messages received from Redis pub/sub",
			},
			[]string{"service", "overlay_id", "platform"},
		),
		MessagesSent: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_sent_total",
				Help: "Messages sent to WebSocket clients",
			},
			[]string{"service", "overlay_id", "result"},
		),
		MessageDeliveryLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_message_delivery_latency_seconds",
				Help:    "Time from receiving message from Redis to sending via WebSocket",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"service"},
		),
		MessagesDropped: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_dropped_total",
				Help: "Messages dropped (client disconnected, buffer full, etc.)",
			},
			[]string{"service", "reason"},
		),
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_http_requests_total",
				Help: "HTTP requests to gateway endpoints",
			},
			[]string{"service", "method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_request_duration_seconds",
				Help:    "HTTP request duration",
				Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"service", "method", "path"},
		),
		PubSubReconnectTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pubsub_reconnect_total",
				Help: "Total number of Redis Pub/Sub reconnect attempts",
			},
			[]string{"service", "overlay_id"},
		),
	}
}

// NewGatewayMetricsForTest creates a new set of gateway metrics using a fresh
// Prometheus registry. Use this in tests to avoid duplicate metric registration
// panics when NewGatewayMetrics() is called more than once per test binary.
func NewGatewayMetricsForTest() *GatewayMetrics {
	reg := prometheus.NewRegistry()
	f := promauto.With(reg)
	return &GatewayMetrics{
		WebSocketConnections: f.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_websocket_connections_active",
				Help: "Number of active WebSocket connections",
			},
			[]string{"service", "connection_type"},
		),
		WebSocketConnectionsTotal: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_websocket_connections_total",
				Help: "Total WebSocket connection attempts",
			},
			[]string{"service", "result"},
		),
		WebSocketConnectionDuration: f.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_websocket_connection_duration_seconds",
				Help:    "Duration of WebSocket connections",
				Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800, 43200, 86400},
			},
			[]string{"service", "disconnect_reason"},
		),
		OverlaySubscriptions: f.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "gateway_overlay_subscriptions_active",
				Help: "Number of active overlay subscriptions",
			},
			[]string{"service", "overlay_id"},
		),
		OverlaySubscriptionEvents: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_overlay_subscription_events_total",
				Help: "Subscription lifecycle events",
			},
			[]string{"service", "event"},
		),
		MessagesReceived: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_received_total",
				Help: "Messages received from Redis pub/sub",
			},
			[]string{"service", "overlay_id", "platform"},
		),
		MessagesSent: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_sent_total",
				Help: "Messages sent to WebSocket clients",
			},
			[]string{"service", "overlay_id", "result"},
		),
		MessageDeliveryLatency: f.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_message_delivery_latency_seconds",
				Help:    "Time from receiving message from Redis to sending via WebSocket",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"service"},
		),
		MessagesDropped: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_messages_dropped_total",
				Help: "Messages dropped (client disconnected, buffer full, etc.)",
			},
			[]string{"service", "reason"},
		),
		HTTPRequestsTotal: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gateway_http_requests_total",
				Help: "HTTP requests to gateway endpoints",
			},
			[]string{"service", "method", "path", "status"},
		),
		HTTPRequestDuration: f.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "gateway_http_request_duration_seconds",
				Help:    "HTTP request duration",
				Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"service", "method", "path"},
		),
		PubSubReconnectTotal: f.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pubsub_reconnect_total",
				Help: "Total number of Redis Pub/Sub reconnect attempts",
			},
			[]string{"service", "overlay_id"},
		),
	}
}

// RecordWebSocketConnection records active WebSocket connection status
func (m *GatewayMetrics) RecordWebSocketConnection(service, connectionType string, delta int) {
	m.WebSocketConnections.WithLabelValues(service, connectionType).Add(float64(delta))
}

// RecordWebSocketConnectionAttempt records a connection attempt
func (m *GatewayMetrics) RecordWebSocketConnectionAttempt(service, result string) {
	m.WebSocketConnectionsTotal.WithLabelValues(service, result).Inc()
}

// RecordOverlaySubscription records overlay subscription status
func (m *GatewayMetrics) RecordOverlaySubscription(service, overlayID string, delta int) {
	m.OverlaySubscriptions.WithLabelValues(service, overlayID).Add(float64(delta))
}

// RecordSubscriptionEvent records a subscription lifecycle event
func (m *GatewayMetrics) RecordSubscriptionEvent(service, event string) {
	m.OverlaySubscriptionEvents.WithLabelValues(service, event).Inc()
}

// RecordMessageReceived records a message received from Redis
func (m *GatewayMetrics) RecordMessageReceived(service, overlayID, platform string) {
	m.MessagesReceived.WithLabelValues(service, overlayID, platform).Inc()
}

// RecordMessageSent records a message sent via WebSocket
func (m *GatewayMetrics) RecordMessageSent(service, overlayID, result string) {
	m.MessagesSent.WithLabelValues(service, overlayID, result).Inc()
}

// RecordMessageDropped records a dropped message
func (m *GatewayMetrics) RecordMessageDropped(service, reason string) {
	m.MessagesDropped.WithLabelValues(service, reason).Inc()
}

// RecordHTTPRequest records an HTTP request
func (m *GatewayMetrics) RecordHTTPRequest(service, method, path, status string) {
	m.HTTPRequestsTotal.WithLabelValues(service, method, path, status).Inc()
}
