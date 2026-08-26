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

// Package metrics holds payment-service's Prometheus instrumentation for Patreon
// entitlement resolution.
//
// These exist because of a failure mode that `up`-style liveness cannot see: the
// service was Ready, scraped fine, and answered every request, while silently
// resolving every paying patron to "no membership" and revoking their premium on the
// next reconcile pass. Entitlement is derived from a third party's response shape, so
// "the process is healthy" and "the answers are right" are independent properties and
// need independent signals.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// trackedStatuses are the subscription status label values ConnectionsByStatus is
// pre-initialised with. Pre-seeding matters more here than for a dashboard metric:
// an alert comparing `active == 0` must be able to observe a *present* series at
// zero. Absent series make the comparison vacuous and the alert silently unfirable,
// which is the same class of invisible failure these metrics exist to catch.
var trackedStatuses = []string{"active", "none", "declined", "former", "expired"}

var (
	// UnmatchedMembers counts Patreon member resources we received but could not
	// attribute to our campaign, and therefore discarded. The identity API only
	// serializes a member's campaign relationship when `include` requests it, so a
	// regression there turns every patron into an apparent non-patron. Any sustained
	// increase means we are throwing away real membership data.
	UnmatchedMembers = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_patreon_unmatched_members_total",
		Help: "Patreon member resources discarded because they could not be attributed to our campaign",
	})

	// ConnectionsByStatus is how many Patreon connections resolved to each
	// subscription status on the last completed reconcile pass. Reported as a gauge
	// of the whole population rather than a counter of transitions, because the
	// question worth alerting on is a statement about the current fleet: "we have
	// connections, and none of them are active."
	ConnectionsByStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "payment_patreon_connections",
		Help: "Patreon connections by resolved subscription status as of the last reconcile pass",
	}, []string{"status"})

	// ReconcileLastSuccess is the Unix timestamp of the last reconcile pass that
	// completed. Without it the gauges above are indistinguishable from fresh when
	// the loop dies: Prometheus keeps serving the last value it saw, so a stalled
	// reconciler looks exactly like a healthy steady state.
	ReconcileLastSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "payment_patreon_reconcile_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last completed Patreon reconcile pass",
	})
)

func init() {
	for _, s := range trackedStatuses {
		ConnectionsByStatus.WithLabelValues(s)
	}
}

// ObserveReconcileStatuses publishes the status tally of a completed reconcile pass.
// Statuses absent from counts are reset to 0 rather than left untouched, so a status
// that stops occurring falls to zero instead of holding its last value forever.
func ObserveReconcileStatuses(counts map[string]int) {
	for _, s := range trackedStatuses {
		ConnectionsByStatus.WithLabelValues(s).Set(float64(counts[s]))
	}
	for s, n := range counts {
		if !tracked(s) {
			ConnectionsByStatus.WithLabelValues(s).Set(float64(n))
		}
	}
}

func tracked(status string) bool {
	for _, s := range trackedStatuses {
		if s == status {
			return true
		}
	}
	return false
}
