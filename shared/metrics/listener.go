package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ListenerMetrics provides common metrics for all chat listener services
type ListenerMetrics struct {
	// Connection Health
	ConnectionStatus     *prometheus.GaugeVec
	ConnectionAttempts   *prometheus.CounterVec
	ConnectionDuration   *prometheus.HistogramVec

	// Channel/Source Monitoring
	ActiveSources  *prometheus.GaugeVec
	SourceEvents   *prometheus.CounterVec

	// Message Ingestion
	MessagesReceived  *prometheus.CounterVec
	MessagesPublished *prometheus.CounterVec
	MessageLatency    *prometheus.HistogramVec
	MessageRate       *prometheus.GaugeVec

	// Rate Limiting & Quotas
	RateLimitHits     *prometheus.CounterVec
	QuotaRemaining    *prometheus.GaugeVec
	QuotaUsagePercent *prometheus.GaugeVec

	// Platform API Calls
	APICallsTotal    *prometheus.CounterVec
	APICallDuration  *prometheus.HistogramVec

	// Error Tracking
	ErrorsTotal *prometheus.CounterVec
}

// NewListenerMetrics creates a new set of listener metrics for a specific platform
func NewListenerMetrics(platform, serviceName string) *ListenerMetrics {
	return &ListenerMetrics{
		ConnectionStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "listener_connection_status",
				Help: "Connection status to platform (1 = connected, 0 = disconnected)",
			},
			[]string{"platform", "service", "connection_type"},
		),
		ConnectionAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_connection_attempts_total",
				Help: "Total connection attempts",
			},
			[]string{"platform", "service", "result"},
		),
		ConnectionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "listener_connection_duration_seconds",
				Help:    "Duration of connection uptime before disconnect",
				Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800},
			},
			[]string{"platform", "service", "disconnect_reason"},
		),
		ActiveSources: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "listener_active_sources_total",
				Help: "Number of currently monitored channels/sources",
			},
			[]string{"platform", "service"},
		),
		SourceEvents: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_source_events_total",
				Help: "Source lifecycle events",
			},
			[]string{"platform", "service", "event"},
		),
		MessagesReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_messages_received_total",
				Help: "Total messages received from platform",
			},
			[]string{"platform", "service", "channel_id", "message_type"},
		),
		MessagesPublished: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_messages_published_total",
				Help: "Total messages published to Redis",
			},
			[]string{"platform", "service", "result"},
		),
		MessageLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "listener_message_latency_seconds",
				Help:    "Time from receiving message from platform to publishing to Redis",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"platform", "service"},
		),
		MessageRate: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "listener_message_rate_per_second",
				Help: "Current message ingestion rate (rolling average)",
			},
			[]string{"platform", "service", "channel_id"},
		),
		RateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_rate_limit_hits_total",
				Help: "Number of times rate limit was hit",
			},
			[]string{"platform", "service", "limit_type"},
		),
		QuotaRemaining: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "listener_quota_remaining",
				Help: "Remaining quota/rate limit capacity",
			},
			[]string{"platform", "service", "quota_type", "limit"},
		),
		QuotaUsagePercent: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "listener_quota_usage_percentage",
				Help: "Current quota usage as percentage (0-100)",
			},
			[]string{"platform", "service", "quota_type"},
		),
		APICallsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_api_calls_total",
				Help: "Total API calls to platform",
			},
			[]string{"platform", "service", "operation", "result", "error_type"},
		),
		APICallDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "listener_api_call_duration_seconds",
				Help:    "Duration of API calls",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"platform", "service", "operation"},
		),
		ErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "listener_errors_total",
				Help: "Total errors by category",
			},
			[]string{"platform", "service", "error_category", "severity"},
		),
	}
}

// RecordConnection records connection status
func (m *ListenerMetrics) RecordConnection(platform, service, connectionType string, connected bool) {
	val := 0.0
	if connected {
		val = 1.0
	}
	m.ConnectionStatus.WithLabelValues(platform, service, connectionType).Set(val)
}

// RecordConnectionAttempt records a connection attempt
func (m *ListenerMetrics) RecordConnectionAttempt(platform, service, result string) {
	m.ConnectionAttempts.WithLabelValues(platform, service, result).Inc()
}

// RecordMessage records a received message
func (m *ListenerMetrics) RecordMessage(platform, service, channelID, messageType string) {
	m.MessagesReceived.WithLabelValues(platform, service, channelID, messageType).Inc()
}

// RecordPublish records a message publish to Redis
func (m *ListenerMetrics) RecordPublish(platform, service, result string) {
	m.MessagesPublished.WithLabelValues(platform, service, result).Inc()
}

// RecordError records an error
func (m *ListenerMetrics) RecordError(platform, service, category, severity string) {
	m.ErrorsTotal.WithLabelValues(platform, service, category, severity).Inc()
}

// RecordAPICall records an API call
func (m *ListenerMetrics) RecordAPICall(platform, service, operation, result, errorType string) {
	m.APICallsTotal.WithLabelValues(platform, service, operation, result, errorType).Inc()
}

// SetActiveSources sets the current number of active sources
func (m *ListenerMetrics) SetActiveSources(platform, service string, count int) {
	m.ActiveSources.WithLabelValues(platform, service).Set(float64(count))
}

// RecordSourceEvent records a source lifecycle event
func (m *ListenerMetrics) RecordSourceEvent(platform, service, event string) {
	m.SourceEvents.WithLabelValues(platform, service, event).Inc()
}

// SetQuotaRemaining sets the remaining quota
func (m *ListenerMetrics) SetQuotaRemaining(platform, service, quotaType, limit string, remaining float64) {
	m.QuotaRemaining.WithLabelValues(platform, service, quotaType, limit).Set(remaining)
}

// SetQuotaUsagePercent sets the quota usage percentage
func (m *ListenerMetrics) SetQuotaUsagePercent(platform, service, quotaType string, percent float64) {
	m.QuotaUsagePercent.WithLabelValues(platform, service, quotaType).Set(percent)
}
