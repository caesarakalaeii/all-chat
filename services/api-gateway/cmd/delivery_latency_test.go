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

package main

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// A chat frame records both halves of the picture: how long this gateway took
// (delivery) and how long the whole pipeline took since Redis accepted the
// message (end-to-end).
func TestRecordDeliveryMetrics_RecordsBothHistograms(t *testing.T) {
	m := metrics.NewGatewayMetricsForTest()
	ingestedAt := time.Now().UTC().Add(-200 * time.Millisecond)

	recordDeliveryMetrics(m, "twitch", &ingestedAt, 5*time.Millisecond, time.Now().UTC())

	if got := testutil.CollectAndCount(m.MessageDeliveryLatency); got != 1 {
		t.Errorf("delivery latency series = %d, want 1", got)
	}
	if got := testutil.CollectAndCount(m.PipelineEndToEndLatency); got != 1 {
		t.Errorf("end-to-end latency series = %d, want 1", got)
	}
}

// Messages published before ingested_at existed, and buffered deletions replayed
// from the registry, carry no ingest time. Delivery latency is still measurable
// for them; end-to-end is not, and must not be invented from the current clock.
func TestRecordDeliveryMetrics_SkipsEndToEndWithoutIngestTime(t *testing.T) {
	m := metrics.NewGatewayMetricsForTest()

	recordDeliveryMetrics(m, "twitch", nil, 5*time.Millisecond, time.Now().UTC())

	if got := testutil.CollectAndCount(m.MessageDeliveryLatency); got != 1 {
		t.Errorf("delivery latency series = %d, want 1", got)
	}
	if got := testutil.CollectAndCount(m.PipelineEndToEndLatency); got != 0 {
		t.Errorf("end-to-end latency series = %d, want 0", got)
	}
}
