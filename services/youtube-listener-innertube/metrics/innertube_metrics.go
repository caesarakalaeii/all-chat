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

	// Deletion buffer tracking - for monitoring buffer overflow
	// High overflow rate indicates message deletion storm or buffer too small
	DeletionBufferOverflows *prometheus.CounterVec // labels: service, channel_id

	// Discovery attempt tracking - for abuse / runaway-loop detection.
	// A high rate(...) means we are scraping YouTube far more often than the
	// intended ~1/min cadence (duplicate loops, reset storms, etc.). Reason about it
	// per instance — max by (channel_id), not sum: discovery is not leader-gated, so
	// a fleet-wide sum scales with replica count and any threshold over it drifts on
	// every scale-up. DiscoveryGaveUp counts channels parked after
	// maxDiscoveryDuration so we can see chronically-offline sources.
	DiscoveryAttempts *prometheus.CounterVec // labels: service, channel_id
	DiscoveryGaveUp   *prometheus.CounterVec // labels: service, channel_id

	// Concurrent discovery loops per channel. The invariant is at most 1 per
	// channel per instance — a loop reserves m.discovering[channelID] before it
	// starts. Anything above 1 is a leaked loop, which multiplies our YouTube
	// scrape rate and cannot be cancelled through the normal demand path. This is
	// the direct signal for that; DiscoveryAttempts only shows it indirectly, as a
	// rate that has to be reasoned about against replica count and backoff phase.
	DiscoveryLoopsActive *prometheus.GaugeVec // labels: service, channel_id
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

		// Deletion buffer metrics
		DeletionBufferOverflows: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_deletion_buffer_overflows_total",
				Help: "Total deletion buffer overflows (oldest events dropped)",
			},
			[]string{"service", "channel_id"},
		),

		// Discovery attempt metrics
		DiscoveryAttempts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_discovery_attempts_total",
				Help: "Total stream-discovery attempts (YouTube scrapes) by channel. Alert on high rate per channel to catch runaway/abusive polling.",
			},
			[]string{"service", "channel_id"},
		),
		DiscoveryGaveUp: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "youtube_listener_discovery_gave_up_total",
				Help: "Total times discovery gave up on a channel after the max polling duration and parked awaiting a refresh.",
			},
			[]string{"service", "channel_id"},
		),
		DiscoveryLoopsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "youtube_listener_discovery_loops_active",
				Help: "Currently-running stream-discovery loops per channel on this instance. Must never exceed 1; >1 means a leaked loop is double-scraping YouTube.",
			},
			[]string{"service", "channel_id"},
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
	ErrorTypeNetwork   = "network"    // Network errors (DNS, connection, timeout)
)

// ReconnectionReasons define standardized reason labels for reconnection tracking
const (
	ReconnectionReasonError       = "error"       // Reconnection after transient error
	ReconnectionReasonOffline     = "offline"     // Reconnection after stream went offline
	ReconnectionReasonBackoff     = "backoff"     // Reconnection after backoff period
	ReconnectionReasonRediscovery = "rediscovery" // Reconnection after stream rediscovery
)
