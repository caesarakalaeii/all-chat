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

package subscribers

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics are the subscriber poller's own metrics, deliberately NOT part of
// InnerTubeMetrics: the canary alerts (YouTubeInnerTubeCapturingNothing,
// YouTubeInnerTubeCanaryDown) read the chat pipeline's metrics, and a
// subscriber-poll failure must never read as ingestion failure.
type Metrics struct {
	SubscribersAnnounced *prometheus.CounterVec // labels: channel_id
	Polls                *prometheus.CounterVec // labels: channel_id
	APIErrors            *prometheus.CounterVec // labels: channel_id, error_class
	TokenErrors          *prometheus.CounterVec // labels: channel_id, error_class
	Refreshes            *prometheus.CounterVec // labels: channel_id, result
	QuotaSkips           *prometheus.CounterVec // labels: channel_id
	AnnounceErrors       *prometheus.CounterVec // labels: channel_id
}

// metricPrefix namespaces the metrics below the youtube_innertube_subscribers_
// family, keeping them unreadable by the chat-pipeline alerts.
const metricPrefix = "youtube_innertube_subscribers_"

// NewMetrics registers subscriber metrics with reg. Passing nil uses the
// default registerer.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	m := &Metrics{
		SubscribersAnnounced: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "announced_total",
			Help: "New public subscribers announced to chat:raw.",
		}, []string{"channel_id"}),
		Polls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "polls_total",
			Help: "Subscriber list polls performed (quota-spending).",
		}, []string{"channel_id"}),
		APIErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "api_errors_total",
			Help: "Subscriptions.list call failures, by error class.",
		}, []string{"channel_id", "error_class"}),
		TokenErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "token_errors_total",
			Help: "Credential resolution/refresh failures, by error class. invalid_grant means the channel must re-auth.",
		}, []string{"channel_id", "error_class"}),
		Refreshes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "token_refreshes_total",
			Help: "Reactive OAuth refreshes (the scheduled refresher owns routine freshness).",
		}, []string{"channel_id", "result"}),
		QuotaSkips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "quota_skips_total",
			Help: "Polls skipped because quota reservation failed or errored.",
		}, []string{"channel_id"}),
		AnnounceErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricPrefix + "announce_errors_total",
			Help: "Failures publishing a subscriber event to chat:raw.",
		}, []string{"channel_id"}),
	}
	reg.MustRegister(
		m.SubscribersAnnounced, m.Polls, m.APIErrors, m.TokenErrors,
		m.Refreshes, m.QuotaSkips, m.AnnounceErrors,
	)
	return m
}

// NopMetrics returns metrics writing to a throwaway registerer, for callers
// that opt out of observability (tests).
func NopMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	return NewMetrics(reg)
}
