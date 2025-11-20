package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ProcessorMetrics provides metrics for the message processor service
type ProcessorMetrics struct {
	// Processing Pipeline
	MessagesConsumed       *prometheus.CounterVec
	MessagesProcessed      *prometheus.CounterVec
	ProcessingDuration     *prometheus.HistogramVec
	StageDuration          *prometheus.HistogramVec

	// Emote Enrichment
	EmoteLookups           *prometheus.CounterVec
	EmoteCacheEntries      *prometheus.GaugeVec
	EmoteCacheOperations   *prometheus.CounterVec
	EmoteEnrichmentDuration *prometheus.HistogramVec

	// Stream Health
	StreamLag              *prometheus.GaugeVec
	StreamErrors           *prometheus.CounterVec

	// Routing & Publishing
	MessagesPublished      *prometheus.CounterVec
	FanoutDuration         *prometheus.HistogramVec
}

// NewProcessorMetrics creates a new set of processor metrics
func NewProcessorMetrics() *ProcessorMetrics {
	return &ProcessorMetrics{
		MessagesConsumed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_messages_consumed_total",
				Help: "Messages consumed from Redis stream chat:raw",
			},
			[]string{"service", "platform", "consumer_group"},
		),
		MessagesProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_messages_processed_total",
				Help: "Messages processed through pipeline",
			},
			[]string{"service", "platform", "stage", "result"},
		),
		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "processor_message_processing_duration_seconds",
				Help:    "End-to-end message processing time",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"service", "platform"},
		),
		StageDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "processor_stage_duration_seconds",
				Help:    "Duration of each processing stage",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
			},
			[]string{"service", "platform", "stage"},
		),
		EmoteLookups: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_emote_lookups_total",
				Help: "Emote provider API lookups",
			},
			[]string{"service", "provider", "result"},
		),
		EmoteCacheEntries: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "processor_emote_cache_entries",
				Help: "Number of entries in emote cache",
			},
			[]string{"service", "provider"},
		),
		EmoteCacheOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_emote_cache_operations_total",
				Help: "Cache operations",
			},
			[]string{"service", "operation", "provider"},
		),
		EmoteEnrichmentDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "processor_emote_enrichment_duration_seconds",
				Help:    "Time to enrich message with emotes",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1},
			},
			[]string{"service", "emote_count_bucket"},
		),
		StreamLag: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "processor_stream_lag_seconds",
				Help: "Lag in consuming from Redis stream (time since last message)",
			},
			[]string{"service", "stream", "consumer_group"},
		),
		StreamErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_stream_errors_total",
				Help: "Stream consumption errors",
			},
			[]string{"service", "error_type"},
		),
		MessagesPublished: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_messages_published_total",
				Help: "Messages published to overlay-specific pub/sub channels",
			},
			[]string{"service", "overlay_id", "platform", "result"},
		),
		FanoutDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "processor_fanout_duration_seconds",
				Help:    "Time to publish message to all overlay subscribers",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
			},
			[]string{"service"},
		),
	}
}

// RecordMessageConsumed records a message consumed from Redis stream
func (m *ProcessorMetrics) RecordMessageConsumed(service, platform, consumerGroup string) {
	m.MessagesConsumed.WithLabelValues(service, platform, consumerGroup).Inc()
}

// RecordMessageProcessed records a message processed through a pipeline stage
func (m *ProcessorMetrics) RecordMessageProcessed(service, platform, stage, result string) {
	m.MessagesProcessed.WithLabelValues(service, platform, stage, result).Inc()
}

// RecordEmoteLookup records an emote provider lookup
func (m *ProcessorMetrics) RecordEmoteLookup(service, provider, result string) {
	m.EmoteLookups.WithLabelValues(service, provider, result).Inc()
}

// RecordEmoteCacheOperation records a cache operation
func (m *ProcessorMetrics) RecordEmoteCacheOperation(service, operation, provider string) {
	m.EmoteCacheOperations.WithLabelValues(service, operation, provider).Inc()
}

// SetEmoteCacheEntries sets the number of cache entries
func (m *ProcessorMetrics) SetEmoteCacheEntries(service, provider string, count int) {
	m.EmoteCacheEntries.WithLabelValues(service, provider).Set(float64(count))
}

// RecordStreamError records a stream error
func (m *ProcessorMetrics) RecordStreamError(service, errorType string) {
	m.StreamErrors.WithLabelValues(service, errorType).Inc()
}

// SetStreamLag sets the current stream lag
func (m *ProcessorMetrics) SetStreamLag(service, stream, consumerGroup string, lagSeconds float64) {
	m.StreamLag.WithLabelValues(service, stream, consumerGroup).Set(lagSeconds)
}

// RecordMessagePublished records a message published to overlay channel
func (m *ProcessorMetrics) RecordMessagePublished(service, overlayID, platform, result string) {
	m.MessagesPublished.WithLabelValues(service, overlayID, platform, result).Inc()
}
