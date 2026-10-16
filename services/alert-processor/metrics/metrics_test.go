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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// TestCounters_LabelAndIncrement: each record helper must move its own counter
// on the (platform, event_type) label pair. The counters are process-global,
// so assertions read the delta around the call.
func TestCounters_LabelAndIncrement(t *testing.T) {
	cases := []struct {
		name    string
		counter *prometheus.CounterVec
		record  func()
	}{
		{"consumed", eventsConsumed, func() { RecordConsumed("twitch", "bits") }},
		{"routed", eventsRouted, func() { RecordRouted("twitch", "bits") }},
		{"persisted", eventsPersisted, func() { RecordPersisted("twitch", "bits") }},
		{"publish_errors", publishErrors, func() { RecordPublishError("twitch", "bits") }},
	}
	for _, tc := range cases {
		child := tc.counter.WithLabelValues("twitch", "bits")
		before := testutil.ToFloat64(child)
		tc.record()
		assert.Equal(t, before+1, testutil.ToFloat64(child),
			"%s counter must increment on its label pair", tc.name)
	}
}
