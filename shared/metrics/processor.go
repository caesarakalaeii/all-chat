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

// ProcessorMetrics provides metrics for the message processor service
type ProcessorMetrics struct {
	// Processing Pipeline
	MessagesConsumed       *prometheus.CounterVec
	MessagesProcessed      *prometheus.CounterVec
	ProcessingDuration     *prometheus.HistogramVec
	StageDuration          *prometheus.HistogramVec

	// Chat content health: total chat messages vs. those arriving with empty
	// text. A sudden spike in the empty ratio for a platform is the signature of
	// a decoder/library break (e.g. an unofficial listener whose upstream schema
	// drifted) where messages still flow but their text silently vanishes —
	// invisible to liveness/error/lag alerts. Events (gift/follow/like) are
	// excluded; only real chat messages are counted.
	ChatMessages      *prometheus.CounterVec
	ChatMessagesEmpty *prometheus.CounterVec

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

	// Deletion Buffer
	DeletionsBuffered        prometheus.Counter
	BufferedDeletionsApplied prometheus.Counter

	// Resilience (DLQ, PEL, publish retry)
	PELPendingMessages *prometheus.GaugeVec
	DLQMessagesTotal   *prometheus.CounterVec
	PublishRetryTotal  *prometheus.CounterVec
	DLQWriteFailures   prometheus.Counter

	// Engagement hot-path hook (issue #523): candidate vote/wager forwards.
	EngagementForwardTotal *prometheus.CounterVec
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
		ChatMessages: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_chat_messages_total",
				Help: "Chat messages (excluding events) consumed, by platform. Denominator for the empty-text ratio.",
			},
			[]string{"service", "platform"},
		),
		ChatMessagesEmpty: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_chat_messages_empty_total",
				Help: "Chat messages that arrived with empty/whitespace-only text, by platform. A high ratio signals a broken decoder.",
			},
			[]string{"service", "platform"},
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
		DeletionsBuffered: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "processor_deletions_buffered_total",
				Help: "Number of deletion events buffered for messages not yet in registry",
			},
		),
		BufferedDeletionsApplied: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "processor_buffered_deletions_applied_total",
				Help: "Number of buffered deletions applied when message arrived",
			},
		),
		PELPendingMessages: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "processor_pel_pending_messages",
				Help: "Number of pending PEL messages awaiting acknowledgement per consumer",
			},
			[]string{"consumer"},
		),
		DLQMessagesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_dlq_messages_total",
				Help: "Total messages written to the dead-letter queue",
			},
			[]string{"source_service", "reason"},
		),
		PublishRetryTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_publish_retry_total",
				Help: "Total Pub/Sub publish retry attempts",
			},
			[]string{"attempt"},
		),
		DLQWriteFailures: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "processor_dlq_write_failures_total",
				Help: "Total failures writing to the dead-letter queue",
			},
		),
		EngagementForwardTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "processor_engagement_forward_total",
				Help: "Engagement vote/wager command forwards from the hot path, by result (hit|miss|error|dropped)",
			},
			[]string{"result"},
		),
	}
}

// RecordEngagementForward records the outcome of a hot-path engagement command
// forward (hit = queued to the stream, miss = no live round, error = Redis failure,
// dropped = forwarder buffer full).
func (m *ProcessorMetrics) RecordEngagementForward(result string) {
	m.EngagementForwardTotal.WithLabelValues(result).Inc()
}

// RecordMessageConsumed records a message consumed from Redis stream
func (m *ProcessorMetrics) RecordMessageConsumed(service, platform, consumerGroup string) {
	m.MessagesConsumed.WithLabelValues(service, platform, consumerGroup).Inc()
}

// RecordMessageProcessed records a message processed through a pipeline stage
func (m *ProcessorMetrics) RecordMessageProcessed(service, platform, stage, result string) {
	m.MessagesProcessed.WithLabelValues(service, platform, stage, result).Inc()
}

// RecordChatText records a chat message's text presence, once per raw message.
// Call only for real chat messages (not gift/follow/like events, which carry
// synthetic text). When empty is true the message arrived with no visible text —
// a per-platform spike in the empty ratio flags a broken/stale decoder.
func (m *ProcessorMetrics) RecordChatText(service, platform string, empty bool) {
	m.ChatMessages.WithLabelValues(service, platform).Inc()
	if empty {
		m.ChatMessagesEmpty.WithLabelValues(service, platform).Inc()
	}
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
