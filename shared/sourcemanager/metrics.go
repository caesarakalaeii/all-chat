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
