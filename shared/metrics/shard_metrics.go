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
	PodLoadScore       *prometheus.GaugeVec // Per-pod composite load scores

	// Coordinator state
	CoordinatorIsLeader  prometheus.Gauge
	ReconciliationCycles prometheus.Counter
	ReconciliationErrors prometheus.Counter
	OrphanedAssignments  prometheus.Gauge

	// Rebalancing operations (Phase 7)
	RebalancingTotal             prometheus.Counter // Total rebalancing operations triggered
	RebalancingCooldownOverrides prometheus.Counter // Escalation overrides
	RebalancingThrashing         prometheus.Counter // Thrashing events detected

	// Migration operations (Phase 8)
	MigrationTotal    *prometheus.CounterVec // Total migrations by status and reason
	MigrationDuration prometheus.Histogram   // Migration duration in seconds
	PodChannelCount   *prometheus.GaugeVec   // Number of channels assigned to each pod
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
		PodLoadScore: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "shard_pod_load_score",
			Help: "Per-pod composite load score (message_rate * 0.7 + channel_count * 0.3)",
		},
			[]string{"pod_id"},
		),
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
		RebalancingTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_rebalancing_total",
			Help: "Total number of rebalancing operations triggered",
		}),
		RebalancingCooldownOverrides: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_rebalancing_cooldown_overrides_total",
			Help: "Total number of escalation overrides that broke cooldown",
		}),
		RebalancingThrashing: promauto.NewCounter(prometheus.CounterOpts{
			Name: "shard_rebalancing_thrashing_total",
			Help: "Total number of thrashing events detected (>3 rebalances in 15min)",
		}),
		MigrationTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "shard_migration_total",
			Help: "Total number of channel migrations",
		},
			[]string{"status", "reason"},
		),
		MigrationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "shard_migration_duration_seconds",
			Help:    "Migration duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		}),
		PodChannelCount: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "shard_channel_count",
			Help: "Number of channels assigned to this pod",
		},
			[]string{"pod_id"},
		),
	}
}
