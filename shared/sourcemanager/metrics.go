package sourcemanager

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	leadershipEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "source_manager_leadership_events_total",
		Help: "Leadership lifecycle events observed by Source Manager clients",
	}, []string{"platform", "event"})

	leadershipActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "source_manager_leadership_active",
		Help: "Number of active leadership leases held by this instance per platform",
	}, []string{"platform"})

	leadershipPeerCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "source_manager_leadership_peer_count",
		Help: "Number of active peers registered for this platform",
	}, []string{"platform"})

	leadershipDesired = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "source_manager_leadership_desired_total",
		Help: "Total number of streams that should be covered across all pods for this platform",
	}, []string{"platform"})

	leadershipRebalanceReleased = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "source_manager_leadership_rebalance_released_total",
		Help: "Cumulative number of leases released by rebalancing",
	}, []string{"platform"})
)

func observeLeadershipEvent(platform, event string) {
	leadershipEvents.WithLabelValues(sanitizeLabel(platform), sanitizeLabel(event)).Inc()
}

func setLeadershipActive(platform string, count int) {
	leadershipActive.WithLabelValues(sanitizeLabel(platform)).Set(float64(count))
}

func sanitizeLabel(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
