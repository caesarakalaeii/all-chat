package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BusinessMetrics provides business-level metrics for the platform
type BusinessMetrics struct {
	// User Growth
	UserRegistrations *prometheus.CounterVec

	// User Engagement
	ActiveOverlays         *prometheus.GaugeVec
	OverlayViews           *prometheus.CounterVec
	OverlaySessionDuration *prometheus.HistogramVec

	// Platform Usage
	MessagesByPlatform        *prometheus.CounterVec
	ActiveUsers               *prometheus.GaugeVec
	ConnectedPlatformsPerUser *prometheus.GaugeVec

	// Source Management
	ActiveSourcesTotal *prometheus.GaugeVec
	SourceOperations   *prometheus.CounterVec
}

// NewBusinessMetrics creates a new set of business metrics registered with the
// default prometheus registry (via promauto).
func NewBusinessMetrics() *BusinessMetrics {
	return newBusinessMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// newBusinessMetricsWithRegistry creates BusinessMetrics registered with the
// provided registerer. Used by tests to avoid conflicts with the default registry.
func newBusinessMetricsWithRegistry(reg prometheus.Registerer) *BusinessMetrics {
	factory := promauto.With(reg)
	return &BusinessMetrics{
		UserRegistrations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "allchat_user_registrations_total",
				Help: "Total new user registrations by auth platform",
			},
			[]string{"platform"},
		),
		ActiveOverlays: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "allchat_active_overlays_total",
				Help: "Number of actively used overlays (with WebSocket connections)",
			},
			[]string{},
		),
		OverlayViews: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "allchat_overlay_views_total",
				Help: "Overlay page views",
			},
			[]string{"overlay_id", "view_type"},
		),
		OverlaySessionDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "allchat_overlay_session_duration_seconds",
				Help:    "How long overlays are actively viewed/used",
				Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800},
			},
			[]string{},
		),
		MessagesByPlatform: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "allchat_messages_by_platform_total",
				Help: "Total messages delivered by platform",
			},
			[]string{"platform"},
		),
		ActiveUsers: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "allchat_active_users_total",
				Help: "Number of users with active sessions",
			},
			[]string{},
		),
		ConnectedPlatformsPerUser: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "allchat_connected_platforms_per_user",
				Help: "Average platforms connected per user",
			},
			[]string{},
		),
		ActiveSourcesTotal: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "allchat_active_sources_total",
				Help: "Total number of active chat sources across all platforms",
			},
			[]string{"platform"},
		),
		SourceOperations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "allchat_source_operations_total",
				Help: "Source management operations (add, remove, update)",
			},
			[]string{"operation", "platform", "result"},
		),
	}
}

// RecordUserRegistration increments the user registration counter for the given platform.
func (m *BusinessMetrics) RecordUserRegistration(platform string) {
	m.UserRegistrations.WithLabelValues(platform).Inc()
}

// SetActiveOverlays sets the number of active overlays
func (m *BusinessMetrics) SetActiveOverlays(count int) {
	m.ActiveOverlays.WithLabelValues().Set(float64(count))
}

// RecordOverlayView records an overlay view
func (m *BusinessMetrics) RecordOverlayView(overlayID, viewType string) {
	m.OverlayViews.WithLabelValues(overlayID, viewType).Inc()
}

// RecordMessageByPlatform records a message from a specific platform
func (m *BusinessMetrics) RecordMessageByPlatform(platform string) {
	m.MessagesByPlatform.WithLabelValues(platform).Inc()
}

// SetActiveUsers sets the number of active users
func (m *BusinessMetrics) SetActiveUsers(count int) {
	m.ActiveUsers.WithLabelValues().Set(float64(count))
}

// SetActiveSourcesTotal sets the total active sources for a platform
func (m *BusinessMetrics) SetActiveSourcesTotal(platform string, count int) {
	m.ActiveSourcesTotal.WithLabelValues(platform).Set(float64(count))
}

// RecordSourceOperation records a source management operation
func (m *BusinessMetrics) RecordSourceOperation(operation, platform, result string) {
	m.SourceOperations.WithLabelValues(operation, platform, result).Inc()
}
