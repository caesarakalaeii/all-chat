package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// YouTubeMetrics provides YouTube-specific operational metrics
type YouTubeMetrics struct {
	// Circuit Breaker Metrics
	CircuitBreakerState       *prometheus.GaugeVec   // 0=CLOSED, 1=HALF_OPEN, 2=OPEN
	CircuitBreakerFailures    *prometheus.GaugeVec   // Consecutive failures
	CircuitBreakerQuotaSaved  *prometheus.CounterVec // Units saved by blocking
	CircuitBreakerTransitions *prometheus.CounterVec // State transitions

	// Quota Drift Tracking
	QuotaDriftDetected     *prometheus.CounterVec // Drift detection events
	QuotaDriftUnits        *prometheus.GaugeVec   // Current drift in units
	QuotaDatabaseSync      *prometheus.CounterVec // Database sync operations
	QuotaDatabaseSyncError *prometheus.CounterVec // Failed syncs

	// Connection-Aware Polling
	PollerConnectionChecks  *prometheus.CounterVec // Connection checks before polls
	PollerStoppedByDisconnect *prometheus.CounterVec // Pollers stopped due to disconnect
	PollerQuotaSaved        *prometheus.CounterVec // Units saved by not polling

	// Emergency Shutoff
	EmergencyShutoffTriggers *prometheus.CounterVec // Emergency shutoff activations
	EmergencyShutoffBlocked  *prometheus.CounterVec // Requests blocked by shutoff
}

// NewYouTubeMetrics creates YouTube-specific metrics
func NewYouTubeMetrics() *YouTubeMetrics {
	return &YouTubeMetrics{
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "youtube_circuit_breaker_state",
				Help: "Circuit breaker state per channel (0=CLOSED, 1=HALF_OPEN, 2=OPEN)",
			},
			[]string{"channel_id"},
		),
		CircuitBreakerFailures: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "youtube_circuit_breaker_failures",
				Help: "Consecutive failures for circuit breaker",
			},
			[]string{"channel_id"},
		),
		CircuitBreakerQuotaSaved: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_circuit_breaker_quota_saved_units_total",
				Help: "Total quota units saved by circuit breaker blocking expensive calls",
			},
			[]string{"channel_id"},
		),
		CircuitBreakerTransitions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_circuit_breaker_transitions_total",
				Help: "Circuit breaker state transitions",
			},
			[]string{"channel_id", "from_state", "to_state"},
		),
		QuotaDriftDetected: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_quota_drift_detected_total",
				Help: "Number of times quota drift was detected during database sync",
			},
			[]string{"severity"}, // "minor" (<50 units), "major" (>=50 units)
		),
		QuotaDriftUnits: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "youtube_quota_drift_units",
				Help: "Current quota drift in units (database - memory)",
			},
			[]string{"direction"}, // "positive" (db > mem), "negative" (db < mem)
		),
		QuotaDatabaseSync: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_quota_database_sync_total",
				Help: "Database sync operations (periodic and on-demand)",
			},
			[]string{"result"}, // "success", "no_change", "synced"
		),
		QuotaDatabaseSyncError: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_quota_database_sync_errors_total",
				Help: "Failed database sync operations",
			},
			[]string{"error_type"},
		),
		PollerConnectionChecks: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_poller_connection_checks_total",
				Help: "Number of connection checks before polls",
			},
			[]string{"result"}, // "connected", "disconnected", "error"
		),
		PollerStoppedByDisconnect: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_poller_stopped_by_disconnect_total",
				Help: "Number of pollers stopped due to overlay disconnect",
			},
			[]string{"channel_id"},
		),
		PollerQuotaSaved: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_poller_quota_saved_units_total",
				Help: "Quota units saved by stopping polls when overlay disconnected",
			},
			[]string{"channel_id"},
		),
		EmergencyShutoffTriggers: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_emergency_shutoff_triggers_total",
				Help: "Number of times emergency shutoff was triggered",
			},
			[]string{"threshold"},
		),
		EmergencyShutoffBlocked: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_emergency_shutoff_blocked_total",
				Help: "Number of API calls blocked by emergency shutoff",
			},
			[]string{"operation", "allow_critical"},
		),
	}
}
