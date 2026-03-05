package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// InnerTubeMetrics provides Prometheus metrics for InnerTube YouTube Listener
// These metrics are designed for canary deployment analysis via Argo Rollouts AnalysisTemplate.
// Metric names match the official youtube-listener exactly for PromQL compatibility.
type InnerTubeMetrics struct {
	// Error tracking - for error rate threshold analysis
	// Used by AnalysisTemplate to detect if canary error rate exceeds baseline
	Errors   *prometheus.CounterVec // labels: service, error_type (http, parse, rate_limit, redis)
	Requests *prometheus.CounterVec // labels: service

	// Message publishing - for message rate comparison
	// Used to verify canary publishes messages at similar rate to baseline
	MessagesPublished *prometheus.CounterVec // labels: service, channel_id

	// Redis health - for downstream failure detection
	// Used to detect Redis publish failures that would affect message delivery
	RedisPublishAttempts *prometheus.CounterVec   // labels: service
	RedisPublishSuccess  *prometheus.CounterVec   // labels: service
	RedisPublishLatency  *prometheus.HistogramVec // labels: service

	// Reconnection tracking - for stability monitoring
	// High reconnection rate indicates instability (potential rollback trigger)
	Reconnections *prometheus.CounterVec // labels: service, channel_id, reason
}

// NewInnerTubeMetrics creates and registers InnerTube Prometheus metrics
// All metrics use service label "youtube-listener-innertube-canary" for canary identification
func NewInnerTubeMetrics() *InnerTubeMetrics {
	return &InnerTubeMetrics{
		// Error tracking metrics
		Errors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_errors_total",
				Help: "Total errors encountered by YouTube listener by type",
			},
			[]string{"service", "error_type"},
		),
		Requests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_requests_total",
				Help: "Total API requests made by YouTube listener",
			},
			[]string{"service"},
		),

		// Message publishing metrics
		MessagesPublished: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_messages_published_total",
				Help: "Total messages published to Redis Streams by YouTube listener",
			},
			[]string{"service", "channel_id"},
		),

		// Redis health metrics
		RedisPublishAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_redis_publish_attempts_total",
				Help: "Total Redis publish attempts by YouTube listener",
			},
			[]string{"service"},
		),
		RedisPublishSuccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_redis_publish_success_total",
				Help: "Total successful Redis publishes by YouTube listener",
			},
			[]string{"service"},
		),
		RedisPublishLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "youtube_listener_redis_publish_latency_seconds",
				Help:    "Redis publish latency in seconds",
				Buckets: prometheus.DefBuckets, // Default buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
			[]string{"service"},
		),

		// Reconnection tracking metrics
		Reconnections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_reconnections_total",
				Help: "Total reconnection attempts by YouTube listener",
			},
			[]string{"service", "channel_id", "reason"},
		),
	}
}

// ServiceLabel is the fixed service label for InnerTube canary deployment
// Matches the Kubernetes Service name from canary deployment manifest
const ServiceLabel = "youtube-listener-innertube-canary"

// ErrorTypes define standardized error type labels for error tracking
const (
	ErrorTypeHTTP      = "http"       // HTTP errors (4xx, 5xx)
	ErrorTypeParse     = "parse"      // JSON parsing errors
	ErrorTypeRateLimit = "rate_limit" // Rate limiting (429)
	ErrorTypeRedis     = "redis"      // Redis publish failures
)

// ReconnectionReasons define standardized reason labels for reconnection tracking
const (
	ReconnectionReasonError    = "error"     // Reconnection after transient error
	ReconnectionReasonOffline  = "offline"   // Reconnection after stream went offline
	ReconnectionReasonBackoff  = "backoff"   // Reconnection after backoff period
	ReconnectionReasonRediscovery = "rediscovery" // Reconnection after stream rediscovery
)
