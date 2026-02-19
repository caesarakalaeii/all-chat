package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ShardMetrics contains Prometheus metrics for sharding operations
type ShardMetrics struct {
	// Assignment operations
	AssignmentsTotal       prometheus.Counter
	AssignmentQueryLatency prometheus.Histogram
	AssignmentErrors       prometheus.Counter

	// Heartbeat operations
	HeartbeatsPublished prometheus.Counter
	HeartbeatErrors     prometheus.Counter

	// Pod health
	HealthyPods prometheus.Gauge
	FailedPods  prometheus.Gauge

	// Load distribution
	PodLoadMax         prometheus.Gauge
	PodLoadAvg         prometheus.Gauge
	LoadImbalanceRatio prometheus.Gauge // max_load / avg_load

	// Coordinator state
	CoordinatorIsLeader  prometheus.Gauge
	ReconciliationCycles prometheus.Counter
	ReconciliationErrors prometheus.Counter
	OrphanedAssignments  prometheus.Gauge
}

// NewShardMetrics creates and registers shard metrics
func NewShardMetrics() *ShardMetrics {
	return &ShardMetrics{
		AssignmentsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_assignments_total",
			Help: "Total number of channel assignments created",
		}),
		AssignmentQueryLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "shard_assignment_query_duration_seconds",
			Help:    "Assignment query latency in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		AssignmentErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_assignment_errors_total",
			Help: "Total number of assignment errors",
		}),
		HeartbeatsPublished: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_heartbeats_published_total",
			Help: "Total number of heartbeats published",
		}),
		HeartbeatErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_heartbeat_errors_total",
			Help: "Total number of heartbeat errors",
		}),
		HealthyPods: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_healthy_pods",
			Help: "Number of healthy listener pods",
		}),
		FailedPods: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_failed_pods",
			Help: "Number of failed listener pods",
		}),
		PodLoadMax: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_pod_load_max",
			Help: "Maximum channel count across all pods",
		}),
		PodLoadAvg: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_pod_load_avg",
			Help: "Average channel count per pod",
		}),
		LoadImbalanceRatio: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_imbalance_ratio",
			Help: "Load imbalance ratio (max_load / avg_load)",
		}),
		CoordinatorIsLeader: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_coordinator_is_leader",
			Help: "1 if this instance is coordinator leader, 0 otherwise",
		}),
		ReconciliationCycles: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_reconciliation_cycles_total",
			Help: "Total number of reconciliation cycles completed",
		}),
		ReconciliationErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_reconciliation_errors_total",
			Help: "Total number of reconciliation errors",
		}),
		OrphanedAssignments: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "shard_orphaned_assignments",
			Help: "Number of orphaned assignments detected",
		}),
	}
}
