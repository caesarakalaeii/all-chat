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

// knownPlatforms lists all OAuth provider values that can appear in
// allchat_user_registrations_total and allchat_viewer_registrations_total.
// Pre-initialising these label combinations ensures the metrics are visible in
// /metrics even before the first registration occurs in the current pod's
// lifetime, which prevents Grafana from showing a flat line simply because no
// user signed up since the last pod restart.
var knownPlatforms = []string{"twitch", "youtube", "kick"}

// BusinessMetrics provides business-level metrics for the platform
type BusinessMetrics struct {
	// User Growth
	UserRegistrations   *prometheus.CounterVec
	ViewerRegistrations *prometheus.CounterVec
	// TotalUsersByPlatform is a gauge seeded from the database at startup so
	// that Grafana always has an accurate baseline even across pod restarts.
	TotalUsersByPlatform *prometheus.GaugeVec

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
	userRegs := factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "allchat_user_registrations_total",
			Help: "Total new streamer registrations by auth platform",
		},
		[]string{"platform"},
	)
	viewerRegs := factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "allchat_viewer_registrations_total",
			Help: "Total new viewer registrations by auth platform",
		},
		[]string{"platform"},
	)
	totalUsers := factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "allchat_total_users_by_platform",
			Help: "Total registered streamers per auth platform, seeded from the database at startup",
		},
		[]string{"platform"},
	)

	// Pre-initialise known label combinations so the metrics are always
	// present in /metrics, even before any sign-up happens in the current
	// pod's lifetime.
	for _, p := range knownPlatforms {
		userRegs.WithLabelValues(p)
		viewerRegs.WithLabelValues(p)
		totalUsers.WithLabelValues(p)
	}

	return &BusinessMetrics{
		UserRegistrations:    userRegs,
		ViewerRegistrations:  viewerRegs,
		TotalUsersByPlatform: totalUsers,
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

// RecordUserRegistration increments the streamer registration counter for the given platform.
func (m *BusinessMetrics) RecordUserRegistration(platform string) {
	m.UserRegistrations.WithLabelValues(platform).Inc()
}

// RecordViewerRegistration increments the viewer registration counter for the given platform.
func (m *BusinessMetrics) RecordViewerRegistration(platform string) {
	m.ViewerRegistrations.WithLabelValues(platform).Inc()
}

// InitTotalUsersByPlatform sets the TotalUsersByPlatform gauge from DB-sourced counts.
// Call this once at startup so Grafana has a persistent baseline across pod restarts.
func (m *BusinessMetrics) InitTotalUsersByPlatform(counts map[string]int64) {
	for platform, count := range counts {
		m.TotalUsersByPlatform.WithLabelValues(platform).Set(float64(count))
	}
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
